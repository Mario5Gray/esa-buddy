package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/meain/esa/internal/token"
	"github.com/sashabaranov/go-openai"
)

const (
	historyTimeFormat    = "20060102-150405"
	defaultModel         = "openai/gpt-5.2-2025-12-11"
	toolCallCommandColor = color.FgCyan
	toolCallOutputColor  = color.FgWhite
	maxRetryCount        = 5
	baseRetryDelay       = 1 * time.Second
	maxRetryDelay        = 1 * time.Minute
)

const (
	defaultCompactionMaxMessages = 40
	defaultCompactionKeepLast    = 12
	defaultCompactionMaxChars    = 20000
	compactionSummaryPrefix      = "Conversation summary (compacted):\n"
)

// Common error messages
const (
	errFailedToLoadConfig    = "failed to load global config"
	errFailedToSetupCache    = "failed to setup cache directory"
	errFailedToLoadHistory   = "failed to load conversation history"
	errFailedToUnmarshalHist = "failed to unmarshal conversation history"
	errFailedToLoadAgent     = "failed to load agent configuration"
	errFailedToSetupClient   = "failed to setup OpenAI client"
)

type Application struct {
	agent           Agent
	agentPath       string
	client          *openai.Client
	debug           bool
	historyFile     string
	messages        []openai.ChatCompletionMessage
	usage           token.Usage // accumulated token counts across LLM calls
	debugPrint      func(section string, v ...any)
	showCommands    bool
	showToolCalls   bool
	showProgress    bool
	lastProgressLen int
	modelFlag       string
	config          *Config
	mcpManager      *MCPManager
	cliAskLevel     string
	prettyOutput    bool
	thinkEnabled    *bool // nil = use agent default, true/false = CLI override
	compactPrompt   bool
	compactMaxMsgs  int
	compactKeepLast int
	compactMaxChars int
}

// providerInfo contains provider-specific configuration
type providerInfo struct {
	baseURL           string
	apiKeyEnvar       string
	apiKeyCanBeEmpty  bool
	additionalHeaders map[string]string
}

// parseModel parses model string in format "provider/model" and
// returns provider, model name, base URL and API key environment
// variable
func (app *Application) parseModel() (provider string, model string, info providerInfo) {
	return parseModel(app.modelFlag, app.agent, app.config)
}

func normalizeCompactionSettings(settings Settings) (enabled bool, maxMsgs int, keepLast int, maxChars int) {
	enabled = settings.PromptCompaction
	maxMsgs = settings.CompactionMaxMessages
	keepLast = settings.CompactionKeepLast
	maxChars = settings.CompactionMaxChars

	if maxMsgs <= 0 {
		maxMsgs = defaultCompactionMaxMessages
	}
	if keepLast <= 0 {
		keepLast = defaultCompactionKeepLast
	}
	if maxChars <= 0 {
		maxChars = defaultCompactionMaxChars
	}

	if keepLast >= maxMsgs {
		keepLast = maxMsgs / 2
		if keepLast < 1 {
			keepLast = 1
		}
	}

	return enabled, maxMsgs, keepLast, maxChars
}

// isRateLimitError checks if the error is a rate limit error (429)
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "Too Many Requests") ||
		strings.Contains(errStr, "rate limit")
}

// createChatCompletionWithRetry creates a chat completion stream with retry logic for rate limiting
func (app *Application) createChatCompletionWithRetry(tools []openai.Tool) (*openai.ChatCompletionStream, error) {
	var stream *openai.ChatCompletionStream
	var err error

	if err := app.compactMessagesIfNeeded(); err != nil {
		app.debugPrint("Compaction", fmt.Sprintf("Compaction skipped: %v", err))
	}

	// Retry logic for rate limiting
	for attempt := 0; attempt <= maxRetryCount; attempt++ {
		stream, err = app.client.CreateChatCompletionStream(
			context.Background(),
			openai.ChatCompletionRequest{
				Model:         app.getModel(),
				Messages:      app.messages,
				Tools:         tools,
				StreamOptions: &openai.StreamOptions{IncludeUsage: true},
			})

		if err == nil {
			return stream, nil // Success
		}

		if !isRateLimitError(err) {
			// Not a rate limit error, return immediately
			return nil, err
		}

		if attempt == maxRetryCount {
			// Last attempt failed
			return nil, fmt.Errorf("ChatCompletionStream error after %d retries: %w", maxRetryCount, err)
		}

		// Calculate delay and wait
		delay := calculateRetryDelay(attempt)
		app.debugPrint("Rate Limit",
			fmt.Sprintf("Rate limit hit, retrying in %v (attempt %d/%d)", delay, attempt+1, maxRetryCount))

		time.Sleep(delay)
	}

	return nil, err // Should never reach here, but for safety
}

func NewApplication(opts *CLIOptions) (*Application, error) {
	// Load global config first
	config, err := LoadConfig(opts.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errFailedToLoadConfig, err)
	}

	cacheDir, err := setupCacheDir()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errFailedToSetupCache, err)
	}

	var (
		messages []openai.ChatCompletionMessage
		usage    token.Usage
	)

	// If conversation index is set without retry, also set continue chat
	if len(opts.Conversation) > 0 && !opts.RetryChat {
		if _, err := findHistoryFile(cacheDir, opts.Conversation); err == nil {
			opts.ContinueChat = true
		}
	}

	if opts.ContinueChat || opts.RetryChat {
		if opts.Conversation == "" {
			opts.Conversation = "1"
		}
	}

	historyFile, hasHistory := getHistoryFilePath(cacheDir, opts)
	if hasHistory && (opts.ContinueChat || opts.RetryChat) {
		var history ConversationHistory
		data, err := os.ReadFile(historyFile)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", errFailedToLoadHistory, err)
		}
		err = json.Unmarshal(data, &history)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", errFailedToUnmarshalHist, err)
		}

		allMessages := history.Messages
		agentPath := history.AgentPath

		// Carry forward token counts from prior turns in this conversation
		if history.Usage != nil {
			usage = *history.Usage
		}

		app := &Application{debug: opts.DebugMode}
		app.debugPrint = createDebugPrinter(app.debug)

		if opts.RetryChat && len(allMessages) > 1 {
			// In retry mode, keep all messages up until the last user message
			var lastUserMessageIndex int = -1

			// Find the last user message in the history
			for i := len(allMessages) - 1; i >= 0; i-- {
				if allMessages[i].Role == "user" {
					lastUserMessageIndex = i
					break
				}
			}

			if lastUserMessageIndex >= 0 {
				// Keep all messages up to and including the last user message
				messages = allMessages[:lastUserMessageIndex+1]

				// If a command string was provided with -r, replace the last user message content
				if opts.CommandStr != "" {
					messages[lastUserMessageIndex].Content = opts.CommandStr
					app.debugPrint("Retry Mode",
						fmt.Sprintf("Keeping %d messages", len(messages)),
						fmt.Sprintf("Replacing last user message with: %q", opts.CommandStr),
						fmt.Sprintf("Agent: %s", agentPath), // Note: Agent might be overridden later if specified in opts
						fmt.Sprintf("History file: %q", historyFile),
					)
				} else {
					app.debugPrint("Retry Mode",
						fmt.Sprintf("Keeping %d messages", len(messages)),
						fmt.Sprintf("Agent: %s", agentPath), // Note: Agent might be overridden later if specified in opts
						fmt.Sprintf("History file: %q", historyFile),
					)
				}
			} else {
				// If we couldn't find a user message, just use the system message
				messages = []openai.ChatCompletionMessage{allMessages[0]}

				app.debugPrint("Retry Mode",
					fmt.Sprintf("No user messages found"),
					fmt.Sprintf("Agent: %s", agentPath),
					fmt.Sprintf("History file: %q", historyFile),
				)
			}
		} else {
			// In continue mode, use all messages
			messages = allMessages
			app.debugPrint("History",
				fmt.Sprintf("Loaded %d messages from history", len(messages)),
				fmt.Sprintf("Agent: %s", agentPath),
				fmt.Sprintf("History file: %q", historyFile),
			)
		}

		if agentPath != "" && opts.AgentPath == "" {
			opts.AgentPath = agentPath
		}

		// Use model from history if none specified in opts
		if history.Model != "" && opts.Model == "" {
			opts.Model = history.Model
		}
	}

	if opts.AgentPath == "" {
		opts.AgentPath = DefaultAgentPath
	}

	if strings.HasPrefix(opts.AgentPath, "builtin:") {
		opts.AgentName = strings.TrimPrefix(opts.AgentPath, "builtin:")
	}

	agent, err := loadConfiguration(opts)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errFailedToLoadAgent, err)
	}

	// If SystemPrompt is set in CLI options, override agent's SystemPrompt
	if opts.SystemPrompt != "" {
		agent.SystemPrompt = opts.SystemPrompt
	}

	client, err := setupOpenAIClient(opts.Model, agent, config)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errFailedToSetupClient, err)
	}

	showCommands := opts.ShowCommands || config.Settings.ShowCommands
	showToolCalls := opts.ShowToolCalls || config.Settings.ShowToolCalls
	compactPrompt, compactMaxMsgs, compactKeepLast, compactMaxChars := normalizeCompactionSettings(config.Settings)

	// Initialize MCP manager
	mcpManager := NewMCPManager()

	// Resolve think flag: CLI flags override agent config
	var thinkEnabled *bool
	if opts.Think {
		t := true
		thinkEnabled = &t
	} else if opts.NoThink {
		f := false
		thinkEnabled = &f
	}

	app := &Application{
		agent:           agent,
		agentPath:       opts.AgentPath,
		client:          client,
		historyFile:     historyFile,
		messages:        messages,
		usage:           usage,
		modelFlag:       opts.Model,
		config:          config,
		mcpManager:      mcpManager,
		cliAskLevel:     opts.AskLevel,
		prettyOutput:    opts.Pretty,
		thinkEnabled:    thinkEnabled,
		compactPrompt:   compactPrompt,
		compactMaxMsgs:  compactMaxMsgs,
		compactKeepLast: compactKeepLast,
		compactMaxChars: compactMaxChars,

		debug:         opts.DebugMode,
		showCommands:  showCommands && !showToolCalls && !opts.DebugMode,
		showToolCalls: showToolCalls && !opts.DebugMode,
		showProgress:  !opts.HideProgress && !opts.DebugMode && !(showCommands || showToolCalls),
	}

	app.debugPrint = createDebugPrinter(app.debug)
	provider, model, info := app.parseModel()

	app.debugPrint("Configuration",
		fmt.Sprintf("Provider: %q", provider),
		fmt.Sprintf("Model: %q", model),
		fmt.Sprintf("Base URL: %q", info.baseURL),
		fmt.Sprintf("API key envar: %q", info.apiKeyEnvar),
		fmt.Sprintf("Agent path: %q", opts.AgentPath),
		fmt.Sprintf("History file: %q", historyFile),
		fmt.Sprintf("Debug mode: %v", app.debug),
		fmt.Sprintf("Show commands: %v", app.showCommands),
		fmt.Sprintf("Show tool calls: %v", app.showToolCalls),
		fmt.Sprintf("Show progress: %v", app.showProgress),
	)

	return app, nil
}

func (app *Application) Run(opts CLIOptions) {
	// Start MCP servers if configured
	if len(app.agent.MCPServers) > 0 {
		ctx := context.Background()
		if err := app.mcpManager.StartServers(ctx, app.agent.MCPServers); err != nil {
			log.Fatalf("Failed to start MCP servers: %v", err)
		}
		// Ensure MCP servers are stopped when the application exits
		defer app.mcpManager.StopAllServers()

		app.debugPrint("MCP Servers", fmt.Sprintf("Started %d MCP servers", len(app.agent.MCPServers)))
	}

	prompt, err := app.getSystemPrompt()
	if err != nil {
		log.Fatalf("Error processing system prompt: %v", err)
	}

	if app.messages == nil {
		app.messages = []openai.ChatCompletionMessage{{
			Role:    "system",
			Content: prompt,
		}}
	}

	// Debug prints before starting communication
	app.debugPrint("System Message", app.messages[0].Content)

	input := readStdin()
	app.debugPrint("Input State",
		fmt.Sprintf("Command string: %q", opts.CommandStr),
		fmt.Sprintf("Stdin: %q", input),
	)

	// If in retry mode and a command string was provided,
	// it means we replaced the last user message content during loading.
	// Don't process input again.
	if !(opts.RetryChat && opts.CommandStr != "") {
		app.processInput(opts.CommandStr, input)
	}

	app.runConversationLoop(opts)
}

func (app *Application) processInput(commandStr, input string) {
	if len(input) > 0 {
		app.messages = append(app.messages, openai.ChatCompletionMessage{
			Role:    "user",
			Content: input,
		})
	}

	if len(commandStr) > 0 {
		app.messages = append(app.messages, openai.ChatCompletionMessage{
			Role:    "user",
			Content: commandStr,
		})
	}

	// If no input from stdin or command line, use initial message from agent config
	prompt, err := app.processInitialMessage(app.agent.InitialMessage)
	if err != nil {
		log.Fatalf("Error processing initial message: %v", err)
	}

	if len(input) == 0 && len(commandStr) == 0 && app.agent.InitialMessage != "" {
		app.messages = append(app.messages, openai.ChatCompletionMessage{
			Role:    "user",
			Content: prompt,
		})
	}
}

func (app *Application) processInitialMessage(message string) (string, error) {
	// Use the same processing logic as system prompt
	return app.processSystemPrompt(message)
}

func (app *Application) runConversationLoop(opts CLIOptions) {
	openAITools := convertFunctionsToTools(app.agent.Functions)

	// Add MCP tools
	mcpTools := app.mcpManager.GetAllTools()
	openAITools = append(openAITools, mcpTools...)

	for {
		stream, err := app.createChatCompletionWithRetry(openAITools)
		if err != nil {
			log.Fatalf("ChatCompletionStream error: %v", err)
		}

		assistantMsg := app.handleStreamResponse(stream)
		app.messages = append(app.messages, assistantMsg)

		// Save history after each assistant response
		app.saveConversationHistory()

		if len(assistantMsg.ToolCalls) == 0 {
			break
		}

		app.handleToolCalls(assistantMsg.ToolCalls, opts)

		// Save history after processing tool calls
		app.saveConversationHistory()
	}
}

func (app *Application) getModel() string {
	_, model, _ := app.parseModel()
	return model
}

// getEffectiveAskLevel returns the ask level to use, with CLI flag taking priority over agent config
func (app *Application) getEffectiveAskLevel() string {
	effectiveLevel := ""
	if app.cliAskLevel != "" {
		effectiveLevel = app.cliAskLevel
		app.debugPrint("Ask Level", fmt.Sprintf("Using CLI ask level: %s", effectiveLevel))
	} else if app.agent.Ask != "" {
		effectiveLevel = app.agent.Ask
		app.debugPrint("Ask Level", fmt.Sprintf("Using agent ask level: %s", effectiveLevel))
	} else {
		effectiveLevel = "unsafe"
		app.debugPrint("Ask Level", fmt.Sprintf("Using default ask level: %s", effectiveLevel))
	}
	return effectiveLevel
}

// filterThinkTags strips <think>...</think> blocks from streamed content.
// It handles tags split across chunk boundaries using a simple state machine.
// inThink tracks whether we're inside a think block; buf accumulates partial
// tag matches that may span chunks.
func filterThinkTags(chunk string, inThink *bool, buf *strings.Builder) string {
	const openTag = "<think>"
	const closeTag = "</think>"

	var out strings.Builder
	buf.WriteString(chunk)
	s := buf.String()
	buf.Reset()

	for len(s) > 0 {
		if *inThink {
			// Look for closing tag
			idx := strings.Index(s, closeTag)
			if idx >= 0 {
				*inThink = false
				s = s[idx+len(closeTag):]
				// Skip leading newline after closing tag
				if len(s) > 0 && s[0] == '\n' {
					s = s[1:]
				}
			} else {
				// Check if the end of s could be a partial </think>
				for i := 1; i < len(closeTag) && i <= len(s); i++ {
					if strings.HasSuffix(s, closeTag[:i]) {
						buf.WriteString(s[len(s)-i:])
						s = s[:len(s)-i]
						break
					}
				}
				// Discard everything before the potential partial match (it's inside think)
				s = ""
			}
		} else {
			// Look for opening tag
			idx := strings.Index(s, openTag)
			if idx >= 0 {
				out.WriteString(s[:idx])
				*inThink = true
				s = s[idx+len(openTag):]
			} else {
				// Check if the end of s could be a partial <think>
				buffered := false
				for i := 1; i < len(openTag) && i <= len(s); i++ {
					if strings.HasSuffix(s, openTag[:i]) {
						out.WriteString(s[:len(s)-i])
						buf.WriteString(s[len(s)-i:])
						buffered = true
						break
					}
				}
				if !buffered {
					out.WriteString(s)
				}
				s = ""
			}
		}
	}

	return out.String()
}

func (app *Application) handleStreamResponse(stream *openai.ChatCompletionStream) openai.ChatCompletionMessage {
	defer stream.Close()

	var assistantMsg openai.ChatCompletionMessage
	var fullContent strings.Builder
	hasContent := false

	// State machine for filtering <think>...</think> blocks from streamed content.
	// Chunks may split tags arbitrarily, so we buffer partial matches.
	inThink := false
	var thinkBuf strings.Builder

	for {
		response, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Stream error: %v", err)
		}

		// The final stream chunk carries token usage for the entire request.
		// Earlier chunks have Usage: nil; only the last one is populated.
		if response.Usage != nil {
			app.usage.Add(response.Usage.PromptTokens, response.Usage.CompletionTokens)
		}

		if len(response.Choices) == 0 {
			continue
		}

		if response.Choices[0].Delta.ToolCalls != nil {
			for _, toolCall := range response.Choices[0].Delta.ToolCalls {
				if toolCall.ID != "" {
					assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, toolCall)
				} else {
					lastToolCall := assistantMsg.ToolCalls[len(assistantMsg.ToolCalls)-1]
					lastToolCall.Function.Arguments += toolCall.Function.Arguments
					assistantMsg.ToolCalls[len(assistantMsg.ToolCalls)-1] = lastToolCall
				}
			}
		} else {
			// Clear progress line before showing result
			if app.showProgress && app.lastProgressLen > 0 {
				fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", app.lastProgressLen))
				app.lastProgressLen = 0
			}

			content := response.Choices[0].Delta.Content
			if content != "" {
				content = filterThinkTags(content, &inThink, &thinkBuf)
				if content != "" {
					hasContent = true
					if !app.prettyOutput {
						fmt.Print(content)
					}
					fullContent.WriteString(content)
				}
			}
		}
	}

	// Flush any remaining buffered content that turned out not to be a think tag
	if thinkBuf.Len() > 0 && !inThink {
		remaining := thinkBuf.String()
		if remaining != "" {
			hasContent = true
			if !app.prettyOutput {
				fmt.Print(remaining)
			}
			fullContent.WriteString(remaining)
		}
	}

	if hasContent {
		if app.prettyOutput {
			// TODO: Add support for rendering pretty markdown in a
			// streming manner (charmbracelet/glow/issues/601)
			printPrettyOutput(fullContent.String())
		} else {
			fmt.Println()
		}
	}

	assistantMsg.Role = "assistant"
	assistantMsg.Content = fullContent.String()
	return assistantMsg
}

type ConversationHistory struct {
	AgentPath string                         `json:"agent_path"`
	Model     string                         `json:"model"`
	Messages  []openai.ChatCompletionMessage `json:"messages"`
	Usage     *token.Usage                   `json:"usage,omitempty"` // nil in history files from before token tracking
}

func (app *Application) saveConversationHistory() {
	provider, model, _ := app.parseModel()
	modelString := fmt.Sprintf("%s/%s", provider, model)

	// Snapshot current usage for persistence. Pointer is nil-safe:
	// if no tokens tracked yet, omit from JSON for clean output.
	var usagePtr *token.Usage
	if !app.usage.Empty() {
		u := app.usage
		usagePtr = &u
	}

	history := ConversationHistory{
		AgentPath: app.agentPath,
		Model:     modelString,
		Messages:  app.messages,
		Usage:     usagePtr,
	}

	if data, err := json.Marshal(history); err == nil {
		if err := os.WriteFile(app.historyFile, data, 0644); err != nil {
			app.debugPrint("Error", fmt.Sprintf("Failed to save history: %v", err))
		}
	}
}

func (app *Application) generateProgressSummary(funcName string, args string) string {
	return fmt.Sprintf("Calling %s...", funcName)
}

func (app *Application) handleToolCalls(toolCalls []openai.ToolCall, opts CLIOptions) {
	for _, toolCall := range toolCalls {
		if toolCall.Type != "function" || toolCall.Function.Name == "" {
			continue
		}

		// Check if it's an MCP tool (starts with "mcp_")
		// FIXME: This might not be reliable, the user might define a
		// function that starts with mcp_
		if strings.HasPrefix(toolCall.Function.Name, "mcp_") {
			app.handleMCPToolCall(toolCall, opts)
			continue
		}

		// Handle regular function
		var matchedFunc FunctionConfig
		for _, fc := range app.agent.Functions {
			if fc.Name == toolCall.Function.Name {
				matchedFunc = fc
				break
			}
		}

		if matchedFunc.Name == "" {
			log.Fatalf("No matching function found for: %s", toolCall.Function.Name)
		}

		if app.showProgress && len(matchedFunc.Output) == 0 {
			if summary := app.generateProgressSummary(matchedFunc.Name, toolCall.Function.Arguments); summary != "" {
				// Clear previous line if exists
				if app.lastProgressLen > 0 {
					fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", app.lastProgressLen))
				}
				msg := fmt.Sprintf("⋮ %s", summary)
				color.New(color.FgBlue).Fprint(os.Stderr, msg)
				app.lastProgressLen = len(msg)
			}
		}

		// Set the provider and model env so that nested esa calls
		// make use of it. Users can override this by setting the
		// value explicitly in the nested esa calls.
		provider, model, _ := app.parseModel()
		os.Setenv("ESA_MODEL", fmt.Sprintf("%s/%s", provider, model))

		approved, command, stdin, result, err := executeFunction(
			app.getEffectiveAskLevel(),
			matchedFunc,
			toolCall.Function.Arguments,
		)
		app.debugPrint("Function Execution",
			fmt.Sprintf("Function: %s", matchedFunc.Name),
			fmt.Sprintf("Approved: %s", fmt.Sprint(approved)),
			fmt.Sprintf("Command: %s", command),
			fmt.Sprintf("Stdin: %s", stdin),
			fmt.Sprintf("Output: %s", result))

		if err != nil {
			app.debugPrint("Function Error", err)
			// Clear progress line before showing error
			if app.showProgress && app.lastProgressLen > 0 {
				fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", app.lastProgressLen))
				app.lastProgressLen = 0
			}

			app.messages = append(app.messages, openai.ChatCompletionMessage{
				Role:       "tool",
				Name:       toolCall.Function.Name,
				Content:    fmt.Sprintf("Error: %v", err),
				ToolCallID: toolCall.ID,
			})
			continue
		}

		content := fmt.Sprintf("Command: %s\n\nOutput: \n%s", command, result)

		// Display command when --show-commands is enabled
		if app.showCommands || app.showToolCalls {
			color.New(toolCallCommandColor).Fprintf(os.Stderr, "$ %s\n", command)
		}

		// Display tool call output when --show-tool-calls is enabled
		if app.showToolCalls {
			color.New(toolCallOutputColor).Fprintf(os.Stderr, "%s\n", result)
		}

		app.messages = append(app.messages, openai.ChatCompletionMessage{
			Role:       "tool",
			Name:       toolCall.Function.Name,
			Content:    content,
			ToolCallID: toolCall.ID,
		})
	}
}

// handleMCPToolCall handles tool calls for MCP servers
func (app *Application) handleMCPToolCall(toolCall openai.ToolCall, opts CLIOptions) {
	if app.showProgress {
		if summary := app.generateProgressSummary(toolCall.Function.Name, toolCall.Function.Arguments); summary != "" {
			// Clear previous line if exists
			if app.lastProgressLen > 0 {
				fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", app.lastProgressLen))
			}
			msg := fmt.Sprintf("⋮ %s", summary)
			color.New(color.FgBlue).Fprint(os.Stderr, msg)
			app.lastProgressLen = len(msg)
		}
	}

	// Parse the arguments
	var arguments any
	if toolCall.Function.Arguments != "" {
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &arguments); err != nil {
			app.debugPrint("MCP Tool Error", fmt.Sprintf("Failed to parse arguments: %v", err))
			// Clear progress line before showing error
			if app.showProgress && app.lastProgressLen > 0 {
				fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", app.lastProgressLen))
				app.lastProgressLen = 0
			}

			app.messages = append(app.messages, openai.ChatCompletionMessage{
				Role:       "tool",
				Name:       toolCall.Function.Name,
				Content:    fmt.Sprintf("Error: Failed to parse arguments: %v", err),
				ToolCallID: toolCall.ID,
			})
			return
		}
	}

	// Call the MCP tool with ask level
	result, err := app.mcpManager.CallTool(toolCall.Function.Name, arguments, app.getEffectiveAskLevel())

	app.debugPrint("MCP Tool Execution",
		fmt.Sprintf("Tool: %s", toolCall.Function.Name),
		fmt.Sprintf("Arguments: %s", toolCall.Function.Arguments),
		fmt.Sprintf("Output: %s", result))

	if err != nil {
		app.debugPrint("MCP Tool Error", err)
		// Clear progress line before showing error
		if app.showProgress && app.lastProgressLen > 0 {
			fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", app.lastProgressLen))
			app.lastProgressLen = 0
		}

		app.messages = append(app.messages, openai.ChatCompletionMessage{
			Role:       "tool",
			Name:       toolCall.Function.Name,
			Content:    fmt.Sprintf("Error: %v", err),
			ToolCallID: toolCall.ID,
		})
		return
	}

	// Format arguments for display
	var argsDisplay string
	if arguments != nil {
		if argsJSON, err := json.Marshal(arguments); err == nil {
			argsDisplay = string(argsJSON)
		} else {
			argsDisplay = fmt.Sprintf("%v", arguments)
		}
	} else {
		argsDisplay = "{}"
	}

	// Display command when --show-commands is enabled
	if app.showCommands || app.showToolCalls {
		color.New(toolCallCommandColor).Fprintf(os.Stderr, "# %s(%s)\n", toolCall.Function.Name, argsDisplay)
	}

	// Display MCP tool call with output when --show-tool-calls is enabled
	if app.showToolCalls {
		color.New(toolCallOutputColor).Fprintf(os.Stderr, "%s\n", result)
	}

	app.messages = append(app.messages, openai.ChatCompletionMessage{
		Role:       "tool",
		Name:       toolCall.Function.Name,
		Content:    result,
		ToolCallID: toolCall.ID,
	})
}

// shouldThink resolves whether thinking is enabled for this request.
// Priority: CLI flag > agent config > default (true, let model decide).
func (app *Application) shouldThink() bool {
	if app.thinkEnabled != nil {
		return *app.thinkEnabled
	}
	if app.agent.Think != nil {
		return *app.agent.Think
	}
	return true // default: let model think
}

func (app *Application) getSystemPrompt() (string, error) {
	var prompt string
	var err error
	if app.agent.SystemPrompt != "" {
		prompt, err = app.processSystemPrompt(app.agent.SystemPrompt)
	} else {
		prompt, err = app.processSystemPrompt(systemPrompt)
	}
	if err != nil {
		return "", err
	}

	if !app.shouldThink() {
		prompt += "\n/no_think"
	}

	return prompt, nil
}

func (app *Application) processSystemPrompt(prompt string) (string, error) {
	return processShellBlocks(prompt)
}

func (app *Application) compactMessagesIfNeeded() error {
	if !app.compactPrompt {
		return nil
	}

	if len(app.messages) <= 1 {
		return nil
	}

	if len(app.messages) <= app.compactMaxMsgs && messagesSize(app.messages) <= app.compactMaxChars {
		return nil
	}

	systemMsg, toSummarize, tail, existingSummary, ok := splitMessagesForCompaction(app.messages, app.compactKeepLast)
	if !ok || len(toSummarize) == 0 {
		return nil
	}

	summaryInput := buildCompactionInput(existingSummary, toSummarize)
	summary, err := app.summarizeConversation(summaryInput)
	if err != nil {
		return err
	}

	summaryMsg := openai.ChatCompletionMessage{
		Role:    "system",
		Content: compactionSummaryPrefix + strings.TrimSpace(summary),
	}

	app.messages = append([]openai.ChatCompletionMessage{systemMsg, summaryMsg}, tail...)
	app.debugPrint("Compaction", fmt.Sprintf("Compacted %d messages into summary", len(toSummarize)))
	return nil
}

func (app *Application) summarizeConversation(input string) (string, error) {
	ctx := context.Background()
	system := "Summarize the conversation for future context. Preserve decisions, constraints, file paths, commands, names, and open tasks. Be concise and factual."
	req := openai.ChatCompletionRequest{
		Model: app.getModel(),
		Messages: []openai.ChatCompletionMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: input},
		},
		Temperature: 0.2,
	}

	resp, err := app.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("empty summary response")
	}
	return resp.Choices[0].Message.Content, nil
}

func splitMessagesForCompaction(messages []openai.ChatCompletionMessage, keepLast int) (system openai.ChatCompletionMessage, toSummarize []openai.ChatCompletionMessage, tail []openai.ChatCompletionMessage, existingSummary string, ok bool) {
	if len(messages) == 0 {
		return system, nil, nil, "", false
	}
	if messages[0].Role != "system" {
		return system, nil, nil, "", false
	}

	system = messages[0]

	var rest []openai.ChatCompletionMessage
	for i := 1; i < len(messages); i++ {
		if isCompactionSummaryMessage(messages[i]) {
			existingSummary = strings.TrimSpace(strings.TrimPrefix(messages[i].Content, compactionSummaryPrefix))
			continue
		}
		rest = append(rest, messages[i])
	}

	if len(rest) <= keepLast {
		return system, nil, rest, existingSummary, true
	}

	cut := len(rest) - keepLast
	toSummarize = rest[:cut]
	tail = rest[cut:]
	return system, toSummarize, tail, existingSummary, true
}

func isCompactionSummaryMessage(msg openai.ChatCompletionMessage) bool {
	return msg.Role == "system" && strings.HasPrefix(msg.Content, compactionSummaryPrefix)
}

func buildCompactionInput(existingSummary string, messages []openai.ChatCompletionMessage) string {
	var b strings.Builder
	if existingSummary != "" {
		b.WriteString("Previous summary:\n")
		b.WriteString(existingSummary)
		b.WriteString("\n\n")
	}
	b.WriteString("Conversation to summarize:\n")
	for _, msg := range messages {
		line := formatMessageForSummary(msg)
		if line == "" {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func formatMessageForSummary(msg openai.ChatCompletionMessage) string {
	role := strings.ToUpper(msg.Role)

	var b strings.Builder
	b.WriteString("[")
	b.WriteString(role)
	b.WriteString("] ")

	if msg.Name != "" {
		b.WriteString(msg.Name)
		b.WriteString(": ")
	}

	if msg.Content != "" {
		b.WriteString(msg.Content)
	}

	if msg.FunctionCall != nil {
		b.WriteString(" FunctionCall: ")
		b.WriteString(msg.FunctionCall.Name)
		if msg.FunctionCall.Arguments != "" {
			b.WriteString(" ")
			b.WriteString(msg.FunctionCall.Arguments)
		}
	}

	if len(msg.ToolCalls) > 0 {
		if msg.Content != "" {
			b.WriteString(" ")
		}
		b.WriteString("ToolCalls: ")
		for i, call := range msg.ToolCalls {
			if i > 0 {
				b.WriteString("; ")
			}
			b.WriteString(call.Function.Name)
			if call.Function.Arguments != "" {
				b.WriteString(" ")
				b.WriteString(call.Function.Arguments)
			}
		}
	}

	if msg.Role == "tool" && msg.ToolCallID != "" {
		if msg.Content != "" {
			b.WriteString(" ")
		}
		b.WriteString("(tool_call_id=")
		b.WriteString(msg.ToolCallID)
		b.WriteString(")")
	}

	return strings.TrimSpace(b.String())
}

func messagesSize(messages []openai.ChatCompletionMessage) int {
	total := 0
	for _, msg := range messages {
		total += messageSize(msg)
	}
	return total
}

func messageSize(msg openai.ChatCompletionMessage) int {
	size := len(msg.Role) + len(msg.Content) + len(msg.Name) + len(msg.ToolCallID)
	if msg.FunctionCall != nil {
		size += len(msg.FunctionCall.Name) + len(msg.FunctionCall.Arguments)
	}
	for _, call := range msg.ToolCalls {
		size += len(call.Function.Name) + len(call.Function.Arguments)
		size += len(call.ID) + len(call.Type)
	}
	return size
}

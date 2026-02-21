package conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/meain/esa/internal/agent"
	"github.com/meain/esa/internal/config"
	"github.com/meain/esa/internal/mcp"
	"github.com/meain/esa/internal/options"
	"github.com/meain/esa/internal/token"
	"github.com/meain/esa/internal/tools"
	"github.com/meain/esa/internal/utils"
	"github.com/sashabaranov/go-openai"
)

const (
	defaultModel         = "openai/gpt-5.2-2025-12-11"
	toolCallCommandColor = color.FgCyan
	toolCallOutputColor  = color.FgWhite
	maxRetryCount        = 5
	baseRetryDelay       = 1 * time.Second
	maxRetryDelay        = 1 * time.Minute
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
	agent           agent.Agent
	agentPath       string
	client          *openai.Client
	clients         map[string]*openai.Client
	debug           bool
	historyFile     string
	messages        []openai.ChatCompletionMessage
	messageMeta     []HistoryMessageMeta
	usage           token.Usage // accumulated token counts across LLM calls
	debugPrint      func(section string, v ...any)
	showCommands    bool
	showToolCalls   bool
	showProgress    bool
	lastProgressLen int
	modelFlag       string
	config          *config.Config
	mcpManager      *mcp.MCPManager
	cliAskLevel     string
	prettyOutput    bool
	thinkEnabled    *bool // nil = use agent default, true/false = CLI override
	compactPrompt   bool
	compactMaxMsgs  int
	compactKeepLast int
	compactMaxChars int
	lastModelUsed   string
}

// ProviderInfo contains provider-specific configuration.
type ProviderInfo struct {
	BaseURL           string
	APIKeyEnvar       string
	APIKeyCanBeEmpty  bool
	AdditionalHeaders map[string]string
}

// parseModel parses model string in format "provider/model" and
// returns provider, model name, base URL and API key environment
// variable
func (app *Application) parseModel() (provider string, model string, info ProviderInfo) {
	modelStr := app.resolveModelString("chat", "")
	return parseModel(modelStr, app.agent, app.config)
}

func (app *Application) ParseModel() (provider string, model string, info ProviderInfo) {
	return app.parseModel()
}

func (app *Application) resolveModelString(purpose string, toolName string) string {
	if app.config != nil {
		ms := app.config.ModelStrategy
		switch purpose {
		case "summarize":
			if ms.Summarize != "" {
				return ms.Summarize
			}
		case "tool":
			if toolName != "" && ms.Tool != nil {
				if val, ok := ms.Tool[toolName]; ok {
					return val
				}
			}
			if ms.ToolDefault != "" {
				return ms.ToolDefault
			}
		default:
			if ms.Chat != "" {
				return ms.Chat
			}
		}
	}

	if app.modelFlag != "" {
		return app.modelFlag
	}
	if app.agent.DefaultModel != "" {
		return app.agent.DefaultModel
	}
	if app.config != nil && app.config.Settings.DefaultModel != "" {
		return app.config.Settings.DefaultModel
	}
	return defaultModel
}

func (app *Application) clientForModel(modelStr string) (*openai.Client, error) {
	if modelStr == "" {
		modelStr = app.resolveModelString("chat", "")
	}
	if app.clients == nil {
		app.clients = make(map[string]*openai.Client)
	}
	if client, ok := app.clients[modelStr]; ok {
		return client, nil
	}

	client, err := setupOpenAIClient(modelStr, app.agent, app.config)
	if err != nil {
		return nil, err
	}
	app.clients[modelStr] = client
	return client, nil
}

func (app *Application) GetEffectiveAskLevel() string {
	return app.getEffectiveAskLevel()
}

func (app *Application) DebugEnabled() bool {
	return app.debug
}

func (app *Application) DebugPrint(section string, v ...any) {
	if app.debugPrint == nil {
		return
	}
	app.debugPrint(section, v...)
}

func (app *Application) Agent() agent.Agent {
	return app.agent
}

func (app *Application) AgentPath() string {
	return app.agentPath
}

func (app *Application) AddMessage(role, content string) {
	app.messages = append(app.messages, openai.ChatCompletionMessage{
		Role:    role,
		Content: content,
	})
	if app.lastModelUsed != "" {
		app.ensureMessageMeta(app.lastModelUsed)
	}
}

func newMessageID() string {
	if id, err := uuid.NewV7(); err == nil {
		return id.String()
	}
	return uuid.New().String()
}

func (app *Application) Messages() []openai.ChatCompletionMessage {
	return app.messages
}

func (app *Application) RunConversationLoop(opts options.CLIOptions) {
	app.runConversationLoop(opts)
}

func (app *Application) GetSystemPrompt() (string, error) {
	return app.getSystemPrompt()
}

func (app *Application) EnsureSystemMessage(prompt string) {
	if app.messages == nil {
		app.messages = []openai.ChatCompletionMessage{{
			Role:    "system",
			Content: prompt,
		}}
	}
}

func (app *Application) StartMCPServers(ctx context.Context) error {
	if len(app.agent.MCPServers) == 0 {
		return nil
	}
	if err := app.mcpManager.StartServers(ctx, app.agent.MCPServers); err != nil {
		return err
	}
	return nil
}

func (app *Application) StopMCPServers() {
	app.mcpManager.StopAllServers()
}

func (app *Application) SetModel(modelStr string) error {
	app.modelFlag = modelStr
	client, err := setupOpenAIClient(modelStr, app.agent, app.config)
	if err != nil {
		return err
	}
	app.client = client
	return nil
}

func (app *Application) SetAgent(agentCfg agent.Agent, agentPath string) {
	app.agent = agentCfg
	app.agentPath = agentPath
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

	modelStr := app.resolveModelString("chat", "")
	app.lastModelUsed = modelStr
	client, err := app.clientForModel(modelStr)
	if err != nil {
		return nil, err
	}

	// Retry logic for rate limiting
	for attempt := 0; attempt <= maxRetryCount; attempt++ {
		stream, err = client.CreateChatCompletionStream(
			context.Background(),
			openai.ChatCompletionRequest{
				Model:         modelStr,
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

func NewApplication(opts *options.CLIOptions) (*Application, error) {
	// Load global config first
	config, err := config.LoadConfig(opts.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errFailedToLoadConfig, err)
	}

	cacheDir, err := utils.SetupCacheDir()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errFailedToSetupCache, err)
	}

	var (
		messages []openai.ChatCompletionMessage
		usage    token.Usage
	)

	// If conversation index is set without retry, also set continue chat
	if len(opts.Conversation) > 0 && !opts.RetryChat {
		if _, err := utils.FindHistoryFile(cacheDir, opts.Conversation); err == nil {
			opts.ContinueChat = true
		}
	}

	if opts.ContinueChat || opts.RetryChat {
		if opts.Conversation == "" {
			opts.Conversation = "1"
		}
	}

	historyFile, hasHistory := utils.GetHistoryFilePath(cacheDir, opts)
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
		messageMeta := history.MessageMeta

		// Carry forward token counts from prior turns in this conversation
		if history.Usage != nil {
			usage = *history.Usage
		}

		app := &Application{
			debug:       opts.DebugMode,
			messageMeta: messageMeta,
		}
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
		opts.AgentPath = agent.DefaultAgentPath()
	}

	if strings.HasPrefix(opts.AgentPath, "builtin:") {
		opts.AgentName = strings.TrimPrefix(opts.AgentPath, "builtin:")
	}

	agentCfg, err := agent.LoadConfiguration(opts)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errFailedToLoadAgent, err)
	}

	// If SystemPrompt is set in CLI options, override agent's SystemPrompt
	if opts.SystemPrompt != "" {
		agentCfg.SystemPrompt = opts.SystemPrompt
	}

	client, err := setupOpenAIClient(opts.Model, agentCfg, config)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errFailedToSetupClient, err)
	}

	showCommands := opts.ShowCommands || config.Settings.ShowCommands
	showToolCalls := opts.ShowToolCalls || config.Settings.ShowToolCalls
	compactPrompt, compactMaxMsgs, compactKeepLast, compactMaxChars := normalizeCompactionSettings(config.Settings)
	if opts.Compaction {
		compactPrompt = true
	}
	if opts.NoCompaction {
		compactPrompt = false
	}

	// Initialize MCP manager
	mcpManager := mcp.NewMCPManager()

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
		agent:           agentCfg,
		agentPath:       opts.AgentPath,
		client:          client,
		clients:         map[string]*openai.Client{opts.Model: client},
		historyFile:     historyFile,
		messages:        messages,
		messageMeta:     nil,
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
		fmt.Sprintf("Base URL: %q", info.BaseURL),
		fmt.Sprintf("API key envar: %q", info.APIKeyEnvar),
		fmt.Sprintf("Agent path: %q", opts.AgentPath),
		fmt.Sprintf("History file: %q", historyFile),
		fmt.Sprintf("Debug mode: %v", app.debug),
		fmt.Sprintf("Show commands: %v", app.showCommands),
		fmt.Sprintf("Show tool calls: %v", app.showToolCalls),
		fmt.Sprintf("Show progress: %v", app.showProgress),
	)

	return app, nil
}

func (app *Application) Run(opts options.CLIOptions) {
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
	provider, model, _ := app.parseModel()
	app.ensureMessageMeta(fmt.Sprintf("%s/%s", provider, model))

	// Debug prints before starting communication
	app.debugPrint("System Message", app.messages[0].Content)

	input := utils.ReadStdin()
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
		if app.lastModelUsed != "" {
			app.ensureMessageMeta(app.lastModelUsed)
		}
	}

	if len(commandStr) > 0 {
		app.messages = append(app.messages, openai.ChatCompletionMessage{
			Role:    "user",
			Content: commandStr,
		})
		if app.lastModelUsed != "" {
			app.ensureMessageMeta(app.lastModelUsed)
		}
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
		if app.lastModelUsed != "" {
			app.ensureMessageMeta(app.lastModelUsed)
		}
	}
}

func (app *Application) processInitialMessage(message string) (string, error) {
	// Use the same processing logic as system prompt
	return app.processSystemPrompt(message)
}

func (app *Application) runConversationLoop(opts options.CLIOptions) {
	openAITools := tools.ConvertFunctionsToTools(app.agent.Functions)

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
		if app.lastModelUsed != "" {
			app.ensureMessageMeta(app.lastModelUsed)
		}

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
	modelStr := app.resolveModelString("chat", "")
	_, model, _ := parseModel(modelStr, app.agent, app.config)
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

type ConversationHistory struct {
	AgentPath   string                         `json:"agent_path"`
	Model       string                         `json:"model"`
	Messages    []openai.ChatCompletionMessage `json:"messages"`
	MessageMeta []HistoryMessageMeta           `json:"message_meta,omitempty"`
	Usage       *token.Usage                   `json:"usage,omitempty"` // nil in history files from before token tracking
}

type HistoryMessageMeta struct {
	ID    string `json:"id"`
	Model string `json:"model,omitempty"`
	Role  string `json:"role,omitempty"`
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

	messageMeta := app.ensureMessageMeta(modelString)

	history := ConversationHistory{
		AgentPath:   app.agentPath,
		Model:       modelString,
		Messages:    app.messages,
		MessageMeta: messageMeta,
		Usage:       usagePtr,
	}

	if data, err := json.Marshal(history); err == nil {
		if err := os.WriteFile(app.historyFile, data, 0644); err != nil {
			app.debugPrint("Error", fmt.Sprintf("Failed to save history: %v", err))
		}
	}
}

func (app *Application) ensureMessageMeta(modelString string) []HistoryMessageMeta {
	if app.messageMeta == nil {
		app.messageMeta = make([]HistoryMessageMeta, 0, len(app.messages))
	}

	// Trim excess if messages were truncated (e.g., retry mode)
	if len(app.messageMeta) > len(app.messages) {
		app.messageMeta = app.messageMeta[:len(app.messages)]
	}

	for len(app.messageMeta) < len(app.messages) {
		msg := app.messages[len(app.messageMeta)]
		app.messageMeta = append(app.messageMeta, HistoryMessageMeta{
			ID:    newMessageID(),
			Model: modelString,
			Role:  msg.Role,
		})
	}

	// Fill missing model/role on existing entries
	for i := range app.messageMeta {
		if app.messageMeta[i].Model == "" {
			app.messageMeta[i].Model = modelString
		}
		if app.messageMeta[i].Role == "" && i < len(app.messages) {
			app.messageMeta[i].Role = app.messages[i].Role
		}
	}

	return app.messageMeta
}

func (app *Application) generateProgressSummary(funcName string, args string) string {
	return fmt.Sprintf("Calling %s...", funcName)
}

func (app *Application) handleToolCalls(toolCalls []openai.ToolCall, opts options.CLIOptions) {
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
		var matchedFunc agent.FunctionConfig
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

		approved, command, stdin, result, err := tools.ExecuteFunction(
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
func (app *Application) handleMCPToolCall(toolCall openai.ToolCall, opts options.CLIOptions) {
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
		prompt, err = app.processSystemPrompt(agent.SystemPrompt)
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
	return utils.ProcessShellBlocks(prompt)
}

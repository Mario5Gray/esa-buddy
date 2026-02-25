package conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	retry "github.com/avast/retry-go/v4"
	"github.com/google/uuid"
	"github.com/meain/esa/internal/agent"
	"github.com/meain/esa/internal/buildinfo"
	"github.com/meain/esa/internal/config"
	"github.com/meain/esa/internal/conversation/history"
	"github.com/meain/esa/internal/conversation/message"
	convtools "github.com/meain/esa/internal/conversation/tools"
	"github.com/meain/esa/internal/executor"
	"github.com/meain/esa/internal/llm"
	"github.com/meain/esa/internal/logging"
	"github.com/meain/esa/internal/mcp"
	"github.com/meain/esa/internal/options"
	"github.com/meain/esa/internal/redaction"
	"github.com/meain/esa/internal/security"
	"github.com/meain/esa/internal/telemetry"
	"github.com/meain/esa/internal/token"
	"github.com/meain/esa/internal/tokenizer"
	"github.com/meain/esa/internal/tools"
	"github.com/meain/esa/internal/utils"
	"github.com/sashabaranov/go-openai"
	"log/slog"
)

const (
	defaultRetryMaxAttempts = 6
	defaultRetryBaseDelay   = 1 * time.Second
	defaultRetryMaxDelay    = 1 * time.Minute
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
	agent                        agent.Agent
	agentPath                    string
	client                       llm.Client
	clients                      map[string]llm.Client
	debug                        bool
	historyFile                  string
	messages                     []openai.ChatCompletionMessage
	messageMeta                  []history.HistoryMessageMeta
	usage                        token.Usage // accumulated token counts across LLM calls
	logger                       *slog.Logger
	telemetry                    telemetry.Telemetry
	debugPrint                   func(section string, v ...any)
	showCommands                 bool
	showToolCalls                bool
	showProgress                 bool
	lastProgressLen              int
	modelFlag                    string
	config                       *config.Config
	mcpManager                   *mcp.MCPManager
	cliAskLevel                  string
	prettyOutput                 bool
	thinkEnabled                 *bool // nil = use agent default, true/false = CLI override
	compactPrompt                bool
	compactMaxMsgs               int
	compactKeepLast              int
	compactMaxChars              int
	compactionTokenThresholdPct  int
	compactionRedactionPolicy    string
	compactionRedactor           redaction.Policy
	compactionSummary            string
	toolSearchEnabled            bool
	toolSearchLimit              int
	toolSearchSelection          map[string]struct{}
	toolSearchIndex              *tools.SearchIndex
	retryMaxAttempts             uint
	retryBaseDelay               time.Duration
	retryMaxDelay                time.Duration
	lastModelUsed                string
	lastCompactionTrigger        string
	lastCompactionMsgCount       int
	lastCompactionCharCount      int
	lastCompactionTokenEstimate  int
	lastCompactionTokenThreshold int
	counterProvider              tokenizer.CounterProvider
	toolGate                     security.GateChain
	execTooler                   executor.Executor
	// transforms is the ordered pipeline of message.Transform functions applied
	// by ingest() before any message enters app.messages. This is the Reference
	// Monitor enforcement point — every write to the LLM context passes here.
	transforms []message.Transform
	out        io.Writer // receives streamed content; defaults to os.Stdout
}

// parseModel parses model string in format "provider/model" and
// returns provider, model name, base URL and API key environment
// variable
func (app *Application) parseModel() (provider string, model string, info llm.ProviderInfo) {
	modelStr := app.resolveModelString("chat", "")
	return llm.ParseModel(modelStr, app.agent, app.config)
}

func (app *Application) ParseModel() (provider string, model string, info llm.ProviderInfo) {
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
	return llm.DefaultModel
}

func (app *Application) clientForModel(modelStr string) (llm.Client, error) {
	if modelStr == "" {
		modelStr = app.resolveModelString("chat", "")
	}
	if app.clients == nil {
		app.clients = make(map[string]llm.Client)
	}
	if client, ok := app.clients[modelStr]; ok {
		return client, nil
	}

	client, err := llm.SetupOpenAIClient(modelStr, app.agent, app.config)
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

// ingest is the single choke point for all writes to app.messages.
// It runs msg through the registered transform pipeline (Reference Monitor,
// Anderson 1972) before appending. No message reaches the LLM context without
// passing here. System messages bypass the pipeline — they are trusted,
// author-controlled, and must not be modified by data-layer transforms.
// ingest is the single choke point for all writes to app.messages.
// It runs msg through the registered transform pipeline (Reference Monitor,
// Anderson 1972) before appending. Each transform is responsible for
// declaring its own role policy via OnlyFor or SkipFor — ingest itself
// is role-agnostic. No message reaches the LLM context without passing here.
func (app *Application) ingest(msg openai.ChatCompletionMessage) {
	for _, t := range app.transforms {
		msg = t(msg)
	}
	app.messages = append(app.messages, msg)
	if app.lastModelUsed != "" {
		app.ensureMessageMeta(app.lastModelUsed)
	}
}

// AddMessage appends a user or assistant message to the conversation context
// through the ingest pipeline.
func (app *Application) AddMessage(role, content string) {
	app.ingest(message.New(role, content).Build())
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

func (app *Application) RunConversationLoop(ctx context.Context, opts options.CLIOptions) error {
	return app.runConversationLoop(ctx, opts)
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

func (app *Application) SetToolGate(chain security.GateChain) {
	app.toolGate = chain
}

func (app *Application) SetToolExecutor(exec executor.Executor) {
	app.execTooler = exec
}

func (app *Application) SetModel(modelStr string) error {
	app.modelFlag = modelStr
	client, err := llm.SetupOpenAIClient(modelStr, app.agent, app.config)
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

// ClearMessages resets the conversation to the system message only.
func (app *Application) ClearMessages() {
	if len(app.messages) == 0 {
		return
	}
	app.messages = app.messages[:1]
}

// UndoLastExchange removes the last user+assistant message pair.
// Returns true if a pair was removed, false if there was nothing to undo.
func (app *Application) UndoLastExchange() bool {
	// Find last assistant message
	assistantIdx := -1
	for i := len(app.messages) - 1; i >= 0; i-- {
		if app.messages[i].Role == "assistant" {
			assistantIdx = i
			break
		}
	}
	if assistantIdx < 0 {
		return false
	}
	// Find the user message immediately before it
	userIdx := -1
	for i := assistantIdx - 1; i >= 0; i-- {
		if app.messages[i].Role == "user" {
			userIdx = i
			break
		}
	}
	if userIdx < 0 {
		return false
	}
	// Remove both messages
	app.messages = append(app.messages[:userIdx], app.messages[assistantIdx+1:]...)
	return true
}

// Usage returns a snapshot of accumulated token counts.
func (app *Application) Usage() token.Usage {
	return app.usage
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

func normalizeRetrySettings(settings config.Settings) (uint, time.Duration, time.Duration) {
	maxAttempts := settings.RetryMaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = defaultRetryMaxAttempts
	}

	baseDelayMs := settings.RetryBaseDelayMs
	if baseDelayMs <= 0 {
		baseDelayMs = int(defaultRetryBaseDelay / time.Millisecond)
	}
	maxDelayMs := settings.RetryMaxDelayMs
	if maxDelayMs <= 0 {
		maxDelayMs = int(defaultRetryMaxDelay / time.Millisecond)
	}
	if maxDelayMs < baseDelayMs {
		maxDelayMs = baseDelayMs
	}

	return uint(maxAttempts), time.Duration(baseDelayMs) * time.Millisecond, time.Duration(maxDelayMs) * time.Millisecond
}

// createChatCompletionWithRetry creates a chat completion stream with retry logic for rate limiting
func (app *Application) createChatCompletionWithRetry(ctx context.Context, tools []openai.Tool) (llm.Stream, error) {
	var stream llm.Stream

	if err := app.compactMessagesIfNeeded(); err != nil {
		app.debugPrint("Compaction", fmt.Sprintf("Compaction skipped: %v", err))
	}

	modelStr := app.resolveModelString("chat", "")
	app.lastModelUsed = modelStr
	client, err := app.clientForModel(modelStr)
	if err != nil {
		return nil, err
	}

	messages := buildRequestMessages(app.messages, app.compactionSummary)
	req := openai.ChatCompletionRequest{
		Model:         modelStr,
		Messages:      messages,
		Tools:         tools,
		StreamOptions: &openai.StreamOptions{IncludeUsage: true},
	}

	err = retry.Do(
		func() error {
			var callErr error
			stream, callErr = client.CreateChatCompletionStream(ctx, req)
			if callErr == nil {
				return nil
			}
			if isRateLimitError(callErr) {
				return callErr
			}
			return retry.Unrecoverable(callErr)
		},
		retry.Attempts(app.retryMaxAttempts),
		retry.Delay(app.retryBaseDelay),
		retry.DelayType(retry.BackOffDelay),
		retry.MaxDelay(app.retryMaxDelay),
		retry.LastErrorOnly(true),
		retry.OnRetry(func(n uint, err error) {
			app.debugPrint("Rate Limit",
				fmt.Sprintf("Rate limit hit, retrying (attempt %d/%d): %v", n+1, app.retryMaxAttempts, err))
			if app.telemetry != nil {
				app.telemetry.Retry(telemetry.RetryContext{
					Attempt: int(n + 1),
					Max:     int(app.retryMaxAttempts),
					Error:   fmt.Sprintf("%v", err),
					Delay:   app.retryBaseDelay,
				})
			}
		}),
	)
	if err != nil {
		return nil, err
	}
	return stream, nil
}

func buildRequestMessages(messages []openai.ChatCompletionMessage, compactionSummary string) []openai.ChatCompletionMessage {
	trimmedSummary := strings.TrimSpace(compactionSummary)
	if trimmedSummary == "" {
		return messages
	}
	summaryMsg := openai.ChatCompletionMessage{
		Role:    "system",
		Content: trimmedSummary,
	}
	if len(messages) > 0 && messages[0].Role == "system" {
		return append(
			[]openai.ChatCompletionMessage{messages[0], summaryMsg},
			messages[1:]...,
		)
	}
	return append([]openai.ChatCompletionMessage{summaryMsg}, messages...)
}

func NewApplication(opts *options.CLIOptions) (*Application, error) {
	// Load global config first
	config, err := config.LoadConfig(opts.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errFailedToLoadConfig, err)
	}

	logger, _, err := logging.Setup(config.Logging)
	if err != nil {
		return nil, err
	}
	telemetrySink := telemetry.NewSlogAdapter(logger)

	cacheDir, err := utils.SetupCacheDir()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errFailedToSetupCache, err)
	}

	var (
		messages          []openai.ChatCompletionMessage
		usage             token.Usage
		compactionSummary string
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
		var history history.ConversationHistory
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
		if history.Compaction != nil && history.Compaction.Summary != "" {
			compactionSummary = history.Compaction.Summary
		}

		// Carry forward token counts from prior turns in this conversation
		if history.Usage != nil {
			usage = *history.Usage
		}

		app := &Application{
			debug:             opts.DebugMode,
			messageMeta:       messageMeta,
			compactionSummary: compactionSummary,
			logger:            logger,
			telemetry:         telemetrySink,
		}
		app.debugPrint = createDebugPrinter(app.debug, app.logger)

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

	client, err := llm.SetupOpenAIClient(opts.Model, agentCfg, config)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errFailedToSetupClient, err)
	}

	showCommands := opts.ShowCommands || config.Settings.ShowCommands
	showToolCalls := opts.ShowToolCalls || config.Settings.ShowToolCalls
	compactPrompt, compactMaxMsgs, compactKeepLast, compactMaxChars, compactionTokenThresholdPct, redactionConfig, legacyRedactionPolicy := normalizeCompactionSettings(config.Settings)
	retryMaxAttempts, retryBaseDelay, retryMaxDelay := normalizeRetrySettings(config.Settings)
	if opts.Compaction {
		compactPrompt = true
	}
	if opts.NoCompaction {
		compactPrompt = false
	}

	toolSearchEnabled := config.Settings.ToolSearchEnabled
	toolSearchLimit := config.Settings.ToolSearchLimit
	if toolSearchLimit <= 0 {
		toolSearchLimit = 8
	}

	compactionRedactionPolicy := legacyRedactionPolicy
	var compactionRedactor redaction.Policy
	if compactPrompt {
		policy, resolvedName, err := redaction.BuildPolicy(redactionConfig, legacyRedactionPolicy)
		if err != nil {
			return nil, err
		}
		compactionRedactionPolicy = resolvedName
		compactionRedactor = policy
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
		agent:                       agentCfg,
		agentPath:                   opts.AgentPath,
		client:                      client,
		clients:                     map[string]llm.Client{opts.Model: client},
		out:                         os.Stdout,
		historyFile:                 historyFile,
		messages:                    messages,
		messageMeta:                 nil,
		usage:                       usage,
		logger:                      logger,
		telemetry:                   telemetrySink,
		modelFlag:                   opts.Model,
		config:                      config,
		mcpManager:                  mcpManager,
		cliAskLevel:                 opts.AskLevel,
		prettyOutput:                opts.Pretty,
		thinkEnabled:                thinkEnabled,
		compactPrompt:               compactPrompt,
		compactMaxMsgs:              compactMaxMsgs,
		compactKeepLast:             compactKeepLast,
		compactMaxChars:             compactMaxChars,
		compactionTokenThresholdPct: compactionTokenThresholdPct,
		compactionRedactionPolicy:   compactionRedactionPolicy,
		compactionRedactor:          compactionRedactor,
		compactionSummary:           compactionSummary,
		toolSearchEnabled:           toolSearchEnabled,
		toolSearchLimit:             toolSearchLimit,
		toolSearchSelection:         map[string]struct{}{},
		retryMaxAttempts:            retryMaxAttempts,
		retryBaseDelay:              retryBaseDelay,
		retryMaxDelay:               retryMaxDelay,
		counterProvider: func() tokenizer.CounterProvider {
			fallback := tokenizer.FallbackCounter{CharsPerToken: 4}
			provider := tokenizer.NewMapProvider(fallback)
			provider.Set("openai", tokenizer.NewTiktokenCounter())
			return provider
		}(),
		toolGate: security.GateChain{
			Gates: []security.Gate{
				security.HumanGate{},
				security.DenyGate{},
			},
		},
		execTooler: executor.DefaultExecutor{},
		transforms: []message.Transform{
			// Trust boundary: wrap external tool output so the model cannot
			// mistake raw data for instruction. Each transform declares its
			// own role scope via OnlyFor/SkipFor — see message/transforms.go.
			message.Envelope,
		},

		debug:         opts.DebugMode,
		showCommands:  showCommands && !showToolCalls && !opts.DebugMode,
		showToolCalls: showToolCalls && !opts.DebugMode,
		showProgress:  !opts.HideProgress && !opts.DebugMode && !(showCommands || showToolCalls),
	}

	app.debugPrint = createDebugPrinter(app.debug, app.logger)
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

	if err := app.runConversationLoop(context.Background(), opts); err != nil {
		app.telemetryError("conversation_loop", err)
		log.Fatalf("Error: %v", err)
	}
}

func (app *Application) processInput(commandStr, input string) {
	if len(input) > 0 {
		app.ingest(message.New("user", input).Build())
	}

	if len(commandStr) > 0 {
		app.ingest(message.New("user", commandStr).Build())
	}

	// If no input from stdin or command line, use initial message from agent config
	prompt, err := app.processInitialMessage(app.agent.InitialMessage)
	if err != nil {
		log.Fatalf("Error processing initial message: %v", err)
	}

	if len(input) == 0 && len(commandStr) == 0 && app.agent.InitialMessage != "" {
		app.ingest(message.New("user", prompt).Build())
	}
}

func (app *Application) processInitialMessage(message string) (string, error) {
	// Use the same processing logic as system prompt
	return app.processSystemPrompt(message)
}

func (app *Application) runConversationLoop(ctx context.Context, opts options.CLIOptions) error {
	allTools := tools.ConvertFunctionsToTools(app.agent.Functions)

	// Add MCP tools
	mcpTools := app.mcpManager.GetAllTools()
	allTools = append(allTools, mcpTools...)

	app.toolSearchIndex = tools.BuildSearchIndex(app.agent.Functions, mcpTools)

	for {
		app.telemetryTurnStarted()
		openAITools := app.resolveToolSearchTools(allTools)
		stream, err := app.createChatCompletionWithRetry(ctx, openAITools)
		if err != nil {
			app.telemetryError("chat_completion", err)
			return fmt.Errorf("ChatCompletionStream error: %w", err)
		}

		assistantMsg, err := app.handleStreamResponse(stream)
		if err != nil {
			app.telemetryError("stream_response", err)
			return err
		}
		app.ingest(assistantMsg)
		app.telemetryTurnCompleted()

		// Save history after each assistant response
		app.saveConversationHistory()

		if len(assistantMsg.ToolCalls) == 0 {
			break
		}

		app.toolDispatcher().HandleToolCalls(assistantMsg.ToolCalls, opts)

		// Save history after processing tool calls
		app.saveConversationHistory()
	}
	return nil
}

func (app *Application) getModel() string {
	modelStr := app.resolveModelString("chat", "")
	_, model, _ := llm.ParseModel(modelStr, app.agent, app.config)
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
	compaction := &history.CompactionMeta{
		Enabled:         app.compactPrompt,
		MaxMessages:     app.compactMaxMsgs,
		KeepLast:        app.compactKeepLast,
		MaxChars:        app.compactMaxChars,
		RedactionPolicy: app.compactionRedactionPolicy,
		Summary:         app.compactionSummary,
	}
	if app.lastCompactionTrigger != "" {
		compaction.LastTrigger = app.lastCompactionTrigger
		compaction.LastMsgCount = app.lastCompactionMsgCount
		compaction.LastCharCount = app.lastCompactionCharCount
		compaction.LastTokenEstimate = app.lastCompactionTokenEstimate
		compaction.LastTokenThreshold = app.lastCompactionTokenThreshold
	}

	historyData := history.ConversationHistory{
		SchemaVersion: history.SchemaVersionCurrent,
		Commit:        buildinfo.Commit,
		AgentPath:     app.agentPath,
		Model:         modelString,
		Messages:      app.messages,
		MessageMeta:   messageMeta,
		Compaction:    compaction,
		Usage:         usagePtr,
	}

	if data, err := json.Marshal(historyData); err == nil {
		if err := os.WriteFile(app.historyFile, data, 0644); err != nil {
			app.debugPrint("Error", fmt.Sprintf("Failed to save history: %v", err))
		}
	}
}

func (app *Application) ensureMessageMeta(modelString string) []history.HistoryMessageMeta {
	app.messageMeta = history.EnsureMessageMeta(app.messageMeta, app.messages, modelString, newMessageID)
	return app.messageMeta
}

func (app *Application) searchTools(query string, limit int) tools.ToolSearchResult {
	if app.toolSearchIndex == nil {
		return tools.ToolSearchResult{Query: query}
	}
	return app.toolSearchIndex.Search(query, limit)
}

func (app *Application) setToolSearchSelection(names []string) {
	if len(names) == 0 {
		app.toolSearchSelection = map[string]struct{}{}
		return
	}
	selection := make(map[string]struct{}, len(names))
	for _, name := range names {
		if name != "" {
			selection[name] = struct{}{}
		}
	}
	app.toolSearchSelection = selection
}

func (app *Application) resolveToolSearchTools(allTools []openai.Tool) []openai.Tool {
	searchTool := tools.SearchToolDefinition()
	if app.toolSearchEnabled {
		return append(allTools, searchTool)
	}
	if len(app.toolSearchSelection) == 0 {
		return []openai.Tool{searchTool}
	}
	filtered := filterToolsByName(allTools, app.toolSearchSelection)
	return append(filtered, searchTool)
}

func filterToolsByName(allTools []openai.Tool, selection map[string]struct{}) []openai.Tool {
	if len(selection) == 0 {
		return nil
	}
	out := make([]openai.Tool, 0, len(selection))
	for _, tool := range allTools {
		if tool.Function == nil || tool.Function.Name == "" {
			continue
		}
		if _, ok := selection[tool.Function.Name]; ok {
			out = append(out, tool)
		}
	}
	return out
}

func (app *Application) telemetryTurnStarted() {
	if app.telemetry == nil {
		return
	}
	provider, model, _ := app.parseModel()
	app.telemetry.TurnStarted(telemetry.TurnContext{
		TurnIndex:    len(app.messages),
		MessageCount: len(app.messages),
		Provider:     provider,
		Model:        model,
	})
}

func (app *Application) telemetryTurnCompleted() {
	if app.telemetry == nil {
		return
	}
	provider, model, _ := app.parseModel()
	app.telemetry.TurnCompleted(telemetry.TurnContext{
		TurnIndex:    len(app.messages),
		MessageCount: len(app.messages),
		Provider:     provider,
		Model:        model,
	})
}

func (app *Application) telemetryError(stage string, err error) {
	if app.telemetry == nil || err == nil {
		return
	}
	app.telemetry.Error(telemetry.ErrorContext{
		Stage: stage,
		Error: err.Error(),
	})
}

func (app *Application) toolDispatcher() *convtools.Dispatcher {
	return convtools.NewDispatcher(convtools.Deps{
		Agent:                app.agent,
		ShowCommands:         app.showCommands,
		ShowToolCalls:        app.showToolCalls,
		ShowProgress:         app.showProgress,
		LastProgressLen:      &app.lastProgressLen,
		AppendMessage:        app.ingest,
		ToolGate:             app.toolGate,
		ExecTooler:           app.execTooler,
		MCPManager:           app.mcpManager,
		DebugPrint:           app.debugPrint,
		ParseModel:           app.parseModel,
		GetEffectiveAskLevel: app.getEffectiveAskLevel,
		SearchTools:          app.searchTools,
		SetToolSelection:     app.setToolSearchSelection,
		Telemetry:            app.telemetry,
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

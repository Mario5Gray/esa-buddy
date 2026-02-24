package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/chzyer/readline"
	"github.com/fatih/color"
	"github.com/meain/esa/internal/agent"
	"github.com/meain/esa/internal/conversation"
	"github.com/meain/esa/internal/options"
	"github.com/meain/esa/internal/utils"
)

// ReplTheme holds lipgloss styles for REPL output.
type ReplTheme struct {
	Banner lipgloss.Style
	You    lipgloss.Style
	Esa    lipgloss.Style
	Error  lipgloss.Style
	Info   lipgloss.Style
	Badge  lipgloss.Style
}

func newReplTheme() ReplTheme {
	return ReplTheme{
		Banner: lipgloss.NewStyle().Foreground(lipgloss.Color("6")),  // cyan
		You:    lipgloss.NewStyle().Foreground(lipgloss.Color("2")),  // green
		Esa:    lipgloss.NewStyle().Foreground(lipgloss.Color("5")),  // magenta
		Error:  lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true), // red bold
		Info:   lipgloss.NewStyle().Faint(true),
		Badge:  lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Faint(true), // cyan dim
	}
}

// buildPrompt constructs the readline prompt string from app state and theme.
func buildPrompt(app *conversation.Application, theme ReplTheme) string {
	provider, model, _ := app.ParseModel()
	badge := theme.Badge.Render(fmt.Sprintf("[%s/%s]", provider, model))
	you := theme.You.Render("you>")
	return badge + " " + you + " "
}

// joinLines assembles continuation lines into a single string.
// Each line that ends with "\" (after trimming trailing spaces) is joined
// with the next line via "\n". The trailing backslash is stripped.
func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, line := range lines {
		trimmed := strings.TrimRight(line, " ")
		if strings.HasSuffix(trimmed, "\\") {
			// Strip trailing backslash, add newline separator
			sb.WriteString(trimmed[:len(trimmed)-1])
			if i < len(lines)-1 {
				sb.WriteByte('\n')
			}
		} else {
			sb.WriteString(line)
		}
	}
	return sb.String()
}

// joinContinuationLines reads additional lines from rl when firstLine ends with "\".
func joinContinuationLines(rl *readline.Instance, theme ReplTheme, firstLine string) (string, error) {
	lines := []string{firstLine}
	origPrompt := rl.Config.Prompt

	for {
		last := strings.TrimRight(lines[len(lines)-1], " ")
		if !strings.HasSuffix(last, "\\") {
			break
		}
		rl.SetPrompt(theme.Info.Render("... "))
		next, err := rl.Readline()
		rl.SetPrompt(origPrompt)
		if err != nil {
			return joinLines(lines), err
		}
		lines = append(lines, next)
	}
	return joinLines(lines), nil
}

// runReplMode starts the REPL (Read-Eval-Print Loop) mode
func runReplMode(opts *options.CLIOptions, args []string) error {
	// TODO: Make progress work in REPL (will have to newline)
	opts.HideProgress = true // Hide progress in REPL mode

	// Handle agent selection with + prefix in the initial query
	initialQuery := strings.Join(args, " ")
	if strings.HasPrefix(initialQuery, "+") {
		opts.CommandStr = initialQuery
		parseAgentCommand(opts)
		initialQuery = opts.CommandStr
	}

	// Initialize application
	app, err := conversation.NewApplication(opts)
	if err != nil {
		return fmt.Errorf("failed to initialize application: %v", err)
	}

	// Start MCP servers if configured
	if len(app.Agent().MCPServers) > 0 {
		ctx := context.Background()
		if err := app.StartMCPServers(ctx); err != nil {
			return fmt.Errorf("failed to start MCP servers: %v", err)
		}

		defer app.StopMCPServers()
		app.DebugPrint("MCP Servers", fmt.Sprintf("Started %d MCP servers", len(app.Agent().MCPServers)))
	}

	prompt, err := app.GetSystemPrompt()
	if err != nil {
		return fmt.Errorf("error processing system prompt: %v", err)
	}

	app.EnsureSystemMessage(prompt)

	// Debug prints before starting communication
	if msgs := app.Messages(); len(msgs) > 0 {
		app.DebugPrint("System Message", msgs[0].Content)
	}

	theme := newReplTheme()

	// Set up persistent history
	cacheDir, _ := utils.SetupCacheDir()
	historyPath := filepath.Join(cacheDir, "repl_history")

	rlPrompt := buildPrompt(app, theme)
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          rlPrompt,
		HistoryFile:     historyPath,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		return fmt.Errorf("failed to initialize readline: %v", err)
	}
	defer rl.Close()

	fmt.Fprintf(
		os.Stderr,
		"%s %s\n\n",
		theme.Banner.Render("[REPL]"),
		strings.Join([]string{
			"Starting interactive mode",
			"- '/exit' or '/quit' to end the session",
			"- '/help' for available commands",
			"- Use /editor command for multi line input",
		}, "\n"),
	)

	// Handle initial query if provided
	if initialQuery != "" {
		fmt.Fprintf(os.Stderr, "%s %s\n", theme.You.Render("you>"), initialQuery)
		app.AddMessage("user", initialQuery)

		fmt.Fprintf(os.Stderr, "\n%s ", theme.Esa.Render("esa>"))
		turnCtx, cancelTurn := context.WithCancel(context.Background())
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)
		go func() {
			select {
			case <-sigCh:
				cancelTurn()
			case <-turnCtx.Done():
			}
		}()
		err := app.RunConversationLoop(turnCtx, *opts)
		signal.Stop(sigCh)
		cancelTurn()
		if errors.Is(err, context.Canceled) {
			fmt.Fprintln(os.Stderr, theme.Info.Render("^C"))
		} else if err != nil {
			fmt.Fprintf(os.Stderr, "\n%s %v\n", theme.Error.Render("[ERROR]"), err)
		} else {
			u := app.Usage()
			fmt.Fprintln(os.Stderr, theme.Info.Render(fmt.Sprintf(
				"  [tokens: %d prompt / %d completion]",
				u.PromptTokens, u.CompletionTokens)))
		}
	}

	// Main REPL loop
	for {
		rl.SetPrompt(buildPrompt(app, theme))
		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				// Ctrl+C during input — clear line and continue
				continue
			}
			if err == io.EOF {
				fmt.Fprintf(os.Stderr, "\n%s %s\n", theme.Banner.Render("[REPL]"), "Goodbye!")
				break
			}
			return fmt.Errorf("error reading input: %v", err)
		}

		line, err = joinContinuationLines(rl, theme, line)
		if err != nil && err != readline.ErrInterrupt && err != io.EOF {
			return fmt.Errorf("error reading continuation: %v", err)
		}

		input := strings.TrimSpace(line)
		if input == "/exit" || input == "/quit" || input == "" {
			if input == "/exit" || input == "/quit" {
				fmt.Fprintf(os.Stderr, "%s %s\n", theme.Banner.Render("[REPL]"), "Goodbye!")
				break
			}
			continue
		}

		// Handle REPL commands
		if strings.HasPrefix(input, "/") {
			if handleReplCommand(input, app, opts, theme) {
				continue
			}
		}

		fmt.Fprintf(os.Stderr, "%s ", theme.Esa.Render("esa>"))
		app.AddMessage("user", input)

		turnCtx, cancelTurn := context.WithCancel(context.Background())
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)
		go func() {
			select {
			case <-sigCh:
				cancelTurn()
			case <-turnCtx.Done():
			}
		}()
		err = app.RunConversationLoop(turnCtx, *opts)
		signal.Stop(sigCh)
		cancelTurn()

		if errors.Is(err, context.Canceled) {
			fmt.Fprintln(os.Stderr, theme.Info.Render("^C"))
			continue
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, theme.Error.Render("[ERROR] "+err.Error()))
			continue
		}
		u := app.Usage()
		fmt.Fprintln(os.Stderr, theme.Info.Render(fmt.Sprintf(
			"  [tokens: %d prompt / %d completion]",
			u.PromptTokens, u.CompletionTokens)))
	}

	return nil
}

// handleReplCommand handles special REPL commands
// Returns true if the command was handled (and should continue REPL loop)
func handleReplCommand(input string, app *conversation.Application, opts *options.CLIOptions, theme ReplTheme) bool {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return false
	}

	command := parts[0]
	args := parts[1:]

	switch command {
	case "/help":
		return handleHelpCommand(theme)
	case "/config":
		return handleConfigCommand(app, theme)
	case "/model":
		return handleModelCommand(args, app, opts, theme)
	case "/agent":
		return handleAgentCommand(args, app, opts, theme)
	case "/editor":
		return handleEditorCommand(app, opts, theme)
	case "/clear":
		app.ClearMessages()
		fmt.Fprintln(os.Stderr, theme.Info.Render("Conversation cleared."))
		return true
	case "/undo":
		if app.UndoLastExchange() {
			fmt.Fprintln(os.Stderr, theme.Info.Render("Last exchange removed."))
		} else {
			fmt.Fprintln(os.Stderr, theme.Info.Render("Nothing to undo."))
		}
		return true
	case "/tokens":
		u := app.Usage()
		fmt.Fprintln(os.Stderr, theme.Info.Render(fmt.Sprintf(
			"prompt: %d | completion: %d | total: %d",
			u.PromptTokens, u.CompletionTokens, u.PromptTokens+u.CompletionTokens)))
		return true
	default:
		return handleUnknownCommand(command)
	}
}

func handleHelpCommand(theme ReplTheme) bool {
	fmt.Fprintf(os.Stderr, "%s %s\n", theme.Banner.Render("[REPL]"), "Available commands:")
	fmt.Fprintf(os.Stderr, "  %s - Exit the session\n", theme.You.Render("/exit, /quit"))
	fmt.Fprintf(os.Stderr, "  %s - Show this help message\n", theme.You.Render("/help"))
	fmt.Fprintf(os.Stderr, "  %s - Show current configuration\n", theme.You.Render("/config"))
	fmt.Fprintf(os.Stderr, "  %s - Show or set model (e.g., /model openai/gpt-4)\n", theme.You.Render("/model <provider/model>"))
	fmt.Fprintf(os.Stderr, "  %s - Show or set agent (e.g., /agent +k8s, /agent myagent)\n", theme.You.Render("/agent <agent>"))
	fmt.Fprintf(os.Stderr, "  %s - Open the default editor\n", theme.You.Render("/editor"))
	fmt.Fprintf(os.Stderr, "  %s - Clear conversation history\n", theme.You.Render("/clear"))
	fmt.Fprintf(os.Stderr, "  %s - Remove last user+assistant exchange\n", theme.You.Render("/undo"))
	fmt.Fprintf(os.Stderr, "  %s - Show token usage\n", theme.You.Render("/tokens"))
	return true
}

func handleConfigCommand(app *conversation.Application, theme ReplTheme) bool {
	labelStyle := color.New(color.FgHiCyan, color.Bold).SprintFunc()

	fmt.Fprintf(os.Stderr, "%s %s\n", theme.Banner.Render("[REPL]"), "Current configuration:")

	provider, model, info := app.ParseModel()
	askLevel := app.GetEffectiveAskLevel()

	fmt.Fprintf(os.Stderr, "%s %s/%s\n", labelStyle("Current Model:"), provider, model)
	fmt.Fprintf(os.Stderr, "%s %s\n", labelStyle("Base URL:"), info.BaseURL)
	fmt.Fprintf(os.Stderr, "%s %s\n", labelStyle("API Key Env:"), info.APIKeyEnvar)
	fmt.Fprintf(os.Stderr, "%s %s\n", labelStyle("Ask Level:"), askLevel)
	fmt.Fprintf(os.Stderr, "%s %v\n", labelStyle("Debug Mode:"), app.DebugEnabled())

	return true
}

func handleModelCommand(args []string, app *conversation.Application, opts *options.CLIOptions, theme ReplTheme) bool {
	if len(args) == 0 {
		provider, model, _ := app.ParseModel()
		fmt.Fprintf(os.Stderr, "%s %s: %s/%s\n", theme.Banner.Render("[REPL]"), "Current model", provider, model)
		return true
	}

	if err := validateAndSetModel(app, opts, args[0]); err != nil {
		fmt.Fprintln(os.Stderr, theme.Error.Render("[ERROR] "+err.Error()))
		return true
	}

	provider, model, _ := app.ParseModel()
	fmt.Fprintf(os.Stderr, "%s %s: %s/%s\n", theme.Banner.Render("[REPL]"), "Model updated to", provider, model)
	return true
}

func handleAgentCommand(args []string, app *conversation.Application, opts *options.CLIOptions, theme ReplTheme) bool {
	if len(args) == 0 {
		// Show current agent information
		fmt.Fprintf(os.Stderr, "%s %s:\n", theme.Banner.Render("[REPL]"), "Current agent")
		printDetailedAgentInfo(app.Agent(), app.AgentPath())

		return true
	}

	agentStr := args[0]
	if err := validateAndSetAgent(app, opts, agentStr); err != nil {
		fmt.Fprintln(os.Stderr, theme.Error.Render("[ERROR] "+err.Error()))
		return true
	}

	// Show confirmation of the switch
	agentName := app.Agent().Name
	if agentName == "" {
		agentName = agentStr
	}
	fmt.Fprintf(os.Stderr, "%s %s: %s\n", theme.Banner.Render("[REPL]"), "Agent switched to", theme.You.Render(agentName))
	return true
}

// handleEditorCommand handles the /editor command to open the default text editor
func handleEditorCommand(app *conversation.Application, opts *options.CLIOptions, theme ReplTheme) bool {
	// Get editor from environment variable or default to nano
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nano"
	}

	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "esa_prompt_*.txt")
	if err != nil {
		fmt.Fprintln(os.Stderr, theme.Error.Render(fmt.Sprintf("[ERROR] Failed to create temporary file: %v", err)))
		return true
	}
	defer os.Remove(tmpFile.Name()) // Clean up

	// Close the file so the editor can open it
	tmpFile.Close()

	fmt.Fprintf(os.Stderr, "%s Opening editor: %s\n", theme.Banner.Render("[REPL]"), editor)

	// Open the editor
	cmd := exec.Command(editor, tmpFile.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, theme.Error.Render(fmt.Sprintf("[ERROR] Failed to run editor: %v", err)))
		return true
	}

	// Read the content back
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		fmt.Fprintln(os.Stderr, theme.Error.Render(fmt.Sprintf("[ERROR] Failed to read temporary file: %v", err)))
		return true
	}

	// Process the content
	finalContent := strings.TrimSpace(string(content))

	if finalContent == "" {
		fmt.Fprintf(os.Stderr, "%s No content entered, canceling.\n", theme.Banner.Render("[REPL]"))
		return true
	}

	// Add the message and run the conversation
	fmt.Fprintf(os.Stderr, "%s Prompt entered via editor\n", theme.Banner.Render("[REPL]"))
	app.AddMessage("user", finalContent)

	fmt.Fprintf(os.Stderr, "%s %s\n", theme.You.Render("you>"), finalContent)
	fmt.Fprintf(os.Stderr, "%s ", theme.Esa.Render("esa>"))

	turnCtx, cancelTurn := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		select {
		case <-sigCh:
			cancelTurn()
		case <-turnCtx.Done():
		}
	}()
	err = app.RunConversationLoop(turnCtx, *opts)
	signal.Stop(sigCh)
	cancelTurn()

	if errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, theme.Info.Render("^C"))
	} else if err != nil {
		fmt.Fprintln(os.Stderr, theme.Error.Render("[ERROR] "+err.Error()))
	} else {
		u := app.Usage()
		fmt.Fprintln(os.Stderr, theme.Info.Render(fmt.Sprintf(
			"  [tokens: %d prompt / %d completion]",
			u.PromptTokens, u.CompletionTokens)))
	}

	return true
}

func handleUnknownCommand(command string) bool {
	if strings.HasPrefix(command, "/") {
		fmt.Fprintf(os.Stderr, "%s %s '%s'. Type /help for available commands.\n",
			color.New(color.FgRed).Sprint("[ERROR]"), "Unknown command", command)
		return true
	}
	return false
}

// validateAndSetModel validates a model string (including aliases) and sets it if valid
func validateAndSetModel(app *conversation.Application, opts *options.CLIOptions, modelStr string) error {
	opts.Model = modelStr

	if err := app.SetModel(modelStr); err != nil {
		return fmt.Errorf("failed to set model '%s': %v", modelStr, err)
	}
	return nil
}

// validateAndSetAgent validates an agent string and sets it if valid
func validateAndSetAgent(app *conversation.Application, opts *options.CLIOptions, agentStr string) error {
	// Parse the agent string to determine the agent name and path
	agentName, agentPath := agent.ParseAgentString(agentStr)

	// Create a temporary CLIOptions to use with loadConfiguration
	tempOpts := &options.CLIOptions{
		AgentName: agentName,
		AgentPath: agentPath,
	}

	// Load the agent using the existing loadConfiguration function
	agentCfg, err := agent.LoadConfiguration(tempOpts)
	if err != nil {
		return fmt.Errorf("failed to load agent '%s': %v", agentStr, err)
	}

	// Update the application and options
	app.SetAgent(agentCfg, tempOpts.AgentPath)
	opts.AgentPath = tempOpts.AgentPath
	if agentName != "" {
		opts.AgentName = agentName
	}

	// Restart MCP servers if needed
	if len(agentCfg.MCPServers) > 0 {
		// Stop existing servers
		app.StopMCPServers()

		// Start new servers
		ctx := context.Background()
		if err := app.StartMCPServers(ctx); err != nil {
			return fmt.Errorf("failed to start MCP servers for agent: %v", err)
		}
	} else {
		// Stop all servers if the new agent doesn't have any
		app.StopMCPServers()
	}

	return nil
}

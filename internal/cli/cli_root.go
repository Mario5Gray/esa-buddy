package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/meain/esa/internal/agent"
	"github.com/meain/esa/internal/buildinfo"
	"github.com/meain/esa/internal/conversation"
	"github.com/meain/esa/internal/options"
	"github.com/spf13/cobra"
)

func CreateRootCommand() *cobra.Command {
	opts := &options.CLIOptions{}

	rootCmd := &cobra.Command{
		Use:          "esa [text]",
		SilenceUsage: true,
		Version:      fmt.Sprintf("%s (built %s)", buildinfo.Commit, buildinfo.Date),
		Short:        "Personalized micro agents",
		Long: "Esa is a command-line tool for interacting with personalized micro agents" +
			" that can execute tasks, answer questions, and assist with various functions.",
		Example: `  esa Will it rain tomorrow
  esa +coder How do I write a function in Go
  esa --repl
  esa --repl "initial query"
  esa --list-agents
  esa --show-agent +coder
  esa --show-agent ~/.config/esa/agents/custom.toml
  esa --list-history
  esa --show-history 1
  esa --show-history 1 --output json
  esa --show-output 1
  esa --show-stats`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Handle REPL mode first
			if opts.ReplMode {
				return runReplMode(opts, args)
			}

			if opts.AskLevel != "" &&
				!slices.Contains([]string{"none", "unsafe", "all"}, opts.AskLevel) {
				return fmt.Errorf(
					"invalid ask level: %s. Must be one of: none, unsafe, all",
					opts.AskLevel,
				)
			}

			if opts.OutputFormat == "" &&
				!slices.Contains([]string{"text", "markdown", "json"}, opts.OutputFormat) {
				return fmt.Errorf(
					"invalid output format: %s. Must be one of: text, markdown, json",
					opts.OutputFormat,
				)
			}

			// Handle list/show flags first
			if opts.ListAgents {
				listAgents()
				return nil
			}

			if opts.ListUserAgents {
				listUserAgents()
				return nil
			}

			if opts.ListHistory {
				listHistory(opts.ShowAll)
				return nil
			}

			if opts.ShowHistory {
				// Require positional argument for history index
				if len(args) == 0 {
					return fmt.Errorf("history index must be provided as argument: esa --show-history <index>")
				}

				handleShowHistory(args[0], opts.OutputFormat)
				return nil
			}

			if opts.ShowOutput {
				// Require positional argument for history index
				if len(args) == 0 {
					return fmt.Errorf("history index must be provided as argument: esa --show-output <index>")
				}

				handleShowOutput(args[0], opts.Pretty)
				return nil
			}

			if opts.Inspect {
				if len(args) == 0 {
					return fmt.Errorf("history index must be provided as argument: esa --inspect <index>")
				}
				if opts.InspectFormat == "" {
					opts.InspectFormat = "text"
				}
				if !slices.Contains([]string{"text", "json"}, opts.InspectFormat) {
					return fmt.Errorf("invalid inspect format: %s. Must be one of: text, json", opts.InspectFormat)
				}
				handleInspect(args[0], opts.InspectFormat)
				return nil
			}

			if opts.ShowStats {
				handleShowStats(opts.ShowAll)
				return nil
			}

			if opts.ShowAgent {
				// Require positional argument for agent
				if len(args) == 0 {
					return fmt.Errorf("agent must be provided as argument: esa --show-agent <agent> or esa --show-agent +<agent>")
				}

				_, agentPath := agent.ParseAgentString(args[0])
				handleShowAgent(agentPath)
				return nil
			}

			// Normal execution - join args as command string
			opts.CommandStr = strings.Join(args, " ")

			// Handle agent selection with + prefix
			if strings.HasPrefix(opts.CommandStr, "+") {
				parseAgentCommand(opts)
			}

			app, err := conversation.NewApplication(opts)
			if err != nil {
				return fmt.Errorf("failed to initialize application: %v", err)
			}

			app.Run(*opts)
			return nil
		},
	}

	// Add flags
	rootCmd.Flags().BoolVar(&opts.DebugMode, "debug", false, "Enable debug mode")
	rootCmd.Flags().BoolVarP(&opts.ContinueChat, "continue", "c", false, "Continue last conversation")
	rootCmd.Flags().StringVarP(&opts.Conversation, "conversation", "C", "", "Specify the conversation to continue or retry")
	rootCmd.Flags().BoolVarP(&opts.RetryChat, "retry", "r", false, "Retry last command")
	rootCmd.Flags().BoolVar(&opts.ReplMode, "repl", false, "Start in REPL mode for interactive conversation")
	rootCmd.Flags().StringVar(&opts.AgentPath, "agent", "", "Path to agent config file")
	rootCmd.Flags().StringVar(&opts.ConfigPath, "config", "", "Path to the global config file (default: ~/.config/esa/config.toml)")
	rootCmd.Flags().StringVarP(&opts.Model, "model", "m", "", "Model to use (e.g., openai/gpt-4)")
	rootCmd.Flags().StringVar(&opts.AskLevel, "ask", "", "Ask level (none, unsafe, all)")
	rootCmd.Flags().BoolVar(&opts.ShowCommands, "show-commands", false, "Show executed commands during run")
	rootCmd.Flags().BoolVar(&opts.ShowToolCalls, "show-tool-calls", false, "Show executed commands and their outputs during run")
	rootCmd.Flags().BoolVar(&opts.HideProgress, "hide-progress", false, "Disable progress info for each function")
	rootCmd.Flags().StringVar(&opts.OutputFormat, "output", "text", "Output format for --show-history (text, markdown, json)")
	rootCmd.Flags().BoolVarP(&opts.Pretty, "pretty", "p", false, "Pretty print markdown output (disables streaming)")
	rootCmd.Flags().StringVar(&opts.SystemPrompt, "system-prompt", "", "Override the system prompt for the agent")
	rootCmd.Flags().BoolVar(&opts.Think, "think", false, "Enable model thinking/chain-of-thought for this request")
	rootCmd.Flags().BoolVar(&opts.NoThink, "no-think", false, "Disable model thinking/chain-of-thought for this request")
	rootCmd.Flags().BoolVar(&opts.Compaction, "compaction", false, "Enable prompt compaction for this request")
	rootCmd.Flags().BoolVar(&opts.NoCompaction, "no-compaction", false, "Disable prompt compaction for this request")

	// List/show flags
	rootCmd.Flags().BoolVar(&opts.ListAgents, "list-agents", false, "List all available agents")
	rootCmd.Flags().BoolVar(&opts.ListUserAgents, "list-user-agents", false, "List only user agents")
	rootCmd.Flags().BoolVar(&opts.ListHistory, "list-history", false, "List all saved conversation histories")
	rootCmd.Flags().BoolVar(&opts.ShowAgent, "show-agent", false, "Show agent details (requires agent name/path as argument)")
	rootCmd.Flags().BoolVar(&opts.ShowHistory, "show-history", false, "Show conversation history (requires history index as argument)")
	rootCmd.Flags().BoolVar(&opts.ShowOutput, "show-output", false, "Show just the output from a history entry (requires history index as argument)")
	rootCmd.Flags().BoolVar(&opts.ShowStats, "show-stats", false, "Show usage statistics based on conversation history")
	rootCmd.Flags().BoolVar(&opts.Inspect, "inspect", false, "Inspect conversation tape from epoch to head (requires history index as argument)")
	rootCmd.Flags().StringVar(&opts.InspectFormat, "inspect-format", "text", "Output format for --inspect (text, json)")
	rootCmd.Flags().BoolVar(&opts.ShowAll, "all", false, "Show all items when used with --list-history or --show-stats")

	// Make history-index required when show-history is used
	rootCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		// Validate output format
		validFormats := map[string]bool{"text": true, "markdown": true, "json": true}
		if !validFormats[opts.OutputFormat] {
			return fmt.Errorf("invalid output format %q. Must be one of: text, markdown, json", opts.OutputFormat)
		}

		return nil
	}

	return rootCmd
}

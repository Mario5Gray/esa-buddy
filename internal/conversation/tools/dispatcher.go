package tools

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/meain/esa/internal/agent"
	"github.com/meain/esa/internal/executor"
	"github.com/meain/esa/internal/llm"
	"github.com/meain/esa/internal/mcp"
	"github.com/meain/esa/internal/options"
	"github.com/meain/esa/internal/security"
	"github.com/sashabaranov/go-openai"
)

const (
	toolCallCommandColor = color.FgCyan
	toolCallOutputColor  = color.FgWhite
)

type Deps struct {
	Agent                agent.Agent
	ShowCommands         bool
	ShowToolCalls        bool
	ShowProgress         bool
	LastProgressLen      *int
	Messages             *[]openai.ChatCompletionMessage
	ToolGate             security.GateChain
	ExecTooler           executor.Executor
	MCPManager           *mcp.MCPManager
	DebugPrint           func(section string, v ...any)
	ParseModel           func() (provider string, model string, info llm.ProviderInfo)
	GetEffectiveAskLevel func() string
}

type Dispatcher struct {
	deps Deps
}

func NewDispatcher(deps Deps) *Dispatcher {
	return &Dispatcher{deps: deps}
}

func (d *Dispatcher) GenerateProgressSummary(funcName string, args string) string {
	_ = args
	return fmt.Sprintf("Calling %s...", funcName)
}

func (d *Dispatcher) HandleToolCalls(toolCalls []openai.ToolCall, opts options.CLIOptions) {
	_ = opts
	for _, toolCall := range toolCalls {
		if toolCall.Type != "function" || toolCall.Function.Name == "" {
			continue
		}

		// Check if it's an MCP tool (starts with "mcp_")
		// FIXME: This might not be reliable, the user might define a
		// function that starts with mcp_
		if strings.HasPrefix(toolCall.Function.Name, "mcp_") {
			d.HandleMCPToolCall(toolCall, opts)
			continue
		}

		// Handle regular function
		var matchedFunc agent.FunctionConfig
		for _, fc := range d.deps.Agent.Functions {
			if fc.Name == toolCall.Function.Name {
				matchedFunc = fc
				break
			}
		}

		if matchedFunc.Name == "" {
			log.Fatalf("No matching function found for: %s", toolCall.Function.Name)
		}

		if d.deps.ShowProgress && len(matchedFunc.Output) == 0 {
			if summary := d.GenerateProgressSummary(matchedFunc.Name, toolCall.Function.Arguments); summary != "" {
				// Clear previous line if exists
				if d.lastProgressLen() > 0 {
					fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", d.lastProgressLen()))
				}
				msg := fmt.Sprintf("⋮ %s", summary)
				color.New(color.FgBlue).Fprint(os.Stderr, msg)
				d.setLastProgressLen(len(msg))
			}
		}

		// Set the provider and model env so that nested esa calls
		// make use of it. Users can override this by setting the
		// value explicitly in the nested esa calls.
		if d.deps.ParseModel != nil {
			provider, model, _ := d.deps.ParseModel()
			os.Setenv("ESA_MODEL", fmt.Sprintf("%s/%s", provider, model))
		}

		intent := security.ToolIntent{
			ToolName: matchedFunc.Name,
			ArgsJSON: toolCall.Function.Arguments,
		}
		// Tool execution is gated by an ordered policy chain. The first gate to
		// return Allow or Deny wins; Abstain continues; all-abstain defaults to Deny.
		decision, _, err := d.deps.ToolGate.Evaluate(intent)
		if err != nil || decision != security.Allow {
			d.debugPrint("Tool Gate",
				fmt.Sprintf("Decision: %v", decision),
				fmt.Sprintf("Error: %v", err))
			d.appendMessage(openai.ChatCompletionMessage{
				Role:       "tool",
				Name:       toolCall.Function.Name,
				Content:    "Tool execution denied by policy.",
				ToolCallID: toolCall.ID,
			})
			continue
		}

		execTooler := d.deps.ExecTooler
		if execTooler == nil {
			execTooler = executor.DefaultExecutor{}
		}
		askLevel := ""
		if d.deps.GetEffectiveAskLevel != nil {
			askLevel = d.deps.GetEffectiveAskLevel()
		}
		approved, command, stdin, result, err := execTooler.Execute(
			askLevel,
			matchedFunc,
			toolCall.Function.Arguments,
		)
		d.debugPrint("Function Execution",
			fmt.Sprintf("Function: %s", matchedFunc.Name),
			fmt.Sprintf("Approved: %s", fmt.Sprint(approved)),
			fmt.Sprintf("Command: %s", command),
			fmt.Sprintf("Stdin: %s", stdin),
			fmt.Sprintf("Output: %s", result))

		if err != nil {
			d.debugPrint("Function Error", err)
			// Clear progress line before showing error
			if d.deps.ShowProgress && d.lastProgressLen() > 0 {
				fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", d.lastProgressLen()))
				d.setLastProgressLen(0)
			}

			d.appendMessage(openai.ChatCompletionMessage{
				Role:       "tool",
				Name:       toolCall.Function.Name,
				Content:    fmt.Sprintf("Error: %v", err),
				ToolCallID: toolCall.ID,
			})
			continue
		}

		content := fmt.Sprintf("Command: %s\n\nOutput: \n%s", command, result)

		// Display command when --show-commands is enabled
		if d.deps.ShowCommands || d.deps.ShowToolCalls {
			color.New(toolCallCommandColor).Fprintf(os.Stderr, "$ %s\n", command)
		}

		// Display tool call output when --show-tool-calls is enabled
		if d.deps.ShowToolCalls {
			color.New(toolCallOutputColor).Fprintf(os.Stderr, "%s\n", result)
		}

		d.appendMessage(openai.ChatCompletionMessage{
			Role:       "tool",
			Name:       toolCall.Function.Name,
			Content:    content,
			ToolCallID: toolCall.ID,
		})
	}
}

// HandleMCPToolCall handles tool calls for MCP servers
func (d *Dispatcher) HandleMCPToolCall(toolCall openai.ToolCall, opts options.CLIOptions) {
	_ = opts
	if d.deps.ShowProgress {
		if summary := d.GenerateProgressSummary(toolCall.Function.Name, toolCall.Function.Arguments); summary != "" {
			// Clear previous line if exists
			if d.lastProgressLen() > 0 {
				fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", d.lastProgressLen()))
			}
			msg := fmt.Sprintf("⋮ %s", summary)
			color.New(color.FgBlue).Fprint(os.Stderr, msg)
			d.setLastProgressLen(len(msg))
		}
	}

	// Parse the arguments
	var arguments any
	if toolCall.Function.Arguments != "" {
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &arguments); err != nil {
			d.debugPrint("MCP Tool Error", fmt.Sprintf("Failed to parse arguments: %v", err))
			// Clear progress line before showing error
			if d.deps.ShowProgress && d.lastProgressLen() > 0 {
				fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", d.lastProgressLen()))
				d.setLastProgressLen(0)
			}

			d.appendMessage(openai.ChatCompletionMessage{
				Role:       "tool",
				Name:       toolCall.Function.Name,
				Content:    fmt.Sprintf("Error: Failed to parse arguments: %v", err),
				ToolCallID: toolCall.ID,
			})
			return
		}
	}

	// Call the MCP tool with ask level
	askLevel := ""
	if d.deps.GetEffectiveAskLevel != nil {
		askLevel = d.deps.GetEffectiveAskLevel()
	}
	result, err := d.deps.MCPManager.CallTool(toolCall.Function.Name, arguments, askLevel)

	d.debugPrint("MCP Tool Execution",
		fmt.Sprintf("Tool: %s", toolCall.Function.Name),
		fmt.Sprintf("Arguments: %s", toolCall.Function.Arguments),
		fmt.Sprintf("Output: %s", result))

	if err != nil {
		d.debugPrint("MCP Tool Error", err)
		// Clear progress line before showing error
		if d.deps.ShowProgress && d.lastProgressLen() > 0 {
			fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", d.lastProgressLen()))
			d.setLastProgressLen(0)
		}

		d.appendMessage(openai.ChatCompletionMessage{
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
	if d.deps.ShowCommands || d.deps.ShowToolCalls {
		color.New(toolCallCommandColor).Fprintf(os.Stderr, "# %s(%s)\n", toolCall.Function.Name, argsDisplay)
	}

	// Display MCP tool call with output when --show-tool-calls is enabled
	if d.deps.ShowToolCalls {
		color.New(toolCallOutputColor).Fprintf(os.Stderr, "%s\n", result)
	}

	d.appendMessage(openai.ChatCompletionMessage{
		Role:       "tool",
		Name:       toolCall.Function.Name,
		Content:    result,
		ToolCallID: toolCall.ID,
	})
}

func (d *Dispatcher) debugPrint(section string, v ...any) {
	if d.deps.DebugPrint != nil {
		d.deps.DebugPrint(section, v...)
	}
}

func (d *Dispatcher) appendMessage(msg openai.ChatCompletionMessage) {
	if d.deps.Messages == nil {
		return
	}
	*d.deps.Messages = append(*d.deps.Messages, msg)
}

func (d *Dispatcher) lastProgressLen() int {
	if d.deps.LastProgressLen == nil {
		return 0
	}
	return *d.deps.LastProgressLen
}

func (d *Dispatcher) setLastProgressLen(v int) {
	if d.deps.LastProgressLen == nil {
		return
	}
	*d.deps.LastProgressLen = v
}

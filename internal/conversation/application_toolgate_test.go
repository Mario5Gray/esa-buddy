package conversation

import (
	"testing"

	"github.com/meain/esa/internal/agent"
	"github.com/meain/esa/internal/options"
	"github.com/meain/esa/internal/security"
	"github.com/sashabaranov/go-openai"
)

func TestHandleToolCallsDeniedByGate(t *testing.T) {
	app := &Application{
		agent: agent.Agent{
			Functions: []agent.FunctionConfig{
				{
					Name:        "echo",
					Description: "echo",
					Command:     "echo hi",
					Safe:        true,
				},
			},
		},
		toolGate: security.GateChain{
			Gates: []security.Gate{security.DenyGate{}},
		},
	}
	app.debugPrint = func(string, ...any) {}

	toolCalls := []openai.ToolCall{
		{
			ID:   "call_1",
			Type: "function",
			Function: openai.FunctionCall{
				Name:      "echo",
				Arguments: "{}",
			},
		},
	}

	app.handleToolCalls(toolCalls, options.CLIOptions{})

	if len(app.messages) == 0 {
		t.Fatalf("expected tool denial message, got none")
	}

	last := app.messages[len(app.messages)-1]
	if last.Role != "tool" {
		t.Fatalf("expected role tool, got %q", last.Role)
	}
	if last.Content != "Tool execution denied by policy." {
		t.Fatalf("unexpected content: %q", last.Content)
	}
	if last.ToolCallID != "call_1" {
		t.Fatalf("expected tool_call_id %q, got %q", "call_1", last.ToolCallID)
	}
}

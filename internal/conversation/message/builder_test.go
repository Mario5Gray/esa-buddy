package message_test

import (
	"strings"
	"testing"

	"github.com/meain/esa/internal/conversation/message"
	"github.com/sashabaranov/go-openai"
)

func should_build_message_with_correct_role_and_content(t *testing.T) {
	msg := message.New("user", "hello world").Build()

	if msg.Role != "user" {
		t.Errorf("expected role %q, got %q", "user", msg.Role)
	}
	if msg.Content != "hello world" {
		t.Errorf("expected content %q, got %q", "hello world", msg.Content)
	}
}

func TestBuild(t *testing.T) {
	should_build_message_with_correct_role_and_content(t)
}

func TestApplySingleTransform(t *testing.T) {
	wrap := func(m openai.ChatCompletionMessage) openai.ChatCompletionMessage {
		m.Content = "[wrapped] " + m.Content
		return m
	}

	msg := message.New("tool", "raw output").Apply(wrap).Build()

	if msg.Content != "[wrapped] raw output" {
		t.Errorf("expected wrapped content, got %q", msg.Content)
	}
}

func TestApplyTransformsInRegistrationOrder(t *testing.T) {
	// Pipes and Filters: order must be deterministic and match registration sequence.
	// (EIP, Hohpe §3 — filter ordering guarantee)
	var order []string

	first := func(m openai.ChatCompletionMessage) openai.ChatCompletionMessage {
		order = append(order, "first")
		m.Content = "A:" + m.Content
		return m
	}
	second := func(m openai.ChatCompletionMessage) openai.ChatCompletionMessage {
		order = append(order, "second")
		m.Content = "B:" + m.Content
		return m
	}
	third := func(m openai.ChatCompletionMessage) openai.ChatCompletionMessage {
		order = append(order, "third")
		m.Content = "C:" + m.Content
		return m
	}

	msg := message.New("tool", "x").Apply(first).Apply(second).Apply(third).Build()

	if order[0] != "first" || order[1] != "second" || order[2] != "third" {
		t.Errorf("transforms applied out of order: %v", order)
	}
	if msg.Content != "C:B:A:x" {
		t.Errorf("unexpected content after chained transforms: %q", msg.Content)
	}
}

func TestTransformCanInspectRole(t *testing.T) {
	// A transform that only acts on tool messages should leave others untouched.
	onlyTool := func(m openai.ChatCompletionMessage) openai.ChatCompletionMessage {
		if m.Role != "tool" {
			return m
		}
		m.Content = "<tool_data>" + m.Content + "</tool_data>"
		return m
	}

	toolMsg := message.New("tool", "output").Apply(onlyTool).Build()
	userMsg := message.New("user", "output").Apply(onlyTool).Build()

	if !strings.HasPrefix(toolMsg.Content, "<tool_data>") {
		t.Errorf("expected tool message to be wrapped, got %q", toolMsg.Content)
	}
	if strings.HasPrefix(userMsg.Content, "<tool_data>") {
		t.Errorf("expected user message to be unchanged, got %q", userMsg.Content)
	}
}

func TestWithNameAndToolCallID(t *testing.T) {
	msg := message.New("tool", "result").
		WithName("run_shell_command").
		WithToolCallID("call_abc123").
		Build()

	if msg.Name != "run_shell_command" {
		t.Errorf("expected name %q, got %q", "run_shell_command", msg.Name)
	}
	if msg.ToolCallID != "call_abc123" {
		t.Errorf("expected tool call ID %q, got %q", "call_abc123", msg.ToolCallID)
	}
}

func TestWithToolCalls(t *testing.T) {
	calls := []openai.ToolCall{
		{ID: "call_1", Type: "function"},
	}

	msg := message.New("assistant", "").
		WithToolCalls(calls).
		Build()

	if len(msg.ToolCalls) != 1 || msg.ToolCalls[0].ID != "call_1" {
		t.Errorf("expected tool calls to be preserved, got %v", msg.ToolCalls)
	}
}

func TestNoTransformsIsPassThrough(t *testing.T) {
	msg := message.New("assistant", "I can help with that.").Build()

	if msg.Content != "I can help with that." {
		t.Errorf("expected content unchanged, got %q", msg.Content)
	}
	if msg.Role != "assistant" {
		t.Errorf("expected role unchanged, got %q", msg.Role)
	}
}

package message_test

import (
	"strings"
	"testing"

	"github.com/meain/esa/internal/conversation/message"
	"github.com/sashabaranov/go-openai"
)

// --- OnlyFor -----------------------------------------------------------------

func TestOnlyFor_should_apply_transform_for_matching_role(t *testing.T) {
	mark := func(m openai.ChatCompletionMessage) openai.ChatCompletionMessage {
		m.Content = "[marked] " + m.Content
		return m
	}
	wrapped := message.OnlyFor("tool")(mark)
	msg := wrapped(openai.ChatCompletionMessage{Role: "tool", Content: "data"})

	if msg.Content != "[marked] data" {
		t.Errorf("expected transform to run for matching role, got %q", msg.Content)
	}
}

func TestOnlyFor_should_skip_transform_for_non_matching_role(t *testing.T) {
	mark := func(m openai.ChatCompletionMessage) openai.ChatCompletionMessage {
		m.Content = "[marked] " + m.Content
		return m
	}
	wrapped := message.OnlyFor("tool")(mark)

	for _, role := range []string{"user", "assistant", "system"} {
		msg := wrapped(openai.ChatCompletionMessage{Role: role, Content: "original"})
		if msg.Content != "original" {
			t.Errorf("role %q: expected pass-through, got %q", role, msg.Content)
		}
	}
}

func TestOnlyFor_should_match_any_of_multiple_roles(t *testing.T) {
	mark := func(m openai.ChatCompletionMessage) openai.ChatCompletionMessage {
		m.Content = "[marked]"
		return m
	}
	wrapped := message.OnlyFor("tool", "user")(mark)

	tool := wrapped(openai.ChatCompletionMessage{Role: "tool", Content: "x"})
	user := wrapped(openai.ChatCompletionMessage{Role: "user", Content: "x"})
	system := wrapped(openai.ChatCompletionMessage{Role: "system", Content: "x"})

	if tool.Content != "[marked]" {
		t.Errorf("expected tool to be marked, got %q", tool.Content)
	}
	if user.Content != "[marked]" {
		t.Errorf("expected user to be marked, got %q", user.Content)
	}
	if system.Content != "x" {
		t.Errorf("expected system to be unchanged, got %q", system.Content)
	}
}

// --- SkipFor -----------------------------------------------------------------

func TestSkipFor_should_skip_transform_for_matching_role(t *testing.T) {
	mark := func(m openai.ChatCompletionMessage) openai.ChatCompletionMessage {
		m.Content = "[marked] " + m.Content
		return m
	}
	wrapped := message.SkipFor("system")(mark)
	msg := wrapped(openai.ChatCompletionMessage{Role: "system", Content: "trusted prompt"})

	if msg.Content != "trusted prompt" {
		t.Errorf("expected system message to be skipped, got %q", msg.Content)
	}
}

func TestSkipFor_should_apply_transform_for_non_matching_role(t *testing.T) {
	mark := func(m openai.ChatCompletionMessage) openai.ChatCompletionMessage {
		m.Content = "[marked] " + m.Content
		return m
	}
	wrapped := message.SkipFor("system")(mark)

	for _, role := range []string{"user", "assistant", "tool"} {
		msg := wrapped(openai.ChatCompletionMessage{Role: role, Content: "data"})
		if msg.Content != "[marked] data" {
			t.Errorf("role %q: expected transform to run, got %q", role, msg.Content)
		}
	}
}

func TestSkipFor_should_skip_multiple_roles(t *testing.T) {
	mark := func(m openai.ChatCompletionMessage) openai.ChatCompletionMessage {
		m.Content = "[marked]"
		return m
	}
	wrapped := message.SkipFor("system", "assistant")(mark)

	system := wrapped(openai.ChatCompletionMessage{Role: "system", Content: "x"})
	assistant := wrapped(openai.ChatCompletionMessage{Role: "assistant", Content: "x"})
	tool := wrapped(openai.ChatCompletionMessage{Role: "tool", Content: "x"})

	if system.Content != "x" {
		t.Errorf("expected system to be unchanged, got %q", system.Content)
	}
	if assistant.Content != "x" {
		t.Errorf("expected assistant to be unchanged, got %q", assistant.Content)
	}
	if tool.Content != "[marked]" {
		t.Errorf("expected tool to be marked, got %q", tool.Content)
	}
}

// --- Envelope ----------------------------------------------------------------

func TestEnvelope_should_wrap_tool_message(t *testing.T) {
	msg := openai.ChatCompletionMessage{Role: "tool", Name: "curl", Content: "response body"}
	result := message.Envelope(msg)

	if !strings.HasPrefix(result.Content, `<tool_data source="curl">`) {
		t.Errorf("expected envelope header, got %q", result.Content)
	}
	if !strings.Contains(result.Content, "response body") {
		t.Errorf("expected payload inside envelope, got %q", result.Content)
	}
}

func TestEnvelope_should_pass_through_user_message(t *testing.T) {
	msg := openai.ChatCompletionMessage{Role: "user", Content: "what is the weather?"}
	result := message.Envelope(msg)

	if result.Content != "what is the weather?" {
		t.Errorf("user message should be unchanged, got %q", result.Content)
	}
}

func TestEnvelope_should_pass_through_system_message(t *testing.T) {
	msg := openai.ChatCompletionMessage{Role: "system", Content: "You are Esa."}
	result := message.Envelope(msg)

	if result.Content != "You are Esa." {
		t.Errorf("system message should be unchanged, got %q", result.Content)
	}
}

func TestEnvelope_should_pass_through_assistant_message(t *testing.T) {
	msg := openai.ChatCompletionMessage{Role: "assistant", Content: "Here is the answer."}
	result := message.Envelope(msg)

	if result.Content != "Here is the answer." {
		t.Errorf("assistant message should be unchanged, got %q", result.Content)
	}
}

func TestEnvelope_source_attribute_reflects_tool_name(t *testing.T) {
	msg := openai.ChatCompletionMessage{Role: "tool", Name: "run_shell_command", Content: "ok"}
	result := message.Envelope(msg)

	if !strings.Contains(result.Content, `source="run_shell_command"`) {
		t.Errorf("source attribute missing or wrong: %q", result.Content)
	}
}

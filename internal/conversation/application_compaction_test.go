package conversation

import (
	"strings"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestSplitMessagesForCompaction(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: "system", Content: "system prompt"},
		{Role: "user", Content: "u1"},
		{Role: "assistant", Content: "a1"},
		{Role: "user", Content: "u2"},
		{Role: "assistant", Content: "a2"},
	}

	system, toSummarize, tail, existingSummary, ok := splitMessagesForCompaction(messages, 2)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if system.Role != "system" || system.Content != "system prompt" {
		t.Fatalf("unexpected system message: %+v", system)
	}
	if existingSummary != "" {
		t.Fatalf("expected empty existing summary, got %q", existingSummary)
	}
	if len(toSummarize) != 2 {
		t.Fatalf("expected 2 messages to summarize, got %d", len(toSummarize))
	}
	if len(tail) != 2 {
		t.Fatalf("expected 2 tail messages, got %d", len(tail))
	}
	if tail[0].Content != "u2" || tail[1].Content != "a2" {
		t.Fatalf("unexpected tail: %+v", tail)
	}
}

func TestFormatMessageForSummaryToolOutputOmitted(t *testing.T) {
	msg := openai.ChatCompletionMessage{
		Role:       "tool",
		Name:       "example_tool",
		Content:    "very long output",
		ToolCallID: "call_123",
	}
	got := formatMessageForSummary(msg)
	if got == "" {
		t.Fatalf("expected formatted message")
	}
	if !strings.Contains(got, "[output omitted") {
		t.Fatalf("expected output omission marker, got %q", got)
	}
	if strings.Contains(got, "very long output") {
		t.Fatalf("tool output should be omitted from summary, got %q", got)
	}
}

func TestBuildCompactionInputIncludesPreviousSummary(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: "user", Content: "u1"},
		{Role: "assistant", Content: "a1"},
	}
	got := buildCompactionInput("prior summary", messages)
	if !strings.Contains(got, "Previous summary:") || !strings.Contains(got, "prior summary") {
		t.Fatalf("expected previous summary in input, got %q", got)
	}
	if !strings.Contains(got, "Conversation to summarize:") {
		t.Fatalf("expected conversation header in input, got %q", got)
	}
}

func TestMessageSizeCountsToolCallFields(t *testing.T) {
	msg := openai.ChatCompletionMessage{
		Role:    "assistant",
		Content: "hi",
		ToolCalls: []openai.ToolCall{
			{
				ID:   "tool_1",
				Type: "function",
				Function: openai.FunctionCall{
					Name:      "demo",
					Arguments: "{\"a\":1}",
				},
			},
		},
	}
	size := messageSize(msg)
	if size <= len(msg.Role)+len(msg.Content) {
		t.Fatalf("expected tool call fields to contribute to size, got %d", size)
	}
}

func TestShouldCompactTokenThreshold(t *testing.T) {
	if !shouldCompact(10, 100, 100, 10000, 800, 750) {
		t.Fatalf("expected token threshold to trigger compaction")
	}
	if shouldCompact(10, 100, 100, 10000, 700, 750) {
		t.Fatalf("did not expect token threshold to trigger compaction")
	}
	if shouldCompact(10, 100, 100, 10000, 0, 750) {
		t.Fatalf("did not expect compaction with zero token estimate")
	}
	if shouldCompact(10, 100, 100, 10000, 800, 0) {
		t.Fatalf("did not expect compaction with zero token threshold")
	}
}

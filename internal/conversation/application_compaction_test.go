package conversation

import (
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestSplitMessagesForCompaction(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: "system", Content: "system prompt"},
		{Role: "system", Content: compactionSummaryPrefix + "old summary"},
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
	if existingSummary != "old summary" {
		t.Fatalf("expected existing summary, got %q", existingSummary)
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

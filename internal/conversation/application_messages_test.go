package conversation

import (
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestBuildRequestMessagesSkipsEmptySummary(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: "system", Content: "system"},
		{Role: "user", Content: "hello"},
	}
	got := buildRequestMessages(messages, "   ")
	if len(got) != len(messages) {
		t.Fatalf("expected no change, got %d messages", len(got))
	}
}

func TestBuildRequestMessagesInsertsAfterSystem(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: "system", Content: "system"},
		{Role: "user", Content: "hello"},
	}
	got := buildRequestMessages(messages, "summary")
	if len(got) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(got))
	}
	if got[1].Role != "system" || got[1].Content != "summary" {
		t.Fatalf("expected summary inserted after system, got %+v", got[1])
	}
	if got[2].Content != "hello" {
		t.Fatalf("unexpected message order: %+v", got)
	}
}

func TestBuildRequestMessagesInsertsFirstWhenNoSystem(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: "user", Content: "hello"},
	}
	got := buildRequestMessages(messages, "summary")
	if len(got) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(got))
	}
	if got[0].Role != "system" || got[0].Content != "summary" {
		t.Fatalf("expected summary at start, got %+v", got[0])
	}
	if got[1].Content != "hello" {
		t.Fatalf("unexpected message order: %+v", got)
	}
}

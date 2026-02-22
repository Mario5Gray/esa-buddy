package history

import (
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestEnsureMessageMetaAppendsAndFills(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: "system", Content: "s"},
		{Role: "user", Content: "u"},
	}
	meta := EnsureMessageMeta(nil, messages, "openai/gpt-test", func() string { return "id" })
	if len(meta) != 2 {
		t.Fatalf("expected 2 meta entries, got %d", len(meta))
	}
	if meta[0].ID == "" || meta[1].ID == "" {
		t.Fatalf("expected IDs to be set: %+v", meta)
	}
	if meta[0].Model != "openai/gpt-test" || meta[1].Model != "openai/gpt-test" {
		t.Fatalf("expected model to be set on all entries: %+v", meta)
	}
	if meta[0].Role != "system" || meta[1].Role != "user" {
		t.Fatalf("expected roles to be set from messages: %+v", meta)
	}
}

func TestEnsureMessageMetaTrims(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: "system", Content: "s"},
	}
	existing := []HistoryMessageMeta{
		{ID: "1", Model: "m1", Role: "system"},
		{ID: "2", Model: "m1", Role: "user"},
	}
	meta := EnsureMessageMeta(existing, messages, "m2", func() string { return "id" })
	if len(meta) != 1 {
		t.Fatalf("expected trimmed meta to length 1, got %d", len(meta))
	}
	if meta[0].ID != "1" {
		t.Fatalf("expected first entry to remain after trim, got %+v", meta[0])
	}
}

func TestEnsureMessageMetaFillsMissing(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: "system", Content: "s"},
		{Role: "user", Content: "u"},
	}
	existing := []HistoryMessageMeta{
		{ID: "1"},
		{ID: "2", Model: "custom"},
	}
	meta := EnsureMessageMeta(existing, messages, "openai/gpt-test", func() string { return "id" })
	if meta[0].Model != "openai/gpt-test" || meta[0].Role != "system" {
		t.Fatalf("expected missing fields to be filled, got %+v", meta[0])
	}
	if meta[1].Model != "custom" || meta[1].Role != "user" {
		t.Fatalf("expected existing model to remain and role filled, got %+v", meta[1])
	}
}

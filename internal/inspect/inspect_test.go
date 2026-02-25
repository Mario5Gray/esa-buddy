package inspect

import (
	"testing"
	"time"

	"github.com/meain/esa/internal/conversation/history"
	"github.com/sashabaranov/go-openai"
)

func TestBuildTapeParsesFileName(t *testing.T) {
	h := history.ConversationHistory{
		AgentPath: "builtin:default",
		Model:     "openai/gpt-test",
		Messages: []openai.ChatCompletionMessage{
			{Role: "system", Content: "s"},
			{Role: "user", Content: "u"},
		},
		Compaction: &history.CompactionMeta{Summary: "summary"},
	}

	tape := BuildTape("session---agent-20240101-120000.json", h)
	if tape.ConversationID != "session" {
		t.Fatalf("expected conversation id, got %q", tape.ConversationID)
	}
	if tape.AgentPath != "builtin:default" || tape.Model != "openai/gpt-test" {
		t.Fatalf("unexpected metadata: %+v", tape)
	}
	if tape.StartTime.IsZero() {
		t.Fatalf("expected start time")
	}
	if tape.StartTime.Year() != 2024 {
		t.Fatalf("expected parsed time, got %s", tape.StartTime)
	}
	if tape.Summary != "summary" {
		t.Fatalf("expected summary")
	}
}

func TestTextRenderer(t *testing.T) {
	tape := Tape{
		ConversationID: "c1",
		AgentPath:      "a1",
		Model:          "m1",
		StartTime:      time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Messages: []historyMessage{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi"},
		},
	}
	out, err := TextRenderer{}.Render(tape)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	if out == "" {
		t.Fatalf("expected output")
	}
}

func TestJSONRenderer(t *testing.T) {
	tape := Tape{
		ConversationID: "c1",
		Messages:       []historyMessage{{Role: "user", Content: "hi"}},
	}
	out, err := JSONRenderer{}.Render(tape)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	if out == "" || out[0] != '{' {
		t.Fatalf("expected json output")
	}
}

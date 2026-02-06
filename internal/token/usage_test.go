package token

import (
	"encoding/json"
	"testing"
)

func TestUsageAdd(t *testing.T) {
	var u Usage

	u.Add(100, 25)
	u.Add(200, 50)

	if u.PromptTokens != 300 {
		t.Errorf("PromptTokens = %d, want 300", u.PromptTokens)
	}
	if u.CompletionTokens != 75 {
		t.Errorf("CompletionTokens = %d, want 75", u.CompletionTokens)
	}
	if u.TotalTokens != 375 {
		t.Errorf("TotalTokens = %d, want 375", u.TotalTokens)
	}
}

func TestUsageEmpty(t *testing.T) {
	var u Usage
	if !u.Empty() {
		t.Error("zero-value Usage should be empty")
	}

	u.Add(1, 0)
	if u.Empty() {
		t.Error("Usage with tokens should not be empty")
	}
}

func TestUsageJSON(t *testing.T) {
	// Verify round-trip through JSON — this is how usage persists in history files
	original := Usage{PromptTokens: 500, CompletionTokens: 120, TotalTokens: 620}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Usage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded != original {
		t.Errorf("round-trip mismatch: got %+v, want %+v", decoded, original)
	}
}

func TestUsageNilInHistory(t *testing.T) {
	// Simulate loading a history file from before token tracking was added.
	// Usage field should unmarshal as nil without error.
	historyJSON := `{"agent_path":"builtin:default","model":"openai/gpt-4o-mini","messages":[]}`

	type fakeHistory struct {
		Usage *Usage `json:"usage,omitempty"`
	}

	var h fakeHistory
	if err := json.Unmarshal([]byte(historyJSON), &h); err != nil {
		t.Fatalf("unmarshal old history: %v", err)
	}

	if h.Usage != nil {
		t.Error("Usage should be nil for old history files without token data")
	}
}

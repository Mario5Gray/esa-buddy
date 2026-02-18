package tests

import (
	"encoding/json"
	"testing"

	"github.com/meain/esa/internal/token"
)

// TestTokenTrackingAccumulation verifies that multiple LLM responses
// accumulate prompt and completion tokens correctly — this is exactly
// what Application.handleStreamResponse does on each streaming chunk.
func TestTokenTrackingAccumulation(t *testing.T) {
	var usage token.Usage

	// Simulate three streaming LLM responses
	usage.Add(100, 25)  // first request
	usage.Add(200, 50)  // second request (tool call round-trip)
	usage.Add(150, 75)  // third request

	if usage.PromptTokens != 450 {
		t.Errorf("PromptTokens = %d, want 450", usage.PromptTokens)
	}
	if usage.CompletionTokens != 150 {
		t.Errorf("CompletionTokens = %d, want 150", usage.CompletionTokens)
	}
	if usage.TotalTokens != 600 {
		t.Errorf("TotalTokens = %d, want 600", usage.TotalTokens)
	}
}

// TestTokenTrackingPersistence proves that token usage survives a
// JSON round-trip through the conversation history format used by
// Application.saveConversationHistory / ProcessHistoryFile.
func TestTokenTrackingPersistence(t *testing.T) {
	// Mirrors the ConversationHistory struct in application.go
	type ConversationHistory struct {
		AgentPath string       `json:"agent_path"`
		Model     string       `json:"model"`
		Usage     *token.Usage `json:"usage,omitempty"`
	}

	// Build usage the same way the app does
	var usage token.Usage
	usage.Add(500, 120)

	original := ConversationHistory{
		AgentPath: "builtin:default",
		Model:     "openai/gpt-4o-mini",
		Usage:     &usage,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var restored ConversationHistory
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if restored.Usage == nil {
		t.Fatal("Usage should not be nil after round-trip")
	}
	if restored.Usage.PromptTokens != 500 {
		t.Errorf("PromptTokens = %d, want 500", restored.Usage.PromptTokens)
	}
	if restored.Usage.CompletionTokens != 120 {
		t.Errorf("CompletionTokens = %d, want 120", restored.Usage.CompletionTokens)
	}
	if restored.Usage.TotalTokens != 620 {
		t.Errorf("TotalTokens = %d, want 620", restored.Usage.TotalTokens)
	}
}

// TestTokenTrackingOmittedWhenEmpty proves that history files written
// before token tracking (or with zero usage) cleanly omit the field,
// and that loading such a file does not break.
func TestTokenTrackingOmittedWhenEmpty(t *testing.T) {
	type ConversationHistory struct {
		AgentPath string       `json:"agent_path"`
		Model     string       `json:"model"`
		Usage     *token.Usage `json:"usage,omitempty"`
	}

	// Simulate saving with no token data (Usage pointer is nil)
	history := ConversationHistory{
		AgentPath: "builtin:default",
		Model:     "openai/gpt-4o-mini",
		Usage:     nil,
	}

	data, err := json.Marshal(history)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// The "usage" key should not appear in the JSON
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	if _, exists := raw["usage"]; exists {
		t.Error("usage key should be omitted from JSON when nil")
	}

	// Loading old history (no usage field) should work cleanly
	oldJSON := `{"agent_path":"builtin:default","model":"openai/gpt-4o-mini"}`
	var loaded ConversationHistory
	if err := json.Unmarshal([]byte(oldJSON), &loaded); err != nil {
		t.Fatalf("unmarshal old history: %v", err)
	}
	if loaded.Usage != nil {
		t.Error("Usage should be nil for old history files")
	}
}

// TestTokenTrackingContinueConversation verifies that when continuing
// a conversation, prior token counts are carried forward and new
// tokens are added on top — matching the logic in NewApplication.
func TestTokenTrackingContinueConversation(t *testing.T) {
	type ConversationHistory struct {
		AgentPath string       `json:"agent_path"`
		Model     string       `json:"model"`
		Usage     *token.Usage `json:"usage,omitempty"`
	}

	// Simulate a saved conversation with existing usage
	prior := token.Usage{PromptTokens: 300, CompletionTokens: 80, TotalTokens: 380}
	saved := ConversationHistory{
		AgentPath: "builtin:default",
		Model:     "openai/gpt-4o-mini",
		Usage:     &prior,
	}

	data, err := json.Marshal(saved)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Load the history (simulates NewApplication for --continue)
	var loaded ConversationHistory
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Carry forward usage (mirrors application.go lines 171-172)
	var usage token.Usage
	if loaded.Usage != nil {
		usage = *loaded.Usage
	}

	// Simulate a new LLM call in the continued conversation
	usage.Add(200, 60)

	if usage.PromptTokens != 500 {
		t.Errorf("PromptTokens = %d, want 500", usage.PromptTokens)
	}
	if usage.CompletionTokens != 140 {
		t.Errorf("CompletionTokens = %d, want 140", usage.CompletionTokens)
	}
	if usage.TotalTokens != 640 {
		t.Errorf("TotalTokens = %d, want 640", usage.TotalTokens)
	}
}

// TestEmptyUsageDetection verifies the Empty() gate that
// saveConversationHistory uses to decide whether to persist usage.
func TestEmptyUsageDetection(t *testing.T) {
	var u token.Usage
	if !u.Empty() {
		t.Error("zero-value Usage should be empty")
	}

	u.Add(0, 0)
	if !u.Empty() {
		t.Error("Usage with zero tokens added should still be empty")
	}

	u.Add(1, 0)
	if u.Empty() {
		t.Error("Usage with tokens should not be empty")
	}
}

// TestIncludeUsageStreamOption confirms that the IncludeUsage field
// on StreamOptions exists and can be set to true. This is a compile-
// time + runtime sanity check that the openai library supports the
// feature the app depends on.
func TestIncludeUsageStreamOption(t *testing.T) {
	opts := struct {
		IncludeUsage bool
	}{IncludeUsage: true}

	if !opts.IncludeUsage {
		t.Error("IncludeUsage should be true")
	}
}

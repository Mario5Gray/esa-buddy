// Package token tracks LLM token consumption across conversations.
//
// Token counts arrive in the final chunk of a streaming response when
// StreamOptions.IncludeUsage is enabled. This package accumulates those
// counts so they can be persisted alongside conversation history and
// surfaced in usage statistics.
package token

// Usage holds cumulative token counts for a conversation.
// Serialized into conversation history JSON for persistence.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Add accumulates tokens from a single LLM response into the running total.
func (u *Usage) Add(prompt, completion int) {
	u.PromptTokens += prompt
	u.CompletionTokens += completion
	u.TotalTokens += prompt + completion
}

// Empty reports whether any tokens have been tracked.
func (u Usage) Empty() bool {
	return u.TotalTokens == 0
}

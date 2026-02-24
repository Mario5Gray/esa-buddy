package history

import (
	"github.com/meain/esa/internal/token"
	"github.com/sashabaranov/go-openai"
)

// SchemaVersionCurrent identifies the latest history schema version.
const SchemaVersionCurrent = 1

type ConversationHistory struct {
	SchemaVersion int                            `json:"schema_version"`
	Commit        string                         `json:"commit,omitempty"`
	AgentPath     string                         `json:"agent_path"`
	Model         string                         `json:"model"`
	Messages      []openai.ChatCompletionMessage `json:"messages"`
	MessageMeta   []HistoryMessageMeta           `json:"message_meta,omitempty"`
	// Compaction.Summary is trusted metadata stored out-of-band (not derived from message content).
	Compaction *CompactionMeta `json:"compaction,omitempty"`
	Usage      *token.Usage    `json:"usage,omitempty"` // nil in history files from before token tracking
}

type HistoryMessageMeta struct {
	ID    string `json:"id"`
	Model string `json:"model,omitempty"`
	Role  string `json:"role,omitempty"`
}

type CompactionMeta struct {
	Enabled            bool   `json:"enabled"`
	MaxMessages        int    `json:"max_messages"`
	KeepLast           int    `json:"keep_last"`
	MaxChars           int    `json:"max_chars"`
	RedactionPolicy    string `json:"redaction_policy,omitempty"`
	Summary            string `json:"summary,omitempty"`
	LastTrigger        string `json:"last_trigger,omitempty"`
	LastMsgCount       int    `json:"last_msg_count,omitempty"`
	LastCharCount      int    `json:"last_char_count,omitempty"`
	LastTokenEstimate  int    `json:"last_token_estimate,omitempty"`
	LastTokenThreshold int    `json:"last_token_threshold,omitempty"`
}

func EnsureMessageMeta(existing []HistoryMessageMeta, messages []openai.ChatCompletionMessage, modelString string, idFn func() string) []HistoryMessageMeta {
	if existing == nil {
		existing = make([]HistoryMessageMeta, 0, len(messages))
	}
	// Trim excess if messages were truncated (e.g., retry mode).
	if len(existing) > len(messages) {
		existing = existing[:len(messages)]
	}

	for len(existing) < len(messages) {
		msg := messages[len(existing)]
		existing = append(existing, HistoryMessageMeta{
			ID:    idFn(),
			Model: modelString,
			Role:  msg.Role,
		})
	}

	// Fill missing model/role on existing entries.
	for i := range existing {
		if existing[i].Model == "" {
			existing[i].Model = modelString
		}
		if existing[i].Role == "" && i < len(messages) {
			existing[i].Role = messages[i].Role
		}
	}
	return existing
}

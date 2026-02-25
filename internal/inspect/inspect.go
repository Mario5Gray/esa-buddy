package inspect

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/meain/esa/internal/conversation/history"
)

// Tape represents a linear view of a conversation from epoch to head.
type Tape struct {
	ConversationID string           `json:"conversation_id,omitempty"`
	AgentPath      string           `json:"agent_path,omitempty"`
	Model          string           `json:"model,omitempty"`
	StartTime      time.Time        `json:"start_time,omitempty"`
	Messages       []historyMessage `json:"messages,omitempty"`
	Summary        string           `json:"summary,omitempty"`
	SchemaVersion  int              `json:"schema_version,omitempty"`
	Commit         string           `json:"commit,omitempty"`
}

type historyMessage struct {
	Role    string `json:"role"`
	Name    string `json:"name,omitempty"`
	Content string `json:"content,omitempty"`
}

// BuildTape constructs a tape from stored history.
func BuildTape(filePath string, hist history.ConversationHistory) Tape {
	conversationID, startTime := parseHistoryFileName(filePath)
	messages := make([]historyMessage, 0, len(hist.Messages))
	for _, msg := range hist.Messages {
		messages = append(messages, historyMessage{
			Role:    msg.Role,
			Name:    msg.Name,
			Content: msg.Content,
		})
	}
	summary := ""
	if hist.Compaction != nil {
		summary = hist.Compaction.Summary
	}
	return Tape{
		ConversationID: conversationID,
		AgentPath:      hist.AgentPath,
		Model:          hist.Model,
		StartTime:      startTime,
		Messages:       messages,
		Summary:        summary,
		SchemaVersion:  hist.SchemaVersion,
		Commit:         hist.Commit,
	}
}

func parseHistoryFileName(filePath string) (string, time.Time) {
	name := strings.TrimSuffix(filepath.Base(filePath), ".json")
	parts := strings.SplitN(name, "-", 5)
	if len(parts) != 5 {
		return "", time.Time{}
	}
	conversationID := parts[0]
	timestampStr := parts[4]
	startTime, _ := time.Parse("20060102-150405", timestampStr)
	return conversationID, startTime
}

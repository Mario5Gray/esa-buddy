package inspect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// Renderer is a Strategy (GoF) for formatting a tape.
type Renderer interface {
	Render(tape Tape) (string, error)
}

type TextRenderer struct{}

func (TextRenderer) Render(tape Tape) (string, error) {
	var b bytes.Buffer
	b.WriteString("Conversation Tape\n")
	if tape.ConversationID != "" {
		b.WriteString(fmt.Sprintf("ID: %s\n", tape.ConversationID))
	}
	if tape.AgentPath != "" {
		b.WriteString(fmt.Sprintf("Agent: %s\n", tape.AgentPath))
	}
	if tape.Model != "" {
		b.WriteString(fmt.Sprintf("Model: %s\n", tape.Model))
	}
	if !tape.StartTime.IsZero() {
		b.WriteString(fmt.Sprintf("Start: %s\n", tape.StartTime.Format(time.RFC3339)))
	}
	b.WriteString(fmt.Sprintf("Messages: %d\n", len(tape.Messages)))

	if tape.Summary != "" {
		b.WriteString("\n[Compaction Summary]\n")
		b.WriteString(tape.Summary)
		b.WriteString("\n")
	}

	for i, msg := range tape.Messages {
		b.WriteString(fmt.Sprintf("\n[%d] %s", i+1, msg.Role))
		if msg.Name != "" {
			b.WriteString(fmt.Sprintf(" (%s)", msg.Name))
		}
		b.WriteString("\n")
		if msg.Content != "" {
			b.WriteString(msg.Content)
			b.WriteString("\n")
		}
	}
	return b.String(), nil
}

type JSONRenderer struct{}

func (JSONRenderer) Render(tape Tape) (string, error) {
	payload, err := json.MarshalIndent(tape, "", "  ")
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

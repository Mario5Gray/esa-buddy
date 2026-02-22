package conversation

import (
	"context"
	"fmt"
	"strings"

	"github.com/meain/esa/internal/config"
	"github.com/meain/esa/internal/redaction"
	"github.com/sashabaranov/go-openai"
)

const (
	defaultCompactionMaxMessages = 40
	defaultCompactionKeepLast    = 12
	defaultCompactionMaxChars    = 20000
)

func normalizeCompactionSettings(settings config.Settings) (enabled bool, maxMsgs int, keepLast int, maxChars int, redactionPolicy string) {
	enabled = settings.PromptCompaction
	maxMsgs = settings.CompactionMaxMessages
	keepLast = settings.CompactionKeepLast
	maxChars = settings.CompactionMaxChars
	redactionPolicy = strings.TrimSpace(settings.CompactionRedaction)
	if redactionPolicy == "" {
		redactionPolicy = redaction.PolicyNone
	}

	if maxMsgs <= 0 {
		maxMsgs = defaultCompactionMaxMessages
	}
	if keepLast <= 0 {
		keepLast = defaultCompactionKeepLast
	}
	if maxChars <= 0 {
		maxChars = defaultCompactionMaxChars
	}

	if keepLast >= maxMsgs {
		keepLast = maxMsgs / 2
		if keepLast < 1 {
			keepLast = 1
		}
	}

	return enabled, maxMsgs, keepLast, maxChars, redactionPolicy
}

func (app *Application) compactMessagesIfNeeded() error {
	if !app.compactPrompt {
		return nil
	}

	if len(app.messages) <= 1 {
		return nil
	}

	msgCount := len(app.messages)
	charCount := messagesSize(app.messages)
	tokenEstimate := app.estimateTokens(buildTokenEstimateInput(app.messages))

	if msgCount <= app.compactMaxMsgs && charCount <= app.compactMaxChars {
		return nil
	}

	systemMsg, toSummarize, tail, existingSummary, ok := splitMessagesForCompaction(app.messages, app.compactKeepLast)
	if !ok || len(toSummarize) == 0 {
		return nil
	}

	if existingSummary == "" {
		existingSummary = app.compactionSummary
	}
	summaryInput := buildCompactionInput(existingSummary, toSummarize)
	if app.compactionRedactor != nil {
		redacted, err := app.compactionRedactor.Redact(summaryInput)
		if err != nil {
			return err
		}
		summaryInput = redacted
	}
	summary, err := app.summarizeConversation(summaryInput)
	if err != nil {
		return err
	}
	summary = strings.TrimSpace(summary)
	if app.compactionRedactor != nil {
		redacted, err := app.compactionRedactor.Redact(summary)
		if err != nil {
			return err
		}
		summary = redacted
	}

	app.compactionSummary = summary
	app.messages = append([]openai.ChatCompletionMessage{systemMsg}, tail...)
	app.debugPrint("Compaction", fmt.Sprintf("Compacted %d messages into summary", len(toSummarize)))
	app.lastCompactionTrigger = "threshold_exceeded"
	app.lastCompactionMsgCount = msgCount
	app.lastCompactionCharCount = charCount
	app.lastCompactionTokenEstimate = tokenEstimate
	return nil
}

func (app *Application) summarizeConversation(input string) (string, error) {
	ctx := context.Background()
	modelStr := app.resolveModelString("summarize", "")
	client, err := app.clientForModel(modelStr)
	if err != nil {
		return "", err
	}
	system := "Summarize the conversation for future context. Preserve decisions, constraints, file paths, commands, names, and open tasks. Be concise and factual."
	req := openai.ChatCompletionRequest{
		Model: modelStr,
		Messages: []openai.ChatCompletionMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: input},
		},
		Temperature: 0.2,
	}

	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("empty summary response")
	}
	return resp.Choices[0].Message.Content, nil
}

func splitMessagesForCompaction(messages []openai.ChatCompletionMessage, keepLast int) (system openai.ChatCompletionMessage, toSummarize []openai.ChatCompletionMessage, tail []openai.ChatCompletionMessage, existingSummary string, ok bool) {
	if len(messages) == 0 {
		return system, nil, nil, "", false
	}
	if messages[0].Role != "system" {
		return system, nil, nil, "", false
	}

	system = messages[0]
	rest := messages[1:]

	if len(rest) <= keepLast {
		return system, nil, rest, existingSummary, true
	}

	cut := len(rest) - keepLast
	toSummarize = rest[:cut]
	tail = rest[cut:]
	return system, toSummarize, tail, existingSummary, true
}

func buildCompactionInput(existingSummary string, messages []openai.ChatCompletionMessage) string {
	var b strings.Builder
	if existingSummary != "" {
		b.WriteString("Previous summary:\n")
		b.WriteString(existingSummary)
		b.WriteString("\n\n")
	}
	b.WriteString("Conversation to summarize:\n")
	for _, msg := range messages {
		line := formatMessageForSummary(msg)
		if line == "" {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func formatMessageForSummary(msg openai.ChatCompletionMessage) string {
	role := strings.ToUpper(msg.Role)

	var b strings.Builder
	b.WriteString("[")
	b.WriteString(role)
	b.WriteString("] ")

	if msg.Name != "" {
		b.WriteString(msg.Name)
		b.WriteString(": ")
	}

	if msg.Role == "tool" {
		// Avoid summarizing tool outputs verbatim; keep a compact marker.
		if msg.Name != "" {
			b.WriteString(msg.Name)
			b.WriteString(" ")
		}
		if msg.ToolCallID != "" {
			b.WriteString("(tool_call_id=")
			b.WriteString(msg.ToolCallID)
			b.WriteString(") ")
		}
		if msg.Content != "" {
			b.WriteString("[output omitted, ")
			b.WriteString(fmt.Sprintf("%d chars]", len(msg.Content)))
		} else {
			b.WriteString("[output omitted]")
		}
		return strings.TrimSpace(b.String())
	}

	if msg.Content != "" {
		b.WriteString(msg.Content)
	}

	if msg.FunctionCall != nil {
		b.WriteString(" FunctionCall: ")
		b.WriteString(msg.FunctionCall.Name)
		if msg.FunctionCall.Arguments != "" {
			b.WriteString(" ")
			b.WriteString(msg.FunctionCall.Arguments)
		}
	}

	if len(msg.ToolCalls) > 0 {
		if msg.Content != "" {
			b.WriteString(" ")
		}
		b.WriteString("ToolCalls: ")
		for i, call := range msg.ToolCalls {
			if i > 0 {
				b.WriteString("; ")
			}
			b.WriteString(call.Function.Name)
			if call.Function.Arguments != "" {
				b.WriteString(" ")
				b.WriteString(call.Function.Arguments)
			}
		}
	}

	if msg.Role == "tool" && msg.ToolCallID != "" {
		if msg.Content != "" {
			b.WriteString(" ")
		}
		b.WriteString("(tool_call_id=")
		b.WriteString(msg.ToolCallID)
		b.WriteString(")")
	}

	return strings.TrimSpace(b.String())
}

func messagesSize(messages []openai.ChatCompletionMessage) int {
	total := 0
	for _, msg := range messages {
		total += messageSize(msg)
	}
	return total
}

func (app *Application) estimateTokens(text string) int {
	if text == "" {
		return 0
	}

	provider, model, _ := app.parseModel()
	if app.counterProvider != nil {
		if counter := app.counterProvider.CounterFor(provider); counter != nil {
			if n := counter.Estimate(text, model); n > 0 {
				return n
			}
		}
		app.debugPrint("Token Estimate",
			fmt.Sprintf("No token counter matched provider=%q model=%q; using 0", provider, model))
	}
	return 0
}

func buildTokenEstimateInput(messages []openai.ChatCompletionMessage) string {
	var b strings.Builder
	for _, msg := range messages {
		line := formatMessageForSummary(msg)
		if line == "" {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func messageSize(msg openai.ChatCompletionMessage) int {
	size := len(msg.Role) + len(msg.Content) + len(msg.Name) + len(msg.ToolCallID)
	if msg.FunctionCall != nil {
		size += len(msg.FunctionCall.Name) + len(msg.FunctionCall.Arguments)
	}
	for _, call := range msg.ToolCalls {
		size += len(call.Function.Name) + len(call.Function.Arguments)
		size += len(call.ID) + len(call.Type)
	}
	return size
}

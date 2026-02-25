package conversation

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/meain/esa/internal/llm"
	"github.com/sashabaranov/go-openai"
)


// filterThinkTags strips <think>...</think> blocks from streamed content.
// It handles tags split across chunk boundaries using a simple state machine.
// inThink tracks whether we're inside a think block; buf accumulates partial
// tag matches that may span chunks.
func filterThinkTags(chunk string, inThink *bool, buf *strings.Builder) string {
	const openTag = "<think>"
	const closeTag = "</think>"

	var out strings.Builder
	buf.WriteString(chunk)
	s := buf.String()
	buf.Reset()

	for len(s) > 0 {
		if *inThink {
			// Look for closing tag
			idx := strings.Index(s, closeTag)
			if idx >= 0 {
				*inThink = false
				s = s[idx+len(closeTag):]
				// Skip leading newline after closing tag
				if len(s) > 0 && s[0] == '\n' {
					s = s[1:]
				}
			} else {
				// Check if the end of s could be a partial </think>
				for i := 1; i < len(closeTag) && i <= len(s); i++ {
					if strings.HasSuffix(s, closeTag[:i]) {
						buf.WriteString(s[len(s)-i:])
						s = s[:len(s)-i]
						break
					}
				}
				// Discard everything before the potential partial match (it's inside think)
				s = ""
			}
		} else {
			// Look for opening tag
			idx := strings.Index(s, openTag)
			if idx >= 0 {
				out.WriteString(s[:idx])
				*inThink = true
				s = s[idx+len(openTag):]
			} else {
				// Check if the end of s could be a partial <think>
				buffered := false
				for i := 1; i < len(openTag) && i <= len(s); i++ {
					if strings.HasSuffix(s, openTag[:i]) {
						out.WriteString(s[:len(s)-i])
						buf.WriteString(s[len(s)-i:])
						buffered = true
						break
					}
				}
				if !buffered {
					out.WriteString(s)
				}
				s = ""
			}
		}
	}

	return out.String()
}

func (app *Application) handleStreamResponse(stream llm.Stream) (openai.ChatCompletionMessage, error) {
	defer stream.Close()

	var assistantMsg openai.ChatCompletionMessage
	var fullContent strings.Builder
	hasContent := false

	// State machine for filtering <think>...</think> blocks from streamed content.
	// Chunks may split tags arbitrarily, so we buffer partial matches.
	inThink := false
	var thinkBuf strings.Builder

	for {
		response, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return openai.ChatCompletionMessage{}, fmt.Errorf("stream error: %w", err)
		}

		// The final stream chunk carries token usage for the entire request.
		// Earlier chunks have Usage: nil; only the last one is populated.
		if response.Usage != nil {
			app.usage.Add(response.Usage.PromptTokens, response.Usage.CompletionTokens)
		}

		if len(response.Choices) == 0 {
			continue
		}

		if response.Choices[0].Delta.ToolCalls != nil {
			for _, toolCall := range response.Choices[0].Delta.ToolCalls {
				if toolCall.ID != "" {
					assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, toolCall)
				} else {
					lastToolCall := assistantMsg.ToolCalls[len(assistantMsg.ToolCalls)-1]
					lastToolCall.Function.Arguments += toolCall.Function.Arguments
					assistantMsg.ToolCalls[len(assistantMsg.ToolCalls)-1] = lastToolCall
				}
			}
		} else {
			// Clear progress line before showing result
			if app.showProgress && app.lastProgressLen > 0 {
				fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", app.lastProgressLen))
				app.lastProgressLen = 0
			}

			content := response.Choices[0].Delta.Content
			if content != "" {
				content = filterThinkTags(content, &inThink, &thinkBuf)
				if content != "" {
					hasContent = true
					if !app.prettyOutput {
						fmt.Fprint(app.out, content)
					}
					fullContent.WriteString(content)
				}
			}
		}
	}

	// Flush any remaining buffered content that turned out not to be a think tag
	if thinkBuf.Len() > 0 && !inThink {
		remaining := thinkBuf.String()
		if remaining != "" {
			hasContent = true
			if !app.prettyOutput {
				fmt.Fprint(app.out, remaining)
			}
			fullContent.WriteString(remaining)
		}
	}

	if hasContent {
		if app.prettyOutput {
			// TODO: Add support for rendering pretty markdown in a
			// streming manner (charmbracelet/glow/issues/601)
			PrintPrettyOutput(fullContent.String())
		} else {
			fmt.Fprintln(app.out)
		}
	}

	assistantMsg.Role = "assistant"
	assistantMsg.Content = fullContent.String()
	return assistantMsg, nil
}

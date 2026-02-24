package message

import (
	"fmt"

	"github.com/sashabaranov/go-openai"
)

// OnlyFor returns a Decorator (GoF, Structural) that applies t only when the
// message role matches one of the given roles. All other roles pass through
// unchanged. Use this when a transform has a specific, bounded scope —
// for example, a transform that only makes sense for tool output.
//
// Example:
//
//	watermark := OnlyFor("tool")(func(m openai.ChatCompletionMessage) openai.ChatCompletionMessage {
//	    m.Content = "[tool] " + m.Content
//	    return m
//	})
func OnlyFor(roles ...string) func(Transform) Transform {
	return func(t Transform) Transform {
		return func(msg openai.ChatCompletionMessage) openai.ChatCompletionMessage {
			for _, r := range roles {
				if msg.Role == r {
					return t(msg)
				}
			}
			return msg
		}
	}
}

// SkipFor returns a Decorator (GoF, Structural) that bypasses t when the
// message role matches any of the given roles, applying t to all others.
// Use this to protect specific roles from a broadly-applicable transform —
// for example, keeping system messages (trusted, author-controlled) out of
// a data-layer pipeline intended for untrusted content.
//
// Example:
//
//	safe := SkipFor("system", "assistant")(redactSecrets)
func SkipFor(roles ...string) func(Transform) Transform {
	return func(t Transform) Transform {
		return func(msg openai.ChatCompletionMessage) openai.ChatCompletionMessage {
			for _, r := range roles {
				if msg.Role == r {
					return msg
				}
			}
			return t(msg)
		}
	}
}

// Envelope is a Transform that wraps tool-role message content in a
// structured <tool_data> tag, making the trust boundary between raw external
// data and trusted instructions explicit in the LLM context.
//
// It is defined using OnlyFor("tool") so its scope is self-documenting:
// user, assistant, and system messages pass through unchanged. Only tool
// results — the primary injection surface — are enveloped.
//
// The source attribute names the tool that produced the output, providing
// an audit trail and preventing the payload from spoofing the envelope header.
var Envelope Transform = OnlyFor("tool")(func(msg openai.ChatCompletionMessage) openai.ChatCompletionMessage {
	msg.Content = fmt.Sprintf("<tool_data source=%q>\n%s\n</tool_data>", msg.Name, msg.Content)
	return msg
})

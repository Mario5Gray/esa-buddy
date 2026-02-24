// Package message provides a fluent Builder for constructing
// openai.ChatCompletionMessage values with an explicit transform pipeline.
//
// Every message entering the LLM context should be constructed here so that
// trust boundaries, enveloping, and redaction can be applied uniformly.
// This is a Pipes and Filters pattern (EIP, Hohpe §3) expressed as a
// GoF Builder — each call to Apply adds a filter stage; Build runs them
// in registration order and materialises the final message.
//
// Usage:
//
//	msg := message.New("tool", result).
//	    WithName(toolName).
//	    WithToolCallID(callID).
//	    Apply(message.Envelope).
//	    Apply(redactSecrets).
//	    Build()
package message

import "github.com/sashabaranov/go-openai"

// Transform is a function that receives a message in its current state and
// returns a new state. Transforms must not mutate the input — return a
// modified copy. They are applied in the order they are registered via Apply.
type Transform func(openai.ChatCompletionMessage) openai.ChatCompletionMessage

// Builder constructs a ChatCompletionMessage through an ordered pipeline of
// Transform functions. The zero value is not useful; use New to construct one.
type Builder struct {
	msg        openai.ChatCompletionMessage
	transforms []Transform
}

// New creates a Builder for a message with the given role and content.
// Role must be one of: "system", "user", "assistant", "tool".
func New(role, content string) *Builder {
	return &Builder{
		msg: openai.ChatCompletionMessage{
			Role:    role,
			Content: content,
		},
	}
}

// WithName sets the Name field. Required for tool-role messages to identify
// which tool produced this result.
func (b *Builder) WithName(name string) *Builder {
	b.msg.Name = name
	return b
}

// WithToolCallID sets the ToolCallID field. Required for tool-role messages
// to correlate the result with the originating tool call from the assistant.
func (b *Builder) WithToolCallID(id string) *Builder {
	b.msg.ToolCallID = id
	return b
}

// WithToolCalls sets the ToolCalls field on an assistant message, recording
// which tools the assistant has decided to invoke in this turn.
func (b *Builder) WithToolCalls(calls []openai.ToolCall) *Builder {
	b.msg.ToolCalls = calls
	return b
}

// Apply adds a Transform to the pipeline. Transforms run in registration
// order when Build is called. Calling Apply after Build has no effect on
// the already-built message.
func (b *Builder) Apply(t Transform) *Builder {
	b.transforms = append(b.transforms, t)
	return b
}

// Build runs all registered transforms in order and returns the final
// ChatCompletionMessage. Each transform receives the output of the previous.
func (b *Builder) Build() openai.ChatCompletionMessage {
	msg := b.msg
	for _, t := range b.transforms {
		msg = t(msg)
	}
	return msg
}

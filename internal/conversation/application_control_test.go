package conversation

import (
	"testing"

	"github.com/sashabaranov/go-openai"
)

// appWithMessages creates a bare Application with the given messages for testing.
func appWithMessages(msgs ...openai.ChatCompletionMessage) *Application {
	app := &Application{
		messages: msgs,
	}
	app.debugPrint = func(string, ...any) {}
	return app
}

func msg(role, content string) openai.ChatCompletionMessage {
	return openai.ChatCompletionMessage{Role: role, Content: content}
}

// --- ClearMessages -----------------------------------------------------------

func TestClearMessages_resets_to_system_only(t *testing.T) {
	app := appWithMessages(
		msg("system", "sys"),
		msg("user", "a"),
		msg("assistant", "b"),
		msg("user", "c"),
		msg("assistant", "d"),
	)

	app.ClearMessages()

	if len(app.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(app.messages))
	}
	if app.messages[0].Role != "system" {
		t.Errorf("expected system role, got %q", app.messages[0].Role)
	}
}

func TestClearMessages_noop_when_no_messages(t *testing.T) {
	app := appWithMessages()

	app.ClearMessages() // must not panic

	if len(app.messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(app.messages))
	}
}

// --- UndoLastExchange --------------------------------------------------------

func TestUndoLastExchange_removes_last_user_and_assistant_pair(t *testing.T) {
	app := appWithMessages(
		msg("system", "sys"),
		msg("user", "a"),
		msg("assistant", "b"),
		msg("user", "c"),
		msg("assistant", "d"),
	)

	ok := app.UndoLastExchange()

	if !ok {
		t.Fatal("expected UndoLastExchange to return true")
	}
	if len(app.messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(app.messages))
	}
	if app.messages[1].Content != "a" || app.messages[2].Content != "b" {
		t.Errorf("unexpected messages after undo: %v", app.messages[1:])
	}
}

func TestUndoLastExchange_returns_false_when_only_system_message(t *testing.T) {
	app := appWithMessages(msg("system", "sys"))

	ok := app.UndoLastExchange()

	if ok {
		t.Fatal("expected UndoLastExchange to return false")
	}
	if len(app.messages) != 1 {
		t.Errorf("messages should be unchanged, got %d", len(app.messages))
	}
}

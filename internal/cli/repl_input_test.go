package cli

import (
	"strings"
	"testing"
)

func TestJoinLines_single_line_unchanged(t *testing.T) {
	result := joinLines([]string{"hello world"})
	if result != "hello world" {
		t.Errorf("expected %q, got %q", "hello world", result)
	}
}

func TestJoinLines_joins_two_lines(t *testing.T) {
	result := joinLines([]string{"hello \\", "world"})
	if result != "hello \nworld" {
		t.Errorf("expected %q, got %q", "hello \nworld", result)
	}
}

func TestJoinLines_strips_trailing_backslash(t *testing.T) {
	result := joinLines([]string{"hello \\", "world"})
	if strings.Contains(result, "\\") {
		t.Errorf("result must not contain trailing backslash, got %q", result)
	}
}

func TestJoinLines_no_join_on_backslash_in_middle(t *testing.T) {
	// A backslash that is not at the end of the line should not trigger joining.
	// "he\\llo" has a backslash in the middle, not trailing.
	result := joinLines([]string{"he\\llo"})
	if result != "he\\llo" {
		t.Errorf("expected %q, got %q", "he\\llo", result)
	}
}

package redaction

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMarkerRegexRedaction(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "esa.redaction.toml")
	config := `
[paths]
include = ["**/*"]

[[rules]]
name = "marker"
type = "marker"
open = "\\[foo"
close = "foo\\]"
replacement = "[REDACTED]"
scope = ["**/*.md"]

[[rules]]
name = "regex"
type = "regex"
pattern = "secret=[A-Za-z0-9_]+"
replacement = "[REDACTED]"
scope = ["**/*"]
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	policy, _, err := BuildPolicy(Config{Kind: KindMarkerRegex, ConfigFile: configPath}, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	input := "hello [foo secret foo] end secret=abc"
	output, err := policy.Redact(Context{ResourcePath: "notes/test.md"}, input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	expected := "hello [REDACTED] end [REDACTED]"
	if output != expected {
		t.Fatalf("expected %q, got %q", expected, output)
	}
}

func TestMarkerRegexRespectsScope(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "esa.redaction.toml")
	config := `
[paths]
include = ["**/*"]

[[rules]]
name = "marker"
type = "marker"
open = "\\[foo"
close = "foo\\]"
replacement = "[REDACTED]"
scope = ["**/*.md"]
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	policy, _, err := BuildPolicy(Config{Kind: KindMarkerRegex, ConfigFile: configPath}, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	input := "hello [foo secret foo] end"
	output, err := policy.Redact(Context{ResourcePath: "notes/test.txt"}, input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if output != input {
		t.Fatalf("expected no redaction outside scope, got %q", output)
	}
}

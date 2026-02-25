package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixture is a minimal markdown document with one scm fenced code block
// containing parens, a function-name symbol, and two @capture variables.
const scmFixture = `# Test

` + "```scm" + `
(call_expression
  function: (selector_expression
    operand: (_) @recv
    field: (field_identifier) @method))
` + "```" + `
`

// TestSCMBlockTokenClasses verifies that a fenced scm block is syntax-
// highlighted by chroma: the output must contain the chroma wrapper class,
// at least one name-variable span (@captures), and at least one punctuation
// span (parentheses).
func TestSCMBlockTokenClasses(t *testing.T) {
	src := writeFixture(t, "single.md", scmFixture)
	dst := filepath.Join(t.TempDir(), "out.html")

	css, err := buildCSS()
	if err != nil {
		t.Fatalf("buildCSS: %v", err)
	}
	if err := convertFile(buildMarkdown(), css, src, dst); err != nil {
		t.Fatalf("convertFile: %v", err)
	}

	out := readFile(t, dst)

	if !strings.Contains(out, `class="chroma"`) {
		t.Error("expected chroma wrapper class, not found")
	}
	if !strings.Contains(out, `class="nv"`) {
		t.Error("expected name-variable spans for @captures, not found")
	}
	if !strings.Contains(out, `class="p"`) {
		t.Error("expected punctuation spans for parentheses, not found")
	}
}

// multiBlockFixture builds a markdown document with n scm fenced code blocks.
func multiBlockFixture(n int) string {
	block := "```scm\n(call_expression\n  operand: (_) @recv)\n```\n\n"
	var b bytes.Buffer
	b.WriteString("# Blocks\n\n")
	for range n {
		b.WriteString(block)
	}
	return b.String()
}

// TestSCMBlockCountPreserved verifies that every scm block in the source
// document produces exactly one chroma-highlighted block in the output — no
// blocks are dropped or merged during conversion.
func TestSCMBlockCountPreserved(t *testing.T) {
	const want = 5
	src := writeFixture(t, "multi.md", multiBlockFixture(want))
	dst := filepath.Join(t.TempDir(), "out.html")

	css, err := buildCSS()
	if err != nil {
		t.Fatalf("buildCSS: %v", err)
	}
	if err := convertFile(buildMarkdown(), css, src, dst); err != nil {
		t.Fatalf("convertFile: %v", err)
	}

	out := readFile(t, dst)
	got := strings.Count(out, `class="chroma"`)
	if got != want {
		t.Errorf("chroma block count = %d, want %d", got, want)
	}
}

// writeFixture writes content to a temp file and returns its path.
func writeFixture(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

// readFile reads a file and returns its content as a string.
func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

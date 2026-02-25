# E2E Test Scenario 7: Page Shell Structure

**Status:** Implemented (`cmd/docgen/main_test.go` — `TestPageShellStructure`)
**Tool:** `docgen`
**Test type:** In-process (calls `convertFile` directly) or subprocess

## Purpose

Verify that the HTML output produced by `docgen` has the correct full-page structure: doctype declaration, title matching the filename stem, inline chroma CSS, and a closing `</html>` tag.

This is a regression guard on the `pageTmpl` template in `main.go`.

## Input

Any valid `.md` file. A minimal fixture such as `# Hello` is sufficient.

For maximum coverage, use a filename with a meaningful stem, e.g. `my-document.md`, so the title assertion is meaningful.

## Steps

1. Write a fixture file named `my-document.md` to a temp directory.
2. Call `convertFile(buildMarkdown(), css, src, dst)` (or run as subprocess).
3. Read the output HTML.

## Assertions

| Assertion | Expected |
|-----------|----------|
| Doctype present | Output contains `<!DOCTYPE html>` |
| Title tag matches stem | `<title>my-document</title>` (filename without `.md`) |
| Inline style block present | Output contains `<style>` |
| Chroma CSS inside style | Style block contains `.chroma` |
| Document closed | Output contains `</html>` |

## Notes

- The title is derived by `strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))` in `convertFile`.
- The CSS is inlined via the `{{ .CSS }}` template field; presence of `.chroma` confirms the chroma stylesheet was written.
- Can be implemented as an in-process test (like scenarios 1–2) since `convertFile` is an exported-friendly internal function — no `os.Exit` involved.

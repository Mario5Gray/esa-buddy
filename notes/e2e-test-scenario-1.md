# E2E Test Scenario 1: SCM Block Token Classes

**Status:** Implemented (`cmd/docgen/main_test.go` — `TestSCMBlockTokenClasses`)
**Tool:** `docgen`
**Test type:** In-process (calls `convertFile` directly)

## Purpose

Verify that a fenced `scm` code block is syntax-highlighted by chroma with the correct token-class spans.

## Input

A minimal markdown file containing one fenced `scm` block with:
- parentheses
- a function-name selector expression
- two `@capture` variable references

```md
# Test

```scm
(call_expression
  function: (selector_expression
    operand: (_) @recv
    field: (field_identifier) @method))
```
```

## Steps

1. Write the fixture to a temp file.
2. Build CSS via `buildCSS()`.
3. Call `convertFile(buildMarkdown(), css, src, dst)`.
4. Read the output HTML.

## Assertions

| Assertion | Expected |
|-----------|----------|
| Chroma wrapper present | `class="chroma"` found in output |
| Name-variable spans present | `class="nv"` found (covers `@recv`, `@method`) |
| Punctuation spans present | `class="p"` found (covers parentheses) |

## Notes

- This is a compile-time + runtime check that goldmark-highlighting correctly classifies SCM token types.
- No subprocess needed; calls internal functions directly.

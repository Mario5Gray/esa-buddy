# E2E Test Scenario 2: SCM Block Count Preserved

**Status:** Implemented (`cmd/docgen/main_test.go` — `TestSCMBlockCountPreserved`)
**Tool:** `docgen`
**Test type:** In-process (calls `convertFile` directly)

## Purpose

Verify that every `scm` fenced block in the source document produces exactly one chroma-highlighted block in the output — no blocks are dropped or merged.

## Input

A markdown file with N=5 identical fenced `scm` blocks:

```md
# Blocks

```scm
(call_expression
  operand: (_) @recv)
```

... (repeated 5 times)
```

## Steps

1. Generate the fixture with `multiBlockFixture(5)`.
2. Write to a temp file.
3. Build CSS via `buildCSS()`.
4. Call `convertFile(buildMarkdown(), css, src, dst)`.
5. Read the output HTML.
6. Count occurrences of `class="chroma"`.

## Assertions

| Assertion | Expected |
|-----------|----------|
| Chroma block count | Exactly 5 (matches input block count) |

## Notes

- Guards against off-by-one errors in the goldmark block renderer.
- The count is deterministic: one `class="chroma"` wrapper per highlighted block.
- No subprocess needed; calls internal functions directly.

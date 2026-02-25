# E2E Test Scenario 5: No Arguments

**Status:** Implemented (`cmd/docgen/subprocess_test.go` — `TestNoArguments`)
**Tool:** `docgen`
**Test type:** Subprocess (`exec.Command`)

## Purpose

Verify that invoking `docgen` with no arguments prints a usage message to stderr and exits with a non-zero exit code.

## Input

No CLI arguments.

## Steps

1. Run: `docgen` (no arguments) via `exec.Command`.
2. Capture stdout and stderr separately.
3. Check exit code and stderr content.

## Assertions

| Assertion | Expected |
|-----------|----------|
| Exit code | Non-zero (1) |
| Stderr contains | `"usage:"` |
| Stdout | Empty (no output written) |

## Notes

- Must use a subprocess (`exec.Command`) because `main()` calls `os.Exit(1)`, which cannot be recovered in-process.
- The exact stderr message is: `usage: docgen <file.md> [file.md ...]`
- Assertion on stderr should be a prefix/contains check (`strings.Contains`) to remain stable if the message wording changes slightly.

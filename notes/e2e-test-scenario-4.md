# E2E Test Scenario 4: Multiple Files in One Invocation

**Status:** Implemented (`cmd/docgen/subprocess_test.go` — `TestMultipleFilesOneInvocation`)
**Tool:** `docgen`
**Test type:** Subprocess (`exec.Command`)

## Purpose

Verify that passing multiple input files in a single run produces one output file per input, in the correct location, and exits with code `0`.

## Input

Two files passed together — generic placeholders, any names work:
- `dir-a/first.md`
- `dir-b/second.md`

Both must exist on disk inside a temp working directory.

## Steps

1. Create a temp working directory (`t.TempDir()`).
2. Create `dir-a/first.md` and `dir-b/second.md` inside the temp dir with minimal markdown content.
3. Run: `docgen dir-a/first.md dir-b/second.md` with `cmd.Dir` set to the temp dir.
4. Check the filesystem.

## Assertions

| Assertion | Expected |
|-----------|----------|
| Exit code | `0` |
| First output file exists | `site/dir-a/first.html` |
| Second output file exists | `site/dir-b/second.html` |
| `site/dir-a/` directory created | Yes |
| `site/dir-b/` directory created | Yes |

## Notes

- Validates that the loop in `main()` processes all arguments and that `os.MkdirAll` creates subdirectory segments for each file independently.
- Also confirms that `failed` is `false` (no errors) so the process exits `0`.

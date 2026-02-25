# E2E Test Scenario 4: Multiple Files in One Invocation

**Status:** Planned (not yet implemented)
**Tool:** `docgen`
**Test type:** Subprocess (`exec.Command`)

## Purpose

Verify that passing multiple input files in a single run produces one output file per input, in the correct location, and exits with code `0`.

## Input

Two files passed together:
- `docs/agents.md`
- `notes/Telemetry-Scope.md`

Both must exist on disk inside a temp working directory.

## Steps

1. Create a temp working directory (`t.TempDir()`).
2. Create `docs/agents.md` and `notes/Telemetry-Scope.md` inside the temp dir with minimal markdown content.
3. Run: `docgen docs/agents.md notes/Telemetry-Scope.md` with `cmd.Dir` set to the temp dir.
4. Check the filesystem.

## Assertions

| Assertion | Expected |
|-----------|----------|
| Exit code | `0` |
| First output file exists | `site/docs/agents.html` |
| Second output file exists | `site/notes/Telemetry-Scope.html` |
| `site/docs/` directory created | Yes |
| `site/notes/` directory created | Yes |

## Notes

- Validates that the loop in `main()` processes all arguments and that `os.MkdirAll` creates subdirectory segments for each file independently.
- Also confirms that `failed` is `false` (no errors) so the process exits `0`.

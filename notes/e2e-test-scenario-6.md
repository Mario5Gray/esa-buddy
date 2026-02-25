# E2E Test Scenario 6: Missing Input File

**Status:** Planned (not yet implemented)
**Tool:** `docgen`
**Test type:** Subprocess (`exec.Command`)

## Purpose

Verify that passing a path that does not exist on disk causes `docgen` to print an error to stderr and exit non-zero.

## Input

A path that does not exist: e.g. `nonexistent/file.md`

### Extended variant (recommended)

Mixed input: one valid file + one missing file.
Confirms that the tool processes the valid file (writes its output), reports the error for the missing one, and still exits non-zero.

## Steps (basic)

1. Run: `docgen nonexistent/file.md` via `exec.Command`.
2. Capture stdout and stderr.
3. Check exit code and stderr.

## Steps (extended variant)

1. Create a temp working directory.
2. Create `docs/agents.md` inside it with minimal content.
3. Run: `docgen docs/agents.md nonexistent/missing.md` with `cmd.Dir` = temp dir.
4. Check exit code, stderr, and filesystem.

## Assertions (basic)

| Assertion | Expected |
|-----------|----------|
| Exit code | Non-zero (1) |
| Stderr contains | `"error:"` |

## Assertions (extended variant)

| Assertion | Expected |
|-----------|----------|
| Exit code | Non-zero (1) — `failed=true` from the missing file |
| Stderr contains | `"error:"` |
| `site/docs/agents.html` exists | Yes — valid file is still processed |

## Notes

- The error prefix `"error:"` comes from `fmt.Fprintln(os.Stderr, "error:", err)` in `main()`.
- The `failed` flag accumulates across all files; a single missing file does not abort processing of valid ones.
- The extended variant is the more valuable test: it confirms graceful degradation, not hard abort.

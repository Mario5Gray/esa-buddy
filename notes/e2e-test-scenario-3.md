# E2E Test Scenario 3: Output Path Derivation

**Status:** Planned (not yet implemented)
**Tool:** `docgen`
**Test type:** Subprocess (`exec.Command`)

## Purpose

Verify that the output path mirrors the input file's directory segment under `site/`.

## Input

File: `notes/Telemetry-Scope.md` (passed as a CLI argument)

The file must exist on disk for this test. Use a temp directory as working directory to avoid polluting the real `site/` tree.

## Steps

1. Create a temp working directory (`t.TempDir()`).
2. Create `notes/Telemetry-Scope.md` inside that temp dir with minimal markdown content.
3. Run: `docgen notes/Telemetry-Scope.md` with the temp dir as the working directory.
4. Check the filesystem.

## Assertions

| Assertion | Expected |
|-----------|----------|
| Exit code | `0` |
| Output file exists | `site/notes/Telemetry-Scope.html` (relative to temp working dir) |
| `site/notes/` directory created | Yes (auto-created by `os.MkdirAll`) |

## Notes

- Tests the path-derivation logic in `main()`: `dst = filepath.Join("site", dir, base+".html")`.
- Must run as a subprocess because the tool uses the process working directory to compute output paths.
- Use `t.TempDir()` as the working directory (`cmd.Dir`) so the generated `site/` tree is isolated and cleaned up automatically.

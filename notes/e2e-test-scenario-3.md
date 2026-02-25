# E2E Test Scenario 3: Output Path Derivation

**Status:** Implemented (`cmd/docgen/subprocess_test.go` — `TestOutputPathDerivation`)
**Tool:** `docgen`
**Test type:** Subprocess (`exec.Command`)

## Purpose

Verify that the output path mirrors the input file's directory segment under `site/`.

## Input

File: `subdir/input.md` (passed as a CLI argument) — any name/directory works; these are generic placeholders.

Use a temp directory as working directory to avoid polluting the real `site/` tree.

## Steps

1. Create a temp working directory (`t.TempDir()`).
2. Create `subdir/input.md` inside that temp dir with minimal markdown content.
3. Run: `docgen subdir/input.md` with the temp dir as the working directory.
4. Check the filesystem.

## Assertions

| Assertion | Expected |
|-----------|----------|
| Exit code | `0` |
| Output file exists | `site/subdir/input.html` (relative to temp working dir) |
| `site/subdir/` directory created | Yes (auto-created by `os.MkdirAll`) |

## Notes

- Tests the path-derivation logic in `main()`: `dst = filepath.Join("site", dir, base+".html")`.
- Must run as a subprocess because the tool uses the process working directory to compute output paths.
- Use `t.TempDir()` as the working directory (`cmd.Dir`) so the generated `site/` tree is isolated and cleaned up automatically.

# docgen

`docgen` is a command-line tool that converts Markdown files into self-contained HTML pages with syntax-highlighted code blocks.

## Usage

```
docgen <file.md> [file.md ...]
```

Pass one or more Markdown files. Each file is converted to an HTML file written under `site/`, preserving the input's directory structure.

## Output paths

The output path is derived from the input path:

```
<input-dir>/<name>.md  →  site/<input-dir>/<name>.html
```

Examples:

| Input | Output |
|-------|--------|
| `notes/overview.md` | `site/notes/overview.html` |
| `docs/agents.md` | `site/docs/agents.html` |
| `README.md` | `site/README.html` |

Output directories are created automatically if they do not exist.

## Features

- **GFM support** — GitHub Flavored Markdown (tables, strikethrough, task lists, etc.)
- **Syntax highlighting** — fenced code blocks are highlighted via [chroma](https://github.com/alecthomas/chroma) using the Monokai style; CSS is inlined into each page
- **SCM / tree-sitter queries** — `scm` fenced blocks are highlighted with correct token classes for capture variables (`@name`) and punctuation
- **Self-contained output** — each HTML file includes all styles inline; no external assets required

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | All files converted successfully |
| `1` | No arguments provided, or one or more files failed to convert |

When a mix of valid and missing files is passed, docgen processes the valid ones, reports each failure to stderr with an `error:` prefix, and exits `1`.

## Error output

All errors and the usage message are written to stderr. Stdout only receives a `wrote <path>` confirmation line per successful file.

## Tests

The tool has a full e2e test suite in `cmd/docgen/`:

| Scenario | Test | What it covers |
|----------|------|----------------|
| 1 | `TestSCMBlockTokenClasses` | chroma token classes for `scm` blocks |
| 2 | `TestSCMBlockCountPreserved` | one highlighted block per input block, no drops |
| 3 | `TestOutputPathDerivation` | output path mirrors input directory segment |
| 4 | `TestMultipleFilesOneInvocation` | multiple inputs produce multiple outputs |
| 5 | `TestNoArguments` | no args exits non-zero with usage on stderr |
| 6 | `TestMissingFile` / `TestMissingFileMixedInput` | missing file exits non-zero; valid files still processed |
| 7 | `TestPageShellStructure` | output contains DOCTYPE, title, inline chroma CSS, closing tag |

Run the tests:

```
go test ./cmd/docgen/...
```

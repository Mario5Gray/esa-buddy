#!/usr/bin/env python3
import argparse
import os
import re
import shutil
import subprocess
import tempfile


BEGIN_MARK = "<!-- tree-sitter-results: begin -->"
END_MARK = "<!-- tree-sitter-results: end -->"


def find_go_files(root):
    files = []
    for base, dirs, filenames in os.walk(root):
        dirs[:] = [d for d in dirs if d not in {".git", "vendor", "node_modules", "dist", "build"}]
        for name in filenames:
            if name.endswith(".go"):
                files.append(os.path.join(base, name))
    return files


def run_query(query_text, paths, scope=None, ansi=False):
    if shutil.which("tree-sitter") is None:
        return None, "tree-sitter binary not found"

    with tempfile.TemporaryDirectory() as tmpdir:
        query_path = os.path.join(tmpdir, "query.scm")
        paths_path = os.path.join(tmpdir, "paths.txt")
        with open(query_path, "w", encoding="utf-8") as f:
            f.write(query_text)
        with open(paths_path, "w", encoding="utf-8") as f:
            for p in paths:
                f.write(p + "\n")

        cmd = ["tree-sitter", "query", query_path, "--paths", paths_path, "--captures"]
        if scope:
            cmd += ["--scope", scope]
        result = subprocess.run(cmd, capture_output=True, text=True)
        if result.returncode != 0:
            return None, result.stderr.strip() or "tree-sitter query failed"
        formatted = format_query_output(result.stdout, ansi=ansi)
        return formatted, None


def format_query_output(output, ansi=False):
    lines = output.splitlines()
    current_file = None
    matches = []

    for line in lines:
        stripped = line.strip()
        if stripped.endswith(".go") and not line.startswith(" "):
            current_file = stripped
            continue
        if "capture:" in line and "start:" in line:
            row = parse_row(line)
            text = parse_text(line)
            if row is not None and current_file is not None:
                matches.append({"file": current_file, "row": row, "text": text})

    snippets = []
    seen = set()
    for match in matches:
        file_path = match["file"]
        row = match["row"]
        text = match.get("text")
        key = (file_path, row)
        if key in seen:
            continue
        seen.add(key)
        snippet = extract_line(file_path, row)
        if snippet:
            if text:
                snippet = highlight_snippet(snippet, text, ansi=ansi)
            snippets.append(f"{file_path}:{row + 1}: {snippet}")
    return "\n".join(snippets)


def parse_row(line):
    # Example: capture: 0 - pkg, start: (757, 2), end: (757, 5), text: `log`
    m = re.search(r"start: \((\d+),", line)
    if not m:
        return None
    return int(m.group(1))


def parse_text(line):
    m = re.search(r"text: `([^`]+)`", line)
    if not m:
        return None
    return m.group(1)


def extract_line(path, row):
    try:
        with open(path, "r", encoding="utf-8") as f:
            for idx, content in enumerate(f):
                if idx == row:
                    return content.rstrip()
    except OSError:
        return None
    return None


def highlight_snippet(line, token, ansi=False):
    if not token or token not in line:
        return line
    if ansi:
        return line.replace(token, f"\x1b[1m{token}\x1b[0m", 1)
    return line.replace(token, f"**{token}**", 1)


def render_markdown(md, root, ansi=False):
    md = re.sub(
        r"\n[ \t]*" + re.escape(BEGIN_MARK) + r".*?" + r"[ \t]*" + re.escape(END_MARK),
        "",
        md,
        flags=re.DOTALL,
    )
    pattern = re.compile(r"(^[ \t]*```scm\n(.*?)\n^[ \t]*```)", re.DOTALL | re.MULTILINE)
    out = []
    last_end = 0
    go_files = find_go_files(root)

    for match in pattern.finditer(md):
        out.append(md[last_end:match.end()])
        full_block = match.group(1)
        query_text = match.group(2).strip()
        indent_match = re.match(r"^[ \\t]*", full_block)
        indent = indent_match.group(0) if indent_match else ""
        rendered, err = run_query(query_text, go_files, scope="source.go", ansi=ansi)

        result_block = []
        result_block.append("")
        result_block.append(indent + BEGIN_MARK)
        if err:
            result_block.append(indent + f"_tree-sitter unavailable: {err}_")
        elif rendered:
            fence = "```go" if not ansi else "```"
            result_block.append(indent + fence)
            for line in rendered.splitlines():
                result_block.append(indent + line)
            result_block.append(indent + "```")
        else:
            result_block.append(indent + "_no matches_")
        result_block.append(indent + END_MARK)

        out.append("\n" + "\n".join(result_block))
        last_end = match.end()

        # Skip any existing result blocks that immediately follow.
        tail = md[last_end:]
        existing = re.search(
            r"^\s*\n[ \t]*" + re.escape(BEGIN_MARK) + r".*?" + r"[ \t]*" + re.escape(END_MARK),
            tail,
            re.DOTALL,
        )
        if existing:
            last_end += existing.end()

    out.append(md[last_end:])
    return "".join(out)


def main():
    parser = argparse.ArgumentParser(description="Render tree-sitter query results into Markdown notes.")
    parser.add_argument("input", help="Input markdown file")
    parser.add_argument("--in-place", action="store_true", help="Rewrite the input file")
    parser.add_argument("--ansi", action="store_true", help="Use ANSI highlight (non in-place only)")
    parser.add_argument("--root", default=".", help="Repo root for searching source files")
    args = parser.parse_args()

    with open(args.input, "r", encoding="utf-8") as f:
        md = f.read()

    rendered = render_markdown(md, args.root, ansi=args.ansi)

    if args.in_place:
        if args.ansi:
            parser.error("--ansi cannot be used with --in-place")
        with open(args.input, "w", encoding="utf-8") as f:
            f.write(rendered)
    else:
        print(rendered)


if __name__ == "__main__":
    main()

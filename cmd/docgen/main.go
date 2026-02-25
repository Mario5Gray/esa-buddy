package main

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
)

const chromaStyle = "monokai"

const pageTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{ .Title }}</title>
  <style>
    body { font-family: sans-serif; max-width: 860px; margin: 2rem auto; padding: 0 1rem; line-height: 1.6; }
    pre { border-radius: 4px; padding: 1em; overflow-x: auto; }
    code { font-size: 0.9em; }
    {{ .CSS }}
  </style>
</head>
<body>
{{ .Body }}
</body>
</html>`

func buildCSS() (string, error) {
	style := styles.Get(chromaStyle)
	if style == nil {
		style = styles.Fallback
	}
	formatter := chromahtml.New(chromahtml.WithClasses(true))
	var buf bytes.Buffer
	if err := formatter.WriteCSS(&buf, style); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func buildMarkdown() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithStyle(chromaStyle),
				highlighting.WithFormatOptions(
					chromahtml.WithClasses(true),
				),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
	)
}

func convertFile(md goldmark.Markdown, css, src, dst string) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}

	var body bytes.Buffer
	if err := md.Convert(raw, &body); err != nil {
		return fmt.Errorf("convert %s: %w", src, err)
	}

	title := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
	tmpl := template.Must(template.New("page").Parse(pageTmpl))

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	f, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer f.Close()

	return tmpl.Execute(f, map[string]any{
		"Title": title,
		"CSS":   template.CSS(css),
		"Body":  template.HTML(body.String()),
	})
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: docgen <file.md> [file.md ...]")
		os.Exit(1)
	}

	css, err := buildCSS()
	if err != nil {
		fmt.Fprintln(os.Stderr, "css:", err)
		os.Exit(1)
	}

	md := buildMarkdown()
	var failed bool

	for _, src := range os.Args[1:] {
		dir := filepath.Dir(src)
		base := strings.TrimSuffix(filepath.Base(src), ".md")
		dst := filepath.Join("site", dir, base+".html")
		if err := convertFile(md, css, src, dst); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			failed = true
			continue
		}
		fmt.Println("wrote", dst)
	}

	if failed {
		os.Exit(1)
	}
}

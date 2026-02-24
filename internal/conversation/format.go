package conversation

import (
	"fmt"
	"log/slog"

	"github.com/charmbracelet/glamour"
	"github.com/fatih/color"
)

func PrintPrettyOutput(content string) {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)
	if err != nil {
		fmt.Println(content)
		return
	}

	out, err := renderer.Render(content)
	if err != nil {
		fmt.Println(content)
		return
	}

	fmt.Print(out)
}

func createDebugPrinter(debugMode bool, logger *slog.Logger) func(string, ...any) {
	if !debugMode {
		return func(section string, v ...any) {}
	}

	headerStyle := color.New(color.FgHiCyan, color.Bold).SprintFunc()
	subStyle := color.New(color.FgHiBlack).SprintFunc()

	return func(section string, v ...any) {
		if logger != nil {
			fields := make([]any, 0, len(v))
			for i, item := range v {
				fields = append(fields, fmt.Sprintf("item_%d", i))
				fields = append(fields, item)
			}
			logger.Debug(section, fields...)
		}
		fmt.Printf("%s %s\n", headerStyle("[DEBUG]"), section)
		for _, item := range v {
			fmt.Printf("  %s %v\n", subStyle("•"), item)
		}
		fmt.Println()
	}
}

package utils

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// ProcessShellBlocks processes special blocks in a string:
// {{$...}} blocks are executed as shell commands and replaced with output
// {{#...}} blocks prompt for user input with the text as prompt
func ProcessShellBlocks(input string) (string, error) {
	// Process shell command blocks {{$...}}
	shellRegex := regexp.MustCompile(`{{\$(.*?)}}`)
	result := shellRegex.ReplaceAllStringFunc(input, func(match string) string {
		command := match[3 : len(match)-2] // Extract command without {{$ and }}
		cmd := exec.Command("sh", "-c", command)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Sprintf("Error: %v", err)
		}
		return strings.TrimSpace(string(output))
	})

	// Process user input blocks {{#...}}
	inputRegex := regexp.MustCompile(`{{#(.*?)}}`)
	result = inputRegex.ReplaceAllStringFunc(result, func(match string) string {
		prompt := match[3 : len(match)-2] // Extract prompt without {{# and }}
		input, err := ReadUserInput(prompt, true)
		if err != nil {
			return fmt.Sprintf("Error: %v", err)
		}

		return input
	})

	return result, nil
}

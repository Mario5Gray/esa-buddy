package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
	"golang.org/x/term"
)

// confirmResponse represents the response type from a confirmation prompt
type confirmResponse struct {
	approved bool
	message  string
}

// openTTY opens /dev/tty for interactive prompts, bypassing piped stdin
func openTTY() (*os.File, error) {
	return os.OpenFile("/dev/tty", os.O_RDWR, 0)
}

// confirm prompts the user for confirmation with yes/no/message options
func confirm(prompt string) confirmResponse {
	cyan := color.New(color.FgCyan).SprintFunc()
	fmt.Fprintf(os.Stderr, "%s %s (m/y/N): ", cyan("[?]"), prompt)

	// Open /dev/tty for interactive input to bypass piped stdin
	tty, err := openTTY()
	if err != nil {
		// Fallback to os.Stdin if /dev/tty is not available
		tty = os.Stdin
	} else {
		defer tty.Close()
	}

	oldState, _ := term.MakeRaw(int(tty.Fd()))

	reader := bufio.NewReader(tty)
	char, err := reader.ReadByte()
	if err != nil {
		return confirmResponse{approved: false, message: ""}
	}

	response := strings.ToLower(string(char))
	fmt.Fprintf(os.Stderr, "%s\n\r", response)

	term.Restore(int(tty.Fd()), oldState)

	if response == "m" {
		fmt.Fprintf(os.Stderr, "%s Enter message: ", cyan("[?]"))
		reader := bufio.NewReader(tty)
		message, _ := reader.ReadString('\n')
		message = strings.TrimSuffix(message, "\n")
		return confirmResponse{approved: false, message: message}
	}

	return confirmResponse{approved: response == "y", message: ""}
}

// Read stdin if exists. Used to detect if input is piped and read it if so
func readStdin() string {
	var input bytes.Buffer
	stat, err := os.Stdin.Stat()
	if err == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
		if _, err := io.Copy(&input, os.Stdin); err != nil {
			return ""
		}
	}
	return input.String()
}

func readUserInput(prompt string, multiline bool) (string, error) {
	// Open /dev/tty for interactive input to bypass piped stdin
	tty, err := openTTY()
	var reader *bufio.Reader
	if err != nil {
		// Fallback to os.Stdin if /dev/tty is not available
		reader = bufio.NewReader(os.Stdin)
	} else {
		defer tty.Close()
		reader = bufio.NewReader(tty)
	}

	if prompt != "" {
		color.New(color.FgBlue).Fprint(os.Stderr, prompt)
		color.New(color.FgHiWhite, color.Italic).Fprint(os.Stderr, " (ctrl+d on empty line to complete)\n")
	}

	var result strings.Builder

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err.Error() == "EOF" {
				// Ctrl+D pressed
				break
			}
			return "", err
		}

		// Return if we just want a single line
		if !multiline {
			return line, nil
		}

		// Remove the trailing newline and add to result
		line = strings.TrimSuffix(line, "\n")

		if result.Len() > 0 {
			result.WriteByte('\n')
		}
		result.WriteString(line)

		// Check if line is empty and we got EOF (Ctrl+D)
		if line == "" {
			nextByte, err := reader.ReadByte()
			if err != nil && err.Error() == "EOF" {
				break
			}
			if err == nil {
				// Put the byte back by creating a new reader with it
				result.WriteByte('\n')
				remaining, _ := reader.ReadString('\n')
				result.WriteString(string(nextByte) + remaining)
			}
		}
	}

	return result.String(), nil
}

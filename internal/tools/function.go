package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/meain/esa/internal/agent"
	"github.com/meain/esa/internal/utils"
	"github.com/sashabaranov/go-openai"
)

func ConvertFunctionsToTools(functions []agent.FunctionConfig) []openai.Tool {
	var tools []openai.Tool
	for _, fc := range functions {
		function := convertToOpenAIFunction(fc)
		tools = append(tools, openai.Tool{
			Type:     openai.ToolTypeFunction,
			Function: &function,
		})
	}
	return tools
}

func convertToOpenAIFunction(fc agent.FunctionConfig) openai.FunctionDefinition {
	properties := make(map[string]any)
	required := []string{}

	for _, param := range fc.Parameters {
		paramProps := map[string]any{
			"type":        param.Type,
			"description": param.Description,
		}
		if len(param.Options) > 0 {
			paramProps["enum"] = param.Options
		}
		properties[param.Name] = paramProps
		if param.Required {
			required = append(required, param.Name)
		}
	}

	desc := fmt.Sprintf(
		"%s\n\nThe templated cli command that will be ran is: `%s`",
		fc.Description,
		fc.Command,
	)

	return openai.FunctionDefinition{
		Name:        fc.Name,
		Description: desc,
		Parameters: map[string]any{
			"type":       "object",
			"properties": properties,
			"required":   required,
		},
	}
}

func ExecuteFunction(
	askLevel string,
	fc agent.FunctionConfig,
	args string,
) (bool, string, string, string, error) {
	parsedArgs, err := parseAndValidateArgs(fc, args)
	if err != nil {
		return false, "", "", "", err
	}

	command, err := prepareCommand(fc, parsedArgs)
	if err != nil {
		return false, "", "", "", err
	}

	origCommand := command
	command = utils.ExpandHomePath(command)

	// Check if confirmation is needed
	if NeedsConfirmation(askLevel, fc.Safe) {
		response := utils.Confirm(fmt.Sprintf("Execute `%s`?", command))
		if !response.Approved {
			if response.Message != "" {
				return false, command, "", fmt.Sprintf("Message from user: %s", response.Message), nil
			}
			return false, command, "", "Command execution cancelled by user.", nil
		}
	}

	output, stdinContent, err := executeShellCommand(command, fc, parsedArgs)
	return true, origCommand, stdinContent, strings.TrimSpace(string(output)), err
}

func parseAndValidateArgs(fc agent.FunctionConfig, args string) (map[string]any, error) {
	if args == "" {
		return make(map[string]any), nil
	}

	var parsedArgs map[string]any
	if err := json.Unmarshal([]byte(args), &parsedArgs); err != nil {
		return nil, fmt.Errorf("error parsing arguments: %v", err)
	}

	// Validate required parameters
	var missingParams []string
	for _, param := range fc.Parameters {
		if param.Required {
			if value, exists := parsedArgs[param.Name]; !exists || value == nil {
				missingParams = append(missingParams, param.Name)
			}
		}
	}

	if len(missingParams) > 0 {
		return nil, fmt.Errorf("missing required parameters: %s", strings.Join(missingParams, ", "))
	}

	return parsedArgs, nil
}

func prepareCommand(fc agent.FunctionConfig, parsedArgs map[string]any) (string, error) {
	command := fc.Command

	// First, process any shell command blocks in the command
	var err error
	command, err = utils.ProcessShellBlocks(command)
	if err != nil {
		return "", fmt.Errorf("error processing shell blocks in command: %v", err)
	}

	// Replace parameters with their values
	for _, param := range fc.Parameters {
		placeholder := fmt.Sprintf("{{%s}}", param.Name)

		if value, exists := parsedArgs[param.Name]; exists {
			replacement, err := getParameterReplacement(param, value)
			if err != nil {
				return "", err
			}
			command = strings.ReplaceAll(command, placeholder, replacement)
		} else if !param.Required {
			command = strings.ReplaceAll(command, placeholder, "")
		}
	}

	// Clean up any extra spaces from removed optional parameters
	return strings.Join(strings.Fields(command), " "), nil
}

func getParameterReplacement(param agent.ParameterConfig, value any) (string, error) {
	switch {
	case param.Format == "boolean":
		boolValue, err := strconv.ParseBool(fmt.Sprintf("%v", value))
		if err != nil {
			return "", fmt.Errorf("invalid boolean value: %v", value)
		}
		if boolValue {
			return param.Format, nil
		}
		return "", nil

	case param.Format != "" && !strings.Contains(param.Format, "%"):
		return param.Format, nil

	case param.Format != "":
		return fmt.Sprintf(param.Format, value), nil

	default:
		return fmt.Sprintf("%v", value), nil
	}
}

func NeedsConfirmation(askLevel string, isSafe bool) bool {
	if askLevel == "" {
		askLevel = "unsafe"
	}
	return askLevel == "all" || (askLevel == "unsafe" && !isSafe)
}

func executeShellCommand(
	command string,
	fc agent.FunctionConfig,
	args map[string]any,
) ([]byte, string, error) {
	var stdinContent string

	if fc.Output != "" {
		// Process output template similar to command
		formattedOutput, err := utils.ProcessShellBlocks(fc.Output)
		if err != nil {
			return nil, "", fmt.Errorf("error processing output template: %v", err)
		}

		// Replace parameters in output template
		for _, param := range fc.Parameters {
			placeholder := fmt.Sprintf("{{%s}}", param.Name)
			if value, exists := args[param.Name]; exists {
				replacement, err := getParameterReplacement(param, value)
				if err != nil {
					return nil, "", err
				}
				formattedOutput = strings.ReplaceAll(formattedOutput, placeholder, replacement)
			}
		}

		fmt.Print(formattedOutput)
	}

	// Set up context with timeout
	ctx := context.Background()
	timeout := fc.Timeout
	if timeout <= 0 {
		timeout = 60 // default to 60 seconds if not set
	}

	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
	}

	// Create command with context
	cmd := exec.CommandContext(ctx, "sh", "-c", command)

	// Set working directory if specified
	if fc.Pwd != "" {
		// Process templates in pwd similar to command
		pwd := fc.Pwd
		for _, param := range fc.Parameters {
			placeholder := fmt.Sprintf("{{%s}}", param.Name)
			if value, exists := args[param.Name]; exists {
				replacement, err := getParameterReplacement(param, value)
				if err != nil {
					return nil, "", err
				}
				pwd = strings.ReplaceAll(pwd, placeholder, replacement)
			}
		}
		pwd = utils.ExpandHomePath(pwd)
		cmd.Dir = os.ExpandEnv(pwd) // Support environment variables in pwd
	}

	if fc.Stdin != "" {
		stdinContent = prepareStdinContent(fc.Stdin, args)
		cmd.Stdin = strings.NewReader(stdinContent)
	} else {
		cmd.Stdin = os.Stdin
	}
	// Start the command and capture output
	var output []byte
	var cmdErr error

	// Use a channel to handle the command execution
	done := make(chan struct{})
	go func() {
		defer close(done)
		output, cmdErr = cmd.CombinedOutput()
	}()

	// Wait for either completion or timeout
	select {
	case <-done:
		// Command completed normally
		if cmdErr != nil {
			return output, stdinContent, fmt.Errorf("%v\nCommand: %s\nOutput: %s", cmdErr, command, string(output))
		}
		return output, stdinContent, nil

	case <-ctx.Done():
		// Context was cancelled (timeout or other cancellation)
		// Try to kill the process
		if cmd.Process != nil {
			cmd.Process.Kill()
		}

		// Wait a bit for the goroutine to finish
		select {
		case <-done:
			// Goroutine finished
		case <-time.After(3 * time.Second):
			// Give it some time to gracefully exit
		}

		if ctx.Err() == context.DeadlineExceeded {
			return nil, "", fmt.Errorf("command timed out after %d seconds: %s", timeout, command)
		}
		return nil, "", fmt.Errorf("command was cancelled: %s", command)
	}
}

func prepareStdinContent(stdinTemplate string, args map[string]any) string {
	// First, process any shell command blocks
	processed, err := utils.ProcessShellBlocks(stdinTemplate)
	if err != nil {
		// If there's an error, just continue with the original template
		processed = stdinTemplate
	}

	// Then replace parameter placeholders
	for key, value := range args {
		placeholder := fmt.Sprintf("{{%s}}", key)
		processed = strings.ReplaceAll(processed, placeholder, fmt.Sprintf("%v", value))
	}
	return processed
}

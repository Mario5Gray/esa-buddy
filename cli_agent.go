package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/fatih/color"
)

// parseAgentCommand handles the +agent syntax, extracting agent name and remaining command
func parseAgentCommand(opts *CLIOptions) {
	parts := strings.SplitN(opts.CommandStr, " ", 2)

	// Extract agent string (with + prefix)
	agentStr := parts[0]

	// Update command string if there's content after the agent name
	if len(parts) < 2 {
		// Clear CommandStr so it can use initial_message
		opts.CommandStr = ""
	} else {
		opts.CommandStr = parts[1]
	}

	// Parse agent string
	agentName, agentPath, agentVersion := ParseAgentStringWithVersion(agentStr)
	opts.AgentName = agentName
	opts.AgentPath = agentPath
	opts.AgentVersion = agentVersion

	// Check if this is a user agent that overrides a builtin
	if strings.HasPrefix(agentPath, "builtin:") && opts.DebugMode {
		userAgentPath := expandHomePath(fmt.Sprintf("%s/%s.toml", DefaultAgentsDir, agentName))
		if _, err := os.Stat(userAgentPath); err == nil {
			fmt.Printf("Note: Using user agent '%s' which overrides the built-in agent with the same name\n", agentName)
			opts.AgentPath = userAgentPath
		}
	}
}

// getUserAgents gets a list of user agents from the default config directory
func getUserAgents(showErrors bool) ([]Agent, []string, bool) {
	var agents []Agent
	var names []string

	// Expand the default config directory
	agentDir := expandHomePath(DefaultAgentsDir)

	// Check if the directory exists
	if _, err := os.Stat(agentDir); os.IsNotExist(err) {
		if showErrors {
			color.Red("Agent directory does not exist: %s\n", agentDir)
		}
		return agents, names, false
	}

	// Read all .toml files in the directory
	files, err := os.ReadDir(agentDir)
	if err != nil {
		if showErrors {
			color.Red("Error reading agent directory: %v\n", err)
		}
		return agents, names, false
	}

	userAgentsFound := false

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".toml") {
			userAgentsFound = true
			agentName := strings.TrimSuffix(file.Name(), ".toml")
			names = append(names, agentName)

			// Load the agent config to get the description
			agentPath := filepath.Join(agentDir, file.Name())
			agent, err := loadAgent(agentPath)

			if err != nil {
				if showErrors {
					color.Red("  %s: Error loading agent\n", agentName)
				}
				continue
			}

			agents = append(agents, agent)
		}
	}

	return agents, names, userAgentsFound
}

// listUserAgents lists only user agents in the default config directory
func listUserAgents() {
	builtinStyle := color.New(color.FgHiMagenta, color.Bold).SprintFunc()
	fmt.Println(builtinStyle("User Agents:"))

	agents, names, userAgentsFound := getUserAgents(true)

	for i := range agents {
		printAgentInfo(agents[i], names[i])
	}

	if !userAgentsFound {
		color.Yellow("  No user agents found in the agent directory.")
	}
}

// listAgents lists all available agents in the default config directory and built-in agents
func listAgents() {
	builtinStyle := color.New(color.FgHiMagenta, color.Bold).SprintFunc()
	foundAgents := false

	// First list built-in agents
	fmt.Println(builtinStyle("Built-in Agents:"))
	for name, tomlContent := range builtinAgents {
		foundAgents = true

		// Parse the agent from TOML content
		var agent Agent
		if _, err := toml.Decode(tomlContent, &agent); err != nil {
			color.Red("%s: Error loading built-in agent\n", name)
			continue
		}

		printAgentInfo(agent, name)
	}

	fmt.Println()
	fmt.Println(builtinStyle("User Agents:"))

	agents, names, userAgentsFound := getUserAgents(false)

	for i := range agents {
		foundAgents = true
		printAgentInfo(agents[i], names[i])
	}

	if !userAgentsFound {
		color.Yellow("  No user agents found in the agent directory.")
	}

	if !foundAgents {
		color.Yellow("No agents found.")
	}
}

// handleShowAgent displays the details of the agent specified by the agentPath.
func handleShowAgent(agentPath string) {
	agent, err := loadAgent(agentPath)
	if err != nil {
		printError(fmt.Sprintf("Error loading agent: %v", err))
		return
	}

	labelStyle := color.New(color.FgHiCyan, color.Bold).SprintFunc()

	// Print agent header
	if agent.Name != "" {
		fmt.Printf("%s %s (%s)\n", labelStyle("Agent:"), agent.Name, filepath.Base(agentPath))
	} else {
		fmt.Printf("%s %s\n", labelStyle("Agent:"), filepath.Base(agentPath))
	}

	if agent.Description != "" {
		fmt.Printf("%s %s\n", labelStyle("Description:"), agent.Description)
	}
	fmt.Println()

	// Print available functions
	if len(agent.Functions) > 0 {
		fmt.Printf("%s\n", labelStyle("Functions:"))
		for _, fn := range agent.Functions {
			printFunctionInfo(fn)
		}
	}

	// Print MCP servers
	if len(agent.MCPServers) > 0 {
		fmt.Printf("%s\n", labelStyle("MCP Servers:"))
		for name, server := range agent.MCPServers {
			printMCPServerInfo(name, server)
		}
	}

	if len(agent.Functions) == 0 && len(agent.MCPServers) == 0 {
		noFuncStyle := color.New(color.FgYellow, color.Italic).SprintFunc()
		fmt.Printf("%s\n", noFuncStyle("No functions or MCP servers available."))
	}
}

package main

import (
	"fmt"
	"strings"
)

// ParseAgentString handles all agent string formats:
// - +name (built-in or user agent by name)
// - name (without + prefix, treated as agent name)
// - /path/to/agent.toml (direct file path)
// - builtin:name (builtin agent specification)
// - name@v1.2.3 (version pinning; version ignored for local resolution)
//
// Returns agentName and agentPath. If the input is a direct path,
// agentName will be empty.
func ParseAgentString(input string) (agentName, agentPath string) {
	agentName, agentPath, _ = ParseAgentStringWithVersion(input)
	return agentName, agentPath
}

// ParseAgentStringWithVersion handles agent strings and extracts the pinned version if present.
// The returned version is the raw suffix after "@", without validation.
func ParseAgentStringWithVersion(input string) (agentName, agentPath, version string) {
	// Handle +agent syntax
	if strings.HasPrefix(input, "+") {
		agentName = input[1:] // Remove + prefix
		agentName, version = splitAgentRefVersion(agentName)

		// Check for builtin agents first
		if _, exists := builtinAgents[agentName]; exists {
			agentPath = "builtin:" + agentName
			return
		}

		// Otherwise treat as user agent name
		agentPath = expandHomePath(fmt.Sprintf("%s/%s.toml", DefaultAgentsDir, agentName))
		return
	}

	// Handle direct path (contains / or ends with .toml)
	if strings.Contains(input, "/") || strings.HasSuffix(input, ".toml") {
		agentPath, version = splitAgentRefVersion(input)
		if !strings.HasPrefix(agentPath, "/") {
			agentPath = expandHomePath(agentPath)
		}
		return
	}

	// Handle plain name without + prefix
	agentName, version = splitAgentRefVersion(input)

	// Check for builtin agents
	if _, exists := builtinAgents[agentName]; exists {
		agentPath = "builtin:" + agentName
		return
	}

	// Treat as user agent name
	agentPath = expandHomePath(fmt.Sprintf("%s/%s.toml", DefaultAgentsDir, agentName))
	return
}

func splitAgentRefVersion(input string) (string, string) {
	at := strings.LastIndex(input, "@")
	if at <= 0 || at == len(input)-1 {
		return input, ""
	}
	return input[:at], input[at+1:]
}

package main

import (
	"fmt"
	"strings"
)

// validateAgent performs validation on an agent configuration
// to ensure all required fields are present and properly formatted.
func validateAgent(agent Agent) (Agent, error) {
	var err error

	if agent.Version != "" && !isValidSemver(agent.Version) {
		return agent, fmt.Errorf("agent '%s' has invalid version: %q", agent.Name, agent.Version)
	}

	// Validate each function configuration
	for i, fc := range agent.Functions {
		if fc.Name == "" {
			return agent, fmt.Errorf("function %d in agent '%s' has no name", i+1, agent.Name)
		}
		if fc.Command == "" {
			return agent, fmt.Errorf("function %s in agent '%s' has no command defined", fc.Name, agent.Name)
		}

		agent.Functions[i].Description, err = processShellBlocks(fc.Description)
		if err != nil {
			return agent, fmt.Errorf("error processing shell blocks in function %s: %v", fc.Name, err)
		}

		// Validate parameters
		for j, param := range fc.Parameters {
			if param.Name == "" {
				return agent, fmt.Errorf("parameter %d in function '%s' has no name", j+1, fc.Name)
			}
			if param.Type == "" {
				return agent, fmt.Errorf("parameter %s in function '%s' has no type defined", param.Name, fc.Name)
			}

			// Validate parameter type
			validTypes := map[string]bool{
				"string":  true,
				"number":  true,
				"boolean": true,
				"array":   true,
				"object":  true,
			}
			if !validTypes[param.Type] {
				return agent, fmt.Errorf("parameter %s in function '%s' has invalid type: %s", param.Name, fc.Name, param.Type)
			}

			agent.Functions[i].Parameters[j].Description, err = processShellBlocks(param.Description)
			if err != nil {
				return agent, fmt.Errorf("error processing shell blocks in parameter %s of function %s: %v",
					param.Name, fc.Name, err)
			}
		}
	}

	// Validate MCP server configurations
	for serverName, serverConfig := range agent.MCPServers {
		if serverConfig.Command == "" {
			return agent, fmt.Errorf("MCP server '%s' has no command defined", serverName)
		}

		// Check that any safe or allowed functions referenced actually exist in the server
		// This would require knowledge of what functions each server exposes
	}

	return agent, nil
}

func isValidSemver(version string) bool {
	if version == "" {
		return false
	}
	if strings.HasPrefix(version, "v") {
		return false
	}

	parts := strings.SplitN(version, "-", 2)
	core := parts[0]
	nums := strings.Split(core, ".")
	if len(nums) != 3 {
		return false
	}
	for _, n := range nums {
		if n == "" {
			return false
		}
		for _, ch := range n {
			if ch < '0' || ch > '9' {
				return false
			}
		}
	}
	return true
}

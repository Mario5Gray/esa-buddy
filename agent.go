package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type Agent struct {
	Name           string                     `toml:"name"`
	Description    string                     `toml:"description"`
	Extends        string                     `toml:"extends"`
	Version        string                     `toml:"version"`
	Functions      []FunctionConfig           `toml:"functions"`
	MCPServers     map[string]MCPServerConfig `toml:"mcp_servers"`
	Ask            string                     `toml:"ask"`
	SystemPrompt   string                     `toml:"system_prompt"`
	InitialMessage string                     `toml:"initial_message"`
	DefaultModel   string                     `toml:"default_model"`
	Think          *bool                      `toml:"think,omitempty"`
}

// MCPServerConfig represents the configuration for an MCP server
type MCPServerConfig struct {
	Command          string   `toml:"command"`
	Args             []string `toml:"args"`
	Safe             bool     `toml:"safe"`              // Whether tools from this server are considered safe by default
	SafeFunctions    []string `toml:"safe_functions"`    // List of specific functions that are safe (overrides server-level safe setting)
	AllowedFunctions []string `toml:"allowed_functions"` // List of functions to expose to the LLM (if empty, all functions are allowed)
}

type FunctionConfig struct {
	Name        string            `toml:"name"`
	Description string            `toml:"description"`
	Command     string            `toml:"command"`
	Parameters  []ParameterConfig `toml:"parameters"`
	Safe        bool              `toml:"safe"`
	Stdin       string            `toml:"stdin,omitempty"`
	Output      string            `toml:"output"`
	Pwd         string            `toml:"pwd,omitempty"`
	Timeout     int               `toml:"timeout"`
}

type ParameterConfig struct {
	Name        string   `toml:"name"`
	Type        string   `toml:"type"`
	Description string   `toml:"description"`
	Required    bool     `toml:"required"`
	Format      string   `toml:"format,omitempty"`
	Options     []string `toml:"options,omitempty"`
}

func loadAgent(agentPath string) (Agent, error) {
	visited := make(map[string]bool)
	agent, err := loadAgentFromFile(agentPath, visited, 0)
	if err != nil {
		return Agent{}, err
	}

	return validateAgent(agent)
}

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

func loadConfiguration(opts *CLIOptions) (Agent, error) {
	if _, exists := builtinAgents[opts.AgentName]; exists {
		visited := make(map[string]bool)
		agent, err := loadAgentFromBuiltin(opts.AgentName, visited, 0)
		if err != nil {
			return Agent{}, err
		}
		return validateAgent(agent)
	}

	agentPath := expandHomePath(opts.AgentPath)
	_, err := os.Stat(agentPath)
	if err != nil {
		if os.IsNotExist(err) && opts.AgentName == "" && opts.AgentPath == DefaultAgentPath {
			visited := make(map[string]bool)
			agent, err := loadAgentFromBuiltin("default", visited, 0)
			if err != nil {
				return Agent{}, err
			}
			return validateAgent(agent)
		}
	}

	return loadAgent(agentPath)
}

const maxExtendsDepth = 20

func loadAgentFromFile(agentPath string, visited map[string]bool, depth int) (Agent, error) {
	if depth > maxExtendsDepth {
		return Agent{}, fmt.Errorf("agent inheritance exceeds max depth (%d)", maxExtendsDepth)
	}

	resolvedPath := expandHomePath(agentPath)
	if absPath, err := filepath.Abs(resolvedPath); err == nil {
		resolvedPath = absPath
	}

	key := "file:" + resolvedPath
	if visited[key] {
		return Agent{}, fmt.Errorf("agent inheritance cycle detected at %s", resolvedPath)
	}
	visited[key] = true
	defer delete(visited, key)

	var agent Agent
	if _, err := toml.DecodeFile(resolvedPath, &agent); err != nil {
		return Agent{}, err
	}

	return resolveAgentExtends(agent, resolvedPath, visited, depth)
}

func loadAgentFromBuiltin(name string, visited map[string]bool, depth int) (Agent, error) {
	if depth > maxExtendsDepth {
		return Agent{}, fmt.Errorf("agent inheritance exceeds max depth (%d)", maxExtendsDepth)
	}

	key := "builtin:" + name
	if visited[key] {
		return Agent{}, fmt.Errorf("agent inheritance cycle detected at %s", key)
	}
	visited[key] = true
	defer delete(visited, key)

	conf, exists := builtinAgents[name]
	if !exists {
		return Agent{}, fmt.Errorf("unknown builtin agent: %s", name)
	}

	var agent Agent
	if _, err := toml.Decode(conf, &agent); err != nil {
		return Agent{}, fmt.Errorf("error loading embedded '%s' agent config: %v", name, err)
	}

	return resolveAgentExtends(agent, "", visited, depth)
}

func resolveAgentExtends(agent Agent, childPath string, visited map[string]bool, depth int) (Agent, error) {
	if strings.TrimSpace(agent.Extends) == "" {
		return agent, nil
	}

	parent, err := loadAgentFromRef(agent.Extends, childPath, visited, depth+1)
	if err != nil {
		return Agent{}, err
	}

	merged := mergeAgents(parent, agent)
	return merged, nil
}

func loadAgentFromRef(ref string, childPath string, visited map[string]bool, depth int) (Agent, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return Agent{}, fmt.Errorf("extends reference is empty")
	}

	ref, _ = splitAgentRefVersion(ref)

	if strings.HasPrefix(ref, "builtin:") {
		return loadAgentFromBuiltin(strings.TrimPrefix(ref, "builtin:"), visited, depth)
	}

	if _, exists := builtinAgents[ref]; exists {
		return loadAgentFromBuiltin(ref, visited, depth)
	}

	if looksLikePath(ref) {
		resolvedPath := resolveAgentPath(ref, childPath)
		return loadAgentFromFile(resolvedPath, visited, depth)
	}

	userAgentPath := expandHomePath(fmt.Sprintf("%s/%s.toml", DefaultAgentsDir, ref))
	return loadAgentFromFile(userAgentPath, visited, depth)
}

func looksLikePath(ref string) bool {
	return strings.Contains(ref, string(os.PathSeparator)) || strings.HasSuffix(ref, ".toml")
}

func resolveAgentPath(ref string, childPath string) string {
	ref = expandHomePath(ref)
	if filepath.IsAbs(ref) {
		return ref
	}

	if childPath != "" {
		return filepath.Join(filepath.Dir(childPath), ref)
	}

	if cwd, err := os.Getwd(); err == nil {
		return filepath.Join(cwd, ref)
	}

	return ref
}

func mergeAgents(parent Agent, child Agent) Agent {
	merged := parent

	if child.Name != "" {
		merged.Name = child.Name
	}
	if child.Description != "" {
		merged.Description = child.Description
	}
	if child.Ask != "" {
		merged.Ask = child.Ask
	}
	if child.InitialMessage != "" {
		merged.InitialMessage = child.InitialMessage
	}
	if child.DefaultModel != "" {
		merged.DefaultModel = child.DefaultModel
	}
	if child.Think != nil {
		merged.Think = child.Think
	}

	if child.SystemPrompt != "" {
		if merged.SystemPrompt != "" {
			merged.SystemPrompt = merged.SystemPrompt + "\n\n" + child.SystemPrompt
		} else {
			merged.SystemPrompt = child.SystemPrompt
		}
	}

	merged.Functions = mergeFunctions(parent.Functions, child.Functions)
	merged.MCPServers = mergeMCPServers(parent.MCPServers, child.MCPServers)
	merged.Extends = child.Extends

	return merged
}

func mergeFunctions(parent []FunctionConfig, child []FunctionConfig) []FunctionConfig {
	if len(parent) == 0 {
		return append([]FunctionConfig{}, child...)
	}
	if len(child) == 0 {
		return append([]FunctionConfig{}, parent...)
	}

	merged := append([]FunctionConfig{}, parent...)
	index := make(map[string]int, len(merged))
	for i, fn := range merged {
		if fn.Name != "" {
			index[fn.Name] = i
		}
	}

	for _, fn := range child {
		if fn.Name != "" {
			if idx, exists := index[fn.Name]; exists {
				merged[idx] = fn
				continue
			}
			index[fn.Name] = len(merged)
		}
		merged = append(merged, fn)
	}

	return merged
}

func mergeMCPServers(parent map[string]MCPServerConfig, child map[string]MCPServerConfig) map[string]MCPServerConfig {
	if parent == nil && child == nil {
		return nil
	}

	merged := make(map[string]MCPServerConfig)
	for name, cfg := range parent {
		merged[name] = cfg
	}
	for name, cfg := range child {
		merged[name] = cfg
	}

	return merged
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

const systemPrompt = `You are Esa, a professional assistant capable of performing various tasks. You will receive a task to complete and have access to different functions that you can use to help you accomplish the task.

When responding to tasks:
1. Analyze the task and determine if you need to use any functions to gather information.
2. If needed, make function calls to gather necessary information.
3. Process the information and formulate your response.
4. Provide only concise responses that directly address the task.

Other information:
- Date: {{$date '+%Y-%m-%d %A'}}
- OS: {{$uname}}
- Current directory: {{$pwd}}

Remember to keep your responses brief and to the point. Do not provide unnecessary explanations or elaborations unless specifically requested.`

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const maxExtendsDepth = 20

func loadAgent(agentPath string) (Agent, error) {
	visited := make(map[string]bool)
	agent, err := loadAgentFromFile(agentPath, visited, 0)
	if err != nil {
		return Agent{}, err
	}

	return validateAgent(agent)
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

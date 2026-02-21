package agent

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

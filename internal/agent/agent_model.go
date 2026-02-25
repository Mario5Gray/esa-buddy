package agent

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

const SystemPrompt = `You are Esa, a professional assistant capable of performing various tasks. You will receive a task to complete and have access to different functions that you can use to help you accomplish the task.

When responding to tasks:
1. Analyze the task and determine if you need to use any functions to gather information.
2. If needed, make function calls to gather necessary information.
3. Process the information and formulate your response.
4. Provide only concise responses that directly address the task.

Tool discovery:
- If you're unsure which tool to use, call tool_search with a short query first.

Other information:
- Date: {{$date '+%Y-%m-%d %A'}}
- OS: {{$uname}}
- Current directory: {{$pwd}}

Remember to keep your responses brief and to the point. Do not provide unnecessary explanations or elaborations unless specifically requested.`

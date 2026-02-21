package options

type CLIOptions struct {
	DebugMode      bool
	ContinueChat   bool
	Conversation   string // continue non-last one
	RetryChat      bool
	ReplMode       bool // Flag for REPL mode
	AgentPath      string
	AskLevel       string
	ShowCommands   bool
	ShowToolCalls  bool
	HideProgress   bool
	CommandStr     string
	AgentName      string
	AgentVersion   string
	Model          string
	ConfigPath     string
	OutputFormat   string // Output format for show-history (text, markdown, json)
	ShowAgent      bool   // Flag for showing agent details
	ListAgents     bool   // Flag for listing agents
	ListUserAgents bool   // Flag for listing only user agents
	ListHistory    bool   // Flag for listing history
	ShowHistory    bool   // Flag for showing specific history
	ShowOutput     bool   // Flag for showing just output from history
	ShowStats      bool   // Flag for showing usage statistics
	ShowAll        bool   // Flag for showing both stats and history
	SystemPrompt   string // System prompt override from CLI
	Pretty         bool   // Pretty print markdown output using glow
	Think          bool   // Enable model thinking/chain-of-thought
	NoThink        bool   // Disable model thinking/chain-of-thought
	Compaction     bool   // Enable prompt compaction
	NoCompaction   bool   // Disable prompt compaction
}

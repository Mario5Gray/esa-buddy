package agent

import (
	"os"
	"path/filepath"
)

// AgentsDir returns the directory where agent configs live.
// Precedence: ESA_AGENTS_DIR, XDG_CONFIG_HOME, os.UserConfigDir().
func AgentsDir() string {
	if env := os.Getenv("ESA_AGENTS_DIR"); env != "" {
		return env
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "esa", "agents")
	}
	if cfgDir, err := os.UserConfigDir(); err == nil && cfgDir != "" {
		return filepath.Join(cfgDir, "esa", "agents")
	}
	// Fallback to a relative directory if config dir can't be resolved.
	return filepath.Join(".config", "esa", "agents")
}

// DefaultAgentPath returns the default agent path.
func DefaultAgentPath() string {
	return filepath.Join(AgentsDir(), "default.toml")
}

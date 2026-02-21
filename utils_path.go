package main

import (
	"os"
	"path/filepath"
	"strings"
)

// expandHomePath expands the ~ character in a path to the user's home directory
func expandHomePath(path string) string {
	if strings.HasPrefix(path, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return path // Return original path if we can't get home dir
		}
		return filepath.Join(homeDir, path[1:])
	}
	return path
}

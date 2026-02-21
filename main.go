package main

import (
	"os"

	"github.com/meain/esa/internal/cli"
)

func main() {
	rootCmd := cli.CreateRootCommand()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

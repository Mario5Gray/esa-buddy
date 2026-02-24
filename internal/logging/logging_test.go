package logging

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/meain/esa/internal/config"
)

func TestSetupWritesToFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "esa.log")
	cfg := config.LoggingConfig{
		Level:      "info",
		Format:     "text",
		File:       logPath,
		ToStdout:   false,
		ToFile:     true,
		MaxAgeDays: 30,
		MaxSizeMB:  1,
	}

	_, _, err := Setup(cfg)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	log.Print("hello-log")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(data), "hello-log") {
		t.Fatalf("expected log content, got %q", string(data))
	}
}

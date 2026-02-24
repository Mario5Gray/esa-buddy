package tools

import (
	"testing"

	"github.com/meain/esa/internal/agent"
	"github.com/sashabaranov/go-openai"
)

func TestSearchIndexFindsByNameAndDescription(t *testing.T) {
	functions := []agent.FunctionConfig{
		{Name: "list_files", Description: "List directory contents"},
		{Name: "disk_usage", Description: "Show filesystem usage"},
	}
	mcpTools := []openai.Tool{
		{
			Type: "function",
			Function: &openai.FunctionDefinition{
				Name:        "mcp_filesystem_read_file",
				Description: "Read a file from disk",
			},
		},
	}

	index := BuildSearchIndex(functions, mcpTools)
	result := index.Search("list", 10)
	if len(result.Results) == 0 || result.Results[0].Name != "list_files" {
		t.Fatalf("expected list_files to be ranked first, got %+v", result.Results)
	}

	result = index.Search("usage", 10)
	if len(result.Results) == 0 || result.Results[0].Name != "disk_usage" {
		t.Fatalf("expected disk_usage to match by description, got %+v", result.Results)
	}
}

func TestSearchIndexLimit(t *testing.T) {
	functions := []agent.FunctionConfig{
		{Name: "a", Description: "alpha"},
		{Name: "b", Description: "beta"},
		{Name: "c", Description: "charlie"},
	}
	index := BuildSearchIndex(functions, nil)
	result := index.Search("a", 1)
	if len(result.Results) != 1 {
		t.Fatalf("expected limit 1, got %d", len(result.Results))
	}
}

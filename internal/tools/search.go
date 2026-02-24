package tools

import (
	"sort"
	"strings"

	"github.com/meain/esa/internal/agent"
	"github.com/sashabaranov/go-openai"
)

const ToolSearchName = "tool_search"

type ToolSummary struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Parameters  []string `json:"parameters,omitempty"`
	Source      string   `json:"source,omitempty"`
}

type ToolSearchResult struct {
	Query   string        `json:"query"`
	Results []ToolSummary `json:"results"`
}

type SearchIndex struct {
	tools []ToolSummary
}

func BuildSearchIndex(functions []agent.FunctionConfig, mcpTools []openai.Tool) *SearchIndex {
	items := make([]ToolSummary, 0, len(functions)+len(mcpTools))
	for _, fn := range functions {
		if fn.Name == "" {
			continue
		}
		items = append(items, ToolSummary{
			Name:        fn.Name,
			Description: strings.TrimSpace(fn.Description),
			Parameters:  paramNames(fn.Parameters),
			Source:      "function",
		})
	}
	for _, tool := range mcpTools {
		if tool.Function == nil || tool.Function.Name == "" {
			continue
		}
		items = append(items, ToolSummary{
			Name:        tool.Function.Name,
			Description: strings.TrimSpace(tool.Function.Description),
			Source:      "mcp",
		})
	}
	return &SearchIndex{tools: items}
}

func (s *SearchIndex) Search(query string, limit int) ToolSearchResult {
	query = strings.TrimSpace(query)
	if limit <= 0 {
		limit = 8
	}
	results := make([]scoredTool, 0, len(s.tools))
	for _, tool := range s.tools {
		score := scoreTool(tool, query)
		if score <= 0 {
			continue
		}
		results = append(results, scoredTool{
			ToolSummary: tool,
			Score:       score,
		})
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].Name < results[j].Name
		}
		return results[i].Score > results[j].Score
	})

	limit = minInt(limit, len(results))
	out := make([]ToolSummary, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, results[i].ToolSummary)
	}
	return ToolSearchResult{
		Query:   query,
		Results: out,
	}
}

type scoredTool struct {
	ToolSummary
	Score int
}

func scoreTool(tool ToolSummary, query string) int {
	if query == "" {
		return 1
	}
	q := strings.ToLower(query)
	name := strings.ToLower(tool.Name)
	desc := strings.ToLower(tool.Description)
	if strings.Contains(name, q) {
		return 3
	}
	if strings.Contains(desc, q) {
		return 1
	}
	return 0
}

func paramNames(params []agent.ParameterConfig) []string {
	if len(params) == 0 {
		return nil
	}
	names := make([]string, 0, len(params))
	for _, p := range params {
		if p.Name != "" {
			names = append(names, p.Name)
		}
	}
	return names
}

func SearchToolDefinition() openai.Tool {
	return openai.Tool{
		Type: "function",
		Function: &openai.FunctionDefinition{
			Name:        ToolSearchName,
			Description: "Search available tools by name or description.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Search query.",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of tools to return.",
					},
				},
				"required": []string{"query"},
			},
		},
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func writeAgentFile(t *testing.T, dir, filename, content string) string {
	t.Helper()
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write agent file: %v", err)
	}
	return path
}

func TestAgentInheritanceSingleParent(t *testing.T) {
	tmpDir := t.TempDir()

	parentPath := writeAgentFile(t, tmpDir, "parent.toml", `
name = "parent"
description = "parent desc"
system_prompt = "Parent prompt"
ask = "unsafe"
default_model = "gpt-parent"
think = true

[[functions]]
name = "foo"
description = "parent foo"
command = "echo parent"
safe = true

[mcp_servers.parent]
command = "parent-cmd"
args = ["--p"]
`)

	_ = parentPath

	childPath := writeAgentFile(t, tmpDir, "child.toml", `
extends = "parent.toml"
description = "child desc"
system_prompt = "Child prompt"
think = false

[[functions]]
name = "foo"
description = "child foo"
command = "echo child"
safe = false

[[functions]]
name = "bar"
description = "child bar"
command = "echo bar"
safe = true

[mcp_servers.child]
command = "child-cmd"
args = ["--c"]
`)

	agent, err := Load(childPath)
	if err != nil {
		t.Fatalf("loadAgent error: %v", err)
	}

	if agent.Name != "parent" {
		t.Fatalf("expected inherited name 'parent', got %q", agent.Name)
	}
	if agent.Description != "child desc" {
		t.Fatalf("expected child description override, got %q", agent.Description)
	}
	if agent.SystemPrompt != "Parent prompt\n\nChild prompt" {
		t.Fatalf("unexpected system prompt: %q", agent.SystemPrompt)
	}
	if agent.DefaultModel != "gpt-parent" {
		t.Fatalf("expected inherited default model, got %q", agent.DefaultModel)
	}
	if agent.Think == nil || *agent.Think != false {
		t.Fatalf("expected child think=false override")
	}

	if len(agent.Functions) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(agent.Functions))
	}
	if agent.Functions[0].Name != "foo" || agent.Functions[0].Command != "echo child" {
		t.Fatalf("expected foo overridden by child, got %+v", agent.Functions[0])
	}
	if agent.Functions[1].Name != "bar" {
		t.Fatalf("expected bar appended, got %+v", agent.Functions[1])
	}

	if _, ok := agent.MCPServers["parent"]; !ok {
		t.Fatalf("expected inherited mcp server 'parent'")
	}
	if _, ok := agent.MCPServers["child"]; !ok {
		t.Fatalf("expected child mcp server 'child'")
	}
}

func TestAgentInheritanceCycle(t *testing.T) {
	tmpDir := t.TempDir()

	writeAgentFile(t, tmpDir, "a.toml", `
extends = "b.toml"
name = "a"
`)
	writeAgentFile(t, tmpDir, "b.toml", `
extends = "a.toml"
name = "b"
`)

	_, err := Load(filepath.Join(tmpDir, "a.toml"))
	if err == nil {
		t.Fatalf("expected cycle error, got nil")
	}
}

func TestAgentInheritanceMissingParent(t *testing.T) {
	tmpDir := t.TempDir()

	childPath := writeAgentFile(t, tmpDir, "child.toml", `
extends = "missing-agent"
name = "child"
`)

	if _, err := Load(childPath); err == nil {
		t.Fatalf("expected missing parent error, got nil")
	}
}

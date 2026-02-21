# 002 - Think Tag Filter

**Date:** 2026-02-19
**Phase:** Quality of life
**Context:** tabbyAPI + Qwen3 integration

---

## What happened

When using esa with Qwen3 models via tabbyAPI, the model outputs `<think>...</think>`
blocks as raw text in the response stream. This is Qwen3's chain-of-thought reasoning
feature. Even with `/no_think` in the system prompt, the model still emits empty
`<think>\n</think>` tags.

## What we did

Added `filterThinkTags()` in `application.go` — a streaming-safe state machine that
strips `<think>...</think>` blocks from model output before display and history storage.

Key design decisions:
- **Streaming-safe**: Tags can arrive split across arbitrary chunk boundaries (even
  character by character) and are handled correctly via a buffer + state flag.
- **No false positives**: Partial `<` sequences that turn out not to be think tags
  are flushed back to output.
- **Applied at the stream level**: Filtered content never reaches display or saved
  conversation history.

## Files changed

- `application.go` — Added `filterThinkTags()` function; integrated into
  `handleStreamResponse()` with buffer flush after stream ends.
- `application_test.go` — Added `TestFilterThinkTags` with 8 test cases covering:
  empty blocks, content blocks, no-think-block passthrough, tags split across chunks,
  char-by-char streaming, content before/after blocks, multiple blocks, and partial
  non-think tags.

## Tests

All 8 cases pass. Full suite (`go test ./...`) clean.

## Part 2: Think toggle (same session)

Added `--think` / `--no-think` CLI flags and `think` agent TOML field to control
whether the model uses chain-of-thought reasoning per request.

### Resolution order

1. CLI flag (`--think` or `--no-think`) — highest priority
2. Agent config (`think = true/false` in TOML)
3. Default: `true` (let the model think)

### How it works

When thinking is disabled, `/no_think` is appended to the system prompt. The
`filterThinkTags` filter from Part 1 still runs regardless, so any stray think
tags are always stripped from output.

### Usage

```bash
# Agent default (thinking on by default)
esa "what time is it"

# Disable thinking for quick queries
esa --no-think "what time is it"

# Enable thinking for complex queries
esa --think "analyze this codebase and suggest improvements"

# Set default per agent in TOML
# think = false
```

### Additional files changed

- `agent.go` — Added `Think *bool` field to `Agent` struct
- `cli.go` — Added `Think` and `NoThink` bool fields to `CLIOptions`; registered
  `--think` and `--no-think` flags
- `application.go` — Added `thinkEnabled *bool` field, `shouldThink()` method;
  updated `getSystemPrompt()` to append `/no_think` when thinking is off

### Tests

Full suite (`go test ./...`) clean.

## Notes

- This filter is model-agnostic — any model that emits `<think>` tags will benefit.
- The think content is discarded entirely. A future option could log it in debug mode
  if someone wants to inspect the model's reasoning.
- Separate from this: Qwen3 tool calling via tabbyAPI may require setting the correct
  `tool_prompt_format` in tabbyAPI's model config (e.g., `hermes`). If tool calls
  arrive as XML text instead of structured OpenAI tool_calls, that's a tabbyAPI config
  issue, not an esa issue.

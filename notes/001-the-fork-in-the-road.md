# 001 - The Fork in the Road

**Date:** 2026-02-05
**Phase:** 0 → 1 (Foundation)
**Vibe:** Fresh start energy. That feeling when you clone something good and see what it could become.

---

## What happened

ESA already works. It connects LLMs to shell commands through TOML-configured agents,
and it does it well. Clean Go, ~4900 lines, five direct dependencies. Abin Simon
built something that respects your time.

But ESA agents live alone. You write a killer Kubernetes agent, and it sits in your
`~/.config/esa/agents/` like a journal nobody reads. The fork - esa-buddy - is about
making agents social. A Hub where you publish, discover, and install agents the way
you'd `go install` a tool.

Today we laid the foundation. Not the Hub itself - that's Phase 2. Today was about
asking the right questions and making decisions that won't haunt us later.

## Decisions made

| Decision | Choice | Why |
|----------|--------|-----|
| First feature | Token Tracking | Quick win. go-openai already returns usage data - we just need to catch it. |
| Inheritance model | Flatten at load time | Simpler. Resolve parent→child once when loading. No runtime surprises. |
| Code layout | Introduce packages | New code goes in `internal/`. Existing main package stays put for now. |
| HTTP client | stdlib `net/http` | Zero new deps. We already have retry patterns in `application.go`. |
| License | Keep Apache 2.0 | Respect the upstream. |

## Vibe check

The codebase reads well. Naming is consistent. The agent/function/parameter hierarchy
is intuitive. `cobra` for CLI, `toml` for config, `go-openai` for LLM calls - each
dependency earns its place.

What I notice: the `stats.go` already has the scaffolding for token tracking (Tokens
field in DayStats, AgentStats, ModelStats) but nothing populates it yet. The stream
handler in `application.go` doesn't capture usage from the final chunk. That's our
opening.

The conversation history JSON stores messages but not token counts. We'll need to
extend `ConversationHistory` without breaking existing history files - backward
compatibility matters when people have real conversations saved.

## What's next

Token tracking. The plan:
1. Enable `StreamOptions.IncludeUsage` in chat completion requests
2. Capture usage from the final stream chunk
3. Store token counts in conversation history
4. Wire it into the existing stats display

Small surface. Big signal.

---

*"The best tools disappear into your workflow. The best communities appear when
you least expect them."*

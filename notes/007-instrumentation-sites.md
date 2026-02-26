# 007 - Instrumentation Sites: Data Movement and Event Coverage

**Date:** 2026-02-25
**Phase:** 1 — Instrumentation
**Context:** The telemetry interface (`internal/telemetry/telemetry.go`) and five
emitted event types already exist. This note catalogs every site where data moves
through the pipeline, maps current coverage, and identifies gaps that need events.

---

## Architecture Orientation

Data flows through esa in one direction:

```
[stdin / CLI / agent config]
        │
        ▼
  processInput()              ← user message origin
        │
        ▼
    ingest()                  ← Reference Monitor choke point (Anderson 1972)
        │  (transform pipeline runs here)
        ▼
  app.messages[]              ← conversation store
        │
        ▼
buildRequestMessages()        ← effective context assembly (+ compaction summary)
        │
        ▼
   LLM API call               ← retry loop wraps this
        │
        ▼
handleStreamResponse()        ← assistant message assembled from stream
        │
        ▼
    ingest()                  ← assistant message enters store
        │
        ▼
  dispatcher.Dispatch()       ← tool calls extracted
        │
  ToolGate.Evaluate()         ← security gate decision
        │
  ExecuteFunction / MCP call  ← tool execution
        │
        ▼
    ingest()                  ← tool result enters store
        │
        ▼
saveConversationHistory()     ← history persisted
```

`ingest()` is the single write choke point for all of `app.messages`.
`buildRequestMessages()` is the single read choke point before every LLM call.
These two are the natural telemetry attachment points for context-level events.

---

## Currently Emitted Events

| Event | Method | Site | File:Line |
|-------|--------|------|-----------|
| Turn start | `TurnStarted` | Before LLM call loop | `application.go:956` |
| Turn complete | `TurnCompleted` | After stream consumed | `application.go:969` |
| Tool call start | `ToolCallStarted` | Before gate evaluation | `dispatcher.go:120` |
| Tool call complete | `ToolCallCompleted` | After tool execution | `dispatcher.go:203` |
| Compaction | `Compaction` | After summary generated | `application_compaction.go:132` |
| Retry | `Retry` | On rate-limit 429 | `application.go:407` |
| Error (stream) | `Error` | Stream/chat failure | `application.go:981` |
| Error (tool exec) | `Error` | Tool execution failure | `dispatcher.go:163,299` |

---

## Gap Analysis: Uninstrumented Data-Movement Sites

### 1. Tool Gate Decision
**Where:** `dispatcher.go:137` — `d.deps.ToolGate.Evaluate(intent)`
**Data moving:** Tool name, decision outcome (Allow / Deny), error
**Current handling:** `debugPrint` only — invisible to any telemetry sink
**Why it matters:** Gate denials are security-relevant. Frequency of denials,
which tools are blocked, and policy errors are dashboardable signals. A spike
in denials could indicate a prompt injection attempt.
**Proposed event:** `GateDecision(ctx GateDecisionContext)` on `Telemetry` interface
```go
type GateDecisionContext struct {
    ToolName string
    Decision string // "allow" | "deny" | "error"
    Stage    string // which gate in the chain decided
}
```
Note: `ToolCallStarted` fires *before* the gate. A denial means
`ToolCallCompleted` never fires — the pair is broken. `GateDecision` fills that gap.

---

### 2. Message Ingest (Reference Monitor)
**Where:** `application.go:207` — `func (app *Application) ingest(...)`
**Data moving:** Every message entering `app.messages`, post-transform
**Current handling:** Silent
**Why it matters:** `ingest()` is where the transform pipeline runs. Observing
message role distribution, content size, and transform application makes the
pipeline inspectable. Also enables counting messages-per-turn without walking
the full slice.
**Proposed event:** `MessageIngested(ctx MessageIngestContext)` — or simpler,
instrument `ingest()` inline before the append, emitting role + content size.
```go
type MessageIngestContext struct {
    Role        string
    ContentSize int
    Source      string // "user_input" | "assistant" | "tool_result" | "system"
}
```
This is a high-frequency event. Route to a separate low-cost sink or gate behind
a debug flag to avoid noise in production dashboards.

---

### 3. Effective Context Snapshot (turn envelope)
**Where:** `application.go:422` — `buildRequestMessages(...)`, called once per
LLM request, assembles the full message slice the model will see.
**Data moving:** Final message list, compaction summary presence, effective size
**Current handling:** Silent
**Why it matters:** `TurnStarted` captures turn index and model. It does not
capture how large the context is, whether a compaction summary is present, or
what the effective token estimate is at the moment of the call. This is the
"context health" signal.
**Proposed approach:** Extend `TurnContext` or emit a separate snapshot before
the API call:
```go
type ContextSnapshotContext struct {
    TurnIndex          int
    MessageCount       int
    HasCompactionSummary bool
    EstimatedTokens    int
    Model              string
}
```
Alternative: add fields to `TurnContext` directly (lower interface churn).

---

### 4. Input Source Discrimination
**Where:** `application.go:764-778` — `processInput(commandStr, input string)`
**Data moving:** User message, but origin (stdin / CLI arg / agent InitialMessage)
is lost by the time `ingest()` is called.
**Current handling:** All three paths call `message.New("user", ...).Build()`
with no source annotation.
**Why it matters:** Distinguishing piped stdin from typed CLI input from an
agent's canned initial prompt matters for security analysis (note 006 — stdin
is the untrusted surface) and for understanding conversation start patterns.
**Proposed approach:** The `MessageIngestContext.Source` field above covers this
if the source is threaded into the ingest call, or if `processInput` emits its
own signal before calling `ingest`.

---

### 5. MCP Server Lifecycle
**Where:** `mcp.go:479` — `client.Start(ctx)` inside `StartServers`
**Data moving:** Server name, process start outcome, tool list discovery
**Current handling:** Error returns to `application.go:718` which calls
`log.Fatalf` — no telemetry event before process exit.
**Why it matters:** MCP server crashes or slow starts are operational failures.
Knowing which server failed and how quickly it started feeds SLO measurement.
**Proposed events:**
- `MCPServerStarted(name, toolCount int)` — after successful start and tool discovery
- `MCPServerStopped(name, reason string)` — on shutdown or crash
- These can be added to the `Telemetry` interface or handled as `Error` events
  with `Stage: "mcp_start"` in the short term.

---

### 6. Tool Search / Selection Change
**Where:** `dispatcher.go` — `d.deps.SetToolSelection(names)` called when the
model invokes the tool-search function.
**Data moving:** New tool selection set (names), previous selection size
**Current handling:** Silent
**Why it matters:** Tool search narrowing is a model behavior signal. If the
model is repeatedly narrowing to the same tools, that's a usage pattern. If it
never narrows, tool search may not be effective.
**Proposed event:** Can be captured as an annotation on `ToolCallCompleted` for
the `search_tools` tool call, or as a dedicated event. Low priority — emit as
a debug-level log initially.

---

### 7. History I/O
**Where:** `saveConversationHistory()` after each turn; history load at startup
**Data moving:** Conversation state to/from disk
**Current handling:** Silent on success; errors surface via `log` or return values
that may be ignored.
**Why it matters:** Silent history write failures mean conversation state is lost
without the user knowing. At minimum, errors should route through `telemetry.Error`.
**Proposed approach:** Wrap save/load errors in `app.telemetry.Error(ErrorContext{Stage: "history_save"})` before returning.

---

### 8. Fatal Log Sites (process-terminating errors)
**Where:** Seven `log.Fatalf` calls — none emit a telemetry event before exit:

| File | Line | Trigger |
|------|------|---------|
| `application.go` | 718 | MCP server start failure |
| `application.go` | 728 | System prompt processing failure |
| `application.go` | 758 | _(general error path)_ |
| `application.go` | 774 | Initial message processing failure |
| `dispatcher.go`  | 92  | No matching function for tool call name |
| `llm/model.go`   | 44  | Invalid model format string |

**Why it matters:** A `log.Fatalf` is a hard crash. Without a preceding
`telemetry.Error`, the event is invisible to any OTel/Loki sink. At minimum,
replace with `app.telemetry.Error(...); log.Fatalf(...)`.

---

## Cross-Cutting Concern: The Missing Turn Envelope

The most valuable single addition is a **turn envelope** — a context snapshot
emitted once per turn that captures everything an operator needs to understand
what the model saw:

```
TurnEnvelope {
    TurnIndex            int
    Model                string
    Provider             string
    MessageCount         int
    HasCompactionSummary bool
    EstimatedTokens      int
    ToolsAvailable       int
    ToolsSelected        int  // after search narrowing
}
```

`TurnStarted` is already the right attachment point. Extending `TurnContext`
with these fields is lower-churn than a new event type. The data is all
available at the call site in `application.go` at the moment `TurnStarted` fires.

---

## Event Priority Order

| Priority | Gap | Reason |
|----------|-----|--------|
| 1 | Turn envelope (extend `TurnContext`) | Single addition, maximum observability gain |
| 2 | Tool gate decision | Security-relevant; fills the broken `ToolCallStarted`/`ToolCallCompleted` pair |
| 3 | Fatal log sites | Low-effort; prevents silent crash blindness |
| 4 | History I/O errors | Low-effort; prevents silent data loss |
| 5 | MCP server lifecycle | Operational health; unblocks MCP SLOs |
| 6 | Message ingest | High-frequency; add behind debug flag |
| 7 | Input source discrimination | Security value; lower urgency than gate |
| 8 | Tool search selection | Usage analytics; lowest priority |

---

## What This Enables Once Done

- **Turn-level traces**: duration, context size, model, compaction state, tools available
- **Gate audit log**: every allow/deny decision, correlatable with turn index
- **Crash attribution**: process exits emit structured error before log.Fatalf
- **MCP health dashboard**: server start latency, tool count, crash rate
- **Context pressure timeline**: token estimates per turn, compaction trigger frequency

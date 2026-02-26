# 007 - Instrumentation Sites: Consolidated Plan (v3)

**Date:** 2026-02-25
**Phase:** 1 — Instrumentation
**Supersedes:** `007-instrumentation-sites.md`, `007-instrumentation-sites-2.md`
**Status:** Planning — not yet implemented

---

## Architecture Orientation

Data flows through esa in one direction:

```
[stdin / CLI / agent config]
        │
        ▼
  processInput()              ← user message origin (source lost here)
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

---

## Resolved Architectural Question: Sink Injection vs EventBus

Doc 2 recommends considering a small `EventBus` (Observer pattern, GoF) so
emitters do not hold sink references directly. The current model injects
`telemetry.Telemetry` via `Deps` — a Strategy pattern that works fine today.

**Decision: retain direct injection for now. Defer EventBus.**

Reason: `Fanout` already provides multi-sink dispatch. An EventBus adds async
queue semantics, which creates the flush problem doc 2 correctly flags — events
in a buffer at crash time are lost. The synchronous Strategy is safer here.

**If** async event delivery is ever needed (e.g., remote OTel export with
batching), wrap it inside a sink implementation, not in the emitter layer. The
emitter interface stays synchronous; the sink decides buffering internally.

⚠️ **Open ambiguity:** This decision means telemetry calls are on the hot path
of every turn. For high-frequency events (ingest), this can add measurable
latency if sinks do any I/O. Must be enforced by contract: all `Telemetry`
implementations must be non-blocking or document their latency.

---

## Critical Addition: Correlation ID

**This was missing from v1 and is the most important structural change.**

Without a common ID threaded through all events for a given turn, events are
unjoined points. You cannot reconstruct "what happened in turn 3" from OTel
spans or Loki logs. Every event context struct must carry a `TurnID`.

```go
// TurnID is a per-turn correlation handle. It is assigned once in
// telemetryTurnStarted and propagated to all event contexts within that turn.
// Format: UUIDv7 (time-ordered, from newMessageID).
type TurnID string
```

**Where it lives:** On `Application` as `currentTurnID string`, assigned at
`telemetryTurnStarted`, cleared at `telemetryTurnCompleted`. Every context
struct that can occur within a turn carries it.

**Structs that need `TurnID` added:**

| Struct | Reason |
|--------|--------|
| `TurnContext` | Already the turn anchor |
| `ToolCallContext` | Tool calls happen within a turn |
| `ErrorContext` | Errors need turn correlation |
| `RetryContext` | Retries happen within a turn's LLM call |
| `CompactionContext` | Compaction is triggered within a turn |
| `GateDecisionContext` | (new — see below) |
| `MessageIngestContext` | (new — gated/debug only) |
| `ContextSnapshotContext` | (new — see below) |

⚠️ **Ambiguity:** MCP lifecycle events (server start/stop) span multiple turns
or occur outside the turn loop entirely. They should carry a `SessionID` or
`ServerName` rather than a `TurnID`. This distinction is not yet defined.

---

## Currently Emitted Events

| Event | Method | Site | File:Line | TurnID needed |
|-------|--------|------|-----------|---------------|
| Turn start | `TurnStarted` | Before LLM call loop | `application.go:956` | ✓ (is the source) |
| Turn complete | `TurnCompleted` | After stream consumed | `application.go:969` | ✓ |
| Tool call start | `ToolCallStarted` | Before gate evaluation | `dispatcher.go:120` | ✓ add |
| Tool call complete | `ToolCallCompleted` | After tool execution | `dispatcher.go:203` | ✓ add |
| Compaction | `Compaction` | After summary generated | `application_compaction.go:132` | ✓ add |
| Retry | `Retry` | On rate-limit 429 | `application.go:407` | ✓ add |
| Error (stream) | `Error` | Stream/chat failure | `application.go:981` | ✓ add |
| Error (tool exec) | `Error` | Tool execution failure | `dispatcher.go:163,299` | ✓ add |

---

## Gap Analysis: Uninstrumented Data-Movement Sites

### Gap 1 — Tool Gate Decision
**Where:** `dispatcher.go:137` — `d.deps.ToolGate.Evaluate(intent)`
**Data moving:** Tool name, decision outcome (Allow / Deny), error
**Current handling:** `debugPrint` only

**Why it matters:** This is the only security-relevant decision in the pipeline.
Denials are the signal that a policy is working (or that something is probing it).
A `ToolCallStarted` without a following `ToolCallCompleted` is currently
uninterpretable — was it denied? did it hang? `GateDecision` closes that gap.

```go
type GateDecisionContext struct {
    TurnID   TurnID
    ToolName string
    Decision string // "allow" | "deny" | "error"
    Stage    string // which gate in the chain decided; use Stage enum (see §Error Taxonomy)
}
```

⚠️ **Ambiguity:** The dispatcher does not currently have access to `TurnID`.
It receives it indirectly if `TurnContext` is threaded through `Deps`.
Options: (a) add `CurrentTurnID func() TurnID` to `Deps`, or (b) set
`currentTurnID` on the application and pass it into `ToolCallContext` before
the dispatcher call. Neither is obvious from the current API shape.

---

### Gap 2 — Effective Context Snapshot (turn envelope)
**Where:** `application.go:422` — `buildRequestMessages(...)`, called once per
LLM request.

**Why it matters:** `TurnStarted` captures turn index and model but not context
health. An operator at 2 AM needs to know: how full is the context, is there a
compaction summary, how many messages is the model seeing?

**Proposed: extend `TurnContext`** (lower interface churn than a new event type):

```go
type TurnContext struct {
    TurnID               TurnID
    TurnIndex            int
    MessageCount         int
    Provider             string
    Model                string
    HasCompactionSummary bool
    EstimatedTokens      int
    ToolsAvailable       int
    ToolsSelected        int  // after search narrowing
}
```

⚠️ **Risk flagged by doc 2:** The data for `TurnContext` is spread across
multiple layers. `EstimatedTokens` requires `tokenizer.CounterProvider`.
`ToolsAvailable` comes from the agent definition after MCP merge. Both are
available on `Application` but not at the same call site. `telemetryTurnStarted`
may need to be called slightly later in the setup sequence, or take additional
parameters. This is a light but real refactor — do not treat it as zero-cost.

---

### Gap 3 — Compaction "Skipped" Decision
**Where:** `application_compaction.go` — `shouldCompact()` returns bool.

This was not in v1. Doc 2 correctly identifies it.

**Why it matters:** Knowing that compaction was evaluated and *not* triggered is
as useful as knowing it was. It lets you tune thresholds: if compaction never
triggers, the threshold may be too high; if it triggers every turn, too low.

```go
type CompactionSkippedContext struct {
    TurnID         TurnID
    Reason         string // "below_token_threshold" | "below_msg_threshold" | "disabled"
    EstimatedTokens int
    Threshold      int
}
```

⚠️ **Structural constraint:** `shouldCompact()` currently returns only `bool`.
It has no access to `app.telemetry`. To emit here, either:
(a) `shouldCompact` returns a reason value and the caller emits, or
(b) `app.telemetry` is available as a receiver method (it is — `shouldCompact`
is called on `*Application`).

Option (b) is cleaner. The call site in the compaction flow already has
`app.telemetry` in scope.

---

### Gap 4 — Tool Output Size
**Where:** `ToolCallCompleted` in `dispatcher.go:203`.

This was not in v1. Doc 2 flags it as needed for the artifact plan.

`ToolCallContext` currently captures `ToolName` and `ArgsSize`. It does not
capture the result size.

```go
type ToolCallContext struct {
    TurnID     TurnID
    ToolName   string
    ArgsSize   int
    ResultSize int  // add: byte length of tool output content
    Error      string // add: non-empty if the call failed
}
```

**Why it matters now:** Before artifact virtualization can be implemented
(Phase 1 §6), the size distribution of tool outputs must be observable. Without
this signal, the threshold for "large enough to virtualize" is a guess.

---

### Gap 5 — Retry Timing Per Turn
**Where:** `application.go:407` — `RetryContext` is already emitted but missing
per-turn retry count.

`RetryContext` has `Attempt`, `Max`, `Delay`, `Error`. It does not have a
`TurnID` (per the TurnID analysis above), and it does not record the
*total* delay accumulated within a turn (which is what matters for SLO
measurement — not the per-retry delay).

**Proposed addition to `RetryContext`:**
```go
type RetryContext struct {
    TurnID  TurnID
    Attempt int
    Max     int
    Error   string
    Delay   time.Duration
    // TotalDelayThisTurn would require tracking on Application; deferred.
}
```

Adding `TurnID` here is the minimum viable change. Total delay per turn is a
derived metric that Prometheus can compute from the retry event stream.

---

### Gap 6 — Fatal Log Sites (process-terminating errors)
**Where:** Seven `log.Fatalf` calls — none emit telemetry before exit.

| File | Line | Trigger |
|------|------|---------|
| `application.go` | 718 | MCP server start failure |
| `application.go` | 728 | System prompt processing failure |
| `application.go` | 758 | General error path |
| `application.go` | 774 | Initial message processing failure |
| `dispatcher.go`  | 92  | No matching function for tool call |
| `llm/model.go`   | 44  | Invalid model format string |

**Fix:** emit `app.telemetry.Error(...)` immediately before each `log.Fatalf`.

⚠️ **Flush risk (doc 2):** If the telemetry sink buffers events (e.g., an
async OTel exporter), calling `Error` then `log.Fatalf` will lose the event.
**Resolution:** The synchronous-sink constraint established above means this is
safe as long as no async sink is wired. Document this in the `Telemetry`
interface contract: *implementations must complete writes before returning*.

Note: `llm/model.go:44` is not on `Application` — it has no `app.telemetry`
in scope. This site can use a package-level logger (`slog`) with a structured
error key as a fallback, or the call site should be moved to where the model is
resolved on `Application`.

---

### Gap 7 — Error Taxonomy (Stage Enum)
**Not a new event site — a schema problem across all error events.**

`ErrorContext.Stage` is currently a free string. Existing values observed:
`"stream_response"`, `"tool_exec"`, `"mcp_tool"`. No canonical list exists.
Grafana dashboards built on free strings will fragment as new stages are added.

```go
// ErrorStage is an enumeration of pipeline stages where errors are classified.
// Add values here as new stages are instrumented; do not use ad-hoc strings.
type ErrorStage string

const (
    StageStreamResponse ErrorStage = "stream_response"
    StageToolExec       ErrorStage = "tool_exec"
    StageMCPTool        ErrorStage = "mcp_tool"
    StageMCPStart       ErrorStage = "mcp_start"
    StageHistorySave    ErrorStage = "history_save"
    StageHistoryLoad    ErrorStage = "history_load"
    StageSystemPrompt   ErrorStage = "system_prompt"
    StageInputProcess   ErrorStage = "input_process"
)
```

Change `ErrorContext.Stage string` → `ErrorContext.Stage ErrorStage`.

---

### Gap 8 — MCP Server Lifecycle
**Where:** `mcp.go:479` — `client.Start(ctx)` inside `StartServers`

MCP has no telemetry dependency today. Adding it requires either:
(a) Passing `telemetry.Telemetry` into `MCPManager`, or
(b) Handling MCP lifecycle events at the `application.go` call site (line 718)
where the result of `StartServers` is already handled.

Option (b) is lower coupling — the application emits on behalf of the MCP layer.

```go
type MCPLifecycleContext struct {
    ServerName string
    Event      string // "started" | "stopped" | "start_failed"
    ToolCount  int    // populated on "started"
    Error      string // populated on "start_failed"
}
```

⚠️ **Ambiguity:** These events are session-scoped, not turn-scoped.
They should not carry `TurnID`. A `SessionID` (assigned once per process
invocation) would be the right correlation handle. No `SessionID` concept
currently exists. This is a prerequisite for MCP lifecycle events to be
joinable with anything else in a trace. Decide: add `SessionID` to the
`Telemetry` context model, or leave MCP as standalone events for now.

---

### Gap 9 — Sampling Policy for High-Frequency Events
**Only applies to Gap 1 (MessageIngest) if implemented.**

Doc 2 contains an informal note: *"Mario's answer - take 5%? for HF spikes =
just rate limit samples?"* This is unresolved.

**Critique of the 5% idea:** A fixed sampling rate is problematic in a CLI tool
with short sessions. At 5%, a 10-message conversation might emit 0 or 1 ingest
events — statistically useless. A rate-limit (e.g., max N events per turn) is
more appropriate here than probabilistic sampling.

**Proposed:** Gate `MessageIngested` behind a `debug` flag on the telemetry sink
rather than sample it. Debug-mode sinks emit all events; production sinks skip
the high-frequency ones entirely. This is simpler than sampling and keeps the
interface clean.

**This decision must be made before implementing Gap 2 (MessageIngest).**
Until resolved, MessageIngest is deferred.

---

### Gap 10 — Input Source Discrimination
**Where:** `application.go:764-778` — `processInput(commandStr, input string)`

All three input origins (stdin pipe, CLI arg, agent `InitialMessage`) produce
identical `"user"` role messages. Origin is lost before `ingest()`.

This is lower priority than the above gaps but relevant to the security picture
from note 006: stdin is the untrusted surface. Capturing source in `MessageIngestContext`
(if that event is implemented) covers this. Alternatively, the Builder can carry
a `Source` annotation that the envelope transform tags into the message metadata.

---

## Schema Stability

⚠️ **Unresolved ambiguity from doc 2:** Are event context types a public
contract or internal?

Recommendation: treat them as **internal** for now. They are in
`internal/telemetry` and not exported from the module. The `Telemetry` interface
is the stable boundary; struct fields can evolve freely inside the package.
If an OTel exporter or Prometheus sink is written, its field mapping is
its own adapter concern — not a constraint on the context structs.

---

## Implementation Priority

| Priority | Work item | Risk | Effort |
|----------|-----------|------|--------|
| 1 | Add `TurnID` to `TurnContext`; assign in `telemetryTurnStarted` | Low | Small |
| 2 | Propagate `TurnID` to all existing event structs | Low | Small |
| 3 | Define `ErrorStage` enum; replace free strings | Low | Small |
| 4 | Extend `TurnContext` with context snapshot fields | Medium (data availability) | Medium |
| 5 | `GateDecisionContext` + emit at `ToolGate.Evaluate` | Medium (TurnID threading) | Medium |
| 6 | Fatal log sites: emit `Error` before `log.Fatalf` | Low | Small |
| 7 | History I/O error telemetry | Low | Small |
| 8 | Add `ResultSize` to `ToolCallContext` | Low | Small |
| 9 | `CompactionSkippedContext` + emit from `shouldCompact` | Low | Small |
| 10 | MCP lifecycle events (defer until `SessionID` resolved) | Medium | Medium |
| 11 | `MessageIngested` event (defer until sampling policy resolved) | High (noise) | Small |
| 12 | Input source discrimination | Low | Small |

Items 1–3 are prerequisites for everything else. They are structural and
should ship together as a single commit.

---

## Open Questions (require decisions before implementation)

1. **TurnID threading to dispatcher:** Which mechanism — `Deps` callback or
   field on `ToolCallContext` pre-populated by `Application`?

2. **SessionID:** Introduce it for MCP lifecycle and session-scoped events,
   or skip and leave MCP events uncorrelated?

3. **Sampling policy for MessageIngest:** Debug-mode gating (recommended above)
   or something else? Must resolve before implementing.

4. **`llm/model.go` fatal site:** No `Application` in scope — handle via
   `slog`, restructure, or leave for now?

5. **Flush contract enforcement:** Where does the interface doc live that
   enforces synchronous-write semantics on `Telemetry` implementations?

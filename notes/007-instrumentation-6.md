# 007 - Instrumentation Sites: Consolidated Plan (v6)

**Date:** 2026-02-25
**Phase:** 1 — Instrumentation
**Supersedes:** all prior 007 documents
**Status:** Planning — next decisions identified below

---

## What Changed Since v3

v3-2 / v3-4 (same document, two drafts) corrected v3:

1. **TurnID is UUID v4, not v7.** `newMessageID` calls `uuid.New()` which is v4.
   The v3 claim of "time-ordered UUIDv7" was wrong. Either use v4 and drop the
   ordering requirement, or change the generator. This is a pre-implementation
   decision — see §Decisions below.

2. **`shouldCompact` must stay pure.** Emit the compaction skip event at the
   call site, not inside `shouldCompact`. The function returns a reason value;
   the caller emits. Preserves testability.

3. **"Non-blocking" is now defined.** Mario's clarification: *"don't hold the
   main path up; IO or compute can happen asynchronously through the native
   scheduler."* The async enqueue pattern satisfies this. The interface contract
   is: callers must not block on network I/O. Internal buffering is the sink's
   concern.

4. **ToolCallStarted missing completion** does not only mean gate denied. It
   can also mean crash, panic, or fatal exit. `GateDecision` fills the gate
   case but does not cover the others. The doc must not imply it does.

**v5 introduced `gocraft/work` + Redis as an async telemetry path.** This is
the architecturally significant change that reshapes several v3 assumptions.

---

## The gocraft/work Introduction: What It Actually Changes

### What was built (per v5)
- A `gocraft/work`-backed telemetry sink that enqueues events into Redis.
- Added as a `Fanout` target when enabled.
- Async, non-blocking enqueue via buffered channel.
- Drop-vs-block on full queue is configurable.
- `telemetry_work.enabled = true` by default (opt-out).
- No worker process wired yet — jobs enqueue but nothing consumes them.

### What this breaks from v3

**v3 synchronous-sink assumption is now violated by design.** v3 stated:
"all implementations must complete writes before returning" and used this
to justify the `log.Fatalf` + telemetry pattern. With async enqueue enabled,
that guarantee does not hold. The `log.Fatalf` crash-safety argument collapses.

Specifically: `log.Fatalf` calls `os.Exit(1)`. Deferred functions do not run.
Any `defer flush()` cannot be relied upon. Calling `telemetry.Error(...)` before
`log.Fatalf` with an async sink is a fire-and-hope, not a guarantee.

**v3's EventBus deferral was effectively overridden.** v3 said defer the
EventBus in favor of direct injection. `gocraft/work` is a Redis-backed job
queue — functionally an EventBus. The architectural decision was made in code
before the documentation caught up.

---

## Critical Assessment

### The opt-out default is the wrong stance for a CLI tool.

`telemetry_work.enabled = true` means every user running `esa` without Redis
will hit a failure path unless a soft-disable fallback is airtight. Redis is
a server process. Local development environments do not have it by default.

A CLI tool should assume a minimal runtime environment. The observability
stack is an opt-in concern, not a default dependency. Opt-in defaults for
infrastructure dependencies are standard practice for good reason — they
don't punish users who just want to run the tool.

⚠️ **Decision required:** Reverse to opt-in (`telemetry_work.enabled = false`
by default), or guarantee that the soft-disable fallback is truly silent and
zero-overhead. "Log a warning" is not good enough — warnings become noise users
ignore, and then miss actual errors.

### gocraft/work is architecturally mismatched for a CLI tool.

`gocraft/work` is designed for persistent web services with stable Redis
connections and long-lived worker processes. A CLI invocation is ephemeral —
it starts, runs a conversation, and exits. The job queue pattern assumes:
- A persistent worker is running.
- Jobs are durable and will eventually be consumed.
- The producer and consumer have independent lifecycles.

None of these hold for a CLI tool. Jobs enqueue into Redis and sit there
indefinitely with no consumer. This is not observability — it is operational
debt that accumulates silently in Redis.

⚠️ **Decision required:** Define the worker lifecycle. Three options:

| Option | Description | Trade-off |
|--------|-------------|-----------|
| A. In-process goroutine | Spawn a worker goroutine in the same `esa` process | No external dependency; dies with the process |
| B. Separate daemon | `esa-worker` daemon that runs persistently | Two processes; better for high-throughput multi-session use |
| C. Drop gocraft/work | Replace with an in-process async channel + slog/OTel export | Eliminates Redis dependency; still async |

Option C is the most aligned with the "non-blocking but async via native
scheduler" definition Mario gave. A buffered channel feeding a goroutine that
writes to slog or an OTel collector requires no Redis and no separate process.
The tradeoff is durability — events are lost on crash, but the slog sink
already handles that and we have accepted best-effort delivery.

### Without a running worker, the work telemetry is inert.

The current state: events enqueue into Redis but nothing consumes them.
The slog sink still works. The work telemetry provides no operational value
until a worker is connected. This means the Redis dependency is already
present (assuming opt-out default) with zero benefit. This is the worst of
both worlds.

### Per-event delivery guarantees are now urgent.

v5 asks this as an open question. It is not open — it is the critical
architectural decision that determines how the flush problem is solved.

**Proposed two-tier model:**

| Tier | Events | Delivery | Implementation |
|------|--------|----------|----------------|
| **Guaranteed** | `Error`, `GateDecision` | Synchronous — caller blocks | Direct slog/file write, no queue |
| **Best-effort** | `TurnStarted`, `TurnCompleted`, `ToolCallStarted`, `ToolCallCompleted`, `Compaction`, `Retry` | Async — enqueue and move on | Buffered channel or work queue |

Security-relevant and error events must be synchronous. Everything else
can be best-effort. This maps naturally to a two-sink Fanout:
- Sink A: synchronous slog (always enabled, receives all events)
- Sink B: async work queue (opt-in, receives best-effort events only)

The `Telemetry` interface does not need to change — routing is the sink's
concern. `Fanout` already handles this. The async sink simply does not
receive `Error` and `GateDecision` calls — those only go to the sync sink.

---

## Corrections from v3-2 / v3-4 Applied

### TurnID Generator: UUID v4 vs v7

**v3 error:** Claimed UUIDv7 (time-ordered) from `newMessageID`.
**Reality:** `uuid.New()` generates v4 (random).

**Decision:** Two options:

| Option | Trade-off |
|--------|-----------|
| Use UUID v4, remove ordering requirement | Simpler; IDs are opaque; ordering by timestamp not possible |
| Switch to UUID v7 (`uuid.NewV7()`) | Time-ordered; better for OTel trace correlation; small dependency change |

If OTel trace correlation is a real goal (it is — see TODO §5), v7 is worth
it. The generator change is one line. The decision should be explicit.

### shouldCompact Must Stay Pure

Corrected from v3. The pattern:

```go
// In the caller (application_compaction.go), not inside shouldCompact:
decision, reason := app.shouldCompact()  // returns (bool, CompactionDecision)
if !decision {
    app.telemetry.CompactionSkipped(telemetry.CompactionSkippedContext{
        TurnID:          app.currentTurnID,
        Reason:          reason,
        EstimatedTokens: ...,
        Threshold:       ...,
    })
    return
}
// ... proceed with compaction
```

`shouldCompact` returns a `CompactionDecision` reason enum alongside the bool.
The caller emits. `shouldCompact` remains a pure function — testable without
a telemetry mock.

### ToolCallStarted / Completed Gap: Complete Taxonomy

A `ToolCallStarted` without a subsequent `ToolCallCompleted` can mean:

1. Gate denied → `GateDecision` event with `decision="deny"` will follow
2. Tool execution error → `Error` event will follow
3. Crash / panic / `log.Fatalf` → no further events (the silent failure case)
4. MCP tool call failure (separate dispatch path) → `Error` event may follow

Case 3 is the unresolvable one without a crash reporting mechanism outside the
process boundary. Acknowledge in documentation; do not imply `GateDecision`
covers all missing-completion cases.

---

## Decisions Required Before Implementation

These must be resolved before any code is written. They are ordered by blocking
dependency.

### Decision 1: Opt-out vs Opt-in for Work Telemetry
**Block:** Everything — this determines whether Redis is a dependency.
**Options:**
- A. Reverse to opt-in: `telemetry_work.enabled = false`. Users enable explicitly.
- B. Keep opt-out with guaranteed silent fallback: if Redis unavailable at
  startup, downgrade to Noop with zero noise. No warning logged unless
  `--verbose` or strict flag set.
- C. Remove gocraft/work entirely; replace with in-process async channel.

**Recommendation:** Option A or C. B is fragile — "guaranteed silent fallback"
is harder to get right than it sounds; probe-on-startup race conditions,
reconnection logic, and test coverage all add up.

### Decision 2: Worker Lifecycle
**Block:** gocraft/work usefulness.
**Options:** In-process goroutine / separate daemon / drop gocraft/work.
See table in §Critical Assessment. Recommend evaluating Option C first.

### Decision 3: Per-Event Delivery Tiers
**Block:** `log.Fatalf` safety, flush semantics, sink architecture.
**Recommendation:** Two-tier model described above. `Error` and `GateDecision`
are synchronous-only; all others are best-effort via async path.

### Decision 4: TurnID Generator (v4 vs v7)
**Block:** TurnID implementation.
**Recommendation:** Switch to UUID v7 if OTel is a real target. One-line change.
Document the choice; do not leave it implicit.

### Decision 5: Flush / Close on Process Exit
**Block:** `log.Fatalf` + async telemetry.
**Regardless of Decision 1/2:** Add `Flush()` / `Close()` to the `Telemetry`
interface. Even if the current slog sink is a no-op for `Close`, the hook must
exist so that async sinks can be added later without a crisis.

```go
type Telemetry interface {
    TurnStarted(TurnContext)
    TurnCompleted(TurnContext)
    ToolCallStarted(ToolCallContext)
    ToolCallCompleted(ToolCallContext)
    Compaction(CompactionContext)
    CompactionSkipped(CompactionSkippedContext)  // new
    GateDecision(GateDecisionContext)            // new
    Retry(RetryContext)
    Error(ErrorContext)
    Close() error  // new — flush and release resources
}
```

On every `log.Fatalf` site, call `app.telemetry.Close()` first. This is safe
even for the slog sink (no-op) and mandatory for any async sink.

---

## Unchanged From v3 (Still Valid)

- `TurnID` on all turn-scoped event structs — still the structural prerequisite
- `ErrorStage` enum in `telemetry` package, shared across all emitters
- Extend `TurnContext` with context snapshot fields (`MessageCount`,
  `HasCompactionSummary`, `EstimatedTokens`, `ToolsAvailable`, `ToolsSelected`)
- `ResultSize` on `ToolCallContext` — needed before artifact virtualization
- MCP lifecycle events deferred until `SessionID` is defined
- `MessageIngested` deferred until sampling/gating policy is resolved
- `GateDecisionContext` — still needed, still the same shape

---

## Updated Implementation Priority

Items 1–5 are structural prerequisites. Resolve decisions above first.

| Priority | Work item | Blocked by decision |
|----------|-----------|-------------------|
| 1 | Resolve Decisions 1–5 (no code yet) | — |
| 2 | Add `Close()` to `Telemetry` interface; implement on Noop, Fanout, slog | Decision 5 |
| 3 | Switch TurnID generator (v4 → v7 or document v4) | Decision 4 |
| 4 | Add `TurnID` field to `TurnContext`; assign in `telemetryTurnStarted` | Decision 4 |
| 5 | Propagate `TurnID` to all existing event structs | 4 |
| 6 | Define `ErrorStage` enum; replace free strings in all emitters | 5 |
| 7 | Extend `TurnContext` with snapshot fields | 5 |
| 8 | Add `GateDecision` event + emit at `ToolGate.Evaluate` | 5 |
| 9 | Add `CompactionSkipped` event + emit at caller | 5 |
| 10 | Add `ResultSize` + `TurnID` to `ToolCallContext` | 5 |
| 11 | Wrap fatal log sites: `telemetry.Close()` then `log.Fatalf` | 2 |
| 12 | History I/O error telemetry | 6 |
| 13 | MCP lifecycle events | SessionID defined |
| 14 | `MessageIngested` event | Gating policy resolved |

---

## Open Questions Carried Forward

1. **SessionID:** MCP lifecycle and session-scoped events need it. Still deferred.
2. **`llm/model.go:44` fatal site:** No `Application` in scope. Options: restructure
   call site, use package-level `slog` structured error, or leave for now.
3. **`ToolsAvailable` vs `ToolsSelected` timing:** If tool search narrows
   mid-turn, which snapshot do we capture in `TurnContext`? Snapshot-at-start
   is simpler and avoids mid-turn mutation. Tool selection change events can
   cover the delta separately.
4. **Two-tier sink routing:** If guaranteed events (Error, GateDecision) bypass
   the async work queue, does that require a change to `Fanout` routing, or
   does each sink simply ignore events it does not care about? Explicit routing
   is cleaner but adds interface complexity.

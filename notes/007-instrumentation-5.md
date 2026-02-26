# 007 - Instrumentation Update (v5)

**Date:** 2026-02-26  
**Context:** Follow-up to `007-instrumentation-sites-3-4.md` after introducing
gocraft/work as an opt-out telemetry execution path.

---

## New App State (What Changed)

1. **Telemetry is now multi-sink with a work-queue option**
   - The existing slog telemetry remains.
   - A gocraft/work-backed telemetry sink can enqueue events into Redis.
   - The sink is added as a Fanout target when enabled.

2. **Opt-out default**
   - `telemetry_work.enabled = true` by default.
   - This means a Redis dependency is now assumed unless explicitly disabled.

3. **Async enqueue is supported**
   - Non-blocking enqueue via buffered channel.
   - Drop-vs-block on full queue is configurable.

4. **Telemetry workers are not wired yet**
   - Jobs are enqueued, but no worker process consumes them unless explicitly run.
   - There is no CLI command yet to start a worker pool.

---

## Implications for 007-instrumentation-sites-3-4.md

### 1) Telemetry hot-path latency constraint
**Impact:** We now have a mechanism that can keep the hot path non-blocking.
The requirement becomes enforceable in code: async enqueue + drop policy.

**Correction to doc assumptions:** The doc assumes synchronous sink behavior for
`log.Fatalf` safety. With work telemetry enabled and async enqueue, events may
be buffered and lost on fatal exit unless a flush/stop is added. This needs to
be explicitly acknowledged.

### 2) Event delivery reliability
**Impact:** Telemetry is no longer guaranteed to be delivered on every call.
If `drop_on_full` is enabled (default), events can be dropped when the queue is full.

**Doc update:** The doc should treat telemetry delivery as “best effort” unless
BlockOnFull is true. This is a real tradeoff and should be stated.

### 3) Redis dependency
**Impact:** Opt-out means Redis is now a runtime dependency for telemetry,
unless explicitly disabled. This shifts the operational baseline.

**Doc update:** Add a callout: “telemetry work is enabled by default and
requires Redis unless disabled.”

---

## Insights

1. **Synchronous vs async tension is now real**
   - The doc’s earlier suggestion that “synchronous sinks make fatal events safe”
     is no longer a guarantee when async enqueue is enabled.

2. **Event-driven system is now partial**
   - There is an event queue, but no defined worker lifecycle in the CLI yet.
   - Without a worker, telemetry events accumulate in Redis indefinitely.

3. **Opt-out default is a strong stance**
   - This favors observability by default, but increases operational complexity.
   - It also means local dev without Redis will see failures unless we add
     a soft-disable fallback.

---

## Suggestions to Reach the Objective

1. **Add a telemetry worker command**
   - Example: `esa --telemetry-worker`
   - This worker should bind to the slog sink (or future OTel sink).

2. **Add a soft-disable fallback**
   - If Redis is unavailable on startup, log a warning and disable work telemetry
     unless a strict flag is set.

3. **Document delivery guarantees**
   - Explicitly state when events can drop or be lost.
   - Provide guidance on using `block_on_full` for strict delivery.

4. **Add flush/stop hooks**
   - If async is used, add a `Close()` or `Flush()` method to the work telemetry
     so fatal paths can attempt to drain the queue.

5. **Update 007-instrumentation-sites-3-4.md**
   - Note that “non-blocking telemetry” is now implemented by work enqueue.
   - Add a risk note about async buffering and fatal exits.
   - Add operational guidance around Redis availability.

---

## Open Questions

1. Do we treat Redis absence as a hard error or silent downgrade?
2. Should `telemetry_work.enabled` remain opt-out, or be gated by env/CLI?
3. Do we need per-event delivery guarantees (e.g., errors are always blocking)?

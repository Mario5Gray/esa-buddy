# 007 - Instrumentation Implementation Plan (v7)

**Date:** 2026-02-26  
**Goal:** Convert the v6 requirements into a no-blocker, high‑level execution plan.  
**Scope:** Architectural segments only (no code-level detail).

---

## Segment 1: Runtime Telemetry Strategy (Execution Model)
**Objective:** Decide how telemetry is executed on the hot path vs background.

**Pieces that change**
- Telemetry runtime model (sync vs async, opt‑in/opt‑out).
- Dependency posture (Redis or no Redis by default).
- Delivery guarantees (best‑effort vs guaranteed).

**Deliverables**
- Final decision on Redis dependency (opt‑in or remove).
- Delivery tier definition (which events are guaranteed).
- Documented non‑blocking contract for sinks.

---

## Segment 2: Telemetry Interface Contract
**Objective:** Make the telemetry interface reflect lifecycle needs and delivery guarantees.

**Pieces that change**
- Telemetry interface shape (lifecycle hooks like Close/Flush).
- Event taxonomy stability (ErrorStage enum).
- Correlation model (TurnID; potential SessionID later).

**Deliverables**
- Lifecycle expectations documented in the interface.
- Error stage taxonomy defined and shared.
- TurnID standardized across all turn‑scoped events.

---

## Segment 3: Turn Envelope & Context Health
**Objective:** Provide a consistent “turn envelope” for observability.

**Pieces that change**
- TurnContext fields (context size, compaction presence, tool availability).
- Turn start/complete emission sites to carry envelope data.

**Deliverables**
- A single, enriched turn snapshot per LLM call.
- Clear definition of “when snapshot is taken” (start‑of‑turn).

---

## Segment 4: Security & Control Events
**Objective:** Instrument security‑critical decision points.

**Pieces that change**
- Gate decision event surface (allow/deny/error).
- Error classification into consistent stages.

**Deliverables**
- Gate decisions observable in telemetry.
- Error events mapped to fixed stages (not free strings).

---

## Segment 5: Compaction Telemetry Completeness
**Objective:** Capture both compaction and skipped‑compaction signals.

**Pieces that change**
- Compaction decision reporting (triggered vs skipped).
- Compaction reasons exposed (below threshold, disabled, etc.).

**Deliverables**
- CompactionSkipped event with reasons.
- Compaction thresholds observable at decision time.

---

## Segment 6: Tool Execution Telemetry Completeness
**Objective:** Cover tool execution sizes and outcomes for future artifact work.

**Pieces that change**
- ToolCallContext shape (result size, errors).
- Tool call lifecycle coverage (start/complete symmetry).

**Deliverables**
- Result size captured for tool outputs.
- Clear handling of missing completion cases.

---

## Segment 7: Fatal & History Failure Visibility
**Objective:** Ensure data loss or crash paths are observable.

**Pieces that change**
- Fatal error handling flow.
- History read/write error telemetry.

**Deliverables**
- Telemetry emitted before fatal exits (as allowed by chosen runtime model).
- History save/load failures surfaced as telemetry errors.

---

## Segment 8: Worker Lifecycle (If Redis Queue Retained)
**Objective:** Make queued telemetry actually consumable.

**Pieces that change**
- Worker lifecycle (in‑process vs daemon).
- CLI or service entrypoint for worker execution.

**Deliverables**
- Defined worker lifecycle.
- Documented operational guidance for running workers.

---

## Sequencing (No‑Blocker Path)

1. **Decide runtime model** (Segment 1).  
   This unblocks everything else.  

2. **Lock interface contract** (Segment 2).  
   All event additions depend on this.  

3. **Implement turn envelope** (Segment 3).  
   Gives immediate observability wins.  

4. **Instrument security & compaction** (Segments 4–5).  
   High‑value signals with low architectural risk.  

5. **Tool execution completeness** (Segment 6).  
   Enables artifact planning later.  

6. **Fatal/history visibility** (Segment 7).  
   Low effort, high diagnostic value.  

7. **Worker lifecycle** (Segment 8, only if Redis is retained).  
   Optional depending on the Segment 1 decision.

---

## Definition of Done (High‑Level)

- Telemetry model chosen and documented.
- Turn envelope emitted consistently.
- Security, compaction, and tool telemetry are complete and typed.
- Fatal and history failures are observable.
- If Redis queue remains, a worker lifecycle is defined and runnable.

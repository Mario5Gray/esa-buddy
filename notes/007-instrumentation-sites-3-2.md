# 007 - Instrumentation Sites (v3) Review + Corrections

**Date:** 2026-02-25  
**Status:** Review notes on `007-instrumentation-sites-3.md`  
**Abstain meaning:** Treated as implicit approval by the author; other reviewers may still object explicitly.

---

## High-Level Agreeance
The v3 document is a strong plan. I agree with the overall direction, the priority ordering, and the emphasis on `TurnID`, `TurnContext` enrichment, and error taxonomy. The refactors are scoped and realistic.

---

## Corrections (with reasons)

1. **ToolCallStarted site placement**
   - **Correction:** The doc says `ToolCallStarted` fires “before gate evaluation” and is the event that pairs with `ToolCallCompleted`. That is accurate, but the implication that a missing completion means “denied or hung” could be misleading.
   - **Reason:** A missing completion can also indicate a crash, panic, or fatal path before tool execution. It’s not just gate denial. The new `GateDecision` event helps, but the doc should acknowledge other failure modes.

2. **TurnID assignment source**
   - **Correction:** The doc implies `newMessageID` could be used for `TurnID` and references “UUIDv7 (time-ordered, from newMessageID)”. In code, `newMessageID` uses `uuid.New().String()` which is v4, not v7.
   - **Reason:** If the plan wants time-ordered IDs, the source function must change. Otherwise document TurnID as UUIDv4 and remove the v7 claim.

3. **Telemetry hot-path latency constraint**
   - **Correction:** The constraint “all `Telemetry` implementations must be non-blocking” is strong but under-specified.
   - **Reason:** Some sinks will inevitably do I/O. The real requirement is “callers must not block on network I/O.” Clarify that sinks should either buffer in-memory or write to a local file synchronously, and any remote export must be async internally.

4. **CompactionSkippedContext reasoning**
   - **Correction:** The doc suggests moving compaction skip reasoning into `shouldCompact` with telemetry emitted there.
   - **Reason:** `shouldCompact` is currently pure and stateless. The more minimal change is to return a reason enum (e.g., `CompactionDecision`) and emit at the caller. This preserves testability and avoids hidden side effects.

---

## Additional Insights

- **Correlation beyond TurnID:** Some events are inherently cross-turn (MCP lifecycle, session start). A lightweight `SessionID` will be necessary once those events are added. For now, the doc correctly defers it.
- **Error taxonomy scope creep:** The proposed `ErrorStage` enum is solid but should live in `telemetry` and be shared by all emitters. Avoid defining enums in multiple packages.
- **Tool output sizing:** Adding `ResultSize` now is low-risk and aligns with the artifact plan. Good call.
- **Sampling vs gating:** Debug-only gating for `MessageIngest` is more predictable than probabilistic sampling in a CLI tool. Agree.

---

## Ambiguities That Still Need Clarification

1. **Where does `TurnID` live?**
   - The doc mentions `Application.currentTurnID`. If this is mutable shared state, ensure it is updated exactly once per turn and cleared on completion.

2. **What is the canonical source of `ToolsAvailable` and `ToolsSelected`?**
   - If tool search runs mid-turn, do we record selection before or after the tool-search tool call? The doc implies “after search narrowing,” but the exact timing is not fixed.

3. **Telemetry flush semantics**
   - The doc says synchronous sinks make `log.Fatalf` safe. If any sink does buffered async export, the fatal path will still lose events unless a flush hook is guaranteed.

---

## Agree / Disagree Summary

**Agree:**
- Add `TurnID` to all turn-bound contexts.
- Extend `TurnContext` for envelope metrics.
- Introduce `ErrorStage` enum.
- Add `GateDecision` and `ResultSize`.
- Defer `MessageIngested` until gating policy is resolved.

**Disagree / Caution:**
- Don’t claim UUIDv7 unless we change the generator.
- Keep compaction decision telemetry emitted at the caller, not inside `shouldCompact`.
- Document telemetry sink behavior explicitly (blocking vs async).

---

## Abstain Notes
Where v3 says “abstain,” interpret it as **author-approved**. I would treat these as “approved unless a reviewer objects,” not as neutral.

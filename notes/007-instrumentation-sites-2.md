# Notes on 007 - Instrumentation Sites (Critique)

## Overall Assessment
This is a solid plan with good coverage of data movement and event gaps. It is
well-anchored in the actual call graph and points to concrete sites and event
shapes. The priority order is reasonable and aligned to observability impact.

## Strengths
- Clear pipeline map that makes attachment points obvious.
- Good emphasis on chokepoints (`ingest`, `buildRequestMessages`).
- Practical gap analysis with concrete event definitions.
- Explicit recognition of broken event pairs (ToolCallStarted without Completed).
- Prioritization is justified and matches operational value.

## Weaknesses / Risks
- High-frequency events (MessageIngest) could overwhelm sinks if not gated.
- `TurnEnvelope` is treated as a single addition, but required data is spread
  across multiple layers; might require light refactors to expose fields.
- Mixing telemetry and fatal logging can still miss events if panic/exit happens
  before telemetry flush; needs explicit flush policy.
- No explicit correlation ID is defined (turn_id, tool_call_id, request_id).
  Without it, traceability across events will remain partial.

## Ambiguities
- **Event ownership**: Which package should emit each event (application vs
  dispatcher vs tools vs mcp)? This affects API churn and test ownership.
- **Event schema stability**: Are these events public contracts or internal?
  This impacts how aggressively fields can evolve.
- **Sampling policy**: Not defined. For high-frequency events (ingest), do we
  sample, or do we require explicit debug mode? Mario's answer - take 5%? for HF spikes =
  just rate limit samples? 
- **Error taxonomy**: `telemetry.Error` is broad; no standard `Stage` or
  `Code` enum exists. That can make dashboards inconsistent.

## Gaps Not Explicitly Called Out
- **Tool output size** and **artifact references** are not in the envelope.
  This will matter once artifacts are introduced.
- **Compaction decision** is only logged post-facto; no event for "compaction
  skipped because threshold not met" (useful for tuning).
- **Retry timing**: no capture of backoff duration or retry count per turn.

## Recommendations
1. Add a `TurnID` or `RequestID` to all event payloads.
2. Define a minimal `Stage` enum for errors (history_save, mcp_start, tool_exec).
3. Add a sampling or `debug` flag for ingest-level events.
4. Consider a tiny `EventBus` (Observer) so emitters don’t hold sink details.
5. Document whether event types are stable (public contract) vs internal.

# Telemetry Scope and Coverage

This note documents where telemetry is emitted, what each signal is good for,
and which areas are not yet abstracted behind the telemetry interface.

## Emitted Sites

1. Turn lifecycle
- `turn.start` / `turn.complete`
- Emitted at the beginning and end of each assistant turn.
- Useful for:
  - Measuring per-turn latency when combined with timestamps.
  - Understanding conversation position (turn index, message count).
  - Correlating model/provider usage across turns.
 - Tree-sitter query (Go) to find call sites:
   ```scm
   (call_expression
     function: (selector_expression
       operand: (_) @recv
       field: (field_identifier) @method)
     (#eq? @method "TurnStarted"))
   ```
   ```scm
   (call_expression
     function: (selector_expression
       operand: (_) @recv
       field: (field_identifier) @method)
     (#eq? @method "TurnCompleted"))
   ```

2. Tool execution
- `tool.start` / `tool.complete`
- Emitted for function tools invoked by the assistant.
- Useful for:
  - Tool usage frequency and performance.
  - Approvals and command volume (via args size).
  - Building dashboards of tool hot paths.
 - Tree-sitter query (Go) to find call sites:
   ```scm
   (call_expression
     function: (selector_expression
       operand: (_) @recv
       field: (field_identifier) @method)
     (#eq? @method "ToolCallStarted"))
   ```
   ```scm
   (call_expression
     function: (selector_expression
       operand: (_) @recv
       field: (field_identifier) @method)
     (#eq? @method "ToolCallCompleted"))
   ```

3. Compaction
- `compaction`
- Emitted after a compaction event.
- Useful for:
  - Observing compaction triggers and thresholds.
  - Monitoring token/char/message pressure over time.
  - Validating compaction effectiveness per model.
 - Tree-sitter query (Go) to find call sites:
   ```scm
   (call_expression
     function: (selector_expression
       operand: (_) @recv
       field: (field_identifier) @method)
     (#eq? @method "Compaction"))
   ```

4. Retry
- `retry`
- Emitted on rate limit retry.
- Useful for:
  - Detecting provider instability or throttling.
  - Tuning retry/backoff strategies.
 - Tree-sitter query (Go) to find call sites:
   ```scm
   (call_expression
     function: (selector_expression
       operand: (_) @recv
       field: (field_identifier) @method)
     (#eq? @method "Retry"))
   ```

5. Errors
- `error`
- Emitted for chat completion failures, stream errors, tool/MCP errors.
- Useful for:
  - Alerting and root-cause analysis.
  - Error-rate tracking by stage.
 - Tree-sitter query (Go) to find call sites:
   ```scm
   (call_expression
     function: (selector_expression
       operand: (_) @recv
       field: (field_identifier) @method)
     (#eq? @method "Error"))
   ```

## What Data These Enable

- Turn-level traces (duration, model/provider, context size)
- Tool usage profiling (top tools, latency patterns, approval rates)
- Compaction health (token thresholds reached, impact on context)
- Provider reliability (retry/error rates)

## Not Yet Abstracted (Pre-Pipeline / Legacy)

The following sites still use ad-hoc logic or are not yet instrumented:

1. Pre-pipeline message ingestion
- Some message creation paths still bypass a centralized telemetry context.
- Example: direct `message.New(...).Build()` calls before a unified “turn envelope”.

2. System prompt resolution
- The system prompt assembly and final prompt contents are not yet emitted as
  telemetry events or trace attributes.

3. Tool gating decisions
- Gate allow/deny outcomes are not yet reported as telemetry events.

4. MCP server lifecycle
- MCP start/stop and tool discovery are not instrumented.

5. Artifact/cache operations
- Cache and history file operations remain uninstrumented.

## Tree-sitter Queries for Gaps

These queries help locate likely pre-pipeline sites that should be routed
through telemetry.

1. Direct `message.New(...).Build()` (pre-pipeline message assembly):
```scm
(call_expression
  function: (selector_expression
    operand: (call_expression
      function: (selector_expression
        operand: (identifier) @pkg
        field: (field_identifier) @ctor))
    field: (field_identifier) @build)
  (#eq? @pkg "message")
  (#eq? @ctor "New")
  (#eq? @build "Build"))
```

2. `log.Fatalf`/`log.Printf` ad-hoc logging:
```scm
(call_expression
  function: (selector_expression
    operand: (identifier) @pkg
    field: (field_identifier) @fn)
  (#eq? @pkg "log")
  (#match? @fn "Fatalf|Printf|Print"))
```

## Next Steps (Suggested)

- Add telemetry to prompt assembly, tool gating, and MCP lifecycle.
- Add a “turn envelope” that captures the effective system prompt and context
  snapshot once per turn.
- Add a trace adaptor for OpenTelemetry to correlate logs with spans.

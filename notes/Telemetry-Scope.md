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

   <!-- tree-sitter-results: begin -->
   ```go
   ./internal/telemetry/telemetry.go:66: 		**sink**.TurnStarted(ctx)
   ./internal/conversation/application.go:956: 	**app.telemetry**.TurnStarted(telemetry.TurnContext{
   ```
   <!-- tree-sitter-results: end -->




   ```scm
   (call_expression
     function: (selector_expression
       operand: (_) @recv
       field: (field_identifier) @method)
     (#eq? @method "TurnCompleted"))
   ```

   <!-- tree-sitter-results: begin -->
   ```go
   ./internal/telemetry/telemetry.go:72: 		**sink**.TurnCompleted(ctx)
   ./internal/conversation/application.go:969: 	**app.telemetry**.TurnCompleted(telemetry.TurnContext{
   ```
   <!-- tree-sitter-results: end -->





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

   <!-- tree-sitter-results: begin -->
   ```go
   ./internal/telemetry/telemetry.go:78: 		**sink**.ToolCallStarted(ctx)
   ./internal/conversation/tools/dispatcher.go:120: 			**d.deps.Telemetry**.ToolCallStarted(telemetry.ToolCallContext{
   ```
   <!-- tree-sitter-results: end -->




   ```scm
   (call_expression
     function: (selector_expression
       operand: (_) @recv
       field: (field_identifier) @method)
     (#eq? @method "ToolCallCompleted"))
   ```

   <!-- tree-sitter-results: begin -->
   ```go
   ./internal/telemetry/telemetry.go:84: 		**sink**.ToolCallCompleted(ctx)
   ./internal/conversation/tools/dispatcher.go:203: 			**d.deps.Telemetry**.ToolCallCompleted(telemetry.ToolCallContext{
   ```
   <!-- tree-sitter-results: end -->





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

   <!-- tree-sitter-results: begin -->
   ```go
   ./internal/telemetry/telemetry.go:90: 		**sink**.Compaction(ctx)
   ./internal/conversation/application_compaction.go:132: 		**app.telemetry**.Compaction(telemetry.CompactionContext{
   ```
   <!-- tree-sitter-results: end -->





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

   <!-- tree-sitter-results: begin -->
   ```go
   ./internal/telemetry/telemetry.go:96: 		**sink**.Retry(ctx)
   ./internal/conversation/application.go:407: 				**app.telemetry**.Retry(telemetry.RetryContext{
   ```
   <!-- tree-sitter-results: end -->





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

   <!-- tree-sitter-results: begin -->
   ```go
   ./cmd/docgen/main_test.go:42: 		**t**.Error("expected chroma wrapper class, not found")
   ./cmd/docgen/main_test.go:45: 		**t**.Error("expected name-variable spans for @captures, not found")
   ./cmd/docgen/main_test.go:48: 		**t**.Error("expected punctuation spans for parentheses, not found")
   ./tests/token_tracking_test.go:105: 		**t**.Error("usage key should be omitted from JSON when nil")
   ./tests/token_tracking_test.go:115: 		**t**.Error("Usage should be nil for old history files")
   ./tests/token_tracking_test.go:173: 		**t**.Error("zero-value Usage should be empty")
   ./tests/token_tracking_test.go:178: 		**t**.Error("Usage with zero tokens added should still be empty")
   ./tests/token_tracking_test.go:183: 		**t**.Error("Usage with tokens should not be empty")
   ./tests/token_tracking_test.go:197: 		**t**.Error("IncludeUsage should be true")
   ./internal/token/usage_test.go:28: 		**t**.Error("zero-value Usage should be empty")
   ./internal/token/usage_test.go:33: 		**t**.Error("Usage with tokens should not be empty")
   ./internal/token/usage_test.go:71: 		**t**.Error("Usage should be nil for old history files without token data")
   ./internal/llm/model_context_test.go:28: 		**t**.Error("expected not found for unknown model")
   ./internal/llm/model_context_test.go:40: 		**t**.Error("expected not found for unknown provider")
   ./internal/llm/model_context_test.go:48: 		**t**.Error("expected not found for empty args")
   ./internal/llm/model_context_test.go:56: 		**t**.Error("expected not found when overrides is nil")
   ./internal/config/config_test.go:50: 		**t**.Error("Expected custom provider to exist")
   ./internal/utils/utils_input.go:98: 			if **err**.Error() == "EOF" {
   ./internal/utils/utils_input.go:121: 			if **err** != nil && err.Error() == "EOF" {
   ./internal/agent/agent_test.go:120: 				if !strings.Contains(**err**.Error(), tt.errContains) {
   ./internal/cli/cli_history.go:23: 		if strings.Contains(**err**.Error(), "no history files found") || strings.Contains(err.Error(), "cache directory does not exist") {
   ./internal/cli/cli_history.go:24: 			printWarning(**err**.Error())
   ./internal/cli/cli_history.go:26: 			printError(**err**.Error())
   ./internal/cli/cli_history.go:114: 		if strings.Contains(**err**.Error(), "no history files found") || strings.Contains(err.Error(), "cache directory does not exist") {
   ./internal/cli/cli_history.go:115: 			color.Yellow(**err**.Error())
   ./internal/cli/cli_history.go:117: 			printError(**err**.Error())
   ./internal/cli/cli_history.go:159: 		if strings.Contains(**err**.Error(), "no history files found") || strings.Contains(err.Error(), "cache directory does not exist") {
   ./internal/cli/cli_history.go:160: 			color.Yellow(**err**.Error())
   ./internal/cli/cli_history.go:162: 			color.Red(**err**.Error())
   ./internal/cli/repl.go:259: 			fmt.Fprintln(os.Std**err**, theme.Error.Render("[ERROR] "+err.Error()))
   ./internal/cli/repl.go:354: 		fmt.Fprintln(os.Std**err**, theme.Error.Render("[ERROR] "+err.Error()))
   ./internal/cli/repl.go:374: 		fmt.Fprintln(os.Std**err**, theme.Error.Render("[ERROR] "+err.Error()))
   ./internal/cli/repl.go:458: 		fmt.Fprintln(os.Std**err**, theme.Error.Render("[ERROR] "+err.Error()))
   ./internal/telemetry/slog.go:87: 	**s.logger**.Error("error", "stage", ctx.Stage, "error", ctx.Error)
   ./internal/telemetry/telemetry.go:102: 		**sink**.Error(ctx)
   ./internal/conversation/application_test.go:341: 			if tt.expectError && **err** != nil && err.Error() != tt.errorDescription {
   ./internal/conversation/application_test.go:342: 				t.Errorf("Expected **err**or message %q, got %q", tt.errorDescription, err.Error())
   ./internal/conversation/application.go:336: 	**err**Str := err.Error()
   ./internal/conversation/application.go:981: 	**app.telemetry**.Error(telemetry.ErrorContext{
   ./internal/conversation/application.go:983: 		Error: **err**.Error(),
   ./internal/conversation/tools/dispatcher.go:163: 				**d.deps.Telemetry**.Error(telemetry.ErrorContext{
   ./internal/conversation/tools/dispatcher.go:165: 					Error: **err**.Error(),
   ./internal/conversation/tools/dispatcher.go:299: 			**d.deps.Telemetry**.Error(telemetry.ErrorContext{
   ./internal/conversation/tools/dispatcher.go:301: 				Error: **err**.Error(),
   ```
   <!-- tree-sitter-results: end -->





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

<!-- tree-sitter-results: begin -->
```go
./internal/conversation/application.go:220: 	app.ingest(**message**.New(role, content).Build())
./internal/conversation/application.go:764: 		app.ingest(**message**.New("user", input).Build())
./internal/conversation/application.go:768: 		app.ingest(**message**.New("user", commandStr).Build())
./internal/conversation/application.go:778: 		app.ingest(**message**.New("user", prompt).Build())
./internal/conversation/application_ingest_test.go:36: 	app.ingest(**message**.New("user", "hello").Build())
./internal/conversation/application_ingest_test.go:52: 	app.ingest(**message**.New("user", "hello").Build())
./internal/conversation/application_ingest_test.go:70: 	app.ingest(**message**.New("user", "x").Build())
./internal/conversation/application_ingest_test.go:79: 	app.ingest(**message**.New("user", "first").Build())
./internal/conversation/application_ingest_test.go:80: 	app.ingest(**message**.New("assistant", "second").Build())
./internal/conversation/application_ingest_test.go:81: 	app.ingest(**message**.New("user", "third").Build())
./internal/conversation/application_ingest_test.go:95: 	app.ingest(**message**.New("user", "what time is it?").Build())
./internal/conversation/application_ingest_test.go:104: 	app.ingest(**message**.New("assistant", "It is 3pm.").Build())
./internal/conversation/message/builder_test.go:12: 	msg := **message**.New("user", "hello world").Build()
./internal/conversation/message/builder_test.go:120: 	msg := **message**.New("assistant", "I can help with that.").Build()
```
<!-- tree-sitter-results: end -->





2. `log.Fatalf`/`log.Printf` ad-hoc logging:
```scm
(call_expression
  function: (selector_expression
    operand: (identifier) @pkg
    field: (field_identifier) @fn)
  (#eq? @pkg "log")
  (#match? @fn "Fatalf|Printf|Print"))
```

<!-- tree-sitter-results: begin -->
```go
./internal/llm/model.go:44: 		**log**.Fatalf("invalid model format %q - must be provider/model", modelStr)
./internal/logging/logging_test.go:31: 	**log**.Print("hello-log")
./internal/conversation/application.go:718: 			**log**.Fatalf("Failed to start MCP servers: %v", err)
./internal/conversation/application.go:728: 		**log**.Fatalf("Error processing system prompt: %v", err)
./internal/conversation/application.go:758: 		**log**.Fatalf("Error: %v", err)
./internal/conversation/application.go:774: 		**log**.Fatalf("Error processing initial message: %v", err)
./internal/conversation/tools/dispatcher.go:92: 			**log**.Fatalf("No matching function found for: %s", toolCall.Function.Name)
```
<!-- tree-sitter-results: end -->





## Next Steps (Suggested)

- Add telemetry to prompt assembly, tool gating, and MCP lifecycle.
- Add a “turn envelope” that captures the effective system prompt and context
  snapshot once per turn.
- Add a trace adaptor for OpenTelemetry to correlate logs with spans.

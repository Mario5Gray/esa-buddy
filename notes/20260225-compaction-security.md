# Compaction Security Plan (2026-02-25)

## Scope
- Harden prompt compaction against data leakage and summary poisoning.
- Ensure sensitive fields are not persisted or reintroduced into context.
- Keep behavior safe-by-default while allowing explicit opt-out.

## Threat Model
- Secrets in tool/function arguments leaking into summaries.
- Prompt injection inside the conversation poisoning summaries.
- Redaction failures silently passing through.
- External redaction adapters exfiltrating sensitive content.

## Current Controls (Implemented)
- Tool outputs are omitted from compaction input.
- Function/tool arguments are summarized as `[arguments omitted, N chars]`.
- Summarizer system prompt treats conversation as untrusted data and forbids secrets.
- Default redaction policy now applies when none is configured.
- Built-in secret keyword redaction masks common key/value patterns.

## Plan Items
1. **Default Redaction Policy**
   - Use `builtin/secret-keywords` when no redaction config is provided.
   - Allow explicit opt-out with `compaction_redaction.kind = "none"`.
   - Add README guidance for defaults and opt-out.

2. **Structured Secret Scrubber**
   - Redact common credential keys in JSON and `key=value` formats.
   - Keep format-preserving replacements where possible.
   - Unit tests covering JSON pairs and header-style formats.

3. **Prompt Injection Hardening**
   - Summarizer system prompt: treat content as data, ignore embedded instructions,
     and forbid secret reproduction.
   - Add regression test to ensure prompt remains hardened.

4. **Fail-Closed Redaction (Optional)**
   - Keep `fail_open = false` as default.
   - Document consequences of enabling fail-open.

5. **External Redaction Egress Warning**
   - Document that external adapters receive compaction input.
   - Recommend opt-in for production.

## Verification Checklist
- Compaction input contains no tool output payloads.
- Function/tool arguments are not copied verbatim.
- Summarizer prompt includes: untrusted content, ignore instructions, no secrets.
- Default redaction applies when config is absent.
- Secret scrubbing tests pass.

## Follow-ups
- Expand secret patterns (JWTs, AWS keys, PEM blocks).
- Add telemetry counters for redaction hits and failures.
- Add E2E tests for compaction + redaction paths.

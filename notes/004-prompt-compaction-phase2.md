# Prompt Compaction Phase 2 (Complete Scope)

## Objective
Evolve compaction from a simple summary into a full context‑management system that preserves fidelity, enables retrieval, and scales with tool outputs.

## Scope
1. **Token‑Aware Triggering**
   - Add token estimation for current context window.
   - Trigger compaction based on configurable token thresholds (e.g., 70–85% of model window).

2. **Virtualized Large Outputs**
   - Detect oversized tool outputs and store them as artifacts (files in cache).
   - Replace raw output in the context with short previews + artifact references.

3. **Artifact Registry**
   - Maintain a structured registry of artifacts (id, type, source tool, file path, size, summary).
   - Expose read/search tooling to recover artifact content on demand.

4. **Compaction Layers**
   - Maintain separate layers:
     - System prompt
     - Compact summary
     - Recent messages
     - Artifact references
   - Ensure each layer can be regenerated without loss.

5. **Pre‑Compaction Archive**
   - Persist the pre‑compaction message slice as an artifact.
   - Allow recovery or re‑summary if needed.

6. **Summary Quality Controls**
   - Summarize with a dedicated model and settings.
   - Enforce structure in summaries (decisions, open tasks, files, commands, constraints).

7. **Redaction & Privacy**
   - Add a redaction policy for summaries (PII, secrets, tokens).
   - Allow opt‑out or strict redaction modes.

8. **Metrics & Debugging**
   - Record compaction events (reason, token counts, size saved).
   - Surface in debug output and history metadata.

## Non‑Goals
- Automatic LLM‑driven planning or retrieval policies.
- UI/visualization (can be added later as a separate phase).

## Deliverables
- `internal/conversation/compaction/` (or similar) with token estimation, artifact management, and summary execution.
- Artifact registry persisted in history files.
- CLI/config flags to tune thresholds and models.
- Tests for triggering, artifact storage, and recovery.

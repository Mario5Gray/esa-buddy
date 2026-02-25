# Vector Database Integration Plan (2026-02-25)

## Goal
Add a semantic memory layer so ESA can retrieve relevant prior context without
stuffing full history into the prompt. This complements compaction, artifacts,
and redaction policies.

## Fit With Roadmap
- Phase 1: Prompt compaction + redaction → summarize and sanitize.
- Phase 1.5: External redaction analyzer → optional pre-index scrub.
- Phase 5: Tool graph + artifacts → store large outputs and index summaries.

## Architecture (Patterns)
- **Repository (PoEAA)**: `MemoryStore` interface abstracts vector DB operations.
- **Strategy (GoF)**: `RetrievalPolicy` selects top-K, filters, and weights sources.
- **Pipeline (EIP)**: `ingest → redact → summarize → embed → store`.
- **Observer (GoF)**: Emit telemetry for retrieval hits/misses and latency.

## Data Model (High Level)
- `MemoryItem`
  - `id`
  - `type` (summary | tool_output | decision | file_snapshot | artifact_ref)
  - `content` (redacted text)
  - `embedding` (vector)
  - `metadata` (repo, branch, commit, file, timestamp, tags)

## Ingestion Flow
1. Produce compaction summary (already exists).
2. Redact summary with configured policy (already exists).
3. Embed summary text.
4. Store in vector DB with metadata.

## Retrieval Flow
1. On new prompt, embed query (or use draft prompt).
2. Retrieve top-K items by similarity.
3. Apply filter policy (repo, commit, recentness).
4. Inject retrieved items into context as a separate “memory” block.

## Security & Privacy
- Only store redacted text.
- Allow opt-out per repo (`memory_enabled = false`).
- Provide retention policy (TTL or size cap).
- Add audit log for retrieval and ingestion events.

## Config Surface (Draft)
```toml
[memory]
enabled = true
provider = "qdrant"            # or "pgvector", "chroma", "weaviate"
top_k = 6
min_score = 0.25
ttl_days = 90

[memory.provider.qdrant]
url = "http://localhost:6333"
collection = "esa"
```

## Tests
- Unit: MemoryStore contract, embedding stub, retrieval policy.
- Integration: write → query → retrieve top-K.
- E2E: compaction → redaction → embed → store → retrieve.

## Milestones
1. Define `MemoryStore` interface and no-op impl.
2. Add in-memory test store.
3. Wire ingestion from compaction summary.
4. Wire retrieval into message build pipeline.
5. Add provider adapter (start with Qdrant or pgvector).

## Risks
- Prompt injection via retrieved memory.
- Stale or conflicting memories.
- Cost/latency of embedding calls.

## Mitigations
- Treat memory as untrusted; label and separate in prompt.
- Add recency and source filters.
- Cache embeddings and batch ingest when possible.

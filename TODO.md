# ESA Roadmap

## Phase 1: Foundation

### 1. Token Tracking
- [x] Track input/output tokens per request
- [x] Display token usage in `--show-stats`
- [x] Optional: estimate costs per provider/model
- [x] TokenCounter interface with tiktoken-backed estimates (fallback if unavailable)

### 2. Agent Inheritance
- [x] Define `extends` field in agent TOML
- [x] Implement function merging (child overrides parent)
- [x] Implement system prompt composition
- [x] flatten at load time

### 3. Agent Versioning
- [x] Define version field in agent TOML
- [x] Version format: semver (`1.0.0`)
- [x] Support version pinning syntax (`agent@v1.2`)

### 4. Prompt Compaction
- [x] Auto-summarize older messages into a system summary
- [x] Add CLI flags to enable/disable compaction
- [x] Track compaction thresholds per conversation (saved in history)
- [x] Avoid summarizing tool outputs verbatim; keep key results only
- [x] Summarize with a dedicated model (configurable)
- [x] Persist compaction metadata (message/char/token estimates) in history
- [x] Redaction policy for summaries (pluggable policy + external analyzer adapter)
- [x] Store summary in history metadata instead of system message

### 5. Prompt Compaction Phase 2 (Project Overview)
- [ ] Token-aware trigger based on model context window
- [ ] Virtualize large tool outputs into artifacts (cache files)
- [ ] Artifact registry with read/search retrieval tools
- [ ] Multi-layer compaction: summary + recent + artifact refs
- [ ] Persist pre-compaction archive for recovery
- [ ] Dedicated summary model + structured summary schema
- [ ] Redaction/PII policy and opt-out modes
- [ ] Compaction metrics and history metadata

## Phase 1.5: Redaction & Analysis

### 1. External Redaction Analyzer
- [ ] Define analyzer interface and lifecycle (sync/async)
- [ ] Wire analyzer outputs into compaction redaction policy
- [ ] Configuration for analyzer endpoint/adapter

## Phase 2: Hub Architecture

### 4. Client Interface Design
- [ ] Define hub client interface (Go interface)
- [ ] Operations: search, list, get, publish, versions
- [ ] Agent metadata schema (author, tags, dependencies, checksum)

### 5. Hub Abstractions
- [ ] Abstract storage backend interface
- [ ] Abstract registry/index interface
- [ ] Define agent manifest format

### 6. Test Hub Implementation
- [ ] In-memory or filesystem-based test hub
- [ ] Implement all client interface operations
- [ ] Seed with sample agents

## Phase 3: Client & Testing

### 7. Hub Client Implementation
- [ ] Implement client against hub interface
- [ ] Agent download and installation to `~/.config/esa/agents/`
- [ ] Dependency resolution for inherited agents
- [ ] Cache management

### 8. Client Testing
- [ ] Unit tests against test hub
- [ ] Mock various failure scenarios
- [ ] Version resolution tests

### 9. E2E Testing
- [ ] Deploy test hub
- [ ] Test agent discovery and installation
- [ ] Test agent updates and version changes
- [ ] Test client reactions to hub mutations

## Phase 4: Production

### 10. CLI Interfaces
- [ ] `esa +hub search <query>`
- [ ] `esa +hub install <agent>[@version]`
- [ ] `esa +hub publish <path>`
- [ ] `esa +hub list` (installed from hub)
- [ ] `esa +hub update [agent]`

### 11. Production Hub Selection
- [ ] Evaluate options: GitHub, S3, custom server, BYO
- [ ] Consider hybrid: GitHub for discovery, object store for files
- [ ] Implement chosen backend(s)

## Considerations

### Security
- [x] Tool execution policy gate (ordered, default deny; human approval supported)
- [ ] Agent signing / verification
- [ ] Trusted publisher registry
- [ ] Checksum verification on install

### Future
- [ ] Agent ratings / popularity metrics
- [ ] Private hubs for teams/orgs
- [ ] Agent dependency graph visualization

## Phase 5: Tool Graph / Lingo Layer

### 12. Tool Signatures
- [ ] Define typed inputs/outputs for tools
- [ ] Map existing function parameter types into signature schema

### 13. Tool Graph Model
- [ ] Graph nodes = tools, edges = dataflow
- [ ] Support fan-out and fan-in compositions

### 14. Validation & Execution
- [ ] Validate input/output compatibility
- [ ] Execute graph in topological order
- [ ] Persist outputs as named facts for downstream steps

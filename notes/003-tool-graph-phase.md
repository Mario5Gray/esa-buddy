# Phase: Tool Graph / Lingo Layer (Draft)

## Objective
Introduce a structured “lingo” for tool usage where tool outputs can be routed as inputs to other tools, enabling explicit workflows, validation, and pattern discovery.

## Scope (Narrow)
1. **Tool Signatures**
   - Define typed inputs/outputs for tools (name, type, optional description).
   - Reuse existing tool parameter types as a base.

2. **Graph Model**
   - Nodes = tools, edges = dataflow.
   - Support fan-out (A -> B, C) and fan-in (B + C -> D).

3. **Validation**
   - Check that outputs satisfy required inputs.
   - Fail fast on missing or incompatible mappings.

4. **Execution**
   - Simple topological execution of the graph.
   - Record outputs as named facts for downstream nodes.

## Non-goals (for this phase)
- No automatic planner or LLM-driven graph synthesis.
- No advanced type system (just basic types + optional labels).
- No UI/visualization layer yet.

## Deliverables
- `internal/toolgraph` package with types + validation + executor.
- Minimal tests for validation and execution ordering.

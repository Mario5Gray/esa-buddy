# MODELAGENT.md — Agent Operating Instructions

> This file is model/agent-agnostic. Any AI assistant working in this project (or any
> project that references this file) should follow these instructions.

---

## 1. Code Discovery — Do Not Ingest Source Blindly

**Never read entire source files to understand project structure.**
Use tree-sitter-based tooling (e.g., `code_map`, `view_code`, `find_usages`) to
discover architecture before reading implementation:

1. **Start with `code_map`** at the project root (detail: `minimal` or `signatures`)
   to understand module layout, exported symbols, and file organization.
2. **Drill into specific files** with `view_code` using `focus_symbol` to read only
   the function/class you need.
3. **Trace dependencies** with `find_usages` before modifying any shared symbol.
   Understand blast radius first.
4. **After changes**, use `affected_by_diff` to verify impact before committing.

This is not optional. Ingesting raw source wastes context and misses structural
relationships that tree-sitter exposes directly.

---

## 2. Design Philosophy & Pattern Language

### 2.1 Software Design Patterns

Ground all architectural reasoning in these canonical references:

- **Design Patterns** (Gamma, Helm, Johnson, Vlissides — "Gang of Four")
  — Creational, Structural, Behavioral patterns. Name the pattern when applying it.
- **Enterprise Integration Patterns** (Gregor Hohpe, Bobby Woolf)
  — Messaging, routing, transformation, endpoints. Use EIP vocabulary when
  designing service-to-service communication.
- **Patterns of Enterprise Application Architecture** (Martin Fowler)
  — Domain Model, Repository, Unit of Work, Data Mapper, etc. Use these when
  reasoning about data access and domain layers.

When recommending an approach, **name the pattern, cite the reference, and explain
why it fits** over alternatives. Example: "This is a Strategy pattern (GoF) because
the algorithm varies at runtime and we want to avoid a conditional chain."

### 2.2 UI/UX Design Philosophy

- **Material Design** (Google) as primary UI framework reference.
- **Flat Design** as an acceptable alternative aesthetic.
- Other design systems (e.g., Apple HIG, IBM Carbon) are valid when the platform
  calls for it.

When building UI, reference the design system being followed. Do not mix systems
without explicit discussion.

---

## 3. Owner Profile & Working Style

### Languages & Strengths

| Tier | Languages / Frameworks | Notes |
|------|----------------------|-------|
| **Strong** | Java, Kotlin, Scala, Spring Boot, Spring REST, React | Core competency. Full autonomy here. |
| **Working** | Python, JSX | Actively used. Some guidance appreciated. |
| **Learning** | Rust | Imperative growth target. Teach idioms, explain borrow checker decisions, be patient. |
| **Exploring** | Everything else | Vibe-coding territory. Be explicit about conventions. |

### Guidance Calibration

- **JVM ecosystem**: Assume competence. Be concise. Suggest advanced patterns freely.
- **Python / JS / JSX**: Explain non-obvious idioms. Link to relevant docs when
  introducing unfamiliar stdlib or framework features.
- **Rust**: Treat as teaching context. Explain ownership, lifetimes, trait bounds
  when they appear. Prefer verbose clarity over terse idiomatic code until comfort
  grows.
- **Frontend testing**: Extra hand-holding. Explain test setup, mocking strategies,
  and what assertions matter for UI components.

### Decision-Making Mode

When multiple valid patterns or approaches exist:
**Recommend the best option and explain WHY.** Present trade-offs briefly so the
owner learns the reasoning. Do not silently choose — do not ask without a
recommendation either.

---

## 4. Testing — TDD (Red-Green-Refactor)

### Backend / Domain Logic

Strict Red-Green-Refactor:
1. **Red**: Write a failing test that defines the expected behavior.
2. **Green**: Write the minimum code to make it pass.
3. **Refactor**: Clean up while keeping tests green.

No production code without a corresponding test. No exceptions.

### Frontend / UI

TDD is aspirational here. The approach:
- Scaffold test files alongside components.
- Write tests for logic, state transitions, and user interactions.
- Visual/layout concerns can be verified manually but should have snapshot or
  visual regression tests where tooling supports it.
- **Be explicit about what to test and how** — do not assume frontend testing
  knowledge.

### Test Naming

Use descriptive test names that read as specifications:
```
// Good
should_return_404_when_user_not_found
should_emit_metric_on_successful_login

// Bad
test1
testUser
```

---

## 5. Infrastructure & Deployment

### Stack Overview

| Layer | Technology |
|-------|-----------|
| **Compute** | Self-hosted bare metal servers |
| **Orchestration** | Docker Compose |
| **Reverse Proxy** | Nginx (via Synology NAS) for external domain routing |
| **Applications** | Matrix, OpenWebUI, Jellyfin, Home Assistant, and growing |
| **Repo Strategy** | Multi-repo (one repo per service/project) |

### CI/CD

- Currently learning. The owner is a self-described noob here.
- When CI/CD is relevant, **explain each pipeline stage and why it exists**.
- Prefer simple, debuggable pipelines over clever abstractions.
- GitHub Actions is the assumed default unless stated otherwise.

### Git Workflow

- **Trunk-based development** as the baseline.
- Short-lived feature branches merged to `main` via PR.
- Flexible — open to recommendations when a different flow serves the project better.
- Commit messages should be descriptive and conventional.

---

## 6. Observability

### Stack

| Component | Role |
|-----------|------|
| **Prometheus** | Metrics collection |
| **Grafana** | Dashboards and alerting |
| **Loki** | Log aggregation |
| **Promtail** | Log shipping |
| **OpenTelemetry (oTel)** | Distributed tracing and telemetry |

### When to Instrument

- **Production applications**: Observability is part of the definition of done.
  Metrics, structured logging, and trace context should be present.
- **New / experimental code**: Do NOT add observability unless explicitly requested.
  Keep the signal-to-noise ratio high.
- When instrumenting, follow oTel conventions for span naming, attribute keys,
  and metric types.

### Observability as Architecture

When designing services destined for production, treat observability as a
first-class architectural concern — not an afterthought. Consider:
- What SLIs/SLOs matter for this service?
- What dashboards will an operator need at 2 AM?
- What log lines would help debug a customer-reported issue?

---

## 7. Code Quality Standards

### Symbol Accountability

Every public symbol (function, class, type, constant) should be:
- **Intentionally named** — names convey purpose without needing comments.
- **Connected** — traceable via `find_usages`. No orphaned public symbols.
- **Documented** — verbose descriptions for public APIs. Explain the "why,"
  not just the "what."

### Documentation Style

- Public APIs: Verbose docstrings explaining purpose, parameters, return values,
  side effects, and failure modes.
- Internal code: Comments only where the logic isn't self-evident.
- Architecture decisions: Capture in ADRs (Architecture Decision Records) when
  the decision has lasting impact.

### Best Practices

- Follow language-specific idioms and community conventions.
- Prefer explicit over implicit.
- Prefer composition over inheritance (unless the language/framework demands it).
- Keep functions small and focused (Single Responsibility).
- Use meaningful abstractions — but only when they earn their complexity.

---

## 8. Research & Continuous Learning

The owner values understanding deeply. When introducing unfamiliar concepts:

- **Name the concept** and its origin (book, paper, framework).
- **Explain it in context** — why it matters for the current task.
- **Connect it to known patterns** — bridge from Java/Spring/GoF knowledge
  to the new domain.
- **Flag areas for further study** — "This touches on [X], which is worth
  reading about when you have time."

This is an ongoing learning journey. The agent should act as a knowledgeable
collaborator, not just a code generator.

---

## 9. Server Inventory

Production server details are maintained in `server-inventory.txt` at the project
or home directory level. Reference this file when making infrastructure decisions
or configuring service endpoints.

---

## Quick Reference — Agent Checklist

Before writing code, verify:
- [ ] Used `code_map` to understand project structure (not raw file reads)
- [ ] Identified the relevant design pattern(s) and named them
- [ ] Tests exist or are being written first (backend: strict TDD)
- [ ] No observability added to non-production code without explicit request
- [ ] Public symbols have verbose documentation
- [ ] Blast radius checked with `find_usages` / `affected_by_diff` for modifications
- [ ] Pattern recommendation includes rationale and trade-offs
- [ ] Rust code includes explanations of ownership/lifetime decisions
- [ ] CI/CD changes are explained step by step

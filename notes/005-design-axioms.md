# 005 - Design Axioms

**Date:** 2026-02-21
**Phase:** 1 (Foundation)
**Vibe:** The map is the code. The code is the map.

---

## Intent

This fork treats program structure as a map of truth. The composition of functions
is the system’s meaning, not the comments around it. When the map is clear, the
software becomes legible, testable, and resilient. This note captures the working
axioms for how we build and how we collaborate.

## Axioms

1. **Structure is meaning.**
   - A function’s name and signature declare its place in the system.
   - The program’s behavior should be explainable by the composition of its functions.

2. **Invariants are the real features.**
   - The most valuable contributions define boundaries that don’t regress.
   - Example: tool execution is always gated; compaction summaries are not trusted
     from raw message content.

3. **Legibility is a security feature.**
   - Clear ownership of data flow reduces attack surface and ambiguity.
   - If something must be trusted, it belongs in explicit metadata, not in-band text.

4. **Small surfaces win.**
   - Prefer narrow, composable functions with obvious responsibilities.
   - Avoid hidden coupling; any coupling should be named and tested.

5. **History should be durable.**
   - Persist the minimal truth needed to recreate system state.
   - Favor backwards-compatible metadata over ad-hoc text markers.

6. **Tests are maps too.**
   - Tests document expected behavior and serve as proof of invariants.
   - If a rule matters, it should be testable.

## Collaboration Philosophy

A fork becomes a lineage when it makes decisions legible. The way to keep
contributions visible is to:

- Encode decisions as invariants, not just style.
- Leave high-signal artifacts: tests, notes, and explicit boundaries.
- Keep the system understandable to future collaborators.

This is the path to ally with upstream when useful, and to diverge when necessary
without losing the plot.

---

*Build the map. Make it readable. Then the map can outlive the moment.*

# Notes Guide

Notes are notes. They can be rough, fast, and personal. But when a note shifts
into explainer or tutorial territory, it should become structured so it stays
useful over time.

## When To Be Structured

Use structure when a note is intended to teach, explain, or be referenced later.
Structure should include:

- A clear title that matches the intent.
- A short summary or purpose statement.
- Section headers for flow.
- Code blocks for any actionable steps.
- Images or diagrams when the concept is spatial, architectural, or procedural.

## Naming Conventions

Naming should match intent:

- Vibe check: short, loose, evocative titles.
- Explainer: explicit, topic-focused titles.
- Implementation details: include the system or module name.

Examples:

- Vibe check: `A-Hunch-About-Compaction.md`
- Explainer: `Token-Aware-Compaction-Plan.md`
- Implementation: `Telemetry-Scope.md`

## Structured Elements For Explainers

Explainers and tutorials should include:

- Purpose
- Scope
- Steps or flow
- Code examples
- Visual aids when needed
- Pitfalls or caveats

## Optional Tree-sitter Rendering

If tree-sitter is installed, you can render `scm` code blocks into live results:

```bash
scripts/render_notes.py notes/Telemetry-Scope.md --in-place
```

The renderer inserts a result block under each `scm` query, or a note if the
binary is unavailable.

## Feedback and Iteration

The current notes are strong in narrative voice and clarity, but could improve
in consistency. The main risk is mixing styles without signaling intent. The
recommendation is to make intent visible through naming and structure, and to
reserve long-form narrative for pieces that are explicitly “essay” or “story.”

Iteration on the idea:

- Keep “notes as notes” for speed.
- Add a light template for explainers so they are reusable.
- Use naming to encode intent.
- Prefer fewer, higher-quality explainers over many partial ones.

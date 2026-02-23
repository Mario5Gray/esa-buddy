What you described (dynamic discovery)

Query model servers (TabbyAPI/OpenAI) to list available models.
Determine tokenizer type per model.
Scan the “classpath” for tokenizer implementations and activate them.
Report which tokenizers were found.
Potentially download tokenizers.
This is viable, but it’s a bigger feature with more moving parts (network calls, plugin system, compatibility checks, security).

Recommended staged approach

Now (simple, robust):
Keep hard‑coded registration (like line ~489 in application.go).
Fallback counter if no matching provider.
Next step (config-driven):
Add a config mapping provider -> tokenizer (tiktoken/hf/fallback).
Still no dynamic discovery.
Later (dynamic discovery):
Optional --discover-tokenizers startup step:
Query models.
Match tokenizer implementations.
Emit report.
Downloading tokenizers (optional):
If model is local, require tokenizer files locally.
If model is remote, only fetch if explicitly allowed.
Treat this as a separate security‑sensitive feature.
Answer to your last question
Yes, it can download tokenizer files, but I’d only do it explicitly (flag or config) and into a cache directory with checksums. Otherwise it’s a security and correctness footgun.

If you want, I can add a TODO/notes entry for the dynamic discovery phase so we keep it in scope but not block progress.

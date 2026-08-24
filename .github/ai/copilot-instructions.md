# Copilot Instructions For Akoflow

Read `AGENTS.md` first.

Use `AI_ARCHITECTURE.md` and `AI_RUNTIME_EXECUTION_FLOW.md` as the canonical repository guidance for architecture and runtime flow.

## Copilot Focus

- Keep responses aligned with the layered architecture.
- When debugging runtime behavior, follow the wrapper -> service -> connector -> repository chain.
- When adding code, preserve the existing boundaries between handlers, services and runtimes.

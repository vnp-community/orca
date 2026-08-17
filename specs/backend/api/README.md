# Backend API & Architecture

- [`backend-hld-c4.md`](./backend-hld-c4.md) — High Level Design for
  `backend/` in C4 format (Context, Container, Component), scoped to the
  post-monorepo-split `backend/` package only. Built from the other docs in
  this directory plus `docs/hld/backend-server-architecture.md`; supersedes
  `docs/hld/v1/` for backend-specific detail.
- [`business-capabilities.md`](./business-capabilities.md) — every business
  capability `backend/` currently provides, organized by domain: what it
  does, how the frontend reaches it, where the work actually happens today
  (backend-local / Postgres / relayed to the Dev Server Agent), which
  Postgres table(s) back it.
- [`backend-agent-target-architecture.md`](./backend-agent-target-architecture.md) —
  a proposal for closing the gap to a clean split: backend =
  coordination/permissions/connections-to-external-parties (always
  PostgreSQL), agent = all detailed execution (source code, filesystem,
  terminal, git, AI agents). Names 7 concrete, already-diagnosed gaps
  between today's code and that model, with a recommended fix and
  sequencing for each — not a rewrite, most of the codebase already matches
  the target.
- [`desktop-only-rpc-parity-gaps.md`](./desktop-only-rpc-parity-gaps.md) —
  147 RPC methods desktop mode has that backend/server mode doesn't
  (2026-08-16 audit), categorized: which are genuinely desktop-only (don't
  port), which are low-risk/real-value (worth porting), and which need a
  product decision first (`orcaProfiles.*`, `claudeAccounts.*`/`codexAccounts.*`
  overlapping with `aiProvider.*`, etc.). Also documents what got ported the
  same day (`rateLimits`/`onboarding`/`crashReports`/`telemetry`/
  `claudeUsage`/`codexUsage`/`openCodeUsage`) and their known limitations.

## Related

- [`specs/frontend/api/`](../../frontend/api/) — the frontend↔backend RPC
  contract (method catalog, gaps, IPC surface, the dispatch-boundary doc
  this directory's capability catalog is built on).
- [`specs/agent/api/`](../../agent/api/) — the backend↔agent wire-level RPC
  contract (connection modes, agent-side method catalog, cross-cutting
  gaps/bugs found auditing that specific boundary).
- `docs/hld/dev-server-architecture.md`,
  `docs/hld/backend-server-architecture.md` — the current-state HLD docs
  this directory's target-architecture proposal extends.

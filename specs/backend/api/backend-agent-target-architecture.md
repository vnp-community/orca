# Target Architecture: Backend = Coordination, Agent = Execution

A proposal for closing the gap between what the codebase does today (cataloged
in [`business-capabilities.md`](./business-capabilities.md) and
[`../../frontend/api/backend-agent-execution-boundary.md`](../../frontend/api/backend-agent-execution-boundary.md))
and a clean split:

- **Backend** — coordination, permission/rights management, and connections
  to external parties (GitHub, GitLab, Jira, Linear, …). **Always
  PostgreSQL-backed.** Never touches a specific dev server's filesystem,
  git state, or terminal directly.
- **Agent** — every detailed task involving source code, filesystem,
  terminal, git, and AI agent execution. Runs *on* the dev server the work
  actually happens on.

This is not a from-scratch design — it's mostly already the codebase's own
intent (`docs/hld/dev-server-architecture.md`'s Control-Plane/Data-Plane
split, ADR-013/018) and, for the largest domains, already the *actual
implementation*. The job here is naming which parts already match, which
don't, and what closing each gap concretely requires.

## The model already exists — `git.*`/`files.*`/`worktree.*` is the reference implementation

Don't design a new pattern. This one, already running in production, **is**
the target architecture for anything touching a specific dev server:

```
Backend RPC method (coordination: who's asking, is it allowed, which target)
  → resolves the worktree's owning host (connectionId or not)
  → not connected to a specific host → executes read-only/generic parts itself
  → connected (SSH target or Dev Server Agent) → relays via a typed provider
      interface (IGitProvider / IFilesystemProvider / IPtyProvider)
      → agent-side handler does the actual work on that host
```

Every new domain that needs to touch a dev server should be built this way:
a typed provider interface, a `connectionId`-keyed registry
(`ssh-git-dispatch.ts` is the template), and backend's RPC handler doing
*only* target resolution + access control before delegating. **Do not**
invent a fourth relay mechanism — three already coexist (Dev Server Agent
WS-RPC, SSH multiplexer, raw SFTP) and that inconsistency is itself a
maintenance cost (see Gap 5).

## Gap 1 — GitHub/GitLab: wrong mechanism, not just wrong location

**Current**: backend shells out to `gh`/`glab` CLI in-process for 64 methods,
using a shared OS keychain with no per-user isolation
(`business-capabilities.md`'s GitHub/GitLab integration section).

**This is not simply "should relay to the agent instead."** Re-read the
target split: GitHub/GitLab issue/PR/comment/label/review/rate-limit
operations are pure HTTP API calls — the exact same shape as Jira/Linear,
which **already do this correctly** (direct REST/GraphQL calls from
backend, per-user credentials, zero CLI, zero agent involvement). The
`gh`/`glab` CLI dependency is the actual mistake, not the process it runs
in. Recommended fix:

1. Replace `child_process.execFile('gh'/'glab', ...)` call sites with direct
   calls to GitHub's REST/GraphQL API and GitLab's REST API, using per-user
   OAuth tokens stored the same way Jira/Linear tokens already are
   (`WebCredentialStore`-pattern encrypted files, keyed per user — not a
   shared keychain). This closes the multi-tenancy gap *and* keeps the
   "connections to other parties" work correctly on backend, no agent
   relay needed, for the ~55 of 64 methods that are pure API interaction
   (list/create/update issues, PR/MR comments, labels, reviewers, checks,
   project-board views, rate limits).
2. **Auth is the one piece that genuinely needs a process to run an
   interactive login** — today that's `gh auth login`/`glab auth login`
   over a PTY relayed to the Dev Server Agent (`github.startAuthLogin`,
   already correctly per-user-isolated, confirmed in
   `specs/agent/api/gaps-and-findings.md` finding #5). Keep that path, or
   replace it with a proper OAuth web-flow (`/auth/github/callback`-style,
   same shape as the existing `/auth/sso/:provider` stub) that never needs
   a CLI or PTY at all — worth a product decision on which is less
   friction for users, but either way it stays out of the "self-executed
   CLI with shared credentials" trap.
3. **Known partial implementation to reuse or retire**:
   `agent/src/relay/external-api-connector.ts` already has a correct,
   per-user-isolated (`buildGhEnv(userId, ...)`/`buildGlabEnv(userId, ...)`)
   CLI-based implementation of ~10 of these methods (PR create/merge, issue
   list/create, auth status, MR create/list, pipeline status). If the CLI
   approach is kept for any subset instead of migrating to HTTP APIs, this
   file — not a new one — is where those methods belong, and it needs
   registering into the actual RPC dispatch (it currently has handler
   functions with no caller wiring them into `agent-rpc-dispatch.ts`'s
   switch). If the HTTP-API approach above is taken instead, this file
   becomes dead weight and should be deleted rather than left as an
   attractive-nuisance third implementation.
4. **`hosted-review.*`'s GitHub/GitLab branches inherit this same fix** —
   they currently delegate to the same self-executed CLI path.

## Gap 2 — Credential handling doesn't fully satisfy "backend never sees plaintext"

**Current**: `aiProvider.writeCredential`/`rotateKey`/`testConnection`
correctly relay ciphertext-only to the Dev Server Agent (ADR-008's core
promise holds for these). But the **use path** — spawning an AI CLI with
that credential — requires backend to forward a plaintext `resolvedApiKey`
to the agent when one is available, because the agent cannot decrypt the
browser's Layer-1-encrypted blob itself
(`specs/agent/api/compliance-audit-2026-08-15.md` §3.3).

**Fix**: the agent should hold and use its own decryption capability for
credentials it already stores at rest (`~/.orca/credentials/<accountId>.enc`)
— spawn-time credential resolution should happen **entirely agent-side**
(agent decrypts its own local file, injects into the spawned process's
env, backend never sees the value in any form). This closes the gap without
backend needing to broker plaintext at all. If there's a reason backend
currently needs to resolve the key itself (e.g. a credential not yet synced
to the target dev server), that's a sync-timing problem to solve
explicitly (proactively push ciphertext on account creation/rotation, not
lazily resolve plaintext at spawn time) rather than accepting the plaintext
hop as permanent.

## Gap 3 — `automation.runNow` has no execution path; don't build a new one, reuse workflow's

**Current**: automation *scheduling* (definitions, triggers, run
bookkeeping) is correctly backend+Postgres (`PgAutomationStore`). *Execution*
is unwired — the dispatcher is intentionally left `undefined` server-side,
so every triggered run resolves `skipped_unavailable`.

**Fix**: automations and workflows already share the same step-type
vocabulary in spirit (agent/shell/notification-style actions). `workflow.*`
already has the correct target-architecture implementation for this exact
problem — `StepExecutors.ts` relays `agent`/`shell`/`notification` step
types to the Dev Server Agent and keeps `webhook`/`condition` backend-local.
Wire `automation.runNow` through the **same** executor rather than building
a second, parallel step-execution engine — an automation run is
structurally a one-step (or few-step) workflow execution, not a different
kind of thing. This also means automations automatically inherit any future
fix to `StepExecutors.ts` (see Gap 4) instead of drifting into its own copy
of the same bugs.

## Gap 4 — `agent.exec`'s param shape is a live P0 blocking the core "agent executes tasks" capability

**Current**: `StepExecutors.executeAgent()` (workflow) sends
`{stepId, prompt, worktreePath, trustPreset, traceId, accountId?, model?}` to
`agent.exec`. The agent's real handler contract (confirmed correct, matches
every other real caller) expects `{binary, args, cwd, env, timeoutMs}`.
**Every `agent`-type workflow step fails, on every connection mode, right
now** (`specs/agent/api/compliance-audit-2026-08-15.md` §2, finding #1) —
this is exactly the capability this whole redesign is centered on
("agent thực thi toàn bộ tác vụ... với AI agents"), currently broken.

**Fix**: this is purely a `backend/` change — `StepExecutors.ts` needs to
build the params `ProfileAwareAgentSpawner.ts:130` already sends correctly
(same call, already fixed once there per its own `FIX TASK-TG-001` comment
— `StepExecutors.ts` was simply never updated to match). Small, contained,
should be the first fix out of this whole proposal given its severity and
size.

## Gap 5 — Two independently-maintained relay implementations undermine "agent executes everything" as a premise

**Current**: `agent/src/relay/` (the "Part A" WS-based dev-server-agent
code) and `desktop/src/relay/` (the actual shipped `relay-ssh` binary,
`relay.ts`/`dispatcher.ts`/`git-handler.ts`/`pty-handler.ts`/`fs-handler.ts`)
are two independently-maintained near-copies of the same logic, synced
**manually, with no sync tooling** — confirmed drifted (multiple fix
sessions landed in `agent/`'s copy alone, leaving the actually-deployed
`desktop/`-built binary with the original security bug still live —
`specs/agent/api/compliance-audit-2026-08-15.md` §1).

**Why this matters for the redesign, not just as a bug**: if "the agent
executes all detailed tasks," there needs to be exactly one place that
logic lives and gets tested, or every future fix has to be applied twice
(and, per the evidence above, often isn't). Recommended fix, in order of
preference:

1. **Merge `desktop/src/relay/` into `agent/`**, making `agent/` the single
   source of truth, with `desktop/`'s build pipeline consuming `agent/`'s
   code (rather than maintaining its own copy) for whatever
   `relay-ssh`-specific packaging it needs (the SCP-deployed standalone
   binary). This directly matches "agent thực thi toàn bộ tác vụ" — one
   execution engine, not two.
2. If a full merge isn't feasible short-term, at minimum add an automated
   diff/lint check (CI-enforced) between the two trees so drift is caught
   at PR time instead of discovered by audit months later.
3. Immediate, small-scope unblock regardless of the above: port the
   already-fixed `agent/`-side security/correctness fixes (the `git.clone`
   handler-override bug in particular — still a **live, unpatched RCE-class
   bug in the real deployed relay-ssh binary** today) to `desktop/`'s copy.
   This should happen before anything else in this proposal, independent of
   the larger merge decision.

## Gap 6 — Browser/computer/emulator automation don't fit either side of the target model

**Current**: `browser.*`/`computer.*`/`emulator.*` automate the **backend
process's own host machine** (Electron `webContents`, local macOS
accessibility APIs, local ADB/`simctl`) — not a dev server, not "connecting
to another party." They're neither coordination nor dev-server execution.

**This needs a product decision, not a mechanical fix.** Three honest
options:
1. **Desktop-only, out of the multi-user server's scope entirely** — these
   features make sense when "backend" is the user's own machine (Electron
   desktop mode) but not when backend is a shared, multi-tenant Postgres
   coordinator with no relationship to any individual user's local
   hardware. Gate them out of the server deployment explicitly rather than
   leaving them silently reachable-but-meaningless there.
2. **Genuinely move to the dev server** — if browser automation is meant
   to test code running *on* a dev server, it belongs behind the same
   provider-relay pattern as everything else (a `BrowserAutomationProvider`
   the agent implements) — a real feature-scoped project, not a quick move.
3. **Leave as backend-local, explicitly documented as an exception** to the
   coordination/execution split, since it's neither "external party" nor
   "dev-server work" — acceptable if scoped narrowly and called out, not
   acceptable if it quietly grows more capability over time under the
   assumption it's "basically the same as computer.*".

Recommendation: (1) for the multi-user Postgres deployment this whole
proposal targets, revisit (2) only if there's a concrete product need for
remote browser automation specifically.

## Gap 7 — `workspacePorts.*` silently drops the case it should relay

**Current**: port scanning for a worktree explicitly filters out any repo
with a `connectionId` (returns empty), rather than relaying the scan to the
owning dev server.

**Fix**: this is the one gap that's a straightforward application of the
existing reference pattern — add a `PortScanProvider` (or extend
`IFilesystemProvider`/reuse an existing agent capability if port-scanning
already has a home there) and relay when `connectionId` is set, instead of
filtering the worktree out. Small, contained, no design decision needed.

## What does *not* need to change

Called out explicitly so this proposal doesn't read as "everything is
broken": `profile.*`, `project.*` (except `agentSpawn`, already correct),
`team.*`, most of `task.*`/`workflow.*` (except the two gaps above),
`annotation.*`, `credentials.*` (external-integration tokens), Jira/Linear
integration, and the entire `git.*`/`files.*`/`worktree.*`/`terminal.*`
dynamic-dispatch core **already match the target architecture** as
implemented today. This proposal is about closing ~7 specific,
already-diagnosed gaps, not a rewrite.

## Suggested sequencing

1. **Gap 4** (`agent.exec` param shape) — smallest, highest-severity, purely
   `backend/`, unblocks the core "agent executes AI tasks" capability that's
   currently just broken.
2. **Gap 5 step 3** (port the live RCE fix to `desktop/`'s relay copy) —
   security-critical, small, no design decisions needed.
3. **Gap 7** (`workspacePorts.*` relay) — small, mechanical, matches an
   existing pattern.
4. **Gap 1** (GitHub/GitLab → HTTP APIs) — the largest by method count, but
   mechanically similar to Jira/Linear's already-working pattern; needs a
   decision on the auth-flow question (keep PTY-based CLI login vs. real
   OAuth) before starting.
5. **Gap 3** (wire `automation.runNow` through `StepExecutors`) — needs Gap
   4 done first (executing through the currently-broken param shape would
   just extend the bug to a second caller).
6. **Gap 2** (credential plaintext-forwarding) — security design work, needs
   its own review before implementation.
7. **Gap 5 steps 1–2** (relay codebase merge/CI-check) — largest, most
   structural, benefits from having Gaps 1/3/4 already landed so there's
   less in-flight work to reconcile across two trees during the merge.
8. **Gap 6** (browser/computer/emulator) — needs a product decision first;
   not blocking anything else in this list.

## Sources

Every gap above is traced to a specific, already-verified finding — none of
this is speculative. Primary sources:
[`business-capabilities.md`](./business-capabilities.md),
[`../../frontend/api/backend-agent-execution-boundary.md`](../../frontend/api/backend-agent-execution-boundary.md)
(this session, 2026-08-15), and `specs/agent/api/gaps-and-findings.md` +
`specs/agent/api/compliance-audit-2026-08-15.md` (a separate, earlier audit
pass this session, backend↔agent wire-contract level). Re-verify
file:line citations in those source documents before treating any of them as
current — several already describe fixes landed *during* the audits that
produced them.

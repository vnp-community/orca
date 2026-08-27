# `ephemeralVm.*` in server/web mode — v2 design (not implemented)

## Status

**Design doc only — no code changed.** This is a deliberate outcome, not a
placeholder: see "Why a design doc instead of code" below. Written in
response to an explicit request to look past the existing "desktop-only,
don't port" classification (see `desktop-only-rpc-parity-gaps.md` §A) and
investigate whether a genuine v2 is groundable, understanding it would be
new-feature work rather than a mechanical port.

## What "ephemeral VM" actually means (read from the desktop source, not the name)

An `OrcaVmRecipe` (`desktop/src/shared/types.ts`) is **not** a VM description
— it's four shell command strings a repo author writes into that repo's own
`orca.yaml` (`environmentRecipes`):

```ts
export type OrcaVmRecipe = {
  id: string
  name: string
  create: string        // required
  suspend?: string
  resume?: string
  destroy?: string
  destroyDisabled?: boolean
}
```

`create`/`suspend`/`resume`/`destroy` are opaque shell command strings —
`gcloud compute instances create ...`, `docker run ...`, a Vagrant wrapper,
whatever the repo's own team wrote. `ephemeral-vm-recipe-process.ts`'s
`runRecipeCommand` runs the chosen command via `child_process.spawn(command,
{ cwd: repoPath, shell: true, env: {...process.env, ORCA_VM_MODE, ORCA_VM_INSTANCE_ID,
ORCA_RECIPE_ID, ORCA_REPO_PATH, ...} })`, inheriting the *executing machine's*
full environment, installed CLIs, and credentials. `create`/`resume` must
print one JSON object on stdout (`EphemeralVmRecipeResultSchema`) that names
how Orca should reach what got provisioned — either:

- `{ type: 'orca-server', pairingCode, projectRoot }` — the recipe stood up
  something already running an Orca Server/Agent; Orca pairs with it the same
  way any other `runtime-environment` pairing code works, or
- `{ type: 'ssh', target: {...}, projectRoot }` — the recipe stood up a bare
  host; **the machine that ran the recipe** then opens its own outbound SSH
  connection to that target (`ephemeral-vm-runtime-ssh.ts`'s
  `connectRuntimeOwnedSshTarget`, via desktop's own SSH connection registry,
  `ipc/ssh.ts`'s `getSshConnectionStore()`) and registers it as a hidden
  "runtime-owned" SSH target so the rest of Orca's fs/git providers can
  operate against it.

So "ephemeral VM recipes run on the local desktop host in v1"
(`ephemeral-vm-recipe-context.ts`, `getRecipeRepo`/`listRecipes`) is a precise
statement, not a vague one: it means the **shell command that IS the recipe**
executes as a child process of whatever machine is currently running the
Orca main process, with `cwd` set to `repo.path` on that same machine (Node's
`statSync(repoPath)` fails otherwise — confirmed in both
`runEphemeralVmRecipeStart`'s `validateRepoPath` and
`doctorEphemeralVmRecipe`). The guard fires specifically when
`repo.connectionId` is set (an SSH-connected repo, no full Agent on the other
end) — it does **not** check `repo.devServerId` today (see "A pre-existing
gap" below).

## Step 2 answer: confirmed — recipe execution belongs on the Dev Server, not the backend container

The task hypothesized two readings of "local desktop host": (a) wherever the
Orca *main process* runs (in server mode, the backend container), or (b)
wherever the repo's actual checkout lives, which in server mode is the
connected Dev Server (via `agent/`), matching this session's `cli.*` /
`preflight.check` relay precedent. **(b) is correct**, on two independent
grounds:

1. **`repo.path` is only real on the machine that hosts the repo.** In server
   mode, a Dev-Server-bound repo's `repo.path` is a path on that Dev Server's
   filesystem, never the backend container's. `runRecipeCommand`'s
   `cwd: repoPath` + `validateRepoPath`'s `statSync` would simply fail on the
   backend container (no such directory) for any Dev-Server-bound repo, same
   as it already does today for `repo.connectionId` repos.
2. **Running a repo-authored shell command in a shared multi-tenant backend
   container, with that container's own environment/credentials, is a real
   security and blast-radius problem** the moment more than one user's repos
   share that container — recipe scripts are arbitrary and repo-controlled,
   not Orca-authored. Running them on the one machine each user is already
   dedicated to (their connected Dev Server) contains that risk the same way
   `cli.install`, `git.exec`, and every other per-user command this session
   already relays does.

Concrete evidence this is the established shape, not a guess:

- `backend/src/main/runtime/rpc/methods/cli.ts` + `agent/src/relay/agent-cli-handler.ts`
  — this session's own precedent: "none of these methods has a legitimate
  local (Orca-backend-container) fallback... In server/web mode that machine
  is always a connected Dev Server."
- `backend/src/shared/execution-host.ts`'s `Repo` already carries a
  `devServerId` field and `getRepoExecutionHostId`/`getRepoProviderConnectionKey`
  already resolve "which transport does this repo's fs/git work go through"
  — SSH-target id or Dev-Server id, uniformly. **The repo→devServer routing
  ephemeralVm.* would need already exists**; it doesn't need to be invented.
- `backend/src/main/runtime/orca-runtime-repo-hooks.ts`'s `getRepoHooks` is
  the exact cross-transport pattern `listRecipes`/`listRecipeCatalog` need:
  local repos read `orca.yaml` off local disk via `loadHooks`; remote repos
  (SSH or Dev-Server, indistinguishable at this layer) go through
  `getRemoteFilesystemProvider(getRepoProviderConnectionKey(repo)).readFile(...)`.
  `environmentRecipes` lives in the same `orca.yaml`/hooks payload this
  function already reads — reading recipe *definitions* remotely is already a
  solved problem, not new work.

## Why a design doc instead of code — the genuine blocker

The 9 methods split cleanly into two groups once you follow where the recipe
result routes:

**Group 1 — reads a recipe definition (`listRecipes`, `listRecipeCatalog`,
`doctor`, `getCleanupCommand`) or reads a persisted runtime record
(`listRuntimes`, `attachWorkspace`'s status flip).** These are groundable
today, cheaply, using the `orca-runtime-repo-hooks.ts` pattern above for the
recipe-definition reads. `doctor` additionally does local `existsSync`/
`accessSync` checks on the recipe's command text
(`ephemeral-vm-recipe-doctor.ts`'s `checkCommandPath`) — portable the same
way, via a remote fs-provider stat/access instead of a raw Node call.

**Group 2 — actually runs a `create`/`suspend`/`resume`/`destroy` command
(`suspendWorkspace`, `resumeWorkspace`, `cleanup`, and transitively
`attachWorkspace`'s owning provision flow).** Running the shell command
itself on the Dev Server is *also* groundable — it's a straightforward new
`agent/src/relay/agent-ephemeral-vm-handler.ts`, shaped like
`agent-cli-handler.ts`/`agent-git-handler.ts`: `spawn(command, { shell: true,
cwd, env })`, same `ORCA_VM_*` env-var contract, same JSON-stdout result
parsing (`ephemeral-vm-recipe-runner.ts`'s logic ports almost verbatim).

**But when the recipe's result is `{ type: 'ssh', target }`, the flow forks
into infrastructure that does not exist anywhere in `agent/` today:**
`connectRuntimeOwnedSshTarget` needs the *executing machine* — now the Dev
Server — to become an **outbound SSH client** to a third host (the freshly
provisioned VM), register it as a hidden target, and expose fs/git
"providers" against it (`ssh-filesystem-dispatch.ts`/`ssh-git-dispatch.ts`)
so the rest of Orca can browse/edit/git-op inside it. Desktop's version of
this is real, mature, security-sensitive infrastructure: SSH identity-file/
agent-forwarding resolution, jump hosts, port-forward bookkeeping, relay
grace periods, a "hidden runtime-owned target" registry distinct from
user-visible SSH targets (`ipc/ssh.ts`'s `getSshConnectionStore()`).

I checked directly whether `agent/` already has any part of this to build
on: it has an `ssh2` dependency and several `agent/src/main/ssh/*` files
(`ssh-channel-multiplexer.ts`, `ssh-filesystem-stream-reader.ts`,
`ssh-git-response-stream-reader.ts`), but **all of them implement the
opposite direction** — the "Part B" transport where *Orca* reaches the Agent
*through* an inbound SSH tunnel (`deploy/agent/README.md`'s direct-SSH mode).
None of it is an outbound SSH *client* the Agent could use to reach a further
host; `grep -rn "from 'ssh2'" agent/src` (excluding tests) returns nothing.
Building that — SSH2 client integration, credential/identity-file resolution
on the Dev Server's filesystem, port-forward/jump-host handling, and a
provider-dispatch registration path backend can reach through a double hop
(backend → Dev Server relay → Dev Server's own new SSH client → VM) — is a
new subsystem on the order of what desktop already has in `ipc/ssh.ts` +
`ssh-filesystem-dispatch.ts` + `ssh-git-dispatch.ts` combined, not a same-
shape relay port like every other method this session has done. It is not
safe to build blind, in the same pass as 8 other methods, under the
concurrent-edit constraints already in play on this branch.

There's a second, independent reason Group 2 needs a product decision before
code, not just an engineering one: **backend's `Store` has no persisted
ephemeral-VM-runtime table at all** — no equivalent of desktop's
`ephemeral-vm-runtime-store.ts` (`listEphemeralVmRuntimes`/
`upsertEphemeralVmRuntime`/`updateEphemeralVmRuntimeStatus`, a local JSON
file under `app.getPath('userData')`). `listRuntimes`/`attachWorkspace`/
`suspendWorkspace`/`resumeWorkspace`/`cleanup`/`getCleanupCommand` all need
*some* durable, shared record of "which runtimes exist, what state are they
in" reachable by every backend replica handling RPCs for the same user/repo
— not a per-process file. This repo is mid-migration of server-mode state to
a single shared Postgres (`413f5c8da`, "ADR-021 — hợp nhất server-mode data
plane về 1 Postgres duy nhất", the tip commit on this branch as of writing).
Adding a new table/migration for this blind, next to an active data-plane
consolidation, is exactly the kind of thing that should be coordinated with
that work rather than guessed at in an unrelated pass.

## Options for whoever picks this up

**Option A — full parity port.** Build the outbound-SSH-client subsystem in
`agent/` (mirroring desktop's `ipc/ssh.ts` + provider dispatch), add the
Postgres-backed runtime-record table (coordinated with ADR-021), then port
all 9 methods feature-complete. Highest cost and highest risk; only choose
this if SSH-type recipes are known to be in real use.

**Option B — ship the groundable half now, gate the rest (recommended
starting point).** Implement Group 1 in full (`listRecipes`,
`listRecipeCatalog`, `doctor`, `getCleanupCommand`, `listRuntimes`) plus
Group 2 restricted to the `orca-server` connection-type result — which
reuses the *already-transport-agnostic* pairing-code environment mechanism
(`addEnvironmentFromPairingCode`) and needs no new SSH infrastructure, only
the new agent-side recipe-exec handler and the new runtime-record
persistence. For a recipe whose result is `{ type: 'ssh', ... }` in server
mode, return the same shape of explicit, clear error the code already
returns for `repo.connectionId` repos today (`'Ephemeral VM recipes run on
the local desktop host in v1.'` → a v2-appropriate message naming the real
gap), rather than silently failing. This is the same "ship the part that's
real, name the part that isn't" shape the existing `starNag.*`/`rateLimits.*`
partial ports already use elsewhere in this codebase
(`desktop-only-rpc-parity-gaps.md` §"Đã fix trong phiên này").

**Option C — hold, pending usage data.** `desktop-only-rpc-parity-gaps.md`
§A already classified `ephemeralVm.*` as "desktop-only, don't port" and
shipped a frontend-side error suppressor for it
(`desktop-only-rpc-error-suppressor.ts`, commit `b647b5119`) — i.e. there is
already a deliberate, shipped decision here, made by a prior pass, that this
investigation is revisiting on explicit request. Before investing in Option
A or B, it may be worth confirming whether any real repo's `orca.yaml` in
production actually defines `environmentRecipes`, and if so, whether any of
them use the `ssh` connection type versus exclusively `orca-server` — that
determines whether Option B's gap is theoretical or actually blocks real
usage.

## Files a future implementer should read first

- `desktop/src/main/runtime/rpc/methods/ephemeral-vm.ts` — the 9-method RPC
  surface to match (`provision`/`cancelProvision` stay out of scope; see its
  own doc comment — broadcast/streaming shape, not request/response).
- `agent/src/relay/agent-cli-handler.ts` + `backend/src/main/runtime/rpc/methods/cli.ts`
  — the concrete "relay to Dev Server" precedent to copy for Group 1/2.
- `backend/src/main/runtime/orca-runtime-repo-hooks.ts` — the concrete
  cross-transport (local/SSH/Dev-Server) pattern for reading a repo's
  `orca.yaml`/`environmentRecipes`, already handling the exact routing
  question this design needs.
- `backend/src/shared/execution-host.ts` — `getRepoExecutionHostId`/
  `getRepoProviderConnectionKey`, the existing repo→transport routing to
  reuse rather than reinvent.
- `desktop/src/shared/ephemeral-vm-recipe-runner.ts`,
  `ephemeral-vm-recipe-process.ts`, `ephemeral-vm-recipe-doctor.ts` — the
  behavior to port into the new agent-side handler (env-var contract,
  process-group kill-on-abort semantics, JSON result parsing, doctor checks).
- `desktop/src/main/ephemeral-vm-runtime-ssh.ts` + `desktop/src/main/ipc/ssh.ts`
  — what Option A's new agent-side SSH-client subsystem would need to match.
- `specs/backend/api/desktop-only-rpc-parity-gaps.md` §A/§C — the prior
  classification this design revisits, and the precedent for how other
  partial/gated ports in this codebase were framed and shipped.

## What this doc does NOT cover

`ephemeralVm.provision`/`ephemeralVm.cancelProvision` — out of scope here for
the same reason desktop's own RPC file excludes them: they stream stdout/
stderr over a broadcast IPC event decoupled from the request/response call,
not `defineMethod`'s shape. Any v2 for `attachWorkspace`'s owning "create a
new runtime" flow needs that streaming-provision redesign solved first
(likely `defineStreamingMethod`, if/when that exists in the backend RPC core)
— a separate piece of work from the 9 methods this doc analyzes.

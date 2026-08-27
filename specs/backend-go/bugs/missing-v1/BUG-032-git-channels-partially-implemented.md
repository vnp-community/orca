# BUG-032: `git.*` — 32 of 34 methods unimplemented in backend-go

**Service:** `git-gateway-service` (via `api-gateway`'s `wscompat` WS layer)
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Severity:** High — largest partial gap in the whole audit (32/34 methods missing), and `git.*` is core to the product's entire code-review/commit workflow.
**Symptom:** Every `git.*` call the frontend makes other than `git.status`/`git.diff` resolves through `notImplementedHandler` and errors out (see BUG-002 for the general "unregistered channel" failure mode).
**Status: ❌ Open**

---

## Description

`wscompat.registerGitChannels` (`channels.go:221-252`) wires exactly **two**
of the 34 `git.*` methods the frontend calls: `git.status` and `git.diff`.
Everything else — including `git.commit`, `git.push`, `git.pull`,
`git.checkout`, `git.stage`/`unstage`, history/branch/diff-comparison
lookups, conflict/rebase operations, and AI commit-message/PR-field
generation — falls through to `registry.go`'s `notImplementedHandler` and
errors immediately.

Do not re-report `git.status` or `git.diff` as missing — they are wired for
real against `git-gateway-service`'s `GetStatus`/`GetDiff` gRPC methods
(`channels.go:222`, `channels.go:237`).

Critically, **not all 32 missing methods are in the same state server-side**:

- **4 methods have a complete, real backing RPC already implemented in
  `git-gateway-service` and exposed over REST — they just aren't wired into
  `wscompat`:** `git.commit` → `Commit`, `git.push` → `Push`, `git.pull` →
  `Pull`, `git.generateCommitMessage` → `GenerateCommitMessage`. These are
  the only other RPCs `git-gateway-service`'s proto defines
  (`backend-go/proto/orca/gitgateway/v1/gitgateway.proto:10-17`), and each
  has a working usecase (`internal/usecase/commit.go`, `push.go`, `pull.go`,
  `generate_commit_message.go`) and gRPC server method
  (`internal/adapter/grpc/server.go:67` Commit, `:79` Push, `:91` Pull,
  `:105` GenerateCommitMessage), already called from the REST layer at
  `internal/adapter/httpgateway/git_routes.go:26-29` (`POST /v1/git/commit`,
  `/push`, `/pull`, `/commit-message`). Adding these to `wscompat` is a thin
  wrapper, not new backend work.
- **The remaining 28 methods have no backing RPC anywhere in
  `git-gateway-service`'s proto at all** — `gitgateway.proto` defines only 6
  RPCs total (`GetStatus`, `GetDiff`, `Commit`, `Push`, `Pull`,
  `GenerateCommitMessage`; `gitgateway.proto:10-17`). There is no
  `CheckIgnored`, `SubmoduleStatus`, `History`, `BranchCompare`,
  `CommitCompare`, `BranchDiff`, `CommitDiff`, `ForkSync`, `Stage`,
  `Unstage`, `Discard`, bulk-variants, `Checkout`, `Fetch`,
  `LocalBranches`, `RemoteCommitUrl`, `RemoteFileUrl`, `UpstreamStatus`,
  `FastForward`, `RebaseFromBase`, `AbortMerge`, `AbortRebase`,
  `ConflictOperation`, cancel-generation, or `GeneratePullRequestFields`
  RPC/message defined. These are genuinely unbuilt server-side, not just
  unwired.

`git.exec` is intentionally absent from both the frontend and backend-go —
per `specs/frontend/api/rpc-catalog.md:13` and `:518`, the raw numstat
passthrough was deliberately removed in favor of `git.status`'s own
per-file stats. Do not report it as a missing method.

---

## Already wired (do not re-report)

| Method | What it does | File:line |
|---|---|---|
| `git.status` | Calls `GitGatewayServiceClient.GetStatus` | `channels.go:222-235` |
| `git.diff` | Calls `GitGatewayServiceClient.GetDiff` | `channels.go:237-251` |

---

## Missing channels

| Method | Frontend call site | Notes |
|---|---|---|
| `git.commit` | `use-code-review.ts`, `useGit.ts`, `runtime-git-client.ts` | Backing RPC exists: `Commit` (`gitgateway.proto:13`, `server.go:67`, REST at `git_routes.go:26`). Wrapper-only gap. |
| `git.push` | `runtime-git-client.ts` | Backing RPC exists: `Push` (`gitgateway.proto:14`, `server.go:79`, REST at `git_routes.go:27`). Wrapper-only gap. |
| `git.pull` | `runtime-git-client.ts` | Backing RPC exists: `Pull` (`gitgateway.proto:15`, `server.go:91`, REST at `git_routes.go:28`). Wrapper-only gap. |
| `git.generateCommitMessage` | `commit-message-generator.tsx`, `useGit.ts`, `runtime-git-client.ts` | Backing RPC exists: `GenerateCommitMessage` (`gitgateway.proto:16`, `server.go:105`, REST at `git_routes.go:29`). Wrapper-only gap. |
| `git.abortMerge`, `git.abortRebase`, `git.branchCompare`, `git.branchDiff`, `git.bulkDiscard`, `git.bulkStage`, `git.bulkUnstage`, `git.cancelGenerateCommitMessage`, `git.cancelGeneratePullRequestFields`, `git.checkIgnored`, `git.checkout`, `git.commitCompare`, `git.commitDiff`, `git.conflictOperation`, `git.discard`, `git.discoverCommitMessageModels`, `git.fastForward`, `git.fetch`, `git.forkSync`, `git.generatePullRequestFields`, `git.history`, `git.localBranches`, `git.rebaseFromBase`, `git.remoteCommitUrl`, `git.remoteFileUrl`, `git.stage`, `git.submoduleStatus`, `git.unstage`, `git.upstreamStatus` (28 methods) | Overwhelmingly `renderer/src/runtime/runtime-git-client.ts`, plus `BranchManager.tsx`, `DiffViewer.tsx`, `GitHistory.tsx`, `useGit.ts` | **No backing RPC exists anywhere in `git-gateway-service`.** `gitgateway.proto` defines only the 6 RPCs listed above (`gitgateway.proto:10-17`) — none of these 28 methods have a proto message, usecase, or gRPC server method. Genuinely unbuilt, not just unwired. |

---

## Dispatch model

🔀 **Dynamic** (old TS backend) — every method used an identical pattern,
decided per-call by whether the target worktree had a `connectionId` (relay
to Dev Server Agent) or was local. 8 methods (`checkIgnored`,
`submoduleStatus`, `history`, `branchCompare`, `commitCompare`,
`branchDiff`, `commitDiff`, `forkSync`) mapped to dedicated agent RPC
methods in the old backend; the rest composed from a generic `git.exec`
passthrough (deliberately not carried forward to the new frontend — see
Description). Zero Postgres involvement in the git chain itself;
`git-gateway-service`'s new design (`gitgateway.proto:7-8`) keeps this same
"stateless dispatcher — resolve owning host, execute locally or relay"
shape for the 6 RPCs it does define.

---

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:221-252` — `registerGitChannels`
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto:10-17` — `GitGatewayService` (full 6-RPC surface)
- `backend-go/services/git-gateway-service/internal/adapter/grpc/server.go:48,59,67,79,91,105` — `GetStatus`/`GetDiff`/`Commit`/`Push`/`Pull`/`GenerateCommitMessage`
- `backend-go/services/git-gateway-service/internal/usecase/commit.go`, `push.go`, `pull.go`, `generate_commit_message.go` — usecases backing the 4 wrapper-only-gap methods
- `backend-go/services/api-gateway/internal/adapter/httpgateway/git_routes.go:22-29` — REST equivalents already calling all 6 RPCs
- `backend-go/services/api-gateway/internal/domain/registry.go:90` — `/v1/git` → `git-gateway-service`, `RouteWired`
- `specs/frontend/api/rpc-catalog.md:196-234` — full `git.*` frontend call-site table (34 methods)
- `specs/frontend/api/rpc-catalog.md:13,518` — `git.exec` deliberately not carried forward
- `specs/backend-go/bugs/api-v1/BUG-002-missing-channel-registrations.md` — sibling bug-report format this follows

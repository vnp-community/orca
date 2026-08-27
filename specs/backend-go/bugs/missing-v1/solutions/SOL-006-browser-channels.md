# SOL-006: `browser.*` is a worktree-scoped remote pane — extend the Dev Server Agent relay, flag the agent-side gap

**Resolves:** [BUG-006](../BUG-006-browser-channels-not-implemented.md)
**Service:** `infra-fleet-service` (one additive proto field, reused `Relay` RPC, new `browser_profiles` CRUD table) + `api-gateway` (new `wscompat` channels)
**Affected files (proposed):**
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (reuse `Relay`, one additive field on `ResolveConnectionRequest` — shared with SOL-005)
- `backend-go/services/infra-fleet-service/internal/usecase/{list,create,delete}_browser_profile.go` (new)
- `backend-go/services/infra-fleet-service/internal/adapter/postgres/browser_profile_repository.go` (new)
- `backend-go/services/infra-fleet-service/migrations/000X_browser_profiles.up.sql` (new)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_browser.go` (new)
- `agent/` (new browser-driving capability — **flagged out of scope**, see below)
**Status:** 🚧 Partially implemented — agent + backend-go layers implemented and tested end-to-end (2026-08 pass); frontend dispatch to the new relay path is a documented, separately-scoped open gap — see TASK-036's "Status by layer" section.

---

## Resolving the dispatch-model uncertainty BUG-006 flags

BUG-006's caveat is the crux of this whole solution: `specs/frontend/api/backend-agent-execution-boundary.md:161`
classifies `browser.*` as 🏠 backend-local ("Drives Electron `webContents`/CDP
**inside the Orca backend process itself**"), but every real call site in
`browser-pane-remote.tsx` passes a `worktree` selector and resolves its
target via `toRuntimeWorktreeSelector(worktreeId)` — i.e. scoped to
whichever host owns that worktree, not unconditionally the backend's own
process. These two descriptions cannot both be describing the same
15-method surface.

Checked both documents BUG-006 names as the disambiguation source:

- **`infra-fleet-service.md`**: no mention of a browser/CDP/webContents
  concept anywhere — its scope is SSH targets, dev servers, connection
  lifecycle, PTY routing, port forwarding, workspace port scanning. Nothing
  browser-pane-shaped exists there today.
- **`08-inter-service-communication.md`**'s "Talking to the Dev Server
  Agent" section: also silent on browser panes specifically, but its
  framing is directly applicable — it describes the Dev Server Agent as the
  execution plane for exactly this class of thing: work that must happen
  *on the target host*, reached via `infra-fleet-service`'s relay, with
  `agent/` changes explicitly out of scope for this redesign.

Neither doc confirms a browser-pane RPC exists in the target architecture —
but the **worktree-scoping evidence in the actual frontend call sites**
(`browser-pane-remote.tsx:14,41,201,349-361`) is concrete and verifiable,
where the execution-boundary doc's 🏠 classification is, per BUG-006's own
caveat, plausibly describing the unrelated `window.api.browser` Electron
surface instead (the same disambiguation `specs/frontend/api/ipc-surface.md`
already draws explicitly). **Conclusion: this is a remote-browser-pane
concept, driven via the Dev Server Agent, not a backend-local Electron
concept** — the execution-boundary doc's `browser.*` entry should be treated
as describing `window.api.browser`, not this RPC namespace, pending someone
tracing the actual old TS backend's `browser.*` RPC handler source to
confirm definitively (BUG-006's own instruction, not fully dischargeable
from specs alone).

Given that, the architecturally consistent home is exactly what BUG-006
itself suggests as the likely answer: `infra-fleet-service`, extending the
same relay pattern already used for PTY/git/fs/ports — **not** a new
service, and **not** inventing a new communication mechanism. It reuses the
same generic `Relay` RPC (`infrafleet.proto:103-116`) SOL-004 and SOL-005
both build on, for the same reason: this is coordination/routing work
(“which host does this browser pane live on, relay the command there”), and
`infra-fleet-service` already owns that exact class of decision.

---

## The honest limit of this proposal — a real, larger `agent/` gap

Unlike SOL-004's `accounts.*` (trivial fs read/write, well within the
agent's existing capabilities) and SOL-005's `testConnection` (a decrypt +
one lightweight HTTP call), driving a browser pane means the Dev Server
Agent must be able to **launch and control a full browser process** —
navigate, inject input, evaluate JS, manage tabs, stream frames back
(`browser.screencast`), and read/import OS-level browser profile data
(`profileDetectBrowsers`/`profileImportFromBrowser`). Nothing in
`specs/agent/api/` (per `infra-fleet-service.md` §10's own reference to that
directory) documents the Dev Server Agent having this capability today —
this is not a small companion change the way SOL-004's 4 fs-read/write
methods are.

Per `08-inter-service-communication.md`'s "Talking to the Dev Server Agent"
section, `agent/` changes are explicitly out of scope for "the Go rewrite
of `backend/`." This solution's `backend-go`-side plumbing (below) is
complete and internally consistent, but **flagging prominently**: it is
inert — every relayed `browser.*` call will fail with "method not found"
from the agent's own JSON-RPC dispatcher — until a substantial, separately
scoped `agent/` change ships a browser-driving capability that does not
exist there today. This is a bigger ask than a simple flag-and-defer; it is
close to `02-microservices-decomposition.md`'s "Browser/computer/emulator
automation... a product decision to make before porting, not a mechanical
translation" framing for the *desktop* case, just for a different
(worktree-scoped) variant of the same underlying question: **is backend-go
expected to support live remote browser panes at all, and if so, is
`agent/` growing a CDP/Playwright driver its own team's roadmap already
plans, or does this wait?** That product decision is not backend-go's to
make unilaterally, and this proposal does not attempt to make it — it only
establishes where the plumbing goes *if* the answer is yes.

---

## Missing channels, grouped by shared pattern

| Group | Methods | Design |
|---|---|---|
| **A — live input/eval relay** | `eval`, `keypress`, `mouseDown`, `mouseMove`, `mouseUp`, `mouseWheel`, `viewport` | One generic relayed command, `Relay(connectionId, "browser.<op>", params)` — same shape as PTY write, no per-op proto message |
| **B — tab lifecycle** | `tabCreate`, `tabClose` | Same relay mechanism, CRUD-shaped |
| **C — profile management** | `profileList`, `profileCreate`, `profileDelete`, `profileClearDefaultCookies`, `profileDetectBrowsers`, `profileImportFromBrowser` | `profileList`/`profileCreate`/`profileDelete` are Postgres-backed metadata CRUD in `infra-fleet-service` (mirrors `ssh_targets`' pattern); `profileClearDefaultCookies`/`profileDetectBrowsers`/`profileImportFromBrowser` are live agent operations relayed the same way as Group A — a profile's actual browser-data directory lives on the dev server, not in Postgres |

---

## Design — Group A & B: relay via the existing `Relay` RPC

Identical mechanism to SOL-004's `accounts.*` and SOL-005's `testConnection` —
no new `InfraFleetService` RPC. The one gap: `Relay` needs a
`connectionId`, but every `browser.*` call site passes a `worktree`
selector, not a `connectionId` — the exact resolution problem `git.*`
already has, solved there by `git-gateway-service` doing the
worktree→connection resolution internally
(`git-gateway-service.md`'s dispatch, called via `project-service`/
`infra-fleet-service`). `api-gateway`'s `wscompat` layer has no equivalent
internal resolver for browser panes, so this solution reuses the same
additive field SOL-005 proposes on `ResolveConnectionRequest`, generalized
to accept a worktree key too:

```protobuf
// infrafleet.proto — additive fields (shared with SOL-005's dev_server_id addition)
message ResolveConnectionRequest {
  string connection_id = 1;
  string dev_server_id = 2; // SOL-005
  string worktree_id = 3;   // NEW for SOL-006 — resolve the connectionId
                             // currently bound to a worktree, mirroring
                             // CreateConnectionRequest's (dev_server_id,
                             // repo_path, worktree_id) tuple
                             // (infrafleet.proto:93-97) in reverse.
}
```

```go
// channels_browser.go
package wscompat

import (
	"context"
	"encoding/json"
	"fmt"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

func registerBrowserChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	// Group A + B — one relay handler per channel, all sharing
	// registerBrowserRelay's resolve-then-relay logic. Representative
	// signatures only shown; each Register call differs only in channel
	// name and relayed agent method name.
	for _, op := range []string{
		"eval", "keypress", "mouseDown", "mouseMove", "mouseUp", "mouseWheel",
		"viewport", "tabCreate", "tabClose",
	} {
		registerBrowserRelay(r, client, "browser."+op, "browser."+op)
	}
}

// registerBrowserRelay is the single representative sketch for all 9
// Group A/B channels — each op's params shape differs (viewport carries
// width/height, mouseMove carries x/y, etc.) but the resolve-then-relay
// skeleton is identical, so params are passed through opaquely rather than
// typed per-op, mirroring RelayRequest.params_json's own "no per-method
// typed message" design choice (infrafleet.proto:103-111).
func registerBrowserRelay(r *Registry, client infrafleetv1.InfraFleetServiceClient, channel, agentMethod string) {
	r.Register(channel, func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("BROWSER_MISSING_ARGS: %s requires a params object", channel)
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(args[0], &raw); err != nil {
			return nil, err
		}
		var worktreeID string
		if wt, ok := raw["worktree"]; ok {
			_ = json.Unmarshal(wt, &worktreeID)
		}
		if worktreeID == "" {
			return nil, fmt.Errorf("BROWSER_NO_WORKTREE: %s requires a worktree selector", channel)
		}

		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()

		resolved, err := client.ResolveConnection(rpcCtx, &infrafleetv1.ResolveConnectionRequest{WorktreeId: worktreeID})
		if err != nil {
			return nil, err
		}
		if !resolved.GetConnected() {
			return nil, fmt.Errorf("BROWSER_NO_CONNECTION: worktree %s has no bound dev server", worktreeID)
		}

		resp, err := client.Relay(rpcCtx, &infrafleetv1.RelayRequest{
			ConnectionId: resolved.GetDevServer().GetId(), // see infra-fleet-service.md §4: connectionId, not dev_server_id — verify Relay's expected key against ResolveConnectionResponse's actual shape at implementation time
			Method:       agentMethod,
			ParamsJson:   string(args[0]),
		})
		if err != nil {
			return nil, err
		}
		var result map[string]any
		if err := json.Unmarshal([]byte(resp.GetResultJson()), &result); err != nil {
			return nil, err
		}
		return result, nil
	})
}
```

`browser.viewport`'s `worktree`/`page`/`width`/`height` params (BUG-006's
call-site note) pass through unchanged inside `params_json` — the agent's
new `browser.viewport` method (once it exists) reads them directly, exactly
as `Relay`'s doc comment already promises for any caller
(`relay.go:19-27`).

**A note on `browser.screencast`**: BUG-006's 15-method list doesn't include
it, but the execution-boundary doc names it alongside `browser.*` as a
namesake concern. A live video/frame stream back from the agent cannot be a
unary `Relay` call — it needs a server-streaming RPC, the same
"resolve once via a unary call, then open a dedicated streaming path" shape
`infra-fleet-service.md` §7 already documents for terminal I/O ("the data
stream is a dedicated server-streaming RPC once the route is resolved, not
through this RPC per-byte"). Flagged here as a design note for whoever picks
up screencast specifically, not designed in this proposal since it's outside
BUG-006's assigned 15 methods.

---

## Design — Group C: browser profile CRUD

The 3 metadata-only operations (`profileList`/`profileCreate`/
`profileDelete`) get a small Postgres table in `infra-fleet-service`,
mirroring `ssh_targets`' shape — a profile is tenant/dev-server-scoped
metadata (name, source browser, default flag), not a place to store cookie
data itself:

```sql
-- infra-fleet-service migrations
CREATE TABLE browser_profiles (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      UUID NOT NULL,
  dev_server_id  UUID NOT NULL REFERENCES dev_servers(id),
  name           TEXT NOT NULL,
  source_browser TEXT,               -- e.g. "chrome", "firefox" — set by profileImportFromBrowser
  is_default     BOOLEAN NOT NULL DEFAULT false,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

```go
// usecase/list_browser_profiles.go — representative of the 3 CRUD ops
type ListBrowserProfiles struct {
	repo BrowserProfileRepository // .List/.Create/.Delete — new port
}

func (uc *ListBrowserProfiles) Execute(ctx context.Context, tenantID, devServerID string) ([]domain.BrowserProfile, error) {
	return uc.repo.List(ctx, tenantID, devServerID)
}
```

The other 3 (`profileClearDefaultCookies`, `profileDetectBrowsers`,
`profileImportFromBrowser`) are **not** Postgres reads — they act on the
dev server's actual filesystem/installed-browser state, so they route
through the same `Relay` mechanism as Group A, keyed by `dev_server_id`
directly (no worktree involved — a profile is a dev-server-level resource,
not a worktree-level one, per `browser.ts`'s call sites which pass no
`worktree` param for these 3, unlike the worktree-scoped pane-control
methods).

| Channel | Mechanism |
|---|---|
| `browser.profileList` | Postgres read via `BrowserProfileRepository.List` |
| `browser.profileCreate` | Postgres write via `.Create` |
| `browser.profileDelete` | Postgres write via `.Delete` |
| `browser.profileClearDefaultCookies` | `Relay(devServerID, "browser.profileClearDefaultCookies", {...})` |
| `browser.profileDetectBrowsers` | `Relay(devServerID, "browser.profileDetectBrowsers", {})` — agent scans installed browsers on its host |
| `browser.profileImportFromBrowser` | `Relay(devServerID, "browser.profileImportFromBrowser", {...})` — agent reads an OS browser's profile dir, then this usecase persists the resulting `BrowserProfile` metadata row via `.Create` |

---

## Test plan

- `infra-fleet-service/internal/usecase/list_browser_profiles_test.go` (+
  create/delete) — fake repo, tenant-scoping enforced.
- `infra-fleet-service`: `ResolveConnectionRequest.worktree_id` — resolving
  by worktree returns the same result a by-connection-id resolve of the
  live connection bound to that worktree would (mirrors SOL-005's
  equivalent `dev_server_id` test).
- `channels_browser_test.go` — one test per Group A/B channel: fake
  `ResolveConnection` + `Relay`, assert `worktree` is required and missing
  it fails fast without calling `Relay`; assert `params_json` passes the
  full raw args object through unmodified.
- `channels_browser_profiles_test.go` — Group C's 3 Postgres-backed
  channels against a fake `BrowserProfileRepository`; the 3 relay-backed
  ones against a fake `Relay` client, same pattern as Group A/B.
- **Explicitly no agent-side test plan here** — that belongs to whichever
  `agent/`-scoped effort implements the browser-driving capability this
  proposal's relay plumbing depends on; out of scope per the flag above.

## References

- `specs/backend-go/tdd/architecture/02-microservices-decomposition.md` — "Browser/computer/emulator automation" out-of-scope framing for the *desktop* case, the closest existing precedent for this proposal's own scope-limit flag
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md` — "Talking to the Dev Server Agent," `agent/` out-of-scope boundary (Option A rationale)
- `specs/backend-go/tdd/services/infra-fleet-service.md` §2,§4,§7,§10 — coordination/execution split; confirms no existing browser-pane concept in this service's scope today
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:65-116` — `ResolveConnection`/`Relay`, reused/extended (shared additive field with SOL-005)
- `backend-go/services/infra-fleet-service/internal/usecase/relay.go` — `Relay` usecase, reused unmodified
- `specs/backend-go/bugs/missing-v1/BUG-006-browser-channels-not-implemented.md` — the dispatch-model caveat this solution resolves, full 15-method call-site table
- `specs/frontend/api/backend-agent-execution-boundary.md:161` — the ambiguous 🏠 classification this solution concludes describes `window.api.browser`, not this RPC namespace
- `specs/frontend/api/ipc-surface.md` — confirms `window.api.browser` is the separate, not-RPC-migrated Electron surface this namespace must not be conflated with
- `frontend/src/renderer/src/components/browser-pane/browser-pane-remote.tsx:14,41,201,349-361` — the worktree-scoping evidence this solution's dispatch-model conclusion rests on
- `specs/backend-go/bugs/missing-v1/solutions/SOL-004-accounts-channels.md`, `SOL-005-aiprovider-channels.md` — the shared `Relay`-reuse pattern this solution follows

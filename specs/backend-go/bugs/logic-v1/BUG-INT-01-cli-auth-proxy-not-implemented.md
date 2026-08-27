# BUG-INT-01: CLI Auth Proxy (gh/glab via SSH Relay) has no backend-go implementation — replaced by a hardcoded stub

**Business Logic:** [BL-INT-01](../../../../docs/logic/remote-integration/BL-INT-01-cli-auth-proxy.md) — CLI Auth Proxy (GitHub/GitLab qua SSH Relay)
**Priority (per spec):** P1
**Status:** NOT_IMPLEMENTED
**Severity:** High
**Symptom:** A user in Web Server mode who wants to check `gh`/`glab` CLI auth status on their Dev Server, or click "Login with GitHub" to run an interactive `gh auth login` over a relayed PTY, gets a permanently-false result — the backend never contacts any Dev Server, never runs `gh`/`glab`, and there is no login-flow endpoint at all. The preflight panel will always show GitHub/GitLab CLI as "not installed."

---

## Spec summary

BL-INT-01 describes proxying `gh`/`glab` CLI preflight checks and interactive OAuth login (`gh auth login` over a relayed PTY) to a Dev Server via SSH relay, since the CLI tools live on the Dev Server, not the (possibly containerized) Orca Web Server. It requires per-user config-dir isolation (`GH_CONFIG_DIR=~/.config/gh/<userId>/`, `GLAB_CONFIG_DIR=~/.config/glab-cli/<userId>/`) injected as env vars on the relayed SSH exec.

## What backend-go has

Only a hardcoded, always-false local stub — no relay, no CLI invocation, no per-user isolation, no login flow:

```go
// backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:565-573
func registerPreflightChannels(r *Registry) {
	r.Register("preflight.check", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		return map[string]any{
			"git":  map[string]any{"installed": true},
			"gh":   map[string]any{"installed": false, "authenticated": false},
			"glab": map[string]any{"installed": false, "authenticated": false},
		}, nil
	})
}
```

The handler's own doc comment (`channels.go:551-564`) states the architectural reason: "scm-integration-service is a direct OAuth API client, deliberately NOT a `gh`/`glab` CLI wrapper. Reporting installed:false/authenticated:false for both is the honest answer." Confirmed: `backend-go/services/scm-integration-service/` implements OAuth directly (`internal/usecase/start_oauth_flow.go`, `complete_oauth_flow.go`, `internal/adapter/oauth/client.go`) rather than proxying to a CLI on a Dev Server.

## What's missing

- No SSH-relay proxy of any preflight/auth check to a Dev Server — `preflight.check` never looks up a DevServer record or dials anywhere (`channels.go:565-573` is a pure local literal, no downstream call per its own comment at `:553-554`).
- No `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` per-user env-var injection anywhere in backend-go (confirmed: `grep -rn "GH_CONFIG_DIR\|GLAB_CONFIG_DIR"` across all of `backend-go/` returns no matches).
- No `gh`/`glab` CLI invocation anywhere in backend-go (confirmed: no `"gh"`/`"glab"` command-exec call sites; the only `gh`/`glab` string hits in backend-go are an unrelated test fixture and generated gRPC code for `scmintegration.v1`).
- No `github.startAuthLogin`-equivalent RPC/channel — no PTY-based interactive CLI login flow exists; `grep -rn "startAuthLogin\|AuthLogin"` under `backend-go/` finds nothing.
- No `RpcMethodContext`-style `devServerManager` injection into a preflight/auth handler — `preflight.check`'s handler signature (`ctx, Identity, args`) has no DevServer resolution step at all.

The functional need this spec addresses (authenticating GitHub/GitLab) is served by a completely different, already-real mechanism in backend-go — direct OAuth via `scm-integration-service` — but that is not what BL-INT-01 documents, and it does not cover the "run `gh`/`glab` on the Dev Server for tools that need the CLI specifically" case at all.

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:551-573` — `registerPreflightChannels`, the hardcoded stub and its doc comment
- `backend-go/services/scm-integration-service/internal/usecase/start_oauth_flow.go`, `complete_oauth_flow.go` — the actual (different) GitHub/GitLab auth mechanism backend-go uses instead
- `backend-go/services/scm-integration-service/internal/adapter/oauth/client.go` — direct OAuth client
- `docs/logic/remote-integration/BL-INT-01-cli-auth-proxy.md` — spec

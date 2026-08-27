# SOL-SSH-02: Version-check before deploy, retry-with-backoff, hash-mismatch re-upload, and crash diagnostics for the relay deploy pipeline

**Resolves:** [BUG-SSH-02](../BUG-SSH-02-deploy-relay-partial.md)
**Service:** `infra-fleet-service`
**Affected files (proposed):**
- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/deploy.go`
- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/launch.go`
- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/provisioner.go`
- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/config.go`
- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/diagnostics.go` (new)
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

`infra-fleet-service.md` §3 already specifies `BootstrapFleetTarget` as a
**streaming** RPC specifically "so callers can show live progress, same
shape as TS's `ssh-relay-deploy*.ts`" (`infra-fleet-service.md:107-110`) —
i.e. the TDD anticipates a deploy pipeline with distinguishable steps
(version-check, upload, verify, launch, diagnose-on-failure) worth reporting
individually, not the current all-or-nothing `Provisioner.Provision`. This
solution's step decomposition (version-probe → conditional-deploy →
retry-wrapped-upload → launch → diagnose-on-failure) is exactly the shape a
future `BootstrapFleetTarget` stream would report progress against, even
though wiring the streaming RPC itself is out of this bug's scope (BUG-SSH-02
is about `ssh.connect`'s existing synchronous path, which is what
`EstablishConnection`→`agent.Health`→`getOrProvisionSession` actually drives
today per `establish_connection.go:66-68` and `client.go:179-200`).

**Scope boundary — what this solution does NOT redesign**: the
single-JS-bundle-run-via-`node` model (vs. a native `orca-relay` binary with
OS/arch-specific selection) is a deliberate, already-documented scope
reduction (`sshrelay`'s package doc comment: "No multi-platform bundle
resolution ... this scaffold runs one platform"). Building real multi-arch
native-binary distribution is a separate, larger initiative (new build
pipeline, artifact storage, per-arch signing) that doesn't fit a bug-fix
solution — this solution instead makes the *existing* single-bundle model's
failure modes diagnosable (remote OS/arch surfaced in error diagnostics) and
version-aware (skip redundant redeploys), without attempting the native-binary
initiative. Flagged explicitly since BUG-SSH-02 lists OS/arch selection as
"missing" — this solution treats it as accepted-scope, not silently ignored.

**BR-SSH-06 (random session-expiring relay token)**: `provisioner.go:118-127`'s
own doc comment states no token check exists for relay-ssh *by design* — "the
SSH connection is already the trust boundary." This matches
`infra-fleet-service.md` §9's per-transport trust-model list exactly: *"the
SSH connection itself as the trust boundary for `relay-ssh`"*
(`infra-fleet-service.md:521-522`) — i.e. the TDD itself endorses this as
correct, not a gap. This solution does not add a token; BUG-SSH-02's finding
here should be read as "confirmed by design," matching how BUG-SSH-02's own
report already frames it ("by design, not by omission").

## Design — version check before upload (BR-SSH-07)

```go
// internal/adapter/sshrelay/version_check.go (new)

// remoteVersion probes an already-deployed relay for its running
// AgentVersion by attempting a lightweight handshake-only connect against
// remoteDir/remoteAgentFile if it exists — cheaper than deploy()'s SFTP
// upload+checksum round trip. Returns ("", false, nil) when no prior
// deployment exists (first-time provision), which Provisioner treats as
// "must deploy," not an error.
func remoteVersionAndPresence(ctx context.Context, conn *sshconn.Connection) (version string, present bool, err error) {
    out, _, err := conn.RunCommand(ctx, fmt.Sprintf(
        `test -f %s && node -e "console.log(require(%s).AGENT_VERSION||'unknown')" 2>/dev/null || true`,
        shellQuote(remoteDir+"/"+remoteAgentFile), jsStringLiteral("./"+remoteDir+"/"+remoteAgentFile)))
    if err != nil {
        return "", false, err // treat a probe failure as "must redeploy", not fatal
    }
    v := strings.TrimSpace(out)
    return v, v != "", nil
}
```

`Provisioner.Provision` gains a version-gate before `deploy()`:

```go
func (p *Provisioner) Provision(ctx context.Context, devServer domain.DevServer) (devserveragent.Transport, devserveragent.HandshakeInfo, error) {
    // ... resolve target, conn := p.connector.Connect ... (unchanged)

    remoteDir := sshRelayRemoteDir
    if version, present, verr := remoteVersionAndPresence(ctx, conn); verr == nil && present && version == p.cfg.OrcaVersion {
        // BR-SSH-07: already the right version — skip the SFTP upload
        // entirely. Any node-launch failure below still falls through to
        // the normal deploy-then-retry path (see deployWithRetry), so a
        // stale/corrupt-but-version-matching bundle isn't a dead end.
    } else if _, err := deployWithRetry(ctx, conn, p.cfg); err != nil {
        _ = conn.Close()
        return nil, devserveragent.HandshakeInfo{}, err
    }

    transport, err := launch(conn, remoteDir, devServer.ID)
    // ... unchanged from here, except receiveHandshake's failure path
    // now collects diagnostics (see below) instead of returning a bare
    // timeout error.
}
```

`AGENT_VERSION` needs to actually be embedded in the bundle for this probe
to work — if `agent/out/agent.js`'s build doesn't already export a version
constant, that's a one-line `agent/` build-config addition (embedding
`ORCA_VERSION`/`package.json` version at build time), flagged here as a
**small `agent/` build-tooling change this solution depends on** — not a
runtime protocol change, and not in the same category as the reconnect-mode
gap `SOL-SSH-03` flags.

## Design — retry-with-backoff on upload/checksum failure (A1, A2)

```go
// internal/adapter/sshrelay/deploy.go

// deployWithRetry wraps deploy() with up to 3 attempts (A1) — a checksum
// mismatch (A2) triggers exactly one re-upload-and-recheck before giving
// up, matching the spec's "re-upload once more, then refuse to connect"
// flow rather than retrying indefinitely on a persistent mismatch (which
// would just mask a real corruption/tampering signal).
func deployWithRetry(ctx context.Context, conn *sshconn.Connection, cfg Config) (string, error) {
    const maxNetworkRetries = 3
    var lastErr error
    for attempt := 0; attempt < maxNetworkRetries; attempt++ {
        dir, err := deploy(ctx, conn, cfg)
        if err == nil {
            return dir, nil
        }
        lastErr = err
        if isChecksumMismatch(err) {
            // A2: exactly one immediate re-upload-and-recheck, not folded
            // into the network-retry budget above (a mismatch isn't a
            // transient network blip — retrying it identically 3x would
            // just repeat the same corrupted transfer).
            if dir, rerr := deploy(ctx, conn, cfg); rerr == nil {
                return dir, nil
            } else if isChecksumMismatch(rerr) {
                return "", fmt.Errorf("sshrelay: relay bundle checksum mismatch persisted after re-upload — refusing to launch a possibly-corrupted/tampered bundle: %w", rerr)
            } else {
                lastErr = rerr
            }
            break // don't network-retry after a checksum-specific failure path
        }
        if attempt < maxNetworkRetries-1 {
            time.Sleep(backoffDelay(attempt)) // 500ms, 1s, 2s — small, deploy is on the connect-latency-sensitive path
        }
    }
    return "", fmt.Errorf("sshrelay: deploy failed after %d attempts: %w", maxNetworkRetries, lastErr)
}

// isChecksumMismatch distinguishes deploy()'s checksum-mismatch error from
// a network/SFTP error — a typed sentinel (ErrChecksumMismatch, wrapped by
// deploy.go's existing fmt.Errorf) rather than string-matching, so this
// dispatch survives message wording changes.
func isChecksumMismatch(err error) bool {
    return errors.Is(err, ErrChecksumMismatch)
}
```

`deploy.go`'s existing checksum-mismatch return (`deploy.go:76-78`) becomes
`fmt.Errorf("...: %w", ErrChecksumMismatch)` so `isChecksumMismatch` can
`errors.Is` it — the only change to `deploy()` itself; the retry loop lives
entirely in the new `deployWithRetry` wrapper, keeping `deploy()`'s existing
single-attempt contract intact for anything that still wants it directly
(e.g. a future `BootstrapFleetTarget` stream step).

## Design — crash diagnostics (A3)

`launch()` currently discards `session.Stderr` entirely. Wire it to a capped
buffer, and surface it when `receiveHandshake` times out:

```go
// launch.go
func launch(conn *sshconn.Connection, remoteDir, devServerID string) (*sshExecTransport, *diagnosticStderr, error) {
    session, err := conn.NewSession()
    // ... stdin/stdout pipes unchanged ...
    stderrBuf := newDiagnosticStderr(64 * 1024) // capped ring buffer — a crash-looping process
                                                 // must never grow this unbounded
    session.Stderr = stderrBuf
    // ... session.Start(cmd) unchanged ...
    return newSSHExecTransport(conn, session, stdin, stdout), stderrBuf, nil
}
```

```go
// provisioner.go
transport, stderrBuf, err := launch(conn, remoteDir, devServer.ID)
// ...
info, err := p.receiveHandshake(ctx, transport)
if err != nil {
    _ = transport.Close("handshake failed")
    diag := collectDiagnostics(ctx, conn, stderrBuf) // remote uname -s/-m, node --version,
                                                        // whoami, plus stderrBuf's captured bytes
    return nil, devserveragent.HandshakeInfo{}, fmt.Errorf("%w\n%s", err, diag)
}
```

```go
// diagnostics.go (new)
// diagnosticStderr is a capped, thread-safe io.Writer — bounded so a
// crash-looping or verbose process can't exhaust memory; matches the "no
// unbounded buffer" discipline devserveragent's own doc comments apply to
// notification channels (routeNotification's drop-on-full path).
type diagnosticStderr struct { mu sync.Mutex; buf []byte; cap int }
func newDiagnosticStderr(capBytes int) *diagnosticStderr { ... }
func (d *diagnosticStderr) Write(p []byte) (int, error) { ... } // truncates from the front, keeps the tail
func (d *diagnosticStderr) String() string { ... }

// collectDiagnostics runs a handful of cheap remote probes (uname -s,
// uname -m, node --version, whoami) after a launch failure — the A3
// "collect diagnostics" requirement, scoped to information, not automatic
// remediation. Best-effort: a probe failure is folded into the diagnostic
// text ("uname -s: <error>"), never returned as a second error that could
// mask the original handshake-timeout cause.
func collectDiagnostics(ctx context.Context, conn *sshconn.Connection, stderrBuf *diagnosticStderr) string {
    osOut, _, _ := conn.RunCommand(ctx, "uname -s")
    archOut, _, _ := conn.RunCommand(ctx, "uname -m")
    nodeOut, _, _ := conn.RunCommand(ctx, "node --version")
    whoamiOut, _, _ := conn.RunCommand(ctx, "whoami")
    return fmt.Sprintf("diagnostics: os=%s arch=%s node=%s user=%s stderr_tail=%q",
        strings.TrimSpace(osOut), strings.TrimSpace(archOut), strings.TrimSpace(nodeOut),
        strings.TrimSpace(whoamiOut), stderrBuf.String())
}
```

This closes A3 (crash diagnostics), gives BUG-SSH-02's "no remote OS/arch
detection" finding a real, if lightweight, answer (surfaced on failure, not
used to select a binary — see scope boundary above), and gives BR-SSH-08/09
("non-root enforcement/reporting") a non-blocking diagnostic signal
(`whoami` in the failure text) without turning it into a hard precondition
this solution doesn't have product sign-off to add.

## Test plan

- `sshrelay/version_check_test.go` — fake `SshTargetResolver`/exec transport:
  matching version skips `deploy()` entirely (assert zero SFTP calls on a
  fake `Connection`); mismatched/absent version deploys.
- `sshrelay/deploy_test.go` — `deployWithRetry`: 2 transient failures then a
  success succeeds within budget; a persistent checksum mismatch fails after
  exactly one re-upload (assert `deploy()` called exactly twice for that
  path, not three times); a persistent network failure fails after exactly 3
  attempts.
- `sshrelay/launch_test.go` — `session.Stderr` is wired and capped; writing
  more than the cap keeps only the tail.
- `sshrelay/provisioner_test.go` — a handshake timeout's returned error
  contains the diagnostic probes' output; a probe failure (e.g. `whoami`
  unsupported) degrades gracefully into the diagnostic text rather than
  swallowing the original timeout error.

## References

- `specs/backend-go/bugs/logic-v1/BUG-SSH-02-deploy-relay-partial.md` — problem statement and the 7 "what's missing" findings this solution addresses (all but multi-binary distribution, explicitly scoped out above)
- `specs/backend-go/tdd/services/infra-fleet-service.md:106-110` (§3 `BootstrapFleetTarget`'s streaming-progress rationale, the shape this solution's step decomposition anticipates), `:518-523` (§9 relay-ssh's trust-model list — confirms BR-SSH-06's by-design absence)
- `specs/backend-go/tdd/architecture/03-clean-architecture-guidelines.md` — typed domain errors over raw `fmt.Errorf` strings (`ErrChecksumMismatch` sentinel)
- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/deploy.go:32-81` — current single-attempt deploy this solution wraps
- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/launch.go:1-45` — current `launch()`, `session.Stderr` never wired
- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/provisioner.go:1-21,82-172` — package doc comment's explicit scope-reduction framing this solution respects
- `docs/logic/remote-development/BL-SSH-02-deploy-relay.md`

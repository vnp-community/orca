# TASK-SSH-02-04: Wire version-gate + `deployWithRetry` into `Provisioner.Provision`

**From Solution:** SOL-SSH-02
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/provisioner.go`
**Depends on:** TASK-SSH-02-02, TASK-SSH-02-03
**Status:** `[ ]` TODO

---

## Context

`Provision` (`provisioner.go:82-115`) calls `deploy(ctx, conn, p.cfg)`
unconditionally. This task makes it skip the SFTP upload when the remote
bundle already matches `p.cfg.OrcaVersion`, and use the retrying wrapper
otherwise.

## Changes to make

In `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/provisioner.go`,
replace the `deploy` call inside `Provision`:

```go
func (p *Provisioner) Provision(ctx context.Context, devServer domain.DevServer) (devserveragent.Transport, devserveragent.HandshakeInfo, error) {
	if devServer.SSHTargetID == "" {
		return nil, devserveragent.HandshakeInfo{}, fmt.Errorf("sshrelay: dev server %q has no ssh_target_id", devServer.ID)
	}
	target, err := p.resolver.Get(ctx, devServer.TenantID, devServer.SSHTargetID)
	if err != nil {
		return nil, devserveragent.HandshakeInfo{}, fmt.Errorf("sshrelay: resolving ssh target %q: %w", devServer.SSHTargetID, err)
	}

	conn, err := p.connector.Connect(ctx, target)
	if err != nil {
		return nil, devserveragent.HandshakeInfo{}, fmt.Errorf("sshrelay: dialing ssh target %q: %w", devServer.SSHTargetID, err)
	}

	// BR-SSH-07: skip the SFTP upload entirely when the already-deployed
	// bundle's AGENT_VERSION matches this backend's OrcaVersion. A
	// version-probe failure (verr != nil) or version mismatch falls through
	// to deployWithRetry as before — deploying is always the safe default,
	// skipping it is the optimization.
	if version, present, verr := remoteVersionAndPresence(ctx, conn); verr == nil && present && version == p.cfg.OrcaVersion {
		// already current — no deploy needed
	} else if _, derr := deployWithRetry(ctx, conn, p.cfg); derr != nil {
		_ = conn.Close()
		return nil, devserveragent.HandshakeInfo{}, derr
	}

	transport, err := launch(conn, remoteDir, devServer.ID)
	if err != nil {
		_ = conn.Close()
		return nil, devserveragent.HandshakeInfo{}, err
	}

	info, err := p.receiveHandshake(ctx, transport)
	if err != nil {
		_ = transport.Close("handshake failed")
		return nil, devserveragent.HandshakeInfo{}, err
	}

	return transport, info, nil
}
```

Note: `remoteDir` here refers to the package-level `sshrelay.remoteDir`
constant (`deploy.go:22`) directly, since `deploy`/`deployWithRetry` always
returns that same constant on success — this task does not change that.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/sshrelay/... -run TestProvision -v
```

Expected new test in `provisioner_test.go`: matching version skips `deploy()`
entirely (assert zero SFTP calls on a fake `Connection`/exec transport);
mismatched or absent version calls `deployWithRetry` (still deploys); a
version-probe error also falls through to deploy rather than aborting.

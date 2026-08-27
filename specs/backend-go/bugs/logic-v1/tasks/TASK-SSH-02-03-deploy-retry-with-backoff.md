# TASK-SSH-02-03: `deployWithRetry` — network retries + one checksum-mismatch re-upload (A1, A2)

**From Solution:** SOL-SSH-02
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/deploy.go`
**Depends on:** none
**Status:** `[x] DONE — ErrChecksumMismatch sentinel + deployWithRetry (3 network retries, 1 checksum re-upload) added to deploy.go; TestDeployWithRetry_* pass`

---

## Context

`deploy()` (`deploy.go:32-81`) makes exactly one attempt and returns a plain
`fmt.Errorf` on checksum mismatch — no distinction from a transient network
failure, no retry of either kind. A1 wants up to 3 network retries; A2 wants
exactly one re-upload-and-recheck on a checksum mismatch before giving up
(not folded into the network-retry budget, since a mismatch isn't a
transient blip).

## Changes to make

In `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/deploy.go`,
add a sentinel and wrap the mismatch return:

```go
import (
	"errors"
	// ... existing imports ...
)

// ErrChecksumMismatch is deploy()'s checksum-mismatch sentinel — lets
// deployWithRetry distinguish it from a network/SFTP error via errors.Is
// rather than string-matching, so dispatch survives message wording changes.
var ErrChecksumMismatch = errors.New("sshrelay: relay bundle checksum mismatch")

// ... inside deploy(), replace the current mismatch return ...
if remoteHex != localHex {
	return "", fmt.Errorf("%w after upload (local=%s remote=%s)", ErrChecksumMismatch, localHex, remoteHex)
}
```

Add the retry wrapper (new function in the same file, `deploy()` itself
unchanged beyond the sentinel wrap above — its single-attempt contract stays
intact for any future direct caller, e.g. a `BootstrapFleetTarget` stream
step):

```go
const maxDeployNetworkRetries = 3

// deployWithRetry wraps deploy() with up to maxDeployNetworkRetries attempts
// (A1). A checksum mismatch (A2) triggers exactly one immediate
// re-upload-and-recheck, not folded into the network-retry budget — a
// persistent mismatch after that one retry fails outright rather than
// retrying identically 3x (which would just repeat the same corrupted
// transfer and mask a real corruption/tampering signal).
func deployWithRetry(ctx context.Context, conn *sshconn.Connection, cfg Config) (string, error) {
	var lastErr error
	for attempt := 0; attempt < maxDeployNetworkRetries; attempt++ {
		dir, err := deploy(ctx, conn, cfg)
		if err == nil {
			return dir, nil
		}
		lastErr = err
		if errors.Is(err, ErrChecksumMismatch) {
			if dir, rerr := deploy(ctx, conn, cfg); rerr == nil {
				return dir, nil
			} else if errors.Is(rerr, ErrChecksumMismatch) {
				return "", fmt.Errorf("sshrelay: relay bundle checksum mismatch persisted after re-upload — refusing to launch a possibly-corrupted/tampered bundle: %w", rerr)
			} else {
				lastErr = rerr
			}
			break // don't network-retry after a checksum-specific failure path
		}
		if attempt < maxDeployNetworkRetries-1 {
			time.Sleep(deployBackoffDelay(attempt))
		}
	}
	return "", fmt.Errorf("sshrelay: deploy failed after %d attempts: %w", maxDeployNetworkRetries, lastErr)
}

// deployBackoffDelay: 500ms, 1s, 2s — small on purpose, deploy sits on the
// connect-latency-sensitive path (a caller is waiting for EstablishConnection
// to return).
func deployBackoffDelay(attempt int) time.Duration {
	return 500 * time.Millisecond * time.Duration(1<<uint(attempt))
}
```

Add `"time"` to `deploy.go`'s imports if not already present.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/sshrelay/... -run TestDeployWithRetry -v
```

Expected new test (`deploy_test.go`): 2 transient failures then a success
succeeds within budget; a persistent checksum mismatch fails after exactly
one re-upload (`deploy()` called exactly twice for that path, not three
times); a persistent network failure fails after exactly 3 attempts.

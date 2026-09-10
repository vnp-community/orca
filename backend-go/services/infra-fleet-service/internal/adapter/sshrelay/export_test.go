package sshrelay

// export_test.go re-exports this package's unexported test surface for the
// external sshrelay_test package (provisioner_test.go et al.), which already
// owns the fakeSSHServer/fakeCA/fakeIssuer test doubles these need — standard
// Go "export_test.go" idiom, avoids duplicating that fake-server plumbing in
// a second, internal test package.

import (
	"context"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/sshconn"
)

var RemoteVersionAndPresenceForTest = remoteVersionAndPresence

var DeployForTest = deploy

var DeployWithRetryForTest = deployWithRetry

var CollectDiagnosticsForTest = collectDiagnostics

// DiagnosticStderrForTest is the exported alias of diagnosticStderr —
// exposed as an interface-shaped pair of constructor+methods since the type
// itself is unexported.
type DiagnosticStderrForTest = diagnosticStderr

var NewDiagnosticStderrForTest = newDiagnosticStderr

// LaunchForTest wraps launch, whose *diagnosticStderr return is unexported —
// callers outside this package get it back as the DiagnosticStderrForTest
// alias instead.
func LaunchForTest(ctx context.Context, conn *sshconn.Connection, remoteDirArg, devServerID string) (*sshExecTransport, string, *DiagnosticStderrForTest, error) {
	return launch(ctx, conn, remoteDirArg, devServerID)
}

// ReattachForTest wraps reattach the same way LaunchForTest wraps launch.
func ReattachForTest(ctx context.Context, conn *sshconn.Connection, remoteDirArg, sockPath string) (*sshExecTransport, string, *DiagnosticStderrForTest, error) {
	return reattach(ctx, conn, remoteDirArg, sockPath)
}

// RemoteDirForTest / RemoteAgentFileForTest expose the package-level
// deploy-path constants so tests can build the same remote paths this
// package's own commands do, without hardcoding a second copy.
const (
	RemoteDirForTest       = remoteDir
	RemoteAgentFileForTest = remoteAgentFile
)

// RelaySockPathForTest exposes relaySockPath for tests building the same
// socket path launch()/reattach() compute internally.
var RelaySockPathForTest = relaySockPath

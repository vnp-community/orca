package sshrelay

// export_test.go re-exports this package's unexported test surface for the
// external sshrelay_test package (provisioner_test.go et al.), which already
// owns the fakeSSHServer/fakeCA/fakeIssuer test doubles these need — standard
// Go "export_test.go" idiom, avoids duplicating that fake-server plumbing in
// a second, internal test package.

import (
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

// LaunchForTest wraps launch, whose second return value's type
// (*diagnosticStderr) is unexported — callers outside this package get it
// back as the DiagnosticStderrForTest alias instead.
func LaunchForTest(conn *sshconn.Connection, remoteDirArg, devServerID string) (*sshExecTransport, *DiagnosticStderrForTest, error) {
	return launch(conn, remoteDirArg, devServerID)
}

// RemoteDirForTest / RemoteAgentFileForTest expose the package-level
// deploy-path constants so tests can build the same remote paths this
// package's own commands do, without hardcoding a second copy.
const (
	RemoteDirForTest       = remoteDir
	RemoteAgentFileForTest = remoteAgentFile
)

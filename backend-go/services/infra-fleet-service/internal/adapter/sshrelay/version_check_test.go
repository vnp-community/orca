package sshrelay_test

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/sshconn"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/sshrelay"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// dialFakeServer is shared connect-plumbing for tests that talk to
// remoteVersionAndPresence/deploy/launch directly rather than through
// Provisioner.Provision — same fakeIssuer/sshconn.Connector dance
// provisioner_test.go's TestProvision_* tests use internally.
func dialFakeServer(t *testing.T, ca *fakeCA, server *fakeSSHServer, targetID string) *sshconn.Connection {
	t.Helper()
	target, err := domain.NewSshTarget(targetID, "tenant-1", "127.0.0.1", server.port(t), "deploy", "role-1", "", "", "", nil)
	if err != nil {
		t.Fatalf("NewSshTarget: %v", err)
	}
	connector := sshconn.NewConnector(&fakeIssuer{ca: ca, principal: "deploy"}, nil, sshconn.Config{DialTimeout: 5 * time.Second}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := connector.Connect(ctx, target)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	return conn
}

func TestRemoteVersionAndPresence_NoPriorDeployment(t *testing.T) {
	ca := newFakeCA(t)
	server := startFakeSSHServer(t, ca.signer.PublicKey(), "deploy", false)
	conn := dialFakeServer(t, ca, server, "target-vc-1")
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	version, present, err := sshrelay.RemoteVersionAndPresenceForTest(ctx, conn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if present {
		t.Errorf("expected present=false when no prior deployment exists, got version=%q", version)
	}
	if version != "" {
		t.Errorf("expected empty version, got %q", version)
	}
}

func TestRemoteVersionAndPresence_PresentAfterDeploy(t *testing.T) {
	ca := newFakeCA(t)
	server := startFakeSSHServer(t, ca.signer.PublicKey(), "deploy", false)
	bundlePath := writeLocalBundle(t, "// fake agent bundle content\n")
	conn := dialFakeServer(t, ca, server, "target-vc-2")
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := sshrelay.DeployForTest(ctx, conn, sshrelay.Config{BundlePath: bundlePath}); err != nil {
		t.Fatalf("deploy: %v", err)
	}

	// The fake server's exec handler doesn't run a real `node`, so it can't
	// print a real AGENT_VERSION — it falls into handleExec's default branch
	// (exit 0, no output), which remoteVersionAndPresence correctly treats
	// as "present=false" (empty/unknown output), same as a probe against a
	// target with no working `node` on PATH. This still proves the "file
	// exists" half of the command runs against the just-uploaded file
	// without erroring.
	_, present, err := sshrelay.RemoteVersionAndPresenceForTest(ctx, conn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if present {
		t.Error("expected present=false against the fake server's non-node exec handler")
	}
}

func TestRemoteVersionAndPresence_ProbeErrorPropagates(t *testing.T) {
	ca := newFakeCA(t)
	server := startFakeSSHServer(t, ca.signer.PublicKey(), "deploy", false)
	conn := dialFakeServer(t, ca, server, "target-vc-3")
	_ = conn.Close() // force RunCommand to fail: session open against a closed connection

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, present, err := sshrelay.RemoteVersionAndPresenceForTest(ctx, conn)
	if err == nil {
		t.Fatal("expected an error when the probe command can't run")
	}
	if present {
		t.Error("expected present=false on a probe error")
	}
}

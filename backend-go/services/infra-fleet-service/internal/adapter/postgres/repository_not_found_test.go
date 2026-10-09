//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// TASK-E2E-017: một id không tồn tại, hoặc không đúng dạng uuid (SQLSTATE
// 22P02), phải ra sentinel miền (errors.Is), không phải lỗi hạ tầng chung.
func TestRepositories_NotFoundReturnsDomainSentinel(t *testing.T) {
	repo, sshStore := setupSshTargetStore(t)
	pool := repo.pool
	ctx := context.Background()
	ids := []string{testUnknownID, "abc"}

	for _, id := range ids {
		if _, err := repo.Get(ctx, testTenant1, id); !errors.Is(err, domain.ErrDevServerNotFound) {
			t.Errorf("DevServer Get(%q): got %v", id, err)
		}
		if _, err := repo.UpdateApprovalStatus(ctx, testTenant1, id, domain.DevServerStatusApproved); !errors.Is(err, domain.ErrDevServerNotFound) {
			t.Errorf("UpdateApprovalStatus(%q): got %v", id, err)
		}
		if _, err := repo.AssignGroup(ctx, testTenant1, id, ""); !errors.Is(err, domain.ErrDevServerNotFound) {
			t.Errorf("AssignGroup(%q): got %v", id, err)
		}
		if _, err := sshStore.Get(ctx, testTenant1, id); !errors.Is(err, domain.ErrSshTargetNotFound) {
			t.Errorf("SshTarget Get(%q): got %v", id, err)
		}
		if _, err := NewDevServerAccessRequestStore(pool).Get(ctx, testTenant1, id); !errors.Is(err, domain.ErrDevServerAccessRequestNotFound) {
			t.Errorf("AccessRequest Get(%q): got %v", id, err)
		}
		if _, err := NewDevServerAccessRequestStore(pool).UpdateStatus(ctx, testTenant1, id, domain.AccessRequestStatusApproved); !errors.Is(err, domain.ErrDevServerAccessRequestNotFound) {
			t.Errorf("AccessRequest UpdateStatus(%q): got %v", id, err)
		}
		if err := NewDevServerGroupGrantStore(pool).Delete(ctx, testTenant1, id); !errors.Is(err, domain.ErrDevServerGroupGrantNotFound) {
			t.Errorf("Grant Delete(%q): got %v", id, err)
		}
		if err := NewBrowserProfileStore(pool).Delete(ctx, testTenant1, id); !errors.Is(err, domain.ErrBrowserProfileNotFound) {
			t.Errorf("BrowserProfile Delete(%q): got %v", id, err)
		}
		if _, err := NewAgentTokenStore(pool).Revoke(ctx, testTenant1, id); !errors.Is(err, domain.ErrAgentTokenNotFound) {
			t.Errorf("AgentToken Revoke(%q): got %v", id, err)
		}
		if _, err := NewFleetDefinitionStore(pool).Get(ctx, testTenant1, id); !errors.Is(err, domain.ErrFleetDefinitionNotFound) {
			t.Errorf("FleetDefinition Get(%q): got %v", id, err)
		}
		runtimes := NewEphemeralVmRuntimeStore(pool)
		if _, err := runtimes.Get(ctx, testTenant1, id); !errors.Is(err, domain.ErrEphemeralVmRuntimeNotFound) {
			t.Errorf("EphemeralVmRuntime Get(%q): got %v", id, err)
		}
		if _, err := runtimes.UpdateStatus(ctx, testTenant1, id, "active", "", ""); !errors.Is(err, domain.ErrEphemeralVmRuntimeNotFound) {
			t.Errorf("EphemeralVmRuntime UpdateStatus(%q): got %v", id, err)
		}
		// found-bool lookups: unknown/malformed id is found=false, never an error.
		if found, _, err := NewAgentSessionStore(pool).Get(ctx, testTenant1, id); err != nil || found {
			t.Errorf("AgentSession Get(%q): found=%v err=%v", id, found, err)
		}
		if found, _, err := NewAgentSessionStore(pool).LatestForWorktree(ctx, testTenant1, id); err != nil || found {
			t.Errorf("AgentSession LatestForWorktree(%q): found=%v err=%v", id, found, err)
		}
		if found, _, err := NewTerminalScrollbackSnapshotStore(pool).Get(ctx, testTenant1, id, "pane"); err != nil || found {
			t.Errorf("ScrollbackSnapshot Get(%q): found=%v err=%v", id, found, err)
		}
		if connected, _, _, err := repo.ResolveConnection(ctx, testTenant1, id); err != nil || connected {
			t.Errorf("ResolveConnection(%q): connected=%v err=%v", id, connected, err)
		}
		if got, err := NewTerminalSessionStore(pool).List(ctx, testTenant1, id); err != nil || len(got) != 0 {
			t.Errorf("TerminalSession List(%q): %v err=%v", id, got, err)
		}
	}

	terminals := NewTerminalSessionStore(pool)
	now := time.Now().UTC()
	if err := terminals.Touch(ctx, testTenant1, "no-such-pty", now); !errors.Is(err, domain.ErrTerminalSessionNotFound) {
		t.Errorf("Touch: got %v", err)
	}
	if err := terminals.Close(ctx, testTenant1, "no-such-pty", now); !errors.Is(err, domain.ErrTerminalSessionNotFound) {
		t.Errorf("Close: got %v", err)
	}
	ev := domain.OutboxEvent{ID: testUnknownID, TenantID: testTenant1, Subject: domain.SubjectTerminalClosed, OccurredAt: now, PayloadJSON: []byte(`{}`)}
	if _, err := terminals.CloseWithEvent(ctx, testTenant1, "no-such-pty", now, ev); !errors.Is(err, domain.ErrTerminalSessionNotFound) {
		t.Errorf("CloseWithEvent: got %v", err)
	}
	conn := domain.Connection{ID: testUnknownID, TenantID: testTenant1, Status: domain.ConnectionStatusClosed}
	if err := repo.UpdateStatus(ctx, testTenant1, conn); !errors.Is(err, domain.ErrConnectionNotFound) {
		t.Errorf("Connection UpdateStatus: got %v", err)
	}
}

//go:build integration

package mysql

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// TASK-E2E-017: một id không tồn tại phải ra sentinel miền (errors.Is). Cột id
// MySQL là CHAR nên id sai định dạng cũng chỉ là "không khớp hàng".
func TestRepositories_NotFoundReturnsDomainSentinel(t *testing.T) {
	db := setupDB(t)
	repo := New(db)
	sshStore := NewSshTargetStore(db)
	ctx := context.Background()
	const missing = "99999999-9999-9999-9999-999999999999"

	for _, id := range []string{missing, "abc"} {
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
		if _, err := NewDevServerAccessRequestStore(db).Get(ctx, testTenant1, id); !errors.Is(err, domain.ErrDevServerAccessRequestNotFound) {
			t.Errorf("AccessRequest Get(%q): got %v", id, err)
		}
		if _, err := NewDevServerAccessRequestStore(db).UpdateStatus(ctx, testTenant1, id, domain.AccessRequestStatusApproved); !errors.Is(err, domain.ErrDevServerAccessRequestNotFound) {
			t.Errorf("AccessRequest UpdateStatus(%q): got %v", id, err)
		}
		if err := NewDevServerGroupGrantStore(db).Delete(ctx, testTenant1, id); !errors.Is(err, domain.ErrDevServerGroupGrantNotFound) {
			t.Errorf("Grant Delete(%q): got %v", id, err)
		}
		if err := NewBrowserProfileStore(db).Delete(ctx, testTenant1, id); !errors.Is(err, domain.ErrBrowserProfileNotFound) {
			t.Errorf("BrowserProfile Delete(%q): got %v", id, err)
		}
		if _, err := NewAgentTokenStore(db).Revoke(ctx, testTenant1, id); !errors.Is(err, domain.ErrAgentTokenNotFound) {
			t.Errorf("AgentToken Revoke(%q): got %v", id, err)
		}
		if _, err := NewFleetDefinitionStore(db).Get(ctx, testTenant1, id); !errors.Is(err, domain.ErrFleetDefinitionNotFound) {
			t.Errorf("FleetDefinition Get(%q): got %v", id, err)
		}
		runtimes := NewEphemeralVmRuntimeStore(db)
		if _, err := runtimes.Get(ctx, testTenant1, id); !errors.Is(err, domain.ErrEphemeralVmRuntimeNotFound) {
			t.Errorf("EphemeralVmRuntime Get(%q): got %v", id, err)
		}
		if _, err := runtimes.UpdateStatus(ctx, testTenant1, id, "active", "", ""); !errors.Is(err, domain.ErrEphemeralVmRuntimeNotFound) {
			t.Errorf("EphemeralVmRuntime UpdateStatus(%q): got %v", id, err)
		}
	}

	terminals := NewTerminalSessionStore(db)
	now := time.Now().UTC()
	if err := terminals.Touch(ctx, testTenant1, "no-such-pty", now); !errors.Is(err, domain.ErrTerminalSessionNotFound) {
		t.Errorf("Touch: got %v", err)
	}
	if err := terminals.Close(ctx, testTenant1, "no-such-pty", now); !errors.Is(err, domain.ErrTerminalSessionNotFound) {
		t.Errorf("Close: got %v", err)
	}
	ev := domain.OutboxEvent{ID: missing, TenantID: testTenant1, Subject: domain.SubjectTerminalClosed, OccurredAt: now, PayloadJSON: []byte(`{}`)}
	if _, err := terminals.CloseWithEvent(ctx, testTenant1, "no-such-pty", now, ev); !errors.Is(err, domain.ErrTerminalSessionNotFound) {
		t.Errorf("CloseWithEvent: got %v", err)
	}
	conn := domain.Connection{ID: missing, TenantID: testTenant1, Status: domain.ConnectionStatusClosed}
	if err := repo.UpdateStatus(ctx, testTenant1, conn); !errors.Is(err, domain.ErrConnectionNotFound) {
		t.Errorf("Connection UpdateStatus: got %v", err)
	}
}

//go:build integration

package mysql

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"
)

func agentEntry(t *testing.T, tenantID string, at time.Time, decision, tool string) domain.AuditEntry {
	t.Helper()
	e, err := domain.NewAuditEntry(uuid.NewString(), tenantID, uuid.NewString(), "mcp.tool_call", "", "mcp_tool", tool,
		map[string]any{"decision": decision, "client_id": "c1", "args_summary": "x"}, domain.OutcomeAllowed, "", at)
	if err != nil {
		t.Fatal(err)
	}
	return e.WithActorType(domain.ActorAgent)
}

func TestAuditActorTypeMigration_UpDownUp(t *testing.T) {
	dsn := testutil.StartMySQL(t, "auth")
	path, _ := filepath.Abs("../../../migrations/mysql")
	run := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("migrate", append([]string{"-path", path, "-database", dsn}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("migrate %v: %v\n%s", args, err, out)
		}
	}
	run("up")
	run("down", "-all")
	run("up")
}

func TestAudit_AgentRowsKeysetFiltersAndIdempotency_MySQL(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID, other := uuid.NewString(), uuid.NewString()
	base := time.Now().UTC().Truncate(time.Microsecond).Add(-time.Hour)

	legacy, _ := domain.NewAuditEntry(uuid.NewString(), tenantID, uuid.NewString(), "user.login", "", "user", "u", nil, domain.OutcomeAllowed, "", base)
	if err := repo.Append(ctx, legacy); err != nil {
		t.Fatal(err)
	}
	// 7 agent rows; three share one timestamp to exercise the id tie-break.
	var want []domain.AuditEntry
	for i := 0; i < 7; i++ {
		at := base.Add(time.Duration(i/3) * time.Minute)
		decision := "allow"
		if i%2 == 1 {
			decision = "deny"
		}
		e := agentEntry(t, tenantID, at, decision, fmt.Sprintf("tool_%d", i%2))
		if err := repo.Append(ctx, e); err != nil {
			t.Fatal(err)
		}
		want = append(want, e)
	}
	if err := repo.Append(ctx, agentEntry(t, other, base, "allow", "tool_0")); err != nil {
		t.Fatal(err)
	}

	// Redelivery: same id twice is one row and no error.
	dup := agentEntry(t, tenantID, base.Add(time.Hour), "allow", "dup")
	for i := 0; i < 3; i++ {
		if err := repo.AppendIdempotent(ctx, dup); err != nil {
			t.Fatal(err)
		}
	}

	f := usecase.AuditQueryFilter{TenantID: tenantID, ActorType: domain.ActorAgent, NewestFirst: true}
	var got []domain.AuditEntry
	token := ""
	for page := 0; page < 10; page++ {
		rows, next, err := repo.Query(ctx, f, token, 3)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, rows...)
		if next == "" {
			break
		}
		token = next
	}
	if len(got) != 8 { // 7 + dup
		t.Fatalf("keyset paging returned %d rows, want 8 (no skips, no repeats)", len(got))
	}
	seen := map[string]bool{}
	for i, e := range got {
		if seen[e.ID] {
			t.Fatalf("duplicate %s across pages", e.ID)
		}
		seen[e.ID] = true
		if e.ActorType != domain.ActorAgent || e.TenantID != tenantID {
			t.Fatalf("filter leak: %+v", e)
		}
		if i > 0 && e.OccurredAt.After(got[i-1].OccurredAt) {
			t.Fatalf("not newest-first at %d", i)
		}
	}
	if got[0].ID != dup.ID {
		t.Fatal("newest row first")
	}

	byDecision, _, err := repo.Query(ctx, usecase.AuditQueryFilter{TenantID: tenantID, ActorType: domain.ActorAgent, NewestFirst: true,
		MetadataEquals: map[string]string{"decision": "deny"}}, "", 50)
	if err != nil || len(byDecision) != 3 {
		t.Fatalf("decision filter: %d %v", len(byDecision), err)
	}
	byTool, _, _ := repo.Query(ctx, usecase.AuditQueryFilter{TenantID: tenantID, TargetID: "tool_1", NewestFirst: true}, "", 50)
	if len(byTool) != 3 {
		t.Fatalf("target filter: %d", len(byTool))
	}
	byClient, _, _ := repo.Query(ctx, usecase.AuditQueryFilter{TenantID: tenantID, MetadataEquals: map[string]string{"client_id": "nope"}}, "", 50)
	if len(byClient) != 0 {
		t.Fatal("client filter")
	}
	if _, _, err := repo.Query(ctx, usecase.AuditQueryFilter{TenantID: tenantID, MetadataEquals: map[string]string{"args_summary": "x"}}, "", 5); err == nil {
		t.Fatal("non-allow-listed key must be refused by the adapter too")
	}

	// Legacy behavior is untouched: id ascending, every actor type, user default.
	all, _, err := repo.Query(ctx, usecase.AuditQueryFilter{TenantID: tenantID}, "", 100)
	if err != nil || len(all) != 9 {
		t.Fatalf("legacy query: %d %v", len(all), err)
	}
	for i := 1; i < len(all); i++ {
		if all[i].ID <= all[i-1].ID {
			t.Fatal("legacy order must stay by id")
		}
	}
	for _, e := range all {
		if e.ID == legacy.ID && e.ActorType != domain.ActorUser {
			t.Fatalf("legacy rows read as user, got %q", e.ActorType)
		}
	}
	users, _, _ := repo.Query(ctx, usecase.AuditQueryFilter{TenantID: tenantID, ActorType: domain.ActorUser}, "", 50)
	if len(users) != 1 {
		t.Fatalf("actor_type=user filter: %d", len(users))
	}
}

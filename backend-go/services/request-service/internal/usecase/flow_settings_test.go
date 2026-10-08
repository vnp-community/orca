package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type memFlowRepo struct {
	rows    map[string]bool
	readErr error
	writes  int
}

func (m *memFlowRepo) Get(ctx context.Context) (domain.FlowSettings, bool, error) {
	if m.readErr != nil {
		return domain.FlowSettings{}, false, m.readErr
	}
	id, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.FlowSettings{}, false, err
	}
	v, ok := m.rows[id]
	return domain.FlowSettings{TenantID: id, Enabled: v}, ok, nil
}

func (m *memFlowRepo) Upsert(ctx context.Context, enabled bool, _ string) error {
	id, _ := tenant.TenantID(ctx)
	m.rows[id] = enabled
	m.writes++
	return nil
}

func flowCtx(tenantID, role string) context.Context {
	ctx := tenant.WithUserID(tenant.WithTenantID(context.Background(), tenantID), "u-1")
	if role != "" {
		ctx = tenant.WithRole(ctx, role)
	}
	return ctx
}

func TestEffectiveFlow_Table(t *testing.T) {
	cases := []struct {
		name    string
		global  bool
		rows    map[string]bool
		readErr error
		want    bool
		wantErr bool
	}{
		{"global off, tenant on", false, map[string]bool{"t1": true}, nil, false, false},
		{"global on, no row", true, map[string]bool{}, nil, false, false},
		{"global on, tenant off", true, map[string]bool{"t1": false}, nil, false, false},
		{"global on, tenant on", true, map[string]bool{"t1": true}, nil, true, false},
		{"global on, read error fails closed", true, map[string]bool{"t1": true}, errors.New("db down"), false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := NewFlowSettings(&memFlowRepo{rows: tc.rows, readErr: tc.readErr}, tc.global, nil, nil)
			got, err := u.Effective(flowCtx("t1", ""))
			if got != tc.want || (err != nil) != tc.wantErr {
				t.Fatalf("got (%v, %v), want (%v, err=%v)", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestEffectiveFlow_GlobalOffSkipsTheDatabase(t *testing.T) {
	repo := &memFlowRepo{rows: map[string]bool{}, readErr: errors.New("must not be read")}
	if on, err := NewFlowSettings(repo, false, nil, nil).Effective(flowCtx("t1", "")); on || err != nil {
		t.Fatalf("got (%v, %v)", on, err)
	}
}

func TestSetFlow_AdminOnlyAndAudited(t *testing.T) {
	repo := &memFlowRepo{rows: map[string]bool{}}
	rec := &captureAudit{}
	u := NewFlowSettings(repo, true, rec, nil)

	_, err := u.Set(flowCtx("t1", "user"), true)
	if code := errCode(err); code != "REQUEST_FLOW_ADMIN_ONLY" {
		t.Fatalf("non-admin: %v", err)
	}
	if repo.writes != 0 {
		t.Fatal("a non-admin write reached the repository")
	}
	if len(rec.events) != 1 || rec.events[0].Outcome != domain.AuditOutcomeDenied || rec.events[0].Action != domain.ActionRequestFlowSet {
		t.Fatalf("denied attempt not audited: %+v", rec.events)
	}

	on, err := u.Set(flowCtx("t1", "admin"), true)
	if err != nil || !on {
		t.Fatalf("admin set: (%v, %v)", on, err)
	}
	if len(rec.events) != 2 || rec.events[1].Outcome != domain.AuditOutcomeAllowed || rec.events[1].TargetType != "tenant" ||
		rec.events[1].TargetID != "t1" || rec.events[1].Metadata["enabled"] != "true" {
		t.Fatalf("allowed change not audited: %+v", rec.events)
	}
}

func TestSetFlow_ReportsEffectiveValueNotTheStoredOne(t *testing.T) {
	u := NewFlowSettings(&memFlowRepo{rows: map[string]bool{}}, false, nil, nil)
	on, err := u.Set(flowCtx("t1", "admin"), true)
	if err != nil || on {
		t.Fatalf("global off: want enabled=false, got (%v, %v)", on, err)
	}
}

func TestFlow_TenantsAreIndependent(t *testing.T) {
	u := NewFlowSettings(&memFlowRepo{rows: map[string]bool{}}, true, nil, nil)
	if _, err := u.Set(flowCtx("tenant-a", "admin"), true); err != nil {
		t.Fatal(err)
	}
	a, _ := u.Effective(flowCtx("tenant-a", ""))
	b, _ := u.Effective(flowCtx("tenant-b", ""))
	if !a || b {
		t.Fatalf("tenant A=%v tenant B=%v, want true/false", a, b)
	}
}

func TestGetFlow_RequiresTenant(t *testing.T) {
	u := NewFlowSettings(&memFlowRepo{rows: map[string]bool{}}, true, nil, nil)
	if _, err := u.Get(context.Background()); err == nil {
		t.Fatal("want tenant-required error")
	}
}

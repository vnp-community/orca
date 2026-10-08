package grpc

import (
	"context"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type memFlowRows struct{ rows map[string]bool }

func (m *memFlowRows) Get(ctx context.Context) (domain.FlowSettings, bool, error) {
	id, _ := tenant.TenantID(ctx)
	v, ok := m.rows[id]
	return domain.FlowSettings{TenantID: id, Enabled: v}, ok, nil
}

func (m *memFlowRows) Upsert(ctx context.Context, enabled bool, _ string) error {
	id, _ := tenant.TenantID(ctx)
	m.rows[id] = enabled
	return nil
}

func flowServer(global bool) *Server {
	return (&Server{}).WithFlowSettings(usecase.NewFlowSettings(&memFlowRows{rows: map[string]bool{}}, global, nil, nil))
}

func roleCtx(tenantID, role string) context.Context {
	return tenant.WithRole(tenant.WithUserID(tenant.WithTenantID(context.Background(), tenantID), "u-1"), role)
}

func TestFlowSettingsRPC_DefaultOffThenAdminTurnsItOn(t *testing.T) {
	s := flowServer(true)
	ctx := roleCtx("t1", "admin")
	got, err := s.GetRequestFlowSettings(ctx, &requestv1.GetRequestFlowSettingsRequest{})
	if err != nil || got.GetEnabled() {
		t.Fatalf("default: (%v, %v), want enabled=false", got, err)
	}
	set, err := s.SetRequestFlowSettings(ctx, &requestv1.SetRequestFlowSettingsRequest{Enabled: true})
	if err != nil || !set.GetEnabled() {
		t.Fatalf("set: (%v, %v)", set, err)
	}
	if got, _ := s.GetRequestFlowSettings(ctx, &requestv1.GetRequestFlowSettingsRequest{}); !got.GetEnabled() {
		t.Fatal("not enabled after set")
	}
	if other, _ := s.GetRequestFlowSettings(roleCtx("t2", "admin"), &requestv1.GetRequestFlowSettingsRequest{}); other.GetEnabled() {
		t.Fatal("tenant t2 must not see t1's setting")
	}
}

func TestFlowSettingsRPC_GlobalOffWinsOverTenantOn(t *testing.T) {
	s := flowServer(false)
	set, err := s.SetRequestFlowSettings(roleCtx("t1", "admin"), &requestv1.SetRequestFlowSettingsRequest{Enabled: true})
	if err != nil || set.GetEnabled() {
		t.Fatalf("(%v, %v): REQUEST_FLOW_ENABLED=false must keep the effective value off", set, err)
	}
	if got, _ := s.GetRequestFlowSettings(roleCtx("t1", "user"), &requestv1.GetRequestFlowSettingsRequest{}); got.GetEnabled() {
		t.Fatal("Get must report the effective value")
	}
}

func TestFlowSettingsRPC_NonAdminCannotSet(t *testing.T) {
	_, err := flowServer(true).SetRequestFlowSettings(roleCtx("t1", "user"), &requestv1.SetRequestFlowSettingsRequest{Enabled: true})
	if status.Code(err) != codes.PermissionDenied || !strings.Contains(err.Error(), "REQUEST_FLOW_ADMIN_ONLY") {
		t.Fatalf("got %v", err)
	}
}

func TestFlowSettingsRPC_UnimplementedUntilWired(t *testing.T) {
	_, err := (&Server{}).GetRequestFlowSettings(context.Background(), &requestv1.GetRequestFlowSettingsRequest{})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("got %v", err)
	}
}

package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

type fakeOriginLister struct {
	gotTenant string
	gotFilter AgentSessionOriginFilter
	out       []domain.AgentSession
	err       error
}

func (f *fakeOriginLister) ListByOrigin(_ context.Context, tenantID string, flt AgentSessionOriginFilter) ([]domain.AgentSession, error) {
	f.gotTenant, f.gotFilter = tenantID, flt
	return f.out, f.err
}

func TestListAgentSessionsByOrigin_RequiresTenantAndOriginFilter(t *testing.T) {
	f := &fakeOriginLister{}
	uc := NewListAgentSessionsByOrigin(f)
	if _, err := uc.Execute(context.Background(), AgentSessionOriginFilter{OriginType: "mcp"}); err == nil {
		t.Fatal("no tenant must fail")
	}
	ctx := tenant.WithTenantID(context.Background(), "t1")
	_, err := uc.Execute(ctx, AgentSessionOriginFilter{})
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindInvalidArgument {
		t.Fatalf("a listing without an origin filter must be rejected, got %v", err)
	}
	if f.gotTenant != "" {
		t.Fatal("the store must not be queried for a rejected filter")
	}
}

func TestListAgentSessionsByOrigin_BoundsLimitAndScopesToTenant(t *testing.T) {
	f := &fakeOriginLister{out: []domain.AgentSession{{ID: "a1"}}}
	uc := NewListAgentSessionsByOrigin(f)
	ctx := tenant.WithTenantID(context.Background(), "t1")
	for in, want := range map[int]int{0: 100, -5: 100, 50: 50, 100000: 500} {
		got, err := uc.Execute(ctx, AgentSessionOriginFilter{OriginSessionID: "s", Limit: in})
		if err != nil || len(got) != 1 {
			t.Fatalf("limit %d: %v %v", in, got, err)
		}
		if f.gotFilter.Limit != want || f.gotTenant != "t1" {
			t.Fatalf("limit %d -> %d (tenant %q), want %d", in, f.gotFilter.Limit, f.gotTenant, want)
		}
	}
	f.err = errors.New("db down")
	if _, err := uc.Execute(ctx, AgentSessionOriginFilter{OriginSessionID: "s"}); err == nil {
		t.Fatal("store errors must surface")
	}
}

package tenant

import (
	"context"
	"testing"
)

func TestRole_AbsentOnBareContext(t *testing.T) {
	role, ok := Role(context.Background())
	if ok {
		t.Fatalf("expected ok=false on a bare context, got ok=true role=%q", role)
	}
	if role != "" {
		t.Fatalf("expected empty role, got %q", role)
	}
}

func TestRole_RoundTripsThroughWithRole(t *testing.T) {
	ctx := WithRole(context.Background(), "admin")
	role, ok := Role(ctx)
	if !ok {
		t.Fatal("expected ok=true after WithRole")
	}
	if role != "admin" {
		t.Fatalf("expected role %q, got %q", "admin", role)
	}
}

func TestRole_EmptyStringTreatedAsAbsent(t *testing.T) {
	ctx := WithRole(context.Background(), "")
	role, ok := Role(ctx)
	if ok {
		t.Fatalf("expected ok=false for an empty role, got ok=true role=%q", role)
	}
	if role != "" {
		t.Fatalf("expected empty role, got %q", role)
	}
}

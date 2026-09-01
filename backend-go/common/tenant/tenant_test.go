package tenant

import (
	"context"
	"testing"
)

func TestRole_ReturnsFalseWhenAbsent(t *testing.T) {
	if v, ok := Role(context.Background()); ok || v != "" {
		t.Errorf("want (\"\", false), got (%q, %v)", v, ok)
	}
}

func TestRole_ReturnsFalseForEmptyString(t *testing.T) {
	// Why: WithRole("") must behave the same as never calling WithRole at
	// all — an empty role is "unknown," never a distinct falsy-but-present
	// state a caller could mistakenly treat as "definitely not admin, but
	// otherwise trusted."
	ctx := WithRole(context.Background(), "")
	if v, ok := Role(ctx); ok || v != "" {
		t.Errorf("want (\"\", false), got (%q, %v)", v, ok)
	}
}

func TestRole_RoundTrips(t *testing.T) {
	ctx := WithRole(context.Background(), "admin")
	v, ok := Role(ctx)
	if !ok || v != "admin" {
		t.Errorf("want (\"admin\", true), got (%q, %v)", v, ok)
	}
}

func TestRole_DoesNotLeakBetweenIndependentContexts(t *testing.T) {
	ctx1 := WithRole(context.Background(), "admin")
	ctx2 := WithTenantID(context.Background(), "t1") // no WithRole call

	if v, ok := Role(ctx2); ok || v != "" {
		t.Errorf("expected ctx2 (never given a role) to report absent, got (%q, %v)", v, ok)
	}
	if v, ok := Role(ctx1); !ok || v != "admin" {
		t.Errorf("expected ctx1 to still carry its own role, got (%q, %v)", v, ok)
	}
}

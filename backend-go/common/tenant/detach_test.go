package tenant

import (
	"context"
	"testing"
)

func TestDetachKeepsIdentityAndDropsCancellation(t *testing.T) {
	ctx := WithActorType(WithRole(WithUserID(WithTenantID(context.Background(), "t1"), "u1"), "admin"), ActorAgent)
	ctx, cancel := context.WithCancel(ctx)

	detached := Detach(ctx)
	cancel()

	if err := detached.Err(); err != nil {
		t.Fatalf("detached context was cancelled: %v", err)
	}
	if got, _ := TenantID(detached); got != "t1" {
		t.Errorf("tenant = %q", got)
	}
	if got, _ := UserID(detached); got != "u1" {
		t.Errorf("user = %q", got)
	}
	if got, _ := Role(detached); got != "admin" {
		t.Errorf("role = %q", got)
	}
	if got := ActorType(detached); got != ActorAgent {
		t.Errorf("actor = %q", got)
	}
	if ctx.Err() == nil {
		t.Fatal("the original context should be cancelled")
	}
}

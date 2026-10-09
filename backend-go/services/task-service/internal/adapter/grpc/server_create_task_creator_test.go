package grpc

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
)

func TestToCreateTaskInput_CreatorIsTheAuthenticatedCaller(t *testing.T) {
	req := &taskv1.CreateTaskRequest{Title: "t", CreatorId: "forged-user"}

	authed := tenant.WithUserID(tenant.WithTenantID(context.Background(), "t1"), "real-user")
	if got := toCreateTaskInput(authed, req).CreatorID; got != "real-user" {
		t.Errorf("an authenticated caller must be the creator, got %q", got)
	}

	// No caller (system-initiated creation): the explicit creator is the only information there is.
	system := tenant.WithTenantID(context.Background(), "t1")
	if got := toCreateTaskInput(system, req).CreatorID; got != "forged-user" {
		t.Errorf("without a caller the request's creator_id is kept, got %q", got)
	}
}

package grpcclient

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/grpcmw"
	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// accessServer answers ResolvePermission and Grant the way task-service does and keeps what it was asked.
type accessServer struct {
	taskv1.UnimplementedTaskServiceServer
	resolveErr, grantErr error
	resolved             []*taskv1.ResolvePermissionRequest
	granted              []*taskv1.GrantRequest
	grantedBy            []string
}

func (s *accessServer) ResolvePermission(_ context.Context, req *taskv1.ResolvePermissionRequest) (*taskv1.ResolvePermissionResponse, error) {
	s.resolved = append(s.resolved, req)
	return &taskv1.ResolvePermissionResponse{}, s.resolveErr
}

func (s *accessServer) Grant(ctx context.Context, req *taskv1.GrantRequest) (*taskv1.GrantResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	by := ""
	if v := md.Get(grpcmw.MetadataUserID); len(v) > 0 {
		by = v[0]
	}
	s.granted, s.grantedBy = append(s.granted, req), append(s.grantedBy, by)
	return &taskv1.GrantResponse{Id: "g1"}, s.grantErr
}

func TestTaskClient_CanExecute_AsksForTheExecuteAction(t *testing.T) {
	s := &accessServer{}
	c := startTaskServerFor(t, s)
	ok, err := c.CanExecute(ctxWith(""), "plan-1", "u-1")
	if err != nil || !ok {
		t.Fatalf("got %v %v", ok, err)
	}
	if r := s.resolved[0]; r.GetTaskId() != "plan-1" || r.GetUserId() != "u-1" || r.GetAction() != "execute" {
		t.Fatalf("request: %+v", r)
	}
}

func TestTaskClient_CanExecute_NoGrantIsAnAnswerNotAnError(t *testing.T) {
	c := startTaskServerFor(t, &accessServer{resolveErr: status.Error(codes.PermissionDenied, "TASK_NO_GRANT: none")})
	if ok, err := c.CanExecute(ctxWith(""), "plan-1", "u-1"); err != nil || ok {
		t.Fatalf("got %v %v", ok, err)
	}
}

func TestTaskClient_CanExecute_OutageAndMissingTask(t *testing.T) {
	c := startTaskServerFor(t, &accessServer{resolveErr: status.Error(codes.Unavailable, "down")})
	if _, err := c.CanExecute(ctxWith(""), "p", "u"); !errors.Is(err, domain.ErrTaskServiceDown) {
		t.Fatalf("got %v", err)
	}
	c = startTaskServerFor(t, &accessServer{resolveErr: status.Error(codes.NotFound, "TASK_NOT_FOUND")})
	if _, err := c.CanExecute(ctxWith(""), "p", "u"); !errors.Is(err, domain.ErrTaskNotFoundRemote) {
		t.Fatalf("got %v", err)
	}
}

func TestTaskClient_GrantExecute_WritesALevelUserTreeGrantAsTheGrantor(t *testing.T) {
	s := &accessServer{}
	c := startTaskServerFor(t, s)
	// The context's own user (the approver) must not leak into the call: task-service checks the grantor.
	if err := c.GrantExecute(ctxWith("approver"), "plan-1", "owner-1", "approver"); err != nil {
		t.Fatal(err)
	}
	g := s.granted[0]
	if g.GetTaskId() != "plan-1" || g.GetSubjectId() != "approver" || g.GetLevel() != taskv1.GrantLevel_GRANT_LEVEL_USER || !g.GetApplyTree() || g.GetExpiresAt() != nil {
		t.Fatalf("grant: %+v", g)
	}
	if s.grantedBy[0] != "owner-1" {
		t.Fatalf("written as %q", s.grantedBy[0])
	}
}

func TestTaskClient_GrantExecute_Refusals(t *testing.T) {
	c := startTaskServerFor(t, &accessServer{grantErr: status.Error(codes.PermissionDenied, "TASK_NO_GRANT: none")})
	if err := c.GrantExecute(ctxWith(""), "p", "o", "u"); !errors.Is(err, domain.ErrTaskForbidden) {
		t.Fatalf("got %v", err)
	}
}

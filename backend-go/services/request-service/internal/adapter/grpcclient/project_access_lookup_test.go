package grpcclient

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
)

type fakeAccessProjectClient struct {
	projectv1.ProjectServiceClient
	calls int
	resp  *projectv1.ResolveProjectAccessResponse
	err   error
	last  *projectv1.ResolveProjectAccessRequest
}

func (f *fakeAccessProjectClient) ResolveProjectAccess(_ context.Context, in *projectv1.ResolveProjectAccessRequest, _ ...grpc.CallOption) (*projectv1.ResolveProjectAccessResponse, error) {
	f.calls++
	f.last = in
	return f.resp, f.err
}

func TestProjectAccessLookup(t *testing.T) {
	ctx := context.Background()
	t.Run("reports existence and role, and caches the answer", func(t *testing.T) {
		f := &fakeAccessProjectClient{resp: &projectv1.ResolveProjectAccessResponse{Exists: true, Role: "owner"}}
		l := NewProjectAccessLookup(f)
		for i := 0; i < 3; i++ {
			exists, role, supported, err := l.Lookup(ctx, "t1", "p1", "u1")
			if err != nil || !exists || role != "owner" || !supported {
				t.Fatalf("got %v %q %v %v", exists, role, supported, err)
			}
		}
		if f.calls != 1 {
			t.Fatalf("expected one RPC, got %d", f.calls)
		}
		if f.last.GetProjectId() != "p1" || f.last.GetUserId() != "u1" {
			t.Fatalf("request = %+v", f.last)
		}
	})
	t.Run("the cache expires", func(t *testing.T) {
		f := &fakeAccessProjectClient{resp: &projectv1.ResolveProjectAccessResponse{Exists: true}}
		l := NewProjectAccessLookup(f)
		now := time.Now()
		l.now = func() time.Time { return now }
		_, _, _, _ = l.Lookup(ctx, "t1", "p1", "u1")
		now = now.Add(l.ttl + time.Second)
		_, _, _, _ = l.Lookup(ctx, "t1", "p1", "u1")
		if f.calls != 2 {
			t.Fatalf("calls = %d, want 2", f.calls)
		}
	})
	t.Run("a missing project is exists=false, not an error", func(t *testing.T) {
		l := NewProjectAccessLookup(&fakeAccessProjectClient{resp: &projectv1.ResolveProjectAccessResponse{}})
		ok, err := l.Exists(ctx, "t1", "ghost")
		if err != nil || ok {
			t.Fatalf("Exists = %v, %v", ok, err)
		}
	})
	t.Run("an older project-service keeps the legacy behaviour", func(t *testing.T) {
		l := NewProjectAccessLookup(&fakeAccessProjectClient{err: status.Error(codes.Unimplemented, "nope")})
		if _, _, supported, err := l.Lookup(ctx, "t1", "p1", "u1"); supported || err != nil {
			t.Fatalf("supported=%v err=%v", supported, err)
		}
		if ok, err := l.Exists(ctx, "t1", "p1"); !ok || err != nil {
			t.Fatalf("an unsupported lookup must assume the project exists, got %v, %v", ok, err)
		}
	})
	t.Run("any other failure is an error, never 'found'", func(t *testing.T) {
		l := NewProjectAccessLookup(&fakeAccessProjectClient{err: errors.New("down")})
		if ok, err := l.Exists(ctx, "t1", "p1"); err == nil || ok {
			t.Fatalf("Exists = %v, %v", ok, err)
		}
	})
}

func TestProjectRoleResolver_UsesAccessLookupWhenAvailable(t *testing.T) {
	f := &fakeAccessProjectClient{resp: &projectv1.ResolveProjectAccessResponse{Exists: true, Role: "member"}}
	r := NewProjectRoleResolver(f).WithAccessLookup(NewProjectAccessLookup(f))
	role, err := r.RoleOf(context.Background(), "t1", "p1", "u1")
	if err != nil || role != "member" {
		t.Fatalf("RoleOf = %q, %v", role, err)
	}
}

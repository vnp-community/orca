package grpcclient

import (
	"context"
	"errors"
	"testing"
	"time"

	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type fakeProjectClient struct {
	projectv1.ProjectServiceClient
	members  []*projectv1.Member
	err      error
	calls    int
	projects []*projectv1.Project
	lastMD   metadata.MD
}

func (f *fakeProjectClient) ListMembers(ctx context.Context, _ *projectv1.ListMembersRequest, _ ...grpc.CallOption) (*projectv1.ListMembersResponse, error) {
	f.calls++
	f.lastMD, _ = metadata.FromOutgoingContext(ctx)
	if f.err != nil {
		return nil, f.err
	}
	return &projectv1.ListMembersResponse{Members: f.members}, nil
}

func (f *fakeProjectClient) ListProjects(_ context.Context, _ *projectv1.ListProjectsRequest, _ ...grpc.CallOption) (*projectv1.ListProjectsResponse, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return &projectv1.ListProjectsResponse{Projects: f.projects}, nil
}

func newResolver(f *fakeProjectClient, now *time.Time) *ProjectRoleResolver {
	r := NewProjectRoleResolver(f)
	r.now = func() time.Time { return *now }
	return r
}

func TestProjectRoleResolver_RoleAndCacheTTL(t *testing.T) {
	now := time.Unix(1000, 0)
	f := &fakeProjectClient{members: []*projectv1.Member{{UserId: "u1", Role: projectv1.ProjectRole_PROJECT_ROLE_OWNER}, {UserId: "u2", Role: projectv1.ProjectRole_PROJECT_ROLE_MEMBER}}}
	r := newResolver(f, &now)
	ctx := context.Background()

	for user, want := range map[string]string{"u1": "owner", "u2": "member", "u3": ""} {
		if got, err := r.RoleOf(ctx, "t", "p", user); err != nil || got != want {
			t.Fatalf("%s: got %q err %v want %q", user, got, err, want)
		}
	}
	if f.calls != 3 {
		t.Fatalf("calls = %d", f.calls)
	}
	if f.lastMD.Get("x-orca-tenant-id")[0] != "t" || f.lastMD.Get("x-orca-user-id")[0] == "" {
		t.Fatalf("identity not forwarded: %v", f.lastMD)
	}

	// A member removed meanwhile stays "owner" until the TTL ends: the accepted risk of the 30 second cache.
	f.members = nil
	now = now.Add(29 * time.Second)
	if got, _ := r.RoleOf(ctx, "t", "p", "u1"); got != "owner" {
		t.Fatalf("cached role should survive inside the TTL, got %q", got)
	}
	now = now.Add(2 * time.Second)
	if got, _ := r.RoleOf(ctx, "t", "p", "u1"); got != "" {
		t.Fatalf("after the TTL the removal is seen, got %q", got)
	}
}

func TestProjectRoleResolver_NotMemberIsEmptyOtherErrorsFailClosed(t *testing.T) {
	now := time.Unix(1, 0)
	f := &fakeProjectClient{err: status.Error(codes.PermissionDenied, "PROJECT_NOT_AUTHORIZED")}
	r := newResolver(f, &now)
	if got, err := r.RoleOf(context.Background(), "t", "p", "u"); err != nil || got != "" {
		t.Fatalf("permission denied means not a member, got %q %v", got, err)
	}

	f2 := &fakeProjectClient{err: status.Error(codes.Unavailable, "down")}
	r2 := newResolver(f2, &now)
	if _, err := r2.RoleOf(context.Background(), "t", "p", "u"); err == nil {
		t.Fatal("an unreachable project-service must be an error, not a silent empty role")
	}
	f2.err = nil
	if got, err := r2.RoleOf(context.Background(), "t", "p", "u"); err != nil || got != "" {
		t.Fatalf("errors are not cached: %q %v", got, err)
	}
	if f2.calls != 2 {
		t.Fatalf("calls %d", f2.calls)
	}
}

func TestProjectRoleResolver_TenantAndUserKeyTheCache(t *testing.T) {
	now := time.Unix(1, 0)
	f := &fakeProjectClient{members: []*projectv1.Member{{UserId: "u", Role: projectv1.ProjectRole_PROJECT_ROLE_MEMBER}}}
	r := newResolver(f, &now)
	_, _ = r.RoleOf(context.Background(), "t1", "p", "u")
	_, _ = r.RoleOf(context.Background(), "t2", "p", "u")
	if f.calls != 2 {
		t.Fatalf("tenants must not share cache entries, calls = %d", f.calls)
	}
}

func TestProjectRoleResolver_ProjectsOf(t *testing.T) {
	now := time.Unix(1, 0)
	f := &fakeProjectClient{projects: []*projectv1.Project{{Id: "p1"}, {Id: "p2"}}}
	r := newResolver(f, &now)
	got, err := r.ProjectsOf(context.Background(), "t", "u")
	if err != nil || len(got) != 2 || got[0] != "p1" {
		t.Fatalf("%v %v", got, err)
	}
	got[0] = "mutated"
	again, _ := r.ProjectsOf(context.Background(), "t", "u")
	if again[0] != "p1" || f.calls != 1 {
		t.Fatalf("cache must return copies and be reused: %v calls %d", again, f.calls)
	}
	f.err = errors.New("boom")
	now = now.Add(time.Minute)
	if _, err := r.ProjectsOf(context.Background(), "t", "u"); err == nil {
		t.Fatal("error expected after expiry")
	}
}

package grpcclient

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type fakeTenantClient struct {
	tenantv1.TenantServiceClient
	teams   map[string][]string
	members map[string][]string
	err     error
	calls   int
	md      metadata.MD
}

func (f *fakeTenantClient) ListTeamsForUser(ctx context.Context, in *tenantv1.ListTeamsForUserRequest, _ ...grpc.CallOption) (*tenantv1.ListTeamsForUserResponse, error) {
	f.calls++
	f.md, _ = metadata.FromOutgoingContext(ctx)
	return &tenantv1.ListTeamsForUserResponse{TeamIds: f.teams[in.GetUserId()]}, f.err
}

func (f *fakeTenantClient) ListTeamMembers(ctx context.Context, in *tenantv1.ListTeamMembersRequest, _ ...grpc.CallOption) (*tenantv1.ListTeamMembersResponse, error) {
	f.calls++
	f.md, _ = metadata.FromOutgoingContext(ctx)
	var ms []*tenantv1.TeamMember
	for _, u := range f.members[in.GetTeamId()] {
		ms = append(ms, &tenantv1.TeamMember{UserId: u})
	}
	return &tenantv1.ListTeamMembersResponse{Members: ms}, f.err
}

func tctx(id string) context.Context { return tenant.WithTenantID(context.Background(), id) }

func TestTeamMembershipResolver_ForwardsTenantInMetadataAndCaches(t *testing.T) {
	c := &fakeTenantClient{teams: map[string][]string{"u1": {"t1", "t2"}}, members: map[string][]string{"t1": {"u1", "u2"}}}
	r := NewTeamMembershipResolver(c, time.Minute)
	for i := 0; i < 2; i++ {
		got, err := r.TeamsForUser(tctx("tenant-a"), "u1")
		if err != nil || !reflect.DeepEqual(got, []string{"t1", "t2"}) {
			t.Fatalf("%v %v", got, err)
		}
	}
	if c.calls != 1 {
		t.Fatalf("second lookup must hit the cache, calls=%d", c.calls)
	}
	if got := c.md.Get(grpcmw.MetadataTenantID); len(got) != 1 || got[0] != "tenant-a" {
		t.Fatalf("tenant must travel in metadata, got %v", c.md)
	}
	if _, err := r.TeamsForUser(tctx("tenant-b"), "u1"); err != nil || c.calls != 2 {
		t.Fatalf("a different tenant must not share the cache entry: calls=%d err=%v", c.calls, err)
	}
	m, err := r.MembersOfTeam(tctx("tenant-a"), "t1")
	if err != nil || !reflect.DeepEqual(m, []string{"u1", "u2"}) {
		t.Fatalf("%v %v", m, err)
	}
	if _, err := r.TeamsForUser(context.Background(), "u1"); err == nil {
		t.Fatal("a call without tenant must fail")
	}
}

func TestTeamMembershipResolver_ErrorsBecomeDirectoryUnavailableAndAreNotCached(t *testing.T) {
	c := &fakeTenantClient{err: errors.New("connection refused")}
	r := NewTeamMembershipResolver(c, time.Minute)
	for i := 0; i < 2; i++ {
		_, err := r.MembersOfTeam(tctx("a"), "t1")
		var ae *apperrors.AppError
		if !errors.As(err, &ae) || ae.Code != "REQUEST_APPROVAL_DIRECTORY_UNAVAILABLE" || ae.Kind != apperrors.KindUnavailable {
			t.Fatalf("err = %v", err)
		}
	}
	if c.calls != 2 {
		t.Fatalf("failures must not be cached, calls=%d", c.calls)
	}
}

func TestDirectoryCache_ExpiresAfterTTL(t *testing.T) {
	c := newDirectoryCache(time.Minute)
	now := time.Unix(1000, 0)
	c.now = func() time.Time { return now }
	c.put("k", []string{"a"})
	if v, ok := c.get("k"); !ok || v[0] != "a" {
		t.Fatal("fresh entry must hit")
	}
	now = now.Add(61 * time.Second)
	if _, ok := c.get("k"); ok {
		t.Fatal("expired entry must miss")
	}
}

type fakeAuthClient struct {
	authv1.AuthServiceClient
	pages map[string]*authv1.ListUsersResponse
	seen  []*authv1.ListUsersRequest
}

func (f *fakeAuthClient) ListUsers(_ context.Context, in *authv1.ListUsersRequest, _ ...grpc.CallOption) (*authv1.ListUsersResponse, error) {
	f.seen = append(f.seen, in)
	return f.pages[in.GetPageToken()], nil
}

func TestAdminDirectoryResolver_PagesFiltersActiveAdminsAndUsesCtxTenant(t *testing.T) {
	c := &fakeAuthClient{pages: map[string]*authv1.ListUsersResponse{
		"":   {Users: []*authv1.User{{Id: "a1", Role: authv1.Role_ROLE_ADMIN, IsActive: true}, {Id: "u1", Role: authv1.Role_ROLE_USER, IsActive: true}}, NextPageToken: "p2"},
		"p2": {Users: []*authv1.User{{Id: "a2", Role: authv1.Role_ROLE_ADMIN, IsActive: true}, {Id: "a3", Role: authv1.Role_ROLE_ADMIN, IsActive: false}}},
	}}
	r := NewAdminDirectoryResolver(c, time.Minute)
	got, err := r.ListAdmins(tctx("tenant-a"), "tenant-a")
	if err != nil || !reflect.DeepEqual(got, []string{"a1", "a2"}) {
		t.Fatalf("%v %v", got, err)
	}
	if len(c.seen) != 2 || c.seen[0].GetTenantId() != "tenant-a" {
		t.Fatalf("requests = %v", c.seen)
	}
	if _, err := r.ListAdmins(tctx("tenant-a"), "tenant-a"); err != nil || len(c.seen) != 2 {
		t.Fatalf("cached: requests=%d err=%v", len(c.seen), err)
	}
	if _, err := r.ListAdmins(tctx("tenant-a"), "tenant-b"); err == nil {
		t.Fatal("asking for another tenant's admins must be refused")
	}
}

func TestUnavailableDirectory_FailsClosed(t *testing.T) {
	u := UnavailableDirectory{}
	if v, err := u.MembersOfTeam(tctx("a"), "t"); v != nil || err == nil {
		t.Fatal("must not answer 'nobody'")
	}
	if v, err := u.ListAdmins(tctx("a"), "a"); v != nil || err == nil {
		t.Fatal("must not answer 'nobody'")
	}
}

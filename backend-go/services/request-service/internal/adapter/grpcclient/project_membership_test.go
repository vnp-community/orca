package grpcclient

import (
	"context"
	"testing"

	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeMemberProjects struct {
	projectv1.ProjectServiceClient
	members []*projectv1.Member
	err     error
}

func (f *fakeMemberProjects) ListMembers(context.Context, *projectv1.ListMembersRequest, ...grpc.CallOption) (*projectv1.ListMembersResponse, error) {
	return &projectv1.ListMembersResponse{Members: f.members}, f.err
}

func TestProjectMembershipClient(t *testing.T) {
	members := []*projectv1.Member{{UserId: "u1"}, {UserId: "u2"}}
	ctx := ctxWith("u1")
	cases := []struct {
		name    string
		client  fakeMemberProjects
		user    string
		want    bool
		wantErr bool
	}{
		{"member", fakeMemberProjects{members: members}, "u2", true, false},
		{"not a member", fakeMemberProjects{members: members}, "u9", false, false},
		{"project not readable", fakeMemberProjects{err: status.Error(codes.PermissionDenied, "x")}, "u1", false, false},
		{"project gone", fakeMemberProjects{err: status.Error(codes.NotFound, "x")}, "u1", false, false},
		{"outage is an error, not 'no'", fakeMemberProjects{err: status.Error(codes.Unavailable, "x")}, "u1", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewProjectMembershipClient(&tc.client).IsMember(ctx, "p1", tc.user)
			if got != tc.want || (err != nil) != tc.wantErr {
				t.Fatalf("got %v %v", got, err)
			}
		})
	}
	if _, err := NewProjectMembershipClient(&fakeMemberProjects{}).IsMember(ctxWith(""), "p1", "u1"); err == nil {
		t.Fatal("the lookup runs as the asking user, so one is required")
	}
	if ok, err := (NoProjectMembership{}).IsMember(ctx, "p", "u"); ok || err != nil {
		t.Fatal("without project-service nobody is a member")
	}
}

//go:build integration

package main

import (
	"context"
	"net"
	"strconv"
	"sync"
	"testing"

	"github.com/stablyai/orca-go/common/grpcmw"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// fakeProjectMembers answers ListMembers for the project-role check: users added with own are owners and users added with
// member are members of every project; everyone else is not a member.
type fakeProjectMembers struct {
	projectv1.UnimplementedProjectServiceServer
	mu      sync.Mutex
	owners  map[string]bool
	members map[string]bool
}

func (f *fakeProjectMembers) own(userID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.owners[userID] = true
}

func (f *fakeProjectMembers) member(userID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.members[userID] = true
}

func (f *fakeProjectMembers) ListMembers(ctx context.Context, _ *projectv1.ListMembersRequest) (*projectv1.ListMembersResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*projectv1.Member
	for _, id := range md.Get(grpcmw.MetadataUserID) {
		if f.owners[id] {
			out = append(out, &projectv1.Member{UserId: id, Role: projectv1.ProjectRole_PROJECT_ROLE_OWNER})
		}
		if f.members[id] {
			out = append(out, &projectv1.Member{UserId: id, Role: projectv1.ProjectRole_PROJECT_ROLE_MEMBER})
		}
	}
	return &projectv1.ListMembersResponse{Members: out}, nil
}

func startFakeProjectMembers(t *testing.T) (addr string, fake *fakeProjectMembers) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	fake = &fakeProjectMembers{owners: map[string]bool{}, members: map[string]bool{}}
	srv := grpc.NewServer()
	projectv1.RegisterProjectServiceServer(srv, fake)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return "127.0.0.1:" + strconv.Itoa(lis.Addr().(*net.TCPAddr).Port), fake
}

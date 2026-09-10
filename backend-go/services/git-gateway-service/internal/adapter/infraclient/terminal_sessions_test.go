package infraclient

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// fakeInfraFleetServiceClient is this package's own minimal
// InfraFleetServiceClient test double — embeds the nil interface and
// overrides only ListTerminalSessions/KillTerminalSession, the two methods
// TerminalSessionLister actually calls.
type fakeInfraFleetServiceClient struct {
	infrafleetv1.InfraFleetServiceClient

	listFunc  func(ctx context.Context, in *infrafleetv1.ListTerminalSessionsRequest) (*infrafleetv1.ListTerminalSessionsResponse, error)
	killErr   error
	gotKillID string
}

func (f *fakeInfraFleetServiceClient) ListTerminalSessions(ctx context.Context, in *infrafleetv1.ListTerminalSessionsRequest, _ ...grpc.CallOption) (*infrafleetv1.ListTerminalSessionsResponse, error) {
	return f.listFunc(ctx, in)
}

func (f *fakeInfraFleetServiceClient) KillTerminalSession(ctx context.Context, in *infrafleetv1.KillTerminalSessionRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	f.gotKillID = in.GetPtyId()
	if f.killErr != nil {
		return nil, f.killErr
	}
	return &emptypb.Empty{}, nil
}

func TestTerminalSessionLister_ListSessions_MapsCwdAndPtyIDCorrectly(t *testing.T) {
	var gotReq *infrafleetv1.ListTerminalSessionsRequest
	client := &fakeInfraFleetServiceClient{
		listFunc: func(_ context.Context, in *infrafleetv1.ListTerminalSessionsRequest) (*infrafleetv1.ListTerminalSessionsResponse, error) {
			gotReq = in
			return &infrafleetv1.ListTerminalSessionsResponse{
				Sessions: []*infrafleetv1.TerminalSession{
					{PtyId: "pty-1", Cwd: "/repo-feature"},
					{PtyId: "pty-2", Cwd: "/repo-other"},
				},
			}, nil
		},
	}
	lister := NewTerminalSessionLister(client)

	refs, err := lister.ListSessions(context.Background(), "conn-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetConnectionId() != "conn-1" {
		t.Errorf("expected ListTerminalSessionsRequest.ConnectionId=conn-1, got %q", gotReq.GetConnectionId())
	}
	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(refs))
	}
	if refs[0].PtyID != "pty-1" || refs[0].Cwd != "/repo-feature" {
		t.Errorf("unexpected ref[0]: %+v", refs[0])
	}
	if refs[1].PtyID != "pty-2" || refs[1].Cwd != "/repo-other" {
		t.Errorf("unexpected ref[1]: %+v", refs[1])
	}
}

func TestTerminalSessionLister_ListSessions_PropagatesError(t *testing.T) {
	client := &fakeInfraFleetServiceClient{
		listFunc: func(_ context.Context, _ *infrafleetv1.ListTerminalSessionsRequest) (*infrafleetv1.ListTerminalSessionsResponse, error) {
			return nil, errors.New("infra-fleet-service unreachable")
		},
	}
	lister := NewTerminalSessionLister(client)

	if _, err := lister.ListSessions(context.Background(), "conn-1"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestTerminalSessionLister_Kill_ForwardsPtyID(t *testing.T) {
	client := &fakeInfraFleetServiceClient{}
	lister := NewTerminalSessionLister(client)

	if err := lister.Kill(context.Background(), "pty-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.gotKillID != "pty-1" {
		t.Errorf("expected KillTerminalSessionRequest.PtyId=pty-1, got %q", client.gotKillID)
	}
}

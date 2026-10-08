package grpcclient

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stablyai/orca-go/common/grpcmw"
	issuetrackingv1 "github.com/stablyai/orca-go/proto/gen/go/orca/issuetracking/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type fakeIssueServer struct {
	issuetrackingv1.UnimplementedIssueTrackingServiceServer
	gotMD  metadata.MD
	gotReq *issuetrackingv1.GetIssueRequest
	reply  *issuetrackingv1.Issue
	err    error
}

func (f *fakeIssueServer) GetIssue(ctx context.Context, in *issuetrackingv1.GetIssueRequest) (*issuetrackingv1.Issue, error) {
	f.gotMD, _ = metadata.FromIncomingContext(ctx)
	f.gotReq = in
	return f.reply, f.err
}

func startIssueServer(t *testing.T, srv *fakeIssueServer) *IssueTrackingClient {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	g := grpc.NewServer()
	issuetrackingv1.RegisterIssueTrackingServiceServer(g, srv)
	go func() { _ = g.Serve(lis) }()
	t.Cleanup(g.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return NewIssueTrackingClient(issuetrackingv1.NewIssueTrackingServiceClient(conn))
}

func TestIssueTrackingClient_ForwardsTenantAndUser(t *testing.T) {
	srv := &fakeIssueServer{reply: &issuetrackingv1.Issue{Title: "T"}}
	c := startIssueServer(t, srv)
	if _, err := c.GetIssue(ctxWithIdentity(), domain.SourceProviderJira, "ENG-1", "site"); err != nil {
		t.Fatal(err)
	}
	if srv.gotMD.Get(grpcmw.MetadataTenantID)[0] != "11111111-1111-1111-1111-111111111111" || srv.gotMD.Get(grpcmw.MetadataUserID)[0] != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("metadata = %v", srv.gotMD)
	}
	if srv.gotReq.GetProvider() != issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA || srv.gotReq.GetIssueId() != "ENG-1" || srv.gotReq.GetWorkspaceId() != "site" {
		t.Fatalf("%+v", srv.gotReq)
	}
}

func TestIssueTrackingClient_NotFoundMapsToSentinel(t *testing.T) {
	c := startIssueServer(t, &fakeIssueServer{err: status.Error(codes.NotFound, "nope")})
	_, err := c.GetIssue(ctxWithIdentity(), domain.SourceProviderLinear, "ABC-1", "")
	if !errors.Is(err, usecase.ErrIssueNotFound) {
		t.Fatalf("err = %v", err)
	}
	c = startIssueServer(t, &fakeIssueServer{err: status.Error(codes.Unavailable, "down")})
	if _, err := c.GetIssue(ctxWithIdentity(), domain.SourceProviderLinear, "ABC-1", ""); err == nil || errors.Is(err, usecase.ErrIssueNotFound) {
		t.Fatalf("other errors must not look like not-found: %v", err)
	}
}

func TestIssueTrackingClient_MapsHints(t *testing.T) {
	c := startIssueServer(t, &fakeIssueServer{reply: &issuetrackingv1.Issue{
		Title: "T", DescriptionMarkdown: "D", Url: "U", Labels: []string{"a"},
		IssueType: &issuetrackingv1.IssueType{Name: "Bug"}, Priority: &issuetrackingv1.Priority{Name: "High"},
	}})
	got, err := c.GetIssue(ctxWithIdentity(), domain.SourceProviderJira, "ENG-1", "")
	if err != nil || got.Title != "T" || got.Body != "D" || got.URL != "U" || got.Hints.IssueType != "Bug" || got.Hints.Priority != "High" || len(got.Hints.Labels) != 1 {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestIssueTrackingClient_UnsupportedProvider(t *testing.T) {
	srv := &fakeIssueServer{reply: &issuetrackingv1.Issue{}}
	c := startIssueServer(t, srv)
	for _, p := range []domain.SourceProvider{domain.SourceProviderGithub, domain.SourceProviderGitlab, domain.SourceProviderManual, domain.SourceProviderWebhook} {
		if _, err := c.GetIssue(ctxWithIdentity(), p, "x", ""); err == nil {
			t.Errorf("%s must be rejected", p)
		}
	}
	if srv.gotReq != nil {
		t.Fatal("no call may reach the tracker for unsupported providers")
	}
}

func TestIdentityForwarding_MissingUserFails(t *testing.T) {
	if _, err := withIdentityMetadata(tenantOnly()); err == nil {
		t.Fatal("want REQUEST_REPORTER_REQUIRED")
	}
	if _, err := withIdentityMetadata(context.Background()); err == nil {
		t.Fatal("want tenant error")
	}
}

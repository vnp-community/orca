package grpcclient

import (
	"context"
	"net"
	"testing"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/internalcaller"
	"github.com/stablyai/orca-go/common/tenant"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const lookupMethod = "/orca.request.v1.RequestService/LookupRequestBySource"

// fakeRequestService answers per tenant, the way the real one scopes by the tenant it extracts from metadata.
type fakeRequestService struct {
	requestv1.UnimplementedRequestServiceServer
	open   map[string]string // tenant|provider|site|ref -> request id
	seen   []*requestv1.LookupRequestBySourceRequest
	tenant []string
}

func (f *fakeRequestService) LookupRequestBySource(ctx context.Context, in *requestv1.LookupRequestBySourceRequest) (*requestv1.LookupRequestBySourceResponse, error) {
	id, _ := tenant.TenantID(ctx)
	f.seen = append(f.seen, in)
	f.tenant = append(f.tenant, id)
	if rid, ok := f.open[id+"|"+in.GetProvider()+"|"+in.GetSite()+"|"+in.GetRef()]; ok {
		return &requestv1.LookupRequestBySourceResponse{Found: true, RequestId: rid}, nil
	}
	return &requestv1.LookupRequestBySourceResponse{}, nil
}

// startRequestService runs a real gRPC server with the same guard and tenant extraction request-service installs,
// and returns a client dialled with the given token.
func startRequestService(t *testing.T, svc *fakeRequestService, serverToken, clientToken string) *RequestClient {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer(grpc.ChainUnaryInterceptor(grpcmw.TenantExtractionInterceptor(), internalcaller.Guard(serverToken, lookupMethod)))
	requestv1.RegisterRequestServiceServer(srv, svc)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(internalcaller.ClientInterceptor(clientToken)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return NewRequestClient(requestv1.NewRequestServiceClient(conn))
}

func TestRequestClient_LookupSendsTenantAndKeyAndReadsFound(t *testing.T) {
	svc := &fakeRequestService{open: map[string]string{"tenant-1|jira|https://a.atlassian.net|ENG-4": "req-1"}}
	c := startRequestService(t, svc, "secret", "secret")
	found, err := c.Lookup(context.Background(), "tenant-1", "jira", "https://a.atlassian.net", "ENG-4")
	if err != nil || !found {
		t.Fatalf("got (%v, %v), want found", found, err)
	}
	if len(svc.tenant) != 1 || svc.tenant[0] != "tenant-1" {
		t.Fatalf("the event's tenant must travel as metadata, got %v", svc.tenant)
	}
	if got := svc.seen[0]; got.GetProvider() != "jira" || got.GetSite() != "https://a.atlassian.net" || got.GetRef() != "ENG-4" {
		t.Fatalf("request %+v", got)
	}
}

func TestRequestClient_NotFoundAndOtherTenantAreFalseNotErrors(t *testing.T) {
	svc := &fakeRequestService{open: map[string]string{"tenant-1|jira||ENG-4": "req-1"}}
	c := startRequestService(t, svc, "secret", "secret")
	for name, tc := range map[string]struct{ tenant, ref string }{"unknown ref": {"tenant-1", "ENG-99"}, "other tenant": {"tenant-2", "ENG-4"}} {
		found, err := c.Lookup(context.Background(), tc.tenant, "jira", "", tc.ref)
		if err != nil || found {
			t.Errorf("%s: got (%v, %v), want (false, nil)", name, found, err)
		}
	}
}

func TestRequestClient_WrongOrMissingTokenSurfacesAsAnErrorSoTheEventIsRetried(t *testing.T) {
	svc := &fakeRequestService{open: map[string]string{"tenant-1|jira||ENG-4": "req-1"}}
	for name, token := range map[string]string{"wrong": "nope", "missing": ""} {
		c := startRequestService(t, svc, "secret", token)
		found, err := c.Lookup(context.Background(), "tenant-1", "jira", "", "ENG-4")
		if found || status.Code(err) != codes.PermissionDenied {
			t.Errorf("%s token: got (%v, %v), want a PermissionDenied error and found=false", name, found, err)
		}
	}
	if len(svc.seen) != 0 {
		t.Fatalf("the guard let %d calls through", len(svc.seen))
	}
}

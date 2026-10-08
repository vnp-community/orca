package contracttest

import (
	"context"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/grpcmw"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	requestgrpc "github.com/stablyai/orca-go/services/request-service/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func startRequestServer(t *testing.T, env Env) requestv1.RequestServiceClient {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer(grpc.ChainUnaryInterceptor(grpcmw.TenantExtractionInterceptor()))
	requestv1.RegisterRequestServiceServer(srv, requestgrpc.NewServer(usecase.NewGetRequest(env.Requests), usecase.NewListRequests(env.Requests)))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return requestv1.NewRequestServiceClient(conn)
}

func rpcCtx(tenantID string) context.Context {
	return metadata.AppendToOutgoingContext(context.Background(), grpcmw.MetadataTenantID, tenantID)
}

// RunRequestServiceContract drives GetRequest and ListRequests over an in-process gRPC
// connection against the dialect's real repository.
func RunRequestServiceContract(t *testing.T, newEnv func(t *testing.T) Env) {
	env := newEnv(t)
	client := startRequestServer(t, env)

	t.Run("TestGetRequest_ReturnsStoredRow", func(t *testing.T) {
		tenantID := newTenant()
		conf := 0.5
		r := createRequest(t, env, CtxForTenant(tenantID), func(r *domain.Request) {
			r.Type, r.Confidence, r.Title = domain.RequestTypeBug, &conf, "stored"
		})
		resp, err := client.GetRequest(rpcCtx(tenantID), &requestv1.GetRequestRequest{Id: r.ID})
		if err != nil {
			t.Fatal(err)
		}
		got := resp.GetRequest()
		if got.GetTitle() != "stored" || got.GetType() != "bug" || got.GetNumber() != 1 || got.GetStatus() != "new" || got.Confidence == nil || *got.Confidence != 0.5 {
			t.Fatalf("unexpected row: %+v", got)
		}
	})

	t.Run("TestGetRequest_NotFound", func(t *testing.T) {
		for _, id := range []string{uuid.NewString(), "not-a-uuid"} {
			_, err := client.GetRequest(rpcCtx(newTenant()), &requestv1.GetRequestRequest{Id: id})
			if status.Code(err) != codes.NotFound || status.Convert(err).Message()[:17] != "REQUEST_NOT_FOUND" {
				t.Errorf("id %q: want NotFound/REQUEST_NOT_FOUND, got %v", id, err)
			}
		}
	})

	t.Run("TestGetRequest_OtherTenantNotFound", func(t *testing.T) {
		owner := newTenant()
		r := createRequest(t, env, CtxForTenant(owner), nil)
		_, err := client.GetRequest(rpcCtx(newTenant()), &requestv1.GetRequestRequest{Id: r.ID})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("other tenant must get NotFound, got %v", err)
		}
	})

	t.Run("TestGetRequest_NoTenantRejected", func(t *testing.T) {
		_, err := client.GetRequest(context.Background(), &requestv1.GetRequestRequest{Id: uuid.NewString()})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("missing tenant metadata must be InvalidArgument, got %v", err)
		}
	})

	t.Run("TestListRequests_PaginatesAndFilters", func(t *testing.T) {
		tenantID := newTenant()
		ctx := CtxForTenant(tenantID)
		proj := uuid.NewString()
		for i := 0; i < 120; i++ {
			i := i
			createRequest(t, env, ctx, func(r *domain.Request) {
				if i%4 == 0 {
					r.ProjectID, r.Type = proj, domain.RequestTypeBug
				}
				if i%3 == 0 {
					r.Status = domain.RequestStatusClassifying
				}
			})
		}
		var sizes []int
		seen := map[string]bool{}
		token := ""
		for {
			resp, err := client.ListRequests(rpcCtx(tenantID), &requestv1.ListRequestsRequest{PageSize: 50, PageToken: token})
			if err != nil {
				t.Fatal(err)
			}
			sizes = append(sizes, len(resp.GetRequests()))
			for _, r := range resp.GetRequests() {
				if seen[r.GetId()] {
					t.Fatalf("duplicate %s", r.GetId())
				}
				seen[r.GetId()] = true
			}
			if token = resp.GetNextPageToken(); token == "" {
				break
			}
		}
		if len(sizes) != 3 || sizes[0] != 50 || sizes[1] != 50 || sizes[2] != 20 || len(seen) != 120 {
			t.Fatalf("pages = %v, distinct = %d; want [50 50 20] and 120", sizes, len(seen))
		}

		count := func(req *requestv1.ListRequestsRequest) int {
			req.PageSize = 200
			resp, err := client.ListRequests(rpcCtx(tenantID), req)
			if err != nil {
				t.Fatal(err)
			}
			return len(resp.GetRequests())
		}
		if got := count(&requestv1.ListRequestsRequest{ProjectId: proj}); got != 30 {
			t.Errorf("project filter: %d, want 30", got)
		}
		if got := count(&requestv1.ListRequestsRequest{Status: []string{"classifying"}}); got != 40 {
			t.Errorf("status filter: %d, want 40", got)
		}
		if got := count(&requestv1.ListRequestsRequest{Status: []string{"classifying", "new"}}); got != 120 {
			t.Errorf("repeated status filter: %d, want 120", got)
		}
		if got := count(&requestv1.ListRequestsRequest{Type: []string{"bug", "task"}}); got != 30 {
			t.Errorf("repeated type filter: %d, want 30", got)
		}
		if got := count(&requestv1.ListRequestsRequest{ProjectId: proj, Status: []string{"classifying"}, Type: []string{"bug"}}); got != 10 {
			t.Errorf("combined filter: %d, want 10", got)
		}
		if _, err := client.ListRequests(rpcCtx(tenantID), &requestv1.ListRequestsRequest{Status: []string{"bogus"}}); status.Code(err) != codes.InvalidArgument {
			t.Errorf("unknown status must be InvalidArgument, got %v", err)
		}
		if _, err := client.ListRequests(rpcCtx(tenantID), &requestv1.ListRequestsRequest{PageToken: "garbage!"}); status.Code(err) != codes.InvalidArgument {
			t.Errorf("bad page token must be InvalidArgument, got %v", err)
		}
	})

	t.Run("TestListRequests_PageSizeCapped", func(t *testing.T) {
		tenantID := newTenant()
		ctx := CtxForTenant(tenantID)
		for i := 0; i < 205; i++ {
			createRequest(t, env, ctx, nil)
		}
		resp, err := client.ListRequests(rpcCtx(tenantID), &requestv1.ListRequestsRequest{PageSize: 500})
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.GetRequests()) != 200 || resp.GetNextPageToken() == "" {
			t.Fatalf("page_size=500 returned %d rows (token %q), want capped at 200 with a next page", len(resp.GetRequests()), resp.GetNextPageToken())
		}
	})
}

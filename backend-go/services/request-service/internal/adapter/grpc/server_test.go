package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// stubRequestRepository serves canned data; unused port methods panic via the nil embed.
type stubRequestRepository struct {
	usecase.RequestRepository
	byID     map[string]domain.Request
	lastList usecase.ListFilter
	listOut  usecase.ListResult
}

func (s *stubRequestRepository) Get(_ context.Context, id string) (domain.Request, error) {
	if r, ok := s.byID[id]; ok {
		return r, nil
	}
	return domain.Request{}, domain.ErrRequestNotFound(id)
}

func (s *stubRequestRepository) List(_ context.Context, f usecase.ListFilter) (usecase.ListResult, error) {
	s.lastList = f
	return s.listOut, nil
}

func newTestServer(repo *stubRequestRepository) *Server {
	return NewServer(usecase.NewGetRequest(repo), usecase.NewListRequests(repo))
}

func tenantCtx() context.Context {
	return tenant.WithTenantID(context.Background(), "11111111-1111-4111-8111-111111111111")
}

func TestServer_ListBacklogUnimplementedUntilExecutionIsAttached(t *testing.T) {
	srv := newTestServer(&stubRequestRepository{})
	_, err := srv.ListBacklog(tenantCtx(), &requestv1.ListBacklogRequest{})
	if status.Code(err) != codes.Unimplemented {
		t.Errorf("ListBacklog must be Unimplemented while no execution use cases are attached, got %v", status.Code(err))
	}
}

func TestGetRequest_NotFound(t *testing.T) {
	srv := newTestServer(&stubRequestRepository{byID: map[string]domain.Request{}})
	for _, id := range []string{uuid.NewString(), "not-a-uuid", ""} {
		_, err := srv.GetRequest(tenantCtx(), &requestv1.GetRequestRequest{Id: id})
		if status.Code(err) != codes.NotFound {
			t.Errorf("id %q: want NotFound, got %v", id, err)
		}
		if err != nil && status.Convert(err).Message()[:len("REQUEST_NOT_FOUND")] != "REQUEST_NOT_FOUND" {
			t.Errorf("id %q: message %q lacks REQUEST_NOT_FOUND", id, status.Convert(err).Message())
		}
	}
}

func TestGetRequest_RequiresTenant(t *testing.T) {
	srv := newTestServer(&stubRequestRepository{})
	_, err := srv.GetRequest(context.Background(), &requestv1.GetRequestRequest{Id: uuid.NewString()})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("missing tenant: want InvalidArgument, got %v", err)
	}
}

func TestListRequests_PageSizeCappedAndFiltersMapped(t *testing.T) {
	repo := &stubRequestRepository{}
	srv := newTestServer(repo)
	_, err := srv.ListRequests(tenantCtx(), &requestv1.ListRequestsRequest{
		ProjectId: "p1", Status: []string{"new", "classifying"}, Type: []string{"bug"}, PageSize: 500, PageToken: "tok",
	})
	if err != nil {
		t.Fatal(err)
	}
	f := repo.lastList
	if f.PageSize != 200 {
		t.Errorf("page size = %d, want capped at 200", f.PageSize)
	}
	if f.ProjectID != "p1" || f.PageToken != "tok" || len(f.Statuses) != 2 || len(f.Types) != 1 || f.Types[0] != domain.RequestTypeBug {
		t.Errorf("filter not mapped: %+v", f)
	}
}

func TestListRequests_RejectsUnknownEnums(t *testing.T) {
	srv := newTestServer(&stubRequestRepository{})
	for _, req := range []*requestv1.ListRequestsRequest{
		{Status: []string{"bogus"}},
		{Type: []string{"bogus"}},
		{PageSize: -1},
	} {
		if _, err := srv.ListRequests(tenantCtx(), req); status.Code(err) != codes.InvalidArgument {
			t.Errorf("%+v: want InvalidArgument, got %v", req, err)
		}
	}
}

func TestMapper_RoundTripFields(t *testing.T) {
	conf := 0.75
	engine := domain.EngineOpenSpec
	now := time.Date(2026, 10, 7, 1, 2, 3, 456000, time.UTC)
	in := domain.Request{
		ID: "id1", ProjectID: "p1", Number: 42, Title: "t", Body: "b", SourceProvider: domain.SourceProviderJira,
		SourceRef: "ABC-1", SourceURL: "https://x/ABC-1", SourceSite: "x", Type: domain.RequestTypeBug,
		TypeSource: domain.TypeSourceAI, Size: domain.RequestSizeM, Urgency: domain.UrgencyUrgent, Confidence: &conf,
		ClassificationReason: "why", Status: domain.RequestStatusRequestBacklog, ReturnedFromStage: domain.ReturnStagePlan,
		ReturnReason: "rr", PlanTaskID: "pt", ReporterID: "u1", CreatedAt: now, UpdatedAt: now.Add(time.Second), Version: 7,
		SolutionEngine: &engine,
	}
	out := toProtoRequest(in)
	if out.Id != "id1" || out.ProjectId != "p1" || out.Number != 42 || out.Title != "t" || out.Body != "b" ||
		out.SourceProvider != "jira" || out.SourceRef != "ABC-1" || out.SourceUrl != "https://x/ABC-1" || out.SourceSite != "x" ||
		out.Type != "bug" || out.TypeSource != "ai" || out.Size != "M" || out.Urgency != "urgent" ||
		out.ClassificationReason != "why" || out.Status != "request_backlog" || out.ReturnedFromStage != "plan" ||
		out.ReturnReason != "rr" || out.PlanTaskId != "pt" || out.ReporterId != "u1" || out.Version != 7 {
		t.Fatalf("scalar mapping wrong: %+v", out)
	}
	if out.Confidence == nil || *out.Confidence != 0.75 {
		t.Errorf("confidence not mapped: %v", out.Confidence)
	}
	if !out.CreatedAt.AsTime().Equal(now) || !out.UpdatedAt.AsTime().Equal(now.Add(time.Second)) {
		t.Errorf("timestamps wrong")
	}

	empty := toProtoRequest(domain.Request{CreatedAt: now, UpdatedAt: now})
	if empty.Type != "" || empty.Size != "" || empty.TypeSource != "" || empty.ReturnedFromStage != "" || empty.ProjectId != "" {
		t.Errorf("empty optional fields must map to empty strings: %+v", empty)
	}
	if empty.Confidence != nil {
		t.Errorf("nil confidence must stay unset, got %v", *empty.Confidence)
	}
}

func TestGetRequest_ReturnsMappedRequest(t *testing.T) {
	id := uuid.NewString()
	repo := &stubRequestRepository{byID: map[string]domain.Request{id: {ID: id, Title: "hello", CreatedAt: time.Now(), UpdatedAt: time.Now()}}}
	resp, err := newTestServer(repo).GetRequest(tenantCtx(), &requestv1.GetRequestRequest{Id: id})
	if err != nil || resp.GetRequest().GetTitle() != "hello" {
		t.Fatalf("resp=%v err=%v", resp, err)
	}
}

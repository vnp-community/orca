package grpc

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"github.com/stablyai/orca-go/services/request-service/schemas"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	artTenant = "11111111-1111-4111-8111-111111111111"
	artUser   = "22222222-2222-4222-8222-222222222222"
	artOther  = "33333333-3333-4333-8333-333333333333"
)

func artCtx(user string) context.Context {
	return tenant.WithUserID(tenant.WithTenantID(context.Background(), artTenant), user)
}

// tenantRepo is a request store that, like the real adapters, hides rows of other tenants.
type tenantRepo struct {
	usecase.RequestRepository
	rows map[string]domain.Request
}

func (r *tenantRepo) Get(ctx context.Context, id string) (domain.Request, error) {
	t, _ := tenant.TenantID(ctx)
	if row, ok := r.rows[id]; ok && row.TenantID == t {
		return row, nil
	}
	return domain.Request{}, domain.ErrRequestNotFound(id)
}

func (r *tenantRepo) GetByNumber(ctx context.Context, n int64) (domain.Request, error) {
	t, _ := tenant.TenantID(ctx)
	for _, row := range r.rows {
		if row.Number == n && row.TenantID == t {
			return row, nil
		}
	}
	return domain.Request{}, domain.ErrRequestNotFound("")
}

type revStub struct{ revs []domain.RequestRevision }

func (s *revStub) Append(context.Context, domain.RequestRevision) error { return nil }
func (s *revStub) Get(_ context.Context, id string, n int) (domain.RequestRevision, error) {
	for _, r := range s.revs {
		if r.RequestID == id && r.Revision == n {
			return r, nil
		}
	}
	return domain.RequestRevision{}, domain.ErrRequestRevisionNotFound(id, n)
}
func (s *revStub) List(_ context.Context, id string, after, limit int) ([]domain.RequestRevision, error) {
	var out []domain.RequestRevision
	for _, r := range s.revs {
		if r.RequestID == id && r.Revision > after && len(out) < limit {
			out = append(out, r)
		}
	}
	return out, nil
}

type covStub struct{ rows []domain.CoverageRow }

func (c *covStub) ReplaceForPlan(context.Context, string, string, []domain.CoverageRow) error {
	return nil
}
func (c *covStub) ListByRequest(context.Context, string) ([]domain.CoverageRow, error) {
	return c.rows, nil
}

type relStub struct{}

func (relStub) Insert(context.Context, domain.ArtifactRelation) (bool, error) { return true, nil }
func (relStub) ListByRequest(context.Context, string) ([]domain.ArtifactRelation, error) {
	return []domain.ArtifactRelation{{Rel: domain.RelationDerivedFrom, FromKind: domain.NodeSolution, FromID: "SOL-7.1", ToKind: domain.NodeRequest, ToID: "REQ-7"}}, nil
}

type linkStub struct{}

func (linkStub) Insert(context.Context, domain.RequestLink) error { return nil }
func (linkStub) ListParents(context.Context, string) ([]domain.RequestLink, error) {
	return nil, nil
}
func (linkStub) ListChildren(context.Context, string) ([]domain.RequestLink, error) { return nil, nil }
func (linkStub) Delete(context.Context, string, string) error                       { return nil }

type indexStub struct{ entry domain.IndexEntry }

func (i indexStub) Insert(context.Context, domain.IndexEntry) (bool, error) { return true, nil }
func (i indexStub) Resolve(ctx context.Context, id string) (domain.IndexEntry, error) {
	if t, _ := tenant.TenantID(ctx); id == i.entry.DisplayID && t == i.entry.TenantID {
		return i.entry, nil
	}
	return domain.IndexEntry{}, domain.ErrArtifactNotFound(id)
}
func (indexStub) NextSolutionSeq(context.Context, string) (int, error) { return 1, nil }
func (indexStub) NextPlanSeq(context.Context, string) (int, error)     { return 1, nil }
func (indexStub) SetSolutionSeq(context.Context, string, int) error    { return nil }

type contentStub struct{ repo *tenantRepo }

func (c contentStub) UpdateContent(_ context.Context, r domain.Request, expected int64) (domain.Request, error) {
	cur := c.repo.rows[r.ID]
	if cur.Version != expected {
		return domain.Request{}, domain.ErrRequestVersionConflict(r.ID, expected)
	}
	r.Version = expected + 1
	c.repo.rows[r.ID] = r
	return r, nil
}

type solutionStub struct{ doc []byte }

func (s solutionStub) Insert(context.Context, domain.Solution) error { return nil }
func (s solutionStub) Get(_ context.Context, id string) (domain.Solution, error) {
	return domain.Solution{ID: id, OptionsJSON: s.doc, Status: domain.SolutionStatusProposed}, nil
}
func (s solutionStub) ListByRequestID(context.Context, string) ([]domain.Solution, error) {
	return nil, nil
}
func (s solutionStub) Update(context.Context, domain.Solution, int64) (domain.Solution, error) {
	return domain.Solution{}, nil
}

type noTx struct{}

func (noTx) InTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

type noOutbox struct{}

func (noOutbox) InsertOutboxEvent(context.Context, domain.OutboxEvent) error { return nil }

type artHarness struct {
	srv  *Server
	repo *tenantRepo
	req  domain.Request
	revs *revStub
}

func newArtHarness(t *testing.T) *artHarness {
	t.Helper()
	req, err := domain.NewRequest(domain.NewRequestInput{TenantID: artTenant, Title: "Lỗi đăng nhập", Body: "Đăng nhập thất bại với mật khẩu có dấu cách ở cuối.", SourceProvider: "manual", ReporterID: artUser})
	if err != nil {
		t.Fatal(err)
	}
	req.Number, req.Status, req.Type = 7, domain.RequestStatusAwaitingTypeConfirmation, domain.RequestTypeBug
	repo := &tenantRepo{rows: map[string]domain.Request{req.ID: req}}
	c, _ := domain.ContentFromRequest(req)
	snap, _ := c.Snapshot(nil)
	revs := &revStub{revs: []domain.RequestRevision{
		{RequestID: req.ID, Revision: 1, Cause: domain.RevisionCauseCreated, Snapshot: snap, Digest: req.ContentDigest, ActorKind: domain.RevisionActorUser, CreatedAt: time.Now()},
		{RequestID: req.ID, Revision: 2, Cause: domain.RevisionCauseEdited, Snapshot: snap, Digest: "d2", ActorKind: domain.RevisionActorUser, CreatedAt: time.Now()},
		{RequestID: req.ID, Revision: 3, Cause: domain.RevisionCauseClarificationAnswered, Snapshot: snap, Digest: "d3", ActorKind: domain.RevisionActorUser, CreatedAt: time.Now()},
	}}
	reg, err := schemas.Default()
	if err != nil {
		t.Fatal(err)
	}
	solID := uuid.NewString()
	idx := indexStub{entry: domain.IndexEntry{TenantID: artTenant, DisplayID: "SOL-7.1", Kind: domain.DisplayKindSolution, RequestID: req.ID, ArtifactID: solID}}
	doc := `{"schema_version":1,"options":[{"id":"opt-1","title":"A","summary":"s","approach":"a","effort":{"size":"M","hours_estimate":2},"risk":{"level":"low","description":"d"},"affected_areas":[]}],"recommendation":{"option_id":"opt-1","reason":"r"}}`
	appendRev := usecase.NewAppendRequestRevision(repo, contentStub{repo}, revs, noTx{}, noOutbox{})
	srv := NewServer(usecase.NewGetRequest(repo), usecase.NewListRequests(repo)).WithArtifact(ArtifactUseCases{
		Edit:   usecase.NewEditRequestContent(repo, appendRev, noTx{}),
		Reader: usecase.NewArtifactReader(repo, revs, &covStub{rows: []domain.CoverageRow{{ACID: "AC-1", TaskID: "t1", PlanTaskID: "p1"}}}, relStub{}, linkStub{}, idx),
		Export: usecase.NewExportArtifactProjection(repo, revs, solutionStub{doc: []byte(doc)}, idx, reg),
	})
	return &artHarness{srv: srv, repo: repo, req: req, revs: revs}
}

func wantCode(t *testing.T, err error, code codes.Code, what string) {
	t.Helper()
	if status.Code(err) != code {
		t.Errorf("%s: want %v, got %v (%v)", what, code, status.Code(err), err)
	}
}

func TestArtifactServer_Read_RequiresTenant(t *testing.T) {
	h := newArtHarness(t)
	bare := context.Background()
	id := h.req.ID
	_, err := h.srv.ListRequestRevisions(bare, &requestv1.ListRequestRevisionsRequest{RequestId: id})
	wantCode(t, err, codes.InvalidArgument, "ListRequestRevisions")
	_, err = h.srv.GetRequestRevision(bare, &requestv1.GetRequestRevisionRequest{RequestId: id, Revision: 1})
	wantCode(t, err, codes.InvalidArgument, "GetRequestRevision")
	_, err = h.srv.GetRequestCoverage(bare, &requestv1.GetRequestCoverageRequest{RequestId: id})
	wantCode(t, err, codes.InvalidArgument, "GetRequestCoverage")
	_, err = h.srv.GetArtifactGraph(bare, &requestv1.GetArtifactGraphRequest{RequestId: id})
	wantCode(t, err, codes.InvalidArgument, "GetArtifactGraph")
	_, err = h.srv.ExportArtifactProjection(bare, &requestv1.ExportArtifactProjectionRequest{RequestId: id})
	wantCode(t, err, codes.InvalidArgument, "ExportArtifactProjection")
	_, err = h.srv.ResolveArtifactRef(bare, &requestv1.ResolveArtifactRefRequest{Ref: "SOL-7.1"})
	wantCode(t, err, codes.InvalidArgument, "ResolveArtifactRef")
	_, err = h.srv.EditRequestContent(bare, &requestv1.EditRequestContentRequest{RequestId: id, ExpectedRevision: 1})
	wantCode(t, err, codes.InvalidArgument, "EditRequestContent")
}

func TestArtifactServer_CrossTenantNotFound(t *testing.T) {
	h := newArtHarness(t)
	other := tenant.WithUserID(tenant.WithTenantID(context.Background(), "99999999-9999-4999-8999-999999999999"), artUser)
	id := h.req.ID
	_, err := h.srv.ListRequestRevisions(other, &requestv1.ListRequestRevisionsRequest{RequestId: id})
	wantCode(t, err, codes.NotFound, "ListRequestRevisions")
	_, err = h.srv.GetRequestRevision(other, &requestv1.GetRequestRevisionRequest{RequestId: id, Revision: 1})
	wantCode(t, err, codes.NotFound, "GetRequestRevision")
	_, err = h.srv.GetRequestCoverage(other, &requestv1.GetRequestCoverageRequest{RequestId: id})
	wantCode(t, err, codes.NotFound, "GetRequestCoverage")
	_, err = h.srv.GetArtifactGraph(other, &requestv1.GetArtifactGraphRequest{RequestId: id})
	wantCode(t, err, codes.NotFound, "GetArtifactGraph")
	_, err = h.srv.ExportArtifactProjection(other, &requestv1.ExportArtifactProjectionRequest{RequestId: id})
	wantCode(t, err, codes.NotFound, "ExportArtifactProjection")
	_, err = h.srv.ResolveArtifactRef(other, &requestv1.ResolveArtifactRefRequest{Ref: "SOL-7.1"})
	wantCode(t, err, codes.NotFound, "ResolveArtifactRef")
	_, err = h.srv.EditRequestContent(other, &requestv1.EditRequestContentRequest{RequestId: id, ExpectedRevision: 1, Title: "x"})
	wantCode(t, err, codes.NotFound, "EditRequestContent")
	for _, bad := range []string{"not-a-uuid", uuid.NewString()} {
		_, err = h.srv.GetRequestCoverage(artCtx(artUser), &requestv1.GetRequestCoverageRequest{RequestId: bad})
		wantCode(t, err, codes.NotFound, "unknown id "+bad)
	}
}

func TestArtifactServer_ListRevisions_PaginatesAndOmitsSnapshot(t *testing.T) {
	h := newArtHarness(t)
	first, err := h.srv.ListRequestRevisions(artCtx(artUser), &requestv1.ListRequestRevisionsRequest{RequestId: h.req.ID, PageSize: 2})
	if err != nil || len(first.Revisions) != 2 || first.NextPageToken != "2" {
		t.Fatalf("%+v %v", first, err)
	}
	if first.Revisions[0].Cause != "created" || first.Revisions[1].Cause != "edited" || first.Revisions[0].ActorKind != "user" {
		t.Fatalf("%+v", first.Revisions)
	}
	second, err := h.srv.ListRequestRevisions(artCtx(artUser), &requestv1.ListRequestRevisionsRequest{RequestId: h.req.ID, PageSize: 2, PageToken: first.NextPageToken})
	if err != nil || len(second.Revisions) != 1 || second.NextPageToken != "" || second.Revisions[0].Cause != "clarified" {
		t.Fatalf("%+v %v", second, err)
	}
	if strings.Contains(first.String(), "Lỗi đăng nhập") {
		t.Fatal("the list carries no snapshot")
	}
	_, err = h.srv.ListRequestRevisions(artCtx(artUser), &requestv1.ListRequestRevisionsRequest{RequestId: h.req.ID, PageToken: "zz"})
	wantCode(t, err, codes.InvalidArgument, "bad token")
}

func TestArtifactServer_GetRevision_ReturnsSnapshot(t *testing.T) {
	h := newArtHarness(t)
	res, err := h.srv.GetRequestRevision(artCtx(artUser), &requestv1.GetRequestRevisionRequest{RequestId: h.req.ID, Revision: 1})
	if err != nil || res.Revision.Revision != 1 || !strings.Contains(res.SnapshotJson, "Lỗi đăng nhập") {
		t.Fatalf("%+v %v", res, err)
	}
	_, err = h.srv.GetRequestRevision(artCtx(artUser), &requestv1.GetRequestRevisionRequest{RequestId: h.req.ID, Revision: 9})
	wantCode(t, err, codes.NotFound, "missing revision")
}

func TestArtifactServer_CoverageGraphResolve(t *testing.T) {
	h := newArtHarness(t)
	cov, err := h.srv.GetRequestCoverage(artCtx(artUser), &requestv1.GetRequestCoverageRequest{RequestId: h.req.ID})
	if err != nil || len(cov.Rows) != 1 || cov.Rows[0].TaskId != "t1" {
		t.Fatalf("%+v %v", cov, err)
	}
	g, err := h.srv.GetArtifactGraph(artCtx(artUser), &requestv1.GetArtifactGraphRequest{RequestId: h.req.ID})
	if err != nil || len(g.Edges) != 1 || g.Edges[0].Rel != "derived_from" || g.Partial {
		t.Fatalf("%+v %v", g, err)
	}
	res, err := h.srv.ResolveArtifactRef(artCtx(artUser), &requestv1.ResolveArtifactRefRequest{Ref: "SOL-7.1"})
	if err != nil || res.Kind != "solution" || res.RequestId != h.req.ID {
		t.Fatalf("%+v %v", res, err)
	}
	ac, err := h.srv.ResolveArtifactRef(artCtx(artUser), &requestv1.ResolveArtifactRefRequest{Ref: "REQ-7"})
	if err != nil || ac.Kind != "request" || ac.ArtifactId != h.req.ID {
		t.Fatalf("%+v %v", ac, err)
	}
	_, err = h.srv.ResolveArtifactRef(artCtx(artUser), &requestv1.ResolveArtifactRefRequest{Ref: "sol-7.1"})
	wantCode(t, err, codes.InvalidArgument, "malformed ref")
}

func TestArtifactServer_Export_Markdown_FilenameAndDigest(t *testing.T) {
	h := newArtHarness(t)
	res, err := h.srv.ExportArtifactProjection(artCtx(artUser), &requestv1.ExportArtifactProjectionRequest{RequestId: h.req.ID, ArtifactRef: "SOL-7.1", Format: "markdown"})
	if err != nil || res.Filename != "SOL-7.1.md" || !strings.HasPrefix(res.Digest, "sha256:") || !strings.HasPrefix(res.Content, "---\n") || len(res.Content) > domain.MaxProjectionBytes {
		t.Fatalf("%+v %v", res, err)
	}
	own, err := h.srv.ExportArtifactProjection(artCtx(artUser), &requestv1.ExportArtifactProjectionRequest{RequestId: h.req.ID})
	if err != nil || own.Filename != "REQ-7.md" {
		t.Fatalf("%+v %v", own, err)
	}
}

func TestArtifactServer_Export_UnknownFormat_InvalidArgument(t *testing.T) {
	h := newArtHarness(t)
	_, err := h.srv.ExportArtifactProjection(artCtx(artUser), &requestv1.ExportArtifactProjectionRequest{RequestId: h.req.ID, ArtifactRef: "SOL-7.1", Format: "pdf"})
	wantCode(t, err, codes.InvalidArgument, "pdf")
}

func TestArtifactServer_EditContent_StaleRevision(t *testing.T) {
	h := newArtHarness(t)
	_, err := h.srv.EditRequestContent(artCtx(artUser), &requestv1.EditRequestContentRequest{RequestId: h.req.ID, ExpectedRevision: 5, Title: "mới"})
	wantCode(t, err, codes.FailedPrecondition, "stale revision")
	res, err := h.srv.EditRequestContent(artCtx(artUser), &requestv1.EditRequestContentRequest{
		RequestId: h.req.ID, ExpectedRevision: 1, Title: "Tiêu đề mới", AcceptanceCriteriaJson: `[{"text":"Đăng nhập được"}]`, TypeFieldsJson: `{"severity":"high"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Request.Title != "Tiêu đề mới" || res.Request.ContentRevision != 2 || !strings.Contains(res.Request.AcceptanceCriteriaJson, `"AC-1"`) || !strings.Contains(res.Request.TypeFieldsJson, `"severity":"high"`) {
		t.Fatalf("%+v", res.Request)
	}
	if _, err := h.srv.EditRequestContent(artCtx(artUser), &requestv1.EditRequestContentRequest{RequestId: h.req.ID, ExpectedRevision: 2, AcceptanceCriteriaJson: "{"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("bad JSON: %v", err)
	}
}

func TestArtifactServer_EditContent_NotEditableAfterAnalysis(t *testing.T) {
	h := newArtHarness(t)
	r := h.repo.rows[h.req.ID]
	r.Status = domain.RequestStatusAnalyzing
	h.repo.rows[r.ID] = r
	_, err := h.srv.EditRequestContent(artCtx(artUser), &requestv1.EditRequestContentRequest{RequestId: r.ID, ExpectedRevision: 1, Title: "x"})
	wantCode(t, err, codes.FailedPrecondition, "after analysis")
	if !strings.Contains(status.Convert(err).Message(), "REQUEST_CONTENT_NOT_EDITABLE") {
		t.Fatalf("%v", err)
	}
	_, err = h.srv.EditRequestContent(artCtx(artOther), &requestv1.EditRequestContentRequest{RequestId: r.ID, ExpectedRevision: 1, Title: "x"})
	wantCode(t, err, codes.PermissionDenied, "someone else")
}

func TestArtifactServer_UnwiredIsUnimplemented(t *testing.T) {
	srv := NewServer(nil, nil)
	_, err := srv.GetRequestCoverage(artCtx(artUser), &requestv1.GetRequestCoverageRequest{})
	wantCode(t, err, codes.Unimplemented, "unwired artifact")
	_, err = srv.AnswerClarification(artCtx(artUser), &requestv1.AnswerClarificationRequest{})
	wantCode(t, err, codes.Unimplemented, "unwired clarification")
	_, err = srv.ConfirmDecision(artCtx(artUser), &requestv1.ConfirmDecisionRequest{})
	wantCode(t, err, codes.Unimplemented, "unwired decision")
}

func TestRequestMapper_NewFields_RoundTrip(t *testing.T) {
	r, err := domain.NewRequest(domain.NewRequestInput{TenantID: artTenant, Title: "t", SourceProvider: "manual", ReporterID: artUser})
	if err != nil {
		t.Fatal(err)
	}
	c, _ := domain.ContentFromRequest(r)
	_, _ = c.AcceptanceCriteria.Add("một", "test")
	c.TypeFields["goal"] = "g"
	r, _ = r.WithContent(c)
	r.ContentRevision = 4
	// database JSON arrives with its own spacing; the wire form is canonical
	r.AcceptanceCriteriaJSON = []byte(`[ {"text": "một", "id": "AC-1", "status": "active", "verify_hint": "test"} ]`)
	out := toProtoRequest(r)
	if out.ContentRevision != 4 || out.ContentSchemaVersion != 1 || out.ContentDigest != r.ContentDigest || out.TypeFieldsJson != `{"ac_next":2,"goal":"g"}` {
		t.Fatalf("%+v", out)
	}
	if out.AcceptanceCriteriaJson != `[{"id":"AC-1","status":"active","text":"một","verify_hint":"test"}]` {
		t.Fatalf("not canonical: %s", out.AcceptanceCriteriaJson)
	}
	var probe []map[string]any
	if err := json.Unmarshal([]byte(out.AcceptanceCriteriaJson), &probe); err != nil {
		t.Fatal(err)
	}
}

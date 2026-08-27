package httpgateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"

	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	annotationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/annotation/v1"
	scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"
)

// pagingAnnotationServiceClient is a func-based fake — unlike
// fakeAnnotationServiceClient (annotation_routes_test.go)'s single static
// response, this one can return a different page per call, needed to
// exercise SubmitPullRequestReview's drain-all-pages loop.
type pagingAnnotationServiceClient struct {
	annotationv1.AnnotationServiceClient

	listAnnotationsFunc func(ctx context.Context, in *annotationv1.ListAnnotationsRequest) (*annotationv1.ListAnnotationsResponse, error)
	calls               int
}

func (f *pagingAnnotationServiceClient) ListAnnotations(ctx context.Context, in *annotationv1.ListAnnotationsRequest, _ ...grpc.CallOption) (*annotationv1.ListAnnotationsResponse, error) {
	f.calls++
	return f.listAnnotationsFunc(ctx, in)
}

func prReviewTestRouter(scmClient scmintegrationv1.ScmIntegrationServiceClient, annotationClient annotationv1.AnnotationServiceClient) chi.Router {
	r := chi.NewRouter()
	mountPRReviewRoutes(r, scmClient, annotationClient)
	return r
}

func TestSubmitPullRequestReview_ZeroAnnotations_400WithoutCallingSCMClient(t *testing.T) {
	scm := &fakeScmIntegrationServiceClient{submitReviewResp: &scmintegrationv1.Review{Id: "should-not-happen"}}
	annotations := &pagingAnnotationServiceClient{
		listAnnotationsFunc: func(_ context.Context, _ *annotationv1.ListAnnotationsRequest) (*annotationv1.ListAnnotationsResponse, error) {
			return &annotationv1.ListAnnotationsResponse{}, nil
		},
	}
	router := prReviewTestRouter(scm, annotations)

	body, _ := json.Marshal(submitReviewRequestBody{RepoID: "repo-1", Provider: "github", ReviewType: "approve"})
	req := httptest.NewRequest(http.MethodPost, "/v1/scm/pull-requests/42/reviews", strings.NewReader(string(body)))
	req = withTestIdentity(req, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if scm.submitReviewReq != nil {
		t.Fatal("expected SubmitReview to never be called for a zero-annotation repo")
	}
}

// TestSubmitPullRequestReview_MultiPageAnnotationsFullyDrained is the
// regression guard against silently submitting a partial comment set:
// every page's annotations must appear in the final SubmitReview call.
func TestSubmitPullRequestReview_MultiPageAnnotationsFullyDrained(t *testing.T) {
	scm := &fakeScmIntegrationServiceClient{submitReviewResp: &scmintegrationv1.Review{Id: "review-1"}}
	annotations := &pagingAnnotationServiceClient{
		listAnnotationsFunc: func(_ context.Context, in *annotationv1.ListAnnotationsRequest) (*annotationv1.ListAnnotationsResponse, error) {
			switch in.GetPageToken() {
			case "":
				return &annotationv1.ListAnnotationsResponse{
					Annotations: []*annotationv1.Annotation{
						{Anchor: &annotationv1.Anchor{FilePath: "a.go", Line: 1}, Content: "first"},
					},
					NextPageToken: "page-2",
				}, nil
			case "page-2":
				return &annotationv1.ListAnnotationsResponse{
					Annotations: []*annotationv1.Annotation{
						{Anchor: &annotationv1.Anchor{FilePath: "b.go", Line: 2}, Content: "second"},
					},
					NextPageToken: "page-3",
				}, nil
			case "page-3":
				return &annotationv1.ListAnnotationsResponse{
					Annotations: []*annotationv1.Annotation{
						{Anchor: &annotationv1.Anchor{FilePath: "c.go", Line: 3}, Content: "third"},
					},
				}, nil
			default:
				t.Fatalf("unexpected page token: %q", in.GetPageToken())
				return nil, nil
			}
		},
	}
	router := prReviewTestRouter(scm, annotations)

	body, _ := json.Marshal(submitReviewRequestBody{RepoID: "repo-1", Provider: "github", ReviewType: "comment", Summary: "lgtm"})
	req := httptest.NewRequest(http.MethodPost, "/v1/scm/pull-requests/42/reviews", strings.NewReader(string(body)))
	req = withTestIdentity(req, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if annotations.calls != 3 {
		t.Fatalf("expected exactly 3 ListAnnotations calls to drain all pages, got %d", annotations.calls)
	}
	if scm.submitReviewReq == nil {
		t.Fatal("SubmitReview was never called")
	}
	comments := scm.submitReviewReq.GetComments()
	if len(comments) != 3 {
		t.Fatalf("expected all 3 pages' annotations in the final SubmitReview call, got %d", len(comments))
	}
	wantPaths := map[string]bool{"a.go": false, "b.go": false, "c.go": false}
	for _, c := range comments {
		if _, ok := wantPaths[c.GetPath()]; !ok {
			t.Fatalf("unexpected comment path %q", c.GetPath())
		}
		wantPaths[c.GetPath()] = true
	}
	for path, seen := range wantPaths {
		if !seen {
			t.Fatalf("expected a comment for %q, none found — a page was silently dropped", path)
		}
	}
	if scm.submitReviewReq.GetPrNumber() != 42 {
		t.Fatalf("expected prNumber=42 from the path, got %d", scm.submitReviewReq.GetPrNumber())
	}
	if scm.submitReviewReq.GetTenantId() != "tenant-1" {
		t.Fatalf("expected tenant_id from identity, got %q", scm.submitReviewReq.GetTenantId())
	}
}

// TestReviewComment_ContractAlignsWithAnnotationAnchor is a loud-break
// contract test: ReviewComment.path/line/body must stay aligned with
// Anchor.file_path/line + Annotation.content — this test's own mapping
// (identical to handleSubmitPullRequestReview's) would fail to compile if
// either proto renamed a field out from under the other.
func TestReviewComment_ContractAlignsWithAnnotationAnchor(t *testing.T) {
	a := &annotationv1.Annotation{
		Anchor:  &annotationv1.Anchor{FilePath: "contract.go", Line: 7},
		Content: "contract body",
	}
	comment := &scmintegrationv1.ReviewComment{
		Path: a.GetAnchor().GetFilePath(), Line: a.GetAnchor().GetLine(), Body: a.GetContent(),
	}
	if comment.GetPath() != "contract.go" || comment.GetLine() != 7 || comment.GetBody() != "contract body" {
		t.Fatalf("ReviewComment mapping drifted from Annotation/Anchor: %+v", comment)
	}
}

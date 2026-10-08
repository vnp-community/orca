package grpc

import (
	"context"
	"testing"
	"time"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestMapper_SourceHintsAndLifecycleFieldsRoundTrip(t *testing.T) {
	r := domain.Request{
		ID: "r", SourceHints: domain.SourceHints{IssueType: "Bug", Labels: []string{"a", "ệ"}, Priority: "High", TypeHint: "bug"},
		ReturnedCategory: domain.ReturnCategoryMissingInfo, ClassificationAttempts: 3, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	p := toProtoRequest(r)
	h := p.GetSourceHints()
	if h.GetIssueType() != "Bug" || len(h.GetLabels()) != 2 || h.GetPriority() != "High" || h.GetTypeHint() != "bug" || p.GetReturnedCategory() != "missing_info" || p.GetClassificationAttempts() != 3 {
		t.Fatalf("%+v", p)
	}
	if toProtoRequest(domain.Request{CreatedAt: time.Now(), UpdatedAt: time.Now()}).SourceHints != nil {
		t.Fatal("empty hints must stay unset on the wire")
	}
}

func TestFilterFromProto_SourceFields(t *testing.T) {
	f, err := filterFromProto(&requestv1.ListRequestsRequest{SourceProvider: "jira", SourceSite: "s", SourceRef: "ENG-1"})
	if err != nil || f.SourceProvider != "jira" || f.SourceSite != "s" || f.SourceRef != "ENG-1" {
		t.Fatalf("%+v %v", f, err)
	}
}

func TestToProtoTypeChange_AgentIsAI(t *testing.T) {
	if got := toProtoTypeChange(domain.RequestTypeChange{ActorKind: domain.ActorKindAgent, At: time.Now()}).GetActorKind(); got != "ai" {
		t.Fatalf("actor_kind = %q", got)
	}
	if got := toProtoTypeChange(domain.RequestTypeChange{ActorKind: domain.ActorKindUser, At: time.Now()}).GetActorKind(); got != "user" {
		t.Fatalf("actor_kind = %q", got)
	}
}

func TestServer_UnwiredRPCsStayUnimplemented(t *testing.T) {
	s := NewServer(nil, nil)
	ctx := context.Background()
	checks := map[string]error{}
	_, checks["create"] = s.CreateRequest(ctx, &requestv1.CreateRequestRequest{})
	_, checks["classify"] = s.ClassifyRequest(ctx, &requestv1.ClassifyRequestRequest{})
	_, checks["confirm"] = s.ConfirmRequestType(ctx, &requestv1.ConfirmRequestTypeRequest{})
	_, checks["flow"] = s.GetRequestFlow(ctx, &requestv1.GetRequestFlowRequest{})
	_, checks["return"] = s.ReturnToBacklog(ctx, &requestv1.ReturnToBacklogRequest{})
	_, checks["links"] = s.ListRequestLinks(ctx, &requestv1.ListRequestLinksRequest{})
	for name, err := range checks {
		if status.Code(err) != codes.Unimplemented {
			t.Errorf("%s: %v", name, err)
		}
	}
}

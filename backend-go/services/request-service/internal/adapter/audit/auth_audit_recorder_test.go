package audit

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/common/auditclient"
	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type fakeAuth struct {
	authv1.AuthServiceClient
	got []*authv1.AppendAuditEntryRequest
	err error
}

func (f *fakeAuth) AppendAuditEntry(_ context.Context, in *authv1.AppendAuditEntryRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	f.got = append(f.got, in)
	return &emptypb.Empty{}, f.err
}

func recorderWith(f *fakeAuth) *AuthAuditRecorder { return NewAuthAuditRecorder(auditclient.New(f)) }

func TestAuthAuditRecorder_SendsEveryFieldAndFiltersMetadata(t *testing.T) {
	f := &fakeAuth{}
	recorderWith(f).Record(context.Background(), usecase.RPCAuditEvent{
		TenantID: "t-1", ActorID: "u-1", ActorKind: domain.AuditActorAgent, Action: domain.ActionRequestCreate,
		TargetType: "request", TargetID: "r-1", Outcome: domain.AuditOutcomeAllowed,
		Metadata: map[string]string{"source_provider": "mcp", "title": "TITLE-MARKER", "body": "BODY-MARKER", "client_name": "claude"},
	})
	if len(f.got) != 1 {
		t.Fatalf("%d calls", len(f.got))
	}
	g := f.got[0]
	if g.GetTenantId() != "t-1" || g.GetActorId() != "u-1" || g.GetActorType() != "agent" || g.GetAction() != "request.create" ||
		g.GetTarget() != "request:r-1" || g.GetTargetType() != "request" || g.GetTargetId() != "r-1" || g.GetOutcome() != "allowed" {
		t.Fatalf("fields: %+v", g)
	}
	var md map[string]string
	if err := json.Unmarshal([]byte(g.GetMetadataJson()), &md); err != nil {
		t.Fatal(err)
	}
	if len(md) != 2 || md["source_provider"] != "mcp" || md["client_name"] != "claude" {
		t.Fatalf("metadata %v", md)
	}
	if strings.Contains(g.GetMetadataJson(), "MARKER") {
		t.Fatalf("title or body leaked: %s", g.GetMetadataJson())
	}
}

func TestAuthAuditRecorder_NoAllowedKeysMeansNoMetadata(t *testing.T) {
	f := &fakeAuth{}
	recorderWith(f).Record(context.Background(), usecase.RPCAuditEvent{TenantID: "t", Action: "request.cancel", TargetType: "request", TargetID: "r",
		Metadata: map[string]string{"reason": "REASON-MARKER"}})
	if f.got[0].GetMetadataJson() != "" {
		t.Fatalf("metadata %q, want none", f.got[0].GetMetadataJson())
	}
}

func TestAuthAuditRecorder_AuthServiceFailureNeverSurfaces(t *testing.T) {
	f := &fakeAuth{err: errors.New("auth-service unreachable")}
	// Record has no error to return and must not panic; the decision it describes already committed.
	recorderWith(f).Record(context.Background(), usecase.RPCAuditEvent{TenantID: "t", Action: "request.cancel", TargetType: "request", TargetID: "r"})
	if len(f.got) != 1 {
		t.Fatal("the append was not even attempted")
	}
}

func TestAuthAuditRecorder_EveryRecordedActionUsesAValidActorType(t *testing.T) {
	for _, k := range []domain.AuditActorKind{domain.AuditActorUser, domain.AuditActorAgent, domain.AuditActorSystem} {
		f := &fakeAuth{}
		recorderWith(f).Record(context.Background(), usecase.RPCAuditEvent{TenantID: "t", ActorKind: k, Action: "request.cancel", TargetType: "request", TargetID: "r"})
		if f.got[0].GetActorType() != string(k) {
			t.Errorf("actor type %q became %q", k, f.got[0].GetActorType())
		}
	}
}

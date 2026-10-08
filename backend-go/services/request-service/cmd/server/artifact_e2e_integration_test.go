//go:build integration

package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/internalcaller"
	"github.com/stablyai/orca-go/common/testutil"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type rpcCaller struct {
	client requestv1.RequestServiceClient
	tenant string
}

func (c rpcCaller) as(user string, role string) context.Context {
	md := metadata.Pairs(grpcmw.MetadataTenantID, c.tenant, grpcmw.MetadataUserID, user)
	if role != "" {
		md.Set(grpcmw.MetadataRole, role)
	}
	return metadata.NewOutgoingContext(context.Background(), md)
}

func mustCode(t *testing.T, err error, want codes.Code, what string) {
	t.Helper()
	if status.Code(err) != want {
		t.Fatalf("%s: want %v, got %v", what, want, err)
	}
}

func waitForStatus(t *testing.T, c rpcCaller, user, id, want string) *requestv1.Request {
	t.Helper()
	deadline := time.Now().Add(40 * time.Second)
	for {
		res, err := c.client.GetRequest(c.as(user, ""), &requestv1.GetRequestRequest{Id: id})
		if err != nil {
			t.Fatal(err)
		}
		if res.Request.Status == want {
			return res.Request
		}
		if time.Now().After(deadline) {
			t.Fatalf("request stayed %s, want %s", res.Request.Status, want)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// The whole artifact and clarification path through the real service: gRPC, wiring, Postgres with RLS, the
// outbox relay and the NATS consumers that move a new request to type confirmation.
func TestRun_ArtifactAndClarificationFlow_Postgres(t *testing.T) {
	runArtifactAndClarificationFlow(t, startMigratedPostgresDSN(t))
}

// Same flow on MySQL: tenant filtering in every WHERE instead of RLS, JSON columns and generated keys.
func TestRun_ArtifactAndClarificationFlow_MySQL(t *testing.T) {
	runArtifactAndClarificationFlow(t, startMigratedMySQLDSN(t))
}

func runArtifactAndClarificationFlow(t *testing.T, dsn string) {
	natsURL := testutil.StartNATS(t)
	cfg := testConfig(t, dsn)
	cfg.NATSURL, cfg.RequestFlowEnabled = natsURL, true
	var members *fakeProjectMembers
	cfg.ProjectServiceAddr, members = startFakeProjectMembers(t)

	assertServiceStartsAndServes(t, cfg, func() {
		conn, err := grpc.NewClient("127.0.0.1:"+itoa(cfg.GRPCPort), grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithUnaryInterceptor(internalcaller.ClientInterceptor(testGatewayToken)))
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		c := rpcCaller{client: requestv1.NewRequestServiceClient(conn), tenant: uuid.NewString()}
		reporter, stranger := uuid.NewString(), uuid.NewString()
		members.own(reporter)
		ctx := c.as(reporter, "")
		// The gate is per tenant (CR-REQ-025): the global switch alone leaves a new tenant disabled.
		if _, err := c.client.SetRequestFlowSettings(c.as(uuid.NewString(), "admin"), &requestv1.SetRequestFlowSettingsRequest{Enabled: true}); err != nil {
			t.Fatal(err)
		}

		created, err := c.client.CreateRequest(ctx, &requestv1.CreateRequestRequest{
			ProjectId: uuid.NewString(), Title: "Đăng nhập chậm", Body: "Đăng nhập mất hơn mười giây khi mạng yếu và máy chủ bận.",
			Source: &requestv1.RequestSource{Provider: "manual"}, AcceptanceCriteriaJson: `[{"text":"Đăng nhập dưới 3 giây","verify_hint":"metric"}]`,
		})
		if err != nil {
			t.Fatal(err)
		}
		r := created.Request
		if r.ContentRevision != 1 || !strings.Contains(r.AcceptanceCriteriaJson, `"AC-1"`) || len(r.ContentDigest) != 64 {
			t.Fatalf("created content: %+v", r)
		}
		// no AI is configured, so classification falls back to waiting for a person
		waitForStatus(t, c, reporter, r.Id, "awaiting_type_confirmation")

		// edit twice; revisions list in order, the first snapshot keeps the original title
		edited, err := c.client.EditRequestContent(ctx, &requestv1.EditRequestContentRequest{RequestId: r.Id, ExpectedRevision: 1, Title: "Đăng nhập chậm (đã sửa)"})
		if err != nil || edited.Request.ContentRevision != 2 || edited.Request.Title != "Đăng nhập chậm (đã sửa)" {
			t.Fatalf("%+v %v", edited, err)
		}
		_, err = c.client.EditRequestContent(ctx, &requestv1.EditRequestContentRequest{RequestId: r.Id, ExpectedRevision: 1, Title: "x"})
		mustCode(t, err, codes.FailedPrecondition, "stale edit")
		_, err = c.client.EditRequestContent(c.as(stranger, ""), &requestv1.EditRequestContentRequest{RequestId: r.Id, ExpectedRevision: 2, Title: "x"})
		mustCode(t, err, codes.PermissionDenied, "edit by a stranger")
		revs, err := c.client.ListRequestRevisions(ctx, &requestv1.ListRequestRevisionsRequest{RequestId: r.Id})
		if err != nil || len(revs.Revisions) != 2 || revs.Revisions[0].Cause != "created" || revs.Revisions[1].Cause != "edited" {
			t.Fatalf("%+v %v", revs, err)
		}
		first, err := c.client.GetRequestRevision(ctx, &requestv1.GetRequestRevisionRequest{RequestId: r.Id, Revision: 1})
		if err != nil || !strings.Contains(first.SnapshotJson, "Đăng nhập chậm") || strings.Contains(first.SnapshotJson, "đã sửa") {
			t.Fatalf("%+v %v", first, err)
		}
		ref, err := c.client.ResolveArtifactRef(ctx, &requestv1.ResolveArtifactRefRequest{Ref: "REQ-" + itoa(int(r.Number))})
		if err != nil || ref.ArtifactId != r.Id || ref.Kind != "request" {
			t.Fatalf("%+v %v", ref, err)
		}
		exp, err := c.client.ExportArtifactProjection(ctx, &requestv1.ExportArtifactProjectionRequest{RequestId: r.Id})
		if err != nil || !strings.HasPrefix(exp.Content, "---\n") || !strings.Contains(exp.Content, "Đăng nhập chậm (đã sửa)") {
			t.Fatalf("%+v %v", exp, err)
		}
		cov, err := c.client.GetRequestCoverage(ctx, &requestv1.GetRequestCoverageRequest{RequestId: r.Id})
		if err != nil || len(cov.Rows) != 0 || len(cov.UncoveredAcIds) != 1 || cov.UncoveredAcIds[0] != "AC-1" {
			t.Fatalf("%+v %v", cov, err)
		}
		graph, err := c.client.GetArtifactGraph(ctx, &requestv1.GetArtifactGraphRequest{RequestId: r.Id})
		if err != nil || graph.Partial {
			t.Fatalf("%+v %v", graph, err)
		}

		// another tenant sees none of it
		other := rpcCaller{client: c.client, tenant: uuid.NewString()}
		_, err = c.client.GetRequestRevision(other.as(reporter, ""), &requestv1.GetRequestRevisionRequest{RequestId: r.Id, Revision: 1})
		mustCode(t, err, codes.NotFound, "cross tenant revision")
		_, err = c.client.ResolveArtifactRef(other.as(reporter, ""), &requestv1.ResolveArtifactRefRequest{Ref: "REQ-" + itoa(int(r.Number))})
		mustCode(t, err, codes.NotFound, "cross tenant resolve")

		// confirm the type: the bug is missing most of its facts, so the request waits for information
		confirmed, err := c.client.ConfirmRequestType(ctx, &requestv1.ConfirmRequestTypeRequest{RequestId: r.Id, Type: "bug", Size: "M"})
		if err != nil || confirmed.Request.Status != "awaiting_information" {
			t.Fatalf("%+v %v", confirmed, err)
		}
		ready, err := c.client.GetRequestReadiness(ctx, &requestv1.GetRequestReadinessRequest{RequestId: r.Id})
		if err != nil || ready.Ready || len(ready.Missing) < 5 {
			t.Fatalf("%+v %v", ready, err)
		}
		pending, err := c.client.ListPendingClarificationsForUser(ctx, &requestv1.ListPendingClarificationsForUserRequest{})
		if err != nil || len(pending.Clarifications) != 1 {
			t.Fatalf("%+v %v", pending, err)
		}
		clar := pending.Clarifications[0]
		if clar.Source != "readiness" || clar.ResumeStatus != "analyzing" || clar.Round != 1 || !strings.HasPrefix(clar.DisplayId, "CLR-") {
			t.Fatalf("%+v", clar)
		}
		none, _ := c.client.ListPendingClarificationsForUser(c.as(stranger, ""), &requestv1.ListPendingClarificationsForUserRequest{})
		if len(none.Clarifications) != 0 {
			t.Fatal("a stranger has nothing pending")
		}
		// Authorization (CR-REQ-035) turns a non-member away; a project member who is not an assignee sees no answers.
		_, err = c.client.GetClarification(c.as(stranger, ""), &requestv1.GetClarificationRequest{Id: clar.Id})
		mustCode(t, err, codes.PermissionDenied, "clarification read by a stranger")
		colleague := uuid.NewString()
		members.member(colleague)
		seen, err := c.client.GetClarification(c.as(colleague, ""), &requestv1.GetClarificationRequest{Id: clar.Id})
		if err != nil {
			t.Fatal(err)
		}
		for _, q := range seen.Clarification.Questions {
			if q.AnswerJson != "" {
				t.Fatal("answers must be hidden from a stranger")
			}
		}
		_, err = c.client.GetClarification(other.as(reporter, ""), &requestv1.GetClarificationRequest{Id: clar.Id})
		mustCode(t, err, codes.NotFound, "cross tenant clarification")

		values := map[string]string{
			"type_fields.repro_steps": `"mở trang đăng nhập\nnhập tài khoản"`, "type_fields.actual": `"mất mười giây"`, "type_fields.expected": `"dưới ba giây"`,
			"type_fields.environment": `"production"`, "type_fields.severity": `"high"`, "acceptance_criteria": `"Đăng nhập nhanh trên mạng yếu"`, "body": `"Chi tiết bổ sung về mạng yếu."`,
		}
		var items []*requestv1.AnswerItem
		for _, q := range clar.Questions {
			items = append(items, &requestv1.AnswerItem{QuestionId: q.Id, ValueJson: values[q.QuestionKey]})
		}
		_, err = c.client.AnswerClarification(c.as(stranger, ""), &requestv1.AnswerClarificationRequest{ClarificationId: clar.Id, Answers: items, Complete: true})
		mustCode(t, err, codes.PermissionDenied, "answer by a stranger")
		draft, err := c.client.AnswerClarification(ctx, &requestv1.AnswerClarificationRequest{ClarificationId: clar.Id, Answers: items[:2]})
		if err != nil || !draft.StillMissing || draft.RequestStatus != "awaiting_information" {
			t.Fatalf("%+v %v", draft, err)
		}
		_, err = c.client.AnswerClarification(ctx, &requestv1.AnswerClarificationRequest{ClarificationId: clar.Id, Answers: items[:2], Complete: true})
		mustCode(t, err, codes.FailedPrecondition, "incomplete answer")
		done, err := c.client.AnswerClarification(ctx, &requestv1.AnswerClarificationRequest{ClarificationId: clar.Id, Answers: items, Complete: true})
		if err != nil || done.RequestStatus != "analyzing" || done.RequestRevision != 3 || done.StillMissing || done.Clarification.Status != requestv1.ClarificationStatus_CLARIFICATION_STATUS_ANSWERED {
			t.Fatalf("%+v %v", done, err)
		}
		again, err := c.client.AnswerClarification(ctx, &requestv1.AnswerClarificationRequest{ClarificationId: clar.Id, Answers: items, Complete: true})
		if err != nil || again.RequestRevision != 3 {
			t.Fatalf("redelivered answer: %+v %v", again, err)
		}
		final := waitForStatus(t, c, reporter, r.Id, "analyzing")
		if final.ContentRevision != 3 || !strings.Contains(final.TypeFieldsJson, `"severity":"high"`) {
			t.Fatalf("%+v", final)
		}
		var acs []map[string]any
		if err := json.Unmarshal([]byte(final.AcceptanceCriteriaJson), &acs); err != nil || len(acs) != 1 || acs[0]["id"] != "AC-1" {
			t.Fatalf("the creation criterion stays AC-1 and no readiness question asked for another: %s", final.AcceptanceCriteriaJson)
		}
		answered, err := c.client.ListClarifications(ctx, &requestv1.ListClarificationsRequest{RequestId: r.Id, Status: requestv1.ClarificationStatus_CLARIFICATION_STATUS_ANSWERED})
		if err != nil || len(answered.Clarifications) != 1 {
			t.Fatalf("%+v %v", answered, err)
		}
		decisions, err := c.client.ListDecisions(ctx, &requestv1.ListDecisionsRequest{RequestId: r.Id})
		if err != nil || len(decisions.Decisions) != 0 {
			t.Fatalf("%+v %v", decisions, err)
		}
		_, err = c.client.ConfirmDecision(ctx, &requestv1.ConfirmDecisionRequest{DecisionId: uuid.NewString(), ConfirmationText: "x"})
		mustCode(t, err, codes.NotFound, "unknown decision")
		// the person who may not waive: readiness waiver is for admins
		_, err = c.client.WaiveReadiness(ctx, &requestv1.WaiveReadinessRequest{RequestId: r.Id, Reason: "x"})
		mustCode(t, err, codes.PermissionDenied, "waive by a non admin")
	})
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

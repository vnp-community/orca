package wscompat

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// planAutoClient lets a test choose the auto-plan block the service returns.
type planAutoClient struct {
	*fakeRequestClient
	auto     *requestv1.PlanAutoState
	retryErr error
	noRun    bool
}

func (c *planAutoClient) RetryPlanGeneration(ctx context.Context, in *requestv1.RetryPlanGenerationRequest, _ ...grpc.CallOption) (*requestv1.RetryPlanGenerationResponse, error) {
	c.rec(ctx, "RetryPlanGeneration", in)
	if c.retryErr != nil {
		return nil, c.retryErr
	}
	return &requestv1.RetryPlanGenerationResponse{AutoPlan: c.auto}, nil
}

func (c *planAutoClient) GetPlanProposal(ctx context.Context, in *requestv1.GetPlanProposalRequest, _ ...grpc.CallOption) (*requestv1.GetPlanProposalResponse, error) {
	c.rec(ctx, "GetPlanProposal", in)
	if c.noRun {
		return &requestv1.GetPlanProposalResponse{AutoPlan: c.auto}, nil
	}
	return &requestv1.GetPlanProposalResponse{RunId: "run-1", Status: "failed", ErrorCode: "REQUEST_PLAN_AI_INVALID_JSON", AutoPlan: c.auto}, nil
}

var gaveUpAuto = &requestv1.PlanAutoState{Enabled: true, State: "gave_up", Origin: "event", Attempt: 2, MaxAttempts: 2, ReasonCode: "REQUEST_PLAN_AI_INVALID_JSON", NextAttemptAt: "2026-10-20T10:30:00Z"}

func TestRequestRetryPlan_ForwardsAndReturnsTheAutoPlanBlock(t *testing.T) {
	c := &planAutoClient{fakeRequestClient: &fakeRequestClient{}, auto: gaveUpAuto}
	reg := NewRegistry()
	registerRequestChannels(reg, c, nil, nil, false)
	res, err := callRequestChannel(t, reg, testRequestIdentity, context.Background(), "request.retryPlan", `{"id":"r1"}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := c.last().in.(*requestv1.RetryPlanGenerationRequest).GetRequestId(); got != "r1" {
		t.Errorf("request id sent = %q", got)
	}
	a := res["autoPlan"].(map[string]any)
	mustEqual(t, a["enabled"], true)
	mustEqual(t, a["state"], "gave_up")
	mustEqual(t, a["origin"], "event")
	mustEqual(t, a["attempt"], float64(2))
	mustEqual(t, a["maxAttempts"], float64(2))
	mustEqual(t, a["reasonCode"], "REQUEST_PLAN_AI_INVALID_JSON")
	mustEqual(t, a["nextAttemptAt"], "2026-10-20T10:30:00Z")
}

func TestRequestRetryPlan_ErrorsKeepTheirCodes(t *testing.T) {
	for code, st := range map[string]codes.Code{"REQUEST_PLAN_AUTO_DISABLED": codes.FailedPrecondition, "REQUEST_STATE_STALE": codes.FailedPrecondition} {
		c := &planAutoClient{fakeRequestClient: &fakeRequestClient{}, retryErr: status.Error(st, code+": nope")}
		reg := NewRegistry()
		registerRequestChannels(reg, c, nil, nil, false)
		_, err := callRequestChannel(t, reg, testRequestIdentity, context.Background(), "request.retryPlan", `{"id":"r1"}`)
		if err == nil || !strings.Contains(err.Error(), code) {
			t.Errorf("%s: err = %v", code, err)
		}
	}
}

func TestRequestPlanProposal_CarriesAutoPlanOnlyWhenTheServiceSendsIt(t *testing.T) {
	c := &planAutoClient{fakeRequestClient: &fakeRequestClient{}, auto: gaveUpAuto}
	reg := NewRegistry()
	registerRequestChannels(reg, c, nil, nil, false)
	res, err := callRequestChannel(t, reg, testRequestIdentity, context.Background(), "request.planProposal", `{"id":"r1"}`)
	if err != nil {
		t.Fatal(err)
	}
	mustEqual(t, res["runId"], "run-1")
	mustEqual(t, res["errorCode"], "REQUEST_PLAN_AI_INVALID_JSON")
	mustEqual(t, res["autoPlan"].(map[string]any)["state"], "gave_up")

	// No run yet: the view stays empty but still says whether Orca would plan by itself.
	c.noRun, c.auto = true, &requestv1.PlanAutoState{Enabled: true}
	res, err = callRequestChannel(t, reg, testRequestIdentity, context.Background(), "request.planProposal", `{"id":"r1"}`)
	if err != nil {
		t.Fatal(err)
	}
	mustEqual(t, res["runId"], "")
	if res["proposal"] != nil {
		t.Errorf("proposal = %v, want null", res["proposal"])
	}
	a := res["autoPlan"].(map[string]any)
	mustEqual(t, a["enabled"], true)
	mustEqual(t, a["state"], "")

	// An older service sends no block: the key is absent, not an object saying "off".
	res, err = callRequestChannel(t, newRequestTestRegistry(&fakeRequestClient{}), testRequestIdentity, context.Background(), "request.planProposal", `{"id":"r1"}`)
	if err != nil {
		t.Fatal(err)
	}
	if _, present := res["autoPlan"]; present {
		t.Errorf("autoPlan present for a service without it: %v", res["autoPlan"])
	}
}

func TestRequestEngineSettingsSet_PlanAutoGenerateIsOptionalAndTyped(t *testing.T) {
	admin := Identity{TenantID: "t1", UserID: "u1", Role: roleAdmin}
	c := &fakeRequestClient{}
	reg := newRequestTestRegistry(c)

	if _, err := callRequestChannel(t, reg, admin, context.Background(), "request.engineSettingsSet", `{"projectId":"p1","engine":"native","expectedVersion":1}`); err != nil {
		t.Fatal(err)
	}
	if got := c.last().in.(*requestv1.SetProjectEngineSettingsRequest); got.PlanAutoGenerate != nil {
		t.Errorf("a missing key must keep the stored value, sent %v", *got.PlanAutoGenerate)
	}
	if _, err := callRequestChannel(t, reg, admin, context.Background(), "request.engineSettingsSet", `{"projectId":"p1","engine":"native","planAutoGenerate":false}`); err != nil {
		t.Fatal(err)
	}
	got := c.last().in.(*requestv1.SetProjectEngineSettingsRequest)
	if got.PlanAutoGenerate == nil || *got.PlanAutoGenerate {
		t.Errorf("explicit false was not forwarded: %v", got.PlanAutoGenerate)
	}
	if _, err := callRequestChannel(t, reg, admin, context.Background(), "request.engineSettingsSet", `{"projectId":"p1","engine":"native","planAutoGenerate":"no"}`); err == nil {
		t.Error("a non-boolean planAutoGenerate must be refused")
	}
}

func TestRequestEngineSettings_ViewCarriesPlanAutoGenerate(t *testing.T) {
	v := engineSettingsView(&requestv1.ProjectEngineSettings{ProjectId: "p1", PlanAutoGenerate: true}, nil)
	if !v.PlanAutoGenerate {
		t.Fatalf("view = %+v", v)
	}
}

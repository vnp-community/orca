//go:build e2e

package e2e

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/internalcaller"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

func requireFlowDisabled(t *testing.T, what string, err error) {
	t.Helper()
	if code(err) != codes.FailedPrecondition || !strings.Contains(errText(err), "REQUEST_FLOW_DISABLED") {
		t.Fatalf("%s: want FailedPrecondition REQUEST_FLOW_DISABLED, got %v", what, err)
	}
}

func requireNotGated(t *testing.T, what string, err error) {
	t.Helper()
	if strings.Contains(errText(err), "REQUEST_FLOW_DISABLED") {
		t.Fatalf("%s must keep working while the flow is off, got %v", what, err)
	}
}

func createRaw(k *kit, ctx func() (*requestv1.CreateRequestResponse, error)) error {
	_, err := ctx()
	return err
}

func tryCreate(k *kit) error {
	return createRaw(k, func() (*requestv1.CreateRequestResponse, error) {
		return k.req.CreateRequest(k.asReporter(), &requestv1.CreateRequestRequest{ProjectId: k.project, Title: "flag probe",
			Source: &requestv1.RequestSource{Provider: "manual"}, ClientRequestId: uuid.NewString()})
	})
}

// E20: off by default, on per tenant, off again in the middle of a Request; reads and exits keep working,
// the advancing RPCs do not, and one tenant never affects another.
func TestE20_FlagLifecycle(t *testing.T) {
	k := newKit(t)

	// Default: the tenant has no row, so the flow is off although the global switch is on.
	got, err := k.req.GetRequestFlowSettings(k.asReporter(), &requestv1.GetRequestFlowSettingsRequest{})
	if err != nil || got.GetEnabled() {
		t.Fatalf("default flag = (%v, %v), want off", got, err)
	}
	requireFlowDisabled(t, "CreateRequest before enabling", tryCreate(k))
	if _, err := k.req.ListRequests(k.asReporter(), &requestv1.ListRequestsRequest{ProjectId: k.project}); err != nil {
		t.Fatalf("reads must work while off: %v", err)
	}

	// Only an admin can switch it, and a refusal is audited.
	_, err = k.req.SetRequestFlowSettings(k.as(k.reporter, "user"), &requestv1.SetRequestFlowSettingsRequest{Enabled: true})
	// Authorization (CR-REQ-035) refuses before the handler's own REQUEST_FLOW_ADMIN_ONLY check is reached.
	if code(err) != codes.PermissionDenied || !strings.Contains(errText(err), "REQUEST_FORBIDDEN") {
		t.Fatalf("non-admin Set = %v", err)
	}
	eventually(t, 10e9, "the denied access audit entry", func() (bool, string) {
		return hasAction(k.audit(), domain.AuditRequestAccessDenied, "denied"), "missing"
	})

	k.enableFlow()
	eventually(t, 10e9, "the allowed request.flow.set audit entry", func() (bool, string) {
		return hasAction(k.audit(), domain.ActionRequestFlowSet, "allowed"), "missing"
	})
	r := k.classified("flag lifecycle", "bug", "M")

	// Switch off in the middle of the Request's life.
	k.setFlow(false)
	if on, _ := k.req.GetRequestFlowSettings(k.asReporter(), &requestv1.GetRequestFlowSettingsRequest{}); on.GetEnabled() {
		t.Fatal("flag still on after Set(false)")
	}
	for name, err := range map[string]error{
		"CreateRequest": tryCreate(k),
		"ConfirmRequestType": func() error {
			_, e := k.req.ConfirmRequestType(k.asReporter(), &requestv1.ConfirmRequestTypeRequest{RequestId: r.GetId(), Type: "bug", Size: "M"})
			return e
		}(),
		"ChangeRequestType": func() error {
			_, e := k.req.ChangeRequestType(k.asReporter(), &requestv1.ChangeRequestTypeRequest{RequestId: r.GetId(), NewType: "task", Reason: "x"})
			return e
		}(),
		"ClassifyRequest": func() error {
			_, e := k.req.ClassifyRequest(k.asReporter(), &requestv1.ClassifyRequestRequest{RequestId: r.GetId()})
			return e
		}(),
		"Approve": func() error {
			_, e := k.appr.Approve(k.asAdmin(), &requestv1.ApproveRequest{Id: uuid.NewString()})
			return e
		}(),
	} {
		requireFlowDisabled(t, name, err)
	}
	// Reads and the internal callback are not gated; the latter answers Unimplemented until CR-REQ-013 lands.
	if got := k.get(r.GetId()); got.GetStatus() != "awaiting_type_confirmation" {
		t.Fatalf("a gated call changed the Request: %s", got.GetStatus())
	}
	if _, err := k.appr.ListApprovals(k.asReporter(), &requestv1.ListApprovalsRequest{RequestId: r.GetId()}); err != nil {
		t.Fatalf("ListApprovals while off: %v", err)
	}
	_, err = k.req.ReportTaskOutcome(k.asReporter(), &requestv1.ReportTaskOutcomeRequest{})
	requireNotGated(t, "ReportTaskOutcome", err)
	_, err = k.req.LookupRequestBySource(k.asReporter(), &requestv1.LookupRequestBySourceRequest{Provider: "jira", Ref: "ENG-1"})
	requireNotGated(t, "LookupRequestBySource", err)

	// Switch on again: the Request continues where it stopped.
	k.enableFlow()
	if got := k.confirm(r, "bug", "M"); got.GetStatus() != "analyzing" {
		t.Fatalf("after re-enabling, confirm leads to %s", got.GetStatus())
	}
}

// Leaving the flow is always allowed: return to backlog and cancel work with the flag off.
func TestE20_SafeExitsWorkWhileOff(t *testing.T) {
	k := newKit(t)
	k.enableFlow()
	toReturn := k.classified("exit by return", "bug", "M")
	toCancel := k.classified("exit by cancel", "bug", "M")
	k.setFlow(false)

	if _, err := k.req.CancelRequest(k.asReporter(), &requestv1.CancelRequestRequest{RequestId: toCancel.GetId(), Reason: "no longer needed"}); err != nil {
		t.Fatalf("CancelRequest while off: %v", err)
	}
	if got := k.get(toCancel.GetId()); got.GetStatus() != "cancelled" {
		t.Fatalf("status %s", got.GetStatus())
	}
	if _, err := k.req.ReturnToBacklog(k.asReporter(), &requestv1.ReturnToBacklogRequest{RequestId: toReturn.GetId(), Stage: "classification", Category: "other", Reason: "park it"}); err != nil {
		t.Fatalf("ReturnToBacklog while off: %v", err)
	}
	if got := k.get(toReturn.GetId()); got.GetStatus() != "request_backlog" {
		t.Fatalf("status %s", got.GetStatus())
	}
	// ...but reopening is moving forward again.
	_, err := k.req.ReopenRequest(k.asReporter(), &requestv1.ReopenRequestRequest{RequestId: toReturn.GetId()})
	requireFlowDisabled(t, "ReopenRequest", err)

	// Rejecting an approval is an exit too.
	k.enableFlow()
	pending := k.classified("exit by reject", "bug", "M")
	approvals, err := k.appr.ListApprovals(k.asReporter(), &requestv1.ListApprovalsRequest{RequestId: pending.GetId()})
	if err != nil || len(approvals.GetApprovals()) == 0 {
		t.Fatalf("no approval to reject: %v", err)
	}
	a := approvals.GetApprovals()[0]
	k.setFlow(false)
	_, err = k.appr.Approve(k.asAdmin(), &requestv1.ApproveRequest{Id: a.GetId(), ExpectedDigest: a.GetSubjectDigest()})
	requireFlowDisabled(t, "Approve", err)
	if _, err := k.appr.Reject(k.asAdmin(), &requestv1.RejectRequest{Id: a.GetId(), Comment: "not now", ExpectedDigest: a.GetSubjectDigest()}); err != nil {
		t.Fatalf("Reject while off: %v", err)
	}
}

func TestE20_TenantsAreIndependent(t *testing.T) {
	a, b := newKit(t), newKit(t)
	a.enableFlow()
	if err := tryCreate(a); err != nil {
		t.Fatalf("tenant A (on): %v", err)
	}
	requireFlowDisabled(t, "tenant B (never enabled)", tryCreate(b))
	b.enableFlow()
	a.setFlow(false)
	if err := tryCreate(b); err != nil {
		t.Fatalf("tenant B (on): %v", err)
	}
	requireFlowDisabled(t, "tenant A (switched off again)", tryCreate(a))
}

// The global switch wins: a tenant that is on is still blocked on a service started with REQUEST_FLOW_ENABLED=false.
func TestE20_GlobalSwitchOffBlocksEnabledTenant(t *testing.T) {
	k := newKit(t)
	k.enableFlow()
	off, err := theStack.startInstance(map[string]string{"REQUEST_FLOW_ENABLED": "false"})
	if err != nil {
		t.Fatal(err)
	}
	defer off.stop()
	c, err := grpc.NewClient(off.grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	client := requestv1.NewRequestServiceClient(c)
	ctx := metadata.AppendToOutgoingContext(k.asReporter(), grpcmw.MetadataTenantID, k.tenantID, internalcaller.MetadataKey, serviceToken)
	_, err = client.CreateRequest(ctx, &requestv1.CreateRequestRequest{ProjectId: k.project, Title: "global off",
		Source: &requestv1.RequestSource{Provider: "manual"}, ClientRequestId: uuid.NewString()})
	requireFlowDisabled(t, "CreateRequest with the global switch off", err)
	got, err := client.GetRequestFlowSettings(ctx, &requestv1.GetRequestFlowSettingsRequest{})
	if err != nil || got.GetEnabled() {
		t.Fatalf("GetRequestFlowSettings must report the effective value (off): (%v, %v)", got, err)
	}
}

// An unreadable flag fails closed for advancing RPCs and leaves reads alone. Runs last: it briefly breaks the table for everyone.
func TestE20_UnreadableFlagFailsClosed(t *testing.T) {
	k := newKit(t)
	k.enableFlow()
	r := k.classified("fail closed", "bug", "M")

	theStack.db.renameTenantSettings(t, "tenant_settings_unreadable")
	restored := false
	restore := func() {
		if !restored {
			restored = true
			theStack.db.renameTenantSettings(t, "tenant_settings")
		}
	}
	defer restore()

	requireFlowDisabled(t, "CreateRequest with an unreadable flag", tryCreate(k))
	if got := k.get(r.GetId()); got.GetId() != r.GetId() {
		t.Fatal("reads must not depend on the flag")
	}
	if _, err := k.req.CancelRequest(k.asReporter(), &requestv1.CancelRequestRequest{RequestId: r.GetId(), Reason: "cleanup"}); err != nil {
		t.Fatalf("a safe exit must not depend on the flag: %v", err)
	}
	restore()
	if err := tryCreate(k); err != nil {
		t.Fatalf("after the table is back the flag reads again: %v", err)
	}
}

//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/internalcaller"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/e2e/stubs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var (
	connOnce sync.Once
	connErr  error
	theConn  *grpc.ClientConn
)

func conn(t *testing.T) *grpc.ClientConn {
	t.Helper()
	connOnce.Do(func() {
		theConn, connErr = grpc.NewClient(theStack.grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	})
	if connErr != nil {
		t.Fatal(connErr)
	}
	return theConn
}

// kit is one isolated tenant with the people the scenarios need. A new tenant per test keeps tests independent
// and lets them run against one shared service.
type kit struct {
	t        *testing.T
	tenantID string
	project  string
	admin    string
	reporter string
	approver string
	outsider string

	req  requestv1.RequestServiceClient
	appr requestv1.ApprovalServiceClient
}

func newKit(t *testing.T) *kit {
	t.Helper()
	k := &kit{t: t, tenantID: uuid.NewString(), project: uuid.NewString(), admin: uuid.NewString(), reporter: uuid.NewString(),
		approver: uuid.NewString(), outsider: uuid.NewString()}
	c := conn(t)
	k.req, k.appr = requestv1.NewRequestServiceClient(c), requestv1.NewApprovalServiceClient(c)
	theStack.stubs.Auth.SetAdmins(k.tenantID, k.admin)
	theStack.stubs.Project.SetMember(k.tenantID, k.project, k.reporter, projectv1.ProjectRole_PROJECT_ROLE_OWNER)
	theStack.stubs.Project.SetMember(k.tenantID, k.project, k.approver, projectv1.ProjectRole_PROJECT_ROLE_MEMBER)
	return k
}

// as is a call context: tenant, user and role travel as metadata exactly as api-gateway sends them,
// plus the shared secret sibling services present.
func (k *kit) as(user, role string) context.Context {
	return metadata.AppendToOutgoingContext(context.Background(),
		grpcmw.MetadataTenantID, k.tenantID, grpcmw.MetadataUserID, user, grpcmw.MetadataRole, role, internalcaller.MetadataKey, serviceToken)
}

func (k *kit) asReporter() context.Context { return k.as(k.reporter, "user") }
func (k *kit) asAdmin() context.Context    { return k.as(k.admin, "admin") }

func (k *kit) enableFlow() {
	k.t.Helper()
	k.setFlow(true)
}

func (k *kit) setFlow(on bool) {
	k.t.Helper()
	if _, err := k.req.SetRequestFlowSettings(k.asAdmin(), &requestv1.SetRequestFlowSettingsRequest{Enabled: on}); err != nil {
		k.t.Fatalf("SetRequestFlowSettings(%v): %v", on, err)
	}
}

// create opens a manual Request. The title carries the marker the stub agent classifies by.
func (k *kit) create(title, typ, size string) *requestv1.Request {
	k.t.Helper()
	marker := stubs.MarkerFor(typ, size)
	if typ == "hotfix" {
		marker = "[e2e:type=hotfix size=" + size + " urgency=urgent]" // a hotfix must be urgent
	}
	req := &requestv1.CreateRequestRequest{
		ProjectId: k.project, Title: fmt.Sprintf("%s %s", title, marker), Body: "e2e body",
		Source: &requestv1.RequestSource{Provider: "manual"}, ClientRequestId: uuid.NewString(),
	}
	resp, err := k.req.CreateRequest(k.asReporter(), req)
	// The write rate limit (burst 10) is part of the service now; scenarios that create in a loop wait it out.
	for attempt := 0; attempt < 20 && code(err) == codes.ResourceExhausted; attempt++ {
		time.Sleep(300 * time.Millisecond)
		resp, err = k.req.CreateRequest(k.asReporter(), req)
	}
	if err != nil {
		k.t.Fatalf("CreateRequest: %v", err)
	}
	return resp.GetRequest()
}

func (k *kit) get(id string) *requestv1.Request {
	k.t.Helper()
	resp, err := k.req.GetRequest(k.asReporter(), &requestv1.GetRequestRequest{Id: id})
	if err != nil {
		k.t.Fatalf("GetRequest: %v", err)
	}
	return resp.GetRequest()
}

// waitStatus polls until the Request reaches want; the transitions it waits for are driven by the outbox
// relay, NATS and the classification runner, so they are asynchronous by design.
func (k *kit) waitStatus(id, want string) *requestv1.Request {
	k.t.Helper()
	var last *requestv1.Request
	eventually(k.t, 30*time.Second, fmt.Sprintf("request %s to reach %s", id, want), func() (bool, string) {
		last = k.get(id)
		return last.GetStatus() == want, "status is " + last.GetStatus()
	})
	return last
}

// classified creates a Request and waits until the AI proposal is waiting for a human.
func (k *kit) classified(title, typ, size string) *requestv1.Request {
	k.t.Helper()
	r := k.create(title, typ, size)
	return k.waitStatus(r.GetId(), "awaiting_type_confirmation")
}

func (k *kit) confirm(r *requestv1.Request, typ, size string) *requestv1.Request {
	k.t.Helper()
	urgency := ""
	if typ == "hotfix" {
		urgency = "urgent"
	}
	// A reason is accepted for every type and required for some (security).
	resp, err := k.req.ConfirmRequestType(k.asReporter(), &requestv1.ConfirmRequestTypeRequest{RequestId: r.GetId(), Type: typ, Size: size, Urgency: urgency, Reason: "confirmed by a human in the e2e run"})
	if err != nil {
		k.t.Fatalf("ConfirmRequestType: %v", err)
	}
	return resp.GetRequest()
}

func (k *kit) outbox(subject string) []string {
	k.t.Helper()
	return theStack.db.outboxPayloads(k.t, k.tenantID, subject)
}

func (k *kit) outboxSubjects() []string {
	k.t.Helper()
	return theStack.db.outboxSubjects(k.t, k.tenantID)
}

func (k *kit) audit() []stubs.AuditEntry { return theStack.stubs.Auth.Entries(k.tenantID) }

// eventually polls cond every 100ms until it holds. Never a fixed sleep: asynchronous steps finish when they finish.
func eventually(t *testing.T, timeout time.Duration, what string, cond func() (bool, string)) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	detail := ""
	for {
		ok, d := cond()
		if ok {
			return
		}
		detail = d
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %s waiting for %s (%s)\nservice log tail:\n%s", timeout, what, detail, theStack.logTail(25))
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func code(err error) codes.Code { return status.Code(err) }

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func hasAction(entries []stubs.AuditEntry, action, outcome string) bool {
	for _, e := range entries {
		if e.Action == action && (outcome == "" || e.Outcome == outcome) {
			return true
		}
	}
	return false
}

func containsAll(haystack, needles string) bool {
	for _, n := range strings.Fields(needles) {
		if !strings.Contains(haystack, n) {
			return false
		}
	}
	return true
}

func changeTypeReq(id, to string) *requestv1.ChangeRequestTypeRequest {
	return &requestv1.ChangeRequestTypeRequest{RequestId: id, NewType: to, Size: "M", Reason: "e2e changes the type"}
}

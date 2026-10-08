package wscompat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// Gateway-side checks must refuse before any RPC: the fake counts calls.
func TestRequestChannels_RefusedBeforeTheRPC(t *testing.T) {
	admin := Identity{TenantID: "t1", UserID: "u1", Role: "admin"}
	cases := []struct {
		name, channel, args string
		id                  Identity
		want                string
	}{
		{"approve without digest", "approval.approve", `{"id":"a1","expectedVersion":2}`, testRequestIdentity, "INVALID_ARGUMENT: expectedVersion and expectedDigest are required"},
		{"approve without version", "approval.approve", `{"id":"a1","expectedDigest":"d"}`, testRequestIdentity, "INVALID_ARGUMENT: expectedVersion and expectedDigest are required"},
		{"reject without digest", "approval.reject", `{"id":"a1","expectedVersion":2,"comment":"c"}`, testRequestIdentity, "INVALID_ARGUMENT"},
		{"reject blank comment", "approval.reject", `{"id":"a1","expectedVersion":2,"expectedDigest":"d","comment":"  "}`, testRequestIdentity, "REQUEST_APPROVAL_COMMENT_REQUIRED"},
		{"flowSet by a member", "request.flowSet", `{"enabled":true}`, testRequestIdentity, "REQUEST_FORBIDDEN"},
		{"flowSet without enabled", "request.flowSet", `{}`, admin, "INVALID_ARGUMENT"},
		{"commit without proposal", "request.generatePlan", `{"id":"r1","mode":"commit"}`, testRequestIdentity, "INVALID_ARGUMENT: proposal required for commit"},
		{"commit with garbage proposal", "request.generatePlan", `{"id":"r1","mode":"commit","proposal":{"phases":"x"}}`, testRequestIdentity, "INVALID_ARGUMENT"},
		{"commit with huge proposal", "request.generatePlan", `{"id":"r1","mode":"commit","proposal":{"notes":"` + strings.Repeat("x", maxPlanProposalBytes) + `"}}`, testRequestIdentity, "INVALID_ARGUMENT"},
		{"unknown plan mode", "request.generatePlan", `{"id":"r1","mode":"fly"}`, testRequestIdentity, "INVALID_ARGUMENT"},
		{"feedback too long", "solution.generate", `{"requestId":"r1","feedback":"` + strings.Repeat("é", 2001) + `"}`, testRequestIdentity, "INVALID_ARGUMENT"},
		{"unknown analysis mode", "solution.generate", `{"requestId":"r1","analysisMode":"deep"}`, testRequestIdentity, "INVALID_ARGUMENT"},
		{"unknown solution kind", "solution.list", `{"requestId":"r1","kind":"poem"}`, testRequestIdentity, "INVALID_ARGUMENT"},
		{"unspecified enum spelled out", "solution.list", `{"requestId":"r1","kind":"unspecified"}`, testRequestIdentity, "INVALID_ARGUMENT"},
		{"unknown approval subject", "approval.list", `{"requestId":"r1","subjectType":"dream"}`, testRequestIdentity, "INVALID_ARGUMENT"},
		{"unknown request type", "request.confirmType", `{"id":"r1","type":"epic"}`, testRequestIdentity, "INVALID_ARGUMENT"},
		{"unknown return stage", "request.returnToBacklog", `{"id":"r1","stage":"lunch","reason":"x"}`, testRequestIdentity, "INVALID_ARGUMENT"},
		{"unknown link reason", "request.spawnChild", `{"id":"r1","linkReason":"fun","title":"t"}`, testRequestIdentity, "INVALID_ARGUMENT"},
		{"unknown list status", "request.list", `{"status":["odd"]}`, testRequestIdentity, "INVALID_ARGUMENT"},
		{"unknown backlog type", "backlog.requests", `{"requestTypes":["odd"]}`, testRequestIdentity, "INVALID_ARGUMENT"},
		{"broken json arg", "request.get", `[1,2]`, testRequestIdentity, "INVALID_ARGUMENT"},
		{"claiming webhook", "request.create", `{"projectId":"p","title":"t","source":{"provider":"webhook"}}`, testRequestIdentity, "REQUEST_SOURCE_FORBIDDEN"},
		{"claiming mcp", "request.create", `{"projectId":"p","title":"t","source":{"provider":"mcp","site":"claude"}}`, testRequestIdentity, "REQUEST_SOURCE_FORBIDDEN"},
		{"claiming manual", "request.create", `{"projectId":"p","title":"t","source":{"provider":"manual"}}`, testRequestIdentity, "REQUEST_SOURCE_FORBIDDEN"},
		{"claiming nonsense", "request.create", `{"projectId":"p","title":"t","source":{"provider":"carrier-pigeon"}}`, testRequestIdentity, "REQUEST_SOURCE_FORBIDDEN"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := &fakeRequestClient{}
			_, err := callRequestChannel(t, newRequestTestRegistry(fake), c.id, context.Background(), c.channel, c.args)
			if err == nil || !strings.HasPrefix(err.Error(), c.want) {
				t.Fatalf("err = %v, want prefix %q", err, c.want)
			}
			if fake.count() != 0 {
				t.Errorf("%d RPC(s) were made; the gateway must refuse first", fake.count())
			}
		})
	}
}

func TestRequestCreate_TrackerSourcesPassAndOriginWins(t *testing.T) {
	fake := &fakeRequestClient{}
	r := newRequestTestRegistry(fake)
	_, err := callRequestChannel(t, r, testRequestIdentity, context.Background(), "request.create",
		`{"projectId":"p","title":"t","source":{"provider":"Jira","ref":"ABC-1","url":"https://x/y","site":"acme"}}`)
	if err != nil {
		t.Fatal(err)
	}
	src := fake.last().in.(*requestv1.CreateRequestRequest).GetSource()
	if src.GetProvider() != "jira" || src.GetRef() != "ABC-1" || src.GetSite() != "acme" || src.GetUrl() != "https://x/y" {
		t.Errorf("tracker source = %+v", src)
	}

	// An MCP origin overrides whatever the arguments say and ignores ref and url.
	ctx := WithToolOrigin(context.Background(), ToolOrigin{ClientName: "claude-code", MCPSessionID: "s", UserID: "u1"})
	if _, err := callRequestChannel(t, r, testRequestIdentity, ctx, "request.create",
		`{"projectId":"p","title":"t","source":{"provider":"jira","ref":"ABC-1"}}`); err != nil {
		t.Fatal(err)
	}
	src = fake.last().in.(*requestv1.CreateRequestRequest).GetSource()
	if src.GetProvider() != "mcp" || src.GetSite() != "claude-code" || src.GetRef() != "" || src.GetUrl() != "" {
		t.Errorf("mcp source = %+v", src)
	}
	// No client name: a placeholder keeps the display field filled.
	ctx = WithToolOrigin(context.Background(), ToolOrigin{})
	if _, err := callRequestChannel(t, r, testRequestIdentity, ctx, "request.create", `{"projectId":"p","title":"t"}`); err != nil {
		t.Fatal(err)
	}
	if got := fake.last().in.(*requestv1.CreateRequestRequest).GetSource().GetSite(); got != "unknown-mcp-client" {
		t.Errorf("site = %q", got)
	}
}

func TestRequestChannels_ErrorShaping(t *testing.T) {
	const marker = "SECRET-BODY-MARKER"
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"service code relayed", status.Error(codes.FailedPrecondition, "REQUEST_VERSION_CONFLICT: reload and retry"), "REQUEST_VERSION_CONFLICT: reload and retry"},
		{"envelope stripped", errors.New("rpc error: code = NotFound desc = REQUEST_NOT_FOUND: no such request"), "REQUEST_NOT_FOUND: no such request"},
		{"approval code relayed", status.Error(codes.PermissionDenied, "REQUEST_APPROVAL_NOT_APPROVER: not an approver"), "REQUEST_APPROVAL_NOT_APPROVER: not an approver"},
		{"unavailable", status.Error(codes.Unavailable, "connection refused 10.0.0.3:9090"), "REQUEST_UNAVAILABLE: request service unavailable"},
		{"deadline", status.Error(codes.DeadlineExceeded, "slow"), "REQUEST_UNAVAILABLE: request service did not respond in time"},
		{"unimplemented", status.Error(codes.Unimplemented, "method X not implemented"), "REQUEST_NOT_IMPLEMENTED: this operation is not available on the server yet"},
		{"not found without code", status.Error(codes.NotFound, "row "+marker), "REQUEST_NOT_FOUND: not found"},
		{"forbidden without code", status.Error(codes.PermissionDenied, "x"), "REQUEST_FORBIDDEN: not permitted"},
		{"rate limited without code", status.Error(codes.ResourceExhausted, "x"), "REQUEST_RATE_LIMITED: too many requests"},
		{"invalid without code", status.Error(codes.InvalidArgument, "title "+marker), "INVALID_ARGUMENT: invalid argument"},
		{"stale without code", status.Error(codes.FailedPrecondition, "x"), "REQUEST_STATE_STALE: the operation is not valid in the current state"},
		{"internal never leaks", status.Error(codes.Internal, "pq: password authentication failed "+marker), "REQUEST_INTERNAL: internal error"},
		{"plain error", errors.New("boom " + marker), "REQUEST_INTERNAL: internal error"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := &fakeRequestClient{err: c.err}
			_, err := callRequestChannel(t, newRequestTestRegistry(fake), testRequestIdentity, context.Background(), "request.create",
				`{"projectId":"p","title":"t","body":"`+marker+`"}`)
			if err == nil || err.Error() != c.want {
				t.Fatalf("err = %v, want %q", err, c.want)
			}
			if strings.Contains(err.Error(), marker) {
				t.Errorf("the request body or a service detail leaked into the error: %v", err)
			}
		})
	}
}

func TestRequestChannels_ApprovalForbiddenUsesApprovalCode(t *testing.T) {
	fake := &fakeRequestClient{err: status.Error(codes.PermissionDenied, "denied")}
	_, err := callRequestChannel(t, newRequestTestRegistry(fake), testRequestIdentity, context.Background(), "approval.approve",
		`{"id":"a","expectedVersion":1,"expectedDigest":"d"}`)
	if err == nil || err.Error() != "REQUEST_APPROVAL_FORBIDDEN: not permitted" {
		t.Fatalf("err = %v", err)
	}
}

func TestRequestChannels_AIDeadlineGetsOurCode(t *testing.T) {
	for _, ch := range []string{"request.classify", "request.generatePlan"} {
		fake := &fakeRequestClient{err: context.DeadlineExceeded}
		_, err := callRequestChannel(t, newRequestTestRegistry(fake), testRequestIdentity, context.Background(), ch, `{"id":"r1"}`)
		if err == nil || !strings.HasPrefix(err.Error(), "REQUEST_AI_COMPLETE_TIMEOUT:") {
			t.Errorf("%s: err = %v, want REQUEST_AI_COMPLETE_TIMEOUT", ch, err)
		}
	}
	// A deadline on a plain read is a service outage, not an AI timeout.
	fake := &fakeRequestClient{err: status.Error(codes.DeadlineExceeded, "x")}
	_, err := callRequestChannel(t, newRequestTestRegistry(fake), testRequestIdentity, context.Background(), "request.get", `{"id":"r1"}`)
	if err == nil || !strings.HasPrefix(err.Error(), "REQUEST_UNAVAILABLE:") {
		t.Errorf("request.get deadline: %v", err)
	}
}

func TestRequestChannels_PageSizeIsClamped(t *testing.T) {
	for _, c := range []struct {
		in   string
		want int32
	}{{`{}`, 20}, {`{"pageSize":0}`, 20}, {`{"pageSize":-5}`, 20}, {`{"pageSize":7}`, 7}, {`{"pageSize":100}`, 100}, {`{"pageSize":101}`, 100}} {
		fake := &fakeRequestClient{}
		if _, err := callRequestChannel(t, newRequestTestRegistry(fake), testRequestIdentity, context.Background(), "request.list", c.in); err != nil {
			t.Fatal(err)
		}
		if got := fake.last().in.(*requestv1.ListRequestsRequest).GetPageSize(); got != c.want {
			t.Errorf("%s -> %d, want %d", c.in, got, c.want)
		}
	}
}

func TestRequestChannels_EmptyListsAreArrays(t *testing.T) {
	r := NewRegistry()
	fake := &fakeRequestClient{empty: true}
	registerRequestChannels(r, fake, fake, nil, false)
	for ch, key := range map[string]string{"request.list": "requests", "solution.list": "solutions", "approval.list": "approvals",
		"approval.listPending": "approvals", "backlog.requests": "requestRows", "backlog.tasks": "groups", "backlog.execute": "groups",
		"request.typeHistory": "changes", "request.checks": "checks"} {
		res, err := callRequestChannel(t, r, testRequestIdentity, context.Background(), ch, `{"id":"r","requestId":"r"}`)
		if err != nil {
			t.Fatalf("%s: %v", ch, err)
		}
		arr, ok := res[key].([]any)
		if !ok || len(arr) != 0 {
			t.Errorf("%s: %s = %#v, want []", ch, key, res[key])
		}
	}
	res, _ := callRequestChannel(t, r, testRequestIdentity, context.Background(), "solution.list", `{"requestId":"r"}`)
	if arr, ok := res["runs"].([]any); !ok || len(arr) != 0 || res["nextPageToken"] != "" {
		t.Errorf("solution.list = %v", res)
	}
}

func TestRequestChannelError_Nil(t *testing.T) {
	if requestChannelError(nil) != nil || approvalChannelError(nil) != nil {
		t.Fatal("nil must stay nil")
	}
	var typedNil *fakeRequestClient
	if !requestClientMissing(typedNil) || requestClientMissing(&fakeRequestClient{}) {
		t.Fatal("requestClientMissing must see a nil pointer inside an interface")
	}
	_ = fmt.Sprint(time.Second)
}

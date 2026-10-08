//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/grpcmw"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
)

// CR-REQ-025 section 2.4: behaviour that differs between Postgres and MySQL, checked through the running service.

func (k *kit) createJira(ref string) *requestv1.Request {
	k.t.Helper()
	resp, err := k.req.CreateRequest(k.asReporter(), &requestv1.CreateRequestRequest{
		ProjectId: k.project, Title: "jira issue " + ref, Body: "from jira", ClientRequestId: uuid.NewString(),
		Source: &requestv1.RequestSource{Provider: "jira", Ref: ref, Site: "https://e2e.atlassian.net"},
		Hints:  &requestv1.SourceHints{IssueType: "Bug"},
	})
	if err != nil {
		k.t.Fatalf("CreateRequest(jira %s): %v", ref, err)
	}
	return resp.GetRequest()
}

// The same client_request_id twice yields one Request and one created event (request_idempotency claim).
func TestDialect_IdempotentCreate(t *testing.T) {
	k := newKit(t)
	k.enableFlow()
	req := &requestv1.CreateRequestRequest{ProjectId: k.project, Title: "once", Source: &requestv1.RequestSource{Provider: "manual"}, ClientRequestId: uuid.NewString()}
	first, err := k.req.CreateRequest(k.asReporter(), req)
	if err != nil || !first.GetCreated() {
		t.Fatalf("first create: (%v, %v)", first, err)
	}
	second, err := k.req.CreateRequest(k.asReporter(), req)
	if err != nil || second.GetCreated() || second.GetRequest().GetId() != first.GetRequest().GetId() {
		t.Fatalf("second create must return the same Request without creating: (%v, %v)", second, err)
	}
	if n := len(k.outbox(domain.SubjectRequestCreated)); n != 1 {
		t.Fatalf("%d created events, want 1", n)
	}
}

// Two approvers decide the same approval at once: exactly one wins (SELECT ... FOR UPDATE on the approval row).
func TestDialect_ConcurrentApproveExactlyOneWins(t *testing.T) {
	k := newKit(t)
	k.enableFlow()
	r := k.classified("race", "bug", "M")
	list, err := k.appr.ListApprovals(k.asReporter(), &requestv1.ListApprovalsRequest{RequestId: r.GetId()})
	if err != nil || len(list.GetApprovals()) == 0 {
		t.Fatalf("no approval: %v", err)
	}
	a := list.GetApprovals()[0]
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, ctx := range []context.Context{k.asAdmin(), k.asReporter()} {
		ctx := ctx
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := k.appr.Approve(ctx, &requestv1.ApproveRequest{Id: a.GetId(), ExpectedDigest: a.GetSubjectDigest()})
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	wins, losses := 0, 0
	for err := range results {
		switch {
		case err == nil:
			wins++
		case code(err) == codes.FailedPrecondition || code(err) == codes.Aborted || code(err) == codes.AlreadyExists:
			losses++
		default:
			t.Fatalf("unexpected error from a concurrent Approve: %v", err)
		}
	}
	if wins != 1 || losses != 1 {
		t.Fatalf("wins=%d losses=%d, want exactly one of each", wins, losses)
	}
}

// ListRequests pages in a stable order with no duplicate and no gap, on both dialects' sort rules.
func TestDialect_ListRequestsPaginationIsStable(t *testing.T) {
	k := newKit(t)
	k.enableFlow()
	want := map[string]bool{}
	for i := 0; i < 12; i++ {
		want[k.create("page", "bug", "S").GetId()] = true
	}
	seen := map[string]bool{}
	token := ""
	for page := 0; page < 10; page++ {
		resp, err := k.req.ListRequests(k.asReporter(), &requestv1.ListRequestsRequest{ProjectId: k.project, PageSize: 5, PageToken: token})
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range resp.GetRequests() {
			if seen[r.GetId()] {
				t.Fatalf("request %s appears on two pages", r.GetId())
			}
			seen[r.GetId()] = true
		}
		if token = resp.GetNextPageToken(); token == "" {
			break
		}
	}
	for id := range want {
		if !seen[id] {
			t.Fatalf("request %s missing from the pages", id)
		}
	}
	if len(seen) != len(want) {
		t.Fatalf("%d requests paged, want %d", len(seen), len(want))
	}
}

// LookupRequestBySource is the internal door issue-status-sync uses: shared secret required, tenant scoped, flag aware.
func TestLookupRequestBySource_InternalGuardTenantAndFlag(t *testing.T) {
	k := newKit(t)
	k.enableFlow()
	r := k.createJira("ENG-42")
	lookup := func(ctx context.Context, ref, site string) (*requestv1.LookupRequestBySourceResponse, error) {
		return k.req.LookupRequestBySource(ctx, &requestv1.LookupRequestBySourceRequest{Provider: "jira", Ref: ref, Site: site})
	}

	got, err := lookup(k.asReporter(), "eng-42", "")
	if err != nil || !got.GetFound() || got.GetRequestId() != r.GetId() {
		t.Fatalf("lookup by ref with any site: (%v, %v)", got, err)
	}
	if got, err := lookup(k.asReporter(), "ENG-42", "https://e2e.atlassian.net/"); err != nil || !got.GetFound() {
		t.Fatalf("lookup with the exact site: (%v, %v)", got, err)
	}
	if got, _ := lookup(k.asReporter(), "ENG-42", "https://other.atlassian.net"); got.GetFound() {
		t.Fatal("another site must not match")
	}
	if got, _ := lookup(k.asReporter(), "ENG-43", ""); got.GetFound() {
		t.Fatal("another issue must not match")
	}

	// Without the shared secret the RPC is refused.
	noToken := metadata.AppendToOutgoingContext(context.Background(), grpcmw.MetadataTenantID, k.tenantID)
	if _, err := lookup(noToken, "ENG-42", ""); code(err) != codes.PermissionDenied {
		t.Fatalf("lookup without the internal token = %v, want PermissionDenied", err)
	}
	// Another tenant never sees it.
	other := newKit(t)
	other.enableFlow()
	if got, err := other.req.LookupRequestBySource(other.asReporter(), &requestv1.LookupRequestBySourceRequest{Provider: "jira", Ref: "ENG-42"}); err != nil || got.GetFound() {
		t.Fatalf("tenant isolation: (%v, %v)", got, err)
	}

	// Flag off: not found, so issue-status-sync falls back to its worktree/PR sync.
	k.setFlow(false)
	if got, err := lookup(k.asReporter(), "ENG-42", ""); err != nil || got.GetFound() {
		t.Fatalf("flag off: (%v, %v), want found=false", got, err)
	}
	k.enableFlow()

	// A finished Request no longer owns the issue.
	if _, err := k.req.CancelRequest(k.asReporter(), &requestv1.CancelRequestRequest{RequestId: r.GetId(), Reason: "done here"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := lookup(k.asReporter(), "ENG-42", ""); got.GetFound() {
		t.Fatal("a cancelled Request must not own the issue")
	}
}

// status_changed carries what issue-status-sync needs to pick the Jira issue, and nothing typed by the user.
func TestStatusChangedPayloadFeedsIssueSync(t *testing.T) {
	k := newKit(t)
	k.enableFlow()
	r := k.createJira("ENG-77")
	k.waitStatus(r.GetId(), "awaiting_type_confirmation")
	var found bool
	for _, raw := range k.outbox(domain.SubjectRequestStatusChanged) {
		var p map[string]any
		if err := json.Unmarshal([]byte(raw), &p); err != nil {
			t.Fatal(err)
		}
		if p["to"] != "classifying" {
			continue
		}
		found = true
		want := map[string]any{"source_provider": "jira", "source_ref": "ENG-77", "source_site": "https://e2e.atlassian.net", "reporter_id": k.reporter,
			"request_id": r.GetId(), "actor_kind": "user"}
		for key, v := range want {
			if p[key] != v {
				t.Errorf("%s = %v, want %v", key, p[key], v)
			}
		}
		if p["version"] == nil || p["number"] == nil {
			t.Errorf("version and number are required for ordering and Jira comments: %v", p)
		}
		if tp, _ := p["traceparent"].(string); !strings.HasPrefix(tp, "00-") {
			t.Errorf("traceparent = %q, want a W3C traceparent (the RPC span is active)", tp)
		}
		if strings.Contains(raw, "jira issue") || strings.Contains(raw, "from jira") {
			t.Errorf("status_changed leaks the title or body: %s", raw)
		}
	}
	if !found {
		t.Fatalf("no status_changed event with the source and reporter: %v", k.outbox(domain.SubjectRequestStatusChanged))
	}
}

// Health checks carry no tenant and must never be touched by the flag gate or the audit interceptor.
func TestServiceHealthChecksWorkWithoutATenant(t *testing.T) {
	hc, err := grpc_health_v1.NewHealthClient(conn(t)).Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
	if err != nil || hc.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
		t.Fatalf("grpc health = (%v, %v)", hc, err)
	}
}

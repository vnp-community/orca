package usecase_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"
	tt "github.com/stablyai/orca-go/services/mcp-service/internal/usecase/usecasetest"
)

var execArgs = []byte(`{"data":"ls -la\n","ptyId":"p1"}`)

func requestApproval(t *testing.T, h *tt.Harness, tool domain.ToolRef, args []byte) usecase.AuthorizeOutput {
	t.Helper()
	out, err := h.Authorize.Execute(tt.Ctx(tenantA, userU, "user"), usecase.AuthorizeInput{Tool: tool, Ctx: tt.CC(), Args: args})
	if err != nil {
		t.Fatal(err)
	}
	if out.Outcome != domain.DecisionRequireApproval || out.ApprovalID == "" || out.CallID != "" {
		t.Fatalf("expected a pending approval, got %+v", out)
	}
	return out
}

func decide(h *tt.Harness, user, role, id, decision, hash, via string) (domain.Approval, error) {
	return h.Decide.Execute(tt.Ctx(tenantA, user, role), usecase.DecideApprovalInput{ApprovalID: id, Decision: decision, ParamsHash: hash, Via: via})
}

func TestApprovalHappyPathIsSingleUse(t *testing.T) {
	h := tt.NewHarness(true)
	out := requestApproval(t, h, tt.ToolExec, execArgs)
	// Asking again with the same call reuses the open approval (idempotent).
	again := requestApproval(t, h, tt.ToolExec, execArgs)
	if again.ApprovalID != out.ApprovalID {
		t.Fatal("same call must reuse the open approval")
	}
	if len(h.Store.EventsOf(domain.SubjectApprovalRequested)) != 1 {
		t.Fatal("exactly one approval.requested event expected")
	}
	if _, err := decide(h, userU, "user", out.ApprovalID, "approve", out.ParamsHash, domain.ViaWeb); err != nil {
		t.Fatal(err)
	}
	ctx := tt.Ctx(tenantA, userU, "user")
	run, err := h.Authorize.Execute(ctx, usecase.AuthorizeInput{Tool: tt.ToolExec, Ctx: tt.CC(), Args: execArgs})
	if err != nil || run.Outcome != domain.DecisionAllow || run.CallID == "" {
		t.Fatalf("approved call must run once: %+v %v", run, err)
	}
	second, _ := h.Authorize.Execute(ctx, usecase.AuthorizeInput{Tool: tt.ToolExec, Ctx: tt.CC(), Args: execArgs})
	if second.Outcome != domain.DecisionRequireApproval || second.ApprovalID == out.ApprovalID {
		t.Fatalf("approval must be single-use, got %+v", second)
	}
	if err := h.Complete.Execute(ctx, usecase.CompleteInput{CallID: run.CallID, Result: "ok", DurationMs: 12}); err != nil {
		t.Fatal(err)
	}
	if h.Store.Calls[run.CallID].Decision != domain.CallApproved || h.Store.Calls[run.CallID].ApprovedBy != userU {
		t.Fatalf("journal row: %+v", h.Store.Calls[run.CallID])
	}
}

func TestApprovalConcurrentConsumeAdmitsExactlyOne(t *testing.T) {
	h := tt.NewHarness(true)
	out := requestApproval(t, h, tt.ToolExec, execArgs)
	if _, err := decide(h, userU, "user", out.ApprovalID, "approve", out.ParamsHash, domain.ViaWeb); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := h.Authorize.Execute(tt.Ctx(tenantA, userU, "user"), usecase.AuthorizeInput{Tool: tt.ToolExec, Ctx: tt.CC(), Args: execArgs})
			if err == nil && r.Outcome == domain.DecisionAllow {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if allowed != 1 {
		t.Fatalf("%d concurrent calls were admitted from one approval, want exactly 1", allowed)
	}
}

func TestApprovalChangedArgumentsNeedNewApproval(t *testing.T) {
	h := tt.NewHarness(true)
	ls := requestApproval(t, h, tt.ToolExec, []byte(`{"data":"ls"}`))
	if _, err := decide(h, userU, "user", ls.ApprovalID, "approve", ls.ParamsHash, domain.ViaWeb); err != nil {
		t.Fatal(err)
	}
	rm := requestApproval(t, h, tt.ToolExec, []byte(`{"data":"rm -rf /"}`))
	if rm.ApprovalID == ls.ApprovalID || rm.ParamsHash == ls.ParamsHash {
		t.Fatal("different arguments must produce a different approval and hash")
	}
}

func TestDecideErrorsAndOwnership(t *testing.T) {
	h := tt.NewHarness(true)
	out := requestApproval(t, h, tt.ToolExec, execArgs)
	id := out.ApprovalID
	cases := []struct {
		name       string
		user, role string
		hash       string
		want       string
	}{
		{"another user", "dddddddd-dddd-dddd-dddd-dddddddddddd", "user", out.ParamsHash, domain.CodeNotFound},
		{"an admin cannot decide for the owner", userAdm, "admin", out.ParamsHash, domain.CodeNotFound},
		{"hash mismatch", userU, "user", "sha256:deadbeef", domain.CodeApprovalHashMismatch},
	}
	for _, c := range cases {
		if _, err := decide(h, c.user, c.role, id, "approve", c.hash, domain.ViaWeb); code(err) != c.want {
			t.Errorf("%s: got %v want %s", c.name, err, c.want)
		}
	}
	// A rejected attempt left the approval pending.
	if h.Store.Approvals[id].Status != domain.ApprovalPending {
		t.Fatal("failed decisions must not change state")
	}
	if _, err := decide(h, userU, "user", "not-a-uuid", "approve", out.ParamsHash, domain.ViaWeb); code(err) != domain.CodeNotFound {
		t.Errorf("bad id: %v", err)
	}
	if _, err := decide(h, userU, "user", id, "approve", "", domain.ViaWeb); code(err) != domain.CodeInvalidArgument {
		t.Errorf("approve without hash: %v", err)
	}
	if _, err := decide(h, userU, "user", id, "maybe", out.ParamsHash, domain.ViaWeb); code(err) != domain.CodeInvalidArgument {
		t.Errorf("bad decision: %v", err)
	}
	if _, err := decide(h, userU, "user", id, "approve", out.ParamsHash, "carrier-pigeon"); code(err) != domain.CodeInvalidArgument {
		t.Errorf("bad via: %v", err)
	}
	// Cross-tenant looks identical to "missing".
	if _, err := h.Decide.Execute(tt.Ctx(tenantB, userU, "user"), usecase.DecideApprovalInput{ApprovalID: id, Decision: "approve", ParamsHash: out.ParamsHash, Via: domain.ViaWeb}); code(err) != domain.CodeNotFound {
		t.Errorf("cross-tenant: %v", err)
	}
	if a, err := decide(h, userU, "mobile-less", id, "approve", out.ParamsHash, domain.ViaMobile); err != nil || a.Status != domain.ApprovalApproved || a.DecidedVia != domain.ViaMobile {
		t.Fatalf("owner decision: %+v %v", a, err)
	}
	// Double decision, either direction.
	for _, d := range []string{"approve", "deny"} {
		if _, err := decide(h, userU, "user", id, d, out.ParamsHash, domain.ViaWeb); code(err) != domain.CodeApprovalAlreadyDecided {
			t.Errorf("double %s: %v", d, err)
		}
	}
}

func TestDenyDoesNotNeedHashAndIsAudited(t *testing.T) {
	h := tt.NewHarness(true)
	out := requestApproval(t, h, tt.ToolExec, execArgs)
	a, err := decide(h, userU, "user", out.ApprovalID, "deny", "", domain.ViaWeb)
	if err != nil || a.Status != domain.ApprovalDenied {
		t.Fatalf("%+v %v", a, err)
	}
	run, _ := h.Authorize.Execute(tt.Ctx(tenantA, userU, "user"), usecase.AuthorizeInput{Tool: tt.ToolExec, Ctx: tt.CC(), Args: execArgs})
	if run.Outcome == domain.DecisionAllow {
		t.Fatal("denied approval must never run the call")
	}
	found := false
	for _, c := range h.Store.Calls {
		found = found || (c.Decision == domain.CallDenied && c.ApprovalID == out.ApprovalID)
	}
	if !found {
		t.Fatal("a denied decision must leave a journal/audit row")
	}
}

func TestApprovalExpiry(t *testing.T) {
	h := tt.NewHarness(true)
	out := requestApproval(t, h, tt.ToolExec, execArgs)
	h.Clock.Advance(11 * time.Minute) // default TTL 600s
	if _, err := decide(h, userU, "user", out.ApprovalID, "approve", out.ParamsHash, domain.ViaWeb); code(err) != domain.CodeApprovalExpired {
		t.Fatalf("lazy expiry on decide: %v", err)
	}
	n, err := h.Expire.Execute(context.Background(), 10)
	if err != nil || n != 1 || h.Store.Approvals[out.ApprovalID].Status != domain.ApprovalExpired {
		t.Fatalf("worker: n=%d err=%v status=%s", n, err, h.Store.Approvals[out.ApprovalID].Status)
	}
	if n, _ := h.Expire.Execute(context.Background(), 10); n != 0 {
		t.Fatal("expiry must be idempotent")
	}
	var audited bool
	for _, c := range h.Store.Calls {
		audited = audited || c.Decision == domain.CallExpired
	}
	if !audited {
		t.Fatal("expiry must be journaled")
	}
	// An approved-but-unused approval also dies with the clock.
	out2 := requestApproval(t, h, tt.ToolExec, execArgs)
	_, _ = decide(h, userU, "user", out2.ApprovalID, "approve", out2.ParamsHash, domain.ViaWeb)
	h.Clock.Advance(11 * time.Minute)
	r, _ := h.Authorize.Execute(tt.Ctx(tenantA, userU, "user"), usecase.AuthorizeInput{Tool: tt.ToolExec, Ctx: tt.CC(), Args: execArgs})
	if r.Outcome == domain.DecisionAllow {
		t.Fatal("expired approval must not authorize a call")
	}
}

func TestListApprovalsOwnOnlyKeyset(t *testing.T) {
	h := tt.NewHarness(true)
	for i := 0; i < 5; i++ {
		requestApproval(t, h, tt.ToolExec, []byte(`{"n":`+string(rune('0'+i))+`}`))
		h.Clock.Advance(time.Second)
	}
	other := tt.Ctx(tenantA, "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee", "user")
	if got, _ := h.List.Execute(other, usecase.ListApprovalsInput{PendingOnly: true}); len(got.Approvals) != 0 {
		t.Fatal("list must only show the caller's approvals")
	}
	ctx := tt.Ctx(tenantA, userU, "user")
	p1, err := h.List.Execute(ctx, usecase.ListApprovalsInput{PendingOnly: true, Limit: 2})
	if err != nil || len(p1.Approvals) != 2 || p1.NextCursor == "" {
		t.Fatalf("%+v %v", p1, err)
	}
	p2, _ := h.List.Execute(ctx, usecase.ListApprovalsInput{PendingOnly: true, Limit: 10, Cursor: p1.NextCursor})
	if len(p2.Approvals) != 3 || !p1.Approvals[1].CreatedAt.After(p2.Approvals[0].CreatedAt) {
		t.Fatalf("keyset order broken: %d", len(p2.Approvals))
	}
	if _, err := h.List.Execute(ctx, usecase.ListApprovalsInput{Cursor: "garbage"}); code(err) != domain.CodeInvalidArgument {
		t.Fatalf("bad cursor: %v", err)
	}
}

func TestWaitApprovalReturnsDecisionButNeverConsumes(t *testing.T) {
	h := tt.NewHarness(true)
	out := requestApproval(t, h, tt.ToolExec, execArgs)
	ctx := tt.Ctx(tenantA, userU, "user")
	st, err := h.Wait.Execute(ctx, out.ApprovalID, 30*time.Millisecond)
	if err != nil || st != domain.ApprovalPending {
		t.Fatalf("timeout must report pending: %s %v", st, err)
	}
	go func() {
		time.Sleep(20 * time.Millisecond)
		_, _ = decide(h, userU, "user", out.ApprovalID, "approve", out.ParamsHash, domain.ViaWeb)
	}()
	st, err = h.Wait.Execute(ctx, out.ApprovalID, 2*time.Second)
	if err != nil || st != domain.ApprovalApproved {
		t.Fatalf("wait: %s %v", st, err)
	}
	if h.Store.Approvals[out.ApprovalID].ConsumedAt != nil {
		t.Fatal("wait must not consume")
	}
	if _, err := h.Wait.Execute(tt.Ctx(tenantA, userAdm, "admin"), out.ApprovalID, time.Millisecond); code(err) != domain.CodeNotFound {
		t.Fatalf("non-owner wait: %v", err)
	}
}

func TestElicitationOnlyDecidesLowRisk(t *testing.T) {
	h := tt.NewHarness(true)
	h.Store.Policies[tenantA] = []domain.ToolPolicy{pol("p", "require_approval", domain.ToolPolicyMatch{Tool: "task_create"})}
	w := requestApproval(t, h, tt.ToolWrite, []byte(`{"title":"x"}`))
	if !w.ElicitationEligible || !strings.Contains(w.Prompt, `"title": "x"`) {
		t.Fatalf("write approval should be elicitation eligible with a server-built prompt: %+v", w)
	}
	e := requestApproval(t, h, tt.ToolExec, execArgs)
	if e.ElicitationEligible || e.Prompt != "" {
		t.Fatal("exec approval must not be elicitation eligible")
	}
	if _, err := decide(h, userU, "user", e.ApprovalID, "approve", e.ParamsHash, domain.ViaElicitation); err == nil {
		t.Fatal("elicitation accept on exec must be rejected")
	}
	if h.Store.Approvals[e.ApprovalID].Status != domain.ApprovalPending {
		t.Fatal("exec approval must stay pending after a faked elicitation accept")
	}
	if _, err := decide(h, userU, "user", w.ApprovalID, "approve", w.ParamsHash, domain.ViaElicitation); err != nil {
		t.Fatalf("elicitation on write: %v", err)
	}
	if _, err := decide(h, userU, "user", e.ApprovalID, "deny", "", domain.ViaElicitation); err != nil {
		t.Fatalf("elicitation decline is safe at any risk: %v", err)
	}
}

func TestApprovalFloodCaps(t *testing.T) {
	h := tt.NewHarness(true)
	ctx := tt.Ctx(tenantA, userU, "user")
	for i := 0; i < h.Cfg.MaxPendingPerClient; i++ {
		b, _ := json.Marshal(map[string]int{"n": i})
		requestApproval(t, h, tt.ToolExec, b)
	}
	out, err := h.Authorize.Execute(ctx, usecase.AuthorizeInput{Tool: tt.ToolExec, Ctx: tt.CC(), Args: []byte(`{"n":999}`)})
	if err != nil || out.Outcome != domain.DecisionDeny || out.Reasons[0] != domain.ReasonApprovalFlood {
		t.Fatalf("approval flood must be denied: %+v %v", out, err)
	}
}

func TestDecideBlockedByKillSwitch(t *testing.T) {
	h := tt.NewHarness(true)
	out := requestApproval(t, h, tt.ToolExec, execArgs)
	if err := h.KillAdmin.Set(tt.Ctx(tenantA, userAdm, "admin"), usecase.SetKillSwitchInput{Scope: "tenant", Reason: "incident", Active: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := decide(h, userU, "user", out.ApprovalID, "approve", out.ParamsHash, domain.ViaWeb); code(err) != domain.CodeKillSwitchActive {
		t.Fatalf("got %v", err)
	}
}

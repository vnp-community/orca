package usecase

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// memDecisions is an in-memory OutcomeReviewDecisionRepository with the rules of the real ones.
type memDecisions struct {
	mu   sync.Mutex
	rows []domain.ItemDecision
}

func (m *memDecisions) Record(_ context.Context, d domain.ItemDecision) (domain.ItemDecision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.rows {
		if m.rows[i].ReviewID == d.ReviewID && m.rows[i].ItemKey == d.ItemKey {
			m.rows[i].Active = false
		}
	}
	d.ID, d.Active, d.DecidedAt = uuid.NewString(), true, time.Now().UTC()
	m.rows = append(m.rows, d)
	return d, nil
}
func (m *memDecisions) InForce(_ context.Context, review string) ([]domain.ItemDecision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.ItemDecision
	for _, d := range m.rows {
		if d.ReviewID == review && d.Active {
			out = append(out, d)
		}
	}
	return out, nil
}
func (m *memDecisions) History(_ context.Context, review, item string) ([]domain.ItemDecision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.ItemDecision
	for i := len(m.rows) - 1; i >= 0; i-- {
		if m.rows[i].ReviewID == review && m.rows[i].ItemKey == item {
			out = append(out, m.rows[i])
		}
	}
	return out, nil
}
func (m *memDecisions) LinkChild(_ context.Context, review string, keys []string, child string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for i := range m.rows {
		for _, k := range keys {
			if m.rows[i].ReviewID == review && m.rows[i].Active && m.rows[i].Decision == domain.DecisionSendBack && m.rows[i].ItemKey == k {
				m.rows[i].ChildRequestID = child
				n++
			}
		}
	}
	return n, nil
}

type memLinks struct{ children map[string][]string }

func (m memLinks) Insert(context.Context, domain.RequestLink) error { return nil }
func (m memLinks) ListParents(context.Context, string) ([]domain.RequestLink, error) {
	return nil, nil
}
func (m memLinks) ListChildren(_ context.Context, parent string) ([]domain.RequestLink, error) {
	var out []domain.RequestLink
	for _, c := range m.children[parent] {
		out = append(out, domain.RequestLink{ParentRequestID: parent, ChildRequestID: c})
	}
	return out, nil
}
func (m memLinks) Delete(context.Context, string, string) error { return nil }

type reviewAuditLog struct {
	mu     sync.Mutex
	events []AuditEvent
}

func (c *reviewAuditLog) Record(_ context.Context, e AuditEvent) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, e)
	return nil
}

type decideEnv struct {
	*runEnv
	decider   *ReviewDecider
	decisions *memDecisions
	audit     *reviewAuditLog
	review    ReviewView
	breaking  string // key of the item that asks for a decision
	claim     string // key of an item an agent claimed that needs none
	fact      string // key of an item Orca measured
}

func deliveredResultWithItems() domain.ContextResult {
	res := goodDeliveredResult()
	res.Items = []domain.AgentReviewItem{
		{Subject: "breaking", Section: domain.SectionContracts, Change: domain.ChangeModified, Title: "A field was removed from GetRequest", Severity: domain.SeverityBreaking, NeedsDecision: true,
			Evidence: []domain.ReviewEvidence{{Kind: domain.EvidenceFile, Ref: "backend-go/a.go"}}},
		{Subject: "claim", Section: domain.SectionArchitecture, Change: domain.ChangeAdded, Title: "A new call to the notifier", Severity: domain.SeverityWarning,
			Evidence: []domain.ReviewEvidence{{Kind: domain.EvidenceFile, Ref: "backend-go/a.go"}}},
	}
	return res
}

// withDesign makes a delivered result carry the design too, as a forced run reads the graph again.
func withDesign(r domain.ContextResult) domain.ContextResult {
	d := goodDesignResult()
	r.KG, r.Design = d.KG, d.Design
	return r
}

func newDecideEnv(t *testing.T) *decideEnv {
	t.Helper()
	e := &decideEnv{runEnv: newRunEnv(t, domain.RequestStatusExecuting), decisions: &memDecisions{}, audit: &reviewAuditLog{}}
	e.agent.replies = []func(AgentPromptInput) (AgentPromptResult, error){agentReply(goodDesignResult()), agentReply(deliveredResultWithItems())}
	if _, err := e.runner.Start(e.ctx, StartContextInput{RequestID: e.req.ID}); err != nil {
		t.Fatal(err)
	}
	finishAll(e.reviewEnv)
	e.setStatus(domain.RequestStatusCompleted)
	if _, err := e.runner.Start(e.ctx, StartContextInput{RequestID: e.req.ID}); err != nil {
		t.Fatal(err)
	}
	e.ctx = tenant.WithRole(tenant.WithUserID(e.ctx, uuid.NewString()), "admin")
	e.svc.Decisions, e.svc.Audit = e.decisions, e.audit
	e.decider = &ReviewDecider{Service: e.svc, Decisions: e.decisions, Links: memLinks{children: map[string][]string{}}}
	e.refresh(t)
	for _, it := range e.review.Review.Items {
		switch {
		case it.Origin == domain.OriginAgent && it.NeedsDecision:
			e.breaking = it.Key
		case it.Origin == domain.OriginAgent && !it.NeedsDecision && it.Section == domain.SectionArchitecture:
			e.claim = it.Key
		case it.Origin == domain.OriginFact && e.fact == "":
			e.fact = it.Key
		}
	}
	if e.breaking == "" || e.claim == "" || e.fact == "" || e.review.Review.Phase != domain.ReviewPhaseDelivered {
		t.Fatalf("fixture: breaking=%q claim=%q fact=%q phase=%s", e.breaking, e.claim, e.fact, e.review.Review.Phase)
	}
	return e
}

func (e *decideEnv) refresh(t *testing.T) {
	t.Helper()
	v, ok, err := e.svc.Latest(e.ctx, e.req.ID)
	if err != nil || !ok {
		t.Fatal(err)
	}
	e.review = v
}

func (e *decideEnv) decide(t *testing.T, item string, d domain.ReviewDecision, note string) (DecideResult, error) {
	t.Helper()
	e.refresh(t)
	return e.decider.Decide(e.ctx, DecideInput{RequestID: e.req.ID, ReviewID: e.review.Stored.ID, ItemKey: item, Decision: d, Note: note, ExpectedVersion: e.review.Stored.Version, DecidedBy: uuid.NewString()})
}

func TestDecide_RulesOfWhatCanBeDecided(t *testing.T) {
	e := newDecideEnv(t)
	cases := []struct {
		name string
		item string
		d    domain.ReviewDecision
		note string
		code string
	}{
		{"accept an exception for an item that asks for one", e.breaking, domain.DecisionAcceptException, "ADR-09 allows it", ""},
		{"accept without a note", e.breaking, domain.DecisionAcceptException, "  ", domain.CodeReviewDecisionInvalid},
		{"accept an item that asks for nothing", e.claim, domain.DecisionAcceptException, "fine", domain.CodeReviewDecisionInvalid},
		{"dismiss a claim an agent made", e.claim, domain.DecisionDismiss, "it is wrong", ""},
		{"dismiss without a note", e.claim, domain.DecisionDismiss, "", domain.CodeReviewDecisionInvalid},
		{"dismiss a fact Orca measured", e.fact, domain.DecisionDismiss, "no", domain.CodeReviewDecisionInvalid},
		{"send back anything", e.fact, domain.DecisionSendBack, "", ""},
		{"a note over 500 characters", e.claim, domain.DecisionDismiss, strings.Repeat("x", 501), domain.CodeReviewDecisionInvalid},
		{"an item that is not in the review", "ffffffffffffffff", domain.DecisionSendBack, "", domain.CodeReviewDecisionInvalid},
		{"a decision that does not exist", e.claim, domain.ReviewDecision("shrug"), "", domain.CodeReviewDecisionInvalid},
	}
	for _, c := range cases {
		res, err := e.decide(t, c.item, c.d, c.note)
		if c.code == "" {
			if err != nil || !res.Decision.Active || res.Decision.ItemKey != c.item {
				t.Errorf("%s: %+v %v", c.name, res, err)
			}
			continue
		}
		if !codeIs(err, c.code) {
			t.Errorf("%s: %v, want %s", c.name, err, c.code)
		}
	}
}

func TestDecide_OneVersionOneWinner(t *testing.T) {
	e := newDecideEnv(t)
	e.refresh(t)
	in := DecideInput{RequestID: e.req.ID, ReviewID: e.review.Stored.ID, ItemKey: e.breaking, Decision: domain.DecisionSendBack, ExpectedVersion: e.review.Stored.Version, DecidedBy: uuid.NewString()}
	if _, err := e.decider.Decide(e.ctx, in); err != nil {
		t.Fatal(err)
	}
	if _, err := e.decider.Decide(e.ctx, in); !codeIs(err, domain.CodeReviewStaleRevision) {
		t.Errorf("the second person with the same version loses: %v", err)
	}
	if got, _ := e.decisions.InForce(e.ctx, e.review.Stored.ID); len(got) != 1 {
		t.Errorf("only the winner is recorded: %+v", got)
	}
}

func TestApprove_OnlyWhenNothingIsOpenAndTheServerSaysSo(t *testing.T) {
	e := newDecideEnv(t)
	e.refresh(t)
	approve := func() (StoredReview, error) {
		e.refresh(t)
		return e.decider.Approve(e.ctx, ApproveInput{RequestID: e.req.ID, ReviewID: e.review.Stored.ID, Note: "ok", ExpectedVersion: e.review.Stored.Version, ApprovedBy: uuid.NewString()})
	}
	if _, err := approve(); !codeIs(err, domain.CodeReviewDecisionsOpen) || !strings.Contains(err.Error(), "1 decision") {
		t.Fatalf("a call that skips the screen is still refused: %v", err)
	}
	if _, err := e.decide(t, e.breaking, domain.DecisionSendBack, ""); err != nil {
		t.Fatal(err)
	}
	got, err := approve()
	if err != nil || got.State != domain.ReviewStateApproved || got.ApprovedBy == "" || got.ApprovalNote != "ok" || got.ApprovedAt == nil {
		t.Fatalf("approve: %+v %v", got, err)
	}
	if e.req.Status != domain.RequestStatusCompleted {
		t.Error("an approval changes no Request status")
	}
	// A stale version cannot approve.
	if _, err := e.decider.Approve(e.ctx, ApproveInput{RequestID: e.req.ID, ReviewID: got.ID, ExpectedVersion: got.Version - 1, ApprovedBy: uuid.NewString()}); !codeIs(err, domain.CodeReviewStaleRevision) {
		t.Errorf("stale version: %v", err)
	}
	if _, err := e.decider.Approve(e.ctx, ApproveInput{RequestID: e.req.ID, ReviewID: got.ID, Note: strings.Repeat("n", 501), ExpectedVersion: got.Version, ApprovedBy: uuid.NewString()}); !codeIs(err, domain.CodeReviewDecisionInvalid) {
		t.Errorf("a long note: %v", err)
	}
	// Deciding again after the approval takes the review back to ready: the person changed their mind.
	back, err := e.decide(t, e.breaking, domain.DecisionAcceptException, "after all it is fine")
	if err != nil || back.Review.State != domain.ReviewStateReady || back.Review.ApprovedBy != "" {
		t.Errorf("a decision after the approval clears it: %+v %v", back.Review, err)
	}
}

// A review whose newest agent run failed holds only measured facts: it cannot be approved until a run succeeds.
func TestApprove_RefusedWhileTheNewestContextRunFailed(t *testing.T) {
	e := newDecideEnv(t)
	e.svc.Runs = e.runs
	if _, err := e.decide(t, e.breaking, domain.DecisionSendBack, ""); err != nil {
		t.Fatal(err)
	}
	endRun := func(status ReviewRunStatus, code string) {
		t.Helper()
		run, err := e.svc.Runs.Start(e.ctx, ReviewRun{ID: uuid.NewString(), ReviewID: e.review.Stored.ID, RequestID: e.req.ID, Kind: ReviewRunContext, Status: ReviewRunRunning})
		if err != nil {
			t.Fatal(err)
		}
		if err := e.svc.Runs.Finish(e.ctx, run.ID, status, code, ""); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Millisecond)
	}
	approve := func() error {
		e.refresh(t)
		_, err := e.decider.Approve(e.ctx, ApproveInput{RequestID: e.req.ID, ReviewID: e.review.Stored.ID, ExpectedVersion: e.review.Stored.Version, ApprovedBy: uuid.NewString()})
		return err
	}
	endRun(ReviewRunFailed, domain.CodeReviewResultInvalid)
	if err := approve(); !codeIs(err, domain.CodeReviewContextFailed) {
		t.Fatalf("a failed agent run must block the approval: %v", err)
	}
	endRun(ReviewRunSucceeded, "")
	if err := approve(); err != nil {
		t.Fatalf("a later run that succeeded lifts the block: %v", err)
	}
}

func TestDecideAndApprove_NotBeforeCodeAndNotOnAStaleReview(t *testing.T) {
	d := newDrawEnv(t, domain.RequestStatusExecuting) // a design review
	d.ctx = tenant.WithRole(tenant.WithUserID(d.ctx, uuid.NewString()), "admin")
	dec := &ReviewDecider{Service: d.svc, Decisions: &memDecisions{}}
	view, _, _ := d.svc.Latest(d.ctx, d.req.ID)
	if _, err := dec.Decide(d.ctx, DecideInput{RequestID: d.req.ID, ReviewID: view.Stored.ID, ItemKey: "x", Decision: domain.DecisionSendBack, ExpectedVersion: view.Stored.Version, DecidedBy: uuid.NewString()}); !codeIs(err, domain.CodeReviewNotReady) {
		t.Errorf("decide before code: %v", err)
	}
	if _, err := dec.Approve(d.ctx, ApproveInput{RequestID: d.req.ID, ReviewID: view.Stored.ID, ExpectedVersion: view.Stored.Version, ApprovedBy: uuid.NewString()}); !codeIs(err, domain.CodeReviewNotReady) {
		t.Errorf("approve before code: %v", err)
	}

	e := newDecideEnv(t)
	e.refresh(t)
	stale, err := e.svc.Reviews.SetState(e.ctx, e.review.Stored.ID, domain.ReviewStateStale, e.review.Stored.Version)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.decider.Decide(e.ctx, DecideInput{RequestID: e.req.ID, ReviewID: stale.ID, ItemKey: e.breaking, Decision: domain.DecisionSendBack, ExpectedVersion: stale.Version, DecidedBy: uuid.NewString()}); !codeIs(err, domain.CodeReviewStaleRevision) {
		t.Errorf("decide on a stale review: %v", err)
	}
	if _, err := e.decider.Approve(e.ctx, ApproveInput{RequestID: e.req.ID, ReviewID: stale.ID, ExpectedVersion: stale.Version, ApprovedBy: uuid.NewString()}); !codeIs(err, domain.CodeReviewStaleRevision) {
		t.Errorf("approve a stale review: %v", err)
	}
}

func TestDecisions_SurviveARegenerationAndOrphansAreKept(t *testing.T) {
	e := newDecideEnv(t)
	if _, err := e.decide(t, e.breaking, domain.DecisionAcceptException, "ADR-09"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.decide(t, e.claim, domain.DecisionDismiss, "wrong"); err != nil {
		t.Fatal(err)
	}
	e.refresh(t)
	approved, err := e.decider.Approve(e.ctx, ApproveInput{RequestID: e.req.ID, ReviewID: e.review.Stored.ID, ExpectedVersion: e.review.Stored.Version, ApprovedBy: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}

	// Regenerate the context: the same breaking item comes back, the claim is gone. Nothing new asks for a decision, so the review
	// stays approved and the claim's decision is an orphan that is still there.
	same := withDesign(deliveredResultWithItems())
	same.Items = same.Items[:1]
	e.agent.replies = []func(AgentPromptInput) (AgentPromptResult, error){agentReply(same)}
	if _, err := e.runner.Start(e.ctx, StartContextInput{RequestID: e.req.ID, Force: true}); err != nil {
		t.Fatal(err)
	}
	e.refresh(t)
	if e.review.Stored.ID != approved.ID || e.review.Stored.State != domain.ReviewStateApproved {
		t.Fatalf("nothing new to decide: the review stays approved: %+v", e.review.Stored)
	}
	list, err := e.decider.List(e.ctx, e.req.ID, e.review.Stored.ID)
	if err != nil || len(list.InForce) != 2 || list.Open != 0 {
		t.Fatalf("list: %+v %v", list, err)
	}
	for _, d := range list.InForce {
		if (d.ItemKey == e.claim) != d.Orphaned {
			t.Errorf("only the decision about the vanished item is an orphan: %+v", d)
		}
	}

	// Now the regeneration brings a new item that asks for a decision: the approval goes, with a warning.
	more := withDesign(deliveredResultWithItems())
	more.Items = append(more.Items[:1], domain.AgentReviewItem{Subject: "second-breaking", Section: domain.SectionContracts, Change: domain.ChangeRemoved, Title: "A channel was removed",
		Severity: domain.SeverityBreaking, NeedsDecision: true, Evidence: []domain.ReviewEvidence{{Kind: domain.EvidenceFile, Ref: "backend-go/a.go"}}})
	e.agent.replies = []func(AgentPromptInput) (AgentPromptResult, error){agentReply(more)}
	if _, err := e.runner.Start(e.ctx, StartContextInput{RequestID: e.req.ID, Force: true}); err != nil {
		t.Fatal(err)
	}
	e.refresh(t)
	if e.review.Stored.State != domain.ReviewStateReady || e.review.Stored.ApprovedBy != "" || !containsString(e.review.Review.Warnings, "new_decisions_required") {
		t.Errorf("a new decision is required: state=%s warnings=%v", e.review.Stored.State, e.review.Review.Warnings)
	}
	list, _ = e.decider.List(e.ctx, e.req.ID, e.review.Stored.ID)
	if list.Open != 1 {
		t.Errorf("open = %d", list.Open)
	}
}

func TestDecide_AuditNeverHoldsTheNoteAndTenantsAreKeptApart(t *testing.T) {
	e := newDecideEnv(t)
	if _, err := e.decide(t, e.breaking, domain.DecisionAcceptException, "SECRET-NOTE-TEXT"); err != nil {
		t.Fatal(err)
	}
	e.refresh(t)
	if _, err := e.decider.Approve(e.ctx, ApproveInput{RequestID: e.req.ID, ReviewID: e.review.Stored.ID, Note: "SECRET-APPROVAL-NOTE", ExpectedVersion: e.review.Stored.Version, ApprovedBy: uuid.NewString()}); err != nil {
		t.Fatal(err)
	}
	var actions []string
	for _, ev := range e.audit.events {
		if ev.Action == domain.AuditReviewDecide || ev.Action == domain.AuditReviewApprove {
			actions = append(actions, ev.Action)
			if ev.TargetID != e.req.ID || ev.Metadata["review_id"] == nil {
				t.Errorf("event: %+v", ev)
			}
		}
		for _, v := range ev.Metadata {
			if s, ok := v.(string); ok && strings.Contains(s, "SECRET") {
				t.Errorf("an audit entry holds a note: %+v", ev)
			}
		}
	}
	if len(actions) != 2 {
		t.Errorf("actions: %v", actions)
	}

	if _, err := e.decider.List(context.Background(), e.req.ID, e.review.Stored.ID); err == nil {
		t.Error("a call without a tenant must fail")
	}
	if _, err := e.decider.List(e.ctx, uuid.NewString(), e.review.Stored.ID); !codeIs(err, domain.CodeReviewNotFound) {
		t.Errorf("a review is read through its own Request: %v", err)
	}
}

func TestDecide_SendBackIsLinkedOnlyToAChildOfTheRequest(t *testing.T) {
	e := newDecideEnv(t)
	if _, err := e.decide(t, e.breaking, domain.DecisionSendBack, "please fix"); err != nil {
		t.Fatal(err)
	}
	e.refresh(t)
	child := uuid.NewString()
	e.decider.Links = memLinks{children: map[string][]string{e.req.ID: {child}}}
	if _, err := e.decider.LinkChild(e.ctx, e.req.ID, e.review.Stored.ID, []string{e.breaking}, uuid.NewString()); !codeIs(err, domain.CodeReviewDecisionInvalid) {
		t.Errorf("a Request that is not a child: %v", err)
	}
	n, err := e.decider.LinkChild(e.ctx, e.req.ID, e.review.Stored.ID, []string{e.breaking, "ffffffffffffffff"}, child)
	if err != nil || n != 1 {
		t.Fatalf("link: %d %v", n, err)
	}
	list, _ := e.decider.List(e.ctx, e.req.ID, e.review.Stored.ID)
	linked := ""
	for _, d := range list.InForce {
		if d.ItemKey == e.breaking {
			linked = d.ChildRequestID
		}
	}
	if linked != child {
		t.Errorf("linked to %q", linked)
	}
	hist, err := e.decider.History(e.ctx, e.req.ID, e.review.Stored.ID, e.breaking)
	if err != nil || len(hist) != 1 {
		t.Errorf("history: %+v %v", hist, err)
	}
}

func TestDecide_OnlyTheReporterOrAnAdminMayDecideAndApprove(t *testing.T) {
	e := newDecideEnv(t)
	e.refresh(t)
	in := DecideInput{RequestID: e.req.ID, ReviewID: e.review.Stored.ID, ItemKey: e.breaking, Decision: domain.DecisionSendBack, ExpectedVersion: e.review.Stored.Version, DecidedBy: uuid.NewString()}

	member := tenant.WithRole(tenant.WithUserID(tenant.WithTenantID(context.Background(), e.req.TenantID), uuid.NewString()), "member")
	if _, err := e.decider.Decide(member, in); !codeIs(err, "REQUEST_REVIEW_FORBIDDEN") {
		t.Errorf("a member who is not the reporter: %v", err)
	}
	if _, err := e.decider.Approve(member, ApproveInput{RequestID: e.req.ID, ReviewID: in.ReviewID, ExpectedVersion: in.ExpectedVersion, ApprovedBy: uuid.NewString()}); !codeIs(err, "REQUEST_REVIEW_FORBIDDEN") {
		t.Errorf("approve by a member: %v", err)
	}
	if _, err := e.decider.LinkChild(member, e.req.ID, in.ReviewID, []string{e.breaking}, uuid.NewString()); !codeIs(err, "REQUEST_REVIEW_FORBIDDEN") {
		t.Errorf("link by a member: %v", err)
	}
	if got, _ := e.decisions.InForce(e.ctx, in.ReviewID); len(got) != 0 {
		t.Errorf("nothing was recorded: %+v", got)
	}
	// Reading is open to whoever can see the Request.
	if _, err := e.decider.List(member, e.req.ID, in.ReviewID); err != nil {
		t.Errorf("a member reads: %v", err)
	}
	reporter := tenant.WithRole(tenant.WithUserID(tenant.WithTenantID(context.Background(), e.req.TenantID), e.req.ReporterID), "member")
	e.req.ReporterID = uuid.NewString()
	e.setStatus(domain.RequestStatusCompleted)
	reporter = tenant.WithUserID(reporter, e.req.ReporterID)
	if _, err := e.decider.Decide(reporter, in); err != nil {
		t.Errorf("the reporter decides: %v", err)
	}
}

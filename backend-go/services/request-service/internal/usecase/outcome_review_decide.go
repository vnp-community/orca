package usecase

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ReviewDecider records what people decide about a review and approves it. The rules live here, on the server: a client that skips
// the screen and calls the API gets the same answers.
type ReviewDecider struct {
	Service   *OutcomeReviewService
	Decisions OutcomeReviewDecisionRepository
	Links     RequestLinkRepository
}

// DecideInput is one decision. ExpectedVersion is the version of the review the person was looking at.
type DecideInput struct {
	RequestID       string
	ReviewID        string
	ItemKey         string
	Decision        domain.ReviewDecision
	Note            string
	ExpectedVersion int64
	DecidedBy       string
}

// DecideResult is the decision and the review as it is after it.
type DecideResult struct {
	Decision domain.ItemDecision
	Review   StoredReview
	Open     int
}

func (d *ReviewDecider) review(ctx context.Context, requestID, reviewID string) (StoredReview, domain.OutcomeReview, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return StoredReview{}, domain.OutcomeReview{}, domain.ErrRequestTenantRequired()
	}
	st, err := d.Service.Reviews.Get(ctx, reviewID)
	if err != nil {
		return StoredReview{}, domain.OutcomeReview{}, err
	}
	if st.RequestID != requestID {
		return StoredReview{}, domain.OutcomeReview{}, domain.ErrReviewNotFound(reviewID) // a review is read through its own Request
	}
	view, err := decodeReview(st)
	if err != nil {
		return StoredReview{}, domain.OutcomeReview{}, err
	}
	view.Review.State = st.State // the row says whether it is stale or replaced; the stored model was written before
	return st, view.Review, nil
}

// authorize is the Go side of the "decide" door: the reporter of the Request, or a tenant admin, lead or owner. The policy file lets any
// person through to this point because who may decide depends on the Request.
func (d *ReviewDecider) authorize(ctx context.Context, requestID string) error {
	req, err := d.Service.Requests.Get(ctx, requestID)
	if err != nil {
		return err
	}
	if rerr := reporterOrAdmin(ctx, req); rerr != nil {
		return domain.ErrReviewForbidden()
	}
	return nil
}

// Decide records the decision when the review is the version the person saw. A decision on an approved review takes it back to ready:
// what was approved is no longer what the person now thinks.
func (d *ReviewDecider) Decide(ctx context.Context, in DecideInput) (DecideResult, error) {
	st, review, err := d.review(ctx, in.RequestID, in.ReviewID)
	if err != nil {
		return DecideResult{}, err
	}
	if err := d.authorize(ctx, in.RequestID); err != nil {
		return DecideResult{}, err
	}
	if st.Version != in.ExpectedVersion {
		return DecideResult{}, domain.ErrReviewStaleRevision()
	}
	if _, err := domain.CheckDecision(review, in.ItemKey, in.Decision, in.Note); err != nil {
		return DecideResult{}, err
	}
	next := st.State
	if next == domain.ReviewStateApproved {
		next = domain.ReviewStateReady
	}
	// The write that moves the version comes first, so two people deciding at once cannot both win.
	bumped, err := d.Service.Reviews.SetState(ctx, st.ID, next, st.Version)
	if err != nil {
		return DecideResult{}, err
	}
	rec, err := d.Decisions.Record(ctx, domain.ItemDecision{
		ReviewID: st.ID, RequestID: st.RequestID, ItemKey: in.ItemKey, Decision: in.Decision, Note: strings.TrimSpace(in.Note), DecidedBy: in.DecidedBy,
	})
	if err != nil {
		return DecideResult{}, err
	}
	inForce, err := d.Decisions.InForce(ctx, st.ID)
	if err != nil {
		return DecideResult{}, err
	}
	d.audit(ctx, domain.AuditReviewDecide, st, map[string]any{"item_key": in.ItemKey, "decision": string(in.Decision), "has_note": strings.TrimSpace(in.Note) != ""})
	return DecideResult{Decision: rec, Review: bumped, Open: len(domain.OpenDecisions(review, inForce))}, nil
}

// ApproveInput approves the review the person saw.
type ApproveInput struct {
	RequestID       string
	ReviewID        string
	Note            string
	ExpectedVersion int64
	ApprovedBy      string
}

// Approve marks the review approved when nothing is waiting for a decision. It changes no Request status and opens no v6 Approval:
// it says only that a person looked at the architecture and the flow and accepted what is left.
func (d *ReviewDecider) Approve(ctx context.Context, in ApproveInput) (StoredReview, error) {
	st, review, err := d.review(ctx, in.RequestID, in.ReviewID)
	if err != nil {
		return StoredReview{}, err
	}
	if err := d.authorize(ctx, in.RequestID); err != nil {
		return StoredReview{}, err
	}
	if review.Phase != domain.ReviewPhaseDelivered {
		return StoredReview{}, domain.ErrReviewPhaseNotReady(review.Phase, "an approval")
	}
	if st.State == domain.ReviewStateStale || st.State == domain.ReviewStateSuperseded {
		return StoredReview{}, domain.ErrReviewStaleRevision()
	}
	if st.Version != in.ExpectedVersion {
		return StoredReview{}, domain.ErrReviewStaleRevision()
	}
	note := strings.TrimSpace(in.Note)
	if utf8.RuneCountInString(note) > domain.MaxDecisionNoteRunes {
		return StoredReview{}, domain.ErrReviewDecisionInvalid("the note is longer than 500 characters")
	}
	inForce, err := d.Decisions.InForce(ctx, st.ID)
	if err != nil {
		return StoredReview{}, err
	}
	if open := domain.OpenDecisions(review, inForce); len(open) > 0 {
		return StoredReview{}, domain.ErrReviewDecisionsOpen(len(open))
	}
	approved, err := d.Service.Reviews.Approve(ctx, st.ID, in.ApprovedBy, note, st.Version)
	if err != nil {
		return StoredReview{}, err
	}
	d.audit(ctx, domain.AuditReviewApprove, st, map[string]any{"has_note": note != "", "inputs_digest": st.InputsDigest})
	return approved, nil
}

// Decisions are the decisions in force for the review with the orphaned ones marked, and how many items still wait.
type Decisions struct {
	InForce []domain.ItemDecision
	Open    int
}

// List returns what a screen needs to show the decisions and the bar.
func (d *ReviewDecider) List(ctx context.Context, requestID, reviewID string) (Decisions, error) {
	st, review, err := d.review(ctx, requestID, reviewID)
	if err != nil {
		return Decisions{}, err
	}
	inForce, err := d.Decisions.InForce(ctx, st.ID)
	if err != nil {
		return Decisions{}, err
	}
	open := 0
	if review.Phase == domain.ReviewPhaseDelivered {
		open = len(domain.OpenDecisions(review, inForce))
	}
	return Decisions{InForce: domain.MarkOrphans(review, inForce), Open: open}, nil
}

// History is every decision about one item, newest first.
func (d *ReviewDecider) History(ctx context.Context, requestID, reviewID, itemKey string) ([]domain.ItemDecision, error) {
	st, _, err := d.review(ctx, requestID, reviewID)
	if err != nil {
		return nil, err
	}
	return d.Decisions.History(ctx, st.ID, itemKey)
}

// LinkChild records which Request took the items that were sent back. The Request has to be a child of the review's own Request:
// a person cannot attach someone else's Request to a review.
func (d *ReviewDecider) LinkChild(ctx context.Context, requestID, reviewID string, itemKeys []string, childRequestID string) (int, error) {
	st, _, err := d.review(ctx, requestID, reviewID)
	if err != nil {
		return 0, err
	}
	if err := d.authorize(ctx, requestID); err != nil {
		return 0, err
	}
	if d.Links == nil {
		return 0, domain.ErrReviewDecisionInvalid("links are not available")
	}
	children, err := d.Links.ListChildren(ctx, st.RequestID)
	if err != nil {
		return 0, err
	}
	ok := false
	for _, c := range children {
		ok = ok || c.ChildRequestID == childRequestID
	}
	if !ok {
		return 0, domain.ErrReviewDecisionInvalid("the Request is not a child of this review's Request")
	}
	return d.Decisions.LinkChild(ctx, st.ID, itemKeys, childRequestID)
}

func (d *ReviewDecider) audit(ctx context.Context, action string, st StoredReview, meta map[string]any) {
	if d.Service.Audit == nil {
		return
	}
	meta["review_id"], meta["phase"] = st.ID, string(st.Phase)
	_ = d.Service.Audit.Record(ctx, AuditEvent{Action: action, TargetType: "request", TargetID: st.RequestID, RequestID: st.RequestID, Outcome: "allowed", Metadata: meta})
}

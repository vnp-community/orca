package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ApplyContext rebuilds the review from the facts it was made from and a checked context result, and stores it. The facts are
// the ones Generate collected: a context run never reads them again, so the review keeps the digest it was found by.
func (s *OutcomeReviewService) ApplyContext(ctx context.Context, req domain.Request, col Collected, st StoredReview, vc domain.ValidatedContext, contextDigest, designDigest string) (ReviewView, error) {
	refs := make([]domain.DiagramRef, 0, len(vc.Diagrams))
	for _, m := range vc.Diagrams {
		refs = append(refs, m.Ref())
	}
	built, err := domain.BuildOutcomeReview(req.ID, domain.ReviewBuildInput{
		Facts: col.Facts, AgentItems: vc.Items, KG: vc.KG, Diagrams: refs, Arch: vc.Arch, FlowVerdict: vc.Flow,
		ExtraWarnings: vc.Warnings, Redact: s.Redact,
	})
	if err != nil {
		return ReviewView{}, err
	}
	model, err := encodeReviewModel(&built)
	if err != nil {
		return ReviewView{}, err
	}
	row := st
	row.Model, row.InputsDigest, row.State = model, built.InputsDigest, domain.ReviewStateReady
	row.KGSnapshot, row.DesignDigest, row.ContextDigest = vc.KG.Snapshot, designDigest, contextDigest
	updated, err := s.updateWithReload(ctx, row)
	if err != nil {
		return ReviewView{}, err
	}
	return decodeReview(updated)
}

// MarkContextFailed records that the context run did not produce a usable result. The review keeps the facts Orca measured.
func (s *OutcomeReviewService) MarkContextFailed(ctx context.Context, reviewID string) error {
	return s.MarkContextWarning(ctx, reviewID, "context_failed")
}

// MarkContextWarning adds a warning to the stored review once; the facts and the digests stay as they are.
func (s *OutcomeReviewService) MarkContextWarning(ctx context.Context, reviewID, warning string) error {
	st, err := s.Reviews.Get(ctx, reviewID)
	if err != nil {
		return err
	}
	var model domain.OutcomeReview
	if err := json.Unmarshal(st.Model, &model); err != nil {
		return err
	}
	for _, w := range model.Warnings {
		if w == warning {
			return nil
		}
	}
	model.Warnings = append(dedupeSortedStrings(model.Warnings), warning)
	raw, err := encodeReviewModel(&model)
	if err != nil {
		return err
	}
	st.Model = raw
	_, err = s.updateWithReload(ctx, st)
	return err
}

// updateWithReload writes the row under its version and, when another writer got in first, applies the same change to the newer row once.
func (s *OutcomeReviewService) updateWithReload(ctx context.Context, row StoredReview) (StoredReview, error) {
	out, err := s.Reviews.Update(ctx, row, row.Version)
	if err == nil || domainCode(err) != domain.CodeReviewStaleRevision {
		return out, err
	}
	cur, gerr := s.Reviews.Get(ctx, row.ID)
	if gerr != nil {
		return StoredReview{}, errors.Join(err, gerr)
	}
	row.Version, row.CreatedAt, row.UpdatedAt = cur.Version, cur.CreatedAt, cur.UpdatedAt
	return s.Reviews.Update(ctx, row, cur.Version)
}

func dedupeSortedStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in)+1)
	for _, v := range in {
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

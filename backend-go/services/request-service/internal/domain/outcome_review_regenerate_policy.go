package domain

import (
	"fmt"
	"unicode/utf8"
)

// RegenScope says how much of a review a person asks to be made again.
type RegenScope string

const (
	// RegenArtifact draws the diagrams of the sections again from the models the review already has.
	RegenArtifact RegenScope = "artifact"
	// RegenContext reads the graph and the code again; only the diagrams whose model changed are drawn again.
	RegenContext RegenScope = "context"
	// RegenAll does the context and then draws every diagram again.
	RegenAll RegenScope = "all"

	MaxRegenNoteRunes = 200
)

func (s RegenScope) Valid() bool { return s == RegenArtifact || s == RegenContext || s == RegenAll }

// RunsContext and Draws say what a scope does.
func (s RegenScope) RunsContext() bool { return s == RegenContext || s == RegenAll }
func (s RegenScope) Draws() bool       { return s == RegenArtifact || s == RegenAll }

// RegenReason is why a person regenerates. It is required: a regeneration costs an agent run and replaces what others may have read.
type RegenReason string

const (
	RegenLooksWrong   RegenReason = "looks_wrong"
	RegenKGChanged    RegenReason = "kg_changed"
	RegenCodeChanged  RegenReason = "code_changed"
	RegenStyleChanged RegenReason = "style_changed"
	RegenRetryFailed  RegenReason = "retry_failed"
	RegenOther        RegenReason = "other"
)

func (r RegenReason) Valid() bool {
	switch r {
	case RegenLooksWrong, RegenKGChanged, RegenCodeChanged, RegenStyleChanged, RegenRetryFailed, RegenOther:
		return true
	}
	return false
}

// RegenRequest is what a person asked for.
type RegenRequest struct {
	Scope    RegenScope
	Sections []ReviewSection
	Reason   RegenReason
	Note     string
}

// Validate refuses a request that cannot be carried out. An artifact regeneration has to name the sections it draws.
func (r RegenRequest) Validate() error {
	switch {
	case !r.Scope.Valid():
		return ErrReviewDecisionInvalid("scope must be artifact, context or all")
	case !r.Reason.Valid():
		return ErrReviewDecisionInvalid("a reason is required: looks_wrong, kg_changed, code_changed, style_changed, retry_failed or other")
	case utf8.RuneCountInString(r.Note) > MaxRegenNoteRunes:
		return ErrReviewDecisionInvalid("the note is longer than 200 characters")
	case r.Scope == RegenArtifact && len(r.Sections) == 0:
		return ErrReviewDecisionInvalid("an artifact regeneration needs at least one section")
	}
	for _, s := range r.Sections {
		if !s.Drawn() {
			return ErrReviewDecisionInvalid(fmt.Sprintf("section %q is not drawn", s))
		}
	}
	return nil
}

// RegenBudget is what a Request and a tenant have used.
type RegenBudget struct {
	Limit          int
	UsedToday      int
	ActiveInTenant int
	MaxConcurrent  int
}

// Check refuses a regeneration over the day's limit for the Request or over the runs the tenant may have going at once.
func (b RegenBudget) Check() error {
	if b.UsedToday >= b.Limit {
		return ErrReviewBudgetExceeded(fmt.Sprintf("this Request was regenerated %d times in the last day; the limit is %d", b.UsedToday, b.Limit))
	}
	if b.MaxConcurrent > 0 && b.ActiveInTenant >= b.MaxConcurrent {
		return ErrReviewBudgetExceeded(fmt.Sprintf("%d review runs are already going for this tenant; the limit is %d", b.ActiveInTenant, b.MaxConcurrent))
	}
	return nil
}

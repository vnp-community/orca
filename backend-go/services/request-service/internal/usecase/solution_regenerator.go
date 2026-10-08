package usecase

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// SolutionRegenerator is the AnalysisStarter behind ResumeAfterClarification: it starts the run that the
// answered clarification was holding back. A missing dev server is a skip, not a failure, so the event is not
// retried forever; the person starts the run again from the UI.
type SolutionRegenerator struct {
	generate *GenerateSolution
}

var (
	_ AnalysisStarter         = (*SolutionRegenerator)(nil)
	_ DeferredAnalysisStarter = (*SolutionRegenerator)(nil)
)

func NewSolutionRegenerator(g *GenerateSolution) *SolutionRegenerator {
	return &SolutionRegenerator{generate: g}
}

// StartSolution is the immediate form (spawns at once); the resume consumer uses StartSolutionDeferred.
func (r *SolutionRegenerator) StartSolution(ctx context.Context, requestID, feedback, idempotencyKey string) error {
	_, err := r.generate.ExecuteForSystem(ctx, GenerateSolutionInput{RequestID: requestID, Feedback: feedback, IdempotencyKey: idempotencyKey})
	return skipWithoutConnection(err)
}

func (r *SolutionRegenerator) StartSolutionDeferred(ctx context.Context, requestID, feedback, idempotencyKey string) (func(), error) {
	_, after, err := r.generate.PrepareForSystem(ctx, GenerateSolutionInput{RequestID: requestID, Feedback: feedback, IdempotencyKey: idempotencyKey})
	return after, skipWithoutConnection(err)
}

func skipWithoutConnection(err error) error {
	switch {
	case err == nil:
		return nil
	case errorHasCode(err, "REQUEST_SOLUTION_NO_CONNECTION"), errorHasCode(err, "REQUEST_ANALYSIS_NO_CONNECTION"),
		errorHasCode(err, "REQUEST_ANALYSIS_NO_REPO_PATH"), errorHasCode(err, "REQUEST_SOLUTION_WRONG_STATE"):
		return fmt.Errorf("%w: %v", ErrResumeSkipped, err)
	}
	return err
}

// ProposedSolutionSuperseder retires what the answer made stale; approved Solutions are left alone by SupersedeOpen.
type ProposedSolutionSuperseder struct {
	solutions SolutionStore
}

var _ SolutionSuperseder = (*ProposedSolutionSuperseder)(nil)

func NewProposedSolutionSuperseder(s SolutionStore) *ProposedSolutionSuperseder {
	return &ProposedSolutionSuperseder{solutions: s}
}

func (p *ProposedSolutionSuperseder) SupersedeProposedByRequest(ctx context.Context, requestID string) error {
	for _, kind := range []domain.SolutionKind{domain.SolutionKindSolution, domain.SolutionKindDiagnosis, domain.SolutionKindFindings, domain.SolutionKindAnswer} {
		if _, err := p.solutions.SupersedeOpen(ctx, requestID, kind, ""); err != nil {
			return err
		}
	}
	return nil
}

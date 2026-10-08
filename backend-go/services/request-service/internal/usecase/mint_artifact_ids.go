package usecase

import (
	"context"
	"strconv"
	"strings"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

const maxSeqAttempts = 3

// MintArtifactIDs hands out display ids and records them in artifact_index. It runs inside the transaction
// that creates the entity, so an id never survives a rollback.
type MintArtifactIDs struct {
	index ArtifactIndexRepository
}

func NewMintArtifactIDs(index ArtifactIndexRepository) *MintArtifactIDs {
	return &MintArtifactIDs{index: index}
}

// MintRequestID indexes REQ-<number>; it is idempotent.
func (m *MintArtifactIDs) MintRequestID(ctx context.Context, r domain.Request) error {
	_, err := m.index.Insert(ctx, domain.IndexEntry{
		DisplayID: domain.FormatRequestID(r.Number), Kind: domain.DisplayKindRequest, RequestID: r.ID, ArtifactID: r.ID,
	})
	return err
}

// MintSolutionID gives sol a seq (keeping one it already has), then indexes the solution and each option of its document.
func (m *MintArtifactIDs) MintSolutionID(ctx context.Context, requestNumber int64, sol *domain.Solution) error {
	if sol.Seq == 0 {
		var err error
		for attempt := 0; attempt < maxSeqAttempts; attempt++ {
			var seq int
			if seq, err = m.index.NextSolutionSeq(ctx, sol.RequestID); err != nil {
				return err
			}
			if err = m.index.SetSolutionSeq(ctx, sol.ID, seq); err == nil {
				sol.Seq = seq
				break
			}
			if !isSeqConflict(err) {
				return err
			}
		}
		if sol.Seq == 0 {
			return err
		}
	}
	id := domain.FormatSolutionID(requestNumber, sol.Seq)
	if _, err := m.index.Insert(ctx, domain.IndexEntry{DisplayID: id, Kind: domain.DisplayKindSolution, RequestID: sol.RequestID, ArtifactID: sol.ID}); err != nil {
		return err
	}
	if len(sol.OptionsJSON) == 0 {
		return nil
	}
	view, err := domain.ParseSolutionCoverage(sol.OptionsJSON)
	if err != nil {
		return nil // a document that is not an options document has no options to index
	}
	for _, optID := range view.OptionIDs {
		n, err := strconv.Atoi(strings.TrimPrefix(optID, "opt-"))
		if err != nil || n < 1 {
			continue
		}
		if _, err := m.index.Insert(ctx, domain.IndexEntry{DisplayID: domain.FormatOptionID(id, n), Kind: domain.DisplayKindOption, RequestID: sol.RequestID, ArtifactID: sol.ID}); err != nil {
			return err
		}
	}
	return nil
}

func isSeqConflict(err error) bool { return errorHasCode(err, "REQUEST_ARTIFACT_SEQ_CONFLICT") }

// PlanPhaseIDs and PlanTreeIDs are the UUIDs of a committed plan tree, in tree order.
type PlanPhaseIDs struct {
	TaskID string
	Tasks  []string
}

type PlanTreeIDs struct {
	PlanTaskID string
	Phases     []PlanPhaseIDs
	// Tasks are the tasks that hang directly under the plan.
	Tasks []string
}

// MintPlanIDs indexes PLN-, PH- and TSK- ids for a plan tree. TSK numbers follow tree order (phases first, then
// direct tasks); they do not replace task-service's TaskNumber. It returns the plan's seq.
func (m *MintArtifactIDs) MintPlanIDs(ctx context.Context, requestNumber int64, requestID string, tree PlanTreeIDs) (int, error) {
	planSeq, err := m.index.NextPlanSeq(ctx, requestID)
	if err != nil {
		return 0, err
	}
	put := func(display string, kind domain.DisplayKind, artifactID string) error {
		_, err := m.index.Insert(ctx, domain.IndexEntry{DisplayID: display, Kind: kind, RequestID: requestID, ArtifactID: artifactID})
		return err
	}
	if err := put(domain.FormatPlanID(requestNumber, planSeq), domain.DisplayKindPlan, tree.PlanTaskID); err != nil {
		return 0, err
	}
	taskN := 0
	for i, ph := range tree.Phases {
		if err := put(domain.FormatPhaseID(requestNumber, planSeq, i+1), domain.DisplayKindPhase, ph.TaskID); err != nil {
			return 0, err
		}
		for _, t := range ph.Tasks {
			taskN++
			if err := put(domain.FormatTaskID(requestNumber, planSeq, taskN), domain.DisplayKindTask, t); err != nil {
				return 0, err
			}
		}
	}
	for _, t := range tree.Tasks {
		taskN++
		if err := put(domain.FormatTaskID(requestNumber, planSeq, taskN), domain.DisplayKindTask, t); err != nil {
			return 0, err
		}
	}
	return planSeq, nil
}

package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// SolutionArtifacts ties a generated Solution to the artifact model (CR-REQ-027): requirement coverage is
// checked before the document is accepted, provenance and revision are stamped before it is stored, and the
// display ids and relations are written in the same transaction.
type SolutionArtifacts interface {
	// CheckCoverage reports the semantic violations of a solution document against the Request's active ACs.
	CheckCoverage(req domain.Request, doc []byte) error
	// CheckChosen is the approval-time rule: the chosen option answers every active AC.
	CheckChosen(req domain.Request, doc []byte, optionID string) error
	Annotate(req domain.Request, run domain.AnalysisRun, sol *domain.Solution) error
	// Record indexes SOL-n.s and its options and writes derived_from and supersedes edges; it joins the ctx transaction.
	Record(ctx context.Context, req domain.Request, sol domain.Solution, run domain.AnalysisRun, superseded []string) error
}

type SolutionArtifactRecorder struct {
	mint      *MintArtifactIDs
	relations ArtifactRelationRepository
	decisions DecisionRepository
	clock     func() time.Time
}

// WithDecisions lets a retired Solution take its live Decision with it, so a stale choice cannot gate a new proposal.
func (r *SolutionArtifactRecorder) WithDecisions(d DecisionRepository) *SolutionArtifactRecorder {
	r.decisions = d
	return r
}

// SupersedeDecisions supersedes the live Decision of each Solution and writes the history rows; it joins the ctx transaction.
func (r *SolutionArtifactRecorder) SupersedeDecisions(ctx context.Context, solutionIDs []string) error {
	if r.decisions == nil {
		return nil
	}
	for _, id := range solutionIDs {
		done, err := r.decisions.SupersedeLiveBySubject(ctx, domain.DecisionSubjectSolutionOption, id)
		if err != nil {
			return err
		}
		for _, d := range done {
			if err := r.decisions.AppendHistory(ctx, domain.DecisionHistory{
				ID: uuid.NewString(), DecisionID: d.ID, Action: domain.DecisionActionSuperseded, OptionID: d.ChosenOptionID, ActorID: callerID(ctx), At: r.clock(),
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

var _ SolutionArtifacts = (*SolutionArtifactRecorder)(nil)

func NewSolutionArtifactRecorder(mint *MintArtifactIDs, relations ArtifactRelationRepository) *SolutionArtifactRecorder {
	return &SolutionArtifactRecorder{mint: mint, relations: relations, clock: func() time.Time { return time.Now().UTC() }}
}

// CheckCoverage runs when a document is generated: no coverage entry may name an unknown AC, and every active
// AC needs an entry. Whether the option that gets chosen answers each AC is CheckChosen's question.
func (r *SolutionArtifactRecorder) CheckCoverage(req domain.Request, doc []byte) error {
	content, view, ok, err := r.parse(req, doc)
	if err != nil || !ok {
		return err
	}
	vs := domain.ValidateArtifactSemantics(domain.ArtifactContext{Request: content, Solution: &view})
	listed := map[string]bool{}
	for _, e := range view.RequirementCoverage {
		listed[e.ACID] = true
	}
	for _, id := range content.AcceptanceCriteria.ActiveIDs() {
		if !listed[id] {
			vs = append(vs, domain.Violation{Path: "/requirement_coverage", Code: domain.CodeArtifactACUncoveredOption, Message: id + " has no requirement_coverage entry"})
		}
	}
	return violationsError(vs)
}

// CheckChosen runs at approval: the picked option must answer every active AC. Documents without
// requirement_coverage (written before the field existed) pass.
func (r *SolutionArtifactRecorder) CheckChosen(req domain.Request, doc []byte, optionID string) error {
	content, view, ok, err := r.parse(req, doc)
	if err != nil || !ok || len(view.RequirementCoverage) == 0 {
		return err
	}
	return violationsError(domain.ValidateArtifactSemantics(domain.ArtifactContext{Request: content, Solution: &view, ChosenOptionID: optionID}))
}

// parse returns ok=false when there is nothing to check: no readable AC, or none active.
func (r *SolutionArtifactRecorder) parse(req domain.Request, doc []byte) (domain.RequestContent, domain.SolutionCoverageView, bool, error) {
	content, err := domain.ContentFromRequest(req)
	if err != nil || len(content.AcceptanceCriteria.ActiveIDs()) == 0 {
		return content, domain.SolutionCoverageView{}, false, nil
	}
	view, err := domain.ParseSolutionCoverage(doc)
	return content, view, err == nil, err
}

func violationsError(vs []domain.Violation) error {
	if len(vs) == 0 {
		return nil
	}
	parts := make([]string, 0, len(vs))
	for _, v := range vs {
		parts = append(parts, fmt.Sprintf("%s %s: %s", v.Path, v.Code, v.Message))
	}
	return fmt.Errorf("requirement coverage is incomplete: %s", strings.Join(parts, "; "))
}

func (r *SolutionArtifactRecorder) Annotate(req domain.Request, run domain.AnalysisRun, sol *domain.Solution) error {
	digest, err := domain.DigestRaw(sol.OptionsJSON)
	if err != nil {
		return err
	}
	tool := "ai.complete"
	if run.Mode == domain.AnalysisModeAgentReadonly {
		tool = "agent.execPrompt"
	}
	input, err := domain.ComputeProvenanceInputDigest(domain.InputDigestParts{
		RequestSnapshot: map[string]any{"request_id": req.ID, "content_revision": req.ContentRevision, "content_digest": req.ContentDigest},
		PromptVersion:   "solution-v1",
	})
	if err != nil {
		return err
	}
	prov := domain.Provenance{
		Generator:   domain.ProvenanceGenerator{Kind: domain.GeneratorNative, Tool: tool, ModelSource: domain.ModelSourceUnknown},
		Prompt:      domain.ProvenancePrompt{Template: string(sol.Kind), Version: "1"},
		RunID:       run.ID,
		Attempt:     run.Attempt,
		InputDigest: input,
		InputRefs:   []domain.InputRef{{Kind: "request", ID: req.ID, Revision: req.ContentRevision}},
		Actor:       domain.ProvenanceActor{ID: run.ActorID, Kind: domain.RevisionActorUser},
		GeneratedAt: r.clock(),
	}
	if err := prov.Validate(); err != nil {
		return err
	}
	raw, err := json.Marshal(prov)
	if err != nil {
		return err
	}
	sol.StampArtifact(raw, digest, maxInt(req.ContentRevision, 1))
	return nil
}

func (r *SolutionArtifactRecorder) Record(ctx context.Context, req domain.Request, sol domain.Solution, run domain.AnalysisRun, superseded []string) error {
	if err := r.mint.MintSolutionID(ctx, req.Number, &sol); err != nil {
		return err
	}
	edge := func(rel domain.Relation, from, to string, toKind domain.NodeKind) error {
		_, err := r.relations.Insert(ctx, domain.ArtifactRelation{
			TenantID: req.TenantID, RequestID: req.ID, Rel: rel, FromKind: domain.NodeSolution, FromID: from, ToKind: toKind, ToID: to, CreatedByRunID: run.ID,
		})
		return err
	}
	if err := edge(domain.RelationDerivedFrom, sol.ID, req.ID, domain.NodeRequest); err != nil {
		return err
	}
	if err := r.SupersedeDecisions(ctx, superseded); err != nil {
		return err
	}
	for _, old := range superseded {
		if err := edge(domain.RelationSupersedes, sol.ID, old, domain.NodeSolution); err != nil {
			return err
		}
	}
	return nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

package wscompat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// EnginePreflightNoteView is one reason a dev server cannot (or will not) run what a project switched on.
type EnginePreflightNoteView struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	FixHint string `json:"fixHint"`
}

// EnginePreflightView is nil in an answer when no check was asked for.
type EnginePreflightView struct {
	Ran             bool                      `json:"ran"`
	OK              bool                      `json:"ok"`
	Blockers        []EnginePreflightNoteView `json:"blockers"`
	Gaps            []EnginePreflightNoteView `json:"gaps"`
	ReviewGaps      []EnginePreflightNoteView `json:"reviewGaps"`
	Warnings        []string                  `json:"warnings"`
	OpenSpecVersion string                    `json:"openspecVersion"`
	CheckedAt       string                    `json:"checkedAt"`
}

type EngineSettingsView struct {
	ProjectID          string `json:"projectId"`
	Engine             string `json:"engine"`
	OpenSpecMinVersion string `json:"openspecMinVersion"`
	SolutionAgent      bool   `json:"solutionAgent"`
	// ExecutionMode is step_by_step or continuous (CR-REQ-038).
	ExecutionMode string `json:"executionMode"`
	// PlanAutoGenerate is whether Orca starts the Plan by itself once the Analysis is approved (CR-REQ-044).
	PlanAutoGenerate bool `json:"planAutoGenerate"`
	// OutcomeReview is off or manual, or empty when the project saved no choice (CR-OR-001).
	OutcomeReview string `json:"outcomeReview"`
	// OutcomeReviewEnabled is whether the review is on for the project now (its choice, else the deployment default, never past the kill switch).
	OutcomeReviewEnabled bool                 `json:"outcomeReviewEnabled"`
	Version              int64                `json:"version"`
	Preflight            *EnginePreflightView `json:"preflight,omitempty"`
}

// looseBool reads true/false or "true"/"false": a REST query string carries every value as text.
type looseBool bool

func (b *looseBool) UnmarshalJSON(raw []byte) error {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return err
	}
	switch x := v.(type) {
	case bool:
		*b = looseBool(x)
	case string:
		*b = looseBool(strings.EqualFold(x, "true") || x == "1")
	case nil:
		*b = false
	default:
		return fmt.Errorf("check must be true or false")
	}
	return nil
}

// registerEngine adds the per-project engine switches: which engine writes the Plan and whether the Solution is written
// by the read-only agent with the project's knowledge graph.
func (c requestChannels) registerEngine(r *Registry) {
	opts := requestChannelOpts{timeout: requestRPCTimeout}
	c.handle(r, "request.engineSettings", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			ProjectID string    `json:"projectId"`
			Check     looseBool `json:"check"`
		}](args)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(in.ProjectID) == "" {
			return nil, invalidRequestArg("projectId is required")
		}
		resp, err := c.req.GetProjectEngineSettings(ctx, &requestv1.GetProjectEngineSettingsRequest{ProjectId: in.ProjectID, Check: bool(in.Check)})
		if err != nil {
			return nil, err
		}
		return engineSettingsView(resp.GetSettings(), resp.GetPreflight()), nil
	})
	c.handle(r, "request.engineSettingsSet", opts, func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		// Fail early with a clear code; request-service checks the role again.
		if id.Role != roleAdmin {
			return nil, requestInputError{msg: "REQUEST_FORBIDDEN: admin role required"}
		}
		in, err := decodeRequestArgs[struct {
			ProjectID       string `json:"projectId"`
			Engine          string `json:"engine"`
			MinVersion      string `json:"openspecMinVersion"`
			SolutionAgent   bool   `json:"solutionAgent"`
			ExecutionMode   string `json:"executionMode"`
			ExpectedVersion int64  `json:"expectedVersion"`
			// A missing key keeps the stored switch; a client that predates it must not flip it.
			PlanAutoGenerate *bool  `json:"planAutoGenerate"`
			OutcomeReview    string `json:"outcomeReview"`
		}](args)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(in.ProjectID) == "" {
			return nil, invalidRequestArg("projectId is required")
		}
		resp, err := c.req.SetProjectEngineSettings(ctx, &requestv1.SetProjectEngineSettingsRequest{
			ProjectId: in.ProjectID, SolutionEngine: in.Engine, OpenspecMinVersion: in.MinVersion,
			SolutionAgent: in.SolutionAgent, ExecutionMode: in.ExecutionMode, ExpectedVersion: in.ExpectedVersion, PlanAutoGenerate: in.PlanAutoGenerate, OutcomeReview: in.OutcomeReview,
		})
		if err != nil {
			return nil, err
		}
		return engineSettingsView(resp.GetSettings(), resp.GetPreflight()), nil
	})
}

func engineSettingsView(s *requestv1.ProjectEngineSettings, p *requestv1.EnginePreflight) EngineSettingsView {
	v := EngineSettingsView{
		ProjectID: s.GetProjectId(), Engine: s.GetSolutionEngine(), OpenSpecMinVersion: s.GetOpenspecMinVersion(),
		SolutionAgent: s.GetSolutionAgent(), ExecutionMode: s.GetExecutionMode(), PlanAutoGenerate: s.GetPlanAutoGenerate(), OutcomeReview: s.GetOutcomeReview(), OutcomeReviewEnabled: s.GetOutcomeReviewEnabled(), Version: s.GetVersion(),
	}
	if p != nil {
		v.Preflight = &EnginePreflightView{
			Ran: p.GetRan(), OK: p.GetOk(), Blockers: engineNotes(p.GetBlockers()), Gaps: engineNotes(p.GetGaps()), ReviewGaps: engineNotes(p.GetReviewGaps()),
			Warnings: nonNilStrings(p.GetWarnings()), OpenSpecVersion: p.GetOpenspecVersion(), CheckedAt: p.GetCheckedAt(),
		}
	}
	return v
}

func engineNotes(in []*requestv1.EnginePreflightNote) []EnginePreflightNoteView {
	out := make([]EnginePreflightNoteView, 0, len(in))
	for _, n := range in {
		out = append(out, EnginePreflightNoteView{Code: n.GetCode(), Message: n.GetMessage(), FixHint: n.GetFixHint()})
	}
	return out
}

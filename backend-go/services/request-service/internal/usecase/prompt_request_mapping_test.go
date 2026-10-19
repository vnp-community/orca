package usecase

import (
	"errors"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/prompts/render"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func TestPromptRequest_ListsOnlyActiveCriteria(t *testing.T) {
	got := promptRequest(goldenRequest(domain.RequestTypeBug))
	if len(got.AcceptanceCriteria) != 2 || got.AcceptanceCriteria[0].ID != "AC-1" || got.AcceptanceCriteria[1].ID != "AC-3" {
		t.Fatalf("the retired AC-2 must not be offered to the model: %+v", got.AcceptanceCriteria)
	}
	if got.Type != "bug" || got.Size != "M" || got.Urgency != "normal" || got.SourceProvider != "manual" || got.Title != "Cache the landing page" {
		t.Fatalf("scalar fields not carried over: %+v", got)
	}
}

func TestPromptRequest_UnreadableCriteriaListNone(t *testing.T) {
	r := goldenRequestWithoutCriteria(domain.RequestTypeBug)
	r.AcceptanceCriteriaJSON = []byte("not json")
	if got := promptRequest(r); len(got.AcceptanceCriteria) != 0 || got.Body != r.Body {
		t.Fatalf("%+v", got)
	}
}

func TestRenderPlanPrompt_PhasesFollowTheShape(t *testing.T) {
	for shape, wantPhases := range map[domain.PlanShape]bool{
		domain.ShapePlanPhases: true, domain.ShapePlanTasks: false, domain.ShapeTaskListShell: false, domain.ShapeSingleTask: false,
	} {
		out := mustPlanPrompt(t, planGoldenInput(domain.RequestTypeChangeRequest, shape))
		if got := strings.Contains(out, "depends_on_phase_indices"); got != wantPhases {
			t.Errorf("shape %v: phases in prompt = %v, want %v", shape, got, wantPhases)
		}
	}
}

func TestRenderPlanPrompt_LabelsAreTheDomainConstants(t *testing.T) {
	for typ, label := range map[domain.RequestType]string{
		domain.RequestTypeBug: domain.PlanLabelTestRegression, domain.RequestTypePerformance: domain.PolicyLabelCheckBaseline,
		domain.RequestTypeRefactor: domain.PolicyLabelCheckTestsAfter, domain.RequestTypeOpsRequest: domain.PolicyLabelRollback,
	} {
		if out := mustPlanPrompt(t, planGoldenInput(typ, domain.ShapePlanTasks)); !strings.Contains(out, `"`+label+`"`) {
			t.Errorf("%s: the prompt does not name the label %q the validator checks", typ, label)
		}
	}
}

func TestRenderOpenSpecPlanPrompt_MissingChangeIDIsATaggedError(t *testing.T) {
	in := openSpecGoldenInput(domain.RequestSizeM, domain.ShapePlanPhases)
	in.ChangeID = ""
	_, err := RenderOpenSpecPlanPrompt(in)
	if !errors.Is(err, errPromptBuild) || !errors.Is(err, render.ErrMissingField) {
		t.Fatalf("got %v", err)
	}
	if f := specAgentFailure(domain.AnalysisRun{ID: "run-1"}, err); f.Code != domain.RunErrInternal {
		t.Fatalf("a prompt that cannot be built is an internal fault, got %s", f.Code)
	}
}

func TestPromptBuildFailure_CarriesNoRequestText(t *testing.T) {
	f := promptBuildFailure(domain.AnalysisRun{ID: "run-1"}, promptBuildError(errors.New("boom")))
	if f.Code != domain.RunErrInternal || strings.Contains(f.Message, "boom") {
		t.Fatalf("%+v", f)
	}
}

func TestTaskRangeFor_FollowsTheSizeAndTheTotalLimit(t *testing.T) {
	limits := domain.PlanLimits{MaxPhases: 2, MaxTasksPerContainer: 3, MaxTasksTotal: 4, MaxTitleRunes: 60}
	for _, c := range []struct {
		size   domain.RequestSize
		lo, hi int
	}{{domain.RequestSizeS, 1, 4}, {domain.RequestSizeM, 3, 4}, {domain.RequestSizeL, 4, 4}, {"", 3, 4}} {
		if lo, hi := TaskRangeFor(c.size, limits); lo != c.lo || hi != c.hi {
			t.Errorf("size %q: %d..%d, want %d..%d", c.size, lo, hi, c.lo, c.hi)
		}
	}
	if OpenSpecRef(142) != "REQ-142" {
		t.Error("OpenSpecRef")
	}
}

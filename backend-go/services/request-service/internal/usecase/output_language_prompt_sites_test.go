package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/prompts/output_language"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

const (
	viLine = "Output language: Vietnamese.\n"
	enLine = "Output language: English.\n"
)

func TestAnalysisSettings_OutputLanguageForIsTheConfiguredLanguage(t *testing.T) {
	if got := DefaultAnalysisSettings().OutputLanguageFor(domain.Request{}); got != (output_language.Language{}) {
		t.Fatalf("the defaults must add no instruction, got %+v", got)
	}
	if got := (AnalysisSettings{}).withDefaults().OutputLanguageFor(domain.Request{}); got != (output_language.Language{}) {
		t.Fatalf("withDefaults must not choose a language, got %+v", got)
	}
	s := AnalysisSettings{OutputLanguage: output_language.English}.withDefaults()
	if got := s.OutputLanguageFor(domain.Request{ID: "r"}); got != output_language.English {
		t.Fatalf("got %+v", got)
	}
}

func TestPromptInputs_ForwardTheLanguage(t *testing.T) {
	req := domain.Request{Title: "t", Type: domain.RequestTypeChangeRequest, Size: domain.RequestSizeM}
	solution, err := RenderSolutionPrompt(SolutionPromptInput{Kind: domain.SolutionKindSolution, Request: req, MinOptions: 2, Language: output_language.Vietnamese})
	if err != nil || !strings.Contains(solution, viLine) {
		t.Errorf("solution prompt: %v\n%.300s", err, solution)
	}
	plan, err := RenderPlanPrompt(PlanPromptInput{Request: req, Shape: domain.ShapePlanPhases, Limits: domain.DefaultPlanLimits, Language: output_language.English})
	if err != nil || !strings.Contains(plan, enLine) {
		t.Errorf("plan prompt: %v\n%.300s", err, plan)
	}
	spec, err := RenderOpenSpecPlanPrompt(OpenSpecPlanPromptInput{
		Request: req, Ref: "REQ-1", ChangeID: "req-1", Shape: domain.ShapePlanPhases, Limits: domain.DefaultPlanLimits, MinTasks: 2, MaxTasks: 5,
		ResultNonce: "n", Language: output_language.Vietnamese,
	})
	if err != nil || !strings.Contains(spec, viLine) {
		t.Errorf("openspec prompt: %v\n%.300s", err, spec)
	}
}

func TestRunSolutionGeneration_PromptCarriesTheConfiguredLanguage(t *testing.T) {
	for _, tc := range []struct {
		name    string
		lang    output_language.Language
		want    string
		without bool
	}{{"vi", output_language.Vietnamese, viLine, false}, {"en", output_language.English, enLine, false}, {"unset", output_language.Language{}, "", true}} {
		t.Run(tc.name, func(t *testing.T) {
			w := newSolWorld()
			run, _ := startRun(t, w, domain.RequestTypeChangeRequest, AnalysisConnection{ConnectionID: "c"})
			completer := &scriptedCompleter{replies: []string{oneOptionReply(), validSolutionReply(t)}}
			settings := AnalysisSettings{OutputLanguage: tc.lang}
			gen := NewRunSolutionGeneration(w, fakeSolutions{w}, completer, nil, w.newWriter(w.transitioner()), settings)
			runner := NewAnalysisRunner(fakeRuns{w}, fakeSolutions{w}, w, gen, nil, settings, testOwner)
			runner.execute(run)
			if completer.calls() != 2 {
				t.Fatalf("calls = %d", completer.calls())
			}
			// Both the first prompt and the retry, which is built from the same input, carry the language.
			for i, p := range completer.prompts {
				if tc.without && strings.Contains(p, "Output language") {
					t.Errorf("prompt %d has a language instruction although none is configured", i)
				}
				if !tc.without && strings.Count(p, tc.want) != 1 {
					t.Errorf("prompt %d lacks %q", i, tc.want)
				}
			}
		})
	}
}

func TestAgentReadonly_PromptCarriesTheConfiguredLanguage(t *testing.T) {
	for typ, reply := range map[domain.RequestType]string{
		domain.RequestTypeBug:      "diagnosis_valid.json",
		domain.RequestTypeSpike:    "findings_valid.json",
		domain.RequestTypeQuestion: "answer_valid.json",
	} {
		k := newAgentKit(t, analysisReply(t, reply))
		k.setting.OutputLanguage = output_language.Vietnamese
		if run, _ := k.run(t, typ); run.Status != domain.RunStatusSucceeded {
			t.Fatalf("%s: run = %s", typ, describe(run))
		}
		if p := k.agent.inputs[0].Prompt; strings.Count(p, viLine) != 1 {
			t.Errorf("%s: prompt lacks the language instruction:\n%.400s", typ, p)
		}
	}
	k := newAgentKit(t, analysisReply(t, "diagnosis_valid.json"))
	if k.run(t, domain.RequestTypeBug); strings.Contains(k.agent.inputs[0].Prompt, "Output language") {
		t.Error("an unset language must add no instruction")
	}
}

// The rebuild without the knowledge graph renders the same input again; it must not lose the language.
func TestAgentReadonly_PromptWithoutTheGraphKeepsTheLanguage(t *testing.T) {
	k := newSolutionAgentKit(t, true)
	k.agentKit.setting.OutputLanguage = output_language.English
	reply := fenced(domainTestdata(t, "solution_options/valid_with_design_references.json"))
	k.agent.res = func(_ int, in AgentPromptInput) (AgentPromptResult, error) {
		if in.KnowledgeRoot != "" {
			return AgentPromptResult{}, ErrAgentKnowledgeHubUnavailable
		}
		return AgentPromptResult{Stdout: reply, AppliedAccessMode: "readonly"}, nil
	}
	run, err := k.start(t)
	if err != nil {
		t.Fatal(err)
	}
	k.runner.execute(run)
	if len(k.agent.inputs) != 2 {
		t.Fatalf("calls = %d", len(k.agent.inputs))
	}
	for i, in := range k.agent.inputs {
		if strings.Count(in.Prompt, enLine) != 1 {
			t.Errorf("call %d (knowledge root %q) lacks the language instruction", i, in.KnowledgeRoot)
		}
	}
}

func TestRunPlanGeneration_PromptCarriesTheConfiguredLanguage(t *testing.T) {
	for _, tc := range []struct {
		lang output_language.Language
		want string
	}{{output_language.Vietnamese, viLine}, {output_language.Language{}, ""}} {
		k, _, run, completer, _ := planRunKit(t, domain.RequestTypeChangeRequest, domain.RequestSizeM, []string{planJSON(true)})
		worker := NewRunPlanGeneration(RunPlanGenerationDeps{
			Requests: k.w, Solutions: fakeSolutions{k.w}, Completer: completer, Policies: fakePolicies{domain.NewPolicyRegistry(domain.PolicyDeps{})},
			Runs: fakeRuns{k.w}, Tx: k.w, Limits: domain.DefaultPlanLimits, Owner: testOwner, Settings: AnalysisSettings{OutputLanguage: tc.lang},
		})
		if err := worker.Execute(k.w.ctx(), run); err != nil {
			t.Fatal(err)
		}
		p := completer.prompts[0]
		if tc.want == "" && strings.Contains(p, "Output language") || tc.want != "" && strings.Count(p, tc.want) != 1 {
			t.Errorf("language %q: prompt = %.400s", tc.lang.Code, p)
		}
	}
}

func TestOpenSpecEngine_PromptCarriesTheConfiguredLanguage(t *testing.T) {
	k := newEngineKit(authoringFeatures)
	k.e.d.Settings.OutputLanguage = output_language.Vietnamese
	k.agentWritesTasks(goodTasksMD)
	if _, err := k.e.GenerateProposal(context.Background(), k.in); err != nil {
		t.Fatal(err)
	}
	if p := k.agent.inputs[0].Prompt; strings.Count(p, viLine) != 1 || !strings.Contains(p, "ORCA_RESULT_*") {
		t.Fatalf("prompt lacks the OpenSpec language instruction:\n%.600s", p)
	}
	k = newEngineKit(authoringFeatures)
	k.agentWritesTasks(goodTasksMD)
	if _, err := k.e.GenerateProposal(context.Background(), k.in); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(k.agent.inputs[0].Prompt, "Output language") {
		t.Error("an unset language must add no instruction")
	}
}

func TestNativeEngine_PromptCarriesTheOptionLanguage(t *testing.T) {
	for _, tc := range []struct {
		opts []NativeEngineOption
		want string
	}{{[]NativeEngineOption{WithOutputLanguage(output_language.Vietnamese)}, viLine}, {nil, ""}} {
		c := &promptCompleter{replies: []string{oneOptionReply(), fenced(validSolutionReply(t))}}
		_, _ = NewNativeEngine(c, nil, tc.opts...).GenerateAnalysis(context.Background(), ProjectRef{}, AnalysisInput{Request: sampleRequest("b")})
		if len(c.prompts) != 2 {
			t.Fatalf("prompts = %d", len(c.prompts))
		}
		for i, p := range c.prompts {
			if tc.want == "" && strings.Contains(p, "Output language") || tc.want != "" && strings.Count(p, tc.want) != 1 {
				t.Errorf("options %d, prompt %d: %.300s", len(tc.opts), i, p)
			}
		}
	}
}

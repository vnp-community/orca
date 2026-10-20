package prompts_test

import (
	"strings"
	"testing"

	"github.com/stablyai/orca-go/prompts/analysis"
	"github.com/stablyai/orca-go/prompts/clarification_answers"
	"github.com/stablyai/orca-go/prompts/classify_request"
	"github.com/stablyai/orca-go/prompts/git_agent_prompt"
	"github.com/stablyai/orca-go/prompts/git_commit_message"
	"github.com/stablyai/orca-go/prompts/git_pull_request"
	"github.com/stablyai/orca-go/prompts/output_language"
	"github.com/stablyai/orca-go/prompts/plan_native"
	"github.com/stablyai/orca-go/prompts/plan_openspec"
	"github.com/stablyai/orca-go/prompts/request_context"
	"github.com/stablyai/orca-go/prompts/solution_openspec"
	"github.com/stablyai/orca-go/prompts/task_agent_prompt"
	"github.com/stablyai/orca-go/prompts/task_decompose"
	"github.com/stablyai/orca-go/prompts/task_execution"
	"github.com/stablyai/orca-go/prompts/task_prompt_template"
)

// maxPromptBytes is the size budget per flow, in bytes of the rendered prompt for the large fixture below. The numbers are
// documented in prompts/README.md; raise one only with a reason in the PR (a bigger prompt is a bigger bill and a smaller
// room for the agent's own context). A flow that caps its text in the template must stay under its budget whatever the
// input size; a flow that sends an uncapped payload (a diff, an approved analysis, a task description) is budgeted as
// payload plus fixed template text.
var maxPromptBytes = map[string]int{
	"classify_request/classify":          34000,
	"analysis/solution":                  200000,
	"analysis/agent_solution":            200000,
	"analysis/diagnosis":                 200000,
	"analysis/findings":                  200000,
	"analysis/answer":                    200000,
	"clarification_answers/answers":      95000,
	"plan_native/plan":                   100000,
	"plan_openspec/plan":                 108000,
	"task_prompt_template/task":          30000,
	"solution_openspec/solution":         17000,
	"task_execution/execute":             490000,
	"task_execution/result_block_suffix": 2000,
	"task_decompose/decompose":           240000,
	"task_agent_prompt/generate":         165000,
	"git_commit_message/commit_message":  560000,
	"git_commit_message/stats_only":      380000,
	"git_pull_request/fields":            545000,
	"git_agent_prompt/issue":             235000,
}

// filler returns about n runes of text that mixes ASCII with multi-byte letters, so a cap counted in bytes instead of runes
// would show up as a different size.
func filler(n int) string {
	const unit = "Tài khoản khách hàng không thể đăng nhập sau khi đổi mật khẩu; the retry loop never ends. "
	runes := []rune(strings.Repeat(unit, n/len([]rune(unit))+1))
	return string(runes[:n])
}

func largeRequest() request_context.Request {
	criteria := make([]request_context.Criterion, 60)
	for i := range criteria {
		criteria[i] = request_context.Criterion{ID: "AC-" + filler(3), Text: filler(400)}
	}
	return request_context.Request{
		Title: filler(300), Type: "change_request", Size: "large", Urgency: "high", SourceProvider: "jira",
		ClassificationReason: filler(2000), Body: filler(200000), AcceptanceCriteria: criteria,
	}
}

func largeProject() request_context.Project {
	return request_context.Project{Name: filler(200), RepoURL: "https://example.com/" + filler(200)}
}

func largeAnswers() []clarification_answers.Answer {
	answers := make([]clarification_answers.Answer, 50)
	for i := range answers {
		answers[i] = clarification_answers.Answer{Key: "q" + filler(4), Question: filler(5000), Answer: filler(20000)}
	}
	return answers
}

func largePriorArtifacts() []analysis.PriorArtifact {
	prior := make([]analysis.PriorArtifact, 30)
	for i := range prior {
		prior[i] = analysis.PriorArtifact{Kind: "solution", Status: "rejected", Content: filler(60000), Feedback: filler(10000)}
	}
	return prior
}

func largeAnalysisInput(kind string, agent bool) analysis.Input {
	return analysis.Input{
		Kind: kind, Request: largeRequest(), Project: largeProject(), MinOptions: 3, PriorArtifacts: largePriorArtifacts(),
		Answers: largeAnswers(), Feedback: filler(50000), RetryNote: filler(50000), AgentSolution: agent, UseKnowledge: agent,
		Language: output_language.Vietnamese,
	}
}

func largeFiles(n int) []git_commit_message.File {
	files := make([]git_commit_message.File, n)
	for i := range files {
		files[i] = git_commit_message.File{State: "M", Path: "services/" + filler(120) + "/file.go"}
	}
	return files
}

type sizeCase struct {
	id     string
	render func() (string, error)
}

func sizeCases() []sizeCase {
	plan := plan_native.Input{
		Request: largeRequest(), Project: largeProject(), Phases: true,
		Limits: plan_native.Limits{MaxPhases: 8, MaxTasksPerContainer: 12, MaxTasksTotal: 60, MaxTitleRunes: 120},
		Labels: plan_native.Labels{
			TestRegression: "test:regression", CheckBaseline: "check:baseline", CheckAfter: "check:after",
			CheckTestsPre: "check:tests-pre", CheckTestsAfter: "check:tests-after", Rollback: "rollback",
		},
		Analysis: filler(80000), Feedback: filler(50000), RetryNote: filler(50000), Language: output_language.Vietnamese,
		PriorPlan: filler(40000),
	}
	revisionFiles := make([]string, 200)
	for i := range revisionFiles {
		revisionFiles[i] = "specs/" + filler(100) + "/spec.md"
	}
	openspec := plan_openspec.Input{
		Request: largeRequest(), Project: largeProject(), Ref: "REQ-142", ChangeID: "change-" + filler(20), Phases: true,
		Limits:   plan_openspec.Limits{MaxPhases: 8, MaxTasksPerContainer: 12, MaxTitleRunes: 120},
		MinTasks: 20, MaxTasks: 60, Analysis: filler(80000), Feedback: filler(50000), KnowledgeAvailable: true,
		ResultNonce: "nonce-" + filler(32), RetryNote: filler(50000), Language: output_language.Vietnamese,
		PreviousFiles: revisionFiles,
	}
	deps := make([]task_execution.Dependency, 30)
	for i := range deps {
		deps[i] = task_execution.Dependency{Title: filler(200), Description: filler(8000)}
	}
	execute := task_execution.Input{
		Task:          task_execution.Task{Title: filler(300), Description: filler(60000), AIContext: filler(40000), PromptTemplate: filler(4000)},
		Parent:        &task_execution.Parent{Title: filler(300), Description: filler(8000)},
		CompletedDeps: deps,
	}
	existing := make([]string, 200)
	for i := range existing {
		existing[i] = filler(120)
	}
	decompose := task_decompose.Input{
		Title: filler(300), Description: filler(60000), AIContext: filler(40000), ProviderContext: filler(20000),
		ProjectName: filler(200), RepoURL: "https://example.com/" + filler(200), Languages: existing[:20], Frameworks: existing[:20],
		ExistingTitles: existing, VelocityTitles: existing,
	}
	comments := make([]string, 50)
	for i := range comments {
		comments[i] = filler(6000)
	}
	satisfies := make([]string, 100)
	for i := range satisfies {
		satisfies[i] = "AC-" + filler(200)
	}
	recent := make([]git_commit_message.Commit, 20)
	for i := range recent {
		recent[i] = git_commit_message.Commit{SHA: strings.Repeat("a1b2c3d4", 5), Message: filler(500)}
	}
	return []sizeCase{
		{classify_request.ID, func() (string, error) {
			labels := make([]string, 50)
			for i := range labels {
				labels[i] = filler(60)
			}
			return classify_request.Render(classify_request.Input{Title: filler(300), IssueType: "Bug", Labels: labels, Body: filler(200000)})
		}},
		{analysis.IDSolution, func() (string, error) { return analysis.Render(largeAnalysisInput(analysis.KindSolution, false)) }},
		{analysis.IDAgentSolution, func() (string, error) { return analysis.Render(largeAnalysisInput(analysis.KindSolution, true)) }},
		{analysis.IDDiagnosis, func() (string, error) { return analysis.Render(largeAnalysisInput(analysis.KindDiagnosis, true)) }},
		{analysis.IDFindings, func() (string, error) { return analysis.Render(largeAnalysisInput(analysis.KindFindings, true)) }},
		{analysis.IDAnswer, func() (string, error) { return analysis.Render(largeAnalysisInput(analysis.KindAnswer, true)) }},
		{clarification_answers.ID, func() (string, error) {
			return clarification_answers.Render(clarification_answers.Input{Answers: largeAnswers()})
		}},
		{plan_native.ID, func() (string, error) { return plan_native.Render(plan) }},
		{plan_openspec.ID, func() (string, error) { return plan_openspec.Render(openspec) }},
		{task_prompt_template.ID, func() (string, error) {
			return task_prompt_template.Render(task_prompt_template.Input{
				TaskID: "T-" + filler(20), ChangeID: filler(40), Ref: "REQ-142", Branch: "feat/" + filler(80), Satisfies: satisfies,
			})
		}},
		{solution_openspec.ID, func() (string, error) {
			return solution_openspec.Render(solution_openspec.Input{ChangeID: filler(40), RequestTitle: filler(300), RequestBody: filler(200000)})
		}},
		{task_execution.ID, func() (string, error) { return task_execution.Render(execute) }},
		{task_execution.ResultBlockID, func() (string, error) {
			return task_execution.RenderResultBlock(task_execution.ResultBlockInput{Nonce: filler(64), RequirePassingCheck: true, KnowledgeHub: true})
		}},
		{task_decompose.ID, func() (string, error) { return task_decompose.Render(decompose) }},
		{task_agent_prompt.ID, func() (string, error) {
			return task_agent_prompt.Render(task_agent_prompt.Input{
				Title: filler(300), Description: filler(60000), AIContext: filler(40000), ProviderContext: filler(20000),
			})
		}},
		{git_commit_message.ID, func() (string, error) {
			return git_commit_message.Render(git_commit_message.Input{Branch: filler(100), Recent: recent, IssueRef: "PROJ-1234", Diff: filler(400000)})
		}},
		{git_commit_message.StatsOnlyID, func() (string, error) {
			return git_commit_message.RenderStatsOnly(git_commit_message.StatsOnlyInput{Files: largeFiles(2000)})
		}},
		{git_pull_request.ID, func() (string, error) { return git_pull_request.Render(git_pull_request.Input{Diff: filler(400000)}) }},
		{git_agent_prompt.ID, func() (string, error) {
			return git_agent_prompt.Render(git_agent_prompt.Input{
				Title: filler(300), Description: filler(100000), AcceptanceCriteria: filler(100000), Comments: comments,
			})
		}},
	}
}

func TestPromptSize_LargestRealisticInputStaysUnderTheBudget(t *testing.T) {
	cases := sizeCases()
	if len(cases) != len(maxPromptBytes) {
		t.Fatalf("%d size cases but %d budgets: every flow needs both", len(cases), len(maxPromptBytes))
	}
	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			limit, ok := maxPromptBytes[c.id]
			if !ok {
				t.Fatalf("flow %s has no budget in maxPromptBytes", c.id)
			}
			out, err := c.render()
			if err != nil {
				t.Fatalf("render with the large fixture: %v", err)
			}
			t.Logf("%s: %d bytes (budget %d)", c.id, len(out), limit)
			if len(out) > limit {
				t.Errorf("%s renders %d bytes for the large fixture, budget is %d", c.id, len(out), limit)
			}
		})
	}
}

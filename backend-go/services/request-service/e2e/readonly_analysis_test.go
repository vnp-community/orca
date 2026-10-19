//go:build e2e

package e2e

import (
	"strings"
	"testing"
	"time"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/e2e/stubs"
	"google.golang.org/grpc/codes"
)

// execPromptsFor returns the agent.execPrompt calls whose prompt mentions the marker.
func execPromptsFor(marker string) []stubs.Call {
	var out []stubs.Call
	for _, c := range theStack.stubs.Script.Calls() {
		if p, _ := c.Params["prompt"].(string); c.Method == "agent.execPrompt" && strings.Contains(p, marker) {
			out = append(out, c)
		}
	}
	return out
}

// analyzingRequest takes a new Request of the type to the analyzing stage.
func analyzingRequest(k *kit, title, typ string) *requestv1.Request {
	k.t.Helper()
	r := k.confirm(k.classified(title, typ, "S"), typ, "S")
	return k.waitStatus(r.GetId(), "analyzing")
}

// Types whose analysis only reads the repo run the agent in the project's repo folder on its dev server.
func TestReadonlyAnalysis_RunsTheAgentInTheRepoFolderOfTheDevServer(t *testing.T) {
	for _, tc := range []struct {
		typ  string
		kind requestv1.SolutionKind
	}{
		{"bug", requestv1.SolutionKind_SOLUTION_KIND_DIAGNOSIS},
		{"spike", requestv1.SolutionKind_SOLUTION_KIND_FINDINGS},
		{"question", requestv1.SolutionKind_SOLUTION_KIND_ANSWER},
	} {
		t.Run(tc.typ, func(t *testing.T) {
			k := newKit(t)
			k.enableFlow()
			marker := "readonly-" + k.tenantID
			r := analyzingRequest(k, marker, tc.typ)
			// The clarification resume may have started the run already; the explicit call then answers with that run.
			gen := retry(k, "GenerateSolution", func() (*requestv1.GenerateSolutionResponse, error) {
				return k.req.GenerateSolution(k.asReporter(), &requestv1.GenerateSolutionRequest{RequestId: r.GetId()})
			})
			eventually(t, 30*time.Second, "the read-only analysis run "+gen.GetRunId(), func() (bool, string) {
				resp, err := k.req.ListSolutions(k.asReporter(), &requestv1.ListSolutionsRequest{RequestId: r.GetId()})
				if err != nil {
					return false, err.Error()
				}
				for _, run := range resp.GetRuns() {
					if run.GetStatus() == "failed" {
						t.Fatalf("analysis run failed: %s: %s", run.GetErrorCode(), run.GetErrorMessage())
					}
				}
				for _, s := range resp.GetSolutions() {
					if s.GetKind() == tc.kind && s.GetStatus() == requestv1.SolutionStatus_SOLUTION_STATUS_PROPOSED {
						return true, ""
					}
				}
				return false, "no proposed Solution of the kind yet"
			})
			calls := execPromptsFor(marker)
			if len(calls) == 0 {
				t.Fatal("the analysis never reached the agent")
			}
			for _, c := range calls {
				if c.DevServerID != stubs.DevServerID || c.Params["worktreePath"] != stubs.DefaultRepoURL {
					t.Fatalf("agent call on %q in %v, want %s in %s", c.DevServerID, c.Params["worktreePath"], stubs.DevServerID, stubs.DefaultRepoURL)
				}
			}
		})
	}
}

// A default repo that is only a remote address names no folder: read-only analysis says so with the structured code
// instead of guessing a directory, and creates no run. Complete mode never reads the repo, so it still works.
func TestReadonlyAnalysis_RemoteOnlyRepoFailsWithAStructuredCode(t *testing.T) {
	k := newKit(t)
	k.enableFlow()
	theStack.stubs.Project.SetRepoURL(k.project, "https://git.example.test/acme/orca.git")
	marker := "remote-" + k.tenantID
	r := analyzingRequest(k, marker, "bug")

	_, err := k.req.GenerateSolution(k.asReporter(), &requestv1.GenerateSolutionRequest{RequestId: r.GetId()})
	if code(err) != codes.FailedPrecondition || !strings.Contains(errText(err), "REQUEST_ANALYSIS_NO_REPO_PATH") {
		t.Fatalf("GenerateSolution = %v, want FailedPrecondition REQUEST_ANALYSIS_NO_REPO_PATH", err)
	}
	if resp, err := k.req.ListSolutions(k.asReporter(), &requestv1.ListSolutionsRequest{RequestId: r.GetId()}); err != nil || len(resp.GetRuns()) != 0 {
		t.Fatalf("no run may exist after the refusal: %v, %v", resp, err)
	}
	if n := len(execPromptsFor(marker)); n != 0 {
		t.Fatalf("the agent ran %d times without a folder to run in", n)
	}
	retry(k, "GenerateSolution complete", func() (*requestv1.GenerateSolutionResponse, error) {
		return k.req.GenerateSolution(k.asReporter(), &requestv1.GenerateSolutionRequest{RequestId: r.GetId(), AnalysisMode: requestv1.AnalysisMode_ANALYSIS_MODE_COMPLETE})
	})
}

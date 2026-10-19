package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type fakeExec struct {
	res  ExecResult
	err  error
	seen []ExecInput
}

func (f *fakeExec) Exec(_ context.Context, _ AnalysisConnection, in ExecInput) (ExecResult, error) {
	f.seen = append(f.seen, in)
	return f.res, f.err
}

var repoConn = fakeConns{conn: AnalysisConnection{ConnectionID: "c", RepoPath: "/srv/repo"}}

func gate(exec DevServerExecutor, conns AnalysisConnectionResolver, min string) *EngineReadinessGate {
	return NewEngineReadinessGate(conns, exec, min)
}

func TestReadinessGate_OKWhenOpenSpecIsNewEnough(t *testing.T) {
	ex := &fakeExec{res: ExecResult{Stdout: "1.3.0\n"}}
	rep, err := gate(ex, repoConn, "1.2.0").Check(context.Background(), "p", nil)
	if err != nil || !rep.OK || rep.OpenSpecVersion != "1.3.0" || len(rep.Failures) != 0 {
		t.Fatalf("%+v %v", rep, err)
	}
	if len(ex.seen) != 1 || ex.seen[0].Binary != "openspec" || len(ex.seen[0].Args) != 1 || ex.seen[0].Args[0] != "--version" || ex.seen[0].Cwd != "/srv/repo" {
		t.Fatalf("the probe must be `openspec --version` in the repo: %+v", ex.seen)
	}
}

func TestReadinessGate_ReportsEachFailure(t *testing.T) {
	cases := []struct {
		name  string
		exec  *fakeExec
		conns AnalysisConnectionResolver
		min   string
		code  string
	}{
		{"not installed", &fakeExec{res: ExecResult{ExitCode: -1, SpawnError: "ENOENT"}}, repoConn, "", "REQUEST_ENGINE_OPENSPEC_MISSING"},
		{"exits non-zero", &fakeExec{res: ExecResult{ExitCode: 127, Stderr: "not found"}}, repoConn, "", "REQUEST_ENGINE_OPENSPEC_MISSING"},
		{"too old", &fakeExec{res: ExecResult{Stdout: "1.1.9"}}, repoConn, "1.2.0", "REQUEST_ENGINE_OPENSPEC_VERSION"},
		{"unreadable version", &fakeExec{res: ExecResult{Stdout: "openspec: development build"}}, repoConn, "1.2.0", "REQUEST_ENGINE_OPENSPEC_VERSION"},
		{"timed out", &fakeExec{res: ExecResult{TimedOut: true, ExitCode: -1}}, repoConn, "", "REQUEST_ENGINE_OPENSPEC_TIMEOUT"},
		{"no dev server", &fakeExec{}, fakeConns{err: ErrNoDevServer}, "", "REQUEST_ENGINE_NO_CONNECTION"},
		{"repo has no path", &fakeExec{}, fakeConns{conn: AnalysisConnection{ConnectionID: "c"}}, "", "REQUEST_ENGINE_REPO_NOT_RESOLVED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rep, err := gate(tc.exec, tc.conns, tc.min).Check(context.Background(), "p", nil)
			if err != nil {
				t.Fatalf("a failed probe is a verdict, not an error: %v", err)
			}
			if rep.OK || len(rep.Failures) != 1 || rep.Failures[0].Code != tc.code || rep.Failures[0].FixHint == "" {
				t.Fatalf("report = %+v", rep)
			}
		})
	}
}

func TestReadinessGate_AnOpenSpecThatExitsZeroButPrintsNothingUseful(t *testing.T) {
	rep, _ := gate(&fakeExec{res: ExecResult{Stdout: ""}}, repoConn, "1.0.0").Check(context.Background(), "p", nil)
	if rep.OK {
		t.Fatalf("empty output cannot satisfy a minimum version: %+v", rep)
	}
}

func TestReadinessGate_ProjectMinimumOverridesTheServerDefault(t *testing.T) {
	ex := &fakeExec{res: ExecResult{Stdout: "1.3.0"}}
	settings := &domain.ProjectEngineSettings{MinVersion: "2.0.0"}
	rep, _ := gate(ex, repoConn, "1.0.0").Check(context.Background(), "p", settings)
	if rep.OK || rep.Failures[0].Code != "REQUEST_ENGINE_OPENSPEC_VERSION" || !strings.Contains(rep.Failures[0].Message, "2.0.0") {
		t.Fatalf("%+v", rep)
	}
	rep, _ = gate(ex, repoConn, "9.9.9").Check(context.Background(), "p", &domain.ProjectEngineSettings{MinVersion: "1.2.0"})
	if !rep.OK {
		t.Fatalf("the project setting must win over the server default: %+v", rep)
	}
}

func TestReadinessGate_NoMinimumMeansAnyVersionPasses_AndABadMinimumWarnsInsteadOfBlocking(t *testing.T) {
	rep, _ := gate(&fakeExec{res: ExecResult{Stdout: "0.0.1"}}, repoConn, "").Check(context.Background(), "p", nil)
	if !rep.OK {
		t.Fatalf("%+v", rep)
	}
	rep, _ = gate(&fakeExec{res: ExecResult{Stdout: "1.3.0"}}, repoConn, "").Check(context.Background(), "p", &domain.ProjectEngineSettings{MinVersion: "latest"})
	if !rep.OK || len(rep.Warnings) != 1 {
		t.Fatalf("%+v", rep)
	}
}

func TestReadinessGate_CheckErrorsWhenTheCheckItselfCannotRun(t *testing.T) {
	boom := errors.New("resolver down")
	if _, err := gate(&fakeExec{}, fakeConns{err: boom}, "").Check(context.Background(), "p", nil); !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
	if _, err := gate(&fakeExec{err: boom}, repoConn, "").Check(context.Background(), "p", nil); !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
}

package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type PreflightFailure struct {
	Code    string
	Message string
	FixHint string
}

type PreflightReport struct {
	OK              bool
	Failures        []PreflightFailure
	Warnings        []string
	CheckedAt       time.Time
	OpenSpecVersion string
}

// openSpecProbeTimeoutMs bounds `openspec --version`; a healthy install answers in well under a second.
const openSpecProbeTimeoutMs = 10_000

// EngineReadinessGate says whether a project's dev server can run the openspec engine. It asks the dev server itself, so
// a missing or old openspec shows up here and not as a failed run later.
type EngineReadinessGate struct {
	conns      AnalysisConnectionResolver
	exec       DevServerExecutor
	minVersion string
	now        func() time.Time
}

func NewEngineReadinessGate(conns AnalysisConnectionResolver, exec DevServerExecutor, minVersion string) *EngineReadinessGate {
	return &EngineReadinessGate{conns: conns, exec: exec, minVersion: minVersion, now: func() time.Time { return time.Now().UTC() }}
}

// Check never returns an error for a failed probe: the verdict is in the report. An error means the check itself could
// not run (no tenant, the resolver broke).
func (g *EngineReadinessGate) Check(ctx context.Context, projectID string, settings *domain.ProjectEngineSettings) (PreflightReport, error) {
	report := PreflightReport{OK: true, CheckedAt: g.now()}
	conn, err := g.conns.ResolveForProject(ctx, projectID)
	if err != nil {
		if errors.Is(err, ErrNoDevServer) {
			return failed(report, "REQUEST_ENGINE_NO_CONNECTION", "no dev server can run this project's repo", "Connect a dev server and add the repo to the project."), nil
		}
		return report, err
	}
	if conn.RepoPath == "" {
		return failed(report, "REQUEST_ENGINE_REPO_NOT_RESOLVED", "the repo has no folder path on the dev server", "Add the repo with a path the dev server can open."), nil
	}

	res, err := g.exec.Exec(ctx, conn, ExecInput{Binary: "openspec", Args: []string{"--version"}, Cwd: conn.RepoPath, TimeoutMs: openSpecProbeTimeoutMs})
	switch {
	case err != nil:
		return report, err
	case res.TimedOut:
		return failed(report, "REQUEST_ENGINE_OPENSPEC_TIMEOUT", "openspec did not answer in time", "Run `openspec --version` on the dev server."), nil
	case res.SpawnError != "" || res.ExitCode != 0:
		return failed(report, "REQUEST_ENGINE_OPENSPEC_MISSING", "openspec is not installed or does not start on the dev server", "Install the OpenSpec CLI on the dev server."), nil
	}

	report.OpenSpecVersion = strings.TrimSpace(res.Stdout)
	ver, err := domain.ParseSemver(res.Stdout)
	if err != nil {
		return failed(report, "REQUEST_ENGINE_OPENSPEC_VERSION", "could not read the openspec version from "+quote(report.OpenSpecVersion), "Update the OpenSpec CLI."), nil
	}
	want := g.minVersion
	if settings != nil && settings.MinVersion != "" {
		want = settings.MinVersion
	}
	if want == "" {
		return report, nil
	}
	min, err := domain.ParseSemver(want)
	if err != nil {
		report.Warnings = append(report.Warnings, "the configured minimum openspec version "+quote(want)+" is not a version; it was ignored")
		return report, nil
	}
	if domain.CompareSemver(ver, min) < 0 {
		return failed(report, "REQUEST_ENGINE_OPENSPEC_VERSION", "openspec "+report.OpenSpecVersion+" is older than the required "+want, "Update the OpenSpec CLI to "+want+" or newer."), nil
	}
	return report, nil
}

func failed(r PreflightReport, code, msg, hint string) PreflightReport {
	r.OK = false
	r.Failures = append(r.Failures, PreflightFailure{Code: code, Message: msg, FixHint: hint})
	return r
}

func quote(s string) string { return `"` + s + `"` }

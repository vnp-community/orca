package usecase

import (
	"context"
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

type EngineReadinessGate struct {
	conns      ConnectionResolver
	exec       DevServerExecutor
	minVersion string
	ttl        time.Duration
}

func NewEngineReadinessGate(conns ConnectionResolver, exec DevServerExecutor, minVersion string) *EngineReadinessGate {
	return &EngineReadinessGate{
		conns:      conns,
		exec:       exec,
		minVersion: minVersion,
		ttl:        10 * time.Minute,
	}
}

func (g *EngineReadinessGate) Check(ctx context.Context, projectID string, settings *domain.ProjectEngineSettings) (PreflightReport, error) {
	report := PreflightReport{OK: true, CheckedAt: time.Now()}

	conn, err := g.conns.Resolve(ctx, projectID)
	if err != nil || !conn.Connected {
		report.OK = false
		report.Failures = append(report.Failures, PreflightFailure{
			Code:    "REQUEST_ENGINE_NO_CONNECTION",
			Message: "no connection to dev server",
		})
		return report, domain.ErrEngineNoConnection
	}

	res, err := g.exec.Exec(ctx, conn.ConnectionID, ExecInput{
		Binary:    "openspec",
		Args:      []string{"--version"},
		Cwd:       conn.RepoPath,
		TimeoutMs: 10000,
	})
	
	if err != nil || res.ExitCode != 0 {
		report.OK = false
		report.Failures = append(report.Failures, PreflightFailure{
			Code:    "REQUEST_ENGINE_OPENSPEC_MISSING",
			Message: "openspec missing",
		})
	} else {
		report.OpenSpecVersion = res.Stdout
		ver, err := domain.ParseSemver(res.Stdout)
		if err == nil {
			minVer := settings.MinVersion
			if minVer == "" {
				minVer = g.minVersion
			}
			if minVer != "" {
				expectedVer, err := domain.ParseSemver(minVer)
				if err == nil && domain.CompareSemver(ver, expectedVer) < 0 {
					report.OK = false
					report.Failures = append(report.Failures, PreflightFailure{
						Code:    "REQUEST_ENGINE_OPENSPEC_VERSION",
						Message: "openspec version too low",
					})
				}
			}
		}
	}

	return report, nil
}

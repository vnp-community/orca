package domain

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ProjectEngineSettings struct {
	TenantID   string
	ProjectID  string
	Engine     EngineName
	MinVersion string
	// SolutionAgent makes the Solution of a Request run through the read-only agent with the project's knowledge graph.
	SolutionAgent bool
	// ExecutionMode is how Requests of this project run after their plan is approved. Empty reads as step_by_step.
	ExecutionMode ExecutionMode
	// PlanAutoGenerate lets Orca start the Plan by itself once the Analysis is approved. On unless a project turns it off.
	PlanAutoGenerate bool
	UpdatedBy        string
	UpdatedAt        time.Time
	Version          int64
	// FromDeploymentDefault marks settings that were not saved for this project but filled in from the deployment's defaults
	// (never stored). The default engine moves only the types that run OpenSpec by default; a type that merely may run it
	// (OpenSpecProfileLight) needs a project that chose it.
	FromDeploymentDefault bool
}

var versionRegex = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

func NewProjectEngineSettings(tenantID, projectID string, engine EngineName, minVersion, actor string, now time.Time) (ProjectEngineSettings, error) {
	if tenantID == "" || projectID == "" || actor == "" {
		return ProjectEngineSettings{}, fmt.Errorf("tenantID, projectID, and actor are required")
	}
	if minVersion != "" && !versionRegex.MatchString(minVersion) {
		return ProjectEngineSettings{}, fmt.Errorf("invalid minVersion format")
	}
	return ProjectEngineSettings{
		TenantID:         tenantID,
		ProjectID:        projectID,
		Engine:           engine,
		MinVersion:       minVersion,
		ExecutionMode:    ExecutionStepByStep,
		PlanAutoGenerate: true,
		UpdatedBy:        actor,
		UpdatedAt:        now,
		Version:          1,
	}, nil
}

func EffectiveEngine(pinned *EngineName, settings *ProjectEngineSettings, profile OpenSpecProfile) EngineName {
	if profile == OpenSpecProfileNone {
		return EngineNative
	}
	if pinned != nil {
		return *pinned
	}
	if settings != nil && settings.Engine != "" {
		if profile == OpenSpecProfileLight && settings.FromDeploymentDefault {
			return EngineNative
		}
		return settings.Engine
	}
	return EngineNative
}

// EngineDefaults is what a project without saved settings runs with. An explicit project setting always wins.
type EngineDefaults struct {
	Engine        EngineName
	SolutionAgent bool
	ExecutionMode ExecutionMode
}

// EngineDefaultsFromEnv reads REQUEST_DEFAULT_ENGINE (native|openspec), REQUEST_DEFAULT_SOLUTION_AGENT (true|false) and
// REQUEST_DEFAULT_EXECUTION_MODE (step_by_step|continuous).
// Unset keeps native and off, so a deployment opts in on purpose; a value that cannot be read keeps the safe default and says so.
func EngineDefaultsFromEnv(getenv func(string) string) (EngineDefaults, []string) {
	d, warnings := EngineDefaults{Engine: EngineNative, ExecutionMode: ExecutionStepByStep}, []string(nil)
	switch raw := strings.ToLower(strings.TrimSpace(getenv("REQUEST_DEFAULT_ENGINE"))); raw {
	case "":
	case string(EngineNative), string(EngineOpenSpec):
		d.Engine = EngineName(raw)
	default:
		warnings = append(warnings, "REQUEST_DEFAULT_ENGINE "+strconv.Quote(raw)+" is not native or openspec; native applies")
	}
	switch raw := strings.ToLower(strings.TrimSpace(getenv("REQUEST_DEFAULT_SOLUTION_AGENT"))); raw {
	case "":
	case "true", "1", "yes", "on":
		d.SolutionAgent = true
	case "false", "0", "no", "off":
	default:
		warnings = append(warnings, "REQUEST_DEFAULT_SOLUTION_AGENT "+strconv.Quote(raw)+" is not true or false; off applies")
	}
	switch raw := strings.ToLower(strings.TrimSpace(getenv("REQUEST_DEFAULT_EXECUTION_MODE"))); raw {
	case "":
	case string(ExecutionStepByStep), string(ExecutionContinuous):
		d.ExecutionMode = ExecutionMode(raw)
	default:
		warnings = append(warnings, "REQUEST_DEFAULT_EXECUTION_MODE "+strconv.Quote(raw)+" is not step_by_step or continuous; step_by_step applies")
	}
	return d, warnings
}

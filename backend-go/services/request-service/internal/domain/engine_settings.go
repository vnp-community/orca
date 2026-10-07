package domain

import (
	"fmt"
	"regexp"
	"time"
)

type ProjectEngineSettings struct {
	TenantID   string
	ProjectID  string
	Engine     EngineName
	MinVersion string
	UpdatedBy  string
	UpdatedAt  time.Time
	Version    int64
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
		TenantID:   tenantID,
		ProjectID:  projectID,
		Engine:     engine,
		MinVersion: minVersion,
		UpdatedBy:  actor,
		UpdatedAt:  now,
		Version:    1,
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
		return settings.Engine
	}
	return EngineNative
}

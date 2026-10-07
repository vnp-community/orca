package domain

import (
	"encoding/json"
	"time"
)

type ToolStatus struct {
	Known     bool
	Installed bool
	Version   string
}

type ClaudeFlags struct {
	Tools           bool
	PermissionMode  bool
	DisallowedTools bool
}

type DevServerCapability struct {
	Source            string
	Degraded          bool
	Connected         bool
	Partial           bool
	AgentBuildVersion string
	ProtocolVersion   int
	Features          []string

	Platform        string
	Tools           map[string]ToolStatus
	ClaudeInstalled bool
	ClaudeKnown     bool
	ClaudeAuth      string
	ClaudeFlags     ClaudeFlags
	EnvPresent      map[string]bool
	EnvKnown        bool
	ProbedAt        time.Time
}

func (c DevServerCapability) HasFeature(name string) bool {
	for _, f := range c.Features {
		if f == name {
			return true
		}
	}
	return false
}

func (c DevServerCapability) Tool(id string) ToolStatus {
	if c.Tools == nil {
		return ToolStatus{Known: false}
	}
	ts, ok := c.Tools[id]
	if !ok {
		return ToolStatus{Known: false}
	}
	return ts
}

func (c DevServerCapability) EnvVar(name string) (present, known bool) {
	if !c.EnvKnown {
		return false, false
	}
	if c.EnvPresent == nil {
		return false, true
	}
	pres, ok := c.EnvPresent[name]
	if !ok {
		return false, true // Known but not present
	}
	return pres, true
}

func ParseCapabilityProfile(source string, degraded, connected bool, buildVersion string, protocolVersion int, features []string, profileJSON []byte, probedAt time.Time) (DevServerCapability, error) {
	c := DevServerCapability{
		Source:            source,
		Degraded:          degraded,
		Connected:         connected,
		AgentBuildVersion: buildVersion,
		ProtocolVersion:   protocolVersion,
		Features:          features,
		ProbedAt:          probedAt,
		Tools:             make(map[string]ToolStatus),
		EnvPresent:        make(map[string]bool),
		EnvKnown:          false, // Will be set to true if 'env' is present
	}

	if len(profileJSON) == 0 {
		return c, nil
	}

	var root struct {
		SchemaVersion int                    `json:"schemaVersion"`
		Partial       bool                   `json:"partial"`
		Host          map[string]interface{} `json:"host"`
		Tools         []struct {
			ID        string `json:"id"`
			Installed bool   `json:"installed"`
			Version   string `json:"version"`
		} `json:"tools"`
		Claude struct {
			Installed bool   `json:"installed"`
			Version   string `json:"version"`
			Auth      string `json:"auth"`
			Flags     struct {
				Tools           bool `json:"tools"`
				PermissionMode  bool `json:"permissionMode"`
				DisallowedTools bool `json:"disallowedTools"`
			} `json:"flags"`
		} `json:"claude"`
		Env []struct {
			Name    string `json:"name"`
			Present bool   `json:"present"`
		} `json:"env"`
	}

	// Try minimal structural check first
	var m map[string]interface{}
	if err := json.Unmarshal(profileJSON, &m); err != nil {
		// Bad JSON syntax -> return error
		return c, err
	}

	if err := json.Unmarshal(profileJSON, &root); err != nil {
		// Ignore type errors according to reqs
	}

	c.Partial = root.Partial

	if root.SchemaVersion > 1 {
		c.Degraded = true
		return c, nil
	}

	if platform, ok := root.Host["platform"].(string); ok {
		c.Platform = platform
	}

	// If Tools key exists in map, then tools are "known"
	if _, ok := m["tools"]; ok {
		for _, t := range root.Tools {
			c.Tools[t.ID] = ToolStatus{
				Known:     true,
				Installed: t.Installed,
				Version:   t.Version,
			}
		}
	}

	if _, ok := m["claude"]; ok {
		c.ClaudeKnown = true
		c.ClaudeInstalled = root.Claude.Installed
		c.ClaudeAuth = root.Claude.Auth
		c.ClaudeFlags = ClaudeFlags{
			Tools:           root.Claude.Flags.Tools,
			PermissionMode:  root.Claude.Flags.PermissionMode,
			DisallowedTools: root.Claude.Flags.DisallowedTools,
		}
	}

	if _, ok := m["env"]; ok {
		c.EnvKnown = true
		for _, e := range root.Env {
			c.EnvPresent[e.Name] = e.Present
		}
	}

	return c, nil
}

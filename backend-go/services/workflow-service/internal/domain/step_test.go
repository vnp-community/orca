package domain

import "testing"

func TestAgentStepConfig_EffectiveTarget(t *testing.T) {
	tests := []struct {
		name string
		cfg  AgentStepConfig
		want string
	}{
		{name: "Target set wins", cfg: AgentStepConfig{Target: "server:s1", ConnectionID: "conn-1"}, want: "server:s1"},
		{name: "falls back to ConnectionID", cfg: AgentStepConfig{ConnectionID: "conn-1"}, want: "connection:conn-1"},
		{name: "neither set is empty", cfg: AgentStepConfig{}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.effectiveTarget(); got != tt.want {
				t.Errorf("effectiveTarget() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestShellStepConfig_EffectiveTarget(t *testing.T) {
	tests := []struct {
		name string
		cfg  ShellStepConfig
		want string
	}{
		{name: "Target set wins", cfg: ShellStepConfig{Target: "project:p1", ConnectionID: "conn-1"}, want: "project:p1"},
		{name: "falls back to ConnectionID", cfg: ShellStepConfig{ConnectionID: "conn-1"}, want: "connection:conn-1"},
		{name: "neither set is empty", cfg: ShellStepConfig{}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.effectiveTarget(); got != tt.want {
				t.Errorf("effectiveTarget() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNotificationStepConfig_EffectiveTarget(t *testing.T) {
	tests := []struct {
		name string
		cfg  NotificationStepConfig
		want string
	}{
		{name: "Target set wins", cfg: NotificationStepConfig{Target: "fleet:tag:ci", ConnectionID: "conn-1"}, want: "fleet:tag:ci"},
		{name: "falls back to ConnectionID", cfg: NotificationStepConfig{ConnectionID: "conn-1"}, want: "connection:conn-1"},
		{name: "neither set is empty", cfg: NotificationStepConfig{}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.effectiveTarget(); got != tt.want {
				t.Errorf("effectiveTarget() = %q, want %q", got, tt.want)
			}
		})
	}
}

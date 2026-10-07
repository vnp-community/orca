package usecase

import (
	"context"
)

type ExecInput struct {
	Binary    string
	Args      []string
	Cwd       string
	Env       map[string]string
	TimeoutMs int
}

type ExecPromptInput struct {
	Prompt       string
	WorktreePath string
	TrustPreset  string
	Env          map[string]string
	TimeoutMs    int
}

type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	TimedOut bool
}

type DevServerExecutor interface {
	Exec(ctx context.Context, connectionID string, in ExecInput) (ExecResult, error)
	ExecPrompt(ctx context.Context, connectionID string, in ExecPromptInput) (ExecResult, error)
}

type Connection struct {
	Connected    bool
	ConnectionID string
	DevServerID  string
	RepoPath     string
}

type ConnectionResolver interface {
	Resolve(ctx context.Context, projectID string) (Connection, error)
}

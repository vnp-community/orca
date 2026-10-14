package usecase

import "context"

// ExecInput is one command for the dev server's agent.exec: a binary and its arguments, never a shell string.
type ExecInput struct {
	Binary    string
	Args      []string
	Cwd       string
	Env       map[string]string
	TimeoutMs int
}

type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	TimedOut bool
	// SpawnError is set when the binary could not be started (for example it is not installed).
	SpawnError string
}

// DevServerExecutor runs a short command on the dev server that holds the project's repo.
type DevServerExecutor interface {
	Exec(ctx context.Context, conn AnalysisConnection, in ExecInput) (ExecResult, error)
}

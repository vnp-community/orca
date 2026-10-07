package grpcclient

import (
	"context"

	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type DevServerExecutor struct {
	// Add grpc client fields here if any
}

func NewDevServerExecutor() *DevServerExecutor {
	return &DevServerExecutor{}
}

func (e *DevServerExecutor) Exec(ctx context.Context, connectionID string, in usecase.ExecInput) (usecase.ExecResult, error) {
	// Stub implementation
	return usecase.ExecResult{
		Stdout:   "openspec version 1.2.3",
		ExitCode: 0,
	}, nil
}

func (e *DevServerExecutor) ExecPrompt(ctx context.Context, connectionID string, in usecase.ExecPromptInput) (usecase.ExecResult, error) {
	// Stub implementation
	return usecase.ExecResult{
		Stdout:   "OK",
		ExitCode: 0,
	}, nil
}

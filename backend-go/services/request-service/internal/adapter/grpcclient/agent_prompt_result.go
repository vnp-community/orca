package grpcclient

import (
	"github.com/stablyai/orca-go/common/agentrelay"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// maxStderrBytes bounds the stderr kept on a run.
const maxStderrBytes = 16 * 1024

// agentPromptResultFrom narrows the agent's answer to what the usecase reads and applies this service's output caps
// (agentrelay returns the agent's text whole).
func agentPromptResultFrom(r agentrelay.ExecResult) usecase.AgentPromptResult {
	out := usecase.AgentPromptResult{
		Stdout: agentrelay.TruncateUTF8(r.Stdout, maxStdoutBytes), Stderr: agentrelay.TruncateUTF8(r.Stderr, maxStderrBytes),
		// No exit code means the process never completed.
		ExitCode: r.ExitCodeOr(-1), TimedOut: r.TimedOut, Warnings: r.Warnings, Stats: runStatsFrom(r.Stats),
	}
	if r.Applied != nil {
		out.AppliedAccessMode = r.Applied.AccessMode
	}
	if c := r.Changes; c != nil {
		out.ChangesAvailable, out.HeadMoved, out.ChangedFiles = c.Available, c.HeadMoved, len(c.ChangedFiles)
	}
	if p := r.Parsed; p != nil {
		if p.OK {
			out.ResultBlock = p.Value
		} else {
			out.ResultBlockCode = p.Code
		}
	}
	return out
}

func runStatsFrom(s agentrelay.Stats) domain.RunStats {
	out := domain.RunStats{ToolCalls: s.ToolCalls}
	if u := s.Usage; u != nil {
		out.Usage = &domain.RunUsage{
			Turns: u.Turns, DurationMS: u.DurationMS, CostUSD: u.CostUSD, InputTokens: u.InputTokens,
			OutputTokens: u.OutputTokens, CacheReadTokens: u.CacheReadTokens, CacheCreationTokens: u.CacheCreationTokens,
		}
	}
	return out
}

package localgit

import (
	"bufio"
	"context"
	"os/exec"
	"sync"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// streamLine is one line captured from a running `git` subprocess's
// stdout or stderr pipe, tagged by which stream it came from — internal
// plumbing between pushLine/pullLine's two scanner goroutines and their
// shared merge loop below.
type streamLine struct {
	line   string
	source string // "stdout" | "stderr"
}

// runStreamed runs `git <args...>` with repoPath as its working directory,
// piping Stdout/Stderr line-by-line to sink as domain.GitProgressLine
// frames (IsFinal=false) as they arrive, then a final IsFinal=true frame
// once the process exits — the real usecase.StreamingGitExecutor contract
// this package's PushStream/PullStream both implement against. A
// non-nil error from sink aborts the subprocess (via ctx cancellation is
// NOT attempted here — sink errors simply stop forwarding further lines;
// the subprocess is still waited on to avoid leaking it, but its exit
// status is not itself surfaced once sink has already errored).
func runStreamed(ctx context.Context, repoPath string, sink func(domain.GitProgressLine) error, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	lines := make(chan streamLine, 64)
	var wg sync.WaitGroup
	wg.Add(2)
	scan := func(r *bufio.Scanner, source string) {
		defer wg.Done()
		for r.Scan() {
			lines <- streamLine{line: r.Text(), source: source}
		}
	}
	go scan(bufio.NewScanner(stdout), "stdout")
	go scan(bufio.NewScanner(stderr), "stderr")
	go func() {
		wg.Wait()
		close(lines)
	}()

	var sinkErr error
	for l := range lines {
		if sinkErr != nil {
			continue // already aborted — drain the channel so the scanner goroutines don't block forever on a full buffer
		}
		if err := sink(domain.GitProgressLine{Line: l.line, Source: l.source}); err != nil {
			sinkErr = err
		}
	}

	waitErr := cmd.Wait()
	if sinkErr != nil {
		return sinkErr
	}

	exitCode := 0
	success := waitErr == nil
	hadConflicts := false
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return waitErr // a real launch/wait failure (not just a non-zero exit), e.g. context cancellation
		}
	}
	return sink(domain.GitProgressLine{
		IsFinal:      true,
		ExitCode:     int32(exitCode),
		Success:      success,
		HadConflicts: hadConflicts,
	})
}

// PushStream runs `git push [remote [branch]]`, streaming stdout/stderr
// line-by-line — same argument-building rule as Executor.Push.
func (e *Executor) PushStream(ctx context.Context, repoPath, remote, branch string, sink func(domain.GitProgressLine) error) error {
	args := []string{"push"}
	if remote != "" {
		args = append(args, remote)
		if branch != "" {
			args = append(args, branch)
		}
	}
	return runStreamed(ctx, repoPath, sink, args...)
}

// PullStream runs `git pull`, streaming stdout/stderr line-by-line. Unlike
// Executor.Pull (which inspects combined output for "CONFLICT" to set
// PullResult.HadConflicts), the streamed final frame does not attempt that
// same best-effort detection — the caller already saw every line as it
// streamed by and can make its own determination; HadConflicts stays false
// here rather than duplicating an inexact heuristic in two places.
func (e *Executor) PullStream(ctx context.Context, repoPath string, sink func(domain.GitProgressLine) error) error {
	return runStreamed(ctx, repoPath, sink, "pull")
}

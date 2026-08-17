// Package localgit implements usecase.GitExecutor by shelling out to the
// host's `git` binary directly — the host-local case from
// specs/backend-go/services/git-gateway-service.md §2 step 3: used when
// ConnectionResolver reports the target worktree has no connectionId (runs
// on the same host as this service). Real git semantics live in `git`
// itself (§2); this package only invokes it and translates stdout/exit
// codes to domain types.
//
// Commands here stick to `git status --porcelain=v1 -b` and plain `git
// diff`/`add`/`commit`/`push`/`pull` — all available since Git 2.5, well
// under the Git 2.25 baseline in docs/reference/git-compatibility.md, so no
// GitCapabilityCache fallback logic is needed for this operation set.
package localgit

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// Executor is a real, os/exec-backed usecase.GitExecutor.
type Executor struct{}

func New() *Executor {
	return &Executor{}
}

// run executes `git <args...>` with its working directory set to repoPath
// and returns combined stdout (stderr is folded in only on failure, so a
// successful call's output stays exactly what a script parsing it expects).
func (e *Executor) run(ctx context.Context, repoPath string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// GetStatus runs `git status --porcelain=v1 -b` and parses its stable,
// script-friendly output into domain.GitStatus.
func (e *Executor) GetStatus(ctx context.Context, repoPath string) (domain.GitStatus, error) {
	out, err := e.run(ctx, repoPath, "status", "--porcelain=v1", "-b")
	if err != nil {
		return domain.GitStatus{}, err
	}
	return parsePorcelainStatus(out), nil
}

// GetDiff runs `git diff` (or `git diff --staged`) and passes its output
// through unparsed — §2: this service does not parse diffs beyond what's
// needed to pass them through.
func (e *Executor) GetDiff(ctx context.Context, repoPath string, staged bool) (domain.DiffResult, error) {
	args := []string{"diff"}
	if staged {
		args = append(args, "--staged")
	}
	out, err := e.run(ctx, repoPath, args...)
	if err != nil {
		return domain.DiffResult{}, err
	}
	return domain.DiffResult{UnifiedDiff: out}, nil
}

// Commit stages the given paths (or everything, if paths is empty — mirrors
// CommitRequest's "empty = all staged" doc comment by staging first) and
// commits, returning the resulting SHA.
func (e *Executor) Commit(ctx context.Context, repoPath, message string, paths []string) (domain.CommitResult, error) {
	addArgs := []string{"add"}
	if len(paths) == 0 {
		addArgs = append(addArgs, "-A")
	} else {
		addArgs = append(addArgs, paths...)
	}
	if _, err := e.run(ctx, repoPath, addArgs...); err != nil {
		return domain.CommitResult{}, err
	}

	if _, err := e.run(ctx, repoPath, "commit", "-m", message); err != nil {
		return domain.CommitResult{}, err
	}

	sha, err := e.run(ctx, repoPath, "rev-parse", "HEAD")
	if err != nil {
		return domain.CommitResult{}, err
	}
	return domain.CommitResult{CommitSHA: strings.TrimSpace(sha)}, nil
}

// Push runs `git push [remote [branch]]`.
func (e *Executor) Push(ctx context.Context, repoPath, remote, branch string) (domain.PushResult, error) {
	args := []string{"push"}
	if remote != "" {
		args = append(args, remote)
		if branch != "" {
			args = append(args, branch)
		}
	}
	if _, err := e.run(ctx, repoPath, args...); err != nil {
		return domain.PushResult{}, err
	}
	return domain.PushResult{Success: true}, nil
}

// Pull runs `git pull`. A merge conflict is a domain outcome
// (PullResult.HadConflicts), not a Go error — the caller asked for a pull
// and got one; it just didn't resolve cleanly, mirroring how the Dev Server
// Agent's git handler reports conflicts as data, per §2's "real git
// semantics... defined by... git itself" framing.
func (e *Executor) Pull(ctx context.Context, repoPath string) (domain.PullResult, error) {
	out, err := e.run(ctx, repoPath, "pull")
	if err != nil {
		if strings.Contains(out, "CONFLICT") || strings.Contains(err.Error(), "CONFLICT") {
			return domain.PullResult{Success: false, HadConflicts: true}, nil
		}
		return domain.PullResult{}, err
	}
	return domain.PullResult{Success: true}, nil
}

// parsePorcelainStatus parses `git status --porcelain=v1 -b` output. The
// first line is "## <branch>[...tracking info]"; subsequent lines are two
// status-code characters (index, worktree), a space, and the path.
func parsePorcelainStatus(out string) domain.GitStatus {
	var status domain.GitStatus
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		if i == 0 && strings.HasPrefix(line, "## ") {
			status.Branch = parseBranchLine(strings.TrimPrefix(line, "## "))
			continue
		}
		if len(line) < 4 {
			continue
		}
		code := line[:2]
		path := strings.TrimSpace(line[2:])
		status.Files = append(status.Files, domain.FileStatus{
			Path:  path,
			State: fileStateFromPorcelainCode(code),
		})
	}
	return status
}

// parseBranchLine extracts the branch name from porcelain -b's header line,
// stripping tracking info (e.g. "main...origin/main [ahead 1]" -> "main")
// and the "no branch" / detached-HEAD case's own formatting.
func parseBranchLine(header string) string {
	if idx := strings.Index(header, "..."); idx >= 0 {
		header = header[:idx]
	}
	if idx := strings.Index(header, " "); idx >= 0 {
		header = header[:idx]
	}
	return header
}

// fileStateFromPorcelainCode maps a porcelain v1 two-character XY code to
// domain.FileState. Conflict codes (both sides changed) take priority since
// they need surfacing distinctly from an ordinary modification.
func fileStateFromPorcelainCode(code string) domain.FileState {
	switch code {
	case "??":
		return domain.FileStateUntracked
	case "UU", "AA", "DD", "AU", "UA", "UD", "DU":
		return domain.FileStateConflicted
	case "R ", " R", "RM", "MR":
		return domain.FileStateRenamed
	}
	x, y := code[0], code[1]
	switch {
	case x == 'A' || y == 'A':
		return domain.FileStateAdded
	case x == 'D' || y == 'D':
		return domain.FileStateDeleted
	default:
		return domain.FileStateModified
	}
}

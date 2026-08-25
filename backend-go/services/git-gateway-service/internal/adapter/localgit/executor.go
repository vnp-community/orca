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

// GetDiff runs `git diff [--staged] -- <filePath>` — per-file, matching the
// real Dev Server Agent contract this local path must stay consistent with
// (see BUG-036/TASK-228).
func (e *Executor) GetDiff(ctx context.Context, repoPath, filePath string, staged bool) (domain.DiffResult, error) {
	args := []string{"diff"}
	if staged {
		args = append(args, "--staged")
	}
	args = append(args, "--", filePath)
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

// Stage runs `git add -- <paths...>`.
func (e *Executor) Stage(ctx context.Context, repoPath string, paths []string) (domain.SimpleResult, error) {
	args := append([]string{"add", "--"}, paths...)
	if _, err := e.run(ctx, repoPath, args...); err != nil {
		return domain.SimpleResult{}, err
	}
	return domain.SimpleResult{Success: true}, nil
}

// Unstage runs `git restore --staged -- <paths...>` (Git 2.23+, well under
// this service's Git 2.5 baseline for the rest of its command set — but
// still above the project's Git 2.25 compatibility floor per
// docs/reference/git-compatibility.md, so no `git reset HEAD --` fallback
// branch is needed).
func (e *Executor) Unstage(ctx context.Context, repoPath string, paths []string) (domain.SimpleResult, error) {
	args := append([]string{"restore", "--staged", "--"}, paths...)
	if _, err := e.run(ctx, repoPath, args...); err != nil {
		return domain.SimpleResult{}, err
	}
	return domain.SimpleResult{Success: true}, nil
}

// History runs `git log` with a stable, tab-delimited format. cursor
// support removed — the real agent has no pagination concept, so this
// local implementation matches it rather than offering a richer feature
// the relay side can't provide (TASK-209's Contract correction section).
func (e *Executor) History(ctx context.Context, repoPath, baseRef string, limit int) ([]domain.CommitRef, error) {
	target := baseRef
	if target == "" {
		target = "HEAD"
	}
	args := []string{"log", target, `--format=%H%x09%an%x09%cn%x09%at%x09%P%x09%s`}
	if limit > 0 {
		args = append(args, fmt.Sprintf("-%d", limit))
	}
	out, err := e.run(ctx, repoPath, args...)
	if err != nil {
		return nil, err
	}
	var commits []domain.CommitRef
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) < 6 {
			continue
		}
		var ts int64
		fmt.Sscanf(f[3], "%d", &ts)
		var parents []string
		if f[4] != "" {
			parents = strings.Split(f[4], " ")
		}
		commits = append(commits, domain.CommitRef{
			SHA: f[0], Author: f[1], Committer: f[2], Timestamp: ts,
			ParentSHAs: parents, Message: f[5],
		})
	}
	return commits, nil
}

// CheckIgnored runs `git check-ignore` per path (its own exit code — 0 for
// ignored, 1 for not — makes a single multi-path invocation ambiguous
// about which paths matched, so this loops rather than parsing combined
// output). Returns only the ignored subset, matching the real agent's
// response shape (TASK-209's Contract correction section).
func (e *Executor) CheckIgnored(ctx context.Context, repoPath string, paths []string) ([]string, error) {
	var ignored []string
	for _, p := range paths {
		if _, err := e.run(ctx, repoPath, "check-ignore", "--quiet", p); err == nil {
			ignored = append(ignored, p)
		}
	}
	return ignored, nil
}

// ForkSync compares HEAD against expectedUpstream (a caller-supplied
// remote-tracking ref, e.g. "origin/main") — expectedUpstream is a
// required param, matching the real agent (TASK-209's Contract correction
// section).
func (e *Executor) ForkSync(ctx context.Context, repoPath, expectedUpstream string) (domain.ForkSyncStatus, error) {
	ahead, behind, err := e.aheadBehind(ctx, repoPath, "HEAD", expectedUpstream)
	if err != nil {
		return domain.ForkSyncStatus{}, err
	}
	return domain.ForkSyncStatus{Ahead: ahead, Behind: behind, Diverged: ahead > 0 && behind > 0}, nil
}

// UpstreamStatus reads the current branch's configured @{upstream}.
// pushTarget is accepted for signature parity with the relay path (real
// git.upstreamStatus takes an optional pushTarget) but unused locally —
// local execution reads the branch's actual git config directly rather
// than resolving a push target.
func (e *Executor) UpstreamStatus(ctx context.Context, repoPath, pushTarget string) (domain.UpstreamStatus, error) {
	upstream, err := e.run(ctx, repoPath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err != nil {
		// No upstream configured is a domain outcome, not a Go error.
		return domain.UpstreamStatus{HasUpstream: false}, nil
	}
	ahead, behind, err := e.aheadBehind(ctx, repoPath, "HEAD", strings.TrimSpace(upstream))
	if err != nil {
		return domain.UpstreamStatus{}, err
	}
	return domain.UpstreamStatus{HasUpstream: true, Ahead: ahead, Behind: behind}, nil
}

// aheadBehind runs `git rev-list --left-right --count a...b` and parses its
// two-column tab-separated output.
func (e *Executor) aheadBehind(ctx context.Context, repoPath, a, b string) (ahead, behind int, err error) {
	out, err := e.run(ctx, repoPath, "rev-list", "--left-right", "--count", a+"..."+b)
	if err != nil {
		return 0, 0, err
	}
	fields := strings.Fields(out)
	if len(fields) != 2 {
		return 0, 0, fmt.Errorf("localgit: unexpected rev-list output %q", out)
	}
	fmt.Sscanf(fields[0], "%d", &ahead)
	fmt.Sscanf(fields[1], "%d", &behind)
	return ahead, behind, nil
}

// RemoteCommitURL resolves origin's URL and pattern-matches the host
// (github.com/gitlab.com/bitbucket.org) to build a commit permalink.
func (e *Executor) RemoteCommitURL(ctx context.Context, repoPath, sha string) (string, error) {
	base, err := e.remoteWebBaseURL(ctx, repoPath)
	if err != nil {
		return "", err
	}
	return base + "/commit/" + sha, nil
}

// RemoteFileURL resolves origin's URL and builds a file-at-ref permalink.
// GitHub/GitLab both use "/blob/<ref>/<path>"; Bitbucket uses
// "/src/<ref>/<path>" — branch on host.
func (e *Executor) RemoteFileURL(ctx context.Context, repoPath, path, ref string) (string, error) {
	base, err := e.remoteWebBaseURL(ctx, repoPath)
	if err != nil {
		return "", err
	}
	if strings.Contains(base, "bitbucket.org") {
		return base + "/src/" + ref + "/" + path, nil
	}
	return base + "/blob/" + ref + "/" + path, nil
}

// remoteWebBaseURL converts `git remote get-url origin`'s SSH or HTTPS form
// into a browsable https://<host>/<org>/<repo> base URL.
func (e *Executor) remoteWebBaseURL(ctx context.Context, repoPath string) (string, error) {
	raw, err := e.run(ctx, repoPath, "remote", "get-url", "origin")
	if err != nil {
		return "", err
	}
	url := strings.TrimSpace(raw)
	url = strings.TrimSuffix(url, ".git")
	if strings.HasPrefix(url, "git@") {
		// git@host:org/repo -> https://host/org/repo
		url = strings.TrimPrefix(url, "git@")
		url = strings.Replace(url, ":", "/", 1)
		url = "https://" + url
	}
	return url, nil
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

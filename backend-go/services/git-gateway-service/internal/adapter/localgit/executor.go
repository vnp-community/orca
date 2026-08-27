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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
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
// git.upstreamStatus takes an optional, now-structured pushTarget per
// TASK-209/210's contract correction) but unused locally — local execution
// reads the branch's actual git config directly rather than resolving a
// push target.
func (e *Executor) UpstreamStatus(ctx context.Context, repoPath string, pushTarget *domain.PushTargetInput) (domain.UpstreamStatus, error) {
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

// Clone runs `git clone <url> <destPath>` and reads back the resulting
// default branch with `git symbolic-ref --short HEAD`.
func (e *Executor) Clone(ctx context.Context, url, destPath string) (string, string, error) {
	cmd := exec.CommandContext(ctx, "git", "clone", url, destPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("git clone: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	branch, err := e.run(ctx, destPath, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return destPath, "", err
	}
	return destPath, strings.TrimSpace(branch), nil
}

// InitRepo runs `git init` (optionally with -b <defaultBranch>, Git 2.28+;
// falls back to a plain `git init` + `git symbolic-ref` rename for older
// Git per docs/reference/git-compatibility.md's 2.25 baseline) at destPath.
func (e *Executor) InitRepo(ctx context.Context, destPath, defaultBranch string) (string, string, error) {
	if err := os.MkdirAll(destPath, 0o755); err != nil {
		return "", "", fmt.Errorf("mkdir dest path: %w", err)
	}
	args := []string{"init"}
	if defaultBranch != "" {
		args = append(args, "-b", defaultBranch)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = destPath
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("git init: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	branch, err := e.run(ctx, destPath, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return destPath, defaultBranch, nil // best-effort: init succeeded even if branch read fails
	}
	return destPath, strings.TrimSpace(branch), nil
}

// BaseRefDefault resolves the remote's default branch via
// `git symbolic-ref refs/remotes/origin/HEAD` (falls back to `git remote
// show origin`'s "HEAD branch:" line pre-Git 2.8, if that boundary ever
// matters for this baseline).
func (e *Executor) BaseRefDefault(ctx context.Context, repoPath string) (string, error) {
	out, err := e.run(ctx, repoPath, "symbolic-ref", "refs/remotes/origin/HEAD")
	if err != nil {
		return "", err
	}
	// "refs/remotes/origin/main" -> "main"
	ref := strings.TrimSpace(out)
	if idx := strings.LastIndex(ref, "/"); idx >= 0 {
		ref = ref[idx+1:]
	}
	return ref, nil
}

// SearchRefs runs `git for-each-ref` filtered by query as a substring match
// over ref short names.
func (e *Executor) SearchRefs(ctx context.Context, repoPath, query string) ([]string, error) {
	out, err := e.run(ctx, repoPath, "for-each-ref", "--format=%(refname:short)")
	if err != nil {
		return nil, err
	}
	var matched []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line != "" && (query == "" || strings.Contains(line, query)) {
			matched = append(matched, line)
		}
	}
	return matched, nil
}

// CheckHooks lists installed hooks under .git/hooks and reports whether
// orca's own hooks (pre-commit, post-checkout — the two orca installs, per
// this scaffold's own install-hooks convention) are present and current.
// "Current" here means present at all — this scaffold does not diff hook
// content against a known-good version; see this service's README "Known
// gaps" if that stronger check is ever needed.
func (e *Executor) CheckHooks(ctx context.Context, repoPath string) ([]string, bool, error) {
	entries, err := os.ReadDir(filepath.Join(repoPath, ".git", "hooks"))
	if err != nil {
		return nil, false, fmt.Errorf("read hooks dir: %w", err)
	}
	var installed []string
	hasPreCommit, hasPostCheckout := false, false
	for _, entry := range entries {
		if entry.IsDir() || strings.HasSuffix(entry.Name(), ".sample") {
			continue
		}
		installed = append(installed, entry.Name())
		switch entry.Name() {
		case "pre-commit":
			hasPreCommit = true
		case "post-checkout":
			hasPostCheckout = true
		}
	}
	return installed, hasPreCommit && hasPostCheckout, nil
}

// issueCommandPath is the well-known location orca writes/reads its
// issue-command config from, relative to the repo root.
const issueCommandPath = ".orca/issue-command.json"

// ReadIssueCommand reads the issue-command config file, reporting
// exists=false (not an error) when it hasn't been created yet.
func (e *Executor) ReadIssueCommand(ctx context.Context, repoPath string) (string, bool, error) {
	data, err := os.ReadFile(filepath.Join(repoPath, issueCommandPath))
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read issue command file: %w", err)
	}
	return string(data), true, nil
}

// WriteIssueCommand writes the issue-command config file, creating the
// .orca/ directory if it doesn't exist yet.
func (e *Executor) WriteIssueCommand(ctx context.Context, repoPath, content string) error {
	dir := filepath.Join(repoPath, ".orca")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir .orca: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "issue-command.json"), []byte(content), 0o644); err != nil {
		return fmt.Errorf("write issue command file: %w", err)
	}
	return nil
}

// ScanSetupScriptImports reads .orca/setup.sh (or setup.ts/setup.js, in
// that preference order) and returns any relative paths its `source`/
// `import`/`require` lines reference — a best-effort static scan, not a
// real shell/JS parser.
func (e *Executor) ScanSetupScriptImports(ctx context.Context, repoPath string) ([]string, error) {
	candidates := []string{"setup.sh", "setup.ts", "setup.js"}
	var script []byte
	for _, name := range candidates {
		data, err := os.ReadFile(filepath.Join(repoPath, ".orca", name))
		if err == nil {
			script = data
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("read setup script %s: %w", name, err)
		}
	}
	if script == nil {
		return []string{}, nil
	}
	var imports []string
	for _, line := range strings.Split(string(script), "\n") {
		line = strings.TrimSpace(line)
		for _, prefix := range []string{"source ", "import ", "require("} {
			if strings.HasPrefix(line, prefix) {
				imports = append(imports, line)
			}
		}
	}
	return imports, nil
}

// minFreeSpaceBytes is [A3]'s soft-warning threshold — no spec-given number
// exists for "cảnh báo dung lượng disk" (BL-WT-01), 500MB is a conservative
// default worth revisiting once product specifies one.
const minFreeSpaceBytes = 500 * 1024 * 1024

// CreateWorktree runs `git worktree add <targetPath> -b <branch> <baseRef>`.
// targetPath, if non-empty, overrides the default repoPath + "-" + branch
// convention (mirrors the old TS backend's convention: worktree path =
// repo root's parent dir / branch name, sanitized) — see SOL-WT-01's
// custom name/path input support.
func (e *Executor) CreateWorktree(ctx context.Context, repoPath, branch, baseRef, targetPath string) (domain.WorktreeCreateResult, error) {
	if targetPath == "" {
		targetPath = repoPath + "-" + sanitizeBranchForPath(branch)
	}
	if _, err := e.run(ctx, repoPath, "worktree", "add", targetPath, "-b", branch, baseRef); err != nil {
		return domain.WorktreeCreateResult{}, err
	}
	sha, err := e.run(ctx, targetPath, "rev-parse", "HEAD")
	if err != nil {
		return domain.WorktreeCreateResult{}, err
	}
	return domain.WorktreeCreateResult{Path: targetPath, HeadSHA: strings.TrimSpace(sha)}, nil
}

// RemoveWorktree runs `git worktree remove [--force] <worktreePath>`. Run
// from the MAIN repo's directory is not required — git worktree remove
// accepts an absolute path to the worktree itself.
func (e *Executor) RemoveWorktree(ctx context.Context, worktreePath string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, worktreePath)
	_, err := e.run(ctx, worktreePath, args...)
	return err
}

// FetchAndResolveRef runs `git fetch origin <ref>` then resolves its local
// SHA via `git rev-parse FETCH_HEAD`.
func (e *Executor) FetchAndResolveRef(ctx context.Context, repoPath, ref string) (string, error) {
	if _, err := e.run(ctx, repoPath, "fetch", "origin", ref); err != nil {
		return "", err
	}
	sha, err := e.run(ctx, repoPath, "rev-parse", "FETCH_HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(sha), nil
}

// ListWorktreePaths runs `git worktree list --porcelain` and extracts
// every `worktree <path>` line — the raw on-disk truth DetectWorktrees
// needs, with no bookkeeping join.
func (e *Executor) ListWorktreePaths(ctx context.Context, repoPath string) ([]string, error) {
	out, err := e.run(ctx, repoPath, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		if p, ok := strings.CutPrefix(line, "worktree "); ok {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

// ForceDeleteBranch runs `git branch -D <branch>` — force delete, no
// merge-check (the caller has already decided this branch's worktree is
// being torn down). Available since Git 2.5, same baseline as this
// package's other commands. Required on GitExecutor (TASK-194): the
// structural fix for the old TS backend's optional
// forceDeletePreservedBranch? crash-bug class (BUG-031) — every
// GitExecutor implementation, including this one, must have this method
// before the package even builds.
func (e *Executor) ForceDeleteBranch(ctx context.Context, repoPath, branch string) error {
	_, err := e.run(ctx, repoPath, "branch", "-D", branch)
	return err
}

// sanitizeBranchForPath replaces path-hostile characters ('/' from e.g.
// "feature/foo") so the worktree's directory name is filesystem-safe.
func sanitizeBranchForPath(branch string) string {
	return strings.ReplaceAll(branch, "/", "-")
}

// ── Group A — branch/ref operations (TASK-207) ─────────────────────────────

// Checkout runs `git checkout <branch> --`, matching the real agent's own
// git.checkout exactly (specs/agent/api/agent-rpc-catalog-git-fs.md:132,
// agent/src/relay/git-handler.ts:702-719) — no create-branch (-b) semantics.
// See CheckoutRequest's proto doc comment for why TASK-207's original
// `create` param was dropped rather than kept as a no-op.
func (e *Executor) Checkout(ctx context.Context, repoPath, branch string) (domain.CheckoutResult, error) {
	if _, err := e.run(ctx, repoPath, "checkout", branch, "--"); err != nil {
		return domain.CheckoutResult{}, err
	}
	current, err := e.run(ctx, repoPath, "branch", "--show-current")
	if err != nil {
		return domain.CheckoutResult{}, err
	}
	return domain.CheckoutResult{Success: true, Branch: strings.TrimSpace(current)}, nil
}

// ListLocalBranches runs `git for-each-ref` against refs/heads with a
// machine-parseable format — ahead/behind and upstream come from
// %(upstream:short)/%(upstream:track) tokens. Richer than the real agent's
// own git.localBranches response (names only); see BranchInfo's proto doc
// comment for why RelayExecutor composes the same richer shape via
// git.exec's for-each-ref instead of calling git.localBranches directly —
// this local implementation stays consistent with that choice rather than
// mirroring the narrower real agent response.
func (e *Executor) ListLocalBranches(ctx context.Context, repoPath string) ([]domain.BranchInfo, error) {
	out, err := e.run(ctx, repoPath,
		"for-each-ref", "--format=%(refname:short)\t%(upstream:short)\t%(upstream:track)\t%(HEAD)",
		"refs/heads/")
	if err != nil {
		return nil, err
	}
	var branches []domain.BranchInfo
	// Split raw output (not TrimSpace(out)) before trimming each line — the
	// %(HEAD) column is intentionally empty for every non-current branch,
	// so TrimSpace on the whole blob would eat a legitimate trailing empty
	// field whenever the alphabetically-last ref isn't the current branch,
	// silently dropping it.
	for _, rawLine := range strings.Split(out, "\n") {
		line := strings.TrimRight(rawLine, "\r")
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 4 {
			continue
		}
		ahead, behind := parseAheadBehind(fields[2])
		branches = append(branches, domain.BranchInfo{
			Name:      fields[0],
			Upstream:  fields[1],
			Ahead:     ahead,
			Behind:    behind,
			IsCurrent: fields[3] == "*",
		})
	}
	return branches, nil
}

// FastForward runs `git pull --ff-only [<remote> <branch>]` — matches the
// real agent's git.fastForward (`pullWithArgs(['--ff-only'])`,
// agent-rpc-catalog-git-fs.md:160, agent/src/relay/git-handler.ts:1190-1192)
// instead of TASK-207's original `git merge --ff-only <branch>` sketch,
// which assumed a bare local-branch merge rather than a pull against a
// resolved push target. pushTarget == nil lets `git pull` use the branch's
// configured upstream, mirroring the real agent's own undefined-pushTarget
// fallback (agent/src/relay/git-handler-push-target.ts:164-166).
func (e *Executor) FastForward(ctx context.Context, repoPath string, pushTarget *domain.PushTargetInput) (domain.FastForwardResult, error) {
	args := []string{"pull", "--ff-only"}
	if pushTarget != nil {
		args = append(args, pushTarget.RemoteName, pushTarget.BranchName)
	}
	if _, err := e.run(ctx, repoPath, args...); err != nil {
		return domain.FastForwardResult{}, err
	}
	sha, err := e.run(ctx, repoPath, "rev-parse", "HEAD")
	if err != nil {
		return domain.FastForwardResult{}, err
	}
	return domain.FastForwardResult{Success: true, ResultSHA: strings.TrimSpace(sha)}, nil
}

// RebaseFromBase runs `git rebase <baseRef>`. A conflict (nonzero exit with
// rebase state left behind) is a domain outcome, not a Go error — same
// posture as Pull's conflict handling above.
func (e *Executor) RebaseFromBase(ctx context.Context, repoPath, baseRef string) (domain.RebaseResult, error) {
	out, err := e.run(ctx, repoPath, "rebase", baseRef)
	if err != nil {
		if strings.Contains(out, "CONFLICT") || strings.Contains(err.Error(), "CONFLICT") {
			return domain.RebaseResult{Success: false, HadConflicts: true}, nil
		}
		return domain.RebaseResult{}, err
	}
	return domain.RebaseResult{Success: true}, nil
}

// AbortRebase runs `git rebase --abort`.
func (e *Executor) AbortRebase(ctx context.Context, repoPath string) (domain.SimpleResult, error) {
	if _, err := e.run(ctx, repoPath, "rebase", "--abort"); err != nil {
		return domain.SimpleResult{}, err
	}
	return domain.SimpleResult{Success: true}, nil
}

// AbortMerge runs `git merge --abort`.
func (e *Executor) AbortMerge(ctx context.Context, repoPath string) (domain.SimpleResult, error) {
	if _, err := e.run(ctx, repoPath, "merge", "--abort"); err != nil {
		return domain.SimpleResult{}, err
	}
	return domain.SimpleResult{Success: true}, nil
}

// conflictedPaths runs `git diff --name-only --diff-filter=U` — the
// cheapest way to list unmerged/conflicted paths, matching what git itself
// considers "conflicted" rather than a marker-file scan.
func (e *Executor) conflictedPaths(ctx context.Context, repoPath string) ([]string, error) {
	out, err := e.run(ctx, repoPath, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			paths = append(paths, line)
		}
	}
	return paths, nil
}

// MergeBranch runs `git merge --no-ff <branch>` ("merge" strategy) or
// `git merge --squash <branch>` followed by a commit ("squash" strategy).
// "rebase" is handled entirely by the caller composing RebaseFromBase+
// FastForward — this method is never called for that strategy, see
// MergeWorktreeIntoBase.Execute. A conflict (nonzero exit with CONFLICT
// markers) is a domain outcome, not a Go error — same posture as
// RebaseFromBase's existing conflict-detection posture.
func (e *Executor) MergeBranch(ctx context.Context, repoPath, branch, strategy, commitMessage string) (domain.MergeResult, error) {
	args := []string{"merge"}
	switch strategy {
	case "merge":
		args = append(args, "--no-ff", branch)
	case "squash":
		args = append(args, "--squash", branch)
	default:
		args = append(args, branch)
	}
	out, err := e.run(ctx, repoPath, args...)
	if err != nil {
		if strings.Contains(out, "CONFLICT") || strings.Contains(err.Error(), "CONFLICT") {
			paths, _ := e.conflictedPaths(ctx, repoPath)
			return domain.MergeResult{HasConflicts: true, ConflictedPaths: paths}, nil
		}
		return domain.MergeResult{}, err
	}
	if strategy == "squash" {
		msg := commitMessage
		if msg == "" {
			msg = "Squash merge " + branch
		}
		if _, err := e.run(ctx, repoPath, "commit", "-m", msg); err != nil {
			return domain.MergeResult{}, err
		}
	}
	sha, err := e.run(ctx, repoPath, "rev-parse", "HEAD")
	if err != nil {
		return domain.MergeResult{}, err
	}
	return domain.MergeResult{ResultSHA: strings.TrimSpace(sha)}, nil
}

// resolveGitDir resolves repoPath's actual .git directory, following a
// linked worktree's "gitdir: <path>" pointer file — mirrors the real
// agent's own resolveGitDir exactly
// (agent/src/relay/git-handler-status-ops.ts:24-36), which ConflictOperation
// below needs to check the same MERGE_HEAD/rebase-merge/CHERRY_PICK_HEAD
// marker files the real agent checks.
func resolveGitDir(repoPath string) string {
	dotGitPath := filepath.Join(repoPath, ".git")
	data, err := os.ReadFile(dotGitPath)
	if err != nil {
		return dotGitPath // ".git" is a directory, not a file — the normal (non-worktree) case
	}
	for _, line := range strings.Split(string(data), "\n") {
		if target, ok := strings.CutPrefix(strings.TrimSpace(line), "gitdir:"); ok {
			target = strings.TrimSpace(target)
			if filepath.IsAbs(target) {
				return target
			}
			return filepath.Join(repoPath, target)
		}
	}
	return dotGitPath
}

// ConflictOperation is a DETECTOR ONLY, matching the real agent's
// detectConflictOperation exactly
// (agent/src/relay/git-handler-status-ops.ts:38-57,
// specs/agent/api/agent-rpc-catalog-git-fs.md:136): presence of MERGE_HEAD /
// rebase-merge|rebase-apply / CHERRY_PICK_HEAD inside the resolved .git dir
// determines the in-progress operation. See GitExecutor.ConflictOperation's
// doc comment for why this takes no path/operation params, unlike
// TASK-207's original sketch (see ResolveConflict for that op).
func (e *Executor) ConflictOperation(ctx context.Context, repoPath string) (string, error) {
	gitDir := resolveGitDir(repoPath)
	if _, err := os.Stat(filepath.Join(gitDir, "MERGE_HEAD")); err == nil {
		return "merge", nil
	}
	if _, err := os.Stat(filepath.Join(gitDir, "rebase-merge")); err == nil {
		return "rebase", nil
	}
	if _, err := os.Stat(filepath.Join(gitDir, "rebase-apply")); err == nil {
		return "rebase", nil
	}
	if _, err := os.Stat(filepath.Join(gitDir, "CHERRY_PICK_HEAD")); err == nil {
		return "cherry-pick", nil
	}
	return "unknown", nil
}

// ResolveConflict resolves one conflicted path: "ours"/"theirs" runs
// `git checkout --ours|--theirs -- <path>` then re-stages it (the checkout
// alone only updates the worktree copy); "markResolved" just stages the
// path as-is (the caller already edited it by hand). No real agent RPC
// backs this over relay — see ResolveConflictRequest's proto doc comment;
// this local implementation is the only one that does real work.
func (e *Executor) ResolveConflict(ctx context.Context, repoPath, path, operation string) (domain.SimpleResult, error) {
	switch operation {
	case "ours":
		if _, err := e.run(ctx, repoPath, "checkout", "--ours", "--", path); err != nil {
			return domain.SimpleResult{}, err
		}
	case "theirs":
		if _, err := e.run(ctx, repoPath, "checkout", "--theirs", "--", path); err != nil {
			return domain.SimpleResult{}, err
		}
	case "markResolved":
		// no worktree change — the caller already resolved the content.
	default:
		return domain.SimpleResult{}, fmt.Errorf("localgit: unknown conflict operation %q", operation)
	}
	if _, err := e.run(ctx, repoPath, "add", "--", path); err != nil {
		return domain.SimpleResult{}, err
	}
	return domain.SimpleResult{Success: true}, nil
}

// Discard restores a tracked path (`git checkout -- <path>`) or removes an
// untracked one (`git clean -f -- <path>`), mirroring TS git.discard's
// untracked-file handling. Which case applies is determined by asking `git
// status --porcelain` for that single path first.
func (e *Executor) Discard(ctx context.Context, repoPath, path string) (domain.SimpleResult, error) {
	out, err := e.run(ctx, repoPath, "status", "--porcelain=v1", "--", path)
	if err != nil {
		return domain.SimpleResult{}, err
	}
	if strings.HasPrefix(strings.TrimSpace(out), "??") {
		if _, err := e.run(ctx, repoPath, "clean", "-f", "--", path); err != nil {
			return domain.SimpleResult{}, err
		}
		return domain.SimpleResult{Success: true}, nil
	}
	if _, err := e.run(ctx, repoPath, "checkout", "--", path); err != nil {
		return domain.SimpleResult{}, err
	}
	return domain.SimpleResult{Success: true}, nil
}

// BulkDiscard calls Discard per path, collecting failures rather than
// stopping at the first one — see BulkDiscardResult's doc comment.
func (e *Executor) BulkDiscard(ctx context.Context, repoPath string, paths []string) (domain.BulkDiscardResult, error) {
	var failed []string
	for _, p := range paths {
		if _, err := e.Discard(ctx, repoPath, p); err != nil {
			failed = append(failed, p)
		}
	}
	return domain.BulkDiscardResult{Success: len(failed) == 0, FailedPaths: failed}, nil
}

// parseAheadBehind parses %(upstream:track)'s "[ahead N, behind M]" (or
// "[ahead N]" / "[behind M]" / "") format.
func parseAheadBehind(track string) (ahead, behind int) {
	track = strings.Trim(track, "[]")
	for _, part := range strings.Split(track, ",") {
		part = strings.TrimSpace(part)
		var n int
		if _, err := fmt.Sscanf(part, "ahead %d", &n); err == nil {
			ahead = n
		}
		if _, err := fmt.Sscanf(part, "behind %d", &n); err == nil {
			behind = n
		}
	}
	return ahead, behind
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

// ── Group C — real compare/diff/submodule shapes (TASK-209's Contract
// correction section, resolved against specs/agent/api/agent-rpc-catalog-git-fs.md
// and the real handler source cited on each method below) ─────────────────

// fullGitObjectIDPattern matches a full 40 (SHA-1) or 64 (SHA-256) hex
// object id — mirrors assertFullGitObjectId
// (agent/src/relay/git-handler-commit-diff-ops.ts:7-13).
var fullGitObjectIDPattern = regexp.MustCompile(`^(?:[0-9a-fA-F]{40}|[0-9a-fA-F]{64})$`)

// gitChangeStats holds one path's added/removed line counts, parsed from
// `git diff --numstat` — mirrors parseNumstat
// (agent/src/shared/git-uncommitted-line-stats.ts:56-76).
type gitChangeStats struct {
	added   int
	removed int
}

// parseNumstatOutput parses `git diff --numstat`/`git diff-tree --numstat`
// output into a per-path lookup.
func parseNumstatOutput(out string) map[string]gitChangeStats {
	stats := make(map[string]gitChangeStats)
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		added, _ := strconv.Atoi(parts[0])
		removed, _ := strconv.Atoi(parts[1])
		stats[parts[2]] = gitChangeStats{added: added, removed: removed}
	}
	return stats
}

// parseChangeStatusChar mirrors parseBranchStatusChar
// (agent/src/relay/git-handler-utils.ts:15-30).
func parseChangeStatusChar(c byte) string {
	switch c {
	case 'M':
		return "modified"
	case 'A':
		return "added"
	case 'D':
		return "deleted"
	case 'R':
		return "renamed"
	case 'C':
		return "copied"
	default:
		return "modified"
	}
}

// parseChangeEntries parses `git diff --name-status`/`git diff-tree
// --name-status` output and joins each path's numstat added/removed counts
// — mirrors parseBranchDiff (agent/src/relay/git-handler-utils.ts:107-134),
// shared by CommitCompare and BranchCompare below.
func parseChangeEntries(nameStatusOut string, stats map[string]gitChangeStats) []domain.GitChangeEntry {
	var entries []domain.GitChangeEntry
	for _, line := range strings.Split(nameStatusOut, "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		rawStatus := parts[0]
		statusChar := byte('M')
		if len(rawStatus) > 0 {
			statusChar = rawStatus[0]
		}
		status := parseChangeStatusChar(statusChar)
		var entry domain.GitChangeEntry
		if strings.HasPrefix(rawStatus, "R") || strings.HasPrefix(rawStatus, "C") {
			if len(parts) < 3 || parts[2] == "" {
				continue
			}
			entry = domain.GitChangeEntry{Path: parts[2], OldPath: parts[1], Status: status}
		} else {
			if len(parts) < 2 || parts[1] == "" {
				continue
			}
			entry = domain.GitChangeEntry{Path: parts[1], Status: status}
		}
		if s, ok := stats[entry.Path]; ok {
			entry.Added = s.added
			entry.Removed = s.removed
		}
		entries = append(entries, entry)
	}
	return entries
}

// shortSHA truncates sha to git's conventional 7-char abbreviation.
func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// firstParentOID parses `git rev-list --parents -n 1 <oid>`'s single-line
// "<oid> [parent1] [parent2...]" output, returning the first parent (empty
// for a root commit) — mirrors parseGitRevListFirstParentOid
// (agent/src/shared/git-rev-list-output.ts).
func firstParentOID(out string) string {
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) < 2 {
		return ""
	}
	return fields[1]
}

// CommitCompare diffs commitID against its own parent (or the empty tree
// for a root commit) — matches the real agent's git.commitCompare exactly
// (agent/src/relay/git-handler-commit-diff-ops.ts:15-122,
// specs/agent/api/agent-rpc-catalog-git-fs.md:50/144), NOT two arbitrary
// commits as TASK-209's original design assumed. commitID must be a full
// 40/64-hex object id.
func (e *Executor) CommitCompare(ctx context.Context, repoPath, commitID string) (domain.CommitCompareResult, error) {
	if !fullGitObjectIDPattern.MatchString(commitID) {
		return domain.CommitCompareResult{}, fmt.Errorf("localgit: commitId must be a full git object id")
	}
	commitOut, err := e.run(ctx, repoPath, "rev-parse", "--verify", "--end-of-options", commitID+"^{commit}")
	if err != nil {
		return domain.CommitCompareResult{
			CompareRef: commitID, BaseRef: "parent", Status: "invalid-commit",
			ErrorMessage: fmt.Sprintf("Commit %s could not be resolved in this repository.", commitID),
		}, nil
	}
	commitOID := strings.TrimSpace(commitOut)
	result := domain.CommitCompareResult{
		CommitOID: commitOID, CompareRef: shortSHA(commitOID), BaseRef: "empty tree", Status: "ready",
	}
	parentsOut, err := e.run(ctx, repoPath, "rev-list", "--parents", "-n", "1", commitOID)
	if err != nil {
		result.Status, result.ErrorMessage = "error", err.Error()
		return result, nil
	}
	parentOID := firstParentOID(parentsOut)
	result.ParentOID = parentOID
	if parentOID != "" {
		result.BaseRef = shortSHA(parentOID)
	}
	var nameStatusArgs, numstatArgs []string
	// Why: root commits have no parent tree; diff-tree --root asks git to
	// compare against the repository's empty tree without hardcoding hash format.
	if parentOID != "" {
		nameStatusArgs = []string{"-c", "core.quotePath=false", "diff", "--name-status", "-M", "-C", parentOID, commitOID}
		numstatArgs = []string{"-c", "core.quotePath=false", "diff", "--numstat", "-M", "-C", parentOID, commitOID}
	} else {
		nameStatusArgs = []string{"-c", "core.quotePath=false", "diff-tree", "--root", "--no-commit-id", "--name-status", "-r", "-M", "-C", commitOID}
		numstatArgs = []string{"-c", "core.quotePath=false", "diff-tree", "--root", "--no-commit-id", "--numstat", "-r", "-M", "-C", commitOID}
	}
	nameStatusOut, err := e.run(ctx, repoPath, nameStatusArgs...)
	if err != nil {
		result.Status, result.ErrorMessage = "error", err.Error()
		return result, nil
	}
	numstatOut, err := e.run(ctx, repoPath, numstatArgs...)
	if err != nil {
		result.Status, result.ErrorMessage = "error", err.Error()
		return result, nil
	}
	result.Entries = parseChangeEntries(nameStatusOut, parseNumstatOutput(numstatOut))
	result.ChangedFiles = len(result.Entries)
	return result, nil
}

// BranchCompare diffs current HEAD against ONE baseRef's merge-base —
// matches the real agent's git.branchCompare exactly
// (agent/src/relay/git-handler-ops.ts:124-214,
// specs/agent/api/agent-rpc-catalog-git-fs.md:49/143), NOT two arbitrary
// branches as TASK-209's original design assumed. baseRef must not start
// with "-" (flag-injection guard, matching git-handler.ts:894-898).
func (e *Executor) BranchCompare(ctx context.Context, repoPath, baseRef string) (domain.BranchCompareResult, error) {
	if strings.HasPrefix(baseRef, "-") {
		return domain.BranchCompareResult{}, fmt.Errorf(`localgit: base ref must not start with "-"`)
	}
	result := domain.BranchCompareResult{BaseRef: baseRef, CompareRef: "HEAD", Status: "loading"}
	if branchOut, err := e.run(ctx, repoPath, "branch", "--show-current"); err == nil {
		if branch := strings.TrimSpace(branchOut); branch != "" {
			result.CompareRef = branch
		}
	}
	headOut, headErr := e.run(ctx, repoPath, "rev-parse", "--verify", "HEAD")
	if headErr != nil {
		// Why: new remote worktrees can be on an unborn branch until the first
		// commit — resolving baseRef alone (no committed branch changes yet)
		// avoids surfacing this as a compare error.
		if baseOut, err := e.run(ctx, repoPath, "rev-parse", "--verify", baseRef); err == nil {
			result.BaseOID = strings.TrimSpace(baseOut)
			result.Status = "ready"
			return result, nil
		}
		result.Status = "unborn-head"
		result.ErrorMessage = "This branch does not have a committed HEAD yet, so compare-to-base is unavailable."
		return result, nil
	}
	result.HeadOID = strings.TrimSpace(headOut)
	baseOut, err := e.run(ctx, repoPath, "rev-parse", "--verify", baseRef)
	if err != nil {
		result.Status = "invalid-base"
		result.ErrorMessage = fmt.Sprintf("Base ref %s could not be resolved in this repository.", baseRef)
		return result, nil
	}
	result.BaseOID = strings.TrimSpace(baseOut)
	mergeBaseOut, err := e.run(ctx, repoPath, "merge-base", result.BaseOID, result.HeadOID)
	if err != nil {
		result.Status = "no-merge-base"
		result.ErrorMessage = fmt.Sprintf("This branch and %s do not share a merge base, so compare-to-base is unavailable.", baseRef)
		return result, nil
	}
	result.MergeBase = strings.TrimSpace(mergeBaseOut)
	nameStatusOut, err := e.run(ctx, repoPath, "-c", "core.quotePath=false", "diff", "--name-status", "-M", "-C", result.MergeBase, result.HeadOID)
	if err != nil {
		result.Status, result.ErrorMessage = "error", err.Error()
		return result, nil
	}
	numstatOut, err := e.run(ctx, repoPath, "-c", "core.quotePath=false", "diff", "--numstat", "-M", "-C", result.MergeBase, result.HeadOID)
	if err != nil {
		result.Status, result.ErrorMessage = "error", err.Error()
		return result, nil
	}
	result.Entries = parseChangeEntries(nameStatusOut, parseNumstatOutput(numstatOut))
	result.ChangedFiles = len(result.Entries)
	if countOut, err := e.run(ctx, repoPath, "rev-list", "--count", result.BaseOID+".."+result.HeadOID); err == nil {
		if n, convErr := strconv.Atoi(strings.TrimSpace(countOut)); convErr == nil {
			result.CommitsAhead = n
		}
	}
	result.Status = "ready"
	return result, nil
}

// binarySniffBytes matches the real agent's isBinaryBuffer heuristic
// (agent/src/shared/binary-buffer.ts:4-12): a NUL byte within the first
// 8192 bytes marks content as binary.
const binarySniffBytes = 8192

func isBinaryContent(content string) bool {
	n := len(content)
	if n > binarySniffBytes {
		n = binarySniffBytes
	}
	return strings.IndexByte(content[:n], 0) >= 0
}

// readBlobAtRef runs `git show <ref>:<path>` and classifies the result as
// binary/text — mirrors readBlobAtOid (agent/src/relay/git-handler-ops.ts:26-43).
// A failed lookup (path doesn't exist at ref, e.g. a newly-added file with
// no parent-side blob) returns empty, non-binary content, matching the real
// agent's own catch-and-default-empty behavior.
func (e *Executor) readBlobAtRef(ctx context.Context, repoPath, ref, path string) (content string, isBinary bool) {
	gitPath := strings.ReplaceAll(path, "\\", "/")
	out, err := e.run(ctx, repoPath, "show", "--end-of-options", ref+":"+gitPath)
	if err != nil {
		return "", false
	}
	return out, isBinaryContent(out)
}

// buildFileDiffResult mirrors buildDiffResult
// (agent/src/relay/git-diff-result.ts:5-38). Binary content is left empty
// even for a previewable image/PDF extension — a deliberate simplification
// vs. the real agent's base64-encoded preview payload, since a raw binary
// blob cannot round-trip through a protobuf string field; consistent with
// this service's existing not-full-parity precedent (see domain.GitStatus's
// doc comment).
func buildFileDiffResult(originalContent, modifiedContent string, originalIsBinary, modifiedIsBinary bool) domain.FileDiffResult {
	if originalIsBinary || modifiedIsBinary {
		return domain.FileDiffResult{Kind: "binary", OriginalIsBinary: originalIsBinary, ModifiedIsBinary: modifiedIsBinary}
	}
	return domain.FileDiffResult{Kind: "text", OriginalContent: originalContent, ModifiedContent: modifiedContent}
}

// CommitDiff returns one required file's before/after content at commitOID
// vs. its (optional) parentOID — matches the real agent's git.commitDiff
// exactly (agent/src/relay/git-handler-commit-diff-ops.ts:124-160,
// specs/agent/api/agent-rpc-catalog-git-fs.md:52/146). Same class of
// per-file-required fix as GetDiff's own TASK-228 correction — the original
// TASK-209 design had no filePath at all and assumed a whole-commit diff.
// parentOID == "" means a root commit: the "before" side is empty (diffed
// against the empty tree), matching commitDiffEntry's own
// undefined-parentOid branch. commitOID/parentOID must be full 40/64-hex
// object ids.
func (e *Executor) CommitDiff(ctx context.Context, repoPath, commitOID, parentOID, filePath, oldPath string) (domain.FileDiffResult, error) {
	if !fullGitObjectIDPattern.MatchString(commitOID) {
		return domain.FileDiffResult{}, fmt.Errorf("localgit: commitOid must be a full git object id")
	}
	if parentOID != "" && !fullGitObjectIDPattern.MatchString(parentOID) {
		return domain.FileDiffResult{}, fmt.Errorf("localgit: parentOid must be a full git object id")
	}
	left := oldPath
	if left == "" {
		left = filePath
	}
	var originalContent string
	var originalIsBinary bool
	if parentOID != "" {
		originalContent, originalIsBinary = e.readBlobAtRef(ctx, repoPath, parentOID, left)
	}
	modifiedContent, modifiedIsBinary := e.readBlobAtRef(ctx, repoPath, commitOID, filePath)
	return buildFileDiffResult(originalContent, modifiedContent, originalIsBinary, modifiedIsBinary), nil
}

// BranchDiff returns one required file's before/after content between HEAD
// and baseRef's merge-base — matches the real agent's git.branchDiff
// exactly (agent/src/relay/git-handler-ops.ts:218-288,
// specs/agent/api/agent-rpc-catalog-git-fs.md:51/145). Same
// per-file-required fix as CommitDiff, plus the same base-ref-only-vs-two-sided
// fix as BranchCompare. baseRef must not start with "-" (same
// flag-injection guard as BranchCompare).
func (e *Executor) BranchDiff(ctx context.Context, repoPath, baseRef, filePath, oldPath string) (domain.FileDiffResult, error) {
	if strings.HasPrefix(baseRef, "-") {
		return domain.FileDiffResult{}, fmt.Errorf(`localgit: base ref must not start with "-"`)
	}
	headOut, err := e.run(ctx, repoPath, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return domain.FileDiffResult{}, err
	}
	headOID := strings.TrimSpace(headOut)
	baseOut, err := e.run(ctx, repoPath, "rev-parse", "--verify", baseRef)
	if err != nil {
		return domain.FileDiffResult{}, err
	}
	mergeBaseOut, err := e.run(ctx, repoPath, "merge-base", strings.TrimSpace(baseOut), headOID)
	if err != nil {
		return domain.FileDiffResult{}, err
	}
	mergeBase := strings.TrimSpace(mergeBaseOut)
	left := oldPath
	if left == "" {
		left = filePath
	}
	originalContent, originalIsBinary := e.readBlobAtRef(ctx, repoPath, mergeBase, left)
	modifiedContent, modifiedIsBinary := e.readBlobAtRef(ctx, repoPath, headOID, filePath)
	return buildFileDiffResult(originalContent, modifiedContent, originalIsBinary, modifiedIsBinary), nil
}

// SubmoduleStatus runs `git status --porcelain=v1 -b` inside the resolved
// submodule directory — matches the real agent's per-submodule
// git.submoduleStatus exactly (agent/src/relay/agent-git-handler-extended.ts:196-230,
// specs/agent/api/agent-rpc-catalog-git-fs.md:55/123), reusing GetStatus's
// own porcelain parser the same way the real agent's getStatusOp is reused
// for both the top-level worktree and a submodule. area is accepted for
// signature parity with the real contract but doesn't change the porcelain
// call locally, and the real agent's own commit-range-diff enrichment for a
// moved submodule pointer isn't replicated here — a deliberate
// simplification, consistent with GetStatus's own not-full-parity
// precedent (see domain.GitStatus's doc comment).
func (e *Executor) SubmoduleStatus(ctx context.Context, repoPath, submodulePath, area string) (domain.GitStatus, error) {
	submoduleDir := filepath.Join(repoPath, submodulePath)
	out, err := e.run(ctx, submoduleDir, "status", "--porcelain=v1", "-b")
	if err != nil {
		return domain.GitStatus{}, err
	}
	return parsePorcelainStatus(out), nil
}

// Fetch runs `git fetch --prune [remote]` — matches the real agent's
// git.fetch exactly (specs/agent/api/agent-rpc-catalog-git-fs.md:155): it
// always prunes, with no separate prune flag. pushTarget's RemoteName (if
// any) is the only field consulted — pushTarget == nil or an empty
// RemoteName lets `git fetch --prune` use the worktree's default remote,
// mirroring the real agent's own undefined-pushTarget fallback
// (agent/src/relay/git-handler-push-target.ts:164-166).
func (e *Executor) Fetch(ctx context.Context, repoPath string, pushTarget *domain.PushTargetInput) (domain.SimpleResult, error) {
	args := []string{"fetch", "--prune"}
	if pushTarget != nil && pushTarget.RemoteName != "" {
		args = append(args, pushTarget.RemoteName)
	}
	if _, err := e.run(ctx, repoPath, args...); err != nil {
		return domain.SimpleResult{}, err
	}
	return domain.SimpleResult{Success: true}, nil
}

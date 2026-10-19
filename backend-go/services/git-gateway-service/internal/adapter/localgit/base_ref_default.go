package localgit

import (
	"context"
	"errors"
	"strings"
)

var errNoDefaultBranch = errors.New("git: repository has no resolvable default branch")

// BaseRefDefault returns the repo's default branch (a short name that may
// contain slashes, e.g. "release/1.x"). Resolution order, mirrored by the
// agent's git.baseRefDefault (agent/src/relay/agent-git-base-ref-resolver.ts):
//
//	(a) refs/remotes/origin/HEAD target (set by `git clone`), if that branch exists
//	(b) `git ls-remote --symref origin HEAD` (repos from `git init` + `remote add`)
//	(c) preferredBranch (the project's configured default) when it is a local branch
//	(d) a conventional local `main`, then `master`
//	(e) the checked-out branch, only for a fresh repo with none of the above
//
// (c)/(d) exist so a local-only repo does not inherit whatever feature branch
// happens to be checked out: a Request without base_ref must start from the
// repo's default branch, not from the working branch.
func (e *Executor) BaseRefDefault(ctx context.Context, repoPath, preferredBranch string) (string, error) {
	out, symErr := e.run(ctx, repoPath, "symbolic-ref", "refs/remotes/origin/HEAD")
	if symErr == nil {
		if branch := branchFromRef(out); branch != "" && e.branchExists(ctx, repoPath, branch) {
			return branch, nil
		}
	}
	if branch, err := e.defaultBranchFromRemoteHead(ctx, repoPath); err == nil && branch != "" {
		return branch, nil
	}
	if branch := branchFromRef(preferredBranch); branch != "" && e.localBranchExists(ctx, repoPath, branch) {
		return branch, nil
	}
	for _, conventional := range []string{"main", "master"} {
		if e.localBranchExists(ctx, repoPath, conventional) {
			return conventional, nil
		}
	}
	// `git symbolic-ref HEAD` resolves even with zero commits (unlike
	// `git branch`), and fails on a detached HEAD, which has no branch name.
	if out, err := e.run(ctx, repoPath, "symbolic-ref", "HEAD"); err == nil {
		if branch := branchFromRef(out); branch != "" {
			return branch, nil
		}
	}
	if symErr != nil {
		return "", symErr
	}
	return "", errNoDefaultBranch
}

// branchExists reports whether branch is a local branch or a remote-tracking
// branch of origin, so a dangling origin/HEAD never wins over a real candidate.
func (e *Executor) branchExists(ctx context.Context, repoPath, branch string) bool {
	return e.localBranchExists(ctx, repoPath, branch) ||
		e.refExists(ctx, repoPath, "refs/remotes/origin/"+branch)
}

func (e *Executor) localBranchExists(ctx context.Context, repoPath, branch string) bool {
	return e.refExists(ctx, repoPath, "refs/heads/"+branch)
}

// refExists uses a fully-qualified ref with --verify, so a branch name that
// starts with "-" can never be read as an option.
func (e *Executor) refExists(ctx context.Context, repoPath, fullRef string) bool {
	_, err := e.run(ctx, repoPath, "show-ref", "--verify", "--quiet", fullRef)
	return err == nil
}

// defaultBranchFromRemoteHead asks the remote itself which branch HEAD
// points at; a fresh `git init` + `git remote add` never gets origin/HEAD.
//
//	ref: refs/heads/main	HEAD
//	<sha>	HEAD
func (e *Executor) defaultBranchFromRemoteHead(ctx context.Context, repoPath string) (string, error) {
	out, err := e.run(ctx, repoPath, "ls-remote", "--symref", "origin", "HEAD")
	if err != nil {
		return "", err
	}
	const prefix = "ref: refs/heads/"
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		rest := strings.TrimPrefix(line, prefix)
		if idx := strings.IndexByte(rest, '\t'); idx >= 0 {
			rest = rest[:idx]
		}
		if rest = strings.TrimSpace(rest); rest != "" {
			return rest, nil
		}
	}
	return "", nil
}

// branchFromRef strips only the known ref prefixes, so slashed branch names
// survive: "refs/heads/feature/x" and "refs/remotes/origin/feature/x" both
// give "feature/x". Remote names never contain "/", so the first segment
// after refs/remotes/ is the remote.
func branchFromRef(ref string) string {
	ref = strings.TrimSpace(ref)
	if rest, ok := strings.CutPrefix(ref, "refs/heads/"); ok {
		return rest
	}
	if rest, ok := strings.CutPrefix(ref, "refs/remotes/"); ok {
		if _, branch, found := strings.Cut(rest, "/"); found {
			return branch
		}
		return ""
	}
	return ref
}

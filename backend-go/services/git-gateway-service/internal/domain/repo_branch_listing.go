package domain

import (
	"sort"
	"strings"
)

// MaxRepoBranches caps ListRepoBranches so a repo with thousands of remote branches cannot flood the picker.
const MaxRepoBranches = 500

// RepoBranch is one entry of the "Start from" list. Name is the short ref: "main" for a local
// branch, "origin/hotfix/x" for a remote-only one (usable as a base ref as-is).
type RepoBranch struct {
	Name   string
	Remote bool
}

const (
	localBranchPrefix  = "refs/heads/"
	remoteBranchPrefix = "refs/remotes/"
)

// RepoBranchListArgs is the git argv (after "git") whose output ParseRepoBranchRefs reads. Both
// executors run exactly this so they cannot drift; flags are git 2.13+ and read-only (the agent's
// git.exec allowlist accepts `branch` without destructive flags).
func RepoBranchListArgs() []string {
	return []string{"branch", "--list", "--all", "--format=%(refname)"}
}

// ParseRepoBranchRefs turns `git branch --list --all --format=%(refname)` output into the ordered,
// deduplicated branch list: local branches by byte order, then remote-only branches by byte order.
// A remote branch is hidden when a local branch carries its name without the remote prefix, and
// <remote>/HEAD is never listed. More than MaxRepoBranches entries are cut and truncated is true.
func ParseRepoBranchRefs(out string) (branches []RepoBranch, truncated bool) {
	var locals, remotes []string
	localSet := make(map[string]struct{})
	for _, line := range strings.Split(out, "\n") {
		ref := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(ref, localBranchPrefix):
			name := strings.TrimPrefix(ref, localBranchPrefix)
			if name == "" {
				continue
			}
			if _, dup := localSet[name]; !dup {
				localSet[name] = struct{}{}
				locals = append(locals, name)
			}
		case strings.HasPrefix(ref, remoteBranchPrefix):
			remotes = append(remotes, strings.TrimPrefix(ref, remoteBranchPrefix))
		}
	}

	sort.Strings(locals)
	sort.Strings(remotes)

	for _, name := range locals {
		branches = append(branches, RepoBranch{Name: name})
	}
	var prev string
	for _, full := range remotes {
		remoteName, branch, ok := strings.Cut(full, "/")
		if !ok || remoteName == "" || branch == "" || branch == "HEAD" || full == prev {
			continue
		}
		prev = full
		// The full-name check keeps a local branch literally named "origin/x" from sharing a Name with remote origin/x.
		_, twin := localSet[branch]
		_, sameName := localSet[full]
		if twin || sameName {
			continue
		}
		branches = append(branches, RepoBranch{Name: full, Remote: true})
	}

	if len(branches) > MaxRepoBranches {
		return branches[:MaxRepoBranches], true
	}
	return branches, false
}

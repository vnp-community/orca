package usecase

import "context"

// MaxProposalReadBytes caps one ReadFile of a proposal document: tasks.md and spec.md are small, and the file comes
// from an agent, so an oversized one is refused rather than read into memory.
const MaxProposalReadBytes = 256 * 1024

// Workspace is the git worktree of one Request's proposal branch, on the dev server that runs the agent.
type Workspace struct {
	WorktreeID string
	// Path is the worktree directory on the dev server; the agent runs there.
	Path    string
	Branch  string
	BaseRef string
}

type EnsureWorkspaceInput struct {
	TenantID  string
	ProjectID string
	// RepoID may be empty: the project's default repo (lowest position) is used, the one tasks execute on.
	RepoID        string
	RequestID     string
	RequestNumber int64
	ChangeID      string
	// BaseRef is the branch or tag the proposal branch starts from; "" means the repository default. Callers pass the Request's
	// value as read, but ReservingProposalWorkspace replaces it with the one stored under the Request lock.
	BaseRef string
}

// ChangedPath is one path git reports as differing from the branch head, including new (untracked), renamed and deleted ones.
type ChangedPath struct {
	Path    string
	OldPath string
	State   string
}

// PathInfo is what Stat knows about one path of the worktree.
type PathInfo struct {
	Exists bool
	IsDir  bool
}

// DirEntry is one name inside a worktree directory.
type DirEntry struct {
	Name  string
	IsDir bool
}

// ProposalWorkspace is the only door to the proposal worktree: Orca never runs git itself. The branch is local and is
// never pushed; Remove keeps it so the proposal stays reviewable after the worktree is gone.
type ProposalWorkspace interface {
	// Ensure is idempotent per Request: a second call returns the same worktree. An existing worktree keeps the base it was cut
	// from (the Request's base ref is frozen once one exists). A base ref that no longer exists fails with REQUEST_BASE_REF_NOT_FOUND.
	Ensure(ctx context.Context, in EnsureWorkspaceInput) (Workspace, error)
	// ReadFile refuses a file larger than maxBytes (at most MaxProposalReadBytes).
	ReadFile(ctx context.Context, ws Workspace, relPath string, maxBytes int) ([]byte, error)
	WriteFile(ctx context.Context, ws Workspace, relPath string, content []byte) error
	// Stat reports a missing path as Exists=false, not as an error.
	Stat(ctx context.Context, ws Workspace, relPath string) (PathInfo, error)
	// ListDir lists one directory; a missing directory is an empty list, not an error.
	ListDir(ctx context.Context, ws Workspace, relPath string) ([]DirEntry, error)
	ChangedPaths(ctx context.Context, ws Workspace) ([]ChangedPath, error)
	// DiscardAll restores tracked files and removes untracked ones, so a retry starts from the branch head.
	DiscardAll(ctx context.Context, ws Workspace) error
	// Commit commits only paths and returns the commit sha.
	Commit(ctx context.Context, ws Workspace, message string, paths []string) (string, error)
	// Remove deletes the worktree and keeps the branch.
	Remove(ctx context.Context, ws Workspace) error
}

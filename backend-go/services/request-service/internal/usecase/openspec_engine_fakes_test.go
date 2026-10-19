package usecase

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// fakeWorkspace is an in-memory proposal worktree: files by relative path, and the paths git would report as changed.
type fakeWorkspace struct {
	mu        sync.Mutex
	ws        Workspace
	files     map[string][]byte
	changed   []ChangedPath
	ensured   int
	discarded int
	commits   []string
	commitErr error
	committed [][]string
	removed   int
	ensureErr error
	// lastEnsure is the input of the latest Ensure, so tests can see which base ref a caller asked for.
	lastEnsure EnsureWorkspaceInput
	discErr    error
	changeErr  error
	readErr    error
}

func newFakeWorkspace() *fakeWorkspace {
	return &fakeWorkspace{ws: Workspace{WorktreeID: "wt-1", Path: "/srv/wt-1", Branch: "request/42-proposal", BaseRef: "main"}, files: map[string][]byte{}}
}

func (f *fakeWorkspace) Ensure(_ context.Context, in EnsureWorkspaceInput) (Workspace, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ensured++
	f.lastEnsure = in
	return f.ws, f.ensureErr
}

func (f *fakeWorkspace) ReadFile(_ context.Context, _ Workspace, rel string, max int) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.readErr != nil {
		return nil, f.readErr
	}
	b, ok := f.files[rel]
	if !ok {
		return nil, errors.New("no such file " + rel)
	}
	if max > 0 && len(b) > max {
		return nil, errors.New("too large")
	}
	return b, nil
}

func (f *fakeWorkspace) WriteFile(_ context.Context, _ Workspace, rel string, b []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.files[rel] = b
	return nil
}

// repoFiles are files of the repository at its base ref, present in every fresh worktree.
func (f *fakeWorkspace) seedRepo(paths ...string) {
	for _, p := range paths {
		f.files[p] = []byte("repo file")
	}
}

func (f *fakeWorkspace) Stat(_ context.Context, _ Workspace, rel string) (PathInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.files[rel]; ok {
		return PathInfo{Exists: true}, nil
	}
	for p := range f.files {
		if len(p) > len(rel) && p[:len(rel)] == rel && p[len(rel)] == '/' {
			return PathInfo{Exists: true, IsDir: true}, nil
		}
	}
	return PathInfo{}, nil
}

func (f *fakeWorkspace) ListDir(_ context.Context, _ Workspace, rel string) ([]DirEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	seen := map[string]bool{}
	var out []DirEntry
	for p := range f.files {
		if len(p) <= len(rel)+1 || p[:len(rel)] != rel || p[len(rel)] != '/' {
			continue
		}
		rest := p[len(rel)+1:]
		name, isDir := rest, false
		for i := 0; i < len(rest); i++ {
			if rest[i] == '/' {
				name, isDir = rest[:i], true
				break
			}
		}
		if !seen[name] {
			seen[name] = true
			out = append(out, DirEntry{Name: name, IsDir: isDir})
		}
	}
	return out, nil
}

func (f *fakeWorkspace) ChangedPaths(context.Context, Workspace) ([]ChangedPath, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]ChangedPath(nil), f.changed...), f.changeErr
}

func (f *fakeWorkspace) DiscardAll(context.Context, Workspace) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.discarded++
	return f.discErr
}

func (f *fakeWorkspace) Commit(_ context.Context, _ Workspace, message string, paths []string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.commitErr != nil {
		return "", f.commitErr
	}
	f.commits = append(f.commits, message)
	f.committed = append(f.committed, paths)
	f.changed = nil // a commit leaves the worktree clean
	return "sha-1", nil
}

func (f *fakeWorkspace) Remove(context.Context, Workspace) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removed++
	return nil
}

// write records a file as an agent would leave it: present, and reported as a new path.
func (f *fakeWorkspace) write(rel, content string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.files[rel] = []byte(content)
	f.changed = append(f.changed, ChangedPath{Path: rel, State: "untracked"})
}

// fakeAgent answers ExecPrompt from a script; onCall lets a test act as the agent before the answer.
type fakeAgent struct {
	mu      sync.Mutex
	results []AgentPromptResult
	errs    []error
	onCall  func(n int, in AgentPromptInput)
	inputs  []AgentPromptInput
}

func (a *fakeAgent) ExecPrompt(_ context.Context, _ AnalysisConnection, in AgentPromptInput) (AgentPromptResult, error) {
	a.mu.Lock()
	n := len(a.inputs)
	a.inputs = append(a.inputs, in)
	cb := a.onCall
	a.mu.Unlock()
	if cb != nil {
		cb(n, in)
	}
	var err error
	if n < len(a.errs) {
		err = a.errs[n]
	}
	if n < len(a.results) {
		return a.results[n], err
	}
	if len(a.results) > 0 {
		return a.results[len(a.results)-1], err
	}
	return AgentPromptResult{}, err
}

func (a *fakeAgent) calls() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.inputs)
}

type specCaps struct {
	features []string
	err      error
}

func (c specCaps) Get(context.Context, DevServerRef, bool) (domain.DevServerCapability, error) {
	return domain.DevServerCapability{Features: c.features}, c.err
}

var authoringFeatures = []string{"agent.execPrompt", "agent.execPrompt.workspaceKind", "agent.execPrompt.changes", "agent.execPrompt.resultBlock", "agent.execPrompt.permissionProfile"}

type fakeGateExec struct{ res ExecResult }

func (g fakeGateExec) Exec(context.Context, AnalysisConnection, ExecInput) (ExecResult, error) {
	return g.res, nil
}

// fakeChanges is the OpenSpec change table in memory, with the real repositories' upsert semantics.
type fakeChanges struct {
	mu     sync.Mutex
	byReq  map[string]domain.OpenSpecChange
	err    error
	upsert int
	// status and updated answer ListWithWorkspace; tests set them by request id.
	status  map[string]domain.RequestStatus
	updated map[string]time.Time
}

func newFakeChanges() *fakeChanges {
	return &fakeChanges{byReq: map[string]domain.OpenSpecChange{}, status: map[string]domain.RequestStatus{}, updated: map[string]time.Time{}}
}

func (f *fakeChanges) GetByRequest(_ context.Context, requestID string) (domain.OpenSpecChange, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.byReq[requestID]
	return c, ok, f.err
}

func (f *fakeChanges) Upsert(_ context.Context, c domain.OpenSpecChange) (domain.OpenSpecChange, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return domain.OpenSpecChange{}, f.err
	}
	f.upsert++
	if prev, ok := f.byReq[c.RequestID]; ok {
		c.ID, c.Version = prev.ID, prev.Version+1
	} else {
		c.Version = 1
	}
	f.byReq[c.RequestID] = c
	return c, nil
}

func (f *fakeChanges) UpdateSync(_ context.Context, id string, u domain.SyncUpdate, expectedVersion int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for req, c := range f.byReq {
		if c.ID != id {
			continue
		}
		if c.Version != expectedVersion {
			return domain.ErrOpenSpecChangeVersionConflict
		}
		c.SyncState, c.SyncDigest, c.SyncError, c.SyncAttempts, c.SyncNextRetryAt = u.State, u.Digest, u.Error, u.Attempts, u.NextRetryAt
		c.Version++
		f.byReq[req] = c
		return nil
	}
	return domain.ErrOpenSpecChangeNotFound
}

// ListSyncWork answers like the repositories: ready, not given up, due, and either pending or of a completed Request.
func (f *fakeChanges) ListSyncWork(context.Context, int) ([]domain.OpenSpecChange, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.OpenSpecChange
	for _, c := range f.byReq {
		due := c.SyncNextRetryAt == nil || !c.SyncNextRetryAt.After(time.Now())
		if c.Status == domain.ChangeStatusReady && c.SyncState != domain.SyncStateFailed && due &&
			(c.SyncState == domain.SyncStatePending || f.status[c.RequestID] == domain.RequestStatusCompleted) {
			out = append(out, c)
		}
	}
	return out, nil
}

// ListWithWorkspace reads the request status through status (request id to status), updated through updated.
func (f *fakeChanges) ListWithWorkspace(context.Context, int) ([]ProposalCleanupCandidate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []ProposalCleanupCandidate
	for _, c := range f.byReq {
		if c.WorkspaceID != "" {
			out = append(out, ProposalCleanupCandidate{Change: c, RequestStatus: f.status[c.RequestID], UpdatedAt: f.updated[c.RequestID]})
		}
	}
	return out, nil
}

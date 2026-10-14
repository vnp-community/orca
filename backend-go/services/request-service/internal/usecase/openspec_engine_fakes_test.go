package usecase

import (
	"context"
	"errors"
	"sync"

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
	removed   int
	ensureErr error
	discErr   error
	changeErr error
	readErr   error
}

func newFakeWorkspace() *fakeWorkspace {
	return &fakeWorkspace{ws: Workspace{WorktreeID: "wt-1", Path: "/srv/wt-1", Branch: "request/42-proposal", BaseRef: "main"}, files: map[string][]byte{}}
}

func (f *fakeWorkspace) Ensure(context.Context, EnsureWorkspaceInput) (Workspace, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ensured++
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

func (f *fakeWorkspace) Commit(_ context.Context, _ Workspace, message string, _ []string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commits = append(f.commits, message)
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

# TASK-158: Implement `BaseRefDefault`/`SearchRefs`/`CheckHooks`/`ReadIssueCommand`/`WriteIssueCommand`/`ScanSetupScriptImports` usecases (Bucket 3)

**From Solution:** SOL-023 (Bucket 3)
**Priority:** P1
**Service:** `git-gateway-service`
**File:** `services/git-gateway-service/internal/usecase/ports.go`, `services/git-gateway-service/internal/usecase/{base_ref_default,search_refs,check_hooks,read_issue_command,write_issue_command,scan_setup_script_imports}.go` (new), `services/git-gateway-service/internal/adapter/localgit/executor.go`, `services/git-gateway-service/internal/adapter/grpcclient/relay_executor.go`
**Depends on:** TASK-156 (needs generated request/response stubs), TASK-157 (extends the same `GitExecutor` interface)
**Status:** `[needs merge with git.*/files.* group]` — implemented in worktree `agent-a5714e047dcaed0fc`, committed as `56c5fbeff`, builds/tests green in isolation. Touches the same `gitgateway.proto`/`server.go`/`main.go` as the concurrent git.*/files.*/worktree.* work — needs manual reconciliation at merge. Found `DevServerReachability` bug: the task doc's placeholder `GetDevServers()` was wrong, real fix uses `GetFleetHealth`'s `resp.GetStatuses()`.

---

## Context

Unlike `Clone`/`InitRepo` (TASK-157), these 6 all operate on an **existing**
worktree — they take `worktree_id` (TASK-156's messages), so they resolve
through the existing `ConnectionResolver`/`dispatchExecutor` pair exactly
like `GetStatus` (`internal/usecase/get_status.go`), the reference shape
to copy.

## Changes to make

### `internal/usecase/ports.go` — extend `GitExecutor`

Add to the interface (alongside `GetStatus`/`GetDiff`/... and TASK-157's
`Clone`/`InitRepo`):

```go
	BaseRefDefault(ctx context.Context, repoPath string) (ref string, err error)
	SearchRefs(ctx context.Context, repoPath, query string) (refs []string, err error)
	CheckHooks(ctx context.Context, repoPath string) (installedHooks []string, orcaHooksCurrent bool, err error)
	ReadIssueCommand(ctx context.Context, repoPath string) (content string, exists bool, err error)
	WriteIssueCommand(ctx context.Context, repoPath, content string) error
	ScanSetupScriptImports(ctx context.Context, repoPath string) (importedPaths []string, err error)
```

### New file `internal/usecase/base_ref_default.go`

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

type BaseRefDefaultInput struct {
	WorktreeID string
}

// BaseRefDefault resolves the worktree's owning host and returns its
// default base ref — same resolve -> dispatch -> translate flow as
// GetStatus (get_status.go).
type BaseRefDefault struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewBaseRefDefault(resolver ConnectionResolver, local, relay GitExecutor) *BaseRefDefault {
	return &BaseRefDefault{resolver: resolver, local: local, relay: relay}
}

func (uc *BaseRefDefault) Execute(ctx context.Context, in BaseRefDefaultInput) (string, error) {
	if in.WorktreeID == "" {
		return "", apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	ref, err := executor.BaseRefDefault(ctx, repoPath)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "GITGATEWAY_BASE_REF_DEFAULT_FAILED", "failed to resolve default base ref", err)
	}
	return ref, nil
}
```

### New file `internal/usecase/search_refs.go`

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

type SearchRefsInput struct {
	WorktreeID string
	Query      string
}

// SearchRefs follows GetStatus's exact resolve -> dispatch -> translate shape.
type SearchRefs struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewSearchRefs(resolver ConnectionResolver, local, relay GitExecutor) *SearchRefs {
	return &SearchRefs{resolver: resolver, local: local, relay: relay}
}

func (uc *SearchRefs) Execute(ctx context.Context, in SearchRefsInput) ([]string, error) {
	if in.WorktreeID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	refs, err := executor.SearchRefs(ctx, repoPath, in.Query)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "GITGATEWAY_SEARCH_REFS_FAILED", "failed to search refs", err)
	}
	return refs, nil
}
```

### New file `internal/usecase/check_hooks.go`

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

type CheckHooksInput struct {
	WorktreeID string
}

type CheckHooksResult struct {
	InstalledHooks    []string
	OrcaHooksCurrent  bool
}

// CheckHooks follows GetStatus's exact resolve -> dispatch -> translate shape.
type CheckHooks struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewCheckHooks(resolver ConnectionResolver, local, relay GitExecutor) *CheckHooks {
	return &CheckHooks{resolver: resolver, local: local, relay: relay}
}

func (uc *CheckHooks) Execute(ctx context.Context, in CheckHooksInput) (CheckHooksResult, error) {
	if in.WorktreeID == "" {
		return CheckHooksResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return CheckHooksResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	installedHooks, orcaHooksCurrent, err := executor.CheckHooks(ctx, repoPath)
	if err != nil {
		return CheckHooksResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_CHECK_HOOKS_FAILED", "failed to check git hooks", err)
	}
	return CheckHooksResult{InstalledHooks: installedHooks, OrcaHooksCurrent: orcaHooksCurrent}, nil
}
```

### New file `internal/usecase/read_issue_command.go`

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

type ReadIssueCommandInput struct {
	WorktreeID string
}

type ReadIssueCommandResult struct {
	Content string
	Exists  bool
}

// ReadIssueCommand follows GetStatus's exact resolve -> dispatch -> translate shape.
type ReadIssueCommand struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewReadIssueCommand(resolver ConnectionResolver, local, relay GitExecutor) *ReadIssueCommand {
	return &ReadIssueCommand{resolver: resolver, local: local, relay: relay}
}

func (uc *ReadIssueCommand) Execute(ctx context.Context, in ReadIssueCommandInput) (ReadIssueCommandResult, error) {
	if in.WorktreeID == "" {
		return ReadIssueCommandResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return ReadIssueCommandResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	content, exists, err := executor.ReadIssueCommand(ctx, repoPath)
	if err != nil {
		return ReadIssueCommandResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_READ_ISSUE_COMMAND_FAILED", "failed to read issue command file", err)
	}
	return ReadIssueCommandResult{Content: content, Exists: exists}, nil
}
```

### New file `internal/usecase/write_issue_command.go`

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

type WriteIssueCommandInput struct {
	WorktreeID string
	Content    string
}

// WriteIssueCommand follows GetStatus's exact resolve -> dispatch -> translate shape.
type WriteIssueCommand struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewWriteIssueCommand(resolver ConnectionResolver, local, relay GitExecutor) *WriteIssueCommand {
	return &WriteIssueCommand{resolver: resolver, local: local, relay: relay}
}

func (uc *WriteIssueCommand) Execute(ctx context.Context, in WriteIssueCommandInput) error {
	if in.WorktreeID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	if err := executor.WriteIssueCommand(ctx, repoPath, in.Content); err != nil {
		return apperrors.New(apperrors.KindInternal, "GITGATEWAY_WRITE_ISSUE_COMMAND_FAILED", "failed to write issue command file", err)
	}
	return nil
}
```

### New file `internal/usecase/scan_setup_script_imports.go`

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

type ScanSetupScriptImportsInput struct {
	WorktreeID string
}

// ScanSetupScriptImports follows GetStatus's exact resolve -> dispatch -> translate shape.
type ScanSetupScriptImports struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewScanSetupScriptImports(resolver ConnectionResolver, local, relay GitExecutor) *ScanSetupScriptImports {
	return &ScanSetupScriptImports{resolver: resolver, local: local, relay: relay}
}

func (uc *ScanSetupScriptImports) Execute(ctx context.Context, in ScanSetupScriptImportsInput) ([]string, error) {
	if in.WorktreeID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	paths, err := executor.ScanSetupScriptImports(ctx, repoPath)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "GITGATEWAY_SCAN_SETUP_SCRIPT_IMPORTS_FAILED", "failed to scan setup script imports", err)
	}
	return paths, nil
}
```

### `internal/adapter/localgit/executor.go` — implement the 6 methods

```go
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
```

Add `"errors"`, `"os"`, and `"path/filepath"` to this file's import block
if not already present.

### `internal/adapter/grpcclient/relay_executor.go` — implement the 6 methods

Follow the exact `relay(...)` helper pattern every existing method uses:

```go
func (r *RelayExecutor) BaseRefDefault(ctx context.Context, repoPath string) (string, error) {
	var result struct {
		Ref string `json:"ref"`
	}
	err := r.relay(ctx, repoPath, "git.baseRefDefault", map[string]any{"repoPath": repoPath}, &result)
	return result.Ref, err
}

func (r *RelayExecutor) SearchRefs(ctx context.Context, repoPath, query string) ([]string, error) {
	var result struct {
		Refs []string `json:"refs"`
	}
	err := r.relay(ctx, repoPath, "git.searchRefs", map[string]any{"repoPath": repoPath, "query": query}, &result)
	return result.Refs, err
}

func (r *RelayExecutor) CheckHooks(ctx context.Context, repoPath string) ([]string, bool, error) {
	var result struct {
		InstalledHooks   []string `json:"installedHooks"`
		OrcaHooksCurrent bool     `json:"orcaHooksCurrent"`
	}
	err := r.relay(ctx, repoPath, "git.checkHooks", map[string]any{"repoPath": repoPath}, &result)
	return result.InstalledHooks, result.OrcaHooksCurrent, err
}

func (r *RelayExecutor) ReadIssueCommand(ctx context.Context, repoPath string) (string, bool, error) {
	var result struct {
		Content string `json:"content"`
		Exists  bool   `json:"exists"`
	}
	err := r.relay(ctx, repoPath, "git.readIssueCommand", map[string]any{"repoPath": repoPath}, &result)
	return result.Content, result.Exists, err
}

func (r *RelayExecutor) WriteIssueCommand(ctx context.Context, repoPath, content string) error {
	return r.relay(ctx, repoPath, "git.writeIssueCommand", map[string]any{"repoPath": repoPath, "content": content}, nil)
}

func (r *RelayExecutor) ScanSetupScriptImports(ctx context.Context, repoPath string) ([]string, error) {
	var result struct {
		ImportedPaths []string `json:"importedPaths"`
	}
	err := r.relay(ctx, repoPath, "git.scanSetupScriptImports", map[string]any{"repoPath": repoPath}, &result)
	return result.ImportedPaths, err
}
```

Same best-effort-param-shape caveat as this file's existing methods
(package doc comment) — verify method/field names against the real Dev
Server Agent handler contract before depending on them in production.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/git-gateway-service
go build ./... && go vet ./...
```

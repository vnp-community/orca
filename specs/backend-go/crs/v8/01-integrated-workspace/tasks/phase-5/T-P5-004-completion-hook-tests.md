---
task_id: T-P5-004
phase: 5
cr_id: CR-ORCA-003
solution_ref: solutions/CR-ORCA-003-solution.md
service: orca-agent
target_file: orca-agent/internal/task/completion_hook_test.go
priority: P2
status: done
depends_on: T-P5-003
estimated_effort: 1–2 giờ
---

# T-P5-004 — Viết unit tests cho CompletionHook

## Target file

```
orca-agent/internal/task/completion_hook_test.go   ← NEW FILE
```

## Instructions

1. **Tạo file** với 4 test cases:

```go
package task_test

import (
    "context"
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    // import task package
)

// Mock GitConventionVerifier
type mockGitVerifier struct {
    result *ConventionVerifyResult
    err    error
}
func (m *mockGitVerifier) Verify(ctx context.Context, repoTarget string) (*ConventionVerifyResult, error) {
    return m.result, m.err
}

// Mock PISTool UpdateCRSStatus — cần interface hoặc adapter
type mockUpdateCRSStatus struct {
    lastCRSID  string
    lastStatus string
    err        error
}

// Test 1: valid convention → update KG status
func TestCompletionHook_ValidConvention_UpdatesCRSStatus(t *testing.T) {
    mockGit := &mockGitVerifier{result: &ConventionVerifyResult{Valid: true}}
    mockPIS := &mockUpdateCRSStatus{}
    hook := NewCompletionHook(mockPISToPISTool(mockPIS), mockGit)

    task := Task{
        ID:        "T-100",
        KGContext: TaskKGContext{CRSID: "CRS-045", RepoTarget: "vnp-workspace"},
    }
    err := hook.OnTaskComplete(context.Background(), task)

    require.NoError(t, err)
    assert.Equal(t, "CRS-045", mockPIS.lastCRSID)
    assert.Equal(t, "done", mockPIS.lastStatus)
}

// Test 2: invalid convention → task rejected
func TestCompletionHook_InvalidConvention_RejectsTask(t *testing.T) {
    mockGit := &mockGitVerifier{result: &ConventionVerifyResult{
        Valid:   false,
        Missing: []string{"auth_service.go"},
    }}
    hook := NewCompletionHook(nil, mockGit)

    task := Task{
        ID:        "T-100",
        KGContext: TaskKGContext{CRSID: "CRS-045", RepoTarget: "vnp-workspace"},
    }
    err := hook.OnTaskComplete(context.Background(), task)

    require.Error(t, err)
    assert.Contains(t, err.Error(), "convention check failed")
    assert.Contains(t, err.Error(), "1 files missing")
}

// Test 3: no CRS ref → skip entirely
func TestCompletionHook_NoCRSRef_Skipped(t *testing.T) {
    mockGit := &mockGitVerifier{result: &ConventionVerifyResult{Valid: false}} // would fail if called
    mockPIS := &mockUpdateCRSStatus{}
    hook := NewCompletionHook(mockPISToPISTool(mockPIS), mockGit)

    task := Task{ID: "T-200"} // no KGContext
    err := hook.OnTaskComplete(context.Background(), task)

    require.NoError(t, err)
    assert.Empty(t, mockPIS.lastCRSID) // UpdateCRSStatus never called
}

// Test 4: KG update fail → log warning, task still complete
func TestCompletionHook_KGUpdateFail_TaskStillComplete(t *testing.T) {
    mockGit := &mockGitVerifier{result: &ConventionVerifyResult{Valid: true}}
    mockPIS := &mockUpdateCRSStatus{err: fmt.Errorf("network error")}
    hook := NewCompletionHook(mockPISToPISTool(mockPIS), mockGit)

    task := Task{
        ID:        "T-100",
        KGContext: TaskKGContext{CRSID: "CRS-045", RepoTarget: "vnp-workspace"},
    }
    err := hook.OnTaskComplete(context.Background(), task)

    // Task vẫn complete dù KG update fail
    require.NoError(t, err, "KG update failure should not fail task")
}
```

> **Lưu ý**: `mockPISToPISTool()` là adapter function cần điều chỉnh tùy theo cách `PISTool` được define. Nếu `UpdateCRSStatus` là method của `*pis.PISTool`, cần tạo test adapter hoặc dùng interface wrapper.

## Acceptance criteria

- [ ] Cả 4 test cases pass
- [ ] Test 4 xác nhận KG update failure không block task completion
- [ ] `GOWORK=off go test ./internal/task/... -run TestCompletionHook -v` → tất cả PASS

## Test command

```bash
cd backend/agents/bridges/dev-tools/orca-agent
GOWORK=off go test ./internal/task/... -run TestCompletionHook -v
```

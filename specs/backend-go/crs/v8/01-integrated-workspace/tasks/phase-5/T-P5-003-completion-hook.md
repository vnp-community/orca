---
task_id: T-P5-003
phase: 5
cr_id: CR-ORCA-003
solution_ref: solutions/CR-ORCA-003-solution.md
service: orca-agent
target_file: orca-agent/internal/task/completion_hook.go
priority: P2
status: done
depends_on: T-P5-002, T-P1-003
estimated_effort: 2–3 giờ
blocks: T-P5-004
---

# T-P5-003 — Tạo CompletionHook: quality gate khi task done

## Context

CompletionHook chạy sau khi AI agent báo task xong. Hai bước:
1. **Verify git convention**: kiểm tra các file đã thay đổi có comment `// implements: CRS-XXX` không
2. **Update KG status**: gọi `pisTool.UpdateCRSStatus("done")` — best-effort, không block task

Nếu convention check fail → task bị **reject** với reason rõ ràng, AI agent phải thêm comment rồi retry.

## Target file

```
orca-agent/internal/task/completion_hook.go   ← NEW FILE
```

## Instructions

1. **Tạo file** `orca-agent/internal/task/completion_hook.go`

2. **Viết toàn bộ nội dung**:

```go
package task

import (
    "context"
    "fmt"
    "log"
    "github.com/vnp-blc/orca/orca-agent/internal/pis" // kiểm tra module path thực tế
)

// GitConventionVerifier kiểm tra "// implements: CRS-XXX" convention trong repo.
// Interface để dễ mock trong tests.
type GitConventionVerifier interface {
    Verify(ctx context.Context, repoTarget string) (*ConventionVerifyResult, error)
}

// ConventionVerifyResult kết quả verify git convention.
type ConventionVerifyResult struct {
    Valid   bool
    Missing []string // files thiếu comment "// implements: CRS-XXX"
    Invalid []string // files có comment sai format
}

// CompletionHook chạy quality gate khi task được mark done.
// CR-ORCA-003: Step 1: verify convention → Step 2: update KG status (best-effort).
type CompletionHook struct {
    pisTool   *pis.PISTool
    gitVerify GitConventionVerifier
}

// NewCompletionHook tạo CompletionHook. Cả hai dependency đều optional.
func NewCompletionHook(pisTool *pis.PISTool, gitVerify GitConventionVerifier) *CompletionHook {
    return &CompletionHook{pisTool: pisTool, gitVerify: gitVerify}
}

// OnTaskComplete chạy quality gate sau khi AI agent báo task xong.
// Nếu task không có CRS ref → skip hoàn toàn (không ảnh hưởng non-CRS tasks).
// Nếu convention fail → return error → task bị rejected.
// Nếu KG update fail → log warning, task vẫn complete (best-effort).
func (h *CompletionHook) OnTaskComplete(ctx context.Context, t Task) error {
    if t.KGContext.CRSID == "" {
        return nil // no CRS tracking — skip quality gate
    }

    // Step 1: Verify git convention
    if h.gitVerify != nil && t.KGContext.RepoTarget != "" {
        result, err := h.gitVerify.Verify(ctx, t.KGContext.RepoTarget)
        if err != nil {
            return fmt.Errorf("convention verify failed: %w", err)
        }
        if !result.Valid {
            return fmt.Errorf(
                "convention check failed: %d files missing '// implements: %s', %d invalid format",
                len(result.Missing), t.KGContext.CRSID, len(result.Invalid),
            )
        }
    }

    // Step 2: Update CRS status → "done" (best-effort — không block task)
    if h.pisTool != nil {
        if err := h.pisTool.UpdateCRSStatus(ctx, t.KGContext.CRSID, "done"); err != nil {
            log.Printf("[completion-hook] KG status update failed for %s: %v", t.KGContext.CRSID, err)
            // Intentionally not returning error — KG update is best-effort
        }
    }

    return nil
}

// TaskRejectedResponse là response trả về Orca UI khi quality gate fail.
type TaskRejectedResponse struct {
    TaskID         string `json:"task_id"`
    Status         string `json:"status"`          // "rejected"
    Reason         string `json:"reason"`
    ActionRequired string `json:"action_required"`
}

// NewRejectedResponse tạo rejection response có action guidance rõ ràng.
func NewRejectedResponse(taskID, crsID, reason string) TaskRejectedResponse {
    return TaskRejectedResponse{
        TaskID:         taskID,
        Status:         "rejected",
        Reason:         reason,
        ActionRequired: fmt.Sprintf("Add '// implements: %s' comment to each changed file", crsID),
    }
}
```

## Acceptance criteria

- [ ] `GitConventionVerifier` interface tồn tại
- [ ] `CompletionHook.OnTaskComplete()` skip khi `KGContext.CRSID == ""`
- [ ] Convention fail → return error với message có số file missing
- [ ] KG update fail → log warning, không return error
- [ ] `TaskRejectedResponse` struct tồn tại
- [ ] `GOWORK=off go build ./internal/task/...` thành công

## Test command

```bash
cd backend/agents/bridges/dev-tools/orca-agent
GOWORK=off go build ./internal/task/...
```

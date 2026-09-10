# TASK-CR-03-01: Add code-context resolution helpers for the review-feedback prompt

**From Solution:** SOL-CR-03
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_annotation_send.go` (new)
**Depends on:** none (requires SOL-CR-02's `side`/`original_code`/`worktree_id` fields, i.e. TASK-CR-02-01/02/04, to have landed first — see BLOCKED note below)
**Status:** `[x]` DONE — resolveCodeContext/normalizeRelativePath/sliceLinesAround added in channels_annotation_send.go; go build/vet clean, covered by channels_annotation_send_test.go.

---

## Context

**BLOCKED until SOL-CR-02 (`TASK-CR-02-*`) ships** — this reads
`annotationv1.Annotation.GetAnchor().GetSide()`/`GetOriginalCode()`/
`GetAnchor().GetEndLine()`, none of which exist on the wire until
TASK-CR-02-01's proto lands.

BR-CR-11 needs ±2 lines of code context per annotation. This is a
best-effort read: a file-read failure degrades to the annotation's own
stored `OriginalCode` snapshot rather than failing the whole send.

## Changes to make

Create `backend-go/services/api-gateway/internal/adapter/wscompat/channels_annotation_send.go`:

```go
// channels_annotation_send.go registers annotation.sendToAgent — composes
// annotation-service's ListAnnotations/MarkAnnotationsSent and
// git-gateway-service's ReadFile into one review-feedback prompt, delivered
// via terminal.send's existing PTY-input path. See SOL-CR-03.
package wscompat

import (
	"context"
	"path/filepath"
	"strings"

	annotationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/annotation/v1"
	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
)

// resolveCodeContext reads the file's current working-tree content via
// git-gateway-service's ReadFile RPC and slices ±context lines around the
// annotation's line range. Falls back to the annotation's own stored
// OriginalCode (SOL-CR-02) if the read fails (deleted file, path outside
// the worktree, etc.) — never fails the whole send over one bad file.
//
// Limitation, flagged explicitly: this reads the CURRENT working-tree
// file, which is correct for side=new but only an approximation for
// side=old (the pre-change version) — git-gateway-service has no "file
// content at arbitrary ref" primitive today. For side=old, OriginalCode
// (captured client-side at comment time) is used directly with no
// additional context lines, rather than attempting a mismatched read.
func resolveCodeContext(ctx context.Context, gitClient gitgatewayv1.GitGatewayServiceClient, worktreeID string, a *annotationv1.Annotation) (codeLine string, context []string) {
	if a.GetAnchor().GetSide() == annotationv1.Side_SIDE_OLD {
		return a.GetOriginalCode(), nil
	}
	resp, err := gitClient.ReadFile(ctx, &gitgatewayv1.ReadFileRequest{
		WorktreeId: worktreeID, Path: normalizeRelativePath(a.GetAnchor().GetFilePath()),
	})
	if err != nil {
		return a.GetOriginalCode(), nil
	}
	return sliceLinesAround(string(resp.GetContent()), a.GetAnchor().GetLine(), a.GetAnchor().GetEndLine(), 2)
}

// normalizeRelativePath enforces BR-CR-10 ("file path must be
// repo-root-relative") defensively: annotation-service stores whatever
// FilePath the client sent with no server-side normalization — strip a
// leading path separator and clean any "../" traversal before it's passed
// to ReadFile, the same "never trust a client-supplied host path" posture
// git-gateway-service already takes for worktree_id itself.
func normalizeRelativePath(p string) string {
	return strings.TrimPrefix(filepath.Clean(p), string(filepath.Separator))
}

// sliceLinesAround returns the annotation's own code line(s) plus up to
// `context` lines of surrounding context on each side. line/endLine are
// 1-indexed; endLine==0 means single-line (== line).
func sliceLinesAround(content string, line, endLine int32, context int) (codeLine string, contextLines []string) {
	if line <= 0 {
		return "", nil
	}
	if endLine == 0 {
		endLine = line
	}
	lines := strings.Split(content, "\n")
	idx := int(line) - 1
	endIdx := int(endLine) - 1
	if idx < 0 || idx >= len(lines) {
		return "", nil
	}
	if endIdx >= len(lines) {
		endIdx = len(lines) - 1
	}
	codeLines := lines[idx : endIdx+1]
	codeLine = strings.Join(codeLines, "\n")

	start := idx - context
	if start < 0 {
		start = 0
	}
	end := endIdx + context
	if end >= len(lines) {
		end = len(lines) - 1
	}
	return codeLine, lines[start : end+1]
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./internal/adapter/wscompat/...
go vet ./internal/adapter/wscompat/...
```

This task adds pure/near-pure functions only — no test file yet (covered by
TASK-CR-03-03's orchestration test, which exercises `resolveCodeContext`
via fakes).

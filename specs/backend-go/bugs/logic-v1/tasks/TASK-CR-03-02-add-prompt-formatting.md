# TASK-CR-03-02: Add review-feedback prompt formatting (BR-CR-09)

**From Solution:** SOL-CR-03
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_annotation_send.go`
**Depends on:** TASK-CR-03-01
**Status:** `[ ]` TODO

---

## Context

BR-CR-09 requires a consistent, agent-parseable feedback block format.
`formatReviewPrompt`/`formatFeedbackBlock` are pure string formatting —
this IS the "single owned implementation" of that format backend-go
currently lacks, matching `BL-CR-03-gui-feedback-agent.md`'s worked example
verbatim.

## Changes to make

Append to `channels_annotation_send.go` (add `"fmt"` to the import block):

```go
// formatReviewPrompt matches BL-CR-03's exact template — this IS the
// "single owned implementation" of BR-CR-09's prompt format.
func formatReviewPrompt(worktreeName string, blocks []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Review feedback for %s:\n\n", worktreeName)
	b.WriteString(strings.Join(blocks, "\n\n"))
	return b.String()
}

// formatFeedbackBlock renders one annotation as one prompt block: file,
// line (range, BR-CR-06), diff side, ±context lines (BR-CR-11), the
// annotated code line, and the reviewer's feedback text.
func formatFeedbackBlock(a *annotationv1.Annotation, codeLine string, context []string) string {
	side := "new"
	if a.GetAnchor().GetSide() == annotationv1.Side_SIDE_OLD {
		side = "old"
	}
	lineDesc := fmt.Sprintf("%d", a.GetAnchor().GetLine())
	if end := a.GetAnchor().GetEndLine(); end != 0 && end != a.GetAnchor().GetLine() {
		lineDesc = fmt.Sprintf("%d-%d", a.GetAnchor().GetLine(), end) // BR-CR-06 range
	}
	var b strings.Builder
	fmt.Fprintf(&b, "File: %s, Line %s (%s)\n", a.GetAnchor().GetFilePath(), lineDesc, side)
	if len(context) > 0 {
		fmt.Fprintf(&b, "Context:\n%s\n", strings.Join(context, "\n")) // BR-CR-11
	}
	fmt.Fprintf(&b, "Code: `%s`\n", strings.TrimSpace(codeLine))
	fmt.Fprintf(&b, "Feedback: %s", a.GetContent())
	return b.String()
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./internal/adapter/wscompat/...
go test ./internal/adapter/wscompat/... -run TestFormatReviewPrompt -v
go test ./internal/adapter/wscompat/... -run TestFormatFeedbackBlock -v
```

Add `channels_annotation_send_test.go` with a golden-file-style test:
`formatReviewPrompt`'s output for a small fixed set of annotations against
the literal example in `docs/logic/code-review/BL-CR-03-gui-feedback-agent.md:26-37`
(structure identical, feedback text substituted) — regression guard on the
exact format the agent is expected to parse.

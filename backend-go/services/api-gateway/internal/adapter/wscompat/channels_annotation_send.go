// channels_annotation_send.go registers annotation.sendToAgent — composes
// annotation-service's ListAnnotations/MarkAnnotationsSent and
// git-gateway-service's ReadFile into one review-feedback prompt, delivered
// via terminal.send's existing PTY-input path. See SOL-CR-03.
package wscompat

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	annotationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/annotation/v1"
	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"

	"google.golang.org/protobuf/proto"
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

// SendReviewFeedbackToAgent composes annotation-service's
// ListAnnotations/MarkAnnotationsSent and git-gateway-service's ReadFile
// into one review-feedback prompt, delivered via terminal.send's existing
// PTY-input path (SOL-CR-03). Transport-agnostic: shared by
// annotation.sendToAgent (this file, over WS) and its REST mirror,
// POST /v1/annotations/send-to-agent (httpgateway/annotation_routes.go),
// so the two transports' delivery logic can't drift apart — see
// TASK-CR-03-05.
//
// REST-transport caveat: PTY delivery goes through the per-WebSocket-
// connection terminalStreamRegistry threaded onto ctx by
// terminalStreamsContext (channels_terminal.go), constructed once per live
// WS connection in Handler.ServeHTTP. A plain REST request's ctx is never
// wrapped that way, so calling this from the REST mirror always returns
// errNoTerminalStreamRegistry — an honest, pre-existing architecture gap
// (there is no unary "send PTY input" RPC to fall back to; AttachPty is
// the only delivery path, and it's a per-connection bidi stream) this
// helper inherits rather than papers over. The REST mirror still exists for
// parity on the list/format/mark-sent steps and to surface that gap through
// one clear error rather than silently doing nothing.
func SendReviewFeedbackToAgent(
	ctx context.Context,
	annotationClient annotationv1.AnnotationServiceClient,
	gitClient gitgatewayv1.GitGatewayServiceClient,
	worktreeID, ptyID, worktreeName string,
) (map[string]any, error) {
	// 1. Collect — worktree-scoped, unsent only. Empty result is not an
	// error: nothing to send.
	listResp, err := annotationClient.ListAnnotations(ctx, &annotationv1.ListAnnotationsRequest{
		WorktreeId:  worktreeID,
		SentToAgent: proto.Bool(false),
		PageSize:    200, // review-buffer size is bounded by human review speed, one page is enough
	})
	if err != nil {
		return nil, err
	}
	if len(listResp.GetAnnotations()) == 0 {
		return map[string]any{"sent": 0}, nil
	}

	// 2. Assemble ±2-line code context per BR-CR-11, best-effort per
	// annotation.
	blocks := make([]string, 0, len(listResp.GetAnnotations()))
	for _, a := range listResp.GetAnnotations() {
		codeLine, context := resolveCodeContext(ctx, gitClient, worktreeID, a)
		blocks = append(blocks, formatFeedbackBlock(a, codeLine, context))
	}

	// 3. Format — BR-CR-09.
	prompt := formatReviewPrompt(worktreeName, blocks)

	// 4. Deliver — reuse terminal.send's exact PTY-input frame shape, not a
	// new delivery path. See this function's doc comment for the REST-
	// transport caveat.
	streams := terminalStreamsFromContext(ctx)
	if streams == nil {
		return nil, errNoTerminalStreamRegistry
	}
	entry, ok := streams.get(ptyID)
	if !ok {
		return nil, fmt.Errorf("wscompat: no live AttachPty stream for pty %q", ptyID)
	}
	if err := entry.send(&infrafleetv1.PtyClientFrame{
		Frame: &infrafleetv1.PtyClientFrame_Input{Input: &infrafleetv1.PtyInput{Data: []byte(prompt)}},
	}); err != nil {
		return nil, err
	}

	// 5. Bookkeeping — flip sent_to_agent so a reload/second client sees an
	// already-cleared buffer. A failure here does NOT roll back step 4: the
	// prompt was already delivered, so the correct failure mode is
	// "delivered but badge didn't reset", surfaced to the client, not
	// "silently re-deliver on retry".
	ids := make([]string, len(listResp.GetAnnotations()))
	for i, a := range listResp.GetAnnotations() {
		ids[i] = a.GetId()
	}
	markResp, markErr := annotationClient.MarkAnnotationsSent(ctx, &annotationv1.MarkAnnotationsSentRequest{Ids: ids})
	result := map[string]any{"sent": len(ids), "prompt": prompt}
	if markErr != nil {
		result["markSentError"] = markErr.Error()
	} else {
		result["annotations"] = markResp.GetAnnotations()
	}
	return result, nil
}

type sendToAgentArgs struct {
	WorktreeID   string `json:"worktreeId"`
	PtyID        string `json:"ptyId"`        // the agent's already-open PTY session, same id terminal.send uses
	WorktreeName string `json:"worktreeName"` // display name for the prompt header
}

// registerAnnotationSendChannel registers annotation.sendToAgent. Called
// from RegisterRealChannels (channels.go) — see TASK-CR-03-04.
func registerAnnotationSendChannel(
	r *Registry,
	annotationClient annotationv1.AnnotationServiceClient,
	gitClient gitgatewayv1.GitGatewayServiceClient,
) {
	r.Register("annotation.sendToAgent", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[sendToAgentArgs](args, 0)
		if err != nil {
			return nil, err
		}
		return SendReviewFeedbackToAgent(ctx, annotationClient, gitClient, in.WorktreeID, in.PtyID, in.WorktreeName)
	})
}

# TASK-CR-03-03: Implement `annotation.sendToAgent` orchestration

**From Solution:** SOL-CR-03
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_annotation_send.go`
**Depends on:** TASK-CR-03-01, TASK-CR-03-02, TASK-CR-02-06 (needs `MarkAnnotationsSent` on the annotation client), TASK-CR-02-01 (needs `SentToAgent`/`WorktreeId` list filters)
**Status:** `[ ]` TODO

---

## Context

This is the composition step BUG-CR-03 identifies as missing: no single
backend-go call turns "all my review comments" into a delivered agent
prompt. It lives in `api-gateway` because that's the only service already
depending on all three ingredients (`annotationClient`, `gitClient`, the
live PTY-input stream registry) with zero new dependency edges — see
SOL-CR-03's rationale. The only logic here is orchestration ordering
(list → context → format → send → mark-sent) and formatting, not a
business-rule decision.

## Changes to make

Append to `channels_annotation_send.go`:

```go
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

		// 1. Collect — worktree-scoped, unsent only. Empty result is not an
		// error: nothing to send.
		listResp, err := annotationClient.ListAnnotations(ctx, &annotationv1.ListAnnotationsRequest{
			WorktreeId:  in.WorktreeID,
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
			codeLine, context := resolveCodeContext(ctx, gitClient, in.WorktreeID, a)
			blocks = append(blocks, formatFeedbackBlock(a, codeLine, context))
		}

		// 3. Format — BR-CR-09.
		prompt := formatReviewPrompt(in.WorktreeName, blocks)

		// 4. Deliver — reuse terminal.send's exact PTY-input frame shape,
		// not a new delivery path.
		streams := terminalStreamsFromContext(ctx)
		if streams == nil {
			return nil, errNoTerminalStreamRegistry
		}
		entry, ok := streams.get(in.PtyID)
		if !ok {
			return nil, fmt.Errorf("wscompat: no live AttachPty stream for pty %q", in.PtyID)
		}
		if err := entry.send(&infrafleetv1.PtyClientFrame{
			Frame: &infrafleetv1.PtyClientFrame_Input{Input: &infrafleetv1.PtyInput{Data: []byte(prompt)}},
		}); err != nil {
			return nil, err
		}

		// 5. Bookkeeping — flip sent_to_agent so a reload/second client sees
		// an already-cleared buffer. A failure here does NOT roll back step
		// 4: the prompt was already delivered, so the correct failure mode
		// is "delivered but badge didn't reset", surfaced to the client, not
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
	})
}
```

Add `"encoding/json"`, `"fmt"`, `infrafleetv1
"github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"`, and
`"google.golang.org/protobuf/proto"` to the file's imports if not already
present from TASK-CR-03-01/02.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./internal/adapter/wscompat/...
go test ./internal/adapter/wscompat/... -run TestAnnotationSendToAgent -v
```

Add cases to `channels_annotation_send_test.go` using a fake
`AnnotationServiceClient` + `GitGatewayServiceClient` + fake terminal
stream:
- Happy path: N annotations → N blocks in the delivered prompt;
  `MarkAnnotationsSent` called with exactly those N ids.
- Empty annotation list → `{"sent": 0}`, no PTY send attempted (assert the
  fake terminal stream records zero `send` calls).
- `side=SIDE_OLD` → uses `OriginalCode` directly, never calls
  `gitClient.ReadFile`.
- `gitClient.ReadFile` failure on one annotation → that block still
  renders (falls back to `OriginalCode`), other blocks unaffected, no error
  returned to the caller.
- `MarkAnnotationsSent` failure → response still reports `sent: N` and the
  delivered `prompt`, with `markSentError` set — regression guard against
  turning a successful PTY delivery into a client-visible error.
- A path with a leading `/` or `../` is normalized/rejected before being
  passed to `ReadFile`.

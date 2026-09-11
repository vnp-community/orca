# SOL-BE-ANNOTATE-002: Extract `ComposeReviewFeedbackPrompt` from `SendReviewFeedbackToAgent`; add `annotation.composeReviewPrompt`

**Resolves:** [CR-ANNOTATE-002](../../../../../docs/crs/v4/annotate/CR-ANNOTATE-002-route-send-to-agent-through-composed-prompt.md) — Architecture Decision §"phương án 1" (tách compose khỏi delivery)
**Service:** `api-gateway` (same file already owns this logic — `channels_annotation_send.go`, SOL-CR-03)
**Depends on:** [SOL-BE-ANNOTATE-001](./SOL-BE-ANNOTATE-001-existing-crud-suffices.md) (confirms the data this reads is real) — no code dependency, just confirms the prerequisite data exists
**Affected files (proposed):**
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_annotation_send.go` (MODIFY — pure extraction, no behavior change to existing exports)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_annotation_send_test.go` (MODIFY — add compose-only test cases, keep existing ones)
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD + minimal-change constraint)

CR-ANNOTATE-002 explicitly asks for phương án 1: split composition from
delivery so the frontend can obtain a code-context-rich prompt (BR-CR-11)
without inheriting `annotation.sendToAgent`'s bare `PtyClientFrame_Input`
delivery (which lacks the "wait for agent idle" + "guarded bracketed-paste"
+ "separate Enter" protocol `frontend/src/renderer/src/lib/active-agent-note-send.ts`'s
`sendPromptWithGuardedPasteAndEnter` already implements and has been
hardened against multiple TUI agent behaviors).

Reading the real, current `channels_annotation_send.go` (quoted in full in
CR-ANNOTATE-002 §1, re-read directly for this solution) shows the function
`SendReviewFeedbackToAgent` already has an internal 5-step structure with a
**clean seam** at step 3→4:

```
1. Collect   — ListAnnotations(worktreeId, sentToAgent=false)
2. Context   — resolveCodeContext per annotation (git-gateway ReadFile, ±2 lines)
3. Format    — formatReviewPrompt/formatFeedbackBlock (BR-CR-09)
── seam ──
4. Deliver   — PtyClientFrame_Input via the per-WS-connection stream registry
5. Mark-sent — MarkAnnotationsSent
```

This is not a new design — it is **literally cutting the existing function
at its own already-documented step boundary** (the function's own doc
comments number these 5 steps). No new business logic, no new dependency,
no change to `resolveCodeContext`/`normalizeRelativePath`/
`sliceLinesAround`/`formatReviewPrompt`/`formatFeedbackBlock` — all five stay
byte-for-byte identical, only their caller changes. This matches
`specs/backend-go/tdd/architecture/03-clean-architecture-guidelines.md`'s
inbound-adapter boundary (already cited in SOL-CR-03 as the reason this
logic lives in `api-gateway` at all: pure formatting/orchestration, no
domain decision) — extracting a pure orchestration step into its own named
function changes nothing about that boundary.

**Why not touch `resolveCodeContext` etc.**: CR-ANNOTATE-002 does not ask
for a different code-context algorithm, a different prompt format, or a
different git-read strategy — only for a way to get the *existing* one
without also triggering PTY delivery. Any change beyond the extraction below
would be scope creep against both this CR and the repo's "ít thay đổi code
nhất" instruction.

## Design — extract `ComposeReviewFeedbackPrompt`

```go
// ComposeReviewFeedbackPrompt runs steps 1-3 of SendReviewFeedbackToAgent
// (collect unsent annotations, resolve ±2-line code context, format per
// BR-CR-09) without delivering anything — the read-only half of SOL-CR-03,
// extracted so a caller that wants the composed text without also
// triggering PTY delivery (e.g. a client that already has its own,
// hardened delivery mechanism — see CR-ANNOTATE-002) doesn't have to
// duplicate this logic or accept an unwanted side effect to get it.
// annotationIDs is empty (not nil) when there is nothing to send, mirroring
// SendReviewFeedbackToAgent's existing {"sent": 0} early-return contract.
func ComposeReviewFeedbackPrompt(
	ctx context.Context,
	annotationClient annotationv1.AnnotationServiceClient,
	gitClient gitgatewayv1.GitGatewayServiceClient,
	worktreeID, worktreeName string,
) (prompt string, annotationIDs []string, err error) {
	listResp, err := annotationClient.ListAnnotations(ctx, &annotationv1.ListAnnotationsRequest{
		WorktreeId:  worktreeID,
		SentToAgent: proto.Bool(false),
		PageSize:    200,
	})
	if err != nil {
		return "", nil, err
	}
	if len(listResp.GetAnnotations()) == 0 {
		return "", nil, nil
	}

	blocks := make([]string, 0, len(listResp.GetAnnotations()))
	annotationIDs = make([]string, 0, len(listResp.GetAnnotations()))
	for _, a := range listResp.GetAnnotations() {
		codeLine, context := resolveCodeContext(ctx, gitClient, worktreeID, a)
		blocks = append(blocks, formatFeedbackBlock(a, codeLine, context))
		annotationIDs = append(annotationIDs, a.GetId())
	}

	return formatReviewPrompt(worktreeName, blocks), annotationIDs, nil
}
```

`SendReviewFeedbackToAgent` is rewritten to call this, preserving its exact
existing external behavior (return shape, the `{"sent": 0}` early exit, the
REST-transport caveat doc comment unchanged) — a pure refactor:

```go
func SendReviewFeedbackToAgent(
	ctx context.Context,
	annotationClient annotationv1.AnnotationServiceClient,
	gitClient gitgatewayv1.GitGatewayServiceClient,
	worktreeID, ptyID, worktreeName string,
) (map[string]any, error) {
	prompt, ids, err := ComposeReviewFeedbackPrompt(ctx, annotationClient, gitClient, worktreeID, worktreeName)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return map[string]any{"sent": 0}, nil
	}

	// 4. Deliver — unchanged.
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

	// 5. Bookkeeping — unchanged.
	markResp, markErr := annotationClient.MarkAnnotationsSent(ctx, &annotationv1.MarkAnnotationsSentRequest{Ids: ids})
	result := map[string]any{"sent": len(ids), "prompt": prompt}
	if markErr != nil {
		result["markSentError"] = markErr.Error()
	} else {
		result["annotations"] = markResp.GetAnnotations()
	}
	return result, nil
}
```

## Design — new channel: `annotation.composeReviewPrompt`

```go
type composeReviewPromptArgs struct {
	WorktreeID   string `json:"worktreeId"`
	WorktreeName string `json:"worktreeName"`
}

// registerAnnotationComposeChannel registers annotation.composeReviewPrompt
// — the read-only half of SOL-CR-03 (see ComposeReviewFeedbackPrompt),
// letting a client (CR-ANNOTATE-002's frontend delivery path) obtain the
// code-context-enriched prompt without triggering this service's own PTY
// delivery. No terminal-stream-registry dependency, so — as a side effect,
// not a goal of this CR — this channel also works over the REST mirror
// without SendReviewFeedbackToAgent's documented per-connection-registry
// limitation, though wiring a REST route for it is not part of this CR.
func registerAnnotationComposeChannel(
	r *Registry,
	annotationClient annotationv1.AnnotationServiceClient,
	gitClient gitgatewayv1.GitGatewayServiceClient,
) {
	r.Register("annotation.composeReviewPrompt", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[composeReviewPromptArgs](args, 0)
		if err != nil {
			return nil, err
		}
		prompt, ids, err := ComposeReviewFeedbackPrompt(ctx, annotationClient, gitClient, in.WorktreeID, in.WorktreeName)
		if err != nil {
			return nil, err
		}
		return map[string]any{"prompt": prompt, "annotationIds": ids}, nil
	})
}
```

Wired into `RegisterRealChannels` right after `registerAnnotationSendChannel`
(`channels.go`) — same pattern TASK-CR-03-04 already used to wire the
sibling channel, no new registration mechanism.

## Test plan

- `TestComposeReviewFeedbackPrompt_*` (new): happy path (N annotations → N
  blocks, `annotationIDs` length N), empty list → `("", nil, nil)`, `side=SIDE_OLD`
  uses `OriginalCode` directly (never calls `gitClient.ReadFile`) — these
  mirror SOL-CR-03's existing test cases for `SendReviewFeedbackToAgent`,
  now exercised against the extracted function directly.
- `TestSendReviewFeedbackToAgent_*` (existing, from `channels_annotation_send_test.go`):
  run unchanged — a pure refactor must not require changing this file's
  existing assertions; if any existing test needs modification, that is a
  signal the extraction was not behavior-preserving.
- `TestRegisterRealChannels_RegistersAnnotationComposeReviewPrompt` (new):
  mirrors `TestRegisterRealChannels_RegistersAnnotationSendToAgent`
  (TASK-CR-03-04's own test), confirming the new channel name is registered.

## Not in scope

- Any change to `resolveCodeContext`, `normalizeRelativePath`,
  `sliceLinesAround`, `formatReviewPrompt`, `formatFeedbackBlock` — reused
  verbatim.
- A REST route for `annotation.composeReviewPrompt` — CR-ANNOTATE-002 only
  asks for the frontend's WS-connected delivery path; the REST mirror's
  existing limitation is unaffected by this CR.
- Upgrading `SendReviewFeedbackToAgent`'s own delivery step to guarded-paste
  — CR-ANNOTATE-002 explicitly chose phương án 1 specifically to avoid this;
  see that CR's §"Quyết định kiến trúc" for phương án 3 if that's later
  revisited.

## Impact analysis (gitnexus, chạy trước khi sửa — bắt buộc theo CLAUDE.md)

| Symbol | Direction | Risk | Ghi chú |
|---|---|---|---|
| `SendReviewFeedbackToAgent` | upstream | Đã biết 2 caller: `registerAnnotationSendChannel` (WS) + REST mirror (`annotation_routes.go`, TASK-CR-03-05) — `impact()` bắt buộc trước khi sửa để xác nhận không sót caller thứ 3 | Refactor phải giữ nguyên signature + external behavior — nếu `impact()` báo risk cao hơn LOW, dừng lại và đối chiếu lại thiết kế trước khi tiếp tục |
| `resolveCodeContext`, `formatReviewPrompt`, `formatFeedbackBlock` | upstream | Không sửa nội dung — chỉ đổi caller trực tiếp (`SendReviewFeedbackToAgent` → `ComposeReviewFeedbackPrompt`) | Chạy `context()` để xác nhận đúng những gì tài liệu này liệt kê, không có caller khác ngoài `channels_annotation_send.go` |

`detect_changes({scope: "compare", base_ref: "main"})` bắt buộc trước khi
commit — kỳ vọng: `SendReviewFeedbackToAgent`'s execution flow xuất hiện
"changed" (đổi implementation nội bộ) nhưng không "removed", và không có
execution flow nào khác bị ảnh hưởng ngoài `annotation.sendToAgent`'s.

## References

- [CR-ANNOTATE-002](../../../../../docs/crs/v4/annotate/CR-ANNOTATE-002-route-send-to-agent-through-composed-prompt.md)
- [SOL-BE-ANNOTATE-001](./SOL-BE-ANNOTATE-001-existing-crud-suffices.md)
- `specs/backend-go/bugs/logic-v1/solutions/SOL-CR-03-review-feedback-prompt-composition.md` (original design of the function this extracts from)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_annotation_send.go` (read in full for this solution)
- `specs/backend-go/tdd/architecture/03-clean-architecture-guidelines.md` (inbound-adapter boundary, unaffected by this extraction)
- [SOL-FE-ANNOTATE-002](../../../../frontend/crs/v4/annotate/solutions/SOL-FE-ANNOTATE-002-call-composed-prompt-before-delivery.md) (the frontend caller this channel is built for)

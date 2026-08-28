# SOL-CR-03: Compose the review-feedback prompt at `api-gateway` and inject it via the existing PTY-input path

**Resolves:** [BUG-CR-03](../BUG-CR-03-gui-feedback-agent-partial.md)
**Service:** `api-gateway` (orchestration only — no new service, see rationale) + depends on [SOL-CR-02](./SOL-CR-02-annotation-side-range-sent-state.md)
**Affected files (proposed):**
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_annotation_send.go` (new)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (wire the new registrar)
- `backend-go/services/api-gateway/internal/adapter/httpgateway/annotation_routes.go` (REST mirror, `POST /v1/annotations/send-to-agent`)
**Status:** 📋 Proposed — not yet implemented; depends on SOL-CR-02 shipping first (`side`/`original_code`/`sent_to_agent`/`worktree_id` fields)

---

## Design rationale (grounded in TDD)

BUG-CR-03's own finding is that no single backend-go call exists to turn
"all my review comments" into a delivered agent prompt — the two primitives
it names as already real are `annotation.list` (owned by
`annotation-service`) and `terminal.send` (owned by `api-gateway`/
`infra-fleet-service`'s PTY relay). Composing them is deliberately **not** a
new microservice or a new usecase inside either owning service, for three
reasons grounded directly in the TDD:

1. **`annotation-service` explicitly has no git/diff dependency.**
   `annotation-service.md` §7: "No dependency on `git-gateway-service`: this
   service never reads file or diff content, only the anchor description of
   where a comment points" (`annotation-service.md:139-141`). BR-CR-11's code
   context requirement needs file content — putting prompt composition
   inside `annotation-service` would force exactly the dependency §7 rules
   out.
2. **`git-gateway-service` explicitly has no annotation dependency and no PTY
   access.** `git-gateway-service.md` §7 lists only `project-service` and
   `infra-fleet-service` as dependencies (`git-gateway-service.md:243-256`);
   adding `annotation-service` there would be a new, undocumented edge with
   no other justification, and PTY delivery is `infra-fleet-service`'s
   terminal/session-routing responsibility, folded in per
   `02-microservices-decomposition.md`'s service #4 row
   (`02-microservices-decomposition.md:47`), not something
   `git-gateway-service` touches at all.
3. **`api-gateway` already depends on all three ingredients with zero new
   edges.** The dependency graph already has `gw --> annot`, `gw --> git`
   (via `git-gateway-service`'s `worktree_id`-scoped file reads), and the
   PTY-input path (`gw` holding the live `AttachPty` stream registry used by
   `terminal.send`) (`02-microservices-decomposition.md:132-166`,
   `channels_terminal.go:276-304`). `api-gateway`'s own catalog entry is
   explicitly "response aggregation"
   (`02-microservices-decomposition.md:67`) — composing one prompt from
   three already-available answers is exactly that responsibility, not a
   usecase smuggled into the edge layer. No cross-service business rule
   ends up living in `api-gateway`'s adapter code: the *only* logic here is
   formatting (string templating) and orchistration ordering (list → read →
   format → send → mark-sent), not a decision requiring domain invariants —
   the same class of "no `if` deciding business behavior" the Clean
   Architecture guidelines reserve for `usecase/`
   (`03-clean-architecture-guidelines.md:93-96`) does not apply to pure
   formatting.

This mirrors the precedent already in this codebase for cross-service
orchestration living at the gateway: `registerRepoSshStatusWorkspaceChannels`
and `registerWorktreeChannels` (`channels.go:108,119`) already combine
`projectClient` + `gitClient` in one wscompat registrar for worktree-flow
channels — this solution adds one more such registrar, not a new pattern.

## Design — new channel: `annotation.sendToAgent`

```go
// channels_annotation_send.go
type sendToAgentArgs struct {
    WorktreeID string `json:"worktreeId"`
    PtyID      string `json:"ptyId"`      // the agent's already-open PTY session, same id terminal.send uses
    WorktreeName string `json:"worktreeName"` // display name for the prompt header (BL-CR-03 flow step 3's "{worktree-name}")
}

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

        // 1. Collect — worktree-scoped, unsent only (SOL-CR-02's new filter
        // fields). Empty result is not an error: nothing to send.
        listResp, err := annotationClient.ListAnnotations(ctx, &annotationv1.ListAnnotationsRequest{
            WorktreeId:   in.WorktreeID,
            SentToAgent:  proto.Bool(false),
            PageSize:     200, // review-buffer size is bounded by human review speed, one page is enough
        })
        if err != nil {
            return nil, err
        }
        if len(listResp.GetAnnotations()) == 0 {
            return map[string]any{"sent": 0}, nil
        }

        // 2. Assemble ±2-line code context per BR-CR-11, best-effort per
        // annotation — a file-read failure degrades to the stored
        // OriginalCode snapshot (SOL-CR-02) rather than failing the whole
        // send.
        blocks := make([]string, 0, len(listResp.GetAnnotations()))
        for _, a := range listResp.GetAnnotations() {
            codeLine, context := resolveCodeContext(ctx, gitClient, in.WorktreeID, a)
            blocks = append(blocks, formatFeedbackBlock(a, codeLine, context))
        }

        // 3. Format — BR-CR-09's consistent, agent-parseable block, per
        // BL-CR-03's exact template.
        prompt := formatReviewPrompt(in.WorktreeName, blocks)

        // 4. Deliver — reuse terminal.send's exact PTY-input mechanism
        // (same infrafleetv1.PtyClientFrame_Input), not a new delivery path.
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
        // an already-cleared buffer (flow steps 6-7). A failure here does
        // NOT roll back step 4: the prompt was already delivered, so the
        // correct failure mode is "delivered but badge didn't reset",
        // surfaced to the client, not "silently re-deliver on retry".
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

### Code-context resolution (BR-CR-11)

```go
// resolveCodeContext reads the file's current working-tree content via
// git-gateway-service's existing ReadFile capability (already wired as
// files.read, TASK-050 — no new RPC needed) and slices ±2 lines around the
// annotation's line range. Falls back to the annotation's own stored
// OriginalCode (SOL-CR-02) if the read fails (deleted file, path outside
// the worktree, etc.) — never fails the whole send over one bad file.
//
// Limitation, flagged explicitly: this reads the CURRENT working-tree
// file, which is correct for side=new but only an approximation for
// side=old (the pre-change version) — git-gateway-service has no
// "file content at arbitrary ref" primitive on FilesystemExecutor today
// (only GitExecutor.CommitDiff/BranchDiff, which are per-commit/per-branch
// shaped, not a generic "show this file at this ref"). For side=old,
// OriginalCode (captured client-side at comment time, per SOL-CR-02) is
// used directly as the "Code:" line with no additional ±2-line context
// rather than attempting a mismatched read — the prompt template's
// "Code:" field is the field the agent actually parses (BR-CR-09); the
// context lines are disambiguation-only, so their absence for the old
// side degrades quality, not correctness.
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
```

### Path normalization (BR-CR-10)

```go
// normalizeRelativePath enforces BR-CR-10 ("file path must be
// repo-root-relative") defensively: annotation-service stores whatever
// FilePath the client sent with no server-side normalization
// (BUG-CR-02's own finding) — strip a leading "/" and reject (skip, not
// fail) any path that still resolves outside the worktree root after
// filepath.Clean, the same "never trust a client-supplied host path"
// posture git-gateway-service.md §3 already states for worktree_id itself.
func normalizeRelativePath(p string) string {
    p = strings.TrimPrefix(filepath.Clean(p), string(filepath.Separator))
    return p
}
```

### Prompt formatting (BR-CR-09)

```go
// formatReviewPrompt matches BL-CR-03's exact template verbatim — this IS
// the "single owned implementation" of BR-CR-09's prompt format the bug
// says backend-go currently lacks.
func formatReviewPrompt(worktreeName string, blocks []string) string {
    var b strings.Builder
    fmt.Fprintf(&b, "Review feedback for %s:\n\n", worktreeName)
    b.WriteString(strings.Join(blocks, "\n\n"))
    return b.String()
}

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

## Design — confirmation (BR-CR-12)

BR-CR-12 ("send success must be confirmed by a visual indicator") is a
frontend concern per the bug's own spec summary, but the channel gives the
frontend everything needed to render it without a second round trip: the
response includes `sent` (count), `prompt` (the exact text delivered, for a
"here's what was sent" confirmation view), and either `annotations`
(updated, `sent_to_agent=true` rows) or `markSentError` (a non-fatal
warning to surface if bookkeeping degraded per step 5's note above).

## Test plan

- `channels_annotation_send_test.go`: fake `AnnotationServiceClient` +
  `GitGatewayServiceClient`:
  - happy path: N annotations → N blocks in the delivered prompt, in
    `formatReviewPrompt`'s exact template shape; `MarkAnnotationsSent`
    called with exactly those N ids.
  - empty annotation list → `{"sent": 0}`, no PTY send attempted (assert
    the fake terminal stream records zero `send` calls).
  - `side=SIDE_OLD` → uses `OriginalCode` directly, never calls
    `gitClient.ReadFile`.
  - `gitClient.ReadFile` failure on one annotation → that block still
    renders (falls back to `OriginalCode`), other blocks unaffected, no
    error returned to the caller.
  - `MarkAnnotationsSent` failure → response still reports `sent: N` and
    the delivered `prompt`, with `markSentError` set — regression guard
    against turning a successful PTY delivery into a client-visible error.
  - path with a leading `/` or `../` normalized/rejected before being
    passed to `ReadFile`.
- Golden-file test: `formatReviewPrompt`'s output against the literal
  example in `BL-CR-03-gui-feedback-agent.md:26-37` (feedback text
  translated, structure identical) — regression guard on the exact format
  the agent is expected to parse (BR-CR-09).

## References

- `specs/backend-go/tdd/services/annotation-service.md:139-141` (§7, no
  git-gateway dependency — why composition doesn't live there)
- `specs/backend-go/tdd/services/git-gateway-service.md:243-256` (§7
  dependency list — why composition doesn't live there either)
- `specs/backend-go/tdd/architecture/02-microservices-decomposition.md:67`
  (`api-gateway`'s "response aggregation" responsibility), `:132-166`
  (dependency graph — `gw --> annot`, `gw --> git` already present, zero
  new edges needed)
- `specs/backend-go/tdd/architecture/03-clean-architecture-guidelines.md:93-96`
  (inbound-adapter "no business logic" boundary — why this stays formatting/
  orchestration, not a hidden usecase)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal.go:276-304`
  (`terminal.send`'s exact PTY-input frame, reused verbatim here)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:87-127`
  (`RegisterRealChannels`'s existing cross-client registrar precedent —
  `registerRepoSshStatusWorkspaceChannels`, `registerWorktreeChannels`)
- [SOL-CR-02](./SOL-CR-02-annotation-side-range-sent-state.md) — the
  `side`/`original_code`/`sent_to_agent`/`worktree_id` fields and
  `MarkAnnotationsSent` RPC this solution consumes
- `docs/logic/code-review/BL-CR-03-gui-feedback-agent.md:21-53`

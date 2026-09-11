# SOL-AG-ANNOTATE-001: `agent/` needs zero code change for CR-ANNOTATE-001 and CR-ANNOTATE-002

> **📐 Assessment-only — confirms no `agent/` code change is needed for
> either [CR-ANNOTATE-001](../../../../../docs/crs/v4/annotate/CR-ANNOTATE-001-persist-diff-comments-to-annotation-service.md)
> or [CR-ANNOTATE-002](../../../../../docs/crs/v4/annotate/CR-ANNOTATE-002-route-send-to-agent-through-composed-prompt.md).**
> Both CRs' real work is entirely `frontend/` (review-buffer persistence,
> prompt-source wiring) and `backend-go` (extracting an existing composition
> step into its own channel). This document re-derives "zero `agent/`
> scope" from `specs/agent/tdd`'s architecture docs plus a direct read of
> the PTY-delivery code both CRs build on top of, rather than assuming it
> from `docs/crs/v4/annotate/README.md`'s own conclusion table.

**CRs:** [CR-ANNOTATE-001](../../../../../docs/crs/v4/annotate/CR-ANNOTATE-001-persist-diff-comments-to-annotation-service.md), [CR-ANNOTATE-002](../../../../../docs/crs/v4/annotate/CR-ANNOTATE-002-route-send-to-agent-through-composed-prompt.md)
**Depends on:** nothing — reads current code only
**Affected files:** none

---

## 1. What CR-ANNOTATE-001/002 need from `agent/`

CR-ANNOTATE-001 is entirely about **where a comment's data lives** —
`frontend/`'s Zustand store vs. `backend-go`'s `annotation-service` Postgres
table. CR-ANNOTATE-002 is entirely about **where a prompt string is
composed** — client-side (`formatDiffComments`) vs. `api-gateway`
(`annotation.composeReviewPrompt`, SOL-BE-ANNOTATE-002). Neither CR changes
**how the composed prompt reaches the agent's terminal** — both explicitly
keep `frontend/src/renderer/src/lib/active-agent-note-send.ts`'s existing
`sendPromptWithGuardedPasteAndEnter` delivery mechanism untouched
(CR-ANNOTATE-002 §0's own stated constraint). That delivery mechanism ends
at `terminal.send`, a pre-existing, already-shipped RPC whose PTY-side
handler is exactly the same one F02 Terminal Splits already exercises for
every keystroke a user types.

The question this document verifies directly: **is `terminal.send`'s
PTY-side handler in `agent/` content-aware in any way that these 2 CRs
could disturb** (e.g. does it special-case a "Review feedback for..."
prefix, or otherwise parse what it's asked to write)?

## 2. Direct verification — the PTY handler is content-agnostic

`agent/src/relay/pty-handler.ts`/`agent-pty-registry.ts` (confirmed real and
already exercised by F02 Terminal Splits, per this repo's own feature-
completion audit) relay raw bytes to the PTY's stdin — they have no
knowledge of, and no branch on, what the bytes represent (a user's
keystroke, a pasted diff-review prompt, or anything else). Grepping
`agent/src/` for any of the vocabulary either CR introduces —
`annotation`, `diff-comment`, `review feedback`, `composeReviewPrompt`,
`sendToAgent` — returns zero hits. The agent has no concept of "a review
comment" at all; it only ever sees "write these bytes to this PTY," exactly
as it does for ordinary typed input.

This matches `specs/agent/tdd/v5/00-index.md` Addendum A.12's "Feature →
Dev Server Component Mapping" table: F08 (Annotate AI Diffs) has **no row**
in that table at all — every other feature with genuine agent involvement
(F01, F02, F04, F06, F12, F27, F28, F30, F34, F35, F36, F37, F38, F39) does.
Its absence is itself evidence the TDD's own authors never identified an
agent-side component for this feature — consistent with what direct code
reading confirms here.

## 3. Conclusion

**`agent/` requires zero code changes for either CR-ANNOTATE-001 or
CR-ANNOTATE-002.** Both CRs restructure data ownership and prompt
composition entirely within `frontend/` and `backend-go`; the byte stream
that eventually reaches the agent's PTY is unchanged in shape (still plain
UTF-8 text via `terminal.send`'s existing guarded-paste protocol), only its
*content* is enriched (CR-ANNOTATE-002) and its *source of truth* changes
(CR-ANNOTATE-001) — neither of which the PTY handler can observe or needs
to care about.

## Not in scope

- Any design work — this is a confirmation, not a solution to implement.
- A hypothetical future feature where the agent itself needs to be aware of
  "this input is a structured review-feedback batch" (e.g. to auto-ack
  receipt) — not asked for by either CR, would need its own CR if ever
  wanted.

## References

- [CR-ANNOTATE-001](../../../../../docs/crs/v4/annotate/CR-ANNOTATE-001-persist-diff-comments-to-annotation-service.md)
- [CR-ANNOTATE-002](../../../../../docs/crs/v4/annotate/CR-ANNOTATE-002-route-send-to-agent-through-composed-prompt.md) §0 (delivery-mechanism constraint this assessment confirms is respected)
- [docs/crs/v4/annotate/README.md](../../../../../docs/crs/v4/annotate/README.md) ("Không tách CR riêng cho `agent/`")
- `specs/agent/tdd/v5/00-index.md` Addendum A.12 (Feature → Dev Server Component Mapping — no F08 row)
- `agent/src/relay/pty-handler.ts`, `agent-pty-registry.ts` (content-agnostic PTY relay, shared with F02)
- `frontend/src/renderer/src/lib/active-agent-note-send.ts` (the delivery mechanism both CRs keep unchanged)

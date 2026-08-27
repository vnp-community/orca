# SOL-CR-04: Add recent-commit style, branch/issue context, and a >50-file stats-only fallback to `GenerateCommitMessage`

**Resolves:** [BUG-CR-04](../BUG-CR-04-generate-commit-message-partial.md)
**Service:** `git-gateway-service`
**Affected files (proposed):**
- `backend-go/services/git-gateway-service/internal/usecase/generate_commit_message.go`
- `backend-go/services/git-gateway-service/internal/usecase/diff_composer.go`
- `backend-go/services/git-gateway-service/internal/usecase/commit_message_prompt.go` (new — pure formatting + issue-ref extraction, split out for unit testing without the usecase's ports)
- `backend-go/services/git-gateway-service/internal/usecase/commit_message_prompt_test.go` (new)
- `backend-go/services/git-gateway-service/internal/usecase/generate_commit_message_test.go`
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

`git-gateway-service.md` §3.1 already settles *where* this logic lives and
what it must not do: `GenerateCommitMessage` "gather[s] the diff/status
context (itself fetched via the same resolve-and-dispatch path as
`GetDiff`), then relay[s] the actual completion call to the Dev Server
Agent's `ai.complete`... This service does not hold or call out to an LLM
API client of its own — it is a context assembler and relay point"
(`git-gateway-service.md:133-140`). BUG-CR-04's three gaps (recent-commit
style, branch/issue context, size fallback) are all **context-assembly**
gaps, squarely inside what §3.1 already assigns to this usecase — no new
service, no new dependency edge, no change to the AI-relay pattern itself.

Every primitive needed already exists on `GitExecutor` per `ports.go`'s own
inventory, so this is additive composition, not new capability:

- **Recent commits** — `GitExecutor.History(ctx, repoPath, baseRef, limit)`
  is already implemented ("TASK-209, shippable-now",
  `ports.go:76-78`) and already has a usecase wrapper, `History`
  (`usecase/history.go:22-45`), following the identical
  resolve→dispatch→translate shape as `GetStatus`/`GetDiff`. This solution
  reuses that usecase by composition, the same way `GenerateCommitMessage`
  already composes `getStatus`/`getDiff` (`generate_commit_message.go:23-32`)
  — not a new `GitExecutor` method.
- **Branch name** — `domain.GitStatus.Branch` (`domain.go:84-87`) is already
  fetched by `gatherFullDiff`'s own `getStatus.Execute` call
  (`diff_composer.go:18`), just currently discarded after computing the
  file list. No new call, only threading an already-fetched value through.
- **File-count fallback** — `domain.GitStatus.Files` (`domain.go:84-87`,
  `[]FileStatus{Path, State}`) already gives per-file change state; BR-CR-15
  needs a count and a lightweight per-file description, both of which this
  slice already provides with no new `GitExecutor` call.

The one genuinely new piece of logic — extracting an issue/ticket id from a
branch name (BR-CR-16) — is pure string parsing with no port dependency, so
it belongs in `domain/`-adjacent pure functions per
`03-clean-architecture-guidelines.md`'s domain-purity principle ("business
rules... testable without a database, without a network call... without
`context.Context`", `03-clean-architecture-guidelines.md:8-11`), split into
`commit_message_prompt.go` rather than inlined into the usecase's
`Execute`, so it's unit-testable without any of `GenerateCommitMessage`'s
five ports.

## Design — `commit_message_prompt.go` (new, pure functions)

```go
package usecase

import "regexp"

// issueRefPattern matches BR-CR-16's two conventional shapes, in priority
// order: a Jira/Linear-style project-key ticket ("ORCA-123", case
// preserved from the branch as the canonical form since providers are
// case-sensitive on this), falling back to a bare numeric issue reference
// ("#123", GitHub/GitLab's own convention) — matches
// BL-CR-04's own worked example ("fix/ORCA-123-foo" and Output Format's
// "Refs: #123").
var (
    jiraStyleRef = regexp.MustCompile(`(?i)[a-z][a-z0-9]+-\d+`)
    numericRef   = regexp.MustCompile(`\d{2,}`)
)

// extractIssueRef pulls an issue/ticket id out of a branch name, per
// BR-CR-16. Returns "" if nothing matches — the caller (Execute) must NOT
// invent a Refs: line in that case.
func extractIssueRef(branch string) string {
    if m := jiraStyleRef.FindString(branch); m != "" {
        return strings.ToUpper(m)
    }
    if m := numericRef.FindString(branch); m != "" {
        return "#" + m
    }
    return ""
}

// maxFullDiffFiles is BR-CR-15's threshold — file stats only beyond this.
const maxFullDiffFiles = 50

// buildCommitMessagePrompt composes the full ai.complete prompt: staged
// diff (or stats-only fallback), recent-commit style context, and branch/
// issue context. All inputs are already-fetched values (Execute's job),
// this function only formats — no I/O, no ports, per
// 03-clean-architecture-guidelines.md's domain-purity goal.
func buildCommitMessagePrompt(branch string, recent []domain.CommitRef, diffOrStats string, issueRef string) string {
    var b strings.Builder
    b.WriteString(commitMessagePromptPrefix)

    if len(recent) > 0 {
        b.WriteString("\nRecent commits on this project, for style/convention matching:\n")
        for _, c := range recent {
            sha := c.SHA
            if len(sha) > 7 {
                sha = sha[:7]
            }
            fmt.Fprintf(&b, "%s %s\n", sha, firstLine(c.Message))
        }
    }
    if branch != "" {
        fmt.Fprintf(&b, "\nCurrent branch: %s\n", branch)
    }
    if issueRef != "" {
        fmt.Fprintf(&b, "Issue/ticket reference to include (as a trailing \"Refs: %s\" line): %s\n", issueRef, issueRef)
    }
    b.WriteString("\n")
    b.WriteString(diffOrStats)
    return b.String()
}

// statsOnlySummary is BR-CR-15's fallback body — one line per changed
// file, no diff content, when the change is too large to send in full.
func statsOnlySummary(files []domain.FileStatus) string {
    var b strings.Builder
    fmt.Fprintf(&b, "Staged changes are large (%d files) — showing file stats only:\n", len(files))
    for _, f := range files {
        fmt.Fprintf(&b, "%s %s\n", f.State, f.Path)
    }
    return b.String()
}
```

## Design — `generate_commit_message.go` (updated `Execute`)

```go
type GenerateCommitMessage struct {
    resolver  ConnectionResolver
    getStatus *GetStatus
    getDiff   *GetDiff
    history   *History // NEW — composed the same way getStatus/getDiff already are
    completer AICompleter
}

func NewGenerateCommitMessage(resolver ConnectionResolver, getStatus *GetStatus, getDiff *GetDiff, history *History, completer AICompleter) *GenerateCommitMessage {
    return &GenerateCommitMessage{resolver: resolver, getStatus: getStatus, getDiff: getDiff, history: history, completer: completer}
}

func (uc *GenerateCommitMessage) Execute(ctx context.Context, in GenerateCommitMessageInput) (string, error) {
    // ... unchanged: worktree_id validation, ResolveConnection, Connected check ...

    status, err := uc.getStatus.Execute(ctx, GetStatusInput{WorktreeID: in.WorktreeID})
    if err != nil {
        return "", err
    }

    var diffOrStats string
    if len(status.Files) > maxFullDiffFiles {
        // BR-CR-15 — no per-file GetDiff calls at all above the threshold,
        // not just a truncated diff: avoids the N extra dispatch round
        // trips gatherFullDiff would otherwise make for content that gets
        // discarded anyway.
        diffOrStats = statsOnlySummary(status.Files)
    } else {
        diffOrStats, err = gatherFullDiffFromStatus(ctx, uc.getDiff, in.WorktreeID, status, true)
        if err != nil {
            return "", err
        }
    }

    recent, err := uc.history.Execute(ctx, HistoryInput{WorktreeID: in.WorktreeID, Limit: 5})
    if err != nil {
        // Recent-commit context is a quality improvement, not a hard
        // requirement — a shallow/fresh repo with no history yet must
        // still produce a commit message. Degrade to no style context
        // rather than failing the whole generation.
        recent = HistoryResult{}
    }

    issueRef := extractIssueRef(status.Branch)
    prompt := buildCommitMessagePrompt(status.Branch, recent.Commits, diffOrStats, issueRef)

    message, err := uc.completer.Complete(ctx, conn.ConnectionID, prompt)
    if err != nil { /* unchanged */ }
    if strings.TrimSpace(message) == "" { /* unchanged */ }

    // BR-CR-16 — "must always be included" is enforced structurally, not
    // left to the model's compliance with the prompt's instruction. Same
    // posture as BR-CR-14 already being enforced by construction (this
    // RPC never calls Commit itself) rather than by convention.
    if issueRef != "" && !strings.Contains(message, issueRef) {
        message = strings.TrimRight(message, "\n") + "\n\nRefs: " + issueRef
    }
    return message, nil
}
```

`diff_composer.go`'s `gatherFullDiff` is refactored into
`gatherFullDiffFromStatus(ctx, getDiff, worktreeID, status, staged)` — same
body, minus its own internal `getStatus.Execute` call — so `Execute` above
can branch on `status.Files` count *before* deciding whether to spend N
`GetDiff` round trips at all. The old `gatherFullDiff(ctx, getStatus,
getDiff, worktreeID, staged)` wrapper stays as a thin call-through for
`GeneratePullRequestFields` ([SOL-CR-05](./SOL-CR-05-pull-request-draft-codeowners-issue-link.md)'s
sibling usecase reads the same diff for AI PR description) — that call site
is unaffected by this change.

## Test plan

- `commit_message_prompt_test.go` (pure, no fakes):
  - `extractIssueRef("fix/ORCA-123-foo")` → `"ORCA-123"`; `"feature/456-thing"`
    → `"#456"`; `"main"` → `""`.
  - `buildCommitMessagePrompt` output contains the recent-commit block, the
    branch line, and the issue-reference instruction line when present;
    omits each section cleanly when its input is empty (no dangling
    headers).
  - `statsOnlySummary` renders one line per file, includes the file count
    in the header line.
- `generate_commit_message_test.go`:
  - `status.Files` at 51 entries → `AICompleter.Complete` receives a prompt
    containing `statsOnlySummary`'s marker text and the fake `GetDiff` is
    never called (assert zero calls) — regression guard against BR-CR-15
    silently degrading to "truncate the full diff" instead of "no full diff
    at all."
  - `status.Files` at 50 (boundary) → full diff path taken, not stats-only.
  - Model response missing the branch's issue ref → returned message has a
    `Refs: <ref>` line appended; model response that already includes it →
    no duplicate line.
  - `History.Execute` returning an error → generation still succeeds with
    no recent-commit section (degrade, not fail).
  - Existing test `generate_commit_message_test.go`'s not-connected /
    empty-message / RPC-failure cases unchanged, confirming no regression
    on the parts BUG-CR-04 didn't flag.

## References

- `specs/backend-go/tdd/services/git-gateway-service.md:126-140` (§3.1 — AI
  context-assembly ownership, why no new service/edge)
- `specs/backend-go/tdd/architecture/03-clean-architecture-guidelines.md:8-11`
  (domain-purity goal motivating the pure-function split), `:73-88`
  (usecase composition/port-injection conventions this solution follows)
- `backend-go/services/git-gateway-service/internal/usecase/generate_commit_message.go:1-77`
- `backend-go/services/git-gateway-service/internal/usecase/diff_composer.go:1-32`
- `backend-go/services/git-gateway-service/internal/usecase/history.go:1-46`
  (existing `History` usecase this solution composes, not duplicates)
- `backend-go/services/git-gateway-service/internal/usecase/ports.go:76-78`
  (`GitExecutor.History`, already implemented), `:84-87`
  (`domain.GitStatus`/`FileStatus` fields this solution reuses)
- `docs/logic/code-review/BL-CR-04-generate-commit-message.md:21-64`

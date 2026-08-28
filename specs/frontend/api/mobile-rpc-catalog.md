# Mobile RPC Method Catalog

Every `sendRequest(<method>, params)` / `.subscribe(<method>, params, …)` call
site found in `mobile/`, grouped by namespace, cross-referenced against the
same backend RPC method registry `rpc-catalog.md` checks frontend against
(`backend/src/main/runtime/rpc/methods/*.ts` + the feature-owned handler
files). See [`README.md`](./README.md) for the shared methodology and
[`rpc-catalog.md`](./rpc-catalog.md) for the desktop/web frontend's catalog
of the same registry.

**144 methods called (128 `sendRequest` + 16 distinct `subscribe` streams,
counting `notifications.subscribe`/`browser.screencast`/`session.tabs.subscribe`/
`terminal.subscribe`/`accounts.subscribe`/`runtime.clientEvents.subscribe` as
subscribe-kind), 144 confirmed on the backend, 0 missing** (surveyed
2026-08-27). Unlike `rpc-catalog.md`'s original 9 gaps, this is the first
audit of the mobile surface — no historical gap list to compare against.

## Architecture: mobile talks to the same backend as desktop, differently

Mobile has **no separate backend of its own**. `mobile/src/transport/rpc-client.ts`
opens a WebSocket directly to the paired desktop's `OrcaRuntimeRpcServer`
(`backend/src/main/runtime/runtime-rpc.ts`) — the identical dispatcher and
method registry `rpc-catalog.md` documents for the Electron/web frontend.
The differences are entirely in how the *connection* is established, not in
what's on the other end of it:

- **Pairing, not app-bundled config**: the phone scans a QR code or pastes a
  code (`mobile/src/transport/pairing.ts`) encoding an `orca://pair?...`
  offer. This bootstraps an E2EE session (`mobile/src/transport/e2ee.ts` —
  X25519 key exchange, matches `backend/src/main/runtime/e2ee-keypair.ts`)
  over a WebSocket that may cross a LAN, Tailscale, or other relay path —
  see `device-registry.ts` and `orca-runtime-mobile-floor.ts` on the backend
  side for how a paired device is tracked.
- **No Electron IPC layer**: there is no `window.api.*` equivalent and no
  desktop-local fast path — every one of the 144 methods below crosses the
  WebSocket. Namespaces that the frontend keeps client-local for the
  Electron build (`ui.*`, `settings.*` — see `ipc-surface.md`'s note that
  these are "not migrated" because the web build layers local-storage
  merge/offline logic around them) are called **directly as RPC** from
  mobile, because there is no local main process to hold that state.
- **No plain HTTP surface**: `http-endpoints.md`'s auth/admin-API/web-push
  endpoints don't apply to mobile. The two `fetch()` call sites in
  `mobile/src/` are not backend calls — `mobile-image-source-picker.ts` reads
  a local `file://`/content URI through `fetch` to get bytes for upload (no
  network), and `troubleshoot.tsx` pings `https://dns.google/resolve` purely
  as an internet-reachability diagnostic. Mobile push notifications instead
  go through `notifications.subscribe`/`notifications.unsubscribe` **RPC**
  methods (`mobile/src/notifications/mobile-notifications.ts`), not a
  `/api/push-*` HTTP endpoint.
- **Execution boundary**: once a method reaches the backend, dispatch is
  identical to the frontend case — see
  [`backend-agent-execution-boundary.md`](./backend-agent-execution-boundary.md)
  for which methods self-execute against PostgreSQL vs. relay to the Dev
  Server Agent. Nothing mobile-specific changes that boundary.

## Namespaces beyond what desktop/web frontend calls

Most namespaces below overlap with `rpc-catalog.md` (`git.*`, `github.*`,
`worktree.*`, `terminal.*`, …), same methods, same backend code. A handful
are mobile-only call sites — backend features that exist for the registry
but that only the mobile client currently exercises:

| Namespace | What it's for | Backend source |
|---|---|---|
| `aiVault` | Read-only listing of AI-vault (Claude/Codex account) session history for the mobile agent-history panel | `runtime/rpc/methods/ai-vault.ts` |
| `clipboard` | Chunked image upload from the phone's clipboard/camera roll into a worktree (`start`/`appendChunk`/`commit`/`abort`) plus `saveImageAsTempFile` | `runtime/rpc/methods/clipboard.ts` |
| `markdown` | Read/save the open tab's markdown document (mobile's floating-markdown-doc equivalent) | `runtime/rpc/methods/*markdown*` |
| `notifications` | Push-subscription lifecycle over RPC instead of HTTP (see architecture note above) | `runtime/rpc/methods/notifications*` |
| `session.tabs` | Query/activate/close/create terminal tabs and subscribe to tab-list changes for the worktree session screen — desktop keeps this as local pane state, mobile must fetch it remotely | `runtime/rpc/methods/session-tabs.ts` |
| `speech` | On-device dictation: model list/download/delete, and streaming a dictation session's setup/start/chunk/cancel/finish | `runtime/rpc/methods/speech.ts` |
| `stats` | Home-screen summary counts (`stats.summary`) | `runtime/rpc/methods/stats.ts` |
| `ui` / `settings` | Called directly as RPC (no local merge layer — see architecture note) | `runtime/rpc/methods/client-ui.ts` |
| `runtime.clientEvents` | Subscribes to backend-pushed client events (used to keep a worktree's live display name in sync) | `runtime/runtime-rpc.ts` |

### `accounts.*`

| Method | Backend? | Called from |
|---|---|---|
| `accounts.list` | ✅ | `app/h/[hostId]/accounts.tsx`<br>`app/index.tsx` |
| `accounts.subscribe` | ✅ | `app/h/[hostId]/accounts.tsx`<br>`app/index.tsx` |

### `aiVault.*`

| Method | Backend? | Called from |
|---|---|---|
| `aiVault.listSessions` | ✅ | `src/agent-history/use-mobile-agent-history-state.ts` |

### `browser.*`

| Method | Backend? | Called from |
|---|---|---|
| `browser.mouseDown` | ✅ | `src/browser/MobileBrowserPane.tsx` |
| `browser.mouseMove` | ✅ | `src/browser/MobileBrowserPane.tsx` |
| `browser.mouseUp` | ✅ | `src/browser/MobileBrowserPane.tsx` |
| `browser.mouseWheel` | ✅ | `src/browser/MobileBrowserPane.tsx` |
| `browser.screencast` | ✅ | `src/browser/MobileBrowserPane.tsx` (subscribe) |
| `browser.tabCreate` | ✅ | `app/h/[hostId]/session/[worktreeId].tsx` |

### `clipboard.*`

| Method | Backend? | Called from |
|---|---|---|
| `clipboard.abortImageUpload` | ✅ | `src/session/mobile-clipboard-image.ts` |
| `clipboard.appendImageUploadChunk` | ✅ | `src/session/mobile-clipboard-image.ts` |
| `clipboard.commitImageUpload` | ✅ | `src/session/mobile-clipboard-image.ts` |
| `clipboard.saveImageAsTempFile` | ✅ | `src/session/mobile-clipboard-image.ts` |
| `clipboard.startImageUpload` | ✅ | `src/session/mobile-clipboard-image.ts` |

### `files.*`

| Method | Backend? | Called from |
|---|---|---|
| `files.createFile` | ✅ | `app/h/[hostId]/session/[worktreeId].tsx` |
| `files.list` | ✅ | `src/files/MobileFileExplorerPanel.tsx` |
| `files.open` | ✅ | `app/h/[hostId]/session/[worktreeId].tsx`<br>`src/session/mobile-terminal-file-tap-open.ts`<br>`src/source-control/use-mobile-source-control-openers.ts` |
| `files.openDiff` | ✅ | `src/session/use-mobile-diff-review-interactions.ts`<br>`src/source-control/use-mobile-source-control-openers.ts` |
| `files.read` | ✅ | `app/h/[hostId]/session/[worktreeId].tsx` |
| `files.readDir` | ✅ | `src/files/MobileFileExplorerPanel.tsx` |
| `files.readPreview` | ✅ | `app/h/[hostId]/session/[worktreeId].tsx` |
| `files.resolveTerminalPath` | ✅ | `src/files/mobile-terminal-artifact-grant-refresh.ts`<br>`src/session/mobile-terminal-file-tap-open.ts` |
| `files.writeTerminalArtifact` | ✅ | `src/files/mobile-file-preview-request.ts` |

### `folderWorkspace.*`

| Method | Backend? | Called from |
|---|---|---|
| `folderWorkspace.list` | ✅ | `src/agent-history/MobileAgentSessionHistoryPanel.tsx` |

### `git.*`

| Method | Backend? | Called from |
|---|---|---|
| `git.branchCompare` | ✅ | `src/session/mobile-diff-review-loaders.ts`<br>`src/session/use-mobile-pr-branch-context.ts`<br>`src/source-control/use-mobile-source-control-loaders.ts` |
| `git.branchDiff` | ✅ | `src/session/mobile-diff-review-loaders.ts`<br>`src/source-control/use-mobile-source-control-openers.ts` |
| `git.cancelGenerateCommitMessage` | ✅ | `src/source-control/mobile-commit-message-ai.ts` |
| `git.commit` | ✅ | `src/source-control/mobile-hosted-review-git-preparation.ts` |
| `git.commitCompare` | ✅ | `src/source-control/MobileGitHistoryList.tsx` |
| `git.diff` | ✅ | `app/h/[hostId]/session/[worktreeId].tsx`<br>`src/session/mobile-diff-review-loaders.ts` |
| `git.generateCommitMessage` | ✅ | `src/source-control/mobile-commit-message-ai.ts` |
| `git.generatePullRequestFields` | ✅ | `src/components/pr-sidebar/MobilePrComposeForm.tsx` |
| `git.history` | ✅ | `src/source-control/mobile-git-history.ts` |
| `git.push` | ✅ | `src/source-control/mobile-hosted-review-service.ts` |
| `git.stage` | ✅ | `src/session/use-mobile-diff-review-git-actions.ts` |
| `git.status` | ✅ | `src/session/mobile-diff-review-loaders.ts`<br>`src/session/use-mobile-pr-branch-context.ts`<br>`src/source-control/mobile-hosted-review-git-preparation.ts`<br>`src/source-control/use-mobile-source-control-loaders.ts` |

### `github.*`

| Method | Backend? | Called from |
|---|---|---|
| `github.addIssueComment` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `github.addPRReviewComment` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `github.addPRReviewCommentReply` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `github.countWorkItems` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `github.listAssignableUsers` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `github.listLabels` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `github.listWorkItems` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `github.mergePR` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `github.prChecks` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `github.prFileContents` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `github.project.addIssueCommentBySlug` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `github.project.deleteIssueCommentBySlug` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `github.project.listAccessible` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `github.project.listAssignableUsersBySlug` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `github.project.listIssueTypesBySlug` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `github.project.listLabelsBySlug` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `github.project.listViews` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `github.project.resolveRef` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `github.project.updateIssueBySlug` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `github.project.updateIssueCommentBySlug` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `github.project.updateIssueTypeBySlug` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `github.project.viewTable` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `github.project.workItemDetailsBySlug` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `github.repoSlug` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `github.requestPRReviewers` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `github.rerunPRChecks` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `github.resolveReviewThread` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `github.setPRFileViewed` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `github.updateIssue` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `github.updatePR` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `github.workItemDetails` | ✅ | `app/h/[hostId]/tasks.tsx` |

### `gitlab.*`

| Method | Backend? | Called from |
|---|---|---|
| `gitlab.listWorkItems` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `gitlab.mergeMR` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `gitlab.todos` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `gitlab.updateIssue` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `gitlab.updateMRState` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `gitlab.workItemDetails` | ✅ | `app/h/[hostId]/tasks.tsx` |

### `hostedReview.*`

| Method | Backend? | Called from |
|---|---|---|
| `hostedReview.create` | ✅ | `src/source-control/mobile-hosted-review-service.ts` |
| `hostedReview.getCreationEligibility` | ✅ | `src/source-control/mobile-hosted-review-service.ts` |

### `linear.*`

| Method | Backend? | Called from |
|---|---|---|
| `linear.addIssueComment` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `linear.connect` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `linear.createIssue` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `linear.getIssue` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `linear.issueComments` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `linear.listIssues` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `linear.listTeams` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `linear.searchIssues` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `linear.selectWorkspace` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `linear.status` | ✅ | `app/h/[hostId]/tasks.tsx`<br>`app/index.tsx` |
| `linear.teamStates` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `linear.updateIssue` | ✅ | `app/h/[hostId]/tasks.tsx` |

### `markdown.*`

| Method | Backend? | Called from |
|---|---|---|
| `markdown.readTab` | ✅ | `app/h/[hostId]/session/[worktreeId].tsx` |
| `markdown.saveTab` | ✅ | `app/h/[hostId]/session/[worktreeId].tsx` |

### `notifications.*`

| Method | Backend? | Called from |
|---|---|---|
| `notifications.subscribe` | ✅ | `src/notifications/mobile-notifications.ts` (subscribe) |
| `notifications.unsubscribe` | ✅ | `src/notifications/mobile-notifications.ts` |

### `preflight.*`

| Method | Backend? | Called from |
|---|---|---|
| `preflight.check` | ✅ | `app/h/[hostId]/tasks.tsx`<br>`app/index.tsx` |
| `preflight.detectAgents` | ✅ | `app/h/[hostId]/session/[worktreeId].tsx`<br>`app/h/[hostId]/tasks.tsx`<br>`src/components/NewWorktreeModal.tsx` |
| `preflight.detectRemoteAgents` | ✅ | `app/h/[hostId]/session/[worktreeId].tsx`<br>`app/h/[hostId]/tasks.tsx`<br>`src/components/NewWorktreeModal.tsx` |

### `projectGroup.*`

| Method | Backend? | Called from |
|---|---|---|
| `projectGroup.list` | ✅ | `src/agent-history/MobileAgentSessionHistoryPanel.tsx` |

### `repo.*`

| Method | Backend? | Called from |
|---|---|---|
| `repo.baseRefDefault` | ✅ | `src/source-control/mobile-base-ref-search.ts`<br>`src/source-control/mobile-branch-base-ref.ts` |
| `repo.hooks` | ✅ | `app/h/[hostId]/tasks.tsx`<br>`src/components/NewWorktreeModal.tsx` |
| `repo.list` | ✅ | `app/h/[hostId]/index.tsx`<br>`app/h/[hostId]/session/[worktreeId].tsx`<br>`app/h/[hostId]/tasks.tsx`<br>`src/agent-history/MobileAgentSessionHistoryPanel.tsx`<br>`src/components/NewWorktreeModal.tsx`<br>`src/source-control/mobile-branch-base-ref.ts` |
| `repo.saveSparsePreset` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `repo.searchRefs` | ✅ | `app/h/[hostId]/tasks.tsx`<br>`src/source-control/mobile-base-ref-search.ts` |
| `repo.sparsePresets` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `repo.update` | ✅ | `app/h/[hostId]/tasks.tsx` |

### `runtime.clientEvents.*`

| Method | Backend? | Called from |
|---|---|---|
| `runtime.clientEvents.subscribe` | ✅ | `src/session/use-live-worktree-name.ts` (subscribe) |

### `session.tabs.*`

| Method | Backend? | Called from |
|---|---|---|
| `session.tabs.activate` | ✅ | `app/h/[hostId]/session/[worktreeId].tsx` |
| `session.tabs.close` | ✅ | `app/h/[hostId]/session/[worktreeId].tsx` |
| `session.tabs.createTerminal` | ✅ | `app/h/[hostId]/session/[worktreeId].tsx`<br>`src/session/ai-vault-resume-launch.ts`<br>`src/session/pr-ai-triage-launch.ts`<br>`src/session/use-mobile-diff-review-send-actions.ts` |
| `session.tabs.list` | ✅ | `app/h/[hostId]/session/[worktreeId].tsx`<br>`src/session/use-mobile-diff-review-send-actions.ts` |
| `session.tabs.subscribe` | ✅ | `app/h/[hostId]/session/[worktreeId].tsx` (subscribe) |

### `settings.*`

| Method | Backend? | Called from |
|---|---|---|
| `settings.get` | ✅ | `app/h/[hostId]/session/[worktreeId].tsx`<br>`app/h/[hostId]/tasks.tsx`<br>`app/index.tsx`<br>`src/agent-history/MobileAgentSessionHistoryPanel.tsx`<br>`src/components/NewWorktreeModal.tsx`<br>`src/session/use-pr-bot-author-overrides.ts` |
| `settings.update` | ✅ | `app/h/[hostId]/tasks.tsx` |

### `speech.*`

| Method | Backend? | Called from |
|---|---|---|
| `speech.dictation.cancel` | ✅ | `src/hooks/mobile-dictation-desktop-start.ts`<br>`src/hooks/use-mobile-dictation.ts` |
| `speech.dictation.chunk` | ✅ | `src/hooks/mobile-dictation-audio-chunk.ts` |
| `speech.dictation.finish` | ✅ | `src/hooks/use-mobile-dictation.ts` |
| `speech.dictation.setup` | ✅ | `src/dictation/mobile-dictation-setup.ts` |
| `speech.dictation.start` | ✅ | `src/hooks/mobile-dictation-desktop-start.ts` |
| `speech.models.delete` | ✅ | `src/dictation/mobile-dictation-setup.ts` |
| `speech.models.download` | ✅ | `src/dictation/mobile-dictation-setup.ts` |
| `speech.models.list` | ✅ | `src/dictation/mobile-dictation-setup.ts` |

### `ssh.*`

| Method | Backend? | Called from |
|---|---|---|
| `ssh.connect` | ✅ | `app/h/[hostId]/tasks.tsx`<br>`src/components/NewWorktreeModal.tsx` |
| `ssh.getState` | ✅ | `app/h/[hostId]/tasks.tsx`<br>`src/components/NewWorktreeModal.tsx` |

### `stats.*`

| Method | Backend? | Called from |
|---|---|---|
| `stats.summary` | ✅ | `app/index.tsx` |

### `status.*`

| Method | Backend? | Called from |
|---|---|---|
| `status.get` | ✅ | `app/h/[hostId]/index.tsx`<br>`app/h/[hostId]/session/[worktreeId].tsx`<br>`app/h/[hostId]/tasks.tsx`<br>`app/pair-confirm.tsx`<br>`app/pair-scan.tsx`<br>`src/agent-history/use-mobile-agent-history-state.ts` |

### `terminal.*`

| Method | Backend? | Called from |
|---|---|---|
| `terminal.clearBuffer` | ✅ | `app/h/[hostId]/session/[worktreeId].tsx` |
| `terminal.close` | ✅ | `app/h/[hostId]/session/[worktreeId].tsx` |
| `terminal.focus` | ✅ | `app/h/[hostId]/session/[worktreeId].tsx` |
| `terminal.getAutoRestoreFit` | ✅ | `app/terminal-settings.tsx` |
| `terminal.list` | ✅ | `app/h/[hostId]/session/[worktreeId].tsx` |
| `terminal.rename` | ✅ | `app/h/[hostId]/session/[worktreeId].tsx` |
| `terminal.send` | ✅ | `app/h/[hostId]/session/[worktreeId].tsx`<br>`src/session/ai-vault-resume-launch.ts`<br>`src/session/mobile-image-attachment.ts`<br>`src/session/pr-ai-triage-launch.ts`<br>`src/session/use-mobile-diff-review-send-actions.ts`<br>`src/session/use-mobile-terminal-paste.ts`<br>`src/terminal/mobile-terminal-query-reply.ts` |
| `terminal.setAutoRestoreFit` | ✅ | `app/terminal-settings.tsx` |
| `terminal.setDisplayMode` | ✅ | `app/h/[hostId]/session/[worktreeId].tsx` |
| `terminal.subscribe` | ✅ | `app/h/[hostId]/session/[worktreeId].tsx` (subscribe) |
| `terminal.updateViewport` | ✅ | `src/terminal/terminal-viewport-refit.ts` |

### `ui.*`

| Method | Backend? | Called from |
|---|---|---|
| `ui.get` | ✅ | `app/h/[hostId]/index.tsx`<br>`app/h/[hostId]/tasks.tsx`<br>`src/components/NewWorktreeModal.tsx` |
| `ui.set` | ✅ | `app/h/[hostId]/index.tsx`<br>`app/h/[hostId]/tasks.tsx`<br>`src/components/NewWorktreeModal.tsx` |

### `worktree.*`

| Method | Backend? | Called from |
|---|---|---|
| `worktree.activate` | ✅ | `app/h/[hostId]/index.tsx`<br>`app/h/[hostId]/session/[worktreeId].tsx` |
| `worktree.create` | ✅ | `app/h/[hostId]/tasks.tsx`<br>`src/components/NewWorktreeModal.tsx` |
| `worktree.ps` | ✅ | `app/h/[hostId]/index.tsx`<br>`app/index.tsx`<br>`src/agent-history/MobileAgentSessionHistoryPanel.tsx` |
| `worktree.resolveMrBase` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `worktree.resolvePrBase` | ✅ | `app/h/[hostId]/tasks.tsx` |
| `worktree.rm` | ✅ | `app/h/[hostId]/index.tsx` |
| `worktree.set` | ✅ | `app/h/[hostId]/index.tsx`<br>`app/h/[hostId]/session/[worktreeId].tsx`<br>`src/session/use-mobile-diff-review-comment-actions.ts`<br>`src/source-control/mobile-pr-link.ts` |
| `worktree.show` | ✅ | `app/h/[hostId]/session/[worktreeId].tsx`<br>`src/session/mobile-diff-review-loaders.ts`<br>`src/session/use-live-worktree-name.ts`<br>`src/source-control/mobile-branch-base-ref.ts`<br>`src/source-control/mobile-pr-link.ts` |
| `worktree.sleep` | ✅ | `app/h/[hostId]/index.tsx` |

---

## Methodology

- **Frontend (mobile) call sites**: every `.ts`/`.tsx` file under `mobile/src`
  and `mobile/app`, scanned for `sendRequest('<method>', …)` and
  `.subscribe('<method>', …)` call expressions (multiline-aware — several
  call sites, especially in the large `app/h/[hostId]/tasks.tsx` screen,
  format the method string on its own line). Test-only call sites
  (`*.test.ts`) were excluded from the tables above but didn't add any
  method not already covered by a production call site.
- **Backend registry**: same 707-method registry `README.md` describes for
  frontend (`grep -rhoE "name: '[a-zA-Z][a-zA-Z0-9]*\.[a-zA-Z0-9_.]+'"` across
  `backend/src/main/`). Note this is up from the 583 recorded when
  `rpc-catalog.md` was last regenerated (2026-08-15) — the registry has grown
  ~124 methods in the 12 days since; `rpc-catalog.md`'s per-namespace tables
  reflect the frontend's call sites as of that date, not the registry's
  current full size (most of the registry, mobile- or frontend-unused, is
  intentionally not enumerated there either — see its README note).
- **Cross-reference**: `comm -23` between the sorted mobile method-name list
  and the sorted backend registry — empty, i.e. 0 gaps.
- Same limits as `rpc-catalog.md`: this is a **static, name-level** check.
  It confirms a method exists on the backend, not that params/response shape
  match between mobile's call site and the handler's zod schema.

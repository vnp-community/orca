# RPC Method Catalog

Every `callRuntimeRpc(target, '<method>', params)` call site found in `frontend/`,
grouped by namespace, cross-referenced against the backend's RPC method registry
(`backend/src/main/runtime/rpc/methods/*.ts` + `project-rpc-handler.ts` +
`team-rpc-handler.ts` + `workflow-rpc-handler.ts`). See [`README.md`](./README.md)
for how this was generated and its limits (name-level only, doesn't check param
shape).

**262 methods called, 262 confirmed on the backend, 0 missing** (updated
2026-08-15, re-run with the same grep/perl methodology described in
[`README.md`](./README.md)). The 9 gaps this doc originally flagged (namespace
typos, a `git.exec` escape hatch, and 4 genuinely-missing features) have all
been fixed on both frontend and backend — see the "Resolved gaps" note at the
bottom of this file for what changed. `gaps-and-mismatches.md` has been
removed since it no longer describes current reality.

Historical note: `project.*` (singular — the real, current namespace; v5.0
collaborative/membership-scoped projects, backend:
`project/project-rpc-handler.ts`) and `projects.*` (plural) used to exist side
by side and were easy to confuse. `MemberManager.tsx`'s 3 calls to the wrong
plural namespace are now fixed to call the real `project.*` methods — the
`projects.*` namespace no longer appears anywhere in the frontend.

| Namespace | What it's for |
|---|---|
| `accounts` | Claude/Codex account selection for AI provider auth |
| `aiProvider` | AI provider (Anthropic/OpenAI/etc.) config CRUD, credentials |
| `annotation` | Code-review inline comments — real backend (`orca_annotations` table, migration 0018) as of 2026-08-15 |
| `automation` | Scheduled/triggered automation definitions + runs |
| `browser` | Embedded browser pane control (navigation, input, cookies, tabs) |
| `credentials` | Web Server mode (`ORCA_MULTI_USER=1`) credential storage (`WebCredentialStore`) — set/revoke/status/list |
| `devServer` | Dev server registration/listing (SSH targets used to run workspaces) |
| `emulator` | Mobile device emulator control |
| `files` | Worktree-scoped file read/write/search/watch (the `files.*` surface documented earlier this session) |
| `fleet` | Multi-host fleet health polling — real backend (`fleet.health.checkAll`, thin wrapper over existing `fleet-health-store`/`fleet-status-service`) as of 2026-08-15 |
| `folderWorkspace` | Non-git "folder" workspaces (no repo attached) |
| `git` | Worktree-scoped git operations (status/diff/commit/push/branches/…) |
| `github` | GitHub issues/PRs/projects integration |
| `gitlab` | GitLab issues/MRs integration |
| `host` | Execution-host capability queries |
| `hostedReview` | Provider-agnostic hosted PR/MR read+create (GitHub/GitLab/Bitbucket/Azure DevOps/Gitea) |
| `jira` | Jira issues integration |
| `linear` | Linear issues/projects integration |
| `nativeChat` | In-app native chat session transport |
| `orchestration` | Agent-team orchestration gates |
| `preflight` | Startup capability/agent-detection checks |
| `profile` | User/company/department profile resolution (v5.0 multi-user) |
| `project` | v5.0 collaborative projects — members, settings (see namespace note above) |
| `projectGroup` | Sidebar project grouping/folders |
| `projectHostSetup` | Binding a project to a dev-server host (legacy repo/host-setup model — see `docs/ui/pages/project-workspace.md` and this session's BUG-FE-RPC-005 fix for how this interacts with `project.*`) |
| `repo` | Repo catalog (list/add/clone/update), the legacy single-user desktop model |
| `ssh` | SSH target/fleet connection management |
| `status` | Runtime status/capability handshake (`status.get`) |
| `task` | Orca's local task graph |
| `team` | Team membership (v5.0 multi-user) |
| `terminal` | PTY session lifecycle (create/read/send/resize/subscribe/…) |
| `workflow` | Saved workflow templates + execution |
| `workspace` | Legacy workspace init/teardown/file-tree/git-status (mostly superseded by `files.*`/`git.*`) |
| `workspacePorts` | Detected dev-server port scanning |
| `worktree` | Worktree lifecycle (create/list/activate/sleep/…) |


### `accounts.*`

| Method | Backend? | Called from |
|---|---|---|
| `accounts.removeClaude` | ✅ | `renderer/src/runtime/runtime-provider-accounts-client.ts` |
| `accounts.removeCodex` | ✅ | `renderer/src/runtime/runtime-provider-accounts-client.ts` |
| `accounts.selectClaude` | ✅ | `renderer/src/runtime/runtime-provider-accounts-client.ts` |
| `accounts.selectCodex` | ✅ | `renderer/src/runtime/runtime-provider-accounts-client.ts` |

### `aiProvider.*`

| Method | Backend? | Called from |
|---|---|---|
| `aiProvider.create` | ✅ | `renderer/src/components/ai-provider/ProviderForm.tsx`<br>`renderer/src/hooks/useAIProviders.ts` |
| `aiProvider.delete` | ✅ | `renderer/src/hooks/useAIProviders.ts` |
| `aiProvider.list` | ✅ | `renderer/src/hooks/useAIProviders.ts` |
| `aiProvider.testConnection` | ✅ | `renderer/src/hooks/useAIProviders.ts` |
| `aiProvider.update` | ✅ | `renderer/src/components/ai-provider/ProviderForm.tsx`<br>`renderer/src/hooks/useAIProviders.ts` |
| `aiProvider.writeCredential` | ✅ | `renderer/src/components/ai-provider/ProviderForm.tsx` |

### `annotation.*`

| Method | Backend? | Called from |
|---|---|---|
| `annotation.create` | ✅ | `renderer/src/components/code-review/annotation-panel.tsx` |
| `annotation.list` | ✅ | `renderer/src/components/code-review/annotation-panel.tsx` |

### `automation.*`

| Method | Backend? | Called from |
|---|---|---|
| `automation.create` | ✅ | `renderer/src/components/automations/automation-host-client.ts` |
| `automation.delete` | ✅ | `renderer/src/components/automations/automation-host-client.ts` |
| `automation.list` | ✅ | `renderer/src/components/automations/automation-host-client.ts` |
| `automation.runNow` | ✅ | `renderer/src/components/automations/automation-host-client.ts` |
| `automation.runs` | ✅ | `renderer/src/components/automations/automation-host-client.ts` |
| `automation.update` | ✅ | `renderer/src/components/automations/automation-host-client.ts` |

### `browser.*`

| Method | Backend? | Called from |
|---|---|---|
| `browser.eval` | ✅ | `renderer/src/components/browser-pane/browser-pane-remote.tsx` |
| `browser.keypress` | ✅ | `renderer/src/components/browser-pane/browser-pane-remote.tsx` |
| `browser.mouseDown` | ✅ | `renderer/src/components/browser-pane/browser-pane-remote.tsx` |
| `browser.mouseMove` | ✅ | `renderer/src/components/browser-pane/browser-pane-remote.tsx` |
| `browser.mouseUp` | ✅ | `renderer/src/components/browser-pane/browser-pane-remote.tsx` |
| `browser.mouseWheel` | ✅ | `renderer/src/components/browser-pane/browser-pane-remote.tsx` |
| `browser.profileClearDefaultCookies` | ✅ | `renderer/src/store/slices/browser.ts` |
| `browser.profileCreate` | ✅ | `renderer/src/store/slices/browser.ts` |
| `browser.profileDelete` | ✅ | `renderer/src/store/slices/browser.ts` |
| `browser.profileDetectBrowsers` | ✅ | `renderer/src/store/slices/browser.ts` |
| `browser.profileImportFromBrowser` | ✅ | `renderer/src/store/slices/browser.ts` |
| `browser.profileList` | ✅ | `renderer/src/store/slices/browser.ts` |
| `browser.tabClose` | ✅ | `renderer/src/components/browser-pane/browser-pane-remote.tsx`<br>`renderer/src/store/slices/browser.ts` |
| `browser.tabCreate` | ✅ | `renderer/src/components/browser-pane/browser-pane-remote.tsx`<br>`renderer/src/lib/workspace-port-actions.ts` |
| `browser.viewport` | ✅ | `renderer/src/components/browser-pane/browser-pane-remote.tsx` |

### `credentials.*`

| Method | Backend? | Called from |
|---|---|---|
| `credentials.list` | ✅ | `renderer/src/runtime/runtime-credentials-client.ts` |
| `credentials.revoke` | ✅ | `renderer/src/components/settings/CredentialInputForm.tsx`<br>`renderer/src/runtime/runtime-credentials-client.ts` |
| `credentials.set` | ✅ | `renderer/src/components/settings/CredentialInputForm.tsx`<br>`renderer/src/runtime/runtime-credentials-client.ts` |
| `credentials.status` | ✅ | `renderer/src/runtime/runtime-credentials-client.ts` |

### `devServer.*`

| Method | Backend? | Called from |
|---|---|---|
| `devServer.list` | ✅ | `renderer/src/components/project/CreateProjectDialog.tsx` |

### `emulator.*`

| Method | Backend? | Called from |
|---|---|---|
| `emulator.attach` | ✅ | `renderer/src/components/emulator-pane/use-emulator-pane-session.ts`<br>`renderer/src/lib/open-mobile-emulator-tab.ts` |
| `emulator.availability` | ✅ | `renderer/src/components/settings/MobileEmulatorSettingsPane.tsx` |
| `emulator.button` | ✅ | `renderer/src/components/emulator-pane/use-emulator-pane-controls.ts` |
| `emulator.gesture` | ✅ | `renderer/src/components/emulator-pane/use-emulator-pane-controls.ts` |
| `emulator.listDevices` | ✅ | `renderer/src/components/emulator-pane/use-emulator-pane-session.ts` |
| `emulator.rotate` | ✅ | `renderer/src/components/emulator-pane/use-emulator-pane-controls.ts` |
| `emulator.shutdown` | ✅ | `renderer/src/components/emulator-pane/use-emulator-pane-shutdown.ts`<br>`renderer/src/lib/simulator-pane-shutdown-scheduler.ts` |
| `emulator.tap` | ✅ | `renderer/src/components/emulator-pane/use-emulator-pane-controls.ts` |

### `files.*`

| Method | Backend? | Called from |
|---|---|---|
| `files.commitUpload` | ✅ | `renderer/src/runtime/runtime-file-client.ts` |
| `files.copy` | ✅ | `renderer/src/runtime/runtime-file-client.ts` |
| `files.createDir` | ✅ | `renderer/src/runtime/runtime-file-client.ts` |
| `files.createDirNoClobber` | ✅ | `renderer/src/runtime/runtime-file-client.ts` |
| `files.delete` | ✅ | `renderer/src/components/workspace/FileContextMenu.tsx`<br>`renderer/src/runtime/runtime-file-client.ts` |
| `files.listAll` | ✅ | `renderer/src/runtime/runtime-file-client.ts` |
| `files.listMarkdownDocuments` | ✅ | `renderer/src/runtime/runtime-file-client.ts` |
| `files.read` | ✅ | `renderer/src/components/workspace/FileViewer.tsx`<br>`renderer/src/runtime/runtime-file-client.ts` |
| `files.readChunk` | ✅ | `renderer/src/runtime/runtime-file-client.ts` |
| `files.readDir` | ✅ | `renderer/src/runtime/runtime-file-client.ts` |
| `files.readPreview` | ✅ | `renderer/src/runtime/runtime-file-client.ts` |
| `files.rename` | ✅ | `renderer/src/runtime/runtime-file-client.ts` |
| `files.search` | ✅ | `renderer/src/components/workspace/FileSearchPanel.tsx`<br>`renderer/src/runtime/runtime-file-client.ts` |
| `files.stat` | ✅ | `renderer/src/runtime/runtime-file-client.ts` |
| `files.unwatch` | ✅ | `renderer/src/runtime/runtime-file-client.ts` |
| `files.write` | ✅ | `renderer/src/runtime/runtime-file-client.ts` |
| `files.writeBase64` | ✅ | `renderer/src/runtime/runtime-file-client.ts` |
| `files.writeBase64Chunk` | ✅ | `renderer/src/runtime/runtime-file-client.ts` |

### `fleet.*`

| Method | Backend? | Called from |
|---|---|---|
| `fleet.health.checkAll` | ✅ | `renderer/src/hooks/use-fleet-health-polling.ts` |

### `folderWorkspace.*`

| Method | Backend? | Called from |
|---|---|---|
| `folderWorkspace.create` | ✅ | `renderer/src/store/slices/repos.ts` |
| `folderWorkspace.delete` | ✅ | `renderer/src/store/slices/repos.ts` |
| `folderWorkspace.getPathStatus` | ✅ | `renderer/src/store/slices/repos.ts` |
| `folderWorkspace.list` | ✅ | `renderer/src/store/slices/repos.ts` |
| `folderWorkspace.update` | ✅ | `renderer/src/store/slices/repos.ts` |

### `git.*`

| Method | Backend? | Called from |
|---|---|---|
| `git.abortMerge` | ✅ | `renderer/src/runtime/runtime-git-client.ts` |
| `git.abortRebase` | ✅ | `renderer/src/runtime/runtime-git-client.ts` |
| `git.branchCompare` | ✅ | `renderer/src/runtime/runtime-git-client.ts` |
| `git.branchDiff` | ✅ | `renderer/src/runtime/runtime-git-client.ts` |
| `git.bulkDiscard` | ✅ | `renderer/src/runtime/runtime-git-client.ts` |
| `git.bulkStage` | ✅ | `renderer/src/hooks/useGit.ts`<br>`renderer/src/runtime/runtime-git-client.ts` |
| `git.bulkUnstage` | ✅ | `renderer/src/hooks/useGit.ts`<br>`renderer/src/runtime/runtime-git-client.ts` |
| `git.cancelGenerateCommitMessage` | ✅ | `renderer/src/runtime/runtime-git-client.ts` |
| `git.cancelGeneratePullRequestFields` | ✅ | `renderer/src/runtime/runtime-git-client.ts` |
| `git.checkIgnored` | ✅ | `renderer/src/runtime/runtime-git-client.ts` |
| `git.checkout` | ✅ | `renderer/src/components/workspace/git/BranchManager.tsx` |
| `git.commit` | ✅ | `renderer/src/hooks/use-code-review.ts`<br>`renderer/src/hooks/useGit.ts`<br>`renderer/src/runtime/runtime-git-client.ts` |
| `git.commitCompare` | ✅ | `renderer/src/runtime/runtime-git-client.ts` |
| `git.commitDiff` | ✅ | `renderer/src/runtime/runtime-git-client.ts` |
| `git.conflictOperation` | ✅ | `renderer/src/runtime/runtime-git-client.ts` |
| `git.diff` | ✅ | `renderer/src/components/workspace/git/DiffViewer.tsx`<br>`renderer/src/hooks/useGit.ts`<br>`renderer/src/runtime/runtime-git-client.ts` |
| `git.discard` | ✅ | `renderer/src/runtime/runtime-git-client.ts` |
| `git.discoverCommitMessageModels` | ✅ | `renderer/src/runtime/runtime-git-client.ts` |
| `git.fastForward` | ✅ | `renderer/src/runtime/runtime-git-client.ts` |
| `git.fetch` | ✅ | `renderer/src/runtime/runtime-git-client.ts` |
| `git.forkSync` | ✅ | `renderer/src/runtime/runtime-git-client.ts` |
| `git.generateCommitMessage` | ✅ | `renderer/src/components/code-review/commit-message-generator.tsx`<br>`renderer/src/hooks/useGit.ts`<br>`renderer/src/runtime/runtime-git-client.ts` |
| `git.generatePullRequestFields` | ✅ | `renderer/src/runtime/runtime-git-client.ts` |
| `git.history` | ✅ | `renderer/src/components/workspace/git/GitHistory.tsx`<br>`renderer/src/runtime/runtime-git-client.ts` |
| `git.localBranches` | ✅ | `renderer/src/components/workspace/git/BranchManager.tsx` |
| `git.pull` | ✅ | `renderer/src/runtime/runtime-git-client.ts` |
| `git.push` | ✅ | `renderer/src/runtime/runtime-git-client.ts` |
| `git.rebaseFromBase` | ✅ | `renderer/src/runtime/runtime-git-client.ts` |
| `git.remoteCommitUrl` | ✅ | `renderer/src/runtime/runtime-git-client.ts` |
| `git.remoteFileUrl` | ✅ | `renderer/src/runtime/runtime-git-client.ts` |
| `git.stage` | ✅ | `renderer/src/hooks/useGit.ts`<br>`renderer/src/runtime/runtime-git-client.ts` |
| `git.status` | ✅ | `renderer/src/context/WorkspaceContext.tsx`<br>`renderer/src/hooks/use-code-review.ts`<br>`renderer/src/hooks/useGit.ts`<br>`renderer/src/runtime/runtime-git-client.ts` |
| `git.submoduleStatus` | ✅ | `renderer/src/runtime/runtime-git-client.ts` |
| `git.unstage` | ✅ | `renderer/src/hooks/useGit.ts`<br>`renderer/src/runtime/runtime-git-client.ts` |
| `git.upstreamStatus` | ✅ | `renderer/src/runtime/runtime-git-client.ts` |

### `github.*`

| Method | Backend? | Called from |
|---|---|---|
| `github.mergePR` | ✅ | `renderer/src/components/task-page-github-review-cells.tsx` |
| `github.prForBranch` | ✅ | `renderer/src/store/slices/github.ts` |
| `github.project.addIssueCommentBySlug` | ✅ | `renderer/src/components/github-project/slug-dialog/Comments.tsx` |
| `github.project.clearItemField` | ✅ | `renderer/src/store/slices/github.ts` |
| `github.project.deleteIssueCommentBySlug` | ✅ | `renderer/src/components/github-project/slug-dialog/Comments.tsx` |
| `github.project.listAccessible` | ✅ | `renderer/src/components/github-project/ProjectPicker.tsx` |
| `github.project.listAssignableUsersBySlug` | ✅ | `renderer/src/hooks/useGitHubSlugMetadata.ts` |
| `github.project.listIssueTypesBySlug` | ✅ | `renderer/src/components/github-project/ProjectCell.tsx` |
| `github.project.listLabelsBySlug` | ✅ | `renderer/src/hooks/useGitHubSlugMetadata.ts` |
| `github.project.listViews` | ✅ | `renderer/src/components/github-project/ProjectPicker.tsx`<br>`renderer/src/components/github-project/ProjectViewWrapper.tsx` |
| `github.project.resolveRef` | ✅ | `renderer/src/components/github-project/ProjectPicker.tsx` |
| `github.project.updateIssueBySlug` | ✅ | `renderer/src/components/task-page-github-assignee-cells.tsx`<br>`renderer/src/store/slices/github.ts` |
| `github.project.updateIssueCommentBySlug` | ✅ | `renderer/src/components/github-project/slug-dialog/Comments.tsx` |
| `github.project.updateIssueTypeBySlug` | ✅ | `renderer/src/store/slices/github.ts` |
| `github.project.updateItemField` | ✅ | `renderer/src/store/slices/github.ts` |
| `github.project.updatePullRequestBySlug` | ✅ | `renderer/src/store/slices/github.ts` |
| `github.project.viewTable` | ✅ | `renderer/src/store/slices/github.ts` |
| `github.project.workItemDetailsBySlug` | ✅ | `renderer/src/components/github-project/slug-dialog/SlugDialogBody.tsx` |
| `github.rateLimit` | ✅ | `renderer/src/components/github/github-rate-limit-display.tsx` |
| `github.removePRReviewers` | ✅ | `renderer/src/components/GitHubItemDialog.tsx`<br>`renderer/src/components/PullRequestPage.tsx`<br>`renderer/src/components/task-page-github-review-cells.tsx` |
| `github.repoSlug` | ✅ | `renderer/src/hooks/useComposerState.ts`<br>`renderer/src/lib/repo-slug-index.ts` |
| `github.requestPRReviewers` | ✅ | `renderer/src/components/GitHubItemDialog.tsx`<br>`renderer/src/components/PullRequestPage.tsx`<br>`renderer/src/components/task-page-github-review-cells.tsx` |
| `github.setPRAutoMerge` | ✅ | `renderer/src/components/task-page-github-review-cells.tsx` |
| `github.updateIssue` | ✅ | `renderer/src/components/task-page-github-assignee-cells.tsx` |

### `gitlab.*`

| Method | Backend? | Called from |
|---|---|---|
| `gitlab.listMRs` | ✅ | `renderer/src/lib/gitlab-work-item-source-lookup.ts` |
| `gitlab.rateLimit` | ✅ | `renderer/src/components/gitlab/gitlab-rate-limit-display.tsx` |
| `gitlab.resolveMRDiscussion` | ✅ | `renderer/src/components/right-sidebar/ChecksPanel.tsx` |
| `gitlab.workItemDetails` | ✅ | `renderer/src/components/right-sidebar/ChecksPanel.tsx` |

### `host.*`

| Method | Backend? | Called from |
|---|---|---|
| `host.gitBash.isAvailable` | ✅ | `renderer/src/lib/windows-terminal-capability-read.ts` |
| `host.pwsh.isAvailable` | ✅ | `renderer/src/lib/windows-terminal-capability-read.ts` |
| `host.wsl.isAvailable` | ✅ | `renderer/src/lib/windows-terminal-capability-read.ts` |
| `host.wsl.listDistros` | ✅ | `renderer/src/lib/windows-terminal-capability-read.ts` |

### `hostedReview.*`

| Method | Backend? | Called from |
|---|---|---|
| `hostedReview.create` | ✅ | `renderer/src/store/slices/hosted-review.ts` |
| `hostedReview.forBranch` | ✅ | `renderer/src/store/slices/hosted-review.ts` |
| `hostedReview.getCreationEligibility` | ✅ | `renderer/src/store/slices/hosted-review.ts` |

### `jira.*`

| Method | Backend? | Called from |
|---|---|---|
| `jira.addIssueComment` | ✅ | `renderer/src/runtime/runtime-jira-client.ts` |
| `jira.connect` | ✅ | `renderer/src/runtime/runtime-jira-client.ts` |
| `jira.createIssue` | ✅ | `renderer/src/runtime/runtime-jira-client.ts` |
| `jira.disconnect` | ✅ | `renderer/src/runtime/runtime-jira-client.ts` |
| `jira.getIssue` | ✅ | `renderer/src/runtime/runtime-jira-client.ts` |
| `jira.getProjectStatusOrder` | ✅ | `renderer/src/runtime/runtime-jira-client.ts` |
| `jira.issueComments` | ✅ | `renderer/src/runtime/runtime-jira-client.ts` |
| `jira.listAssignableUsers` | ✅ | `renderer/src/runtime/runtime-jira-client.ts` |
| `jira.listCreateFields` | ✅ | `renderer/src/runtime/runtime-jira-client.ts` |
| `jira.listIssueTypes` | ✅ | `renderer/src/runtime/runtime-jira-client.ts` |
| `jira.listIssues` | ✅ | `renderer/src/runtime/runtime-jira-client.ts` |
| `jira.listPriorities` | ✅ | `renderer/src/runtime/runtime-jira-client.ts` |
| `jira.listProjects` | ✅ | `renderer/src/runtime/runtime-jira-client.ts` |
| `jira.listTransitions` | ✅ | `renderer/src/runtime/runtime-jira-client.ts` |
| `jira.searchIssues` | ✅ | `renderer/src/runtime/runtime-jira-client.ts` |
| `jira.selectSite` | ✅ | `renderer/src/runtime/runtime-jira-client.ts` |
| `jira.status` | ✅ | `renderer/src/runtime/runtime-jira-client.ts` |
| `jira.testConnection` | ✅ | `renderer/src/runtime/runtime-jira-client.ts` |
| `jira.updateIssue` | ✅ | `renderer/src/runtime/runtime-jira-client.ts` |

### `linear.*`

| Method | Backend? | Called from |
|---|---|---|
| `linear.addIssueComment` | ✅ | `renderer/src/runtime/runtime-linear-client.ts` |
| `linear.connect` | ✅ | `renderer/src/runtime/runtime-linear-client.ts` |
| `linear.createIssue` | ✅ | `renderer/src/runtime/runtime-linear-client.ts` |
| `linear.createProject` | ✅ | `renderer/src/runtime/runtime-linear-client.ts` |
| `linear.disconnect` | ✅ | `renderer/src/runtime/runtime-linear-client.ts` |
| `linear.getCustomView` | ✅ | `renderer/src/runtime/runtime-linear-client.ts` |
| `linear.getIssue` | ✅ | `renderer/src/runtime/runtime-linear-client.ts` |
| `linear.getProject` | ✅ | `renderer/src/runtime/runtime-linear-client.ts` |
| `linear.issueComments` | ✅ | `renderer/src/runtime/runtime-linear-client.ts` |
| `linear.listIssues` | ✅ | `renderer/src/runtime/runtime-linear-client.ts` |
| `linear.listTeams` | ✅ | `renderer/src/runtime/runtime-linear-client.ts` |
| `linear.searchIssues` | ✅ | `renderer/src/runtime/runtime-linear-client.ts` |
| `linear.selectWorkspace` | ✅ | `renderer/src/runtime/runtime-linear-client.ts` |
| `linear.status` | ✅ | `renderer/src/runtime/runtime-linear-client.ts` |
| `linear.teamLabels` | ✅ | `renderer/src/runtime/runtime-linear-client.ts` |
| `linear.teamMembers` | ✅ | `renderer/src/runtime/runtime-linear-client.ts` |
| `linear.teamStates` | ✅ | `renderer/src/runtime/runtime-linear-client.ts` |
| `linear.testConnection` | ✅ | `renderer/src/runtime/runtime-linear-client.ts` |
| `linear.updateIssue` | ✅ | `renderer/src/runtime/runtime-linear-client.ts` |

### `nativeChat.*`

| Method | Backend? | Called from |
|---|---|---|
| `nativeChat.readSession` | ✅ | `renderer/src/components/native-chat/native-chat-session-transport.ts` |

### `orchestration.*`

| Method | Backend? | Called from |
|---|---|---|
| `orchestration.dispatchShow` | ✅ | `renderer/src/components/terminal-pane/terminal-orchestration-task-links.ts` |

### `preflight.*`

| Method | Backend? | Called from |
|---|---|---|
| `preflight.check` | ✅ | `renderer/src/store/slices/preflight.ts` |

### `profile.*`

| Method | Backend? | Called from |
|---|---|---|
| `profile.getResolved` | ✅ | `renderer/src/context/WorkspaceContext.tsx`<br>`renderer/src/hooks/useProfile.ts` |
| `profile.getUserProfile` | ✅ | `renderer/src/hooks/useProfile.ts` |
| `profile.listDepts` | ✅ | `renderer/src/components/profile/CompanyProfileAdmin.tsx`<br>`renderer/src/components/profile/DeptProfileAdmin.tsx` |
| `profile.updateCompany` | ✅ | `renderer/src/hooks/useProfile.ts` |
| `profile.updateDept` | ✅ | `renderer/src/hooks/useProfile.ts` |
| `profile.updateUser` | ✅ | `renderer/src/hooks/useProfile.ts` |

### `project.*`

| Method | Backend? | Called from |
|---|---|---|
| `project.create` | ✅ | `renderer/src/components/project/CreateProjectDialog.tsx` |
| `project.get` | ✅ | `renderer/src/context/WorkspaceContext.tsx` |
| `project.getMembers` | ✅ | `renderer/src/components/project/MemberManager.tsx` |
| `project.list` | ✅ | `renderer/src/components/project/ProjectSwitcher.tsx` |
| `project.removeMember` | ✅ | `renderer/src/components/project/MemberManager.tsx` |
| `project.update` | ✅ | `renderer/src/store/slices/repos.ts` |
| `project.updateMemberRole` | ✅ | `renderer/src/components/project/MemberManager.tsx` |

### `projectGroup.*`

| Method | Backend? | Called from |
|---|---|---|
| `projectGroup.create` | ✅ | `renderer/src/store/slices/repos.ts` |
| `projectGroup.delete` | ✅ | `renderer/src/store/slices/repos.ts` |
| `projectGroup.importNested` | ✅ | `renderer/src/store/slices/repos.ts` |
| `projectGroup.list` | ✅ | `renderer/src/store/slices/repos.ts` |
| `projectGroup.moveProject` | ✅ | `renderer/src/store/slices/repos.ts` |
| `projectGroup.scanNested` | ✅ | `renderer/src/store/slices/repos.ts` |
| `projectGroup.update` | ✅ | `renderer/src/store/slices/repos.ts` |

### `projectHostSetup.*`

| Method | Backend? | Called from |
|---|---|---|
| `projectHostSetup.create` | ✅ | `renderer/src/store/slices/repos.ts` |
| `projectHostSetup.delete` | ✅ | `renderer/src/store/slices/repos.ts` |
| `projectHostSetup.list` | ✅ | `renderer/src/store/slices/repos.ts` |
| `projectHostSetup.setupExistingFolder` | ✅ | `renderer/src/store/slices/repos.ts` |
| `projectHostSetup.update` | ✅ | `renderer/src/store/slices/repos.ts` |

### `repo.*`

| Method | Backend? | Called from |
|---|---|---|
| `repo.add` | ✅ | `renderer/src/store/slices/repos.ts` |
| `repo.baseRefDefault` | ✅ | `renderer/src/runtime/runtime-repo-client.ts` |
| `repo.clone` | ✅ | `renderer/src/components/onboarding/use-onboarding-flow.ts`<br>`renderer/src/components/sidebar/useAddRepoCloneFlow.ts`<br>`renderer/src/store/slices/repos.ts` |
| `repo.create` | ✅ | `renderer/src/components/sidebar/useCreateRepo.ts` |
| `repo.hooksCheck` | ✅ | `renderer/src/runtime/runtime-hooks-client.ts` |
| `repo.issueCommandRead` | ✅ | `renderer/src/runtime/runtime-hooks-client.ts` |
| `repo.issueCommandWrite` | ✅ | `renderer/src/runtime/runtime-hooks-client.ts` |
| `repo.list` | ✅ | `renderer/src/store/slices/repos.ts` |
| `repo.reorder` | ✅ | `renderer/src/store/slices/repos.ts` |
| `repo.rm` | ✅ | `renderer/src/store/slices/repos.ts` |
| `repo.searchRefs` | ✅ | `renderer/src/runtime/runtime-repo-client.ts` |
| `repo.setupScriptImports` | ✅ | `renderer/src/runtime/runtime-hooks-client.ts` |
| `repo.update` | ✅ | `renderer/src/store/slices/github.ts`<br>`renderer/src/store/slices/repos.ts` |

### `ssh.*`

| Method | Backend? | Called from |
|---|---|---|
| `ssh.connect` | ✅ | `renderer/src/runtime/runtime-environment-ssh-state.ts`<br>`renderer/src/runtime/runtime-ssh-client.ts` |
| `ssh.getState` | ✅ | `renderer/src/runtime/runtime-environment-ssh-state.ts`<br>`renderer/src/runtime/runtime-ssh-client.ts` |
| `ssh.getUserAccount` | ✅ | `renderer/src/hooks/useSshUserAccount.ts` — real, sourced from the SSH target's actual configured username (there is no separate "Linux account provisioning" concept on the backend; see backend `runtime/rpc/methods/ssh.ts`) |
| `ssh.listTargets` | ✅ | `renderer/src/runtime/runtime-environment-ssh-state.ts`<br>`renderer/src/runtime/runtime-ssh-client.ts` |

### `status.*`

| Method | Backend? | Called from |
|---|---|---|
| `status.get` | ✅ | `renderer/src/components/browser-pane/browser-pane-remote.tsx`<br>`renderer/src/lib/windows-terminal-capability-read.ts` |

### `task.*`

| Method | Backend? | Called from |
|---|---|---|
| `task.aiApply` | ✅ | `renderer/src/hooks/useTask.ts` |
| `task.aiDecompose` | ✅ | `renderer/src/hooks/useTask.ts` |
| `task.delete` | ✅ | `renderer/src/hooks/useTask.ts` |
| `task.execute` | ✅ | `renderer/src/components/task/TaskDetail.tsx`<br>`renderer/src/components/task/TaskPromptEditor.tsx` |
| `task.getDependencies` | ✅ | `renderer/src/components/task/TaskDetail.tsx` |
| `task.list` | ✅ | `renderer/src/hooks/useTasks.ts` |
| `task.update` | ✅ | `renderer/src/hooks/useTask.ts` |

### `team.*`

| Method | Backend? | Called from |
|---|---|---|
| `team.addMember` | ✅ | `renderer/src/components/admin/TeamAdmin.tsx` |
| `team.create` | ✅ | `renderer/src/components/admin/TeamAdmin.tsx` |
| `team.list` | ✅ | `renderer/src/components/admin/TeamAdmin.tsx` |
| `team.listMembers` | ✅ | `renderer/src/components/admin/TeamAdmin.tsx` |
| `team.removeMember` | ✅ | `renderer/src/components/admin/TeamAdmin.tsx` |

### `terminal.*`

| Method | Backend? | Called from |
|---|---|---|
| `terminal.agentStatus` | ✅ | `renderer/src/lib/active-agent-terminal-send-readiness.ts` |
| `terminal.close` | ✅ | `renderer/src/lib/launch-agent-background-session.ts` |
| `terminal.create` | ✅ | `renderer/src/lib/launch-agent-background-session.ts` |
| `terminal.focus` | ✅ | `renderer/src/components/terminal-pane/terminal-handle-links.ts`<br>`renderer/src/components/terminal-pane/terminal-orchestration-task-links.ts` |
| `terminal.inspectProcess` | ✅ | `renderer/src/runtime/runtime-terminal-inspection.ts` |
| `terminal.isRunningAgent` | ✅ | `renderer/src/lib/active-agent-note-target.ts`<br>`renderer/src/lib/active-agent-terminal-send-readiness.ts` |
| `terminal.list` | ✅ | `renderer/src/lib/active-agent-note-target.ts` |
| `terminal.send` | ✅ | `renderer/src/lib/active-agent-note-send.ts`<br>`renderer/src/runtime/runtime-terminal-inspection.ts` |
| `terminal.stop` | ✅ | `renderer/src/store/slices/repos.ts` |
| `terminal.wait` | ✅ | `renderer/src/lib/active-agent-note-send.ts`<br>`renderer/src/lib/automation-session-observer.ts`<br>`renderer/src/lib/launch-agent-background-session.ts` |

### `workflow.*`

| Method | Backend? | Called from |
|---|---|---|
| `workflow.cancel` | ✅ | `renderer/src/hooks/useWorkflowExecution.ts` |
| `workflow.execute` | ✅ | `renderer/src/hooks/useWorkflow.ts` |
| `workflow.template.create` | ✅ | `renderer/src/hooks/useWorkflow.ts` |
| `workflow.template.update` | ✅ | `renderer/src/hooks/useWorkflow.ts` |

### `workspace.*`

| Method | Backend? | Called from |
|---|---|---|
| `workspace.refreshFileTree` | ✅ | `renderer/src/context/WorkspaceContext.tsx` |

### `workspacePorts.*`

| Method | Backend? | Called from |
|---|---|---|
| `workspacePorts.kill` | ✅ | `renderer/src/lib/workspace-port-actions.ts` |
| `workspacePorts.scan` | ✅ | `renderer/src/lib/workspace-port-actions.ts` |

### `worktree.*`

| Method | Backend? | Called from |
|---|---|---|
| `worktree.detectedList` | ✅ | `renderer/src/store/slices/worktrees.ts` |
| `worktree.forceDeleteBranch` | ✅ | `renderer/src/store/slices/worktrees.ts` |
| `worktree.list` | ✅ | `renderer/src/store/slices/worktrees.ts` |
| `worktree.prefetchCreateBase` | ✅ | `renderer/src/store/slices/worktrees.ts` |
| `worktree.resolveMrBase` | ✅ | `renderer/src/hooks/useComposerState.ts`<br>`renderer/src/store/slices/worktrees.ts` |
| `worktree.resolvePrBase` | ✅ | `renderer/src/lib/github-pr-start-point.ts`<br>`renderer/src/store/slices/worktrees.ts` |
| `worktree.rm` | ✅ | `renderer/src/store/slices/worktrees.ts` |
| `worktree.set` | ✅ | `renderer/src/store/slices/diffComments.ts`<br>`renderer/src/store/slices/worktrees.ts` |

---

## Resolved gaps (2026-08-15)

All 9 gaps this doc originally listed (formerly in a separate
`gaps-and-mismatches.md`, now removed) are fixed on both frontend and backend:

| Gap | Fix |
|---|---|
| `projects.listMembers/removeMember/updateMemberRole` | Renamed to the real `project.getMembers/removeMember/updateMemberRole` in `MemberManager.tsx`. |
| `annotation.list`/`annotation.create` | Built for real: `orca_annotations` table (migration `0018_annotations.ts`), `backend/src/main/runtime/rpc/methods/annotation.ts`, `backend/src/main/code-review/annotation-store.ts`. |
| `git.exec` (raw numstat passthrough) | Removed — `git.status` already returns per-file line-change stats; `renderer/src/hooks/use-code-review.ts` now merges those into the same shape the old numstat parse produced. No raw-exec RPC was added (deliberately — see the namespace table's `git` entry for why). |
| `ssh.getUserAccount` | Built — returns the SSH target's real configured username; "Linux account provisioning" was never a real backend concept (confirmed via code search; the only related helper, `resolveSshTargetForUser`, is dead code with zero callers), so the method reflects that honestly instead of fabricating a provisioning flow. |
| `fleet.health.checkAll` | Built — thin batch wrapper over the existing `fleet-health-store`/`fleet-status-service` primitives (deliberately not routed through `getFleetStatus()`, whose `fleetId ?? targetId` keying could silently mismatch the raw IDs the frontend sends). |
| `workflow.template.update` | Built in `TemplateResolver.ts`/`workflow-rpc-handler.ts` — ownership-checked, bumps `version`. Also fixed the frontend call site, which was flat-spreading params instead of nesting steps under `definition` (BUG-FE-RPC-006). Note: `workflow.template.create`'s call site has the same flat-spread pattern and is likely equally mismatched — not fixed in this pass, flagged for follow-up. |

See also: the Electron IPC → RPC migration described in
[`ipc-surface.md`](./ipc-surface.md), done alongside these fixes — several
namespaces (`gh`, `gl`, `repos`, `worktrees`, `fs`, `ssh`, `credentials`)
that used to route through `window.api.*` on the web build now call these RPC
methods directly via `renderer/src/runtime/runtime-*-client.ts` wrappers.

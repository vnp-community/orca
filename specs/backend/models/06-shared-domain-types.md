# Shared Domain Types — `backend/src/shared/*.ts`

Danh mục các type nghiệp vụ định nghĩa trong `shared/types.ts` (3908 dòng), `shared/ssh-types.ts` (238 dòng),
`shared/automations-types.ts`, `shared/ai-provider-types.ts`, `shared/team-types.ts`. Đây là các type dùng
xuyên suốt main/preload/renderer — phần lớn là "cell" bên trong `PersistedState` (desktop mode, xem
[03](./03-electron-desktop-store.md)); 4 type gốc (`Project`, `Repo`, `SshTarget`, `GlobalSettings`) còn được
`IStateRepository` dùng lại ở server mode (xem [01](./01-storage-architecture.md) §6).

> ⚠️ Lưu ý đặt tên trùng: `Project` (type ở đây, `types.ts:108`) **khác** `orca_v5_projects`
> (bảng SQL, [02](./02-sql-schema-catalog.md) nhóm F) **khác** `orca_projects` (bảng SQL, nhóm C, lưu
> `Project` này dưới dạng JSON blob khi server mode dùng SQL backend). 3 tầng nghĩa khác nhau của "Project"
> cùng tồn tại — xác định đúng ngữ cảnh trước khi map field.

## 1. Project, Repo & SSH

**`Project`** (`types.ts:108`) — `{id, displayName, badgeColor, repoIcon?, kind?, providerIdentity?,
gitRemoteIdentity?, localWindowsRuntimePreference?, sourceRepoIds: string[], createdAt, updatedAt}`. Identity
"project" độc lập host, gom nhiều `Repo` checkout (local/SSH/runtime) dưới 1 badge/tên. Persist ở
`PersistedState.projects`.

**`ProjectHostSetup`** (`types.ts:139`) — cách 1 `Project` được materialize trên 1 execution host cụ thể
(clone/import/provision): `{id, projectId, hostId, repoId, path, displayName, kind?, connectionId?,
executionHostId?, worktreeBasePath?, hookSettings?, gitUsername?, setupState, setupMethod,
sourceControlAi?, createdAt, updatedAt}`. `setupState` = ready|not-set-up|setting-up|error|unsupported.
`setupMethod` = legacy-repo|imported-existing-folder|cloned|provisioned. Persist ở
`PersistedState.projectHostSetups`.

**`Repo`** (`types.ts:232`) — entity checkout git cốt lõi: `{id, path, displayName, badgeColor, repoIcon?,
upstream?, addedAt, kind?, gitUsername?, worktreeBaseRef?, worktreeBasePath?, hookSettings?, connectionId?
(SSH target id, null=local), executionHostId?: 'local'|ssh:<id>|runtime:<id>|devServer:<id>|null,
issueSourcePreference?, forkSyncMode?, gitRemoteIdentity?, symlinkPaths?, projectGroupId?, projectGroupOrder?,
sourceControlAi?, projectHostSetupMethod?, devServerId?}`. Neo cho worktree/hook/project-group membership/định
tuyến execution-host. Persist ở `PersistedState.repos`.

**`SshTarget`** (`ssh-types.ts:10`) — `{id, label, owner?, configHost?, host, port, username, identityFile?,
identityAgent?, identitiesOnly?, proxyCommand?, jumpHost?, source: 'ssh-config'|'manual',
relayGracePeriodSeconds?, lastRequiredPassphrase?, portForwards?: SavedPortForward[],
systemSshConnectionReuse?, project?, team?, environment?: development|staging|production, tags?: string[],
repos?: [{path,name,url?,branch?}], fleetConfigSource?, fleetId?}`. Không lưu key material — chỉ path.
Trường `project`/`team`/`environment`/`tags` biến `SshTarget` thành 1 record "fleet inventory" nhẹ (khác
`Team`/`Project` thật — chỉ là string tag). Persist ở `PersistedState.sshTargets`.

Phụ trợ SSH: `SavedPortForward` (`{localPort, remoteHost, remotePort, label?}`, lồng trong
`SshTarget.portForwards` — **đây là cơ chế thật lưu port-forward auto-restore**, khác bảng SQL
`orca_port_forwards` dormant), `SshRepoReadoption`, `SshConnectionStatus/State` (state máy kết nối, không
persist), `SshRemotePtyLease` (`{targetId, ptyId, worktreeId?, tabId?, leafId?, state, createdAt, updatedAt,
lastAttachedAt?, lastDetachedAt?}` — theo dõi attach/detach PTY từ xa để phục hồi sau reconnect, persist ở
`PersistedState.sshRemotePtyLeases`), `SshTargetGroup` (group theo `SshTarget.project` cho UI fleet).

## 2. Workspace / Project-Group / Worktree

**`ProjectGroup`** (`types.ts:294`) — nhóm sidebar dạng thư mục (cây qua `parentGroupId`): `{id, name,
parentPath, connectionId?, executionHostId?, parentGroupId, createdFrom, tabOrder, isCollapsed, color,
createdAt, updatedAt}`.

**`FolderWorkspace`** (`types.ts:317`) — workspace "thư mục" không phải git, sibling của `Worktree` bên trong
1 `ProjectGroup`: `{id, projectGroupId, name, folderPath, connectionId?, linkedTask, comment, isArchived,
isUnread, isPinned, sortOrder, workspaceStatus?, createdWithAgent?, lastActivityAt, createdAt, updatedAt}`.
`WorkspaceScope`/`WorkspaceKey` = con trỏ phân biệt `worktree:<id>` | `folder:<id>` dùng xuyên suốt persistence.

**`Worktree`** (`types.ts:460`, `& GitWorktreeInfo`) — entity trung tâm người dùng thao tác: `{id
(=${repoId}::${path}), instanceId?, repoId, projectId?, hostId?, projectHostSetupId?, displayName, comment,
linkedIssue/PR/LinearIssue/GitLabMR/GitLabIssue/BitbucketPR/AzureDevOpsPR/GiteaPR, isArchived, isUnread,
isPinned, sortOrder, lastActivityAt, createdWithAgent?, sparseDirectories?, sparseBaseRef?, sparsePresetId?,
baseRef?, pushTarget?, priorWorktreeIds?, workspaceStatus?, diffComments?: DiffComment[],
mobileDiffReview?, automationProvenance?}`. `WorktreeMeta` (`types.ts:573`) là bản persisted-subset
(bỏ field runtime-only, thêm `preserveBranchOnDelete`, `orcaCreatedAt`, `orcaCreationSource`,
`orcaCreationWorkspaceLayout`), lưu ở `PersistedState.worktreeMeta: Record<id, WorktreeMeta>`.

Phụ trợ: `DetectedWorktree` (worktree phát hiện trên đĩa chưa adopt), `AutomationWorkspaceProvenance` (đánh
dấu worktree do automation tạo — `automationId/RunId`, `executionTargetType`, `projectId`), `GitPushTarget`,
`WorktreeLineageCapture`/`WorktreeLineage`/`WorkspaceLineage` (quan hệ cha-con giữa các nhánh worktree, persist
ở `worktreeLineageById`/`workspaceLineageByChildKey`), `DiffComment`, `MobileDiffReviewState`.

## 3. Tab, Terminal & Browser (UI/session)

**`Tab`** (`types.ts:789`) / **`TabGroup`** (`types.ts:811`) — abstraction tab hợp nhất (terminal/editor/
diff/browser/simulator...) + split-pane group (`TabGroupLayoutNode` cây leaf/split đệ quy).

**`TerminalTab`** (`types.ts:826`, legacy pre-unified-tabs) — vẫn là shape persisted thật trong
`WorkspaceSessionState.tabsByWorktree`: `{id, ptyId, worktreeId, title, customTitle, color, isPinned?,
sortOrder, createdAt, shellOverride?, startupCwd?, launchAgent?: TuiAgent, ...}`.

**`TerminalLayoutSnapshot`** (`types.ts:1012`) — layout split-pane + scrollback cho 1 terminal tab:
`{root: TerminalPaneLayoutNode|null, activeLeafId, expandedLeafId, ptyIdsByLeafId?, buffersByLeafId?,
scrollbackRefsByLeafId?, titlesByLeafId?}`. **Đây là cơ chế thật lưu scrollback terminal ở desktop mode**
(khác bảng SQL `orca_terminal_sessions` dormant — xem [02](./02-sql-schema-catalog.md) nhóm J).

**`BrowserWorkspace`** (`types.ts:926`) — tab browser trong app: `{id, worktreeId, label?, sessionProfileId?,
sessionPartition?, activePageId?, pageIds?, url, title, loading, canGoBack, canGoForward, loadError,
createdAt}`, nested `BrowserPage[]`. `BrowserSessionProfile` — identity partition/cookie-jar Electron
(`scope: default|isolated|imported`).

**`PersistedOpenFile`** (`types.ts:1031`) — tab editor phục hồi sau restart.

**`WorkspaceSessionState`** (`types.ts:1050`) — aggregate session lớn nhất: map theo worktree/tab của
`tabsByWorktree`, `terminalLayoutsByTabId`, `openFilesByWorktree`, `browserTabsByWorktree`, `unifiedTabs`,
`tabGroups`, `tabGroupLayouts`, `sleepingAgentSessionsByPaneKey`, v.v. — snapshot "cái gì đang mở" theo từng
execution host, phục hồi lúc app restart. Persist per-host ở `PersistedState.workspaceSessionsByHostId`
(+ `workspaceSession` legacy cho host local).

## 4. Team / Org

**`Team`** (`team-types.ts:15`) — `{id, name, createdAt, updatedAt}`. **`TeamMember`** (`team-types.ts:28`) —
`{teamId, userId, role, priority, addedAt}` — `priority` là tiebreaker cascade-merge khi user thuộc nhiều
team. Đây là type client-side tương ứng bảng SQL `orca_teams`/`orca_team_members` ([02](./02-sql-schema-catalog.md)
nhóm I/L). Lưu ý: `SshTarget.team` (string tag) **không** tham chiếu `Team` entity này bằng id — 2 khái niệm
"team" độc lập trong hệ thống.

## 5. AI Providers / Managed Accounts

**`AIProviderAccount`** (`ai-provider-types.ts:39`) — mirror client của `orca_ai_provider_accounts`:
`{id, devServerId, provider, scope, scopeRefId?, label, model?, baseUrl?, status, lastHealthCheck?,
rotationGraceUntil?, quotaLimitDay, quotaUsedToday?, createdBy, createdAt, updatedAt}`. Xác nhận rõ ràng
trong comment: credential **không bao giờ** lưu trên Orca Server, chỉ trên Dev Server qua relay.
`CredentialWriteRequest` — payload gửi relay (xem [05](./05-credential-secret-stores.md) §4).

**Managed CLI accounts** (trong `GlobalSettings`, không phải SQL) — `CodexManagedAccount`/
`ClaudeManagedAccount` (+ Summary variant): `{id, email, managedHomePath, providerAccountId?,
workspaceLabel?, createdAt, updatedAt, lastAuthenticatedAt}` — cho phép 1 install Orca luân phiên nhiều tài
khoản CLI đã đăng nhập cục bộ (xoay khi rate-limit). `CodexRateLimitAccountsState`/
`ClaudeRateLimitAccountsState` theo dõi account đang active theo runtime (host/WSL distro).

**`TuiAgent`** (`types.ts:2362`) — union string literal ~35 giá trị (claude, codex, gemini, aider, goose, amp,
cursor, copilot, grok, devin...) — discriminator "agent CLI nào", dùng lại ở `Worktree.createdWithAgent`,
`Automation.agentId`, `TerminalTab.launchAgent`, `FolderWorkspace.createdWithAgent`.

## 6. Workflow / Automation

**`Automation`** (`automations-types.ts:90`) — tương đương "WorkflowTemplate" ở desktop mode: `{id, name,
prompt, precheck, agentId: TuiAgent, runContext?, sourceContext?, projectId (deprecated), executionTargetType:
local|ssh, executionTargetId, schedulerOwner, workspaceMode: existing|new_per_run, workspaceId, baseBranch,
reuseSession, timezone, rrule, dtstart, enabled, nextRunAt, lastRunAt?, missedRunPolicy, createdAt, updatedAt}`
— job định kỳ theo RRULE dispatch agent vào workspace. Persist ở `PersistedState.automations`
(**không phải** bảng SQL `automations` dormant, xem [02](./02-sql-schema-catalog.md) nhóm B).

**`AutomationRun`** (`automations-types.ts:128`) — tương đương "WorkflowExecution": `{id, automationId,
title, scheduledFor, status, trigger: scheduled|manual, workspaceId, sessionKind: terminal, chatSessionId,
terminalSessionId/PaneKey/PtyId, outputSnapshot, precheckResult, usage, error, startedAt, dispatchedAt,
createdAt}`. Persist ở `PersistedState.automationRuns`, dọn định kỳ qua `automation-run-retention.ts`.

**External automation bridges** (Hermes/OpenClaw) — `ExternalAutomationManager`/`Job`/`Run` — mirror
job/run của automation runner bên thứ 3 tích hợp ngoài.

## 7. Task (work-item / issue-tracker mirror — KHÔNG phải Task Graph SQL)

> Lưu ý: đây là **mirror client-side của PR/Issue/Linear-issue** hiển thị ở view "Tasks" cross-repo — khác
> hoàn toàn `orca_tasks` (Task Graph SQL, [02](./02-sql-schema-catalog.md) nhóm I) và `TaskRow`
> (OrchestrationDb, [04](./04-orchestration-db.md)). 3 khái niệm "task" khác nhau.

`PRInfo` (`types.ts:1141`), `IssueInfo` (`types.ts:1421`) — mirror PR/issue GitHub, cache ở
`PersistedState.githubCache`. `GitHubWorkItem` (`types.ts:1456`) — shape hợp nhất PR/issue cho danh sách Task
cross-repo. `LinearIssue`/`LinearTeam`/`LinearWorkspace`/`LinearProjectSummary` (`types.ts:1598+`) — tương
đương phía Linear. Phụ trợ: `PRComment`, `PRCheckDetail` (cây CI check), `GitHubPRFile`, GitLab/GitHub
rate-limit type re-export.

## 8. `GlobalSettings` (`types.ts:2497`, ~230 field) — tổng hợp preference toàn app

Quá lớn để liệt kê từng field — nhóm theo khu vực (marker comment trong source):

| Nhóm | Field tiêu biểu |
|---|---|
| Fleet/dev-server | `hostSettingOverrides?`, `activeDevServerId?`, `fleetAlertWebhookUrl?`, `fleetHealthPingIntervalMs?` |
| Web Push | `vapidKeys?`, `webPushSubscriptions?` |
| Workspace/branch | `nestWorkspaces`, `workspaceDir?`, `refreshLocalBaseRefOnWorktreeCreate`, `branchPrefix` |
| Theme/UI | `theme`, `leftSidebarAppearanceMode`, `uiLanguage`, `appIcon` |
| Editor | `editorAutoSave`, `editorMinimapEnabled`, `editorWordWrap?` |
| Terminal (cụm lớn nhất, ~50 field) | font/cursor/opacity/scrollback/GPU, Windows/WSL shell, quick commands |
| Proxy/network | `httpProxyUrl?` (mã hoá), `httpProxyBypassRules?`, `openLinksInApp` |
| Source control | `sourceControlViewMode`, `sourceControlAi?`, `prBotAuthorOverrides` |
| Notifications | `notifications: NotificationSettings` |
| AI managed accounts | `codexManagedAccounts`, `claudeManagedAccounts` (xem §5) |
| Agent/TUI | `defaultTuiAgent`, `disabledTuiAgents`, `agentCmdOverrides` |
| Task/provider view | `defaultTaskViewPreset`, `visibleTaskProviders`, `opencodeSessionCookie` (mã hoá) |
| Confirm/UX guard | `skipDeleteWorktreeConfirm`, `skipDeleteAutomationConfirm` |
| Mobile/emulator | `mobileEmulatorEnabled?`, `androidSdkPath?` |
| Experimental flags | `experimentalMobile`, `experimentalPet`, `experimentalEphemeralVms`, ... |
| Integration state | `githubProjects?`, `gitlabProjects?`, `commitMessageAi?`, `telemetry?`, `voice?`, `keybindings?` |

Persist nguyên khối ở `PersistedState.settings`; đây là 1 trong 4 type mà `IStateRepository` (server mode)
cũng dùng lại nguyên xi.

## 9. `PersistedUIState` (`types.ts:3315`, ~90 field) & `PersistedState` root

`PersistedUIState` — chrome UI thuần tuý (không phải business logic): view/lựa chọn hiện tại, độ rộng sidebar,
sort/group/filter, `statusBarItems`, window bounds, onboarding nudge dismissal, `taskResumeState?`, easter-egg
pet state. Hầu hết field đi kèm 1 boolean guard `*Migrated?`/`*Defaulted?` cho migration 1 lần.

`PersistedState` — root object toàn bộ file `orca-data.json`, xem shape đầy đủ ở
[03-electron-desktop-store.md](./03-electron-desktop-store.md) §2 — mọi entity ở trên cuối cùng đều treo vào
đây.

## 10. Misc

`OrcaHooks`/`RepoHookSettings` (`types.ts:1984/2018`) — script lifecycle theo repo (`setup`/`archive`,
`environmentRecipes: OrcaVmRecipe[]` — VM lifecycle create/suspend/resume/destroy cho on-demand environment).
`ChangelogRelease`, `UpdateCheckOptions`/`UpdateStatus` — self-update app. `OnboardingState`/
`OnboardingChecklistState` — checklist first-run theo fleet server. `WebPushSubscription`
(`{id, endpoint, keys:{auth,p256dh}, addedAt, userAgent?}`) — nested cả trong `GlobalSettings` lẫn
`PersistedState` (xem lưu ý dormant `orca_push_subscriptions` ở [02](./02-sql-schema-catalog.md) nhóm K).
`MemorySnapshot`/`AppMemory`/`WorktreeMemory` — snapshot resource-metrics runtime, **không persist**.
`DirEntry`/`MarkdownDocument`/`FsChangeEvent`, `GitBranchCompareSummary`/`GitDiffResult`,
`SearchMatch`/`SearchResult` — DTO filesystem/git/search, không phải entity lưu trữ.

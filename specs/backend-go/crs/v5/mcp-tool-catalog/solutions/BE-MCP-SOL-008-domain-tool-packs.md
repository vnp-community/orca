# BE-MCP-SOL-008: Domain tool packs 1–4 theo tên channel thật

> **🔲 Designed — chưa implement.** Phụ thuộc BE-MCP-SOL-007 (ToolSpec/Executor/parity). Pack 2–4 **không bật ở production** trước BE-MCP-SOL-012/013 (policy + approval).

**CR:** [CR-MCP-008](../../../../../../docs/crs/v5/mcp-tool-catalog/CR-MCP-008-domain-tool-packs.md)
**Service:** `api-gateway` (`internal/adapter/mcpserver/tools/pack*.go`)
**TDD tham chiếu:** `api-gateway.md` §6 (T1, T2); AGENTS.md (SSH, Git ≥ 2.25, GitLab/provider-neutral, `gh` rate limit)
**CONTRACT items hiện thực:** trường `pack`, `namespace`, `risk`, `requiredScope`, `annotations` của `McpToolView` (§1) cho kênh `mcp.admin.tool.list` (BE-007 §2.H).

---

## 1. Trạng thái hiện tại (re-verify)

Danh sách channel dưới đây lấy từ **registry chạy thật** (test kiểm kê của BE-007 §2.F: 449 channel, 57 namespace), không phải grep literal. Đối chiếu CR:

| Khẳng định của CR | Kết quả | Lệch? |
|---|---|---|
| Đợt 1 có `github/gitlab/hostedReview` "đọc PR/issue/checks" | Có `github.issues/listWorkItems/prForBranch/issueComments/project.*`, `gitlab.issues/listMRs/workItemDetails`, `hostedReview.forBranch/getCreationEligibility/suggestReviewers`. **Không có channel đọc "checks/CI"** (khác `store/slices/github-checks.ts` ở FE — FE tự gọi nguồn khác) | **Lệch:** "checks" chưa có channel ⇒ không có tool `*_checks` ở v1; ghi vào backlog (cần channel mới ở `scm-integration-service`) |
| Đợt 2 "tạo/sửa workflow/automation ở trạng thái *draft*" | `workflow.template.create/update/clone`, `automation.create/update` tồn tại; **không xác minh được khái niệm `draft`** trong view/DTO (grep `channels_workflow.go`, `channels_automation_task.go`) | **(chưa xác minh)** — nếu không có `draft`, tool tạo automation phải bắt buộc `enabled:false` trong `Args()` |
| Đợt 4 "`git` force-push/reset" | **Không tồn tại** channel `git.reset`; `git.push` có comment "`publish`/`forceWithLease` NOT honored" (`channels_git.go`, BUG-021) | **Lệch:** không có tool force-push/reset; thêm vào backlog thay vì giả định |
| Đợt 4 "xoá workflow" | **Không có** `workflow.delete`/`template.delete`; chỉ `workflow.cancel`, `automation.delete`, `task.delete` | **Lệch:** "xoá workflow" ⇒ `automation_delete` |
| Đợt 4 "merge PR" | `github.mergePR`, `github.setPRAutoMerge`, `worktree.merge` có thật; GitLab **không có** `gitlab.mergeMR` | Lệch nhỏ: merge chỉ GitHub; thiết kế tên chung `hostedReview_merge` chưa khả thi (backlog, AGENTS.md "provider-neutral") |
| Loại trừ `admin force-revoke/tạo user` | Có thật: `admin.createUser/forceRevokeSession/forceRevokeAllSessions/updateUserRole/…` | Không |
| Loại trừ `devServerAgentTokens` | Channel tên thật là `devServer.agentTokens.create/list/revoke` | Lệch tên (đúng ý) |
| `files.*` "đọc/grep/glob, giới hạn kích thước" | Có `files.readPreview(maxBytes)`, `readChunk`, `search(maxResults)`, `listAll(maxResults)`, `readDir`, `stat`. `files.read` trả cả file (`bytes content` ⇒ base64 qua `encoding/json`), **không giới hạn ở gateway**; kiểm đường dẫn chỉ là `filepath.Clean`+tiền tố (git-gateway `localfs.resolve`), **không `EvalSymlinks`** | **Lệch quan trọng:** dùng `readPreview`/`readChunk`, loại trừ `files.read`; symlink do BE-010 xử lý |

## Quyết định khác/thêm so với CR gốc

1. Mở rộng cấu trúc: pack không chỉ là danh sách; mỗi `ToolSpec` mang `Pack`, và `config` `MCP_TOOL_PACKS_ENABLED` (mặc định `1`; env `MCP_*` theo T8) quyết định pack nào được đăng ký vào `Catalog`. Admin tenant bật từng pack/namespace qua policy (BE-012) — không qua cờ build.
2. Quy tắc phân loại rủi ro: **mạng ra ngoài hoặc spawn tiến trình = `exec`** (kể cả `git_fetch`, `git_pull`); chỉ đổi trạng thái cục bộ có `git reflog`/hoàn tác = `write_reversible`; mất dữ liệu không thể phục hồi, đổi quyền/nhân sự, merge = `destructive|admin`.
3. `files.write/createFile/createDir/rename/copy` **không** đưa vào v1 (CR không nêu; agent client đã có công cụ sửa file riêng; ghi file qua MCP mở rộng bề mặt ghi). Ghi trong `excluded_channels.yaml` để xét lại.
4. Tên tool theo D7 (`.`→`_`, giữ camelCase của phần sau): `git.branch.create → git_branch_create`, `github.project.workItemDetailsBySlug → github_project_workItemDetailsBySlug` (56 ký tự ≤ 64; test bắt >64 ⇒ buộc `NameReason`).
5. Channel nhận định danh từ body (đã xác minh): `agent.start/resume/switchAccount` đọc `userId` từ args (`channels_agent.go`). `Args()` **luôn** ghi đè bằng `Identity.UserID` và schema đầu vào không có trường này. Các channel khác có thể có trường tương tự (chưa xác minh toàn bộ ~450) ⇒ test `TestInputSchemaHasNoIdentityFields` cấm `tenantId|userId|tenant_id|user_id` trong mọi `Input` schema.

## 2. Bảng pack theo namespace (tên channel THẬT, từ registry chạy)

Ký hiệu: **R** read, **W** write_reversible, **X** exec, **D** destructive, **A** admin. Cột "Loại trừ" ghi lý do (vào `excluded_channels.yaml`).

### Pack 1 — Khám phá & đọc (scope `orca:read`, `readOnlyHint:true`, mặc định bật)

| Namespace | Channel được phơi (tên tool = đổi `.`→`_`) | Ghi chú |
|---|---|---|
| project / projectGroup / orcaProjects / repo | `project.list`, `project.get`, `project.getMembers`, `projectGroup.list`, `orcaProjects.list`, `orcaProjects.getProjectData`, `repo.list`, `repo.getMembers`, `repo.baseRefDefault`, `repo.searchRefs` | `repo.*` có SSH `connectionId`/devServer trong kết quả ⇒ che host nội bộ (§4) |
| worktree | `worktree.list` (arg `projectId`), `worktree.detectedList`, `worktree.lineageList`, `worktree.checkDeleteSafety`, `worktree.compare` | `Path` tuyệt đối của host: giữ (cần cho agent) nhưng chỉ trong phạm vi tenant |
| git (đọc) | `git.status`, `git.diff`, `git.history`, `git.localBranches`, `git.branchCompare`, `git.branchDiff`, `git.commitCompare`, `git.commitDiff`, `git.checkIgnored`, `git.submoduleStatus`, `git.upstreamStatus`, `git.conflictOperation`, `git.remoteCommitUrl`, `git.remoteFileUrl` | Kết quả `git.diff` cắt 64 KiB + `truncated`; URL remote che token |
| task / annotation | `task.list`, `task.get`, `task.getSubtree`, `task.getDependencies`, `task.listComments`, `task.hasActiveExecutions`, `task.getSource`, `annotation.list`, `annotation.composeReviewPrompt` | `task.activity.subscribe` (stream) ⇒ dành cho BE-010 |
| github | `github.issues`, `github.listWorkItems`, `github.prForBranch`, `github.issueComments`, `github.repoSlug`, `github.rateLimit`, `github.project.listAccessible/listViews/viewTable/workItemDetailsBySlug/listLabelsBySlug/listIssueTypesBySlug/listAssignableUsersBySlug` | Gộp nhóm trong 1 lượt dựng catalog; cache 30s theo (tenant,tool,paramsHash) để tiết kiệm `gh` rate limit (AGENTS.md); mọi tool trả `rateLimit` còn lại nếu có |
| gitlab / hostedReview | `gitlab.issues`, `gitlab.listMRs`, `gitlab.workItemDetails`, `gitlab.rateLimit`, `hostedReview.forBranch`, `hostedReview.getCreationEligibility`, `hostedReview.suggestReviewers`, **`hostedReview_get`** (Composite mới: `provider`+`repo`+`number` → `github.project.workItemDetailsBySlug` / `gitlab.workItemDetails`; dùng bởi resource `orca://review/…` ở BE-010) | Tên chung `hostedReview_*` cho khái niệm provider-neutral; `provider` ∈ github/gitlab trong input |
| jira / linear | `jira.status/listProjects/listIssues/searchIssues/getIssue/issueComments/listIssueTypes/listTransitions/listPriorities/listAssignableUsers/listCreateFields/getProjectStatusOrder`; `linear.status/listTeams/listIssues/searchIssues/getIssue/issueComments/getProject/getCustomView/teamLabels/teamMembers/teamStates` | Nội dung ticket = **dữ liệu không tin cậy** (nhãn `untrusted` trong structuredContent) |
| workflow / automation | `workflow.listExecutions`, `workflow.getExecution`, `workflow.hasActiveExecutions`, `workflow.template.list`, `workflow.template.resolve`, `automation.list`, `automation.runs` | |
| files | `files.readPreview` (Composite → `files_read`), `files.readChunk`, `files.readDir`, `files.stat`, `files.search`, `files.listAll`, `files.listMarkdownDocuments` | `files_read` trả UTF-8 ≤ 64 KiB; nhị phân ⇒ metadata; áp deny-list file nhạy cảm của BE-010 §3 |
| aiProvider / infra | `aiProvider.list` (test quét: không có `credentialRef` thô), `devServer.list`, `devServer.listForUser`, `ephemeralVm.listRecipes/listRecipeCatalog/listRuntimes`, `terminal.list`, `agentSession.listActive`, `team.list`, `team.listMembers`, `auth.listTenantMemberDirectory`, `fleet.health.checkAll`, `connectivity.getSummary` | `team/auth.listTenantMemberDirectory` chứa PII: email bị che theo cấu hình tenant (`MCP_PII_MASK`) |

### Pack 2 — Ghi có thể hoàn tác (scope `orca:write`; cần BE-012/013 cho prod)

| Namespace | Channel | Cách hoàn tác (ghi trong mô tả tool) |
|---|---|---|
| task / annotation | `task.create`, `task.update`, `task.addComment`, `task.addEdge`, `task.createFromSource`, `task.aiDecompose`, `task.aiApply`, `task.recalculateProgress`, `annotation.create`, `annotation.update`, `annotation.markSent` | `task.update` đưa lại trạng thái cũ; `aiApply` tốn quota provider ⇒ `openWorld:true` |
| worktree | `worktree.create`, `worktree.createFromIssue`, `worktree.set`, `worktree.fanOut` (trần `MCP_FANOUT_MAX=5` trong `Args()`) | xoá bằng pack 4 |
| git (cục bộ) | `git.stage`, `git.unstage`, `git.bulkStage`, `git.bulkUnstage`, `git.commit`, `git.checkout`, `git.branch.create`, `git.stash.push`, `git.stash.pop`, `git.merge`, `git.rebaseFromBase`, `git.abortMerge`, `git.abortRebase`, `git.resolveConflict` | `reflog`; `git.merge`/`rebase` đi kèm `abort*` |
| scm | `github.project.addIssueCommentBySlug`, `…updateIssueCommentBySlug`, `…updateIssueBySlug`, `…updateIssueTypeBySlug`, `…updatePullRequestBySlug`, `…updateItemField`, `…clearItemField`, `github.updateIssue`, `github.updatePRTitle`, `github.requestPRReviewers`, `github.removePRReviewers`, `gitlab.resolveMRDiscussion`, `hostedReview.create`, `hostedReview.submit` | `hostedReview.create/submit` = `openWorld`, idempotent=false |
| jira / linear | `jira.addIssueComment/createIssue/updateIssue`, `linear.addIssueComment/createIssue/updateIssue/createProject` | đổi trạng thái issue = chuyển lại |
| workflow / automation / project | `workflow.template.create/update/clone`, `automation.create/update` (bắt buộc `enabled:false` nếu không có `draft`), `project.create`, `project.update`, `projectGroup.create/update/moveProject` | |

### Pack 3 — Thực thi (scope `orca:exec`; **mặc định `require_approval`**)

`terminal.*` và `agent.*` thành tool Composite (BE-009): `terminal_start/send/read/stop`, `terminal_wait`, `agent_start/resume/send/status/stop`; `workflow.execute`, `workflow.executeAdHocStep`, `workflow.pause/resume/cancel`, `workflow_run_status` (Kind=Channel, `NameReason`, → `workflow.getExecution`; `workflow_run` → `workflow.execute`), `automation.runNow`, `task.execute`, `annotation.sendToAgent`, `git.push`, `git.pull`, `git.fetch`, `git.forkSync`, `git.fastForward`, `worktree.prefetchCreateBase`, `repo.clone`, `ephemeralVm.provision/attachWorkspace/resumeWorkspace/suspendWorkspace/cancelProvision`, `workspacePorts.scan`, `workspacePorts.kill`, `git.generateCommitMessage`/`generatePullRequestFields` (gọi provider AI = `openWorld`).

### Pack 4 — Quản trị & phá huỷ (scope `orca:admin`; mặc định **tắt**, bật theo tenant + approval mỗi lần)

`worktree.rm`, `worktree.forceDeleteBranch`, `worktree.merge`, `git.branch.delete`, `git.discard`, `git.bulkDiscard`, `files.delete` (chưa phơi v1), `repo.rm`, `project.delete`, `automation.delete`, `task.delete`, `github.mergePR`, `github.setPRAutoMerge`, `ephemeralVm.cleanup`, `project.addMember/removeMember/updateMemberRole`, `repo.addMember/removeMember/updateMemberRole`, `team.create/addMember/removeMember`, `devServer.approve/reject/assignGroup/resolveAccessRequest`, đọc admin: `admin.listUsers`, `admin.listSessions`, `admin.queryAuditLog`, `admin.listPolicies`, `admin.getPolicy` (risk `admin`, `readOnly:true`).

### Loại trừ mặc định (vào `excluded_channels.yaml`)

| Pattern | Lý do |
|---|---|
| `credentials.*`, `aiProvider.create/update/delete/writeCredential/resolve/testConnection`, `devServer.agentTokens.*`, `github.startAuthLogin/revokeAuth`, `gitlab.startAuthLogin/revokeAuth`, `*.checkAuthStatus`, `auth.*` mint | secret/đăng nhập — `listedAsHardDenied: true` cho `credentials.*`, `devServer.agentTokens.*` |
| `admin.createUser/deactivateUser/reactivateUser/updateUserRole/forceRevokeSession/forceRevokeAllSessions/createPolicy/updatePolicy/deletePolicy` | leo thang đặc quyền / tự khoá — `listedAsHardDenied: true` |
| `mobile.*` (3), `clientState.*`, `workspaceSession.*`, `starNag.*` (10), `onboarding.*` (10), `telemetry.track`, `crashReports.*`, `emulator.*` (8), `browser.*` (19, gồm `browser.screencast` binary), `session.tabs.*`, `notifications.subscribe`, `runtime.clientEvents.subscribe`, `nativeChat.readSession`, `status.get`, `rateLimits.get`, `apiGateway.rateLimits.get`, `host.*`, `preflight.check`, `connection.teardown`, `ssh.*`, `sparsePresets.*`, `folderWorkspace.*`, `projectHostSetup.*`, `accounts.*`, `profile.*` | gắn với UI/thiết bị/cài đặt cá nhân hoặc không có ngữ nghĩa cho agent |
| `terminal.multiplex` (binary), `terminal.subscribe/unsubscribe/updateViewport/reattachSend/focus/resize/scrollback.*/agentStatus/isRunningAgent/inspectProcess/close`, `agent.subscribeStatus/kill`, `files.read`, `files.write*`, `files.createFile/createDir/createDirNoClobber/rename/copy/commitUpload/watch/unwatch/browseServerDir` | thay bằng Composite BE-009 hoặc chưa phơi v1; stream/binary không dùng được qua `tools/call` |
| `workspace.subscribe`, `task.activity.subscribe`, `git.pull.progress`, `git.push.progress`, `accounts.subscribe`, `mobile.statusSubscribe`, `workspacePorts.subscribe` | channel `stream`: dành cho BE-010 (resources subscribe) hoặc không phơi |

Ước lượng quy mô (chưa chốt, đếm lại bằng `TestChannelInventory`): pack 1 ≈ 120 channel, pack 2 ≈ 60, pack 3 ≈ 35, pack 4 ≈ 30, loại trừ ≈ 200. CR khuyến nghị 60–100 tool giá trị cao ⇒ `tools/list` chỉ trả tool trong pack được bật + policy; thêm tool meta `tool_search{query, namespace?}` (read, Composite) nếu danh sách > 100 (đo ở rollout, BE-015).

## 3. `Args()` — ba ví dụ đại diện

**(a) `git_status` (read)** — channel `git.status` đọc khoá `worktree`, chấp nhận tiền tố `id:`:

```go
type GitStatusInput struct {
	WorktreeID string `json:"worktree_id" jsonschema:"required,description=Worktree id from worktree_list"`
}
func gitStatusArgs(in json.RawMessage, _ wscompat.Identity) ([]json.RawMessage, error) {
	var v GitStatusInput; if err := decodeStrict(in, &v); err != nil { return nil, err }
	return one(map[string]any{"worktree": "id:" + v.WorktreeID}) // key thật là "worktree" (channels.go:673), KHÔNG phải worktreeId
}
```

**(b) `task_create` (write)** — `task.create` đọc `{title,parentId,projectId}`; `TenantId` do handler lấy từ `Identity`:

```go
type TaskCreateInput struct {
	Title     string `json:"title" jsonschema:"required,minLength=1,maxLength=500"`
	ParentID  string `json:"parent_id,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
}
// → [{"title":…, "parentId":…, "projectId":…}]; không nhận tenant/user từ input
```

**(c) `worktree_create` (write)** — `worktree.create` dùng khoá `repo`/`name`/`baseBranch` (không phải repoId/branch/baseRef — comment `channels_worktree.go`), có trường lineage; MCP luôn đặt nguồn:

```go
func worktreeCreateArgs(in json.RawMessage, id wscompat.Identity) ([]json.RawMessage, error) {
	var v WorktreeCreateInput; /* repo_id, name, base_branch?, linked_issue_ref? */
	return one(map[string]any{
		"repo": v.RepoID, "name": v.Name, "baseBranch": v.BaseBranch,
		"origin": "mcp", "captureSource": "mcp", // lineage: người dùng thấy worktree do agent tạo
	})
}
```

Mỗi hàm có `TestArgs_*` bảng (đầu vào JSON → mảng args), và **test hợp đồng** chạy handler thật với client gRPC giả để chắc `decodeArg` nhận đúng khoá.

## 4. Quy tắc đầu ra, phân trang, SSH, provider

- **Che dữ liệu** (bộ chung BE-007 §2.D + luật theo tool): URL remote `https://user:token@host/x.git` ⇒ `https://***@host/x.git` (áp cho `git.remoteCommitUrl/remoteFileUrl`, `repo.*`, `git.fetch/pull/push` stderr, `ephemeralVm.*` log); chuỗi dạng token (`ghp_`, `github_pat_`, `glpat-`, `AKIA…`, `Bearer`, khoá PEM) ở mọi trường chuỗi; trường có tên `credentialRef|apiKey|token|secret` bị xoá khỏi `structuredContent`; test quét schema đầu ra (`TestOutputSchemaHasNoSecretFields`) + golden output.
- **Phân trang (C7):** nhiều channel trả mảng đầy đủ (vd `task.list`, `jira.listIssues`). `ToolExecutor` áp `paginate.go`: tham số chung `limit` (mặc định 50, tối đa 200) và `cursor` (opaque = base64 của `{key, offset}`); kết quả đầy đủ được giữ trong LRU theo phiên MCP 120s (key = `sha256(tool+paramsHash)`), cursor hết hạn ⇒ `INVALID_CURSOR` (chạy lại). Channel có phân trang gốc (`git.history.limit`, `files.search.maxResults`) ánh xạ thẳng, không dùng LRU.
- **SSH/relay (AGENTS.md):** tool đi qua cùng channel nên tự thừa hưởng đường `infra-fleet`/relay; **cấm** `Composite` nào gọi `os/exec` hay chạm filesystem của gateway. Input chứa `connection_id`/`dev_server_id` chỉ chấp nhận id thuộc tenant (handler hạ nguồn kiểm; kèm test e2e với devServer SSH giả).
- **Git ≥ 2.25:** không thêm lệnh git mới ở gateway; lỗi `unsupported` từ `git-gateway` (qua `GitCapabilityCache`) được trả `isError` kèm gợi ý, không retry.
- **Provider-neutral:** tool dùng `hostedReview_*`/`provider` cho khái niệm chung; chỉ giữ tiền tố `github_`/`gitlab_` cho channel vốn đặc thù. Mô tả tool nêu "GitHub only" khi đúng.
- **`gh` rate limit:** cache 30s + `rateLimit` trong kết quả; `ToolExecutor` áp rate limit theo (tenant, namespace=github) 30 req/phút (T6 mở rộng khoá).

## 5. Kịch bản e2e (parity UI ↔ MCP)

1. **Đợt 1:** `project_list` → `worktree_list{projectId}` → `git_status` → `git_diff` → `hostedReview_forBranch` (+ `github_project_workItemDetailsBySlug`): so sánh với gọi trực tiếp channel qua WS `Registry` trên cùng dữ liệu (cùng entity, cùng số mục).
2. **Đợt 2:** `task_create` → `worktree_create` → ghi file trong worktree bằng cách khác → `git_stage` → `git_commit` trên repo thử nghiệm; kiểm UI thấy cùng task/worktree (`origin:"mcp"` trong lineage).
3. **Đợt 3/4 ẩn:** khi BE-012/013 chưa bật (`MCP_POLICY_ENFORCEMENT=off`), `tools/list` không chứa pack 3–4 (test cấu hình).
4. **Không rò secret:** `TestNoToolAcceptsOrReturnsSecretFields` quét mọi `Input/Output` schema + golden.

## Hợp đồng với frontend
`mcp.admin.tool.list`: `pack` ∈ 1..4 từ `ToolSpec.Pack`; tool Composite BE-009 `pack:3`; mục `listedAsHardDenied` ⇒ `pack:4,risk:'admin',hardDenied:true`. FE-MCP-SOL-005 nhóm/lọc theo các trường này.

## Sửa TDD kèm theo
T1, T2 (như BE-007). Không thêm T mới.

## Kiểm thử
`go test ./internal/adapter/mcpserver/tools/... -run 'TestArgs_|TestOutput|TestToolParity'`; e2e: `make e2e-mcp` (BE-015). Impact (chưa chạy): `gitnexus_impact({target:"RegisterRealChannels", direction:"upstream"})` — chỉ đọc, rủi ro dự kiến LOW.

## Rủi ro & phụ thuộc
Quá nhiều tool làm LLM chọn sai ⇒ giữ pack nhỏ, `tool_search`. Lệch hành vi UI ⇒ parity e2e. Phụ thuộc BE-007, BE-012/013, BE-009 (Composite pack 3).

## Không thuộc phạm vi
Cấu hình pack ở UI (FE-MCP-SOL-005 chỉ đọc; chỉnh policy ở FE-MCP-SOL-008); channel mới "checks", `mergeMR`, `reset`.

## Liên quan
`wscompat/channels*.go`; BE-MCP-SOL-007, 009, 010.

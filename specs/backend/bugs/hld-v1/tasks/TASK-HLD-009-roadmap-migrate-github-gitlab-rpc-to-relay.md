# TASK-HLD-009: [ROADMAP] Di chuyển toàn bộ RPC GitHub/GitLab nghiệp vụ từ local exec sang `relay.call`

**Priority:** 🟡 LOW (roadmap, không block fix tối thiểu) — nhưng cần thiết để không mất tính năng GitHub/GitLab trong multi-user mode sau khi TASK-HLD-006 bật guard
**Effort:** 🔴 LARGE — cần roadmap/CR riêng, chia nhiều PR nhỏ theo nhóm method
**Status:** 🟡 REVIEWED — 2026-08-09 (roadmap đã đọc và xác nhận nội dung hợp lý, prerequisite TASK-HLD-006/007/008 đã DONE nên guard đang chặn thật các thao tác gh/glab nghiệp vụ trong multi-user mode. Chưa tách thành sub-ticket cụ thể theo đúng "definition of done" của chính task này — cần một phiên làm việc riêng do effort LARGE, không thực hiện trong đợt thực thi này.)
**Bug refs:** BUG-BE-HLD-004, BUG-BE-HLD-005
**Solution ref:** [SOLUTION-github-gitlab-relay-exact.md](../solutions/SOLUTION-github-gitlab-relay-exact.md)
**Depends on:** TASK-HLD-006, TASK-HLD-007, TASK-HLD-008 (fix tối thiểu) — task này là roadmap dài hạn, **KHÔNG block** 3 task trên và cũng không bị 3 task trên block ngược lại về mặt lập kế hoạch, nhưng nên triển khai sớm nhất có thể sau khi guard ở TASK-HLD-006 được bật, để tránh mất chức năng nghiệp vụ GitHub/GitLab trong `ORCA_MULTI_USER=1`.

---

## Mục tiêu

Đây là task dạng **ROADMAP** — không phải code fix ngay, mà là bảng liệt kê phạm vi công việc cần làm để di chuyển dần các RPC method GitHub/GitLab nghiệp vụ hiện đang gọi `OrcaRuntimeService` cục bộ (→ `ghExecFileAsync`/`glabExecFileAsync`) sang `relay.call('git.*'|'github.*'|'gitlab.*', ...)`, để chạy đúng trên Dev Server Agent với cách ly credential theo user.

Việc này **cần thiết** vì sau khi TASK-HLD-006 bật guard `assertLocalGhCliAllowed()`, mọi thao tác GitHub/GitLab nghiệp vụ (list PR/issues, rate-limit, comments...) sẽ throw lỗi ngay khi chạy dưới `ORCA_MULTI_USER=1` cho đến khi các method dưới đây được migrate.

## File cần sửa/tạo

Không sửa file nào trong task này (roadmap only). Các file liên quan sẽ được xác định lại khi từng nhóm method được lập PR riêng:

```
backend/src/main/runtime/rpc/methods/github.ts       (đích sửa — tương lai)
backend/src/main/runtime/rpc/methods/gitlab.ts        (đích sửa — tương lai)
agent/src/relay/agent-rpc-dispatch.ts                 (đích sửa — tương lai, mục 4b)
agent/src/relay/external-api-connector.ts             (đích sửa — tương lai, mục 4c — viết handler mới)
backend/src/main/github/rate-limit.ts                 (đích migrate — tương lai, mục 4c)
backend/src/main/github/work-item-details.ts          (đích migrate — tương lai, mục 4c)
backend/src/main/github/project-view/*                (đích migrate — tương lai, mục 4c)
backend/src/main/github/pr-review-comment-lines.ts     (đích migrate — tương lai, mục 4c)
backend/src/main/github/merged-pr-commit-membership.ts (đích migrate — tương lai, mục 4c)
backend/src/main/github/comment-reactions.ts           (đích migrate — tương lai, mục 4c)
backend/src/main/gitlab/issues.ts                      (đích migrate — tương lai, mục 4c)
backend/src/main/gitlab/merge-request-creation.ts      (đích migrate — tương lai, mục 4c)
backend/src/main/gitlab/gitlab-project-recents.ts      (đích migrate — tương lai, mục 4c)
```

## Thay đổi cụ thể

### 4a. Case đã có sẵn ở Agent (`agent/src/relay/agent-rpc-dispatch.ts`) — chỉ cần Backend đổi caller

| RPC method | Agent handler | Dòng dispatch |
|---|---|---|
| `git.pr.create` | `agent-git-handler.ts::handleGitPrCreate` | :411 |
| `git.worktree.list/add/remove` | `agent-git-handler.ts` | :422-451 |
| `github.pr.create` | `external-api-connector.ts::handleGitHubPrCreate` | :488 |
| `github.pr.merge` | `external-api-connector.ts::handleGitHubPrMerge` | :499 |
| `github.issue.list` | `external-api-connector.ts::handleGitHubIssueList` | :510 |
| `github.issue.create` | `external-api-connector.ts::handleGitHubIssueCreate` | :521 |
| `gitlab.mr.create` | `external-api-connector.ts::handleGitLabMrCreate` | :532 |
| `gitlab.pipeline.status` | `external-api-connector.ts::handleGitLabPipelineStatus` | :543 |

Backend cần sửa các handler tương ứng trong `backend/src/main/runtime/rpc/methods/github.ts`/`gitlab.ts` (nơi hiện gọi `OrcaRuntimeService.listRepoIssues()` → `listIssues()` → `ghExecFileAsync()`) để thay bằng `ctx.devServerManager.getRelay(devServerId).call('github.issue.list', { userId: ctx.userId, cwd, ... })`.

Đây là nhóm **ưu tiên cao nhất trong roadmap** — nên làm ngay sau khi TASK-HLD-006/007/008 xong, vì đã có sẵn Agent handler + dispatch case, chỉ cần sửa Backend caller.

### 4b. Handler đã viết ở Agent nhưng **chưa được đăng ký** trong `agent-rpc-dispatch.ts` — cần bổ sung case trước khi Backend có thể gọi

Grep xác nhận các hàm này **tồn tại** trong `external-api-connector.ts` nhưng **không có case tương ứng** trong `agent-rpc-dispatch.ts`:

- `handleGitHubAuthStatus` (dòng 311) → cần thêm `case 'github.auth.status':`
- `handleGitLabAuthStatus` (dòng 460) → cần thêm `case 'gitlab.auth.status':`
- `handleGitLabMrList` (dòng 394) → cần thêm `case 'gitlab.mr.list':`

3 case này là bước trung gian bắt buộc trước khi Backend có thể relay các method tương ứng — effort nhỏ (chỉ thêm dispatch case), nên gộp chung với 4a trong cùng đợt PR đầu tiên.

### 4c. Nghiệp vụ Backend hiện có mà Agent **chưa có handler nào** — cần viết mới cả 2 phía (Agent handler + dispatch case + Backend caller)

Dựa trên bằng chứng BUG-BE-HLD-004 (`backend/src/main/github/rate-limit.ts`, `work-item-details.ts`, `project-view/*`, `pr-review-comment-lines.ts`, `merged-pr-commit-membership.ts`, `comment-reactions.ts`; `backend/src/main/gitlab/issues.ts`, `merge-request-creation.ts`, `gitlab-project-recents.ts`) chạy hoàn toàn cục bộ và không có method Agent song song nào:

- `github.rateLimit.check`
- `github.workItem.details`
- `github.projectView.*`
- `github.pr.comments` / `github.pr.reviewComments`
- `gitlab.issue.list` / `gitlab.issue.create`
- `gitlab.project.recents` / `gitlab.project.refResolve`

Đây là nhóm effort lớn nhất — viết handler Agent mới + đăng ký dispatch case + sửa Backend caller cho từng method. Cần thiết kế payload/response schema riêng cho từng method (không có sẵn pattern để copy như 4a/4b).

## Khuyến nghị thứ tự triển khai

1. Đợt PR 1: gộp mục 4a (8 method, chỉ sửa Backend caller) + mục 4b (3 dispatch case còn thiếu) — effort nhỏ, tận dụng hạ tầng Agent có sẵn.
2. Đợt PR 2+: mục 4c, tách theo nhóm nghiệp vụ (list vs. create vs. auth-status vs. rate-limit) để dễ review và test riêng biệt, mỗi nhóm là 1 PR độc lập.
3. Ưu tiên trong 4c: các luồng PR/issue/MR list phổ biến nhất trước (để giảm thời gian mất tính năng trong multi-user mode do guard ở TASK-HLD-006), rate-limit/work-item-details/project-view có thể làm sau.

## Verification

Không áp dụng trực tiếp cho task roadmap này (không có code thay đổi). Khi từng đợt PR con được triển khai dựa trên roadmap này, mỗi PR con cần:

```bash
pnpm tsc --noEmit
pnpm vitest run backend/src/main/runtime/rpc/methods/github.test.ts
pnpm vitest run backend/src/main/runtime/rpc/methods/gitlab.test.ts
pnpm vitest run agent/src/relay/agent-rpc-dispatch.test.ts
pnpm vitest run agent/src/relay/external-api-connector.test.ts

# Regression: guard TASK-HLD-006 không còn throw cho method đã migrate
ORCA_MULTI_USER=1 <chạy method đã migrate qua relay và xác nhận không throw MULTI_USER_GH_CLI_NOT_SUPPORTED>
```

Task roadmap này được coi là "hoàn tất" khi toàn bộ danh sách ở mục 4a/4b/4c đã được tách thành task/PR con cụ thể và có ticket riêng theo dõi tiến độ — không yêu cầu code hoàn chỉnh trong phạm vi task này.

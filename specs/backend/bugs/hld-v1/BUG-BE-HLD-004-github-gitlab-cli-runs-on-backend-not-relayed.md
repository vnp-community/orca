# BUG-BE-HLD-004 — Backend tự thực thi `gh`/`glab` CLI trực tiếp thay vì relay đến Dev Server Agent

**Mức độ:** 🟠 HIGH (Security/Architecture — vi phạm "Auth never through Gateway")
**Status:** 🔴 Open
**Module:** `backend/src/main/github/*`, `backend/src/main/gitlab/*`, `backend/src/main/git/runner.ts`
**Phát hiện:** 2026-08-08/09 (audit `backend/` code vs thiết kế — `audit/backend/backend-vs-design-review.md` §2.12b, §5.9/F30)

---

## Mô tả

`docs/hld/dev-server-architecture.md §12` quy định rõ: GitHub/GitLab API calls (Category A integration) **phải** chạy trên Dev Server (vai trò "External API Caller"), **không phải** trên Backend/Gateway — nguyên tắc cốt lõi "Auth never through Gateway | GitHub/GitLab tokens chỉ nằm trên Dev Server filesystem". `docs/features/F30-remote-integrations.md` xác nhận lại kiến trúc này ở mục "Kiến trúc tổng thể".

Phía Dev Server Agent (`agent/src/relay/agent-git-handler.ts`, `agent/src/relay/external-api-connector.ts`) **đã implement đúng thiết kế này** (RPC method `git.pr.create`, per-user `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR`).

Nhưng phía Backend — nơi thực sự phục vụ RPC `github.*`/`gitlab.*` cho frontend — **không hề gọi các method này**. Luồng thật:

```
backend/src/main/runtime/rpc/methods/github.ts (handler)
  → OrcaRuntimeService.listRepoIssues() (backend/src/main/runtime/orca-runtime.ts:12515)
  → listIssues() (backend/src/main/github/issues.ts:100)
  → ghExecFileAsync() (backend/src/main/github/gh-utils.ts:3, re-export từ ../git/runner)
  → child_process.execFile('gh', ...) NGAY TRONG PROCESS BACKEND (backend/src/main/git/runner.ts)
```

Hầu hết mọi thao tác GitHub/GitLab nghiệp vụ (list PR/issues, rate-limit, project-view, comments, work-item-details...) — không chỉ PR-create — chạy theo đường này. Chỉ 2 luồng hẹp thật sự relay: `*.startAuthLogin` (PTY spawn `gh auth login`) và `preflight.check` khi có `devServerId`.

## Hậu quả

- Trong Web Server multi-user mode, `gh`/`glab` trên host Backend dùng **chung 1 auth context cho mọi user** — rò rỉ/nhầm quyền giữa user.
- Backend host bắt buộc phải cài + đăng nhập sẵn `gh`/`glab` — trái mô hình "Gateway không giữ token".
- 2 bộ implementation (`backend/src/main/github/*` và `agent/src/relay/agent-git-handler.ts`) tiếp tục lệch nhau (feature drift) vì không dùng chung code path.

## Bằng chứng

- `backend/src/main/github/github-repository-identity.ts:39-50` (`ghRepoExecOptions()`) — khi repo có `connectionId` (SSH), trả `{}` (không set cwd) → `gh` chạy dựa vào `--repo owner/repo`, thực thi ngay trên host Backend, không relay.
- Grep repo-wide cho `git.pr.`/`git.issue.`/`git.mr.`/`git.repo.clone`/`git.pipeline.`: chỉ xuất hiện ở nơi định nghĩa handler (`agent/src/relay/agent-git-handler.ts`, `agent-rpc-dispatch.ts` và bản sao `desktop/`) — **0 caller** từ `backend/`.
- `backend/src/main/gitlab/issues.ts`, `gl-utils.ts` — cùng mẫu hình với GitHub (dùng `glab` CLI local).
- Xem thêm [BUG-BE-HLD-005](./BUG-BE-HLD-005-gh-config-dir-never-passed-to-relay.md) cho việc `GH_CONFIG_DIR` không hoạt động ngay cả ở 2 luồng relay hẹp còn lại.

## Đề xuất fix

1. Thay các handler trong `backend/src/main/runtime/rpc/methods/github.ts`/`gitlab.ts` bằng `relay.call('git.pr.create'|'git.issue.list'|...)` tới implementation đã có sẵn ở Agent, thay vì gọi `OrcaRuntimeService`/`ghExecFileAsync` cục bộ.
2. Nếu quyết định giữ đường chạy cục bộ cho Desktop/SSH-Target (một use case hợp lệ khác với Web-Server-Fleet), phải tài liệu hoá rõ 2 luồng khác nhau và giới hạn đường cục bộ chỉ dùng khi *không* multi-user.
3. Bổ sung per-user isolation (`GH_CONFIG_DIR`) cho nhánh Web Server multi-user trước khi release.

## Tham khảo

- Audit: `audit/backend/backend-vs-design-review.md` §2.12b, §5.9 (F30), §6 mục 2
- Doc gốc: `docs/hld/dev-server-architecture.md` §12, `docs/features/F30-remote-integrations.md`
- Liên quan: [BUG-BE-HLD-005](./BUG-BE-HLD-005-gh-config-dir-never-passed-to-relay.md), [BUG-PI-001](../project-integration/BUG-PI-001-credential-service-missing-github.md) (bug cũ liên quan credential GitHub/GitLab, đánh dấu "FIXED" nhưng chưa xác nhận khớp với finding này)

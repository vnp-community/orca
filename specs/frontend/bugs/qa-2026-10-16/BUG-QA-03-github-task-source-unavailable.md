# BUG-QA-03: Nguồn task GitHub ở tab Tasks không kết nối được

**Mức độ:** P1 · **Khu vực:** api-gateway (`github.listWorkItems`), frontend (TaskPage), scm-integration (thông tin xác thực) · **Trạng thái:** tạm hoãn theo yêu cầu (ưu tiên Jira); QA-GH-01 và QA-GH-03 vẫn đỏ

## Triệu chứng

Tab Tasks → nguồn GitHub hiện dải "Some GitHub source hosts unavailable: 4 source hosts" và danh sách rỗng ("No matching GitHub work"). Nhãn máy chủ hiện UUID (`8808c663-…, b9de01d5-…`) thay vì tên (`dev-local-mac`), trạng thái ghi "disconnected" và "source tool unavailable" cho cả dev server đang kết nối.

## Tái hiện (2026-10-16)

1. Tasks → biểu tượng GitHub. Tooltip dải cảnh báo: `…b9de01d5… disconnected, …8808c663… disconnected, … source tool unavailable ×2`.
2. Bắt khung WS khi bấm làm mới: giao diện gọi `github.listWorkItems` với `{"repo":"id:<repoId>","repoId":"<repoId>",…}` cho cả 6 repo; **mọi** phản hồi là `errors.issues = {type:"not_found", message:"no GitHub remote configured (tried upstream, origin): … WORKTREE_REPO_NOT_FOUND: repo does not exist"}`, kể cả `vnp-qa-platform` có `origin = git@github.com:blcvn/vnp-qa-platform.git`.
3. Log `git-gateway`: `GetRepo` → `PROJECT_REPO_NOT_FOUND`; project-service nhận `repo_id = "id:530cc591-…"` (kèm tiền tố `id:`) nên `WHERE id = $1` không khớp dù repo có trong `project.repos`.
4. `which gh` → không có `gh` trên máy dev server; `preflight.check` qua relay trả `github-cli-auth: skip ("not available on this connection mode")`; `onboarding.getPreflightStatus` quá 20 giây không trả lời.
5. Thử `github.rateLimit`: `SCM_CREDENTIAL_NOT_FOUND: no OAuth credential configured for github`.

## Nguyên nhân gốc (bốn lớp, phải sửa cả bốn thì danh sách mới có dữ liệu)

| # | Lớp | Nguyên nhân | Bằng chứng |
|---|---|---|---|
| 1 | Gateway | `github.listWorkItems` (và `github.countWorkItems`) đọc trường `repo` như `repoId` thô; giao diện nay gửi `repo:"id:<repoId>"` (selector) cùng `repoId` thô. Tiền tố `id:` chạy xuống git-gateway → project-service → NotFound. Hợp đồng dây lệch giữa hai phía | `channels_scm.go:202-216` (`Repo string // repoId, despite the field name`), `:1195` `resolveGitHubOwnerRepo(…, in.Repo)` |
| 2 | Frontend | Điều kiện "nguồn khả dụng" dựa trên `gh` cài sẵn và đã đăng nhập trên host (`preflightStatus.gh`), trong khi backend lấy dữ liệu qua scm-integration (không cần `gh`). Trạng thái host lấy từ bảng SSH nên dev server hiện "disconnected"; nhãn host không tra được tên | `task-source-provider-availability.ts:23-53`, `task-source-context-summary.ts` (`getUnavailableHosts`, `getHostLabel`) |
| 3 | Thông tin xác thực | Không có thông tin OAuth/token GitHub cho tenant/người dùng; giao diện không có đường "Connect GitHub" ở tab Tasks, và lỗi bị gói thành `unknown`/`not_found` | `SCM_CREDENTIAL_NOT_FOUND`; `ClassifiedError` không có loại `auth_required` |
| 4 | Preflight | `onboarding.getPreflightStatus` gọi relay `preflight.check` của agent và không có giới hạn thời gian ngắn; `preflight.check` của gateway trả `skip` cho `github-cli-auth` | `channels_onboarding.go:470-520`, `channels.go:1147` |

## Ảnh hưởng

Không dùng được GitHub làm nguồn task; người dùng không biết cần làm gì (cài `gh`? kết nối lại? cấu hình OAuth?).

## Giải pháp và task

[solutions/SOL-QA-03-github-task-source.md](./solutions/SOL-QA-03-github-task-source.md); task T-QA-03-xx; test QA-GH-01..03 (Python) và QA-UI-GH-01 (Playwright).

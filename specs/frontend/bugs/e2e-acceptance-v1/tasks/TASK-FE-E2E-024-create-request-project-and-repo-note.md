# TASK-FE-E2E-024: Create request chọn được project và hiện rõ repo mà Request sẽ chạy

**From Solution:** SOL-FE-E2E-005 (mục 1.2 hệ quả, 3.3) · **Resolves:** project nhiều repo gây nhầm vì Request chạy trên repo đầu tiên
**Priority:** P1 · **Area:** frontend (request)
**Files:**
- frontend/src/renderer/src/components/request/CreateRequestDialog.tsx (cắm ghi chú; hộp thoại đã chọn project sẵn)
- frontend/src/renderer/src/components/request/CreateRequestProjectRepoNote.tsx (mới) + `.test.tsx`
- frontend/src/renderer/src/components/request/use-request-project-repos.ts (mới)
- i18n 5 ngôn ngữ
**Depends on:** không (backend Request KHÔNG đổi)
**Status:** `[x]` XONG (2026-10-11): mã, vitest, Playwright mock và kiểm live. Kiểm live (2026-10-11): e2e `--real addproject` XANH 2/2 (Claude thật); `--real all` 7/7 XANH (24.1 phút); `--repro add-project-worktree` xanh; dữ liệu thật của người dùng không đổi.

---

## Context

`CreateRequestDialog.tsx:66-98` đã tải `project.list` và `:193-216` có Select "Project" (bắt buộc: `blocked` khi `projectId===''`). Backend chạy Request trên `repos[0]` của project (`ai_connection_resolver.go:54-62`, `project_execution_resolver.go:132`) mà UI không nói.

## Việc cần làm

- [x] Xác nhận hộp thoại hiện có đã chọn project (có; ghi vào "Kết quả thực thi").
- [x] `use-request-project-repos.ts`: với `projectId`, `repo.list {projectId}` (bỏ qua kết quả cũ khi đổi project, bắt lỗi).
- [x] `CreateRequestProjectRepoNote`: "Request sẽ chạy trên repo: <tên> (<đường dẫn>)" + host nếu có; N>1: "(repo đầu tiên trong N repo của project)"; không có repo: cảnh báo "Project này chưa có repo, Request không chạy được" (chỉ hiển thị, không chặn). `data-testid="create-request-repo-note"`.
- [x] Không đổi payload `request.create`, không đổi backend.

## Tiêu chí hoàn thành

Chọn project 1 repo thấy tên repo; project 2 repo thấy repo đầu và "đầu tiên trong 2"; lỗi tải không làm hộp thoại hỏng.

## Kiểm thử

Vitest `CreateRequestProjectRepoNote.test.tsx`, `request-action-components.test.tsx` (dialog), i18n coverage. Playwright mock: bước trong `add-project-own-project.web.e2e.ts` hoặc spec request-web hiện có.

## Blast radius

`CreateRequestDialog` được gọi từ `RequestPage.tsx`, `CreateRequestIssueButton.tsx`, `request-action-components.test.tsx`; chỉ thêm một khối phụ. THẤP.

## Rủi ro · Ước lượng

THẤP. 0,5 ngày.

## Kết quả thực thi

**Trạng thái:** XONG (mã, vitest, Playwright mock; chưa kiểm live).

**Xác nhận:** `CreateRequestDialog` đã có Select "Project" bắt buộc (`project.list`); không đổi.

**Tệp đổi:** `components/request/use-request-project-repos.ts` (mới), `CreateRequestProjectRepoNote.tsx` (mới) + `.test.tsx` (6 test), `CreateRequestDialog.tsx` (+2 dòng), i18n `auto.components.request.CreateRequestProjectRepoNote.{runsOn,runsOnWithHost,firstOfMany,noRepo}`, Playwright mock `request-answer-and-project-note.web.e2e.ts` (b).
- Selector: `create-request-repo-note`.

**Lệch so với tài liệu:** không. Tên host lấy từ `devServers` của store (rơi về id nếu chưa có). Không đổi payload `request.create`, không đổi backend.

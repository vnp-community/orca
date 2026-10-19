# TASK-FE-E2E-002: Trạng thái chặn có tên khi không tải được câu hỏi làm rõ

**From Solution:** SOL-FE-E2E-001 · **Resolves:** FE-E2E-A
**Priority:** P0 · **Area:** frontend
**Files:**
- frontend/src/renderer/src/hooks/useClarifications.ts
- frontend/src/renderer/src/components/request/clarification/ClarificationPanel.tsx
- frontend/src/renderer/src/components/request/RequestDetailPane.tsx
- i18n: auto.components.request.clarification.*
**Depends on:** TASK-E2E-001
**Status:** `[x]` DONE (2026-10-10): mã + vitest + Playwright mock; chưa kiểm riêng ca kênh thiếu trên stack thật (stack thật có `clarification.list`)

---

## Context

Hook đặt `error=null` khi `unsupported`. Panel không có gì để hiển thị khi Request ở `awaiting_information`.

## Việc cần làm

- [x] Hook trả thêm `loadFailure: {kind, code} | null` (tên thật; thay vì `blocked`) khi liệt kê thất bại. Hook chỉ được gắn với một Request khi nó ở `awaiting_information` (panel truyền `null` ở trạng thái khác), nên mọi lỗi liệt kê đều là ngõ cụt cần hiện.
- [x] Panel hiển thị trạng thái chặn: tiêu đề, nguyên nhân theo `kind` (`unsupported` có câu riêng, kind khác dùng `requestErrorMessage`), mã lỗi, nút "Retry". Dùng `border-destructive/30`, `text-destructive`, `Button` đang có (G1). Danh sách rỗng (không lỗi) giữ dòng "Waiting for the information..." và thêm nút "Refresh".
- [x] Giữ nguyên hành vi im lặng ở các trạng thái không chờ người (panel không gọi RPC, không render).
- [x] Chuỗi i18n mới đủ 5 ngôn ngữ (en, es, ja, ko, zh).

## Tiêu chí hoàn thành

- [x] Vitest: `awaiting_information` + `unsupported` → `loadFailure` (hook) và `clarification-load-failed` (panel); `completed` + `unsupported` → panel rỗng, không gọi RPC.
- [x] Playwright `request-web`: kịch bản kênh thiếu. — _ca g của `request-solution-start-and-ai-setup.web.e2e.ts` (runtime không có `clarification.list` → giải thích trạng thái chặn); chạy được sau khi sửa hạ tầng mock (config project `request-web`, import, `webServer.cwd`), 8/8 xanh_

## Kiểm thử

Vitest + Playwright (mock).

## Kết quả thực thi

**File đổi**
- `frontend/src/renderer/src/hooks/useClarifications.ts`: `loadFailure`; `error` giờ giữ cả `unsupported`; lần tải lại thất bại KHÔNG xoá câu hỏi đang mở (giữ bản nháp người dùng đang gõ).
- `frontend/src/renderer/src/components/request/clarification/ClarificationLoadFailureNotice.tsx` (mới).
- `frontend/src/renderer/src/components/request/clarification/ClarificationPanel.tsx`: thứ tự `loading` → `loadFailure` → skeleton 3 giây → dòng chờ + "Refresh".
- `RequestDetailPane.tsx`: **không cần đổi**, đã chỉ gắn panel khi `awaiting_information`.
- Locale `en/es/ja/ko/zh.json`: `clarification.loadFailedTitle|loadFailedUnsupported|loadFailedCode|refresh`.
- Test: `useClarifications.test.ts` (+4), `ClarificationPanel.test.tsx` (+4).

**Selector cho Playwright** (agent viết spec live): `data-testid="clarification-load-failed"` (role=alert, thuộc tính `data-error-kind`), `clarification-load-failed-code` (chứa `Error code: <mã>`), nút tên "Retry"; dòng chờ `clarification-waiting` nay có nút "Refresh"; `clarification-skeleton`, `clarification-panel`.

**Lệnh đã chạy**: `pnpm exec vitest run --config config/vitest.config.ts src/renderer/src/hooks/useClarifications.test.ts src/renderer/src/components/request/clarification` → 3 file, 31+ test xanh (ổn định sau 6 lần chạy lặp).

**Blast radius (rà tay)**: `useClarifications` chỉ có một caller là `ClarificationPanel`; panel chỉ mount ở `RequestDetailPane`. `error` của hook không còn bị nuốt, nhưng panel không đọc `error`. Cùng mã chạy Electron/web (qua `callRequestRpc`).

**Điều đáng lưu ý**: test cũ khẳng định "unsupported được coi là rỗng" đã đổi theo nghiệp vụ mới (đúng yêu cầu task).

---
cr_id: CR-OR-022, CR-OR-030 (UI)
solution_id: FE-OR-SOL-003
product: orca
layer: frontend
status: draft
priority: P0
effort: 5–6 ngày
depends_on: FE-OR-SOL-001, 002; BE-OR-SOL-004
created: 2026-10-20
---

# FE-OR-SOL-003 — Quyết định, "gửi lại" bằng Request con, nối review code v7, e2e UI

## 1. Phân tích
- Quyết định chỉ ở pha `delivered`; pha `design` không có thanh quyết định.
- "Gửi lại" = Request con (`SpawnChildRequestDialog.tsx`); ĐỌC TRƯỚC quy tắc cho phép tạo từ Request `completed` (backend `request_child_rules.go`).
- v7 có lối vào `openReviewFromEntryPoint(worktreeId, source, {lens, filter})` (no-op khi `codeIntelSupportState.state !== 'enabled'`); id lens đã thấy: `impact, architecture, dataflow, erd, storage, structure, contract` (`requirements` chưa xác minh). `ReviewEntrySource` hiện: `agent-row|source-control|cmd-k|right-sidebar|notification`, ánh xạ sang enum telemetry (nhiều bản sao).
- Giao diện Request chưa có sẵn `worktreeId`.

## 2. Thiết kế

### 2.1 Quyết định và duyệt
`OutcomeDecisionCard` (Accept as exception với ghi chú bắt buộc và lỗi cạnh ô, Send back, Change decision); mục `advisory` (AM-3) không có nút quyết định, chỉ nhãn "Advisory". `OutcomeDecisionBar`: ô ghi chú, **Approve** (nút chính duy nhất), "Request changes (n)". Bấm Approve khi còn mục mở: lỗi cạnh nút "Resolve N open decisions first", cuộn tới mục đầu (không vô hiệu hoá nút); máy chủ vẫn chặn. Sau duyệt: "Review approved by <name>. This is not a release sign-off."

### 2.2 Request changes
`OutcomeSendBackDialog` liệt kê mục `send_back`; `outcome-send-back-draft.ts` (hàm thuần) dựng mô tả Request con: mục, bằng chứng, id KG + snapshot, qua bộ che; mở `SpawnChildRequestDialog` điền sẵn (tiêu đề "Address review findings for …", loại gợi ý `bug` nếu chủ yếu hồi quy, ngược lại `task`), liên kết với Request gốc; lưu `childRequestId` vào quyết định; dải "A follow-up Request finished. Regenerate this review to check." khi con xong (không tự sinh lại).

### 2.3 Nối review code
`outcome-code-review-link.ts`: Architecture→`architecture`, Flow→`dataflow`, Data→`erd`, Contracts→`contract`, Requirements→`requirements`, mục có `file`→`impact`; huy hiệu container (danh sách "Inside containers") mở lens `architecture` (khả năng lọc theo container chưa xác minh). Gọi `openReviewFromEntryPoint(worktreeId, 'request-outcome', {lens})`: thêm `'request-outcome'` vào `ReviewEntrySource` và ánh xạ telemetry (tránh sửa enum telemetry nếu có thể: dùng nguồn sẵn có làm phương án lùi, Q2). Thiếu `worktreeId` hoặc v7 tắt: nút vô hiệu **kèm giải thích**. `worktreeId` suy từ `subject.heads[].taskId`; nhiều worktree → chọn.

### 2.4 Cấu trúc file
`outcome-review/decision/OutcomeDecisionBar.tsx`, `OutcomeDecisionCard.tsx`, `OutcomeSendBackDialog.tsx`, `outcome-send-back-draft.ts`, `outcome-code-review-link.ts`, `use-outcome-decisions.ts`.

### 2.5 e2e UI
`frontend/.../ui-live/outcome-review.web.e2e.ts` + `live-outcome-review-steps.ts`: mở tab sau Execution; pha `design` (không có Approve); Execution xong → tạo `delivered`; hình trong iframe sandbox; đổi theme; sinh lại; quyết định; Request changes tạo Request con. Dùng agent giả; bản thật (tốn tiền) chỉ khi người dùng đồng ý, theo quy tắc quiet-checkout.

## 3. Kiểm thử
Thuần: `outcome-send-back-draft` (che secret, giới hạn độ dài), `outcome-code-review-link`. RTL: thanh quyết định (lỗi cạnh nút, cuộn), thẻ, hộp thoại, hai render target. Từ cấm (không "safe to release"). Locale 5 ngôn ngữ.

## 4. Rủi ro
`request_child_rules.go` có thể chặn con từ Request `completed`; enum telemetry nhiều bản sao; id lens `requirements` chưa xác minh; người duyệt tự duyệt Request của chính mình không bị chặn ở MVP.

# Solutions: request-frontend (frontend, v6)

> 🚧 **In Progress.** Rà soát ngày 2026-10-07: SOL-018 đến 021 ✅ Done (2026-10-08, e2e web SPA + WS giả); 022 trở đi xem các PARTIAL-INDEX. Tiến độ ~22% (12/54 tasks). Soạn ngày 2026-10-06 từ [docs/crs/v6/request-frontend](../../../../../../docs/crs/v6/request-frontend/README.md).

## Bảng CR → Solution

| CR | Solution | Nội dung | Trạng thái |
|---|---|---|---|
| CR-REQ-018 | [FE-REQ-SOL-018](./FE-REQ-SOL-018-request-frontend-foundation.md) | Kiểu, registry luồng, RPC client, hook, store, định tuyến `requests`, badge, sidebar; gỡ `backlog` khỏi `TaskStatus` | ✅ Done (2026-10-08) |
| CR-REQ-019 | [FE-REQ-SOL-019](./FE-REQ-SOL-019-request-list-detail-classification-ui.md) | Danh sách, chi tiết, xác nhận phân loại, lịch sử, Request con, "Tạo Request" từ Tasks | ✅ Done (2026-10-08, web e2e) |
| CR-REQ-020 | [FE-REQ-SOL-020](./FE-REQ-SOL-020-solution-review-ui.md) | Xem, so sánh, chọn, duyệt, từ chối Solution (Chẩn đoán, Findings, Answer) | ✅ Done (2026-10-08, web e2e) |
| CR-REQ-021 | [FE-REQ-SOL-021](./FE-REQ-SOL-021-plan-phase-tree-and-approval-ui.md) | Cây Plan → Phase → Task, duyệt Plan/Phase/`pre_deploy`, lọc khỏi Board | ✅ Done (2026-10-08, web e2e) |
| CR-REQ-022 | [FE-REQ-SOL-022](./FE-REQ-SOL-022-approval-inbox.md) | Hộp duyệt chờ xử lý | ✅ Done (7/7 tasks, verified 2026-10-07) |
| CR-REQ-023 | [FE-REQ-SOL-023](./FE-REQ-SOL-023-backlog-screens.md) | Màn Backlog ba view | ✅ Done (7/7 tasks, verified 2026-10-07) |
| CR-REQ-032 | [FE-REQ-SOL-032](./FE-REQ-SOL-032-graph-canvas-and-lenses.md) | Canvas đồ thị và các lens | 🚧 In Progress (7/8 DONE, 032-07 BLOCKED chờ duyệt `elkjs`) |
| CR-REQ-036 | [FE-REQ-SOL-036](./FE-REQ-SOL-036-clarification-decision-readiness-impact-ui.md) | Giao diện hỏi lại, quyết định, sẵn sàng, tác động | ✅ Done (8/8 DONE, 2026-10-08) |

Các solution 018 đến 021 để chỗ cắm cho 022 và 023 (`ApprovalInboxTab`, `BacklogTab` trong `RequestPage`, `RejectReasonDialog`, `useApprovals`, `useBacklog` với `backlog.requests|tasks|execute`).

## Thứ tự phụ thuộc

```
FE-REQ-SOL-018 ─▶ FE-REQ-SOL-019 ─▶ FE-REQ-SOL-020 ─▶ FE-REQ-SOL-021 ─▶ 022, 023, 032, 036 (xem PARTIAL-INDEX-022-023.md và PARTIAL-INDEX-032-036.md)
   (018-06 làm được ngay, không cần backend; 021-01 cũng không cần backend)
```

Backend cần có (qua CR-REQ-016): 018/019 cần `request.*`, `request.flowStatus`, `request.subscribe`; 020 cần `solution.*`, `approval.*`; 021 cần `request.generatePlan`, `request.startPhase` và `task.list` (xem điểm 4 dưới).

## Quyết định chung

- Tên kênh, tham số, mã lỗi lấy từ CR-REQ-016 (28 kênh, `CODE: message`). `specs/backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md` chưa có khi soạn: mọi chỗ "tạm" phải đối chiếu lại khi file đó ra.
- Một view cấp cao `requests` ba tab; điều hướng sâu bằng `openRequestPage({section, requestId})`.
- Runtime không có kênh (`method_not_found`, `enabled=false` ở `request.flowStatus`): ẩn tính năng, không toast, không lỗi đỏ.
- Phân loại lỗi một nơi (`request-errors.ts`); component không đọc mã thô.
- Duyệt dùng `approval.approve/reject` với `expectedVersion`, `expectedDigest`; từ chối bắt buộc lý do >= 10 ký tự (`RejectReasonDialog`); Duyệt không có phím tắt; `Mod+Enter` (`isScreenSubmitShortcut`) chỉ gửi hộp nhập.
- Thời gian thực: `request.subscribe` qua `subscribeRuntimeStreamChannel` (chỉ target `environment`), target `local` hoặc stream lỗi thì polling.
- i18n: khoá đọc theo tên, tiền tố `auto.components.request.`, 5 locale, test phủ khoá.
- UI chỉ token và primitive có sẵn; không hex, không emoji.
- SSH/remote: mọi lời gọi qua `callRuntimeRpc` với `getActiveRuntimeTarget`.
- Không thêm `max-lines` disable; `TaskPage.tsx` chỉ nhận một dòng gắn component riêng.

## Điểm lệch giữa CR, README v6 và code (đã xử lý trong từng solution)

1. Tên kênh: CR-018/020 ghi `solution.chooseOption`, `backlog.list {view}`; CR-016 là `solution.choose`, `backlog.requests|tasks|execute`.
2. Tham số duyệt: CR-020/021 ghi `{approvalId, version}`; CR-016 là `{id, expectedVersion, expectedDigest, comment?}`. `startPhase`: CR-021 ghi `{requestId, phaseId}`; CR-016 là `{id, phaseTaskId}`.
3. Cờ tính năng: CR-018 để mở; CR-016 có `request.flowStatus {enabled}`; SOL-018 dùng kênh này.
4. `task.list` chỉ nhận `projectId`, `pageToken`, `pageSize` (đã đọc `channels_automation_task.go:343`); CR-021 giả định `requestId`. SOL-021 dựng cây ở client tạm.
5. `TaskTreeView` không tự "nâng Task mồ côi lên gốc" như CR-021 giả định; phải sửa (021-01).
6. `TaskDetail()` không nhận props; "Chạy" bị khoá qua `executionGateByTaskId` trong store.
7. CR-016 không có kênh liệt kê Request con, nội dung đầy đủ Solution, `viewerCan`, tìm chữ hay lọc người báo cáo cho `request.list`.
8. `frontend/package.json` chỉ có `build`, `dev`, `test`, `test:watch`; không có typecheck/lint. `package.json` gốc khai `tc:web`, `lint`, `test:e2e` nhưng `config/` gốc thiếu các tsconfig; chưa kiểm chứng các lệnh gốc chạy được.

## Cách chạy kiểm thử (chung cho 018 đến 021)

- Vitest: `pnpm --filter orca-frontend test -- <đường dẫn>` (script `vitest run --config config/vitest.config.ts`).
- Typecheck (chưa kiểm chứng): `pnpm --filter orca-frontend exec tsc --noEmit -p tsconfig.json`; baseline lỗi có sẵn phải ghi trước khi sửa (FE-TASK-001 từng ghi 114 lỗi).
- E2E (chưa kiểm chứng): `npx playwright test tests/e2e/<file> --config tests/playwright.config.ts --project electron-headless` từ gốc repo (`tests/e2e/` có `tasks-page.spec.ts` làm mẫu).
- Trước khi sửa symbol: GitNexus `impact`; trước khi commit: `detect_changes` (theo CLAUDE.md; chưa chạy).

## Chỉ mục bổ sung

- [PARTIAL-INDEX-022-023.md](./PARTIAL-INDEX-022-023.md): CR-REQ-022 (hộp duyệt) và 023 (Backlog), kèm yêu cầu bổ sung cho FE-REQ-SOL-018 và 020.
- [PARTIAL-INDEX-032-036.md](./PARTIAL-INDEX-032-036.md): CR-REQ-032 (đồ thị) và 036 (hỏi lại, quyết định, sẵn sàng, tác động).

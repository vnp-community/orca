# PARTIAL-INDEX: Solutions của CR-REQ-022 và CR-REQ-023 (frontend)

Phần này để người điều phối gộp vào `solutions/README.md`. Không phải README.

## Bảng CR → Solution

| CR | Tên | Solution | Task | Priority | Phụ thuộc chính |
|---|---|---|---|---|---|
| CR-REQ-022 | Hộp duyệt chờ xử lý | [FE-REQ-SOL-022](./FE-REQ-SOL-022-approval-inbox.md) | FE-REQ-TASK-022-01 đến 022-07 | 🟠 P1 | FE-REQ-SOL-018, 020 (`RejectReasonDialog`), 019/021 (đích mở); backend CR-016, 009, 010 |
| CR-REQ-023 | Màn hình Backlog ba view | [FE-REQ-SOL-023](./FE-REQ-SOL-023-backlog-screens.md) | FE-REQ-TASK-023-01 đến 023-07 | 🟠 P1 | FE-REQ-SOL-018, 019, 021, FE-REQ-SOL-022 (hook dùng chung); backend CR-016, 015, 006 |

## Thứ tự phụ thuộc

```
FE-REQ-SOL-018 ──▶ FE-REQ-SOL-019 ──▶ FE-REQ-SOL-020 ──▶ FE-REQ-SOL-021
        │                                   │                   │
        ▼                                   ▼                   ▼
   FE-REQ-SOL-022 (cần 018, 020)       (RejectReasonDialog)   (đích Plan)
        │
        └─ task 022-03 (useRequestSummaries, useMinuteClock, request-relative-time)
        └─ task 022-04 (useRowListKeyboardNavigation)
                │
                ▼
         FE-REQ-SOL-023 (cần 018, 019, 021 và hai task trên)
```

022 và 023 chạy song song sau khi 018 xong, trừ việc 023-04 trở đi cần 022-03 và 022-04. Có thể làm 022-03, 022-04 trước để mở khoá 023.

## Quyết định chung của hai solution

- Kênh WS theo CR-REQ-016 (README v6 mục 8 #12): `approval.listPending|approve|reject`, `backlog.requests|tasks|execute`, `request.reopen|cancel|get|subscribe`. Không dùng `backlog.list` như CR-REQ-018/022/023 viết.
- Mọi lệnh duyệt gửi `expectedVersion` và `expectedDigest` (CR-016 D3).
- Không có kênh sự kiện `approval.*`/`backlog.*`: chỉ `request.subscribe` → `request.event`, nên sự kiện chỉ kích hoạt tải lại, không vá hàng tại chỗ.
- Không có `total` ở `approval.listPending` và `backlog.*`: số mục chỉ gần đúng, ghi `+` hoặc `99+`.
- Lọc phía client cho điều server không hỗ trợ (dự án, quá hạn, nhóm subject, tìm kiếm, loại Request).
- Chỉ dùng token và shadcn primitives; chỉ khoá i18n đọc theo tên + test phủ khoá 5 locale (`en, es, ja, ko, zh`).
- Lệnh test: `pnpm --filter orca-frontend test <đường dẫn>` (`frontend/package.json` chỉ có `build`, `dev`, `test`, `test:watch`); typecheck/lint root chưa kiểm chứng đường chạy.

## Yêu cầu bổ sung cho FE-REQ-SOL-018 (để người điều phối chuyển)

1. `requestPage.focus?: 'type_confirmation'|'analysis'|'plan'` và action đổi `backlogView`.
2. Chấm số chờ duyệt đếm bằng `approval.listPending {pageSize: 100}` (số hàng, `99+` khi >99 hoặc còn `nextPageToken`), không dùng `limit 1`.
3. `Approval` có `subjectDigest`, `version`, `dueAt`, `createdAt`.
4. Thay `TaskBacklogItem`, `ExecuteBacklogItem`, `BacklogView` bằng `shared/request-backlog-types.ts` (FE-REQ-TASK-023-01); chữ ký `useBacklog` theo FE-REQ-TASK-023-02; tên kênh `backlog.requests|tasks|execute`.
5. `RejectReasonDialog` (SOL-020) xuất props `{open, onOpenChange, title, minLength, onSubmit(comment): Promise<void>}`.

## Mâu thuẫn và thiếu sót phát hiện (tóm tắt, chi tiết ở mục 1 của từng solution)

- CR-018/022/023 dùng `backlog.list`, `approval.approve {approvalId, version}`; CR-016 chốt khác (xem trên).
- CR-023 nói mở lại quay về bước `returned_from_stage`; CR-006 nói về `classifying`.
- Mã lỗi: CR-016 `APPROVAL_*`, CR-009/010 `REQUEST_APPROVAL_*`.
- `ListPendingForUser` không nhúng Request (N+1 `request.get`); không có `total`; không có luồng sự kiện approval.
- `BacklogTaskRow` không có mốc thời gian; "Chưa chia Phase" không có cờ riêng.
- `request.reopen` thiếu `note`/`expectedVersion` so với CR-006 và CR-023.

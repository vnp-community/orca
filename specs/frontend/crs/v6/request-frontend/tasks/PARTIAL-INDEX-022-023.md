# PARTIAL-INDEX: Task của CR-REQ-022 và CR-REQ-023 (frontend)

Phần này để người điều phối gộp vào `tasks/README.md`. Không phải README. Tất cả ở trạng thái `[ ] TODO`; chưa chạy test nào.

## Bảng Solution → Task

### FE-REQ-SOL-022 (Hộp duyệt)

| Task | Tên | Priority | File chính | Phụ thuộc |
|---|---|---|---|---|
| [FE-REQ-TASK-022-01](./FE-REQ-TASK-022-01-approval-inbox-rules.md) | Quy tắc thuần (sắp xếp, duyệt nhanh, nhóm lọc, đích mở) | P1 | `components/request/approval/approval-inbox-rules.ts` | SOL-018 (kiểu) |
| [FE-REQ-TASK-022-02](./FE-REQ-TASK-022-02-use-approval-inbox-hook.md) | `useApprovalInbox` + kết cục lỗi quyết định | P1 | `hooks/useApprovalInbox.ts`, `lib/approval-decision-outcome.ts` | 022-01, SOL-018 |
| [FE-REQ-TASK-022-03](./FE-REQ-TASK-022-03-request-summaries-minute-clock-relative-time.md) | `useRequestSummaries`, `useMinuteClock`, thời gian tương đối | P1 | `hooks/useRequestSummaries.ts`, `hooks/useMinuteClock.ts`, `lib/request-relative-time.ts` | SOL-018 |
| [FE-REQ-TASK-022-04](./FE-REQ-TASK-022-04-row-list-keyboard-navigation.md) | Hook `j`/`k`/`Enter` cho danh sách hàng | P1 | `hooks/useRowListKeyboardNavigation.ts` | không |
| [FE-REQ-TASK-022-05](./FE-REQ-TASK-022-05-approval-row-list-toolbar-states.md) | `ApprovalRow`, `ApprovalList`, thanh lọc, trạng thái | P1 | `components/request/approval/*.tsx` | 022-01, 03, 04; SOL-018, 020 |
| [FE-REQ-TASK-022-06](./FE-REQ-TASK-022-06-approval-inbox-tab-wiring-and-count.md) | `ApprovalInboxTab`: lắp ghép, xác nhận, từ chối, chấm số | P1 | `components/request/approval/ApprovalInboxTab.tsx`, `RequestPage.tsx`, `SidebarNav.test.tsx` | 022-02, 03, 05; SOL-018, 020 |
| [FE-REQ-TASK-022-07](./FE-REQ-TASK-022-07-approval-inbox-i18n-and-docs.md) | i18n 5 locale, test phủ khoá, docs | P1 | `i18n/locales/*.json`, `i18n/request-approval-locale-coverage.test.ts`, `docs/ui/pages/requests.md` | 022-05, 06 |

### FE-REQ-SOL-023 (Backlog)

| Task | Tên | Priority | File chính | Phụ thuộc |
|---|---|---|---|---|
| [FE-REQ-TASK-023-01](./FE-REQ-TASK-023-01-backlog-wire-types-and-parsers.md) | Kiểu và parser ba view backlog | P1 | `shared/request-backlog-types.ts` | SOL-018 |
| [FE-REQ-TASK-023-02](./FE-REQ-TASK-023-02-use-backlog-hook.md) | `useBacklog` (phân trang, sự kiện, polling, lỗi riêng) | P1 | `hooks/useBacklog.ts` | 023-01, SOL-018 |
| [FE-REQ-TASK-023-03](./FE-REQ-TASK-023-03-reopen-and-cancel-request-dialogs.md) | `ReopenRequestDialog`, `CancelRequestDialog` | P1 | `components/request/backlog/{Reopen,Cancel}RequestDialog.tsx` | SOL-018 |
| [FE-REQ-TASK-023-04](./FE-REQ-TASK-023-04-backlog-tab-shell-segments-and-states.md) | Khung `BacklogTab`, phân đoạn, phím `1/2/3`, trạng thái | P1 | `components/request/backlog/{BacklogTab,BacklogSegmentControl,BacklogToolbar,BacklogStates}.tsx` | 023-02; 022-03, 022-04 |
| [FE-REQ-TASK-023-05](./FE-REQ-TASK-023-05-request-backlog-table.md) | `RequestBacklogTable` (Mở lại, Hủy) | P1 | `components/request/backlog/RequestBacklog{Table,Row}.tsx` | 023-01, 03, 04; 022-03 |
| [FE-REQ-TASK-023-06](./FE-REQ-TASK-023-06-task-and-execute-backlog-tables.md) | `TaskBacklogTable`, `ExecuteBacklogTable` (chỉ đọc) | P1 | `components/request/backlog/{Task,Execute}Backlog*.tsx`, `BacklogTaskSheet.tsx` | 023-01, 04; 022-03 |
| [FE-REQ-TASK-023-07](./FE-REQ-TASK-023-07-board-hint-i18n-and-docs.md) | Gợi ý Board, i18n, test phủ khoá, docs | P1 | `components/task/TaskBoardView.tsx`, `i18n/*`, `docs/ui/pages/requests.md` | 023-04, 05, 06; SOL-018 |

## Thứ tự phụ thuộc

```
SOL-018 xong
   │
   ├─ 022-04 ─────────────────────────────┐
   ├─ 022-03 ─────────────────────────────┤  (hai task này mở khoá 023)
   ├─ 022-01 ─▶ 022-02 ─┐                 │
   │                    ├─▶ 022-06 ─▶ 022-07
   │      022-05 ◀(01,03,04, SOL-020)──┘
   │
   ├─ 023-01 ─▶ 023-02 ─▶ 023-04 ─┬─▶ 023-05 ─┐
   ├─ 023-03 ─────────────────────┘           ├─▶ 023-07
   │                              └─▶ 023-06 ─┘
```

Chi tiết: 023-04 cần 023-02 và (022-03, 022-04); 023-05 cần 023-01, 03, 04; 023-06 cần 023-01, 04. 023-03 độc lập, làm song song với 023-01, 02.

## Lệnh kiểm chứng chung (chưa chạy)

- Một file: `pnpm --filter orca-frontend test <đường dẫn từ gốc repo hoặc từ frontend/>` (vitest `run` với `config/vitest.config.ts`).
- Cả hai nhóm: `pnpm --filter orca-frontend test frontend/src/renderer/src/components/request`.
- Typecheck: `frontend/package.json` không có script; thử `pnpm --filter orca-frontend exec tsc --noEmit -p tsconfig.json` (chưa kiểm chứng). Script `typecheck`/`lint` ở `package.json` gốc trỏ `config/tsconfig.*.json` không thấy trong repo.
- i18n: `pnpm verify:localization-catalog`, `pnpm verify:localization-coverage` ở gốc (chưa kiểm chứng chạy được).

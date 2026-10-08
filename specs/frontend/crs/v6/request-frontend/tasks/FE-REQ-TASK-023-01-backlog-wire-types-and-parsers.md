# FE-REQ-TASK-023-01: Kiểu và parser cho ba view backlog

**From Solution:** [FE-REQ-SOL-023](../solutions/FE-REQ-SOL-023-backlog-screens.md) mục 2.2, bảng Correction C3, C5, C8
**Priority:** P1
**Area:** frontend (shared types, parser thuần)
**File:** `frontend/src/shared/request-backlog-types.ts` (mới), `frontend/src/shared/request-backlog-types.test.ts` (mới)
**Depends on:** FE-REQ-SOL-018 (`request-types.ts`: `RequestType`, `ReturnedFromStage`; `request-wire-parsers.ts`)
**Status:** [x] DONE (verified 2026-10-07: request-backlog-types.test.ts 7/7; oxlint sạch, không thêm lỗi tsc ở file của task)

## Context

- CR-REQ-015 mục 2.1 định nghĩa proto: `BacklogRequestRow {request_id, number, title, type, source_provider, source_ref, source_url, returned_from_stage, returned_category, return_reason, returned_by, returned_at, parent_request_ids}`, `BacklogGroup {request_id, plan_task_id, plan_title, phase_task_id, phase_title, gate_status, tasks[]}`, `BacklogTaskRow {task_id, title, status, estimated_hours, assignee_id, blocked_by_task_ids, last_engine, last_link_status, failed_attempts, last_error}`, `ListBacklogResponse {request_rows, groups, next_page_token}`.
- Gateway trả camelCase (CR-016 mục 2.2: không trả message proto trực tiếp, view struct camelCase), tên cụ thể chưa liệt kê. Parser đọc camelCase là chính, tạm chấp nhận snake_case.
- CR-018 mục 2.1 liệt kê `RequestBacklogItem`, `TaskBacklogItem`, `ExecuteBacklogItem` (phẳng) trong `request-types.ts`; task này thay hai kiểu sau bằng kiểu nhóm (yêu cầu cho SOL-018, ghi ở SOL-023 mục 4). Giá trị `ReturnedFromStage` ∈ `classification|analysis|plan|phase|task` (README v6 mục 3.3).
- `shared/` chỉ chứa code không phụ thuộc DOM và React; dùng được ở main, preload, renderer.

## Việc cần làm

1. Khai báo kiểu: `BacklogView = 'request'|'task'|'execute'`; `GateStatus = 'approved'|'pending'|'rejected'|'none'|'unknown'`; `ReturnedCategory = 'missing_info'|'infeasible'|'blocked_dependency'|'rejected'|'other'|'unknown'`; `RequestBacklogRowData`, `BacklogTaskRowData` (`estimatedHours: number|null`, `blockedByTaskIds: string[]`, `failedAttempts: number`, `lastStartedAt?: string`), `BacklogGroupData`, `BacklogPage<T> = {items: T[]; nextPageToken: string|null}` theo SOL-023 mục 2.2.
2. `parseRequestBacklogPage(raw: unknown): BacklogPage<RequestBacklogRowData>`: đọc `requestRows` (hoặc `request_rows`), bỏ phần tử không có `requestId`; enum lạ → `'unknown'`; `sourceUrl` chỉ giữ khi `new URL(x).protocol` là `http:` hoặc `https:` (chặn `javascript:`; ném lỗi `URL` thì bỏ); `parentRequestIds` mặc định `[]`.
3. `parseTaskBacklogPage(raw: unknown): BacklogPage<BacklogGroupData>` dùng cho cả `task` và `execute`: đọc `groups`; mỗi `tasks` bỏ phần tử không có `taskId`; số âm hoặc không phải số của `estimatedHours` → `null`, `failedAttempts` → `0`; `blockedByTaskIds` mặc định `[]`; `gateStatus` lạ → `'unknown'`.
4. `normalizeNextPageToken(raw)`: chuỗi rỗng hoặc thiếu → `null`.
5. Hằng `BACKLOG_RPC_BY_VIEW = { request: 'backlog.requests', task: 'backlog.tasks', execute: 'backlog.execute' } as const` (tên kênh theo CR-016 mục 2.4; không dùng `backlog.list`). Nếu SOL-018 đã khai báo trong `request-rpc-methods.ts`, import từ đó và bỏ hằng này.
6. Không dùng `any`; parser nhận `unknown` và thu hẹp bằng hàm kiểm kiểu nhỏ (`asString`, `asNumber`, `asArray`) đặt cục bộ trong file.

## Kiểm thử

`request-backlog-types.test.ts`:
- trang đủ trường; trang thiếu trường (không ném lỗi); enum lạ (`gateStatus: 'weird'` → `'unknown'`, `returnedCategory: 'x'` → `'unknown'`);
- `sourceUrl: 'javascript:alert(1)'` bị bỏ, `'https://x/y'` giữ, chuỗi rác bị bỏ;
- cùng dữ liệu ở camelCase và snake_case cho cùng kết quả;
- `nextPageToken: ''` → `null`; `groups` null → mảng rỗng;
- `estimatedHours: -3` → `null`.

Chạy (chưa chạy): `pnpm --filter orca-frontend test frontend/src/shared/request-backlog-types.test.ts`.

## Tiêu chí hoàn thành

- [ ] Parser không ném lỗi với đầu vào bất kỳ (`undefined`, `null`, chuỗi, mảng).
- [ ] `sourceUrl` chỉ http/https.
- [ ] Cấu trúc nhóm giữ nguyên thứ tự server.
- [ ] Kiểu xuất được `useBacklog` (task 02) và các bảng (task 05, 06) dùng, không trùng với kiểu phẳng của SOL-018.

## Rủi ro và lưu ý

- Tên trường camelCase là giả định cho tới khi CR-016 chốt; một test "snake_case chấp nhận" bảo vệ khỏi lệch nhẹ nhưng nên xoá nhánh này khi chốt (đánh dấu `// TODO` ngắn kèm lý do).
- Không có `total` trong `ListBacklogResponse`: không khai báo trường `total`.

## Ghi chú triển khai (2026-10-07)

- `BacklogView` giữ kiểu có sẵn `'requests'|'tasks'|'execute'` (slice đang dùng) thay vì đổi sang số ít; `BACKLOG_RPC_BY_VIEW` map theo giá trị đó. Hai kiểu phẳng `TaskBacklogItem`/`ExecuteBacklogItem` ở `request-types.ts` vẫn còn (không còn nơi dùng ngoài `parseBacklogItem`).

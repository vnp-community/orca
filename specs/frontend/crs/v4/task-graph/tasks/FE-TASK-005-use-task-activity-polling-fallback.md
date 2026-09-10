# FE-TASK-005: `useTaskActivity` — polling fallback (áp dụng thiết kế có sẵn)

**Domain:** task-graph
**Solution Ref:** FE-SOL-001 Phần 5
**Priority:** 🟠 P1
**Estimated:** 35 phút
**Status:** [x] DONE

---

## Kết quả thực thi (2026-09-09)

- Copy nguyên văn design của flow-task's FE-TASK-003 (đọc trực tiếp file đó +
  `FE-SOL-001-unified-task-execution-ui.md` §3 để lấy code mẫu đầy đủ, vì file task-graph này chỉ
  tóm tắt không có code): `useTaskActivity.ts` dùng `useReducer`, `TASK_ACTIVITY_POLL_INTERVAL_MS =
  4_000` (cùng hằng số `useWorkflowExecution.ts`), `isLive: false` cố định, poll `task.get` ngay khi
  mount + mỗi 4s, cleanup đúng (`cancelled` flag + `clearInterval`).
- Xác nhận lại hạ tầng KHÔNG đổi: `subscribeRuntimeEvent` vẫn 0 kết quả, `RuntimeClientEvent` vẫn
  chưa có variant task/workflow activity — đúng kết luận "vẫn đúng nguyên văn hôm nay" của task file.
- `TaskDetail.tsx`: thêm `useTaskActivity(task?.id ?? null)` đặt SAU effect
  `task.resolvePermission` (điểm khác biệt 1 của task), hiển thị `<TaskStatusBadge status=
  {polledTask?.status ?? task.status} />` cạnh Status Select (điểm khác biệt 2 — tận dụng
  `TaskStatusBadge` 7-status đầy đủ đã vá ở FE-TASK-003 thay vì render text thô).
- Test: `useTaskActivity.test.ts` (5 case: poll ngay khi mount, advance 4000ms gọi lại + cập nhật
  state, unmount clearInterval, taskId=null không effect, isLive luôn false), thêm 1 case vào
  `TaskDetail.test.tsx` (polledTask.status='done' → badge hiển thị "✅ Done" thay vì
  task.status='todo' gốc). `npx vitest run` cả 2 file → 16/16 pass.
- Không vá `subscribeRuntimeEvent`/thêm variant event — đúng phạm vi "Không làm ở task này".

## Mục tiêu

FE-SOL-001 §5 tự nói rõ: *"áp dụng thiết kế có sẵn, không thiết kế lại"* — implement `useTaskActivity`
theo đúng `CR-FLOW-TASK-005` §2's thiết kế, với fallback polling `task.get` mỗi 4 giây khi kênh
`task.activity:{taskId}` (CR-FLOW-TASK-003) chưa sẵn sàng. Task này **là chính task đó**, đã được
đặc tả đầy đủ ở series flow-task trước — không granularize thêm, chỉ trỏ đúng bản đã có và xác nhận
lại tình trạng hạ tầng chưa đổi.

## Xác nhận đã đọc code thật trước khi viết task (2026-09-09) — tình trạng hạ tầng KHÔNG đổi từ lúc viết flow-task

- `grep -rn "subscribeRuntimeEvent" frontend/src` vẫn = 0 kết quả — hàm này chưa từng tồn tại, đúng
  như xác nhận cũ.
- Cơ chế push-event thật duy nhất vẫn là `subscribeRuntimeClientEvents(environmentId, onEvent,
  onError, onReplayed)` (`frontend/src/renderer/src/runtime/runtime-client-events.ts:12-36`), vẫn
  giới hạn bởi union đóng `isRuntimeClientEvent` (dòng 60-75) — không có variant nào cho
  task/workflow activity, và không nhận target `'local'`.
- `task.get` RPC (`channels.go:311-323`) vẫn tồn tại thật, dùng chung với refetch thủ công hiện có
  của `TaskDetail.tsx`/`useTask.ts` — RPC này không đổi.
- `useWorkflowExecution.ts:8` (`EXECUTION_POLL_INTERVAL_MS = 4_000`) vẫn là pattern polling mẫu duy
  nhất trong codebase để copy — không có pattern thứ 2 nào mới xuất hiện.

**Kết luận: mọi phân tích + code mẫu của
[flow-task's FE-TASK-003](../../../v3/flow-task/tasks/FE-TASK-003-use-task-activity-polling-fallback.md)
vẫn đúng nguyên văn hôm nay.** Task này không viết lại nội dung đó (tránh trùng lặp/rời rạc 2 nguồn
sự thật) — implement theo file đó, chỉ khác 2 điểm nêu ở dưới do bối cảnh task-graph series đã có
thêm UI mới (FE-TASK-001..004 ở series này).

## Files cần sửa

Giống hệt bảng ở
[flow-task's FE-TASK-003](../../../v3/flow-task/tasks/FE-TASK-003-use-task-activity-polling-fallback.md#files-cần-sửa):

| File | Action |
|------|--------|
| `frontend/src/renderer/src/hooks/useTaskActivity.ts` | NEW — polling fallback |
| `frontend/src/renderer/src/components/task/TaskDetail.tsx` | MODIFY — dùng `useTaskActivity(task.id)` thay theo dõi im lặng sau `handleRunAgent` |
| `frontend/src/renderer/src/hooks/__tests__/useTaskActivity.test.ts` | NEW |
| `frontend/src/renderer/src/components/task/__tests__/TaskDetail.test.tsx` | MODIFY — case polled task status hiển thị lại |

## Các bước thực thi

Copy nguyên văn nội dung "Các bước thực thi" của
[flow-task's FE-TASK-003](../../../v3/flow-task/tasks/FE-TASK-003-use-task-activity-polling-fallback.md#các-bước-thực-thi)
— cùng `TASK_ACTIVITY_POLL_INTERVAL_MS = 4_000`, cùng `isLive: false` cố định, cùng RPC `task.get`,
cùng poll-ngay-khi-mount. **2 điểm khác biệt do task-graph series đã build thêm UI ở
`TaskDetail.tsx` (FE-TASK-004 ở series này thêm tab Access + permission badge):**

1. Đặt `useTaskActivity(task.id)` **sau** khối `useEffect` fetch `task.resolvePermission` của
   FE-TASK-004 (nếu FE-TASK-004 đã merge trước) — 2 effect độc lập, thứ tự khai báo không ảnh hưởng
   hành vi, chỉ giữ code dễ đọc theo nhóm chức năng (permission trước, activity sau).
2. `polledTask?.status` hiển thị lại nên đặt cạnh `TaskStatusBadge` nếu FE-TASK-003 (Board view, task
   này KHÔNG phải cùng số nhưng cùng series) đã thêm badge 7-status đầy đủ vào
   `TaskStatusBadge.tsx` — dùng `<TaskStatusBadge status={polledTask?.status ?? task.status} />`
   thay vì render text thô, tận dụng badge đã có màu/icon đúng cho cả 7 status thay vì tự vẽ lại.

```tsx
// TaskDetail.tsx — ví dụ cụ thể hoá điểm 2, đặt trong tab Details cạnh Status Select hiện có
const { task: polledTask } = useTaskActivity(task.id)
// ...
<TaskStatusBadge status={polledTask?.status ?? task.status} />
```

## Không làm ở task này

Giống hệt mục "Không làm ở task này" của
[flow-task's FE-TASK-003](../../../v3/flow-task/tasks/FE-TASK-003-use-task-activity-polling-fallback.md#không-làm-ở-task-này-task-follow-up-riêng-khi-cr-003-xong)
— không vá `subscribeRuntimeEvent`/thêm variant event, việc đó đụng `preload`/main process, ngoài
phạm vi "Tác động" CR-005.

## Test cases cần cover

Giống hệt
[flow-task's FE-TASK-003](../../../v3/flow-task/tasks/FE-TASK-003-use-task-activity-polling-fallback.md#test-cases-cần-cover),
cộng 1 case mới do điểm khác biệt 2 ở trên:

```
useTaskActivity.test.ts — copy nguyên 4 case gốc (mount gọi ngay, advance 4000ms gọi lại, unmount
clearInterval, taskId null không effect)

TaskDetail.test.tsx (case mới, thêm vào case gốc)
└── polledTask.status đổi → <TaskStatusBadge> hiển thị đúng icon/label mới (không phải text thô)
```

## Verify

```bash
cd frontend && npx vitest run \
  src/renderer/src/hooks/__tests__/useTaskActivity.test.ts \
  src/renderer/src/components/task/__tests__/TaskDetail.test.tsx
cd frontend && npx tsc --noEmit -p .
```

## gitnexus

Chạy trước khi sửa (bắt buộc theo CLAUDE.md):
```
impact({target: "TaskDetail", direction: "upstream"})
```
Task này sửa cùng vùng `TaskDetail.tsx` với FE-TASK-004 ở series này (Access tab + permission badge)
— khuyến nghị làm **sau** FE-TASK-004 để giảm xung đột merge (không phải hard dependency chức năng).
Risk dự kiến LOW — dán kết quả thật vào PR.

## Depends on

Không có phụ thuộc chức năng cứng. Khuyến nghị làm sau FE-TASK-004 (cùng file `TaskDetail.tsx`) để
tránh conflict merge.

## Blocking

Không có

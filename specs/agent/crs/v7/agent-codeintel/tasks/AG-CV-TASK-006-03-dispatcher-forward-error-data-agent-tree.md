# AG-CV-TASK-006-03: Chuyển `error.data` ở `RelayDispatcher.handleRequest` (cây `agent/`)

**From Solution:** [AG-CV-SOL-006-relay-ssh-part-b-handlers](../solutions/AG-CV-SOL-006-relay-ssh-part-b-handlers.md) mục 2.2
**Priority:** P2
**Area:** `agent/`
**File:** `agent/src/relay/dispatcher.ts` (sửa ≤ 5 dòng), test dispatcher hiện có (sửa)
**Depends on:** [002](./AG-CV-TASK-006-02-relay-handlers-and-throwable-mapping.md); điều kiện: O-5, Q2
**Status:** [x] DONE

## Context
`dispatcher.ts:485-491` gửi `{code, message}`; kiểu lỗi đã có `data?`. Contract §3.4: sửa ≤ 5 dòng ở cả hai cây. File có sẵn `eslint-disable max-lines` ở đầu: **không thêm** cái mới.

## Việc cần làm
1. **Impact trước khi sửa:** `gitnexus impact -r /opt/repos/orca handleRequest --direction upstream`, chọn uid `agent/src/relay/dispatcher.ts` (có nhiều `handleRequest` cùng tên: `ssh-channel-multiplexer.ts`, `relay-daemon-health-server.ts`...); báo HIGH/CRITICAL.
2. Trong catch: lấy `data` từ `err`; `sendResponse(client, req.id, undefined, { code, message, ...(data !== undefined && { data }) })`.
3. Rà các handler Part B khác throw lỗi có `.data` (grep `data:` trong `Object.assign(new Error`); ghi vào PR.

## Kiểm thử
Thêm vào test dispatcher hiện có: lỗi có `data` -> khung có `data`; lỗi không `data` -> khung **y hệt** cũ.
Lệnh: `ls src/relay | grep dispatcher` để lấy tên file test, rồi `pnpm exec vitest run src/relay/<tên>.test.ts`; `pnpm test`.

## Tiêu chí hoàn thành
- [x] Diff ≤ 5 dòng; hồi quy xanh.

## Rủi ro
- Phương án dự phòng (mã trong `message`) không khuyến nghị.

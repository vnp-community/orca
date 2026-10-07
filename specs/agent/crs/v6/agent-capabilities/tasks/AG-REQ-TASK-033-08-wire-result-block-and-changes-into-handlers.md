# AG-REQ-TASK-033-08: Nối khối kết quả, danh sách file thay đổi và cảnh báo vào hai handler

**From Solution:** [AG-REQ-SOL-033-result-block-changes-and-output-cap](../solutions/AG-REQ-SOL-033-result-block-changes-and-output-cap.md) mục 2.5 và 2.6
**Priority:** P1
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/agent-print-mode-exec.ts` (sửa), `agent/src/relay/agent-print-mode-exec.test.ts` (sửa)
**Depends on:** [04](./AG-REQ-TASK-033-04-wire-readonly-and-workspace-into-handlers.md), [05](./AG-REQ-TASK-033-05-bounded-output-buffer.md), [06](./AG-REQ-TASK-033-06-result-block-parser.md), [07](./AG-REQ-TASK-033-07-worktree-change-snapshot.md)
**Status:** [x] DONE

## Context

Sau 04 đến 07, các mảnh đã có: tham số `reportChanges`, `resultBlock.nonce`, `maxOutputBytes` đã được kiểm (task 01); `BoundedOutputBuffer` (05); `parseResultBlock` (06); `captureSnapshot`/`diffSnapshots` (07); `applied`, `warnings` cho chế độ chỉ đọc (04). Task này lắp chúng vào luồng chạy và định hình phản hồi cuối. Phụ thuộc đã chốt với CR-REQ-029 Q4 (`SimpleExecutor` gửi `resultBlock`, `reportChanges`) và CR-REQ-008 (dùng `changes`/`READONLY_VIOLATION` để loại kết quả).

Đã đọc: handler thường trả `{ jsonrpc, id, result: { ...result, stepId } }`; handler stream hiện không cộng dồn gì và `stream.end` chỉ có `exitCode`; nhánh hết giờ của handler stream gửi `stream.end` với `exitCode: -1` và đặt `settled = true` ngay lập tức.

Thứ tự (solution 2.5): validate workspace -> `buildAgentEnv` -> readonly policy -> `[reportChanges] captureSnapshot(before)` -> `spawn` -> thu thập -> `close`/hết giờ -> `[reportChanges] captureSnapshot(after)` -> diff -> `[resultBlock] parseResultBlock(stdoutTail)` -> dựng kết quả. Chụp "trước" xảy ra SAU `buildAgentEnv` và trước `spawn`, nên nếu `buildAgentEnv` ném lỗi thì không chụp; chụp "sau" chạy cả khi hết giờ (tiến trình bị `SIGKILL` có thể đã ghi dở file, đó là điều cần biết).

## Việc cần làm

1. Chế độ chụp: `snapshotMode = options.workspaceKind === 'scratch' ? 'directory' : 'git'`. `cwd` chụp là `realPath` (hoặc `worktreePath` với kind `worktree`).
2. Handler thường: nếu `options.reportChanges` thì `before = await captureSnapshot(cwd, snapshotMode)` ngay trước `spawn` (hạn 30 giây trong chính hàm chụp). Sau `finish(...)` (kể cả hết giờ): `after = await captureSnapshot(...)`, `changes = diffSnapshots(before, after)`.
3. Nếu `options.resultBlockNonce` thì `parsed = parseResultBlock(stdoutBuf.toString(), nonce)` (chạy trên phần đuôi giữ lại, đã qua `maxOutputBytes`). Nếu `stdoutBuf.truncated` và khối bị mất `BEGIN` thì kết quả `RESULT_BLOCK_MISSING` (hệ quả có chủ ý).
4. `READONLY_VIOLATION`: khi `options.accessMode === 'readonly'` và `changes.available` và (`changes.changedFiles.length > 0` hoặc `changes.headMoved`) thì thêm `'READONLY_VIOLATION'` vào `warnings`. Chỉ cảnh báo; KHÔNG hoàn tác, KHÔNG đổi `exitCode`.
5. Dựng kết quả: `{ ...result, stepId }` rồi thêm theo điều kiện: `applied` (task 04), `changes` (khi `reportChanges`), `parsed` (khi có nonce), `truncated` (task 05), `warnings` (khi không rỗng). Gọi `fitResultToFrame` (task 05) CUỐI CÙNG. Không thêm khoá nào khi tham số tương ứng vắng.
6. Handler stream: nếu `reportChanges`, chụp trước/sau như trên. Nếu `resultBlockNonce`, gom thêm đuôi `stdout` bằng một `BoundedOutputBuffer` (chỉ để phân tích; chunk gửi đi vẫn đầy đủ, KHÔNG cắt). Ở `stream.end` gửi `{ type: 'stream.end', exitCode, ...(parsed), ...(changes), ...(truncated), ...(warnings), ...(applied) }` cho cả nhánh `close` và nhánh hết giờ. Vì chụp "sau" là bất đồng bộ, nhánh `close` phải `await` nó trước khi gửi `stream.end`; giữ cờ `settled` để chỉ gửi đúng một `stream.end`.
7. `truncated` ở stream chỉ nói về bộ đệm phân tích; ghi chú ngắn trong mã (lý do: chunk gửi đầy đủ nên người nhận stream tự có toàn bộ).
8. Cập nhật chú thích đầu hai hàm bằng một dòng nêu tham số mới (không viết lại cơ chế).

## Kiểm thử

Thêm vào `agent-print-mode-exec.test.ts` (mock `spawn` như hiện có; `vi.mock('./agent-worktree-change-snapshot', ...)` trả snapshot dựng sẵn để không cần Git):
- `adds parsed.ok=true with the block value when stdout ends with a correct nonce block`.
- `adds parsed.ok=false RESULT_BLOCK_MISSING when the nonce in stdout is wrong`.
- `does not add parsed when resultBlock is absent`.
- `rejects a malformed nonce with INVALID_RESULT_BLOCK_NONCE before spawning` (tích hợp với task 01).
- `adds changes when reportChanges is true and omits changes otherwise`.
- `captures the after-snapshot even when the process times out`.
- `adds READONLY_VIOLATION when readonly and changedFiles is not empty`; `... when headMoved`; `does not add it when changedFiles is empty`.
- `does not roll back or alter exitCode on READONLY_VIOLATION`.
- `returns changes.available=false NOT_A_GIT_REPO outside a repo and does not throw`.
- Stream: `stream.end carries parsed and changes`, `timeout path still sends exactly one stream.end with exitCode -1 and the after-snapshot changes`, `stream chunks stay complete when the analysis buffer is truncated`.
- Hồi quy: mọi `it` cũ xanh; test `legacy result shape` của task 04 vẫn đúng (không thêm khoá khi không tham số).
- Tích hợp với Git thật (một `it` duy nhất, `describe.skipIf(!gitAvailable)`): chạy handler với `spawn` giả mà trong lúc "chạy" ghi một file vào repo tạm; khẳng định `changes.changedFiles` chứa file đó.

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run src/relay/agent-print-mode-exec.test.ts`; rồi `pnpm test`.

Kiểm với `claude` thật (CHƯA CHẠY): trên dev server, gọi `agent.execPrompt` với prompt yêu cầu in đúng khối với nonce được cấp; ghi lại `parsed` và so với `stdout`. Chỉ dùng tham số CR-033 đã nêu; không thêm cờ CLI.

## Tiêu chí hoàn thành

- [x] `parsed` có mặt đúng khi và chỉ khi có `resultBlock`; mã lỗi `RESULT_BLOCK_*` đúng.
- [x] `changes` đúng cho thêm, sửa, xoá, commit; `available:false` ngoài repo Git.
- [x] `READONLY_VIOLATION` xuất hiện đúng và không đổi `exitCode`.
- [x] Handler stream gửi đúng một `stream.end` mang các trường mới, kể cả khi hết giờ.
- [x] Kết quả khi không có tham số mới giống hệt trước (test hồi quy).

## Rủi ro và lưu ý

- Chụp "sau" làm trễ phản hồi tới 30 giây trên đường hết giờ; backend phải cộng thêm vào timeout phía gọi (task-service đặt timeout gọi relay; chưa đọc lại giá trị).
- Backend Go phải đọc `stream.end` có thêm trường: chưa xác nhận decoder của `devserveragent` bỏ qua trường lạ ở khung `stream.end` (đã thấy `isTerminalStreamResponse` quyết định kết thúc nhưng chưa đọc phần bóc `result`); kiểm khi phối hợp với người làm infra-fleet.
- Hai handler vẫn có nhiều đoạn lặp; việc dọn thuộc CR khác.
- `parsed` phân tích trên phần đuôi bị cắt: backend không được giả định mọi lần chạy cũ giống nhau khi `maxOutputBytes` nhỏ hơn đầu ra thật; nên giữ mặc định 4 MiB cho tác vụ cần khối.
- Backend cũ gọi agent mới: không gửi tham số mới thì không đổi gì (trừ giới hạn 4 MiB ở task 05). Backend mới gọi agent cũ: không có `parsed`, `changes`; backend tự phân tích `stdout` và dùng kiểm repo trước/sau (CR-REQ-008, 029).

## Không làm trong task này

- Không kiểm schema nội dung `parsed.value` (việc của backend, CR-REQ-029).
- Không hoàn tác khi `READONLY_VIOLATION`.
- Không đổi hành vi `notify` và các frame `stream.chunk`.
- Không thêm `agent.execPrompt.changes` hay `agent.execPrompt.resultBlock` vào `features` (task 12).

## Thứ tự thực hiện gợi ý

1. Viết test `parsed` cho handler thường (đơn giản nhất), rồi nối.
2. Viết test `changes` với snapshot giả, rồi nối; sau đó một `it` với Git thật.
3. Thêm `READONLY_VIOLATION`.
4. Làm handler stream cuối cùng vì có hai nhánh kết thúc (`close` và hết giờ) và phải bảo đảm đúng một `stream.end`.

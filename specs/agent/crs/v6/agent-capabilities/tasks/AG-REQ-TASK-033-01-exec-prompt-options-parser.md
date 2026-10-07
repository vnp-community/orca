# AG-REQ-TASK-033-01: Bộ đọc và kiểm tham số mới của `agent.execPrompt` (`parseExecPromptOptions`)

**From Solution:** [AG-REQ-SOL-033-exec-prompt-readonly-and-workspace](../solutions/AG-REQ-SOL-033-exec-prompt-readonly-and-workspace.md) mục 2.2
**Priority:** P1
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/agent-exec-prompt-options.ts` (mới), `agent/src/relay/agent-exec-prompt-options.test.ts` (mới), `agent/src/relay/agent-print-mode-exec.ts` (sửa, chỉ thêm lời gọi ở đầu hai handler)
**Depends on:** không
**Status:** [x] DONE

## Context

Đã đọc `agent-print-mode-exec.ts` (382 dòng). Hai handler `handleAgentExecPrompt` (dòng 31 trở đi) và `handleAgentExecPromptStream` (dòng 207 trở đi) đọc tham số bằng chuỗi `typeof params.x === 'string' ? ... : ''`, bỏ qua mọi khoá lạ, không bao giờ báo lỗi cho khoá sai kiểu ngoài `prompt`/`worktreePath`. Hai handler không dùng chung hàm đọc tham số: handler thường đọc thêm `taskId`, `projectId` (dòng 36 đến 40), handler stream thì không (dùng `taskId: stepId ?? ''` ở lời gọi `buildAgentEnv`). Lỗi `InvalidParams` hiện được dựng thủ công thành `{ jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message } }`; file này CỐ Ý không import `makeError` từ `agent-rpc-dispatch.ts` (tránh vòng phụ thuộc: dispatcher import động file này), nên task này cũng tự dựng đối tượng lỗi, không import dispatcher.

Task này là nền cho cả nhóm A và B: định nghĩa kiểu `ExecPromptOptions` chứa luôn các tham số của nhóm B (`reportChanges`, `resultBlock`, `maxOutputBytes`) để mọi tham số mới được kiểm ở một chỗ và nhóm B không phải mở lại file này. Tham số an toàn sai kiểu thì lỗi, không âm thầm về mặc định (lý do: `accessMode` sai mà thành `write` là hạ cấp âm thầm).

Khác biệt có sẵn cần ghi nhớ, KHÔNG sửa trong task này: handler stream không truyền `taskId`/`projectId` vào `buildAgentEnv` (câu hỏi mở 1 của solution).

## Việc cần làm

1. Tạo `agent-exec-prompt-options.ts` với các kiểu:
   ```ts
   export type ExecPromptErrorCode =
     | 'INVALID_ACCESS_MODE' | 'INVALID_WORKSPACE_KIND' | 'INVALID_REPORT_CHANGES'
     | 'INVALID_RESULT_BLOCK' | 'INVALID_RESULT_BLOCK_NONCE' | 'INVALID_MAX_OUTPUT_BYTES'
   export type ExecPromptOptions = {
     accessMode: 'write' | 'readonly'
     workspaceKind: 'worktree' | 'repo_root' | 'scratch'
     explicit: { accessMode: boolean; workspaceKind: boolean }
     reportChanges: boolean
     resultBlockNonce: string | null
     maxOutputBytes: number
   }
   ```
   và hằng `DEFAULT_MAX_OUTPUT_BYTES = 4 * 1024 * 1024`, `MAX_OUTPUT_BYTES_CEILING = 12 * 1024 * 1024`, `MIN_OUTPUT_BYTES = 64 * 1024`, `RESULT_NONCE_PATTERN = /^[A-Za-z0-9]{16,64}$/`.
2. Cài `parseExecPromptOptions(params): { ok: true; value } | { ok: false; error: { code; message } }` theo bảng:
   | Khoá | Vắng | Hợp lệ | Sai |
   |---|---|---|---|
   | `accessMode` | `write`, `explicit.accessMode=false` | `'write'`, `'readonly'` | bất kỳ giá trị khác (kể cả `'READONLY'`, `null`, số): `INVALID_ACCESS_MODE` |
   | `workspaceKind` | `worktree` | `'worktree'`, `'repo_root'`, `'scratch'` | `INVALID_WORKSPACE_KIND` |
   | `reportChanges` | `false` | boolean | chuỗi, số: `INVALID_REPORT_CHANGES` |
   | `resultBlock` | `resultBlockNonce = null` | đối tượng (không mảng) có `nonce` khớp `RESULT_NONCE_PATTERN` | không phải đối tượng: `INVALID_RESULT_BLOCK`; `nonce` sai: `INVALID_RESULT_BLOCK_NONCE` |
   | `maxOutputBytes` | `DEFAULT_MAX_OUTPUT_BYTES` | số nguyên `>= MIN_OUTPUT_BYTES`; lớn hơn trần thì KẸP về trần | không nguyên, `NaN`, nhỏ hơn sàn, không phải số: `INVALID_MAX_OUTPUT_BYTES` |
   Giá trị `null` cho một khoá tuỳ chọn được coi là SAI KIỂU (không coi là vắng), trừ khi khoá hoàn toàn không có trong `params` (`!(key in params)`); lý do: backend Go có thể marshal `null` cho trường rỗng, cần tín hiệu rõ ràng. Nếu khi viết thấy backend `SpawnAgent` (`agent_methods.go`) dùng `omitempty` thì giữ nguyên quy tắc này; nếu không, chốt lại với backend (xem Rủi ro).
3. Thêm `toExecPromptErrorResponse(method: 'agent.execPrompt' | 'agent.execPromptStream', id, error)` trả `{ jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: \`${method}: ${error.message} (${error.code})\`, data: { reason: error.code } } }`. `message` chứa chuỗi mã (theo quy ước `UNSUPPORTED_MODEL_FOR_ONE_SHOT_EXEC` hiện có) và `data.reason` để máy đọc.
4. Trong `handleAgentExecPrompt`, ngay sau khối kiểm `!prompt || !worktreePath` và trước `resolveAgentSpec`: gọi `parseExecPromptOptions(params)`; lỗi thì `span.fail(code)` và trả `toExecPromptErrorResponse`. Lưu `options` vào biến cục bộ (chưa dùng; task 04 và 08 dùng). Làm tương tự trong `handleAgentExecPromptStream` nhưng gửi bằng `sendFrame(ws, wireState, ...)` rồi `return`, không có `stream.chunk` nào.
5. Chưa đổi `args`, `env` hay kết quả. Hành vi khi không có tham số mới phải y hệt hiện nay.

## Kiểm thử

Trong `agent-exec-prompt-options.test.ts` (vitest, không mock):
- `returns defaults when no new param is present` (kiểm `explicit` đều `false`, `maxOutputBytes === DEFAULT_MAX_OUTPUT_BYTES`).
- `accepts accessMode write and readonly and marks explicit`.
- `rejects accessMode with wrong case, null, number and empty string` (bảng `it.each`).
- `rejects workspaceKind outside the three allowed values`.
- `rejects reportChanges given as string "true"`.
- `rejects resultBlock that is an array or string`; `rejects nonce shorter than 16, longer than 64, or containing "-" or space`.
- `clamps maxOutputBytes above the ceiling to 12 MiB`; `rejects maxOutputBytes below 64 KiB, fractional, NaN and string`.
- `toExecPromptErrorResponse puts the code both in message and in error.data.reason`.

Trong `agent-print-mode-exec.test.ts` thêm hai `it` trong mỗi `describe`: `returns InvalidParams INVALID_ACCESS_MODE before spawning` (kiểm `spawnMock` không được gọi) và bản stream (một frame lỗi duy nhất, giải mã bằng `decodeFrame` như các test stream hiện có).

Lệnh (trong `/opt/repos/orca/agent`):
`pnpm exec vitest run src/relay/agent-exec-prompt-options.test.ts src/relay/agent-print-mode-exec.test.ts`
Rồi `pnpm test` để chắc không hỏng test khác. Kiểm kiểu: `npx tsc --noEmit` và chỉ so sánh với trước khi sửa (tài liệu v4 ghi 53 lỗi cũ ở file test khác; chưa chạy lại).

## Tiêu chí hoàn thành

- [x] Gọi `agent.execPrompt` và `agent.execPromptStream` không tham số mới cho kết quả và frame y hệt trước (mọi `it` cũ của `agent-print-mode-exec.test.ts` xanh, không sửa).
- [x] Mọi giá trị sai ở bảng bước 2 trả `InvalidParams -32602` với `error.data.reason` đúng mã và không gọi `spawn`.
- [x] Không có import từ `agent-rpc-dispatch.ts` trong `agent-exec-prompt-options.ts` (tránh vòng).
- [x] File mới không dùng tên `helpers`, `utils`, `common`; không thêm `max-lines` disable.
- [x] `agent-print-mode-exec.ts` chỉ tăng vài chục dòng.

## Rủi ro và lưu ý

- Quy tắc "`null` là sai kiểu" có thể làm vỡ backend nếu `SpawnAgent`-kiểu marshal `null` cho trường rỗng; phải khớp với người viết client Go gọi `agent.execPrompt` (hiện ở `task-service` `SimpleExecutor`, chưa đọc lại trong đợt này). Nếu nghi ngờ, chấp nhận `null` như vắng CHỈ cho `reportChanges`, `resultBlock`, `maxOutputBytes`, KHÔNG cho `accessMode` và `workspaceKind`.
- Một cách đọc `maxOutputBytes` khác (kẹp, không lỗi, ở cả hai đầu) đơn giản hơn nhưng giấu lỗi cấu hình; solution chọn lỗi ở đầu thấp, kẹp ở đầu cao.
- Tham số `resultBlock` ở dạng `{ nonce }` là tên theo CR-033 2.1; CR-REQ-029 Q4 yêu cầu xác nhận tên: task này CHỐT `resultBlock.nonce`, `reportChanges`, `maxOutputBytes`, `accessMode`, `workspaceKind` (phía backend khớp theo).
- Khi tách đường dẫn kết quả giữa handler thường và handler stream, đừng gộp hai handler thành một trong task này (ngoài phạm vi, tăng rủi ro hồi quy).

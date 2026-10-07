# AG-REQ-TASK-033-05: Bộ đệm đầu ra có giới hạn và tham số `maxOutputBytes` cho handler thường

**From Solution:** [AG-REQ-SOL-033-result-block-changes-and-output-cap](../solutions/AG-REQ-SOL-033-result-block-changes-and-output-cap.md) mục 2.2
**Priority:** P1
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/agent-bounded-output-buffer.ts` (mới), `agent/src/relay/agent-bounded-output-buffer.test.ts` (mới), `agent/src/relay/agent-print-mode-exec.ts` (sửa, chỉ `handleAgentExecPrompt`)
**Depends on:** [01](./AG-REQ-TASK-033-01-exec-prompt-options-parser.md) (đã đọc và kiểm `maxOutputBytes`)
**Status:** [x] DONE

## Context

Đã đọc `agent-print-mode-exec.ts`: handler thường cộng dồn `stdout += chunk` và `stderr += chunk` (chunk là `d.toString('utf8')` của từng `Buffer`), không giới hạn, trong khi `MAX_TIMEOUT_MS` là 15 phút. Khung truyền tối đa 16 MiB: đã xác nhận `MAX_MESSAGE_SIZE = 16 * 1024 * 1024` ở `packages/dev-agent-transport/src/relay-protocol.ts:21`; kích thước tính SAU khi `JSON.stringify` thoát ký tự (dấu nháy và xuống dòng thành 2 byte, ký tự điều khiển tới 6 byte). Kết quả lớn hơn mức đó chưa rõ xử lý ra sao (CR mục 1.2 ghi chưa kiểm chứng).

Quyết định của solution B: ngưỡng `maxOutputBytes` là TỔNG `stdout` + `stderr` giữ lại, chia 3/4 cho `stdout` và 1/4 cho `stderr`; giữ PHẦN CUỐI của mỗi luồng (khối kết quả nằm cuối `stdout`); mặc định 4 MiB, trần 12 MiB, sàn 64 KiB (kiểm ở task 01). Dấu hiệu `truncated: { stdout: boolean, stderr: boolean }` chỉ có khi có luồng bị cắt, vắng nghĩa là không cắt. Chunk `notify` (`agent.execPrompt.output`) vẫn gửi đầy đủ (không đổi). Người gọi cũ không gửi tham số lần đầu bị giới hạn 4 MiB: đổi hành vi có chủ ý, vì kết quả lớn hơn khung 16 MiB vốn không đi qua được.

Lỗi có sẵn: mỗi chunk decode UTF-8 riêng nên ký tự nhiều byte bị cắt giữa hai chunk bị hỏng. Bộ đệm mới giữ `Buffer` thô và chỉ decode một lần cuối nên sửa luôn lỗi này cho phần kết quả (không cho `notify`).

## Việc cần làm

1. Tạo `agent-bounded-output-buffer.ts`:
   ```ts
   export class BoundedOutputBuffer {
     constructor(maxBytes: number)
     append(chunk: Buffer): void       // giữ tối đa maxBytes byte CUỐI
     get truncated(): boolean          // đã bỏ ít nhất một byte đầu
     get byteLength(): number
     toString(): string                // decode utf8; bỏ các byte tiếp nối (0b10xxxxxx) mở đầu nếu bị cắt giữa ký tự
   }
   ```
   Cài bằng danh sách `Buffer[]` và tổng byte; khi vượt thì `shift` hoặc `subarray` từ đầu, không nối chuỗi. `append` với chunk lớn hơn `maxBytes` thì giữ `chunk.subarray(chunk.length - maxBytes)`.
2. Thêm `splitOutputBudget(total: number): { stdoutBytes: number; stderrBytes: number }` trả `stdoutBytes = Math.floor(total * 3 / 4)`, `stderrBytes = total - stdoutBytes`.
3. Thêm `fitResultToFrame(result, frameLimitBytes = 15 * 1024 * 1024)` (cùng file hoặc `agent-bounded-output-buffer.ts`; KHÔNG đặt tên `utils`): nếu `Buffer.byteLength(JSON.stringify(result))` vượt giới hạn thì cắt thêm 25% đầu của `stdout`, rồi `stderr`, lặp tối đa 8 lần cho tới khi vừa; đặt cờ `truncated` tương ứng. Lý do: 12 MiB văn bản nhiều dấu nháy có thể nở gần gấp đôi sau `JSON.stringify`. Hàm giữ nguyên mọi trường khác (`exitCode`, `timedOut`, `stepId`, `parsed`, `changes`).
4. Trong `handleAgentExecPrompt`: thay `let stdout = ''`/`let stderr = ''` bằng hai `BoundedOutputBuffer` dựng theo `splitOutputBudget(options.maxOutputBytes)`. Trong `child.stdout?.on('data', (d: Buffer) => ...)` gọi `stdoutBuf.append(d)` và GIỮ nguyên `notify?.(... chunk ...)` với `d.toString('utf8')`. Các điểm `finish({ stdout, stderr, ... })` đổi thành `stdout: stdoutBuf.toString()`; nhánh `child.on('error')` giữ `stderr: err.message` như cũ.
5. Sau khi `finish`, dựng kết quả như cũ rồi: nếu `stdoutBuf.truncated || stderrBuf.truncated` thì thêm `truncated: { stdout, stderr }`; gọi `fitResultToFrame` trước khi trả. Không thêm `truncated` khi không cắt.
6. KHÔNG đổi handler stream trong task này (không cộng dồn, chunk gửi đầy đủ); task 08 thêm bộ đệm đuôi cho stream khi cần `resultBlock`.

## Kiểm thử

`agent-bounded-output-buffer.test.ts` (không mock):
- `keeps everything under the limit and reports truncated=false`.
- `drops the oldest bytes and keeps the tail when over the limit`.
- `keeps only the tail of a single chunk larger than the limit`.
- `does not return a broken leading character when the cut falls inside a multibyte UTF-8 sequence` (chuỗi tiếng Việt, ví dụ "Việt Nam đẹp lắm", cắt ở từng vị trí 1..N, khẳng định `toString()` không chứa `�` ở đầu).
- `decodes a multibyte character split across two appended chunks correctly` (hai chunk chia giữa ký tự `ệ`).
- `splitOutputBudget gives 3/4 to stdout and the rest to stderr and sums to the total`.
- `fitResultToFrame leaves a small result untouched` và `shrinks stdout first when JSON escaping pushes it over the frame limit` (kết quả có 14 MiB dấu `"`, khẳng định `Buffer.byteLength(JSON.stringify(out)) <= 15 MiB` và `truncated.stdout === true`).

`agent-print-mode-exec.test.ts` (mock `spawn`, `FakeChild`): thêm vào `describe('handleAgentExecPrompt')`:
- `returns the exact legacy result shape for small output when maxOutputBytes is omitted` (không có khoá `truncated`).
- `keeps only the tail of stdout when output exceeds maxOutputBytes and sets truncated.stdout` (`maxOutputBytes: 64 * 1024`, đẩy 200 KiB qua `child.stdout.emit('data', Buffer)`).
- `still emits every chunk through notify even when the returned stdout is truncated`.
- `rejects maxOutputBytes below 64 KiB with INVALID_MAX_OUTPUT_BYTES` (đã có ở task 01; chỉ kiểm tích hợp).

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run src/relay/agent-bounded-output-buffer.test.ts src/relay/agent-print-mode-exec.test.ts`; rồi `pnpm test`.

## Tiêu chí hoàn thành

- [x] `stdout` vượt `maxOutputBytes` bị cắt, `truncated.stdout === true`, phản hồi JSON dưới 16 MiB (test `fitResultToFrame`).
- [x] Không có `truncated` khi không cắt; kết quả cũ không đổi (test hồi quy xanh).
- [x] `notify` vẫn nhận mọi chunk đầy đủ.
- [x] Ký tự UTF-8 nhiều byte ở ranh giới chunk và ranh giới cắt không bị hỏng.
- [x] Không có chuỗi cộng dồn không giới hạn nào còn lại trong `handleAgentExecPrompt`.

## Rủi ro và lưu ý

- Thay đổi hành vi cho người gọi cũ có đầu ra giữa 4 MiB và 15 MiB: trước đó đi qua nguyên vẹn, nay bị cắt còn 4 MiB. Cần báo `task-service` (người gọi `SimpleExecutor`) và ghi vào ghi chú phát hành; nếu muốn bảo toàn tuyệt đối thì đổi mặc định thành "không giới hạn dưới trần khung" (câu hỏi mở cho CR-REQ-033).
- `fitResultToFrame` tốn thêm một lần `JSON.stringify` trên kết quả lớn (vài chục ms cho vài MiB); chấp nhận, chưa đo.
- Khung thực tế còn thêm phần bọc JSON-RPC (`jsonrpc`, `id`, `result`) và tiêu đề khung; mốc 15 MiB để dư chỗ.
- Không đổi hành vi của `notify`: chunk của một tiến trình ồn ào vẫn có thể làm nghẽn WS; ngoài phạm vi.

## Không làm trong task này

- Không cắt chunk `agent.execPrompt.output` đã gửi qua `notify`.
- Không thêm bộ đệm vào handler stream (task 08 làm khi cần `resultBlock`).
- Không đổi `timeoutMs` hay các hằng `DEFAULT_TIMEOUT_MS`, `MAX_TIMEOUT_MS`.

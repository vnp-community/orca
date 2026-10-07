# AG-REQ-TASK-033-06: Bộ phân tích khối kết quả `ORCA_RESULT_BEGIN/END <nonce>` (`agent-result-block-parser.ts`)

**From Solution:** [AG-REQ-SOL-033-result-block-changes-and-output-cap](../solutions/AG-REQ-SOL-033-result-block-changes-and-output-cap.md) mục 2.3
**Priority:** P1
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/agent-result-block-parser.ts` (mới), `agent/src/relay/agent-result-block-parser.test.ts` (mới)
**Depends on:** không (hàm thuần; task 08 nối vào handler)
**Status:** [x] DONE

## Context

CR-REQ-033 mục 2.5 và CR-REQ-029 mục 2.5 thống nhất khuôn dạng:

```
ORCA_RESULT_BEGIN <nonce>
{ ...một đối tượng JSON... }
ORCA_RESULT_END <nonce>
```

Backend sinh `nonce` ngẫu nhiên mỗi lần chạy, đưa vào prompt và vào `resultBlock.nonce`. Agent chỉ nhận khối có đúng `nonce`; khối chứa sẵn trong nội dung Request (Jira, GitHub) mà không có nonce đúng thì bị bỏ qua (chống chèn chỉ dẫn qua nội dung ngoài, xem CR-REQ-035 bộ test đối kháng có "khối `ORCA_RESULT_BEGIN` giả"). Agent KHÔNG kiểm schema nội dung: schema theo `kind` và TaskSpec do backend kiểm (CR-REQ-029). Kết quả được thêm vào `result` của RPC với tên `parsed` (không dùng `result` để khỏi lồng `result.result`).

Mã lỗi theo CR: `RESULT_BLOCK_MISSING`, `RESULT_BLOCK_INVALID_JSON`, `RESULT_BLOCK_TOO_LARGE`, `RESULT_BLOCK_NOT_OBJECT`; tối đa 256 KiB cho phần giữa. Backend cũ hơn (agent cũ không có `parsed`) tự tìm khối trong `stdout` bằng cùng thuật toán (CR-REQ-029 mục 2.5), nên thuật toán dưới đây là hợp đồng dùng chung hai phía: mọi chi tiết phải đúng từng chữ để hai bên cho cùng kết quả. File này là hàm thuần (không `fs`, không `process`, không log nội dung) để dễ dùng làm bộ test vàng cho phía Go.

## Việc cần làm

1. Tạo `agent-result-block-parser.ts` với:
   ```ts
   export const RESULT_BLOCK_MAX_BYTES = 256 * 1024
   export type ResultBlockErrorCode =
     | 'RESULT_BLOCK_MISSING' | 'RESULT_BLOCK_INVALID_JSON'
     | 'RESULT_BLOCK_TOO_LARGE' | 'RESULT_BLOCK_NOT_OBJECT'
   export type ParsedResult =
     | { ok: true; value: Record<string, unknown> }
     | { ok: false; code: ResultBlockErrorCode; detail: string }
   export function parseResultBlock(stdout: string, nonce: string): ParsedResult
   ```
2. Thuật toán chính xác:
   1. `lines = stdout.split('\n')` rồi bỏ một `\r` cuối mỗi dòng.
   2. `beginMarker = 'ORCA_RESULT_BEGIN ' + nonce`, `endMarker = 'ORCA_RESULT_END ' + nonce`. Một dòng là đánh dấu hợp lệ khi `line.trim() === marker` (so BẰNG, không tìm chuỗi con; nonce khác, thiếu nonce, hoặc có chữ thừa thì không hợp lệ).
   3. `endIndex` = chỉ số dòng END hợp lệ CUỐI CÙNG. Không có thì `RESULT_BLOCK_MISSING`.
   4. `beginIndex` = chỉ số dòng BEGIN hợp lệ GẦN NHẤT phía trước `endIndex`. Không có thì `RESULT_BLOCK_MISSING`. (Nếu có nhiều BEGIN liên tiếp, dùng cái gần END nhất.)
   5. `body = lines.slice(beginIndex + 1, endIndex).join('\n')`. Nếu `Buffer.byteLength(body, 'utf8') > RESULT_BLOCK_MAX_BYTES` thì `RESULT_BLOCK_TOO_LARGE`, kiểm TRƯỚC `JSON.parse`; `detail` là số byte.
   6. `JSON.parse(body)`; lỗi thì `RESULT_BLOCK_INVALID_JSON` với `detail = error.message.slice(0, 200)` (KHÔNG chép nội dung khối vào `detail`).
   7. Kết quả `null`, mảng, số, chuỗi, boolean thì `RESULT_BLOCK_NOT_OBJECT`; còn lại `{ ok: true, value }`.
3. Không ném lỗi ra ngoài (mọi nhánh trả `ParsedResult`). Nonce rỗng hoặc sai khuôn thì hàm vẫn không ném (task 01 đã chặn nonce sai ở biên RPC); nonce rỗng cho `RESULT_BLOCK_MISSING`.
4. Giữ độ phức tạp tuyến tính: không dùng regex có thể quay lui trên chuỗi lớn; duyệt dòng một lần từ cuối để tìm END rồi lùi tìm BEGIN.
5. Không xuất mảng ca mẫu từ mã sản phẩm; đặt các ca vàng trong chính file test dưới dạng bảng `it.each` để phía Go sao chép khi viết bộ phân tích tương ứng (ghi chú trong test: "ca vàng dùng chung với backend").

## Kiểm thử

`agent-result-block-parser.test.ts` (vitest, không mock). Ca bảng (`it.each`), mỗi ca ghi `stdout` mẫu và kỳ vọng:
- `parses a well formed block with the right nonce` (đối tượng `{"summary":"x","files":["a"]}`).
- `uses the LAST pair when the model prints the format twice` (khối đầu `{"a":1}`, khối cuối `{"a":2}` thì `a === 2`).
- `ignores a block whose nonce differs` thì `RESULT_BLOCK_MISSING`.
- `ignores BEGIN without a nonce (injected from untrusted content)` thì `RESULT_BLOCK_MISSING`.
- `ignores a marker embedded in the middle of a sentence` ("xem ORCA_RESULT_BEGIN <nonce> ở trên") thì `RESULT_BLOCK_MISSING`.
- `a forged block placed BEFORE the real one does not win` (giả mạo nonce sai ở trước, thật ở sau thì lấy thật).
- `a forged block with the right nonce can only come from the model, still parsed` (ghi nhận hành vi: ai biết nonce thì giả được; nonce phải bí mật với nội dung ngoài).
- `handles CRLF line endings`.
- `handles Vietnamese text and emoji inside strings` ("Đã sửa 3 tệp, ổn ✅").
- `rejects invalid JSON with RESULT_BLOCK_INVALID_JSON and a detail that does not echo the body`.
- `rejects arrays, null, numbers and strings with RESULT_BLOCK_NOT_OBJECT`.
- `rejects a body larger than 256 KiB with RESULT_BLOCK_TOO_LARGE before parsing` (thân 300 KiB, không JSON hợp lệ vẫn ra `TOO_LARGE`).
- `accepts a body of exactly 256 KiB` (biên).
- `returns RESULT_BLOCK_MISSING for empty stdout, END only, BEGIN only`.
- `END before BEGIN is missing` (thứ tự đảo).
- `handles a 5 MiB stdout in linear time` (đo thô: dưới 500 ms; chỉ khẳng định kết quả đúng, không khẳng định thời gian chặt, để khỏi flaky).

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run src/relay/agent-result-block-parser.test.ts`; rồi `pnpm test`.

## Tiêu chí hoàn thành

- [x] Nonce đúng thì phân tích; nonce sai, thiếu, đánh dấu giữa câu thì bị bỏ qua.
- [x] Bốn mã lỗi đúng theo bảng CR; `TOO_LARGE` kiểm trước `JSON.parse`.
- [x] Khối cuối thắng khi có nhiều khối.
- [x] `detail` không chứa nội dung khối (test khẳng định bằng chuỗi nhận diện).
- [x] Hàm thuần, không import `fs`/`os`/`process`; không log.

## Rủi ro và lưu ý

- "Chống giả mạo" chỉ đúng chừng nào `nonce` không lọt vào nội dung ngoài: backend phải tạo nonce mới mỗi lần chạy và KHÔNG chép vào prompt phần nội dung nào mà mô hình có thể trích lại từ nguồn ngoài; nội dung ngoài không nên chứa nonce vì nó sinh sau.
- Mô hình có thể in khối trong code fence (```). Dòng có "```" không phải đánh dấu hợp lệ nên khối nằm TRONG hàng của dòng đánh dấu vẫn đúng, miễn dòng đánh dấu đứng riêng một dòng; nếu mô hình in `ORCA_RESULT_BEGIN <nonce>` ngay sau "```json" trên cùng dòng thì không khớp. Đó là việc của prompt backend (CR-REQ-029); ghi vào câu hỏi mở nếu thực nghiệm cho thấy hay gặp.
- Ngữ nghĩa phải trùng phía Go (backend tự phân tích khi gặp agent cũ). Mọi thay đổi thuật toán phải đổi cả hai nơi; ca vàng trong test là điểm đối chiếu.
- Không cố sửa JSON hỏng (ví dụ dấu phẩy thừa): sửa là mở rộng bề mặt tấn công và che lỗi của mô hình.

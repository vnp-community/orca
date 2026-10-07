# AG-CV-TASK-001-01: Mã lỗi `CODEINTEL_*` và kiểm tham số chặt

**From Solution:** [AG-CV-SOL-001-codeintel-agent-foundation](../solutions/AG-CV-SOL-001-codeintel-agent-foundation.md) mục 2.2, 2.3
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/codeintel-errors.ts` (mới), `agent/src/relay/codeintel-params-validation.ts` (mới), `agent/src/relay/codeintel-errors.test.ts` (mới), `agent/src/relay/codeintel-params-validation.test.ts` (mới)
**Depends on:** không
**Status:** [x] DONE

## Context

Hợp đồng `CONTRACT-codeintel-agent-rpc.md` §3.1–3.2 chốt một bảng mã lỗi duy nhất và §2.1 chốt quy tắc tham số (khoá lạ bị từ chối, chuỗi không bắt đầu `-`, đường dẫn tương đối). Đã đọc `shared/agent-wire-protocol.ts:35-52` (`AgentErrorCode`: `-32602`, `-33002`, `-32000`; không thêm số mới) và `agent-rpc-dispatch.ts:408-419` (`makeError` giữ `data`). PQ-03 tách `CODEINTEL_SYMBOL_NOT_FOUND` khỏi `INVALID_PARAMS`.

Lõi codeintel phải trung lập truyền tải (CR-CV-006): module lỗi **không** import `makeError`; chỉ trả tải trọng thuần `{code, message, data}` để Part A (`makeError`) và Part B (`Object.assign(new Error, {code, data})`) tự bọc.

## Việc cần làm

1. `codeintel-errors.ts`: `CodeIntelErrorCode` đủ 16 mã (liệt kê ở solution 2.2), `class CodeIntelError(code, message, data?)`, `toErrorPayload(err)`; bảng ánh xạ `data.code` -> `error.code` (`INVALID_PARAMS|SYMBOL_NOT_FOUND|PROFILE_UNKNOWN|RUN_NOT_FOUND` -> `-32602`; `PATH_NOT_ALLOWED` -> `-33002`; còn lại -> `-32000`) dùng hằng của `AgentErrorCode`. `message` cắt 300 ký tự, bỏ stack. Lỗi không phải `CodeIntelError` -> `CODEINTEL_TOOL_FAILED` không rò `err.message` chứa đường dẫn: chỉ thông điệp chung `"internal error"` và log đầy đủ ở dispatcher (task 09).
2. `codeintel-params-validation.ts`: `validateCodeIntelParams(params, spec)` với `spec` là bảng `{ [name]: {kind:'string'|'int'|'bool'|'stringList'|'enum'|'object', required?, min?, max?, default?, enum?, maxLen?} }`; luôn thêm `workspaceRoot` (bắt buộc) và cho phép `_trace`. Khoá lạ -> `INVALID_PARAMS data{field}`. Hàm phụ: `assertSafeClientString(v, field)` (1..512, không NUL/điều khiển, chuẩn hoá NFKC rồi cấm bắt đầu `-`, U+FF0D, U+2212), `assertRelativeRepoPath(v, field)` (không tuyệt đối, không `..`, không `\`), `assertGitRef(v, field)` (1..256, `[A-Za-z0-9._/@^~{}+-]`, không bắt đầu `-`).
3. `workspaceRoot`: kiểm hình dạng ở đây (kiểu string, `path.isAbsolute`, không NUL, <= 4096); `realpath` và allowed roots thuộc task 06.
4. Xuất `FORBIDDEN_PARAM_NAMES = ['args','argv','command','cmd','cwd','env','repo','cypher','shell','timeout','tool']` và `assertSchemaHasNoForbiddenParams(spec)` để test phản chiếu schema của mọi method (contract §2.1, CR-072 §2.2) dùng lại.

## Kiểm thử

`codeintel-errors.test.ts`: bảng ánh xạ 16 mã; `message` > 300 bị cắt; `toErrorPayload(new Error('x /home/u/y'))` không chứa đường dẫn; `data.code` luôn có mặt.
`codeintel-params-validation.test.ts`: thiếu `workspaceRoot`; khoá lạ (`foo`, `args`, `cwd`); `_trace` được phép; chuỗi `-x`, `－x`, `−x`, NUL, `\n`, 513 ký tự; đường dẫn `../a`, `/a`, `a\b`; ref `-x`, `a b`; `assertSchemaHasNoForbiddenParams` ném khi spec có `cwd`.

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run src/relay/codeintel-errors.test.ts src/relay/codeintel-params-validation.test.ts`

## Tiêu chí hoàn thành

- [x] 16 mã khai đủ; số `error.code` đúng bảng; không thêm số mới vào `AgentErrorCode`.
- [x] Module lỗi không import `agent-rpc-dispatch`, `ws`, `orca-dev-agent-transport` (test quét import).
- [x] Mọi giá trị chuỗi bắt đầu `-` (kể cả U+FF0D, U+2212 sau NFKC) bị từ chối.
- [x] Khoá lạ trả `INVALID_PARAMS` với `data.field` đúng tên khoá.

## Rủi ro và lưu ý

- NFKC có thể biến ký tự khác thành `-`: dùng chuỗi đã chuẩn hoá cho kiểm tra nhưng **giá trị đi tiếp là chuỗi gốc đã chuẩn hoá** (không dùng bản gốc chưa chuẩn hoá).
- Bảng mã dùng chung với AG-CV-SOL-081 (`PROFILE_UNKNOWN`, `ENV_NOT_READY`, `RUN_*`); không đổi tên mã sau khi merge.

# AG-CV-TASK-001-04: Mở rộng `runToolCommand` và chặn động từ ghi ở tool `gitnexus`/`codegraph` cũ

**From Solution:** [AG-CV-SOL-001-codeintel-agent-foundation](../solutions/AG-CV-SOL-001-codeintel-agent-foundation.md) mục 2.4, 3 (quyết định 5, 6)
**Priority:** P0
**Area:** `agent/`
**File:** `agent/src/relay/agent-tool-registry.ts` (sửa), `agent/src/relay/__tests__/agent-tool-registry.test.ts` (sửa)
**Depends on:** không
**Status:** [x] DONE

## Context

Đã đọc `agent-tool-registry.ts:72-113`: `runToolCommand` gom stdout/stderr vào mảng không cap, hết hạn chỉ `child.kill('SIGTERM')` rồi resolve `exitCode 124` (không `SIGKILL`, không diệt cả cây), `child.stdin?.end()` ngay. Hàm được 7 tool gọi (`claude_code`, `gh`, `git`, `gitnexus`, `codegraph`, `docker`, `shell` theo CR-002 mục 6, **chưa kiểm lại đầy đủ**), nên mọi thay đổi phải tương thích ngược. Bản `desktop/src/relay/agent-tool-registry.ts` giống hệt ở 400 dòng đầu (đã `diff`): cùng patch được áp ở SOL-006 task 05.

Tool `gitnexus` (`:184-203`) và `codegraph` (`:206-224`) nhận `args` tự do: có thể chạy `gitnexus analyze` (ghi `AGENTS.md`/`CLAUDE.md`), `clean`, `remove`. CR-001 2.9(i) đề xuất từ chối động từ ghi; chưa biết client nào gọi `tools/call` với hai tool này (CR Q3).

## Việc cần làm

1. **Impact trước khi sửa:** chạy `gitnexus impact -r /opt/repos/orca runToolCommand --direction upstream` (có hai bản `agent/` và `desktop/`: chọn uid của `agent/`); ghi blast radius vào PR; báo người duyệt nếu HIGH/CRITICAL.
2. Thêm tuỳ chọn không bắt buộc vào `opts`: `maxOutputBytes`, `stdoutFile`, `killGraceMs` (mặc định 5000 khi dùng `detached`/`signal`; không đổi hành vi khi không truyền), `signal: AbortSignal`, `detached: boolean`, `stdinText: string`.
   - `maxOutputBytes`: đếm byte (`Buffer.byteLength`) từng chunk; vượt -> `SIGTERM` nhóm, `meta.truncated = true`, resolve với phần đã có.
   - `stdoutFile`: mở fd (`fs.openSync(path, 'wx', 0o600)`) và truyền `stdio: ['pipe', fd, 'pipe']`; `stdout` trả về `''`; đóng fd trong mọi nhánh.
   - `detached` (POSIX): `spawn(..., {detached:true})`, huỷ bằng `process.kill(-child.pid, sig)`; `SIGTERM` rồi `SIGKILL` sau `killGraceMs`; Windows bỏ qua `detached` (mặc định, vì codeintel chặn Windows).
   - `signal`: `abort` -> cùng đường huỷ; không rò listener (`removeEventListener` ở `close`).
   - `stdinText`: ghi rồi `end()`; mặc định `end()` ngay như cũ.
   - `ToolResult.meta` thêm `{ truncated, timedOut, durationMs }` **chỉ khi** dùng tuỳ chọn mới (để `formatMcpResult` của `tools/call` giữ hình dạng cũ).
3. Tool `gitnexus`/`codegraph` (handler): nếu `args[0]` (sau bỏ khoảng trắng) thuộc `{analyze, clean, remove, uninstall, publish, setup, index, init, uninit, sync, serve, mcp, wiki, group, daemon, unlock, install, upgrade, telemetry, eval-server}` -> trả `{stdout:'', stderr:'tool verb not allowed over tools/call', exitCode: 2}` không spawn. **Có điều kiện:** nếu người điều phối chốt Q3 là "không chặn", bỏ bước này nhưng giữ test hiện trạng và ghi vào solution.
4. Cập nhật chú thích đầu `runToolCommand` bằng một đoạn "Why" ngắn: tuỳ chọn phục vụ `codeintel.*`, mặc định giữ nguyên cho `tools/call`.

## Kiểm thử

Sửa `__tests__/agent-tool-registry.test.ts` (đã có), thêm `describe('runToolCommand options')` dùng script Node tạm làm binary (`PATH` trong `env`): (a) hành vi mặc định giống trước (stdout/stderr, `exitCode 124` khi quá hạn, `meta` không đổi); (b) `maxOutputBytes` cắt và `meta.truncated`; (c) `stdoutFile` 5 MB rồi `process.exit(0)` ngay -> tệp đủ 5 MB, `stdout === ''`; (d) `detached` + `killGraceMs` diệt cả tiến trình con (script sinh `sleep`) -> không mồ côi; (e) `signal.abort()`; (f) `stdinText`; (g) tool `gitnexus` với `args:['analyze']` bị từ chối và **không** spawn (spy `spawn`), `args:['status']` chạy bình thường. Bỏ qua ca (d) trên Windows (`describe.skipIf(process.platform==='win32')`).

Lệnh: `pnpm exec vitest run src/relay/__tests__/agent-tool-registry.test.ts`; rồi `pnpm test` để chắc `tools/call` không hồi quy.

## Tiêu chí hoàn thành

- [x] Gọi `runToolCommand` với tham số cũ cho kết quả y hệt (test hiện có xanh không sửa).
- [x] Cả cây tiến trình bị diệt sau `killGraceMs`; không rò listener/fd.
- [x] `tools/call gitnexus analyze` bị từ chối (nếu chốt); tool khác không đổi.

## Rủi ro và lưu ý

- Hàm dùng chung 7 tool: không thêm hành vi mặc định mới. Chạy `gitnexus context`/`impact` trước khi merge.
- `detached` làm tiến trình con thoát khỏi nhóm của agent: nếu agent bị `SIGKILL`, con mồ côi tiếp tục chạy (chấp nhận; SOL-004 xử lý journal).
- Chưa kiểm chứng `process.kill(-pid)` trên macOS; Windows chưa hỗ trợ (`taskkill /T` chưa thử).
- Bản `desktop/` cần cùng patch (SOL-006 task 05); không sửa `desktop/` ở task này.

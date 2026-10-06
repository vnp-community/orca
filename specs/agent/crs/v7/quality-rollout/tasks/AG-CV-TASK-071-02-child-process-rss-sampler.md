# AG-CV-TASK-071-02: Lấy mẫu RSS tiến trình con (Linux `/proc`, macOS `ps`)

**From Solution:** [AG-CV-SOL-071-perf-block-and-bench](../solutions/AG-CV-SOL-071-perf-block-and-bench.md)
**Priority:** P1
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/codeintel/child-process-rss-sampler.ts` (mới), `agent/src/relay/codeintel/child-process-rss-sampler.test.ts` (mới)
**Depends on:** không
**Status:** [ ] TODO

## Context

Node không có `wait4`/`rusage` cho tiến trình con; chỉ đọc khi còn sống. `rssPeakKb` = đỉnh các lần lấy mẫu của tiến trình trực tiếp; vắng khi không đọc được (Windows bỏ vì method trả `unsupported_platform`).
Mọi con số hiệu năng là giả định từ CR-CV-071 (một lần đo, một máy), chưa phải phân phối.

## Việc cần làm

1. Cài `createRssSampler({pid,intervalMs=100,read,platform})` theo solution 2.3: Linux đọc `/proc/<pid>/status` (`VmHWM`, rồi `VmRSS`); macOS `execFile("ps",["-o","rss=","-p",pid])` (không shell, `intervalMs` ≥ 200); nền tảng khác → `stop()` trả `undefined`.
2. `stop()` đọc một lần cuối, dừng timer; lỗi đọc nuốt (không ném, không chặn quá `intervalMs`).
3. Tách bộ phân tích văn bản `/proc` và `ps` thành hàm thuần để test.

## Kiểm thử

- `parses VmHWM and falls back to VmRSS`
- `returns the maximum across samples`
- `returns undefined when every read fails`
- `stop clears the timer (fake timers) and never throws`
- `macOS reader invokes ps without a shell`
- `does not sample on win32`

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run <đường dẫn test>`. Chưa chạy.

## Tiêu chí hoàn thành

- [ ] Test xanh bằng reader tiêm; thử thật (chưa chạy): spawn `node -e` cấp phát 100 MiB và so sánh.
- [ ] Không thêm phụ thuộc, không `max-lines` disable, tên tệp theo khái niệm.

## Rủi ro và lưu ý

- Tài liệu ghi rõ đây là đỉnh lấy mẫu, không tuyệt đối.

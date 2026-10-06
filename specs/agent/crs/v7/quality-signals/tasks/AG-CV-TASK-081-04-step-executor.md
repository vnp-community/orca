# AG-CV-TASK-081-04: Executor một bước: spawn ra tệp, giám sát kích thước, timeout (`quality-run-step-executor.ts`)

**From Solution:** [AG-CV-SOL-081-quality-runner-core](../solutions/AG-CV-SOL-081-quality-runner-core.md) mục 5.2,5.3
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-run-types.ts` (mới), `quality-run-step-executor.ts` (mới), `.test.ts` (mới)
**Depends on:** AG-CV-TASK-081-01, 081-03
**Status:** [ ] TODO

## Context

Đã đọc `runToolCommand` (`agent-tool-registry.ts:72-113`): pipe, SIGTERM, không giới hạn. Executor mới theo solution 5.3. Parser thật do SOL-082 cắm; ở đây chỉ định nghĩa `StepParser`.

## Việc cần làm

1. Tạo `quality-run-types.ts` với `PlannedStep`, `StepExecResult`, `StepParser`, `QualityEnvMissing` đúng solution 5.2.
2. `export async function executeStep(step: PlannedStep, ctx: { runDir: string; signal: AbortSignal; heavyGate?: { acquire(signal: AbortSignal, waitMs: number): Promise<() => void> } }): Promise<StepExecResult>`.
3. Tạo thư mục run `0700` (`mkdtemp` dưới `<os.tmpdir()>/orca-quality-<uid>/`), mở `stdout`/`stderr` bằng cờ `wx` mode `0600`; spawn `shell:false`, `detached: platform !== "win32"`, `stdio:["ignore", fdOut, fdErr]`, `windowsHide:true`; `os.setPriority(pid, 10)` bọc try/catch.
4. Vòng giám sát 500 ms `fs.stat` hai tệp; tổng > `maxOutputBytes` → `killProcessTree`, trả `kind:"output_too_large"`. `timeoutMs` → `kind:"timeout"`. `signal.abort` → `kind:"cancelled"`. `spawn` lỗi (ENOENT) → `spawn_error`. Bước `heavy`: lấy cổng trước khi spawn; hết hạn chờ → `kind:"gate_timeout"` (không spawn).
5. Luôn đóng fd và dọn timer ở mọi đường thoát; không để tiến trình nào còn sau khi trả kết quả (kiểm `kill(-pid,0)`).
6. Không bao giờ đưa tên biến môi trường/argv đầy đủ vào lỗi trả ra.

## Kiểm thử

Công cụ giả = script Node qua `file: process.execPath` + argv: in nhiều MiB (output_too_large), ngủ lâu (timeout), thoát mã 3, spawn con cháu rồi bị huỷ, bỏ qua SIGTERM, binary không tồn tại. Khẳng định tệp mode `0600`, thư mục `0700`, không rò tiến trình. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-run-step-executor.test.ts`.

## Tiêu chí hoàn thành

- [ ] Mọi giá trị `kind` có test; không rò tiến trình/fd.
- [ ] Đầu ra không qua pipe; env chỉ là `step.env`.

## Rủi ro

Giám sát 500 ms có thể vượt `maxOutputBytes` một khoảng ngắn trước khi diệt (ghi trong tài liệu hàm). `os.setPriority` EPERM trong container: nuốt, log.

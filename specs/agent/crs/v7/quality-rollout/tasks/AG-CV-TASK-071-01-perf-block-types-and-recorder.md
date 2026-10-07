# AG-CV-TASK-071-01: Kiểu `CodeIntelPerf` và `PerfRecorder`

**From Solution:** [AG-CV-SOL-071-perf-block-and-bench](../solutions/AG-CV-SOL-071-perf-block-and-bench.md)
**Priority:** P1
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/codeintel/perf-block.ts` (mới), `agent/src/relay/codeintel/perf-block.test.ts` (mới)
**Depends on:** không (AG-CV-SOL-001 cắm ở task 03)
**Status:** [x] DONE

## Context

Agent-rpc §2.2: `perf{totalMs,queueWaitMs,cliCalls,cli[{tool,command,ms,stdoutBytes,rssPeakKb}],parseMs,truncated}`; `command` chỉ là hằng whitelist `cypher|context|impact|query|status|callers|callees|files|affected|detect-changes`; thay `toolTimingsMs` (PQ-19).
BE-CV-SOL-071 mục báo hợp đồng thiếu: `perf.command` không có `check`, `trace`, `list`, `analyze`, `sync` (lệnh hợp lệ khác trong whitelist §9.1). Task này **không tự mở rộng** tập: nếu runner chạy lệnh ngoài tập, ghi `command` đúng như hợp đồng cho phép hoặc bỏ mục khỏi `cli[]`; quyết định cuối chờ sửa hợp đồng (solution mục 7).
Mọi con số hiệu năng là giả định từ CR-CV-071 (một lần đo, một máy), chưa phải phân phối.

## Việc cần làm

1. Cài kiểu và `PerfRecorder`, `perfForCacheHit` theo solution 2.2; `recordCli` với `command` ngoài `CODEINTEL_CLI_COMMANDS` ném lỗi lập trình (có thông điệp rõ) — runner của 001 phải map lệnh ngoài tập sang chính sách trên.
2. Số nguyên không âm; đồng hồ tiêm được.
3. `build()` trả bản sao (không chia sẻ mảng).

## Kiểm thử

- `records queue wait, cli calls and parse time`
- `rejects a command outside the closed set`
- `perfForCacheHit has cliCalls 0 and empty cli`
- `JSON.stringify(perf) contains no path-like or symbol-like strings` (chỉ hằng + số)
- `build is idempotent and does not alias internal arrays`

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run src/relay/codeintel/perf-block.test.ts`.

## Tiêu chí hoàn thành

- [x] Test xanh; không phụ thuộc runner.
- [x] Không thêm phụ thuộc, không `max-lines` disable, tên tệp theo khái niệm.

## Rủi ro và lưu ý

- Tập `command` trùng hợp đồng §2.2 từng ký tự.

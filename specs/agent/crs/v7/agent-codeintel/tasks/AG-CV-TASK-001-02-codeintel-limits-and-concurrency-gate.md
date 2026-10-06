# AG-CV-TASK-001-02: Giới hạn tài nguyên và cổng đồng thời

**From Solution:** [AG-CV-SOL-001-codeintel-agent-foundation](../solutions/AG-CV-SOL-001-codeintel-agent-foundation.md) mục 2.4, 2.7
**Priority:** P0
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-limits.ts` (mới), `agent/src/relay/codeintel-concurrency-gate.ts` (mới), `agent/src/relay/codeintel-limits.test.ts` (mới), `agent/src/relay/codeintel-concurrency-gate.test.ts` (mới)
**Depends on:** không (dùng `CodeIntelError` của [01](./AG-CV-TASK-001-01-codeintel-errors-and-strict-params.md) khi tích hợp; có thể làm song song và nối ở task 05)
**Status:** [ ] TODO

## Context

Hợp đồng §2.3 (bảng giới hạn) và §9 mục 6 (tải): timeout một lần CLI 20 000 ms; timeout method 25 000 ms; `detectChanges`/`structuralFacts` 55 000 ms (`ORCA_CODEINTEL_DETECT_TIMEOUT_MS` hạ về 25 000 nếu infra-fleet chưa nâng); stdout 16 MiB; JSON kết quả 8 MiB; stderr 64 KiB; tiến trình đồng thời 3 (`gitnexus` <= 2, `codegraph` <= 3); hàng đợi <= 16, chờ <= 10 s quá thì `CODEINTEL_TIMEOUT data.reason="queue_wait"`; cache repo 30 s; phát hiện công cụ 60 s; `headCommit` 5 s. Giá trị sai bị bỏ và ghi cảnh báo (CR-001 2.5). PQ-13: agent luôn hết hạn trước Go (30 s / 90 s).

## Việc cần làm

1. `codeintel-limits.ts`: `CODEINTEL_DEFAULT_LIMITS` (hằng) và `readCodeIntelLimits(env: NodeJS.ProcessEnv, log)` trả `CodeIntelLimits` bất biến; tên env `ORCA_CODEINTEL_TOOL_TIMEOUT_MS`, `ORCA_CODEINTEL_METHOD_TIMEOUT_MS`, `ORCA_CODEINTEL_DETECT_TIMEOUT_MS`, `ORCA_CODEINTEL_TOOL_MAX_OUTPUT_BYTES`, `ORCA_CODEINTEL_RESULT_MAX_BYTES`, `ORCA_CODEINTEL_MAX_CONCURRENT_TOOLS`, `ORCA_CODEINTEL_QUEUE_MAX`, `ORCA_CODEINTEL_QUEUE_WAIT_MS` (đặt tên theo tiền tố hợp đồng §2.3/2.4; đây là đề xuất tên cho từng giới hạn, hợp đồng chỉ chốt tiền tố). Số không nguyên dương hoặc ngoài biên (ví dụ timeout CLI > timeout method) bị bỏ + `log.warn`. `killGraceMs` = 5000.
2. `codeintel-concurrency-gate.ts`: `createConcurrencyGate({maxTotal, perTool:{gitnexus:2, codegraph:3}, queueMax, queueWaitMs})`; `acquire(tool, signal?)` trả hàm `release`; hàng đợi FIFO, hết chỗ -> `CodeIntelError('CODEINTEL_TIMEOUT', ..., {reason:'queue_wait', elapsedMs})`; hàng đầy (16) -> cùng mã `reason:'queue_wait'` ngay lập tức (không xếp). Huỷ bằng `AbortSignal` khi đang chờ giải phóng chỗ và loại khỏi hàng. Một cổng toàn tiến trình (`getCodeIntelConcurrencyGate()`), có `resetForTests()`.
3. Giới hạn `reindex`/`quality` có cổng riêng ở các solution của chúng; cổng này chỉ cho tiến trình công cụ đọc (contract §2.3 "Tiến trình công cụ đồng thời toàn agent: 3").

## Kiểm thử

`codeintel-limits.test.ts`: mặc định đúng bảng; override hợp lệ; override sai (`abc`, `-1`, `0`, vượt biên) bị bỏ và có cảnh báo; `ORCA_CODEINTEL_DETECT_TIMEOUT_MS=25000`.
`codeintel-concurrency-gate.test.ts` (đồng hồ giả `vi.useFakeTimers`): 4 lệnh đồng thời -> 3 chạy, 1 chờ rồi chạy khi có chỗ; `gitnexus` thứ 3 chờ dù tổng < 3; chờ > 10 s -> `TIMEOUT queue_wait`; hàng đầy 17 -> lỗi ngay; huỷ khi chờ; `release` gọi hai lần không làm âm bộ đếm.

Lệnh: `pnpm exec vitest run src/relay/codeintel-limits.test.ts src/relay/codeintel-concurrency-gate.test.ts`

## Tiêu chí hoàn thành

- [ ] Bảng mặc định khớp contract §2.3 từng dòng (test so khớp).
- [ ] Không bao giờ có > 3 tiến trình đang giữ chỗ; `gitnexus` <= 2.
- [ ] Hết hạn chờ trả `CODEINTEL_TIMEOUT` với `data.reason === 'queue_wait'`.

## Rủi ro và lưu ý

- Giới hạn 3 chưa đo RAM trên dev server nhỏ (`lbug` ~1,6 GB trên đĩa); giữ làm cấu hình (O-15).
- Cổng toàn process dùng chung nhiều phiên (`--detach` có nhiều kết nối); đúng chủ ý (giới hạn theo máy).

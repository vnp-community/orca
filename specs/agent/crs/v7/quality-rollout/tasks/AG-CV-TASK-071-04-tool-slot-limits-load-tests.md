# AG-CV-TASK-071-04: Test bất biến đồng thời và hàng đợi bằng CLI giả

**From Solution:** [AG-CV-SOL-071-perf-block-and-bench](../solutions/AG-CV-SOL-071-perf-block-and-bench.md)
**Priority:** P1
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/codeintel/tool-slot-limits.load.test.ts` (mới), `agent/src/relay/codeintel/fake-codeintel-cli.ts` (mới, test-only: tạo tệp thực thi tạm)
**Depends on:** 071-03; AG-CV-SOL-001
**Status:** [x] DONE

## Context

Agent-rpc §2.3: tổng 3 tiến trình; `gitnexus` ≤ 2; `codegraph` ≤ 3; hàng đợi ≤ 16; chờ ≤ 10 s → `CODEINTEL_TIMEOUT data.reason="queue_wait"`; singleflight cache 60 s.
CR-071 nói 5 s và `RATE_LIMITED`: **hợp đồng thắng** (solution bảng Correction 1).
Biến env cho thời gian chờ chưa có trong hợp đồng (solution mục 6): test tiêm cấu hình runner thay vì env nếu 001 không có biến.

## Việc cần làm

1. `fake-codeintel-cli.ts`: ghi tệp thực thi `#!/usr/bin/env node` vào thư mục tạm; ngủ `N` ms; mỗi lần chạy tạo/xoá một tệp khoá để đếm tiến trình sống; đặt đầu `toolPath`.
2. Viết các test ở solution 2.4 (3 tổng, 2 gitnexus, 3 codegraph, hàng đợi 16, hết hạn chờ, `queueWaitMs`, singleflight).
3. Dùng `vi.useFakeTimers` chỉ cho phần đếm thời gian chờ, không cho CLI thật; ngưỡng ms rộng.

## Kiểm thử

- Như mục 2; mỗi test dùng thư mục tạm riêng để chạy song song trong vitest.

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run src/relay/codeintel/tool-slot-limits.load.test.ts`.

## Tiêu chí hoàn thành

- [x] Bất biến giữ khi 20 lời gọi hỗn hợp; lời gọi bị từ chối không spawn tiến trình.
- [x] Không thêm phụ thuộc, không `max-lines` disable, tên tệp theo khái niệm.

## Rủi ro và lưu ý

- Chỉ POSIX (shebang); có thể flaky trên CI chậm: dùng sync bằng tệp, không dựa sleep chính xác.
- Hợp đồng chưa nói hành vi khi hàng đợi đầy (solution câu hỏi 4): test chốt theo AG-CV-SOL-001 và ghi lại.

# AG-CV-TASK-071-06: Bộ chạy benchmark (lạnh/ấm, 10 phiên, 10 worktree) và báo cáo JSON

**From Solution:** [AG-CV-SOL-071-perf-block-and-bench](../solutions/AG-CV-SOL-071-perf-block-and-bench.md)
**Priority:** P1
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/codeintel/codeintel-bench-runner.ts` + `codeintel-bench-runner.smoke.test.ts` (mới), agent/scripts/bench-codeintel.mjs (mới)
**Depends on:** 071-03, 071-04, 071-05; AG-CV-SOL-070 task 03 (bộ chạy entry TS)
**Status:** [ ] TODO

## Context

CR-071 §2.2 (dòng "Agent (TS)"): 30 lần lạnh, 100 lần ấm, đo thời gian, kích thước, RSS đỉnh, số lần `truncated`; bài thử tải 10 phiên cùng worktree (singleflight) và 10 worktree khác nhau (hàng đợi giữ ≤ 2 gitnexus). Chạy trên máy chuyên dụng có chỉ mục Orca dựng sẵn (người vận hành tự chạy `analyze`).
Báo cáo agent riêng `codeintel-agent-bench-<commit>.json` (solution Correction 4), lưu artifact, không vào git.
Mọi con số hiệu năng là giả định từ CR-CV-071 (một lần đo, một máy), chưa phải phân phối.

## Việc cần làm

1. `runBench({methods, coldRuns, warmRuns, workspaceRoots, outDir})`: gọi dispatcher thật không qua mạng; xoá cache agent giữa lần lạnh; ghi p50/p95/p99 `totalMs`, byte kết quả, `truncatedRate`, `rssPeakKbMax`, `concurrency.{gitnexusMax,totalMax,queueWaitTimeouts}` (đếm bằng sampler số tiến trình sống).
2. Kịch bản "10 phiên" và "10 worktree".
3. `smoke.test.ts`: chạy với CLI giả (071-04), `coldRuns=2, warmRuns=3`, ngưỡng rộng, kiểm hình dạng báo cáo.
4. Vỏ `.mjs`: bundle entry rồi gọi; tham số `--commit`, `--out`; in tóm tắt.

## Kiểm thử

- `smoke`: báo cáo đúng schema, p95 là số, concurrency quan sát ≤ giới hạn
- Thủ công trên máy bench (CHƯA CHẠY): chạy đủ kịch bản trên Orca; điền số đo đầu tiên vào PR để hiệu chỉnh ngân sách một lần.

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run <đường dẫn test>`. Chưa chạy.

## Tiêu chí hoàn thành

- [ ] Smoke xanh ở PR; bench chạy được bằng `node scripts/bench-codeintel.mjs`.
- [ ] Không thêm phụ thuộc, không `max-lines` disable, tên tệp theo khái niệm.

## Rủi ro và lưu ý

- Số đo lần đầu có thể vượt ngân sách giả định: ngân sách được hiệu chỉnh một lần.
- Cây tiến trình: chỉ đo tiến trình trực tiếp (solution 2.3).
- Không `analyze` trong bench trừ khi cờ riêng.

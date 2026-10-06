# AG-CV-TASK-071-07: Workflow benchmark đêm `code-intel-bench.yml`

**From Solution:** [AG-CV-SOL-071-perf-block-and-bench](../solutions/AG-CV-SOL-071-perf-block-and-bench.md)
**Priority:** P1
**Area:** `agent/` (Dev Server Agent)
**File:** `.github/workflows/code-intel-bench.yml` (mới)
**Depends on:** 071-05, 071-06
**Status:** [ ] TODO

## Context

CR-071 §2.2 "Khi nào chạy": hằng đêm và `workflow_dispatch` trên runner chuyên dụng, không chặn PR. Hiện **không có** runner chuyên dụng nào được ghi nhận (chưa kiểm); nhãn do người vận hành chốt (câu hỏi mở 1 của solution).

## Việc cần làm

1. Workflow `schedule` + `workflow_dispatch`, `runs-on` nhãn tham số (`inputs.runner`, mặc định chưa đặt → job bị bỏ qua nếu không có), `continue-on-error: true`.
2. Các bước: checkout, setup node/pnpm như `pr.yml`, build bundle bench, chạy `bench-codeintel.mjs`, chạy `check-codeintel-bench-budgets.mjs`, upload artifact báo cáo.
3. Ghi chú đầu tệp: chỉ chạy trên máy đã có `.gitnexus/`/`.codegraph/` của commit Orca cố định.

## Kiểm thử

- `workflow_dispatch` thủ công trên runner thử (CHƯA CHẠY); kiểm cú pháp bằng đọc lại (không có `actionlint` đã xác nhận).

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run <đường dẫn test>`. Chưa chạy.

## Tiêu chí hoàn thành

- [ ] Workflow tồn tại, không chặn PR, tải được artifact.
- [ ] Không thêm phụ thuộc, không `max-lines` disable, tên tệp theo khái niệm.

## Rủi ro và lưu ý

- Phụ thuộc runner chưa có; nếu không có thì task chỉ để sẵn workflow và hướng dẫn chạy tay.

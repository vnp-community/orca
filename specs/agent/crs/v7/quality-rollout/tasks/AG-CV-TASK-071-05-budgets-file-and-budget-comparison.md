# AG-CV-TASK-071-05: `codeintel-budgets.json` và bộ so sánh báo cáo với ngân sách

**From Solution:** [AG-CV-SOL-071-perf-block-and-bench](../solutions/AG-CV-SOL-071-perf-block-and-bench.md)
**Priority:** P1
**Area:** `agent/` (Dev Server Agent)
**File:** agent/scripts/codeintel-budgets.json (mới), `agent/src/relay/codeintel/bench-budget-comparison.ts` + `.test.ts` (mới), agent/scripts/check-codeintel-bench-budgets.mjs (mới)
**Depends on:** 071-01
**Status:** [x] DONE

## Context

CR-071 D1: ngân sách là dữ liệu; vượt thì thoát ≠ 0 (mẫu `check-terminal-perf-report-budgets.mjs` ở `package.json` gốc, chưa mở tệp). Script `check-*` ở nơi khác nằm `desktop/config/scripts/`; script này nằm `agent/scripts/` (đóng gói cùng gói).
Phần `agent` của ngân sách (solution 2.5); phần `views` của BE-CV-SOL-071 (BE-CV-TASK-071-01). Cần thống nhất tên khoá chung.
Mọi con số hiệu năng là giả định từ CR-CV-071 (một lần đo, một máy), chưa phải phân phối.

## Việc cần làm

1. Tạo `codeintel-budgets.json` (có `provenance` ghi rõ giả định).
2. `compareBenchToBudgets(report, budgets)`: trả `Violation[]`; **thiếu trường trong báo cáo = vi phạm "missing"**, không phải đạt.
3. Vỏ `.mjs` đọc hai tệp JSON, gọi hàm (bundle bằng `esbuild-run-typescript-entry.mjs` của 070-03), in bảng, thoát 0/1.

## Kiểm thử

- báo cáo trong ngân sách → không vi phạm
- báo cáo vượt `payloadBytes`/`rssPeakKbMax`/`concurrency` → vi phạm đúng metric
- báo cáo thiếu `rssPeakKbMax` → vi phạm "missing"
- ngân sách thiếu khoá → lỗi cấu hình
- Chạy vỏ `.mjs` với báo cáo cố ý vượt → exit 1 (thủ công, chưa chạy)

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run src/relay/codeintel/bench-budget-comparison.test.ts`.

## Tiêu chí hoàn thành

- [x] Test xanh; script thoát đúng mã.
- [x] Không thêm phụ thuộc, không `max-lines` disable, tên tệp theo khái niệm.

## Rủi ro và lưu ý

- Nút chỉnh duy nhất là sửa JSON qua PR có lý do.

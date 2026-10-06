# BE-CV-TASK-071-01: `codeintel-budgets.json` và checker so báo cáo benchmark

**From Solution:** BE-CV-SOL-071
**Priority:** P1
**Service:** `backend-go/ci`
**File:** `backend-go/ci/code-intel-bench/codeintel-budgets.json` (mới), `backend-go/ci/code-intel-bench/check-codeintel-bench-budgets.mjs` (mới), `backend-go/ci/code-intel-bench/check-codeintel-bench-budgets.test.mjs` (mới), `backend-go/ci/code-intel-bench/testdata/report-within-budget.json`, `report-over-budget.json` (mới)
**Depends on:** BE-CV-SOL-010 (cấu trúc service); không cần code sản phẩm
**Status:** `[ ] TODO`

---

## Context

- Mẫu so báo cáo với ngân sách: `desktop/config/scripts/check-terminal-perf-report-budgets.mjs` (+ `.test.mjs`). Lưu ý `package.json` gốc trỏ `config/scripts/...` nhưng thư mục đó chỉ còn 3 tệp (README v7 §8 điểm 20): **không** dựa vào đường dẫn gốc.
- Số của CR-071 §2.1 là giả định chưa đo; hợp đồng sửa một số (SOL-071 L4, L5): snapshot 3 MiB, chờ slot 10 s, stdout 16 MiB, JSON cuối 8 MiB, tách `GetClusterOverview` khỏi `GetArchitecture` (C4).
- Hàng cho RPC chưa có số: `null` + `"unmeasured": true`, checker chỉ cảnh báo.

## Việc cần làm

1. Viết `codeintel-budgets.json` (cấu trúc ở SOL-071 mục 5.1) với các hàng RPC của CR-071 §2.1 đã sửa và mọi RPC còn lại (đối chiếu §3.1/§3.2 hợp đồng) ở trạng thái `null`.
2. Checker Node (cùng ngôn ngữ mẫu): đọc báo cáo `codeintel-bench-<commit>.json` (`{commit,tools,host,views:[{rpc,cold:{p50,p95,p99,n},warm:{...},bytes:{p50,p95,max},truncatedRate,agentRssPeakMiB}]}`), so `p95` với ngân sách, `bytes.max` với trần, `agentRssPeakMiB` với `maxProcessTreeRssMiB`; thoát 1 khi vượt, in bảng; `rpc` không có trong ngân sách ⇒ lỗi (bắt RPC mới thêm mà quên).
3. Test `.test.mjs`: báo cáo đạt ⇒ 0; báo cáo cố ý vượt ⇒ 1 và thông điệp nêu RPC; ô `null` ⇒ 0 + cảnh báo; thiếu RPC ⇒ 1.
4. Không thêm phụ thuộc npm mới; chạy bằng `node` thuần.

## Kiểm thử

- `node --test backend-go/ci/code-intel-bench/check-codeintel-bench-budgets.test.mjs`.
- Chưa chạy.

## Tiêu chí hoàn thành

- [ ] Ngân sách là dữ liệu; đổi số chỉ qua sửa JSON.
- [ ] Báo cáo cố ý vượt làm checker thoát khác 0.
- [ ] RPC mới không có trong JSON làm checker đỏ.

## Rủi ro và lưu ý

- Số ngân sách là giả định; ghi `assumption` trong tệp để người đọc không coi là SLO.
- Hiệu chỉnh một lần sau benchmark đầu (CR-071 §2.2), sau đó chỉ đổi qua PR có lý do.

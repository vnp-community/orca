# BE-CV-TASK-071-07: Benchmark Go (`bench/`) và workflow benchmark đêm

**From Solution:** BE-CV-SOL-071
**Priority:** P2
**Service:** `code-intel-service`, `.github/workflows`
**File:** `backend-go/services/code-intel-service/bench/normalize_bench_test.go`, `bench/erd_parse_bench_test.go`, `bench/collector_replay_bench_test.go`, `bench/report_writer.go` (mới, build tag `bench`), `.github/workflows/code-intel-bench.yml` (mới)
**Depends on:** BE-CV-TASK-071-01; BE-CV-SOL-020, 021, 031-sql-migration-parser (ERD), `AG-CV-SOL-071-perf-block-and-bench`
**Status:** `[ ] TODO`

---

## Context

- Đo chi phí phía Go không cần agent thật: chuẩn hoá `SymbolRef`, hợp nhất hai nguồn (CR-020), parse SQL → ERD (CR-031) trên **588 tệp `.sql`** dưới `backend-go/` (đã đếm), JSON và `proto.Size`.
- Benchmark đầu-cuối trên Orca (30 lạnh/100 ấm, RSS cây tiến trình) chạy bằng script agent (`bench-codeintel.mjs`, thuộc `AG-CV-SOL-071`) trên runner chuyên dụng; **runner đó chưa có** (CR-071 Q1).
- Báo cáo là artifact CI, **không** vào git; schema ở CR-071 §2.2.

## Việc cần làm

1. `bench/` với `//go:build bench`: `testing.B` cho (a) chuẩn hoá `SymbolRef` trên kết quả vàng nhân bản tới 5 000 nút, (b) hợp nhất hai nguồn, (c) `proto.Size` + marshal JSON của phản hồi 2 MiB, (d) parse toàn bộ tệp `.sql` của `backend-go/` (đường dẫn truyền bằng biến môi trường, không đọc ngoài repo), (e) replay collector với agent giả có độ trễ ghi nhận.
2. `report_writer.go`: ghi `codeintel-bench-<commit>.json` đúng schema (trường nào không đo được để vắng, không đoán).
3. Workflow `code-intel-bench.yml`: `workflow_dispatch` + `schedule` (đêm) trên nhãn runner `code-intel-bench` (chưa tồn tại: job `if: vars.CODEINTEL_BENCH_RUNNER != ''`); chạy `go test -tags=bench -run '^$' -bench . -benchtime=...`, rồi `check-codeintel-bench-budgets.mjs`; tải báo cáo lên artifact. Trên PR: chỉ micro-benchmark với ngưỡng rộng (không chặn trừ hồi quy khổng lồ, ngưỡng do task 01 cấu hình).
4. Bài thử tải 10 phiên (singleflight gộp; 10 worktree khác nhau): kịch bản Go dùng agent giả, khẳng định số lần gọi agent bằng số worktree khác nhau.

## Kiểm thử

- `go test -tags=bench -bench . ./bench/...` chạy được trên máy phát triển; checker nhận báo cáo ra.
- Chưa chạy: số liệu là giả định; **không** đưa kết quả vào ngân sách trước khi có báo cáo trên runner đúng cấu hình.

## Tiêu chí hoàn thành

- [ ] Benchmark biên dịch với tag `bench` và không chạy trong `go test ./...` thường.
- [ ] Báo cáo đúng schema, đọc được bởi checker.
- [ ] Workflow không chặn PR.

## Rủi ro và lưu ý

- Không có runner chuyên dụng thì chỉ chạy cục bộ; ghi rõ trong README service.
- Phụ thuộc phần cứng nên không dùng số tuyệt đối làm điều kiện PR.

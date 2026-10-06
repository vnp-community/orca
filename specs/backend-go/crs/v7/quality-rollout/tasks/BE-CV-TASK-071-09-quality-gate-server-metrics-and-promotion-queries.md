# BE-CV-TASK-071-09: Số liệu server của CR-CV-095: metric cổng/miễn trừ và truy vấn quản trị

**From Solution:** BE-CV-SOL-071
**Priority:** P2
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/metrics/quality_gate_metrics.go` (mới), `.../internal/adapter/metrics/quality_gate_metrics_test.go` (mới), `backend-go/services/code-intel-service/ops/quality-admin-queries/{postgres,mysql}/*.sql` (mới), `.../internal/adapter/postgres/quality_admin_queries_integration_test.go`, `.../internal/adapter/mysql/quality_admin_queries_integration_test.go` (mới, tag `integration`)
**Depends on:** BE-CV-TASK-071-02, BE-CV-SOL-085-quality-gate-evaluator-and-profiles, BE-CV-SOL-085-waivers-and-trend (bảng T10–T12), BE-CV-SOL-037 (`finding_dismissals`), BE-CV-SOL-089 (`agent_turns`)
**Status:** `[ ] TODO`

---

## Context

- CR-095 §2.6: bộ đếm có nhãn enum, **không** nhãn tenant/repo/rule; phân tích theo tenant bằng truy vấn quản trị trên bảng của chính tenant. §2.8: ngưỡng nâng cổng từ `inform` lên `block` (cỡ mẫu ≥ 200 đánh giá, ≥ 50 `fail`; báo nhầm ≤ 5 %; miễn trừ ≤ 10 %; `unknown` ≤ 10 %…) là **số đề xuất chưa hiệu chỉnh**.
- Lệch (SOL-071 L8, L9): tên metric có tiền tố `orca_`; không có enum `false_positive` ở `finding_dismissals.reason` (varchar 500 tự do) nên tỉ lệ báo nhầm chỉ là **xấp xỉ** (`disposition='ignored'`).
- Bảng: T11 `quality_waivers`, T12 `quality_trend_points`, T5 `finding_dismissals`, T14 `agent_turns` (hợp đồng §4.2); mọi truy vấn có `tenant_id`.

## Việc cần làm

1. Metric: `orca_codeintel_quality_gate_evaluations_total{verdict}` (`pass|warn|fail|unknown`), `orca_codeintel_quality_waivers_total{kind,action}`, `orca_codeintel_quality_gate_unknown_total{reason}` (tập `reason` đóng do BE-CV-SOL-085 khai báo; giá trị lạ → `other`). Cổng `QualityGateObserver` ở `usecase`; evaluator gọi khi `verdict` được tính.
2. Truy vấn quản trị (SQL chỉ đọc, hai dialect, tham số `:tenant_id` bắt buộc, `:from`, `:to`): (a) cỡ mẫu và tỉ lệ `unknown` theo `profile_ref` (T12); (b) tỉ lệ miễn trừ trên `fail` (T11 ∩ T12); (c) quy tắc bị bỏ qua nhiều nhất (`finding_key` ở T5, `disposition='ignored'`); (d) số lượt `agent_turns` theo `repo_binding_id`. Mỗi tệp đầu ghi chú: "xấp xỉ, xem SOL-071 L9".
3. Tài liệu ngắn trong `docs/guides/code-intel/code-intel-performance-and-metrics.md` (task 08) chỉ cách chạy truy vấn (qua `psql`/`mysql` bởi người vận hành có quyền).
4. Không thêm RPC, không thêm bảng, không đưa ngưỡng vào code sản phẩm.

## Kiểm thử

- Test metric: tập nhãn đóng; không nhãn tenant/repo/rule.
- Tích hợp hai dialect (ma trận `dialect`): dữ liệu hai tenant; truy vấn của tenant A không trả dòng của B; chạy không có `tenant_id` thì lỗi; kết quả đúng trên bộ dữ liệu cố định nhỏ.
- `go test -tags=integration ./internal/adapter/postgres/... ./internal/adapter/mysql/...`.

## Tiêu chí hoàn thành

- [ ] Ba metric có mặt, nhãn đóng.
- [ ] Bốn truy vấn chạy trên Postgres và MySQL, cô lập tenant.
- [ ] Ghi rõ giới hạn "xấp xỉ" và "ngưỡng đề xuất".

## Rủi ro và lưu ý

- Nếu hợp đồng thêm enum báo nhầm, cập nhật truy vấn (c).
- Truy vấn lớn trên bảng lớn có thể chậm; chưa đo (T12 giữ ≤ 200 điểm/binding, 90 ngày).

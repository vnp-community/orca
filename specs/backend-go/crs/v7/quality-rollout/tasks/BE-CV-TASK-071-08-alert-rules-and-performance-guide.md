# BE-CV-TASK-071-08: Quy tắc cảnh báo `codeintel.rules.yaml` và trang hướng dẫn hiệu năng

**From Solution:** BE-CV-SOL-071
**Priority:** P2
**Service:** `backend-go/deploy/alerts`, `docs`
**File:** `backend-go/deploy/alerts/codeintel.rules.yaml` (mới), `backend-go/deploy/alerts/codeintel_rules_test.go` (mới, hoặc test trong module phù hợp), `docs/guides/code-intel/code-intel-performance-and-metrics.md` (mới), `backend-go/services/code-intel-service/README.md`
**Depends on:** BE-CV-TASK-071-02, 071-04, 071-05
**Status:** `[ ] TODO`

---

## Context

- Mẫu: `backend-go/deploy/alerts/mcp.rules.yaml` (dòng đầu: "Not loaded by anything in this repository … Only metrics that exist today are used"). Cảnh báo của code-intel cùng tình trạng chưa có nơi nạp.
- Mười rule CR-071 §2.7 với biểu thức sửa theo SOL-071 (L3: tập `result` loại khỏi tỉ lệ lỗi; L6: không nhãn `command` cho drift; L10: gauge `streams_limit`).
- Ngưỡng chưa có dữ liệu để hiệu chỉnh.

## Việc cần làm

1. Viết `codeintel.rules.yaml` nhóm `orca-codeintel`: `CodeIntelLatencyOverBudget`, `CodeIntelErrorRate`, `CodeIntelToolFormatDrift`, `CodeIntelToolIncompatible` (dùng `tool_compat{state="unsupported"}`), `CodeIntelReindexSlow`, `CodeIntelReindexStuck`, `CodeIntelCacheNearCap`, `CodeIntelOutboxLag`, `CodeIntelResponseTruncatedHigh`, `CodeIntelGatewayStreamsSaturated` (`streams_active / streams_limit > 0.8`). Giữ ghi chú "chưa được nạp".
2. Test: parse YAML, trích mọi tên metric trong `expr`, khẳng định tồn tại trong tập tên xuất từ ba registry (đọc từ test dùng `testutil.CollectAndLint` hoặc danh sách tên do mỗi gói metrics xuất); `promtool check rules` nếu có trong CI (chưa kiểm chứng có sẵn).
3. Tài liệu `code-intel-performance-and-metrics.md` (tiếng Việt): bảng ngân sách (ghi "giả định chưa đo"), danh sách metric, cách đọc một trace, cách chạy benchmark, cách đọc alert; cập nhật `README.md` service (hàng metrics, `/metrics` trên cổng health).

## Kiểm thử

- Test rule; `go test ./deploy/alerts/...` hoặc module chứa test (chốt khi viết).
- Chưa chạy.

## Tiêu chí hoàn thành

- [ ] Mọi metric trong rule tồn tại (test đỏ khi đổi tên metric).
- [ ] Tài liệu nêu rõ ngưỡng là giả định.
- [ ] Ghi chú "chưa được nạp" có mặt.

## Rủi ro và lưu ý

- Rule đúng nhưng không ai nhận cảnh báo cho tới khi có hạ tầng nạp (chưa xác minh nơi nạp).
- Tài liệu ở `docs/guides/` (đường dẫn thật là `guides/` ở docs, README v7 §8 điểm 19; xác minh thư mục khi viết).

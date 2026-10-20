# Outcome Review Core — Change Requests (v9)

> Mô hình và dữ liệu của review mức kết quả: dữ kiện xác định do Orca tính (tầng F) và ngữ cảnh thiết kế do agent đọc từ tdd knowledge graph (tầng C). Bối cảnh, quyết định D1–D11, hợp đồng chung và thứ tự đợt ở [README v9](../README.md). **Khi README và CR khác nhau thì theo CR.**

| CR | Vấn đề | Priority | Effort | Trạng thái |
|----|--------|----------|--------|------------|
| [CR-OR-001](./CR-OR-001-outcome-review-model-and-facts.md) | Dữ liệu kết quả của một Request nằm rải ở độ phủ AC, đánh giá tác động, tóm tắt thực thi, handover; chưa có bản ghi gắn với commit để xem, so sánh, quyết định | 🔴 P0 | Large | 📝 Đề xuất, chưa triển khai |
| [CR-OR-002](./CR-OR-002-context-run-kg-and-system-diff.md) | Orca không đọc KG và không có lượt agent nào dựng "diff mức hệ thống" (kiến trúc, luồng, hành vi, dữ liệu, hợp đồng); cần kết quả có cấu trúc, có bằng chứng, giới hạn chi phí | 🔴 P0 | Large | 📝 Đề xuất, chưa triển khai |

## Thứ tự thực thi

```
CR-OR-001 (bảng, mô hình thuần, dữ kiện F, RPC Estimate/Generate/Get/List)
   └▶ CR-OR-002 (context run, bằng chứng, mô hình sơ đồ D)
```

001 trước: nó định nghĩa `OutcomeReview`, `ReviewItem`, `inputsDigest` mà 002 điền vào. 001 chạy độc lập (chỉ tầng F) nên giao được một màn hình hữu ích khi chưa có KG.

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|-----------|-------|
| C1 | Mô hình dựng bằng **hàm thuần** `(facts, context) → OutcomeReview` | Kiểm thử bằng fixture, tái lập, cùng đầu vào cho cùng `inputsDigest` |
| C2 | `origin=fact|agent` có mặt ở **mọi** mục; chỉ `fact` mới đi vào kết luận | README v9 D2 |
| C3 | Bằng chứng bắt buộc cho mục `origin=agent`; thiếu hoặc không xác minh được hiển thị "Chưa xác minh" | Chống bịa; Orca kiểm được tệp có trong diff và định dạng id |
| C4 | Context run **chỉ-đọc**, không gọi `tdd-knowledge-writeback`, không ghi KG | Quyết định T1.7 hiện có: Orca không áp thay đổi vào KG |
| C5 | Kết quả context run được **cache** theo đầu vào xác định; chỉ chạy lại khi người dùng sinh lại hoặc hết hạn | Tiết kiệm token |

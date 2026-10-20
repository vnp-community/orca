# Outcome Review Rollout — Change Requests (v9)

> Bảo mật artifact do LLM sinh, bộ kiểm thử với fixture vàng, đo chi phí và tỉ lệ trúng cache, cờ và rollout theo project. Bối cảnh, quyết định và hợp đồng chung ở [README v9](../README.md). **Khi README và CR khác nhau thì theo CR.**

| CR | Vấn đề | Priority | Effort | Trạng thái |
|----|--------|----------|--------|------------|
| [CR-OR-030](./CR-OR-030-security-testing-metrics-and-rollout.md) | HTML/SVG do agent sinh là bề mặt tấn công; chi phí AI và hiệu quả cache chưa đo; chưa có kế hoạch bật theo project, kiểm thử e2e và tài liệu vận hành | 🔴 P0 | Medium | 📝 Đề xuất, chưa triển khai |

## Thứ tự thực thi

CR-OR-030 chạy **song song** từ đợt 2: bộ kiểm thử hợp đồng với fixture ngay khi CR-OR-002 chốt định dạng; phần bảo mật artifact ngay khi CR-OR-010/021 có mã; rollout cuối cùng.

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|-----------|-------|
| R1 | Artifact và kết quả agent là **dữ liệu không tin cậy** ở mọi lớp (kiểm khi lưu, CSP khi hiển thị, lược đồ khi nhận) | LLM đọc nội dung do người khác viết (AC, diff, KG) |
| R2 | **Bộ kiểm chứng "canary"** cho bí mật (mẫu `secret_canary.go` của code-intel-service) | Mẫu đã có; chứng minh bộ che hoạt động |
| R3 | Chỉ số thành công chính là **tỉ lệ trúng cache** và **token mỗi review**; ngưỡng chốt sau khi đo trên Request pilot | Mục tiêu "đỡ tốn token" phải đo được |
| R4 | Bật theo project, mặc định tắt, có công tắc tắt khẩn; tắt không xoá dữ liệu | Rollback an toàn |

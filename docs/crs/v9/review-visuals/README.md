# Review Visuals — Change Requests (v9)

> Hình vẽ của Outcome Review: vẽ bằng skill [`diagram-design`](https://github.com/cathrynlavery/diagram-design), **lưu lại theo nội dung để không vẽ lại**, **sinh lại khi người dùng yêu cầu**, và cho phép tuỳ chỉnh qua hồ sơ vẽ (MVP chỉ có preset mặc định tốt nhất). Bối cảnh, quyết định D4–D9 và preset `orca-default` ở [README v9](../README.md) (mục 3.8). **Khi README và CR khác nhau thì theo CR.**

| CR | Vấn đề | Priority | Effort | Trạng thái |
|----|--------|----------|--------|------------|
| [CR-OR-010](./CR-OR-010-diagram-design-drawing-pipeline.md) | Chưa có cách vẽ sơ đồ chất lượng, nhất quán, không tương tác, tái lập từ mô hình sơ đồ | 🔴 P0 | Large | 📝 Đề xuất, chưa triển khai |
| [CR-OR-011](./CR-OR-011-review-artifact-store-and-cache.md) | Hình do agent sinh tốn token; không lưu thì mỗi lần mở là một lần trả tiền | 🔴 P0 | Medium | 📝 Đề xuất, chưa triển khai |
| [CR-OR-012](./CR-OR-012-regenerate-review-artifacts.md) | Hình/ngữ cảnh có thể sai hoặc cũ; cần sinh lại có kiểm soát, giữ bản cũ, so sánh | 🔴 P0 | Medium | 📝 Đề xuất, chưa triển khai |
| [CR-OR-013](./CR-OR-013-drawing-profile-customization.md) | Mỗi tenant có thể muốn loại sơ đồ, mức chi tiết, màu khác; cần tuỳ chỉnh an toàn | 🟠 P1 | Medium | 📝 Đề xuất, chưa triển khai |

## Thứ tự thực thi

```
CR-OR-011 (kho) ─▶ CR-OR-010 (vẽ, ghi vào kho) ─▶ CR-OR-012 (sinh lại) ─▶ CR-OR-013 (tuỳ chỉnh)
```

011 trước vì 010 cần chỗ ghi và khoá cache. 012 chỉ có nghĩa khi đã có hình để so sánh. 013 làm sau khi preset mặc định chạy ổn và đã có số liệu chi phí.

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|-----------|-------|
| V1 | Khoá cache là **digest của mô hình sơ đồ** + digest preset + phiên bản skill, không phải digest cả review | Đổi một phần review chỉ vẽ lại phần đó; đổi commit nhưng sơ đồ y hệt thì tái dùng |
| V2 | Màu theo **vai trò**, giá trị token điền lúc hiển thị | Một artifact dùng cho sáng/tối; đổi theme không tốn token |
| V3 | Bản skill **ghim** trong `skills/diagram-design/` với LICENSE | Tái lập; mã chạy trên dev server phải qua duyệt |
| V4 | Chỉ cho phép script đã duyệt (`self_check.py`); không chạy xuất PNG/Playwright | Giảm bề mặt; không cần ảnh ở MVP |
| V5 | Tối đa **một** lần sửa khi `self_check` lỗi; vẫn lỗi thì quay về bảng | Chặn vòng lặp tốn token |
| V6 | Mọi artifact là HTML/SVG **không tin cậy** | CR-OR-030 |

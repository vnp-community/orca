# Outcome Review Frontend — Change Requests (v9)

> Tab **Outcome review** (ngay sau Execution) trong màn hình chi tiết Request: kết luận mức kết quả, "cần bạn quyết định", yêu cầu giao so với hỏi, tab diff mức hệ thống với hình do skill vẽ, và vòng phản hồi. Bối cảnh, quyết định D1–D12 và hợp đồng chung ở [README v9](../README.md). **Khi README và CR khác nhau thì theo CR.** Mockup tương tác đã dùng làm tham chiếu thị giác trong phiên thiết kế 2026-10-20 (kết luận, diff theo tab, tab Flow); không phải đặc tả pixel.

| CR | Vấn đề | Priority | Effort | Trạng thái |
|----|--------|----------|--------|------------|
| [CR-OR-020](./CR-OR-020-outcome-review-tab-shell.md) | Màn Request chưa có nơi cho review mức kết quả; cần khung, trạng thái và thẻ kết luận không overclaim | 🔴 P0 | Large | 📝 Đề xuất, chưa triển khai |
| [CR-OR-021](./CR-OR-021-system-diff-tabs-and-artifact-viewer.md) | Cần các tab diff (Architecture, Flow, Behavior, Data, Contracts, Requirements, Config) và cách hiển thị an toàn hình do agent vẽ | 🔴 P0 | Large | 📝 Đề xuất, chưa triển khai |
| [CR-OR-022](./CR-OR-022-decisions-feedback-and-code-review-link.md) | Chưa có chỗ ghi quyết định (chấp nhận ngoại lệ, trả lại, duyệt), "gửi lại agent" và nối sang review code v7 | 🔴 P0 | Medium | 📝 Đề xuất, chưa triển khai |

## Thứ tự thực thi

```
CR-OR-020 (khung, kết luận, bảng yêu cầu; chưa hình) ─▶ CR-OR-022 (quyết định)
CR-OR-020 ─▶ CR-OR-021 (tab diff + trình xem; cần CR-OR-010/011 cho hình)
```

020 giao được độc lập khi mới có tầng F (CR-OR-001). 021 chỉ hiện hình khi có artifact; thiếu thì dùng bảng.

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|-----------|-------|
| U1 | Tab nằm trong `RequestDetailPane`, thư mục mới `components/request/outcome-review/`; không chạm `components/review-map/` ngoài việc gọi lối vào | v7 đã sở hữu `review-map/`; tránh hai nơi sửa cùng một khối |
| U2 | **Một nút chính** mỗi màn (Approve); "Request changes" là phụ | Quy tắc "restraint" của STYLEGUIDE/CDS |
| U3 | Chữ không overclaim: mục `origin=agent` luôn mang nhãn "Agent reported"; `unknown` hiển thị là "Unknown"; "Approved" không đi kèm "safe to release" | README v9 mục 6 |
| U4 | Artifact hiển thị trong **iframe sandbox không script**; màu điền từ token tại thời điểm hiển thị | D6, D9 |
| U5 | Nhãn giao diện tiếng Anh, khoá i18n đủ 5 locale (en, es, ja, ko, zh) và test bao phủ locale riêng | Quy ước dự án (handoff mục 2) |
| U6 | Hai render target (Electron `window.api`, web `web-preload-api`): chỉ gọi qua lớp gateway giống `useExecutionSummary` | Quy ước chung |
| U8 | Architecture và Flow là hai tab con đầu tiên và có thẻ riêng ở đầu tab; mức C4 level 2 (D13, D14); chi tiết component dẫn sang lens v7 | Người duyệt tập trung kiểm soát kiến trúc và luồng |
| U7 | Architecture, Flow, Behavior, Data có hai hình (`design` từ KG và `delta` sau code); `design` tái dùng giữa các lần sinh và giữa hai pha | Đối chiếu kiến trúc/luồng trước và sau code |
| U9 | Tab tên "Outcome review", đứng ngay sau Execution; hiện từ khi Request có Plan, kể cả khi Execution chưa xong (chỉ phần Design); phần After code và quyết định có sau khi Execution xong | Yêu cầu của người dùng; tránh nhầm với tab Review của v7 |

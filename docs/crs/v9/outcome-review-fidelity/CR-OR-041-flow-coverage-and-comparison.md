# CR-OR-041 — Phủ flow bị ảnh hưởng, ghép Design | After code và so sánh đúng cơ sở

**Series:** `outcome-review-fidelity` · **Trạng thái:** 📝 Đề xuất (2026-10-21) · **Giải pháp:** BE-OR-SOL-005 §4, FE-OR-SOL-004 · **Checklist:** `phase-7-coverage-and-resilience.md` (T701–T708), FE `phase-4-coverage-ui-and-e2e.md`

## 1. Vấn đề

Sau code, review phải trả lời: *Request này chạm flow nào, mỗi flow đã đổi thế nào so với thiết kế?* Hiện:

1. **Không có danh sách flow bị ảnh hưởng.** Agent tự chọn vẽ gì; Orca không biết còn flow nào chưa được vẽ. #2318 chạm 2 flow (Chat FR-NTF-03, nhập Sheet FR-NTF-05) và chỉ 1 được vẽ (README E6).
2. **Design và After code không ghép theo flow** (E7): hai cột theo `variant`, tên flow khác nhau.
3. **Cạnh so với thiết kế theo cặp `from→to` trên mọi flow** (E8) và không nói cơ sở yếu. Thiết kế co lại làm cạnh đúng thành `deviation`.
4. **Trần im lặng** (E9): 16 hình, 4 hình mỗi lô, 12 nút, 6 lane; vượt 16 hình thì từ chối cả kết quả.
5. **Huy hiệu số liệu có thể lệch hàng** (E12).

## 2. Phạm vi

**Có:** tập flow ứng viên; hợp đồng "mỗi flow ứng viên được trả lời"; `flowRef` trên hình; `model.flowCoverage`; so sánh theo flow đã ghép; hạ `advisory`; trần có thông báo; thanh phủ và thẻ ghép trên UI; vẽ từng flow theo yêu cầu.

**Không:** vẽ flow ở mức class/hàm (giữ mức container, D13); tự động chạy review (D11).

## 3. Yêu cầu

| # | Yêu cầu |
|---|---|
| R1 | Orca tính **flow ứng viên** = hợp của (a) flow thiết kế có tin nhắn chạm một container có code đổi, (b) flow nối với FR/AC của Request (khi KG nối được). Mỗi ứng viên có `reason` (`code_touches`, `requirement_link`). |
| R2 | Agent phải trả cho **mỗi flow ứng viên** một câu trả lời: hình delta, hoặc trạng thái `unchanged`, `not_applicable`, `undrawable` kèm `reason` và bằng chứng. Thiếu ⇒ vi phạm, vào vòng thử lại hiện có. |
| R3 | Flow mà code thêm nhưng thiết kế **chưa có** (như luồng nhập Sheet nếu SAD không mô tả) agent khai `flowRef` mới với `source=code`; hệ thống gắn nhãn **"mới, chưa có trong thiết kế"**, không gán `deviation` cho từng cạnh. |
| R4 | `DiagramModel` mang `flowRef`; cặp Design | After code ghép theo `flowRef`. |
| R5 | Cạnh của hình delta so với **flow thiết kế cùng `flowRef`**; không có flow thiết kế tương ứng ⇒ không tính `inDesign` (không `deviation`/`missing`). |
| R6 | Khi cơ sở thiết kế **yếu** (có `design_shrank`, hoặc flow tương ứng thiếu, hoặc phần tử KG chưa duyệt) thì nhãn `deviation`/`missing` hạ xuống `advisory` kèm lý do. |
| R7 | `model.flowCoverage = {total, drawn, items[{flowRef, name, reason, status, hasDesign, hasDelta}]}`; verdict Flow tính theo flow đã ghép. |
| R8 | Trần số hình: thay "từ chối khi vượt 16" bằng **cắt theo ưu tiên phủ** (flow ứng viên trước) và `diagrams_capped:N`; trần flow delta cấu hình `REQUEST_REVIEW_MAX_FLOW_DIAGRAMS` (mặc định 8). Phần dư xuất hiện trong `flowCoverage` với trạng thái `capped`. |
| R9 | UI: tab Flow có thanh **"Đã vẽ X/Y flow"**, danh sách flow chưa vẽ kèm lý do và nút **"Vẽ flow này"** (hiện ước tính chi phí; dùng sinh lại hình theo `section` + `flowRef`, không chạy lại context). |
| R10 | UI: mỗi flow là **một thẻ** Design | After code; không còn hai cột dài. Số trên huy hiệu tab **bằng số hàng** mang nhãn tương ứng (test). |
| R11 | Chi phí vẽ nhìn thấy được: mỗi run vẽ hiện số hình và chi phí; ước tính trước khi "Vẽ flow này". |

## 4. Tiêu chí chấp nhận

| # | Kiểm | Cách đo |
|---|---|---|
| AC1 | SAD mẫu + Request chạm 2 flow ⇒ `flowCoverage.total = 2`; agent giả chỉ vẽ 1 ⇒ vi phạm và retry, hoặc `drawn = 1` kèm hàng chưa vẽ | test domain/usecase |
| AC2 | Thiết kế 5 container (bản 07:14 của #2318) + cạnh "ms-integration → Google Chat" ⇒ nhãn `advisory`, không `deviation` | fixture từ dữ liệu #2318 |
| AC3 | 17 hình hợp lệ ⇒ kết quả **được nhận**, 16 hình giữ lại theo ưu tiên, `diagrams_capped:1` | test domain |
| AC4 | Cặp thẻ Design | After code hiện theo `flowRef` ngay cả khi tên flow khác nhau | vitest |
| AC5 | Thanh "Đã vẽ 1/2 flow" + nút "Vẽ flow này" chỉ cho người có quyền sinh lại | vitest |
| AC6 | Huy hiệu `not in design` = số hàng mang nhãn đó, cho mọi section | vitest |
| AC7 | E2E thật BL-E05: tab Flow báo **đã vẽ 2/2** sau review sau code | e2e |

## 5. Rủi ro

| Rủi ro | Giảm thiểu |
|---|---|
| KG không nối FR/AC với flow (chưa kiểm chứng) | R1(a) đủ để chạy; R1(b) là bổ sung có cờ `requirement_link` khi có dữ liệu. |
| Agent trả `undrawable` hàng loạt để né vẽ | Đếm và hiện trong thanh phủ; ngưỡng cảnh báo `flows_undrawn_ratio`; kiểm lý do có bằng chứng. |
| Chi phí vẽ tăng theo số flow (mỗi hình khoảng 0,13–0,18 USD ở #2318) | Trần 8 flow; vẽ theo yêu cầu cho phần dư; ước tính hiện trước khi vẽ. |
| Đổi hợp đồng agent | Nâng `promptVersion`/schema; đọc được lock cũ (O4); trường mới tùy chọn để model đã lưu vẫn hiển thị. |

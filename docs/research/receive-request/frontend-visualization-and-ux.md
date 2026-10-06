# Hiển thị Request, Solution, Plan, Thực thi, Task và đồ thị đa chiều ở frontend

| Trường | Giá trị |
|---|---|
| **Ngày** | 2026-10-06 |
| **Loại** | Đề xuất thiết kế giao diện (chưa phải CR, chưa dựng thử) |
| **Mục tiêu** | Hiển thị Request, Solution, Plan, thực thi, Task và các chiều tác động (Kiến trúc, Tương thích hợp đồng, Dữ liệu, Phạm vi ảnh hưởng...) dạng đồ thị càng nhiều càng tốt, nhưng trải nghiệm phải đơn giản nhất |
| **Căn cứ trong repo** | `frontend/package.json` (`@xyflow/react`, `mermaid`, `@tanstack/react-virtual`, `cmdk`, `radix-ui`, `zustand`); `frontend/src/renderer/src/components/task/TaskDAGView.tsx` và `TaskGraph.tsx`; `guides/STYLEGUIDE.md` (mục UX rules và Screen UX review rubric) |
| **Liên quan** | [impact-assessment-and-risk-scoring.md](./impact-assessment-and-risk-scoring.md), [artifact-formats-ontology-and-execution-readiness.md](./artifact-formats-ontology-and-execution-readiness.md), [CR-REQ-018 đến 023](../../crs/v6/request-frontend/README.md) |

> Mọi thứ trong file này là đề xuất. Chưa dựng thử, chưa đo hiệu năng. Các điểm chưa kiểm chứng ở mục 9.

## 1. Kết luận ngắn

Một khung chi tiết theo từng bước, **mặc định chỉ hiện tóm tắt**; một canvas đồ thị duy nhất có nhiều **góc nhìn (lens)** chuyển bằng chip. Không làm mỗi chiều một màn hình riêng.

## 2. Những gì frontend đã có để dùng lại

| Có sẵn | Ghi chú |
|---|---|
| `@xyflow/react` (React Flow) | Đã dùng ở `TaskDAGView.tsx` và `TaskGraph.tsx`. Dùng làm nền cho mọi đồ thị |
| `mermaid` | Có trong `package.json`; hợp cho sơ đồ tĩnh nhúng trong văn bản, không cho đồ thị tương tác |
| `@tanstack/react-virtual`, `cmdk`, `radix-ui`, `zustand` | Danh sách dài, tìm kiếm và nhảy nhanh, popover, store |
| Quy tắc ở `guides/STYLEGUIDE.md` | Tiết lộ dần, 1 đến 2 thao tác cho việc thường gặp, focus mặc định, so sánh cạnh nhau chỉ khi so sánh là mục đích, tránh thẻ lồng thẻ, độ trễ SSH, copy không nói quá |

Hai điểm cần lưu ý:
- `TaskDAGView.tsx` đang dùng **màu hex cứng** (trái STYLEGUIDE, vì STYLEGUIDE yêu cầu dùng token) và tự xếp node theo "làn" thứ tự (`buildDAGLayout`), chưa có thư viện tự bố trí.
- Đồ thị kiến trúc với cụm và cạnh chéo cần một thư viện bố trí (ví dụ elkjs hoặc dagre). Đó là phụ thuộc mới, cần duyệt.

## 3. Bố cục tổng thể: ba màn hình, một luồng

```
Requests
 ├─ Hộp việc cần tôi   : mọi thứ đang chờ tôi (duyệt, trả lời, xác nhận loại)  ← mở mặc định
 ├─ Danh sách          : lọc theo loại, trạng thái, nguồn
 └─ Backlog            : Request backlog, Task backlog, Execute backlog (3 phân đoạn)
```

Chi tiết một Request là một dòng chảy dọc, mỗi bước là một khối có thể thu gọn:

```
REQ-123  Thêm đồng bộ site Jira      [change_request · M · normal]   Rủi ro: Trung bình
─────────────────────────────────────────────────────────────────────────────────────
 ● Yêu cầu     ✓ đã xác nhận          (thu gọn)
 ● Giải pháp   ▶ đang chờ bạn chọn    [ Xem 3 phương án ]   ← nút chính duy nhất
 ○ Kế hoạch    chưa bắt đầu
 ○ Thực thi    chưa bắt đầu
 ○ Hoàn tất
```

- Mỗi lúc chỉ **một nút chính** (hành động tiếp theo). Bước đã xong thu gọn thành một dòng, bấm để mở.
- Hỏi lại (Clarification) hiện ngay tại bước đang chờ, không mở trang riêng.

## 4. Một canvas, nhiều góc nhìn (lens)

Một thành phần đồ thị duy nhất, đổi lens bằng chip ở thanh trên. Người dùng học một lần, dùng cho mọi chiều.

| Lens | Node | Cạnh | Màu và dấu hiệu | Trả lời câu hỏi |
|---|---|---|---|---|
| **Luồng** | Các bước Request | Chuyển trạng thái | Trạng thái | Đang ở đâu |
| **Kiến trúc** | Service, module | Phụ thuộc, gọi | Cạnh thêm hoặc bớt (nét liền hoặc đứt, nhãn "+" "−") | Thay đổi cấu trúc gì |
| **Hợp đồng** | proto, kênh WS, sự kiện, tool MCP | Ai gọi hoặc dùng | Đánh dấu phá vỡ tương thích | Ai bị vỡ |
| **Dữ liệu** | Bảng, migration | Khoá, chủ sở hữu | Cờ không đảo ngược, hai dialect | Đụng dữ liệu nào |
| **Phạm vi ảnh hưởng** | Symbol, luồng thực thi | Gọi | Vòng đồng tâm theo bước nhảy (đổi, người gọi trực tiếp, gián tiếp) | Lan tới đâu |
| **Kế hoạch** | Phase, Task | Phụ thuộc | Biểu đồ nhiệt rủi ro, trạng thái | Làm theo thứ tự nào |
| **Thực thi** | Task | Phụ thuộc | Trạng thái trực tiếp, lệch so với dự kiến | Đang chạy thế nào |

Cùng một dữ liệu node và cạnh, chỉ khác bộ lọc và cách tô màu. Nên có một **hợp đồng dữ liệu đồ thị chung**: node `{id, kind, label, group, risk, status}`, cạnh `{from, to, kind, change: added|removed|unchanged}`. Backend trả đúng dạng này cho từng lens.

## 5. Giữ cho đơn giản dù đồ thị nhiều thông tin

| Kỹ thuật | Cách làm |
|---|---|
| **Tóm tắt trước, đồ thị sau** | Mặc định chỉ hiện thẻ rủi ro (mức, điểm, ba lý do chính) và nút "Xem đồ thị". Đồ thị là lớp chi tiết, không phải trang đầu |
| **Chọn lens mặc định thông minh** | Mở sẵn lens có chiều rủi ro cao nhất |
| **Gom nhóm** | Giới hạn khoảng 50 node nhìn thấy; phần còn lại gom thành "+12 service không bị ảnh hưởng", bấm để mở |
| **Thu phóng theo ngữ nghĩa** | Thu nhỏ: service. Phóng to: module. Phóng sát: file và symbol |
| **Chế độ tập trung** | Bấm node: làm sáng lân cận, làm mờ phần còn lại; Esc để thoát |
| **Trước và sau** | Một công tắc, không phải hai đồ thị cạnh nhau; cạnh thêm hoặc bớt có nhãn |
| **Luôn có dạng danh sách** | Mỗi đồ thị có nút chuyển sang bảng cùng dữ liệu. Dùng được bằng bàn phím, đọc được bằng trình đọc màn hình, nhanh hơn khi chỉ cần tìm một mục |
| **Tìm và nhảy bằng `cmdk`** | `/` mở tìm node; Enter để nhảy tới |
| **Không mã hoá chỉ bằng màu** | Kèm hình dạng và nhãn chữ (rủi ro Cao có biểu tượng và chữ, không chỉ màu đỏ) |

## 6. Từng màn hình

**Giải pháp.** Các phương án là một **bảng so sánh cạnh nhau** (so sánh chính là mục đích, nên hợp lý theo STYLEGUIDE): hàng là chiều (rủi ro, effort, phá vỡ tương thích, dữ liệu, quay lui), cột là phương án. Mỗi cột có huy hiệu rủi ro, một đồ thị thu nhỏ và nút "Chọn". Phương án đề xuất được đánh dấu nhưng không chọn sẵn. Chọn xong thì nút chính chuyển sang "Duyệt giải pháp". Từ chối bắt buộc nêu lý do.

**Kế hoạch.** Dạng cây Plan → Phase → Task (danh sách) và đồ thị (lens Kế hoạch), chuyển bằng một công tắc. Duyệt Plan và duyệt từng Phase nằm đúng chỗ của Phase đó. Phát hiện rủi ro mức Cao trở lên có hộp xác nhận chấp nhận từng mục.

**Thực thi.** Hàng tiến độ theo Phase, Task đang chạy ở đầu. Bấm Task mở ngăn bên với nhật ký, kết quả kiểm tra, lệch so với dự kiến (file thực tế so với phạm vi). Lens Thực thi cho cái nhìn tổng quan; lệch vượt ngưỡng hiện thành một dải cảnh báo kèm nút "Xem và duyệt lại".

**Task.** Dùng lại Board, Tree, Graph hiện có; lọc ẩn task `plan` và `phase` khỏi Board mặc định (đã nêu ở CR-REQ-021).

**Hộp duyệt và Backlog.** Danh sách theo hàng đợi, Enter mở, một phím để duyệt kèm xác nhận khi mức rủi ro cao; không duyệt hàng loạt cho mục có tác động.

## 7. Quy tắc trải nghiệm áp dụng (từ STYLEGUIDE)

- **Một nút chính mỗi khối**; hành động hiếm (huỷ, đổi loại, ghi đè) vào menu thừa. Huỷ và Đóng không phải hành động phá huỷ.
- **Focus mặc định** ở hành động chính của hộp thoại; Enter xác nhận, Esc thoát.
- **Phản hồi theo thời gian chờ:** sinh Solution và Plan mất vài giây đến phút nên có nhãn từng giai đoạn; dưới 1 giây chỉ làm mờ nút. Với SSH hoãn hiển thị tải khoảng 200 ms nhưng vô hiệu nút ngay.
- **Không nói quá trong chữ:** không viết "đã phân tích xong" khi chưa có kết quả; không viết "an toàn", mà viết "rủi ro Thấp, đánh giá lúc <giờ>, dựa trên <công cụ>".
- **Trạng thái rỗng và lỗi:** có hành động cụ thể (ví dụ "Chưa có dev server kết nối" kèm nút kết nối); lỗi cần đọc thì hiện tại chỗ, không chỉ toast.
- **Màu:** dùng token hiện có trong `main.css`, không thêm giá trị màu mới. Khi dùng lại `TaskDAGView` thì thay hex cứng bằng token.
- **Phím tắt:** nhãn theo nền tảng (⌘ trên Mac, Ctrl trên Linux và Windows); chỉ hiện chip phím tắt khi đã triển khai thật.

## 8. Cần làm ở frontend

| Việc | Ghi chú |
|---|---|
| Thành phần đồ thị chung và hợp đồng dữ liệu đồ thị | Một node và một cạnh tuỳ biến dùng token; lens là cấu hình |
| Thư viện bố trí tự động | Cần chọn (elkjs hoặc dagre); hiện chưa có |
| Chuyển `TaskDAGView` sang token màu | Sửa hex cứng |
| Gom nhóm, thu phóng ngữ nghĩa, chế độ tập trung | Trên React Flow |
| Chế độ danh sách tương ứng cho mọi đồ thị | Bảng ảo hoá với `react-virtual` |
| Màn hình: Hộp việc cần tôi, chi tiết Request dạng dòng chảy, bảng so sánh Solution, ngăn bên thực thi | Mở rộng CR-REQ-019 đến 023 |
| Truy vấn đồ thị từ backend | CR-REQ-030 (tác động) cần trả dữ liệu đúng dạng đồ thị |

## 9. Chưa kiểm chứng

| Điểm | Ghi chú |
|---|---|
| Hiệu năng React Flow với hàng trăm node | Cần thử với dữ liệu thật của repo (cỡ hàng nghìn symbol) |
| Token màu cho rủi ro Cao và Nghiêm trọng | Chưa đọc `main.css`; nếu thiếu thì phải thêm token và cập nhật STYLEGUIDE |
| Phần nào của `TaskGraph.tsx` và các màn hình Task hiện có dùng lại nguyên được | Chưa đọc hết |
| Màn hình hẹp và ứng dụng di động (Orca có mobile companion) | Đề xuất mặc định: di động chỉ có hộp duyệt và tóm tắt, đồ thị chỉ để xem; chưa kiểm tra |
| Chế độ sáng và tối cho các lens | Chưa kiểm tra |
| Thư viện bố trí tự động | Chưa chọn, cần đánh giá kích thước bundle và giấy phép |

## 10. Bước tiếp

Đề xuất **CR-REQ-032 (thành phần đồ thị và lens, đã viết)** và cập nhật CR-REQ-019 đến 023 theo bố cục trên. Số hiệu đã thống nhất: đồ thị là CR-REQ-032, Source Registry là CR-REQ-031.

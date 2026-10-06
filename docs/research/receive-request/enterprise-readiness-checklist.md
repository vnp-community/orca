# Danh sách kiểm tra sẵn sàng trước khi tạo CR và đưa lên mức doanh nghiệp

| Trường | Giá trị |
|---|---|
| **Ngày** | 2026-10-06 |
| **Loại** | Đánh giá khoảng trống và kế hoạch chuẩn bị (chưa phải CR) |
| **Câu hỏi** | Còn thiếu mục nào trước khi tạo CR để hệ thống thực thi và chạy được ở mức enterprise-level, product-ready? |
| **Phương pháp** | Tổng hợp từ những gì đã đọc trong repo và các file nghiên cứu cùng thư mục. **Chưa chạy hệ thống, chưa đo tải, chưa kiểm chứng môi trường khách hàng.** |
| **Liên quan** | [README.md](./README.md), [CR v6](../../crs/v6/README.md), [request-pipeline-existing-capabilities-and-build-scope.md](./request-pipeline-existing-capabilities-and-build-scope.md), [ai-steps-and-dev-server-connection-flows.md](./ai-steps-and-dev-server-connection-flows.md) |

## 1. Kết luận ngắn

Phần còn thiếu chủ yếu **không phải CR**. Đó là: (a) chứng minh tính khả thi của các giả định, (b) định nghĩa sản phẩm và phạm vi bản đầu, (c) các yêu cầu phi chức năng ở mức doanh nghiệp, (d) chất lượng AI đo được, (e) vận hành và triển khai. Hiện có 25 CR cộng các đề xuất 026 đến 031, nhưng gần như toàn bộ dựa trên đọc code, chưa chạy thử điều gì.

## 2. Việc cần làm trước khi viết thêm CR (chặn)

### 2.1 Thử nghiệm kiểm chứng tính khả thi (spike)

Nhiều CR đứng trên giả định chưa kiểm chứng; một giả định sai thì cả nhóm CR phải viết lại. Mỗi spike cần tiêu chí đạt rõ ràng.

| Spike | Giả định cần kiểm chứng | CR bị ảnh hưởng nếu sai |
|---|---|---|
| Đường AI trên dev server thật | `ai.complete` có khoá API trong môi trường agent; trả JSON đúng khuôn với prompt cỡ Plan; thời gian chấp nhận được | 005, 007, 012 |
| `agent.execPrompt` | Chạy ổn định trong worktree thật; trả được khối kết quả ở cuối đầu ra; giới hạn 15 phút đủ cho task thực | 008, 013, 029 |
| Worktree dùng chung giữa các task | project-service chấp nhận một worktree gắn nhiều task; hai run nối tiếp đúng | 013 (ép chạy tuần tự) |
| `claude --print` với hook, skill, MCP, slash command | Có dùng được không; chế độ chỉ đọc có cờ nào | 008, 026 |
| CodeGraph và GitNexus làm nguồn | Chạy trên dev server; đầu ra dùng được bằng chương trình; độ chính xác xuyên ranh giới gRPC, WS, outbox | 030, 031 |
| Jira thật | Đường từ issue đến Request đến worktree chạy được; workflow Jira tuỳ biến; `task_sources` trên server dev hiện có 0 dòng | 004, 024 |
| MCP ngoài | Registry của `mcp-service` đã dùng được chưa; agent nhận cấu hình MCP | 017, nguồn dữ liệu |
| Hai DB | Cột sinh `STORED`, `SKIP LOCKED`, CHECK trên MySQL (và TiDB nếu hỗ trợ) | mọi migration |
| Tải | Số Request, số task, số lời gọi AI đồng thời của một tenant | kích thước hạ tầng |

### 2.2 Hợp nhất 25 CR hiện có

Các agent soạn song song để lại điểm lệch nhau, đã liệt kê ở [README v6 mục 8](../../crs/v6/README.md). Cần một lượt đối soát chéo về: tên bảng, trigger, mã lỗi, RPC thiếu, số migration, và quyền ghi ở mức Request (chưa ai chốt). Cập nhật README v6 theo kết quả để hai nơi không nói khác nhau.

### 2.3 Chốt quyết định còn mở

| Quyết định | Trạng thái |
|---|---|
| Plan và Phase dùng lại Task với `type` mới | Là cách hiểu của người soạn, cần xác nhận |
| Backlog là view tính toán, không phải status backend | Là cách hiểu của người soạn, cần xác nhận |
| Trạng thái `in_progress`, `review`, `done` của Plan/Phase suy ra từ con | Mặc định đề xuất, chưa xác nhận |
| `TaskNumber` cho Plan và Phase | Mặc định đề xuất (bỏ qua), chưa xác nhận |
| Cột `backlog` ở frontend | Mặc định đề xuất (gỡ), chưa xác nhận |
| Số hiệu CR-REQ-031 bị đề xuất hai lần | Cần thống nhất |
| Quyền ghi ở mức Request (Request chưa có Grant) | Chưa chốt |
| Bật cờ theo loại Request (hiện chỉ theo tenant) | Chưa chốt |

### 2.4 Ghi lại quyết định kiến trúc (ADR)

Đặt trong `docs/adrs/` (đã có). Tối thiểu:
- Tách `request-service`.
- Dùng lại Task cho Plan và Phase.
- Approval tổng quát (giữ `DecisionGate` riêng).
- Đường AI đi qua dev server, không gọi LLM trực tiếp.
- Mô hình chuẩn và bản chiếu cho công cụ ngoài.
- Điểm số rủi ro tính bằng quy tắc, không do AI chấm.

## 3. Định nghĩa sản phẩm (chưa có)

| Còn thiếu | Vì sao cần |
|---|---|
| Tài liệu sản phẩm ngắn: người dùng chính (ai tạo, ai duyệt, ai vận hành), vấn đề, giá trị | CR hiện mô tả cách làm, chưa nêu vì ai và để đo gì |
| **Phạm vi bản đầu (MVP) và thứ tự giá trị** | 11 loại Request và 25 đến 31 CR không ra cùng lúc. Nên chọn lát cắt dọc nhỏ chạy đầu cuối (ví dụ `change_request` và `bug` từ Jira đến PR) |
| Chỉ số thành công (KPI) | Thời gian từ Request đến PR, tỉ lệ task qua cổng lần đầu, tỉ lệ bị từ chối, chi phí AI mỗi Request |
| Thử nghiệm giao diện với người dùng | Bố cục đề xuất chưa có nguyên mẫu hay phản hồi |
| Kế hoạch thí điểm (pilot) | Chọn một đội, một project, tiêu chí vào và ra |
| Mô hình chi phí và giá | AI chạy theo từng bước tốn tiền; cần ngân sách theo tenant |

## 4. Yêu cầu phi chức năng ở mức doanh nghiệp

Phần này gần như vắng trong 25 CR. Mỗi mục nên là một tài liệu riêng hoặc một mục bắt buộc trong CR.

| Lĩnh vực | Còn thiếu | Điểm đáng chú ý trong repo |
|---|---|---|
| **Bảo mật** | Mô hình đe doạ cho luồng AI; chèn chỉ dẫn qua nội dung ngoài; cách ly tenant; quản lý bí mật; kiểm tra bảo mật độc lập | README `api-gateway` tự ghi chưa có kiểm quyền OPA trước định tuyến ("Still not production-safe"). Mọi RPC của `request-service` phải tự kiểm quyền |
| **Quyền và vai trò** | Mô hình quyền ở mức Request (Request chưa có Grant); tách nhiệm vụ; ai được duyệt gì | Vai trò hiện chỉ `admin\|user` và team |
| **Tuân thủ và dữ liệu** | Lưu giữ và xoá dữ liệu, quyền riêng tư, vị trí dữ liệu, **mã nguồn gửi cho nhà cung cấp LLM** (hợp đồng xử lý dữ liệu, nhà cung cấp nào) | Chưa thấy trong docs |
| **Độ tin cậy** | Mục tiêu dịch vụ (SLO), chịu lỗi khi dev server mất kết nối, thử lại, áp lực ngược, ngắt mạch | Task đang chạy không dừng được khi Request bị trả hoặc huỷ |
| **Mở rộng và công suất** | Số tenant, Request, đồng thời; giới hạn tốc độ theo tenant cho AI | Chưa đo |
| **Kiểm soát chi phí AI** | Ngân sách và hạn mức theo tenant, theo loại Request; cảnh báo; chặn khi vượt | `usage-service` có thể là nền |
| **Quan sát** | Chỉ số, nhật ký, trace, cảnh báo, bảng điều khiển cho luồng mới | `common/outbox` và `common/eventbus` không truyền trace context; `issue-status-sync` chưa có `/metrics` |
| **Sao lưu và phục hồi thảm hoạ** | Cho DB mới của `request-service` | |
| **Kiểm toán** | Ghi ai sinh, ai chọn, ai duyệt; lưu bao lâu; xuất cho tuân thủ | `common/auditclient.Append` chưa mang `actor_type`, `target_type`, `metadata_json` (đã nêu ở CR-024) |
| **Khả dụng truy cập và đa ngôn ngữ** | Giao diện đồ thị phải dùng được bằng bàn phím; chuỗi dịch | Có dạng danh sách thay thế (đã đề xuất ở file frontend) |
| **Tương thích ngược** | Luồng "Start work" từ Jira hiện có cùng tồn tại với Request flow; có chuyển dữ liệu cũ không | |
| **Phiên bản API và hợp đồng** | Phiên bản schema, chính sách ngừng hỗ trợ | `make proto-lint` chạy `buf breaking ... \|\| true` nên không chặn gì |

## 5. Chất lượng AI

| Còn thiếu | Ghi chú |
|---|---|
| **Bộ đánh giá (eval harness)** với tập mẫu vàng | Đo chất lượng phân loại, Solution, Plan, TaskSpec; chạy trong CI khi đổi prompt |
| Quản lý phiên bản prompt và template | Mỗi Solution và Plan ghi phiên bản (đã đề xuất) |
| Trừu tượng hoá nhà cung cấp model và phương án dự phòng | `ai.complete` mặc định cứng một model; chưa có chọn model theo bước hay chuyển khi lỗi |
| Chính sách người trong vòng lặp | Khi nào bắt buộc duyệt, khi nào được tự động (theo loại, size, mức rủi ro) |
| Biện pháp chống ảo giác | Trích dẫn nguồn, kiểm tra tồn tại đường dẫn và lệnh, kiểm chứng độc lập sau chạy |
| Khả năng tái lập và lưu vết | Lưu đầu vào, prompt, phiên bản, đầu ra để điều tra |
| Hiệu chỉnh điểm rủi ro bằng dữ liệu thật | Cần giai đoạn chạy bóng (shadow) trước khi dùng điểm để chặn cổng |

## 6. Vận hành và triển khai

| Còn thiếu | Ghi chú |
|---|---|
| Môi trường thử (staging) có Jira, GitHub, dev server thật | `deploy/dev` có chạy được trong CI hay không: chưa kiểm chứng |
| Chiến lược triển khai và quay lui cho `request-service` | CR-025 đã phác; cần làm thành kế hoạch có diễn tập |
| Cờ tính năng theo tenant và theo loại Request | Hiện chỉ theo tenant (CR-025) |
| Runbook và quy trình trực | Cho lỗi AI, dev server mất kết nối, Request kẹt |
| Hỗ trợ khách hàng | Cách người dùng xem vì sao một Request bị trả hoặc lỗi |
| Tài liệu người dùng và quản trị | |
| Quy trình thêm một loại Request hoặc nguồn mới | Để không phải sửa nhiều chỗ |

## 7. Pháp lý và nhà cung cấp

- Giấy phép và điều khoản của OpenSpec, Spec Kit, các máy chủ MCP bên ngoài.
- Thoả thuận xử lý dữ liệu với nhà cung cấp LLM (mã nguồn khách hàng đi ra ngoài).
- Chính sách khi khách hàng không cho phép dữ liệu ra khỏi mạng (chế độ tại chỗ, model nội bộ).

## 8. Thứ tự đề xuất

| Giai đoạn | Việc | Kết quả |
|---|---|---|
| **0. Quyết định** (vài ngày) | Xác nhận hai cách hiểu; chọn MVP và lát cắt dọc; chọn số hiệu CR; viết các ADR chính | Phạm vi và quyết định chốt |
| **1. Kiểm chứng** (1 đến 2 tuần) | Chạy các spike ở mục 2.1 trên môi trường thật; báo cáo ngắn mỗi spike | Giả định đúng hoặc sai, có số đo |
| **2. Nền tảng phi chức năng** | Mô hình đe doạ, mô hình quyền Request, chính sách dữ liệu, ngân sách AI, SLO, kế hoạch eval | Tài liệu ràng buộc cho CR |
| **3. Hợp nhất CR** | Đối soát 25 CR, cập nhật README v6, viết CR 026 đến 031, sửa theo kết quả spike | Bộ CR nhất quán |
| **4. Triển khai lát cắt MVP** | Theo đợt 1 và 2 của README v6, sau cờ tính năng | Chạy đầu cuối trên một project thí điểm |
| **5. Thí điểm và hiệu chỉnh** | Chạy bóng điểm rủi ro, đo KPI, sửa prompt bằng eval | Số liệu thật |
| **6. Mở rộng** | Thêm loại Request, nguồn dữ liệu, đồ thị đầy đủ | Sản phẩm hoàn thiện |

## 9. Nhận định

- **Rủi ro lớn nhất là thiếu kiểm chứng, không phải thiếu CR.** Điểm chặn có khả năng cao nhất: phụ thuộc dev server đang kết nối cho mọi bước AI; khoá API đến agent bằng đường nào; worktree dùng chung (ép chạy tuần tự, thời gian thực thi dài).
- **Phạm vi hiện quá rộng cho một lần ra mắt.** 11 loại Request, 9 loại nguồn, đồ thị 7 lens, chấm điểm rủi ro đều có giá trị, nhưng nên ra bằng lát cắt nhỏ chạy thật rồi mở rộng.
- **Phần chưa được xem kỹ và có thể che thêm vấn đề:** hiệu năng, dung lượng, mô hình triển khai thật của khách hàng, cách đội đang vận hành dev server, ràng buộc pháp lý của khách hàng. Cần chủ sản phẩm bổ sung.

## 10. Việc đề xuất làm tiếp

| Việc | Ghi chú |
|---|---|
| Soạn kế hoạch cho từng spike ở mục 2.1 | Mục tiêu, bước thực hiện, môi trường, tiêu chí đạt, rủi ro, người làm |
| Soạn các ADR ở mục 2.4 | Đặt trong `docs/adrs/` |
| Soạn tài liệu sản phẩm ngắn và chọn MVP | Mục 3 |
| Đối soát 25 CR và cập nhật README v6 | Mục 2.2 |

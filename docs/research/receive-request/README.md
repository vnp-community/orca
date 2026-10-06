# Nghiên cứu: luồng Request → Solution → Plan → Phase → Task

Thư mục này gom toàn bộ nghiên cứu cho luồng nhận yêu cầu từ Jira, GitHub, MCP, phân loại, lập giải pháp, kế hoạch, thực thi bằng AI agent và phản hồi ngược. Kết quả đã chuyển thành các CR trong [docs/crs/v6/](../../crs/v6/README.md). Các file dưới đây là cơ sở và phần mở rộng đang đề xuất thêm.

> Trạng thái chung: **đề xuất, chưa triển khai**. Phần lớn nội dung đọc từ docs và code, chưa chạy hệ thống. Mỗi file có mục "chưa kiểm chứng".

## Thứ tự đọc gợi ý

| # | File | Nội dung | Ngày |
|---|---|---|---|
| 1 | [request-to-task-pipeline-gap-analysis.md](./request-to-task-pipeline-gap-analysis.md) | Luồng mục tiêu và đối chiếu 15 bước với hiện trạng Orca (viết từ docs; có ghi chú đính chính) | 2026-10-05 |
| 2 | [request-pipeline-existing-capabilities-and-build-scope.md](./request-pipeline-existing-capabilities-and-build-scope.md) | Orca đã có gì (đọc code), cần xây gì, bốn quyết định kiến trúc (`request-service` riêng, Plan/Phase là Task, Approval tổng quát, backlog là view) | 2026-10-05 |
| 3 | [request-classification-and-flows.md](./request-classification-and-flows.md) | 11 loại Request, luồng theo loại, máy trạng thái, đổi loại, trả backlog | 2026-10-05 |
| 4 | [solution-plan-execution-walkthrough.md](./solution-plan-execution-walkthrough.md) | Chi tiết cách sinh Solution, lập Plan và thực thi Plan (theo CR-007, 012, 013) | 2026-10-06 |
| 5 | [ai-steps-and-dev-server-connection-flows.md](./ai-steps-and-dev-server-connection-flows.md) | Các bước dùng AI và luồng kết nối dev server qua agent (`ai.complete`, `agent.execPrompt`, ba chế độ kết nối) | 2026-10-06 |
| 6 | [openspec-and-ai-tooling-integration.md](./openspec-and-ai-tooling-integration.md) | Dùng OpenSpec và các công cụ AI khác; có nên đổi luồng hoặc mở rộng agent | 2026-10-06 |
| 7 | [artifact-formats-ontology-and-execution-readiness.md](./artifact-formats-ontology-and-execution-readiness.md) | Định dạng, ontology, kiểm soát, bổ sung dữ liệu, xác nhận lựa chọn; hợp đồng thực thi (`TaskSpec`) và cổng sẵn sàng | 2026-10-06 |
| 8 | [impact-assessment-and-risk-scoring.md](./impact-assessment-and-risk-scoring.md) | Đánh giá tác động và chấm điểm rủi ro cho Solution và Plan | 2026-10-06 |
| 9 | [frontend-visualization-and-ux.md](./frontend-visualization-and-ux.md) | Hiển thị Request, Solution, Plan, thực thi bằng đồ thị đa chiều (lens) và nguyên tắc trải nghiệm | 2026-10-06 |
| 10 | [data-sources-and-mcp-integration.md](./data-sources-and-mcp-integration.md) | Danh mục nguồn dữ liệu cần có và kiến trúc kết nối qua MCP (Source Registry, Context Pack) | 2026-10-06 |
| 11 | [enterprise-readiness-checklist.md](./enterprise-readiness-checklist.md) | Còn thiếu gì trước khi tạo CR để đạt mức doanh nghiệp: spike, hợp nhất CR, định nghĩa sản phẩm, phi chức năng, chất lượng AI, vận hành | 2026-10-06 |

## Các CR đề xuất thêm (đã viết tại docs/crs/v6, ngày 2026-10-06; trạng thái các CR 026 đến 036 đều là đề xuất)

| Mã đề xuất | Nội dung | Nguồn |
|---|---|---|
| CR-REQ-026 | OpenSpec solution engine | file 6 |
| CR-REQ-027 | Lược đồ và ontology có phiên bản, bản chiếu Markdown/YAML | file 7 |
| CR-REQ-028 | Hỏi lại (Clarification) và ghi nhận quyết định (Decision), trạng thái `awaiting_information` | file 7 |
| CR-REQ-029 | Hợp đồng thực thi và cổng sẵn sàng (`TaskSpec`, `ExecutionPacket`, `ReadinessGate`) | file 7 |
| CR-REQ-030 | Đánh giá tác động và chấm điểm rủi ro | file 8 |
| CR-REQ-031 | Source Registry và Context Pack Builder (đã viết) | file 10 |
| CR-REQ-032 | Thành phần đồ thị và lens ở frontend (đã viết) | file 9 |
| CR nhỏ | Hai thay đổi ưu tiên cao ở agent (chế độ chỉ đọc, làm rõ `worktreePath`); hồ sơ năng lực dev server | file 5, 6, 10 |

## Điểm cần chú ý khi dùng các file này

- Docs và code từng lệch nhau (đã gặp: trạng thái `backlog` không có trong code; `mcp-service` đã tồn tại dù docs v5 ghi chưa có). Luôn đọc lại code trước khi triển khai.
- Hai quyết định (Plan/Phase dùng lại Task với `type` mới; backlog là view tính toán) là cách hiểu của người soạn từ câu trả lời của chủ yêu cầu, cần xác nhận.
- Mô tả các công cụ ngoài (OpenSpec, Spec Kit, Taskmaster, Context7, Promptfoo, Semgrep...) dựa trên hiểu biết chung, chưa đối chiếu tài liệu hiện hành.

## Bộ solution và task thực thi

Từ các CR này đã soạn solution và task ở `specs/backend-go/crs/v6/`, `specs/frontend/crs/v6/`, `specs/agent/crs/v6/`; xem README ở từng nơi. Số hiệu các CR đã thống nhất: 026 OpenSpec, 027 lược đồ, 028 hỏi lại và quyết định, 029 hợp đồng thực thi, 030 tác động và rủi ro, 031 nguồn dữ liệu và Context Pack, 032 đồ thị frontend, 033 agent, 034 quản trị AI, 035 bảo mật, 036 giao diện hỏi lại và tác động.

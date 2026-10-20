# Mốc đo v1 (trước khi xóa)

Đo ngày 2026-10-12 trên MiniPay CR-001 (feature `SCHEDULED-TRANSFER`), tsagent v1 bản 0.1 (Oxigraph, ontology `tsp:`). Fixture và đầu ra lưu ở [fixtures-v1/](./fixtures-v1/).

## 1. Số liệu

| Chỉ số | v1, LLM `none` | v1, LLM `claude-cli` (workflow 2026-10-09) |
| --- | --- | --- |
| Đầu vào | `kg-slice.ttl` + `solution.md` + `supplement.md` | như cột trái (workflow chế độ `agent`) |
| Test Requirement | 25 (L1: 1, L2: 23, L3: 1) | 25 |
| Test Design | 40 | 40 |
| Coverage item | 183 | 183 |
| Test Case | 183 (L1: 2, L2: 158, L3: 23) | 183 |
| TC viết bằng LLM | 0 | 183 (chỉ lời văn) |
| AC suy luận (`inferred`) | 0 | 0 |
| Nút `pendingConfirmation` | 56 TC có ghi "chờ xác nhận" | 123 nút |
| Lượt gọi LLM của tsagent | 0 | khoảng 44 (theo cache) |
| Chi phí phiên rà soát testspec-agent | — | khoảng $0,47 |
| Thời gian sinh (không tính LLM) | 1,3 s | — |
| Lỗi chặn T1–T20 | 0 | 0 |

Đầu ra: [fixtures-v1/specs-v1-none/](./fixtures-v1/specs-v1-none/) và [fixtures-v1/specs-v1-llm/](./fixtures-v1/specs-v1-llm/).

## 2. Khoảng trống đã biết của v1 (v2 phải phủ)

| # | Khoảng trống | Nguồn |
| --- | --- | --- |
| G1 | Không có test hai scheduler cùng chiếm một lần chạy (trùng đồng thời) | Đánh giá Test Requirement 2026-10-09, mục 1.2 |
| G2 | Không có test tiến trình chết giữa chiếm lần chạy và chuyển tiền (lease 120 s) | như trên |
| G3 | Không có test ngân sách thời gian xấu nhất (retry × timeout so với 60 s) | như trên |
| G4 | MECH-SchedulerThrottle (20 lần mỗi chu kỳ, 10 rps) không có tiêu chí | như trên |
| G5 | NFR-010 mất phần "99 %", NFR-013 mất điều kiện "100 lần chạy cùng lúc" | như trên |
| G6 | Test NFR vô nghĩa: "101 %", "đúng ngưỡng 0 giao dịch trùng", "6 chu kỳ" | như trên, mục 1.3 |
| G7 | Không có TR mức L3 cho MOD-Notify, INT-02, INT-03 | như trên |
| G8 | Chỉ một hành trình L1; không có hành trình tạo lịch → chạy → lịch sử | như trên |
| G9 | Không có phần hồi quy cho chức năng cũ dùng chung lõi chuyển tiền | như trên |
| G10 | BR-10 (tính theo UTC+7) không có trong TR, TC | như trên |
| G11 | TR chép tên FR; tiêu chí dồn một ô; TR EP, SM, L3 chỉ có câu khung | như trên, mục 2 |
| G12 | Ba TR chồng lấn cùng chức năng, không nói phạm vi | như trên, mục 2.3 |
| G13 | `pendingConfirmation` lan qua Integration dùng chung (123 nút) | như trên, mục 2.4 |
| G14 | Phân quyền không kiểm cho từng endpoint đọc | như trên |

## 3. Hành vi của v1 cần giữ (viết lại thành test v2)

| # | Hành vi | Giai đoạn v2 |
| --- | --- | --- |
| B1 | Phiên bản theo nút; nút trùng nội dung được dùng lại; nút đổi tạo phiên bản mới nối `supersedes` | 2 |
| B2 | SpecVersion Draft được ghi lại tại chỗ, không tạo phiên bản mới | 2 |
| B3 | Nút Baselined bất biến; chỉ chuyển sang Superseded, Retired | 2 |
| B4 | Nút bị CR bỏ thành Retired có CR; nút cũ vẫn truy vấn được | 2 |
| B5 | CR ngầm định khi nguồn đổi mà không có CR | 3 |
| B6 | Lỗi giữa chừng không để lại ghi dở | 2, 7 |
| B7 | md được ghi tại chỗ; từ chối ghi đè bản sửa tay | 6 |
| B8 | Chạy hai lần cùng đầu vào cho md giống từng byte | 6 |
| B9 | Idempotent theo `runKey` | 7 |
| B10 | Chính sách lỗi LLM: 429, 5xx, lỗi mạng retry; timeout unknownOutcome; 4xx permanent; 401, 403 dừng run; JSON sai schema được một lượt sửa | 4 |
| B11 | Cache LLM theo băm nội dung; trần số lượt; circuit breaker | 4 |
| B12 | LLM không đổi số liệu, mã lỗi, số bước; bịa mã thì bị loại | 5, 6 |
| B13 | Dữ liệu nhạy cảm chỉ dùng `secretRef`; quét mẫu bí mật | 6 |
| B14 | MCP: Bearer token, bind loopback mặc định, kiểm Origin, giới hạn kích thước yêu cầu, cắt kết quả 30.000 ký tự, đọc tiếp bằng offset | 7 |
| B15 | `start_run` chạy nền, trả `runId`; `paths` chỉ trong `TSAGENT_SOURCE_ROOTS`; `documents` ghi vào hộp thư | 7 |
| B16 | Duyệt, baseline qua MCP chỉ khi `TSAGENT_MCP_REVIEW=1` | 7 |
| B17 | Mã feature chỉ `A-Z a-z 0-9 . _ -`, tối đa 64 ký tự, chống path traversal | 1 |
| B18 | Baseline bị chặn khi TDD nguồn chưa Baselined (T17) hoặc còn nút suy luận chưa duyệt (T12) | 7 |
| B19 | Workflow dùng các tool: `initialize`, `tools/list`, `start_run`, `get_run_status`, `get_spec` (có `offset`, `max_chars`), `get_coverage`, `get_validation`, `get_traceability`, `list_pending_review`, `approve_nodes`, `baseline_spec` | 7 |
| B20 | `claude-cli`: không tool, không MCP, prompt qua stdin, thư mục trống | 4 |

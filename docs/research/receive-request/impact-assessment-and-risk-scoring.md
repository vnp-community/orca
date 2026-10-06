# Đánh giá tác động và chấm điểm rủi ro cho Solution và Plan

| Trường | Giá trị |
|---|---|
| **Ngày** | 2026-10-06 |
| **Loại** | Đề xuất thiết kế (chưa phải CR, chưa có code) |
| **Bối cảnh** | Solution và Plan khi thực thi có thể đổi kiến trúc, chất lượng code, luồng của các service hiện có. Người dùng cần thấy tác động, chấm điểm rủi ro và kiểm soát trước khi duyệt và trong khi chạy. |
| **Căn cứ trong repo** | `docs/crs/v3/project-workspace/IMPACT-ASSESSMENT-2026-09-15-worktree-session-jira.md` (mẫu đánh giá tác động thủ công, dùng `gitnexus impact`); `backend-go/proto/buf.yaml` (`breaking: FILE`); `backend-go/Makefile` (`proto-lint` chạy `buf breaking ... || true`); `backend-go/ci/`; `backend-go/policy/`; `CLAUDE.md` của repo (bắt buộc chạy `impact` trước khi sửa symbol) |
| **Liên quan** | [artifact-formats-ontology-and-execution-readiness.md](./artifact-formats-ontology-and-execution-readiness.md), [openspec-and-ai-tooling-integration.md](./openspec-and-ai-tooling-integration.md), [CR v6](../../crs/v6/README.md) |

> Mọi thứ trong file này là đề xuất mới, chưa nằm trong CR v6. Các điểm chưa kiểm chứng ở mục 9.

## 1. Tóm tắt cách làm

Biến "tác động và rủi ro" thành một tài liệu có cấu trúc đi kèm Solution và Plan. Dữ liệu đầu vào do công cụ chạy xác định thu thập. Điểm số tính bằng quy tắc minh bạch có phiên bản. AI chỉ giải thích và gợi ý giảm rủi ro, **không được đổi điểm**.

## 2. Những gì đã có để dựa vào

| Có sẵn | Dùng cho |
|---|---|
| Tài liệu đánh giá tác động thủ công (mẫu) | Hình mẫu để tự động hoá: bảng mức độ, blast radius, effort, giá trị; có ghi "LOW risk, 3 impacted" hoặc "CRITICAL" theo `gitnexus impact` |
| CodeGraph và GitNexus (có index sẵn) | Người gọi, luồng thực thi bị ảnh hưởng, mức rủi ro |
| `buf.yaml` với `breaking: FILE` | Kiểm tra phá vỡ tương thích proto. **Lưu ý:** `make proto-lint` hiện có `|| true`, nên không chặn gì; phải chạy riêng và đọc kết quả |
| Approval và `ReadinessGate` (CR-REQ-009, file hợp đồng thực thi) | Điểm gắn cổng duyệt theo mức rủi ro |
| `backend-go/ci/`, `backend-go/policy/` (OPA) | Các kiểm tra và chính sách hiện có |

## 3. Tài liệu ImpactAssessment

Gắn vào ba chỗ, tính ở ba thời điểm:

| Đối tượng | Thời điểm | Nguồn dữ liệu |
|---|---|---|
| **Option của Solution** | Khi sinh Solution (ước lượng) | Khu vực dự kiến (`affected_areas`) cộng truy vấn CodeGraph/GitNexus trên các khu vực đó |
| **Plan, Phase, Task** | Khi sinh Plan (chi tiết hơn) | `scope` của từng task, đồ thị phụ thuộc, danh sách file |
| **Thực tế** | Sau mỗi task và sau mỗi Phase | `git diff` thật, chạy lại phân tích trên các file đã đổi |

Nội dung một bản đánh giá: phiên bản, `digest`, danh sách lần chạy công cụ (công cụ, phiên bản, thời điểm, tuổi của index), điểm từng chiều, mức tổng, các luật kích hoạt cứng, danh sách phát hiện kèm bằng chứng, đề xuất giảm rủi ro.

## 4. Các chiều chấm điểm

| Chiều | Tín hiệu đo được (xác định) |
|---|---|
| **Kiến trúc** | Thêm service hoặc module mới; thêm cạnh phụ thuộc giữa service; đi qua ranh giới (api-gateway, wscompat, proto); đổi phân tầng |
| **Tương thích hợp đồng** | Kết quả `buf breaking`; đổi kênh WS, route HTTP, tool MCP (có parity test); đổi schema sự kiện outbox |
| **Dữ liệu** | Có migration; xoá cột hoặc đổi kiểu; không đảo ngược; cần chạy trên cả Postgres và MySQL; có backfill |
| **Phạm vi ảnh hưởng** | Số người gọi trực tiếp và gián tiếp, số luồng thực thi bị chạm, số service chạm tới (từ GitNexus) |
| **Bảo mật và quyền** | Chạm xác thực, cách ly tenant, credential, phân quyền, OPA; đổi xử lý dữ liệu không tin cậy |
| **Vận hành** | Thứ tự deploy giữa các service; cần cờ tính năng; có đường quay lui không; ảnh hưởng SSH, remote, đa nền tảng (macOS, Linux, Windows) theo AGENTS.md |
| **Chất lượng** | Phần symbol bị ảnh hưởng có test phủ không; độ phức tạp; trạng thái xanh của build, lint, test nền; có dùng `max-lines` disable không (bị cấm) |
| **Bất định** | Số `open_questions`, giả định chưa kiểm chứng, độ tin cậy của AI, index lỗi thời |
| **Quy mô** | Số file, số dòng, số service, số task |

Mỗi chiều có điểm 0 đến 100 theo bảng ngưỡng cố định (ví dụ số người gọi: dưới 5 là thấp, trên 50 là cao). Mức tổng lấy từ chiều cao nhất cộng một phần trung bình có trọng số, **không phải trung bình đơn thuần**, để một chiều nguy hiểm không bị che bởi các chiều thấp. Bậc: Thấp, Trung bình, Cao, Nghiêm trọng.

### 4.1 Luật kích hoạt cứng (nâng mức bất kể điểm)

| Điều kiện | Mức tối thiểu |
|---|---|
| Phá vỡ tương thích proto hoặc kênh công khai | Cao |
| Migration không đảo ngược, hoặc không có bản down | Cao |
| Chạm xác thực, cách ly tenant hoặc credential | Cao |
| Không có đường quay lui, hoặc có bước `irreversible` mà thiếu cờ tính năng | Cao |
| Chạm hơn N service (N cấu hình, đề xuất 3) | Cao |
| Index phân tích lỗi thời hoặc không có kết quả | Tăng một bậc và ghi "chưa đánh giá được" |
| Tác động không đo được ở chiều nào đó | Ghi rõ là bất định, không tính là thấp |

## 5. Hiển thị cho người dùng

**Màn xem Solution (CR-REQ-020):**
1. Thẻ tóm tắt cho mỗi phương án: mức rủi ro, điểm, ba lý do chính, độ tin cậy của đánh giá.
2. Bảng so sánh các phương án theo từng chiều.
3. Bản đồ thay đổi: service và module bị chạm, cạnh phụ thuộc thêm hoặc bớt (trước và sau).
4. Danh sách luồng thực thi bị ảnh hưởng, danh sách hợp đồng bị đổi (proto, kênh WS, sự kiện, bảng DB).
5. Danh sách file và symbol xếp theo phạm vi ảnh hưởng, kèm độ phủ test.
6. Mỗi phát hiện bấm vào xem được bằng chứng (kết quả truy vấn, đường gọi).

**Màn Plan và Phase (CR-REQ-021):**
- Biểu đồ nhiệt theo Phase và task: chỗ rủi ro cao nằm ở đâu trong thứ tự thực thi.
- Gợi ý giảm rủi ro: chia nhỏ Phase, thêm cờ tính năng, thêm test, đổi thứ tự.

**Khi duyệt:**
- Mỗi phát hiện mức Cao trở lên phải được người duyệt **xác nhận chấp nhận** từng mục kèm lý do (`RiskAcceptance`), không chỉ bấm duyệt chung.
- `digest` của bản đánh giá nằm trong `subject_digest` của Approval: đánh giá đổi thì phải duyệt lại.

**Khi đang thực thi:**
- Bảng lệch so với dự kiến: file và service thực tế đã đổi so với kế hoạch, điểm thực tế so với điểm dự kiến.
- Lệch vượt ngưỡng thì tạm dừng Phase và yêu cầu duyệt lại.

**Sau khi xong:** ghi kết quả thật (có sự cố, có phải quay lui không) để hiệu chỉnh bảng ngưỡng theo thời gian.

## 6. Điểm số điều khiển cổng duyệt

| Mức | Hệ quả |
|---|---|
| Thấp | Luồng bình thường theo loại Request |
| Trung bình | Người duyệt phải xem phần tác động; không cho duyệt hàng loạt |
| Cao | Cần người duyệt có vai trò phù hợp (trưởng nhóm hoặc kiến trúc), xác nhận chấp nhận từng phát hiện, bắt buộc có cờ tính năng và đường quay lui, thêm cổng `pre_deploy` |
| Nghiêm trọng | Bắt buộc chia thành các Phase nhỏ có cổng riêng, người duyệt thứ hai, thử nghiệm quay lui trước khi triển khai |

Kiểm soát sau khi bắt đầu thực thi:
- **Lệch kế hoạch:** thay đổi thực tế chạm file hoặc service ngoài dự kiến, hoặc điểm thực tế cao hơn dự kiến quá ngưỡng thì dừng và quay về duyệt lại (tương tự `agent_defect` ở file hợp đồng thực thi).
- **Ghi đè:** người có quyền được bỏ qua một cổng, nhưng phải nêu lý do và được ghi kiểm toán.

## 7. Ai tính điểm

| Việc | Do ai |
|---|---|
| Thu thập dữ liệu (đường gọi, luồng, kiểm tra proto, migration, độ phủ test) | Công cụ chạy xác định (CodeGraph, GitNexus, buf, quét migration, test) |
| Tính điểm và mức | Quy tắc cố định, có phiên bản, ai chạy cũng ra cùng kết quả |
| Giải thích bằng ngôn ngữ, gợi ý giảm rủi ro | AI, đánh dấu rõ là diễn giải, **không được đổi điểm** |

Lý do tách: điểm do AI chấm không tái lập được, khó kiểm toán và dễ bị lèo lái bởi nội dung Request.

## 8. Thay đổi cần có

| Việc | Ghi chú |
|---|---|
| Thực thể `ImpactAssessment`, `RiskAcceptance`, `RiskPolicy` | Ở `request-service`; hai dialect Postgres và MySQL |
| Bộ thu thập đánh giá tác động | Chạy công cụ trên dev server qua `agent.exec` hoặc MCP; cần có CLI và index trên dev server |
| Bảng ngưỡng và luật cứng có phiên bản | Cấu hình theo tenant, có giá trị mặc định |
| Mở rộng `Solution.options[]` và `PlanProposal` | Thêm khối `impact` tóm tắt, tham chiếu bản đánh giá đầy đủ |
| Màn hình tác động ở CR-REQ-020, 021, 022 | Tuân thủ STYLEGUIDE (`guides/STYLEGUIDE.md`) |
| Kết hợp với `ReadinessGate` và CR-REQ-014 | Mức rủi ro quyết định cổng bổ sung |
| Chạy `buf breaking` thật | Không dùng `|| true` làm tín hiệu |

## 9. Chưa kiểm chứng và rủi ro của cách làm

| Điểm | Ghi chú |
|---|---|
| Độ chính xác của GitNexus và CodeGraph trên repo hỗn hợp Go, TypeScript, proto | Chưa biết có bắt được tác động xuyên ranh giới gRPC, kênh WS, sự kiện outbox không; phần này có thể bị đánh giá thấp |
| Index lỗi thời | Hook của repo từng báo index GitNexus lỗi thời; kết quả cũ cho điểm sai. Cần kiểm tra tuổi index và đánh dấu bất định |
| Chạy được trên dev server không | CLI và index phải có ở đó; thời gian truy vấn trên repo cỡ 248 nghìn symbol chưa đo |
| Định dạng đầu ra của công cụ để phân tích bằng chương trình | Chưa kiểm tra |
| Ngưỡng và trọng số | Ước lượng ban đầu, cần hiệu chỉnh bằng dữ liệu thật; chấm sai ở giai đoạn đầu là dự kiến |
| Bản ước lượng ở bước Solution dựa vào `affected_areas` do AI nêu | Chính nó có thể sai; bản thực tế sau khi chạy đáng tin hơn |
| Hai bản agent (`agent/` và `desktop/src/relay/`) | Thay đổi liên quan agent cần kiểm tra cả hai |
| Hoạt động của `buf breaking --against '.git#branch=main'` trong môi trường chạy | Chưa chạy |

## 10. Bước tiếp

Đề xuất **CR-REQ-030 — Đánh giá tác động và chấm điểm rủi ro**, phụ thuộc CR-027 (lược đồ), 029 (hợp đồng thực thi), 009 (Approval) và 014 (chính sách theo loại).

Việc chạy thử nhỏ trước khi viết CR: lấy 3 thay đổi đã có trong lịch sử repo, chạy bộ thu thập và so điểm tính ra với tác động thật đã xảy ra, để xem ngưỡng có hợp lý không.

## Why

Người dùng phải tự viết từng test case: hôm nay `POST /functional/case/ai/chat` và `/transform` trả 501 (`internal/usecase/ai/ai.go:23-29`), `batch/save` lưu từng case ngoài giao dịch nên lỗi giữa chừng để lại dữ liệu dở dang (`ai.go:31-50`), và cấu hình AI của case nằm trong Redis theo hình dạng `AIConfig{workspaceId,…}` khác hợp đồng UI (`ai.go:52-72`). Mục tiêu kinh doanh **BO-02** ("Rút ngắn thiết kế và nhập test case": 20 case từ 4–6 giờ xuống 30–60 phút) phụ thuộc đúng đường AI này.

Yêu cầu REQ-1235 (BL-H02) đòi: mô tả một tính năng qua chat ⇒ nhận ≥ 10 test case hợp lệ (AC-1); chọn và lưu hàng loạt trong một giao dịch có audit (AC-2); JSON sai định dạng ⇒ thử lại một lần rồi báo lỗi, không lưu gì (AC-3); > 20 khối hoặc > 100 case ⇒ từ chối, không lưu gì (AC-4); `case_ai_config` thiếu một trong ba trường bắt buộc ⇒ từ chối kèm lỗi cấu hình (AC-5); lỗi giữa chừng ⇒ không còn case nào của lần lưu đó (AC-6).

## What Changes

- Thêm thư viện thuần `backend/shared/pkg/aiprompt`: system prompt bắt buộc marker `featureCaseStart / caseExpand / featureCaseEnd` mà UI đang tách, `ParseCaseBlocks`, `CaseTransformSchema`, `ValidateTransform`.
- Viết lại `usecase/ai` của case-management theo hợp đồng UI: `Chat` trả **chuỗi markdown** (không SSE), `Transform` gọi LLM theo JSON schema và **thử lại đúng một lần** rồi 502, `BatchSave` tách khối từ `prompt`, chặn **> 20 khối / > 100 case trước mọi lời gọi LLM và mọi lệnh ghi**, rồi tạo case trong **một** giao dịch.
- Số thứ tự `num` của cả lô cấp bằng một `AllocNums(projectID, n)` trong chính giao dịch đó (INT-03, NFR-DA-04); phát một `case.imported` thay vì N `case.created`.
- **BREAKING (nội bộ, theo hợp đồng UI đã đăng ký)**: `case_ai_config` chuyển từ Redis sang MySQL với hình dạng `CaseAiChatConfig{designConfig, templateConfig}`; `domain.AIConfig` và `domain.AIGenerateRequest` cũ bị xoá; `POST /functional/case/ai/chat` không còn `text/event-stream`.
- **BREAKING (hành vi tài liệu hoá)**: chat/transform không còn trả 501; `AI_ENABLED=false` ⇒ 503 có thông điệp thật (không 200 giả) — xem mục Impact về FR-SYS-07.
- LLM gọi qua một interface nhỏ trong case-management, có bản `fake` để test xanh trước khi CR-H01 (`shared/pkg/aiclient`) hoàn tất.

## Capabilities

### New Capabilities
- `case-ai-generation`: chat sinh khối test case, chuyển đổi một khối thành dữ liệu case, lưu hàng loạt trong một giao dịch có audit và giới hạn lô.
- `case-ai-config`: lưu và kiểm tra `case_ai_config` (ba trường bắt buộc) kèm giá trị mặc định khi chưa lưu.

### Modified Capabilities
- (không có spec openspec nào tồn tại trước thay đổi này — `openspec/specs/` rỗng, nên mọi yêu cầu ở đây là ADDED. Yêu cầu bị sửa trong tài liệu nguồn là **FR-SYS-07** của SRS: xem Impact.)

## Impact

**Mã nguồn**
- Mới: `backend/shared/pkg/aiprompt/` (`prompt.go`, `blocks.go`, `transform.go`).
- Sửa: `backend/services/qa-core/case-management/internal/usecase/ai/ai.go`, `internal/handler/ai.go`, `internal/domain/ai.go`, `internal/handler/router.go`, `internal/config/config.go`.
- Mới trong case-management: `internal/usecase/ai/gate.go` + `fake.go`, `internal/repository/mysql/case_ai_config.go`, `internal/usecase/case/batch_add.go`.
- Dữ liệu: `backend/services/qa-core/case-management/migrations/V025__case_ai_config.sql` + khai báo bảng trong `backend/db/ownership.yaml` (ADR-06: mỗi bảng thuộc đúng một service, migration `V###__*.sql` kiểm bằng `dbown`).

**Giao diện HTTP** (đường dẫn giữ nguyên, thân/response đổi theo hợp đồng UI đã đăng ký — NFR-CO-01)
`POST /functional/case/ai/chat`, `POST /functional/case/ai/transform`, `POST /functional/case/ai/batch/save`, `GET /functional/case/ai/get/config`, `POST|PUT /functional/case/ai/save/config` (`internal/handler/router.go:257-265`).

**Yêu cầu và quyết định của tài liệu**
- **FR-SYS-07** (SRS `docs/SRS.md:432`): hôm nay ghi "các hành động chat và chuyển văn bản **chưa nối AI phải trả 501 trung thực**", nghiệm thu "[T] Chat/transform trả 501 có thông điệp; … khi nối AI (T-F3-18) ca kiểm thử đổi sang đường thật". Thay đổi này **thực hiện đúng điều kiện đã ghi trong chính nghiệm thu đó**: nối AI nên 501 biến mất. Đề xuất: PH-6 cập nhật dòng FR-SYS-07 (câu yêu cầu + cột Nghiệm thu) sang đường thật; không sửa SAD.
- **ADR-12 / BRL-64 / NFR-CO-02**: "tính năng hoãn ẩn hoàn toàn … **không được có route 501 hoặc 200 giả**", và ADR-12 ghi hệ quả "có ngoại lệ 501 đang tồn tại cho AI (OI-02)". Thay đổi này **xoá đúng ngoại lệ đó** cho chat/transform của case: route trở thành thật, `AI_ENABLED=false` ⇒ 503 có thông điệp (không 200 giả). F24.9 và F24.10 vẫn hoãn (`tools/conformance/deferred_features.yaml:131-153`) và không được mount.
- **NFR-AU-01** + **INT-07**: mỗi lần `batch/save` sinh một dòng `operation_log` (INSERT-only) qua `internal/middleware/audit_log.go:45` (module `FUNCTIONAL_CASE`), cộng chi tiết thay đổi bằng `audit.RecordAdded` (`shared/pkg/audit/track.go:141`).
- **INT-03** + **NFR-DA-04**: `num` cấp từ `functional_case_num_seq` theo dự án **trong cùng giao dịch tạo case**, không trùng khi tạo đồng thời.
- **FR-CAS-02** (tạo case theo template) và **FR-CAS-03** (lịch sử theo `change_id`): lô AI đi qua đúng đường tạo case và ghi lịch sử như nhập hàng loạt.
- **ADR-06**: bảng mới `case_ai_config` thuộc case-management; SAD (SVC-03) liệt kê kho dữ liệu `functional_case*`, `case_review*`, `test_*` — bảng mới nằm ngoài ba tiền tố đó (xem Open Questions trong design.md).
- **NFR-PE-01** (p95 ≤ 200 ms cho đọc thường): các route gọi LLM không thể đạt ngưỡng này; tài liệu chưa có mục tiêu riêng cho lời gọi LLM (open question).

**Phụ thuộc**: CR-H01 (`shared/pkg/aiclient` + `AIService`) — hôm nay chưa tồn tại (`backend/shared/pkg/aiclient` không có), nên H02 code theo interface và test bằng `fake`.

## Out of Scope

- Đo chất lượng prompt trên 10 mẫu với LLM **thật** (⏸ ENV B10) — mọi nghiệm thu ở đây chạy với `mock-ext` OpenAI và `fake` client.
- F24.9 (sinh case từ thiết kế) và F24.10 — vẫn hoãn, không mount route.
- Không sửa `services/infrastructure/integration-hub/` (thư mục được bảo vệ) và không sửa SAD.

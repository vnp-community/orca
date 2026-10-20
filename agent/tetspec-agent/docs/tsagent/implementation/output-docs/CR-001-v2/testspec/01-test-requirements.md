---
spec: TSD-SCHEDULED-TRANSFER-TestRequirements
feature: SCHEDULED-TRANSFER
docKind: TestRequirements
version: 1
status: Baselined
supersedes: none
baseline: b-bd72d688fca2
changes: [CR-001]
contentHash: af3d2126a97085bd6885f94c5e7a1d5b75dbcff5ec5e000b588c246a94d4b09e
---
# Test Requirements: SCHEDULED-TRANSFER

## 0. Thông tin tài liệu

| Trường | Giá trị |
| --- | --- |
| id | TSD-SCHEDULED-TRANSFER-TestRequirements_v1 |
| Feature | SCHEDULED-TRANSFER |
| Phiên bản | 1 |
| Trạng thái | Baselined |
| Baseline TDD | DOC-SRS-CR001 (b-bd72d688fca2) |
| CR của phiên bản này | CR-001 |
| Ontology | tso v1 |

## 1. Phạm vi và mục tiêu

- Phạm vi: feature SCHEDULED-TRANSFER; mức test: L1 Nghiệp vụ đầu-cuối; L2 Nghiệp vụ; L3 Cô lập service
- Mục tiêu: mọi yêu cầu trong phạm vi có obligation; mọi điều kiện có oracle, coverage item và test case truy vết được

## 2. Test basis

| Tài liệu | Loại | Phiên bản | Trạng thái |
| --- | --- | --- | --- |
| DOC-HLD | SAD/HLD | 0.1 | Baselined |
| DOC-SRS | SRS | 0.1 | Baselined |
| DOC-SRS-CR001 | SRS | 0.1 | Baselined |
| DOC-TDD | TDD | 0.1 | InReview |
| DOC-TEST | Test Plan | 0.1 | InReview |
| SOL-CR-001 | solution |  | Baselined |

Nút TDD trong phạm vi được dùng làm căn cứ: AcceptanceCriterion 43, DataEntity 1, Endpoint 6, Field 25, FunctionalRequirement 10, Mechanism 4, Module 1, NonFunctionalRequirement 5, Response 34, Risk 1, Scenario 1, StateMachine 2, Transition 15; nút ngữ cảnh: 8.

## 3. Phân tích (tầng A)

### TOB-SCHEDULED-TRANSFER-EP-CancelSchedule-contract

- Chủ thể: `EP-CancelSchedule`; nguồn gốc `seedRule`; ưu tiên P1
- Câu hỏi: Chứng minh DELETE /v1/scheduled-transfers/{scheduleId} kiểm tra Authorization và scheduleId, trả đúng 200/401/403/404/409 kèm errorCode theo hợp đồng, và đổi đúng trạng thái lịch và các lần chạy PENDING khi hủy thành công.

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-EP-CancelSchedule-concurrency-concurrent-dup` | concurrency | Hai yêu cầu DELETE cùng Idempotency-Key, cùng scheduleId của lịch ACTIVE, gửi đồng thời. | Lịch chuyển CANCELLED đúng một lần, các lần PENDING chuyển CANCELLED một lần, chỉ một bản ghi audit hủy; cả hai yêu cầu nhận HTTP 200 cùng kết quả. | invariant: 200, nội dung phản hồi giống lần đầu; số bản ghi audit hủy không tăng. (đo: So sánh phản hồi hai lần và đếm audit theo scheduleId.) | `AC-FR-043-2`, `EP-CancelSchedule` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-EP-CancelSchedule-idempotency-payload-mismatch` | idempotency | DELETE cùng Idempotency-Key nhưng khác nội dung/tham số so với lần trước; nguồn chưa nêu chính sách. | Chưa xác định mã lỗi và hành vi; chờ chính sách xung đột khóa idempotency. Bất biến: không tạo thêm hiệu ứng hủy. | invariant: 200, nội dung phản hồi giống lần đầu; số bản ghi audit hủy không tăng. (đo: So sánh phản hồi hai lần và đếm audit theo scheduleId.) | `EP-CancelSchedule` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-EP-CancelSchedule-idempotency-replay` | idempotency | Gửi lại đúng yêu cầu DELETE (cùng Idempotency-Key, cùng scheduleId) sau khi lần đầu đã hủy thành công. | Trả HTTP 200 với kết quả cũ; không ghi thêm audit hủy, không đổi thêm trạng thái lịch hay lần chạy. | invariant: 200, nội dung phản hồi giống lần đầu; số bản ghi audit hủy không tăng. (đo: So sánh phản hồi hai lần và đếm audit theo scheduleId.) | `AC-FR-043-2`, `EP-CancelSchedule` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FLD-Cancel-Authorization-inputValidation-missing` | inputValidation | DELETE /v1/scheduled-transfers/{scheduleId} không có header Authorization. | HTTP 401, errorCode UNAUTHORIZED; lịch và lần chạy không thay đổi, không có audit hủy. | schema: Status 401, body errorCode = UNAUTHORIZED. (đo: Kiểm tra status, errorCode và trạng thái lịch trong DB không đổi.) | `EP-CancelSchedule`, `FLD-Cancel-Authorization`, `RSP-Cancel-401` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FLD-Cancel-scheduleId-inputValidation-missing` | inputValidation | DELETE tới đường dẫn thiếu giá trị scheduleId (path rỗng) hoặc định dạng không hợp lệ; nguồn chưa nêu mã lỗi. | Yêu cầu bị từ chối, không đổi dữ liệu; mã HTTP và errorCode chờ làm rõ. | schema: Bị từ chối, không có thay đổi trạng thái; mã lỗi chờ làm rõ. (đo: Kiểm tra không có lịch nào đổi trạng thái và không có audit hủy.) | `EP-CancelSchedule`, `FLD-Cancel-scheduleId` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-Cancel-200-functional` | functional | DELETE lịch ACTIVE của chính người gọi, không có lần RUNNING. | HTTP 200; lịch chuyển CANCELLED, các lần chạy PENDING chuyển CANCELLED; ghi audit hủy. | postcondition: 200; lịch = CANCELLED; mọi lần PENDING trước đó = CANCELLED. (đo: Đọc lại lịch và các lần chạy từ DB sau phản hồi.) | `AC-FR-043-1`, `AC-FR-043-2`, `EP-CancelSchedule`, `RSP-Cancel-200` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-Cancel-401-access` | access | DELETE với API key sai hoặc không hợp lệ. | HTTP 401, errorCode UNAUTHORIZED; không thay đổi dữ liệu. | errorContract: Status 401, errorCode = UNAUTHORIZED. (đo: Kiểm tra status, errorCode và lịch vẫn ACTIVE.) | `EP-CancelSchedule`, `MOD-API`, `RSP-Cancel-401` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-Cancel-403-access` | access | DELETE lịch tồn tại bởi người gọi có API key hợp lệ nhưng không phải chủ lịch. | HTTP 403, errorCode FORBIDDEN; lịch và các lần chạy không đổi. | errorContract: Status 403, errorCode = FORBIDDEN; lịch vẫn ACTIVE. (đo: Kiểm tra status, errorCode và trạng thái lịch trong DB.) | `EP-CancelSchedule`, `RSP-Cancel-403` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-Cancel-404-inputValidation` | inputValidation | DELETE với scheduleId đúng định dạng nhưng không tồn tại. | HTTP 404, errorCode SCHEDULE_NOT_FOUND; không thay đổi dữ liệu. | errorContract: Status 404, errorCode = SCHEDULE_NOT_FOUND. (đo: Kiểm tra status và errorCode trong phản hồi.) | `EP-CancelSchedule`, `RSP-Cancel-404` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-Cancel-409-inputValidation` | inputValidation | DELETE lịch đã COMPLETED, hoặc lịch có một lần chạy đang RUNNING. | HTTP 409, errorCode SCHEDULE_NOT_CANCELLABLE; lịch và các lần chạy giữ nguyên trạng thái. | errorContract: Status 409, errorCode = SCHEDULE_NOT_CANCELLABLE; trạng thái lịch và lần chạy không đổi. (đo: Kiểm tra phản hồi và đọc lại DB cho cả hai ca.) | `AC-FR-043-3`, `EP-CancelSchedule`, `RSP-Cancel-409` | llm/approved |

### TOB-SCHEDULED-TRANSFER-EP-CancelSchedule-proposal

- Chủ thể: `EP-CancelSchedule`; nguồn gốc `llmProposal`; ưu tiên P1
- Câu hỏi: Điều kiện bổ sung do LLM đề xuất cho EP-CancelSchedule

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-EP-CancelSchedule-audit-p1cc3a7` | audit | Hủy lịch thành công phải ghi audit đầy đủ trường theo FR-048. | Có đúng một bản ghi audit loại sự kiện hủy, chứa correlationId, thời điểm, tác nhân là mã tài khoản và mã lịch. | postcondition: Một bản ghi audit với correlationId, thời điểm, actor = mã tài khoản, loại sự kiện hủy, mã lịch. (đo: Truy vấn bảng audit theo mã lịch sau phản hồi 200.) | `AC-FR-048-1`, `EP-CancelSchedule`, `FR-048` | llm/approved |

### TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract

- Chủ thể: `EP-CreateSchedule`; nguồn gốc `seedRule`; ưu tiên P1
- Câu hỏi: POST /v1/scheduled-transfers có xác thực, kiểm tra đầu vào (thời gian, hạn mức, tài khoản, số lịch tối đa, idempotency) và trả đúng mã HTTP/errorCode theo hợp đồng (201, 200, 400, 401, 403, 404, 409, 422) mà không thay đổi số dư không?

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-concurrency-concurrent-dup` | concurrency | Hai yêu cầu POST cùng Idempotency-Key và cùng nội dung được gửi đồng thời tới /v1/scheduled-transfers. | Chỉ tạo đúng một lịch (một scheduleId). Yêu cầu còn lại nhận kết quả của yêu cầu thắng, không tạo lịch thứ hai. | invariant: HTTP 200, scheduleId = S; tổng số lịch và số bản ghi audit tạo lịch không tăng. (đo: Đếm bản ghi lịch và audit theo Idempotency-Key sau khi cả hai yêu cầu hoàn tất.) | `EP-CreateSchedule` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-idempotency-payload-mismatch` | idempotency | Gửi POST với Idempotency-Key đã dùng nhưng nội dung body khác lần đầu. | Trả HTTP 409, errorCode IDEMPOTENCY_CONFLICT; không tạo lịch mới và không đổi lịch đã có. | errorContract: HTTP 409 với errorCode IDEMPOTENCY_CONFLICT; số lịch không đổi. | `EP-CreateSchedule`, `RSP-Create-409` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-idempotency-replay` | idempotency | Gửi lại đúng yêu cầu đã thành công (cùng Idempotency-Key, cùng nội dung) tới POST /v1/scheduled-transfers. | Trả HTTP 200 kèm lịch đã tạo (cùng scheduleId); không tạo lịch mới, không ghi thêm audit tạo lịch. | invariant: HTTP 200, scheduleId = S; tổng số lịch và số bản ghi audit tạo lịch không tăng. (đo: Đếm bản ghi lịch và audit theo Idempotency-Key sau khi cả hai yêu cầu hoàn tất.) | `EP-CreateSchedule`, `RSP-Create-200` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FLD-Create-Authorization-inputValidation-missing` | inputValidation | POST /v1/scheduled-transfers không có header Authorization. | Trả HTTP 401, errorCode UNAUTHORIZED; không tạo lịch. | schema: HTTP 401 với errorCode UNAUTHORIZED; không có bản ghi lịch mới. | `EP-CreateSchedule`, `FLD-Create-Authorization` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FLD-Create-IdempotencyKey-inputValidation-missing` | inputValidation | POST /v1/scheduled-transfers không có header Idempotency-Key. | Trả HTTP 400, errorCode INVALID_REQUEST; không tạo lịch. | schema: HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. | `EP-CreateSchedule`, `FLD-Create-IdempotencyKey`, `RSP-Create-400` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FLD-Create-amount-inputValidation-max` | inputValidation | POST với amount vượt cận trên 50.000.000 VND (biên: 50000000 hợp lệ, 50000001 không hợp lệ). | amount=50000001 trả HTTP 422, errorCode LIMIT_EXCEEDED; amount=50000000 được chấp nhận (201) nếu các trường khác hợp lệ. | schema: 50000001 -> HTTP 422 LIMIT_EXCEEDED, không tạo lịch; 50000000 -> HTTP 201. | `EP-CreateSchedule`, `FLD-Create-amount`, `RSP-Create-422-amount` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FLD-Create-amount-inputValidation-min` | inputValidation | POST với amount dưới cận dưới 1 (biên: 0 không hợp lệ, 1 hợp lệ). | amount=0 trả HTTP 400, errorCode INVALID_REQUEST; amount=1 được chấp nhận nếu các trường khác hợp lệ. | schema: HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. | `EP-CreateSchedule`, `FLD-Create-amount`, `RSP-Create-400` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FLD-Create-amount-inputValidation-missing` | inputValidation | POST thiếu trường bắt buộc amount trong body. | Trả HTTP 400, errorCode INVALID_REQUEST; không tạo lịch. | schema: HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. | `EP-CreateSchedule`, `FLD-Create-amount`, `RSP-Create-400` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FLD-Create-endDate-inputValidation-missing` | inputValidation | POST thiếu endDate trong body. | Với lịch định kỳ có maxRuns thì vẫn tạo được (201); chỉ khi thiếu cả endDate và maxRuns mới trả 400 INVALID_REQUEST. Hành vi với ONCE chưa rõ. | schema: HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. | `AC-FR-041-1`, `AC-FR-041-2`, `EP-CreateSchedule`, `FLD-Create-endDate` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FLD-Create-firstRunAt-inputValidation-missing` | inputValidation | POST thiếu trường bắt buộc firstRunAt trong body. | Trả HTTP 400, errorCode INVALID_REQUEST; không tạo lịch. | schema: HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. | `EP-CreateSchedule`, `FLD-Create-firstRunAt`, `RSP-Create-400` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FLD-Create-fromAccountId-inputValidation-missing` | inputValidation | POST thiếu trường bắt buộc fromAccountId trong body. | Trả HTTP 400, errorCode INVALID_REQUEST; không tạo lịch. | schema: HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. | `EP-CreateSchedule`, `FLD-Create-fromAccountId`, `RSP-Create-400` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FLD-Create-maxRuns-inputValidation-max` | inputValidation | POST với maxRuns vượt cận trên 60 (biên: 60 hợp lệ, 61 không hợp lệ). | maxRuns=61 trả HTTP 400, errorCode INVALID_REQUEST; maxRuns=60 được chấp nhận nếu các trường khác hợp lệ. | schema: HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. | `EP-CreateSchedule`, `FLD-Create-maxRuns`, `RSP-Create-400` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FLD-Create-maxRuns-inputValidation-min` | inputValidation | POST với maxRuns dưới cận dưới 1 (biên: 0 không hợp lệ, 1 hợp lệ). | maxRuns=0 trả HTTP 400, errorCode INVALID_REQUEST; maxRuns=1 được chấp nhận nếu các trường khác hợp lệ. | schema: HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. | `EP-CreateSchedule`, `FLD-Create-maxRuns`, `RSP-Create-400` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FLD-Create-maxRuns-inputValidation-missing` | inputValidation | POST thiếu maxRuns trong body. | Với lịch định kỳ có endDate thì vẫn tạo được (201); chỉ khi thiếu cả endDate và maxRuns mới trả 400 INVALID_REQUEST. Hành vi với ONCE chưa rõ. | schema: HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. | `AC-FR-041-1`, `AC-FR-041-2`, `EP-CreateSchedule`, `FLD-Create-maxRuns` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FLD-Create-recurrence-inputValidation-enum` | inputValidation | POST với recurrence nằm ngoài tập {ONCE, DAILY, WEEKLY, MONTHLY}. | Giá trị ngoài tập trả HTTP 400, errorCode INVALID_REQUEST; bốn giá trị hợp lệ được chấp nhận khi các trường khác hợp lệ. | schema: HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. | `EP-CreateSchedule`, `FLD-Create-recurrence`, `RSP-Create-400` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FLD-Create-recurrence-inputValidation-missing` | inputValidation | POST thiếu trường bắt buộc recurrence trong body. | Trả HTTP 400, errorCode INVALID_REQUEST; không tạo lịch. | schema: HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. | `EP-CreateSchedule`, `FLD-Create-recurrence`, `RSP-Create-400` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FLD-Create-toAccountId-inputValidation-missing` | inputValidation | POST thiếu trường bắt buộc toAccountId trong body. | Trả HTTP 400, errorCode INVALID_REQUEST; không tạo lịch. | schema: HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. | `EP-CreateSchedule`, `FLD-Create-toAccountId`, `RSP-Create-400` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-Create-200-functional` | functional | POST với Idempotency-Key đã dùng và nội dung giống hệt lần trước. | Trả HTTP 200 kèm lịch đã tạo (cùng scheduleId, status hiện tại của lịch); không tạo lịch mới. | postcondition: HTTP 200, body chứa lịch đã tạo với scheduleId ban đầu. | `EP-CreateSchedule`, `RSP-Create-200` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-Create-201-functional` | functional | POST với đầu vào hợp lệ, Idempotency-Key mới, tài khoản nguồn thuộc chủ API key. | Trả HTTP 201, có scheduleId, status=ACTIVE; số dư hai tài khoản không đổi và không có ledger_entries mới. | postcondition: HTTP 201, scheduleId không rỗng, status=ACTIVE; số dư và số bản ghi ledger_entries không đổi. | `AC-FR-040-1`, `AC-FR-040-2`, `EP-CreateSchedule`, `RSP-Create-201` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-Create-400-inputValidation` | inputValidation | POST với đầu vào vi phạm hợp đồng: thiếu Idempotency-Key/trường bắt buộc, amount không nguyên dương, recurrence sai, định kỳ thiếu cả endDate và maxRuns, maxRuns ngoài 1..60, hoặc firstRunAt sai định dạng. | Mỗi trường hợp trả HTTP 400, errorCode INVALID_REQUEST; không tạo lịch, không ghi nhận hiệu ứng phụ. | errorContract: HTTP 400 với errorCode INVALID_REQUEST cho từng biến thể; số lịch không đổi. | `AC-FR-041-2`, `EP-CreateSchedule`, `RSP-Create-400` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-Create-401-access` | access | Gọi POST /v1/scheduled-transfers với header Authorization thiếu hoặc API key sai. | Trả HTTP 401, errorCode UNAUTHORIZED; không tạo lịch nào và không ghi bản ghi lịch mới. | errorContract: statusCode=401, errorCode=UNAUTHORIZED; số lịch của tài khoản không đổi. | `EP-CreateSchedule`, `FLD-Create-Authorization`, `RSP-Create-401` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-Create-403-access` | access | Gọi POST với API key hợp lệ nhưng fromAccountId không thuộc chủ API key. | Trả HTTP 403, errorCode FORBIDDEN; không tạo lịch. | errorContract: statusCode=403, errorCode=FORBIDDEN; không có lịch mới cho tài khoản B. | `AC-FR-040-5`, `EP-CreateSchedule`, `FLD-Create-fromAccountId`, `RSP-Create-403` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-Create-404-inputValidation` | inputValidation | Gọi POST với toAccountId không tồn tại trong hệ thống. | Trả HTTP 404, errorCode ACCOUNT_NOT_FOUND; không tạo lịch. | errorContract: statusCode=404, errorCode=ACCOUNT_NOT_FOUND; không có lịch mới. | `AC-FR-040-4`, `EP-CreateSchedule`, `FLD-Create-toAccountId`, `RSP-Create-404` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-Create-409-inputValidation` | inputValidation | Gửi lại POST với cùng Idempotency-Key đã dùng nhưng nội dung body khác lần đầu. | Trả HTTP 409, errorCode IDEMPOTENCY_CONFLICT; không tạo lịch mới và lịch đã tạo lần đầu không bị thay đổi. | errorContract: statusCode=409, errorCode=IDEMPOTENCY_CONFLICT; lịch gốc giữ nguyên, tổng số lịch không tăng. | `EP-CreateSchedule`, `FLD-Create-IdempotencyKey`, `RSP-Create-200`, `RSP-Create-409` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-Create-422-amount-inputValidation` | inputValidation | Gọi POST với amount lớn hơn 50.000.000 VND (biên: 50.000.000 hợp lệ, 50.000.001 bị từ chối). | amount=50.000.001 trả HTTP 422, errorCode LIMIT_EXCEEDED; amount=50.000.000 không bị từ chối vì lý do hạn mức này. | errorContract: statusCode=422, errorCode=LIMIT_EXCEEDED; không tạo lịch. | `EP-CreateSchedule`, `FLD-Create-amount`, `RSP-Create-422-amount` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-Create-422-limit-inputValidation` | inputValidation | Tạo lịch thứ 21 cho tài khoản đã có 20 lịch ở trạng thái ACTIVE hoặc PAUSED. | Trả HTTP 422, errorCode SCHEDULE_LIMIT_REACHED; không tạo lịch thứ 21. | errorContract: statusCode=422, errorCode=SCHEDULE_LIMIT_REACHED; số lịch ACTIVE/PAUSED vẫn bằng 20. | `AC-FR-040-6`, `EP-CreateSchedule`, `RSP-Create-422-limit` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-Create-422-same-inputValidation` | inputValidation | Gọi POST với fromAccountId bằng toAccountId. | Trả HTTP 422, errorCode SAME_ACCOUNT; không tạo lịch. | errorContract: statusCode=422, errorCode=SAME_ACCOUNT; không có lịch mới. | `EP-CreateSchedule`, `FLD-Create-fromAccountId`, `FLD-Create-toAccountId`, `RSP-Create-422-same` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-Create-422-time-inputValidation` | inputValidation | Gọi POST với firstRunAt cách hiện tại < 5 phút hoặc > 90 ngày, hoặc endDate trước lần chạy đầu hoặc quá 366 ngày. | Trả HTTP 422, errorCode SCHEDULE_TIME_INVALID; không tạo lịch. | errorContract: statusCode=422, errorCode=SCHEDULE_TIME_INVALID; không có lịch mới. | `AC-FR-040-3`, `EP-CreateSchedule`, `FLD-Create-endDate`, `FLD-Create-firstRunAt`, `RSP-Create-422-time` | llm/approved |

### TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-proposal

- Chủ thể: `EP-CreateSchedule`; nguồn gốc `llmProposal`; ưu tiên P1
- Câu hỏi: Điều kiện bổ sung do LLM đề xuất cho EP-CreateSchedule

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-access-p73a390` | access | fromAccountId khác tài khoản của chủ API key. | Trả HTTP 403, errorCode FORBIDDEN; không tạo lịch. | errorContract: HTTP 403 FORBIDDEN; không có bản ghi lịch mới. | `AC-FR-040-5`, `EP-CreateSchedule`, `RSP-Create-403` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-audit-p28bd0d` | audit | Tạo lịch thành công phải ghi audit. | Có bản ghi audit gồm correlationId, thời điểm, tác nhân là mã tài khoản, loại sự kiện và mã lịch. | acceptanceCriterion: Một bản ghi audit đủ các trường nêu trên. | `AC-FR-048-1`, `EP-CreateSchedule`, `MOD-Audit` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-audit-pf42db3` | audit | Tạo lịch thành công ghi audit. | Có đúng một bản ghi audit loại sự kiện tạo lịch với correlationId, thời điểm, tác nhân là mã tài khoản và mã lịch. | postcondition: Một bản ghi audit chứa correlationId, thời điểm, tác nhân = mã tài khoản, loại sự kiện tạo lịch, scheduleId. | `AC-FR-048-1`, `EP-CreateSchedule`, `FR-048` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-concurrency-pdcbc85` | concurrency | Hai request đồng thời cùng Idempotency-Key và cùng nội dung. | Chỉ một lịch được tạo; request còn lại trả cùng lịch. | invariant: Đúng 1 lịch với key K; hai phản hồi cùng scheduleId. | `EP-CreateSchedule`, `FLD-Create-IdempotencyKey`, `RSP-Create-200` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-errorHandling-pe11059` | errorHandling | toAccountId không tồn tại. | Trả HTTP 404, errorCode ACCOUNT_NOT_FOUND; không tạo lịch. | errorContract: HTTP 404 ACCOUNT_NOT_FOUND; không có bản ghi lịch mới. | `AC-FR-040-4`, `EP-CreateSchedule`, `RSP-Create-404` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-functional-p2341c6` | functional | Lịch MONTHLY đặt ngày 29, 30 hoặc 31 được tạo và các lần chạy sau rơi đúng ngày cuối tháng. | Tạo thành công (201); ở tháng không đủ ngày lần chạy vào ngày cuối tháng, và 29/02 ở năm nhuận. | acceptanceCriterion: Lần chạy ở tháng 30 ngày vào ngày 30, tháng 2 vào ngày 28 (hoặc 29 năm nhuận). | `AC-FR-041-4`, `EP-CreateSchedule`, `FR-041` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-functional-pab4508` | functional | Tạo lịch ONCE hợp lệ thì trả 201, có scheduleId, status ACTIVE. | HTTP 201 kèm scheduleId và status=ACTIVE. | acceptanceCriterion: 201, có scheduleId, status=ACTIVE. | `AC-FR-040-1`, `EP-CreateSchedule`, `RSP-Create-201` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-guardViolation-p588a23` | guardViolation | fromAccountId bằng toAccountId. | Trả HTTP 422, errorCode SAME_ACCOUNT; không tạo lịch. | errorContract: HTTP 422 SAME_ACCOUNT; không có bản ghi lịch mới. | `EP-CreateSchedule`, `RSP-Create-422-same` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-guardViolation-pc9af5d` | guardViolation | Biên giới hạn lịch: tài khoản đã có 20 lịch ACTIVE/PAUSED thì lịch thứ 21 bị từ chối; lịch thứ 20 được chấp nhận. | Lịch thứ 21 trả HTTP 422, errorCode SCHEDULE_LIMIT_REACHED; lịch thứ 20 trả 201. | errorContract: Lịch thứ 20 -> 201; lịch thứ 21 -> 422 SCHEDULE_LIMIT_REACHED. | `AC-FR-040-6`, `EP-CreateSchedule`, `RSP-Create-422-limit` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-idempotency-p50d7db` | idempotency | Gửi lại cùng Idempotency-Key và cùng nội dung thì trả lịch đã tạo. | HTTP 200 trả lịch đã tạo, không tạo lịch mới. | postcondition: 200, cùng scheduleId; tổng số lịch không tăng. | `EP-CreateSchedule`, `RSP-Create-200` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-inputValidation-p4e161f` | inputValidation | Lịch định kỳ thiếu cả endDate và maxRuns; hoặc maxRuns ngoài 1..60. | HTTP 400, errorCode INVALID_REQUEST; không tạo lịch. | errorContract: 400 INVALID_REQUEST. | `AC-FR-041-2`, `EP-CreateSchedule`, `FLD-Create-maxRuns`, `RSP-Create-400` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-invariant-p420954` | invariant | Tạo lịch không làm đổi số dư và không sinh ledger_entries. | Số dư hai tài khoản không đổi, không có bản ghi ledger_entries mới. | invariant: Số dư hai tài khoản và số ledger_entries không đổi. | `AC-FR-040-2`, `EP-CreateSchedule` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-timing-p31f6cf` | timing | Biên thời gian firstRunAt: cách hiện tại dưới 5 phút hoặc trên 90 ngày bị từ chối. | firstRunAt ngoài khoảng 5 phút đến 90 ngày trả HTTP 422, errorCode SCHEDULE_TIME_INVALID; trong khoảng được chấp nhận. | errorContract: Giá trị dưới 5 phút và trên 90 ngày -> 422 SCHEDULE_TIME_INVALID; giá trị trong khoảng -> 201. | `AC-FR-040-3`, `EP-CreateSchedule`, `RSP-Create-422-time` | llm/approved |

### TOB-SCHEDULED-TRANSFER-EP-GetSchedule-contract

- Chủ thể: `EP-GetSchedule`; nguồn gốc `seedRule`; ưu tiên P1
- Câu hỏi: Chứng minh GET /v1/scheduled-transfers/{scheduleId} trả 200 kèm status và nextRunAt cho chủ tài khoản nguồn, và trả đúng 401 UNAUTHORIZED, 403 FORBIDDEN, 404 SCHEDULE_NOT_FOUND khi thiếu/sai API key, sai chủ sở hữu, hoặc không có lịch.

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-FLD-Get-Authorization-inputValidation-missing` | inputValidation | Gọi GET /v1/scheduled-transfers/{scheduleId} với scheduleId hợp lệ nhưng thiếu header Authorization (bắt buộc). | Trả HTTP 401 với errorCode UNAUTHORIZED; không trả dữ liệu lịch. | schema: HTTP 401, body có errorCode = UNAUTHORIZED, không chứa status hay nextRunAt. | `EP-GetSchedule`, `FLD-Get-Authorization`, `RSP-Get-401` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FLD-Get-scheduleId-inputValidation-missing` | inputValidation | Gọi GET tới đường dẫn thiếu giá trị scheduleId (path rỗng) với API key hợp lệ. | Yêu cầu bị từ chối, không trả dữ liệu lịch; mã lỗi cụ thể chưa được nguồn xác định (xem gaps). | schema: Yêu cầu bị từ chối, không có phản hồi 200; mã HTTP và errorCode chờ làm rõ. | `EP-GetSchedule`, `FLD-Get-scheduleId` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-Get-200-functional` | functional | Chủ tài khoản nguồn gọi GET với API key hợp lệ và scheduleId của lịch tồn tại. | Trả HTTP 200 kèm trạng thái lịch (status) và lần chạy kế tiếp (nextRunAt). | postcondition: HTTP 200, body chứa status và nextRunAt khớp với dữ liệu lịch đã lưu. | `AC-FR-042-1`, `EP-GetSchedule`, `RSP-Get-200` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-Get-401-access` | access | Gọi GET với API key sai hoặc không hợp lệ trong header Authorization. | Trả HTTP 401 với errorCode UNAUTHORIZED. | errorContract: HTTP 401, errorCode = UNAUTHORIZED, không lộ dữ liệu lịch. | `EP-GetSchedule`, `MOD-API`, `RSP-Get-401` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-Get-403-access` | access | Người gọi có API key hợp lệ nhưng không phải chủ tài khoản nguồn của lịch. | Trả HTTP 403 với errorCode FORBIDDEN; không lộ dữ liệu lịch. | errorContract: HTTP 403, errorCode = FORBIDDEN, không chứa status/nextRunAt. | `AC-FR-042-4`, `EP-GetSchedule`, `RSP-Get-403` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-Get-404-inputValidation` | inputValidation | Gọi GET với scheduleId đúng định dạng nhưng không tồn tại lịch tương ứng. | Trả HTTP 404 với errorCode SCHEDULE_NOT_FOUND. | errorContract: HTTP 404, errorCode = SCHEDULE_NOT_FOUND. | `AC-FR-042-4`, `EP-GetSchedule`, `RSP-Get-404` | llm/approved |

### TOB-SCHEDULED-TRANSFER-EP-GetSchedule-proposal

- Chủ thể: `EP-GetSchedule`; nguồn gốc `llmProposal`; ưu tiên P1
- Câu hỏi: Điều kiện bổ sung do LLM đề xuất cho EP-GetSchedule

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-EP-GetSchedule-idempotency-p52c40c` | idempotency | GET một lịch là thao tác idempotent: gọi lặp lại nhiều lần không làm thay đổi trạng thái lịch. | Hai lần GET liên tiếp khi lịch không đổi trả cùng status và nextRunAt; dữ liệu lịch không bị thay đổi. | invariant: Hai phản hồi 200 có status và nextRunAt giống nhau; dữ liệu lịch trong kho không đổi sau các lần GET. | `EP-GetSchedule` | llm/approved |

### TOB-SCHEDULED-TRANSFER-EP-ListRuns-contract

- Chủ thể: `EP-ListRuns`; nguồn gốc `seedRule`; ưu tiên P1
- Câu hỏi: GET /v1/scheduled-transfers/{scheduleId}/runs có từ chối đúng đầu vào sai (Authorization, scheduleId, limit 1..100, offset >= 0), trả 401/403/404 đúng mã lỗi và 200 với danh sách lần chạy mới nhất trước gồm scheduledAt, status, transferId, reasonCode không?

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-FLD-ListRuns-Authorization-inputValidation-missing` | inputValidation | Gọi GET /v1/scheduled-transfers/{scheduleId}/runs khi thiếu header Authorization (bắt buộc, Bearer API key). | Hệ thống trả HTTP 401 với errorCode UNAUTHORIZED; không trả dữ liệu lần chạy nào. | schema: HTTP 401, errorCode UNAUTHORIZED | `EP-ListRuns`, `FLD-ListRuns-Authorization`, `RSP-ListRuns-401` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FLD-ListRuns-limit-inputValidation-max` | inputValidation | Gọi GET /runs với limit = 101, vượt cận trên 100 (biên hợp lệ là 100). | Yêu cầu bị từ chối với lỗi kiểm tra đầu vào; limit = 100 vẫn được chấp nhận. Mã HTTP và errorCode chưa xác định trong nguồn. | schema: Yêu cầu bị từ chối với lỗi kiểm tra đầu vào; không có dữ liệu lần chạy | `EP-ListRuns`, `FLD-ListRuns-limit`, `MOD-API` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FLD-ListRuns-limit-inputValidation-min` | inputValidation | Gọi GET /runs với limit = 0, dưới cận dưới 1 (biên hợp lệ là 1). | Yêu cầu bị từ chối với lỗi kiểm tra đầu vào; limit = 1 được chấp nhận. Mã HTTP và errorCode chưa xác định trong nguồn. | schema: Yêu cầu bị từ chối với lỗi kiểm tra đầu vào; không có dữ liệu lần chạy | `EP-ListRuns`, `FLD-ListRuns-limit`, `MOD-API` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FLD-ListRuns-offset-inputValidation-min` | inputValidation | Gọi GET /runs với offset = -1, dưới cận dưới 0 (biên hợp lệ là 0). | Yêu cầu bị từ chối với lỗi kiểm tra đầu vào; offset = 0 được chấp nhận. Mã HTTP và errorCode chưa xác định trong nguồn. | schema: Yêu cầu bị từ chối với lỗi kiểm tra đầu vào; không có dữ liệu lần chạy | `EP-ListRuns`, `FLD-ListRuns-offset`, `MOD-API` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FLD-ListRuns-scheduleId-inputValidation-missing` | inputValidation | Gọi endpoint runs khi thiếu scheduleId (path, bắt buộc), tức đường dẫn không có phần định danh lịch. | Yêu cầu bị từ chối với lỗi kiểm tra đầu vào; không trả danh sách lần chạy. Mã HTTP cụ thể chưa xác định trong nguồn. | schema: Yêu cầu bị từ chối với lỗi kiểm tra đầu vào; không có dữ liệu lần chạy | `EP-ListRuns`, `FLD-ListRuns-scheduleId`, `MOD-API` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-ListRuns-200-functional` | functional | Chủ lịch gọi GET /runs cho lịch tồn tại với Authorization hợp lệ. | HTTP 200; trả các lần chạy sắp mới nhất trước, mỗi lần có scheduledAt, status, transferId, reasonCode. | postcondition: HTTP 200; danh sách giảm dần theo scheduledAt; mỗi phần tử có scheduledAt, status, transferId, reasonCode | `AC-FR-042-3`, `EP-ListRuns`, `FR-042`, `RSP-ListRuns-200` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-ListRuns-401-access` | access | Gọi GET /runs với API key thiếu hoặc sai. | HTTP 401 với errorCode UNAUTHORIZED; không lộ dữ liệu lần chạy. | errorContract: HTTP 401, errorCode UNAUTHORIZED | `EP-ListRuns`, `MOD-API`, `RSP-ListRuns-401` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-ListRuns-403-access` | access | Người gọi có API key hợp lệ nhưng không phải chủ tài khoản nguồn của lịch gọi GET /runs. | HTTP 403 với errorCode FORBIDDEN; không trả lần chạy nào. | errorContract: HTTP 403, errorCode FORBIDDEN | `AC-FR-042-4`, `EP-ListRuns`, `RSP-ListRuns-403` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-ListRuns-404-inputValidation` | inputValidation | Gọi GET /runs với scheduleId không tương ứng lịch nào. | HTTP 404 với errorCode SCHEDULE_NOT_FOUND. | errorContract: HTTP 404, errorCode SCHEDULE_NOT_FOUND | `AC-FR-042-4`, `EP-ListRuns`, `RSP-ListRuns-404` | llm/approved |

### TOB-SCHEDULED-TRANSFER-EP-ListSchedules-contract

- Chủ thể: `EP-ListSchedules`; nguồn gốc `seedRule`; ưu tiên P1
- Câu hỏi: GET /v1/accounts/{accountId}/scheduled-transfers có trả 200 với danh sách lịch của đúng tài khoản, lọc theo status, phân trang theo limit/offset, và trả đúng 400 INVALID_REQUEST, 401 UNAUTHORIZED, 403 FORBIDDEN khi đầu vào hoặc quyền không hợp lệ không?

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-Authorization-inputValidation-missing` | inputValidation | Gọi GET /v1/accounts/{accountId}/scheduled-transfers không có header Authorization (Bearer API key). | Trả HTTP 401 với errorCode UNAUTHORIZED; không trả dữ liệu lịch nào. | schema: HTTP 401, errorCode UNAUTHORIZED, body không chứa phần tử lịch. | `EP-ListSchedules`, `FLD-ListSchedules-Authorization`, `RSP-ListSchedules-401` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-accountId-inputValidation-missing` | inputValidation | Gọi endpoint khi thiếu đoạn path accountId (ví dụ /v1/accounts//scheduled-transfers). | Yêu cầu bị từ chối, không trả danh sách lịch. Mã HTTP và errorCode chưa được nguồn xác định (xem gaps). | schema: Giá trị trong enum: HTTP 200. Giá trị ngoài enum: HTTP 400, errorCode INVALID_REQUEST. | `EP-ListSchedules`, `FLD-ListSchedules-accountId` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-limit-inputValidation-max` | inputValidation | Gọi endpoint với limit = 101, vượt cận trên 100; limit = 100 là giá trị biên hợp lệ. | limit=101 bị từ chối (dự kiến HTTP 400 INVALID_REQUEST, chờ xác nhận); limit=100 được chấp nhận, trả tối đa 100 phần tử. | schema: Giá trị trong enum: HTTP 200. Giá trị ngoài enum: HTTP 400, errorCode INVALID_REQUEST. | `EP-ListSchedules`, `FLD-ListSchedules-limit`, `RSP-ListSchedules-400` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-limit-inputValidation-min` | inputValidation | Gọi endpoint với limit = 0, dưới cận dưới 1; limit = 1 là giá trị biên hợp lệ. | limit=0 bị từ chối (dự kiến HTTP 400 INVALID_REQUEST, chờ xác nhận); limit=1 được chấp nhận, trả tối đa 1 phần tử. | schema: Giá trị trong enum: HTTP 200. Giá trị ngoài enum: HTTP 400, errorCode INVALID_REQUEST. | `EP-ListSchedules`, `FLD-ListSchedules-limit`, `RSP-ListSchedules-400` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-offset-inputValidation-min` | inputValidation | Gọi endpoint với offset = -1, dưới cận dưới 0; offset = 0 là giá trị biên hợp lệ. | offset=-1 bị từ chối (dự kiến HTTP 400 INVALID_REQUEST, chờ xác nhận); offset=0 được chấp nhận, danh sách bắt đầu từ phần tử đầu. | schema: Giá trị trong enum: HTTP 200. Giá trị ngoài enum: HTTP 400, errorCode INVALID_REQUEST. | `EP-ListSchedules`, `FLD-ListSchedules-offset`, `RSP-ListSchedules-400` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-status-inputValidation-enum` | inputValidation | Gọi endpoint với status thuộc {ACTIVE, PAUSED, COMPLETED, CANCELLED} và với giá trị ngoài tập (ví dụ UNKNOWN). | Bốn giá trị hợp lệ trả HTTP 200; giá trị ngoài tập trả HTTP 400 với errorCode INVALID_REQUEST. | schema: Giá trị trong enum: HTTP 200. Giá trị ngoài enum: HTTP 400, errorCode INVALID_REQUEST. | `EP-ListSchedules`, `FLD-ListSchedules-status`, `RSP-ListSchedules-400` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-ListSchedules-200-functional` | functional | Người gọi là chủ tài khoản, có API key hợp lệ, gọi danh sách lịch của chính tài khoản đó, có thể kèm status, limit, offset hợp lệ. | HTTP 200; chỉ gồm lịch của accountId, đúng status nếu có lọc, đúng cửa sổ phân trang theo limit/offset. | postcondition: HTTP 200; mọi phần tử thuộc accountId và có status=ACTIVE; số phần tử <= 2; bắt đầu từ vị trí offset=1 của tập đã lọc. | `AC-FR-042-2`, `EP-ListSchedules`, `RSP-ListSchedules-200` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-ListSchedules-400-inputValidation` | inputValidation | Gọi endpoint với status không thuộc enum ACTIVE, PAUSED, COMPLETED, CANCELLED. | Trả HTTP 400 với errorCode INVALID_REQUEST; không trả danh sách lịch. | errorContract: HTTP 400, errorCode INVALID_REQUEST. | `EP-ListSchedules`, `RSP-ListSchedules-400` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-ListSchedules-401-access` | access | Gọi endpoint với API key thiếu hoặc sai. | Trả HTTP 401 với errorCode UNAUTHORIZED; không lộ dữ liệu lịch. | errorContract: HTTP 401, errorCode UNAUTHORIZED. | `EP-ListSchedules`, `RSP-ListSchedules-401` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-ListSchedules-403-access` | access | Người gọi có API key hợp lệ nhưng accountId trong path khác tài khoản của người gọi. | Trả HTTP 403 với errorCode FORBIDDEN; không lộ lịch của tài khoản khác. | errorContract: HTTP 403, errorCode FORBIDDEN, body không chứa lịch của B. | `AC-FR-042-4`, `EP-ListSchedules`, `RSP-ListSchedules-403` | llm/approved |

### TOB-SCHEDULED-TRANSFER-EP-ListSchedules-proposal

- Chủ thể: `EP-ListSchedules`; nguồn gốc `llmProposal`; ưu tiên P1
- Câu hỏi: Điều kiện bổ sung do LLM đề xuất cho EP-ListSchedules

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-EP-ListSchedules-idempotency-p6e1377` | idempotency | Gọi lặp lại cùng GET danh sách lịch với cùng tham số khi dữ liệu không đổi. | Mọi lần gọi trả cùng kết quả và không làm thay đổi trạng thái lịch hay lần chạy. | postcondition: Hai phản hồi HTTP 200 có cùng nội dung; trạng thái lịch và lần chạy không đổi. | `EP-ListSchedules` | llm/approved |

### TOB-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-contract

- Chủ thể: `EP-UpdateScheduleStatus`; nguồn gốc `seedRule`; ưu tiên P1
- Câu hỏi: PATCH /v1/scheduled-transfers/{scheduleId} có xác thực API key, kiểm tra scheduleId và status (PAUSED/ACTIVE), phân quyền chủ lịch, kiểm tra trạng thái nguồn rồi trả đúng 200/400/401/403/404/409 với errorCode tương ứng, đồng thời ghi audit không?

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-FLD-Patch-Authorization-inputValidation-missing` | inputValidation | PATCH /v1/scheduled-transfers/{scheduleId} không có header Authorization (Bearer API key) bị từ chối. | Trả HTTP 401 với errorCode UNAUTHORIZED; trạng thái lịch không đổi và không có lần chuyển trạng thái nào được thực hiện. | schema: HTTP 401, errorCode UNAUTHORIZED, trạng thái lịch giữ nguyên. | `EP-UpdateScheduleStatus`, `FLD-Patch-Authorization`, `RSP-Patch-401` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FLD-Patch-scheduleId-inputValidation-missing` | inputValidation | scheduleId (path) là trường bắt buộc; nguồn chỉ nêu 400 cho status, không nêu kết quả khi scheduleId thiếu/rỗng. | Chưa xác định được kết quả mong đợi khi scheduleId thiếu; seed mẫu gán 400 của status nên không đáng tin. | schema: HTTP 400, errorCode INVALID_REQUEST, lịch giữ nguyên trạng thái. | `EP-UpdateScheduleStatus`, `FLD-Patch-scheduleId` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FLD-Patch-status-inputValidation-enum` | inputValidation | status (body) có giá trị ngoài tập {PAUSED, ACTIVE} bị từ chối. | Trả HTTP 400 với errorCode INVALID_REQUEST; trạng thái lịch không đổi. | schema: HTTP 400, errorCode INVALID_REQUEST, lịch giữ nguyên trạng thái. | `EP-UpdateScheduleStatus`, `FLD-Patch-status`, `RSP-Patch-400` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FLD-Patch-status-inputValidation-missing` | inputValidation | Body PATCH không có trường bắt buộc status bị từ chối. | Trả HTTP 400 với errorCode INVALID_REQUEST; trạng thái lịch không đổi. | schema: HTTP 400, errorCode INVALID_REQUEST, lịch giữ nguyên trạng thái. | `EP-UpdateScheduleStatus`, `FLD-Patch-status`, `RSP-Patch-400` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-Patch-200-functional` | functional | Chuyển trạng thái hợp lệ: PAUSED trên lịch định kỳ ACTIVE, hoặc ACTIVE trên lịch PAUSED, bởi chủ lịch. | Trả HTTP 200 kèm lịch có status mới. Với PAUSED, scheduler không chạy lịch này; với ACTIVE, các lần bỏ lỡ lúc tạm dừng có trạng thái SKIPPED, không chạy bù. | postcondition: HTTP 200, body lịch có status = giá trị yêu cầu; sau PAUSED không phát sinh lần chạy mới; sau ACTIVE các lần bỏ lỡ là SKIPPED và không chạy bù. | `AC-FR-044-1`, `AC-FR-044-2`, `EP-UpdateScheduleStatus`, `RSP-Patch-200` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-Patch-400-inputValidation` | inputValidation | PATCH với status thiếu hoặc ngoài PAUSED/ACTIVE trả lỗi 400. | HTTP 400, errorCode INVALID_REQUEST; không đổi trạng thái lịch, không ghi audit chuyển trạng thái thành công. | errorContract: HTTP 400, errorCode INVALID_REQUEST. | `EP-UpdateScheduleStatus`, `RSP-Patch-400` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-Patch-401-access` | access | PATCH với API key thiếu hoặc sai bị từ chối. | HTTP 401, errorCode UNAUTHORIZED; lịch không bị thay đổi. | errorContract: HTTP 401, errorCode UNAUTHORIZED. | `EP-UpdateScheduleStatus`, `RSP-Patch-401` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-Patch-403-access` | access | API key hợp lệ nhưng người gọi không phải chủ lịch. | HTTP 403, errorCode FORBIDDEN; trạng thái lịch không đổi. | errorContract: HTTP 403, errorCode FORBIDDEN, lịch giữ nguyên. | `EP-UpdateScheduleStatus`, `RSP-Patch-403` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-Patch-404-inputValidation` | inputValidation | PATCH với scheduleId không tồn tại. | HTTP 404, errorCode SCHEDULE_NOT_FOUND. | errorContract: HTTP 404, errorCode SCHEDULE_NOT_FOUND. | `EP-UpdateScheduleStatus`, `RSP-Patch-404` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-RSP-Patch-409-inputValidation` | inputValidation | Chuyển trạng thái không hợp lệ: PAUSED trên lịch không ACTIVE, ACTIVE trên lịch không PAUSED, hoặc bất kỳ PATCH trên lịch ONCE. | HTTP 409, errorCode SCHEDULE_STATE_INVALID; trạng thái lịch không đổi. | errorContract: HTTP 409, errorCode SCHEDULE_STATE_INVALID. | `AC-FR-044-3`, `EP-UpdateScheduleStatus`, `RSP-Patch-409` | llm/approved |

### TOB-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-proposal

- Chủ thể: `EP-UpdateScheduleStatus`; nguồn gốc `llmProposal`; ưu tiên P1
- Câu hỏi: Điều kiện bổ sung do LLM đề xuất cho EP-UpdateScheduleStatus

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-audit-p664d35` | audit | Tạm dừng/tiếp tục lịch thành công phải ghi audit. | Có bản ghi audit với correlationId, thời điểm, tác nhân là mã tài khoản, loại sự kiện và mã lịch. | postcondition: Tồn tại bản ghi audit đủ các trường correlationId, thời điểm, tác nhân = mã tài khoản, loại sự kiện, mã lịch. | `AC-FR-048-1`, `EP-UpdateScheduleStatus`, `FR-048` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-concurrency-p1e8657` | concurrency | Scheduler đang chiếm lần chạy đến hạn trong khi lịch bị PATCH PAUSED. | Chưa xác định hành vi; không đoán. | humanJudgment: Cần xác nhận: lần chạy đã chiếm có tiếp tục chạy hay bị hủy/SKIPPED. | `EP-UpdateScheduleStatus`, `MOD-Scheduler` | llm/approved |

### TOB-SCHEDULED-TRANSFER-FR-040-func

- Chủ thể: `FR-040`; nguồn gốc `seedRule`; ưu tiên P0; rủi ro `TRISK-SCHEDULED-TRANSFER-FR-040`
- Câu hỏi: Cần chứng minh: POST /v1/scheduled-transfers với lịch ONCE hợp lệ tạo được lịch ACTIVE mà không chuyển tiền ngay, và các yêu cầu sai thời gian, tài khoản đích không tồn tại, sai chủ tài khoản nguồn, vượt 20 lịch ACTIVE/PAUSED bị từ chối đúng mã lỗi HTTP và errorCode.

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-AC-FR-040-1-functional` | functional | Tạo lịch ONCE hợp lệ qua POST /v1/scheduled-transfers với API key hợp lệ, Idempotency-Key, fromAccountId thuộc người gọi, toAccountId tồn tại, firstRunAt trong khoảng 5 phút đến 90 ngày, amount 1..50.000.000. | Trả 201, body có scheduleId và status=ACTIVE; bản ghi lịch được lưu với status ACTIVE. | acceptanceCriterion: HTTP 201; body có scheduleId không rỗng và status=ACTIVE. (đo: Kiểm tra mã trạng thái và trường body của phản hồi, đối chiếu bản ghi trong kho scheduled_transfers.) | `AC-FR-040-1`, `EP-CreateSchedule`, `FR-040`, `RSP-Create-201` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-AC-FR-040-2-functional` | functional | Việc tạo lịch chỉ đăng ký lịch, không thực hiện chuyển tiền tại thời điểm tạo. | Sau khi tạo lịch thành công, số dư của fromAccountId và toAccountId bằng giá trị trước đó và số bản ghi ledger_entries không tăng. | acceptanceCriterion: Số dư hai tài khoản không đổi; số bản ghi ledger_entries sau bằng số trước. (đo: Truy vấn số dư và đếm ledger_entries ngay trước và ngay sau lời gọi, trước khi lần chạy đến hạn.) | `AC-FR-040-2`, `FR-040` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-AC-FR-040-3-functional` | functional | firstRunAt cách thời điểm hiện tại dưới 5 phút hoặc quá 90 ngày bị từ chối; kiểm tra các giá trị biên quanh 5 phút và 90 ngày. | Trả 422 với errorCode SCHEDULE_TIME_INVALID và không tạo bản ghi lịch. | acceptanceCriterion: HTTP 422, errorCode=SCHEDULE_TIME_INVALID cho giá trị ngoài khoảng; không có bản ghi lịch mới. (đo: Kiểm tra mã HTTP, errorCode và số bản ghi scheduled_transfers không đổi.) | `AC-FR-040-3`, `FLD-Create-firstRunAt`, `FR-040`, `RSP-Create-422-time` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-AC-FR-040-4-functional` | functional | toAccountId không tồn tại trong hệ thống thì yêu cầu tạo lịch bị từ chối. | Trả 404 với errorCode ACCOUNT_NOT_FOUND và không tạo bản ghi lịch. | acceptanceCriterion: HTTP 404, errorCode=ACCOUNT_NOT_FOUND; không có bản ghi lịch mới. (đo: Kiểm tra mã HTTP, errorCode và số bản ghi scheduled_transfers không đổi.) | `AC-FR-040-4`, `FR-040`, `RSP-Create-404` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-AC-FR-040-5-functional` | functional | fromAccountId không thuộc chủ của API key trong header Authorization thì bị từ chối. | Trả 403 với errorCode FORBIDDEN và không tạo bản ghi lịch. | acceptanceCriterion: HTTP 403, errorCode=FORBIDDEN; không có bản ghi lịch mới. (đo: Kiểm tra mã HTTP, errorCode và số bản ghi scheduled_transfers không đổi.) | `AC-FR-040-5`, `FR-040`, `RSP-Create-403` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-AC-FR-040-6-functional` | functional | Tài khoản nguồn đã có 20 lịch ACTIVE hoặc PAUSED thì lịch thứ 21 bị từ chối; kiểm tra biên 19→20 (thành công) và 20→21 (bị từ chối). | Lịch thứ 21 trả 422 với errorCode SCHEDULE_LIMIT_REACHED và không tạo bản ghi; lịch thứ 20 vẫn được tạo thành công. | acceptanceCriterion: HTTP 422, errorCode=SCHEDULE_LIMIT_REACHED; số lịch ACTIVE/PAUSED của tài khoản vẫn là 20. (đo: Kiểm tra mã HTTP, errorCode và đếm lịch ACTIVE/PAUSED của tài khoản sau lời gọi.) | `AC-FR-040-6`, `DE-ScheduledTransfer`, `FR-040`, `RSP-Create-422-limit` | llm/approved |

### TOB-SCHEDULED-TRANSFER-FR-040-proposal

- Chủ thể: `FR-040`; nguồn gốc `llmProposal`; ưu tiên P0
- Câu hỏi: Điều kiện bổ sung do LLM đề xuất cho FR-040

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-FR-040-access-p4f809f` | access | Thiếu hoặc sai API key thì yêu cầu tạo lịch bị từ chối. | Trả 401 UNAUTHORIZED và không tạo lịch. | errorContract: HTTP 401, errorCode=UNAUTHORIZED; không có bản ghi lịch mới. (đo: Kiểm tra mã HTTP, errorCode và số bản ghi lịch.) | `EP-CreateSchedule`, `FR-040`, `RSP-Create-401` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FR-040-audit-p7a419a` | audit | Việc tạo lịch thành công phải được ghi audit. | Có bản ghi audit cho thao tác tạo lịch tương ứng scheduleId vừa tạo. | postcondition: Tồn tại bản ghi audit tạo lịch cho scheduleId đã trả. (đo: Truy vấn bảng audit theo scheduleId sau lời gọi.) | `EP-CreateSchedule`, `FR-040`, `FR-048` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FR-040-concurrency-pd7d28d` | concurrency | Hai yêu cầu tạo lịch đồng thời khi tài khoản đã có 19 lịch ACTIVE/PAUSED không được cùng thành công làm vượt 20 lịch. | Tối đa một yêu cầu trả 201; yêu cầu còn lại trả 422 SCHEDULE_LIMIT_REACHED; tổng lịch ACTIVE/PAUSED không quá 20. | invariant: Số lịch ACTIVE/PAUSED của tài khoản luôn ≤ 20. (đo: Đếm lịch ACTIVE/PAUSED sau khi cả hai yêu cầu hoàn tất.) | `AC-FR-040-6`, `DE-ScheduledTransfer`, `FR-040`, `RSP-Create-422-limit` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FR-040-decisionRule-pc6e1a6` | decisionRule | amount lớn hơn 50.000.000 VND hoặc fromAccountId bằng toAccountId thì bị từ chối; kiểm tra biên amount=50.000.000 (chấp nhận) và 50.000.001 (từ chối). | amount vượt hạn mức trả 422 LIMIT_EXCEEDED; hai tài khoản trùng nhau trả 422 SAME_ACCOUNT; không tạo lịch. | errorContract: HTTP 422 với errorCode LIMIT_EXCEEDED hoặc SAME_ACCOUNT tương ứng; không có bản ghi lịch mới. (đo: Kiểm tra mã HTTP, errorCode và số bản ghi lịch.) | `EP-CreateSchedule`, `FLD-Create-amount`, `FR-040`, `RSP-Create-422-amount`, `RSP-Create-422-same` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FR-040-idempotency-p8f0346` | idempotency | Gửi lại cùng Idempotency-Key với cùng nội dung thì trả lịch đã tạo, không tạo lịch mới. | Trả 200 với lịch đã tạo; số lịch của tài khoản không tăng. | errorContract: HTTP 200, cùng scheduleId như lần đầu; chỉ có một bản ghi lịch. (đo: So sánh scheduleId và đếm bản ghi scheduled_transfers.) | `EP-CreateSchedule`, `FR-040`, `RSP-Create-200` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FR-040-idempotency-pccb8db` | idempotency | Cùng Idempotency-Key nhưng nội dung khác thì bị từ chối. | Trả 409 IDEMPOTENCY_CONFLICT và không tạo lịch mới. | errorContract: HTTP 409, errorCode=IDEMPOTENCY_CONFLICT; số lịch không đổi. (đo: Kiểm tra mã HTTP, errorCode và đếm bản ghi lịch.) | `EP-CreateSchedule`, `FR-040`, `RSP-Create-409` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FR-040-inputValidation-p49f012` | inputValidation | Đầu vào sai định dạng hoặc thiếu trường bắt buộc bị từ chối: thiếu Idempotency-Key, amount không nguyên dương, recurrence ngoài enum, maxRuns ngoài 1..60, firstRunAt sai định dạng. | Trả 400 INVALID_REQUEST và không tạo lịch. | errorContract: HTTP 400, errorCode=INVALID_REQUEST cho mỗi trường hợp; không có bản ghi lịch mới. (đo: Kiểm tra mã HTTP, errorCode và số bản ghi lịch cho từng ca.) | `EP-CreateSchedule`, `FLD-Create-amount`, `FLD-Create-maxRuns`, `FLD-Create-recurrence`, `FR-040`, `RSP-Create-400` | llm/approved |

### TOB-SCHEDULED-TRANSFER-FR-041-func

- Chủ thể: `FR-041`; nguồn gốc `seedRule`; ưu tiên P0; rủi ro `TRISK-SCHEDULED-TRANSFER-FR-041`
- Câu hỏi: Chứng minh: (1) POST /v1/scheduled-transfers tạo được lịch DAILY/WEEKLY/MONTHLY khi có endDate hoặc maxRuns (201, ACTIVE); (2) thiếu cả hai trả 400 INVALID_REQUEST; (3) các lần chạy PENDING kế tiếp sinh đúng chu kỳ rồi lịch COMPLETED; (4) MONTHLY ngày 29/30/31 rơi vào ngày cuối tháng khi tháng thiếu ngày.

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-AC-FR-041-1-functional` | functional | Tạo lịch với recurrence DAILY, WEEKLY hoặc MONTHLY, kèm endDate hoặc maxRuns hợp lệ (1..60), các trường khác hợp lệ, firstRunAt cách hiện tại từ 5 phút đến 90 ngày, endDate không quá 366 ngày. | Trả 201, lịch có status=ACTIVE và có mã lịch; lịch được lưu trong kho scheduled_transfers cùng lần chạy đầu PENDING. | acceptanceCriterion: HTTP 201, status=ACTIVE, có mã lịch trong phản hồi; kiểm tra cho từng recurrence và từng cách đặt giới hạn (chỉ endDate, chỉ maxRuns, cả hai). (đo: So sánh mã trạng thái và nội dung phản hồi; đọc lại bản ghi lịch trong CSDL.) | `AC-FR-041-1`, `EP-CreateSchedule`, `FR-041`, `RSP-Create-201` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-AC-FR-041-2-functional` | functional | Tạo lịch định kỳ (DAILY/WEEKLY/MONTHLY) thiếu cả endDate và maxRuns bị từ chối ở bước kiểm tra đầu vào. | Trả 400 với errorCode INVALID_REQUEST; không tạo bản ghi lịch hay lần chạy nào. | acceptanceCriterion: HTTP 400, errorCode=INVALID_REQUEST; số bản ghi scheduled_transfers của tài khoản không đổi. (đo: So sánh mã HTTP và errorCode; đếm bản ghi trước và sau yêu cầu.) | `AC-FR-041-2`, `FR-041`, `RSP-Create-400` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-AC-FR-041-3-functional` | functional | Khi một lần chạy của lịch định kỳ kết thúc, Scheduler sinh lần chạy PENDING kế tiếp đúng chu kỳ (1 ngày, 7 ngày, 1 tháng) cho đến khi đạt maxRuns hoặc vượt endDate, sau đó đặt lịch COMPLETED. | Mỗi lần chạy kết thúc tạo đúng một lần chạy PENDING kế tiếp đúng thời điểm theo chu kỳ; không sinh thêm sau lần cuối; lịch chuyển COMPLETED khi hết số lần hoặc quá endDate. | acceptanceCriterion: Với maxRuns=N: có đúng N lần chạy, thời điểm cách nhau đúng chu kỳ, lịch COMPLETED sau lần thứ N. Với endDate: không có lần chạy nào sau endDate, lịch COMPLETED. (đo: Đọc bảng scheduled_runs và trạng thái scheduled_transfers sau mỗi chu kỳ quét; dùng đồng hồ giả lập.) | `AC-FR-041-3`, `DI-Scheduler`, `FR-041`, `MOD-Scheduler`, `SCN-ScheduledRun` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-AC-FR-041-4-functional` | functional | Lịch MONTHLY có ngày neo 29, 30 hoặc 31 chạy vào ngày cuối tháng ở tháng không đủ ngày; với ngày neo 29 thì chạy 29/02 ở năm nhuận. | Thời điểm dự kiến các lần chạy là ngày cuối tháng khi tháng ngắn hơn ngày neo (28/02 năm thường, 29/02 năm nhuận, 30 cho tháng 30 ngày); tháng đủ ngày thì chạy đúng ngày neo. | acceptanceCriterion: Neo 31: 31/01 → 28/02 (năm thường) hoặc 29/02 (năm nhuận) → 31/03 → 30/04. Neo 30: tháng 2 → ngày cuối tháng. Neo 29: 28/02 năm thường, 29/02 năm nhuận. (đo: So sánh ngày của các lần chạy PENDING sinh ra với bảng ngày kỳ vọng, dùng cả năm nhuận và năm thường.) | `AC-FR-041-4`, `FR-041`, `SCN-ScheduledRun` | llm/approved |

### TOB-SCHEDULED-TRANSFER-FR-041-proposal

- Chủ thể: `FR-041`; nguồn gốc `llmProposal`; ưu tiên P0
- Câu hỏi: Điều kiện bổ sung do LLM đề xuất cho FR-041

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-FR-041-access-pfae77d` | access | Tạo lịch định kỳ với fromAccountId không thuộc người gọi hoặc thiếu/sai API key bị từ chối. | fromAccountId khác tài khoản người gọi: 403 FORBIDDEN. Thiếu hoặc sai API key: 401 UNAUTHORIZED. Không tạo lịch. | errorContract: HTTP 403 FORBIDDEN cho sai chủ tài khoản; HTTP 401 UNAUTHORIZED cho API key thiếu/sai. (đo: So sánh mã HTTP và errorCode; đếm bản ghi lịch.) | `FLD-Create-Authorization`, `FR-041`, `RSP-Create-401`, `RSP-Create-403` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FR-041-audit-p1cd2c4` | audit | Tạo lịch định kỳ thành công phải được ghi audit. | Có bản ghi audit cho thao tác tạo lịch ứng với mã lịch vừa tạo. | postcondition: Tồn tại bản ghi audit tạo lịch liên kết mã lịch; trường tác nhân cụ thể cho thao tác tạo của người dùng chưa nêu trong nguồn. (đo: Truy vấn bảng audit theo mã lịch sau khi nhận 201.) | `FR-041`, `FR-048`, `MOD-Audit` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FR-041-guardViolation-p241776` | guardViolation | endDate trước lần chạy đầu hoặc cách hiện tại quá 366 ngày bị từ chối khi tạo lịch định kỳ. | Trả 422 SCHEDULE_TIME_INVALID và không tạo lịch. | errorContract: HTTP 422, errorCode=SCHEDULE_TIME_INVALID; kiểm tra biên 366 và 367 ngày. (đo: So sánh mã HTTP và errorCode.) | `FLD-Create-endDate`, `FR-041`, `RSP-Create-422-time` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FR-041-guardViolation-pcbab3f` | guardViolation | Tài khoản đã có 20 lịch ACTIVE hoặc PAUSED không tạo thêm được lịch định kỳ. | Trả 422 SCHEDULE_LIMIT_REACHED; số lịch không đổi. | errorContract: HTTP 422, errorCode=SCHEDULE_LIMIT_REACHED; với 19 lịch thì tạo thành công. (đo: So sánh mã HTTP, errorCode và đếm lịch.) | `DE-ScheduledTransfer`, `FR-041`, `RSP-Create-422-limit` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FR-041-idempotency-pd36026` | idempotency | Gửi lại cùng Idempotency-Key với cùng nội dung lịch định kỳ không tạo lịch thứ hai; khác nội dung bị từ chối. | Cùng nội dung: 200 trả lịch đã tạo, chỉ một bản ghi. Khác nội dung: 409 IDEMPOTENCY_CONFLICT. | errorContract: Cùng nội dung: HTTP 200, cùng mã lịch, tổng số lịch không đổi. Khác nội dung: HTTP 409, IDEMPOTENCY_CONFLICT. (đo: So sánh mã HTTP, mã lịch và đếm bản ghi.) | `FLD-Create-IdempotencyKey`, `FR-041`, `RSP-Create-200`, `RSP-Create-409` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FR-041-inputValidation-p31ec2a` | inputValidation | maxRuns ngoài 1..60 hoặc không nguyên bị từ chối khi tạo lịch định kỳ. | Trả 400 INVALID_REQUEST và không tạo lịch. | errorContract: HTTP 400, errorCode=INVALID_REQUEST; kiểm tra biên 0, 1, 60, 61. (đo: So sánh mã HTTP và errorCode.) | `FLD-Create-maxRuns`, `FR-041`, `RSP-Create-400` | llm/approved |

### TOB-SCHEDULED-TRANSFER-FR-042-func

- Chủ thể: `FR-042`; nguồn gốc `seedRule`; ưu tiên P0; rủi ro `TRISK-SCHEDULED-TRANSFER-FR-042`
- Câu hỏi: Phải chứng minh: chủ tài khoản nguồn tra cứu được một lịch (status, nextRunAt), danh sách lịch có lọc status và phân trang limit/offset, và các lần chạy của lịch (mới nhất trước); người không phải chủ nhận 403 FORBIDDEN, mã lịch không tồn tại nhận 404 SCHEDULE_NOT_FOUND.

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-AC-FR-042-1-functional` | functional | GET /v1/scheduled-transfers/{scheduleId} bởi chủ tài khoản nguồn với API key hợp lệ cho lịch đang tồn tại. | Trả 200 kèm trạng thái lịch (status) và lần chạy kế tiếp (nextRunAt). Yêu cầu chỉ đọc, không làm thay đổi lịch hay lần chạy. | acceptanceCriterion: HTTP 200; body có status và nextRunAt khớp giá trị lưu trong ScheduledTransfer. | `AC-FR-042-1`, `EP-GetSchedule`, `FR-042`, `RSP-Get-200` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-AC-FR-042-2-functional` | functional | GET /v1/accounts/{accountId}/scheduled-transfers của chính tài khoản người gọi, có hoặc không có status (ACTIVE/PAUSED/COMPLETED/CANCELLED), limit (1..100), offset (>=0). | Trả 200 danh sách lịch của tài khoản; khi có status chỉ gồm lịch đúng trạng thái đó; limit giới hạn số phần tử tối đa trả về và offset bỏ qua đúng số phần tử đầu. | acceptanceCriterion: HTTP 200; mọi phần tử thuộc accountId và đúng status lọc; số phần tử <= limit; trang kế tiếp không trùng, không sót phần tử so với trang trước. | `AC-FR-042-2`, `EP-ListSchedules`, `FLD-ListSchedules-limit`, `FLD-ListSchedules-offset`, `FLD-ListSchedules-status`, `FR-042`, `RSP-ListSchedules-200` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-AC-FR-042-3-functional` | functional | GET /v1/scheduled-transfers/{scheduleId}/runs bởi chủ lịch cho lịch có nhiều lần chạy. | Trả 200 các lần chạy, mỗi lần có scheduledAt, status, transferId và reasonCode, sắp theo thứ tự mới nhất trước. | acceptanceCriterion: HTTP 200; danh sách giảm dần theo scheduledAt; mỗi phần tử có đủ scheduledAt, status, transferId, reasonCode khớp ScheduledRun. | `AC-FR-042-3`, `DE-ScheduledRun`, `EP-ListRuns`, `FR-042`, `RSP-ListRuns-200` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-AC-FR-042-4-functional` | functional | Gọi các GET tra cứu lịch bởi người không phải chủ tài khoản nguồn, hoặc với scheduleId không tồn tại. | Người không phải chủ nhận 403 FORBIDDEN; scheduleId không có nhận 404 SCHEDULE_NOT_FOUND; không lộ dữ liệu lịch trong body lỗi. | acceptanceCriterion: HTTP 403 mã FORBIDDEN cho người không phải chủ; HTTP 404 mã SCHEDULE_NOT_FOUND cho lịch không tồn tại. | `AC-FR-042-4`, `FR-042`, `RSP-Get-403`, `RSP-Get-404`, `RSP-ListRuns-403`, `RSP-ListRuns-404` | llm/approved |

### TOB-SCHEDULED-TRANSFER-FR-042-proposal

- Chủ thể: `FR-042`; nguồn gốc `llmProposal`; ưu tiên P0
- Câu hỏi: Điều kiện bổ sung do LLM đề xuất cho FR-042

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-FR-042-access-p2aa8d6` | access | Thiếu hoặc sai API key khi gọi ba endpoint GET tra cứu lịch. | Trả 401 UNAUTHORIZED, không trả dữ liệu lịch. | errorContract: HTTP 401 với errorCode UNAUTHORIZED cho cả ba endpoint. | `EP-GetSchedule`, `EP-ListRuns`, `EP-ListSchedules`, `FR-042`, `RSP-Get-401`, `RSP-ListRuns-401`, `RSP-ListSchedules-401` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FR-042-access-p72e502` | access | Danh sách lịch với accountId khác tài khoản của người gọi. | Trả 403 FORBIDDEN, không lộ lịch của tài khoản khác. | errorContract: HTTP 403 với errorCode FORBIDDEN. | `EP-ListSchedules`, `FR-042`, `RSP-ListSchedules-403` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FR-042-inputValidation-p433493` | inputValidation | Giá trị biên của limit (1..100) và offset (>=0) ở danh sách lịch và danh sách runs. | Giá trị trong biên được chấp nhận; giá trị ngoài biên (limit=0, limit=101, offset=-1) bị xử lý theo hợp đồng lỗi. | schema: limit=1, 100 và offset=0 trả 200; giá trị ngoài biên bị từ chối (mã lỗi cụ thể xem gaps). | `FLD-ListRuns-limit`, `FLD-ListRuns-offset`, `FLD-ListSchedules-limit`, `FLD-ListSchedules-offset`, `FR-042` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FR-042-inputValidation-p473406` | inputValidation | Danh sách lịch với status ngoài enum ACTIVE/PAUSED/COMPLETED/CANCELLED. | Trả 400 INVALID_REQUEST. | errorContract: HTTP 400 với errorCode INVALID_REQUEST. | `EP-ListSchedules`, `FLD-ListSchedules-status`, `FR-042`, `RSP-ListSchedules-400` | llm/approved |

### TOB-SCHEDULED-TRANSFER-FR-043-func

- Chủ thể: `FR-043`; nguồn gốc `seedRule`; ưu tiên P0; rủi ro `TRISK-SCHEDULED-TRANSFER-FR-043`
- Câu hỏi: Chứng minh DELETE /v1/scheduled-transfers/{scheduleId} hủy đúng lịch ACTIVE (lịch và các lần PENDING thành CANCELLED), idempotent với lịch đã CANCELLED (200, kết quả cũ), và từ chối bằng 409 SCHEDULE_NOT_CANCELLABLE khi lịch COMPLETED hoặc có lần chạy RUNNING.

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-AC-FR-043-1-functional` | functional | Chủ lịch gọi DELETE /v1/scheduled-transfers/{scheduleId} với API key hợp lệ cho lịch đang ACTIVE, các lần chạy của lịch ở trạng thái PENDING. | Trả 200. Lịch chuyển sang CANCELLED; mọi lần chạy PENDING của lịch chuyển sang CANCELLED; lần chạy đã ở trạng thái cuối khác không bị thay đổi. | acceptanceCriterion: HTTP 200; ScheduledTransfer.status = CANCELLED; mọi ScheduledRun PENDING của lịch có status = CANCELLED. (đo: Đọc lại lịch và các lần chạy từ CSDL sau phản hồi, so sánh trạng thái trước/sau.) | `AC-FR-043-1`, `EP-CancelSchedule`, `FR-043`, `RSP-Cancel-200` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-AC-FR-043-2-functional` | functional | Chủ lịch gọi lại DELETE cho lịch đã ở trạng thái CANCELLED (gọi lặp). | Trả 200 với kết quả như lần hủy trước; không đổi trạng thái lịch hay lần chạy, không phát sinh thay đổi dữ liệu mới. | acceptanceCriterion: HTTP 200, nội dung phản hồi tương đương kết quả hủy ban đầu; trạng thái lịch vẫn CANCELLED. (đo: So sánh phản hồi lần 1 và lần 2 và trạng thái trong CSDL trước/sau lần gọi lặp.) | `AC-FR-043-2`, `EP-CancelSchedule`, `FR-043`, `RSP-Cancel-200` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-AC-FR-043-3-functional` | functional | Gọi DELETE khi lịch đã COMPLETED, hoặc lịch có ít nhất một lần chạy đang RUNNING. | Trả 409 với errorCode SCHEDULE_NOT_CANCELLABLE; trạng thái lịch và các lần chạy không đổi. | acceptanceCriterion: HTTP 409, errorCode = SCHEDULE_NOT_CANCELLABLE; lịch và lần chạy giữ nguyên trạng thái trước khi gọi. (đo: Kiểm tra mã HTTP, errorCode và đọc lại trạng thái trong CSDL cho cả hai trường hợp A và B.) | `AC-FR-043-3`, `FR-043`, `RSP-Cancel-409` | llm/approved |

### TOB-SCHEDULED-TRANSFER-FR-043-proposal

- Chủ thể: `FR-043`; nguồn gốc `llmProposal`; ưu tiên P0
- Câu hỏi: Điều kiện bổ sung do LLM đề xuất cho FR-043

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-FR-043-access-p4bf0be` | access | DELETE thiếu hoặc sai API key phải bị từ chối. | Trả 401 UNAUTHORIZED; lịch không bị thay đổi. | errorContract: HTTP 401, errorCode UNAUTHORIZED; trạng thái lịch không đổi. (đo: Kiểm tra phản hồi và đọc lại trạng thái lịch.) | `EP-CancelSchedule`, `FR-043`, `RSP-Cancel-401` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FR-043-access-pb30d3e` | access | Người gọi không phải chủ lịch cố hủy lịch của người khác. | Trả 403 FORBIDDEN; lịch và các lần chạy không đổi. | errorContract: HTTP 403, errorCode FORBIDDEN; trạng thái lịch không đổi. (đo: Kiểm tra phản hồi và đọc lại trạng thái lịch.) | `EP-CancelSchedule`, `FR-043`, `RSP-Cancel-403` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FR-043-audit-p1438e9` | audit | Hủy lịch thành công phải được ghi audit. | Có bản ghi audit cho thao tác hủy lịch vừa thực hiện. | postcondition: Tồn tại bản ghi audit cho thao tác hủy lịch gắn với scheduleId. (đo: Truy vấn bảng audit sau khi hủy thành công.) | `EP-CancelSchedule`, `FR-043`, `FR-048`, `MOD-Audit` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FR-043-concurrency-p4b4873` | concurrency | Scheduler chiếm lần chạy PENDING để chạy cùng lúc DELETE hủy lịch. | Kết quả nhất quán: hoặc lần chạy đã được chiếm (RUNNING) và DELETE trả 409, hoặc lần chạy bị hủy và không được thực thi; không có lần chạy vừa CANCELLED vừa tạo giao dịch. | invariant: Không tồn tại lần chạy CANCELLED có giao dịch chuyển tiền được ghi sổ; DELETE trả 200 hoặc 409 tương ứng với kết quả chiếm. (đo: Lặp nhiều lần tranh chấp, đối chiếu trạng thái lần chạy với sổ giao dịch.) | `FR-043`, `MOD-Scheduler`, `RISK-005`, `SCN-ScheduledRun` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FR-043-errorHandling-pa62ef8` | errorHandling | DELETE với scheduleId không tồn tại. | Trả 404 SCHEDULE_NOT_FOUND; không có thay đổi dữ liệu. | errorContract: HTTP 404, errorCode SCHEDULE_NOT_FOUND. (đo: Kiểm tra mã HTTP và errorCode.) | `FLD-Cancel-scheduleId`, `FR-043`, `RSP-Cancel-404` | llm/approved |

### TOB-SCHEDULED-TRANSFER-FR-044-func

- Chủ thể: `FR-044`; nguồn gốc `seedRule`; ưu tiên P1; rủi ro `TRISK-SCHEDULED-TRANSFER-FR-044`
- Câu hỏi: Phải chứng minh: PATCH status=PAUSED trên lịch định kỳ ACTIVE làm lịch PAUSED và scheduler không chạy; PATCH status=ACTIVE trên lịch PAUSED đưa về ACTIVE, các lần bỏ lỡ là SKIPPED, không chạy bù; sai trạng thái nguồn hoặc lịch ONCE trả 409 SCHEDULE_STATE_INVALID.

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-AC-FR-044-1-functional` | functional | Khi chủ lịch gọi PATCH /v1/scheduled-transfers/{scheduleId} với status=PAUSED trên lịch định kỳ đang ACTIVE, lịch chuyển sang PAUSED và scheduler không chạy lịch này. | Trả 200 kèm lịch có status=PAUSED; ở các chu kỳ quét 30 giây sau đó, scan_and_run không tạo hoặc chiếm lần chạy nào cho lịch này và không gọi lõi chuyển tiền. | acceptanceCriterion: HTTP 200, status=PAUSED trong phản hồi và trong bản ghi lịch; không phát sinh giao dịch chuyển tiền mới cho lịch khi qua thời điểm dự kiến. (đo: Đọc lại lịch và đếm giao dịch/lần chạy của lịch sau ít nhất 2 chu kỳ quét (60 giây) vượt qua thời điểm dự kiến.) | `AC-FR-044-1`, `EP-UpdateScheduleStatus`, `FR-044`, `MOD-Scheduler`, `RSP-Patch-200` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-AC-FR-044-2-functional` | functional | Khi chủ lịch gọi PATCH với status=ACTIVE trên lịch đang PAUSED, lịch chuyển sang ACTIVE; các lần đã bỏ lỡ trong lúc tạm dừng có trạng thái SKIPPED và không bị chạy bù. | Trả 200 kèm lịch status=ACTIVE; các lần chạy có thời điểm dự kiến nằm trong khoảng tạm dừng được đánh dấu SKIPPED, không sinh giao dịch; chỉ các lần sau thời điểm tiếp tục mới chạy bình thường. | acceptanceCriterion: HTTP 200, status=ACTIVE; mọi lần bỏ lỡ trong lúc tạm dừng có status SKIPPED và không có giao dịch tương ứng; chạy bù (FR-049) không áp dụng cho các lần này. (đo: Đọc scheduled_runs của lịch và giao dịch liên quan sau ít nhất 2 chu kỳ quét (60 giây) kể từ khi tiếp tục.) | `AC-FR-044-2`, `DE-ScheduledRun`, `FR-044`, `FR-049`, `RSP-Patch-200` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-AC-FR-044-3-functional` | functional | PATCH tạm dừng lịch không ACTIVE, tiếp tục lịch không PAUSED, hoặc PATCH trên lịch ONCE bị từ chối với 409 SCHEDULE_STATE_INVALID. | Trả HTTP 409 với errorCode SCHEDULE_STATE_INVALID; trạng thái lịch và các lần chạy không đổi. | acceptanceCriterion: HTTP 409, errorCode=SCHEDULE_STATE_INVALID; bản ghi lịch giữ nguyên status trước đó. (đo: So sánh bản ghi lịch và scheduled_runs trước và sau lời gọi PATCH.) | `AC-FR-044-3`, `EP-UpdateScheduleStatus`, `FR-044`, `RSP-Patch-409` | llm/approved |

### TOB-SCHEDULED-TRANSFER-FR-044-proposal

- Chủ thể: `FR-044`; nguồn gốc `llmProposal`; ưu tiên P1
- Câu hỏi: Điều kiện bổ sung do LLM đề xuất cho FR-044

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-FR-044-access-p3b5af1` | access | Người gọi không phải chủ lịch bị từ chối 403 FORBIDDEN. | Trả 403 FORBIDDEN; lịch không thay đổi. | errorContract: HTTP 403, errorCode=FORBIDDEN, trạng thái lịch không đổi. | `EP-UpdateScheduleStatus`, `FR-044`, `RSP-Patch-403` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FR-044-access-pf4b5f6` | access | PATCH thiếu hoặc sai API key bị từ chối 401 UNAUTHORIZED. | Trả 401 UNAUTHORIZED; lịch không thay đổi. | errorContract: HTTP 401, errorCode=UNAUTHORIZED, trạng thái lịch không đổi. | `FLD-Patch-Authorization`, `FR-044`, `RSP-Patch-401` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FR-044-audit-pafc81c` | audit | Tạm dừng và tiếp tục lịch thành công phải được ghi audit. | Mỗi PATCH PAUSED/ACTIVE thành công sinh bản ghi audit tương ứng cho thao tác tạm dừng hoặc tiếp tục lịch. | postcondition: Có đúng một bản ghi audit cho mỗi lần chuyển trạng thái thành công, tham chiếu scheduleId. | `FR-044`, `FR-048`, `MOD-Audit` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FR-044-concurrency-p5afd1f` | concurrency | Lịch đang được scheduler quét và chiếm lần chạy đúng lúc nhận PATCH PAUSED không được sinh giao dịch trùng hoặc mất nhất quán. | Mỗi lần chạy chỉ được chiếm một lần (UPDATE có điều kiện); sau khi PATCH PAUSED thành công, không lần chạy mới nào của lịch được khởi tạo. | invariant: Không có hai giao dịch cho cùng Idempotency-Key (mã lịch + thời điểm dự kiến); lịch cuối cùng ở PAUSED. | `CON-010`, `FR-044`, `MOD-Scheduler`, `RISK-005` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FR-044-errorHandling-pb66379` | errorHandling | PATCH với scheduleId không tồn tại trả 404 SCHEDULE_NOT_FOUND. | Trả 404 SCHEDULE_NOT_FOUND; không tạo hoặc đổi bản ghi nào. | errorContract: HTTP 404, errorCode=SCHEDULE_NOT_FOUND. | `FLD-Patch-scheduleId`, `FR-044`, `RSP-Patch-404` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FR-044-inputValidation-p847ccb` | inputValidation | PATCH với status thiếu hoặc ngoài PAUSED/ACTIVE bị từ chối 400 INVALID_REQUEST. | Trả 400 INVALID_REQUEST; lịch không thay đổi. | errorContract: HTTP 400, errorCode=INVALID_REQUEST, trạng thái lịch không đổi. | `FLD-Patch-status`, `FR-044`, `RSP-Patch-400` | llm/approved |

### TOB-SCHEDULED-TRANSFER-FR-045-func

- Chủ thể: `FR-045`; nguồn gốc `seedRule`; ưu tiên P0; rủi ro `TRISK-SCHEDULED-TRANSFER-FR-045`
- Câu hỏi: Phải chứng minh: khi lần chạy đến hạn, scheduler gọi lõi chuyển tiền dùng chung (execute_transfer) tạo đúng một giao dịch COMPLETED, Idempotency-Key = mã lịch + thời điểm dự kiến, và các kiểm tra (hạn mức, số dư, trạng thái tài khoản, Fraud Check) diễn ra tại lúc chạy.

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-AC-FR-045-1-functional` | functional | Với lần chạy đến hạn của lịch hợp lệ, đủ số dư, hạn mức và Fraud Check chấp thuận, scheduler tạo giao dịch qua lõi chuyển tiền dùng chung. | Có đúng 1 bản ghi Transfer trạng thái COMPLETED cho lần chạy; scheduled_runs chuyển sang SUCCEEDED và lưu transferId của giao dịch đó; không có giao dịch thứ hai. | acceptanceCriterion: Đếm Transfer theo lần chạy = 1 và trạng thái COMPLETED; run.status = SUCCEEDED; run.transferId = id của Transfer đó. (đo: Truy vấn CSDL SQLite sau khi quét xong một chu kỳ; đối chiếu với TC-S04.) | `AC-FR-045-1`, `DI-TransferCore`, `FR-045`, `SCN-ScheduledRun` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-AC-FR-045-2-functional` | functional | Idempotency-Key truyền vào lõi chuyển tiền cho lần chạy đầu được dựng từ mã lịch và thời điểm dự kiến của lần chạy. | Transfer sinh ra mang Idempotency-Key = mã lịch + thời điểm dự kiến; cùng lịch và cùng thời điểm dự kiến luôn cho cùng khóa, nên lần gọi lặp không tạo giao dịch mới. | acceptanceCriterion: Idempotency-Key ghi nhận trên giao dịch bằng đúng chuỗi ghép mã lịch và thời điểm dự kiến; hai lần gọi cùng cặp này trả về cùng một giao dịch. (đo: So khớp khóa lưu trong CSDL với giá trị tính từ mã lịch và thời điểm dự kiến của run.) | `AC-FR-045-2`, `FR-045`, `RISK-005` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-AC-FR-045-3-functional` | functional | Hạn mức ngày, số dư, trạng thái tài khoản và Fraud Check được đánh giá theo trạng thái tại thời điểm chạy, không theo trạng thái lúc tạo lịch. | Nếu điều kiện đổi giữa lúc tạo lịch và lúc chạy, kết quả theo trạng thái lúc chạy: đủ điều kiện lúc chạy thì thành công dù lúc tạo thì không, và ngược lại thì lần chạy không tạo giao dịch COMPLETED. | acceptanceCriterion: Quyết định chấp nhận/từ chối khớp với số dư, hạn mức, trạng thái tài khoản và kết quả Fraud Check tại lúc chạy; lời gọi Fraud Check xảy ra trong lúc chạy. (đo: Chạy từng kịch bản thay đổi một yếu tố giữa lúc tạo và lúc chạy; kiểm tra kết quả run/Transfer và số lời gọi tới mock Fraud Check.) | `AC-FR-045-3`, `FR-045`, `INT-01`, `SCN-ScheduledRun` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-AC-FR-045-4-functional` | functional | Giao dịch sinh từ lịch được cộng vào hạn mức ngày đã dùng của tài khoản nguồn và xuất hiện trong lịch sử giao dịch như giao dịch thường. | Sau lần chạy thành công, tổng đã dùng trong ngày tăng đúng bằng số tiền giao dịch; truy vấn lịch sử giao dịch của tài khoản trả về giao dịch này; giao dịch kế tiếp trong ngày bị kiểm tra hạn mức với phần đã dùng đã gồm giao dịch này. | acceptanceCriterion: Hạn mức đã dùng = X + A; lịch sử giao dịch chứa Transfer của lần chạy với số tiền A. (đo: Đọc hạn mức đã dùng và gọi tra cứu lịch sử giao dịch trước và sau lần chạy.) | `AC-FR-045-4`, `DE-Transfer`, `FR-045`, `MOD-Transfer` | llm/approved |

### TOB-SCHEDULED-TRANSFER-FR-045-proposal

- Chủ thể: `FR-045`; nguồn gốc `llmProposal`; ưu tiên P0
- Câu hỏi: Điều kiện bổ sung do LLM đề xuất cho FR-045

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-FR-045-concurrency-p174b2f` | concurrency | Hai scheduler cùng quét một lần chạy đến hạn: chỉ một bên chiếm được lần chạy bằng UPDATE có điều kiện. | Chỉ tạo 1 giao dịch cho lần chạy, bên còn lại không thực thi; số giao dịch trùng = 0. | threshold: Số giao dịch trùng cho một lần chạy = 0. (đo: Chạy song song 2 scheduler trên cùng CSDL, lặp nhiều lần; đếm Transfer theo lần chạy.) | `FR-045`, `MECH-RunClaim`, `NFR-011`, `RISK-005` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FR-045-observability-pd8cb5a` | observability | Mỗi lần chạy có correlationId riêng đi theo giao dịch, audit và lời gọi Fraud Check. | 100% lần chạy có correlationId duy nhất, xuất hiện nhất quán trên Transfer, bản ghi audit và lời gọi Fraud Check. | threshold: 100% lần chạy có correlationId riêng và khớp ở giao dịch, audit, lời gọi Fraud Check. (đo: Đối chiếu toàn bộ lần chạy trong một đợt kiểm thử với log/bản ghi audit và request tới mock Fraud Check.) | `FR-045`, `INT-01`, `NFR-014`, `SCN-ScheduledRun` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FR-045-timing-pb0682f` | timing | Lần chạy đến hạn phải bắt đầu trong 60 giây kể từ thời điểm dự kiến. | 99% lần chạy bắt đầu trong tối đa 60 giây sau thời điểm dự kiến. | threshold: Độ trễ bắt đầu <= 60 giây cho 99% lần chạy. (đo: Phân vị 99 của (thời điểm bắt đầu − thời điểm dự kiến) trên tập lần chạy của một đợt kiểm thử, ghi theo giờ hệ thống.) | `FR-045`, `NFR-010`, `SCN-ScheduledRun` | llm/approved |

### TOB-SCHEDULED-TRANSFER-FR-046-func

- Chủ thể: `FR-046`; nguồn gốc `seedRule`; ưu tiên P0; rủi ro `TRISK-SCHEDULED-TRANSFER-FR-046`
- Câu hỏi: Chứng minh rằng khi một lần chạy lịch thất bại, scheduler xử lý đúng theo loại lỗi (BR-13): lỗi tạm thời được thử lại đúng số lần và khoảng cách, lỗi cố định làm run FAILED ngay, và lịch định kỳ tự chuyển PAUSED sau 3 lần chạy FAILED liên tiếp.

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-AC-FR-046-1-functional` | functional | Khi lõi chuyển tiền trả FRAUD_CHECK_UNAVAILABLE, scheduler thử lại lần chạy tối đa 3 lần, mỗi lần cách nhau 5 phút; hết 3 lần thử lại vẫn lỗi thì run chuyển FAILED và ghi reasonCode. | Có đúng 3 lần thử lại cách nhau 5 phút; nếu lần thử lại nào thành công thì run COMPLETED và dừng thử. Nếu cả 3 lần đều lỗi, run ở FAILED với reasonCode FRAUD_CHECK_UNAVAILABLE, không có lần thử thứ 4. | acceptanceCriterion: Số lần thử lại = 3, khoảng cách 5 phút; sau lần lỗi cuối run = FAILED, reasonCode = FRAUD_CHECK_UNAVAILABLE. (đo: Đối chiếu bản ghi scheduled_runs (trạng thái, số lần thử, thời điểm thử) với đồng hồ giả lập trong kiểm thử tích hợp.) | `AC-FR-046-1`, `FR-046`, `MECH-ScheduleRetry` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-AC-FR-046-2-functional` | functional | Khi lõi chuyển tiền trả INSUFFICIENT_FUNDS, scheduler thử lại lần chạy tối đa 2 lần, mỗi lần cách nhau 1 giờ; hết lượt thì run chuyển FAILED. | Có đúng 2 lần thử lại cách nhau 1 giờ; nếu một lần thử lại thành công (đủ số dư) thì run COMPLETED. Nếu cả 2 lần đều thiếu số dư, run ở FAILED, không có lần thử thứ 3. | acceptanceCriterion: Số lần thử lại = 2, khoảng cách 1 giờ; sau lần lỗi cuối run = FAILED. (đo: Đối chiếu bản ghi scheduled_runs (trạng thái, số lần thử, thời điểm thử) với đồng hồ giả lập trong kiểm thử tích hợp.) | `AC-FR-046-2`, `FR-046`, `MECH-ScheduleRetry` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-AC-FR-046-3-functional` | functional | Khi lõi chuyển tiền trả ACCOUNT_INACTIVE, FRAUD_REJECTED hoặc LIMIT_EXCEEDED, scheduler đặt run FAILED ngay trong lần xử lý đó và không lên lịch thử lại. | Mỗi mã lỗi trong 3 mã trên làm run chuyển FAILED ngay ở lần xử lý đầu tiên, số lần thử lại = 0, không có lần chạy lại nào được lên lịch. | acceptanceCriterion: Run = FAILED ngay với reasonCode tương ứng mã lỗi; số lần thử lại = 0; không có lần thử nào ở các chu kỳ quét sau. (đo: Chạy 3 ca (mỗi mã lỗi một ca), quét thêm ít nhất 1 chu kỳ sau đó và xác nhận không có lần thử mới trong scheduled_runs.) | `AC-FR-046-3`, `FR-046`, `MECH-ScheduleRetry` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-AC-FR-046-4-functional` | functional | Khi lịch định kỳ có 3 lần chạy FAILED liên tiếp, lịch tự chuyển từ ACTIVE sang PAUSED. | Sau lần chạy FAILED thứ 3 liên tiếp, trạng thái lịch là PAUSED và không sinh thêm lần chạy mới. Với 2 lần FAILED liên tiếp hoặc chuỗi bị gián đoạn bởi một run thành công, lịch vẫn ACTIVE. | acceptanceCriterion: Lịch = PAUSED ngay sau lần FAILED thứ 3 liên tiếp; lịch vẫn ACTIVE nếu chuỗi FAILED liên tiếp < 3. (đo: Đọc trạng thái scheduled_transfers sau mỗi lần chạy trong kiểm thử tích hợp, gồm ca chuỗi 2 FAILED, ca 3 FAILED liên tiếp và ca có 1 run thành công xen giữa.) | `AC-FR-046-4`, `FR-046`, `MECH-ScheduleRetry` | llm/approved |

### TOB-SCHEDULED-TRANSFER-FR-047-func

- Chủ thể: `FR-047`; nguồn gốc `seedRule`; ưu tiên P2
- Câu hỏi: Chứng minh rằng mỗi lần chạy lịch có kết quả cuối (thành công hoặc FAILED) đều phát đúng một sự kiện tương ứng (ScheduledRunCompleted kèm transferId, ScheduledRunFailed kèm reasonCode) qua outbox, và SMS kết quả được gửi cho người gửi mà lỗi gửi SMS không làm đổi trạng thái lần chạy.

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-AC-FR-047-1-functional` | functional | Khi một lần chạy lịch chuyển tiền kết thúc thành công, hệ thống phát sự kiện ScheduledRunCompleted chứa transferId của giao dịch vừa tạo, ghi vào bảng outbox. | Có đúng một bản ghi outbox loại ScheduledRunCompleted cho lần chạy, payload có transferId khớp giao dịch đã ghi sổ; bản ghi outbox được commit cùng giao dịch CSDL với lần chạy và được relay lên queue. | acceptanceCriterion: Đúng 1 sự kiện ScheduledRunCompleted trong outbox cho mỗi lần chạy thành công; payload.transferId khác rỗng và bằng transferId của giao dịch. (đo: Truy vấn bảng outbox và bảng giao dịch sau khi lần chạy kết thúc, đối chiếu transferId; chạy trên ENV-Demo với SQLite.) | `AC-FR-047-1`, `FR-047`, `MECH-Outbox`, `SCN-ScheduledRun` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-AC-FR-047-2-functional` | functional | Khi một lần chạy lịch kết thúc ở trạng thái FAILED, hệ thống phát sự kiện ScheduledRunFailed chứa reasonCode của nguyên nhân thất bại, ghi vào bảng outbox. | Có đúng một bản ghi outbox loại ScheduledRunFailed cho lần chạy FAILED, payload có reasonCode khác rỗng khớp nguyên nhân lỗi đã ghi cho lần chạy; không phát ScheduledRunCompleted cho lần chạy đó. | acceptanceCriterion: Đúng 1 sự kiện ScheduledRunFailed trong outbox cho mỗi lần chạy FAILED; payload.reasonCode khác rỗng và bằng mã lý do lưu trên lần chạy. (đo: Truy vấn bảng outbox và bản ghi lần chạy sau khi chuyển FAILED; chạy trên ENV-Demo với SQLite.) | `AC-FR-047-2`, `FR-046`, `FR-047`, `MECH-Outbox` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-AC-FR-047-3-functional` | functional | SMS kết quả lần chạy được gửi cho người gửi qua SMS Gateway sau khi consumer nhận sự kiện từ queue; nếu gửi SMS lỗi thì trạng thái lần chạy giữ nguyên. | Với sự kiện ScheduledRunCompleted hoặc ScheduledRunFailed, có một bản ghi SMS gửi tới người gửi (sms_log trong demo); khi SMS Gateway lỗi, trạng thái lần chạy (thành công hoặc FAILED) không đổi và không có giao dịch bù nào được tạo. | acceptanceCriterion: Trường hợp bình thường: có bản ghi SMS cho người gửi với nội dung từ mẫu kết quả lần chạy. Trường hợp lỗi SMS: trạng thái lần chạy trước và sau khi gửi lỗi giống nhau. (đo: So sánh trạng thái lần chạy trước và sau khi tiêm lỗi SMS Gateway; kiểm tra bảng sms_log; đo thời gian trường hợp xấu nhất từ lúc ghi outbox đến lúc ghi sms_log trên ENV-Demo.) | `AC-FR-047-3`, `DI-SchedNotify`, `FR-047`, `INT-03`, `MOD-Notify`, `SCN-Notify` | llm/approved |

### TOB-SCHEDULED-TRANSFER-FR-047-proposal

- Chủ thể: `FR-047`; nguồn gốc `llmProposal`; ưu tiên P2
- Câu hỏi: Điều kiện bổ sung do LLM đề xuất cho FR-047

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-FR-047-idempotency-pd600b3` | idempotency | Consumer khử trùng sự kiện: cùng một sự kiện kết quả lần chạy được giao lại từ queue không được tạo SMS thứ hai cho người gửi. | Sau khi sự kiện bị giao lại, số bản ghi SMS cho lần chạy đó vẫn là 1. | postcondition: Số bản ghi sms_log cho một lần chạy và một loại sự kiện bằng 1 sau khi giao lại. (đo: Đếm bản ghi sms_log sau khi giao lại sự kiện trên ENV-Demo.) | `FR-047`, `INT-02`, `INT-03`, `SCN-Notify` | llm/approved |

### TOB-SCHEDULED-TRANSFER-FR-048-func

- Chủ thể: `FR-048`; nguồn gốc `seedRule`; ưu tiên P1
- Câu hỏi: Phải chứng minh: mỗi thao tác tạo, hủy, tạm dừng, tiếp tục lịch thành công đều tạo đúng một bản ghi AuditLog (correlationId, thời điểm, tác nhân là mã tài khoản, loại sự kiện, mã lịch), và mọi kết quả lần chạy đều có bản ghi audit với tác nhân SCHEDULER và mã lịch.

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-AC-FR-048-1-functional` | functional | Khi tạo (POST), hủy (DELETE), tạm dừng hoặc tiếp tục (PATCH status=PAUSED/ACTIVE) lịch thành công, hệ thống ghi một bản ghi audit vào AuditLog cho mỗi thao tác. | Mỗi thao tác thành công sinh bản ghi audit có đủ 5 thuộc tính: correlationId không rỗng, thời điểm, tác nhân = mã tài khoản của người gọi, loại sự kiện tương ứng (tạo/hủy/tạm dừng/tiếp tục), mã lịch đúng với lịch bị tác động. | acceptanceCriterion: Sau 4 thao tác thành công, AuditLog có đúng 4 bản ghi mới cho mã lịch đó, mỗi bản ghi đủ correlationId, thời điểm, tác nhân = mã tài khoản (không phải SCHEDULER), loại sự kiện phân biệt, mã lịch. (đo: Truy vấn AuditLog theo mã lịch sau mỗi thao tác trong SQLite và so khớp từng trường.) | `AC-FR-048-1`, `DE-AuditLog`, `DI-SchedAudit`, `EP-CancelSchedule`, `EP-CreateSchedule`, `EP-UpdateScheduleStatus`, `FR-048` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-AC-FR-048-2-functional` | functional | Mỗi lần chạy lịch đến hạn do scheduler xử lý, với mọi kết quả (thành công, thất bại, FAILED, MISSED...), đều ghi một bản ghi audit. | Mỗi kết quả lần chạy có bản ghi audit với tác nhân = SCHEDULER (không phải mã tài khoản) và mã lịch đúng; bản ghi được ghi ở bước cuối 'audit + sự kiện' của SCN-ScheduledRun. | acceptanceCriterion: Với mỗi lần chạy đã có kết quả, AuditLog có bản ghi actor = SCHEDULER, mã lịch = lịch của lần chạy; số bản ghi khớp số lần chạy đã xử lý. (đo: Truy vấn AuditLog theo mã lịch sau khi lần chạy chuyển sang trạng thái cuối; đối chiếu với trạng thái lần chạy (kịch bản TC-S07 mức integration).) | `AC-FR-048-2`, `DE-AuditLog`, `DI-SchedAudit`, `FR-048`, `SCN-ScheduledRun`, `TC-S07` | llm/approved |

### TOB-SCHEDULED-TRANSFER-FR-048-proposal

- Chủ thể: `FR-048`; nguồn gốc `llmProposal`; ưu tiên P1
- Câu hỏi: Điều kiện bổ sung do LLM đề xuất cho FR-048

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-FR-048-guardViolation-pf87b1a` | guardViolation | Yêu cầu tạo, hủy, tạm dừng hoặc tiếp tục bị từ chối (401, 403, 404, 409, 400, 422) thì không ghi audit sự kiện lịch thành công. | Không có bản ghi audit loại tạo/hủy/tạm dừng/tiếp tục mới cho yêu cầu bị từ chối; trạng thái lịch không đổi. | errorContract: Phản hồi 403 FORBIDDEN và 409 SCHEDULE_STATE_INVALID; số bản ghi audit sự kiện lịch không tăng. (đo: Đếm AuditLog theo mã lịch trước và sau.) | `FR-048`, `RSP-Patch-403`, `RSP-Patch-409` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FR-048-invariant-pdabb36` | invariant | AuditLog chỉ thêm, không sửa/xóa: sau khi ghi audit cho thao tác lịch hoặc lần chạy, bản ghi không bị thay đổi hay xóa. | Bản ghi audit đã ghi giữ nguyên nội dung; mọi thao tác UPDATE/DELETE lên AuditLog bị từ chối hoặc không có đường thực hiện. | invariant: Các bản ghi audit cũ không đổi (mọi trường) sau các thao tác tiếp theo; số bản ghi chỉ tăng. (đo: So băm nội dung các bản ghi cũ trước và sau.) | `DE-AuditLog`, `FR-048` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-FR-048-observability-p477475` | observability | correlationId riêng của mỗi lần chạy phải xuất hiện nhất quán trong bản ghi audit, giao dịch và lời gọi Fraud Check. | 100% lần chạy có correlationId riêng; bản ghi audit của lần chạy mang đúng correlationId đó, khác nhau giữa các lần chạy. | threshold: Tỉ lệ lần chạy có correlationId riêng trong audit khớp với giao dịch và lời gọi Fraud Check là 100%. (đo: Đếm trên toàn bộ lần chạy trong một lượt kiểm thử, đối chiếu AuditLog, giao dịch và log Fraud Check.) | `FR-048`, `NFR-014`, `SCN-ScheduledRun` | llm/approved |

### TOB-SCHEDULED-TRANSFER-FR-049-func

- Chủ thể: `FR-049`; nguồn gốc `seedRule`; ưu tiên P1; rủi ro `TRISK-SCHEDULED-TRANSFER-FR-049`
- Câu hỏi: Cần chứng minh: sau khi hệ thống ngừng rồi chạy lại, mỗi lần chạy quá hạn không quá 24 giờ được chạy bù đúng một lần (không trùng giao dịch), còn lần quá hạn quá 24 giờ chuyển MISSED, phát ScheduledRunFailed reasonCode MISSED và không tạo giao dịch.

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-AC-FR-049-1-functional` | functional | Khi Scheduler khởi động lại sau thời gian ngừng, lần chạy có thời điểm dự kiến quá hạn không quá 24 giờ được chạy bù đúng một lần qua lõi chuyển tiền. | Lần chạy quá hạn <= 24 giờ được chiếm và thực thi đúng một lần: tạo đúng 1 giao dịch với Idempotency-Key cố định theo mã lịch + thời điểm dự kiến, lần chạy cập nhật kết quả; quét lại không sinh giao dịch thứ hai. | acceptanceCriterion: Đúng 1 giao dịch cho lần chạy bù; lần chạy không còn ở trạng thái chờ; quét thêm không tạo giao dịch mới. (đo: Đếm giao dịch theo Idempotency-Key của lần chạy sau tối thiểu 6 chu kỳ quét; đếm chu kỳ quét tính từ lúc khởi động lại.) | `AC-FR-049-1`, `FR-049`, `MECH-CatchUp`, `NFR-011`, `NFR-012` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-AC-FR-049-2-functional` | functional | Khi Scheduler quét thấy lần chạy quá hạn quá 24 giờ so với thời điểm dự kiến, lần chạy không được chạy bù mà chuyển MISSED. | Lần chạy chuyển trạng thái MISSED, phát sự kiện ScheduledRunFailed với reasonCode MISSED qua outbox và không tạo giao dịch nào, số dư không đổi. | acceptanceCriterion: Trạng thái lần chạy = MISSED; có 1 sự kiện ScheduledRunFailed reasonCode MISSED; 0 giao dịch cho lần chạy này; số dư tài khoản nguồn và đích không đổi. (đo: Truy vấn scheduled_runs, bảng giao dịch theo Idempotency-Key của lần chạy, outbox và số dư sau chu kỳ quét.) | `AC-FR-049-2`, `FR-049`, `MECH-CatchUp` | llm/approved |

### TOB-SCHEDULED-TRANSFER-FR-049-proposal

- Chủ thể: `FR-049`; nguồn gốc `llmProposal`; ưu tiên P1
- Câu hỏi: Điều kiện bổ sung do LLM đề xuất cho FR-049

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-FR-049-concurrency-pe3c163` | concurrency | Khi hai Scheduler cùng quét lại sau thời gian ngừng, một lần chạy quá hạn trong 24 giờ chỉ được chiếm bởi một Scheduler. | Chỉ một bên chiếm thành công bằng UPDATE có điều kiện; không có giao dịch trùng cho lần chạy bù. | threshold: 0 giao dịch trùng; đúng 1 giao dịch cho lần chạy. (đo: Đếm giao dịch theo Idempotency-Key sau nhiều vòng quét đồng thời lặp lại (mỗi vòng cả hai Scheduler quét cùng lúc).) | `FR-049`, `NFR-011`, `RISK-005` | llm/approved |

### TOB-SCHEDULED-TRANSFER-MECH-CatchUp-mechanism

- Chủ thể: `MECH-CatchUp`; nguồn gốc `seedRule`; ưu tiên P1
- Câu hỏi: Sau khi hệ thống ngừng rồi khởi động lại, cơ chế chạy bù có chạy đúng một lần mỗi lần chạy quá hạn không quá 24 giờ, chuyển MISSED (kèm ScheduledRunFailed reasonCode MISSED, không giao dịch) cho lần quá hạn quá 24 giờ, và xử lý xong trong 5 chu kỳ quét đầu không?

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-MECH-CatchUp-resilience-holds` | resilience | Sau khi hệ thống ngừng và khởi động lại, Scheduler chạy bù mỗi lần chạy quá hạn không quá 24 giờ đúng một lần; lần quá hạn quá 24 giờ chuyển MISSED, không chạy bù. | Lần quá hạn ≤ 24 giờ được thực thi đúng một lần trong 5 chu kỳ quét đầu sau khởi động lại. Lần quá hạn > 24 giờ chuyển MISSED, phát ScheduledRunFailed với reasonCode MISSED và không tạo giao dịch. | postcondition: Lần quá hạn ≤ 24 giờ: chạy đúng 1 lần, xử lý xong trong 5 chu kỳ quét đầu. Lần quá hạn > 24 giờ: trạng thái MISSED, sự kiện ScheduledRunFailed reasonCode=MISSED, 0 giao dịch. (đo: Đếm số chu kỳ quét kể từ lần quét đầu sau khởi động lại cho đến khi lần chạy quá hạn ≤ 24 giờ được xử lý; trường hợp xấu nhất, giới hạn 20 lần chạy mỗi chu kỳ.) | `AC-FR-049-1`, `AC-FR-049-2`, `AC-NFR-012-1`, `FR-049`, `MECH-CatchUp`, `NFR-012` | llm/approved |

### TOB-SCHEDULED-TRANSFER-MECH-CatchUp-proposal

- Chủ thể: `MECH-CatchUp`; nguồn gốc `llmProposal`; ưu tiên P1
- Câu hỏi: Điều kiện bổ sung do LLM đề xuất cho MECH-CatchUp

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-MECH-CatchUp-idempotency-p602a3c` | idempotency | Lần chạy quá hạn ≤ 24 giờ chỉ được chạy bù đúng một lần, kể cả khi khởi động lại nhiều lần hoặc có nhiều chu kỳ quét liên tiếp. | Mỗi lần chạy quá hạn ≤ 24 giờ tạo tối đa một giao dịch; các lần quét sau không chiếm hay chạy lại lần đã xử lý. | postcondition: Số giao dịch của mỗi lần chạy bù = 1; không có bản ghi chạy trùng. | `AC-FR-049-1`, `FR-049`, `MECH-CatchUp`, `MOD-Scheduler` | llm/approved |

### TOB-SCHEDULED-TRANSFER-MECH-RunClaim-mechanism

- Chủ thể: `MECH-RunClaim`; nguồn gốc `seedRule`; ưu tiên P0; rủi ro `TRISK-SCHEDULED-TRANSFER-MECH-RunClaim`
- Câu hỏi: Chứng minh rằng việc chiếm lần chạy bằng UPDATE có điều kiện trên scheduled_runs chỉ cho đúng một scheduler chiếm được mỗi lần chạy (0 giao dịch trùng, NFR-011), và sau khi tiến trình khởi động lại giữa lúc chiếm và lúc chuyển tiền thì không phát sinh giao dịch thứ hai.

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-MECH-RunClaim-concurrency-parallel` | concurrency | Hai scheduler cùng quét và cùng cố chiếm một lần chạy đến hạn bằng UPDATE có điều kiện trên scheduled_runs trong cùng một chu kỳ quét 30 giây. | Đúng một scheduler chiếm được lần chạy và gọi lõi chuyển tiền; bên còn lại không chiếm được và không chuyển tiền. Tổng cộng chỉ có một giao dịch được tạo cho lần chạy đó. | invariant: Số giao dịch tạo ra cho lần chạy = 1; số scheduler chiếm thành công = 1; số giao dịch trùng = 0. (đo: Đếm giao dịch theo Idempotency-Key (mã lịch + thời điểm dự kiến) sau khi cả hai scheduler hoàn tất; kiểm chứng bằng TC-S10 (test_two_schedulers_no_duplicate).) | `AC-NFR-011-1`, `FR-045`, `MECH-RunClaim`, `NFR-011`, `TC-S10` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-MECH-RunClaim-resilience-crash-mid` | resilience | Tiến trình chết sau khi đã chiếm lần chạy (UPDATE có điều kiện đã commit) và trước khi hoàn tất chuyển tiền, rồi khởi động lại. | Sau khởi động lại không tạo giao dịch thứ hai và không mất lần chạy: tổng số giao dịch cho lần chạy tối đa là 1, và lần chạy không bị bỏ sót. Cách phục hồi cụ thể chưa được nguồn mô tả. | invariant: Số giao dịch cho lần chạy ≤ 1 sau khởi động lại; không có giao dịch thứ hai. (đo: Đếm giao dịch theo Idempotency-Key sau khi khởi động lại và chạy vài chu kỳ quét; kiểm chứng bằng TC-S11 (test_restart_between_claim_and_transfer).) | `AC-NFR-011-2`, `FR-045`, `MECH-RunClaim`, `NFR-011`, `TC-S11` | llm/approved |

### TOB-SCHEDULED-TRANSFER-MECH-ScheduleRetry-mechanism

- Chủ thể: `MECH-ScheduleRetry`; nguồn gốc `seedRule`; ưu tiên P1
- Câu hỏi: Chứng minh rằng khi một lần chạy lịch chuyển tiền thất bại, cơ chế thử lại đúng theo loại lỗi BR-13 (số lần, khoảng cách, lỗi không thử lại) và lịch định kỳ tự chuyển PAUSED sau 3 lần chạy FAILED liên tiếp.

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-MECH-ScheduleRetry-resilience-holds` | resilience | Khi lần chạy lịch thất bại, Scheduler xử lý theo loại lỗi BR-13: FRAUD_CHECK_UNAVAILABLE thử lại tối đa 3 lần cách 5 phút; INSUFFICIENT_FUNDS thử lại tối đa 2 lần cách 1 giờ; ACCOUNT_INACTIVE, FRAUD_REJECTED, LIMIT_EXCEEDED FAILED ngay không thử lại. Sau 3 lần chạy FAILED liên tiếp lịch định kỳ chuyển PAUSED. | Số lần thử lại và khoảng cách đúng theo từng loại lỗi; hết lượt thử thì run FAILED kèm reasonCode; lỗi không thử lại thì FAILED ngay; sau 3 lần chạy FAILED liên tiếp lịch chuyển trạng thái PAUSED. | postcondition: FRAUD_CHECK_UNAVAILABLE: tối đa 3 lần thử lại, cách 5 phút, hết lượt run FAILED có reasonCode. INSUFFICIENT_FUNDS: tối đa 2 lần thử lại, cách 1 giờ, hết lượt run FAILED. ACCOUNT_INACTIVE/FRAUD_REJECTED/LIMIT_EXCEEDED: FAILED ngay, 0 lần thử lại. Sau 3 run FAILED liên tiếp lịch = PAUSED. (đo: Kiểm tra số lần gọi lõi chuyển tiền, mốc thời gian mỗi lần thử (đồng hồ giả lập), trạng thái run và lịch trong DB (tích hợp, TC-S05).) | `AC-FR-046-1`, `AC-FR-046-2`, `AC-FR-046-3`, `AC-FR-046-4`, `FR-046`, `MECH-ScheduleRetry` | llm/approved |

### TOB-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-mechanism

- Chủ thể: `MECH-SchedulerThrottle`; nguồn gốc `seedRule`; ưu tiên P0; rủi ro `TRISK-SCHEDULED-TRANSFER-MECH-SchedulerThrottle`
- Câu hỏi: Phải chứng minh mỗi chu kỳ quét (30 giây) scheduler chọn và xử lý tối đa 20 lần chạy đến hạn, rate limit 10 rps được áp dụng, và phần vượt không bị mất mà được hoãn sang chu kỳ sau; đồng thời p95 chuyển tiền qua API vẫn <= 500 ms khi 100 lần chạy đến hạn cùng lúc.

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-performance-at-limit` | performance | Khi đúng 20 lần chạy đến hạn trong một chu kỳ quét, scheduler xử lý đủ cả 20 lần chạy và tốc độ gọi không vượt 10 rps. | Cả 20 lần chạy được chọn và xử lý trong chu kỳ đó; không lần nào bị bỏ sót hay hoãn; tốc độ gọi lõi chuyển tiền không quá 10 rps. | threshold: Số lần chạy được xử lý trong một chu kỳ <= 20; tốc độ yêu cầu <= 10 rps; p95 thời gian chuyển tiền qua API <= 500 ms. (đo: Chạy benchmark 100 lần chạy đến hạn cùng lúc (TC-S13); đếm số lần chạy được xử lý mỗi chu kỳ; đo p95 thời gian chuyển tiền qua API trên toàn bộ lần chạy benchmark.) | `AC-NFR-013-2`, `MECH-SchedulerThrottle`, `NFR-013` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-performance-over-limit` | performance | Khi số lần chạy đến hạn vượt 20 trong một chu kỳ quét (ví dụ 100 lần chạy cùng lúc), scheduler chỉ xử lý tối đa 20 lần chạy trong chu kỳ đó. | Chu kỳ đó xử lý không quá 20 lần chạy; phần còn lại chưa bị chiếm và chưa xử lý trong chu kỳ này. Cách xử lý phần vượt ở các chu kỳ sau (hoãn, thứ tự ưu tiên) chưa được nguồn nêu rõ. | threshold: Số lần chạy được xử lý trong một chu kỳ <= 20; tốc độ yêu cầu <= 10 rps; p95 thời gian chuyển tiền qua API <= 500 ms. (đo: Chạy benchmark 100 lần chạy đến hạn cùng lúc (TC-S13); đếm số lần chạy được xử lý mỗi chu kỳ; đo p95 thời gian chuyển tiền qua API trên toàn bộ lần chạy benchmark.) | `AC-NFR-013-1`, `AC-NFR-013-2`, `MECH-SchedulerThrottle`, `NFR-013`, `TC-S13` | llm/approved |

### TOB-SCHEDULED-TRANSFER-MOD-Scheduler-deps

- Chủ thể: `MOD-Scheduler`; nguồn gốc `seedRule`; ưu tiên P1; rủi ro `TRISK-SCHEDULED-TRANSFER-MOD-Scheduler`
- Câu hỏi: Khi từng phụ thuộc (Quản lý tài khoản, Audit, Risk adapter, Chuyển tiền) lỗi hoặc không phản hồi lúc Scheduler gọi, lần chạy và lịch có kết thúc ở trạng thái, reasonCode, số lần thử lại và hiệu ứng phụ đúng như thiết kế, và không sinh giao dịch trùng không?

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-MOD-Scheduler-errorHandling-dep-MOD-Account` | errorHandling | Quản lý tài khoản lỗi hoặc không phản hồi khi Scheduler gọi (tra cứu tài khoản nguồn/đích lúc tạo lịch hoặc lúc chạy). | Chưa xác định: nguồn không nêu mã lỗi, trạng thái run, việc có thử lại hay không khi Quản lý tài khoản gặp sự cố hạ tầng. Chỉ biết ACCOUNT_INACTIVE làm run FAILED ngay, không thử lại. | errorContract: Chờ làm rõ hành vi; tối thiểu không có giao dịch COMPLETED trùng và run không kẹt ở RUNNING. (đo: Đối chiếu bảng scheduled_runs và ledger_entries sau khi tiêm lỗi.) | `AC-FR-046-3`, `MOD-Account`, `MOD-Scheduler` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-MOD-Scheduler-errorHandling-dep-MOD-Audit` | errorHandling | Audit lỗi hoặc không phản hồi khi Scheduler ghi audit cho thao tác lịch hoặc kết quả lần chạy (tác nhân SCHEDULER). | Chưa xác định: nguồn không nêu việc lỗi ghi audit có làm hỏng thao tác chính (rollback) hay chỉ cảnh báo và tiếp tục. | errorContract: Chờ làm rõ hành vi; tối thiểu trạng thái run và giao dịch phải nhất quán với nhau. (đo: Đối chiếu trạng thái run, ledger_entries và bản ghi audit sau khi tiêm lỗi.) | `FR-048`, `MOD-Audit`, `MOD-Scheduler` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-MOD-Scheduler-errorHandling-dep-MOD-Risk` | errorHandling | Risk adapter lỗi hoặc không phản hồi (Fraud Check timeout, 5xx, 4xx hoặc response sai schema) khi lần chạy đến hạn gọi lõi chuyển tiền. | Áp FB-INT01: số tiền < 5.000.000 VND được cho qua với riskCheck=SKIPPED; từ 5.000.000 VND bị từ chối FRAUD_CHECK_UNAVAILABLE (503). Với FRAUD_CHECK_UNAVAILABLE, Scheduler thử lại tối đa 3 lần cách 5 phút, hết lượt thì run FAILED kèm reasonCode. | errorContract: Số tiền >= 5.000.000 VND: không có giao dịch, retry 3 lần cách 5 phút, sau đó run FAILED với reasonCode FRAUD_CHECK_UNAVAILABLE. Số tiền < 5.000.000 VND: giao dịch COMPLETED với riskCheck=SKIPPED, run SUCCEEDED. (đo: Đồng hồ giả; đếm số lần gọi mock Fraud và số lần thử lại của run.) | `AC-FR-046-1`, `ERR-INT01-4xx`, `ERR-INT01-5xx`, `ERR-INT01-schema`, `ERR-INT01-timeout`, `FB-INT01`, `INT-01`, `MOD-Risk`, `MOD-Scheduler`, `RT-INT01`, `TO-INT01` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-MOD-Scheduler-errorHandling-dep-MOD-Transfer` | errorHandling | Chuyển tiền lỗi hoặc không phản hồi khi Scheduler gọi lõi chuyển tiền cho lần chạy đến hạn. | Lỗi nghiệp vụ đã xác định: INSUFFICIENT_FUNDS thử lại tối đa 2 lần cách 1 giờ rồi run FAILED; ACCOUNT_INACTIVE, FRAUD_REJECTED, LIMIT_EXCEEDED làm run FAILED ngay, không thử lại. Sau 3 lần FAILED liên tiếp, lịch định kỳ chuyển PAUSED. Lỗi hạ tầng hoặc không phản hồi chưa được nguồn nêu. | errorContract: Với lỗi nghiệp vụ: số lần thử lại, khoảng cách và trạng thái run đúng như AC-FR-046-2/3; sau 3 run FAILED liên tiếp lịch là PAUSED; không có giao dịch trùng. (đo: Đồng hồ giả; kiểm tra scheduled_runs, trạng thái lịch và ledger_entries.) | `AC-FR-046-2`, `AC-FR-046-3`, `AC-FR-046-4`, `FR-046`, `MOD-Scheduler`, `MOD-Transfer` | llm/approved |

### TOB-SCHEDULED-TRANSFER-MOD-Scheduler-proposal

- Chủ thể: `MOD-Scheduler`; nguồn gốc `llmProposal`; ưu tiên P1
- Câu hỏi: Điều kiện bổ sung do LLM đề xuất cho MOD-Scheduler

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-MOD-Scheduler-concurrency-p933b8e` | concurrency | Nhiều scheduler cùng quét và chiếm cùng một lần chạy đến hạn. | Chỉ một scheduler chiếm được run (UPDATE có điều kiện); chỉ tạo đúng một giao dịch, Idempotency-Key cố định là mã lịch + thời điểm dự kiến. | invariant: Đúng một giao dịch COMPLETED cho run này; tiến trình còn lại không chiếm được run. (đo: Chạy song song 2 luồng trên cùng CSDL SQLite, đếm bản ghi ledger_entries theo Idempotency-Key.) | `AC-FR-045-2`, `FR-045`, `MOD-Scheduler`, `RISK-005` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-MOD-Scheduler-timing-p077a10` | timing | Một chu kỳ quét có nhiều hơn 20 run đến hạn. | Mỗi chu kỳ chỉ chọn tối đa 20 lần chạy; phần còn lại xử lý ở chu kỳ sau, vẫn đảm bảo 99% run bắt đầu trong 60 giây theo NFR-010. | threshold: Tối đa 20 run được chiếm mỗi chu kỳ; ít nhất 99% run bắt đầu trong 60 giây kể từ thời điểm dự kiến. (đo: Đồng hồ giả, chu kỳ quét 30 giây, đo trên toàn bộ tập run của kịch bản (phân vị 99).) | `AC-NFR-010-1`, `MOD-Scheduler`, `NFR-010`, `RISK-005`, `SCN-ScheduledRun` | llm/approved |

### TOB-SCHEDULED-TRANSFER-NFR-010-nfr

- Chủ thể: `NFR-010`; nguồn gốc `seedRule`; ưu tiên P0; rủi ro `TRISK-SCHEDULED-TRANSFER-NFR-010`
- Câu hỏi: Chứng minh rằng với đồng hồ giả và chu kỳ quét 30 giây, ít nhất 99% lần chạy lịch chuyển tiền bắt đầu trong vòng 60 giây kể từ thời điểm dự kiến (độ trễ bắt đầu = thời điểm chiếm lần chạy trừ thời điểm dự kiến <= 60 s).

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-AC-NFR-010-1-timing` | timing | Với đồng hồ giả và chu kỳ quét 30 giây, tập lần chạy đến hạn phân bố trải đều trong chu kỳ quét; ít nhất 99% lần chạy phải bắt đầu trong 60 giây kể từ thời điểm dự kiến. | Với mỗi lần chạy, độ trễ bắt đầu (thời điểm chiếm lần chạy trừ thời điểm dự kiến) <= 60 s; tỷ lệ lần chạy đạt >= 99% trên toàn bộ mẫu. | acceptanceCriterion: Số lần chạy có độ trễ bắt đầu <= 60 s chia tổng số lần chạy >= 0,99. (đo: Đo trên đồng hồ giả, trên toàn bộ mẫu lần chạy của kịch bản (không dùng thời gian thực); báo cáo tỷ lệ đạt và độ trễ lớn nhất.) | `AC-NFR-010-1`, `AD-05`, `NFR-010`, `SCN-ScheduledRun`, `TC-S09` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-NFR-010-timing-target` | timing | Chỉ tiêu NFR-010: 99% lần chạy lịch chuyển tiền bắt đầu trong 60 giây kể từ thời điểm dự kiến. | Tỷ lệ lần chạy có độ trễ bắt đầu <= 60 s đạt tối thiểu 99% (chỉ tiêu 60 s) trong cửa sổ đo. | threshold: Ít nhất 99% lần chạy có (thời điểm bắt đầu - thời điểm dự kiến) <= 60 s. (đo: Tỷ lệ trên toàn bộ lần chạy trong cửa sổ đo với đồng hồ giả (TC-S09); trường hợp xấu nhất gồm cả lần chạy đến hạn ngay sau một lần quét; mốc thời gian lấy theo thời điểm chiếm lần chạy.) | `AD-05`, `ASM-010`, `NFR-010`, `TC-S09` | llm/approved |

### TOB-SCHEDULED-TRANSFER-NFR-010-proposal

- Chủ thể: `NFR-010`; nguồn gốc `llmProposal`; ưu tiên P0
- Câu hỏi: Điều kiện bổ sung do LLM đề xuất cho NFR-010

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-NFR-010-timing-p9746f5` | timing | Khi có nhiều hơn 20 lần chạy đến hạn trong một chu kỳ, phần vượt giới hạn 20 lần chạy mỗi chu kỳ bị hoãn sang chu kỳ sau; cần xác nhận độ trễ của các lần chạy này vẫn nằm trong 60 s. | Lần chạy bị hoãn một chu kỳ vẫn bắt đầu <= 60 s; số lần chạy hoãn nhiều chu kỳ được đếm vào phần 1% được phép trễ. | threshold: Mỗi chu kỳ chọn tối đa 20 lần chạy; tỷ lệ lần chạy có độ trễ <= 60 s vẫn >= 99% khi quá tải 40 lần chạy đồng thời. (đo: Đồng hồ giả, một lần phát 40 lần chạy đồng thời, đo độ trễ từng lần chạy; trường hợp xấu nhất tính theo lần chạy cuối cùng.) | `MOD-Scheduler`, `NFR-010`, `RISK-005`, `SCN-ScheduledRun` | llm/approved |

### TOB-SCHEDULED-TRANSFER-NFR-011-nfr

- Chủ thể: `NFR-011`; nguồn gốc `seedRule`; ưu tiên P1
- Câu hỏi: Chứng minh rằng với mỗi lần chạy đến hạn, số giao dịch được tạo luôn <= 1 (0 giao dịch trùng), kể cả khi hai scheduler cùng quét và khi khởi động lại giữa lúc chiếm lần chạy và lúc chuyển tiền.

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-AC-NFR-011-1-concurrency` | concurrency | Hai scheduler quét đồng thời cùng một lần chạy đến hạn trong scheduled_runs; cả hai cùng thực hiện UPDATE có điều kiện để chiếm lần chạy. | Chỉ một scheduler chiếm thành công (UPDATE ảnh hưởng 1 dòng) và gọi lõi chuyển tiền; scheduler còn lại bỏ qua lần chạy. Kết quả: đúng 1 giao dịch cho lần chạy đó, 0 giao dịch trùng. | acceptanceCriterion: Đúng 1 giao dịch ứng với Idempotency-Key (mã lịch + thời điểm dự kiến); chỉ 1 scheduler chiếm được lần chạy; không có giao dịch thứ hai. (đo: Đếm số giao dịch theo Idempotency-Key của lần chạy sau khi cả hai scheduler hoàn tất chu kỳ quét (TC-S10).) | `AC-NFR-011-1`, `MECH-RunClaim`, `MOD-Scheduler`, `NFR-011` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-AC-NFR-011-2-concurrency` | concurrency | Scheduler khởi động lại (dừng đột ngột) sau khi đã chiếm lần chạy nhưng trước khi lõi chuyển tiền hoàn tất, rồi quét lại lần chạy đó. | Sau khởi động lại, tổng số giao dịch của lần chạy vẫn <= 1: không tạo giao dịch thứ hai nhờ Idempotency-Key cố định (mã lịch + thời điểm dự kiến); lần chạy được hoàn tất hoặc xử lý theo luồng thử lại/chạy bù. | acceptanceCriterion: Số giao dịch ứng với Idempotency-Key của lần chạy <= 1 sau khi khởi động lại và quét lại; không có giao dịch thứ hai. (đo: Đếm giao dịch theo Idempotency-Key sau khi khởi động lại và chạy hết các chu kỳ quét cần thiết (TC-S11).) | `AC-NFR-011-2`, `FR-045`, `FR-049`, `MECH-RunClaim`, `NFR-011` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-NFR-011-concurrency-target` | concurrency | Với mỗi lần chạy đến hạn, số giao dịch trùng phải bằng 0 trong mọi tình huống nhiều scheduler cùng quét. | Số giao dịch trùng cho một lần chạy = 0 (chỉ tiêu targetValue 0 giao dịch trùng), nhờ chiếm lần chạy bằng UPDATE có điều kiện và Idempotency-Key cố định. | threshold: Số giao dịch trùng cho mỗi lần chạy = 0. (đo: Với mỗi lần chạy trong lần kiểm thử có >= 2 scheduler quét đồng thời, đếm giao dịch theo Idempotency-Key; mọi lần chạy phải có <= 1 giao dịch (TC-S10, TC-S11).) | `MECH-RunClaim`, `NFR-011`, `RISK-005` | llm/approved |

### TOB-SCHEDULED-TRANSFER-NFR-012-nfr

- Chủ thể: `NFR-012`; nguồn gốc `seedRule`; ưu tiên P1
- Câu hỏi: Sau khi Scheduler khởi động lại, mọi lần chạy quá hạn không quá 24 giờ có được xử lý (chạy bù) trong 5 chu kỳ quét đầu (chu kỳ 30 giây, tối đa 20 lần chạy/chu kỳ) không, và lần quá hạn hơn 24 giờ có được đánh dấu MISSED thay vì chạy không?

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-AC-NFR-012-1-resilience` | resilience | Sau khi Scheduler khởi động lại, các lần chạy quá hạn không quá 24 giờ phải được xử lý hết trong 5 chu kỳ quét đầu tiên. | Trong 5 chu kỳ quét đầu sau khởi động lại, mọi lần chạy quá hạn ≤ 24 giờ được chiếm và xử lý (kết quả cuối COMPLETED hoặc FAILED theo FR-046); không còn lần quá hạn ≤ 24 giờ ở trạng thái chờ sau chu kỳ thứ 5. | acceptanceCriterion: Sau chu kỳ quét thứ 5, số lần chạy quá hạn ≤ 24 giờ còn chưa xử lý bằng 0; mỗi chu kỳ xử lý tối đa 20 lần chạy. (đo: Đếm số chu kỳ quét từ lần quét đầu sau khởi động đến khi hết lần chạy quá hạn; tập dữ liệu ≤ 100 lần chạy quá hạn.) | `AC-NFR-012-1`, `MECH-CatchUp`, `MOD-Scheduler`, `NFR-012` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-NFR-012-resilience-target` | resilience | Chỉ tiêu phục hồi NFR-012: sau khởi động lại, lần chạy quá hạn trong 24 giờ được xử lý trong 5 chu kỳ quét đầu. | Với tập lần chạy quá hạn ≤ 24 giờ, thời gian phục hồi không vượt 5 chu kỳ quét (tối đa khoảng 150 giây theo chu kỳ 30 giây); lần quá hạn > 24 giờ chuyển MISSED, không chạy. | threshold: Số chu kỳ quét cần để xử lý hết lần quá hạn ≤ 24 giờ ≤ 5; lần quá hạn > 24 giờ có trạng thái MISSED. (đo: Trường hợp xấu nhất: tải 100 lần chạy quá hạn (5 chu kỳ × 20), đo bằng số chu kỳ quét tính từ lần quét đầu sau khởi động, chu kỳ 30 giây.) | `FR-049`, `MECH-CatchUp`, `NFR-012`, `SCN-ScheduledRun` | llm/approved |

### TOB-SCHEDULED-TRANSFER-NFR-012-proposal

- Chủ thể: `NFR-012`; nguồn gốc `llmProposal`; ưu tiên P1
- Câu hỏi: Điều kiện bổ sung do LLM đề xuất cho NFR-012

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-NFR-012-concurrency-p92271d` | concurrency | Khi chạy bù sau khởi động lại, hai scheduler cùng quét không được tạo giao dịch trùng cho cùng một lần chạy quá hạn. | Mỗi lần chạy quá hạn chỉ được chiếm bởi một scheduler (UPDATE có điều kiện); 0 giao dịch trùng. | threshold: 0 giao dịch trùng cho mỗi lần chạy. (đo: Đếm giao dịch theo Idempotency-Key (mã lịch + thời điểm dự kiến) sau khi xử lý hết tập quá hạn.) | `FR-045`, `MOD-Scheduler`, `NFR-011`, `NFR-012` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-NFR-012-timing-pe0788e` | timing | Biên 24 giờ của chạy bù: lần chạy quá hạn đúng ngưỡng 24 giờ và vượt ngưỡng phải được phân loại đúng. | Lần quá hạn không quá 24 giờ được chạy bù; lần quá hạn hơn 24 giờ chuyển MISSED và không sinh giao dịch. | decisionRow: ≤ 24 giờ: chạy bù; > 24 giờ: MISSED, không giao dịch. | `FR-049`, `MECH-CatchUp`, `NFR-012` | llm/approved |

### TOB-SCHEDULED-TRANSFER-NFR-013-nfr

- Chủ thể: `NFR-013`; nguồn gốc `seedRule`; ưu tiên P1
- Câu hỏi: Phải chứng minh: khi 100 lần chạy lịch chuyển tiền đến hạn cùng lúc, p95 thời gian chuyển tiền qua API không vượt 500 ms, và mỗi chu kỳ quét chỉ xử lý tối đa 20 lần chạy (giới hạn của MECH-SchedulerThrottle).

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-AC-NFR-013-1-performance` | performance | Với 100 lần chạy đến hạn cùng một thời điểm, p95 thời gian chuyển tiền qua API không vượt 500 ms. | Phân vị 95 của thời gian phản hồi lời gọi chuyển tiền qua API trong lô 100 lần chạy là <= 500 ms; không lần chạy nào bị bỏ sót. | acceptanceCriterion: p95 thời gian chuyển tiền qua API <= 500 ms khi 100 lần chạy đến hạn cùng lúc. (đo: Phân vị 95 trên toàn bộ 100 lần chạy của một đợt benchmark (TC-S13), đo thời gian từ lúc gửi yêu cầu chuyển tiền đến khi nhận phản hồi; lặp lại nhiều đợt, lấy đợt xấu nhất, không cộng p95 các thành phần.) | `AC-NFR-013-1`, `NFR-013`, `TC-S13` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-AC-NFR-013-2-performance` | performance | Một chu kỳ quét của scheduler xử lý tối đa 20 lần chạy, kể cả khi có nhiều hơn 20 lần chạy đến hạn. | Số lần chạy được chiếm và xử lý trong mỗi chu kỳ quét <= 20; các lần chạy còn lại được để cho chu kỳ quét kế tiếp. | acceptanceCriterion: Số lần chạy được chọn và chiếm trong một chu kỳ quét <= 20. (đo: Đếm số lần chạy được chiếm (UPDATE có điều kiện thành công) trong từng chu kỳ quét 30 giây của đợt benchmark 100 lần chạy; kiểm tra mọi chu kỳ.) | `AC-NFR-013-2`, `MECH-SchedulerThrottle`, `MOD-Scheduler`, `NFR-013` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-NFR-013-performance-target` | performance | Hệ thống xử lý 100 lần chạy đến hạn cùng lúc mà p95 chuyển tiền qua API vẫn <= 500 ms. | Chỉ tiêu hiệu năng NFR-013 đạt: p95 <= 500 ms với tải 100 lần chạy đến hạn đồng thời, trong khi giới hạn 20 lần chạy mỗi chu kỳ và rate limit 10 rps vẫn được giữ. | threshold: p95 thời gian chuyển tiền qua API <= 500 ms (chỉ tiêu 500 ms). (đo: p95 trên 100 lần chạy trong một đợt benchmark TC-S13, lặp nhiều đợt và lấy đợt xấu nhất; ghi rõ môi trường và tải đo.) | `MECH-SchedulerThrottle`, `NFR-013`, `TC-S13` | llm/approved |

### TOB-SCHEDULED-TRANSFER-NFR-013-proposal

- Chủ thể: `NFR-013`; nguồn gốc `llmProposal`; ưu tiên P1
- Câu hỏi: Điều kiện bổ sung do LLM đề xuất cho NFR-013

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-NFR-013-concurrency-p4dc0d8` | concurrency | Khi 100 lần chạy đến hạn cùng lúc, không có giao dịch trùng cho cùng một lần chạy. | Mỗi lần chạy sinh đúng một giao dịch; số giao dịch trùng bằng 0. | invariant: 0 giao dịch trùng cho một lần chạy. (đo: Đối chiếu số giao dịch với số lần chạy theo Idempotency-Key sau đợt benchmark.) | `NFR-011`, `NFR-013`, `RISK-005` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-NFR-013-performance-pd5ac38` | performance | Rate limit 10 rps của scheduler được giữ khi 100 lần chạy đến hạn cùng lúc, nhằm không đẩy thêm tải quá mức lên Fraud Check giờ cao điểm. | Tốc độ gọi lõi chuyển tiền từ scheduler không vượt 10 yêu cầu mỗi giây. | threshold: Tốc độ yêu cầu từ scheduler <= 10 rps. (đo: Số yêu cầu mỗi giây, đo theo cửa sổ 1 giây trượt trong toàn bộ đợt benchmark 100 lần chạy; lấy cửa sổ lớn nhất.) | `MECH-SchedulerThrottle`, `NFR-013`, `RISK-005` | llm/approved |

### TOB-SCHEDULED-TRANSFER-NFR-014-nfr

- Chủ thể: `NFR-014`; nguồn gốc `seedRule`; ưu tiên P1
- Câu hỏi: Chứng minh 100% lần chạy lịch chuyển tiền có correlationId riêng (không trùng giữa các lần chạy) và correlationId đó xuất hiện trong giao dịch, audit, lời gọi Fraud Check (INT-01) và sự kiện của chính lần chạy ấy.

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-AC-NFR-014-1-observability` | observability | Khi scheduler chiếm và xử lý nhiều lần chạy đến hạn, mỗi lần chạy được sinh một correlationId riêng và không có hai lần chạy nào dùng chung giá trị. | Với N lần chạy trong một lượt kiểm thử (gồm cả các lần chạy trong cùng chu kỳ quét, tối đa 20), tập correlationId có đúng N giá trị phân biệt, không rỗng, không null. | acceptanceCriterion: Số correlationId phân biệt = số lần chạy; mọi lần chạy đều có correlationId khác rỗng. (đo: Truy vấn correlationId của các lần chạy sau khi xử lý xong và đếm DISTINCT so với số lần chạy.) | `AC-NFR-014-1`, `NFR-014`, `SCN-ScheduledRun` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-AC-NFR-014-2-observability` | observability | correlationId sinh cho một lần chạy được truyền xuyên suốt và xuất hiện ở bốn nơi: giao dịch, bản ghi audit, lời gọi Fraud Check (INT-01) và sự kiện của lần chạy đó. | Với mỗi lần chạy, giao dịch, audit (tác nhân SCHEDULER), request gọi Fraud Check và sự kiện (ScheduledRunCompleted/ScheduledRunFailed qua outbox) đều mang cùng đúng một correlationId của lần chạy đó, không lẫn với lần chạy khác. | acceptanceCriterion: Giá trị correlationId trong giao dịch, audit, lời gọi Fraud Check và sự kiện bằng đúng giá trị correlationId của lần chạy. (đo: Đối chiếu correlationId của lần chạy với bản ghi giao dịch, audit, request ghi nhận tại mock Fraud Check và payload sự kiện trong outbox (xem TC-S12).) | `AC-NFR-014-2`, `INT-01`, `NFR-014`, `SCN-ScheduledRun`, `TC-S12` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-NFR-014-observability-target` | observability | Chỉ tiêu truy vết: 100% lần chạy có correlationId riêng và correlationId đó đi theo giao dịch, audit và lời gọi Fraud Check. | Trong tập lần chạy kiểm thử (gồm thành công, thử lại, FAILED), tỷ lệ lần chạy có correlationId riêng và xuất hiện đủ ở giao dịch, audit, lời gọi Fraud Check đạt 100%. | threshold: Tỷ lệ lần chạy truy vết đầy đủ = 100% (targetValue 100, đơn vị %). (đo: Tỷ lệ = số lần chạy có correlationId riêng và xuất hiện đủ ở giao dịch, audit, lời gọi Fraud Check / tổng số lần chạy trong tập kiểm thử; đo trên toàn bộ lần chạy, không lấy mẫu.) | `INT-01`, `NFR-014`, `SCN-ScheduledRun` | llm/approved |

### TOB-SCHEDULED-TRANSFER-NFR-014-proposal

- Chủ thể: `NFR-014`; nguồn gốc `llmProposal`; ưu tiên P1
- Câu hỏi: Điều kiện bổ sung do LLM đề xuất cho NFR-014

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-NFR-014-observability-p8ac829` | observability | Khi lần chạy được thử lại, correlationId phải nhất quán theo quy tắc đã định và vẫn xuất hiện đủ ở giao dịch, audit và lời gọi Fraud Check. | Các lần thử lại của cùng một lần chạy không làm mất correlationId; mọi bản ghi và lời gọi Fraud Check của lần thử lại đều mang correlationId. | invariant: Mọi bản ghi và lời gọi Fraud Check của lần chạy, kể cả khi thử lại, đều có correlationId khác rỗng. (đo: Đối chiếu các bản ghi theo lần chạy sau khi thử lại.) | `NFR-014`, `SCN-ScheduledRun` | llm/approved |

### TOB-SCHEDULED-TRANSFER-SCN-ScheduledRun-flow

- Chủ thể: `MOD-API`, `MOD-Scheduler`, `MOD-Transfer`, `SCN-ScheduledRun`; nguồn gốc `seedRule`; ưu tiên P1
- Câu hỏi: Chứng minh mỗi lần chạy đến hạn đi đúng chuỗi: quét 30 giây, chọn tối đa 20 lần chạy, chiếm lần chạy, sinh correlationId, lõi chuyển tiền, cập nhật lần chạy, thử lại/FAILED, ghi audit (tác nhân SCHEDULER) và phát sự kiện qua outbox, qua MOD-API, MOD-Scheduler, MOD-Transfer.

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-errorHandling-fail-INT-01` | errorHandling | Khi Fraud Check (INT-01) lỗi (timeout, 5xx, 4xx hoặc sai schema) trong lần chạy lịch đến hạn, hệ thống áp dụng dự phòng FB-INT01 theo số tiền của lần chạy. | Số tiền < 5.000.000 VND: giao dịch được cho qua với riskCheck=SKIPPED, run SUCCEEDED. Số tiền ≥ 5.000.000 VND: lõi chuyển tiền từ chối FRAUD_CHECK_UNAVAILABLE (503), run được thử lại tối đa 3 lần cách 5 phút, hết lượt thì run FAILED kèm reasonCode. | errorContract: Dưới 5.000.000 VND: riskCheck=SKIPPED, giao dịch COMPLETED. Từ 5.000.000 VND (kể cả đúng 5.000.000): FRAUD_CHECK_UNAVAILABLE (503), không có giao dịch, thử lại tối đa 3 lần cách 5 phút rồi FAILED với reasonCode. | `AC-FR-046-1`, `ERR-INT01-4xx`, `ERR-INT01-5xx`, `ERR-INT01-schema`, `ERR-INT01-timeout`, `FB-INT01`, `INT-01`, `SCN-ScheduledRun` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-functional-main` | functional | Luồng chính: scheduler quét mỗi 30 giây, chọn tối đa 20 lần chạy đến hạn, chiếm từng lần bằng UPDATE có điều kiện, sinh correlationId riêng, gọi lõi chuyển tiền với Idempotency-Key = mã lịch + thời điểm dự kiến, rồi cập nhật lần chạy. | Mỗi lần chạy đến hạn tạo đúng một giao dịch COMPLETED, run SUCCEEDED có transferId; audit tác nhân SCHEDULER và mã lịch được ghi; sự kiện ScheduledRunCompleted có transferId được ghi vào outbox cùng giao dịch CSDL. | postcondition: 1 giao dịch COMPLETED, run SUCCEEDED có transferId, 1 bản ghi audit tác nhân SCHEDULER, 1 sự kiện ScheduledRunCompleted trong outbox; giao dịch tính vào hạn mức ngày và hiện trong lịch sử. | `AC-FR-045-1`, `AC-FR-045-2`, `AC-FR-047-1`, `AC-FR-048-2`, `MECH-Outbox`, `MECH-RunClaim`, `MOD-API`, `MOD-Scheduler`, `MOD-Transfer`, `SCN-ScheduledRun` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-timing-budget` | timing | Một lần xử lý của luồng Chạy lịch chuyển tiền đến hạn, từ bước chiếm lần chạy đến bước ghi audit/sự kiện, hoàn tất trong ngân sách 60000 ms kể cả trường hợp xấu nhất của lời gọi Fraud Check. | Thời gian xử lý một lần chạy ≤ 60000 ms trong trường hợp xấu nhất. | threshold: ≤ 60000 ms (đo: Trường hợp xấu nhất, đo từ bước đầu tới bước cuối của một lần xử lý; Fraud Check tính 2 lần × 1000 ms; không cộng p95. Khoảng cách thử lại 5 phút giữa các lần xử lý không tính (chờ xác nhận).) | `AD-05`, `NFR-010`, `SCN-ScheduledRun` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-timing-worst-INT-01` | timing | Trường hợp xấu nhất khi gọi Fraud Check: tối đa 2 lần gọi × 1000 ms (totalMs) + backoff 0 ms = 2000 ms, nằm trong ngân sách 60000 ms của kịch bản. | Tổng thời gian gọi Fraud Check trường hợp xấu nhất = 2000 ms ≤ 60000 ms; sau đó áp FB-INT01. | threshold: 2000 ms ≤ 60000 ms (ngân sách của SCN-ScheduledRun) (đo: Cộng trường hợp xấu nhất: timeout mỗi lần (1000 ms) × số lần (2) + backoff (none = 0 ms); không dùng p95.) | `INT-01`, `RT-INT01`, `SCN-ScheduledRun`, `TO-INT01` | llm/approved |

### TOB-SCHEDULED-TRANSFER-SCN-ScheduledRun-proposal

- Chủ thể: `SCN-ScheduledRun`; nguồn gốc `llmProposal`; ưu tiên P1
- Câu hỏi: Điều kiện bổ sung do LLM đề xuất cho SCN-ScheduledRun

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-atomicity-p12901f` | atomicity | Tiến trình khởi động lại giữa lúc chiếm lần chạy và lúc chuyển tiền. | Không tạo giao dịch thứ hai khi lần chạy được xử lý lại; Idempotency-Key = mã lịch + thời điểm dự kiến chặn trùng. | invariant: Tối đa một giao dịch cho lần chạy. | `AC-FR-045-2`, `AC-NFR-011-2`, `NFR-011`, `SCN-ScheduledRun` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-concurrency-p775a37` | concurrency | Hai scheduler cùng quét và cùng chiếm một lần chạy đến hạn. | Chỉ một scheduler chiếm thành công; chỉ tạo một giao dịch, không có giao dịch trùng. | invariant: Số giao dịch trùng cho một lần chạy = 0; đúng một UPDATE có điều kiện thành công. | `AC-NFR-011-1`, `MECH-RunClaim`, `NFR-011`, `SCN-ScheduledRun` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-errorHandling-p00fe49` | errorHandling | Lỗi gửi SMS kết quả của lần chạy. | Trạng thái lần chạy không đổi; sự kiện vẫn nằm trong outbox. | postcondition: Trạng thái run giữ nguyên (SUCCEEDED hoặc FAILED) sau lỗi SMS. | `AC-FR-047-3`, `FR-047`, `MECH-Outbox`, `SCN-ScheduledRun` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-observability-p12d52f` | observability | Mỗi lần chạy có correlationId riêng, khác nhau giữa các lần chạy, và nó xuất hiện trong giao dịch, audit, lời gọi Fraud Check và sự kiện. | 100% lần chạy có correlationId duy nhất, truy vết được qua giao dịch, audit, Fraud Check và sự kiện. | invariant: Không có hai lần chạy chung correlationId; correlationId xuất hiện ở cả 4 nơi. | `AC-NFR-014-1`, `AC-NFR-014-2`, `MECH-Correlation`, `NFR-014`, `SCN-ScheduledRun` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-performance-pa101d9` | performance | Khi 100 lần chạy đến hạn cùng lúc, mỗi chu kỳ quét xử lý tối đa 20 lần chạy và p95 chuyển tiền qua API vẫn không quá 500 ms. | Mỗi chu kỳ ≤ 20 lần chạy; p95 thời gian chuyển tiền ≤ 500 ms. | threshold: p95 ≤ 500 ms; số lần chạy mỗi chu kỳ ≤ 20. (đo: p95 của thời gian chuyển tiền qua API trong toàn bộ đợt 100 lần chạy, đo ở tải 100 lần chạy đến hạn cùng lúc.) | `AC-NFR-013-1`, `AC-NFR-013-2`, `MECH-SchedulerThrottle`, `NFR-013`, `SCN-ScheduledRun` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-stateTransition-pd98b3f` | stateTransition | Lần chạy quá hạn sau khi hệ thống ngừng: chạy bù nếu quá hạn không quá 24 giờ, ngược lại MISSED. | Quá hạn ≤ 24 giờ: chạy đúng một lần, trong 5 chu kỳ quét đầu. Quá hạn > 24 giờ: MISSED, phát ScheduledRunFailed reasonCode MISSED, không có giao dịch. | decisionRow: ≤ 24 giờ: 1 giao dịch, xử lý trong 5 chu kỳ quét đầu; > 24 giờ: MISSED, 0 giao dịch, ScheduledRunFailed reasonCode MISSED. (đo: Đếm số chu kỳ quét 30 giây kể từ khi khởi động; kiểm tra biên đúng 24 giờ theo đồng hồ giả.) | `AC-FR-049-1`, `AC-FR-049-2`, `AC-NFR-012-1`, `FR-049`, `MECH-CatchUp`, `NFR-012`, `SCN-ScheduledRun` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-stateTransition-pfec2a2` | stateTransition | Sau 3 lần chạy FAILED liên tiếp, lịch định kỳ tự chuyển PAUSED; các lỗi ACCOUNT_INACTIVE, FRAUD_REJECTED, LIMIT_EXCEEDED làm run FAILED ngay, không thử lại. | Lịch chuyển PAUSED sau lần FAILED thứ 3 liên tiếp; lỗi không retry được thì run FAILED ngay và phát ScheduledRunFailed kèm reasonCode. | stateMachine: Lịch ACTIVE → PAUSED sau 3 run FAILED liên tiếp; run SUCCEEDED xen giữa thì đếm lại từ đầu (chờ xác nhận). | `AC-FR-046-3`, `AC-FR-046-4`, `AC-FR-047-2`, `FR-046`, `MECH-ScheduleRetry`, `SCN-ScheduledRun` | llm/approved |

### TOB-SCHEDULED-TRANSFER-SM-Schedule-proposal

- Chủ thể: `SM-Schedule`; nguồn gốc `llmProposal`; ưu tiên P1
- Câu hỏi: Điều kiện bổ sung do LLM đề xuất cho SM-Schedule

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-SM-Schedule-concurrency-p34e996` | concurrency | PATCH status=PAUSED hoặc DELETE xảy ra đồng thời với lúc scheduler chiếm lần chạy đến hạn bằng UPDATE có điều kiện. | Mỗi lần chạy chỉ được chiếm một lần; lịch không bị chạy sau khi đã PAUSED hoặc CANCELLED; DELETE chỉ thành công khi không có lần RUNNING. | invariant: Không có lần chạy bị thực hiện 2 lần; không có lần RUNNING của lịch đã CANCELLED; trạng thái cuối nhất quán với một thứ tự tuần tự hợp lệ. | `MOD-Scheduler`, `SM-Schedule` | llm/approved |

### TOB-SCHEDULED-TRANSFER-SM-Schedule-states

- Chủ thể: `SM-Schedule`; nguồn gốc `seedRule`; ưu tiên P1
- Câu hỏi: Chứng minh lịch chuyển tiền (ScheduledTransfer) chỉ chuyển trạng thái theo 7 chuyển tiếp đã định nghĩa (ACTIVE↔PAUSED, ACTIVE/PAUSED→CANCELLED, ACTIVE/PAUSED→COMPLETED). Mỗi chuyển tiếp phải có đúng hiệu ứng (audit, trạng thái các lần chạy, consecutive_failures). Khi guard sai thì trạng thái giữ nguyên.

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-TRN-Schedule-1-guardViolation-guard` | guardViolation | PATCH status=PAUSED trên lịch ACTIVE nhưng không phải lịch định kỳ (guard 'lịch định kỳ' sai) thì bị từ chối. | Yêu cầu bị từ chối, lịch giữ ACTIVE; không ghi audit SCHEDULE_PAUSED; các lần chạy không đổi. | stateMachine: Trạng thái vẫn ACTIVE, không có audit SCHEDULE_PAUSED mới. Mã lỗi chưa xác định trong nguồn. | `SM-Schedule`, `TRN-Schedule-1` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-TRN-Schedule-1-stateTransition` | stateTransition | PATCH status=PAUSED trên lịch định kỳ đang ACTIVE chuyển lịch sang PAUSED. | Trạng thái PAUSED; ghi audit SCHEDULE_PAUSED; scheduler không còn chạy lịch này ở các lần quét sau. | stateMachine: ACTIVE → PAUSED; có đúng 1 audit SCHEDULE_PAUSED; lần quét 30 giây kế tiếp không chiếm lần chạy nào của lịch này. | `SM-Schedule`, `TRN-Schedule-1` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-TRN-Schedule-2-guardViolation-guard` | guardViolation | PATCH status=ACTIVE trên lịch PAUSED khi không còn lần chạy nào theo BR-11 (guard sai) không được chuyển sang ACTIVE theo nhánh TRN-Schedule-2. | Không chuyển sang ACTIVE theo TRN-Schedule-2; không sinh lần chạy kế tiếp và không ghi audit SCHEDULE_RESUMED. Trường hợp này thuộc TRN-Schedule-7 (COMPLETED). | stateMachine: Không có trạng thái ACTIVE, không có audit SCHEDULE_RESUMED, không có lần chạy mới. Kết quả cuối xem TRN-Schedule-7. | `SM-Schedule`, `TRN-Schedule-2`, `TRN-Schedule-7` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-TRN-Schedule-2-stateTransition` | stateTransition | PATCH status=ACTIVE trên lịch PAUSED còn lần chạy theo BR-11 chuyển lịch sang ACTIVE. | Trạng thái ACTIVE; các lần bỏ lỡ trong lúc tạm dừng thành SKIPPED; sinh lần chạy kế tiếp; consecutive_failures về 0; ghi audit SCHEDULE_RESUMED. | stateMachine: PAUSED → ACTIVE; lần bỏ lỡ = SKIPPED; có 1 lần chạy kế tiếp mới; consecutive_failures = 0; audit SCHEDULE_RESUMED. | `SM-Schedule`, `TRN-Schedule-2` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-TRN-Schedule-3-guardViolation-guard` | guardViolation | Lần chạy FAILED thứ 3 liên tiếp của lịch không phải định kỳ (guard sai) thì scheduler không tự tạm dừng. | Lịch giữ ACTIVE; không ghi audit SCHEDULE_AUTO_PAUSED. | stateMachine: Trạng thái vẫn ACTIVE, không có audit SCHEDULE_AUTO_PAUSED. | `SM-Schedule`, `TRN-Schedule-3` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-TRN-Schedule-3-stateTransition` | stateTransition | Lần chạy FAILED thứ 3 liên tiếp của lịch định kỳ ACTIVE khiến scheduler tự chuyển lịch sang PAUSED. | Trạng thái PAUSED; audit SCHEDULE_AUTO_PAUSED với tác nhân SCHEDULER; không sinh lần chạy mới. | stateMachine: ACTIVE → PAUSED; audit SCHEDULE_AUTO_PAUSED có actor=SCHEDULER; không có lần chạy PENDING mới được tạo. | `SM-Schedule`, `TRN-Schedule-3` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-TRN-Schedule-4-guardViolation-guard` | guardViolation | Lần chạy cuối kết thúc nhưng lịch chưa thuộc ONCE đã xong, chưa đạt maxRuns, chưa quá endDate thì không hoàn tất. | Lịch giữ ACTIVE và vẫn sinh/giữ lần chạy kế tiếp theo lịch; không chuyển COMPLETED. | stateMachine: Trạng thái vẫn ACTIVE, không chuyển COMPLETED. | `SM-Schedule`, `TRN-Schedule-4` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-TRN-Schedule-4-stateTransition` | stateTransition | Khi lần chạy cuối kết thúc và lịch ONCE đã xong, hoặc đạt maxRuns, hoặc đạt endDate thì lịch chuyển COMPLETED. | Trạng thái COMPLETED; không sinh lần chạy mới. | stateMachine: ACTIVE → COMPLETED; không có lần chạy mới nào được tạo. | `SM-Schedule`, `TRN-Schedule-4` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-TRN-Schedule-5-guardViolation-guard` | guardViolation | DELETE lịch ACTIVE đang có lần chạy RUNNING (guard sai) bị từ chối. | Lịch giữ ACTIVE; lần PENDING không bị đổi thành CANCELLED; không ghi audit SCHEDULE_CANCELLED. | stateMachine: Trạng thái vẫn ACTIVE, lần PENDING giữ nguyên, không có audit SCHEDULE_CANCELLED. Mã lỗi chưa xác định trong nguồn. | `SM-Schedule`, `TRN-Schedule-5` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-TRN-Schedule-5-stateTransition` | stateTransition | DELETE lịch ACTIVE không có lần chạy RUNNING chuyển lịch sang CANCELLED. | Trạng thái CANCELLED; mọi lần PENDING thành CANCELLED; ghi audit SCHEDULE_CANCELLED. | stateMachine: ACTIVE → CANCELLED; không còn lần PENDING; có 1 audit SCHEDULE_CANCELLED. | `SM-Schedule`, `TRN-Schedule-5` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-TRN-Schedule-6-guardViolation-guard` | guardViolation | DELETE lịch PAUSED đang có lần chạy RUNNING (guard sai) bị từ chối. | Lịch giữ PAUSED; lần PENDING không đổi; không ghi audit SCHEDULE_CANCELLED. | stateMachine: Trạng thái vẫn PAUSED, không có audit SCHEDULE_CANCELLED. Mã lỗi chưa xác định trong nguồn. | `SM-Schedule`, `TRN-Schedule-6` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-TRN-Schedule-6-stateTransition` | stateTransition | DELETE lịch PAUSED không có lần chạy RUNNING chuyển lịch sang CANCELLED. | Trạng thái CANCELLED; mọi lần PENDING thành CANCELLED; ghi audit SCHEDULE_CANCELLED. | stateMachine: PAUSED → CANCELLED; không còn lần PENDING; có 1 audit SCHEDULE_CANCELLED. | `SM-Schedule`, `TRN-Schedule-6` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-TRN-Schedule-7-guardViolation-guard` | guardViolation | PATCH status=ACTIVE trên lịch PAUSED mà sau khi đánh dấu SKIPPED vẫn còn lần chạy theo BR-11 thì không chuyển COMPLETED. | Lịch không sang COMPLETED; xử lý theo TRN-Schedule-2 (sang ACTIVE). Giữ PAUSED chỉ khi yêu cầu bị từ chối. | stateMachine: Không chuyển COMPLETED. Trạng thái cuối do TRN-Schedule-2 quyết định; nguồn mẫu 'giữ PAUSED' mâu thuẫn, cần làm rõ. | `SM-Schedule`, `TRN-Schedule-2`, `TRN-Schedule-7` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-TRN-Schedule-7-stateTransition` | stateTransition | PATCH status=ACTIVE trên lịch PAUSED khi không còn lần chạy nào theo BR-11 sau khi đánh dấu SKIPPED thì lịch chuyển COMPLETED. | Trạng thái COMPLETED; các lần còn lại SKIPPED; không sinh lần mới. | stateMachine: PAUSED → COMPLETED; các lần còn lại = SKIPPED; không có lần chạy mới. | `SM-Schedule`, `TRN-Schedule-7` | llm/approved |

### TOB-SCHEDULED-TRANSFER-SM-ScheduledRun-proposal

- Chủ thể: `SM-ScheduledRun`; nguồn gốc `llmProposal`; ưu tiên P1
- Câu hỏi: Điều kiện bổ sung do LLM đề xuất cho SM-ScheduledRun

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-SM-ScheduledRun-concurrency-p4b41b3` | concurrency | Hai scheduler cùng chiếm một lần chạy PENDING đến hạn: chỉ một UPDATE có điều kiện trả rowcount=1. | Chỉ một bên chuyển PENDING → RUNNING; attempts chỉ tăng 1; lõi chuyển tiền được gọi đúng một lần. | invariant: Đúng 1 bên rowcount=1; attempts = cũ + 1; đúng 1 lời gọi lõi. (đo: Chạy song song 2 tiến trình trên cùng bản ghi, lặp nhiều lần; đếm lời gọi lõi.) | `MOD-Scheduler`, `SM-ScheduledRun`, `TRN-ScheduledRun-1` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-SM-ScheduledRun-idempotency-p456351` | idempotency | Sau khi phục hồi lần chạy kẹt, lần chiếm lại dùng cùng khóa idempotency nên không tạo giao dịch trùng nếu lõi đã xử lý lần trước. | Lõi chuyển tiền phát lại giao dịch cũ; lần chạy sang SUCCEEDED theo TRN-ScheduledRun-2; không tạo thêm giao dịch. | postcondition: Số giao dịch của lần chạy = 1; transferId trùng giao dịch ban đầu; status=SUCCEEDED. (đo: Đếm giao dịch theo khóa idempotency sau chiếm lại.) | `SM-ScheduledRun`, `TRN-ScheduledRun-2`, `TRN-ScheduledRun-8` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-SM-ScheduledRun-performance-pa68a97` | performance | Mỗi chu kỳ quét 30 giây chỉ xử lý tối đa 20 lần chạy; lần chạy đến hạn vượt số này để lại PENDING cho chu kỳ sau. | Không quá 20 lần chuyển PENDING → RUNNING mỗi chu kỳ; phần còn lại giữ PENDING. | invariant: Số lần chạy được chiếm trong một chu kỳ ≤ 20. (đo: Seed 25 lần chạy đến hạn, đếm số bản ghi sang RUNNING sau đúng một chu kỳ.) | `MOD-Scheduler`, `SM-ScheduledRun` | llm/approved |

### TOB-SCHEDULED-TRANSFER-SM-ScheduledRun-states

- Chủ thể: `SM-ScheduledRun`; nguồn gốc `seedRule`; ưu tiên P1
- Câu hỏi: Chứng minh vòng đời lần chạy (khởi tạo PENDING) chỉ chuyển trạng thái qua 8 transition đã định nghĩa, mỗi transition chỉ xảy ra khi guard thỏa và tạo đủ hiệu ứng (attempts, audit, sự kiện, next_attempt_at, reasonCode); khi guard không thỏa thì trạng thái không đổi.

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-1-guardViolation-guard` | guardViolation | Lần chạy PENDING không được chiếm khi vi phạm guard: chưa đến hạn (scheduled_at/next_attempt_at > now), lịch không ACTIVE, hoặc UPDATE có điều kiện trả rowcount=0. | Lần chạy giữ PENDING; attempts không tăng; không sinh correlationId; không gọi lõi chuyển tiền. | stateMachine: status=PENDING, attempts không đổi, không có lời gọi lõi chuyển tiền. (đo: Đọc bản ghi ScheduledRun và log lời gọi lõi sau chu kỳ quét.) | `SM-ScheduledRun`, `TRN-ScheduledRun-1` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-1-stateTransition` | stateTransition | Lần chạy PENDING đã đến hạn, lịch ACTIVE, UPDATE có điều kiện rowcount=1 thì chuyển sang RUNNING. | status=RUNNING; attempts tăng 1; sinh correlationId; gọi lõi chuyển tiền đúng một lần. | stateMachine: PENDING → RUNNING; attempts = giá trị cũ + 1; correlationId khác rỗng; 1 lời gọi lõi chuyển tiền. (đo: Đọc bản ghi ScheduledRun và log lời gọi lõi sau khi chiếm.) | `SM-ScheduledRun`, `TRN-ScheduledRun-1` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-2-stateTransition` | stateTransition | Khi lõi chuyển tiền trả giao dịch COMPLETED (hoặc phát lại giao dịch đã hoàn tất), lần chạy RUNNING chuyển sang SUCCEEDED. | status=SUCCEEDED; lưu transferId; consecutive_failures=0; ghi audit; phát ScheduledRunCompleted; sinh lần kế tiếp hoặc đặt lịch COMPLETED nếu hết lần. | stateMachine: RUNNING → SUCCEEDED; transferId đã lưu; consecutive_failures=0; 1 audit; 1 sự kiện ScheduledRunCompleted; lần kế tiếp được sinh hoặc lịch COMPLETED. (đo: Đọc ScheduledRun, bảng audit và outbox/sự kiện sau khi xử lý.) | `SM-ScheduledRun`, `TRN-ScheduledRun-2` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-3-guardViolation-guard` | guardViolation | Lõi trả lỗi tạm thời/thiếu số dư nhưng đã hết lượt thử lại (FRAUD_CHECK_UNAVAILABLE quá 3 lần, INSUFFICIENT_FUNDS quá 2 lần) thì không quay về PENDING. | Không đặt next_attempt_at; lần chạy không về PENDING mà được xử lý theo TRN-ScheduledRun-4 (FAILED). | stateMachine: Không có chuyển RUNNING → PENDING; next_attempt_at không đổi; kết thúc ở FAILED theo TRN-ScheduledRun-4. (đo: Đọc ScheduledRun sau xử lý lỗi, với từng mã lỗi ở đúng biên giới hạn.) | `SM-ScheduledRun`, `TRN-ScheduledRun-3`, `TRN-ScheduledRun-4` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-3-stateTransition` | stateTransition | Lõi trả lỗi tạm thời hoặc thiếu số dư khi còn lượt thử lại thì lần chạy RUNNING về PENDING để thử lại. | status=PENDING; next_attempt_at = now + 5 phút hoặc now + 1 giờ tùy loại lỗi. | stateMachine: RUNNING → PENDING; next_attempt_at = now + 5 phút hoặc now + 1 giờ. (đo: So sánh next_attempt_at với thời điểm xử lý lỗi, mỗi mã lỗi một ca.) | `SM-ScheduledRun`, `TRN-ScheduledRun-3` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-4-guardViolation-guard` | guardViolation | Lõi trả lỗi nhưng lỗi không vĩnh viễn và còn lượt thử lại thì lần chạy không sang FAILED. | Không ghi reasonCode, consecutive_failures không tăng, không phát ScheduledRunFailed; lần chạy xử lý theo TRN-ScheduledRun-3. | stateMachine: status không phải FAILED; consecutive_failures không đổi; không có sự kiện ScheduledRunFailed. (đo: Đọc ScheduledRun và outbox sau xử lý lỗi.) | `SM-ScheduledRun`, `TRN-ScheduledRun-4` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-4-stateTransition` | stateTransition | Lõi trả lỗi vĩnh viễn hoặc đã hết lượt thử lại thì lần chạy RUNNING sang FAILED. | status=FAILED; lưu reasonCode; consecutive_failures tăng 1; ghi audit; phát ScheduledRunFailed. | stateMachine: RUNNING → FAILED; reasonCode = mã lỗi từ lõi; consecutive_failures = cũ + 1; 1 audit; 1 sự kiện ScheduledRunFailed. (đo: Đọc ScheduledRun, bảng audit và outbox sau xử lý.) | `SM-ScheduledRun`, `TRN-ScheduledRun-4` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-5-guardViolation-guard` | guardViolation | Khi tiếp tục lịch PAUSED, lần chạy PENDING chưa quá hạn trong thời gian tạm dừng thì không bị SKIPPED. | Lần chạy giữ PENDING và chạy theo lịch bình thường. | stateMachine: status=PENDING; không có chuyển sang SKIPPED. (đo: Đọc ScheduledRun ngay sau khi gọi tiếp tục lịch.) | `SM-ScheduledRun`, `TRN-ScheduledRun-5` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-5-stateTransition` | stateTransition | Khi tiếp tục lịch PAUSED, lần chạy PENDING đã quá hạn trong lúc tạm dừng chuyển sang SKIPPED và không chạy bù. | status=SKIPPED; không gọi lõi chuyển tiền cho lần chạy này; không tạo lần chạy bù. | stateMachine: PENDING → SKIPPED; không có lời gọi lõi chuyển tiền và không có lần chạy bù mới. (đo: Đọc ScheduledRun, log lời gọi lõi và số lần chạy của lịch sau khi tiếp tục.) | `SM-ScheduledRun`, `TRN-ScheduledRun-5` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-6-guardViolation-guard` | guardViolation | Đầu chu kỳ quét, lần chạy PENDING quá hạn không hơn 24 giờ so với scheduled_at thì không bị MISSED. | Lần chạy giữ PENDING; không phát ScheduledRunFailed với MISSED; không ghi audit MISSED. | stateMachine: status=PENDING; không có sự kiện MISSED. (đo: Đọc ScheduledRun và outbox sau chu kỳ quét; thử tại 23h59m và đúng 24h00m.) | `SM-ScheduledRun`, `TRN-ScheduledRun-6` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-6-stateTransition` | stateTransition | Đầu chu kỳ quét, lần chạy PENDING quá hạn hơn 24 giờ so với scheduled_at chuyển sang MISSED. | status=MISSED; phát ScheduledRunFailed với reasonCode MISSED; ghi audit; consecutive_failures không tăng. | stateMachine: PENDING → MISSED; 1 sự kiện ScheduledRunFailed có reasonCode=MISSED; 1 audit; consecutive_failures không đổi. (đo: Đọc ScheduledRun, audit, outbox và consecutive_failures trước/sau chu kỳ quét.) | `SM-ScheduledRun`, `TRN-ScheduledRun-6` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-7-stateTransition` | stateTransition | Khi DELETE lịch, lần chạy PENDING của lịch chuyển sang CANCELLED. | status=CANCELLED; lần chạy không còn được scheduler chiếm và không gọi lõi chuyển tiền. | stateMachine: PENDING → CANCELLED; chu kỳ quét sau đó không chiếm lần chạy này. (đo: Đọc ScheduledRun sau DELETE và sau một chu kỳ quét kế tiếp (30 giây).) | `SM-ScheduledRun`, `TRN-ScheduledRun-7` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-8-guardViolation-guard` | guardViolation | Lần chạy RUNNING có started_at chưa cũ hơn lease (120 giây) thì không bị phục hồi về PENDING. | Lần chạy giữ RUNNING; attempts không đổi. | stateMachine: status=RUNNING; attempts không đổi. (đo: Đọc ScheduledRun sau tiến trình phục hồi, thử tại 119 giây và đúng 120 giây.) | `SM-ScheduledRun`, `TRN-ScheduledRun-8` | llm/approved |
| `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-8-stateTransition` | stateTransition | Lần chạy RUNNING có started_at cũ hơn lease 120 giây được scheduler phục hồi về PENDING và giữ nguyên attempts. | status=PENDING; attempts giữ nguyên để lần chiếm sau dùng lại cùng khóa idempotency. | stateMachine: RUNNING → PENDING; attempts bằng giá trị trước phục hồi. (đo: Đọc ScheduledRun trước/sau phục hồi; lease 120 giây là đề xuất Q-S2, chưa chốt.) | `SM-ScheduledRun`, `TRN-ScheduledRun-8` | llm/approved |

## 4. Test Requirements theo mức

### 4.1 L1: Nghiệp vụ đầu-cuối

#### TR-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-functional

Mỗi chu kỳ quét 30 giây chọn tối đa 20 lần chạy đến hạn, chiếm từng lần bằng UPDATE có điều kiện, sinh correlationId riêng, gọi lõi chuyển tiền với Idempotency-Key = mã lịch + thời điểm dự kiến; kết quả là 1 giao dịch COMPLETED, run SUCCEEDED có transferId, 1 audit SCHEDULER, 1 sự kiện ScheduledRunCompleted trong outbox.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| SCN-ScheduledRun | functional | P1 | test | llm/approved |

Phạm vi: Chỉ phủ luồng chính thành công ở mức L1. Không phủ lỗi Fraud Check (TR L1 độ tin cậy), thời gian xử lý (TR L1 hiệu năng), chạy bù, PAUSED, đồng thời, khởi động lại hay tải 100 lần chạy (các TR L2).

Tiêu chí đạt:

1. Có đúng 1 giao dịch COMPLETED, run SUCCEEDED có transferId, 1 bản ghi audit tác nhân SCHEDULER kèm mã lịch, 1 sự kiện ScheduledRunCompleted có transferId trong outbox cùng giao dịch CSDL; giao dịch tính vào hạn mức ngày và hiện trong lịch sử. (`TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-functional-main`)

#### TR-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-performance

Một lần xử lý, từ bước chiếm lần chạy đến bước ghi audit/sự kiện, phải xong trong 60000 ms ở trường hợp xấu nhất; riêng Fraud Check tối đa 2 lần × 1000 ms + backoff 0 ms = 2000 ms.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| SCN-ScheduledRun | performance | P1 | test | llm/approved |

Phạm vi: Chỉ phủ ngân sách thời gian trường hợp xấu nhất của một lần xử lý và của lời gọi Fraud Check ở L1. Không phủ p95 chuyển tiền dưới tải 100 lần chạy (TR L2 hiệu năng), không tính khoảng chờ thử lại 5 phút, không phủ hành vi dự phòng FB-INT01.

Tiêu chí đạt:

1. Thời gian một lần xử lý đo từ bước đầu đến bước cuối ≤ 60000 ms trong trường hợp xấu nhất (Fraud Check tính 2 × 1000 ms, không dùng p95, không cộng khoảng thử lại 5 phút). (`TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-timing-budget`)
2. Tổng thời gian gọi Fraud Check trường hợp xấu nhất = 1000 ms × 2 lần + backoff 0 ms = 2000 ms ≤ 60000 ms. (`TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-timing-worst-INT-01`)

#### TR-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-reliability

Khi Fraud Check (INT-01) lỗi do timeout, 5xx, 4xx hoặc sai schema, hệ thống áp dụng FB-INT01 theo số tiền: dưới 5.000.000 VND cho qua với riskCheck=SKIPPED; từ 5.000.000 VND (kể cả đúng mức này) bị từ chối FRAUD_CHECK_UNAVAILABLE (503).

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| SCN-ScheduledRun | reliability | P1 | test | llm/approved |

Phạm vi: Chỉ phủ dự phòng khi INT-01 lỗi ở L1, gồm biên 5.000.000 VND. Không phủ ngân sách thời gian gọi Fraud Check (TR L1 hiệu năng), lỗi gửi SMS, đồng thời hay khởi động lại (TR L2 độ tin cậy).

Tiêu chí đạt:

1. Dưới 5.000.000 VND: riskCheck=SKIPPED, giao dịch COMPLETED, run SUCCEEDED. Từ 5.000.000 VND: FRAUD_CHECK_UNAVAILABLE (503), không có giao dịch, thử lại tối đa 3 lần cách 5 phút, hết lượt thì run FAILED kèm reasonCode. (`TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-errorHandling-fail-INT-01`)

### 4.2 L2: Nghiệp vụ

#### TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-functional

Xác nhận DELETE /v1/scheduled-transfers/{scheduleId}: hủy lịch ACTIVE của chính người gọi (không có lần RUNNING) trả 200, lịch và các lần PENDING thành CANCELLED, có đúng một audit hủy đủ trường; thiếu Authorization trả 401 UNAUTHORIZED; scheduleId không tồn tại trả 404 SCHEDULE_NOT_FOUND; lịch COMPLETED hoặc có lần RUNNING trả 409 SCHEDULE_NOT_CANCELLABLE; scheduleId thiếu/sai định dạng bị từ chối.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| EP-CancelSchedule | functional | P1 | test | llm/approved |

Phạm vi: Phủ hành vi chức năng và kiểm tra đầu vào của endpoint hủy lịch (thành công, audit, Authorization thiếu, scheduleId thiếu/không tồn tại, 409). KHÔNG phủ idempotency/đồng thời (TR reliability) và API key sai hoặc không phải chủ lịch (TR security).

Tiêu chí đạt:

1. Sau phản hồi 200, truy vấn bảng audit theo mã lịch thấy đúng một bản ghi loại sự kiện hủy, có correlationId, thời điểm, actor = mã tài khoản và mã lịch. (`TCND-SCHEDULED-TRANSFER-EP-CancelSchedule-audit-p1cc3a7`)
2. Status 401, errorCode = UNAUTHORIZED; trạng thái lịch và các lần chạy trong DB không đổi, không có audit hủy. (`TCND-SCHEDULED-TRANSFER-FLD-Cancel-Authorization-inputValidation-missing`)
3. Yêu cầu bị từ chối; không lịch nào đổi trạng thái, không có audit hủy. Mã HTTP và errorCode chưa xác định (chờ làm rõ), không khẳng định giá trị cụ thể. (`TCND-SCHEDULED-TRANSFER-FLD-Cancel-scheduleId-inputValidation-missing`)
4. Status 200; đọc lại DB thấy lịch = CANCELLED và mọi lần PENDING trước đó = CANCELLED. (`TCND-SCHEDULED-TRANSFER-RSP-Cancel-200-functional`)
5. Status 404, errorCode = SCHEDULE_NOT_FOUND; không có thay đổi dữ liệu. (`TCND-SCHEDULED-TRANSFER-RSP-Cancel-404-inputValidation`)
6. Cả hai ca (lịch COMPLETED; lịch có lần RUNNING) trả 409, errorCode = SCHEDULE_NOT_CANCELLABLE; trạng thái lịch và các lần chạy trong DB không đổi. (`TCND-SCHEDULED-TRANSFER-RSP-Cancel-409-inputValidation`)

#### TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-reliability

Xác nhận tính idempotent của DELETE hủy lịch: gửi lại cùng Idempotency-Key và scheduleId sau khi hủy thành công trả lại 200 cùng kết quả cũ; hai yêu cầu đồng thời cùng khóa chỉ hủy một lần; dùng lại khóa với tham số khác không tạo thêm hiệu ứng hủy.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| EP-CancelSchedule | reliability | P1 | test | llm/approved |

Phạm vi: Phủ replay, đồng thời trùng khóa và xung đột payload của Idempotency-Key. KHÔNG phủ luồng hủy thành công đơn lẻ và kiểm tra đầu vào (TR functional) hay xác thực/phân quyền (TR security).

Tiêu chí đạt:

1. Cả hai yêu cầu nhận 200 với nội dung giống nhau; lịch CANCELLED đúng một lần, các lần PENDING chuyển CANCELLED một lần, chỉ có một bản ghi audit hủy theo scheduleId. (`TCND-SCHEDULED-TRANSFER-EP-CancelSchedule-concurrency-concurrent-dup`)
2. Mã lỗi và hành vi chưa xác định (chờ chính sách xung đột khóa idempotency). Bất biến phải đạt: số bản ghi audit hủy không tăng và không phát sinh thêm hiệu ứng hủy. (`TCND-SCHEDULED-TRANSFER-EP-CancelSchedule-idempotency-payload-mismatch`)
3. Lần gửi lại trả 200, nội dung giống lần đầu; số bản ghi audit hủy không tăng; trạng thái lịch và các lần chạy không đổi thêm. (`TCND-SCHEDULED-TRANSFER-EP-CancelSchedule-idempotency-replay`)

#### TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-security

Xác nhận kiểm soát truy cập của DELETE hủy lịch: API key sai/không hợp lệ bị trả 401 UNAUTHORIZED; API key hợp lệ nhưng không phải chủ lịch bị trả 403 FORBIDDEN; cả hai trường hợp lịch vẫn ACTIVE và không đổi dữ liệu.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| EP-CancelSchedule | security | P1 | test | llm/approved |

Phạm vi: Chỉ phủ xác thực (API key sai) và phân quyền theo chủ lịch. KHÔNG phủ thiếu header Authorization (TR functional), hủy thành công, idempotency và đồng thời.

Tiêu chí đạt:

1. Status 401, errorCode = UNAUTHORIZED; lịch vẫn ACTIVE, không thay đổi dữ liệu. (`TCND-SCHEDULED-TRANSFER-RSP-Cancel-401-access`)
2. Status 403, errorCode = FORBIDDEN; lịch vẫn ACTIVE và các lần chạy không đổi trong DB. (`TCND-SCHEDULED-TRANSFER-RSP-Cancel-403-access`)

#### TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional

POST /v1/scheduled-transfers tạo lịch đúng với đầu vào hợp lệ (201, ACTIVE, ghi audit, không đổi số dư/ledger, lịch MONTHLY ngày 29-31 chạy vào ngày cuối tháng) và từ chối đầu vào sai bằng đúng mã lỗi: 400 INVALID_REQUEST, 404 ACCOUNT_NOT_FOUND, 409 IDEMPOTENCY_CONFLICT, 422 (LIMIT_EXCEEDED, SCHEDULE_LIMIT_REACHED, SAME_ACCOUNT, SCHEDULE_TIME_INVALID), 200 khi phát lại.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| EP-CreateSchedule | functional | P1 | test | llm/approved |

Phạm vi: Phủ hành vi chức năng: kiểm tra trường, biên giá trị (amount 1/50.000.000, maxRuns 1..60), hạn mức 20 lịch, audit, bất biến số dư. Không phủ đồng thời/độ tin cậy (reliability), phân quyền 401/403 (security), biên thời gian firstRunAt theo hiệu năng (performance).

Tiêu chí đạt:

1. Sau khi tạo lịch thành công có một bản ghi audit đủ correlationId, thời điểm, tác nhân là mã tài khoản, loại sự kiện và mã lịch. (`TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-audit-p28bd0d`)
2. Có đúng một bản ghi audit loại tạo lịch chứa correlationId, thời điểm, tác nhân = mã tài khoản, scheduleId. (`TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-audit-pf42db3`)
3. Lịch MONTHLY ngày 29/30/31 tạo được (201); lần chạy ở tháng 30 ngày rơi vào ngày 30, tháng 2 vào ngày 28 (29 ở năm nhuận). (`TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-functional-p2341c6`)
4. Lịch ONCE hợp lệ trả HTTP 201, có scheduleId, status=ACTIVE. (`TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-functional-pab4508`)
5. fromAccountId bằng toAccountId trả HTTP 422 SAME_ACCOUNT; không có bản ghi lịch mới. (`TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-guardViolation-p588a23`)
6. Lịch thứ 20 (ACTIVE/PAUSED) trả 201; lịch thứ 21 trả 422 SCHEDULE_LIMIT_REACHED. (`TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-guardViolation-pc9af5d`)
7. Lịch định kỳ thiếu cả endDate và maxRuns, hoặc maxRuns ngoài 1..60, trả 400 INVALID_REQUEST; không tạo lịch. (`TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-inputValidation-p4e161f`)
8. Sau khi tạo lịch, số dư hai tài khoản và số bản ghi ledger_entries không đổi. (`TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-invariant-p420954`)
9. Thiếu header Authorization trả HTTP 401 UNAUTHORIZED; không có bản ghi lịch mới. (`TCND-SCHEDULED-TRANSFER-FLD-Create-Authorization-inputValidation-missing`)
10. Thiếu header Idempotency-Key trả HTTP 400 INVALID_REQUEST; không có bản ghi lịch mới. (`TCND-SCHEDULED-TRANSFER-FLD-Create-IdempotencyKey-inputValidation-missing`)
11. amount=50000001 trả 422 LIMIT_EXCEEDED, không tạo lịch; amount=50000000 trả 201 khi các trường khác hợp lệ. (`TCND-SCHEDULED-TRANSFER-FLD-Create-amount-inputValidation-max`)
12. amount=0 trả 400 INVALID_REQUEST, không có bản ghi lịch mới; amount=1 được chấp nhận khi các trường khác hợp lệ. (`TCND-SCHEDULED-TRANSFER-FLD-Create-amount-inputValidation-min`)
13. Thiếu amount trả 400 INVALID_REQUEST; không có bản ghi lịch mới. (`TCND-SCHEDULED-TRANSFER-FLD-Create-amount-inputValidation-missing`)
14. Thiếu endDate (đồng thời thiếu maxRuns) trả 400 INVALID_REQUEST, không có lịch mới; nếu có maxRuns thì lịch định kỳ vẫn tạo được (201). Hành vi ONCE chưa xác định. (`TCND-SCHEDULED-TRANSFER-FLD-Create-endDate-inputValidation-missing`)
15. Thiếu firstRunAt trả 400 INVALID_REQUEST; không có bản ghi lịch mới. (`TCND-SCHEDULED-TRANSFER-FLD-Create-firstRunAt-inputValidation-missing`)
16. Thiếu fromAccountId trả 400 INVALID_REQUEST; không có bản ghi lịch mới. (`TCND-SCHEDULED-TRANSFER-FLD-Create-fromAccountId-inputValidation-missing`)
17. maxRuns=61 trả 400 INVALID_REQUEST, không có lịch mới; maxRuns=60 được chấp nhận khi các trường khác hợp lệ. (`TCND-SCHEDULED-TRANSFER-FLD-Create-maxRuns-inputValidation-max`)
18. maxRuns=0 trả 400 INVALID_REQUEST, không có lịch mới; maxRuns=1 được chấp nhận khi các trường khác hợp lệ. (`TCND-SCHEDULED-TRANSFER-FLD-Create-maxRuns-inputValidation-min`)
19. Thiếu maxRuns (đồng thời thiếu endDate) trả 400 INVALID_REQUEST, không có lịch mới; nếu có endDate thì lịch định kỳ vẫn tạo được (201). Hành vi ONCE chưa xác định. (`TCND-SCHEDULED-TRANSFER-FLD-Create-maxRuns-inputValidation-missing`)
20. recurrence=YEARLY (ngoài tập) trả 400 INVALID_REQUEST, không có lịch mới; ONCE, DAILY, WEEKLY, MONTHLY được chấp nhận khi các trường khác hợp lệ. (`TCND-SCHEDULED-TRANSFER-FLD-Create-recurrence-inputValidation-enum`)
21. Thiếu recurrence trả 400 INVALID_REQUEST; không có bản ghi lịch mới. (`TCND-SCHEDULED-TRANSFER-FLD-Create-recurrence-inputValidation-missing`)
22. Thiếu toAccountId trả 400 INVALID_REQUEST; không có bản ghi lịch mới. (`TCND-SCHEDULED-TRANSFER-FLD-Create-toAccountId-inputValidation-missing`)
23. Gửi lại cùng Idempotency-Key và nội dung giống hệt trả HTTP 200, body chứa lịch với scheduleId ban đầu; không tạo lịch mới. (`TCND-SCHEDULED-TRANSFER-RSP-Create-200-functional`)
24. Đầu vào hợp lệ, Idempotency-Key mới, tài khoản nguồn thuộc chủ API key: 201, scheduleId không rỗng, status=ACTIVE; số dư và số ledger_entries không đổi. (`TCND-SCHEDULED-TRANSFER-RSP-Create-201-functional`)
25. Mỗi biến thể vi phạm hợp đồng (thiếu key/trường bắt buộc, amount không nguyên dương, recurrence sai, thiếu endDate và maxRuns, maxRuns ngoài 1..60, firstRunAt sai định dạng) trả 400 INVALID_REQUEST; số lịch không đổi. (`TCND-SCHEDULED-TRANSFER-RSP-Create-400-inputValidation`)
26. toAccountId không tồn tại trả statusCode=404, errorCode=ACCOUNT_NOT_FOUND; không có lịch mới. (`TCND-SCHEDULED-TRANSFER-RSP-Create-404-inputValidation`)
27. Cùng Idempotency-Key nhưng body khác trả 409 IDEMPOTENCY_CONFLICT; lịch gốc giữ nguyên, tổng số lịch không tăng. (`TCND-SCHEDULED-TRANSFER-RSP-Create-409-inputValidation`)
28. amount=50.000.001 trả 422 LIMIT_EXCEEDED, không tạo lịch; amount=50.000.000 không bị từ chối vì hạn mức này. (`TCND-SCHEDULED-TRANSFER-RSP-Create-422-amount-inputValidation`)
29. Lịch thứ 21 khi đã có 20 lịch ACTIVE/PAUSED trả 422 SCHEDULE_LIMIT_REACHED; số lịch ACTIVE/PAUSED vẫn bằng 20. (`TCND-SCHEDULED-TRANSFER-RSP-Create-422-limit-inputValidation`)
30. fromAccountId bằng toAccountId trả statusCode=422, errorCode=SAME_ACCOUNT; không có lịch mới. (`TCND-SCHEDULED-TRANSFER-RSP-Create-422-same-inputValidation`)
31. firstRunAt cách hiện tại <5 phút hoặc >90 ngày, hoặc endDate trước lần chạy đầu hoặc quá 366 ngày, trả 422 SCHEDULE_TIME_INVALID; không có lịch mới. (`TCND-SCHEDULED-TRANSFER-RSP-Create-422-time-inputValidation`)

#### TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-performance

Kiểm biên thời gian của firstRunAt: giá trị cách thời điểm hiện tại dưới 5 phút hoặc trên 90 ngày bị từ chối bằng 422 SCHEDULE_TIME_INVALID, giá trị trong khoảng 5 phút đến 90 ngày được chấp nhận.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| EP-CreateSchedule | performance | P1 | test | llm/approved |

Phạm vi: Chỉ phủ biên khoảng thời gian của firstRunAt. Không phủ endDate (quá 366 ngày/trước lần chạy đầu), kiểm tra trường khác (functional), đồng thời/idempotency (reliability) và phân quyền (security).

Tiêu chí đạt:

1. firstRunAt cách hiện tại dưới 5 phút hoặc trên 90 ngày trả 422 SCHEDULE_TIME_INVALID; giá trị trong khoảng 5 phút đến 90 ngày trả 201. (`TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-timing-p31f6cf`)

#### TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-reliability

Kiểm tính idempotent và đồng thời của POST tạo lịch: gửi lại cùng key và nội dung trả lại đúng lịch cũ (200), hai yêu cầu đồng thời cùng key chỉ sinh một lịch, khác nội dung bị 409, và toAccountId không tồn tại bị 404.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| EP-CreateSchedule | reliability | P1 | test | llm/approved |

Phạm vi: Phủ replay, trùng đồng thời, xung đột payload và lỗi tài khoản đích không tồn tại. Không phủ kiểm tra trường/biên giá trị và audit lần đầu (functional), biên thời gian (performance), 401/403 (security).

Tiêu chí đạt:

1. Hai POST đồng thời cùng key và nội dung: chỉ một lịch (một scheduleId); yêu cầu còn lại nhận HTTP 200 với scheduleId = S; số lịch và audit tạo lịch theo key chỉ tăng một lần. (`TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-concurrency-concurrent-dup`)
2. Hai request đồng thời cùng key K và nội dung: đúng 1 lịch với key K; hai phản hồi cùng scheduleId. (`TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-concurrency-pdcbc85`)
3. toAccountId không tồn tại trả HTTP 404 ACCOUNT_NOT_FOUND; không có bản ghi lịch mới. (`TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-errorHandling-pe11059`)
4. Gửi lại cùng key và nội dung trả 200 với cùng scheduleId; tổng số lịch không tăng. (`TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-idempotency-p50d7db`)
5. Key đã dùng nhưng body khác trả 409 IDEMPOTENCY_CONFLICT; số lịch không đổi, lịch đã có không bị thay đổi. (`TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-idempotency-payload-mismatch`)
6. Phát lại yêu cầu đã thành công trả 200 với scheduleId = S; tổng số lịch và số bản ghi audit tạo lịch không tăng. (`TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-idempotency-replay`)

#### TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-security

Kiểm kiểm soát truy cập của POST tạo lịch: thiếu Authorization hoặc API key sai bị 401 UNAUTHORIZED; API key hợp lệ nhưng fromAccountId không thuộc chủ khóa bị 403 FORBIDDEN; cả hai trường hợp không tạo lịch.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| EP-CreateSchedule | security | P1 | test | llm/approved |

Phạm vi: Chỉ phủ xác thực (401) và phân quyền theo chủ tài khoản nguồn (403). Không phủ kiểm tra trường/biên giá trị (functional), idempotency/đồng thời (reliability) và biên thời gian (performance).

Tiêu chí đạt:

1. fromAccountId khác tài khoản của chủ API key trả HTTP 403 FORBIDDEN; không có bản ghi lịch mới. (`TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-access-p73a390`)
2. Authorization thiếu hoặc API key sai trả statusCode=401, errorCode=UNAUTHORIZED; số lịch của tài khoản không đổi. (`TCND-SCHEDULED-TRANSFER-RSP-Create-401-access`)
3. API key hợp lệ nhưng fromAccountId không thuộc chủ khóa trả statusCode=403, errorCode=FORBIDDEN; không có lịch mới cho tài khoản B. (`TCND-SCHEDULED-TRANSFER-RSP-Create-403-access`)

#### TR-SCHEDULED-TRANSFER-L2-EP-GetSchedule-functional

Xác nhận GET /v1/scheduled-transfers/{scheduleId} trả 200 kèm status và nextRunAt đúng dữ liệu đã lưu cho chủ tài khoản nguồn với lịch tồn tại; trả 404 SCHEDULE_NOT_FOUND khi scheduleId đúng định dạng nhưng không tồn tại; từ chối khi thiếu scheduleId hoặc thiếu header Authorization (401 UNAUTHORIZED).

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| EP-GetSchedule | functional | P1 | test | llm/approved |

Phạm vi: Phủ luồng thành công, lịch không tồn tại và thiếu tham số bắt buộc (Authorization, scheduleId). Không phủ API key sai/không phải chủ lịch (TR security) và tính idempotent khi gọi lặp (TR reliability).

Tiêu chí đạt:

1. Thiếu header Authorization: HTTP 401, body có errorCode = UNAUTHORIZED, không chứa status hay nextRunAt. (`TCND-SCHEDULED-TRANSFER-FLD-Get-Authorization-inputValidation-missing`)
2. Path thiếu scheduleId: yêu cầu bị từ chối, không có phản hồi 200 và không trả dữ liệu lịch; mã HTTP và errorCode chờ làm rõ. (`TCND-SCHEDULED-TRANSFER-FLD-Get-scheduleId-inputValidation-missing`)
3. HTTP 200, body chứa status và nextRunAt khớp với dữ liệu lịch đã lưu. (`TCND-SCHEDULED-TRANSFER-RSP-Get-200-functional`)
4. HTTP 404, errorCode = SCHEDULE_NOT_FOUND. (`TCND-SCHEDULED-TRANSFER-RSP-Get-404-inputValidation`)

#### TR-SCHEDULED-TRANSFER-L2-EP-GetSchedule-reliability

Xác nhận GET lịch là idempotent: hai hoặc nhiều lần gọi liên tiếp khi lịch không đổi đều trả 200 với cùng status và nextRunAt, và dữ liệu lịch trong kho không bị thay đổi sau các lần gọi.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| EP-GetSchedule | reliability | P1 | test | llm/approved |

Phạm vi: Chỉ phủ tính idempotent khi gọi lặp lại. Không phủ nội dung phản hồi lần đầu so với dữ liệu lưu ngoài so sánh giữa các lần (TR functional) và kiểm soát truy cập (TR security).

Tiêu chí đạt:

1. Hai phản hồi 200 có status và nextRunAt giống nhau; dữ liệu lịch trong kho không đổi sau các lần GET. (`TCND-SCHEDULED-TRANSFER-EP-GetSchedule-idempotency-p52c40c`)

#### TR-SCHEDULED-TRANSFER-L2-EP-GetSchedule-security

Xác nhận GET lịch từ chối truy cập trái phép: API key sai/không hợp lệ nhận 401 UNAUTHORIZED; API key hợp lệ nhưng không phải chủ tài khoản nguồn của lịch nhận 403 FORBIDDEN; cả hai trường hợp không lộ dữ liệu lịch.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| EP-GetSchedule | security | P1 | test | llm/approved |

Phạm vi: Phủ xác thực (API key sai) và phân quyền (không phải chủ lịch). Không phủ trường hợp thiếu header Authorization (TR functional), luồng 200/404 và tính idempotent.

Tiêu chí đạt:

1. HTTP 401, errorCode = UNAUTHORIZED, không lộ dữ liệu lịch. (`TCND-SCHEDULED-TRANSFER-RSP-Get-401-access`)
2. HTTP 403, errorCode = FORBIDDEN, body không chứa status/nextRunAt. (`TCND-SCHEDULED-TRANSFER-RSP-Get-403-access`)

#### TR-SCHEDULED-TRANSFER-L2-EP-ListRuns-functional

Kiểm tra GET /v1/scheduled-transfers/{scheduleId}/runs: chủ lịch với Authorization hợp lệ nhận HTTP 200, danh sách giảm dần theo scheduledAt; từ chối thiếu Authorization (401 UNAUTHORIZED), limit=0 và limit=101, offset=-1, thiếu scheduleId, scheduleId không tồn tại (404 SCHEDULE_NOT_FOUND); chấp nhận biên limit=1, limit=100, offset=0.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| EP-ListRuns | functional | P1 | test | llm/approved |

Phạm vi: Phủ chức năng và kiểm tra đầu vào (Authorization thiếu, limit, offset, scheduleId thiếu/không tồn tại, kết quả 200). KHÔNG phủ kiểm soát truy cập với API key sai hoặc không phải chủ lịch (401/403 nằm ở TR security).

Tiêu chí đạt:

1. Khi thiếu header Authorization: HTTP 401, errorCode UNAUTHORIZED; không trả dữ liệu lần chạy nào. (`TCND-SCHEDULED-TRANSFER-FLD-ListRuns-Authorization-inputValidation-missing`)
2. limit=101 bị từ chối với lỗi kiểm tra đầu vào, không có dữ liệu lần chạy; limit=100 được chấp nhận. Mã HTTP/errorCode chưa xác định trong nguồn nên không khẳng định. (`TCND-SCHEDULED-TRANSFER-FLD-ListRuns-limit-inputValidation-max`)
3. limit=0 bị từ chối với lỗi kiểm tra đầu vào, không có dữ liệu lần chạy; limit=1 được chấp nhận. Mã HTTP/errorCode chưa xác định trong nguồn nên không khẳng định. (`TCND-SCHEDULED-TRANSFER-FLD-ListRuns-limit-inputValidation-min`)
4. offset=-1 bị từ chối với lỗi kiểm tra đầu vào, không có dữ liệu lần chạy; offset=0 được chấp nhận. Mã HTTP/errorCode chưa xác định trong nguồn nên không khẳng định. (`TCND-SCHEDULED-TRANSFER-FLD-ListRuns-offset-inputValidation-min`)
5. Đường dẫn thiếu scheduleId bị từ chối với lỗi kiểm tra đầu vào, không trả danh sách lần chạy. Mã HTTP cụ thể chưa xác định trong nguồn. (`TCND-SCHEDULED-TRANSFER-FLD-ListRuns-scheduleId-inputValidation-missing`)
6. HTTP 200; danh sách giảm dần theo scheduledAt (mới nhất trước); mỗi phần tử có scheduledAt, status, transferId, reasonCode. (`TCND-SCHEDULED-TRANSFER-RSP-ListRuns-200-functional`)
7. scheduleId không tương ứng lịch nào: HTTP 404, errorCode SCHEDULE_NOT_FOUND. (`TCND-SCHEDULED-TRANSFER-RSP-ListRuns-404-inputValidation`)

#### TR-SCHEDULED-TRANSFER-L2-EP-ListRuns-security

Kiểm tra kiểm soát truy cập của GET /v1/scheduled-transfers/{scheduleId}/runs: API key thiếu hoặc sai bị trả 401 UNAUTHORIZED; API key hợp lệ nhưng không phải chủ tài khoản nguồn của lịch bị trả 403 FORBIDDEN; cả hai trường hợp không lộ dữ liệu lần chạy.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| EP-ListRuns | security | P1 | test | llm/approved |

Phạm vi: Chỉ phủ xác thực (401) và phân quyền theo chủ lịch (403). KHÔNG phủ kiểm tra tham số limit, offset, scheduleId, kết quả 200 hay 404 (nằm ở TR functional).

Tiêu chí đạt:

1. API key thiếu hoặc sai: HTTP 401, errorCode UNAUTHORIZED; không lộ dữ liệu lần chạy. (`TCND-SCHEDULED-TRANSFER-RSP-ListRuns-401-access`)
2. API key hợp lệ nhưng không phải chủ tài khoản nguồn của lịch: HTTP 403, errorCode FORBIDDEN; không trả lần chạy nào. (`TCND-SCHEDULED-TRANSFER-RSP-ListRuns-403-access`)

#### TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules-functional

Kiểm tra GET danh sách lịch chuyển tiền: thiếu Authorization trả 401 UNAUTHORIZED; thiếu accountId bị từ chối; limit biên 0/1 và 100/101; offset biên -1/0; status thuộc 4 giá trị hợp lệ trả 200, UNKNOWN trả 400 INVALID_REQUEST; luồng thành công trả đúng lịch của chủ tài khoản theo lọc và phân trang.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| EP-ListSchedules | functional | P1 | test | llm/approved |

Phạm vi: Phủ xác thực đầu vào (Authorization, accountId, limit, offset, status), phản hồi 200 và 400 do status sai. Không phủ API key sai/khác tài khoản (401/403 ở TR security) và gọi lặp (TR reliability).

Tiêu chí đạt:

1. Không có header Authorization: HTTP 401, errorCode UNAUTHORIZED, body không chứa phần tử lịch nào. (`TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-Authorization-inputValidation-missing`)
2. Thiếu đoạn path accountId: yêu cầu bị từ chối, không trả danh sách lịch. Mã HTTP và errorCode chưa được nguồn xác định (gap), cần xác nhận trước khi chốt. (`TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-accountId-inputValidation-missing`)
3. limit=100: HTTP 200, tối đa 100 phần tử. limit=101: HTTP 400, errorCode INVALID_REQUEST (dự kiến, chờ xác nhận). (`TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-limit-inputValidation-max`)
4. limit=1: HTTP 200, tối đa 1 phần tử. limit=0: HTTP 400, errorCode INVALID_REQUEST (dự kiến, chờ xác nhận). (`TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-limit-inputValidation-min`)
5. offset=0: HTTP 200, danh sách bắt đầu từ phần tử đầu. offset=-1: HTTP 400, errorCode INVALID_REQUEST (dự kiến, chờ xác nhận). (`TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-offset-inputValidation-min`)
6. status là ACTIVE, PAUSED, COMPLETED hoặc CANCELLED: HTTP 200. status=UNKNOWN: HTTP 400, errorCode INVALID_REQUEST. (`TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-status-inputValidation-enum`)
7. HTTP 200; mọi phần tử thuộc accountId và có status=ACTIVE; số phần tử <= 2; bắt đầu từ vị trí offset=1 của tập đã lọc. (`TCND-SCHEDULED-TRANSFER-RSP-ListSchedules-200-functional`)
8. status ngoài enum: HTTP 400, errorCode INVALID_REQUEST, không trả danh sách lịch. (`TCND-SCHEDULED-TRANSFER-RSP-ListSchedules-400-inputValidation`)

#### TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules-reliability

Kiểm tra tính idempotent của GET danh sách lịch: gọi lặp lại hai lần với cùng tham số khi dữ liệu không đổi thì cho cùng kết quả và không làm đổi trạng thái lịch hay lần chạy.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| EP-ListSchedules | reliability | P1 | test | llm/approved |

Phạm vi: Chỉ phủ tính lặp lại an toàn của GET khi dữ liệu không đổi. Không phủ kiểm tra đầu vào, phân trang (TR functional) hay xác thực/phân quyền (TR security).

Tiêu chí đạt:

1. Hai phản hồi HTTP 200 có cùng nội dung; trạng thái lịch và lần chạy trước và sau các lần gọi không đổi. (`TCND-SCHEDULED-TRANSFER-EP-ListSchedules-idempotency-p6e1377`)

#### TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules-security

Kiểm tra kiểm soát truy cập của GET danh sách lịch: API key thiếu hoặc sai bị trả 401 UNAUTHORIZED; API key hợp lệ nhưng accountId thuộc tài khoản khác bị trả 403 FORBIDDEN, không lộ dữ liệu lịch.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| EP-ListSchedules | security | P1 | test | llm/approved |

Phạm vi: Chỉ phủ xác thực API key (401) và phân quyền theo tài khoản (403). Không phủ kiểm tra giá trị tham số limit/offset/status (TR functional) hay tính lặp lại (TR reliability).

Tiêu chí đạt:

1. API key thiếu hoặc sai: HTTP 401, errorCode UNAUTHORIZED, không lộ dữ liệu lịch. (`TCND-SCHEDULED-TRANSFER-RSP-ListSchedules-401-access`)
2. API key hợp lệ nhưng accountId khác tài khoản người gọi: HTTP 403, errorCode FORBIDDEN, body không chứa lịch của tài khoản B. (`TCND-SCHEDULED-TRANSFER-RSP-ListSchedules-403-access`)

#### TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-functional

Kiểm PATCH đổi trạng thái lịch: chuyển hợp lệ PAUSED/ACTIVE bởi chủ lịch trả 200; status thiếu hoặc ngoài {PAUSED, ACTIVE} trả 400 INVALID_REQUEST; scheduleId không tồn tại trả 404; chuyển không hợp lệ (kể cả lịch ONCE) trả 409; Authorization thiếu trả 401; thành công phải có audit đủ trường.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| EP-UpdateScheduleStatus | functional | P1 | test | llm/approved |

Phạm vi: Phủ hành vi chức năng và kiểm tra đầu vào của PATCH. Không phủ API key sai/không phải chủ lịch (TR security) và đua với scheduler (TR reliability). Kết quả khi scheduleId thiếu chưa chắc chắn.

Tiêu chí đạt:

1. Sau PATCH thành công tồn tại bản ghi audit có đủ correlationId, thời điểm, tác nhân = mã tài khoản, loại sự kiện, mã lịch. (`TCND-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-audit-p664d35`)
2. Thiếu header Authorization: HTTP 401, errorCode UNAUTHORIZED, trạng thái lịch giữ nguyên. (`TCND-SCHEDULED-TRANSFER-FLD-Patch-Authorization-inputValidation-missing`)
3. scheduleId thiếu: oracle ghi HTTP 400, errorCode INVALID_REQUEST, lịch giữ nguyên; nguồn chưa xác nhận nên cần xác nhận lại trước khi chốt. (`TCND-SCHEDULED-TRANSFER-FLD-Patch-scheduleId-inputValidation-missing`)
4. status PAUSED/ACTIVE hợp lệ được chấp nhận về định dạng; status ngoài tập: HTTP 400, errorCode INVALID_REQUEST, lịch giữ nguyên trạng thái. (`TCND-SCHEDULED-TRANSFER-FLD-Patch-status-inputValidation-enum`)
5. Body thiếu status: HTTP 400, errorCode INVALID_REQUEST, lịch giữ nguyên trạng thái. (`TCND-SCHEDULED-TRANSFER-FLD-Patch-status-inputValidation-missing`)
6. HTTP 200, body lịch có status = giá trị yêu cầu; sau PAUSED không phát sinh lần chạy mới; sau ACTIVE các lần bỏ lỡ là SKIPPED và không chạy bù. (`TCND-SCHEDULED-TRANSFER-RSP-Patch-200-functional`)
7. HTTP 400, errorCode INVALID_REQUEST; trạng thái lịch không đổi, không ghi audit chuyển trạng thái thành công. (`TCND-SCHEDULED-TRANSFER-RSP-Patch-400-inputValidation`)
8. HTTP 404, errorCode SCHEDULE_NOT_FOUND. (`TCND-SCHEDULED-TRANSFER-RSP-Patch-404-inputValidation`)
9. HTTP 409, errorCode SCHEDULE_STATE_INVALID; trạng thái lịch không đổi. (`TCND-SCHEDULED-TRANSFER-RSP-Patch-409-inputValidation`)

#### TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-reliability

Kiểm tình huống đồng thời: scheduler đang chiếm lần chạy đến hạn đúng lúc lịch bị PATCH sang PAUSED, và ghi nhận hành vi của lần chạy đã chiếm.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| EP-UpdateScheduleStatus | reliability | P1 | test | llm/approved |

Phạm vi: Chỉ phủ tranh chấp giữa scheduler và PATCH PAUSED. Không phủ chuyển trạng thái thông thường hay kiểm tra đầu vào (TR functional) và xác thực/phân quyền (TR security).

Tiêu chí đạt:

1. Hành vi chưa xác định, oracle là đánh giá của con người: cần xác nhận lần chạy đã chiếm tiếp tục chạy hay bị hủy/SKIPPED; không tự suy đoán kết quả. (`TCND-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-concurrency-p1e8657`)

#### TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-security

Kiểm quyền truy cập PATCH: API key thiếu hoặc sai bị từ chối 401 UNAUTHORIZED; API key hợp lệ nhưng không phải chủ lịch bị từ chối 403 FORBIDDEN; lịch không bị thay đổi.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| EP-UpdateScheduleStatus | security | P1 | test | llm/approved |

Phạm vi: Chỉ phủ xác thực API key và quyền sở hữu lịch. Không phủ logic chuyển trạng thái, kiểm tra body/path (TR functional) và đua với scheduler (TR reliability).

Tiêu chí đạt:

1. API key thiếu hoặc sai: HTTP 401, errorCode UNAUTHORIZED; lịch không bị thay đổi. (`TCND-SCHEDULED-TRANSFER-RSP-Patch-401-access`)
2. API key hợp lệ nhưng không phải chủ lịch: HTTP 403, errorCode FORBIDDEN; trạng thái lịch giữ nguyên. (`TCND-SCHEDULED-TRANSFER-RSP-Patch-403-access`)

#### TR-SCHEDULED-TRANSFER-L2-FR-040-functional

Xác nhận POST /v1/scheduled-transfers tạo lịch ONCE hợp lệ trả 201 (scheduleId, status=ACTIVE), không chuyển tiền ngay, có audit; từ chối đúng mã lỗi khi firstRunAt ngoài 5 phút..90 ngày, toAccountId không tồn tại, fromAccountId không thuộc chủ key, vượt 20 lịch, amount >50.000.000 hoặc trùng tài khoản, đầu vào sai.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| FR-040 | functional | P0 | test | llm/approved |

Phạm vi: Phủ luồng chức năng, quy tắc nghiệp vụ, kiểm đầu vào và audit của việc tạo lịch. KHÔNG phủ đồng thời và idempotency (TR reliability), thiếu/sai API key 401 (TR security).

Tiêu chí đạt:

1. HTTP 201; body có scheduleId không rỗng và status=ACTIVE; bản ghi trong scheduled_transfers có status ACTIVE. (`TCND-SCHEDULED-TRANSFER-AC-FR-040-1-functional`)
2. Số dư fromAccountId và toAccountId không đổi; số bản ghi ledger_entries sau bằng số trước lời gọi. (`TCND-SCHEDULED-TRANSFER-AC-FR-040-2-functional`)
3. firstRunAt ngoài khoảng (dưới 5 phút hoặc quá 90 ngày, kiểm các biên): HTTP 422, errorCode=SCHEDULE_TIME_INVALID; không có bản ghi lịch mới. (`TCND-SCHEDULED-TRANSFER-AC-FR-040-3-functional`)
4. HTTP 404, errorCode=ACCOUNT_NOT_FOUND; số bản ghi scheduled_transfers không đổi. (`TCND-SCHEDULED-TRANSFER-AC-FR-040-4-functional`)
5. HTTP 403, errorCode=FORBIDDEN; không có bản ghi lịch mới. (`TCND-SCHEDULED-TRANSFER-AC-FR-040-5-functional`)
6. Lịch thứ 20 (19→20) tạo thành công; lịch thứ 21 (20→21): HTTP 422, errorCode=SCHEDULE_LIMIT_REACHED, số lịch ACTIVE/PAUSED vẫn là 20. (`TCND-SCHEDULED-TRANSFER-AC-FR-040-6-functional`)
7. Tồn tại bản ghi audit tạo lịch ứng với scheduleId đã trả. (`TCND-SCHEDULED-TRANSFER-FR-040-audit-p7a419a`)
8. amount=50.000.000 được chấp nhận; amount=50.000.001 trả HTTP 422 LIMIT_EXCEEDED; fromAccountId=toAccountId trả HTTP 422 SAME_ACCOUNT; không có bản ghi lịch mới khi bị từ chối. (`TCND-SCHEDULED-TRANSFER-FR-040-decisionRule-pc6e1a6`)
9. Mỗi ca (thiếu Idempotency-Key, amount không nguyên dương, recurrence ngoài enum, maxRuns ngoài 1..60, firstRunAt sai định dạng) trả HTTP 400, errorCode=INVALID_REQUEST; không có bản ghi lịch mới. (`TCND-SCHEDULED-TRANSFER-FR-040-inputValidation-p49f012`)

#### TR-SCHEDULED-TRANSFER-L2-FR-040-reliability

Xác nhận tính nhất quán khi tạo lịch: hai yêu cầu đồng thời lúc tài khoản có 19 lịch ACTIVE/PAUSED không vượt 20 lịch; gửi lại cùng Idempotency-Key và cùng nội dung trả 200 với lịch cũ; cùng key khác nội dung trả 409.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| FR-040 | reliability | P0 | test | llm/approved |

Phạm vi: Chỉ phủ đồng thời và idempotency của việc tạo lịch. KHÔNG phủ kiểm đầu vào, quy tắc nghiệp vụ đơn lẻ (TR functional) và xác thực API key (TR security).

Tiêu chí đạt:

1. Tối đa một yêu cầu trả 201, yêu cầu còn lại trả 422 SCHEDULE_LIMIT_REACHED; số lịch ACTIVE/PAUSED của tài khoản luôn ≤ 20. (`TCND-SCHEDULED-TRANSFER-FR-040-concurrency-pd7d28d`)
2. HTTP 200, trả cùng scheduleId như lần đầu; chỉ có một bản ghi lịch trong scheduled_transfers. (`TCND-SCHEDULED-TRANSFER-FR-040-idempotency-p8f0346`)
3. HTTP 409, errorCode=IDEMPOTENCY_CONFLICT; số bản ghi lịch không đổi. (`TCND-SCHEDULED-TRANSFER-FR-040-idempotency-pccb8db`)

#### TR-SCHEDULED-TRANSFER-L2-FR-040-security

Xác nhận yêu cầu tạo lịch thiếu API key hoặc dùng API key sai bị từ chối với HTTP 401 UNAUTHORIZED và không tạo lịch.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| FR-040 | security | P0 | test | llm/approved |

Phạm vi: Chỉ phủ xác thực API key (thiếu/sai). KHÔNG phủ quyền sở hữu fromAccountId (403, TR functional), đồng thời và idempotency (TR reliability).

Tiêu chí đạt:

1. HTTP 401, errorCode=UNAUTHORIZED cho cả trường hợp thiếu và sai API key; không có bản ghi lịch mới. (`TCND-SCHEDULED-TRANSFER-FR-040-access-p4f809f`)

#### TR-SCHEDULED-TRANSFER-L2-FR-041-functional

Xác minh tạo lịch chuyển tiền định kỳ: hợp lệ (DAILY/WEEKLY/MONTHLY, endDate hoặc maxRuns 1..60, firstRunAt 5 phút..90 ngày, endDate ≤366 ngày) trả 201 ACTIVE; thiếu cả endDate và maxRuns trả 400; Scheduler sinh lần chạy kế tiếp đúng chu kỳ và đóng lịch; ngày neo 29/30/31 xử lý đúng cuối tháng; có audit; endDate sai và vượt 20 lịch bị từ chối; maxRuns ngoài 1..60 bị 400.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| FR-041 | functional | P0 | test | llm/approved |

Phạm vi: Phủ hành vi chức năng của FR-041: tạo lịch, kiểm tra đầu vào, sinh lần chạy kế tiếp, ngày neo cuối tháng, audit, các guard 422. KHÔNG phủ idempotency (TR reliability) và xác thực/phân quyền 401/403 (TR security).

Tiêu chí đạt:

1. HTTP 201, status=ACTIVE, có mã lịch trong phản hồi; bản ghi lịch đọc lại từ CSDL khớp, lần chạy đầu PENDING; đúng cho từng recurrence và cho cả ba cách đặt giới hạn (chỉ endDate, chỉ maxRuns, cả hai). (`TCND-SCHEDULED-TRANSFER-AC-FR-041-1-functional`)
2. HTTP 400, errorCode=INVALID_REQUEST khi thiếu cả endDate và maxRuns; số bản ghi scheduled_transfers của tài khoản không đổi. (`TCND-SCHEDULED-TRANSFER-AC-FR-041-2-functional`)
3. Với maxRuns=N có đúng N lần chạy cách nhau đúng chu kỳ (1 ngày, 7 ngày, 1 tháng), lịch COMPLETED sau lần thứ N; với endDate không có lần chạy nào sau endDate và lịch COMPLETED. (`TCND-SCHEDULED-TRANSFER-AC-FR-041-3-functional`)
4. Neo 31: 31/01 → 28/02 (năm thường) hoặc 29/02 (năm nhuận) → 31/03 → 30/04. Neo 30: tháng 2 chạy ngày cuối tháng. Neo 29: 28/02 năm thường, 29/02 năm nhuận. (`TCND-SCHEDULED-TRANSFER-AC-FR-041-4-functional`)
5. Sau khi nhận 201, tồn tại bản ghi audit tạo lịch liên kết đúng mã lịch vừa tạo. (`TCND-SCHEDULED-TRANSFER-FR-041-audit-p1cd2c4`)
6. HTTP 422, errorCode=SCHEDULE_TIME_INVALID, không tạo lịch; kiểm tra biên 366 ngày (chấp nhận) và 367 ngày (từ chối), và endDate trước lần chạy đầu. (`TCND-SCHEDULED-TRANSFER-FR-041-guardViolation-p241776`)
7. Với 20 lịch ACTIVE/PAUSED: HTTP 422, errorCode=SCHEDULE_LIMIT_REACHED, số lịch không đổi; với 19 lịch thì tạo thành công. (`TCND-SCHEDULED-TRANSFER-FR-041-guardViolation-pcbab3f`)
8. HTTP 400, errorCode=INVALID_REQUEST và không tạo lịch với maxRuns không nguyên hoặc ngoài 1..60; biên 0 và 61 bị từ chối, 1 và 60 được chấp nhận. (`TCND-SCHEDULED-TRANSFER-FR-041-inputValidation-p31ec2a`)

#### TR-SCHEDULED-TRANSFER-L2-FR-041-reliability

Xác minh gửi lại yêu cầu tạo lịch định kỳ với cùng Idempotency-Key: cùng nội dung không sinh lịch thứ hai và trả lại lịch đã tạo (200); khác nội dung bị từ chối bằng 409 IDEMPOTENCY_CONFLICT.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| FR-041 | reliability | P0 | test | llm/approved |

Phạm vi: Chỉ phủ tính idempotent khi gửi lại yêu cầu tạo lịch. KHÔNG phủ logic tạo lịch và các guard nghiệp vụ (TR functional) hay xác thực/phân quyền (TR security).

Tiêu chí đạt:

1. Cùng nội dung: HTTP 200, cùng mã lịch, tổng số lịch không đổi (một bản ghi). Khác nội dung: HTTP 409, errorCode=IDEMPOTENCY_CONFLICT. (`TCND-SCHEDULED-TRANSFER-FR-041-idempotency-pd36026`)

#### TR-SCHEDULED-TRANSFER-L2-FR-041-security

Xác minh yêu cầu tạo lịch định kỳ bị từ chối khi fromAccountId không thuộc người gọi (403 FORBIDDEN) hoặc khi API key thiếu/sai (401 UNAUTHORIZED), và không tạo lịch nào.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| FR-041 | security | P0 | test | llm/approved |

Phạm vi: Chỉ phủ kiểm soát truy cập khi tạo lịch (sở hữu tài khoản, API key). KHÔNG phủ chức năng tạo lịch và guard nghiệp vụ (TR functional) hay idempotency (TR reliability).

Tiêu chí đạt:

1. fromAccountId của người khác: HTTP 403 FORBIDDEN. API key thiếu hoặc sai: HTTP 401 UNAUTHORIZED. Số bản ghi lịch không đổi trong cả hai trường hợp. (`TCND-SCHEDULED-TRANSFER-FR-041-access-pfae77d`)

#### TR-SCHEDULED-TRANSFER-L2-FR-042-functional

Chủ tài khoản nguồn tra cứu được một lịch (trả 200 kèm status, nextRunAt khớp dữ liệu lưu, không đổi dữ liệu), danh sách lịch của chính mình (lọc status, phân trang limit 1..100, offset >=0, không trùng/sót giữa các trang), danh sách lần chạy mới nhất trước; người không phải chủ nhận 403, lịch không tồn tại nhận 404; tham số biên và status ngoài enum được xử lý đúng.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| FR-042 | functional | P0 | test | llm/approved |

Phạm vi: Phủ hành vi chức năng và kiểm tra đầu vào của ba endpoint GET: tra cứu một lịch, danh sách lịch, danh sách runs. Không phủ lỗi xác thực 401 và truy cập danh sách lịch của accountId khác (thuộc TR security).

Tiêu chí đạt:

1. HTTP 200; body có status và nextRunAt khớp giá trị lưu trong ScheduledTransfer; lịch và lần chạy không bị thay đổi sau yêu cầu. (`TCND-SCHEDULED-TRANSFER-AC-FR-042-1-functional`)
2. HTTP 200; mọi phần tử thuộc accountId và đúng status lọc (nếu có); số phần tử <= limit; trang kế tiếp không trùng, không sót phần tử so với trang trước. (`TCND-SCHEDULED-TRANSFER-AC-FR-042-2-functional`)
3. HTTP 200; danh sách giảm dần theo scheduledAt; mỗi phần tử có đủ scheduledAt, status, transferId, reasonCode khớp ScheduledRun. (`TCND-SCHEDULED-TRANSFER-AC-FR-042-3-functional`)
4. Người không phải chủ nhận HTTP 403 mã FORBIDDEN; scheduleId không tồn tại nhận HTTP 404 mã SCHEDULE_NOT_FOUND; body lỗi không lộ dữ liệu lịch. (`TCND-SCHEDULED-TRANSFER-AC-FR-042-4-functional`)
5. limit=1, limit=100 và offset=0 trả HTTP 200; limit=0, limit=101, offset=-1 bị từ chối theo hợp đồng lỗi (mã cụ thể theo gaps). (`TCND-SCHEDULED-TRANSFER-FR-042-inputValidation-p433493`)
6. Danh sách lịch với status ngoài ACTIVE/PAUSED/COMPLETED/CANCELLED trả HTTP 400 với errorCode INVALID_REQUEST. (`TCND-SCHEDULED-TRANSFER-FR-042-inputValidation-p473406`)

#### TR-SCHEDULED-TRANSFER-L2-FR-042-security

Ba endpoint GET tra cứu lịch từ chối yêu cầu thiếu hoặc sai API key bằng 401 UNAUTHORIZED, và từ chối danh sách lịch của accountId khác tài khoản người gọi bằng 403 FORBIDDEN, không trả dữ liệu lịch.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| FR-042 | security | P0 | test | llm/approved |

Phạm vi: Chỉ phủ kiểm soát truy cập: xác thực API key (401) trên cả ba endpoint và 403 khi accountId khác người gọi ở danh sách lịch. Không phủ kết quả 200, phân trang, 404 hay kiểm tra đầu vào (thuộc TR functional).

Tiêu chí đạt:

1. HTTP 401 với errorCode UNAUTHORIZED cho cả ba endpoint khi thiếu hoặc sai API key; body không chứa dữ liệu lịch. (`TCND-SCHEDULED-TRANSFER-FR-042-access-p2aa8d6`)
2. Danh sách lịch với accountId khác tài khoản người gọi trả HTTP 403 với errorCode FORBIDDEN; không lộ lịch của tài khoản khác. (`TCND-SCHEDULED-TRANSFER-FR-042-access-p72e502`)

#### TR-SCHEDULED-TRANSFER-L2-FR-043-functional

Xác minh DELETE /v1/scheduled-transfers/{scheduleId} của chủ lịch: hủy lịch ACTIVE trả 200 và chuyển lịch cùng mọi lần chạy PENDING sang CANCELLED; gọi lặp trên lịch đã CANCELLED vẫn trả 200 không đổi dữ liệu; lịch COMPLETED hoặc có lần chạy RUNNING trả 409 SCHEDULE_NOT_CANCELLABLE; hủy thành công có bản ghi audit.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| FR-043 | functional | P0 | test | llm/approved |

Phạm vi: Phủ luồng chức năng hủy lịch (thành công, idempotent, không thể hủy) và ghi audit khi hủy thành công. Không phủ tranh chấp với Scheduler, lỗi 404 (TR reliability) và lỗi 401/403 (TR security).

Tiêu chí đạt:

1. HTTP 200; ScheduledTransfer.status = CANCELLED; mọi ScheduledRun PENDING của lịch có status = CANCELLED; lần chạy đã ở trạng thái cuối khác không đổi (đọc lại từ CSDL trước/sau). (`TCND-SCHEDULED-TRANSFER-AC-FR-043-1-functional`)
2. Lần gọi lặp trả HTTP 200, nội dung phản hồi tương đương lần hủy đầu; trạng thái lịch vẫn CANCELLED, không có thay đổi dữ liệu mới. (`TCND-SCHEDULED-TRANSFER-AC-FR-043-2-functional`)
3. Cả hai trường hợp (lịch COMPLETED; lịch có ít nhất một lần chạy RUNNING) trả HTTP 409, errorCode = SCHEDULE_NOT_CANCELLABLE; lịch và lần chạy giữ nguyên trạng thái trước khi gọi. (`TCND-SCHEDULED-TRANSFER-AC-FR-043-3-functional`)
4. Sau khi hủy thành công, bảng audit có bản ghi cho thao tác hủy lịch gắn với scheduleId. (`TCND-SCHEDULED-TRANSFER-FR-043-audit-p1438e9`)

#### TR-SCHEDULED-TRANSFER-L2-FR-043-reliability

Xác minh độ tin cậy khi hủy lịch: khi Scheduler chiếm lần chạy PENDING đồng thời với DELETE thì kết quả phải nhất quán (DELETE 200 và lần chạy bị hủy không thực thi, hoặc DELETE 409 do lần chạy đã RUNNING); DELETE với scheduleId không tồn tại trả 404 SCHEDULE_NOT_FOUND.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| FR-043 | reliability | P0 | test | llm/approved |

Phạm vi: Phủ tranh chấp Scheduler–DELETE và lỗi lịch không tồn tại. Không phủ luồng hủy thành công/idempotent/409 tĩnh (TR functional) và xác thực/phân quyền (TR security).

Tiêu chí đạt:

1. Qua nhiều lần lặp tranh chấp, không tồn tại lần chạy CANCELLED có giao dịch chuyển tiền được ghi sổ; DELETE trả 200 hoặc 409 tương ứng kết quả chiếm lần chạy. (`TCND-SCHEDULED-TRANSFER-FR-043-concurrency-p4b4873`)
2. HTTP 404, errorCode SCHEDULE_NOT_FOUND; không có thay đổi dữ liệu. (`TCND-SCHEDULED-TRANSFER-FR-043-errorHandling-pa62ef8`)

#### TR-SCHEDULED-TRANSFER-L2-FR-043-security

Xác minh kiểm soát truy cập khi hủy lịch: DELETE thiếu hoặc sai API key bị từ chối 401 UNAUTHORIZED; người gọi không phải chủ lịch bị từ chối 403 FORBIDDEN; cả hai trường hợp lịch và các lần chạy không bị thay đổi.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| FR-043 | security | P0 | test | llm/approved |

Phạm vi: Chỉ phủ xác thực (401) và phân quyền theo chủ lịch (403). Không phủ luồng hủy hợp lệ, audit (TR functional) và tranh chấp, 404 (TR reliability).

Tiêu chí đạt:

1. HTTP 401, errorCode UNAUTHORIZED khi thiếu hoặc sai API key; trạng thái lịch không đổi khi đọc lại. (`TCND-SCHEDULED-TRANSFER-FR-043-access-p4bf0be`)
2. HTTP 403, errorCode FORBIDDEN khi người gọi không phải chủ lịch; trạng thái lịch và các lần chạy không đổi. (`TCND-SCHEDULED-TRANSFER-FR-043-access-pb30d3e`)

#### TR-SCHEDULED-TRANSFER-L2-FR-044-functional

Kiểm PATCH /v1/scheduled-transfers/{scheduleId}: status=PAUSED trên lịch ACTIVE trả 200 và scheduler không chạy lịch; status=ACTIVE trên lịch PAUSED trả 200, các lần bỏ lỡ là SKIPPED, không chạy bù; chuyển trạng thái sai hoặc lịch ONCE trả 409; status thiếu/ngoài PAUSED/ACTIVE trả 400; thao tác thành công có audit.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| FR-044 | functional | P1 | test | llm/approved |

Phạm vi: Chỉ phủ hành vi chức năng, kiểm tra đầu vào và audit của tạm dừng/tiếp tục. Không phủ đồng thời với scheduler và 404 (TR reliability), cũng không phủ 401/403 (TR security).

Tiêu chí đạt:

1. HTTP 200, status=PAUSED trong phản hồi và bản ghi lịch; sau ít nhất 2 chu kỳ quét (60 giây) vượt thời điểm dự kiến, không có giao dịch hoặc lần chạy mới cho lịch. (`TCND-SCHEDULED-TRANSFER-AC-FR-044-1-functional`)
2. HTTP 200, status=ACTIVE; mọi lần bỏ lỡ trong lúc tạm dừng có status SKIPPED, không có giao dịch tương ứng, không chạy bù (FR-049); kiểm sau ít nhất 60 giây kể từ khi tiếp tục. (`TCND-SCHEDULED-TRANSFER-AC-FR-044-2-functional`)
3. HTTP 409, errorCode=SCHEDULE_STATE_INVALID; bản ghi lịch và scheduled_runs trước/sau PATCH giữ nguyên. (`TCND-SCHEDULED-TRANSFER-AC-FR-044-3-functional`)
4. Có đúng một bản ghi audit cho mỗi lần chuyển trạng thái thành công, tham chiếu scheduleId. (`TCND-SCHEDULED-TRANSFER-FR-044-audit-pafc81c`)
5. HTTP 400, errorCode=INVALID_REQUEST, trạng thái lịch không đổi. (`TCND-SCHEDULED-TRANSFER-FR-044-inputValidation-p847ccb`)

#### TR-SCHEDULED-TRANSFER-L2-FR-044-reliability

Kiểm khi PATCH PAUSED đến đúng lúc scheduler đang quét và chiếm lần chạy của lịch thì không phát sinh giao dịch trùng Idempotency-Key và lịch kết thúc ở PAUSED; PATCH với scheduleId không tồn tại trả 404.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| FR-044 | reliability | P1 | test | llm/approved |

Phạm vi: Chỉ phủ đồng thời giữa PATCH và scheduler, cùng lỗi 404 khi lịch không tồn tại. Không phủ luồng chức năng thông thường (TR functional) và kiểm soát truy cập (TR security).

Tiêu chí đạt:

1. Không có hai giao dịch cho cùng Idempotency-Key (mã lịch + thời điểm dự kiến); mỗi lần chạy chỉ được chiếm một lần; lịch cuối cùng ở PAUSED và không có lần chạy mới nào được khởi tạo. (`TCND-SCHEDULED-TRANSFER-FR-044-concurrency-p5afd1f`)
2. HTTP 404, errorCode=SCHEDULE_NOT_FOUND; không tạo hoặc đổi bản ghi nào. (`TCND-SCHEDULED-TRANSFER-FR-044-errorHandling-pb66379`)

#### TR-SCHEDULED-TRANSFER-L2-FR-044-security

Kiểm PATCH tạm dừng/tiếp tục lịch bị từ chối khi người gọi không phải chủ lịch (403 FORBIDDEN) hoặc thiếu/sai API key (401 UNAUTHORIZED), và lịch không bị thay đổi.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| FR-044 | security | P1 | test | llm/approved |

Phạm vi: Chỉ phủ xác thực (401) và phân quyền theo chủ lịch (403). Không phủ kết quả chức năng khi thành công, 400/404/409 hay đồng thời với scheduler (các TR khác).

Tiêu chí đạt:

1. HTTP 403, errorCode=FORBIDDEN, trạng thái lịch không đổi. (`TCND-SCHEDULED-TRANSFER-FR-044-access-p3b5af1`)
2. HTTP 401, errorCode=UNAUTHORIZED, trạng thái lịch không đổi. (`TCND-SCHEDULED-TRANSFER-FR-044-access-pf4b5f6`)

#### TR-SCHEDULED-TRANSFER-L2-FR-045-functional

Khi một lần chạy của lịch hợp lệ đến hạn, scheduler gọi lõi chuyển tiền dùng chung và tạo đúng 1 Transfer COMPLETED. Idempotency-Key dựng từ mã lịch + thời điểm dự kiến. Số dư, hạn mức ngày, trạng thái tài khoản, Fraud Check được đánh giá lúc chạy. Hạn mức đã dùng và lịch sử giao dịch được cập nhật.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| FR-045 | functional | P0 | test | llm/approved |

Phạm vi: Phủ 4 tiêu chí chấp nhận chức năng AC-FR-045-1..4 (tạo giao dịch, khóa idempotency, đánh giá tại thời điểm chạy, cập nhật hạn mức/lịch sử). Không phủ correlationId (TR maintainability), độ trễ bắt đầu 60 giây (TR performance) và chạy song song hai scheduler (TR reliability).

Tiêu chí đạt:

1. Sau một chu kỳ quét: số Transfer theo lần chạy = 1 và trạng thái COMPLETED; run.status = SUCCEEDED; run.transferId = id của Transfer đó; không có giao dịch thứ hai. (`TCND-SCHEDULED-TRANSFER-AC-FR-045-1-functional`)
2. Idempotency-Key lưu trên Transfer bằng đúng chuỗi ghép mã lịch và thời điểm dự kiến của run; hai lần gọi cùng cặp này trả về cùng một giao dịch, không tạo giao dịch mới. (`TCND-SCHEDULED-TRANSFER-AC-FR-045-2-functional`)
3. Quyết định chấp nhận/từ chối khớp số dư, hạn mức, trạng thái tài khoản và kết quả Fraud Check tại lúc chạy: đủ điều kiện lúc chạy thì thành công dù lúc tạo thì không; ngược lại không có Transfer COMPLETED; lời gọi Fraud Check xảy ra trong lúc chạy. (`TCND-SCHEDULED-TRANSFER-AC-FR-045-3-functional`)
4. Hạn mức đã dùng sau lần chạy = X + A (A là số tiền giao dịch); lịch sử giao dịch của tài khoản chứa Transfer của lần chạy với số tiền A; giao dịch kế tiếp trong ngày được kiểm hạn mức với phần đã dùng đã gồm A. (`TCND-SCHEDULED-TRANSFER-AC-FR-045-4-functional`)

#### TR-SCHEDULED-TRANSFER-L2-FR-045-maintainability

Mỗi lần chạy của lịch có một correlationId riêng, được ghi nhất quán trên Transfer, bản ghi audit và request gửi tới Fraud Check, với tỷ lệ 100% số lần chạy trong đợt kiểm thử.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| FR-045 | maintainability | P0 | test | llm/approved |

Phạm vi: Chỉ phủ khả năng truy vết bằng correlationId. Không phủ kết quả nghiệp vụ của giao dịch (TR functional), độ trễ bắt đầu (TR performance) hay xử lý song song hai scheduler (TR reliability).

Tiêu chí đạt:

1. 100% lần chạy trong đợt kiểm thử có correlationId riêng (không trùng giữa các lần chạy) và giá trị khớp ở Transfer, bản ghi audit và request tới mock Fraud Check. (`TCND-SCHEDULED-TRANSFER-FR-045-observability-pd8cb5a`)

#### TR-SCHEDULED-TRANSFER-L2-FR-045-performance

Lần chạy đến hạn phải bắt đầu trong tối đa 60 giây kể từ thời điểm dự kiến, đạt với 99% số lần chạy của một đợt kiểm thử (phân vị 99 ≤ 60 giây).

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| FR-045 | performance | P0 | test | llm/approved |

Phạm vi: Chỉ phủ độ trễ bắt đầu lần chạy. Không phủ tính đúng của giao dịch (TR functional), correlationId (TR maintainability) hay tranh chấp giữa hai scheduler (TR reliability).

Tiêu chí đạt:

1. Phân vị 99 của (thời điểm bắt đầu − thời điểm dự kiến), theo giờ hệ thống trên tập lần chạy của đợt kiểm thử, ≤ 60 giây. (`TCND-SCHEDULED-TRANSFER-FR-045-timing-pb0682f`)

#### TR-SCHEDULED-TRANSFER-L2-FR-045-reliability

Khi hai scheduler cùng quét một lần chạy đến hạn trên cùng CSDL, chỉ một bên chiếm được lần chạy bằng UPDATE có điều kiện và tạo giao dịch; bên còn lại không thực thi. Số giao dịch trùng bằng 0.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| FR-045 | reliability | P0 | test | llm/approved |

Phạm vi: Chỉ phủ tranh chấp đồng thời khi chiếm lần chạy. Không phủ nội dung giao dịch và khóa idempotency (TR functional), correlationId (TR maintainability) hay độ trễ bắt đầu (TR performance).

Tiêu chí đạt:

1. Chạy song song 2 scheduler trên cùng CSDL, lặp nhiều lần: số Transfer cho mỗi lần chạy = 1, số giao dịch trùng = 0; chỉ một bên chiếm được lần chạy. (`TCND-SCHEDULED-TRANSFER-FR-045-concurrency-p174b2f`)

#### TR-SCHEDULED-TRANSFER-L2-FR-046-functional

Kiểm tra scheduler xử lý lần chạy thất bại theo loại lỗi: FRAUD_CHECK_UNAVAILABLE thử lại 3 lần cách nhau 5 phút rồi FAILED; INSUFFICIENT_FUNDS thử lại 2 lần cách nhau 1 giờ rồi FAILED; ACCOUNT_INACTIVE, FRAUD_REJECTED, LIMIT_EXCEEDED FAILED ngay, không thử lại; 3 run FAILED liên tiếp thì lịch chuyển PAUSED.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| FR-046 | functional | P0 | test | llm/approved |

Phạm vi: Chỉ phủ chức năng mức L2 của FR-046: số lần và khoảng cách thử lại, trạng thái run, reasonCode, chuyển lịch sang PAUSED. Không phủ cơ chế lập lịch/quét chu kỳ nói chung, logic nội bộ của lõi chuyển tiền hay kiểm tra gian lận.

Tiêu chí đạt:

1. Số lần thử lại = 3, khoảng cách 5 phút; nếu một lần thử lại thành công thì run COMPLETED và dừng. Sau lần lỗi cuối run = FAILED, reasonCode = FRAUD_CHECK_UNAVAILABLE, không có lần thử thứ 4. (`TCND-SCHEDULED-TRANSFER-AC-FR-046-1-functional`)
2. Số lần thử lại = 2, khoảng cách 1 giờ; nếu một lần thử lại đủ số dư thì run COMPLETED. Sau lần lỗi cuối run = FAILED, không có lần thử thứ 3. (`TCND-SCHEDULED-TRANSFER-AC-FR-046-2-functional`)
3. Với mỗi mã ACCOUNT_INACTIVE, FRAUD_REJECTED, LIMIT_EXCEEDED: run = FAILED ngay với reasonCode tương ứng, số lần thử lại = 0, không có lần thử mới ở ít nhất 1 chu kỳ quét sau đó. (`TCND-SCHEDULED-TRANSFER-AC-FR-046-3-functional`)
4. Lịch = PAUSED ngay sau lần FAILED thứ 3 liên tiếp và không sinh run mới; lịch vẫn ACTIVE khi chuỗi FAILED liên tiếp < 3 (gồm ca 2 FAILED và ca có 1 run thành công xen giữa). (`TCND-SCHEDULED-TRANSFER-AC-FR-046-4-functional`)

#### TR-SCHEDULED-TRANSFER-L2-FR-047-functional

Kiểm trên ENV-Demo (SQLite) rằng mỗi lần chạy lịch kết thúc thành công ghi đúng 1 sự kiện ScheduledRunCompleted vào outbox với payload.transferId khớp giao dịch đã ghi sổ; mỗi lần chạy FAILED ghi đúng 1 sự kiện ScheduledRunFailed với payload.reasonCode khớp mã lý do lưu trên lần chạy; người gửi nhận SMS (sms_log) sau khi consumer nhận sự kiện, và lỗi SMS Gateway không đổi trạng thái lần chạy.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| FR-047 | functional | P2 | test | llm/approved |

Phạm vi: Phủ phát sự kiện vào outbox (thành công, FAILED) và gửi SMS kèm hành vi khi SMS lỗi. KHÔNG phủ việc khử trùng khi sự kiện bị giao lại từ queue (thuộc TR độ tin cậy FR-047).

Tiêu chí đạt:

1. Outbox có đúng 1 sự kiện ScheduledRunCompleted cho mỗi lần chạy thành công; payload.transferId khác rỗng và bằng transferId của giao dịch; bản ghi outbox commit cùng giao dịch CSDL và được relay lên queue. (`TCND-SCHEDULED-TRANSFER-AC-FR-047-1-functional`)
2. Outbox có đúng 1 sự kiện ScheduledRunFailed cho mỗi lần chạy FAILED; payload.reasonCode khác rỗng và bằng mã lý do lưu trên lần chạy; không có ScheduledRunCompleted cho lần chạy đó. (`TCND-SCHEDULED-TRANSFER-AC-FR-047-2-functional`)
3. Trường hợp bình thường: sms_log có bản ghi gửi người gửi với nội dung từ mẫu kết quả lần chạy. Khi tiêm lỗi SMS Gateway: trạng thái lần chạy trước và sau giống nhau và không tạo giao dịch bù. (`TCND-SCHEDULED-TRANSFER-AC-FR-047-3-functional`)

#### TR-SCHEDULED-TRANSFER-L2-FR-047-reliability

Kiểm trên ENV-Demo rằng khi cùng một sự kiện kết quả lần chạy bị giao lại từ queue, consumer khử trùng và số bản ghi sms_log cho lần chạy và loại sự kiện đó vẫn bằng 1.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| FR-047 | reliability | P2 | test | llm/approved |

Phạm vi: Chỉ phủ tính idempotent của consumer khi sự kiện được giao lại. KHÔNG phủ việc phát sự kiện vào outbox, nội dung payload hay hành vi khi SMS Gateway lỗi (thuộc TR chức năng FR-047).

Tiêu chí đạt:

1. Sau khi giao lại sự kiện, đếm sms_log cho một lần chạy và một loại sự kiện bằng đúng 1; không có SMS thứ hai cho người gửi. (`TCND-SCHEDULED-TRANSFER-FR-047-idempotency-pd600b3`)

#### TR-SCHEDULED-TRANSFER-L2-FR-048-functional

Kiểm tra AuditLog ghi đúng: (1) mỗi thao tác tạo, hủy, tạm dừng, tiếp tục lịch thành công sinh đúng 1 bản ghi với 5 thuộc tính; (2) mỗi lần chạy đến hạn, mọi kết quả, có bản ghi actor = SCHEDULER; (3) yêu cầu bị từ chối không sinh audit; (4) bản ghi audit không bị sửa hay xóa.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| FR-048 | functional | P1 | test | llm/approved |

Phạm vi: Phủ nội dung và thời điểm ghi audit cho thao tác lịch, lần chạy, yêu cầu bị từ chối và tính bất biến của AuditLog. KHÔNG phủ tính nhất quán correlationId giữa audit, giao dịch và Fraud Check (thuộc TR maintainability).

Tiêu chí đạt:

1. Sau 4 thao tác thành công (tạo/hủy/tạm dừng/tiếp tục), AuditLog có đúng 4 bản ghi mới cho mã lịch; mỗi bản ghi có correlationId không rỗng, thời điểm, tác nhân = mã tài khoản người gọi (không phải SCHEDULER), loại sự kiện phân biệt, mã lịch đúng. (`TCND-SCHEDULED-TRANSFER-AC-FR-048-1-functional`)
2. Với mỗi lần chạy đã có kết quả cuối, AuditLog có bản ghi actor = SCHEDULER và mã lịch đúng; số bản ghi bằng số lần chạy đã xử lý (kịch bản TC-S07). (`TCND-SCHEDULED-TRANSFER-AC-FR-048-2-functional`)
3. Yêu cầu bị từ chối trả 403 FORBIDDEN hoặc 409 SCHEDULE_STATE_INVALID (và các mã 401, 404, 400, 422 tương ứng); số bản ghi audit sự kiện lịch theo mã lịch không tăng và trạng thái lịch không đổi. (`TCND-SCHEDULED-TRANSFER-FR-048-guardViolation-pf87b1a`)
4. Sau các thao tác tiếp theo, băm nội dung mọi bản ghi audit cũ không đổi ở mọi trường; số bản ghi chỉ tăng, không giảm. (`TCND-SCHEDULED-TRANSFER-FR-048-invariant-pdabb36`)

#### TR-SCHEDULED-TRANSFER-L2-FR-048-maintainability

Kiểm tra mỗi lần chạy lịch có correlationId riêng, và giá trị đó xuất hiện nhất quán trong bản ghi audit, giao dịch và lời gọi Fraud Check; đạt 100% số lần chạy trong lượt kiểm thử, các lần chạy khác nhau có correlationId khác nhau.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| FR-048 | maintainability | P1 | test | llm/approved |

Phạm vi: Chỉ phủ khả năng truy vết bằng correlationId của lần chạy giữa audit, giao dịch và log Fraud Check. KHÔNG phủ việc có ghi audit hay không, tác nhân, hay tính bất biến (thuộc TR functional).

Tiêu chí đạt:

1. Trên toàn bộ lần chạy trong lượt kiểm thử, 100% lần chạy có correlationId riêng trong AuditLog khớp với giao dịch và lời gọi Fraud Check; correlationId khác nhau giữa các lần chạy. (`TCND-SCHEDULED-TRANSFER-FR-048-observability-p477475`)

#### TR-SCHEDULED-TRANSFER-L2-FR-049-functional

Sau khi Scheduler khởi động lại, lần chạy quá hạn ≤ 24 giờ so với thời điểm dự kiến phải được chạy bù đúng 1 lần qua lõi chuyển tiền với Idempotency-Key cố định theo mã lịch + thời điểm dự kiến; lần chạy quá hạn > 24 giờ phải chuyển MISSED, phát ScheduledRunFailed (reasonCode MISSED) qua outbox và không tạo giao dịch.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| FR-049 | functional | P1 | test | llm/approved |

Phạm vi: Phủ hành vi chức năng của chạy bù (≤ 24 giờ) và đánh dấu MISSED (> 24 giờ). Không phủ tranh chấp chiếm lần chạy giữa hai Scheduler đồng thời (thuộc TR độ tin cậy cùng FR-049).

Tiêu chí đạt:

1. Sau tối thiểu 6 chu kỳ quét kể từ lúc khởi động lại: đúng 1 giao dịch theo Idempotency-Key của lần chạy bù; lần chạy không còn ở trạng thái chờ; quét thêm không tạo giao dịch mới. (`TCND-SCHEDULED-TRANSFER-AC-FR-049-1-functional`)
2. Sau chu kỳ quét: trạng thái lần chạy = MISSED; có 1 sự kiện ScheduledRunFailed reasonCode MISSED trong outbox; 0 giao dịch theo Idempotency-Key của lần chạy; số dư tài khoản nguồn và đích không đổi. (`TCND-SCHEDULED-TRANSFER-AC-FR-049-2-functional`)

#### TR-SCHEDULED-TRANSFER-L2-FR-049-reliability

Khi hai Scheduler cùng quét lại sau thời gian ngừng, một lần chạy quá hạn trong 24 giờ chỉ được một Scheduler chiếm thành công bằng UPDATE có điều kiện; kết quả là đúng 1 giao dịch cho lần chạy, không có giao dịch trùng.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| FR-049 | reliability | P1 | test | llm/approved |

Phạm vi: Chỉ phủ tính đúng đắn khi hai Scheduler quét đồng thời (chiếm lần chạy, không trùng giao dịch). Không phủ logic chạy bù đơn lẻ và đánh dấu MISSED (thuộc TR chức năng cùng FR-049).

Tiêu chí đạt:

1. Sau nhiều vòng quét đồng thời lặp lại (mỗi vòng cả hai Scheduler quét cùng lúc), đếm theo Idempotency-Key: 0 giao dịch trùng và đúng 1 giao dịch cho lần chạy. (`TCND-SCHEDULED-TRANSFER-FR-049-concurrency-pe3c163`)

#### TR-SCHEDULED-TRANSFER-L2-NFR-010-performance

Xác nhận trên đồng hồ giả (kịch bản TC-S09) rằng ít nhất 99% lần chạy lịch chuyển tiền có độ trễ bắt đầu (thời điểm chiếm lần chạy trừ thời điểm dự kiến) <= 60 s, gồm cả khi chu kỳ quét 30 s với lần chạy phân bố đều và khi quá tải 40 lần chạy đồng thời với giới hạn 20 lần chạy mỗi chu kỳ.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| NFR-010 | performance | P0 | test | llm/approved |

Phạm vi: Chỉ phủ chỉ tiêu đúng giờ (độ trễ bắt đầu, tỷ lệ >= 99% trong 60 s) ở mức L2 bằng đồng hồ giả. Không phủ tính đúng đắn của nghiệp vụ chuyển tiền, kết quả giao dịch hay đo bằng thời gian thực.

Tiêu chí đạt:

1. Trên toàn bộ mẫu lần chạy với chu kỳ quét 30 s, số lần chạy có độ trễ bắt đầu <= 60 s chia tổng số lần chạy >= 0,99; báo cáo tỷ lệ đạt và độ trễ lớn nhất. (`TCND-SCHEDULED-TRANSFER-AC-NFR-010-1-timing`)
2. Mỗi chu kỳ chọn tối đa 20 lần chạy; khi phát 40 lần chạy đồng thời, tỷ lệ lần chạy có độ trễ <= 60 s vẫn >= 99%, trường hợp xấu nhất tính theo lần chạy cuối cùng; lần chạy hoãn nhiều chu kỳ được tính vào phần 1% được phép trễ. (`TCND-SCHEDULED-TRANSFER-NFR-010-timing-p9746f5`)
3. Trong cửa sổ đo trên đồng hồ giả, ít nhất 99% lần chạy có (thời điểm bắt đầu - thời điểm dự kiến) <= 60 s, tính cả lần chạy đến hạn ngay sau một lần quét; mốc thời gian lấy theo thời điểm chiếm lần chạy. (`TCND-SCHEDULED-TRANSFER-NFR-010-timing-target`)

#### TR-SCHEDULED-TRANSFER-L2-NFR-011-reliability

Kiểm ở mức L2 rằng mỗi lần chạy lịch chuyển tiền đến hạn tạo tối đa 1 giao dịch (0 giao dịch trùng) khi có từ 2 scheduler quét đồng thời và khi scheduler khởi động lại giữa chừng, nhờ UPDATE có điều kiện để chiếm lần chạy và Idempotency-Key cố định (mã lịch + thời điểm dự kiến).

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| NFR-011 | reliability | P1 | test | llm/approved |

Phạm vi: Phủ độ tin cậy dạng đồng thời (concurrency) của NFR-011 ở L2: hai scheduler cùng chiếm lần chạy, khởi động lại sau khi chiếm, và chỉ tiêu 0 giao dịch trùng. Không phủ tính đúng đắn của lõi chuyển tiền, số tiền, hay các chỉ tiêu khác của lịch chuyển tiền.

Tiêu chí đạt:

1. Sau khi cả hai scheduler hoàn tất chu kỳ quét (TC-S10), đếm theo Idempotency-Key của lần chạy được đúng 1 giao dịch; chỉ 1 scheduler chiếm được (UPDATE ảnh hưởng 1 dòng), scheduler kia bỏ qua; không có giao dịch thứ hai. (`TCND-SCHEDULED-TRANSFER-AC-NFR-011-1-concurrency`)
2. Sau khi khởi động lại và quét hết các chu kỳ cần thiết (TC-S11), số giao dịch theo Idempotency-Key của lần chạy <= 1; không phát sinh giao dịch thứ hai; lần chạy được hoàn tất hoặc đi theo luồng thử lại/chạy bù. (`TCND-SCHEDULED-TRANSFER-AC-NFR-011-2-concurrency`)
3. Với mọi lần chạy trong lần kiểm thử có >= 2 scheduler quét đồng thời (TC-S10, TC-S11), số giao dịch trùng = 0 giao dịch trùng, tức mỗi lần chạy có <= 1 giao dịch theo Idempotency-Key. (`TCND-SCHEDULED-TRANSFER-NFR-011-concurrency-target`)

#### TR-SCHEDULED-TRANSFER-L2-NFR-012-performance

Kiểm tra phân loại lần chạy quá hạn quanh biên 24 giờ: lần quá hạn đúng 24 giờ và các lần ≤ 24 giờ được chạy bù; lần quá hạn vượt 24 giờ chuyển MISSED và không sinh giao dịch.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| NFR-012 | performance | P1 | test | llm/approved |

Phạm vi: Chỉ phủ quy tắc phân loại theo biên thời gian 24 giờ (chạy bù hay MISSED). Không phủ số chu kỳ quét cần để xử lý hết tập quá hạn, và không phủ chống giao dịch trùng giữa các scheduler; các phần đó thuộc TR reliability.

Tiêu chí đạt:

1. Lần quá hạn ≤ 24 giờ (gồm đúng 24 giờ) được chạy bù; lần quá hạn > 24 giờ có trạng thái MISSED và không sinh giao dịch nào. (`TCND-SCHEDULED-TRANSFER-NFR-012-timing-pe0788e`)

#### TR-SCHEDULED-TRANSFER-L2-NFR-012-reliability

Kiểm tra sau khởi động lại Scheduler: mọi lần quá hạn ≤ 24 giờ (tải tối đa 100 lần, mỗi chu kỳ xử lý tối đa 20 lần) được xử lý hết trong 5 chu kỳ quét đầu (chu kỳ 30 giây, khoảng 150 giây); hai scheduler cùng quét không tạo giao dịch trùng.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| NFR-012 | reliability | P1 | test | llm/approved |

Phạm vi: Phủ thời gian phục hồi theo số chu kỳ quét và tính duy nhất của giao dịch khi chiếm lần chạy đồng thời. Không phủ việc phân loại tại biên 24 giờ như một quy tắc riêng; phần đó thuộc TR performance.

Tiêu chí đạt:

1. Sau chu kỳ quét thứ 5, số lần quá hạn ≤ 24 giờ chưa xử lý bằng 0; mỗi chu kỳ xử lý tối đa 20 lần; kết quả cuối là COMPLETED hoặc FAILED. (`TCND-SCHEDULED-TRANSFER-AC-NFR-012-1-resilience`)
2. Mỗi lần chạy quá hạn chỉ được một scheduler chiếm; đếm theo Idempotency-Key (mã lịch + thời điểm dự kiến) cho 0 giao dịch trùng mỗi lần chạy. (`TCND-SCHEDULED-TRANSFER-NFR-012-concurrency-p92271d`)
3. Với tải xấu nhất 100 lần quá hạn, số chu kỳ quét cần để xử lý hết ≤ 5 tính từ lần quét đầu sau khởi động; lần quá hạn > 24 giờ có trạng thái MISSED. (`TCND-SCHEDULED-TRANSFER-NFR-012-resilience-target`)

#### TR-SCHEDULED-TRANSFER-L2-NFR-013-performance

Với tải 100 lần chạy chuyển tiền định kỳ đến hạn cùng lúc (benchmark TC-S13), p95 thời gian phản hồi chuyển tiền qua API <= 500 ms, mỗi chu kỳ quét 30 giây chiếm tối đa 20 lần chạy và tốc độ gọi lõi chuyển tiền từ scheduler không quá 10 rps.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| NFR-013 | performance | P1 | benchmark | llm/approved |

Phạm vi: Phủ phần hiệu năng L2: p95 thời gian phản hồi, giới hạn 20 lần chạy mỗi chu kỳ quét và rate limit 10 rps. KHÔNG phủ tính duy nhất của giao dịch (không trùng), thuộc TR độ tin cậy cùng chủ thể.

Tiêu chí đạt:

1. p95 trên toàn bộ 100 lần chạy của một đợt <= 500 ms, tính từ lúc gửi yêu cầu đến khi nhận phản hồi; lặp nhiều đợt, lấy đợt xấu nhất; không lần chạy nào bị bỏ sót. (`TCND-SCHEDULED-TRANSFER-AC-NFR-013-1-performance`)
2. Trong mọi chu kỳ quét 30 giây của đợt 100 lần chạy, số lần chạy được chiếm (UPDATE có điều kiện thành công) <= 20; phần còn lại để cho chu kỳ kế tiếp. (`TCND-SCHEDULED-TRANSFER-AC-NFR-013-2-performance`)
3. Theo cửa sổ 1 giây trượt trong toàn đợt, cửa sổ lớn nhất có tốc độ yêu cầu từ scheduler <= 10 rps. (`TCND-SCHEDULED-TRANSFER-NFR-013-performance-pd5ac38`)
4. p95 chuyển tiền qua API <= 500 ms trên 100 lần chạy của đợt xấu nhất khi lặp nhiều đợt; ghi rõ môi trường và tải đo; giới hạn 20 lần/chu kỳ và 10 rps vẫn được giữ. (`TCND-SCHEDULED-TRANSFER-NFR-013-performance-target`)

#### TR-SCHEDULED-TRANSFER-L2-NFR-013-reliability

Khi 100 lần chạy đến hạn cùng lúc, mỗi lần chạy sinh đúng một giao dịch; số giao dịch trùng cho cùng một lần chạy bằng 0, đối chiếu theo Idempotency-Key sau đợt benchmark.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| NFR-013 | reliability | P1 | test | llm/approved |

Phạm vi: Chỉ phủ tính không trùng giao dịch dưới đồng thời ở mức L2. KHÔNG phủ p95 thời gian phản hồi, giới hạn 20 lần/chu kỳ và rate limit 10 rps, thuộc TR hiệu năng cùng chủ thể.

Tiêu chí đạt:

1. Sau đợt benchmark, đối chiếu số giao dịch với số lần chạy theo Idempotency-Key: mỗi lần chạy có đúng 1 giao dịch, số giao dịch trùng = 0. (`TCND-SCHEDULED-TRANSFER-NFR-013-concurrency-p4dc0d8`)

#### TR-SCHEDULED-TRANSFER-L2-NFR-014-maintainability

Kiểm rằng mỗi lần chạy lệnh chuyển tiền định kỳ (kể cả tối đa 20 lần chạy trong cùng chu kỳ quét, thành công, thử lại hoặc FAILED) có correlationId riêng, khác rỗng, và id này xuất hiện nhất quán ở giao dịch, audit (tác nhân SCHEDULER), lời gọi Fraud Check (INT-01) và sự kiện outbox; tỷ lệ truy vết đầy đủ đạt 100%.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| NFR-014 | maintainability | P1 | test | llm/approved |

Phạm vi: Phủ tính duy nhất của correlationId, việc truyền xuyên suốt qua bốn nơi, tính nhất quán khi thử lại và chỉ tiêu 100% ở mức L2 (kịch bản SCN-ScheduledRun, TC-S12). Không phủ hiệu năng, logic nghiệp vụ chuyển tiền hay nội dung xử lý của Fraud Check ngoài việc ghi nhận correlationId.

Tiêu chí đạt:

1. Với N lần chạy (tối đa 20 trong cùng chu kỳ quét), COUNT(DISTINCT correlationId) = N và mọi giá trị khác rỗng, khác null. (`TCND-SCHEDULED-TRANSFER-AC-NFR-014-1-observability`)
2. Với mỗi lần chạy, correlationId trong giao dịch, audit, request tại mock Fraud Check và payload sự kiện ScheduledRunCompleted/ScheduledRunFailed trong outbox bằng đúng correlationId của lần chạy đó, không lẫn với lần chạy khác. (`TCND-SCHEDULED-TRANSFER-AC-NFR-014-2-observability`)
3. Sau khi thử lại, mọi bản ghi và lời gọi Fraud Check của lần chạy đều có correlationId khác rỗng, nhất quán theo quy tắc đã định, không bị mất. (`TCND-SCHEDULED-TRANSFER-NFR-014-observability-p8ac829`)
4. Tỷ lệ lần chạy có correlationId riêng và xuất hiện đủ ở giao dịch, audit, lời gọi Fraud Check = 100% (targetValue 100, đơn vị %), đo trên toàn bộ lần chạy, không lấy mẫu. (`TCND-SCHEDULED-TRANSFER-NFR-014-observability-target`)

#### TR-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-functional

Lần chạy quá hạn sau khi hệ thống ngừng: quá hạn ≤ 24 giờ thì chạy bù đúng một lần trong 5 chu kỳ quét đầu, > 24 giờ thì MISSED. Sau 3 run FAILED liên tiếp lịch chuyển ACTIVE → PAUSED; ACCOUNT_INACTIVE, FRAUD_REJECTED, LIMIT_EXCEEDED làm run FAILED ngay, không thử lại.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| SCN-ScheduledRun | functional | P1 | test | llm/approved |

Phạm vi: Chỉ phủ chạy bù/MISSED và chuyển trạng thái lịch sang PAUSED ở L2. Không phủ luồng chính thành công (TR L1), lỗi Fraud Check do sự cố INT-01 (TR L1 độ tin cậy), đồng thời hay lỗi SMS (TR L2 độ tin cậy).

Tiêu chí đạt:

1. Quá hạn ≤ 24 giờ (kể cả đúng 24 giờ theo đồng hồ giả): 1 giao dịch, xử lý trong 5 chu kỳ quét 30 giây đầu. Quá hạn > 24 giờ: MISSED, 0 giao dịch, phát ScheduledRunFailed reasonCode MISSED. (`TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-stateTransition-pd98b3f`)
2. Lịch ACTIVE → PAUSED sau 3 run FAILED liên tiếp; run SUCCEEDED xen giữa thì đếm lại từ đầu (chờ xác nhận). Lỗi không retry được: run FAILED ngay và phát ScheduledRunFailed kèm reasonCode. (`TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-stateTransition-pfec2a2`)

#### TR-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-maintainability

Mỗi lần chạy có một correlationId riêng, không trùng giữa các lần chạy, và correlationId đó xuất hiện đủ ở giao dịch, audit, lời gọi Fraud Check và sự kiện.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| SCN-ScheduledRun | maintainability | P1 | test | llm/approved |

Phạm vi: Chỉ phủ tính duy nhất và khả năng truy vết correlationId ở L2. Không phủ kết quả nghiệp vụ của luồng chính (TR L1 chức năng), tải đồng thời hay chuyển trạng thái lịch.

Tiêu chí đạt:

1. 100% lần chạy có correlationId duy nhất: không có hai lần chạy chung correlationId, và correlationId xuất hiện ở cả 4 nơi (giao dịch, audit, Fraud Check, sự kiện). (`TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-observability-p12d52f`)

#### TR-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-performance

Khi 100 lần chạy đến hạn cùng lúc, mỗi chu kỳ quét xử lý tối đa 20 lần chạy và p95 thời gian chuyển tiền qua API trên toàn đợt 100 lần chạy ≤ 500 ms.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| SCN-ScheduledRun | performance | P1 | benchmark | llm/approved |

Phạm vi: Chỉ phủ giới hạn công suất mỗi chu kỳ và p95 dưới tải 100 lần chạy ở L2. Không phủ ngân sách 60000 ms trường hợp xấu nhất của một lần xử lý (TR L1 hiệu năng) hay tính đúng đắn nghiệp vụ.

Tiêu chí đạt:

1. p95 thời gian chuyển tiền qua API ≤ 500 ms trong đợt 100 lần chạy đến hạn cùng lúc; số lần chạy mỗi chu kỳ quét ≤ 20. (`TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-performance-pa101d9`)

#### TR-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-reliability

Lần chạy giữ tính toàn vẹn khi tiến trình khởi động lại giữa lúc chiếm và lúc chuyển tiền, khi hai scheduler cùng chiếm một lần chạy, và khi gửi SMS kết quả lỗi.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| SCN-ScheduledRun | reliability | P1 | test | llm/approved |

Phạm vi: Chỉ phủ khởi động lại, đồng thời giữa hai scheduler và lỗi SMS ở L2. Không phủ lỗi Fraud Check (TR L1 độ tin cậy), chạy bù/PAUSED (TR L2 chức năng) hay hiệu năng.

Tiêu chí đạt:

1. Sau khi xử lý lại lần chạy bị gián đoạn, có tối đa một giao dịch; Idempotency-Key = mã lịch + thời điểm dự kiến chặn giao dịch thứ hai. (`TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-atomicity-p12901f`)
2. Với hai scheduler cùng chiếm, đúng một UPDATE có điều kiện thành công và số giao dịch trùng cho lần chạy = 0. (`TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-concurrency-p775a37`)
3. Sau lỗi gửi SMS, trạng thái run giữ nguyên (SUCCEEDED hoặc FAILED) và sự kiện vẫn nằm trong outbox. (`TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-errorHandling-p00fe49`)

#### TR-SCHEDULED-TRANSFER-L2-SM-Schedule-functional

Kiểm các chuyển trạng thái và guard của vòng đời lịch chuyển tiền: tạm dừng thủ công, tiếp tục, tự tạm dừng sau 3 lần FAILED liên tiếp, hoàn tất (ONCE/maxRuns/endDate), hủy khi ACTIVE hoặc PAUSED, và nhánh COMPLETED khi tiếp tục mà hết lần chạy. Mỗi chuyển trạng thái có một ca guard sai tương ứng.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| SM-Schedule | functional | P1 | test | llm/approved |

Phạm vi: Phủ 14 điều kiện chức năng (7 chuyển trạng thái, mỗi chuyển 1 stateTransition + 1 guardViolation) của SM-Schedule. KHÔNG phủ xung đột đồng thời giữa PATCH/DELETE và scheduler (thuộc TR reliability). Mã lỗi khi bị từ chối chưa xác định trong nguồn nên không kiểm mã lỗi cụ thể.

Tiêu chí đạt:

1. PATCH status=PAUSED trên lịch ACTIVE không định kỳ bị từ chối; trạng thái vẫn ACTIVE; không có audit SCHEDULE_PAUSED mới; các lần chạy không đổi. Mã lỗi chưa xác định trong nguồn. (`TCND-SCHEDULED-TRANSFER-TRN-Schedule-1-guardViolation-guard`)
2. ACTIVE → PAUSED; có đúng 1 audit SCHEDULE_PAUSED; lần quét 30 giây kế tiếp không chiếm lần chạy nào của lịch này. (`TCND-SCHEDULED-TRANSFER-TRN-Schedule-1-stateTransition`)
3. Khi không còn lần chạy theo BR-11, lịch không sang ACTIVE theo nhánh TRN-Schedule-2: không có audit SCHEDULE_RESUMED, không có lần chạy mới; kết quả cuối xem TRN-Schedule-7. (`TCND-SCHEDULED-TRANSFER-TRN-Schedule-2-guardViolation-guard`)
4. PAUSED → ACTIVE; các lần bỏ lỡ = SKIPPED; có 1 lần chạy kế tiếp mới; consecutive_failures = 0; có audit SCHEDULE_RESUMED. (`TCND-SCHEDULED-TRANSFER-TRN-Schedule-2-stateTransition`)
5. Lần FAILED thứ 3 liên tiếp của lịch không định kỳ: trạng thái vẫn ACTIVE, không có audit SCHEDULE_AUTO_PAUSED. (`TCND-SCHEDULED-TRANSFER-TRN-Schedule-3-guardViolation-guard`)
6. ACTIVE → PAUSED; audit SCHEDULE_AUTO_PAUSED có actor=SCHEDULER; không có lần chạy PENDING mới được tạo. (`TCND-SCHEDULED-TRANSFER-TRN-Schedule-3-stateTransition`)
7. Khi chưa thỏa ONCE xong, maxRuns hay endDate: trạng thái vẫn ACTIVE, không chuyển COMPLETED; vẫn sinh/giữ lần chạy kế tiếp theo lịch. (`TCND-SCHEDULED-TRANSFER-TRN-Schedule-4-guardViolation-guard`)
8. ACTIVE → COMPLETED khi ONCE xong, đạt maxRuns hoặc đạt endDate; không có lần chạy mới nào được tạo. (`TCND-SCHEDULED-TRANSFER-TRN-Schedule-4-stateTransition`)
9. DELETE lịch ACTIVE có lần RUNNING bị từ chối; trạng thái vẫn ACTIVE; lần PENDING giữ nguyên; không có audit SCHEDULE_CANCELLED. Mã lỗi chưa xác định trong nguồn. (`TCND-SCHEDULED-TRANSFER-TRN-Schedule-5-guardViolation-guard`)
10. ACTIVE → CANCELLED; không còn lần PENDING; có đúng 1 audit SCHEDULE_CANCELLED. (`TCND-SCHEDULED-TRANSFER-TRN-Schedule-5-stateTransition`)
11. DELETE lịch PAUSED có lần RUNNING bị từ chối; trạng thái vẫn PAUSED; lần PENDING không đổi; không có audit SCHEDULE_CANCELLED. Mã lỗi chưa xác định trong nguồn. (`TCND-SCHEDULED-TRANSFER-TRN-Schedule-6-guardViolation-guard`)
12. PAUSED → CANCELLED; không còn lần PENDING; có đúng 1 audit SCHEDULE_CANCELLED. (`TCND-SCHEDULED-TRANSFER-TRN-Schedule-6-stateTransition`)
13. Sau khi đánh dấu SKIPPED vẫn còn lần chạy theo BR-11: không chuyển COMPLETED; trạng thái cuối do TRN-Schedule-2 quyết định (ACTIVE). Nguồn mẫu 'giữ PAUSED' mâu thuẫn, cần làm rõ. (`TCND-SCHEDULED-TRANSFER-TRN-Schedule-7-guardViolation-guard`)
14. PAUSED → COMPLETED; các lần còn lại = SKIPPED; không có lần chạy mới. (`TCND-SCHEDULED-TRANSFER-TRN-Schedule-7-stateTransition`)

#### TR-SCHEDULED-TRANSFER-L2-SM-Schedule-reliability

Kiểm tính nhất quán khi PATCH status=PAUSED hoặc DELETE chạy đồng thời với scheduler đang chiếm lần chạy đến hạn bằng UPDATE có điều kiện: mỗi lần chạy chỉ được chiếm một lần và không có lần chạy nào của lịch đã PAUSED/CANCELLED.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| SM-Schedule | reliability | P1 | test | llm/approved |

Phạm vi: Chỉ phủ điều kiện đồng thời (1 điều kiện concurrency) giữa PATCH/DELETE và scheduler. KHÔNG phủ các chuyển trạng thái tuần tự và guard đơn lẻ (thuộc TR functional).

Tiêu chí đạt:

1. Không có lần chạy nào bị thực hiện 2 lần; không có lần RUNNING của lịch đã CANCELLED; DELETE chỉ thành công khi không có lần RUNNING; trạng thái cuối nhất quán với một thứ tự tuần tự hợp lệ. (`TCND-SCHEDULED-TRANSFER-SM-Schedule-concurrency-p34e996`)

#### TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-functional

Kiểm các chuyển trạng thái và guard của lần chạy: chiếm PENDING→RUNNING (attempts+1, có correlationId, 1 lời gọi lõi); SUCCEEDED; thử lại về PENDING (+5 phút/+1 giờ) với giới hạn FRAUD_CHECK_UNAVAILABLE 3 lần, INSUFFICIENT_FUNDS 2 lần; FAILED; SKIPPED khi tiếp tục lịch; MISSED sau 24 giờ; CANCELLED khi DELETE; phục hồi lease 120 giây.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| SM-ScheduledRun | functional | P1 | test | llm/approved |

Phạm vi: Phủ 14 điều kiện stateTransition/guardViolation của TRN-ScheduledRun-1..8. Không phủ giới hạn 20 lần chạy mỗi chu kỳ (TR performance) và tranh chấp hai scheduler hoặc idempotency khi chiếm lại (TR reliability).

Tiêu chí đạt:

1. Khi chưa đến hạn, lịch không ACTIVE hoặc rowcount=0: status=PENDING, attempts không đổi, không sinh correlationId, không có lời gọi lõi chuyển tiền. (`TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-1-guardViolation-guard`)
2. PENDING→RUNNING; attempts = giá trị cũ + 1; correlationId khác rỗng; đúng 1 lời gọi lõi chuyển tiền. (`TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-1-stateTransition`)
3. RUNNING→SUCCEEDED; transferId đã lưu; consecutive_failures=0; 1 audit; 1 sự kiện ScheduledRunCompleted; lần kế tiếp được sinh hoặc lịch COMPLETED. (`TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-2-stateTransition`)
4. Ở biên hết lượt (FRAUD_CHECK_UNAVAILABLE quá 3 lần, INSUFFICIENT_FUNDS quá 2 lần): không có chuyển RUNNING→PENDING, next_attempt_at không đổi, kết thúc FAILED theo TRN-ScheduledRun-4. (`TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-3-guardViolation-guard`)
5. RUNNING→PENDING khi còn lượt; next_attempt_at = now + 5 phút hoặc now + 1 giờ tùy mã lỗi, kiểm mỗi mã lỗi một ca. (`TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-3-stateTransition`)
6. Lỗi không vĩnh viễn còn lượt thử: status không phải FAILED; consecutive_failures không đổi; không có sự kiện ScheduledRunFailed. (`TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-4-guardViolation-guard`)
7. RUNNING→FAILED; reasonCode = mã lỗi từ lõi; consecutive_failures = cũ + 1; 1 audit; 1 sự kiện ScheduledRunFailed. (`TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-4-stateTransition`)
8. Tiếp tục lịch PAUSED với lần chạy chưa quá hạn: status=PENDING; không có chuyển sang SKIPPED. (`TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-5-guardViolation-guard`)
9. PENDING→SKIPPED; không có lời gọi lõi chuyển tiền và không có lần chạy bù mới. (`TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-5-stateTransition`)
10. Quá hạn không hơn 24 giờ (thử tại 23h59m và đúng 24h00m): status=PENDING; không có sự kiện MISSED. (`TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-6-guardViolation-guard`)
11. Quá hạn hơn 24 giờ: PENDING→MISSED; 1 sự kiện ScheduledRunFailed reasonCode=MISSED; 1 audit; consecutive_failures không đổi. (`TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-6-stateTransition`)
12. Sau DELETE lịch: PENDING→CANCELLED; sau một chu kỳ quét kế tiếp (30 giây) scheduler không chiếm lần chạy này. (`TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-7-stateTransition`)
13. started_at chưa cũ hơn lease (thử tại 119 giây và đúng 120 giây): status=RUNNING; attempts không đổi. (`TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-8-guardViolation-guard`)
14. started_at cũ hơn lease 120 giây: RUNNING→PENDING; attempts bằng giá trị trước phục hồi (lease 120 giây là đề xuất Q-S2, chưa chốt). (`TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-8-stateTransition`)

#### TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-performance

Kiểm giới hạn thông lượng của chu kỳ quét 30 giây: khi có 25 lần chạy đến hạn, tối đa 20 lần được chiếm sang RUNNING trong một chu kỳ, phần còn lại giữ PENDING cho chu kỳ sau.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| SM-ScheduledRun | performance | P1 | benchmark | llm/approved |

Phạm vi: Chỉ phủ giới hạn 20 lần chiếm mỗi chu kỳ. Không phủ logic chuyển trạng thái của từng lần chạy (TR functional) và tranh chấp/idempotency (TR reliability).

Tiêu chí đạt:

1. Seed 25 lần chạy đến hạn, sau đúng một chu kỳ số bản ghi sang RUNNING ≤ 20; các bản ghi còn lại vẫn PENDING. (`TCND-SCHEDULED-TRANSFER-SM-ScheduledRun-performance-pa68a97`)

#### TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-reliability

Kiểm độ tin cậy khi chiếm lần chạy: hai scheduler song song trên cùng bản ghi PENDING chỉ một bên thắng; sau phục hồi lần chạy kẹt, chiếm lại dùng cùng khóa idempotency nên không tạo giao dịch trùng.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| SM-ScheduledRun | reliability | P1 | test | llm/approved |

Phạm vi: Phủ tranh chấp đồng thời khi chiếm và idempotency sau phục hồi. Không phủ từng chuyển trạng thái đơn lẻ (TR functional) và giới hạn 20 lần mỗi chu kỳ (TR performance).

Tiêu chí đạt:

1. Chạy song song 2 tiến trình nhiều lần: đúng 1 bên rowcount=1; attempts = cũ + 1; đúng 1 lời gọi lõi chuyển tiền. (`TCND-SCHEDULED-TRANSFER-SM-ScheduledRun-concurrency-p4b41b3`)
2. Sau chiếm lại: số giao dịch theo khóa idempotency của lần chạy = 1; transferId trùng giao dịch ban đầu; status=SUCCEEDED. (`TCND-SCHEDULED-TRANSFER-SM-ScheduledRun-idempotency-p456351`)

### 4.3 L3: Cô lập service

#### TR-SCHEDULED-TRANSFER-L3-MECH-CatchUp-reliability

Sau khi hệ thống ngừng rồi khởi động lại (kể cả khởi động lại nhiều lần và nhiều chu kỳ quét liên tiếp), mỗi lần chạy quá hạn ≤ 24 giờ phải được chạy bù đúng 1 lần, sinh đúng 1 giao dịch và xử lý xong trong 5 chu kỳ quét đầu; lần quá hạn > 24 giờ chuyển MISSED, không chạy bù, không tạo giao dịch.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| MECH-CatchUp | reliability | P1 | test | llm/approved |

Phạm vi: Phủ độ tin cậy L3 của cơ chế chạy bù: tính idempotent (không chạy trùng) và khả năng phục hồi sau khởi động lại (thời hạn 5 chu kỳ quét, MISSED khi quá 24 giờ). Không phủ chức năng chạy theo lịch thông thường, nội dung giao dịch hay hiệu năng ngoài giới hạn 20 lần chạy mỗi chu kỳ.

Tiêu chí đạt:

1. Số giao dịch của mỗi lần chạy bù quá hạn ≤ 24 giờ = 1; không có bản ghi chạy trùng sau khi khởi động lại nhiều lần hoặc qua nhiều chu kỳ quét liên tiếp. (`TCND-SCHEDULED-TRANSFER-MECH-CatchUp-idempotency-p602a3c`)
2. Lần quá hạn ≤ 24 giờ chạy đúng 1 lần và xử lý xong trong 5 chu kỳ quét đầu kể từ lần quét đầu sau khởi động lại (trường hợp xấu nhất giới hạn 20 lần chạy mỗi chu kỳ). Lần quá hạn > 24 giờ: trạng thái MISSED, phát ScheduledRunFailed với reasonCode=MISSED, 0 giao dịch. (`TCND-SCHEDULED-TRANSFER-MECH-CatchUp-resilience-holds`)

#### TR-SCHEDULED-TRANSFER-L3-MECH-RunClaim-reliability

Kiểm tra cơ chế chiếm lần chạy bằng UPDATE có điều kiện trên scheduled_runs không tạo giao dịch trùng trong hai tình huống: (1) hai scheduler cùng chiếm một lần chạy đến hạn trong cùng chu kỳ quét 30 giây; (2) tiến trình chết sau khi chiếm đã commit nhưng trước khi hoàn tất chuyển tiền, rồi khởi động lại.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| MECH-RunClaim | reliability | P0 | test | llm/approved |

Phạm vi: Phủ độ tin cậy mức L3 của MECH-RunClaim: tranh chấp song song giữa hai scheduler (TC-S10) và phục hồi sau khi tiến trình chết giữa chiếm và chuyển tiền (TC-S11). Không phủ logic lõi chuyển tiền, tính thời điểm đến hạn hay các chỉ tiêu phi chức năng khác.

Tiêu chí đạt:

1. Sau khi cả hai scheduler hoàn tất, đếm theo Idempotency-Key (mã lịch + thời điểm dự kiến): số giao dịch tạo ra cho lần chạy = 1; số scheduler chiếm thành công = 1; số giao dịch trùng = 0 (TC-S10). (`TCND-SCHEDULED-TRANSFER-MECH-RunClaim-concurrency-parallel`)
2. Sau khởi động lại và chạy vài chu kỳ quét, đếm theo Idempotency-Key: số giao dịch cho lần chạy ≤ 1, không có giao dịch thứ hai và lần chạy không bị bỏ sót (TC-S11). (`TCND-SCHEDULED-TRANSFER-MECH-RunClaim-resilience-crash-mid`)

#### TR-SCHEDULED-TRANSFER-L3-MECH-ScheduleRetry-reliability

Xác nhận khi một lần chạy lịch chuyển tiền thất bại, số lần thử lại và khoảng cách giữa các lần đúng theo loại lỗi: FRAUD_CHECK_UNAVAILABLE tối đa 3 lần cách 5 phút; INSUFFICIENT_FUNDS tối đa 2 lần cách 1 giờ; ACCOUNT_INACTIVE, FRAUD_REJECTED, LIMIT_EXCEEDED không thử lại (0 lần). Hết lượt thì run FAILED kèm reasonCode; sau 3 run FAILED liên tiếp lịch chuyển PAUSED.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| MECH-ScheduleRetry | reliability | P1 | test | llm/approved |

Phạm vi: Phủ chính sách thử lại theo loại lỗi, trạng thái run khi hết lượt hoặc lỗi không thử lại, và chuyển lịch sang PAUSED sau 3 run FAILED liên tiếp, ở mức tích hợp L3 với đồng hồ giả lập. Không phủ logic nghiệp vụ lõi chuyển tiền, cách tạo/sửa lịch hay các điều kiện chức năng khác của Scheduler.

Tiêu chí đạt:

1. FRAUD_CHECK_UNAVAILABLE: tối đa 3 lần thử lại, mỗi lần cách nhau 5 phút (đồng hồ giả lập), hết lượt run = FAILED có reasonCode. INSUFFICIENT_FUNDS: tối đa 2 lần thử lại, cách 1 giờ, hết lượt run = FAILED. ACCOUNT_INACTIVE/FRAUD_REJECTED/LIMIT_EXCEEDED: run FAILED ngay, 0 lần thử lại (số lần gọi lõi chuyển tiền = 1). Sau 3 run FAILED liên tiếp lịch trong DB = PAUSED. (`TCND-SCHEDULED-TRANSFER-MECH-ScheduleRetry-resilience-holds`)

#### TR-SCHEDULED-TRANSFER-L3-MECH-SchedulerThrottle-performance

Kiểm 2 điều kiện hiệu năng (performance) của MECH-SchedulerThrottle ở mức L3: Khi đúng 20 lần chạy đến hạn trong một chu kỳ quét, scheduler xử lý đủ cả 20 lần; Khi số lần chạy đến hạn vượt 20 trong một chu kỳ quét (ví dụ 100 lần chạy cùng l

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| MECH-SchedulerThrottle | performance | P0 | benchmark | rule |

Phạm vi: Chỉ phần hiệu năng mức L3 của MECH-SchedulerThrottle (Giới hạn 20 lần chạy mỗi chu kỳ quét và rate limit 10 rps cho scheduler)

Tiêu chí đạt:

1. Khi đúng 20 lần chạy đến hạn trong một chu kỳ quét, scheduler xử lý đủ cả 20 lần chạy và tốc độ gọi không vượt 10 rps. → Cả 20 lần chạy được chọn và xử lý trong chu kỳ đó; không lần nào bị bỏ sót hay hoãn; tốc độ gọi lõi chuyển tiền không quá 10 rps. (`TCND-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-performance-at-limit`)
2. Khi số lần chạy đến hạn vượt 20 trong một chu kỳ quét (ví dụ 100 lần chạy cùng lúc), scheduler chỉ xử lý tối đa 20 lần chạy trong chu kỳ đó. → Chu kỳ đó xử lý không quá 20 lần chạy; phần còn lại chưa bị chiếm và chưa xử lý trong chu kỳ này. Cách xử lý phần vượt ở các chu kỳ sau (hoãn, thứ tự ưu tiên) chưa được nguồn nêu rõ. (`TCND-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-performance-over-limit`)

#### TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler-performance

Khi một chu kỳ quét 30 giây có hơn 20 run đến hạn, Scheduler chỉ chiếm tối đa 20 run trong chu kỳ đó, phần còn lại dời sang chu kỳ sau mà vẫn có ít nhất 99% run bắt đầu trong 60 giây kể từ thời điểm dự kiến.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| MOD-Scheduler | performance | P1 | test | llm/approved |

Phạm vi: Chỉ phủ giới hạn 20 run mỗi chu kỳ và độ trễ bắt đầu run (phân vị 99) trên đồng hồ giả. Không phủ tranh chấp chiếm run, lỗi phụ thuộc và thử lại (thuộc TR reliability).

Tiêu chí đạt:

1. Mỗi chu kỳ có tối đa 20 run được chiếm; ít nhất 99% run (phân vị 99 trên toàn tập run của kịch bản) bắt đầu trong 60 giây kể từ thời điểm dự kiến, với chu kỳ quét 30 giây, đo bằng đồng hồ giả. (`TCND-SCHEDULED-TRANSFER-MOD-Scheduler-timing-p077a10`)

#### TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler-reliability

Scheduler phải giữ nhất quán khi nhiều scheduler tranh chấp một run và khi các module phụ thuộc (Account, Audit, Risk, Transfer) lỗi hoặc không phản hồi: không có giao dịch COMPLETED trùng, run không kẹt RUNNING, retry và trạng thái đúng quy định.

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| MOD-Scheduler | reliability | P1 | test | llm/approved |

Phạm vi: Phủ tranh chấp chiếm run và lỗi từ Account, Audit, Risk, Transfer. Không phủ giới hạn 20 run mỗi chu kỳ và độ trễ bắt đầu (thuộc TR performance). Hành vi Account và Audit khi lỗi hạ tầng chưa được nguồn xác định.

Tiêu chí đạt:

1. Chạy song song 2 luồng trên cùng CSDL SQLite: đúng một giao dịch COMPLETED cho run này (đếm ledger_entries theo Idempotency-Key); tiến trình còn lại không chiếm được run. (`TCND-SCHEDULED-TRANSFER-MOD-Scheduler-concurrency-p933b8e`)
2. Hành vi chi tiết chờ làm rõ; tối thiểu sau khi tiêm lỗi không có giao dịch COMPLETED trùng và run không kẹt ở RUNNING (đối chiếu scheduled_runs và ledger_entries). (`TCND-SCHEDULED-TRANSFER-MOD-Scheduler-errorHandling-dep-MOD-Account`)
3. Hành vi chi tiết chờ làm rõ; tối thiểu sau khi tiêm lỗi, trạng thái run, ledger_entries và bản ghi audit phải nhất quán với nhau. (`TCND-SCHEDULED-TRANSFER-MOD-Scheduler-errorHandling-dep-MOD-Audit`)
4. Số tiền >= 5.000.000 VND: không có giao dịch, retry 3 lần cách 5 phút, rồi run FAILED với reasonCode FRAUD_CHECK_UNAVAILABLE. Số tiền < 5.000.000 VND: giao dịch COMPLETED với riskCheck=SKIPPED, run SUCCEEDED. (`TCND-SCHEDULED-TRANSFER-MOD-Scheduler-errorHandling-dep-MOD-Risk`)
5. INSUFFICIENT_FUNDS thử lại tối đa 2 lần cách 1 giờ rồi run FAILED; ACCOUNT_INACTIVE, FRAUD_REJECTED, LIMIT_EXCEEDED làm run FAILED ngay, không thử lại; sau 3 run FAILED liên tiếp lịch chuyển PAUSED; không có giao dịch trùng. (`TCND-SCHEDULED-TRANSFER-MOD-Scheduler-errorHandling-dep-MOD-Transfer`)

## 5. Truy vết

| Nguồn | Obligation | Điều kiện | TR | Coverage item | Test case |
| --- | --- | --- | --- | --- | --- |
| `AC-FR-040-1`, `EP-CreateSchedule`, `FR-040`, `RSP-Create-201` | `TOB-SCHEDULED-TRANSFER-FR-040-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-040-1-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-040-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-040-1-functional-c` |
| `AC-FR-040-2`, `FR-040` | `TOB-SCHEDULED-TRANSFER-FR-040-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-040-2-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-040-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-040-2-functional-c` |
| `AC-FR-040-3`, `FLD-Create-firstRunAt`, `FR-040`, `RSP-Create-422-time` | `TOB-SCHEDULED-TRANSFER-FR-040-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-040-3-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-040-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-040-3-functional-c` |
| `AC-FR-040-4`, `FR-040`, `RSP-Create-404` | `TOB-SCHEDULED-TRANSFER-FR-040-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-040-4-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-040-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-040-4-functional-c` |
| `AC-FR-040-5`, `FR-040`, `RSP-Create-403` | `TOB-SCHEDULED-TRANSFER-FR-040-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-040-5-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-040-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-040-5-functional-c` |
| `AC-FR-040-6`, `DE-ScheduledTransfer`, `FR-040`, `RSP-Create-422-limit` | `TOB-SCHEDULED-TRANSFER-FR-040-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-040-6-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-040-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-040-6-functional-c` |
| `AC-FR-041-1`, `EP-CreateSchedule`, `FR-041`, `RSP-Create-201` | `TOB-SCHEDULED-TRANSFER-FR-041-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-041-1-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-041-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-041-1-functional-c` |
| `AC-FR-041-2`, `FR-041`, `RSP-Create-400` | `TOB-SCHEDULED-TRANSFER-FR-041-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-041-2-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-041-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-041-2-functional-c` |
| `AC-FR-041-3`, `DI-Scheduler`, `FR-041`, `MOD-Scheduler`, `SCN-ScheduledRun` | `TOB-SCHEDULED-TRANSFER-FR-041-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-041-3-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-041-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-041-3-functional-c` |
| `AC-FR-041-4`, `FR-041`, `SCN-ScheduledRun` | `TOB-SCHEDULED-TRANSFER-FR-041-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-041-4-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-041-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-041-4-functional-c` |
| `AC-FR-042-1`, `EP-GetSchedule`, `FR-042`, `RSP-Get-200` | `TOB-SCHEDULED-TRANSFER-FR-042-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-042-1-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-042-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-042-1-functional-c` |
| `AC-FR-042-2`, `EP-ListSchedules`, `FLD-ListSchedules-limit`, `FLD-ListSchedules-offset`, `FLD-ListSchedules-status`, `FR-042`, `RSP-ListSchedules-200` | `TOB-SCHEDULED-TRANSFER-FR-042-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-042-2-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-042-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-042-2-functional-c` |
| `AC-FR-042-3`, `DE-ScheduledRun`, `EP-ListRuns`, `FR-042`, `RSP-ListRuns-200` | `TOB-SCHEDULED-TRANSFER-FR-042-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-042-3-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-042-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-042-3-functional-c` |
| `AC-FR-042-4`, `FR-042`, `RSP-Get-403`, `RSP-Get-404`, `RSP-ListRuns-403`, `RSP-ListRuns-404` | `TOB-SCHEDULED-TRANSFER-FR-042-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-042-4-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-042-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-042-4-functional-c` |
| `AC-FR-043-1`, `EP-CancelSchedule`, `FR-043`, `RSP-Cancel-200` | `TOB-SCHEDULED-TRANSFER-FR-043-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-043-1-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-043-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-043-1-functional-c` |
| `AC-FR-043-2`, `EP-CancelSchedule`, `FR-043`, `RSP-Cancel-200` | `TOB-SCHEDULED-TRANSFER-FR-043-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-043-2-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-043-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-043-2-functional-c` |
| `AC-FR-043-3`, `FR-043`, `RSP-Cancel-409` | `TOB-SCHEDULED-TRANSFER-FR-043-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-043-3-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-043-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-043-3-functional-c` |
| `AC-FR-044-1`, `EP-UpdateScheduleStatus`, `FR-044`, `MOD-Scheduler`, `RSP-Patch-200` | `TOB-SCHEDULED-TRANSFER-FR-044-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-044-1-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-044-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-044-1-functional-c` |
| `AC-FR-044-2`, `DE-ScheduledRun`, `FR-044`, `FR-049`, `RSP-Patch-200` | `TOB-SCHEDULED-TRANSFER-FR-044-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-044-2-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-044-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-044-2-functional-c` |
| `AC-FR-044-3`, `EP-UpdateScheduleStatus`, `FR-044`, `RSP-Patch-409` | `TOB-SCHEDULED-TRANSFER-FR-044-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-044-3-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-044-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-044-3-functional-c` |
| `AC-FR-045-1`, `DI-TransferCore`, `FR-045`, `SCN-ScheduledRun` | `TOB-SCHEDULED-TRANSFER-FR-045-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-045-1-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-045-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-045-1-functional-c` |
| `AC-FR-045-2`, `FR-045`, `RISK-005` | `TOB-SCHEDULED-TRANSFER-FR-045-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-045-2-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-045-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-045-2-functional-c` |
| `AC-FR-045-3`, `FR-045`, `INT-01`, `SCN-ScheduledRun` | `TOB-SCHEDULED-TRANSFER-FR-045-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-045-3-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-045-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-045-3-functional-c` |
| `AC-FR-045-4`, `DE-Transfer`, `FR-045`, `MOD-Transfer` | `TOB-SCHEDULED-TRANSFER-FR-045-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-045-4-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-045-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-045-4-functional-c` |
| `AC-FR-046-1`, `FR-046`, `MECH-ScheduleRetry` | `TOB-SCHEDULED-TRANSFER-FR-046-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-046-1-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-046-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-046-1-functional-c` |
| `AC-FR-046-2`, `FR-046`, `MECH-ScheduleRetry` | `TOB-SCHEDULED-TRANSFER-FR-046-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-046-2-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-046-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-046-2-functional-c` |
| `AC-FR-046-3`, `FR-046`, `MECH-ScheduleRetry` | `TOB-SCHEDULED-TRANSFER-FR-046-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-046-3-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-046-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-046-3-functional-c` |
| `AC-FR-046-4`, `FR-046`, `MECH-ScheduleRetry` | `TOB-SCHEDULED-TRANSFER-FR-046-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-046-4-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-046-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-046-4-functional-c` |
| `AC-FR-047-1`, `FR-047`, `MECH-Outbox`, `SCN-ScheduledRun` | `TOB-SCHEDULED-TRANSFER-FR-047-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-047-1-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-047-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-047-1-functional-c` |
| `AC-FR-047-2`, `FR-046`, `FR-047`, `MECH-Outbox` | `TOB-SCHEDULED-TRANSFER-FR-047-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-047-2-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-047-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-047-2-functional-c` |
| `AC-FR-047-3`, `DI-SchedNotify`, `FR-047`, `INT-03`, `MOD-Notify`, `SCN-Notify` | `TOB-SCHEDULED-TRANSFER-FR-047-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-047-3-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-047-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-047-3-functional-c` |
| `AC-FR-048-1`, `DE-AuditLog`, `DI-SchedAudit`, `EP-CancelSchedule`, `EP-CreateSchedule`, `EP-UpdateScheduleStatus`, `FR-048` | `TOB-SCHEDULED-TRANSFER-FR-048-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-048-1-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-048-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-048-1-functional-c` |
| `AC-FR-048-2`, `DE-AuditLog`, `DI-SchedAudit`, `FR-048`, `SCN-ScheduledRun`, `TC-S07` | `TOB-SCHEDULED-TRANSFER-FR-048-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-048-2-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-048-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-048-2-functional-c` |
| `AC-FR-049-1`, `FR-049`, `MECH-CatchUp`, `NFR-011`, `NFR-012` | `TOB-SCHEDULED-TRANSFER-FR-049-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-049-1-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-049-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-049-1-functional-c` |
| `AC-FR-049-2`, `FR-049`, `MECH-CatchUp` | `TOB-SCHEDULED-TRANSFER-FR-049-func` | `TCND-SCHEDULED-TRANSFER-AC-FR-049-2-functional` | `TR-SCHEDULED-TRANSFER-L2-FR-049-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-FR-049-2-functional-c` |
| `AC-NFR-010-1`, `AD-05`, `NFR-010`, `SCN-ScheduledRun`, `TC-S09` | `TOB-SCHEDULED-TRANSFER-NFR-010-nfr` | `TCND-SCHEDULED-TRANSFER-AC-NFR-010-1-timing` | `TR-SCHEDULED-TRANSFER-L2-NFR-010-performance` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-NFR-010-1-timing-c` |
| `AC-NFR-011-1`, `MECH-RunClaim`, `MOD-Scheduler`, `NFR-011` | `TOB-SCHEDULED-TRANSFER-NFR-011-nfr` | `TCND-SCHEDULED-TRANSFER-AC-NFR-011-1-concurrency` | `TR-SCHEDULED-TRANSFER-L2-NFR-011-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-NFR-011-1-concurrency-c` |
| `AC-NFR-011-2`, `FR-045`, `FR-049`, `MECH-RunClaim`, `NFR-011` | `TOB-SCHEDULED-TRANSFER-NFR-011-nfr` | `TCND-SCHEDULED-TRANSFER-AC-NFR-011-2-concurrency` | `TR-SCHEDULED-TRANSFER-L2-NFR-011-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-NFR-011-2-concurrency-c` |
| `AC-NFR-012-1`, `MECH-CatchUp`, `MOD-Scheduler`, `NFR-012` | `TOB-SCHEDULED-TRANSFER-NFR-012-nfr` | `TCND-SCHEDULED-TRANSFER-AC-NFR-012-1-resilience` | `TR-SCHEDULED-TRANSFER-L2-NFR-012-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-NFR-012-1-resilience-c` |
| `AC-NFR-013-1`, `NFR-013`, `TC-S13` | `TOB-SCHEDULED-TRANSFER-NFR-013-nfr` | `TCND-SCHEDULED-TRANSFER-AC-NFR-013-1-performance` | `TR-SCHEDULED-TRANSFER-L2-NFR-013-performance` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-NFR-013-1-performance-c` |
| `AC-NFR-013-2`, `MECH-SchedulerThrottle`, `MOD-Scheduler`, `NFR-013` | `TOB-SCHEDULED-TRANSFER-NFR-013-nfr` | `TCND-SCHEDULED-TRANSFER-AC-NFR-013-2-performance` | `TR-SCHEDULED-TRANSFER-L2-NFR-013-performance` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-NFR-013-2-performance-c` |
| `AC-NFR-014-1`, `NFR-014`, `SCN-ScheduledRun` | `TOB-SCHEDULED-TRANSFER-NFR-014-nfr` | `TCND-SCHEDULED-TRANSFER-AC-NFR-014-1-observability` | `TR-SCHEDULED-TRANSFER-L2-NFR-014-maintainability` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-NFR-014-1-observability-c` |
| `AC-NFR-014-2`, `INT-01`, `NFR-014`, `SCN-ScheduledRun`, `TC-S12` | `TOB-SCHEDULED-TRANSFER-NFR-014-nfr` | `TCND-SCHEDULED-TRANSFER-AC-NFR-014-2-observability` | `TR-SCHEDULED-TRANSFER-L2-NFR-014-maintainability` | 1 | `TC-L2-SCHEDULED-TRANSFER-AC-NFR-014-2-observability-c` |
| `AC-FR-048-1`, `EP-CancelSchedule`, `FR-048` | `TOB-SCHEDULED-TRANSFER-EP-CancelSchedule-proposal` | `TCND-SCHEDULED-TRANSFER-EP-CancelSchedule-audit-p1cc3a7` | `TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-audit-p1cc3a7-c` |
| `AC-FR-043-2`, `EP-CancelSchedule` | `TOB-SCHEDULED-TRANSFER-EP-CancelSchedule-contract` | `TCND-SCHEDULED-TRANSFER-EP-CancelSchedule-concurrency-concurrent-dup` | `TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-concurrency-concurrent-dup-c` |
| `EP-CancelSchedule` | `TOB-SCHEDULED-TRANSFER-EP-CancelSchedule-contract` | `TCND-SCHEDULED-TRANSFER-EP-CancelSchedule-idempotency-payload-mismatch` | `TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-idempotency-payload-mismatch-c` |
| `AC-FR-043-2`, `EP-CancelSchedule` | `TOB-SCHEDULED-TRANSFER-EP-CancelSchedule-contract` | `TCND-SCHEDULED-TRANSFER-EP-CancelSchedule-idempotency-replay` | `TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-idempotency-replay-c` |
| `AC-FR-040-5`, `EP-CreateSchedule`, `RSP-Create-403` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-proposal` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-access-p73a390` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-security` | 1 | `TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-access-p73a390-c` |
| `AC-FR-048-1`, `EP-CreateSchedule`, `MOD-Audit` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-proposal` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-audit-p28bd0d` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-audit-p28bd0d-c` |
| `AC-FR-048-1`, `EP-CreateSchedule`, `FR-048` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-proposal` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-audit-pf42db3` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-audit-pf42db3-c` |
| `EP-CreateSchedule` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-concurrency-concurrent-dup` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-concurrency-concurrent-dup-c` |
| `EP-CreateSchedule`, `FLD-Create-IdempotencyKey`, `RSP-Create-200` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-proposal` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-concurrency-pdcbc85` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-concurrency-pdcbc85-c` |
| `AC-FR-040-4`, `EP-CreateSchedule`, `RSP-Create-404` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-proposal` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-errorHandling-pe11059` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-errorHandling-pe11059-c` |
| `AC-FR-041-4`, `EP-CreateSchedule`, `FR-041` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-proposal` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-functional-p2341c6` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-functional-p2341c6-c` |
| `AC-FR-040-1`, `EP-CreateSchedule`, `RSP-Create-201` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-proposal` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-functional-pab4508` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-functional-pab4508-c` |
| `EP-CreateSchedule`, `RSP-Create-422-same` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-proposal` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-guardViolation-p588a23` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-guardViolation-p588a23-c` |
| `AC-FR-040-6`, `EP-CreateSchedule`, `RSP-Create-422-limit` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-proposal` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-guardViolation-pc9af5d` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-guardViolation-pc9af5d-c` |
| `EP-CreateSchedule`, `RSP-Create-200` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-proposal` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-idempotency-p50d7db` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-idempotency-p50d7db-c` |
| `EP-CreateSchedule`, `RSP-Create-409` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-idempotency-payload-mismatch` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-idempotency-payload-mismatch-c` |
| `EP-CreateSchedule`, `RSP-Create-200` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-idempotency-replay` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-idempotency-replay-c` |
| `AC-FR-041-2`, `EP-CreateSchedule`, `FLD-Create-maxRuns`, `RSP-Create-400` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-proposal` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-inputValidation-p4e161f` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-inputValidation-p4e161f-c` |
| `AC-FR-040-2`, `EP-CreateSchedule` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-proposal` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-invariant-p420954` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-invariant-p420954-c` |
| `AC-FR-040-3`, `EP-CreateSchedule`, `RSP-Create-422-time` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-proposal` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-timing-p31f6cf` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-performance` | 1 | `TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-timing-p31f6cf-c` |
| `EP-GetSchedule` | `TOB-SCHEDULED-TRANSFER-EP-GetSchedule-proposal` | `TCND-SCHEDULED-TRANSFER-EP-GetSchedule-idempotency-p52c40c` | `TR-SCHEDULED-TRANSFER-L2-EP-GetSchedule-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-EP-GetSchedule-idempotency-p52c40c-c` |
| `EP-ListSchedules` | `TOB-SCHEDULED-TRANSFER-EP-ListSchedules-proposal` | `TCND-SCHEDULED-TRANSFER-EP-ListSchedules-idempotency-p6e1377` | `TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-idempotency-p6e1377-c` |
| `AC-FR-048-1`, `EP-UpdateScheduleStatus`, `FR-048` | `TOB-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-proposal` | `TCND-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-audit-p664d35` | `TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-audit-p664d35-c` |
| `EP-UpdateScheduleStatus`, `MOD-Scheduler` | `TOB-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-proposal` | `TCND-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-concurrency-p1e8657` | `TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-concurrency-p1e8657-c` |
| `EP-CancelSchedule`, `FLD-Cancel-Authorization`, `RSP-Cancel-401` | `TOB-SCHEDULED-TRANSFER-EP-CancelSchedule-contract` | `TCND-SCHEDULED-TRANSFER-FLD-Cancel-Authorization-inputValidation-missing` | `TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FLD-Cancel-Authorization-inputValidation-missing-v0` |
| `EP-CancelSchedule`, `FLD-Cancel-scheduleId` | `TOB-SCHEDULED-TRANSFER-EP-CancelSchedule-contract` | `TCND-SCHEDULED-TRANSFER-FLD-Cancel-scheduleId-inputValidation-missing` | `TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FLD-Cancel-scheduleId-inputValidation-missing-v0` |
| `EP-CreateSchedule`, `FLD-Create-Authorization` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` | `TCND-SCHEDULED-TRANSFER-FLD-Create-Authorization-inputValidation-missing` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FLD-Create-Authorization-inputValidation-missing-v0` |
| `EP-CreateSchedule`, `FLD-Create-IdempotencyKey`, `RSP-Create-400` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` | `TCND-SCHEDULED-TRANSFER-FLD-Create-IdempotencyKey-inputValidation-missing` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FLD-Create-IdempotencyKey-inputValidation-missing-v0` |
| `EP-CreateSchedule`, `FLD-Create-amount`, `RSP-Create-422-amount` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` | `TCND-SCHEDULED-TRANSFER-FLD-Create-amount-inputValidation-max` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` | 2 | `TC-L2-SCHEDULED-TRANSFER-FLD-Create-amount-inputValidation-max-v0`, `TC-L2-SCHEDULED-TRANSFER-FLD-Create-amount-inputValidation-max-v1` |
| `EP-CreateSchedule`, `FLD-Create-amount`, `RSP-Create-400` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` | `TCND-SCHEDULED-TRANSFER-FLD-Create-amount-inputValidation-min` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` | 2 | `TC-L2-SCHEDULED-TRANSFER-FLD-Create-amount-inputValidation-min-v0`, `TC-L2-SCHEDULED-TRANSFER-FLD-Create-amount-inputValidation-min-v1` |
| `EP-CreateSchedule`, `FLD-Create-amount`, `RSP-Create-400` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` | `TCND-SCHEDULED-TRANSFER-FLD-Create-amount-inputValidation-missing` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FLD-Create-amount-inputValidation-missing-v0` |
| `AC-FR-041-1`, `AC-FR-041-2`, `EP-CreateSchedule`, `FLD-Create-endDate` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` | `TCND-SCHEDULED-TRANSFER-FLD-Create-endDate-inputValidation-missing` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FLD-Create-endDate-inputValidation-missing-v0` |
| `EP-CreateSchedule`, `FLD-Create-firstRunAt`, `RSP-Create-400` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` | `TCND-SCHEDULED-TRANSFER-FLD-Create-firstRunAt-inputValidation-missing` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FLD-Create-firstRunAt-inputValidation-missing-v0` |
| `EP-CreateSchedule`, `FLD-Create-fromAccountId`, `RSP-Create-400` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` | `TCND-SCHEDULED-TRANSFER-FLD-Create-fromAccountId-inputValidation-missing` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FLD-Create-fromAccountId-inputValidation-missing-v0` |
| `EP-CreateSchedule`, `FLD-Create-maxRuns`, `RSP-Create-400` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` | `TCND-SCHEDULED-TRANSFER-FLD-Create-maxRuns-inputValidation-max` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` | 2 | `TC-L2-SCHEDULED-TRANSFER-FLD-Create-maxRuns-inputValidation-max-v0`, `TC-L2-SCHEDULED-TRANSFER-FLD-Create-maxRuns-inputValidation-max-v1` |
| `EP-CreateSchedule`, `FLD-Create-maxRuns`, `RSP-Create-400` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` | `TCND-SCHEDULED-TRANSFER-FLD-Create-maxRuns-inputValidation-min` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` | 2 | `TC-L2-SCHEDULED-TRANSFER-FLD-Create-maxRuns-inputValidation-min-v0`, `TC-L2-SCHEDULED-TRANSFER-FLD-Create-maxRuns-inputValidation-min-v1` |
| `AC-FR-041-1`, `AC-FR-041-2`, `EP-CreateSchedule`, `FLD-Create-maxRuns` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` | `TCND-SCHEDULED-TRANSFER-FLD-Create-maxRuns-inputValidation-missing` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FLD-Create-maxRuns-inputValidation-missing-v0` |
| `EP-CreateSchedule`, `FLD-Create-recurrence`, `RSP-Create-400` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` | `TCND-SCHEDULED-TRANSFER-FLD-Create-recurrence-inputValidation-enum` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` | 5 | `TC-L2-SCHEDULED-TRANSFER-FLD-Create-recurrence-inputValidation-enum-v0`, `TC-L2-SCHEDULED-TRANSFER-FLD-Create-recurrence-inputValidation-enum-v1`, `TC-L2-SCHEDULED-TRANSFER-FLD-Create-recurrence-inputValidation-enum-v2`, `TC-L2-SCHEDULED-TRANSFER-FLD-Create-recurrence-inputValidation-enum-v3`, `TC-L2-SCHEDULED-TRANSFER-FLD-Create-recurrence-inputValidation-enum-v4` |
| `EP-CreateSchedule`, `FLD-Create-recurrence`, `RSP-Create-400` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` | `TCND-SCHEDULED-TRANSFER-FLD-Create-recurrence-inputValidation-missing` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FLD-Create-recurrence-inputValidation-missing-v0` |
| `EP-CreateSchedule`, `FLD-Create-toAccountId`, `RSP-Create-400` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` | `TCND-SCHEDULED-TRANSFER-FLD-Create-toAccountId-inputValidation-missing` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FLD-Create-toAccountId-inputValidation-missing-v0` |
| `EP-GetSchedule`, `FLD-Get-Authorization`, `RSP-Get-401` | `TOB-SCHEDULED-TRANSFER-EP-GetSchedule-contract` | `TCND-SCHEDULED-TRANSFER-FLD-Get-Authorization-inputValidation-missing` | `TR-SCHEDULED-TRANSFER-L2-EP-GetSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FLD-Get-Authorization-inputValidation-missing-v0` |
| `EP-GetSchedule`, `FLD-Get-scheduleId` | `TOB-SCHEDULED-TRANSFER-EP-GetSchedule-contract` | `TCND-SCHEDULED-TRANSFER-FLD-Get-scheduleId-inputValidation-missing` | `TR-SCHEDULED-TRANSFER-L2-EP-GetSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FLD-Get-scheduleId-inputValidation-missing-v0` |
| `EP-ListRuns`, `FLD-ListRuns-Authorization`, `RSP-ListRuns-401` | `TOB-SCHEDULED-TRANSFER-EP-ListRuns-contract` | `TCND-SCHEDULED-TRANSFER-FLD-ListRuns-Authorization-inputValidation-missing` | `TR-SCHEDULED-TRANSFER-L2-EP-ListRuns-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FLD-ListRuns-Authorization-inputValidation-missing-v0` |
| `EP-ListRuns`, `FLD-ListRuns-limit`, `MOD-API` | `TOB-SCHEDULED-TRANSFER-EP-ListRuns-contract` | `TCND-SCHEDULED-TRANSFER-FLD-ListRuns-limit-inputValidation-max` | `TR-SCHEDULED-TRANSFER-L2-EP-ListRuns-functional` | 2 | `TC-L2-SCHEDULED-TRANSFER-FLD-ListRuns-limit-inputValidation-max-v0`, `TC-L2-SCHEDULED-TRANSFER-FLD-ListRuns-limit-inputValidation-max-v1` |
| `EP-ListRuns`, `FLD-ListRuns-limit`, `MOD-API` | `TOB-SCHEDULED-TRANSFER-EP-ListRuns-contract` | `TCND-SCHEDULED-TRANSFER-FLD-ListRuns-limit-inputValidation-min` | `TR-SCHEDULED-TRANSFER-L2-EP-ListRuns-functional` | 2 | `TC-L2-SCHEDULED-TRANSFER-FLD-ListRuns-limit-inputValidation-min-v0`, `TC-L2-SCHEDULED-TRANSFER-FLD-ListRuns-limit-inputValidation-min-v1` |
| `EP-ListRuns`, `FLD-ListRuns-offset`, `MOD-API` | `TOB-SCHEDULED-TRANSFER-EP-ListRuns-contract` | `TCND-SCHEDULED-TRANSFER-FLD-ListRuns-offset-inputValidation-min` | `TR-SCHEDULED-TRANSFER-L2-EP-ListRuns-functional` | 2 | `TC-L2-SCHEDULED-TRANSFER-FLD-ListRuns-offset-inputValidation-min-v0`, `TC-L2-SCHEDULED-TRANSFER-FLD-ListRuns-offset-inputValidation-min-v1` |
| `EP-ListRuns`, `FLD-ListRuns-scheduleId`, `MOD-API` | `TOB-SCHEDULED-TRANSFER-EP-ListRuns-contract` | `TCND-SCHEDULED-TRANSFER-FLD-ListRuns-scheduleId-inputValidation-missing` | `TR-SCHEDULED-TRANSFER-L2-EP-ListRuns-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FLD-ListRuns-scheduleId-inputValidation-missing-v0` |
| `EP-ListSchedules`, `FLD-ListSchedules-Authorization`, `RSP-ListSchedules-401` | `TOB-SCHEDULED-TRANSFER-EP-ListSchedules-contract` | `TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-Authorization-inputValidation-missing` | `TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-Authorization-inputValidation-missing-v0` |
| `EP-ListSchedules`, `FLD-ListSchedules-accountId` | `TOB-SCHEDULED-TRANSFER-EP-ListSchedules-contract` | `TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-accountId-inputValidation-missing` | `TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-accountId-inputValidation-missing-v0` |
| `EP-ListSchedules`, `FLD-ListSchedules-limit`, `RSP-ListSchedules-400` | `TOB-SCHEDULED-TRANSFER-EP-ListSchedules-contract` | `TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-limit-inputValidation-max` | `TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules-functional` | 2 | `TC-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-limit-inputValidation-max-v0`, `TC-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-limit-inputValidation-max-v1` |
| `EP-ListSchedules`, `FLD-ListSchedules-limit`, `RSP-ListSchedules-400` | `TOB-SCHEDULED-TRANSFER-EP-ListSchedules-contract` | `TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-limit-inputValidation-min` | `TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules-functional` | 2 | `TC-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-limit-inputValidation-min-v0`, `TC-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-limit-inputValidation-min-v1` |
| `EP-ListSchedules`, `FLD-ListSchedules-offset`, `RSP-ListSchedules-400` | `TOB-SCHEDULED-TRANSFER-EP-ListSchedules-contract` | `TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-offset-inputValidation-min` | `TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules-functional` | 2 | `TC-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-offset-inputValidation-min-v0`, `TC-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-offset-inputValidation-min-v1` |
| `EP-ListSchedules`, `FLD-ListSchedules-status`, `RSP-ListSchedules-400` | `TOB-SCHEDULED-TRANSFER-EP-ListSchedules-contract` | `TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-status-inputValidation-enum` | `TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules-functional` | 5 | `TC-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-status-inputValidation-enum-v0`, `TC-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-status-inputValidation-enum-v1`, `TC-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-status-inputValidation-enum-v2`, `TC-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-status-inputValidation-enum-v3`, `TC-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-status-inputValidation-enum-v4` |
| `EP-UpdateScheduleStatus`, `FLD-Patch-Authorization`, `RSP-Patch-401` | `TOB-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-contract` | `TCND-SCHEDULED-TRANSFER-FLD-Patch-Authorization-inputValidation-missing` | `TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FLD-Patch-Authorization-inputValidation-missing-v0` |
| `EP-UpdateScheduleStatus`, `FLD-Patch-scheduleId` | `TOB-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-contract` | `TCND-SCHEDULED-TRANSFER-FLD-Patch-scheduleId-inputValidation-missing` | `TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FLD-Patch-scheduleId-inputValidation-missing-v0` |
| `EP-UpdateScheduleStatus`, `FLD-Patch-status`, `RSP-Patch-400` | `TOB-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-contract` | `TCND-SCHEDULED-TRANSFER-FLD-Patch-status-inputValidation-enum` | `TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-functional` | 3 | `TC-L2-SCHEDULED-TRANSFER-FLD-Patch-status-inputValidation-enum-v0`, `TC-L2-SCHEDULED-TRANSFER-FLD-Patch-status-inputValidation-enum-v1`, `TC-L2-SCHEDULED-TRANSFER-FLD-Patch-status-inputValidation-enum-v2` |
| `EP-UpdateScheduleStatus`, `FLD-Patch-status`, `RSP-Patch-400` | `TOB-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-contract` | `TCND-SCHEDULED-TRANSFER-FLD-Patch-status-inputValidation-missing` | `TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FLD-Patch-status-inputValidation-missing-v0` |
| `EP-CreateSchedule`, `FR-040`, `RSP-Create-401` | `TOB-SCHEDULED-TRANSFER-FR-040-proposal` | `TCND-SCHEDULED-TRANSFER-FR-040-access-p4f809f` | `TR-SCHEDULED-TRANSFER-L2-FR-040-security` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-040-access-p4f809f-c` |
| `EP-CreateSchedule`, `FR-040`, `FR-048` | `TOB-SCHEDULED-TRANSFER-FR-040-proposal` | `TCND-SCHEDULED-TRANSFER-FR-040-audit-p7a419a` | `TR-SCHEDULED-TRANSFER-L2-FR-040-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-040-audit-p7a419a-c` |
| `AC-FR-040-6`, `DE-ScheduledTransfer`, `FR-040`, `RSP-Create-422-limit` | `TOB-SCHEDULED-TRANSFER-FR-040-proposal` | `TCND-SCHEDULED-TRANSFER-FR-040-concurrency-pd7d28d` | `TR-SCHEDULED-TRANSFER-L2-FR-040-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-040-concurrency-pd7d28d-c` |
| `EP-CreateSchedule`, `FLD-Create-amount`, `FR-040`, `RSP-Create-422-amount`, `RSP-Create-422-same` | `TOB-SCHEDULED-TRANSFER-FR-040-proposal` | `TCND-SCHEDULED-TRANSFER-FR-040-decisionRule-pc6e1a6` | `TR-SCHEDULED-TRANSFER-L2-FR-040-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-040-decisionRule-pc6e1a6-c` |
| `EP-CreateSchedule`, `FR-040`, `RSP-Create-200` | `TOB-SCHEDULED-TRANSFER-FR-040-proposal` | `TCND-SCHEDULED-TRANSFER-FR-040-idempotency-p8f0346` | `TR-SCHEDULED-TRANSFER-L2-FR-040-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-040-idempotency-p8f0346-c` |
| `EP-CreateSchedule`, `FR-040`, `RSP-Create-409` | `TOB-SCHEDULED-TRANSFER-FR-040-proposal` | `TCND-SCHEDULED-TRANSFER-FR-040-idempotency-pccb8db` | `TR-SCHEDULED-TRANSFER-L2-FR-040-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-040-idempotency-pccb8db-c` |
| `EP-CreateSchedule`, `FLD-Create-amount`, `FLD-Create-maxRuns`, `FLD-Create-recurrence`, `FR-040`, `RSP-Create-400` | `TOB-SCHEDULED-TRANSFER-FR-040-proposal` | `TCND-SCHEDULED-TRANSFER-FR-040-inputValidation-p49f012` | `TR-SCHEDULED-TRANSFER-L2-FR-040-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-040-inputValidation-p49f012-c` |
| `FLD-Create-Authorization`, `FR-041`, `RSP-Create-401`, `RSP-Create-403` | `TOB-SCHEDULED-TRANSFER-FR-041-proposal` | `TCND-SCHEDULED-TRANSFER-FR-041-access-pfae77d` | `TR-SCHEDULED-TRANSFER-L2-FR-041-security` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-041-access-pfae77d-c` |
| `FR-041`, `FR-048`, `MOD-Audit` | `TOB-SCHEDULED-TRANSFER-FR-041-proposal` | `TCND-SCHEDULED-TRANSFER-FR-041-audit-p1cd2c4` | `TR-SCHEDULED-TRANSFER-L2-FR-041-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-041-audit-p1cd2c4-c` |
| `FLD-Create-endDate`, `FR-041`, `RSP-Create-422-time` | `TOB-SCHEDULED-TRANSFER-FR-041-proposal` | `TCND-SCHEDULED-TRANSFER-FR-041-guardViolation-p241776` | `TR-SCHEDULED-TRANSFER-L2-FR-041-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-041-guardViolation-p241776-c` |
| `DE-ScheduledTransfer`, `FR-041`, `RSP-Create-422-limit` | `TOB-SCHEDULED-TRANSFER-FR-041-proposal` | `TCND-SCHEDULED-TRANSFER-FR-041-guardViolation-pcbab3f` | `TR-SCHEDULED-TRANSFER-L2-FR-041-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-041-guardViolation-pcbab3f-c` |
| `FLD-Create-IdempotencyKey`, `FR-041`, `RSP-Create-200`, `RSP-Create-409` | `TOB-SCHEDULED-TRANSFER-FR-041-proposal` | `TCND-SCHEDULED-TRANSFER-FR-041-idempotency-pd36026` | `TR-SCHEDULED-TRANSFER-L2-FR-041-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-041-idempotency-pd36026-c` |
| `FLD-Create-maxRuns`, `FR-041`, `RSP-Create-400` | `TOB-SCHEDULED-TRANSFER-FR-041-proposal` | `TCND-SCHEDULED-TRANSFER-FR-041-inputValidation-p31ec2a` | `TR-SCHEDULED-TRANSFER-L2-FR-041-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-041-inputValidation-p31ec2a-c` |
| `EP-GetSchedule`, `EP-ListRuns`, `EP-ListSchedules`, `FR-042`, `RSP-Get-401`, `RSP-ListRuns-401`, `RSP-ListSchedules-401` | `TOB-SCHEDULED-TRANSFER-FR-042-proposal` | `TCND-SCHEDULED-TRANSFER-FR-042-access-p2aa8d6` | `TR-SCHEDULED-TRANSFER-L2-FR-042-security` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-042-access-p2aa8d6-c` |
| `EP-ListSchedules`, `FR-042`, `RSP-ListSchedules-403` | `TOB-SCHEDULED-TRANSFER-FR-042-proposal` | `TCND-SCHEDULED-TRANSFER-FR-042-access-p72e502` | `TR-SCHEDULED-TRANSFER-L2-FR-042-security` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-042-access-p72e502-c` |
| `FLD-ListRuns-limit`, `FLD-ListRuns-offset`, `FLD-ListSchedules-limit`, `FLD-ListSchedules-offset`, `FR-042` | `TOB-SCHEDULED-TRANSFER-FR-042-proposal` | `TCND-SCHEDULED-TRANSFER-FR-042-inputValidation-p433493` | `TR-SCHEDULED-TRANSFER-L2-FR-042-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-042-inputValidation-p433493-c` |
| `EP-ListSchedules`, `FLD-ListSchedules-status`, `FR-042`, `RSP-ListSchedules-400` | `TOB-SCHEDULED-TRANSFER-FR-042-proposal` | `TCND-SCHEDULED-TRANSFER-FR-042-inputValidation-p473406` | `TR-SCHEDULED-TRANSFER-L2-FR-042-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-042-inputValidation-p473406-c` |
| `EP-CancelSchedule`, `FR-043`, `RSP-Cancel-401` | `TOB-SCHEDULED-TRANSFER-FR-043-proposal` | `TCND-SCHEDULED-TRANSFER-FR-043-access-p4bf0be` | `TR-SCHEDULED-TRANSFER-L2-FR-043-security` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-043-access-p4bf0be-c` |
| `EP-CancelSchedule`, `FR-043`, `RSP-Cancel-403` | `TOB-SCHEDULED-TRANSFER-FR-043-proposal` | `TCND-SCHEDULED-TRANSFER-FR-043-access-pb30d3e` | `TR-SCHEDULED-TRANSFER-L2-FR-043-security` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-043-access-pb30d3e-c` |
| `EP-CancelSchedule`, `FR-043`, `FR-048`, `MOD-Audit` | `TOB-SCHEDULED-TRANSFER-FR-043-proposal` | `TCND-SCHEDULED-TRANSFER-FR-043-audit-p1438e9` | `TR-SCHEDULED-TRANSFER-L2-FR-043-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-043-audit-p1438e9-c` |
| `FR-043`, `MOD-Scheduler`, `RISK-005`, `SCN-ScheduledRun` | `TOB-SCHEDULED-TRANSFER-FR-043-proposal` | `TCND-SCHEDULED-TRANSFER-FR-043-concurrency-p4b4873` | `TR-SCHEDULED-TRANSFER-L2-FR-043-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-043-concurrency-p4b4873-c` |
| `FLD-Cancel-scheduleId`, `FR-043`, `RSP-Cancel-404` | `TOB-SCHEDULED-TRANSFER-FR-043-proposal` | `TCND-SCHEDULED-TRANSFER-FR-043-errorHandling-pa62ef8` | `TR-SCHEDULED-TRANSFER-L2-FR-043-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-043-errorHandling-pa62ef8-c` |
| `EP-UpdateScheduleStatus`, `FR-044`, `RSP-Patch-403` | `TOB-SCHEDULED-TRANSFER-FR-044-proposal` | `TCND-SCHEDULED-TRANSFER-FR-044-access-p3b5af1` | `TR-SCHEDULED-TRANSFER-L2-FR-044-security` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-044-access-p3b5af1-c` |
| `FLD-Patch-Authorization`, `FR-044`, `RSP-Patch-401` | `TOB-SCHEDULED-TRANSFER-FR-044-proposal` | `TCND-SCHEDULED-TRANSFER-FR-044-access-pf4b5f6` | `TR-SCHEDULED-TRANSFER-L2-FR-044-security` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-044-access-pf4b5f6-c` |
| `FR-044`, `FR-048`, `MOD-Audit` | `TOB-SCHEDULED-TRANSFER-FR-044-proposal` | `TCND-SCHEDULED-TRANSFER-FR-044-audit-pafc81c` | `TR-SCHEDULED-TRANSFER-L2-FR-044-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-044-audit-pafc81c-c` |
| `CON-010`, `FR-044`, `MOD-Scheduler`, `RISK-005` | `TOB-SCHEDULED-TRANSFER-FR-044-proposal` | `TCND-SCHEDULED-TRANSFER-FR-044-concurrency-p5afd1f` | `TR-SCHEDULED-TRANSFER-L2-FR-044-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-044-concurrency-p5afd1f-c` |
| `FLD-Patch-scheduleId`, `FR-044`, `RSP-Patch-404` | `TOB-SCHEDULED-TRANSFER-FR-044-proposal` | `TCND-SCHEDULED-TRANSFER-FR-044-errorHandling-pb66379` | `TR-SCHEDULED-TRANSFER-L2-FR-044-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-044-errorHandling-pb66379-c` |
| `FLD-Patch-status`, `FR-044`, `RSP-Patch-400` | `TOB-SCHEDULED-TRANSFER-FR-044-proposal` | `TCND-SCHEDULED-TRANSFER-FR-044-inputValidation-p847ccb` | `TR-SCHEDULED-TRANSFER-L2-FR-044-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-044-inputValidation-p847ccb-c` |
| `FR-045`, `MECH-RunClaim`, `NFR-011`, `RISK-005` | `TOB-SCHEDULED-TRANSFER-FR-045-proposal` | `TCND-SCHEDULED-TRANSFER-FR-045-concurrency-p174b2f` | `TR-SCHEDULED-TRANSFER-L2-FR-045-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-045-concurrency-p174b2f-c` |
| `FR-045`, `INT-01`, `NFR-014`, `SCN-ScheduledRun` | `TOB-SCHEDULED-TRANSFER-FR-045-proposal` | `TCND-SCHEDULED-TRANSFER-FR-045-observability-pd8cb5a` | `TR-SCHEDULED-TRANSFER-L2-FR-045-maintainability` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-045-observability-pd8cb5a-c` |
| `FR-045`, `NFR-010`, `SCN-ScheduledRun` | `TOB-SCHEDULED-TRANSFER-FR-045-proposal` | `TCND-SCHEDULED-TRANSFER-FR-045-timing-pb0682f` | `TR-SCHEDULED-TRANSFER-L2-FR-045-performance` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-045-timing-pb0682f-c` |
| `FR-047`, `INT-02`, `INT-03`, `SCN-Notify` | `TOB-SCHEDULED-TRANSFER-FR-047-proposal` | `TCND-SCHEDULED-TRANSFER-FR-047-idempotency-pd600b3` | `TR-SCHEDULED-TRANSFER-L2-FR-047-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-047-idempotency-pd600b3-c` |
| `FR-048`, `RSP-Patch-403`, `RSP-Patch-409` | `TOB-SCHEDULED-TRANSFER-FR-048-proposal` | `TCND-SCHEDULED-TRANSFER-FR-048-guardViolation-pf87b1a` | `TR-SCHEDULED-TRANSFER-L2-FR-048-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-048-guardViolation-pf87b1a-c` |
| `DE-AuditLog`, `FR-048` | `TOB-SCHEDULED-TRANSFER-FR-048-proposal` | `TCND-SCHEDULED-TRANSFER-FR-048-invariant-pdabb36` | `TR-SCHEDULED-TRANSFER-L2-FR-048-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-048-invariant-pdabb36-c` |
| `FR-048`, `NFR-014`, `SCN-ScheduledRun` | `TOB-SCHEDULED-TRANSFER-FR-048-proposal` | `TCND-SCHEDULED-TRANSFER-FR-048-observability-p477475` | `TR-SCHEDULED-TRANSFER-L2-FR-048-maintainability` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-048-observability-p477475-c` |
| `FR-049`, `NFR-011`, `RISK-005` | `TOB-SCHEDULED-TRANSFER-FR-049-proposal` | `TCND-SCHEDULED-TRANSFER-FR-049-concurrency-pe3c163` | `TR-SCHEDULED-TRANSFER-L2-FR-049-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-FR-049-concurrency-pe3c163-c` |
| `AC-FR-049-1`, `FR-049`, `MECH-CatchUp`, `MOD-Scheduler` | `TOB-SCHEDULED-TRANSFER-MECH-CatchUp-proposal` | `TCND-SCHEDULED-TRANSFER-MECH-CatchUp-idempotency-p602a3c` | `TR-SCHEDULED-TRANSFER-L3-MECH-CatchUp-reliability` | 1 | `TC-L3-SCHEDULED-TRANSFER-MECH-CatchUp-idempotency-p602a3c-c` |
| `AC-FR-049-1`, `AC-FR-049-2`, `AC-NFR-012-1`, `FR-049`, `MECH-CatchUp`, `NFR-012` | `TOB-SCHEDULED-TRANSFER-MECH-CatchUp-mechanism` | `TCND-SCHEDULED-TRANSFER-MECH-CatchUp-resilience-holds` | `TR-SCHEDULED-TRANSFER-L3-MECH-CatchUp-reliability` | 1 | `TC-L3-SCHEDULED-TRANSFER-MECH-CatchUp-resilience-holds-c` |
| `AC-NFR-011-1`, `FR-045`, `MECH-RunClaim`, `NFR-011`, `TC-S10` | `TOB-SCHEDULED-TRANSFER-MECH-RunClaim-mechanism` | `TCND-SCHEDULED-TRANSFER-MECH-RunClaim-concurrency-parallel` | `TR-SCHEDULED-TRANSFER-L3-MECH-RunClaim-reliability` | 1 | `TC-L3-SCHEDULED-TRANSFER-MECH-RunClaim-concurrency-parallel-c` |
| `AC-NFR-011-2`, `FR-045`, `MECH-RunClaim`, `NFR-011`, `TC-S11` | `TOB-SCHEDULED-TRANSFER-MECH-RunClaim-mechanism` | `TCND-SCHEDULED-TRANSFER-MECH-RunClaim-resilience-crash-mid` | `TR-SCHEDULED-TRANSFER-L3-MECH-RunClaim-reliability` | 1 | `TC-L3-SCHEDULED-TRANSFER-MECH-RunClaim-resilience-crash-mid-c` |
| `AC-FR-046-1`, `AC-FR-046-2`, `AC-FR-046-3`, `AC-FR-046-4`, `FR-046`, `MECH-ScheduleRetry` | `TOB-SCHEDULED-TRANSFER-MECH-ScheduleRetry-mechanism` | `TCND-SCHEDULED-TRANSFER-MECH-ScheduleRetry-resilience-holds` | `TR-SCHEDULED-TRANSFER-L3-MECH-ScheduleRetry-reliability` | 1 | `TC-L3-SCHEDULED-TRANSFER-MECH-ScheduleRetry-resilience-holds-c` |
| `AC-NFR-013-2`, `MECH-SchedulerThrottle`, `NFR-013` | `TOB-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-mechanism` | `TCND-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-performance-at-limit` | `TR-SCHEDULED-TRANSFER-L3-MECH-SchedulerThrottle-performance` | 1 | `TC-L3-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-performance-at-limit-c` |
| `AC-NFR-013-1`, `AC-NFR-013-2`, `MECH-SchedulerThrottle`, `NFR-013`, `TC-S13` | `TOB-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-mechanism` | `TCND-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-performance-over-limit` | `TR-SCHEDULED-TRANSFER-L3-MECH-SchedulerThrottle-performance` | 1 | `TC-L3-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-performance-over-limit-c` |
| `AC-FR-045-2`, `FR-045`, `MOD-Scheduler`, `RISK-005` | `TOB-SCHEDULED-TRANSFER-MOD-Scheduler-proposal` | `TCND-SCHEDULED-TRANSFER-MOD-Scheduler-concurrency-p933b8e` | `TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler-reliability` | 1 | `TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-concurrency-p933b8e-c` |
| `AC-FR-046-3`, `MOD-Account`, `MOD-Scheduler` | `TOB-SCHEDULED-TRANSFER-MOD-Scheduler-deps` | `TCND-SCHEDULED-TRANSFER-MOD-Scheduler-errorHandling-dep-MOD-Account` | `TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler-reliability` | 1 | `TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-errorHandling-dep-MOD-Account-c` |
| `FR-048`, `MOD-Audit`, `MOD-Scheduler` | `TOB-SCHEDULED-TRANSFER-MOD-Scheduler-deps` | `TCND-SCHEDULED-TRANSFER-MOD-Scheduler-errorHandling-dep-MOD-Audit` | `TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler-reliability` | 1 | `TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-errorHandling-dep-MOD-Audit-c` |
| `AC-FR-046-1`, `ERR-INT01-4xx`, `ERR-INT01-5xx`, `ERR-INT01-schema`, `ERR-INT01-timeout`, `FB-INT01`, `INT-01`, `MOD-Risk`, `MOD-Scheduler`, `RT-INT01`, `TO-INT01` | `TOB-SCHEDULED-TRANSFER-MOD-Scheduler-deps` | `TCND-SCHEDULED-TRANSFER-MOD-Scheduler-errorHandling-dep-MOD-Risk` | `TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler-reliability` | 1 | `TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-errorHandling-dep-MOD-Risk-c` |
| `AC-FR-046-2`, `AC-FR-046-3`, `AC-FR-046-4`, `FR-046`, `MOD-Scheduler`, `MOD-Transfer` | `TOB-SCHEDULED-TRANSFER-MOD-Scheduler-deps` | `TCND-SCHEDULED-TRANSFER-MOD-Scheduler-errorHandling-dep-MOD-Transfer` | `TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler-reliability` | 1 | `TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-errorHandling-dep-MOD-Transfer-c` |
| `AC-NFR-010-1`, `MOD-Scheduler`, `NFR-010`, `RISK-005`, `SCN-ScheduledRun` | `TOB-SCHEDULED-TRANSFER-MOD-Scheduler-proposal` | `TCND-SCHEDULED-TRANSFER-MOD-Scheduler-timing-p077a10` | `TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler-performance` | 1 | `TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-timing-p077a10-c` |
| `MOD-Scheduler`, `NFR-010`, `RISK-005`, `SCN-ScheduledRun` | `TOB-SCHEDULED-TRANSFER-NFR-010-proposal` | `TCND-SCHEDULED-TRANSFER-NFR-010-timing-p9746f5` | `TR-SCHEDULED-TRANSFER-L2-NFR-010-performance` | 1 | `TC-L2-SCHEDULED-TRANSFER-NFR-010-timing-p9746f5-c` |
| `AD-05`, `ASM-010`, `NFR-010`, `TC-S09` | `TOB-SCHEDULED-TRANSFER-NFR-010-nfr` | `TCND-SCHEDULED-TRANSFER-NFR-010-timing-target` | `TR-SCHEDULED-TRANSFER-L2-NFR-010-performance` | 1 | `TC-L2-SCHEDULED-TRANSFER-NFR-010-timing-target-c` |
| `MECH-RunClaim`, `NFR-011`, `RISK-005` | `TOB-SCHEDULED-TRANSFER-NFR-011-nfr` | `TCND-SCHEDULED-TRANSFER-NFR-011-concurrency-target` | `TR-SCHEDULED-TRANSFER-L2-NFR-011-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-NFR-011-concurrency-target-c` |
| `FR-045`, `MOD-Scheduler`, `NFR-011`, `NFR-012` | `TOB-SCHEDULED-TRANSFER-NFR-012-proposal` | `TCND-SCHEDULED-TRANSFER-NFR-012-concurrency-p92271d` | `TR-SCHEDULED-TRANSFER-L2-NFR-012-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-NFR-012-concurrency-p92271d-c` |
| `FR-049`, `MECH-CatchUp`, `NFR-012`, `SCN-ScheduledRun` | `TOB-SCHEDULED-TRANSFER-NFR-012-nfr` | `TCND-SCHEDULED-TRANSFER-NFR-012-resilience-target` | `TR-SCHEDULED-TRANSFER-L2-NFR-012-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-NFR-012-resilience-target-c` |
| `FR-049`, `MECH-CatchUp`, `NFR-012` | `TOB-SCHEDULED-TRANSFER-NFR-012-proposal` | `TCND-SCHEDULED-TRANSFER-NFR-012-timing-pe0788e` | `TR-SCHEDULED-TRANSFER-L2-NFR-012-performance` | 1 | `TC-L2-SCHEDULED-TRANSFER-NFR-012-timing-pe0788e-c` |
| `NFR-011`, `NFR-013`, `RISK-005` | `TOB-SCHEDULED-TRANSFER-NFR-013-proposal` | `TCND-SCHEDULED-TRANSFER-NFR-013-concurrency-p4dc0d8` | `TR-SCHEDULED-TRANSFER-L2-NFR-013-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-NFR-013-concurrency-p4dc0d8-c` |
| `MECH-SchedulerThrottle`, `NFR-013`, `RISK-005` | `TOB-SCHEDULED-TRANSFER-NFR-013-proposal` | `TCND-SCHEDULED-TRANSFER-NFR-013-performance-pd5ac38` | `TR-SCHEDULED-TRANSFER-L2-NFR-013-performance` | 1 | `TC-L2-SCHEDULED-TRANSFER-NFR-013-performance-pd5ac38-c` |
| `MECH-SchedulerThrottle`, `NFR-013`, `TC-S13` | `TOB-SCHEDULED-TRANSFER-NFR-013-nfr` | `TCND-SCHEDULED-TRANSFER-NFR-013-performance-target` | `TR-SCHEDULED-TRANSFER-L2-NFR-013-performance` | 1 | `TC-L2-SCHEDULED-TRANSFER-NFR-013-performance-target-c` |
| `NFR-014`, `SCN-ScheduledRun` | `TOB-SCHEDULED-TRANSFER-NFR-014-proposal` | `TCND-SCHEDULED-TRANSFER-NFR-014-observability-p8ac829` | `TR-SCHEDULED-TRANSFER-L2-NFR-014-maintainability` | 1 | `TC-L2-SCHEDULED-TRANSFER-NFR-014-observability-p8ac829-c` |
| `INT-01`, `NFR-014`, `SCN-ScheduledRun` | `TOB-SCHEDULED-TRANSFER-NFR-014-nfr` | `TCND-SCHEDULED-TRANSFER-NFR-014-observability-target` | `TR-SCHEDULED-TRANSFER-L2-NFR-014-maintainability` | 1 | `TC-L2-SCHEDULED-TRANSFER-NFR-014-observability-target-c` |
| `AC-FR-043-1`, `AC-FR-043-2`, `EP-CancelSchedule`, `RSP-Cancel-200` | `TOB-SCHEDULED-TRANSFER-EP-CancelSchedule-contract` | `TCND-SCHEDULED-TRANSFER-RSP-Cancel-200-functional` | `TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-Cancel-200-functional-c` |
| `EP-CancelSchedule`, `MOD-API`, `RSP-Cancel-401` | `TOB-SCHEDULED-TRANSFER-EP-CancelSchedule-contract` | `TCND-SCHEDULED-TRANSFER-RSP-Cancel-401-access` | `TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-security` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-Cancel-401-access-c` |
| `EP-CancelSchedule`, `RSP-Cancel-403` | `TOB-SCHEDULED-TRANSFER-EP-CancelSchedule-contract` | `TCND-SCHEDULED-TRANSFER-RSP-Cancel-403-access` | `TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-security` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-Cancel-403-access-c` |
| `EP-CancelSchedule`, `RSP-Cancel-404` | `TOB-SCHEDULED-TRANSFER-EP-CancelSchedule-contract` | `TCND-SCHEDULED-TRANSFER-RSP-Cancel-404-inputValidation` | `TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-Cancel-404-inputValidation-c` |
| `AC-FR-043-3`, `EP-CancelSchedule`, `RSP-Cancel-409` | `TOB-SCHEDULED-TRANSFER-EP-CancelSchedule-contract` | `TCND-SCHEDULED-TRANSFER-RSP-Cancel-409-inputValidation` | `TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-Cancel-409-inputValidation-c` |
| `EP-CreateSchedule`, `RSP-Create-200` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` | `TCND-SCHEDULED-TRANSFER-RSP-Create-200-functional` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-Create-200-functional-c` |
| `AC-FR-040-1`, `AC-FR-040-2`, `EP-CreateSchedule`, `RSP-Create-201` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` | `TCND-SCHEDULED-TRANSFER-RSP-Create-201-functional` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-Create-201-functional-c` |
| `AC-FR-041-2`, `EP-CreateSchedule`, `RSP-Create-400` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` | `TCND-SCHEDULED-TRANSFER-RSP-Create-400-inputValidation` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-Create-400-inputValidation-c` |
| `EP-CreateSchedule`, `FLD-Create-Authorization`, `RSP-Create-401` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` | `TCND-SCHEDULED-TRANSFER-RSP-Create-401-access` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-security` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-Create-401-access-c` |
| `AC-FR-040-5`, `EP-CreateSchedule`, `FLD-Create-fromAccountId`, `RSP-Create-403` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` | `TCND-SCHEDULED-TRANSFER-RSP-Create-403-access` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-security` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-Create-403-access-c` |
| `AC-FR-040-4`, `EP-CreateSchedule`, `FLD-Create-toAccountId`, `RSP-Create-404` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` | `TCND-SCHEDULED-TRANSFER-RSP-Create-404-inputValidation` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-Create-404-inputValidation-c` |
| `EP-CreateSchedule`, `FLD-Create-IdempotencyKey`, `RSP-Create-200`, `RSP-Create-409` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` | `TCND-SCHEDULED-TRANSFER-RSP-Create-409-inputValidation` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-Create-409-inputValidation-c` |
| `EP-CreateSchedule`, `FLD-Create-amount`, `RSP-Create-422-amount` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` | `TCND-SCHEDULED-TRANSFER-RSP-Create-422-amount-inputValidation` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-Create-422-amount-inputValidation-c` |
| `AC-FR-040-6`, `EP-CreateSchedule`, `RSP-Create-422-limit` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` | `TCND-SCHEDULED-TRANSFER-RSP-Create-422-limit-inputValidation` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-Create-422-limit-inputValidation-c` |
| `EP-CreateSchedule`, `FLD-Create-fromAccountId`, `FLD-Create-toAccountId`, `RSP-Create-422-same` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` | `TCND-SCHEDULED-TRANSFER-RSP-Create-422-same-inputValidation` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-Create-422-same-inputValidation-c` |
| `AC-FR-040-3`, `EP-CreateSchedule`, `FLD-Create-endDate`, `FLD-Create-firstRunAt`, `RSP-Create-422-time` | `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` | `TCND-SCHEDULED-TRANSFER-RSP-Create-422-time-inputValidation` | `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-Create-422-time-inputValidation-c` |
| `AC-FR-042-1`, `EP-GetSchedule`, `RSP-Get-200` | `TOB-SCHEDULED-TRANSFER-EP-GetSchedule-contract` | `TCND-SCHEDULED-TRANSFER-RSP-Get-200-functional` | `TR-SCHEDULED-TRANSFER-L2-EP-GetSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-Get-200-functional-c` |
| `EP-GetSchedule`, `MOD-API`, `RSP-Get-401` | `TOB-SCHEDULED-TRANSFER-EP-GetSchedule-contract` | `TCND-SCHEDULED-TRANSFER-RSP-Get-401-access` | `TR-SCHEDULED-TRANSFER-L2-EP-GetSchedule-security` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-Get-401-access-c` |
| `AC-FR-042-4`, `EP-GetSchedule`, `RSP-Get-403` | `TOB-SCHEDULED-TRANSFER-EP-GetSchedule-contract` | `TCND-SCHEDULED-TRANSFER-RSP-Get-403-access` | `TR-SCHEDULED-TRANSFER-L2-EP-GetSchedule-security` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-Get-403-access-c` |
| `AC-FR-042-4`, `EP-GetSchedule`, `RSP-Get-404` | `TOB-SCHEDULED-TRANSFER-EP-GetSchedule-contract` | `TCND-SCHEDULED-TRANSFER-RSP-Get-404-inputValidation` | `TR-SCHEDULED-TRANSFER-L2-EP-GetSchedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-Get-404-inputValidation-c` |
| `AC-FR-042-3`, `EP-ListRuns`, `FR-042`, `RSP-ListRuns-200` | `TOB-SCHEDULED-TRANSFER-EP-ListRuns-contract` | `TCND-SCHEDULED-TRANSFER-RSP-ListRuns-200-functional` | `TR-SCHEDULED-TRANSFER-L2-EP-ListRuns-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-ListRuns-200-functional-c` |
| `EP-ListRuns`, `MOD-API`, `RSP-ListRuns-401` | `TOB-SCHEDULED-TRANSFER-EP-ListRuns-contract` | `TCND-SCHEDULED-TRANSFER-RSP-ListRuns-401-access` | `TR-SCHEDULED-TRANSFER-L2-EP-ListRuns-security` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-ListRuns-401-access-c` |
| `AC-FR-042-4`, `EP-ListRuns`, `RSP-ListRuns-403` | `TOB-SCHEDULED-TRANSFER-EP-ListRuns-contract` | `TCND-SCHEDULED-TRANSFER-RSP-ListRuns-403-access` | `TR-SCHEDULED-TRANSFER-L2-EP-ListRuns-security` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-ListRuns-403-access-c` |
| `AC-FR-042-4`, `EP-ListRuns`, `RSP-ListRuns-404` | `TOB-SCHEDULED-TRANSFER-EP-ListRuns-contract` | `TCND-SCHEDULED-TRANSFER-RSP-ListRuns-404-inputValidation` | `TR-SCHEDULED-TRANSFER-L2-EP-ListRuns-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-ListRuns-404-inputValidation-c` |
| `AC-FR-042-2`, `EP-ListSchedules`, `RSP-ListSchedules-200` | `TOB-SCHEDULED-TRANSFER-EP-ListSchedules-contract` | `TCND-SCHEDULED-TRANSFER-RSP-ListSchedules-200-functional` | `TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-ListSchedules-200-functional-c` |
| `EP-ListSchedules`, `RSP-ListSchedules-400` | `TOB-SCHEDULED-TRANSFER-EP-ListSchedules-contract` | `TCND-SCHEDULED-TRANSFER-RSP-ListSchedules-400-inputValidation` | `TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-ListSchedules-400-inputValidation-c` |
| `EP-ListSchedules`, `RSP-ListSchedules-401` | `TOB-SCHEDULED-TRANSFER-EP-ListSchedules-contract` | `TCND-SCHEDULED-TRANSFER-RSP-ListSchedules-401-access` | `TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules-security` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-ListSchedules-401-access-c` |
| `AC-FR-042-4`, `EP-ListSchedules`, `RSP-ListSchedules-403` | `TOB-SCHEDULED-TRANSFER-EP-ListSchedules-contract` | `TCND-SCHEDULED-TRANSFER-RSP-ListSchedules-403-access` | `TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules-security` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-ListSchedules-403-access-c` |
| `AC-FR-044-1`, `AC-FR-044-2`, `EP-UpdateScheduleStatus`, `RSP-Patch-200` | `TOB-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-contract` | `TCND-SCHEDULED-TRANSFER-RSP-Patch-200-functional` | `TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-Patch-200-functional-c` |
| `EP-UpdateScheduleStatus`, `RSP-Patch-400` | `TOB-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-contract` | `TCND-SCHEDULED-TRANSFER-RSP-Patch-400-inputValidation` | `TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-Patch-400-inputValidation-c` |
| `EP-UpdateScheduleStatus`, `RSP-Patch-401` | `TOB-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-contract` | `TCND-SCHEDULED-TRANSFER-RSP-Patch-401-access` | `TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-security` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-Patch-401-access-c` |
| `EP-UpdateScheduleStatus`, `RSP-Patch-403` | `TOB-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-contract` | `TCND-SCHEDULED-TRANSFER-RSP-Patch-403-access` | `TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-security` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-Patch-403-access-c` |
| `EP-UpdateScheduleStatus`, `RSP-Patch-404` | `TOB-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-contract` | `TCND-SCHEDULED-TRANSFER-RSP-Patch-404-inputValidation` | `TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-Patch-404-inputValidation-c` |
| `AC-FR-044-3`, `EP-UpdateScheduleStatus`, `RSP-Patch-409` | `TOB-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-contract` | `TCND-SCHEDULED-TRANSFER-RSP-Patch-409-inputValidation` | `TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-RSP-Patch-409-inputValidation-c` |
| `AC-FR-045-2`, `AC-NFR-011-2`, `NFR-011`, `SCN-ScheduledRun` | `TOB-SCHEDULED-TRANSFER-SCN-ScheduledRun-proposal` | `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-atomicity-p12901f` | `TR-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-SCN-ScheduledRun-atomicity-p12901f-c` |
| `AC-NFR-011-1`, `MECH-RunClaim`, `NFR-011`, `SCN-ScheduledRun` | `TOB-SCHEDULED-TRANSFER-SCN-ScheduledRun-proposal` | `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-concurrency-p775a37` | `TR-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-SCN-ScheduledRun-concurrency-p775a37-c` |
| `AC-FR-046-1`, `ERR-INT01-4xx`, `ERR-INT01-5xx`, `ERR-INT01-schema`, `ERR-INT01-timeout`, `FB-INT01`, `INT-01`, `SCN-ScheduledRun` | `TOB-SCHEDULED-TRANSFER-SCN-ScheduledRun-flow` | `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-errorHandling-fail-INT-01` | `TR-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-reliability` | 1 | `TC-L1-SCHEDULED-TRANSFER-SCN-ScheduledRun-errorHandling-fail-INT-01-c` |
| `AC-FR-047-3`, `FR-047`, `MECH-Outbox`, `SCN-ScheduledRun` | `TOB-SCHEDULED-TRANSFER-SCN-ScheduledRun-proposal` | `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-errorHandling-p00fe49` | `TR-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-SCN-ScheduledRun-errorHandling-p00fe49-c` |
| `AC-FR-045-1`, `AC-FR-045-2`, `AC-FR-047-1`, `AC-FR-048-2`, `MECH-Outbox`, `MECH-RunClaim`, `MOD-API`, `MOD-Scheduler`, `MOD-Transfer`, `SCN-ScheduledRun` | `TOB-SCHEDULED-TRANSFER-SCN-ScheduledRun-flow` | `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-functional-main` | `TR-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-functional` | 1 | `TC-L1-SCHEDULED-TRANSFER-SCN-ScheduledRun-functional-main-c` |
| `AC-NFR-014-1`, `AC-NFR-014-2`, `MECH-Correlation`, `NFR-014`, `SCN-ScheduledRun` | `TOB-SCHEDULED-TRANSFER-SCN-ScheduledRun-proposal` | `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-observability-p12d52f` | `TR-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-maintainability` | 1 | `TC-L2-SCHEDULED-TRANSFER-SCN-ScheduledRun-observability-p12d52f-c` |
| `AC-NFR-013-1`, `AC-NFR-013-2`, `MECH-SchedulerThrottle`, `NFR-013`, `SCN-ScheduledRun` | `TOB-SCHEDULED-TRANSFER-SCN-ScheduledRun-proposal` | `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-performance-pa101d9` | `TR-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-performance` | 1 | `TC-L2-SCHEDULED-TRANSFER-SCN-ScheduledRun-performance-pa101d9-c` |
| `AC-FR-049-1`, `AC-FR-049-2`, `AC-NFR-012-1`, `FR-049`, `MECH-CatchUp`, `NFR-012`, `SCN-ScheduledRun` | `TOB-SCHEDULED-TRANSFER-SCN-ScheduledRun-proposal` | `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-stateTransition-pd98b3f` | `TR-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-SCN-ScheduledRun-stateTransition-pd98b3f-c` |
| `AC-FR-046-3`, `AC-FR-046-4`, `AC-FR-047-2`, `FR-046`, `MECH-ScheduleRetry`, `SCN-ScheduledRun` | `TOB-SCHEDULED-TRANSFER-SCN-ScheduledRun-proposal` | `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-stateTransition-pfec2a2` | `TR-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-SCN-ScheduledRun-stateTransition-pfec2a2-c` |
| `AD-05`, `NFR-010`, `SCN-ScheduledRun` | `TOB-SCHEDULED-TRANSFER-SCN-ScheduledRun-flow` | `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-timing-budget` | `TR-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-performance` | 1 | `TC-L1-SCHEDULED-TRANSFER-SCN-ScheduledRun-timing-budget-c` |
| `INT-01`, `RT-INT01`, `SCN-ScheduledRun`, `TO-INT01` | `TOB-SCHEDULED-TRANSFER-SCN-ScheduledRun-flow` | `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-timing-worst-INT-01` | `TR-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-performance` | 1 | `TC-L1-SCHEDULED-TRANSFER-SCN-ScheduledRun-timing-worst-INT-01-c` |
| `MOD-Scheduler`, `SM-Schedule` | `TOB-SCHEDULED-TRANSFER-SM-Schedule-proposal` | `TCND-SCHEDULED-TRANSFER-SM-Schedule-concurrency-p34e996` | `TR-SCHEDULED-TRANSFER-L2-SM-Schedule-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-SM-Schedule-concurrency-p34e996-c` |
| `MOD-Scheduler`, `SM-ScheduledRun`, `TRN-ScheduledRun-1` | `TOB-SCHEDULED-TRANSFER-SM-ScheduledRun-proposal` | `TCND-SCHEDULED-TRANSFER-SM-ScheduledRun-concurrency-p4b41b3` | `TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-concurrency-p4b41b3-c` |
| `SM-ScheduledRun`, `TRN-ScheduledRun-2`, `TRN-ScheduledRun-8` | `TOB-SCHEDULED-TRANSFER-SM-ScheduledRun-proposal` | `TCND-SCHEDULED-TRANSFER-SM-ScheduledRun-idempotency-p456351` | `TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-reliability` | 1 | `TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-idempotency-p456351-c` |
| `MOD-Scheduler`, `SM-ScheduledRun` | `TOB-SCHEDULED-TRANSFER-SM-ScheduledRun-proposal` | `TCND-SCHEDULED-TRANSFER-SM-ScheduledRun-performance-pa68a97` | `TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-performance` | 1 | `TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-performance-pa68a97-c` |
| `SM-Schedule`, `TRN-Schedule-1` | `TOB-SCHEDULED-TRANSFER-SM-Schedule-states` | `TCND-SCHEDULED-TRANSFER-TRN-Schedule-1-guardViolation-guard` | `TR-SCHEDULED-TRANSFER-L2-SM-Schedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-TRN-Schedule-1-guardViolation-guard-c` |
| `SM-Schedule`, `TRN-Schedule-1` | `TOB-SCHEDULED-TRANSFER-SM-Schedule-states` | `TCND-SCHEDULED-TRANSFER-TRN-Schedule-1-stateTransition` | `TR-SCHEDULED-TRANSFER-L2-SM-Schedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-TRN-Schedule-1-stateTransition-c` |
| `SM-Schedule`, `TRN-Schedule-2`, `TRN-Schedule-7` | `TOB-SCHEDULED-TRANSFER-SM-Schedule-states` | `TCND-SCHEDULED-TRANSFER-TRN-Schedule-2-guardViolation-guard` | `TR-SCHEDULED-TRANSFER-L2-SM-Schedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-TRN-Schedule-2-guardViolation-guard-c` |
| `SM-Schedule`, `TRN-Schedule-2` | `TOB-SCHEDULED-TRANSFER-SM-Schedule-states` | `TCND-SCHEDULED-TRANSFER-TRN-Schedule-2-stateTransition` | `TR-SCHEDULED-TRANSFER-L2-SM-Schedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-TRN-Schedule-2-stateTransition-c` |
| `SM-Schedule`, `TRN-Schedule-3` | `TOB-SCHEDULED-TRANSFER-SM-Schedule-states` | `TCND-SCHEDULED-TRANSFER-TRN-Schedule-3-guardViolation-guard` | `TR-SCHEDULED-TRANSFER-L2-SM-Schedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-TRN-Schedule-3-guardViolation-guard-c` |
| `SM-Schedule`, `TRN-Schedule-3` | `TOB-SCHEDULED-TRANSFER-SM-Schedule-states` | `TCND-SCHEDULED-TRANSFER-TRN-Schedule-3-stateTransition` | `TR-SCHEDULED-TRANSFER-L2-SM-Schedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-TRN-Schedule-3-stateTransition-c` |
| `SM-Schedule`, `TRN-Schedule-4` | `TOB-SCHEDULED-TRANSFER-SM-Schedule-states` | `TCND-SCHEDULED-TRANSFER-TRN-Schedule-4-guardViolation-guard` | `TR-SCHEDULED-TRANSFER-L2-SM-Schedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-TRN-Schedule-4-guardViolation-guard-c` |
| `SM-Schedule`, `TRN-Schedule-4` | `TOB-SCHEDULED-TRANSFER-SM-Schedule-states` | `TCND-SCHEDULED-TRANSFER-TRN-Schedule-4-stateTransition` | `TR-SCHEDULED-TRANSFER-L2-SM-Schedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-TRN-Schedule-4-stateTransition-c` |
| `SM-Schedule`, `TRN-Schedule-5` | `TOB-SCHEDULED-TRANSFER-SM-Schedule-states` | `TCND-SCHEDULED-TRANSFER-TRN-Schedule-5-guardViolation-guard` | `TR-SCHEDULED-TRANSFER-L2-SM-Schedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-TRN-Schedule-5-guardViolation-guard-c` |
| `SM-Schedule`, `TRN-Schedule-5` | `TOB-SCHEDULED-TRANSFER-SM-Schedule-states` | `TCND-SCHEDULED-TRANSFER-TRN-Schedule-5-stateTransition` | `TR-SCHEDULED-TRANSFER-L2-SM-Schedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-TRN-Schedule-5-stateTransition-c` |
| `SM-Schedule`, `TRN-Schedule-6` | `TOB-SCHEDULED-TRANSFER-SM-Schedule-states` | `TCND-SCHEDULED-TRANSFER-TRN-Schedule-6-guardViolation-guard` | `TR-SCHEDULED-TRANSFER-L2-SM-Schedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-TRN-Schedule-6-guardViolation-guard-c` |
| `SM-Schedule`, `TRN-Schedule-6` | `TOB-SCHEDULED-TRANSFER-SM-Schedule-states` | `TCND-SCHEDULED-TRANSFER-TRN-Schedule-6-stateTransition` | `TR-SCHEDULED-TRANSFER-L2-SM-Schedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-TRN-Schedule-6-stateTransition-c` |
| `SM-Schedule`, `TRN-Schedule-2`, `TRN-Schedule-7` | `TOB-SCHEDULED-TRANSFER-SM-Schedule-states` | `TCND-SCHEDULED-TRANSFER-TRN-Schedule-7-guardViolation-guard` | `TR-SCHEDULED-TRANSFER-L2-SM-Schedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-TRN-Schedule-7-guardViolation-guard-c` |
| `SM-Schedule`, `TRN-Schedule-7` | `TOB-SCHEDULED-TRANSFER-SM-Schedule-states` | `TCND-SCHEDULED-TRANSFER-TRN-Schedule-7-stateTransition` | `TR-SCHEDULED-TRANSFER-L2-SM-Schedule-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-TRN-Schedule-7-stateTransition-c` |
| `SM-ScheduledRun`, `TRN-ScheduledRun-1` | `TOB-SCHEDULED-TRANSFER-SM-ScheduledRun-states` | `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-1-guardViolation-guard` | `TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-TRN-ScheduledRun-1-guardViolation-guard-c` |
| `SM-ScheduledRun`, `TRN-ScheduledRun-1` | `TOB-SCHEDULED-TRANSFER-SM-ScheduledRun-states` | `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-1-stateTransition` | `TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-TRN-ScheduledRun-1-stateTransition-c` |
| `SM-ScheduledRun`, `TRN-ScheduledRun-2` | `TOB-SCHEDULED-TRANSFER-SM-ScheduledRun-states` | `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-2-stateTransition` | `TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-TRN-ScheduledRun-2-stateTransition-c` |
| `SM-ScheduledRun`, `TRN-ScheduledRun-3`, `TRN-ScheduledRun-4` | `TOB-SCHEDULED-TRANSFER-SM-ScheduledRun-states` | `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-3-guardViolation-guard` | `TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-TRN-ScheduledRun-3-guardViolation-guard-c` |
| `SM-ScheduledRun`, `TRN-ScheduledRun-3` | `TOB-SCHEDULED-TRANSFER-SM-ScheduledRun-states` | `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-3-stateTransition` | `TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-TRN-ScheduledRun-3-stateTransition-c` |
| `SM-ScheduledRun`, `TRN-ScheduledRun-4` | `TOB-SCHEDULED-TRANSFER-SM-ScheduledRun-states` | `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-4-guardViolation-guard` | `TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-TRN-ScheduledRun-4-guardViolation-guard-c` |
| `SM-ScheduledRun`, `TRN-ScheduledRun-4` | `TOB-SCHEDULED-TRANSFER-SM-ScheduledRun-states` | `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-4-stateTransition` | `TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-TRN-ScheduledRun-4-stateTransition-c` |
| `SM-ScheduledRun`, `TRN-ScheduledRun-5` | `TOB-SCHEDULED-TRANSFER-SM-ScheduledRun-states` | `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-5-guardViolation-guard` | `TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-TRN-ScheduledRun-5-guardViolation-guard-c` |
| `SM-ScheduledRun`, `TRN-ScheduledRun-5` | `TOB-SCHEDULED-TRANSFER-SM-ScheduledRun-states` | `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-5-stateTransition` | `TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-TRN-ScheduledRun-5-stateTransition-c` |
| `SM-ScheduledRun`, `TRN-ScheduledRun-6` | `TOB-SCHEDULED-TRANSFER-SM-ScheduledRun-states` | `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-6-guardViolation-guard` | `TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-TRN-ScheduledRun-6-guardViolation-guard-c` |
| `SM-ScheduledRun`, `TRN-ScheduledRun-6` | `TOB-SCHEDULED-TRANSFER-SM-ScheduledRun-states` | `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-6-stateTransition` | `TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-TRN-ScheduledRun-6-stateTransition-c` |
| `SM-ScheduledRun`, `TRN-ScheduledRun-7` | `TOB-SCHEDULED-TRANSFER-SM-ScheduledRun-states` | `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-7-stateTransition` | `TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-TRN-ScheduledRun-7-stateTransition-c` |
| `SM-ScheduledRun`, `TRN-ScheduledRun-8` | `TOB-SCHEDULED-TRANSFER-SM-ScheduledRun-states` | `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-8-guardViolation-guard` | `TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-TRN-ScheduledRun-8-guardViolation-guard-c` |
| `SM-ScheduledRun`, `TRN-ScheduledRun-8` | `TOB-SCHEDULED-TRANSFER-SM-ScheduledRun-states` | `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-8-stateTransition` | `TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-functional` | 1 | `TC-L2-SCHEDULED-TRANSFER-TRN-ScheduledRun-8-stateTransition-c` |

## 6. Khoảng trống mở (TestGap)

| Mã | Thiếu | Câu hỏi | Owner | Hạn | Chặn |
| --- | --- | --- | --- | --- | --- |
| `TGAP-SCHEDULED-TRANSFER-EP-CancelSchedule-llm-87d534` | Mã lỗi khi scheduleId thiếu hoặc sai định dạng; hợp đồng chỉ có 404 cho lịch không tồn tại. | Khi scheduleId thiếu hoặc sai định dạng, API trả HTTP status và errorCode nào (400 VALIDATION_ERROR hay 404 SCHEDULE_NOT_FOUND)? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-FLD-Cancel-scheduleId-inputValidation-missing`, `TOB-SCHEDULED-TRANSFER-EP-CancelSchedule-contract` |
| `TGAP-SCHEDULED-TRANSFER-EP-CancelSchedule-llm-a6ab2b` | Chính sách khi cùng Idempotency-Key nhưng khác payload/scheduleId cho DELETE (mã HTTP, errorCode). | Khi DELETE gửi lại cùng Idempotency-Key nhưng khác scheduleId hoặc nội dung, hệ thống trả mã HTTP và errorCode nào? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-EP-CancelSchedule-idempotency-payload-mismatch`, `TOB-SCHEDULED-TRANSFER-EP-CancelSchedule-contract` |
| `TGAP-SCHEDULED-TRANSFER-EP-CreateSchedule-llm-40f8fd` | Nguồn không nêu rõ thứ tự ưu tiên khi một request vi phạm đồng thời nhiều lỗi (ví dụ 401/403/404/422). | Thứ tự kiểm tra và mã lỗi trả về khi request vi phạm nhiều điều kiện cùng lúc là gì? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-RSP-Create-404-inputValidation`, `TCND-SCHEDULED-TRANSFER-RSP-Create-422-same-inputValidation`, `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` |
| `TGAP-SCHEDULED-TRANSFER-EP-CreateSchedule-llm-76ef97` | Nguồn không nêu biên chính xác của 5 phút/90 ngày/366 ngày (bao gồm hay loại trừ) và mốc tính 366 ngày của endDate. | firstRunAt đúng bằng now+5 phút hoặc now+90 ngày có hợp lệ không, và endDate quá 366 ngày tính từ mốc nào (hiện tại hay firstRunAt)? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-RSP-Create-422-time-inputValidation`, `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` |
| `TGAP-SCHEDULED-TRANSFER-EP-CreateSchedule-llm-aa4397` | Chưa nêu mã phản hồi của yêu cầu đồng thời thua khi hai yêu cầu cùng khóa chạy song song (201 hay 200, hay chờ). | Khi hai yêu cầu cùng Idempotency-Key và cùng nội dung đến đồng thời, yêu cầu thua nhận mã HTTP nào và thời hạn lưu Idempotency-Key là bao lâu? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-concurrency-concurrent-dup`, `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` |
| `TGAP-SCHEDULED-TRANSFER-EP-CreateSchedule-llm-ac15cd` | Nguồn đánh dấu endDate và maxRuns đều required=true nhưng FR-041 chỉ cần một trong hai cho lịch định kỳ; chưa rõ ONCE có cần/được phép gửi endDate hoặc maxRuns hay không. | endDate và maxRuns có bắt buộc tuyệt đối không, hay chỉ cần một trong hai cho DAILY/WEEKLY/MONTHLY và tùy chọn với ONCE? Thiếu từng trường riêng lẻ với ONCE trả mã nào? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-FLD-Create-endDate-inputValidation-missing`, `TCND-SCHEDULED-TRANSFER-FLD-Create-maxRuns-inputValidation-missing`, `TOB-SCHEDULED-TRANSFER-EP-CreateSchedule-contract` |
| `TGAP-SCHEDULED-TRANSFER-EP-GetSchedule-llm-14f50a` | Mã HTTP và errorCode khi scheduleId (path) thiếu hoặc sai định dạng; nguồn chỉ nêu 401/403/404 cho endpoint này. | Khi scheduleId thiếu hoặc sai định dạng, API trả mã HTTP và errorCode nào (ví dụ 400 VALIDATION_ERROR hay 404 SCHEDULE_NOT_FOUND)? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-FLD-Get-scheduleId-inputValidation-missing`, `TOB-SCHEDULED-TRANSFER-EP-GetSchedule-contract` |
| `TGAP-SCHEDULED-TRANSFER-EP-GetSchedule-llm-a47ba4` | Thứ tự ưu tiên kiểm tra khi vừa không có lịch vừa không phải chủ sở hữu (404 hay 403). | Nếu người gọi không phải chủ tài khoản và scheduleId không tồn tại thì trả 404 hay 403? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-RSP-Get-403-access`, `TCND-SCHEDULED-TRANSFER-RSP-Get-404-inputValidation`, `TOB-SCHEDULED-TRANSFER-EP-GetSchedule-contract` |
| `TGAP-SCHEDULED-TRANSFER-EP-ListRuns-llm-aed052` | Mã HTTP và errorCode khi limit ngoài [1,100] không hợp lệ | Khi limit < 1 hoặc > 100 thì API trả HTTP status và errorCode nào (ví dụ 400 VALIDATION_ERROR)? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-FLD-ListRuns-limit-inputValidation-max`, `TCND-SCHEDULED-TRANSFER-FLD-ListRuns-limit-inputValidation-min`, `TOB-SCHEDULED-TRANSFER-EP-ListRuns-contract` |
| `TGAP-SCHEDULED-TRANSFER-EP-ListRuns-llm-d51712` | Hành vi khi scheduleId thiếu/rỗng trong path (route không khớp hay lỗi kiểm tra đầu vào) và mã lỗi tương ứng | Request tới /v1/scheduled-transfers//runs trả HTTP status và errorCode nào? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-FLD-ListRuns-scheduleId-inputValidation-missing`, `TOB-SCHEDULED-TRANSFER-EP-ListRuns-contract` |
| `TGAP-SCHEDULED-TRANSFER-EP-ListRuns-llm-f2b3df` | Mã HTTP và errorCode khi offset < 0 | Khi offset âm thì API trả HTTP status và errorCode nào? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-FLD-ListRuns-offset-inputValidation-min`, `TOB-SCHEDULED-TRANSFER-EP-ListRuns-contract` |
| `TGAP-SCHEDULED-TRANSFER-EP-ListSchedules-llm-35e859` | Nguồn chỉ gán 400 INVALID_REQUEST cho 'status ngoài enum'; chưa nói limit ngoài [1,100] hoặc offset < 0 trả mã gì (400 hay clamp). | limit=0, limit=101 và offset=-1 trả 400 INVALID_REQUEST hay được điều chỉnh về giá trị hợp lệ? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-limit-inputValidation-max`, `TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-limit-inputValidation-min`, `TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-offset-inputValidation-min`, `TOB-SCHEDULED-TRANSFER-EP-ListSchedules-contract` |
| `TGAP-SCHEDULED-TRANSFER-EP-ListSchedules-llm-79d96d` | Chưa có giá trị mặc định của limit/offset khi bỏ trống, thứ tự sắp xếp danh sách và cấu trúc phần tử/tổng số trong body 200. | Mặc định limit và offset là bao nhiêu, danh sách sắp theo trường nào, và body 200 gồm những trường nào (có total không)? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-RSP-ListSchedules-200-functional`, `TOB-SCHEDULED-TRANSFER-EP-ListSchedules-contract` |
| `TGAP-SCHEDULED-TRANSFER-EP-ListSchedules-llm-a7d6c1` | Không có đặc tả hành vi khi thiếu accountId trong path (route không khớp, có thể 404 hoặc 400). | Khi accountId rỗng/thiếu trong path, endpoint trả mã HTTP và errorCode nào? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-accountId-inputValidation-missing`, `TOB-SCHEDULED-TRANSFER-EP-ListSchedules-contract` |
| `TGAP-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-llm-504db5` | Không rõ thứ tự ưu tiên khi nhiều lỗi cùng xảy ra (403 so với 404 so với 400 so với 409) và PATCH lên lịch đã hủy. | Thứ tự kiểm tra là gì, và PATCH lên lịch đã CANCELLED trả mã nào? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-RSP-Patch-403-access`, `TCND-SCHEDULED-TRANSFER-RSP-Patch-409-inputValidation`, `TOB-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-contract` |
| `TGAP-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-llm-85bd4b` | Nguồn không nêu kết quả khi scheduleId (path) thiếu hoặc rỗng; seed mẫu gán 400 của status. | Khi scheduleId thiếu/rỗng hoặc sai định dạng, API trả mã HTTP và errorCode nào (404, 400 hay lỗi routing)? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-FLD-Patch-scheduleId-inputValidation-missing`, `TOB-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-contract` |
| `TGAP-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-llm-c34efc` | Không rõ hành vi khi PATCH PAUSED đồng thời với scheduler đang chiếm lần chạy đến hạn. | Lần chạy đã được chiếm trước khi PAUSED có tiếp tục thực thi hay bị hủy/SKIPPED? | QA Lead |  | `TOB-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-contract` |
| `TGAP-SCHEDULED-TRANSFER-FR-040-llm-054334` | Thứ tự ưu tiên kiểm tra khi một yêu cầu vi phạm nhiều điều kiện cùng lúc (403, 404, 422, 400) không được nêu. | Khi yêu cầu vi phạm đồng thời nhiều quy tắc (ví dụ sai chủ tài khoản và đích không tồn tại), mã lỗi nào được trả trước? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-AC-FR-040-4-functional`, `TCND-SCHEDULED-TRANSFER-AC-FR-040-5-functional`, `TOB-SCHEDULED-TRANSFER-FR-040-func` |
| `TGAP-SCHEDULED-TRANSFER-FR-040-llm-8ec604` | Nguồn không nêu rõ biên chính xác của firstRunAt (đúng 5 phút và đúng 90 ngày có hợp lệ hay không) và cách tính 'cách hiện tại'. | firstRunAt đúng bằng hiện tại + 5 phút và đúng + 90 ngày có được chấp nhận không, và dùng đồng hồ nào để so sánh? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-AC-FR-040-3-functional`, `TOB-SCHEDULED-TRANSFER-FR-040-func` |
| `TGAP-SCHEDULED-TRANSFER-FR-041-llm-0dd02e` | Nguồn không nêu điều kiện endDate có bao gồm lần chạy trùng ngày endDate hay không (so sánh theo ngày hay theo thời điểm, múi giờ). | Lần chạy có thời điểm trong ngày endDate nhưng sau giờ của firstRunAt có được chạy không, và endDate tính theo múi giờ nào? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-AC-FR-041-3-functional`, `TOB-SCHEDULED-TRANSFER-FR-041-func` |
| `TGAP-SCHEDULED-TRANSFER-FR-041-llm-6a4874` | Nguồn không nêu ngày neo của lịch MONTHLY có được giữ nguyên (ví dụ 31) sau khi bị kẹp xuống cuối tháng ngắn hay tính tiếp từ ngày đã kẹp (28/02 → 28/03). | Lần chạy MONTHLY tháng sau tính từ ngày neo ban đầu (31 → 31/03) hay từ ngày của lần chạy trước (28/02 → 28/03)? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-AC-FR-041-4-functional`, `TOB-SCHEDULED-TRANSFER-FR-041-func` |
| `TGAP-SCHEDULED-TRANSFER-FR-041-llm-a2144e` | Nguồn không nêu khi cả endDate và maxRuns cùng có thì điều kiện dừng nào áp dụng, và lần chạy kế tiếp được sinh khi lần chạy hiện tại FAILED/MISSED hay không. | Khi có cả endDate và maxRuns, lịch dừng ở điều kiện nào đến trước? Lần chạy FAILED/MISSED có được tính vào maxRuns và có sinh lần chạy kế tiếp không? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-AC-FR-041-1-functional`, `TCND-SCHEDULED-TRANSFER-AC-FR-041-3-functional`, `TOB-SCHEDULED-TRANSFER-FR-041-func` |
| `TGAP-SCHEDULED-TRANSFER-FR-042-llm-60943b` | Nguồn không nêu phản hồi khi limit/offset ngoài biên (limit=0, 101, offset âm) và mặc định của limit khi bỏ trống. | limit hoặc offset ngoài biên trả mã HTTP và errorCode nào, hay bị cắt về biên? Giá trị mặc định của limit là bao nhiêu? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-AC-FR-042-2-functional`, `TCND-SCHEDULED-TRANSFER-AC-FR-042-3-functional`, `TOB-SCHEDULED-TRANSFER-FR-042-func` |
| `TGAP-SCHEDULED-TRANSFER-FR-042-llm-8f66a9` | Nguồn không nêu thứ tự phụ của danh sách lịch và của runs khi scheduledAt trùng nhau. | Danh sách lịch sắp theo trường nào? Runs có cùng scheduledAt được sắp ổn định theo khóa nào? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-AC-FR-042-2-functional`, `TCND-SCHEDULED-TRANSFER-AC-FR-042-3-functional`, `TOB-SCHEDULED-TRANSFER-FR-042-func` |
| `TGAP-SCHEDULED-TRANSFER-FR-042-llm-afd35f` | Nguồn không nêu giá trị nextRunAt khi lịch COMPLETED/CANCELLED/PAUSED, và transferId/reasonCode khi lần chạy chưa có chuyển tiền hoặc thành công. | nextRunAt là null hay bỏ trường khi lịch không còn lần chạy kế tiếp? transferId và reasonCode là null hay vắng mặt tùy trạng thái lần chạy? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-AC-FR-042-1-functional`, `TCND-SCHEDULED-TRANSFER-AC-FR-042-3-functional`, `TOB-SCHEDULED-TRANSFER-FR-042-func` |
| `TGAP-SCHEDULED-TRANSFER-FR-043-llm-67fe31` | Nguồn chưa nêu rõ hành vi với lịch PAUSED hoặc lịch có trạng thái cuối khác (FAILED/MISSED), và lần chạy ở các trạng thái này khi hủy. | DELETE lịch PAUSED có được hủy không, và các lần chạy PENDING của lịch PAUSED xử lý thế nào? Các trạng thái lịch nào ngoài ACTIVE/CANCELLED/COMPLETED cho phép hủy? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-AC-FR-043-1-functional`, `TCND-SCHEDULED-TRANSFER-AC-FR-043-3-functional`, `TOB-SCHEDULED-TRANSFER-FR-043-func` |
| `TGAP-SCHEDULED-TRANSFER-FR-043-llm-e8e035` | Nguồn chưa mô tả cấu trúc nội dung phản hồi 200 ('kết quả cũ') gồm những trường nào. | Body phản hồi 200 của DELETE gồm những trường nào (ví dụ trạng thái lịch, số lần chạy bị hủy)? Lần gọi lặp có trả đúng cùng body không? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-AC-FR-043-2-functional`, `TOB-SCHEDULED-TRANSFER-FR-043-func` |
| `TGAP-SCHEDULED-TRANSFER-FR-044-llm-2d373a` | Nguồn không nêu cách xác định các lần bỏ lỡ: có tạo bản ghi SKIPPED cho từng thời điểm dự kiến trong thời gian tạm dừng ngay lúc tiếp tục hay khi scheduler quét, và tạo từ đâu. | Các bản ghi SKIPPED được tạo ngay khi PATCH ACTIVE hay ở chu kỳ quét kế tiếp, và có giới hạn số bản ghi cho khoảng tạm dừng dài không? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-AC-FR-044-2-functional`, `TOB-SCHEDULED-TRANSFER-FR-044-func` |
| `TGAP-SCHEDULED-TRANSFER-FR-044-llm-4c9409` | Nguồn không nêu thứ tự ưu tiên giữa các mã lỗi khi nhiều điều kiện cùng xảy ra (ví dụ lịch ONCE kèm người gọi không phải chủ lịch, hoặc status sai enum trên lịch ONCE). | Thứ tự kiểm tra 400, 401, 403, 404, 409 là gì khi nhiều điều kiện lỗi cùng đúng? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-AC-FR-044-3-functional`, `TOB-SCHEDULED-TRANSFER-FR-044-func` |
| `TGAP-SCHEDULED-TRANSFER-FR-044-llm-6d8e04` | Nguồn không nói rõ lần chạy đang được chiếm hoặc đang thực hiện tại thời điểm PATCH PAUSED có hoàn tất hay bị hủy. | Khi PATCH PAUSED đến lúc lần chạy đã được chiếm (đang chạy), lần chạy đó hoàn tất bình thường hay bị dừng, và trạng thái cuối là gì? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-AC-FR-044-1-functional`, `TOB-SCHEDULED-TRANSFER-FR-044-func` |
| `TGAP-SCHEDULED-TRANSFER-FR-045-llm-1db06b` | Hành vi mong đợi (trạng thái run, mã lỗi) khi lõi chuyển tiền từ chối lúc chạy do thiếu số dư, vượt hạn mức, tài khoản không hoạt động hoặc Fraud Check từ chối. | Khi một kiểm tra tại lúc chạy thất bại, run chuyển sang trạng thái nào và dùng mã lỗi nào cho từng nguyên nhân (thử lại hay FAILED theo BR-13)? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-AC-FR-045-3-functional`, `TOB-SCHEDULED-TRANSFER-FR-045-func` |
| `TGAP-SCHEDULED-TRANSFER-FR-045-llm-9af403` | Định dạng chính xác của Idempotency-Key (ký tự nối, định dạng thời điểm dự kiến) và cách tạo khóa cho các lần chạy sau lần đầu của lịch định kỳ. | Idempotency-Key ghép mã lịch và thời điểm dự kiến theo định dạng nào (dấu nối, ISO-8601 hay epoch, múi giờ)? Các lần chạy sau lần đầu và lần thử lại dùng khóa gì? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-AC-FR-045-2-functional`, `TOB-SCHEDULED-TRANSFER-FR-045-func` |
| `TGAP-SCHEDULED-TRANSFER-FR-046-llm-0795ee` | Nguồn chưa nêu một 'lần chạy FAILED' cho việc đếm 3 lần liên tiếp là run FAILED sau khi hết lượt thử lại hay mỗi lần thử lại lỗi; cũng chưa nêu run FAILED vì lỗi không thử lại có được tính. | Bộ đếm 3 lần FAILED liên tiếp đếm các run đã FAILED cuối cùng (kể cả lỗi không thử lại), hay đếm từng lần thử lỗi? Run thành công có đặt lại bộ đếm về 0 không? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-AC-FR-046-4-functional`, `TOB-SCHEDULED-TRANSFER-FR-046-func` |
| `TGAP-SCHEDULED-TRANSFER-FR-046-llm-f38f3b` | Chưa rõ giá trị reasonCode cụ thể khi hết lượt thử lại (có bằng mã lỗi gốc không) và cách xử lý thử lại khi lỗi thay đổi loại giữa các lần thử. | reasonCode của run FAILED là mã lỗi gốc của lần thử cuối hay mã riêng? Nếu lần thử lại trả loại lỗi khác thì áp chính sách của loại lỗi mới hay giữ bộ đếm cũ? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-AC-FR-046-1-functional`, `TCND-SCHEDULED-TRANSFER-AC-FR-046-2-functional`, `TOB-SCHEDULED-TRANSFER-FR-046-func` |
| `TGAP-SCHEDULED-TRANSFER-FR-047-llm-0b387a` | Nguồn không liệt kê danh mục reasonCode hợp lệ và nội dung mẫu SMS của scheduled_run_sms cho từng kết quả. | Tập giá trị reasonCode hợp lệ là gì, và mẫu nội dung SMS cho kết quả thành công và FAILED cụ thể ra sao (có chứa số tiền, mã lịch không)? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-AC-FR-047-2-functional`, `TCND-SCHEDULED-TRANSFER-AC-FR-047-3-functional`, `TOB-SCHEDULED-TRANSFER-FR-047-func` |
| `TGAP-SCHEDULED-TRANSFER-FR-047-llm-714bfe` | Nguồn không nêu số lần thử lại, chính sách retry hay cách ghi nhận khi SMS Gateway lỗi (ghi log, đánh dấu, hay bỏ qua). | Khi gửi SMS lỗi, consumer có thử lại không, tối đa mấy lần, và lỗi được ghi nhận ở đâu (sms_log trạng thái FAILED, audit hay chỉ log)? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-AC-FR-047-3-functional`, `TOB-SCHEDULED-TRANSFER-FR-047-func` |
| `TGAP-SCHEDULED-TRANSFER-FR-048-llm-18130b` | Nguồn không nêu tên giá trị loại sự kiện audit (ví dụ mã chuỗi cho tạo/hủy/tạm dừng/tiếp tục/kết quả lần chạy) và định dạng trường audit. | Danh sách chính xác các giá trị loại sự kiện audit cho tạo, hủy, tạm dừng, tiếp tục và từng kết quả lần chạy là gì? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-AC-FR-048-1-functional`, `TCND-SCHEDULED-TRANSFER-AC-FR-048-2-functional`, `TOB-SCHEDULED-TRANSFER-FR-048-func` |
| `TGAP-SCHEDULED-TRANSFER-FR-048-llm-8adeb8` | Nguồn không nêu việc ghi audit có cùng giao dịch DB với thay đổi trạng thái lịch hay không, và hành vi khi ghi audit lỗi. | Ghi audit có nằm trong cùng giao dịch với thay đổi lịch/lần chạy không, và khi ghi audit lỗi thì thao tác có bị hoàn tác không? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-AC-FR-048-1-functional`, `TCND-SCHEDULED-TRANSFER-AC-FR-048-2-functional`, `TOB-SCHEDULED-TRANSFER-FR-048-func` |
| `TGAP-SCHEDULED-TRANSFER-FR-048-llm-ff4b4d` | Nguồn không nói rõ audit có ghi khi hủy lặp lại lịch đã CANCELLED (RSP-Cancel-200 trả kết quả cũ) hay khi POST tạo lặp cùng Idempotency-Key (200) hay không. | Các lần gọi lặp idempotent (hủy lịch đã CANCELLED, tạo lại cùng Idempotency-Key) có ghi thêm bản ghi audit không? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-AC-FR-048-1-functional`, `TOB-SCHEDULED-TRANSFER-FR-048-func` |
| `TGAP-SCHEDULED-TRANSFER-FR-049-llm-914174` | Nguồn không nêu cách xử lý khi nhiều lần chạy quá hạn vượt giới hạn 20 lần mỗi chu kỳ, và việc chạy bù có làm lần chạy tiếp theo của lịch định kỳ bị ảnh hưởng hay không. | Khi số lần chạy quá hạn lớn hơn 20 mỗi chu kỳ, các lần còn lại có thể bị quá 24 giờ trong lúc chờ và chuyển MISSED không? Thứ tự ưu tiên chọn lần chạy là gì? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-AC-FR-049-2-functional`, `TOB-SCHEDULED-TRANSFER-FR-049-func` |
| `TGAP-SCHEDULED-TRANSFER-FR-049-llm-e817df` | Nguồn không nêu chính xác ranh giới 24 giờ: đúng bằng 24 giờ 00 phút có tính là chạy bù hay MISSED, và mốc tính (thời điểm dự kiến so với giờ quét). | Lần chạy quá hạn đúng 24 giờ 00 phút 00 giây tại thời điểm quét thì chạy bù hay MISSED, và tính từ thời điểm dự kiến đến giờ quét? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-AC-FR-049-1-functional`, `TCND-SCHEDULED-TRANSFER-AC-FR-049-2-functional`, `TOB-SCHEDULED-TRANSFER-FR-049-func` |
| `TGAP-SCHEDULED-TRANSFER-MECH-CatchUp-llm-971512` | Nguồn không nêu cách xử lý khi số lần chạy quá hạn vượt giới hạn 20 lần mỗi chu kỳ so với yêu cầu xử lý trong 5 chu kỳ quét đầu (tối đa 100 lần). | Nếu có hơn 100 lần chạy quá hạn ≤ 24 giờ sau khởi động lại thì NFR-012 áp dụng thế nào: có miễn trừ, hay giới hạn 20 lần mỗi chu kỳ được nâng khi chạy bù? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-MECH-CatchUp-resilience-holds`, `TOB-SCHEDULED-TRANSFER-MECH-CatchUp-mechanism` |
| `TGAP-SCHEDULED-TRANSFER-MECH-CatchUp-llm-d2dd08` | Nguồn không nêu rõ mốc 24 giờ tính từ thời điểm nào (thời điểm đến hạn hay thời điểm ngừng) và hành vi đúng tại biên đúng 24 giờ. | Mốc 24 giờ được tính từ thời điểm đến hạn của lần chạy tới thời điểm khởi động lại/quét, hay từ lúc hệ thống ngừng? Lần quá hạn đúng 24 giờ được chạy bù hay MISSED? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-MECH-CatchUp-resilience-holds`, `TOB-SCHEDULED-TRANSFER-MECH-CatchUp-mechanism` |
| `TGAP-SCHEDULED-TRANSFER-MECH-RunClaim-llm-233715` | Cách phục hồi khi tiến trình chết giữa chừng: trạng thái lần chạy sau khi chiếm, cách nhận lại/chạy bù lần chạy đã chiếm nhưng chưa chuyển tiền, và thời hạn coi lần chạy bị treo. | Sau khi chiếm lần chạy mà tiến trình chết trước khi chuyển tiền, lần chạy ở trạng thái nào và ai/khi nào nhận lại để chuyển tiền (chạy bù, thời hạn hết hiệu lực của việc chiếm)? Kết quả cuối cùng mong đợi của lần chạy là gì? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-MECH-RunClaim-resilience-crash-mid`, `TOB-SCHEDULED-TRANSFER-MECH-RunClaim-mechanism` |
| `TGAP-SCHEDULED-TRANSFER-MECH-ScheduleRetry-llm-71796c` | Nguồn không nêu dung sai thời gian cho khoảng cách thử lại 5 phút và 1 giờ, cũng như hành vi khi lỗi đổi loại giữa các lần thử. | Dung sai cho phép của khoảng cách thử lại là bao nhiêu, và nếu lần thử sau trả lỗi loại khác thì áp dụng chính sách của loại lỗi nào? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-MECH-ScheduleRetry-resilience-holds`, `TOB-SCHEDULED-TRANSFER-MECH-ScheduleRetry-mechanism` |
| `TGAP-SCHEDULED-TRANSFER-MECH-ScheduleRetry-llm-9c573d` | Nguồn chưa nói rõ 'lần chạy FAILED liên tiếp' có được reset khi có một run SUCCESS xen giữa không, và một run đã hết lượt thử lại tính là 1 lần FAILED hay mỗi lần thử tính 1. | Bộ đếm FAILED liên tiếp được reset khi nào, và mỗi lần thử lại có được tính vào 3 lần FAILED không hay chỉ tính run cuối cùng đã hết lượt? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-MECH-ScheduleRetry-resilience-holds`, `TOB-SCHEDULED-TRANSFER-MECH-ScheduleRetry-mechanism` |
| `TGAP-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-llm-8a1566` | Hành vi chính xác với phần vượt giới hạn: hoãn sang chu kỳ sau hay từ chối, thứ tự chọn, và rate limit 10 rps áp dụng bằng cách trì hoãn hay trả lỗi (mã lỗi). | Khi có hơn 20 lần chạy đến hạn hoặc vượt 10 rps, phần vượt được giữ lại xử lý ở chu kỳ sau (theo thứ tự nào) hay bị từ chối với mã lỗi nào? Rate limit 10 rps áp dụng cho đối tượng nào (gọi lõi chuyển tiền, Fraud Check)? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-performance-over-limit`, `TOB-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-mechanism` |
| `TGAP-SCHEDULED-TRANSFER-MOD-Scheduler-llm-1d97ab` | Chính sách khi Audit lỗi: fail-open hay fail-closed, có rollback thao tác chính không. | Nếu ghi audit thất bại, thao tác lịch hoặc kết quả run vẫn được giữ (chỉ cảnh báo) hay phải rollback/từ chối? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-MOD-Scheduler-errorHandling-dep-MOD-Audit`, `TOB-SCHEDULED-TRANSFER-MOD-Scheduler-deps` |
| `TGAP-SCHEDULED-TRANSFER-MOD-Scheduler-llm-37be00` | Hành vi khi MOD-Transfer lỗi hạ tầng, timeout hoặc chết giữa chừng (kết quả không rõ). | Khi lõi chuyển tiền timeout hoặc lỗi không phân loại, run có được thử lại với cùng Idempotency-Key không, bao nhiêu lần, và kết thúc ở trạng thái nào? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-MOD-Scheduler-errorHandling-dep-MOD-Transfer`, `TOB-SCHEDULED-TRANSFER-MOD-Scheduler-deps` |
| `TGAP-SCHEDULED-TRANSFER-MOD-Scheduler-llm-fbc52e` | Hành vi của Scheduler khi Quản lý tài khoản lỗi hạ tầng hoặc timeout: mã lỗi, trạng thái run, có thử lại không, reasonCode. | Khi MOD-Account lỗi hoặc timeout lúc tạo lịch hoặc lúc chạy, API trả mã gì và run chuyển trạng thái nào (thử lại hay FAILED ngay, reasonCode là gì)? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-MOD-Scheduler-errorHandling-dep-MOD-Account`, `TOB-SCHEDULED-TRANSFER-MOD-Scheduler-deps` |
| `TGAP-SCHEDULED-TRANSFER-NFR-010-llm-39e29a` | Nguồn chưa nêu cách xử lý độ trễ khi một chu kỳ quét kéo dài quá 30 giây (do gọi Fraud Check đồng bộ hoặc tải 20 lần chạy) và chưa nêu chu kỳ quét có chồng lấn hay không. | Nếu một chu kỳ quét xử lý lâu hơn 30 giây thì chu kỳ kế tiếp chờ hay chạy chồng, và độ trễ này có tính vào chỉ tiêu 60 s không? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-NFR-010-timing-target`, `TOB-SCHEDULED-TRANSFER-NFR-010-nfr` |
| `TGAP-SCHEDULED-TRANSFER-NFR-010-llm-dbaa30` | Nguồn chưa nêu rõ mốc kết thúc của 'bắt đầu' (lúc chọn lần chạy, lúc chiếm bằng UPDATE có điều kiện hay lúc gọi lõi chuyển tiền) và cỡ mẫu tối thiểu để tính 99%. | 'Bắt đầu' của một lần chạy được đo tại thời điểm nào, và cỡ mẫu tối thiểu bao nhiêu lần chạy để kết luận đạt 99%? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-AC-NFR-010-1-timing`, `TCND-SCHEDULED-TRANSFER-NFR-010-timing-target`, `TOB-SCHEDULED-TRANSFER-NFR-010-nfr` |
| `TGAP-SCHEDULED-TRANSFER-NFR-011-llm-6f9627` | Nguồn không nêu trạng thái của lần chạy đã được chiếm nhưng chưa hoàn tất sau khi khởi động lại (có được chiếm lại không, sau bao lâu, bằng cơ chế nào như lease/timeout), nên không xác định được kết quả cuối cùng của lần chạy đó. | Sau khi khởi động lại, lần chạy đã chiếm nhưng chưa chuyển tiền được phục hồi thế nào (chiếm lại sau timeout/lease hay chuyển trạng thái khác) và trạng thái cuối cùng mong đợi là gì? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-AC-NFR-011-2-concurrency`, `TOB-SCHEDULED-TRANSFER-NFR-011-nfr` |
| `TGAP-SCHEDULED-TRANSFER-NFR-012-llm-25d25b` | Nguồn không nói rõ tại ngưỡng đúng 24 giờ là chạy bù hay MISSED, và mốc tính 24 giờ (từ thời điểm dự kiến hay từ lúc hệ thống ngừng). | Mốc 24 giờ được tính từ thời điểm dự kiến của lần chạy hay từ lúc hệ thống ngừng, và đúng 24 giờ thì xử lý thế nào? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-NFR-012-resilience-target`, `TOB-SCHEDULED-TRANSFER-NFR-012-nfr` |
| `TGAP-SCHEDULED-TRANSFER-NFR-012-llm-85d1d6` | Nguồn không nêu cách xử lý khi số lần chạy quá hạn ≤ 24 giờ vượt 100 (5 chu kỳ × giới hạn 20 lần/chu kỳ), tức không thể đạt chỉ tiêu 5 chu kỳ. | Chỉ tiêu 5 chu kỳ áp dụng cho tối đa bao nhiêu lần chạy quá hạn, và nếu vượt 100 lần thì kết quả mong đợi là gì? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-AC-NFR-012-1-resilience`, `TCND-SCHEDULED-TRANSFER-NFR-012-resilience-target`, `TOB-SCHEDULED-TRANSFER-NFR-012-nfr` |
| `TGAP-SCHEDULED-TRANSFER-NFR-013-llm-5ef611` | Chưa rõ cấu hình môi trường đo (phần cứng, kích thước dữ liệu, số worker, độ trễ của mock Fraud Check) và cách xác định thời gian 'chuyển tiền qua API' có gồm thời gian chờ chiếm lần chạy trong hàng đợi giữa các chu kỳ hay không. | Môi trường và tải nền nào dùng để đo p95 <= 500 ms, và thời gian chuyển tiền tính từ lúc gọi API đến phản hồi hay gồm cả thời gian chờ đến chu kỳ quét kế tiếp? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-AC-NFR-013-1-performance`, `TCND-SCHEDULED-TRANSFER-NFR-013-performance-target`, `TOB-SCHEDULED-TRANSFER-NFR-013-nfr` |
| `TGAP-SCHEDULED-TRANSFER-NFR-014-llm-686d43` | Chưa rõ khi một lần chạy được thử lại thì giữ nguyên correlationId hay sinh mới; cũng chưa rõ định dạng/độ dài correlationId và cơ chế sinh ở scheduler (MECH-Correlation mô tả sinh ở API). | Lần thử lại của cùng một lần chạy dùng lại correlationId cũ hay sinh mới? Định dạng correlationId là gì (UUID?) và scheduler sinh trực tiếp hay gọi qua API? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-AC-NFR-014-1-observability`, `TOB-SCHEDULED-TRANSFER-NFR-014-nfr` |
| `TGAP-SCHEDULED-TRANSFER-NFR-014-llm-8f3d67` | Chưa rõ tên trường/vị trí chứa correlationId trong giao dịch, audit, header lời gọi Fraud Check và payload sự kiện. | correlationId được lưu ở trường nào trong giao dịch, audit, sự kiện và truyền qua header nào khi gọi Fraud Check (X-Correlation-Id?) | QA Lead |  | `TCND-SCHEDULED-TRANSFER-AC-NFR-014-2-observability`, `TCND-SCHEDULED-TRANSFER-NFR-014-observability-target`, `TOB-SCHEDULED-TRANSFER-NFR-014-nfr` |
| `TGAP-SCHEDULED-TRANSFER-SCN-ScheduledRun-llm-a277cc` | Ngân sách 60000 ms chưa nói rõ có bao gồm khoảng chờ thử lại 5 phút (FRAUD_CHECK_UNAVAILABLE) hay chỉ tính một lần xử lý; NFR-010 lại đo thời điểm bắt đầu trong 60 giây. | Ngân sách 60000 ms của SCN-ScheduledRun áp cho một lần xử lý (một lần thử) hay cho toàn bộ run gồm các lần thử lại cách 5 phút? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-timing-budget`, `TOB-SCHEDULED-TRANSFER-SCN-ScheduledRun-flow` |
| `TGAP-SCHEDULED-TRANSFER-SCN-ScheduledRun-llm-fbd3f6` | Chưa rõ cách Fraud Check lỗi với số tiền < 5.000.000 VND ghi nhận trong audit/sự kiện, và việc đếm 'FAILED liên tiếp' khi có run SUCCEEDED xen giữa. | Với riskCheck=SKIPPED, audit và sự kiện ScheduledRunCompleted có đánh dấu riêng không? Bộ đếm FAILED liên tiếp có reset khi gặp run SUCCEEDED không? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-errorHandling-fail-INT-01`, `TOB-SCHEDULED-TRANSFER-SCN-ScheduledRun-flow` |
| `TGAP-SCHEDULED-TRANSFER-SM-Schedule-llm-816f6a` | Định nghĩa guard 'lịch định kỳ' và hành vi với lịch ONCE khi tạm dừng hoặc khi FAILED thứ 3. | Lịch ONCE có được PAUSE không? Nếu lịch ONCE FAILED thì chuyển sang trạng thái nào? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-TRN-Schedule-1-guardViolation-guard`, `TCND-SCHEDULED-TRANSFER-TRN-Schedule-3-guardViolation-guard`, `TOB-SCHEDULED-TRANSFER-SM-Schedule-states` |
| `TGAP-SCHEDULED-TRANSFER-SM-Schedule-llm-beb614` | Nội dung BR-11 và ranh giới xác định 'còn lần chạy' không có trong gói; hai guard của TRN-Schedule-2 và TRN-Schedule-7 đối lập nhau nên guardViolation seed mô tả 'giữ PAUSED' mâu thuẫn. | BR-11 quy định chính xác thế nào là còn lần chạy sau khi tiếp tục lịch (endDate, maxRuns, lần bỏ lỡ)? Hai guard guardViolation của Schedule-2/7 có cần bỏ không? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-TRN-Schedule-2-guardViolation-guard`, `TCND-SCHEDULED-TRANSFER-TRN-Schedule-2-stateTransition`, `TCND-SCHEDULED-TRANSFER-TRN-Schedule-7-guardViolation-guard`, `TCND-SCHEDULED-TRANSFER-TRN-Schedule-7-stateTransition`, `TOB-SCHEDULED-TRANSFER-SM-Schedule-states` |
| `TGAP-SCHEDULED-TRANSFER-SM-Schedule-llm-de33e4` | Mã lỗi/HTTP status và thông điệp khi PATCH/DELETE vi phạm guard (lịch không định kỳ, có lần RUNNING). | Khi guard của PATCH/DELETE sai, API trả mã lỗi và HTTP status nào? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-TRN-Schedule-1-guardViolation-guard`, `TCND-SCHEDULED-TRANSFER-TRN-Schedule-5-guardViolation-guard`, `TCND-SCHEDULED-TRANSFER-TRN-Schedule-6-guardViolation-guard`, `TOB-SCHEDULED-TRANSFER-SM-Schedule-states` |
| `TGAP-SCHEDULED-TRANSFER-SM-ScheduledRun-llm-0eb7ca` | Lease 120 giây mới là đề xuất Q-S2, chưa được chốt. | Q-S2 đã được duyệt chưa; giá trị lease cuối cùng là bao nhiêu giây? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-8-guardViolation-guard`, `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-8-stateTransition`, `TOB-SCHEDULED-TRANSFER-SM-ScheduledRun-states` |
| `TGAP-SCHEDULED-TRANSFER-SM-ScheduledRun-llm-0f2719` | Chưa rõ lỗi nào dùng độ trễ 5 phút, lỗi nào dùng 1 giờ; chưa rõ ranh giới '≤ 3 lần'/'≤ 2 lần' tính theo số lần thử hay số lần thử lại. | FRAUD_CHECK_UNAVAILABLE và INSUFFICIENT_FUNDS mỗi loại dùng độ trễ nào (5 phút hay 1 giờ)? Giới hạn 3 và 2 là tổng số lần chạy hay số lần thử lại sau lần đầu? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-3-guardViolation-guard`, `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-3-stateTransition`, `TOB-SCHEDULED-TRANSFER-SM-ScheduledRun-states` |
| `TGAP-SCHEDULED-TRANSFER-SM-ScheduledRun-llm-484b70` | Chưa định nghĩa danh sách lỗi vĩnh viễn và reasonCode tương ứng; chưa nêu thứ tự ưu tiên khi lần chạy PENDING thỏa cả điều kiện SKIPPED và MISSED, hay DELETE lịch khi lần chạy đang RUNNING. | Danh sách mã lỗi vĩnh viễn và reasonCode là gì? Lần chạy quá hạn trong lúc PAUSED hơn 24 giờ sang SKIPPED hay MISSED? DELETE lịch khi lần chạy đang RUNNING xử lý thế nào? | QA Lead |  | `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-4-stateTransition`, `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-5-stateTransition`, `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-6-stateTransition`, `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-7-stateTransition`, `TOB-SCHEDULED-TRANSFER-SM-ScheduledRun-states` |

## 7. Báo cáo kiểm (gate)

Gate GS: đạt

| Luật | Mức | Nội dung | Đã kiểm | Vi phạm |
| --- | --- | --- | --- | --- |
| TSO-001 | block | Mọi nút có basedOn tới nút TDD tồn tại ở baseline | 3380 | 0 |
| TSO-002 | block | Nút derivation=llm có model, promptHash, evidence | 2938 | 0 |
| TSO-003 | block | Mọi mã tham chiếu thuộc danh sách trắng | 3380 | 0 |
| TSO-004 | block | Mọi Requirement trong phạm vi có TestObligation | 15 | 0 |
| TSO-005 | block | Nút TDD có luật seed có đủ TestCondition seed | 192 | 0 |
| TSO-006 | block | Mọi TestCondition có oracle; oracle threshold có measurement | 237 | 0 |
| TSO-007 | blockBaseline | TestGap có owner; điều kiện bị gap chặn không được baseline | 66 | 0 |
| TSO-008 | block | Mọi TestCondition được ít nhất một CoverageItem hiện thực | 237 | 0 |
| TSO-009 | block | TR có passCriteria đánh số trỏ tới điều kiện; scopeNote khi cùng chủ thể có nhiều TR | 68 | 0 |
| TSO-010 | warn | Kỹ thuật phù hợp conditionKind | 257 | 0 |
| TSO-011 | block | Mọi CoverageItem có TestCase phủ; mọi TestCase có bước when và Assertion | 514 | 0 |
| TSO-012 | block | Assertion.expected khớp oracle (đơn vị, ngưỡng, mã lỗi) | 258 | 0 |
| TSO-013 | block | FaultModel có FaultInjection; timing=afterCommit có assertion về hiệu ứng phụ | 5 | 0 |
| TSO-014 | block | Có idempotency thì phủ đủ: gửi lại, khác payload, trùng đồng thời | 2 | 0 |
| TSO-015 | block | Điều kiện timing dùng trường hợp xấu nhất, không cộng p95 thành phần | 1 | 0 |
| TSO-016 | block | Điều kiện performance có workload; không có test vượt ngưỡng + 1 cho chỉ tiêu 0 hoặc 100 % | 8 | 0 |
| TSO-017 | block | Mỗi điểm thực thi quyền có test cho phép và từ chối | 8 | 0 |
| TSO-018 | block | Không có giá trị bí mật trong dữ liệu, bước, assertion | 1908 | 0 |
| TSO-019 | block | TR mức L1 đi qua ScenarioPath có ít nhất hai Module | 3 | 0 |
| TSO-020 | blockBaseline | Nút llm chỉ baseline khi đã duyệt, trừ loại nút và ngưỡng policy cho phép | 2938 | 0 |
| TSO-021 | block | Phiên bản hóa: Baselined bất biến; supersedes thẳng; Retired có retiredBy | 1 | 0 |

## 8. Lịch sử phiên bản và phạm vi hồi quy

| Phiên bản | Trạng thái | CR | Thêm | Sửa | Bỏ | Số nút |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | Baselined | CR-001 | 3380 | 0 | 0 | 3380 |

Hồi quy (0 test case đã baseline, thuộc phần bị ảnh hưởng nhưng không đổi): không có

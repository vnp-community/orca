---
spec: TSD-SCHEDULED-TRANSFER-TestDesign
feature: SCHEDULED-TRANSFER
docKind: TestDesign
version: 1
status: Baselined
supersedes: none
baseline: b-bd72d688fca2
changes: [CR-001]
contentHash: 5c9bd99e7ff3442baa026f0a5414e140b175579ea0f6001d7a94d4dbeb822154
---
# Test Design: SCHEDULED-TRANSFER

## 0. Thông tin tài liệu

| Trường | Giá trị |
| --- | --- |
| id | TSD-SCHEDULED-TRANSFER-TestDesign_v1 |
| Feature | SCHEDULED-TRANSFER |
| Phiên bản | 1 |
| Trạng thái | Baselined |
| Baseline TDD | DOC-SRS-CR001 (b-bd72d688fca2) |
| CR của phiên bản này | CR-001 |
| Ontology | tso v1 |

## 1. Cách tiếp cận theo mức

| Mức | Ý nghĩa | Môi trường | Thành phần |
| --- | --- | --- | --- |
| L1 | Nghiệp vụ đầu-cuối | Môi trường tích hợp đầu-cuối (L1) | toàn bộ service của feature, CSDL, hàng đợi, stub đối tác ngoài |
| L2 | Nghiệp vụ | Môi trường service (L2) | service đang kiểm, CSDL thật, stub phụ thuộc ngoài |
| L3 | Cô lập service | Môi trường service cô lập (L3) | service đang kiểm, stub mọi phụ thuộc, đồng hồ giả |

## 2. Thiết kế test

### TD-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-boundaryValue

- TR: `TR-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-performance` (L1, performance)
- Kỹ thuật: boundaryValue; tiêu chí phủ: Mỗi biên hai giá trị (trong và ngoài biên); mục tiêu 100 %
- Lý do: boundaryValue phù hợp timing

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-timing-budget-c` | `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-timing-budget` | condition | Một lần xử lý trọn luồng hoàn tất ≤ 60000 ms ở trường hợp xấu nhất. |  | ≤ 60000 ms | `TORC-SCHEDULED-TRANSFER-SCN-ScheduledRun-budget` |
| `TCOV-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-timing-worst-INT-01-c` | `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-timing-worst-INT-01` | condition | Fraud Check xấu nhất: 2 lần × 1000 ms + backoff 0 ms = 2000 ms, trong ngân sách 60000 ms. |  | 2000 ms ≤ 60000 ms (ngân sách của SCN-ScheduledRun) | `TORC-SCHEDULED-TRANSFER-SCN-ScheduledRun-worst-INT-01` |

### TD-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-scenario-functional

- TR: `TR-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-functional` (L1, functional)
- Kỹ thuật: scenario; tiêu chí phủ: Mỗi đường đi (chính, lỗi) một coverage item; mục tiêu 100 %
- Lý do: scenario phù hợp functional

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-functional-main-c` | `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-functional-main` | path | Luồng chính đến hạn: quét 30 giây, tối đa 20 lần chạy, chiếm bằng UPDATE có điều kiện, correlationId riêng, Idempotency-Key = mã lịch + thời điểm dự kiến, cập nhật run. |  | 1 giao dịch COMPLETED, run SUCCEEDED có transferId, 1 bản ghi audit tác nhân SCHEDULER, 1 sự kiện ScheduledRunCompleted trong outbox; giao dịch tính vào hạn mức ngày và hiện trong lịch sử. | `TORC-SCHEDULED-TRANSFER-SCN-ScheduledRun-main` |

### TD-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-scenario-reliability

- TR: `TR-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-reliability` (L1, reliability)
- Kỹ thuật: scenario; tiêu chí phủ: Mỗi đường đi (chính, lỗi) một coverage item; mục tiêu 100 %
- Lý do: scenario phù hợp errorHandling

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-errorHandling-fail-INT-01-c` | `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-errorHandling-fail-INT-01` | path | Lỗi INT-01 (timeout, 5xx, 4xx, sai schema) với số tiền dưới và từ 5.000.000 VND, áp dụng FB-INT01. |  | Dưới 5.000.000 VND: riskCheck=SKIPPED, giao dịch COMPLETED. Từ 5.000.000 VND (kể cả đúng 5.000.000): FRAUD_CHECK_UNAVAILABLE (503), không có giao dịch, thử lại tối đa 3 lần cách 5 phút rồi FAILED với reasonCode. | `TORC-SCHEDULED-TRANSFER-FB-INT01-fail-SCN-ScheduledRun` |

### TD-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-accessMatrix

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-security` (L2, security)
- Kỹ thuật: accessMatrix; tiêu chí phủ: Mỗi điểm thực thi quyền × {cho phép, từ chối}; mục tiêu 100 %
- Lý do: accessMatrix phù hợp access

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-Cancel-401-access-c` | `TCND-SCHEDULED-TRANSFER-RSP-Cancel-401-access` | condition | DELETE với API key sai hoặc không hợp lệ. |  | Status 401, errorCode = UNAUTHORIZED. | `TORC-SCHEDULED-TRANSFER-RSP-Cancel-401` |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-Cancel-403-access-c` | `TCND-SCHEDULED-TRANSFER-RSP-Cancel-403-access` | condition | DELETE lịch tồn tại bởi người gọi có API key hợp lệ nhưng không phải chủ lịch. |  | Status 403, errorCode = FORBIDDEN; lịch vẫn ACTIVE. | `TORC-SCHEDULED-TRANSFER-RSP-Cancel-403` |

### TD-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-equivalencePartitioning

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-functional` (L2, functional)
- Kỹ thuật: equivalencePartitioning; tiêu chí phủ: Mỗi lớp tương đương một coverage item; mục tiêu 100 %
- Lý do: equivalencePartitioning phù hợp inputValidation

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Cancel-Authorization-inputValidation-missing-v0` | `TCND-SCHEDULED-TRANSFER-FLD-Cancel-Authorization-inputValidation-missing` | partitionValue | Header Authorization = <thiếu> (không hợp lệ). | <thiếu> | Status 401, body errorCode = UNAUTHORIZED. | `TORC-SCHEDULED-TRANSFER-RSP-Cancel-401-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Cancel-scheduleId-inputValidation-missing-v0` | `TCND-SCHEDULED-TRANSFER-FLD-Cancel-scheduleId-inputValidation-missing` | partitionValue | scheduleId (path) = <thiếu> (không hợp lệ). | <thiếu> | Bị từ chối, không có thay đổi trạng thái; mã lỗi chờ làm rõ. | `TORC-SCHEDULED-TRANSFER-EP-CancelSchedule-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-Cancel-404-inputValidation-c` | `TCND-SCHEDULED-TRANSFER-RSP-Cancel-404-inputValidation` | condition | scheduleId đúng định dạng nhưng không tồn tại. |  | Status 404, errorCode = SCHEDULE_NOT_FOUND. | `TORC-SCHEDULED-TRANSFER-RSP-Cancel-404` |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-Cancel-409-inputValidation-c` | `TCND-SCHEDULED-TRANSFER-RSP-Cancel-409-inputValidation` | condition | Lịch đã COMPLETED, và lịch có một lần chạy RUNNING (hai ca). |  | Status 409, errorCode = SCHEDULE_NOT_CANCELLABLE; trạng thái lịch và lần chạy không đổi. | `TORC-SCHEDULED-TRANSFER-RSP-Cancel-409` |

### TD-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-errorGuessing

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-reliability` (L2, reliability)
- Kỹ thuật: errorGuessing; tiêu chí phủ: Mỗi tình huống lỗi đoán trước một coverage item; mục tiêu 100 %
- Lý do: errorGuessing phù hợp idempotency

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-idempotency-payload-mismatch-c` | `TCND-SCHEDULED-TRANSFER-EP-CancelSchedule-idempotency-payload-mismatch` | condition | DELETE cùng Idempotency-Key nhưng khác tham số so với lần trước. |  | 200, nội dung phản hồi giống lần đầu; số bản ghi audit hủy không tăng. | `TORC-SCHEDULED-TRANSFER-EP-CancelSchedule-idem` |
| `TCOV-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-idempotency-replay-c` | `TCND-SCHEDULED-TRANSFER-EP-CancelSchedule-idempotency-replay` | condition | Gửi lại đúng yêu cầu DELETE (cùng khóa, cùng scheduleId) sau khi lần đầu hủy thành công. |  | 200, nội dung phản hồi giống lần đầu; số bản ghi audit hủy không tăng. | `TORC-SCHEDULED-TRANSFER-EP-CancelSchedule-idem` |

### TD-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-faultInjection

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-reliability` (L2, reliability)
- Kỹ thuật: faultInjection; tiêu chí phủ: Mỗi mô hình lỗi được tiêm ít nhất một lần; mục tiêu 100 %
- Lý do: faultInjection phù hợp concurrency

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-concurrency-concurrent-dup-c` | `TCND-SCHEDULED-TRANSFER-EP-CancelSchedule-concurrency-concurrent-dup` | condition | Hai DELETE đồng thời cùng Idempotency-Key và scheduleId của lịch ACTIVE. |  | 200, nội dung phản hồi giống lần đầu; số bản ghi audit hủy không tăng. | `TORC-SCHEDULED-TRANSFER-EP-CancelSchedule-idem` |

### TD-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-scenario

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-functional` (L2, functional)
- Kỹ thuật: scenario; tiêu chí phủ: Mỗi đường đi (chính, lỗi) một coverage item; mục tiêu 100 %
- Lý do: scenario phù hợp audit

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-audit-p1cc3a7-c` | `TCND-SCHEDULED-TRANSFER-EP-CancelSchedule-audit-p1cc3a7` | condition | Hủy lịch thành công và kiểm tra bản ghi audit hủy đủ trường theo FR-048. |  | Một bản ghi audit với correlationId, thời điểm, actor = mã tài khoản, loại sự kiện hủy, mã lịch. | `TORC-SCHEDULED-TRANSFER-AC-FR-048-1-p1cc3a7` |

### TD-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-useCase

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-functional` (L2, functional)
- Kỹ thuật: useCase; tiêu chí phủ: Mỗi tiêu chí chấp nhận một coverage item; mục tiêu 100 %
- Lý do: useCase phù hợp functional

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-Cancel-200-functional-c` | `TCND-SCHEDULED-TRANSFER-RSP-Cancel-200-functional` | condition | Hủy lịch ACTIVE của chính người gọi, không có lần RUNNING. |  | 200; lịch = CANCELLED; mọi lần PENDING trước đó = CANCELLED. | `TORC-SCHEDULED-TRANSFER-RSP-Cancel-200` |

### TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-accessMatrix

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-security` (L2, security)
- Kỹ thuật: accessMatrix; tiêu chí phủ: Mỗi điểm thực thi quyền × {cho phép, từ chối}; mục tiêu 100 %
- Lý do: accessMatrix phù hợp access

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-access-p73a390-c` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-access-p73a390` | condition | fromAccountId khác tài khoản của chủ API key. |  | HTTP 403 FORBIDDEN; không có bản ghi lịch mới. | `TORC-SCHEDULED-TRANSFER-AC-FR-040-5-p73a390` |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-Create-401-access-c` | `TCND-SCHEDULED-TRANSFER-RSP-Create-401-access` | condition | Gọi POST /v1/scheduled-transfers với header Authorization thiếu hoặc API key sai. |  | statusCode=401, errorCode=UNAUTHORIZED; số lịch của tài khoản không đổi. | `TORC-SCHEDULED-TRANSFER-RSP-Create-401` |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-Create-403-access-c` | `TCND-SCHEDULED-TRANSFER-RSP-Create-403-access` | condition | Gọi POST với API key hợp lệ nhưng fromAccountId không thuộc chủ API key. |  | statusCode=403, errorCode=FORBIDDEN; không có lịch mới cho tài khoản B. | `TORC-SCHEDULED-TRANSFER-RSP-Create-403` |

### TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-boundaryValue-functional

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` (L2, functional)
- Kỹ thuật: boundaryValue; tiêu chí phủ: Mỗi biên hai giá trị (trong và ngoài biên); mục tiêu 100 %
- Lý do: boundaryValue phù hợp inputValidation

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-amount-inputValidation-max-v0` | `TCND-SCHEDULED-TRANSFER-FLD-Create-amount-inputValidation-max` | validBoundary | amount (body) = 50000000 (hợp lệ) | 50000000 | amount (body) = 50000000 được chấp nhận | `TORC-SCHEDULED-TRANSFER-RSP-Create-422-amount-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-amount-inputValidation-max-v1` | `TCND-SCHEDULED-TRANSFER-FLD-Create-amount-inputValidation-max` | partitionValue | amount (body) = 50000001 (không hợp lệ) | 50000001 | 50000001 -> HTTP 422 LIMIT_EXCEEDED, không tạo lịch; 50000000 -> HTTP 201. | `TORC-SCHEDULED-TRANSFER-RSP-Create-422-amount-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-amount-inputValidation-min-v0` | `TCND-SCHEDULED-TRANSFER-FLD-Create-amount-inputValidation-min` | partitionValue | amount (body) = 0 (không hợp lệ) | 0 | HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-amount-inputValidation-min-v1` | `TCND-SCHEDULED-TRANSFER-FLD-Create-amount-inputValidation-min` | validBoundary | amount (body) = 1 (hợp lệ) | 1 | amount (body) = 1 được chấp nhận | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-maxRuns-inputValidation-max-v0` | `TCND-SCHEDULED-TRANSFER-FLD-Create-maxRuns-inputValidation-max` | validBoundary | maxRuns (body) = 60 (hợp lệ) | 60 | maxRuns (body) = 60 được chấp nhận | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-maxRuns-inputValidation-max-v1` | `TCND-SCHEDULED-TRANSFER-FLD-Create-maxRuns-inputValidation-max` | partitionValue | maxRuns (body) = 61 (không hợp lệ) | 61 | HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-maxRuns-inputValidation-min-v0` | `TCND-SCHEDULED-TRANSFER-FLD-Create-maxRuns-inputValidation-min` | partitionValue | maxRuns (body) = 0 (không hợp lệ) | 0 | HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-maxRuns-inputValidation-min-v1` | `TCND-SCHEDULED-TRANSFER-FLD-Create-maxRuns-inputValidation-min` | validBoundary | maxRuns (body) = 1 (hợp lệ) | 1 | maxRuns (body) = 1 được chấp nhận | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |

### TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-boundaryValue-performance

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-performance` (L2, performance)
- Kỹ thuật: boundaryValue; tiêu chí phủ: Mỗi biên hai giá trị (trong và ngoài biên); mục tiêu 100 %
- Lý do: boundaryValue phù hợp timing

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-timing-p31f6cf-c` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-timing-p31f6cf` | condition | Biên thời gian firstRunAt: cách hiện tại dưới 5 phút hoặc trên 90 ngày bị từ chối. |  | Giá trị dưới 5 phút và trên 90 ngày -> 422 SCHEDULE_TIME_INVALID; giá trị trong khoảng -> 201. | `TORC-SCHEDULED-TRANSFER-AC-FR-040-3-p31f6cf` |

### TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-equivalencePartitioning

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` (L2, functional)
- Kỹ thuật: equivalencePartitioning; tiêu chí phủ: Mỗi lớp tương đương một coverage item; mục tiêu 100 %
- Lý do: equivalencePartitioning phù hợp inputValidation

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-inputValidation-p4e161f-c` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-inputValidation-p4e161f` | condition | Lịch định kỳ thiếu cả endDate và maxRuns; hoặc maxRuns ngoài 1..60. |  | 400 INVALID_REQUEST. | `TORC-SCHEDULED-TRANSFER-AC-FR-041-2-p4e161f` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-Authorization-inputValidation-missing-v0` | `TCND-SCHEDULED-TRANSFER-FLD-Create-Authorization-inputValidation-missing` | partitionValue | Authorization (header) = <thiếu> (không hợp lệ) | <thiếu> | HTTP 401 với errorCode UNAUTHORIZED; không có bản ghi lịch mới. | `TORC-SCHEDULED-TRANSFER-RSP-Create-401-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-IdempotencyKey-inputValidation-missing-v0` | `TCND-SCHEDULED-TRANSFER-FLD-Create-IdempotencyKey-inputValidation-missing` | partitionValue | Idempotency-Key (header) = <thiếu> (không hợp lệ) | <thiếu> | HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-amount-inputValidation-missing-v0` | `TCND-SCHEDULED-TRANSFER-FLD-Create-amount-inputValidation-missing` | partitionValue | amount (body) = <thiếu> (không hợp lệ) | <thiếu> | HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-endDate-inputValidation-missing-v0` | `TCND-SCHEDULED-TRANSFER-FLD-Create-endDate-inputValidation-missing` | partitionValue | endDate (body) = <thiếu> (không hợp lệ) | <thiếu> | HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-firstRunAt-inputValidation-missing-v0` | `TCND-SCHEDULED-TRANSFER-FLD-Create-firstRunAt-inputValidation-missing` | partitionValue | firstRunAt (body) = <thiếu> (không hợp lệ) | <thiếu> | HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-fromAccountId-inputValidation-missing-v0` | `TCND-SCHEDULED-TRANSFER-FLD-Create-fromAccountId-inputValidation-missing` | partitionValue | fromAccountId (body) = <thiếu> (không hợp lệ) | <thiếu> | HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-maxRuns-inputValidation-missing-v0` | `TCND-SCHEDULED-TRANSFER-FLD-Create-maxRuns-inputValidation-missing` | partitionValue | maxRuns (body) = <thiếu> (không hợp lệ) | <thiếu> | HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-recurrence-inputValidation-enum-v0` | `TCND-SCHEDULED-TRANSFER-FLD-Create-recurrence-inputValidation-enum` | validClass | recurrence (body) = ONCE (hợp lệ) | ONCE | recurrence (body) = ONCE được chấp nhận | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-recurrence-inputValidation-enum-v1` | `TCND-SCHEDULED-TRANSFER-FLD-Create-recurrence-inputValidation-enum` | validClass | recurrence (body) = DAILY (hợp lệ) | DAILY | recurrence (body) = DAILY được chấp nhận | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-recurrence-inputValidation-enum-v2` | `TCND-SCHEDULED-TRANSFER-FLD-Create-recurrence-inputValidation-enum` | validClass | recurrence (body) = WEEKLY (hợp lệ) | WEEKLY | recurrence (body) = WEEKLY được chấp nhận | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-recurrence-inputValidation-enum-v3` | `TCND-SCHEDULED-TRANSFER-FLD-Create-recurrence-inputValidation-enum` | validClass | recurrence (body) = MONTHLY (hợp lệ) | MONTHLY | recurrence (body) = MONTHLY được chấp nhận | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-recurrence-inputValidation-enum-v4` | `TCND-SCHEDULED-TRANSFER-FLD-Create-recurrence-inputValidation-enum` | partitionValue | recurrence (body) = YEARLY (không hợp lệ) | YEARLY | HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-recurrence-inputValidation-missing-v0` | `TCND-SCHEDULED-TRANSFER-FLD-Create-recurrence-inputValidation-missing` | partitionValue | recurrence (body) = <thiếu> (không hợp lệ) | <thiếu> | HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-toAccountId-inputValidation-missing-v0` | `TCND-SCHEDULED-TRANSFER-FLD-Create-toAccountId-inputValidation-missing` | partitionValue | toAccountId (body) = <thiếu> (không hợp lệ) | <thiếu> | HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-Create-400-inputValidation-c` | `TCND-SCHEDULED-TRANSFER-RSP-Create-400-inputValidation` | condition | POST với đầu vào vi phạm hợp đồng: thiếu Idempotency-Key/trường bắt buộc, amount không nguyên dương, recurrence sai, định kỳ thiếu cả endDate và maxRuns, maxRuns ngoài 1..60, hoặc firstRunAt sai định dạng. |  | HTTP 400 với errorCode INVALID_REQUEST cho từng biến thể; số lịch không đổi. | `TORC-SCHEDULED-TRANSFER-RSP-Create-400` |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-Create-404-inputValidation-c` | `TCND-SCHEDULED-TRANSFER-RSP-Create-404-inputValidation` | condition | Gọi POST với toAccountId không tồn tại trong hệ thống. |  | statusCode=404, errorCode=ACCOUNT_NOT_FOUND; không có lịch mới. | `TORC-SCHEDULED-TRANSFER-RSP-Create-404` |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-Create-409-inputValidation-c` | `TCND-SCHEDULED-TRANSFER-RSP-Create-409-inputValidation` | condition | Gửi lại POST với cùng Idempotency-Key đã dùng nhưng nội dung body khác lần đầu. |  | statusCode=409, errorCode=IDEMPOTENCY_CONFLICT; lịch gốc giữ nguyên, tổng số lịch không tăng. | `TORC-SCHEDULED-TRANSFER-RSP-Create-409` |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-Create-422-amount-inputValidation-c` | `TCND-SCHEDULED-TRANSFER-RSP-Create-422-amount-inputValidation` | condition | Gọi POST với amount lớn hơn 50.000.000 VND (biên: 50.000.000 hợp lệ, 50.000.001 bị từ chối). |  | statusCode=422, errorCode=LIMIT_EXCEEDED; không tạo lịch. | `TORC-SCHEDULED-TRANSFER-RSP-Create-422-amount` |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-Create-422-limit-inputValidation-c` | `TCND-SCHEDULED-TRANSFER-RSP-Create-422-limit-inputValidation` | condition | Tạo lịch thứ 21 cho tài khoản đã có 20 lịch ở trạng thái ACTIVE hoặc PAUSED. |  | statusCode=422, errorCode=SCHEDULE_LIMIT_REACHED; số lịch ACTIVE/PAUSED vẫn bằng 20. | `TORC-SCHEDULED-TRANSFER-RSP-Create-422-limit` |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-Create-422-same-inputValidation-c` | `TCND-SCHEDULED-TRANSFER-RSP-Create-422-same-inputValidation` | condition | Gọi POST với fromAccountId bằng toAccountId. |  | statusCode=422, errorCode=SAME_ACCOUNT; không có lịch mới. | `TORC-SCHEDULED-TRANSFER-RSP-Create-422-same` |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-Create-422-time-inputValidation-c` | `TCND-SCHEDULED-TRANSFER-RSP-Create-422-time-inputValidation` | condition | Gọi POST với firstRunAt cách hiện tại < 5 phút hoặc > 90 ngày, hoặc endDate trước lần chạy đầu hoặc quá 366 ngày. |  | statusCode=422, errorCode=SCHEDULE_TIME_INVALID; không có lịch mới. | `TORC-SCHEDULED-TRANSFER-RSP-Create-422-time` |

### TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-errorGuessing-functional

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` (L2, functional)
- Kỹ thuật: errorGuessing; tiêu chí phủ: Mỗi tình huống lỗi đoán trước một coverage item; mục tiêu 100 %
- Lý do: errorGuessing phù hợp invariant

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-invariant-p420954-c` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-invariant-p420954` | condition | Tạo lịch không làm đổi số dư và không sinh ledger_entries. |  | Số dư hai tài khoản và số ledger_entries không đổi. | `TORC-SCHEDULED-TRANSFER-AC-FR-040-2-p420954` |

### TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-errorGuessing-reliability

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-reliability` (L2, reliability)
- Kỹ thuật: errorGuessing; tiêu chí phủ: Mỗi tình huống lỗi đoán trước một coverage item; mục tiêu 100 %
- Lý do: errorGuessing phù hợp idempotency

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-idempotency-p50d7db-c` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-idempotency-p50d7db` | condition | Gửi lại cùng Idempotency-Key và cùng nội dung thì trả lịch đã tạo. |  | 200, cùng scheduleId; tổng số lịch không tăng. | `TORC-SCHEDULED-TRANSFER-RSP-Create-200-p50d7db` |
| `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-idempotency-payload-mismatch-c` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-idempotency-payload-mismatch` | condition | Gửi POST với Idempotency-Key đã dùng nhưng nội dung body khác lần đầu. |  | HTTP 409 với errorCode IDEMPOTENCY_CONFLICT; số lịch không đổi. | `TORC-SCHEDULED-TRANSFER-RSP-Create-409-idem-conflict` |
| `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-idempotency-replay-c` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-idempotency-replay` | condition | Gửi lại đúng yêu cầu đã thành công (cùng Idempotency-Key, cùng nội dung) tới POST /v1/scheduled-transfers. |  | HTTP 200, scheduleId = S; tổng số lịch và số bản ghi audit tạo lịch không tăng. | `TORC-SCHEDULED-TRANSFER-EP-CreateSchedule-idem` |

### TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-faultInjection

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-reliability` (L2, reliability)
- Kỹ thuật: faultInjection; tiêu chí phủ: Mỗi mô hình lỗi được tiêm ít nhất một lần; mục tiêu 100 %
- Lý do: faultInjection phù hợp concurrency, errorHandling

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-concurrency-concurrent-dup-c` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-concurrency-concurrent-dup` | condition | Hai yêu cầu POST cùng Idempotency-Key và cùng nội dung được gửi đồng thời tới /v1/scheduled-transfers. |  | HTTP 200, scheduleId = S; tổng số lịch và số bản ghi audit tạo lịch không tăng. | `TORC-SCHEDULED-TRANSFER-EP-CreateSchedule-idem` |
| `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-concurrency-pdcbc85-c` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-concurrency-pdcbc85` | condition | Hai request đồng thời cùng Idempotency-Key và cùng nội dung. |  | Đúng 1 lịch với key K; hai phản hồi cùng scheduleId. | `TORC-SCHEDULED-TRANSFER-RSP-Create-200-pdcbc85` |
| `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-errorHandling-pe11059-c` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-errorHandling-pe11059` | condition | toAccountId không tồn tại. |  | HTTP 404 ACCOUNT_NOT_FOUND; không có bản ghi lịch mới. | `TORC-SCHEDULED-TRANSFER-AC-FR-040-4-pe11059` |

### TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-scenario

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` (L2, functional)
- Kỹ thuật: scenario; tiêu chí phủ: Mỗi đường đi (chính, lỗi) một coverage item; mục tiêu 100 %
- Lý do: scenario phù hợp audit

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-audit-p28bd0d-c` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-audit-p28bd0d` | condition | Tạo lịch thành công phải ghi audit. |  | Một bản ghi audit đủ các trường nêu trên. | `TORC-SCHEDULED-TRANSFER-AC-FR-048-1-p28bd0d` |
| `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-audit-pf42db3-c` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-audit-pf42db3` | condition | Tạo lịch thành công ghi audit. |  | Một bản ghi audit chứa correlationId, thời điểm, tác nhân = mã tài khoản, loại sự kiện tạo lịch, scheduleId. | `TORC-SCHEDULED-TRANSFER-AC-FR-048-1-pf42db3` |

### TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-stateTransition

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` (L2, functional)
- Kỹ thuật: stateTransition; tiêu chí phủ: Mỗi chuyển trạng thái và mỗi guard bị vi phạm; mục tiêu 100 %
- Lý do: stateTransition phù hợp guardViolation

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-guardViolation-p588a23-c` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-guardViolation-p588a23` | condition | fromAccountId bằng toAccountId. |  | HTTP 422 SAME_ACCOUNT; không có bản ghi lịch mới. | `TORC-SCHEDULED-TRANSFER-RSP-Create-422-same-p588a23` |
| `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-guardViolation-pc9af5d-c` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-guardViolation-pc9af5d` | condition | Biên giới hạn lịch: tài khoản đã có 20 lịch ACTIVE/PAUSED thì lịch thứ 21 bị từ chối; lịch thứ 20 được chấp nhận. |  | Lịch thứ 20 -> 201; lịch thứ 21 -> 422 SCHEDULE_LIMIT_REACHED. | `TORC-SCHEDULED-TRANSFER-AC-FR-040-6-pc9af5d` |

### TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-useCase

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional` (L2, functional)
- Kỹ thuật: useCase; tiêu chí phủ: Mỗi tiêu chí chấp nhận một coverage item; mục tiêu 100 %
- Lý do: useCase phù hợp functional

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional-p2341c6-c` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-functional-p2341c6` | condition | Lịch MONTHLY đặt ngày 29, 30 hoặc 31 được tạo và các lần chạy sau rơi đúng ngày cuối tháng. |  | Lần chạy ở tháng 30 ngày vào ngày 30, tháng 2 vào ngày 28 (hoặc 29 năm nhuận). | `TORC-SCHEDULED-TRANSFER-AC-FR-041-4-p2341c6` |
| `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional-pab4508-c` | `TCND-SCHEDULED-TRANSFER-EP-CreateSchedule-functional-pab4508` | condition | Tạo lịch ONCE hợp lệ thì trả 201, có scheduleId, status ACTIVE. |  | 201, có scheduleId, status=ACTIVE. | `TORC-SCHEDULED-TRANSFER-AC-FR-040-1-pab4508` |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-Create-200-functional-c` | `TCND-SCHEDULED-TRANSFER-RSP-Create-200-functional` | condition | POST với Idempotency-Key đã dùng và nội dung giống hệt lần trước. |  | HTTP 200, body chứa lịch đã tạo với scheduleId ban đầu. | `TORC-SCHEDULED-TRANSFER-RSP-Create-200` |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-Create-201-functional-c` | `TCND-SCHEDULED-TRANSFER-RSP-Create-201-functional` | condition | POST với đầu vào hợp lệ, Idempotency-Key mới, tài khoản nguồn thuộc chủ API key. |  | HTTP 201, scheduleId không rỗng, status=ACTIVE; số dư và số bản ghi ledger_entries không đổi. | `TORC-SCHEDULED-TRANSFER-RSP-Create-201` |

### TD-SCHEDULED-TRANSFER-L2-EP-GetSchedule-accessMatrix

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-GetSchedule-security` (L2, security)
- Kỹ thuật: accessMatrix; tiêu chí phủ: Mỗi điểm thực thi quyền × {cho phép, từ chối}; mục tiêu 100 %
- Lý do: accessMatrix phù hợp access

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-Get-401-access-c` | `TCND-SCHEDULED-TRANSFER-RSP-Get-401-access` | condition | Gọi GET với API key sai hoặc không hợp lệ trong header Authorization. |  | HTTP 401, errorCode = UNAUTHORIZED, không lộ dữ liệu lịch. | `TORC-SCHEDULED-TRANSFER-RSP-Get-401` |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-Get-403-access-c` | `TCND-SCHEDULED-TRANSFER-RSP-Get-403-access` | condition | API key hợp lệ của người gọi không phải chủ tài khoản nguồn của lịch. |  | HTTP 403, errorCode = FORBIDDEN, không chứa status/nextRunAt. | `TORC-SCHEDULED-TRANSFER-RSP-Get-403` |

### TD-SCHEDULED-TRANSFER-L2-EP-GetSchedule-equivalencePartitioning

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-GetSchedule-functional` (L2, functional)
- Kỹ thuật: equivalencePartitioning; tiêu chí phủ: Mỗi lớp tương đương một coverage item; mục tiêu 100 %
- Lý do: equivalencePartitioning phù hợp inputValidation

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Get-Authorization-inputValidation-missing-v0` | `TCND-SCHEDULED-TRANSFER-FLD-Get-Authorization-inputValidation-missing` | partitionValue | Header Authorization bị thiếu (giá trị không hợp lệ), scheduleId hợp lệ. | <thiếu> | HTTP 401, body có errorCode = UNAUTHORIZED, không chứa status hay nextRunAt. | `TORC-SCHEDULED-TRANSFER-RSP-Get-401-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Get-scheduleId-inputValidation-missing-v0` | `TCND-SCHEDULED-TRANSFER-FLD-Get-scheduleId-inputValidation-missing` | partitionValue | Path scheduleId bị thiếu (rỗng), API key hợp lệ. | <thiếu> | Yêu cầu bị từ chối, không có phản hồi 200; mã HTTP và errorCode chờ làm rõ. | `TORC-SCHEDULED-TRANSFER-EP-GetSchedule-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-Get-404-inputValidation-c` | `TCND-SCHEDULED-TRANSFER-RSP-Get-404-inputValidation` | condition | scheduleId đúng định dạng nhưng không có lịch tương ứng. |  | HTTP 404, errorCode = SCHEDULE_NOT_FOUND. | `TORC-SCHEDULED-TRANSFER-RSP-Get-404` |

### TD-SCHEDULED-TRANSFER-L2-EP-GetSchedule-errorGuessing

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-GetSchedule-reliability` (L2, reliability)
- Kỹ thuật: errorGuessing; tiêu chí phủ: Mỗi tình huống lỗi đoán trước một coverage item; mục tiêu 100 %
- Lý do: errorGuessing phù hợp idempotency

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-EP-GetSchedule-idempotency-p52c40c-c` | `TCND-SCHEDULED-TRANSFER-EP-GetSchedule-idempotency-p52c40c` | condition | Gọi GET lặp lại nhiều lần trên cùng một lịch không đổi; so sánh phản hồi và dữ liệu lưu. |  | Hai phản hồi 200 có status và nextRunAt giống nhau; dữ liệu lịch trong kho không đổi sau các lần GET. | `TORC-SCHEDULED-TRANSFER-EP-GetSchedule-p52c40c` |

### TD-SCHEDULED-TRANSFER-L2-EP-GetSchedule-useCase

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-GetSchedule-functional` (L2, functional)
- Kỹ thuật: useCase; tiêu chí phủ: Mỗi tiêu chí chấp nhận một coverage item; mục tiêu 100 %
- Lý do: useCase phù hợp functional

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-Get-200-functional-c` | `TCND-SCHEDULED-TRANSFER-RSP-Get-200-functional` | condition | Chủ tài khoản nguồn gọi GET với API key hợp lệ và scheduleId của lịch tồn tại. |  | HTTP 200, body chứa status và nextRunAt khớp với dữ liệu lịch đã lưu. | `TORC-SCHEDULED-TRANSFER-RSP-Get-200` |

### TD-SCHEDULED-TRANSFER-L2-EP-ListRuns-accessMatrix

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-ListRuns-security` (L2, security)
- Kỹ thuật: accessMatrix; tiêu chí phủ: Mỗi điểm thực thi quyền × {cho phép, từ chối}; mục tiêu 100 %
- Lý do: accessMatrix phù hợp access

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-ListRuns-401-access-c` | `TCND-SCHEDULED-TRANSFER-RSP-ListRuns-401-access` | condition | Gọi GET /runs với API key thiếu hoặc sai; kỳ vọng 401 UNAUTHORIZED. |  | HTTP 401, errorCode UNAUTHORIZED | `TORC-SCHEDULED-TRANSFER-RSP-ListRuns-401` |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-ListRuns-403-access-c` | `TCND-SCHEDULED-TRANSFER-RSP-ListRuns-403-access` | condition | API key hợp lệ của người không phải chủ tài khoản nguồn gọi GET /runs; kỳ vọng 403 FORBIDDEN. |  | HTTP 403, errorCode FORBIDDEN | `TORC-SCHEDULED-TRANSFER-RSP-ListRuns-403` |

### TD-SCHEDULED-TRANSFER-L2-EP-ListRuns-boundaryValue

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-ListRuns-functional` (L2, functional)
- Kỹ thuật: boundaryValue; tiêu chí phủ: Mỗi biên hai giá trị (trong và ngoài biên); mục tiêu 100 %
- Lý do: boundaryValue phù hợp inputValidation

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListRuns-limit-inputValidation-max-v0` | `TCND-SCHEDULED-TRANSFER-FLD-ListRuns-limit-inputValidation-max` | validBoundary | limit = 100, biên trên hợp lệ, được chấp nhận. | 100 | limit (query) = 100 được chấp nhận | `TORC-SCHEDULED-TRANSFER-EP-ListRuns-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListRuns-limit-inputValidation-max-v1` | `TCND-SCHEDULED-TRANSFER-FLD-ListRuns-limit-inputValidation-max` | partitionValue | limit = 101, vượt cận trên, bị từ chối. | 101 | Yêu cầu bị từ chối với lỗi kiểm tra đầu vào; không có dữ liệu lần chạy | `TORC-SCHEDULED-TRANSFER-EP-ListRuns-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListRuns-limit-inputValidation-min-v0` | `TCND-SCHEDULED-TRANSFER-FLD-ListRuns-limit-inputValidation-min` | partitionValue | limit = 0, dưới cận dưới, bị từ chối. | 0 | Yêu cầu bị từ chối với lỗi kiểm tra đầu vào; không có dữ liệu lần chạy | `TORC-SCHEDULED-TRANSFER-EP-ListRuns-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListRuns-limit-inputValidation-min-v1` | `TCND-SCHEDULED-TRANSFER-FLD-ListRuns-limit-inputValidation-min` | validBoundary | limit = 1, biên dưới hợp lệ, được chấp nhận. | 1 | limit (query) = 1 được chấp nhận | `TORC-SCHEDULED-TRANSFER-EP-ListRuns-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListRuns-offset-inputValidation-min-v0` | `TCND-SCHEDULED-TRANSFER-FLD-ListRuns-offset-inputValidation-min` | partitionValue | offset = -1, dưới cận dưới, bị từ chối. | -1 | Yêu cầu bị từ chối với lỗi kiểm tra đầu vào; không có dữ liệu lần chạy | `TORC-SCHEDULED-TRANSFER-EP-ListRuns-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListRuns-offset-inputValidation-min-v1` | `TCND-SCHEDULED-TRANSFER-FLD-ListRuns-offset-inputValidation-min` | validBoundary | offset = 0, biên dưới hợp lệ, được chấp nhận. | 0 | offset (query) = 0 được chấp nhận | `TORC-SCHEDULED-TRANSFER-EP-ListRuns-validation` |

### TD-SCHEDULED-TRANSFER-L2-EP-ListRuns-equivalencePartitioning

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-ListRuns-functional` (L2, functional)
- Kỹ thuật: equivalencePartitioning; tiêu chí phủ: Mỗi lớp tương đương một coverage item; mục tiêu 100 %
- Lý do: equivalencePartitioning phù hợp inputValidation

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListRuns-Authorization-inputValidation-missing-v0` | `TCND-SCHEDULED-TRANSFER-FLD-ListRuns-Authorization-inputValidation-missing` | partitionValue | Authorization (header) thiếu, không hợp lệ; kỳ vọng 401 UNAUTHORIZED. | <thiếu> | HTTP 401, errorCode UNAUTHORIZED | `TORC-SCHEDULED-TRANSFER-RSP-ListRuns-401-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListRuns-scheduleId-inputValidation-missing-v0` | `TCND-SCHEDULED-TRANSFER-FLD-ListRuns-scheduleId-inputValidation-missing` | partitionValue | scheduleId (path) thiếu, không hợp lệ, bị từ chối. | <thiếu> | Yêu cầu bị từ chối với lỗi kiểm tra đầu vào; không có dữ liệu lần chạy | `TORC-SCHEDULED-TRANSFER-EP-ListRuns-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-ListRuns-404-inputValidation-c` | `TCND-SCHEDULED-TRANSFER-RSP-ListRuns-404-inputValidation` | condition | scheduleId không tương ứng lịch nào; kỳ vọng 404 SCHEDULE_NOT_FOUND. |  | HTTP 404, errorCode SCHEDULE_NOT_FOUND | `TORC-SCHEDULED-TRANSFER-RSP-ListRuns-404` |

### TD-SCHEDULED-TRANSFER-L2-EP-ListRuns-useCase

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-ListRuns-functional` (L2, functional)
- Kỹ thuật: useCase; tiêu chí phủ: Mỗi tiêu chí chấp nhận một coverage item; mục tiêu 100 %
- Lý do: useCase phù hợp functional

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-ListRuns-200-functional-c` | `TCND-SCHEDULED-TRANSFER-RSP-ListRuns-200-functional` | condition | Chủ lịch gọi GET /runs cho lịch tồn tại với Authorization hợp lệ; kỳ vọng 200. |  | HTTP 200; danh sách giảm dần theo scheduledAt; mỗi phần tử có scheduledAt, status, transferId, reasonCode | `TORC-SCHEDULED-TRANSFER-RSP-ListRuns-200` |

### TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-accessMatrix

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules-security` (L2, security)
- Kỹ thuật: accessMatrix; tiêu chí phủ: Mỗi điểm thực thi quyền × {cho phép, từ chối}; mục tiêu 100 %
- Lý do: accessMatrix phù hợp access

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-ListSchedules-401-access-c` | `TCND-SCHEDULED-TRANSFER-RSP-ListSchedules-401-access` | condition | Gọi endpoint với API key thiếu hoặc sai. |  | HTTP 401, errorCode UNAUTHORIZED. | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-401` |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-ListSchedules-403-access-c` | `TCND-SCHEDULED-TRANSFER-RSP-ListSchedules-403-access` | condition | API key hợp lệ nhưng accountId trong path khác tài khoản của người gọi. |  | HTTP 403, errorCode FORBIDDEN, body không chứa lịch của B. | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-403` |

### TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-boundaryValue

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules-functional` (L2, functional)
- Kỹ thuật: boundaryValue; tiêu chí phủ: Mỗi biên hai giá trị (trong và ngoài biên); mục tiêu 100 %
- Lý do: boundaryValue phù hợp inputValidation

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListSchedules-limit-inputValidation-max-v0` | `TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-limit-inputValidation-max` | validBoundary | limit=100, biên trên hợp lệ. | 100 | limit (query) = 100 được chấp nhận | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListSchedules-limit-inputValidation-max-v1` | `TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-limit-inputValidation-max` | partitionValue | limit=101, vượt biên trên (không hợp lệ). | 101 | Giá trị trong enum: HTTP 200. Giá trị ngoài enum: HTTP 400, errorCode INVALID_REQUEST. | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListSchedules-limit-inputValidation-min-v0` | `TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-limit-inputValidation-min` | partitionValue | limit=0, dưới biên dưới (không hợp lệ). | 0 | Giá trị trong enum: HTTP 200. Giá trị ngoài enum: HTTP 400, errorCode INVALID_REQUEST. | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListSchedules-limit-inputValidation-min-v1` | `TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-limit-inputValidation-min` | validBoundary | limit=1, biên dưới hợp lệ. | 1 | limit (query) = 1 được chấp nhận | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListSchedules-offset-inputValidation-min-v0` | `TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-offset-inputValidation-min` | partitionValue | offset=-1, dưới biên dưới (không hợp lệ). | -1 | Giá trị trong enum: HTTP 200. Giá trị ngoài enum: HTTP 400, errorCode INVALID_REQUEST. | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListSchedules-offset-inputValidation-min-v1` | `TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-offset-inputValidation-min` | validBoundary | offset=0, biên dưới hợp lệ. | 0 | offset (query) = 0 được chấp nhận | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-400-validation` |

### TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-equivalencePartitioning

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules-functional` (L2, functional)
- Kỹ thuật: equivalencePartitioning; tiêu chí phủ: Mỗi lớp tương đương một coverage item; mục tiêu 100 %
- Lý do: equivalencePartitioning phù hợp inputValidation

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListSchedules-Authorization-inputValidation-missing-v0` | `TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-Authorization-inputValidation-missing` | partitionValue | Thiếu header Authorization (không hợp lệ). | <thiếu header Authorization> | HTTP 401, errorCode UNAUTHORIZED, body không chứa phần tử lịch. | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-401-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListSchedules-accountId-inputValidation-missing-v0` | `TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-accountId-inputValidation-missing` | partitionValue | Thiếu đoạn path accountId (không hợp lệ). | <thiếu accountId> | Giá trị trong enum: HTTP 200. Giá trị ngoài enum: HTTP 400, errorCode INVALID_REQUEST. | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListSchedules-status-inputValidation-enum-v0` | `TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-status-inputValidation-enum` | validClass | status=ACTIVE (hợp lệ). | ACTIVE | status (query) = ACTIVE được chấp nhận | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListSchedules-status-inputValidation-enum-v1` | `TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-status-inputValidation-enum` | validClass | status=PAUSED (hợp lệ). | PAUSED | status (query) = PAUSED được chấp nhận | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListSchedules-status-inputValidation-enum-v2` | `TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-status-inputValidation-enum` | validClass | status=COMPLETED (hợp lệ). | COMPLETED | status (query) = COMPLETED được chấp nhận | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListSchedules-status-inputValidation-enum-v3` | `TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-status-inputValidation-enum` | validClass | status=CANCELLED (hợp lệ). | CANCELLED | status (query) = CANCELLED được chấp nhận | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListSchedules-status-inputValidation-enum-v4` | `TCND-SCHEDULED-TRANSFER-FLD-ListSchedules-status-inputValidation-enum` | partitionValue | status=UNKNOWN, ngoài enum (không hợp lệ). | UNKNOWN | Giá trị trong enum: HTTP 200. Giá trị ngoài enum: HTTP 400, errorCode INVALID_REQUEST. | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-ListSchedules-400-inputValidation-c` | `TCND-SCHEDULED-TRANSFER-RSP-ListSchedules-400-inputValidation` | condition | Gọi với status ngoài enum ACTIVE, PAUSED, COMPLETED, CANCELLED. |  | HTTP 400, errorCode INVALID_REQUEST. | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-400` |

### TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-errorGuessing

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules-reliability` (L2, reliability)
- Kỹ thuật: errorGuessing; tiêu chí phủ: Mỗi tình huống lỗi đoán trước một coverage item; mục tiêu 100 %
- Lý do: errorGuessing phù hợp idempotency

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-idempotency-p6e1377-c` | `TCND-SCHEDULED-TRANSFER-EP-ListSchedules-idempotency-p6e1377` | condition | Gọi lặp lại cùng GET danh sách lịch với cùng tham số khi dữ liệu không đổi. |  | Hai phản hồi HTTP 200 có cùng nội dung; trạng thái lịch và lần chạy không đổi. | `TORC-SCHEDULED-TRANSFER-EP-ListSchedules-p6e1377` |

### TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-useCase

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules-functional` (L2, functional)
- Kỹ thuật: useCase; tiêu chí phủ: Mỗi tiêu chí chấp nhận một coverage item; mục tiêu 100 %
- Lý do: useCase phù hợp functional

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-ListSchedules-200-functional-c` | `TCND-SCHEDULED-TRANSFER-RSP-ListSchedules-200-functional` | condition | Chủ tài khoản có API key hợp lệ gọi danh sách của chính mình với status, limit, offset hợp lệ. |  | HTTP 200; mọi phần tử thuộc accountId và có status=ACTIVE; số phần tử <= 2; bắt đầu từ vị trí offset=1 của tập đã lọc. | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-200` |

### TD-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-accessMatrix

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-security` (L2, security)
- Kỹ thuật: accessMatrix; tiêu chí phủ: Mỗi điểm thực thi quyền × {cho phép, từ chối}; mục tiêu 100 %
- Lý do: accessMatrix phù hợp access

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-Patch-401-access-c` | `TCND-SCHEDULED-TRANSFER-RSP-Patch-401-access` | condition | PATCH với API key thiếu hoặc sai. |  | HTTP 401, errorCode UNAUTHORIZED. | `TORC-SCHEDULED-TRANSFER-RSP-Patch-401` |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-Patch-403-access-c` | `TCND-SCHEDULED-TRANSFER-RSP-Patch-403-access` | condition | API key hợp lệ nhưng người gọi không phải chủ lịch. |  | HTTP 403, errorCode FORBIDDEN, lịch giữ nguyên. | `TORC-SCHEDULED-TRANSFER-RSP-Patch-403` |

### TD-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-equivalencePartitioning

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-functional` (L2, functional)
- Kỹ thuật: equivalencePartitioning; tiêu chí phủ: Mỗi lớp tương đương một coverage item; mục tiêu 100 %
- Lý do: equivalencePartitioning phù hợp inputValidation

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Patch-Authorization-inputValidation-missing-v0` | `TCND-SCHEDULED-TRANSFER-FLD-Patch-Authorization-inputValidation-missing` | partitionValue | Authorization (header) = <thiếu>, không hợp lệ. | <thiếu> | HTTP 401, errorCode UNAUTHORIZED, trạng thái lịch giữ nguyên. | `TORC-SCHEDULED-TRANSFER-RSP-Patch-401-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Patch-scheduleId-inputValidation-missing-v0` | `TCND-SCHEDULED-TRANSFER-FLD-Patch-scheduleId-inputValidation-missing` | partitionValue | scheduleId (path) = <thiếu>, không hợp lệ; oracle cần xác nhận. | <thiếu> | HTTP 400, errorCode INVALID_REQUEST, lịch giữ nguyên trạng thái. | `TORC-SCHEDULED-TRANSFER-RSP-Patch-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Patch-status-inputValidation-enum-v0` | `TCND-SCHEDULED-TRANSFER-FLD-Patch-status-inputValidation-enum` | validClass | status (body) = PAUSED, hợp lệ. | PAUSED | status (body) = PAUSED được chấp nhận | `TORC-SCHEDULED-TRANSFER-RSP-Patch-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Patch-status-inputValidation-enum-v1` | `TCND-SCHEDULED-TRANSFER-FLD-Patch-status-inputValidation-enum` | validClass | status (body) = ACTIVE, hợp lệ. | ACTIVE | status (body) = ACTIVE được chấp nhận | `TORC-SCHEDULED-TRANSFER-RSP-Patch-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Patch-status-inputValidation-enum-v2` | `TCND-SCHEDULED-TRANSFER-FLD-Patch-status-inputValidation-enum` | partitionValue | status (body) = <ngoài tập>, không hợp lệ. | <ngoài tập> | HTTP 400, errorCode INVALID_REQUEST, lịch giữ nguyên trạng thái. | `TORC-SCHEDULED-TRANSFER-RSP-Patch-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-FLD-Patch-status-inputValidation-missing-v0` | `TCND-SCHEDULED-TRANSFER-FLD-Patch-status-inputValidation-missing` | partitionValue | status (body) = <thiếu>, không hợp lệ. | <thiếu> | HTTP 400, errorCode INVALID_REQUEST, lịch giữ nguyên trạng thái. | `TORC-SCHEDULED-TRANSFER-RSP-Patch-400-validation` |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-Patch-400-inputValidation-c` | `TCND-SCHEDULED-TRANSFER-RSP-Patch-400-inputValidation` | condition | PATCH với status thiếu hoặc ngoài PAUSED/ACTIVE. |  | HTTP 400, errorCode INVALID_REQUEST. | `TORC-SCHEDULED-TRANSFER-RSP-Patch-400` |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-Patch-404-inputValidation-c` | `TCND-SCHEDULED-TRANSFER-RSP-Patch-404-inputValidation` | condition | PATCH với scheduleId không tồn tại. |  | HTTP 404, errorCode SCHEDULE_NOT_FOUND. | `TORC-SCHEDULED-TRANSFER-RSP-Patch-404` |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-Patch-409-inputValidation-c` | `TCND-SCHEDULED-TRANSFER-RSP-Patch-409-inputValidation` | condition | PAUSED trên lịch không ACTIVE, ACTIVE trên lịch không PAUSED, hoặc PATCH trên lịch ONCE. |  | HTTP 409, errorCode SCHEDULE_STATE_INVALID. | `TORC-SCHEDULED-TRANSFER-RSP-Patch-409` |

### TD-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-faultInjection

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-reliability` (L2, reliability)
- Kỹ thuật: faultInjection; tiêu chí phủ: Mỗi mô hình lỗi được tiêm ít nhất một lần; mục tiêu 100 %
- Lý do: faultInjection phù hợp concurrency

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-concurrency-p1e8657-c` | `TCND-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-concurrency-p1e8657` | condition | Scheduler chiếm lần chạy đến hạn đồng thời với PATCH PAUSED. |  | Cần xác nhận: lần chạy đã chiếm có tiếp tục chạy hay bị hủy/SKIPPED. | `TORC-SCHEDULED-TRANSFER-MOD-Scheduler-p1e8657` |

### TD-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-scenario

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-functional` (L2, functional)
- Kỹ thuật: scenario; tiêu chí phủ: Mỗi đường đi (chính, lỗi) một coverage item; mục tiêu 100 %
- Lý do: scenario phù hợp audit

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-audit-p664d35-c` | `TCND-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-audit-p664d35` | condition | Chuyển trạng thái thành công (PAUSED hoặc ACTIVE) và kiểm bản ghi audit. |  | Tồn tại bản ghi audit đủ các trường correlationId, thời điểm, tác nhân = mã tài khoản, loại sự kiện, mã lịch. | `TORC-SCHEDULED-TRANSFER-AC-FR-048-1-p664d35` |

### TD-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-useCase

- TR: `TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-functional` (L2, functional)
- Kỹ thuật: useCase; tiêu chí phủ: Mỗi tiêu chí chấp nhận một coverage item; mục tiêu 100 %
- Lý do: useCase phù hợp functional

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-RSP-Patch-200-functional-c` | `TCND-SCHEDULED-TRANSFER-RSP-Patch-200-functional` | condition | PAUSED trên lịch định kỳ ACTIVE và ACTIVE trên lịch PAUSED, bởi chủ lịch. |  | HTTP 200, body lịch có status = giá trị yêu cầu; sau PAUSED không phát sinh lần chạy mới; sau ACTIVE các lần bỏ lỡ là SKIPPED và không chạy bù. | `TORC-SCHEDULED-TRANSFER-RSP-Patch-200` |

### TD-SCHEDULED-TRANSFER-L2-FR-040-accessMatrix

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-040-security` (L2, security)
- Kỹ thuật: accessMatrix; tiêu chí phủ: Mỗi điểm thực thi quyền × {cho phép, từ chối}; mục tiêu 100 %
- Lý do: accessMatrix phù hợp access

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-040-access-p4f809f-c` | `TCND-SCHEDULED-TRANSFER-FR-040-access-p4f809f` | condition | Hai ca: thiếu API key và API key sai, đều kỳ vọng 401 UNAUTHORIZED. |  | HTTP 401, errorCode=UNAUTHORIZED; không có bản ghi lịch mới. | `TORC-SCHEDULED-TRANSFER-RSP-Create-401-p4f809f` |

### TD-SCHEDULED-TRANSFER-L2-FR-040-decisionTable

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-040-functional` (L2, functional)
- Kỹ thuật: decisionTable; tiêu chí phủ: Mỗi dòng bảng quyết định; mục tiêu 100 %
- Lý do: decisionTable phù hợp decisionRule

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-040-decisionRule-pc6e1a6-c` | `TCND-SCHEDULED-TRANSFER-FR-040-decisionRule-pc6e1a6` | condition | Biên amount 50.000.000 (chấp nhận), 50.000.001 (từ chối) và trường hợp trùng tài khoản. |  | HTTP 422 với errorCode LIMIT_EXCEEDED hoặc SAME_ACCOUNT tương ứng; không có bản ghi lịch mới. | `TORC-SCHEDULED-TRANSFER-RSP-Create-422-amount-pc6e1a6` |

### TD-SCHEDULED-TRANSFER-L2-FR-040-equivalencePartitioning

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-040-functional` (L2, functional)
- Kỹ thuật: equivalencePartitioning; tiêu chí phủ: Mỗi lớp tương đương một coverage item; mục tiêu 100 %
- Lý do: equivalencePartitioning phù hợp inputValidation

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-040-inputValidation-p49f012-c` | `TCND-SCHEDULED-TRANSFER-FR-040-inputValidation-p49f012` | condition | Từng ca đầu vào sai định dạng/thiếu trường, kỳ vọng 400 INVALID_REQUEST. |  | HTTP 400, errorCode=INVALID_REQUEST cho mỗi trường hợp; không có bản ghi lịch mới. | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-p49f012` |

### TD-SCHEDULED-TRANSFER-L2-FR-040-errorGuessing

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-040-reliability` (L2, reliability)
- Kỹ thuật: errorGuessing; tiêu chí phủ: Mỗi tình huống lỗi đoán trước một coverage item; mục tiêu 100 %
- Lý do: errorGuessing phù hợp idempotency

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-040-idempotency-p8f0346-c` | `TCND-SCHEDULED-TRANSFER-FR-040-idempotency-p8f0346` | condition | Gửi lại cùng Idempotency-Key và cùng nội dung. |  | HTTP 200, cùng scheduleId như lần đầu; chỉ có một bản ghi lịch. | `TORC-SCHEDULED-TRANSFER-RSP-Create-200-p8f0346` |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-040-idempotency-pccb8db-c` | `TCND-SCHEDULED-TRANSFER-FR-040-idempotency-pccb8db` | condition | Gửi cùng Idempotency-Key với nội dung khác. |  | HTTP 409, errorCode=IDEMPOTENCY_CONFLICT; số lịch không đổi. | `TORC-SCHEDULED-TRANSFER-RSP-Create-409-pccb8db` |

### TD-SCHEDULED-TRANSFER-L2-FR-040-faultInjection

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-040-reliability` (L2, reliability)
- Kỹ thuật: faultInjection; tiêu chí phủ: Mỗi mô hình lỗi được tiêm ít nhất một lần; mục tiêu 100 %
- Lý do: faultInjection phù hợp concurrency

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-040-concurrency-pd7d28d-c` | `TCND-SCHEDULED-TRANSFER-FR-040-concurrency-pd7d28d` | condition | Hai yêu cầu song song khi tài khoản có 19 lịch ACTIVE/PAUSED. |  | Số lịch ACTIVE/PAUSED của tài khoản luôn ≤ 20. | `TORC-SCHEDULED-TRANSFER-DE-ScheduledTransfer-pd7d28d` |

### TD-SCHEDULED-TRANSFER-L2-FR-040-scenario

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-040-functional` (L2, functional)
- Kỹ thuật: scenario; tiêu chí phủ: Mỗi đường đi (chính, lỗi) một coverage item; mục tiêu 100 %
- Lý do: scenario phù hợp audit

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-040-audit-p7a419a-c` | `TCND-SCHEDULED-TRANSFER-FR-040-audit-p7a419a` | condition | Kiểm bản ghi audit sau khi tạo lịch thành công. |  | Tồn tại bản ghi audit tạo lịch cho scheduleId đã trả. | `TORC-SCHEDULED-TRANSFER-FR-048-p7a419a` |

### TD-SCHEDULED-TRANSFER-L2-FR-040-useCase

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-040-functional` (L2, functional)
- Kỹ thuật: useCase; tiêu chí phủ: Mỗi tiêu chí chấp nhận một coverage item; mục tiêu 100 %
- Lý do: useCase phù hợp functional

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-040-1-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-040-1-functional` | condition | Tạo lịch ONCE với dữ liệu hợp lệ đầy đủ, kỳ vọng 201 và ACTIVE. |  | HTTP 201; body có scheduleId không rỗng và status=ACTIVE. | `TORC-SCHEDULED-TRANSFER-AC-FR-040-1` |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-040-2-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-040-2-functional` | condition | So sánh số dư và ledger_entries trước/sau khi tạo lịch. |  | Số dư hai tài khoản không đổi; số bản ghi ledger_entries sau bằng số trước. | `TORC-SCHEDULED-TRANSFER-AC-FR-040-2` |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-040-3-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-040-3-functional` | condition | firstRunAt tại các biên quanh 5 phút và 90 ngày, kỳ vọng SCHEDULE_TIME_INVALID khi ngoài khoảng. |  | HTTP 422, errorCode=SCHEDULE_TIME_INVALID cho giá trị ngoài khoảng; không có bản ghi lịch mới. | `TORC-SCHEDULED-TRANSFER-AC-FR-040-3` |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-040-4-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-040-4-functional` | condition | toAccountId không tồn tại, kỳ vọng 404 ACCOUNT_NOT_FOUND. |  | HTTP 404, errorCode=ACCOUNT_NOT_FOUND; không có bản ghi lịch mới. | `TORC-SCHEDULED-TRANSFER-AC-FR-040-4` |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-040-5-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-040-5-functional` | condition | fromAccountId không thuộc chủ API key, kỳ vọng 403 FORBIDDEN. |  | HTTP 403, errorCode=FORBIDDEN; không có bản ghi lịch mới. | `TORC-SCHEDULED-TRANSFER-AC-FR-040-5` |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-040-6-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-040-6-functional` | condition | Biên số lịch 19→20 thành công và 20→21 bị từ chối. |  | HTTP 422, errorCode=SCHEDULE_LIMIT_REACHED; số lịch ACTIVE/PAUSED của tài khoản vẫn là 20. | `TORC-SCHEDULED-TRANSFER-AC-FR-040-6` |

### TD-SCHEDULED-TRANSFER-L2-FR-041-accessMatrix

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-041-security` (L2, security)
- Kỹ thuật: accessMatrix; tiêu chí phủ: Mỗi điểm thực thi quyền × {cho phép, từ chối}; mục tiêu 100 %
- Lý do: accessMatrix phù hợp access

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-041-access-pfae77d-c` | `TCND-SCHEDULED-TRANSFER-FR-041-access-pfae77d` | condition | Từ chối khi fromAccountId không thuộc người gọi (403) hoặc API key thiếu/sai (401). |  | HTTP 403 FORBIDDEN cho sai chủ tài khoản; HTTP 401 UNAUTHORIZED cho API key thiếu/sai. | `TORC-SCHEDULED-TRANSFER-RSP-Create-403-pfae77d` |

### TD-SCHEDULED-TRANSFER-L2-FR-041-equivalencePartitioning

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-041-functional` (L2, functional)
- Kỹ thuật: equivalencePartitioning; tiêu chí phủ: Mỗi lớp tương đương một coverage item; mục tiêu 100 %
- Lý do: equivalencePartitioning phù hợp inputValidation

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-041-inputValidation-p31ec2a-c` | `TCND-SCHEDULED-TRANSFER-FR-041-inputValidation-p31ec2a` | condition | maxRuns không nguyên hoặc ngoài 1..60 bị 400 INVALID_REQUEST; biên 0, 1, 60, 61. |  | HTTP 400, errorCode=INVALID_REQUEST; kiểm tra biên 0, 1, 60, 61. | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-p31ec2a` |

### TD-SCHEDULED-TRANSFER-L2-FR-041-errorGuessing

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-041-reliability` (L2, reliability)
- Kỹ thuật: errorGuessing; tiêu chí phủ: Mỗi tình huống lỗi đoán trước một coverage item; mục tiêu 100 %
- Lý do: errorGuessing phù hợp idempotency

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-041-idempotency-pd36026-c` | `TCND-SCHEDULED-TRANSFER-FR-041-idempotency-pd36026` | condition | Gửi lại cùng Idempotency-Key: cùng nội dung trả 200 không tạo thêm lịch; khác nội dung trả 409. |  | Cùng nội dung: HTTP 200, cùng mã lịch, tổng số lịch không đổi. Khác nội dung: HTTP 409, IDEMPOTENCY_CONFLICT. | `TORC-SCHEDULED-TRANSFER-RSP-Create-409-pd36026` |

### TD-SCHEDULED-TRANSFER-L2-FR-041-scenario

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-041-functional` (L2, functional)
- Kỹ thuật: scenario; tiêu chí phủ: Mỗi đường đi (chính, lỗi) một coverage item; mục tiêu 100 %
- Lý do: scenario phù hợp audit

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-041-audit-p1cd2c4-c` | `TCND-SCHEDULED-TRANSFER-FR-041-audit-p1cd2c4` | condition | Tạo lịch thành công có bản ghi audit theo mã lịch. |  | Tồn tại bản ghi audit tạo lịch liên kết mã lịch; trường tác nhân cụ thể cho thao tác tạo của người dùng chưa nêu trong nguồn. | `TORC-SCHEDULED-TRANSFER-FR-048-p1cd2c4` |

### TD-SCHEDULED-TRANSFER-L2-FR-041-stateTransition

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-041-functional` (L2, functional)
- Kỹ thuật: stateTransition; tiêu chí phủ: Mỗi chuyển trạng thái và mỗi guard bị vi phạm; mục tiêu 100 %
- Lý do: stateTransition phù hợp guardViolation

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-041-guardViolation-p241776-c` | `TCND-SCHEDULED-TRANSFER-FR-041-guardViolation-p241776` | condition | endDate trước lần chạy đầu hoặc quá 366 ngày bị 422 SCHEDULE_TIME_INVALID; biên 366/367 ngày. |  | HTTP 422, errorCode=SCHEDULE_TIME_INVALID; kiểm tra biên 366 và 367 ngày. | `TORC-SCHEDULED-TRANSFER-RSP-Create-422-time-p241776` |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-041-guardViolation-pcbab3f-c` | `TCND-SCHEDULED-TRANSFER-FR-041-guardViolation-pcbab3f` | condition | Tài khoản đã có 20 lịch ACTIVE/PAUSED bị 422 SCHEDULE_LIMIT_REACHED; 19 lịch vẫn tạo được. |  | HTTP 422, errorCode=SCHEDULE_LIMIT_REACHED; với 19 lịch thì tạo thành công. | `TORC-SCHEDULED-TRANSFER-RSP-Create-422-limit-pcbab3f` |

### TD-SCHEDULED-TRANSFER-L2-FR-041-useCase

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-041-functional` (L2, functional)
- Kỹ thuật: useCase; tiêu chí phủ: Mỗi tiêu chí chấp nhận một coverage item; mục tiêu 100 %
- Lý do: useCase phù hợp functional

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-041-1-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-041-1-functional` | condition | Tạo lịch hợp lệ cho từng recurrence DAILY, WEEKLY, MONTHLY và từng cách đặt giới hạn, kỳ vọng 201 ACTIVE. |  | HTTP 201, status=ACTIVE, có mã lịch trong phản hồi; kiểm tra cho từng recurrence và từng cách đặt giới hạn (chỉ endDate, chỉ maxRuns, cả hai). | `TORC-SCHEDULED-TRANSFER-AC-FR-041-1` |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-041-2-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-041-2-functional` | condition | Yêu cầu định kỳ thiếu cả endDate và maxRuns bị 400 INVALID_REQUEST. |  | HTTP 400, errorCode=INVALID_REQUEST; số bản ghi scheduled_transfers của tài khoản không đổi. | `TORC-SCHEDULED-TRANSFER-AC-FR-041-2` |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-041-3-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-041-3-functional` | condition | Scheduler sinh lần chạy PENDING kế tiếp đúng chu kỳ đến khi đạt maxRuns hoặc vượt endDate, rồi COMPLETED. |  | Với maxRuns=N: có đúng N lần chạy, thời điểm cách nhau đúng chu kỳ, lịch COMPLETED sau lần thứ N. Với endDate: không có lần chạy nào sau endDate, lịch COMPLETED. | `TORC-SCHEDULED-TRANSFER-AC-FR-041-3` |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-041-4-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-041-4-functional` | condition | Lịch MONTHLY neo 29/30/31 chạy ngày cuối tháng ở tháng thiếu ngày, xét cả năm nhuận và năm thường. |  | Neo 31: 31/01 → 28/02 (năm thường) hoặc 29/02 (năm nhuận) → 31/03 → 30/04. Neo 30: tháng 2 → ngày cuối tháng. Neo 29: 28/02 năm thường, 29/02 năm nhuận. | `TORC-SCHEDULED-TRANSFER-AC-FR-041-4` |

### TD-SCHEDULED-TRANSFER-L2-FR-042-accessMatrix

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-042-security` (L2, security)
- Kỹ thuật: accessMatrix; tiêu chí phủ: Mỗi điểm thực thi quyền × {cho phép, từ chối}; mục tiêu 100 %
- Lý do: accessMatrix phù hợp access

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-042-access-p2aa8d6-c` | `TCND-SCHEDULED-TRANSFER-FR-042-access-p2aa8d6` | condition | Thiếu hoặc sai API key trên ba endpoint GET tra cứu lịch. |  | HTTP 401 với errorCode UNAUTHORIZED cho cả ba endpoint. | `TORC-SCHEDULED-TRANSFER-RSP-Get-401-p2aa8d6` |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-042-access-p72e502-c` | `TCND-SCHEDULED-TRANSFER-FR-042-access-p72e502` | condition | Danh sách lịch với accountId khác tài khoản của người gọi. |  | HTTP 403 với errorCode FORBIDDEN. | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-403-p72e502` |

### TD-SCHEDULED-TRANSFER-L2-FR-042-equivalencePartitioning

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-042-functional` (L2, functional)
- Kỹ thuật: equivalencePartitioning; tiêu chí phủ: Mỗi lớp tương đương một coverage item; mục tiêu 100 %
- Lý do: equivalencePartitioning phù hợp inputValidation

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-042-inputValidation-p433493-c` | `TCND-SCHEDULED-TRANSFER-FR-042-inputValidation-p433493` | condition | Giá trị biên limit (1, 100) và offset (0) ở danh sách lịch và danh sách runs. |  | limit=1, 100 và offset=0 trả 200; giá trị ngoài biên bị từ chối (mã lỗi cụ thể xem gaps). | `TORC-SCHEDULED-TRANSFER-FLD-ListSchedules-limit-p433493` |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-042-inputValidation-p473406-c` | `TCND-SCHEDULED-TRANSFER-FR-042-inputValidation-p473406` | condition | Danh sách lịch với status ngoài enum cho phép. |  | HTTP 400 với errorCode INVALID_REQUEST. | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-400-p473406` |

### TD-SCHEDULED-TRANSFER-L2-FR-042-useCase

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-042-functional` (L2, functional)
- Kỹ thuật: useCase; tiêu chí phủ: Mỗi tiêu chí chấp nhận một coverage item; mục tiêu 100 %
- Lý do: useCase phù hợp functional

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-042-1-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-042-1-functional` | condition | Tra cứu một lịch đang tồn tại bởi chủ tài khoản nguồn với API key hợp lệ. |  | HTTP 200; body có status và nextRunAt khớp giá trị lưu trong ScheduledTransfer. | `TORC-SCHEDULED-TRANSFER-AC-FR-042-1` |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-042-2-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-042-2-functional` | condition | Danh sách lịch của chính tài khoản người gọi, có/không status, limit, offset. |  | HTTP 200; mọi phần tử thuộc accountId và đúng status lọc; số phần tử <= limit; trang kế tiếp không trùng, không sót phần tử so với trang trước. | `TORC-SCHEDULED-TRANSFER-AC-FR-042-2` |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-042-3-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-042-3-functional` | condition | Danh sách runs của lịch có nhiều lần chạy bởi chủ lịch. |  | HTTP 200; danh sách giảm dần theo scheduledAt; mỗi phần tử có đủ scheduledAt, status, transferId, reasonCode khớp ScheduledRun. | `TORC-SCHEDULED-TRANSFER-AC-FR-042-3` |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-042-4-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-042-4-functional` | condition | GET tra cứu lịch bởi người không phải chủ, hoặc với scheduleId không tồn tại. |  | HTTP 403 mã FORBIDDEN cho người không phải chủ; HTTP 404 mã SCHEDULE_NOT_FOUND cho lịch không tồn tại. | `TORC-SCHEDULED-TRANSFER-AC-FR-042-4` |

### TD-SCHEDULED-TRANSFER-L2-FR-043-accessMatrix

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-043-security` (L2, security)
- Kỹ thuật: accessMatrix; tiêu chí phủ: Mỗi điểm thực thi quyền × {cho phép, từ chối}; mục tiêu 100 %
- Lý do: accessMatrix phù hợp access

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-043-access-p4bf0be-c` | `TCND-SCHEDULED-TRANSFER-FR-043-access-p4bf0be` | condition | DELETE thiếu API key hoặc dùng API key sai. |  | HTTP 401, errorCode UNAUTHORIZED; trạng thái lịch không đổi. | `TORC-SCHEDULED-TRANSFER-RSP-Cancel-401-p4bf0be` |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-043-access-pb30d3e-c` | `TCND-SCHEDULED-TRANSFER-FR-043-access-pb30d3e` | condition | Người không phải chủ lịch gọi DELETE lịch của người khác. |  | HTTP 403, errorCode FORBIDDEN; trạng thái lịch không đổi. | `TORC-SCHEDULED-TRANSFER-RSP-Cancel-403-pb30d3e` |

### TD-SCHEDULED-TRANSFER-L2-FR-043-faultInjection

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-043-reliability` (L2, reliability)
- Kỹ thuật: faultInjection; tiêu chí phủ: Mỗi mô hình lỗi được tiêm ít nhất một lần; mục tiêu 100 %
- Lý do: faultInjection phù hợp concurrency, errorHandling

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-043-concurrency-p4b4873-c` | `TCND-SCHEDULED-TRANSFER-FR-043-concurrency-p4b4873` | condition | Scheduler chiếm lần chạy PENDING cùng lúc DELETE hủy lịch. |  | Không tồn tại lần chạy CANCELLED có giao dịch chuyển tiền được ghi sổ; DELETE trả 200 hoặc 409 tương ứng với kết quả chiếm. | `TORC-SCHEDULED-TRANSFER-FR-043-p4b4873` |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-043-errorHandling-pa62ef8-c` | `TCND-SCHEDULED-TRANSFER-FR-043-errorHandling-pa62ef8` | condition | DELETE với scheduleId không tồn tại. |  | HTTP 404, errorCode SCHEDULE_NOT_FOUND. | `TORC-SCHEDULED-TRANSFER-RSP-Cancel-404-pa62ef8` |

### TD-SCHEDULED-TRANSFER-L2-FR-043-scenario

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-043-functional` (L2, functional)
- Kỹ thuật: scenario; tiêu chí phủ: Mỗi đường đi (chính, lỗi) một coverage item; mục tiêu 100 %
- Lý do: scenario phù hợp audit

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-043-audit-p1438e9-c` | `TCND-SCHEDULED-TRANSFER-FR-043-audit-p1438e9` | condition | Hủy lịch thành công và kiểm tra bản ghi audit tương ứng. |  | Tồn tại bản ghi audit cho thao tác hủy lịch gắn với scheduleId. | `TORC-SCHEDULED-TRANSFER-FR-048-p1438e9` |

### TD-SCHEDULED-TRANSFER-L2-FR-043-useCase

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-043-functional` (L2, functional)
- Kỹ thuật: useCase; tiêu chí phủ: Mỗi tiêu chí chấp nhận một coverage item; mục tiêu 100 %
- Lý do: useCase phù hợp functional

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-043-1-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-043-1-functional` | condition | Chủ lịch hủy lịch ACTIVE có các lần chạy PENDING bằng API key hợp lệ. |  | HTTP 200; ScheduledTransfer.status = CANCELLED; mọi ScheduledRun PENDING của lịch có status = CANCELLED. | `TORC-SCHEDULED-TRANSFER-AC-FR-043-1` |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-043-2-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-043-2-functional` | condition | Chủ lịch gọi lặp DELETE trên lịch đã CANCELLED. |  | HTTP 200, nội dung phản hồi tương đương kết quả hủy ban đầu; trạng thái lịch vẫn CANCELLED. | `TORC-SCHEDULED-TRANSFER-AC-FR-043-2` |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-043-3-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-043-3-functional` | condition | DELETE khi lịch COMPLETED, và khi lịch có ít nhất một lần chạy RUNNING. |  | HTTP 409, errorCode = SCHEDULE_NOT_CANCELLABLE; lịch và lần chạy giữ nguyên trạng thái trước khi gọi. | `TORC-SCHEDULED-TRANSFER-AC-FR-043-3` |

### TD-SCHEDULED-TRANSFER-L2-FR-044-accessMatrix

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-044-security` (L2, security)
- Kỹ thuật: accessMatrix; tiêu chí phủ: Mỗi điểm thực thi quyền × {cho phép, từ chối}; mục tiêu 100 %
- Lý do: accessMatrix phù hợp access

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-044-access-p3b5af1-c` | `TCND-SCHEDULED-TRANSFER-FR-044-access-p3b5af1` | condition | Người gọi không phải chủ lịch: 403 FORBIDDEN. |  | HTTP 403, errorCode=FORBIDDEN, trạng thái lịch không đổi. | `TORC-SCHEDULED-TRANSFER-RSP-Patch-403-p3b5af1` |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-044-access-pf4b5f6-c` | `TCND-SCHEDULED-TRANSFER-FR-044-access-pf4b5f6` | condition | API key thiếu hoặc sai: 401 UNAUTHORIZED. |  | HTTP 401, errorCode=UNAUTHORIZED, trạng thái lịch không đổi. | `TORC-SCHEDULED-TRANSFER-RSP-Patch-401-pf4b5f6` |

### TD-SCHEDULED-TRANSFER-L2-FR-044-equivalencePartitioning

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-044-functional` (L2, functional)
- Kỹ thuật: equivalencePartitioning; tiêu chí phủ: Mỗi lớp tương đương một coverage item; mục tiêu 100 %
- Lý do: equivalencePartitioning phù hợp inputValidation

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-044-inputValidation-p847ccb-c` | `TCND-SCHEDULED-TRANSFER-FR-044-inputValidation-p847ccb` | condition | status thiếu hoặc ngoài PAUSED/ACTIVE: 400 INVALID_REQUEST. |  | HTTP 400, errorCode=INVALID_REQUEST, trạng thái lịch không đổi. | `TORC-SCHEDULED-TRANSFER-RSP-Patch-400-p847ccb` |

### TD-SCHEDULED-TRANSFER-L2-FR-044-faultInjection

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-044-reliability` (L2, reliability)
- Kỹ thuật: faultInjection; tiêu chí phủ: Mỗi mô hình lỗi được tiêm ít nhất một lần; mục tiêu 100 %
- Lý do: faultInjection phù hợp concurrency, errorHandling

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-044-concurrency-p5afd1f-c` | `TCND-SCHEDULED-TRANSFER-FR-044-concurrency-p5afd1f` | condition | PATCH PAUSED đồng thời với scheduler đang chiếm lần chạy: không giao dịch trùng, lịch cuối PAUSED. |  | Không có hai giao dịch cho cùng Idempotency-Key (mã lịch + thời điểm dự kiến); lịch cuối cùng ở PAUSED. | `TORC-SCHEDULED-TRANSFER-RISK-005-p5afd1f` |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-044-errorHandling-pb66379-c` | `TCND-SCHEDULED-TRANSFER-FR-044-errorHandling-pb66379` | condition | scheduleId không tồn tại: 404 SCHEDULE_NOT_FOUND. |  | HTTP 404, errorCode=SCHEDULE_NOT_FOUND. | `TORC-SCHEDULED-TRANSFER-RSP-Patch-404-pb66379` |

### TD-SCHEDULED-TRANSFER-L2-FR-044-scenario

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-044-functional` (L2, functional)
- Kỹ thuật: scenario; tiêu chí phủ: Mỗi đường đi (chính, lỗi) một coverage item; mục tiêu 100 %
- Lý do: scenario phù hợp audit

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-044-audit-pafc81c-c` | `TCND-SCHEDULED-TRANSFER-FR-044-audit-pafc81c` | condition | Mỗi lần tạm dừng/tiếp tục thành công có một bản ghi audit. |  | Có đúng một bản ghi audit cho mỗi lần chuyển trạng thái thành công, tham chiếu scheduleId. | `TORC-SCHEDULED-TRANSFER-FR-048-pafc81c` |

### TD-SCHEDULED-TRANSFER-L2-FR-044-useCase

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-044-functional` (L2, functional)
- Kỹ thuật: useCase; tiêu chí phủ: Mỗi tiêu chí chấp nhận một coverage item; mục tiêu 100 %
- Lý do: useCase phù hợp functional

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-044-1-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-044-1-functional` | condition | Tạm dừng lịch ACTIVE của chủ lịch: 200, PAUSED, scheduler không chạy. |  | HTTP 200, status=PAUSED trong phản hồi và trong bản ghi lịch; không phát sinh giao dịch chuyển tiền mới cho lịch khi qua thời điểm dự kiến. | `TORC-SCHEDULED-TRANSFER-AC-FR-044-1` |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-044-2-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-044-2-functional` | condition | Tiếp tục lịch PAUSED: 200, ACTIVE, các lần bỏ lỡ SKIPPED, không chạy bù. |  | HTTP 200, status=ACTIVE; mọi lần bỏ lỡ trong lúc tạm dừng có status SKIPPED và không có giao dịch tương ứng; chạy bù (FR-049) không áp dụng cho các lần này. | `TORC-SCHEDULED-TRANSFER-AC-FR-044-2` |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-044-3-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-044-3-functional` | condition | Chuyển trạng thái không hợp lệ hoặc lịch ONCE: 409 SCHEDULE_STATE_INVALID. |  | HTTP 409, errorCode=SCHEDULE_STATE_INVALID; bản ghi lịch giữ nguyên status trước đó. | `TORC-SCHEDULED-TRANSFER-AC-FR-044-3` |

### TD-SCHEDULED-TRANSFER-L2-FR-045-boundaryValue

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-045-performance` (L2, performance)
- Kỹ thuật: boundaryValue; tiêu chí phủ: Mỗi biên hai giá trị (trong và ngoài biên); mục tiêu 100 %
- Lý do: boundaryValue phù hợp timing

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-045-timing-pb0682f-c` | `TCND-SCHEDULED-TRANSFER-FR-045-timing-pb0682f` | condition | Độ trễ bắt đầu lần chạy đến hạn không quá 60 giây cho 99% lần chạy. |  | Độ trễ bắt đầu <= 60 giây cho 99% lần chạy. | `TORC-SCHEDULED-TRANSFER-FR-045-pb0682f` |

### TD-SCHEDULED-TRANSFER-L2-FR-045-faultInjection-maintainability

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-045-maintainability` (L2, maintainability)
- Kỹ thuật: faultInjection; tiêu chí phủ: Mỗi mô hình lỗi được tiêm ít nhất một lần; mục tiêu 100 %
- Lý do: faultInjection phù hợp observability

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-045-observability-pd8cb5a-c` | `TCND-SCHEDULED-TRANSFER-FR-045-observability-pd8cb5a` | condition | Mỗi lần chạy có correlationId riêng đi theo giao dịch, audit và lời gọi Fraud Check. |  | 100% lần chạy có correlationId riêng và khớp ở giao dịch, audit, lời gọi Fraud Check. | `TORC-SCHEDULED-TRANSFER-FR-045-pd8cb5a` |

### TD-SCHEDULED-TRANSFER-L2-FR-045-faultInjection-reliability

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-045-reliability` (L2, reliability)
- Kỹ thuật: faultInjection; tiêu chí phủ: Mỗi mô hình lỗi được tiêm ít nhất một lần; mục tiêu 100 %
- Lý do: faultInjection phù hợp concurrency

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-045-concurrency-p174b2f-c` | `TCND-SCHEDULED-TRANSFER-FR-045-concurrency-p174b2f` | condition | Hai scheduler cùng quét một lần chạy đến hạn, chỉ một bên chiếm được bằng UPDATE có điều kiện. |  | Số giao dịch trùng cho một lần chạy = 0. | `TORC-SCHEDULED-TRANSFER-FR-045-p174b2f` |

### TD-SCHEDULED-TRANSFER-L2-FR-045-useCase

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-045-functional` (L2, functional)
- Kỹ thuật: useCase; tiêu chí phủ: Mỗi tiêu chí chấp nhận một coverage item; mục tiêu 100 %
- Lý do: useCase phù hợp functional

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-045-1-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-045-1-functional` | condition | Lần chạy đến hạn của lịch hợp lệ, đủ điều kiện, tạo đúng 1 giao dịch COMPLETED qua lõi chuyển tiền dùng chung. |  | Đếm Transfer theo lần chạy = 1 và trạng thái COMPLETED; run.status = SUCCEEDED; run.transferId = id của Transfer đó. | `TORC-SCHEDULED-TRANSFER-AC-FR-045-1` |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-045-2-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-045-2-functional` | condition | Idempotency-Key của lần chạy đầu bằng mã lịch ghép thời điểm dự kiến. |  | Idempotency-Key ghi nhận trên giao dịch bằng đúng chuỗi ghép mã lịch và thời điểm dự kiến; hai lần gọi cùng cặp này trả về cùng một giao dịch. | `TORC-SCHEDULED-TRANSFER-AC-FR-045-2` |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-045-3-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-045-3-functional` | condition | Các yếu tố số dư, hạn mức, trạng thái tài khoản, Fraud Check được đánh giá theo trạng thái lúc chạy. |  | Quyết định chấp nhận/từ chối khớp với số dư, hạn mức, trạng thái tài khoản và kết quả Fraud Check tại lúc chạy; lời gọi Fraud Check xảy ra trong lúc chạy. | `TORC-SCHEDULED-TRANSFER-AC-FR-045-3` |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-045-4-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-045-4-functional` | condition | Giao dịch từ lịch được cộng vào hạn mức ngày đã dùng và hiện trong lịch sử giao dịch. |  | Hạn mức đã dùng = X + A; lịch sử giao dịch chứa Transfer của lần chạy với số tiền A. | `TORC-SCHEDULED-TRANSFER-AC-FR-045-4` |

### TD-SCHEDULED-TRANSFER-L2-FR-046-useCase

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-046-functional` (L2, functional)
- Kỹ thuật: useCase; tiêu chí phủ: Mỗi tiêu chí chấp nhận một coverage item; mục tiêu 100 %
- Lý do: useCase phù hợp functional

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-046-1-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-046-1-functional` | condition | Phủ xử lý FRAUD_CHECK_UNAVAILABLE: 3 lần thử lại cách nhau 5 phút, thành công thì COMPLETED, hết lượt thì FAILED kèm reasonCode. |  | Số lần thử lại = 3, khoảng cách 5 phút; sau lần lỗi cuối run = FAILED, reasonCode = FRAUD_CHECK_UNAVAILABLE. | `TORC-SCHEDULED-TRANSFER-AC-FR-046-1` |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-046-2-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-046-2-functional` | condition | Phủ xử lý INSUFFICIENT_FUNDS: 2 lần thử lại cách nhau 1 giờ, thành công thì COMPLETED, hết lượt thì FAILED. |  | Số lần thử lại = 2, khoảng cách 1 giờ; sau lần lỗi cuối run = FAILED. | `TORC-SCHEDULED-TRANSFER-AC-FR-046-2` |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-046-3-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-046-3-functional` | condition | Phủ 3 mã lỗi không thử lại (ACCOUNT_INACTIVE, FRAUD_REJECTED, LIMIT_EXCEEDED): FAILED ngay, 0 lần thử lại. |  | Run = FAILED ngay với reasonCode tương ứng mã lỗi; số lần thử lại = 0; không có lần thử nào ở các chu kỳ quét sau. | `TORC-SCHEDULED-TRANSFER-AC-FR-046-3` |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-046-4-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-046-4-functional` | condition | Phủ chuyển lịch ACTIVE sang PAUSED sau 3 run FAILED liên tiếp, và giữ ACTIVE khi chuỗi chưa đủ 3 hoặc bị run thành công ngắt. |  | Lịch = PAUSED ngay sau lần FAILED thứ 3 liên tiếp; lịch vẫn ACTIVE nếu chuỗi FAILED liên tiếp < 3. | `TORC-SCHEDULED-TRANSFER-AC-FR-046-4` |

### TD-SCHEDULED-TRANSFER-L2-FR-047-errorGuessing

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-047-reliability` (L2, reliability)
- Kỹ thuật: errorGuessing; tiêu chí phủ: Mỗi tình huống lỗi đoán trước một coverage item; mục tiêu 100 %
- Lý do: errorGuessing phù hợp idempotency

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-047-idempotency-pd600b3-c` | `TCND-SCHEDULED-TRANSFER-FR-047-idempotency-pd600b3` | condition | Sự kiện kết quả lần chạy giao lại từ queue không tạo SMS thứ hai; sms_log vẫn có 1 bản ghi. |  | Số bản ghi sms_log cho một lần chạy và một loại sự kiện bằng 1 sau khi giao lại. | `TORC-SCHEDULED-TRANSFER-SCN-Notify-pd600b3` |

### TD-SCHEDULED-TRANSFER-L2-FR-047-useCase

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-047-functional` (L2, functional)
- Kỹ thuật: useCase; tiêu chí phủ: Mỗi tiêu chí chấp nhận một coverage item; mục tiêu 100 %
- Lý do: useCase phù hợp functional

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-047-1-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-047-1-functional` | condition | Lần chạy thành công phát đúng 1 ScheduledRunCompleted vào outbox với transferId khớp giao dịch. |  | Đúng 1 sự kiện ScheduledRunCompleted trong outbox cho mỗi lần chạy thành công; payload.transferId khác rỗng và bằng transferId của giao dịch. | `TORC-SCHEDULED-TRANSFER-AC-FR-047-1` |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-047-2-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-047-2-functional` | condition | Lần chạy FAILED phát đúng 1 ScheduledRunFailed vào outbox với reasonCode khớp lý do đã lưu. |  | Đúng 1 sự kiện ScheduledRunFailed trong outbox cho mỗi lần chạy FAILED; payload.reasonCode khác rỗng và bằng mã lý do lưu trên lần chạy. | `TORC-SCHEDULED-TRANSFER-AC-FR-047-2` |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-047-3-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-047-3-functional` | condition | SMS kết quả được ghi vào sms_log cho người gửi sau khi consumer nhận sự kiện; lỗi SMS Gateway giữ nguyên trạng thái lần chạy. |  | Trường hợp bình thường: có bản ghi SMS cho người gửi với nội dung từ mẫu kết quả lần chạy. Trường hợp lỗi SMS: trạng thái lần chạy trước và sau khi gửi lỗi giống nhau. | `TORC-SCHEDULED-TRANSFER-AC-FR-047-3` |

### TD-SCHEDULED-TRANSFER-L2-FR-048-errorGuessing

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-048-functional` (L2, functional)
- Kỹ thuật: errorGuessing; tiêu chí phủ: Mỗi tình huống lỗi đoán trước một coverage item; mục tiêu 100 %
- Lý do: errorGuessing phù hợp invariant

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-048-invariant-pdabb36-c` | `TCND-SCHEDULED-TRANSFER-FR-048-invariant-pdabb36` | condition | Bản ghi AuditLog đã ghi không bị sửa hay xóa sau các thao tác lịch và lần chạy tiếp theo. |  | Các bản ghi audit cũ không đổi (mọi trường) sau các thao tác tiếp theo; số bản ghi chỉ tăng. | `TORC-SCHEDULED-TRANSFER-DE-AuditLog-pdabb36` |

### TD-SCHEDULED-TRANSFER-L2-FR-048-faultInjection

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-048-maintainability` (L2, maintainability)
- Kỹ thuật: faultInjection; tiêu chí phủ: Mỗi mô hình lỗi được tiêm ít nhất một lần; mục tiêu 100 %
- Lý do: faultInjection phù hợp observability

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-048-observability-p477475-c` | `TCND-SCHEDULED-TRANSFER-FR-048-observability-p477475` | condition | correlationId của mỗi lần chạy nhất quán giữa audit, giao dịch và Fraud Check; ngưỡng 100%. |  | Tỉ lệ lần chạy có correlationId riêng trong audit khớp với giao dịch và lời gọi Fraud Check là 100%. | `TORC-SCHEDULED-TRANSFER-NFR-014-p477475` |

### TD-SCHEDULED-TRANSFER-L2-FR-048-stateTransition

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-048-functional` (L2, functional)
- Kỹ thuật: stateTransition; tiêu chí phủ: Mỗi chuyển trạng thái và mỗi guard bị vi phạm; mục tiêu 100 %
- Lý do: stateTransition phù hợp guardViolation

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-048-guardViolation-pf87b1a-c` | `TCND-SCHEDULED-TRANSFER-FR-048-guardViolation-pf87b1a` | condition | Yêu cầu bị từ chối (401, 403, 404, 409, 400, 422) không ghi audit sự kiện lịch thành công. |  | Phản hồi 403 FORBIDDEN và 409 SCHEDULE_STATE_INVALID; số bản ghi audit sự kiện lịch không tăng. | `TORC-SCHEDULED-TRANSFER-RSP-Patch-403-pf87b1a` |

### TD-SCHEDULED-TRANSFER-L2-FR-048-useCase

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-048-functional` (L2, functional)
- Kỹ thuật: useCase; tiêu chí phủ: Mỗi tiêu chí chấp nhận một coverage item; mục tiêu 100 %
- Lý do: useCase phù hợp functional

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-048-1-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-048-1-functional` | condition | Bốn thao tác thành công (tạo, hủy, tạm dừng, tiếp tục) mỗi thao tác ghi một bản ghi audit đủ 5 thuộc tính. |  | Sau 4 thao tác thành công, AuditLog có đúng 4 bản ghi mới cho mã lịch đó, mỗi bản ghi đủ correlationId, thời điểm, tác nhân = mã tài khoản (không phải SCHEDULER), loại sự kiện phân biệt, mã lịch. | `TORC-SCHEDULED-TRANSFER-AC-FR-048-1` |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-048-2-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-048-2-functional` | condition | Mọi kết quả lần chạy do scheduler xử lý (thành công, thất bại, FAILED, MISSED) đều có bản ghi audit actor = SCHEDULER. |  | Với mỗi lần chạy đã có kết quả, AuditLog có bản ghi actor = SCHEDULER, mã lịch = lịch của lần chạy; số bản ghi khớp số lần chạy đã xử lý. | `TORC-SCHEDULED-TRANSFER-AC-FR-048-2` |

### TD-SCHEDULED-TRANSFER-L2-FR-049-faultInjection

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-049-reliability` (L2, reliability)
- Kỹ thuật: faultInjection; tiêu chí phủ: Mỗi mô hình lỗi được tiêm ít nhất một lần; mục tiêu 100 %
- Lý do: faultInjection phù hợp concurrency

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-FR-049-concurrency-pe3c163-c` | `TCND-SCHEDULED-TRANSFER-FR-049-concurrency-pe3c163` | condition | Hai Scheduler quét đồng thời một lần chạy quá hạn trong 24 giờ; chỉ một bên chiếm thành công. |  | 0 giao dịch trùng; đúng 1 giao dịch cho lần chạy. | `TORC-SCHEDULED-TRANSFER-NFR-011-pe3c163` |

### TD-SCHEDULED-TRANSFER-L2-FR-049-useCase

- TR: `TR-SCHEDULED-TRANSFER-L2-FR-049-functional` (L2, functional)
- Kỹ thuật: useCase; tiêu chí phủ: Mỗi tiêu chí chấp nhận một coverage item; mục tiêu 100 %
- Lý do: useCase phù hợp functional

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-049-1-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-049-1-functional` | condition | Lần chạy quá hạn không quá 24 giờ sau khi khởi động lại được chạy bù đúng một lần qua lõi chuyển tiền. |  | Đúng 1 giao dịch cho lần chạy bù; lần chạy không còn ở trạng thái chờ; quét thêm không tạo giao dịch mới. | `TORC-SCHEDULED-TRANSFER-AC-FR-049-1` |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-049-2-functional-c` | `TCND-SCHEDULED-TRANSFER-AC-FR-049-2-functional` | condition | Lần chạy quá hạn quá 24 giờ được chuyển MISSED, không chạy bù. |  | Trạng thái lần chạy = MISSED; có 1 sự kiện ScheduledRunFailed reasonCode MISSED; 0 giao dịch cho lần chạy này; số dư tài khoản nguồn và đích không đổi. | `TORC-SCHEDULED-TRANSFER-AC-FR-049-2` |

### TD-SCHEDULED-TRANSFER-L2-NFR-010-boundaryValue

- TR: `TR-SCHEDULED-TRANSFER-L2-NFR-010-performance` (L2, performance)
- Kỹ thuật: boundaryValue; tiêu chí phủ: Mỗi biên hai giá trị (trong và ngoài biên); mục tiêu 100 %
- Lý do: boundaryValue phù hợp timing

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-NFR-010-1-timing-c` | `TCND-SCHEDULED-TRANSFER-AC-NFR-010-1-timing` | condition | Lần chạy đến hạn phân bố đều trong chu kỳ quét 30 s trên đồng hồ giả; tỷ lệ bắt đầu trong 60 s phải >= 99%. |  | Số lần chạy có độ trễ bắt đầu <= 60 s chia tổng số lần chạy >= 0,99. | `TORC-SCHEDULED-TRANSFER-AC-NFR-010-1` |
| `TCOV-SCHEDULED-TRANSFER-L2-NFR-010-timing-p9746f5-c` | `TCND-SCHEDULED-TRANSFER-NFR-010-timing-p9746f5` | condition | Quá tải 40 lần chạy đồng thời, tối đa 20 lần mỗi chu kỳ; phần vượt bị hoãn sang chu kỳ sau nhưng độ trễ vẫn trong 60 s (tỷ lệ >= 99%). |  | Mỗi chu kỳ chọn tối đa 20 lần chạy; tỷ lệ lần chạy có độ trễ <= 60 s vẫn >= 99% khi quá tải 40 lần chạy đồng thời. | `TORC-SCHEDULED-TRANSFER-NFR-010-p9746f5` |
| `TCOV-SCHEDULED-TRANSFER-L2-NFR-010-timing-target-c` | `TCND-SCHEDULED-TRANSFER-NFR-010-timing-target` | condition | Chỉ tiêu 60 s: tỷ lệ lần chạy bắt đầu trong 60 s kể từ thời điểm dự kiến >= 99% trong cửa sổ đo. |  | Ít nhất 99% lần chạy có (thời điểm bắt đầu - thời điểm dự kiến) <= 60 s. | `TORC-SCHEDULED-TRANSFER-NFR-010` |

### TD-SCHEDULED-TRANSFER-L2-NFR-011-faultInjection

- TR: `TR-SCHEDULED-TRANSFER-L2-NFR-011-reliability` (L2, reliability)
- Kỹ thuật: faultInjection; tiêu chí phủ: Mỗi mô hình lỗi được tiêm ít nhất một lần; mục tiêu 100 %
- Lý do: faultInjection phù hợp concurrency

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-NFR-011-1-concurrency-c` | `TCND-SCHEDULED-TRANSFER-AC-NFR-011-1-concurrency` | condition | Tình huống hai scheduler cùng quét một lần chạy đến hạn trong scheduled_runs và cùng thực hiện UPDATE có điều kiện để chiếm lần chạy. |  | Đúng 1 giao dịch ứng với Idempotency-Key (mã lịch + thời điểm dự kiến); chỉ 1 scheduler chiếm được lần chạy; không có giao dịch thứ hai. | `TORC-SCHEDULED-TRANSFER-AC-NFR-011-1` |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-NFR-011-2-concurrency-c` | `TCND-SCHEDULED-TRANSFER-AC-NFR-011-2-concurrency` | condition | Tình huống scheduler dừng đột ngột sau khi chiếm lần chạy nhưng trước khi lõi chuyển tiền hoàn tất, rồi khởi động lại và quét lại lần chạy đó. |  | Số giao dịch ứng với Idempotency-Key của lần chạy <= 1 sau khi khởi động lại và quét lại; không có giao dịch thứ hai. | `TORC-SCHEDULED-TRANSFER-AC-NFR-011-2` |
| `TCOV-SCHEDULED-TRANSFER-L2-NFR-011-concurrency-target-c` | `TCND-SCHEDULED-TRANSFER-NFR-011-concurrency-target` | condition | Mọi tình huống nhiều scheduler cùng quét một lần chạy đến hạn; số giao dịch trùng phải bằng 0. |  | Số giao dịch trùng cho mỗi lần chạy = 0. | `TORC-SCHEDULED-TRANSFER-NFR-011` |

### TD-SCHEDULED-TRANSFER-L2-NFR-012-boundaryValue

- TR: `TR-SCHEDULED-TRANSFER-L2-NFR-012-performance` (L2, performance)
- Kỹ thuật: boundaryValue; tiêu chí phủ: Mỗi biên hai giá trị (trong và ngoài biên); mục tiêu 100 %
- Lý do: boundaryValue phù hợp timing

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-NFR-012-timing-pe0788e-c` | `TCND-SCHEDULED-TRANSFER-NFR-012-timing-pe0788e` | condition | Biên 24 giờ: kiểm lần quá hạn đúng 24 giờ (chạy bù) và lần vượt ngưỡng 24 giờ (MISSED, không giao dịch). |  | ≤ 24 giờ: chạy bù; > 24 giờ: MISSED, không giao dịch. | `TORC-SCHEDULED-TRANSFER-FR-049-pe0788e` |

### TD-SCHEDULED-TRANSFER-L2-NFR-012-faultInjection

- TR: `TR-SCHEDULED-TRANSFER-L2-NFR-012-reliability` (L2, reliability)
- Kỹ thuật: faultInjection; tiêu chí phủ: Mỗi mô hình lỗi được tiêm ít nhất một lần; mục tiêu 100 %
- Lý do: faultInjection phù hợp concurrency, resilience

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-NFR-012-1-resilience-c` | `TCND-SCHEDULED-TRANSFER-AC-NFR-012-1-resilience` | condition | Xử lý hết lần quá hạn ≤ 24 giờ trong 5 chu kỳ quét đầu sau khi Scheduler khởi động lại. |  | Sau chu kỳ quét thứ 5, số lần chạy quá hạn ≤ 24 giờ còn chưa xử lý bằng 0; mỗi chu kỳ xử lý tối đa 20 lần chạy. | `TORC-SCHEDULED-TRANSFER-AC-NFR-012-1` |
| `TCOV-SCHEDULED-TRANSFER-L2-NFR-012-concurrency-p92271d-c` | `TCND-SCHEDULED-TRANSFER-NFR-012-concurrency-p92271d` | condition | Hai scheduler cùng quét khi chạy bù: không có giao dịch trùng cho cùng một lần quá hạn. |  | 0 giao dịch trùng cho mỗi lần chạy. | `TORC-SCHEDULED-TRANSFER-NFR-011-p92271d` |
| `TCOV-SCHEDULED-TRANSFER-L2-NFR-012-resilience-target-c` | `TCND-SCHEDULED-TRANSFER-NFR-012-resilience-target` | condition | Chỉ tiêu phục hồi: tải 100 lần quá hạn được xử lý trong ≤ 5 chu kỳ quét (chu kỳ 30 giây). |  | Số chu kỳ quét cần để xử lý hết lần quá hạn ≤ 24 giờ ≤ 5; lần quá hạn > 24 giờ có trạng thái MISSED. | `TORC-SCHEDULED-TRANSFER-NFR-012` |

### TD-SCHEDULED-TRANSFER-L2-NFR-013-faultInjection

- TR: `TR-SCHEDULED-TRANSFER-L2-NFR-013-reliability` (L2, reliability)
- Kỹ thuật: faultInjection; tiêu chí phủ: Mỗi mô hình lỗi được tiêm ít nhất một lần; mục tiêu 100 %
- Lý do: faultInjection phù hợp concurrency

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-NFR-013-concurrency-p4dc0d8-c` | `TCND-SCHEDULED-TRANSFER-NFR-013-concurrency-p4dc0d8` | condition | 100 lần chạy đến hạn đồng thời: không có giao dịch trùng cho cùng một lần chạy. | 0 | 0 giao dịch trùng cho một lần chạy. | `TORC-SCHEDULED-TRANSFER-NFR-011-p4dc0d8` |

### TD-SCHEDULED-TRANSFER-L2-NFR-013-loadModel

- TR: `TR-SCHEDULED-TRANSFER-L2-NFR-013-performance` (L2, performance)
- Kỹ thuật: loadModel; tiêu chí phủ: Mỗi chỉ tiêu tải đo theo workload khai báo; mục tiêu 100 %
- Lý do: loadModel phù hợp performance

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-NFR-013-1-performance-c` | `TCND-SCHEDULED-TRANSFER-AC-NFR-013-1-performance` | condition | Đợt 100 lần chạy đến hạn đồng thời: p95 thời gian chuyển tiền qua API không vượt ngưỡng 500 ms. | 500 | p95 thời gian chuyển tiền qua API <= 500 ms khi 100 lần chạy đến hạn cùng lúc. | `TORC-SCHEDULED-TRANSFER-AC-NFR-013-1` |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-NFR-013-2-performance-c` | `TCND-SCHEDULED-TRANSFER-AC-NFR-013-2-performance` | condition | Mỗi chu kỳ quét xử lý tối đa 20 lần chạy dù có hơn 20 lần chạy đến hạn. | 20 | Số lần chạy được chọn và chiếm trong một chu kỳ quét <= 20. | `TORC-SCHEDULED-TRANSFER-AC-NFR-013-2` |
| `TCOV-SCHEDULED-TRANSFER-L2-NFR-013-performance-pd5ac38-c` | `TCND-SCHEDULED-TRANSFER-NFR-013-performance-pd5ac38` | condition | Rate limit scheduler giữ ở mức <= 10 rps khi 100 lần chạy đến hạn cùng lúc. | 10 | Tốc độ yêu cầu từ scheduler <= 10 rps. | `TORC-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-pd5ac38` |
| `TCOV-SCHEDULED-TRANSFER-L2-NFR-013-performance-target-c` | `TCND-SCHEDULED-TRANSFER-NFR-013-performance-target` | condition | Chỉ tiêu NFR-013: p95 chuyển tiền qua API <= 500 ms với tải 100 lần chạy đồng thời. | 500 | p95 thời gian chuyển tiền qua API <= 500 ms (chỉ tiêu 500 ms). | `TORC-SCHEDULED-TRANSFER-NFR-013` |

### TD-SCHEDULED-TRANSFER-L2-NFR-014-faultInjection

- TR: `TR-SCHEDULED-TRANSFER-L2-NFR-014-maintainability` (L2, maintainability)
- Kỹ thuật: faultInjection; tiêu chí phủ: Mỗi mô hình lỗi được tiêm ít nhất một lần; mục tiêu 100 %
- Lý do: faultInjection phù hợp observability

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-NFR-014-1-observability-c` | `TCND-SCHEDULED-TRANSFER-AC-NFR-014-1-observability` | condition | Phủ tính phân biệt của correlationId giữa các lần chạy được xử lý, kể cả trong cùng chu kỳ quét (tối đa 20). |  | Số correlationId phân biệt = số lần chạy; mọi lần chạy đều có correlationId khác rỗng. | `TORC-SCHEDULED-TRANSFER-AC-NFR-014-1` |
| `TCOV-SCHEDULED-TRANSFER-L2-AC-NFR-014-2-observability-c` | `TCND-SCHEDULED-TRANSFER-AC-NFR-014-2-observability` | condition | Phủ việc correlationId của một lần chạy xuất hiện đúng ở giao dịch, audit, lời gọi Fraud Check (INT-01) và sự kiện outbox. |  | Giá trị correlationId trong giao dịch, audit, lời gọi Fraud Check và sự kiện bằng đúng giá trị correlationId của lần chạy. | `TORC-SCHEDULED-TRANSFER-AC-NFR-014-2` |
| `TCOV-SCHEDULED-TRANSFER-L2-NFR-014-observability-p8ac829-c` | `TCND-SCHEDULED-TRANSFER-NFR-014-observability-p8ac829` | condition | Phủ việc correlationId vẫn nhất quán và đầy đủ ở giao dịch, audit, lời gọi Fraud Check khi lần chạy được thử lại. |  | Mọi bản ghi và lời gọi Fraud Check của lần chạy, kể cả khi thử lại, đều có correlationId khác rỗng. | `TORC-SCHEDULED-TRANSFER-NFR-014-p8ac829` |
| `TCOV-SCHEDULED-TRANSFER-L2-NFR-014-observability-target-c` | `TCND-SCHEDULED-TRANSFER-NFR-014-observability-target` | condition | Phủ chỉ tiêu truy vết đầy đủ 100% trên toàn bộ tập lần chạy kiểm thử (thành công, thử lại, FAILED). | 100 | Tỷ lệ lần chạy truy vết đầy đủ = 100% (targetValue 100, đơn vị %). | `TORC-SCHEDULED-TRANSFER-NFR-014` |

### TD-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-faultInjection-maintainability

- TR: `TR-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-maintainability` (L2, maintainability)
- Kỹ thuật: faultInjection; tiêu chí phủ: Mỗi mô hình lỗi được tiêm ít nhất một lần; mục tiêu 100 %
- Lý do: faultInjection phù hợp observability

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-observability-p12d52f-c` | `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-observability-p12d52f` | condition | correlationId riêng mỗi lần chạy và có mặt ở giao dịch, audit, Fraud Check, sự kiện. |  | Không có hai lần chạy chung correlationId; correlationId xuất hiện ở cả 4 nơi. | `TORC-SCHEDULED-TRANSFER-AC-NFR-014-2-p12d52f` |

### TD-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-faultInjection-reliability

- TR: `TR-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-reliability` (L2, reliability)
- Kỹ thuật: faultInjection; tiêu chí phủ: Mỗi mô hình lỗi được tiêm ít nhất một lần; mục tiêu 100 %
- Lý do: faultInjection phù hợp atomicity, concurrency, errorHandling

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-atomicity-p12901f-c` | `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-atomicity-p12901f` | condition | Khởi động lại tiến trình giữa chiếm lần chạy và chuyển tiền. |  | Tối đa một giao dịch cho lần chạy. | `TORC-SCHEDULED-TRANSFER-AC-NFR-011-2-p12901f` |
| `TCOV-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-concurrency-p775a37-c` | `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-concurrency-p775a37` | condition | Hai scheduler cùng quét và chiếm một lần chạy đến hạn. |  | Số giao dịch trùng cho một lần chạy = 0; đúng một UPDATE có điều kiện thành công. | `TORC-SCHEDULED-TRANSFER-AC-NFR-011-1-p775a37` |
| `TCOV-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-errorHandling-p00fe49-c` | `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-errorHandling-p00fe49` | condition | Lỗi gửi SMS kết quả của lần chạy. |  | Trạng thái run giữ nguyên (SUCCEEDED hoặc FAILED) sau lỗi SMS. | `TORC-SCHEDULED-TRANSFER-AC-FR-047-3-p00fe49` |

### TD-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-loadModel

- TR: `TR-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-performance` (L2, performance)
- Kỹ thuật: loadModel; tiêu chí phủ: Mỗi chỉ tiêu tải đo theo workload khai báo; mục tiêu 100 %
- Lý do: loadModel phù hợp performance

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-performance-pa101d9-c` | `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-performance-pa101d9` | condition | Tải 100 lần chạy đến hạn cùng lúc: tối đa 20 lần mỗi chu kỳ và p95 ≤ 500 ms. |  | p95 ≤ 500 ms; số lần chạy mỗi chu kỳ ≤ 20. | `TORC-SCHEDULED-TRANSFER-AC-NFR-013-1-pa101d9` |

### TD-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-stateTransition

- TR: `TR-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-functional` (L2, functional)
- Kỹ thuật: stateTransition; tiêu chí phủ: Mỗi chuyển trạng thái và mỗi guard bị vi phạm; mục tiêu 100 %
- Lý do: stateTransition phù hợp stateTransition

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-stateTransition-pd98b3f-c` | `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-stateTransition-pd98b3f` | condition | Chạy bù khi quá hạn ≤ 24 giờ, MISSED khi > 24 giờ, gồm biên đúng 24 giờ. |  | ≤ 24 giờ: 1 giao dịch, xử lý trong 5 chu kỳ quét đầu; > 24 giờ: MISSED, 0 giao dịch, ScheduledRunFailed reasonCode MISSED. | `TORC-SCHEDULED-TRANSFER-AC-FR-049-2-pd98b3f` |
| `TCOV-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-stateTransition-pfec2a2-c` | `TCND-SCHEDULED-TRANSFER-SCN-ScheduledRun-stateTransition-pfec2a2` | condition | Lịch sang PAUSED sau 3 lần FAILED liên tiếp; lỗi không thử lại làm run FAILED ngay. |  | Lịch ACTIVE → PAUSED sau 3 run FAILED liên tiếp; run SUCCEEDED xen giữa thì đếm lại từ đầu (chờ xác nhận). | `TORC-SCHEDULED-TRANSFER-AC-FR-046-4-pfec2a2` |

### TD-SCHEDULED-TRANSFER-L2-SM-Schedule-faultInjection

- TR: `TR-SCHEDULED-TRANSFER-L2-SM-Schedule-reliability` (L2, reliability)
- Kỹ thuật: faultInjection; tiêu chí phủ: Mỗi mô hình lỗi được tiêm ít nhất một lần; mục tiêu 100 %
- Lý do: faultInjection phù hợp concurrency

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-concurrency-p34e996-c` | `TCND-SCHEDULED-TRANSFER-SM-Schedule-concurrency-p34e996` | condition | PATCH PAUSED hoặc DELETE đồng thời với scheduler chiếm lần chạy đến hạn bằng UPDATE có điều kiện. |  | Không có lần chạy bị thực hiện 2 lần; không có lần RUNNING của lịch đã CANCELLED; trạng thái cuối nhất quán với một thứ tự tuần tự hợp lệ. | `TORC-SCHEDULED-TRANSFER-MOD-Scheduler-p34e996` |

### TD-SCHEDULED-TRANSFER-L2-SM-Schedule-stateTransition

- TR: `TR-SCHEDULED-TRANSFER-L2-SM-Schedule-functional` (L2, functional)
- Kỹ thuật: stateTransition; tiêu chí phủ: Mỗi chuyển trạng thái và mỗi guard bị vi phạm; mục tiêu 100 %
- Lý do: stateTransition phù hợp guardViolation, stateTransition

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-TRN-Schedule-1-guardViolation-guard-c` | `TCND-SCHEDULED-TRANSFER-TRN-Schedule-1-guardViolation-guard` | condition | Tạm dừng lịch ACTIVE không định kỳ bị từ chối, giữ ACTIVE. |  | Trạng thái vẫn ACTIVE, không có audit SCHEDULE_PAUSED mới. Mã lỗi chưa xác định trong nguồn. | `TORC-SCHEDULED-TRANSFER-TRN-Schedule-1-guard` |
| `TCOV-SCHEDULED-TRANSFER-L2-TRN-Schedule-1-stateTransition-c` | `TCND-SCHEDULED-TRANSFER-TRN-Schedule-1-stateTransition` | condition | Tạm dừng lịch định kỳ ACTIVE → PAUSED, ghi audit. |  | ACTIVE → PAUSED; có đúng 1 audit SCHEDULE_PAUSED; lần quét 30 giây kế tiếp không chiếm lần chạy nào của lịch này. | `TORC-SCHEDULED-TRANSFER-TRN-Schedule-1` |
| `TCOV-SCHEDULED-TRANSFER-L2-TRN-Schedule-2-guardViolation-guard-c` | `TCND-SCHEDULED-TRANSFER-TRN-Schedule-2-guardViolation-guard` | condition | Tiếp tục lịch PAUSED hết lần chạy theo BR-11 không sang ACTIVE theo TRN-Schedule-2. |  | Không có trạng thái ACTIVE, không có audit SCHEDULE_RESUMED, không có lần chạy mới. Kết quả cuối xem TRN-Schedule-7. | `TORC-SCHEDULED-TRANSFER-TRN-Schedule-2-guard` |
| `TCOV-SCHEDULED-TRANSFER-L2-TRN-Schedule-2-stateTransition-c` | `TCND-SCHEDULED-TRANSFER-TRN-Schedule-2-stateTransition` | condition | Tiếp tục lịch PAUSED còn lần chạy → ACTIVE. |  | PAUSED → ACTIVE; lần bỏ lỡ = SKIPPED; có 1 lần chạy kế tiếp mới; consecutive_failures = 0; audit SCHEDULE_RESUMED. | `TORC-SCHEDULED-TRANSFER-TRN-Schedule-2` |
| `TCOV-SCHEDULED-TRANSFER-L2-TRN-Schedule-3-guardViolation-guard-c` | `TCND-SCHEDULED-TRANSFER-TRN-Schedule-3-guardViolation-guard` | condition | FAILED thứ 3 liên tiếp của lịch không định kỳ không tự tạm dừng. |  | Trạng thái vẫn ACTIVE, không có audit SCHEDULE_AUTO_PAUSED. | `TORC-SCHEDULED-TRANSFER-TRN-Schedule-3-guard` |
| `TCOV-SCHEDULED-TRANSFER-L2-TRN-Schedule-3-stateTransition-c` | `TCND-SCHEDULED-TRANSFER-TRN-Schedule-3-stateTransition` | condition | FAILED thứ 3 liên tiếp của lịch định kỳ → tự PAUSED. |  | ACTIVE → PAUSED; audit SCHEDULE_AUTO_PAUSED có actor=SCHEDULER; không có lần chạy PENDING mới được tạo. | `TORC-SCHEDULED-TRANSFER-TRN-Schedule-3` |
| `TCOV-SCHEDULED-TRANSFER-L2-TRN-Schedule-4-guardViolation-guard-c` | `TCND-SCHEDULED-TRANSFER-TRN-Schedule-4-guardViolation-guard` | condition | Lần chạy cuối kết thúc nhưng chưa đủ điều kiện hoàn tất, giữ ACTIVE. |  | Trạng thái vẫn ACTIVE, không chuyển COMPLETED. | `TORC-SCHEDULED-TRANSFER-TRN-Schedule-4-guard` |
| `TCOV-SCHEDULED-TRANSFER-L2-TRN-Schedule-4-stateTransition-c` | `TCND-SCHEDULED-TRANSFER-TRN-Schedule-4-stateTransition` | condition | Đủ điều kiện hoàn tất (ONCE/maxRuns/endDate) → COMPLETED. |  | ACTIVE → COMPLETED; không có lần chạy mới nào được tạo. | `TORC-SCHEDULED-TRANSFER-TRN-Schedule-4` |
| `TCOV-SCHEDULED-TRANSFER-L2-TRN-Schedule-5-guardViolation-guard-c` | `TCND-SCHEDULED-TRANSFER-TRN-Schedule-5-guardViolation-guard` | condition | DELETE lịch ACTIVE có lần RUNNING bị từ chối. |  | Trạng thái vẫn ACTIVE, lần PENDING giữ nguyên, không có audit SCHEDULE_CANCELLED. Mã lỗi chưa xác định trong nguồn. | `TORC-SCHEDULED-TRANSFER-TRN-Schedule-5-guard` |
| `TCOV-SCHEDULED-TRANSFER-L2-TRN-Schedule-5-stateTransition-c` | `TCND-SCHEDULED-TRANSFER-TRN-Schedule-5-stateTransition` | condition | DELETE lịch ACTIVE không có lần RUNNING → CANCELLED. |  | ACTIVE → CANCELLED; không còn lần PENDING; có 1 audit SCHEDULE_CANCELLED. | `TORC-SCHEDULED-TRANSFER-TRN-Schedule-5` |
| `TCOV-SCHEDULED-TRANSFER-L2-TRN-Schedule-6-guardViolation-guard-c` | `TCND-SCHEDULED-TRANSFER-TRN-Schedule-6-guardViolation-guard` | condition | DELETE lịch PAUSED có lần RUNNING bị từ chối. |  | Trạng thái vẫn PAUSED, không có audit SCHEDULE_CANCELLED. Mã lỗi chưa xác định trong nguồn. | `TORC-SCHEDULED-TRANSFER-TRN-Schedule-6-guard` |
| `TCOV-SCHEDULED-TRANSFER-L2-TRN-Schedule-6-stateTransition-c` | `TCND-SCHEDULED-TRANSFER-TRN-Schedule-6-stateTransition` | condition | DELETE lịch PAUSED không có lần RUNNING → CANCELLED. |  | PAUSED → CANCELLED; không còn lần PENDING; có 1 audit SCHEDULE_CANCELLED. | `TORC-SCHEDULED-TRANSFER-TRN-Schedule-6` |
| `TCOV-SCHEDULED-TRANSFER-L2-TRN-Schedule-7-guardViolation-guard-c` | `TCND-SCHEDULED-TRANSFER-TRN-Schedule-7-guardViolation-guard` | condition | Tiếp tục lịch PAUSED còn lần chạy sau SKIPPED không sang COMPLETED. |  | Không chuyển COMPLETED. Trạng thái cuối do TRN-Schedule-2 quyết định; nguồn mẫu 'giữ PAUSED' mâu thuẫn, cần làm rõ. | `TORC-SCHEDULED-TRANSFER-TRN-Schedule-7-guard` |
| `TCOV-SCHEDULED-TRANSFER-L2-TRN-Schedule-7-stateTransition-c` | `TCND-SCHEDULED-TRANSFER-TRN-Schedule-7-stateTransition` | condition | Tiếp tục lịch PAUSED hết lần chạy sau SKIPPED → COMPLETED. |  | PAUSED → COMPLETED; các lần còn lại = SKIPPED; không có lần chạy mới. | `TORC-SCHEDULED-TRANSFER-TRN-Schedule-7` |

### TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-errorGuessing

- TR: `TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-reliability` (L2, reliability)
- Kỹ thuật: errorGuessing; tiêu chí phủ: Mỗi tình huống lỗi đoán trước một coverage item; mục tiêu 100 %
- Lý do: errorGuessing phù hợp idempotency

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-idempotency-p456351-c` | `TCND-SCHEDULED-TRANSFER-SM-ScheduledRun-idempotency-p456351` | condition | Sau khi phục hồi lần chạy kẹt, lần chiếm lại dùng cùng khóa idempotency nên không tạo giao dịch trùng nếu lõi đã xử lý lần trước. |  | Số giao dịch của lần chạy = 1; transferId trùng giao dịch ban đầu; status=SUCCEEDED. | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-8-p456351` |

### TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-faultInjection

- TR: `TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-reliability` (L2, reliability)
- Kỹ thuật: faultInjection; tiêu chí phủ: Mỗi mô hình lỗi được tiêm ít nhất một lần; mục tiêu 100 %
- Lý do: faultInjection phù hợp concurrency

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-concurrency-p4b41b3-c` | `TCND-SCHEDULED-TRANSFER-SM-ScheduledRun-concurrency-p4b41b3` | condition | Hai scheduler cùng chiếm một lần chạy PENDING đến hạn: chỉ một UPDATE có điều kiện trả rowcount=1. |  | Đúng 1 bên rowcount=1; attempts = cũ + 1; đúng 1 lời gọi lõi. | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-1-p4b41b3` |

### TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-loadModel

- TR: `TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-performance` (L2, performance)
- Kỹ thuật: loadModel; tiêu chí phủ: Mỗi chỉ tiêu tải đo theo workload khai báo; mục tiêu 100 %
- Lý do: loadModel phù hợp performance

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-performance-pa68a97-c` | `TCND-SCHEDULED-TRANSFER-SM-ScheduledRun-performance-pa68a97` | condition | Mỗi chu kỳ quét 30 giây chỉ xử lý tối đa 20 lần chạy; lần chạy đến hạn vượt số này để lại PENDING cho chu kỳ sau. |  | Số lần chạy được chiếm trong một chu kỳ ≤ 20. | `TORC-SCHEDULED-TRANSFER-MOD-Scheduler-pa68a97` |

### TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-stateTransition

- TR: `TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-functional` (L2, functional)
- Kỹ thuật: stateTransition; tiêu chí phủ: Mỗi chuyển trạng thái và mỗi guard bị vi phạm; mục tiêu 100 %
- Lý do: stateTransition phù hợp guardViolation, stateTransition

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L2-TRN-ScheduledRun-1-guardViolation-guard-c` | `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-1-guardViolation-guard` | condition | Lần chạy PENDING không được chiếm khi vi phạm guard: chưa đến hạn (scheduled_at/next_attempt_at > now), lịch không ACTIVE, hoặc UPDATE có điều kiện trả rowcount=0. |  | status=PENDING, attempts không đổi, không có lời gọi lõi chuyển tiền. | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-1-guard` |
| `TCOV-SCHEDULED-TRANSFER-L2-TRN-ScheduledRun-1-stateTransition-c` | `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-1-stateTransition` | condition | Lần chạy PENDING đã đến hạn, lịch ACTIVE, UPDATE có điều kiện rowcount=1 thì chuyển sang RUNNING. |  | PENDING → RUNNING; attempts = giá trị cũ + 1; correlationId khác rỗng; 1 lời gọi lõi chuyển tiền. | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-1` |
| `TCOV-SCHEDULED-TRANSFER-L2-TRN-ScheduledRun-2-stateTransition-c` | `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-2-stateTransition` | condition | Khi lõi chuyển tiền trả giao dịch COMPLETED (hoặc phát lại giao dịch đã hoàn tất), lần chạy RUNNING chuyển sang SUCCEEDED. |  | RUNNING → SUCCEEDED; transferId đã lưu; consecutive_failures=0; 1 audit; 1 sự kiện ScheduledRunCompleted; lần kế tiếp được sinh hoặc lịch COMPLETED. | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-2` |
| `TCOV-SCHEDULED-TRANSFER-L2-TRN-ScheduledRun-3-guardViolation-guard-c` | `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-3-guardViolation-guard` | condition | Lõi trả lỗi tạm thời/thiếu số dư nhưng đã hết lượt thử lại (FRAUD_CHECK_UNAVAILABLE quá 3 lần, INSUFFICIENT_FUNDS quá 2 lần) thì không quay về PENDING. |  | Không có chuyển RUNNING → PENDING; next_attempt_at không đổi; kết thúc ở FAILED theo TRN-ScheduledRun-4. | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-3-guard` |
| `TCOV-SCHEDULED-TRANSFER-L2-TRN-ScheduledRun-3-stateTransition-c` | `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-3-stateTransition` | condition | Lõi trả lỗi tạm thời hoặc thiếu số dư khi còn lượt thử lại thì lần chạy RUNNING về PENDING để thử lại. |  | RUNNING → PENDING; next_attempt_at = now + 5 phút hoặc now + 1 giờ. | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-3` |
| `TCOV-SCHEDULED-TRANSFER-L2-TRN-ScheduledRun-4-guardViolation-guard-c` | `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-4-guardViolation-guard` | condition | Lõi trả lỗi nhưng lỗi không vĩnh viễn và còn lượt thử lại thì lần chạy không sang FAILED. |  | status không phải FAILED; consecutive_failures không đổi; không có sự kiện ScheduledRunFailed. | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-4-guard` |
| `TCOV-SCHEDULED-TRANSFER-L2-TRN-ScheduledRun-4-stateTransition-c` | `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-4-stateTransition` | condition | Lõi trả lỗi vĩnh viễn hoặc đã hết lượt thử lại thì lần chạy RUNNING sang FAILED. |  | RUNNING → FAILED; reasonCode = mã lỗi từ lõi; consecutive_failures = cũ + 1; 1 audit; 1 sự kiện ScheduledRunFailed. | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-4` |
| `TCOV-SCHEDULED-TRANSFER-L2-TRN-ScheduledRun-5-guardViolation-guard-c` | `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-5-guardViolation-guard` | condition | Khi tiếp tục lịch PAUSED, lần chạy PENDING chưa quá hạn trong thời gian tạm dừng thì không bị SKIPPED. |  | status=PENDING; không có chuyển sang SKIPPED. | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-5-guard` |
| `TCOV-SCHEDULED-TRANSFER-L2-TRN-ScheduledRun-5-stateTransition-c` | `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-5-stateTransition` | condition | Khi tiếp tục lịch PAUSED, lần chạy PENDING đã quá hạn trong lúc tạm dừng chuyển sang SKIPPED và không chạy bù. |  | PENDING → SKIPPED; không có lời gọi lõi chuyển tiền và không có lần chạy bù mới. | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-5` |
| `TCOV-SCHEDULED-TRANSFER-L2-TRN-ScheduledRun-6-guardViolation-guard-c` | `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-6-guardViolation-guard` | condition | Đầu chu kỳ quét, lần chạy PENDING quá hạn không hơn 24 giờ so với scheduled_at thì không bị MISSED. |  | status=PENDING; không có sự kiện MISSED. | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-6-guard` |
| `TCOV-SCHEDULED-TRANSFER-L2-TRN-ScheduledRun-6-stateTransition-c` | `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-6-stateTransition` | condition | Đầu chu kỳ quét, lần chạy PENDING quá hạn hơn 24 giờ so với scheduled_at chuyển sang MISSED. |  | PENDING → MISSED; 1 sự kiện ScheduledRunFailed có reasonCode=MISSED; 1 audit; consecutive_failures không đổi. | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-6` |
| `TCOV-SCHEDULED-TRANSFER-L2-TRN-ScheduledRun-7-stateTransition-c` | `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-7-stateTransition` | condition | Khi DELETE lịch, lần chạy PENDING của lịch chuyển sang CANCELLED. |  | PENDING → CANCELLED; chu kỳ quét sau đó không chiếm lần chạy này. | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-7` |
| `TCOV-SCHEDULED-TRANSFER-L2-TRN-ScheduledRun-8-guardViolation-guard-c` | `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-8-guardViolation-guard` | condition | Lần chạy RUNNING có started_at chưa cũ hơn lease (120 giây) thì không bị phục hồi về PENDING. |  | status=RUNNING; attempts không đổi. | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-8-guard` |
| `TCOV-SCHEDULED-TRANSFER-L2-TRN-ScheduledRun-8-stateTransition-c` | `TCND-SCHEDULED-TRANSFER-TRN-ScheduledRun-8-stateTransition` | condition | Lần chạy RUNNING có started_at cũ hơn lease 120 giây được scheduler phục hồi về PENDING và giữ nguyên attempts. |  | RUNNING → PENDING; attempts bằng giá trị trước phục hồi. | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-8` |

### TD-SCHEDULED-TRANSFER-L3-MECH-CatchUp-errorGuessing

- TR: `TR-SCHEDULED-TRANSFER-L3-MECH-CatchUp-reliability` (L3, reliability)
- Kỹ thuật: errorGuessing; tiêu chí phủ: Mỗi tình huống lỗi đoán trước một coverage item; mục tiêu 100 %
- Lý do: errorGuessing phù hợp idempotency

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L3-MECH-CatchUp-idempotency-p602a3c-c` | `TCND-SCHEDULED-TRANSFER-MECH-CatchUp-idempotency-p602a3c` | condition | Lần chạy quá hạn ≤ 24 giờ được chạy bù đúng 1 lần dù khởi động lại nhiều lần hoặc có nhiều chu kỳ quét liên tiếp. |  | Số giao dịch của mỗi lần chạy bù = 1; không có bản ghi chạy trùng. | `TORC-SCHEDULED-TRANSFER-AC-FR-049-1-p602a3c` |

### TD-SCHEDULED-TRANSFER-L3-MECH-CatchUp-faultInjection

- TR: `TR-SCHEDULED-TRANSFER-L3-MECH-CatchUp-reliability` (L3, reliability)
- Kỹ thuật: faultInjection; tiêu chí phủ: Mỗi mô hình lỗi được tiêm ít nhất một lần; mục tiêu 100 %
- Lý do: faultInjection phù hợp resilience

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L3-MECH-CatchUp-resilience-holds-c` | `TCND-SCHEDULED-TRANSFER-MECH-CatchUp-resilience-holds` | condition | Sau khởi động lại, lần quá hạn ≤ 24 giờ chạy bù đúng 1 lần trong 5 chu kỳ quét đầu; lần quá hạn > 24 giờ chuyển MISSED, không chạy bù. |  | Lần quá hạn ≤ 24 giờ: chạy đúng 1 lần, xử lý xong trong 5 chu kỳ quét đầu. Lần quá hạn > 24 giờ: trạng thái MISSED, sự kiện ScheduledRunFailed reasonCode=MISSED, 0 giao dịch. | `TORC-SCHEDULED-TRANSFER-MECH-CatchUp-holds` |

### TD-SCHEDULED-TRANSFER-L3-MECH-RunClaim-faultInjection

- TR: `TR-SCHEDULED-TRANSFER-L3-MECH-RunClaim-reliability` (L3, reliability)
- Kỹ thuật: faultInjection; tiêu chí phủ: Mỗi mô hình lỗi được tiêm ít nhất một lần; mục tiêu 100 %
- Lý do: faultInjection phù hợp concurrency, resilience

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L3-MECH-RunClaim-concurrency-parallel-c` | `TCND-SCHEDULED-TRANSFER-MECH-RunClaim-concurrency-parallel` | condition | Hai scheduler cùng cố chiếm một lần chạy đến hạn trong cùng chu kỳ quét 30 giây; đúng một bên chiếm được. |  | Số giao dịch tạo ra cho lần chạy = 1; số scheduler chiếm thành công = 1; số giao dịch trùng = 0. | `TORC-SCHEDULED-TRANSFER-MECH-RunClaim-parallel` |
| `TCOV-SCHEDULED-TRANSFER-L3-MECH-RunClaim-resilience-crash-mid-c` | `TCND-SCHEDULED-TRANSFER-MECH-RunClaim-resilience-crash-mid` | faultModel | Tiến trình chết giữa lúc đã commit chiếm lần chạy và lúc hoàn tất chuyển tiền, sau đó khởi động lại; tổng giao dịch ≤ 1. |  | Số giao dịch cho lần chạy ≤ 1 sau khởi động lại; không có giao dịch thứ hai. | `TORC-SCHEDULED-TRANSFER-MECH-RunClaim-crash` |

### TD-SCHEDULED-TRANSFER-L3-MECH-ScheduleRetry-faultInjection

- TR: `TR-SCHEDULED-TRANSFER-L3-MECH-ScheduleRetry-reliability` (L3, reliability)
- Kỹ thuật: faultInjection; tiêu chí phủ: Mỗi mô hình lỗi được tiêm ít nhất một lần; mục tiêu 100 %
- Lý do: faultInjection phù hợp resilience

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L3-MECH-ScheduleRetry-resilience-holds-c` | `TCND-SCHEDULED-TRANSFER-MECH-ScheduleRetry-resilience-holds` | condition | Một coverage item cho điều kiện resilience: kiểm chính sách thử lại của từng loại lỗi BR-13 (số lần, khoảng cách, FAILED kèm reasonCode, FAILED ngay) và chuyển lịch PAUSED sau 3 run FAILED liên tiếp. |  | FRAUD_CHECK_UNAVAILABLE: tối đa 3 lần thử lại, cách 5 phút, hết lượt run FAILED có reasonCode. INSUFFICIENT_FUNDS: tối đa 2 lần thử lại, cách 1 giờ, hết lượt run FAILED. ACCOUNT_INACTIVE/FRAUD_REJECTED/LIMIT_EXCEEDED: FAILED ngay, 0 lần thử lại. Sau 3 run FAILED liên tiếp lịch = PAUSED. | `TORC-SCHEDULED-TRANSFER-MECH-ScheduleRetry-holds` |

### TD-SCHEDULED-TRANSFER-L3-MECH-SchedulerThrottle-loadModel

- TR: `TR-SCHEDULED-TRANSFER-L3-MECH-SchedulerThrottle-performance` (L3, performance)
- Kỹ thuật: loadModel; tiêu chí phủ: Mỗi chỉ tiêu tải đo theo workload khai báo; mục tiêu 100 %
- Lý do: loadModel phù hợp performance

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L3-MECH-SchedulerThrottle-performance-at-limit-c` | `TCND-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-performance-at-limit` | condition | Khi đúng 20 lần chạy đến hạn trong một chu kỳ quét, scheduler xử lý đủ cả 20 lần chạy và tốc độ gọi không vượt 10 rps. |  | Số lần chạy được xử lý trong một chu kỳ <= 20; tốc độ yêu cầu <= 10 rps; p95 thời gian chuyển tiền qua API <= 500 ms. | `TORC-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-limit` |
| `TCOV-SCHEDULED-TRANSFER-L3-MECH-SchedulerThrottle-performance-over-limit-c` | `TCND-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-performance-over-limit` | condition | Khi số lần chạy đến hạn vượt 20 trong một chu kỳ quét (ví dụ 100 lần chạy cùng lúc), scheduler chỉ xử lý tối đa 20 lần chạy trong chu kỳ đó. |  | Số lần chạy được xử lý trong một chu kỳ <= 20; tốc độ yêu cầu <= 10 rps; p95 thời gian chuyển tiền qua API <= 500 ms. | `TORC-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-limit` |

### TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-boundaryValue

- TR: `TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler-performance` (L3, performance)
- Kỹ thuật: boundaryValue; tiêu chí phủ: Mỗi biên hai giá trị (trong và ngoài biên); mục tiêu 100 %
- Lý do: boundaryValue phù hợp timing

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-timing-p077a10-c` | `TCND-SCHEDULED-TRANSFER-MOD-Scheduler-timing-p077a10` | condition | Chu kỳ quét có nhiều hơn 20 run đến hạn; kiểm số run được chiếm mỗi chu kỳ và độ trễ bắt đầu của phần còn lại. |  | Tối đa 20 run được chiếm mỗi chu kỳ; ít nhất 99% run bắt đầu trong 60 giây kể từ thời điểm dự kiến. | `TORC-SCHEDULED-TRANSFER-AC-NFR-010-1-p077a10` |

### TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-faultInjection

- TR: `TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler-reliability` (L3, reliability)
- Kỹ thuật: faultInjection; tiêu chí phủ: Mỗi mô hình lỗi được tiêm ít nhất một lần; mục tiêu 100 %
- Lý do: faultInjection phù hợp concurrency, errorHandling

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-concurrency-p933b8e-c` | `TCND-SCHEDULED-TRANSFER-MOD-Scheduler-concurrency-p933b8e` | condition | Hai scheduler cùng quét và chiếm một run đến hạn. |  | Đúng một giao dịch COMPLETED cho run này; tiến trình còn lại không chiếm được run. | `TORC-SCHEDULED-TRANSFER-RISK-005-p933b8e` |
| `TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-errorHandling-dep-MOD-Account-c` | `TCND-SCHEDULED-TRANSFER-MOD-Scheduler-errorHandling-dep-MOD-Account` | faultModel | Account lỗi hoặc không phản hồi khi Scheduler tra cứu tài khoản. |  | Chờ làm rõ hành vi; tối thiểu không có giao dịch COMPLETED trùng và run không kẹt ở RUNNING. | `TORC-SCHEDULED-TRANSFER-MOD-Account-dep-MOD-Scheduler` |
| `TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-errorHandling-dep-MOD-Audit-c` | `TCND-SCHEDULED-TRANSFER-MOD-Scheduler-errorHandling-dep-MOD-Audit` | faultModel | Audit lỗi hoặc không phản hồi khi Scheduler ghi audit (tác nhân SCHEDULER). |  | Chờ làm rõ hành vi; tối thiểu trạng thái run và giao dịch phải nhất quán với nhau. | `TORC-SCHEDULED-TRANSFER-MOD-Audit-dep-MOD-Scheduler` |
| `TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-errorHandling-dep-MOD-Risk-c` | `TCND-SCHEDULED-TRANSFER-MOD-Scheduler-errorHandling-dep-MOD-Risk` | faultModel | Risk adapter lỗi (timeout, 5xx, 4xx, sai schema) khi run đến hạn gọi lõi chuyển tiền. |  | Số tiền >= 5.000.000 VND: không có giao dịch, retry 3 lần cách 5 phút, sau đó run FAILED với reasonCode FRAUD_CHECK_UNAVAILABLE. Số tiền < 5.000.000 VND: giao dịch COMPLETED với riskCheck=SKIPPED, run SUCCEEDED. | `TORC-SCHEDULED-TRANSFER-MOD-Risk-dep-MOD-Scheduler` |
| `TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-errorHandling-dep-MOD-Transfer-c` | `TCND-SCHEDULED-TRANSFER-MOD-Scheduler-errorHandling-dep-MOD-Transfer` | faultModel | Lõi chuyển tiền lỗi hoặc không phản hồi khi Scheduler gọi cho run đến hạn. |  | Với lỗi nghiệp vụ: số lần thử lại, khoảng cách và trạng thái run đúng như AC-FR-046-2/3; sau 3 run FAILED liên tiếp lịch là PAUSED; không có giao dịch trùng. | `TORC-SCHEDULED-TRANSFER-MOD-Transfer-dep-MOD-Scheduler` |

## 3. Oracle

| Mã | Loại | Kết quả mong đợi | Cách đo | Dung sai | Nguồn |
| --- | --- | --- | --- | --- | --- |
| `TORC-SCHEDULED-TRANSFER-AC-FR-040-1` | acceptanceCriterion | HTTP 201; body có scheduleId không rỗng và status=ACTIVE. | Kiểm tra mã trạng thái và trường body của phản hồi, đối chiếu bản ghi trong kho scheduled_transfers. |  | `AC-FR-040-1` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-040-1-pab4508` | acceptanceCriterion | 201, có scheduleId, status=ACTIVE. |  |  | `AC-FR-040-1` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-040-2` | acceptanceCriterion | Số dư hai tài khoản không đổi; số bản ghi ledger_entries sau bằng số trước. | Truy vấn số dư và đếm ledger_entries ngay trước và ngay sau lời gọi, trước khi lần chạy đến hạn. | Không dung sai (0 VND, 0 bản ghi). | `AC-FR-040-2` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-040-2-p420954` | invariant | Số dư hai tài khoản và số ledger_entries không đổi. |  |  | `AC-FR-040-2` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-040-3` | acceptanceCriterion | HTTP 422, errorCode=SCHEDULE_TIME_INVALID cho giá trị ngoài khoảng; không có bản ghi lịch mới. | Kiểm tra mã HTTP, errorCode và số bản ghi scheduled_transfers không đổi. |  | `AC-FR-040-3` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-040-3-p31f6cf` | errorContract | Giá trị dưới 5 phút và trên 90 ngày -> 422 SCHEDULE_TIME_INVALID; giá trị trong khoảng -> 201. |  | Việc biên 5 phút và 90 ngày đúng bằng có được chấp nhận hay không nguồn chưa nêu. | `AC-FR-040-3` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-040-4` | acceptanceCriterion | HTTP 404, errorCode=ACCOUNT_NOT_FOUND; không có bản ghi lịch mới. | Kiểm tra mã HTTP, errorCode và số bản ghi scheduled_transfers không đổi. |  | `AC-FR-040-4` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-040-4-pe11059` | errorContract | HTTP 404 ACCOUNT_NOT_FOUND; không có bản ghi lịch mới. |  |  | `AC-FR-040-4` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-040-5` | acceptanceCriterion | HTTP 403, errorCode=FORBIDDEN; không có bản ghi lịch mới. | Kiểm tra mã HTTP, errorCode và số bản ghi scheduled_transfers không đổi. |  | `AC-FR-040-5` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-040-5-p73a390` | errorContract | HTTP 403 FORBIDDEN; không có bản ghi lịch mới. |  |  | `AC-FR-040-5` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-040-6` | acceptanceCriterion | HTTP 422, errorCode=SCHEDULE_LIMIT_REACHED; số lịch ACTIVE/PAUSED của tài khoản vẫn là 20. | Kiểm tra mã HTTP, errorCode và đếm lịch ACTIVE/PAUSED của tài khoản sau lời gọi. |  | `AC-FR-040-6` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-040-6-pc9af5d` | errorContract | Lịch thứ 20 -> 201; lịch thứ 21 -> 422 SCHEDULE_LIMIT_REACHED. |  |  | `AC-FR-040-6` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-041-1` | acceptanceCriterion | HTTP 201, status=ACTIVE, có mã lịch trong phản hồi; kiểm tra cho từng recurrence và từng cách đặt giới hạn (chỉ endDate, chỉ maxRuns, cả hai). | So sánh mã trạng thái và nội dung phản hồi; đọc lại bản ghi lịch trong CSDL. |  | `AC-FR-041-1` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-041-2` | acceptanceCriterion | HTTP 400, errorCode=INVALID_REQUEST; số bản ghi scheduled_transfers của tài khoản không đổi. | So sánh mã HTTP và errorCode; đếm bản ghi trước và sau yêu cầu. |  | `AC-FR-041-2` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-041-2-p4e161f` | errorContract | 400 INVALID_REQUEST. |  |  | `AC-FR-041-2` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-041-3` | acceptanceCriterion | Với maxRuns=N: có đúng N lần chạy, thời điểm cách nhau đúng chu kỳ, lịch COMPLETED sau lần thứ N. Với endDate: không có lần chạy nào sau endDate, lịch COMPLETED. | Đọc bảng scheduled_runs và trạng thái scheduled_transfers sau mỗi chu kỳ quét; dùng đồng hồ giả lập. | Thời điểm dự kiến của lần kế tiếp đúng theo chu kỳ, không sai lệch. | `AC-FR-041-3` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-041-4` | acceptanceCriterion | Neo 31: 31/01 → 28/02 (năm thường) hoặc 29/02 (năm nhuận) → 31/03 → 30/04. Neo 30: tháng 2 → ngày cuối tháng. Neo 29: 28/02 năm thường, 29/02 năm nhuận. | So sánh ngày của các lần chạy PENDING sinh ra với bảng ngày kỳ vọng, dùng cả năm nhuận và năm thường. |  | `AC-FR-041-4` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-041-4-p2341c6` | acceptanceCriterion | Lần chạy ở tháng 30 ngày vào ngày 30, tháng 2 vào ngày 28 (hoặc 29 năm nhuận). |  |  | `AC-FR-041-4` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-042-1` | acceptanceCriterion | HTTP 200; body có status và nextRunAt khớp giá trị lưu trong ScheduledTransfer. |  |  | `AC-FR-042-1` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-042-2` | acceptanceCriterion | HTTP 200; mọi phần tử thuộc accountId và đúng status lọc; số phần tử <= limit; trang kế tiếp không trùng, không sót phần tử so với trang trước. |  |  | `AC-FR-042-2` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-042-3` | acceptanceCriterion | HTTP 200; danh sách giảm dần theo scheduledAt; mỗi phần tử có đủ scheduledAt, status, transferId, reasonCode khớp ScheduledRun. |  |  | `AC-FR-042-3` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-042-4` | acceptanceCriterion | HTTP 403 mã FORBIDDEN cho người không phải chủ; HTTP 404 mã SCHEDULE_NOT_FOUND cho lịch không tồn tại. |  |  | `AC-FR-042-4` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-043-1` | acceptanceCriterion | HTTP 200; ScheduledTransfer.status = CANCELLED; mọi ScheduledRun PENDING của lịch có status = CANCELLED. | Đọc lại lịch và các lần chạy từ CSDL sau phản hồi, so sánh trạng thái trước/sau. |  | `AC-FR-043-1` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-043-2` | acceptanceCriterion | HTTP 200, nội dung phản hồi tương đương kết quả hủy ban đầu; trạng thái lịch vẫn CANCELLED. | So sánh phản hồi lần 1 và lần 2 và trạng thái trong CSDL trước/sau lần gọi lặp. |  | `AC-FR-043-2` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-043-3` | acceptanceCriterion | HTTP 409, errorCode = SCHEDULE_NOT_CANCELLABLE; lịch và lần chạy giữ nguyên trạng thái trước khi gọi. | Kiểm tra mã HTTP, errorCode và đọc lại trạng thái trong CSDL cho cả hai trường hợp A và B. |  | `AC-FR-043-3` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-044-1` | acceptanceCriterion | HTTP 200, status=PAUSED trong phản hồi và trong bản ghi lịch; không phát sinh giao dịch chuyển tiền mới cho lịch khi qua thời điểm dự kiến. | Đọc lại lịch và đếm giao dịch/lần chạy của lịch sau ít nhất 2 chu kỳ quét (60 giây) vượt qua thời điểm dự kiến. | Không dung sai về số giao dịch phát sinh: đúng 0. | `AC-FR-044-1` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-044-2` | acceptanceCriterion | HTTP 200, status=ACTIVE; mọi lần bỏ lỡ trong lúc tạm dừng có status SKIPPED và không có giao dịch tương ứng; chạy bù (FR-049) không áp dụng cho các lần này. | Đọc scheduled_runs của lịch và giao dịch liên quan sau ít nhất 2 chu kỳ quét (60 giây) kể từ khi tiếp tục. | Không dung sai: số giao dịch phát sinh cho các lần bỏ lỡ đúng bằng 0. | `AC-FR-044-2` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-044-3` | acceptanceCriterion | HTTP 409, errorCode=SCHEDULE_STATE_INVALID; bản ghi lịch giữ nguyên status trước đó. | So sánh bản ghi lịch và scheduled_runs trước và sau lời gọi PATCH. |  | `AC-FR-044-3` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-045-1` | acceptanceCriterion | Đếm Transfer theo lần chạy = 1 và trạng thái COMPLETED; run.status = SUCCEEDED; run.transferId = id của Transfer đó. | Truy vấn CSDL SQLite sau khi quét xong một chu kỳ; đối chiếu với TC-S04. | Không dung sai về số lượng giao dịch. | `AC-FR-045-1` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-045-2` | acceptanceCriterion | Idempotency-Key ghi nhận trên giao dịch bằng đúng chuỗi ghép mã lịch và thời điểm dự kiến; hai lần gọi cùng cặp này trả về cùng một giao dịch. | So khớp khóa lưu trong CSDL với giá trị tính từ mã lịch và thời điểm dự kiến của run. |  | `AC-FR-045-2` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-045-3` | acceptanceCriterion | Quyết định chấp nhận/từ chối khớp với số dư, hạn mức, trạng thái tài khoản và kết quả Fraud Check tại lúc chạy; lời gọi Fraud Check xảy ra trong lúc chạy. | Chạy từng kịch bản thay đổi một yếu tố giữa lúc tạo và lúc chạy; kiểm tra kết quả run/Transfer và số lời gọi tới mock Fraud Check. |  | `AC-FR-045-3` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-045-4` | acceptanceCriterion | Hạn mức đã dùng = X + A; lịch sử giao dịch chứa Transfer của lần chạy với số tiền A. | Đọc hạn mức đã dùng và gọi tra cứu lịch sử giao dịch trước và sau lần chạy. |  | `AC-FR-045-4` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-046-1` | acceptanceCriterion | Số lần thử lại = 3, khoảng cách 5 phút; sau lần lỗi cuối run = FAILED, reasonCode = FRAUD_CHECK_UNAVAILABLE. | Đối chiếu bản ghi scheduled_runs (trạng thái, số lần thử, thời điểm thử) với đồng hồ giả lập trong kiểm thử tích hợp. | Khoảng cách thử lại tính theo chu kỳ quét 30 giây; chưa có nguồn nêu sai số cho phép. | `AC-FR-046-1` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-046-2` | acceptanceCriterion | Số lần thử lại = 2, khoảng cách 1 giờ; sau lần lỗi cuối run = FAILED. | Đối chiếu bản ghi scheduled_runs (trạng thái, số lần thử, thời điểm thử) với đồng hồ giả lập trong kiểm thử tích hợp. | Khoảng cách thử lại tính theo chu kỳ quét 30 giây; chưa có nguồn nêu sai số cho phép. | `AC-FR-046-2` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-046-3` | acceptanceCriterion | Run = FAILED ngay với reasonCode tương ứng mã lỗi; số lần thử lại = 0; không có lần thử nào ở các chu kỳ quét sau. | Chạy 3 ca (mỗi mã lỗi một ca), quét thêm ít nhất 1 chu kỳ sau đó và xác nhận không có lần thử mới trong scheduled_runs. |  | `AC-FR-046-3` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-046-4` | acceptanceCriterion | Lịch = PAUSED ngay sau lần FAILED thứ 3 liên tiếp; lịch vẫn ACTIVE nếu chuỗi FAILED liên tiếp < 3. | Đọc trạng thái scheduled_transfers sau mỗi lần chạy trong kiểm thử tích hợp, gồm ca chuỗi 2 FAILED, ca 3 FAILED liên tiếp và ca có 1 run thành công xen giữa. |  | `AC-FR-046-4` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-046-4-pfec2a2` | stateMachine | Lịch ACTIVE → PAUSED sau 3 run FAILED liên tiếp; run SUCCEEDED xen giữa thì đếm lại từ đầu (chờ xác nhận). |  |  | `AC-FR-046-4` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-047-1` | acceptanceCriterion | Đúng 1 sự kiện ScheduledRunCompleted trong outbox cho mỗi lần chạy thành công; payload.transferId khác rỗng và bằng transferId của giao dịch. | Truy vấn bảng outbox và bảng giao dịch sau khi lần chạy kết thúc, đối chiếu transferId; chạy trên ENV-Demo với SQLite. | Không dung sai về số lượng sự kiện (0 thiếu, 0 trùng). | `AC-FR-047-1` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-047-2` | acceptanceCriterion | Đúng 1 sự kiện ScheduledRunFailed trong outbox cho mỗi lần chạy FAILED; payload.reasonCode khác rỗng và bằng mã lý do lưu trên lần chạy. | Truy vấn bảng outbox và bản ghi lần chạy sau khi chuyển FAILED; chạy trên ENV-Demo với SQLite. | Không dung sai về số lượng sự kiện (0 thiếu, 0 trùng). | `AC-FR-047-2` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-047-3` | acceptanceCriterion | Trường hợp bình thường: có bản ghi SMS cho người gửi với nội dung từ mẫu kết quả lần chạy. Trường hợp lỗi SMS: trạng thái lần chạy trước và sau khi gửi lỗi giống nhau. | So sánh trạng thái lần chạy trước và sau khi tiêm lỗi SMS Gateway; kiểm tra bảng sms_log; đo thời gian trường hợp xấu nhất từ lúc ghi outbox đến lúc ghi sms_log trên ENV-Demo. | Không dung sai đối với trạng thái lần chạy; tổng thời gian từ outbox đến SMS nằm trong ngân sách 60000 ms của SCN-Notify. | `AC-FR-047-3` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-047-3-p00fe49` | postcondition | Trạng thái run giữ nguyên (SUCCEEDED hoặc FAILED) sau lỗi SMS. |  |  | `AC-FR-047-3` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-048-1` | acceptanceCriterion | Sau 4 thao tác thành công, AuditLog có đúng 4 bản ghi mới cho mã lịch đó, mỗi bản ghi đủ correlationId, thời điểm, tác nhân = mã tài khoản (không phải SCHEDULER), loại sự kiện phân biệt, mã lịch. | Truy vấn AuditLog theo mã lịch sau mỗi thao tác trong SQLite và so khớp từng trường. | Thời điểm audit lệch không quá thời điểm xử lý yêu cầu theo đồng hồ hệ thống; chưa có nguồn định lượng dung sai chính xác. | `AC-FR-048-1` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-048-1-p1cc3a7` | postcondition | Một bản ghi audit với correlationId, thời điểm, actor = mã tài khoản, loại sự kiện hủy, mã lịch. | Truy vấn bảng audit theo mã lịch sau phản hồi 200. |  | `AC-FR-048-1` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-048-1-p28bd0d` | acceptanceCriterion | Một bản ghi audit đủ các trường nêu trên. |  |  | `AC-FR-048-1` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-048-1-p664d35` | postcondition | Tồn tại bản ghi audit đủ các trường correlationId, thời điểm, tác nhân = mã tài khoản, loại sự kiện, mã lịch. |  |  | `AC-FR-048-1` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-048-1-pf42db3` | postcondition | Một bản ghi audit chứa correlationId, thời điểm, tác nhân = mã tài khoản, loại sự kiện tạo lịch, scheduleId. |  |  | `AC-FR-048-1` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-048-2` | acceptanceCriterion | Với mỗi lần chạy đã có kết quả, AuditLog có bản ghi actor = SCHEDULER, mã lịch = lịch của lần chạy; số bản ghi khớp số lần chạy đã xử lý. | Truy vấn AuditLog theo mã lịch sau khi lần chạy chuyển sang trạng thái cuối; đối chiếu với trạng thái lần chạy (kịch bản TC-S07 mức integration). |  | `AC-FR-048-2` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-049-1` | acceptanceCriterion | Đúng 1 giao dịch cho lần chạy bù; lần chạy không còn ở trạng thái chờ; quét thêm không tạo giao dịch mới. | Đếm giao dịch theo Idempotency-Key của lần chạy sau tối thiểu 6 chu kỳ quét; đếm chu kỳ quét tính từ lúc khởi động lại. | Xử lý trong 5 chu kỳ quét đầu sau khởi động lại (NFR-012). | `AC-FR-049-1` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-049-1-p602a3c` | postcondition | Số giao dịch của mỗi lần chạy bù = 1; không có bản ghi chạy trùng. |  | Không dung sai. | `AC-FR-049-1` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-049-2` | acceptanceCriterion | Trạng thái lần chạy = MISSED; có 1 sự kiện ScheduledRunFailed reasonCode MISSED; 0 giao dịch cho lần chạy này; số dư tài khoản nguồn và đích không đổi. | Truy vấn scheduled_runs, bảng giao dịch theo Idempotency-Key của lần chạy, outbox và số dư sau chu kỳ quét. |  | `AC-FR-049-2` |
| `TORC-SCHEDULED-TRANSFER-AC-FR-049-2-pd98b3f` | decisionRow | ≤ 24 giờ: 1 giao dịch, xử lý trong 5 chu kỳ quét đầu; > 24 giờ: MISSED, 0 giao dịch, ScheduledRunFailed reasonCode MISSED. | Đếm số chu kỳ quét 30 giây kể từ khi khởi động; kiểm tra biên đúng 24 giờ theo đồng hồ giả. |  | `AC-FR-049-2` |
| `TORC-SCHEDULED-TRANSFER-AC-NFR-010-1` | acceptanceCriterion | Số lần chạy có độ trễ bắt đầu <= 60 s chia tổng số lần chạy >= 0,99. | Đo trên đồng hồ giả, trên toàn bộ mẫu lần chạy của kịch bản (không dùng thời gian thực); báo cáo tỷ lệ đạt và độ trễ lớn nhất. | Không có dung sai dưới 99%; độ trễ tối đa lý thuyết với chu kỳ 30 s là khoảng 30 s. | `AC-NFR-010-1` |
| `TORC-SCHEDULED-TRANSFER-AC-NFR-010-1-p077a10` | threshold | Tối đa 20 run được chiếm mỗi chu kỳ; ít nhất 99% run bắt đầu trong 60 giây kể từ thời điểm dự kiến. | Đồng hồ giả, chu kỳ quét 30 giây, đo trên toàn bộ tập run của kịch bản (phân vị 99). |  | `AC-NFR-010-1` |
| `TORC-SCHEDULED-TRANSFER-AC-NFR-011-1` | acceptanceCriterion | Đúng 1 giao dịch ứng với Idempotency-Key (mã lịch + thời điểm dự kiến); chỉ 1 scheduler chiếm được lần chạy; không có giao dịch thứ hai. | Đếm số giao dịch theo Idempotency-Key của lần chạy sau khi cả hai scheduler hoàn tất chu kỳ quét (TC-S10). | Không có dung sai: số giao dịch trùng phải bằng 0. | `AC-NFR-011-1` |
| `TORC-SCHEDULED-TRANSFER-AC-NFR-011-1-p775a37` | invariant | Số giao dịch trùng cho một lần chạy = 0; đúng một UPDATE có điều kiện thành công. |  |  | `AC-NFR-011-1` |
| `TORC-SCHEDULED-TRANSFER-AC-NFR-011-2` | acceptanceCriterion | Số giao dịch ứng với Idempotency-Key của lần chạy <= 1 sau khi khởi động lại và quét lại; không có giao dịch thứ hai. | Đếm giao dịch theo Idempotency-Key sau khi khởi động lại và chạy hết các chu kỳ quét cần thiết (TC-S11). | Không có dung sai: 0 giao dịch trùng. | `AC-NFR-011-2` |
| `TORC-SCHEDULED-TRANSFER-AC-NFR-011-2-p12901f` | invariant | Tối đa một giao dịch cho lần chạy. |  |  | `AC-NFR-011-2` |
| `TORC-SCHEDULED-TRANSFER-AC-NFR-012-1` | acceptanceCriterion | Sau chu kỳ quét thứ 5, số lần chạy quá hạn ≤ 24 giờ còn chưa xử lý bằng 0; mỗi chu kỳ xử lý tối đa 20 lần chạy. | Đếm số chu kỳ quét từ lần quét đầu sau khởi động đến khi hết lần chạy quá hạn; tập dữ liệu ≤ 100 lần chạy quá hạn. | Không dung sai về số chu kỳ: tối đa 5 chu kỳ quét đầu. | `AC-NFR-012-1` |
| `TORC-SCHEDULED-TRANSFER-AC-NFR-013-1` | acceptanceCriterion | p95 thời gian chuyển tiền qua API <= 500 ms khi 100 lần chạy đến hạn cùng lúc. | Phân vị 95 trên toàn bộ 100 lần chạy của một đợt benchmark (TC-S13), đo thời gian từ lúc gửi yêu cầu chuyển tiền đến khi nhận phản hồi; lặp lại nhiều đợt, lấy đợt xấu nhất, không cộng p95 các thành phần. | Không có dung sai trên ngưỡng 500 ms. | `AC-NFR-013-1` |
| `TORC-SCHEDULED-TRANSFER-AC-NFR-013-1-pa101d9` | threshold | p95 ≤ 500 ms; số lần chạy mỗi chu kỳ ≤ 20. | p95 của thời gian chuyển tiền qua API trong toàn bộ đợt 100 lần chạy, đo ở tải 100 lần chạy đến hạn cùng lúc. |  | `AC-NFR-013-1` |
| `TORC-SCHEDULED-TRANSFER-AC-NFR-013-2` | acceptanceCriterion | Số lần chạy được chọn và chiếm trong một chu kỳ quét <= 20. | Đếm số lần chạy được chiếm (UPDATE có điều kiện thành công) trong từng chu kỳ quét 30 giây của đợt benchmark 100 lần chạy; kiểm tra mọi chu kỳ. | Không có dung sai: 21 lần chạy trở lên trong một chu kỳ là vi phạm. | `AC-NFR-013-2` |
| `TORC-SCHEDULED-TRANSFER-AC-NFR-014-1` | acceptanceCriterion | Số correlationId phân biệt = số lần chạy; mọi lần chạy đều có correlationId khác rỗng. | Truy vấn correlationId của các lần chạy sau khi xử lý xong và đếm DISTINCT so với số lần chạy. | Không dung sai: 0 cặp lần chạy trùng correlationId. | `AC-NFR-014-1` |
| `TORC-SCHEDULED-TRANSFER-AC-NFR-014-2` | acceptanceCriterion | Giá trị correlationId trong giao dịch, audit, lời gọi Fraud Check và sự kiện bằng đúng giá trị correlationId của lần chạy. | Đối chiếu correlationId của lần chạy với bản ghi giao dịch, audit, request ghi nhận tại mock Fraud Check và payload sự kiện trong outbox (xem TC-S12). | Không dung sai: cả 4 nơi đều phải có và khớp. | `AC-NFR-014-2` |
| `TORC-SCHEDULED-TRANSFER-AC-NFR-014-2-p12d52f` | invariant | Không có hai lần chạy chung correlationId; correlationId xuất hiện ở cả 4 nơi. |  |  | `AC-NFR-014-2` |
| `TORC-SCHEDULED-TRANSFER-DE-AuditLog-pdabb36` | invariant | Các bản ghi audit cũ không đổi (mọi trường) sau các thao tác tiếp theo; số bản ghi chỉ tăng. | So băm nội dung các bản ghi cũ trước và sau. |  | `DE-AuditLog` |
| `TORC-SCHEDULED-TRANSFER-DE-ScheduledTransfer-pd7d28d` | invariant | Số lịch ACTIVE/PAUSED của tài khoản luôn ≤ 20. | Đếm lịch ACTIVE/PAUSED sau khi cả hai yêu cầu hoàn tất. |  | `DE-ScheduledTransfer` |
| `TORC-SCHEDULED-TRANSFER-EP-CancelSchedule-idem` | invariant | 200, nội dung phản hồi giống lần đầu; số bản ghi audit hủy không tăng. | So sánh phản hồi hai lần và đếm audit theo scheduleId. | Các trường thời gian sinh động có thể khác nếu phản hồi lần đầu không được lưu lại. | `EP-CancelSchedule` |
| `TORC-SCHEDULED-TRANSFER-EP-CancelSchedule-validation` | schema | Bị từ chối, không có thay đổi trạng thái; mã lỗi chờ làm rõ. | Kiểm tra không có lịch nào đổi trạng thái và không có audit hủy. |  | `EP-CancelSchedule` |
| `TORC-SCHEDULED-TRANSFER-EP-CreateSchedule-idem` | invariant | HTTP 200, scheduleId = S; tổng số lịch và số bản ghi audit tạo lịch không tăng. | Đếm bản ghi lịch và audit theo Idempotency-Key sau khi cả hai yêu cầu hoàn tất. | Một phản hồi là 201, phản hồi kia là 200 hoặc 201 cùng nội dung; nguồn chưa quy định rõ. | `EP-CreateSchedule` |
| `TORC-SCHEDULED-TRANSFER-EP-GetSchedule-p52c40c` | invariant | Hai phản hồi 200 có status và nextRunAt giống nhau; dữ liệu lịch trong kho không đổi sau các lần GET. |  |  | `EP-GetSchedule` |
| `TORC-SCHEDULED-TRANSFER-EP-GetSchedule-validation` | schema | Yêu cầu bị từ chối, không có phản hồi 200; mã HTTP và errorCode chờ làm rõ. |  | Chưa xác định mã lỗi cụ thể. | `EP-GetSchedule` |
| `TORC-SCHEDULED-TRANSFER-EP-ListRuns-validation` | schema | Yêu cầu bị từ chối với lỗi kiểm tra đầu vào; không có dữ liệu lần chạy |  |  | `EP-ListRuns` |
| `TORC-SCHEDULED-TRANSFER-EP-ListSchedules-p6e1377` | postcondition | Hai phản hồi HTTP 200 có cùng nội dung; trạng thái lịch và lần chạy không đổi. |  |  | `EP-ListSchedules` |
| `TORC-SCHEDULED-TRANSFER-FB-INT01-fail-SCN-ScheduledRun` | errorContract | Dưới 5.000.000 VND: riskCheck=SKIPPED, giao dịch COMPLETED. Từ 5.000.000 VND (kể cả đúng 5.000.000): FRAUD_CHECK_UNAVAILABLE (503), không có giao dịch, thử lại tối đa 3 lần cách 5 phút rồi FAILED với reasonCode. |  |  | `FB-INT01` |
| `TORC-SCHEDULED-TRANSFER-FLD-ListSchedules-limit-p433493` | schema | limit=1, 100 và offset=0 trả 200; giá trị ngoài biên bị từ chối (mã lỗi cụ thể xem gaps). |  |  | `FLD-ListSchedules-limit` |
| `TORC-SCHEDULED-TRANSFER-FR-043-p4b4873` | invariant | Không tồn tại lần chạy CANCELLED có giao dịch chuyển tiền được ghi sổ; DELETE trả 200 hoặc 409 tương ứng với kết quả chiếm. | Lặp nhiều lần tranh chấp, đối chiếu trạng thái lần chạy với sổ giao dịch. |  | `FR-043` |
| `TORC-SCHEDULED-TRANSFER-FR-045-p174b2f` | threshold | Số giao dịch trùng cho một lần chạy = 0. | Chạy song song 2 scheduler trên cùng CSDL, lặp nhiều lần; đếm Transfer theo lần chạy. | 0 | `FR-045` |
| `TORC-SCHEDULED-TRANSFER-FR-045-pb0682f` | threshold | Độ trễ bắt đầu <= 60 giây cho 99% lần chạy. | Phân vị 99 của (thời điểm bắt đầu − thời điểm dự kiến) trên tập lần chạy của một đợt kiểm thử, ghi theo giờ hệ thống. |  | `FR-045` |
| `TORC-SCHEDULED-TRANSFER-FR-045-pd8cb5a` | threshold | 100% lần chạy có correlationId riêng và khớp ở giao dịch, audit, lời gọi Fraud Check. | Đối chiếu toàn bộ lần chạy trong một đợt kiểm thử với log/bản ghi audit và request tới mock Fraud Check. | 0 lần chạy thiếu hoặc trùng correlationId | `FR-045` |
| `TORC-SCHEDULED-TRANSFER-FR-048-p1438e9` | postcondition | Tồn tại bản ghi audit cho thao tác hủy lịch gắn với scheduleId. | Truy vấn bảng audit sau khi hủy thành công. |  | `FR-048` |
| `TORC-SCHEDULED-TRANSFER-FR-048-p1cd2c4` | postcondition | Tồn tại bản ghi audit tạo lịch liên kết mã lịch; trường tác nhân cụ thể cho thao tác tạo của người dùng chưa nêu trong nguồn. | Truy vấn bảng audit theo mã lịch sau khi nhận 201. |  | `FR-048` |
| `TORC-SCHEDULED-TRANSFER-FR-048-p7a419a` | postcondition | Tồn tại bản ghi audit tạo lịch cho scheduleId đã trả. | Truy vấn bảng audit theo scheduleId sau lời gọi. |  | `FR-048` |
| `TORC-SCHEDULED-TRANSFER-FR-048-pafc81c` | postcondition | Có đúng một bản ghi audit cho mỗi lần chuyển trạng thái thành công, tham chiếu scheduleId. |  |  | `FR-048` |
| `TORC-SCHEDULED-TRANSFER-FR-049-pe0788e` | decisionRow | ≤ 24 giờ: chạy bù; > 24 giờ: MISSED, không giao dịch. |  |  | `FR-049` |
| `TORC-SCHEDULED-TRANSFER-MECH-CatchUp-holds` | postcondition | Lần quá hạn ≤ 24 giờ: chạy đúng 1 lần, xử lý xong trong 5 chu kỳ quét đầu. Lần quá hạn > 24 giờ: trạng thái MISSED, sự kiện ScheduledRunFailed reasonCode=MISSED, 0 giao dịch. | Đếm số chu kỳ quét kể từ lần quét đầu sau khởi động lại cho đến khi lần chạy quá hạn ≤ 24 giờ được xử lý; trường hợp xấu nhất, giới hạn 20 lần chạy mỗi chu kỳ. | Không dung sai về số lần chạy (đúng 1) và số giao dịch (0 với MISSED); tối đa 5 chu kỳ quét đầu. | `MECH-CatchUp` |
| `TORC-SCHEDULED-TRANSFER-MECH-RunClaim-crash` | invariant | Số giao dịch cho lần chạy ≤ 1 sau khởi động lại; không có giao dịch thứ hai. | Đếm giao dịch theo Idempotency-Key sau khi khởi động lại và chạy vài chu kỳ quét; kiểm chứng bằng TC-S11 (test_restart_between_claim_and_transfer). | Không dung sai: 0 giao dịch trùng. | `MECH-RunClaim` |
| `TORC-SCHEDULED-TRANSFER-MECH-RunClaim-parallel` | invariant | Số giao dịch tạo ra cho lần chạy = 1; số scheduler chiếm thành công = 1; số giao dịch trùng = 0. | Đếm giao dịch theo Idempotency-Key (mã lịch + thời điểm dự kiến) sau khi cả hai scheduler hoàn tất; kiểm chứng bằng TC-S10 (test_two_schedulers_no_duplicate). | Không dung sai: 0 giao dịch trùng. | `MECH-RunClaim` |
| `TORC-SCHEDULED-TRANSFER-MECH-ScheduleRetry-holds` | postcondition | FRAUD_CHECK_UNAVAILABLE: tối đa 3 lần thử lại, cách 5 phút, hết lượt run FAILED có reasonCode. INSUFFICIENT_FUNDS: tối đa 2 lần thử lại, cách 1 giờ, hết lượt run FAILED. ACCOUNT_INACTIVE/FRAUD_REJECTED/LIMIT_EXCEEDED: FAILED ngay, 0 lần thử lại. Sau 3 run FAILED liên tiếp lịch = PAUSED. | Kiểm tra số lần gọi lõi chuyển tiền, mốc thời gian mỗi lần thử (đồng hồ giả lập), trạng thái run và lịch trong DB (tích hợp, TC-S05). | Khoảng cách thử lại tính theo chu kỳ quét 30 giây của Scheduler; nguồn chưa nêu sai số cụ thể. | `MECH-ScheduleRetry` |
| `TORC-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-limit` | threshold | Số lần chạy được xử lý trong một chu kỳ <= 20; tốc độ yêu cầu <= 10 rps; p95 thời gian chuyển tiền qua API <= 500 ms. | Chạy benchmark 100 lần chạy đến hạn cùng lúc (TC-S13); đếm số lần chạy được xử lý mỗi chu kỳ; đo p95 thời gian chuyển tiền qua API trên toàn bộ lần chạy benchmark. | Không dung sai với giới hạn 20 lần chạy; p95 <= 500 ms theo NFR-013. | `MECH-SchedulerThrottle` |
| `TORC-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-pd5ac38` | threshold | Tốc độ yêu cầu từ scheduler <= 10 rps. | Số yêu cầu mỗi giây, đo theo cửa sổ 1 giây trượt trong toàn bộ đợt benchmark 100 lần chạy; lấy cửa sổ lớn nhất. | Không có dung sai. | `MECH-SchedulerThrottle` |
| `TORC-SCHEDULED-TRANSFER-MOD-Account-dep-MOD-Scheduler` | errorContract | Chờ làm rõ hành vi; tối thiểu không có giao dịch COMPLETED trùng và run không kẹt ở RUNNING. | Đối chiếu bảng scheduled_runs và ledger_entries sau khi tiêm lỗi. |  | `MOD-Account` |
| `TORC-SCHEDULED-TRANSFER-MOD-Audit-dep-MOD-Scheduler` | errorContract | Chờ làm rõ hành vi; tối thiểu trạng thái run và giao dịch phải nhất quán với nhau. | Đối chiếu trạng thái run, ledger_entries và bản ghi audit sau khi tiêm lỗi. |  | `MOD-Audit` |
| `TORC-SCHEDULED-TRANSFER-MOD-Risk-dep-MOD-Scheduler` | errorContract | Số tiền >= 5.000.000 VND: không có giao dịch, retry 3 lần cách 5 phút, sau đó run FAILED với reasonCode FRAUD_CHECK_UNAVAILABLE. Số tiền < 5.000.000 VND: giao dịch COMPLETED với riskCheck=SKIPPED, run SUCCEEDED. | Đồng hồ giả; đếm số lần gọi mock Fraud và số lần thử lại của run. | Mỗi lần gọi Fraud tối đa 1000 ms, tối đa 2 lần gọi (retry 1 lần với timeout/5xx) = 2000 ms; 4xx và sai schema không retry. | `MOD-Risk` |
| `TORC-SCHEDULED-TRANSFER-MOD-Scheduler-p1e8657` | humanJudgment | Cần xác nhận: lần chạy đã chiếm có tiếp tục chạy hay bị hủy/SKIPPED. |  |  | `MOD-Scheduler` |
| `TORC-SCHEDULED-TRANSFER-MOD-Scheduler-p34e996` | invariant | Không có lần chạy bị thực hiện 2 lần; không có lần RUNNING của lịch đã CANCELLED; trạng thái cuối nhất quán với một thứ tự tuần tự hợp lệ. |  |  | `MOD-Scheduler` |
| `TORC-SCHEDULED-TRANSFER-MOD-Scheduler-pa68a97` | invariant | Số lần chạy được chiếm trong một chu kỳ ≤ 20. | Seed 25 lần chạy đến hạn, đếm số bản ghi sang RUNNING sau đúng một chu kỳ. |  | `MOD-Scheduler` |
| `TORC-SCHEDULED-TRANSFER-MOD-Transfer-dep-MOD-Scheduler` | errorContract | Với lỗi nghiệp vụ: số lần thử lại, khoảng cách và trạng thái run đúng như AC-FR-046-2/3; sau 3 run FAILED liên tiếp lịch là PAUSED; không có giao dịch trùng. | Đồng hồ giả; kiểm tra scheduled_runs, trạng thái lịch và ledger_entries. |  | `MOD-Transfer` |
| `TORC-SCHEDULED-TRANSFER-NFR-010` | threshold | Ít nhất 99% lần chạy có (thời điểm bắt đầu - thời điểm dự kiến) <= 60 s. | Tỷ lệ trên toàn bộ lần chạy trong cửa sổ đo với đồng hồ giả (TC-S09); trường hợp xấu nhất gồm cả lần chạy đến hạn ngay sau một lần quét; mốc thời gian lấy theo thời điểm chiếm lần chạy. | Ngưỡng cứng 60 s, tỷ lệ tối thiểu 99%. | `NFR-010` |
| `TORC-SCHEDULED-TRANSFER-NFR-010-p9746f5` | threshold | Mỗi chu kỳ chọn tối đa 20 lần chạy; tỷ lệ lần chạy có độ trễ <= 60 s vẫn >= 99% khi quá tải 40 lần chạy đồng thời. | Đồng hồ giả, một lần phát 40 lần chạy đồng thời, đo độ trễ từng lần chạy; trường hợp xấu nhất tính theo lần chạy cuối cùng. |  | `NFR-010` |
| `TORC-SCHEDULED-TRANSFER-NFR-011` | threshold | Số giao dịch trùng cho mỗi lần chạy = 0. | Với mỗi lần chạy trong lần kiểm thử có >= 2 scheduler quét đồng thời, đếm giao dịch theo Idempotency-Key; mọi lần chạy phải có <= 1 giao dịch (TC-S10, TC-S11). | Không có dung sai. | `NFR-011` |
| `TORC-SCHEDULED-TRANSFER-NFR-011-p4dc0d8` | invariant | 0 giao dịch trùng cho một lần chạy. | Đối chiếu số giao dịch với số lần chạy theo Idempotency-Key sau đợt benchmark. | Không có dung sai. | `NFR-011` |
| `TORC-SCHEDULED-TRANSFER-NFR-011-p92271d` | threshold | 0 giao dịch trùng cho mỗi lần chạy. | Đếm giao dịch theo Idempotency-Key (mã lịch + thời điểm dự kiến) sau khi xử lý hết tập quá hạn. | Không dung sai. | `NFR-011` |
| `TORC-SCHEDULED-TRANSFER-NFR-011-pe3c163` | threshold | 0 giao dịch trùng; đúng 1 giao dịch cho lần chạy. | Đếm giao dịch theo Idempotency-Key sau nhiều vòng quét đồng thời lặp lại (mỗi vòng cả hai Scheduler quét cùng lúc). | Không dung sai. | `NFR-011` |
| `TORC-SCHEDULED-TRANSFER-NFR-012` | threshold | Số chu kỳ quét cần để xử lý hết lần quá hạn ≤ 24 giờ ≤ 5; lần quá hạn > 24 giờ có trạng thái MISSED. | Trường hợp xấu nhất: tải 100 lần chạy quá hạn (5 chu kỳ × 20), đo bằng số chu kỳ quét tính từ lần quét đầu sau khởi động, chu kỳ 30 giây. | 0 chu kỳ vượt chỉ tiêu 5. | `NFR-012` |
| `TORC-SCHEDULED-TRANSFER-NFR-013` | threshold | p95 thời gian chuyển tiền qua API <= 500 ms (chỉ tiêu 500 ms). | p95 trên 100 lần chạy trong một đợt benchmark TC-S13, lặp nhiều đợt và lấy đợt xấu nhất; ghi rõ môi trường và tải đo. | Không có dung sai trên ngưỡng. | `NFR-013` |
| `TORC-SCHEDULED-TRANSFER-NFR-014` | threshold | Tỷ lệ lần chạy truy vết đầy đủ = 100% (targetValue 100, đơn vị %). | Tỷ lệ = số lần chạy có correlationId riêng và xuất hiện đủ ở giao dịch, audit, lời gọi Fraud Check / tổng số lần chạy trong tập kiểm thử; đo trên toàn bộ lần chạy, không lấy mẫu. | Không dung sai; một lần chạy thiếu hoặc trùng correlationId là không đạt. | `NFR-014` |
| `TORC-SCHEDULED-TRANSFER-NFR-014-p477475` | threshold | Tỉ lệ lần chạy có correlationId riêng trong audit khớp với giao dịch và lời gọi Fraud Check là 100%. | Đếm trên toàn bộ lần chạy trong một lượt kiểm thử, đối chiếu AuditLog, giao dịch và log Fraud Check. | 0 lần chạy sai lệch | `NFR-014` |
| `TORC-SCHEDULED-TRANSFER-NFR-014-p8ac829` | invariant | Mọi bản ghi và lời gọi Fraud Check của lần chạy, kể cả khi thử lại, đều có correlationId khác rỗng. | Đối chiếu các bản ghi theo lần chạy sau khi thử lại. |  | `NFR-014` |
| `TORC-SCHEDULED-TRANSFER-RISK-005-p5afd1f` | invariant | Không có hai giao dịch cho cùng Idempotency-Key (mã lịch + thời điểm dự kiến); lịch cuối cùng ở PAUSED. |  |  | `RISK-005` |
| `TORC-SCHEDULED-TRANSFER-RISK-005-p933b8e` | invariant | Đúng một giao dịch COMPLETED cho run này; tiến trình còn lại không chiếm được run. | Chạy song song 2 luồng trên cùng CSDL SQLite, đếm bản ghi ledger_entries theo Idempotency-Key. |  | `RISK-005` |
| `TORC-SCHEDULED-TRANSFER-RSP-Cancel-200` | postcondition | 200; lịch = CANCELLED; mọi lần PENDING trước đó = CANCELLED. | Đọc lại lịch và các lần chạy từ DB sau phản hồi. |  | `RSP-Cancel-200` |
| `TORC-SCHEDULED-TRANSFER-RSP-Cancel-401` | errorContract | Status 401, errorCode = UNAUTHORIZED. | Kiểm tra status, errorCode và lịch vẫn ACTIVE. |  | `RSP-Cancel-401` |
| `TORC-SCHEDULED-TRANSFER-RSP-Cancel-401-p4bf0be` | errorContract | HTTP 401, errorCode UNAUTHORIZED; trạng thái lịch không đổi. | Kiểm tra phản hồi và đọc lại trạng thái lịch. |  | `RSP-Cancel-401` |
| `TORC-SCHEDULED-TRANSFER-RSP-Cancel-401-validation` | schema | Status 401, body errorCode = UNAUTHORIZED. | Kiểm tra status, errorCode và trạng thái lịch trong DB không đổi. |  | `RSP-Cancel-401` |
| `TORC-SCHEDULED-TRANSFER-RSP-Cancel-403` | errorContract | Status 403, errorCode = FORBIDDEN; lịch vẫn ACTIVE. | Kiểm tra status, errorCode và trạng thái lịch trong DB. |  | `RSP-Cancel-403` |
| `TORC-SCHEDULED-TRANSFER-RSP-Cancel-403-pb30d3e` | errorContract | HTTP 403, errorCode FORBIDDEN; trạng thái lịch không đổi. | Kiểm tra phản hồi và đọc lại trạng thái lịch. |  | `RSP-Cancel-403` |
| `TORC-SCHEDULED-TRANSFER-RSP-Cancel-404` | errorContract | Status 404, errorCode = SCHEDULE_NOT_FOUND. | Kiểm tra status và errorCode trong phản hồi. |  | `RSP-Cancel-404` |
| `TORC-SCHEDULED-TRANSFER-RSP-Cancel-404-pa62ef8` | errorContract | HTTP 404, errorCode SCHEDULE_NOT_FOUND. | Kiểm tra mã HTTP và errorCode. |  | `RSP-Cancel-404` |
| `TORC-SCHEDULED-TRANSFER-RSP-Cancel-409` | errorContract | Status 409, errorCode = SCHEDULE_NOT_CANCELLABLE; trạng thái lịch và lần chạy không đổi. | Kiểm tra phản hồi và đọc lại DB cho cả hai ca. |  | `RSP-Cancel-409` |
| `TORC-SCHEDULED-TRANSFER-RSP-Create-200` | postcondition | HTTP 200, body chứa lịch đã tạo với scheduleId ban đầu. |  |  | `RSP-Create-200` |
| `TORC-SCHEDULED-TRANSFER-RSP-Create-200-p50d7db` | postcondition | 200, cùng scheduleId; tổng số lịch không tăng. |  |  | `RSP-Create-200` |
| `TORC-SCHEDULED-TRANSFER-RSP-Create-200-p8f0346` | errorContract | HTTP 200, cùng scheduleId như lần đầu; chỉ có một bản ghi lịch. | So sánh scheduleId và đếm bản ghi scheduled_transfers. |  | `RSP-Create-200` |
| `TORC-SCHEDULED-TRANSFER-RSP-Create-200-pdcbc85` | invariant | Đúng 1 lịch với key K; hai phản hồi cùng scheduleId. |  |  | `RSP-Create-200` |
| `TORC-SCHEDULED-TRANSFER-RSP-Create-201` | postcondition | HTTP 201, scheduleId không rỗng, status=ACTIVE; số dư và số bản ghi ledger_entries không đổi. |  |  | `RSP-Create-201` |
| `TORC-SCHEDULED-TRANSFER-RSP-Create-400` | errorContract | HTTP 400 với errorCode INVALID_REQUEST cho từng biến thể; số lịch không đổi. |  |  | `RSP-Create-400` |
| `TORC-SCHEDULED-TRANSFER-RSP-Create-400-p31ec2a` | errorContract | HTTP 400, errorCode=INVALID_REQUEST; kiểm tra biên 0, 1, 60, 61. | So sánh mã HTTP và errorCode. |  | `RSP-Create-400` |
| `TORC-SCHEDULED-TRANSFER-RSP-Create-400-p49f012` | errorContract | HTTP 400, errorCode=INVALID_REQUEST cho mỗi trường hợp; không có bản ghi lịch mới. | Kiểm tra mã HTTP, errorCode và số bản ghi lịch cho từng ca. |  | `RSP-Create-400` |
| `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` | schema | HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. |  |  | `RSP-Create-400` |
| `TORC-SCHEDULED-TRANSFER-RSP-Create-401` | errorContract | statusCode=401, errorCode=UNAUTHORIZED; số lịch của tài khoản không đổi. |  |  | `RSP-Create-401` |
| `TORC-SCHEDULED-TRANSFER-RSP-Create-401-p4f809f` | errorContract | HTTP 401, errorCode=UNAUTHORIZED; không có bản ghi lịch mới. | Kiểm tra mã HTTP, errorCode và số bản ghi lịch. |  | `RSP-Create-401` |
| `TORC-SCHEDULED-TRANSFER-RSP-Create-401-validation` | schema | HTTP 401 với errorCode UNAUTHORIZED; không có bản ghi lịch mới. |  |  | `RSP-Create-401` |
| `TORC-SCHEDULED-TRANSFER-RSP-Create-403` | errorContract | statusCode=403, errorCode=FORBIDDEN; không có lịch mới cho tài khoản B. |  |  | `RSP-Create-403` |
| `TORC-SCHEDULED-TRANSFER-RSP-Create-403-pfae77d` | errorContract | HTTP 403 FORBIDDEN cho sai chủ tài khoản; HTTP 401 UNAUTHORIZED cho API key thiếu/sai. | So sánh mã HTTP và errorCode; đếm bản ghi lịch. |  | `RSP-Create-403` |
| `TORC-SCHEDULED-TRANSFER-RSP-Create-404` | errorContract | statusCode=404, errorCode=ACCOUNT_NOT_FOUND; không có lịch mới. |  |  | `RSP-Create-404` |
| `TORC-SCHEDULED-TRANSFER-RSP-Create-409` | errorContract | statusCode=409, errorCode=IDEMPOTENCY_CONFLICT; lịch gốc giữ nguyên, tổng số lịch không tăng. |  |  | `RSP-Create-409` |
| `TORC-SCHEDULED-TRANSFER-RSP-Create-409-idem-conflict` | errorContract | HTTP 409 với errorCode IDEMPOTENCY_CONFLICT; số lịch không đổi. |  |  | `RSP-Create-409` |
| `TORC-SCHEDULED-TRANSFER-RSP-Create-409-pccb8db` | errorContract | HTTP 409, errorCode=IDEMPOTENCY_CONFLICT; số lịch không đổi. | Kiểm tra mã HTTP, errorCode và đếm bản ghi lịch. |  | `RSP-Create-409` |
| `TORC-SCHEDULED-TRANSFER-RSP-Create-409-pd36026` | errorContract | Cùng nội dung: HTTP 200, cùng mã lịch, tổng số lịch không đổi. Khác nội dung: HTTP 409, IDEMPOTENCY_CONFLICT. | So sánh mã HTTP, mã lịch và đếm bản ghi. |  | `RSP-Create-409` |
| `TORC-SCHEDULED-TRANSFER-RSP-Create-422-amount` | errorContract | statusCode=422, errorCode=LIMIT_EXCEEDED; không tạo lịch. |  |  | `RSP-Create-422-amount` |
| `TORC-SCHEDULED-TRANSFER-RSP-Create-422-amount-pc6e1a6` | errorContract | HTTP 422 với errorCode LIMIT_EXCEEDED hoặc SAME_ACCOUNT tương ứng; không có bản ghi lịch mới. | Kiểm tra mã HTTP, errorCode và số bản ghi lịch. |  | `RSP-Create-422-amount` |
| `TORC-SCHEDULED-TRANSFER-RSP-Create-422-amount-validation` | schema | 50000001 -> HTTP 422 LIMIT_EXCEEDED, không tạo lịch; 50000000 -> HTTP 201. |  |  | `RSP-Create-422-amount` |
| `TORC-SCHEDULED-TRANSFER-RSP-Create-422-limit` | errorContract | statusCode=422, errorCode=SCHEDULE_LIMIT_REACHED; số lịch ACTIVE/PAUSED vẫn bằng 20. |  |  | `RSP-Create-422-limit` |
| `TORC-SCHEDULED-TRANSFER-RSP-Create-422-limit-pcbab3f` | errorContract | HTTP 422, errorCode=SCHEDULE_LIMIT_REACHED; với 19 lịch thì tạo thành công. | So sánh mã HTTP, errorCode và đếm lịch. |  | `RSP-Create-422-limit` |
| `TORC-SCHEDULED-TRANSFER-RSP-Create-422-same` | errorContract | statusCode=422, errorCode=SAME_ACCOUNT; không có lịch mới. |  |  | `RSP-Create-422-same` |
| `TORC-SCHEDULED-TRANSFER-RSP-Create-422-same-p588a23` | errorContract | HTTP 422 SAME_ACCOUNT; không có bản ghi lịch mới. |  |  | `RSP-Create-422-same` |
| `TORC-SCHEDULED-TRANSFER-RSP-Create-422-time` | errorContract | statusCode=422, errorCode=SCHEDULE_TIME_INVALID; không có lịch mới. |  |  | `RSP-Create-422-time` |
| `TORC-SCHEDULED-TRANSFER-RSP-Create-422-time-p241776` | errorContract | HTTP 422, errorCode=SCHEDULE_TIME_INVALID; kiểm tra biên 366 và 367 ngày. | So sánh mã HTTP và errorCode. |  | `RSP-Create-422-time` |
| `TORC-SCHEDULED-TRANSFER-RSP-Get-200` | postcondition | HTTP 200, body chứa status và nextRunAt khớp với dữ liệu lịch đã lưu. |  |  | `RSP-Get-200` |
| `TORC-SCHEDULED-TRANSFER-RSP-Get-401` | errorContract | HTTP 401, errorCode = UNAUTHORIZED, không lộ dữ liệu lịch. |  |  | `RSP-Get-401` |
| `TORC-SCHEDULED-TRANSFER-RSP-Get-401-p2aa8d6` | errorContract | HTTP 401 với errorCode UNAUTHORIZED cho cả ba endpoint. |  |  | `RSP-Get-401` |
| `TORC-SCHEDULED-TRANSFER-RSP-Get-401-validation` | schema | HTTP 401, body có errorCode = UNAUTHORIZED, không chứa status hay nextRunAt. |  |  | `RSP-Get-401` |
| `TORC-SCHEDULED-TRANSFER-RSP-Get-403` | errorContract | HTTP 403, errorCode = FORBIDDEN, không chứa status/nextRunAt. |  |  | `RSP-Get-403` |
| `TORC-SCHEDULED-TRANSFER-RSP-Get-404` | errorContract | HTTP 404, errorCode = SCHEDULE_NOT_FOUND. |  |  | `RSP-Get-404` |
| `TORC-SCHEDULED-TRANSFER-RSP-ListRuns-200` | postcondition | HTTP 200; danh sách giảm dần theo scheduledAt; mỗi phần tử có scheduledAt, status, transferId, reasonCode |  |  | `RSP-ListRuns-200` |
| `TORC-SCHEDULED-TRANSFER-RSP-ListRuns-401` | errorContract | HTTP 401, errorCode UNAUTHORIZED |  |  | `RSP-ListRuns-401` |
| `TORC-SCHEDULED-TRANSFER-RSP-ListRuns-401-validation` | schema | HTTP 401, errorCode UNAUTHORIZED |  |  | `RSP-ListRuns-401` |
| `TORC-SCHEDULED-TRANSFER-RSP-ListRuns-403` | errorContract | HTTP 403, errorCode FORBIDDEN |  |  | `RSP-ListRuns-403` |
| `TORC-SCHEDULED-TRANSFER-RSP-ListRuns-404` | errorContract | HTTP 404, errorCode SCHEDULE_NOT_FOUND |  |  | `RSP-ListRuns-404` |
| `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-200` | postcondition | HTTP 200; mọi phần tử thuộc accountId và có status=ACTIVE; số phần tử <= 2; bắt đầu từ vị trí offset=1 của tập đã lọc. |  |  | `RSP-ListSchedules-200` |
| `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-400` | errorContract | HTTP 400, errorCode INVALID_REQUEST. |  |  | `RSP-ListSchedules-400` |
| `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-400-p473406` | errorContract | HTTP 400 với errorCode INVALID_REQUEST. |  |  | `RSP-ListSchedules-400` |
| `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-400-validation` | schema | Giá trị trong enum: HTTP 200. Giá trị ngoài enum: HTTP 400, errorCode INVALID_REQUEST. |  |  | `RSP-ListSchedules-400` |
| `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-401` | errorContract | HTTP 401, errorCode UNAUTHORIZED. |  |  | `RSP-ListSchedules-401` |
| `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-401-validation` | schema | HTTP 401, errorCode UNAUTHORIZED, body không chứa phần tử lịch. |  |  | `RSP-ListSchedules-401` |
| `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-403` | errorContract | HTTP 403, errorCode FORBIDDEN, body không chứa lịch của B. |  |  | `RSP-ListSchedules-403` |
| `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-403-p72e502` | errorContract | HTTP 403 với errorCode FORBIDDEN. |  |  | `RSP-ListSchedules-403` |
| `TORC-SCHEDULED-TRANSFER-RSP-Patch-200` | postcondition | HTTP 200, body lịch có status = giá trị yêu cầu; sau PAUSED không phát sinh lần chạy mới; sau ACTIVE các lần bỏ lỡ là SKIPPED và không chạy bù. |  |  | `RSP-Patch-200` |
| `TORC-SCHEDULED-TRANSFER-RSP-Patch-400` | errorContract | HTTP 400, errorCode INVALID_REQUEST. |  |  | `RSP-Patch-400` |
| `TORC-SCHEDULED-TRANSFER-RSP-Patch-400-p847ccb` | errorContract | HTTP 400, errorCode=INVALID_REQUEST, trạng thái lịch không đổi. |  |  | `RSP-Patch-400` |
| `TORC-SCHEDULED-TRANSFER-RSP-Patch-400-validation` | schema | HTTP 400, errorCode INVALID_REQUEST, lịch giữ nguyên trạng thái. |  |  | `RSP-Patch-400` |
| `TORC-SCHEDULED-TRANSFER-RSP-Patch-401` | errorContract | HTTP 401, errorCode UNAUTHORIZED. |  |  | `RSP-Patch-401` |
| `TORC-SCHEDULED-TRANSFER-RSP-Patch-401-pf4b5f6` | errorContract | HTTP 401, errorCode=UNAUTHORIZED, trạng thái lịch không đổi. |  |  | `RSP-Patch-401` |
| `TORC-SCHEDULED-TRANSFER-RSP-Patch-401-validation` | schema | HTTP 401, errorCode UNAUTHORIZED, trạng thái lịch giữ nguyên. |  |  | `RSP-Patch-401` |
| `TORC-SCHEDULED-TRANSFER-RSP-Patch-403` | errorContract | HTTP 403, errorCode FORBIDDEN, lịch giữ nguyên. |  |  | `RSP-Patch-403` |
| `TORC-SCHEDULED-TRANSFER-RSP-Patch-403-p3b5af1` | errorContract | HTTP 403, errorCode=FORBIDDEN, trạng thái lịch không đổi. |  |  | `RSP-Patch-403` |
| `TORC-SCHEDULED-TRANSFER-RSP-Patch-403-pf87b1a` | errorContract | Phản hồi 403 FORBIDDEN và 409 SCHEDULE_STATE_INVALID; số bản ghi audit sự kiện lịch không tăng. | Đếm AuditLog theo mã lịch trước và sau. |  | `RSP-Patch-403` |
| `TORC-SCHEDULED-TRANSFER-RSP-Patch-404` | errorContract | HTTP 404, errorCode SCHEDULE_NOT_FOUND. |  |  | `RSP-Patch-404` |
| `TORC-SCHEDULED-TRANSFER-RSP-Patch-404-pb66379` | errorContract | HTTP 404, errorCode=SCHEDULE_NOT_FOUND. |  |  | `RSP-Patch-404` |
| `TORC-SCHEDULED-TRANSFER-RSP-Patch-409` | errorContract | HTTP 409, errorCode SCHEDULE_STATE_INVALID. |  |  | `RSP-Patch-409` |
| `TORC-SCHEDULED-TRANSFER-SCN-Notify-pd600b3` | postcondition | Số bản ghi sms_log cho một lần chạy và một loại sự kiện bằng 1 sau khi giao lại. | Đếm bản ghi sms_log sau khi giao lại sự kiện trên ENV-Demo. | 0 SMS trùng. | `SCN-Notify` |
| `TORC-SCHEDULED-TRANSFER-SCN-ScheduledRun-budget` | threshold | ≤ 60000 ms | Trường hợp xấu nhất, đo từ bước đầu tới bước cuối của một lần xử lý; Fraud Check tính 2 lần × 1000 ms; không cộng p95. Khoảng cách thử lại 5 phút giữa các lần xử lý không tính (chờ xác nhận). |  | `SCN-ScheduledRun` |
| `TORC-SCHEDULED-TRANSFER-SCN-ScheduledRun-main` | postcondition | 1 giao dịch COMPLETED, run SUCCEEDED có transferId, 1 bản ghi audit tác nhân SCHEDULER, 1 sự kiện ScheduledRunCompleted trong outbox; giao dịch tính vào hạn mức ngày và hiện trong lịch sử. |  |  | `SCN-ScheduledRun` |
| `TORC-SCHEDULED-TRANSFER-SCN-ScheduledRun-worst-INT-01` | threshold | 2000 ms ≤ 60000 ms (ngân sách của SCN-ScheduledRun) | Cộng trường hợp xấu nhất: timeout mỗi lần (1000 ms) × số lần (2) + backoff (none = 0 ms); không dùng p95. |  | `SCN-ScheduledRun` |
| `TORC-SCHEDULED-TRANSFER-TRN-Schedule-1` | stateMachine | ACTIVE → PAUSED; có đúng 1 audit SCHEDULE_PAUSED; lần quét 30 giây kế tiếp không chiếm lần chạy nào của lịch này. |  |  | `TRN-Schedule-1` |
| `TORC-SCHEDULED-TRANSFER-TRN-Schedule-1-guard` | stateMachine | Trạng thái vẫn ACTIVE, không có audit SCHEDULE_PAUSED mới. Mã lỗi chưa xác định trong nguồn. |  |  | `TRN-Schedule-1` |
| `TORC-SCHEDULED-TRANSFER-TRN-Schedule-2` | stateMachine | PAUSED → ACTIVE; lần bỏ lỡ = SKIPPED; có 1 lần chạy kế tiếp mới; consecutive_failures = 0; audit SCHEDULE_RESUMED. |  |  | `TRN-Schedule-2` |
| `TORC-SCHEDULED-TRANSFER-TRN-Schedule-2-guard` | stateMachine | Không có trạng thái ACTIVE, không có audit SCHEDULE_RESUMED, không có lần chạy mới. Kết quả cuối xem TRN-Schedule-7. |  |  | `TRN-Schedule-2` |
| `TORC-SCHEDULED-TRANSFER-TRN-Schedule-3` | stateMachine | ACTIVE → PAUSED; audit SCHEDULE_AUTO_PAUSED có actor=SCHEDULER; không có lần chạy PENDING mới được tạo. |  |  | `TRN-Schedule-3` |
| `TORC-SCHEDULED-TRANSFER-TRN-Schedule-3-guard` | stateMachine | Trạng thái vẫn ACTIVE, không có audit SCHEDULE_AUTO_PAUSED. |  |  | `TRN-Schedule-3` |
| `TORC-SCHEDULED-TRANSFER-TRN-Schedule-4` | stateMachine | ACTIVE → COMPLETED; không có lần chạy mới nào được tạo. |  |  | `TRN-Schedule-4` |
| `TORC-SCHEDULED-TRANSFER-TRN-Schedule-4-guard` | stateMachine | Trạng thái vẫn ACTIVE, không chuyển COMPLETED. |  |  | `TRN-Schedule-4` |
| `TORC-SCHEDULED-TRANSFER-TRN-Schedule-5` | stateMachine | ACTIVE → CANCELLED; không còn lần PENDING; có 1 audit SCHEDULE_CANCELLED. |  |  | `TRN-Schedule-5` |
| `TORC-SCHEDULED-TRANSFER-TRN-Schedule-5-guard` | stateMachine | Trạng thái vẫn ACTIVE, lần PENDING giữ nguyên, không có audit SCHEDULE_CANCELLED. Mã lỗi chưa xác định trong nguồn. |  |  | `TRN-Schedule-5` |
| `TORC-SCHEDULED-TRANSFER-TRN-Schedule-6` | stateMachine | PAUSED → CANCELLED; không còn lần PENDING; có 1 audit SCHEDULE_CANCELLED. |  |  | `TRN-Schedule-6` |
| `TORC-SCHEDULED-TRANSFER-TRN-Schedule-6-guard` | stateMachine | Trạng thái vẫn PAUSED, không có audit SCHEDULE_CANCELLED. Mã lỗi chưa xác định trong nguồn. |  |  | `TRN-Schedule-6` |
| `TORC-SCHEDULED-TRANSFER-TRN-Schedule-7` | stateMachine | PAUSED → COMPLETED; các lần còn lại = SKIPPED; không có lần chạy mới. |  |  | `TRN-Schedule-7` |
| `TORC-SCHEDULED-TRANSFER-TRN-Schedule-7-guard` | stateMachine | Không chuyển COMPLETED. Trạng thái cuối do TRN-Schedule-2 quyết định; nguồn mẫu 'giữ PAUSED' mâu thuẫn, cần làm rõ. |  |  | `TRN-Schedule-7` |
| `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-1` | stateMachine | PENDING → RUNNING; attempts = giá trị cũ + 1; correlationId khác rỗng; 1 lời gọi lõi chuyển tiền. | Đọc bản ghi ScheduledRun và log lời gọi lõi sau khi chiếm. |  | `TRN-ScheduledRun-1` |
| `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-1-guard` | stateMachine | status=PENDING, attempts không đổi, không có lời gọi lõi chuyển tiền. | Đọc bản ghi ScheduledRun và log lời gọi lõi sau chu kỳ quét. |  | `TRN-ScheduledRun-1` |
| `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-1-p4b41b3` | invariant | Đúng 1 bên rowcount=1; attempts = cũ + 1; đúng 1 lời gọi lõi. | Chạy song song 2 tiến trình trên cùng bản ghi, lặp nhiều lần; đếm lời gọi lõi. |  | `TRN-ScheduledRun-1` |
| `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-2` | stateMachine | RUNNING → SUCCEEDED; transferId đã lưu; consecutive_failures=0; 1 audit; 1 sự kiện ScheduledRunCompleted; lần kế tiếp được sinh hoặc lịch COMPLETED. | Đọc ScheduledRun, bảng audit và outbox/sự kiện sau khi xử lý. |  | `TRN-ScheduledRun-2` |
| `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-3` | stateMachine | RUNNING → PENDING; next_attempt_at = now + 5 phút hoặc now + 1 giờ. | So sánh next_attempt_at với thời điểm xử lý lỗi, mỗi mã lỗi một ca. | Sai lệch so với giờ hệ thống lúc xử lý tối đa 1 giây (giả định, chờ xác nhận). | `TRN-ScheduledRun-3` |
| `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-3-guard` | stateMachine | Không có chuyển RUNNING → PENDING; next_attempt_at không đổi; kết thúc ở FAILED theo TRN-ScheduledRun-4. | Đọc ScheduledRun sau xử lý lỗi, với từng mã lỗi ở đúng biên giới hạn. |  | `TRN-ScheduledRun-3` |
| `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-4` | stateMachine | RUNNING → FAILED; reasonCode = mã lỗi từ lõi; consecutive_failures = cũ + 1; 1 audit; 1 sự kiện ScheduledRunFailed. | Đọc ScheduledRun, bảng audit và outbox sau xử lý. |  | `TRN-ScheduledRun-4` |
| `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-4-guard` | stateMachine | status không phải FAILED; consecutive_failures không đổi; không có sự kiện ScheduledRunFailed. | Đọc ScheduledRun và outbox sau xử lý lỗi. |  | `TRN-ScheduledRun-4` |
| `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-5` | stateMachine | PENDING → SKIPPED; không có lời gọi lõi chuyển tiền và không có lần chạy bù mới. | Đọc ScheduledRun, log lời gọi lõi và số lần chạy của lịch sau khi tiếp tục. |  | `TRN-ScheduledRun-5` |
| `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-5-guard` | stateMachine | status=PENDING; không có chuyển sang SKIPPED. | Đọc ScheduledRun ngay sau khi gọi tiếp tục lịch. |  | `TRN-ScheduledRun-5` |
| `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-6` | stateMachine | PENDING → MISSED; 1 sự kiện ScheduledRunFailed có reasonCode=MISSED; 1 audit; consecutive_failures không đổi. | Đọc ScheduledRun, audit, outbox và consecutive_failures trước/sau chu kỳ quét. |  | `TRN-ScheduledRun-6` |
| `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-6-guard` | stateMachine | status=PENDING; không có sự kiện MISSED. | Đọc ScheduledRun và outbox sau chu kỳ quét; thử tại 23h59m và đúng 24h00m. |  | `TRN-ScheduledRun-6` |
| `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-7` | stateMachine | PENDING → CANCELLED; chu kỳ quét sau đó không chiếm lần chạy này. | Đọc ScheduledRun sau DELETE và sau một chu kỳ quét kế tiếp (30 giây). |  | `TRN-ScheduledRun-7` |
| `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-8` | stateMachine | RUNNING → PENDING; attempts bằng giá trị trước phục hồi. | Đọc ScheduledRun trước/sau phục hồi; lease 120 giây là đề xuất Q-S2, chưa chốt. |  | `TRN-ScheduledRun-8` |
| `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-8-guard` | stateMachine | status=RUNNING; attempts không đổi. | Đọc ScheduledRun sau tiến trình phục hồi, thử tại 119 giây và đúng 120 giây. |  | `TRN-ScheduledRun-8` |
| `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-8-p456351` | postcondition | Số giao dịch của lần chạy = 1; transferId trùng giao dịch ban đầu; status=SUCCEEDED. | Đếm giao dịch theo khóa idempotency sau chiếm lại. |  | `TRN-ScheduledRun-8` |

## 4. Lớp tương đương và biên

| Mã | Tham số | Loại | Giá trị |
| --- | --- | --- | --- |
| `TPRT-SCHEDULED-TRANSFER-FLD-Cancel-Authorization-missing` | Authorization (header) | missing | <thiếu> (không hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-Cancel-scheduleId-missing` | scheduleId (path) | missing | <thiếu> (không hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-Create-Authorization-missing` | Authorization (header) | missing | <thiếu> (không hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-Create-IdempotencyKey-missing` | Idempotency-Key (header) | missing | <thiếu> (không hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-Create-amount-max` | amount (body) | boundary | 50000000 (hợp lệ); 50000001 (không hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-Create-amount-min` | amount (body) | boundary | 0 (không hợp lệ); 1 (hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-Create-amount-missing` | amount (body) | missing | <thiếu> (không hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-Create-endDate-missing` | endDate (body) | missing | <thiếu> (không hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-Create-firstRunAt-missing` | firstRunAt (body) | missing | <thiếu> (không hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-Create-fromAccountId-missing` | fromAccountId (body) | missing | <thiếu> (không hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-Create-maxRuns-max` | maxRuns (body) | boundary | 60 (hợp lệ); 61 (không hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-Create-maxRuns-min` | maxRuns (body) | boundary | 0 (không hợp lệ); 1 (hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-Create-maxRuns-missing` | maxRuns (body) | missing | <thiếu> (không hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-Create-recurrence-enum` | recurrence (body) | enum | ONCE (hợp lệ); DAILY (hợp lệ); WEEKLY (hợp lệ); MONTHLY (hợp lệ); YEARLY (không hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-Create-recurrence-missing` | recurrence (body) | missing | <thiếu> (không hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-Create-toAccountId-missing` | toAccountId (body) | missing | <thiếu> (không hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-Get-Authorization-missing` | Authorization (header) | missing | <thiếu> (không hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-Get-scheduleId-missing` | scheduleId (path) | missing | <thiếu> (không hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-ListRuns-Authorization-missing` | Authorization (header) | missing | <thiếu> (không hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-ListRuns-limit-max` | limit (query) | boundary | 100 (hợp lệ); 101 (không hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-ListRuns-limit-min` | limit (query) | boundary | 0 (không hợp lệ); 1 (hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-ListRuns-offset-min` | offset (query) | boundary | -1 (không hợp lệ); 0 (hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-ListRuns-scheduleId-missing` | scheduleId (path) | missing | <thiếu> (không hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-ListSchedules-Authorization-missing` | Authorization (header) | missing | <thiếu header Authorization> (không hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-ListSchedules-accountId-missing` | accountId (path) | missing | <thiếu accountId> (không hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-ListSchedules-limit-max` | limit (query) | boundary | 100 (hợp lệ); 101 (không hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-ListSchedules-limit-min` | limit (query) | boundary | 0 (không hợp lệ); 1 (hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-ListSchedules-offset-min` | offset (query) | boundary | -1 (không hợp lệ); 0 (hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-ListSchedules-status-enum` | status (query) | enum | ACTIVE (hợp lệ); PAUSED (hợp lệ); COMPLETED (hợp lệ); CANCELLED (hợp lệ); UNKNOWN (không hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-Patch-Authorization-missing` | Authorization (header) | missing | <thiếu> (không hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-Patch-scheduleId-missing` | scheduleId (path) | missing | <thiếu> (không hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-Patch-status-enum` | status (body) | enum | PAUSED (hợp lệ); ACTIVE (hợp lệ); <ngoài tập> (không hợp lệ) |
| `TPRT-SCHEDULED-TRANSFER-FLD-Patch-status-missing` | status (body) | missing | <thiếu> (không hợp lệ) |

## 5. Mô hình lỗi và tiêm lỗi

| Mã | Loại lỗi | Điểm tiêm | Thời điểm | Xử lý mong đợi | Cách tiêm |
| --- | --- | --- | --- | --- | --- |
| `TFM-SCHEDULED-TRANSFER-MECH-RunClaim-crash-mid` | processCrash | MECH-RunClaim | afterCommit | Sau khởi động lại không tạo giao dịch thứ hai cho cùng lần chạy; việc chuyển tiền lặp lại (nếu có) dùng cùng Idempotency-Key nên không sinh hiệu ứng trùng. Cơ chế nhận lại lần chạy đã chiếm còn là khoảng trống. | Dừng cưỡng bức tiến trình scheduler bằng SIGKILL tại điểm tiêm, sau đó khởi động lại tiến trình. — Tại MECH-RunClaim, ngay sau khi giao dịch đã commit và trước khi trả kết quả. |
| `TFM-SCHEDULED-TRANSFER-MOD-Scheduler-dep-MOD-Account` | dependencyFailure | MOD-Account | during | Chưa có quy tắc xử lý lỗi hạ tầng của Quản lý tài khoản; cần bổ sung trước khi viết kết quả mong đợi cụ thể. | Stub MOD-Account trả lỗi hoặc không phản hồi (timeout). — Kích hoạt khi Scheduler đang gọi MOD-Account trong lúc thao tác chính chạy. |
| `TFM-SCHEDULED-TRANSFER-MOD-Scheduler-dep-MOD-Audit` | dependencyFailure | MOD-Audit | during | Chưa có quy tắc cho lỗi Audit (fail-open hay fail-closed); cần bổ sung. | Stub MOD-Audit trả lỗi hoặc không phản hồi (timeout). — Kích hoạt khi Scheduler gọi MOD-Audit trong lúc thao tác chính chạy. |
| `TFM-SCHEDULED-TRANSFER-MOD-Scheduler-dep-MOD-Risk` | dependencyFailure | MOD-Risk | during | Timeout và 5xx: retry 1 lần cùng transactionId rồi áp FB-INT01. 4xx và sai schema: không retry, coi là không khả dụng, ghi cảnh báo vi phạm contract. | Mock Fraud Check trả timeout, HTTP 5xx, HTTP 4xx hoặc HTTP 200 sai schema. — Kích hoạt khi Scheduler gọi lõi chuyển tiền và lõi gọi MOD-Risk cho run đến hạn. |
| `TFM-SCHEDULED-TRANSFER-MOD-Scheduler-dep-MOD-Transfer` | dependencyFailure | MOD-Transfer | during | Lỗi nghiệp vụ xử lý theo bảng BR-13 (FR-046). Lỗi hạ tầng hoặc không phản hồi của MOD-Transfer chưa có quy tắc. | Stub MOD-Transfer trả lỗi nghiệp vụ hoặc không phản hồi. — Kích hoạt khi Scheduler gọi MOD-Transfer trong lúc thao tác chính chạy. |

## 6. Stub

| Mã | Đích | Hành vi | HTTP | Trễ (ms) |
| --- | --- | --- | --- | --- |
| `TSTUB-SCHEDULED-TRANSFER-INT-01-success` | INT-01 | success |  |  |
| `TSTUB-SCHEDULED-TRANSFER-MOD-Account-errorTransient` | MOD-Account | errorTransient |  |  |
| `TSTUB-SCHEDULED-TRANSFER-MOD-Account-success` | MOD-Account | success |  |  |
| `TSTUB-SCHEDULED-TRANSFER-MOD-Audit-errorTransient` | MOD-Audit | errorTransient |  |  |
| `TSTUB-SCHEDULED-TRANSFER-MOD-Audit-success` | MOD-Audit | success |  |  |
| `TSTUB-SCHEDULED-TRANSFER-MOD-Risk-errorTransient-200` | MOD-Risk | errorTransient | 200 |  |
| `TSTUB-SCHEDULED-TRANSFER-MOD-Risk-success` | MOD-Risk | success |  |  |
| `TSTUB-SCHEDULED-TRANSFER-MOD-Transfer-errorTransient` | MOD-Transfer | errorTransient |  |  |
| `TSTUB-SCHEDULED-TRANSFER-MOD-Transfer-success` | MOD-Transfer | success |  |  |

## 7. Đường đi kịch bản

| Mã | Loại | Các bước |
| --- | --- | --- |
| `TPATH-SCHEDULED-TRANSFER-SCN-ScheduledRun-fail-INT-01` | failure | Quét mỗi 30 giây → chọn tối đa 20 lần chạy đến hạn → chiếm lần chạy (UPDATE có điều kiện) → sinh correlationId → lõi chuyển tiền (idempotency, hạn mức/số dư, Fraud Check, ghi sổ + outbox) → cập nhật lần chạy → thử lại/FAILED → Fraud Check lỗi |
| `TPATH-SCHEDULED-TRANSFER-SCN-ScheduledRun-main` | main | Quét mỗi 30 giây → Chọn tối đa 20 lần chạy đến hạn → Chiếm lần chạy bằng UPDATE có điều kiện → Sinh correlationId riêng → Lõi chuyển tiền: idempotency, hạn mức/số dư, Fraud Check, ghi sổ + outbox → Cập nhật run SUCCEEDED kèm transferId → Ghi audit tác nhân SCHEDULER → Phát ScheduledRunCompleted qua outbox |

## 8. Tập test

| Thứ tự | Mã | Mục đích | Môi trường | Điều kiện vào | Điều kiện ra | Số TC |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | `TSET-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun` | Các test case mức L1 của SCN-ScheduledRun | `TENV-SCHEDULED-TRANSFER-L1` | Môi trường tích hợp đầu-cuối sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 4 |
| 2 | `TSET-SCHEDULED-TRANSFER-L2-EP-CancelSchedule` | Các test case mức L2 của EP-CancelSchedule | `TENV-SCHEDULED-TRANSFER-L2` | Môi trường service sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 11 |
| 3 | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | Các test case mức L2 của EP-CreateSchedule | `TENV-SCHEDULED-TRANSFER-L2` | Môi trường service sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 49 |
| 4 | `TSET-SCHEDULED-TRANSFER-L2-EP-GetSchedule` | Các test case mức L2 của EP-GetSchedule | `TENV-SCHEDULED-TRANSFER-L2` | Môi trường service sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 7 |
| 5 | `TSET-SCHEDULED-TRANSFER-L2-EP-ListRuns` | Các test case mức L2 của EP-ListRuns | `TENV-SCHEDULED-TRANSFER-L2` | Môi trường service sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 12 |
| 6 | `TSET-SCHEDULED-TRANSFER-L2-EP-ListSchedules` | Các test case mức L2 của EP-ListSchedules | `TENV-SCHEDULED-TRANSFER-L2` | Môi trường service sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 18 |
| 7 | `TSET-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus` | Các test case mức L2 của EP-UpdateScheduleStatus | `TENV-SCHEDULED-TRANSFER-L2` | Môi trường service sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 14 |
| 8 | `TSET-SCHEDULED-TRANSFER-L2-FR-040` | Các test case mức L2 của FR-040 | `TENV-SCHEDULED-TRANSFER-L2` | Môi trường service sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 13 |
| 9 | `TSET-SCHEDULED-TRANSFER-L2-FR-041` | Các test case mức L2 của FR-041 | `TENV-SCHEDULED-TRANSFER-L2` | Môi trường service sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 10 |
| 10 | `TSET-SCHEDULED-TRANSFER-L2-FR-042` | Các test case mức L2 của FR-042 | `TENV-SCHEDULED-TRANSFER-L2` | Môi trường service sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 8 |
| 11 | `TSET-SCHEDULED-TRANSFER-L2-FR-043` | Các test case mức L2 của FR-043 | `TENV-SCHEDULED-TRANSFER-L2` | Môi trường service sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 8 |
| 12 | `TSET-SCHEDULED-TRANSFER-L2-FR-044` | Các test case mức L2 của FR-044 | `TENV-SCHEDULED-TRANSFER-L2` | Môi trường service sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 9 |
| 13 | `TSET-SCHEDULED-TRANSFER-L2-FR-045` | Các test case mức L2 của FR-045 | `TENV-SCHEDULED-TRANSFER-L2` | Môi trường service sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 7 |
| 14 | `TSET-SCHEDULED-TRANSFER-L2-FR-046` | Các test case mức L2 của FR-046 | `TENV-SCHEDULED-TRANSFER-L2` | Môi trường service sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 4 |
| 15 | `TSET-SCHEDULED-TRANSFER-L2-FR-047` | Các test case mức L2 của FR-047 | `TENV-SCHEDULED-TRANSFER-L2` | Môi trường service sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 4 |
| 16 | `TSET-SCHEDULED-TRANSFER-L2-FR-048` | Các test case mức L2 của FR-048 | `TENV-SCHEDULED-TRANSFER-L2` | Môi trường service sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 5 |
| 17 | `TSET-SCHEDULED-TRANSFER-L2-FR-049` | Các test case mức L2 của FR-049 | `TENV-SCHEDULED-TRANSFER-L2` | Môi trường service sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 3 |
| 18 | `TSET-SCHEDULED-TRANSFER-L2-NFR-010` | Các test case mức L2 của NFR-010 | `TENV-SCHEDULED-TRANSFER-L2` | Môi trường service sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 3 |
| 19 | `TSET-SCHEDULED-TRANSFER-L2-NFR-011` | Các test case mức L2 của NFR-011 | `TENV-SCHEDULED-TRANSFER-L2` | Môi trường service sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 3 |
| 20 | `TSET-SCHEDULED-TRANSFER-L2-NFR-012` | Các test case mức L2 của NFR-012 | `TENV-SCHEDULED-TRANSFER-L2` | Môi trường service sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 4 |
| 21 | `TSET-SCHEDULED-TRANSFER-L2-NFR-013` | Các test case mức L2 của NFR-013 | `TENV-SCHEDULED-TRANSFER-L2` | Môi trường service sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 5 |
| 22 | `TSET-SCHEDULED-TRANSFER-L2-NFR-014` | Các test case mức L2 của NFR-014 | `TENV-SCHEDULED-TRANSFER-L2` | Môi trường service sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 4 |
| 23 | `TSET-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun` | Các test case mức L2 của SCN-ScheduledRun | `TENV-SCHEDULED-TRANSFER-L2` | Môi trường service sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 7 |
| 24 | `TSET-SCHEDULED-TRANSFER-L2-SM-Schedule` | Các test case mức L2 của SM-Schedule | `TENV-SCHEDULED-TRANSFER-L2` | Môi trường service sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 15 |
| 25 | `TSET-SCHEDULED-TRANSFER-L2-SM-ScheduledRun` | Các test case mức L2 của SM-ScheduledRun | `TENV-SCHEDULED-TRANSFER-L2` | Môi trường service sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 17 |
| 26 | `TSET-SCHEDULED-TRANSFER-L3-MECH-CatchUp` | Các test case mức L3 của MECH-CatchUp | `TENV-SCHEDULED-TRANSFER-L3` | Môi trường service cô lập sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 2 |
| 27 | `TSET-SCHEDULED-TRANSFER-L3-MECH-RunClaim` | Các test case mức L3 của MECH-RunClaim | `TENV-SCHEDULED-TRANSFER-L3` | Môi trường service cô lập sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 2 |
| 28 | `TSET-SCHEDULED-TRANSFER-L3-MECH-ScheduleRetry` | Các test case mức L3 của MECH-ScheduleRetry | `TENV-SCHEDULED-TRANSFER-L3` | Môi trường service cô lập sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 1 |
| 29 | `TSET-SCHEDULED-TRANSFER-L3-MECH-SchedulerThrottle` | Các test case mức L3 của MECH-SchedulerThrottle | `TENV-SCHEDULED-TRANSFER-L3` | Môi trường service cô lập sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 2 |
| 30 | `TSET-SCHEDULED-TRANSFER-L3-MOD-Scheduler` | Các test case mức L3 của MOD-Scheduler | `TENV-SCHEDULED-TRANSFER-L3` | Môi trường service cô lập sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 6 |

## 9. Lịch sử phiên bản và phạm vi hồi quy

| Phiên bản | Trạng thái | CR | Thêm | Sửa | Bỏ | Số nút |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | Baselined | CR-001 | 3380 | 0 | 0 | 3380 |

Hồi quy (0 test case đã baseline, thuộc phần bị ảnh hưởng nhưng không đổi): không có

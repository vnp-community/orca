---
spec: TSD-SCHEDULED-TRANSFER-TestDesign
feature: SCHEDULED-TRANSFER
docKind: TestDesign
version: 1
status: Draft
supersedes: none
baseline: b-a31cdc541d67
changes: [CR-001]
contentHash: 01c6e8f1d6c74299e335c3c6ba2d9ca8cdc4b13fc47287af4f997efa28323b3b
---
# Test Design: SCHEDULED-TRANSFER

## 0. Thông tin tài liệu

| Trường | Giá trị |
| --- | --- |
| id | TSD-SCHEDULED-TRANSFER-TestDesign_v1 |
| Feature | SCHEDULED-TRANSFER |
| Phiên bản | 1 |
| Trạng thái | Draft |
| Baseline TDD | DOC-SRS-CR001 (b-a31cdc541d67) |
| CR của phiên bản này | CR-001 |

## 1. Cách tiếp cận theo mức

| Mức | Đối tượng | Thật | Stub | Công cụ |
| --- | --- | --- | --- | --- |
| L1 | Hành trình nghiệp vụ xuyên Module (Scenario) | Mọi Module của hệ thống | Chỉ hệ thống ngoài | pytest, httpx |
| L2 | Một feature qua API hoặc giao diện | Module của feature | Phụ thuộc ngoài (outbound Integration) | pytest, httpx |
| L3 | Một Module hoặc Integration | Chính Module | Mọi phụ thuộc và Integration outbound | pytest, httpx |

## 2. Thiết kế test

### TD-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-JRN

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-JRN |
| Phủ TR | TR-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun |
| Đối tượng thiết kế | Scenario SCN-ScheduledRun |
| Kỹ thuật | useCase |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-JRN |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-JRN-BUDGET | timeoutBoundary | SCN-ScheduledRun hoàn tất trong 60000 ms | responseBudgetMs | 60000 | có |
| TCOV-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-JRN-MAIN | scenarioStep | Luồng chính SCN-ScheduledRun | SCN-ScheduledRun | 8 | có |

### TD-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API |
| Phủ TR | TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule |
| Đối tượng thiết kế | Endpoint EP-CancelSchedule |
| Kỹ thuật | equivalencePartitioning |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API-RSP-Cancel-200 | partition | DELETE /v1/scheduled-transfers/{scheduleId} → 200: Hủy thành công (lịch CANCELLED, lần PENDING thành CANCELLED) hoặc lịch đã CANCELLED: trả kết quả cũ | response | 200 |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API-RSP-Cancel-401 | partition | DELETE /v1/scheduled-transfers/{scheduleId} → 401 UNAUTHORIZED: Thiếu hoặc sai API key | response | 401 UNAUTHORIZED |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API-RSP-Cancel-403 | partition | DELETE /v1/scheduled-transfers/{scheduleId} → 403 FORBIDDEN: Người gọi không phải chủ lịch | response | 403 FORBIDDEN |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API-RSP-Cancel-404 | partition | DELETE /v1/scheduled-transfers/{scheduleId} → 404 SCHEDULE_NOT_FOUND: Không có lịch với scheduleId | response | 404 SCHEDULE_NOT_FOUND |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API-RSP-Cancel-409 | partition | DELETE /v1/scheduled-transfers/{scheduleId} → 409 SCHEDULE_NOT_CANCELLABLE: Lịch đã COMPLETED hoặc có lần chạy đang RUNNING | response | 409 SCHEDULE_NOT_CANCELLABLE |  |

### TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Phủ TR | TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule |
| Đối tượng thiết kế | Endpoint EP-CreateSchedule |
| Kỹ thuật | equivalencePartitioning |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-amount-ABOVE-MAX | schemaField | POST /v1/scheduled-transfers: amount (body) lớn hơn lớn nhất (50000001) | amount (body) | 50000001 |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-amount-BELOW-MIN | schemaField | POST /v1/scheduled-transfers: amount (body) nhỏ hơn nhỏ nhất (0) | amount (body) | 0 |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-amount-MAX | schemaField | POST /v1/scheduled-transfers: amount (body) đúng giá trị lớn nhất 50000000 | amount (body) | 50000000 |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-amount-MIN | schemaField | POST /v1/scheduled-transfers: amount (body) đúng giá trị nhỏ nhất 1 | amount (body) | 1 |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-maxRuns-ABOVE-MAX | schemaField | POST /v1/scheduled-transfers: maxRuns (body) lớn hơn lớn nhất (61) | maxRuns (body) | 61 |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-maxRuns-BELOW-MIN | schemaField | POST /v1/scheduled-transfers: maxRuns (body) nhỏ hơn nhỏ nhất (0) | maxRuns (body) | 0 |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-maxRuns-MAX | schemaField | POST /v1/scheduled-transfers: maxRuns (body) đúng giá trị lớn nhất 60 | maxRuns (body) | 60 |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-maxRuns-MIN | schemaField | POST /v1/scheduled-transfers: maxRuns (body) đúng giá trị nhỏ nhất 1 | maxRuns (body) | 1 |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-DAILY | schemaField | POST /v1/scheduled-transfers: recurrence (body) = DAILY (giá trị hợp lệ) | recurrence (body) | DAILY |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-INVALID | schemaField | POST /v1/scheduled-transfers: recurrence (body) ngoài danh sách ONCE, DAILY, WEEKLY, MONTHLY | recurrence (body) | <ngoài danh sách> |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-MONTHLY | schemaField | POST /v1/scheduled-transfers: recurrence (body) = MONTHLY (giá trị hợp lệ) | recurrence (body) | MONTHLY |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-ONCE | schemaField | POST /v1/scheduled-transfers: recurrence (body) = ONCE (giá trị hợp lệ) | recurrence (body) | ONCE |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-WEEKLY | schemaField | POST /v1/scheduled-transfers: recurrence (body) = WEEKLY (giá trị hợp lệ) | recurrence (body) | WEEKLY |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-200 | partition | POST /v1/scheduled-transfers → 200: Cùng Idempotency-Key và cùng nội dung: trả lịch đã tạo | response | 200 |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-201 | partition | POST /v1/scheduled-transfers → 201: Tạo lịch thành công, status=ACTIVE, trả mã lịch | response | 201 |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-400 | partition | POST /v1/scheduled-transfers → 400 INVALID_REQUEST: Thiếu Idempotency-Key hoặc trường bắt buộc, amount không nguyên dương, recurrence sai, định kỳ thiếu cả endDate và maxRuns, maxRuns ngoài 1..60, firstRunAt sai định dạng | response | 400 INVALID_REQUEST |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-401 | partition | POST /v1/scheduled-transfers → 401 UNAUTHORIZED: Thiếu hoặc sai API key | response | 401 UNAUTHORIZED |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-403 | partition | POST /v1/scheduled-transfers → 403 FORBIDDEN: fromAccountId khác tài khoản của người gọi | response | 403 FORBIDDEN |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-404 | partition | POST /v1/scheduled-transfers → 404 ACCOUNT_NOT_FOUND: Tài khoản đích không tồn tại | response | 404 ACCOUNT_NOT_FOUND |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-409 | partition | POST /v1/scheduled-transfers → 409 IDEMPOTENCY_CONFLICT: Cùng Idempotency-Key nhưng nội dung khác | response | 409 IDEMPOTENCY_CONFLICT |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-422-amount | partition | POST /v1/scheduled-transfers → 422 LIMIT_EXCEEDED: amount > 50.000.000 VND | response | 422 LIMIT_EXCEEDED |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-422-limit | partition | POST /v1/scheduled-transfers → 422 SCHEDULE_LIMIT_REACHED: Tài khoản đã có 20 lịch ACTIVE hoặc PAUSED | response | 422 SCHEDULE_LIMIT_REACHED |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-422-same | partition | POST /v1/scheduled-transfers → 422 SAME_ACCOUNT: fromAccountId bằng toAccountId | response | 422 SAME_ACCOUNT |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-422-time | partition | POST /v1/scheduled-transfers → 422 SCHEDULE_TIME_INVALID: firstRunAt cách hiện tại < 5 phút hoặc > 90 ngày; endDate trước lần chạy đầu hoặc quá 366 ngày | response | 422 SCHEDULE_TIME_INVALID |  |

### TD-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API |
| Phủ TR | TR-SCHEDULED-TRANSFER-L2-EP-GetSchedule |
| Đối tượng thiết kế | Endpoint EP-GetSchedule |
| Kỹ thuật | equivalencePartitioning |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API-RSP-Get-200 | partition | GET /v1/scheduled-transfers/{scheduleId} → 200: Trả trạng thái lịch và lần chạy kế tiếp (nextRunAt) | response | 200 |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API-RSP-Get-401 | partition | GET /v1/scheduled-transfers/{scheduleId} → 401 UNAUTHORIZED: Thiếu hoặc sai API key | response | 401 UNAUTHORIZED |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API-RSP-Get-403 | partition | GET /v1/scheduled-transfers/{scheduleId} → 403 FORBIDDEN: Người gọi không phải chủ tài khoản nguồn của lịch | response | 403 FORBIDDEN |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API-RSP-Get-404 | partition | GET /v1/scheduled-transfers/{scheduleId} → 404 SCHEDULE_NOT_FOUND: Không có lịch với scheduleId | response | 404 SCHEDULE_NOT_FOUND |  |

### TD-SCHEDULED-TRANSFER-L2-EP-ListRuns-API

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L2-EP-ListRuns-API |
| Phủ TR | TR-SCHEDULED-TRANSFER-L2-EP-ListRuns |
| Đối tượng thiết kế | Endpoint EP-ListRuns |
| Kỹ thuật | equivalencePartitioning |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListRuns-API |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-FLD-ListRuns-limit-ABOVE-MAX | schemaField | GET /v1/scheduled-transfers/{scheduleId}/runs: limit (query) lớn hơn lớn nhất (101) | limit (query) | 101 |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-FLD-ListRuns-limit-BELOW-MIN | schemaField | GET /v1/scheduled-transfers/{scheduleId}/runs: limit (query) nhỏ hơn nhỏ nhất (0) | limit (query) | 0 |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-FLD-ListRuns-limit-MAX | schemaField | GET /v1/scheduled-transfers/{scheduleId}/runs: limit (query) đúng giá trị lớn nhất 100 | limit (query) | 100 |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-FLD-ListRuns-limit-MIN | schemaField | GET /v1/scheduled-transfers/{scheduleId}/runs: limit (query) đúng giá trị nhỏ nhất 1 | limit (query) | 1 |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-FLD-ListRuns-offset-BELOW-MIN | schemaField | GET /v1/scheduled-transfers/{scheduleId}/runs: offset (query) nhỏ hơn nhỏ nhất (-1) | offset (query) | -1 |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-FLD-ListRuns-offset-MIN | schemaField | GET /v1/scheduled-transfers/{scheduleId}/runs: offset (query) đúng giá trị nhỏ nhất 0 | offset (query) | 0 |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-RSP-ListRuns-200 | partition | GET /v1/scheduled-transfers/{scheduleId}/runs → 200: Trả các lần chạy, mới nhất trước, mỗi lần có thời điểm dự kiến, trạng thái, transferId, reasonCode | response | 200 |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-RSP-ListRuns-401 | partition | GET /v1/scheduled-transfers/{scheduleId}/runs → 401 UNAUTHORIZED: Thiếu hoặc sai API key | response | 401 UNAUTHORIZED |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-RSP-ListRuns-403 | partition | GET /v1/scheduled-transfers/{scheduleId}/runs → 403 FORBIDDEN: Người gọi không phải chủ lịch | response | 403 FORBIDDEN |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-RSP-ListRuns-404 | partition | GET /v1/scheduled-transfers/{scheduleId}/runs → 404 SCHEDULE_NOT_FOUND: Không có lịch với scheduleId | response | 404 SCHEDULE_NOT_FOUND |  |

### TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Phủ TR | TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules |
| Đối tượng thiết kế | Endpoint EP-ListSchedules |
| Kỹ thuật | equivalencePartitioning |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-limit-ABOVE-MAX | schemaField | GET /v1/accounts/{accountId}/scheduled-transfers: limit (query) lớn hơn lớn nhất (101) | limit (query) | 101 |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-limit-BELOW-MIN | schemaField | GET /v1/accounts/{accountId}/scheduled-transfers: limit (query) nhỏ hơn nhỏ nhất (0) | limit (query) | 0 |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-limit-MAX | schemaField | GET /v1/accounts/{accountId}/scheduled-transfers: limit (query) đúng giá trị lớn nhất 100 | limit (query) | 100 |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-limit-MIN | schemaField | GET /v1/accounts/{accountId}/scheduled-transfers: limit (query) đúng giá trị nhỏ nhất 1 | limit (query) | 1 |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-offset-BELOW-MIN | schemaField | GET /v1/accounts/{accountId}/scheduled-transfers: offset (query) nhỏ hơn nhỏ nhất (-1) | offset (query) | -1 |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-offset-MIN | schemaField | GET /v1/accounts/{accountId}/scheduled-transfers: offset (query) đúng giá trị nhỏ nhất 0 | offset (query) | 0 |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-ACTIVE | schemaField | GET /v1/accounts/{accountId}/scheduled-transfers: status (query) = ACTIVE (giá trị hợp lệ) | status (query) | ACTIVE |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-CANCELLED | schemaField | GET /v1/accounts/{accountId}/scheduled-transfers: status (query) = CANCELLED (giá trị hợp lệ) | status (query) | CANCELLED |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-COMPLETED | schemaField | GET /v1/accounts/{accountId}/scheduled-transfers: status (query) = COMPLETED (giá trị hợp lệ) | status (query) | COMPLETED |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-INVALID | schemaField | GET /v1/accounts/{accountId}/scheduled-transfers: status (query) ngoài danh sách ACTIVE, PAUSED, COMPLETED, CANCELLED | status (query) | <ngoài danh sách> |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-PAUSED | schemaField | GET /v1/accounts/{accountId}/scheduled-transfers: status (query) = PAUSED (giá trị hợp lệ) | status (query) | PAUSED |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-RSP-ListSchedules-200 | partition | GET /v1/accounts/{accountId}/scheduled-transfers → 200: Trả danh sách lịch của tài khoản, phân trang, lọc theo status | response | 200 |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-RSP-ListSchedules-400 | partition | GET /v1/accounts/{accountId}/scheduled-transfers → 400 INVALID_REQUEST: status ngoài enum | response | 400 INVALID_REQUEST |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-RSP-ListSchedules-401 | partition | GET /v1/accounts/{accountId}/scheduled-transfers → 401 UNAUTHORIZED: Thiếu hoặc sai API key | response | 401 UNAUTHORIZED |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-RSP-ListSchedules-403 | partition | GET /v1/accounts/{accountId}/scheduled-transfers → 403 FORBIDDEN: accountId khác tài khoản của người gọi | response | 403 FORBIDDEN |  |

### TD-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API |
| Phủ TR | TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus |
| Đối tượng thiết kế | Endpoint EP-UpdateScheduleStatus |
| Kỹ thuật | equivalencePartitioning |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-ACTIVE | schemaField | PATCH /v1/scheduled-transfers/{scheduleId}: status (body) = ACTIVE (giá trị hợp lệ) | status (body) | ACTIVE |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-INVALID | schemaField | PATCH /v1/scheduled-transfers/{scheduleId}: status (body) ngoài danh sách PAUSED, ACTIVE | status (body) | <ngoài danh sách> |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-PAUSED | schemaField | PATCH /v1/scheduled-transfers/{scheduleId}: status (body) = PAUSED (giá trị hợp lệ) | status (body) | PAUSED |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-RSP-Patch-200 | partition | PATCH /v1/scheduled-transfers/{scheduleId} → 200: Chuyển PAUSED hoặc ACTIVE thành công, trả lịch | response | 200 |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-RSP-Patch-400 | partition | PATCH /v1/scheduled-transfers/{scheduleId} → 400 INVALID_REQUEST: status thiếu hoặc ngoài PAUSED/ACTIVE | response | 400 INVALID_REQUEST |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-RSP-Patch-401 | partition | PATCH /v1/scheduled-transfers/{scheduleId} → 401 UNAUTHORIZED: Thiếu hoặc sai API key | response | 401 UNAUTHORIZED |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-RSP-Patch-403 | partition | PATCH /v1/scheduled-transfers/{scheduleId} → 403 FORBIDDEN: Người gọi không phải chủ lịch | response | 403 FORBIDDEN |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-RSP-Patch-404 | partition | PATCH /v1/scheduled-transfers/{scheduleId} → 404 SCHEDULE_NOT_FOUND: Không có lịch với scheduleId | response | 404 SCHEDULE_NOT_FOUND |  |
| TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-RSP-Patch-409 | partition | PATCH /v1/scheduled-transfers/{scheduleId} → 409 SCHEDULE_STATE_INVALID: Tạm dừng lịch không ACTIVE, tiếp tục lịch không PAUSED, hoặc lịch ONCE | response | 409 SCHEDULE_STATE_INVALID |  |

### TD-SCHEDULED-TRANSFER-L2-FR-040-AC

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L2-FR-040-AC |
| Phủ TR | TR-SCHEDULED-TRANSFER-L2-FR-040 |
| Đối tượng thiết kế | Requirement FR-040 |
| Kỹ thuật | useCase |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-040-AC |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L2-FR-040-AC-AC-FR-040-1 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-040-1 | AC-FR-040-1 | POST tạo lịch ONCE hợp lệ trả 201, có scheduleId và status ACTIVE | có |
| TCOV-SCHEDULED-TRANSFER-L2-FR-040-AC-AC-FR-040-2 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-040-2 | AC-FR-040-2 | Số dư hai tài khoản không đổi và không có bản ghi ledger_entries mới sau khi tạo lịch | có |
| TCOV-SCHEDULED-TRANSFER-L2-FR-040-AC-AC-FR-040-3 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-040-3 | AC-FR-040-3 | firstRunAt cách hiện tại < 5 phút hoặc > 90 ngày thì trả 422 SCHEDULE_TIME_INVALID | có |
| TCOV-SCHEDULED-TRANSFER-L2-FR-040-AC-AC-FR-040-4 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-040-4 | AC-FR-040-4 | toAccountId không tồn tại thì trả 404 ACCOUNT_NOT_FOUND | có |
| TCOV-SCHEDULED-TRANSFER-L2-FR-040-AC-AC-FR-040-5 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-040-5 | AC-FR-040-5 | fromAccountId khác chủ API key thì trả 403 FORBIDDEN | có |
| TCOV-SCHEDULED-TRANSFER-L2-FR-040-AC-AC-FR-040-6 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-040-6 | AC-FR-040-6 | Lịch thứ 21 ACTIVE/PAUSED của một tài khoản trả 422 SCHEDULE_LIMIT_REACHED | có |
| TCOV-SCHEDULED-TRANSFER-L2-FR-040-AC-AC-REV-003 | acceptanceCriterion | Tiêu chí chấp nhận AC-REV-003 | AC-REV-003 | Chỉ lịch ACTIVE hoặc PAUSED được tính vào giới hạn 20 lịch của tài khoản; lịch COMPLETED hoặc CANCELLED không được tính | có |

### TD-SCHEDULED-TRANSFER-L2-FR-041-AC

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L2-FR-041-AC |
| Phủ TR | TR-SCHEDULED-TRANSFER-L2-FR-041 |
| Đối tượng thiết kế | Requirement FR-041 |
| Kỹ thuật | useCase |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-041-AC |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L2-FR-041-AC-AC-FR-041-1 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-041-1 | AC-FR-041-1 | Lịch DAILY, WEEKLY hoặc MONTHLY có endDate hoặc maxRuns được tạo thành công (201) | có |
| TCOV-SCHEDULED-TRANSFER-L2-FR-041-AC-AC-FR-041-2 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-041-2 | AC-FR-041-2 | Lịch định kỳ thiếu cả endDate và maxRuns trả 400 INVALID_REQUEST | có |
| TCOV-SCHEDULED-TRANSFER-L2-FR-041-AC-AC-FR-041-3 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-041-3 | AC-FR-041-3 | Sau khi một lần chạy kết thúc, hệ thống sinh lần chạy PENDING kế tiếp đúng chu kỳ cho đến khi đạt maxRuns hoặc endDate, rồi lịch COMPLETED | có |
| TCOV-SCHEDULED-TRANSFER-L2-FR-041-AC-AC-FR-041-4 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-041-4 | AC-FR-041-4 | Lịch MONTHLY đặt ngày 29, 30 hoặc 31 chạy vào ngày cuối tháng ở tháng không đủ ngày, và ngày 29/02 ở năm nhuận | có |
| TCOV-SCHEDULED-TRANSFER-L2-FR-041-AC-AC-REV-001 | acceptanceCriterion | Tiêu chí chấp nhận AC-REV-001 | AC-REV-001 | Lịch định kỳ có endDate trước ngày của lần chạy đầu, hoặc endDate quá 366 ngày kể từ lần chạy đầu, thì trả 422 SCHEDULE_TIME_INVALID | có |
| TCOV-SCHEDULED-TRANSFER-L2-FR-041-AC-AC-REV-002 | acceptanceCriterion | Tiêu chí chấp nhận AC-REV-002 | AC-REV-002 | Lịch định kỳ có cả endDate và maxRuns kết thúc (COMPLETED) ở điều kiện đạt trước, không sinh thêm lần chạy PENDING | có |

### TD-SCHEDULED-TRANSFER-L2-FR-042-AC

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L2-FR-042-AC |
| Phủ TR | TR-SCHEDULED-TRANSFER-L2-FR-042 |
| Đối tượng thiết kế | Requirement FR-042 |
| Kỹ thuật | useCase |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-042-AC |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L2-FR-042-AC-AC-FR-042-1 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-042-1 | AC-FR-042-1 | GET một lịch trả status và nextRunAt | có |
| TCOV-SCHEDULED-TRANSFER-L2-FR-042-AC-AC-FR-042-2 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-042-2 | AC-FR-042-2 | GET danh sách lịch của tài khoản hỗ trợ lọc status và phân trang limit, offset | có |
| TCOV-SCHEDULED-TRANSFER-L2-FR-042-AC-AC-FR-042-3 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-042-3 | AC-FR-042-3 | GET runs trả mỗi lần chạy với scheduledAt, status, transferId và reasonCode, sắp mới nhất trước | có |
| TCOV-SCHEDULED-TRANSFER-L2-FR-042-AC-AC-FR-042-4 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-042-4 | AC-FR-042-4 | Người không phải chủ tài khoản nguồn nhận 403 FORBIDDEN; mã lịch không có nhận 404 SCHEDULE_NOT_FOUND | có |

### TD-SCHEDULED-TRANSFER-L2-FR-043-AC

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L2-FR-043-AC |
| Phủ TR | TR-SCHEDULED-TRANSFER-L2-FR-043 |
| Đối tượng thiết kế | Requirement FR-043 |
| Kỹ thuật | useCase |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-043-AC |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L2-FR-043-AC-AC-FR-043-1 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-043-1 | AC-FR-043-1 | DELETE lịch ACTIVE đổi lịch thành CANCELLED và các lần chạy PENDING thành CANCELLED | có |
| TCOV-SCHEDULED-TRANSFER-L2-FR-043-AC-AC-FR-043-2 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-043-2 | AC-FR-043-2 | DELETE lịch đã CANCELLED trả 200 với kết quả cũ | có |
| TCOV-SCHEDULED-TRANSFER-L2-FR-043-AC-AC-FR-043-3 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-043-3 | AC-FR-043-3 | DELETE khi có lần chạy RUNNING hoặc lịch COMPLETED trả 409 SCHEDULE_NOT_CANCELLABLE | có |

### TD-SCHEDULED-TRANSFER-L2-FR-044-AC

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L2-FR-044-AC |
| Phủ TR | TR-SCHEDULED-TRANSFER-L2-FR-044 |
| Đối tượng thiết kế | Requirement FR-044 |
| Kỹ thuật | useCase |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-044-AC |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L2-FR-044-AC-AC-FR-044-1 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-044-1 | AC-FR-044-1 | PATCH status=PAUSED trên lịch định kỳ ACTIVE đổi sang PAUSED, scheduler không chạy lịch này | có |
| TCOV-SCHEDULED-TRANSFER-L2-FR-044-AC-AC-FR-044-2 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-044-2 | AC-FR-044-2 | PATCH status=ACTIVE trên lịch PAUSED đổi sang ACTIVE và các lần đã bỏ lỡ trong lúc tạm dừng có trạng thái SKIPPED, không bị chạy bù | có |
| TCOV-SCHEDULED-TRANSFER-L2-FR-044-AC-AC-FR-044-3 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-044-3 | AC-FR-044-3 | PATCH sai trạng thái nguồn hoặc trên lịch ONCE trả 409 SCHEDULE_STATE_INVALID | có |

### TD-SCHEDULED-TRANSFER-L2-FR-045-AC

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L2-FR-045-AC |
| Phủ TR | TR-SCHEDULED-TRANSFER-L2-FR-045 |
| Đối tượng thiết kế | Requirement FR-045 |
| Kỹ thuật | useCase |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-045-AC |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L2-FR-045-AC-AC-FR-045-1 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-045-1 | AC-FR-045-1 | Lần chạy đến hạn tạo đúng một giao dịch COMPLETED qua luồng chuyển tiền hiện có, run SUCCEEDED có transferId | có |
| TCOV-SCHEDULED-TRANSFER-L2-FR-045-AC-AC-FR-045-2 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-045-2 | AC-FR-045-2 | Idempotency-Key của lần chạy đầu bằng mã lịch + thời điểm dự kiến | có |
| TCOV-SCHEDULED-TRANSFER-L2-FR-045-AC-AC-FR-045-3 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-045-3 | AC-FR-045-3 | Hạn mức ngày, số dư, trạng thái tài khoản và Fraud Check được kiểm tra tại lúc chạy, không phải lúc tạo lịch | có |
| TCOV-SCHEDULED-TRANSFER-L2-FR-045-AC-AC-FR-045-4 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-045-4 | AC-FR-045-4 | Giao dịch sinh từ lịch được tính vào hạn mức ngày và hiển thị trong lịch sử giao dịch | có |
| TCOV-SCHEDULED-TRANSFER-L2-FR-045-AC-AC-REV-004 | acceptanceCriterion | Tiêu chí chấp nhận AC-REV-004 | AC-REV-004 | Giao dịch sinh từ lịch hiển thị trong lịch sử giao dịch (FR-013) kèm trường scheduleId là mã lịch | có |

### TD-SCHEDULED-TRANSFER-L2-FR-045-SCN

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L2-FR-045-SCN |
| Phủ TR | TR-SCHEDULED-TRANSFER-L2-FR-045 |
| Đối tượng thiết kế | Scenario SCN-ScheduledRun |
| Kỹ thuật | useCase |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-045-SCN |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L2-FR-045-SCN-SCN-ScheduledRun | scenarioStep | Luồng chính SCN-ScheduledRun | SCN-ScheduledRun | 8 |  |

### TD-SCHEDULED-TRANSFER-L2-FR-046-AC

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L2-FR-046-AC |
| Phủ TR | TR-SCHEDULED-TRANSFER-L2-FR-046 |
| Đối tượng thiết kế | Requirement FR-046 |
| Kỹ thuật | useCase |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-046-AC |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L2-FR-046-AC-AC-FR-046-1 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-046-1 | AC-FR-046-1 | FRAUD_CHECK_UNAVAILABLE được thử lại tối đa 3 lần cách 5 phút, hết lượt thì run FAILED với reasonCode | có |
| TCOV-SCHEDULED-TRANSFER-L2-FR-046-AC-AC-FR-046-2 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-046-2 | AC-FR-046-2 | INSUFFICIENT_FUNDS được thử lại tối đa 2 lần cách 1 giờ, hết lượt thì run FAILED | có |
| TCOV-SCHEDULED-TRANSFER-L2-FR-046-AC-AC-FR-046-3 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-046-3 | AC-FR-046-3 | ACCOUNT_INACTIVE, FRAUD_REJECTED, LIMIT_EXCEEDED làm run FAILED ngay, không thử lại | có |
| TCOV-SCHEDULED-TRANSFER-L2-FR-046-AC-AC-FR-046-4 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-046-4 | AC-FR-046-4 | Sau 3 lần chạy FAILED liên tiếp lịch định kỳ chuyển PAUSED | có |

### TD-SCHEDULED-TRANSFER-L2-FR-046-SCN

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L2-FR-046-SCN |
| Phủ TR | TR-SCHEDULED-TRANSFER-L2-FR-046 |
| Đối tượng thiết kế | Scenario SCN-ScheduledRun |
| Kỹ thuật | useCase |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-046-SCN |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L2-FR-046-SCN-SCN-ScheduledRun | scenarioStep | Luồng chính SCN-ScheduledRun | SCN-ScheduledRun | 8 |  |

### TD-SCHEDULED-TRANSFER-L2-FR-047-AC

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L2-FR-047-AC |
| Phủ TR | TR-SCHEDULED-TRANSFER-L2-FR-047 |
| Đối tượng thiết kế | Requirement FR-047 |
| Kỹ thuật | useCase |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-047-AC |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L2-FR-047-AC-AC-FR-047-1 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-047-1 | AC-FR-047-1 | Mỗi lần chạy thành công phát sự kiện ScheduledRunCompleted có transferId qua outbox | có |
| TCOV-SCHEDULED-TRANSFER-L2-FR-047-AC-AC-FR-047-2 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-047-2 | AC-FR-047-2 | Mỗi lần chạy FAILED phát sự kiện ScheduledRunFailed có reasonCode qua outbox | có |
| TCOV-SCHEDULED-TRANSFER-L2-FR-047-AC-AC-FR-047-3 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-047-3 | AC-FR-047-3 | SMS kết quả được gửi cho người gửi; lỗi gửi SMS không đổi trạng thái lần chạy | có |
| TCOV-SCHEDULED-TRANSFER-L2-FR-047-AC-AC-REV-005 | acceptanceCriterion | Tiêu chí chấp nhận AC-REV-005 | AC-REV-005 | Sự kiện ScheduledRunCompleted/ScheduledRunFailed có đủ các trường eventId, scheduleId, runId, fromAccountId, toAccountId, amount, correlationId, kèm transferId (thành công) hoặc reasonCode (thất bại), trên cùng topic với sự kiện giao dịch | có |
| TCOV-SCHEDULED-TRANSFER-L2-FR-047-AC-AC-REV-006 | acceptanceCriterion | Tiêu chí chấp nhận AC-REV-006 | AC-REV-006 | Nội dung SMS của lần chạy thất bại nêu rõ số dư chỉ được kiểm tra lúc chạy | có |

### TD-SCHEDULED-TRANSFER-L2-FR-047-SCN

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L2-FR-047-SCN |
| Phủ TR | TR-SCHEDULED-TRANSFER-L2-FR-047 |
| Đối tượng thiết kế | Scenario SCN-ScheduledRun |
| Kỹ thuật | useCase |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-047-SCN |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L2-FR-047-SCN-SCN-ScheduledRun | scenarioStep | Luồng chính SCN-ScheduledRun | SCN-ScheduledRun | 8 |  |

### TD-SCHEDULED-TRANSFER-L2-FR-048-AC

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L2-FR-048-AC |
| Phủ TR | TR-SCHEDULED-TRANSFER-L2-FR-048 |
| Đối tượng thiết kế | Requirement FR-048 |
| Kỹ thuật | useCase |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-048-AC |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L2-FR-048-AC-AC-FR-048-1 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-048-1 | AC-FR-048-1 | Tạo, hủy, tạm dừng và tiếp tục lịch đều ghi audit với correlationId, thời điểm, tác nhân là mã tài khoản, loại sự kiện và mã lịch |  |
| TCOV-SCHEDULED-TRANSFER-L2-FR-048-AC-AC-FR-048-2 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-048-2 | AC-FR-048-2 | Mọi kết quả lần chạy ghi audit với tác nhân SCHEDULER và mã lịch |  |

### TD-SCHEDULED-TRANSFER-L2-FR-048-SCN

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L2-FR-048-SCN |
| Phủ TR | TR-SCHEDULED-TRANSFER-L2-FR-048 |
| Đối tượng thiết kế | Scenario SCN-ScheduledRun |
| Kỹ thuật | useCase |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-048-SCN |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L2-FR-048-SCN-SCN-ScheduledRun | scenarioStep | Luồng chính SCN-ScheduledRun | SCN-ScheduledRun | 8 |  |

### TD-SCHEDULED-TRANSFER-L2-FR-049-AC

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L2-FR-049-AC |
| Phủ TR | TR-SCHEDULED-TRANSFER-L2-FR-049 |
| Đối tượng thiết kế | Requirement FR-049 |
| Kỹ thuật | useCase |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-049-AC |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L2-FR-049-AC-AC-FR-049-1 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-049-1 | AC-FR-049-1 | Lần chạy quá hạn không quá 24 giờ sau khi hệ thống ngừng được chạy đúng một lần | có |
| TCOV-SCHEDULED-TRANSFER-L2-FR-049-AC-AC-FR-049-2 | acceptanceCriterion | Tiêu chí chấp nhận AC-FR-049-2 | AC-FR-049-2 | Lần chạy quá hạn quá 24 giờ chuyển MISSED, phát ScheduledRunFailed với reasonCode MISSED và không có giao dịch | có |

### TD-SCHEDULED-TRANSFER-L2-FR-049-SCN

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L2-FR-049-SCN |
| Phủ TR | TR-SCHEDULED-TRANSFER-L2-FR-049 |
| Đối tượng thiết kế | Scenario SCN-ScheduledRun |
| Kỹ thuật | useCase |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-049-SCN |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L2-FR-049-SCN-SCN-ScheduledRun | scenarioStep | Luồng chính SCN-ScheduledRun | SCN-ScheduledRun | 8 |  |

### TD-SCHEDULED-TRANSFER-L2-NFR-010-NFR

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L2-NFR-010-NFR |
| Phủ TR | TR-SCHEDULED-TRANSFER-L2-NFR-010 |
| Đối tượng thiết kế | Requirement NFR-010 |
| Kỹ thuật | boundaryValue |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L2-NFR-010-NFR |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L2-NFR-010-NFR-AT | boundary | Đúng ngưỡng 60 s | NFR-010 | 60 |  |
| TCOV-SCHEDULED-TRANSFER-L2-NFR-010-NFR-OVER | boundary | Vượt ngưỡng (60+1) | NFR-010 | 61 |  |

### TD-SCHEDULED-TRANSFER-L2-NFR-011-NFR

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L2-NFR-011-NFR |
| Phủ TR | TR-SCHEDULED-TRANSFER-L2-NFR-011 |
| Đối tượng thiết kế | Requirement NFR-011 |
| Kỹ thuật | boundaryValue |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L2-NFR-011-NFR |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L2-NFR-011-NFR-AT | boundary | Đúng ngưỡng 0 giao dịch trùng | NFR-011 | 0 |  |
| TCOV-SCHEDULED-TRANSFER-L2-NFR-011-NFR-OVER | boundary | Vượt ngưỡng (0+1) | NFR-011 | 1 |  |

### TD-SCHEDULED-TRANSFER-L2-NFR-012-NFR

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L2-NFR-012-NFR |
| Phủ TR | TR-SCHEDULED-TRANSFER-L2-NFR-012 |
| Đối tượng thiết kế | Requirement NFR-012 |
| Kỹ thuật | boundaryValue |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L2-NFR-012-NFR |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L2-NFR-012-NFR-AT | boundary | Đúng ngưỡng 5 chu kỳ quét | NFR-012 | 5 |  |
| TCOV-SCHEDULED-TRANSFER-L2-NFR-012-NFR-OVER | boundary | Vượt ngưỡng (5+1) | NFR-012 | 6 |  |

### TD-SCHEDULED-TRANSFER-L2-NFR-013-NFR

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L2-NFR-013-NFR |
| Phủ TR | TR-SCHEDULED-TRANSFER-L2-NFR-013 |
| Đối tượng thiết kế | Requirement NFR-013 |
| Kỹ thuật | boundaryValue |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L2-NFR-013-NFR |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L2-NFR-013-NFR-AT | boundary | Đúng ngưỡng 500 ms | NFR-013 | 500 |  |
| TCOV-SCHEDULED-TRANSFER-L2-NFR-013-NFR-OVER | boundary | Vượt ngưỡng (500+1) | NFR-013 | 501 |  |

### TD-SCHEDULED-TRANSFER-L2-NFR-014-NFR

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L2-NFR-014-NFR |
| Phủ TR | TR-SCHEDULED-TRANSFER-L2-NFR-014 |
| Đối tượng thiết kế | Requirement NFR-014 |
| Kỹ thuật | boundaryValue |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L2-NFR-014-NFR |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L2-NFR-014-NFR-AT | boundary | Đúng ngưỡng 100 % | NFR-014 | 100 |  |
| TCOV-SCHEDULED-TRANSFER-L2-NFR-014-NFR-OVER | boundary | Vượt ngưỡng (100+1) | NFR-014 | 101 |  |

### TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Phủ TR | TR-SCHEDULED-TRANSFER-L2-SM-Schedule |
| Đối tượng thiết kế | StateMachine SM-Schedule |
| Kỹ thuật | stateTransition |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-ACTIVE-PATCH-status-ACTIVE | transition | PATCH status=ACTIVE từ trạng thái ACTIVE không được phép | ACTIVE/PATCH status=ACTIVE | invalid |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-CANCELLED-DELETE | transition | DELETE từ trạng thái CANCELLED không được phép | CANCELLED/DELETE | invalid |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-CANCELLED-PATCH-status-ACTIVE | transition | PATCH status=ACTIVE từ trạng thái CANCELLED không được phép | CANCELLED/PATCH status=ACTIVE | invalid |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-CANCELLED-PATCH-status-PAUSED | transition | PATCH status=PAUSED từ trạng thái CANCELLED không được phép | CANCELLED/PATCH status=PAUSED | invalid |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-CANCELLED-l-n-ch-y-cu-i-k-t-th-c | transition | lần chạy cuối kết thúc từ trạng thái CANCELLED không được phép | CANCELLED/lần chạy cuối kết thúc | invalid |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-CANCELLED-scheduler-l-n-ch-y-FAILED-th-3-li-n-ti-p | transition | scheduler: lần chạy FAILED thứ 3 liên tiếp từ trạng thái CANCELLED không được phép | CANCELLED/scheduler: lần chạy FAILED thứ 3 liên tiếp | invalid |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-COMPLETED-DELETE | transition | DELETE từ trạng thái COMPLETED không được phép | COMPLETED/DELETE | invalid |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-COMPLETED-PATCH-status-ACTIVE | transition | PATCH status=ACTIVE từ trạng thái COMPLETED không được phép | COMPLETED/PATCH status=ACTIVE | invalid |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-COMPLETED-PATCH-status-PAUSED | transition | PATCH status=PAUSED từ trạng thái COMPLETED không được phép | COMPLETED/PATCH status=PAUSED | invalid |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-COMPLETED-l-n-ch-y-cu-i-k-t-th-c | transition | lần chạy cuối kết thúc từ trạng thái COMPLETED không được phép | COMPLETED/lần chạy cuối kết thúc | invalid |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-TRN-Schedule-1 | transition | ACTIVE -> PAUSED khi PATCH status=PAUSED | TRN-Schedule-1 | ACTIVE->PAUSED |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-TRN-Schedule-2 | transition | PAUSED -> ACTIVE khi PATCH status=ACTIVE | TRN-Schedule-2 | PAUSED->ACTIVE |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-TRN-Schedule-3 | transition | ACTIVE -> PAUSED khi scheduler: lần chạy FAILED thứ 3 liên tiếp | TRN-Schedule-3 | ACTIVE->PAUSED |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-TRN-Schedule-4 | transition | ACTIVE -> COMPLETED khi lần chạy cuối kết thúc | TRN-Schedule-4 | ACTIVE->COMPLETED |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-TRN-Schedule-5 | transition | ACTIVE -> CANCELLED khi DELETE | TRN-Schedule-5 | ACTIVE->CANCELLED |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-TRN-Schedule-6 | transition | PAUSED -> CANCELLED khi DELETE | TRN-Schedule-6 | PAUSED->CANCELLED |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-TRN-Schedule-7 | transition | PAUSED -> COMPLETED khi PATCH status=ACTIVE | TRN-Schedule-7 | PAUSED->COMPLETED |  |

### TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Phủ TR | TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun |
| Đối tượng thiết kế | StateMachine SM-ScheduledRun |
| Kỹ thuật | stateTransition |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED--u-chu-k-qu-t | transition | đầu chu kỳ quét từ trạng thái CANCELLED không được phép | CANCELLED/đầu chu kỳ quét | invalid |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED-DELETE-l-ch | transition | DELETE lịch từ trạng thái CANCELLED không được phép | CANCELLED/DELETE lịch | invalid |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED-l-i-chuy-n-ti-n-tr-giao-d-ch-COMPLETED-ho-c-ph-t-l-i | transition | lõi chuyển tiền trả giao dịch COMPLETED (hoặc phát lại) từ trạng thái CANCELLED không được phép | CANCELLED/lõi chuyển tiền trả giao dịch COMPLETED (hoặc phát lại) | invalid |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED-l-i-tr-l-i | transition | lõi trả lỗi từ trạng thái CANCELLED không được phép | CANCELLED/lõi trả lỗi | invalid |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED-l-i-tr-l-i-t-m-th-i-ho-c-thi-u-s-d | transition | lõi trả lỗi tạm thời hoặc thiếu số dư từ trạng thái CANCELLED không được phép | CANCELLED/lõi trả lỗi tạm thời hoặc thiếu số dư | invalid |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED-scheduler-chi-m-l-n-ch-y | transition | scheduler chiếm lần chạy từ trạng thái CANCELLED không được phép | CANCELLED/scheduler chiếm lần chạy | invalid |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED-scheduler-ph-c-h-i-l-n-ch-y-k-t | transition | scheduler phục hồi lần chạy kẹt từ trạng thái CANCELLED không được phép | CANCELLED/scheduler phục hồi lần chạy kẹt | invalid |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED-ti-p-t-c-l-ch-PAUSED | transition | tiếp tục lịch PAUSED từ trạng thái CANCELLED không được phép | CANCELLED/tiếp tục lịch PAUSED | invalid |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-FAILED-DELETE-l-ch | transition | DELETE lịch từ trạng thái FAILED không được phép | FAILED/DELETE lịch | invalid |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-FAILED-l-i-chuy-n-ti-n-tr-giao-d-ch-COMPLETED-ho-c-ph-t-l-i | transition | lõi chuyển tiền trả giao dịch COMPLETED (hoặc phát lại) từ trạng thái FAILED không được phép | FAILED/lõi chuyển tiền trả giao dịch COMPLETED (hoặc phát lại) | invalid |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-1 | transition | PENDING -> RUNNING khi scheduler chiếm lần chạy | TRN-ScheduledRun-1 | PENDING->RUNNING |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-2 | transition | RUNNING -> SUCCEEDED khi lõi chuyển tiền trả giao dịch COMPLETED (hoặc phát lại) | TRN-ScheduledRun-2 | RUNNING->SUCCEEDED |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-3 | transition | RUNNING -> PENDING khi lõi trả lỗi tạm thời hoặc thiếu số dư | TRN-ScheduledRun-3 | RUNNING->PENDING |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-4 | transition | RUNNING -> FAILED khi lõi trả lỗi | TRN-ScheduledRun-4 | RUNNING->FAILED |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-5 | transition | PENDING -> SKIPPED khi tiếp tục lịch PAUSED | TRN-ScheduledRun-5 | PENDING->SKIPPED |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-6 | transition | PENDING -> MISSED khi đầu chu kỳ quét | TRN-ScheduledRun-6 | PENDING->MISSED |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-7 | transition | PENDING -> CANCELLED khi DELETE lịch | TRN-ScheduledRun-7 | PENDING->CANCELLED |  |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-8 | transition | RUNNING -> PENDING khi scheduler phục hồi lần chạy kẹt | TRN-ScheduledRun-8 | RUNNING->PENDING |  |

### TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-BRK-INT-01

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-BRK-INT-01 |
| Phủ TR | TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler |
| Đối tượng thiết kế | Integration INT-01 |
| Kỹ thuật | decisionTable |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-BRK-INT-01 |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-BRK-INT-01-RECLOSE | breakerTrip | sau 30 giây breaker thử lại | openDurationSec | 30 | có |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-BRK-INT-01-TRIP | breakerTrip | tỷ lệ lỗi >= 100% sau 5 lượt thì breaker mở | failureRatePercent | 100 | có |

### TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Account

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Account |
| Phủ TR | TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler |
| Đối tượng thiết kế | Module MOD-Scheduler |
| Kỹ thuật | errorGuessing |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Account |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Account-FAIL | dependencyFailure | MOD-Account lỗi, MOD-Scheduler xử lý lỗi có kiểm soát | dependency | MOD-Account |  |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Account-OK | dependencyFailure | MOD-Scheduler gọi MOD-Account thành công | dependency | MOD-Account |  |

### TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Audit

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Audit |
| Phủ TR | TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler |
| Đối tượng thiết kế | Module MOD-Scheduler |
| Kỹ thuật | errorGuessing |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Audit |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Audit-FAIL | dependencyFailure | MOD-Audit lỗi, MOD-Scheduler xử lý lỗi có kiểm soát | dependency | MOD-Audit |  |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Audit-OK | dependencyFailure | MOD-Scheduler gọi MOD-Audit thành công | dependency | MOD-Audit |  |

### TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Risk

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Risk |
| Phủ TR | TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler |
| Đối tượng thiết kế | Module MOD-Scheduler |
| Kỹ thuật | errorGuessing |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Risk |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Risk-FAIL | dependencyFailure | MOD-Risk lỗi, MOD-Scheduler xử lý lỗi có kiểm soát | dependency | MOD-Risk |  |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Risk-OK | dependencyFailure | MOD-Scheduler gọi MOD-Risk thành công | dependency | MOD-Risk |  |

### TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Transfer

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Transfer |
| Phủ TR | TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler |
| Đối tượng thiết kế | Module MOD-Scheduler |
| Kỹ thuật | errorGuessing |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Transfer |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Transfer-FAIL | dependencyFailure | MOD-Transfer lỗi, MOD-Scheduler xử lý lỗi có kiểm soát | dependency | MOD-Transfer |  |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Transfer-OK | dependencyFailure | MOD-Scheduler gọi MOD-Transfer thành công | dependency | MOD-Transfer |  |

### TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01 |
| Phủ TR | TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler |
| Đối tượng thiết kế | Integration INT-01 |
| Kỹ thuật | errorGuessing |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01 |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01-ERR-INT01-4xx | errorCategory | ERR-INT01-4xx: HTTP 4xx (400, 401, 422) là permanent | trigger | permanent | có |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01-ERR-INT01-5xx | errorCategory | ERR-INT01-5xx: HTTP 5xx là transient | trigger | transient | có |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01-ERR-INT01-schema | errorCategory | ERR-INT01-schema: HTTP 200 nhưng response sai schema là permanent | trigger | permanent | có |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01-ERR-INT01-timeout | errorCategory | ERR-INT01-timeout: Timeout (connect/read) hoặc lỗi mạng là unknownOutcome | trigger | unknownOutcome | có |

### TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-IDM-INT-01

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-IDM-INT-01 |
| Phủ TR | TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler |
| Đối tượng thiết kế | Integration INT-01 |
| Kỹ thuật | errorGuessing |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-IDM-INT-01 |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-IDM-INT-01-REPLAY | idempotentReplay | gửi lại cùng yêu cầu tới INT-01 (idempotent=true) | replay | 2 | có |

### TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-maxPayloadKB

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-maxPayloadKB |
| Phủ TR | TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler |
| Đối tượng thiết kế | Integration INT-01 |
| Kỹ thuật | boundaryValue |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-maxPayloadKB |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-maxPayloadKB-AT | rateLimit | đúng giới hạn maxPayloadKB = 16 KB | maxPayloadKB | 16 | có |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-maxPayloadKB-OVER | rateLimit | vượt giới hạn maxPayloadKB = 16 KB | maxPayloadKB | 17 | có |

### TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-requestsPerSecond

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-requestsPerSecond |
| Phủ TR | TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler |
| Đối tượng thiết kế | Integration INT-01 |
| Kỹ thuật | boundaryValue |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-requestsPerSecond |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-requestsPerSecond-AT | rateLimit | đúng giới hạn requestsPerSecond = 50 req/s | requestsPerSecond | 50 | có |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-requestsPerSecond-OVER | rateLimit | vượt giới hạn requestsPerSecond = 50 req/s | requestsPerSecond | 51 | có |

### TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-RTY-INT-01

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-RTY-INT-01 |
| Phủ TR | TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler |
| Đối tượng thiết kế | Integration INT-01 |
| Kỹ thuật | boundaryValue |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-RTY-INT-01 |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-RTY-INT-01-EXHAUST | retryAttempt | lỗi tạm thời đủ 2 lần | maxAttempts | 2 | có |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-RTY-INT-01-RECOVER | retryAttempt | lỗi tạm thời 1 lần rồi thành công | maxAttempts | 2 | có |

### TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-TMO-INT-01

| Trường | Giá trị |
| --- | --- |
| id | TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-TMO-INT-01 |
| Phủ TR | TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler |
| Đối tượng thiết kế | Integration INT-01 |
| Kỹ thuật | boundaryValue |
| Độ đo phủ và ngưỡng | mọi coverage item được ít nhất một TestCase (100%) |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-TMO-INT-01 |
| Loại thay đổi | new |

| TCOV | Loại | Mô tả | Tham số | Giá trị | Chờ xác nhận |
| --- | --- | --- | --- | --- | --- |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-TMO-INT-01-OVER | timeoutBoundary | INT-01 không trả lời trước totalMs (1000 ms) | delayMs | 2000 | có |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-TMO-INT-01-UNDER | timeoutBoundary | INT-01 trả lời chậm nhưng trong readMs | delayMs | 400 | có |

## 3. State machine

### SM-Schedule: Vòng đời lịch chuyển tiền

| Transition | Từ | Đến | Trigger | Guard | Effect | Nguồn |
| --- | --- | --- | --- | --- | --- | --- |
| TRN-Schedule-1 | ACTIVE | PAUSED | PATCH status=PAUSED | lịch định kỳ | Audit SCHEDULE_PAUSED; scheduler không chạy lịch | stated |
| TRN-Schedule-2 | PAUSED | ACTIVE | PATCH status=ACTIVE | còn lần chạy theo BR-11 | Lần bỏ lỡ thành SKIPPED; sinh lần chạy kế tiếp; consecutive_failures về 0; audit SCHEDULE_RESUMED | stated |
| TRN-Schedule-3 | ACTIVE | PAUSED | scheduler: lần chạy FAILED thứ 3 liên tiếp | lịch định kỳ | Audit SCHEDULE_AUTO_PAUSED (tác nhân SCHEDULER); không sinh lần chạy mới | stated |
| TRN-Schedule-4 | ACTIVE | COMPLETED | lần chạy cuối kết thúc | lịch ONCE đã xong, hoặc đạt maxRuns hoặc endDate | Không sinh lần chạy mới | stated |
| TRN-Schedule-5 | ACTIVE | CANCELLED | DELETE | không có lần chạy RUNNING | Lần PENDING thành CANCELLED; audit SCHEDULE_CANCELLED | stated |
| TRN-Schedule-6 | PAUSED | CANCELLED | DELETE | không có lần chạy RUNNING | Lần PENDING thành CANCELLED; audit SCHEDULE_CANCELLED | stated |
| TRN-Schedule-7 | PAUSED | COMPLETED | PATCH status=ACTIVE | không còn lần chạy nào theo BR-11 sau khi đánh dấu SKIPPED | Các lần còn lại SKIPPED; không sinh lần mới | stated |

### SM-ScheduledRun: Vòng đời lần chạy

| Transition | Từ | Đến | Trigger | Guard | Effect | Nguồn |
| --- | --- | --- | --- | --- | --- | --- |
| TRN-ScheduledRun-1 | PENDING | RUNNING | scheduler chiếm lần chạy | đến hạn (scheduled_at hoặc next_attempt_at ≤ now), lịch ACTIVE, UPDATE có điều kiện rowcount=1 | attempts+1; sinh correlationId; gọi lõi chuyển tiền | stated |
| TRN-ScheduledRun-2 | RUNNING | SUCCEEDED | lõi chuyển tiền trả giao dịch COMPLETED (hoặc phát lại) |  | Lưu transferId; consecutive_failures về 0; audit; phát ScheduledRunCompleted; sinh lần kế tiếp hoặc COMPLETED lịch | stated |
| TRN-ScheduledRun-3 | RUNNING | PENDING | lõi trả lỗi tạm thời hoặc thiếu số dư | còn lượt thử lại (FRAUD_CHECK_UNAVAILABLE ≤ 3 lần, INSUFFICIENT_FUNDS ≤ 2 lần) | Đặt next_attempt_at = now + 5 phút hoặc 1 giờ | stated |
| TRN-ScheduledRun-4 | RUNNING | FAILED | lõi trả lỗi | lỗi vĩnh viễn hoặc hết lượt thử lại | Lưu reasonCode; consecutive_failures+1; audit; phát ScheduledRunFailed | stated |
| TRN-ScheduledRun-5 | PENDING | SKIPPED | tiếp tục lịch PAUSED | lần chạy đã quá hạn trong lúc tạm dừng | Không chạy bù | stated |
| TRN-ScheduledRun-6 | PENDING | MISSED | đầu chu kỳ quét | quá hạn hơn 24 giờ so với scheduled_at | Phát ScheduledRunFailed với reasonCode MISSED; audit; không tính FAILED liên tiếp | stated |
| TRN-ScheduledRun-7 | PENDING | CANCELLED | DELETE lịch |  |  | stated |
| TRN-ScheduledRun-8 | RUNNING | PENDING | scheduler phục hồi lần chạy kẹt | started_at cũ hơn lease (120 giây, đề xuất Q-S2) | Giữ attempts để dùng lại cùng khóa idempotency | stated |

## 4. Yêu cầu dữ liệu test

| DATA | Thực thể | Trường | Lớp giá trị | Ràng buộc | Ví dụ | Nhạy cảm | Tạo hoặc dọn |
| --- | --- | --- | --- | --- | --- | --- | --- |
| DATA-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-JRN | SCN-ScheduledRun | SCN-ScheduledRun | main | Luồng chính SCN-ScheduledRun | 8 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-JRN | SCN-ScheduledRun | responseBudgetMs | budget | SCN-ScheduledRun hoàn tất trong 60000 ms | 60000 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API | EP-CancelSchedule | response | rsp-cancel-200 | DELETE /v1/scheduled-transfers/{scheduleId} → 200: Hủy thành công (lịch CANCELLED, lần PENDING thành CANCELLED) hoặc lịch đã CANCELLED: trả kết quả cũ | 200 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API | EP-CancelSchedule | response | rsp-cancel-401 | DELETE /v1/scheduled-transfers/{scheduleId} → 401 UNAUTHORIZED: Thiếu hoặc sai API key | 401 UNAUTHORIZED | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API | EP-CancelSchedule | response | rsp-cancel-403 | DELETE /v1/scheduled-transfers/{scheduleId} → 403 FORBIDDEN: Người gọi không phải chủ lịch | 403 FORBIDDEN | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API | EP-CancelSchedule | response | rsp-cancel-404 | DELETE /v1/scheduled-transfers/{scheduleId} → 404 SCHEDULE_NOT_FOUND: Không có lịch với scheduleId | 404 SCHEDULE_NOT_FOUND | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API | EP-CancelSchedule | response | rsp-cancel-409 | DELETE /v1/scheduled-transfers/{scheduleId} → 409 SCHEDULE_NOT_CANCELLABLE: Lịch đã COMPLETED hoặc có lần chạy đang RUNNING | 409 SCHEDULE_NOT_CANCELLABLE | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API | EP-CreateSchedule | amount (body) | fld-create-amount-min | POST /v1/scheduled-transfers: amount (body) đúng giá trị nhỏ nhất 1 | 1 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API | EP-CreateSchedule | recurrence (body) | fld-create-recurrence-enum-daily | POST /v1/scheduled-transfers: recurrence (body) = DAILY (giá trị hợp lệ) | DAILY | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API | EP-CreateSchedule | recurrence (body) | fld-create-recurrence-enum-weekly | POST /v1/scheduled-transfers: recurrence (body) = WEEKLY (giá trị hợp lệ) | WEEKLY | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API | EP-CreateSchedule | recurrence (body) | fld-create-recurrence-enum-monthly | POST /v1/scheduled-transfers: recurrence (body) = MONTHLY (giá trị hợp lệ) | MONTHLY | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API | EP-CreateSchedule | recurrence (body) | fld-create-recurrence-enum-invalid | POST /v1/scheduled-transfers: recurrence (body) ngoài danh sách ONCE, DAILY, WEEKLY, MONTHLY | <ngoài danh sách> | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API | EP-CreateSchedule | response | rsp-create-200 | POST /v1/scheduled-transfers → 200: Cùng Idempotency-Key và cùng nội dung: trả lịch đã tạo | 200 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API | EP-CreateSchedule | response | rsp-create-201 | POST /v1/scheduled-transfers → 201: Tạo lịch thành công, status=ACTIVE, trả mã lịch | 201 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API | EP-CreateSchedule | response | rsp-create-400 | POST /v1/scheduled-transfers → 400 INVALID_REQUEST: Thiếu Idempotency-Key hoặc trường bắt buộc, amount không nguyên dương, recurrence sai, định kỳ thiếu cả endDate và maxRuns, maxRuns ngoài 1..60, firstRunAt sai định dạng | 400 INVALID_REQUEST | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API | EP-CreateSchedule | response | rsp-create-401 | POST /v1/scheduled-transfers → 401 UNAUTHORIZED: Thiếu hoặc sai API key | 401 UNAUTHORIZED | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API | EP-CreateSchedule | response | rsp-create-403 | POST /v1/scheduled-transfers → 403 FORBIDDEN: fromAccountId khác tài khoản của người gọi | 403 FORBIDDEN | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API | EP-CreateSchedule | response | rsp-create-404 | POST /v1/scheduled-transfers → 404 ACCOUNT_NOT_FOUND: Tài khoản đích không tồn tại | 404 ACCOUNT_NOT_FOUND | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API | EP-CreateSchedule | amount (body) | fld-create-amount-below-min | POST /v1/scheduled-transfers: amount (body) nhỏ hơn nhỏ nhất (0) | 0 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API | EP-CreateSchedule | response | rsp-create-409 | POST /v1/scheduled-transfers → 409 IDEMPOTENCY_CONFLICT: Cùng Idempotency-Key nhưng nội dung khác | 409 IDEMPOTENCY_CONFLICT | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API | EP-CreateSchedule | response | rsp-create-422-amount | POST /v1/scheduled-transfers → 422 LIMIT_EXCEEDED: amount > 50.000.000 VND | 422 LIMIT_EXCEEDED | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API | EP-CreateSchedule | response | rsp-create-422-limit | POST /v1/scheduled-transfers → 422 SCHEDULE_LIMIT_REACHED: Tài khoản đã có 20 lịch ACTIVE hoặc PAUSED | 422 SCHEDULE_LIMIT_REACHED | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API | EP-CreateSchedule | response | rsp-create-422-same | POST /v1/scheduled-transfers → 422 SAME_ACCOUNT: fromAccountId bằng toAccountId | 422 SAME_ACCOUNT | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API | EP-CreateSchedule | response | rsp-create-422-time | POST /v1/scheduled-transfers → 422 SCHEDULE_TIME_INVALID: firstRunAt cách hiện tại < 5 phút hoặc > 90 ngày; endDate trước lần chạy đầu hoặc quá 366 ngày | 422 SCHEDULE_TIME_INVALID | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API | EP-CreateSchedule | amount (body) | fld-create-amount-max | POST /v1/scheduled-transfers: amount (body) đúng giá trị lớn nhất 50000000 | 50000000 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API | EP-CreateSchedule | amount (body) | fld-create-amount-above-max | POST /v1/scheduled-transfers: amount (body) lớn hơn lớn nhất (50000001) | 50000001 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API | EP-CreateSchedule | maxRuns (body) | fld-create-maxruns-min | POST /v1/scheduled-transfers: maxRuns (body) đúng giá trị nhỏ nhất 1 | 1 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API | EP-CreateSchedule | maxRuns (body) | fld-create-maxruns-below-min | POST /v1/scheduled-transfers: maxRuns (body) nhỏ hơn nhỏ nhất (0) | 0 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API | EP-CreateSchedule | maxRuns (body) | fld-create-maxruns-max | POST /v1/scheduled-transfers: maxRuns (body) đúng giá trị lớn nhất 60 | 60 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API | EP-CreateSchedule | maxRuns (body) | fld-create-maxruns-above-max | POST /v1/scheduled-transfers: maxRuns (body) lớn hơn lớn nhất (61) | 61 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API | EP-CreateSchedule | recurrence (body) | fld-create-recurrence-enum-once | POST /v1/scheduled-transfers: recurrence (body) = ONCE (giá trị hợp lệ) | ONCE | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API | EP-GetSchedule | response | rsp-get-200 | GET /v1/scheduled-transfers/{scheduleId} → 200: Trả trạng thái lịch và lần chạy kế tiếp (nextRunAt) | 200 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API | EP-GetSchedule | response | rsp-get-401 | GET /v1/scheduled-transfers/{scheduleId} → 401 UNAUTHORIZED: Thiếu hoặc sai API key | 401 UNAUTHORIZED | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API | EP-GetSchedule | response | rsp-get-403 | GET /v1/scheduled-transfers/{scheduleId} → 403 FORBIDDEN: Người gọi không phải chủ tài khoản nguồn của lịch | 403 FORBIDDEN | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API | EP-GetSchedule | response | rsp-get-404 | GET /v1/scheduled-transfers/{scheduleId} → 404 SCHEDULE_NOT_FOUND: Không có lịch với scheduleId | 404 SCHEDULE_NOT_FOUND | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-ListRuns-API | EP-ListRuns | limit (query) | fld-listruns-limit-min | GET /v1/scheduled-transfers/{scheduleId}/runs: limit (query) đúng giá trị nhỏ nhất 1 | 1 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-ListRuns-API | EP-ListRuns | response | rsp-listruns-404 | GET /v1/scheduled-transfers/{scheduleId}/runs → 404 SCHEDULE_NOT_FOUND: Không có lịch với scheduleId | 404 SCHEDULE_NOT_FOUND | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-ListRuns-API | EP-ListRuns | limit (query) | fld-listruns-limit-below-min | GET /v1/scheduled-transfers/{scheduleId}/runs: limit (query) nhỏ hơn nhỏ nhất (0) | 0 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-ListRuns-API | EP-ListRuns | limit (query) | fld-listruns-limit-max | GET /v1/scheduled-transfers/{scheduleId}/runs: limit (query) đúng giá trị lớn nhất 100 | 100 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-ListRuns-API | EP-ListRuns | limit (query) | fld-listruns-limit-above-max | GET /v1/scheduled-transfers/{scheduleId}/runs: limit (query) lớn hơn lớn nhất (101) | 101 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-ListRuns-API | EP-ListRuns | offset (query) | fld-listruns-offset-min | GET /v1/scheduled-transfers/{scheduleId}/runs: offset (query) đúng giá trị nhỏ nhất 0 | 0 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-ListRuns-API | EP-ListRuns | offset (query) | fld-listruns-offset-below-min | GET /v1/scheduled-transfers/{scheduleId}/runs: offset (query) nhỏ hơn nhỏ nhất (-1) | -1 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-ListRuns-API | EP-ListRuns | response | rsp-listruns-200 | GET /v1/scheduled-transfers/{scheduleId}/runs → 200: Trả các lần chạy, mới nhất trước, mỗi lần có thời điểm dự kiến, trạng thái, transferId, reasonCode | 200 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-ListRuns-API | EP-ListRuns | response | rsp-listruns-401 | GET /v1/scheduled-transfers/{scheduleId}/runs → 401 UNAUTHORIZED: Thiếu hoặc sai API key | 401 UNAUTHORIZED | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-ListRuns-API | EP-ListRuns | response | rsp-listruns-403 | GET /v1/scheduled-transfers/{scheduleId}/runs → 403 FORBIDDEN: Người gọi không phải chủ lịch | 403 FORBIDDEN | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API | EP-ListSchedules | limit (query) | fld-listschedules-limit-min | GET /v1/accounts/{accountId}/scheduled-transfers: limit (query) đúng giá trị nhỏ nhất 1 | 1 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API | EP-ListSchedules | status (query) | fld-listschedules-status-enum-cancelled | GET /v1/accounts/{accountId}/scheduled-transfers: status (query) = CANCELLED (giá trị hợp lệ) | CANCELLED | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API | EP-ListSchedules | status (query) | fld-listschedules-status-enum-invalid | GET /v1/accounts/{accountId}/scheduled-transfers: status (query) ngoài danh sách ACTIVE, PAUSED, COMPLETED, CANCELLED | <ngoài danh sách> | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API | EP-ListSchedules | response | rsp-listschedules-200 | GET /v1/accounts/{accountId}/scheduled-transfers → 200: Trả danh sách lịch của tài khoản, phân trang, lọc theo status | 200 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API | EP-ListSchedules | response | rsp-listschedules-400 | GET /v1/accounts/{accountId}/scheduled-transfers → 400 INVALID_REQUEST: status ngoài enum | 400 INVALID_REQUEST | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API | EP-ListSchedules | response | rsp-listschedules-401 | GET /v1/accounts/{accountId}/scheduled-transfers → 401 UNAUTHORIZED: Thiếu hoặc sai API key | 401 UNAUTHORIZED | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API | EP-ListSchedules | response | rsp-listschedules-403 | GET /v1/accounts/{accountId}/scheduled-transfers → 403 FORBIDDEN: accountId khác tài khoản của người gọi | 403 FORBIDDEN | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API | EP-ListSchedules | limit (query) | fld-listschedules-limit-below-min | GET /v1/accounts/{accountId}/scheduled-transfers: limit (query) nhỏ hơn nhỏ nhất (0) | 0 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API | EP-ListSchedules | limit (query) | fld-listschedules-limit-max | GET /v1/accounts/{accountId}/scheduled-transfers: limit (query) đúng giá trị lớn nhất 100 | 100 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API | EP-ListSchedules | limit (query) | fld-listschedules-limit-above-max | GET /v1/accounts/{accountId}/scheduled-transfers: limit (query) lớn hơn lớn nhất (101) | 101 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API | EP-ListSchedules | offset (query) | fld-listschedules-offset-min | GET /v1/accounts/{accountId}/scheduled-transfers: offset (query) đúng giá trị nhỏ nhất 0 | 0 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API | EP-ListSchedules | offset (query) | fld-listschedules-offset-below-min | GET /v1/accounts/{accountId}/scheduled-transfers: offset (query) nhỏ hơn nhỏ nhất (-1) | -1 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API | EP-ListSchedules | status (query) | fld-listschedules-status-enum-active | GET /v1/accounts/{accountId}/scheduled-transfers: status (query) = ACTIVE (giá trị hợp lệ) | ACTIVE | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API | EP-ListSchedules | status (query) | fld-listschedules-status-enum-paused | GET /v1/accounts/{accountId}/scheduled-transfers: status (query) = PAUSED (giá trị hợp lệ) | PAUSED | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API | EP-ListSchedules | status (query) | fld-listschedules-status-enum-completed | GET /v1/accounts/{accountId}/scheduled-transfers: status (query) = COMPLETED (giá trị hợp lệ) | COMPLETED | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API | EP-UpdateScheduleStatus | status (body) | fld-patch-status-enum-paused | PATCH /v1/scheduled-transfers/{scheduleId}: status (body) = PAUSED (giá trị hợp lệ) | PAUSED | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API | EP-UpdateScheduleStatus | status (body) | fld-patch-status-enum-active | PATCH /v1/scheduled-transfers/{scheduleId}: status (body) = ACTIVE (giá trị hợp lệ) | ACTIVE | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API | EP-UpdateScheduleStatus | status (body) | fld-patch-status-enum-invalid | PATCH /v1/scheduled-transfers/{scheduleId}: status (body) ngoài danh sách PAUSED, ACTIVE | <ngoài danh sách> | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API | EP-UpdateScheduleStatus | response | rsp-patch-200 | PATCH /v1/scheduled-transfers/{scheduleId} → 200: Chuyển PAUSED hoặc ACTIVE thành công, trả lịch | 200 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API | EP-UpdateScheduleStatus | response | rsp-patch-400 | PATCH /v1/scheduled-transfers/{scheduleId} → 400 INVALID_REQUEST: status thiếu hoặc ngoài PAUSED/ACTIVE | 400 INVALID_REQUEST | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API | EP-UpdateScheduleStatus | response | rsp-patch-401 | PATCH /v1/scheduled-transfers/{scheduleId} → 401 UNAUTHORIZED: Thiếu hoặc sai API key | 401 UNAUTHORIZED | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API | EP-UpdateScheduleStatus | response | rsp-patch-403 | PATCH /v1/scheduled-transfers/{scheduleId} → 403 FORBIDDEN: Người gọi không phải chủ lịch | 403 FORBIDDEN | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API | EP-UpdateScheduleStatus | response | rsp-patch-404 | PATCH /v1/scheduled-transfers/{scheduleId} → 404 SCHEDULE_NOT_FOUND: Không có lịch với scheduleId | 404 SCHEDULE_NOT_FOUND | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API | EP-UpdateScheduleStatus | response | rsp-patch-409 | PATCH /v1/scheduled-transfers/{scheduleId} → 409 SCHEDULE_STATE_INVALID: Tạm dừng lịch không ACTIVE, tiếp tục lịch không PAUSED, hoặc lịch ONCE | 409 SCHEDULE_STATE_INVALID | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-040-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-FR-040-1 | ac-fr-040-1 | Tiêu chí chấp nhận AC-FR-040-1 | POST tạo lịch ONCE hợp lệ trả 201, có scheduleId và status ACTIVE | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-040-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-FR-040-2 | ac-fr-040-2 | Tiêu chí chấp nhận AC-FR-040-2 | Số dư hai tài khoản không đổi và không có bản ghi ledger_entries mới sau khi tạo lịch | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-040-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-FR-040-3 | ac-fr-040-3 | Tiêu chí chấp nhận AC-FR-040-3 | firstRunAt cách hiện tại < 5 phút hoặc > 90 ngày thì trả 422 SCHEDULE_TIME_INVALID | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-040-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-FR-040-4 | ac-fr-040-4 | Tiêu chí chấp nhận AC-FR-040-4 | toAccountId không tồn tại thì trả 404 ACCOUNT_NOT_FOUND | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-040-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-FR-040-5 | ac-fr-040-5 | Tiêu chí chấp nhận AC-FR-040-5 | fromAccountId khác chủ API key thì trả 403 FORBIDDEN | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-040-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-FR-040-6 | ac-fr-040-6 | Tiêu chí chấp nhận AC-FR-040-6 | Lịch thứ 21 ACTIVE/PAUSED của một tài khoản trả 422 SCHEDULE_LIMIT_REACHED | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-040-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-REV-003 | ac-rev-003 | Tiêu chí chấp nhận AC-REV-003 | Chỉ lịch ACTIVE hoặc PAUSED được tính vào giới hạn 20 lịch của tài khoản; lịch COMPLETED hoặc CANCELLED không được tính | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-041-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-FR-041-1 | ac-fr-041-1 | Tiêu chí chấp nhận AC-FR-041-1 | Lịch DAILY, WEEKLY hoặc MONTHLY có endDate hoặc maxRuns được tạo thành công (201) | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-041-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-FR-041-2 | ac-fr-041-2 | Tiêu chí chấp nhận AC-FR-041-2 | Lịch định kỳ thiếu cả endDate và maxRuns trả 400 INVALID_REQUEST | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-041-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-FR-041-3 | ac-fr-041-3 | Tiêu chí chấp nhận AC-FR-041-3 | Sau khi một lần chạy kết thúc, hệ thống sinh lần chạy PENDING kế tiếp đúng chu kỳ cho đến khi đạt maxRuns hoặc endDate, rồi lịch COMPLETED | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-041-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-FR-041-4 | ac-fr-041-4 | Tiêu chí chấp nhận AC-FR-041-4 | Lịch MONTHLY đặt ngày 29, 30 hoặc 31 chạy vào ngày cuối tháng ở tháng không đủ ngày, và ngày 29/02 ở năm nhuận | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-041-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-REV-001 | ac-rev-001 | Tiêu chí chấp nhận AC-REV-001 | Lịch định kỳ có endDate trước ngày của lần chạy đầu, hoặc endDate quá 366 ngày kể từ lần chạy đầu, thì trả 422 SCHEDULE_TIME_INVALID | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-041-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-REV-002 | ac-rev-002 | Tiêu chí chấp nhận AC-REV-002 | Lịch định kỳ có cả endDate và maxRuns kết thúc (COMPLETED) ở điều kiện đạt trước, không sinh thêm lần chạy PENDING | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-042-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-FR-042-1 | ac-fr-042-1 | Tiêu chí chấp nhận AC-FR-042-1 | GET một lịch trả status và nextRunAt | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-042-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-FR-042-2 | ac-fr-042-2 | Tiêu chí chấp nhận AC-FR-042-2 | GET danh sách lịch của tài khoản hỗ trợ lọc status và phân trang limit, offset | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-042-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-FR-042-3 | ac-fr-042-3 | Tiêu chí chấp nhận AC-FR-042-3 | GET runs trả mỗi lần chạy với scheduledAt, status, transferId và reasonCode, sắp mới nhất trước | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-042-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-FR-042-4 | ac-fr-042-4 | Tiêu chí chấp nhận AC-FR-042-4 | Người không phải chủ tài khoản nguồn nhận 403 FORBIDDEN; mã lịch không có nhận 404 SCHEDULE_NOT_FOUND | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-043-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-FR-043-1 | ac-fr-043-1 | Tiêu chí chấp nhận AC-FR-043-1 | DELETE lịch ACTIVE đổi lịch thành CANCELLED và các lần chạy PENDING thành CANCELLED | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-043-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-FR-043-2 | ac-fr-043-2 | Tiêu chí chấp nhận AC-FR-043-2 | DELETE lịch đã CANCELLED trả 200 với kết quả cũ | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-043-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-FR-043-3 | ac-fr-043-3 | Tiêu chí chấp nhận AC-FR-043-3 | DELETE khi có lần chạy RUNNING hoặc lịch COMPLETED trả 409 SCHEDULE_NOT_CANCELLABLE | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-044-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-FR-044-1 | ac-fr-044-1 | Tiêu chí chấp nhận AC-FR-044-1 | PATCH status=PAUSED trên lịch định kỳ ACTIVE đổi sang PAUSED, scheduler không chạy lịch này | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-044-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-FR-044-2 | ac-fr-044-2 | Tiêu chí chấp nhận AC-FR-044-2 | PATCH status=ACTIVE trên lịch PAUSED đổi sang ACTIVE và các lần đã bỏ lỡ trong lúc tạm dừng có trạng thái SKIPPED, không bị chạy bù | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-044-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-FR-044-3 | ac-fr-044-3 | Tiêu chí chấp nhận AC-FR-044-3 | PATCH sai trạng thái nguồn hoặc trên lịch ONCE trả 409 SCHEDULE_STATE_INVALID | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-045-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-FR-045-1 | ac-fr-045-1 | Tiêu chí chấp nhận AC-FR-045-1 | Lần chạy đến hạn tạo đúng một giao dịch COMPLETED qua luồng chuyển tiền hiện có, run SUCCEEDED có transferId | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-045-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-FR-045-2 | ac-fr-045-2 | Tiêu chí chấp nhận AC-FR-045-2 | Idempotency-Key của lần chạy đầu bằng mã lịch + thời điểm dự kiến | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-045-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-FR-045-3 | ac-fr-045-3 | Tiêu chí chấp nhận AC-FR-045-3 | Hạn mức ngày, số dư, trạng thái tài khoản và Fraud Check được kiểm tra tại lúc chạy, không phải lúc tạo lịch | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-045-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-FR-045-4 | ac-fr-045-4 | Tiêu chí chấp nhận AC-FR-045-4 | Giao dịch sinh từ lịch được tính vào hạn mức ngày và hiển thị trong lịch sử giao dịch | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-045-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-REV-004 | ac-rev-004 | Tiêu chí chấp nhận AC-REV-004 | Giao dịch sinh từ lịch hiển thị trong lịch sử giao dịch (FR-013) kèm trường scheduleId là mã lịch | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-045-SCN | SCN-ScheduledRun | SCN-ScheduledRun | scn-scheduledrun | Luồng chính SCN-ScheduledRun | 8 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-046-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-FR-046-1 | ac-fr-046-1 | Tiêu chí chấp nhận AC-FR-046-1 | FRAUD_CHECK_UNAVAILABLE được thử lại tối đa 3 lần cách 5 phút, hết lượt thì run FAILED với reasonCode | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-046-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-FR-046-2 | ac-fr-046-2 | Tiêu chí chấp nhận AC-FR-046-2 | INSUFFICIENT_FUNDS được thử lại tối đa 2 lần cách 1 giờ, hết lượt thì run FAILED | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-046-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-FR-046-3 | ac-fr-046-3 | Tiêu chí chấp nhận AC-FR-046-3 | ACCOUNT_INACTIVE, FRAUD_REJECTED, LIMIT_EXCEEDED làm run FAILED ngay, không thử lại | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-046-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-FR-046-4 | ac-fr-046-4 | Tiêu chí chấp nhận AC-FR-046-4 | Sau 3 lần chạy FAILED liên tiếp lịch định kỳ chuyển PAUSED | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-046-SCN | SCN-ScheduledRun | SCN-ScheduledRun | scn-scheduledrun | Luồng chính SCN-ScheduledRun | 8 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-047-AC | FR-047 | AC-FR-047-1 | ac-fr-047-1 | Tiêu chí chấp nhận AC-FR-047-1 | Mỗi lần chạy thành công phát sự kiện ScheduledRunCompleted có transferId qua outbox | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-047-AC | FR-047 | AC-FR-047-2 | ac-fr-047-2 | Tiêu chí chấp nhận AC-FR-047-2 | Mỗi lần chạy FAILED phát sự kiện ScheduledRunFailed có reasonCode qua outbox | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-047-AC | FR-047 | AC-FR-047-3 | ac-fr-047-3 | Tiêu chí chấp nhận AC-FR-047-3 | SMS kết quả được gửi cho người gửi; lỗi gửi SMS không đổi trạng thái lần chạy | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-047-AC | FR-047 | AC-REV-005 | ac-rev-005 | Tiêu chí chấp nhận AC-REV-005 | Sự kiện ScheduledRunCompleted/ScheduledRunFailed có đủ các trường eventId, scheduleId, runId, fromAccountId, toAccountId, amount, correlationId, kèm transferId (thành công) hoặc reasonCode (thất bại), trên cùng topic với sự kiện giao dịch | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-047-AC | FR-047 | AC-REV-006 | ac-rev-006 | Tiêu chí chấp nhận AC-REV-006 | Nội dung SMS của lần chạy thất bại nêu rõ số dư chỉ được kiểm tra lúc chạy | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-047-SCN | SCN-ScheduledRun | SCN-ScheduledRun | scn-scheduledrun | Luồng chính SCN-ScheduledRun | 8 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-048-AC | DE-AuditLog | AC-FR-048-1 | ac-fr-048-1 | Tiêu chí chấp nhận AC-FR-048-1 | Tạo, hủy, tạm dừng và tiếp tục lịch đều ghi audit với correlationId, thời điểm, tác nhân là mã tài khoản, loại sự kiện và mã lịch | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-048-AC | DE-AuditLog | AC-FR-048-2 | ac-fr-048-2 | Tiêu chí chấp nhận AC-FR-048-2 | Mọi kết quả lần chạy ghi audit với tác nhân SCHEDULER và mã lịch | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-048-SCN | SCN-ScheduledRun | SCN-ScheduledRun | scn-scheduledrun | Luồng chính SCN-ScheduledRun | 8 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-049-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-FR-049-1 | ac-fr-049-1 | Tiêu chí chấp nhận AC-FR-049-1 | Lần chạy quá hạn không quá 24 giờ sau khi hệ thống ngừng được chạy đúng một lần | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-049-AC | DE-ScheduledRun, DE-ScheduledTransfer | AC-FR-049-2 | ac-fr-049-2 | Tiêu chí chấp nhận AC-FR-049-2 | Lần chạy quá hạn quá 24 giờ chuyển MISSED, phát ScheduledRunFailed với reasonCode MISSED và không có giao dịch | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-FR-049-SCN | SCN-ScheduledRun | SCN-ScheduledRun | scn-scheduledrun | Luồng chính SCN-ScheduledRun | 8 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-NFR-010-NFR | NFR-010 | NFR-010 | at | Đúng ngưỡng 60 s | 60 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-NFR-010-NFR | NFR-010 | NFR-010 | over | Vượt ngưỡng (60+1) | 61 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-NFR-011-NFR | NFR-011 | NFR-011 | at | Đúng ngưỡng 0 giao dịch trùng | 0 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-NFR-011-NFR | NFR-011 | NFR-011 | over | Vượt ngưỡng (0+1) | 1 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-NFR-012-NFR | NFR-012 | NFR-012 | at | Đúng ngưỡng 5 chu kỳ quét | 5 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-NFR-012-NFR | NFR-012 | NFR-012 | over | Vượt ngưỡng (5+1) | 6 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-NFR-013-NFR | NFR-013 | NFR-013 | at | Đúng ngưỡng 500 ms | 500 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-NFR-013-NFR | NFR-013 | NFR-013 | over | Vượt ngưỡng (500+1) | 501 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-NFR-014-NFR | NFR-014 | NFR-014 | at | Đúng ngưỡng 100 % | 100 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-NFR-014-NFR | NFR-014 | NFR-014 | over | Vượt ngưỡng (100+1) | 101 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-Schedule-STM | SM-Schedule | TRN-Schedule-1 | trn-schedule-1 | ACTIVE -> PAUSED khi PATCH status=PAUSED | ACTIVE->PAUSED | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-Schedule-STM | SM-Schedule | CANCELLED/PATCH status=ACTIVE | inv-cancelled-patch-status-active | PATCH status=ACTIVE từ trạng thái CANCELLED không được phép | invalid | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-Schedule-STM | SM-Schedule | CANCELLED/PATCH status=PAUSED | inv-cancelled-patch-status-paused | PATCH status=PAUSED từ trạng thái CANCELLED không được phép | invalid | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-Schedule-STM | SM-Schedule | CANCELLED/lần chạy cuối kết thúc | inv-cancelled-l-n-ch-y-cu-i-k-t-th-c | lần chạy cuối kết thúc từ trạng thái CANCELLED không được phép | invalid | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-Schedule-STM | SM-Schedule | CANCELLED/scheduler: lần chạy FAILED thứ 3 liên tiếp | inv-cancelled-scheduler-l-n-ch-y-failed-th-3-li-n-ti-p | scheduler: lần chạy FAILED thứ 3 liên tiếp từ trạng thái CANCELLED không được phép | invalid | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-Schedule-STM | SM-Schedule | COMPLETED/DELETE | inv-completed-delete | DELETE từ trạng thái COMPLETED không được phép | invalid | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-Schedule-STM | SM-Schedule | COMPLETED/PATCH status=ACTIVE | inv-completed-patch-status-active | PATCH status=ACTIVE từ trạng thái COMPLETED không được phép | invalid | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-Schedule-STM | SM-Schedule | COMPLETED/PATCH status=PAUSED | inv-completed-patch-status-paused | PATCH status=PAUSED từ trạng thái COMPLETED không được phép | invalid | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-Schedule-STM | SM-Schedule | COMPLETED/lần chạy cuối kết thúc | inv-completed-l-n-ch-y-cu-i-k-t-th-c | lần chạy cuối kết thúc từ trạng thái COMPLETED không được phép | invalid | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-Schedule-STM | SM-Schedule | TRN-Schedule-2 | trn-schedule-2 | PAUSED -> ACTIVE khi PATCH status=ACTIVE | PAUSED->ACTIVE | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-Schedule-STM | SM-Schedule | TRN-Schedule-3 | trn-schedule-3 | ACTIVE -> PAUSED khi scheduler: lần chạy FAILED thứ 3 liên tiếp | ACTIVE->PAUSED | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-Schedule-STM | SM-Schedule | TRN-Schedule-4 | trn-schedule-4 | ACTIVE -> COMPLETED khi lần chạy cuối kết thúc | ACTIVE->COMPLETED | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-Schedule-STM | SM-Schedule | TRN-Schedule-5 | trn-schedule-5 | ACTIVE -> CANCELLED khi DELETE | ACTIVE->CANCELLED | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-Schedule-STM | SM-Schedule | TRN-Schedule-6 | trn-schedule-6 | PAUSED -> CANCELLED khi DELETE | PAUSED->CANCELLED | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-Schedule-STM | SM-Schedule | TRN-Schedule-7 | trn-schedule-7 | PAUSED -> COMPLETED khi PATCH status=ACTIVE | PAUSED->COMPLETED | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-Schedule-STM | SM-Schedule | ACTIVE/PATCH status=ACTIVE | inv-active-patch-status-active | PATCH status=ACTIVE từ trạng thái ACTIVE không được phép | invalid | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-Schedule-STM | SM-Schedule | CANCELLED/DELETE | inv-cancelled-delete | DELETE từ trạng thái CANCELLED không được phép | invalid | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM | SM-ScheduledRun | TRN-ScheduledRun-1 | trn-scheduledrun-1 | PENDING -> RUNNING khi scheduler chiếm lần chạy | PENDING->RUNNING | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM | SM-ScheduledRun | CANCELLED/lõi chuyển tiền trả giao dịch COMPLETED (hoặc phát lại) | inv-cancelled-l-i-chuy-n-ti-n-tr-giao-d-ch-completed-ho-c-ph-t-l-i | lõi chuyển tiền trả giao dịch COMPLETED (hoặc phát lại) từ trạng thái CANCELLED không được phép | invalid | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM | SM-ScheduledRun | CANCELLED/lõi trả lỗi | inv-cancelled-l-i-tr-l-i | lõi trả lỗi từ trạng thái CANCELLED không được phép | invalid | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM | SM-ScheduledRun | CANCELLED/lõi trả lỗi tạm thời hoặc thiếu số dư | inv-cancelled-l-i-tr-l-i-t-m-th-i-ho-c-thi-u-s-d | lõi trả lỗi tạm thời hoặc thiếu số dư từ trạng thái CANCELLED không được phép | invalid | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM | SM-ScheduledRun | CANCELLED/scheduler chiếm lần chạy | inv-cancelled-scheduler-chi-m-l-n-ch-y | scheduler chiếm lần chạy từ trạng thái CANCELLED không được phép | invalid | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM | SM-ScheduledRun | CANCELLED/scheduler phục hồi lần chạy kẹt | inv-cancelled-scheduler-ph-c-h-i-l-n-ch-y-k-t | scheduler phục hồi lần chạy kẹt từ trạng thái CANCELLED không được phép | invalid | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM | SM-ScheduledRun | CANCELLED/tiếp tục lịch PAUSED | inv-cancelled-ti-p-t-c-l-ch-paused | tiếp tục lịch PAUSED từ trạng thái CANCELLED không được phép | invalid | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM | SM-ScheduledRun | CANCELLED/đầu chu kỳ quét | inv-cancelled--u-chu-k-qu-t | đầu chu kỳ quét từ trạng thái CANCELLED không được phép | invalid | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM | SM-ScheduledRun | FAILED/DELETE lịch | inv-failed-delete-l-ch | DELETE lịch từ trạng thái FAILED không được phép | invalid | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM | SM-ScheduledRun | FAILED/lõi chuyển tiền trả giao dịch COMPLETED (hoặc phát lại) | inv-failed-l-i-chuy-n-ti-n-tr-giao-d-ch-completed-ho-c-ph-t-l-i | lõi chuyển tiền trả giao dịch COMPLETED (hoặc phát lại) từ trạng thái FAILED không được phép | invalid | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM | SM-ScheduledRun | TRN-ScheduledRun-2 | trn-scheduledrun-2 | RUNNING -> SUCCEEDED khi lõi chuyển tiền trả giao dịch COMPLETED (hoặc phát lại) | RUNNING->SUCCEEDED | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM | SM-ScheduledRun | TRN-ScheduledRun-3 | trn-scheduledrun-3 | RUNNING -> PENDING khi lõi trả lỗi tạm thời hoặc thiếu số dư | RUNNING->PENDING | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM | SM-ScheduledRun | TRN-ScheduledRun-4 | trn-scheduledrun-4 | RUNNING -> FAILED khi lõi trả lỗi | RUNNING->FAILED | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM | SM-ScheduledRun | TRN-ScheduledRun-5 | trn-scheduledrun-5 | PENDING -> SKIPPED khi tiếp tục lịch PAUSED | PENDING->SKIPPED | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM | SM-ScheduledRun | TRN-ScheduledRun-6 | trn-scheduledrun-6 | PENDING -> MISSED khi đầu chu kỳ quét | PENDING->MISSED | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM | SM-ScheduledRun | TRN-ScheduledRun-7 | trn-scheduledrun-7 | PENDING -> CANCELLED khi DELETE lịch | PENDING->CANCELLED | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM | SM-ScheduledRun | TRN-ScheduledRun-8 | trn-scheduledrun-8 | RUNNING -> PENDING khi scheduler phục hồi lần chạy kẹt | RUNNING->PENDING | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM | SM-ScheduledRun | CANCELLED/DELETE lịch | inv-cancelled-delete-l-ch | DELETE lịch từ trạng thái CANCELLED không được phép | invalid | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L3-MOD-Scheduler-BRK-INT-01 | INT-01 | failureRatePercent | trip | tỷ lệ lỗi >= 100% sau 5 lượt thì breaker mở | 100 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L3-MOD-Scheduler-BRK-INT-01 | INT-01 | openDurationSec | reclose | sau 30 giây breaker thử lại | 30 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Account | MOD-Scheduler | dependency | ok | MOD-Scheduler gọi MOD-Account thành công | MOD-Account | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Account | MOD-Scheduler | dependency | fail | MOD-Account lỗi, MOD-Scheduler xử lý lỗi có kiểm soát | MOD-Account | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Audit | MOD-Scheduler | dependency | ok | MOD-Scheduler gọi MOD-Audit thành công | MOD-Audit | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Audit | MOD-Scheduler | dependency | fail | MOD-Audit lỗi, MOD-Scheduler xử lý lỗi có kiểm soát | MOD-Audit | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Risk | MOD-Scheduler | dependency | ok | MOD-Scheduler gọi MOD-Risk thành công | MOD-Risk | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Risk | MOD-Scheduler | dependency | fail | MOD-Risk lỗi, MOD-Scheduler xử lý lỗi có kiểm soát | MOD-Risk | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Transfer | MOD-Scheduler | dependency | ok | MOD-Scheduler gọi MOD-Transfer thành công | MOD-Transfer | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Transfer | MOD-Scheduler | dependency | fail | MOD-Transfer lỗi, MOD-Scheduler xử lý lỗi có kiểm soát | MOD-Transfer | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01 | INT-01 | trigger | err-int01-5xx | ERR-INT01-5xx: HTTP 5xx là transient | transient | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01 | INT-01 | trigger | err-int01-4xx | ERR-INT01-4xx: HTTP 4xx (400, 401, 422) là permanent | permanent | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01 | INT-01 | trigger | err-int01-schema | ERR-INT01-schema: HTTP 200 nhưng response sai schema là permanent | permanent | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01 | INT-01 | trigger | err-int01-timeout | ERR-INT01-timeout: Timeout (connect/read) hoặc lỗi mạng là unknownOutcome | unknownOutcome | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L3-MOD-Scheduler-IDM-INT-01 | INT-01 | replay | replay | gửi lại cùng yêu cầu tới INT-01 (idempotent=true) | 2 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-maxPayloadKB | INT-01 | maxPayloadKB | at | đúng giới hạn maxPayloadKB = 16 KB | 16 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-maxPayloadKB | INT-01 | maxPayloadKB | over | vượt giới hạn maxPayloadKB = 16 KB | 17 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-requestsPerSecond | INT-01 | requestsPerSecond | at | đúng giới hạn requestsPerSecond = 50 req/s | 50 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-requestsPerSecond | INT-01 | requestsPerSecond | over | vượt giới hạn requestsPerSecond = 50 req/s | 51 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L3-MOD-Scheduler-RTY-INT-01 | INT-01 | maxAttempts | recover | lỗi tạm thời 1 lần rồi thành công | 2 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L3-MOD-Scheduler-RTY-INT-01 | INT-01 | maxAttempts | exhaust | lỗi tạm thời đủ 2 lần | 2 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L3-MOD-Scheduler-TMO-INT-01 | INT-01 | delayMs | under | INT-01 trả lời chậm nhưng trong readMs | 400 | không | create+cleanup |
| DATA-SCHEDULED-TRANSFER-L3-MOD-Scheduler-TMO-INT-01 | INT-01 | delayMs | over | INT-01 không trả lời trước totalMs (1000 ms) | 2000 | không | create+cleanup |

## 5. Đặc tả stub

| STUB | Đối tượng bị giả | Hành vi | Mã | Độ trễ (ms) |
| --- | --- | --- | --- | --- |
| STUB-SCHEDULED-TRANSFER-INT-01-errorPermanent-200 | INT-01 | errorPermanent | 200 |  |
| STUB-SCHEDULED-TRANSFER-INT-01-errorPermanent-400 | INT-01 | errorPermanent | 400 |  |
| STUB-SCHEDULED-TRANSFER-INT-01-errorTransient | INT-01 | errorTransient |  |  |
| STUB-SCHEDULED-TRANSFER-INT-01-errorTransient-503 | INT-01 | errorTransient | 503 |  |
| STUB-SCHEDULED-TRANSFER-INT-01-slow | INT-01 | slow |  | 400 |
| STUB-SCHEDULED-TRANSFER-INT-01-success | INT-01 | success |  |  |
| STUB-SCHEDULED-TRANSFER-INT-01-timeout | INT-01 | timeout |  | 2000 |
| STUB-SCHEDULED-TRANSFER-INT-02-success | INT-02 | success |  |  |
| STUB-SCHEDULED-TRANSFER-INT-03-success | INT-03 | success |  |  |
| STUB-SCHEDULED-TRANSFER-MOD-Account-errorTransient-503 | MOD-Account | errorTransient | 503 |  |
| STUB-SCHEDULED-TRANSFER-MOD-Account-success | MOD-Account | success |  |  |
| STUB-SCHEDULED-TRANSFER-MOD-Audit-errorTransient-503 | MOD-Audit | errorTransient | 503 |  |
| STUB-SCHEDULED-TRANSFER-MOD-Audit-success | MOD-Audit | success |  |  |
| STUB-SCHEDULED-TRANSFER-MOD-Risk-errorTransient-503 | MOD-Risk | errorTransient | 503 |  |
| STUB-SCHEDULED-TRANSFER-MOD-Risk-success | MOD-Risk | success |  |  |
| STUB-SCHEDULED-TRANSFER-MOD-Transfer-errorTransient-503 | MOD-Transfer | errorTransient | 503 |  |
| STUB-SCHEDULED-TRANSFER-MOD-Transfer-success | MOD-Transfer | success |  |  |

## 6. Yêu cầu môi trường

| ENVT | Tên | Môi trường | Dịch vụ | Công cụ | Tham chiếu bí mật (tên biến) |
| --- | --- | --- | --- | --- | --- |
| ENVT-SCHEDULED-TRANSFER-L1 | e2e | — | MOD-API, MOD-Account, MOD-Audit, MOD-Ledger, MOD-Notify, MOD-Risk, MOD-Scheduler, MOD-Transfer | pytest, httpx | — |
| ENVT-SCHEDULED-TRANSFER-L2 | nghiệp vụ | — | MOD-API, MOD-Account, MOD-Audit, MOD-Ledger, MOD-Notify, MOD-Risk, MOD-Scheduler, MOD-Transfer | pytest, httpx | — |
| ENVT-SCHEDULED-TRANSFER-L3 | cô lập service | — | — | pytest, httpx | — |

## 7. Test set và thứ tự chạy

| TS | Mục đích | Thứ tự | Điều kiện vào | Điều kiện ra | Tạm dừng |
| --- | --- | --- | --- | --- | --- |
| TS-SCHEDULED-TRANSFER-L2-FR-040-AC | useCase: FR-040 (AC) | 1 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L2-FR-041-AC | useCase: FR-041 (AC) | 2 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L2-FR-042-AC | useCase: FR-042 (AC) | 3 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L2-FR-043-AC | useCase: FR-043 (AC) | 4 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L2-FR-044-AC | useCase: FR-044 (AC) | 5 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L2-FR-045-AC | useCase: FR-045 (AC) | 6 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L2-FR-045-SCN | useCase: SCN-ScheduledRun (SCN) | 7 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L2-FR-046-AC | useCase: FR-046 (AC) | 8 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L2-FR-046-SCN | useCase: SCN-ScheduledRun (SCN) | 9 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L2-FR-047-AC | useCase: FR-047 (AC) | 10 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L2-FR-047-SCN | useCase: SCN-ScheduledRun (SCN) | 11 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L2-FR-048-AC | useCase: FR-048 (AC) | 12 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L2-FR-048-SCN | useCase: SCN-ScheduledRun (SCN) | 13 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L2-FR-049-AC | useCase: FR-049 (AC) | 14 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L2-FR-049-SCN | useCase: SCN-ScheduledRun (SCN) | 15 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API | equivalencePartitioning: EP-CancelSchedule (API) | 16 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API | equivalencePartitioning: EP-CreateSchedule (API) | 17 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API | equivalencePartitioning: EP-GetSchedule (API) | 18 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L2-EP-ListRuns-API | equivalencePartitioning: EP-ListRuns (API) | 19 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API | equivalencePartitioning: EP-ListSchedules (API) | 20 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API | equivalencePartitioning: EP-UpdateScheduleStatus (API) | 21 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L2-NFR-010-NFR | boundaryValue: NFR-010 (NFR) | 22 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L2-NFR-011-NFR | boundaryValue: NFR-011 (NFR) | 23 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L2-NFR-012-NFR | boundaryValue: NFR-012 (NFR) | 24 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L2-NFR-013-NFR | boundaryValue: NFR-013 (NFR) | 25 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L2-NFR-014-NFR | boundaryValue: NFR-014 (NFR) | 26 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM | stateTransition: SM-Schedule (STM) | 27 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM | stateTransition: SM-ScheduledRun (STM) | 28 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01 | errorGuessing: INT-01 (ERR-INT-01) | 29 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-TMO-INT-01 | boundaryValue: INT-01 (TMO-INT-01) | 30 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-RTY-INT-01 | boundaryValue: INT-01 (RTY-INT-01) | 31 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-BRK-INT-01 | decisionTable: INT-01 (BRK-INT-01) | 32 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-IDM-INT-01 | errorGuessing: INT-01 (IDM-INT-01) | 33 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-requestsPerSecond | boundaryValue: INT-01 (LIM-INT-01-requestsPerSecond) | 34 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-maxPayloadKB | boundaryValue: INT-01 (LIM-INT-01-maxPayloadKB) | 35 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Account | errorGuessing: MOD-Scheduler (DEP-MOD-Account) | 36 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Audit | errorGuessing: MOD-Scheduler (DEP-MOD-Audit) | 37 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Risk | errorGuessing: MOD-Scheduler (DEP-MOD-Risk) | 38 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Transfer | errorGuessing: MOD-Scheduler (DEP-MOD-Transfer) | 39 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |
| TS-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-JRN | useCase: SCN-ScheduledRun (JRN) | 40 | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp | mọi TestCase trong set đã chạy | stub không phản hồi hoặc dữ liệu nền không nạp được |

## 8. Ma trận phủ (TCOV → TC)

| TCOV | Test Case |
| --- | --- |
| TCOV-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-JRN-BUDGET | TC-L1-SCHEDULED-TRANSFER-SCN-ScheduledRun-JRN-BUDGET |
| TCOV-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-JRN-MAIN | TC-L1-SCHEDULED-TRANSFER-SCN-ScheduledRun-JRN-MAIN |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API-RSP-Cancel-200 | TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-200 |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API-RSP-Cancel-401 | TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-401 |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API-RSP-Cancel-403 | TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-403 |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API-RSP-Cancel-404 | TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-404 |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API-RSP-Cancel-409 | TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-409 |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-amount-ABOVE-MAX | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-amount-ABOVE-MAX |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-amount-BELOW-MIN | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-amount-BELOW-MIN |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-amount-MAX | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-amount-MAX |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-amount-MIN | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-amount-MIN |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-maxRuns-ABOVE-MAX | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-maxRuns-ABOVE-MAX |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-maxRuns-BELOW-MIN | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-maxRuns-BELOW-MIN |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-maxRuns-MAX | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-maxRuns-MAX |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-maxRuns-MIN | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-maxRuns-MIN |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-DAILY | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-DAILY |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-INVALID | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-INVALID |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-MONTHLY | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-MONTHLY |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-ONCE | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-ONCE |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-WEEKLY | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-WEEKLY |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-200 | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-200 |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-201 | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-201 |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-400 | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-400 |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-401 | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-401 |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-403 | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-403 |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-404 | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-404 |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-409 | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-409 |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-422-amount | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-422-amount |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-422-limit | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-422-limit |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-422-same | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-422-same |
| TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-422-time | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-422-time |
| TCOV-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API-RSP-Get-200 | TC-L2-SCHEDULED-TRANSFER-EP-GetSchedule-API-RSP-Get-200 |
| TCOV-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API-RSP-Get-401 | TC-L2-SCHEDULED-TRANSFER-EP-GetSchedule-API-RSP-Get-401 |
| TCOV-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API-RSP-Get-403 | TC-L2-SCHEDULED-TRANSFER-EP-GetSchedule-API-RSP-Get-403 |
| TCOV-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API-RSP-Get-404 | TC-L2-SCHEDULED-TRANSFER-EP-GetSchedule-API-RSP-Get-404 |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-FLD-ListRuns-limit-ABOVE-MAX | TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-limit-ABOVE-MAX |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-FLD-ListRuns-limit-BELOW-MIN | TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-limit-BELOW-MIN |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-FLD-ListRuns-limit-MAX | TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-limit-MAX |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-FLD-ListRuns-limit-MIN | TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-limit-MIN |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-FLD-ListRuns-offset-BELOW-MIN | TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-offset-BELOW-MIN |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-FLD-ListRuns-offset-MIN | TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-offset-MIN |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-RSP-ListRuns-200 | TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-RSP-ListRuns-200 |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-RSP-ListRuns-401 | TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-RSP-ListRuns-401 |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-RSP-ListRuns-403 | TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-RSP-ListRuns-403 |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-RSP-ListRuns-404 | TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-RSP-ListRuns-404 |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-limit-ABOVE-MAX | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-limit-ABOVE-MAX |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-limit-BELOW-MIN | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-limit-BELOW-MIN |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-limit-MAX | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-limit-MAX |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-limit-MIN | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-limit-MIN |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-offset-BELOW-MIN | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-offset-BELOW-MIN |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-offset-MIN | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-offset-MIN |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-ACTIVE | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-ACTIVE |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-CANCELLED | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-CANCELLED |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-COMPLETED | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-COMPLETED |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-INVALID | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-INVALID |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-PAUSED | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-PAUSED |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-RSP-ListSchedules-200 | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-RSP-ListSchedules-200 |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-RSP-ListSchedules-400 | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-RSP-ListSchedules-400 |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-RSP-ListSchedules-401 | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-RSP-ListSchedules-401 |
| TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-RSP-ListSchedules-403 | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-RSP-ListSchedules-403 |
| TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-ACTIVE | TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-ACTIVE |
| TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-INVALID | TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-INVALID |
| TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-PAUSED | TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-PAUSED |
| TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-RSP-Patch-200 | TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-200 |
| TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-RSP-Patch-400 | TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-400 |
| TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-RSP-Patch-401 | TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-401 |
| TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-RSP-Patch-403 | TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-403 |
| TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-RSP-Patch-404 | TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-404 |
| TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-RSP-Patch-409 | TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-409 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-040-AC-AC-FR-040-1 | TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-1 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-040-AC-AC-FR-040-2 | TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-2 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-040-AC-AC-FR-040-3 | TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-3 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-040-AC-AC-FR-040-4 | TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-4 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-040-AC-AC-FR-040-5 | TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-5 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-040-AC-AC-FR-040-6 | TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-6 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-040-AC-AC-REV-003 | TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-REV-003 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-041-AC-AC-FR-041-1 | TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-FR-041-1 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-041-AC-AC-FR-041-2 | TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-FR-041-2 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-041-AC-AC-FR-041-3 | TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-FR-041-3 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-041-AC-AC-FR-041-4 | TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-FR-041-4 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-041-AC-AC-REV-001 | TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-REV-001 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-041-AC-AC-REV-002 | TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-REV-002 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-042-AC-AC-FR-042-1 | TC-L2-SCHEDULED-TRANSFER-FR-042-AC-AC-FR-042-1 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-042-AC-AC-FR-042-2 | TC-L2-SCHEDULED-TRANSFER-FR-042-AC-AC-FR-042-2 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-042-AC-AC-FR-042-3 | TC-L2-SCHEDULED-TRANSFER-FR-042-AC-AC-FR-042-3 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-042-AC-AC-FR-042-4 | TC-L2-SCHEDULED-TRANSFER-FR-042-AC-AC-FR-042-4 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-043-AC-AC-FR-043-1 | TC-L2-SCHEDULED-TRANSFER-FR-043-AC-AC-FR-043-1 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-043-AC-AC-FR-043-2 | TC-L2-SCHEDULED-TRANSFER-FR-043-AC-AC-FR-043-2 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-043-AC-AC-FR-043-3 | TC-L2-SCHEDULED-TRANSFER-FR-043-AC-AC-FR-043-3 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-044-AC-AC-FR-044-1 | TC-L2-SCHEDULED-TRANSFER-FR-044-AC-AC-FR-044-1 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-044-AC-AC-FR-044-2 | TC-L2-SCHEDULED-TRANSFER-FR-044-AC-AC-FR-044-2 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-044-AC-AC-FR-044-3 | TC-L2-SCHEDULED-TRANSFER-FR-044-AC-AC-FR-044-3 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-045-AC-AC-FR-045-1 | TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-FR-045-1 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-045-AC-AC-FR-045-2 | TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-FR-045-2 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-045-AC-AC-FR-045-3 | TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-FR-045-3 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-045-AC-AC-FR-045-4 | TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-FR-045-4 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-045-AC-AC-REV-004 | TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-REV-004 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-045-SCN-SCN-ScheduledRun | TC-L2-SCHEDULED-TRANSFER-FR-045-SCN-SCN-ScheduledRun |
| TCOV-SCHEDULED-TRANSFER-L2-FR-046-AC-AC-FR-046-1 | TC-L2-SCHEDULED-TRANSFER-FR-046-AC-AC-FR-046-1 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-046-AC-AC-FR-046-2 | TC-L2-SCHEDULED-TRANSFER-FR-046-AC-AC-FR-046-2 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-046-AC-AC-FR-046-3 | TC-L2-SCHEDULED-TRANSFER-FR-046-AC-AC-FR-046-3 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-046-AC-AC-FR-046-4 | TC-L2-SCHEDULED-TRANSFER-FR-046-AC-AC-FR-046-4 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-046-SCN-SCN-ScheduledRun | TC-L2-SCHEDULED-TRANSFER-FR-046-SCN-SCN-ScheduledRun |
| TCOV-SCHEDULED-TRANSFER-L2-FR-047-AC-AC-FR-047-1 | TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-FR-047-1 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-047-AC-AC-FR-047-2 | TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-FR-047-2 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-047-AC-AC-FR-047-3 | TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-FR-047-3 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-047-AC-AC-REV-005 | TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-REV-005 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-047-AC-AC-REV-006 | TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-REV-006 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-047-SCN-SCN-ScheduledRun | TC-L2-SCHEDULED-TRANSFER-FR-047-SCN-SCN-ScheduledRun |
| TCOV-SCHEDULED-TRANSFER-L2-FR-048-AC-AC-FR-048-1 | TC-L2-SCHEDULED-TRANSFER-FR-048-AC-AC-FR-048-1 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-048-AC-AC-FR-048-2 | TC-L2-SCHEDULED-TRANSFER-FR-048-AC-AC-FR-048-2 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-048-SCN-SCN-ScheduledRun | TC-L2-SCHEDULED-TRANSFER-FR-048-SCN-SCN-ScheduledRun |
| TCOV-SCHEDULED-TRANSFER-L2-FR-049-AC-AC-FR-049-1 | TC-L2-SCHEDULED-TRANSFER-FR-049-AC-AC-FR-049-1 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-049-AC-AC-FR-049-2 | TC-L2-SCHEDULED-TRANSFER-FR-049-AC-AC-FR-049-2 |
| TCOV-SCHEDULED-TRANSFER-L2-FR-049-SCN-SCN-ScheduledRun | TC-L2-SCHEDULED-TRANSFER-FR-049-SCN-SCN-ScheduledRun |
| TCOV-SCHEDULED-TRANSFER-L2-NFR-010-NFR-AT | TC-L2-SCHEDULED-TRANSFER-NFR-010-NFR-AT |
| TCOV-SCHEDULED-TRANSFER-L2-NFR-010-NFR-OVER | TC-L2-SCHEDULED-TRANSFER-NFR-010-NFR-OVER |
| TCOV-SCHEDULED-TRANSFER-L2-NFR-011-NFR-AT | TC-L2-SCHEDULED-TRANSFER-NFR-011-NFR-AT |
| TCOV-SCHEDULED-TRANSFER-L2-NFR-011-NFR-OVER | TC-L2-SCHEDULED-TRANSFER-NFR-011-NFR-OVER |
| TCOV-SCHEDULED-TRANSFER-L2-NFR-012-NFR-AT | TC-L2-SCHEDULED-TRANSFER-NFR-012-NFR-AT |
| TCOV-SCHEDULED-TRANSFER-L2-NFR-012-NFR-OVER | TC-L2-SCHEDULED-TRANSFER-NFR-012-NFR-OVER |
| TCOV-SCHEDULED-TRANSFER-L2-NFR-013-NFR-AT | TC-L2-SCHEDULED-TRANSFER-NFR-013-NFR-AT |
| TCOV-SCHEDULED-TRANSFER-L2-NFR-013-NFR-OVER | TC-L2-SCHEDULED-TRANSFER-NFR-013-NFR-OVER |
| TCOV-SCHEDULED-TRANSFER-L2-NFR-014-NFR-AT | TC-L2-SCHEDULED-TRANSFER-NFR-014-NFR-AT |
| TCOV-SCHEDULED-TRANSFER-L2-NFR-014-NFR-OVER | TC-L2-SCHEDULED-TRANSFER-NFR-014-NFR-OVER |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-ACTIVE-PATCH-status-ACTIVE | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-ACTIVE-PATCH-status-ACTIVE |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-CANCELLED-DELETE | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-DELETE |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-CANCELLED-PATCH-status-ACTIVE | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-PATCH-status-ACTIVE |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-CANCELLED-PATCH-status-PAUSED | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-PATCH-status-PAUSED |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-CANCELLED-l-n-ch-y-cu-i-k-t-th-c | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-l-n-ch-y-cu-i-k-t-th-c |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-CANCELLED-scheduler-l-n-ch-y-FAILED-th-3-li-n-ti-p | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-scheduler-l-n-ch-y-FAILED-th-3-li-n-ti-p |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-COMPLETED-DELETE | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-COMPLETED-DELETE |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-COMPLETED-PATCH-status-ACTIVE | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-COMPLETED-PATCH-status-ACTIVE |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-COMPLETED-PATCH-status-PAUSED | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-COMPLETED-PATCH-status-PAUSED |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-COMPLETED-l-n-ch-y-cu-i-k-t-th-c | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-COMPLETED-l-n-ch-y-cu-i-k-t-th-c |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-TRN-Schedule-1 | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-1 |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-TRN-Schedule-2 | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-2 |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-TRN-Schedule-3 | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-3 |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-TRN-Schedule-4 | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-4 |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-TRN-Schedule-5 | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-5 |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-TRN-Schedule-6 | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-6 |
| TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-TRN-Schedule-7 | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-7 |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED--u-chu-k-qu-t | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED--u-chu-k-qu-t |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED-DELETE-l-ch | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-DELETE-l-ch |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED-l-i-chuy-n-ti-n-tr-giao-d-ch-COMPLETED-ho-c-ph-t-l-i | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-l-i-chuy-n-ti-n-tr-giao-d-ch-COMPLETED-ho-c-ph-t-l-i |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED-l-i-tr-l-i | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-l-i-tr-l-i |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED-l-i-tr-l-i-t-m-th-i-ho-c-thi-u-s-d | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-l-i-tr-l-i-t-m-th-i-ho-c-thi-u-s-d |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED-scheduler-chi-m-l-n-ch-y | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-scheduler-chi-m-l-n-ch-y |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED-scheduler-ph-c-h-i-l-n-ch-y-k-t | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-scheduler-ph-c-h-i-l-n-ch-y-k-t |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED-ti-p-t-c-l-ch-PAUSED | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-ti-p-t-c-l-ch-PAUSED |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-FAILED-DELETE-l-ch | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-FAILED-DELETE-l-ch |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-FAILED-l-i-chuy-n-ti-n-tr-giao-d-ch-COMPLETED-ho-c-ph-t-l-i | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-FAILED-l-i-chuy-n-ti-n-tr-giao-d-ch-COMPLETED-ho-c-ph-t-l-i |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-1 | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-1 |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-2 | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-2 |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-3 | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-3 |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-4 | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-4 |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-5 | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-5 |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-6 | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-6 |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-7 | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-7 |
| TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-8 | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-8 |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-BRK-INT-01-RECLOSE | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-BRK-INT-01-RECLOSE |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-BRK-INT-01-TRIP | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-BRK-INT-01-TRIP |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Account-FAIL | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Account-FAIL |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Account-OK | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Account-OK |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Audit-FAIL | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Audit-FAIL |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Audit-OK | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Audit-OK |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Risk-FAIL | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Risk-FAIL |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Risk-OK | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Risk-OK |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Transfer-FAIL | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Transfer-FAIL |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Transfer-OK | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Transfer-OK |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01-ERR-INT01-4xx | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-ERR-INT-01-ERR-INT01-4xx |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01-ERR-INT01-5xx | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-ERR-INT-01-ERR-INT01-5xx |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01-ERR-INT01-schema | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-ERR-INT-01-ERR-INT01-schema |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01-ERR-INT01-timeout | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-ERR-INT-01-ERR-INT01-timeout |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-IDM-INT-01-REPLAY | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-IDM-INT-01-REPLAY |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-maxPayloadKB-AT | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-LIM-INT-01-maxPayloadKB-AT |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-maxPayloadKB-OVER | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-LIM-INT-01-maxPayloadKB-OVER |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-requestsPerSecond-AT | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-LIM-INT-01-requestsPerSecond-AT |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-requestsPerSecond-OVER | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-LIM-INT-01-requestsPerSecond-OVER |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-RTY-INT-01-EXHAUST | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-RTY-INT-01-EXHAUST |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-RTY-INT-01-RECOVER | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-RTY-INT-01-RECOVER |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-TMO-INT-01-OVER | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-TMO-INT-01-OVER |
| TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-TMO-INT-01-UNDER | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-TMO-INT-01-UNDER |

## Lịch sử phiên bản

| Phiên bản | Trạng thái | Baseline TDD | CR | Nút thêm | Nút đổi | Nút bỏ |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | Draft | b-a31cdc541d67 | CR-001 | 1555 | 0 | 0 |

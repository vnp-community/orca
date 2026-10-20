---
spec: TSD-SCHEDULED-TRANSFER-TestCases
feature: SCHEDULED-TRANSFER
docKind: TestCases
version: 1
status: Baselined
supersedes: none
baseline: b-bd72d688fca2
changes: [CR-001]
contentHash: d1163c5fe538cfd3c388e912202f099c11ffa5d9f0bc998e9089ad4efb699ea5
---
# Test Cases: SCHEDULED-TRANSFER

## 0. Thông tin tài liệu

| Trường | Giá trị |
| --- | --- |
| id | TSD-SCHEDULED-TRANSFER-TestCases_v1 |
| Feature | SCHEDULED-TRANSFER |
| Phiên bản | 1 |
| Trạng thái | Baselined |
| Baseline TDD | DOC-SRS-CR001 (b-bd72d688fca2) |
| CR của phiên bản này | CR-001 |
| Ontology | tso v1 |

## 1. Quy ước

- Bước: given (chuẩn bị), when (hành động), then (quan sát), cleanup (dọn).
- Kết quả mong đợi của assertion lấy từ oracle; dữ liệu nhạy cảm chỉ ghi `secretRef:<tên>`.

## 2. Test Cases theo mức

### 2.1 L1: Nghiệp vụ đầu-cuối

#### TC-L1-SCHEDULED-TRANSFER-SCN-ScheduledRun-errorHandling-fail-INT-01-c

Chứng minh khi Fraud Check (INT-01) lỗi (timeout, 5xx, 4xx hoặc sai schema) trong lần chạy lịch đến hạn, hệ thống áp dụng dự phòng FB-INT01 theo số tiền của lần chạy.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-errorHandling-fail-INT-01-c` | `TSET-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun` | llm/approved |

Tiền điều kiện:

- Lần chạy đã được chiếm và có correlationId
- Fraud Check (INT-01) không khả dụng: timeout, 5xx, 4xx hoặc HTTP 200 sai schema
- Có hai lần chạy: một dưới 5.000.000 VND và một từ 5.000.000 VND (gồm đúng 5.000.000)

Fixture: `TFX-SCHEDULED-TRANSFER-SCN-ScheduledRun-1d15a6` Tạo lần chạy đã được chiếm có correlationId; đặt Fraud Check ở trạng thái không khả dụng (timeout, 5xx, 4xx hoặc HTTP 200 sai schema). / dọn: Xóa dữ liệu lần chạy, giao dịch và khôi phục Fraud Check do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Cấu hình stub Fraud Check trả lỗi (lần lượt timeout, 5xx, 4xx, HTTP 200 sai schema); tạo lần chạy đã chiếm có correlationId với số tiền 4.999.999 VND và lần chạy khác với số tiền 5.000.000 VND. | {"amounts": [4999999, 5000000], "faults": ["timeout", "5xx", "4xx", "schema"]} |  |
| 2 | when | Lõi chuyển tiền gọi Fraud Check, lần gọi cuối (tối đa 2 lần, timeout 1000 ms mỗi lần) vẫn lỗi, áp dụng FB-INT01 theo số tiền. | {"maxCalls": 2, "timeoutMs": 1000} | SCN-ScheduledRun |
| 3 | then | Kiểm tra lần chạy dưới 5.000.000 VND: riskCheck=SKIPPED, giao dịch COMPLETED. Kiểm tra lần chạy từ 5.000.000 VND: FRAUD_CHECK_UNAVAILABLE (503), không có giao dịch, thử lại tối đa 3 lần cách 5 phút rồi FAILED kèm reasonCode. |  |  |
| 4 | cleanup | Xóa lần chạy, giao dịch và cấu hình stub do kiểm thử tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | Dưới 5.000.000 VND: riskCheck=SKIPPED, giao dịch COMPLETED. Từ 5.000.000 VND (kể cả đúng 5.000.000): FRAUD_CHECK_UNAVAILABLE (503), không có giao dịch, thử lại tối đa 3 lần cách 5 phút rồi FAILED với reasonCode. |  | `TORC-SCHEDULED-TRANSFER-FB-INT01-fail-SCN-ScheduledRun` |

#### TC-L1-SCHEDULED-TRANSFER-SCN-ScheduledRun-functional-main-c

Chứng minh luồng chính: scheduler quét mỗi 30 giây, chiếm lần chạy đến hạn bằng UPDATE có điều kiện, sinh correlationId riêng, gọi lõi chuyển tiền với Idempotency-Key = mã lịch + thời điểm dự kiến và cập nhật lần chạy.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-functional-main-c` | `TSET-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun` | llm/approved |

Tiền điều kiện:

- Lịch ACTIVE có lần chạy đến hạn (thời điểm dự kiến ≤ hiện tại)
- Tài khoản đủ số dư và hạn mức
- Fraud Check khả dụng

Fixture: `TFX-SCHEDULED-TRANSFER-SCN-ScheduledRun-a0c05b` Tạo lịch ACTIVE có lần chạy đến hạn, tài khoản đủ số dư và hạn mức, bảo đảm Fraud Check khả dụng. / dọn: Xóa dữ liệu lịch, lần chạy, giao dịch do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE có một lần chạy đến hạn, tài khoản đủ số dư và hạn mức, Fraud Check khả dụng. |  |  |
| 2 | when | Chờ chu kỳ quét 30 giây của MOD-Scheduler tìm thấy lần chạy có thời điểm dự kiến ≤ hiện tại, chiếm lần chạy, gọi lõi chuyển tiền và cập nhật run. | {"maxRunsPerScan": 20, "scanIntervalSec": 30} | SCN-ScheduledRun |
| 3 | then | Kiểm tra có 1 giao dịch COMPLETED, run SUCCEEDED có transferId, 1 bản ghi audit tác nhân SCHEDULER, 1 sự kiện ScheduledRunCompleted trong outbox; giao dịch tính vào hạn mức ngày và hiện trong lịch sử. |  |  |
| 4 | cleanup | Xóa lịch, lần chạy, giao dịch, audit và sự kiện outbox do kiểm thử tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | 1 giao dịch COMPLETED, run SUCCEEDED có transferId, 1 bản ghi audit tác nhân SCHEDULER, 1 sự kiện ScheduledRunCompleted trong outbox; giao dịch tính vào hạn mức ngày và hiện trong lịch sử. |  | `TORC-SCHEDULED-TRANSFER-SCN-ScheduledRun-main` |

#### TC-L1-SCHEDULED-TRANSFER-SCN-ScheduledRun-timing-budget-c

Chứng minh một lần xử lý luồng Chạy lịch đến hạn, từ bước chiếm đến bước ghi audit/sự kiện, hoàn tất trong ngân sách 60000 ms kể cả khi Fraud Check ở trường hợp xấu nhất.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-timing-budget-c` | `TSET-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun` | llm/approved |

Tiền điều kiện:

- Một chu kỳ quét có tối đa 20 lần chạy đến hạn
- Fraud Check chậm hoặc lỗi
- Đồng hồ đo thời gian bắt đầu từ bước chiếm lần chạy

Fixture: `TFX-SCHEDULED-TRANSFER-SCN-ScheduledRun-01a089` Tạo một chu kỳ quét có 20 lần chạy đến hạn; cấu hình Fraud Check chậm hoặc lỗi. / dọn: Xóa dữ liệu lần chạy và khôi phục Fraud Check do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo 20 lần chạy đến hạn trong một chu kỳ quét và cấu hình Fraud Check chậm hoặc lỗi. |  |  |
| 2 | when | Để lần chạy được chiếm và đi qua toàn bộ các bước của kịch bản; đo thời gian từ lúc chiếm đến lúc ghi audit/sự kiện. | {"runsDue": 20} | SCN-ScheduledRun |
| 3 | then | Kiểm tra thời gian xử lý một lần chạy ≤ 60000 ms. |  |  |
| 4 | cleanup | Xóa các lần chạy và dữ liệu liên quan do kiểm thử tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | ≤ 60000 ms | ms | `TORC-SCHEDULED-TRANSFER-SCN-ScheduledRun-budget` |

#### TC-L1-SCHEDULED-TRANSFER-SCN-ScheduledRun-timing-worst-INT-01-c

Chứng minh trường hợp xấu nhất khi gọi Fraud Check (2 lần × 1000 ms + backoff 0 ms = 2000 ms) nằm trong ngân sách 60000 ms của kịch bản.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-timing-worst-INT-01-c` | `TSET-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun` | llm/approved |

Tiền điều kiện:

- Fraud Check timeout ở cả hai lần gọi
- Retry dùng cùng transactionId, backoff 0 ms

Fixture: `TFX-SCHEDULED-TRANSFER-SCN-ScheduledRun-dad1e6` Cấu hình Fraud Check timeout ở cả hai lần gọi. / dọn: Gỡ cấu hình timeout và xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Cấu hình stub Fraud Check không phản hồi để timeout 1000 ms ở cả hai lần gọi. |  |  |
| 2 | when | Lần gọi đầu timeout 1000 ms, retry một lần cùng transactionId, lần hai timeout 1000 ms; đo tổng thời gian gọi. | {"backoffMs": 0, "calls": 2, "timeoutMs": 1000} | SCN-ScheduledRun |
| 3 | then | Kiểm tra tổng thời gian gọi Fraud Check là 2000 ms, không vượt ngân sách 60000 ms, sau đó FB-INT01 được áp dụng. |  |  |
| 4 | cleanup | Xóa dữ liệu lần chạy và gỡ cấu hình stub do kiểm thử tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | 2000 ms ≤ 60000 ms (ngân sách của SCN-ScheduledRun) | ms | `TORC-SCHEDULED-TRANSFER-SCN-ScheduledRun-worst-INT-01` |

### 2.2 L2: Nghiệp vụ

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-040-1-functional-c

Chứng minh tạo lịch ONCE hợp lệ trả 201 với scheduleId không rỗng và status=ACTIVE.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-040-1-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-040` | llm/approved |

Tiền điều kiện:

- API key hợp lệ: secretRef:api_key_owner_a
- Hai tài khoản nguồn/đích tồn tại và khác nhau
- Tài khoản nguồn có dưới 20 lịch ACTIVE/PAUSED

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-89e2d4` Tạo API key hợp lệ, hai tài khoản khác nhau, tài khoản nguồn có dưới 20 lịch ACTIVE/PAUSED. / dọn: Xóa lịch, tài khoản và API key do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị API key hợp lệ, hai tài khoản khác nhau, tài khoản nguồn có dưới 20 lịch ACTIVE/PAUSED. |  |  |
| 2 | when | Gửi POST /v1/scheduled-transfers tạo lịch ONCE với firstRunAt hợp lệ, amount trong 1..50.000.000 và Idempotency-Key mới. | {"Idempotency-Key": "key-0001", "amount": 1000000, "firstRunAt": "hiện tại + 1 ngày", "fromAccountId": "ACC-A", "recurrence": "ONCE", "toAccountId": "ACC-B"} | EP-CreateSchedule |
| 3 | then | Kiểm tra HTTP 201, body có scheduleId không rỗng và status=ACTIVE; bản ghi lịch được lưu ACTIVE. |  |  |
| 4 | cleanup | Xóa lịch và dữ liệu fixture đã tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 201; body có scheduleId không rỗng và status=ACTIVE. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-040-1` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-040-2-functional-c

Chứng minh việc tạo lịch chỉ đăng ký lịch, không chuyển tiền và không ghi ledger tại thời điểm tạo.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-040-2-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-040` | llm/approved |

Tiền điều kiện:

- API key hợp lệ: secretRef:api_key_owner_a
- Hai tài khoản tồn tại với số dư đã biết

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-07b555` Ghi lại số dư hai tài khoản và số bản ghi ledger_entries làm mốc trước khi gọi. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Ghi lại số dư của fromAccountId, toAccountId và số bản ghi ledger_entries trước khi gọi. |  |  |
| 2 | when | Gửi POST tạo lịch ONCE hợp lệ và nhận 201. | {"Idempotency-Key": "key-0002", "amount": 500000, "firstRunAt": "hiện tại + 1 ngày", "recurrence": "ONCE"} | EP-CreateSchedule |
| 3 | then | Đọc lại số dư hai tài khoản và số bản ghi ledger_entries, so sánh với giá trị trước. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Số dư hai tài khoản không đổi; số bản ghi ledger_entries sau bằng số trước. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-040-2` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-040-3-functional-c

Chứng minh firstRunAt ngoài khoảng 5 phút đến 90 ngày bị từ chối, kiểm tra biên quanh 5 phút và 90 ngày.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-040-3-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-040` | llm/approved |

Tiền điều kiện:

- Đồng hồ hệ thống cố định, thời điểm hiện tại đã biết
- Các trường khác hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-b6fdd4` Cố định đồng hồ hệ thống và chuẩn bị yêu cầu hợp lệ khác. / dọn: Khôi phục đồng hồ và xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Cố định đồng hồ hệ thống và chuẩn bị yêu cầu hợp lệ ngoại trừ firstRunAt. |  |  |
| 2 | when | Gửi POST với firstRunAt = hiện tại + 4 phút. | {"firstRunAt": "now+4m"} | EP-CreateSchedule |
| 3 | when | Gửi POST với firstRunAt = hiện tại + 91 ngày. | {"firstRunAt": "now+91d"} | EP-CreateSchedule |
| 4 | when | Gửi POST với các giá trị sát biên 5 phút và 90 ngày để xác định ranh giới chấp nhận/từ chối. | {"firstRunAt": ["now+5m", "now+90d"]} | EP-CreateSchedule |
| 5 | then | Kiểm tra giá trị ngoài khoảng trả 422 SCHEDULE_TIME_INVALID và không có bản ghi lịch mới. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 422, errorCode=SCHEDULE_TIME_INVALID cho giá trị ngoài khoảng; không có bản ghi lịch mới. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-040-3` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-040-4-functional-c

Chứng minh toAccountId không tồn tại bị từ chối với 404 ACCOUNT_NOT_FOUND.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-040-4-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-040` | llm/approved |

Tiền điều kiện:

- fromAccountId thuộc người gọi
- toAccountId là mã chưa từng được tạo

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-21ecc8` Tạo tài khoản nguồn thuộc người gọi; đảm bảo mã tài khoản đích chưa tồn tại. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị tài khoản nguồn của người gọi và một toAccountId chưa từng tồn tại. |  |  |
| 2 | when | Gửi POST tạo lịch với toAccountId không tồn tại. | {"Idempotency-Key": "key-0004", "fromAccountId": "ACC-A", "toAccountId": "ACC-NOT-EXIST-999"} | EP-CreateSchedule |
| 3 | then | Kiểm tra HTTP 404, errorCode=ACCOUNT_NOT_FOUND và không có bản ghi lịch mới. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 404, errorCode=ACCOUNT_NOT_FOUND; không có bản ghi lịch mới. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-040-4` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-040-5-functional-c

Chứng minh fromAccountId không thuộc chủ API key bị từ chối với 403 FORBIDDEN.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-040-5-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-040` | llm/approved |

Tiền điều kiện:

- API key hợp lệ của tài khoản A: secretRef:api_key_owner_a
- Tài khoản B tồn tại và thuộc người khác

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-7a9282` Tạo API key hợp lệ của A, tài khoản B thuộc người khác. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị API key của A và tài khoản B thuộc người dùng khác. |  |  |
| 2 | when | Gửi POST tạo lịch với API key của A và fromAccountId là tài khoản B. | {"Authorization": "secretRef:api_key_owner_a", "Idempotency-Key": "key-0005", "fromAccountId": "ACC-B", "toAccountId": "ACC-C"} | EP-CreateSchedule |
| 3 | then | Kiểm tra HTTP 403, errorCode=FORBIDDEN và không có bản ghi lịch mới. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 403, errorCode=FORBIDDEN; không có bản ghi lịch mới. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-040-5` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-040-6-functional-c

Chứng minh biên số lịch: lịch thứ 20 thành công, lịch thứ 21 bị từ chối SCHEDULE_LIMIT_REACHED.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-040-6-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-040` | llm/approved |

Tiền điều kiện:

- Tài khoản nguồn có 19 lịch ACTIVE/PAUSED ban đầu rồi đạt 20

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-b6510f` Tạo tài khoản nguồn với số lượng lịch ACTIVE/PAUSED đúng như kịch bản biên. / dọn: Xóa các lịch và dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị tài khoản nguồn có 19 lịch ACTIVE/PAUSED. |  |  |
| 2 | when | Tạo lịch ONCE hợp lệ thứ 20 (biên 19→20) và xác nhận thành công. | {"Idempotency-Key": "key-0006a", "recurrence": "ONCE"} | EP-CreateSchedule |
| 3 | when | Tạo lịch ONCE hợp lệ thứ 21 cho cùng fromAccountId. | {"Idempotency-Key": "key-0006b", "recurrence": "ONCE"} | EP-CreateSchedule |
| 4 | then | Kiểm tra lịch thứ 21 trả HTTP 422 SCHEDULE_LIMIT_REACHED và số lịch ACTIVE/PAUSED vẫn là 20. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 422, errorCode=SCHEDULE_LIMIT_REACHED; số lịch ACTIVE/PAUSED của tài khoản vẫn là 20. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-040-6` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-041-1-functional-c

Chứng minh tạo lịch định kỳ hợp lệ (DAILY, WEEKLY, MONTHLY; chỉ endDate, chỉ maxRuns hoặc cả hai) được chấp nhận với HTTP 201 và status=ACTIVE.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-041-1-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-041` | llm/approved |

Tiền điều kiện:

- Người gọi có API key hợp lệ (secretRef:caller_api_key)
- fromAccountId thuộc người gọi, toAccountId tồn tại và khác from
- Tài khoản có dưới 20 lịch ACTIVE/PAUSED

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-600d35` Tạo người gọi với API key hợp lệ, tài khoản nguồn của người gọi, tài khoản đích khác nguồn; tài khoản nguồn có dưới 20 lịch ACTIVE/PAUSED. / dọn: Xóa các lịch, lần chạy và tài khoản do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị tài khoản nguồn của người gọi và tài khoản đích khác nguồn; tài khoản nguồn có dưới 20 lịch ACTIVE/PAUSED. |  |  |
| 2 | when | Gửi POST /v1/scheduled-transfers với Idempotency-Key mới, lặp cho từng recurrence DAILY, WEEKLY, MONTHLY và từng cách đặt giới hạn (chỉ endDate, chỉ maxRuns, cả hai). | recurrence=DAILY\|WEEKLY\|MONTHLY; maxRuns=3; endDate trong 366 ngày; firstRunAt=hiện tại+1 ngày; Authorization=secretRef:caller_api_key | EP-CreateSchedule |
| 3 | then | Kiểm tra mỗi phản hồi có HTTP 201, status=ACTIVE và có mã lịch; lịch được lưu cùng lần chạy đầu PENDING. |  |  |
| 4 | cleanup | Xóa các lịch đã tạo trong test. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 201, status=ACTIVE, có mã lịch trong phản hồi; kiểm tra cho từng recurrence và từng cách đặt giới hạn (chỉ endDate, chỉ maxRuns, cả hai). |  | `TORC-SCHEDULED-TRANSFER-AC-FR-041-1` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-041-2-functional-c

Chứng minh yêu cầu tạo lịch định kỳ thiếu cả endDate và maxRuns bị từ chối 400 INVALID_REQUEST ở bước kiểm tra đầu vào.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-041-2-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-041` | llm/approved |

Tiền điều kiện:

- Người gọi xác thực hợp lệ
- Các trường khác hợp lệ
- Idempotency-Key có mặt

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-3b455e` Tạo người gọi xác thực hợp lệ với các trường còn lại hợp lệ và Idempotency-Key sẵn sàng. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Ghi lại số bản ghi scheduled_transfers hiện có của tài khoản nguồn. |  |  |
| 2 | when | Gửi POST /v1/scheduled-transfers với recurrence DAILY, WEEKLY hoặc MONTHLY, không có endDate và không có maxRuns. | recurrence=WEEKLY; không có endDate; không có maxRuns; Idempotency-Key=key-moi-001 | EP-CreateSchedule |
| 3 | then | Kiểm tra phản hồi HTTP 400 errorCode=INVALID_REQUEST và số bản ghi scheduled_transfers không đổi. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 400, errorCode=INVALID_REQUEST; số bản ghi scheduled_transfers của tài khoản không đổi. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-041-2` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-041-3-functional-c

Chứng minh Scheduler sinh lần chạy PENDING kế tiếp đúng chu kỳ đến khi đạt maxRuns hoặc vượt endDate, rồi đặt lịch COMPLETED.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-041-3-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-041` | llm/approved |

Tiền điều kiện:

- Lịch ACTIVE định kỳ có maxRuns (ví dụ 3) hoặc endDate
- Lần chạy hiện tại đến hạn và được chiếm bởi scan_and_run

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-b178ba` Tạo lịch ACTIVE định kỳ có maxRuns hoặc endDate, lần chạy hiện tại đến hạn và được scan_and_run chiếm. / dọn: Xóa các lịch và lần chạy do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE định kỳ với maxRuns=3 (và một lịch khác dùng endDate) có lần chạy đến hạn. |  |  |
| 2 | when | Cho Scheduler quét (chu kỳ 30 giây) và hoàn tất lần chạy hiện tại, lặp đến khi không còn lần chạy mới. | maxRuns=3; recurrence=DAILY; chu kỳ quét 30 giây | EP-CreateSchedule |
| 3 | then | Kiểm tra số lần chạy, khoảng cách thời điểm đúng chu kỳ, không có lần chạy sau endDate và lịch chuyển COMPLETED. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Với maxRuns=N: có đúng N lần chạy, thời điểm cách nhau đúng chu kỳ, lịch COMPLETED sau lần thứ N. Với endDate: không có lần chạy nào sau endDate, lịch COMPLETED. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-041-3` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-041-4-functional-c

Chứng minh lịch MONTHLY neo ngày 29, 30 hoặc 31 chạy vào ngày cuối tháng ở tháng thiếu ngày, và 29/02 ở năm nhuận.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-041-4-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-041` | llm/approved |

Tiền điều kiện:

- Lịch MONTHLY ACTIVE với firstRunAt ngày 29, 30 hoặc 31
- maxRuns đủ lớn để đi qua tháng 2

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-d77385` Tạo lịch MONTHLY ACTIVE với firstRunAt ngày 29, 30 hoặc 31 và maxRuns đủ lớn để qua tháng 2. / dọn: Xóa các lịch và lần chạy do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo các lịch MONTHLY neo 29, 30, 31, cho cả năm thường và năm nhuận, với maxRuns đủ lớn qua tháng 2. |  |  |
| 2 | when | Hoàn tất từng lần chạy để Scheduler sinh lần chạy kế tiếp và ghi lại thời điểm dự kiến. | recurrence=MONTHLY; firstRunAt neo 31/01; maxRuns=4 | EP-CreateSchedule |
| 3 | then | So sánh các thời điểm lần chạy sinh ra với ngày cuối tháng mong đợi. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Neo 31: 31/01 → 28/02 (năm thường) hoặc 29/02 (năm nhuận) → 31/03 → 30/04. Neo 30: tháng 2 → ngày cuối tháng. Neo 29: 28/02 năm thường, 29/02 năm nhuận. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-041-4` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-042-1-functional-c

Chứng minh chủ tài khoản nguồn với API key hợp lệ tra cứu được một lịch đang tồn tại và nhận status, nextRunAt khớp dữ liệu lưu.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-042-1-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-042` | llm/approved |

Tiền điều kiện:

- Tồn tại lịch scheduleId thuộc tài khoản nguồn của người gọi
- API key hợp lệ của chủ tài khoản nguồn: secretRef:owner_api_key

Fixture: `TFX-SCHEDULED-TRANSFER-EP-GetSchedule-a9d1f0` Tạo lịch scheduleId thuộc tài khoản nguồn của người gọi và cấp API key hợp lệ. / dọn: Xóa lịch, API key và dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch chuyển tiền định kỳ thuộc tài khoản nguồn của người gọi và ghi lại scheduleId, status, nextRunAt đã lưu. |  |  |
| 2 | when | Gửi GET /v1/scheduled-transfers/{scheduleId} với header Authorization hợp lệ. | Authorization: secretRef:owner_api_key; scheduleId của lịch vừa tạo | EP-GetSchedule |
| 3 | then | Kiểm tra HTTP 200, body có status và nextRunAt khớp giá trị lưu; xác nhận lịch và các lần chạy không bị thay đổi. |  |  |
| 4 | cleanup | Xóa lịch và dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 200; body có status và nextRunAt khớp giá trị lưu trong ScheduledTransfer. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-042-1` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-042-2-functional-c

Chứng minh danh sách lịch của chính tài khoản người gọi lọc đúng theo status, giới hạn theo limit, phân trang đúng theo offset và không lẫn lịch tài khoản khác.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-042-2-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-042` | llm/approved |

Tiền điều kiện:

- Tài khoản có nhiều lịch ở các trạng thái khác nhau (tối đa 20 lịch ACTIVE/PAUSED theo BR-15)
- Tồn tại lịch của tài khoản khác
- API key hợp lệ: secretRef:owner_api_key

Fixture: `TFX-SCHEDULED-TRANSFER-EP-GetSchedule-2a7441` Tạo tập lịch nhiều trạng thái cho tài khoản người gọi (tối đa 20 lịch ACTIVE/PAUSED theo BR-15) và lịch của tài khoản khác. / dọn: Xóa toàn bộ lịch do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo nhiều lịch ACTIVE, PAUSED, COMPLETED, CANCELLED cho tài khoản người gọi và thêm lịch cho tài khoản khác. |  |  |
| 2 | when | Gửi GET /v1/accounts/{accountId}/scheduled-transfers với status=ACTIVE, limit=2, offset=1; sau đó thử các tổ hợp khác (không status, offset kế tiếp). | status=ACTIVE&limit=2&offset=1; Authorization: secretRef:owner_api_key | EP-GetSchedule |
| 3 | then | Kiểm tra HTTP 200, mọi phần tử thuộc accountId và có status lọc, số phần tử <= limit, trang kế tiếp không trùng và không sót so với trang trước. |  |  |
| 4 | cleanup | Xóa các lịch đã tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 200; mọi phần tử thuộc accountId và đúng status lọc; số phần tử <= limit; trang kế tiếp không trùng, không sót phần tử so với trang trước. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-042-2` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-042-3-functional-c

Chứng minh chủ lịch lấy được danh sách các lần chạy sắp xếp mới nhất trước, mỗi phần tử đủ scheduledAt, status, transferId, reasonCode khớp ScheduledRun.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-042-3-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-042` | llm/approved |

Tiền điều kiện:

- Lịch có ít nhất 3 lần chạy với scheduledAt khác nhau
- Có lần thành công (có transferId) và lần thất bại (có reasonCode)
- API key của chủ lịch: secretRef:owner_api_key

Fixture: `TFX-SCHEDULED-TRANSFER-EP-GetSchedule-153677` Tạo lịch có ít nhất 3 lần chạy scheduledAt khác nhau, gồm lần thành công (transferId) và lần thất bại (reasonCode). / dọn: Xóa lịch và các lần chạy do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch với ít nhất 3 lần chạy có scheduledAt khác nhau, gồm 1 lần thành công có transferId và 1 lần thất bại có reasonCode. |  |  |
| 2 | when | Gửi GET /v1/scheduled-transfers/{scheduleId}/runs với Authorization hợp lệ, có thể kèm limit và offset. | Authorization: secretRef:owner_api_key; limit=10; offset=0 | EP-GetSchedule |
| 3 | then | Kiểm tra HTTP 200, danh sách giảm dần theo scheduledAt, mỗi phần tử có đủ scheduledAt, status, transferId, reasonCode khớp ScheduledRun. |  |  |
| 4 | cleanup | Xóa lịch và các lần chạy đã tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 200; danh sách giảm dần theo scheduledAt; mỗi phần tử có đủ scheduledAt, status, transferId, reasonCode khớp ScheduledRun. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-042-3` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-042-4-functional-c

Chứng minh người không phải chủ nhận 403 FORBIDDEN và scheduleId không tồn tại nhận 404 SCHEDULE_NOT_FOUND, không lộ dữ liệu lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-042-4-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-042` | llm/approved |

Tiền điều kiện:

- Có lịch của tài khoản A
- Người gọi B có API key hợp lệ nhưng không phải chủ: secretRef:other_api_key
- Có một scheduleId chưa tồn tại

Fixture: `TFX-SCHEDULED-TRANSFER-EP-GetSchedule-ac075b` Tạo lịch của tài khoản A, API key hợp lệ của B không phải chủ và chọn một scheduleId chưa tồn tại. / dọn: Xóa lịch, API key và dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch cho tài khoản A, chuẩn bị API key hợp lệ của B và một scheduleId không tồn tại. |  |  |
| 2 | when | B gọi GET lịch và GET runs của lịch A; chủ gọi GET với scheduleId không tồn tại. | B: Authorization secretRef:other_api_key, scheduleId của A; chủ: Authorization secretRef:owner_api_key, scheduleId không tồn tại | EP-GetSchedule |
| 3 | then | Kiểm tra HTTP 403 mã FORBIDDEN cho B, HTTP 404 mã SCHEDULE_NOT_FOUND cho scheduleId không tồn tại, body lỗi không chứa dữ liệu lịch. |  |  |
| 4 | cleanup | Xóa lịch và dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 403 mã FORBIDDEN cho người không phải chủ; HTTP 404 mã SCHEDULE_NOT_FOUND cho lịch không tồn tại. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-042-4` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-043-1-functional-c

Chứng minh chủ lịch hủy được lịch ACTIVE bằng API key hợp lệ: lịch và mọi lần chạy PENDING chuyển sang CANCELLED.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-043-1-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-043` | llm/approved |

Tiền điều kiện:

- Có lịch ACTIVE thuộc người gọi, có ít nhất một ScheduledRun PENDING và không có run RUNNING
- API key hợp lệ của chủ lịch: secretRef:owner_api_key

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CancelSchedule-34c753` Tạo lịch ACTIVE thuộc người gọi, có ít nhất một lần chạy PENDING, không có lần chạy RUNNING. / dọn: Xóa lịch và các lần chạy do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE của chủ lịch với 2 lần chạy PENDING, không có lần chạy RUNNING; ghi lại scheduleId. | {"pendingRuns": 2, "scheduleId": "sch_0001"} |  |
| 2 | when | Gửi DELETE /v1/scheduled-transfers/sch_0001 kèm header Authorization: Bearer <API key hợp lệ>. | {"Authorization": "Bearer secretRef:owner_api_key", "method": "DELETE", "path": "/v1/scheduled-transfers/sch_0001"} | EP-CancelSchedule |
| 3 | then | Kiểm tra phản hồi HTTP 200, truy vấn lại lịch và các lần chạy để xác nhận trạng thái CANCELLED. |  |  |
| 4 | cleanup | Xóa lịch và các lần chạy do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 200; ScheduledTransfer.status = CANCELLED; mọi ScheduledRun PENDING của lịch có status = CANCELLED. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-043-1` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-043-2-functional-c

Chứng minh DELETE lặp trên lịch đã CANCELLED là idempotent: trả 200 như lần đầu, không đổi dữ liệu.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-043-2-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-043` | llm/approved |

Tiền điều kiện:

- Lịch đã CANCELLED và các lần chạy PENDING trước đó đã CANCELLED
- Dùng cùng API key của chủ lịch: secretRef:owner_api_key

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CancelSchedule-15cefc` Tạo lịch đã CANCELLED, các lần chạy PENDING trước đó đã CANCELLED. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch, hủy một lần thành công, ghi lại phản hồi và trạng thái ban đầu. | {"scheduleId": "sch_0002"} |  |
| 2 | when | Gửi lại DELETE /v1/scheduled-transfers/sch_0002 với cùng API key. | {"Authorization": "Bearer secretRef:owner_api_key", "method": "DELETE", "path": "/v1/scheduled-transfers/sch_0002"} | EP-CancelSchedule |
| 3 | then | Kiểm tra HTTP 200, nội dung tương đương lần hủy đầu, trạng thái lịch vẫn CANCELLED, không phát sinh thay đổi dữ liệu. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 200, nội dung phản hồi tương đương kết quả hủy ban đầu; trạng thái lịch vẫn CANCELLED. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-043-2` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-043-3-functional-c

Chứng minh DELETE bị từ chối bằng 409 SCHEDULE_NOT_CANCELLABLE khi lịch COMPLETED hoặc có lần chạy RUNNING.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-043-3-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-043` | llm/approved |

Tiền điều kiện:

- Trường hợp A: lịch COMPLETED; trường hợp B: lịch có ít nhất một lần chạy RUNNING
- API key của chủ lịch: secretRef:owner_api_key

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CancelSchedule-a85401` Tạo lịch COMPLETED (trường hợp A) và lịch có lần chạy RUNNING (trường hợp B). / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị lịch ở trường hợp A (COMPLETED) và lịch ở trường hợp B (có run RUNNING); ghi lại trạng thái trước khi gọi. | {"caseA": "sch_0003", "caseB": "sch_0004"} |  |
| 2 | when | Gửi DELETE /v1/scheduled-transfers/{scheduleId} bằng API key của chủ lịch, lần lượt cho từng trường hợp. | {"Authorization": "Bearer secretRef:owner_api_key", "method": "DELETE", "scheduleIds": ["sch_0003", "sch_0004"]} | EP-CancelSchedule |
| 3 | then | Kiểm tra mỗi lần gọi trả HTTP 409 với errorCode SCHEDULE_NOT_CANCELLABLE và trạng thái lịch, lần chạy không đổi. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 409, errorCode = SCHEDULE_NOT_CANCELLABLE; lịch và lần chạy giữ nguyên trạng thái trước khi gọi. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-043-3` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-044-1-functional-c

Chứng minh chủ lịch tạm dừng được lịch định kỳ ACTIVE bằng PATCH status=PAUSED: trả 200, lịch chuyển PAUSED và scheduler không chạy lịch này.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-044-1-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-044` | llm/approved |

Tiền điều kiện:

- Lịch định kỳ (DAILY/WEEKLY/MONTHLY) có status=ACTIVE, lần chạy dự kiến đến hạn trong tương lai gần
- Người gọi là chủ lịch, có API key hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-79ff2b` Tạo lịch định kỳ ACTIVE có lần chạy dự kiến đến hạn trong tương lai gần và API key hợp lệ của chủ lịch. / dọn: Xóa lịch, lần chạy và dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch DAILY status=ACTIVE thuộc chủ lịch, có lần chạy dự kiến đến hạn sau vài chu kỳ quét 30 giây; chuẩn bị API key hợp lệ của chủ lịch. | {"frequency": "DAILY", "scheduleId": "sch_0001", "status": "ACTIVE"} |  |
| 2 | when | Gửi PATCH /v1/scheduled-transfers/{scheduleId} với body status=PAUSED và header Authorization Bearer hợp lệ. | {"Authorization": "secretRef:owner_api_key", "body": {"status": "PAUSED"}} | EP-UpdateScheduleStatus |
| 3 | then | Kiểm tra phản hồi 200 có status=PAUSED, đọc lại bản ghi lịch và chờ qua thời điểm dự kiến để xác nhận không có giao dịch chuyển tiền mới. |  |  |
| 4 | cleanup | Xóa lịch và dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 200, status=PAUSED trong phản hồi và trong bản ghi lịch; không phát sinh giao dịch chuyển tiền mới cho lịch khi qua thời điểm dự kiến. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-044-1` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-044-2-functional-c

Chứng minh chủ lịch tiếp tục lịch PAUSED bằng status=ACTIVE: lịch thành ACTIVE, các lần bỏ lỡ được đánh dấu SKIPPED và không chạy bù.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-044-2-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-044` | llm/approved |

Tiền điều kiện:

- Lịch định kỳ có status=PAUSED
- Ít nhất một thời điểm dự kiến đã trôi qua trong lúc tạm dừng
- Người gọi là chủ lịch

Fixture: `TFX-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-3ead32` Tạo lịch định kỳ PAUSED có ít nhất một thời điểm dự kiến đã trôi qua và API key của chủ lịch. / dọn: Xóa lịch, lần chạy và dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch định kỳ PAUSED đã bỏ lỡ ít nhất một thời điểm dự kiến trong lúc tạm dừng; chuẩn bị API key hợp lệ của chủ lịch. | {"missedRuns": 1, "scheduleId": "sch_0002", "status": "PAUSED"} |  |
| 2 | when | Gửi PATCH với body status=ACTIVE và Authorization Bearer hợp lệ. | {"Authorization": "secretRef:owner_api_key", "body": {"status": "ACTIVE"}} | EP-UpdateScheduleStatus |
| 3 | then | Kiểm tra phản hồi 200 status=ACTIVE; các lần bỏ lỡ có status SKIPPED, không có giao dịch tương ứng và không bị chạy bù theo FR-049. |  |  |
| 4 | cleanup | Xóa lịch, các lần chạy và dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 200, status=ACTIVE; mọi lần bỏ lỡ trong lúc tạm dừng có status SKIPPED và không có giao dịch tương ứng; chạy bù (FR-049) không áp dụng cho các lần này. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-044-2` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-044-3-functional-c

Chứng minh PATCH chuyển trạng thái không hợp lệ (PAUSED→PAUSED, ACTIVE→ACTIVE, lịch đã hủy/trạng thái khác) hoặc trên lịch ONCE bị từ chối 409 SCHEDULE_STATE_INVALID.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-044-3-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-044` | llm/approved |

Tiền điều kiện:

- Có lịch ở một trong các trường hợp: PAUSED, ACTIVE, đã hủy hoặc ONCE
- Người gọi là chủ lịch với API key hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-c51155` Tạo các lịch: PAUSED, ACTIVE, đã hủy và ONCE để thử các chuyển trạng thái không hợp lệ. / dọn: Xóa các lịch và dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ở trạng thái nguồn không khớp, ví dụ lịch ACTIVE sẽ nhận status=ACTIVE; lặp lại cho các trường hợp PAUSED→PAUSED, lịch đã hủy và lịch ONCE. | {"scheduleId": "sch_0003", "status": "ACTIVE"} |  |
| 2 | when | Gửi PATCH với status hợp lệ nhưng không khớp trạng thái nguồn (hoặc gửi tới lịch ONCE) bằng API key của chủ lịch. | {"Authorization": "secretRef:owner_api_key", "body": {"status": "ACTIVE"}} | EP-UpdateScheduleStatus |
| 3 | then | Kiểm tra phản hồi 409 errorCode=SCHEDULE_STATE_INVALID và đọc lại lịch để xác nhận status cùng các lần chạy không đổi. |  |  |
| 4 | cleanup | Xóa lịch và dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 409, errorCode=SCHEDULE_STATE_INVALID; bản ghi lịch giữ nguyên status trước đó. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-044-3` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-045-1-functional-c

Chứng minh lần chạy đến hạn của lịch hợp lệ, đủ số dư, hạn mức và Fraud Check chấp thuận tạo đúng 1 giao dịch COMPLETED qua lõi chuyển tiền dùng chung.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-045-1-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-045` | llm/approved |

Tiền điều kiện:

- Lịch ở trạng thái ACTIVE, có một lần chạy đến hạn chưa được chiếm
- Tài khoản nguồn đủ số dư và hạn mức
- Fraud Check (mock) được cấu hình trả chấp thuận

Fixture: `TFX-SCHEDULED-TRANSFER-FR-045-060bee` Tạo lịch ACTIVE, một lần chạy đến hạn chưa chiếm, tài khoản nguồn đủ số dư và hạn mức, Fraud Check mock trả chấp thuận. / dọn: Xóa lịch, lần chạy, giao dịch và cấu hình mock do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE với một lần chạy đến hạn chưa chiếm; nạp số dư và hạn mức đủ cho tài khoản nguồn; cấu hình Fraud Check mock trả chấp thuận. |  |  |
| 2 | when | Kích hoạt chu kỳ quét 30 giây: scheduler chọn lần chạy đến hạn, chiếm bằng UPDATE có điều kiện và gọi execute_transfer. | scan_interval=30s; run_status=PENDING | FR-045 |
| 3 | then | Đếm Transfer theo lần chạy, kiểm tra trạng thái giao dịch, scheduled_runs.status và run.transferId. |  |  |
| 4 | cleanup | Xóa lịch, lần chạy, giao dịch và dữ liệu mock do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Đếm Transfer theo lần chạy = 1 và trạng thái COMPLETED; run.status = SUCCEEDED; run.transferId = id của Transfer đó. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-045-1` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-045-2-functional-c

Chứng minh Idempotency-Key truyền vào lõi chuyển tiền cho lần chạy đầu được dựng từ mã lịch ghép thời điểm dự kiến của lần chạy.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-045-2-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-045` | llm/approved |

Tiền điều kiện:

- Lịch ACTIVE có lần chạy đầu với thời điểm dự kiến xác định
- Tài khoản nguồn đủ điều kiện thực hiện giao dịch

Fixture: `TFX-SCHEDULED-TRANSFER-FR-045-059d98` Tạo lịch ACTIVE có lần chạy đầu với thời điểm dự kiến xác định. / dọn: Xóa lịch, lần chạy và giao dịch do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE với lần chạy đầu có thời điểm dự kiến cố định và ghi lại mã lịch. |  |  |
| 2 | when | Scheduler gọi execute_transfer cho lần chạy đầu của lịch, sau đó gọi lại với cùng mã lịch và thời điểm dự kiến. | scheduleId + scheduledAt của lần chạy đầu | FR-045 |
| 3 | then | So sánh Idempotency-Key trên giao dịch với chuỗi ghép mã lịch và thời điểm dự kiến; kiểm tra hai lần gọi trả về cùng một giao dịch. |  |  |
| 4 | cleanup | Xóa lịch, lần chạy và giao dịch do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Idempotency-Key ghi nhận trên giao dịch bằng đúng chuỗi ghép mã lịch và thời điểm dự kiến; hai lần gọi cùng cặp này trả về cùng một giao dịch. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-045-2` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-045-3-functional-c

Chứng minh hạn mức ngày, số dư, trạng thái tài khoản và Fraud Check được đánh giá theo trạng thái tại thời điểm chạy, không theo lúc tạo lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-045-3-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-045` | llm/approved |

Tiền điều kiện:

- Lịch đã được tạo
- Sau khi tạo, số dư, hạn mức đã dùng, trạng thái tài khoản hoặc phản hồi Fraud Check đã thay đổi trước thời điểm chạy

Fixture: `TFX-SCHEDULED-TRANSFER-FR-045-15de27` Tạo lịch, rồi thay đổi số dư, hạn mức đã dùng, trạng thái tài khoản hoặc phản hồi Fraud Check trước thời điểm chạy. / dọn: Khôi phục trạng thái tài khoản và xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch, sau đó thay đổi số dư, hạn mức đã dùng, trạng thái tài khoản hoặc phản hồi Fraud Check mock trước thời điểm chạy. |  |  |
| 2 | when | Scheduler xử lý lần chạy đến hạn qua execute_transfer. | lần chạy đến hạn sau khi điều kiện đã thay đổi | FR-045 |
| 3 | then | Kiểm tra quyết định chấp nhận/từ chối khớp trạng thái lúc chạy và xác nhận lời gọi Fraud Check xảy ra trong lúc chạy. |  |  |
| 4 | cleanup | Khôi phục tài khoản, xóa lịch, lần chạy và giao dịch do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Quyết định chấp nhận/từ chối khớp với số dư, hạn mức, trạng thái tài khoản và kết quả Fraud Check tại lúc chạy; lời gọi Fraud Check xảy ra trong lúc chạy. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-045-3` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-045-4-functional-c

Chứng minh giao dịch sinh từ lịch được cộng vào hạn mức ngày đã dùng của tài khoản nguồn và xuất hiện trong lịch sử giao dịch như giao dịch thường.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-045-4-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-045` | llm/approved |

Tiền điều kiện:

- Tài khoản nguồn có hạn mức ngày đã dùng X
- Có lần chạy đến hạn với số tiền A

Fixture: `TFX-SCHEDULED-TRANSFER-FR-045-93cf45` Tạo tài khoản nguồn với hạn mức ngày đã dùng X và lần chạy đến hạn với số tiền A. / dọn: Xóa giao dịch, lần chạy và đặt lại hạn mức ngày.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Thiết lập tài khoản nguồn với hạn mức ngày đã dùng X và một lần chạy đến hạn với số tiền A. |  |  |
| 2 | when | Cho lần chạy thành công, sau đó truy vấn lịch sử giao dịch và thử chuyển tiền thêm trong cùng ngày. | amount=A; used_before=X | FR-045 |
| 3 | then | Đọc hạn mức đã dùng và lịch sử giao dịch của tài khoản nguồn; kiểm tra giao dịch kế tiếp được kiểm hạn mức với phần đã dùng đã gồm giao dịch này. |  |  |
| 4 | cleanup | Xóa giao dịch, lần chạy và đặt lại hạn mức do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Hạn mức đã dùng = X + A; lịch sử giao dịch chứa Transfer của lần chạy với số tiền A. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-045-4` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-046-1-functional-c

Chứng minh khi lõi chuyển tiền trả FRAUD_CHECK_UNAVAILABLE, scheduler thử lại tối đa 3 lần cách nhau 5 phút; hết lượt vẫn lỗi thì run FAILED với reasonCode FRAUD_CHECK_UNAVAILABLE.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-046-1-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-046` | llm/approved |

Tiền điều kiện:

- Có lịch chuyển tiền với một lần chạy đến hạn đã được scheduler chiếm
- Fraud Check (INT-01) không khả dụng, luôn trả lỗi
- Đồng hồ hệ thống có thể điều khiển để tua 5 phút mỗi lần

Fixture: `TFX-SCHEDULED-TRANSFER-FR-046-2d9da3` Tạo lần chạy đến hạn đã được chiếm và đặt Fraud Check (INT-01) ở trạng thái không khả dụng. / dọn: Xóa dữ liệu và khôi phục INT-01 do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lần chạy đến hạn đã được chiếm và cấu hình INT-01 không khả dụng cho mọi lời gọi. | {"int01": "UNAVAILABLE", "runStatus": "CLAIMED"} |  |
| 2 | when | Gọi scan_and_run để xử lý lần chạy; lõi chuyển tiền trả FRAUD_CHECK_UNAVAILABLE. Sau mỗi lần lỗi, tua đồng hồ 5 phút và chạy lại quét cho đến khi hết 3 lần thử lại. | {"maxRetries": 3, "retryIntervalMinutes": 5} | FR-046 |
| 3 | then | Kiểm tra có đúng 3 lần thử lại cách nhau 5 phút, không có lần thứ 4, run = FAILED và reasonCode = FRAUD_CHECK_UNAVAILABLE. |  |  |
| 4 | cleanup | Xóa lịch, run và cấu hình giả lập INT-01 do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Số lần thử lại = 3, khoảng cách 5 phút; sau lần lỗi cuối run = FAILED, reasonCode = FRAUD_CHECK_UNAVAILABLE. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-046-1` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-046-2-functional-c

Chứng minh khi lõi chuyển tiền trả INSUFFICIENT_FUNDS, scheduler thử lại tối đa 2 lần cách nhau 1 giờ; hết lượt thì run chuyển FAILED.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-046-2-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-046` | llm/approved |

Tiền điều kiện:

- Có lần chạy đến hạn đã được chiếm
- Số dư tài khoản nguồn nhỏ hơn số tiền chuyển và không được nạp thêm
- Đồng hồ hệ thống có thể điều khiển để tua 1 giờ mỗi lần

Fixture: `TFX-SCHEDULED-TRANSFER-FR-046-d35835` Tạo lần chạy đến hạn đã được chiếm và tài khoản nguồn có số dư nhỏ hơn số tiền chuyển. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lần chạy đến hạn đã được chiếm với số dư nguồn nhỏ hơn số tiền chuyển. | {"runStatus": "CLAIMED", "sourceBalance": "nhỏ hơn số tiền chuyển"} |  |
| 2 | when | Gọi scan_and_run; lõi chuyển tiền trả INSUFFICIENT_FUNDS. Sau mỗi lần lỗi tua đồng hồ 1 giờ và chạy lại quét cho đến khi hết 2 lần thử lại. | {"maxRetries": 2, "retryIntervalHours": 1} | FR-046 |
| 3 | then | Kiểm tra có đúng 2 lần thử lại cách nhau 1 giờ, không có lần thứ 3 và run = FAILED. |  |  |
| 4 | cleanup | Xóa lịch, run và dữ liệu tài khoản do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Số lần thử lại = 2, khoảng cách 1 giờ; sau lần lỗi cuối run = FAILED. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-046-2` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-046-3-functional-c

Chứng minh khi lõi chuyển tiền trả ACCOUNT_INACTIVE, FRAUD_REJECTED hoặc LIMIT_EXCEEDED, scheduler đặt run FAILED ngay và không lên lịch thử lại.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-046-3-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-046` | llm/approved |

Tiền điều kiện:

- Có lần chạy đến hạn đã được chiếm
- Điều kiện gây lỗi tương ứng đang xảy ra: tài khoản không hoạt động, Fraud Check từ chối hoặc vượt hạn mức

Fixture: `TFX-SCHEDULED-TRANSFER-FR-046-9c39e1` Tạo lần chạy đến hạn đã được chiếm và thiết lập điều kiện gây lỗi tương ứng (tài khoản không hoạt động, Fraud Check từ chối, vượt hạn mức). / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lần chạy đến hạn đã được chiếm và thiết lập điều kiện gây lỗi cho một trong ba mã lỗi (lặp lại cho từng mã). | {"errorCodes": ["ACCOUNT_INACTIVE", "FRAUD_REJECTED", "LIMIT_EXCEEDED"]} |  |
| 2 | when | Gọi scan_and_run; lõi chuyển tiền trả một trong ba mã lỗi. |  | FR-046 |
| 3 | then | Kiểm tra run = FAILED ngay với reasonCode tương ứng, số lần thử lại = 0; chạy thêm các chu kỳ quét sau và xác nhận không có lần thử nào. |  |  |
| 4 | cleanup | Xóa lịch, run và dữ liệu do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Run = FAILED ngay với reasonCode tương ứng mã lỗi; số lần thử lại = 0; không có lần thử nào ở các chu kỳ quét sau. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-046-3` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-046-4-functional-c

Chứng minh khi lịch định kỳ có 3 lần chạy FAILED liên tiếp, lịch tự chuyển từ ACTIVE sang PAUSED, và vẫn ACTIVE nếu chuỗi chưa đủ 3.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-046-4-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-046` | llm/approved |

Tiền điều kiện:

- Lịch định kỳ ở trạng thái ACTIVE
- Hai lần chạy gần nhất của lịch đều FAILED liên tiếp

Fixture: `TFX-SCHEDULED-TRANSFER-FR-046-235ab3` Tạo lịch định kỳ ACTIVE đã có 2 lần chạy FAILED liên tiếp gần nhất. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch định kỳ ACTIVE với 2 run FAILED liên tiếp gần nhất. | {"consecutiveFailedRuns": 2, "scheduleStatus": "ACTIVE"} |  |
| 2 | when | Cho lần chạy thứ 3 của lịch kết thúc ở trạng thái FAILED. | {"thirdRunStatus": "FAILED"} | FR-046 |
| 3 | then | Kiểm tra lịch = PAUSED ngay sau lần FAILED thứ 3 và không sinh lần chạy mới; đối chứng: với chuỗi chỉ 2 FAILED hoặc bị run thành công ngắt, lịch vẫn ACTIVE. |  |  |
| 4 | cleanup | Xóa lịch và các run do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Lịch = PAUSED ngay sau lần FAILED thứ 3 liên tiếp; lịch vẫn ACTIVE nếu chuỗi FAILED liên tiếp < 3. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-046-4` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-047-1-functional-c

Chứng minh khi lần chạy lịch chuyển tiền thành công, hệ thống ghi đúng một sự kiện ScheduledRunCompleted vào outbox với transferId khớp giao dịch vừa ghi sổ.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P2 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-047-1-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-047` | llm/approved |

Tiền điều kiện:

- Lần chạy đến hạn đã được chiếm bởi scheduler
- Lõi chuyển tiền đã hoàn tất thành công và giao dịch đã ghi sổ, có transferId
- Bảng outbox chưa có sự kiện ScheduledRunCompleted nào cho lần chạy này

Fixture: `TFX-SCHEDULED-TRANSFER-FR-047-18fb28` Tạo lần chạy đến hạn đã được chiếm, lõi chuyển tiền hoàn tất thành công và giao dịch đã ghi sổ. / dọn: Xóa toàn bộ dữ liệu lần chạy, giao dịch và outbox do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị lần chạy đến hạn đã được chiếm, lõi chuyển tiền trả kết quả thành công và giao dịch đã ghi sổ với transferId xác định; outbox của lần chạy đang trống. |  |  |
| 2 | when | Scheduler cập nhật lần chạy sang trạng thái thành công sau khi lõi chuyển tiền trả kết quả; bản ghi outbox được commit cùng giao dịch CSDL. | Kết quả lõi chuyển tiền: thành công, kèm transferId của giao dịch vừa ghi sổ | FR-047 |
| 3 | then | Truy vấn bảng outbox theo lần chạy, đếm bản ghi ScheduledRunCompleted và so sánh payload.transferId với transferId của giao dịch. |  |  |
| 4 | cleanup | Xóa lần chạy, giao dịch và bản ghi outbox do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Đúng 1 sự kiện ScheduledRunCompleted trong outbox cho mỗi lần chạy thành công; payload.transferId khác rỗng và bằng transferId của giao dịch. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-047-1` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-047-2-functional-c

Chứng minh khi lần chạy kết thúc ở trạng thái FAILED, hệ thống ghi đúng một sự kiện ScheduledRunFailed vào outbox với reasonCode khớp nguyên nhân đã lưu và không phát ScheduledRunCompleted.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P2 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-047-2-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-047` | llm/approved |

Tiền điều kiện:

- Lần chạy đã được chiếm bởi scheduler
- Lõi chuyển tiền trả lỗi không thể thử lại hoặc đã hết số lần thử lại
- Outbox chưa có sự kiện nào cho lần chạy này

Fixture: `TFX-SCHEDULED-TRANSFER-FR-046-143d9b` Tạo lần chạy đã được chiếm, lõi chuyển tiền trả lỗi không thể thử lại hoặc hết số lần thử lại, lần chạy chuyển sang FAILED. / dọn: Xóa toàn bộ dữ liệu lần chạy và outbox do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị lần chạy đã được chiếm, lõi chuyển tiền trả lỗi không thể thử lại (hoặc đã hết số lần thử lại) với mã lý do xác định. |  |  |
| 2 | when | Scheduler đặt lần chạy sang trạng thái FAILED và lưu mã lý do thất bại trên lần chạy. | Kết quả lõi chuyển tiền: lỗi không thể thử lại, kèm reasonCode được lưu trên lần chạy | FR-046 |
| 3 | then | Truy vấn outbox theo lần chạy: đếm ScheduledRunFailed, so sánh payload.reasonCode với mã lý do lưu trên lần chạy, và xác nhận không có ScheduledRunCompleted. |  |  |
| 4 | cleanup | Xóa lần chạy và bản ghi outbox do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Đúng 1 sự kiện ScheduledRunFailed trong outbox cho mỗi lần chạy FAILED; payload.reasonCode khác rỗng và bằng mã lý do lưu trên lần chạy. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-047-2` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-047-3-functional-c

Chứng minh consumer của MOD-Notify gửi SMS kết quả cho người gửi sau khi nhận sự kiện từ queue, và khi SMS Gateway lỗi thì trạng thái lần chạy giữ nguyên.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P2 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-047-3-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-047` | llm/approved |

Tiền điều kiện:

- Sự kiện kết quả lần chạy đã nằm trong outbox và đã được relay lên queue
- Lần chạy đang ở trạng thái cuối (thành công hoặc FAILED)
- SMS Gateway có thể được cấu hình trả thành công hoặc lỗi

Fixture: `TFX-SCHEDULED-TRANSFER-FR-047-fd8dd4` Tạo sự kiện kết quả lần chạy trong outbox, đã relay lên queue; lần chạy ở trạng thái cuối. / dọn: Xóa toàn bộ dữ liệu sự kiện, sms_log và lần chạy do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị sự kiện kết quả lần chạy đã được relay lên queue, lần chạy ở trạng thái cuối; ghi nhận trạng thái lần chạy trước khi xử lý. |  |  |
| 2 | when | Trường hợp bình thường: consumer của MOD-Notify nhận sự kiện và gọi SMS Gateway thành công. | Sự kiện ScheduledRunCompleted hoặc ScheduledRunFailed; SMS Gateway trả thành công | FR-047 |
| 3 | then | Kiểm tra sms_log có bản ghi cho người gửi với nội dung sinh từ mẫu kết quả lần chạy. |  |  |
| 4 | when | Trường hợp lỗi: consumer xử lý sự kiện khác trong khi SMS Gateway trả lỗi. | Sự kiện kết quả lần chạy; SMS Gateway trả lỗi | FR-047 |
| 5 | then | So sánh trạng thái lần chạy trước và sau khi gửi lỗi: giống nhau, không có giao dịch bù nào được tạo. |  |  |
| 6 | cleanup | Xóa sms_log, sự kiện và lần chạy do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Trường hợp bình thường: có bản ghi SMS cho người gửi với nội dung từ mẫu kết quả lần chạy. Trường hợp lỗi SMS: trạng thái lần chạy trước và sau khi gửi lỗi giống nhau. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-047-3` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-048-1-functional-c

Chứng minh mỗi thao tác tạo, tạm dừng, tiếp tục, hủy lịch thành công đều ghi đúng một bản ghi audit vào AuditLog với đủ 5 thuộc tính.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-048-1-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-048` | llm/approved |

Tiền điều kiện:

- Tài khoản người gọi hợp lệ, có API key (secretRef:caller_api_key).
- AuditLog đã ghi nhận số bản ghi ban đầu cho mã lịch sẽ tạo (0 bản ghi).
- Lịch sẽ được tạo mới, định kỳ, thuộc người gọi, để chuyển được ACTIVE → PAUSED → ACTIVE → hủy.

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CancelSchedule-c6afb8` Tạo tài khoản người gọi hợp lệ có API key; chuẩn bị môi trường để lịch định kỳ thuộc người gọi có thể chuyển trạng thái ACTIVE, PAUSED và hủy. / dọn: Xóa lịch, tài khoản và bản ghi test do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị tài khoản người gọi hợp lệ với API key và ghi lại số bản ghi AuditLog hiện có. |  |  |
| 2 | when | Gọi tạo lịch chuyển tiền định kỳ, mong đợi 201, lấy mã lịch từ phản hồi. | POST tạo lịch định kỳ, header X-API-Key=secretRef:caller_api_key | EP-CreateSchedule |
| 3 | when | Tạm dừng lịch vừa tạo, mong đợi 200. | PATCH status=PAUSED | EP-UpdateScheduleStatus |
| 4 | when | Tiếp tục lịch đã tạm dừng, mong đợi 200. | PATCH status=ACTIVE | EP-UpdateScheduleStatus |
| 5 | when | Hủy lịch (không ở trạng thái COMPLETED), mong đợi 200. | DELETE theo mã lịch | EP-CancelSchedule |
| 6 | then | Truy vấn AuditLog theo mã lịch: có đúng 4 bản ghi mới, mỗi bản ghi có correlationId không rỗng, thời điểm, tác nhân là mã tài khoản (không phải SCHEDULER), loại sự kiện phân biệt (tạo/tạm dừng/tiếp tục/hủy) và mã lịch đúng. |  |  |
| 7 | cleanup | Xóa dữ liệu test do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Sau 4 thao tác thành công, AuditLog có đúng 4 bản ghi mới cho mã lịch đó, mỗi bản ghi đủ correlationId, thời điểm, tác nhân = mã tài khoản (không phải SCHEDULER), loại sự kiện phân biệt, mã lịch. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-048-1` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-048-2-functional-c

Chứng minh mỗi lần chạy lịch đến hạn do scheduler xử lý, dù thành công hay thất bại, đều có bản ghi audit với tác nhân SCHEDULER.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-048-2-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-048` | llm/approved |

Tiền điều kiện:

- Có lịch ACTIVE với lần chạy đã đến hạn.
- Scheduler quét mỗi 30 giây, tối đa 20 lần chạy mỗi chu kỳ.
- Fraud Check mock sẵn sàng và đổi được kết quả (đạt/không đạt).

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CancelSchedule-49e6d6` Tạo lịch ACTIVE có lần chạy đến hạn, bật Fraud Check mock đổi được kết quả, scheduler hoạt động với chu kỳ 30 giây và tối đa 20 lần chạy mỗi chu kỳ. / dọn: Xóa lịch, lần chạy, bản ghi test và tắt Fraud Check mock.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE có lần chạy đến hạn, cấu hình Fraud Check mock trả kết quả đạt cho lần chạy đầu và ghi lại số bản ghi AuditLog hiện có. |  |  |
| 2 | when | Chờ scheduler chiếm lần chạy đến hạn và thực hiện chuyển tiền thành công trong một chu kỳ quét. | Fraud Check mock = đạt | SCN-ScheduledRun |
| 3 | when | Đổi Fraud Check mock sang kết quả thất bại, tạo lần chạy đến hạn tiếp theo và chờ scheduler xử lý. | Fraud Check mock = thất bại | SCN-ScheduledRun |
| 4 | then | Truy vấn AuditLog theo mã lịch: mỗi lần chạy đã có kết quả có một bản ghi actor = SCHEDULER, mã lịch đúng, và số bản ghi khớp số lần chạy đã xử lý. |  |  |
| 5 | cleanup | Xóa lịch, lần chạy và dữ liệu test, khôi phục Fraud Check mock. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Với mỗi lần chạy đã có kết quả, AuditLog có bản ghi actor = SCHEDULER, mã lịch = lịch của lần chạy; số bản ghi khớp số lần chạy đã xử lý. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-048-2` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-049-1-functional-c

Chứng minh lần chạy quá hạn không quá 24 giờ sau khi Scheduler khởi động lại được chạy bù đúng một lần qua lõi chuyển tiền.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-049-1-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-049` | llm/approved |

Tiền điều kiện:

- Lịch ở trạng thái ACTIVE, có một ScheduledRun đến hạn và chưa được chạy.
- Thời điểm dự kiến cách hiện tại 23 giờ 59 phút do hệ thống ngừng.
- Số dư tài khoản nguồn và hạn mức đủ cho giao dịch.

Fixture: `TFX-SCHEDULED-TRANSFER-FR-049-127b9c` Tạo lịch ACTIVE với một ScheduledRun đến hạn, chưa chạy, thời điểm dự kiến cách hiện tại 23 giờ 59 phút; số dư và hạn mức đủ. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE với một ScheduledRun chưa chạy, thời điểm dự kiến cách hiện tại 23 giờ 59 phút; đảm bảo số dư và hạn mức đủ. | {"overdue": "23h59m", "runStatus": "PENDING", "scheduleStatus": "ACTIVE"} |  |
| 2 | when | Khởi động lại Scheduler và chạy các chu kỳ quét (30 giây/chu kỳ) cho đến khi lần chạy được xử lý. | {"scanIntervalSeconds": 30} | FR-049 |
| 3 | then | Kiểm tra có đúng 1 giao dịch cho lần chạy bù, lần chạy không còn ở trạng thái chờ, và chạy thêm một chu kỳ quét không tạo giao dịch mới. |  |  |
| 4 | cleanup | Xóa lịch, lần chạy và giao dịch do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Đúng 1 giao dịch cho lần chạy bù; lần chạy không còn ở trạng thái chờ; quét thêm không tạo giao dịch mới. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-049-1` |

#### TC-L2-SCHEDULED-TRANSFER-AC-FR-049-2-functional-c

Chứng minh lần chạy quá hạn quá 24 giờ không được chạy bù mà chuyển sang trạng thái MISSED.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-FR-049-2-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-049` | llm/approved |

Tiền điều kiện:

- Lịch ở trạng thái ACTIVE, có một ScheduledRun chưa chạy.
- Thời điểm dự kiến cách hiện tại 24 giờ 1 phút do hệ thống ngừng.
- Ghi nhận số dư tài khoản nguồn và đích trước khi quét.

Fixture: `TFX-SCHEDULED-TRANSFER-FR-049-26c84a` Tạo lịch ACTIVE với một ScheduledRun chưa chạy, thời điểm dự kiến cách hiện tại 24 giờ 1 phút. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE với một ScheduledRun chưa chạy, thời điểm dự kiến cách hiện tại 24 giờ 1 phút; ghi lại số dư nguồn và đích. | {"overdue": "24h01m", "runStatus": "PENDING", "scheduleStatus": "ACTIVE"} |  |
| 2 | when | Khởi động lại Scheduler và chạy một chu kỳ quét. |  | FR-049 |
| 3 | then | Kiểm tra trạng thái lần chạy là MISSED, có 1 sự kiện ScheduledRunFailed reasonCode MISSED, không có giao dịch nào và số dư nguồn, đích không đổi. |  |  |
| 4 | cleanup | Xóa lịch, lần chạy và sự kiện outbox do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Trạng thái lần chạy = MISSED; có 1 sự kiện ScheduledRunFailed reasonCode MISSED; 0 giao dịch cho lần chạy này; số dư tài khoản nguồn và đích không đổi. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-049-2` |

#### TC-L2-SCHEDULED-TRANSFER-AC-NFR-010-1-timing-c

Chứng minh với đồng hồ giả và chu kỳ quét 30 giây, các lần chạy đến hạn phân bố đều trong chu kỳ có ít nhất 99% lần bắt đầu trong 60 giây kể từ thời điểm dự kiến.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-NFR-010-1-timing-c` | `TSET-SCHEDULED-TRANSFER-L2-NFR-010` | llm/approved |

Tiền điều kiện:

- Đồng hồ giả đã được cài đặt và điều khiển được
- Vòng quét Scheduler cấu hình chu kỳ 30 giây
- Có tập lịch ACTIVE với các lần chạy đến hạn lệch pha khác nhau so với chu kỳ quét, mỗi chu kỳ không quá 20 lần chạy

Fixture: `TFX-SCHEDULED-TRANSFER-NFR-010-6fb419` Đặt đồng hồ giả, chu kỳ quét Scheduler 30 giây, tạo tập lịch ACTIVE với các lần chạy đến hạn lệch pha khác nhau, tối đa 20 lần chạy mỗi chu kỳ. / dọn: Xóa dữ liệu lịch và lần chạy do fixture tạo, khôi phục đồng hồ thật.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Khởi tạo đồng hồ giả, cấu hình chu kỳ quét 30 giây và nạp tập lịch ACTIVE có lần chạy đến hạn ở nhiều độ lệch pha khác nhau trong chu kỳ quét (tối đa 20 lần chạy mỗi chu kỳ). | {"maxRunsPerCycle": 20, "scanIntervalSeconds": 30, "scheduleStatus": "ACTIVE"} |  |
| 2 | when | Tiến đồng hồ giả qua từng chu kỳ 30 giây để scan_and_run chạy; ghi thời điểm dự kiến và thời điểm chiếm lần chạy của từng lần chạy. | {"advanceStepSeconds": 30, "record": ["scheduledAt", "claimedAt"]} | MOD-Scheduler |
| 3 | then | Tính độ trễ bắt đầu = thời điểm chiếm - thời điểm dự kiến cho từng lần chạy, rồi tính tỷ lệ lần chạy có độ trễ <= 60 s trên tổng số lần chạy và so với ngưỡng 0,99. |  |  |
| 4 | cleanup | Khôi phục đồng hồ thật và xóa các lịch cùng lần chạy do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Số lần chạy có độ trễ bắt đầu <= 60 s chia tổng số lần chạy >= 0,99. |  | `TORC-SCHEDULED-TRANSFER-AC-NFR-010-1` |

#### TC-L2-SCHEDULED-TRANSFER-AC-NFR-011-1-concurrency-c

Chứng minh khi hai scheduler cùng quét và cùng UPDATE có điều kiện để chiếm một lần chạy đến hạn, chỉ một scheduler chiếm được và chỉ tạo đúng 1 giao dịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-NFR-011-1-concurrency-c` | `TSET-SCHEDULED-TRANSFER-L2-NFR-011` | llm/approved |

Tiền điều kiện:

- Bảng scheduled_runs có đúng một lần chạy đến hạn ở trạng thái chưa được chiếm.
- Hai tiến trình scheduler đang hoạt động và dùng chung cơ sở dữ liệu.
- Lõi chuyển tiền sẵn sàng; chưa có giao dịch nào ứng với Idempotency-Key của lần chạy.

Fixture: `TFX-SCHEDULED-TRANSFER-NFR-011-faceb0` Tạo một lần chạy đến hạn chưa được chiếm trong scheduled_runs và bật hai tiến trình scheduler hoạt động. / dọn: Xóa lần chạy, giao dịch và dữ liệu do fixture tạo; dừng hai tiến trình scheduler.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo một lần chạy đến hạn chưa được chiếm trong scheduled_runs và khởi động hai tiến trình scheduler A và B. |  |  |
| 2 | when | Cho cả hai scheduler cùng đến chu kỳ quét 30 giây, chọn cùng lần chạy đến hạn và cùng thực hiện UPDATE có điều kiện để chiếm lần chạy. | Hai scheduler quét đồng thời; Idempotency-Key = mã lịch + thời điểm dự kiến của lần chạy | NFR-011 |
| 3 | then | Kiểm tra chỉ một scheduler có UPDATE ảnh hưởng 1 dòng và gọi lõi chuyển tiền, scheduler còn lại bỏ qua; đếm giao dịch theo Idempotency-Key. |  |  |
| 4 | cleanup | Xóa lần chạy, giao dịch và dữ liệu do fixture tạo; dừng hai scheduler. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Đúng 1 giao dịch ứng với Idempotency-Key (mã lịch + thời điểm dự kiến); chỉ 1 scheduler chiếm được lần chạy; không có giao dịch thứ hai. |  | `TORC-SCHEDULED-TRANSFER-AC-NFR-011-1` |

#### TC-L2-SCHEDULED-TRANSFER-AC-NFR-011-2-concurrency-c

Chứng minh khi scheduler dừng đột ngột sau khi chiếm lần chạy nhưng trước khi chuyển tiền hoàn tất, rồi khởi động lại và quét lại, vẫn không có quá 1 giao dịch cho lần chạy đó.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-NFR-011-2-concurrency-c` | `TSET-SCHEDULED-TRANSFER-L2-NFR-011` | llm/approved |

Tiền điều kiện:

- Lần chạy đã được scheduler chiếm trong scheduled_runs.
- Chưa có giao dịch hoàn tất ứng với Idempotency-Key của lần chạy.
- Có thể dừng đột ngột và khởi động lại tiến trình scheduler.

Fixture: `TFX-SCHEDULED-TRANSFER-FR-045-08808b` Tạo lần chạy đã được scheduler chiếm, chưa có giao dịch hoàn tất cho lần chạy. / dọn: Xóa lần chạy, giao dịch và dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Đặt lần chạy ở trạng thái đã được scheduler chiếm, chưa ghi nhận giao dịch hoàn tất. |  |  |
| 2 | when | Dừng đột ngột scheduler giữa bước chiếm lần chạy và bước chuyển tiền, khởi động lại rồi để scheduler quét lại lần chạy ở chu kỳ sau. | Kill tiến trình scheduler sau khi chiếm lần chạy; khởi động lại; chờ chu kỳ quét 30 giây kế tiếp | FR-045 |
| 3 | then | Đếm số giao dịch theo Idempotency-Key của lần chạy và kiểm tra lần chạy được hoàn tất hoặc xử lý theo luồng thử lại/chạy bù. |  |  |
| 4 | cleanup | Xóa lần chạy, giao dịch và dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Số giao dịch ứng với Idempotency-Key của lần chạy <= 1 sau khi khởi động lại và quét lại; không có giao dịch thứ hai. |  | `TORC-SCHEDULED-TRANSFER-AC-NFR-011-2` |

#### TC-L2-SCHEDULED-TRANSFER-AC-NFR-012-1-resilience-c

Chứng minh sau khi Scheduler khởi động lại, mọi lần chạy quá hạn không quá 24 giờ được xử lý hết trong 5 chu kỳ quét đầu tiên, mỗi chu kỳ tối đa 20 lần.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-NFR-012-1-resilience-c` | `TSET-SCHEDULED-TRANSFER-L2-NFR-012` | llm/approved |

Tiền điều kiện:

- Scheduler đang ngừng
- Có 100 lần chạy đến hạn chưa xử lý, độ trễ ≤ 24 giờ
- Chu kỳ quét cấu hình 30 giây

Fixture: `TFX-SCHEDULED-TRANSFER-NFR-012-198aa0` Dừng Scheduler; tạo 100 lần chạy đến hạn chưa xử lý có độ trễ ≤ 24 giờ. / dọn: Xóa các lần chạy và giao dịch do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Nạp 100 lần chạy đến hạn chưa xử lý với độ trễ ≤ 24 giờ vào CSDL khi Scheduler đang ngừng. | {"doTreToiDa": "24h", "soLanChayQuaHan": 100, "trangThai": "chờ xử lý"} |  |
| 2 | when | Khởi động lại Scheduler và để chạy đủ 5 chu kỳ quét, mỗi chu kỳ 30 giây. | {"chuKyGiay": 30, "soChuKy": 5, "toiDaMoiChuKy": 20} | NFR-012 |
| 3 | then | Sau chu kỳ thứ 5, đếm các lần chạy quá hạn ≤ 24 giờ còn chưa xử lý và số lần xử lý trong mỗi chu kỳ. |  |  |
| 4 | cleanup | Xóa dữ liệu lần chạy và giao dịch do fixture tạo, dừng Scheduler. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Sau chu kỳ quét thứ 5, số lần chạy quá hạn ≤ 24 giờ còn chưa xử lý bằng 0; mỗi chu kỳ xử lý tối đa 20 lần chạy. |  | `TORC-SCHEDULED-TRANSFER-AC-NFR-012-1` |

#### TC-L2-SCHEDULED-TRANSFER-AC-NFR-013-1-performance-c

Chứng minh với 100 lần chạy đến hạn cùng một thời điểm, p95 thời gian chuyển tiền qua API không vượt 500 ms và không lần chạy nào bị bỏ sót.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-NFR-013-1-performance-c` | `TSET-SCHEDULED-TRANSFER-L2-NFR-013` | llm/approved |

Tiền điều kiện:

- Có 100 lịch chuyển tiền ACTIVE với thời điểm dự kiến trùng nhau, đã đến hạn.
- Fraud Check (mock) hoạt động bình thường.
- Scheduler quét chu kỳ 30 giây, tối đa 20 lần chạy mỗi chu kỳ.

Fixture: `TFX-SCHEDULED-TRANSFER-NFR-013-528077` Tạo 100 lịch chuyển tiền ACTIVE trùng thời điểm dự kiến, đã đến hạn; bật Fraud Check mock hoạt động bình thường. / dọn: Xóa toàn bộ lịch, lần chạy, giao dịch và cấu hình mock do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-AC-NFR-013-1-performance-c` (NFR-013, none): value = 500 (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo 100 lịch chuyển tiền ACTIVE có thời điểm dự kiến trùng nhau và đã đến hạn; cấu hình Fraud Check mock phản hồi bình thường. |  |  |
| 2 | when | Kích hoạt Scheduler xử lý toàn bộ 100 lần chạy đến hạn qua nhiều chu kỳ quét và đo thời gian phản hồi của từng lời gọi chuyển tiền qua API. | {"soLanChay": 100, "value": 500} | NFR-013 |
| 3 | then | Tính phân vị 95 của thời gian phản hồi và xác nhận p95 <= 500 ms, đồng thời đủ 100 lần chạy được xử lý. |  |  |
| 4 | cleanup | Xóa các lịch và giao dịch do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | p95 thời gian chuyển tiền qua API <= 500 ms khi 100 lần chạy đến hạn cùng lúc. |  | `TORC-SCHEDULED-TRANSFER-AC-NFR-013-1` |

#### TC-L2-SCHEDULED-TRANSFER-AC-NFR-013-2-performance-c

Chứng minh một chu kỳ quét của scheduler chỉ chọn và chiếm tối đa 20 lần chạy, kể cả khi có hơn 20 lần chạy đến hạn.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-NFR-013-2-performance-c` | `TSET-SCHEDULED-TRANSFER-L2-NFR-013` | llm/approved |

Tiền điều kiện:

- Có 100 lần chạy đến hạn đang chờ xử lý.
- Scheduler sẵn sàng bắt đầu một chu kỳ quét.

Fixture: `TFX-SCHEDULED-TRANSFER-NFR-013-96766e` Tạo 100 lần chạy đến hạn ở trạng thái chờ xử lý. / dọn: Xóa toàn bộ lần chạy và dữ liệu liên quan do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-AC-NFR-013-2-performance-c` (NFR-013, none): value = 20 (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo 100 lần chạy đến hạn ở trạng thái chờ xử lý. |  |  |
| 2 | when | Bắt đầu đúng một chu kỳ quét của scheduler và đếm số lần chạy được chọn và chiếm. | {"soLanChayDenHan": 100, "value": 20} | NFR-013 |
| 3 | then | Xác nhận số lần chạy được chiếm và xử lý trong chu kỳ <= 20; phần còn lại vẫn chờ chu kỳ kế tiếp. |  |  |
| 4 | cleanup | Xóa các lần chạy do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Số lần chạy được chọn và chiếm trong một chu kỳ quét <= 20. |  | `TORC-SCHEDULED-TRANSFER-AC-NFR-013-2` |

#### TC-L2-SCHEDULED-TRANSFER-AC-NFR-014-1-observability-c

Chứng minh mỗi lần chạy đến hạn được sinh một correlationId riêng, không rỗng, không trùng với lần chạy nào khác, kể cả trong cùng chu kỳ quét (tối đa 20 lần chạy).

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-NFR-014-1-observability-c` | `TSET-SCHEDULED-TRANSFER-L2-NFR-014` | llm/approved |

Tiền điều kiện:

- Có nhiều lịch chuyển tiền đến hạn (ví dụ 20 lịch), mỗi lịch có một lần chạy ở trạng thái chờ chạy.
- Scheduler hoạt động với chu kỳ quét 30 giây.
- Fixture TFX-SCHEDULED-TRANSFER-NFR-014-e1ca94 đã được thiết lập.

Fixture: `TFX-SCHEDULED-TRANSFER-NFR-014-e1ca94` Đưa hệ thống về trạng thái có nhiều lịch chuyển tiền đến hạn, mỗi lịch có một lần chạy ở trạng thái chờ chạy. / dọn: Xóa dữ liệu lịch, lần chạy và bản ghi do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo 20 lịch chuyển tiền đến hạn, mỗi lịch có một lần chạy ở trạng thái chờ chạy; ghi lại danh sách mã lần chạy. | {"soLichDenHan": 20} |  |
| 2 | when | Kích hoạt một chu kỳ quét của scheduler để chiếm và xử lý các lần chạy đến hạn, bước sinh correlationId được thực thi cho từng lần chạy. |  | NFR-014 |
| 3 | then | Thu thập correlationId của từng lần chạy, kiểm tra không rỗng/null và đếm số giá trị phân biệt so với số lần chạy. |  |  |
| 4 | cleanup | Xóa các lịch, lần chạy và bản ghi do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Số correlationId phân biệt = số lần chạy; mọi lần chạy đều có correlationId khác rỗng. |  | `TORC-SCHEDULED-TRANSFER-AC-NFR-014-1` |

#### TC-L2-SCHEDULED-TRANSFER-AC-NFR-014-2-observability-c

Chứng minh correlationId của một lần chạy được truyền xuyên suốt và xuất hiện đúng ở giao dịch, bản ghi audit, lời gọi Fraud Check (INT-01) và sự kiện của chính lần chạy đó.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-AC-NFR-014-2-observability-c` | `TSET-SCHEDULED-TRANSFER-L2-NFR-014` | llm/approved |

Tiền điều kiện:

- Có một lần chạy đến hạn.
- Fraud Check (mock) sẵn sàng nhận lời gọi và ghi lại request.
- Fixture TFX-SCHEDULED-TRANSFER-NFR-014-9187e0 đã được thiết lập.

Fixture: `TFX-SCHEDULED-TRANSFER-NFR-014-9187e0` Đưa hệ thống về trạng thái có một lần chạy đến hạn và Fraud Check (mock) sẵn sàng nhận lời gọi. / dọn: Xóa dữ liệu do fixture tạo và reset mock Fraud Check.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo một lịch chuyển tiền đến hạn với một lần chạy chờ chạy và cấu hình Fraud Check mock ghi lại request nhận được. |  |  |
| 2 | when | Cho scheduler thực thi lần chạy qua lõi chuyển tiền, hoàn tất bước audit (tác nhân SCHEDULER) và ghi sự kiện qua outbox. |  | NFR-014 |
| 3 | then | Đọc correlationId của lần chạy rồi đối chiếu với giá trị trong giao dịch, bản ghi audit, request Fraud Check (INT-01) và sự kiện outbox. |  |  |
| 4 | cleanup | Xóa dữ liệu lần chạy, giao dịch, audit, sự kiện và reset mock Fraud Check. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Giá trị correlationId trong giao dịch, audit, lời gọi Fraud Check và sự kiện bằng đúng giá trị correlationId của lần chạy. |  | `TORC-SCHEDULED-TRANSFER-AC-NFR-014-2` |

#### TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-audit-p1cc3a7-c

Chứng minh hủy lịch thành công ghi đúng một bản ghi audit hủy đầy đủ trường theo FR-048.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-audit-p1cc3a7-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CancelSchedule` | llm/approved |

Tiền điều kiện:

- Có lịch ACTIVE thuộc tài khoản của người gọi
- Người gọi có API key hợp lệ (secretRef:owner_api_key)

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CancelSchedule-8a13d2` Tạo tài khoản chủ lịch và một lịch ACTIVE thuộc tài khoản đó. / dọn: Xóa lịch, lần chạy và bản ghi audit do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE thuộc tài khoản người gọi và ghi nhận số bản ghi audit hủy hiện có của lịch này. |  |  |
| 2 | when | Gửi DELETE /v1/scheduled-transfers/{scheduleId} lần đầu bằng API key của chủ lịch, kèm Idempotency-Key mới. | {"Authorization": "secretRef:owner_api_key", "Idempotency-Key": "idem-audit-001"} | EP-CancelSchedule |
| 3 | then | Truy vấn kho audit theo mã lịch, kiểm tra có đúng một bản ghi hủy mới với correlationId, thời điểm, actor là mã tài khoản, loại sự kiện hủy, mã lịch. |  |  |
| 4 | cleanup | Xóa lịch và bản ghi audit do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | Một bản ghi audit với correlationId, thời điểm, actor = mã tài khoản, loại sự kiện hủy, mã lịch. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-048-1-p1cc3a7` |

#### TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-concurrency-concurrent-dup-c

Chứng minh hai DELETE đồng thời cùng Idempotency-Key và scheduleId chỉ hủy lịch một lần, chỉ một audit, cả hai nhận 200 cùng kết quả.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-concurrency-concurrent-dup-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CancelSchedule` | llm/approved |

Tiền điều kiện:

- Lịch ACTIVE thuộc người gọi, có ít nhất một lần chạy PENDING, không có lần RUNNING
- Người gọi có API key hợp lệ (secretRef:owner_api_key)

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CancelSchedule-cbf211` Tạo lịch ACTIVE thuộc người gọi với ít nhất một lần PENDING và không có lần RUNNING. / dọn: Xóa lịch, lần chạy, bản ghi idempotency và audit do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE có 2 lần chạy PENDING, không có lần RUNNING; ghi nhận số audit hủy ban đầu. |  |  |
| 2 | when | Gửi đồng thời hai DELETE giống hệt (cùng Idempotency-Key, cùng scheduleId) từ hai luồng. | {"Authorization": "secretRef:owner_api_key", "Idempotency-Key": "idem-conc-001", "parallel": 2} | EP-CancelSchedule |
| 3 | then | Kiểm tra cả hai phản hồi HTTP 200 có nội dung giống nhau; lịch CANCELLED, các lần PENDING CANCELLED một lần, chỉ một bản ghi audit hủy. |  |  |
| 4 | cleanup | Xóa dữ liệu do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | count | == | 200, nội dung phản hồi giống lần đầu; số bản ghi audit hủy không tăng. |  | `TORC-SCHEDULED-TRANSFER-EP-CancelSchedule-idem` |

#### TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-idempotency-payload-mismatch-c

Ghi nhận hành vi khi DELETE dùng lại Idempotency-Key với tham số khác lần trước; chính sách xung đột khóa chưa được nguồn nêu, bất biến là không tạo thêm hiệu ứng hủy.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-idempotency-payload-mismatch-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CancelSchedule` | llm/approved |

Tiền điều kiện:

- Khóa idempotency đã được dùng cho một DELETE trước đó
- Có thêm một scheduleId khác thuộc người gọi để thay đổi tham số

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CancelSchedule-586156` Tạo khóa idempotency đã dùng cho một DELETE trước đó và các lịch cần thiết. / dọn: Xóa lịch, bản ghi idempotency và audit do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Gửi DELETE đầu tiên với khóa idem-mismatch-001 cho lịch S1 và xác nhận hoàn tất; ghi nhận số audit hủy. |  |  |
| 2 | when | Gửi DELETE với cùng khóa idem-mismatch-001 nhưng scheduleId khác (S2) hoặc payload khác. | {"Authorization": "secretRef:owner_api_key", "Idempotency-Key": "idem-mismatch-001", "scheduleId": "S2"} | EP-CancelSchedule |
| 3 | then | Kiểm tra theo oracle: phản hồi và số bản ghi audit hủy không tăng; không phát sinh thêm hiệu ứng hủy. |  |  |
| 4 | cleanup | Xóa dữ liệu do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | count | == | 200, nội dung phản hồi giống lần đầu; số bản ghi audit hủy không tăng. |  | `TORC-SCHEDULED-TRANSFER-EP-CancelSchedule-idem` |

#### TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-idempotency-replay-c

Chứng minh gửi lại đúng DELETE (cùng Idempotency-Key, cùng scheduleId) sau khi hủy thành công trả 200 như cũ và không ghi thêm audit.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-idempotency-replay-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CancelSchedule` | llm/approved |

Tiền điều kiện:

- Lịch đã CANCELLED bởi yêu cầu đầu tiên cùng khóa
- Người gọi có API key hợp lệ (secretRef:owner_api_key)

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CancelSchedule-3960ca` Tạo lịch và hủy thành công bằng yêu cầu đầu tiên với khóa idempotency cố định. / dọn: Xóa lịch, bản ghi idempotency và audit do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Gửi DELETE đầu tiên cho lịch ACTIVE với Idempotency-Key idem-replay-001, lưu phản hồi và số bản ghi audit hủy. |  |  |
| 2 | when | Gửi lại DELETE giống hệt (cùng khóa, cùng scheduleId). | {"Authorization": "secretRef:owner_api_key", "Idempotency-Key": "idem-replay-001"} | EP-CancelSchedule |
| 3 | then | Kiểm tra phản hồi 200 giống lần đầu, số audit hủy không tăng, trạng thái lịch và lần chạy không đổi thêm. |  |  |
| 4 | cleanup | Xóa dữ liệu do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | count | == | 200, nội dung phản hồi giống lần đầu; số bản ghi audit hủy không tăng. |  | `TORC-SCHEDULED-TRANSFER-EP-CancelSchedule-idem` |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-access-p73a390-c

Chứng minh hệ thống từ chối tạo lịch khi fromAccountId không thuộc chủ API key, trả 403 FORBIDDEN và không tạo lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-access-p73a390-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- API key hợp lệ của người A đã được cấp
- Tài khoản nguồn thuộc người B tồn tại trong hệ thống

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-0eb782` Tạo API key hợp lệ cho người A và tài khoản fromAccountId thuộc người B. / dọn: Xóa tài khoản, API key và dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị API key hợp lệ của người A và một tài khoản thuộc người B; ghi lại tổng số lịch hiện có. |  |  |
| 2 | when | Gửi POST /v1/scheduled-transfers bằng API key của người A với fromAccountId là tài khoản của người B. | {"body": {"amount": 100000, "fromAccountId": "ACC-B-001", "recurrence": "ONCE", "toAccountId": "ACC-A-002"}, "header": {"X-API-Key": "secretRef:apiKeyUserA"}} | EP-CreateSchedule |
| 3 | then | Kiểm tra phản hồi là HTTP 403 với errorCode FORBIDDEN và tổng số lịch không đổi. |  |  |
| 4 | cleanup | Xóa dữ liệu fixture đã tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 403 FORBIDDEN; không có bản ghi lịch mới. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-040-5-p73a390` |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-audit-p28bd0d-c

Chứng minh tạo lịch thành công (201) ghi một bản ghi audit đầy đủ các trường.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-audit-p28bd0d-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- API key hợp lệ
- Tài khoản nguồn còn dưới 20 lịch ACTIVE/PAUSED
- Kho audit đang truy cập được

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-15cab8` Chuẩn bị tài khoản và API key hợp lệ để tạo lịch thành công. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu tạo lịch hợp lệ và ghi lại số bản ghi audit tạo lịch hiện có. |  |  |
| 2 | when | Gửi POST tạo lịch hợp lệ và nhận HTTP 201. | {"body": {"amount": 100000, "fromAccountId": "ACC-A-001", "recurrence": "ONCE", "toAccountId": "ACC-A-002"}, "header": {"Idempotency-Key": "key-audit-001", "X-API-Key": "secretRef:apiKeyUserA"}} | EP-CreateSchedule |
| 3 | then | Truy vấn kho audit và kiểm tra có một bản ghi mới đủ các trường correlationId, thời điểm, tác nhân, loại sự kiện, mã lịch. |  |  |
| 4 | cleanup | Xóa lịch và bản ghi audit thử nghiệm. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Một bản ghi audit đủ các trường nêu trên. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-048-1-p28bd0d` |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-audit-pf42db3-c

Chứng minh sau khi tạo lịch hợp lệ (201) kho audit có đúng một bản ghi sự kiện tạo lịch với đủ thông tin.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-audit-pf42db3-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- API key hợp lệ
- Tài khoản nguồn dưới 20 lịch ACTIVE/PAUSED

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-9bdce6` Chuẩn bị dữ liệu để tạo lịch hợp lệ thành công (201). / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch hợp lệ thành công và nhận HTTP 201 kèm scheduleId. |  |  |
| 2 | when | Truy vấn kho audit sau khi nhận 201 để tìm bản ghi theo scheduleId. | {"scheduleId": "<scheduleId từ phản hồi 201>"} | EP-CreateSchedule |
| 3 | then | Kiểm tra có đúng một bản ghi gồm correlationId, thời điểm, tác nhân là mã tài khoản, loại sự kiện tạo lịch và scheduleId. |  |  |
| 4 | cleanup | Xóa lịch và audit thử nghiệm. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | Một bản ghi audit chứa correlationId, thời điểm, tác nhân = mã tài khoản, loại sự kiện tạo lịch, scheduleId. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-048-1-pf42db3` |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-concurrency-concurrent-dup-c

Chứng minh hai yêu cầu đồng thời cùng Idempotency-Key và cùng nội dung chỉ tạo một lịch, yêu cầu còn lại nhận kết quả của yêu cầu thắng.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-concurrency-concurrent-dup-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- Idempotency-Key chưa từng được dùng
- Tài khoản nguồn còn dưới 20 lịch ACTIVE/PAUSED

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-570e1b` Bảo đảm Idempotency-Key chưa dùng và tài khoản nguồn còn dưới 20 lịch ACTIVE/PAUSED. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị hai yêu cầu giống hệt nhau cùng Idempotency-Key và ghi lại tổng số lịch ban đầu. |  |  |
| 2 | when | Gửi đồng thời 2 yêu cầu POST giống hệt nhau tới /v1/scheduled-transfers. | {"body": {"amount": 100000, "fromAccountId": "ACC-A-001", "recurrence": "ONCE", "toAccountId": "ACC-A-002"}, "header": {"Idempotency-Key": "key-concurrent-001", "X-API-Key": "secretRef:apiKeyUserA"}, "parallelRequests": 2} | EP-CreateSchedule |
| 3 | then | Kiểm tra chỉ có một scheduleId được tạo; yêu cầu còn lại nhận lịch của yêu cầu thắng và không tạo thêm lịch hay audit. |  |  |
| 4 | cleanup | Xóa lịch và audit thử nghiệm. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 200, scheduleId = S; tổng số lịch và số bản ghi audit tạo lịch không tăng. |  | `TORC-SCHEDULED-TRANSFER-EP-CreateSchedule-idem` |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-concurrency-pdcbc85-c

Chứng minh hai request song song cùng key K và cùng body chỉ tạo đúng một lịch và trả cùng scheduleId.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-concurrency-pdcbc85-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- Chưa có lịch nào với Idempotency-Key K

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-184a08` Đảm bảo chưa có lịch nào gắn với key K. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Xác nhận chưa có lịch với key K. |  |  |
| 2 | when | Gửi hai POST song song cùng K và cùng body. | {"body": {"amount": 50000, "fromAccountId": "ACC-A-001", "recurrence": "ONCE", "toAccountId": "ACC-A-002"}, "header": {"Idempotency-Key": "K-concurrent-002", "X-API-Key": "secretRef:apiKeyUserA"}, "parallelRequests": 2} | EP-CreateSchedule |
| 3 | then | Đếm số lịch với key K và so sánh scheduleId của hai phản hồi. |  |  |
| 4 | cleanup | Xóa lịch thử nghiệm. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | count | == | Đúng 1 lịch với key K; hai phản hồi cùng scheduleId. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-200-pdcbc85` |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-errorHandling-pe11059-c

Chứng minh hệ thống trả 404 ACCOUNT_NOT_FOUND khi toAccountId không tồn tại và không tạo lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-errorHandling-pe11059-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- fromAccountId hợp lệ thuộc chủ API key
- toAccountId không có trong hệ thống

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-77ea3a` Tạo fromAccountId hợp lệ và bảo đảm toAccountId không tồn tại. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị fromAccountId hợp lệ và một toAccountId chưa từng tồn tại; ghi lại tổng số lịch. |  |  |
| 2 | when | Gửi POST tạo lịch với toAccountId không tồn tại. | {"body": {"amount": 100000, "fromAccountId": "ACC-A-001", "recurrence": "ONCE", "toAccountId": "ACC-NOT-EXIST-999"}, "header": {"X-API-Key": "secretRef:apiKeyUserA"}} | EP-CreateSchedule |
| 3 | then | Kiểm tra phản hồi HTTP 404 ACCOUNT_NOT_FOUND và tổng số lịch không đổi. |  |  |
| 4 | cleanup | Xóa dữ liệu fixture đã tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 404 ACCOUNT_NOT_FOUND; không có bản ghi lịch mới. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-040-4-pe11059` |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-functional-p2341c6-c

Chứng minh lịch MONTHLY đặt ngày 29/30/31 được tạo và các lần chạy sau rơi đúng ngày cuối tháng với tháng ngắn.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional-p2341c6-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- API key hợp lệ
- Dưới 20 lịch ACTIVE/PAUSED

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-b51e5f` Chuẩn bị lịch MONTHLY có firstRunAt ngày 31 và maxRuns hoặc endDate hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị lịch MONTHLY có firstRunAt ngày 31 và maxRuns hoặc endDate hợp lệ. |  |  |
| 2 | when | Tạo lịch MONTHLY và xem các lần chạy PENDING sinh ra qua các tháng. | {"body": {"amount": 100000, "firstRunAt": "2027-01-31T09:00:00Z", "fromAccountId": "ACC-A-001", "maxRuns": 6, "recurrence": "MONTHLY", "toAccountId": "ACC-A-002"}, "header": {"Idempotency-Key": "key-monthly-001", "X-API-Key": "secretRef:apiKeyUserA"}} | EP-CreateSchedule |
| 3 | then | Kiểm tra lần chạy tháng 30 ngày vào ngày 30 và tháng 2 vào ngày 28 (hoặc 29 năm nhuận). |  |  |
| 4 | cleanup | Xóa lịch và các lần chạy thử nghiệm. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Lần chạy ở tháng 30 ngày vào ngày 30, tháng 2 vào ngày 28 (hoặc 29 năm nhuận). |  | `TORC-SCHEDULED-TRANSFER-AC-FR-041-4-p2341c6` |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-functional-pab4508-c

Chứng minh tạo lịch ONCE hợp lệ trả 201, có scheduleId và status ACTIVE.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-functional-pab4508-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- API key hợp lệ
- Dưới 20 lịch ACTIVE/PAUSED

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-f551ed` Chuẩn bị API key hợp lệ và tài khoản nguồn có dưới 20 lịch ACTIVE/PAUSED. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị API key hợp lệ và tài khoản nguồn có dưới 20 lịch ACTIVE/PAUSED. |  |  |
| 2 | when | Gửi POST lịch ONCE hợp lệ. | {"body": {"amount": 100000, "firstRunAt": "2027-01-15T09:00:00Z", "fromAccountId": "ACC-A-001", "recurrence": "ONCE", "toAccountId": "ACC-A-002"}, "header": {"Idempotency-Key": "key-once-001", "X-API-Key": "secretRef:apiKeyUserA"}} | EP-CreateSchedule |
| 3 | then | Kiểm tra HTTP 201, phản hồi có scheduleId và status=ACTIVE. |  |  |
| 4 | cleanup | Xóa lịch thử nghiệm. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | 201, có scheduleId, status=ACTIVE. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-040-1-pab4508` |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-guardViolation-p588a23-c

Chứng minh hệ thống từ chối lịch có fromAccountId bằng toAccountId với 422 SAME_ACCOUNT.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-guardViolation-p588a23-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- Tài khoản thuộc người gọi

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-79a4ec` Tạo tài khoản thuộc người gọi. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị tài khoản thuộc người gọi; ghi lại tổng số lịch. |  |  |
| 2 | when | Gửi POST với fromAccountId = toAccountId. | {"body": {"amount": 100000, "fromAccountId": "ACC-A-001", "recurrence": "ONCE", "toAccountId": "ACC-A-001"}, "header": {"X-API-Key": "secretRef:apiKeyUserA"}} | EP-CreateSchedule |
| 3 | then | Kiểm tra HTTP 422 SAME_ACCOUNT và tổng số lịch không đổi. |  |  |
| 4 | cleanup | Xóa dữ liệu fixture đã tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 422 SAME_ACCOUNT; không có bản ghi lịch mới. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-422-same-p588a23` |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-guardViolation-pc9af5d-c

Chứng minh biên giới hạn: lịch thứ 20 được chấp nhận (201), lịch thứ 21 bị từ chối 422 SCHEDULE_LIMIT_REACHED.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-guardViolation-pc9af5d-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- Tài khoản nguồn đang có 19 lịch ACTIVE/PAUSED

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-3fed70` Tạo sẵn 19 lịch ACTIVE/PAUSED cho tài khoản nguồn. / dọn: Xóa các lịch và dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị tài khoản nguồn có đúng 19 lịch ACTIVE/PAUSED. |  |  |
| 2 | when | Tạo lịch hợp lệ thứ 20, sau đó tạo tiếp lịch hợp lệ thứ 21. | {"body": {"amount": 100000, "fromAccountId": "ACC-A-001", "recurrence": "ONCE", "toAccountId": "ACC-A-002"}, "header": {"Idempotency-Key": "key-limit-020", "X-API-Key": "secretRef:apiKeyUserA"}, "nextRequestIdempotencyKey": "key-limit-021"} | EP-CreateSchedule |
| 3 | then | Kiểm tra lịch thứ 20 trả 201 và lịch thứ 21 trả 422 SCHEDULE_LIMIT_REACHED. |  |  |
| 4 | cleanup | Xóa toàn bộ lịch thử nghiệm. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | Lịch thứ 20 -> 201; lịch thứ 21 -> 422 SCHEDULE_LIMIT_REACHED. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-040-6-pc9af5d` |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-idempotency-p50d7db-c

Chứng minh gửi lại cùng Idempotency-Key và cùng nội dung trả lịch đã tạo (200) mà không tạo lịch mới.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-idempotency-p50d7db-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- Đã tạo lịch với Idempotency-Key K và body X

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-12ace5` Tạo sẵn lịch với Idempotency-Key K và body X. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch với Idempotency-Key K và body X, ghi lại scheduleId và tổng số lịch. |  |  |
| 2 | when | Gửi lại POST với cùng K và body X. | {"body": {"amount": 100000, "fromAccountId": "ACC-A-001", "recurrence": "ONCE", "toAccountId": "ACC-A-002"}, "header": {"Idempotency-Key": "K-replay-001", "X-API-Key": "secretRef:apiKeyUserA"}} | EP-CreateSchedule |
| 3 | then | Kiểm tra HTTP 200, cùng scheduleId và tổng số lịch không tăng. |  |  |
| 4 | cleanup | Xóa lịch thử nghiệm. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | 200, cùng scheduleId; tổng số lịch không tăng. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-200-p50d7db` |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-idempotency-payload-mismatch-c

Chứng minh dùng lại Idempotency-Key với body khác bị từ chối 409 IDEMPOTENCY_CONFLICT.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-idempotency-payload-mismatch-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- Đã có lịch tạo thành công với Idempotency-Key K

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-7f478a` Tạo lịch thành công với Idempotency-Key K và lưu scheduleId. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch thành công với Idempotency-Key K và amount 100000; ghi lại tổng số lịch. |  |  |
| 2 | when | Gửi lại khóa K với body khác (amount 200000). | {"body": {"amount": 200000, "fromAccountId": "ACC-A-001", "recurrence": "ONCE", "toAccountId": "ACC-A-002"}, "header": {"Idempotency-Key": "K-mismatch-001", "X-API-Key": "secretRef:apiKeyUserA"}} | EP-CreateSchedule |
| 3 | then | Kiểm tra HTTP 409 IDEMPOTENCY_CONFLICT, số lịch không đổi và lịch cũ giữ nguyên. |  |  |
| 4 | cleanup | Xóa lịch thử nghiệm. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 409 với errorCode IDEMPOTENCY_CONFLICT; số lịch không đổi. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-409-idem-conflict` |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-idempotency-replay-c

Chứng minh gửi lại đúng yêu cầu đã thành công trả 200 cùng scheduleId, không tạo lịch hay audit mới.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-idempotency-replay-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- Lần gửi đầu đã trả 201 với scheduleId S

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-f25cfb` Thực hiện lần gửi đầu để nhận 201 với scheduleId S. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Gửi yêu cầu lần đầu, nhận 201 với scheduleId S; ghi lại tổng số lịch và số audit tạo lịch. |  |  |
| 2 | when | Gửi lại yêu cầu y hệt lần đầu (cùng Idempotency-Key, cùng body). | {"body": {"amount": 100000, "fromAccountId": "ACC-A-001", "recurrence": "ONCE", "toAccountId": "ACC-A-002"}, "header": {"Idempotency-Key": "K-replay-002", "X-API-Key": "secretRef:apiKeyUserA"}} | EP-CreateSchedule |
| 3 | then | Kiểm tra HTTP 200, scheduleId = S, tổng số lịch và số audit tạo lịch không tăng. |  |  |
| 4 | cleanup | Xóa lịch và audit thử nghiệm. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 200, scheduleId = S; tổng số lịch và số bản ghi audit tạo lịch không tăng. |  | `TORC-SCHEDULED-TRANSFER-EP-CreateSchedule-idem` |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-inputValidation-p4e161f-c

Chứng minh lịch định kỳ thiếu cả endDate và maxRuns, hoặc maxRuns ngoài 1..60, bị từ chối 400 INVALID_REQUEST.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-inputValidation-p4e161f-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- API key hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-638315` Chuẩn bị API key hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị API key hợp lệ và ghi lại tổng số lịch. |  |  |
| 2 | when | Gửi POST recurrence DAILY không có endDate và maxRuns; lặp lại với maxRuns=0 và maxRuns=61. | {"body": {"amount": 100000, "fromAccountId": "ACC-A-001", "maxRuns": 61, "recurrence": "DAILY", "toAccountId": "ACC-A-002"}, "header": {"X-API-Key": "secretRef:apiKeyUserA"}} | EP-CreateSchedule |
| 3 | then | Kiểm tra mỗi trường hợp trả HTTP 400 INVALID_REQUEST và không tạo lịch. |  |  |
| 4 | cleanup | Xóa dữ liệu fixture đã tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | 400 INVALID_REQUEST. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-041-2-p4e161f` |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-invariant-p420954-c

Chứng minh tạo lịch không làm đổi số dư hai tài khoản và không sinh ledger_entries.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-invariant-p420954-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- API key hợp lệ
- Dưới 20 lịch ACTIVE/PAUSED

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-d08a6b` Ghi lại số dư hai tài khoản và số ledger_entries trước khi tạo. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Ghi lại số dư hai tài khoản và số ledger_entries trước khi tạo. |  |  |
| 2 | when | Gửi POST tạo lịch thành công. | {"body": {"amount": 100000, "fromAccountId": "ACC-A-001", "recurrence": "ONCE", "toAccountId": "ACC-A-002"}, "header": {"Idempotency-Key": "key-invariant-001", "X-API-Key": "secretRef:apiKeyUserA"}} | EP-CreateSchedule |
| 3 | then | So sánh số dư hai tài khoản và số ledger_entries với giá trị đã ghi trước đó. |  |  |
| 4 | cleanup | Xóa lịch thử nghiệm. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | count | == | Số dư hai tài khoản và số ledger_entries không đổi. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-040-2-p420954` |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-timing-p31f6cf-c

Chứng minh firstRunAt cách hiện tại dưới 5 phút hoặc trên 90 ngày bị từ chối bằng 422 SCHEDULE_TIME_INVALID, còn giá trị trong khoảng được chấp nhận.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-timing-p31f6cf-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- Đồng hồ hệ thống được cố định trong test
- Authorization và Idempotency-Key hợp lệ, các trường body khác hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-5507b3` Cố định đồng hồ hệ thống và chuẩn bị các trường khác hợp lệ. / dọn: Xóa dữ liệu lịch do fixture tạo và bỏ cố định đồng hồ.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Cố định đồng hồ hệ thống, chuẩn bị header Authorization (secretRef:authToken), Idempotency-Key duy nhất và body hợp lệ. |  |  |
| 2 | when | Gửi lần lượt POST /v1/scheduled-transfers với firstRunAt ở các mốc biên thời gian. | {"firstRunAt": ["now+4 phút", "now+5 phút+1 giây", "now+90 ngày", "now+90 ngày+1 giây"]} | EP-CreateSchedule |
| 3 | then | Kiểm tra mã trạng thái và errorCode của từng phản hồi theo khoảng hợp lệ 5 phút đến 90 ngày. |  |  |
| 4 | cleanup | Xóa các lịch đã tạo ở phản hồi 201 và khôi phục đồng hồ. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | Giá trị dưới 5 phút và trên 90 ngày -> 422 SCHEDULE_TIME_INVALID; giá trị trong khoảng -> 201. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-040-3-p31f6cf` |

#### TC-L2-SCHEDULED-TRANSFER-EP-GetSchedule-idempotency-p52c40c-c

Chứng minh GET một lịch là idempotent: gọi lặp lại không làm thay đổi trạng thái lịch và hai phản hồi giống nhau.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-EP-GetSchedule-idempotency-p52c40c-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-GetSchedule` | llm/approved |

Tiền điều kiện:

- Lịch chuyển tiền định kỳ tồn tại với scheduleId đã biết
- Chưa có lần chạy đến hạn trong khoảng giữa hai lần gọi
- Có Authorization Bearer hợp lệ của chủ tài khoản nguồn (secretRef:owner_api_key)

Fixture: `TFX-SCHEDULED-TRANSFER-EP-GetSchedule-284def` Tạo lịch tồn tại với scheduleId đã biết, đảm bảo chưa có lần chạy đến hạn giữa hai lần gọi (nextRunAt ở tương lai). / dọn: Xóa lịch và dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch hợp lệ, ghi lại status và nextRunAt hiện lưu trong kho dữ liệu. |  |  |
| 2 | when | Gọi GET /v1/scheduled-transfers/{scheduleId} lần thứ nhất với Authorization hợp lệ. | {"Authorization": "secretRef:owner_api_key", "scheduleId": "sch_0001"} | EP-GetSchedule |
| 3 | when | Gọi lại GET /v1/scheduled-transfers/{scheduleId} lần thứ hai với cùng scheduleId và Authorization. | {"Authorization": "secretRef:owner_api_key", "scheduleId": "sch_0001"} | EP-GetSchedule |
| 4 | then | So sánh hai phản hồi 200 (status, nextRunAt) và đối chiếu dữ liệu lịch trong kho với giá trị ghi trước đó. |  |  |
| 5 | cleanup | Xóa lịch và dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | count | == | Hai phản hồi 200 có status và nextRunAt giống nhau; dữ liệu lịch trong kho không đổi sau các lần GET. |  | `TORC-SCHEDULED-TRANSFER-EP-GetSchedule-p52c40c` |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-idempotency-p6e1377-c

Chứng minh GET danh sách lịch gọi lặp với cùng tham số khi dữ liệu không đổi trả cùng kết quả và không làm đổi trạng thái lịch hay lần chạy.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-idempotency-p6e1377-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-ListSchedules` | llm/approved |

Tiền điều kiện:

- Tài khoản có ít nhất một lịch chuyển tiền
- API key hợp lệ
- Không có thay đổi dữ liệu giữa hai lần gọi

Fixture: `TFX-SCHEDULED-TRANSFER-EP-ListSchedules-49f5f7` Tạo tài khoản có lịch, không có thay đổi giữa hai lần gọi. / dọn: Xóa lịch và tài khoản do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo tài khoản có lịch và ghi lại trạng thái lịch cùng lần chạy ban đầu. |  |  |
| 2 | when | Gửi GET danh sách lịch với cùng tham số hai lần liên tiếp, không thay đổi dữ liệu ở giữa. | {"Authorization": "secretRef:api_key_hop_le", "lanGoi": 2, "method": "GET", "path": "/v1/accounts/{accountId}/scheduled-transfers"} | EP-ListSchedules |
| 3 | then | So sánh hai phản hồi và đối chiếu trạng thái lịch, lần chạy với trạng thái ban đầu. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | Hai phản hồi HTTP 200 có cùng nội dung; trạng thái lịch và lần chạy không đổi. |  | `TORC-SCHEDULED-TRANSFER-EP-ListSchedules-p6e1377` |

#### TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-audit-p664d35-c

Chứng minh việc tạm dừng hoặc tiếp tục lịch thành công (HTTP 200) sinh bản ghi audit đầy đủ trường.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-audit-p664d35-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus` | llm/approved |

Tiền điều kiện:

- Có lịch định kỳ hợp lệ ở trạng thái ACTIVE hoặc PAUSED, thuộc tài khoản của người gọi
- API key hợp lệ: secretRef:api_key_chu_lich
- Kho audit truy vấn được theo mã lịch

Fixture: `TFX-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-332419` Tạo tài khoản chủ lịch và một lịch hợp lệ để chuyển trạng thái. / dọn: Xóa lịch, tài khoản và bản ghi audit do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch định kỳ hợp lệ ở trạng thái ACTIVE cho tài khoản chủ lịch và ghi lại scheduleId. |  |  |
| 2 | when | Gửi PATCH đổi trạng thái lịch sang PAUSED (có thể lặp lại với ACTIVE) bằng API key của chủ lịch. | {"Authorization": "secretRef:api_key_chu_lich", "scheduleId": "sch_0001", "status": "PAUSED"} | EP-UpdateScheduleStatus |
| 3 | then | Kiểm tra phản hồi 200, rồi truy vấn kho audit theo scheduleId và kiểm tra các trường correlationId, thời điểm, tác nhân, loại sự kiện, mã lịch. |  |  |
| 4 | cleanup | Xóa lịch và bản ghi audit do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | Tồn tại bản ghi audit đủ các trường correlationId, thời điểm, tác nhân = mã tài khoản, loại sự kiện, mã lịch. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-048-1-p664d35` |

#### TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-concurrency-p1e8657-c

Quan sát hành vi khi scheduler chiếm lần chạy đến hạn đúng lúc lịch bị PATCH PAUSED; kết quả mong đợi còn cần xác nhận, không suy đoán.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-concurrency-p1e8657-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus` | llm/approved |

Tiền điều kiện:

- Lịch ACTIVE có lần chạy đến hạn trong chu kỳ quét 30 giây
- Có thể điều khiển thời điểm quét của scheduler (đồng hồ giả hoặc hook)
- API key chủ lịch: secretRef:api_key_chu_lich

Fixture: `TFX-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-2183d1` Tạo lịch ACTIVE có lần chạy đến hạn trong chu kỳ quét 30 giây. / dọn: Xóa lịch và lần chạy do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE có lần chạy đến hạn trong chu kỳ quét 30 giây tới và tạm dừng scheduler ngay trước điểm chiếm lần chạy. |  |  |
| 2 | when | Cho scheduler chiếm lần chạy và gửi PATCH status=PAUSED đồng thời. | {"Authorization": "secretRef:api_key_chu_lich", "scheduleId": "sch_0002", "status": "PAUSED"} | EP-UpdateScheduleStatus |
| 3 | then | Ghi nhận trạng thái lần chạy đã chiếm (tiếp tục chạy hay bị hủy/SKIPPED) và trạng thái lịch để đối chiếu khi oracle được xác nhận. |  |  |
| 4 | cleanup | Xóa lịch và các lần chạy do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Cần xác nhận: lần chạy đã chiếm có tiếp tục chạy hay bị hủy/SKIPPED. |  | `TORC-SCHEDULED-TRANSFER-MOD-Scheduler-p1e8657` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Cancel-Authorization-inputValidation-missing-v0

Chứng minh DELETE thiếu header Authorization bị từ chối 401 UNAUTHORIZED và không làm thay đổi dữ liệu.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Cancel-Authorization-inputValidation-missing-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-CancelSchedule` | llm/approved |

Tiền điều kiện:

- Có lịch ACTIVE tồn tại

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CancelSchedule-a59b32` Tạo một lịch ACTIVE. / dọn: Xóa lịch và dữ liệu liên quan do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Cancel-Authorization-inputValidation-missing-v0` (EP-CancelSchedule, secret): Authorization (header) = secretRef:authorization_header_missing (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE và ghi nhận trạng thái lịch, lần chạy, số audit hủy. |  |  |
| 2 | when | Gửi DELETE /v1/scheduled-transfers/{scheduleId} không kèm header Authorization. | {"Authorization (header)": "<thiếu>"} | EP-CancelSchedule |
| 3 | then | Kiểm tra status 401, body errorCode = UNAUTHORIZED; lịch và lần chạy không đổi, không có audit hủy. |  |  |
| 4 | cleanup | Xóa dữ liệu do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | Status 401, body errorCode = UNAUTHORIZED. |  | `TORC-SCHEDULED-TRANSFER-RSP-Cancel-401-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Cancel-scheduleId-inputValidation-missing-v0

Chứng minh DELETE tới đường dẫn thiếu scheduleId bị từ chối và không làm thay đổi trạng thái; mã lỗi chờ làm rõ.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Cancel-scheduleId-inputValidation-missing-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-CancelSchedule` | llm/approved |

Tiền điều kiện:

- Người gọi xác thực hợp lệ (secretRef:owner_api_key)

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CancelSchedule-d36c12` Tạo người gọi có API key hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Cancel-scheduleId-inputValidation-missing-v0` (EP-CancelSchedule, none): scheduleId (path) = <thiếu> (path rỗng: /v1/scheduled-transfers/) (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị người gọi có API key hợp lệ và ghi nhận trạng thái các lịch hiện có. |  |  |
| 2 | when | Gửi DELETE /v1/scheduled-transfers/ với scheduleId (path) để trống. | {"Authorization": "secretRef:owner_api_key", "scheduleId (path)": "<thiếu>"} | EP-CancelSchedule |
| 3 | then | Kiểm tra yêu cầu bị từ chối và không có thay đổi trạng thái nào; mã HTTP và errorCode chờ làm rõ. |  |  |
| 4 | cleanup | Xóa dữ liệu do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | Bị từ chối, không có thay đổi trạng thái; mã lỗi chờ làm rõ. |  | `TORC-SCHEDULED-TRANSFER-EP-CancelSchedule-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Create-Authorization-inputValidation-missing-v0

Chứng minh yêu cầu tạo lịch thiếu header Authorization bị từ chối với HTTP 401 UNAUTHORIZED và không tạo lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-Authorization-inputValidation-missing-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- Body và Idempotency-Key hợp lệ
- Chưa có lịch nào được tạo với Idempotency-Key này

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-2b4d22` Chuẩn bị body và Idempotency-Key hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Create-Authorization-inputValidation-missing-v0` (EP-CreateSchedule, secret): Authorization (header) = secretRef:authToken (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị body hợp lệ và Idempotency-Key duy nhất, không đính kèm header Authorization. |  |  |
| 2 | when | Gửi POST /v1/scheduled-transfers không có header Authorization. | {"Authorization": "<thiếu>"} | EP-CreateSchedule |
| 3 | then | Kiểm tra phản hồi HTTP 401 với errorCode UNAUTHORIZED và không có bản ghi lịch mới. |  |  |
| 4 | cleanup | Xóa dữ liệu thử nghiệm nếu có. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 401 với errorCode UNAUTHORIZED; không có bản ghi lịch mới. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-401-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Create-IdempotencyKey-inputValidation-missing-v0

Chứng minh yêu cầu tạo lịch thiếu header Idempotency-Key bị từ chối với HTTP 400 INVALID_REQUEST và không tạo lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-IdempotencyKey-inputValidation-missing-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- Authorization hợp lệ (secretRef:authToken)
- Body hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-f93a80` Chuẩn bị Authorization và body hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Create-IdempotencyKey-inputValidation-missing-v0` (EP-CreateSchedule, none): Idempotency-Key (header) = <thiếu> (không gửi header Idempotency-Key) (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị Authorization hợp lệ và body hợp lệ, không đính kèm header Idempotency-Key. |  |  |
| 2 | when | Gửi POST /v1/scheduled-transfers không có header Idempotency-Key. | {"Idempotency-Key": "<thiếu>"} | EP-CreateSchedule |
| 3 | then | Kiểm tra phản hồi HTTP 400 với errorCode INVALID_REQUEST và không có bản ghi lịch mới. |  |  |
| 4 | cleanup | Xóa dữ liệu thử nghiệm nếu có. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Create-amount-inputValidation-max-v0

Chứng minh amount = 50000000 (cận trên hợp lệ) được chấp nhận khi tạo lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-amount-inputValidation-max-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- Authorization và Idempotency-Key hợp lệ
- Các trường body khác hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-79d26b` Tạo tài khoản nguồn thuộc người gọi và tài khoản đích; các trường khác hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Create-amount-inputValidation-max-v0` (EP-CreateSchedule, none): amount (body) = 50000000 (hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị header hợp lệ và body hợp lệ với các trường khác đúng định dạng. |  |  |
| 2 | when | Gửi POST /v1/scheduled-transfers với amount = 50000000. | {"amount": 50000000} | EP-CreateSchedule |
| 3 | then | Kiểm tra yêu cầu được chấp nhận (HTTP 201) và lịch được tạo. |  |  |
| 4 | cleanup | Xóa lịch đã tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | amount (body) = 50000000 được chấp nhận |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-422-amount-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Create-amount-inputValidation-max-v1

Chứng minh amount = 50000001 vượt cận trên bị từ chối với HTTP 422 LIMIT_EXCEEDED và không tạo lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-amount-inputValidation-max-v1` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- Authorization và Idempotency-Key hợp lệ
- Các trường body khác hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-79d26b` Tạo tài khoản nguồn thuộc người gọi và tài khoản đích; các trường khác hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Create-amount-inputValidation-max-v1` (EP-CreateSchedule, none): amount (body) = 50000001 (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị header hợp lệ và body hợp lệ với các trường khác đúng định dạng. |  |  |
| 2 | when | Gửi POST /v1/scheduled-transfers với amount = 50000001. | {"amount": 50000001} | EP-CreateSchedule |
| 3 | then | Kiểm tra phản hồi HTTP 422 với errorCode LIMIT_EXCEEDED và không có bản ghi lịch mới. |  |  |
| 4 | cleanup | Xóa dữ liệu thử nghiệm nếu có. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | 50000001 -> HTTP 422 LIMIT_EXCEEDED, không tạo lịch; 50000000 -> HTTP 201. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-422-amount-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Create-amount-inputValidation-min-v0

Chứng minh amount = 0 dưới cận dưới bị từ chối với HTTP 400 INVALID_REQUEST và không tạo lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-amount-inputValidation-min-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- Authorization và Idempotency-Key hợp lệ
- Các trường body khác hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-79d26b` Tạo tài khoản nguồn thuộc người gọi và tài khoản đích; các trường khác hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Create-amount-inputValidation-min-v0` (EP-CreateSchedule, none): amount (body) = 0 (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị header hợp lệ và body hợp lệ với các trường khác đúng định dạng. |  |  |
| 2 | when | Gửi POST /v1/scheduled-transfers với amount = 0. | {"amount": 0} | EP-CreateSchedule |
| 3 | then | Kiểm tra phản hồi HTTP 400 với errorCode INVALID_REQUEST và không có bản ghi lịch mới. |  |  |
| 4 | cleanup | Xóa dữ liệu thử nghiệm nếu có. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Create-amount-inputValidation-min-v1

Chứng minh amount = 1 (cận dưới hợp lệ) được chấp nhận khi tạo lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-amount-inputValidation-min-v1` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- Authorization và Idempotency-Key hợp lệ
- Các trường body khác hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-79d26b` Tạo tài khoản nguồn thuộc người gọi và tài khoản đích; các trường khác hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Create-amount-inputValidation-min-v1` (EP-CreateSchedule, none): amount (body) = 1 (hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị header hợp lệ và body hợp lệ với các trường khác đúng định dạng. |  |  |
| 2 | when | Gửi POST /v1/scheduled-transfers với amount = 1. | {"amount": 1} | EP-CreateSchedule |
| 3 | then | Kiểm tra yêu cầu được chấp nhận và lịch được tạo. |  |  |
| 4 | cleanup | Xóa lịch đã tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | amount (body) = 1 được chấp nhận |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Create-amount-inputValidation-missing-v0

Chứng minh yêu cầu thiếu trường bắt buộc amount bị từ chối với HTTP 400 INVALID_REQUEST và không tạo lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-amount-inputValidation-missing-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- Header hợp lệ
- Các trường body khác hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-77e61a` Tạo tài khoản nguồn thuộc người gọi; header và các trường body khác hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Create-amount-inputValidation-missing-v0` (EP-CreateSchedule, none): amount (body) = <thiếu> (body không chứa trường amount) (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị header hợp lệ và body đầy đủ các trường trừ amount. |  |  |
| 2 | when | Gửi POST /v1/scheduled-transfers với body bỏ trường amount. | {"amount": "<thiếu>"} | EP-CreateSchedule |
| 3 | then | Kiểm tra phản hồi HTTP 400 với errorCode INVALID_REQUEST và không có bản ghi lịch mới. |  |  |
| 4 | cleanup | Xóa dữ liệu thử nghiệm nếu có. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Create-endDate-inputValidation-missing-v0

Kiểm tra hành vi khi body thiếu endDate: yêu cầu bị từ chối 400 INVALID_REQUEST khi thiếu cả endDate lẫn maxRuns; lịch định kỳ có maxRuns vẫn tạo được.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-endDate-inputValidation-missing-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- Header hợp lệ
- Các trường body khác hợp lệ
- Ca kiểm thử chỉ rõ recurrence và maxRuns (không có cả endDate và maxRuns)

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-72dbb3` Chuẩn bị các trường khác hợp lệ, chỉ rõ recurrence và maxRuns của ca kiểm thử. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Create-endDate-inputValidation-missing-v0` (EP-CreateSchedule, none): endDate (body) = <thiếu> (body không chứa trường endDate) (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị header hợp lệ và body lịch định kỳ có recurrence xác định, không có endDate và không có maxRuns. |  |  |
| 2 | when | Gửi POST /v1/scheduled-transfers với body bỏ trường endDate. | {"endDate": "<thiếu>"} | EP-CreateSchedule |
| 3 | then | Kiểm tra phản hồi HTTP 400 với errorCode INVALID_REQUEST và không có bản ghi lịch mới. |  |  |
| 4 | cleanup | Xóa dữ liệu thử nghiệm nếu có. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Create-firstRunAt-inputValidation-missing-v0

Chứng minh yêu cầu thiếu trường bắt buộc firstRunAt bị từ chối với HTTP 400 INVALID_REQUEST và không tạo lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-firstRunAt-inputValidation-missing-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- Header hợp lệ
- Các trường body khác hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-77e61a` Tạo tài khoản nguồn thuộc người gọi; header và các trường body khác hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Create-firstRunAt-inputValidation-missing-v0` (EP-CreateSchedule, none): firstRunAt (body) = <thiếu> (body không chứa trường firstRunAt) (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị header hợp lệ và body đầy đủ các trường trừ firstRunAt. |  |  |
| 2 | when | Gửi POST /v1/scheduled-transfers với body bỏ trường firstRunAt. | {"firstRunAt": "<thiếu>"} | EP-CreateSchedule |
| 3 | then | Kiểm tra phản hồi HTTP 400 với errorCode INVALID_REQUEST và không có bản ghi lịch mới. |  |  |
| 4 | cleanup | Xóa dữ liệu thử nghiệm nếu có. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Create-fromAccountId-inputValidation-missing-v0

Chứng minh yêu cầu thiếu trường bắt buộc fromAccountId bị từ chối với HTTP 400 INVALID_REQUEST và không tạo lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-fromAccountId-inputValidation-missing-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- Header hợp lệ
- Các trường body khác hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-77e61a` Tạo tài khoản nguồn thuộc người gọi; header và các trường body khác hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Create-fromAccountId-inputValidation-missing-v0` (EP-CreateSchedule, none): fromAccountId (body) = <thiếu> (body không chứa trường fromAccountId) (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị header hợp lệ và body đầy đủ các trường trừ fromAccountId. |  |  |
| 2 | when | Gửi POST /v1/scheduled-transfers với body bỏ trường fromAccountId. | {"fromAccountId": "<thiếu>"} | EP-CreateSchedule |
| 3 | then | Kiểm tra phản hồi HTTP 400 với errorCode INVALID_REQUEST và không có bản ghi lịch mới. |  |  |
| 4 | cleanup | Xóa dữ liệu thử nghiệm nếu có. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Create-maxRuns-inputValidation-max-v0

Chứng minh maxRuns = 60 (cận trên hợp lệ) được chấp nhận với lịch định kỳ.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-maxRuns-inputValidation-max-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- Authorization và Idempotency-Key hợp lệ
- Lịch định kỳ, các trường body khác hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-ece279` Chuẩn bị lịch định kỳ với các trường khác hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Create-maxRuns-inputValidation-max-v0` (EP-CreateSchedule, none): maxRuns (body) = 60 (hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị header hợp lệ và body lịch định kỳ hợp lệ. |  |  |
| 2 | when | Gửi POST /v1/scheduled-transfers với maxRuns = 60. | {"maxRuns": 60} | EP-CreateSchedule |
| 3 | then | Kiểm tra yêu cầu được chấp nhận và lịch được tạo. |  |  |
| 4 | cleanup | Xóa lịch đã tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | maxRuns (body) = 60 được chấp nhận |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Create-maxRuns-inputValidation-max-v1

Chứng minh maxRuns = 61 vượt cận trên bị từ chối với HTTP 400 INVALID_REQUEST và không tạo lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-maxRuns-inputValidation-max-v1` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- Authorization và Idempotency-Key hợp lệ
- Lịch định kỳ, các trường body khác hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-ece279` Chuẩn bị lịch định kỳ với các trường khác hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Create-maxRuns-inputValidation-max-v1` (EP-CreateSchedule, none): maxRuns (body) = 61 (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị header hợp lệ và body lịch định kỳ hợp lệ. |  |  |
| 2 | when | Gửi POST /v1/scheduled-transfers với maxRuns = 61. | {"maxRuns": 61} | EP-CreateSchedule |
| 3 | then | Kiểm tra phản hồi HTTP 400 với errorCode INVALID_REQUEST và không có bản ghi lịch mới. |  |  |
| 4 | cleanup | Xóa dữ liệu thử nghiệm nếu có. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Create-maxRuns-inputValidation-min-v0

Chứng minh maxRuns = 0 dưới cận dưới bị từ chối với HTTP 400 INVALID_REQUEST và không tạo lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-maxRuns-inputValidation-min-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- Authorization và Idempotency-Key hợp lệ
- Lịch định kỳ, các trường body khác hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-ece279` Chuẩn bị lịch định kỳ với các trường khác hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Create-maxRuns-inputValidation-min-v0` (EP-CreateSchedule, none): maxRuns (body) = 0 (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị header hợp lệ và body lịch định kỳ hợp lệ. |  |  |
| 2 | when | Gửi POST /v1/scheduled-transfers với maxRuns = 0. | {"maxRuns": 0} | EP-CreateSchedule |
| 3 | then | Kiểm tra phản hồi HTTP 400 với errorCode INVALID_REQUEST và không có bản ghi lịch mới. |  |  |
| 4 | cleanup | Xóa dữ liệu thử nghiệm nếu có. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Create-maxRuns-inputValidation-min-v1

Chứng minh maxRuns = 1 (cận dưới hợp lệ) được chấp nhận với lịch định kỳ.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-maxRuns-inputValidation-min-v1` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- Authorization và Idempotency-Key hợp lệ
- Lịch định kỳ, các trường body khác hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-ece279` Chuẩn bị lịch định kỳ với các trường khác hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Create-maxRuns-inputValidation-min-v1` (EP-CreateSchedule, none): maxRuns (body) = 1 (hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị header hợp lệ và body lịch định kỳ hợp lệ. |  |  |
| 2 | when | Gửi POST /v1/scheduled-transfers với maxRuns = 1. | {"maxRuns": 1} | EP-CreateSchedule |
| 3 | then | Kiểm tra yêu cầu được chấp nhận và lịch được tạo. |  |  |
| 4 | cleanup | Xóa lịch đã tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | maxRuns (body) = 1 được chấp nhận |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Create-maxRuns-inputValidation-missing-v0

Chứng minh khi body thiếu maxRuns (và không có endDate) thì hệ thống từ chối với HTTP 400 INVALID_REQUEST và không tạo lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-maxRuns-inputValidation-missing-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- API key hợp lệ thuộc chủ tài khoản nguồn
- Các trường body khác hợp lệ
- Body chỉ định recurrence định kỳ (ví dụ WEEKLY) và không có endDate

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-257441` Tạo tài khoản nguồn thuộc người gọi và tài khoản đích; chuẩn bị body hợp lệ với recurrence định kỳ, không có endDate. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Create-maxRuns-inputValidation-missing-v0` (EP-CreateSchedule, none): maxRuns (body) = <thiếu> (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị header Authorization hợp lệ (secretRef:api_key_owner), Idempotency-Key mới, body hợp lệ với recurrence=WEEKLY và không có endDate. |  |  |
| 2 | when | Gửi POST tới EP-CreateSchedule với body đã bỏ trường maxRuns. | {"endDate": "<thiếu>", "maxRuns": "<thiếu>", "recurrence": "WEEKLY"} | EP-CreateSchedule |
| 3 | then | Kiểm tra phản hồi HTTP 400, errorCode INVALID_REQUEST và số lịch của tài khoản nguồn không tăng. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Create-recurrence-inputValidation-enum-v0

Chứng minh recurrence=ONCE là giá trị hợp lệ và được chấp nhận.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-recurrence-inputValidation-enum-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- API key hợp lệ
- Các trường body khác hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-79d26b` Tạo tài khoản nguồn thuộc người gọi và tài khoản đích; các trường khác hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Create-recurrence-inputValidation-enum-v0` (EP-CreateSchedule, none): recurrence (body) = ONCE (hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị header Authorization hợp lệ (secretRef:api_key_owner), Idempotency-Key mới và body hợp lệ. |  |  |
| 2 | when | Gửi POST tới EP-CreateSchedule với recurrence=ONCE. | {"recurrence": "ONCE"} | EP-CreateSchedule |
| 3 | then | Kiểm tra giá trị recurrence=ONCE được chấp nhận, không bị từ chối vì sai enum. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | recurrence (body) = ONCE được chấp nhận |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Create-recurrence-inputValidation-enum-v1

Chứng minh recurrence=DAILY là giá trị hợp lệ và được chấp nhận.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-recurrence-inputValidation-enum-v1` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- API key hợp lệ
- Các trường body khác hợp lệ (có endDate hoặc maxRuns)

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-79d26b` Tạo tài khoản nguồn thuộc người gọi và tài khoản đích; các trường khác hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Create-recurrence-inputValidation-enum-v1` (EP-CreateSchedule, none): recurrence (body) = DAILY (hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị header Authorization hợp lệ (secretRef:api_key_owner), Idempotency-Key mới và body hợp lệ. |  |  |
| 2 | when | Gửi POST tới EP-CreateSchedule với recurrence=DAILY. | {"recurrence": "DAILY"} | EP-CreateSchedule |
| 3 | then | Kiểm tra giá trị recurrence=DAILY được chấp nhận. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | recurrence (body) = DAILY được chấp nhận |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Create-recurrence-inputValidation-enum-v2

Chứng minh recurrence=WEEKLY là giá trị hợp lệ và được chấp nhận.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-recurrence-inputValidation-enum-v2` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- API key hợp lệ
- Các trường body khác hợp lệ (có endDate hoặc maxRuns)

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-79d26b` Tạo tài khoản nguồn thuộc người gọi và tài khoản đích; các trường khác hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Create-recurrence-inputValidation-enum-v2` (EP-CreateSchedule, none): recurrence (body) = WEEKLY (hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị header Authorization hợp lệ (secretRef:api_key_owner), Idempotency-Key mới và body hợp lệ. |  |  |
| 2 | when | Gửi POST tới EP-CreateSchedule với recurrence=WEEKLY. | {"recurrence": "WEEKLY"} | EP-CreateSchedule |
| 3 | then | Kiểm tra giá trị recurrence=WEEKLY được chấp nhận. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | recurrence (body) = WEEKLY được chấp nhận |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Create-recurrence-inputValidation-enum-v3

Chứng minh recurrence=MONTHLY là giá trị hợp lệ và được chấp nhận.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-recurrence-inputValidation-enum-v3` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- API key hợp lệ
- Các trường body khác hợp lệ (có endDate hoặc maxRuns)

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-79d26b` Tạo tài khoản nguồn thuộc người gọi và tài khoản đích; các trường khác hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Create-recurrence-inputValidation-enum-v3` (EP-CreateSchedule, none): recurrence (body) = MONTHLY (hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị header Authorization hợp lệ (secretRef:api_key_owner), Idempotency-Key mới và body hợp lệ. |  |  |
| 2 | when | Gửi POST tới EP-CreateSchedule với recurrence=MONTHLY. | {"recurrence": "MONTHLY"} | EP-CreateSchedule |
| 3 | then | Kiểm tra giá trị recurrence=MONTHLY được chấp nhận. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | recurrence (body) = MONTHLY được chấp nhận |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Create-recurrence-inputValidation-enum-v4

Chứng minh recurrence=YEARLY (ngoài tập cho phép) bị từ chối với HTTP 400 INVALID_REQUEST.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-recurrence-inputValidation-enum-v4` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- API key hợp lệ
- Các trường body khác hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-79d26b` Tạo tài khoản nguồn thuộc người gọi và tài khoản đích; các trường khác hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Create-recurrence-inputValidation-enum-v4` (EP-CreateSchedule, none): recurrence (body) = YEARLY (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị header Authorization hợp lệ (secretRef:api_key_owner), Idempotency-Key mới và body hợp lệ. |  |  |
| 2 | when | Gửi POST tới EP-CreateSchedule với recurrence=YEARLY. | {"recurrence": "YEARLY"} | EP-CreateSchedule |
| 3 | then | Kiểm tra phản hồi HTTP 400, errorCode INVALID_REQUEST và không có lịch mới được tạo. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Create-recurrence-inputValidation-missing-v0

Chứng minh thiếu trường bắt buộc recurrence thì bị từ chối với HTTP 400 INVALID_REQUEST và không tạo lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-recurrence-inputValidation-missing-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- API key hợp lệ
- Header và các trường body khác hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-77e61a` Tạo tài khoản nguồn thuộc người gọi; header và các trường body khác hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Create-recurrence-inputValidation-missing-v0` (EP-CreateSchedule, none): recurrence (body) = <thiếu> (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị header Authorization hợp lệ (secretRef:api_key_owner), Idempotency-Key mới và body hợp lệ nhưng bỏ recurrence. |  |  |
| 2 | when | Gửi POST tới EP-CreateSchedule với body không có trường recurrence. | {"recurrence": "<thiếu>"} | EP-CreateSchedule |
| 3 | then | Kiểm tra phản hồi HTTP 400, errorCode INVALID_REQUEST và không có lịch mới. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Create-toAccountId-inputValidation-missing-v0

Chứng minh thiếu trường bắt buộc toAccountId thì bị từ chối với HTTP 400 INVALID_REQUEST và không tạo lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Create-toAccountId-inputValidation-missing-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- API key hợp lệ
- Header và các trường body khác hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-77e61a` Tạo tài khoản nguồn thuộc người gọi; header và các trường body khác hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Create-toAccountId-inputValidation-missing-v0` (EP-CreateSchedule, none): toAccountId (body) = <thiếu> (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị header Authorization hợp lệ (secretRef:api_key_owner), Idempotency-Key mới và body hợp lệ nhưng bỏ toAccountId. |  |  |
| 2 | when | Gửi POST tới EP-CreateSchedule với body không có trường toAccountId. | {"toAccountId": "<thiếu>"} | EP-CreateSchedule |
| 3 | then | Kiểm tra phản hồi HTTP 400, errorCode INVALID_REQUEST và không có lịch mới. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 400 với errorCode INVALID_REQUEST; không có bản ghi lịch mới. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Get-Authorization-inputValidation-missing-v0

Chứng minh khi thiếu header Authorization (scheduleId hợp lệ), hệ thống trả HTTP 401 UNAUTHORIZED và không lộ dữ liệu lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Get-Authorization-inputValidation-missing-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-GetSchedule` | llm/approved |

Tiền điều kiện:

- Tồn tại một lịch hợp lệ với scheduleId đã biết
- Yêu cầu không có header Authorization

Fixture: `TFX-SCHEDULED-TRANSFER-EP-GetSchedule-40c67a` Tạo một lịch hợp lệ với scheduleId đã biết. / dọn: Xóa lịch và dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Get-Authorization-inputValidation-missing-v0` (EP-GetSchedule, secret): Authorization (header) = secretRef:authorization_header_missing (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch hợp lệ và ghi lại scheduleId. |  |  |
| 2 | when | Gửi GET /v1/scheduled-transfers/{scheduleId} với scheduleId hợp lệ và không kèm header Authorization. | {"Authorization (header)": "<thiếu>", "scheduleId": "sch_0001"} | EP-GetSchedule |
| 3 | then | Kiểm tra mã HTTP 401, errorCode = UNAUTHORIZED và body không chứa status hay nextRunAt. |  |  |
| 4 | cleanup | Xóa lịch do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 401, body có errorCode = UNAUTHORIZED, không chứa status hay nextRunAt. |  | `TORC-SCHEDULED-TRANSFER-RSP-Get-401-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Get-scheduleId-inputValidation-missing-v0

Chứng minh khi gọi GET với path scheduleId rỗng (API key hợp lệ), yêu cầu bị từ chối và không có phản hồi 200.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Get-scheduleId-inputValidation-missing-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-GetSchedule` | llm/approved |

Tiền điều kiện:

- API key hợp lệ (secretRef:owner_api_key)
- Đường dẫn được gọi thiếu giá trị scheduleId

Fixture: `TFX-SCHEDULED-TRANSFER-EP-GetSchedule-638315` Cấp API key hợp lệ cho người gọi thử nghiệm. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Get-scheduleId-inputValidation-missing-v0` (EP-GetSchedule, none): scheduleId (path) = <thiếu> (chuỗi rỗng ở path scheduleId) (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị API key hợp lệ trong header Authorization. |  |  |
| 2 | when | Gửi GET /v1/scheduled-transfers/ với scheduleId rỗng và Authorization hợp lệ. | {"Authorization": "secretRef:owner_api_key", "scheduleId (path)": "<thiếu>"} | EP-GetSchedule |
| 3 | then | Kiểm tra yêu cầu bị từ chối, không phải 200; ghi nhận mã HTTP/errorCode thực tế vì chờ làm rõ. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | Yêu cầu bị từ chối, không có phản hồi 200; mã HTTP và errorCode chờ làm rõ. |  | `TORC-SCHEDULED-TRANSFER-EP-GetSchedule-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-ListRuns-Authorization-inputValidation-missing-v0

Chứng minh khi GET /v1/scheduled-transfers/{scheduleId}/runs thiếu header Authorization thì hệ thống trả HTTP 401 UNAUTHORIZED và không trả dữ liệu lần chạy.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListRuns-Authorization-inputValidation-missing-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-ListRuns` | llm/approved |

Tiền điều kiện:

- Lịch với scheduleId đã tồn tại trong hệ thống
- Request không chứa header Authorization

Fixture: `TFX-SCHEDULED-TRANSFER-EP-ListRuns-a73017` Tạo lịch với scheduleId tồn tại; cấu hình client không gửi header Authorization. / dọn: Xóa lịch và dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-ListRuns-Authorization-inputValidation-missing-v0` (EP-ListRuns, secret): Authorization (header) = secretRef:api_key_ListRuns (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch chuyển tiền định kỳ có scheduleId hợp lệ; chuẩn bị request không có header Authorization. |  |  |
| 2 | when | Gửi GET /v1/scheduled-transfers/{scheduleId}/runs không kèm header Authorization. | {"Authorization (header)": "<thiếu>"} | EP-ListRuns |
| 3 | then | Kiểm tra phản hồi là HTTP 401 với errorCode UNAUTHORIZED và không có dữ liệu lần chạy. |  |  |
| 4 | cleanup | Xóa lịch và dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 401, errorCode UNAUTHORIZED |  | `TORC-SCHEDULED-TRANSFER-RSP-ListRuns-401-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-ListRuns-limit-inputValidation-max-v0

Chứng minh limit = 100, biên trên hợp lệ, được chấp nhận khi gọi GET /runs.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListRuns-limit-inputValidation-max-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-ListRuns` | llm/approved |

Tiền điều kiện:

- Caller có API key hợp lệ và là chủ lịch
- Lịch tồn tại

Fixture: `TFX-SCHEDULED-TRANSFER-EP-ListRuns-862c45` Tạo lịch thuộc caller hợp lệ và cấp API key cho caller. / dọn: Xóa lịch và dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-ListRuns-limit-inputValidation-max-v0` (EP-ListRuns, none): limit (query) = 100 (hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị lịch tồn tại thuộc caller và API key hợp lệ (secretRef:api_key_owner). |  |  |
| 2 | when | Gửi GET /v1/scheduled-transfers/{scheduleId}/runs?limit=100 với Authorization hợp lệ. | {"limit (query)": 100} | EP-ListRuns |
| 3 | then | Kiểm tra yêu cầu được chấp nhận với limit = 100, không bị lỗi kiểm tra đầu vào. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | limit (query) = 100 được chấp nhận |  | `TORC-SCHEDULED-TRANSFER-EP-ListRuns-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-ListRuns-limit-inputValidation-max-v1

Chứng minh limit = 101, vượt cận trên 100, bị từ chối với lỗi kiểm tra đầu vào và không trả dữ liệu lần chạy.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListRuns-limit-inputValidation-max-v1` | `TSET-SCHEDULED-TRANSFER-L2-EP-ListRuns` | llm/approved |

Tiền điều kiện:

- Caller có API key hợp lệ và là chủ lịch
- Lịch tồn tại

Fixture: `TFX-SCHEDULED-TRANSFER-EP-ListRuns-862c45` Tạo lịch thuộc caller hợp lệ và cấp API key cho caller. / dọn: Xóa lịch và dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-ListRuns-limit-inputValidation-max-v1` (EP-ListRuns, none): limit (query) = 101 (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị lịch tồn tại thuộc caller và API key hợp lệ (secretRef:api_key_owner). |  |  |
| 2 | when | Gửi GET /v1/scheduled-transfers/{scheduleId}/runs?limit=101 với Authorization hợp lệ. | {"limit (query)": 101} | EP-ListRuns |
| 3 | then | Kiểm tra yêu cầu bị từ chối với lỗi kiểm tra đầu vào và không có dữ liệu lần chạy. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | Yêu cầu bị từ chối với lỗi kiểm tra đầu vào; không có dữ liệu lần chạy |  | `TORC-SCHEDULED-TRANSFER-EP-ListRuns-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-ListRuns-limit-inputValidation-min-v0

Chứng minh limit = 0, dưới cận dưới 1, bị từ chối với lỗi kiểm tra đầu vào và không trả dữ liệu lần chạy.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListRuns-limit-inputValidation-min-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-ListRuns` | llm/approved |

Tiền điều kiện:

- Caller có API key hợp lệ và là chủ lịch
- Lịch tồn tại

Fixture: `TFX-SCHEDULED-TRANSFER-EP-ListRuns-862c45` Tạo lịch thuộc caller hợp lệ và cấp API key cho caller. / dọn: Xóa lịch và dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-ListRuns-limit-inputValidation-min-v0` (EP-ListRuns, none): limit (query) = 0 (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị lịch tồn tại thuộc caller và API key hợp lệ (secretRef:api_key_owner). |  |  |
| 2 | when | Gửi GET /v1/scheduled-transfers/{scheduleId}/runs?limit=0 với Authorization hợp lệ. | {"limit (query)": 0} | EP-ListRuns |
| 3 | then | Kiểm tra yêu cầu bị từ chối với lỗi kiểm tra đầu vào và không có dữ liệu lần chạy. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | Yêu cầu bị từ chối với lỗi kiểm tra đầu vào; không có dữ liệu lần chạy |  | `TORC-SCHEDULED-TRANSFER-EP-ListRuns-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-ListRuns-limit-inputValidation-min-v1

Chứng minh limit = 1, biên dưới hợp lệ, được chấp nhận khi gọi GET /runs.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListRuns-limit-inputValidation-min-v1` | `TSET-SCHEDULED-TRANSFER-L2-EP-ListRuns` | llm/approved |

Tiền điều kiện:

- Caller có API key hợp lệ và là chủ lịch
- Lịch tồn tại

Fixture: `TFX-SCHEDULED-TRANSFER-EP-ListRuns-862c45` Tạo lịch thuộc caller hợp lệ và cấp API key cho caller. / dọn: Xóa lịch và dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-ListRuns-limit-inputValidation-min-v1` (EP-ListRuns, none): limit (query) = 1 (hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị lịch tồn tại thuộc caller và API key hợp lệ (secretRef:api_key_owner). |  |  |
| 2 | when | Gửi GET /v1/scheduled-transfers/{scheduleId}/runs?limit=1 với Authorization hợp lệ. | {"limit (query)": 1} | EP-ListRuns |
| 3 | then | Kiểm tra yêu cầu được chấp nhận với limit = 1, không bị lỗi kiểm tra đầu vào. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | limit (query) = 1 được chấp nhận |  | `TORC-SCHEDULED-TRANSFER-EP-ListRuns-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-ListRuns-offset-inputValidation-min-v0

Chứng minh offset = -1, dưới cận dưới 0, bị từ chối với lỗi kiểm tra đầu vào và không trả dữ liệu lần chạy.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListRuns-offset-inputValidation-min-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-ListRuns` | llm/approved |

Tiền điều kiện:

- Caller có API key hợp lệ và là chủ lịch
- Lịch tồn tại

Fixture: `TFX-SCHEDULED-TRANSFER-EP-ListRuns-862c45` Tạo lịch thuộc caller hợp lệ và cấp API key cho caller. / dọn: Xóa lịch và dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-ListRuns-offset-inputValidation-min-v0` (EP-ListRuns, none): offset (query) = -1 (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị lịch tồn tại thuộc caller và API key hợp lệ (secretRef:api_key_owner). |  |  |
| 2 | when | Gửi GET /v1/scheduled-transfers/{scheduleId}/runs?offset=-1 với Authorization hợp lệ. | {"offset (query)": -1} | EP-ListRuns |
| 3 | then | Kiểm tra yêu cầu bị từ chối với lỗi kiểm tra đầu vào và không có dữ liệu lần chạy. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | Yêu cầu bị từ chối với lỗi kiểm tra đầu vào; không có dữ liệu lần chạy |  | `TORC-SCHEDULED-TRANSFER-EP-ListRuns-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-ListRuns-offset-inputValidation-min-v1

Chứng minh offset = 0, biên dưới hợp lệ, được chấp nhận khi gọi GET /runs.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListRuns-offset-inputValidation-min-v1` | `TSET-SCHEDULED-TRANSFER-L2-EP-ListRuns` | llm/approved |

Tiền điều kiện:

- Caller có API key hợp lệ và là chủ lịch
- Lịch tồn tại

Fixture: `TFX-SCHEDULED-TRANSFER-EP-ListRuns-862c45` Tạo lịch thuộc caller hợp lệ và cấp API key cho caller. / dọn: Xóa lịch và dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-ListRuns-offset-inputValidation-min-v1` (EP-ListRuns, none): offset (query) = 0 (hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị lịch tồn tại thuộc caller và API key hợp lệ (secretRef:api_key_owner). |  |  |
| 2 | when | Gửi GET /v1/scheduled-transfers/{scheduleId}/runs?offset=0 với Authorization hợp lệ. | {"offset (query)": 0} | EP-ListRuns |
| 3 | then | Kiểm tra yêu cầu được chấp nhận với offset = 0, không bị lỗi kiểm tra đầu vào. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | offset (query) = 0 được chấp nhận |  | `TORC-SCHEDULED-TRANSFER-EP-ListRuns-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-ListRuns-scheduleId-inputValidation-missing-v0

Chứng minh khi đường dẫn thiếu scheduleId thì yêu cầu bị từ chối với lỗi kiểm tra đầu vào và không trả danh sách lần chạy.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListRuns-scheduleId-inputValidation-missing-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-ListRuns` | llm/approved |

Tiền điều kiện:

- Caller có API key hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-ListRuns-f418f1` Cấp API key hợp lệ cho caller. / dọn: Thu hồi API key và xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-ListRuns-scheduleId-inputValidation-missing-v0` (EP-ListRuns, none): scheduleId (path) = <thiếu> (đường dẫn /v1/scheduled-transfers//runs) (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị API key hợp lệ (secretRef:api_key_owner) cho caller. |  |  |
| 2 | when | Gửi GET tới /v1/scheduled-transfers//runs với scheduleId rỗng/thiếu và Authorization hợp lệ. | {"scheduleId (path)": "<thiếu>"} | EP-ListRuns |
| 3 | then | Kiểm tra yêu cầu bị từ chối với lỗi kiểm tra đầu vào và không có dữ liệu lần chạy. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | Yêu cầu bị từ chối với lỗi kiểm tra đầu vào; không có dữ liệu lần chạy |  | `TORC-SCHEDULED-TRANSFER-EP-ListRuns-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-Authorization-inputValidation-missing-v0

Chứng minh khi gọi GET danh sách lịch thiếu header Authorization thì bị từ chối 401 UNAUTHORIZED và không trả dữ liệu lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListSchedules-Authorization-inputValidation-missing-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-ListSchedules` | llm/approved |

Tiền điều kiện:

- accountId hợp lệ
- Tham số query hợp lệ hoặc bỏ trống

Fixture: `TFX-SCHEDULED-TRANSFER-EP-ListSchedules-e5db2b` Chuẩn bị accountId hợp lệ và tham số query hợp lệ hoặc bỏ trống. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-Authorization-inputValidation-missing-v0` (EP-ListSchedules, secret): Authorization (header) = secretRef:api_key_hop_le (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị accountId hợp lệ và các tham số query hợp lệ hoặc bỏ trống. |  |  |
| 2 | when | Gửi GET không kèm header Authorization. | {"Authorization": "<thiếu header Authorization>"} | EP-ListSchedules |
| 3 | then | Kiểm tra mã HTTP, errorCode và body không chứa phần tử lịch. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 401, errorCode UNAUTHORIZED, body không chứa phần tử lịch. |  | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-401-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-accountId-inputValidation-missing-v0

Chứng minh khi thiếu đoạn path accountId (/v1/accounts//scheduled-transfers) yêu cầu được xử lý theo đúng kết quả mong đợi của oracle.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListSchedules-accountId-inputValidation-missing-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-ListSchedules` | llm/approved |

Tiền điều kiện:

- Có API key hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-ListSchedules-7fbbc3` Chuẩn bị API key hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-accountId-inputValidation-missing-v0` (EP-ListSchedules, none): accountId (path) = <thiếu accountId> (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị API key hợp lệ. |  |  |
| 2 | when | Gửi GET với accountId rỗng trong path. | {"Authorization": "secretRef:api_key_hop_le", "accountId (path)": "<thiếu accountId>", "path": "/v1/accounts//scheduled-transfers"} | EP-ListSchedules |
| 3 | then | Đối chiếu phản hồi với kết quả mong đợi của assertion. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | Giá trị trong enum: HTTP 200. Giá trị ngoài enum: HTTP 400, errorCode INVALID_REQUEST. |  | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-limit-inputValidation-max-v0

Chứng minh limit=100 (biên trên hợp lệ) được chấp nhận.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListSchedules-limit-inputValidation-max-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-ListSchedules` | llm/approved |

Tiền điều kiện:

- Tài khoản của người gọi có hơn 100 lịch
- API key hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-ListSchedules-02293e` Tạo tài khoản của người gọi có hơn 100 lịch; API key hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-limit-inputValidation-max-v0` (EP-ListSchedules, none): limit (query) = 100 (hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo tài khoản có hơn 100 lịch và chuẩn bị API key hợp lệ. |  |  |
| 2 | when | Gửi GET danh sách lịch với limit=100. | {"Authorization": "secretRef:api_key_hop_le", "limit (query)": 100} | EP-ListSchedules |
| 3 | then | Kiểm tra yêu cầu được chấp nhận. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | limit (query) = 100 được chấp nhận |  | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-limit-inputValidation-max-v1

Chứng minh limit=101 (vượt biên trên) được xử lý đúng theo kết quả mong đợi của oracle.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListSchedules-limit-inputValidation-max-v1` | `TSET-SCHEDULED-TRANSFER-L2-EP-ListSchedules` | llm/approved |

Tiền điều kiện:

- Tài khoản của người gọi có hơn 100 lịch
- API key hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-ListSchedules-02293e` Tạo tài khoản của người gọi có hơn 100 lịch; API key hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-limit-inputValidation-max-v1` (EP-ListSchedules, none): limit (query) = 101 (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo tài khoản có hơn 100 lịch và chuẩn bị API key hợp lệ. |  |  |
| 2 | when | Gửi GET danh sách lịch với limit=101. | {"Authorization": "secretRef:api_key_hop_le", "limit (query)": 101} | EP-ListSchedules |
| 3 | then | Đối chiếu phản hồi với kết quả mong đợi của assertion. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | Giá trị trong enum: HTTP 200. Giá trị ngoài enum: HTTP 400, errorCode INVALID_REQUEST. |  | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-limit-inputValidation-min-v0

Chứng minh limit=0 (dưới biên dưới) được xử lý đúng theo kết quả mong đợi của oracle.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListSchedules-limit-inputValidation-min-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-ListSchedules` | llm/approved |

Tiền điều kiện:

- Tài khoản của người gọi có ít nhất 2 lịch
- API key hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-ListSchedules-257f91` Tạo tài khoản của người gọi có ít nhất 2 lịch; API key hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-limit-inputValidation-min-v0` (EP-ListSchedules, none): limit (query) = 0 (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo tài khoản có ít nhất 2 lịch và chuẩn bị API key hợp lệ. |  |  |
| 2 | when | Gửi GET danh sách lịch với limit=0. | {"Authorization": "secretRef:api_key_hop_le", "limit (query)": 0} | EP-ListSchedules |
| 3 | then | Đối chiếu phản hồi với kết quả mong đợi của assertion. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | Giá trị trong enum: HTTP 200. Giá trị ngoài enum: HTTP 400, errorCode INVALID_REQUEST. |  | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-limit-inputValidation-min-v1

Chứng minh limit=1 (biên dưới hợp lệ) được chấp nhận.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListSchedules-limit-inputValidation-min-v1` | `TSET-SCHEDULED-TRANSFER-L2-EP-ListSchedules` | llm/approved |

Tiền điều kiện:

- Tài khoản của người gọi có ít nhất 2 lịch
- API key hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-ListSchedules-257f91` Tạo tài khoản của người gọi có ít nhất 2 lịch; API key hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-limit-inputValidation-min-v1` (EP-ListSchedules, none): limit (query) = 1 (hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo tài khoản có ít nhất 2 lịch và chuẩn bị API key hợp lệ. |  |  |
| 2 | when | Gửi GET danh sách lịch với limit=1. | {"Authorization": "secretRef:api_key_hop_le", "limit (query)": 1} | EP-ListSchedules |
| 3 | then | Kiểm tra yêu cầu được chấp nhận. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | limit (query) = 1 được chấp nhận |  | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-offset-inputValidation-min-v0

Chứng minh offset=-1 (dưới biên dưới) được xử lý đúng theo kết quả mong đợi của oracle.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListSchedules-offset-inputValidation-min-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-ListSchedules` | llm/approved |

Tiền điều kiện:

- Tài khoản của người gọi có ít nhất một lịch
- API key hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-ListSchedules-e9f591` Tạo tài khoản của người gọi có ít nhất một lịch; API key hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-offset-inputValidation-min-v0` (EP-ListSchedules, none): offset (query) = -1 (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo tài khoản có ít nhất một lịch và chuẩn bị API key hợp lệ. |  |  |
| 2 | when | Gửi GET danh sách lịch với offset=-1. | {"Authorization": "secretRef:api_key_hop_le", "offset (query)": -1} | EP-ListSchedules |
| 3 | then | Đối chiếu phản hồi với kết quả mong đợi của assertion. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | Giá trị trong enum: HTTP 200. Giá trị ngoài enum: HTTP 400, errorCode INVALID_REQUEST. |  | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-offset-inputValidation-min-v1

Chứng minh offset=0 (biên dưới hợp lệ) được chấp nhận.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListSchedules-offset-inputValidation-min-v1` | `TSET-SCHEDULED-TRANSFER-L2-EP-ListSchedules` | llm/approved |

Tiền điều kiện:

- Tài khoản của người gọi có ít nhất một lịch
- API key hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-ListSchedules-e9f591` Tạo tài khoản của người gọi có ít nhất một lịch; API key hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-offset-inputValidation-min-v1` (EP-ListSchedules, none): offset (query) = 0 (hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo tài khoản có ít nhất một lịch và chuẩn bị API key hợp lệ. |  |  |
| 2 | when | Gửi GET danh sách lịch với offset=0. | {"Authorization": "secretRef:api_key_hop_le", "offset (query)": 0} | EP-ListSchedules |
| 3 | then | Kiểm tra yêu cầu được chấp nhận. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | offset (query) = 0 được chấp nhận |  | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-status-inputValidation-enum-v0

Chứng minh status=ACTIVE (giá trị enum hợp lệ) được chấp nhận.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListSchedules-status-inputValidation-enum-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-ListSchedules` | llm/approved |

Tiền điều kiện:

- API key hợp lệ
- accountId thuộc người gọi

Fixture: `TFX-SCHEDULED-TRANSFER-EP-ListSchedules-4c6479` Chuẩn bị API key hợp lệ và tài khoản ACC-A-001 thuộc người gọi. / dọn: Xóa dữ liệu và tài khoản do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-status-inputValidation-enum-v0` (EP-ListSchedules, none): status (query) = ACTIVE (hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị API key hợp lệ và accountId thuộc người gọi. |  |  |
| 2 | when | Gửi GET danh sách lịch với status=ACTIVE. | {"Authorization": "secretRef:api_key_hop_le", "status (query)": "ACTIVE"} | EP-ListSchedules |
| 3 | then | Kiểm tra yêu cầu được chấp nhận. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | status (query) = ACTIVE được chấp nhận |  | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-status-inputValidation-enum-v1

Chứng minh status=PAUSED (giá trị enum hợp lệ) được chấp nhận.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListSchedules-status-inputValidation-enum-v1` | `TSET-SCHEDULED-TRANSFER-L2-EP-ListSchedules` | llm/approved |

Tiền điều kiện:

- API key hợp lệ
- accountId thuộc người gọi

Fixture: `TFX-SCHEDULED-TRANSFER-EP-ListSchedules-4c6479` Chuẩn bị API key hợp lệ và tài khoản ACC-A-001 thuộc người gọi. / dọn: Xóa dữ liệu và tài khoản do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-status-inputValidation-enum-v1` (EP-ListSchedules, none): status (query) = PAUSED (hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị API key hợp lệ và accountId thuộc người gọi. |  |  |
| 2 | when | Gửi GET danh sách lịch với status=PAUSED. | {"Authorization": "secretRef:api_key_hop_le", "status (query)": "PAUSED"} | EP-ListSchedules |
| 3 | then | Kiểm tra yêu cầu được chấp nhận. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | status (query) = PAUSED được chấp nhận |  | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-status-inputValidation-enum-v2

Chứng minh status=COMPLETED (giá trị enum hợp lệ) được chấp nhận.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListSchedules-status-inputValidation-enum-v2` | `TSET-SCHEDULED-TRANSFER-L2-EP-ListSchedules` | llm/approved |

Tiền điều kiện:

- API key hợp lệ
- accountId thuộc người gọi

Fixture: `TFX-SCHEDULED-TRANSFER-EP-ListSchedules-4c6479` Chuẩn bị API key hợp lệ và tài khoản ACC-A-001 thuộc người gọi. / dọn: Xóa dữ liệu và tài khoản do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-status-inputValidation-enum-v2` (EP-ListSchedules, none): status (query) = COMPLETED (hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị API key hợp lệ và accountId thuộc người gọi. |  |  |
| 2 | when | Gửi GET danh sách lịch với status=COMPLETED. | {"Authorization": "secretRef:api_key_hop_le", "status (query)": "COMPLETED"} | EP-ListSchedules |
| 3 | then | Kiểm tra yêu cầu được chấp nhận. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | status (query) = COMPLETED được chấp nhận |  | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-status-inputValidation-enum-v3

Chứng minh status=CANCELLED (giá trị enum hợp lệ) được chấp nhận.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListSchedules-status-inputValidation-enum-v3` | `TSET-SCHEDULED-TRANSFER-L2-EP-ListSchedules` | llm/approved |

Tiền điều kiện:

- API key hợp lệ
- accountId thuộc người gọi

Fixture: `TFX-SCHEDULED-TRANSFER-EP-ListSchedules-4c6479` Chuẩn bị API key hợp lệ và tài khoản ACC-A-001 thuộc người gọi. / dọn: Xóa dữ liệu và tài khoản do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-status-inputValidation-enum-v3` (EP-ListSchedules, none): status (query) = CANCELLED (hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị API key hợp lệ và accountId thuộc người gọi. |  |  |
| 2 | when | Gửi GET danh sách lịch với status=CANCELLED. | {"Authorization": "secretRef:api_key_hop_le", "status (query)": "CANCELLED"} | EP-ListSchedules |
| 3 | then | Kiểm tra yêu cầu được chấp nhận. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | status (query) = CANCELLED được chấp nhận |  | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-status-inputValidation-enum-v4

Chứng minh status=UNKNOWN (ngoài enum) được xử lý đúng theo kết quả mong đợi của oracle.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-ListSchedules-status-inputValidation-enum-v4` | `TSET-SCHEDULED-TRANSFER-L2-EP-ListSchedules` | llm/approved |

Tiền điều kiện:

- API key hợp lệ
- accountId thuộc người gọi

Fixture: `TFX-SCHEDULED-TRANSFER-EP-ListSchedules-4c6479` Chuẩn bị API key hợp lệ và tài khoản ACC-A-001 thuộc người gọi. / dọn: Xóa dữ liệu và tài khoản do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-ListSchedules-status-inputValidation-enum-v4` (EP-ListSchedules, none): status (query) = UNKNOWN (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị API key hợp lệ và accountId thuộc người gọi. |  |  |
| 2 | when | Gửi GET danh sách lịch với status=UNKNOWN. | {"Authorization": "secretRef:api_key_hop_le", "status (query)": "UNKNOWN"} | EP-ListSchedules |
| 3 | then | Đối chiếu phản hồi với kết quả mong đợi của assertion. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | Giá trị trong enum: HTTP 200. Giá trị ngoài enum: HTTP 400, errorCode INVALID_REQUEST. |  | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Patch-Authorization-inputValidation-missing-v0

Chứng minh PATCH không có header Authorization bị từ chối và lịch không đổi trạng thái.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Patch-Authorization-inputValidation-missing-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus` | llm/approved |

Tiền điều kiện:

- Lịch tồn tại, đang ACTIVE hoặc PAUSED

Fixture: `TFX-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-0fa9cb` Tạo lịch tồn tại ở trạng thái ACTIVE hoặc PAUSED. / dọn: Xóa lịch do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Patch-Authorization-inputValidation-missing-v0` (EP-UpdateScheduleStatus, secret): Authorization (header) = secretRef:api_key_chu_lich (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ở trạng thái ACTIVE và ghi lại scheduleId cùng trạng thái ban đầu. |  |  |
| 2 | when | Gửi PATCH với body status hợp lệ nhưng không kèm header Authorization. | {"Authorization": "<thiếu>", "scheduleId": "sch_0003", "status": "PAUSED"} | EP-UpdateScheduleStatus |
| 3 | then | Kiểm tra HTTP 401, errorCode UNAUTHORIZED và đọc lại lịch để xác nhận trạng thái không đổi. |  |  |
| 4 | cleanup | Xóa lịch do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 401, errorCode UNAUTHORIZED, trạng thái lịch giữ nguyên. |  | `TORC-SCHEDULED-TRANSFER-RSP-Patch-401-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Patch-scheduleId-inputValidation-missing-v0

Kiểm tra phản hồi khi PATCH thiếu scheduleId ở path; kết quả mong đợi của oracle cần xác nhận.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Patch-scheduleId-inputValidation-missing-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus` | llm/approved |

Tiền điều kiện:

- Hệ thống sẵn sàng, dữ liệu nền đã nạp
- API key hợp lệ: secretRef:api_key_chu_lich

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Patch-scheduleId-inputValidation-missing-v0` (EP-UpdateScheduleStatus, none): scheduleId (path) = <thiếu> (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Khởi động hệ thống với dữ liệu nền và chuẩn bị API key hợp lệ của chủ lịch. |  |  |
| 2 | when | Gọi PATCH /v1/scheduled-transfers/ không có scheduleId, body status hợp lệ. | {"Authorization": "secretRef:api_key_chu_lich", "scheduleId": "<thiếu>", "status": "PAUSED"} | EP-UpdateScheduleStatus |
| 3 | then | Kiểm tra mã phản hồi và errorCode theo oracle, đồng thời xác nhận không lịch nào bị đổi trạng thái. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 400, errorCode INVALID_REQUEST, lịch giữ nguyên trạng thái. |  | `TORC-SCHEDULED-TRANSFER-RSP-Patch-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Patch-status-inputValidation-enum-v0

Chứng minh status = PAUSED thuộc tập hợp lệ và được chấp nhận.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Patch-status-inputValidation-enum-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus` | llm/approved |

Tiền điều kiện:

- Lịch tồn tại, người gọi là chủ lịch, API key hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-b4d9cb` Tạo lịch tồn tại, chủ lịch và API key hợp lệ. / dọn: Xóa lịch và dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Patch-status-inputValidation-enum-v0` (EP-UpdateScheduleStatus, none): status (body) = PAUSED (hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE thuộc chủ lịch và chuẩn bị API key hợp lệ. |  |  |
| 2 | when | Gửi PATCH với status=PAUSED. | {"Authorization": "secretRef:api_key_chu_lich", "scheduleId": "sch_0004", "status": "PAUSED"} | EP-UpdateScheduleStatus |
| 3 | then | Kiểm tra giá trị status = PAUSED được hệ thống chấp nhận. |  |  |
| 4 | cleanup | Xóa lịch do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | status (body) = PAUSED được chấp nhận |  | `TORC-SCHEDULED-TRANSFER-RSP-Patch-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Patch-status-inputValidation-enum-v1

Chứng minh status = ACTIVE thuộc tập hợp lệ và được chấp nhận.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Patch-status-inputValidation-enum-v1` | `TSET-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus` | llm/approved |

Tiền điều kiện:

- Lịch tồn tại, người gọi là chủ lịch, API key hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-b4d9cb` Tạo lịch tồn tại, chủ lịch và API key hợp lệ. / dọn: Xóa lịch và dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Patch-status-inputValidation-enum-v1` (EP-UpdateScheduleStatus, none): status (body) = ACTIVE (hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch định kỳ ở trạng thái PAUSED thuộc chủ lịch và chuẩn bị API key hợp lệ. |  |  |
| 2 | when | Gửi PATCH với status=ACTIVE. | {"Authorization": "secretRef:api_key_chu_lich", "scheduleId": "sch_0005", "status": "ACTIVE"} | EP-UpdateScheduleStatus |
| 3 | then | Kiểm tra giá trị status = ACTIVE được hệ thống chấp nhận. |  |  |
| 4 | cleanup | Xóa lịch do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | status (body) = ACTIVE được chấp nhận |  | `TORC-SCHEDULED-TRANSFER-RSP-Patch-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Patch-status-inputValidation-enum-v2

Chứng minh status có giá trị ngoài tập {PAUSED, ACTIVE} bị từ chối với 400.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Patch-status-inputValidation-enum-v2` | `TSET-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus` | llm/approved |

Tiền điều kiện:

- Lịch tồn tại, người gọi là chủ lịch, API key hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-b4d9cb` Tạo lịch tồn tại, chủ lịch và API key hợp lệ. / dọn: Xóa lịch và dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Patch-status-inputValidation-enum-v2` (EP-UpdateScheduleStatus, none): status (body) = INVALID_STATUS (giá trị ngoài tập) (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch thuộc chủ lịch, ghi lại trạng thái ban đầu và chuẩn bị API key hợp lệ. |  |  |
| 2 | when | Gửi PATCH với status là chuỗi không thuộc enum. | {"Authorization": "secretRef:api_key_chu_lich", "scheduleId": "sch_0006", "status": "INVALID_STATUS"} | EP-UpdateScheduleStatus |
| 3 | then | Kiểm tra HTTP 400, errorCode INVALID_REQUEST và trạng thái lịch không đổi. |  |  |
| 4 | cleanup | Xóa lịch do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 400, errorCode INVALID_REQUEST, lịch giữ nguyên trạng thái. |  | `TORC-SCHEDULED-TRANSFER-RSP-Patch-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FLD-Patch-status-inputValidation-missing-v0

Chứng minh body PATCH thiếu trường bắt buộc status bị từ chối với 400.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FLD-Patch-status-inputValidation-missing-v0` | `TSET-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus` | llm/approved |

Tiền điều kiện:

- Lịch tồn tại, người gọi là chủ lịch, API key hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-b4d9cb` Tạo lịch tồn tại, chủ lịch và API key hợp lệ. / dọn: Xóa lịch và dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-FLD-Patch-status-inputValidation-missing-v0` (EP-UpdateScheduleStatus, none): status (body) = <thiếu> (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch thuộc chủ lịch, ghi lại trạng thái ban đầu và chuẩn bị API key hợp lệ. |  |  |
| 2 | when | Gửi PATCH với body rỗng, không chứa trường status. | {"Authorization": "secretRef:api_key_chu_lich", "body": {}, "scheduleId": "sch_0007"} | EP-UpdateScheduleStatus |
| 3 | then | Kiểm tra HTTP 400, errorCode INVALID_REQUEST và trạng thái lịch không đổi. |  |  |
| 4 | cleanup | Xóa lịch do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 400, errorCode INVALID_REQUEST, lịch giữ nguyên trạng thái. |  | `TORC-SCHEDULED-TRANSFER-RSP-Patch-400-validation` |

#### TC-L2-SCHEDULED-TRANSFER-FR-040-access-p4f809f-c

Chứng minh thiếu hoặc sai API key bị từ chối với 401 UNAUTHORIZED.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-040-access-p4f809f-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-040` | llm/approved |

Tiền điều kiện:

- Không có header Authorization, hoặc dùng key sai

Fixture: `TFX-SCHEDULED-TRANSFER-FR-040-5568af` Chuẩn bị yêu cầu không có hoặc có API key sai. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị hai yêu cầu: không có Authorization và có Authorization với key sai. |  |  |
| 2 | when | Gửi POST tạo lịch không có header Authorization. | {"Authorization": null} | FR-040 |
| 3 | when | Gửi POST tạo lịch với API key sai. | {"Authorization": "secretRef:invalid_api_key"} | FR-040 |
| 4 | then | Kiểm tra cả hai trả HTTP 401, errorCode=UNAUTHORIZED và không có bản ghi lịch mới. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 401, errorCode=UNAUTHORIZED; không có bản ghi lịch mới. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-401-p4f809f` |

#### TC-L2-SCHEDULED-TRANSFER-FR-040-audit-p7a419a-c

Chứng minh tạo lịch thành công được ghi audit.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-040-audit-p7a419a-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-040` | llm/approved |

Tiền điều kiện:

- Yêu cầu tạo lịch hợp lệ với API key secretRef:api_key_owner_a

Fixture: `TFX-SCHEDULED-TRANSFER-FR-040-a8115b` Chuẩn bị dữ liệu cho yêu cầu tạo lịch hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu tạo lịch hợp lệ. |  |  |
| 2 | when | Gửi POST tạo lịch và nhận HTTP 201 kèm scheduleId. | {"Idempotency-Key": "key-0007", "recurrence": "ONCE"} | FR-040 |
| 3 | then | Truy vấn audit log theo scheduleId đã trả và xác nhận có bản ghi tạo lịch. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | Tồn tại bản ghi audit tạo lịch cho scheduleId đã trả. |  | `TORC-SCHEDULED-TRANSFER-FR-048-p7a419a` |

#### TC-L2-SCHEDULED-TRANSFER-FR-040-concurrency-pd7d28d-c

Chứng minh hai yêu cầu đồng thời khi có 19 lịch không làm vượt 20 lịch ACTIVE/PAUSED.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-040-concurrency-pd7d28d-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-040` | llm/approved |

Tiền điều kiện:

- Tài khoản nguồn có 19 lịch ACTIVE/PAUSED

Fixture: `TFX-SCHEDULED-TRANSFER-FR-040-10a9b6` Tạo tài khoản nguồn có 19 lịch ACTIVE/PAUSED. / dọn: Xóa các lịch và dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị tài khoản nguồn có 19 lịch ACTIVE/PAUSED và hai yêu cầu hợp lệ. |  |  |
| 2 | when | Gửi đồng thời hai POST tạo lịch với Idempotency-Key khác nhau. | {"Idempotency-Key": ["key-0008a", "key-0008b"], "requests": 2} | FR-040 |
| 3 | then | Kiểm tra tối đa một yêu cầu trả 201, yêu cầu còn lại 422 SCHEDULE_LIMIT_REACHED và tổng lịch ACTIVE/PAUSED không quá 20. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | count | == | Số lịch ACTIVE/PAUSED của tài khoản luôn ≤ 20. |  | `TORC-SCHEDULED-TRANSFER-DE-ScheduledTransfer-pd7d28d` |

#### TC-L2-SCHEDULED-TRANSFER-FR-040-decisionRule-pc6e1a6-c

Chứng minh amount vượt hạn mức hoặc trùng tài khoản bị từ chối, kiểm tra biên 50.000.000 và 50.000.001.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-040-decisionRule-pc6e1a6-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-040` | llm/approved |

Tiền điều kiện:

- API key hợp lệ: secretRef:api_key_owner_a
- Các trường khác hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-FR-040-77c61f` Chuẩn bị API key hợp lệ và các trường khác hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị API key hợp lệ và yêu cầu hợp lệ cho các trường khác. |  |  |
| 2 | when | Gửi POST với amount=50.000.000 để xác nhận biên được chấp nhận. | {"amount": 50000000} | FR-040 |
| 3 | when | Gửi POST với amount=50.000.001. | {"amount": 50000001} | FR-040 |
| 4 | when | Gửi POST với fromAccountId bằng toAccountId. | {"fromAccountId": "ACC-A", "toAccountId": "ACC-A"} | FR-040 |
| 5 | then | Kiểm tra các ca bị từ chối trả 422 với LIMIT_EXCEEDED hoặc SAME_ACCOUNT tương ứng và không có bản ghi lịch mới. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 422 với errorCode LIMIT_EXCEEDED hoặc SAME_ACCOUNT tương ứng; không có bản ghi lịch mới. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-422-amount-pc6e1a6` |

#### TC-L2-SCHEDULED-TRANSFER-FR-040-idempotency-p8f0346-c

Chứng minh gửi lại cùng Idempotency-Key và cùng nội dung trả lịch đã tạo, không tạo lịch mới.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-040-idempotency-p8f0346-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-040` | llm/approved |

Tiền điều kiện:

- Đã tạo lịch thành công với Idempotency-Key K

Fixture: `TFX-SCHEDULED-TRANSFER-FR-040-596acf` Tạo lịch thành công với Idempotency-Key K. / dọn: Xóa lịch và dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch thành công với Idempotency-Key K và ghi lại scheduleId. |  |  |
| 2 | when | Gửi lại POST với cùng K và cùng nội dung. | {"Idempotency-Key": "K-0009", "body": "giống lần đầu"} | FR-040 |
| 3 | then | Kiểm tra HTTP 200, cùng scheduleId như lần đầu và chỉ có một bản ghi lịch. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 200, cùng scheduleId như lần đầu; chỉ có một bản ghi lịch. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-200-p8f0346` |

#### TC-L2-SCHEDULED-TRANSFER-FR-040-idempotency-pccb8db-c

Chứng minh cùng Idempotency-Key nhưng nội dung khác bị từ chối với 409 IDEMPOTENCY_CONFLICT.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-040-idempotency-pccb8db-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-040` | llm/approved |

Tiền điều kiện:

- Đã tạo lịch với Idempotency-Key K

Fixture: `TFX-SCHEDULED-TRANSFER-FR-040-26d9ac` Tạo lịch với Idempotency-Key K. / dọn: Xóa lịch và dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch với Idempotency-Key K và ghi lại số lịch hiện có. |  |  |
| 2 | when | Gửi POST với cùng K nhưng amount hoặc toAccountId khác. | {"Idempotency-Key": "K-0010", "amount": 2000000} | FR-040 |
| 3 | then | Kiểm tra HTTP 409, errorCode=IDEMPOTENCY_CONFLICT và số lịch không đổi. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 409, errorCode=IDEMPOTENCY_CONFLICT; số lịch không đổi. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-409-pccb8db` |

#### TC-L2-SCHEDULED-TRANSFER-FR-040-inputValidation-p49f012-c

Chứng minh đầu vào sai định dạng hoặc thiếu trường bắt buộc bị từ chối với 400 INVALID_REQUEST.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-040-inputValidation-p49f012-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-040` | llm/approved |

Tiền điều kiện:

- API key hợp lệ: secretRef:api_key_owner_a

Fixture: `TFX-SCHEDULED-TRANSFER-FR-040-638315` Chuẩn bị API key hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị API key hợp lệ và yêu cầu nền hợp lệ để thay từng giá trị sai riêng lẻ. |  |  |
| 2 | when | Gửi lần lượt từng POST với một giá trị sai: thiếu Idempotency-Key, amount=0, recurrence=YEARLY, maxRuns=61, firstRunAt sai định dạng. | {"cases": ["thiếu Idempotency-Key", "amount=0", "recurrence=YEARLY", "maxRuns=61", "firstRunAt=\\"không-phải-ngày\\""]} | FR-040 |
| 3 | then | Kiểm tra mỗi ca trả HTTP 400, errorCode=INVALID_REQUEST và không có bản ghi lịch mới. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 400, errorCode=INVALID_REQUEST cho mỗi trường hợp; không có bản ghi lịch mới. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-p49f012` |

#### TC-L2-SCHEDULED-TRANSFER-FR-041-access-pfae77d-c

Chứng minh tạo lịch bị từ chối khi fromAccountId không thuộc người gọi (403) hoặc API key thiếu/sai (401).

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-041-access-pfae77d-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-041` | llm/approved |

Tiền điều kiện:

- Hai tài khoản tồn tại, người gọi chỉ sở hữu một

Fixture: `TFX-SCHEDULED-TRANSFER-FR-041-d8af9c` Tạo hai tài khoản, người gọi chỉ sở hữu một. / dọn: Xóa các tài khoản do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị hai tài khoản, người gọi chỉ sở hữu một tài khoản. |  |  |
| 2 | when | Gửi POST tạo lịch với fromAccountId của tài khoản khác dùng API key hợp lệ. | fromAccountId=acc-khac-chu; Authorization=secretRef:caller_api_key | FR-041 |
| 3 | when | Gửi POST tạo lịch khi thiếu Authorization, rồi khi Authorization sai. | Authorization=(bỏ trống) \| secretRef:invalid_api_key | FR-041 |
| 4 | then | Kiểm tra 403 FORBIDDEN cho sai chủ tài khoản, 401 UNAUTHORIZED cho API key thiếu/sai và không có lịch nào được tạo. |  |  |
| 5 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 403 FORBIDDEN cho sai chủ tài khoản; HTTP 401 UNAUTHORIZED cho API key thiếu/sai. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-403-pfae77d` |

#### TC-L2-SCHEDULED-TRANSFER-FR-041-audit-p1cd2c4-c

Chứng minh tạo lịch định kỳ thành công (201) được ghi bản ghi audit theo mã lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-041-audit-p1cd2c4-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-041` | llm/approved |

Tiền điều kiện:

- Yêu cầu tạo lịch định kỳ hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-FR-041-b28e83` Chuẩn bị yêu cầu tạo lịch định kỳ hợp lệ. / dọn: Xóa lịch và bản ghi audit test do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu tạo lịch định kỳ hợp lệ với Idempotency-Key mới. |  |  |
| 2 | when | Gửi POST /v1/scheduled-transfers và nhận 201. | recurrence=MONTHLY; maxRuns=6; Authorization=secretRef:caller_api_key | FR-041 |
| 3 | then | Truy vấn audit theo mã lịch vừa tạo và kiểm tra có bản ghi tạo lịch. |  | MOD-Audit |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | Tồn tại bản ghi audit tạo lịch liên kết mã lịch; trường tác nhân cụ thể cho thao tác tạo của người dùng chưa nêu trong nguồn. |  | `TORC-SCHEDULED-TRANSFER-FR-048-p1cd2c4` |

#### TC-L2-SCHEDULED-TRANSFER-FR-041-guardViolation-p241776-c

Chứng minh endDate trước lần chạy đầu hoặc cách hiện tại quá 366 ngày bị từ chối 422 SCHEDULE_TIME_INVALID.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-041-guardViolation-p241776-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-041` | llm/approved |

Tiền điều kiện:

- Các trường khác của yêu cầu hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-FR-041-1f580e` Chuẩn bị yêu cầu hợp lệ ở các trường khác. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu hợp lệ ở các trường khác, firstRunAt cách hiện tại 1 ngày. |  |  |
| 2 | when | Gửi POST với endDate nhỏ hơn ngày của firstRunAt. | endDate = ngày trước firstRunAt | FR-041 |
| 3 | when | Gửi POST với endDate bằng hiện tại + 366 ngày rồi hiện tại + 367 ngày. | endDate = hiện tại+366 ngày \| hiện tại+367 ngày | FR-041 |
| 4 | then | Kiểm tra các trường hợp không hợp lệ trả 422 SCHEDULE_TIME_INVALID, không tạo lịch; biên 366 ngày được kiểm tra riêng với biên 367 ngày. |  |  |
| 5 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 422, errorCode=SCHEDULE_TIME_INVALID; kiểm tra biên 366 và 367 ngày. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-422-time-p241776` |

#### TC-L2-SCHEDULED-TRANSFER-FR-041-guardViolation-pcbab3f-c

Chứng minh tài khoản đã có 20 lịch ACTIVE/PAUSED không tạo thêm được lịch (422 SCHEDULE_LIMIT_REACHED), còn 19 lịch vẫn tạo được.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-041-guardViolation-pcbab3f-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-041` | llm/approved |

Tiền điều kiện:

- Tài khoản có đúng 20 lịch ACTIVE/PAUSED

Fixture: `TFX-SCHEDULED-TRANSFER-FR-041-5997d7` Tạo đúng 20 lịch ACTIVE/PAUSED cho tài khoản nguồn. / dọn: Xóa toàn bộ lịch của tài khoản.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo sẵn 20 lịch ACTIVE/PAUSED cho tài khoản nguồn. |  |  |
| 2 | when | Gửi POST tạo lịch định kỳ hợp lệ thứ 21. | recurrence=DAILY; maxRuns=5; Idempotency-Key mới | FR-041 |
| 3 | then | Kiểm tra HTTP 422 errorCode=SCHEDULE_LIMIT_REACHED và số lịch vẫn là 20. |  |  |
| 4 | cleanup | Xóa các lịch do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 422, errorCode=SCHEDULE_LIMIT_REACHED; với 19 lịch thì tạo thành công. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-422-limit-pcbab3f` |

#### TC-L2-SCHEDULED-TRANSFER-FR-041-idempotency-pd36026-c

Chứng minh gửi lại cùng Idempotency-Key với cùng nội dung không tạo lịch thứ hai, còn khác nội dung bị từ chối 409.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-041-idempotency-pd36026-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-041` | llm/approved |

Tiền điều kiện:

- Lịch đã được tạo thành công với Idempotency-Key K

Fixture: `TFX-SCHEDULED-TRANSFER-FR-041-e649be` Tạo lịch thành công với Idempotency-Key K. / dọn: Xóa lịch và khóa idempotency do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch thành công với Idempotency-Key K và ghi lại mã lịch cùng tổng số lịch. | Idempotency-Key=K |  |
| 2 | when | Gửi lại POST với key K và nội dung giống hệt. | Idempotency-Key=K; body giống lần đầu | FR-041 |
| 3 | when | Gửi lại POST với key K nhưng nội dung khác. | Idempotency-Key=K; maxRuns khác lần đầu | FR-041 |
| 4 | then | Kiểm tra cùng nội dung trả 200 cùng mã lịch, tổng số lịch không đổi; khác nội dung trả 409 IDEMPOTENCY_CONFLICT. |  |  |
| 5 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | Cùng nội dung: HTTP 200, cùng mã lịch, tổng số lịch không đổi. Khác nội dung: HTTP 409, IDEMPOTENCY_CONFLICT. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-409-pd36026` |

#### TC-L2-SCHEDULED-TRANSFER-FR-041-inputValidation-p31ec2a-c

Chứng minh maxRuns ngoài 1..60 hoặc không nguyên bị từ chối 400 INVALID_REQUEST khi tạo lịch định kỳ.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-041-inputValidation-p31ec2a-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-041` | llm/approved |

Tiền điều kiện:

- Các trường khác của yêu cầu hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-FR-041-1f580e` Chuẩn bị yêu cầu hợp lệ ở các trường khác. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu hợp lệ ở các trường khác. |  |  |
| 2 | when | Gửi POST với maxRuns lần lượt là 0, 61 và một giá trị không nguyên. | maxRuns = 0 \| 61 \| 2.5 | FR-041 |
| 3 | when | Gửi POST với maxRuns biên hợp lệ 1 và 60 để đối chứng. | maxRuns = 1 \| 60 | FR-041 |
| 4 | then | Kiểm tra giá trị không hợp lệ trả HTTP 400 INVALID_REQUEST và không tạo lịch; biên 1 và 60 được chấp nhận. |  |  |
| 5 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 400, errorCode=INVALID_REQUEST; kiểm tra biên 0, 1, 60, 61. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-400-p31ec2a` |

#### TC-L2-SCHEDULED-TRANSFER-FR-042-access-p2aa8d6-c

Chứng minh thiếu hoặc sai API key trên cả ba endpoint GET tra cứu lịch đều bị trả 401 UNAUTHORIZED và không lộ dữ liệu.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-042-access-p2aa8d6-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-042` | llm/approved |

Tiền điều kiện:

- Không có header Authorization hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-FR-042-68c75a` Đảm bảo yêu cầu không mang header Authorization hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu không có header Authorization và yêu cầu có API key sai: secretRef:invalid_api_key. |  |  |
| 2 | when | Gửi GET lịch, GET danh sách lịch và GET runs, lần lượt không kèm API key và kèm API key sai. | Không có Authorization; hoặc Authorization: secretRef:invalid_api_key | FR-042 |
| 3 | then | Kiểm tra cả ba endpoint trả HTTP 401 với errorCode UNAUTHORIZED, không trả dữ liệu lịch. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 401 với errorCode UNAUTHORIZED cho cả ba endpoint. |  | `TORC-SCHEDULED-TRANSFER-RSP-Get-401-p2aa8d6` |

#### TC-L2-SCHEDULED-TRANSFER-FR-042-access-p72e502-c

Chứng minh người gọi không thể liệt kê lịch của tài khoản khác: nhận 403 FORBIDDEN.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-042-access-p72e502-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-042` | llm/approved |

Tiền điều kiện:

- Người gọi có API key hợp lệ của tài khoản A: secretRef:account_a_api_key
- Tồn tại tài khoản B có lịch

Fixture: `TFX-SCHEDULED-TRANSFER-FR-042-96a20d` Tạo API key hợp lệ của tài khoản A và tài khoản B có lịch. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị API key hợp lệ của tài khoản A và accountId của tài khoản B. |  |  |
| 2 | when | Gửi GET /v1/accounts/{accountId của B}/scheduled-transfers bằng API key của A. | Authorization: secretRef:account_a_api_key; accountId=accountId của B | FR-042 |
| 3 | then | Kiểm tra HTTP 403 với errorCode FORBIDDEN và body không chứa lịch của tài khoản B. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 403 với errorCode FORBIDDEN. |  | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-403-p72e502` |

#### TC-L2-SCHEDULED-TRANSFER-FR-042-inputValidation-p433493-c

Chứng minh limit=1, limit=100 và offset=0 được chấp nhận (200) còn giá trị ngoài biên (limit=0, 101, offset=-1) bị từ chối ở danh sách lịch và danh sách runs.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-042-inputValidation-p433493-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-042` | llm/approved |

Tiền điều kiện:

- Người gọi là chủ lịch với API key hợp lệ: secretRef:owner_api_key

Fixture: `TFX-SCHEDULED-TRANSFER-FR-042-29b9a1` Tạo chủ lịch với API key hợp lệ, một lịch và các lần chạy. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị chủ lịch có API key hợp lệ cùng ít nhất một lịch và một lần chạy. |  |  |
| 2 | when | Gửi GET danh sách lịch và GET runs với các giá trị limit=0, 1, 100, 101 và offset=-1, 0. | limit ∈ {0,1,100,101}; offset ∈ {-1,0}; Authorization: secretRef:owner_api_key | FR-042 |
| 3 | then | Kiểm tra limit=1, limit=100, offset=0 trả HTTP 200; limit=0, limit=101, offset=-1 bị từ chối (mã lỗi cụ thể xem gaps). |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | limit=1, 100 và offset=0 trả 200; giá trị ngoài biên bị từ chối (mã lỗi cụ thể xem gaps). |  | `TORC-SCHEDULED-TRANSFER-FLD-ListSchedules-limit-p433493` |

#### TC-L2-SCHEDULED-TRANSFER-FR-042-inputValidation-p473406-c

Chứng minh danh sách lịch với status ngoài enum (UNKNOWN) bị từ chối bằng 400 INVALID_REQUEST.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-042-inputValidation-p473406-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-042` | llm/approved |

Tiền điều kiện:

- Người gọi là chủ tài khoản với API key hợp lệ: secretRef:owner_api_key

Fixture: `TFX-SCHEDULED-TRANSFER-FR-042-b41067` Tạo chủ tài khoản với API key hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị chủ tài khoản có API key hợp lệ và accountId của chính mình. |  |  |
| 2 | when | Gửi GET danh sách lịch với status=UNKNOWN. | status=UNKNOWN; Authorization: secretRef:owner_api_key | FR-042 |
| 3 | then | Kiểm tra HTTP 400 với errorCode INVALID_REQUEST. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 400 với errorCode INVALID_REQUEST. |  | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-400-p473406` |

#### TC-L2-SCHEDULED-TRANSFER-FR-043-access-p4bf0be-c

Chứng minh DELETE thiếu hoặc sai API key bị từ chối 401 UNAUTHORIZED và lịch không đổi.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-043-access-p4bf0be-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-043` | llm/approved |

Tiền điều kiện:

- Tồn tại lịch ACTIVE

Fixture: `TFX-SCHEDULED-TRANSFER-FR-043-a59b32` Tạo lịch ACTIVE. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE và ghi lại trạng thái. | {"scheduleId": "sch_0005"} |  |
| 2 | when | Gửi DELETE /v1/scheduled-transfers/sch_0005 không có header Authorization, rồi gửi lại với API key sai. | {"Authorization": ["(không có)", "Bearer secretRef:invalid_api_key"], "method": "DELETE", "path": "/v1/scheduled-transfers/sch_0005"} | FR-043 |
| 3 | then | Kiểm tra cả hai lần trả HTTP 401, errorCode UNAUTHORIZED và lịch vẫn ACTIVE. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 401, errorCode UNAUTHORIZED; trạng thái lịch không đổi. |  | `TORC-SCHEDULED-TRANSFER-RSP-Cancel-401-p4bf0be` |

#### TC-L2-SCHEDULED-TRANSFER-FR-043-access-pb30d3e-c

Chứng minh người không phải chủ lịch không thể hủy lịch của người khác: nhận 403 FORBIDDEN.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-043-access-pb30d3e-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-043` | llm/approved |

Tiền điều kiện:

- Lịch ACTIVE thuộc tài khoản A
- API key hợp lệ của tài khoản B: secretRef:account_b_api_key

Fixture: `TFX-SCHEDULED-TRANSFER-FR-043-86a792` Tạo lịch ACTIVE thuộc tài khoản A và API key hợp lệ của tài khoản B. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE thuộc tài khoản A và chuẩn bị API key hợp lệ của tài khoản B. | {"scheduleId": "sch_0006"} |  |
| 2 | when | Gửi DELETE /v1/scheduled-transfers/sch_0006 bằng API key của B. | {"Authorization": "Bearer secretRef:account_b_api_key", "method": "DELETE", "path": "/v1/scheduled-transfers/sch_0006"} | FR-043 |
| 3 | then | Kiểm tra HTTP 403, errorCode FORBIDDEN; lịch và các lần chạy không đổi. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 403, errorCode FORBIDDEN; trạng thái lịch không đổi. |  | `TORC-SCHEDULED-TRANSFER-RSP-Cancel-403-pb30d3e` |

#### TC-L2-SCHEDULED-TRANSFER-FR-043-audit-p1438e9-c

Chứng minh hủy lịch thành công để lại bản ghi audit gắn với scheduleId.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-043-audit-p1438e9-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-043` | llm/approved |

Tiền điều kiện:

- Lịch ACTIVE thuộc người gọi
- API key của chủ lịch: secretRef:owner_api_key

Fixture: `TFX-SCHEDULED-TRANSFER-FR-043-4dc051` Tạo lịch ACTIVE thuộc người gọi. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE của chủ lịch; ghi nhận số bản ghi audit hiện có cho scheduleId. | {"scheduleId": "sch_0007"} |  |
| 2 | when | Gửi DELETE /v1/scheduled-transfers/sch_0007 bằng API key chủ lịch và nhận HTTP 200. | {"Authorization": "Bearer secretRef:owner_api_key", "method": "DELETE", "path": "/v1/scheduled-transfers/sch_0007"} | FR-043 |
| 3 | then | Truy vấn audit log theo scheduleId và xác nhận có bản ghi cho thao tác hủy lịch. |  |  |
| 4 | cleanup | Xóa lịch và bản ghi audit do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | Tồn tại bản ghi audit cho thao tác hủy lịch gắn với scheduleId. |  | `TORC-SCHEDULED-TRANSFER-FR-048-p1438e9` |

#### TC-L2-SCHEDULED-TRANSFER-FR-043-concurrency-p4b4873-c

Chứng minh khi DELETE và scheduler chiếm run cùng lúc, kết quả nhất quán: không có run CANCELLED mà vẫn ghi sổ giao dịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-043-concurrency-p4b4873-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-043` | llm/approved |

Tiền điều kiện:

- Lịch ACTIVE có lần chạy PENDING đã đến hạn
- API key của chủ lịch: secretRef:owner_api_key

Fixture: `TFX-SCHEDULED-TRANSFER-FR-043-33c81e` Tạo lịch ACTIVE có lần chạy PENDING đến hạn. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE có một lần chạy PENDING đến hạn. | {"scheduleId": "sch_0008"} |  |
| 2 | when | Kích hoạt đồng thời DELETE /v1/scheduled-transfers/sch_0008 và scan_and_run của scheduler (dùng barrier để bắt đầu cùng lúc). | {"concurrentCalls": ["DELETE sch_0008", "scan_and_run"]} | FR-043 |
| 3 | then | Kiểm tra DELETE trả 200 hoặc 409 tương ứng kết quả chiếm, và không có run CANCELLED nào có giao dịch chuyển tiền được ghi sổ. |  |  |
| 4 | cleanup | Xóa dữ liệu lịch, run và giao dịch do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | count | == | Không tồn tại lần chạy CANCELLED có giao dịch chuyển tiền được ghi sổ; DELETE trả 200 hoặc 409 tương ứng với kết quả chiếm. |  | `TORC-SCHEDULED-TRANSFER-FR-043-p4b4873` |

#### TC-L2-SCHEDULED-TRANSFER-FR-043-errorHandling-pa62ef8-c

Chứng minh DELETE với scheduleId không tồn tại trả 404 SCHEDULE_NOT_FOUND và không thay đổi dữ liệu.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-043-errorHandling-pa62ef8-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-043` | llm/approved |

Tiền điều kiện:

- Không có lịch nào với scheduleId được dùng
- API key hợp lệ: secretRef:owner_api_key

Fixture: `TFX-SCHEDULED-TRANSFER-FR-043-b30497` Đảm bảo không có lịch với scheduleId được dùng. / dọn: Xóa dữ liệu do fixture tạo (nếu có).

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chọn scheduleId chưa tồn tại và xác nhận không có bản ghi tương ứng. | {"scheduleId": "sch_does_not_exist"} |  |
| 2 | when | Gửi DELETE /v1/scheduled-transfers/sch_does_not_exist với API key hợp lệ. | {"Authorization": "Bearer secretRef:owner_api_key", "method": "DELETE", "path": "/v1/scheduled-transfers/sch_does_not_exist"} | FR-043 |
| 3 | then | Kiểm tra HTTP 404, errorCode SCHEDULE_NOT_FOUND và không có thay đổi dữ liệu. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo (nếu có). |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 404, errorCode SCHEDULE_NOT_FOUND. |  | `TORC-SCHEDULED-TRANSFER-RSP-Cancel-404-pa62ef8` |

#### TC-L2-SCHEDULED-TRANSFER-FR-044-access-p3b5af1-c

Chứng minh người gọi không phải chủ lịch bị từ chối 403 FORBIDDEN và lịch không đổi.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-044-access-p3b5af1-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-044` | llm/approved |

Tiền điều kiện:

- Lịch thuộc tài khoản A
- Có API key hợp lệ của tài khoản B

Fixture: `TFX-SCHEDULED-TRANSFER-FR-044-fde6d5` Tạo lịch thuộc tài khoản A và API key hợp lệ của tài khoản B. / dọn: Xóa lịch, tài khoản và dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE thuộc tài khoản A; chuẩn bị API key hợp lệ của tài khoản B. | {"owner": "account_A", "scheduleId": "sch_0004"} |  |
| 2 | when | Gửi PATCH status hợp lệ lên lịch của tài khoản A bằng API key của tài khoản B. | {"Authorization": "secretRef:account_b_api_key", "body": {"status": "PAUSED"}} | FR-044 |
| 3 | then | Kiểm tra phản hồi 403 errorCode=FORBIDDEN và lịch vẫn giữ trạng thái cũ. |  |  |
| 4 | cleanup | Xóa lịch và dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 403, errorCode=FORBIDDEN, trạng thái lịch không đổi. |  | `TORC-SCHEDULED-TRANSFER-RSP-Patch-403-p3b5af1` |

#### TC-L2-SCHEDULED-TRANSFER-FR-044-access-pf4b5f6-c

Chứng minh PATCH thiếu hoặc sai API key bị từ chối 401 UNAUTHORIZED và lịch không đổi.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-044-access-pf4b5f6-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-044` | llm/approved |

Tiền điều kiện:

- Lịch đã tồn tại

Fixture: `TFX-SCHEDULED-TRANSFER-FR-044-6b10ae` Tạo một lịch tồn tại trong hệ thống. / dọn: Xóa lịch và dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE tồn tại; chuẩn bị hai yêu cầu: không có header Authorization và có Authorization sai. | {"scheduleId": "sch_0005"} |  |
| 2 | when | Gửi PATCH không có Authorization, rồi gửi PATCH với Authorization sai. | {"Authorization": "secretRef:invalid_api_key", "body": {"status": "PAUSED"}} | FR-044 |
| 3 | then | Kiểm tra mỗi phản hồi là 401 errorCode=UNAUTHORIZED và lịch không đổi trạng thái. |  |  |
| 4 | cleanup | Xóa lịch và dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 401, errorCode=UNAUTHORIZED, trạng thái lịch không đổi. |  | `TORC-SCHEDULED-TRANSFER-RSP-Patch-401-pf4b5f6` |

#### TC-L2-SCHEDULED-TRANSFER-FR-044-audit-pafc81c-c

Chứng minh mỗi lần tạm dừng hoặc tiếp tục lịch thành công đều được ghi đúng một bản ghi audit.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-044-audit-pafc81c-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-044` | llm/approved |

Tiền điều kiện:

- Lịch ACTIVE hoặc PAUSED hợp lệ
- Người gọi là chủ lịch với API key hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-FR-044-bef73c` Tạo lịch ACTIVE hợp lệ (và PAUSED cho lần tiếp tục) cùng API key của chủ lịch. / dọn: Xóa lịch và dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE và ghi nhận số bản ghi audit hiện có của lịch này. | {"scheduleId": "sch_0006", "status": "ACTIVE"} |  |
| 2 | when | Gửi PATCH thành công chuyển ACTIVE→PAUSED, sau đó PATCH chuyển PAUSED→ACTIVE. | {"Authorization": "secretRef:owner_api_key", "body": {"status": "PAUSED"}} | FR-044 |
| 3 | then | Truy vấn audit và xác nhận mỗi lần chuyển thành công có đúng một bản ghi, tham chiếu scheduleId. |  |  |
| 4 | cleanup | Xóa lịch và bản ghi audit thử nghiệm. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | Có đúng một bản ghi audit cho mỗi lần chuyển trạng thái thành công, tham chiếu scheduleId. |  | `TORC-SCHEDULED-TRANSFER-FR-048-pafc81c` |

#### TC-L2-SCHEDULED-TRANSFER-FR-044-concurrency-p5afd1f-c

Chứng minh khi PATCH PAUSED chạy đồng thời với scan_and_run đang chiếm lần chạy thì không sinh giao dịch trùng và lịch cuối ở PAUSED.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-044-concurrency-p5afd1f-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-044` | llm/approved |

Tiền điều kiện:

- Lịch ACTIVE có lần chạy đến hạn trong chu kỳ quét hiện tại

Fixture: `TFX-SCHEDULED-TRANSFER-FR-044-4b500d` Tạo lịch ACTIVE có lần chạy đến hạn trong chu kỳ quét hiện tại. / dọn: Xóa lịch, lần chạy và giao dịch do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE có lần chạy đến hạn ngay trong chu kỳ quét hiện tại; chuẩn bị cơ chế phát PATCH và scan_and_run cùng lúc. | {"scheduleId": "sch_0007", "status": "ACTIVE"} |  |
| 2 | when | Gửi PATCH status=PAUSED đồng thời với lúc scan_and_run chiếm lần chạy của lịch. | {"Authorization": "secretRef:owner_api_key", "body": {"status": "PAUSED"}} | FR-044 |
| 3 | then | Đếm giao dịch theo Idempotency-Key (mã lịch + thời điểm dự kiến) và đọc trạng thái cuối của lịch. |  |  |
| 4 | cleanup | Xóa lịch, lần chạy và giao dịch thử nghiệm. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | count | == | Không có hai giao dịch cho cùng Idempotency-Key (mã lịch + thời điểm dự kiến); lịch cuối cùng ở PAUSED. |  | `TORC-SCHEDULED-TRANSFER-RISK-005-p5afd1f` |

#### TC-L2-SCHEDULED-TRANSFER-FR-044-errorHandling-pb66379-c

Chứng minh PATCH với scheduleId không tồn tại trả 404 SCHEDULE_NOT_FOUND.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-044-errorHandling-pb66379-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-044` | llm/approved |

Tiền điều kiện:

- Không tồn tại lịch với scheduleId được dùng

Fixture: `TFX-SCHEDULED-TRANSFER-FR-044-b30497` Bảo đảm không có lịch nào với scheduleId được dùng. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chọn scheduleId chắc chắn không tồn tại và xác nhận không có bản ghi tương ứng. | {"scheduleId": "sch_nonexistent_999"} |  |
| 2 | when | Gửi PATCH status hợp lệ với scheduleId không tồn tại. | {"Authorization": "secretRef:owner_api_key", "body": {"status": "PAUSED"}} | FR-044 |
| 3 | then | Kiểm tra phản hồi 404 errorCode=SCHEDULE_NOT_FOUND và không có bản ghi nào được tạo hoặc đổi. |  |  |
| 4 | cleanup | Không cần dọn thêm; xóa dữ liệu fixture nếu có. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 404, errorCode=SCHEDULE_NOT_FOUND. |  | `TORC-SCHEDULED-TRANSFER-RSP-Patch-404-pb66379` |

#### TC-L2-SCHEDULED-TRANSFER-FR-044-inputValidation-p847ccb-c

Chứng minh PATCH với status thiếu hoặc ngoài PAUSED/ACTIVE bị từ chối 400 INVALID_REQUEST và lịch không đổi.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-044-inputValidation-p847ccb-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-044` | llm/approved |

Tiền điều kiện:

- Lịch tồn tại
- Người gọi là chủ lịch với API key hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-FR-044-f161cc` Tạo lịch tồn tại thuộc chủ lịch cùng API key hợp lệ. / dọn: Xóa lịch và dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE thuộc chủ lịch; chuẩn bị hai body: thiếu status và status=CANCELLED. | {"scheduleId": "sch_0008"} |  |
| 2 | when | Gửi PATCH với body rỗng, rồi PATCH với status ngoài enum. | {"Authorization": "secretRef:owner_api_key", "body": {"status": "CANCELLED"}} | FR-044 |
| 3 | then | Kiểm tra mỗi phản hồi là 400 errorCode=INVALID_REQUEST và lịch không đổi trạng thái. |  |  |
| 4 | cleanup | Xóa lịch và dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 400, errorCode=INVALID_REQUEST, trạng thái lịch không đổi. |  | `TORC-SCHEDULED-TRANSFER-RSP-Patch-400-p847ccb` |

#### TC-L2-SCHEDULED-TRANSFER-FR-045-concurrency-p174b2f-c

Chứng minh khi hai scheduler cùng quét một lần chạy đến hạn thì chỉ một bên chiếm được lần chạy bằng UPDATE có điều kiện.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-045-concurrency-p174b2f-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-045` | llm/approved |

Tiền điều kiện:

- Một lần chạy đến hạn chưa được chiếm
- Hai scheduler cùng đọc thấy lần chạy này

Fixture: `TFX-SCHEDULED-TRANSFER-FR-045-b26a0f` Tạo một lần chạy đến hạn chưa chiếm và hai scheduler cùng đọc thấy lần chạy này. / dọn: Dừng các scheduler và xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo một lần chạy đến hạn chưa chiếm và khởi động hai instance scheduler cùng đọc thấy lần chạy đó. |  |  |
| 2 | when | Cho cả hai scheduler thực hiện chiếm lần chạy đồng thời bằng UPDATE có điều kiện. | 2 scheduler đồng thời, cùng một runId | FR-045 |
| 3 | then | Đếm số giao dịch tạo ra cho lần chạy và xác nhận chỉ một bên thực thi. |  |  |
| 4 | cleanup | Dừng hai scheduler, xóa lần chạy và giao dịch do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | Số giao dịch trùng cho một lần chạy = 0. |  | `TORC-SCHEDULED-TRANSFER-FR-045-p174b2f` |

#### TC-L2-SCHEDULED-TRANSFER-FR-045-observability-pd8cb5a-c

Chứng minh mỗi lần chạy có correlationId riêng đi theo giao dịch, audit và lời gọi Fraud Check.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-045-observability-pd8cb5a-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-045` | llm/approved |

Tiền điều kiện:

- Có nhiều lần chạy đến hạn

Fixture: `TFX-SCHEDULED-TRANSFER-FR-045-ea2d8d` Tạo nhiều lần chạy đến hạn. / dọn: Xóa lần chạy, giao dịch và audit do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo nhiều lần chạy đến hạn thuộc các lịch hợp lệ. |  |  |
| 2 | when | Scheduler sinh correlationId khi chiếm từng lần chạy và xử lý các lần chạy. | nhiều lần chạy đến hạn | FR-045 |
| 3 | then | Với mỗi lần chạy, đối chiếu correlationId trên Transfer, bản ghi audit và lời gọi Fraud Check; kiểm tra không trùng giữa các lần chạy. |  |  |
| 4 | cleanup | Xóa các lần chạy, giao dịch và bản ghi audit do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | 100% lần chạy có correlationId riêng và khớp ở giao dịch, audit, lời gọi Fraud Check. |  | `TORC-SCHEDULED-TRANSFER-FR-045-pd8cb5a` |

#### TC-L2-SCHEDULED-TRANSFER-FR-045-timing-pb0682f-c

Chứng minh lần chạy đến hạn bắt đầu trong vòng 60 giây kể từ thời điểm dự kiến.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-045-timing-pb0682f-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-045` | llm/approved |

Tiền điều kiện:

- Nhiều lần chạy đến hạn trong tải bình thường
- Chu kỳ quét của scheduler là 30 giây

Fixture: `TFX-SCHEDULED-TRANSFER-FR-045-8edbab` Tạo nhiều lần chạy đến hạn trong tải bình thường với chu kỳ quét 30 giây. / dọn: Xóa lần chạy và giao dịch do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo nhiều lần chạy đến hạn với thời điểm dự kiến đã biết, tải bình thường, chu kỳ quét 30 giây. |  |  |
| 2 | when | Scheduler quét theo chu kỳ và bắt đầu xử lý các lần chạy đến hạn. | scan_interval=30s | FR-045 |
| 3 | then | Tính độ trễ giữa thời điểm bắt đầu và thời điểm dự kiến cho từng lần chạy, rồi xác định phân vị thứ 99. |  |  |
| 4 | cleanup | Xóa các lần chạy và giao dịch do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | Độ trễ bắt đầu <= 60 giây cho 99% lần chạy. | giây | `TORC-SCHEDULED-TRANSFER-FR-045-pb0682f` |

#### TC-L2-SCHEDULED-TRANSFER-FR-047-idempotency-pd600b3-c

Chứng minh consumer khử trùng sự kiện: cùng một sự kiện kết quả lần chạy được queue giao lại không tạo SMS thứ hai cho người gửi.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P2 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-047-idempotency-pd600b3-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-047` | llm/approved |

Tiền điều kiện:

- Sự kiện ScheduledRunCompleted đã được consumer xử lý
- sms_log đã có đúng 1 bản ghi SMS tương ứng cho lần chạy

Fixture: `TFX-SCHEDULED-TRANSFER-FR-047-7c7553` Tạo sự kiện ScheduledRunCompleted đã được xử lý và có SMS tương ứng trong sms_log. / dọn: Xóa toàn bộ dữ liệu sms_log và sự kiện do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị sự kiện ScheduledRunCompleted đã được xử lý và sms_log có 1 bản ghi cho lần chạy này. |  |  |
| 2 | when | Queue giao lại cùng sự kiện (cùng định danh sự kiện) lần thứ hai cho consumer. | Sự kiện ScheduledRunCompleted trùng, giao lần 2 | FR-047 |
| 3 | then | Đếm bản ghi sms_log theo lần chạy và loại sự kiện, xác nhận bằng 1. |  |  |
| 4 | cleanup | Xóa sms_log và dữ liệu sự kiện do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | Số bản ghi sms_log cho một lần chạy và một loại sự kiện bằng 1 sau khi giao lại. |  | `TORC-SCHEDULED-TRANSFER-SCN-Notify-pd600b3` |

#### TC-L2-SCHEDULED-TRANSFER-FR-048-guardViolation-pf87b1a-c

Chứng minh yêu cầu bị từ chối (403, 409) không ghi bản ghi audit sự kiện lịch thành công và trạng thái lịch không đổi.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-048-guardViolation-pf87b1a-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-048` | llm/approved |

Tiền điều kiện:

- Có một lịch thuộc tài khoản khác với người gọi.
- Có một lịch đã ở trạng thái PAUSED thuộc người gọi.
- Đã ghi lại số bản ghi audit sự kiện lịch hiện có.

Fixture: `TFX-SCHEDULED-TRANSFER-FR-048-8eb6e9` Tạo lịch thuộc tài khoản khác và lịch ở trạng thái PAUSED của người gọi để các yêu cầu bị từ chối. / dọn: Xóa lịch, tài khoản và dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị lịch của tài khoản khác và lịch PAUSED của người gọi; ghi lại số bản ghi audit sự kiện lịch và trạng thái từng lịch. |  |  |
| 2 | when | Người không phải chủ lịch gọi PATCH đổi trạng thái lịch của tài khoản khác, mong đợi 403 FORBIDDEN. | PATCH status=PAUSED trên lịch của tài khoản khác, header X-API-Key=secretRef:non_owner_api_key | EP-UpdateScheduleStatus |
| 3 | when | Chủ lịch gọi PATCH tạm dừng lịch đã PAUSED, mong đợi 409 SCHEDULE_STATE_INVALID. | PATCH status=PAUSED trên lịch đã PAUSED, header X-API-Key=secretRef:owner_api_key | EP-UpdateScheduleStatus |
| 4 | then | Kiểm tra phản hồi 403 FORBIDDEN và 409 SCHEDULE_STATE_INVALID; số bản ghi audit sự kiện lịch không tăng và trạng thái hai lịch không đổi. |  |  |
| 5 | cleanup | Xóa các lịch và dữ liệu test. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | Phản hồi 403 FORBIDDEN và 409 SCHEDULE_STATE_INVALID; số bản ghi audit sự kiện lịch không tăng. |  | `TORC-SCHEDULED-TRANSFER-RSP-Patch-403-pf87b1a` |

#### TC-L2-SCHEDULED-TRANSFER-FR-048-invariant-pdabb36-c

Chứng minh AuditLog chỉ thêm: các bản ghi cũ giữ nguyên mọi trường và không bị xóa sau các thao tác lịch tiếp theo.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-048-invariant-pdabb36-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-048` | llm/approved |

Tiền điều kiện:

- AuditLog đã có ít nhất một bản ghi từ thao tác lịch.
- Đã chụp snapshot đầy đủ các trường của các bản ghi audit hiện có.

Fixture: `TFX-SCHEDULED-TRANSFER-FR-048-287eac` Tạo sẵn AuditLog có bản ghi từ các thao tác lịch trước đó. / dọn: Xóa lịch và bản ghi test do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chụp snapshot toàn bộ trường của các bản ghi AuditLog hiện có và ghi lại số bản ghi. |  |  |
| 2 | when | Thực hiện thêm một thao tác lịch khác (ví dụ tạo lịch mới hoặc đổi trạng thái lịch) rồi so sánh các bản ghi cũ với snapshot. | Thao tác lịch bổ sung thành công với tài khoản hợp lệ | FR-048 |
| 3 | then | Xác nhận các bản ghi audit cũ không đổi ở mọi trường, không bị xóa, và số bản ghi chỉ tăng. |  |  |
| 4 | cleanup | Xóa dữ liệu test do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | count | == | Các bản ghi audit cũ không đổi (mọi trường) sau các thao tác tiếp theo; số bản ghi chỉ tăng. |  | `TORC-SCHEDULED-TRANSFER-DE-AuditLog-pdabb36` |

#### TC-L2-SCHEDULED-TRANSFER-FR-048-observability-p477475-c

Chứng minh correlationId riêng của mỗi lần chạy xuất hiện nhất quán trong audit, giao dịch và lời gọi Fraud Check, đạt tỉ lệ 100%.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-048-observability-p477475-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-048` | llm/approved |

Tiền điều kiện:

- Có ít nhất hai lần chạy đến hạn để scheduler xử lý.
- Fraud Check mock ghi lại correlationId của mỗi lời gọi.

Fixture: `TFX-SCHEDULED-TRANSFER-FR-048-086cb9` Tạo ít nhất hai lần chạy đến hạn cho scheduler xử lý, Fraud Check mock ghi lại correlationId. / dọn: Xóa lịch, lần chạy, giao dịch và dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị ít nhất hai lần chạy đến hạn và bật ghi nhận correlationId ở Fraud Check mock. |  |  |
| 2 | when | Để scheduler xử lý hai lần chạy liên tiếp. | Hai lần chạy đến hạn liên tiếp | SCN-ScheduledRun |
| 3 | then | Với mỗi lần chạy, đối chiếu correlationId trong bản ghi audit, giao dịch và lời gọi Fraud Check: phải trùng nhau, và khác nhau giữa hai lần chạy; tỉ lệ khớp 100%. |  |  |
| 4 | cleanup | Xóa lịch, lần chạy và dữ liệu test. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | Tỉ lệ lần chạy có correlationId riêng trong audit khớp với giao dịch và lời gọi Fraud Check là 100%. |  | `TORC-SCHEDULED-TRANSFER-NFR-014-p477475` |

#### TC-L2-SCHEDULED-TRANSFER-FR-049-concurrency-pe3c163-c

Chứng minh khi hai Scheduler cùng quét một lần chạy quá hạn trong 24 giờ thì chỉ một Scheduler chiếm được lần chạy, không sinh giao dịch trùng.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-FR-049-concurrency-pe3c163-c` | `TSET-SCHEDULED-TRANSFER-L2-FR-049` | llm/approved |

Tiền điều kiện:

- Có một ScheduledRun quá hạn không quá 24 giờ, chưa chạy.
- Hai tiến trình Scheduler sẵn sàng quét cùng cơ sở dữ liệu.

Fixture: `TFX-SCHEDULED-TRANSFER-FR-049-f311fb` Tạo một ScheduledRun quá hạn không quá 24 giờ, chưa chạy; chuẩn bị hai tiến trình Scheduler cùng quét. / dọn: Dừng các Scheduler và xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo một ScheduledRun chưa chạy, quá hạn không quá 24 giờ; khởi động hai tiến trình Scheduler và đồng bộ để chúng quét cùng lúc. | {"runStatus": "PENDING", "schedulers": 2} |  |
| 2 | when | Cho hai Scheduler quét đồng thời lần chạy này. | {"concurrentScanners": 2} | FR-049 |
| 3 | then | Kiểm tra chỉ một bên chiếm thành công, có 0 giao dịch trùng và đúng 1 giao dịch cho lần chạy. |  |  |
| 4 | cleanup | Dừng hai Scheduler và xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | 0 giao dịch trùng; đúng 1 giao dịch cho lần chạy. |  | `TORC-SCHEDULED-TRANSFER-NFR-011-pe3c163` |

#### TC-L2-SCHEDULED-TRANSFER-NFR-010-timing-p9746f5-c

Chứng minh khi có hơn 20 lần chạy đến hạn trong một chu kỳ, phần vượt bị hoãn sang chu kỳ sau mà độ trễ vẫn trong 60 s, tỷ lệ đạt >= 99%.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-NFR-010-timing-p9746f5-c` | `TSET-SCHEDULED-TRANSFER-L2-NFR-010` | llm/approved |

Tiền điều kiện:

- Đồng hồ giả đã được cài đặt
- Có 40 lần chạy đến hạn cùng một thời điểm dự kiến
- Chu kỳ quét 30 giây, giới hạn 20 lần chạy mỗi chu kỳ

Fixture: `TFX-SCHEDULED-TRANSFER-NFR-010-c59f98` Đặt đồng hồ giả và tạo 40 lần chạy đến hạn cùng một thời điểm dự kiến. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Khởi tạo đồng hồ giả và tạo 40 lần chạy đến hạn cùng một thời điểm dự kiến. | {"dueRuns": 40, "maxRunsPerCycle": 20, "scanIntervalSeconds": 30} |  |
| 2 | when | Tiến đồng hồ giả qua các chu kỳ quét 30 giây liên tiếp và ghi số lần chạy được chọn ở mỗi chu kỳ cùng độ trễ bắt đầu của từng lần chạy. | {"advanceStepSeconds": 30} | MOD-Scheduler |
| 3 | then | Kiểm tra mỗi chu kỳ chọn tối đa 20 lần chạy và tỷ lệ lần chạy có độ trễ <= 60 s đạt >= 99%; lần chạy hoãn nhiều chu kỳ được tính vào phần 1% được phép trễ. |  |  |
| 4 | cleanup | Xóa 40 lần chạy và dữ liệu do fixture tạo, khôi phục đồng hồ thật. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | Mỗi chu kỳ chọn tối đa 20 lần chạy; tỷ lệ lần chạy có độ trễ <= 60 s vẫn >= 99% khi quá tải 40 lần chạy đồng thời. | s | `TORC-SCHEDULED-TRANSFER-NFR-010-p9746f5` |

#### TC-L2-SCHEDULED-TRANSFER-NFR-010-timing-target-c

Chứng minh chỉ tiêu NFR-010: ít nhất 99% lần chạy lịch chuyển tiền bắt đầu trong 60 giây kể từ thời điểm dự kiến.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L2-NFR-010-timing-target-c` | `TSET-SCHEDULED-TRANSFER-L2-NFR-010` | llm/approved |

Tiền điều kiện:

- Scheduler chạy trong tiến trình MiniPay với chu kỳ quét 30 giây
- CSDL SQLite chung sẵn sàng
- Có các lần chạy đến hạn trong cửa sổ đo

Fixture: `TFX-SCHEDULED-TRANSFER-NFR-010-277e53` Chạy Scheduler trong tiến trình MiniPay, chu kỳ quét 30 giây, CSDL SQLite chung, và tạo các lần chạy đến hạn. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Khởi động Scheduler trong tiến trình MiniPay với chu kỳ quét 30 giây, dùng CSDL SQLite chung và nạp các lần chạy đến hạn. | {"database": "SQLite", "scanIntervalSeconds": 30} |  |
| 2 | when | Để scan_and_run quét và chiếm các lần chạy đến hạn trong cửa sổ đo; ghi thời điểm dự kiến và thời điểm bắt đầu của từng lần chạy. | {"record": ["scheduledAt", "startedAt"]} | SCN-ScheduledRun |
| 3 | then | Tính tỷ lệ lần chạy có (thời điểm bắt đầu - thời điểm dự kiến) <= 60 s và so với ngưỡng 99%. |  |  |
| 4 | cleanup | Dừng Scheduler và xóa dữ liệu lần chạy do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | Ít nhất 99% lần chạy có (thời điểm bắt đầu - thời điểm dự kiến) <= 60 s. | s | `TORC-SCHEDULED-TRANSFER-NFR-010` |

#### TC-L2-SCHEDULED-TRANSFER-NFR-011-concurrency-target-c

Chứng minh với mỗi lần chạy đến hạn, số giao dịch trùng bằng 0 khi từ hai scheduler trở lên cùng quét trong cùng chu kỳ.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-NFR-011-concurrency-target-c` | `TSET-SCHEDULED-TRANSFER-L2-NFR-011` | llm/approved |

Tiền điều kiện:

- Từ hai scheduler trở lên đang hoạt động.
- Có nhiều lần chạy đến hạn (tối đa 20 lần chạy mỗi chu kỳ quét).
- Chưa có giao dịch nào ứng với các Idempotency-Key của các lần chạy này.

Fixture: `TFX-SCHEDULED-TRANSFER-NFR-011-15c87f` Bật từ hai scheduler trở lên và tạo các lần chạy đến hạn (tối đa 20 lần chạy mỗi chu kỳ quét). / dọn: Xóa các lần chạy, giao dịch và dữ liệu do fixture tạo; dừng các scheduler.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo tối đa 20 lần chạy đến hạn trong scheduled_runs và khởi động từ hai scheduler trở lên. |  |  |
| 2 | when | Cho các scheduler cùng quét các lần chạy đến hạn trong cùng chu kỳ 30 giây. | Tối đa 20 lần chạy đến hạn; số scheduler >= 2; chu kỳ quét 30 giây | NFR-011 |
| 3 | then | Với từng lần chạy, đếm giao dịch theo Idempotency-Key và kiểm tra số giao dịch trùng bằng 0. |  |  |
| 4 | cleanup | Xóa các lần chạy, giao dịch và dữ liệu do fixture tạo; dừng các scheduler. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | Số giao dịch trùng cho mỗi lần chạy = 0. |  | `TORC-SCHEDULED-TRANSFER-NFR-011` |

#### TC-L2-SCHEDULED-TRANSFER-NFR-012-concurrency-p92271d-c

Chứng minh khi chạy bù sau khởi động lại, hai scheduler quét đồng thời không tạo giao dịch trùng cho cùng một lần chạy quá hạn.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-NFR-012-concurrency-p92271d-c` | `TSET-SCHEDULED-TRANSFER-L2-NFR-012` | llm/approved |

Tiền điều kiện:

- Hai instance scheduler dùng chung một CSDL
- Có các lần chạy quá hạn ≤ 24 giờ chưa xử lý

Fixture: `TFX-SCHEDULED-TRANSFER-NFR-012-46ce35` Khởi động lại hai instance scheduler cùng lúc với các lần chạy quá hạn ≤ 24 giờ. / dọn: Dừng hai instance và xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo các lần chạy quá hạn ≤ 24 giờ và chuẩn bị hai instance scheduler cùng khởi động lại. | {"doTre": "≤ 24h", "soInstance": 2} |  |
| 2 | when | Cho cả hai instance quét đồng thời trong 5 chu kỳ đầu. | {"quetDongThoi": true, "soChuKy": 5} | NFR-012 |
| 3 | then | Đếm số giao dịch theo từng lần chạy và xác nhận mỗi lần chạy chỉ được một scheduler chiếm. |  |  |
| 4 | cleanup | Dừng hai instance và xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | 0 giao dịch trùng cho mỗi lần chạy. |  | `TORC-SCHEDULED-TRANSFER-NFR-011-p92271d` |

#### TC-L2-SCHEDULED-TRANSFER-NFR-012-resilience-target-c

Chứng minh chỉ tiêu phục hồi NFR-012: sau khởi động lại, lần chạy quá hạn trong 24 giờ được xử lý trong 5 chu kỳ quét đầu, lần quá hạn > 24 giờ chuyển MISSED.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-NFR-012-resilience-target-c` | `TSET-SCHEDULED-TRANSFER-L2-NFR-012` | llm/approved |

Tiền điều kiện:

- Scheduler vừa khởi động lại
- Có lần chạy quá hạn ≤ 24 giờ và lần quá hạn > 24 giờ
- Chu kỳ quét 30 giây

Fixture: `TFX-SCHEDULED-TRANSFER-NFR-012-c58a4f` Khởi động lại Scheduler với cả lần quá hạn ≤ 24 giờ và > 24 giờ. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Nạp tập lần chạy quá hạn ≤ 24 giờ (tải 100 lần) và một số lần quá hạn > 24 giờ. | {"quaHanTren24h": "vài lần", "quaHanTrong24h": 100} |  |
| 2 | when | Chạy chu kỳ quét đầu tiên sau khởi động lại và tiếp tục đến khi xử lý hết lần ≤ 24 giờ, ghi số chu kỳ đã dùng. | {"chuKyGiay": 30} | NFR-012 |
| 3 | then | Kiểm tra số chu kỳ cần thiết và trạng thái của các lần quá hạn > 24 giờ. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | Số chu kỳ quét cần để xử lý hết lần quá hạn ≤ 24 giờ ≤ 5; lần quá hạn > 24 giờ có trạng thái MISSED. |  | `TORC-SCHEDULED-TRANSFER-NFR-012` |

#### TC-L2-SCHEDULED-TRANSFER-NFR-012-timing-pe0788e-c

Chứng minh biên 24 giờ của chạy bù được phân loại đúng: lần quá hạn ≤ 24 giờ được chạy bù, lần vượt 24 giờ chuyển MISSED và không sinh giao dịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-NFR-012-timing-pe0788e-c` | `TSET-SCHEDULED-TRANSFER-L2-NFR-012` | llm/approved |

Tiền điều kiện:

- Có ba lần chạy quá hạn 23h59m, đúng 24h và 24h01m tại thời điểm khởi động lại
- Scheduler sẵn sàng khởi động lại

Fixture: `TFX-SCHEDULED-TRANSFER-NFR-012-5d57c0` Tạo ba lần chạy quá hạn 23h59m, 24h00m, 24h01m tại thời điểm khởi động lại. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo ba lần chạy quá hạn lần lượt 23h59m, 24h00m và 24h01m so với thời điểm khởi động lại. | {"doTre": ["23h59m", "24h00m", "24h01m"]} |  |
| 2 | when | Khởi động lại Scheduler và chạy chu kỳ quét đầu tiên. | {"chuKy": 1} | NFR-012 |
| 3 | then | Kiểm tra trạng thái và giao dịch của từng lần chạy: hai lần ≤ 24 giờ được chạy bù, lần 24h01m là MISSED và không có giao dịch. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | ≤ 24 giờ: chạy bù; > 24 giờ: MISSED, không giao dịch. |  | `TORC-SCHEDULED-TRANSFER-FR-049-pe0788e` |

#### TC-L2-SCHEDULED-TRANSFER-NFR-013-concurrency-p4dc0d8-c

Chứng minh khi 100 lần chạy đến hạn cùng lúc và nhiều scheduler cùng quét, không có giao dịch trùng cho cùng một lần chạy.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-NFR-013-concurrency-p4dc0d8-c` | `TSET-SCHEDULED-TRANSFER-L2-NFR-013` | llm/approved |

Tiền điều kiện:

- Có 100 lần chạy đến hạn.
- Có thể có hai scheduler cùng quét và chiếm lần chạy đồng thời.
- Tải ở mức benchmark.

Fixture: `TFX-SCHEDULED-TRANSFER-NFR-013-d3bfec` Tạo 100 lần chạy đến hạn và chuẩn bị hai scheduler có thể quét đồng thời. / dọn: Xóa lần chạy, giao dịch và dừng các scheduler do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-NFR-013-concurrency-p4dc0d8-c` (NFR-013, none): value = 0 (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo 100 lần chạy đến hạn và khởi động hai scheduler cùng quét. |  |  |
| 2 | when | Cho các scheduler quét và chiếm lần chạy đồng thời dưới tải benchmark cho đến khi xử lý xong. | {"soLanChay": 100, "soScheduler": 2, "value": 0} | NFR-013 |
| 3 | then | Đếm giao dịch theo từng lần chạy và xác nhận số giao dịch trùng cho cùng một lần chạy bằng 0. |  |  |
| 4 | cleanup | Xóa lần chạy và giao dịch do fixture tạo, dừng scheduler phụ. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | count | == | 0 giao dịch trùng cho một lần chạy. |  | `TORC-SCHEDULED-TRANSFER-NFR-011-p4dc0d8` |

#### TC-L2-SCHEDULED-TRANSFER-NFR-013-performance-pd5ac38-c

Chứng minh rate limit của scheduler được giữ ở mức <= 10 rps khi 100 lần chạy đến hạn cùng lúc, để không đẩy tải quá mức lên Fraud Check giờ cao điểm.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-NFR-013-performance-pd5ac38-c` | `TSET-SCHEDULED-TRANSFER-L2-NFR-013` | llm/approved |

Tiền điều kiện:

- Có 100 lần chạy đến hạn cùng lúc.
- Có công cụ đo tốc độ yêu cầu từ scheduler sang lõi chuyển tiền.

Fixture: `TFX-SCHEDULED-TRANSFER-NFR-013-49f762` Tạo 100 lần chạy đến hạn cùng lúc và bật công cụ đo tốc độ yêu cầu. / dọn: Xóa lần chạy và số liệu đo do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-NFR-013-performance-pd5ac38-c` (NFR-013, none): value = 10 (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo 100 lần chạy đến hạn cùng lúc và bật đo tốc độ yêu cầu của scheduler. |  |  |
| 2 | when | Cho scheduler xử lý liên tiếp các chu kỳ quét và ghi lại số yêu cầu gọi lõi chuyển tiền theo từng giây. | {"soLanChay": 100, "value": 10} | NFR-013 |
| 3 | then | Xác nhận tốc độ yêu cầu từ scheduler ở mọi giây không vượt 10 rps. |  |  |
| 4 | cleanup | Xóa dữ liệu lần chạy và số liệu đo do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | Tốc độ yêu cầu từ scheduler <= 10 rps. | rps | `TORC-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-pd5ac38` |

#### TC-L2-SCHEDULED-TRANSFER-NFR-013-performance-target-c

Chứng minh chỉ tiêu NFR-013: với 100 lần chạy đến hạn đồng thời, p95 chuyển tiền qua API vẫn <= 500 ms trong khi giới hạn 20 lần/chu kỳ và 10 rps được giữ.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-NFR-013-performance-target-c` | `TSET-SCHEDULED-TRANSFER-L2-NFR-013` | llm/approved |

Tiền điều kiện:

- Có 100 lần chạy đến hạn cùng lúc.
- Scheduler và lõi chuyển tiền chạy bình thường.
- Fraud Check phản hồi bình thường.

Fixture: `TFX-SCHEDULED-TRANSFER-NFR-013-de02e4` Tạo 100 lần chạy đến hạn cùng lúc; đảm bảo scheduler, lõi chuyển tiền và Fraud Check hoạt động bình thường. / dọn: Xóa toàn bộ dữ liệu benchmark do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-NFR-013-performance-target-c` (NFR-013, none): value = 500 (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị 100 lần chạy đến hạn cùng lúc; scheduler, lõi chuyển tiền và Fraud Check ở trạng thái bình thường. |  |  |
| 2 | when | Chạy benchmark scripts/benchmark_scheduler.py::main (TC-S13) với 100 lần chạy và thu thập thời gian chuyển tiền qua API. | {"benchmark": "scripts/benchmark_scheduler.py::main", "soLanChay": 100, "value": 500} | NFR-013 |
| 3 | then | Xác nhận p95 thời gian chuyển tiền qua API <= 500 ms và giới hạn 20 lần chạy mỗi chu kỳ cùng rate limit 10 rps vẫn được giữ. |  |  |
| 4 | cleanup | Xóa dữ liệu benchmark do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | p95 thời gian chuyển tiền qua API <= 500 ms (chỉ tiêu 500 ms). | ms | `TORC-SCHEDULED-TRANSFER-NFR-013` |

#### TC-L2-SCHEDULED-TRANSFER-NFR-014-observability-p8ac829-c

Chứng minh khi một lần chạy được thử lại, correlationId vẫn nhất quán theo quy tắc đã định và xuất hiện đủ ở giao dịch, audit và lời gọi Fraud Check.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-NFR-014-observability-p8ac829-c` | `TSET-SCHEDULED-TRANSFER-L2-NFR-014` | llm/approved |

Tiền điều kiện:

- Có một lần chạy đã gặp lỗi tạm thời và được lên lịch thử lại.
- Fraud Check (mock) sẵn sàng nhận lời gọi.
- Fixture TFX-SCHEDULED-TRANSFER-NFR-014-f4aa83 đã được thiết lập.

Fixture: `TFX-SCHEDULED-TRANSFER-NFR-014-f4aa83` Đưa hệ thống về trạng thái có một lần chạy gặp lỗi tạm thời và được lên lịch thử lại. / dọn: Xóa dữ liệu do fixture tạo và reset mock Fraud Check.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lần chạy đã thất bại tạm thời ở lần thử đầu, đã có lịch thử lại; ghi lại correlationId của lần thử đầu. |  |  |
| 2 | when | Cho scheduler thử lại lần chạy tới khi hoàn tất lần thử lại. |  | NFR-014 |
| 3 | then | Kiểm tra mọi giao dịch, bản ghi audit và lời gọi Fraud Check của lần chạy (cả lần thử đầu và lần thử lại) đều có correlationId khác rỗng, nhất quán với quy tắc đã định. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo và reset mock Fraud Check. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | count | == | Mọi bản ghi và lời gọi Fraud Check của lần chạy, kể cả khi thử lại, đều có correlationId khác rỗng. |  | `TORC-SCHEDULED-TRANSFER-NFR-014-p8ac829` |

#### TC-L2-SCHEDULED-TRANSFER-NFR-014-observability-target-c

Chứng minh chỉ tiêu truy vết: 100% lần chạy trong tập kiểm thử (thành công, thử lại, FAILED) có correlationId riêng và correlationId đó đi theo giao dịch, audit và lời gọi Fraud Check.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-NFR-014-observability-target-c` | `TSET-SCHEDULED-TRANSFER-L2-NFR-014` | llm/approved |

Tiền điều kiện:

- Tập lần chạy kiểm thử đa dạng kết quả (thành công, thử lại, FAILED) đã được xử lý xong.
- Fixture TFX-SCHEDULED-TRANSFER-NFR-014-b3529b đã được thiết lập.

Fixture: `TFX-SCHEDULED-TRANSFER-NFR-014-b3529b` Đưa hệ thống về trạng thái tập lần chạy kiểm thử đa dạng kết quả đã được xử lý xong. / dọn: Xóa dữ liệu do fixture tạo.

Dữ liệu `TDATA-L2-SCHEDULED-TRANSFER-NFR-014-observability-target-c` (NFR-014, none): value = 100 (không hợp lệ)

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị tập lần chạy đã xử lý xong gồm các kết quả thành công, thử lại và FAILED. |  |  |
| 2 | when | Đối chiếu correlationId của từng lần chạy ở giao dịch, audit và lời gọi Fraud Check, rồi tính tỷ lệ lần chạy truy vết đầy đủ với chỉ tiêu 100. | {"value": 100} | NFR-014 |
| 3 | then | So sánh tỷ lệ truy vết đầy đủ tính được với giá trị chỉ tiêu 100%. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | Tỷ lệ lần chạy truy vết đầy đủ = 100% (targetValue 100, đơn vị %). |  | `TORC-SCHEDULED-TRANSFER-NFR-014` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-Cancel-200-functional-c

Chứng minh chủ lịch hủy được lịch ACTIVE không có lần RUNNING: trả 200, lịch và các lần PENDING chuyển CANCELLED.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-Cancel-200-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CancelSchedule` | llm/approved |

Tiền điều kiện:

- Lịch ACTIVE thuộc người gọi, có ít nhất một lần PENDING, không có lần RUNNING
- API key hợp lệ của chủ lịch (secretRef:owner_api_key)

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CancelSchedule-4cf3b2` Tạo lịch ACTIVE thuộc người gọi với ít nhất một lần PENDING, không có lần RUNNING. / dọn: Xóa lịch, lần chạy và audit do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE của người gọi với 2 lần chạy PENDING và không có lần RUNNING. |  |  |
| 2 | when | Gửi DELETE /v1/scheduled-transfers/{scheduleId} với API key hợp lệ của chủ lịch. | {"Authorization": "secretRef:owner_api_key", "Idempotency-Key": "idem-200-001"} | EP-CancelSchedule |
| 3 | then | Kiểm tra status 200, lịch = CANCELLED và mọi lần PENDING trước đó = CANCELLED. |  |  |
| 4 | cleanup | Xóa dữ liệu do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | 200; lịch = CANCELLED; mọi lần PENDING trước đó = CANCELLED. |  | `TORC-SCHEDULED-TRANSFER-RSP-Cancel-200` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-Cancel-401-access-c

Chứng minh DELETE với API key sai/không hợp lệ bị từ chối 401 UNAUTHORIZED và không đổi dữ liệu.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-Cancel-401-access-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CancelSchedule` | llm/approved |

Tiền điều kiện:

- Có lịch ACTIVE tồn tại

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CancelSchedule-a59b32` Tạo một lịch ACTIVE. / dọn: Xóa lịch và dữ liệu liên quan do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE và ghi nhận trạng thái ban đầu. |  |  |
| 2 | when | Gửi DELETE với header Authorization chứa API key không hợp lệ. | {"Authorization": "secretRef:invalid_api_key"} | EP-CancelSchedule |
| 3 | then | Kiểm tra status 401, errorCode = UNAUTHORIZED; lịch vẫn ACTIVE, dữ liệu không đổi. |  |  |
| 4 | cleanup | Xóa dữ liệu do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | Status 401, errorCode = UNAUTHORIZED. |  | `TORC-SCHEDULED-TRANSFER-RSP-Cancel-401` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-Cancel-403-access-c

Chứng minh người gọi hợp lệ nhưng không phải chủ lịch bị từ chối 403 FORBIDDEN và lịch không đổi.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-Cancel-403-access-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CancelSchedule` | llm/approved |

Tiền điều kiện:

- Lịch ACTIVE thuộc tài khoản A
- Người gọi là tài khoản B với API key hợp lệ (secretRef:account_b_api_key)

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CancelSchedule-c23ac8` Tạo hai tài khoản A và B, lịch ACTIVE thuộc A. / dọn: Xóa lịch và hai tài khoản do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo tài khoản A với lịch ACTIVE và tài khoản B có API key hợp lệ. |  |  |
| 2 | when | Tài khoản B gửi DELETE với scheduleId của lịch thuộc A. | {"Authorization": "secretRef:account_b_api_key"} | EP-CancelSchedule |
| 3 | then | Kiểm tra status 403, errorCode = FORBIDDEN; lịch vẫn ACTIVE và các lần chạy không đổi. |  |  |
| 4 | cleanup | Xóa dữ liệu do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | Status 403, errorCode = FORBIDDEN; lịch vẫn ACTIVE. |  | `TORC-SCHEDULED-TRANSFER-RSP-Cancel-403` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-Cancel-404-inputValidation-c

Chứng minh DELETE với scheduleId đúng định dạng nhưng không tồn tại trả 404 SCHEDULE_NOT_FOUND và không đổi dữ liệu.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-Cancel-404-inputValidation-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CancelSchedule` | llm/approved |

Tiền điều kiện:

- Không có lịch nào với scheduleId được dùng
- API key hợp lệ (secretRef:owner_api_key)

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CancelSchedule-9fb98f` Đảm bảo không tồn tại lịch với scheduleId được dùng. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chọn scheduleId đúng định dạng nhưng xác nhận không tồn tại trong hệ thống. |  |  |
| 2 | when | Gửi DELETE với scheduleId không tồn tại và API key hợp lệ. | {"Authorization": "secretRef:owner_api_key", "scheduleId": "sch_00000000000000000000"} | EP-CancelSchedule |
| 3 | then | Kiểm tra status 404, errorCode = SCHEDULE_NOT_FOUND; không có thay đổi dữ liệu. |  |  |
| 4 | cleanup | Xóa dữ liệu do test tạo (nếu có). |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | Status 404, errorCode = SCHEDULE_NOT_FOUND. |  | `TORC-SCHEDULED-TRANSFER-RSP-Cancel-404` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-Cancel-409-inputValidation-c

Chứng minh DELETE lịch COMPLETED hoặc lịch có lần RUNNING trả 409 SCHEDULE_NOT_CANCELLABLE và giữ nguyên trạng thái.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-Cancel-409-inputValidation-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CancelSchedule` | llm/approved |

Tiền điều kiện:

- Hai ca: (a) lịch COMPLETED; (b) lịch ACTIVE có một lần RUNNING
- Chủ lịch có API key hợp lệ (secretRef:owner_api_key)

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CancelSchedule-c1f05f` Tạo lịch COMPLETED và lịch ACTIVE có một lần RUNNING thuộc người gọi. / dọn: Xóa các lịch, lần chạy và dữ liệu liên quan do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo ca (a) lịch COMPLETED và ca (b) lịch ACTIVE có một lần RUNNING, cùng thuộc người gọi; ghi nhận trạng thái. |  |  |
| 2 | when | Chủ lịch gửi DELETE cho lịch COMPLETED (ca a). | {"Authorization": "secretRef:owner_api_key", "case": "a-completed"} | EP-CancelSchedule |
| 3 | when | Chủ lịch gửi DELETE cho lịch ACTIVE có lần RUNNING (ca b). | {"Authorization": "secretRef:owner_api_key", "case": "b-running"} | EP-CancelSchedule |
| 4 | then | Kiểm tra cả hai ca trả status 409, errorCode = SCHEDULE_NOT_CANCELLABLE; trạng thái lịch và lần chạy không đổi. |  |  |
| 5 | cleanup | Xóa dữ liệu do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | Status 409, errorCode = SCHEDULE_NOT_CANCELLABLE; trạng thái lịch và lần chạy không đổi. |  | `TORC-SCHEDULED-TRANSFER-RSP-Cancel-409` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-Create-200-functional-c

Chứng minh gửi lại POST với Idempotency-Key đã dùng và nội dung giống hệt trả HTTP 200 kèm lịch gốc, không tạo lịch mới.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-Create-200-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- Đã có lịch tạo thành công với Idempotency-Key K
- Có lưu scheduleId và body của lần tạo đầu

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-7f478a` Tạo lịch thành công với Idempotency-Key K và lưu scheduleId. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch thành công lần đầu với Idempotency-Key K, ghi nhận scheduleId và body gốc. |  |  |
| 2 | when | Gửi lại POST y hệt (cùng body, cùng khóa K) tới EP-CreateSchedule. | {"Idempotency-Key": "K", "body": "giống hệt lần đầu"} | EP-CreateSchedule |
| 3 | then | Kiểm tra HTTP 200, body chứa lịch với scheduleId ban đầu và tổng số lịch không tăng. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 200, body chứa lịch đã tạo với scheduleId ban đầu. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-200` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-Create-201-functional-c

Chứng minh POST hợp lệ với Idempotency-Key mới tạo lịch thành công (HTTP 201, status=ACTIVE) mà không thay đổi số dư hay ledger.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-Create-201-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- Tài khoản nguồn thuộc chủ API key, tài khoản đích tồn tại
- Nguồn còn dưới 20 lịch ACTIVE/PAUSED
- firstRunAt cách hiện tại từ 5 phút đến 90 ngày
- Đã ghi nhận số dư hai tài khoản và số bản ghi ledger_entries

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-27075a` Tạo tài khoản nguồn thuộc người gọi, tài khoản đích, dưới 20 lịch ACTIVE/PAUSED, firstRunAt hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Ghi nhận số dư hai tài khoản và số ledger_entries; chuẩn bị Authorization hợp lệ (secretRef:api_key_owner) và Idempotency-Key mới. |  |  |
| 2 | when | Gửi POST hợp lệ tới EP-CreateSchedule với recurrence=ONCE. | {"Idempotency-Key": "key-new-001", "recurrence": "ONCE"} | EP-CreateSchedule |
| 3 | then | Kiểm tra HTTP 201, scheduleId không rỗng, status=ACTIVE; số dư và số ledger_entries không đổi. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 201, scheduleId không rỗng, status=ACTIVE; số dư và số bản ghi ledger_entries không đổi. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-201` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-Create-400-inputValidation-c

Chứng minh các đầu vào vi phạm hợp đồng đều bị từ chối với HTTP 400 INVALID_REQUEST và không tạo lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-Create-400-inputValidation-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- Authorization hợp lệ
- Ghi nhận số lịch hiện có của tài khoản nguồn

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-35ab9b` Chuẩn bị API key hợp lệ và tài khoản nguồn thuộc người gọi. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị Authorization hợp lệ (secretRef:api_key_owner) và ghi nhận số lịch hiện có. |  |  |
| 2 | when | Lần lượt gửi từng biến thể vi phạm tới EP-CreateSchedule, ví dụ firstRunAt sai định dạng. | {"firstRunAt": "2026-13-40"} | EP-CreateSchedule |
| 3 | then | Kiểm tra mỗi biến thể trả HTTP 400, errorCode INVALID_REQUEST và số lịch không đổi. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 400 với errorCode INVALID_REQUEST cho từng biến thể; số lịch không đổi. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-400` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-Create-401-access-c

Chứng minh gọi POST thiếu Authorization hoặc API key sai bị từ chối với HTTP 401 UNAUTHORIZED và không tạo lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-Create-401-access-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- Request không có API key hợp lệ
- Body hợp lệ
- Ghi nhận số lịch của tài khoản trước khi gọi

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-14ef0e` Chuẩn bị request không có API key hợp lệ và body hợp lệ. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị body hợp lệ; không gửi Authorization hoặc dùng Bearer secretRef:api_key_invalid. |  |  |
| 2 | when | Gửi POST /v1/scheduled-transfers tới EP-CreateSchedule không có Authorization hoặc với API key sai. | {"Authorization": "<thiếu> hoặc secretRef:api_key_invalid"} | EP-CreateSchedule |
| 3 | then | Kiểm tra statusCode=401, errorCode=UNAUTHORIZED và số lịch của tài khoản không đổi. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | statusCode=401, errorCode=UNAUTHORIZED; số lịch của tài khoản không đổi. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-401` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-Create-403-access-c

Chứng minh fromAccountId không thuộc chủ API key bị từ chối với HTTP 403 FORBIDDEN và không tạo lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-Create-403-access-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- API key hợp lệ thuộc tài khoản A
- Tài khoản B tồn tại và không thuộc chủ API key

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-0a6b08` Tạo tài khoản A (chủ API key) và tài khoản B thuộc chủ khác. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị Authorization hợp lệ của chủ tài khoản A (secretRef:api_key_owner_a) và Idempotency-Key mới. |  |  |
| 2 | when | Gửi POST tới EP-CreateSchedule với fromAccountId là tài khoản B, các trường khác hợp lệ. | {"fromAccountId": "tài khoản B (không thuộc chủ API key)"} | EP-CreateSchedule |
| 3 | then | Kiểm tra statusCode=403, errorCode=FORBIDDEN và không có lịch mới cho tài khoản B. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | statusCode=403, errorCode=FORBIDDEN; không có lịch mới cho tài khoản B. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-403` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-Create-404-inputValidation-c

Chứng minh toAccountId không tồn tại bị từ chối với HTTP 404 ACCOUNT_NOT_FOUND và không tạo lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-Create-404-inputValidation-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- API key hợp lệ
- fromAccountId thuộc người gọi
- toAccountId chưa từng được cấp trong hệ thống

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-316b8d` Tạo tài khoản nguồn thuộc người gọi; bảo đảm toAccountId dùng trong test không tồn tại. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị Authorization hợp lệ (secretRef:api_key_owner), fromAccountId hợp lệ và một toAccountId chưa từng được cấp. |  |  |
| 2 | when | Gửi POST tới EP-CreateSchedule với toAccountId không tồn tại, các trường khác hợp lệ. | {"toAccountId": "acc_nonexistent_999"} | EP-CreateSchedule |
| 3 | then | Kiểm tra statusCode=404, errorCode=ACCOUNT_NOT_FOUND và không có lịch mới. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | statusCode=404, errorCode=ACCOUNT_NOT_FOUND; không có lịch mới. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-404` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-Create-409-inputValidation-c

Chứng minh dùng lại Idempotency-Key với body khác bị từ chối với HTTP 409 IDEMPOTENCY_CONFLICT, lịch gốc giữ nguyên.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-Create-409-inputValidation-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- Đã tạo thành công lịch với Idempotency-Key K và body X
- Ghi nhận tổng số lịch và nội dung lịch gốc

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-da05e6` Tạo lịch thành công với Idempotency-Key K và body X. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch thành công với Idempotency-Key K và body X; ghi nhận tổng số lịch và lịch gốc. |  |  |
| 2 | when | Gửi POST tới EP-CreateSchedule với cùng Idempotency-Key K nhưng body Y khác X (ví dụ amount khác). | {"Idempotency-Key": "K", "body": "Y khác X (amount khác)"} | EP-CreateSchedule |
| 3 | then | Kiểm tra statusCode=409, errorCode=IDEMPOTENCY_CONFLICT; lịch gốc không đổi và tổng số lịch không tăng. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | statusCode=409, errorCode=IDEMPOTENCY_CONFLICT; lịch gốc giữ nguyên, tổng số lịch không tăng. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-409` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-Create-422-amount-inputValidation-c

Chứng minh API từ chối tạo lịch chuyển tiền khi amount vượt hạn mức 50.000.000 VND (amount = 50.000.001) bằng HTTP 422 và errorCode LIMIT_EXCEEDED, không tạo lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-Create-422-amount-inputValidation-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- API key hợp lệ đã được cấp cho người gọi.
- Tài khoản nguồn và tài khoản đích hợp lệ, thuộc người gọi.
- Các trường khác (firstRunAt, endDate) nằm trong khoảng hợp lệ.

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-71846f` Tạo API key hợp lệ và các tài khoản hợp lệ; chuẩn bị các trường khác hợp lệ. / dọn: Xóa lịch, tài khoản và API key do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị API key hợp lệ và payload tạo lịch với các trường khác hợp lệ (fromAccountId, toAccountId khác nhau, firstRunAt hợp lệ, endDate hợp lệ). |  |  |
| 2 | when | Gửi POST tạo lịch chuyển tiền với amount vượt hạn mức 1 đơn vị so với biên. | {"amount": 50000001} | EP-CreateSchedule |
| 3 | then | Kiểm tra phản hồi HTTP 422 với errorCode LIMIT_EXCEEDED và xác nhận không có lịch mới được tạo. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | statusCode=422, errorCode=LIMIT_EXCEEDED; không tạo lịch. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-422-amount` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-Create-422-limit-inputValidation-c

Chứng minh API từ chối tạo lịch thứ 21 cho tài khoản nguồn đã có 20 lịch ACTIVE/PAUSED bằng HTTP 422 và errorCode SCHEDULE_LIMIT_REACHED.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-Create-422-limit-inputValidation-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- API key hợp lệ đã được cấp cho người gọi.
- Tài khoản nguồn có đúng 20 lịch ở trạng thái ACTIVE hoặc PAUSED.
- Payload tạo lịch hợp lệ ở mọi trường.

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-143fa6` Tạo tài khoản nguồn và đúng 20 lịch ở trạng thái ACTIVE hoặc PAUSED. / dọn: Xóa 20 lịch và tài khoản do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Khởi tạo tài khoản nguồn với đúng 20 lịch ACTIVE/PAUSED và chuẩn bị payload tạo lịch hợp lệ cho tài khoản đó. |  |  |
| 2 | when | Gửi POST tạo lịch hợp lệ thứ 21 cho tài khoản nguồn này. | {"amount": 1000000, "fromAccountId": "tài khoản nguồn đã có 20 lịch"} | EP-CreateSchedule |
| 3 | then | Kiểm tra phản hồi HTTP 422 với errorCode SCHEDULE_LIMIT_REACHED và số lịch ACTIVE/PAUSED vẫn là 20. |  |  |
| 4 | cleanup | Xóa 20 lịch và dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | statusCode=422, errorCode=SCHEDULE_LIMIT_REACHED; số lịch ACTIVE/PAUSED vẫn bằng 20. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-422-limit` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-Create-422-same-inputValidation-c

Chứng minh API từ chối tạo lịch khi fromAccountId bằng toAccountId bằng HTTP 422 và errorCode SAME_ACCOUNT, không tạo lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-Create-422-same-inputValidation-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- API key hợp lệ đã được cấp cho người gọi.
- Tài khoản dùng trong yêu cầu thuộc người gọi.
- Các trường khác hợp lệ.

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-f6664e` Tạo API key hợp lệ và một tài khoản thuộc người gọi. / dọn: Xóa tài khoản và API key do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị API key hợp lệ và một tài khoản thuộc người gọi để dùng làm cả nguồn và đích. |  |  |
| 2 | when | Gửi POST tạo lịch với fromAccountId trùng toAccountId. | {"fromAccountId": "cùng một tài khoản của người gọi", "toAccountId": "cùng một tài khoản của người gọi"} | EP-CreateSchedule |
| 3 | then | Kiểm tra phản hồi HTTP 422 với errorCode SAME_ACCOUNT và xác nhận không có lịch mới. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | statusCode=422, errorCode=SAME_ACCOUNT; không có lịch mới. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-422-same` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-Create-422-time-inputValidation-c

Chứng minh API từ chối tạo lịch bằng HTTP 422 và errorCode SCHEDULE_TIME_INVALID khi firstRunAt hoặc endDate nằm ngoài khoảng cho phép.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-Create-422-time-inputValidation-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-CreateSchedule` | llm/approved |

Tiền điều kiện:

- API key hợp lệ đã được cấp cho người gọi.
- Các trường khác (fromAccountId, toAccountId, amount) hợp lệ.
- Biết thời điểm hiện tại (now) để tính các giá trị thời gian.

Fixture: `TFX-SCHEDULED-TRANSFER-EP-CreateSchedule-71846f` Tạo API key hợp lệ và các tài khoản hợp lệ; chuẩn bị các trường khác hợp lệ. / dọn: Xóa lịch, tài khoản và API key do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị API key hợp lệ và payload hợp lệ ở các trường ngoài thời gian. |  |  |
| 2 | when | Gửi POST với firstRunAt sớm hơn now+5 phút (ví dụ now+2 phút). | {"firstRunAt": "now+2 phút"} | EP-CreateSchedule |
| 3 | when | Gửi POST với firstRunAt muộn hơn now+90 ngày (ví dụ now+91 ngày). | {"firstRunAt": "now+91 ngày"} | EP-CreateSchedule |
| 4 | when | Gửi POST với endDate trước ngày chạy đầu tiên. | {"endDate": "now+9 ngày", "firstRunAt": "now+10 ngày"} | EP-CreateSchedule |
| 5 | when | Gửi POST với endDate vượt 366 ngày. | {"endDate": "now+400 ngày", "firstRunAt": "now+10 ngày"} | EP-CreateSchedule |
| 6 | then | Với mỗi yêu cầu, kiểm tra phản hồi HTTP 422 với errorCode SCHEDULE_TIME_INVALID và xác nhận không có lịch mới. |  |  |
| 7 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | statusCode=422, errorCode=SCHEDULE_TIME_INVALID; không có lịch mới. |  | `TORC-SCHEDULED-TRANSFER-RSP-Create-422-time` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-Get-200-functional-c

Chứng minh chủ tài khoản nguồn gọi GET với API key hợp lệ và scheduleId tồn tại nhận HTTP 200 kèm status và nextRunAt đúng.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-Get-200-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-GetSchedule` | llm/approved |

Tiền điều kiện:

- Lịch tồn tại với scheduleId đã biết
- Người gọi là chủ tài khoản nguồn của lịch
- API key hợp lệ (secretRef:owner_api_key)

Fixture: `TFX-SCHEDULED-TRANSFER-EP-GetSchedule-1cca41` Tạo lịch tồn tại thuộc tài khoản nguồn của người gọi và cấp API key hợp lệ. / dọn: Xóa lịch và dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch thuộc tài khoản nguồn của người gọi và ghi lại status, nextRunAt đã lưu. |  |  |
| 2 | when | Gửi GET /v1/scheduled-transfers/{scheduleId} với Authorization Bearer hợp lệ. | {"Authorization": "secretRef:owner_api_key", "scheduleId": "sch_0001"} | EP-GetSchedule |
| 3 | then | Kiểm tra HTTP 200 và status, nextRunAt trong body khớp dữ liệu lịch đã lưu. |  |  |
| 4 | cleanup | Xóa lịch do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 200, body chứa status và nextRunAt khớp với dữ liệu lịch đã lưu. |  | `TORC-SCHEDULED-TRANSFER-RSP-Get-200` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-Get-401-access-c

Chứng minh khi dùng API key sai hoặc không hợp lệ trong Authorization, hệ thống trả HTTP 401 UNAUTHORIZED và không lộ dữ liệu lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-Get-401-access-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-GetSchedule` | llm/approved |

Tiền điều kiện:

- Lịch tồn tại với scheduleId đã biết
- Có API key sai/không hợp lệ (secretRef:invalid_api_key)

Fixture: `TFX-SCHEDULED-TRANSFER-EP-GetSchedule-6b10ae` Tạo một lịch tồn tại với scheduleId đã biết. / dọn: Xóa lịch và dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch hợp lệ và chuẩn bị API key không hợp lệ. |  |  |
| 2 | when | Gửi GET /v1/scheduled-transfers/{scheduleId} với Authorization chứa API key sai. | {"Authorization": "secretRef:invalid_api_key", "scheduleId": "sch_0001"} | EP-GetSchedule |
| 3 | then | Kiểm tra HTTP 401, errorCode = UNAUTHORIZED và body không chứa dữ liệu lịch. |  |  |
| 4 | cleanup | Xóa lịch do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 401, errorCode = UNAUTHORIZED, không lộ dữ liệu lịch. |  | `TORC-SCHEDULED-TRANSFER-RSP-Get-401` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-Get-403-access-c

Chứng minh người gọi có API key hợp lệ nhưng không phải chủ tài khoản nguồn của lịch nhận HTTP 403 FORBIDDEN và không lộ dữ liệu lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-Get-403-access-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-GetSchedule` | llm/approved |

Tiền điều kiện:

- Lịch thuộc tài khoản A
- Người gọi xác thực là chủ tài khoản B với API key hợp lệ (secretRef:account_b_api_key)

Fixture: `TFX-SCHEDULED-TRANSFER-EP-GetSchedule-06a2e8` Tạo lịch thuộc tài khoản A và tài khoản B với API key hợp lệ riêng. / dọn: Xóa lịch, tài khoản và dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch thuộc tài khoản A và cấp API key hợp lệ cho chủ tài khoản B. |  |  |
| 2 | when | Gửi GET /v1/scheduled-transfers/{scheduleId} của lịch thuộc tài khoản A bằng API key của B. | {"Authorization": "secretRef:account_b_api_key", "scheduleId": "sch_0001"} | EP-GetSchedule |
| 3 | then | Kiểm tra HTTP 403, errorCode = FORBIDDEN và body không chứa status/nextRunAt. |  |  |
| 4 | cleanup | Xóa lịch và các tài khoản do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 403, errorCode = FORBIDDEN, không chứa status/nextRunAt. |  | `TORC-SCHEDULED-TRANSFER-RSP-Get-403` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-Get-404-inputValidation-c

Chứng minh scheduleId đúng định dạng nhưng không có lịch tương ứng trả HTTP 404 SCHEDULE_NOT_FOUND.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-Get-404-inputValidation-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-GetSchedule` | llm/approved |

Tiền điều kiện:

- API key hợp lệ (secretRef:owner_api_key)
- Không có lịch nào với scheduleId được dùng

Fixture: `TFX-SCHEDULED-TRANSFER-EP-GetSchedule-89b59c` Cấp API key hợp lệ và đảm bảo không tồn tại lịch với scheduleId được dùng. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chọn scheduleId đúng định dạng và xác nhận không tồn tại trong kho dữ liệu. |  |  |
| 2 | when | Gửi GET /v1/scheduled-transfers/{scheduleId} với scheduleId không tồn tại và Authorization hợp lệ. | {"Authorization": "secretRef:owner_api_key", "scheduleId": "sch_does_not_exist_9999"} | EP-GetSchedule |
| 3 | then | Kiểm tra HTTP 404 và errorCode = SCHEDULE_NOT_FOUND. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 404, errorCode = SCHEDULE_NOT_FOUND. |  | `TORC-SCHEDULED-TRANSFER-RSP-Get-404` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-ListRuns-200-functional-c

Chứng minh chủ lịch gọi GET /runs với Authorization hợp lệ nhận HTTP 200, danh sách giảm dần theo scheduledAt, mỗi phần tử đủ trường.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-ListRuns-200-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-ListRuns` | llm/approved |

Tiền điều kiện:

- Lịch tồn tại thuộc về caller
- Lịch có ít nhất 2 lần chạy với scheduledAt khác nhau

Fixture: `TFX-SCHEDULED-TRANSFER-EP-ListRuns-2b6f2c` Tạo lịch thuộc caller và 2 lần chạy có thời điểm khác nhau. / dọn: Xóa lịch, các lần chạy và dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch thuộc caller với 2 lần chạy có scheduledAt khác nhau; chuẩn bị API key của chủ lịch (secretRef:api_key_owner). |  |  |
| 2 | when | Gửi GET /v1/scheduled-transfers/{scheduleId}/runs với Authorization hợp lệ của chủ lịch. |  | EP-ListRuns |
| 3 | then | Kiểm tra HTTP 200, danh sách giảm dần theo scheduledAt, mỗi phần tử có scheduledAt, status, transferId, reasonCode. |  |  |
| 4 | cleanup | Xóa lịch và các lần chạy do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 200; danh sách giảm dần theo scheduledAt; mỗi phần tử có scheduledAt, status, transferId, reasonCode |  | `TORC-SCHEDULED-TRANSFER-RSP-ListRuns-200` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-ListRuns-401-access-c

Chứng minh GET /runs với API key thiếu hoặc sai trả HTTP 401 UNAUTHORIZED và không lộ dữ liệu lần chạy.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-ListRuns-401-access-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-ListRuns` | llm/approved |

Tiền điều kiện:

- Lịch với scheduleId tồn tại

Fixture: `TFX-SCHEDULED-TRANSFER-EP-ListRuns-ebd41e` Tạo lịch với scheduleId tồn tại. / dọn: Xóa lịch và dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch với scheduleId tồn tại; chuẩn bị request thiếu Authorization hoặc dùng API key sai (secretRef:api_key_invalid). |  |  |
| 2 | when | Gửi GET /v1/scheduled-transfers/{scheduleId}/runs với Authorization thiếu hoặc API key sai. |  | EP-ListRuns |
| 3 | then | Kiểm tra phản hồi HTTP 401 với errorCode UNAUTHORIZED và không lộ dữ liệu lần chạy. |  |  |
| 4 | cleanup | Xóa lịch và dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 401, errorCode UNAUTHORIZED |  | `TORC-SCHEDULED-TRANSFER-RSP-ListRuns-401` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-ListRuns-403-access-c

Chứng minh người có API key hợp lệ nhưng không phải chủ tài khoản nguồn của lịch nhận HTTP 403 FORBIDDEN khi gọi GET /runs.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-ListRuns-403-access-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-ListRuns` | llm/approved |

Tiền điều kiện:

- Lịch tồn tại thuộc chủ khác với caller
- Caller có API key hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-ListRuns-198f75` Tạo lịch thuộc chủ A và API key hợp lệ của người dùng B. / dọn: Xóa lịch, thu hồi API key và xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch thuộc chủ A; chuẩn bị API key hợp lệ của người dùng B không phải chủ lịch (secretRef:api_key_other_user). |  |  |
| 2 | when | Gửi GET /v1/scheduled-transfers/{scheduleId}/runs bằng API key của người B. |  | EP-ListRuns |
| 3 | then | Kiểm tra phản hồi HTTP 403 với errorCode FORBIDDEN và không trả lần chạy nào. |  |  |
| 4 | cleanup | Xóa lịch và dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 403, errorCode FORBIDDEN |  | `TORC-SCHEDULED-TRANSFER-RSP-ListRuns-403` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-ListRuns-404-inputValidation-c

Chứng minh GET /runs với scheduleId không tương ứng lịch nào trả HTTP 404 SCHEDULE_NOT_FOUND.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-ListRuns-404-inputValidation-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-ListRuns` | llm/approved |

Tiền điều kiện:

- Không tồn tại lịch với scheduleId được dùng
- Caller có API key hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-ListRuns-8ac015` Đảm bảo scheduleId dùng trong test không tồn tại; cấp API key hợp lệ cho caller. / dọn: Thu hồi API key và xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chọn scheduleId chưa tồn tại trong hệ thống; chuẩn bị API key hợp lệ (secretRef:api_key_owner). |  |  |
| 2 | when | Gửi GET /v1/scheduled-transfers/{scheduleId}/runs với scheduleId không tồn tại. |  | EP-ListRuns |
| 3 | then | Kiểm tra phản hồi HTTP 404 với errorCode SCHEDULE_NOT_FOUND. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 404, errorCode SCHEDULE_NOT_FOUND |  | `TORC-SCHEDULED-TRANSFER-RSP-ListRuns-404` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-ListSchedules-200-functional-c

Chứng minh chủ tài khoản có API key hợp lệ lấy được danh sách lịch của chính mình, lọc đúng status và phân trang đúng theo limit/offset.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-ListSchedules-200-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-ListSchedules` | llm/approved |

Tiền điều kiện:

- Tài khoản có nhiều lịch ở các status khác nhau
- Tài khoản khác cũng có lịch
- API key hợp lệ của chủ tài khoản

Fixture: `TFX-SCHEDULED-TRANSFER-EP-ListSchedules-712403` Tạo tài khoản có nhiều lịch ở các status khác nhau và tài khoản khác cũng có lịch. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo tài khoản có nhiều lịch ở các status khác nhau và một tài khoản khác cũng có lịch. |  |  |
| 2 | when | Gửi GET /v1/accounts/{accountId}/scheduled-transfers?status=ACTIVE&limit=2&offset=1 bằng API key của chủ tài khoản. | {"Authorization": "secretRef:api_key_hop_le", "limit": 2, "offset": 1, "status": "ACTIVE"} | EP-ListSchedules |
| 3 | then | Kiểm tra mã HTTP, accountId và status của từng phần tử, số phần tử và vị trí bắt đầu. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 200; mọi phần tử thuộc accountId và có status=ACTIVE; số phần tử <= 2; bắt đầu từ vị trí offset=1 của tập đã lọc. |  | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-200` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-ListSchedules-400-inputValidation-c

Chứng minh endpoint liệt kê lịch chuyển tiền từ chối giá trị status nằm ngoài enum ACTIVE, PAUSED, COMPLETED, CANCELLED bằng HTTP 400 INVALID_REQUEST và không trả danh sách lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-ListSchedules-400-inputValidation-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-ListSchedules` | llm/approved |

Tiền điều kiện:

- API key hợp lệ của chủ tài khoản đã được cấp (secretRef:api_key_owner_a).
- accountId thuộc về người gọi và tồn tại trong hệ thống.

Fixture: `TFX-SCHEDULED-TRANSFER-EP-ListSchedules-4c6479` Chuẩn bị API key hợp lệ và tài khoản ACC-A-001 thuộc người gọi. / dọn: Xóa dữ liệu và tài khoản do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo fixture: API key hợp lệ và accountId thuộc người gọi; xác nhận tài khoản tồn tại. | {"accountId": "ACC-A-001", "apiKey": "secretRef:api_key_owner_a"} |  |
| 2 | when | Gửi GET danh sách lịch chuyển tiền của tài khoản với tham số status không thuộc enum, kèm header Authorization Bearer. | {"headers": {"Authorization": "Bearer secretRef:api_key_owner_a"}, "method": "GET", "path": "/v1/accounts/ACC-A-001/scheduled-transfers?status=UNKNOWN"} | EP-ListSchedules |
| 3 | then | Kiểm tra phản hồi có HTTP 400, errorCode INVALID_REQUEST và body không chứa danh sách lịch. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 400, errorCode INVALID_REQUEST. |  | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-400` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-ListSchedules-401-access-c

Chứng minh endpoint liệt kê lịch chuyển tiền trả HTTP 401 UNAUTHORIZED khi API key thiếu hoặc sai và không làm lộ dữ liệu lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-ListSchedules-401-access-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-ListSchedules` | llm/approved |

Tiền điều kiện:

- Tài khoản ACC-A-001 tồn tại trong hệ thống.
- Có sẵn một giá trị API key không hợp lệ để thử (secretRef:api_key_invalid).

Fixture: `TFX-SCHEDULED-TRANSFER-EP-ListSchedules-c42dfa` Chuẩn bị tài khoản ACC-A-001 tồn tại. / dọn: Xóa dữ liệu và tài khoản do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo fixture: tài khoản ACC-A-001 tồn tại. | {"accountId": "ACC-A-001"} |  |
| 2 | when | Gửi GET danh sách lịch chuyển tiền không kèm header Authorization. | {"headers": {}, "method": "GET", "path": "/v1/accounts/ACC-A-001/scheduled-transfers"} | EP-ListSchedules |
| 3 | when | Gửi lại GET danh sách lịch chuyển tiền kèm Bearer API key không hợp lệ. | {"headers": {"Authorization": "Bearer secretRef:api_key_invalid"}, "method": "GET", "path": "/v1/accounts/ACC-A-001/scheduled-transfers"} | EP-ListSchedules |
| 4 | then | Kiểm tra cả hai phản hồi có HTTP 401, errorCode UNAUTHORIZED và body không chứa dữ liệu lịch. |  |  |
| 5 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 401, errorCode UNAUTHORIZED. |  | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-401` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-ListSchedules-403-access-c

Chứng minh người gọi có API key hợp lệ của tài khoản A nhưng truy cập lịch của tài khoản B nhận HTTP 403 FORBIDDEN và không lộ lịch của B.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-ListSchedules-403-access-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-ListSchedules` | llm/approved |

Tiền điều kiện:

- Tồn tại hai tài khoản A (ACC-A-001) và B (ACC-B-002).
- Tài khoản B có ít nhất một lịch chuyển tiền.
- Người gọi là chủ tài khoản A và có API key hợp lệ (secretRef:api_key_owner_a).

Fixture: `TFX-SCHEDULED-TRANSFER-EP-ListSchedules-a4f7d2` Chuẩn bị hai tài khoản A và B, B có lịch chuyển tiền, API key thuộc chủ A. / dọn: Xóa lịch, tài khoản và API key do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo fixture: hai tài khoản A và B, tạo ít nhất một lịch chuyển tiền cho B, cấp API key cho chủ A. | {"accountA": "ACC-A-001", "accountB": "ACC-B-002", "apiKey": "secretRef:api_key_owner_a"} |  |
| 2 | when | Gửi GET danh sách lịch của tài khoản B bằng API key của A. | {"headers": {"Authorization": "Bearer secretRef:api_key_owner_a"}, "method": "GET", "path": "/v1/accounts/ACC-B-002/scheduled-transfers"} | EP-ListSchedules |
| 3 | then | Kiểm tra phản hồi có HTTP 403, errorCode FORBIDDEN và body không chứa lịch của B. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 403, errorCode FORBIDDEN, body không chứa lịch của B. |  | `TORC-SCHEDULED-TRANSFER-RSP-ListSchedules-403` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-Patch-200-functional-c

Chứng minh chủ lịch chuyển trạng thái hợp lệ (ACTIVE→PAUSED hoặc PAUSED→ACTIVE) trên lịch định kỳ nhận HTTP 200 và hành vi scheduler đúng.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-Patch-200-functional-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus` | llm/approved |

Tiền điều kiện:

- Lịch định kỳ (không phải ONCE) ở trạng thái nguồn phù hợp
- Người gọi là chủ lịch với API key hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-6fcb9b` Tạo lịch định kỳ không phải ONCE ở trạng thái nguồn phù hợp, người gọi là chủ lịch. / dọn: Xóa lịch và lần chạy do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch định kỳ ACTIVE (và một lịch PAUSED có lần chạy bị bỏ lỡ) thuộc chủ lịch. |  |  |
| 2 | when | Gửi PATCH status=PAUSED trên lịch ACTIVE, sau đó status=ACTIVE trên lịch PAUSED. | {"Authorization": "secretRef:api_key_chu_lich", "scheduleId": "sch_0008", "status": "PAUSED"} | EP-UpdateScheduleStatus |
| 3 | then | Kiểm tra HTTP 200 và status trong body bằng giá trị yêu cầu; sau PAUSED không có lần chạy mới; sau ACTIVE các lần bỏ lỡ là SKIPPED, không chạy bù. |  |  |
| 4 | cleanup | Xóa các lịch và lần chạy do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 200, body lịch có status = giá trị yêu cầu; sau PAUSED không phát sinh lần chạy mới; sau ACTIVE các lần bỏ lỡ là SKIPPED và không chạy bù. |  | `TORC-SCHEDULED-TRANSFER-RSP-Patch-200` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-Patch-400-inputValidation-c

Chứng minh PATCH với status thiếu hoặc ngoài PAUSED/ACTIVE trả lỗi 400.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-Patch-400-inputValidation-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus` | llm/approved |

Tiền điều kiện:

- API key hợp lệ, lịch tồn tại, người gọi là chủ lịch

Fixture: `TFX-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-cfe53b` Tạo API key hợp lệ, lịch tồn tại thuộc chủ lịch. / dọn: Xóa lịch và dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch thuộc chủ lịch và ghi lại trạng thái ban đầu. |  |  |
| 2 | when | Gửi PATCH với status không thuộc enum (lặp lại với body thiếu status). | {"Authorization": "secretRef:api_key_chu_lich", "scheduleId": "sch_0009", "status": "INVALID_STATUS"} | EP-UpdateScheduleStatus |
| 3 | then | Kiểm tra HTTP 400, errorCode INVALID_REQUEST, trạng thái lịch không đổi. |  |  |
| 4 | cleanup | Xóa lịch do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 400, errorCode INVALID_REQUEST. |  | `TORC-SCHEDULED-TRANSFER-RSP-Patch-400` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-Patch-401-access-c

Chứng minh PATCH với API key thiếu hoặc sai bị từ chối bằng 401.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-Patch-401-access-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus` | llm/approved |

Tiền điều kiện:

- Lịch tồn tại

Fixture: `TFX-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-6b10ae` Tạo một lịch tồn tại. / dọn: Xóa lịch do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch tồn tại và ghi lại trạng thái ban đầu. |  |  |
| 2 | when | Gửi PATCH không có Authorization, rồi gửi lại với API key không hợp lệ. | {"Authorization": "secretRef:api_key_sai", "scheduleId": "sch_0010", "status": "PAUSED"} | EP-UpdateScheduleStatus |
| 3 | then | Kiểm tra HTTP 401, errorCode UNAUTHORIZED và lịch không bị thay đổi. |  |  |
| 4 | cleanup | Xóa lịch do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 401, errorCode UNAUTHORIZED. |  | `TORC-SCHEDULED-TRANSFER-RSP-Patch-401` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-Patch-403-access-c

Chứng minh API key hợp lệ nhưng người gọi không phải chủ lịch bị từ chối bằng 403.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-Patch-403-access-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus` | llm/approved |

Tiền điều kiện:

- Lịch thuộc tài khoản A
- Người gọi xác thực là tài khoản B với API key hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-454531` Tạo lịch thuộc tài khoản A và xác thực người gọi là tài khoản B. / dọn: Xóa lịch và tài khoản do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo tài khoản A và B, tạo lịch thuộc A, chuẩn bị API key của B. |  |  |
| 2 | when | Tài khoản B gửi PATCH status hợp lệ lên lịch của A. | {"Authorization": "secretRef:api_key_tai_khoan_b", "scheduleId": "sch_0011", "status": "PAUSED"} | EP-UpdateScheduleStatus |
| 3 | then | Kiểm tra HTTP 403, errorCode FORBIDDEN và trạng thái lịch của A không đổi. |  |  |
| 4 | cleanup | Xóa lịch và hai tài khoản do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 403, errorCode FORBIDDEN, lịch giữ nguyên. |  | `TORC-SCHEDULED-TRANSFER-RSP-Patch-403` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-Patch-404-inputValidation-c

Chứng minh PATCH với scheduleId không tồn tại trả 404.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-Patch-404-inputValidation-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus` | llm/approved |

Tiền điều kiện:

- Không có lịch nào với scheduleId được gửi
- API key hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-4fcf8e` Đảm bảo không có lịch nào với scheduleId được gửi. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chọn scheduleId chưa tồn tại và xác nhận không có lịch tương ứng. |  |  |
| 2 | when | Gửi PATCH status hợp lệ với scheduleId không tồn tại. | {"Authorization": "secretRef:api_key_chu_lich", "scheduleId": "sch_khong_ton_tai", "status": "PAUSED"} | EP-UpdateScheduleStatus |
| 3 | then | Kiểm tra HTTP 404, errorCode SCHEDULE_NOT_FOUND. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 404, errorCode SCHEDULE_NOT_FOUND. |  | `TORC-SCHEDULED-TRANSFER-RSP-Patch-404` |

#### TC-L2-SCHEDULED-TRANSFER-RSP-Patch-409-inputValidation-c

Chứng minh chuyển trạng thái không hợp lệ (PAUSED trên lịch không ACTIVE, ACTIVE trên lịch không PAUSED, hoặc PATCH trên lịch ONCE) trả 409.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-RSP-Patch-409-inputValidation-c` | `TSET-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus` | llm/approved |

Tiền điều kiện:

- Lịch ở trạng thái nguồn sai (đã PAUSED hoặc đã hủy) hoặc là lịch ONCE
- Người gọi là chủ lịch với API key hợp lệ

Fixture: `TFX-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-714e24` Tạo lịch ở trạng thái nguồn sai (đã PAUSED hoặc đã hủy) và một lịch ONCE. / dọn: Xóa các lịch do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch PAUSED, lịch ACTIVE và lịch ONCE thuộc chủ lịch; ghi lại trạng thái ban đầu. |  |  |
| 2 | when | Gửi PATCH status=PAUSED trên lịch PAUSED, status=ACTIVE trên lịch ACTIVE và PATCH trên lịch ONCE. | {"Authorization": "secretRef:api_key_chu_lich", "scheduleId": "sch_0012", "status": "PAUSED"} | EP-UpdateScheduleStatus |
| 3 | then | Kiểm tra mỗi trường hợp trả HTTP 409, errorCode SCHEDULE_STATE_INVALID và trạng thái lịch không đổi. |  |  |
| 4 | cleanup | Xóa các lịch do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | status | == | HTTP 409, errorCode SCHEDULE_STATE_INVALID. |  | `TORC-SCHEDULED-TRANSFER-RSP-Patch-409` |

#### TC-L2-SCHEDULED-TRANSFER-SCN-ScheduledRun-atomicity-p12901f-c

Chứng minh khi tiến trình khởi động lại giữa lúc chiếm lần chạy và lúc chuyển tiền, scheduler quét lại không tạo giao dịch thứ hai cho cùng lần chạy.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-atomicity-p12901f-c` | `TSET-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun` | llm/approved |

Tiền điều kiện:

- Có một lần chạy đã được chiếm (claimed) nhưng chưa có giao dịch nào gắn với nó.
- Idempotency-Key của lần chạy = mã lịch + thời điểm dự kiến.

Fixture: `TFX-SCHEDULED-TRANSFER-SCN-ScheduledRun-7e65d0` Tạo lịch và lần chạy ở trạng thái đã được chiếm nhưng chưa có giao dịch. / dọn: Xóa lịch, lần chạy và giao dịch do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch và một lần chạy ở trạng thái đã chiếm, chưa có giao dịch; xác nhận số giao dịch của lần chạy = 0. |  |  |
| 2 | when | Dừng tiến trình scheduler rồi khởi động lại; để scheduler quét lại lần chạy đó và thực hiện chuyển tiền. | {"hanhDong": "restart-process", "quetLai": true} | SCN-ScheduledRun |
| 3 | then | Đếm giao dịch gắn với lần chạy và kiểm tra Idempotency-Key chặn trùng; số giao dịch tối đa là 1. |  |  |
| 4 | cleanup | Xóa lịch, lần chạy và giao dịch do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | count | == | Tối đa một giao dịch cho lần chạy. |  | `TORC-SCHEDULED-TRANSFER-AC-NFR-011-2-p12901f` |

#### TC-L2-SCHEDULED-TRANSFER-SCN-ScheduledRun-concurrency-p775a37-c

Chứng minh khi hai scheduler cùng chiếm một lần chạy đến hạn, chỉ một UPDATE có điều kiện thành công và không có giao dịch trùng.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-concurrency-p775a37-c` | `TSET-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun` | llm/approved |

Tiền điều kiện:

- Có một lần chạy đến hạn ở trạng thái chưa được chiếm.
- Hai instance scheduler cùng hoạt động trên cùng cơ sở dữ liệu.

Fixture: `TFX-SCHEDULED-TRANSFER-SCN-ScheduledRun-7f2889` Tạo một lần chạy đến hạn ở trạng thái chưa được chiếm. / dọn: Xóa lần chạy và giao dịch do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo một lần chạy đến hạn chưa được chiếm; chuẩn bị hai scheduler với một rào đồng bộ để khởi động cùng lúc. |  |  |
| 2 | when | Cho hai scheduler thực hiện UPDATE có điều kiện chiếm lần chạy đồng thời trên cùng lần chạy. | {"soScheduler": 2, "thaoTac": "conditional-update-claim"} | SCN-ScheduledRun |
| 3 | then | Đếm số UPDATE thành công và số giao dịch của lần chạy. |  |  |
| 4 | cleanup | Xóa dữ liệu lần chạy và giao dịch đã tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | count | == | Số giao dịch trùng cho một lần chạy = 0; đúng một UPDATE có điều kiện thành công. |  | `TORC-SCHEDULED-TRANSFER-AC-NFR-011-1-p775a37` |

#### TC-L2-SCHEDULED-TRANSFER-SCN-ScheduledRun-errorHandling-p00fe49-c

Chứng minh lỗi gửi SMS kết quả không làm đổi trạng thái lần chạy và sự kiện vẫn nằm trong outbox.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-errorHandling-p00fe49-c` | `TSET-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun` | llm/approved |

Tiền điều kiện:

- Lần chạy đã kết thúc (SUCCEEDED hoặc FAILED).
- Sự kiện kết quả đã được ghi vào outbox.

Fixture: `TFX-SCHEDULED-TRANSFER-SCN-ScheduledRun-f3a1ae` Tạo lần chạy đã kết thúc và sự kiện đã ghi vào outbox. / dọn: Xóa lần chạy và sự kiện outbox do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lần chạy đã kết thúc và sự kiện tương ứng trong outbox; ghi lại trạng thái run. |  |  |
| 2 | when | Cho dịch vụ SMS trả lỗi khi worker outbox gửi thông báo kết quả. | {"smsService": "error"} | SCN-ScheduledRun |
| 3 | then | Đọc lại trạng thái lần chạy và kiểm tra sự kiện còn trong outbox. |  |  |
| 4 | cleanup | Khôi phục stub SMS và xóa dữ liệu fixture. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | Trạng thái run giữ nguyên (SUCCEEDED hoặc FAILED) sau lỗi SMS. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-047-3-p00fe49` |

#### TC-L2-SCHEDULED-TRANSFER-SCN-ScheduledRun-observability-p12d52f-c

Chứng minh mỗi lần chạy có correlationId riêng, không trùng giữa các lần chạy, và xuất hiện trong giao dịch, audit, lời gọi Fraud Check và sự kiện.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-observability-p12d52f-c` | `TSET-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun` | llm/approved |

Tiền điều kiện:

- Nhiều lần chạy đến hạn trong cùng chu kỳ quét.
- Có thể truy vấn giao dịch, audit log, log lời gọi Fraud Check và outbox.

Fixture: `TFX-SCHEDULED-TRANSFER-SCN-ScheduledRun-569c46` Tạo nhiều lần chạy đến hạn trong cùng chu kỳ xử lý. / dọn: Xóa lần chạy, giao dịch, audit và sự kiện do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo nhiều lần chạy đến hạn trong cùng chu kỳ quét. |  |  |
| 2 | when | Để scheduler chiếm và xử lý các lần chạy. |  | SCN-ScheduledRun |
| 3 | then | Với mỗi lần chạy, lấy correlationId và tìm trong giao dịch, audit, lời gọi Fraud Check và sự kiện; kiểm tra không có hai lần chạy chung correlationId. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | count | == | Không có hai lần chạy chung correlationId; correlationId xuất hiện ở cả 4 nơi. |  | `TORC-SCHEDULED-TRANSFER-AC-NFR-014-2-p12d52f` |

#### TC-L2-SCHEDULED-TRANSFER-SCN-ScheduledRun-performance-pa101d9-c

Chứng minh khi 100 lần chạy đến hạn cùng lúc, mỗi chu kỳ quét xử lý tối đa 20 lần chạy và p95 chuyển tiền qua API không quá 500 ms.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-performance-pa101d9-c` | `TSET-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun` | llm/approved |

Tiền điều kiện:

- 100 lần chạy đến hạn cùng thời điểm.
- Chu kỳ quét liên tiếp cách nhau 30 giây.
- Công cụ đo độ trễ API chuyển tiền sẵn sàng.

Fixture: `TFX-SCHEDULED-TRANSFER-SCN-ScheduledRun-2cb1a7` Tạo 100 lần chạy đến hạn cùng thời điểm. / dọn: Xóa các lần chạy và giao dịch do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo 100 lần chạy đến hạn cùng thời điểm và bật đo độ trễ API chuyển tiền. |  |  |
| 2 | when | Cho scheduler chạy các chu kỳ quét liên tiếp mỗi 30 giây cho đến khi xử lý hết. | {"chuKyQuetGiay": 30, "soLanChayDenHan": 100} | SCN-ScheduledRun |
| 3 | then | Đếm số lần chạy được xử lý mỗi chu kỳ và tính p95 thời gian chuyển tiền. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | p95 ≤ 500 ms; số lần chạy mỗi chu kỳ ≤ 20. | ms | `TORC-SCHEDULED-TRANSFER-AC-NFR-013-1-pa101d9` |

#### TC-L2-SCHEDULED-TRANSFER-SCN-ScheduledRun-stateTransition-pd98b3f-c

Chứng minh lần chạy quá hạn sau khi hệ thống ngừng được chạy bù nếu quá hạn không quá 24 giờ, ngược lại chuyển MISSED, gồm biên đúng 24 giờ.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-stateTransition-pd98b3f-c` | `TSET-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun` | llm/approved |

Tiền điều kiện:

- Hệ thống vừa khởi động lại.
- Có các lần chạy quá hạn với độ trễ ≤ 24 giờ (gồm đúng 24 giờ) và > 24 giờ.

Fixture: `TFX-SCHEDULED-TRANSFER-SCN-ScheduledRun-cd924c` Khởi động lại hệ thống với các lần chạy quá hạn ở các mức độ trễ khác nhau. / dọn: Xóa lần chạy, giao dịch và sự kiện do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo các lần chạy quá hạn: một lần quá hạn ≤ 24 giờ, một lần đúng 24 giờ, một lần > 24 giờ; khởi động lại hệ thống. |  |  |
| 2 | when | Cho scheduler thực hiện các chu kỳ quét sau khởi động. |  | SCN-ScheduledRun |
| 3 | then | Kiểm tra lần ≤ 24 giờ có 1 giao dịch trong 5 chu kỳ quét đầu; lần > 24 giờ ở MISSED, 0 giao dịch và có sự kiện ScheduledRunFailed reasonCode MISSED. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | ≤ 24 giờ: 1 giao dịch, xử lý trong 5 chu kỳ quét đầu; > 24 giờ: MISSED, 0 giao dịch, ScheduledRunFailed reasonCode MISSED. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-049-2-pd98b3f` |

#### TC-L2-SCHEDULED-TRANSFER-SCN-ScheduledRun-stateTransition-pfec2a2-c

Chứng minh lịch ACTIVE chuyển PAUSED sau lần chạy FAILED thứ 3 liên tiếp và các lỗi không thử lại làm run FAILED ngay.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun-stateTransition-pfec2a2-c` | `TSET-SCHEDULED-TRANSFER-L2-SCN-ScheduledRun` | llm/approved |

Tiền điều kiện:

- Lịch định kỳ ở trạng thái ACTIVE.
- Lịch đã có 2 lần chạy FAILED liên tiếp.

Fixture: `TFX-SCHEDULED-TRANSFER-SCN-ScheduledRun-21a9e6` Tạo lịch định kỳ ACTIVE đã có 2 lần chạy FAILED liên tiếp. / dọn: Xóa lịch, lần chạy và sự kiện do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE với 2 lần chạy FAILED liên tiếp gần nhất. |  |  |
| 2 | when | Kích hoạt lần chạy thứ 3 và để nó kết thúc FAILED (ví dụ lỗi ACCOUNT_INACTIVE, FRAUD_REJECTED hoặc LIMIT_EXCEEDED, không thử lại). | {"reasonCode": "ACCOUNT_INACTIVE"} | SCN-ScheduledRun |
| 3 | then | Kiểm tra run FAILED ngay không retry, sự kiện ScheduledRunFailed kèm reasonCode và lịch chuyển PAUSED. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | Lịch ACTIVE → PAUSED sau 3 run FAILED liên tiếp; run SUCCEEDED xen giữa thì đếm lại từ đầu (chờ xác nhận). |  | `TORC-SCHEDULED-TRANSFER-AC-FR-046-4-pfec2a2` |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-concurrency-p34e996-c

Chứng minh khi PATCH status=PAUSED hoặc DELETE xảy ra đồng thời với việc scheduler chiếm lần chạy đến hạn bằng UPDATE có điều kiện, kết quả vẫn nhất quán với một thứ tự tuần tự hợp lệ.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-concurrency-p34e996-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-Schedule` | llm/approved |

Tiền điều kiện:

- Có lịch ở trạng thái ACTIVE
- Lịch có một lần chạy PENDING đã đến hạn trong chu kỳ quét 30 giây của scheduler
- Có thể phát đồng thời yêu cầu PATCH/DELETE và lần quét của scheduler

Fixture: `TFX-SCHEDULED-TRANSFER-SM-Schedule-ff8ad0` Tạo lịch ACTIVE có lần chạy PENDING đến hạn trong chu kỳ quét 30 giây. / dọn: Xóa lịch, các lần chạy và audit do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE có một lần chạy PENDING đến hạn trong chu kỳ quét 30 giây kế tiếp. | {"runStatus": "PENDING", "scheduleStatus": "ACTIVE"} |  |
| 2 | when | Gửi PATCH status=PAUSED (hoặc DELETE) đồng thời với lần quét của scheduler chiếm lần chạy bằng UPDATE có điều kiện. | {"concurrentWith": "scheduler scan", "request": "PATCH status=PAUSED hoặc DELETE"} | SM-Schedule |
| 3 | then | Đọc trạng thái lịch và các lần chạy sau khi cả hai bên kết thúc, đối chiếu với một thứ tự tuần tự hợp lệ. |  |  |
| 4 | cleanup | Xóa lịch và các lần chạy do kịch bản tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | count | == | Không có lần chạy bị thực hiện 2 lần; không có lần RUNNING của lịch đã CANCELLED; trạng thái cuối nhất quán với một thứ tự tuần tự hợp lệ. |  | `TORC-SCHEDULED-TRANSFER-MOD-Scheduler-p34e996` |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-concurrency-p4b41b3-c

Chứng minh khi hai scheduler cùng chiếm một lần chạy PENDING đến hạn thì chỉ một UPDATE có điều kiện trả rowcount=1 và lõi chuyển tiền chỉ được gọi một lần.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-concurrency-p4b41b3-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-ScheduledRun` | llm/approved |

Tiền điều kiện:

- Có một lần chạy PENDING đã đến hạn (scheduled_at ≤ now)
- Lịch chuyển tiền ở trạng thái ACTIVE
- Lõi chuyển tiền được stub để đếm số lời gọi

Fixture: `TFX-SCHEDULED-TRANSFER-SM-ScheduledRun-b4f367` Tạo lịch ACTIVE và một lần chạy PENDING đến hạn trong cơ sở dữ liệu kiểm thử. / dọn: Xóa lịch và lần chạy do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE và một lần chạy PENDING đến hạn; ghi lại giá trị attempts ban đầu. |  |  |
| 2 | when | Khởi chạy đồng thời hai tiến trình scheduler cùng chiếm lần chạy này bằng UPDATE có điều kiện. | {"runStatus": "PENDING", "scheduleStatus": "ACTIVE", "schedulers": 2} | SM-ScheduledRun |
| 3 | then | Kiểm tra đúng một bên nhận rowcount=1, attempts bằng giá trị cũ + 1 và lõi chuyển tiền được gọi đúng một lần. |  |  |
| 4 | cleanup | Xóa lịch và lần chạy do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | count | == | Đúng 1 bên rowcount=1; attempts = cũ + 1; đúng 1 lời gọi lõi. |  | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-1-p4b41b3` |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-idempotency-p456351-c

Chứng minh sau khi phục hồi lần chạy bị kẹt, lần chiếm lại dùng cùng khóa idempotency nên không tạo giao dịch trùng.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-idempotency-p456351-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-ScheduledRun` | llm/approved |

Tiền điều kiện:

- Lần chạy từng RUNNING bị kẹt sau khi lõi đã tạo giao dịch COMPLETED
- Lần chạy đã được phục hồi về PENDING
- Khóa idempotency của lần chạy được giữ nguyên

Fixture: `TFX-SCHEDULED-TRANSFER-SM-ScheduledRun-a92cb8` Tạo lần chạy đã phục hồi về PENDING cùng giao dịch COMPLETED có cùng khóa idempotency. / dọn: Xóa lần chạy và giao dịch do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị lần chạy đã phục hồi về PENDING và giao dịch COMPLETED đã tồn tại với cùng khóa idempotency; ghi lại transferId ban đầu. |  |  |
| 2 | when | Scheduler chiếm lại lần chạy đã phục hồi và gọi lõi chuyển tiền với cùng khóa idempotency. | {"reuseIdempotencyKey": true, "runStatus": "PENDING"} | SM-ScheduledRun |
| 3 | then | Kiểm tra số giao dịch của lần chạy là 1, transferId trùng giao dịch ban đầu và status=SUCCEEDED. |  |  |
| 4 | cleanup | Xóa lần chạy và giao dịch do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | Số giao dịch của lần chạy = 1; transferId trùng giao dịch ban đầu; status=SUCCEEDED. |  | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-8-p456351` |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-performance-pa68a97-c

Chứng minh mỗi chu kỳ quét chỉ chiếm tối đa 20 lần chạy, phần đến hạn còn lại giữ PENDING cho chu kỳ sau.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-performance-pa68a97-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-ScheduledRun` | llm/approved |

Tiền điều kiện:

- Có hơn 20 lần chạy PENDING đến hạn (ví dụ 25)
- Các lịch tương ứng đều ACTIVE

Fixture: `TFX-SCHEDULED-TRANSFER-SM-ScheduledRun-93117f` Tạo hơn 20 lần chạy PENDING đến hạn thuộc các lịch ACTIVE. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo 25 lần chạy PENDING đến hạn thuộc các lịch ACTIVE. | {"dueRuns": 25} |  |
| 2 | when | Kích hoạt một chu kỳ quét của scheduler. | {"cycles": 1} | SM-ScheduledRun |
| 3 | then | Đếm số lần chạy chuyển sang RUNNING và xác nhận không quá 20; các lần còn lại vẫn PENDING. |  |  |
| 4 | cleanup | Xóa các lần chạy và lịch do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | count | == | Số lần chạy được chiếm trong một chu kỳ ≤ 20. |  | `TORC-SCHEDULED-TRANSFER-MOD-Scheduler-pa68a97` |

#### TC-L2-SCHEDULED-TRANSFER-TRN-Schedule-1-guardViolation-guard-c

Chứng minh PATCH status=PAUSED trên lịch ACTIVE không phải lịch định kỳ bị từ chối và lịch giữ ACTIVE.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-TRN-Schedule-1-guardViolation-guard-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-Schedule` | llm/approved |

Tiền điều kiện:

- Có lịch ACTIVE không phải lịch định kỳ

Fixture: `TFX-SCHEDULED-TRANSFER-TRN-Schedule-1-42774b` Tạo lịch ở ACTIVE, không phải lịch định kỳ. / dọn: Xóa lịch và dữ liệu liên quan do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE không định kỳ (ví dụ chạy một lần). | {"recurring": false, "scheduleStatus": "ACTIVE"} |  |
| 2 | when | Gửi PATCH status=PAUSED lên lịch. | {"status": "PAUSED"} | TRN-Schedule-1 |
| 3 | then | Kiểm tra yêu cầu bị từ chối, lịch vẫn ACTIVE và không có audit SCHEDULE_PAUSED mới. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | Trạng thái vẫn ACTIVE, không có audit SCHEDULE_PAUSED mới. Mã lỗi chưa xác định trong nguồn. |  | `TORC-SCHEDULED-TRANSFER-TRN-Schedule-1-guard` |

#### TC-L2-SCHEDULED-TRANSFER-TRN-Schedule-1-stateTransition-c

Chứng minh PATCH status=PAUSED trên lịch định kỳ ACTIVE chuyển lịch sang PAUSED và ghi audit.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-TRN-Schedule-1-stateTransition-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-Schedule` | llm/approved |

Tiền điều kiện:

- Có lịch định kỳ ở ACTIVE

Fixture: `TFX-SCHEDULED-TRANSFER-TRN-Schedule-1-4e5a07` Tạo lịch định kỳ ở ACTIVE. / dọn: Xóa lịch, lần chạy và audit do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch định kỳ ở ACTIVE có lần chạy kế tiếp. | {"recurring": true, "scheduleStatus": "ACTIVE"} |  |
| 2 | when | Gửi PATCH status=PAUSED lên lịch. | {"status": "PAUSED"} | TRN-Schedule-1 |
| 3 | then | Kiểm tra trạng thái PAUSED, đúng 1 audit SCHEDULE_PAUSED và lần quét 30 giây kế tiếp không chiếm lần chạy nào của lịch. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | ACTIVE → PAUSED; có đúng 1 audit SCHEDULE_PAUSED; lần quét 30 giây kế tiếp không chiếm lần chạy nào của lịch này. |  | `TORC-SCHEDULED-TRANSFER-TRN-Schedule-1` |

#### TC-L2-SCHEDULED-TRANSFER-TRN-Schedule-2-guardViolation-guard-c

Chứng minh PATCH status=ACTIVE trên lịch PAUSED hết lần chạy theo BR-11 không chuyển sang ACTIVE theo nhánh TRN-Schedule-2.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-TRN-Schedule-2-guardViolation-guard-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-Schedule` | llm/approved |

Tiền điều kiện:

- Có lịch PAUSED không còn lần chạy nào theo BR-11

Fixture: `TFX-SCHEDULED-TRANSFER-TRN-Schedule-2-1a3ed1` Tạo lịch PAUSED không còn lần chạy theo BR-11. / dọn: Xóa lịch và dữ liệu liên quan do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch PAUSED mà mọi lần còn lại đều không còn hợp lệ theo BR-11. | {"remainingRunsPerBR11": 0, "scheduleStatus": "PAUSED"} |  |
| 2 | when | Gửi PATCH status=ACTIVE lên lịch. | {"status": "ACTIVE"} | TRN-Schedule-2 |
| 3 | then | Kiểm tra lịch không ở ACTIVE, không có audit SCHEDULE_RESUMED và không sinh lần chạy mới; kết quả cuối xem TRN-Schedule-7. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | Không có trạng thái ACTIVE, không có audit SCHEDULE_RESUMED, không có lần chạy mới. Kết quả cuối xem TRN-Schedule-7. |  | `TORC-SCHEDULED-TRANSFER-TRN-Schedule-2-guard` |

#### TC-L2-SCHEDULED-TRANSFER-TRN-Schedule-2-stateTransition-c

Chứng minh PATCH status=ACTIVE trên lịch PAUSED còn lần chạy theo BR-11 chuyển lịch sang ACTIVE, đánh dấu lần bỏ lỡ và đặt lại bộ đếm lỗi.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-TRN-Schedule-2-stateTransition-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-Schedule` | llm/approved |

Tiền điều kiện:

- Có lịch PAUSED có lần bỏ lỡ và còn lần chạy theo BR-11
- consecutive_failures > 0

Fixture: `TFX-SCHEDULED-TRANSFER-TRN-Schedule-2-b86791` Tạo lịch PAUSED có lần bỏ lỡ, còn lần chạy theo BR-11, consecutive_failures > 0. / dọn: Xóa lịch, lần chạy và audit do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch PAUSED có lần bỏ lỡ trong lúc tạm dừng, còn lần chạy theo BR-11 và consecutive_failures > 0. | {"consecutive_failures": 2, "scheduleStatus": "PAUSED"} |  |
| 2 | when | Gửi PATCH status=ACTIVE lên lịch. | {"status": "ACTIVE"} | TRN-Schedule-2 |
| 3 | then | Kiểm tra trạng thái ACTIVE, lần bỏ lỡ = SKIPPED, có 1 lần chạy kế tiếp mới, consecutive_failures = 0 và audit SCHEDULE_RESUMED. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | PAUSED → ACTIVE; lần bỏ lỡ = SKIPPED; có 1 lần chạy kế tiếp mới; consecutive_failures = 0; audit SCHEDULE_RESUMED. |  | `TORC-SCHEDULED-TRANSFER-TRN-Schedule-2` |

#### TC-L2-SCHEDULED-TRANSFER-TRN-Schedule-3-guardViolation-guard-c

Chứng minh lần chạy FAILED thứ 3 liên tiếp của lịch không định kỳ không làm scheduler tự tạm dừng lịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-TRN-Schedule-3-guardViolation-guard-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-Schedule` | llm/approved |

Tiền điều kiện:

- Có lịch ACTIVE không định kỳ đã có 2 lần FAILED liên tiếp

Fixture: `TFX-SCHEDULED-TRANSFER-TRN-Schedule-3-12667b` Tạo lịch ACTIVE không định kỳ đã có 2 lần FAILED liên tiếp. / dọn: Xóa lịch, lần chạy và audit do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE không định kỳ với 2 lần chạy FAILED liên tiếp. | {"consecutiveFailed": 2, "recurring": false, "scheduleStatus": "ACTIVE"} |  |
| 2 | when | Để scheduler xử lý lần chạy FAILED thứ 3 liên tiếp. | {"run": "FAILED lần 3"} | TRN-Schedule-3 |
| 3 | then | Kiểm tra lịch vẫn ACTIVE và không có audit SCHEDULE_AUTO_PAUSED. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | Trạng thái vẫn ACTIVE, không có audit SCHEDULE_AUTO_PAUSED. |  | `TORC-SCHEDULED-TRANSFER-TRN-Schedule-3-guard` |

#### TC-L2-SCHEDULED-TRANSFER-TRN-Schedule-3-stateTransition-c

Chứng minh lần chạy FAILED thứ 3 liên tiếp của lịch định kỳ ACTIVE khiến scheduler tự chuyển lịch sang PAUSED.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-TRN-Schedule-3-stateTransition-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-Schedule` | llm/approved |

Tiền điều kiện:

- Có lịch định kỳ ACTIVE đã có 2 lần FAILED liên tiếp

Fixture: `TFX-SCHEDULED-TRANSFER-TRN-Schedule-3-621fc1` Tạo lịch định kỳ ACTIVE đã có 2 lần FAILED liên tiếp. / dọn: Xóa lịch, lần chạy và audit do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch định kỳ ACTIVE với 2 lần chạy FAILED liên tiếp. | {"consecutiveFailed": 2, "recurring": true, "scheduleStatus": "ACTIVE"} |  |
| 2 | when | Để scheduler xử lý lần chạy FAILED thứ 3 liên tiếp. | {"run": "FAILED lần 3"} | TRN-Schedule-3 |
| 3 | then | Kiểm tra lịch PAUSED, audit SCHEDULE_AUTO_PAUSED có actor=SCHEDULER và không có lần chạy PENDING mới. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | ACTIVE → PAUSED; audit SCHEDULE_AUTO_PAUSED có actor=SCHEDULER; không có lần chạy PENDING mới được tạo. |  | `TORC-SCHEDULED-TRANSFER-TRN-Schedule-3` |

#### TC-L2-SCHEDULED-TRANSFER-TRN-Schedule-4-guardViolation-guard-c

Chứng minh khi lần chạy cuối kết thúc nhưng lịch chưa đủ điều kiện hoàn tất thì lịch không chuyển COMPLETED.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-TRN-Schedule-4-guardViolation-guard-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-Schedule` | llm/approved |

Tiền điều kiện:

- Lịch ACTIVE, số lần chạy < maxRuns, chưa quá endDate, không phải ONCE đã xong

Fixture: `TFX-SCHEDULED-TRANSFER-TRN-Schedule-4-b09cc7` Tạo lịch ACTIVE, số lần chạy < maxRuns, chưa quá endDate, không phải ONCE đã xong. / dọn: Xóa lịch và lần chạy do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE có số lần chạy nhỏ hơn maxRuns, chưa quá endDate và không phải ONCE đã xong. | {"runCount": "< maxRuns", "scheduleStatus": "ACTIVE"} |  |
| 2 | when | Kết thúc lần chạy cuối hiện có của lịch. | {"event": "lần chạy cuối kết thúc"} | TRN-Schedule-4 |
| 3 | then | Kiểm tra lịch vẫn ACTIVE và không chuyển COMPLETED. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | Trạng thái vẫn ACTIVE, không chuyển COMPLETED. |  | `TORC-SCHEDULED-TRANSFER-TRN-Schedule-4-guard` |

#### TC-L2-SCHEDULED-TRANSFER-TRN-Schedule-4-stateTransition-c

Chứng minh khi lần chạy cuối kết thúc và lịch thỏa một trong các điều kiện ONCE đã xong, đạt maxRuns hoặc đạt endDate thì lịch chuyển COMPLETED.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-TRN-Schedule-4-stateTransition-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-Schedule` | llm/approved |

Tiền điều kiện:

- Lịch ACTIVE thỏa đúng một điều kiện kết thúc cho mỗi lần kiểm

Fixture: `TFX-SCHEDULED-TRANSFER-TRN-Schedule-4-5963b3` Tạo lịch ACTIVE thỏa một trong ba điều kiện kết thúc, kiểm riêng từng điều kiện. / dọn: Xóa lịch và lần chạy do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE thỏa một điều kiện kết thúc (lần lượt ONCE đã xong, đạt maxRuns, đạt endDate). | {"conditions": ["ONCE đã xong", "đạt maxRuns", "đạt endDate"]} |  |
| 2 | when | Kết thúc lần chạy cuối của lịch. | {"event": "lần chạy cuối kết thúc"} | TRN-Schedule-4 |
| 3 | then | Kiểm tra lịch chuyển COMPLETED và không có lần chạy mới nào được tạo. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | ACTIVE → COMPLETED; không có lần chạy mới nào được tạo. |  | `TORC-SCHEDULED-TRANSFER-TRN-Schedule-4` |

#### TC-L2-SCHEDULED-TRANSFER-TRN-Schedule-5-guardViolation-guard-c

Chứng minh DELETE lịch ACTIVE đang có lần chạy RUNNING bị từ chối và lịch giữ nguyên.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-TRN-Schedule-5-guardViolation-guard-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-Schedule` | llm/approved |

Tiền điều kiện:

- Có lịch ACTIVE với ít nhất 1 lần chạy RUNNING và 1 lần PENDING

Fixture: `TFX-SCHEDULED-TRANSFER-TRN-Schedule-5-c9adef` Tạo lịch ACTIVE có ít nhất 1 lần chạy RUNNING. / dọn: Xóa lịch, lần chạy và audit do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE có ít nhất 1 lần chạy RUNNING và 1 lần PENDING. | {"pendingRuns": 1, "runningRuns": 1, "scheduleStatus": "ACTIVE"} |  |
| 2 | when | Gửi DELETE lên lịch. | {"method": "DELETE"} | TRN-Schedule-5 |
| 3 | then | Kiểm tra yêu cầu bị từ chối, lịch vẫn ACTIVE, lần PENDING giữ nguyên và không có audit SCHEDULE_CANCELLED. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | Trạng thái vẫn ACTIVE, lần PENDING giữ nguyên, không có audit SCHEDULE_CANCELLED. Mã lỗi chưa xác định trong nguồn. |  | `TORC-SCHEDULED-TRANSFER-TRN-Schedule-5-guard` |

#### TC-L2-SCHEDULED-TRANSFER-TRN-Schedule-5-stateTransition-c

Chứng minh DELETE lịch ACTIVE không có lần RUNNING chuyển lịch sang CANCELLED và hủy các lần PENDING.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-TRN-Schedule-5-stateTransition-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-Schedule` | llm/approved |

Tiền điều kiện:

- Có lịch ACTIVE không có lần RUNNING, có ít nhất 1 lần PENDING

Fixture: `TFX-SCHEDULED-TRANSFER-TRN-Schedule-5-8ce624` Tạo lịch ACTIVE không có lần RUNNING, có ít nhất 1 lần PENDING. / dọn: Xóa lịch, lần chạy và audit do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE không có lần RUNNING và có ít nhất 1 lần PENDING. | {"pendingRuns": 1, "runningRuns": 0, "scheduleStatus": "ACTIVE"} |  |
| 2 | when | Gửi DELETE lên lịch. | {"method": "DELETE"} | TRN-Schedule-5 |
| 3 | then | Kiểm tra lịch CANCELLED, không còn lần PENDING và có 1 audit SCHEDULE_CANCELLED. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | ACTIVE → CANCELLED; không còn lần PENDING; có 1 audit SCHEDULE_CANCELLED. |  | `TORC-SCHEDULED-TRANSFER-TRN-Schedule-5` |

#### TC-L2-SCHEDULED-TRANSFER-TRN-Schedule-6-guardViolation-guard-c

Chứng minh DELETE lịch PAUSED đang có lần chạy RUNNING bị từ chối và lịch giữ PAUSED.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-TRN-Schedule-6-guardViolation-guard-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-Schedule` | llm/approved |

Tiền điều kiện:

- Có lịch PAUSED với ít nhất 1 lần chạy RUNNING

Fixture: `TFX-SCHEDULED-TRANSFER-TRN-Schedule-6-023691` Tạo lịch PAUSED có ít nhất 1 lần chạy RUNNING. / dọn: Xóa lịch, lần chạy và audit do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch PAUSED có ít nhất 1 lần chạy RUNNING và lần PENDING. | {"runningRuns": 1, "scheduleStatus": "PAUSED"} |  |
| 2 | when | Gửi DELETE lên lịch. | {"method": "DELETE"} | TRN-Schedule-6 |
| 3 | then | Kiểm tra yêu cầu bị từ chối, lịch vẫn PAUSED, lần PENDING không đổi và không có audit SCHEDULE_CANCELLED. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | Trạng thái vẫn PAUSED, không có audit SCHEDULE_CANCELLED. Mã lỗi chưa xác định trong nguồn. |  | `TORC-SCHEDULED-TRANSFER-TRN-Schedule-6-guard` |

#### TC-L2-SCHEDULED-TRANSFER-TRN-Schedule-6-stateTransition-c

Chứng minh DELETE lịch PAUSED không có lần RUNNING chuyển lịch sang CANCELLED và hủy các lần PENDING.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-TRN-Schedule-6-stateTransition-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-Schedule` | llm/approved |

Tiền điều kiện:

- Có lịch PAUSED không có lần RUNNING

Fixture: `TFX-SCHEDULED-TRANSFER-TRN-Schedule-6-54253d` Tạo lịch PAUSED không có lần RUNNING. / dọn: Xóa lịch, lần chạy và audit do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch PAUSED không có lần RUNNING, có lần PENDING. | {"pendingRuns": 1, "runningRuns": 0, "scheduleStatus": "PAUSED"} |  |
| 2 | when | Gửi DELETE lên lịch. | {"method": "DELETE"} | TRN-Schedule-6 |
| 3 | then | Kiểm tra lịch CANCELLED, không còn lần PENDING và có 1 audit SCHEDULE_CANCELLED. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | PAUSED → CANCELLED; không còn lần PENDING; có 1 audit SCHEDULE_CANCELLED. |  | `TORC-SCHEDULED-TRANSFER-TRN-Schedule-6` |

#### TC-L2-SCHEDULED-TRANSFER-TRN-Schedule-7-guardViolation-guard-c

Chứng minh PATCH status=ACTIVE trên lịch PAUSED còn lần chạy theo BR-11 sau khi đánh dấu SKIPPED thì không chuyển COMPLETED.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-TRN-Schedule-7-guardViolation-guard-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-Schedule` | llm/approved |

Tiền điều kiện:

- Có lịch PAUSED còn lần chạy theo BR-11 sau khi đánh dấu SKIPPED

Fixture: `TFX-SCHEDULED-TRANSFER-TRN-Schedule-7-807549` Tạo lịch PAUSED còn lần chạy theo BR-11 sau khi đánh dấu SKIPPED. / dọn: Xóa lịch, lần chạy và audit do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch PAUSED có lần bỏ lỡ và vẫn còn lần chạy hợp lệ theo BR-11 sau khi đánh dấu SKIPPED. | {"remainingRunsPerBR11": ">0", "scheduleStatus": "PAUSED"} |  |
| 2 | when | Gửi PATCH status=ACTIVE lên lịch. | {"status": "ACTIVE"} | TRN-Schedule-7 |
| 3 | then | Kiểm tra lịch không chuyển COMPLETED; trạng thái cuối do TRN-Schedule-2 quyết định (nguồn mẫu 'giữ PAUSED' mâu thuẫn, cần làm rõ). |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | Không chuyển COMPLETED. Trạng thái cuối do TRN-Schedule-2 quyết định; nguồn mẫu 'giữ PAUSED' mâu thuẫn, cần làm rõ. |  | `TORC-SCHEDULED-TRANSFER-TRN-Schedule-7-guard` |

#### TC-L2-SCHEDULED-TRANSFER-TRN-Schedule-7-stateTransition-c

Chứng minh PATCH status=ACTIVE trên lịch PAUSED không còn lần chạy nào theo BR-11 sau khi đánh dấu SKIPPED thì lịch chuyển COMPLETED.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-TRN-Schedule-7-stateTransition-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-Schedule` | llm/approved |

Tiền điều kiện:

- Có lịch PAUSED mà mọi lần còn lại đã quá hạn theo BR-11 (quá endDate hoặc đạt maxRuns)

Fixture: `TFX-SCHEDULED-TRANSFER-TRN-Schedule-7-3072e7` Tạo lịch PAUSED mà mọi lần còn lại đã quá hạn theo BR-11 (quá endDate hoặc đạt maxRuns). / dọn: Xóa lịch, lần chạy và audit do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch PAUSED mà mọi lần còn lại đã quá hạn theo BR-11 (quá endDate hoặc đạt maxRuns). | {"remainingRunsPerBR11": 0, "scheduleStatus": "PAUSED"} |  |
| 2 | when | Gửi PATCH status=ACTIVE lên lịch. | {"status": "ACTIVE"} | TRN-Schedule-7 |
| 3 | then | Kiểm tra lịch chuyển COMPLETED, các lần còn lại = SKIPPED và không có lần chạy mới. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | PAUSED → COMPLETED; các lần còn lại = SKIPPED; không có lần chạy mới. |  | `TORC-SCHEDULED-TRANSFER-TRN-Schedule-7` |

#### TC-L2-SCHEDULED-TRANSFER-TRN-ScheduledRun-1-guardViolation-guard-c

Chứng minh lần chạy PENDING không bị chiếm khi vi phạm guard: chưa đến hạn, lịch không ACTIVE hoặc rowcount=0.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-TRN-ScheduledRun-1-guardViolation-guard-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-ScheduledRun` | llm/approved |

Tiền điều kiện:

- Lần chạy ở trạng thái PENDING
- Một trong ba vi phạm guard được thiết lập: chưa đến hạn, lịch PAUSED, hoặc scheduler khác đã chiếm

Fixture: `TFX-SCHEDULED-TRANSFER-TRN-ScheduledRun-1-2a01eb` Tạo lần chạy PENDING cùng một trong ba vi phạm guard. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lần chạy PENDING và thiết lập một vi phạm guard (chưa đến hạn / lịch PAUSED / rowcount=0); ghi attempts ban đầu. |  |  |
| 2 | when | Scheduler cố chiếm lần chạy trong chu kỳ quét. | {"violation": "NOT_DUE \| SCHEDULE_PAUSED \| ROWCOUNT_0"} | TRN-ScheduledRun-1 |
| 3 | then | Kiểm tra status=PENDING, attempts không đổi và lõi chuyển tiền không bị gọi. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | status=PENDING, attempts không đổi, không có lời gọi lõi chuyển tiền. |  | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-1-guard` |

#### TC-L2-SCHEDULED-TRANSFER-TRN-ScheduledRun-1-stateTransition-c

Chứng minh lần chạy PENDING đã đến hạn, lịch ACTIVE, UPDATE có điều kiện rowcount=1 thì chuyển sang RUNNING.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-TRN-ScheduledRun-1-stateTransition-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-ScheduledRun` | llm/approved |

Tiền điều kiện:

- Lần chạy PENDING có scheduled_at hoặc next_attempt_at ≤ now
- Lịch ở trạng thái ACTIVE

Fixture: `TFX-SCHEDULED-TRANSFER-TRN-ScheduledRun-1-555dbb` Tạo lịch ACTIVE và lần chạy PENDING có thời điểm đến hạn ≤ now. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE và lần chạy PENDING đến hạn; ghi attempts ban đầu. |  |  |
| 2 | when | Scheduler chiếm lần chạy bằng UPDATE có điều kiện. |  | TRN-ScheduledRun-1 |
| 3 | then | Kiểm tra status chuyển PENDING → RUNNING, attempts = cũ + 1, correlationId khác rỗng và lõi chuyển tiền được gọi đúng một lần. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | PENDING → RUNNING; attempts = giá trị cũ + 1; correlationId khác rỗng; 1 lời gọi lõi chuyển tiền. |  | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-1` |

#### TC-L2-SCHEDULED-TRANSFER-TRN-ScheduledRun-2-stateTransition-c

Chứng minh khi lõi trả giao dịch COMPLETED (kể cả phát lại) thì lần chạy RUNNING chuyển sang SUCCEEDED.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-TRN-ScheduledRun-2-stateTransition-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-ScheduledRun` | llm/approved |

Tiền điều kiện:

- Lần chạy ở trạng thái RUNNING
- consecutive_failures có thể lớn hơn 0

Fixture: `TFX-SCHEDULED-TRANSFER-TRN-ScheduledRun-2-1f7505` Tạo lần chạy RUNNING với consecutive_failures có thể > 0. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lần chạy RUNNING với consecutive_failures > 0; stub lõi trả giao dịch COMPLETED. |  |  |
| 2 | when | Lõi trả giao dịch COMPLETED (bao gồm trường hợp phát lại giao dịch đã hoàn tất). | {"coreResult": "COMPLETED"} | TRN-ScheduledRun-2 |
| 3 | then | Kiểm tra RUNNING → SUCCEEDED, transferId đã lưu, consecutive_failures=0, 1 audit, 1 sự kiện ScheduledRunCompleted và lần kế tiếp được sinh hoặc lịch COMPLETED. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | RUNNING → SUCCEEDED; transferId đã lưu; consecutive_failures=0; 1 audit; 1 sự kiện ScheduledRunCompleted; lần kế tiếp được sinh hoặc lịch COMPLETED. |  | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-2` |

#### TC-L2-SCHEDULED-TRANSFER-TRN-ScheduledRun-3-guardViolation-guard-c

Chứng minh khi lỗi tạm thời/thiếu số dư đã hết lượt thử lại thì lần chạy không quay về PENDING.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-TRN-ScheduledRun-3-guardViolation-guard-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-ScheduledRun` | llm/approved |

Tiền điều kiện:

- Lần chạy ở trạng thái RUNNING
- Số lần thử lại đã đạt giới hạn (FRAUD_CHECK_UNAVAILABLE quá 3 lần hoặc INSUFFICIENT_FUNDS quá 2 lần)

Fixture: `TFX-SCHEDULED-TRANSFER-TRN-ScheduledRun-3-95866f` Tạo lần chạy RUNNING có số lần thử lại đã đạt giới hạn. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lần chạy RUNNING với bộ đếm thử lại đã vượt giới hạn; ghi next_attempt_at ban đầu. |  |  |
| 2 | when | Stub lõi trả FRAUD_CHECK_UNAVAILABLE hoặc INSUFFICIENT_FUNDS. | {"errorCode": "FRAUD_CHECK_UNAVAILABLE \| INSUFFICIENT_FUNDS"} | TRN-ScheduledRun-3 |
| 3 | then | Kiểm tra không có chuyển RUNNING → PENDING, next_attempt_at không đổi và lần chạy kết thúc ở FAILED theo TRN-ScheduledRun-4. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | Không có chuyển RUNNING → PENDING; next_attempt_at không đổi; kết thúc ở FAILED theo TRN-ScheduledRun-4. |  | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-3-guard` |

#### TC-L2-SCHEDULED-TRANSFER-TRN-ScheduledRun-3-stateTransition-c

Chứng minh khi lõi trả lỗi tạm thời hoặc thiếu số dư và còn lượt thử lại thì lần chạy RUNNING về PENDING để thử lại.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-TRN-ScheduledRun-3-stateTransition-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-ScheduledRun` | llm/approved |

Tiền điều kiện:

- Lần chạy ở trạng thái RUNNING
- Số lần thử lại của mã lỗi chưa vượt giới hạn

Fixture: `TFX-SCHEDULED-TRANSFER-TRN-ScheduledRun-3-219a47` Tạo lần chạy RUNNING còn lượt thử lại. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lần chạy RUNNING còn lượt thử lại; ghi thời điểm now. |  |  |
| 2 | when | Stub lõi trả FRAUD_CHECK_UNAVAILABLE hoặc INSUFFICIENT_FUNDS. | {"errorCode": "FRAUD_CHECK_UNAVAILABLE \| INSUFFICIENT_FUNDS"} | TRN-ScheduledRun-3 |
| 3 | then | Kiểm tra RUNNING → PENDING và next_attempt_at = now + 5 phút hoặc now + 1 giờ theo loại lỗi. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | RUNNING → PENDING; next_attempt_at = now + 5 phút hoặc now + 1 giờ. |  | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-3` |

#### TC-L2-SCHEDULED-TRANSFER-TRN-ScheduledRun-4-guardViolation-guard-c

Chứng minh khi lỗi không vĩnh viễn và còn lượt thử lại thì lần chạy không sang FAILED.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-TRN-ScheduledRun-4-guardViolation-guard-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-ScheduledRun` | llm/approved |

Tiền điều kiện:

- Lần chạy ở trạng thái RUNNING
- Lỗi là tạm thời và còn lượt thử lại

Fixture: `TFX-SCHEDULED-TRANSFER-TRN-ScheduledRun-4-9f101a` Tạo lần chạy RUNNING với lỗi tạm thời và còn lượt thử lại. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lần chạy RUNNING còn lượt thử lại; ghi consecutive_failures ban đầu. |  |  |
| 2 | when | Stub lõi trả lỗi tạm thời. | {"errorType": "TRANSIENT"} | TRN-ScheduledRun-4 |
| 3 | then | Kiểm tra status không phải FAILED, consecutive_failures không đổi và không có sự kiện ScheduledRunFailed. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | status không phải FAILED; consecutive_failures không đổi; không có sự kiện ScheduledRunFailed. |  | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-4-guard` |

#### TC-L2-SCHEDULED-TRANSFER-TRN-ScheduledRun-4-stateTransition-c

Chứng minh khi lõi trả lỗi vĩnh viễn hoặc đã hết lượt thử lại thì lần chạy RUNNING sang FAILED.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-TRN-ScheduledRun-4-stateTransition-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-ScheduledRun` | llm/approved |

Tiền điều kiện:

- Lần chạy ở trạng thái RUNNING
- Lỗi vĩnh viễn hoặc đã hết lượt thử lại

Fixture: `TFX-SCHEDULED-TRANSFER-TRN-ScheduledRun-4-6c7ae4` Tạo lần chạy RUNNING với lỗi vĩnh viễn hoặc đã hết lượt thử lại. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lần chạy RUNNING với lỗi vĩnh viễn hoặc hết lượt thử lại; ghi consecutive_failures ban đầu. |  |  |
| 2 | when | Stub lõi chuyển tiền trả lỗi. | {"errorType": "PERMANENT \| RETRY_EXHAUSTED"} | TRN-ScheduledRun-4 |
| 3 | then | Kiểm tra RUNNING → FAILED, reasonCode là mã lỗi từ lõi, consecutive_failures = cũ + 1, 1 audit và 1 sự kiện ScheduledRunFailed. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | RUNNING → FAILED; reasonCode = mã lỗi từ lõi; consecutive_failures = cũ + 1; 1 audit; 1 sự kiện ScheduledRunFailed. |  | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-4` |

#### TC-L2-SCHEDULED-TRANSFER-TRN-ScheduledRun-5-guardViolation-guard-c

Chứng minh khi tiếp tục lịch PAUSED, lần chạy PENDING chưa quá hạn không bị chuyển sang SKIPPED.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-TRN-ScheduledRun-5-guardViolation-guard-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-ScheduledRun` | llm/approved |

Tiền điều kiện:

- Lịch đang PAUSED
- Lần chạy PENDING có scheduled_at ở tương lai tại thời điểm tiếp tục lịch

Fixture: `TFX-SCHEDULED-TRANSFER-TRN-ScheduledRun-5-b6df73` Tạo lịch PAUSED và lần chạy PENDING có scheduled_at ở tương lai. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch PAUSED và lần chạy PENDING có scheduled_at ở tương lai. |  |  |
| 2 | when | Tiếp tục (resume) lịch PAUSED. |  | TRN-ScheduledRun-5 |
| 3 | then | Kiểm tra status=PENDING và không có chuyển sang SKIPPED. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | status=PENDING; không có chuyển sang SKIPPED. |  | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-5-guard` |

#### TC-L2-SCHEDULED-TRANSFER-TRN-ScheduledRun-5-stateTransition-c

Chứng minh khi tiếp tục lịch PAUSED, lần chạy PENDING đã quá hạn trong lúc tạm dừng chuyển sang SKIPPED và không chạy bù.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-TRN-ScheduledRun-5-stateTransition-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-ScheduledRun` | llm/approved |

Tiền điều kiện:

- Lịch đang PAUSED
- Lần chạy PENDING có scheduled_at nằm trong khoảng tạm dừng và đã qua

Fixture: `TFX-SCHEDULED-TRANSFER-TRN-ScheduledRun-5-afbb6e` Tạo lịch PAUSED và lần chạy PENDING có scheduled_at trong khoảng tạm dừng đã qua. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch PAUSED và lần chạy PENDING có scheduled_at trong khoảng tạm dừng, đã qua. |  |  |
| 2 | when | Tiếp tục (resume) lịch PAUSED. |  | TRN-ScheduledRun-5 |
| 3 | then | Kiểm tra PENDING → SKIPPED, không có lời gọi lõi chuyển tiền và không có lần chạy bù mới. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | PENDING → SKIPPED; không có lời gọi lõi chuyển tiền và không có lần chạy bù mới. |  | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-5` |

#### TC-L2-SCHEDULED-TRANSFER-TRN-ScheduledRun-6-guardViolation-guard-c

Chứng minh lần chạy PENDING quá hạn không hơn 24 giờ (gồm đúng biên 24 giờ) không bị MISSED ở đầu chu kỳ quét.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-TRN-ScheduledRun-6-guardViolation-guard-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-ScheduledRun` | llm/approved |

Tiền điều kiện:

- Lần chạy PENDING có now − scheduled_at ≤ 24 giờ
- Bao gồm ca biên đúng 24 giờ

Fixture: `TFX-SCHEDULED-TRANSFER-TRN-ScheduledRun-6-b3deb0` Tạo lần chạy PENDING có now − scheduled_at ≤ 24 giờ, gồm cả đúng biên 24 giờ. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lần chạy PENDING với now − scheduled_at = 24 giờ (và một ca nhỏ hơn 24 giờ). | {"overdue": "24h"} |  |
| 2 | when | Chạy đầu chu kỳ quét của scheduler. |  | TRN-ScheduledRun-6 |
| 3 | then | Kiểm tra status=PENDING và không có sự kiện MISSED. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | status=PENDING; không có sự kiện MISSED. |  | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-6-guard` |

#### TC-L2-SCHEDULED-TRANSFER-TRN-ScheduledRun-6-stateTransition-c

Chứng minh lần chạy PENDING quá hạn hơn 24 giờ so với scheduled_at chuyển sang MISSED ở đầu chu kỳ quét.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-TRN-ScheduledRun-6-stateTransition-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-ScheduledRun` | llm/approved |

Tiền điều kiện:

- Lần chạy PENDING có now − scheduled_at > 24 giờ

Fixture: `TFX-SCHEDULED-TRANSFER-TRN-ScheduledRun-6-02d09f` Tạo lần chạy PENDING có now − scheduled_at > 24 giờ. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lần chạy PENDING với now − scheduled_at lớn hơn 24 giờ; ghi consecutive_failures ban đầu. | {"overdue": ">24h"} |  |
| 2 | when | Chạy đầu chu kỳ quét của scheduler. |  | TRN-ScheduledRun-6 |
| 3 | then | Kiểm tra PENDING → MISSED, 1 sự kiện ScheduledRunFailed có reasonCode=MISSED, 1 audit và consecutive_failures không đổi. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | PENDING → MISSED; 1 sự kiện ScheduledRunFailed có reasonCode=MISSED; 1 audit; consecutive_failures không đổi. |  | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-6` |

#### TC-L2-SCHEDULED-TRANSFER-TRN-ScheduledRun-7-stateTransition-c

Chứng minh khi DELETE lịch thì lần chạy PENDING của lịch đó chuyển sang CANCELLED và không còn bị scheduler chiếm.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-TRN-ScheduledRun-7-stateTransition-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-ScheduledRun` | llm/approved |

Tiền điều kiện:

- Có lịch với một lần chạy PENDING thuộc lịch đó

Fixture: `TFX-SCHEDULED-TRANSFER-TRN-ScheduledRun-7-272722` Tạo lịch cùng một lần chạy PENDING thuộc lịch bị xóa. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch và một lần chạy PENDING thuộc lịch đó. |  |  |
| 2 | when | Gửi DELETE lịch. |  | TRN-ScheduledRun-7 |
| 3 | then | Kiểm tra PENDING → CANCELLED và chu kỳ quét sau đó không chiếm lần chạy này. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | PENDING → CANCELLED; chu kỳ quét sau đó không chiếm lần chạy này. |  | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-7` |

#### TC-L2-SCHEDULED-TRANSFER-TRN-ScheduledRun-8-guardViolation-guard-c

Chứng minh lần chạy RUNNING có started_at chưa cũ hơn lease 120 giây (kể cả đúng biên 120 giây) không bị scheduler phục hồi về PENDING.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-TRN-ScheduledRun-8-guardViolation-guard-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-ScheduledRun` | llm/approved |

Tiền điều kiện:

- Có một lần chạy định kỳ ở trạng thái RUNNING với started_at xác định.
- now − started_at ≤ 120 giây (có ca đúng biên 120 giây).
- Ghi nhận giá trị attempts trước khi phục hồi.

Fixture: `TFX-SCHEDULED-TRANSFER-TRN-ScheduledRun-8-64e899` Tạo lần chạy RUNNING có now − started_at ≤ 120 giây (gồm đúng biên 120 giây). / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lần chạy ở trạng thái RUNNING với started_at sao cho now − started_at ≤ 120 giây (ví dụ 119 giây, và một ca đúng 120 giây); ghi lại attempts hiện tại. |  |  |
| 2 | when | Kích hoạt scheduler phục hồi lần chạy kẹt trên lần chạy này. | {"now_minus_started_at_seconds": [119, 120]} | TRN-ScheduledRun-8 |
| 3 | then | Đọc lại lần chạy và kiểm tra status vẫn là RUNNING, attempts bằng giá trị đã ghi trước đó. |  |  |
| 4 | cleanup | Xóa dữ liệu lần chạy do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | status=RUNNING; attempts không đổi. |  | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-8-guard` |

#### TC-L2-SCHEDULED-TRANSFER-TRN-ScheduledRun-8-stateTransition-c

Chứng minh lần chạy RUNNING có started_at cũ hơn lease 120 giây được scheduler phục hồi về PENDING và giữ nguyên attempts.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L2-TRN-ScheduledRun-8-stateTransition-c` | `TSET-SCHEDULED-TRANSFER-L2-SM-ScheduledRun` | llm/approved |

Tiền điều kiện:

- Có một lần chạy định kỳ ở trạng thái RUNNING với started_at xác định.
- now − started_at > 120 giây.
- Ghi nhận giá trị attempts trước khi phục hồi.

Fixture: `TFX-SCHEDULED-TRANSFER-TRN-ScheduledRun-8-99b10d` Tạo lần chạy RUNNING có now − started_at > 120 giây. / dọn: Xóa dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lần chạy ở trạng thái RUNNING với started_at sao cho now − started_at > 120 giây (ví dụ 121 giây); ghi lại attempts hiện tại. |  |  |
| 2 | when | Kích hoạt scheduler phục hồi lần chạy kẹt trên lần chạy này. | {"now_minus_started_at_seconds": 121} | TRN-ScheduledRun-8 |
| 3 | then | Đọc lại lần chạy và kiểm tra status chuyển RUNNING → PENDING, attempts bằng giá trị trước phục hồi. |  |  |
| 4 | cleanup | Xóa dữ liệu lần chạy do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | RUNNING → PENDING; attempts bằng giá trị trước phục hồi. |  | `TORC-SCHEDULED-TRANSFER-TRN-ScheduledRun-8` |

### 2.3 L3: Cô lập service

#### TC-L3-SCHEDULED-TRANSFER-MECH-CatchUp-idempotency-p602a3c-c

Chứng minh lần chạy quá hạn ≤ 24 giờ chỉ được chạy bù đúng một lần, kể cả khi hệ thống khởi động lại nhiều lần hoặc có nhiều chu kỳ quét liên tiếp.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L3-MECH-CatchUp-idempotency-p602a3c-c` | `TSET-SCHEDULED-TRANSFER-L3-MECH-CatchUp` | llm/approved |

Tiền điều kiện:

- Fixture TFX-SCHEDULED-TRANSFER-MECH-CatchUp-0627fe đã được thiết lập.
- Tồn tại ít nhất một lần chạy quá hạn ≤ 24 giờ ở trạng thái chưa được xử lý.
- Scheduler (MOD-Scheduler) sẵn sàng khởi động và chu kỳ quét hoạt động.

Fixture: `TFX-SCHEDULED-TRANSFER-MECH-CatchUp-0627fe` Tạo dữ liệu đưa hệ thống về trạng thái có lần chạy quá hạn ≤ 24 giờ chưa được xử lý. / dọn: Xóa toàn bộ dữ liệu do fixture tạo, gồm lần chạy, giao dịch và bản ghi liên quan.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Xác nhận tồn tại lần chạy quá hạn ≤ 24 giờ chưa được xử lý và chưa có giao dịch nào gắn với lần chạy đó. |  |  |
| 2 | when | Khởi động lại hệ thống rồi để Scheduler thực hiện nhiều chu kỳ quét liên tiếp; lặp lại việc khởi động lại để kiểm tra không chạy trùng. | Số lần khởi động lại ≥ 2; số chu kỳ quét liên tiếp sau mỗi lần khởi động ≥ 2 | MECH-CatchUp |
| 3 | then | Đếm số giao dịch và bản ghi chạy của từng lần chạy bù; xác nhận mỗi lần chỉ có 1 giao dịch và không có bản ghi chạy trùng. |  |  |
| 4 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | Số giao dịch của mỗi lần chạy bù = 1; không có bản ghi chạy trùng. |  | `TORC-SCHEDULED-TRANSFER-AC-FR-049-1-p602a3c` |

#### TC-L3-SCHEDULED-TRANSFER-MECH-CatchUp-resilience-holds-c

Chứng minh sau khi hệ thống ngừng và khởi động lại, Scheduler chạy bù đúng một lần mỗi lần chạy quá hạn ≤ 24 giờ, còn lần quá hạn > 24 giờ chuyển MISSED và không chạy bù.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L3-MECH-CatchUp-resilience-holds-c` | `TSET-SCHEDULED-TRANSFER-L3-MECH-CatchUp` | llm/approved |

Tiền điều kiện:

- Fixture TFX-SCHEDULED-TRANSFER-MECH-CatchUp-0f32c3 đã được thiết lập.
- Hệ thống đang ở trạng thái ngừng.
- Tồn tại các lần chạy đến hạn chưa xử lý: một số quá hạn ≤ 24 giờ và một số quá hạn > 24 giờ so với thời điểm khởi động lại.
- Chu kỳ quét của Scheduler là 30 giây.

Fixture: `TFX-SCHEDULED-TRANSFER-MECH-CatchUp-0f32c3` Đưa hệ thống về trạng thái đã ngừng, tạo các lần chạy chưa xử lý quá hạn ≤ 24 giờ và > 24 giờ so với thời điểm khởi động lại. / dọn: Xóa toàn bộ dữ liệu do fixture tạo, gồm lần chạy, giao dịch và sự kiện liên quan.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Xác nhận hệ thống đã ngừng và ghi nhận danh sách lần chạy chưa xử lý, phân loại theo quá hạn ≤ 24 giờ và > 24 giờ. |  |  |
| 2 | when | Khởi động lại hệ thống để Scheduler bắt đầu các chu kỳ quét, theo dõi ít nhất 5 chu kỳ quét đầu. | Chu kỳ quét 30 giây; theo dõi 5 chu kỳ đầu | MECH-CatchUp |
| 3 | then | Kiểm tra lần quá hạn ≤ 24 giờ được chạy đúng 1 lần và hoàn tất trong 5 chu kỳ quét đầu. |  |  |
| 4 | then | Kiểm tra lần quá hạn > 24 giờ có trạng thái MISSED, có sự kiện ScheduledRunFailed với reasonCode=MISSED và 0 giao dịch. |  |  |
| 5 | cleanup | Xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | Lần quá hạn ≤ 24 giờ: chạy đúng 1 lần, xử lý xong trong 5 chu kỳ quét đầu. Lần quá hạn > 24 giờ: trạng thái MISSED, sự kiện ScheduledRunFailed reasonCode=MISSED, 0 giao dịch. |  | `TORC-SCHEDULED-TRANSFER-MECH-CatchUp-holds` |

#### TC-L3-SCHEDULED-TRANSFER-MECH-RunClaim-concurrency-parallel-c

Chứng minh khi hai scheduler cùng cố chiếm một lần chạy đến hạn bằng UPDATE có điều kiện trên scheduled_runs trong cùng chu kỳ quét 30 giây, đúng một bên chiếm được và chỉ có một giao dịch được tạo.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L3-MECH-RunClaim-concurrency-parallel-c` | `TSET-SCHEDULED-TRANSFER-L3-MECH-RunClaim` | llm/approved |

Tiền điều kiện:

- Có một lịch chuyển tiền với một lần chạy đến hạn trong bảng scheduled_runs, chưa được chiếm.
- Chưa có giao dịch nào sinh ra từ lần chạy này.
- Hai tiến trình scheduler sẵn sàng, cùng kết nối tới cơ sở dữ liệu và cùng nằm trong một chu kỳ quét 30 giây.

Fixture: `TFX-SCHEDULED-TRANSFER-MECH-RunClaim-8e1280` Tạo lịch chuyển tiền cùng một lần chạy đến hạn chưa được chiếm trong scheduled_runs, chưa có giao dịch nào sinh ra từ lần chạy này. / dọn: Xóa lịch, lần chạy và mọi giao dịch do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Nạp fixture: một lần chạy đến hạn của một lịch chuyển tiền, trạng thái chưa chiếm, chưa có giao dịch liên quan. |  |  |
| 2 | given | Khởi động hai tiến trình scheduler A và B, đặt rào đồng bộ để cả hai thực hiện bước chiếm cùng lúc. |  |  |
| 3 | when | Cho cả hai scheduler đồng thời thực hiện UPDATE có điều kiện trên scheduled_runs để chiếm cùng một lần chạy; bên chiếm được gọi lõi chuyển tiền. | 2 tiến trình đồng thời, cùng một mã lần chạy đến hạn, cùng chu kỳ quét 30 giây | MECH-RunClaim |
| 4 | then | Đếm số scheduler chiếm thành công, số giao dịch tạo ra cho lần chạy và số giao dịch trùng; kết quả phải khớp assertion. |  |  |
| 5 | cleanup | Xóa lần chạy, lịch và giao dịch do fixture tạo; dừng hai scheduler. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | count | == | Số giao dịch tạo ra cho lần chạy = 1; số scheduler chiếm thành công = 1; số giao dịch trùng = 0. |  | `TORC-SCHEDULED-TRANSFER-MECH-RunClaim-parallel` |

#### TC-L3-SCHEDULED-TRANSFER-MECH-RunClaim-resilience-crash-mid-c

Chứng minh khi tiến trình scheduler chết sau khi đã commit chiếm lần chạy và trước khi hoàn tất chuyển tiền, rồi khởi động lại, tổng số giao dịch cho lần chạy không vượt quá 1 và không có giao dịch thứ hai.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L3-MECH-RunClaim-resilience-crash-mid-c` | `TSET-SCHEDULED-TRANSFER-L3-MECH-RunClaim` | llm/approved |

Tiền điều kiện:

- Lần chạy đã được chiếm (UPDATE có điều kiện đã commit) nhưng giao dịch chuyển tiền chưa hoàn tất.
- Idempotency-Key cố định = mã lịch + thời điểm dự kiến.
- Cơ chế tiêm lỗi dừng tiến trình (SIGKILL) đã sẵn sàng và có thể khởi động lại scheduler.

Fixture: `TFX-SCHEDULED-TRANSFER-MECH-RunClaim-acda4e` Tạo lần chạy đã được chiếm (commit) với giao dịch chuyển tiền chưa hoàn tất; đặt Idempotency-Key cố định = mã lịch + thời điểm dự kiến. / dọn: Xóa lần chạy, giao dịch và dữ liệu do fixture tạo.

Tiêm lỗi: `TFI-SCHEDULED-TRANSFER-MECH-RunClaim-crash-mid` Dừng cưỡng bức tiến trình scheduler bằng SIGKILL tại điểm tiêm, sau đó khởi động lại tiến trình. — Tại MECH-RunClaim, ngay sau khi giao dịch đã commit và trước khi trả kết quả.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Nạp fixture: lần chạy đã được chiếm (commit), giao dịch chuyển tiền chưa hoàn tất, Idempotency-Key = mã lịch + thời điểm dự kiến. |  |  |
| 2 | given | Bật tiêm lỗi processCrash tại MECH-RunClaim, điểm afterCommit: ngay sau khi giao dịch commit, trước khi trả kết quả. |  |  |
| 3 | when | Chạy scheduler để chiếm lần chạy; tiến trình bị dừng đột ngột bằng SIGKILL tại điểm tiêm, sau đó khởi động lại scheduler. | SIGKILL tại afterCommit; khởi động lại tiến trình; Idempotency-Key = mã lịch + thời điểm dự kiến | MECH-RunClaim |
| 4 | then | Sau khi khởi động lại, đếm số giao dịch của lần chạy; kết quả phải ≤ 1 và không có giao dịch thứ hai. |  |  |
| 5 | cleanup | Tắt tiêm lỗi, dừng scheduler và xóa dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | count | == | Số giao dịch cho lần chạy ≤ 1 sau khởi động lại; không có giao dịch thứ hai. |  | `TORC-SCHEDULED-TRANSFER-MECH-RunClaim-crash` |
| 2 | count | == | Hiệu ứng phụ sau commit đúng một lần: Số giao dịch cho lần chạy ≤ 1 sau khởi động lại; không có giao dịch thứ hai. |  | `TORC-SCHEDULED-TRANSFER-MECH-RunClaim-crash` |

#### TC-L3-SCHEDULED-TRANSFER-MECH-ScheduleRetry-resilience-holds-c

Chứng minh Scheduler áp dụng đúng chính sách thử lại theo từng loại lỗi BR-13 (số lần, khoảng cách, FAILED kèm reasonCode, FAILED ngay) và chuyển lịch sang PAUSED sau 3 lần chạy FAILED liên tiếp.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L3-MECH-ScheduleRetry-resilience-holds-c` | `TSET-SCHEDULED-TRANSFER-L3-MECH-ScheduleRetry` | llm/approved |

Tiền điều kiện:

- Lịch định kỳ ở trạng thái ACTIVE.
- Lần chạy đến hạn đã được Scheduler chiếm bằng UPDATE có điều kiện.
- Chưa có lần chạy FAILED liên tiếp nào của lịch này.
- Lõi chuyển tiền có thể được cấu hình để trả về từng loại lỗi BR-13.

Fixture: `TFX-SCHEDULED-TRANSFER-MECH-ScheduleRetry-68568b` Đưa hệ thống về trạng thái: lịch định kỳ ACTIVE, lần chạy đến hạn đã được Scheduler chiếm bằng UPDATE có điều kiện, chưa có run FAILED liên tiếp nào. / dọn: Xóa lịch, các lần chạy và mọi dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch định kỳ ACTIVE với lần chạy đến hạn, để Scheduler chiếm lần chạy bằng UPDATE có điều kiện; xác nhận số run FAILED liên tiếp bằng 0. |  |  |
| 2 | when | Cấu hình lõi chuyển tiền trả lỗi FRAUD_CHECK_UNAVAILABLE ở mọi lần gọi, cho Scheduler thực hiện lần chạy và các lần thử lại; ghi nhận số lần thử lại, khoảng cách giữa các lần và trạng thái run cuối. | {"errorType": "FRAUD_CHECK_UNAVAILABLE"} | MECH-ScheduleRetry |
| 3 | when | Cấu hình lõi chuyển tiền trả lỗi INSUFFICIENT_FUNDS ở mọi lần gọi, cho Scheduler thực hiện lần chạy và các lần thử lại; ghi nhận số lần thử lại, khoảng cách và trạng thái run cuối. | {"errorType": "INSUFFICIENT_FUNDS"} | MECH-ScheduleRetry |
| 4 | when | Lần lượt cho lõi chuyển tiền trả lỗi ACCOUNT_INACTIVE, FRAUD_REJECTED, LIMIT_EXCEEDED; ghi nhận run chuyển FAILED ngay và số lần thử lại. | {"errorTypes": ["ACCOUNT_INACTIVE", "FRAUD_REJECTED", "LIMIT_EXCEEDED"]} | MECH-ScheduleRetry |
| 5 | then | Kiểm tra: FRAUD_CHECK_UNAVAILABLE tối đa 3 lần thử lại cách 5 phút, hết lượt run FAILED có reasonCode; INSUFFICIENT_FUNDS tối đa 2 lần thử lại cách 1 giờ, hết lượt run FAILED; ba loại còn lại FAILED ngay với 0 lần thử lại; sau 3 run FAILED liên tiếp lịch = PAUSED. |  |  |
| 6 | cleanup | Xóa lịch, các lần chạy và dữ liệu do fixture tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | FRAUD_CHECK_UNAVAILABLE: tối đa 3 lần thử lại, cách 5 phút, hết lượt run FAILED có reasonCode. INSUFFICIENT_FUNDS: tối đa 2 lần thử lại, cách 1 giờ, hết lượt run FAILED. ACCOUNT_INACTIVE/FRAUD_REJECTED/LIMIT_EXCEEDED: FAILED ngay, 0 lần thử lại. Sau 3 run FAILED liên tiếp lịch = PAUSED. |  | `TORC-SCHEDULED-TRANSFER-MECH-ScheduleRetry-holds` |

#### TC-L3-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-performance-at-limit-c

Chứng minh khi có đúng 20 lần chạy đến hạn trong một chu kỳ quét, scheduler xử lý đủ cả 20 lần và tốc độ gọi lõi chuyển tiền không vượt 10 rps.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L3-MECH-SchedulerThrottle-performance-at-limit-c` | `TSET-SCHEDULED-TRANSFER-L3-MECH-SchedulerThrottle` | llm/approved |

Tiền điều kiện:

- Có đúng 20 lần chạy chuyển tiền định kỳ đã đến hạn, trạng thái chưa được chiếm.
- Scheduler được cấu hình quét mỗi 30 giây và đang hoạt động.
- Công cụ đo tốc độ yêu cầu (rps) và độ trễ p95 của API chuyển tiền đã sẵn sàng.

Fixture: `TFX-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-3ad9ae` Khởi tạo đúng 20 lần chạy đến hạn, chưa được chiếm; cấu hình scheduler quét mỗi 30 giây. / dọn: Xóa toàn bộ lần chạy và dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo đúng 20 lần chạy đến hạn, chưa được chiếm; xác nhận scheduler quét mỗi 30 giây và bật công cụ đo rps, p95. | {"dueRuns": 20, "scanIntervalSeconds": 30} |  |
| 2 | when | Chờ đến chu kỳ quét 30 giây của scheduler để scheduler chọn và xử lý các lần chạy đến hạn. | {"trigger": "scheduler scan cycle 30s"} | MECH-SchedulerThrottle |
| 3 | then | Kiểm tra cả 20 lần chạy được xử lý trong chu kỳ, không lần nào bị bỏ sót hay hoãn, tốc độ gọi tối đa không quá 10 rps và p95 thời gian chuyển tiền qua API không quá 500 ms. |  |  |
| 4 | cleanup | Xóa các lần chạy và dữ liệu do fixture tạo, dừng công cụ đo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | Số lần chạy được xử lý trong một chu kỳ <= 20; tốc độ yêu cầu <= 10 rps; p95 thời gian chuyển tiền qua API <= 500 ms. | rps | `TORC-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-limit` |

#### TC-L3-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-performance-over-limit-c

Chứng minh khi số lần chạy đến hạn vượt 20 trong một chu kỳ quét (ví dụ 100 lần), scheduler chỉ xử lý tối đa 20 lần trong chu kỳ đó.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-SCHEDULED-TRANSFER-L3-MECH-SchedulerThrottle-performance-over-limit-c` | `TSET-SCHEDULED-TRANSFER-L3-MECH-SchedulerThrottle` | llm/approved |

Tiền điều kiện:

- Có 100 lần chạy chuyển tiền định kỳ đến hạn cùng lúc, trạng thái chưa được chiếm.
- Scheduler được cấu hình quét mỗi 30 giây và đang hoạt động.
- Công cụ đo tốc độ yêu cầu (rps) và độ trễ p95 của API chuyển tiền đã sẵn sàng.

Fixture: `TFX-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-93d0ec` Khởi tạo 100 lần chạy đến hạn cùng lúc, chưa được chiếm; cấu hình scheduler quét mỗi 30 giây. / dọn: Xóa toàn bộ lần chạy và dữ liệu do fixture tạo.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo 100 lần chạy đến hạn cùng lúc, chưa được chiếm; bật công cụ đo rps và p95. | {"dueRuns": 100, "scanIntervalSeconds": 30} |  |
| 2 | when | Chờ đến chu kỳ quét 30 giây của scheduler để scheduler chọn và xử lý các lần chạy đến hạn. | {"trigger": "scheduler scan cycle 30s"} | MECH-SchedulerThrottle |
| 3 | then | Kiểm tra chu kỳ đó xử lý không quá 20 lần chạy, phần còn lại chưa bị chiếm và chưa xử lý trong chu kỳ này, tốc độ không quá 10 rps và p95 không quá 500 ms. |  |  |
| 4 | cleanup | Xóa các lần chạy và dữ liệu do fixture tạo, dừng công cụ đo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | Số lần chạy được xử lý trong một chu kỳ <= 20; tốc độ yêu cầu <= 10 rps; p95 thời gian chuyển tiền qua API <= 500 ms. | rps | `TORC-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-limit` |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-concurrency-p933b8e-c

Chứng minh khi hai scheduler cùng quét một run PENDING đến hạn thì chỉ một scheduler chiếm được run và chỉ tạo đúng một giao dịch.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-concurrency-p933b8e-c` | `TSET-SCHEDULED-TRANSFER-L3-MOD-Scheduler` | llm/approved |

Tiền điều kiện:

- Có một lịch ACTIVE với một run PENDING đã đến hạn
- Hai tiến trình scheduler cùng kết nối một cơ sở dữ liệu
- Các stub INT-01, MOD-Account, MOD-Audit, MOD-Risk, MOD-Transfer trả thành công

Fixture: `TFX-SCHEDULED-TRANSFER-MOD-Scheduler-782aad` Tạo một run PENDING đến hạn và chuẩn bị hai tiến trình scheduler cùng quét. / dọn: Xóa lịch, run và giao dịch do fixture tạo; dừng hai scheduler.

Stub: `TSTUB-SCHEDULED-TRANSFER-INT-01-success` INT-01 → success; `TSTUB-SCHEDULED-TRANSFER-MOD-Account-success` MOD-Account → success; `TSTUB-SCHEDULED-TRANSFER-MOD-Audit-success` MOD-Audit → success; `TSTUB-SCHEDULED-TRANSFER-MOD-Risk-success` MOD-Risk → success; `TSTUB-SCHEDULED-TRANSFER-MOD-Transfer-success` MOD-Transfer → success

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE và một run PENDING có thời điểm dự kiến đã qua; khởi động hai tiến trình scheduler A và B. | {"runStatus": "PENDING", "scanIntervalSeconds": 30, "scheduleId": "SCH-0001"} |  |
| 2 | when | Cho scheduler A và B cùng chạy scan_and_run trong cùng chu kỳ 30 giây (dùng rào đồng bộ để khởi động đồng thời). | {"cycleSeconds": 30, "schedulers": ["A", "B"]} | MOD-Scheduler |
| 3 | then | Kiểm tra số giao dịch COMPLETED của run và kết quả chiếm run của từng scheduler. |  |  |
| 4 | cleanup | Dừng hai scheduler và xóa lịch, run, giao dịch do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | count | == | Đúng một giao dịch COMPLETED cho run này; tiến trình còn lại không chiếm được run. |  | `TORC-SCHEDULED-TRANSFER-RISK-005-p933b8e` |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-errorHandling-dep-MOD-Account-c

Chứng minh khi MOD-Account lỗi hoặc không phản hồi lúc Scheduler tra cứu tài khoản, hệ thống không tạo giao dịch COMPLETED trùng và run không kẹt ở RUNNING.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-errorHandling-dep-MOD-Account-c` | `TSET-SCHEDULED-TRANSFER-L3-MOD-Scheduler` | llm/approved |

Tiền điều kiện:

- Có lịch ACTIVE
- Lần chạy đến hạn đã được chiếm hoặc đang xử lý yêu cầu tạo lịch
- Các stub khác trả thành công

Fixture: `TFX-SCHEDULED-TRANSFER-MOD-Scheduler-4d4ba6` Tạo lịch ACTIVE có lần chạy đến hạn đã được chiếm hoặc đang xử lý yêu cầu tạo lịch. / dọn: Xóa lịch, run và giao dịch do fixture tạo.

Stub: `TSTUB-SCHEDULED-TRANSFER-INT-01-success` INT-01 → success; `TSTUB-SCHEDULED-TRANSFER-MOD-Account-errorTransient` MOD-Account → errorTransient; `TSTUB-SCHEDULED-TRANSFER-MOD-Audit-success` MOD-Audit → success; `TSTUB-SCHEDULED-TRANSFER-MOD-Risk-success` MOD-Risk → success; `TSTUB-SCHEDULED-TRANSFER-MOD-Transfer-success` MOD-Transfer → success

Tiêm lỗi: `TFI-SCHEDULED-TRANSFER-MOD-Scheduler-dep-MOD-Account` Stub MOD-Account trả lỗi hoặc không phản hồi (timeout). — Kích hoạt khi Scheduler đang gọi MOD-Account trong lúc thao tác chính chạy.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch ACTIVE và run đến hạn đã được chiếm. | {"runStatus": "RUNNING", "scheduleId": "SCH-0002"} |  |
| 2 | given | Cấu hình stub MOD-Account trả lỗi tạm thời hoặc không phản hồi trong lúc Scheduler gọi. | {"behavior": "errorTransient", "stub": "MOD-Account"} |  |
| 3 | when | Cho Scheduler xử lý run, gọi MOD-Account để tra cứu tài khoản nguồn/đích trong lúc đã tiêm lỗi. | {"fault": "dependencyFailure", "phase": "during"} | MOD-Scheduler |
| 4 | then | Kiểm tra số giao dịch COMPLETED của lịch và trạng thái run sau khi xử lý. |  |  |
| 5 | cleanup | Gỡ tiêm lỗi và xóa dữ liệu do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | Chờ làm rõ hành vi; tối thiểu không có giao dịch COMPLETED trùng và run không kẹt ở RUNNING. |  | `TORC-SCHEDULED-TRANSFER-MOD-Account-dep-MOD-Scheduler` |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-errorHandling-dep-MOD-Audit-c

Chứng minh khi MOD-Audit lỗi hoặc không phản hồi lúc Scheduler ghi audit (tác nhân SCHEDULER), trạng thái run và giao dịch vẫn nhất quán với nhau.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-errorHandling-dep-MOD-Audit-c` | `TSET-SCHEDULED-TRANSFER-L3-MOD-Scheduler` | llm/approved |

Tiền điều kiện:

- Lần chạy vừa có kết quả hoặc lịch vừa được tạo, hủy, tạm dừng, tiếp tục
- Các stub khác trả thành công

Fixture: `TFX-SCHEDULED-TRANSFER-MOD-Scheduler-e3f21d` Tạo lần chạy vừa có kết quả hoặc lịch vừa được tạo, hủy, tạm dừng, tiếp tục. / dọn: Xóa lịch, run, giao dịch và bản ghi audit do fixture tạo.

Stub: `TSTUB-SCHEDULED-TRANSFER-INT-01-success` INT-01 → success; `TSTUB-SCHEDULED-TRANSFER-MOD-Account-success` MOD-Account → success; `TSTUB-SCHEDULED-TRANSFER-MOD-Audit-errorTransient` MOD-Audit → errorTransient; `TSTUB-SCHEDULED-TRANSFER-MOD-Risk-success` MOD-Risk → success; `TSTUB-SCHEDULED-TRANSFER-MOD-Transfer-success` MOD-Transfer → success

Tiêm lỗi: `TFI-SCHEDULED-TRANSFER-MOD-Scheduler-dep-MOD-Audit` Stub MOD-Audit trả lỗi hoặc không phản hồi (timeout). — Kích hoạt khi Scheduler gọi MOD-Audit trong lúc thao tác chính chạy.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo tình huống lần chạy vừa có kết quả hoặc lịch vừa thay đổi trạng thái, cần ghi audit. | {"actor": "SCHEDULER", "scheduleId": "SCH-0003"} |  |
| 2 | given | Cấu hình stub MOD-Audit trả lỗi tạm thời hoặc không phản hồi tại bước audit. | {"behavior": "errorTransient", "stub": "MOD-Audit"} |  |
| 3 | when | Cho Scheduler thực hiện thao tác và ghi audit trong lúc MOD-Audit đã bị tiêm lỗi. | {"fault": "dependencyFailure", "phase": "during"} | MOD-Scheduler |
| 4 | then | So sánh trạng thái run với giao dịch tương ứng để xác nhận nhất quán. |  |  |
| 5 | cleanup | Gỡ tiêm lỗi và xóa dữ liệu do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | Chờ làm rõ hành vi; tối thiểu trạng thái run và giao dịch phải nhất quán với nhau. |  | `TORC-SCHEDULED-TRANSFER-MOD-Audit-dep-MOD-Scheduler` |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-errorHandling-dep-MOD-Risk-c

Chứng minh khi Fraud Check lỗi (timeout, 5xx, 4xx, sai schema), Scheduler áp FB-INT01: dưới 5.000.000 VND thì cho qua với riskCheck=SKIPPED, từ 5.000.000 VND thì thử lại 3 lần rồi run FAILED.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-errorHandling-dep-MOD-Risk-c` | `TSET-SCHEDULED-TRANSFER-L3-MOD-Scheduler` | llm/approved |

Tiền điều kiện:

- Lịch ACTIVE có run PENDING đến hạn
- Fraud Check không khả dụng
- Chuẩn bị hai lịch: một số tiền 5.000.000 VND và một số tiền dưới 5.000.000 VND

Fixture: `TFX-SCHEDULED-TRANSFER-MOD-Scheduler-243ccd` Tạo lịch ACTIVE có run PENDING đến hạn và làm Fraud Check không khả dụng. / dọn: Xóa lịch, run, giao dịch do fixture tạo và khôi phục mock Fraud Check.

Stub: `TSTUB-SCHEDULED-TRANSFER-INT-01-success` INT-01 → success; `TSTUB-SCHEDULED-TRANSFER-MOD-Account-success` MOD-Account → success; `TSTUB-SCHEDULED-TRANSFER-MOD-Audit-success` MOD-Audit → success; `TSTUB-SCHEDULED-TRANSFER-MOD-Risk-errorTransient-200` MOD-Risk → errorTransient (200); `TSTUB-SCHEDULED-TRANSFER-MOD-Transfer-success` MOD-Transfer → success

Tiêm lỗi: `TFI-SCHEDULED-TRANSFER-MOD-Scheduler-dep-MOD-Risk` Mock Fraud Check trả timeout, HTTP 5xx, HTTP 4xx hoặc HTTP 200 sai schema. — Kích hoạt khi Scheduler gọi lõi chuyển tiền và lõi gọi MOD-Risk cho run đến hạn.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo hai lịch ACTIVE có run PENDING đến hạn: lịch lớn và lịch nhỏ. | {"largeAmountVND": 5000000, "smallAmountVND": 1000000} |  |
| 2 | given | Cấu hình mock Fraud Check lần lượt trả timeout, HTTP 5xx, HTTP 4xx và HTTP 200 sai schema. | {"faults": ["timeout", "HTTP 5xx", "HTTP 4xx", "HTTP 200 sai schema"], "stub": "MOD-Risk"} |  |
| 3 | when | Cho Scheduler xử lý các run đến hạn, gọi lõi chuyển tiền khi Fraud Check đang lỗi, lặp cho từng loại lỗi. | {"fault": "dependencyFailure", "phase": "during"} | MOD-Scheduler |
| 4 | then | Kiểm tra giao dịch, số lần retry, khoảng cách retry, trạng thái run và reasonCode của cả hai lịch. |  |  |
| 5 | cleanup | Gỡ mock và xóa dữ liệu do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | Số tiền >= 5.000.000 VND: không có giao dịch, retry 3 lần cách 5 phút, sau đó run FAILED với reasonCode FRAUD_CHECK_UNAVAILABLE. Số tiền < 5.000.000 VND: giao dịch COMPLETED với riskCheck=SKIPPED, run SUCCEEDED. |  | `TORC-SCHEDULED-TRANSFER-MOD-Risk-dep-MOD-Scheduler` |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-errorHandling-dep-MOD-Transfer-c

Chứng minh khi lõi chuyển tiền trả lỗi nghiệp vụ, Scheduler thử lại và đặt trạng thái run đúng AC-FR-046-2/3, lịch chuyển PAUSED sau 3 run FAILED liên tiếp và không có giao dịch trùng.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-errorHandling-dep-MOD-Transfer-c` | `TSET-SCHEDULED-TRANSFER-L3-MOD-Scheduler` | llm/approved |

Tiền điều kiện:

- Lịch ACTIVE có run PENDING đến hạn
- Scheduler đã chiếm run bằng UPDATE có điều kiện
- Các stub khác trả thành công

Fixture: `TFX-SCHEDULED-TRANSFER-MOD-Scheduler-c6c441` Tạo lịch ACTIVE có run PENDING đến hạn đã được Scheduler chiếm bằng UPDATE có điều kiện. / dọn: Xóa lịch, run và giao dịch do fixture tạo.

Stub: `TSTUB-SCHEDULED-TRANSFER-INT-01-success` INT-01 → success; `TSTUB-SCHEDULED-TRANSFER-MOD-Account-success` MOD-Account → success; `TSTUB-SCHEDULED-TRANSFER-MOD-Audit-success` MOD-Audit → success; `TSTUB-SCHEDULED-TRANSFER-MOD-Risk-success` MOD-Risk → success; `TSTUB-SCHEDULED-TRANSFER-MOD-Transfer-errorTransient` MOD-Transfer → errorTransient

Tiêm lỗi: `TFI-SCHEDULED-TRANSFER-MOD-Scheduler-dep-MOD-Transfer` Stub MOD-Transfer trả lỗi nghiệp vụ hoặc không phản hồi. — Kích hoạt khi Scheduler gọi MOD-Transfer trong lúc thao tác chính chạy.

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch định kỳ ACTIVE và run PENDING đã được Scheduler chiếm. | {"runStatus": "RUNNING", "scheduleId": "SCH-0005"} |  |
| 2 | given | Cấu hình stub MOD-Transfer trả lỗi nghiệp vụ (INSUFFICIENT_FUNDS, ACCOUNT_INACTIVE, FRAUD_REJECTED, LIMIT_EXCEEDED) hoặc không phản hồi. | {"behavior": "errorTransient", "stub": "MOD-Transfer"} |  |
| 3 | when | Cho Scheduler gọi lõi chuyển tiền cho run đến hạn trong lúc đã tiêm lỗi; lặp qua nhiều chu kỳ để đủ 3 run FAILED liên tiếp. | {"fault": "dependencyFailure", "phase": "during"} | MOD-Scheduler |
| 4 | then | Kiểm tra số lần thử lại, khoảng cách, trạng thái run, trạng thái lịch và số giao dịch. |  |  |
| 5 | cleanup | Gỡ tiêm lỗi và xóa dữ liệu do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | Với lỗi nghiệp vụ: số lần thử lại, khoảng cách và trạng thái run đúng như AC-FR-046-2/3; sau 3 run FAILED liên tiếp lịch là PAUSED; không có giao dịch trùng. |  | `TORC-SCHEDULED-TRANSFER-MOD-Transfer-dep-MOD-Scheduler` |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-timing-p077a10-c

Chứng minh khi có hơn 20 run đến hạn, mỗi chu kỳ quét chỉ chiếm tối đa 20 run và ít nhất 99% run bắt đầu trong 60 giây kể từ thời điểm dự kiến.

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-timing-p077a10-c` | `TSET-SCHEDULED-TRANSFER-L3-MOD-Scheduler` | llm/approved |

Tiền điều kiện:

- Có hơn 20 run PENDING đến hạn cùng lúc
- Các stub phụ thuộc trả thành công
- Đồng hồ test có thể ghi nhận thời điểm dự kiến và thời điểm bắt đầu của từng run

Fixture: `TFX-SCHEDULED-TRANSFER-MOD-Scheduler-b59c5a` Tạo hơn 20 run PENDING đến hạn cùng lúc. / dọn: Xóa lịch, run và giao dịch do fixture tạo.

Stub: `TSTUB-SCHEDULED-TRANSFER-INT-01-success` INT-01 → success; `TSTUB-SCHEDULED-TRANSFER-MOD-Account-success` MOD-Account → success; `TSTUB-SCHEDULED-TRANSFER-MOD-Audit-success` MOD-Audit → success; `TSTUB-SCHEDULED-TRANSFER-MOD-Risk-success` MOD-Risk → success; `TSTUB-SCHEDULED-TRANSFER-MOD-Transfer-success` MOD-Transfer → success

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo 45 run PENDING cùng đến hạn và ghi lại thời điểm dự kiến của từng run. | {"pendingRuns": 45} |  |
| 2 | when | Chạy một chu kỳ quét 30 giây của Scheduler rồi tiếp tục các chu kỳ sau cho đến khi xử lý hết. | {"cycleSeconds": 30} | MOD-Scheduler |
| 3 | then | Đếm số run được chiếm trong từng chu kỳ và tính tỷ lệ run bắt đầu trong 60 giây kể từ thời điểm dự kiến. |  |  |
| 4 | cleanup | Xóa các lịch, run và giao dịch do test tạo. |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | Tối đa 20 run được chiếm mỗi chu kỳ; ít nhất 99% run bắt đầu trong 60 giây kể từ thời điểm dự kiến. | giây | `TORC-SCHEDULED-TRANSFER-AC-NFR-010-1-p077a10` |

## 3. Lịch sử phiên bản và phạm vi hồi quy

| Phiên bản | Trạng thái | CR | Thêm | Sửa | Bỏ | Số nút |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | Baselined | CR-001 | 3380 | 0 | 0 | 3380 |

Hồi quy (0 test case đã baseline, thuộc phần bị ảnh hưởng nhưng không đổi): không có

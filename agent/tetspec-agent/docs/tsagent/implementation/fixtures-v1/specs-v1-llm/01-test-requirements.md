---
spec: TSD-SCHEDULED-TRANSFER-TestRequirements
feature: SCHEDULED-TRANSFER
docKind: TestRequirements
version: 1
status: Draft
supersedes: none
baseline: b-ec21ecfae083
changes: [CR-001]
contentHash: 638386bb4076c7c87f8d13ca4d3ce69761c40e3e9e4b76a5d0f1e9aa327aab14
---
# Test Requirements: SCHEDULED-TRANSFER

## 0. Thông tin tài liệu

| Trường | Giá trị |
| --- | --- |
| id | TSD-SCHEDULED-TRANSFER-TestRequirements_v1 |
| Feature | SCHEDULED-TRANSFER |
| Phiên bản | 1 |
| Trạng thái | Draft |
| Baseline TDD | DOC-SRS-CR001 (b-ec21ecfae083) |
| CR của phiên bản này | CR-001 |

## 1. Phạm vi và mục tiêu

- Phạm vi: feature SCHEDULED-TRANSFER; ba mức test: L1 Nghiệp vụ E2E; L2 Nghiệp vụ; L3 Cô lập service
- Mục tiêu: mọi yêu cầu có Test Requirement; mọi Test Requirement có thiết kế và test case truy vết được

## 2. Test basis

| Tài liệu | Loại | Phiên bản | Trạng thái |
| --- | --- | --- | --- |
| DOC-HLD | SAD/HLD | 0.1 | Baselined |
| DOC-SRS | SRS | 0.1 | Baselined |
| DOC-SRS-CR001 | SRS | 0.1 | Baselined |
| DOC-TDD | TDD | 0.1 | InReview |
| DOC-TEST | Test Plan | 0.1 | InReview |
| SOL-CR-001 | solution | 1 | Baselined |
| SUP-CR-001 | solution | 1 | Baselined |

## 3. Test Requirements theo mức

### 3.1 L1: Nghiệp vụ E2E

#### TR-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun

| Trường | Giá trị |
| --- | --- |
| id | TR-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun |
| Mức, loại test | L1 · functional |
| Mô tả điều cần kiểm | Kiểm hành trình SCN-ScheduledRun: |
| Nguồn truy vết | FR-045, FR-046, FR-047, FR-048, FR-049, INT-01, MOD-API, MOD-Scheduler, MOD-Transfer, SCN-ScheduledRun |
| Tiêu chí chấp nhận | luồng chính hoàn tất đúng thứ tự và trong ngân sách thời gian |
| Ngưỡng NFR | — |
| Phương pháp xác minh | Test |
| Ưu tiên và rủi ro | P0 · rủi ro: RISK-001, RISK-005 |
| Loại thay đổi | new |
| Phụ thuộc chưa xác nhận | ASM-001, OQ-001 (pendingConfirmation) |

### 3.2 L2: Nghiệp vụ

#### TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule

| Trường | Giá trị |
| --- | --- |
| id | TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule |
| Mức, loại test | L2 · functional |
| Mô tả điều cần kiểm | Kiểm API DELETE /v1/scheduled-transfers/{scheduleId} theo solution |
| Nguồn truy vết | EP-CancelSchedule, FR-043, FR-048, MOD-Audit, MOD-Scheduler |
| Tiêu chí chấp nhận | mọi quy tắc đầu vào của trường và mọi mã phản hồi của endpoint được kiểm |
| Ngưỡng NFR | — |
| Phương pháp xác minh | Test |
| Ưu tiên và rủi ro | P0 · rủi ro: RISK-005 |
| Loại thay đổi | new |
| Phụ thuộc chưa xác nhận | — |

#### TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule

| Trường | Giá trị |
| --- | --- |
| id | TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule |
| Mức, loại test | L2 · functional |
| Mô tả điều cần kiểm | Kiểm API POST /v1/scheduled-transfers theo solution |
| Nguồn truy vết | EP-CreateSchedule, FR-040, FR-041, FR-048, MOD-Audit, MOD-Scheduler |
| Tiêu chí chấp nhận | mọi quy tắc đầu vào của trường và mọi mã phản hồi của endpoint được kiểm |
| Ngưỡng NFR | — |
| Phương pháp xác minh | Test |
| Ưu tiên và rủi ro | P0 · rủi ro: RISK-005 |
| Loại thay đổi | new |
| Phụ thuộc chưa xác nhận | — |

#### TR-SCHEDULED-TRANSFER-L2-EP-GetSchedule

| Trường | Giá trị |
| --- | --- |
| id | TR-SCHEDULED-TRANSFER-L2-EP-GetSchedule |
| Mức, loại test | L2 · functional |
| Mô tả điều cần kiểm | Kiểm API GET /v1/scheduled-transfers/{scheduleId} theo solution |
| Nguồn truy vết | EP-GetSchedule, FR-042, MOD-Scheduler |
| Tiêu chí chấp nhận | mọi quy tắc đầu vào của trường và mọi mã phản hồi của endpoint được kiểm |
| Ngưỡng NFR | — |
| Phương pháp xác minh | Test |
| Ưu tiên và rủi ro | P0 · rủi ro: RISK-005 |
| Loại thay đổi | new |
| Phụ thuộc chưa xác nhận | — |

#### TR-SCHEDULED-TRANSFER-L2-EP-ListRuns

| Trường | Giá trị |
| --- | --- |
| id | TR-SCHEDULED-TRANSFER-L2-EP-ListRuns |
| Mức, loại test | L2 · functional |
| Mô tả điều cần kiểm | Kiểm API GET /v1/scheduled-transfers/{scheduleId}/runs theo solution |
| Nguồn truy vết | EP-ListRuns, FR-042, MOD-Scheduler |
| Tiêu chí chấp nhận | mọi quy tắc đầu vào của trường và mọi mã phản hồi của endpoint được kiểm |
| Ngưỡng NFR | — |
| Phương pháp xác minh | Test |
| Ưu tiên và rủi ro | P0 · rủi ro: RISK-005 |
| Loại thay đổi | new |
| Phụ thuộc chưa xác nhận | — |

#### TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules

| Trường | Giá trị |
| --- | --- |
| id | TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules |
| Mức, loại test | L2 · functional |
| Mô tả điều cần kiểm | Kiểm API GET /v1/accounts/{accountId}/scheduled-transfers theo solution |
| Nguồn truy vết | EP-ListSchedules, FR-042, MOD-Scheduler |
| Tiêu chí chấp nhận | mọi quy tắc đầu vào của trường và mọi mã phản hồi của endpoint được kiểm |
| Ngưỡng NFR | — |
| Phương pháp xác minh | Test |
| Ưu tiên và rủi ro | P0 · rủi ro: RISK-005 |
| Loại thay đổi | new |
| Phụ thuộc chưa xác nhận | — |

#### TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus

| Trường | Giá trị |
| --- | --- |
| id | TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus |
| Mức, loại test | L2 · functional |
| Mô tả điều cần kiểm | Kiểm API PATCH /v1/scheduled-transfers/{scheduleId} theo solution |
| Nguồn truy vết | EP-UpdateScheduleStatus, FR-044, FR-048, MOD-Audit, MOD-Scheduler |
| Tiêu chí chấp nhận | mọi quy tắc đầu vào của trường và mọi mã phản hồi của endpoint được kiểm |
| Ngưỡng NFR | — |
| Phương pháp xác minh | Test |
| Ưu tiên và rủi ro | P0 · rủi ro: RISK-005 |
| Loại thay đổi | new |
| Phụ thuộc chưa xác nhận | — |

#### TR-SCHEDULED-TRANSFER-L2-FR-040

| Trường | Giá trị |
| --- | --- |
| id | TR-SCHEDULED-TRANSFER-L2-FR-040 |
| Mức, loại test | L2 · functional |
| Mô tả điều cần kiểm | Kiểm FR-040: Tạo lịch chuyển tiền một lần tại thời điểm trong tương lai |
| Nguồn truy vết | FR-040, MOD-Scheduler |
| Tiêu chí chấp nhận | POST tạo lịch ONCE hợp lệ trả 201, có scheduleId và status ACTIVE; Số dư hai tài khoản không đổi và không có bản ghi ledger_entries mới sau khi tạo lịch; firstRunAt cách hiện tại < 5 phút hoặc > 90 ngày thì trả 422 SCHEDULE_TIME_INVALID; toAccountId không tồn tại thì trả 404 ACCOUNT_NOT_FOUND; fromAccountId khác chủ API key thì trả 403 FORBIDDEN; Lịch thứ 21 ACTIVE/PAUSED của một tài khoản trả 422 SCHEDULE_LIMIT_REACHED; Chỉ lịch ACTIVE hoặc PAUSED được tính vào giới hạn 20 lịch của tài khoản; lịch COMPLETED hoặc CANCELLED không được tính |
| Ngưỡng NFR | — |
| Phương pháp xác minh | Test |
| Ưu tiên và rủi ro | P0 · rủi ro: RISK-005 |
| Loại thay đổi | new |
| Phụ thuộc chưa xác nhận | ASM-001, OQ-001 (pendingConfirmation) |

#### TR-SCHEDULED-TRANSFER-L2-FR-041

| Trường | Giá trị |
| --- | --- |
| id | TR-SCHEDULED-TRANSFER-L2-FR-041 |
| Mức, loại test | L2 · functional |
| Mô tả điều cần kiểm | Kiểm FR-041: Tạo lịch định kỳ DAILY/WEEKLY/MONTHLY có ngày kết thúc hoặc số lần tối đa |
| Nguồn truy vết | FR-041, MOD-Scheduler |
| Tiêu chí chấp nhận | Lịch DAILY, WEEKLY hoặc MONTHLY có endDate hoặc maxRuns được tạo thành công (201); Lịch định kỳ thiếu cả endDate và maxRuns trả 400 INVALID_REQUEST; Sau khi một lần chạy kết thúc, hệ thống sinh lần chạy PENDING kế tiếp đúng chu kỳ cho đến khi đạt maxRuns hoặc endDate, rồi lịch COMPLETED; Lịch MONTHLY đặt ngày 29, 30 hoặc 31 chạy vào ngày cuối tháng ở tháng không đủ ngày, và ngày 29/02 ở năm nhuận; Lịch định kỳ có endDate trước ngày của lần chạy đầu, hoặc endDate quá 366 ngày kể từ lần chạy đầu, thì trả 422 SCHEDULE_TIME_INVALID; Lịch định kỳ có cả endDate và maxRuns kết thúc (COMPLETED) ở điều kiện đạt trước, không sinh thêm lần chạy PENDING |
| Ngưỡng NFR | — |
| Phương pháp xác minh | Test |
| Ưu tiên và rủi ro | P0 · rủi ro: RISK-005 |
| Loại thay đổi | new |
| Phụ thuộc chưa xác nhận | ASM-001, OQ-001 (pendingConfirmation) |

#### TR-SCHEDULED-TRANSFER-L2-FR-042

| Trường | Giá trị |
| --- | --- |
| id | TR-SCHEDULED-TRANSFER-L2-FR-042 |
| Mức, loại test | L2 · functional |
| Mô tả điều cần kiểm | Kiểm FR-042: Tra cứu một lịch, danh sách lịch của tài khoản và các lần chạy của lịch |
| Nguồn truy vết | FR-042, MOD-Scheduler |
| Tiêu chí chấp nhận | GET một lịch trả status và nextRunAt; GET danh sách lịch của tài khoản hỗ trợ lọc status và phân trang limit, offset; GET runs trả mỗi lần chạy với scheduledAt, status, transferId và reasonCode, sắp mới nhất trước; Người không phải chủ tài khoản nguồn nhận 403 FORBIDDEN; mã lịch không có nhận 404 SCHEDULE_NOT_FOUND |
| Ngưỡng NFR | — |
| Phương pháp xác minh | Test |
| Ưu tiên và rủi ro | P0 · rủi ro: RISK-005 |
| Loại thay đổi | new |
| Phụ thuộc chưa xác nhận | ASM-001, OQ-001 (pendingConfirmation) |

#### TR-SCHEDULED-TRANSFER-L2-FR-043

| Trường | Giá trị |
| --- | --- |
| id | TR-SCHEDULED-TRANSFER-L2-FR-043 |
| Mức, loại test | L2 · functional |
| Mô tả điều cần kiểm | Kiểm FR-043: Hủy lịch chuyển tiền |
| Nguồn truy vết | FR-043, MOD-Scheduler |
| Tiêu chí chấp nhận | DELETE lịch ACTIVE đổi lịch thành CANCELLED và các lần chạy PENDING thành CANCELLED; DELETE lịch đã CANCELLED trả 200 với kết quả cũ; DELETE khi có lần chạy RUNNING hoặc lịch COMPLETED trả 409 SCHEDULE_NOT_CANCELLABLE |
| Ngưỡng NFR | — |
| Phương pháp xác minh | Test |
| Ưu tiên và rủi ro | P0 · rủi ro: RISK-005 |
| Loại thay đổi | new |
| Phụ thuộc chưa xác nhận | ASM-001, OQ-001 (pendingConfirmation) |

#### TR-SCHEDULED-TRANSFER-L2-FR-044

| Trường | Giá trị |
| --- | --- |
| id | TR-SCHEDULED-TRANSFER-L2-FR-044 |
| Mức, loại test | L2 · functional |
| Mô tả điều cần kiểm | Kiểm FR-044: Tạm dừng và tiếp tục lịch định kỳ |
| Nguồn truy vết | FR-044, MOD-Scheduler |
| Tiêu chí chấp nhận | PATCH status=PAUSED trên lịch định kỳ ACTIVE đổi sang PAUSED, scheduler không chạy lịch này; PATCH status=ACTIVE trên lịch PAUSED đổi sang ACTIVE và các lần đã bỏ lỡ trong lúc tạm dừng có trạng thái SKIPPED, không bị chạy bù; PATCH sai trạng thái nguồn hoặc trên lịch ONCE trả 409 SCHEDULE_STATE_INVALID |
| Ngưỡng NFR | — |
| Phương pháp xác minh | Test |
| Ưu tiên và rủi ro | P0 · rủi ro: RISK-005 |
| Loại thay đổi | new |
| Phụ thuộc chưa xác nhận | ASM-001, OQ-001 (pendingConfirmation) |

#### TR-SCHEDULED-TRANSFER-L2-FR-045

| Trường | Giá trị |
| --- | --- |
| id | TR-SCHEDULED-TRANSFER-L2-FR-045 |
| Mức, loại test | L2 · functional |
| Mô tả điều cần kiểm | Kiểm FR-045: Đến hạn, tự chuyển tiền bằng luồng chuyển tiền hiện có; Idempotency-Key = mã lịch + thời điểm dự kiến |
| Nguồn truy vết | FR-045, MOD-Scheduler, SCN-ScheduledRun |
| Tiêu chí chấp nhận | Lần chạy đến hạn tạo đúng một giao dịch COMPLETED qua luồng chuyển tiền hiện có, run SUCCEEDED có transferId; Idempotency-Key của lần chạy đầu bằng mã lịch + thời điểm dự kiến; Hạn mức ngày, số dư, trạng thái tài khoản và Fraud Check được kiểm tra tại lúc chạy, không phải lúc tạo lịch; Giao dịch sinh từ lịch được tính vào hạn mức ngày và hiển thị trong lịch sử giao dịch; Giao dịch sinh từ lịch hiển thị trong lịch sử giao dịch (FR-013) kèm trường scheduleId là mã lịch |
| Ngưỡng NFR | — |
| Phương pháp xác minh | Test |
| Ưu tiên và rủi ro | P0 · rủi ro: RISK-005 |
| Loại thay đổi | new |
| Phụ thuộc chưa xác nhận | ASM-001, OQ-001 (pendingConfirmation) |

#### TR-SCHEDULED-TRANSFER-L2-FR-046

| Trường | Giá trị |
| --- | --- |
| id | TR-SCHEDULED-TRANSFER-L2-FR-046 |
| Mức, loại test | L2 · functional |
| Mô tả điều cần kiểm | Kiểm FR-046: Xử lý thất bại của lần chạy theo loại lỗi (BR-13): thử lại, FAILED, tự PAUSED sau 3 lần FAILED liên tiếp |
| Nguồn truy vết | FR-046, MOD-Scheduler, SCN-ScheduledRun |
| Tiêu chí chấp nhận | FRAUD_CHECK_UNAVAILABLE được thử lại tối đa 3 lần cách 5 phút, hết lượt thì run FAILED với reasonCode; INSUFFICIENT_FUNDS được thử lại tối đa 2 lần cách 1 giờ, hết lượt thì run FAILED; ACCOUNT_INACTIVE, FRAUD_REJECTED, LIMIT_EXCEEDED làm run FAILED ngay, không thử lại; Sau 3 lần chạy FAILED liên tiếp lịch định kỳ chuyển PAUSED |
| Ngưỡng NFR | — |
| Phương pháp xác minh | Test |
| Ưu tiên và rủi ro | P0 · rủi ro: RISK-005 |
| Loại thay đổi | new |
| Phụ thuộc chưa xác nhận | ASM-001, OQ-001 (pendingConfirmation) |

#### TR-SCHEDULED-TRANSFER-L2-FR-047

| Trường | Giá trị |
| --- | --- |
| id | TR-SCHEDULED-TRANSFER-L2-FR-047 |
| Mức, loại test | L2 · functional |
| Mô tả điều cần kiểm | Kiểm FR-047: Thông báo kết quả mỗi lần chạy qua SMS (ScheduledRunCompleted/ScheduledRunFailed qua outbox) |
| Nguồn truy vết | FR-047, MOD-Notify, SCN-ScheduledRun |
| Tiêu chí chấp nhận | Mỗi lần chạy thành công phát sự kiện ScheduledRunCompleted có transferId qua outbox; Mỗi lần chạy FAILED phát sự kiện ScheduledRunFailed có reasonCode qua outbox; SMS kết quả được gửi cho người gửi; lỗi gửi SMS không đổi trạng thái lần chạy; Sự kiện ScheduledRunCompleted/ScheduledRunFailed có đủ các trường eventId, scheduleId, runId, fromAccountId, toAccountId, amount, correlationId, kèm transferId (thành công) hoặc reasonCode (thất bại), trên cùng topic với sự kiện giao dịch; Nội dung SMS của lần chạy thất bại nêu rõ số dư chỉ được kiểm tra lúc chạy |
| Ngưỡng NFR | — |
| Phương pháp xác minh | Test |
| Ưu tiên và rủi ro | P1 · rủi ro: — |
| Loại thay đổi | new |
| Phụ thuộc chưa xác nhận | ASM-002 (pendingConfirmation) |

#### TR-SCHEDULED-TRANSFER-L2-FR-048

| Trường | Giá trị |
| --- | --- |
| id | TR-SCHEDULED-TRANSFER-L2-FR-048 |
| Mức, loại test | L2 · functional |
| Mô tả điều cần kiểm | Kiểm FR-048: Ghi audit cho tạo, hủy, tạm dừng, tiếp tục lịch và mọi kết quả lần chạy, tác nhân SCHEDULER |
| Nguồn truy vết | FR-048, MOD-Audit, SCN-ScheduledRun |
| Tiêu chí chấp nhận | Tạo, hủy, tạm dừng và tiếp tục lịch đều ghi audit với correlationId, thời điểm, tác nhân là mã tài khoản, loại sự kiện và mã lịch; Mọi kết quả lần chạy ghi audit với tác nhân SCHEDULER và mã lịch |
| Ngưỡng NFR | — |
| Phương pháp xác minh | Test |
| Ưu tiên và rủi ro | P0 · rủi ro: — |
| Loại thay đổi | new |
| Phụ thuộc chưa xác nhận | — |

#### TR-SCHEDULED-TRANSFER-L2-FR-049

| Trường | Giá trị |
| --- | --- |
| id | TR-SCHEDULED-TRANSFER-L2-FR-049 |
| Mức, loại test | L2 · functional |
| Mô tả điều cần kiểm | Kiểm FR-049: Chạy bù các lần chạy quá hạn không quá 24 giờ sau khi hệ thống ngừng; quá hạn thì MISSED |
| Nguồn truy vết | FR-049, MOD-Scheduler, SCN-ScheduledRun |
| Tiêu chí chấp nhận | Lần chạy quá hạn không quá 24 giờ sau khi hệ thống ngừng được chạy đúng một lần; Lần chạy quá hạn quá 24 giờ chuyển MISSED, phát ScheduledRunFailed với reasonCode MISSED và không có giao dịch |
| Ngưỡng NFR | — |
| Phương pháp xác minh | Test |
| Ưu tiên và rủi ro | P0 · rủi ro: RISK-005 |
| Loại thay đổi | new |
| Phụ thuộc chưa xác nhận | ASM-001, OQ-001 (pendingConfirmation) |

#### TR-SCHEDULED-TRANSFER-L2-NFR-010

| Trường | Giá trị |
| --- | --- |
| id | TR-SCHEDULED-TRANSFER-L2-NFR-010 |
| Mức, loại test | L2 · performance |
| Mô tả điều cần kiểm | Kiểm NFR-010: Đúng giờ: 99% lần chạy bắt đầu trong 60 giây kể từ thời điểm dự kiến |
| Nguồn truy vết | MOD-Scheduler, NFR-010 |
| Tiêu chí chấp nhận | đạt 60 s |
| Ngưỡng NFR | NFR-010: 60 s |
| Phương pháp xác minh | Test |
| Ưu tiên và rủi ro | P0 · rủi ro: RISK-005 |
| Loại thay đổi | new |
| Phụ thuộc chưa xác nhận | — |

#### TR-SCHEDULED-TRANSFER-L2-NFR-011

| Trường | Giá trị |
| --- | --- |
| id | TR-SCHEDULED-TRANSFER-L2-NFR-011 |
| Mức, loại test | L2 · performance |
| Mô tả điều cần kiểm | Kiểm NFR-011: Không chạy trùng: 0 giao dịch trùng cho một lần chạy, kể cả khi hai scheduler cùng quét |
| Nguồn truy vết | NFR-011 |
| Tiêu chí chấp nhận | đạt 0 giao dịch trùng |
| Ngưỡng NFR | NFR-011: 0 giao dịch trùng |
| Phương pháp xác minh | Test |
| Ưu tiên và rủi ro | P0 · rủi ro: — |
| Loại thay đổi | new |
| Phụ thuộc chưa xác nhận | — |

#### TR-SCHEDULED-TRANSFER-L2-NFR-012

| Trường | Giá trị |
| --- | --- |
| id | TR-SCHEDULED-TRANSFER-L2-NFR-012 |
| Mức, loại test | L2 · performance |
| Mô tả điều cần kiểm | Kiểm NFR-012: Phục hồi: sau khởi động lại, lần chạy quá hạn trong 24 giờ được xử lý trong 5 chu kỳ quét đầu |
| Nguồn truy vết | NFR-012 |
| Tiêu chí chấp nhận | đạt 5 chu kỳ quét |
| Ngưỡng NFR | NFR-012: 5 chu kỳ quét |
| Phương pháp xác minh | Test |
| Ưu tiên và rủi ro | P0 · rủi ro: — |
| Loại thay đổi | new |
| Phụ thuộc chưa xác nhận | — |

#### TR-SCHEDULED-TRANSFER-L2-NFR-013

| Trường | Giá trị |
| --- | --- |
| id | TR-SCHEDULED-TRANSFER-L2-NFR-013 |
| Mức, loại test | L2 · performance |
| Mô tả điều cần kiểm | Kiểm NFR-013: Xử lý 100 lần chạy đến hạn cùng lúc mà p95 chuyển tiền qua API vẫn <= 500 ms |
| Nguồn truy vết | NFR-013 |
| Tiêu chí chấp nhận | đạt 500 ms |
| Ngưỡng NFR | NFR-013: 500 ms |
| Phương pháp xác minh | Test |
| Ưu tiên và rủi ro | P0 · rủi ro: — |
| Loại thay đổi | new |
| Phụ thuộc chưa xác nhận | — |

#### TR-SCHEDULED-TRANSFER-L2-NFR-014

| Trường | Giá trị |
| --- | --- |
| id | TR-SCHEDULED-TRANSFER-L2-NFR-014 |
| Mức, loại test | L2 · performance |
| Mô tả điều cần kiểm | Kiểm NFR-014: Truy vết: 100% lần chạy có correlationId riêng, đi theo giao dịch, audit và lời gọi Fraud Check |
| Nguồn truy vết | NFR-014 |
| Tiêu chí chấp nhận | đạt 100 % |
| Ngưỡng NFR | NFR-014: 100 % |
| Phương pháp xác minh | Test |
| Ưu tiên và rủi ro | P0 · rủi ro: — |
| Loại thay đổi | new |
| Phụ thuộc chưa xác nhận | — |

#### TR-SCHEDULED-TRANSFER-L2-SM-Schedule

| Trường | Giá trị |
| --- | --- |
| id | TR-SCHEDULED-TRANSFER-L2-SM-Schedule |
| Mức, loại test | L2 · functional |
| Mô tả điều cần kiểm | Kiểm vòng đời Vòng đời lịch chuyển tiền |
| Nguồn truy vết | DE-ScheduledTransfer, FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049, MOD-Scheduler, SM-Schedule |
| Tiêu chí chấp nhận | mọi chuyển trạng thái hợp lệ được thực hiện; chuyển không hợp lệ bị từ chối |
| Ngưỡng NFR | — |
| Phương pháp xác minh | Test |
| Ưu tiên và rủi ro | P0 · rủi ro: RISK-005 |
| Loại thay đổi | new |
| Phụ thuộc chưa xác nhận | — |

#### TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun

| Trường | Giá trị |
| --- | --- |
| id | TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun |
| Mức, loại test | L2 · functional |
| Mô tả điều cần kiểm | Kiểm vòng đời Vòng đời lần chạy |
| Nguồn truy vết | DE-ScheduledRun, FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049, MOD-Scheduler, SM-ScheduledRun |
| Tiêu chí chấp nhận | mọi chuyển trạng thái hợp lệ được thực hiện; chuyển không hợp lệ bị từ chối |
| Ngưỡng NFR | — |
| Phương pháp xác minh | Test |
| Ưu tiên và rủi ro | P0 · rủi ro: RISK-005 |
| Loại thay đổi | new |
| Phụ thuộc chưa xác nhận | — |

### 3.3 L3: Cô lập service

#### TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler

| Trường | Giá trị |
| --- | --- |
| id | TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler |
| Mức, loại test | L3 · resilience |
| Mô tả điều cần kiểm | Kiểm MOD-Scheduler khi mọi phụ thuộc bị giả (stub) |
| Nguồn truy vết | FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049, INT-01, MOD-Account, MOD-Audit, MOD-Risk, MOD-Scheduler, MOD-Transfer |
| Tiêu chí chấp nhận | mọi phụ thuộc lỗi được xử lý theo ErrorMapping; hợp đồng interface được giữ |
| Ngưỡng NFR | — |
| Phương pháp xác minh | Test |
| Ưu tiên và rủi ro | P0 · rủi ro: RISK-001, RISK-005 |
| Loại thay đổi | new |
| Phụ thuộc chưa xác nhận | ASM-001, OQ-001 (pendingConfirmation) |

## 4. Test Requirement phi chức năng

| TR | NFR | Nhóm | Ngưỡng | Đơn vị | Loại test |
| --- | --- | --- | --- | --- | --- |
| TR-SCHEDULED-TRANSFER-L2-NFR-010 | NFR-010 |  | 60 | s | performance |
| TR-SCHEDULED-TRANSFER-L2-NFR-011 | NFR-011 |  | 0 | giao dịch trùng | performance |
| TR-SCHEDULED-TRANSFER-L2-NFR-012 | NFR-012 |  | 5 | chu kỳ quét | performance |
| TR-SCHEDULED-TRANSFER-L2-NFR-013 | NFR-013 |  | 500 | ms | performance |
| TR-SCHEDULED-TRANSFER-L2-NFR-014 | NFR-014 |  | 100 | % | performance |

## 5. Tiêu chí vào và ra, tạm dừng và tiếp tục

| Mục | Nội dung |
| --- | --- |
| Điều kiện vào | môi trường đã sẵn sàng; stub và dữ liệu nền đã nạp |
| Điều kiện ra | mọi TestCase trong set đã chạy |
| Tạm dừng | stub không phản hồi hoặc dữ liệu nền không nạp được |

## 6. Truy vết (nguồn → TR)

| Nguồn | Test Requirement |
| --- | --- |
| DE-ScheduledRun | TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun |
| DE-ScheduledTransfer | TR-SCHEDULED-TRANSFER-L2-SM-Schedule |
| EP-CancelSchedule | TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule |
| EP-CreateSchedule | TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule |
| EP-GetSchedule | TR-SCHEDULED-TRANSFER-L2-EP-GetSchedule |
| EP-ListRuns | TR-SCHEDULED-TRANSFER-L2-EP-ListRuns |
| EP-ListSchedules | TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules |
| EP-UpdateScheduleStatus | TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus |
| FR-040 | TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule, TR-SCHEDULED-TRANSFER-L2-FR-040, TR-SCHEDULED-TRANSFER-L2-SM-Schedule, TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun, TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler |
| FR-041 | TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule, TR-SCHEDULED-TRANSFER-L2-FR-041, TR-SCHEDULED-TRANSFER-L2-SM-Schedule, TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun, TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler |
| FR-042 | TR-SCHEDULED-TRANSFER-L2-EP-GetSchedule, TR-SCHEDULED-TRANSFER-L2-EP-ListRuns, TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules, TR-SCHEDULED-TRANSFER-L2-FR-042, TR-SCHEDULED-TRANSFER-L2-SM-Schedule, TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun, TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler |
| FR-043 | TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule, TR-SCHEDULED-TRANSFER-L2-FR-043, TR-SCHEDULED-TRANSFER-L2-SM-Schedule, TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun, TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler |
| FR-044 | TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus, TR-SCHEDULED-TRANSFER-L2-FR-044, TR-SCHEDULED-TRANSFER-L2-SM-Schedule, TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun, TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler |
| FR-045 | TR-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun, TR-SCHEDULED-TRANSFER-L2-FR-045, TR-SCHEDULED-TRANSFER-L2-SM-Schedule, TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun, TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler |
| FR-046 | TR-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun, TR-SCHEDULED-TRANSFER-L2-FR-046, TR-SCHEDULED-TRANSFER-L2-SM-Schedule, TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun, TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler |
| FR-047 | TR-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun, TR-SCHEDULED-TRANSFER-L2-FR-047 |
| FR-048 | TR-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun, TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule, TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule, TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus, TR-SCHEDULED-TRANSFER-L2-FR-048 |
| FR-049 | TR-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun, TR-SCHEDULED-TRANSFER-L2-FR-049, TR-SCHEDULED-TRANSFER-L2-SM-Schedule, TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun, TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler |
| INT-01 | TR-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun, TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler |
| MOD-API | TR-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun |
| MOD-Account | TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler |
| MOD-Audit | TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule, TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule, TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus, TR-SCHEDULED-TRANSFER-L2-FR-048, TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler |
| MOD-Notify | TR-SCHEDULED-TRANSFER-L2-FR-047 |
| MOD-Risk | TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler |
| MOD-Scheduler | TR-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun, TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule, TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule, TR-SCHEDULED-TRANSFER-L2-EP-GetSchedule, TR-SCHEDULED-TRANSFER-L2-EP-ListRuns, TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules, TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus, TR-SCHEDULED-TRANSFER-L2-FR-040, TR-SCHEDULED-TRANSFER-L2-FR-041, TR-SCHEDULED-TRANSFER-L2-FR-042, TR-SCHEDULED-TRANSFER-L2-FR-043, TR-SCHEDULED-TRANSFER-L2-FR-044, TR-SCHEDULED-TRANSFER-L2-FR-045, TR-SCHEDULED-TRANSFER-L2-FR-046, TR-SCHEDULED-TRANSFER-L2-FR-049, TR-SCHEDULED-TRANSFER-L2-NFR-010, TR-SCHEDULED-TRANSFER-L2-SM-Schedule, TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun, TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler |
| MOD-Transfer | TR-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun, TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler |
| NFR-010 | TR-SCHEDULED-TRANSFER-L2-NFR-010 |
| NFR-011 | TR-SCHEDULED-TRANSFER-L2-NFR-011 |
| NFR-012 | TR-SCHEDULED-TRANSFER-L2-NFR-012 |
| NFR-013 | TR-SCHEDULED-TRANSFER-L2-NFR-013 |
| NFR-014 | TR-SCHEDULED-TRANSFER-L2-NFR-014 |
| SCN-ScheduledRun | TR-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun, TR-SCHEDULED-TRANSFER-L2-FR-045, TR-SCHEDULED-TRANSFER-L2-FR-046, TR-SCHEDULED-TRANSFER-L2-FR-047, TR-SCHEDULED-TRANSFER-L2-FR-048, TR-SCHEDULED-TRANSFER-L2-FR-049 |
| SM-Schedule | TR-SCHEDULED-TRANSFER-L2-SM-Schedule |
| SM-ScheduledRun | TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun |

## 7. Báo cáo phủ và khoảng trống

| Luật | Đã kiểm | Kết quả | Chi tiết |
| --- | --- | --- | --- |
| T1 | 15 | đạt |  |
| T2 | 25 | đạt |  |
| T3 | 183 | đạt |  |
| T4 | 183 | đạt |  |

Test Requirement đang chờ xác nhận: TR-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun, TR-SCHEDULED-TRANSFER-L2-FR-040, TR-SCHEDULED-TRANSFER-L2-FR-041, TR-SCHEDULED-TRANSFER-L2-FR-042, TR-SCHEDULED-TRANSFER-L2-FR-043, TR-SCHEDULED-TRANSFER-L2-FR-044, TR-SCHEDULED-TRANSFER-L2-FR-045, TR-SCHEDULED-TRANSFER-L2-FR-046, TR-SCHEDULED-TRANSFER-L2-FR-047, TR-SCHEDULED-TRANSFER-L2-FR-049, TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler

## Lịch sử phiên bản

| Phiên bản | Trạng thái | Baseline TDD | CR | Nút thêm | Nút đổi | Nút bỏ |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | Draft | b-ec21ecfae083 | CR-001 | 1555 | 0 | 0 |

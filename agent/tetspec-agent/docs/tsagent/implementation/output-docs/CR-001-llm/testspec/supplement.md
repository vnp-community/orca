# Bổ sung từ rà soát test specs CR-001

## Tiêu chí chấp nhận

| Mã | Yêu cầu | Tiêu chí | Căn cứ |
| --- | --- | --- | --- |
| AC-REV-001 | FR-041 | Lịch định kỳ có endDate trước ngày của lần chạy đầu, hoặc endDate quá 366 ngày kể từ lần chạy đầu, thì trả 422 SCHEDULE_TIME_INVALID | SRS BR-11; SRS mã lỗi SCHEDULE_TIME_INVALID; solution §3.8 bước 4 |
| AC-REV-002 | FR-041 | Lịch định kỳ có cả endDate và maxRuns kết thúc (COMPLETED) ở điều kiện đạt trước, không sinh thêm lần chạy PENDING | SRS BR-11 ("đủ điều kiện nào trước thì kết thúc trước") |
| AC-REV-003 | FR-040 | Chỉ lịch ACTIVE hoặc PAUSED được tính vào giới hạn 20 lịch của tài khoản; lịch COMPLETED hoặc CANCELLED không được tính | SRS BR-15; solution §3.8 bước 5 |
| AC-REV-004 | FR-045 | Giao dịch sinh từ lịch hiển thị trong lịch sử giao dịch (FR-013) kèm trường scheduleId là mã lịch | SRS BR-16; solution §3.5 (Q-S6) |
| AC-REV-005 | FR-047 | Sự kiện ScheduledRunCompleted/ScheduledRunFailed có đủ các trường eventId, scheduleId, runId, fromAccountId, toAccountId, amount, correlationId, kèm transferId (thành công) hoặc reasonCode (thất bại), trên cùng topic với sự kiện giao dịch | SRS mục Giao diện ngoài (INT-02); solution §3.9 |
| AC-REV-006 | FR-047 | Nội dung SMS của lần chạy thất bại nêu rõ số dư chỉ được kiểm tra lúc chạy | CR cr.md rủi ro CR1-R3; solution §3.9 mẫu SMS |

# SRS bổ sung: Chuyển tiền hẹn lịch và định kỳ (MiniPay)

Oct 7, 2026 · @Dat Nguyen Tien

Tài liệu này đặc tả yêu cầu cho tính năng chuyển tiền hẹn lịch và định kỳ của MiniPay (CR-001); tính năng chưa được triển khai và chỉ có hiệu lực sau khi CR-001 được duyệt.

## Giới thiệu và phạm vi

SRS này bổ sung cho SRS gốc của MiniPay, không thay thế: mọi yêu cầu, quy tắc và mã lỗi trong SRS gốc (FR-001 đến FR-030, BR-01 đến BR-07, mã lỗi HTTP) vẫn áp dụng. Mã yêu cầu mới bắt đầu từ FR-040, NFR-010, BR-10 để không trùng mã cũ.

Trong phạm vi: tạo, tra cứu, hủy, tạm dừng và tiếp tục lịch chuyển tiền; tự động chuyển tiền đúng hạn; thử lại, chạy bù, thông báo, audit. Ngoài phạm vi: giữ chỗ số dư, sửa lịch đã tạo, đa đồng tiền, giao diện web hoặc đi động.

| Thuật ngữ | Ý nghĩa |
| --- | --- |
| Lịch chuyển tiền (ScheduledTransfer) | Yêu cầu chuyển tiền được lưu để thực hiện sau, một lần hoặc lặp lại |
| Lần chạy (ScheduledRun) | Một lần thực hiện dự kiến của lịch tại một thời điểm cụ thể |
| Chu kỳ lặp (recurrence) | Một trong ONCE, DAILY, WEEKLY, MONTHLY |
| Scheduler | Thành phần quét các lần chạy đến hạn và gọi luồng chuyển tiền |
| Chạy bù | Thực hiện lần chạy đã quá hạn vì hệ thống ngừng hoạt động |

## Mô tả tổng quan

Khách hàng đặt lịch qua API; đến hạn, hệ thống tự chuyển tiền bằng đúng luồng chuyển tiền hiện có, nên mọi kiểm tra (idempotency, hạn mức, số dư, Fraud Check) được áp dụng lại tại thời điểm chạy chứ không tại lúc đặt lịch.

| Đối tượng | Vai trò |
| --- | --- |
| Khách hàng | Gọi API để tạo, xem, hủy, tạm dừng, tiếp tục lịch trên tài khoản của mình |
| Scheduler (nội bộ) | Tự động tạo giao dịch khi lần chạy đến hạn; ghi audit với tác nhân SCHEDULER |
| Fraud Check, SMS Gateway (ngoài) | Dùng lại INT-01 và INT-03, không đổi contract |

Ràng buộc: CON-001 đến CON-004 của SRS gốc vẫn áp dụng; thêm CON-010: scheduler chạy cùng tiến trình và cùng CSDL với MiniPay (không dùng hạ tầng lập lịch riêng).

Giả định:

- Thời gian lịch tính theo UTC+7, nhất quán với BR-05 của SRS gốc.
- Khách hàng chấp nhận số dư chỉ được kiểm tra lúc chạy, không được đảm bảo khi đặt lịch.

## Yêu cầu chức năng

Tính năng có 10 yêu cầu chức năng, 7 yêu cầu ưu tiên Must; mỗi yêu cầu có mã để truy vết sang thiết kế và test.

| Mã | Yêu cầu | Tiêu chí chấp nhận | Ưu tiên |
| --- | --- | --- | --- |
| FR-040 | Hệ thống phải cho phép tạo lịch chuyển tiền một lần tại thời điểm trong tương lai | Trả mã lịch và trạng thái ACTIVE; chỉ chủ tài khoản nguồn được tạo; thời điểm không hợp lệ thì trả SCHEDULE\_TIME\_INVALID; tài khoản đích không tồn tại thì trả ACCOUNT\_NOT\_FOUND; không trừ tiền lúc tạo | Must |
| FR-041 | Hệ thống phải cho phép tạo lịch định kỳ theo DAILY, WEEKLY hoặc MONTHLY | Lịch có ngày kết thúc hoặc số lần tối đa (BR-11); thiếu cả hai thì trả INVALID\_REQUEST; hệ thống sinh lần chạy kế tiếp sau mỗi lần chạy kết thúc | Must |
| FR-042 | Hệ thống phải cho phép tra cứu một lịch, danh sách lịch của tài khoản và các lần chạy của lịch | Trả trạng thái lịch, lần chạy kế tiếp, với mỗi lần chạy: thời điểm dự kiến, trạng thái, mã giao dịch, mã lý do nếu thất bại; phân trang; chỉ chủ tài khoản được xem | Must |
| FR-043 | Hệ thống phải cho phép hủy lịch | Lịch chuyển CANCELLED; các lần chạy PENDING chuyển CANCELLED; lần đang RUNNING không bị hủy, trả SCHEDULE\_NOT\_CANCELLABLE; hủy lịch đã hủy trả kết quả cũ | Must |
| FR-044 | Hệ thống phải cho phép tạm dừng và tiếp tục lịch định kỳ | Lịch PAUSED không sinh lần chạy mới; khi tiếp tục, các lần đã bị bỏ lỡ trong lúc tạm dừng được đánh dấu SKIPPED, không chạy bù | Should |
| FR-045 | Đến hạn, hệ thống phải tự chuyển tiền bằng luồng chuyển tiền hiện có (FR-010 đến FR-012, FR-020) | Mỗi lần chạy tạo tối đa một giao dịch; Idempotency-Key = mã lịch + thời điểm dự kiến; hạn mức, số dư, Fraud Check kiểm tra tại lúc chạy; thành công thì lần chạy SUCCEEDED kèm mã giao dịch | Must |
| FR-046 | Hệ thống phải xử lý thất bại của lần chạy theo loại lỗi (BR-13) | Lỗi tạm thời được thử lại đúng số lần và khoảng cách; hết lượt thử hoặc lỗi vĩnh viễn thì lần chạy FAILED kèm mã lý do; lịch định kỳ PAUSED sau 3 lần FAILED liên tiếp | Must |
| FR-047 | Hệ thống phải thông báo kết quả mỗi lần chạy qua SMS cho người gửi | Phát sự kiện ScheduledRunCompleted hoặc ScheduledRunFailed qua outbox; lỗi gửi SMS không đổi kết quả lần chạy | Should |
| FR-048 | Hệ thống phải ghi audit cho tạo, hủy, tạm dừng, tiếp tục lịch và mọi kết quả lần chạy | Mỗi bản ghi có correlationId, thời điểm, tác nhân (mã tài khoản hoặc SCHEDULER), loại sự kiện, mã lịch; bổ sung cho FR-030 | Must |
| FR-049 | Sau khi hệ thống ngừng hoạt động, hệ thống phải chạy bù các lần chạy đã quá hạn trong thời hạn cho phép | Lần quá hạn không quá 24 giờ được chạy một lần; quá 24 giờ đánh dấu MISSED và thông báo (thời hạn chờ xác nhận OQ-012) | Should |

## Quy tắc nghiệp vụ, trạng thái và mã lỗi

Quy tắc quan trọng nhất: lịch không bảo đảm có tiền; mọi kiểm tra tài chính xảy ra lúc chạy (BR-14).

| Mã | Quy tắc |
| --- | --- |
| BR-10 | Thời điểm chạy phải cách thời điểm tạo từ 5 phút đến 90 ngày; tính theo UTC+7 |
| BR-11 | Lịch định kỳ phải có ngày kết thúc (không quá 366 ngày kể từ lần chạy đầu) hoặc số lần tối đa (không quá 60); đủ điều kiện nào trước thì kết thúc trước |
| BR-12 | Lịch MONTHLY đặt ngày 29, 30 hoặc 31 thì tháng không đủ ngày chạy vào ngày cuối tháng |
| BR-13 | Phân loại thất bại: tạm thời (FRAUD\_CHECK\_UNAVAILABLE) thử lại tối đa 3 lần, cách 5 phút; thiếu số dư (INSUFFICIENT\_FUNDS) thử lại tối đa 2 lần, cách 1 giờ (chờ OQ-010); vĩnh viễn (ACCOUNT\_INACTIVE, FRAUD\_REJECTED, LIMIT\_EXCEEDED) không thử lại |
| BR-14 | Số dư, hạn mức ngày, trạng thái hai tài khoản và Fraud Check được kiểm tra tại lúc chạy; lúc tạo lịch chỉ kiểm tra định dạng, số tiền dương không quá 50.000.000 VND (BR-04 gốc) và tài khoản đích tồn tại |
| BR-15 | Mỗi tài khoản có tối đa 20 lịch ACTIVE hoặc PAUSED (chờ xác nhận OQ-011) |
| BR-16 | Giao dịch sinh từ lịch tính vào hạn mức ngày giống giao dịch thường và hiển thị trong lịch sử (FR-013) kèm mã lịch |

Trạng thái lịch và lần chạy:

| Đối tượng | Trạng thái | Ý nghĩa |
| --- | --- | --- |
| Lịch | ACTIVE | Đang sinh lần chạy |
| Lịch | PAUSED | Tạm dừng do người dùng hoặc do 3 lần FAILED liên tiếp |
| Lịch | COMPLETED | Đã hết lần chạy theo BR-11 hoặc lần chạy duy nhất đã xong |
| Lịch | CANCELLED | Người dùng hủy |
| Lần chạy | PENDING | Chưa đến hạn hoặc chờ thử lại |
| Lần chạy | RUNNING | Đang tạo giao dịch |
| Lần chạy | SUCCEEDED | Giao dịch COMPLETED |
| Lần chạy | FAILED | Hết lượt thử hoặc lỗi vĩnh viễn, kèm mã lý do |
| Lần chạy | SKIPPED | Bỏ qua do lịch bị tạm dừng |
| Lần chạy | MISSED | Quá thời hạn chạy bù (FR-049) |
| Lần chạy | CANCELLED | Lịch bị hủy trước khi chạy |

Mã lỗi mới (mã lỗi cũ như INVALID\_REQUEST, FORBIDDEN, ACCOUNT\_NOT\_FOUND dùng lại):

| Mã lỗi | HTTP | Khi nào |
| --- | --- | --- |
| SCHEDULE\_NOT\_FOUND | 404 | Không có lịch hoặc lần chạy |
| SCHEDULE\_TIME\_INVALID | 422 | Thời điểm chạy ngoài khoảng BR-10 hoặc ngày kết thúc trước lần chạy đầu |
| SCHEDULE\_LIMIT\_REACHED | 422 | Vượt số lịch cho phép (BR-15) |
| SCHEDULE\_NOT\_CANCELLABLE | 409 | Lịch đã COMPLETED hoặc lần chạy đang RUNNING |
| SCHEDULE\_STATE\_INVALID | 409 | Tạm dừng lịch không phải ACTIVE, tiếp tục lịch không phải PAUSED, hoặc lịch một lần |

## Yêu cầu phi chức năng

Hai yêu cầu khó nhất là đúng giờ (NFR-010) và không chạy trùng (NFR-011); NFR-001 và NFR-006 của SRS gốc vẫn giữ nguyên khi scheduler hoạt động.

| Mã | Yêu cầu | Chỉ số đo | Cách kiểm chứng |
| --- | --- | --- | --- |
| NFR-010 | Đúng giờ | 99% lần chạy bắt đầu trong 60 giây kể từ thời điểm dự kiến khi hệ thống hoạt động bình thường | Test tự động với đồng hồ giả, đo khi benchmark |
| NFR-011 | Không chạy trùng | 0 giao dịch trùng cho một lần chạy, kể cả khi hai scheduler cùng quét hoặc chạy lại sau lỗi | Test song song hai scheduler, test khởi động lại giữa lần chạy |
| NFR-012 | Phục hồi | Sau khi khởi động lại, lần chạy quá hạn trong 24 giờ được xử lý trong 5 chu kỳ quét đầu (chờ xác nhận OQ-012) | Test tự động |
| NFR-013 | Hiệu năng | Xử lý 100 lần chạy đến hạn cùng lúc mà p95 chuyển tiền qua API vẫn ≤ 500 ms (NFR-001) | Benchmark |
| NFR-014 | Truy vết | 100% lần chạy có correlationId riêng, đi theo giao dịch, audit và lời gọi Fraud Check | Test tự động |

## Giao diện API mới và giao diện ngoài

Chỉ thêm sáu endpoint REST; xác thực, header Authorization và X-Correlation-Id giống SRS gốc. Contract chi tiết (schema, timeout, retry) được chốt trong TDD.

| API | Yêu cầu liên quan | Ghi chú |
| --- | --- | --- |
| `POST /v1/scheduled-transfers` | FR-040, FR-041 | Bắt buộc header `Idempotency-Key`; body gồm `fromAccountId`, `toAccountId`, `amount`, `note`, `recurrence`, `firstRunAt`, và `endDate` hoặc `maxRuns` nếu định kỳ |
| `GET /v1/scheduled-transfers/{scheduleId}` | FR-042 | Trả trạng thái và lần chạy kế tiếp |
| `GET /v1/accounts/{accountId}/scheduled-transfers` | FR-042 | Tham số `status`, `limit`, `offset` |
| `GET /v1/scheduled-transfers/{scheduleId}/runs` | FR-042 | Danh sách lần chạy, mới nhất trước |
| `DELETE /v1/scheduled-transfers/{scheduleId}` | FR-043 | Hủy lịch |
| `PATCH /v1/scheduled-transfers/{scheduleId}` | FR-044 | Body `{"status": "PAUSED"}` hoặc `{"status": "ACTIVE"}` |

Giao diện ngoài:

| Giao diện | Thay đổi |
| --- | --- |
| Fraud Check (INT-01) | Không đổi contract; scheduler gọi thêm nên tải tăng, cần xác nhận lại ASM-001 |
| Sự kiện giao dịch (INT-02) | Thêm hai loại sự kiện `ScheduledRunCompleted` và `ScheduledRunFailed` trên cùng topic, trường tương thích ngược: `eventId`, `scheduleId`, `runId`, `fromAccountId`, `toAccountId`, `amount`, `correlationId`, thêm `transferId` (khi thành công) hoặc `reasonCode` (khi thất bại) |
| SMS Gateway (INT-03) | Không đổi contract; thêm hai mẫu nội dung tin nhắn |

## Luồng xử lý

&#91;embedded content: luồng tạo lịch và chạy lịch · 1 quyết định, 3 kết quả\]

Scheduler quét mỗi 30 giây, chiếm lần chạy đến hạn bằng một cập nhật có điều kiện (không chạy trùng, NFR-011) rồi gọi luồng chuyển tiền hiện có; lỗi tạm thời thì thử lại, lỗi vĩnh viễn hoặc hết lượt thử thì FAILED.

## Truy vết, giả định và câu hỏi mở

Mỗi yêu cầu mới đều dựa vào một yêu cầu của SRS gốc mà không sửa nó; sau khi duyệt CR-001, các mã này được thêm vào knowledge graph của TDD.

| Yêu cầu mới | Dựa vào (SRS gốc) | Quan hệ |
| --- | --- | --- |
| FR-040, FR-041 | FR-002, FR-011, BR-04, BR-05 | Dùng xác thực; kiểm tra số tiền tối đa lúc tạo |
| FR-042 | FR-013 | Tương tự tra cứu giao dịch; giao dịch sinh ra hiển thị trong lịch sử |
| FR-045 | FR-010, FR-011, FR-012, FR-020 | Gọi lại toàn bộ luồng chuyển tiền |
| FR-047 | FR-021 | Mở rộng sự kiện và thông báo |
| FR-048 | FR-030 | Mở rộng audit |
| NFR-010 đến NFR-014 | NFR-001, NFR-002, NFR-005, NFR-006 | Không được làm xấu các chỉ số gốc |

Giả định và câu hỏi mở:

| Mã | Nội dung | Loại | Chủ sở hữu | Hạn xác nhận |
| --- | --- | --- | --- | --- |
| ASM-010 | Chu kỳ quét 30 giây đủ cho NFR-010 | Giả định | Tech Lead | Trước khi duyệt CR-001 |
| ASM-011 | Fraud Check chịu được tải thêm từ scheduler (liên quan ASM-001) | Giả định | Tech Lead | Trước khi duyệt CR-001 |
| OQ-010 | Quy tắc thử lại khi thiếu số dư: thử lại hay thất bại ngay, và thử bao nhiêu lần | Câu hỏi mở | Business Analyst | Trước khi duyệt CR-001 |
| OQ-011 | Giới hạn 20 lịch đang hoạt động mỗi tài khoản có phù hợp nghiệp vụ | Câu hỏi mở | Business Analyst | Trước khi duyệt CR-001 |
| OQ-012 | Thời hạn chạy bù khi hệ thống ngừng: 24 giờ hay khác | Câu hỏi mở | Business Analyst | Trước khi duyệt CR-001 |

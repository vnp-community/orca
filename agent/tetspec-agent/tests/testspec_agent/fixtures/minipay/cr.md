# Change Request CR-001: Chuyển tiền hẹn lịch và định kỳ cho MiniPay

Oct 7, 2026 · @Dat Nguyen Tien

CR-001 đề xuất thêm tính năng chuyển tiền hẹn lịch và định kỳ vào MiniPay; tính năng chưa được triển khai và đang chờ duyệt.

## Thông tin change request

| Mục | Nội dung |
| --- | --- |
| Mã CR | CR-001 |
| Tên | Chuyển tiền hẹn lịch và định kỳ |
| Loại thay đổi | Tính năng mới (không sửa hành vi của các tính năng đã baseline) |
| Mức ưu tiên đề xuất | Trung bình |
| Hệ thống | MiniPay, phiên bản hiện hành theo SRS và SAD/HLD đã baseline |
| Trạng thái | Chờ phân tích và phê duyệt |
| Chủ sở hữu yêu cầu | Business Analyst |
| Tài liệu đi kèm | SRS bổ sung: Chuyển tiền hẹn lịch và định kỳ (mã yêu cầu FR-040 đến FR-049, NFR-010 đến NFR-014) |
| Tài liệu chịu ảnh hưởng | SRS, SAD/HLD, TDD, LLD, bộ test, knowledge graph của TDD |

## Lý do và mục tiêu thay đổi

Hiện MiniPay chỉ chuyển tiền ngay lúc gọi API, nên người dùng phải tự gọi lại mỗi lần cho các khoản trả định kỳ như tiền nhà, học phí, trả góp. Mục tiêu của CR-001 là cho phép khách hàng đặt lịch một lần hoặc lặp lại, và hệ thống tự chuyển tiền đúng hạn.

- Vấn đề hiện tại: khách hàng phải nhớ và gọi API thủ công đúng ngày; quên thì trễ khoản trả.
- Lợi ích mong đợi: giảm số lần gọi thủ công cho các khoản trả lặp lại, và có thông báo rõ khi một lần chuyển thất bại.
- Mục tiêu kỹ thuật: tái sử dụng nguyên luồng chuyển tiền hiện có (idempotency, hạn mức, Fraud Check, sổ cái, outbox, audit), không tạo đường ghi sổ thứ hai.
- Tiêu chí thành công đề xuất: 99% lần chạy đến hạn bắt đầu trong 60 giây (NFR-010); không có lần chạy nào tạo hai giao dịch (NFR-011).

## Phạm vi thay đổi

CR-001 thêm một module lập lịch và hai bảng dữ liệu mới, còn luồng chuyển tiền hiện có giữ nguyên.

| Trong phạm vi | Ngoài phạm vi |
| --- | --- |
| Tạo lịch chuyển tiền một lần và định kỳ (hàng ngày, hàng tuần, hàng tháng) | Chuyển tiền đa đồng tiền, quy đổi tỷ giá |
| Tra cứu, hủy, tạm dừng và tiếp tục lịch | Giữ chỗ (hold) số dư khi tạo lịch |
| Thực thi lần chạy đến hạn qua luồng chuyển tiền FR-010 đến FR-012 và FR-020 | Sửa lịch đã tạo (người dùng hủy rồi tạo lịch mới) |
| Thử lại, đánh dấu thất bại, tự tạm dừng lịch định kỳ | Giao diện web hoặc đi động (vẫn chỉ có API) |
| Chạy bù khi hệ thống ngừng hoạt động | Thu tiền từ nguồn bên ngoài (direct debit) |
| Thông báo SMS kết quả mỗi lần chạy, audit |  |

## Phân tích tác động

Tác động lớn nhất nằm ở kiến trúc (module lập lịch, khóa chống chạy trùng) và tài liệu; các hệ thống ngoài không đổi contract. Các khảo sát bên dưới theo quy định baseline: tài liệu đã Baselined không sửa tại chỗ mà tạo bản mới.

| Thành phần | Thay đổi cần làm | Mức độ |
| --- | --- | --- |
| SRS gốc | Thêm tham chiếu tới SRS bổ sung; FR-030 (audit) và FR-021 (thông báo) được SRS bổ sung mở rộng, không sửa nội dung gốc | Thấp |
| SAD/HLD | Thêm MOD-Scheduler, thực thể ScheduledTransfer và ScheduledRun, quyết định AD-05 (scheduler polling trong tiến trình, không dùng delayed queue), cập nhật sơ đồ kiến trúc | Cao |
| TDD | Thêm Scenario SCN-ScheduledRun và ngân sách thời gian; thêm loại sự kiện ScheduledRunFailed vào contract INT-02; chốt công nghệ lập lịch, khóa chống chạy trùng, chu kỳ quét, số lần thử lại | Cao |
| LLD và mã nguồn | Module scheduler, hai bảng `scheduled_transfers` và `scheduled_runs`, 6 endpoint mới, hàm tách lõi chuyển tiền để scheduler gọi nội bộ | Cao |
| Module hiện có | MOD-Transfer: tách lõi xử lý để gọi không qua API key; MOD-Notify: thêm mẫu SMS cho lần chạy; MOD-Audit: thêm loại sự kiện | Trung bình |
| Dữ liệu | Hai bảng mới; không đổi bảng hiện có (giao dịch sinh ra vẫn nằm trong `transfers`) | Trung bình |
| Tích hợp ngoài | INT-01 (Fraud Check) được gọi thêm từ scheduler, cần xác nhận tải; INT-02 thêm một loại sự kiện, schema tương thích ngược; INT-03 không đổi | Trung bình |
| Test | Test nghiệp vụ cho từng FR mới, test chạy trùng, test chạy bù, benchmark tải scheduler | Cao |
| Knowledge graph | Thêm nút Requirement, Module, Scenario, Decision, Mechanism, DataEntity, Risk, TestCase, DesignItem; chạy lại R1 đến R17; các nút Baselined cũ không được sửa (R17) | Trung bình |

## Rủi ro của thay đổi

Rủi ro cao nhất là chuyển tiền trùng khi có nhiều tiến trình scheduler; các rủi ro còn lại xử lý được bằng cơ chế sẵn có.

| Mã | Rủi ro | Khả năng | Tác động | Giảm thiểu |
| --- | --- | --- | --- | --- |
| CR1-R1 | Một lần chạy tạo hai giao dịch (hai scheduler, hoặc chạy lại sau lỗi) | Trung bình | Cao | Khóa theo lần chạy bằng cập nhật có điều kiện trên `scheduled_runs`; Idempotency-Key cố định = mã lịch + thời điểm dự kiến |
| CR1-R2 | Đến giờ cao điểm (nhiều lịch cùng 00:00 hoặc 08:00) làm chậm giao dịch thường | Cao | Trung bình | Giới hạn số lần chạy mỗi chu kỳ quét; rate limit khi gọi Fraud Check (ASM-001 chưa xác nhận 50 request mỗi giây) |
| CR1-R3 | Người dùng hiểu nhầm lịch đảm bảo có tiền | Trung bình | Trung bình | Nêu rõ trong API và SMS: số dư chỉ được kiểm tra lúc chạy; thất bại được thông báo |
| CR1-R4 | SQLite chỉ có một luồng ghi, scheduler cạnh tranh với API | Cao | Thấp | Chỉ đúng với bản demo (RISK-004); môi trường thật dùng CSDL quan hệ |
| CR1-R5 | Quy tắc ngày cuối tháng và múi giờ gây sai ngày chuyển | Trung bình | Trung bình | Chốt quy tắc BR-12 và test các ngày 29 đến 31, năm nhuận |
| CR1-R6 | Phạm vi trượt (thêm hold số dư, sửa lịch) | Trung bình | Trung bình | Giữ hai mục này ngoài phạm vi, đưa vào CR riêng nếu cần |

## Phương án và đề xuất

Đề xuất chọn phương án A: scheduler quét bảng định kỳ trong cùng tiến trình, vì giữ được kiến trúc modular monolith (AP-01) và không thêm hạ tầng.

| Phương án | Ưu điểm | Nhược điểm | Kết luận |
| --- | --- | --- | --- |
| A. Quét bảng định kỳ trong tiến trình (mỗi 30 giây) | Đơn giản, dữ liệu lịch và giao dịch cùng CSDL, dễ test | Độ chính xác thời gian theo chu kỳ quét; cần khóa khi chạy nhiều instance | **Chọn** |
| B. Delayed message trên message queue | Chính xác theo thời điểm, tách biệt | Queue hiện chỉ được mô phỏng; khó hủy và truy vấn lịch; phụ thuộc hạ tầng | Loại |
| C. Cron của hệ điều hành gọi API nội bộ | Không sửa mã lõi | Mỗi lần chạy phải có API key; không kiểm soát được chạy trùng và chạy bù | Loại |
| D. Không làm | Không tốn công | Khách hàng tiếp tục gọi thủ công | Không chọn |

Thời gian chạy có sai số tối đa bằng chu kỳ quét cộng thời gian xử lý; với chu kỳ 30 giây vẫn nằm trong mục tiêu 60 giây của NFR-010.

## Kế hoạch, công sức và tiêu chí chấp nhận

Ước lượng ban đầu là 13 ngày công, theo thứ tự Waterfall: phê duyệt CR, cập nhật tài liệu và baseline TDD, rồi mới viết LLD và mã. Đây là ước lượng thô, cần Tech Lead xác nhận.

| Bước | Công việc | Ngày công ước lượng |
| --- | --- | --- |
| 1 | Phê duyệt CR-001 và SRS bổ sung | 1 |
| 2 | Cập nhật SAD/HLD (MOD-Scheduler, dữ liệu, AD-05) | 1 |
| 3 | Viết và baseline phần TDD cho scheduler, cập nhật contract INT-02 | 2 |
| 4 | Cập nhật knowledge graph, chạy lại R1 đến R17 | 1 |
| 5 | Viết LLD | 1 |
| 6 | Lập trình scheduler, bảng dữ liệu, API mới, tách lõi chuyển tiền | 4 |
| 7 | Test nghiệp vụ, chạy trùng, chạy bù, benchmark | 3 |
|  | Tổng | 13 |

Tiêu chí chấp nhận để đóng CR:

- Toàn bộ FR đánh dấu Must (FR-040 đến FR-043, FR-045, FR-046, FR-048) có test đạt.
- NFR-010 và NFR-011 có bằng chứng đo đạc: 99% lần chạy bắt đầu trong 60 giây; 0 giao dịch trùng trong test chạy song song hai scheduler.
- Test hồi quy của 36 test hiện có vẫn đạt, p95 chuyển tiền (NFR-001) không tăng khi scheduler chạy.
- Knowledge graph sau cập nhật không có vi phạm mức Chặn baseline.

## Phê duyệt và việc cần chốt

CR-001 cần bốn vai trò duyệt trước khi cập nhật SAD/HLD và TDD.

| Vai trò | Phạm vi duyệt | Quyết định |
| --- | --- | --- |
| Business Analyst | Phạm vi và yêu cầu trong SRS bổ sung | Chờ duyệt |
| Tech Lead | Kiến trúc, phương án A, ước lượng | Chờ duyệt |
| QA Lead | Tiêu chí chấp nhận, kế hoạch test | Chờ duyệt |
| Product Owner | Ưu tiên và thứ tự so với các việc khác | Chờ duyệt |

Các việc cần chốt trước khi duyệt (các câu hỏi mở chi tiết nằm trong SRS bổ sung):

- [ ] Quy tắc thử lại khi không đủ số dư (OQ-010).
- [ ] Giới hạn số lịch đang hoạt động mỗi tài khoản (OQ-011).
- [ ] Thời hạn chạy bù khi hệ thống ngừng, hiện đề xuất 24 giờ (OQ-012).

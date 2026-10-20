---
spec: TSD-SCHEDULED-TRANSFER-TestCases
feature: SCHEDULED-TRANSFER
docKind: TestCases
version: 1
status: Draft
supersedes: none
baseline: b-ec21ecfae083
changes: [CR-001]
contentHash: c5d3a84463dedfbd03ce30d11916bfc098a8178e464327508fe95640b9416c6f
---
# Test Cases: SCHEDULED-TRANSFER

## 0. Thông tin tài liệu

| Trường | Giá trị |
| --- | --- |
| id | TSD-SCHEDULED-TRANSFER-TestCases_v1 |
| Feature | SCHEDULED-TRANSFER |
| Phiên bản | 1 |
| Trạng thái | Draft |
| Baseline TDD | DOC-SRS-CR001 (b-ec21ecfae083) |
| CR của phiên bản này | CR-001 |

## 1. Quy ước

- id: `TC-L<mức>-<scope>-<đối tượng>-<nhóm>-<khóa>`; ưu tiên P0 đến P3; loại thay đổi new, modified, unchanged
- Bước theo Given, When, Then; kết quả mong đợi là assertion đọc máy được

## 2. Tiền điều kiện chung

- Môi trường, dữ liệu nền và stub theo Test Design (tài liệu 02)
- Bí mật chỉ tham chiếu theo tên biến: —. Không ghi giá trị bí mật vào tài liệu này

## 3. Test Cases theo mức

### 3.1 L1: Nghiệp vụ E2E

#### TC-L1-SCHEDULED-TRANSFER-SCN-ScheduledRun-JRN-BUDGET: Kiểm tra kịch bản SCN-ScheduledRun hoàn tất trong ngân sách thời gian cho phép.

| Trường | Giá trị |
| --- | --- |
| id | TC-L1-SCHEDULED-TRANSFER-SCN-ScheduledRun-JRN-BUDGET |
| Mục tiêu | Kiểm tra kịch bản SCN-ScheduledRun hoàn tất trong ngân sách thời gian cho phép. |
| Mức, loại, ưu tiên | L1 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-JRN · TCOV TCOV-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-JRN-BUDGET · nguồn FR-045, FR-046, FR-047, FR-048, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-JRN |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| responseBudgetMs | budget | 60000 | SCN-ScheduledRun hoàn tất trong 60000 ms |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống ở trạng thái ban đầu. | MOD-API, MOD-Scheduler, MOD-Transfer |  |
| 2 | when | Chạy luồng SCN-ScheduledRun và đo tổng thời gian thực hiện. | — |  |
| 3 | then | Xác nhận tổng thời gian không vượt quá ngân sách thời gian. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | latency | <= | 60000 | ms |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L1-SCHEDULED-TRANSFER-SCN-ScheduledRun-JRN-MAIN: Kiểm tra luồng chính của kịch bản SCN-ScheduledRun chạy trọn vẹn từ đầu đến cuối.

| Trường | Giá trị |
| --- | --- |
| id | TC-L1-SCHEDULED-TRANSFER-SCN-ScheduledRun-JRN-MAIN |
| Mục tiêu | Kiểm tra luồng chính của kịch bản SCN-ScheduledRun chạy trọn vẹn từ đầu đến cuối. |
| Mức, loại, ưu tiên | L1 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-JRN · TCOV TCOV-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-JRN-MAIN · nguồn FR-045, FR-046, FR-047, FR-048, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-JRN |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| SCN-ScheduledRun | main | 8 | Luồng chính SCN-ScheduledRun |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống và các Module tham gia ở trạng thái ban đầu, stub của hệ thống ngoài trả về thành công. | MOD-API, MOD-Scheduler, MOD-Transfer |  |
| 2 | when | Quét danh sách lần chạy theo chu kỳ 30 giây một lần. | — |  |
| 3 | when | Chọn tối đa 20 lần chạy đã đến hạn. | — |  |
| 4 | when | Chiếm lần chạy bằng câu lệnh UPDATE có điều kiện. | — |  |
| 5 | when | Sinh correlationId cho lần chạy. | — |  |
| 6 | when | Chuyển sang lõi chuyển tiền để xử lý idempotency, kiểm tra hạn mức/số dư, Fraud Check, ghi sổ và ghi outbox. | — |  |
| 7 | when | Cập nhật trạng thái lần chạy. | — |  |
| 8 | when | Xử lý thử lại hoặc chuyển sang trạng thái FAILED khi cần. | — |  |
| 9 | when | Ghi audit và phát sự kiện. | — |  |
| 10 | then | Xác nhận luồng đã hoàn tất và kiểm tra trạng thái cuối ở mọi Module tham gia. | MOD-API, MOD-Scheduler, MOD-Transfer |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | hoàn tất |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

### 3.2 L2: Nghiệp vụ

#### TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-200: Kiểm tra API DELETE /v1/scheduled-transfers/{scheduleId} trả về mã 200 khi hủy lịch chuyển tiền thành công.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-200 |
| Mục tiêu | Kiểm tra API DELETE /v1/scheduled-transfers/{scheduleId} trả về mã 200 khi hủy lịch chuyển tiền thành công. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API-RSP-Cancel-200 · nguồn FR-043, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Lịch được hủy thành công (lịch chuyển sang CANCELLED, lần chạy PENDING chuyển sang CANCELLED), hoặc lịch đã ở trạng thái CANCELLED từ trước và hệ thống trả lại kết quả cũ. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-cancel-200 | 200 | DELETE /v1/scheduled-transfers/{scheduleId} → 200: Hủy thành công (lịch CANCELLED, lần PENDING thành CANCELLED) hoặc lịch đã CANCELLED: trả kết quả cũ |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị điều kiện: hủy thành công (lịch CANCELLED, lần PENDING thành CANCELLED) hoặc lịch đã CANCELLED nên trả kết quả cũ. | EP-CancelSchedule |  |
| 2 | when | Gọi DELETE /v1/scheduled-transfers/{scheduleId} trong điều kiện nêu trên. | EP-CancelSchedule |  |
| 3 | then | Kiểm tra phản hồi trả về mã 200. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-401: Kiểm tra API DELETE /v1/scheduled-transfers/{scheduleId} trả về mã 401 UNAUTHORIZED khi xác thực không hợp lệ.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-401 |
| Mục tiêu | Kiểm tra API DELETE /v1/scheduled-transfers/{scheduleId} trả về mã 401 UNAUTHORIZED khi xác thực không hợp lệ. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API-RSP-Cancel-401 · nguồn FR-043, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Yêu cầu thiếu API key hoặc dùng API key sai. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-cancel-401 | 401 UNAUTHORIZED | DELETE /v1/scheduled-transfers/{scheduleId} → 401 UNAUTHORIZED: Thiếu hoặc sai API key |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị điều kiện: yêu cầu thiếu API key hoặc API key sai. | EP-CancelSchedule |  |
| 2 | when | Gọi DELETE /v1/scheduled-transfers/{scheduleId} trong điều kiện nêu trên. | EP-CancelSchedule |  |
| 3 | then | Kiểm tra phản hồi trả về mã 401 UNAUTHORIZED. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 401 |  |
| 2 | errorCode | equals | UNAUTHORIZED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-403: Kiểm tra API DELETE /v1/scheduled-transfers/{scheduleId} trả về mã 403 FORBIDDEN khi người gọi không có quyền hủy lịch.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-403 |
| Mục tiêu | Kiểm tra API DELETE /v1/scheduled-transfers/{scheduleId} trả về mã 403 FORBIDDEN khi người gọi không có quyền hủy lịch. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API-RSP-Cancel-403 · nguồn FR-043, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Người gọi API không phải chủ sở hữu của lịch. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-cancel-403 | 403 FORBIDDEN | DELETE /v1/scheduled-transfers/{scheduleId} → 403 FORBIDDEN: Người gọi không phải chủ lịch |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị điều kiện: người gọi không phải chủ của lịch. | EP-CancelSchedule |  |
| 2 | when | Gọi DELETE /v1/scheduled-transfers/{scheduleId} trong điều kiện nêu trên. | EP-CancelSchedule |  |
| 3 | then | Kiểm tra phản hồi trả về mã 403 FORBIDDEN. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 403 |  |
| 2 | errorCode | equals | FORBIDDEN |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-404: Kiểm tra API DELETE /v1/scheduled-transfers/{scheduleId} trả về mã 404 SCHEDULE_NOT_FOUND khi không tìm thấy lịch.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-404 |
| Mục tiêu | Kiểm tra API DELETE /v1/scheduled-transfers/{scheduleId} trả về mã 404 SCHEDULE_NOT_FOUND khi không tìm thấy lịch. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API-RSP-Cancel-404 · nguồn FR-043, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Không tồn tại lịch nào ứng với scheduleId được truyền vào. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-cancel-404 | 404 SCHEDULE_NOT_FOUND | DELETE /v1/scheduled-transfers/{scheduleId} → 404 SCHEDULE_NOT_FOUND: Không có lịch với scheduleId |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị điều kiện: không có lịch nào với scheduleId đã dùng. | EP-CancelSchedule |  |
| 2 | when | Gọi DELETE /v1/scheduled-transfers/{scheduleId} trong điều kiện nêu trên. | EP-CancelSchedule |  |
| 3 | then | Kiểm tra phản hồi trả về mã 404 SCHEDULE_NOT_FOUND. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 404 |  |
| 2 | errorCode | equals | SCHEDULE_NOT_FOUND |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-409: Kiểm tra API DELETE /v1/scheduled-transfers/{scheduleId} trả về mã 409 SCHEDULE_NOT_CANCELLABLE khi lịch không thể hủy.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-409 |
| Mục tiêu | Kiểm tra API DELETE /v1/scheduled-transfers/{scheduleId} trả về mã 409 SCHEDULE_NOT_CANCELLABLE khi lịch không thể hủy. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API-RSP-Cancel-409 · nguồn FR-043, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Lịch đã ở trạng thái COMPLETED hoặc đang có một lần chạy ở trạng thái RUNNING. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-cancel-409 | 409 SCHEDULE_NOT_CANCELLABLE | DELETE /v1/scheduled-transfers/{scheduleId} → 409 SCHEDULE_NOT_CANCELLABLE: Lịch đã COMPLETED hoặc có lần chạy đang RUNNING |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị điều kiện: lịch đã COMPLETED hoặc có lần chạy đang RUNNING. | EP-CancelSchedule |  |
| 2 | when | Gọi DELETE /v1/scheduled-transfers/{scheduleId} trong điều kiện nêu trên. | EP-CancelSchedule |  |
| 3 | then | Kiểm tra phản hồi trả về mã 409 SCHEDULE_NOT_CANCELLABLE. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 409 |  |
| 2 | errorCode | equals | SCHEDULE_NOT_CANCELLABLE |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-amount-ABOVE-MAX: Kiểm tra POST /v1/scheduled-transfers khi trường amount (body) lớn hơn giá trị lớn nhất cho phép (50000001).

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-amount-ABOVE-MAX |
| Mục tiêu | Kiểm tra POST /v1/scheduled-transfers khi trường amount (body) lớn hơn giá trị lớn nhất cho phép (50000001). |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-amount-ABOVE-MAX · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| amount (body) | fld-create-amount-above-max | 50000001 | POST /v1/scheduled-transfers: amount (body) lớn hơn lớn nhất (50000001) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu POST /v1/scheduled-transfers hợp lệ ở mọi trường trừ amount (body); các phụ thuộc bên ngoài đều trả về thành công. | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers với amount (body) = 50000001. | EP-CreateSchedule | amount (body)=50000001 |
| 3 | then | Xác nhận yêu cầu bị từ chối và không gây ra tác dụng phụ nào. | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 400 |  |
| 2 | errorCode | equals | INVALID_REQUEST |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-amount-BELOW-MIN: Kiểm tra POST /v1/scheduled-transfers khi trường amount (body) nhỏ hơn giá trị nhỏ nhất cho phép (0).

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-amount-BELOW-MIN |
| Mục tiêu | Kiểm tra POST /v1/scheduled-transfers khi trường amount (body) nhỏ hơn giá trị nhỏ nhất cho phép (0). |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-amount-BELOW-MIN · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| amount (body) | fld-create-amount-below-min | 0 | POST /v1/scheduled-transfers: amount (body) nhỏ hơn nhỏ nhất (0) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu POST /v1/scheduled-transfers hợp lệ ở mọi trường trừ amount (body); các phụ thuộc bên ngoài đều trả về thành công. | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers với amount (body) = 0. | EP-CreateSchedule | amount (body)=0 |
| 3 | then | Xác nhận yêu cầu bị từ chối và không gây ra tác dụng phụ nào. | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 400 |  |
| 2 | errorCode | equals | INVALID_REQUEST |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-amount-MAX: Kiểm tra POST /v1/scheduled-transfers khi trường amount (body) đúng bằng giá trị lớn nhất cho phép là 50000000.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-amount-MAX |
| Mục tiêu | Kiểm tra POST /v1/scheduled-transfers khi trường amount (body) đúng bằng giá trị lớn nhất cho phép là 50000000. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-amount-MAX · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| amount (body) | fld-create-amount-max | 50000000 | POST /v1/scheduled-transfers: amount (body) đúng giá trị lớn nhất 50000000 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu POST /v1/scheduled-transfers hợp lệ ở mọi trường trừ amount (body); các phụ thuộc bên ngoài đều trả về thành công. | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers với amount (body) = 50000000. | EP-CreateSchedule | amount (body)=50000000 |
| 3 | then | Xác nhận yêu cầu được chấp nhận. | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-amount-MIN: Kiểm tra POST /v1/scheduled-transfers khi trường amount (body) đúng bằng giá trị nhỏ nhất cho phép là 1.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-amount-MIN |
| Mục tiêu | Kiểm tra POST /v1/scheduled-transfers khi trường amount (body) đúng bằng giá trị nhỏ nhất cho phép là 1. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-amount-MIN · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| amount (body) | fld-create-amount-min | 1 | POST /v1/scheduled-transfers: amount (body) đúng giá trị nhỏ nhất 1 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu POST /v1/scheduled-transfers hợp lệ ở mọi trường trừ amount (body); các phụ thuộc bên ngoài đều trả về thành công. | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers với amount (body) = 1. | EP-CreateSchedule | amount (body)=1 |
| 3 | then | Xác nhận yêu cầu được chấp nhận. | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-maxRuns-ABOVE-MAX: Kiểm tra POST /v1/scheduled-transfers khi trường maxRuns (body) lớn hơn giá trị lớn nhất cho phép (61).

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-maxRuns-ABOVE-MAX |
| Mục tiêu | Kiểm tra POST /v1/scheduled-transfers khi trường maxRuns (body) lớn hơn giá trị lớn nhất cho phép (61). |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-maxRuns-ABOVE-MAX · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| maxRuns (body) | fld-create-maxruns-above-max | 61 | POST /v1/scheduled-transfers: maxRuns (body) lớn hơn lớn nhất (61) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu POST /v1/scheduled-transfers hợp lệ ở mọi trường trừ maxRuns (body); các phụ thuộc bên ngoài đều trả về thành công. | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers với maxRuns (body) = 61. | EP-CreateSchedule | maxRuns (body)=61 |
| 3 | then | Xác nhận yêu cầu bị từ chối và không gây ra tác dụng phụ nào. | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 400 |  |
| 2 | errorCode | equals | INVALID_REQUEST |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-maxRuns-BELOW-MIN: Kiểm tra POST /v1/scheduled-transfers khi trường maxRuns (body) nhỏ hơn giá trị nhỏ nhất cho phép (0).

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-maxRuns-BELOW-MIN |
| Mục tiêu | Kiểm tra POST /v1/scheduled-transfers khi trường maxRuns (body) nhỏ hơn giá trị nhỏ nhất cho phép (0). |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-maxRuns-BELOW-MIN · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| maxRuns (body) | fld-create-maxruns-below-min | 0 | POST /v1/scheduled-transfers: maxRuns (body) nhỏ hơn nhỏ nhất (0) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu POST /v1/scheduled-transfers hợp lệ ở mọi trường trừ maxRuns (body); các phụ thuộc bên ngoài đều trả về thành công. | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers với maxRuns (body) = 0. | EP-CreateSchedule | maxRuns (body)=0 |
| 3 | then | Xác nhận yêu cầu bị từ chối và không gây ra tác dụng phụ nào. | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 400 |  |
| 2 | errorCode | equals | INVALID_REQUEST |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-maxRuns-MAX: Kiểm tra POST /v1/scheduled-transfers khi trường maxRuns (body) đúng bằng giá trị lớn nhất cho phép là 60.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-maxRuns-MAX |
| Mục tiêu | Kiểm tra POST /v1/scheduled-transfers khi trường maxRuns (body) đúng bằng giá trị lớn nhất cho phép là 60. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-maxRuns-MAX · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| maxRuns (body) | fld-create-maxruns-max | 60 | POST /v1/scheduled-transfers: maxRuns (body) đúng giá trị lớn nhất 60 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu POST /v1/scheduled-transfers hợp lệ ở mọi trường trừ maxRuns (body); các phụ thuộc bên ngoài đều trả về thành công. | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers với maxRuns (body) = 60. | EP-CreateSchedule | maxRuns (body)=60 |
| 3 | then | Xác nhận yêu cầu được chấp nhận. | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-maxRuns-MIN: Kiểm tra POST /v1/scheduled-transfers khi trường maxRuns (body) đúng bằng giá trị nhỏ nhất cho phép là 1.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-maxRuns-MIN |
| Mục tiêu | Kiểm tra POST /v1/scheduled-transfers khi trường maxRuns (body) đúng bằng giá trị nhỏ nhất cho phép là 1. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-maxRuns-MIN · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| maxRuns (body) | fld-create-maxruns-min | 1 | POST /v1/scheduled-transfers: maxRuns (body) đúng giá trị nhỏ nhất 1 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu POST /v1/scheduled-transfers hợp lệ ở mọi trường trừ maxRuns (body); các phụ thuộc bên ngoài đều trả về thành công. | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers với maxRuns (body) = 1. | EP-CreateSchedule | maxRuns (body)=1 |
| 3 | then | Xác nhận yêu cầu được chấp nhận. | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-DAILY: Kiểm tra POST /v1/scheduled-transfers khi trường recurrence (body) = DAILY, là một giá trị hợp lệ.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-DAILY |
| Mục tiêu | Kiểm tra POST /v1/scheduled-transfers khi trường recurrence (body) = DAILY, là một giá trị hợp lệ. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-DAILY · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| recurrence (body) | fld-create-recurrence-enum-daily | DAILY | POST /v1/scheduled-transfers: recurrence (body) = DAILY (giá trị hợp lệ) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu POST /v1/scheduled-transfers hợp lệ ở mọi trường trừ recurrence (body); các phụ thuộc bên ngoài đều trả về thành công. | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers với recurrence (body) = DAILY. | EP-CreateSchedule | recurrence (body)=DAILY |
| 3 | then | Xác nhận yêu cầu được chấp nhận. | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-INVALID: Kiểm tra POST /v1/scheduled-transfers khi trường recurrence (body) có giá trị nằm ngoài danh sách ONCE, DAILY, WEEKLY, MONTHLY.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-INVALID |
| Mục tiêu | Kiểm tra POST /v1/scheduled-transfers khi trường recurrence (body) có giá trị nằm ngoài danh sách ONCE, DAILY, WEEKLY, MONTHLY. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-INVALID · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| recurrence (body) | fld-create-recurrence-enum-invalid | <ngoài danh sách> | POST /v1/scheduled-transfers: recurrence (body) ngoài danh sách ONCE, DAILY, WEEKLY, MONTHLY |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu POST /v1/scheduled-transfers hợp lệ ở mọi trường trừ recurrence (body); các phụ thuộc bên ngoài đều trả về thành công. | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers với recurrence (body) = (giá trị ngoài danh sách). | EP-CreateSchedule | recurrence (body)=(giá trị ngoài danh sách) |
| 3 | then | Xác nhận yêu cầu bị từ chối và không gây ra tác dụng phụ nào. | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 400 |  |
| 2 | errorCode | equals | INVALID_REQUEST |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-MONTHLY: Kiểm tra POST /v1/scheduled-transfers khi trường recurrence (body) = MONTHLY, là một giá trị hợp lệ.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-MONTHLY |
| Mục tiêu | Kiểm tra POST /v1/scheduled-transfers khi trường recurrence (body) = MONTHLY, là một giá trị hợp lệ. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-MONTHLY · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| recurrence (body) | fld-create-recurrence-enum-monthly | MONTHLY | POST /v1/scheduled-transfers: recurrence (body) = MONTHLY (giá trị hợp lệ) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu POST /v1/scheduled-transfers hợp lệ ở mọi trường trừ recurrence (body); các phụ thuộc bên ngoài đều trả về thành công. | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers với recurrence (body) = MONTHLY. | EP-CreateSchedule | recurrence (body)=MONTHLY |
| 3 | then | Xác nhận yêu cầu được chấp nhận. | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-ONCE: Kiểm tra POST /v1/scheduled-transfers khi trường recurrence (body) = ONCE, là một giá trị hợp lệ.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-ONCE |
| Mục tiêu | Kiểm tra POST /v1/scheduled-transfers khi trường recurrence (body) = ONCE, là một giá trị hợp lệ. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-ONCE · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| recurrence (body) | fld-create-recurrence-enum-once | ONCE | POST /v1/scheduled-transfers: recurrence (body) = ONCE (giá trị hợp lệ) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu POST /v1/scheduled-transfers hợp lệ ở mọi trường trừ recurrence (body); các phụ thuộc bên ngoài đều trả về thành công. | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers với recurrence (body) = ONCE. | EP-CreateSchedule | recurrence (body)=ONCE |
| 3 | then | Xác nhận yêu cầu được chấp nhận. | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-WEEKLY: Kiểm tra POST /v1/scheduled-transfers khi trường recurrence (body) = WEEKLY, là một giá trị hợp lệ.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-WEEKLY |
| Mục tiêu | Kiểm tra POST /v1/scheduled-transfers khi trường recurrence (body) = WEEKLY, là một giá trị hợp lệ. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-WEEKLY · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| recurrence (body) | fld-create-recurrence-enum-weekly | WEEKLY | POST /v1/scheduled-transfers: recurrence (body) = WEEKLY (giá trị hợp lệ) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu POST /v1/scheduled-transfers hợp lệ ở mọi trường trừ recurrence (body); các phụ thuộc bên ngoài đều trả về thành công. | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers với recurrence (body) = WEEKLY. | EP-CreateSchedule | recurrence (body)=WEEKLY |
| 3 | then | Xác nhận yêu cầu được chấp nhận. | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-200: Kiểm tra POST /v1/scheduled-transfers trả về mã 200.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-200 |
| Mục tiêu | Kiểm tra POST /v1/scheduled-transfers trả về mã 200. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-200 · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Gửi lại cùng Idempotency-Key với cùng nội dung yêu cầu: hệ thống trả về lịch đã tạo trước đó. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-create-200 | 200 | POST /v1/scheduled-transfers → 200: Cùng Idempotency-Key và cùng nội dung: trả lịch đã tạo |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Sử dụng cùng Idempotency-Key và cùng nội dung yêu cầu: hệ thống trả về lịch đã tạo. | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers trong điều kiện trên. | EP-CreateSchedule |  |
| 3 | then | Xác nhận phản hồi là 200. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-201: Kiểm tra POST /v1/scheduled-transfers trả về mã 201.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-201 |
| Mục tiêu | Kiểm tra POST /v1/scheduled-transfers trả về mã 201. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-201 · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Tạo lịch thành công với status=ACTIVE và hệ thống trả về mã lịch. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-create-201 | 201 | POST /v1/scheduled-transfers → 201: Tạo lịch thành công, status=ACTIVE, trả mã lịch |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch thành công với status=ACTIVE và nhận về mã lịch. | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers trong điều kiện trên. | EP-CreateSchedule |  |
| 3 | then | Xác nhận phản hồi là 201. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 201 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-400: Kiểm tra POST /v1/scheduled-transfers trả về mã 400 INVALID_REQUEST.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-400 |
| Mục tiêu | Kiểm tra POST /v1/scheduled-transfers trả về mã 400 INVALID_REQUEST. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-400 · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Yêu cầu rơi vào một trong các trường hợp: thiếu Idempotency-Key hoặc trường bắt buộc; amount không phải số nguyên dương; recurrence sai; lịch định kỳ thiếu cả endDate lẫn maxRuns; maxRuns nằm ngoài khoảng 1..60; firstRunAt sai định dạng. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-create-400 | 400 INVALID_REQUEST | POST /v1/scheduled-transfers → 400 INVALID_REQUEST: Thiếu Idempotency-Key hoặc trường bắt buộc, amount không nguyên dương, recurrence sai, định kỳ thiếu cả endDate và maxRuns, maxRuns ngoài 1..60, firstRunAt sai định dạng |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu rơi vào một trong các trường hợp: thiếu Idempotency-Key hoặc trường bắt buộc; amount không phải số nguyên dương; recurrence sai; lịch định kỳ thiếu cả endDate lẫn maxRuns; maxRuns nằm ngoài khoảng 1..60; firstRunAt sai định dạng. | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers trong điều kiện trên. | EP-CreateSchedule |  |
| 3 | then | Xác nhận phản hồi là 400 INVALID_REQUEST. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 400 |  |
| 2 | errorCode | equals | INVALID_REQUEST |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-401: Kiểm tra POST /v1/scheduled-transfers trả về mã 401 UNAUTHORIZED.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-401 |
| Mục tiêu | Kiểm tra POST /v1/scheduled-transfers trả về mã 401 UNAUTHORIZED. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-401 · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Yêu cầu thiếu API key hoặc dùng API key sai. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-create-401 | 401 UNAUTHORIZED | POST /v1/scheduled-transfers → 401 UNAUTHORIZED: Thiếu hoặc sai API key |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu thiếu API key hoặc dùng API key sai. | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers trong điều kiện trên. | EP-CreateSchedule |  |
| 3 | then | Xác nhận phản hồi là 401 UNAUTHORIZED. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 401 |  |
| 2 | errorCode | equals | UNAUTHORIZED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-403: Kiểm tra POST /v1/scheduled-transfers trả về mã 403 FORBIDDEN.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-403 |
| Mục tiêu | Kiểm tra POST /v1/scheduled-transfers trả về mã 403 FORBIDDEN. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-403 · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | fromAccountId khác với tài khoản của người gọi. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-create-403 | 403 FORBIDDEN | POST /v1/scheduled-transfers → 403 FORBIDDEN: fromAccountId khác tài khoản của người gọi |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu có fromAccountId khác với tài khoản của người gọi. | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers trong điều kiện trên. | EP-CreateSchedule |  |
| 3 | then | Xác nhận phản hồi là 403 FORBIDDEN. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 403 |  |
| 2 | errorCode | equals | FORBIDDEN |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-404: Kiểm tra POST /v1/scheduled-transfers trả về mã 404 ACCOUNT_NOT_FOUND.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-404 |
| Mục tiêu | Kiểm tra POST /v1/scheduled-transfers trả về mã 404 ACCOUNT_NOT_FOUND. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-404 · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Tài khoản đích không tồn tại. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-create-404 | 404 ACCOUNT_NOT_FOUND | POST /v1/scheduled-transfers → 404 ACCOUNT_NOT_FOUND: Tài khoản đích không tồn tại |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu trỏ tới một tài khoản đích không tồn tại. | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers trong điều kiện trên. | EP-CreateSchedule |  |
| 3 | then | Xác nhận phản hồi là 404 ACCOUNT_NOT_FOUND. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 404 |  |
| 2 | errorCode | equals | ACCOUNT_NOT_FOUND |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-409: Kiểm tra POST /v1/scheduled-transfers trả về mã 409 IDEMPOTENCY_CONFLICT.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-409 |
| Mục tiêu | Kiểm tra POST /v1/scheduled-transfers trả về mã 409 IDEMPOTENCY_CONFLICT. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-409 · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Dùng cùng Idempotency-Key nhưng nội dung yêu cầu khác với lần trước. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-create-409 | 409 IDEMPOTENCY_CONFLICT | POST /v1/scheduled-transfers → 409 IDEMPOTENCY_CONFLICT: Cùng Idempotency-Key nhưng nội dung khác |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Gửi yêu cầu với cùng Idempotency-Key nhưng nội dung khác với lần trước. | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers trong điều kiện trên. | EP-CreateSchedule |  |
| 3 | then | Xác nhận phản hồi là 409 IDEMPOTENCY_CONFLICT. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 409 |  |
| 2 | errorCode | equals | IDEMPOTENCY_CONFLICT |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-422-amount: Kiểm tra POST /v1/scheduled-transfers trả về mã 422 LIMIT_EXCEEDED.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-422-amount |
| Mục tiêu | Kiểm tra POST /v1/scheduled-transfers trả về mã 422 LIMIT_EXCEEDED. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-422-amount · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | amount lớn hơn 50.000.000 VND. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-create-422-amount | 422 LIMIT_EXCEEDED | POST /v1/scheduled-transfers → 422 LIMIT_EXCEEDED: amount > 50.000.000 VND |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu có amount lớn hơn 50.000.000 VND. | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers trong điều kiện trên. | EP-CreateSchedule |  |
| 3 | then | Xác nhận phản hồi là 422 LIMIT_EXCEEDED. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 422 |  |
| 2 | errorCode | equals | LIMIT_EXCEEDED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-422-limit: Kiểm tra POST /v1/scheduled-transfers trả về mã 422 SCHEDULE_LIMIT_REACHED.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-422-limit |
| Mục tiêu | Kiểm tra POST /v1/scheduled-transfers trả về mã 422 SCHEDULE_LIMIT_REACHED. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-422-limit · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Tài khoản đã có 20 lịch ở trạng thái ACTIVE hoặc PAUSED. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-create-422-limit | 422 SCHEDULE_LIMIT_REACHED | POST /v1/scheduled-transfers → 422 SCHEDULE_LIMIT_REACHED: Tài khoản đã có 20 lịch ACTIVE hoặc PAUSED |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Sử dụng tài khoản đã có 20 lịch ở trạng thái ACTIVE hoặc PAUSED. | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers trong điều kiện trên. | EP-CreateSchedule |  |
| 3 | then | Xác nhận phản hồi là 422 SCHEDULE_LIMIT_REACHED. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 422 |  |
| 2 | errorCode | equals | SCHEDULE_LIMIT_REACHED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-422-same: Kiểm tra POST /v1/scheduled-transfers trả về mã 422 SAME_ACCOUNT.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-422-same |
| Mục tiêu | Kiểm tra POST /v1/scheduled-transfers trả về mã 422 SAME_ACCOUNT. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-422-same · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | fromAccountId trùng với toAccountId. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-create-422-same | 422 SAME_ACCOUNT | POST /v1/scheduled-transfers → 422 SAME_ACCOUNT: fromAccountId bằng toAccountId |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu có fromAccountId trùng với toAccountId. | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers trong điều kiện trên. | EP-CreateSchedule |  |
| 3 | then | Xác nhận phản hồi là 422 SAME_ACCOUNT. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 422 |  |
| 2 | errorCode | equals | SAME_ACCOUNT |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-422-time: Kiểm tra POST /v1/scheduled-transfers trả về mã 422 SCHEDULE_TIME_INVALID.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-422-time |
| Mục tiêu | Kiểm tra POST /v1/scheduled-transfers trả về mã 422 SCHEDULE_TIME_INVALID. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-422-time · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Thời gian không hợp lệ: firstRunAt cách thời điểm hiện tại dưới 5 phút hoặc quá 90 ngày; hoặc endDate nằm trước lần chạy đầu tiên hoặc vượt quá 366 ngày. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-create-422-time | 422 SCHEDULE_TIME_INVALID | POST /v1/scheduled-transfers → 422 SCHEDULE_TIME_INVALID: firstRunAt cách hiện tại < 5 phút hoặc > 90 ngày; endDate trước lần chạy đầu hoặc quá 366 ngày |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu có thời gian không hợp lệ: firstRunAt cách thời điểm hiện tại dưới 5 phút hoặc quá 90 ngày; hoặc endDate nằm trước lần chạy đầu tiên hoặc vượt quá 366 ngày. | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers trong điều kiện trên. | EP-CreateSchedule |  |
| 3 | then | Xác nhận phản hồi là 422 SCHEDULE_TIME_INVALID. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 422 |  |
| 2 | errorCode | equals | SCHEDULE_TIME_INVALID |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-GetSchedule-API-RSP-Get-200: Kiểm tra GET /v1/scheduled-transfers/{scheduleId} trả về mã 200 cùng trạng thái lịch và lần chạy kế tiếp (nextRunAt).

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-GetSchedule-API-RSP-Get-200 |
| Mục tiêu | Kiểm tra GET /v1/scheduled-transfers/{scheduleId} trả về mã 200 cùng trạng thái lịch và lần chạy kế tiếp (nextRunAt). |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-GetSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API-RSP-Get-200 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Lịch chuyển tiền tồn tại và hệ thống có thể trả về trạng thái lịch cùng lần chạy kế tiếp (nextRunAt). |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-get-200 | 200 | GET /v1/scheduled-transfers/{scheduleId} → 200: Trả trạng thái lịch và lần chạy kế tiếp (nextRunAt) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị điều kiện: hệ thống trả về trạng thái lịch và lần chạy kế tiếp (nextRunAt). | EP-GetSchedule |  |
| 2 | when | Gọi GET /v1/scheduled-transfers/{scheduleId} trong điều kiện trên. | EP-GetSchedule |  |
| 3 | then | Xác nhận phản hồi trả về mã 200. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-GetSchedule-API-RSP-Get-401: Kiểm tra GET /v1/scheduled-transfers/{scheduleId} trả về mã 401 UNAUTHORIZED khi thiếu hoặc sai API key.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-GetSchedule-API-RSP-Get-401 |
| Mục tiêu | Kiểm tra GET /v1/scheduled-transfers/{scheduleId} trả về mã 401 UNAUTHORIZED khi thiếu hoặc sai API key. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-GetSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API-RSP-Get-401 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Yêu cầu không có API key hoặc dùng API key sai. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-get-401 | 401 UNAUTHORIZED | GET /v1/scheduled-transfers/{scheduleId} → 401 UNAUTHORIZED: Thiếu hoặc sai API key |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị điều kiện: yêu cầu thiếu API key hoặc dùng API key sai. | EP-GetSchedule |  |
| 2 | when | Gọi GET /v1/scheduled-transfers/{scheduleId} trong điều kiện trên. | EP-GetSchedule |  |
| 3 | then | Xác nhận phản hồi trả về mã 401 UNAUTHORIZED. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 401 |  |
| 2 | errorCode | equals | UNAUTHORIZED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-GetSchedule-API-RSP-Get-403: Kiểm tra GET /v1/scheduled-transfers/{scheduleId} trả về mã 403 FORBIDDEN khi người gọi không phải chủ tài khoản nguồn của lịch.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-GetSchedule-API-RSP-Get-403 |
| Mục tiêu | Kiểm tra GET /v1/scheduled-transfers/{scheduleId} trả về mã 403 FORBIDDEN khi người gọi không phải chủ tài khoản nguồn của lịch. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-GetSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API-RSP-Get-403 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Người gọi không phải là chủ tài khoản nguồn của lịch. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-get-403 | 403 FORBIDDEN | GET /v1/scheduled-transfers/{scheduleId} → 403 FORBIDDEN: Người gọi không phải chủ tài khoản nguồn của lịch |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị điều kiện: người gọi không phải là chủ tài khoản nguồn của lịch. | EP-GetSchedule |  |
| 2 | when | Gọi GET /v1/scheduled-transfers/{scheduleId} trong điều kiện trên. | EP-GetSchedule |  |
| 3 | then | Xác nhận phản hồi trả về mã 403 FORBIDDEN. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 403 |  |
| 2 | errorCode | equals | FORBIDDEN |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-GetSchedule-API-RSP-Get-404: Kiểm tra GET /v1/scheduled-transfers/{scheduleId} trả về mã 404 SCHEDULE_NOT_FOUND khi không tồn tại lịch ứng với scheduleId.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-GetSchedule-API-RSP-Get-404 |
| Mục tiêu | Kiểm tra GET /v1/scheduled-transfers/{scheduleId} trả về mã 404 SCHEDULE_NOT_FOUND khi không tồn tại lịch ứng với scheduleId. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-GetSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API-RSP-Get-404 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Không có lịch nào ứng với scheduleId được truyền vào. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-get-404 | 404 SCHEDULE_NOT_FOUND | GET /v1/scheduled-transfers/{scheduleId} → 404 SCHEDULE_NOT_FOUND: Không có lịch với scheduleId |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị điều kiện: không có lịch nào ứng với scheduleId. | EP-GetSchedule |  |
| 2 | when | Gọi GET /v1/scheduled-transfers/{scheduleId} trong điều kiện trên. | EP-GetSchedule |  |
| 3 | then | Xác nhận phản hồi trả về mã 404 SCHEDULE_NOT_FOUND. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 404 |  |
| 2 | errorCode | equals | SCHEDULE_NOT_FOUND |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-limit-ABOVE-MAX: Kiểm tra GET /v1/scheduled-transfers/{scheduleId}/runs khi tham số truy vấn limit lớn hơn giá trị lớn nhất cho phép (101).

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-limit-ABOVE-MAX |
| Mục tiêu | Kiểm tra GET /v1/scheduled-transfers/{scheduleId}/runs khi tham số truy vấn limit lớn hơn giá trị lớn nhất cho phép (101). |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListRuns · TD TD-SCHEDULED-TRANSFER-L2-EP-ListRuns-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-FLD-ListRuns-limit-ABOVE-MAX · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListRuns-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| limit (query) | fld-listruns-limit-above-max | 101 | GET /v1/scheduled-transfers/{scheduleId}/runs: limit (query) lớn hơn lớn nhất (101) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị một yêu cầu hợp lệ cho GET /v1/scheduled-transfers/{scheduleId}/runs, chỉ riêng trường limit (query) được xét riêng; các phụ thuộc bên ngoài đều trả về thành công. | EP-ListRuns |  |
| 2 | when | Gọi GET /v1/scheduled-transfers/{scheduleId}/runs với limit (query) = 101. | EP-ListRuns | limit (query)=101 |
| 3 | then | Hệ thống từ chối yêu cầu và không gây ra tác dụng phụ nào. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 4xx |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-limit-BELOW-MIN: Kiểm tra GET /v1/scheduled-transfers/{scheduleId}/runs khi tham số truy vấn limit nhỏ hơn giá trị nhỏ nhất cho phép (0).

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-limit-BELOW-MIN |
| Mục tiêu | Kiểm tra GET /v1/scheduled-transfers/{scheduleId}/runs khi tham số truy vấn limit nhỏ hơn giá trị nhỏ nhất cho phép (0). |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListRuns · TD TD-SCHEDULED-TRANSFER-L2-EP-ListRuns-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-FLD-ListRuns-limit-BELOW-MIN · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListRuns-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| limit (query) | fld-listruns-limit-below-min | 0 | GET /v1/scheduled-transfers/{scheduleId}/runs: limit (query) nhỏ hơn nhỏ nhất (0) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị một yêu cầu hợp lệ cho GET /v1/scheduled-transfers/{scheduleId}/runs, chỉ riêng trường limit (query) được xét riêng; các phụ thuộc bên ngoài đều trả về thành công. | EP-ListRuns |  |
| 2 | when | Gọi GET /v1/scheduled-transfers/{scheduleId}/runs với limit (query) = 0. | EP-ListRuns | limit (query)=0 |
| 3 | then | Hệ thống từ chối yêu cầu và không gây ra tác dụng phụ nào. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 4xx |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-limit-MAX: Kiểm tra GET /v1/scheduled-transfers/{scheduleId}/runs khi tham số truy vấn limit nhận đúng giá trị lớn nhất là 100.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-limit-MAX |
| Mục tiêu | Kiểm tra GET /v1/scheduled-transfers/{scheduleId}/runs khi tham số truy vấn limit nhận đúng giá trị lớn nhất là 100. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListRuns · TD TD-SCHEDULED-TRANSFER-L2-EP-ListRuns-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-FLD-ListRuns-limit-MAX · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListRuns-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| limit (query) | fld-listruns-limit-max | 100 | GET /v1/scheduled-transfers/{scheduleId}/runs: limit (query) đúng giá trị lớn nhất 100 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị một yêu cầu hợp lệ cho GET /v1/scheduled-transfers/{scheduleId}/runs, chỉ riêng trường limit (query) được xét riêng; các phụ thuộc bên ngoài đều trả về thành công. | EP-ListRuns |  |
| 2 | when | Gọi GET /v1/scheduled-transfers/{scheduleId}/runs với limit (query) = 100. | EP-ListRuns | limit (query)=100 |
| 3 | then | Hệ thống chấp nhận yêu cầu. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-limit-MIN: Kiểm tra GET /v1/scheduled-transfers/{scheduleId}/runs khi tham số truy vấn limit nhận đúng giá trị nhỏ nhất là 1.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-limit-MIN |
| Mục tiêu | Kiểm tra GET /v1/scheduled-transfers/{scheduleId}/runs khi tham số truy vấn limit nhận đúng giá trị nhỏ nhất là 1. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListRuns · TD TD-SCHEDULED-TRANSFER-L2-EP-ListRuns-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-FLD-ListRuns-limit-MIN · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListRuns-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| limit (query) | fld-listruns-limit-min | 1 | GET /v1/scheduled-transfers/{scheduleId}/runs: limit (query) đúng giá trị nhỏ nhất 1 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị một yêu cầu hợp lệ cho GET /v1/scheduled-transfers/{scheduleId}/runs, chỉ riêng trường limit (query) được xét riêng; các phụ thuộc bên ngoài đều trả về thành công. | EP-ListRuns |  |
| 2 | when | Gọi GET /v1/scheduled-transfers/{scheduleId}/runs với limit (query) = 1. | EP-ListRuns | limit (query)=1 |
| 3 | then | Hệ thống chấp nhận yêu cầu. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-offset-BELOW-MIN: Kiểm tra GET /v1/scheduled-transfers/{scheduleId}/runs khi tham số truy vấn offset nhỏ hơn giá trị nhỏ nhất cho phép (-1).

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-offset-BELOW-MIN |
| Mục tiêu | Kiểm tra GET /v1/scheduled-transfers/{scheduleId}/runs khi tham số truy vấn offset nhỏ hơn giá trị nhỏ nhất cho phép (-1). |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListRuns · TD TD-SCHEDULED-TRANSFER-L2-EP-ListRuns-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-FLD-ListRuns-offset-BELOW-MIN · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListRuns-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| offset (query) | fld-listruns-offset-below-min | -1 | GET /v1/scheduled-transfers/{scheduleId}/runs: offset (query) nhỏ hơn nhỏ nhất (-1) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị một yêu cầu hợp lệ cho GET /v1/scheduled-transfers/{scheduleId}/runs, chỉ riêng trường offset (query) được xét riêng; các phụ thuộc bên ngoài đều trả về thành công. | EP-ListRuns |  |
| 2 | when | Gọi GET /v1/scheduled-transfers/{scheduleId}/runs với offset (query) = -1. | EP-ListRuns | offset (query)=-1 |
| 3 | then | Hệ thống từ chối yêu cầu và không gây ra tác dụng phụ nào. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 4xx |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-offset-MIN: Kiểm tra GET /v1/scheduled-transfers/{scheduleId}/runs khi tham số truy vấn offset nhận đúng giá trị nhỏ nhất là 0.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-offset-MIN |
| Mục tiêu | Kiểm tra GET /v1/scheduled-transfers/{scheduleId}/runs khi tham số truy vấn offset nhận đúng giá trị nhỏ nhất là 0. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListRuns · TD TD-SCHEDULED-TRANSFER-L2-EP-ListRuns-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-FLD-ListRuns-offset-MIN · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListRuns-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| offset (query) | fld-listruns-offset-min | 0 | GET /v1/scheduled-transfers/{scheduleId}/runs: offset (query) đúng giá trị nhỏ nhất 0 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị một yêu cầu hợp lệ cho GET /v1/scheduled-transfers/{scheduleId}/runs, chỉ riêng trường offset (query) được xét riêng; các phụ thuộc bên ngoài đều trả về thành công. | EP-ListRuns |  |
| 2 | when | Gọi GET /v1/scheduled-transfers/{scheduleId}/runs với offset (query) = 0. | EP-ListRuns | offset (query)=0 |
| 3 | then | Hệ thống chấp nhận yêu cầu. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-RSP-ListRuns-200: Kiểm tra GET /v1/scheduled-transfers/{scheduleId}/runs trả về phản hồi 200.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-RSP-ListRuns-200 |
| Mục tiêu | Kiểm tra GET /v1/scheduled-transfers/{scheduleId}/runs trả về phản hồi 200. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListRuns · TD TD-SCHEDULED-TRANSFER-L2-EP-ListRuns-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-RSP-ListRuns-200 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Hệ thống trả về các lần chạy, sắp xếp mới nhất trước; mỗi lần chạy gồm thời điểm dự kiến, trạng thái, transferId và reasonCode. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListRuns-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-listruns-200 | 200 | GET /v1/scheduled-transfers/{scheduleId}/runs → 200: Trả các lần chạy, mới nhất trước, mỗi lần có thời điểm dự kiến, trạng thái, transferId, reasonCode |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống trả về các lần chạy, sắp xếp mới nhất trước; mỗi lần chạy gồm thời điểm dự kiến, trạng thái, transferId và reasonCode. | EP-ListRuns |  |
| 2 | when | Gọi GET /v1/scheduled-transfers/{scheduleId}/runs trong điều kiện trên. | EP-ListRuns |  |
| 3 | then | Hệ thống phản hồi 200. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-RSP-ListRuns-401: Kiểm tra GET /v1/scheduled-transfers/{scheduleId}/runs trả về 401 UNAUTHORIZED.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-RSP-ListRuns-401 |
| Mục tiêu | Kiểm tra GET /v1/scheduled-transfers/{scheduleId}/runs trả về 401 UNAUTHORIZED. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListRuns · TD TD-SCHEDULED-TRANSFER-L2-EP-ListRuns-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-RSP-ListRuns-401 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | API key bị thiếu hoặc không đúng. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListRuns-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-listruns-401 | 401 UNAUTHORIZED | GET /v1/scheduled-transfers/{scheduleId}/runs → 401 UNAUTHORIZED: Thiếu hoặc sai API key |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | API key bị thiếu hoặc không đúng. | EP-ListRuns |  |
| 2 | when | Gọi GET /v1/scheduled-transfers/{scheduleId}/runs trong điều kiện trên. | EP-ListRuns |  |
| 3 | then | Hệ thống phản hồi 401 UNAUTHORIZED. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 401 |  |
| 2 | errorCode | equals | UNAUTHORIZED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-RSP-ListRuns-403: Kiểm tra GET /v1/scheduled-transfers/{scheduleId}/runs trả về 403 FORBIDDEN.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-RSP-ListRuns-403 |
| Mục tiêu | Kiểm tra GET /v1/scheduled-transfers/{scheduleId}/runs trả về 403 FORBIDDEN. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListRuns · TD TD-SCHEDULED-TRANSFER-L2-EP-ListRuns-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-RSP-ListRuns-403 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Người gọi không phải chủ sở hữu của lịch. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListRuns-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-listruns-403 | 403 FORBIDDEN | GET /v1/scheduled-transfers/{scheduleId}/runs → 403 FORBIDDEN: Người gọi không phải chủ lịch |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Người gọi không phải chủ sở hữu của lịch. | EP-ListRuns |  |
| 2 | when | Gọi GET /v1/scheduled-transfers/{scheduleId}/runs trong điều kiện trên. | EP-ListRuns |  |
| 3 | then | Hệ thống phản hồi 403 FORBIDDEN. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 403 |  |
| 2 | errorCode | equals | FORBIDDEN |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-RSP-ListRuns-404: Kiểm tra GET /v1/scheduled-transfers/{scheduleId}/runs trả về 404 SCHEDULE_NOT_FOUND.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-RSP-ListRuns-404 |
| Mục tiêu | Kiểm tra GET /v1/scheduled-transfers/{scheduleId}/runs trả về 404 SCHEDULE_NOT_FOUND. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListRuns · TD TD-SCHEDULED-TRANSFER-L2-EP-ListRuns-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-RSP-ListRuns-404 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Không tồn tại lịch nào ứng với scheduleId. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListRuns-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-listruns-404 | 404 SCHEDULE_NOT_FOUND | GET /v1/scheduled-transfers/{scheduleId}/runs → 404 SCHEDULE_NOT_FOUND: Không có lịch với scheduleId |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Không tồn tại lịch nào ứng với scheduleId. | EP-ListRuns |  |
| 2 | when | Gọi GET /v1/scheduled-transfers/{scheduleId}/runs trong điều kiện trên. | EP-ListRuns |  |
| 3 | then | Hệ thống phản hồi 404 SCHEDULE_NOT_FOUND. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 404 |  |
| 2 | errorCode | equals | SCHEDULE_NOT_FOUND |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-limit-ABOVE-MAX: Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query limit lớn hơn giá trị lớn nhất (101).

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-limit-ABOVE-MAX |
| Mục tiêu | Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query limit lớn hơn giá trị lớn nhất (101). |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules · TD TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-limit-ABOVE-MAX · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của test case. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| limit (query) | fld-listschedules-limit-above-max | 101 | GET /v1/accounts/{accountId}/scheduled-transfers: limit (query) lớn hơn lớn nhất (101) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu hợp lệ cho GET /v1/accounts/{accountId}/scheduled-transfers, chỉ trừ tham số query limit; các phụ thuộc bên ngoài đều trả về thành công. | EP-ListSchedules |  |
| 2 | when | Gọi GET /v1/accounts/{accountId}/scheduled-transfers với limit (query) = 101. | EP-ListSchedules | limit (query)=101 |
| 3 | then | Hệ thống từ chối yêu cầu và không gây ra tác dụng phụ nào. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 400 |  |
| 2 | errorCode | equals | INVALID_REQUEST |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-limit-BELOW-MIN: Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query limit nhỏ hơn giá trị nhỏ nhất (0).

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-limit-BELOW-MIN |
| Mục tiêu | Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query limit nhỏ hơn giá trị nhỏ nhất (0). |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules · TD TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-limit-BELOW-MIN · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của test case. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| limit (query) | fld-listschedules-limit-below-min | 0 | GET /v1/accounts/{accountId}/scheduled-transfers: limit (query) nhỏ hơn nhỏ nhất (0) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu hợp lệ cho GET /v1/accounts/{accountId}/scheduled-transfers, chỉ trừ tham số query limit; các phụ thuộc bên ngoài đều trả về thành công. | EP-ListSchedules |  |
| 2 | when | Gọi GET /v1/accounts/{accountId}/scheduled-transfers với limit (query) = 0. | EP-ListSchedules | limit (query)=0 |
| 3 | then | Hệ thống từ chối yêu cầu và không gây ra tác dụng phụ nào. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 400 |  |
| 2 | errorCode | equals | INVALID_REQUEST |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-limit-MAX: Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query limit bằng đúng giá trị lớn nhất là 100.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-limit-MAX |
| Mục tiêu | Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query limit bằng đúng giá trị lớn nhất là 100. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules · TD TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-limit-MAX · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của test case. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| limit (query) | fld-listschedules-limit-max | 100 | GET /v1/accounts/{accountId}/scheduled-transfers: limit (query) đúng giá trị lớn nhất 100 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu hợp lệ cho GET /v1/accounts/{accountId}/scheduled-transfers, chỉ trừ tham số query limit; các phụ thuộc bên ngoài đều trả về thành công. | EP-ListSchedules |  |
| 2 | when | Gọi GET /v1/accounts/{accountId}/scheduled-transfers với limit (query) = 100. | EP-ListSchedules | limit (query)=100 |
| 3 | then | Hệ thống chấp nhận yêu cầu. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-limit-MIN: Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query limit bằng đúng giá trị nhỏ nhất là 1.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-limit-MIN |
| Mục tiêu | Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query limit bằng đúng giá trị nhỏ nhất là 1. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules · TD TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-limit-MIN · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của test case. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| limit (query) | fld-listschedules-limit-min | 1 | GET /v1/accounts/{accountId}/scheduled-transfers: limit (query) đúng giá trị nhỏ nhất 1 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu hợp lệ cho GET /v1/accounts/{accountId}/scheduled-transfers, chỉ trừ tham số query limit; các phụ thuộc bên ngoài đều trả về thành công. | EP-ListSchedules |  |
| 2 | when | Gọi GET /v1/accounts/{accountId}/scheduled-transfers với limit (query) = 1. | EP-ListSchedules | limit (query)=1 |
| 3 | then | Hệ thống chấp nhận yêu cầu. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-offset-BELOW-MIN: Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query offset nhỏ hơn giá trị nhỏ nhất (-1).

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-offset-BELOW-MIN |
| Mục tiêu | Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query offset nhỏ hơn giá trị nhỏ nhất (-1). |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules · TD TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-offset-BELOW-MIN · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của test case. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| offset (query) | fld-listschedules-offset-below-min | -1 | GET /v1/accounts/{accountId}/scheduled-transfers: offset (query) nhỏ hơn nhỏ nhất (-1) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu hợp lệ cho GET /v1/accounts/{accountId}/scheduled-transfers, chỉ trừ tham số query offset; các phụ thuộc bên ngoài đều trả về thành công. | EP-ListSchedules |  |
| 2 | when | Gọi GET /v1/accounts/{accountId}/scheduled-transfers với offset (query) = -1. | EP-ListSchedules | offset (query)=-1 |
| 3 | then | Hệ thống từ chối yêu cầu và không gây ra tác dụng phụ nào. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 400 |  |
| 2 | errorCode | equals | INVALID_REQUEST |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-offset-MIN: Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query offset bằng đúng giá trị nhỏ nhất là 0.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-offset-MIN |
| Mục tiêu | Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query offset bằng đúng giá trị nhỏ nhất là 0. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules · TD TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-offset-MIN · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của test case. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| offset (query) | fld-listschedules-offset-min | 0 | GET /v1/accounts/{accountId}/scheduled-transfers: offset (query) đúng giá trị nhỏ nhất 0 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu hợp lệ cho GET /v1/accounts/{accountId}/scheduled-transfers, chỉ trừ tham số query offset; các phụ thuộc bên ngoài đều trả về thành công. | EP-ListSchedules |  |
| 2 | when | Gọi GET /v1/accounts/{accountId}/scheduled-transfers với offset (query) = 0. | EP-ListSchedules | offset (query)=0 |
| 3 | then | Hệ thống chấp nhận yêu cầu. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-ACTIVE: Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query status = ACTIVE (giá trị hợp lệ).

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-ACTIVE |
| Mục tiêu | Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query status = ACTIVE (giá trị hợp lệ). |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules · TD TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-ACTIVE · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của test case. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| status (query) | fld-listschedules-status-enum-active | ACTIVE | GET /v1/accounts/{accountId}/scheduled-transfers: status (query) = ACTIVE (giá trị hợp lệ) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu hợp lệ cho GET /v1/accounts/{accountId}/scheduled-transfers, chỉ trừ tham số query status; các phụ thuộc bên ngoài đều trả về thành công. | EP-ListSchedules |  |
| 2 | when | Gọi GET /v1/accounts/{accountId}/scheduled-transfers với status (query) = ACTIVE. | EP-ListSchedules | status (query)=ACTIVE |
| 3 | then | Hệ thống chấp nhận yêu cầu. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-CANCELLED: Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query status = CANCELLED (giá trị hợp lệ).

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-CANCELLED |
| Mục tiêu | Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query status = CANCELLED (giá trị hợp lệ). |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules · TD TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-CANCELLED · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của test case. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| status (query) | fld-listschedules-status-enum-cancelled | CANCELLED | GET /v1/accounts/{accountId}/scheduled-transfers: status (query) = CANCELLED (giá trị hợp lệ) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu hợp lệ cho GET /v1/accounts/{accountId}/scheduled-transfers, chỉ trừ tham số query status; các phụ thuộc bên ngoài đều trả về thành công. | EP-ListSchedules |  |
| 2 | when | Gọi GET /v1/accounts/{accountId}/scheduled-transfers với status (query) = CANCELLED. | EP-ListSchedules | status (query)=CANCELLED |
| 3 | then | Hệ thống chấp nhận yêu cầu. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-COMPLETED: Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query status = COMPLETED (giá trị hợp lệ).

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-COMPLETED |
| Mục tiêu | Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query status = COMPLETED (giá trị hợp lệ). |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules · TD TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-COMPLETED · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của test case. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| status (query) | fld-listschedules-status-enum-completed | COMPLETED | GET /v1/accounts/{accountId}/scheduled-transfers: status (query) = COMPLETED (giá trị hợp lệ) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu hợp lệ cho GET /v1/accounts/{accountId}/scheduled-transfers, chỉ trừ tham số query status; các phụ thuộc bên ngoài đều trả về thành công. | EP-ListSchedules |  |
| 2 | when | Gọi GET /v1/accounts/{accountId}/scheduled-transfers với status (query) = COMPLETED. | EP-ListSchedules | status (query)=COMPLETED |
| 3 | then | Hệ thống chấp nhận yêu cầu. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-INVALID: Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query status có giá trị nằm ngoài danh sách ACTIVE, PAUSED, COMPLETED, CANCELLED.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-INVALID |
| Mục tiêu | Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query status có giá trị nằm ngoài danh sách ACTIVE, PAUSED, COMPLETED, CANCELLED. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules · TD TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-INVALID · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của test case. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| status (query) | fld-listschedules-status-enum-invalid | <ngoài danh sách> | GET /v1/accounts/{accountId}/scheduled-transfers: status (query) ngoài danh sách ACTIVE, PAUSED, COMPLETED, CANCELLED |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu hợp lệ cho GET /v1/accounts/{accountId}/scheduled-transfers, chỉ trừ tham số query status; các phụ thuộc bên ngoài đều trả về thành công. | EP-ListSchedules |  |
| 2 | when | Gọi GET /v1/accounts/{accountId}/scheduled-transfers với status (query) = (giá trị ngoài danh sách). | EP-ListSchedules | status (query)=(giá trị ngoài danh sách) |
| 3 | then | Hệ thống từ chối yêu cầu và không gây ra tác dụng phụ nào. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 400 |  |
| 2 | errorCode | equals | INVALID_REQUEST |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-PAUSED: Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query status = PAUSED (giá trị hợp lệ).

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-PAUSED |
| Mục tiêu | Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query status = PAUSED (giá trị hợp lệ). |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules · TD TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-PAUSED · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của test case. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| status (query) | fld-listschedules-status-enum-paused | PAUSED | GET /v1/accounts/{accountId}/scheduled-transfers: status (query) = PAUSED (giá trị hợp lệ) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu hợp lệ cho GET /v1/accounts/{accountId}/scheduled-transfers, chỉ trừ tham số query status; các phụ thuộc bên ngoài đều trả về thành công. | EP-ListSchedules |  |
| 2 | when | Gọi GET /v1/accounts/{accountId}/scheduled-transfers với status (query) = PAUSED. | EP-ListSchedules | status (query)=PAUSED |
| 3 | then | Hệ thống chấp nhận yêu cầu. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-RSP-ListSchedules-200: Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers trả về phản hồi 200.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-RSP-ListSchedules-200 |
| Mục tiêu | Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers trả về phản hồi 200. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules · TD TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-RSP-ListSchedules-200 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | API trả về danh sách lịch chuyển tiền của tài khoản, có phân trang và có thể lọc theo status. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-listschedules-200 | 200 | GET /v1/accounts/{accountId}/scheduled-transfers → 200: Trả danh sách lịch của tài khoản, phân trang, lọc theo status |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | API trả về danh sách lịch chuyển tiền của tài khoản, có phân trang và có thể lọc theo status. | EP-ListSchedules |  |
| 2 | when | Gọi GET /v1/accounts/{accountId}/scheduled-transfers trong điều kiện trên. | EP-ListSchedules |  |
| 3 | then | Hệ thống phản hồi 200. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-RSP-ListSchedules-400: Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers trả về phản hồi 400 INVALID_REQUEST.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-RSP-ListSchedules-400 |
| Mục tiêu | Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers trả về phản hồi 400 INVALID_REQUEST. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules · TD TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-RSP-ListSchedules-400 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Giá trị status nằm ngoài enum cho phép. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-listschedules-400 | 400 INVALID_REQUEST | GET /v1/accounts/{accountId}/scheduled-transfers → 400 INVALID_REQUEST: status ngoài enum |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Giá trị status nằm ngoài enum cho phép. | EP-ListSchedules |  |
| 2 | when | Gọi GET /v1/accounts/{accountId}/scheduled-transfers trong điều kiện trên. | EP-ListSchedules |  |
| 3 | then | Hệ thống phản hồi 400 INVALID_REQUEST. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 400 |  |
| 2 | errorCode | equals | INVALID_REQUEST |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-RSP-ListSchedules-401: Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers trả về phản hồi 401 UNAUTHORIZED.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-RSP-ListSchedules-401 |
| Mục tiêu | Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers trả về phản hồi 401 UNAUTHORIZED. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules · TD TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-RSP-ListSchedules-401 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | API key bị thiếu hoặc không đúng. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-listschedules-401 | 401 UNAUTHORIZED | GET /v1/accounts/{accountId}/scheduled-transfers → 401 UNAUTHORIZED: Thiếu hoặc sai API key |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | API key bị thiếu hoặc không đúng. | EP-ListSchedules |  |
| 2 | when | Gọi GET /v1/accounts/{accountId}/scheduled-transfers trong điều kiện trên. | EP-ListSchedules |  |
| 3 | then | Hệ thống phản hồi 401 UNAUTHORIZED. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 401 |  |
| 2 | errorCode | equals | UNAUTHORIZED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-RSP-ListSchedules-403: Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers trả về phản hồi 403 FORBIDDEN.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-RSP-ListSchedules-403 |
| Mục tiêu | Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers trả về phản hồi 403 FORBIDDEN. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules · TD TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-RSP-ListSchedules-403 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | accountId không thuộc tài khoản của người gọi. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-listschedules-403 | 403 FORBIDDEN | GET /v1/accounts/{accountId}/scheduled-transfers → 403 FORBIDDEN: accountId khác tài khoản của người gọi |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | accountId không thuộc tài khoản của người gọi. | EP-ListSchedules |  |
| 2 | when | Gọi GET /v1/accounts/{accountId}/scheduled-transfers trong điều kiện trên. | EP-ListSchedules |  |
| 3 | then | Hệ thống phản hồi 403 FORBIDDEN. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 403 |  |
| 2 | errorCode | equals | FORBIDDEN |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-ACTIVE: Kiểm tra PATCH /v1/scheduled-transfers/{scheduleId} chấp nhận trường status (body) có giá trị hợp lệ ACTIVE.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-ACTIVE |
| Mục tiêu | Kiểm tra PATCH /v1/scheduled-transfers/{scheduleId} chấp nhận trường status (body) có giá trị hợp lệ ACTIVE. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus · TD TD-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-ACTIVE · nguồn FR-044, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| status (body) | fld-patch-status-enum-active | ACTIVE | PATCH /v1/scheduled-transfers/{scheduleId}: status (body) = ACTIVE (giá trị hợp lệ) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu PATCH /v1/scheduled-transfers/{scheduleId} hợp lệ ở mọi trường, trừ trường status (body); các phụ thuộc bên ngoài đều trả về thành công. | EP-UpdateScheduleStatus |  |
| 2 | when | Gọi PATCH /v1/scheduled-transfers/{scheduleId} với status (body) = ACTIVE. | EP-UpdateScheduleStatus | status (body)=ACTIVE |
| 3 | then | Xác nhận yêu cầu được chấp nhận. | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-INVALID: Kiểm tra PATCH /v1/scheduled-transfers/{scheduleId} từ chối trường status (body) có giá trị nằm ngoài danh sách cho phép (PAUSED, ACTIVE).

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-INVALID |
| Mục tiêu | Kiểm tra PATCH /v1/scheduled-transfers/{scheduleId} từ chối trường status (body) có giá trị nằm ngoài danh sách cho phép (PAUSED, ACTIVE). |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus · TD TD-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-INVALID · nguồn FR-044, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| status (body) | fld-patch-status-enum-invalid | <ngoài danh sách> | PATCH /v1/scheduled-transfers/{scheduleId}: status (body) ngoài danh sách PAUSED, ACTIVE |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu PATCH /v1/scheduled-transfers/{scheduleId} hợp lệ ở mọi trường, trừ trường status (body); các phụ thuộc bên ngoài đều trả về thành công. | EP-UpdateScheduleStatus |  |
| 2 | when | Gọi PATCH /v1/scheduled-transfers/{scheduleId} với status (body) là một giá trị ngoài danh sách cho phép. | EP-UpdateScheduleStatus | status (body)=(giá trị ngoài danh sách) |
| 3 | then | Xác nhận yêu cầu bị từ chối và không gây ra tác dụng phụ nào. | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 400 |  |
| 2 | errorCode | equals | INVALID_REQUEST |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-PAUSED: Kiểm tra PATCH /v1/scheduled-transfers/{scheduleId} chấp nhận trường status (body) có giá trị hợp lệ PAUSED.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-PAUSED |
| Mục tiêu | Kiểm tra PATCH /v1/scheduled-transfers/{scheduleId} chấp nhận trường status (body) có giá trị hợp lệ PAUSED. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus · TD TD-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-PAUSED · nguồn FR-044, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| status (body) | fld-patch-status-enum-paused | PAUSED | PATCH /v1/scheduled-transfers/{scheduleId}: status (body) = PAUSED (giá trị hợp lệ) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị yêu cầu PATCH /v1/scheduled-transfers/{scheduleId} hợp lệ ở mọi trường, trừ trường status (body); các phụ thuộc bên ngoài đều trả về thành công. | EP-UpdateScheduleStatus |  |
| 2 | when | Gọi PATCH /v1/scheduled-transfers/{scheduleId} với status (body) = PAUSED. | EP-UpdateScheduleStatus | status (body)=PAUSED |
| 3 | then | Xác nhận yêu cầu được chấp nhận. | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-200: Kiểm tra PATCH /v1/scheduled-transfers/{scheduleId} trả về mã 200 khi cập nhật trạng thái thành công.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-200 |
| Mục tiêu | Kiểm tra PATCH /v1/scheduled-transfers/{scheduleId} trả về mã 200 khi cập nhật trạng thái thành công. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus · TD TD-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-RSP-Patch-200 · nguồn FR-044, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Việc chuyển lịch sang PAUSED hoặc ACTIVE thực hiện thành công và hệ thống trả về thông tin lịch. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-patch-200 | 200 | PATCH /v1/scheduled-transfers/{scheduleId} → 200: Chuyển PAUSED hoặc ACTIVE thành công, trả lịch |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thiết lập điều kiện: chuyển lịch sang PAUSED hoặc ACTIVE thành công và trả về thông tin lịch. | EP-UpdateScheduleStatus |  |
| 2 | when | Gọi PATCH /v1/scheduled-transfers/{scheduleId} trong điều kiện trên. | EP-UpdateScheduleStatus |  |
| 3 | then | Xác nhận phản hồi là 200. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-400: Kiểm tra PATCH /v1/scheduled-transfers/{scheduleId} trả về mã 400 INVALID_REQUEST khi yêu cầu không hợp lệ.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-400 |
| Mục tiêu | Kiểm tra PATCH /v1/scheduled-transfers/{scheduleId} trả về mã 400 INVALID_REQUEST khi yêu cầu không hợp lệ. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus · TD TD-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-RSP-Patch-400 · nguồn FR-044, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Trường status bị thiếu hoặc có giá trị nằm ngoài PAUSED/ACTIVE. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-patch-400 | 400 INVALID_REQUEST | PATCH /v1/scheduled-transfers/{scheduleId} → 400 INVALID_REQUEST: status thiếu hoặc ngoài PAUSED/ACTIVE |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thiết lập điều kiện: trường status bị thiếu hoặc có giá trị ngoài PAUSED/ACTIVE. | EP-UpdateScheduleStatus |  |
| 2 | when | Gọi PATCH /v1/scheduled-transfers/{scheduleId} trong điều kiện trên. | EP-UpdateScheduleStatus |  |
| 3 | then | Xác nhận phản hồi là 400 INVALID_REQUEST. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 400 |  |
| 2 | errorCode | equals | INVALID_REQUEST |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-401: Kiểm tra PATCH /v1/scheduled-transfers/{scheduleId} trả về mã 401 UNAUTHORIZED khi xác thực thất bại.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-401 |
| Mục tiêu | Kiểm tra PATCH /v1/scheduled-transfers/{scheduleId} trả về mã 401 UNAUTHORIZED khi xác thực thất bại. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus · TD TD-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-RSP-Patch-401 · nguồn FR-044, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | API key bị thiếu hoặc không đúng. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-patch-401 | 401 UNAUTHORIZED | PATCH /v1/scheduled-transfers/{scheduleId} → 401 UNAUTHORIZED: Thiếu hoặc sai API key |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thiết lập điều kiện: thiếu API key hoặc dùng API key sai. | EP-UpdateScheduleStatus |  |
| 2 | when | Gọi PATCH /v1/scheduled-transfers/{scheduleId} trong điều kiện trên. | EP-UpdateScheduleStatus |  |
| 3 | then | Xác nhận phản hồi là 401 UNAUTHORIZED. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 401 |  |
| 2 | errorCode | equals | UNAUTHORIZED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-403: Kiểm tra PATCH /v1/scheduled-transfers/{scheduleId} trả về mã 403 FORBIDDEN khi người gọi không có quyền.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-403 |
| Mục tiêu | Kiểm tra PATCH /v1/scheduled-transfers/{scheduleId} trả về mã 403 FORBIDDEN khi người gọi không có quyền. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus · TD TD-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-RSP-Patch-403 · nguồn FR-044, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Người gọi không phải là chủ sở hữu của lịch. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-patch-403 | 403 FORBIDDEN | PATCH /v1/scheduled-transfers/{scheduleId} → 403 FORBIDDEN: Người gọi không phải chủ lịch |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thiết lập điều kiện: người gọi không phải chủ sở hữu của lịch. | EP-UpdateScheduleStatus |  |
| 2 | when | Gọi PATCH /v1/scheduled-transfers/{scheduleId} trong điều kiện trên. | EP-UpdateScheduleStatus |  |
| 3 | then | Xác nhận phản hồi là 403 FORBIDDEN. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 403 |  |
| 2 | errorCode | equals | FORBIDDEN |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-404: Kiểm tra PATCH /v1/scheduled-transfers/{scheduleId} trả về mã 404 SCHEDULE_NOT_FOUND khi không tìm thấy lịch.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-404 |
| Mục tiêu | Kiểm tra PATCH /v1/scheduled-transfers/{scheduleId} trả về mã 404 SCHEDULE_NOT_FOUND khi không tìm thấy lịch. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus · TD TD-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-RSP-Patch-404 · nguồn FR-044, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Không tồn tại lịch nào ứng với scheduleId. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-patch-404 | 404 SCHEDULE_NOT_FOUND | PATCH /v1/scheduled-transfers/{scheduleId} → 404 SCHEDULE_NOT_FOUND: Không có lịch với scheduleId |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thiết lập điều kiện: không có lịch nào ứng với scheduleId. | EP-UpdateScheduleStatus |  |
| 2 | when | Gọi PATCH /v1/scheduled-transfers/{scheduleId} trong điều kiện trên. | EP-UpdateScheduleStatus |  |
| 3 | then | Xác nhận phản hồi là 404 SCHEDULE_NOT_FOUND. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 404 |  |
| 2 | errorCode | equals | SCHEDULE_NOT_FOUND |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-409: Kiểm tra PATCH /v1/scheduled-transfers/{scheduleId} trả về mã 409 SCHEDULE_STATE_INVALID khi trạng thái lịch không cho phép thao tác.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-409 |
| Mục tiêu | Kiểm tra PATCH /v1/scheduled-transfers/{scheduleId} trả về mã 409 SCHEDULE_STATE_INVALID khi trạng thái lịch không cho phép thao tác. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus · TD TD-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-RSP-Patch-409 · nguồn FR-044, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Tạm dừng một lịch không ở trạng thái ACTIVE, tiếp tục một lịch không ở trạng thái PAUSED, hoặc thao tác trên lịch loại ONCE. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-patch-409 | 409 SCHEDULE_STATE_INVALID | PATCH /v1/scheduled-transfers/{scheduleId} → 409 SCHEDULE_STATE_INVALID: Tạm dừng lịch không ACTIVE, tiếp tục lịch không PAUSED, hoặc lịch ONCE |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thiết lập điều kiện: tạm dừng lịch không ở trạng thái ACTIVE, tiếp tục lịch không ở trạng thái PAUSED, hoặc thao tác trên lịch loại ONCE. | EP-UpdateScheduleStatus |  |
| 2 | when | Gọi PATCH /v1/scheduled-transfers/{scheduleId} trong điều kiện trên. | EP-UpdateScheduleStatus |  |
| 3 | then | Xác nhận phản hồi là 409 SCHEDULE_STATE_INVALID. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 409 |  |
| 2 | errorCode | equals | SCHEDULE_STATE_INVALID |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-1: Xác nhận FR-040 được đáp ứng: gọi POST tạo lịch chuyển tiền ONCE hợp lệ thì hệ thống trả 201, kèm scheduleId và status ACTIVE.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-1 |
| Mục tiêu | Xác nhận FR-040 được đáp ứng: gọi POST tạo lịch chuyển tiền ONCE hợp lệ thì hệ thống trả 201, kèm scheduleId và status ACTIVE. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-040 · TD TD-SCHEDULED-TRANSFER-L2-FR-040-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-040-AC-AC-FR-040-1 · nguồn FR-040 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo STUB của test case. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-040-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-040-1 | ac-fr-040-1 | POST tạo lịch ONCE hợp lệ trả 201, có scheduleId và status ACTIVE | Tiêu chí chấp nhận AC-FR-040-1 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống đang ở trạng thái ban đầu, dữ liệu nền và các stub phụ thuộc bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-040 theo tiêu chí: POST tạo lịch ONCE hợp lệ trả 201, có scheduleId và status ACTIVE. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả trả về và trạng thái dữ liệu. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | POST tạo lịch ONCE hợp lệ trả 201, có scheduleId và status ACTIVE |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-2: Xác nhận FR-040 được đáp ứng: sau khi tạo lịch, số dư của hai tài khoản không đổi và không phát sinh bản ghi mới trong ledger_entries.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-2 |
| Mục tiêu | Xác nhận FR-040 được đáp ứng: sau khi tạo lịch, số dư của hai tài khoản không đổi và không phát sinh bản ghi mới trong ledger_entries. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-040 · TD TD-SCHEDULED-TRANSFER-L2-FR-040-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-040-AC-AC-FR-040-2 · nguồn FR-040 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo STUB của test case. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-040-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-040-2 | ac-fr-040-2 | Số dư hai tài khoản không đổi và không có bản ghi ledger_entries mới sau khi tạo lịch | Tiêu chí chấp nhận AC-FR-040-2 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống đang ở trạng thái ban đầu, dữ liệu nền và các stub phụ thuộc bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-040 theo tiêu chí: số dư hai tài khoản không đổi và không có bản ghi ledger_entries mới sau khi tạo lịch. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả trả về và trạng thái dữ liệu. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Số dư hai tài khoản không đổi và không có bản ghi ledger_entries mới sau khi tạo lịch |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-3: Xác nhận FR-040 được đáp ứng: khi firstRunAt cách thời điểm hiện tại dưới 5 phút hoặc quá 90 ngày thì hệ thống trả 422 SCHEDULE_TIME_INVALID.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-3 |
| Mục tiêu | Xác nhận FR-040 được đáp ứng: khi firstRunAt cách thời điểm hiện tại dưới 5 phút hoặc quá 90 ngày thì hệ thống trả 422 SCHEDULE_TIME_INVALID. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-040 · TD TD-SCHEDULED-TRANSFER-L2-FR-040-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-040-AC-AC-FR-040-3 · nguồn FR-040 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo STUB của test case. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-040-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-040-3 | ac-fr-040-3 | firstRunAt cách hiện tại < 5 phút hoặc > 90 ngày thì trả 422 SCHEDULE_TIME_INVALID | Tiêu chí chấp nhận AC-FR-040-3 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống đang ở trạng thái ban đầu, dữ liệu nền và các stub phụ thuộc bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-040 theo tiêu chí: firstRunAt cách hiện tại < 5 phút hoặc > 90 ngày thì trả 422 SCHEDULE_TIME_INVALID. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả trả về và trạng thái dữ liệu. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | firstRunAt cách hiện tại < 5 phút hoặc > 90 ngày thì trả 422 SCHEDULE_TIME_INVALID |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-4: Xác nhận FR-040 được đáp ứng: khi toAccountId không tồn tại thì hệ thống trả 404 ACCOUNT_NOT_FOUND.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-4 |
| Mục tiêu | Xác nhận FR-040 được đáp ứng: khi toAccountId không tồn tại thì hệ thống trả 404 ACCOUNT_NOT_FOUND. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-040 · TD TD-SCHEDULED-TRANSFER-L2-FR-040-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-040-AC-AC-FR-040-4 · nguồn FR-040 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo STUB của test case. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-040-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-040-4 | ac-fr-040-4 | toAccountId không tồn tại thì trả 404 ACCOUNT_NOT_FOUND | Tiêu chí chấp nhận AC-FR-040-4 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống đang ở trạng thái ban đầu, dữ liệu nền và các stub phụ thuộc bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-040 theo tiêu chí: toAccountId không tồn tại thì trả 404 ACCOUNT_NOT_FOUND. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả trả về và trạng thái dữ liệu. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | toAccountId không tồn tại thì trả 404 ACCOUNT_NOT_FOUND |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-5: Xác nhận FR-040 được đáp ứng: khi fromAccountId không thuộc chủ sở hữu của API key thì hệ thống trả 403 FORBIDDEN.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-5 |
| Mục tiêu | Xác nhận FR-040 được đáp ứng: khi fromAccountId không thuộc chủ sở hữu của API key thì hệ thống trả 403 FORBIDDEN. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-040 · TD TD-SCHEDULED-TRANSFER-L2-FR-040-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-040-AC-AC-FR-040-5 · nguồn FR-040 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo STUB của test case. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-040-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-040-5 | ac-fr-040-5 | fromAccountId khác chủ API key thì trả 403 FORBIDDEN | Tiêu chí chấp nhận AC-FR-040-5 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống đang ở trạng thái ban đầu, dữ liệu nền và các stub phụ thuộc bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-040 theo tiêu chí: fromAccountId khác chủ API key thì trả 403 FORBIDDEN. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả trả về và trạng thái dữ liệu. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | fromAccountId khác chủ API key thì trả 403 FORBIDDEN |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-6: Xác nhận FR-040 được đáp ứng: khi một tài khoản tạo lịch thứ 21 ở trạng thái ACTIVE/PAUSED thì hệ thống trả 422 SCHEDULE_LIMIT_REACHED.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-6 |
| Mục tiêu | Xác nhận FR-040 được đáp ứng: khi một tài khoản tạo lịch thứ 21 ở trạng thái ACTIVE/PAUSED thì hệ thống trả 422 SCHEDULE_LIMIT_REACHED. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-040 · TD TD-SCHEDULED-TRANSFER-L2-FR-040-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-040-AC-AC-FR-040-6 · nguồn FR-040 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo STUB của test case. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-040-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-040-6 | ac-fr-040-6 | Lịch thứ 21 ACTIVE/PAUSED của một tài khoản trả 422 SCHEDULE_LIMIT_REACHED | Tiêu chí chấp nhận AC-FR-040-6 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống đang ở trạng thái ban đầu, dữ liệu nền và các stub phụ thuộc bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-040 theo tiêu chí: Lịch thứ 21 ACTIVE/PAUSED của một tài khoản trả 422 SCHEDULE_LIMIT_REACHED. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả trả về và trạng thái dữ liệu. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Lịch thứ 21 ACTIVE/PAUSED của một tài khoản trả 422 SCHEDULE_LIMIT_REACHED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-REV-003: Xác nhận FR-040 được đáp ứng: chỉ các lịch ACTIVE hoặc PAUSED được tính vào giới hạn 20 lịch của một tài khoản; lịch COMPLETED hoặc CANCELLED không được tính.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-REV-003 |
| Mục tiêu | Xác nhận FR-040 được đáp ứng: chỉ các lịch ACTIVE hoặc PAUSED được tính vào giới hạn 20 lịch của một tài khoản; lịch COMPLETED hoặc CANCELLED không được tính. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-040 · TD TD-SCHEDULED-TRANSFER-L2-FR-040-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-040-AC-AC-REV-003 · nguồn FR-040 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo STUB của test case. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-040-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-REV-003 | ac-rev-003 | Chỉ lịch ACTIVE hoặc PAUSED được tính vào giới hạn 20 lịch của tài khoản; lịch COMPLETED hoặc CANCELLED không được tính | Tiêu chí chấp nhận AC-REV-003 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống đang ở trạng thái ban đầu, dữ liệu nền và các stub phụ thuộc bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-040 theo tiêu chí: chỉ lịch ACTIVE hoặc PAUSED được tính vào giới hạn 20 lịch của tài khoản; lịch COMPLETED hoặc CANCELLED không được tính. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả trả về và trạng thái dữ liệu. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Chỉ lịch ACTIVE hoặc PAUSED được tính vào giới hạn 20 lịch của tài khoản; lịch COMPLETED hoặc CANCELLED không được tính |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-FR-041-1: Xác nhận FR-041 được đáp ứng: lịch DAILY, WEEKLY hoặc MONTHLY có endDate hoặc maxRuns được tạo thành công và trả về 201.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-FR-041-1 |
| Mục tiêu | Xác nhận FR-041 được đáp ứng: lịch DAILY, WEEKLY hoặc MONTHLY có endDate hoặc maxRuns được tạo thành công và trả về 201. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-041 · TD TD-SCHEDULED-TRANSFER-L2-FR-041-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-041-AC-AC-FR-041-1 · nguồn FR-041 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và các stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-041-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-041-1 | ac-fr-041-1 | Lịch DAILY, WEEKLY hoặc MONTHLY có endDate hoặc maxRuns được tạo thành công (201) | Tiêu chí chấp nhận AC-FR-041-1 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống ở trạng thái ban đầu, dữ liệu nền và các stub phụ thuộc bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-041 theo tiêu chí: lịch DAILY, WEEKLY hoặc MONTHLY có endDate hoặc maxRuns được tạo thành công (201). | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả trả về và trạng thái dữ liệu. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Lịch DAILY, WEEKLY hoặc MONTHLY có endDate hoặc maxRuns được tạo thành công (201) |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-FR-041-2: Xác nhận FR-041 được đáp ứng: lịch định kỳ thiếu cả endDate lẫn maxRuns thì trả về 400 INVALID_REQUEST.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-FR-041-2 |
| Mục tiêu | Xác nhận FR-041 được đáp ứng: lịch định kỳ thiếu cả endDate lẫn maxRuns thì trả về 400 INVALID_REQUEST. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-041 · TD TD-SCHEDULED-TRANSFER-L2-FR-041-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-041-AC-AC-FR-041-2 · nguồn FR-041 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và các stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-041-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-041-2 | ac-fr-041-2 | Lịch định kỳ thiếu cả endDate và maxRuns trả 400 INVALID_REQUEST | Tiêu chí chấp nhận AC-FR-041-2 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống ở trạng thái ban đầu, dữ liệu nền và các stub phụ thuộc bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-041 theo tiêu chí: lịch định kỳ thiếu cả endDate và maxRuns trả 400 INVALID_REQUEST. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả trả về và trạng thái dữ liệu. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Lịch định kỳ thiếu cả endDate và maxRuns trả 400 INVALID_REQUEST |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-FR-041-3: Xác nhận FR-041 được đáp ứng: sau khi một lần chạy kết thúc, hệ thống sinh lần chạy PENDING kế tiếp đúng chu kỳ cho đến khi đạt maxRuns hoặc endDate, sau đó lịch chuyển sang COMPLETED.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-FR-041-3 |
| Mục tiêu | Xác nhận FR-041 được đáp ứng: sau khi một lần chạy kết thúc, hệ thống sinh lần chạy PENDING kế tiếp đúng chu kỳ cho đến khi đạt maxRuns hoặc endDate, sau đó lịch chuyển sang COMPLETED. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-041 · TD TD-SCHEDULED-TRANSFER-L2-FR-041-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-041-AC-AC-FR-041-3 · nguồn FR-041 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và các stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-041-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-041-3 | ac-fr-041-3 | Sau khi một lần chạy kết thúc, hệ thống sinh lần chạy PENDING kế tiếp đúng chu kỳ cho đến khi đạt maxRuns hoặc endDate, rồi lịch COMPLETED | Tiêu chí chấp nhận AC-FR-041-3 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống ở trạng thái ban đầu, dữ liệu nền và các stub phụ thuộc bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-041 theo tiêu chí: sau khi một lần chạy kết thúc, hệ thống sinh lần chạy PENDING kế tiếp đúng chu kỳ cho đến khi đạt maxRuns hoặc endDate, rồi lịch COMPLETED. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Sau khi một lần chạy kết thúc, hệ thống sinh lần chạy PENDING kế tiếp đúng chu kỳ cho đến khi đạt maxRuns hoặc endDate, rồi lịch COMPLETED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-FR-041-4: Xác nhận FR-041 được đáp ứng: lịch MONTHLY đặt vào ngày 29, 30 hoặc 31 sẽ chạy vào ngày cuối tháng ở những tháng không đủ ngày, và vào ngày 29/02 ở năm nhuận.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-FR-041-4 |
| Mục tiêu | Xác nhận FR-041 được đáp ứng: lịch MONTHLY đặt vào ngày 29, 30 hoặc 31 sẽ chạy vào ngày cuối tháng ở những tháng không đủ ngày, và vào ngày 29/02 ở năm nhuận. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-041 · TD TD-SCHEDULED-TRANSFER-L2-FR-041-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-041-AC-AC-FR-041-4 · nguồn FR-041 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và các stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-041-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-041-4 | ac-fr-041-4 | Lịch MONTHLY đặt ngày 29, 30 hoặc 31 chạy vào ngày cuối tháng ở tháng không đủ ngày, và ngày 29/02 ở năm nhuận | Tiêu chí chấp nhận AC-FR-041-4 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống ở trạng thái ban đầu, dữ liệu nền và các stub phụ thuộc bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-041 theo tiêu chí: lịch MONTHLY đặt ngày 29, 30 hoặc 31 chạy vào ngày cuối tháng ở tháng không đủ ngày, và vào ngày 29/02 ở năm nhuận. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Lịch MONTHLY đặt ngày 29, 30 hoặc 31 chạy vào ngày cuối tháng ở tháng không đủ ngày, và ngày 29/02 ở năm nhuận |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-REV-001: Xác nhận FR-041 được đáp niệm: lịch định kỳ có endDate trước ngày của lần chạy đầu, hoặc endDate quá 366 ngày kể từ lần chạy đầu, thì trả về 422 SCHEDULE_TIME_INVALID.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-REV-001 |
| Mục tiêu | Xác nhận FR-041 được đáp niệm: lịch định kỳ có endDate trước ngày của lần chạy đầu, hoặc endDate quá 366 ngày kể từ lần chạy đầu, thì trả về 422 SCHEDULE_TIME_INVALID. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-041 · TD TD-SCHEDULED-TRANSFER-L2-FR-041-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-041-AC-AC-REV-001 · nguồn FR-041 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và các stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-041-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-REV-001 | ac-rev-001 | Lịch định kỳ có endDate trước ngày của lần chạy đầu, hoặc endDate quá 366 ngày kể từ lần chạy đầu, thì trả 422 SCHEDULE_TIME_INVALID | Tiêu chí chấp nhận AC-REV-001 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống ở trạng thái ban đầu, dữ liệu nền và các stub phụ thuộc bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-041 theo tiêu chí: lịch định kỳ có endDate trước ngày của lần chạy đầu, hoặc endDate quá 366 ngày kể từ lần chạy đầu, thì trả 422 SCHEDULE_TIME_INVALID. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả trả về và trạng thái dữ liệu. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Lịch định kỳ có endDate trước ngày của lần chạy đầu, hoặc endDate quá 366 ngày kể từ lần chạy đầu, thì trả 422 SCHEDULE_TIME_INVALID |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-REV-002: Xác nhận FR-041 được đáp ứng: lịch định kỳ có cả endDate và maxRuns sẽ kết thúc (COMPLETED) theo điều kiện đạt trước, và không sinh thêm lần chạy PENDING.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-REV-002 |
| Mục tiêu | Xác nhận FR-041 được đáp ứng: lịch định kỳ có cả endDate và maxRuns sẽ kết thúc (COMPLETED) theo điều kiện đạt trước, và không sinh thêm lần chạy PENDING. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-041 · TD TD-SCHEDULED-TRANSFER-L2-FR-041-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-041-AC-AC-REV-002 · nguồn FR-041 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và các stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-041-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-REV-002 | ac-rev-002 | Lịch định kỳ có cả endDate và maxRuns kết thúc (COMPLETED) ở điều kiện đạt trước, không sinh thêm lần chạy PENDING | Tiêu chí chấp nhận AC-REV-002 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống ở trạng thái ban đầu, dữ liệu nền và các stub phụ thuộc bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-041 theo tiêu chí: lịch định kỳ có cả endDate và maxRuns kết thúc (COMPLETED) ở điều kiện đạt trước, không sinh thêm lần chạy PENDING. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Lịch định kỳ có cả endDate và maxRuns kết thúc (COMPLETED) ở điều kiện đạt trước, không sinh thêm lần chạy PENDING |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-042-AC-AC-FR-042-1: Xác nhận yêu cầu FR-042 được đáp ứng: khi gọi GET một lịch chuyển tiền, hệ thống trả về status và nextRunAt.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-042-AC-AC-FR-042-1 |
| Mục tiêu | Xác nhận yêu cầu FR-042 được đáp ứng: khi gọi GET một lịch chuyển tiền, hệ thống trả về status và nextRunAt. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-042 · TD TD-SCHEDULED-TRANSFER-L2-FR-042-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-042-AC-AC-FR-042-1 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-042-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-042-1 | ac-fr-042-1 | GET một lịch trả status và nextRunAt | Tiêu chí chấp nhận AC-FR-042-1 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống ở trạng thái ban đầu, dữ liệu nền và các stub phụ thuộc bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-042 theo tiêu chí: GET một lịch trả status và nextRunAt. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả trả về và trạng thái dữ liệu sau khi thực hiện. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | GET một lịch trả status và nextRunAt |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-042-AC-AC-FR-042-2: Xác nhận yêu cầu FR-042 được đáp ứng: GET danh sách lịch của tài khoản hỗ trợ lọc theo status và phân trang bằng limit, offset.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-042-AC-AC-FR-042-2 |
| Mục tiêu | Xác nhận yêu cầu FR-042 được đáp ứng: GET danh sách lịch của tài khoản hỗ trợ lọc theo status và phân trang bằng limit, offset. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-042 · TD TD-SCHEDULED-TRANSFER-L2-FR-042-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-042-AC-AC-FR-042-2 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-042-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-042-2 | ac-fr-042-2 | GET danh sách lịch của tài khoản hỗ trợ lọc status và phân trang limit, offset | Tiêu chí chấp nhận AC-FR-042-2 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống ở trạng thái ban đầu, dữ liệu nền và các stub phụ thuộc bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-042 theo tiêu chí: GET danh sách lịch của tài khoản hỗ trợ lọc status và phân trang limit, offset. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả trả về và trạng thái dữ liệu sau khi thực hiện. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | GET danh sách lịch của tài khoản hỗ trợ lọc status và phân trang limit, offset |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-042-AC-AC-FR-042-3: Xác nhận yêu cầu FR-042 được đáp ứng: GET runs trả về từng lần chạy kèm scheduledAt, status, transferId và reasonCode, sắp xếp lần mới nhất lên trước.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-042-AC-AC-FR-042-3 |
| Mục tiêu | Xác nhận yêu cầu FR-042 được đáp ứng: GET runs trả về từng lần chạy kèm scheduledAt, status, transferId và reasonCode, sắp xếp lần mới nhất lên trước. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-042 · TD TD-SCHEDULED-TRANSFER-L2-FR-042-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-042-AC-AC-FR-042-3 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-042-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-042-3 | ac-fr-042-3 | GET runs trả mỗi lần chạy với scheduledAt, status, transferId và reasonCode, sắp mới nhất trước | Tiêu chí chấp nhận AC-FR-042-3 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống ở trạng thái ban đầu, dữ liệu nền và các stub phụ thuộc bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-042 theo tiêu chí: GET runs trả mỗi lần chạy với scheduledAt, status, transferId và reasonCode, sắp mới nhất trước. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả trả về và trạng thái dữ liệu sau khi thực hiện. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | GET runs trả mỗi lần chạy với scheduledAt, status, transferId và reasonCode, sắp mới nhất trước |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-042-AC-AC-FR-042-4: Xác nhận yêu cầu FR-042 được đáp ứng: người không phải chủ tài khoản nguồn nhận 403 FORBIDDEN; mã lịch không tồn tại nhận 404 SCHEDULE_NOT_FOUND.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-042-AC-AC-FR-042-4 |
| Mục tiêu | Xác nhận yêu cầu FR-042 được đáp ứng: người không phải chủ tài khoản nguồn nhận 403 FORBIDDEN; mã lịch không tồn tại nhận 404 SCHEDULE_NOT_FOUND. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-042 · TD TD-SCHEDULED-TRANSFER-L2-FR-042-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-042-AC-AC-FR-042-4 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-042-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-042-4 | ac-fr-042-4 | Người không phải chủ tài khoản nguồn nhận 403 FORBIDDEN; mã lịch không có nhận 404 SCHEDULE_NOT_FOUND | Tiêu chí chấp nhận AC-FR-042-4 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống ở trạng thái ban đầu, dữ liệu nền và các stub phụ thuộc bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-042 theo tiêu chí: người không phải chủ tài khoản nguồn nhận 403 FORBIDDEN; mã lịch không có nhận 404 SCHEDULE_NOT_FOUND. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả trả về và trạng thái dữ liệu sau khi thực hiện. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Người không phải chủ tài khoản nguồn nhận 403 FORBIDDEN; mã lịch không có nhận 404 SCHEDULE_NOT_FOUND |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-043-AC-AC-FR-043-1: Xác nhận FR-043 được đáp ứng: khi gọi DELETE trên lịch đang ACTIVE, lịch chuyển sang CANCELLED và các lần chạy đang PENDING cũng chuyển sang CANCELLED.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-043-AC-AC-FR-043-1 |
| Mục tiêu | Xác nhận FR-043 được đáp ứng: khi gọi DELETE trên lịch đang ACTIVE, lịch chuyển sang CANCELLED và các lần chạy đang PENDING cũng chuyển sang CANCELLED. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-043 · TD TD-SCHEDULED-TRANSFER-L2-FR-043-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-043-AC-AC-FR-043-1 · nguồn FR-043 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-043-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-043-1 | ac-fr-043-1 | DELETE lịch ACTIVE đổi lịch thành CANCELLED và các lần chạy PENDING thành CANCELLED | Tiêu chí chấp nhận AC-FR-043-1 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống ở trạng thái ban đầu, dữ liệu nền và các stub phụ thuộc bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-043 theo tiêu chí: gọi DELETE trên lịch ACTIVE để lịch chuyển sang CANCELLED và các lần chạy PENDING chuyển sang CANCELLED. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả trả về và trạng thái dữ liệu sau khi thực hiện. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | DELETE lịch ACTIVE đổi lịch thành CANCELLED và các lần chạy PENDING thành CANCELLED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-043-AC-AC-FR-043-2: Xác nhận FR-043 được đáp ứng: khi gọi DELETE trên lịch đã CANCELLED, hệ thống trả về 200 cùng kết quả như lần trước.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-043-AC-AC-FR-043-2 |
| Mục tiêu | Xác nhận FR-043 được đáp ứng: khi gọi DELETE trên lịch đã CANCELLED, hệ thống trả về 200 cùng kết quả như lần trước. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-043 · TD TD-SCHEDULED-TRANSFER-L2-FR-043-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-043-AC-AC-FR-043-2 · nguồn FR-043 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-043-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-043-2 | ac-fr-043-2 | DELETE lịch đã CANCELLED trả 200 với kết quả cũ | Tiêu chí chấp nhận AC-FR-043-2 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống ở trạng thái ban đầu, dữ liệu nền và các stub phụ thuộc bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-043 theo tiêu chí: gọi DELETE trên lịch đã CANCELLED và kiểm tra hệ thống trả 200 với kết quả cũ. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả trả về và trạng thái dữ liệu sau khi thực hiện. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | DELETE lịch đã CANCELLED trả 200 với kết quả cũ |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-043-AC-AC-FR-043-3: Xác nhận FR-043 được đáp ứng: khi gọi DELETE mà có lần chạy đang RUNNING hoặc lịch đã COMPLETED, hệ thống trả về 409 SCHEDULE_NOT_CANCELLABLE.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-043-AC-AC-FR-043-3 |
| Mục tiêu | Xác nhận FR-043 được đáp ứng: khi gọi DELETE mà có lần chạy đang RUNNING hoặc lịch đã COMPLETED, hệ thống trả về 409 SCHEDULE_NOT_CANCELLABLE. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-043 · TD TD-SCHEDULED-TRANSFER-L2-FR-043-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-043-AC-AC-FR-043-3 · nguồn FR-043 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-043-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-043-3 | ac-fr-043-3 | DELETE khi có lần chạy RUNNING hoặc lịch COMPLETED trả 409 SCHEDULE_NOT_CANCELLABLE | Tiêu chí chấp nhận AC-FR-043-3 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống ở trạng thái ban đầu, dữ liệu nền và các stub phụ thuộc bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-043 theo tiêu chí: gọi DELETE khi có lần chạy RUNNING hoặc lịch COMPLETED và kiểm tra hệ thống trả 409 SCHEDULE_NOT_CANCELLABLE. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả trả về và trạng thái dữ liệu sau khi thực hiện. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | DELETE khi có lần chạy RUNNING hoặc lịch COMPLETED trả 409 SCHEDULE_NOT_CANCELLABLE |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-044-AC-AC-FR-044-1: Xác nhận FR-044 được đáp ứng: khi gửi PATCH với status=PAUSED lên lịch định kỳ đang ACTIVE, lịch chuyển sang PAUSED và scheduler không chạy lịch này nữa.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-044-AC-AC-FR-044-1 |
| Mục tiêu | Xác nhận FR-044 được đáp ứng: khi gửi PATCH với status=PAUSED lên lịch định kỳ đang ACTIVE, lịch chuyển sang PAUSED và scheduler không chạy lịch này nữa. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-044 · TD TD-SCHEDULED-TRANSFER-L2-FR-044-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-044-AC-AC-FR-044-1 · nguồn FR-044 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-044-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-044-1 | ac-fr-044-1 | PATCH status=PAUSED trên lịch định kỳ ACTIVE đổi sang PAUSED, scheduler không chạy lịch này | Tiêu chí chấp nhận AC-FR-044-1 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống ở trạng thái ban đầu, dữ liệu nền và các stub phụ thuộc ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-044 theo tiêu chí: gửi PATCH với status=PAUSED lên lịch định kỳ đang ACTIVE, lịch chuyển sang PAUSED và scheduler không chạy lịch này. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả trả về và trạng thái dữ liệu sau khi thực hiện. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | PATCH status=PAUSED trên lịch định kỳ ACTIVE đổi sang PAUSED, scheduler không chạy lịch này |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-044-AC-AC-FR-044-2: Xác nhận FR-044 được đáp ứng: khi gửi PATCH với status=ACTIVE lên lịch đang PAUSED, lịch chuyển sang ACTIVE; các lần đã bỏ lỡ trong thời gian tạm dừng có trạng thái SKIPPED và không bị chạy bù.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-044-AC-AC-FR-044-2 |
| Mục tiêu | Xác nhận FR-044 được đáp ứng: khi gửi PATCH với status=ACTIVE lên lịch đang PAUSED, lịch chuyển sang ACTIVE; các lần đã bỏ lỡ trong thời gian tạm dừng có trạng thái SKIPPED và không bị chạy bù. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-044 · TD TD-SCHEDULED-TRANSFER-L2-FR-044-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-044-AC-AC-FR-044-2 · nguồn FR-044 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-044-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-044-2 | ac-fr-044-2 | PATCH status=ACTIVE trên lịch PAUSED đổi sang ACTIVE và các lần đã bỏ lỡ trong lúc tạm dừng có trạng thái SKIPPED, không bị chạy bù | Tiêu chí chấp nhận AC-FR-044-2 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống ở trạng thái ban đầu, dữ liệu nền và các stub phụ thuộc ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-044 theo tiêu chí: gửi PATCH với status=ACTIVE lên lịch đang PAUSED, lịch chuyển sang ACTIVE; các lần đã bỏ lỡ trong thời gian tạm dừng có trạng thái SKIPPED và không bị chạy bù. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả trả về và trạng thái dữ liệu sau khi thực hiện. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | PATCH status=ACTIVE trên lịch PAUSED đổi sang ACTIVE và các lần đã bỏ lỡ trong lúc tạm dừng có trạng thái SKIPPED, không bị chạy bù |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-044-AC-AC-FR-044-3: Xác nhận FR-044 được đáp ứng: khi gửi PATCH với trạng thái nguồn không hợp lệ hoặc trên lịch ONCE, hệ thống trả về 409 SCHEDULE_STATE_INVALID.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-044-AC-AC-FR-044-3 |
| Mục tiêu | Xác nhận FR-044 được đáp ứng: khi gửi PATCH với trạng thái nguồn không hợp lệ hoặc trên lịch ONCE, hệ thống trả về 409 SCHEDULE_STATE_INVALID. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-044 · TD TD-SCHEDULED-TRANSFER-L2-FR-044-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-044-AC-AC-FR-044-3 · nguồn FR-044 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-044-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-044-3 | ac-fr-044-3 | PATCH sai trạng thái nguồn hoặc trên lịch ONCE trả 409 SCHEDULE_STATE_INVALID | Tiêu chí chấp nhận AC-FR-044-3 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống ở trạng thái ban đầu, dữ liệu nền và các stub phụ thuộc ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-044 theo tiêu chí: gửi PATCH với trạng thái nguồn không hợp lệ hoặc trên lịch ONCE và nhận về 409 SCHEDULE_STATE_INVALID. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả trả về và trạng thái dữ liệu sau khi thực hiện. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | PATCH sai trạng thái nguồn hoặc trên lịch ONCE trả 409 SCHEDULE_STATE_INVALID |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-FR-045-1: Xác nhận FR-045 được đáp ứng: mỗi lần chạy đến hạn tạo đúng một giao dịch ở trạng thái COMPLETED thông qua luồng chuyển tiền hiện có, và lượt chạy (run) có trạng thái SUCCEEDED kèm transferId.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-FR-045-1 |
| Mục tiêu | Xác nhận FR-045 được đáp ứng: mỗi lần chạy đến hạn tạo đúng một giao dịch ở trạng thái COMPLETED thông qua luồng chuyển tiền hiện có, và lượt chạy (run) có trạng thái SUCCEEDED kèm transferId. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-045 · TD TD-SCHEDULED-TRANSFER-L2-FR-045-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-045-AC-AC-FR-045-1 · nguồn FR-045 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và các stub được cấu hình theo mục STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-045-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-045-1 | ac-fr-045-1 | Lần chạy đến hạn tạo đúng một giao dịch COMPLETED qua luồng chuyển tiền hiện có, run SUCCEEDED có transferId | Tiêu chí chấp nhận AC-FR-045-1 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống ở trạng thái ban đầu, dữ liệu nền và các stub cho dịch vụ bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-045 theo tiêu chí: lần chạy đến hạn tạo đúng một giao dịch COMPLETED qua luồng chuyển tiền hiện có, run SUCCEEDED có transferId. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và kiểm tra trạng thái dữ liệu sau khi chạy. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Lần chạy đến hạn tạo đúng một giao dịch COMPLETED qua luồng chuyển tiền hiện có, run SUCCEEDED có transferId |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-FR-045-2: Xác nhận FR-045 được đáp ứng: Idempotency-Key của lần chạy đầu tiên được tạo từ mã lịch kết hợp với thời điểm dự kiến chạy.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-FR-045-2 |
| Mục tiêu | Xác nhận FR-045 được đáp ứng: Idempotency-Key của lần chạy đầu tiên được tạo từ mã lịch kết hợp với thời điểm dự kiến chạy. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-045 · TD TD-SCHEDULED-TRANSFER-L2-FR-045-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-045-AC-AC-FR-045-2 · nguồn FR-045 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và các stub được cấu hình theo mục STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-045-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-045-2 | ac-fr-045-2 | Idempotency-Key của lần chạy đầu bằng mã lịch + thời điểm dự kiến | Tiêu chí chấp nhận AC-FR-045-2 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống ở trạng thái ban đầu, dữ liệu nền và các stub cho dịch vụ bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-045 theo tiêu chí: Idempotency-Key của lần chạy đầu bằng mã lịch + thời điểm dự kiến. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và kiểm tra trạng thái dữ liệu sau khi chạy. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Idempotency-Key của lần chạy đầu bằng mã lịch + thời điểm dự kiến |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-FR-045-3: Xác nhận FR-045 được đáp ứng: hạn mức ngày, số dư, trạng thái tài khoản và Fraud Check được kiểm tra tại thời điểm chạy lịch, không phải tại thời điểm tạo lịch.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-FR-045-3 |
| Mục tiêu | Xác nhận FR-045 được đáp ứng: hạn mức ngày, số dư, trạng thái tài khoản và Fraud Check được kiểm tra tại thời điểm chạy lịch, không phải tại thời điểm tạo lịch. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-045 · TD TD-SCHEDULED-TRANSFER-L2-FR-045-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-045-AC-AC-FR-045-3 · nguồn FR-045 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và các stub được cấu hình theo mục STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-045-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-045-3 | ac-fr-045-3 | Hạn mức ngày, số dư, trạng thái tài khoản và Fraud Check được kiểm tra tại lúc chạy, không phải lúc tạo lịch | Tiêu chí chấp nhận AC-FR-045-3 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống ở trạng thái ban đầu, dữ liệu nền và các stub cho dịch vụ bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-045 theo tiêu chí: hạn mức ngày, số dư, trạng thái tài khoản và Fraud Check được kiểm tra tại lúc chạy, không phải lúc tạo lịch. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và kiểm tra trạng thái dữ liệu sau khi chạy. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Hạn mức ngày, số dư, trạng thái tài khoản và Fraud Check được kiểm tra tại lúc chạy, không phải lúc tạo lịch |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-FR-045-4: Xác nhận FR-045 được đáp ứng: giao dịch sinh ra từ lịch được tính vào hạn mức ngày và hiển thị trong lịch sử giao dịch.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-FR-045-4 |
| Mục tiêu | Xác nhận FR-045 được đáp ứng: giao dịch sinh ra từ lịch được tính vào hạn mức ngày và hiển thị trong lịch sử giao dịch. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-045 · TD TD-SCHEDULED-TRANSFER-L2-FR-045-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-045-AC-AC-FR-045-4 · nguồn FR-045 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và các stub được cấu hình theo mục STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-045-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-045-4 | ac-fr-045-4 | Giao dịch sinh từ lịch được tính vào hạn mức ngày và hiển thị trong lịch sử giao dịch | Tiêu chí chấp nhận AC-FR-045-4 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống ở trạng thái ban đầu, dữ liệu nền và các stub cho dịch vụ bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-045 theo tiêu chí: giao dịch sinh từ lịch được tính vào hạn mức ngày và hiển thị trong lịch sử giao dịch. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và kiểm tra trạng thái dữ liệu sau khi chạy. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Giao dịch sinh từ lịch được tính vào hạn mức ngày và hiển thị trong lịch sử giao dịch |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-REV-004: Xác nhận FR-045 được đáp ứng: giao dịch sinh từ lịch hiển thị trong lịch sử giao dịch (FR-013) và có trường scheduleId mang giá trị là mã lịch.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-REV-004 |
| Mục tiêu | Xác nhận FR-045 được đáp ứng: giao dịch sinh từ lịch hiển thị trong lịch sử giao dịch (FR-013) và có trường scheduleId mang giá trị là mã lịch. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-045 · TD TD-SCHEDULED-TRANSFER-L2-FR-045-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-045-AC-AC-REV-004 · nguồn FR-045 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và các stub được cấu hình theo mục STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-045-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-REV-004 | ac-rev-004 | Giao dịch sinh từ lịch hiển thị trong lịch sử giao dịch (FR-013) kèm trường scheduleId là mã lịch | Tiêu chí chấp nhận AC-REV-004 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống ở trạng thái ban đầu, dữ liệu nền và các stub cho dịch vụ bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-045 theo tiêu chí: giao dịch sinh từ lịch hiển thị trong lịch sử giao dịch (FR-013) kèm trường scheduleId là mã lịch. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và kiểm tra trạng thái dữ liệu sau khi chạy. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Giao dịch sinh từ lịch hiển thị trong lịch sử giao dịch (FR-013) kèm trường scheduleId là mã lịch |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-045-SCN-SCN-ScheduledRun: Kiểm tra toàn bộ luồng chính SCN-ScheduledRun chạy trọn vẹn từ đầu đến cuối, với kết quả đúng ở mọi Module tham gia.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-045-SCN-SCN-ScheduledRun |
| Mục tiêu | Kiểm tra toàn bộ luồng chính SCN-ScheduledRun chạy trọn vẹn từ đầu đến cuối, với kết quả đúng ở mọi Module tham gia. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-045 · TD TD-SCHEDULED-TRANSFER-L2-FR-045-SCN · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-045-SCN-SCN-ScheduledRun · nguồn FR-045 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-045-SCN |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| SCN-ScheduledRun | scn-scheduledrun | 8 | Luồng chính SCN-ScheduledRun |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống và các Module tham gia đang ở trạng thái ban đầu, đồng thời stub của hệ thống ngoài trả về kết quả thành công. | MOD-API, MOD-Scheduler, MOD-Transfer |  |
| 2 | when | Thực hiện quét các lần chạy theo lịch với chu kỳ 30 giây một lần. | — |  |
| 3 | when | Chọn tối đa 20 lần chạy đã đến hạn. | — |  |
| 4 | when | Chiếm lần chạy bằng câu lệnh UPDATE có điều kiện. | — |  |
| 5 | when | Sinh correlationId cho lần chạy. | — |  |
| 6 | when | Thực hiện lõi chuyển tiền, gồm kiểm tra idempotency, kiểm tra hạn mức và số dư, Fraud Check, rồi ghi sổ cùng outbox. | — |  |
| 7 | when | Cập nhật kết quả của lần chạy. | — |  |
| 8 | when | Xử lý thử lại hoặc chuyển sang trạng thái FAILED khi cần. | — |  |
| 9 | when | Ghi audit và phát sự kiện. | — |  |
| 10 | then | Xác nhận luồng đã hoàn tất, sau đó kiểm tra trạng thái cuối ở mọi Module tham gia. | MOD-API, MOD-Scheduler, MOD-Transfer |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | hoàn tất |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-FR-046-AC-AC-FR-046-1: Kiểm tra FR-046 được đáp ứng: khi gặp lỗi FRAUD_CHECK_UNAVAILABLE, hệ thống thử lại tối đa 3 lần, mỗi lần cách nhau 5 phút; hết số lần thử mà vẫn lỗi thì run chuyển sang FAILED kèm reasonCode.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-046-AC-AC-FR-046-1 |
| Mục tiêu | Kiểm tra FR-046 được đáp ứng: khi gặp lỗi FRAUD_CHECK_UNAVAILABLE, hệ thống thử lại tối đa 3 lần, mỗi lần cách nhau 5 phút; hết số lần thử mà vẫn lỗi thì run chuyển sang FAILED kèm reasonCode. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-046 · TD TD-SCHEDULED-TRANSFER-L2-FR-046-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-046-AC-AC-FR-046-1 · nguồn FR-046 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo mục STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-046-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-046-1 | ac-fr-046-1 | FRAUD_CHECK_UNAVAILABLE được thử lại tối đa 3 lần cách 5 phút, hết lượt thì run FAILED với reasonCode | Tiêu chí chấp nhận AC-FR-046-1 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống đang ở trạng thái ban đầu, dữ liệu nền và các stub cho dịch vụ bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-046 theo tiêu chí: FRAUD_CHECK_UNAVAILABLE được thử lại tối đa 3 lần cách 5 phút, hết lượt thì run FAILED với reasonCode. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả trả về và trạng thái dữ liệu sau khi thực hiện. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | FRAUD_CHECK_UNAVAILABLE được thử lại tối đa 3 lần cách 5 phút, hết lượt thì run FAILED với reasonCode |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-046-AC-AC-FR-046-2: Kiểm tra FR-046 được đáp ứng: khi gặp lỗi INSUFFICIENT_FUNDS, hệ thống thử lại tối đa 2 lần, mỗi lần cách nhau 1 giờ; hết số lần thử mà vẫn lỗi thì run chuyển sang FAILED.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-046-AC-AC-FR-046-2 |
| Mục tiêu | Kiểm tra FR-046 được đáp ứng: khi gặp lỗi INSUFFICIENT_FUNDS, hệ thống thử lại tối đa 2 lần, mỗi lần cách nhau 1 giờ; hết số lần thử mà vẫn lỗi thì run chuyển sang FAILED. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-046 · TD TD-SCHEDULED-TRANSFER-L2-FR-046-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-046-AC-AC-FR-046-2 · nguồn FR-046 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo mục STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-046-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-046-2 | ac-fr-046-2 | INSUFFICIENT_FUNDS được thử lại tối đa 2 lần cách 1 giờ, hết lượt thì run FAILED | Tiêu chí chấp nhận AC-FR-046-2 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống đang ở trạng thái ban đầu, dữ liệu nền và các stub cho dịch vụ bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-046 theo tiêu chí: INSUFFICIENT_FUNDS được thử lại tối đa 2 lần cách 1 giờ, hết lượt thì run FAILED. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả trả về và trạng thái dữ liệu sau khi thực hiện. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | INSUFFICIENT_FUNDS được thử lại tối đa 2 lần cách 1 giờ, hết lượt thì run FAILED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-046-AC-AC-FR-046-3: Kiểm tra FR-046 được đáp ứng: khi gặp lỗi ACCOUNT_INACTIVE, FRAUD_REJECTED hoặc LIMIT_EXCEEDED, run chuyển sang FAILED ngay lập tức và không được thử lại.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-046-AC-AC-FR-046-3 |
| Mục tiêu | Kiểm tra FR-046 được đáp ứng: khi gặp lỗi ACCOUNT_INACTIVE, FRAUD_REJECTED hoặc LIMIT_EXCEEDED, run chuyển sang FAILED ngay lập tức và không được thử lại. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-046 · TD TD-SCHEDULED-TRANSFER-L2-FR-046-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-046-AC-AC-FR-046-3 · nguồn FR-046 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo mục STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-046-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-046-3 | ac-fr-046-3 | ACCOUNT_INACTIVE, FRAUD_REJECTED, LIMIT_EXCEEDED làm run FAILED ngay, không thử lại | Tiêu chí chấp nhận AC-FR-046-3 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống đang ở trạng thái ban đầu, dữ liệu nền và các stub cho dịch vụ bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-046 theo tiêu chí: ACCOUNT_INACTIVE, FRAUD_REJECTED, LIMIT_EXCEEDED làm run FAILED ngay, không thử lại. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả trả về và trạng thái dữ liệu sau khi thực hiện. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | ACCOUNT_INACTIVE, FRAUD_REJECTED, LIMIT_EXCEEDED làm run FAILED ngay, không thử lại |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-046-AC-AC-FR-046-4: Kiểm tra FR-046 được đáp ứng: sau 3 lần chạy FAILED liên tiếp, lịch chuyển tiền định kỳ chuyển sang trạng thái PAUSED.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-046-AC-AC-FR-046-4 |
| Mục tiêu | Kiểm tra FR-046 được đáp ứng: sau 3 lần chạy FAILED liên tiếp, lịch chuyển tiền định kỳ chuyển sang trạng thái PAUSED. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-046 · TD TD-SCHEDULED-TRANSFER-L2-FR-046-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-046-AC-AC-FR-046-4 · nguồn FR-046 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo mục STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-046-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-046-4 | ac-fr-046-4 | Sau 3 lần chạy FAILED liên tiếp lịch định kỳ chuyển PAUSED | Tiêu chí chấp nhận AC-FR-046-4 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống đang ở trạng thái ban đầu, dữ liệu nền và các stub cho dịch vụ bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-046 theo tiêu chí: sau 3 lần chạy FAILED liên tiếp, lịch định kỳ chuyển PAUSED. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả trả về và trạng thái dữ liệu sau khi thực hiện. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Sau 3 lần chạy FAILED liên tiếp lịch định kỳ chuyển PAUSED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-046-SCN-SCN-ScheduledRun: Kiểm tra toàn bộ luồng chính SCN-ScheduledRun, từ lúc quét lần chạy đến hạn cho đến khi hoàn tất xử lý chuyển tiền theo lịch.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-046-SCN-SCN-ScheduledRun |
| Mục tiêu | Kiểm tra toàn bộ luồng chính SCN-ScheduledRun, từ lúc quét lần chạy đến hạn cho đến khi hoàn tất xử lý chuyển tiền theo lịch. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-046 · TD TD-SCHEDULED-TRANSFER-L2-FR-046-SCN · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-046-SCN-SCN-ScheduledRun · nguồn FR-046 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và các stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-046-SCN |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| SCN-ScheduledRun | scn-scheduledrun | 8 | Luồng chính SCN-ScheduledRun |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống và các Module tham gia đang ở trạng thái ban đầu, và stub của hệ thống ngoài trả về kết quả thành công. | MOD-API, MOD-Scheduler, MOD-Transfer |  |
| 2 | when | Hệ thống quét các lần chạy theo chu kỳ 30 giây một lần. | — |  |
| 3 | when | Chọn tối đa 20 lần chạy đã đến hạn. | — |  |
| 4 | when | Chiếm lần chạy bằng câu lệnh UPDATE có điều kiện. | — |  |
| 5 | when | Sinh correlationId cho lần chạy. | — |  |
| 6 | when | Thực hiện lõi chuyển tiền, gồm kiểm tra idempotency, hạn mức và số dư, Fraud Check, ghi sổ cùng outbox. | — |  |
| 7 | when | Cập nhật trạng thái của lần chạy. | — |  |
| 8 | when | Xử lý thử lại hoặc chuyển sang trạng thái FAILED khi cần. | — |  |
| 9 | when | Ghi audit và phát sự kiện. | — |  |
| 10 | then | Xác nhận luồng đã hoàn tất, rồi kiểm tra trạng thái cuối ở mọi Module tham gia. | MOD-API, MOD-Scheduler, MOD-Transfer |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | hoàn tất |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-FR-047-1: Xác nhận FR-047 được đáp ứng: mỗi lần chạy thành công đều phát sự kiện ScheduledRunCompleted kèm transferId thông qua outbox.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-FR-047-1 |
| Mục tiêu | Xác nhận FR-047 được đáp ứng: mỗi lần chạy thành công đều phát sự kiện ScheduledRunCompleted kèm transferId thông qua outbox. |
| Mức, loại, ưu tiên | L2 · functional · P1 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-047 · TD TD-SCHEDULED-TRANSFER-L2-FR-047-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-047-AC-AC-FR-047-1 · nguồn FR-047 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và các stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-02-success, STUB-SCHEDULED-TRANSFER-INT-03-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-047-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-047-1 | ac-fr-047-1 | Mỗi lần chạy thành công phát sự kiện ScheduledRunCompleted có transferId qua outbox | Tiêu chí chấp nhận AC-FR-047-1 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống ở trạng thái ban đầu, dữ liệu nền đã nạp và các stub của dịch vụ ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-047 theo tiêu chí: mỗi lần chạy thành công phát sự kiện ScheduledRunCompleted có transferId qua outbox. | MOD-Notify |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu. | MOD-Notify |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Mỗi lần chạy thành công phát sự kiện ScheduledRunCompleted có transferId qua outbox |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-FR-047-2: Xác nhận FR-047 được đáp ứng: mỗi lần chạy FAILED đều phát sự kiện ScheduledRunFailed kèm reasonCode thông qua outbox.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-FR-047-2 |
| Mục tiêu | Xác nhận FR-047 được đáp ứng: mỗi lần chạy FAILED đều phát sự kiện ScheduledRunFailed kèm reasonCode thông qua outbox. |
| Mức, loại, ưu tiên | L2 · functional · P1 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-047 · TD TD-SCHEDULED-TRANSFER-L2-FR-047-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-047-AC-AC-FR-047-2 · nguồn FR-047 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và các stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-02-success, STUB-SCHEDULED-TRANSFER-INT-03-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-047-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-047-2 | ac-fr-047-2 | Mỗi lần chạy FAILED phát sự kiện ScheduledRunFailed có reasonCode qua outbox | Tiêu chí chấp nhận AC-FR-047-2 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống ở trạng thái ban đầu, dữ liệu nền đã nạp và các stub của dịch vụ ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-047 theo tiêu chí: mỗi lần chạy FAILED phát sự kiện ScheduledRunFailed có reasonCode qua outbox. | MOD-Notify |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu. | MOD-Notify |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Mỗi lần chạy FAILED phát sự kiện ScheduledRunFailed có reasonCode qua outbox |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-FR-047-3: Xác nhận FR-047 được đáp ứng: SMS kết quả được gửi cho người gửi, và việc gửi SMS bị lỗi không làm thay đổi trạng thái của lần chạy.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-FR-047-3 |
| Mục tiêu | Xác nhận FR-047 được đáp ứng: SMS kết quả được gửi cho người gửi, và việc gửi SMS bị lỗi không làm thay đổi trạng thái của lần chạy. |
| Mức, loại, ưu tiên | L2 · functional · P1 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-047 · TD TD-SCHEDULED-TRANSFER-L2-FR-047-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-047-AC-AC-FR-047-3 · nguồn FR-047 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và các stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-02-success, STUB-SCHEDULED-TRANSFER-INT-03-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-047-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-047-3 | ac-fr-047-3 | SMS kết quả được gửi cho người gửi; lỗi gửi SMS không đổi trạng thái lần chạy | Tiêu chí chấp nhận AC-FR-047-3 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống ở trạng thái ban đầu, dữ liệu nền đã nạp và các stub của dịch vụ ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-047 theo tiêu chí: SMS kết quả được gửi cho người gửi; lỗi gửi SMS không đổi trạng thái lần chạy. | MOD-Notify |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu. | MOD-Notify |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | SMS kết quả được gửi cho người gửi; lỗi gửi SMS không đổi trạng thái lần chạy |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-REV-005: Xác nhận FR-047 được đáp ứng: sự kiện ScheduledRunCompleted/ScheduledRunFailed có đủ các trường eventId, scheduleId, runId, fromAccountId, toAccountId, amount, correlationId, kèm transferId (khi thành công) hoặc reasonCode (khi thất bại), và được phát trên cùng topic với sự kiện giao dịch.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-REV-005 |
| Mục tiêu | Xác nhận FR-047 được đáp ứng: sự kiện ScheduledRunCompleted/ScheduledRunFailed có đủ các trường eventId, scheduleId, runId, fromAccountId, toAccountId, amount, correlationId, kèm transferId (khi thành công) hoặc reasonCode (khi thất bại), và được phát trên cùng topic với sự kiện giao dịch. |
| Mức, loại, ưu tiên | L2 · functional · P1 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-047 · TD TD-SCHEDULED-TRANSFER-L2-FR-047-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-047-AC-AC-REV-005 · nguồn FR-047 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và các stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-02-success, STUB-SCHEDULED-TRANSFER-INT-03-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-047-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-REV-005 | ac-rev-005 | Sự kiện ScheduledRunCompleted/ScheduledRunFailed có đủ các trường eventId, scheduleId, runId, fromAccountId, toAccountId, amount, correlationId, kèm transferId (thành công) hoặc reasonCode (thất bại), trên cùng topic với sự kiện giao dịch | Tiêu chí chấp nhận AC-REV-005 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống ở trạng thái ban đầu, dữ liệu nền đã nạp và các stub của dịch vụ ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-047 theo tiêu chí: sự kiện ScheduledRunCompleted/ScheduledRunFailed có đủ các trường eventId, scheduleId, runId, fromAccountId, toAccountId, amount, correlationId, kèm transferId (thành công) hoặc reasonCode (thất bại), trên cùng topic với sự kiện giao dịch. | MOD-Notify |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu. | MOD-Notify |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Sự kiện ScheduledRunCompleted/ScheduledRunFailed có đủ các trường eventId, scheduleId, runId, fromAccountId, toAccountId, amount, correlationId, kèm transferId (thành công) hoặc reasonCode (thất bại), trên cùng topic với sự kiện giao dịch |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-REV-006: Xác nhận FR-047 được đáp ứng: nội dung SMS của lần chạy thất bại nêu rõ rằng số dư chỉ được kiểm tra tại thời điểm chạy.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-REV-006 |
| Mục tiêu | Xác nhận FR-047 được đáp ứng: nội dung SMS của lần chạy thất bại nêu rõ rằng số dư chỉ được kiểm tra tại thời điểm chạy. |
| Mức, loại, ưu tiên | L2 · functional · P1 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-047 · TD TD-SCHEDULED-TRANSFER-L2-FR-047-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-047-AC-AC-REV-006 · nguồn FR-047 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và các stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-02-success, STUB-SCHEDULED-TRANSFER-INT-03-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-047-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-REV-006 | ac-rev-006 | Nội dung SMS của lần chạy thất bại nêu rõ số dư chỉ được kiểm tra lúc chạy | Tiêu chí chấp nhận AC-REV-006 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống ở trạng thái ban đầu, dữ liệu nền đã nạp và các stub của dịch vụ ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-047 theo tiêu chí: nội dung SMS của lần chạy thất bại nêu rõ số dư chỉ được kiểm tra lúc chạy. | MOD-Notify |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu. | MOD-Notify |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Nội dung SMS của lần chạy thất bại nêu rõ số dư chỉ được kiểm tra lúc chạy |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-047-SCN-SCN-ScheduledRun: Kiểm tra luồng chính SCN-ScheduledRun chạy trọn vẹn từ đầu đến cuối, gồm quét lần chạy đến hạn, thực hiện chuyển tiền theo lịch và ghi nhận kết quả.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-047-SCN-SCN-ScheduledRun |
| Mục tiêu | Kiểm tra luồng chính SCN-ScheduledRun chạy trọn vẹn từ đầu đến cuối, gồm quét lần chạy đến hạn, thực hiện chuyển tiền theo lịch và ghi nhận kết quả. |
| Mức, loại, ưu tiên | L2 · functional · P1 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-047 · TD TD-SCHEDULED-TRANSFER-L2-FR-047-SCN · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-047-SCN-SCN-ScheduledRun · nguồn FR-047 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-02-success, STUB-SCHEDULED-TRANSFER-INT-03-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-047-SCN |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| SCN-ScheduledRun | scn-scheduledrun | 8 | Luồng chính SCN-ScheduledRun |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống và các Module tham gia đang ở trạng thái ban đầu, đồng thời stub của hệ thống ngoài trả về kết quả thành công. | MOD-API, MOD-Scheduler, MOD-Transfer |  |
| 2 | when | Hệ thống quét các lần chạy theo lịch với chu kỳ 30 giây một lần. | — |  |
| 3 | when | Chọn tối đa 20 lần chạy đã đến hạn. | — |  |
| 4 | when | Chiếm lần chạy bằng câu lệnh UPDATE có điều kiện. | — |  |
| 5 | when | Sinh correlationId cho lần chạy. | — |  |
| 6 | when | Gọi lõi chuyển tiền để xử lý idempotency, kiểm tra hạn mức/số dư, Fraud Check, ghi sổ và ghi outbox. | — |  |
| 7 | when | Cập nhật trạng thái và kết quả của lần chạy. | — |  |
| 8 | when | Xử lý thử lại hoặc chuyển sang trạng thái FAILED khi cần. | — |  |
| 9 | when | Ghi audit và phát sự kiện. | — |  |
| 10 | then | Xác nhận luồng đã hoàn tất và kiểm tra trạng thái cuối ở mọi Module tham gia. | MOD-API, MOD-Scheduler, MOD-Transfer |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | hoàn tất |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-FR-048-AC-AC-FR-048-1: Xác nhận FR-048 được đáp ứng: mỗi thao tác tạo, hủy, tạm dừng và tiếp tục lịch đều được ghi audit đầy đủ các thông tin gồm correlationId, thời điểm, tác nhân (là mã tài khoản), loại sự kiện và mã lịch.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-048-AC-AC-FR-048-1 |
| Mục tiêu | Xác nhận FR-048 được đáp ứng: mỗi thao tác tạo, hủy, tạm dừng và tiếp tục lịch đều được ghi audit đầy đủ các thông tin gồm correlationId, thời điểm, tác nhân (là mã tài khoản), loại sự kiện và mã lịch. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-048 · TD TD-SCHEDULED-TRANSFER-L2-FR-048-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-048-AC-AC-FR-048-1 · nguồn FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-048-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-048-1 | ac-fr-048-1 | Tạo, hủy, tạm dừng và tiếp tục lịch đều ghi audit với correlationId, thời điểm, tác nhân là mã tài khoản, loại sự kiện và mã lịch | Tiêu chí chấp nhận AC-FR-048-1 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống ở trạng thái ban đầu, dữ liệu nền và các stub phụ thuộc bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-048 theo tiêu chí: tạo, hủy, tạm dừng và tiếp tục lịch đều ghi audit với correlationId, thời điểm, tác nhân là mã tài khoản, loại sự kiện và mã lịch. | MOD-Audit |  |
| 3 | then | Quan sát kết quả trả về và trạng thái dữ liệu sau khi thực hiện. | MOD-Audit |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Tạo, hủy, tạm dừng và tiếp tục lịch đều ghi audit với correlationId, thời điểm, tác nhân là mã tài khoản, loại sự kiện và mã lịch |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-FR-048-AC-AC-FR-048-2: Xác nhận FR-048 được đáp ứng: mọi kết quả của lần chạy lịch đều được ghi audit với tác nhân là SCHEDULER và kèm mã lịch.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-048-AC-AC-FR-048-2 |
| Mục tiêu | Xác nhận FR-048 được đáp ứng: mọi kết quả của lần chạy lịch đều được ghi audit với tác nhân là SCHEDULER và kèm mã lịch. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-048 · TD TD-SCHEDULED-TRANSFER-L2-FR-048-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-048-AC-AC-FR-048-2 · nguồn FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-048-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-048-2 | ac-fr-048-2 | Mọi kết quả lần chạy ghi audit với tác nhân SCHEDULER và mã lịch | Tiêu chí chấp nhận AC-FR-048-2 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống ở trạng thái ban đầu, dữ liệu nền và các stub phụ thuộc bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-048 theo tiêu chí: mọi kết quả lần chạy đều ghi audit với tác nhân SCHEDULER và mã lịch. | MOD-Audit |  |
| 3 | then | Quan sát kết quả trả về và trạng thái dữ liệu sau khi thực hiện. | MOD-Audit |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Mọi kết quả lần chạy ghi audit với tác nhân SCHEDULER và mã lịch |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-FR-048-SCN-SCN-ScheduledRun: Kiểm tra toàn bộ luồng chính SCN-ScheduledRun, từ lúc hệ thống quét lần chạy đến hạn cho tới khi luồng hoàn tất và trạng thái cuối ở các Module tham gia đúng như mong đợi.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-048-SCN-SCN-ScheduledRun |
| Mục tiêu | Kiểm tra toàn bộ luồng chính SCN-ScheduledRun, từ lúc hệ thống quét lần chạy đến hạn cho tới khi luồng hoàn tất và trạng thái cuối ở các Module tham gia đúng như mong đợi. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-048 · TD TD-SCHEDULED-TRANSFER-L2-FR-048-SCN · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-048-SCN-SCN-ScheduledRun · nguồn FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-048-SCN |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| SCN-ScheduledRun | scn-scheduledrun | 8 | Luồng chính SCN-ScheduledRun |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống và các Module tham gia ở trạng thái ban đầu, đồng thời stub hệ thống ngoài trả về kết quả thành công. | MOD-API, MOD-Scheduler, MOD-Transfer |  |
| 2 | when | Hệ thống quét các lần chạy đến hạn mỗi 30 giây. | — |  |
| 3 | when | Chọn tối đa 20 lần chạy đến hạn trong mỗi lượt quét. | — |  |
| 4 | when | Chiếm lần chạy bằng câu lệnh UPDATE có điều kiện. | — |  |
| 5 | when | Sinh correlationId cho lần chạy. | — |  |
| 6 | when | Gọi lõi chuyển tiền, gồm kiểm tra idempotency, hạn mức/số dư, Fraud Check, ghi sổ và ghi outbox. | — |  |
| 7 | when | Cập nhật trạng thái của lần chạy. | — |  |
| 8 | when | Xử lý thử lại hoặc chuyển sang trạng thái FAILED khi cần. | — |  |
| 9 | when | Ghi audit và phát sự kiện. | — |  |
| 10 | then | Xác nhận luồng đã hoàn tất và kiểm tra trạng thái cuối ở mọi Module tham gia. | MOD-API, MOD-Scheduler, MOD-Transfer |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | hoàn tất |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-FR-049-AC-AC-FR-049-1: Xác nhận FR-049 được đáp ứng: khi lần chạy bị quá hạn không quá 24 giờ do hệ thống ngừng hoạt động, lần chạy đó được thực hiện đúng một lần.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-049-AC-AC-FR-049-1 |
| Mục tiêu | Xác nhận FR-049 được đáp ứng: khi lần chạy bị quá hạn không quá 24 giờ do hệ thống ngừng hoạt động, lần chạy đó được thực hiện đúng một lần. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-049 · TD TD-SCHEDULED-TRANSFER-L2-FR-049-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-049-AC-AC-FR-049-1 · nguồn FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo mục STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-049-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-049-1 | ac-fr-049-1 | Lần chạy quá hạn không quá 24 giờ sau khi hệ thống ngừng được chạy đúng một lần | Tiêu chí chấp nhận AC-FR-049-1 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đưa hệ thống về trạng thái ban đầu, đảm bảo dữ liệu nền và các stub phụ thuộc bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-049 theo tiêu chí: lần chạy quá hạn không quá 24 giờ sau khi hệ thống ngừng thì được chạy đúng một lần. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Lần chạy quá hạn không quá 24 giờ sau khi hệ thống ngừng được chạy đúng một lần |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-049-AC-AC-FR-049-2: Xác nhận FR-049 được đáp ứng: khi lần chạy quá hạn hơn 24 giờ, lần chạy được chuyển sang trạng thái MISSED, hệ thống phát sự kiện ScheduledRunFailed với reasonCode MISSED và không tạo giao dịch.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-049-AC-AC-FR-049-2 |
| Mục tiêu | Xác nhận FR-049 được đáp ứng: khi lần chạy quá hạn hơn 24 giờ, lần chạy được chuyển sang trạng thái MISSED, hệ thống phát sự kiện ScheduledRunFailed với reasonCode MISSED và không tạo giao dịch. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-049 · TD TD-SCHEDULED-TRANSFER-L2-FR-049-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-049-AC-AC-FR-049-2 · nguồn FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo mục STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-049-AC |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-049-2 | ac-fr-049-2 | Lần chạy quá hạn quá 24 giờ chuyển MISSED, phát ScheduledRunFailed với reasonCode MISSED và không có giao dịch | Tiêu chí chấp nhận AC-FR-049-2 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đưa hệ thống về trạng thái ban đầu, đảm bảo dữ liệu nền và các stub phụ thuộc bên ngoài đã sẵn sàng. | — |  |
| 2 | when | Thực hiện luồng của FR-049 theo tiêu chí: lần chạy quá hạn hơn 24 giờ thì chuyển sang MISSED, phát ScheduledRunFailed với reasonCode MISSED và không tạo giao dịch. | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Lần chạy quá hạn quá 24 giờ chuyển MISSED, phát ScheduledRunFailed với reasonCode MISSED và không có giao dịch |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-049-SCN-SCN-ScheduledRun: Kiểm tra toàn bộ luồng chính SCN-ScheduledRun, từ lúc hệ thống quét các lần chạy đến hạn cho tới khi ghi nhận audit và sự kiện.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-049-SCN-SCN-ScheduledRun |
| Mục tiêu | Kiểm tra toàn bộ luồng chính SCN-ScheduledRun, từ lúc hệ thống quét các lần chạy đến hạn cho tới khi ghi nhận audit và sự kiện. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-049 · TD TD-SCHEDULED-TRANSFER-L2-FR-049-SCN · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-049-SCN-SCN-ScheduledRun · nguồn FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-049-SCN |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| SCN-ScheduledRun | scn-scheduledrun | 8 | Luồng chính SCN-ScheduledRun |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo hệ thống và các Module tham gia ở trạng thái ban đầu, và stub hệ thống ngoài trả về kết quả thành công. | MOD-API, MOD-Scheduler, MOD-Transfer |  |
| 2 | when | Hệ thống quét các lần chạy đến hạn mỗi 30 giây. | — |  |
| 3 | when | Hệ thống chọn tối đa 20 lần chạy đến hạn. | — |  |
| 4 | when | Hệ thống chiếm lần chạy bằng câu lệnh UPDATE có điều kiện. | — |  |
| 5 | when | Hệ thống sinh correlationId. | — |  |
| 6 | when | Hệ thống gọi lõi chuyển tiền, gồm kiểm tra idempotency, hạn mức và số dư, Fraud Check, ghi sổ và ghi outbox. | — |  |
| 7 | when | Hệ thống cập nhật trạng thái của lần chạy. | — |  |
| 8 | when | Hệ thống xử lý thử lại hoặc chuyển lần chạy sang trạng thái FAILED khi cần. | — |  |
| 9 | when | Hệ thống ghi audit và phát sự kiện. | — |  |
| 10 | then | Xác nhận luồng đã hoàn tất và kiểm tra trạng thái cuối ở mọi Module tham gia. | MOD-API, MOD-Scheduler, MOD-Transfer |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | hoàn tất |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-NFR-010-NFR-AT: Xác nhận hệ thống đạt ngưỡng NFR-010 khi giá trị đo đúng bằng ngưỡng 60 s.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-NFR-010-NFR-AT |
| Mục tiêu | Xác nhận hệ thống đạt ngưỡng NFR-010 khi giá trị đo đúng bằng ngưỡng 60 s. |
| Mức, loại, ưu tiên | L2 · performance · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-NFR-010 · TD TD-SCHEDULED-TRANSFER-L2-NFR-010-NFR · TCOV TCOV-SCHEDULED-TRANSFER-L2-NFR-010-NFR-AT · nguồn NFR-010 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-NFR-010-NFR |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| NFR-010 | at | 60 | Đúng ngưỡng 60 s |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Cho hệ thống hoạt động trong điều kiện đo lường theo yêu cầu phi chức năng (NFR). | — |  |
| 2 | when | Thực hiện đo NFR-010 ở mức 60 s. | — | 60 |
| 3 | then | So sánh kết quả đo được với ngưỡng quy định. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | latency | <= | 60 | s |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-NFR-010-NFR-OVER: Xác nhận hệ thống không đạt NFR-010 khi giá trị đo vượt ngưỡng một đơn vị (60+1).

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-NFR-010-NFR-OVER |
| Mục tiêu | Xác nhận hệ thống không đạt NFR-010 khi giá trị đo vượt ngưỡng một đơn vị (60+1). |
| Mức, loại, ưu tiên | L2 · performance · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-NFR-010 · TD TD-SCHEDULED-TRANSFER-L2-NFR-010-NFR · TCOV TCOV-SCHEDULED-TRANSFER-L2-NFR-010-NFR-OVER · nguồn NFR-010 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-NFR-010-NFR |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| NFR-010 | over | 61 | Vượt ngưỡng (60+1) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Cho hệ thống hoạt động trong điều kiện đo lường theo yêu cầu phi chức năng (NFR). | — |  |
| 2 | when | Thực hiện đo NFR-010 ở mức 61 s. | — | 61 |
| 3 | then | So sánh kết quả đo được với ngưỡng quy định. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | latency | > | 60 | s |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-NFR-011-NFR-AT: Xác nhận hệ thống đạt ngưỡng NFR-011: số giao dịch trùng đo được đúng bằng ngưỡng cho phép là 0.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-NFR-011-NFR-AT |
| Mục tiêu | Xác nhận hệ thống đạt ngưỡng NFR-011: số giao dịch trùng đo được đúng bằng ngưỡng cho phép là 0. |
| Mức, loại, ưu tiên | L2 · performance · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-NFR-011 · TD TD-SCHEDULED-TRANSFER-L2-NFR-011-NFR · TCOV TCOV-SCHEDULED-TRANSFER-L2-NFR-011-NFR-AT · nguồn NFR-011 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-NFR-011-NFR |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| NFR-011 | at | 0 | Đúng ngưỡng 0 giao dịch trùng |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Cho hệ thống vận hành trong điều kiện đo theo NFR. | — |  |
| 2 | when | Đo NFR-011 và ghi nhận kết quả ở mức 0 giao dịch trùng. | — | 0 |
| 3 | then | So sánh kết quả đo được với ngưỡng của NFR-011. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | record | <= | 0 | giao dịch trùng |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-NFR-011-NFR-OVER: Xác nhận hệ thống vượt ngưỡng NFR-011 khi số giao dịch trùng đo được là 1, tức vượt ngưỡng 0 thêm 1 (0+1).

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-NFR-011-NFR-OVER |
| Mục tiêu | Xác nhận hệ thống vượt ngưỡng NFR-011 khi số giao dịch trùng đo được là 1, tức vượt ngưỡng 0 thêm 1 (0+1). |
| Mức, loại, ưu tiên | L2 · performance · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-NFR-011 · TD TD-SCHEDULED-TRANSFER-L2-NFR-011-NFR · TCOV TCOV-SCHEDULED-TRANSFER-L2-NFR-011-NFR-OVER · nguồn NFR-011 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-NFR-011-NFR |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| NFR-011 | over | 1 | Vượt ngưỡng (0+1) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Cho hệ thống vận hành trong điều kiện đo theo NFR. | — |  |
| 2 | when | Đo NFR-011 và ghi nhận kết quả ở mức 1 giao dịch trùng. | — | 1 |
| 3 | then | So sánh kết quả đo được với ngưỡng của NFR-011. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | record | > | 0 | giao dịch trùng |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-NFR-012-NFR-AT: Xác nhận hệ thống đạt ngưỡng NFR-012 khi số chu kỳ quét đúng bằng ngưỡng quy định là 5 chu kỳ.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-NFR-012-NFR-AT |
| Mục tiêu | Xác nhận hệ thống đạt ngưỡng NFR-012 khi số chu kỳ quét đúng bằng ngưỡng quy định là 5 chu kỳ. |
| Mức, loại, ưu tiên | L2 · performance · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-NFR-012 · TD TD-SCHEDULED-TRANSFER-L2-NFR-012-NFR · TCOV TCOV-SCHEDULED-TRANSFER-L2-NFR-012-NFR-AT · nguồn NFR-012 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo mục STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-NFR-012-NFR |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| NFR-012 | at | 5 | Đúng ngưỡng 5 chu kỳ quét |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đặt hệ thống vào điều kiện đo theo yêu cầu phi chức năng (NFR). | — |  |
| 2 | when | Thực hiện đo NFR-012 ở mức 5 chu kỳ quét. | — | 5 |
| 3 | then | So sánh kết quả đo được với ngưỡng quy định. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | record | <= | 5 | chu kỳ quét |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-NFR-012-NFR-OVER: Xác nhận hệ thống được đánh giá đúng khi vượt ngưỡng NFR-012, với số chu kỳ quét là 6, tức vượt ngưỡng 5 thêm 1 chu kỳ.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-NFR-012-NFR-OVER |
| Mục tiêu | Xác nhận hệ thống được đánh giá đúng khi vượt ngưỡng NFR-012, với số chu kỳ quét là 6, tức vượt ngưỡng 5 thêm 1 chu kỳ. |
| Mức, loại, ưu tiên | L2 · performance · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-NFR-012 · TD TD-SCHEDULED-TRANSFER-L2-NFR-012-NFR · TCOV TCOV-SCHEDULED-TRANSFER-L2-NFR-012-NFR-OVER · nguồn NFR-012 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo mục STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-NFR-012-NFR |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| NFR-012 | over | 6 | Vượt ngưỡng (5+1) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đặt hệ thống vào điều kiện đo theo yêu cầu phi chức năng (NFR). | — |  |
| 2 | when | Thực hiện đo NFR-012 ở mức 6 chu kỳ quét. | — | 6 |
| 3 | then | So sánh kết quả đo được với ngưỡng quy định. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | record | > | 5 | chu kỳ quét |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-NFR-013-NFR-AT: Kiểm tra hệ thống đạt ngưỡng NFR-013 khi giá trị đo đúng bằng ngưỡng 500 ms.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-NFR-013-NFR-AT |
| Mục tiêu | Kiểm tra hệ thống đạt ngưỡng NFR-013 khi giá trị đo đúng bằng ngưỡng 500 ms. |
| Mức, loại, ưu tiên | L2 · performance · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-NFR-013 · TD TD-SCHEDULED-TRANSFER-L2-NFR-013-NFR · TCOV TCOV-SCHEDULED-TRANSFER-L2-NFR-013-NFR-AT · nguồn NFR-013 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-NFR-013-NFR |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| NFR-013 | at | 500 | Đúng ngưỡng 500 ms |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Cho hệ thống hoạt động trong điều kiện đo theo yêu cầu phi chức năng (NFR). | — |  |
| 2 | when | Thực hiện đo NFR-013 ở mức 500 ms. | — | 500 |
| 3 | then | So sánh kết quả đo được với ngưỡng quy định. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | latency | <= | 500 | ms |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-NFR-013-NFR-OVER: Kiểm tra hệ thống khi giá trị đo vượt ngưỡng NFR-013 một đơn vị (500+1), tức 501 ms.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-NFR-013-NFR-OVER |
| Mục tiêu | Kiểm tra hệ thống khi giá trị đo vượt ngưỡng NFR-013 một đơn vị (500+1), tức 501 ms. |
| Mức, loại, ưu tiên | L2 · performance · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-NFR-013 · TD TD-SCHEDULED-TRANSFER-L2-NFR-013-NFR · TCOV TCOV-SCHEDULED-TRANSFER-L2-NFR-013-NFR-OVER · nguồn NFR-013 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-NFR-013-NFR |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| NFR-013 | over | 501 | Vượt ngưỡng (500+1) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Cho hệ thống hoạt động trong điều kiện đo theo yêu cầu phi chức năng (NFR). | — |  |
| 2 | when | Thực hiện đo NFR-013 ở mức 501 ms. | — | 501 |
| 3 | then | So sánh kết quả đo được với ngưỡng quy định. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | latency | > | 500 | ms |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-NFR-014-NFR-AT: Kiểm tra hệ thống đạt đúng ngưỡng NFR-014 ở mức 100 %.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-NFR-014-NFR-AT |
| Mục tiêu | Kiểm tra hệ thống đạt đúng ngưỡng NFR-014 ở mức 100 %. |
| Mức, loại, ưu tiên | L2 · performance · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-NFR-014 · TD TD-SCHEDULED-TRANSFER-L2-NFR-014-NFR · TCOV TCOV-SCHEDULED-TRANSFER-L2-NFR-014-NFR-AT · nguồn NFR-014 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-NFR-014-NFR |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| NFR-014 | at | 100 | Đúng ngưỡng 100 % |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Cho hệ thống chạy trong điều kiện đo lường theo yêu cầu phi chức năng (NFR). | — |  |
| 2 | when | Đo NFR-014 ở mức 100 %. | — | 100 |
| 3 | then | So sánh kết quả đo được với ngưỡng quy định. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | record | <= | 100 | % |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-NFR-014-NFR-OVER: Kiểm tra hệ thống khi vượt ngưỡng NFR-014 (100+1), tức ở mức 101 %.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-NFR-014-NFR-OVER |
| Mục tiêu | Kiểm tra hệ thống khi vượt ngưỡng NFR-014 (100+1), tức ở mức 101 %. |
| Mức, loại, ưu tiên | L2 · performance · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-NFR-014 · TD TD-SCHEDULED-TRANSFER-L2-NFR-014-NFR · TCOV TCOV-SCHEDULED-TRANSFER-L2-NFR-014-NFR-OVER · nguồn NFR-014 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-NFR-014-NFR |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| NFR-014 | over | 101 | Vượt ngưỡng (100+1) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Cho hệ thống chạy trong điều kiện đo lường theo yêu cầu phi chức năng (NFR). | — |  |
| 2 | when | Đo NFR-014 ở mức 101 %. | — | 101 |
| 3 | then | So sánh kết quả đo được với ngưỡng quy định. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | record | > | 100 | % |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-ACTIVE-PATCH-status-ACTIVE: Kiểm tra hệ thống từ chối PATCH status=ACTIVE khi thực thể đang ở trạng thái ACTIVE.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-ACTIVE-PATCH-status-ACTIVE |
| Mục tiêu | Kiểm tra hệ thống từ chối PATCH status=ACTIVE khi thực thể đang ở trạng thái ACTIVE. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-ACTIVE-PATCH-status-ACTIVE · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo phần STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo thực thể đang ở trạng thái ACTIVE. | — |  |
| 2 | when | Gửi yêu cầu PATCH với status=ACTIVE. | — |  |
| 3 | then | Xác nhận yêu cầu bị từ chối và trạng thái của thực thể không thay đổi. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-DELETE: Kiểm tra hệ thống từ chối DELETE khi thực thể đang ở trạng thái CANCELLED.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-DELETE |
| Mục tiêu | Kiểm tra hệ thống từ chối DELETE khi thực thể đang ở trạng thái CANCELLED. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-CANCELLED-DELETE · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo phần STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| CANCELLED/DELETE | inv-cancelled-delete | invalid | DELETE từ trạng thái CANCELLED không được phép |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo thực thể đang ở trạng thái CANCELLED. | — |  |
| 2 | when | Gửi yêu cầu DELETE. | — |  |
| 3 | then | Xác nhận yêu cầu bị từ chối và trạng thái của thực thể không thay đổi. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-PATCH-status-ACTIVE: Kiểm tra hệ thống từ chối PATCH status=ACTIVE khi thực thể đang ở trạng thái CANCELLED.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-PATCH-status-ACTIVE |
| Mục tiêu | Kiểm tra hệ thống từ chối PATCH status=ACTIVE khi thực thể đang ở trạng thái CANCELLED. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-CANCELLED-PATCH-status-ACTIVE · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo phần STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo thực thể đang ở trạng thái CANCELLED. | — |  |
| 2 | when | Gửi yêu cầu PATCH với status=ACTIVE. | — |  |
| 3 | then | Xác nhận yêu cầu bị từ chối và trạng thái của thực thể không thay đổi. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-PATCH-status-PAUSED: Kiểm tra hệ thống từ chối PATCH status=PAUSED khi thực thể đang ở trạng thái CANCELLED.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-PATCH-status-PAUSED |
| Mục tiêu | Kiểm tra hệ thống từ chối PATCH status=PAUSED khi thực thể đang ở trạng thái CANCELLED. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-CANCELLED-PATCH-status-PAUSED · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo phần STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo thực thể đang ở trạng thái CANCELLED. | — |  |
| 2 | when | Gửi yêu cầu PATCH với status=PAUSED. | — |  |
| 3 | then | Xác nhận yêu cầu bị từ chối và trạng thái của thực thể không thay đổi. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-l-n-ch-y-cu-i-k-t-th-c: Kiểm tra hệ thống từ chối sự kiện lần chạy cuối kết thúc khi thực thể đang ở trạng thái CANCELLED.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-l-n-ch-y-cu-i-k-t-th-c |
| Mục tiêu | Kiểm tra hệ thống từ chối sự kiện lần chạy cuối kết thúc khi thực thể đang ở trạng thái CANCELLED. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-CANCELLED-l-n-ch-y-cu-i-k-t-th-c · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo phần STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo thực thể đang ở trạng thái CANCELLED. | — |  |
| 2 | when | Kích hoạt sự kiện lần chạy cuối kết thúc. | — |  |
| 3 | then | Xác nhận yêu cầu bị từ chối và trạng thái của thực thể không thay đổi. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-scheduler-l-n-ch-y-FAILED-th-3-li-n-ti-p: Kiểm tra hệ thống từ chối sự kiện scheduler ghi nhận lần chạy FAILED thứ 3 liên tiếp khi thực thể đang ở trạng thái CANCELLED.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-scheduler-l-n-ch-y-FAILED-th-3-li-n-ti-p |
| Mục tiêu | Kiểm tra hệ thống từ chối sự kiện scheduler ghi nhận lần chạy FAILED thứ 3 liên tiếp khi thực thể đang ở trạng thái CANCELLED. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-CANCELLED-scheduler-l-n-ch-y-FAILED-th-3-li-n-ti-p · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo phần STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo thực thể đang ở trạng thái CANCELLED. | — |  |
| 2 | when | Kích hoạt sự kiện scheduler ghi nhận lần chạy FAILED thứ 3 liên tiếp. | — |  |
| 3 | then | Xác nhận yêu cầu bị từ chối và trạng thái của thực thể không thay đổi. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-COMPLETED-DELETE: Kiểm tra hệ thống từ chối DELETE khi thực thể đang ở trạng thái COMPLETED.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-COMPLETED-DELETE |
| Mục tiêu | Kiểm tra hệ thống từ chối DELETE khi thực thể đang ở trạng thái COMPLETED. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-COMPLETED-DELETE · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo phần STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| COMPLETED/DELETE | inv-completed-delete | invalid | DELETE từ trạng thái COMPLETED không được phép |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo thực thể đang ở trạng thái COMPLETED. | — |  |
| 2 | when | Gửi yêu cầu DELETE. | — |  |
| 3 | then | Xác nhận yêu cầu bị từ chối và trạng thái của thực thể không thay đổi. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-COMPLETED-PATCH-status-ACTIVE: Kiểm tra hệ thống từ chối PATCH status=ACTIVE khi thực thể đang ở trạng thái COMPLETED.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-COMPLETED-PATCH-status-ACTIVE |
| Mục tiêu | Kiểm tra hệ thống từ chối PATCH status=ACTIVE khi thực thể đang ở trạng thái COMPLETED. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-COMPLETED-PATCH-status-ACTIVE · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo phần STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo thực thể đang ở trạng thái COMPLETED. | — |  |
| 2 | when | Gửi yêu cầu PATCH với status=ACTIVE. | — |  |
| 3 | then | Xác nhận yêu cầu bị từ chối và trạng thái của thực thể không thay đổi. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-COMPLETED-PATCH-status-PAUSED: Kiểm tra hệ thống từ chối PATCH status=PAUSED khi thực thể đang ở trạng thái COMPLETED.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-COMPLETED-PATCH-status-PAUSED |
| Mục tiêu | Kiểm tra hệ thống từ chối PATCH status=PAUSED khi thực thể đang ở trạng thái COMPLETED. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-COMPLETED-PATCH-status-PAUSED · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo phần STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo thực thể đang ở trạng thái COMPLETED. | — |  |
| 2 | when | Gửi yêu cầu PATCH với status=PAUSED. | — |  |
| 3 | then | Xác nhận yêu cầu bị từ chối và trạng thái của thực thể không thay đổi. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-COMPLETED-l-n-ch-y-cu-i-k-t-th-c: Kiểm tra hệ thống từ chối sự kiện lần chạy cuối kết thúc khi thực thể đang ở trạng thái COMPLETED.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-COMPLETED-l-n-ch-y-cu-i-k-t-th-c |
| Mục tiêu | Kiểm tra hệ thống từ chối sự kiện lần chạy cuối kết thúc khi thực thể đang ở trạng thái COMPLETED. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-COMPLETED-l-n-ch-y-cu-i-k-t-th-c · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo phần STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo thực thể đang ở trạng thái COMPLETED. | — |  |
| 2 | when | Kích hoạt sự kiện lần chạy cuối kết thúc. | — |  |
| 3 | then | Xác nhận yêu cầu bị từ chối và trạng thái của thực thể không thay đổi. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-1: Kiểm tra thực thể chuyển từ trạng thái ACTIVE sang PAUSED.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-1 |
| Mục tiêu | Kiểm tra thực thể chuyển từ trạng thái ACTIVE sang PAUSED. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-TRN-Schedule-1 · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo phần STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| TRN-Schedule-1 | trn-schedule-1 | ACTIVE->PAUSED | ACTIVE -> PAUSED khi PATCH status=PAUSED |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo thực thể đang ở trạng thái ACTIVE. | — |  |
| 2 | when | Gửi PATCH với status=PAUSED để chuyển thực thể từ ACTIVE sang PAUSED. | — |  |
| 3 | then | Xác nhận thực thể đã ở trạng thái PAUSED. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | PAUSED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-2: Kiểm tra thực thể chuyển từ trạng thái PAUSED sang ACTIVE.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-2 |
| Mục tiêu | Kiểm tra thực thể chuyển từ trạng thái PAUSED sang ACTIVE. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-TRN-Schedule-2 · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo phần STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| TRN-Schedule-2 | trn-schedule-2 | PAUSED->ACTIVE | PAUSED -> ACTIVE khi PATCH status=ACTIVE |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo thực thể đang ở trạng thái PAUSED. | — |  |
| 2 | when | Gửi PATCH với status=ACTIVE để chuyển thực thể từ PAUSED sang ACTIVE. | — |  |
| 3 | then | Xác nhận thực thể đã ở trạng thái ACTIVE. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | ACTIVE |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-3: Kiểm tra thực thể chuyển từ ACTIVE sang PAUSED khi scheduler ghi nhận lần chạy FAILED thứ 3 liên tiếp.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-3 |
| Mục tiêu | Kiểm tra thực thể chuyển từ ACTIVE sang PAUSED khi scheduler ghi nhận lần chạy FAILED thứ 3 liên tiếp. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-TRN-Schedule-3 · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo phần STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| TRN-Schedule-3 | trn-schedule-3 | ACTIVE->PAUSED | ACTIVE -> PAUSED khi scheduler: lần chạy FAILED thứ 3 liên tiếp |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo thực thể đang ở trạng thái ACTIVE. | — |  |
| 2 | when | Để scheduler có lần chạy FAILED thứ 3 liên tiếp, qua đó chuyển thực thể từ ACTIVE sang PAUSED. | — |  |
| 3 | then | Xác nhận thực thể đã ở trạng thái PAUSED. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | PAUSED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-4: Kiểm tra thực thể chuyển từ trạng thái ACTIVE sang COMPLETED khi lần chạy cuối kết thúc.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-4 |
| Mục tiêu | Kiểm tra thực thể chuyển từ trạng thái ACTIVE sang COMPLETED khi lần chạy cuối kết thúc. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-TRN-Schedule-4 · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo phần STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| TRN-Schedule-4 | trn-schedule-4 | ACTIVE->COMPLETED | ACTIVE -> COMPLETED khi lần chạy cuối kết thúc |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo thực thể đang ở trạng thái ACTIVE. | — |  |
| 2 | when | Cho lần chạy cuối kết thúc để thực thể chuyển từ ACTIVE sang COMPLETED. | — |  |
| 3 | then | Xác nhận thực thể đã ở trạng thái COMPLETED. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | COMPLETED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-5: Kiểm tra thực thể chuyển từ trạng thái ACTIVE sang CANCELLED.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-5 |
| Mục tiêu | Kiểm tra thực thể chuyển từ trạng thái ACTIVE sang CANCELLED. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-TRN-Schedule-5 · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo phần STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| TRN-Schedule-5 | trn-schedule-5 | ACTIVE->CANCELLED | ACTIVE -> CANCELLED khi DELETE |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo thực thể đang ở trạng thái ACTIVE. | — |  |
| 2 | when | Gửi yêu cầu DELETE để chuyển thực thể từ ACTIVE sang CANCELLED. | — |  |
| 3 | then | Xác nhận thực thể đã ở trạng thái CANCELLED. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | CANCELLED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-6: Kiểm tra thực thể chuyển từ trạng thái PAUSED sang CANCELLED.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-6 |
| Mục tiêu | Kiểm tra thực thể chuyển từ trạng thái PAUSED sang CANCELLED. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-TRN-Schedule-6 · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo phần STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| TRN-Schedule-6 | trn-schedule-6 | PAUSED->CANCELLED | PAUSED -> CANCELLED khi DELETE |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo thực thể đang ở trạng thái PAUSED. | — |  |
| 2 | when | Gửi yêu cầu DELETE để chuyển thực thể từ PAUSED sang CANCELLED. | — |  |
| 3 | then | Xác nhận thực thể đã ở trạng thái CANCELLED. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | CANCELLED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-7: Kiểm tra thực thể chuyển từ trạng thái PAUSED sang COMPLETED.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-7 |
| Mục tiêu | Kiểm tra thực thể chuyển từ trạng thái PAUSED sang COMPLETED. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-TRN-Schedule-7 · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo phần STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| TRN-Schedule-7 | trn-schedule-7 | PAUSED->COMPLETED | PAUSED -> COMPLETED khi PATCH status=ACTIVE |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Đảm bảo thực thể đang ở trạng thái PAUSED. | — |  |
| 2 | when | Gửi PATCH với status=ACTIVE để chuyển thực thể từ PAUSED sang COMPLETED. | — |  |
| 3 | then | Xác nhận thực thể đã ở trạng thái COMPLETED. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | COMPLETED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED--u-chu-k-qu-t: Kiểm tra hệ thống từ chối yêu cầu không hợp lệ khi thực thể đang ở trạng thái không hợp lệ cho yêu cầu đó.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED--u-chu-k-qu-t |
| Mục tiêu | Kiểm tra hệ thống từ chối yêu cầu không hợp lệ khi thực thể đang ở trạng thái không hợp lệ cho yêu cầu đó. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED--u-chu-k-qu-t · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Xác nhận thực thể đang ở trạng thái None. | — |  |
| 2 | when | Gửi yêu cầu None. | — |  |
| 3 | then | Xác nhận yêu cầu bị từ chối và trạng thái không thay đổi. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-DELETE-l-ch: Kiểm tra hệ thống từ chối yêu cầu không hợp lệ khi thực thể đang ở trạng thái không hợp lệ cho yêu cầu đó.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-DELETE-l-ch |
| Mục tiêu | Kiểm tra hệ thống từ chối yêu cầu không hợp lệ khi thực thể đang ở trạng thái không hợp lệ cho yêu cầu đó. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED-DELETE-l-ch · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Xác nhận thực thể đang ở trạng thái None. | — |  |
| 2 | when | Gửi yêu cầu None. | — |  |
| 3 | then | Xác nhận yêu cầu bị từ chối và trạng thái không thay đổi. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-l-i-chuy-n-ti-n-tr-giao-d-ch-COMPLETED-ho-c-ph-t-l-i: Kiểm tra hệ thống từ chối yêu cầu không hợp lệ khi thực thể đang ở trạng thái không hợp lệ cho yêu cầu đó.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-l-i-chuy-n-ti-n-tr-giao-d-ch-COMPLETED-ho-c-ph-t-l-i |
| Mục tiêu | Kiểm tra hệ thống từ chối yêu cầu không hợp lệ khi thực thể đang ở trạng thái không hợp lệ cho yêu cầu đó. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED-l-i-chuy-n-ti-n-tr-giao-d-ch-COMPLETED-ho-c-ph-t-l-i · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Xác nhận thực thể đang ở trạng thái None. | — |  |
| 2 | when | Gửi yêu cầu None. | — |  |
| 3 | then | Xác nhận yêu cầu bị từ chối và trạng thái không thay đổi. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-l-i-tr-l-i: Kiểm tra hệ thống từ chối yêu cầu không hợp lệ khi thực thể đang ở trạng thái không hợp lệ cho yêu cầu đó.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-l-i-tr-l-i |
| Mục tiêu | Kiểm tra hệ thống từ chối yêu cầu không hợp lệ khi thực thể đang ở trạng thái không hợp lệ cho yêu cầu đó. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED-l-i-tr-l-i · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Xác nhận thực thể đang ở trạng thái None. | — |  |
| 2 | when | Gửi yêu cầu None. | — |  |
| 3 | then | Xác nhận yêu cầu bị từ chối và trạng thái không thay đổi. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-l-i-tr-l-i-t-m-th-i-ho-c-thi-u-s-d: Kiểm tra hệ thống từ chối yêu cầu không hợp lệ khi thực thể đang ở trạng thái không hợp lệ cho yêu cầu đó.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-l-i-tr-l-i-t-m-th-i-ho-c-thi-u-s-d |
| Mục tiêu | Kiểm tra hệ thống từ chối yêu cầu không hợp lệ khi thực thể đang ở trạng thái không hợp lệ cho yêu cầu đó. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED-l-i-tr-l-i-t-m-th-i-ho-c-thi-u-s-d · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Xác nhận thực thể đang ở trạng thái None. | — |  |
| 2 | when | Gửi yêu cầu None. | — |  |
| 3 | then | Xác nhận yêu cầu bị từ chối và trạng thái không thay đổi. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-scheduler-chi-m-l-n-ch-y: Kiểm tra hệ thống từ chối yêu cầu không hợp lệ khi thực thể đang ở trạng thái không hợp lệ cho yêu cầu đó.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-scheduler-chi-m-l-n-ch-y |
| Mục tiêu | Kiểm tra hệ thống từ chối yêu cầu không hợp lệ khi thực thể đang ở trạng thái không hợp lệ cho yêu cầu đó. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED-scheduler-chi-m-l-n-ch-y · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Xác nhận thực thể đang ở trạng thái None. | — |  |
| 2 | when | Gửi yêu cầu None. | — |  |
| 3 | then | Xác nhận yêu cầu bị từ chối và trạng thái không thay đổi. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-scheduler-ph-c-h-i-l-n-ch-y-k-t: Kiểm tra hệ thống từ chối yêu cầu không hợp lệ khi thực thể đang ở trạng thái không hợp lệ cho yêu cầu đó.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-scheduler-ph-c-h-i-l-n-ch-y-k-t |
| Mục tiêu | Kiểm tra hệ thống từ chối yêu cầu không hợp lệ khi thực thể đang ở trạng thái không hợp lệ cho yêu cầu đó. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED-scheduler-ph-c-h-i-l-n-ch-y-k-t · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Xác nhận thực thể đang ở trạng thái None. | — |  |
| 2 | when | Gửi yêu cầu None. | — |  |
| 3 | then | Xác nhận yêu cầu bị từ chối và trạng thái không thay đổi. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-ti-p-t-c-l-ch-PAUSED: Kiểm tra hệ thống từ chối yêu cầu không hợp lệ khi thực thể đang ở trạng thái không hợp lệ cho yêu cầu đó.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-ti-p-t-c-l-ch-PAUSED |
| Mục tiêu | Kiểm tra hệ thống từ chối yêu cầu không hợp lệ khi thực thể đang ở trạng thái không hợp lệ cho yêu cầu đó. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED-ti-p-t-c-l-ch-PAUSED · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Xác nhận thực thể đang ở trạng thái None. | — |  |
| 2 | when | Gửi yêu cầu None. | — |  |
| 3 | then | Xác nhận yêu cầu bị từ chối và trạng thái không thay đổi. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-FAILED-DELETE-l-ch: Kiểm tra hệ thống từ chối yêu cầu không hợp lệ khi thực thể đang ở trạng thái không hợp lệ cho yêu cầu đó.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-FAILED-DELETE-l-ch |
| Mục tiêu | Kiểm tra hệ thống từ chối yêu cầu không hợp lệ khi thực thể đang ở trạng thái không hợp lệ cho yêu cầu đó. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-FAILED-DELETE-l-ch · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Xác nhận thực thể đang ở trạng thái None. | — |  |
| 2 | when | Gửi yêu cầu None. | — |  |
| 3 | then | Xác nhận yêu cầu bị từ chối và trạng thái không thay đổi. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-FAILED-l-i-chuy-n-ti-n-tr-giao-d-ch-COMPLETED-ho-c-ph-t-l-i: Kiểm tra hệ thống từ chối yêu cầu không hợp lệ khi thực thể đang ở trạng thái không hợp lệ cho yêu cầu đó.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-FAILED-l-i-chuy-n-ti-n-tr-giao-d-ch-COMPLETED-ho-c-ph-t-l-i |
| Mục tiêu | Kiểm tra hệ thống từ chối yêu cầu không hợp lệ khi thực thể đang ở trạng thái không hợp lệ cho yêu cầu đó. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-FAILED-l-i-chuy-n-ti-n-tr-giao-d-ch-COMPLETED-ho-c-ph-t-l-i · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Xác nhận thực thể đang ở trạng thái None. | — |  |
| 2 | when | Gửi yêu cầu None. | — |  |
| 3 | then | Xác nhận yêu cầu bị từ chối và trạng thái không thay đổi. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-1: Kiểm tra lần chạy chuyển từ trạng thái PENDING sang RUNNING.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-1 |
| Mục tiêu | Kiểm tra lần chạy chuyển từ trạng thái PENDING sang RUNNING. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-1 · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| TRN-ScheduledRun-1 | trn-scheduledrun-1 | PENDING->RUNNING | PENDING -> RUNNING khi scheduler chiếm lần chạy |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Xác nhận thực thể đang ở trạng thái PENDING. | — |  |
| 2 | when | Để scheduler chiếm lần chạy, khi đó trạng thái chuyển từ PENDING sang RUNNING. | — |  |
| 3 | then | Xác nhận thực thể đang ở trạng thái RUNNING. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | RUNNING |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-2: Kiểm tra lần chạy chuyển từ trạng thái RUNNING sang SUCCEEDED.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-2 |
| Mục tiêu | Kiểm tra lần chạy chuyển từ trạng thái RUNNING sang SUCCEEDED. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-2 · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| TRN-ScheduledRun-2 | trn-scheduledrun-2 | RUNNING->SUCCEEDED | RUNNING -> SUCCEEDED khi lõi chuyển tiền trả giao dịch COMPLETED (hoặc phát lại) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Xác nhận thực thể đang ở trạng thái RUNNING. | — |  |
| 2 | when | Khi lõi chuyển tiền trả về giao dịch COMPLETED (hoặc phát lại), trạng thái chuyển từ RUNNING sang SUCCEEDED. | — |  |
| 3 | then | Xác nhận thực thể đang ở trạng thái SUCCEEDED. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | SUCCEEDED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-3: Kiểm tra lần chạy chuyển từ trạng thái RUNNING về PENDING.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-3 |
| Mục tiêu | Kiểm tra lần chạy chuyển từ trạng thái RUNNING về PENDING. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-3 · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| TRN-ScheduledRun-3 | trn-scheduledrun-3 | RUNNING->PENDING | RUNNING -> PENDING khi lõi trả lỗi tạm thời hoặc thiếu số dư |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Xác nhận thực thể đang ở trạng thái RUNNING. | — |  |
| 2 | when | Khi lõi trả về lỗi tạm thời hoặc báo thiếu số dư, trạng thái chuyển từ RUNNING về PENDING. | — |  |
| 3 | then | Xác nhận thực thể đang ở trạng thái PENDING. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | PENDING |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-4: Kiểm tra lần chạy chuyển từ trạng thái RUNNING sang FAILED.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-4 |
| Mục tiêu | Kiểm tra lần chạy chuyển từ trạng thái RUNNING sang FAILED. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-4 · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| TRN-ScheduledRun-4 | trn-scheduledrun-4 | RUNNING->FAILED | RUNNING -> FAILED khi lõi trả lỗi |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Xác nhận thực thể đang ở trạng thái RUNNING. | — |  |
| 2 | when | Khi lõi trả về lỗi, trạng thái chuyển từ RUNNING sang FAILED. | — |  |
| 3 | then | Xác nhận thực thể đang ở trạng thái FAILED. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | FAILED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-5: Kiểm tra lần chạy chuyển từ trạng thái PENDING sang SKIPPED.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-5 |
| Mục tiêu | Kiểm tra lần chạy chuyển từ trạng thái PENDING sang SKIPPED. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-5 · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| TRN-ScheduledRun-5 | trn-scheduledrun-5 | PENDING->SKIPPED | PENDING -> SKIPPED khi tiếp tục lịch PAUSED |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Xác nhận thực thể đang ở trạng thái PENDING. | — |  |
| 2 | when | Khi tiếp tục lịch đang ở trạng thái PAUSED, trạng thái chuyển từ PENDING sang SKIPPED. | — |  |
| 3 | then | Xác nhận thực thể đang ở trạng thái SKIPPED. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | SKIPPED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-6: Kiểm tra lần chạy chuyển từ trạng thái PENDING sang MISSED.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-6 |
| Mục tiêu | Kiểm tra lần chạy chuyển từ trạng thái PENDING sang MISSED. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-6 · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| TRN-ScheduledRun-6 | trn-scheduledrun-6 | PENDING->MISSED | PENDING -> MISSED khi đầu chu kỳ quét |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Xác nhận thực thể đang ở trạng thái PENDING. | — |  |
| 2 | when | Khi bắt đầu chu kỳ quét, trạng thái chuyển từ PENDING sang MISSED. | — |  |
| 3 | then | Xác nhận thực thể đang ở trạng thái MISSED. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | MISSED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-7: Kiểm tra lần chạy chuyển từ trạng thái PENDING sang CANCELLED.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-7 |
| Mục tiêu | Kiểm tra lần chạy chuyển từ trạng thái PENDING sang CANCELLED. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-7 · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| TRN-ScheduledRun-7 | trn-scheduledrun-7 | PENDING->CANCELLED | PENDING -> CANCELLED khi DELETE lịch |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Xác nhận thực thể đang ở trạng thái PENDING. | — |  |
| 2 | when | Khi gửi DELETE để xóa lịch, trạng thái chuyển từ PENDING sang CANCELLED. | — |  |
| 3 | then | Xác nhận thực thể đang ở trạng thái CANCELLED. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | CANCELLED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-8: Kiểm tra lần chạy chuyển từ trạng thái RUNNING về PENDING khi được phục hồi.

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-8 |
| Mục tiêu | Kiểm tra lần chạy chuyển từ trạng thái RUNNING về PENDING khi được phục hồi. |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-8 · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo STUB của TestCase. |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| TRN-ScheduledRun-8 | trn-scheduledrun-8 | RUNNING->PENDING | RUNNING -> PENDING khi scheduler phục hồi lần chạy kẹt |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Xác nhận thực thể đang ở trạng thái RUNNING. | — |  |
| 2 | when | Để scheduler phục hồi lần chạy bị kẹt, khi đó trạng thái chuyển từ RUNNING về PENDING. | — |  |
| 3 | then | Xác nhận thực thể đang ở trạng thái PENDING. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | PENDING |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

### 3.3 L3: Cô lập service

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-BRK-INT-01-RECLOSE: Kiểm tra circuit breaker của INT-01 thử gọi lại sau 30 giây.

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-BRK-INT-01-RECLOSE |
| Mục tiêu | Kiểm tra circuit breaker của INT-01 thử gọi lại sau 30 giây. |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-BRK-INT-01 · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-BRK-INT-01-RECLOSE · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo phần STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-errorTransient-503, STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-BRK-INT-01 |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| openDurationSec | reclose | 30 | sau 30 giây breaker thử lại |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Cấu hình stub trả về lỗi errorTransient với mã 503. | INT-01 |  |
| 2 | when | Cho MOD-Scheduler gọi INT-01 liên tiếp. | INT-01 |  |
| 3 | then | Quan sát trạng thái của circuit breaker. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | half-open |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-BRK-INT-01-TRIP: Kiểm tra circuit breaker của INT-01 chuyển sang trạng thái mở khi tỷ lệ lỗi đạt từ 100% trở lên sau 5 lượt gọi.

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-BRK-INT-01-TRIP |
| Mục tiêu | Kiểm tra circuit breaker của INT-01 chuyển sang trạng thái mở khi tỷ lệ lỗi đạt từ 100% trở lên sau 5 lượt gọi. |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-BRK-INT-01 · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-BRK-INT-01-TRIP · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo phần STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-errorTransient-503, STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-BRK-INT-01 |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| failureRatePercent | trip | 100 | tỷ lệ lỗi >= 100% sau 5 lượt thì breaker mở |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Cấu hình stub trả về lỗi errorTransient với mã 503. | INT-01 |  |
| 2 | when | Cho MOD-Scheduler gọi INT-01 liên tiếp. | INT-01 |  |
| 3 | then | Quan sát trạng thái của circuit breaker. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | open |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Account-FAIL: Kiểm tra MOD-Scheduler xử lý đúng khi MOD-Account gặp lỗi.

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Account-FAIL |
| Mục tiêu | Kiểm tra MOD-Scheduler xử lý đúng khi MOD-Account gặp lỗi. |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Account · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Account-FAIL · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-errorTransient-503, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Account |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| dependency | fail | MOD-Account | MOD-Account lỗi, MOD-Scheduler xử lý lỗi có kiểm soát |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Cấu hình stub trả về lỗi tạm thời (errorTransient) với mã 503. | MOD-Account |  |
| 2 | when | Cho MOD-Scheduler gọi sang MOD-Account. | MOD-Account |  |
| 3 | then | Kiểm tra kết quả và trạng thái của MOD-Scheduler. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | handled-error |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Account-OK: Kiểm tra MOD-Scheduler xử lý đúng khi MOD-Account hoạt động bình thường.

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Account-OK |
| Mục tiêu | Kiểm tra MOD-Scheduler xử lý đúng khi MOD-Account hoạt động bình thường. |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Account · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Account-OK · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-errorTransient-503, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Account |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| dependency | ok | MOD-Account | MOD-Scheduler gọi MOD-Account thành công |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Cấu hình stub trả về kết quả thành công (success). | MOD-Account |  |
| 2 | when | Cho MOD-Scheduler gọi sang MOD-Account. | MOD-Account |  |
| 3 | then | Kiểm tra kết quả và trạng thái của MOD-Scheduler. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | success |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Audit-FAIL: Kiểm tra MOD-Scheduler xử lý đúng khi module phụ thuộc MOD-Audit trả về lỗi.

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Audit-FAIL |
| Mục tiêu | Kiểm tra MOD-Scheduler xử lý đúng khi module phụ thuộc MOD-Audit trả về lỗi. |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Audit · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Audit-FAIL · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo phần STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-errorTransient-503, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Audit |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| dependency | fail | MOD-Audit | MOD-Audit lỗi, MOD-Scheduler xử lý lỗi có kiểm soát |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Cấu hình stub trả về lỗi tạm thời (errorTransient) với mã 503. | MOD-Audit |  |
| 2 | when | Cho MOD-Scheduler gọi MOD-Audit. | MOD-Audit |  |
| 3 | then | Kiểm tra kết quả trả về và trạng thái của MOD-Scheduler. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | handled-error |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Audit-OK: Kiểm tra MOD-Scheduler xử lý đúng khi module phụ thuộc MOD-Audit hoạt động bình thường.

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Audit-OK |
| Mục tiêu | Kiểm tra MOD-Scheduler xử lý đúng khi module phụ thuộc MOD-Audit hoạt động bình thường. |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Audit · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Audit-OK · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo phần STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-errorTransient-503, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Audit |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| dependency | ok | MOD-Audit | MOD-Scheduler gọi MOD-Audit thành công |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Cấu hình stub trả về kết quả thành công (success). | MOD-Audit |  |
| 2 | when | Cho MOD-Scheduler gọi MOD-Audit. | MOD-Audit |  |
| 3 | then | Kiểm tra kết quả trả về và trạng thái của MOD-Scheduler. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | success |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Risk-FAIL: Kiểm tra MOD-Scheduler xử lý đúng khi phụ thuộc MOD-Risk gặp lỗi.

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Risk-FAIL |
| Mục tiêu | Kiểm tra MOD-Scheduler xử lý đúng khi phụ thuộc MOD-Risk gặp lỗi. |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Risk · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Risk-FAIL · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo phần STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-errorTransient-503, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Risk |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| dependency | fail | MOD-Risk | MOD-Risk lỗi, MOD-Scheduler xử lý lỗi có kiểm soát |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Cấu hình stub trả về lỗi tạm thời (errorTransient) với mã 503. | MOD-Risk |  |
| 2 | when | Cho MOD-Scheduler gọi MOD-Risk. | MOD-Risk |  |
| 3 | then | Kiểm tra kết quả và trạng thái của MOD-Scheduler. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | handled-error |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Risk-OK: Kiểm tra MOD-Scheduler xử lý đúng khi phụ thuộc MOD-Risk hoạt động bình thường.

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Risk-OK |
| Mục tiêu | Kiểm tra MOD-Scheduler xử lý đúng khi phụ thuộc MOD-Risk hoạt động bình thường. |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Risk · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Risk-OK · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo phần STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-errorTransient-503, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Risk |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| dependency | ok | MOD-Risk | MOD-Scheduler gọi MOD-Risk thành công |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Cấu hình stub trả về kết quả thành công (success). | MOD-Risk |  |
| 2 | when | Cho MOD-Scheduler gọi MOD-Risk. | MOD-Risk |  |
| 3 | then | Kiểm tra kết quả và trạng thái của MOD-Scheduler. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | success |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Transfer-FAIL: Kiểm tra cách MOD-Scheduler xử lý khi phụ thuộc MOD-Transfer trả về lỗi.

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Transfer-FAIL |
| Mục tiêu | Kiểm tra cách MOD-Scheduler xử lý khi phụ thuộc MOD-Transfer trả về lỗi. |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Transfer · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Transfer-FAIL · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo mục STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-errorTransient-503, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Transfer |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| dependency | fail | MOD-Transfer | MOD-Transfer lỗi, MOD-Scheduler xử lý lỗi có kiểm soát |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Cấu hình stub trả về lỗi tạm thời (errorTransient) với mã 503. | MOD-Transfer |  |
| 2 | when | Cho MOD-Scheduler gọi MOD-Transfer. | MOD-Transfer |  |
| 3 | then | Kiểm tra kết quả và trạng thái của MOD-Scheduler. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | handled-error |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Transfer-OK: Kiểm tra MOD-Scheduler hoạt động đúng khi phụ thuộc MOD-Transfer xử lý thành công.

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Transfer-OK |
| Mục tiêu | Kiểm tra MOD-Scheduler hoạt động đúng khi phụ thuộc MOD-Transfer xử lý thành công. |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Transfer · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Transfer-OK · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo mục STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-errorTransient-503, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Transfer |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| dependency | ok | MOD-Transfer | MOD-Scheduler gọi MOD-Transfer thành công |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Cấu hình stub trả về kết quả thành công (success). | MOD-Transfer |  |
| 2 | when | Cho MOD-Scheduler gọi MOD-Transfer. | MOD-Transfer |  |
| 3 | then | Kiểm tra kết quả và trạng thái của MOD-Scheduler. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | success |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-ERR-INT-01-ERR-INT01-4xx: INT-01: Kiểm tra lỗi HTTP 4xx (400, 401, 422) được xử lý như lỗi vĩnh viễn (permanent).

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-ERR-INT-01-ERR-INT01-4xx |
| Mục tiêu | INT-01: Kiểm tra lỗi HTTP 4xx (400, 401, 422) được xử lý như lỗi vĩnh viễn (permanent). |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01 · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01-ERR-INT01-4xx · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo mục STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-errorPermanent-200, STUB-SCHEDULED-TRANSFER-INT-01-errorPermanent-400, STUB-SCHEDULED-TRANSFER-INT-01-errorTransient, STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-INT-01-timeout, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01 |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| trigger | err-int01-4xx | permanent | ERR-INT01-4xx: HTTP 4xx (400, 401, 422) là permanent |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Cấu hình stub ở chế độ errorPermanent với mã 400. | INT-01 |  |
| 2 | when | Để MOD-Scheduler gọi INT-01 khi stub đang ở chế độ errorPermanent với mã 400. | INT-01 | HTTP 4xx (400, 401, 422) |
| 3 | then | Xác nhận lỗi được phân loại là permanent và hệ thống thực hiện: không retry; coi Fraud là không khả dụng và áp dụng chính sách FB-INT01; ghi cảnh báo vi phạm contract. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | permanent |  |
| 2 | record | equals | Không retry; coi là Fraud không khả dụng, áp chính sách FB-INT01; ghi cảnh báo vi phạm contract |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-ERR-INT-01-ERR-INT01-5xx: INT-01: Kiểm tra lỗi HTTP 5xx được xử lý như lỗi tạm thời (transient).

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-ERR-INT-01-ERR-INT01-5xx |
| Mục tiêu | INT-01: Kiểm tra lỗi HTTP 5xx được xử lý như lỗi tạm thời (transient). |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01 · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01-ERR-INT01-5xx · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo mục STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-errorPermanent-200, STUB-SCHEDULED-TRANSFER-INT-01-errorPermanent-400, STUB-SCHEDULED-TRANSFER-INT-01-errorTransient, STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-INT-01-timeout, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01 |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| trigger | err-int01-5xx | transient | ERR-INT01-5xx: HTTP 5xx là transient |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Cấu hình stub ở chế độ errorTransient. | INT-01 |  |
| 2 | when | Để MOD-Scheduler gọi INT-01 khi stub đang ở chế độ errorTransient. | INT-01 | HTTP 5xx |
| 3 | then | Xác nhận lỗi được phân loại là transient và hệ thống thực hiện: retry 1 lần; nếu vẫn lỗi thì coi là không khả dụng và áp dụng FB-INT01; đồng thời tính lỗi này vào circuit breaker. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | transient |  |
| 2 | record | equals | Retry 1 lần; vẫn lỗi thì coi là không khả dụng, áp FB-INT01; tính vào circuit breaker |  |
| 3 | event | <= | 2 | lượt gọi |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-ERR-INT-01-ERR-INT01-schema: INT-01: Kiểm tra phản hồi HTTP 200 nhưng sai schema được xử lý như lỗi vĩnh viễn (permanent).

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-ERR-INT-01-ERR-INT01-schema |
| Mục tiêu | INT-01: Kiểm tra phản hồi HTTP 200 nhưng sai schema được xử lý như lỗi vĩnh viễn (permanent). |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01 · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01-ERR-INT01-schema · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo mục STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-errorPermanent-200, STUB-SCHEDULED-TRANSFER-INT-01-errorPermanent-400, STUB-SCHEDULED-TRANSFER-INT-01-errorTransient, STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-INT-01-timeout, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01 |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| trigger | err-int01-schema | permanent | ERR-INT01-schema: HTTP 200 nhưng response sai schema là permanent |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Cấu hình stub ở chế độ errorPermanent với mã 200. | INT-01 |  |
| 2 | when | Để MOD-Scheduler gọi INT-01 khi stub đang ở chế độ errorPermanent với mã 200. | INT-01 | HTTP 200 nhưng response sai schema |
| 3 | then | Xác nhận lỗi được phân loại là permanent và hệ thống thực hiện: không retry; coi là không khả dụng; ghi cảnh báo vi phạm contract. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | permanent |  |
| 2 | record | equals | Không retry; coi là không khả dụng; ghi cảnh báo vi phạm contract |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-ERR-INT-01-ERR-INT01-timeout: INT-01: Kiểm tra timeout (connect/read) hoặc lỗi mạng được xử lý như kết quả không xác định (unknownOutcome).

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-ERR-INT-01-ERR-INT01-timeout |
| Mục tiêu | INT-01: Kiểm tra timeout (connect/read) hoặc lỗi mạng được xử lý như kết quả không xác định (unknownOutcome). |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01 · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01-ERR-INT01-timeout · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo mục STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-errorPermanent-200, STUB-SCHEDULED-TRANSFER-INT-01-errorPermanent-400, STUB-SCHEDULED-TRANSFER-INT-01-errorTransient, STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-INT-01-timeout, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01 |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| trigger | err-int01-timeout | unknownOutcome | ERR-INT01-timeout: Timeout (connect/read) hoặc lỗi mạng là unknownOutcome |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Cấu hình stub ở chế độ timeout với độ trễ 2000 ms. | INT-01 |  |
| 2 | when | Để MOD-Scheduler gọi INT-01 khi stub đang ở chế độ timeout với độ trễ 2000 ms. | INT-01 | Timeout (connect/read) hoặc lỗi mạng |
| 3 | then | Xác nhận lỗi được phân loại là unknownOutcome và hệ thống thực hiện: retry 1 lần với cùng transactionId (idempotent); nếu vẫn lỗi thì áp dụng FB-INT01. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | unknownOutcome |  |
| 2 | record | equals | Retry 1 lần với cùng transactionId (idempotent); vẫn lỗi thì áp FB-INT01 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-IDM-INT-01-REPLAY: INT-01: Kiểm tra hệ thống xử lý đúng khi cùng một yêu cầu được gửi lại (replay).

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-IDM-INT-01-REPLAY |
| Mục tiêu | INT-01: Kiểm tra hệ thống xử lý đúng khi cùng một yêu cầu được gửi lại (replay). |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-IDM-INT-01 · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-IDM-INT-01-REPLAY · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub được cấu hình theo mục STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-IDM-INT-01 |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| replay | replay | 2 | gửi lại cùng yêu cầu tới INT-01 (idempotent=true) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Có sẵn một yêu cầu đã được xử lý thành công. | INT-01 |  |
| 2 | when | Gửi lại đúng yêu cầu đó tới INT-01. | INT-01 |  |
| 3 | then | Kiểm tra kết quả trả về và số bản ghi được tạo ra. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | record | equals | 1 |  |
| 2 | state | equals | không tạo trùng |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-LIM-INT-01-maxPayloadKB-AT: INT-01: kiểm tra hệ thống xử lý đúng khi dung lượng payload đạt đúng giới hạn maxPayloadKB = 16 KB.

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-LIM-INT-01-maxPayloadKB-AT |
| Mục tiêu | INT-01: kiểm tra hệ thống xử lý đúng khi dung lượng payload đạt đúng giới hạn maxPayloadKB = 16 KB. |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-maxPayloadKB · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-maxPayloadKB-AT · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-maxPayloadKB |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| maxPayloadKB | at | 16 | đúng giới hạn maxPayloadKB = 16 KB |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Client chuẩn bị yêu cầu theo giới hạn đã cấu hình. | INT-01 |  |
| 2 | when | Gửi yêu cầu có dung lượng 16 KB tới INT-01. | INT-01 | 16 |
| 3 | then | Kiểm tra phản hồi nhận được. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-LIM-INT-01-maxPayloadKB-OVER: INT-01: kiểm tra hệ thống xử lý đúng khi dung lượng payload vượt giới hạn maxPayloadKB = 16 KB.

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-LIM-INT-01-maxPayloadKB-OVER |
| Mục tiêu | INT-01: kiểm tra hệ thống xử lý đúng khi dung lượng payload vượt giới hạn maxPayloadKB = 16 KB. |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-maxPayloadKB · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-maxPayloadKB-OVER · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-maxPayloadKB |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| maxPayloadKB | over | 17 | vượt giới hạn maxPayloadKB = 16 KB |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Client chuẩn bị yêu cầu theo giới hạn đã cấu hình. | INT-01 |  |
| 2 | when | Gửi yêu cầu có dung lượng 17 KB tới INT-01. | INT-01 | 17 |
| 3 | then | Kiểm tra phản hồi nhận được. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | in | 429\|413 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-LIM-INT-01-requestsPerSecond-AT: INT-01: kiểm tra hệ thống xử lý đúng khi tải đạt chính xác giới hạn requestsPerSecond = 50 req/s.

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-LIM-INT-01-requestsPerSecond-AT |
| Mục tiêu | INT-01: kiểm tra hệ thống xử lý đúng khi tải đạt chính xác giới hạn requestsPerSecond = 50 req/s. |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-requestsPerSecond · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-requestsPerSecond-AT · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-requestsPerSecond |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| requestsPerSecond | at | 50 | đúng giới hạn requestsPerSecond = 50 req/s |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Client chuẩn bị gửi yêu cầu theo đúng giới hạn đã cấu hình. | INT-01 |  |
| 2 | when | Gửi 50 req/s tới INT-01. | INT-01 | 50 |
| 3 | then | Kiểm tra phản hồi nhận được. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-LIM-INT-01-requestsPerSecond-OVER: INT-01: kiểm tra hệ thống xử lý khi tải vượt giới hạn requestsPerSecond = 50 req/s.

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-LIM-INT-01-requestsPerSecond-OVER |
| Mục tiêu | INT-01: kiểm tra hệ thống xử lý khi tải vượt giới hạn requestsPerSecond = 50 req/s. |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-requestsPerSecond · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-requestsPerSecond-OVER · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-requestsPerSecond |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| requestsPerSecond | over | 51 | vượt giới hạn requestsPerSecond = 50 req/s |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Client chuẩn bị gửi yêu cầu theo giới hạn đã cấu hình. | INT-01 |  |
| 2 | when | Gửi 51 req/s tới INT-01. | INT-01 | 51 |
| 3 | then | Kiểm tra phản hồi nhận được. | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | in | 429\|413 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-RTY-INT-01-EXHAUST: Kiểm tra INT-01 xử lý đúng khi đã hết lượt thử sau 2 lỗi tạm thời liên tiếp.

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-RTY-INT-01-EXHAUST |
| Mục tiêu | Kiểm tra INT-01 xử lý đúng khi đã hết lượt thử sau 2 lỗi tạm thời liên tiếp. |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-RTY-INT-01 · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-RTY-INT-01-EXHAUST · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-errorTransient-503, STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-RTY-INT-01 |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| maxAttempts | exhaust | 2 | lỗi tạm thời đủ 2 lần |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Cấu hình stub errorTransient trả mã 503: lỗi ở cả 2 lần gọi. | INT-01 |  |
| 2 | when | Cho MOD-Scheduler gọi INT-01. | INT-01 |  |
| 3 | then | Đếm số lượt gọi và kiểm tra kết quả trả về. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | event | equals | 2 | lượt gọi |
| 2 | state | equals | fallback |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-RTY-INT-01-RECOVER: Kiểm tra INT-01 có thể phục hồi và trả kết quả thành công sau khi gặp 1 lỗi tạm thời.

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-RTY-INT-01-RECOVER |
| Mục tiêu | Kiểm tra INT-01 có thể phục hồi và trả kết quả thành công sau khi gặp 1 lỗi tạm thời. |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-RTY-INT-01 · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-RTY-INT-01-RECOVER · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-errorTransient-503, STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-RTY-INT-01 |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| maxAttempts | recover | 2 | lỗi tạm thời 1 lần rồi thành công |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Cấu hình stub errorTransient trả mã 503: lỗi ở lần gọi đầu tiên, sau đó thành công. | INT-01 |  |
| 2 | when | Cho MOD-Scheduler gọi INT-01. | INT-01 |  |
| 3 | then | Đếm số lượt gọi và kiểm tra kết quả trả về. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | event | equals | 2 | lượt gọi |
| 2 | state | equals | success |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-TMO-INT-01-OVER: Kiểm tra khi INT-01 không phản hồi thì MOD-Scheduler dừng chờ sau khi hết totalMs.

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-TMO-INT-01-OVER |
| Mục tiêu | Kiểm tra khi INT-01 không phản hồi thì MOD-Scheduler dừng chờ sau khi hết totalMs. |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-TMO-INT-01 · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-TMO-INT-01-OVER · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo phần STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-slow, STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-INT-01-timeout, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-TMO-INT-01 |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| delayMs | over | 2000 | INT-01 không trả lời trước totalMs (1000 ms) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Cấu hình stub ở chế độ timeout, với độ trễ 2000 ms. | INT-01 |  |
| 2 | when | MOD-Scheduler gọi INT-01. | INT-01 |  |
| 3 | then | Xác nhận MOD-Scheduler hết hạn chờ và xử lý kết quả như unknownOutcome. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | unknownOutcome |  |
| 2 | latency | <= | 1000 | ms |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-TMO-INT-01-UNDER: Kiểm tra khi INT-01 phản hồi chậm nhưng vẫn trong thời gian cho phép thì kết quả vẫn được ghi nhận là thành công.

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-TMO-INT-01-UNDER |
| Mục tiêu | Kiểm tra khi INT-01 phản hồi chậm nhưng vẫn trong thời gian cho phép thì kết quả vẫn được ghi nhận là thành công. |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-TMO-INT-01 · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-TMO-INT-01-UNDER · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Môi trường đã sẵn sàng, dữ liệu nền đã được nạp và stub đã được cấu hình theo phần STUB của TestCase. |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-slow, STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-INT-01-timeout, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-TMO-INT-01 |
| Nguồn nội dung | llm (độ tin cậy 0.7) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| delayMs | under | 400 | INT-01 trả lời chậm nhưng trong readMs |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Cấu hình stub ở chế độ slow, với độ trễ 400 ms. | INT-01 |  |
| 2 | when | MOD-Scheduler gọi INT-01. | INT-01 |  |
| 3 | then | Xác nhận MOD-Scheduler nhận được kết quả thành công. | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | success |  |
| 2 | latency | <= | 800 | ms |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

## 4. Mục lục TC và thống kê

Tổng số Test Case: 183

| Mức | Loại | Ưu tiên | Số TC |
| --- | --- | --- | --- |
| L1 | functional | P0 | 2 |
| L2 | functional | P0 | 142 |
| L2 | functional | P1 | 6 |
| L2 | performance | P0 | 10 |
| L3 | resilience | P0 | 23 |

| Test Case | Mức | Ưu tiên | Loại thay đổi | Mục tiêu |
| --- | --- | --- | --- | --- |
| TC-L1-SCHEDULED-TRANSFER-SCN-ScheduledRun-JRN-BUDGET | L1 | P0 | new | Kiểm tra kịch bản SCN-ScheduledRun hoàn tất trong ngân sách thời gian cho phép. |
| TC-L1-SCHEDULED-TRANSFER-SCN-ScheduledRun-JRN-MAIN | L1 | P0 | new | Kiểm tra luồng chính của kịch bản SCN-ScheduledRun chạy trọn vẹn từ đầu đến cuối. |
| TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-200 | L2 | P0 | new | Kiểm tra API DELETE /v1/scheduled-transfers/{scheduleId} trả về mã 200 khi hủy lịch chuyển tiền thành công. |
| TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-401 | L2 | P0 | new | Kiểm tra API DELETE /v1/scheduled-transfers/{scheduleId} trả về mã 401 UNAUTHORIZED khi xác thực không hợp lệ. |
| TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-403 | L2 | P0 | new | Kiểm tra API DELETE /v1/scheduled-transfers/{scheduleId} trả về mã 403 FORBIDDEN khi người gọi không có quyền hủy lịch. |
| TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-404 | L2 | P0 | new | Kiểm tra API DELETE /v1/scheduled-transfers/{scheduleId} trả về mã 404 SCHEDULE_NOT_FOUND khi không tìm thấy lịch. |
| TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-409 | L2 | P0 | new | Kiểm tra API DELETE /v1/scheduled-transfers/{scheduleId} trả về mã 409 SCHEDULE_NOT_CANCELLABLE khi lịch không thể hủy. |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-amount-ABOVE-MAX | L2 | P0 | new | Kiểm tra POST /v1/scheduled-transfers khi trường amount (body) lớn hơn giá trị lớn nhất cho phép (50000001). |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-amount-BELOW-MIN | L2 | P0 | new | Kiểm tra POST /v1/scheduled-transfers khi trường amount (body) nhỏ hơn giá trị nhỏ nhất cho phép (0). |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-amount-MAX | L2 | P0 | new | Kiểm tra POST /v1/scheduled-transfers khi trường amount (body) đúng bằng giá trị lớn nhất cho phép là 50000000. |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-amount-MIN | L2 | P0 | new | Kiểm tra POST /v1/scheduled-transfers khi trường amount (body) đúng bằng giá trị nhỏ nhất cho phép là 1. |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-maxRuns-ABOVE-MAX | L2 | P0 | new | Kiểm tra POST /v1/scheduled-transfers khi trường maxRuns (body) lớn hơn giá trị lớn nhất cho phép (61). |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-maxRuns-BELOW-MIN | L2 | P0 | new | Kiểm tra POST /v1/scheduled-transfers khi trường maxRuns (body) nhỏ hơn giá trị nhỏ nhất cho phép (0). |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-maxRuns-MAX | L2 | P0 | new | Kiểm tra POST /v1/scheduled-transfers khi trường maxRuns (body) đúng bằng giá trị lớn nhất cho phép là 60. |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-maxRuns-MIN | L2 | P0 | new | Kiểm tra POST /v1/scheduled-transfers khi trường maxRuns (body) đúng bằng giá trị nhỏ nhất cho phép là 1. |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-DAILY | L2 | P0 | new | Kiểm tra POST /v1/scheduled-transfers khi trường recurrence (body) = DAILY, là một giá trị hợp lệ. |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-INVALID | L2 | P0 | new | Kiểm tra POST /v1/scheduled-transfers khi trường recurrence (body) có giá trị nằm ngoài danh sách ONCE, DAILY, WEEKLY, MONTHLY. |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-MONTHLY | L2 | P0 | new | Kiểm tra POST /v1/scheduled-transfers khi trường recurrence (body) = MONTHLY, là một giá trị hợp lệ. |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-ONCE | L2 | P0 | new | Kiểm tra POST /v1/scheduled-transfers khi trường recurrence (body) = ONCE, là một giá trị hợp lệ. |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-WEEKLY | L2 | P0 | new | Kiểm tra POST /v1/scheduled-transfers khi trường recurrence (body) = WEEKLY, là một giá trị hợp lệ. |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-200 | L2 | P0 | new | Kiểm tra POST /v1/scheduled-transfers trả về mã 200. |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-201 | L2 | P0 | new | Kiểm tra POST /v1/scheduled-transfers trả về mã 201. |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-400 | L2 | P0 | new | Kiểm tra POST /v1/scheduled-transfers trả về mã 400 INVALID_REQUEST. |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-401 | L2 | P0 | new | Kiểm tra POST /v1/scheduled-transfers trả về mã 401 UNAUTHORIZED. |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-403 | L2 | P0 | new | Kiểm tra POST /v1/scheduled-transfers trả về mã 403 FORBIDDEN. |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-404 | L2 | P0 | new | Kiểm tra POST /v1/scheduled-transfers trả về mã 404 ACCOUNT_NOT_FOUND. |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-409 | L2 | P0 | new | Kiểm tra POST /v1/scheduled-transfers trả về mã 409 IDEMPOTENCY_CONFLICT. |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-422-amount | L2 | P0 | new | Kiểm tra POST /v1/scheduled-transfers trả về mã 422 LIMIT_EXCEEDED. |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-422-limit | L2 | P0 | new | Kiểm tra POST /v1/scheduled-transfers trả về mã 422 SCHEDULE_LIMIT_REACHED. |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-422-same | L2 | P0 | new | Kiểm tra POST /v1/scheduled-transfers trả về mã 422 SAME_ACCOUNT. |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-422-time | L2 | P0 | new | Kiểm tra POST /v1/scheduled-transfers trả về mã 422 SCHEDULE_TIME_INVALID. |
| TC-L2-SCHEDULED-TRANSFER-EP-GetSchedule-API-RSP-Get-200 | L2 | P0 | new | Kiểm tra GET /v1/scheduled-transfers/{scheduleId} trả về mã 200 cùng trạng thái lịch và lần chạy kế tiếp (nextRunAt). |
| TC-L2-SCHEDULED-TRANSFER-EP-GetSchedule-API-RSP-Get-401 | L2 | P0 | new | Kiểm tra GET /v1/scheduled-transfers/{scheduleId} trả về mã 401 UNAUTHORIZED khi thiếu hoặc sai API key. |
| TC-L2-SCHEDULED-TRANSFER-EP-GetSchedule-API-RSP-Get-403 | L2 | P0 | new | Kiểm tra GET /v1/scheduled-transfers/{scheduleId} trả về mã 403 FORBIDDEN khi người gọi không phải chủ tài khoản nguồn của lịch. |
| TC-L2-SCHEDULED-TRANSFER-EP-GetSchedule-API-RSP-Get-404 | L2 | P0 | new | Kiểm tra GET /v1/scheduled-transfers/{scheduleId} trả về mã 404 SCHEDULE_NOT_FOUND khi không tồn tại lịch ứng với scheduleId. |
| TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-limit-ABOVE-MAX | L2 | P0 | new | Kiểm tra GET /v1/scheduled-transfers/{scheduleId}/runs khi tham số truy vấn limit lớn hơn giá trị lớn nhất cho phép (101). |
| TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-limit-BELOW-MIN | L2 | P0 | new | Kiểm tra GET /v1/scheduled-transfers/{scheduleId}/runs khi tham số truy vấn limit nhỏ hơn giá trị nhỏ nhất cho phép (0). |
| TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-limit-MAX | L2 | P0 | new | Kiểm tra GET /v1/scheduled-transfers/{scheduleId}/runs khi tham số truy vấn limit nhận đúng giá trị lớn nhất là 100. |
| TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-limit-MIN | L2 | P0 | new | Kiểm tra GET /v1/scheduled-transfers/{scheduleId}/runs khi tham số truy vấn limit nhận đúng giá trị nhỏ nhất là 1. |
| TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-offset-BELOW-MIN | L2 | P0 | new | Kiểm tra GET /v1/scheduled-transfers/{scheduleId}/runs khi tham số truy vấn offset nhỏ hơn giá trị nhỏ nhất cho phép (-1). |
| TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-offset-MIN | L2 | P0 | new | Kiểm tra GET /v1/scheduled-transfers/{scheduleId}/runs khi tham số truy vấn offset nhận đúng giá trị nhỏ nhất là 0. |
| TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-RSP-ListRuns-200 | L2 | P0 | new | Kiểm tra GET /v1/scheduled-transfers/{scheduleId}/runs trả về phản hồi 200. |
| TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-RSP-ListRuns-401 | L2 | P0 | new | Kiểm tra GET /v1/scheduled-transfers/{scheduleId}/runs trả về 401 UNAUTHORIZED. |
| TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-RSP-ListRuns-403 | L2 | P0 | new | Kiểm tra GET /v1/scheduled-transfers/{scheduleId}/runs trả về 403 FORBIDDEN. |
| TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-RSP-ListRuns-404 | L2 | P0 | new | Kiểm tra GET /v1/scheduled-transfers/{scheduleId}/runs trả về 404 SCHEDULE_NOT_FOUND. |
| TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-limit-ABOVE-MAX | L2 | P0 | new | Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query limit lớn hơn giá trị lớn nhất (101). |
| TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-limit-BELOW-MIN | L2 | P0 | new | Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query limit nhỏ hơn giá trị nhỏ nhất (0). |
| TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-limit-MAX | L2 | P0 | new | Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query limit bằng đúng giá trị lớn nhất là 100. |
| TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-limit-MIN | L2 | P0 | new | Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query limit bằng đúng giá trị nhỏ nhất là 1. |
| TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-offset-BELOW-MIN | L2 | P0 | new | Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query offset nhỏ hơn giá trị nhỏ nhất (-1). |
| TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-offset-MIN | L2 | P0 | new | Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query offset bằng đúng giá trị nhỏ nhất là 0. |
| TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-ACTIVE | L2 | P0 | new | Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query status = ACTIVE (giá trị hợp lệ). |
| TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-CANCELLED | L2 | P0 | new | Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query status = CANCELLED (giá trị hợp lệ). |
| TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-COMPLETED | L2 | P0 | new | Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query status = COMPLETED (giá trị hợp lệ). |
| TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-INVALID | L2 | P0 | new | Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query status có giá trị nằm ngoài danh sách ACTIVE, PAUSED, COMPLETED, CANCELLED. |
| TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-PAUSED | L2 | P0 | new | Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers khi tham số query status = PAUSED (giá trị hợp lệ). |
| TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-RSP-ListSchedules-200 | L2 | P0 | new | Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers trả về phản hồi 200. |
| TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-RSP-ListSchedules-400 | L2 | P0 | new | Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers trả về phản hồi 400 INVALID_REQUEST. |
| TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-RSP-ListSchedules-401 | L2 | P0 | new | Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers trả về phản hồi 401 UNAUTHORIZED. |
| TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-RSP-ListSchedules-403 | L2 | P0 | new | Kiểm tra GET /v1/accounts/{accountId}/scheduled-transfers trả về phản hồi 403 FORBIDDEN. |
| TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-ACTIVE | L2 | P0 | new | Kiểm tra PATCH /v1/scheduled-transfers/{scheduleId} chấp nhận trường status (body) có giá trị hợp lệ ACTIVE. |
| TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-INVALID | L2 | P0 | new | Kiểm tra PATCH /v1/scheduled-transfers/{scheduleId} từ chối trường status (body) có giá trị nằm ngoài danh sách cho phép (PAUSED, ACTIVE). |
| TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-PAUSED | L2 | P0 | new | Kiểm tra PATCH /v1/scheduled-transfers/{scheduleId} chấp nhận trường status (body) có giá trị hợp lệ PAUSED. |
| TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-200 | L2 | P0 | new | Kiểm tra PATCH /v1/scheduled-transfers/{scheduleId} trả về mã 200 khi cập nhật trạng thái thành công. |
| TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-400 | L2 | P0 | new | Kiểm tra PATCH /v1/scheduled-transfers/{scheduleId} trả về mã 400 INVALID_REQUEST khi yêu cầu không hợp lệ. |
| TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-401 | L2 | P0 | new | Kiểm tra PATCH /v1/scheduled-transfers/{scheduleId} trả về mã 401 UNAUTHORIZED khi xác thực thất bại. |
| TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-403 | L2 | P0 | new | Kiểm tra PATCH /v1/scheduled-transfers/{scheduleId} trả về mã 403 FORBIDDEN khi người gọi không có quyền. |
| TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-404 | L2 | P0 | new | Kiểm tra PATCH /v1/scheduled-transfers/{scheduleId} trả về mã 404 SCHEDULE_NOT_FOUND khi không tìm thấy lịch. |
| TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-409 | L2 | P0 | new | Kiểm tra PATCH /v1/scheduled-transfers/{scheduleId} trả về mã 409 SCHEDULE_STATE_INVALID khi trạng thái lịch không cho phép thao tác. |
| TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-1 | L2 | P0 | new | Xác nhận FR-040 được đáp ứng: gọi POST tạo lịch chuyển tiền ONCE hợp lệ thì hệ thống trả 201, kèm scheduleId và status ACTIVE. |
| TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-2 | L2 | P0 | new | Xác nhận FR-040 được đáp ứng: sau khi tạo lịch, số dư của hai tài khoản không đổi và không phát sinh bản ghi mới trong ledger_entries. |
| TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-3 | L2 | P0 | new | Xác nhận FR-040 được đáp ứng: khi firstRunAt cách thời điểm hiện tại dưới 5 phút hoặc quá 90 ngày thì hệ thống trả 422 SCHEDULE_TIME_INVALID. |
| TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-4 | L2 | P0 | new | Xác nhận FR-040 được đáp ứng: khi toAccountId không tồn tại thì hệ thống trả 404 ACCOUNT_NOT_FOUND. |
| TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-5 | L2 | P0 | new | Xác nhận FR-040 được đáp ứng: khi fromAccountId không thuộc chủ sở hữu của API key thì hệ thống trả 403 FORBIDDEN. |
| TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-6 | L2 | P0 | new | Xác nhận FR-040 được đáp ứng: khi một tài khoản tạo lịch thứ 21 ở trạng thái ACTIVE/PAUSED thì hệ thống trả 422 SCHEDULE_LIMIT_REACHED. |
| TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-REV-003 | L2 | P0 | new | Xác nhận FR-040 được đáp ứng: chỉ các lịch ACTIVE hoặc PAUSED được tính vào giới hạn 20 lịch của một tài khoản; lịch COMPLETED hoặc CANCELLED không được tính. |
| TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-FR-041-1 | L2 | P0 | new | Xác nhận FR-041 được đáp ứng: lịch DAILY, WEEKLY hoặc MONTHLY có endDate hoặc maxRuns được tạo thành công và trả về 201. |
| TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-FR-041-2 | L2 | P0 | new | Xác nhận FR-041 được đáp ứng: lịch định kỳ thiếu cả endDate lẫn maxRuns thì trả về 400 INVALID_REQUEST. |
| TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-FR-041-3 | L2 | P0 | new | Xác nhận FR-041 được đáp ứng: sau khi một lần chạy kết thúc, hệ thống sinh lần chạy PENDING kế tiếp đúng chu kỳ cho đến khi đạt maxRuns hoặc endDate, sau đó lịch chuyển sang COMPLETED. |
| TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-FR-041-4 | L2 | P0 | new | Xác nhận FR-041 được đáp ứng: lịch MONTHLY đặt vào ngày 29, 30 hoặc 31 sẽ chạy vào ngày cuối tháng ở những tháng không đủ ngày, và vào ngày 29/02 ở năm nhuận. |
| TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-REV-001 | L2 | P0 | new | Xác nhận FR-041 được đáp niệm: lịch định kỳ có endDate trước ngày của lần chạy đầu, hoặc endDate quá 366 ngày kể từ lần chạy đầu, thì trả về 422 SCHEDULE_TIME_INVALID. |
| TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-REV-002 | L2 | P0 | new | Xác nhận FR-041 được đáp ứng: lịch định kỳ có cả endDate và maxRuns sẽ kết thúc (COMPLETED) theo điều kiện đạt trước, và không sinh thêm lần chạy PENDING. |
| TC-L2-SCHEDULED-TRANSFER-FR-042-AC-AC-FR-042-1 | L2 | P0 | new | Xác nhận yêu cầu FR-042 được đáp ứng: khi gọi GET một lịch chuyển tiền, hệ thống trả về status và nextRunAt. |
| TC-L2-SCHEDULED-TRANSFER-FR-042-AC-AC-FR-042-2 | L2 | P0 | new | Xác nhận yêu cầu FR-042 được đáp ứng: GET danh sách lịch của tài khoản hỗ trợ lọc theo status và phân trang bằng limit, offset. |
| TC-L2-SCHEDULED-TRANSFER-FR-042-AC-AC-FR-042-3 | L2 | P0 | new | Xác nhận yêu cầu FR-042 được đáp ứng: GET runs trả về từng lần chạy kèm scheduledAt, status, transferId và reasonCode, sắp xếp lần mới nhất lên trước. |
| TC-L2-SCHEDULED-TRANSFER-FR-042-AC-AC-FR-042-4 | L2 | P0 | new | Xác nhận yêu cầu FR-042 được đáp ứng: người không phải chủ tài khoản nguồn nhận 403 FORBIDDEN; mã lịch không tồn tại nhận 404 SCHEDULE_NOT_FOUND. |
| TC-L2-SCHEDULED-TRANSFER-FR-043-AC-AC-FR-043-1 | L2 | P0 | new | Xác nhận FR-043 được đáp ứng: khi gọi DELETE trên lịch đang ACTIVE, lịch chuyển sang CANCELLED và các lần chạy đang PENDING cũng chuyển sang CANCELLED. |
| TC-L2-SCHEDULED-TRANSFER-FR-043-AC-AC-FR-043-2 | L2 | P0 | new | Xác nhận FR-043 được đáp ứng: khi gọi DELETE trên lịch đã CANCELLED, hệ thống trả về 200 cùng kết quả như lần trước. |
| TC-L2-SCHEDULED-TRANSFER-FR-043-AC-AC-FR-043-3 | L2 | P0 | new | Xác nhận FR-043 được đáp ứng: khi gọi DELETE mà có lần chạy đang RUNNING hoặc lịch đã COMPLETED, hệ thống trả về 409 SCHEDULE_NOT_CANCELLABLE. |
| TC-L2-SCHEDULED-TRANSFER-FR-044-AC-AC-FR-044-1 | L2 | P0 | new | Xác nhận FR-044 được đáp ứng: khi gửi PATCH với status=PAUSED lên lịch định kỳ đang ACTIVE, lịch chuyển sang PAUSED và scheduler không chạy lịch này nữa. |
| TC-L2-SCHEDULED-TRANSFER-FR-044-AC-AC-FR-044-2 | L2 | P0 | new | Xác nhận FR-044 được đáp ứng: khi gửi PATCH với status=ACTIVE lên lịch đang PAUSED, lịch chuyển sang ACTIVE; các lần đã bỏ lỡ trong thời gian tạm dừng có trạng thái SKIPPED và không bị chạy bù. |
| TC-L2-SCHEDULED-TRANSFER-FR-044-AC-AC-FR-044-3 | L2 | P0 | new | Xác nhận FR-044 được đáp ứng: khi gửi PATCH với trạng thái nguồn không hợp lệ hoặc trên lịch ONCE, hệ thống trả về 409 SCHEDULE_STATE_INVALID. |
| TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-FR-045-1 | L2 | P0 | new | Xác nhận FR-045 được đáp ứng: mỗi lần chạy đến hạn tạo đúng một giao dịch ở trạng thái COMPLETED thông qua luồng chuyển tiền hiện có, và lượt chạy (run) có trạng thái SUCCEEDED kèm transferId. |
| TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-FR-045-2 | L2 | P0 | new | Xác nhận FR-045 được đáp ứng: Idempotency-Key của lần chạy đầu tiên được tạo từ mã lịch kết hợp với thời điểm dự kiến chạy. |
| TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-FR-045-3 | L2 | P0 | new | Xác nhận FR-045 được đáp ứng: hạn mức ngày, số dư, trạng thái tài khoản và Fraud Check được kiểm tra tại thời điểm chạy lịch, không phải tại thời điểm tạo lịch. |
| TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-FR-045-4 | L2 | P0 | new | Xác nhận FR-045 được đáp ứng: giao dịch sinh ra từ lịch được tính vào hạn mức ngày và hiển thị trong lịch sử giao dịch. |
| TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-REV-004 | L2 | P0 | new | Xác nhận FR-045 được đáp ứng: giao dịch sinh từ lịch hiển thị trong lịch sử giao dịch (FR-013) và có trường scheduleId mang giá trị là mã lịch. |
| TC-L2-SCHEDULED-TRANSFER-FR-045-SCN-SCN-ScheduledRun | L2 | P0 | new | Kiểm tra toàn bộ luồng chính SCN-ScheduledRun chạy trọn vẹn từ đầu đến cuối, với kết quả đúng ở mọi Module tham gia. |
| TC-L2-SCHEDULED-TRANSFER-FR-046-AC-AC-FR-046-1 | L2 | P0 | new | Kiểm tra FR-046 được đáp ứng: khi gặp lỗi FRAUD_CHECK_UNAVAILABLE, hệ thống thử lại tối đa 3 lần, mỗi lần cách nhau 5 phút; hết số lần thử mà vẫn lỗi thì run chuyển sang FAILED kèm reasonCode. |
| TC-L2-SCHEDULED-TRANSFER-FR-046-AC-AC-FR-046-2 | L2 | P0 | new | Kiểm tra FR-046 được đáp ứng: khi gặp lỗi INSUFFICIENT_FUNDS, hệ thống thử lại tối đa 2 lần, mỗi lần cách nhau 1 giờ; hết số lần thử mà vẫn lỗi thì run chuyển sang FAILED. |
| TC-L2-SCHEDULED-TRANSFER-FR-046-AC-AC-FR-046-3 | L2 | P0 | new | Kiểm tra FR-046 được đáp ứng: khi gặp lỗi ACCOUNT_INACTIVE, FRAUD_REJECTED hoặc LIMIT_EXCEEDED, run chuyển sang FAILED ngay lập tức và không được thử lại. |
| TC-L2-SCHEDULED-TRANSFER-FR-046-AC-AC-FR-046-4 | L2 | P0 | new | Kiểm tra FR-046 được đáp ứng: sau 3 lần chạy FAILED liên tiếp, lịch chuyển tiền định kỳ chuyển sang trạng thái PAUSED. |
| TC-L2-SCHEDULED-TRANSFER-FR-046-SCN-SCN-ScheduledRun | L2 | P0 | new | Kiểm tra toàn bộ luồng chính SCN-ScheduledRun, từ lúc quét lần chạy đến hạn cho đến khi hoàn tất xử lý chuyển tiền theo lịch. |
| TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-FR-047-1 | L2 | P1 | new | Xác nhận FR-047 được đáp ứng: mỗi lần chạy thành công đều phát sự kiện ScheduledRunCompleted kèm transferId thông qua outbox. |
| TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-FR-047-2 | L2 | P1 | new | Xác nhận FR-047 được đáp ứng: mỗi lần chạy FAILED đều phát sự kiện ScheduledRunFailed kèm reasonCode thông qua outbox. |
| TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-FR-047-3 | L2 | P1 | new | Xác nhận FR-047 được đáp ứng: SMS kết quả được gửi cho người gửi, và việc gửi SMS bị lỗi không làm thay đổi trạng thái của lần chạy. |
| TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-REV-005 | L2 | P1 | new | Xác nhận FR-047 được đáp ứng: sự kiện ScheduledRunCompleted/ScheduledRunFailed có đủ các trường eventId, scheduleId, runId, fromAccountId, toAccountId, amount, correlationId, kèm transferId (khi thành công) hoặc reasonCode (khi thất bại), và được phát trên cùng topic với sự kiện giao dịch. |
| TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-REV-006 | L2 | P1 | new | Xác nhận FR-047 được đáp ứng: nội dung SMS của lần chạy thất bại nêu rõ rằng số dư chỉ được kiểm tra tại thời điểm chạy. |
| TC-L2-SCHEDULED-TRANSFER-FR-047-SCN-SCN-ScheduledRun | L2 | P1 | new | Kiểm tra luồng chính SCN-ScheduledRun chạy trọn vẹn từ đầu đến cuối, gồm quét lần chạy đến hạn, thực hiện chuyển tiền theo lịch và ghi nhận kết quả. |
| TC-L2-SCHEDULED-TRANSFER-FR-048-AC-AC-FR-048-1 | L2 | P0 | new | Xác nhận FR-048 được đáp ứng: mỗi thao tác tạo, hủy, tạm dừng và tiếp tục lịch đều được ghi audit đầy đủ các thông tin gồm correlationId, thời điểm, tác nhân (là mã tài khoản), loại sự kiện và mã lịch. |
| TC-L2-SCHEDULED-TRANSFER-FR-048-AC-AC-FR-048-2 | L2 | P0 | new | Xác nhận FR-048 được đáp ứng: mọi kết quả của lần chạy lịch đều được ghi audit với tác nhân là SCHEDULER và kèm mã lịch. |
| TC-L2-SCHEDULED-TRANSFER-FR-048-SCN-SCN-ScheduledRun | L2 | P0 | new | Kiểm tra toàn bộ luồng chính SCN-ScheduledRun, từ lúc hệ thống quét lần chạy đến hạn cho tới khi luồng hoàn tất và trạng thái cuối ở các Module tham gia đúng như mong đợi. |
| TC-L2-SCHEDULED-TRANSFER-FR-049-AC-AC-FR-049-1 | L2 | P0 | new | Xác nhận FR-049 được đáp ứng: khi lần chạy bị quá hạn không quá 24 giờ do hệ thống ngừng hoạt động, lần chạy đó được thực hiện đúng một lần. |
| TC-L2-SCHEDULED-TRANSFER-FR-049-AC-AC-FR-049-2 | L2 | P0 | new | Xác nhận FR-049 được đáp ứng: khi lần chạy quá hạn hơn 24 giờ, lần chạy được chuyển sang trạng thái MISSED, hệ thống phát sự kiện ScheduledRunFailed với reasonCode MISSED và không tạo giao dịch. |
| TC-L2-SCHEDULED-TRANSFER-FR-049-SCN-SCN-ScheduledRun | L2 | P0 | new | Kiểm tra toàn bộ luồng chính SCN-ScheduledRun, từ lúc hệ thống quét các lần chạy đến hạn cho tới khi ghi nhận audit và sự kiện. |
| TC-L2-SCHEDULED-TRANSFER-NFR-010-NFR-AT | L2 | P0 | new | Xác nhận hệ thống đạt ngưỡng NFR-010 khi giá trị đo đúng bằng ngưỡng 60 s. |
| TC-L2-SCHEDULED-TRANSFER-NFR-010-NFR-OVER | L2 | P0 | new | Xác nhận hệ thống không đạt NFR-010 khi giá trị đo vượt ngưỡng một đơn vị (60+1). |
| TC-L2-SCHEDULED-TRANSFER-NFR-011-NFR-AT | L2 | P0 | new | Xác nhận hệ thống đạt ngưỡng NFR-011: số giao dịch trùng đo được đúng bằng ngưỡng cho phép là 0. |
| TC-L2-SCHEDULED-TRANSFER-NFR-011-NFR-OVER | L2 | P0 | new | Xác nhận hệ thống vượt ngưỡng NFR-011 khi số giao dịch trùng đo được là 1, tức vượt ngưỡng 0 thêm 1 (0+1). |
| TC-L2-SCHEDULED-TRANSFER-NFR-012-NFR-AT | L2 | P0 | new | Xác nhận hệ thống đạt ngưỡng NFR-012 khi số chu kỳ quét đúng bằng ngưỡng quy định là 5 chu kỳ. |
| TC-L2-SCHEDULED-TRANSFER-NFR-012-NFR-OVER | L2 | P0 | new | Xác nhận hệ thống được đánh giá đúng khi vượt ngưỡng NFR-012, với số chu kỳ quét là 6, tức vượt ngưỡng 5 thêm 1 chu kỳ. |
| TC-L2-SCHEDULED-TRANSFER-NFR-013-NFR-AT | L2 | P0 | new | Kiểm tra hệ thống đạt ngưỡng NFR-013 khi giá trị đo đúng bằng ngưỡng 500 ms. |
| TC-L2-SCHEDULED-TRANSFER-NFR-013-NFR-OVER | L2 | P0 | new | Kiểm tra hệ thống khi giá trị đo vượt ngưỡng NFR-013 một đơn vị (500+1), tức 501 ms. |
| TC-L2-SCHEDULED-TRANSFER-NFR-014-NFR-AT | L2 | P0 | new | Kiểm tra hệ thống đạt đúng ngưỡng NFR-014 ở mức 100 %. |
| TC-L2-SCHEDULED-TRANSFER-NFR-014-NFR-OVER | L2 | P0 | new | Kiểm tra hệ thống khi vượt ngưỡng NFR-014 (100+1), tức ở mức 101 %. |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-ACTIVE-PATCH-status-ACTIVE | L2 | P0 | new | Kiểm tra hệ thống từ chối PATCH status=ACTIVE khi thực thể đang ở trạng thái ACTIVE. |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-DELETE | L2 | P0 | new | Kiểm tra hệ thống từ chối DELETE khi thực thể đang ở trạng thái CANCELLED. |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-PATCH-status-ACTIVE | L2 | P0 | new | Kiểm tra hệ thống từ chối PATCH status=ACTIVE khi thực thể đang ở trạng thái CANCELLED. |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-PATCH-status-PAUSED | L2 | P0 | new | Kiểm tra hệ thống từ chối PATCH status=PAUSED khi thực thể đang ở trạng thái CANCELLED. |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-l-n-ch-y-cu-i-k-t-th-c | L2 | P0 | new | Kiểm tra hệ thống từ chối sự kiện lần chạy cuối kết thúc khi thực thể đang ở trạng thái CANCELLED. |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-scheduler-l-n-ch-y-FAILED-th-3-li-n-ti-p | L2 | P0 | new | Kiểm tra hệ thống từ chối sự kiện scheduler ghi nhận lần chạy FAILED thứ 3 liên tiếp khi thực thể đang ở trạng thái CANCELLED. |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-COMPLETED-DELETE | L2 | P0 | new | Kiểm tra hệ thống từ chối DELETE khi thực thể đang ở trạng thái COMPLETED. |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-COMPLETED-PATCH-status-ACTIVE | L2 | P0 | new | Kiểm tra hệ thống từ chối PATCH status=ACTIVE khi thực thể đang ở trạng thái COMPLETED. |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-COMPLETED-PATCH-status-PAUSED | L2 | P0 | new | Kiểm tra hệ thống từ chối PATCH status=PAUSED khi thực thể đang ở trạng thái COMPLETED. |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-COMPLETED-l-n-ch-y-cu-i-k-t-th-c | L2 | P0 | new | Kiểm tra hệ thống từ chối sự kiện lần chạy cuối kết thúc khi thực thể đang ở trạng thái COMPLETED. |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-1 | L2 | P0 | new | Kiểm tra thực thể chuyển từ trạng thái ACTIVE sang PAUSED. |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-2 | L2 | P0 | new | Kiểm tra thực thể chuyển từ trạng thái PAUSED sang ACTIVE. |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-3 | L2 | P0 | new | Kiểm tra thực thể chuyển từ ACTIVE sang PAUSED khi scheduler ghi nhận lần chạy FAILED thứ 3 liên tiếp. |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-4 | L2 | P0 | new | Kiểm tra thực thể chuyển từ trạng thái ACTIVE sang COMPLETED khi lần chạy cuối kết thúc. |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-5 | L2 | P0 | new | Kiểm tra thực thể chuyển từ trạng thái ACTIVE sang CANCELLED. |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-6 | L2 | P0 | new | Kiểm tra thực thể chuyển từ trạng thái PAUSED sang CANCELLED. |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-7 | L2 | P0 | new | Kiểm tra thực thể chuyển từ trạng thái PAUSED sang COMPLETED. |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED--u-chu-k-qu-t | L2 | P0 | new | Kiểm tra hệ thống từ chối yêu cầu không hợp lệ khi thực thể đang ở trạng thái không hợp lệ cho yêu cầu đó. |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-DELETE-l-ch | L2 | P0 | new | Kiểm tra hệ thống từ chối yêu cầu không hợp lệ khi thực thể đang ở trạng thái không hợp lệ cho yêu cầu đó. |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-l-i-chuy-n-ti-n-tr-giao-d-ch-COMPLETED-ho-c-ph-t-l-i | L2 | P0 | new | Kiểm tra hệ thống từ chối yêu cầu không hợp lệ khi thực thể đang ở trạng thái không hợp lệ cho yêu cầu đó. |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-l-i-tr-l-i | L2 | P0 | new | Kiểm tra hệ thống từ chối yêu cầu không hợp lệ khi thực thể đang ở trạng thái không hợp lệ cho yêu cầu đó. |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-l-i-tr-l-i-t-m-th-i-ho-c-thi-u-s-d | L2 | P0 | new | Kiểm tra hệ thống từ chối yêu cầu không hợp lệ khi thực thể đang ở trạng thái không hợp lệ cho yêu cầu đó. |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-scheduler-chi-m-l-n-ch-y | L2 | P0 | new | Kiểm tra hệ thống từ chối yêu cầu không hợp lệ khi thực thể đang ở trạng thái không hợp lệ cho yêu cầu đó. |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-scheduler-ph-c-h-i-l-n-ch-y-k-t | L2 | P0 | new | Kiểm tra hệ thống từ chối yêu cầu không hợp lệ khi thực thể đang ở trạng thái không hợp lệ cho yêu cầu đó. |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-ti-p-t-c-l-ch-PAUSED | L2 | P0 | new | Kiểm tra hệ thống từ chối yêu cầu không hợp lệ khi thực thể đang ở trạng thái không hợp lệ cho yêu cầu đó. |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-FAILED-DELETE-l-ch | L2 | P0 | new | Kiểm tra hệ thống từ chối yêu cầu không hợp lệ khi thực thể đang ở trạng thái không hợp lệ cho yêu cầu đó. |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-FAILED-l-i-chuy-n-ti-n-tr-giao-d-ch-COMPLETED-ho-c-ph-t-l-i | L2 | P0 | new | Kiểm tra hệ thống từ chối yêu cầu không hợp lệ khi thực thể đang ở trạng thái không hợp lệ cho yêu cầu đó. |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-1 | L2 | P0 | new | Kiểm tra lần chạy chuyển từ trạng thái PENDING sang RUNNING. |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-2 | L2 | P0 | new | Kiểm tra lần chạy chuyển từ trạng thái RUNNING sang SUCCEEDED. |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-3 | L2 | P0 | new | Kiểm tra lần chạy chuyển từ trạng thái RUNNING về PENDING. |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-4 | L2 | P0 | new | Kiểm tra lần chạy chuyển từ trạng thái RUNNING sang FAILED. |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-5 | L2 | P0 | new | Kiểm tra lần chạy chuyển từ trạng thái PENDING sang SKIPPED. |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-6 | L2 | P0 | new | Kiểm tra lần chạy chuyển từ trạng thái PENDING sang MISSED. |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-7 | L2 | P0 | new | Kiểm tra lần chạy chuyển từ trạng thái PENDING sang CANCELLED. |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-8 | L2 | P0 | new | Kiểm tra lần chạy chuyển từ trạng thái RUNNING về PENDING khi được phục hồi. |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-BRK-INT-01-RECLOSE | L3 | P0 | new | Kiểm tra circuit breaker của INT-01 thử gọi lại sau 30 giây. |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-BRK-INT-01-TRIP | L3 | P0 | new | Kiểm tra circuit breaker của INT-01 chuyển sang trạng thái mở khi tỷ lệ lỗi đạt từ 100% trở lên sau 5 lượt gọi. |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Account-FAIL | L3 | P0 | new | Kiểm tra MOD-Scheduler xử lý đúng khi MOD-Account gặp lỗi. |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Account-OK | L3 | P0 | new | Kiểm tra MOD-Scheduler xử lý đúng khi MOD-Account hoạt động bình thường. |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Audit-FAIL | L3 | P0 | new | Kiểm tra MOD-Scheduler xử lý đúng khi module phụ thuộc MOD-Audit trả về lỗi. |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Audit-OK | L3 | P0 | new | Kiểm tra MOD-Scheduler xử lý đúng khi module phụ thuộc MOD-Audit hoạt động bình thường. |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Risk-FAIL | L3 | P0 | new | Kiểm tra MOD-Scheduler xử lý đúng khi phụ thuộc MOD-Risk gặp lỗi. |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Risk-OK | L3 | P0 | new | Kiểm tra MOD-Scheduler xử lý đúng khi phụ thuộc MOD-Risk hoạt động bình thường. |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Transfer-FAIL | L3 | P0 | new | Kiểm tra cách MOD-Scheduler xử lý khi phụ thuộc MOD-Transfer trả về lỗi. |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Transfer-OK | L3 | P0 | new | Kiểm tra MOD-Scheduler hoạt động đúng khi phụ thuộc MOD-Transfer xử lý thành công. |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-ERR-INT-01-ERR-INT01-4xx | L3 | P0 | new | INT-01: Kiểm tra lỗi HTTP 4xx (400, 401, 422) được xử lý như lỗi vĩnh viễn (permanent). |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-ERR-INT-01-ERR-INT01-5xx | L3 | P0 | new | INT-01: Kiểm tra lỗi HTTP 5xx được xử lý như lỗi tạm thời (transient). |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-ERR-INT-01-ERR-INT01-schema | L3 | P0 | new | INT-01: Kiểm tra phản hồi HTTP 200 nhưng sai schema được xử lý như lỗi vĩnh viễn (permanent). |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-ERR-INT-01-ERR-INT01-timeout | L3 | P0 | new | INT-01: Kiểm tra timeout (connect/read) hoặc lỗi mạng được xử lý như kết quả không xác định (unknownOutcome). |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-IDM-INT-01-REPLAY | L3 | P0 | new | INT-01: Kiểm tra hệ thống xử lý đúng khi cùng một yêu cầu được gửi lại (replay). |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-LIM-INT-01-maxPayloadKB-AT | L3 | P0 | new | INT-01: kiểm tra hệ thống xử lý đúng khi dung lượng payload đạt đúng giới hạn maxPayloadKB = 16 KB. |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-LIM-INT-01-maxPayloadKB-OVER | L3 | P0 | new | INT-01: kiểm tra hệ thống xử lý đúng khi dung lượng payload vượt giới hạn maxPayloadKB = 16 KB. |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-LIM-INT-01-requestsPerSecond-AT | L3 | P0 | new | INT-01: kiểm tra hệ thống xử lý đúng khi tải đạt chính xác giới hạn requestsPerSecond = 50 req/s. |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-LIM-INT-01-requestsPerSecond-OVER | L3 | P0 | new | INT-01: kiểm tra hệ thống xử lý khi tải vượt giới hạn requestsPerSecond = 50 req/s. |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-RTY-INT-01-EXHAUST | L3 | P0 | new | Kiểm tra INT-01 xử lý đúng khi đã hết lượt thử sau 2 lỗi tạm thời liên tiếp. |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-RTY-INT-01-RECOVER | L3 | P0 | new | Kiểm tra INT-01 có thể phục hồi và trả kết quả thành công sau khi gặp 1 lỗi tạm thời. |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-TMO-INT-01-OVER | L3 | P0 | new | Kiểm tra khi INT-01 không phản hồi thì MOD-Scheduler dừng chờ sau khi hết totalMs. |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-TMO-INT-01-UNDER | L3 | P0 | new | Kiểm tra khi INT-01 phản hồi chậm nhưng vẫn trong thời gian cho phép thì kết quả vẫn được ghi nhận là thành công. |

## Lịch sử phiên bản

| Phiên bản | Trạng thái | Baseline TDD | CR | Nút thêm | Nút đổi | Nút bỏ |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | Draft | b-ec21ecfae083 | CR-001 | 1555 | 0 | 0 |

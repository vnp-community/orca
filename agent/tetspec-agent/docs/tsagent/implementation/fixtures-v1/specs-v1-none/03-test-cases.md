---
spec: TSD-SCHEDULED-TRANSFER-TestCases
feature: SCHEDULED-TRANSFER
docKind: TestCases
version: 1
status: Draft
supersedes: none
baseline: b-a31cdc541d67
changes: [CR-001]
contentHash: 67d4d1cd0281f5711cd4b058d3ce220d493926f827f41bb8f36e6559feb51c22
---
# Test Cases: SCHEDULED-TRANSFER

## 0. Thông tin tài liệu

| Trường | Giá trị |
| --- | --- |
| id | TSD-SCHEDULED-TRANSFER-TestCases_v1 |
| Feature | SCHEDULED-TRANSFER |
| Phiên bản | 1 |
| Trạng thái | Draft |
| Baseline TDD | DOC-SRS-CR001 (b-a31cdc541d67) |
| CR của phiên bản này | CR-001 |

## 1. Quy ước

- id: `TC-L<mức>-<scope>-<đối tượng>-<nhóm>-<khóa>`; ưu tiên P0 đến P3; loại thay đổi new, modified, unchanged
- Bước theo Given, When, Then; kết quả mong đợi là assertion đọc máy được

## 2. Tiền điều kiện chung

- Môi trường, dữ liệu nền và stub theo Test Design (tài liệu 02)
- Bí mật chỉ tham chiếu theo tên biến: —. Không ghi giá trị bí mật vào tài liệu này

## 3. Test Cases theo mức

### 3.1 L1: Nghiệp vụ E2E

#### TC-L1-SCHEDULED-TRANSFER-SCN-ScheduledRun-JRN-BUDGET: SCN-ScheduledRun hoàn tất trong ngân sách thời gian

| Trường | Giá trị |
| --- | --- |
| id | TC-L1-SCHEDULED-TRANSFER-SCN-ScheduledRun-JRN-BUDGET |
| Mục tiêu | SCN-ScheduledRun hoàn tất trong ngân sách thời gian |
| Mức, loại, ưu tiên | L1 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-JRN · TCOV TCOV-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-JRN-BUDGET · nguồn FR-045, FR-046, FR-047, FR-048, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-JRN |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| responseBudgetMs | budget | 60000 | SCN-ScheduledRun hoàn tất trong 60000 ms |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu | MOD-API, MOD-Scheduler, MOD-Transfer |  |
| 2 | when | Chạy luồng SCN-ScheduledRun và đo tổng thời gian | — |  |
| 3 | then | Tổng thời gian không vượt ngân sách | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | latency | <= | 60000 | ms |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L1-SCHEDULED-TRANSFER-SCN-ScheduledRun-JRN-MAIN: Chạy trọn luồng chính SCN-ScheduledRun

| Trường | Giá trị |
| --- | --- |
| id | TC-L1-SCHEDULED-TRANSFER-SCN-ScheduledRun-JRN-MAIN |
| Mục tiêu | Chạy trọn luồng chính SCN-ScheduledRun |
| Mức, loại, ưu tiên | L1 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-JRN · TCOV TCOV-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-JRN-MAIN · nguồn FR-045, FR-046, FR-047, FR-048, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L1-SCN-ScheduledRun-JRN |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| SCN-ScheduledRun | main | 8 | Luồng chính SCN-ScheduledRun |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống và các Module tham gia ở trạng thái ban đầu; stub hệ thống ngoài thành công | MOD-API, MOD-Scheduler, MOD-Transfer |  |
| 2 | when | Quét mỗi 30 giây | — |  |
| 3 | when | chọn tối đa 20 lần chạy đến hạn | — |  |
| 4 | when | chiếm lần chạy (UPDATE có điều kiện) | — |  |
| 5 | when | sinh correlationId | — |  |
| 6 | when | lõi chuyển tiền (idempotency, hạn mức/số dư, Fraud Check, ghi sổ + outbox) | — |  |
| 7 | when | cập nhật lần chạy | — |  |
| 8 | when | thử lại/FAILED | — |  |
| 9 | when | audit + sự kiện | — |  |
| 10 | then | Luồng hoàn tất; kiểm trạng thái cuối ở mọi Module tham gia | MOD-API, MOD-Scheduler, MOD-Transfer |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | hoàn tất |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

### 3.2 L2: Nghiệp vụ

#### TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-200: DELETE /v1/scheduled-transfers/{scheduleId} trả 200

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-200 |
| Mục tiêu | DELETE /v1/scheduled-transfers/{scheduleId} trả 200 |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API-RSP-Cancel-200 · nguồn FR-043, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Hủy thành công (lịch CANCELLED, lần PENDING thành CANCELLED) hoặc lịch đã CANCELLED: trả kết quả cũ |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-cancel-200 | 200 | DELETE /v1/scheduled-transfers/{scheduleId} → 200: Hủy thành công (lịch CANCELLED, lần PENDING thành CANCELLED) hoặc lịch đã CANCELLED: trả kết quả cũ |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hủy thành công (lịch CANCELLED, lần PENDING thành CANCELLED) hoặc lịch đã CANCELLED: trả kết quả cũ | EP-CancelSchedule |  |
| 2 | when | Gọi DELETE /v1/scheduled-transfers/{scheduleId} trong điều kiện trên | EP-CancelSchedule |  |
| 3 | then | Phản hồi 200 | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-401: DELETE /v1/scheduled-transfers/{scheduleId} trả 401 UNAUTHORIZED

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-401 |
| Mục tiêu | DELETE /v1/scheduled-transfers/{scheduleId} trả 401 UNAUTHORIZED |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API-RSP-Cancel-401 · nguồn FR-043, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Thiếu hoặc sai API key |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-cancel-401 | 401 UNAUTHORIZED | DELETE /v1/scheduled-transfers/{scheduleId} → 401 UNAUTHORIZED: Thiếu hoặc sai API key |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thiếu hoặc sai API key | EP-CancelSchedule |  |
| 2 | when | Gọi DELETE /v1/scheduled-transfers/{scheduleId} trong điều kiện trên | EP-CancelSchedule |  |
| 3 | then | Phản hồi 401 UNAUTHORIZED | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 401 |  |
| 2 | errorCode | equals | UNAUTHORIZED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-403: DELETE /v1/scheduled-transfers/{scheduleId} trả 403 FORBIDDEN

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-403 |
| Mục tiêu | DELETE /v1/scheduled-transfers/{scheduleId} trả 403 FORBIDDEN |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API-RSP-Cancel-403 · nguồn FR-043, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Người gọi không phải chủ lịch |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-cancel-403 | 403 FORBIDDEN | DELETE /v1/scheduled-transfers/{scheduleId} → 403 FORBIDDEN: Người gọi không phải chủ lịch |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Người gọi không phải chủ lịch | EP-CancelSchedule |  |
| 2 | when | Gọi DELETE /v1/scheduled-transfers/{scheduleId} trong điều kiện trên | EP-CancelSchedule |  |
| 3 | then | Phản hồi 403 FORBIDDEN | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 403 |  |
| 2 | errorCode | equals | FORBIDDEN |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-404: DELETE /v1/scheduled-transfers/{scheduleId} trả 404 SCHEDULE_NOT_FOUND

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-404 |
| Mục tiêu | DELETE /v1/scheduled-transfers/{scheduleId} trả 404 SCHEDULE_NOT_FOUND |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API-RSP-Cancel-404 · nguồn FR-043, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Không có lịch với scheduleId |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-cancel-404 | 404 SCHEDULE_NOT_FOUND | DELETE /v1/scheduled-transfers/{scheduleId} → 404 SCHEDULE_NOT_FOUND: Không có lịch với scheduleId |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Không có lịch với scheduleId | EP-CancelSchedule |  |
| 2 | when | Gọi DELETE /v1/scheduled-transfers/{scheduleId} trong điều kiện trên | EP-CancelSchedule |  |
| 3 | then | Phản hồi 404 SCHEDULE_NOT_FOUND | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 404 |  |
| 2 | errorCode | equals | SCHEDULE_NOT_FOUND |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-409: DELETE /v1/scheduled-transfers/{scheduleId} trả 409 SCHEDULE_NOT_CANCELLABLE

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-409 |
| Mục tiêu | DELETE /v1/scheduled-transfers/{scheduleId} trả 409 SCHEDULE_NOT_CANCELLABLE |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CancelSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API-RSP-Cancel-409 · nguồn FR-043, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Lịch đã COMPLETED hoặc có lần chạy đang RUNNING |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CancelSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-cancel-409 | 409 SCHEDULE_NOT_CANCELLABLE | DELETE /v1/scheduled-transfers/{scheduleId} → 409 SCHEDULE_NOT_CANCELLABLE: Lịch đã COMPLETED hoặc có lần chạy đang RUNNING |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Lịch đã COMPLETED hoặc có lần chạy đang RUNNING | EP-CancelSchedule |  |
| 2 | when | Gọi DELETE /v1/scheduled-transfers/{scheduleId} trong điều kiện trên | EP-CancelSchedule |  |
| 3 | then | Phản hồi 409 SCHEDULE_NOT_CANCELLABLE | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 409 |  |
| 2 | errorCode | equals | SCHEDULE_NOT_CANCELLABLE |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-amount-ABOVE-MAX: POST /v1/scheduled-transfers: amount (body) lớn hơn lớn nhất (50000001)

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-amount-ABOVE-MAX |
| Mục tiêu | POST /v1/scheduled-transfers: amount (body) lớn hơn lớn nhất (50000001) |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-amount-ABOVE-MAX · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| amount (body) | fld-create-amount-above-max | 50000001 | POST /v1/scheduled-transfers: amount (body) lớn hơn lớn nhất (50000001) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho POST /v1/scheduled-transfers trừ trường amount (body); phụ thuộc ngoài trả thành công | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers với amount (body) = 50000001 | EP-CreateSchedule | amount (body)=50000001 |
| 3 | then | Yêu cầu bị từ chối và không có tác dụng phụ | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 400 |  |
| 2 | errorCode | equals | INVALID_REQUEST |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-amount-BELOW-MIN: POST /v1/scheduled-transfers: amount (body) nhỏ hơn nhỏ nhất (0)

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-amount-BELOW-MIN |
| Mục tiêu | POST /v1/scheduled-transfers: amount (body) nhỏ hơn nhỏ nhất (0) |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-amount-BELOW-MIN · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| amount (body) | fld-create-amount-below-min | 0 | POST /v1/scheduled-transfers: amount (body) nhỏ hơn nhỏ nhất (0) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho POST /v1/scheduled-transfers trừ trường amount (body); phụ thuộc ngoài trả thành công | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers với amount (body) = 0 | EP-CreateSchedule | amount (body)=0 |
| 3 | then | Yêu cầu bị từ chối và không có tác dụng phụ | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 400 |  |
| 2 | errorCode | equals | INVALID_REQUEST |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-amount-MAX: POST /v1/scheduled-transfers: amount (body) đúng giá trị lớn nhất 50000000

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-amount-MAX |
| Mục tiêu | POST /v1/scheduled-transfers: amount (body) đúng giá trị lớn nhất 50000000 |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-amount-MAX · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| amount (body) | fld-create-amount-max | 50000000 | POST /v1/scheduled-transfers: amount (body) đúng giá trị lớn nhất 50000000 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho POST /v1/scheduled-transfers trừ trường amount (body); phụ thuộc ngoài trả thành công | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers với amount (body) = 50000000 | EP-CreateSchedule | amount (body)=50000000 |
| 3 | then | Yêu cầu được chấp nhận | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-amount-MIN: POST /v1/scheduled-transfers: amount (body) đúng giá trị nhỏ nhất 1

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-amount-MIN |
| Mục tiêu | POST /v1/scheduled-transfers: amount (body) đúng giá trị nhỏ nhất 1 |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-amount-MIN · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| amount (body) | fld-create-amount-min | 1 | POST /v1/scheduled-transfers: amount (body) đúng giá trị nhỏ nhất 1 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho POST /v1/scheduled-transfers trừ trường amount (body); phụ thuộc ngoài trả thành công | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers với amount (body) = 1 | EP-CreateSchedule | amount (body)=1 |
| 3 | then | Yêu cầu được chấp nhận | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-maxRuns-ABOVE-MAX: POST /v1/scheduled-transfers: maxRuns (body) lớn hơn lớn nhất (61)

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-maxRuns-ABOVE-MAX |
| Mục tiêu | POST /v1/scheduled-transfers: maxRuns (body) lớn hơn lớn nhất (61) |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-maxRuns-ABOVE-MAX · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| maxRuns (body) | fld-create-maxruns-above-max | 61 | POST /v1/scheduled-transfers: maxRuns (body) lớn hơn lớn nhất (61) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho POST /v1/scheduled-transfers trừ trường maxRuns (body); phụ thuộc ngoài trả thành công | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers với maxRuns (body) = 61 | EP-CreateSchedule | maxRuns (body)=61 |
| 3 | then | Yêu cầu bị từ chối và không có tác dụng phụ | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 400 |  |
| 2 | errorCode | equals | INVALID_REQUEST |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-maxRuns-BELOW-MIN: POST /v1/scheduled-transfers: maxRuns (body) nhỏ hơn nhỏ nhất (0)

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-maxRuns-BELOW-MIN |
| Mục tiêu | POST /v1/scheduled-transfers: maxRuns (body) nhỏ hơn nhỏ nhất (0) |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-maxRuns-BELOW-MIN · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| maxRuns (body) | fld-create-maxruns-below-min | 0 | POST /v1/scheduled-transfers: maxRuns (body) nhỏ hơn nhỏ nhất (0) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho POST /v1/scheduled-transfers trừ trường maxRuns (body); phụ thuộc ngoài trả thành công | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers với maxRuns (body) = 0 | EP-CreateSchedule | maxRuns (body)=0 |
| 3 | then | Yêu cầu bị từ chối và không có tác dụng phụ | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 400 |  |
| 2 | errorCode | equals | INVALID_REQUEST |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-maxRuns-MAX: POST /v1/scheduled-transfers: maxRuns (body) đúng giá trị lớn nhất 60

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-maxRuns-MAX |
| Mục tiêu | POST /v1/scheduled-transfers: maxRuns (body) đúng giá trị lớn nhất 60 |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-maxRuns-MAX · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| maxRuns (body) | fld-create-maxruns-max | 60 | POST /v1/scheduled-transfers: maxRuns (body) đúng giá trị lớn nhất 60 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho POST /v1/scheduled-transfers trừ trường maxRuns (body); phụ thuộc ngoài trả thành công | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers với maxRuns (body) = 60 | EP-CreateSchedule | maxRuns (body)=60 |
| 3 | then | Yêu cầu được chấp nhận | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-maxRuns-MIN: POST /v1/scheduled-transfers: maxRuns (body) đúng giá trị nhỏ nhất 1

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-maxRuns-MIN |
| Mục tiêu | POST /v1/scheduled-transfers: maxRuns (body) đúng giá trị nhỏ nhất 1 |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-maxRuns-MIN · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| maxRuns (body) | fld-create-maxruns-min | 1 | POST /v1/scheduled-transfers: maxRuns (body) đúng giá trị nhỏ nhất 1 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho POST /v1/scheduled-transfers trừ trường maxRuns (body); phụ thuộc ngoài trả thành công | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers với maxRuns (body) = 1 | EP-CreateSchedule | maxRuns (body)=1 |
| 3 | then | Yêu cầu được chấp nhận | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-DAILY: POST /v1/scheduled-transfers: recurrence (body) = DAILY (giá trị hợp lệ)

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-DAILY |
| Mục tiêu | POST /v1/scheduled-transfers: recurrence (body) = DAILY (giá trị hợp lệ) |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-DAILY · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| recurrence (body) | fld-create-recurrence-enum-daily | DAILY | POST /v1/scheduled-transfers: recurrence (body) = DAILY (giá trị hợp lệ) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho POST /v1/scheduled-transfers trừ trường recurrence (body); phụ thuộc ngoài trả thành công | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers với recurrence (body) = DAILY | EP-CreateSchedule | recurrence (body)=DAILY |
| 3 | then | Yêu cầu được chấp nhận | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-INVALID: POST /v1/scheduled-transfers: recurrence (body) ngoài danh sách ONCE, DAILY, WEEKLY, MONTHLY

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-INVALID |
| Mục tiêu | POST /v1/scheduled-transfers: recurrence (body) ngoài danh sách ONCE, DAILY, WEEKLY, MONTHLY |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-INVALID · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| recurrence (body) | fld-create-recurrence-enum-invalid | <ngoài danh sách> | POST /v1/scheduled-transfers: recurrence (body) ngoài danh sách ONCE, DAILY, WEEKLY, MONTHLY |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho POST /v1/scheduled-transfers trừ trường recurrence (body); phụ thuộc ngoài trả thành công | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers với recurrence (body) = (giá trị ngoài danh sách) | EP-CreateSchedule | recurrence (body)=(giá trị ngoài danh sách) |
| 3 | then | Yêu cầu bị từ chối và không có tác dụng phụ | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 400 |  |
| 2 | errorCode | equals | INVALID_REQUEST |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-MONTHLY: POST /v1/scheduled-transfers: recurrence (body) = MONTHLY (giá trị hợp lệ)

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-MONTHLY |
| Mục tiêu | POST /v1/scheduled-transfers: recurrence (body) = MONTHLY (giá trị hợp lệ) |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-MONTHLY · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| recurrence (body) | fld-create-recurrence-enum-monthly | MONTHLY | POST /v1/scheduled-transfers: recurrence (body) = MONTHLY (giá trị hợp lệ) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho POST /v1/scheduled-transfers trừ trường recurrence (body); phụ thuộc ngoài trả thành công | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers với recurrence (body) = MONTHLY | EP-CreateSchedule | recurrence (body)=MONTHLY |
| 3 | then | Yêu cầu được chấp nhận | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-ONCE: POST /v1/scheduled-transfers: recurrence (body) = ONCE (giá trị hợp lệ)

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-ONCE |
| Mục tiêu | POST /v1/scheduled-transfers: recurrence (body) = ONCE (giá trị hợp lệ) |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-ONCE · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| recurrence (body) | fld-create-recurrence-enum-once | ONCE | POST /v1/scheduled-transfers: recurrence (body) = ONCE (giá trị hợp lệ) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho POST /v1/scheduled-transfers trừ trường recurrence (body); phụ thuộc ngoài trả thành công | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers với recurrence (body) = ONCE | EP-CreateSchedule | recurrence (body)=ONCE |
| 3 | then | Yêu cầu được chấp nhận | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-WEEKLY: POST /v1/scheduled-transfers: recurrence (body) = WEEKLY (giá trị hợp lệ)

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-WEEKLY |
| Mục tiêu | POST /v1/scheduled-transfers: recurrence (body) = WEEKLY (giá trị hợp lệ) |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-WEEKLY · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| recurrence (body) | fld-create-recurrence-enum-weekly | WEEKLY | POST /v1/scheduled-transfers: recurrence (body) = WEEKLY (giá trị hợp lệ) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho POST /v1/scheduled-transfers trừ trường recurrence (body); phụ thuộc ngoài trả thành công | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers với recurrence (body) = WEEKLY | EP-CreateSchedule | recurrence (body)=WEEKLY |
| 3 | then | Yêu cầu được chấp nhận | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-200: POST /v1/scheduled-transfers trả 200

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-200 |
| Mục tiêu | POST /v1/scheduled-transfers trả 200 |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-200 · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Cùng Idempotency-Key và cùng nội dung: trả lịch đã tạo |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-create-200 | 200 | POST /v1/scheduled-transfers → 200: Cùng Idempotency-Key và cùng nội dung: trả lịch đã tạo |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Cùng Idempotency-Key và cùng nội dung: trả lịch đã tạo | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers trong điều kiện trên | EP-CreateSchedule |  |
| 3 | then | Phản hồi 200 | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-201: POST /v1/scheduled-transfers trả 201

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-201 |
| Mục tiêu | POST /v1/scheduled-transfers trả 201 |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-201 · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Tạo lịch thành công, status=ACTIVE, trả mã lịch |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-create-201 | 201 | POST /v1/scheduled-transfers → 201: Tạo lịch thành công, status=ACTIVE, trả mã lịch |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Tạo lịch thành công, status=ACTIVE, trả mã lịch | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers trong điều kiện trên | EP-CreateSchedule |  |
| 3 | then | Phản hồi 201 | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 201 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-400: POST /v1/scheduled-transfers trả 400 INVALID_REQUEST

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-400 |
| Mục tiêu | POST /v1/scheduled-transfers trả 400 INVALID_REQUEST |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-400 · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Thiếu Idempotency-Key hoặc trường bắt buộc, amount không nguyên dương, recurrence sai, định kỳ thiếu cả endDate và maxRuns, maxRuns ngoài 1..60, firstRunAt sai định dạng |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-create-400 | 400 INVALID_REQUEST | POST /v1/scheduled-transfers → 400 INVALID_REQUEST: Thiếu Idempotency-Key hoặc trường bắt buộc, amount không nguyên dương, recurrence sai, định kỳ thiếu cả endDate và maxRuns, maxRuns ngoài 1..60, firstRunAt sai định dạng |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thiếu Idempotency-Key hoặc trường bắt buộc, amount không nguyên dương, recurrence sai, định kỳ thiếu cả endDate và maxRuns, maxRuns ngoài 1..60, firstRunAt sai định dạng | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers trong điều kiện trên | EP-CreateSchedule |  |
| 3 | then | Phản hồi 400 INVALID_REQUEST | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 400 |  |
| 2 | errorCode | equals | INVALID_REQUEST |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-401: POST /v1/scheduled-transfers trả 401 UNAUTHORIZED

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-401 |
| Mục tiêu | POST /v1/scheduled-transfers trả 401 UNAUTHORIZED |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-401 · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Thiếu hoặc sai API key |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-create-401 | 401 UNAUTHORIZED | POST /v1/scheduled-transfers → 401 UNAUTHORIZED: Thiếu hoặc sai API key |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thiếu hoặc sai API key | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers trong điều kiện trên | EP-CreateSchedule |  |
| 3 | then | Phản hồi 401 UNAUTHORIZED | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 401 |  |
| 2 | errorCode | equals | UNAUTHORIZED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-403: POST /v1/scheduled-transfers trả 403 FORBIDDEN

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-403 |
| Mục tiêu | POST /v1/scheduled-transfers trả 403 FORBIDDEN |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-403 · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | fromAccountId khác tài khoản của người gọi |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-create-403 | 403 FORBIDDEN | POST /v1/scheduled-transfers → 403 FORBIDDEN: fromAccountId khác tài khoản của người gọi |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | fromAccountId khác tài khoản của người gọi | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers trong điều kiện trên | EP-CreateSchedule |  |
| 3 | then | Phản hồi 403 FORBIDDEN | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 403 |  |
| 2 | errorCode | equals | FORBIDDEN |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-404: POST /v1/scheduled-transfers trả 404 ACCOUNT_NOT_FOUND

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-404 |
| Mục tiêu | POST /v1/scheduled-transfers trả 404 ACCOUNT_NOT_FOUND |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-404 · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Tài khoản đích không tồn tại |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-create-404 | 404 ACCOUNT_NOT_FOUND | POST /v1/scheduled-transfers → 404 ACCOUNT_NOT_FOUND: Tài khoản đích không tồn tại |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Tài khoản đích không tồn tại | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers trong điều kiện trên | EP-CreateSchedule |  |
| 3 | then | Phản hồi 404 ACCOUNT_NOT_FOUND | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 404 |  |
| 2 | errorCode | equals | ACCOUNT_NOT_FOUND |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-409: POST /v1/scheduled-transfers trả 409 IDEMPOTENCY_CONFLICT

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-409 |
| Mục tiêu | POST /v1/scheduled-transfers trả 409 IDEMPOTENCY_CONFLICT |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-409 · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Cùng Idempotency-Key nhưng nội dung khác |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-create-409 | 409 IDEMPOTENCY_CONFLICT | POST /v1/scheduled-transfers → 409 IDEMPOTENCY_CONFLICT: Cùng Idempotency-Key nhưng nội dung khác |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Cùng Idempotency-Key nhưng nội dung khác | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers trong điều kiện trên | EP-CreateSchedule |  |
| 3 | then | Phản hồi 409 IDEMPOTENCY_CONFLICT | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 409 |  |
| 2 | errorCode | equals | IDEMPOTENCY_CONFLICT |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-422-amount: POST /v1/scheduled-transfers trả 422 LIMIT_EXCEEDED

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-422-amount |
| Mục tiêu | POST /v1/scheduled-transfers trả 422 LIMIT_EXCEEDED |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-422-amount · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | amount > 50.000.000 VND |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-create-422-amount | 422 LIMIT_EXCEEDED | POST /v1/scheduled-transfers → 422 LIMIT_EXCEEDED: amount > 50.000.000 VND |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | amount > 50.000.000 VND | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers trong điều kiện trên | EP-CreateSchedule |  |
| 3 | then | Phản hồi 422 LIMIT_EXCEEDED | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 422 |  |
| 2 | errorCode | equals | LIMIT_EXCEEDED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-422-limit: POST /v1/scheduled-transfers trả 422 SCHEDULE_LIMIT_REACHED

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-422-limit |
| Mục tiêu | POST /v1/scheduled-transfers trả 422 SCHEDULE_LIMIT_REACHED |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-422-limit · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Tài khoản đã có 20 lịch ACTIVE hoặc PAUSED |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-create-422-limit | 422 SCHEDULE_LIMIT_REACHED | POST /v1/scheduled-transfers → 422 SCHEDULE_LIMIT_REACHED: Tài khoản đã có 20 lịch ACTIVE hoặc PAUSED |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Tài khoản đã có 20 lịch ACTIVE hoặc PAUSED | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers trong điều kiện trên | EP-CreateSchedule |  |
| 3 | then | Phản hồi 422 SCHEDULE_LIMIT_REACHED | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 422 |  |
| 2 | errorCode | equals | SCHEDULE_LIMIT_REACHED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-422-same: POST /v1/scheduled-transfers trả 422 SAME_ACCOUNT

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-422-same |
| Mục tiêu | POST /v1/scheduled-transfers trả 422 SAME_ACCOUNT |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-422-same · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | fromAccountId bằng toAccountId |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-create-422-same | 422 SAME_ACCOUNT | POST /v1/scheduled-transfers → 422 SAME_ACCOUNT: fromAccountId bằng toAccountId |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | fromAccountId bằng toAccountId | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers trong điều kiện trên | EP-CreateSchedule |  |
| 3 | then | Phản hồi 422 SAME_ACCOUNT | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 422 |  |
| 2 | errorCode | equals | SAME_ACCOUNT |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-422-time: POST /v1/scheduled-transfers trả 422 SCHEDULE_TIME_INVALID

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-422-time |
| Mục tiêu | POST /v1/scheduled-transfers trả 422 SCHEDULE_TIME_INVALID |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-CreateSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API-RSP-Create-422-time · nguồn FR-040, FR-041, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | firstRunAt cách hiện tại < 5 phút hoặc > 90 ngày; endDate trước lần chạy đầu hoặc quá 366 ngày |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-CreateSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-create-422-time | 422 SCHEDULE_TIME_INVALID | POST /v1/scheduled-transfers → 422 SCHEDULE_TIME_INVALID: firstRunAt cách hiện tại < 5 phút hoặc > 90 ngày; endDate trước lần chạy đầu hoặc quá 366 ngày |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | firstRunAt cách hiện tại < 5 phút hoặc > 90 ngày; endDate trước lần chạy đầu hoặc quá 366 ngày | EP-CreateSchedule |  |
| 2 | when | Gọi POST /v1/scheduled-transfers trong điều kiện trên | EP-CreateSchedule |  |
| 3 | then | Phản hồi 422 SCHEDULE_TIME_INVALID | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 422 |  |
| 2 | errorCode | equals | SCHEDULE_TIME_INVALID |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-GetSchedule-API-RSP-Get-200: GET /v1/scheduled-transfers/{scheduleId} trả 200

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-GetSchedule-API-RSP-Get-200 |
| Mục tiêu | GET /v1/scheduled-transfers/{scheduleId} trả 200 |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-GetSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API-RSP-Get-200 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Trả trạng thái lịch và lần chạy kế tiếp (nextRunAt) |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-get-200 | 200 | GET /v1/scheduled-transfers/{scheduleId} → 200: Trả trạng thái lịch và lần chạy kế tiếp (nextRunAt) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Trả trạng thái lịch và lần chạy kế tiếp (nextRunAt) | EP-GetSchedule |  |
| 2 | when | Gọi GET /v1/scheduled-transfers/{scheduleId} trong điều kiện trên | EP-GetSchedule |  |
| 3 | then | Phản hồi 200 | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-GetSchedule-API-RSP-Get-401: GET /v1/scheduled-transfers/{scheduleId} trả 401 UNAUTHORIZED

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-GetSchedule-API-RSP-Get-401 |
| Mục tiêu | GET /v1/scheduled-transfers/{scheduleId} trả 401 UNAUTHORIZED |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-GetSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API-RSP-Get-401 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Thiếu hoặc sai API key |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-get-401 | 401 UNAUTHORIZED | GET /v1/scheduled-transfers/{scheduleId} → 401 UNAUTHORIZED: Thiếu hoặc sai API key |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thiếu hoặc sai API key | EP-GetSchedule |  |
| 2 | when | Gọi GET /v1/scheduled-transfers/{scheduleId} trong điều kiện trên | EP-GetSchedule |  |
| 3 | then | Phản hồi 401 UNAUTHORIZED | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 401 |  |
| 2 | errorCode | equals | UNAUTHORIZED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-GetSchedule-API-RSP-Get-403: GET /v1/scheduled-transfers/{scheduleId} trả 403 FORBIDDEN

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-GetSchedule-API-RSP-Get-403 |
| Mục tiêu | GET /v1/scheduled-transfers/{scheduleId} trả 403 FORBIDDEN |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-GetSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API-RSP-Get-403 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Người gọi không phải chủ tài khoản nguồn của lịch |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-get-403 | 403 FORBIDDEN | GET /v1/scheduled-transfers/{scheduleId} → 403 FORBIDDEN: Người gọi không phải chủ tài khoản nguồn của lịch |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Người gọi không phải chủ tài khoản nguồn của lịch | EP-GetSchedule |  |
| 2 | when | Gọi GET /v1/scheduled-transfers/{scheduleId} trong điều kiện trên | EP-GetSchedule |  |
| 3 | then | Phản hồi 403 FORBIDDEN | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 403 |  |
| 2 | errorCode | equals | FORBIDDEN |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-GetSchedule-API-RSP-Get-404: GET /v1/scheduled-transfers/{scheduleId} trả 404 SCHEDULE_NOT_FOUND

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-GetSchedule-API-RSP-Get-404 |
| Mục tiêu | GET /v1/scheduled-transfers/{scheduleId} trả 404 SCHEDULE_NOT_FOUND |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-GetSchedule · TD TD-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API-RSP-Get-404 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Không có lịch với scheduleId |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-GetSchedule-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-get-404 | 404 SCHEDULE_NOT_FOUND | GET /v1/scheduled-transfers/{scheduleId} → 404 SCHEDULE_NOT_FOUND: Không có lịch với scheduleId |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Không có lịch với scheduleId | EP-GetSchedule |  |
| 2 | when | Gọi GET /v1/scheduled-transfers/{scheduleId} trong điều kiện trên | EP-GetSchedule |  |
| 3 | then | Phản hồi 404 SCHEDULE_NOT_FOUND | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 404 |  |
| 2 | errorCode | equals | SCHEDULE_NOT_FOUND |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-limit-ABOVE-MAX: GET /v1/scheduled-transfers/{scheduleId}/runs: limit (query) lớn hơn lớn nhất (101)

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-limit-ABOVE-MAX |
| Mục tiêu | GET /v1/scheduled-transfers/{scheduleId}/runs: limit (query) lớn hơn lớn nhất (101) |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListRuns · TD TD-SCHEDULED-TRANSFER-L2-EP-ListRuns-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-FLD-ListRuns-limit-ABOVE-MAX · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListRuns-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| limit (query) | fld-listruns-limit-above-max | 101 | GET /v1/scheduled-transfers/{scheduleId}/runs: limit (query) lớn hơn lớn nhất (101) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho GET /v1/scheduled-transfers/{scheduleId}/runs trừ trường limit (query); phụ thuộc ngoài trả thành công | EP-ListRuns |  |
| 2 | when | Gọi GET /v1/scheduled-transfers/{scheduleId}/runs với limit (query) = 101 | EP-ListRuns | limit (query)=101 |
| 3 | then | Yêu cầu bị từ chối và không có tác dụng phụ | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 4xx |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-limit-BELOW-MIN: GET /v1/scheduled-transfers/{scheduleId}/runs: limit (query) nhỏ hơn nhỏ nhất (0)

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-limit-BELOW-MIN |
| Mục tiêu | GET /v1/scheduled-transfers/{scheduleId}/runs: limit (query) nhỏ hơn nhỏ nhất (0) |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListRuns · TD TD-SCHEDULED-TRANSFER-L2-EP-ListRuns-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-FLD-ListRuns-limit-BELOW-MIN · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListRuns-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| limit (query) | fld-listruns-limit-below-min | 0 | GET /v1/scheduled-transfers/{scheduleId}/runs: limit (query) nhỏ hơn nhỏ nhất (0) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho GET /v1/scheduled-transfers/{scheduleId}/runs trừ trường limit (query); phụ thuộc ngoài trả thành công | EP-ListRuns |  |
| 2 | when | Gọi GET /v1/scheduled-transfers/{scheduleId}/runs với limit (query) = 0 | EP-ListRuns | limit (query)=0 |
| 3 | then | Yêu cầu bị từ chối và không có tác dụng phụ | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 4xx |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-limit-MAX: GET /v1/scheduled-transfers/{scheduleId}/runs: limit (query) đúng giá trị lớn nhất 100

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-limit-MAX |
| Mục tiêu | GET /v1/scheduled-transfers/{scheduleId}/runs: limit (query) đúng giá trị lớn nhất 100 |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListRuns · TD TD-SCHEDULED-TRANSFER-L2-EP-ListRuns-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-FLD-ListRuns-limit-MAX · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListRuns-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| limit (query) | fld-listruns-limit-max | 100 | GET /v1/scheduled-transfers/{scheduleId}/runs: limit (query) đúng giá trị lớn nhất 100 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho GET /v1/scheduled-transfers/{scheduleId}/runs trừ trường limit (query); phụ thuộc ngoài trả thành công | EP-ListRuns |  |
| 2 | when | Gọi GET /v1/scheduled-transfers/{scheduleId}/runs với limit (query) = 100 | EP-ListRuns | limit (query)=100 |
| 3 | then | Yêu cầu được chấp nhận | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-limit-MIN: GET /v1/scheduled-transfers/{scheduleId}/runs: limit (query) đúng giá trị nhỏ nhất 1

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-limit-MIN |
| Mục tiêu | GET /v1/scheduled-transfers/{scheduleId}/runs: limit (query) đúng giá trị nhỏ nhất 1 |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListRuns · TD TD-SCHEDULED-TRANSFER-L2-EP-ListRuns-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-FLD-ListRuns-limit-MIN · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListRuns-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| limit (query) | fld-listruns-limit-min | 1 | GET /v1/scheduled-transfers/{scheduleId}/runs: limit (query) đúng giá trị nhỏ nhất 1 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho GET /v1/scheduled-transfers/{scheduleId}/runs trừ trường limit (query); phụ thuộc ngoài trả thành công | EP-ListRuns |  |
| 2 | when | Gọi GET /v1/scheduled-transfers/{scheduleId}/runs với limit (query) = 1 | EP-ListRuns | limit (query)=1 |
| 3 | then | Yêu cầu được chấp nhận | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-offset-BELOW-MIN: GET /v1/scheduled-transfers/{scheduleId}/runs: offset (query) nhỏ hơn nhỏ nhất (-1)

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-offset-BELOW-MIN |
| Mục tiêu | GET /v1/scheduled-transfers/{scheduleId}/runs: offset (query) nhỏ hơn nhỏ nhất (-1) |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListRuns · TD TD-SCHEDULED-TRANSFER-L2-EP-ListRuns-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-FLD-ListRuns-offset-BELOW-MIN · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListRuns-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| offset (query) | fld-listruns-offset-below-min | -1 | GET /v1/scheduled-transfers/{scheduleId}/runs: offset (query) nhỏ hơn nhỏ nhất (-1) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho GET /v1/scheduled-transfers/{scheduleId}/runs trừ trường offset (query); phụ thuộc ngoài trả thành công | EP-ListRuns |  |
| 2 | when | Gọi GET /v1/scheduled-transfers/{scheduleId}/runs với offset (query) = -1 | EP-ListRuns | offset (query)=-1 |
| 3 | then | Yêu cầu bị từ chối và không có tác dụng phụ | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 4xx |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-offset-MIN: GET /v1/scheduled-transfers/{scheduleId}/runs: offset (query) đúng giá trị nhỏ nhất 0

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-offset-MIN |
| Mục tiêu | GET /v1/scheduled-transfers/{scheduleId}/runs: offset (query) đúng giá trị nhỏ nhất 0 |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListRuns · TD TD-SCHEDULED-TRANSFER-L2-EP-ListRuns-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-FLD-ListRuns-offset-MIN · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListRuns-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| offset (query) | fld-listruns-offset-min | 0 | GET /v1/scheduled-transfers/{scheduleId}/runs: offset (query) đúng giá trị nhỏ nhất 0 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho GET /v1/scheduled-transfers/{scheduleId}/runs trừ trường offset (query); phụ thuộc ngoài trả thành công | EP-ListRuns |  |
| 2 | when | Gọi GET /v1/scheduled-transfers/{scheduleId}/runs với offset (query) = 0 | EP-ListRuns | offset (query)=0 |
| 3 | then | Yêu cầu được chấp nhận | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-RSP-ListRuns-200: GET /v1/scheduled-transfers/{scheduleId}/runs trả 200

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-RSP-ListRuns-200 |
| Mục tiêu | GET /v1/scheduled-transfers/{scheduleId}/runs trả 200 |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListRuns · TD TD-SCHEDULED-TRANSFER-L2-EP-ListRuns-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-RSP-ListRuns-200 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Trả các lần chạy, mới nhất trước, mỗi lần có thời điểm dự kiến, trạng thái, transferId, reasonCode |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListRuns-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-listruns-200 | 200 | GET /v1/scheduled-transfers/{scheduleId}/runs → 200: Trả các lần chạy, mới nhất trước, mỗi lần có thời điểm dự kiến, trạng thái, transferId, reasonCode |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Trả các lần chạy, mới nhất trước, mỗi lần có thời điểm dự kiến, trạng thái, transferId, reasonCode | EP-ListRuns |  |
| 2 | when | Gọi GET /v1/scheduled-transfers/{scheduleId}/runs trong điều kiện trên | EP-ListRuns |  |
| 3 | then | Phản hồi 200 | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-RSP-ListRuns-401: GET /v1/scheduled-transfers/{scheduleId}/runs trả 401 UNAUTHORIZED

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-RSP-ListRuns-401 |
| Mục tiêu | GET /v1/scheduled-transfers/{scheduleId}/runs trả 401 UNAUTHORIZED |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListRuns · TD TD-SCHEDULED-TRANSFER-L2-EP-ListRuns-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-RSP-ListRuns-401 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Thiếu hoặc sai API key |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListRuns-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-listruns-401 | 401 UNAUTHORIZED | GET /v1/scheduled-transfers/{scheduleId}/runs → 401 UNAUTHORIZED: Thiếu hoặc sai API key |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thiếu hoặc sai API key | EP-ListRuns |  |
| 2 | when | Gọi GET /v1/scheduled-transfers/{scheduleId}/runs trong điều kiện trên | EP-ListRuns |  |
| 3 | then | Phản hồi 401 UNAUTHORIZED | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 401 |  |
| 2 | errorCode | equals | UNAUTHORIZED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-RSP-ListRuns-403: GET /v1/scheduled-transfers/{scheduleId}/runs trả 403 FORBIDDEN

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-RSP-ListRuns-403 |
| Mục tiêu | GET /v1/scheduled-transfers/{scheduleId}/runs trả 403 FORBIDDEN |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListRuns · TD TD-SCHEDULED-TRANSFER-L2-EP-ListRuns-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-RSP-ListRuns-403 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Người gọi không phải chủ lịch |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListRuns-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-listruns-403 | 403 FORBIDDEN | GET /v1/scheduled-transfers/{scheduleId}/runs → 403 FORBIDDEN: Người gọi không phải chủ lịch |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Người gọi không phải chủ lịch | EP-ListRuns |  |
| 2 | when | Gọi GET /v1/scheduled-transfers/{scheduleId}/runs trong điều kiện trên | EP-ListRuns |  |
| 3 | then | Phản hồi 403 FORBIDDEN | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 403 |  |
| 2 | errorCode | equals | FORBIDDEN |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-RSP-ListRuns-404: GET /v1/scheduled-transfers/{scheduleId}/runs trả 404 SCHEDULE_NOT_FOUND

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-RSP-ListRuns-404 |
| Mục tiêu | GET /v1/scheduled-transfers/{scheduleId}/runs trả 404 SCHEDULE_NOT_FOUND |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListRuns · TD TD-SCHEDULED-TRANSFER-L2-EP-ListRuns-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListRuns-API-RSP-ListRuns-404 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Không có lịch với scheduleId |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListRuns-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-listruns-404 | 404 SCHEDULE_NOT_FOUND | GET /v1/scheduled-transfers/{scheduleId}/runs → 404 SCHEDULE_NOT_FOUND: Không có lịch với scheduleId |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Không có lịch với scheduleId | EP-ListRuns |  |
| 2 | when | Gọi GET /v1/scheduled-transfers/{scheduleId}/runs trong điều kiện trên | EP-ListRuns |  |
| 3 | then | Phản hồi 404 SCHEDULE_NOT_FOUND | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 404 |  |
| 2 | errorCode | equals | SCHEDULE_NOT_FOUND |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-limit-ABOVE-MAX: GET /v1/accounts/{accountId}/scheduled-transfers: limit (query) lớn hơn lớn nhất (101)

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-limit-ABOVE-MAX |
| Mục tiêu | GET /v1/accounts/{accountId}/scheduled-transfers: limit (query) lớn hơn lớn nhất (101) |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules · TD TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-limit-ABOVE-MAX · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| limit (query) | fld-listschedules-limit-above-max | 101 | GET /v1/accounts/{accountId}/scheduled-transfers: limit (query) lớn hơn lớn nhất (101) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho GET /v1/accounts/{accountId}/scheduled-transfers trừ trường limit (query); phụ thuộc ngoài trả thành công | EP-ListSchedules |  |
| 2 | when | Gọi GET /v1/accounts/{accountId}/scheduled-transfers với limit (query) = 101 | EP-ListSchedules | limit (query)=101 |
| 3 | then | Yêu cầu bị từ chối và không có tác dụng phụ | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 400 |  |
| 2 | errorCode | equals | INVALID_REQUEST |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-limit-BELOW-MIN: GET /v1/accounts/{accountId}/scheduled-transfers: limit (query) nhỏ hơn nhỏ nhất (0)

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-limit-BELOW-MIN |
| Mục tiêu | GET /v1/accounts/{accountId}/scheduled-transfers: limit (query) nhỏ hơn nhỏ nhất (0) |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules · TD TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-limit-BELOW-MIN · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| limit (query) | fld-listschedules-limit-below-min | 0 | GET /v1/accounts/{accountId}/scheduled-transfers: limit (query) nhỏ hơn nhỏ nhất (0) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho GET /v1/accounts/{accountId}/scheduled-transfers trừ trường limit (query); phụ thuộc ngoài trả thành công | EP-ListSchedules |  |
| 2 | when | Gọi GET /v1/accounts/{accountId}/scheduled-transfers với limit (query) = 0 | EP-ListSchedules | limit (query)=0 |
| 3 | then | Yêu cầu bị từ chối và không có tác dụng phụ | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 400 |  |
| 2 | errorCode | equals | INVALID_REQUEST |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-limit-MAX: GET /v1/accounts/{accountId}/scheduled-transfers: limit (query) đúng giá trị lớn nhất 100

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-limit-MAX |
| Mục tiêu | GET /v1/accounts/{accountId}/scheduled-transfers: limit (query) đúng giá trị lớn nhất 100 |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules · TD TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-limit-MAX · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| limit (query) | fld-listschedules-limit-max | 100 | GET /v1/accounts/{accountId}/scheduled-transfers: limit (query) đúng giá trị lớn nhất 100 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho GET /v1/accounts/{accountId}/scheduled-transfers trừ trường limit (query); phụ thuộc ngoài trả thành công | EP-ListSchedules |  |
| 2 | when | Gọi GET /v1/accounts/{accountId}/scheduled-transfers với limit (query) = 100 | EP-ListSchedules | limit (query)=100 |
| 3 | then | Yêu cầu được chấp nhận | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-limit-MIN: GET /v1/accounts/{accountId}/scheduled-transfers: limit (query) đúng giá trị nhỏ nhất 1

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-limit-MIN |
| Mục tiêu | GET /v1/accounts/{accountId}/scheduled-transfers: limit (query) đúng giá trị nhỏ nhất 1 |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules · TD TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-limit-MIN · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| limit (query) | fld-listschedules-limit-min | 1 | GET /v1/accounts/{accountId}/scheduled-transfers: limit (query) đúng giá trị nhỏ nhất 1 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho GET /v1/accounts/{accountId}/scheduled-transfers trừ trường limit (query); phụ thuộc ngoài trả thành công | EP-ListSchedules |  |
| 2 | when | Gọi GET /v1/accounts/{accountId}/scheduled-transfers với limit (query) = 1 | EP-ListSchedules | limit (query)=1 |
| 3 | then | Yêu cầu được chấp nhận | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-offset-BELOW-MIN: GET /v1/accounts/{accountId}/scheduled-transfers: offset (query) nhỏ hơn nhỏ nhất (-1)

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-offset-BELOW-MIN |
| Mục tiêu | GET /v1/accounts/{accountId}/scheduled-transfers: offset (query) nhỏ hơn nhỏ nhất (-1) |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules · TD TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-offset-BELOW-MIN · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| offset (query) | fld-listschedules-offset-below-min | -1 | GET /v1/accounts/{accountId}/scheduled-transfers: offset (query) nhỏ hơn nhỏ nhất (-1) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho GET /v1/accounts/{accountId}/scheduled-transfers trừ trường offset (query); phụ thuộc ngoài trả thành công | EP-ListSchedules |  |
| 2 | when | Gọi GET /v1/accounts/{accountId}/scheduled-transfers với offset (query) = -1 | EP-ListSchedules | offset (query)=-1 |
| 3 | then | Yêu cầu bị từ chối và không có tác dụng phụ | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 400 |  |
| 2 | errorCode | equals | INVALID_REQUEST |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-offset-MIN: GET /v1/accounts/{accountId}/scheduled-transfers: offset (query) đúng giá trị nhỏ nhất 0

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-offset-MIN |
| Mục tiêu | GET /v1/accounts/{accountId}/scheduled-transfers: offset (query) đúng giá trị nhỏ nhất 0 |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules · TD TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-offset-MIN · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| offset (query) | fld-listschedules-offset-min | 0 | GET /v1/accounts/{accountId}/scheduled-transfers: offset (query) đúng giá trị nhỏ nhất 0 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho GET /v1/accounts/{accountId}/scheduled-transfers trừ trường offset (query); phụ thuộc ngoài trả thành công | EP-ListSchedules |  |
| 2 | when | Gọi GET /v1/accounts/{accountId}/scheduled-transfers với offset (query) = 0 | EP-ListSchedules | offset (query)=0 |
| 3 | then | Yêu cầu được chấp nhận | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-ACTIVE: GET /v1/accounts/{accountId}/scheduled-transfers: status (query) = ACTIVE (giá trị hợp lệ)

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-ACTIVE |
| Mục tiêu | GET /v1/accounts/{accountId}/scheduled-transfers: status (query) = ACTIVE (giá trị hợp lệ) |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules · TD TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-ACTIVE · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| status (query) | fld-listschedules-status-enum-active | ACTIVE | GET /v1/accounts/{accountId}/scheduled-transfers: status (query) = ACTIVE (giá trị hợp lệ) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho GET /v1/accounts/{accountId}/scheduled-transfers trừ trường status (query); phụ thuộc ngoài trả thành công | EP-ListSchedules |  |
| 2 | when | Gọi GET /v1/accounts/{accountId}/scheduled-transfers với status (query) = ACTIVE | EP-ListSchedules | status (query)=ACTIVE |
| 3 | then | Yêu cầu được chấp nhận | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-CANCELLED: GET /v1/accounts/{accountId}/scheduled-transfers: status (query) = CANCELLED (giá trị hợp lệ)

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-CANCELLED |
| Mục tiêu | GET /v1/accounts/{accountId}/scheduled-transfers: status (query) = CANCELLED (giá trị hợp lệ) |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules · TD TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-CANCELLED · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| status (query) | fld-listschedules-status-enum-cancelled | CANCELLED | GET /v1/accounts/{accountId}/scheduled-transfers: status (query) = CANCELLED (giá trị hợp lệ) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho GET /v1/accounts/{accountId}/scheduled-transfers trừ trường status (query); phụ thuộc ngoài trả thành công | EP-ListSchedules |  |
| 2 | when | Gọi GET /v1/accounts/{accountId}/scheduled-transfers với status (query) = CANCELLED | EP-ListSchedules | status (query)=CANCELLED |
| 3 | then | Yêu cầu được chấp nhận | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-COMPLETED: GET /v1/accounts/{accountId}/scheduled-transfers: status (query) = COMPLETED (giá trị hợp lệ)

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-COMPLETED |
| Mục tiêu | GET /v1/accounts/{accountId}/scheduled-transfers: status (query) = COMPLETED (giá trị hợp lệ) |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules · TD TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-COMPLETED · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| status (query) | fld-listschedules-status-enum-completed | COMPLETED | GET /v1/accounts/{accountId}/scheduled-transfers: status (query) = COMPLETED (giá trị hợp lệ) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho GET /v1/accounts/{accountId}/scheduled-transfers trừ trường status (query); phụ thuộc ngoài trả thành công | EP-ListSchedules |  |
| 2 | when | Gọi GET /v1/accounts/{accountId}/scheduled-transfers với status (query) = COMPLETED | EP-ListSchedules | status (query)=COMPLETED |
| 3 | then | Yêu cầu được chấp nhận | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-INVALID: GET /v1/accounts/{accountId}/scheduled-transfers: status (query) ngoài danh sách ACTIVE, PAUSED, COMPLETED, CANCELLED

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-INVALID |
| Mục tiêu | GET /v1/accounts/{accountId}/scheduled-transfers: status (query) ngoài danh sách ACTIVE, PAUSED, COMPLETED, CANCELLED |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules · TD TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-INVALID · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| status (query) | fld-listschedules-status-enum-invalid | <ngoài danh sách> | GET /v1/accounts/{accountId}/scheduled-transfers: status (query) ngoài danh sách ACTIVE, PAUSED, COMPLETED, CANCELLED |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho GET /v1/accounts/{accountId}/scheduled-transfers trừ trường status (query); phụ thuộc ngoài trả thành công | EP-ListSchedules |  |
| 2 | when | Gọi GET /v1/accounts/{accountId}/scheduled-transfers với status (query) = (giá trị ngoài danh sách) | EP-ListSchedules | status (query)=(giá trị ngoài danh sách) |
| 3 | then | Yêu cầu bị từ chối và không có tác dụng phụ | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 400 |  |
| 2 | errorCode | equals | INVALID_REQUEST |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-PAUSED: GET /v1/accounts/{accountId}/scheduled-transfers: status (query) = PAUSED (giá trị hợp lệ)

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-PAUSED |
| Mục tiêu | GET /v1/accounts/{accountId}/scheduled-transfers: status (query) = PAUSED (giá trị hợp lệ) |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules · TD TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-PAUSED · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| status (query) | fld-listschedules-status-enum-paused | PAUSED | GET /v1/accounts/{accountId}/scheduled-transfers: status (query) = PAUSED (giá trị hợp lệ) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho GET /v1/accounts/{accountId}/scheduled-transfers trừ trường status (query); phụ thuộc ngoài trả thành công | EP-ListSchedules |  |
| 2 | when | Gọi GET /v1/accounts/{accountId}/scheduled-transfers với status (query) = PAUSED | EP-ListSchedules | status (query)=PAUSED |
| 3 | then | Yêu cầu được chấp nhận | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-RSP-ListSchedules-200: GET /v1/accounts/{accountId}/scheduled-transfers trả 200

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-RSP-ListSchedules-200 |
| Mục tiêu | GET /v1/accounts/{accountId}/scheduled-transfers trả 200 |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules · TD TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-RSP-ListSchedules-200 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Trả danh sách lịch của tài khoản, phân trang, lọc theo status |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-listschedules-200 | 200 | GET /v1/accounts/{accountId}/scheduled-transfers → 200: Trả danh sách lịch của tài khoản, phân trang, lọc theo status |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Trả danh sách lịch của tài khoản, phân trang, lọc theo status | EP-ListSchedules |  |
| 2 | when | Gọi GET /v1/accounts/{accountId}/scheduled-transfers trong điều kiện trên | EP-ListSchedules |  |
| 3 | then | Phản hồi 200 | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-RSP-ListSchedules-400: GET /v1/accounts/{accountId}/scheduled-transfers trả 400 INVALID_REQUEST

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-RSP-ListSchedules-400 |
| Mục tiêu | GET /v1/accounts/{accountId}/scheduled-transfers trả 400 INVALID_REQUEST |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules · TD TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-RSP-ListSchedules-400 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | status ngoài enum |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-listschedules-400 | 400 INVALID_REQUEST | GET /v1/accounts/{accountId}/scheduled-transfers → 400 INVALID_REQUEST: status ngoài enum |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | status ngoài enum | EP-ListSchedules |  |
| 2 | when | Gọi GET /v1/accounts/{accountId}/scheduled-transfers trong điều kiện trên | EP-ListSchedules |  |
| 3 | then | Phản hồi 400 INVALID_REQUEST | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 400 |  |
| 2 | errorCode | equals | INVALID_REQUEST |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-RSP-ListSchedules-401: GET /v1/accounts/{accountId}/scheduled-transfers trả 401 UNAUTHORIZED

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-RSP-ListSchedules-401 |
| Mục tiêu | GET /v1/accounts/{accountId}/scheduled-transfers trả 401 UNAUTHORIZED |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules · TD TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-RSP-ListSchedules-401 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Thiếu hoặc sai API key |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-listschedules-401 | 401 UNAUTHORIZED | GET /v1/accounts/{accountId}/scheduled-transfers → 401 UNAUTHORIZED: Thiếu hoặc sai API key |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thiếu hoặc sai API key | EP-ListSchedules |  |
| 2 | when | Gọi GET /v1/accounts/{accountId}/scheduled-transfers trong điều kiện trên | EP-ListSchedules |  |
| 3 | then | Phản hồi 401 UNAUTHORIZED | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 401 |  |
| 2 | errorCode | equals | UNAUTHORIZED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-RSP-ListSchedules-403: GET /v1/accounts/{accountId}/scheduled-transfers trả 403 FORBIDDEN

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-RSP-ListSchedules-403 |
| Mục tiêu | GET /v1/accounts/{accountId}/scheduled-transfers trả 403 FORBIDDEN |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-ListSchedules · TD TD-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API-RSP-ListSchedules-403 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | accountId khác tài khoản của người gọi |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-ListSchedules-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-listschedules-403 | 403 FORBIDDEN | GET /v1/accounts/{accountId}/scheduled-transfers → 403 FORBIDDEN: accountId khác tài khoản của người gọi |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | accountId khác tài khoản của người gọi | EP-ListSchedules |  |
| 2 | when | Gọi GET /v1/accounts/{accountId}/scheduled-transfers trong điều kiện trên | EP-ListSchedules |  |
| 3 | then | Phản hồi 403 FORBIDDEN | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 403 |  |
| 2 | errorCode | equals | FORBIDDEN |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-ACTIVE: PATCH /v1/scheduled-transfers/{scheduleId}: status (body) = ACTIVE (giá trị hợp lệ)

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-ACTIVE |
| Mục tiêu | PATCH /v1/scheduled-transfers/{scheduleId}: status (body) = ACTIVE (giá trị hợp lệ) |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus · TD TD-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-ACTIVE · nguồn FR-044, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| status (body) | fld-patch-status-enum-active | ACTIVE | PATCH /v1/scheduled-transfers/{scheduleId}: status (body) = ACTIVE (giá trị hợp lệ) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho PATCH /v1/scheduled-transfers/{scheduleId} trừ trường status (body); phụ thuộc ngoài trả thành công | EP-UpdateScheduleStatus |  |
| 2 | when | Gọi PATCH /v1/scheduled-transfers/{scheduleId} với status (body) = ACTIVE | EP-UpdateScheduleStatus | status (body)=ACTIVE |
| 3 | then | Yêu cầu được chấp nhận | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-INVALID: PATCH /v1/scheduled-transfers/{scheduleId}: status (body) ngoài danh sách PAUSED, ACTIVE

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-INVALID |
| Mục tiêu | PATCH /v1/scheduled-transfers/{scheduleId}: status (body) ngoài danh sách PAUSED, ACTIVE |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus · TD TD-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-INVALID · nguồn FR-044, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| status (body) | fld-patch-status-enum-invalid | <ngoài danh sách> | PATCH /v1/scheduled-transfers/{scheduleId}: status (body) ngoài danh sách PAUSED, ACTIVE |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho PATCH /v1/scheduled-transfers/{scheduleId} trừ trường status (body); phụ thuộc ngoài trả thành công | EP-UpdateScheduleStatus |  |
| 2 | when | Gọi PATCH /v1/scheduled-transfers/{scheduleId} với status (body) = (giá trị ngoài danh sách) | EP-UpdateScheduleStatus | status (body)=(giá trị ngoài danh sách) |
| 3 | then | Yêu cầu bị từ chối và không có tác dụng phụ | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 400 |  |
| 2 | errorCode | equals | INVALID_REQUEST |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-PAUSED: PATCH /v1/scheduled-transfers/{scheduleId}: status (body) = PAUSED (giá trị hợp lệ)

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-PAUSED |
| Mục tiêu | PATCH /v1/scheduled-transfers/{scheduleId}: status (body) = PAUSED (giá trị hợp lệ) |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus · TD TD-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-PAUSED · nguồn FR-044, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| status (body) | fld-patch-status-enum-paused | PAUSED | PATCH /v1/scheduled-transfers/{scheduleId}: status (body) = PAUSED (giá trị hợp lệ) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Yêu cầu hợp lệ cho PATCH /v1/scheduled-transfers/{scheduleId} trừ trường status (body); phụ thuộc ngoài trả thành công | EP-UpdateScheduleStatus |  |
| 2 | when | Gọi PATCH /v1/scheduled-transfers/{scheduleId} với status (body) = PAUSED | EP-UpdateScheduleStatus | status (body)=PAUSED |
| 3 | then | Yêu cầu được chấp nhận | MOD-Audit, MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-200: PATCH /v1/scheduled-transfers/{scheduleId} trả 200

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-200 |
| Mục tiêu | PATCH /v1/scheduled-transfers/{scheduleId} trả 200 |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus · TD TD-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-RSP-Patch-200 · nguồn FR-044, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Chuyển PAUSED hoặc ACTIVE thành công, trả lịch |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-patch-200 | 200 | PATCH /v1/scheduled-transfers/{scheduleId} → 200: Chuyển PAUSED hoặc ACTIVE thành công, trả lịch |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Chuyển PAUSED hoặc ACTIVE thành công, trả lịch | EP-UpdateScheduleStatus |  |
| 2 | when | Gọi PATCH /v1/scheduled-transfers/{scheduleId} trong điều kiện trên | EP-UpdateScheduleStatus |  |
| 3 | then | Phản hồi 200 | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-400: PATCH /v1/scheduled-transfers/{scheduleId} trả 400 INVALID_REQUEST

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-400 |
| Mục tiêu | PATCH /v1/scheduled-transfers/{scheduleId} trả 400 INVALID_REQUEST |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus · TD TD-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-RSP-Patch-400 · nguồn FR-044, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | status thiếu hoặc ngoài PAUSED/ACTIVE |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-patch-400 | 400 INVALID_REQUEST | PATCH /v1/scheduled-transfers/{scheduleId} → 400 INVALID_REQUEST: status thiếu hoặc ngoài PAUSED/ACTIVE |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | status thiếu hoặc ngoài PAUSED/ACTIVE | EP-UpdateScheduleStatus |  |
| 2 | when | Gọi PATCH /v1/scheduled-transfers/{scheduleId} trong điều kiện trên | EP-UpdateScheduleStatus |  |
| 3 | then | Phản hồi 400 INVALID_REQUEST | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 400 |  |
| 2 | errorCode | equals | INVALID_REQUEST |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-401: PATCH /v1/scheduled-transfers/{scheduleId} trả 401 UNAUTHORIZED

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-401 |
| Mục tiêu | PATCH /v1/scheduled-transfers/{scheduleId} trả 401 UNAUTHORIZED |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus · TD TD-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-RSP-Patch-401 · nguồn FR-044, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Thiếu hoặc sai API key |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-patch-401 | 401 UNAUTHORIZED | PATCH /v1/scheduled-transfers/{scheduleId} → 401 UNAUTHORIZED: Thiếu hoặc sai API key |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thiếu hoặc sai API key | EP-UpdateScheduleStatus |  |
| 2 | when | Gọi PATCH /v1/scheduled-transfers/{scheduleId} trong điều kiện trên | EP-UpdateScheduleStatus |  |
| 3 | then | Phản hồi 401 UNAUTHORIZED | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 401 |  |
| 2 | errorCode | equals | UNAUTHORIZED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-403: PATCH /v1/scheduled-transfers/{scheduleId} trả 403 FORBIDDEN

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-403 |
| Mục tiêu | PATCH /v1/scheduled-transfers/{scheduleId} trả 403 FORBIDDEN |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus · TD TD-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-RSP-Patch-403 · nguồn FR-044, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Người gọi không phải chủ lịch |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-patch-403 | 403 FORBIDDEN | PATCH /v1/scheduled-transfers/{scheduleId} → 403 FORBIDDEN: Người gọi không phải chủ lịch |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Người gọi không phải chủ lịch | EP-UpdateScheduleStatus |  |
| 2 | when | Gọi PATCH /v1/scheduled-transfers/{scheduleId} trong điều kiện trên | EP-UpdateScheduleStatus |  |
| 3 | then | Phản hồi 403 FORBIDDEN | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 403 |  |
| 2 | errorCode | equals | FORBIDDEN |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-404: PATCH /v1/scheduled-transfers/{scheduleId} trả 404 SCHEDULE_NOT_FOUND

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-404 |
| Mục tiêu | PATCH /v1/scheduled-transfers/{scheduleId} trả 404 SCHEDULE_NOT_FOUND |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus · TD TD-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-RSP-Patch-404 · nguồn FR-044, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Không có lịch với scheduleId |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-patch-404 | 404 SCHEDULE_NOT_FOUND | PATCH /v1/scheduled-transfers/{scheduleId} → 404 SCHEDULE_NOT_FOUND: Không có lịch với scheduleId |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Không có lịch với scheduleId | EP-UpdateScheduleStatus |  |
| 2 | when | Gọi PATCH /v1/scheduled-transfers/{scheduleId} trong điều kiện trên | EP-UpdateScheduleStatus |  |
| 3 | then | Phản hồi 404 SCHEDULE_NOT_FOUND | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 404 |  |
| 2 | errorCode | equals | SCHEDULE_NOT_FOUND |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-409: PATCH /v1/scheduled-transfers/{scheduleId} trả 409 SCHEDULE_STATE_INVALID

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-409 |
| Mục tiêu | PATCH /v1/scheduled-transfers/{scheduleId} trả 409 SCHEDULE_STATE_INVALID |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus · TD TD-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API · TCOV TCOV-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API-RSP-Patch-409 · nguồn FR-044, FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | Tạm dừng lịch không ACTIVE, tiếp tục lịch không PAUSED, hoặc lịch ONCE |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-EP-UpdateScheduleStatus-API |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| response | rsp-patch-409 | 409 SCHEDULE_STATE_INVALID | PATCH /v1/scheduled-transfers/{scheduleId} → 409 SCHEDULE_STATE_INVALID: Tạm dừng lịch không ACTIVE, tiếp tục lịch không PAUSED, hoặc lịch ONCE |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Tạm dừng lịch không ACTIVE, tiếp tục lịch không PAUSED, hoặc lịch ONCE | EP-UpdateScheduleStatus |  |
| 2 | when | Gọi PATCH /v1/scheduled-transfers/{scheduleId} trong điều kiện trên | EP-UpdateScheduleStatus |  |
| 3 | then | Phản hồi 409 SCHEDULE_STATE_INVALID | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 409 |  |
| 2 | errorCode | equals | SCHEDULE_STATE_INVALID |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-1: Xác nhận FR-040 thỏa: POST tạo lịch ONCE hợp lệ trả 201, có scheduleId và status ACTIVE

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-1 |
| Mục tiêu | Xác nhận FR-040 thỏa: POST tạo lịch ONCE hợp lệ trả 201, có scheduleId và status ACTIVE |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-040 · TD TD-SCHEDULED-TRANSFER-L2-FR-040-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-040-AC-AC-FR-040-1 · nguồn FR-040 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-040-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-040-1 | ac-fr-040-1 | POST tạo lịch ONCE hợp lệ trả 201, có scheduleId và status ACTIVE | Tiêu chí chấp nhận AC-FR-040-1 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-040 theo tiêu chí: POST tạo lịch ONCE hợp lệ trả 201, có scheduleId và status ACTIVE | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | POST tạo lịch ONCE hợp lệ trả 201, có scheduleId và status ACTIVE |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-2: Xác nhận FR-040 thỏa: Số dư hai tài khoản không đổi và không có bản ghi ledger_entries mới sau khi tạo lịch

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-2 |
| Mục tiêu | Xác nhận FR-040 thỏa: Số dư hai tài khoản không đổi và không có bản ghi ledger_entries mới sau khi tạo lịch |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-040 · TD TD-SCHEDULED-TRANSFER-L2-FR-040-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-040-AC-AC-FR-040-2 · nguồn FR-040 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-040-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-040-2 | ac-fr-040-2 | Số dư hai tài khoản không đổi và không có bản ghi ledger_entries mới sau khi tạo lịch | Tiêu chí chấp nhận AC-FR-040-2 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-040 theo tiêu chí: Số dư hai tài khoản không đổi và không có bản ghi ledger_entries mới sau khi tạo lịch | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Số dư hai tài khoản không đổi và không có bản ghi ledger_entries mới sau khi tạo lịch |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-3: Xác nhận FR-040 thỏa: firstRunAt cách hiện tại < 5 phút hoặc > 90 ngày thì trả 422 SCHEDULE_TIME_INVALID

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-3 |
| Mục tiêu | Xác nhận FR-040 thỏa: firstRunAt cách hiện tại < 5 phút hoặc > 90 ngày thì trả 422 SCHEDULE_TIME_INVALID |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-040 · TD TD-SCHEDULED-TRANSFER-L2-FR-040-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-040-AC-AC-FR-040-3 · nguồn FR-040 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-040-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-040-3 | ac-fr-040-3 | firstRunAt cách hiện tại < 5 phút hoặc > 90 ngày thì trả 422 SCHEDULE_TIME_INVALID | Tiêu chí chấp nhận AC-FR-040-3 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-040 theo tiêu chí: firstRunAt cách hiện tại < 5 phút hoặc > 90 ngày thì trả 422 SCHEDULE_TIME_INVALID | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | firstRunAt cách hiện tại < 5 phút hoặc > 90 ngày thì trả 422 SCHEDULE_TIME_INVALID |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-4: Xác nhận FR-040 thỏa: toAccountId không tồn tại thì trả 404 ACCOUNT_NOT_FOUND

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-4 |
| Mục tiêu | Xác nhận FR-040 thỏa: toAccountId không tồn tại thì trả 404 ACCOUNT_NOT_FOUND |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-040 · TD TD-SCHEDULED-TRANSFER-L2-FR-040-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-040-AC-AC-FR-040-4 · nguồn FR-040 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-040-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-040-4 | ac-fr-040-4 | toAccountId không tồn tại thì trả 404 ACCOUNT_NOT_FOUND | Tiêu chí chấp nhận AC-FR-040-4 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-040 theo tiêu chí: toAccountId không tồn tại thì trả 404 ACCOUNT_NOT_FOUND | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | toAccountId không tồn tại thì trả 404 ACCOUNT_NOT_FOUND |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-5: Xác nhận FR-040 thỏa: fromAccountId khác chủ API key thì trả 403 FORBIDDEN

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-5 |
| Mục tiêu | Xác nhận FR-040 thỏa: fromAccountId khác chủ API key thì trả 403 FORBIDDEN |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-040 · TD TD-SCHEDULED-TRANSFER-L2-FR-040-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-040-AC-AC-FR-040-5 · nguồn FR-040 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-040-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-040-5 | ac-fr-040-5 | fromAccountId khác chủ API key thì trả 403 FORBIDDEN | Tiêu chí chấp nhận AC-FR-040-5 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-040 theo tiêu chí: fromAccountId khác chủ API key thì trả 403 FORBIDDEN | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | fromAccountId khác chủ API key thì trả 403 FORBIDDEN |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-6: Xác nhận FR-040 thỏa: Lịch thứ 21 ACTIVE/PAUSED của một tài khoản trả 422 SCHEDULE_LIMIT_REACHED

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-6 |
| Mục tiêu | Xác nhận FR-040 thỏa: Lịch thứ 21 ACTIVE/PAUSED của một tài khoản trả 422 SCHEDULE_LIMIT_REACHED |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-040 · TD TD-SCHEDULED-TRANSFER-L2-FR-040-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-040-AC-AC-FR-040-6 · nguồn FR-040 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-040-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-040-6 | ac-fr-040-6 | Lịch thứ 21 ACTIVE/PAUSED của một tài khoản trả 422 SCHEDULE_LIMIT_REACHED | Tiêu chí chấp nhận AC-FR-040-6 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-040 theo tiêu chí: Lịch thứ 21 ACTIVE/PAUSED của một tài khoản trả 422 SCHEDULE_LIMIT_REACHED | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Lịch thứ 21 ACTIVE/PAUSED của một tài khoản trả 422 SCHEDULE_LIMIT_REACHED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-REV-003: Xác nhận FR-040 thỏa: Chỉ lịch ACTIVE hoặc PAUSED được tính vào giới hạn 20 lịch của tài khoản; lịch COMPLETED hoặc CANCELLED không được tính

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-REV-003 |
| Mục tiêu | Xác nhận FR-040 thỏa: Chỉ lịch ACTIVE hoặc PAUSED được tính vào giới hạn 20 lịch của tài khoản; lịch COMPLETED hoặc CANCELLED không được tính |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-040 · TD TD-SCHEDULED-TRANSFER-L2-FR-040-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-040-AC-AC-REV-003 · nguồn FR-040 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-040-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-REV-003 | ac-rev-003 | Chỉ lịch ACTIVE hoặc PAUSED được tính vào giới hạn 20 lịch của tài khoản; lịch COMPLETED hoặc CANCELLED không được tính | Tiêu chí chấp nhận AC-REV-003 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-040 theo tiêu chí: Chỉ lịch ACTIVE hoặc PAUSED được tính vào giới hạn 20 lịch của tài khoản; lịch COMPLETED hoặc CANCELLED không được tính | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Chỉ lịch ACTIVE hoặc PAUSED được tính vào giới hạn 20 lịch của tài khoản; lịch COMPLETED hoặc CANCELLED không được tính |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-FR-041-1: Xác nhận FR-041 thỏa: Lịch DAILY, WEEKLY hoặc MONTHLY có endDate hoặc maxRuns được tạo thành công (201)

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-FR-041-1 |
| Mục tiêu | Xác nhận FR-041 thỏa: Lịch DAILY, WEEKLY hoặc MONTHLY có endDate hoặc maxRuns được tạo thành công (201) |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-041 · TD TD-SCHEDULED-TRANSFER-L2-FR-041-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-041-AC-AC-FR-041-1 · nguồn FR-041 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-041-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-041-1 | ac-fr-041-1 | Lịch DAILY, WEEKLY hoặc MONTHLY có endDate hoặc maxRuns được tạo thành công (201) | Tiêu chí chấp nhận AC-FR-041-1 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-041 theo tiêu chí: Lịch DAILY, WEEKLY hoặc MONTHLY có endDate hoặc maxRuns được tạo thành công (201) | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Lịch DAILY, WEEKLY hoặc MONTHLY có endDate hoặc maxRuns được tạo thành công (201) |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-FR-041-2: Xác nhận FR-041 thỏa: Lịch định kỳ thiếu cả endDate và maxRuns trả 400 INVALID_REQUEST

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-FR-041-2 |
| Mục tiêu | Xác nhận FR-041 thỏa: Lịch định kỳ thiếu cả endDate và maxRuns trả 400 INVALID_REQUEST |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-041 · TD TD-SCHEDULED-TRANSFER-L2-FR-041-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-041-AC-AC-FR-041-2 · nguồn FR-041 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-041-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-041-2 | ac-fr-041-2 | Lịch định kỳ thiếu cả endDate và maxRuns trả 400 INVALID_REQUEST | Tiêu chí chấp nhận AC-FR-041-2 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-041 theo tiêu chí: Lịch định kỳ thiếu cả endDate và maxRuns trả 400 INVALID_REQUEST | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Lịch định kỳ thiếu cả endDate và maxRuns trả 400 INVALID_REQUEST |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-FR-041-3: Xác nhận FR-041 thỏa: Sau khi một lần chạy kết thúc, hệ thống sinh lần chạy PENDING kế tiếp đúng chu kỳ cho đến khi đạt maxRuns hoặc endDate, rồi lịch COMPLETED

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-FR-041-3 |
| Mục tiêu | Xác nhận FR-041 thỏa: Sau khi một lần chạy kết thúc, hệ thống sinh lần chạy PENDING kế tiếp đúng chu kỳ cho đến khi đạt maxRuns hoặc endDate, rồi lịch COMPLETED |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-041 · TD TD-SCHEDULED-TRANSFER-L2-FR-041-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-041-AC-AC-FR-041-3 · nguồn FR-041 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-041-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-041-3 | ac-fr-041-3 | Sau khi một lần chạy kết thúc, hệ thống sinh lần chạy PENDING kế tiếp đúng chu kỳ cho đến khi đạt maxRuns hoặc endDate, rồi lịch COMPLETED | Tiêu chí chấp nhận AC-FR-041-3 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-041 theo tiêu chí: Sau khi một lần chạy kết thúc, hệ thống sinh lần chạy PENDING kế tiếp đúng chu kỳ cho đến khi đạt maxRuns hoặc endDate, rồi lịch COMPLETED | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Sau khi một lần chạy kết thúc, hệ thống sinh lần chạy PENDING kế tiếp đúng chu kỳ cho đến khi đạt maxRuns hoặc endDate, rồi lịch COMPLETED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-FR-041-4: Xác nhận FR-041 thỏa: Lịch MONTHLY đặt ngày 29, 30 hoặc 31 chạy vào ngày cuối tháng ở tháng không đủ ngày, và ngày 29/02 ở năm nhuận

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-FR-041-4 |
| Mục tiêu | Xác nhận FR-041 thỏa: Lịch MONTHLY đặt ngày 29, 30 hoặc 31 chạy vào ngày cuối tháng ở tháng không đủ ngày, và ngày 29/02 ở năm nhuận |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-041 · TD TD-SCHEDULED-TRANSFER-L2-FR-041-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-041-AC-AC-FR-041-4 · nguồn FR-041 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-041-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-041-4 | ac-fr-041-4 | Lịch MONTHLY đặt ngày 29, 30 hoặc 31 chạy vào ngày cuối tháng ở tháng không đủ ngày, và ngày 29/02 ở năm nhuận | Tiêu chí chấp nhận AC-FR-041-4 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-041 theo tiêu chí: Lịch MONTHLY đặt ngày 29, 30 hoặc 31 chạy vào ngày cuối tháng ở tháng không đủ ngày, và ngày 29/02 ở năm nhuận | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Lịch MONTHLY đặt ngày 29, 30 hoặc 31 chạy vào ngày cuối tháng ở tháng không đủ ngày, và ngày 29/02 ở năm nhuận |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-REV-001: Xác nhận FR-041 thỏa: Lịch định kỳ có endDate trước ngày của lần chạy đầu, hoặc endDate quá 366 ngày kể từ lần chạy đầu, thì trả 422 SCHEDULE_TIME_INVALID

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-REV-001 |
| Mục tiêu | Xác nhận FR-041 thỏa: Lịch định kỳ có endDate trước ngày của lần chạy đầu, hoặc endDate quá 366 ngày kể từ lần chạy đầu, thì trả 422 SCHEDULE_TIME_INVALID |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-041 · TD TD-SCHEDULED-TRANSFER-L2-FR-041-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-041-AC-AC-REV-001 · nguồn FR-041 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-041-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-REV-001 | ac-rev-001 | Lịch định kỳ có endDate trước ngày của lần chạy đầu, hoặc endDate quá 366 ngày kể từ lần chạy đầu, thì trả 422 SCHEDULE_TIME_INVALID | Tiêu chí chấp nhận AC-REV-001 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-041 theo tiêu chí: Lịch định kỳ có endDate trước ngày của lần chạy đầu, hoặc endDate quá 366 ngày kể từ lần chạy đầu, thì trả 422 SCHEDULE_TIME_INVALID | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Lịch định kỳ có endDate trước ngày của lần chạy đầu, hoặc endDate quá 366 ngày kể từ lần chạy đầu, thì trả 422 SCHEDULE_TIME_INVALID |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-REV-002: Xác nhận FR-041 thỏa: Lịch định kỳ có cả endDate và maxRuns kết thúc (COMPLETED) ở điều kiện đạt trước, không sinh thêm lần chạy PENDING

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-REV-002 |
| Mục tiêu | Xác nhận FR-041 thỏa: Lịch định kỳ có cả endDate và maxRuns kết thúc (COMPLETED) ở điều kiện đạt trước, không sinh thêm lần chạy PENDING |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-041 · TD TD-SCHEDULED-TRANSFER-L2-FR-041-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-041-AC-AC-REV-002 · nguồn FR-041 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-041-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-REV-002 | ac-rev-002 | Lịch định kỳ có cả endDate và maxRuns kết thúc (COMPLETED) ở điều kiện đạt trước, không sinh thêm lần chạy PENDING | Tiêu chí chấp nhận AC-REV-002 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-041 theo tiêu chí: Lịch định kỳ có cả endDate và maxRuns kết thúc (COMPLETED) ở điều kiện đạt trước, không sinh thêm lần chạy PENDING | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Lịch định kỳ có cả endDate và maxRuns kết thúc (COMPLETED) ở điều kiện đạt trước, không sinh thêm lần chạy PENDING |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-042-AC-AC-FR-042-1: Xác nhận FR-042 thỏa: GET một lịch trả status và nextRunAt

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-042-AC-AC-FR-042-1 |
| Mục tiêu | Xác nhận FR-042 thỏa: GET một lịch trả status và nextRunAt |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-042 · TD TD-SCHEDULED-TRANSFER-L2-FR-042-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-042-AC-AC-FR-042-1 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-042-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-042-1 | ac-fr-042-1 | GET một lịch trả status và nextRunAt | Tiêu chí chấp nhận AC-FR-042-1 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-042 theo tiêu chí: GET một lịch trả status và nextRunAt | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | GET một lịch trả status và nextRunAt |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-042-AC-AC-FR-042-2: Xác nhận FR-042 thỏa: GET danh sách lịch của tài khoản hỗ trợ lọc status và phân trang limit, offset

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-042-AC-AC-FR-042-2 |
| Mục tiêu | Xác nhận FR-042 thỏa: GET danh sách lịch của tài khoản hỗ trợ lọc status và phân trang limit, offset |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-042 · TD TD-SCHEDULED-TRANSFER-L2-FR-042-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-042-AC-AC-FR-042-2 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-042-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-042-2 | ac-fr-042-2 | GET danh sách lịch của tài khoản hỗ trợ lọc status và phân trang limit, offset | Tiêu chí chấp nhận AC-FR-042-2 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-042 theo tiêu chí: GET danh sách lịch của tài khoản hỗ trợ lọc status và phân trang limit, offset | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | GET danh sách lịch của tài khoản hỗ trợ lọc status và phân trang limit, offset |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-042-AC-AC-FR-042-3: Xác nhận FR-042 thỏa: GET runs trả mỗi lần chạy với scheduledAt, status, transferId và reasonCode, sắp mới nhất trước

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-042-AC-AC-FR-042-3 |
| Mục tiêu | Xác nhận FR-042 thỏa: GET runs trả mỗi lần chạy với scheduledAt, status, transferId và reasonCode, sắp mới nhất trước |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-042 · TD TD-SCHEDULED-TRANSFER-L2-FR-042-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-042-AC-AC-FR-042-3 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-042-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-042-3 | ac-fr-042-3 | GET runs trả mỗi lần chạy với scheduledAt, status, transferId và reasonCode, sắp mới nhất trước | Tiêu chí chấp nhận AC-FR-042-3 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-042 theo tiêu chí: GET runs trả mỗi lần chạy với scheduledAt, status, transferId và reasonCode, sắp mới nhất trước | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | GET runs trả mỗi lần chạy với scheduledAt, status, transferId và reasonCode, sắp mới nhất trước |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-042-AC-AC-FR-042-4: Xác nhận FR-042 thỏa: Người không phải chủ tài khoản nguồn nhận 403 FORBIDDEN; mã lịch không có nhận 404 SCHEDULE_NOT_FOUND

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-042-AC-AC-FR-042-4 |
| Mục tiêu | Xác nhận FR-042 thỏa: Người không phải chủ tài khoản nguồn nhận 403 FORBIDDEN; mã lịch không có nhận 404 SCHEDULE_NOT_FOUND |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-042 · TD TD-SCHEDULED-TRANSFER-L2-FR-042-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-042-AC-AC-FR-042-4 · nguồn FR-042 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-042-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-042-4 | ac-fr-042-4 | Người không phải chủ tài khoản nguồn nhận 403 FORBIDDEN; mã lịch không có nhận 404 SCHEDULE_NOT_FOUND | Tiêu chí chấp nhận AC-FR-042-4 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-042 theo tiêu chí: Người không phải chủ tài khoản nguồn nhận 403 FORBIDDEN; mã lịch không có nhận 404 SCHEDULE_NOT_FOUND | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Người không phải chủ tài khoản nguồn nhận 403 FORBIDDEN; mã lịch không có nhận 404 SCHEDULE_NOT_FOUND |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-043-AC-AC-FR-043-1: Xác nhận FR-043 thỏa: DELETE lịch ACTIVE đổi lịch thành CANCELLED và các lần chạy PENDING thành CANCELLED

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-043-AC-AC-FR-043-1 |
| Mục tiêu | Xác nhận FR-043 thỏa: DELETE lịch ACTIVE đổi lịch thành CANCELLED và các lần chạy PENDING thành CANCELLED |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-043 · TD TD-SCHEDULED-TRANSFER-L2-FR-043-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-043-AC-AC-FR-043-1 · nguồn FR-043 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-043-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-043-1 | ac-fr-043-1 | DELETE lịch ACTIVE đổi lịch thành CANCELLED và các lần chạy PENDING thành CANCELLED | Tiêu chí chấp nhận AC-FR-043-1 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-043 theo tiêu chí: DELETE lịch ACTIVE đổi lịch thành CANCELLED và các lần chạy PENDING thành CANCELLED | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | DELETE lịch ACTIVE đổi lịch thành CANCELLED và các lần chạy PENDING thành CANCELLED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-043-AC-AC-FR-043-2: Xác nhận FR-043 thỏa: DELETE lịch đã CANCELLED trả 200 với kết quả cũ

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-043-AC-AC-FR-043-2 |
| Mục tiêu | Xác nhận FR-043 thỏa: DELETE lịch đã CANCELLED trả 200 với kết quả cũ |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-043 · TD TD-SCHEDULED-TRANSFER-L2-FR-043-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-043-AC-AC-FR-043-2 · nguồn FR-043 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-043-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-043-2 | ac-fr-043-2 | DELETE lịch đã CANCELLED trả 200 với kết quả cũ | Tiêu chí chấp nhận AC-FR-043-2 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-043 theo tiêu chí: DELETE lịch đã CANCELLED trả 200 với kết quả cũ | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | DELETE lịch đã CANCELLED trả 200 với kết quả cũ |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-043-AC-AC-FR-043-3: Xác nhận FR-043 thỏa: DELETE khi có lần chạy RUNNING hoặc lịch COMPLETED trả 409 SCHEDULE_NOT_CANCELLABLE

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-043-AC-AC-FR-043-3 |
| Mục tiêu | Xác nhận FR-043 thỏa: DELETE khi có lần chạy RUNNING hoặc lịch COMPLETED trả 409 SCHEDULE_NOT_CANCELLABLE |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-043 · TD TD-SCHEDULED-TRANSFER-L2-FR-043-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-043-AC-AC-FR-043-3 · nguồn FR-043 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-043-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-043-3 | ac-fr-043-3 | DELETE khi có lần chạy RUNNING hoặc lịch COMPLETED trả 409 SCHEDULE_NOT_CANCELLABLE | Tiêu chí chấp nhận AC-FR-043-3 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-043 theo tiêu chí: DELETE khi có lần chạy RUNNING hoặc lịch COMPLETED trả 409 SCHEDULE_NOT_CANCELLABLE | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | DELETE khi có lần chạy RUNNING hoặc lịch COMPLETED trả 409 SCHEDULE_NOT_CANCELLABLE |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-044-AC-AC-FR-044-1: Xác nhận FR-044 thỏa: PATCH status=PAUSED trên lịch định kỳ ACTIVE đổi sang PAUSED, scheduler không chạy lịch này

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-044-AC-AC-FR-044-1 |
| Mục tiêu | Xác nhận FR-044 thỏa: PATCH status=PAUSED trên lịch định kỳ ACTIVE đổi sang PAUSED, scheduler không chạy lịch này |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-044 · TD TD-SCHEDULED-TRANSFER-L2-FR-044-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-044-AC-AC-FR-044-1 · nguồn FR-044 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-044-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-044-1 | ac-fr-044-1 | PATCH status=PAUSED trên lịch định kỳ ACTIVE đổi sang PAUSED, scheduler không chạy lịch này | Tiêu chí chấp nhận AC-FR-044-1 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-044 theo tiêu chí: PATCH status=PAUSED trên lịch định kỳ ACTIVE đổi sang PAUSED, scheduler không chạy lịch này | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | PATCH status=PAUSED trên lịch định kỳ ACTIVE đổi sang PAUSED, scheduler không chạy lịch này |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-044-AC-AC-FR-044-2: Xác nhận FR-044 thỏa: PATCH status=ACTIVE trên lịch PAUSED đổi sang ACTIVE và các lần đã bỏ lỡ trong lúc tạm dừng có trạng thái SKIPPED, không bị chạy bù

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-044-AC-AC-FR-044-2 |
| Mục tiêu | Xác nhận FR-044 thỏa: PATCH status=ACTIVE trên lịch PAUSED đổi sang ACTIVE và các lần đã bỏ lỡ trong lúc tạm dừng có trạng thái SKIPPED, không bị chạy bù |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-044 · TD TD-SCHEDULED-TRANSFER-L2-FR-044-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-044-AC-AC-FR-044-2 · nguồn FR-044 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-044-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-044-2 | ac-fr-044-2 | PATCH status=ACTIVE trên lịch PAUSED đổi sang ACTIVE và các lần đã bỏ lỡ trong lúc tạm dừng có trạng thái SKIPPED, không bị chạy bù | Tiêu chí chấp nhận AC-FR-044-2 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-044 theo tiêu chí: PATCH status=ACTIVE trên lịch PAUSED đổi sang ACTIVE và các lần đã bỏ lỡ trong lúc tạm dừng có trạng thái SKIPPED, không bị chạy bù | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | PATCH status=ACTIVE trên lịch PAUSED đổi sang ACTIVE và các lần đã bỏ lỡ trong lúc tạm dừng có trạng thái SKIPPED, không bị chạy bù |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-044-AC-AC-FR-044-3: Xác nhận FR-044 thỏa: PATCH sai trạng thái nguồn hoặc trên lịch ONCE trả 409 SCHEDULE_STATE_INVALID

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-044-AC-AC-FR-044-3 |
| Mục tiêu | Xác nhận FR-044 thỏa: PATCH sai trạng thái nguồn hoặc trên lịch ONCE trả 409 SCHEDULE_STATE_INVALID |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-044 · TD TD-SCHEDULED-TRANSFER-L2-FR-044-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-044-AC-AC-FR-044-3 · nguồn FR-044 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-044-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-044-3 | ac-fr-044-3 | PATCH sai trạng thái nguồn hoặc trên lịch ONCE trả 409 SCHEDULE_STATE_INVALID | Tiêu chí chấp nhận AC-FR-044-3 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-044 theo tiêu chí: PATCH sai trạng thái nguồn hoặc trên lịch ONCE trả 409 SCHEDULE_STATE_INVALID | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | PATCH sai trạng thái nguồn hoặc trên lịch ONCE trả 409 SCHEDULE_STATE_INVALID |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-FR-045-1: Xác nhận FR-045 thỏa: Lần chạy đến hạn tạo đúng một giao dịch COMPLETED qua luồng chuyển tiền hiện có, run SUCCEEDED có transferId

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-FR-045-1 |
| Mục tiêu | Xác nhận FR-045 thỏa: Lần chạy đến hạn tạo đúng một giao dịch COMPLETED qua luồng chuyển tiền hiện có, run SUCCEEDED có transferId |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-045 · TD TD-SCHEDULED-TRANSFER-L2-FR-045-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-045-AC-AC-FR-045-1 · nguồn FR-045 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-045-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-045-1 | ac-fr-045-1 | Lần chạy đến hạn tạo đúng một giao dịch COMPLETED qua luồng chuyển tiền hiện có, run SUCCEEDED có transferId | Tiêu chí chấp nhận AC-FR-045-1 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-045 theo tiêu chí: Lần chạy đến hạn tạo đúng một giao dịch COMPLETED qua luồng chuyển tiền hiện có, run SUCCEEDED có transferId | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Lần chạy đến hạn tạo đúng một giao dịch COMPLETED qua luồng chuyển tiền hiện có, run SUCCEEDED có transferId |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-FR-045-2: Xác nhận FR-045 thỏa: Idempotency-Key của lần chạy đầu bằng mã lịch + thời điểm dự kiến

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-FR-045-2 |
| Mục tiêu | Xác nhận FR-045 thỏa: Idempotency-Key của lần chạy đầu bằng mã lịch + thời điểm dự kiến |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-045 · TD TD-SCHEDULED-TRANSFER-L2-FR-045-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-045-AC-AC-FR-045-2 · nguồn FR-045 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-045-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-045-2 | ac-fr-045-2 | Idempotency-Key của lần chạy đầu bằng mã lịch + thời điểm dự kiến | Tiêu chí chấp nhận AC-FR-045-2 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-045 theo tiêu chí: Idempotency-Key của lần chạy đầu bằng mã lịch + thời điểm dự kiến | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Idempotency-Key của lần chạy đầu bằng mã lịch + thời điểm dự kiến |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-FR-045-3: Xác nhận FR-045 thỏa: Hạn mức ngày, số dư, trạng thái tài khoản và Fraud Check được kiểm tra tại lúc chạy, không phải lúc tạo lịch

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-FR-045-3 |
| Mục tiêu | Xác nhận FR-045 thỏa: Hạn mức ngày, số dư, trạng thái tài khoản và Fraud Check được kiểm tra tại lúc chạy, không phải lúc tạo lịch |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-045 · TD TD-SCHEDULED-TRANSFER-L2-FR-045-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-045-AC-AC-FR-045-3 · nguồn FR-045 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-045-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-045-3 | ac-fr-045-3 | Hạn mức ngày, số dư, trạng thái tài khoản và Fraud Check được kiểm tra tại lúc chạy, không phải lúc tạo lịch | Tiêu chí chấp nhận AC-FR-045-3 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-045 theo tiêu chí: Hạn mức ngày, số dư, trạng thái tài khoản và Fraud Check được kiểm tra tại lúc chạy, không phải lúc tạo lịch | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Hạn mức ngày, số dư, trạng thái tài khoản và Fraud Check được kiểm tra tại lúc chạy, không phải lúc tạo lịch |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-FR-045-4: Xác nhận FR-045 thỏa: Giao dịch sinh từ lịch được tính vào hạn mức ngày và hiển thị trong lịch sử giao dịch

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-FR-045-4 |
| Mục tiêu | Xác nhận FR-045 thỏa: Giao dịch sinh từ lịch được tính vào hạn mức ngày và hiển thị trong lịch sử giao dịch |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-045 · TD TD-SCHEDULED-TRANSFER-L2-FR-045-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-045-AC-AC-FR-045-4 · nguồn FR-045 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-045-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-045-4 | ac-fr-045-4 | Giao dịch sinh từ lịch được tính vào hạn mức ngày và hiển thị trong lịch sử giao dịch | Tiêu chí chấp nhận AC-FR-045-4 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-045 theo tiêu chí: Giao dịch sinh từ lịch được tính vào hạn mức ngày và hiển thị trong lịch sử giao dịch | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Giao dịch sinh từ lịch được tính vào hạn mức ngày và hiển thị trong lịch sử giao dịch |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-REV-004: Xác nhận FR-045 thỏa: Giao dịch sinh từ lịch hiển thị trong lịch sử giao dịch (FR-013) kèm trường scheduleId là mã lịch

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-REV-004 |
| Mục tiêu | Xác nhận FR-045 thỏa: Giao dịch sinh từ lịch hiển thị trong lịch sử giao dịch (FR-013) kèm trường scheduleId là mã lịch |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-045 · TD TD-SCHEDULED-TRANSFER-L2-FR-045-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-045-AC-AC-REV-004 · nguồn FR-045 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-045-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-REV-004 | ac-rev-004 | Giao dịch sinh từ lịch hiển thị trong lịch sử giao dịch (FR-013) kèm trường scheduleId là mã lịch | Tiêu chí chấp nhận AC-REV-004 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-045 theo tiêu chí: Giao dịch sinh từ lịch hiển thị trong lịch sử giao dịch (FR-013) kèm trường scheduleId là mã lịch | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Giao dịch sinh từ lịch hiển thị trong lịch sử giao dịch (FR-013) kèm trường scheduleId là mã lịch |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-045-SCN-SCN-ScheduledRun: Chạy trọn luồng chính SCN-ScheduledRun

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-045-SCN-SCN-ScheduledRun |
| Mục tiêu | Chạy trọn luồng chính SCN-ScheduledRun |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-045 · TD TD-SCHEDULED-TRANSFER-L2-FR-045-SCN · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-045-SCN-SCN-ScheduledRun · nguồn FR-045 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-045-SCN |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| SCN-ScheduledRun | scn-scheduledrun | 8 | Luồng chính SCN-ScheduledRun |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống và các Module tham gia ở trạng thái ban đầu; stub hệ thống ngoài thành công | MOD-API, MOD-Scheduler, MOD-Transfer |  |
| 2 | when | Quét mỗi 30 giây | — |  |
| 3 | when | chọn tối đa 20 lần chạy đến hạn | — |  |
| 4 | when | chiếm lần chạy (UPDATE có điều kiện) | — |  |
| 5 | when | sinh correlationId | — |  |
| 6 | when | lõi chuyển tiền (idempotency, hạn mức/số dư, Fraud Check, ghi sổ + outbox) | — |  |
| 7 | when | cập nhật lần chạy | — |  |
| 8 | when | thử lại/FAILED | — |  |
| 9 | when | audit + sự kiện | — |  |
| 10 | then | Luồng hoàn tất; kiểm trạng thái cuối ở mọi Module tham gia | MOD-API, MOD-Scheduler, MOD-Transfer |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | hoàn tất |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-FR-046-AC-AC-FR-046-1: Xác nhận FR-046 thỏa: FRAUD_CHECK_UNAVAILABLE được thử lại tối đa 3 lần cách 5 phút, hết lượt thì run FAILED với reasonCode

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-046-AC-AC-FR-046-1 |
| Mục tiêu | Xác nhận FR-046 thỏa: FRAUD_CHECK_UNAVAILABLE được thử lại tối đa 3 lần cách 5 phút, hết lượt thì run FAILED với reasonCode |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-046 · TD TD-SCHEDULED-TRANSFER-L2-FR-046-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-046-AC-AC-FR-046-1 · nguồn FR-046 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-046-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-046-1 | ac-fr-046-1 | FRAUD_CHECK_UNAVAILABLE được thử lại tối đa 3 lần cách 5 phút, hết lượt thì run FAILED với reasonCode | Tiêu chí chấp nhận AC-FR-046-1 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-046 theo tiêu chí: FRAUD_CHECK_UNAVAILABLE được thử lại tối đa 3 lần cách 5 phút, hết lượt thì run FAILED với reasonCode | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | FRAUD_CHECK_UNAVAILABLE được thử lại tối đa 3 lần cách 5 phút, hết lượt thì run FAILED với reasonCode |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-046-AC-AC-FR-046-2: Xác nhận FR-046 thỏa: INSUFFICIENT_FUNDS được thử lại tối đa 2 lần cách 1 giờ, hết lượt thì run FAILED

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-046-AC-AC-FR-046-2 |
| Mục tiêu | Xác nhận FR-046 thỏa: INSUFFICIENT_FUNDS được thử lại tối đa 2 lần cách 1 giờ, hết lượt thì run FAILED |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-046 · TD TD-SCHEDULED-TRANSFER-L2-FR-046-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-046-AC-AC-FR-046-2 · nguồn FR-046 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-046-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-046-2 | ac-fr-046-2 | INSUFFICIENT_FUNDS được thử lại tối đa 2 lần cách 1 giờ, hết lượt thì run FAILED | Tiêu chí chấp nhận AC-FR-046-2 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-046 theo tiêu chí: INSUFFICIENT_FUNDS được thử lại tối đa 2 lần cách 1 giờ, hết lượt thì run FAILED | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | INSUFFICIENT_FUNDS được thử lại tối đa 2 lần cách 1 giờ, hết lượt thì run FAILED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-046-AC-AC-FR-046-3: Xác nhận FR-046 thỏa: ACCOUNT_INACTIVE, FRAUD_REJECTED, LIMIT_EXCEEDED làm run FAILED ngay, không thử lại

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-046-AC-AC-FR-046-3 |
| Mục tiêu | Xác nhận FR-046 thỏa: ACCOUNT_INACTIVE, FRAUD_REJECTED, LIMIT_EXCEEDED làm run FAILED ngay, không thử lại |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-046 · TD TD-SCHEDULED-TRANSFER-L2-FR-046-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-046-AC-AC-FR-046-3 · nguồn FR-046 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-046-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-046-3 | ac-fr-046-3 | ACCOUNT_INACTIVE, FRAUD_REJECTED, LIMIT_EXCEEDED làm run FAILED ngay, không thử lại | Tiêu chí chấp nhận AC-FR-046-3 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-046 theo tiêu chí: ACCOUNT_INACTIVE, FRAUD_REJECTED, LIMIT_EXCEEDED làm run FAILED ngay, không thử lại | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | ACCOUNT_INACTIVE, FRAUD_REJECTED, LIMIT_EXCEEDED làm run FAILED ngay, không thử lại |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-046-AC-AC-FR-046-4: Xác nhận FR-046 thỏa: Sau 3 lần chạy FAILED liên tiếp lịch định kỳ chuyển PAUSED

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-046-AC-AC-FR-046-4 |
| Mục tiêu | Xác nhận FR-046 thỏa: Sau 3 lần chạy FAILED liên tiếp lịch định kỳ chuyển PAUSED |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-046 · TD TD-SCHEDULED-TRANSFER-L2-FR-046-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-046-AC-AC-FR-046-4 · nguồn FR-046 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-046-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-046-4 | ac-fr-046-4 | Sau 3 lần chạy FAILED liên tiếp lịch định kỳ chuyển PAUSED | Tiêu chí chấp nhận AC-FR-046-4 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-046 theo tiêu chí: Sau 3 lần chạy FAILED liên tiếp lịch định kỳ chuyển PAUSED | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Sau 3 lần chạy FAILED liên tiếp lịch định kỳ chuyển PAUSED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-046-SCN-SCN-ScheduledRun: Chạy trọn luồng chính SCN-ScheduledRun

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-046-SCN-SCN-ScheduledRun |
| Mục tiêu | Chạy trọn luồng chính SCN-ScheduledRun |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-046 · TD TD-SCHEDULED-TRANSFER-L2-FR-046-SCN · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-046-SCN-SCN-ScheduledRun · nguồn FR-046 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-046-SCN |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| SCN-ScheduledRun | scn-scheduledrun | 8 | Luồng chính SCN-ScheduledRun |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống và các Module tham gia ở trạng thái ban đầu; stub hệ thống ngoài thành công | MOD-API, MOD-Scheduler, MOD-Transfer |  |
| 2 | when | Quét mỗi 30 giây | — |  |
| 3 | when | chọn tối đa 20 lần chạy đến hạn | — |  |
| 4 | when | chiếm lần chạy (UPDATE có điều kiện) | — |  |
| 5 | when | sinh correlationId | — |  |
| 6 | when | lõi chuyển tiền (idempotency, hạn mức/số dư, Fraud Check, ghi sổ + outbox) | — |  |
| 7 | when | cập nhật lần chạy | — |  |
| 8 | when | thử lại/FAILED | — |  |
| 9 | when | audit + sự kiện | — |  |
| 10 | then | Luồng hoàn tất; kiểm trạng thái cuối ở mọi Module tham gia | MOD-API, MOD-Scheduler, MOD-Transfer |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | hoàn tất |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-FR-047-1: Xác nhận FR-047 thỏa: Mỗi lần chạy thành công phát sự kiện ScheduledRunCompleted có transferId qua outbox

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-FR-047-1 |
| Mục tiêu | Xác nhận FR-047 thỏa: Mỗi lần chạy thành công phát sự kiện ScheduledRunCompleted có transferId qua outbox |
| Mức, loại, ưu tiên | L2 · functional · P1 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-047 · TD TD-SCHEDULED-TRANSFER-L2-FR-047-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-047-AC-AC-FR-047-1 · nguồn FR-047 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-02-success, STUB-SCHEDULED-TRANSFER-INT-03-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-047-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-047-1 | ac-fr-047-1 | Mỗi lần chạy thành công phát sự kiện ScheduledRunCompleted có transferId qua outbox | Tiêu chí chấp nhận AC-FR-047-1 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-047 theo tiêu chí: Mỗi lần chạy thành công phát sự kiện ScheduledRunCompleted có transferId qua outbox | MOD-Notify |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Notify |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Mỗi lần chạy thành công phát sự kiện ScheduledRunCompleted có transferId qua outbox |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-FR-047-2: Xác nhận FR-047 thỏa: Mỗi lần chạy FAILED phát sự kiện ScheduledRunFailed có reasonCode qua outbox

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-FR-047-2 |
| Mục tiêu | Xác nhận FR-047 thỏa: Mỗi lần chạy FAILED phát sự kiện ScheduledRunFailed có reasonCode qua outbox |
| Mức, loại, ưu tiên | L2 · functional · P1 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-047 · TD TD-SCHEDULED-TRANSFER-L2-FR-047-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-047-AC-AC-FR-047-2 · nguồn FR-047 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-02-success, STUB-SCHEDULED-TRANSFER-INT-03-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-047-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-047-2 | ac-fr-047-2 | Mỗi lần chạy FAILED phát sự kiện ScheduledRunFailed có reasonCode qua outbox | Tiêu chí chấp nhận AC-FR-047-2 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-047 theo tiêu chí: Mỗi lần chạy FAILED phát sự kiện ScheduledRunFailed có reasonCode qua outbox | MOD-Notify |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Notify |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Mỗi lần chạy FAILED phát sự kiện ScheduledRunFailed có reasonCode qua outbox |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-FR-047-3: Xác nhận FR-047 thỏa: SMS kết quả được gửi cho người gửi; lỗi gửi SMS không đổi trạng thái lần chạy

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-FR-047-3 |
| Mục tiêu | Xác nhận FR-047 thỏa: SMS kết quả được gửi cho người gửi; lỗi gửi SMS không đổi trạng thái lần chạy |
| Mức, loại, ưu tiên | L2 · functional · P1 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-047 · TD TD-SCHEDULED-TRANSFER-L2-FR-047-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-047-AC-AC-FR-047-3 · nguồn FR-047 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-02-success, STUB-SCHEDULED-TRANSFER-INT-03-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-047-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-047-3 | ac-fr-047-3 | SMS kết quả được gửi cho người gửi; lỗi gửi SMS không đổi trạng thái lần chạy | Tiêu chí chấp nhận AC-FR-047-3 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-047 theo tiêu chí: SMS kết quả được gửi cho người gửi; lỗi gửi SMS không đổi trạng thái lần chạy | MOD-Notify |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Notify |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | SMS kết quả được gửi cho người gửi; lỗi gửi SMS không đổi trạng thái lần chạy |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-REV-005: Xác nhận FR-047 thỏa: Sự kiện ScheduledRunCompleted/ScheduledRunFailed có đủ các trường eventId, scheduleId, runId, fromAccountId, toAccountId, amount, correlationId, kèm transferId (thành công) hoặc reasonCode (thất bại), trên cùng topic với sự kiện giao dịch

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-REV-005 |
| Mục tiêu | Xác nhận FR-047 thỏa: Sự kiện ScheduledRunCompleted/ScheduledRunFailed có đủ các trường eventId, scheduleId, runId, fromAccountId, toAccountId, amount, correlationId, kèm transferId (thành công) hoặc reasonCode (thất bại), trên cùng topic với sự kiện giao dịch |
| Mức, loại, ưu tiên | L2 · functional · P1 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-047 · TD TD-SCHEDULED-TRANSFER-L2-FR-047-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-047-AC-AC-REV-005 · nguồn FR-047 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-02-success, STUB-SCHEDULED-TRANSFER-INT-03-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-047-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-REV-005 | ac-rev-005 | Sự kiện ScheduledRunCompleted/ScheduledRunFailed có đủ các trường eventId, scheduleId, runId, fromAccountId, toAccountId, amount, correlationId, kèm transferId (thành công) hoặc reasonCode (thất bại), trên cùng topic với sự kiện giao dịch | Tiêu chí chấp nhận AC-REV-005 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-047 theo tiêu chí: Sự kiện ScheduledRunCompleted/ScheduledRunFailed có đủ các trường eventId, scheduleId, runId, fromAccountId, toAccountId, amount, correlationId, kèm transferId (thành công) hoặc reasonCode (thất bại), trên cùng topic với sự kiện giao dịch | MOD-Notify |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Notify |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Sự kiện ScheduledRunCompleted/ScheduledRunFailed có đủ các trường eventId, scheduleId, runId, fromAccountId, toAccountId, amount, correlationId, kèm transferId (thành công) hoặc reasonCode (thất bại), trên cùng topic với sự kiện giao dịch |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-REV-006: Xác nhận FR-047 thỏa: Nội dung SMS của lần chạy thất bại nêu rõ số dư chỉ được kiểm tra lúc chạy

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-REV-006 |
| Mục tiêu | Xác nhận FR-047 thỏa: Nội dung SMS của lần chạy thất bại nêu rõ số dư chỉ được kiểm tra lúc chạy |
| Mức, loại, ưu tiên | L2 · functional · P1 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-047 · TD TD-SCHEDULED-TRANSFER-L2-FR-047-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-047-AC-AC-REV-006 · nguồn FR-047 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-02-success, STUB-SCHEDULED-TRANSFER-INT-03-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-047-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-REV-006 | ac-rev-006 | Nội dung SMS của lần chạy thất bại nêu rõ số dư chỉ được kiểm tra lúc chạy | Tiêu chí chấp nhận AC-REV-006 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-047 theo tiêu chí: Nội dung SMS của lần chạy thất bại nêu rõ số dư chỉ được kiểm tra lúc chạy | MOD-Notify |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Notify |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Nội dung SMS của lần chạy thất bại nêu rõ số dư chỉ được kiểm tra lúc chạy |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-047-SCN-SCN-ScheduledRun: Chạy trọn luồng chính SCN-ScheduledRun

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-047-SCN-SCN-ScheduledRun |
| Mục tiêu | Chạy trọn luồng chính SCN-ScheduledRun |
| Mức, loại, ưu tiên | L2 · functional · P1 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-047 · TD TD-SCHEDULED-TRANSFER-L2-FR-047-SCN · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-047-SCN-SCN-ScheduledRun · nguồn FR-047 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-02-success, STUB-SCHEDULED-TRANSFER-INT-03-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-047-SCN |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| SCN-ScheduledRun | scn-scheduledrun | 8 | Luồng chính SCN-ScheduledRun |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống và các Module tham gia ở trạng thái ban đầu; stub hệ thống ngoài thành công | MOD-API, MOD-Scheduler, MOD-Transfer |  |
| 2 | when | Quét mỗi 30 giây | — |  |
| 3 | when | chọn tối đa 20 lần chạy đến hạn | — |  |
| 4 | when | chiếm lần chạy (UPDATE có điều kiện) | — |  |
| 5 | when | sinh correlationId | — |  |
| 6 | when | lõi chuyển tiền (idempotency, hạn mức/số dư, Fraud Check, ghi sổ + outbox) | — |  |
| 7 | when | cập nhật lần chạy | — |  |
| 8 | when | thử lại/FAILED | — |  |
| 9 | when | audit + sự kiện | — |  |
| 10 | then | Luồng hoàn tất; kiểm trạng thái cuối ở mọi Module tham gia | MOD-API, MOD-Scheduler, MOD-Transfer |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | hoàn tất |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-FR-048-AC-AC-FR-048-1: Xác nhận FR-048 thỏa: Tạo, hủy, tạm dừng và tiếp tục lịch đều ghi audit với correlationId, thời điểm, tác nhân là mã tài khoản, loại sự kiện và mã lịch

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-048-AC-AC-FR-048-1 |
| Mục tiêu | Xác nhận FR-048 thỏa: Tạo, hủy, tạm dừng và tiếp tục lịch đều ghi audit với correlationId, thời điểm, tác nhân là mã tài khoản, loại sự kiện và mã lịch |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-048 · TD TD-SCHEDULED-TRANSFER-L2-FR-048-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-048-AC-AC-FR-048-1 · nguồn FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-048-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-048-1 | ac-fr-048-1 | Tạo, hủy, tạm dừng và tiếp tục lịch đều ghi audit với correlationId, thời điểm, tác nhân là mã tài khoản, loại sự kiện và mã lịch | Tiêu chí chấp nhận AC-FR-048-1 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-048 theo tiêu chí: Tạo, hủy, tạm dừng và tiếp tục lịch đều ghi audit với correlationId, thời điểm, tác nhân là mã tài khoản, loại sự kiện và mã lịch | MOD-Audit |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Audit |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Tạo, hủy, tạm dừng và tiếp tục lịch đều ghi audit với correlationId, thời điểm, tác nhân là mã tài khoản, loại sự kiện và mã lịch |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-FR-048-AC-AC-FR-048-2: Xác nhận FR-048 thỏa: Mọi kết quả lần chạy ghi audit với tác nhân SCHEDULER và mã lịch

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-048-AC-AC-FR-048-2 |
| Mục tiêu | Xác nhận FR-048 thỏa: Mọi kết quả lần chạy ghi audit với tác nhân SCHEDULER và mã lịch |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-048 · TD TD-SCHEDULED-TRANSFER-L2-FR-048-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-048-AC-AC-FR-048-2 · nguồn FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-048-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-048-2 | ac-fr-048-2 | Mọi kết quả lần chạy ghi audit với tác nhân SCHEDULER và mã lịch | Tiêu chí chấp nhận AC-FR-048-2 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-048 theo tiêu chí: Mọi kết quả lần chạy ghi audit với tác nhân SCHEDULER và mã lịch | MOD-Audit |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Audit |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Mọi kết quả lần chạy ghi audit với tác nhân SCHEDULER và mã lịch |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-FR-048-SCN-SCN-ScheduledRun: Chạy trọn luồng chính SCN-ScheduledRun

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-048-SCN-SCN-ScheduledRun |
| Mục tiêu | Chạy trọn luồng chính SCN-ScheduledRun |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-048 · TD TD-SCHEDULED-TRANSFER-L2-FR-048-SCN · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-048-SCN-SCN-ScheduledRun · nguồn FR-048 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-048-SCN |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| SCN-ScheduledRun | scn-scheduledrun | 8 | Luồng chính SCN-ScheduledRun |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống và các Module tham gia ở trạng thái ban đầu; stub hệ thống ngoài thành công | MOD-API, MOD-Scheduler, MOD-Transfer |  |
| 2 | when | Quét mỗi 30 giây | — |  |
| 3 | when | chọn tối đa 20 lần chạy đến hạn | — |  |
| 4 | when | chiếm lần chạy (UPDATE có điều kiện) | — |  |
| 5 | when | sinh correlationId | — |  |
| 6 | when | lõi chuyển tiền (idempotency, hạn mức/số dư, Fraud Check, ghi sổ + outbox) | — |  |
| 7 | when | cập nhật lần chạy | — |  |
| 8 | when | thử lại/FAILED | — |  |
| 9 | when | audit + sự kiện | — |  |
| 10 | then | Luồng hoàn tất; kiểm trạng thái cuối ở mọi Module tham gia | MOD-API, MOD-Scheduler, MOD-Transfer |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | hoàn tất |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-FR-049-AC-AC-FR-049-1: Xác nhận FR-049 thỏa: Lần chạy quá hạn không quá 24 giờ sau khi hệ thống ngừng được chạy đúng một lần

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-049-AC-AC-FR-049-1 |
| Mục tiêu | Xác nhận FR-049 thỏa: Lần chạy quá hạn không quá 24 giờ sau khi hệ thống ngừng được chạy đúng một lần |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-049 · TD TD-SCHEDULED-TRANSFER-L2-FR-049-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-049-AC-AC-FR-049-1 · nguồn FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-049-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-049-1 | ac-fr-049-1 | Lần chạy quá hạn không quá 24 giờ sau khi hệ thống ngừng được chạy đúng một lần | Tiêu chí chấp nhận AC-FR-049-1 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-049 theo tiêu chí: Lần chạy quá hạn không quá 24 giờ sau khi hệ thống ngừng được chạy đúng một lần | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Lần chạy quá hạn không quá 24 giờ sau khi hệ thống ngừng được chạy đúng một lần |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-049-AC-AC-FR-049-2: Xác nhận FR-049 thỏa: Lần chạy quá hạn quá 24 giờ chuyển MISSED, phát ScheduledRunFailed với reasonCode MISSED và không có giao dịch

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-049-AC-AC-FR-049-2 |
| Mục tiêu | Xác nhận FR-049 thỏa: Lần chạy quá hạn quá 24 giờ chuyển MISSED, phát ScheduledRunFailed với reasonCode MISSED và không có giao dịch |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-049 · TD TD-SCHEDULED-TRANSFER-L2-FR-049-AC · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-049-AC-AC-FR-049-2 · nguồn FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-049-AC |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| AC-FR-049-2 | ac-fr-049-2 | Lần chạy quá hạn quá 24 giờ chuyển MISSED, phát ScheduledRunFailed với reasonCode MISSED và không có giao dịch | Tiêu chí chấp nhận AC-FR-049-2 |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái ban đầu; dữ liệu nền và stub phụ thuộc ngoài đã sẵn sàng | — |  |
| 2 | when | Thực hiện luồng của FR-049 theo tiêu chí: Lần chạy quá hạn quá 24 giờ chuyển MISSED, phát ScheduledRunFailed với reasonCode MISSED và không có giao dịch | MOD-Scheduler |  |
| 3 | then | Quan sát kết quả và trạng thái dữ liệu | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | satisfies | Lần chạy quá hạn quá 24 giờ chuyển MISSED, phát ScheduledRunFailed với reasonCode MISSED và không có giao dịch |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L2-SCHEDULED-TRANSFER-FR-049-SCN-SCN-ScheduledRun: Chạy trọn luồng chính SCN-ScheduledRun

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-FR-049-SCN-SCN-ScheduledRun |
| Mục tiêu | Chạy trọn luồng chính SCN-ScheduledRun |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-FR-049 · TD TD-SCHEDULED-TRANSFER-L2-FR-049-SCN · TCOV TCOV-SCHEDULED-TRANSFER-L2-FR-049-SCN-SCN-ScheduledRun · nguồn FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success |
| Test set | TS-SCHEDULED-TRANSFER-L2-FR-049-SCN |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| SCN-ScheduledRun | scn-scheduledrun | 8 | Luồng chính SCN-ScheduledRun |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống và các Module tham gia ở trạng thái ban đầu; stub hệ thống ngoài thành công | MOD-API, MOD-Scheduler, MOD-Transfer |  |
| 2 | when | Quét mỗi 30 giây | — |  |
| 3 | when | chọn tối đa 20 lần chạy đến hạn | — |  |
| 4 | when | chiếm lần chạy (UPDATE có điều kiện) | — |  |
| 5 | when | sinh correlationId | — |  |
| 6 | when | lõi chuyển tiền (idempotency, hạn mức/số dư, Fraud Check, ghi sổ + outbox) | — |  |
| 7 | when | cập nhật lần chạy | — |  |
| 8 | when | thử lại/FAILED | — |  |
| 9 | when | audit + sự kiện | — |  |
| 10 | then | Luồng hoàn tất; kiểm trạng thái cuối ở mọi Module tham gia | MOD-API, MOD-Scheduler, MOD-Transfer |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | hoàn tất |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-NFR-010-NFR-AT: Đạt ngưỡng NFR-010: Đúng ngưỡng 60 s

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-NFR-010-NFR-AT |
| Mục tiêu | Đạt ngưỡng NFR-010: Đúng ngưỡng 60 s |
| Mức, loại, ưu tiên | L2 · performance · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-NFR-010 · TD TD-SCHEDULED-TRANSFER-L2-NFR-010-NFR · TCOV TCOV-SCHEDULED-TRANSFER-L2-NFR-010-NFR-AT · nguồn NFR-010 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-NFR-010-NFR |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| NFR-010 | at | 60 | Đúng ngưỡng 60 s |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống chịu điều kiện đo theo NFR | — |  |
| 2 | when | Đo NFR-010 ở mức 60 s | — | 60 |
| 3 | then | So sánh kết quả đo với ngưỡng | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | latency | <= | 60 | s |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-NFR-010-NFR-OVER: Vượt ngưỡng NFR-010: Vượt ngưỡng (60+1)

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-NFR-010-NFR-OVER |
| Mục tiêu | Vượt ngưỡng NFR-010: Vượt ngưỡng (60+1) |
| Mức, loại, ưu tiên | L2 · performance · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-NFR-010 · TD TD-SCHEDULED-TRANSFER-L2-NFR-010-NFR · TCOV TCOV-SCHEDULED-TRANSFER-L2-NFR-010-NFR-OVER · nguồn NFR-010 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-NFR-010-NFR |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| NFR-010 | over | 61 | Vượt ngưỡng (60+1) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống chịu điều kiện đo theo NFR | — |  |
| 2 | when | Đo NFR-010 ở mức 61 s | — | 61 |
| 3 | then | So sánh kết quả đo với ngưỡng | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | latency | > | 60 | s |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-NFR-011-NFR-AT: Đạt ngưỡng NFR-011: Đúng ngưỡng 0 giao dịch trùng

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-NFR-011-NFR-AT |
| Mục tiêu | Đạt ngưỡng NFR-011: Đúng ngưỡng 0 giao dịch trùng |
| Mức, loại, ưu tiên | L2 · performance · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-NFR-011 · TD TD-SCHEDULED-TRANSFER-L2-NFR-011-NFR · TCOV TCOV-SCHEDULED-TRANSFER-L2-NFR-011-NFR-AT · nguồn NFR-011 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-NFR-011-NFR |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| NFR-011 | at | 0 | Đúng ngưỡng 0 giao dịch trùng |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống chịu điều kiện đo theo NFR | — |  |
| 2 | when | Đo NFR-011 ở mức 0 giao dịch trùng | — | 0 |
| 3 | then | So sánh kết quả đo với ngưỡng | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | record | <= | 0 | giao dịch trùng |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-NFR-011-NFR-OVER: Vượt ngưỡng NFR-011: Vượt ngưỡng (0+1)

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-NFR-011-NFR-OVER |
| Mục tiêu | Vượt ngưỡng NFR-011: Vượt ngưỡng (0+1) |
| Mức, loại, ưu tiên | L2 · performance · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-NFR-011 · TD TD-SCHEDULED-TRANSFER-L2-NFR-011-NFR · TCOV TCOV-SCHEDULED-TRANSFER-L2-NFR-011-NFR-OVER · nguồn NFR-011 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-NFR-011-NFR |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| NFR-011 | over | 1 | Vượt ngưỡng (0+1) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống chịu điều kiện đo theo NFR | — |  |
| 2 | when | Đo NFR-011 ở mức 1 giao dịch trùng | — | 1 |
| 3 | then | So sánh kết quả đo với ngưỡng | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | record | > | 0 | giao dịch trùng |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-NFR-012-NFR-AT: Đạt ngưỡng NFR-012: Đúng ngưỡng 5 chu kỳ quét

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-NFR-012-NFR-AT |
| Mục tiêu | Đạt ngưỡng NFR-012: Đúng ngưỡng 5 chu kỳ quét |
| Mức, loại, ưu tiên | L2 · performance · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-NFR-012 · TD TD-SCHEDULED-TRANSFER-L2-NFR-012-NFR · TCOV TCOV-SCHEDULED-TRANSFER-L2-NFR-012-NFR-AT · nguồn NFR-012 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-NFR-012-NFR |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| NFR-012 | at | 5 | Đúng ngưỡng 5 chu kỳ quét |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống chịu điều kiện đo theo NFR | — |  |
| 2 | when | Đo NFR-012 ở mức 5 chu kỳ quét | — | 5 |
| 3 | then | So sánh kết quả đo với ngưỡng | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | record | <= | 5 | chu kỳ quét |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-NFR-012-NFR-OVER: Vượt ngưỡng NFR-012: Vượt ngưỡng (5+1)

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-NFR-012-NFR-OVER |
| Mục tiêu | Vượt ngưỡng NFR-012: Vượt ngưỡng (5+1) |
| Mức, loại, ưu tiên | L2 · performance · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-NFR-012 · TD TD-SCHEDULED-TRANSFER-L2-NFR-012-NFR · TCOV TCOV-SCHEDULED-TRANSFER-L2-NFR-012-NFR-OVER · nguồn NFR-012 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-NFR-012-NFR |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| NFR-012 | over | 6 | Vượt ngưỡng (5+1) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống chịu điều kiện đo theo NFR | — |  |
| 2 | when | Đo NFR-012 ở mức 6 chu kỳ quét | — | 6 |
| 3 | then | So sánh kết quả đo với ngưỡng | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | record | > | 5 | chu kỳ quét |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-NFR-013-NFR-AT: Đạt ngưỡng NFR-013: Đúng ngưỡng 500 ms

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-NFR-013-NFR-AT |
| Mục tiêu | Đạt ngưỡng NFR-013: Đúng ngưỡng 500 ms |
| Mức, loại, ưu tiên | L2 · performance · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-NFR-013 · TD TD-SCHEDULED-TRANSFER-L2-NFR-013-NFR · TCOV TCOV-SCHEDULED-TRANSFER-L2-NFR-013-NFR-AT · nguồn NFR-013 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-NFR-013-NFR |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| NFR-013 | at | 500 | Đúng ngưỡng 500 ms |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống chịu điều kiện đo theo NFR | — |  |
| 2 | when | Đo NFR-013 ở mức 500 ms | — | 500 |
| 3 | then | So sánh kết quả đo với ngưỡng | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | latency | <= | 500 | ms |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-NFR-013-NFR-OVER: Vượt ngưỡng NFR-013: Vượt ngưỡng (500+1)

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-NFR-013-NFR-OVER |
| Mục tiêu | Vượt ngưỡng NFR-013: Vượt ngưỡng (500+1) |
| Mức, loại, ưu tiên | L2 · performance · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-NFR-013 · TD TD-SCHEDULED-TRANSFER-L2-NFR-013-NFR · TCOV TCOV-SCHEDULED-TRANSFER-L2-NFR-013-NFR-OVER · nguồn NFR-013 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-NFR-013-NFR |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| NFR-013 | over | 501 | Vượt ngưỡng (500+1) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống chịu điều kiện đo theo NFR | — |  |
| 2 | when | Đo NFR-013 ở mức 501 ms | — | 501 |
| 3 | then | So sánh kết quả đo với ngưỡng | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | latency | > | 500 | ms |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-NFR-014-NFR-AT: Đạt ngưỡng NFR-014: Đúng ngưỡng 100 %

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-NFR-014-NFR-AT |
| Mục tiêu | Đạt ngưỡng NFR-014: Đúng ngưỡng 100 % |
| Mức, loại, ưu tiên | L2 · performance · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-NFR-014 · TD TD-SCHEDULED-TRANSFER-L2-NFR-014-NFR · TCOV TCOV-SCHEDULED-TRANSFER-L2-NFR-014-NFR-AT · nguồn NFR-014 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-NFR-014-NFR |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| NFR-014 | at | 100 | Đúng ngưỡng 100 % |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống chịu điều kiện đo theo NFR | — |  |
| 2 | when | Đo NFR-014 ở mức 100 % | — | 100 |
| 3 | then | So sánh kết quả đo với ngưỡng | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | record | <= | 100 | % |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-NFR-014-NFR-OVER: Vượt ngưỡng NFR-014: Vượt ngưỡng (100+1)

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-NFR-014-NFR-OVER |
| Mục tiêu | Vượt ngưỡng NFR-014: Vượt ngưỡng (100+1) |
| Mức, loại, ưu tiên | L2 · performance · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-NFR-014 · TD TD-SCHEDULED-TRANSFER-L2-NFR-014-NFR · TCOV TCOV-SCHEDULED-TRANSFER-L2-NFR-014-NFR-OVER · nguồn NFR-014 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-NFR-014-NFR |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| NFR-014 | over | 101 | Vượt ngưỡng (100+1) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống chịu điều kiện đo theo NFR | — |  |
| 2 | when | Đo NFR-014 ở mức 101 % | — | 101 |
| 3 | then | So sánh kết quả đo với ngưỡng | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | record | > | 100 | % |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-ACTIVE-PATCH-status-ACTIVE: Từ chối None khi ở trạng thái None

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-ACTIVE-PATCH-status-ACTIVE |
| Mục tiêu | Từ chối None khi ở trạng thái None |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-ACTIVE-PATCH-status-ACTIVE · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái None | — |  |
| 2 | when | Gửi yêu cầu None | — |  |
| 3 | then | Yêu cầu bị từ chối; trạng thái không đổi | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-DELETE: Từ chối None khi ở trạng thái None

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-DELETE |
| Mục tiêu | Từ chối None khi ở trạng thái None |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-CANCELLED-DELETE · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| CANCELLED/DELETE | inv-cancelled-delete | invalid | DELETE từ trạng thái CANCELLED không được phép |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái None | — |  |
| 2 | when | Gửi yêu cầu None | — |  |
| 3 | then | Yêu cầu bị từ chối; trạng thái không đổi | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-PATCH-status-ACTIVE: Từ chối None khi ở trạng thái None

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-PATCH-status-ACTIVE |
| Mục tiêu | Từ chối None khi ở trạng thái None |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-CANCELLED-PATCH-status-ACTIVE · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái None | — |  |
| 2 | when | Gửi yêu cầu None | — |  |
| 3 | then | Yêu cầu bị từ chối; trạng thái không đổi | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-PATCH-status-PAUSED: Từ chối None khi ở trạng thái None

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-PATCH-status-PAUSED |
| Mục tiêu | Từ chối None khi ở trạng thái None |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-CANCELLED-PATCH-status-PAUSED · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái None | — |  |
| 2 | when | Gửi yêu cầu None | — |  |
| 3 | then | Yêu cầu bị từ chối; trạng thái không đổi | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-l-n-ch-y-cu-i-k-t-th-c: Từ chối None khi ở trạng thái None

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-l-n-ch-y-cu-i-k-t-th-c |
| Mục tiêu | Từ chối None khi ở trạng thái None |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-CANCELLED-l-n-ch-y-cu-i-k-t-th-c · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái None | — |  |
| 2 | when | Gửi yêu cầu None | — |  |
| 3 | then | Yêu cầu bị từ chối; trạng thái không đổi | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-scheduler-l-n-ch-y-FAILED-th-3-li-n-ti-p: Từ chối None khi ở trạng thái None

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-scheduler-l-n-ch-y-FAILED-th-3-li-n-ti-p |
| Mục tiêu | Từ chối None khi ở trạng thái None |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-CANCELLED-scheduler-l-n-ch-y-FAILED-th-3-li-n-ti-p · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái None | — |  |
| 2 | when | Gửi yêu cầu None | — |  |
| 3 | then | Yêu cầu bị từ chối; trạng thái không đổi | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-COMPLETED-DELETE: Từ chối None khi ở trạng thái None

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-COMPLETED-DELETE |
| Mục tiêu | Từ chối None khi ở trạng thái None |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-COMPLETED-DELETE · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| COMPLETED/DELETE | inv-completed-delete | invalid | DELETE từ trạng thái COMPLETED không được phép |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái None | — |  |
| 2 | when | Gửi yêu cầu None | — |  |
| 3 | then | Yêu cầu bị từ chối; trạng thái không đổi | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-COMPLETED-PATCH-status-ACTIVE: Từ chối None khi ở trạng thái None

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-COMPLETED-PATCH-status-ACTIVE |
| Mục tiêu | Từ chối None khi ở trạng thái None |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-COMPLETED-PATCH-status-ACTIVE · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái None | — |  |
| 2 | when | Gửi yêu cầu None | — |  |
| 3 | then | Yêu cầu bị từ chối; trạng thái không đổi | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-COMPLETED-PATCH-status-PAUSED: Từ chối None khi ở trạng thái None

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-COMPLETED-PATCH-status-PAUSED |
| Mục tiêu | Từ chối None khi ở trạng thái None |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-COMPLETED-PATCH-status-PAUSED · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái None | — |  |
| 2 | when | Gửi yêu cầu None | — |  |
| 3 | then | Yêu cầu bị từ chối; trạng thái không đổi | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-COMPLETED-l-n-ch-y-cu-i-k-t-th-c: Từ chối None khi ở trạng thái None

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-COMPLETED-l-n-ch-y-cu-i-k-t-th-c |
| Mục tiêu | Từ chối None khi ở trạng thái None |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-INV-COMPLETED-l-n-ch-y-cu-i-k-t-th-c · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái None | — |  |
| 2 | when | Gửi yêu cầu None | — |  |
| 3 | then | Yêu cầu bị từ chối; trạng thái không đổi | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-1: Chuyển ACTIVE sang PAUSED

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-1 |
| Mục tiêu | Chuyển ACTIVE sang PAUSED |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-TRN-Schedule-1 · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| TRN-Schedule-1 | trn-schedule-1 | ACTIVE->PAUSED | ACTIVE -> PAUSED khi PATCH status=PAUSED |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái ACTIVE | — |  |
| 2 | when | ACTIVE -> PAUSED khi PATCH status=PAUSED | — |  |
| 3 | then | Thực thể ở trạng thái PAUSED | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | PAUSED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-2: Chuyển PAUSED sang ACTIVE

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-2 |
| Mục tiêu | Chuyển PAUSED sang ACTIVE |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-TRN-Schedule-2 · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| TRN-Schedule-2 | trn-schedule-2 | PAUSED->ACTIVE | PAUSED -> ACTIVE khi PATCH status=ACTIVE |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái PAUSED | — |  |
| 2 | when | PAUSED -> ACTIVE khi PATCH status=ACTIVE | — |  |
| 3 | then | Thực thể ở trạng thái ACTIVE | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | ACTIVE |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-3: Chuyển ACTIVE sang PAUSED

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-3 |
| Mục tiêu | Chuyển ACTIVE sang PAUSED |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-TRN-Schedule-3 · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| TRN-Schedule-3 | trn-schedule-3 | ACTIVE->PAUSED | ACTIVE -> PAUSED khi scheduler: lần chạy FAILED thứ 3 liên tiếp |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái ACTIVE | — |  |
| 2 | when | ACTIVE -> PAUSED khi scheduler: lần chạy FAILED thứ 3 liên tiếp | — |  |
| 3 | then | Thực thể ở trạng thái PAUSED | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | PAUSED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-4: Chuyển ACTIVE sang COMPLETED

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-4 |
| Mục tiêu | Chuyển ACTIVE sang COMPLETED |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-TRN-Schedule-4 · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| TRN-Schedule-4 | trn-schedule-4 | ACTIVE->COMPLETED | ACTIVE -> COMPLETED khi lần chạy cuối kết thúc |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái ACTIVE | — |  |
| 2 | when | ACTIVE -> COMPLETED khi lần chạy cuối kết thúc | — |  |
| 3 | then | Thực thể ở trạng thái COMPLETED | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | COMPLETED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-5: Chuyển ACTIVE sang CANCELLED

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-5 |
| Mục tiêu | Chuyển ACTIVE sang CANCELLED |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-TRN-Schedule-5 · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| TRN-Schedule-5 | trn-schedule-5 | ACTIVE->CANCELLED | ACTIVE -> CANCELLED khi DELETE |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái ACTIVE | — |  |
| 2 | when | ACTIVE -> CANCELLED khi DELETE | — |  |
| 3 | then | Thực thể ở trạng thái CANCELLED | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | CANCELLED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-6: Chuyển PAUSED sang CANCELLED

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-6 |
| Mục tiêu | Chuyển PAUSED sang CANCELLED |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-TRN-Schedule-6 · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| TRN-Schedule-6 | trn-schedule-6 | PAUSED->CANCELLED | PAUSED -> CANCELLED khi DELETE |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái PAUSED | — |  |
| 2 | when | PAUSED -> CANCELLED khi DELETE | — |  |
| 3 | then | Thực thể ở trạng thái CANCELLED | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | CANCELLED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-7: Chuyển PAUSED sang COMPLETED

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-7 |
| Mục tiêu | Chuyển PAUSED sang COMPLETED |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-Schedule · TD TD-SCHEDULED-TRANSFER-L2-SM-Schedule-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-Schedule-STM-TRN-Schedule-7 · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-Schedule-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| TRN-Schedule-7 | trn-schedule-7 | PAUSED->COMPLETED | PAUSED -> COMPLETED khi PATCH status=ACTIVE |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái PAUSED | — |  |
| 2 | when | PAUSED -> COMPLETED khi PATCH status=ACTIVE | — |  |
| 3 | then | Thực thể ở trạng thái COMPLETED | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | COMPLETED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED--u-chu-k-qu-t: Từ chối None khi ở trạng thái None

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED--u-chu-k-qu-t |
| Mục tiêu | Từ chối None khi ở trạng thái None |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED--u-chu-k-qu-t · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái None | — |  |
| 2 | when | Gửi yêu cầu None | — |  |
| 3 | then | Yêu cầu bị từ chối; trạng thái không đổi | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-DELETE-l-ch: Từ chối None khi ở trạng thái None

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-DELETE-l-ch |
| Mục tiêu | Từ chối None khi ở trạng thái None |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED-DELETE-l-ch · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái None | — |  |
| 2 | when | Gửi yêu cầu None | — |  |
| 3 | then | Yêu cầu bị từ chối; trạng thái không đổi | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-l-i-chuy-n-ti-n-tr-giao-d-ch-COMPLETED-ho-c-ph-t-l-i: Từ chối None khi ở trạng thái None

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-l-i-chuy-n-ti-n-tr-giao-d-ch-COMPLETED-ho-c-ph-t-l-i |
| Mục tiêu | Từ chối None khi ở trạng thái None |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED-l-i-chuy-n-ti-n-tr-giao-d-ch-COMPLETED-ho-c-ph-t-l-i · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái None | — |  |
| 2 | when | Gửi yêu cầu None | — |  |
| 3 | then | Yêu cầu bị từ chối; trạng thái không đổi | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-l-i-tr-l-i: Từ chối None khi ở trạng thái None

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-l-i-tr-l-i |
| Mục tiêu | Từ chối None khi ở trạng thái None |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED-l-i-tr-l-i · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái None | — |  |
| 2 | when | Gửi yêu cầu None | — |  |
| 3 | then | Yêu cầu bị từ chối; trạng thái không đổi | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-l-i-tr-l-i-t-m-th-i-ho-c-thi-u-s-d: Từ chối None khi ở trạng thái None

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-l-i-tr-l-i-t-m-th-i-ho-c-thi-u-s-d |
| Mục tiêu | Từ chối None khi ở trạng thái None |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED-l-i-tr-l-i-t-m-th-i-ho-c-thi-u-s-d · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái None | — |  |
| 2 | when | Gửi yêu cầu None | — |  |
| 3 | then | Yêu cầu bị từ chối; trạng thái không đổi | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-scheduler-chi-m-l-n-ch-y: Từ chối None khi ở trạng thái None

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-scheduler-chi-m-l-n-ch-y |
| Mục tiêu | Từ chối None khi ở trạng thái None |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED-scheduler-chi-m-l-n-ch-y · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái None | — |  |
| 2 | when | Gửi yêu cầu None | — |  |
| 3 | then | Yêu cầu bị từ chối; trạng thái không đổi | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-scheduler-ph-c-h-i-l-n-ch-y-k-t: Từ chối None khi ở trạng thái None

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-scheduler-ph-c-h-i-l-n-ch-y-k-t |
| Mục tiêu | Từ chối None khi ở trạng thái None |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED-scheduler-ph-c-h-i-l-n-ch-y-k-t · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái None | — |  |
| 2 | when | Gửi yêu cầu None | — |  |
| 3 | then | Yêu cầu bị từ chối; trạng thái không đổi | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-ti-p-t-c-l-ch-PAUSED: Từ chối None khi ở trạng thái None

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-ti-p-t-c-l-ch-PAUSED |
| Mục tiêu | Từ chối None khi ở trạng thái None |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-CANCELLED-ti-p-t-c-l-ch-PAUSED · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái None | — |  |
| 2 | when | Gửi yêu cầu None | — |  |
| 3 | then | Yêu cầu bị từ chối; trạng thái không đổi | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-FAILED-DELETE-l-ch: Từ chối None khi ở trạng thái None

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-FAILED-DELETE-l-ch |
| Mục tiêu | Từ chối None khi ở trạng thái None |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-FAILED-DELETE-l-ch · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái None | — |  |
| 2 | when | Gửi yêu cầu None | — |  |
| 3 | then | Yêu cầu bị từ chối; trạng thái không đổi | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-FAILED-l-i-chuy-n-ti-n-tr-giao-d-ch-COMPLETED-ho-c-ph-t-l-i: Từ chối None khi ở trạng thái None

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-FAILED-l-i-chuy-n-ti-n-tr-giao-d-ch-COMPLETED-ho-c-ph-t-l-i |
| Mục tiêu | Từ chối None khi ở trạng thái None |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-INV-FAILED-l-i-chuy-n-ti-n-tr-giao-d-ch-COMPLETED-ho-c-ph-t-l-i · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái None | — |  |
| 2 | when | Gửi yêu cầu None | — |  |
| 3 | then | Yêu cầu bị từ chối; trạng thái không đổi | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | errorCode | equals | rejected |  |
| 2 | state | equals |  |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-1: Chuyển PENDING sang RUNNING

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-1 |
| Mục tiêu | Chuyển PENDING sang RUNNING |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-1 · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| TRN-ScheduledRun-1 | trn-scheduledrun-1 | PENDING->RUNNING | PENDING -> RUNNING khi scheduler chiếm lần chạy |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái PENDING | — |  |
| 2 | when | PENDING -> RUNNING khi scheduler chiếm lần chạy | — |  |
| 3 | then | Thực thể ở trạng thái RUNNING | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | RUNNING |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-2: Chuyển RUNNING sang SUCCEEDED

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-2 |
| Mục tiêu | Chuyển RUNNING sang SUCCEEDED |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-2 · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| TRN-ScheduledRun-2 | trn-scheduledrun-2 | RUNNING->SUCCEEDED | RUNNING -> SUCCEEDED khi lõi chuyển tiền trả giao dịch COMPLETED (hoặc phát lại) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái RUNNING | — |  |
| 2 | when | RUNNING -> SUCCEEDED khi lõi chuyển tiền trả giao dịch COMPLETED (hoặc phát lại) | — |  |
| 3 | then | Thực thể ở trạng thái SUCCEEDED | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | SUCCEEDED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-3: Chuyển RUNNING sang PENDING

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-3 |
| Mục tiêu | Chuyển RUNNING sang PENDING |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-3 · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| TRN-ScheduledRun-3 | trn-scheduledrun-3 | RUNNING->PENDING | RUNNING -> PENDING khi lõi trả lỗi tạm thời hoặc thiếu số dư |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái RUNNING | — |  |
| 2 | when | RUNNING -> PENDING khi lõi trả lỗi tạm thời hoặc thiếu số dư | — |  |
| 3 | then | Thực thể ở trạng thái PENDING | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | PENDING |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-4: Chuyển RUNNING sang FAILED

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-4 |
| Mục tiêu | Chuyển RUNNING sang FAILED |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-4 · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| TRN-ScheduledRun-4 | trn-scheduledrun-4 | RUNNING->FAILED | RUNNING -> FAILED khi lõi trả lỗi |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái RUNNING | — |  |
| 2 | when | RUNNING -> FAILED khi lõi trả lỗi | — |  |
| 3 | then | Thực thể ở trạng thái FAILED | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | FAILED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-5: Chuyển PENDING sang SKIPPED

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-5 |
| Mục tiêu | Chuyển PENDING sang SKIPPED |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-5 · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| TRN-ScheduledRun-5 | trn-scheduledrun-5 | PENDING->SKIPPED | PENDING -> SKIPPED khi tiếp tục lịch PAUSED |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái PENDING | — |  |
| 2 | when | PENDING -> SKIPPED khi tiếp tục lịch PAUSED | — |  |
| 3 | then | Thực thể ở trạng thái SKIPPED | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | SKIPPED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-6: Chuyển PENDING sang MISSED

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-6 |
| Mục tiêu | Chuyển PENDING sang MISSED |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-6 · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| TRN-ScheduledRun-6 | trn-scheduledrun-6 | PENDING->MISSED | PENDING -> MISSED khi đầu chu kỳ quét |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái PENDING | — |  |
| 2 | when | PENDING -> MISSED khi đầu chu kỳ quét | — |  |
| 3 | then | Thực thể ở trạng thái MISSED | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | MISSED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-7: Chuyển PENDING sang CANCELLED

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-7 |
| Mục tiêu | Chuyển PENDING sang CANCELLED |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-7 · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| TRN-ScheduledRun-7 | trn-scheduledrun-7 | PENDING->CANCELLED | PENDING -> CANCELLED khi DELETE lịch |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái PENDING | — |  |
| 2 | when | PENDING -> CANCELLED khi DELETE lịch | — |  |
| 3 | then | Thực thể ở trạng thái CANCELLED | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | CANCELLED |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-8: Chuyển RUNNING sang PENDING

| Trường | Giá trị |
| --- | --- |
| id | TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-8 |
| Mục tiêu | Chuyển RUNNING sang PENDING |
| Mức, loại, ưu tiên | L2 · functional · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L2-SM-ScheduledRun · TD TD-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM · TCOV TCOV-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM-TRN-ScheduledRun-8 · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | — |
| Test set | TS-SCHEDULED-TRANSFER-L2-SM-ScheduledRun-STM |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| TRN-ScheduledRun-8 | trn-scheduledrun-8 | RUNNING->PENDING | RUNNING -> PENDING khi scheduler phục hồi lần chạy kẹt |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Thực thể ở trạng thái RUNNING | — |  |
| 2 | when | RUNNING -> PENDING khi scheduler phục hồi lần chạy kẹt | — |  |
| 3 | then | Thực thể ở trạng thái PENDING | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | PENDING |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

### 3.3 L3: Cô lập service

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-BRK-INT-01-RECLOSE: INT-01: sau 30 giây breaker thử lại

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-BRK-INT-01-RECLOSE |
| Mục tiêu | INT-01: sau 30 giây breaker thử lại |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-BRK-INT-01 · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-BRK-INT-01-RECLOSE · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-errorTransient-503, STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-BRK-INT-01 |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| openDurationSec | reclose | 30 | sau 30 giây breaker thử lại |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | stub errorTransient mã 503 | INT-01 |  |
| 2 | when | MOD-Scheduler gọi INT-01 liên tiếp | INT-01 |  |
| 3 | then | Quan sát trạng thái circuit breaker | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | half-open |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-BRK-INT-01-TRIP: INT-01: tỷ lệ lỗi >= 100% sau 5 lượt thì breaker mở

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-BRK-INT-01-TRIP |
| Mục tiêu | INT-01: tỷ lệ lỗi >= 100% sau 5 lượt thì breaker mở |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-BRK-INT-01 · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-BRK-INT-01-TRIP · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-errorTransient-503, STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-BRK-INT-01 |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| failureRatePercent | trip | 100 | tỷ lệ lỗi >= 100% sau 5 lượt thì breaker mở |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | stub errorTransient mã 503 | INT-01 |  |
| 2 | when | MOD-Scheduler gọi INT-01 liên tiếp | INT-01 |  |
| 3 | then | Quan sát trạng thái circuit breaker | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | open |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Account-FAIL: MOD-Scheduler: MOD-Account lỗi

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Account-FAIL |
| Mục tiêu | MOD-Scheduler: MOD-Account lỗi |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Account · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Account-FAIL · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-errorTransient-503, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Account |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| dependency | fail | MOD-Account | MOD-Account lỗi, MOD-Scheduler xử lý lỗi có kiểm soát |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | stub errorTransient mã 503 | MOD-Account |  |
| 2 | when | MOD-Scheduler gọi MOD-Account | MOD-Account |  |
| 3 | then | Kiểm kết quả và trạng thái của MOD-Scheduler | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | handled-error |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Account-OK: MOD-Scheduler: MOD-Account hoạt động

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Account-OK |
| Mục tiêu | MOD-Scheduler: MOD-Account hoạt động |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Account · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Account-OK · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-errorTransient-503, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Account |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| dependency | ok | MOD-Account | MOD-Scheduler gọi MOD-Account thành công |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | stub success | MOD-Account |  |
| 2 | when | MOD-Scheduler gọi MOD-Account | MOD-Account |  |
| 3 | then | Kiểm kết quả và trạng thái của MOD-Scheduler | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | success |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Audit-FAIL: MOD-Scheduler: MOD-Audit lỗi

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Audit-FAIL |
| Mục tiêu | MOD-Scheduler: MOD-Audit lỗi |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Audit · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Audit-FAIL · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-errorTransient-503, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Audit |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| dependency | fail | MOD-Audit | MOD-Audit lỗi, MOD-Scheduler xử lý lỗi có kiểm soát |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | stub errorTransient mã 503 | MOD-Audit |  |
| 2 | when | MOD-Scheduler gọi MOD-Audit | MOD-Audit |  |
| 3 | then | Kiểm kết quả và trạng thái của MOD-Scheduler | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | handled-error |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Audit-OK: MOD-Scheduler: MOD-Audit hoạt động

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Audit-OK |
| Mục tiêu | MOD-Scheduler: MOD-Audit hoạt động |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Audit · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Audit-OK · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-errorTransient-503, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Audit |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| dependency | ok | MOD-Audit | MOD-Scheduler gọi MOD-Audit thành công |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | stub success | MOD-Audit |  |
| 2 | when | MOD-Scheduler gọi MOD-Audit | MOD-Audit |  |
| 3 | then | Kiểm kết quả và trạng thái của MOD-Scheduler | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | success |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Risk-FAIL: MOD-Scheduler: MOD-Risk lỗi

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Risk-FAIL |
| Mục tiêu | MOD-Scheduler: MOD-Risk lỗi |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Risk · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Risk-FAIL · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-errorTransient-503, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Risk |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| dependency | fail | MOD-Risk | MOD-Risk lỗi, MOD-Scheduler xử lý lỗi có kiểm soát |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | stub errorTransient mã 503 | MOD-Risk |  |
| 2 | when | MOD-Scheduler gọi MOD-Risk | MOD-Risk |  |
| 3 | then | Kiểm kết quả và trạng thái của MOD-Scheduler | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | handled-error |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Risk-OK: MOD-Scheduler: MOD-Risk hoạt động

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Risk-OK |
| Mục tiêu | MOD-Scheduler: MOD-Risk hoạt động |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Risk · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Risk-OK · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-errorTransient-503, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Risk |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| dependency | ok | MOD-Risk | MOD-Scheduler gọi MOD-Risk thành công |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | stub success | MOD-Risk |  |
| 2 | when | MOD-Scheduler gọi MOD-Risk | MOD-Risk |  |
| 3 | then | Kiểm kết quả và trạng thái của MOD-Scheduler | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | success |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Transfer-FAIL: MOD-Scheduler: MOD-Transfer lỗi

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Transfer-FAIL |
| Mục tiêu | MOD-Scheduler: MOD-Transfer lỗi |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Transfer · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Transfer-FAIL · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-errorTransient-503, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Transfer |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| dependency | fail | MOD-Transfer | MOD-Transfer lỗi, MOD-Scheduler xử lý lỗi có kiểm soát |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | stub errorTransient mã 503 | MOD-Transfer |  |
| 2 | when | MOD-Scheduler gọi MOD-Transfer | MOD-Transfer |  |
| 3 | then | Kiểm kết quả và trạng thái của MOD-Scheduler | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | handled-error |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Transfer-OK: MOD-Scheduler: MOD-Transfer hoạt động

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Transfer-OK |
| Mục tiêu | MOD-Scheduler: MOD-Transfer hoạt động |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Transfer · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Transfer-OK · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-errorTransient-503, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-DEP-MOD-Transfer |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| dependency | ok | MOD-Transfer | MOD-Scheduler gọi MOD-Transfer thành công |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | stub success | MOD-Transfer |  |
| 2 | when | MOD-Scheduler gọi MOD-Transfer | MOD-Transfer |  |
| 3 | then | Kiểm kết quả và trạng thái của MOD-Scheduler | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | success |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-ERR-INT-01-ERR-INT01-4xx: INT-01: HTTP 4xx (400, 401, 422) được xử lý như permanent

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-ERR-INT-01-ERR-INT01-4xx |
| Mục tiêu | INT-01: HTTP 4xx (400, 401, 422) được xử lý như permanent |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01 · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01-ERR-INT01-4xx · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-errorPermanent-200, STUB-SCHEDULED-TRANSFER-INT-01-errorPermanent-400, STUB-SCHEDULED-TRANSFER-INT-01-errorTransient, STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-INT-01-timeout, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01 |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| trigger | err-int01-4xx | permanent | ERR-INT01-4xx: HTTP 4xx (400, 401, 422) là permanent |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | stub errorPermanent mã 400 | INT-01 |  |
| 2 | when | MOD-Scheduler gọi INT-01 khi stub errorPermanent mã 400 | INT-01 | HTTP 4xx (400, 401, 422) |
| 3 | then | Phân loại là permanent và thực hiện: Không retry; coi là Fraud không khả dụng, áp chính sách FB-INT01; ghi cảnh báo vi phạm contract | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | permanent |  |
| 2 | record | equals | Không retry; coi là Fraud không khả dụng, áp chính sách FB-INT01; ghi cảnh báo vi phạm contract |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-ERR-INT-01-ERR-INT01-5xx: INT-01: HTTP 5xx được xử lý như transient

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-ERR-INT-01-ERR-INT01-5xx |
| Mục tiêu | INT-01: HTTP 5xx được xử lý như transient |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01 · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01-ERR-INT01-5xx · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-errorPermanent-200, STUB-SCHEDULED-TRANSFER-INT-01-errorPermanent-400, STUB-SCHEDULED-TRANSFER-INT-01-errorTransient, STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-INT-01-timeout, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01 |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| trigger | err-int01-5xx | transient | ERR-INT01-5xx: HTTP 5xx là transient |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | stub errorTransient | INT-01 |  |
| 2 | when | MOD-Scheduler gọi INT-01 khi stub errorTransient | INT-01 | HTTP 5xx |
| 3 | then | Phân loại là transient và thực hiện: Retry 1 lần; vẫn lỗi thì coi là không khả dụng, áp FB-INT01; tính vào circuit breaker | MOD-Scheduler |  |

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

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-ERR-INT-01-ERR-INT01-schema: INT-01: HTTP 200 nhưng response sai schema được xử lý như permanent

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-ERR-INT-01-ERR-INT01-schema |
| Mục tiêu | INT-01: HTTP 200 nhưng response sai schema được xử lý như permanent |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01 · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01-ERR-INT01-schema · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-errorPermanent-200, STUB-SCHEDULED-TRANSFER-INT-01-errorPermanent-400, STUB-SCHEDULED-TRANSFER-INT-01-errorTransient, STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-INT-01-timeout, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01 |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| trigger | err-int01-schema | permanent | ERR-INT01-schema: HTTP 200 nhưng response sai schema là permanent |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | stub errorPermanent mã 200 | INT-01 |  |
| 2 | when | MOD-Scheduler gọi INT-01 khi stub errorPermanent mã 200 | INT-01 | HTTP 200 nhưng response sai schema |
| 3 | then | Phân loại là permanent và thực hiện: Không retry; coi là không khả dụng; ghi cảnh báo vi phạm contract | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | permanent |  |
| 2 | record | equals | Không retry; coi là không khả dụng; ghi cảnh báo vi phạm contract |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-ERR-INT-01-ERR-INT01-timeout: INT-01: Timeout (connect/read) hoặc lỗi mạng được xử lý như unknownOutcome

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-ERR-INT-01-ERR-INT01-timeout |
| Mục tiêu | INT-01: Timeout (connect/read) hoặc lỗi mạng được xử lý như unknownOutcome |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01 · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01-ERR-INT01-timeout · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-errorPermanent-200, STUB-SCHEDULED-TRANSFER-INT-01-errorPermanent-400, STUB-SCHEDULED-TRANSFER-INT-01-errorTransient, STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-INT-01-timeout, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-ERR-INT-01 |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| trigger | err-int01-timeout | unknownOutcome | ERR-INT01-timeout: Timeout (connect/read) hoặc lỗi mạng là unknownOutcome |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | stub timeout, trễ 2000 ms | INT-01 |  |
| 2 | when | MOD-Scheduler gọi INT-01 khi stub timeout, trễ 2000 ms | INT-01 | Timeout (connect/read) hoặc lỗi mạng |
| 3 | then | Phân loại là unknownOutcome và thực hiện: Retry 1 lần với cùng transactionId (idempotent); vẫn lỗi thì áp FB-INT01 | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | unknownOutcome |  |
| 2 | record | equals | Retry 1 lần với cùng transactionId (idempotent); vẫn lỗi thì áp FB-INT01 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-IDM-INT-01-REPLAY: INT-01: gửi lại cùng yêu cầu

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-IDM-INT-01-REPLAY |
| Mục tiêu | INT-01: gửi lại cùng yêu cầu |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-IDM-INT-01 · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-IDM-INT-01-REPLAY · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-IDM-INT-01 |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| replay | replay | 2 | gửi lại cùng yêu cầu tới INT-01 (idempotent=true) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Một yêu cầu đã xử lý thành công | INT-01 |  |
| 2 | when | Gửi lại đúng yêu cầu đó tới INT-01 | INT-01 |  |
| 3 | then | Kiểm kết quả và số bản ghi tạo ra | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | record | equals | 1 |  |
| 2 | state | equals | không tạo trùng |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-LIM-INT-01-maxPayloadKB-AT: INT-01: đúng giới hạn maxPayloadKB = 16 KB

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-LIM-INT-01-maxPayloadKB-AT |
| Mục tiêu | INT-01: đúng giới hạn maxPayloadKB = 16 KB |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-maxPayloadKB · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-maxPayloadKB-AT · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-maxPayloadKB |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| maxPayloadKB | at | 16 | đúng giới hạn maxPayloadKB = 16 KB |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Client gửi theo giới hạn cấu hình | INT-01 |  |
| 2 | when | Gửi 16 KB tới INT-01 | INT-01 | 16 |
| 3 | then | Kiểm phản hồi | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-LIM-INT-01-maxPayloadKB-OVER: INT-01: vượt giới hạn maxPayloadKB = 16 KB

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-LIM-INT-01-maxPayloadKB-OVER |
| Mục tiêu | INT-01: vượt giới hạn maxPayloadKB = 16 KB |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-maxPayloadKB · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-maxPayloadKB-OVER · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-maxPayloadKB |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| maxPayloadKB | over | 17 | vượt giới hạn maxPayloadKB = 16 KB |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Client gửi theo giới hạn cấu hình | INT-01 |  |
| 2 | when | Gửi 17 KB tới INT-01 | INT-01 | 17 |
| 3 | then | Kiểm phản hồi | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | in | 429\|413 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-LIM-INT-01-requestsPerSecond-AT: INT-01: đúng giới hạn requestsPerSecond = 50 req/s

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-LIM-INT-01-requestsPerSecond-AT |
| Mục tiêu | INT-01: đúng giới hạn requestsPerSecond = 50 req/s |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-requestsPerSecond · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-requestsPerSecond-AT · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-requestsPerSecond |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| requestsPerSecond | at | 50 | đúng giới hạn requestsPerSecond = 50 req/s |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Client gửi theo giới hạn cấu hình | INT-01 |  |
| 2 | when | Gửi 50 req/s tới INT-01 | INT-01 | 50 |
| 3 | then | Kiểm phản hồi | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | equals | 200 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-LIM-INT-01-requestsPerSecond-OVER: INT-01: vượt giới hạn requestsPerSecond = 50 req/s

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-LIM-INT-01-requestsPerSecond-OVER |
| Mục tiêu | INT-01: vượt giới hạn requestsPerSecond = 50 req/s |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-requestsPerSecond · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-requestsPerSecond-OVER · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-LIM-INT-01-requestsPerSecond |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| requestsPerSecond | over | 51 | vượt giới hạn requestsPerSecond = 50 req/s |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | Client gửi theo giới hạn cấu hình | INT-01 |  |
| 2 | when | Gửi 51 req/s tới INT-01 | INT-01 | 51 |
| 3 | then | Kiểm phản hồi | — |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | status | in | 429\|413 |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-RTY-INT-01-EXHAUST: INT-01: hết lượt thử sau 2 lỗi tạm thời

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-RTY-INT-01-EXHAUST |
| Mục tiêu | INT-01: hết lượt thử sau 2 lỗi tạm thời |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-RTY-INT-01 · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-RTY-INT-01-EXHAUST · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-errorTransient-503, STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-RTY-INT-01 |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| maxAttempts | exhaust | 2 | lỗi tạm thời đủ 2 lần |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | stub errorTransient mã 503; lỗi cả 2 lần | INT-01 |  |
| 2 | when | MOD-Scheduler gọi INT-01 | INT-01 |  |
| 3 | then | Đếm số lượt gọi và kiểm kết quả | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | event | equals | 2 | lượt gọi |
| 2 | state | equals | fallback |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-RTY-INT-01-RECOVER: INT-01: phục hồi sau 1 lỗi tạm thời

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-RTY-INT-01-RECOVER |
| Mục tiêu | INT-01: phục hồi sau 1 lỗi tạm thời |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-RTY-INT-01 · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-RTY-INT-01-RECOVER · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-errorTransient-503, STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-RTY-INT-01 |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| maxAttempts | recover | 2 | lỗi tạm thời 1 lần rồi thành công |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | stub errorTransient mã 503; lỗi 1 lần rồi thành công | INT-01 |  |
| 2 | when | MOD-Scheduler gọi INT-01 | INT-01 |  |
| 3 | then | Đếm số lượt gọi và kiểm kết quả | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | event | equals | 2 | lượt gọi |
| 2 | state | equals | success |  |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-TMO-INT-01-OVER: INT-01 không trả lời: dừng chờ sau totalMs

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-TMO-INT-01-OVER |
| Mục tiêu | INT-01 không trả lời: dừng chờ sau totalMs |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-TMO-INT-01 · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-TMO-INT-01-OVER · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-slow, STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-INT-01-timeout, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-TMO-INT-01 |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| delayMs | over | 2000 | INT-01 không trả lời trước totalMs (1000 ms) |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | stub timeout, trễ 2000 ms | INT-01 |  |
| 2 | when | MOD-Scheduler gọi INT-01 | INT-01 |  |
| 3 | then | Hết hạn chờ và xử lý như unknownOutcome | MOD-Scheduler |  |

Kết quả mong đợi:

| # | Loại kiểm | Toán tử | Giá trị mong đợi | Đơn vị |
| --- | --- | --- | --- | --- |
| 1 | state | equals | unknownOutcome |  |
| 2 | latency | <= | 1000 | ms |

| Trường | Giá trị |
| --- | --- |
| Hậu điều kiện và dọn dẹp | dọn dữ liệu đã tạo; gỡ cấu hình stub |
| Trạng thái duyệt | proposed · chờ xác nhận |

#### TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-TMO-INT-01-UNDER: INT-01 trả lời chậm vẫn thành công

| Trường | Giá trị |
| --- | --- |
| id | TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-TMO-INT-01-UNDER |
| Mục tiêu | INT-01 trả lời chậm vẫn thành công |
| Mức, loại, ưu tiên | L3 · resilience · P0 |
| Truy vết | TR TR-SCHEDULED-TRANSFER-L3-MOD-Scheduler · TD TD-SCHEDULED-TRANSFER-L3-MOD-Scheduler-TMO-INT-01 · TCOV TCOV-SCHEDULED-TRANSFER-L3-MOD-Scheduler-TMO-INT-01-UNDER · nguồn FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-049 |
| Loại thay đổi | new |
| Tự động hóa | có: mô tả đủ cấu trúc để sinh script và dữ liệu mock ở bước sau |
| Tiền điều kiện | môi trường đã sẵn sàng; dữ liệu nền đã nạp; stub được cấu hình theo STUB của TestCase |
| Stub cần | STUB-SCHEDULED-TRANSFER-INT-01-slow, STUB-SCHEDULED-TRANSFER-INT-01-success, STUB-SCHEDULED-TRANSFER-INT-01-timeout, STUB-SCHEDULED-TRANSFER-MOD-Account-success, STUB-SCHEDULED-TRANSFER-MOD-Audit-success, STUB-SCHEDULED-TRANSFER-MOD-Risk-success, STUB-SCHEDULED-TRANSFER-MOD-Transfer-success |
| Test set | TS-SCHEDULED-TRANSFER-L3-MOD-Scheduler-TMO-INT-01 |
| Nguồn nội dung | rule (độ tin cậy 0.6) |

Dữ liệu cần:

| Trường | Lớp giá trị | Giá trị | Ràng buộc |
| --- | --- | --- | --- |
| delayMs | under | 400 | INT-01 trả lời chậm nhưng trong readMs |

Các bước:

| # | Loại | Hành động hoặc điều kiện | Đối tượng | Đầu vào |
| --- | --- | --- | --- | --- |
| 1 | given | stub slow, trễ 400 ms | INT-01 |  |
| 2 | when | MOD-Scheduler gọi INT-01 | INT-01 |  |
| 3 | then | Nhận được kết quả thành công | MOD-Scheduler |  |

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
| TC-L1-SCHEDULED-TRANSFER-SCN-ScheduledRun-JRN-BUDGET | L1 | P0 | new | SCN-ScheduledRun hoàn tất trong ngân sách thời gian |
| TC-L1-SCHEDULED-TRANSFER-SCN-ScheduledRun-JRN-MAIN | L1 | P0 | new | Chạy trọn luồng chính SCN-ScheduledRun |
| TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-200 | L2 | P0 | new | DELETE /v1/scheduled-transfers/{scheduleId} trả 200 |
| TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-401 | L2 | P0 | new | DELETE /v1/scheduled-transfers/{scheduleId} trả 401 UNAUTHORIZED |
| TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-403 | L2 | P0 | new | DELETE /v1/scheduled-transfers/{scheduleId} trả 403 FORBIDDEN |
| TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-404 | L2 | P0 | new | DELETE /v1/scheduled-transfers/{scheduleId} trả 404 SCHEDULE_NOT_FOUND |
| TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-API-RSP-Cancel-409 | L2 | P0 | new | DELETE /v1/scheduled-transfers/{scheduleId} trả 409 SCHEDULE_NOT_CANCELLABLE |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-amount-ABOVE-MAX | L2 | P0 | new | POST /v1/scheduled-transfers: amount (body) lớn hơn lớn nhất (50000001) |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-amount-BELOW-MIN | L2 | P0 | new | POST /v1/scheduled-transfers: amount (body) nhỏ hơn nhỏ nhất (0) |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-amount-MAX | L2 | P0 | new | POST /v1/scheduled-transfers: amount (body) đúng giá trị lớn nhất 50000000 |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-amount-MIN | L2 | P0 | new | POST /v1/scheduled-transfers: amount (body) đúng giá trị nhỏ nhất 1 |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-maxRuns-ABOVE-MAX | L2 | P0 | new | POST /v1/scheduled-transfers: maxRuns (body) lớn hơn lớn nhất (61) |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-maxRuns-BELOW-MIN | L2 | P0 | new | POST /v1/scheduled-transfers: maxRuns (body) nhỏ hơn nhỏ nhất (0) |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-maxRuns-MAX | L2 | P0 | new | POST /v1/scheduled-transfers: maxRuns (body) đúng giá trị lớn nhất 60 |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-maxRuns-MIN | L2 | P0 | new | POST /v1/scheduled-transfers: maxRuns (body) đúng giá trị nhỏ nhất 1 |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-DAILY | L2 | P0 | new | POST /v1/scheduled-transfers: recurrence (body) = DAILY (giá trị hợp lệ) |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-INVALID | L2 | P0 | new | POST /v1/scheduled-transfers: recurrence (body) ngoài danh sách ONCE, DAILY, WEEKLY, MONTHLY |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-MONTHLY | L2 | P0 | new | POST /v1/scheduled-transfers: recurrence (body) = MONTHLY (giá trị hợp lệ) |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-ONCE | L2 | P0 | new | POST /v1/scheduled-transfers: recurrence (body) = ONCE (giá trị hợp lệ) |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-FLD-Create-recurrence-ENUM-WEEKLY | L2 | P0 | new | POST /v1/scheduled-transfers: recurrence (body) = WEEKLY (giá trị hợp lệ) |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-200 | L2 | P0 | new | POST /v1/scheduled-transfers trả 200 |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-201 | L2 | P0 | new | POST /v1/scheduled-transfers trả 201 |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-400 | L2 | P0 | new | POST /v1/scheduled-transfers trả 400 INVALID_REQUEST |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-401 | L2 | P0 | new | POST /v1/scheduled-transfers trả 401 UNAUTHORIZED |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-403 | L2 | P0 | new | POST /v1/scheduled-transfers trả 403 FORBIDDEN |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-404 | L2 | P0 | new | POST /v1/scheduled-transfers trả 404 ACCOUNT_NOT_FOUND |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-409 | L2 | P0 | new | POST /v1/scheduled-transfers trả 409 IDEMPOTENCY_CONFLICT |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-422-amount | L2 | P0 | new | POST /v1/scheduled-transfers trả 422 LIMIT_EXCEEDED |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-422-limit | L2 | P0 | new | POST /v1/scheduled-transfers trả 422 SCHEDULE_LIMIT_REACHED |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-422-same | L2 | P0 | new | POST /v1/scheduled-transfers trả 422 SAME_ACCOUNT |
| TC-L2-SCHEDULED-TRANSFER-EP-CreateSchedule-API-RSP-Create-422-time | L2 | P0 | new | POST /v1/scheduled-transfers trả 422 SCHEDULE_TIME_INVALID |
| TC-L2-SCHEDULED-TRANSFER-EP-GetSchedule-API-RSP-Get-200 | L2 | P0 | new | GET /v1/scheduled-transfers/{scheduleId} trả 200 |
| TC-L2-SCHEDULED-TRANSFER-EP-GetSchedule-API-RSP-Get-401 | L2 | P0 | new | GET /v1/scheduled-transfers/{scheduleId} trả 401 UNAUTHORIZED |
| TC-L2-SCHEDULED-TRANSFER-EP-GetSchedule-API-RSP-Get-403 | L2 | P0 | new | GET /v1/scheduled-transfers/{scheduleId} trả 403 FORBIDDEN |
| TC-L2-SCHEDULED-TRANSFER-EP-GetSchedule-API-RSP-Get-404 | L2 | P0 | new | GET /v1/scheduled-transfers/{scheduleId} trả 404 SCHEDULE_NOT_FOUND |
| TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-limit-ABOVE-MAX | L2 | P0 | new | GET /v1/scheduled-transfers/{scheduleId}/runs: limit (query) lớn hơn lớn nhất (101) |
| TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-limit-BELOW-MIN | L2 | P0 | new | GET /v1/scheduled-transfers/{scheduleId}/runs: limit (query) nhỏ hơn nhỏ nhất (0) |
| TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-limit-MAX | L2 | P0 | new | GET /v1/scheduled-transfers/{scheduleId}/runs: limit (query) đúng giá trị lớn nhất 100 |
| TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-limit-MIN | L2 | P0 | new | GET /v1/scheduled-transfers/{scheduleId}/runs: limit (query) đúng giá trị nhỏ nhất 1 |
| TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-offset-BELOW-MIN | L2 | P0 | new | GET /v1/scheduled-transfers/{scheduleId}/runs: offset (query) nhỏ hơn nhỏ nhất (-1) |
| TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-FLD-ListRuns-offset-MIN | L2 | P0 | new | GET /v1/scheduled-transfers/{scheduleId}/runs: offset (query) đúng giá trị nhỏ nhất 0 |
| TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-RSP-ListRuns-200 | L2 | P0 | new | GET /v1/scheduled-transfers/{scheduleId}/runs trả 200 |
| TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-RSP-ListRuns-401 | L2 | P0 | new | GET /v1/scheduled-transfers/{scheduleId}/runs trả 401 UNAUTHORIZED |
| TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-RSP-ListRuns-403 | L2 | P0 | new | GET /v1/scheduled-transfers/{scheduleId}/runs trả 403 FORBIDDEN |
| TC-L2-SCHEDULED-TRANSFER-EP-ListRuns-API-RSP-ListRuns-404 | L2 | P0 | new | GET /v1/scheduled-transfers/{scheduleId}/runs trả 404 SCHEDULE_NOT_FOUND |
| TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-limit-ABOVE-MAX | L2 | P0 | new | GET /v1/accounts/{accountId}/scheduled-transfers: limit (query) lớn hơn lớn nhất (101) |
| TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-limit-BELOW-MIN | L2 | P0 | new | GET /v1/accounts/{accountId}/scheduled-transfers: limit (query) nhỏ hơn nhỏ nhất (0) |
| TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-limit-MAX | L2 | P0 | new | GET /v1/accounts/{accountId}/scheduled-transfers: limit (query) đúng giá trị lớn nhất 100 |
| TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-limit-MIN | L2 | P0 | new | GET /v1/accounts/{accountId}/scheduled-transfers: limit (query) đúng giá trị nhỏ nhất 1 |
| TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-offset-BELOW-MIN | L2 | P0 | new | GET /v1/accounts/{accountId}/scheduled-transfers: offset (query) nhỏ hơn nhỏ nhất (-1) |
| TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-offset-MIN | L2 | P0 | new | GET /v1/accounts/{accountId}/scheduled-transfers: offset (query) đúng giá trị nhỏ nhất 0 |
| TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-ACTIVE | L2 | P0 | new | GET /v1/accounts/{accountId}/scheduled-transfers: status (query) = ACTIVE (giá trị hợp lệ) |
| TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-CANCELLED | L2 | P0 | new | GET /v1/accounts/{accountId}/scheduled-transfers: status (query) = CANCELLED (giá trị hợp lệ) |
| TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-COMPLETED | L2 | P0 | new | GET /v1/accounts/{accountId}/scheduled-transfers: status (query) = COMPLETED (giá trị hợp lệ) |
| TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-INVALID | L2 | P0 | new | GET /v1/accounts/{accountId}/scheduled-transfers: status (query) ngoài danh sách ACTIVE, PAUSED, COMPLETED, CANCELLED |
| TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-FLD-ListSchedules-status-ENUM-PAUSED | L2 | P0 | new | GET /v1/accounts/{accountId}/scheduled-transfers: status (query) = PAUSED (giá trị hợp lệ) |
| TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-RSP-ListSchedules-200 | L2 | P0 | new | GET /v1/accounts/{accountId}/scheduled-transfers trả 200 |
| TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-RSP-ListSchedules-400 | L2 | P0 | new | GET /v1/accounts/{accountId}/scheduled-transfers trả 400 INVALID_REQUEST |
| TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-RSP-ListSchedules-401 | L2 | P0 | new | GET /v1/accounts/{accountId}/scheduled-transfers trả 401 UNAUTHORIZED |
| TC-L2-SCHEDULED-TRANSFER-EP-ListSchedules-API-RSP-ListSchedules-403 | L2 | P0 | new | GET /v1/accounts/{accountId}/scheduled-transfers trả 403 FORBIDDEN |
| TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-ACTIVE | L2 | P0 | new | PATCH /v1/scheduled-transfers/{scheduleId}: status (body) = ACTIVE (giá trị hợp lệ) |
| TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-INVALID | L2 | P0 | new | PATCH /v1/scheduled-transfers/{scheduleId}: status (body) ngoài danh sách PAUSED, ACTIVE |
| TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-FLD-Patch-status-ENUM-PAUSED | L2 | P0 | new | PATCH /v1/scheduled-transfers/{scheduleId}: status (body) = PAUSED (giá trị hợp lệ) |
| TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-200 | L2 | P0 | new | PATCH /v1/scheduled-transfers/{scheduleId} trả 200 |
| TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-400 | L2 | P0 | new | PATCH /v1/scheduled-transfers/{scheduleId} trả 400 INVALID_REQUEST |
| TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-401 | L2 | P0 | new | PATCH /v1/scheduled-transfers/{scheduleId} trả 401 UNAUTHORIZED |
| TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-403 | L2 | P0 | new | PATCH /v1/scheduled-transfers/{scheduleId} trả 403 FORBIDDEN |
| TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-404 | L2 | P0 | new | PATCH /v1/scheduled-transfers/{scheduleId} trả 404 SCHEDULE_NOT_FOUND |
| TC-L2-SCHEDULED-TRANSFER-EP-UpdateScheduleStatus-API-RSP-Patch-409 | L2 | P0 | new | PATCH /v1/scheduled-transfers/{scheduleId} trả 409 SCHEDULE_STATE_INVALID |
| TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-1 | L2 | P0 | new | Xác nhận FR-040 thỏa: POST tạo lịch ONCE hợp lệ trả 201, có scheduleId và status ACTIVE |
| TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-2 | L2 | P0 | new | Xác nhận FR-040 thỏa: Số dư hai tài khoản không đổi và không có bản ghi ledger_entries mới sau khi tạo lịch |
| TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-3 | L2 | P0 | new | Xác nhận FR-040 thỏa: firstRunAt cách hiện tại < 5 phút hoặc > 90 ngày thì trả 422 SCHEDULE_TIME_INVALID |
| TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-4 | L2 | P0 | new | Xác nhận FR-040 thỏa: toAccountId không tồn tại thì trả 404 ACCOUNT_NOT_FOUND |
| TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-5 | L2 | P0 | new | Xác nhận FR-040 thỏa: fromAccountId khác chủ API key thì trả 403 FORBIDDEN |
| TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-FR-040-6 | L2 | P0 | new | Xác nhận FR-040 thỏa: Lịch thứ 21 ACTIVE/PAUSED của một tài khoản trả 422 SCHEDULE_LIMIT_REACHED |
| TC-L2-SCHEDULED-TRANSFER-FR-040-AC-AC-REV-003 | L2 | P0 | new | Xác nhận FR-040 thỏa: Chỉ lịch ACTIVE hoặc PAUSED được tính vào giới hạn 20 lịch của tài khoản; lịch COMPLETED hoặc CANCELLED không được tính |
| TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-FR-041-1 | L2 | P0 | new | Xác nhận FR-041 thỏa: Lịch DAILY, WEEKLY hoặc MONTHLY có endDate hoặc maxRuns được tạo thành công (201) |
| TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-FR-041-2 | L2 | P0 | new | Xác nhận FR-041 thỏa: Lịch định kỳ thiếu cả endDate và maxRuns trả 400 INVALID_REQUEST |
| TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-FR-041-3 | L2 | P0 | new | Xác nhận FR-041 thỏa: Sau khi một lần chạy kết thúc, hệ thống sinh lần chạy PENDING kế tiếp đúng chu kỳ cho đến khi đạt maxRuns hoặc endDate, rồi lịch COMPLETED |
| TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-FR-041-4 | L2 | P0 | new | Xác nhận FR-041 thỏa: Lịch MONTHLY đặt ngày 29, 30 hoặc 31 chạy vào ngày cuối tháng ở tháng không đủ ngày, và ngày 29/02 ở năm nhuận |
| TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-REV-001 | L2 | P0 | new | Xác nhận FR-041 thỏa: Lịch định kỳ có endDate trước ngày của lần chạy đầu, hoặc endDate quá 366 ngày kể từ lần chạy đầu, thì trả 422 SCHEDULE_TIME_INVALID |
| TC-L2-SCHEDULED-TRANSFER-FR-041-AC-AC-REV-002 | L2 | P0 | new | Xác nhận FR-041 thỏa: Lịch định kỳ có cả endDate và maxRuns kết thúc (COMPLETED) ở điều kiện đạt trước, không sinh thêm lần chạy PENDING |
| TC-L2-SCHEDULED-TRANSFER-FR-042-AC-AC-FR-042-1 | L2 | P0 | new | Xác nhận FR-042 thỏa: GET một lịch trả status và nextRunAt |
| TC-L2-SCHEDULED-TRANSFER-FR-042-AC-AC-FR-042-2 | L2 | P0 | new | Xác nhận FR-042 thỏa: GET danh sách lịch của tài khoản hỗ trợ lọc status và phân trang limit, offset |
| TC-L2-SCHEDULED-TRANSFER-FR-042-AC-AC-FR-042-3 | L2 | P0 | new | Xác nhận FR-042 thỏa: GET runs trả mỗi lần chạy với scheduledAt, status, transferId và reasonCode, sắp mới nhất trước |
| TC-L2-SCHEDULED-TRANSFER-FR-042-AC-AC-FR-042-4 | L2 | P0 | new | Xác nhận FR-042 thỏa: Người không phải chủ tài khoản nguồn nhận 403 FORBIDDEN; mã lịch không có nhận 404 SCHEDULE_NOT_FOUND |
| TC-L2-SCHEDULED-TRANSFER-FR-043-AC-AC-FR-043-1 | L2 | P0 | new | Xác nhận FR-043 thỏa: DELETE lịch ACTIVE đổi lịch thành CANCELLED và các lần chạy PENDING thành CANCELLED |
| TC-L2-SCHEDULED-TRANSFER-FR-043-AC-AC-FR-043-2 | L2 | P0 | new | Xác nhận FR-043 thỏa: DELETE lịch đã CANCELLED trả 200 với kết quả cũ |
| TC-L2-SCHEDULED-TRANSFER-FR-043-AC-AC-FR-043-3 | L2 | P0 | new | Xác nhận FR-043 thỏa: DELETE khi có lần chạy RUNNING hoặc lịch COMPLETED trả 409 SCHEDULE_NOT_CANCELLABLE |
| TC-L2-SCHEDULED-TRANSFER-FR-044-AC-AC-FR-044-1 | L2 | P0 | new | Xác nhận FR-044 thỏa: PATCH status=PAUSED trên lịch định kỳ ACTIVE đổi sang PAUSED, scheduler không chạy lịch này |
| TC-L2-SCHEDULED-TRANSFER-FR-044-AC-AC-FR-044-2 | L2 | P0 | new | Xác nhận FR-044 thỏa: PATCH status=ACTIVE trên lịch PAUSED đổi sang ACTIVE và các lần đã bỏ lỡ trong lúc tạm dừng có trạng thái SKIPPED, không bị chạy bù |
| TC-L2-SCHEDULED-TRANSFER-FR-044-AC-AC-FR-044-3 | L2 | P0 | new | Xác nhận FR-044 thỏa: PATCH sai trạng thái nguồn hoặc trên lịch ONCE trả 409 SCHEDULE_STATE_INVALID |
| TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-FR-045-1 | L2 | P0 | new | Xác nhận FR-045 thỏa: Lần chạy đến hạn tạo đúng một giao dịch COMPLETED qua luồng chuyển tiền hiện có, run SUCCEEDED có transferId |
| TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-FR-045-2 | L2 | P0 | new | Xác nhận FR-045 thỏa: Idempotency-Key của lần chạy đầu bằng mã lịch + thời điểm dự kiến |
| TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-FR-045-3 | L2 | P0 | new | Xác nhận FR-045 thỏa: Hạn mức ngày, số dư, trạng thái tài khoản và Fraud Check được kiểm tra tại lúc chạy, không phải lúc tạo lịch |
| TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-FR-045-4 | L2 | P0 | new | Xác nhận FR-045 thỏa: Giao dịch sinh từ lịch được tính vào hạn mức ngày và hiển thị trong lịch sử giao dịch |
| TC-L2-SCHEDULED-TRANSFER-FR-045-AC-AC-REV-004 | L2 | P0 | new | Xác nhận FR-045 thỏa: Giao dịch sinh từ lịch hiển thị trong lịch sử giao dịch (FR-013) kèm trường scheduleId là mã lịch |
| TC-L2-SCHEDULED-TRANSFER-FR-045-SCN-SCN-ScheduledRun | L2 | P0 | new | Chạy trọn luồng chính SCN-ScheduledRun |
| TC-L2-SCHEDULED-TRANSFER-FR-046-AC-AC-FR-046-1 | L2 | P0 | new | Xác nhận FR-046 thỏa: FRAUD_CHECK_UNAVAILABLE được thử lại tối đa 3 lần cách 5 phút, hết lượt thì run FAILED với reasonCode |
| TC-L2-SCHEDULED-TRANSFER-FR-046-AC-AC-FR-046-2 | L2 | P0 | new | Xác nhận FR-046 thỏa: INSUFFICIENT_FUNDS được thử lại tối đa 2 lần cách 1 giờ, hết lượt thì run FAILED |
| TC-L2-SCHEDULED-TRANSFER-FR-046-AC-AC-FR-046-3 | L2 | P0 | new | Xác nhận FR-046 thỏa: ACCOUNT_INACTIVE, FRAUD_REJECTED, LIMIT_EXCEEDED làm run FAILED ngay, không thử lại |
| TC-L2-SCHEDULED-TRANSFER-FR-046-AC-AC-FR-046-4 | L2 | P0 | new | Xác nhận FR-046 thỏa: Sau 3 lần chạy FAILED liên tiếp lịch định kỳ chuyển PAUSED |
| TC-L2-SCHEDULED-TRANSFER-FR-046-SCN-SCN-ScheduledRun | L2 | P0 | new | Chạy trọn luồng chính SCN-ScheduledRun |
| TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-FR-047-1 | L2 | P1 | new | Xác nhận FR-047 thỏa: Mỗi lần chạy thành công phát sự kiện ScheduledRunCompleted có transferId qua outbox |
| TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-FR-047-2 | L2 | P1 | new | Xác nhận FR-047 thỏa: Mỗi lần chạy FAILED phát sự kiện ScheduledRunFailed có reasonCode qua outbox |
| TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-FR-047-3 | L2 | P1 | new | Xác nhận FR-047 thỏa: SMS kết quả được gửi cho người gửi; lỗi gửi SMS không đổi trạng thái lần chạy |
| TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-REV-005 | L2 | P1 | new | Xác nhận FR-047 thỏa: Sự kiện ScheduledRunCompleted/ScheduledRunFailed có đủ các trường eventId, scheduleId, runId, fromAccountId, toAccountId, amount, correlationId, kèm transferId (thành công) hoặc reasonCode (thất bại), trên cùng topic với sự kiện giao dịch |
| TC-L2-SCHEDULED-TRANSFER-FR-047-AC-AC-REV-006 | L2 | P1 | new | Xác nhận FR-047 thỏa: Nội dung SMS của lần chạy thất bại nêu rõ số dư chỉ được kiểm tra lúc chạy |
| TC-L2-SCHEDULED-TRANSFER-FR-047-SCN-SCN-ScheduledRun | L2 | P1 | new | Chạy trọn luồng chính SCN-ScheduledRun |
| TC-L2-SCHEDULED-TRANSFER-FR-048-AC-AC-FR-048-1 | L2 | P0 | new | Xác nhận FR-048 thỏa: Tạo, hủy, tạm dừng và tiếp tục lịch đều ghi audit với correlationId, thời điểm, tác nhân là mã tài khoản, loại sự kiện và mã lịch |
| TC-L2-SCHEDULED-TRANSFER-FR-048-AC-AC-FR-048-2 | L2 | P0 | new | Xác nhận FR-048 thỏa: Mọi kết quả lần chạy ghi audit với tác nhân SCHEDULER và mã lịch |
| TC-L2-SCHEDULED-TRANSFER-FR-048-SCN-SCN-ScheduledRun | L2 | P0 | new | Chạy trọn luồng chính SCN-ScheduledRun |
| TC-L2-SCHEDULED-TRANSFER-FR-049-AC-AC-FR-049-1 | L2 | P0 | new | Xác nhận FR-049 thỏa: Lần chạy quá hạn không quá 24 giờ sau khi hệ thống ngừng được chạy đúng một lần |
| TC-L2-SCHEDULED-TRANSFER-FR-049-AC-AC-FR-049-2 | L2 | P0 | new | Xác nhận FR-049 thỏa: Lần chạy quá hạn quá 24 giờ chuyển MISSED, phát ScheduledRunFailed với reasonCode MISSED và không có giao dịch |
| TC-L2-SCHEDULED-TRANSFER-FR-049-SCN-SCN-ScheduledRun | L2 | P0 | new | Chạy trọn luồng chính SCN-ScheduledRun |
| TC-L2-SCHEDULED-TRANSFER-NFR-010-NFR-AT | L2 | P0 | new | Đạt ngưỡng NFR-010: Đúng ngưỡng 60 s |
| TC-L2-SCHEDULED-TRANSFER-NFR-010-NFR-OVER | L2 | P0 | new | Vượt ngưỡng NFR-010: Vượt ngưỡng (60+1) |
| TC-L2-SCHEDULED-TRANSFER-NFR-011-NFR-AT | L2 | P0 | new | Đạt ngưỡng NFR-011: Đúng ngưỡng 0 giao dịch trùng |
| TC-L2-SCHEDULED-TRANSFER-NFR-011-NFR-OVER | L2 | P0 | new | Vượt ngưỡng NFR-011: Vượt ngưỡng (0+1) |
| TC-L2-SCHEDULED-TRANSFER-NFR-012-NFR-AT | L2 | P0 | new | Đạt ngưỡng NFR-012: Đúng ngưỡng 5 chu kỳ quét |
| TC-L2-SCHEDULED-TRANSFER-NFR-012-NFR-OVER | L2 | P0 | new | Vượt ngưỡng NFR-012: Vượt ngưỡng (5+1) |
| TC-L2-SCHEDULED-TRANSFER-NFR-013-NFR-AT | L2 | P0 | new | Đạt ngưỡng NFR-013: Đúng ngưỡng 500 ms |
| TC-L2-SCHEDULED-TRANSFER-NFR-013-NFR-OVER | L2 | P0 | new | Vượt ngưỡng NFR-013: Vượt ngưỡng (500+1) |
| TC-L2-SCHEDULED-TRANSFER-NFR-014-NFR-AT | L2 | P0 | new | Đạt ngưỡng NFR-014: Đúng ngưỡng 100 % |
| TC-L2-SCHEDULED-TRANSFER-NFR-014-NFR-OVER | L2 | P0 | new | Vượt ngưỡng NFR-014: Vượt ngưỡng (100+1) |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-ACTIVE-PATCH-status-ACTIVE | L2 | P0 | new | Từ chối None khi ở trạng thái None |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-DELETE | L2 | P0 | new | Từ chối None khi ở trạng thái None |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-PATCH-status-ACTIVE | L2 | P0 | new | Từ chối None khi ở trạng thái None |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-PATCH-status-PAUSED | L2 | P0 | new | Từ chối None khi ở trạng thái None |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-l-n-ch-y-cu-i-k-t-th-c | L2 | P0 | new | Từ chối None khi ở trạng thái None |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-CANCELLED-scheduler-l-n-ch-y-FAILED-th-3-li-n-ti-p | L2 | P0 | new | Từ chối None khi ở trạng thái None |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-COMPLETED-DELETE | L2 | P0 | new | Từ chối None khi ở trạng thái None |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-COMPLETED-PATCH-status-ACTIVE | L2 | P0 | new | Từ chối None khi ở trạng thái None |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-COMPLETED-PATCH-status-PAUSED | L2 | P0 | new | Từ chối None khi ở trạng thái None |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-INV-COMPLETED-l-n-ch-y-cu-i-k-t-th-c | L2 | P0 | new | Từ chối None khi ở trạng thái None |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-1 | L2 | P0 | new | Chuyển ACTIVE sang PAUSED |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-2 | L2 | P0 | new | Chuyển PAUSED sang ACTIVE |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-3 | L2 | P0 | new | Chuyển ACTIVE sang PAUSED |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-4 | L2 | P0 | new | Chuyển ACTIVE sang COMPLETED |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-5 | L2 | P0 | new | Chuyển ACTIVE sang CANCELLED |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-6 | L2 | P0 | new | Chuyển PAUSED sang CANCELLED |
| TC-L2-SCHEDULED-TRANSFER-SM-Schedule-STM-TRN-Schedule-7 | L2 | P0 | new | Chuyển PAUSED sang COMPLETED |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED--u-chu-k-qu-t | L2 | P0 | new | Từ chối None khi ở trạng thái None |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-DELETE-l-ch | L2 | P0 | new | Từ chối None khi ở trạng thái None |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-l-i-chuy-n-ti-n-tr-giao-d-ch-COMPLETED-ho-c-ph-t-l-i | L2 | P0 | new | Từ chối None khi ở trạng thái None |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-l-i-tr-l-i | L2 | P0 | new | Từ chối None khi ở trạng thái None |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-l-i-tr-l-i-t-m-th-i-ho-c-thi-u-s-d | L2 | P0 | new | Từ chối None khi ở trạng thái None |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-scheduler-chi-m-l-n-ch-y | L2 | P0 | new | Từ chối None khi ở trạng thái None |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-scheduler-ph-c-h-i-l-n-ch-y-k-t | L2 | P0 | new | Từ chối None khi ở trạng thái None |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-CANCELLED-ti-p-t-c-l-ch-PAUSED | L2 | P0 | new | Từ chối None khi ở trạng thái None |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-FAILED-DELETE-l-ch | L2 | P0 | new | Từ chối None khi ở trạng thái None |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-INV-FAILED-l-i-chuy-n-ti-n-tr-giao-d-ch-COMPLETED-ho-c-ph-t-l-i | L2 | P0 | new | Từ chối None khi ở trạng thái None |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-1 | L2 | P0 | new | Chuyển PENDING sang RUNNING |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-2 | L2 | P0 | new | Chuyển RUNNING sang SUCCEEDED |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-3 | L2 | P0 | new | Chuyển RUNNING sang PENDING |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-4 | L2 | P0 | new | Chuyển RUNNING sang FAILED |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-5 | L2 | P0 | new | Chuyển PENDING sang SKIPPED |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-6 | L2 | P0 | new | Chuyển PENDING sang MISSED |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-7 | L2 | P0 | new | Chuyển PENDING sang CANCELLED |
| TC-L2-SCHEDULED-TRANSFER-SM-ScheduledRun-STM-TRN-ScheduledRun-8 | L2 | P0 | new | Chuyển RUNNING sang PENDING |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-BRK-INT-01-RECLOSE | L3 | P0 | new | INT-01: sau 30 giây breaker thử lại |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-BRK-INT-01-TRIP | L3 | P0 | new | INT-01: tỷ lệ lỗi >= 100% sau 5 lượt thì breaker mở |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Account-FAIL | L3 | P0 | new | MOD-Scheduler: MOD-Account lỗi |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Account-OK | L3 | P0 | new | MOD-Scheduler: MOD-Account hoạt động |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Audit-FAIL | L3 | P0 | new | MOD-Scheduler: MOD-Audit lỗi |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Audit-OK | L3 | P0 | new | MOD-Scheduler: MOD-Audit hoạt động |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Risk-FAIL | L3 | P0 | new | MOD-Scheduler: MOD-Risk lỗi |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Risk-OK | L3 | P0 | new | MOD-Scheduler: MOD-Risk hoạt động |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Transfer-FAIL | L3 | P0 | new | MOD-Scheduler: MOD-Transfer lỗi |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-DEP-MOD-Transfer-OK | L3 | P0 | new | MOD-Scheduler: MOD-Transfer hoạt động |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-ERR-INT-01-ERR-INT01-4xx | L3 | P0 | new | INT-01: HTTP 4xx (400, 401, 422) được xử lý như permanent |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-ERR-INT-01-ERR-INT01-5xx | L3 | P0 | new | INT-01: HTTP 5xx được xử lý như transient |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-ERR-INT-01-ERR-INT01-schema | L3 | P0 | new | INT-01: HTTP 200 nhưng response sai schema được xử lý như permanent |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-ERR-INT-01-ERR-INT01-timeout | L3 | P0 | new | INT-01: Timeout (connect/read) hoặc lỗi mạng được xử lý như unknownOutcome |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-IDM-INT-01-REPLAY | L3 | P0 | new | INT-01: gửi lại cùng yêu cầu |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-LIM-INT-01-maxPayloadKB-AT | L3 | P0 | new | INT-01: đúng giới hạn maxPayloadKB = 16 KB |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-LIM-INT-01-maxPayloadKB-OVER | L3 | P0 | new | INT-01: vượt giới hạn maxPayloadKB = 16 KB |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-LIM-INT-01-requestsPerSecond-AT | L3 | P0 | new | INT-01: đúng giới hạn requestsPerSecond = 50 req/s |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-LIM-INT-01-requestsPerSecond-OVER | L3 | P0 | new | INT-01: vượt giới hạn requestsPerSecond = 50 req/s |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-RTY-INT-01-EXHAUST | L3 | P0 | new | INT-01: hết lượt thử sau 2 lỗi tạm thời |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-RTY-INT-01-RECOVER | L3 | P0 | new | INT-01: phục hồi sau 1 lỗi tạm thời |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-TMO-INT-01-OVER | L3 | P0 | new | INT-01 không trả lời: dừng chờ sau totalMs |
| TC-L3-SCHEDULED-TRANSFER-MOD-Scheduler-TMO-INT-01-UNDER | L3 | P0 | new | INT-01 trả lời chậm vẫn thành công |

## Lịch sử phiên bản

| Phiên bản | Trạng thái | Baseline TDD | CR | Nút thêm | Nút đổi | Nút bỏ |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | Draft | b-a31cdc541d67 | CR-001 | 1555 | 0 | 0 |

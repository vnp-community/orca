---
spec: TSD-PAY-TestCases
feature: PAY
docKind: TestCases
version: 1
status: Draft
supersedes: none
baseline: b-c845d6c1c614
changes: []
llm: unknown
llm_model: ""
contentHash: 05d70ce6444cbbf210fd6cd47293b0b4a3d0f62f003162bb2a0df70995ff3dcc
---
# Test Cases: PAY

## 0. Thông tin tài liệu

| Trường | Giá trị |
| --- | --- |
| id | TSD-PAY-TestCases_v1 |
| Feature | PAY |
| Phiên bản | 1 |
| Trạng thái | Draft |
| Baseline TDD | TDD-PAY (b-c845d6c1c614) |
| CR của phiên bản này | — |
| Ontology | tso v1 |

## 1. Quy ước

- Bước: given (chuẩn bị), when (hành động), then (quan sát), cleanup (dọn).
- Kết quả mong đợi của assertion lấy từ oracle; dữ liệu nhạy cảm chỉ ghi `secretRef:<tên>`.

## 2. Test Cases theo mức

### 2.1 L1: Nghiệp vụ đầu-cuối

#### TC-L1-PAY-SCN-01-errorHandling-fail-INT-01-c

Chứng minh: Cổng thanh toán lỗi trong luồng Thanh toán đơn hàng

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L1-SCN-01-errorHandling-fail-INT-01-c` | `TSET-PAY-L1-SCN-01` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Cổng thanh toán lỗi trong luồng Thanh toán đơn hàng |  | SCN-01 |
| 3 | then | Quan sát kết quả: FB-01 |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | FB-01 |  | `TORC-PAY-FB-01-fail-SCN-01` |

#### TC-L1-PAY-SCN-01-functional-main-c

Chứng minh: Luồng chính Thanh toán đơn hàng: Khách gửi yêu cầu thanh toán → MOD-Payment gọi cổng thanh toán → MOD-Ledger ghi sổ

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L1-SCN-01-functional-main-c` | `TSET-PAY-L1-SCN-01` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Luồng chính Thanh toán đơn hàng: Khách gửi yêu cầu thanh toán → MOD-Payment gọi cổng thanh toán → MOD-Ledger ghi sổ |  | SCN-01 |
| 3 | then | Quan sát kết quả: MOD-Ledger ghi sổ |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | MOD-Ledger ghi sổ |  | `TORC-PAY-SCN-01-main` |

#### TC-L1-PAY-SCN-01-timing-budget-c

Chứng minh: Luồng Thanh toán đơn hàng hoàn tất trong 30000 ms

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L1-SCN-01-timing-budget-c` | `TSET-PAY-L1-SCN-01` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Luồng Thanh toán đơn hàng hoàn tất trong 30000 ms |  | SCN-01 |
| 3 | then | Quan sát kết quả: ≤ 30000 ms |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | ≤ 30000 ms | ms | `TORC-PAY-SCN-01-budget` |

#### TC-L1-PAY-SCN-01-timing-worst-INT-01-c

Chứng minh: Trường hợp xấu nhất khi gọi Cổng thanh toán: 3 lần × 25000 ms + backoff 0 ms = 75000 ms

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L1-SCN-01-timing-worst-INT-01-c` | `TSET-PAY-L1-SCN-01` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Trường hợp xấu nhất khi gọi Cổng thanh toán: 3 lần × 25000 ms + backoff 0 ms = 75000 ms |  | SCN-01 |
| 3 | then | Quan sát kết quả: 75000 ms ≤ 30000 ms (ngân sách của SCN-01) |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | 75000 ms ≤ 30000 ms (ngân sách của SCN-01) | ms | `TORC-PAY-SCN-01-worst-INT-01` |

### 2.2 L2: Nghiệp vụ

#### TC-L2-PAY-AC-FR-001-1-functional-c

Chứng minh: Thanh toán hợp lệ thì ghi sổ và trả mã giao dịch

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L2-AC-FR-001-1-functional-c` | `TSET-PAY-L2-FR-001` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Thanh toán hợp lệ thì ghi sổ và trả mã giao dịch |  | FR-001 |
| 3 | then | Quan sát kết quả: Thanh toán hợp lệ thì ghi sổ và trả mã giao dịch |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Thanh toán hợp lệ thì ghi sổ và trả mã giao dịch |  | `TORC-PAY-AC-FR-001-1` |

#### TC-L2-PAY-NFR-001-performance-target-c

Chứng minh: Thời gian đáp ứng thanh toán p95 dưới 200 ms

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P2 | có | `TCOV-PAY-L2-NFR-001-performance-target-c` | `TSET-PAY-L2-NFR-001` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Thời gian đáp ứng thanh toán p95 dưới 200 ms |  | NFR-001 |
| 3 | then | Quan sát kết quả: Thời gian đáp ứng thanh toán p95 dưới 200 ms (chỉ tiêu 200 ms) |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | Thời gian đáp ứng thanh toán p95 dưới 200 ms (chỉ tiêu 200 ms) | ms | `TORC-PAY-NFR-001` |

#### TC-L2-PAY-TRN-001-guardViolation-guard-c

Chứng minh: pay khi không thỏa điều kiện: số dư đủ

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L2-TRN-001-guardViolation-guard-c` | `TSET-PAY-L2-SM-001` | rule |

Fixture: `TFX-PAY-TRN-001-5dce5f` Đưa hệ thống về: Trạng thái NEW, không thỏa số dư đủ / dọn: Xóa dữ liệu do fixture tạo

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị: Trạng thái NEW, không thỏa số dư đủ |  |  |
| 2 | when | pay |  | TRN-001 |
| 3 | then | Quan sát kết quả: Không chuyển sang PAID; giữ NEW |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | Không chuyển sang PAID; giữ NEW |  | `TORC-PAY-TRN-001-guard` |

#### TC-L2-PAY-TRN-001-stateTransition-c

Chứng minh: NEW --pay--> PAID

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L2-TRN-001-stateTransition-c` | `TSET-PAY-L2-SM-001` | rule |

Fixture: `TFX-PAY-TRN-001-c5c90d` Đưa hệ thống về: Trạng thái NEW, số dư đủ / dọn: Xóa dữ liệu do fixture tạo

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị: Trạng thái NEW, số dư đủ |  |  |
| 2 | when | pay |  | TRN-001 |
| 3 | then | Quan sát kết quả: NEW → PAID; ghi sổ |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | NEW → PAID; ghi sổ |  | `TORC-PAY-TRN-001` |

#### TC-L2-PAY-TRN-002-guardViolation-guard-c

Chứng minh: refund khi không thỏa điều kiện: trong 30 ngày

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L2-TRN-002-guardViolation-guard-c` | `TSET-PAY-L2-SM-001` | rule |

Fixture: `TFX-PAY-TRN-002-c873c0` Đưa hệ thống về: Trạng thái PAID, không thỏa trong 30 ngày / dọn: Xóa dữ liệu do fixture tạo

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị: Trạng thái PAID, không thỏa trong 30 ngày |  |  |
| 2 | when | refund |  | TRN-002 |
| 3 | then | Quan sát kết quả: Không chuyển sang REFUNDED; giữ PAID |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | Không chuyển sang REFUNDED; giữ PAID |  | `TORC-PAY-TRN-002-guard` |

#### TC-L2-PAY-TRN-002-stateTransition-c

Chứng minh: PAID --refund--> REFUNDED

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L2-TRN-002-stateTransition-c` | `TSET-PAY-L2-SM-001` | rule |

Fixture: `TFX-PAY-TRN-002-f90e96` Đưa hệ thống về: Trạng thái PAID, trong 30 ngày / dọn: Xóa dữ liệu do fixture tạo

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Chuẩn bị: Trạng thái PAID, trong 30 ngày |  |  |
| 2 | when | refund |  | TRN-002 |
| 3 | then | Quan sát kết quả: PAID → REFUNDED; hoàn tiền |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | PAID → REFUNDED; hoàn tiền |  | `TORC-PAY-TRN-002` |

#### TC-L2-PAY-UR-001-functional-trace-c

Chứng minh: Khách hàng thanh toán đơn hàng nhanh và chắc chắn (qua FR-001)

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L2-UR-001-functional-trace-c` | `TSET-PAY-L2-UR-001` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Khách hàng thanh toán đơn hàng nhanh và chắc chắn (qua FR-001) |  | UR-001 |
| 3 | then | Quan sát kết quả: Các FR truy vết đều đạt: FR-001 |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Các FR truy vết đều đạt: FR-001 |  | `TORC-PAY-UR-001-trace` |

### 2.3 L3: Cô lập service

#### TC-L3-PAY-CB-01-stateTransition-close-c

Chứng minh: Lượt thử khi nửa mở thành công

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-PAY-L3-CB-01-stateTransition-close-c` | `TSET-PAY-L3-INT-01` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Lượt thử khi nửa mở thành công |  | CB-01 |
| 3 | then | Quan sát kết quả: Breaker đóng, lời gọi đi bình thường |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | Breaker đóng, lời gọi đi bình thường |  | `TORC-PAY-CB-01-close` |

#### TC-L3-PAY-CB-01-stateTransition-halfopen-c

Chứng minh: Hết 60 giây mở

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-PAY-L3-CB-01-stateTransition-halfopen-c` | `TSET-PAY-L3-INT-01` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Hết 60 giây mở |  | CB-01 |
| 3 | then | Quan sát kết quả: Breaker nửa mở, cho một lượt thử |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | Breaker nửa mở, cho một lượt thử |  | `TORC-PAY-CB-01-halfopen` |

#### TC-L3-PAY-CB-01-stateTransition-open-c

Chứng minh: Lỗi đạt 50% lỗi trong tối thiểu 10 lượt

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-PAY-L3-CB-01-stateTransition-open-c` | `TSET-PAY-L3-INT-01` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Lỗi đạt 50% lỗi trong tối thiểu 10 lượt |  | CB-01 |
| 3 | then | Quan sát kết quả: Breaker mở 60 giây, chặn lời gọi tới Cổng thanh toán |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | state | == | Breaker mở 60 giây, chặn lời gọi tới Cổng thanh toán |  | `TORC-PAY-CB-01-open` |

#### TC-L3-PAY-ERR-01-429-errorHandling-c

Chứng minh: Cổng thanh toán trả HTTP 429

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-PAY-L3-ERR-01-429-errorHandling-c` | `TSET-PAY-L3-INT-01` | rule |

Stub: `TSTUB-PAY-INT-01-errorTransient-429` INT-01 → errorTransient (429)

Tiêm lỗi: `TFI-PAY-INT-01-err-ERR-01-429` Stub INT-01 tạo lỗi transient — Tại INT-01, trước khi thao tác chính ghi dữ liệu

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | given | Bật tiêm lỗi: transient tại INT-01 (before) |  |  |
| 3 | when | Cổng thanh toán trả HTTP 429 |  | ERR-01-429 |
| 4 | then | Quan sát kết quả: chờ Retry-After rồi thử lại |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | chờ Retry-After rồi thử lại |  | `TORC-PAY-ERR-01-429` |

#### TC-L3-PAY-ERR-01-4xx-errorHandling-c

Chứng minh: Cổng thanh toán trả HTTP 400

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-PAY-L3-ERR-01-4xx-errorHandling-c` | `TSET-PAY-L3-INT-01` | rule |

Stub: `TSTUB-PAY-INT-01-errorPermanent-400` INT-01 → errorPermanent (400)

Tiêm lỗi: `TFI-PAY-INT-01-err-ERR-01-4xx` Stub INT-01 tạo lỗi permanent — Tại INT-01, trước khi thao tác chính ghi dữ liệu

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | given | Bật tiêm lỗi: permanent tại INT-01 (before) |  |  |
| 3 | when | Cổng thanh toán trả HTTP 400 |  | ERR-01-4xx |
| 4 | then | Quan sát kết quả: trả lỗi cho người dùng |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | trả lỗi cho người dùng |  | `TORC-PAY-ERR-01-4xx` |

#### TC-L3-PAY-ERR-01-5xx-errorHandling-c

Chứng minh: Cổng thanh toán trả HTTP 503

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-PAY-L3-ERR-01-5xx-errorHandling-c` | `TSET-PAY-L3-INT-01` | rule |

Stub: `TSTUB-PAY-INT-01-errorTransient-503` INT-01 → errorTransient (503)

Tiêm lỗi: `TFI-PAY-INT-01-err-ERR-01-5xx` Stub INT-01 tạo lỗi transient — Tại INT-01, trước khi thao tác chính ghi dữ liệu

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | given | Bật tiêm lỗi: transient tại INT-01 (before) |  |  |
| 3 | when | Cổng thanh toán trả HTTP 503 |  | ERR-01-5xx |
| 4 | then | Quan sát kết quả: backoff rồi thử lại |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | backoff rồi thử lại |  | `TORC-PAY-ERR-01-5xx` |

#### TC-L3-PAY-ERR-01-timeout-errorHandling-c

Chứng minh: Cổng thanh toán trả readMs hết hạn

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-PAY-L3-ERR-01-timeout-errorHandling-c` | `TSET-PAY-L3-INT-01` | rule |

Stub: `TSTUB-PAY-INT-01-timeout` INT-01 → timeout

Tiêm lỗi: `TFI-PAY-INT-01-err-ERR-01-timeout` Stub INT-01 tạo lỗi unknownOutcome — Tại INT-01, ngay sau khi giao dịch đã commit, trước khi trả kết quả

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | given | Bật tiêm lỗi: unknownOutcome tại INT-01 (afterCommit) |  |  |
| 3 | when | Cổng thanh toán trả readMs hết hạn |  | ERR-01-timeout |
| 4 | then | Quan sát kết quả: tra cứu trạng thái giao dịch |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | tra cứu trạng thái giao dịch |  | `TORC-PAY-ERR-01-timeout` |
| 2 | count | == | Hiệu ứng phụ sau commit đúng một lần: tra cứu trạng thái giao dịch |  | `TORC-PAY-ERR-01-timeout` |

#### TC-L3-PAY-ERR-02-bad-errorHandling-c

Chứng minh: API thanh toán trả HTTP 401

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L3-ERR-02-bad-errorHandling-c` | `TSET-PAY-L3-INT-02` | rule |

Stub: `TSTUB-PAY-INT-02-errorPermanent-401` INT-02 → errorPermanent (401)

Tiêm lỗi: `TFI-PAY-INT-02-err-ERR-02-bad` Stub INT-02 tạo lỗi permanent — Tại INT-02, trước khi thao tác chính ghi dữ liệu

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | given | Bật tiêm lỗi: permanent tại INT-02 (before) |  |  |
| 3 | when | API thanh toán trả HTTP 401 |  | ERR-02-bad |
| 4 | then | Quan sát kết quả: client sửa thông tin xác thực |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | client sửa thông tin xác thực |  | `TORC-PAY-ERR-02-bad` |

#### TC-L3-PAY-ERR-02-busy-errorHandling-c

Chứng minh: API thanh toán trả HTTP 503

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L3-ERR-02-busy-errorHandling-c` | `TSET-PAY-L3-INT-02` | rule |

Stub: `TSTUB-PAY-INT-02-errorTransient-503` INT-02 → errorTransient (503)

Tiêm lỗi: `TFI-PAY-INT-02-err-ERR-02-busy` Stub INT-02 tạo lỗi transient — Tại INT-02, trước khi thao tác chính ghi dữ liệu

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | given | Bật tiêm lỗi: transient tại INT-02 (before) |  |  |
| 3 | when | API thanh toán trả HTTP 503 |  | ERR-02-busy |
| 4 | then | Quan sát kết quả: client thử lại có backoff |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | client thử lại có backoff |  | `TORC-PAY-ERR-02-busy` |

#### TC-L3-PAY-ERR-02-lost-errorHandling-c

Chứng minh: API thanh toán trả client hết thời gian chờ

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L3-ERR-02-lost-errorHandling-c` | `TSET-PAY-L3-INT-02` | rule |

Stub: `TSTUB-PAY-INT-02-timeout` INT-02 → timeout

Tiêm lỗi: `TFI-PAY-INT-02-err-ERR-02-lost` Stub INT-02 tạo lỗi unknownOutcome — Tại INT-02, ngay sau khi giao dịch đã commit, trước khi trả kết quả

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | given | Bật tiêm lỗi: unknownOutcome tại INT-02 (afterCommit) |  |  |
| 3 | when | API thanh toán trả client hết thời gian chờ |  | ERR-02-lost |
| 4 | then | Quan sát kết quả: tra cứu trạng thái |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | tra cứu trạng thái |  | `TORC-PAY-ERR-02-lost` |
| 2 | count | == | Hiệu ứng phụ sau commit đúng một lần: tra cứu trạng thái |  | `TORC-PAY-ERR-02-lost` |

#### TC-L3-PAY-INT-01-concurrency-concurrent-dup-c

Chứng minh: Hai yêu cầu cùng khóa idempotency gửi đồng thời tới Cổng thanh toán

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-PAY-L3-INT-01-concurrency-concurrent-dup-c` | `TSET-PAY-L3-INT-01` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Hai yêu cầu cùng khóa idempotency gửi đồng thời tới Cổng thanh toán |  | INT-01 |
| 3 | then | Quan sát kết quả: Cùng khóa idempotency không tạo hiệu ứng lặp |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | count | == | Cùng khóa idempotency không tạo hiệu ứng lặp |  | `TORC-PAY-INT-01-idem` |

#### TC-L3-PAY-INT-01-idempotency-payload-mismatch-c

Chứng minh: Cùng khóa idempotency, khác payload tới Cổng thanh toán

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-PAY-L3-INT-01-idempotency-payload-mismatch-c` | `TSET-PAY-L3-INT-01` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Cùng khóa idempotency, khác payload tới Cổng thanh toán |  | INT-01 |
| 3 | then | Quan sát kết quả: Cùng khóa idempotency không tạo hiệu ứng lặp |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | count | == | Cùng khóa idempotency không tạo hiệu ứng lặp |  | `TORC-PAY-INT-01-idem` |

#### TC-L3-PAY-INT-01-idempotency-replay-c

Chứng minh: Gửi lại cùng yêu cầu (cùng khóa idempotency) tới Cổng thanh toán

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-PAY-L3-INT-01-idempotency-replay-c` | `TSET-PAY-L3-INT-01` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Gửi lại cùng yêu cầu (cùng khóa idempotency) tới Cổng thanh toán |  | INT-01 |
| 3 | then | Quan sát kết quả: Cùng khóa idempotency không tạo hiệu ứng lặp |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | count | == | Cùng khóa idempotency không tạo hiệu ứng lặp |  | `TORC-PAY-INT-01-idem` |

#### TC-L3-PAY-INT-01-timing-worst-INT-01-c

Chứng minh: Trường hợp xấu nhất khi gọi Cổng thanh toán: 3 lần × 25000 ms + backoff 0 ms = 75000 ms

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-PAY-L3-INT-01-timing-worst-INT-01-c` | `TSET-PAY-L3-INT-01` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Trường hợp xấu nhất khi gọi Cổng thanh toán: 3 lần × 25000 ms + backoff 0 ms = 75000 ms |  | INT-01 |
| 3 | then | Quan sát kết quả: 75000 ms ≤ 30000 ms (ngân sách của SCN-01) |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | 75000 ms ≤ 30000 ms (ngân sách của SCN-01) | ms | `TORC-PAY-INT-01-worst-INT-01` |

#### TC-L3-PAY-LIM-01-performance-requestsPerSecond-at-c

Chứng minh: Cổng thanh toán ở đúng giới hạn requestsPerSecond=50 request/giây

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-PAY-L3-LIM-01-performance-requestsPerSecond-at-c` | `TSET-PAY-L3-INT-01` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Cổng thanh toán ở đúng giới hạn requestsPerSecond=50 request/giây |  | LIM-01 |
| 3 | then | Quan sát kết quả: requestsPerSecond ≤ 50 request/giây |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | requestsPerSecond ≤ 50 request/giây | request/giây | `TORC-PAY-LIM-01-requestsPerSecond` |

#### TC-L3-PAY-LIM-01-performance-requestsPerSecond-over-c

Chứng minh: Cổng thanh toán vượt giới hạn requestsPerSecond=50 request/giây

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P0 | có | `TCOV-PAY-L3-LIM-01-performance-requestsPerSecond-over-c` | `TSET-PAY-L3-INT-01` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Cổng thanh toán vượt giới hạn requestsPerSecond=50 request/giây |  | LIM-01 |
| 3 | then | Quan sát kết quả: requestsPerSecond ≤ 50 request/giây |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | requestsPerSecond ≤ 50 request/giây | request/giây | `TORC-PAY-LIM-01-requestsPerSecond` |

#### TC-L3-PAY-LIM-02-inputValidation-maxPayloadKB-at-c

Chứng minh: API thanh toán ở đúng giới hạn maxPayloadKB=64 KB

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L3-LIM-02-inputValidation-maxPayloadKB-at-c` | `TSET-PAY-L3-INT-02` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | API thanh toán ở đúng giới hạn maxPayloadKB=64 KB |  | LIM-02 |
| 3 | then | Quan sát kết quả: maxPayloadKB ≤ 64 KB |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | maxPayloadKB ≤ 64 KB | KB | `TORC-PAY-LIM-02-maxPayloadKB` |

#### TC-L3-PAY-LIM-02-inputValidation-maxPayloadKB-over-c

Chứng minh: API thanh toán vượt giới hạn maxPayloadKB=64 KB

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L3-LIM-02-inputValidation-maxPayloadKB-over-c` | `TSET-PAY-L3-INT-02` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | API thanh toán vượt giới hạn maxPayloadKB=64 KB |  | LIM-02 |
| 3 | then | Quan sát kết quả: maxPayloadKB ≤ 64 KB |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | maxPayloadKB ≤ 64 KB | KB | `TORC-PAY-LIM-02-maxPayloadKB` |

#### TC-L3-PAY-LIM-02-performance-requestsPerSecond-at-c

Chứng minh: API thanh toán ở đúng giới hạn requestsPerSecond=20 request/giây

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L3-LIM-02-performance-requestsPerSecond-at-c` | `TSET-PAY-L3-INT-02` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | API thanh toán ở đúng giới hạn requestsPerSecond=20 request/giây |  | LIM-02 |
| 3 | then | Quan sát kết quả: requestsPerSecond ≤ 20 request/giây |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | requestsPerSecond ≤ 20 request/giây | request/giây | `TORC-PAY-LIM-02-requestsPerSecond` |

#### TC-L3-PAY-LIM-02-performance-requestsPerSecond-over-c

Chứng minh: API thanh toán vượt giới hạn requestsPerSecond=20 request/giây

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L3-LIM-02-performance-requestsPerSecond-over-c` | `TSET-PAY-L3-INT-02` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | API thanh toán vượt giới hạn requestsPerSecond=20 request/giây |  | LIM-02 |
| 3 | then | Quan sát kết quả: requestsPerSecond ≤ 20 request/giây |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | threshold | <= | requestsPerSecond ≤ 20 request/giây | request/giây | `TORC-PAY-LIM-02-requestsPerSecond` |

#### TC-L3-PAY-MECH-01-invariant-holds-c

Chứng minh: MECH-01

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P2 | có | `TCOV-PAY-L3-MECH-01-invariant-holds-c` | `TSET-PAY-L3-MECH-01` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | MECH-01 |  | MECH-01 |
| 3 | then | Quan sát kết quả: MECH-01 |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | count | == | MECH-01 |  | `TORC-PAY-MECH-01-holds` |

#### TC-L3-PAY-MOD-Payment-errorHandling-dep-MOD-Ledger-c

Chứng minh: Sổ cái lỗi hoặc không phản hồi khi Thanh toán gọi

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P2 | có | `TCOV-PAY-L3-MOD-Payment-errorHandling-dep-MOD-Ledger-c` | `TSET-PAY-L3-MOD-Payment` | rule |

Stub: `TSTUB-PAY-INT-01-success` INT-01 → success; `TSTUB-PAY-MOD-Ledger-errorTransient` MOD-Ledger → errorTransient

Tiêm lỗi: `TFI-PAY-MOD-Payment-dep-MOD-Ledger` Stub phụ thuộc trả lỗi hoặc không phản hồi — Tại MOD-Ledger, trong khi thao tác chính đang chạy

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | given | Bật tiêm lỗi: dependencyFailure tại MOD-Ledger (during) |  |  |
| 3 | when | Sổ cái lỗi hoặc không phản hồi khi Thanh toán gọi |  | MOD-Payment |
| 4 | then | Quan sát kết quả: Thanh toán xử lý lỗi của Sổ cái theo thiết kế |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | errorCode | == | Thanh toán xử lý lỗi của Sổ cái theo thiết kế |  | `TORC-PAY-MOD-Ledger-dep-MOD-Payment` |

#### TC-L3-PAY-SEC-02-access-allow-c

Chứng minh: Gọi API thanh toán với thông tin SEC-02 hợp lệ

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L3-SEC-02-access-allow-c` | `TSET-PAY-L3-INT-02` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Gọi API thanh toán với thông tin SEC-02 hợp lệ |  | SEC-02 |
| 3 | then | Quan sát kết quả: Chỉ yêu cầu xác thực đúng SEC-02 được phục vụ |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Chỉ yêu cầu xác thực đúng SEC-02 được phục vụ |  | `TORC-PAY-SEC-02-access` |

#### TC-L3-PAY-SEC-02-access-deny-c

Chứng minh: Gọi API thanh toán thiếu hoặc sai SEC-02

| Ưu tiên | Tự động | Phủ | Tập | Nguồn gốc |
| --- | --- | --- | --- | --- |
| P1 | có | `TCOV-PAY-L3-SEC-02-access-deny-c` | `TSET-PAY-L3-INT-02` | rule |

| # | Loại | Hành động | Đầu vào | Đích |
| --- | --- | --- | --- | --- |
| 1 | given | Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp |  |  |
| 2 | when | Gọi API thanh toán thiếu hoặc sai SEC-02 |  | SEC-02 |
| 3 | then | Quan sát kết quả: Chỉ yêu cầu xác thực đúng SEC-02 được phục vụ |  |  |

| # | Loại | Toán tử | Mong đợi | Đơn vị | Oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | value | == | Chỉ yêu cầu xác thực đúng SEC-02 được phục vụ |  | `TORC-PAY-SEC-02-access` |

## 3. Lịch sử phiên bản và phạm vi hồi quy

| Phiên bản | Trạng thái | CR | Thêm | Sửa | Bỏ | Số nút |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | Draft | — | 441 | 0 | 0 | 441 |

Hồi quy (0 test case đã baseline, thuộc phần bị ảnh hưởng nhưng không đổi): không có

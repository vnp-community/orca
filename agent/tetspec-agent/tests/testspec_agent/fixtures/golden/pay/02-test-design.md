---
spec: TSD-PAY-TestDesign
feature: PAY
docKind: TestDesign
version: 1
status: Draft
supersedes: none
baseline: b-c845d6c1c614
changes: []
llm: unknown
llm_model: ""
contentHash: 3f752d6cacabc57dc00340bc7e806da30753340aca890b674e161e5e5cd1f7a5
---
# Test Design: PAY

## 0. Thông tin tài liệu

| Trường | Giá trị |
| --- | --- |
| id | TSD-PAY-TestDesign_v1 |
| Feature | PAY |
| Phiên bản | 1 |
| Trạng thái | Draft |
| Baseline TDD | TDD-PAY (b-c845d6c1c614) |
| CR của phiên bản này | — |
| Ontology | tso v1 |

## 1. Cách tiếp cận theo mức

| Mức | Ý nghĩa | Môi trường | Thành phần |
| --- | --- | --- | --- |
| L1 | Nghiệp vụ đầu-cuối | Môi trường tích hợp đầu-cuối (L1) | toàn bộ service của feature, CSDL, hàng đợi, stub đối tác ngoài |
| L2 | Nghiệp vụ | Môi trường service (L2) | service đang kiểm, CSDL thật, stub phụ thuộc ngoài |
| L3 | Cô lập service | Môi trường service cô lập (L3) | service đang kiểm, stub mọi phụ thuộc, đồng hồ giả |

## 2. Thiết kế test

### TD-PAY-L1-SCN-01-boundaryValue

- TR: `TR-PAY-L1-SCN-01-performance` (L1, performance)
- Kỹ thuật: boundaryValue; tiêu chí phủ: Mỗi biên hai giá trị (trong và ngoài biên); mục tiêu 100 %
- Lý do: boundaryValue phù hợp timing

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-PAY-L1-SCN-01-timing-budget-c` | `TCND-PAY-SCN-01-timing-budget` | condition | Luồng Thanh toán đơn hàng hoàn tất trong 30000 ms |  | ≤ 30000 ms | `TORC-PAY-SCN-01-budget` |
| `TCOV-PAY-L1-SCN-01-timing-worst-INT-01-c` | `TCND-PAY-SCN-01-timing-worst-INT-01` | condition | Trường hợp xấu nhất khi gọi Cổng thanh toán: 3 lần × 25000 ms + backoff 0 ms = 75000 ms |  | 75000 ms ≤ 30000 ms (ngân sách của SCN-01) | `TORC-PAY-SCN-01-worst-INT-01` |

### TD-PAY-L1-SCN-01-scenario-functional

- TR: `TR-PAY-L1-SCN-01-functional` (L1, functional)
- Kỹ thuật: scenario; tiêu chí phủ: Mỗi đường đi (chính, lỗi) một coverage item; mục tiêu 100 %
- Lý do: scenario phù hợp functional

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-PAY-L1-SCN-01-functional-main-c` | `TCND-PAY-SCN-01-functional-main` | path | Luồng chính Thanh toán đơn hàng: Khách gửi yêu cầu thanh toán → MOD-Payment gọi cổng thanh toán → MOD-Ledger ghi sổ |  | MOD-Ledger ghi sổ | `TORC-PAY-SCN-01-main` |

### TD-PAY-L1-SCN-01-scenario-reliability

- TR: `TR-PAY-L1-SCN-01-reliability` (L1, reliability)
- Kỹ thuật: scenario; tiêu chí phủ: Mỗi đường đi (chính, lỗi) một coverage item; mục tiêu 100 %
- Lý do: scenario phù hợp errorHandling

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-PAY-L1-SCN-01-errorHandling-fail-INT-01-c` | `TCND-PAY-SCN-01-errorHandling-fail-INT-01` | path | Cổng thanh toán lỗi trong luồng Thanh toán đơn hàng |  | FB-01 | `TORC-PAY-FB-01-fail-SCN-01` |

### TD-PAY-L2-FR-001-useCase

- TR: `TR-PAY-L2-FR-001-functional` (L2, functional)
- Kỹ thuật: useCase; tiêu chí phủ: Mỗi tiêu chí chấp nhận một coverage item; mục tiêu 100 %
- Lý do: useCase phù hợp functional

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-PAY-L2-AC-FR-001-1-functional-c` | `TCND-PAY-AC-FR-001-1-functional` | condition | Thanh toán hợp lệ thì ghi sổ và trả mã giao dịch |  | Thanh toán hợp lệ thì ghi sổ và trả mã giao dịch | `TORC-PAY-AC-FR-001-1` |

### TD-PAY-L2-NFR-001-loadModel

- TR: `TR-PAY-L2-NFR-001-performance` (L2, performance)
- Kỹ thuật: loadModel; tiêu chí phủ: Mỗi chỉ tiêu tải đo theo workload khai báo; mục tiêu 100 %
- Lý do: loadModel phù hợp performance

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-PAY-L2-NFR-001-performance-target-c` | `TCND-PAY-NFR-001-performance-target` | condition | Thời gian đáp ứng thanh toán p95 dưới 200 ms |  | Thời gian đáp ứng thanh toán p95 dưới 200 ms (chỉ tiêu 200 ms) | `TORC-PAY-NFR-001` |

### TD-PAY-L2-SM-001-stateTransition

- TR: `TR-PAY-L2-SM-001-functional` (L2, functional)
- Kỹ thuật: stateTransition; tiêu chí phủ: Mỗi chuyển trạng thái và mỗi guard bị vi phạm; mục tiêu 100 %
- Lý do: stateTransition phù hợp guardViolation, stateTransition

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-PAY-L2-TRN-001-guardViolation-guard-c` | `TCND-PAY-TRN-001-guardViolation-guard` | condition | pay khi không thỏa điều kiện: số dư đủ |  | Không chuyển sang PAID; giữ NEW | `TORC-PAY-TRN-001-guard` |
| `TCOV-PAY-L2-TRN-001-stateTransition-c` | `TCND-PAY-TRN-001-stateTransition` | condition | NEW --pay--> PAID |  | NEW → PAID; ghi sổ | `TORC-PAY-TRN-001` |
| `TCOV-PAY-L2-TRN-002-guardViolation-guard-c` | `TCND-PAY-TRN-002-guardViolation-guard` | condition | refund khi không thỏa điều kiện: trong 30 ngày |  | Không chuyển sang REFUNDED; giữ PAID | `TORC-PAY-TRN-002-guard` |
| `TCOV-PAY-L2-TRN-002-stateTransition-c` | `TCND-PAY-TRN-002-stateTransition` | condition | PAID --refund--> REFUNDED |  | PAID → REFUNDED; hoàn tiền | `TORC-PAY-TRN-002` |

### TD-PAY-L2-UR-001-useCase

- TR: `TR-PAY-L2-UR-001-functional` (L2, functional)
- Kỹ thuật: useCase; tiêu chí phủ: Mỗi tiêu chí chấp nhận một coverage item; mục tiêu 100 %
- Lý do: useCase phù hợp functional

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-PAY-L2-UR-001-functional-trace-c` | `TCND-PAY-UR-001-functional-trace` | condition | Khách hàng thanh toán đơn hàng nhanh và chắc chắn (qua FR-001) |  | Các FR truy vết đều đạt: FR-001 | `TORC-PAY-UR-001-trace` |

### TD-PAY-L3-INT-01-boundaryValue

- TR: `TR-PAY-L3-INT-01-performance` (L3, performance)
- Kỹ thuật: boundaryValue; tiêu chí phủ: Mỗi biên hai giá trị (trong và ngoài biên); mục tiêu 100 %
- Lý do: boundaryValue phù hợp timing

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-PAY-L3-INT-01-timing-worst-INT-01-c` | `TCND-PAY-INT-01-timing-worst-INT-01` | condition | Trường hợp xấu nhất khi gọi Cổng thanh toán: 3 lần × 25000 ms + backoff 0 ms = 75000 ms |  | 75000 ms ≤ 30000 ms (ngân sách của SCN-01) | `TORC-PAY-INT-01-worst-INT-01` |

### TD-PAY-L3-INT-01-errorGuessing

- TR: `TR-PAY-L3-INT-01-reliability` (L3, reliability)
- Kỹ thuật: errorGuessing; tiêu chí phủ: Mỗi tình huống lỗi đoán trước một coverage item; mục tiêu 100 %
- Lý do: errorGuessing phù hợp idempotency

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-PAY-L3-INT-01-idempotency-payload-mismatch-c` | `TCND-PAY-INT-01-idempotency-payload-mismatch` | condition | Cùng khóa idempotency, khác payload tới Cổng thanh toán |  | Cùng khóa idempotency không tạo hiệu ứng lặp | `TORC-PAY-INT-01-idem` |
| `TCOV-PAY-L3-INT-01-idempotency-replay-c` | `TCND-PAY-INT-01-idempotency-replay` | condition | Gửi lại cùng yêu cầu (cùng khóa idempotency) tới Cổng thanh toán |  | Cùng khóa idempotency không tạo hiệu ứng lặp | `TORC-PAY-INT-01-idem` |

### TD-PAY-L3-INT-01-faultInjection

- TR: `TR-PAY-L3-INT-01-reliability` (L3, reliability)
- Kỹ thuật: faultInjection; tiêu chí phủ: Mỗi mô hình lỗi được tiêm ít nhất một lần; mục tiêu 100 %
- Lý do: faultInjection phù hợp concurrency, errorHandling

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-PAY-L3-ERR-01-429-errorHandling-c` | `TCND-PAY-ERR-01-429-errorHandling` | faultModel | Cổng thanh toán trả HTTP 429 |  | chờ Retry-After rồi thử lại | `TORC-PAY-ERR-01-429` |
| `TCOV-PAY-L3-ERR-01-4xx-errorHandling-c` | `TCND-PAY-ERR-01-4xx-errorHandling` | faultModel | Cổng thanh toán trả HTTP 400 |  | trả lỗi cho người dùng | `TORC-PAY-ERR-01-4xx` |
| `TCOV-PAY-L3-ERR-01-5xx-errorHandling-c` | `TCND-PAY-ERR-01-5xx-errorHandling` | faultModel | Cổng thanh toán trả HTTP 503 |  | backoff rồi thử lại | `TORC-PAY-ERR-01-5xx` |
| `TCOV-PAY-L3-ERR-01-timeout-errorHandling-c` | `TCND-PAY-ERR-01-timeout-errorHandling` | faultModel | Cổng thanh toán trả readMs hết hạn |  | tra cứu trạng thái giao dịch | `TORC-PAY-ERR-01-timeout` |
| `TCOV-PAY-L3-INT-01-concurrency-concurrent-dup-c` | `TCND-PAY-INT-01-concurrency-concurrent-dup` | condition | Hai yêu cầu cùng khóa idempotency gửi đồng thời tới Cổng thanh toán |  | Cùng khóa idempotency không tạo hiệu ứng lặp | `TORC-PAY-INT-01-idem` |

### TD-PAY-L3-INT-01-loadModel

- TR: `TR-PAY-L3-INT-01-performance` (L3, performance)
- Kỹ thuật: loadModel; tiêu chí phủ: Mỗi chỉ tiêu tải đo theo workload khai báo; mục tiêu 100 %
- Lý do: loadModel phù hợp performance

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-PAY-L3-LIM-01-performance-requestsPerSecond-at-c` | `TCND-PAY-LIM-01-performance-requestsPerSecond-at` | condition | Cổng thanh toán ở đúng giới hạn requestsPerSecond=50 request/giây |  | requestsPerSecond ≤ 50 request/giây | `TORC-PAY-LIM-01-requestsPerSecond` |
| `TCOV-PAY-L3-LIM-01-performance-requestsPerSecond-over-c` | `TCND-PAY-LIM-01-performance-requestsPerSecond-over` | condition | Cổng thanh toán vượt giới hạn requestsPerSecond=50 request/giây |  | requestsPerSecond ≤ 50 request/giây | `TORC-PAY-LIM-01-requestsPerSecond` |

### TD-PAY-L3-INT-01-stateTransition

- TR: `TR-PAY-L3-INT-01-functional` (L3, functional)
- Kỹ thuật: stateTransition; tiêu chí phủ: Mỗi chuyển trạng thái và mỗi guard bị vi phạm; mục tiêu 100 %
- Lý do: stateTransition phù hợp stateTransition

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-PAY-L3-CB-01-stateTransition-close-c` | `TCND-PAY-CB-01-stateTransition-close` | condition | Lượt thử khi nửa mở thành công |  | Breaker đóng, lời gọi đi bình thường | `TORC-PAY-CB-01-close` |
| `TCOV-PAY-L3-CB-01-stateTransition-halfopen-c` | `TCND-PAY-CB-01-stateTransition-halfopen` | condition | Hết 60 giây mở |  | Breaker nửa mở, cho một lượt thử | `TORC-PAY-CB-01-halfopen` |
| `TCOV-PAY-L3-CB-01-stateTransition-open-c` | `TCND-PAY-CB-01-stateTransition-open` | condition | Lỗi đạt 50% lỗi trong tối thiểu 10 lượt |  | Breaker mở 60 giây, chặn lời gọi tới Cổng thanh toán | `TORC-PAY-CB-01-open` |

### TD-PAY-L3-INT-02-accessMatrix

- TR: `TR-PAY-L3-INT-02-security` (L3, security)
- Kỹ thuật: accessMatrix; tiêu chí phủ: Mỗi điểm thực thi quyền × {cho phép, từ chối}; mục tiêu 100 %
- Lý do: accessMatrix phù hợp access

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-PAY-L3-SEC-02-access-allow-c` | `TCND-PAY-SEC-02-access-allow` | condition | Gọi API thanh toán với thông tin SEC-02 hợp lệ |  | Chỉ yêu cầu xác thực đúng SEC-02 được phục vụ | `TORC-PAY-SEC-02-access` |
| `TCOV-PAY-L3-SEC-02-access-deny-c` | `TCND-PAY-SEC-02-access-deny` | condition | Gọi API thanh toán thiếu hoặc sai SEC-02 |  | Chỉ yêu cầu xác thực đúng SEC-02 được phục vụ | `TORC-PAY-SEC-02-access` |

### TD-PAY-L3-INT-02-equivalencePartitioning

- TR: `TR-PAY-L3-INT-02-functional` (L3, functional)
- Kỹ thuật: equivalencePartitioning; tiêu chí phủ: Mỗi lớp tương đương một coverage item; mục tiêu 100 %
- Lý do: equivalencePartitioning phù hợp inputValidation

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-PAY-L3-LIM-02-inputValidation-maxPayloadKB-at-c` | `TCND-PAY-LIM-02-inputValidation-maxPayloadKB-at` | condition | API thanh toán ở đúng giới hạn maxPayloadKB=64 KB |  | maxPayloadKB ≤ 64 KB | `TORC-PAY-LIM-02-maxPayloadKB` |
| `TCOV-PAY-L3-LIM-02-inputValidation-maxPayloadKB-over-c` | `TCND-PAY-LIM-02-inputValidation-maxPayloadKB-over` | condition | API thanh toán vượt giới hạn maxPayloadKB=64 KB |  | maxPayloadKB ≤ 64 KB | `TORC-PAY-LIM-02-maxPayloadKB` |

### TD-PAY-L3-INT-02-faultInjection

- TR: `TR-PAY-L3-INT-02-reliability` (L3, reliability)
- Kỹ thuật: faultInjection; tiêu chí phủ: Mỗi mô hình lỗi được tiêm ít nhất một lần; mục tiêu 100 %
- Lý do: faultInjection phù hợp errorHandling

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-PAY-L3-ERR-02-bad-errorHandling-c` | `TCND-PAY-ERR-02-bad-errorHandling` | faultModel | API thanh toán trả HTTP 401 |  | client sửa thông tin xác thực | `TORC-PAY-ERR-02-bad` |
| `TCOV-PAY-L3-ERR-02-busy-errorHandling-c` | `TCND-PAY-ERR-02-busy-errorHandling` | faultModel | API thanh toán trả HTTP 503 |  | client thử lại có backoff | `TORC-PAY-ERR-02-busy` |
| `TCOV-PAY-L3-ERR-02-lost-errorHandling-c` | `TCND-PAY-ERR-02-lost-errorHandling` | faultModel | API thanh toán trả client hết thời gian chờ |  | tra cứu trạng thái | `TORC-PAY-ERR-02-lost` |

### TD-PAY-L3-INT-02-loadModel

- TR: `TR-PAY-L3-INT-02-performance` (L3, performance)
- Kỹ thuật: loadModel; tiêu chí phủ: Mỗi chỉ tiêu tải đo theo workload khai báo; mục tiêu 100 %
- Lý do: loadModel phù hợp performance

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-PAY-L3-LIM-02-performance-requestsPerSecond-at-c` | `TCND-PAY-LIM-02-performance-requestsPerSecond-at` | condition | API thanh toán ở đúng giới hạn requestsPerSecond=20 request/giây |  | requestsPerSecond ≤ 20 request/giây | `TORC-PAY-LIM-02-requestsPerSecond` |
| `TCOV-PAY-L3-LIM-02-performance-requestsPerSecond-over-c` | `TCND-PAY-LIM-02-performance-requestsPerSecond-over` | condition | API thanh toán vượt giới hạn requestsPerSecond=20 request/giây |  | requestsPerSecond ≤ 20 request/giây | `TORC-PAY-LIM-02-requestsPerSecond` |

### TD-PAY-L3-MECH-01-errorGuessing

- TR: `TR-PAY-L3-MECH-01-functional` (L3, functional)
- Kỹ thuật: errorGuessing; tiêu chí phủ: Mỗi tình huống lỗi đoán trước một coverage item; mục tiêu 100 %
- Lý do: errorGuessing phù hợp invariant

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-PAY-L3-MECH-01-invariant-holds-c` | `TCND-PAY-MECH-01-invariant-holds` | condition | MECH-01 |  | MECH-01 | `TORC-PAY-MECH-01-holds` |

### TD-PAY-L3-MOD-Payment-faultInjection

- TR: `TR-PAY-L3-MOD-Payment-reliability` (L3, reliability)
- Kỹ thuật: faultInjection; tiêu chí phủ: Mỗi mô hình lỗi được tiêm ít nhất một lần; mục tiêu 100 %
- Lý do: faultInjection phù hợp errorHandling

| Coverage item | Điều kiện | Loại | Mô tả | Giá trị | Kết quả mong đợi | Oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `TCOV-PAY-L3-MOD-Payment-errorHandling-dep-MOD-Ledger-c` | `TCND-PAY-MOD-Payment-errorHandling-dep-MOD-Ledger` | faultModel | Sổ cái lỗi hoặc không phản hồi khi Thanh toán gọi |  | Thanh toán xử lý lỗi của Sổ cái theo thiết kế | `TORC-PAY-MOD-Ledger-dep-MOD-Payment` |

## 3. Oracle

| Mã | Loại | Kết quả mong đợi | Cách đo | Dung sai | Nguồn |
| --- | --- | --- | --- | --- | --- |
| `TORC-PAY-AC-FR-001-1` | acceptanceCriterion | Thanh toán hợp lệ thì ghi sổ và trả mã giao dịch |  |  | `AC-FR-001-1` |
| `TORC-PAY-CB-01-close` | stateMachine | Breaker đóng, lời gọi đi bình thường |  |  | `CB-01` |
| `TORC-PAY-CB-01-halfopen` | stateMachine | Breaker nửa mở, cho một lượt thử |  |  | `CB-01` |
| `TORC-PAY-CB-01-open` | stateMachine | Breaker mở 60 giây, chặn lời gọi tới Cổng thanh toán |  |  | `CB-01` |
| `TORC-PAY-ERR-01-429` | errorContract | chờ Retry-After rồi thử lại |  |  | `ERR-01-429` |
| `TORC-PAY-ERR-01-4xx` | errorContract | trả lỗi cho người dùng |  |  | `ERR-01-4xx` |
| `TORC-PAY-ERR-01-5xx` | errorContract | backoff rồi thử lại |  |  | `ERR-01-5xx` |
| `TORC-PAY-ERR-01-timeout` | errorContract | tra cứu trạng thái giao dịch |  |  | `ERR-01-timeout` |
| `TORC-PAY-ERR-02-bad` | errorContract | client sửa thông tin xác thực |  |  | `ERR-02-bad` |
| `TORC-PAY-ERR-02-busy` | errorContract | client thử lại có backoff |  |  | `ERR-02-busy` |
| `TORC-PAY-ERR-02-lost` | errorContract | tra cứu trạng thái |  |  | `ERR-02-lost` |
| `TORC-PAY-FB-01-fail-SCN-01` | errorContract | FB-01 |  |  | `FB-01` |
| `TORC-PAY-INT-01-idem` | invariant | Cùng khóa idempotency không tạo hiệu ứng lặp |  |  | `INT-01` |
| `TORC-PAY-INT-01-worst-INT-01` | threshold | 75000 ms ≤ 30000 ms (ngân sách của SCN-01) | Cộng thời gian trường hợp xấu nhất (timeout mỗi lần × số lần + backoff), không cộng p95 |  | `INT-01` |
| `TORC-PAY-LIM-01-requestsPerSecond` | threshold | requestsPerSecond ≤ 50 request/giây | Đếm request/giây trong cửa sổ 1 giây |  | `LIM-01` |
| `TORC-PAY-LIM-02-maxPayloadKB` | threshold | maxPayloadKB ≤ 64 KB | Kích thước thân yêu cầu |  | `LIM-02` |
| `TORC-PAY-LIM-02-requestsPerSecond` | threshold | requestsPerSecond ≤ 20 request/giây | Đếm request/giây trong cửa sổ 1 giây |  | `LIM-02` |
| `TORC-PAY-MECH-01-holds` | invariant | MECH-01 |  |  | `MECH-01` |
| `TORC-PAY-MOD-Ledger-dep-MOD-Payment` | errorContract | Thanh toán xử lý lỗi của Sổ cái theo thiết kế |  |  | `MOD-Ledger` |
| `TORC-PAY-NFR-001` | threshold | Thời gian đáp ứng thanh toán p95 dưới 200 ms (chỉ tiêu 200 ms) | Thời gian đáp ứng thanh toán p95 dưới 200 ms |  | `NFR-001` |
| `TORC-PAY-SCN-01-budget` | threshold | ≤ 30000 ms | Đo từ bước đầu tới bước cuối mỗi lần chạy; gồm trường hợp xấu nhất của lời gọi ngoài |  | `SCN-01` |
| `TORC-PAY-SCN-01-main` | postcondition | MOD-Ledger ghi sổ |  |  | `SCN-01` |
| `TORC-PAY-SCN-01-worst-INT-01` | threshold | 75000 ms ≤ 30000 ms (ngân sách của SCN-01) | Cộng thời gian trường hợp xấu nhất (timeout mỗi lần × số lần + backoff), không cộng p95 |  | `SCN-01` |
| `TORC-PAY-SEC-02-access` | reference | Chỉ yêu cầu xác thực đúng SEC-02 được phục vụ |  |  | `SEC-02` |
| `TORC-PAY-TRN-001` | stateMachine | NEW → PAID; ghi sổ |  |  | `TRN-001` |
| `TORC-PAY-TRN-001-guard` | stateMachine | Không chuyển sang PAID; giữ NEW |  |  | `TRN-001` |
| `TORC-PAY-TRN-002` | stateMachine | PAID → REFUNDED; hoàn tiền |  |  | `TRN-002` |
| `TORC-PAY-TRN-002-guard` | stateMachine | Không chuyển sang REFUNDED; giữ PAID |  |  | `TRN-002` |
| `TORC-PAY-UR-001-trace` | reference | Các FR truy vết đều đạt: FR-001 |  |  | `UR-001` |

## 4. Lớp tương đương và biên

| Mã | Tham số | Loại | Giá trị |
| --- | --- | --- | --- |

## 5. Mô hình lỗi và tiêm lỗi

| Mã | Loại lỗi | Điểm tiêm | Thời điểm | Xử lý mong đợi | Cách tiêm |
| --- | --- | --- | --- | --- | --- |
| `TFM-PAY-INT-01-err-ERR-01-429` | transient | INT-01 | before | chờ Retry-After rồi thử lại | Stub INT-01 tạo lỗi transient — Tại INT-01, trước khi thao tác chính ghi dữ liệu |
| `TFM-PAY-INT-01-err-ERR-01-4xx` | permanent | INT-01 | before | trả lỗi cho người dùng | Stub INT-01 tạo lỗi permanent — Tại INT-01, trước khi thao tác chính ghi dữ liệu |
| `TFM-PAY-INT-01-err-ERR-01-5xx` | transient | INT-01 | before | backoff rồi thử lại | Stub INT-01 tạo lỗi transient — Tại INT-01, trước khi thao tác chính ghi dữ liệu |
| `TFM-PAY-INT-01-err-ERR-01-timeout` | unknownOutcome | INT-01 | afterCommit | tra cứu trạng thái giao dịch | Stub INT-01 tạo lỗi unknownOutcome — Tại INT-01, ngay sau khi giao dịch đã commit, trước khi trả kết quả |
| `TFM-PAY-INT-02-err-ERR-02-bad` | permanent | INT-02 | before | client sửa thông tin xác thực | Stub INT-02 tạo lỗi permanent — Tại INT-02, trước khi thao tác chính ghi dữ liệu |
| `TFM-PAY-INT-02-err-ERR-02-busy` | transient | INT-02 | before | client thử lại có backoff | Stub INT-02 tạo lỗi transient — Tại INT-02, trước khi thao tác chính ghi dữ liệu |
| `TFM-PAY-INT-02-err-ERR-02-lost` | unknownOutcome | INT-02 | afterCommit | tra cứu trạng thái | Stub INT-02 tạo lỗi unknownOutcome — Tại INT-02, ngay sau khi giao dịch đã commit, trước khi trả kết quả |
| `TFM-PAY-MOD-Payment-dep-MOD-Ledger` | dependencyFailure | MOD-Ledger | during | Thanh toán xử lý khi Sổ cái lỗi | Stub phụ thuộc trả lỗi hoặc không phản hồi — Tại MOD-Ledger, trong khi thao tác chính đang chạy |

## 6. Stub

| Mã | Đích | Hành vi | HTTP | Trễ (ms) |
| --- | --- | --- | --- | --- |
| `TSTUB-PAY-INT-01-errorPermanent-400` | INT-01 | errorPermanent | 400 |  |
| `TSTUB-PAY-INT-01-errorTransient-429` | INT-01 | errorTransient | 429 |  |
| `TSTUB-PAY-INT-01-errorTransient-503` | INT-01 | errorTransient | 503 |  |
| `TSTUB-PAY-INT-01-success` | INT-01 | success |  |  |
| `TSTUB-PAY-INT-01-timeout` | INT-01 | timeout |  |  |
| `TSTUB-PAY-INT-02-errorPermanent-401` | INT-02 | errorPermanent | 401 |  |
| `TSTUB-PAY-INT-02-errorTransient-503` | INT-02 | errorTransient | 503 |  |
| `TSTUB-PAY-INT-02-timeout` | INT-02 | timeout |  |  |
| `TSTUB-PAY-MOD-Ledger-errorTransient` | MOD-Ledger | errorTransient |  |  |

## 7. Đường đi kịch bản

| Mã | Loại | Các bước |
| --- | --- | --- |
| `TPATH-PAY-SCN-01-fail-INT-01` | failure | Khách gửi yêu cầu thanh toán → MOD-Payment gọi cổng thanh toán → Cổng thanh toán lỗi |
| `TPATH-PAY-SCN-01-main` | main | Khách gửi yêu cầu thanh toán → MOD-Payment gọi cổng thanh toán → MOD-Ledger ghi sổ |

## 8. Tập test

| Thứ tự | Mã | Mục đích | Môi trường | Điều kiện vào | Điều kiện ra | Số TC |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | `TSET-PAY-L1-SCN-01` | Các test case mức L1 của SCN-01 | `TENV-PAY-L1` | Môi trường tích hợp đầu-cuối sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 4 |
| 2 | `TSET-PAY-L2-FR-001` | Các test case mức L2 của FR-001 | `TENV-PAY-L2` | Môi trường service sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 1 |
| 3 | `TSET-PAY-L2-NFR-001` | Các test case mức L2 của NFR-001 | `TENV-PAY-L2` | Môi trường service sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 1 |
| 4 | `TSET-PAY-L2-SM-001` | Các test case mức L2 của SM-001 | `TENV-PAY-L2` | Môi trường service sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 4 |
| 5 | `TSET-PAY-L2-UR-001` | Các test case mức L2 của UR-001 | `TENV-PAY-L2` | Môi trường service sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 1 |
| 6 | `TSET-PAY-L3-INT-01` | Các test case mức L3 của INT-01 | `TENV-PAY-L3` | Môi trường service cô lập sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 13 |
| 7 | `TSET-PAY-L3-INT-02` | Các test case mức L3 của INT-02 | `TENV-PAY-L3` | Môi trường service cô lập sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 9 |
| 8 | `TSET-PAY-L3-MECH-01` | Các test case mức L3 của MECH-01 | `TENV-PAY-L3` | Môi trường service cô lập sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 1 |
| 9 | `TSET-PAY-L3-MOD-Payment` | Các test case mức L3 của MOD-Payment | `TENV-PAY-L3` | Môi trường service cô lập sẵn sàng; Dữ liệu và stub của tập đã nạp | 100% test case của tập được chạy; 0 test case P0/P1 thất bại chưa xử lý | 1 |

## 9. Lịch sử phiên bản và phạm vi hồi quy

| Phiên bản | Trạng thái | CR | Thêm | Sửa | Bỏ | Số nút |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | Draft | — | 441 | 0 | 0 | 441 |

Hồi quy (0 test case đã baseline, thuộc phần bị ảnh hưởng nhưng không đổi): không có

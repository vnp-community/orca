---
spec: TSD-PAY-TestRequirements
feature: PAY
docKind: TestRequirements
version: 1
status: Draft
supersedes: none
baseline: b-c845d6c1c614
changes: []
llm: unknown
llm_model: ""
contentHash: 996685a80ad7f072eb6afd96f1a7a662b619688fb77ba2eb14be7b88da801834
---
# Test Requirements: PAY

## 0. Thông tin tài liệu

| Trường | Giá trị |
| --- | --- |
| id | TSD-PAY-TestRequirements_v1 |
| Feature | PAY |
| Phiên bản | 1 |
| Trạng thái | Draft |
| Baseline TDD | TDD-PAY (b-c845d6c1c614) |
| CR của phiên bản này | — |
| Ontology | tso v1 |

## 1. Phạm vi và mục tiêu

- Phạm vi: feature PAY; mức test: L1 Nghiệp vụ đầu-cuối; L2 Nghiệp vụ; L3 Cô lập service
- Mục tiêu: mọi yêu cầu trong phạm vi có obligation; mọi điều kiện có oracle, coverage item và test case truy vết được

## 2. Test basis

| Tài liệu | Loại | Phiên bản | Trạng thái |
| --- | --- | --- | --- |
| SRS-PAY | srs |  | Baselined |
| TDD-PAY | tdd |  | Baselined |
| URD-PAY | urd |  | Baselined |

Nút TDD trong phạm vi được dùng làm căn cứ: AcceptanceCriterion 1, CircuitBreaker 1, ErrorMapping 7, FallbackStrategy 1, FunctionalRequirement 2, Integration 2, Mechanism 1, Module 2, NonFunctionalRequirement 1, PartnerLimit 2, Scenario 1, SecurityScheme 1, StateMachine 1, Transition 2, UserRequirement 1; nút ngữ cảnh: 0.

## 3. Phân tích (tầng A)

### TOB-PAY-FR-001-func

- Chủ thể: `FR-001`; nguồn gốc `seedRule`; ưu tiên P1
- Câu hỏi: FR-001 được đáp ứng: Hệ thống cho phép thanh toán đơn hàng qua cổng thanh toán

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-PAY-AC-FR-001-1-functional` | functional | Thanh toán hợp lệ thì ghi sổ và trả mã giao dịch | Thanh toán hợp lệ thì ghi sổ và trả mã giao dịch | acceptanceCriterion: Thanh toán hợp lệ thì ghi sổ và trả mã giao dịch | `AC-FR-001-1`, `FR-001` | stated |

### TOB-PAY-FR-002-func

- Chủ thể: `FR-002`; nguồn gốc `seedRule`; ưu tiên P2
- Câu hỏi: FR-002 được đáp ứng: Hệ thống cho phép xem lịch sử thanh toán

### TOB-PAY-INT-01-resilience

- Chủ thể: `INT-01`; nguồn gốc `seedRule`; ưu tiên P0; rủi ro `TRISK-PAY-INT-01`
- Câu hỏi: Tích hợp Cổng thanh toán xử lý đúng lỗi, thời gian, giới hạn

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-PAY-CB-01-stateTransition-close` | stateTransition | Lượt thử khi nửa mở thành công | Breaker đóng, lời gọi đi bình thường | stateMachine: Breaker đóng, lời gọi đi bình thường | `CB-01`, `INT-01` | rule |
| `TCND-PAY-CB-01-stateTransition-halfopen` | stateTransition | Hết 60 giây mở | Breaker nửa mở, cho một lượt thử | stateMachine: Breaker nửa mở, cho một lượt thử | `CB-01`, `INT-01` | rule |
| `TCND-PAY-CB-01-stateTransition-open` | stateTransition | Lỗi đạt 50% lỗi trong tối thiểu 10 lượt | Breaker mở 60 giây, chặn lời gọi tới Cổng thanh toán | stateMachine: Breaker mở 60 giây, chặn lời gọi tới Cổng thanh toán | `CB-01`, `INT-01` | rule |
| `TCND-PAY-ERR-01-429-errorHandling` | errorHandling | Cổng thanh toán trả HTTP 429 | chờ Retry-After rồi thử lại | errorContract: chờ Retry-After rồi thử lại | `ERR-01-429`, `INT-01` | rule |
| `TCND-PAY-ERR-01-4xx-errorHandling` | errorHandling | Cổng thanh toán trả HTTP 400 | trả lỗi cho người dùng | errorContract: trả lỗi cho người dùng | `ERR-01-4xx`, `INT-01` | rule |
| `TCND-PAY-ERR-01-5xx-errorHandling` | errorHandling | Cổng thanh toán trả HTTP 503 | backoff rồi thử lại | errorContract: backoff rồi thử lại | `ERR-01-5xx`, `INT-01` | rule |
| `TCND-PAY-ERR-01-timeout-errorHandling` | errorHandling | Cổng thanh toán trả readMs hết hạn | tra cứu trạng thái giao dịch | errorContract: tra cứu trạng thái giao dịch | `ERR-01-timeout`, `INT-01` | rule |
| `TCND-PAY-INT-01-concurrency-concurrent-dup` | concurrency | Hai yêu cầu cùng khóa idempotency gửi đồng thời tới Cổng thanh toán | Chỉ một hiệu ứng; yêu cầu còn lại nhận kết quả của yêu cầu thắng | invariant: Cùng khóa idempotency không tạo hiệu ứng lặp | `INT-01` | rule |
| `TCND-PAY-INT-01-idempotency-payload-mismatch` | idempotency | Cùng khóa idempotency, khác payload tới Cổng thanh toán | Bị từ chối theo chính sách xung đột | invariant: Cùng khóa idempotency không tạo hiệu ứng lặp | `INT-01` | rule |
| `TCND-PAY-INT-01-idempotency-replay` | idempotency | Gửi lại cùng yêu cầu (cùng khóa idempotency) tới Cổng thanh toán | Trả kết quả lần đầu, không tạo hiệu ứng lặp | invariant: Cùng khóa idempotency không tạo hiệu ứng lặp | `INT-01` | rule |
| `TCND-PAY-INT-01-timing-worst-INT-01` | timing | Trường hợp xấu nhất khi gọi Cổng thanh toán: 3 lần × 25000 ms + backoff 0 ms = 75000 ms | 75000 ms ≤ 30000 ms (ngân sách của SCN-01) | threshold: 75000 ms ≤ 30000 ms (ngân sách của SCN-01) (đo: Cộng thời gian trường hợp xấu nhất (timeout mỗi lần × số lần + backoff), không cộng p95) | `INT-01`, `RT-01`, `TO-01` | rule |
| `TCND-PAY-LIM-01-performance-requestsPerSecond-at` | performance | Cổng thanh toán ở đúng giới hạn requestsPerSecond=50 request/giây | Được phục vụ bình thường | threshold: requestsPerSecond ≤ 50 request/giây (đo: Đếm request/giây trong cửa sổ 1 giây) | `INT-01`, `LIM-01` | rule |
| `TCND-PAY-LIM-01-performance-requestsPerSecond-over` | performance | Cổng thanh toán vượt giới hạn requestsPerSecond=50 request/giây | Bị giới hạn hoặc từ chối theo hợp đồng đối tác | threshold: requestsPerSecond ≤ 50 request/giây (đo: Đếm request/giây trong cửa sổ 1 giây) | `INT-01`, `LIM-01` | rule |

### TOB-PAY-INT-02-resilience

- Chủ thể: `INT-02`; nguồn gốc `seedRule`; ưu tiên P1
- Câu hỏi: Tích hợp API thanh toán xử lý đúng lỗi, thời gian, giới hạn

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-PAY-ERR-02-bad-errorHandling` | errorHandling | API thanh toán trả HTTP 401 | client sửa thông tin xác thực | errorContract: client sửa thông tin xác thực | `ERR-02-bad`, `INT-02` | rule |
| `TCND-PAY-ERR-02-busy-errorHandling` | errorHandling | API thanh toán trả HTTP 503 | client thử lại có backoff | errorContract: client thử lại có backoff | `ERR-02-busy`, `INT-02` | rule |
| `TCND-PAY-ERR-02-lost-errorHandling` | errorHandling | API thanh toán trả client hết thời gian chờ | tra cứu trạng thái | errorContract: tra cứu trạng thái | `ERR-02-lost`, `INT-02` | rule |
| `TCND-PAY-LIM-02-inputValidation-maxPayloadKB-at` | inputValidation | API thanh toán ở đúng giới hạn maxPayloadKB=64 KB | Được phục vụ bình thường | threshold: maxPayloadKB ≤ 64 KB (đo: Kích thước thân yêu cầu) | `INT-02`, `LIM-02` | rule |
| `TCND-PAY-LIM-02-inputValidation-maxPayloadKB-over` | inputValidation | API thanh toán vượt giới hạn maxPayloadKB=64 KB | Bị giới hạn hoặc từ chối theo hợp đồng đối tác | threshold: maxPayloadKB ≤ 64 KB (đo: Kích thước thân yêu cầu) | `INT-02`, `LIM-02` | rule |
| `TCND-PAY-LIM-02-performance-requestsPerSecond-at` | performance | API thanh toán ở đúng giới hạn requestsPerSecond=20 request/giây | Được phục vụ bình thường | threshold: requestsPerSecond ≤ 20 request/giây (đo: Đếm request/giây trong cửa sổ 1 giây) | `INT-02`, `LIM-02` | rule |
| `TCND-PAY-LIM-02-performance-requestsPerSecond-over` | performance | API thanh toán vượt giới hạn requestsPerSecond=20 request/giây | Bị giới hạn hoặc từ chối theo hợp đồng đối tác | threshold: requestsPerSecond ≤ 20 request/giây (đo: Đếm request/giây trong cửa sổ 1 giây) | `INT-02`, `LIM-02` | rule |
| `TCND-PAY-SEC-02-access-allow` | access | Gọi API thanh toán với thông tin SEC-02 hợp lệ | Được phục vụ | reference: Chỉ yêu cầu xác thực đúng SEC-02 được phục vụ | `INT-02`, `SEC-02` | rule |
| `TCND-PAY-SEC-02-access-deny` | access | Gọi API thanh toán thiếu hoặc sai SEC-02 | Bị từ chối, không lộ dữ liệu | reference: Chỉ yêu cầu xác thực đúng SEC-02 được phục vụ | `INT-02`, `SEC-02` | rule |

### TOB-PAY-MECH-01-mechanism

- Chủ thể: `MECH-01`; nguồn gốc `seedRule`; ưu tiên P2
- Câu hỏi: Cơ chế MECH-01 hoạt động đúng

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-PAY-MECH-01-invariant-holds` | invariant | MECH-01 | MECH-01 | invariant: MECH-01 | `MECH-01`, `NFR-001` | rule |

### TOB-PAY-MOD-Payment-deps

- Chủ thể: `MOD-Payment`; nguồn gốc `seedRule`; ưu tiên P2
- Câu hỏi: Thanh toán xử lý đúng khi phụ thuộc lỗi

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-PAY-MOD-Payment-errorHandling-dep-MOD-Ledger` | errorHandling | Sổ cái lỗi hoặc không phản hồi khi Thanh toán gọi | Thanh toán xử lý lỗi của Sổ cái theo thiết kế | errorContract: Thanh toán xử lý lỗi của Sổ cái theo thiết kế | `MOD-Ledger`, `MOD-Payment` | rule |

### TOB-PAY-NFR-001-nfr

- Chủ thể: `NFR-001`; nguồn gốc `seedRule`; ưu tiên P2
- Câu hỏi: NFR-001 đạt chỉ tiêu: Thời gian đáp ứng thanh toán p95 dưới 200 ms

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-PAY-NFR-001-performance-target` | performance | Thời gian đáp ứng thanh toán p95 dưới 200 ms | Thời gian đáp ứng thanh toán p95 dưới 200 ms (chỉ tiêu 200 ms) | threshold: Thời gian đáp ứng thanh toán p95 dưới 200 ms (chỉ tiêu 200 ms) (đo: Thời gian đáp ứng thanh toán p95 dưới 200 ms) | `NFR-001` | rule |

### TOB-PAY-SCN-01-flow

- Chủ thể: `MOD-Ledger`, `MOD-Payment`, `SCN-01`; nguồn gốc `seedRule`; ưu tiên P1
- Câu hỏi: Luồng Thanh toán đơn hàng chạy đúng đầu-cuối qua MOD-Ledger, MOD-Payment

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-PAY-SCN-01-errorHandling-fail-INT-01` | errorHandling | Cổng thanh toán lỗi trong luồng Thanh toán đơn hàng | FB-01 | errorContract: FB-01 | `FB-01`, `INT-01`, `SCN-01` | rule |
| `TCND-PAY-SCN-01-functional-main` | functional | Luồng chính Thanh toán đơn hàng: Khách gửi yêu cầu thanh toán → MOD-Payment gọi cổng thanh toán → MOD-Ledger ghi sổ | MOD-Ledger ghi sổ | postcondition: MOD-Ledger ghi sổ | `MOD-Ledger`, `MOD-Payment`, `SCN-01` | rule |
| `TCND-PAY-SCN-01-timing-budget` | timing | Luồng Thanh toán đơn hàng hoàn tất trong 30000 ms | ≤ 30000 ms | threshold: ≤ 30000 ms (đo: Đo từ bước đầu tới bước cuối mỗi lần chạy; gồm trường hợp xấu nhất của lời gọi ngoài) | `SCN-01` | rule |
| `TCND-PAY-SCN-01-timing-worst-INT-01` | timing | Trường hợp xấu nhất khi gọi Cổng thanh toán: 3 lần × 25000 ms + backoff 0 ms = 75000 ms | 75000 ms ≤ 30000 ms (ngân sách của SCN-01) | threshold: 75000 ms ≤ 30000 ms (ngân sách của SCN-01) (đo: Cộng thời gian trường hợp xấu nhất (timeout mỗi lần × số lần + backoff), không cộng p95) | `INT-01`, `RT-01`, `SCN-01`, `TO-01` | rule |

### TOB-PAY-SM-001-states

- Chủ thể: `SM-001`; nguồn gốc `seedRule`; ưu tiên P1
- Câu hỏi: Vòng đời Vòng đời đơn hàng của Order chuyển đúng trạng thái

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-PAY-TRN-001-guardViolation-guard` | guardViolation | pay khi không thỏa điều kiện: số dư đủ | Không chuyển sang PAID; giữ NEW | stateMachine: Không chuyển sang PAID; giữ NEW | `SM-001`, `TRN-001` | rule |
| `TCND-PAY-TRN-001-stateTransition` | stateTransition | NEW --pay--> PAID | Trạng thái PAID; ghi sổ | stateMachine: NEW → PAID; ghi sổ | `SM-001`, `TRN-001` | stated |
| `TCND-PAY-TRN-002-guardViolation-guard` | guardViolation | refund khi không thỏa điều kiện: trong 30 ngày | Không chuyển sang REFUNDED; giữ PAID | stateMachine: Không chuyển sang REFUNDED; giữ PAID | `SM-001`, `TRN-002` | rule |
| `TCND-PAY-TRN-002-stateTransition` | stateTransition | PAID --refund--> REFUNDED | Trạng thái REFUNDED; hoàn tiền | stateMachine: PAID → REFUNDED; hoàn tiền | `SM-001`, `TRN-002` | stated |

### TOB-PAY-UR-001-trace

- Chủ thể: `UR-001`; nguồn gốc `seedRule`; ưu tiên P1
- Câu hỏi: UR-001 được đáp ứng qua các FR truy vết: Khách hàng thanh toán đơn hàng nhanh và chắc chắn

| Điều kiện | Loại | Nội dung | Kết quả mong đợi | Oracle | Căn cứ | Nguồn gốc |
| --- | --- | --- | --- | --- | --- | --- |
| `TCND-PAY-UR-001-functional-trace` | functional | Khách hàng thanh toán đơn hàng nhanh và chắc chắn (qua FR-001) | Các FR truy vết đều đạt: FR-001 | reference: Các FR truy vết đều đạt: FR-001 | `FR-001`, `UR-001` | rule |

## 4. Test Requirements theo mức

### 4.1 L1: Nghiệp vụ đầu-cuối

#### TR-PAY-L1-SCN-01-functional

Kiểm 1 điều kiện chức năng (functional) của SCN-01 ở mức L1: Luồng chính Thanh toán đơn hàng: Khách gửi yêu cầu thanh toán → MOD-Payment gọi 

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| SCN-01 | functional | P1 | test | rule |

Phạm vi: Chỉ phần chức năng mức L1 của SCN-01 (SCN-01); phần khác của cùng chủ thể nằm ở TR L1/performance, L1/reliability

Tiêu chí đạt:

1. Luồng chính Thanh toán đơn hàng: Khách gửi yêu cầu thanh toán → MOD-Payment gọi cổng thanh toán → MOD-Ledger ghi sổ → MOD-Ledger ghi sổ (`TCND-PAY-SCN-01-functional-main`)

#### TR-PAY-L1-SCN-01-performance

Kiểm 2 điều kiện hiệu năng (timing) của SCN-01 ở mức L1: Luồng Thanh toán đơn hàng hoàn tất trong 30000 ms; Trường hợp xấu nhất khi gọi Cổng thanh toán: 3 lần × 25000 ms + backoff 0 ms = 7

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| SCN-01 | performance | P1 | test | rule |

Phạm vi: Chỉ phần hiệu năng mức L1 của SCN-01 (SCN-01); phần khác của cùng chủ thể nằm ở TR L1/functional, L1/reliability

Tiêu chí đạt:

1. Luồng Thanh toán đơn hàng hoàn tất trong 30000 ms → ≤ 30000 ms (`TCND-PAY-SCN-01-timing-budget`)
2. Trường hợp xấu nhất khi gọi Cổng thanh toán: 3 lần × 25000 ms + backoff 0 ms = 75000 ms → 75000 ms ≤ 30000 ms (ngân sách của SCN-01) (`TCND-PAY-SCN-01-timing-worst-INT-01`)

#### TR-PAY-L1-SCN-01-reliability

Kiểm 1 điều kiện độ tin cậy (errorHandling) của SCN-01 ở mức L1: Cổng thanh toán lỗi trong luồng Thanh toán đơn hàng

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| SCN-01 | reliability | P1 | test | rule |

Phạm vi: Chỉ phần độ tin cậy mức L1 của SCN-01 (SCN-01); phần khác của cùng chủ thể nằm ở TR L1/functional, L1/performance

Tiêu chí đạt:

1. Cổng thanh toán lỗi trong luồng Thanh toán đơn hàng → FB-01 (`TCND-PAY-SCN-01-errorHandling-fail-INT-01`)

### 4.2 L2: Nghiệp vụ

#### TR-PAY-L2-FR-001-functional

Kiểm 1 điều kiện chức năng (functional) của FR-001 ở mức L2: Thanh toán hợp lệ thì ghi sổ và trả mã giao dịch

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| FR-001 | functional | P1 | test | rule |

Phạm vi: Chỉ phần chức năng mức L2 của FR-001 (FR-001)

Tiêu chí đạt:

1. Thanh toán hợp lệ thì ghi sổ và trả mã giao dịch (`TCND-PAY-AC-FR-001-1-functional`)

#### TR-PAY-L2-NFR-001-performance

Kiểm 1 điều kiện hiệu năng (performance) của NFR-001 ở mức L2: Thời gian đáp ứng thanh toán p95 dưới 200 ms

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| NFR-001 | performance | P2 | benchmark | rule |

Phạm vi: Chỉ phần hiệu năng mức L2 của NFR-001 (NFR-001)

Tiêu chí đạt:

1. Thời gian đáp ứng thanh toán p95 dưới 200 ms → Thời gian đáp ứng thanh toán p95 dưới 200 ms (chỉ tiêu 200 ms) (`TCND-PAY-NFR-001-performance-target`)

#### TR-PAY-L2-SM-001-functional

Kiểm 4 điều kiện chức năng (guardViolation, stateTransition) của SM-001 ở mức L2: pay khi không thỏa điều kiện: số dư đủ; NEW --pay--> PAID; refund khi không thỏa điều kiện: trong 30 ngày; …

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| SM-001 | functional | P1 | test | rule |

Phạm vi: Chỉ phần chức năng mức L2 của SM-001 (SM-001)

Tiêu chí đạt:

1. pay khi không thỏa điều kiện: số dư đủ → Không chuyển sang PAID; giữ NEW (`TCND-PAY-TRN-001-guardViolation-guard`)
2. NEW --pay--> PAID → Trạng thái PAID; ghi sổ (`TCND-PAY-TRN-001-stateTransition`)
3. refund khi không thỏa điều kiện: trong 30 ngày → Không chuyển sang REFUNDED; giữ PAID (`TCND-PAY-TRN-002-guardViolation-guard`)
4. PAID --refund--> REFUNDED → Trạng thái REFUNDED; hoàn tiền (`TCND-PAY-TRN-002-stateTransition`)

#### TR-PAY-L2-UR-001-functional

Kiểm 1 điều kiện chức năng (functional) của UR-001 ở mức L2: Khách hàng thanh toán đơn hàng nhanh và chắc chắn (qua FR-001)

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| UR-001 | functional | P1 | test | rule |

Phạm vi: Chỉ phần chức năng mức L2 của UR-001 (UR-001)

Tiêu chí đạt:

1. Khách hàng thanh toán đơn hàng nhanh và chắc chắn (qua FR-001) → Các FR truy vết đều đạt: FR-001 (`TCND-PAY-UR-001-functional-trace`)

### 4.3 L3: Cô lập service

#### TR-PAY-L3-INT-01-functional

Kiểm 3 điều kiện chức năng (stateTransition) của INT-01 ở mức L3: Lượt thử khi nửa mở thành công; Hết 60 giây mở; Lỗi đạt 50% lỗi trong tối thiểu 10 lượt

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| INT-01 | functional | P0 | test | rule |

Phạm vi: Chỉ phần chức năng mức L3 của INT-01 (INT-01); phần khác của cùng chủ thể nằm ở TR L3/performance, L3/reliability

Tiêu chí đạt:

1. Lượt thử khi nửa mở thành công → Breaker đóng, lời gọi đi bình thường (`TCND-PAY-CB-01-stateTransition-close`)
2. Hết 60 giây mở → Breaker nửa mở, cho một lượt thử (`TCND-PAY-CB-01-stateTransition-halfopen`)
3. Lỗi đạt 50% lỗi trong tối thiểu 10 lượt → Breaker mở 60 giây, chặn lời gọi tới Cổng thanh toán (`TCND-PAY-CB-01-stateTransition-open`)

#### TR-PAY-L3-INT-01-performance

Kiểm 3 điều kiện hiệu năng (performance, timing) của INT-01 ở mức L3: Trường hợp xấu nhất khi gọi Cổng thanh toán: 3 lần × 25000 ms + backoff 0 ms = 7; Cổng thanh toán ở đúng giới hạn requestsPerSecond=50 request/giây; Cổng thanh toán vượt giới hạn requestsPerSecond=50 request/giây

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| INT-01 | performance | P0 | test | rule |

Phạm vi: Chỉ phần hiệu năng mức L3 của INT-01 (INT-01); phần khác của cùng chủ thể nằm ở TR L3/functional, L3/reliability

Tiêu chí đạt:

1. Trường hợp xấu nhất khi gọi Cổng thanh toán: 3 lần × 25000 ms + backoff 0 ms = 75000 ms → 75000 ms ≤ 30000 ms (ngân sách của SCN-01) (`TCND-PAY-INT-01-timing-worst-INT-01`)
2. Cổng thanh toán ở đúng giới hạn requestsPerSecond=50 request/giây → Được phục vụ bình thường (`TCND-PAY-LIM-01-performance-requestsPerSecond-at`)
3. Cổng thanh toán vượt giới hạn requestsPerSecond=50 request/giây → Bị giới hạn hoặc từ chối theo hợp đồng đối tác (`TCND-PAY-LIM-01-performance-requestsPerSecond-over`)

#### TR-PAY-L3-INT-01-reliability

Kiểm 7 điều kiện độ tin cậy (concurrency, errorHandling, idempotency) của INT-01 ở mức L3: Cổng thanh toán trả HTTP 429; Cổng thanh toán trả HTTP 400; Cổng thanh toán trả HTTP 503; …

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| INT-01 | reliability | P0 | test | rule |

Phạm vi: Chỉ phần độ tin cậy mức L3 của INT-01 (INT-01); phần khác của cùng chủ thể nằm ở TR L3/functional, L3/performance

Tiêu chí đạt:

1. Cổng thanh toán trả HTTP 429 → chờ Retry-After rồi thử lại (`TCND-PAY-ERR-01-429-errorHandling`)
2. Cổng thanh toán trả HTTP 400 → trả lỗi cho người dùng (`TCND-PAY-ERR-01-4xx-errorHandling`)
3. Cổng thanh toán trả HTTP 503 → backoff rồi thử lại (`TCND-PAY-ERR-01-5xx-errorHandling`)
4. Cổng thanh toán trả readMs hết hạn → tra cứu trạng thái giao dịch (`TCND-PAY-ERR-01-timeout-errorHandling`)
5. Hai yêu cầu cùng khóa idempotency gửi đồng thời tới Cổng thanh toán → Chỉ một hiệu ứng; yêu cầu còn lại nhận kết quả của yêu cầu thắng (`TCND-PAY-INT-01-concurrency-concurrent-dup`)
6. Cùng khóa idempotency, khác payload tới Cổng thanh toán → Bị từ chối theo chính sách xung đột (`TCND-PAY-INT-01-idempotency-payload-mismatch`)
7. Gửi lại cùng yêu cầu (cùng khóa idempotency) tới Cổng thanh toán → Trả kết quả lần đầu, không tạo hiệu ứng lặp (`TCND-PAY-INT-01-idempotency-replay`)

#### TR-PAY-L3-INT-02-functional

Kiểm 2 điều kiện chức năng (inputValidation) của INT-02 ở mức L3: API thanh toán ở đúng giới hạn maxPayloadKB=64 KB; API thanh toán vượt giới hạn maxPayloadKB=64 KB

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| INT-02 | functional | P1 | test | rule |

Phạm vi: Chỉ phần chức năng mức L3 của INT-02 (INT-02); phần khác của cùng chủ thể nằm ở TR L3/performance, L3/reliability, L3/security

Tiêu chí đạt:

1. API thanh toán ở đúng giới hạn maxPayloadKB=64 KB → Được phục vụ bình thường (`TCND-PAY-LIM-02-inputValidation-maxPayloadKB-at`)
2. API thanh toán vượt giới hạn maxPayloadKB=64 KB → Bị giới hạn hoặc từ chối theo hợp đồng đối tác (`TCND-PAY-LIM-02-inputValidation-maxPayloadKB-over`)

#### TR-PAY-L3-INT-02-performance

Kiểm 2 điều kiện hiệu năng (performance) của INT-02 ở mức L3: API thanh toán ở đúng giới hạn requestsPerSecond=20 request/giây; API thanh toán vượt giới hạn requestsPerSecond=20 request/giây

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| INT-02 | performance | P1 | benchmark | rule |

Phạm vi: Chỉ phần hiệu năng mức L3 của INT-02 (INT-02); phần khác của cùng chủ thể nằm ở TR L3/functional, L3/reliability, L3/security

Tiêu chí đạt:

1. API thanh toán ở đúng giới hạn requestsPerSecond=20 request/giây → Được phục vụ bình thường (`TCND-PAY-LIM-02-performance-requestsPerSecond-at`)
2. API thanh toán vượt giới hạn requestsPerSecond=20 request/giây → Bị giới hạn hoặc từ chối theo hợp đồng đối tác (`TCND-PAY-LIM-02-performance-requestsPerSecond-over`)

#### TR-PAY-L3-INT-02-reliability

Kiểm 3 điều kiện độ tin cậy (errorHandling) của INT-02 ở mức L3: API thanh toán trả HTTP 401; API thanh toán trả HTTP 503; API thanh toán trả client hết thời gian chờ

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| INT-02 | reliability | P1 | test | rule |

Phạm vi: Chỉ phần độ tin cậy mức L3 của INT-02 (INT-02); phần khác của cùng chủ thể nằm ở TR L3/functional, L3/performance, L3/security

Tiêu chí đạt:

1. API thanh toán trả HTTP 401 → client sửa thông tin xác thực (`TCND-PAY-ERR-02-bad-errorHandling`)
2. API thanh toán trả HTTP 503 → client thử lại có backoff (`TCND-PAY-ERR-02-busy-errorHandling`)
3. API thanh toán trả client hết thời gian chờ → tra cứu trạng thái (`TCND-PAY-ERR-02-lost-errorHandling`)

#### TR-PAY-L3-INT-02-security

Kiểm 2 điều kiện bảo mật (access) của INT-02 ở mức L3: Gọi API thanh toán với thông tin SEC-02 hợp lệ; Gọi API thanh toán thiếu hoặc sai SEC-02

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| INT-02 | security | P1 | test | rule |

Phạm vi: Chỉ phần bảo mật mức L3 của INT-02 (INT-02); phần khác của cùng chủ thể nằm ở TR L3/functional, L3/performance, L3/reliability

Tiêu chí đạt:

1. Gọi API thanh toán với thông tin SEC-02 hợp lệ → Được phục vụ (`TCND-PAY-SEC-02-access-allow`)
2. Gọi API thanh toán thiếu hoặc sai SEC-02 → Bị từ chối, không lộ dữ liệu (`TCND-PAY-SEC-02-access-deny`)

#### TR-PAY-L3-MECH-01-functional

Kiểm 1 điều kiện chức năng (invariant) của MECH-01 ở mức L3: MECH-01

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| MECH-01 | functional | P2 | test | rule |

Phạm vi: Chỉ phần chức năng mức L3 của MECH-01 (MECH-01)

Tiêu chí đạt:

1. MECH-01 (`TCND-PAY-MECH-01-invariant-holds`)

#### TR-PAY-L3-MOD-Payment-reliability

Kiểm 1 điều kiện độ tin cậy (errorHandling) của MOD-Payment ở mức L3: Sổ cái lỗi hoặc không phản hồi khi Thanh toán gọi

| Chủ thể | Loại test | Ưu tiên | Phương pháp | Nguồn gốc |
| --- | --- | --- | --- | --- |
| MOD-Payment | reliability | P2 | test | rule |

Phạm vi: Chỉ phần độ tin cậy mức L3 của MOD-Payment (MOD-Payment)

Tiêu chí đạt:

1. Sổ cái lỗi hoặc không phản hồi khi Thanh toán gọi → Thanh toán xử lý lỗi của Sổ cái theo thiết kế (`TCND-PAY-MOD-Payment-errorHandling-dep-MOD-Ledger`)

## 5. Truy vết

| Nguồn | Obligation | Điều kiện | TR | Coverage item | Test case |
| --- | --- | --- | --- | --- | --- |
| `AC-FR-001-1`, `FR-001` | `TOB-PAY-FR-001-func` | `TCND-PAY-AC-FR-001-1-functional` | `TR-PAY-L2-FR-001-functional` | 1 | `TC-L2-PAY-AC-FR-001-1-functional-c` |
| `CB-01`, `INT-01` | `TOB-PAY-INT-01-resilience` | `TCND-PAY-CB-01-stateTransition-close` | `TR-PAY-L3-INT-01-functional` | 1 | `TC-L3-PAY-CB-01-stateTransition-close-c` |
| `CB-01`, `INT-01` | `TOB-PAY-INT-01-resilience` | `TCND-PAY-CB-01-stateTransition-halfopen` | `TR-PAY-L3-INT-01-functional` | 1 | `TC-L3-PAY-CB-01-stateTransition-halfopen-c` |
| `CB-01`, `INT-01` | `TOB-PAY-INT-01-resilience` | `TCND-PAY-CB-01-stateTransition-open` | `TR-PAY-L3-INT-01-functional` | 1 | `TC-L3-PAY-CB-01-stateTransition-open-c` |
| `ERR-01-429`, `INT-01` | `TOB-PAY-INT-01-resilience` | `TCND-PAY-ERR-01-429-errorHandling` | `TR-PAY-L3-INT-01-reliability` | 1 | `TC-L3-PAY-ERR-01-429-errorHandling-c` |
| `ERR-01-4xx`, `INT-01` | `TOB-PAY-INT-01-resilience` | `TCND-PAY-ERR-01-4xx-errorHandling` | `TR-PAY-L3-INT-01-reliability` | 1 | `TC-L3-PAY-ERR-01-4xx-errorHandling-c` |
| `ERR-01-5xx`, `INT-01` | `TOB-PAY-INT-01-resilience` | `TCND-PAY-ERR-01-5xx-errorHandling` | `TR-PAY-L3-INT-01-reliability` | 1 | `TC-L3-PAY-ERR-01-5xx-errorHandling-c` |
| `ERR-01-timeout`, `INT-01` | `TOB-PAY-INT-01-resilience` | `TCND-PAY-ERR-01-timeout-errorHandling` | `TR-PAY-L3-INT-01-reliability` | 1 | `TC-L3-PAY-ERR-01-timeout-errorHandling-c` |
| `ERR-02-bad`, `INT-02` | `TOB-PAY-INT-02-resilience` | `TCND-PAY-ERR-02-bad-errorHandling` | `TR-PAY-L3-INT-02-reliability` | 1 | `TC-L3-PAY-ERR-02-bad-errorHandling-c` |
| `ERR-02-busy`, `INT-02` | `TOB-PAY-INT-02-resilience` | `TCND-PAY-ERR-02-busy-errorHandling` | `TR-PAY-L3-INT-02-reliability` | 1 | `TC-L3-PAY-ERR-02-busy-errorHandling-c` |
| `ERR-02-lost`, `INT-02` | `TOB-PAY-INT-02-resilience` | `TCND-PAY-ERR-02-lost-errorHandling` | `TR-PAY-L3-INT-02-reliability` | 1 | `TC-L3-PAY-ERR-02-lost-errorHandling-c` |
| `INT-01` | `TOB-PAY-INT-01-resilience` | `TCND-PAY-INT-01-concurrency-concurrent-dup` | `TR-PAY-L3-INT-01-reliability` | 1 | `TC-L3-PAY-INT-01-concurrency-concurrent-dup-c` |
| `INT-01` | `TOB-PAY-INT-01-resilience` | `TCND-PAY-INT-01-idempotency-payload-mismatch` | `TR-PAY-L3-INT-01-reliability` | 1 | `TC-L3-PAY-INT-01-idempotency-payload-mismatch-c` |
| `INT-01` | `TOB-PAY-INT-01-resilience` | `TCND-PAY-INT-01-idempotency-replay` | `TR-PAY-L3-INT-01-reliability` | 1 | `TC-L3-PAY-INT-01-idempotency-replay-c` |
| `INT-01`, `RT-01`, `TO-01` | `TOB-PAY-INT-01-resilience` | `TCND-PAY-INT-01-timing-worst-INT-01` | `TR-PAY-L3-INT-01-performance` | 1 | `TC-L3-PAY-INT-01-timing-worst-INT-01-c` |
| `INT-01`, `LIM-01` | `TOB-PAY-INT-01-resilience` | `TCND-PAY-LIM-01-performance-requestsPerSecond-at` | `TR-PAY-L3-INT-01-performance` | 1 | `TC-L3-PAY-LIM-01-performance-requestsPerSecond-at-c` |
| `INT-01`, `LIM-01` | `TOB-PAY-INT-01-resilience` | `TCND-PAY-LIM-01-performance-requestsPerSecond-over` | `TR-PAY-L3-INT-01-performance` | 1 | `TC-L3-PAY-LIM-01-performance-requestsPerSecond-over-c` |
| `INT-02`, `LIM-02` | `TOB-PAY-INT-02-resilience` | `TCND-PAY-LIM-02-inputValidation-maxPayloadKB-at` | `TR-PAY-L3-INT-02-functional` | 1 | `TC-L3-PAY-LIM-02-inputValidation-maxPayloadKB-at-c` |
| `INT-02`, `LIM-02` | `TOB-PAY-INT-02-resilience` | `TCND-PAY-LIM-02-inputValidation-maxPayloadKB-over` | `TR-PAY-L3-INT-02-functional` | 1 | `TC-L3-PAY-LIM-02-inputValidation-maxPayloadKB-over-c` |
| `INT-02`, `LIM-02` | `TOB-PAY-INT-02-resilience` | `TCND-PAY-LIM-02-performance-requestsPerSecond-at` | `TR-PAY-L3-INT-02-performance` | 1 | `TC-L3-PAY-LIM-02-performance-requestsPerSecond-at-c` |
| `INT-02`, `LIM-02` | `TOB-PAY-INT-02-resilience` | `TCND-PAY-LIM-02-performance-requestsPerSecond-over` | `TR-PAY-L3-INT-02-performance` | 1 | `TC-L3-PAY-LIM-02-performance-requestsPerSecond-over-c` |
| `MECH-01`, `NFR-001` | `TOB-PAY-MECH-01-mechanism` | `TCND-PAY-MECH-01-invariant-holds` | `TR-PAY-L3-MECH-01-functional` | 1 | `TC-L3-PAY-MECH-01-invariant-holds-c` |
| `MOD-Ledger`, `MOD-Payment` | `TOB-PAY-MOD-Payment-deps` | `TCND-PAY-MOD-Payment-errorHandling-dep-MOD-Ledger` | `TR-PAY-L3-MOD-Payment-reliability` | 1 | `TC-L3-PAY-MOD-Payment-errorHandling-dep-MOD-Ledger-c` |
| `NFR-001` | `TOB-PAY-NFR-001-nfr` | `TCND-PAY-NFR-001-performance-target` | `TR-PAY-L2-NFR-001-performance` | 1 | `TC-L2-PAY-NFR-001-performance-target-c` |
| `FB-01`, `INT-01`, `SCN-01` | `TOB-PAY-SCN-01-flow` | `TCND-PAY-SCN-01-errorHandling-fail-INT-01` | `TR-PAY-L1-SCN-01-reliability` | 1 | `TC-L1-PAY-SCN-01-errorHandling-fail-INT-01-c` |
| `MOD-Ledger`, `MOD-Payment`, `SCN-01` | `TOB-PAY-SCN-01-flow` | `TCND-PAY-SCN-01-functional-main` | `TR-PAY-L1-SCN-01-functional` | 1 | `TC-L1-PAY-SCN-01-functional-main-c` |
| `SCN-01` | `TOB-PAY-SCN-01-flow` | `TCND-PAY-SCN-01-timing-budget` | `TR-PAY-L1-SCN-01-performance` | 1 | `TC-L1-PAY-SCN-01-timing-budget-c` |
| `INT-01`, `RT-01`, `SCN-01`, `TO-01` | `TOB-PAY-SCN-01-flow` | `TCND-PAY-SCN-01-timing-worst-INT-01` | `TR-PAY-L1-SCN-01-performance` | 1 | `TC-L1-PAY-SCN-01-timing-worst-INT-01-c` |
| `INT-02`, `SEC-02` | `TOB-PAY-INT-02-resilience` | `TCND-PAY-SEC-02-access-allow` | `TR-PAY-L3-INT-02-security` | 1 | `TC-L3-PAY-SEC-02-access-allow-c` |
| `INT-02`, `SEC-02` | `TOB-PAY-INT-02-resilience` | `TCND-PAY-SEC-02-access-deny` | `TR-PAY-L3-INT-02-security` | 1 | `TC-L3-PAY-SEC-02-access-deny-c` |
| `SM-001`, `TRN-001` | `TOB-PAY-SM-001-states` | `TCND-PAY-TRN-001-guardViolation-guard` | `TR-PAY-L2-SM-001-functional` | 1 | `TC-L2-PAY-TRN-001-guardViolation-guard-c` |
| `SM-001`, `TRN-001` | `TOB-PAY-SM-001-states` | `TCND-PAY-TRN-001-stateTransition` | `TR-PAY-L2-SM-001-functional` | 1 | `TC-L2-PAY-TRN-001-stateTransition-c` |
| `SM-001`, `TRN-002` | `TOB-PAY-SM-001-states` | `TCND-PAY-TRN-002-guardViolation-guard` | `TR-PAY-L2-SM-001-functional` | 1 | `TC-L2-PAY-TRN-002-guardViolation-guard-c` |
| `SM-001`, `TRN-002` | `TOB-PAY-SM-001-states` | `TCND-PAY-TRN-002-stateTransition` | `TR-PAY-L2-SM-001-functional` | 1 | `TC-L2-PAY-TRN-002-stateTransition-c` |
| `FR-001`, `UR-001` | `TOB-PAY-UR-001-trace` | `TCND-PAY-UR-001-functional-trace` | `TR-PAY-L2-UR-001-functional` | 1 | `TC-L2-PAY-UR-001-functional-trace-c` |

## 6. Khoảng trống mở (TestGap)

| Mã | Thiếu | Câu hỏi | Owner | Hạn | Chặn |
| --- | --- | --- | --- | --- | --- |
| `TGAP-PAY-FR-002-no-ac` | FR-002 chưa có tiêu chí chấp nhận | Tiêu chí chấp nhận nào chứng minh FR-002 đạt? | Business Analyst |  | `TOB-PAY-FR-002-func` |
| `TGAP-PAY-INT-01-ASM-01` | ASM-01 chưa xác nhận: Cổng cho phép 50 kết nối | Xác nhận ASM-01: Cổng cho phép 50 kết nối? | QA Lead | 2099-01-01 | `TCND-PAY-INT-01-concurrency-concurrent-dup`, `TCND-PAY-INT-01-idempotency-payload-mismatch`, `TCND-PAY-INT-01-idempotency-replay`, `TCND-PAY-INT-01-timing-worst-INT-01`, `TOB-PAY-INT-01-resilience` |
| `TGAP-PAY-INT-01-content-idempotency-payload-mismatch` | chính sách khi cùng khóa khác payload | Bổ sung vào TDD: chính sách khi cùng khóa khác payload? | QA Lead |  | `TCND-PAY-INT-01-idempotency-payload-mismatch` |
| `TGAP-PAY-LIM-01-content-performance-requestsPerSecond-over` | hành vi khi vượt requestsPerSecond của Cổng thanh toán | Bổ sung vào TDD: hành vi khi vượt requestsPerSecond của Cổng thanh toán? | QA Lead |  | `TCND-PAY-LIM-01-performance-requestsPerSecond-over` |
| `TGAP-PAY-LIM-02-content-inputValidation-maxPayloadKB-over` | hành vi khi vượt maxPayloadKB của API thanh toán | Bổ sung vào TDD: hành vi khi vượt maxPayloadKB của API thanh toán? | QA Lead |  | `TCND-PAY-LIM-02-inputValidation-maxPayloadKB-over` |
| `TGAP-PAY-LIM-02-content-performance-requestsPerSecond-over` | hành vi khi vượt requestsPerSecond của API thanh toán | Bổ sung vào TDD: hành vi khi vượt requestsPerSecond của API thanh toán? | QA Lead |  | `TCND-PAY-LIM-02-performance-requestsPerSecond-over` |
| `TGAP-PAY-MOD-Payment-content-errorHandling-dep-MOD-Ledger` | hành vi của Thanh toán khi Sổ cái lỗi | Bổ sung vào TDD: hành vi của Thanh toán khi Sổ cái lỗi? | QA Lead |  | `TCND-PAY-MOD-Payment-errorHandling-dep-MOD-Ledger` |
| `TGAP-PAY-SEC-02-content-access-deny` | mã lỗi khi sai xác thực API thanh toán | Bổ sung vào TDD: mã lỗi khi sai xác thực API thanh toán? | QA Lead |  | `TCND-PAY-SEC-02-access-deny` |

## 7. Báo cáo kiểm (gate)

Gate GS: không đạt (chặn: TSO-007)

| Luật | Mức | Nội dung | Đã kiểm | Vi phạm |
| --- | --- | --- | --- | --- |
| TSO-001 | block | Mọi nút có basedOn tới nút TDD tồn tại ở baseline | 441 | 0 |
| TSO-002 | block | Nút derivation=llm có model, promptHash, evidence | 0 | 0 |
| TSO-003 | block | Mọi mã tham chiếu thuộc danh sách trắng | 441 | 0 |
| TSO-004 | block | Mọi Requirement trong phạm vi có TestObligation | 4 | 0 |
| TSO-005 | block | Nút TDD có luật seed có đủ TestCondition seed | 45 | 0 |
| TSO-006 | block | Mọi TestCondition có oracle; oracle threshold có measurement | 35 | 0 |
| TSO-007 | blockBaseline | TestGap có owner; điều kiện bị gap chặn không được baseline | 8 | 10 |
| TSO-008 | block | Mọi TestCondition được ít nhất một CoverageItem hiện thực | 35 | 0 |
| TSO-009 | block | TR có passCriteria đánh số trỏ tới điều kiện; scopeNote khi cùng chủ thể có nhiều TR | 16 | 0 |
| TSO-010 | warn | Kỹ thuật phù hợp conditionKind | 35 | 0 |
| TSO-011 | block | Mọi CoverageItem có TestCase phủ; mọi TestCase có bước when và Assertion | 70 | 0 |
| TSO-012 | block | Assertion.expected khớp oracle (đơn vị, ngưỡng, mã lỗi) | 37 | 0 |
| TSO-013 | block | FaultModel có FaultInjection; timing=afterCommit có assertion về hiệu ứng phụ | 8 | 0 |
| TSO-014 | block | Có idempotency thì phủ đủ: gửi lại, khác payload, trùng đồng thời | 1 | 0 |
| TSO-015 | block | Điều kiện timing dùng trường hợp xấu nhất, không cộng p95 thành phần | 2 | 0 |
| TSO-016 | block | Điều kiện performance có workload; không có test vượt ngưỡng + 1 cho chỉ tiêu 0 hoặc 100 % | 5 | 0 |
| TSO-017 | warn | Mỗi điểm thực thi quyền có test cho phép và từ chối | 1 | 0 |
| TSO-018 | block | Không có giá trị bí mật trong dữ liệu, bước, assertion | 201 | 0 |
| TSO-019 | block | TR mức L1 đi qua ScenarioPath có ít nhất hai Module | 3 | 0 |
| TSO-020 | blockBaseline | Nút llm chỉ baseline khi đã duyệt, trừ loại nút và ngưỡng policy cho phép | 0 | 0 |
| TSO-021 | block | Phiên bản hóa: Baselined bất biến; supersedes thẳng; Retired có retiredBy | 1 | 0 |

## 8. Lịch sử phiên bản và phạm vi hồi quy

| Phiên bản | Trạng thái | CR | Thêm | Sửa | Bỏ | Số nút |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | Draft | — | 441 | 0 | 0 | 441 |

Hồi quy (0 test case đã baseline, thuộc phần bị ảnh hưởng nhưng không đổi): không có

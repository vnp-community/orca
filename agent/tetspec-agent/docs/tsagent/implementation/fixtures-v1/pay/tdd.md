---
doc_id: TDD-PAY
doc_type: tdd
title: TDD thanh toán
version: "1.0"
status: active
---

# TDD thanh toán

## 3. Module

| Mã | Tên | Trách nhiệm | layer | Phụ thuộc | ownsData |
| --- | --- | --- | --- | --- | --- |
| MOD-Payment | Thanh toán | Xử lý thanh toán qua cổng | domain | MOD-Ledger | DE-ORDER |
| MOD-Ledger | Sổ cái | Ghi sổ và truy vấn lịch sử | domain | | |

## 5. Dữ liệu

| Mã | name | description |
| --- | --- | --- |
| DE-ORDER | Order | Đơn hàng |

## 6. Giao tiếp bên ngoài

| Mã | Tên | integrationType | direction | interactionStyle | protocol | idempotent | Module tích hợp | assumes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| INT-01 | Cổng thanh toán | ThirdParty | outbound | sync | HTTPS | true | MOD-Payment | ASM-01 |
| INT-02 | API thanh toán | InternalService | inbound | sync | HTTPS | false | MOD-Payment | |

### 6.1 INT-01: Cổng thanh toán

| Mã | Lớp | Thuộc tính |
| --- | --- | --- |
| PARTY-GW | ExternalParty | partyType=ThirdParty; contractConfirmed=false |
| TO-01 | TimeoutPolicy | connectMs=2000; readMs=10000; totalMs=25000; poolAcquireMs=3000 |
| RT-01 | RetryPolicy | maxAttempts=3; backoffType=exponential; jitter=true; idempotentOnly=true |
| POOL-01 | ConnectionPool | maxTotal=50; maxPerRoute=50; idleTimeoutSec=30 |
| CB-01 | CircuitBreaker | failureRatePercent=50; minimumCalls=10; openDurationSec=60 |
| SEC-01 | SecurityScheme | schemeType=APIKey; secretStore=biến môi trường GATEWAY_API_KEY |
| LIM-01 | PartnerLimit | requestsPerSecond=50; maxConcurrentConnections=50 |
| FB-01 | FallbackStrategy | mode=degraded |
| MON-01 | MonitoringSpec | metrics=latency |

| Mã | trigger | errorCategory | retryable | action |
| --- | --- | --- | --- | --- |
| ERR-01-429 | HTTP 429 | transient | true | chờ Retry-After rồi thử lại |
| ERR-01-5xx | HTTP 503 | transient | true | backoff rồi thử lại |
| ERR-01-4xx | HTTP 400 | permanent | false | trả lỗi cho người dùng |
| ERR-01-timeout | readMs hết hạn | unknownOutcome | true | tra cứu trạng thái giao dịch |

| Mã | direction | Schema |
| --- | --- | --- |
| MSG-01 | request | SCH-01 |

| Mã | format | location |
| --- | --- | --- |
| SCH-01 | JSONSchema | schemas/pay.json |

### 6.2 INT-02: API thanh toán

| Mã | Lớp | Thuộc tính |
| --- | --- | --- |
| PARTY-CONS | ExternalParty | partyType=InternalConsumer; contractConfirmed=true |
| SEC-02 | SecurityScheme | schemeType=Bearer; secretStore=biến môi trường PAY_TOKEN |
| LIM-02 | PartnerLimit | requestsPerSecond=20; maxPayloadKB=64 |

| Mã | trigger | errorCategory | retryable | action |
| --- | --- | --- | --- | --- |
| ERR-02-busy | HTTP 503 | transient | true | client thử lại có backoff |
| ERR-02-bad | HTTP 401 | permanent | false | client sửa thông tin xác thực |
| ERR-02-lost | client hết thời gian chờ | unknownOutcome | true | tra cứu trạng thái |

| Mã | direction | Schema |
| --- | --- | --- |
| MSG-02 | request | SCH-02 |

| Mã | format | location |
| --- | --- | --- |
| SCH-02 | JSONSchema | schemas/api.json |

| Mã | name | required |
| --- | --- | --- |
| HDR-02 | Idempotency-Key | true |

## 8. Luồng xử lý

### 8.1 SCN-01: Thanh toán đơn hàng

`responseBudgetMs`=30000; `participatesIn`=MOD-Payment, MOD-Ledger; `invokes`=INT-01

| # | Bước |
| --- | --- |
| 1 | Khách gửi yêu cầu thanh toán |
| 2 | MOD-Payment gọi cổng thanh toán |
| 3 | MOD-Ledger ghi sổ |

## 10. Môi trường

| Mã | name | hostingType |
| --- | --- | --- |
| ENV-test | test | container |
| ENV-staging | staging | container |

## 11. Rủi ro và giả định

| Mã | Mô tả | likelihood | impact | mitigation | Gắn vào |
| --- | --- | --- | --- | --- | --- |
| RISK-01 | Cổng thanh toán chậm | Medium | High | timeout và retry | INT-01 |

| Mã | Giả định | owner | dueDate | confirmStatus |
| --- | --- | --- | --- | --- |
| ASM-01 | Cổng cho phép 50 kết nối | Tech Lead | 2099-01-01 | Unconfirmed |

## 9. Cơ chế

| Mã | mechanismKind | Mô tả | Đáp ứng |
| --- | --- | --- | --- |
| MECH-01 | cache | Cache kết quả tra cứu | NFR-001 |

## 12. Test

| Mã | level | Nội dung |
| --- | --- | --- |
| TST-01 | system | Đo p95 thanh toán |

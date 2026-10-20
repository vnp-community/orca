# TDD MiniPay (sinh từ knowledge graph)

Tài liệu này được render tự động từ `tdd_kg.db` (KG nền + các patch CR đã duyệt). Nguồn sự thật là KG; không sửa tay tệp này, hãy chạy lại `python -m tdd_kg.render_tdd`.

## Yêu cầu chức năng (11)

### FR-001 - Tạo tài khoản ví, trả mã 10 chữ số và API key
Lớp: FunctionalRequirement | trạng thái: Baselined v0.1 | nguồn: SRS Yêu cầu chức năng

Yêu cầu: Tạo tài khoản ví, trả mã 10 chữ số và API key (ưu tiên Must)

#### Đáp ứng bởi
- MOD-Account: Quản lý tài khoản

#### Module
- MOD-Account: Quản lý tài khoản

#### Mã nguồn (LLD)
- DI-Account: minipay/service.py::create_account

#### Kiểm chứng bởi
- TC-001 [integration]: tests/test_minipay.py::test_create_account_and_auth

### FR-002 - Xác thực mọi API (trừ tạo tài khoản) bằng API key; chỉ thao tác trên tài khoản của mình
Lớp: FunctionalRequirement | trạng thái: Baselined v0.1 | nguồn: SRS Yêu cầu chức năng

Yêu cầu: Xác thực mọi API (trừ tạo tài khoản) bằng API key; chỉ thao tác trên tài khoản của mình (ưu tiên Must)

#### Đáp ứng bởi
- MECH-APIKeyAuth: API key dạng băm SHA-256, Bearer token

#### Module
- MOD-API: API Layer

#### Mã nguồn (LLD)
- DI-API: minipay/api.py::create_app

#### Kiểm chứng bởi
- TC-001 [integration]: tests/test_minipay.py::test_create_account_and_auth

### FR-003 - Nạp tiền (giả lập) vào tài khoản của mình
Lớp: FunctionalRequirement | trạng thái: Baselined v0.1 | nguồn: SRS Yêu cầu chức năng

Yêu cầu: Nạp tiền (giả lập) vào tài khoản của mình (ưu tiên Must)

#### Đáp ứng bởi
- MOD-Ledger: Sổ cái

#### Module
- MOD-Transfer: Chuyển tiền
- MOD-Ledger: Sổ cái

#### Mã nguồn (LLD)
- DI-Transfer: minipay/service.py::transfer
- DI-Ledger: minipay/service.py::balance_of

#### Kiểm chứng bởi
- TC-002 [integration]: tests/test_minipay.py::test_transfer_ok_and_ledger_invariant

#### Rủi ro
- RISK-002: Số dư âm do hai giao dịch chạy đồng thời

### FR-004 - Tra cứu số dư và thông tin tài khoản
Lớp: FunctionalRequirement | trạng thái: Baselined v0.1 | nguồn: SRS Yêu cầu chức năng

Yêu cầu: Tra cứu số dư và thông tin tài khoản (ưu tiên Must)

#### Đáp ứng bởi
- MOD-Account: Quản lý tài khoản

#### Module
- MOD-Account: Quản lý tài khoản

#### Mã nguồn (LLD)
- DI-Account: minipay/service.py::create_account

#### Kiểm chứng bởi
- TC-001 [integration]: tests/test_minipay.py::test_create_account_and_auth

### FR-010 - Chuyển tiền giữa hai tài khoản ACTIVE trong cùng một giao dịch CSDL
Lớp: FunctionalRequirement | trạng thái: Baselined v0.1 | nguồn: SRS Yêu cầu chức năng

Yêu cầu: Chuyển tiền giữa hai tài khoản ACTIVE trong cùng một giao dịch CSDL (ưu tiên Must)

#### Đáp ứng bởi
- AD-03: Tiền là số nguyên đơn vị đồng
- MOD-Ledger: Sổ cái
- MECH-WriteLock: BEGIN IMMEDIATE và kiểm tra lại số dư/hạn mức trong giao dịch

#### Module
- MOD-Transfer: Chuyển tiền
- MOD-Ledger: Sổ cái

#### Tích hợp ngoài liên quan
- INT-01: Fraud Check

#### Mã nguồn (LLD)
- DI-Transfer: minipay/service.py::transfer
- DI-Ledger: minipay/service.py::balance_of
- DI-FraudClient-INT01: minipay/fraud_client.py::check

#### Kiểm chứng bởi
- TC-002 [integration]: tests/test_minipay.py::test_transfer_ok_and_ledger_invariant

#### Rủi ro
- RISK-002: Số dư âm do hai giao dịch chạy đồng thời
- RISK-001: Fraud Check chậm hoặc không sẵn sàng

### FR-011 - Kiểm tra hạn mức 50.000.000 VND/giao dịch và 200.000.000 VND/ngày
Lớp: FunctionalRequirement | trạng thái: Baselined v0.1 | nguồn: SRS Yêu cầu chức năng

Yêu cầu: Kiểm tra hạn mức 50.000.000 VND/giao dịch và 200.000.000 VND/ngày (ưu tiên Must)

#### Đáp ứng bởi
- MECH-WriteLock: BEGIN IMMEDIATE và kiểm tra lại số dư/hạn mức trong giao dịch

#### Module
- MOD-Transfer: Chuyển tiền

#### Tích hợp ngoài liên quan
- INT-01: Fraud Check

#### Mã nguồn (LLD)
- DI-Transfer: minipay/service.py::transfer
- DI-FraudClient-INT01: minipay/fraud_client.py::check

#### Kiểm chứng bởi
- TC-003 [integration]: tests/test_minipay.py::test_rejections
- TC-004 [integration]: tests/test_minipay.py::test_daily_limit

#### Rủi ro
- RISK-001: Fraud Check chậm hoặc không sẵn sàng

### FR-012 - Chống trùng giao dịch bằng Idempotency-Key
Lớp: FunctionalRequirement | trạng thái: Baselined v0.1 | nguồn: SRS Yêu cầu chức năng

Yêu cầu: Chống trùng giao dịch bằng Idempotency-Key (ưu tiên Must)

#### Đáp ứng bởi
- MECH-Idempotency: Idempotency-Key + request hash

#### Module
- MOD-Transfer: Chuyển tiền

#### Tích hợp ngoài liên quan
- INT-01: Fraud Check

#### Mã nguồn (LLD)
- DI-Transfer: minipay/service.py::transfer
- DI-FraudClient-INT01: minipay/fraud_client.py::check

#### Kiểm chứng bởi
- TC-005 [integration]: tests/test_minipay.py::test_idempotency_replay_and_conflict

#### Rủi ro
- RISK-001: Fraud Check chậm hoặc không sẵn sàng

### FR-013 - Tra cứu giao dịch và lịch sử giao dịch của một tài khoản
Lớp: FunctionalRequirement | trạng thái: Baselined v0.1 | nguồn: SRS Yêu cầu chức năng

Yêu cầu: Tra cứu giao dịch và lịch sử giao dịch của một tài khoản (ưu tiên Should)

#### Đáp ứng bởi
- MOD-Transfer: Chuyển tiền

#### Module
- MOD-Transfer: Chuyển tiền

#### Mã nguồn (LLD)
- DI-Transfer: minipay/service.py::transfer

#### Kiểm chứng bởi
- TC-015 [integration]: tests/test_minipay.py::test_history_and_get_transfer

### FR-020 - Kiểm tra rủi ro qua dịch vụ Fraud Check trước khi ghi nợ
Lớp: FunctionalRequirement | trạng thái: Baselined v0.1 | nguồn: SRS Yêu cầu chức năng

Yêu cầu: Kiểm tra rủi ro qua dịch vụ Fraud Check trước khi ghi nợ (ưu tiên Must)

#### Đáp ứng bởi
- AD-04: Fraud Check lỗi: cho qua giao dịch nhỏ, từ chối giao dịch lớn
- MECH-CircuitBreaker: Circuit breaker ở Risk adapter

#### Module
- MOD-Risk: Risk adapter

#### Tích hợp ngoài liên quan
- INT-01: Fraud Check

#### Mã nguồn (LLD)
- DI-FraudClient: minipay/fraud_client.py::FraudClient
- DI-FraudClient-INT01: minipay/fraud_client.py::check

#### Kiểm chứng bởi
- TC-006 [integration]: tests/test_minipay.py::test_fraud_reject
- TC-007 [integration]: tests/test_minipay.py::test_fraud_down_policy

#### Rủi ro
- RISK-001: Fraud Check chậm hoặc không sẵn sàng

### FR-021 - Phát sự kiện TransferCompleted để gửi SMS thông báo
Lớp: FunctionalRequirement | trạng thái: Baselined v0.1 | nguồn: SRS Yêu cầu chức năng

Yêu cầu: Phát sự kiện TransferCompleted để gửi SMS thông báo (ưu tiên Should)

#### Đáp ứng bởi
- AD-02: Phát sự kiện qua outbox
- MECH-Outbox: Bảng outbox ghi cùng giao dịch CSDL, relay lên queue
- MECH-IdempotentConsumer: Consumer khử trùng theo eventId (processed_events)

#### Module
- MOD-Notify: Thông báo

#### Tích hợp ngoài liên quan
- INT-02: Sự kiện giao dịch (message queue)
- INT-03: SMS Gateway

#### Mã nguồn (LLD)
- DI-Notify: minipay/notify.py::consume
- DI-Outbox: minipay/notify.py::relay_outbox
- DI-Consumer: minipay/notify.py::consume

#### Kiểm chứng bởi
- TC-008 [integration]: tests/test_minipay.py::test_outbox_sms_and_dedupe
- TC-009 [integration]: tests/test_minipay.py::test_sms_failure_does_not_change_transfer

#### Rủi ro
- RISK-003: Sự kiện thông báo bị gửi trùng

### FR-030 - Ghi audit cho thao tác thay đổi dữ liệu và giao dịch bị từ chối
Lớp: FunctionalRequirement | trạng thái: Baselined v0.1 | nguồn: SRS Yêu cầu chức năng

Yêu cầu: Ghi audit cho thao tác thay đổi dữ liệu và giao dịch bị từ chối (ưu tiên Must)

#### Đáp ứng bởi
- MOD-Audit: Audit

#### Module
- MOD-Audit: Audit

#### Mã nguồn (LLD)
- DI-Audit: minipay/service.py::audit

#### Kiểm chứng bởi
- TC-010 [integration]: tests/test_minipay.py::test_audit_written

## Yêu cầu phi chức năng (7)

### NFR-001 - Hiệu năng chuyển tiền: p95 <= 500 ms ở mức 50 giao dịch/giây
Lớp: NonFunctionalRequirement | trạng thái: Baselined v0.1 | nguồn: SRS Yêu cầu phi chức năng

Yêu cầu: Hiệu năng chuyển tiền: p95 <= 500 ms ở mức 50 giao dịch/giây (ưu tiên Must)
Mục tiêu: 500 ms

#### Đáp ứng bởi
- MOD-Transfer: Chuyển tiền
- MECH-TimeoutBudget: Ngân sách timeout chia từ ngoài vào trong

#### Module
- MOD-Transfer: Chuyển tiền

#### Tích hợp ngoài liên quan
- INT-01: Fraud Check

#### Mã nguồn (LLD)
- DI-Transfer: minipay/service.py::transfer
- DI-FraudClient-INT01: minipay/fraud_client.py::check

#### Kiểm chứng bởi
- TC-BENCH [system]: scripts/benchmark.py

#### Rủi ro
- RISK-001: Fraud Check chậm hoặc không sẵn sàng

### NFR-002 - Toàn vẹn dữ liệu: số dư >= 0 mọi lúc; tổng số dư không đổi sau mọi giao dịch chuyển tiền
Lớp: NonFunctionalRequirement | trạng thái: Baselined v0.1 | nguồn: SRS Yêu cầu phi chức năng

Yêu cầu: Toàn vẹn dữ liệu: số dư >= 0 mọi lúc; tổng số dư không đổi sau mọi giao dịch chuyển tiền (ưu tiên Must)
Mục tiêu: 0 VND (sai lệch tổng)

#### Đáp ứng bởi
- AD-01: Modular monolith thay vì nhiều service
- MECH-AppendOnlyLedger: Sổ cái chỉ thêm, số dư = balance_after của bút toán cuối
- MECH-WriteLock: BEGIN IMMEDIATE và kiểm tra lại số dư/hạn mức trong giao dịch

#### Kiểm chứng bởi
- TC-002 [integration]: tests/test_minipay.py::test_transfer_ok_and_ledger_invariant

### NFR-003 - Bảo mật: API key lưu dạng băm SHA-256; log che số tài khoản
Lớp: NonFunctionalRequirement | trạng thái: Baselined v0.1 | nguồn: SRS Yêu cầu phi chức năng

Yêu cầu: Bảo mật: API key lưu dạng băm SHA-256; log che số tài khoản (ưu tiên Must)
Mục tiêu: 4 chữ số cuối được giữ trong log

#### Đáp ứng bởi
- MECH-APIKeyAuth: API key dạng băm SHA-256, Bearer token

#### Kiểm chứng bởi
- TC-001 [integration]: tests/test_minipay.py::test_create_account_and_auth

### NFR-004 - Sẵn sàng 99,5% mỗi tháng cho API chuyển tiền
Lớp: NonFunctionalRequirement | trạng thái: Baselined v0.1 | nguồn: SRS Yêu cầu phi chức năng

Yêu cầu: Sẵn sàng 99,5% mỗi tháng cho API chuyển tiền (ưu tiên Must)
Mục tiêu: 99.5 %

#### Đáp ứng bởi
- MECH-HealthCheck: Endpoint /health cho load balancer

### NFR-005 - Truy vết: 100% yêu cầu có correlationId, xuất hiện trong log và audit
Lớp: NonFunctionalRequirement | trạng thái: Baselined v0.1 | nguồn: SRS Yêu cầu phi chức năng

Yêu cầu: Truy vết: 100% yêu cầu có correlationId, xuất hiện trong log và audit (ưu tiên Must)
Mục tiêu: 100 %

#### Đáp ứng bởi
- MECH-Correlation: X-Correlation-Id sinh ở API, ghi vào giao dịch, audit, lời gọi ngoài
- MOD-Audit: Audit

#### Module
- MOD-Audit: Audit

#### Mã nguồn (LLD)
- DI-Audit: minipay/service.py::audit

#### Kiểm chứng bởi
- TC-002 [integration]: tests/test_minipay.py::test_transfer_ok_and_ledger_invariant
- TC-010 [integration]: tests/test_minipay.py::test_audit_written

### NFR-006 - Thời gian phản hồi tối đa 3 giây kể cả khi dịch vụ ngoài chậm
Lớp: NonFunctionalRequirement | trạng thái: Baselined v0.1 | nguồn: SRS Yêu cầu phi chức năng

Yêu cầu: Thời gian phản hồi tối đa 3 giây kể cả khi dịch vụ ngoài chậm (ưu tiên Must)
Mục tiêu: 3000 ms

#### Đáp ứng bởi
- MECH-TimeoutBudget: Ngân sách timeout chia từ ngoài vào trong
- MECH-CircuitBreaker: Circuit breaker ở Risk adapter
- MOD-Risk: Risk adapter

#### Module
- MOD-Risk: Risk adapter

#### Tích hợp ngoài liên quan
- INT-01: Fraud Check

#### Mã nguồn (LLD)
- DI-FraudClient: minipay/fraud_client.py::FraudClient
- DI-FraudClient-INT01: minipay/fraud_client.py::check

#### Kiểm chứng bởi
- TC-007 [integration]: tests/test_minipay.py::test_fraud_down_policy
- TC-011 [contract]: tests/test_minipay.py::test_fraud_client_contract_headers_payload
- TC-012 [contract]: tests/test_minipay.py::test_fraud_client_retry_once_on_5xx
- TC-014 [contract]: tests/test_minipay.py::test_fraud_client_circuit_breaker_opens

#### Rủi ro
- RISK-001: Fraud Check chậm hoặc không sẵn sàng

### NFR-007 - Khả năng kiểm thử: mọi tích hợp ngoài có mock và contract test
Lớp: NonFunctionalRequirement | trạng thái: Baselined v0.1 | nguồn: SRS Yêu cầu phi chức năng

Yêu cầu: Khả năng kiểm thử: mọi tích hợp ngoài có mock và contract test (ưu tiên Must)
Mục tiêu: 100 %

#### Đáp ứng bởi
- MOD-Risk: Risk adapter

#### Module
- MOD-Risk: Risk adapter

#### Tích hợp ngoài liên quan
- INT-01: Fraud Check

#### Mã nguồn (LLD)
- DI-FraudClient: minipay/fraud_client.py::FraudClient
- DI-FraudClient-INT01: minipay/fraud_client.py::check

#### Kiểm chứng bởi
- TC-011 [contract]: tests/test_minipay.py::test_fraud_client_contract_headers_payload
- TC-012 [contract]: tests/test_minipay.py::test_fraud_client_retry_once_on_5xx
- TC-013 [contract]: tests/test_minipay.py::test_fraud_client_no_retry_on_4xx_and_bad_schema
- TC-014 [contract]: tests/test_minipay.py::test_fraud_client_circuit_breaker_opens

#### Rủi ro
- RISK-001: Fraud Check chậm hoặc không sẵn sàng

## Module (7)

### MOD-API - API Layer
Lớp: Module | trạng thái: InReview v0.1 | nguồn: SAD/HLD Phân rã module

Trách nhiệm: Nhận REST, xác thực API key, kiểm tra đầu vào, đọc Idempotency-Key và X-Correlation-Id, đổi lỗi nghiệp vụ thành mã HTTP

#### Yêu cầu
- FR-002: Xác thực mọi API (trừ tạo tài khoản) bằng API key; chỉ thao tác trên tài khoản của mình

#### Phụ thuộc
- MOD-Account: Quản lý tài khoản
- MOD-Transfer: Chuyển tiền

#### Công nghệ
- TECH-FastAPI: FastAPI
- TECH-Uvicorn: Uvicorn
- TECH-Python: Python

#### Mã nguồn (LLD)
- DI-API: minipay/api.py::create_app

### MOD-Account - Quản lý tài khoản
Lớp: Module | trạng thái: InReview v0.1 | nguồn: SAD/HLD Phân rã module

Trách nhiệm: Tạo tài khoản, cấp và băm API key, tra cứu tài khoản

#### Yêu cầu
- FR-001: Tạo tài khoản ví, trả mã 10 chữ số và API key
- FR-004: Tra cứu số dư và thông tin tài khoản
- FR-001: Tạo tài khoản ví, trả mã 10 chữ số và API key
- FR-004: Tra cứu số dư và thông tin tài khoản

#### Phụ thuộc
- MOD-Audit: Audit

#### Công nghệ
- TECH-SQLite: SQLite (WAL)

#### Dữ liệu sở hữu
- DE-Account: Account

#### Mã nguồn (LLD)
- DI-Account: minipay/service.py::create_account

### MOD-Audit - Audit
Lớp: Module | trạng thái: InReview v0.1 | nguồn: SAD/HLD Phân rã module

Trách nhiệm: Ghi audit cho thao tác thay đổi dữ liệu và giao dịch bị từ chối

#### Yêu cầu
- FR-030: Ghi audit cho thao tác thay đổi dữ liệu và giao dịch bị từ chối
- FR-030: Ghi audit cho thao tác thay đổi dữ liệu và giao dịch bị từ chối
- NFR-005: Truy vết: 100% yêu cầu có correlationId, xuất hiện trong log và audit

#### Công nghệ
- TECH-SQLite: SQLite (WAL)

#### Dữ liệu sở hữu
- DE-AuditLog: AuditLog

#### Mã nguồn (LLD)
- DI-Audit: minipay/service.py::audit

### MOD-Ledger - Sổ cái
Lớp: Module | trạng thái: InReview v0.1 | nguồn: SAD/HLD Phân rã module

Trách nhiệm: Ghi bút toán ghi nợ và ghi có, giữ số dư không âm, bảo toàn tổng số dư

#### Yêu cầu
- FR-010: Chuyển tiền giữa hai tài khoản ACTIVE trong cùng một giao dịch CSDL
- FR-003: Nạp tiền (giả lập) vào tài khoản của mình

#### Công nghệ
- TECH-SQLite: SQLite (WAL)

#### Dữ liệu sở hữu
- DE-LedgerEntry: LedgerEntry

#### Mã nguồn (LLD)
- DI-Ledger: minipay/service.py::balance_of

#### Rủi ro
- RISK-002: Số dư âm do hai giao dịch chạy đồng thời

### MOD-Notify - Thông báo
Lớp: Module | trạng thái: InReview v0.1 | nguồn: SAD/HLD Phân rã module

Trách nhiệm: Đưa sự kiện từ outbox lên message queue; nhận sự kiện và gửi SMS

#### Yêu cầu
- FR-021: Phát sự kiện TransferCompleted để gửi SMS thông báo

#### Phụ thuộc
- MOD-Transfer: Chuyển tiền

#### Công nghệ
- TECH-SQLite: SQLite (WAL)
- TECH-Python: Python

#### Tích hợp ngoài
- INT-02: Sự kiện giao dịch (message queue)
- INT-03: SMS Gateway

#### Mã nguồn (LLD)
- DI-Notify: minipay/notify.py::consume

#### Contract INT-02 (Sự kiện giao dịch (message queue))
- Kiểu: MessageQueue, hướng outbound, async, idempotent=True, giao thức queue
- Đối tác: Message broker nền tảng (contractConfirmed=True, sandbox=True)
- Channel: Topic minipay.transfer.events.v1 key=fromAccountId partitions=3 atLeastOnce dlq=minipay.transfer.events.v1.dlq consumerIdempotent=True
  - message: {"eventId":"TRF-AB12","transferId":"TRF-AB12","fromAccountId":"..","toAccountId":"..","amount":1000,"correlationId":".."} | schema: contracts/transfer-completed.event.schema.json
- Timeout (ms): connect=500 read=1000 total=2000 poolAcquire=200
- Retry: max=3 backoff=exponential idempotentOnly=True
- Pool: maxTotal=5 maxPerRoute=5 idleTimeout=60s
- Bảo mật: mTLS, secret: secret manager, xoay 365 ngày
- Giới hạn đối tác: 1000 rps, 100 kết nối
- Lỗi [transient] Broker không khả dụng / 5xx -> retryable=True: Giữ sự kiện PENDING trong outbox, retry theo backoff
- Lỗi [permanent] Sự kiện sai schema hoặc quá maxMessageKB -> retryable=False: Không retry; chuyển DLQ
- Lỗi [unknownOutcome] Timeout khi publish -> retryable=True: Retry; có thể trùng sự kiện, consumer khử trùng theo eventId
- LLD/code: minipay/notify.py::relay_outbox

#### Contract INT-03 (SMS Gateway)
- Kiểu: ThirdParty, hướng outbound, async, idempotent=False, giao thức REST/JSON
- Đối tác: SMS Gateway (contractConfirmed=False, sandbox=True)
- Endpoint: POST /v1/sms; header: Idempotency-Key*, Authorization*
  - request: {"to":"0912345678","content":"MiniPay: +1,000 VND","messageId":"TRF-AB12"} | schema: contracts/sms.request.schema.json
- Timeout (ms): connect=500 read=2000 total=3000 poolAcquire=300
- Retry: max=3 backoff=exponential idempotentOnly=False
- Pool: maxTotal=5 maxPerRoute=5 idleTimeout=30s
- Bảo mật: APIKey, secret: secret manager, xoay 90 ngày
- Giới hạn đối tác: 20 rps, 10 kết nối
- Dự phòng: degraded - Sau 3 lần thất bại ghi nhận và bỏ qua; không đổi kết quả chuyển tiền
- Lỗi [transient] HTTP 5xx -> retryable=True: Retry tối đa 3 lần
- Lỗi [permanent] HTTP 4xx (số nhận không hợp lệ) -> retryable=False: Không retry; ghi nhận và bỏ qua
- Lỗi [unknownOutcome] Timeout -> retryable=True: Retry với cùng Idempotency-Key; gateway khử trùng
- LLD/code: minipay/notify.py::consume
- Chưa chốt: ASM-002 SMS Gateway có môi trường sandbox để chạy test (hạn 2026-10-21, QA Lead)

### MOD-Risk - Risk adapter
Lớp: Module | trạng thái: InReview v0.1 | nguồn: SAD/HLD Phân rã module

Trách nhiệm: Gọi Fraud Check và áp chính sách khi dịch vụ lỗi hoặc quá thời gian

#### Yêu cầu
- FR-020: Kiểm tra rủi ro qua dịch vụ Fraud Check trước khi ghi nợ
- NFR-006: Thời gian phản hồi tối đa 3 giây kể cả khi dịch vụ ngoài chậm
- NFR-007: Khả năng kiểm thử: mọi tích hợp ngoài có mock và contract test

#### Công nghệ
- TECH-httpx: httpx

#### Tích hợp ngoài
- INT-01: Fraud Check

#### Mã nguồn (LLD)
- DI-FraudClient: minipay/fraud_client.py::FraudClient

#### Contract INT-01 (Fraud Check)
- Kiểu: ThirdParty, hướng outbound, sync, idempotent=True, giao thức REST/JSON
- Đối tác: Dịch vụ Fraud Check (contractConfirmed=False, sandbox=True)
- Endpoint: POST /v1/fraud/checks; header: X-Correlation-Id*, Authorization*
  - request: {"transactionId":"TRF-AB12","fromAccount":"0123456789","toAccount":"9876543210","amount":1000000,"currency":"VND"} | schema: contracts/fraud-check.request.schema.json
  - response: {"decision":"APPROVE","riskScore":10,"correlationId":"..."} | schema: contracts/fraud-check.response.schema.json
- Timeout (ms): connect=300 read=800 total=1000 poolAcquire=200
- Retry: max=2 backoff=none idempotentOnly=True
- Circuit breaker: 100% / tối thiểu 5 lần, mở 30s
- Pool: maxTotal=20 maxPerRoute=20 idleTimeout=30s
- Bảo mật: APIKey, secret: biến môi trường (demo); secret manager khi triển khai thật, xoay 90 ngày
- Giới hạn đối tác: 50 rps, 50 kết nối
- Dự phòng: degraded - Số tiền < 5.000.000 VND: cho qua, riskCheck=SKIPPED. Từ 5.000.000 VND: từ chối FRAUD_CHECK_UNAVAILABLE (503)
- Lỗi [permanent] HTTP 4xx (400, 401, 422) -> retryable=False: Không retry; coi là Fraud không khả dụng, áp chính sách FB-INT01; ghi cảnh báo vi phạm contract
- Lỗi [transient] HTTP 5xx -> retryable=True: Retry 1 lần; vẫn lỗi thì coi là không khả dụng, áp FB-INT01; tính vào circuit breaker
- Lỗi [unknownOutcome] Timeout (connect/read) hoặc lỗi mạng -> retryable=True: Retry 1 lần với cùng transactionId (idempotent); vẫn lỗi thì áp FB-INT01
- Lỗi [permanent] HTTP 200 nhưng response sai schema -> retryable=False: Không retry; coi là không khả dụng; ghi cảnh báo vi phạm contract
- LLD/code: minipay/fraud_client.py::check
- Chưa chốt: ASM-001 Dịch vụ Fraud Check chịu được 50 request mỗi giây; đối tác chưa xác nhận giới hạn này (hạn 2026-10-21, Tech Lead)
- Chưa chốt: OQ-001 Hạn mức mỗi giao dịch và mỗi ngày cuối cùng do bộ phận nào phê duyệt (hạn 2026-10-28, Business Analyst)

### MOD-Transfer - Chuyển tiền
Lớp: Module | trạng thái: InReview v0.1 | nguồn: SAD/HLD Phân rã module

Trách nhiệm: Điều phối luồng chuyển tiền: idempotency, hạn mức, gọi Risk, ghi sổ, ghi outbox; xử lý nạp tiền và tra cứu giao dịch

#### Yêu cầu
- FR-003: Nạp tiền (giả lập) vào tài khoản của mình
- FR-010: Chuyển tiền giữa hai tài khoản ACTIVE trong cùng một giao dịch CSDL
- FR-011: Kiểm tra hạn mức 50.000.000 VND/giao dịch và 200.000.000 VND/ngày
- FR-012: Chống trùng giao dịch bằng Idempotency-Key
- FR-013: Tra cứu giao dịch và lịch sử giao dịch của một tài khoản
- FR-013: Tra cứu giao dịch và lịch sử giao dịch của một tài khoản
- NFR-001: Hiệu năng chuyển tiền: p95 <= 500 ms ở mức 50 giao dịch/giây

#### Phụ thuộc
- MOD-Ledger: Sổ cái
- MOD-Risk: Risk adapter
- MOD-Audit: Audit
- MOD-Account: Quản lý tài khoản

#### Công nghệ
- TECH-SQLite: SQLite (WAL)

#### Dữ liệu sở hữu
- DE-Transfer: Transfer
- DE-OutboxEvent: OutboxEvent

#### Mã nguồn (LLD)
- DI-Transfer: minipay/service.py::transfer

## Quyết định kỹ thuật (7)

### AD-01 - Modular monolith thay vì nhiều service
Lớp: Decision | trạng thái: InReview v0.1 | nguồn: SAD/HLD Quyết định
- AD-01 -[describedIn]-> DOC-HLD: SAD/HLD MiniPay
- AD-01 -[rejects]-> ALT-Microservices: Tách nhiều service
- NFR-002: Toàn vẹn dữ liệu: số dư >= 0 mọi lúc; tổng số dư không đổi sau mọi giao dịch chuyển tiền -[satisfiedBy]-> AD-01

```json
{
 "statement": "Modular monolith thay vì nhiều service",
 "rationale": "Quy mô nhỏ, giữ giao dịch CSDL đơn giản"
}
```

### AD-02 - Phát sự kiện qua outbox
Lớp: Decision | trạng thái: InReview v0.1 | nguồn: SAD/HLD Quyết định
- AD-02 -[describedIn]-> DOC-HLD: SAD/HLD MiniPay
- AD-02 -[rejects]-> ALT-DirectPublish: Phát sự kiện trực tiếp trong luồng nghiệp vụ
- FR-021: Phát sự kiện TransferCompleted để gửi SMS thông báo -[satisfiedBy]-> AD-02

```json
{
 "statement": "Phát sự kiện qua outbox",
 "rationale": "Sự kiện và nghiệp vụ cùng một giao dịch CSDL, không mất sự kiện"
}
```

### AD-03 - Tiền là số nguyên đơn vị đồng
Lớp: Decision | trạng thái: InReview v0.1 | nguồn: SAD/HLD Quyết định
- AD-03 -[describedIn]-> DOC-HLD: SAD/HLD MiniPay
- AD-03 -[rejects]-> ALT-Float: Dùng số thực cho tiền
- FR-010: Chuyển tiền giữa hai tài khoản ACTIVE trong cùng một giao dịch CSDL -[satisfiedBy]-> AD-03

```json
{
 "statement": "Tiền là số nguyên đơn vị đồng",
 "rationale": "Tránh sai số số thực"
}
```

### AD-04 - Fraud Check lỗi: cho qua giao dịch nhỏ, từ chối giao dịch lớn
Lớp: Decision | trạng thái: InReview v0.1 | nguồn: SAD/HLD Quyết định
- AD-04 -[assumes]-> ASM-001: Dịch vụ Fraud Check chịu được 50 request mỗi giây; đối tác chưa xác nhận giới hạn này
- AD-04 -[describedIn]-> DOC-HLD: SAD/HLD MiniPay
- AD-04 -[rejects]-> ALT-FailAlwaysReject: Fraud lỗi thì luôn từ chối
- AD-04 -[rejects]-> ALT-FailAlwaysAllow: Fraud lỗi thì luôn cho qua
- FR-020: Kiểm tra rủi ro qua dịch vụ Fraud Check trước khi ghi nợ -[satisfiedBy]-> AD-04

```json
{
 "statement": "Fraud Check lỗi: cho qua giao dịch nhỏ, từ chối giao dịch lớn",
 "rationale": "Cân bằng trải nghiệm và rủi ro (FR-020)"
}
```

### ADR-001 - Chọn FastAPI + Uvicorn cho API
Lớp: Decision | trạng thái: InReview v0.1 | nguồn: TDD 4
- ADR-001 -[constrainedBy]-> CON-001: Dùng Python 3.10 trở lên
- ADR-001 -[describedIn]-> DOC-TDD: TDD MiniPay
- ADR-001 -[rejects]-> ALT-Flask: Flask
- ADR-001 -[selects]-> TECH-FastAPI: FastAPI

```json
{
 "statement": "Chọn FastAPI + Uvicorn cho API",
 "rationale": "Kiểm tra đầu vào, async, tài liệu API tự sinh"
}
```

### ADR-002 - Chọn SQLite (WAL) cho bản demo
Lớp: Decision | trạng thái: InReview v0.1 | nguồn: TDD 4
- ADR-002 -[constrainedBy]-> CON-002: Bản demo dùng SQLite; môi trường thật cần CSDL quan hệ hỗ trợ ACID
- ADR-002 -[describedIn]-> DOC-TDD: TDD MiniPay
- ADR-002 -[hasRisk]-> RISK-004: SQLite chỉ một luồng ghi tại một thời điểm
- ADR-002 -[rejects]-> ALT-PostgreSQL: PostgreSQL cho bản demo
- ADR-002 -[selects]-> TECH-SQLite: SQLite (WAL)
- ADR-002 -[supportedBy]-> EV-001: Benchmark chuyển tiền (demo)

```json
{
 "statement": "Chọn SQLite (WAL) cho bản demo",
 "rationale": "Không cần cài đặt; đủ giao dịch ACID cho demo; thật thì thay bằng CSDL quan hệ"
}
```

### ADR-003 - Chọn httpx làm HTTP client cho tích hợp ngoài
Lớp: Decision | trạng thái: InReview v0.1 | nguồn: TDD 4
- ADR-003 -[describedIn]-> DOC-TDD: TDD MiniPay
- ADR-003 -[rejects]-> ALT-Requests: requests
- ADR-003 -[selects]-> TECH-httpx: httpx

```json
{
 "statement": "Chọn httpx làm HTTP client cho tích hợp ngoài",
 "rationale": "Cấu hình rõ timeout (connect/read/total) và connection pool"
}
```

## Cơ chế (10)

### MECH-APIKeyAuth - API key dạng băm SHA-256, Bearer token
Lớp: Mechanism | trạng thái: InReview v0.1 | nguồn: TDD 7, 9
- FR-002: Xác thực mọi API (trừ tạo tài khoản) bằng API key; chỉ thao tác trên tài khoản của mình -[satisfiedBy]-> MECH-APIKeyAuth
- NFR-003: Bảo mật: API key lưu dạng băm SHA-256; log che số tài khoản -[satisfiedBy]-> MECH-APIKeyAuth

```json
{
 "mechanismKind": "security"
}
```

### MECH-AppendOnlyLedger - Sổ cái chỉ thêm, số dư = balance_after của bút toán cuối
Lớp: Mechanism | trạng thái: InReview v0.1 | nguồn: TDD 7, 9
- RISK-002: Số dư âm do hai giao dịch chạy đồng thời -[mitigatedBy]-> MECH-AppendOnlyLedger
- NFR-002: Toàn vẹn dữ liệu: số dư >= 0 mọi lúc; tổng số dư không đổi sau mọi giao dịch chuyển tiền -[satisfiedBy]-> MECH-AppendOnlyLedger

```json
{
 "mechanismKind": "integrity"
}
```

### MECH-CircuitBreaker - Circuit breaker ở Risk adapter
Lớp: Mechanism | trạng thái: InReview v0.1 | nguồn: TDD 7, 9
- RISK-001: Fraud Check chậm hoặc không sẵn sàng -[mitigatedBy]-> MECH-CircuitBreaker
- FR-020: Kiểm tra rủi ro qua dịch vụ Fraud Check trước khi ghi nợ -[satisfiedBy]-> MECH-CircuitBreaker
- NFR-006: Thời gian phản hồi tối đa 3 giây kể cả khi dịch vụ ngoài chậm -[satisfiedBy]-> MECH-CircuitBreaker

```json
{
 "mechanismKind": "resilience"
}
```

### MECH-Correlation - X-Correlation-Id sinh ở API, ghi vào giao dịch, audit, lời gọi ngoài
Lớp: Mechanism | trạng thái: InReview v0.1 | nguồn: TDD 7, 9
- NFR-005: Truy vết: 100% yêu cầu có correlationId, xuất hiện trong log và audit -[satisfiedBy]-> MECH-Correlation

```json
{
 "mechanismKind": "observability"
}
```

### MECH-HealthCheck - Endpoint /health cho load balancer
Lớp: Mechanism | trạng thái: InReview v0.1 | nguồn: TDD 7, 9
- NFR-004: Sẵn sàng 99,5% mỗi tháng cho API chuyển tiền -[satisfiedBy]-> MECH-HealthCheck

```json
{
 "mechanismKind": "availability"
}
```

### MECH-Idempotency - Idempotency-Key + request hash
Lớp: Mechanism | trạng thái: InReview v0.1 | nguồn: TDD 7, 9
- FR-012: Chống trùng giao dịch bằng Idempotency-Key -[satisfiedBy]-> MECH-Idempotency

```json
{
 "mechanismKind": "idempotency"
}
```

### MECH-IdempotentConsumer - Consumer khử trùng theo eventId (processed_events)
Lớp: Mechanism | trạng thái: InReview v0.1 | nguồn: TDD 7, 9
- RISK-003: Sự kiện thông báo bị gửi trùng -[mitigatedBy]-> MECH-IdempotentConsumer
- FR-021: Phát sự kiện TransferCompleted để gửi SMS thông báo -[satisfiedBy]-> MECH-IdempotentConsumer

```json
{
 "mechanismKind": "messaging"
}
```

### MECH-Outbox - Bảng outbox ghi cùng giao dịch CSDL, relay lên queue
Lớp: Mechanism | trạng thái: InReview v0.1 | nguồn: TDD 7, 9
- FR-021: Phát sự kiện TransferCompleted để gửi SMS thông báo -[satisfiedBy]-> MECH-Outbox

```json
{
 "mechanismKind": "messaging"
}
```

### MECH-TimeoutBudget - Ngân sách timeout chia từ ngoài vào trong
Lớp: Mechanism | trạng thái: InReview v0.1 | nguồn: TDD 7, 9
- RISK-001: Fraud Check chậm hoặc không sẵn sàng -[mitigatedBy]-> MECH-TimeoutBudget
- NFR-001: Hiệu năng chuyển tiền: p95 <= 500 ms ở mức 50 giao dịch/giây -[satisfiedBy]-> MECH-TimeoutBudget
- NFR-006: Thời gian phản hồi tối đa 3 giây kể cả khi dịch vụ ngoài chậm -[satisfiedBy]-> MECH-TimeoutBudget

```json
{
 "mechanismKind": "resilience"
}
```

### MECH-WriteLock - BEGIN IMMEDIATE và kiểm tra lại số dư/hạn mức trong giao dịch
Lớp: Mechanism | trạng thái: InReview v0.1 | nguồn: TDD 7, 9
- RISK-002: Số dư âm do hai giao dịch chạy đồng thời -[mitigatedBy]-> MECH-WriteLock
- FR-010: Chuyển tiền giữa hai tài khoản ACTIVE trong cùng một giao dịch CSDL -[satisfiedBy]-> MECH-WriteLock
- FR-011: Kiểm tra hạn mức 50.000.000 VND/giao dịch và 200.000.000 VND/ngày -[satisfiedBy]-> MECH-WriteLock
- NFR-002: Toàn vẹn dữ liệu: số dư >= 0 mọi lúc; tổng số dư không đổi sau mọi giao dịch chuyển tiền -[satisfiedBy]-> MECH-WriteLock

```json
{
 "mechanismKind": "concurrency"
}
```

## Luồng xử lý (2)

### SCN-Notify - Thông báo sau chuyển tiền (bất đồng bộ)
Lớp: Scenario | trạng thái: InReview v0.1 | nguồn: TDD 8
- SCN-Notify -[invokes]-> INT-02: Sự kiện giao dịch (message queue)
- SCN-Notify -[invokes]-> INT-03: SMS Gateway
- MOD-Notify: Thông báo -[participatesIn]-> SCN-Notify
- FR-021: Phát sự kiện TransferCompleted để gửi SMS thông báo -[realizedBy]-> SCN-Notify

```json
{
 "responseBudgetMs": 60000,
 "steps": "Outbox -> queue -> consumer khử trùng -> SMS Gateway"
}
```

### SCN-Transfer - Chuyển tiền (đồng bộ)
Lớp: Scenario | trạng thái: InReview v0.1 | nguồn: TDD 8
- SCN-Transfer -[invokes]-> INT-01: Fraud Check
- MOD-API: API Layer -[participatesIn]-> SCN-Transfer
- MOD-Transfer: Chuyển tiền -[participatesIn]-> SCN-Transfer
- MOD-Ledger: Sổ cái -[participatesIn]-> SCN-Transfer
- MOD-Risk: Risk adapter -[participatesIn]-> SCN-Transfer
- MOD-Audit: Audit -[participatesIn]-> SCN-Transfer
- FR-010: Chuyển tiền giữa hai tài khoản ACTIVE trong cùng một giao dịch CSDL -[realizedBy]-> SCN-Transfer
- FR-011: Kiểm tra hạn mức 50.000.000 VND/giao dịch và 200.000.000 VND/ngày -[realizedBy]-> SCN-Transfer
- FR-012: Chống trùng giao dịch bằng Idempotency-Key -[realizedBy]-> SCN-Transfer
- FR-020: Kiểm tra rủi ro qua dịch vụ Fraud Check trước khi ghi nợ -[realizedBy]-> SCN-Transfer
- NFR-001: Hiệu năng chuyển tiền: p95 <= 500 ms ở mức 50 giao dịch/giây -[realizedBy]-> SCN-Transfer
- NFR-006: Thời gian phản hồi tối đa 3 giây kể cả khi dịch vụ ngoài chậm -[realizedBy]-> SCN-Transfer

```json
{
 "responseBudgetMs": 3000,
 "steps": "Xác thực -> kiểm tra đầu vào -> idempotency -> hạn mức/số dư -> Fraud Check -> ghi sổ + outbox -> trả kết quả"
}
```

## Điểm tích hợp (contract) (3)

### INT-01 - Fraud Check
Lớp: Integration | trạng thái: InReview v0.1 | nguồn: TDD 6.1

#### Contract INT-01 (Fraud Check)
- Kiểu: ThirdParty, hướng outbound, sync, idempotent=True, giao thức REST/JSON
- Đối tác: Dịch vụ Fraud Check (contractConfirmed=False, sandbox=True)
- Endpoint: POST /v1/fraud/checks; header: X-Correlation-Id*, Authorization*
  - request: {"transactionId":"TRF-AB12","fromAccount":"0123456789","toAccount":"9876543210","amount":1000000,"currency":"VND"} | schema: contracts/fraud-check.request.schema.json
  - response: {"decision":"APPROVE","riskScore":10,"correlationId":"..."} | schema: contracts/fraud-check.response.schema.json
- Timeout (ms): connect=300 read=800 total=1000 poolAcquire=200
- Retry: max=2 backoff=none idempotentOnly=True
- Circuit breaker: 100% / tối thiểu 5 lần, mở 30s
- Pool: maxTotal=20 maxPerRoute=20 idleTimeout=30s
- Bảo mật: APIKey, secret: biến môi trường (demo); secret manager khi triển khai thật, xoay 90 ngày
- Giới hạn đối tác: 50 rps, 50 kết nối
- Dự phòng: degraded - Số tiền < 5.000.000 VND: cho qua, riskCheck=SKIPPED. Từ 5.000.000 VND: từ chối FRAUD_CHECK_UNAVAILABLE (503)
- Lỗi [permanent] HTTP 4xx (400, 401, 422) -> retryable=False: Không retry; coi là Fraud không khả dụng, áp chính sách FB-INT01; ghi cảnh báo vi phạm contract
- Lỗi [transient] HTTP 5xx -> retryable=True: Retry 1 lần; vẫn lỗi thì coi là không khả dụng, áp FB-INT01; tính vào circuit breaker
- Lỗi [unknownOutcome] Timeout (connect/read) hoặc lỗi mạng -> retryable=True: Retry 1 lần với cùng transactionId (idempotent); vẫn lỗi thì áp FB-INT01
- Lỗi [permanent] HTTP 200 nhưng response sai schema -> retryable=False: Không retry; coi là không khả dụng; ghi cảnh báo vi phạm contract
- LLD/code: minipay/fraud_client.py::check
- Chưa chốt: ASM-001 Dịch vụ Fraud Check chịu được 50 request mỗi giây; đối tác chưa xác nhận giới hạn này (hạn 2026-10-21, Tech Lead)
- Chưa chốt: OQ-001 Hạn mức mỗi giao dịch và mỗi ngày cuối cùng do bộ phận nào phê duyệt (hạn 2026-10-28, Business Analyst)

### INT-02 - Sự kiện giao dịch (message queue)
Lớp: Integration | trạng thái: InReview v0.1 | nguồn: TDD 6.2

#### Contract INT-02 (Sự kiện giao dịch (message queue))
- Kiểu: MessageQueue, hướng outbound, async, idempotent=True, giao thức queue
- Đối tác: Message broker nền tảng (contractConfirmed=True, sandbox=True)
- Channel: Topic minipay.transfer.events.v1 key=fromAccountId partitions=3 atLeastOnce dlq=minipay.transfer.events.v1.dlq consumerIdempotent=True
  - message: {"eventId":"TRF-AB12","transferId":"TRF-AB12","fromAccountId":"..","toAccountId":"..","amount":1000,"correlationId":".."} | schema: contracts/transfer-completed.event.schema.json
- Timeout (ms): connect=500 read=1000 total=2000 poolAcquire=200
- Retry: max=3 backoff=exponential idempotentOnly=True
- Pool: maxTotal=5 maxPerRoute=5 idleTimeout=60s
- Bảo mật: mTLS, secret: secret manager, xoay 365 ngày
- Giới hạn đối tác: 1000 rps, 100 kết nối
- Lỗi [transient] Broker không khả dụng / 5xx -> retryable=True: Giữ sự kiện PENDING trong outbox, retry theo backoff
- Lỗi [permanent] Sự kiện sai schema hoặc quá maxMessageKB -> retryable=False: Không retry; chuyển DLQ
- Lỗi [unknownOutcome] Timeout khi publish -> retryable=True: Retry; có thể trùng sự kiện, consumer khử trùng theo eventId
- LLD/code: minipay/notify.py::relay_outbox

### INT-03 - SMS Gateway
Lớp: Integration | trạng thái: InReview v0.1 | nguồn: TDD 6.3

#### Contract INT-03 (SMS Gateway)
- Kiểu: ThirdParty, hướng outbound, async, idempotent=False, giao thức REST/JSON
- Đối tác: SMS Gateway (contractConfirmed=False, sandbox=True)
- Endpoint: POST /v1/sms; header: Idempotency-Key*, Authorization*
  - request: {"to":"0912345678","content":"MiniPay: +1,000 VND","messageId":"TRF-AB12"} | schema: contracts/sms.request.schema.json
- Timeout (ms): connect=500 read=2000 total=3000 poolAcquire=300
- Retry: max=3 backoff=exponential idempotentOnly=False
- Pool: maxTotal=5 maxPerRoute=5 idleTimeout=30s
- Bảo mật: APIKey, secret: secret manager, xoay 90 ngày
- Giới hạn đối tác: 20 rps, 10 kết nối
- Dự phòng: degraded - Sau 3 lần thất bại ghi nhận và bỏ qua; không đổi kết quả chuyển tiền
- Lỗi [transient] HTTP 5xx -> retryable=True: Retry tối đa 3 lần
- Lỗi [permanent] HTTP 4xx (số nhận không hợp lệ) -> retryable=False: Không retry; ghi nhận và bỏ qua
- Lỗi [unknownOutcome] Timeout -> retryable=True: Retry với cùng Idempotency-Key; gateway khử trùng
- LLD/code: minipay/notify.py::consume
- Chưa chốt: ASM-002 SMS Gateway có môi trường sandbox để chạy test (hạn 2026-10-21, QA Lead)

## Dữ liệu (5)

### DE-Account - Account
Lớp: DataEntity | trạng thái: InReview v0.1 | nguồn: SAD/HLD Dữ liệu mức khái niệm
- DE-Account -[relatesTo]-> DE-LedgerEntry: LedgerEntry
- MOD-Account: Quản lý tài khoản -[ownsData]-> DE-Account

```json
{
 "retentionRule": "Giữ vô thời hạn",
 "description": "Giữ vô thời hạn"
}
```

### DE-AuditLog - AuditLog
Lớp: DataEntity | trạng thái: InReview v0.1 | nguồn: SAD/HLD Dữ liệu mức khái niệm
- MOD-Audit: Audit -[ownsData]-> DE-AuditLog

```json
{
 "retentionRule": "Chỉ thêm, không sửa/xóa",
 "description": "Chỉ thêm, không sửa/xóa"
}
```

### DE-LedgerEntry - LedgerEntry
Lớp: DataEntity | trạng thái: InReview v0.1 | nguồn: SAD/HLD Dữ liệu mức khái niệm
- MOD-Ledger: Sổ cái -[ownsData]-> DE-LedgerEntry
- DE-Account: Account -[relatesTo]-> DE-LedgerEntry
- DE-Transfer: Transfer -[relatesTo]-> DE-LedgerEntry

```json
{
 "retentionRule": "Chỉ thêm, không sửa/xóa",
 "description": "Chỉ thêm, không sửa/xóa"
}
```

### DE-OutboxEvent - OutboxEvent
Lớp: DataEntity | trạng thái: InReview v0.1 | nguồn: SAD/HLD Dữ liệu mức khái niệm
- MOD-Transfer: Chuyển tiền -[ownsData]-> DE-OutboxEvent
- DE-Transfer: Transfer -[relatesTo]-> DE-OutboxEvent

```json
{
 "retentionRule": "Xóa sau khi đã phát và quá hạn lưu",
 "description": "Xóa sau khi đã phát và quá hạn lưu"
}
```

### DE-Transfer - Transfer
Lớp: DataEntity | trạng thái: InReview v0.1 | nguồn: SAD/HLD Dữ liệu mức khái niệm
- DE-Transfer -[relatesTo]-> DE-LedgerEntry: LedgerEntry
- DE-Transfer -[relatesTo]-> DE-OutboxEvent: OutboxEvent
- MOD-Transfer: Chuyển tiền -[ownsData]-> DE-Transfer

```json
{
 "retentionRule": "Giữ 5 năm",
 "description": "Giữ 5 năm"
}
```

## Rủi ro (4)

### RISK-001 - Fraud Check chậm hoặc không sẵn sàng
Lớp: Risk | trạng thái: InReview v0.1 | nguồn: TDD 11
- RISK-001 -[mitigatedBy]-> MECH-CircuitBreaker: Circuit breaker ở Risk adapter
- RISK-001 -[mitigatedBy]-> MECH-TimeoutBudget: Ngân sách timeout chia từ ngoài vào trong
- INT-01: Fraud Check -[hasRisk]-> RISK-001

```json
{
 "likelihood": "Medium",
 "impact": "High",
 "mitigation": "Timeout chặt, retry có giới hạn, circuit breaker, chính sách fail theo ngưỡng (AD-04)"
}
```

### RISK-002 - Số dư âm do hai giao dịch chạy đồng thời
Lớp: Risk | trạng thái: InReview v0.1 | nguồn: TDD 11
- RISK-002 -[mitigatedBy]-> MECH-WriteLock: BEGIN IMMEDIATE và kiểm tra lại số dư/hạn mức trong giao dịch
- RISK-002 -[mitigatedBy]-> MECH-AppendOnlyLedger: Sổ cái chỉ thêm, số dư = balance_after của bút toán cuối
- MOD-Ledger: Sổ cái -[hasRisk]-> RISK-002

```json
{
 "likelihood": "Medium",
 "impact": "High",
 "mitigation": ""
}
```

### RISK-003 - Sự kiện thông báo bị gửi trùng
Lớp: Risk | trạng thái: InReview v0.1 | nguồn: TDD 11
- RISK-003 -[mitigatedBy]-> MECH-IdempotentConsumer: Consumer khử trùng theo eventId (processed_events)
- INT-02: Sự kiện giao dịch (message queue) -[hasRisk]-> RISK-003

```json
{
 "likelihood": "High",
 "impact": "Low",
 "mitigation": ""
}
```

### RISK-004 - SQLite chỉ một luồng ghi tại một thời điểm
Lớp: Risk | trạng thái: InReview v0.1 | nguồn: TDD 11
- ADR-002: Chọn SQLite (WAL) cho bản demo -[hasRisk]-> RISK-004

```json
{
 "likelihood": "High",
 "impact": "Medium",
 "mitigation": "Chỉ dùng cho demo; môi trường thật dùng CSDL quan hệ riêng (ADR-002)"
}
```

## Giả định (2)

### ASM-001 - Dịch vụ Fraud Check chịu được 50 request mỗi giây; đối tác chưa xác nhận giới hạn này
Lớp: Assumption | trạng thái: InReview v0.1 | nguồn: SRS Giả định
- AD-04: Fraud Check lỗi: cho qua giao dịch nhỏ, từ chối giao dịch lớn -[assumes]-> ASM-001
- INT-01: Fraud Check -[assumes]-> ASM-001

```json
{
 "statement": "Dịch vụ Fraud Check chịu được 50 request mỗi giây; đối tác chưa xác nhận giới hạn này",
 "dueDate": "2026-10-21",
 "confirmStatus": "Open"
}
```

### ASM-002 - SMS Gateway có môi trường sandbox để chạy test
Lớp: Assumption | trạng thái: InReview v0.1 | nguồn: SRS Giả định
- INT-03: SMS Gateway -[assumes]-> ASM-002

```json
{
 "statement": "SMS Gateway có môi trường sandbox để chạy test",
 "dueDate": "2026-10-21",
 "confirmStatus": "Open"
}
```

## Câu hỏi mở (1)

### OQ-001 - Hạn mức mỗi giao dịch và mỗi ngày cuối cùng do bộ phận nào phê duyệt
Lớp: OpenQuestion | trạng thái: InReview v0.1 | nguồn: SRS Câu hỏi mở
- INT-01: Fraud Check -[blockedBy]-> OQ-001

```json
{
 "statement": "Hạn mức mỗi giao dịch và mỗi ngày cuối cùng do bộ phận nào phê duyệt",
 "dueDate": "2026-10-28",
 "confirmStatus": "Open"
}
```

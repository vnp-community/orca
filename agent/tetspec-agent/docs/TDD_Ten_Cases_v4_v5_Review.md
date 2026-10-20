# Review mười trường hợp cần TDD và coverage ontology v0.4/v0.5

Ngày: 2026-10-07. Phạm vi: frozen v0.4 và runtime v0.5 hiện có. Đây là review, không phải proposal hoặc xác nhận nghiệm thu; không thay đổi ontology/code theo review này.

Kết luận: danh sách trigger hợp lý; đánh giá các khoảng trống nhìn chung đúng. Tuy nhiên, “đủ mô hình 3/10” không chứng minh đủ thông tin để sinh thiết kế có thể triển khai. v0.5 cải thiện provenance, delta, intake, trace, edit và publication, nhưng chưa bổ sung semantic profiles/validators đủ cho mười trường hợp.

## Cách dùng danh sách trigger

Các dấu hiệu giúp quyết định nhu cầu và độ sâu thiết kế. Không nên hiểu một dấu hiệu luôn bắt buộc một TDD mới đầy đủ: có thể cập nhật TDD hiện có, viết scoped TDD/ADR hoặc dùng change spec gắn baseline. CRUD vẫn cần thiết kế khi chạm dữ liệu nhạy cảm, quyền, invariant hoặc compatibility. Có nhiều developer là dấu hiệu phối hợp; mức độ thiết kế còn phụ thuộc độ mới của hợp đồng và blast radius. Payment là profile rủi ro tham khảo; ontology nền không nên mặc định toàn bộ dự án là hệ thống tài chính.

Agent cần ra quyết định có evidence trước generation: mức thiết kế, TDD baseline/owner bị tác động, required sections/needs/rules, lý do áp dụng hoặc không áp dụng. Triệu chứng kỹ thuật, thay đổi behavior, mức rủi ro và authority cần được ghi lại; chưa đủ dữ liệu thì yêu cầu làm rõ.

## Các điểm cần sửa trong nhận xét v0.4

1. `ConcurrencyControl` là vocabulary enum của `Operation.concurrency_control`, không phải class. `RoutePolicy.retry` dùng enum none/connect_only/read/all; đúng là chưa phải retry policy có attempts/backoff/jitter/retryable-errors.
2. `DecisionTable` có inputs/outputs/hit_policy; `DecisionRule` có when/then kiểu JSON; quan hệ part_of cho phép các dòng thuộc bảng. Ma trận phân quyền có thể biểu diễn; thiếu contract cho dữ liệu biểu thức, mapping role/resource/action/context và kiểm coverage/deny/conflict. Không nên nói ontology hoàn toàn không có chỗ cho ma trận dữ liệu.
3. L-01..L-09 đã có khai báo; validate3.py có kiểm một phần L-04/05/08/09. L-06/07 còn insufficient_data; L-01 không thực sự tính tích miền; L-02/03 chưa kiểm đầy đủ. Chính xác là chưa thực thi đầy đủ, không phải cả nhóm không chạy được.
4. RTO/RPO có thể lưu qua QualityTarget/Quantity; decimal có thể ghi trong Attribute.datatype; invariant có thể ghi bằng Condition/BusinessRule/Formula. Khoảng trống là mô hình/đơn vị/quan hệ và validator chuyên biệt có contract kiểm được. Generic text/json/ext cho phép lưu nhưng không tự đảm bảo semantic completeness.
5. Component thiếu team ownership và contract version compatibility: nhận xét đúng. owner_doc là tài liệu quản lý; v0.5 Capability.owner và planner owner chưa xác định một mô hình team/service ownership hoàn chỉnh.
6. v0.4 có DocumentTemplate/SectionSpec.required/holds và ChangeRequest.target_docs. Có chỗ lưu cấu trúc mục và tài liệu đích; chưa có quy tắc quyết định từ mười trigger tới các mục/needs/validators phải có.

## Đối chiếu từng trường hợp

| # | Trường hợp | v0.4 | v0.5 đã bổ sung/kiểm được | Còn thiếu để chứng nhận đủ |
|---|---|---|---|---|
| 1 | Logic đáng kể | Có Algorithm/Formula/Condition/BusinessRule/FSM và decision tables; steps/expressions còn yếu về semantic contract | Giữ BusinessRule; intake State/Transition; structured scenarios/acceptance và semantic render | Algorithm pre/post/input/output; decision coverage/exclusion; FSM reachability/guard/action; negative/boundary behavior; QG không kiểm đầy đủ các lớp này |
| 2 | Architecture/component/module | Inventory, dependency và Decision/Alternative đủ làm nền mô tả | Immutable baseline/delta, impact/review invalidation, repo hash; design/work/test trace | Topology/responsibility và rationale có kiểm; runtime/deployment impact, constraints, architecture compatibility; không phải mọi Alternative/Decision đều đã được kiểm đủ |
| 3 | Integration | Interface/Operation/Event/ExternalSystem/DataStore; delivery và ordering có enum/field | QG kiểm khe API request/response/auth/error và write idempotency/concurrency; immutable scoped pack | Typed retry/circuit breaker/fallback/DLQ, event ordering/dedup, cache TTL/invalidation, blockchain finality/reorg khi áp dụng; typed closure chưa đi hết security/event/config relations |
| 4 | Schema/data/migration | Entity/Attribute/Index/View/Retention; Migration chỉ path/reversible | Logical attributes giữ evidence/unknown type; gap physical schema; QG entity schema check; delta/source pinning | Migration stages/backfill, lock/downtime/volume, forward/backward compatibility, rollback/recovery; validation migration chưa chuyên biệt |
| 5 | Performance/TPS/concurrency/HA | Quantity/QualityTarget/Budget/Slo/Metric, concurrency enum, scaling string | Giữ QualityTarget và evidence; QG đòi measurement | Workload/load distribution, burst/payload/growth, capacity model, replicas/failure domains/failover, RTO/RPO gắn topology và tests; measurement presence chưa chứng minh đo được |
| 6 | Transaction/retry/timeout/idempotency/consistency | Local operation policy và delivery semantics; thiếu transaction/saga contract | Có timeout property edit/history/conflict; write API cần khe idempotency/concurrency | Boundary/isolation/atomicity, consistency scope, saga/compensation, retry policy/error classes, timeout budget/cancellation, outbox/inbox và failure-path tests; khe có giá trị chưa chứng minh cơ chế đúng |
| 7 | Security/authentication/authorization | Vocabulary khá rộng: Role/Scope/Credential/Control/Threat/DataClass và relations | Evidence/review/conflict gates, API auth/error check | Permission matrix semantics/default-deny/object-level rules, trust-boundary/control coverage, credential lifecycle tests; QG/TS không thay security validator; spec closure chưa bao phủ requires_role/requires_scope/mitigates/protects |
| 8 | Impact lên hệ thống hiện hữu | Baseline/ExternalSystem/dependency và impact engine | Repo/spec snapshots, immutable closure, stale/collision checks và downstream invalidation | Live contract/consumer inventory, released contract versions, rollout coexistence, verified dependency completeness; source/code snapshots chưa là inventory môi trường đang chạy |
| 9 | Nhiều team | Decision/WorkItem/Gate, owner_doc và interface versioning | Capability.owner, planner assignments, WorkItem dependencies/DAG/DoD | Team/ownership accountability, producer/consumer contract acceptance, compatibility rules, release coordination và approvals; file owner không đồng nghĩa responsible team |
| 10 | Profile rủi ro cao | Risk/Control/Threat và generic invariant/formula/retention concepts | Shared evidence/version/trace and review policy | Conditional domain contract: currency/decimal/rounding, ledger balance/reversal, reconciliation, immutable audit, duplicate settlement prevention và property tests khi áp dụng; chưa có domain profile/validators này |

Không gán phần trăm coverage từ số lớp. Mỗi hàng cần fixtures và acceptance theo behavior thực; ontology không cần áp mọi nhóm cho mọi change. PBOM pilot và sáu authoring runs là bằng chứng trên scope hẹp, không nghiệm thu cả mười nhóm.

## Finding quan trọng ở runtime v0.5

### F1 — Mandatory semantics của lớp kế thừa chưa được thực thi đầy đủ

`envelope5.validate_element` chỉ kiểm required của các lớp khai báo mới trong ontology.json. Lớp legacy được chấp nhận theo tên; field/maturity semantics của toàn bộ classes.yaml không được áp tự động. Quan hệ legacy đã kiểm domain/range nhưng chưa đủ để thay kiểm nội dung.

Đã chạy diagnostic read-only: một Algorithm maturity=specified, review_status=approved, thiếu inputs/outputs/pre/post vẫn validate_graph ok và QG-01..QG-10 đều pass. Evidence nguồn chỉ có metadata, không có source content nên kiểm quote/hash cũng không thực thi ở case này. Kết quả không đồng nghĩa TS/spec readiness pass; nó chứng minh QG chưa đảm bảo class completeness và source verification khi không cấp nguồn.

Bằng chứng: `TDD_Ten_Cases_v4_v5_Review.evidence.json`. Không thay đổi graph production. Cổng fail-closed hiện chỉ chặn các nhu cầu/luật đã được yêu cầu; domain rule chưa nằm trong profile có thể bị bỏ sót.

### F2 — Tên field giữa v0.4 và checks v0.5 chưa đồng bộ

v0.4 Operation dùng path_or_name/authn/idempotent/idempotency_key/concurrency_control; QG kiểm path/auth/idempotency/concurrency. v0.4 QualityTarget có measurement_ref; QG đòi measurement. Need một canonical property contract hoặc adapter rõ ràng, bảo toàn Quantity/unit/basis. Nếu không, fact hợp lệ có thể bị báo thiếu, hoặc field arbitrary trong props được coi là đủ.

### F3 — Chưa có risk-based TDD routing và domain needs

GenerationProfile hiện có ba profile theo luồng; mandatory_needs còn generic. Dynamic request nhận scope/profile và needs từ caller. Applicability dựa record được cấp, chưa tự phân loại mười trigger để suy ra mức TDD, target docs, required sections và domain validators. planner ownership ưu tiên owner_doc. Cần giải quyết routing cùng semantic extensions, không chỉ thêm class tên mới.

### F4 — Coverage retrieval/evaluation chưa phải full design assurance

Spec typed_closure dùng whitelist relations, chưa đầy đủ các quan hệ role/scope/security/event/deployment/config/logic. Budget truncation được báo nhưng quan hệ không được traverse không tự trở thành missing blocker. QG contract/measurement/work checks chủ yếu presence, và evidence verification phụ thuộc source content có được truyền vào hay không. 54 test pass và CLI pass không chứng minh đủ cả mười use cases.

## Đề xuất thứ tự xử lý

1. Chốt định nghĩa readiness và scope cho từng nhóm; canonical property/quantity/provenance contract; thực thi mandatory legacy class semantics. Có regression chống false ready trước khi mở rộng.
2. Thêm routing assessment có trigger/evidence/risk/affected-baseline/output-depth; chọn conditional profiles và required sections/needs/validators.
3. Ưu tiên behavioral resilience + transaction/consistency, migration/compatibility và workload/HA; có thể là class hoặc cấu trúc typed lồng nhau, nhưng cần quan hệ và checks thực.
4. Bổ sung security/consumer/team/domain profiles tùy applicability; mở closure từ registry relation semantics thay whitelist thiếu; phải ghi missing dependency.
5. Dùng các profiles ở cả ba luồng: initial/CR cần assessment; prompt edit phải reroute khi đổi risk/behavior; KG→spec phải tiêu thụ assessment và completeness evidence.
6. Nghiệm thu fixtures đủ/thiếu/mâu thuẫn cho cả mười nhóm, gồm scenarios bình thường/lỗi/biên, trace tới design/work/tests và source verification. Chưa dùng tuyên bố “đủ cả mười” trước bước này.

## Sources đã đọc

- v0.4: classes.yaml, vocab.yaml, relations.yaml, generated schemas, rules.yaml và tools/validate3.py.
- v0.5: ontology.json, tools/envelope5.py, tools/ontology5.py, tools/quality5.py, tools/techspec_rules.py, tools/readiness5.py và harness/dynamic.py.
- Trạng thái test/benchmark đã lưu trong implementation-status và run-quality-authoring; không chạy thêm model trong lượt review này.

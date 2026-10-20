# 09 — Ví dụ contract, kết quả kiểm tra và ranh giới implementation

Các payload dưới đây là ví dụ thiết kế, không phải dữ liệu dự án đã duyệt hoặc executable fixtures. IDs, hashes, actors và facts giả định dùng để giải thích contract; không đưa vào accepted graph. `<...>` là placeholder có chủ đích và MUST bị production validator từ chối nếu chưa resolve.

## 1. Meta-schema để chuyển data dictionaries thành schemas

Một `TypeDefinition` có `qualifiedName`, `version`, `owningDomain`, `baseTypes`, `identityPolicy`, `properties`, `relations`, `invariants`, `lifecycleMachine`, `maturityRequirements` và `sourceSpecRef`. Public domain type references đi qua link contract, không import vòng toàn bộ model.

`PropertyDefinition` bắt buộc khai:

| Thuộc tính | Nghĩa / constraints |
|---|---|
| name, datatype | Qualified name và primitive/value-object/ref type; không dùng free-form object cho normative predicate |
| minCount, maxCount | Cardinality chính xác; unbounded explicit; min không âm và ≤ max |
| requiredWhen | Typed applicability predicate theo state/profile, không chỉ field present |
| valueDomain | Enum/range/pattern hoặc referenced constraint |
| unitDimension | Bắt buộc khi Quantity; không ngầm suy từ property name |
| nullPolicy | forbidden/allowed/unknown-via-gap; null không chứa business meaning tùy ý |
| collectionSemantics | set/list/multiset; list có order, set unique theo defined key |
| referencePolicy | target type, revision-required, cross-project allowance, deletion/tombstone behavior |
| authorityPolicy | Source/authoring authority nào được ghi hoặc đổi field |
| derivationPolicy | Whether computed, rule dependencies, no manual override hoặc authorized exception |
| changeSemantics | presentation/semantic/authority/evidence và invalidation facets |
| sourceBinding | Property-level anchors hoặc approved derivation; không chỉ entity-level citation chung |
| validationRules | IDs của constraints/checks và maturity cần thực thi |

Fields trong dictionaries 00–07 dùng shorthand; implementation MUST expand theo meta-schema này. Đây là bước schema authoring cần làm, không được coi catalog JSON index đã là full schemas. Mỗi property normative phải có rule hoặc explicit rationale vì sao free text chưa thể formalize; free text vẫn có source/review obligation.

`RelationDefinition` cần domain/range, endpoint role names, cardinalities, inverse query name, symmetry/transitivity declarations (default false), uniqueness, scope compatibility, temporal compatibility, evidence policy và cycle policy. Tránh suy symmetric/transitive từ tên quan hệ. `contains` và `dependsOn` không có cùng cycle semantics.

Lifecycle machine gồm transitions `{from,to,trigger,guard,requiredRoles,evidenceRequirements,effects,invalidationRules}`. Lifecycle transition creates event/revision, không update một enum độc lập rồi bỏ kiểm guard.

## 2. Constant và quality requirement

Giả định hệ thống S cho phép cấu hình deadline operation Submit. Một SRS target là worst-case deadline trong scope giới hạn đã mô tả, không phải p95.

```json
{
  "identity": "srs:S:deadline-submit",
  "revision": "r2",
  "type": "srs:QualityRequirement",
  "content": {
    "modality": "MUST",
    "metric": "end_to_end_deadline",
    "statistic": "upper_bound",
    "comparator": "le",
    "target": {"value": "8", "unit": "s", "dimension": "time"},
    "workloadRef": "foundation:S:workload-bounded@r1",
    "environmentRef": "foundation:S:env-prod@r1",
    "measurementWindowRef": "foundation:S:single-submit-operation@r1",
    "method": "bounded-budget-analysis-and-execution-check"
  },
  "provenance": ["foundation:S:assertion-deadline@r1"],
  "scopeRef": "foundation:S:submit-scope@r1"
}
```

Đây chỉ là content excerpt, chưa đủ envelope/schema loadable. Rule FND-005 kiểm time dimension; SRS-004 kiểm bound/workload/environment semantics; M17 kiểm TDD budget; G5 execution check riêng. Không dùng target 8s để kết luận đã đạt 8s.

Một TDD sequential retry model N=3, mỗi attempt 2s, backoffs=[1s,2s], overhead=0.5s tạo bound 9.5s. Nếu không có deadline enforcement/cancellation hoặc model khác được chấp nhận thì violate. Nếu có absolute deadline cap ở 8s, cần thêm evidence cho propagation/cancellation/side-effects/recovery; không chỉ đổi phép tính min(8,9.5) rồi declare mọi obligation pass.

## 3. Business rule có exceptions và constants

Ví dụ giả định: khách VIP được giảm 10% trên eligible subtotal, hàng thuộc category X không eligible, giảm không vượt 200.000 VND, làm tròn đến đồng theo half-up. Đây là luật minh họa, không phải nghiệp vụ đã có của PBOM.

Nguồn SRS phải xác định VIP tại thời điểm nào, tax/shipping có trong subtotal không, stacking với discount khác thế nào. Chưa có ba thông tin này → KnowledgeGap, không tự default.

```json
{
  "type": "srs:BusinessRule",
  "ruleKind": "calculation",
  "expression": {
    "op": "call",
    "functionRef": "foundation:round-money@1",
    "args": [
      {
        "op": "min",
        "args": [
          {
            "op": "mul",
            "args": [
              {"op": "ref", "name": "eligibleSubtotal"},
              {"op": "ref", "name": "srs:vip-discount-rate@r1"}
            ]
          },
          {"op": "ref", "name": "srs:vip-discount-cap@r1"}
        ]
      },
      {"op": "literal", "type": "RoundingMode", "value": "half_up"}
    ]
  },
  "requiredInputs": ["eligibleSubtotal", "VIPAtEligibilityTime"],
  "unresolvedQuestions": ["eligibilityTime", "taxShippingBasis", "discountStacking"]
}
```

`call` là hình thức invoke registry function trong AST contract; chỉ hỗ trợ khi function signature/version registered. Money×dimensionless→Money; min đòi cùng currency; round-money phải có scale/rounding semantics. TDD implementation dùng float hoặc round-down có thể sai dù API/output schema đúng. M16 kiểm rule preservation bằng boundary cases, counterexamples hoặc proof/review theo supported domain; unknown stacking vẫn chặn gate yêu cầu business rule hoàn chỉnh.

## 4. Mapping nhiều-nhiều và contributions

Giả định một criterion “request hợp lệ được xử lý đúng và không ghi trùng khi retry” tách thành SRS-R1 functional processing, SRS-R2 duplicate prevention. TDD operation D1 và idempotency policy D2 cùng giải quyết; chỉ D1 không đủ.

```json
{
  "id": "link:S:processing-design",
  "revision": "r1",
  "mappingType": "M05",
  "sourceRefs": ["srs:S:R1@r2", "srs:S:R2@r1"],
  "targetRefs": ["tdd:S:D1@r3", "tdd:S:D2@r2"],
  "context": {"baselineRef": "flow:S:B2", "scopeRef": "foundation:S:submit-scope@r1"},
  "contributions": [
    {"endpointRef": "tdd:S:D1@r3", "obligationIds": ["O-functional"], "compositionGroup": "ALL"},
    {"endpointRef": "tdd:S:D2@r2", "obligationIds": ["O-duplicate", "O-concurrent-retry"], "compositionGroup": "ALL"}
  ],
  "assessmentState": "proposed",
  "missingEvidence": ["atomic-key-reservation-proof"]
}
```

Mapping proposed không thành validated chỉ vì endpoints tồn tại. Khi D2 thiếu atomic reservation/concurrent duplicate behavior, O-concurrent-retry unresolved; M05 coverage partial, G3 blocked. Không truyền kết luận “R2 covered” qua một generic traces_to edge.

## 5. Rule execution và finding mẫu

```json
{
  "id": "check:S:budget-001",
  "ruleId": "TDD-008",
  "ruleVersion": "<rule-version>",
  "engineVersion": "<engine-version>",
  "inputBaselineDigest": "<resolved-baseline-hash>",
  "subjectRefs": ["tdd:S:submit-retry@r1"],
  "scope": "foundation:S:submit-scope@r1",
  "applicability": "applicable",
  "executionStatus": "completed",
  "evaluationOutcome": "fail",
  "witness": {
    "maxAttempts": 3,
    "attemptTimeoutSeconds": [2, 2, 2],
    "backoffSeconds": [1, 2],
    "overheadSeconds": "0.5",
    "worstCaseSeconds": "9.5",
    "deadlineSeconds": "8"
  },
  "coverageBounds": {"topology": "sequential", "deadlineEnforced": false},
  "evidenceRefs": ["<calculation-and-source-evidence>"],
  "affectedGate": "G3"
}
```

Production execution còn timestamps và current source proof; placeholder không được import như actual check. Finding phải dẫn tới cả SRS deadline và TDD policy anchors, ghi remediation owner Dev/SA và route redesign; nếu muốn đổi deadline thì upstream CR/SRS review, không auto-update requirement từ budget.

## 6. Coverage example không tăng phần trăm giả

Expected obligations=10: N/A có evidence=1; applicability unknown=1; applicable=8. Trong applicable: satisfied=5, violated=1, unresolved=2. Hai unresolved có thể đồng thời reason unsupported và stale, không cộng thành 4 obligations.

Ratio design satisfaction=5/8=62.5%, kèm one unknown-applicability và three applicable obligations chưa satisfied. Gate vẫn blocked/failed; không báo 5/5=100% bằng cách bỏ missing/violated/unknown. CR acceptance là report khác, chưa thể tính từ design ratio.

## 7. Competency queries để nghiệm thu hệ thống

| Query ID | Câu hỏi | Kết quả tối thiểu |
|---|---|---|
| CQ-01 | Knowledge nào chỉ là extracted candidate, chưa có authority? | Assertion IDs, sources, state, missing reviews |
| CQ-02 | Bộ tài liệu baseline B có scope/constant/business rule nào mâu thuẫn? | Both endpoints, overlapping scope, witness hoặc unresolved reason |
| CQ-03 | Criterion C còn thiếu gì để nghiệm thu? | Requirement/design/spec/test/evidence/BA gaps theo revisions |
| CQ-04 | Sửa rule R ảnh hưởng những thiết kế và consumers nào? | Complete closure, reason paths, unknown external dependencies |
| CQ-05 | Section đã đổi số, mappings nào stale và mappings nào rebind an toàn? | Before/after anchors, equivalence review, dependent decisions |
| CQ-06 | Interface removal có còn consumer chưa migrate? | Consumer inventory, compatibility design, blockers |
| CQ-07 | TDD no_change có thật sự đủ cho config mới? | Allowed domain, effective value, guarantee checks, approval |
| CQ-08 | Vì sao change item này được phép triển khai? | CR authority, SRS intent, actual TDD, SAD conformance, gates |
| CQ-09 | Test pass này chứng minh criterion nào trên build nào? | Assertion-level mapping, expected/observed, build/config/environment |
| CQ-10 | Ai duyệt và đã duyệt payload nào tại thời điểm nào? | Actor/role validity, hash, revision, scope và evidence |
| CQ-11 | Hai CR đang sửa cùng requirement có merge được không? | Semantic facet overlap, scope proof, conflicts và revalidation |
| CQ-12 | Tại thời điểm T, dự án biết và chấp nhận điều gì? | Bitemporal baseline reconstruction, không latest substitution |
| CQ-13 | Adapter làm mất thông tin nào? | Canonical field/obligation → artifact anchor/loss classification |
| CQ-14 | Vì sao validator trả unknown? | Unsupported semantics/missing source/bounds, owner và next action |
| CQ-15 | Code/runtime đang khác normative design ở đâu? | as-specified/as-designed/as-built/as-operated diff có provenance |

## 8. Ranh giới tối thiểu trước triển khai

Các bước cần làm để chuyển thiết kế thành hệ thống có thể tin cậy:

1. Expand data dictionaries thành versioned Type/Property/Relation definitions, compile schemas/shapes và consistency checks.
2. Hiện thực immutable revision/source/evidence stores và atomic baseline registry; authority integration dùng danh tính thực.
3. Author executable supported fragments của expression/constraint engine; unsupported phải đi đường unknown/review đã thiết kế.
4. Implement ingestion/reconciliation với source fidelity fixtures; không chỉ load JSON mẫu.
5. Implement mappings, coverage inventory, semantic checks và impact invalidation.
6. Integrate G0–G6/GA và authoring preflight; check callers không bypass theo compatibility path.
7. Implement two adapters và end-to-end fixtures; migration runtime hiện có có loss report.
8. Human domain reviews và acceptance trên tài liệu thật; source-level correctness không thể được xác nhận từ catalogue rule alone.

Không ghi ontology “đã đảm bảo toàn vẹn” trước khi các bước này có actual evidence. Bản thiết kế đảm bảo mỗi loại vấn đề có nơi biểu diễn, rule/obligation kiểm, owner xử lý và gate chặn; độ đúng của implementation phải được kiểm chứng riêng.

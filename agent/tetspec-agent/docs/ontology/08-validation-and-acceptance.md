# 08 — Validation contract và bộ nghiệm thu

## 1. Phạm vi cam kết

Bộ ontology phải phát hiện hoặc biểu diễn rõ thiếu thông tin/mâu thuẫn trong phạm vi đã khai báo. Không tuyên bố chứng minh mọi nghiệp vụ bằng schema hay LLM. Integrity có ít nhất sáu chiều: source fidelity, structural integrity, domain logic, cross-domain consistency, workflow authority, executed evidence. Mỗi chiều có kết quả riêng.

## 2. Validation architecture đề xuất

| Lớp | Kiểm gì | Cách thực hiện dự kiến |
|---|---|---|
| L0 Source | Bytes/hash/anchors/layout và complete inventory | Parser/resolver + source audit |
| L1 Shape/type | Required fields, datatypes, cardinality, domain/range, enum | JSON Schema hoặc graph/relational constraints |
| L2 Identity/graph | References, revision selection, DAG/SCC theo relation, membership | Deterministic graph checks |
| L3 Domain semantics | Predicate satisfiability, rule precedence, values, permissions, budgets | Typed expression evaluator, finite checking, solver/model checks trong supported fragment |
| L4 Integration | Mapping obligations, refinement, composition, cross-domain conflicts | Typed mappings + specialist checks + reviewed semantic evidence |
| L5 Workflow/evidence | Approval/freshness/realization/publication/test/acceptance | State machine + evidence verifier + policy engine |

OWL/RDF có thể biểu diễn conceptual semantics; không dùng open-world inference thay completeness gate. JSON Schema biểu diễn shape không đủ chứng minh behavior. Một executable model cần registry nguồn duy nhất, generates/reconciles các projections; namespace/type/constraint drift giữa representations phải fail. Việc chọn công nghệ là bước implementation, không điều kiện phải dùng graph database.

## 3. RuleDefinition contract

Fields MUST có:

- `id`, `version`, `owningModule`, `description`, `sourceSpecificationRef`.
- `subjectTypes`, `inputProperties`, `relationDependencies`, `requiredInventory`.
- `applicabilityPredicate`, `requiredAtStages`, `severity`, `waivability`.
- `constraintKind`, `typedExpressionOrProcedureRef`, `supportedSemantics`, `resourceBounds`.
- `expectedEvidenceKinds`, `reviewRequirement`, `positiveFixtures`, `negativeFixtures`, `unknownFixtures`.
- `invalidationTriggers`, `explanationTemplate`, `remediationOwner`, `compatibilityVersion`.

Rule catalog JSON đi kèm hiện chỉ là **index thiết kế**: ID, statement, method, gate, source. Những field implementation như executable predicate, input dependencies và fixtures cần được author/validate trước rule được đăng ký supported. Không gắn `implemented=true` từ việc có row trong catalog.

Severity: blocker/error/warning/info. Core mandatory invariants không waive. Warning editorial có thể không chặn nếu không ảnh hưởng semantics và được policy tách riêng. Risk acceptance không waiver nguồn/approval/trace/evidence mandatory; nếu muốn thay requirement phải sửa nguồn bằng đúng authority/workflow.

## 4. ValidationExecution / Finding contract

`ValidationExecution = {id, ruleId, ruleVersion, engineVersion, inputBaselineDigest, subjectRefs, scope, applicability, startedAt, endedAt, executionStatus, evaluationOutcome, evidenceRefs, witness, coverageBounds, diagnostics, reviewerReceipt?}`.

`Finding = {id, ruleRef, subjectRefs, sourceAnchors, category, severity, observed, expected, overlapScope, witnessOrUnknownReason, affectedGates, remediationOwner, suggestedRoute, resolutionRefs, status}`.

Finding status open/acknowledged/resolved/superseded; acknowledged không tương đương resolved. Một finding sửa bằng upstream revision phải cite revision mới và rerun result; không tick checkbox “resolved” để pass.

Execution status completed/unsupported/error/not_run/stale; evaluation pass/fail/unknown/not_applicable. Not-applicable có applicability proof/review. Unsupported guard proof nhưng shape pass phải có hai executions riêng; không lấy pass của shape thay semantic check.

## 5. Gate aggregation có thể kiểm toán

Cho gate g, baseline B, policy P:

1. Tính expected obligations `O(B,P,g)` từ đầy đủ source/inventory/scope và applicable profiles, không từ matched nodes.
2. Resolve applicability từng obligation: applicable/N/A/unknown; lưu evidence N/A. Unknown mandatory là blocker.
3. Tính required rules/reviews cho mỗi obligation; missing checker/unsupported method sinh execution unresolved.
4. `usable(e,B)` khi input/rule/profile/subject digests và evidence applicability khớp B, execution completed, kết quả có đủ bounds/evidence.
5. Gate pass khi tất cả applicable mandatory obligations có usable passing checks và required human reviews; không unresolved blocker; inventory/closure complete; prerequisites valid.
6. Gate failure khi có definitive violation; gate blocked/unknown khi thiếu evidence hoặc applicability chưa rõ. Cả hai đều không cho downstream action.

Không vacuous pass: `O=empty` chỉ hợp lệ khi explicit scoped no-op/disposition hoặc gate không applicable được duyệt; báo N/A/no-op, không 100% verified. Global prerequisites nguồn/authority tồn tại ngay cả khi business obligation set rỗng.

G0 intake, G1 requirements, GA architecture, G2 impact, G3 actual TDD, G4 CS, G5 verification, G6 acceptance. `ALL` trong catalog là integrity constraint tại mọi gate tiêu thụ đối tượng liên quan. Rule có gate đầu tiên không cần mọi input downstream ở bootstrap; ví dụ CR acceptance rule chỉ bắt buộc G6.

## 6. Coverage report contract

Báo cáo mỗi domain và mỗi mapping stage:

`{baseline, scope, expectedCount, applicableCount, notApplicableCount, unknownApplicabilityCount, satisfiedCount, violatedCount, unknownCount, missingCount, staleCount, unsupportedCount, items[], blockedGates[]}`.

Counts thực thi như missing/stale/unsupported có thể là dimensions chồng lên outcome unknown; report MUST khai rõ chúng không cộng trực tiếp thành denominator. Primary partition của expected obligations: N/A, applicability-unknown, applicable. Applicable partition: satisfied, violated, unresolved. Unresolved có reason codes đa trị. Weighted metrics chỉ bổ sung, không thay all-mandatory gate.

Coverage ratio nếu dùng: satisfied / applicable; report cùng unknown-applicability và mandatory unresolved counts. Không có applicable thì ratio=null, không 1. Criterion partial coverage luôn tách outcome-level obligations; item/test count không đủ đo semantic coverage.

## 7. Profiles và extensibility

Profile có namespace/version, domain types/rules, trigger predicates, required knowledge, validators, applicability proofs và fixture suites. Core obligations tích lũy với specialist obligations. Composition conflict (ví dụ retention≥3 năm và purge≤1 năm cùng scope) phải phát hiện trước ready; không chọn profile ưu tiên âm thầm.

Profile mới: xác định domain extension ownership → types/relations/units → trigger/obligations → supported rule semantics → positive/negative/unknown fixtures → compatibility checks → reviewed registry release. Không buộc core biết hết finance/health/logistics/scientific domains. Custom business rules phải có source authority; ontology chỉ cung cấp khả năng biểu diễn/kiểm, không tự sinh chân lý nghiệp vụ.

Migration schema version: pinned source/target, explicit field mappings/default policy, loss report, round-trip/rollback plan và revalidation. New required property không auto-fill guessed values. Legacy aliases chỉ normalize khi mapping không ambiguous; conflicting aliases giữ conflict.

## 8. Bộ scenario nghiệm thu thiết kế

Các scenario sau là **đặc tả tests chưa chạy**, không phải test results. Fixture implementation cần source bytes, domain revisions, mappings, expected findings và gate decisions. Mỗi positive phải có negative mutation tối thiểu để chứng minh checker không pass hình thức.

| Case | Input / trigger | Expected result |
|---|---|---|
| AT-01 | CR đầy scope/criteria/authority, SRS/SAD consistent, mixed add/update/no_change TDD và actual realization | G0–G4 pass với sources/evidence đúng |
| AT-02 | Thiếu CR criterion nhưng có đủ CS items | G0/G6 blocked; criterion denominator không từ CS subset |
| AT-03 | Criterion chỉ “nhanh hơn” | CR-003 gap; không tự điền threshold |
| AT-04 | Table có header ms, parser coi cell 3000 là seconds | FND-005/FLOW-002 fail hoặc unresolved extraction |
| AT-05 | Footnote “trừ admin” bị bỏ khi extract permission rule | MAP-013 fidelity fail |
| AT-06 | OCR mất từ “không” | Candidate uncertainty/source audit chặn promote |
| AT-07 | Same label Customer ở hai bounded scopes có khác definition | Không merge; glossary/alignment review |
| AT-08 | MUST allow và MUST_NOT allow cùng context feasible | SRS-003 fail với witness |
| AT-09 | Hai rule giống predicate nhưng environments disjoint đã chứng minh | Không false conflict; separate scopes giữ |
| AT-10 | Unique decision table có overlapping rows | SRS-006 fail với input witness |
| AT-11 | Complete table bỏ sót boundary value | SRS-006 fail/gap |
| AT-12 | Null logical field bị physical NOT NULL nhưng required null input hợp lệ | TDD-011/MAP-004 fail |
| AT-13 | SAD service A không được gọi external, TDD thêm outbound call | M04 conformance fail, SAD CR hoặc redesign |
| AT-14 | ADR accepted nhưng mâu thuẫn ADR accepted khác cùng element | SAD-006 fail, không newest-wins |
| AT-15 | Runtime/deployment/security views không có feasible path đã claim | SAD-008 fail; abstraction scope xét trước |
| AT-16 | Retry 3 attempts, per-attempt 2s, backoffs 1s+2s, overhead 0.5s, deadline 8s, không cap/cancel | Worst case 9.5s; TDD-008 fail |
| AT-17 | Timeout design range 1–10s, đổi 5→3 nhưng làm retry guarantee sai | Không no_change; reassess update |
| AT-18 | Same change 5→3, guarantees đã chứng minh đúng, actual TDD giữ revision | no_change hợp lệ, CS config vẫn dual-source |
| AT-19 | Idempotency có key nhưng hai concurrent writes có thể commit duplicate | TDD-009 fail dù shape pass |
| AT-20 | Saga có irreversible external effect nhưng compensation ghi “rollback DB” | TDD-013 fail, cần recovery design |
| AT-21 | Migration stages cycle hoặc không tương thích old readers | TDD-014 fail |
| AT-22 | FSM structurally reachable, arbitrary guard checker không hỗ trợ | execution unsupported, G3 blocked; không lấy reachability pass |
| AT-23 | p95 service components cộng thành p95 end-to-end không distribution model | MAP-014/TDD-017 unknown/fail model claim |
| AT-24 | Đủ sources nhưng impact approval thiếu hoặc agent tự approve | G3/G4 blocked; không invoke CS authoring |
| AT-25 | Approved mixed plan thiếu một added TDD actual | FLOW-009 fail; G4 toàn scope bị chặn |
| AT-26 | TDD update đúng approved plan | Approval plan giữ applicable, G3 kiểm new actual revision |
| AT-27 | SRS/SAD đổi sau impact approval | Approval stale; quay G2 |
| AT-28 | CS có refs hợp lệ nhưng outcome trái SRS | CS-003 fail; ID existence không đủ |
| AT-29 | Requirement remove, CS vẫn dùng old TDD không có decommission design | M09/M19 fail hoặc missing actual design |
| AT-30 | Split requirement không chuyển một negative obligation | SRS-013/MAP-010 gap |
| AT-31 | Defect code sai nhưng SRS đúng và unchanged | Authorized discrepancy flow hợp lệ, không fake modify |
| AT-32 | Refactor không delta, có unchanged SRS binding/CR intent và design | Context alternative hợp lệ, vẫn G2/G3/G4 |
| AT-33 | Test pass nhưng oracle khác criterion | VER-002/CS-007 fail |
| AT-34 | Test plan có đầy nhưng chưa chạy | G5/G6 pending; không verified |
| AT-35 | Test pass trên old config, new config đổi behavior | Evidence stale; G5 rerun/reassess |
| AT-36 | Một test pass, mandatory second test skipped | VER-003 blocked |
| AT-37 | BA receipt đúng criterion nhưng CR revision đã đổi nghĩa | VER-006 stale, cần review mới |
| AT-38 | Hai CR cùng requirement, receipt chỉ cover một CR scope | CR còn lại không accepted |
| AT-39 | Source restricted/missing khỏi pack | FLOW-001/FND-015 unknown, vẫn trong denominator |
| AT-40 | Closure giới hạn depth bỏ external consumer | MAP-009 blocked/incomplete |
| AT-41 | Hai publish transactions cùng expected head | Một commit; một conflict, không partial overwrite |
| AT-42 | Document đổi số section nhưng không đổi nghĩa | Explicit anchor rebind + equivalence, không silent ref rewrite |
| AT-43 | OpenSpec archive/task checked thiếu receipts | CS-009 không import authority |
| AT-44 | Export OpenSpec giữ requirements nhưng làm mất exception | CS-008 blocked; loss report có canonical facet |
| AT-45 | Adapter JSON/Markdown khác giữ cùng canonical semantics/trace | Fidelity pass không phụ thuộc OpenSpec |
| AT-46 | Profile mới yêu cầu rule chưa supported | FND-016/VAL-003 blocked, không N/A |
| AT-47 | Legacy approved label thiếu evidence | FLOW-006 unverified, historical claim giữ riêng |
| AT-48 | Incident hotfix đã có trước authorization | Deviation/corrective flow; không backdate approval |
| AT-49 | Obligation inventory empty do ingestion lỗi | Không vacuous ready; FLOW-001/VAL-002 fail |
| AT-50 | CR cancel sau implement | CR-009 requires effects/corrective disposition, history còn |

## 9. Validation rules

| ID | Invariant / điều kiện | Evaluation | Gate |
|---|---|---|---|
| VAL-001 | Rule execution pin subject/input/rule/profile/engine; stale/unsupported/error không pass | execution record validation | ALL |
| VAL-002 | Gate denominator đến từ complete expected obligations; empty scope cần explicit no-op/N/A authority | obligation set reconciliation | ALL |
| VAL-003 | Required checker unsupported hoặc missing tạo blocker, không tự downgrade warning/N/A | support registry check | ALL |
| VAL-004 | Semantic assertion ngoài formal fragment cần review evidence có scope, không confidence-to-proof | proof capability check | ALL |
| VAL-005 | Findings resolved chỉ bằng new evidence/revision và successful rerun, không acknowledgement | finding lifecycle check | ALL |
| VAL-006 | Profile composition không waive core và phải phát hiện constraint conflicts | profile solver/review | ALL |
| VAL-007 | Graph/schema/adapter artifacts giữ manifest/type/constraint consistency | generated artifact reconciliation | ALL |
| VAL-008 | Migration không mất normative content/trace/history và không auto-fill unknown required facts | migration loss/round-trip checks | ALL |
| VAL-009 | Mandatory rule có positive/negative/unknown fixtures trước supported registration | validator qualification | ALL |
| VAL-010 | Coverage counts phân biệt primary partitions và overlapping diagnostics; không inflated percent | report arithmetic audit | ALL |

## 10. Điều kiện nghiệm thu implementation sau này

- Source ingestion: fixtures với text/table/diagram/negation/units/exceptions có source-to-graph và graph-to-source audit.
- Domain correctness: mandatory constraints của tất cả dictionaries và rules applicable được thực thi; không chỉ class-name recognition.
- Mapping correctness: many-to-many contributions, refinement, constants, rules, quality/security và removed items được kiểm bằng counterexamples.
- Workflow correctness: bootstrap, onboarding, normal change, defect no-delta, architecture CR, concurrent changes, cancellation có end-to-end evidence.
- Evidence correctness: stale/wrong-build/missing-assertion/unauthorized-receipt negatives đều chặn.
- Portability: ít nhất hai adapters cùng canonical fixtures và semantics; OpenSpec không quyết định readiness.
- Human review: BA/SA/Dev rà soát rubric semantic fidelity, completeness, consistency, trace, explainability và applicability; agent không tự chấm human acceptance.
- Migration: giữ frozen historical snapshots, loss report và rollback; runtime regression riêng không thay end-to-end acceptance.

Không có số lượng rule/test nào tự chứng minh ontology hoàn chỉnh cho mọi ngành. Extensibility được nghiệm thu bằng domain profile mới có nghĩa vụ chưa có trong bộ ví dụ, giữ nguyên core gates và trả unknown đúng khi checker chưa hỗ trợ.

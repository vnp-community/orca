# 02 — SRS ontology: requirement và logic nghiệp vụ

SRS sở hữu nghĩa vụ hệ thống và business semantics được chấp nhận. Không đồng nhất business entity của ứng dụng với ontology entity quản trị tài liệu: `srs:LogicalEntity("Invoice")` mô tả khái niệm Invoice, không phải hóa đơn cụ thể trong production.

## 1. Data dictionary

| Entity | Fields | Quan hệ/cardinality và constraints |
|---|---|---|
| RequirementSet | `scope! owner! baselineRef? completenessClaim!` | `members RequirementRevision+`; expected inventory riêng |
| RequirementRevision | `statement! modality! requirementKind! applicability! priority! verificationMethod! owner!` | `originAssertions+`, `refines*`, `dependsOn*`, `conflictsWith*` |
| FunctionalRequirement | `subject! trigger? precondition! requiredEffects+ prohibitedEffects* postcondition!` | Invariant requirement có trigger N/A reason; inherits RequirementRevision |
| QualityRequirement | `qualityKind! metric! comparator! target! workload! environment! measurementWindow! method!` | Statistic khi metric phân phối; load model không được bỏ |
| BusinessRule | `ruleKind! variables* condition! conclusion! exceptions* authority!` | `constrains Requirement/DataConcept/Scenario+` |
| DecisionTable | `inputs+ outputs+ rows+ hitPolicy! defaultBehavior!` | unique/first/priority/collect; collect có aggregation semantics |
| DecisionRow | `inputPredicates+ outputAssignments+ priority?` | Thuộc 1 table revision; precedence explicit |
| ActorRole | `name! capabilities* restrictions*` | Khác foundation actor đã thực hiện review |
| UseCase | `goal! primaryActor! supportingActors* preconditions! postconditions!` | `scenarios+`, `requirements+` |
| Scenario | `kind! trigger! orderedSteps+ expectedOutcomes+ preconditions!` | positive/negative/alternative/boundary; normative subject refs+ |
| ScenarioStep | `position! performer! input? action! observableResult?` | Step order duy nhất, referenced concept resolve |
| LogicalEntity | `name! definition! identityRule! lifecycleRef?` | `attributes*`, `relationships*`, `invariants*` |
| LogicalAttribute | `name! meaning! logicalType! cardinality! nullSemantics! unit? allowedValues?` | 1 owner logical entity; physical type không bắt buộc |
| LogicalRelationship | `source! target! sourceMultiplicity! targetMultiplicity! ownership? deletionSemantics!` | Không suy FK cụ thể |
| DataConstraint | `scope! predicate! enforcementExpectation! exceptions*` | uniqueness, range, referential, temporal, aggregate |
| BusinessStateModel | `subject! states+ initialStates+ transitions+ terminalStates*` | Trạng thái nghiệp vụ, chưa quy định worker/thread |
| BusinessTransition | `from! to! event! guard! effect! actorConstraint?` | Có rule/requirement nguồn |
| ExternalInterfaceRequirement | `provider! consumers+ inputs! outcomes+ errorOutcomes* authExpectation! compatibility!` | HTTP code/path bắt buộc chỉ khi normative source yêu cầu |
| RequirementConstraint | `constraintKind! predicate! imposedBy! rationale!` | Business/design/regulatory/operational constraint; authority explicit |
| GlossaryBinding | `termRef! conceptRef! scope!` | Từ đồng âm khác nghĩa không merge |
| Assumption | `statement! owner! validationMethod! expiryOrTrigger! consequencesIfFalse!` | Không biến thành fact reviewed |
| RequirementDeltaSet | `beforeBaseline! afterBaseline! authorizedIntentRefs+ deltas+` | before có thể empty manifest khi bootstrap |
| RequirementDelta | `action! beforeRef? afterRef? changedFacets+ rationale! sourceAuthorityRefs+` | add/modify/remove; refs theo action |
| RequirementDiscrepancy | `normativeRef! observedEvidence+ deviationPredicate! affectedScope!` | Bug fix yêu cầu giữ nguyên có discrepancy, không giả semantic modify |

Các subtype chia sẻ RequirementRevision identity system nhưng distinct types; một requirement nhiều khía cạnh có thể multi-type nếu fields không xung đột. BusinessRule độc lập có identity, không copy câu luật vào từng FR rồi coi là nhiều luật khác nhau.

## 2. Rule logic và consistency

Modality core: MUST, MUST_NOT, SHOULD, MAY. SHOULD vẫn cần disposition khi không làm theo; MAY không tự chuyển thành implementation obligation nếu chưa được chọn vào scope. MUST_NOT có forbidden condition/effect; không chỉ lưu chữ “không” trong label.

`R = (scope, precondition, trigger, permittedInputs, requiredOutcomes, forbiddenOutcomes, modality)`. Đối chiếu hai R trên overlap scope và feasible preconditions. `requires(P)` và `forbids(P)` cùng context là conflict. Những yêu cầu ở environments/roles/time windows khác nhau không mặc định conflict. Không có proof overlap/disjoint thì unknown.

Rule types: derivation, constraint, authorization, calculation, eligibility, sequencing. Decision table unique phải chứng minh không overlap; complete table phải bao phủ declared input domain. first-hit có thứ tự là intentional nhưng vẫn cần review shadowed rows. Calendar, currency, rounding, precedence và exception specificity là semantics nghiệp vụ phải được lưu.

Ví dụ rule “giảm giá 10% cho khách VIP trừ hàng X” phải model VIP predicate, base amount, rate=0.10, excluded category, currency rounding và thời điểm eligibility. Không tự chọn thứ tự áp dụng nhiều ưu đãi; thiếu stacking/precedence tạo gap.

## 3. Contracts cho change và defects

Add: before=null, after present; remove: before present, after=null; modify: cả hai, cùng identity trừ declared replacement, có semantic facet diff. Text edit không tạo requirement semantic delta nếu đã xác nhận equivalent. Split/merge thông qua lineage với coverage transfer.

Với defect **không đổi yêu cầu mong muốn**, vẫn bắt buộc SRS authority: `RequirementDiscrepancy` pin requirement revision hiện tại, observed behavior và requested conformance restoration. Không làm giả before≠after. Đề xuất biểu diễn `SrsChangeContext = deltaSet? + discrepancies* + scopeBindings+`, có ít nhất một authorized delta/discrepancy hoặc explicit unchanged-requirement binding với authorized refactor intent. Refactor không có defect không cần tạo discrepancy giả. Đây là phần cần reconcile với runtime đang yêu cầu delta; không tự bypass G1/G2. Mọi CS cho defect vẫn có SRS section thực và actual TDD. Nếu expected behavior chưa rõ, BA clarification → requirement revision trước.

## 4. Lifecycle

`draft → in_review → approved → superseded | retired`. Required checks/review phải pass trước approved. Baseline requirement cũ không cần mang CR mới; change context của run phải chỉ ra lý do sử dụng/sửa. Retired requirement vẫn là before endpoint của removal. Rollback tạo revision/baseline event mới, không phục hồi trạng thái bằng xóa lịch sử.

## 5. Rule catalog

| ID | Invariant / điều kiện | Evaluation | Gate |
|---|---|---|---|
| SRS-001 | Mandatory requirement có modality, scope, actor/system subject và outcome có thể kiểm | shape + BA semantic review | G1 |
| SRS-002 | Compound requirement có atomic obligations/composition; không lấy một scenario thay toàn bộ requirement | decomposition coverage | G1 |
| SRS-003 | MUST và MUST_NOT không cho outcomes đối nghịch trong feasible overlapping context | constraint satisfiability / review | G1 |
| SRS-004 | Quality threshold có metric/unit/statistic/workload/window/environment phù hợp | measurement type check | G1 |
| SRS-005 | BusinessRule AST type-safe, authority resolve, exceptions/precedence không mơ hồ | typed logic + review | G1 |
| SRS-006 | Decision table hit policy đúng; unique không overlap; completeness claim có coverage proof | finite enumeration / solver | G1 |
| SRS-007 | Calculation xác định rounding/unit/currency/zero/null behavior khi applicable | expression checker | G1 |
| SRS-008 | Attribute ranges/cardinality/null constraints không mâu thuẫn và logical relationships resolve | data constraint satisfiability | G1 |
| SRS-009 | Business transitions dùng states hợp lệ, initial reachable scope, guards phù hợp rules và protected operations | graph + guard checks | G1 |
| SRS-010 | Scenario expected results phù hợp normative rules và permissions trong cùng context | scenario simulation / review | G1 |
| SRS-011 | Refinement graph acyclic; child không nới parent MUST nếu không có authorized upstream revision | refinement proof / review | G1 |
| SRS-012 | Every in-scope delta authorized và add/modify/remove before/after đúng; no phantom changes | delta reconciliation | G1 |
| SRS-013 | Split/merge/removal giữ criteria coverage và negative/removal obligations | lineage coverage | G2 |
| SRS-014 | Interface requirement đủ outcomes/error/auth/compatibility theo risk; unknown không tự default | contract completeness | G1 |
| SRS-015 | Assumption có validation owner/trigger; critical unresolved assumptions giữ blocker tại gate cần chứng minh | applicability policy | G1 |
| SRS-016 | Business constants/defaults đồng nhất across sections hoặc scoped explicitly | binding/conflict check | G1 |
| SRS-017 | No semantic delta defect có discrepancy và unchanged SRS refs; không giả add/modify để đủ schema | context case check | G1 |
| SRS-018 | Expected requirement inventory gồm mọi nguồn/phụ lục/bảng/negative clauses; omission giữ gap | source-to-entity audit | G1 |

## 6. Câu hỏi competency

Requirement nào không có thẩm quyền CR/URD? Hai business rules xung đột với input cụ thể nào? Table có case bỏ sót không? Đơn vị/giá trị mặc định nào bị khác giữa các sections? Removal còn consumer hoặc acceptance criterion nào chưa được xử lý? Defect sửa implementation hay thay expected requirement?

# 06 — Mapping, thống nhất knowledge và propagation

Mapping là assertions được quản lý độc lập với source/target domains. Nó không chuyển ownership; không suy `sameAs` chỉ vì tên giống nhau. Các revision đang xét phải cùng baseline hoặc có explicit baseline bridge được kiểm.

## 1. MappingAssertion contract

| Field | Type/cardinality | Nghĩa |
|---|---|---|
| id, revision, mappingType | Identifier! mỗi field | Identity/version riêng của mapping |
| sourceRefs, targetRefs | EntityRevisionRef+ | Endpoints có role tên rõ, không positional ambiguity |
| sourceAnchors, targetAnchors | SectionAnchor* | Mandatory khi endpoint được dùng làm nguồn tài liệu ở G4 |
| context | MappingContext! | project, baseline, scope, validTime, environment, variants |
| rationale | Text! | Vì sao quan hệ đúng |
| obligations | MappingObligation+ | Điều gì phải chứng minh để mapping usable |
| contributions | Contribution* | Endpoint đóng góp phần nào, composition |
| evidenceRefs | Evidence* | Mandatory trước validated, trừ fully computed proof có execution evidence |
| derivation | Derivation? | Inputs/rule/assumptions nếu generated |
| assessmentState | Enum! | proposed/validated/rejected/stale |
| reviewRequirement | PolicyRef! | Actor/checks cần thiết; inference không tự reviewer |
| assessedBy, assessedAt | ActorRef?, Instant? | Mandatory khi assessed |
| contractVersion, inputDigest | Version!, Hash! | Pinned signature/rules và source snapshot |

`MappingObligation = {id, predicateOrQuestion, scope, requiredEvidenceKind, validatorRef, applicability, status}`. `Contribution = {endpointRef, obligationIds, compositionGroup, contributionKind}`. `MappingContext` pin effective scope; source/target properties không cần giống nhau mọi mặt, chỉ nghĩa quan hệ đòi hỏi.

## 2. Signatures và cardinality

Cardinality trên từng mapping instance; global coverage theo obligation inventory riêng. M01–M14 giữ ý nghĩa của bản thiết kế tổng; M08 bổ sung discrepancy case có guard để hỗ trợ defect không đổi requirement.

| Mapping | Source endpoints → target endpoints | Cardinality | Required semantic obligations |
|---|---|---|---|
| M01 operationalizedBy | CR Criterion → SRS RequirementRevision | 1 → 1..* | Scope/observable outcome/threshold/negative obligations được cụ thể hóa đầy đủ |
| M02 authorizedBy | SRS Delta hoặc Discrepancy → CR revision và scope/criteria | 1 → CR 1..*, criterion/scope 1..* | Thay đổi/restoration trong authorized intent; technical enabler có derivation |
| M03 motivates | SRS requirement → SAD driver/scenario/decision | 1..* → 1..* | Architecture significance, tradeoffs không ngược upstream MUST |
| M04 constrains | SAD constraint/decision → SRS requirement hoặc TDD design | 1 → 1..* | Applicability đúng, design/requirement feasible trong giới hạn kiến trúc |
| M05 addressedByDesign | SRS requirement → TDD design revisions | 1..* → 1..* | Mọi obligation trong scope có thiết kế, failure/negative cases không rơi |
| M06 allocatedTo | TDD unit/element → SAD architecture element | 1..* → 1..* | Responsibilities, boundaries, data ownership và trust phù hợp |
| M07 refinesContract | TDD interface/data/behavior contract → SRS interface/data/behavior | 1..* → 1..* | Inputs, outputs, units, semantics và admissible behavior preserved |
| M08 implementsDelta | CS item → SRS delta hoặc discrepancy và requirement endpoints | 1 → intent 1..*, requirement 1..* | Action/effect đúng change intent; discrepancy giữ requirement revision |
| M09 realizesDesign | CS item → actual TDD revisions | 1 → 1..* | Design hiện thực sau approved plan, sections resolve và đủ cho target |
| M10 verifiedBy | CS verification criterion → test/manual verification specification | 1 → 1..* | Oracle/method kiểm đúng expected outcome trên đủ scope |
| M11 acceptanceDerivedFrom | CS verification criterion → CR criteria | 1 → 1..* | Chứng minh đóng góp tới business acceptance, có derivation cho enabler |
| M12 implementsItem | implementation artifact revisions → CS items | 1..* → 1..* | Diff thực nằm trong approved item, không thêm/thiếu behavior |
| M13 evaluates | execution → test spec + build/config manifest | 1 → test 1, build 1 | Actual inputs/environment/assertions đủ applicability |
| M14 acceptsCriterion | acceptance receipt → CR criteria | 1 → 1..* | Human authority, scope, đúng revision và evidence đủ |
| M15 bindsConstant | downstream value/binding → upstream constant/constraint | 1 → 1..* | Value/dimension/effective precedence không trái upstream bounds |
| M16 refinesRule | TDD algorithm/decision table → SRS business rules | 1..* → 1..* | Equivalent required outputs và forbidden outcomes trên input domain |
| M17 allocatesQuality | SAD/TDD budget model → SRS quality requirement | 1..* → 1..* | Aggregation assumptions/topology/units/workload đúng; budget không proof achievement |
| M18 preservesInvariant | TDD invariant/enforcement → SRS data/business invariant | 1..* → 1..* | Enforcement bao mọi relevant mutation path, concurrency/partial failure |
| M19 decommissions | CS removal/design retirement → obsolete requirement/design/interface | 1..* → 1..* | Consumers, retention, compatibility và negative checks được xử lý |
| M20 correspondsToSource | semantic revision/assertion → document anchors | 1 → 1..* | Statement fidelity, modality, exception, quantities và authority preserved |

Mỗi instance M01–M20 phải resolve các type/field/cardinality ở signatures này. Nếu mapping type mới xuất hiện, integration profile phải đăng ký signature, semantics, obligations, validators và migration policy trước khi dùng tại gate. Một generic `relatedTo` có thể dùng cho tìm kiếm nhưng không chứng minh bất kỳ coverage obligation nào.

Mapping ngược chỉ là query view, không tạo second assertion với semantics khác. Một consumer có thể query `designedFor` bằng inverse M05 nhưng vẫn trỏ cùng mapping ID. M03 không bắt mỗi FR có ADR; áp dụng architecture significance decision. M05 không bắt non-software CR criterion có TDD; criterion đó vẫn có acceptance path riêng.

## 3. Refinement proof obligations

Đối với behavior contract B có accepted inputs I, preconditions P, outcomes O và forbidden outcomes F:

- Downstream phải hỗ trợ toàn bộ **required upstream inputs trong mapped scope**. Không được thêm precondition làm loại caller hợp lệ nếu chưa được upstream cho phép.
- Với mỗi input hợp lệ và context khả thi, mọi observable downstream outcome phải nằm trong upstream allowed outcomes và bảo đảm required postconditions; không tạo outcome thuộc F.
- Nondeterminism/underspecification giữ đúng upstream choice space. Không suy ra mọi allowed behavior đều required.
- Error, timeout, permission và side effects là observable contract nếu upstream coi chúng là nghĩa vụ.
- Internal constraints có thể mạnh hơn chỉ khi không làm mất required behaviors hoặc mâu thuẫn quality obligations.

Formal check có thể dùng entailment/counterexample trong supported expression model. Ngoài fragment, lưu review question + citations + human conclusion, không tuyên bố proof tự động.

Đối với logical→physical data: có transformation f và inverse/projection g khi cần; kiểm identity preservation, allowed ranges, null semantics, precision, timezone, encoding, cardinality, references và deletion/retention. Many-to-many mappings như một logical field tách nhiều columns phải có composition/extraction expression, không ép one-to-one.

Đối với quality: so metric definition/statistic/population/window/environment; threshold quan hệ theo comparator. Giá trị latency nhỏ hơn tốt hơn không áp dụng cho throughput. p95 của nhiều components không được cộng rồi gắn nhãn p95 end-to-end nếu không có model hợp lệ.

## 4. Coverage obligations và composition

Tạo `CoverageObligation` từ nguồn normative đầy đủ, trước khi tìm matches. Fields: id, originatingRevision, facet, requiredStage, scope, decomposition, applicability, owner. Facets có thể là input case, outcome, invariant, quality metric, security restriction, failure/removal behavior. Matcher chỉ đề xuất mappings; không quyết định denominator.

All-of group chỉ satisfied khi mọi member applicable pass; any-of chỉ khi source authority cho phép alternatives và ít nhất một chứng minh đầy đủ; threshold group dùng formula có source, không tự coi 80% là đủ. N/A không được tính như pass mà trình bày separately. Một mapping M05 cho 3 requirements có contribution matrix để tránh thiết kế chỉ đáp ứng 1/3 nhưng mark cả 3 covered.

Trace completeness gồm hai chiều: upstream nghĩa vụ nào chưa được đáp ứng; downstream fact/item nào không có justification hoặc là unapproved addition. Technical derivation hợp lệ vẫn phải có bounded purpose và evidence, không lấy bất kỳ CR criterion nào nối tượng trưng.

## 5. Conflict model

| Kind | Phát hiện | Hướng giải quyết |
|---|---|---|
| Identity | Cùng ID khác nghĩa; khác ID cùng candidate concept | Entity reconciliation review; giữ aliases có scope |
| Terminology | Cùng từ khác nghĩa trong domain/scope | GlossaryBinding, không global string replace |
| Value/unit | Threshold/default/range lệch | Normalize có provenance rồi so effective bindings |
| Logical | P và not-P, guards overlap, impossible postcondition | Witness/counterexample; domain owner sửa nguồn |
| Authority | Proposal/prompt đè normative baseline | Quay owner của CR/SRS/SAD; không “chọn mới nhất” |
| Temporal/variant | Một source outdated hoặc khác environment | Scope/time reconciliation; không tự coi disjoint khi thiếu scope |
| Structural | Missing target, broken anchor, untyped relation | Source/ref repair và revalidate |
| Coverage | Source obligation thiếu design/spec/test hoặc downstream orphan | Add knowledge qua approved workflow, không auto-invent |
| Evidence | Pass record cũ, wrong build, source inaccessible | Rerun/renew evidence đúng scope |

`ConflictResolution` pin conflict refs, chosen action, new revisions/mappings, rationale, actor và gate impact. Resolution không xóa các assertions bị bác; đánh dấu retracted/superseded và retain source.

## 6. Dependency propagation

Mỗi property/entity/mapping/rule có dependency edges xác định: `reads`, `derivesFrom`, `constrains`, `consumes`, `proves`, `approvedAgainst`. Change impact theo facets thay đổi, không chỉ node adjacency.

Pseudocode thiết kế:

1. Pin before/after inventories; compute semantic delta và changed facets; include removed nodes, changed assumptions, profile changes.
2. Seed affected obligations/refs; traverse reverse dependencies và consumer edges đến fixed point; record reason/path cho từng candidate.
3. Với mỗi candidate, evaluate affected/unaffected/unknown; unaffected cần chứng minh scope disjoint hoặc facet independent.
4. So lại toàn inventory TDD và obligation inventory; phần không có match vẫn là explicit assessment, không mặc định no_change.
5. Mark affected evidence/mappings/gates stale; immutable historical reports giữ nguyên. Tạo new review obligations.
6. Chặn nếu closure truncated, external contracts unavailable hoặc dependency uncertainty bắt buộc unresolved.

Cycles trong runtime dependencies có thể hợp lệ; traversal dùng visited (revision, facet, scope) để terminate và SCC summary. **Không** áp DAG constraint chung lên service dependencies. DAG chỉ bắt buộc với lineage, proof support không tự hỗ trợ, stage/task execution khi semantics yêu cầu ordering.

## 7. Rule catalog

| ID | Invariant / điều kiện | Evaluation | Gate |
|---|---|---|---|
| MAP-001 | Mapping signature/domain/range/cardinality đúng contract và endpoints pin revisions | typed resolver | G1 |
| MAP-002 | Mapping validated có obligations/evidence đủ, source/target scopes compatible | evidence/scope evaluation | G2 |
| MAP-003 | Không suy sameAs/transitivity/coverage từ label hoặc trace path tồn tại | inference policy check | ALL |
| MAP-004 | Refinement không loại valid input hay nới forbidden outcome trong required scope | contract entailment / review | G3 |
| MAP-005 | Constants/rules/units/thresholds đồng nhất hoặc scoped authorized transformation | binding/refinement checks | G3 |
| MAP-006 | Many-to-many có contributions/composition đủ; không mark all covered từ một partial match | obligation matrix check | G3 |
| MAP-007 | Delta/criteria denominator xuất phát complete inventories, không từ matched subset | inventory coverage audit | ALL |
| MAP-008 | Source revision/assumption/profile đổi làm dependent mappings/evidence stale theo facets | dependency invalidation | ALL |
| MAP-009 | Impact closure đầy đủ và reason paths lưu; truncated/unresolved mandatory dependencies chặn | fixed-point completeness | G2 |
| MAP-010 | Removal/split/merge xử lý consumers, residual obligations và tombstones | lineage/consumer analysis | G3 |
| MAP-011 | Cross-domain conflicts do đúng owner giải quyết và decision pin payload, không silent overwrite | resolution authority check | ALL |
| MAP-012 | Actual TDD mappings không dùng planned/baseline stale sections làm realized evidence | realization resolver | G4 |
| MAP-013 | Source↔semantic fidelity giữ negation, modality, exceptions, numbers và conditions | bidirectional source audit | G1 |
| MAP-014 | Quality/data/security mappings chứng minh theo semantics chuyên biệt, không generic similarity | specialist validators / review | G3 |

## 8. Knowledge views cần cung cấp

- Project document integrity: documents/sections chưa số hóa hoặc anchors lỗi; authority/version conflicts.
- Requirements coverage: criterion→requirement→design obligations, missing và contradictory.
- Architecture conformance: design placements, trust/data paths và violated constraints.
- Change blast radius: affected sources/design/consumers/spec/tests/approvals với reason paths.
- Delivery trace: item→build→assertions→criteria→receipt, đúng snapshot.
- Knowledge debt: assumptions/gaps/unknown checks, owners và gates bị chặn.
- Historical reconstruction: bộ knowledge được biết và được duyệt tại một thời điểm, không dùng latest.

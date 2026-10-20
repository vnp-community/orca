# 07 — Số hóa, bootstrap và quản lý thay đổi

## 1. Phân biệt các graph và stores

| Logical store | Nội dung | Quyền promote |
|---|---|---|
| Raw source vault | Original bytes, document versions, OCR/layout artifacts và access policy | Ingestion ghi bất biến |
| Candidate graph | Extracted entities/assertions/links, confidence, unknowns và source spans | Agent/parser có thể đề xuất |
| Reviewed domain graphs | CR/SRS/SAD/TDD/CS revisions đã kiểm theo domain | Domain reviewer/authorizer theo policy |
| Integration graph | Versioned mappings, conflicts, coverage obligations và dependency closure | Mapping checks/review |
| Evidence ledger | Executions, reviews, decisions, acceptance receipts | Append-only producer identity, không sửa lịch sử |
| Baseline registry | Immutable pinned manifests và current baseline pointers | Atomic publish sau gate checks |
| Projection store | Rendered documents/OpenSpec/exports + manifests | Fidelity-gated adapter |

Đây là phân vùng logic; không bắt triển khai bằng nhiều database. Một graph engine hoặc relational/document storage có thể đáp ứng nếu giữ được isolation, immutability, referential integrity, transactions và access control. Candidate graph không xuất hiện mặc định trong truy vấn “accepted knowledge”.

## 2. Entities của pipeline và workflow

| Entity | Fields / invariants |
|---|---|
| IngestionRun | `sourceInventory! parserVersions+ startedAt! artifacts+ extractionResults* missingSources* status!` |
| ExtractionResult | `artifactRevision! spanInventory! candidates* omittedSpans* uncertaintyRecords* toolVersion!` |
| CandidateRecord | `candidateId! proposedType? proposedFields! sourceSpans+ extractionMethod! confidenceByFacet* unresolvedFields*` |
| ExtractionCoverage | `expectedSpans+ dispositions+ unprocessedSpans* completenessClaim!` |
| ReconciliationDecision | `candidates+ action! targetIdentity? rationale! evidence+ reviewer!` |
| PromotionBatch | `inputDigest! candidates+ domainReviewRefs+ validationReport! approvedOutputRevisions+` |
| BaselineBridge | `fromBaseline! toBaseline! identityMappings+ compatibilityEvidence+ scope!` |
| ProjectBootstrap | `project! sourceInventory! authorityPolicy! architectureWorkstream! baselineCandidates+ gaps*` |
| DeliveryRun | `intentRefs+ beforeBaseline! targetScope! stage! policyProfile! expectedInventory! currentGateDecisions*` |
| ImpactReport | `inputBaseline! srsContext! sadRef! tddInventory! perDeltaCoverage+ perTddDecisions* findings* closureReport!` |
| ImpactEntry | `tddIdentityOrPlannedIdentity! action! affectedSections* sourceRefs+ rationale! evidence+` |
| ImpactApproval | `reportRef! decisionPayloadHash! inputHashes! approver! role! decidedAt! decision!` |
| MaterializationPlan | `approvedImpact! steps* plannedTdds* plannedAnchors* expectedEffects+ conformanceChecks+` |
| TddSetRealization | `planRef! baselineSnapshot! actualSnapshot! mappings* unplannedChanges* conformanceReport!` |
| RealizationEntry | `identity! action! beforeRevision? actualRevision! actualSections+ checks+` |
| GateDecision | `gateId! inputDigest! ruleProfile! executionRefs+ reviewerRefs* outcome! decidedAt! blockers*` |
| InvalidationEvent | `changedRefs+ changedFacets+ affectedRefs+ reasons+ priorDecisionRefs* createdAt!` |
| PublicationTransaction | `expectedHead! proposedManifest! validationDigest! changedArtifacts+ outcome!` |
| Deviation / Incident | `observedEvidence+ expectedRefs+ affectedScope! risk! urgency! owner! disposition!` |

Giá trị * cho inventory ban đầu có thể rỗng; report bootstrap phải có explicit empty baseline evidence và planned add entries khi có design obligations. Không cho “rỗng vì chưa đọc hết” trông giống empty inventory hợp lệ.

## 3. Ingestion pipeline và guards

### 3.1. Register và capture

Lập inventory trước extraction: tài liệu chính, phụ lục, bảng, diagram, spreadsheet attachments, external contracts và nguồn thay thế. Record owner, declared authority, revision, effective scope, access policy và completeness expectation. Capture original bytes/hash và parser version. URL sống không đủ pin source; lưu snapshot hoặc resolver có version đảm bảo reproducibility.

PDF scan/image cần OCR/layout extraction; Word/Markdown/HTML/spreadsheet cần giữ heading/table/footnote/merged-cell semantics. Công cụ đọc thực tế được chọn lúc implementation; ontology không giới hạn format. Đồ thị/mũi tên trong diagram phải có source regions và extraction uncertainty; không tự suy direction khi hình mơ hồ.

### 3.2. Segment và extract

Tạo stable source spans/anchors cho clause, list, table cell, diagram element. Giữ raw text và normalized text khác fields. Footnote ngoại lệ, merged header đơn vị, column inheritance và negation là nội dung semantic; parser không được bỏ khi flatten bảng.

Extract candidate entities + atomic assertions + terms + constants + rules + references. Mỗi numeric assertion gắn đơn vị/context nguồn; confidence riêng cho OCR/type/entity match/relation, không chỉ một score. Confidence dùng xếp ưu tiên review, không tự cấp authority.

### 3.3. Reconcile và classify

Resolve identity bằng explicit ID trước, rồi candidate matching theo scoped term/definition/lineage. Match tự động deterministic được phép theo declared rule; ambiguous matches cần review. Không tạo duplicate chỉ vì tài liệu lặp lại cùng requirement; giữ nhiều source assertions. Không merge khác concept chỉ vì cùng tên.

Classify assertion vào domain theo semantics/authority. Nếu SRS bắt buộc một algorithm, phần normative behavior/constraint thuộc SRS; TDD actual realization có mapping. Không “chuyển câu” khỏi SRS làm mất thẩm quyền.

### 3.4. Validate, review và promote

Structural validation → source fidelity → domain rules → cross-domain mappings → completeness → human review cần thiết. Candidate chưa đủ vẫn lưu cùng gaps; không promote thành accepted. Domain owner review candidate khác review extraction accuracy: nhận dạng đúng câu không chứng minh câu đúng hoặc được duyệt.

Promotion là atomic batch với expected source/baseline hashes. Mỗi accepted assertion có source hoặc authorized authoring provenance. Không overwrite domain graph accepted bằng output LLM/parser. Dữ liệu mâu thuẫn vẫn được nhập ở candidate/disputed state để phản ánh tài liệu thực, nhưng chặn ready trong affected scope.

### 3.5. Render và round-trip

Render theo template views từ pinned semantic baseline; generated sections có manifest ánh xạ entity→anchor. Nội dung chưa formalize giữ original clause và editorial disposition, không xóa để đồ thị “sạch”. Export→reimport kiểm semantic identity, polarity, modality, constants, exceptions, scope và refs. Bố cục thay đổi được phép nếu anchors rebind có chứng cứ; loss mandatory chặn publication.

Document-first: external edit được ingest thành candidates và semantic diff. Graph-first: domain revisions được author/review rồi render. Một edit chỉ promote qua một transaction có baseline check; không hai hướng silent overwrite nhau. Presentation edit có equivalence assessment; nếu không chứng minh thì semantic review.

## 4. Bootstrap một dự án mới

1. Tạo project, role/authority policy, terminology registry, domain profiles và expected document inventory.
2. Nhập CR/URD/business needs. Chưa có CR riêng thì URD authority vẫn phải được BA xác nhận; không tạo human receipt giả.
3. Lập SRS candidate, domain validation và BA/Dev/SA review. Theo dõi open questions/assumptions theo gate.
4. Architecture workstream tạo SAD phù hợp SRS drivers; kiến trúc và requirements có thể iterate ở draft. Chỉ establish approved SAD khi conflicts chặn đã được giải quyết bởi đúng owners.
5. Pin coherent CR/URD+SRS+SAD baseline. TDD inventory có thể empty nhưng phải explicit; G2 tạo approved add plans.
6. Materialize actual TDD, domain/cross-domain validation và G3 realization.
7. G4 canonical CS → adapter nếu cần → implementation/test → G5/G6 evidence/acceptance.

Không giả sử dự án mới luôn có SRS/SAD hoàn hảo từ đầu. Các draft co-evolve; gates điều khiển quyền dùng chúng downstream. Product change flow không được sửa SAD giữa chừng dưới nhãn bootstrap.

## 5. Onboarding dự án đang chạy

Nhập hiện trạng tài liệu và repo/deployment evidence thành các graph riêng: **as-specified**, **as-designed**, **as-built**, **as-operated**. Không coi code hiện tại là nguồn đủ thẩm quyền để rewrite SRS. Reconciliation phát hiện undocumented behavior, obsolete design, missing requirements, drift và historical approval gaps.

Tạo baseline candidate + known gaps; chỉ accepted baseline khi có review/authority/evidence phù hợp. Legacy status “approved” thiếu receipt không được đổi thành verified human approval. Có thể giữ historical status claim riêng, ghi trust level và gate blocker. Từng scope được đưa vào delivery run phải đủ nguồn theo gates; không cần giả rằng toàn bộ di sản đã được chứng minh.

## 6. Change flow và phát sinh

| Loại phát sinh | Đường xử lý | Điều cấm |
|---|---|---|
| Nhu cầu mới | CR → SRS delta → G2/G3/G4 | Tạo code task chỉ từ prompt |
| Bug với SRS rõ, không đổi expected behavior | CR + discrepancy + pinned SRS → impact/TDD → CS | Giả semantic delta hoặc bỏ SRS |
| Bug nhưng SRS mơ hồ/mâu thuẫn | BA clarification → authorized SRS revision → impact | Lấy current code làm expected result |
| Đổi kiến trúc | Separate SAD CR/workstream → approved SAD → rebaseline product run | Agent sửa SAD readonly |
| Thay config trong miền thiết kế | Validate effective value + guarantees → no_change TDD nếu chứng minh được → CS | no_change chỉ vì không đổi file TDD |
| Refactor không đổi behavior | CR scope + unchanged requirement bindings/discrepancy nếu có → impact, design, regression obligations | Tạo requirement business giả cho refactor |
| Thay đổi external contract | Capture contract revision → consumer impact → CR/SRS/design context | Bỏ consumer vì ngoài repository |
| Incident/security urgency | Urgent CR/assessment/review scheduling theo policy, cùng mandatory source/approval gates | Dùng emergency label để tự bypass gates đã chốt |
| Manual hotfix đã xảy ra ngoài workflow | Ingest as-built deviation, mark unverified; corrective CR và retrospective review riêng | Backdate approval hoặc gọi workflow-compliant |
| Editorial edit | Source diff + semantic-equivalence review → rebind anchors | Coi thay negation/number là presentation |
| CR bị hủy | Stop pending work, assess materialized/implemented effects, corrective plan nếu cần | Xóa code/history và receipt tự động |
| Runtime drift | Operational evidence → deviation/impact → authorized change nếu cần | Silent sync runtime config thành normative baseline |

Refactor/context không delta cần contract `SrsChangeContext` có explicit unchanged requirement bindings, authorized intent và assessment; đây là đề xuất mở rộng input representation, không thay nguồn/gates đã chốt. Chỉ discrepancy bắt buộc cho defect; refactor không cần invent deviation.

## 7. Stage gates và responsibility

| Gate | Precondition | Output | Quyền / blocker |
|---|---|---|---|
| G0 Intake | Sources captured, roles, CR/URD criteria/scope | Intake decision | BA/source owner; missing authority/conflict chặn |
| G1 Requirements | Complete SRS context + CR authority | Pinned requirement scope/delta/discrepancy/unchanged bindings | BA + Dev/SA review; rule conflicts/gaps chặn |
| GA Architecture | SRS drivers + architecture model | Approved SAD baseline | SA/ARB theo policy; chưa có không qua G2 |
| G2 Impact | G1 + GA + complete TDD inventory | Three-action report + human approval | SA/Dev; agent không tự approve |
| G3 Realization | Approved plan + pinned sources | Complete actual TDD set/conformance | Dev/SA; add/update thiếu, unplanned drift chặn |
| G4 Spec | G3 + same current upstream sources | Canonical eligible CS và fidelity-checked projection | Dev/agent author; dual-source/CR trace thiếu chặn |
| G5 Verify | Approved CS + actual build/config | Executed verification report | Dev/QA/authorized verifiers; plans không thay executions |
| G6 Accept | Current criterion scope + applicable verification | BA receipt và criterion coverage | BA/acceptance owner; partial không full closure |

Mỗi gate có mandatory rule set theo applicable profiles và explicit check execution. Gate report bind exact inputs, không chỉ boolean cached. Review role arrangement cụ thể do project policy; không tự áp thêm two-person approval ngoài policy đã thống nhất.

## 8. Transactions, concurrency và invalidation

Publish dùng compare-and-swap expected baseline pointer + source/manifest checks trong transaction. Nếu head đổi, publication abort thành conflict; rebase/merge có semantic diff rồi checks cần thiết. Multi-artifact projection dùng staging + atomic manifest switch hoặc transactional equivalent; crash không để half-published state trông ready.

Hai CR sửa cùng entity ở scopes disjoint có thể merge khi chứng minh disjoint; overlapping phải reconcile. Thay mapping/profile/rule versions cũng là dependency changes. Baseline cũ giữ nguyên; pointer mới không sửa historical evidence. Rollback bằng event/baseline mới có lineage, không xóa run thất bại.

Invalidation granularity: field/facet/entity/mapping/document/scope. Source hash đổi làm source checks stale; nếu verified presentation-only thì rebind/revalidate source fidelity và retain semantic decision applicability bằng explicit review record. Không làm mọi approval stale chỉ vì TDD được cập nhật đúng approved plan; actual realization chứng minh điều này.

## 9. Rule catalog

| ID | Invariant / điều kiện | Evaluation | Gate |
|---|---|---|---|
| FLOW-001 | Source inventory pin đầy đủ docs/phụ lục/diagrams; missing/restricted không disappear | inventory reconciliation | G0 |
| FLOW-002 | Extraction giữ raw spans và dispositions; không bỏ negation/exceptions/table units | source fidelity audit | G1 |
| FLOW-003 | Candidate/confidence không tự promote normative knowledge hoặc human approval | promotion authority check | ALL |
| FLOW-004 | Domain ownership theo semantics/authority, không file name; ambiguous classification giữ review gap | classification review | G1 |
| FLOW-005 | Bootstrap có explicit empty baseline và approved SAD trước product impact; không fake refs | bootstrap branch validation | G2 |
| FLOW-006 | Legacy/as-built/as-operated claims tách normative baseline, thiếu receipt giữ unverified | onboarding trust validation | G1 |
| FLOW-007 | Impact đối chiếu mọi SRS intent và toàn TDD inventory; add/update/no_change có evidence | bidirectional impact coverage | G2 |
| FLOW-008 | ImpactApproval bind human role/time/payload/input hashes; stale/rejected/missing chặn | approval validation | G3 |
| FLOW-009 | Realization đủ approved plan, actual refs và no_change revision; unplanned drift quay G2 | plan conformance | G3 |
| FLOW-010 | Không author CS trước G2/G3, kể cả gọi draft; source readiness không thay approval | authoring preflight | G4 |
| FLOW-011 | Product SAD immutable; architecture CR hoàn tất vẫn cần rebaseline/reimpact | SAD branch check | G2 |
| FLOW-012 | Baseline publish atomic, expected head/source hashes khớp; race không silent overwrite | transaction/CAS check | ALL |
| FLOW-013 | Evidence/invalidation append-only; corrective changes không backdate approvals | audit event integrity | ALL |
| FLOW-014 | Urgency/prompt/tool status không waive mandatory source/approval/realization gates | policy non-bypass check | ALL |
| FLOW-015 | Document↔graph edits có reconciliation/fidelity trước promote; không silent bidirectional sync | round-trip/diff audit | G1 |
| FLOW-016 | No-delta refactor/defect vẫn có authorized SRS context, source refs và impact đầy đủ | context alternative validator | G1 |

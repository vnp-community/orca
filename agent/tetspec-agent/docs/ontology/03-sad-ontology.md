# 03 — SAD ontology: cấu trúc, quyết định và ràng buộc kiến trúc

SAD sở hữu các kiến trúc views và quyết định có thẩm quyền. Diagram là projection của architecture model, không là nguồn identity riêng cho mỗi hình. Thiết kế này không yêu cầu dự án dùng một notation cụ thể.

## 1. Data dictionary

| Entity | Fields | Relations/cardinality và constraints |
|---|---|---|
| ArchitectureDescription | `systemScope! accountableArchitect! revisionPurpose!` | `views+ concerns+ decisions* constraints* risks*` |
| StakeholderConcern | `stakeholderRole! concern! priority! acceptanceApproach!` | `addressedBy View+` trước architecture acceptance |
| ArchitectureDriver | `kind! statement! priority! sourceRefs+` | Functional/quality/business/constraint; không tự tạo requirement |
| Viewpoint | `name! concerns+ elementKinds+ relationKinds+ consistencyRules* notation!` | Một view dùng một viewpoint; composed viewpoint được định nghĩa riêng |
| ArchitectureView | `viewpoint! scope! elements* relations* purpose!` | Element refs shared across views, source anchors+ |
| ArchitectureElement | `elementKind! name! responsibilities+ owner! boundary! lifecycle!` | contains* (DAG), dependsOn*, serves*; explicit external flag |
| Responsibility | `description! subjectCapability! exclusivity!` | assignedTo elements+; shared responsibility cần collaboration/ownership rule |
| Connector | `source! target! direction! interactionStyle! dataClassification! contractRef?` | Sync/async/batch/file; logical direction khác network initiation |
| ArchitectureInterface | `provider! consumers* contractBoundary! compatibilityPolicy!` | TDD interface refines qua mapping |
| ArchitectureConstraint | `predicate! applicability! rationale! authority! exceptionPolicy!` | constrained elements/scopes+ |
| ArchitecturePrinciple | `statement! rationale! implications! applicability!` | Có operational checks hoặc review method |
| ArchitectureDecision | `context! drivers+ selectedOption! alternatives+ rationale! consequences+ status!` | supersedes*; accepted có decision record |
| ArchitectureOption | `description! tradeoffs+ addressedDrivers+ rejectedReason?` | Không đòi alternatives giả; single feasible option cần rationale |
| QualityScenario | `sourceOfStimulus! stimulus! environment! artifact! response! responseMeasure!` | sources SRS NFR refs+ qua mapping |
| QualityStrategy | `targetScenarios+ tactics+ assumptions* tradeoffs* verificationApproach!` | Strategy không chứng minh achieved target |
| DeploymentNode | `nodeKind! environment! resourceConstraints* failureDomain!` | hosts ArchitectureElement*, connectsTo* |
| TrustBoundary | `insideScope! outsideScope! trustAssumptions+ crossingRules+` | mapped flows/connectors* |
| DataOwnership | `dataConceptRef! authoritativeOwner! permittedReplicas* writeAuthority! consistencyExpectation!` | Authoritative writer có thể distributed nếu protocol explicit |
| ExternalDependency | `provider! contract! availabilityAssumptions! versionPolicy! failureStrategy! owner!` | Pin revision hoặc snapshot của external contract |
| ArchitectureRisk | `risk! affectedDecisions+ likelihood? impact! mitigation! owner! reviewTrigger!` | residual disposition explicit |
| ArchitectureAssumption | `predicate! validationOwner! evidencePlan! invalidationTrigger!` | Không accepted fact khi chưa verified |
| ConformanceObligation | `constraintRef! subjectScope! method! requiredEvidence!` | actual conformance receipt thuộc verification |

## 2. View consistency

System/container/component/deployment/data/security views là ví dụ viewpoint, không required enum. Các views dùng cùng entity refs. Nếu logical view service A gọi B nhưng deployment/security view chặn toàn bộ đường khả thi, sinh inconsistency với path witness. Một diagram bỏ chi tiết do abstraction không tự là conflict: view phải khai báo `completeFor` và abstraction mapping.

Kiểm data ownership giữa data và runtime views: mọi đường ghi tới dữ liệu có writer/transaction ownership; nếu replicas thì consistency/replication assumptions rõ. Kiểm trust boundary: crossing được nối tới authN/authZ/encryption/audit expectations đúng scope; không suy “có Gateway” là mọi service đã bảo vệ.

## 3. Kiến trúc và ngân sách chất lượng

Allocation từ SRS quality target tới architecture strategy phải nêu decomposition semantics. Latency budgets có topology critical path, overhead và concurrency; không cộng mọi latency mặc định. Availability composition cần dependency/correlation model; không nhân SLA độc lập nếu common failure domain. Throughput và capacity dùng workload assumptions, không chỉ node count.

ADR accepted không đồng nghĩa mọi constraint satisfiable. Lựa chọn A và B ở hai ADR khác nhau có thể mâu thuẫn; conflict phải được phát hiện trên affected elements/scopes. Có thể chấp nhận tradeoff trong phạm vi SRS cho phép; không waive SRS MUST bằng ADR.

## 4. Lifecycle và workstream

Description revision: draft → reviewed → approved → superseded. ADR: proposed → accepted/rejected; accepted → superseded/deprecated bằng decision mới. Architecture workstream initial tạo SAD baseline; product flow chỉ bắt đầu design-impact gate sau SAD approved. Architecture CR dùng CR/SRS context liên quan và reviewers có role SA/ARB theo policy; không coi agent là approver.

Trong product flow, SAD incompatible tạo blocker + architecture change request dependency. SAD mới approved không tự unlock product flow: rebaseline, re-evaluate SRS feasibility, TDD inventory/impact và mappings. Có thể tiếp tục phân tích preview trong thời gian chờ; không sinh implementation-ready CS.

## 5. Rule catalog

| ID | Invariant / điều kiện | Evaluation | Gate |
|---|---|---|---|
| SAD-001 | Approved architecture có owner, scope, concerns, views và constraints inventory rõ | shape + inventory review | GA |
| SAD-002 | View dùng viewpoint hợp lệ và shared identities; duplicate labels không tạo equivalence tự động | view/model resolver | GA |
| SAD-003 | Containment không cycle; connector endpoints resolve đúng scope hoặc explicit external | graph validation | GA |
| SAD-004 | Mỗi responsibility có owner/allocation; exclusive responsibilities không bị gán mâu thuẫn | allocation check | GA |
| SAD-005 | Accepted ADR có authority, drivers, alternatives/rationale và consequences; supersedes không cycle | decision/provenance check | GA |
| SAD-006 | Accepted decisions/constraints satisfiable trên overlap; conflict giữ witness và chặn | constraint solver / architect review | GA |
| SAD-007 | Quality strategy trace tới NFR/scenario và assumptions/budget có semantics hợp lệ | budget/semantic review | GA |
| SAD-008 | Cross-view runtime/deployment/data/security paths không mâu thuẫn với completeness claims | graph path/conformance | GA |
| SAD-009 | Data writers/replicas/ownership phù hợp responsibilities và consistency assumptions | ownership path review | GA |
| SAD-010 | Trust-boundary crossing có obligations cần thiết; thiếu đường trust/control không suy safe | security path check | GA |
| SAD-011 | Unresolved critical risk/assumption có owner và chặn gate tương ứng theo declared policy | risk/obligation evaluation | GA |
| SAD-012 | Product run pin readonly SAD; architecture mismatch cần separate SAD CR, không local waiver | run-baseline check | G2 |
| SAD-013 | SAD mới tạo rebaseline và invalidation downstream; old approval không tự tái dùng | dependency invalidation | G2 |
| SAD-014 | Ngoại lệ kiến trúc phải là authorized SAD decision đúng scope/thời hạn; không miễn upstream MUST | exception/authority check | GA |

`GA` là gate duyệt kiến trúc riêng, prerequisite của G2, không chèn một bước sửa SAD tự do vào product-change flow.

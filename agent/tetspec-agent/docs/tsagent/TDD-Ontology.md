# 04 — TDD ontology: thiết kế có thể thực hiện và kiểm chứng

TDD sở hữu design semantics chi tiết. Module này có các subdomains cùng identity/provenance contract; specialist profiles lựa chọn applicability theo behavior/risk, không theo danh sách tài liệu đã có. Tables dưới mô tả core và conditional entities, không bắt mọi thiết kế tạo mọi type.

## 1. Design organization và obligations

| Entity | Fields | Relations / constraints |
|---|---|---|
| DesignUnit | `name! responsibility! scope! owner!` | sections+, elements*, dependencies*; allocatedTo SAD qua link |
| DesignSection | `purpose! coverageScope! anchors+` | describes elements*; section không thay semantic entity |
| DesignDecision | `question! alternatives+ selection! rationale! constraints+ tradeoffs*` | decision evidence; không sửa SRS/SAD authority |
| DesignDependency | `source! target! kind! contractVersion! failureImpact!` | calls/consumes/writes/reads/dependsOnInvariant |
| DesignObligation | `question! subjectScope! sourceMappingRefs+ applicability! requiredMaturity! requiredChecks+` | Không chỉ “có field”; phải có resolution đủ nghĩa |
| ObligationResolution | `obligation! designRefs+ explanation! evidence+ coverageComposition! status!` | planned/proposed/verified/accepted; accepted có reviewer nếu policy yêu cầu |
| DesignGap | `obligation! missingKnowledge! impact! owner! blockingGates+` | Open/resolved/superseded |
| ApplicabilityDecision | `profileRule! subject! result! rationale! evidence+ reviewer?` | applicable/not_applicable/unknown; N/A có chứng minh |

## 2. Behavior và algorithm

| Entity | Fields | Constraints |
|---|---|---|
| Operation | `inputs! outputs! preconditions! postconditions! errorOutcomes* sideEffects* authContext!` | Có owned interface/module, requirement mapping |
| InteractionFlow | `participants+ entryCondition! steps+ successConditions! failurePaths*` | Step ordering/parallel branches explicit |
| InteractionStep | `actor! operation! inputBinding! outputBinding! ordering! branchGuard?` | Bindings type-safe, producer trước consumer theo partial order |
| Algorithm | `inputs! outputs! preconditions! postconditions! stepsOrExpression! invariants* terminationArgument? complexityModel?` | Required termination/resource proof theo risk/profile |
| DecisionTable | `inputs+ rows+ hitPolicy! completenessScope!` | Có thể reuse rule semantics foundation, khác SRS identity |
| StateMachine | `subject! states+ initialStates+ transitions+ terminalStates* invariants*` | Deployment/process state và business state map có chủ đích |
| Transition | `from! to! trigger! guard! effects* atomicity!` | Guard/action interplay checked, không chỉ reachability |
| Invariant | `predicate! scope! enforcementPoints+ failureBehavior!` | State/transaction/temporal classification |
| ConcurrencyPolicy | `sharedResources+ isolation! ordering! conflictHandling! progressGuarantee!` | Deadlock/livelock checks trong supported model |

## 3. Interface, event và failure contracts

| Entity | Fields | Constraints |
|---|---|---|
| InterfaceContract | `provider! consumers* protocol! version! operations+ compatibilityPolicy!` | Consumer inventory completeness explicit |
| Endpoint | `method! path! inputSchema! successResponses+ errorResponses* authPolicy! rateLimits?` | HTTP-specific subtype, không áp cho mọi interface |
| EventContract | `eventType! schema! producer! consumers* partitionKey? ordering! deliveryGuarantee! retention!` | Event time vs processing time explicit |
| MessageSchema | `fields+ requiredFields* additionalFieldsPolicy! schemaVersion!` | Type/unit/nullability/enum meanings |
| ErrorContract | `condition! classification! representation! retryable! sideEffectState!` | Timeout không tự có nghĩa chưa commit |
| ContractCompatibility | `providerBefore! providerAfter! consumerVersions+ compatibilityMode! evidence!` | Input/output variance, schema/meaning preserved |
| TimeoutBudget | `scope! endToEndDeadline! callBudgets* overhead! cancellation! clockAssumptions!` | Deadline absolute/relative, units explicit |
| RetryPolicy | `conditions+ maxAttempts! backoff! jitter! retryBudget! stopCondition!` | Attempts gồm lần đầu hay số retry phải thống nhất |
| IdempotencyPolicy | `operation! keyDerivation! keyScope! payloadFingerprint! retention! atomicStore! concurrentDuplicateBehavior! resultReplay!` | Key collision/reuse with differing payload handled |
| FailurePolicy | `failureClass! detection! effectState! response! recoveryAction! escalation!` | Không chỉ generic “retry” |
| CircuitBreakerPolicy | `failureThreshold! window! openDuration! halfOpenProbe! fallback!` | Fallback giữ SRS invariants |

## 4. Data, transaction và migration

| Entity | Fields | Constraints |
|---|---|---|
| PhysicalDataModel | `store! modelKind! version! ownership!` | logical mappings+, tables/entities*, constraints* |
| Table / Collection | `fields+ identity! partitioning? indexes* lifecycle!` | Physical model cụ thể theo datastore |
| Field | `physicalType! nullability! default? unit? encoding? domainConstraints*` | Logical meaning mapping khi applicable |
| Index | `fieldsOrExpression+ uniqueness! filter? purpose!` | Performance hypothesis không tự proof |
| IntegrityConstraint | `predicate! enforcementMechanism! enforcementBoundary!` | FK/check/unique/application/distributed enforcement |
| Transaction | `operations+ boundary! atomicity! isolation! invariants+ commitPoint! rollbackBehavior!` | Cross-store writes không giả local ACID |
| Saga | `steps+ ordering! compensationPolicy! completionCondition! failureTerminalStates+` | Step effects/compensations separately specified |
| SagaStep | `forwardOperation! compensation? idempotency! timeout! irreversibility!` | Irreversible step cần recovery/manual intervention design |
| ConsistencyModel | `scope! guarantee! visibility! conflictResolution! stalenessBound?` | Read/write guarantees có consumer assumptions |
| Migration | `beforeModel! afterModel! stages+ compatibilityWindow! rolloutPlan! rollbackOrRecovery! validationQueries+` | Irreversible migration không ghi rollback giả |
| MigrationStage | `inputs! transform! outputs! dependencies* preconditions! postconditions! resumability!` | Stage DAG, checkpoint/retry/dedupe rõ |
| RetentionPolicy | `dataScope! retentionDuration! startEvent! holdRules* disposal! authorityRefs+` | Audit/business retain vs delete conflicts reviewed |
| ReconciliationPolicy | `sources+ comparisonKey! invariant! tolerance! cadence! discrepancyAction! owner!` | Tolerance có authority; không tự bỏ mismatch |

## 5. Security, quality và operations

| Entity | Fields | Constraints |
|---|---|---|
| PermissionDesign | `subject! resource! action! context! decisionPredicate! enforcementPoints+ denyBehavior! audit!` | Default rule và policy precedence explicit |
| SecurityControl | `threatOrRequirement! mechanism! placement! evidencePlan! failureMode!` | Control presence khác effectiveness |
| SecretHandling | `secretKind! reference! accessPolicy! rotation! revocation! exposurePrevention!` | Không chứa secret value |
| AuditDesign | `events+ fields+ actorAttribution! integrity! retention! access! failureBehavior!` | Sensitive values redaction policy |
| LoadProfile | `workloadMix! arrivalPattern! concurrency! payloadSizes! dataVolume! duration!` | Source assumptions/measured lineage |
| CapacityModel | `resources+ workload! formulaOrModel! limits! headroom! bottlenecks! assumptions+` | Không claim achieved capacity từ model |
| SLOAllocation | `upstreamTarget! subBudgets+ compositionModel! margin! environment!` | Cross-domain mapping tới SRS/SAD |
| HATopology | `replicas! failureDomains! quorum? failover! splitBrainPrevention! degradedMode!` | Common-mode failure explicit |
| RecoveryPlan | `failureScope! RTO! RPO! procedure! restoreValidation! drillPlan!` | Target khác measurement |
| ObservabilityDesign | `signals+ correlation! thresholds! actions! owners! retention!` | Signal phát hiện violation có diagnostic coverage |
| RolloutPlan | `stages+ entryChecks+ exitChecks+ abortConditions+ rollbackOrRecovery! compatibilityWindow!` | Monitor/decision owner cho từng stage |
| OperationalOwnership | `scope! responsible! escalation! onCallOrSupport! runbooks+` | Có quyền/phương tiện thực hiện runbook |

## 6. Logic thiết kế và preservation

Design phải preserve upstream required behaviors trong applicability domain; có thể thêm internal detail không đổi observable semantics. Mọi side effect mới (external write, audit export, data replication) kích hoạt applicability reassessment cho data/security/resilience. Không áp profile dựa trên việc “không thấy transaction node” để kết luận không cần transaction.

Ngân sách retry cho sequential attempts với maxAttempts N: worst-case latency ≤ tổng timeout từng attempt + tổng backoff tối đa + overhead, trừ khi cancellation/deadline cap được thiết kế và kiểm. Parallel branches dùng critical path; budget equations phải khai assumptions. Đây là model-bound check, không chứng minh percentile production bằng cách cộng p95 thành phần.

Idempotency cần atomic key reservation/commit hoặc protocol tương đương, scope đúng actor/operation/resource, fingerprint để chặn cùng key khác payload, retention phủ retry/redelivery horizon. Chỉ có field key chưa chứng minh exactly-once effect. Delivery guarantee của broker khác business effect guarantee.

State reachability trên graph không chứng minh guard feasible; guard satisfiability khác safety invariant preservation; cả hai khác liveness. Checker không hỗ trợ concurrency/unbounded guard phải trả unsupported và yêu cầu review/proof khác.

## 7. Rule catalog

| ID | Invariant / điều kiện | Evaluation | Gate |
|---|---|---|---|
| TDD-001 | Design inventory complete; mỗi unit có owner/scope và actual sections resolve được | inventory/reference check | G3 |
| TDD-002 | Mandatory obligations sinh từ sources/risk/profile đầy đủ; thiếu node không làm obligation N/A | applicability reconciliation | G3 |
| TDD-003 | Operations/algorithms đủ input/output/pre/post/error/side-effects theo applicability | shape + design review | G3 |
| TDD-004 | Flow bindings type-safe, ordering đúng, failure branches không bỏ required outcomes | dataflow/branch analysis | G3 |
| TDD-005 | State transitions feasible trong model, bảo toàn invariants; reachability không thay semantic proof | graph + solver / review | G3 |
| TDD-006 | Required termination/resource bounds có argument phù hợp supported algorithm model | static/model/human check | G3 |
| TDD-007 | Interface input/output/error/auth contracts nhất quán giữa producer và consumers | contract compatibility | G3 |
| TDD-008 | Retry/deadline/backoff/cancellation đồng bộ; worst-case budget hợp lệ theo declared model | budget evaluator | G3 |
| TDD-009 | Retry write có duplicate/concurrency handling đủ key scope, fingerprint, atomicity và retention | idempotency scenario checks | G3 |
| TDD-010 | Failure/timeout không giả no-effect; recovery/fallback giữ upstream invariants | failure effect analysis | G3 |
| TDD-011 | Physical data mapping giữ meaning/range/unit/null/identity/constraints của logical model | data refinement proof | G3 |
| TDD-012 | Transactions/invariants đúng boundary/isolation; cross-store atomicity có protocol thực | transaction model review | G3 |
| TDD-013 | Saga compensation/recovery đủ failed/partial/irreversible states, có owner cho manual path | failure-state coverage | G3 |
| TDD-014 | Migration DAG hợp lệ, old/new compatibility và backfill/retry/rollback-or-recovery có verification | migration model checks | G3 |
| TDD-015 | Permission enforcement bao mọi access paths trong scope; default/precedence/deny nhất quán | access-path/decision checks | G3 |
| TDD-016 | Security/audit design đáp ứng classification, no-secret exposure và tamper/retention obligations | controls + review | G3 |
| TDD-017 | Capacity/SLO model có workload/unit/assumptions; target không gắn nhãn measured achievement | quantity/model provenance | G3 |
| TDD-018 | HA/quorum/failure domains và RPO/RTO phù hợp SAD/SRS với degraded-mode nghĩa rõ | topology/recovery analysis | G3 |
| TDD-019 | Rollout có detection, abort, owner và rollback/recovery tương thích migration/data | operational consistency | G3 |
| TDD-020 | New side-effect/consumer/risk làm reassess profiles và impact closure | semantic diff trigger | G2 |
| TDD-021 | no_change chỉ khi current design bao phủ cả value domain và guarantees; no match không đủ | reuse conformance proof | G2 |
| TDD-022 | Realization giữ action, planned scope và actual anchors; ngoài plan quay impact approval | plan-realization comparison | G3 |
| TDD-023 | Design decisions không override SRS/SAD; conflicts trả upstream, không local approve | mapping/authority review | G3 |
| TDD-024 | Concurrency locking/ordering/isolation không vi phạm safety/progress trong claimed model | concurrency/model review | G3 |

## 8. Maturity

`identified` chỉ biết entity; `specified` đủ nội dung; `reviewed` có review; `verified` có checks đúng revision; `accepted` có authority nếu policy yêu cầu. Entity maturity không ép cùng lifecycle với tài liệu. TDD document “approved” có section chưa verified phải báo riêng và chặn nếu section nằm trong scope bắt buộc.

## 9. Sơ đồ

Các sơ đồ dưới vẽ lại mục 1 đến 8. Quan hệ lấy từ cột Fields và Relations: một trường trỏ tới entity khác (ví dụ `InteractionStep.operation`, `Migration.stages`) được vẽ thành cạnh. Mỗi lớp chỉ ghi các trường bắt buộc chính; danh sách đầy đủ xem bảng ở mục tương ứng.

### 9.1 Tổng quan

Năm nhóm entity dùng chung một contract định danh và provenance. Obligation nối nguồn upstream (SRS, SAD, rủi ro, profile) với các entity thiết kế. Rule catalog và maturity kiểm cả năm nhóm.

```mermaid
flowchart TB
  UP["Upstream: SRS · SAD · risk · specialist profile"]
  subgraph ORG["1 · Design organization và obligations"]
    DU[DesignUnit] --- DS[DesignSection]
    DD[DesignDecision]
    DEP[DesignDependency]
    OB[DesignObligation] --> RES[ObligationResolution]
    OB --> GAP[DesignGap]
    APP[ApplicabilityDecision]
  end
  subgraph BEH["2 · Behavior và algorithm"]
    OP[Operation] --- FLOW[InteractionFlow]
    ALG[Algorithm] --- DT[DecisionTable]
    SM[StateMachine] --- INV[Invariant]
    CONC[ConcurrencyPolicy]
  end
  subgraph CON["3 · Interface, event, failure contracts"]
    IC[InterfaceContract] --- EP[Endpoint]
    EV[EventContract] --- MS[MessageSchema]
    ERR[ErrorContract] --- FP[FailurePolicy]
    TB[TimeoutBudget] --- RP[RetryPolicy]
    IDP[IdempotencyPolicy] --- CB[CircuitBreakerPolicy]
  end
  subgraph DAT["4 · Data, transaction, migration"]
    PDM[PhysicalDataModel] --- TBL[Table / Collection]
    TX[Transaction] --- SAGA[Saga]
    MIG[Migration]
    RET[RetentionPolicy] --- REC[ReconciliationPolicy]
  end
  subgraph OPS["5 · Security, quality, operations"]
    PERM[PermissionDesign] --- SEC[SecurityControl]
    AUD[AuditDesign] --- SECRET[SecretHandling]
    LOAD[LoadProfile] --- CAP[CapacityModel]
    HA[HATopology] --- RECOV[RecoveryPlan]
    ROLL[RolloutPlan] --- OWN[OperationalOwnership]
  end
  UP -->|sinh obligation| OB
  UP -->|profileRule| APP
  APP -->|applicable / not_applicable / unknown| OB
  RES -->|designRefs| BEH
  RES -->|designRefs| CON
  RES -->|designRefs| DAT
  RES -->|designRefs| OPS
  RULES{{"Rule catalog TDD-001…024<br/>gate G2 / G3"}} -.kiểm.-> ORG & BEH & CON & DAT & OPS
  MAT{{"Maturity theo entity<br/>identified → accepted"}} -.gắn lên.-> ORG & BEH & CON & DAT & OPS
```

### 9.2 Design organization và obligations

```mermaid
classDiagram
  direction LR
  class DesignUnit {
    name
    responsibility
    scope
    owner
  }
  class DesignSection {
    purpose
    coverageScope
    anchors
  }
  class DesignDecision {
    question
    alternatives
    selection
    rationale
    constraints
  }
  class DesignDependency {
    kind
    contractVersion
    failureImpact
  }
  class DesignObligation {
    question
    subjectScope
    applicability
    requiredMaturity
    requiredChecks
  }
  class ObligationResolution {
    explanation
    evidence
    coverageComposition
    status
  }
  class DesignGap {
    missingKnowledge
    impact
    owner
    blockingGates
  }
  class ApplicabilityDecision {
    profileRule
    result
    rationale
    evidence
    reviewer
  }
  class DesignElement
  class SAD
  class SourceMapping
  DesignUnit "1" --> "1..*" DesignSection : sections
  DesignUnit "1" --> "*" DesignElement : elements
  DesignUnit "1" --> "*" DesignDependency : dependencies
  DesignUnit --> SAD : allocatedTo
  DesignSection --> DesignElement : describes
  DesignDependency --> DesignElement : source
  DesignDependency --> DesignElement : target
  DesignObligation --> SourceMapping : sourceMappingRefs
  ObligationResolution --> DesignObligation : obligation
  ObligationResolution --> DesignElement : designRefs
  DesignGap --> DesignObligation : obligation
  ApplicabilityDecision --> DesignElement : subject
```

`DesignElement` đại diện cho mọi entity thiết kế ở mục 2 đến 5. `SAD` và `SourceMapping` thuộc các module ontology khác.

### 9.3 Behavior và algorithm

```mermaid
classDiagram
  direction LR
  class Operation {
    inputs
    outputs
    preconditions
    postconditions
    errorOutcomes
    sideEffects
    authContext
  }
  class InteractionFlow {
    participants
    entryCondition
    successConditions
    failurePaths
  }
  class InteractionStep {
    actor
    inputBinding
    outputBinding
    ordering
    branchGuard
  }
  class Algorithm {
    inputs
    outputs
    stepsOrExpression
    terminationArgument
    complexityModel
  }
  class DecisionTable {
    inputs
    rows
    hitPolicy
    completenessScope
  }
  class StateMachine {
    subject
    states
    initialStates
    terminalStates
  }
  class Transition {
    trigger
    guard
    effects
    atomicity
  }
  class Invariant {
    predicate
    scope
    enforcementPoints
    failureBehavior
  }
  class ConcurrencyPolicy {
    sharedResources
    isolation
    ordering
    conflictHandling
    progressGuarantee
  }
  class InterfaceOrModule
  class Requirement
  Operation --> InterfaceOrModule : owned by
  Operation --> Requirement : requirement mapping
  InteractionFlow "1" --> "1..*" InteractionStep : steps
  InteractionStep --> Operation : operation
  StateMachine "1" --> "1..*" Transition : transitions
  StateMachine --> Invariant : invariants
  Algorithm --> Invariant : invariants
  Transition --> StateMachine : from / to state
```

### 9.4 Interface, event và failure contracts

```mermaid
classDiagram
  direction LR
  class InterfaceContract {
    provider
    consumers
    protocol
    version
    compatibilityPolicy
  }
  class Endpoint {
    method
    path
    inputSchema
    successResponses
    errorResponses
    authPolicy
    rateLimits
  }
  class EventContract {
    eventType
    producer
    consumers
    partitionKey
    ordering
    deliveryGuarantee
    retention
  }
  class MessageSchema {
    fields
    requiredFields
    additionalFieldsPolicy
    schemaVersion
  }
  class ErrorContract {
    condition
    classification
    representation
    retryable
    sideEffectState
  }
  class ContractCompatibility {
    consumerVersions
    compatibilityMode
    evidence
  }
  class TimeoutBudget {
    scope
    endToEndDeadline
    callBudgets
    overhead
    cancellation
  }
  class RetryPolicy {
    conditions
    maxAttempts
    backoff
    jitter
    retryBudget
    stopCondition
  }
  class IdempotencyPolicy {
    keyDerivation
    keyScope
    payloadFingerprint
    retention
    atomicStore
    concurrentDuplicateBehavior
    resultReplay
  }
  class FailurePolicy {
    failureClass
    detection
    effectState
    response
    recoveryAction
    escalation
  }
  class CircuitBreakerPolicy {
    failureThreshold
    window
    openDuration
    halfOpenProbe
    fallback
  }
  class Operation
  InterfaceContract "1" --> "1..*" Operation : operations
  Endpoint --|> InterfaceContract : HTTP subtype
  EventContract --> MessageSchema : schema
  ContractCompatibility --> InterfaceContract : providerBefore
  ContractCompatibility --> InterfaceContract : providerAfter
  IdempotencyPolicy --> Operation : operation
  Operation --> ErrorContract : errorOutcomes
  RetryPolicy ..> TimeoutBudget : đồng bộ deadline (TDD-008)
  RetryPolicy ..> IdempotencyPolicy : retry write cần (TDD-009)
  FailurePolicy ..> ErrorContract : failureClass
  CircuitBreakerPolicy ..> FailurePolicy : fallback giữ invariant
```

### 9.5 Data, transaction và migration

```mermaid
classDiagram
  direction LR
  class PhysicalDataModel {
    store
    modelKind
    version
    ownership
  }
  class Table {
    fields
    identity
    partitioning
    lifecycle
  }
  class Field {
    physicalType
    nullability
    default
    unit
    encoding
  }
  class Index {
    fieldsOrExpression
    uniqueness
    filter
    purpose
  }
  class IntegrityConstraint {
    predicate
    enforcementMechanism
    enforcementBoundary
  }
  class Transaction {
    boundary
    atomicity
    isolation
    commitPoint
    rollbackBehavior
  }
  class Saga {
    ordering
    compensationPolicy
    completionCondition
    failureTerminalStates
  }
  class SagaStep {
    compensation
    idempotency
    timeout
    irreversibility
  }
  class ConsistencyModel {
    scope
    guarantee
    visibility
    conflictResolution
    stalenessBound
  }
  class Migration {
    compatibilityWindow
    rolloutPlan
    rollbackOrRecovery
    validationQueries
  }
  class MigrationStage {
    inputs
    transform
    outputs
    preconditions
    postconditions
    resumability
  }
  class RetentionPolicy {
    dataScope
    retentionDuration
    startEvent
    holdRules
    disposal
  }
  class ReconciliationPolicy {
    sources
    comparisonKey
    tolerance
    cadence
    discrepancyAction
    owner
  }
  class LogicalModel
  class Operation
  class Invariant
  PhysicalDataModel --> LogicalModel : logical mappings
  PhysicalDataModel "1" --> "*" Table : tables
  PhysicalDataModel "1" --> "*" IntegrityConstraint : constraints
  Table "1" --> "1..*" Field : fields
  Table "1" --> "*" Index : indexes
  Transaction "1" --> "1..*" Operation : operations
  Transaction "1" --> "1..*" Invariant : invariants
  Saga "1" --> "1..*" SagaStep : steps
  SagaStep --> Operation : forwardOperation
  Migration --> PhysicalDataModel : beforeModel
  Migration --> PhysicalDataModel : afterModel
  Migration "1" --> "1..*" MigrationStage : stages
  MigrationStage --> MigrationStage : dependencies
  ReconciliationPolicy --> Invariant : invariant
```

### 9.6 Security, quality và operations

```mermaid
classDiagram
  direction LR
  class PermissionDesign {
    subject
    resource
    action
    context
    decisionPredicate
    enforcementPoints
    denyBehavior
    audit
  }
  class SecurityControl {
    threatOrRequirement
    mechanism
    placement
    evidencePlan
    failureMode
  }
  class SecretHandling {
    secretKind
    reference
    accessPolicy
    rotation
    revocation
    exposurePrevention
  }
  class AuditDesign {
    events
    fields
    actorAttribution
    integrity
    retention
    access
  }
  class LoadProfile {
    workloadMix
    arrivalPattern
    concurrency
    payloadSizes
    dataVolume
    duration
  }
  class CapacityModel {
    resources
    formulaOrModel
    limits
    headroom
    bottlenecks
    assumptions
  }
  class SLOAllocation {
    upstreamTarget
    subBudgets
    compositionModel
    margin
    environment
  }
  class HATopology {
    replicas
    failureDomains
    quorum
    failover
    splitBrainPrevention
    degradedMode
  }
  class RecoveryPlan {
    failureScope
    RTO
    RPO
    procedure
    restoreValidation
    drillPlan
  }
  class ObservabilityDesign {
    signals
    correlation
    thresholds
    actions
    owners
  }
  class RolloutPlan {
    stages
    entryChecks
    exitChecks
    abortConditions
    rollbackOrRecovery
    compatibilityWindow
  }
  class OperationalOwnership {
    scope
    responsible
    escalation
    onCallOrSupport
    runbooks
  }
  class Migration
  class SRS_SAD["SRS / SAD target"]
  PermissionDesign ..> AuditDesign : audit
  CapacityModel --> LoadProfile : workload
  SLOAllocation --> SRS_SAD : upstreamTarget
  RecoveryPlan ..> HATopology : failureScope
  RolloutPlan ..> Migration : tương thích (TDD-019)
  RolloutPlan ..> OperationalOwnership : owner từng stage
  ObservabilityDesign ..> OperationalOwnership : owners
```

### 9.7 Vòng đời obligation và maturity

```mermaid
flowchart LR
  SRC["Nguồn: SRS · SAD · risk · profile"] --> OB["DesignObligation"]
  OB --> AD{"ApplicabilityDecision"}
  AD -- not_applicable<br/>có chứng minh --> NA["Đóng obligation"]
  AD -- unknown --> GAP["DesignGap<br/>owner · blockingGates"]
  AD -- applicable --> RES["ObligationResolution<br/>designRefs + evidence"]
  RES --> CHK{"Rule TDD-0xx<br/>đúng revision"}
  CHK -- đạt --> OK["status verified / accepted"]
  CHK -- không đạt --> GAP
  GAP -- bổ sung thiết kế --> RES
```

```mermaid
stateDiagram-v2
  direction LR
  [*] --> identified : biết entity
  identified --> specified : đủ nội dung
  specified --> reviewed : có review
  reviewed --> verified : checks đúng revision
  verified --> accepted : authority (nếu policy yêu cầu)
  accepted --> [*]
```

### 9.8 Rule catalog theo nhóm

```mermaid
flowchart LR
  subgraph G2["Gate G2: thay đổi và tái sử dụng"]
    R20["TDD-020 side effect mới → reassess"]
    R21["TDD-021 no_change cần chứng minh"]
  end
  subgraph G3["Gate G3: thiết kế đủ và đúng"]
    direction TB
    A["Tổ chức: TDD-001, 002, 022, 023"]
    B["Behavior: TDD-003, 004, 005, 006, 024"]
    C["Contract và lỗi: TDD-007, 008, 009, 010"]
    D["Data: TDD-011, 012, 013, 014"]
    E["Bảo mật và vận hành: TDD-015, 016, 017, 018, 019"]
  end
  A --- ORGN[Design organization]
  B --- BEHN[Behavior]
  C --- CONN[Contracts]
  D --- DATN[Data]
  E --- OPSN[Operations]
```

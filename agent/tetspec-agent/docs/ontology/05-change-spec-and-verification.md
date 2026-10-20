# 05 — Change Spec, implementation và verification

CS là ontology độc lập về gói thay đổi cần thực hiện. Verification là module evidence liên kết downstream, không đổi ownership criteria của CR hoặc requirements của SRS.

## 1. Change Spec data dictionary

| Entity | Fields | Quan hệ/constraints |
|---|---|---|
| ChangeSpecification | `purpose! deliveryScope! inputBaseline! readiness!` | `items+`, `constraints*`, `projections*`; no-op dùng disposition thay empty fake spec |
| ChangeItem | `action! desiredEffect! rationale! impactClass!` | `targets+ verificationCriteria+`, M08/M09/M11 bắt buộc |
| ChangeTarget | `kind! repositoryRef! baselineRevision! locator! resolutionState!` | Code/config/schema/interface/infrastructure; add dùng planned target hợp lệ |
| BehaviorChange | `beforeContract? afterContract? preservedObligations+ compatibilityImpact!` | Action before/after semantics như SRS nhưng khác identity |
| VerificationCriterion | `subject! preconditions! stimulusOrInspection! expectedResult! method! requiredEvidence!` | CR-derived mapping+, source SRS/design refs+ |
| ImplementationTask | `description! assignedRole! dependencies* completionDefinition!` | realizes items+; nhiều task có thể phục vụ cùng item |
| DeliveryConstraint | `kind! predicate! sourceDesignRefs+ enforcementPoint!` | Ordering/compatibility/migration/security |
| AtomicDeliveryGroup | `members+ atomicityMeaning! failureHandling! groupGate!` | Không dùng để che dependency cycle chưa hiểu |
| ProjectionRecord | `canonicalRevision! inputDigest! adapterId! adapterVersion! artifacts+ mappingManifest! lossReport! fidelityReport!` | Không cấp approval |
| ImportCandidate | `sourceProjection! proposedEntities* unresolvedRefs* detectedLosses*` | Phải qua canonical gates, không ready từ tool status |

Change target phân biệt **tồn tại** và **sẽ tạo**. Modify/remove phải resolve target trong repo snapshot hoặc external target manifest. Add có proposed path/symbol, collision check và source design; không đòi file tồn tại trước khi implement. Locator không là entity identity.

Verification criteria cover positive, negative, failure, compatibility và removal obligations theo scope. Implementation tasks là cách tổ chức việc; một task nhỏ như thêm index vẫn cần mapping từ change item tới SRS quality/behavior requirement và actual TDD. Không tạo requirement giả nếu thiếu; ghi gap và clarify upstream.

## 2. Verification data dictionary

| Entity | Fields | Quan hệ/constraints |
|---|---|---|
| TestSpecification | `purpose! subjectRefs+ preconditions! stepsOrGenerator! oracle! requiredEnvironment! coverageClaims+` | automated/manual/model/demo/inspection methods |
| TestAssertion | `expectedPredicate! observedValueBinding! tolerance? criterionRefs+` | Assertion độc lập test name |
| TestExecution | `testRevision! buildManifest! environmentSnapshot! startedAt! endedAt! result! assertionResults+ artifacts+ runnerIdentity!` | pass/fail/error/skipped/aborted; retry execution có ID mới |
| AssertionResult | `assertionRevision! actualObservation! outcome! evidenceRef!` | Expected predicate không chỉnh theo observed |
| BuildManifest | `repositorySnapshots+ artifactDigests+ configDigest! dependencyLockDigest? generationProvenance!` | Cấu hình cũng là tested artifact |
| ImplementationArtifact | `kind! revision! contentHash! targetRefs+ changeItemRefs+ authoringProvenance!` | Code/config/DDL/IaC… |
| EnvironmentSnapshot | `runtimeVersions! relevantConfig! resourceProfile! externalDependencies! datasetRef?` | Không lưu secrets; reproducibility scope explicit |
| ManualVerification | `procedureRevision! subjectDigest! observations+ result! verifier! timestamp! evidence+` | Không ghi pass chỉ bằng chữ “đã xem” |
| MeasurementExecution | `measurementDefinition! observations+ environment! methodVersion! result!` | Target vs measurement tách entities |
| VerificationReport | `baseline! requiredChecks+ executions* missingChecks* staleChecks* outcome!` | Completeness gồm missing/error |
| AcceptanceReceipt | `crRevision! criterionRevisions+ deliveryScope! evidenceRefs+ decision! actor! role! payloadDigest! decidedAt! conditions*` | accepted/rejected/conditional; conditional không full accepted |
| ReleaseManifest | `releaseId! buildManifest! approvedSpecRevisions+ verificationReport! rolloutState!` | Release evidence khác BA acceptance |
| OperationalObservation | `releaseRef! metricOrIncident! window! environment! evidence!` | Trigger drift/incident, không tự rewrite normative design |

Test pass ở build cũ có thể tái sử dụng chỉ khi applicability reassessment chứng minh affected inputs/assertions/environment không đổi và có reusable-evidence decision. Không giữ nguyên pass bằng cách đổi build ID trên receipt. Flaky tests giữ toàn bộ attempts; policy pass không được xóa thất bại hay bỏ assertion mandatory.

## 3. Canonical→adapter contract

Adapter interface thiết kế: `export(canonicalSnapshot, adapterProfile) -> artifacts + mappingManifest + lossReport + fidelityReport`; `import(artifacts, sidecar?) -> candidates + unresolved + losses`. Đây là contract, chưa là API đã triển khai.

Mapping manifest cho mỗi canonical field/obligation: source identity/revision, artifact path/anchor/hash, transform, round-trip support, information-loss category. Sidecar cũng pin schema/adapter version. `loss = none | presentation_only | semantic | trace | authority`; mandatory semantic/trace/authority loss chặn publication. Không import tool approval/task checkbox thành human acceptance.

OpenSpec projection: proposal từ CR/CS rationale; design từ actual TDD/SAD; specs từ behavior changes và SRS; tasks từ work breakdown. MODIFY phải giữ surviving scenarios; REMOVE phải giữ nghĩa vụ loại bỏ/compatibility, không chỉ xóa paragraph. Adapter Markdown/JSON hoặc issue-tracker phải dùng cùng canonical gate.

## 4. Rule catalog

| ID | Invariant / điều kiện | Evaluation | Gate |
|---|---|---|---|
| CS-001 | Chưa human impact approval hoặc actual realization đủ thì không invoke CS authoring | workflow preflight | G4 |
| CS-002 | Every eligible item có stable ID, action, target, SRS và actual TDD sections, CR-derived criteria | shape + dual-source resolver | G4 |
| CS-003 | Item semantic effect phù hợp SRS context và actual design, không invent behavior/design | semantic refinement review | G4 |
| CS-004 | Modify/remove target resolve baseline; add target noncolliding và thuộc approved scope | repository target resolution | G4 |
| CS-005 | Preserved behavior/scenarios không bị rơi khi modify; removal có decommission/compatibility design | before-after fidelity | G4 |
| CS-006 | Task dependency DAG hoặc explicit atomic group có executable meaning; task completion không pass criteria | dependency/group checks | G4 |
| CS-007 | Criteria có assertions/method/expected outcomes đúng CR lineage; negative/failure obligations không mất | oracle coverage review | G4 |
| CS-008 | Projection pin canonical/version/hashes và mandatory semantics không loss | round-trip/fidelity validation | G4 |
| CS-009 | Import tạo candidates; thiếu references/authority không suy ra từ OpenSpec archive/checkbox | import trust-boundary check | G4 |
| CS-010 | Production publication recheck nguồn và target inventory dưới atomic transaction/lock | optimistic concurrency check | G4 |
| VER-001 | Test definition khác execution; receipt pin test/build/config/environment và timestamps/logs | evidence resolver | G5 |
| VER-002 | Required assertions executed đúng oracle; test name/result tổng không đủ chứng minh criteria | assertion-level coverage | G5 |
| VER-003 | Fail/error/skipped/aborted/missing/stale mandatory execution không thành verified | outcome aggregation | G5 |
| VER-004 | Code/config/schema changes đều mapped tới approved items; unplanned normative behavior chặn | implementation diff review | G5 |
| VER-005 | Performance/recovery/security achievement dùng actual measurement phù hợp target scope | measurement applicability | G5 |
| VER-006 | BA acceptance có đúng authority, CR/criterion revisions, scope và supporting evidence | receipt/role validator | G6 |
| VER-007 | Conditional/partial acceptance không full closure; multi-run criteria aggregation đúng composition | acceptance aggregation | G6 |
| VER-008 | Evidence reuse có reassessment record; không sửa receipt/build ID hoặc bỏ failed attempts | evidence history integrity | G5 |
| VER-009 | Release/operation drift tạo new findings và affected evidence stale; không sửa accepted baseline âm thầm | drift-to-change reconciliation | G6 |

## 5. Readiness projections

Report riêng `specReady`, `implementationComplete`, `verificationComplete`, `businessAccepted`, `releaseReady`, `operationallyValidated`. Một dự án có thể verified nhưng chưa BA accepted, hoặc BA accepted chức năng nhưng chưa đủ deployment prerequisites. Không gộp chúng thành một state “done”. Release gate theo policy riêng; ontology không tự triển khai hay phát hành khi acceptance event xuất hiện.

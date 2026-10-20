# 00 — Foundation, identity, assertion và logic

## 1. Quy ước data dictionary

`!` = đúng một giá trị bắt buộc; `?` = 0..1; `+` = 1..*; `*` = 0..*. `Ref<T>` phải resolve đúng identity/revision/type. Enum đóng chỉ mở rộng qua schema/profile version; extension không được thay nghĩa field chuẩn. Mảng dạng set không có thứ tự; sequence phải dùng `position` hoặc order semantics tường minh. Field domain chưa biết dùng `KnowledgeGap`, không dùng chuỗi `TBD` làm giá trị hợp lệ. Null khác empty, false, zero và unknown.

Tất cả semantic entity revisions có envelope:

`{identity: EntityId!, revision: RevisionId!, type: QualifiedType!, project: Ref<Project>!, scope: Ref<Scope>!, content: Object!, provenance: Ref<Assertion>+, createdBy: Ref<Actor>!, recordedAt: Instant!, supersedes: Ref<EntityRevision>*, lifecycle: DomainState!}`.

Draft do người soạn cũng cần authoring assertion. Candidate chưa resolve đủ có envelope riêng `CandidateRecord`, không giả là accepted revision. `supersedes` là DAG; split/merge khai báo lineage operations, không lạm dụng quan hệ supersedes một-một.

## 2. Entities dùng chung

| Type | Fields ngoài envelope / invariants |
|---|---|
| Project | `id! name! owners+ policyProfile!` |
| ProductScope | `id! product! subsystem? release? variant?` |
| Scope | `project! productScopes+ environments* actorClasses* workloadPredicate? validInterval? featurePredicate? exclusions*` |
| EntityIdentity | `qualifiedId! owningDomain! entityKind! stableExternalIds*` |
| EntityRevision | Envelope; body bất biến, metadata audit thêm qua event |
| ArtifactRevision | `artifactId! revision! format! uri! bytesHash! bytesLength! mediaType! accessClassification!` |
| DocumentRevision | `artifactRef! declaredRoles+ language! documentOwner! sectionInventory! authorityState!` |
| SectionAnchor | `documentRef! stableSectionId! locator! excerptHash! extractionSpan?` |
| SourceSpan | `artifactRef! start/end? page/region? table/row/column? diagramElement? rawText?` |
| Assertion | `subject! predicate! object! polarity! modality! scope! sourceAnchors* authoringEvent? derivation? authority! epistemicState!` |
| Derivation | `inputAssertions+ ruleRef! executionRef! outputAssertion! assumptions*` |
| Evidence | `kind! subjectRefs+ artifacts+ producer! capturedAt! applicabilityScope! integrityStatus!` |
| KnowledgeGap | `subject! missingPropertyOrObligation! reason! owner! blocksGates+ status!` |
| ConflictSet | `members+ conflictKind! overlapScope! witness? status! resolutionDecision?` |
| DecisionRecord | `subjectRefs+ decision! rationale! actor! roleAssignment! payloadHash! decidedAt! evidence+` |
| ReviewRecord | `subjectDigest! reviewKind! findings* reviewer! reviewedAt! outcome!` |
| RoleAssignment | `actor! role! project! domainScope! validFrom! validTo? issuer!` |
| TerminologyEntry | `term! language! definition! domain! scope! aliases* externalConceptRefs*` |
| ConceptAlignment | `left! right! relation! rationale! evidence+` với relation exact/narrower/broader/related/disjoint |
| BaselineManifest | `id! members+ membershipHash! policyVersions+ inventoryRefs+ status!` |
| Inventory | `scope! expectedSources+ discoveredMembers* exclusions* completenessEvidence+ assessedAt!` |
| LineageOperation | `kind! beforeRefs* afterRefs* identityRationale! decisionRef?` |

Mỗi revision được chọn tối đa một lần theo identity trong cùng baseline **và cùng effective scope**. Hai biến thể được giữ khi scopes tách biệt và explicit. Baseline chứa document revisions, semantic revisions, mappings, rule/profile versions; hash graph phải dùng canonical serialization version pinned. Không hash trực tiếp thứ tự tùy ý của JSON/set.

Scope không khai báo không có nghĩa all. Mỗi dimension có mode `all | restricted | unknown | not_applicable`; restricted có values/predicate, all phải được source/policy cho phép, unknown làm overlap/applicability unresolved. Collection rỗng có ý nghĩa theo property definition, không dùng chung cho all/none/missing. Scope inheritance chỉ qua declared parent và phải giữ provenance; override không được nới normative restriction upstream.

Một assertion có ít nhất một trong source anchors, authoring event hoặc derivation hợp lệ. Đối với normative requirement/design phải có nguồn normative hoặc human-authorized authoring, không đủ với derived inference của agent. `epistemicState = candidate | asserted | reviewed | disputed | retracted`; nó độc lập `authority = normative | informative | assumption | proposal | derived` và độc lập lifecycle entity.

## 3. Datatypes, constants và tham số

| Type | Định nghĩa |
|---|---|
| Identifier | Namespace + stable local ID; case/Unicode normalization policy cố định |
| Instant / Interval | Timestamp có timezone; interval có inclusive/exclusive bounds; validFrom ≤ validTo |
| Decimal | Decimal chính xác cho tiền/ngưỡng cần chính xác; precision/rounding explicit |
| Quantity | `value! unit! dimension! precision? tolerance?` |
| Money | `amount! currency! scale! rounding! valuationTime?` |
| Measurement | `metric! statistic! value! unit! population! window! environment! method! sampleEvidence!` |
| Threshold | `metric! comparator! quantity! statistic? window? scope!` |
| Range | `lower? upper? lowerInclusive! upperInclusive! unit!` ít nhất một bound |
| EnumDefinition | `values+ unknownPolicy! version! aliases*` |
| ConstantDefinition | `name! typedValue! scope! sourceAuthority! effectiveInterval! owner!` |
| ParameterDefinition | `name! datatype! allowedDomain! default? overridePolicy! sensitivity! owner!` |
| ParameterBinding | `parameterRef! value! scope! precedence! source!` |
| Calculation | `inputs+ expression! resultType! rounding? assumptions*` |

Constant là cố định **trong revision/scope**, không có nghĩa bất biến toàn dự án mãi mãi. SRS sở hữu business constants; SAD sở hữu architecture limits; TDD sở hữu design constants/parameters. Các domain tham chiếu hoặc map constraints, không âm thầm copy cùng tên. Default không phải giá trị thực tế; configured không phải measured. Ví dụ SRS timeout≤3s khác TDD timeout default=3s và deployment override=5s: phải so sánh effective binding.

Đổi đơn vị phải có dimension-compatible conversion record. Không tự chuyển calendar day thành 24h, p95 thành mean, MB thành MiB, throughput/window thành instantaneous rate. Currency conversion phải có nguồn rate và thời điểm nếu nghiệp vụ yêu cầu. Bounds có tolerance chỉ khi upstream cho phép.

## 4. Ngôn ngữ biểu thức logic có kiểu

Thiết kế `Expression` AST để lưu luật nghiệp vụ/constraints mà không chạy arbitrary code. Nodes: `literal`, `ref`, `and`, `or`, `not`, `implies`, `eq/ne/lt/le/gt/ge`, `in`, `isPresent`, `add/sub/mul/div`, `count/sum/min/max`, `forall/exists` trên tập được bound, `before/after/during`, `call` tới registered typed function. Functions ngoài tập này cần registry entry có type signature, deterministic semantics và implementation version; unsupported trả unknown.

Mỗi Expression có `id, ast, freeVariables, inputTypes, resultType, scope, semanticsVersion`. Predicate phải trả Boolean hoặc unresolved; chia zero, unit mismatch, collection chưa đóng, ref missing là error/unknown tương ứng, không false. Formula được đánh giá trên baseline pin; ghi witness/counterexample hoặc proof certificate/report.

Logic cho fact: **supported, refuted, both, unknown** — giữ riêng support và refutation. Nếu hai nguồn đủ thẩm quyền phát biểu P và ¬P trong overlapping scope thì both, không collapse thành unknown/pass. Evaluation dùng `pass | fail | unknown | not_applicable`; execution dùng `completed | unsupported | error | not_run | stale`. Only completed+pass hỗ trợ gate pass. Đây là hai trục khác nhau.

Đối với invariants safety: chứng minh một counterexample hợp lệ đủ fail. Đối với liveness/unbounded properties: checker bounded chỉ chứng minh trong bounds, không được tuyên bố general proof. Natural-language predicate chưa formalize cần human semantic review; không ép thành boolean bằng confidence score.

## 5. Temporal và identity semantics

Lưu hai thời gian: **valid time** (phát biểu có hiệu lực nghiệp vụ khi nào), **recorded time** (hệ thống biết lúc nào). Correction hồi tố tạo assertion/event mới; không sửa audit log. Revision ID không dùng thay effective date. Approval cho một payload tại thời điểm xác định không tự hiệu lực cho tương lai đã đổi.

Alias đổi tên giữ identity khi nghĩa không đổi; identical labels không chứng minh identical identity. Split/merge cần mapping old→new và obligation reassignment; removal dùng tombstone. Cấm hard-delete normative revisions có references; retention/redaction thực hiện bằng policy và audit marker, không giả source vẫn verify được khi bytes không còn.

## 6. Authority, confidentiality và integrity

Scope quyền của actor phải resolve tại thời điểm quyết định. Agent được trích xuất, đề xuất, kiểm logic trong supported scope; không đóng vai human reviewer. Evidence có hash không tự chứng minh nguồn đáng tin hay người ký có thẩm quyền. Proof of origin và proof of content là hai checks.

Fact nguồn hạn chế truy cập vẫn tồn tại trong inventory; consumer thiếu quyền nhận `unavailable/restricted` và blocker nếu bắt buộc, không bỏ nó khỏi denominator. Export không phát lộ source bí mật; có redacted projection và access-bound evidence resolver. Secrets trong source được thay secure reference khi đưa vào graph; chứng cứ raw nằm kho được kiểm soát.

## 7. Foundation rules

Các cột rule thống nhất: ID | invariant/vi phạm | cách đánh giá | gate đầu tiên bắt buộc. Mọi rule ở đây là mandatory khi applicable; applicability unknown chặn gate. Xem contract execution tại 08.

| ID | Invariant / điều kiện | Evaluation | Gate |
|---|---|---|---|
| FND-001 | Qualified identity duy nhất; reference resolve đúng type/revision/project hoặc explicit external contract | structural resolver | G0 |
| FND-002 | Revision immutable; supersedes/lineage không cycle và không tự tham chiếu | digest + DAG check | G0 |
| FND-003 | Normative assertion có source bytes/anchor verify được hoặc authorized authoring event; derived có đủ inputs | provenance traversal | G1 |
| FND-004 | Scope overlap được xác định trước so sánh conflict; scope unknown không suy ra disjoint | predicate evaluator / review | G1 |
| FND-005 | Quantity, threshold, constants có kiểu/unit/dimension phù hợp; phép chuyển có nguồn | type/unit checker | G1 |
| FND-006 | Accepted snapshot không chọn hai revision conflicting cùng identity/effective scope | baseline set + overlap check | G1 |
| FND-007 | Approval role hợp lệ, actor thật đúng loại và payload hash khớp | identity/policy check | G0 |
| FND-008 | Both/unknown/unsupported/error/stale không trở thành gate pass cho mandatory rule | outcome aggregation | ALL |
| FND-009 | Alias/rename không đổi nghĩa nếu không có lineage revision; split/merge có phân bổ obligations | diff + review | G1 |
| FND-010 | Constants/bindings overlapping không có hai effective values khác nhau cùng precedence | scope/binding resolution | G1 |
| FND-011 | AST type-safe, quantified domain có completeness, function version supported | static + execution check | G1 |
| FND-012 | Hash/anchor/inventory mismatch tạo stale/unresolved; không silent rebind theo heading | source resolver | ALL |
| FND-013 | Derived graph không có self-support proof cycle; conclusion giữ input/rule dependencies | provenance DAG | G1 |
| FND-014 | Valid-time và recorded-time hợp lệ, timezone/bounds rõ; receipt không backdate giả | temporal check | G0 |
| FND-015 | Redacted/unavailable source không được báo verified nếu không có authorized verifier receipt đủ scope | evidence/access check | ALL |
| FND-016 | Domain extension không nới mandatory core; profile/version collision hoặc unsupported check chặn | registry compatibility | ALL |

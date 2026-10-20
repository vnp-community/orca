# 01 — CR ontology: nhu cầu, phạm vi và nghiệm thu

Áp dụng envelope/datatype trong [00](00-foundation-and-logic.md). Ontology này sở hữu lý do thay đổi và quyền cho phép thay đổi. Nó không sở hữu giải pháp triển khai. Các entity `BusinessNeed`/`BusinessObjective` có thể xuất phát từ URD khi khởi tạo; không bắt buộc đổi tên tài liệu URD thành CR.

## 1. Data dictionary

| Entity | Fields | Quan hệ/cardinality và constraints |
|---|---|---|
| ChangeRequest | `title! changeKind! requester! accountableOwner! urgency! rationale!` | `addresses BusinessNeed+`, `hasScope ScopeItem+`, `hasCriterion AcceptanceCriterion+` khi authorized |
| BusinessNeed | `statement! affectedStakeholders+ context! painOrOpportunity!` | `supportedBy Evidence*`; thiếu evidence khi chỉ là nhận định phải ghi assumption |
| ProblemStatement | `observedBehavior! expectedOutcome! affectedPopulation! impact! occurrenceContext!` | `supports BusinessNeed+`; expected outcome chưa được duyệt không thành SRS |
| BusinessObjective | `outcome! successMeasure? horizon? owner!` | `pursuedBy CR+`; objective định tính cần acceptance method phù hợp |
| Stakeholder | `partyRef! interest! responsibility!` | CR `involves Stakeholder+` |
| ScopeItem | `subjectRefOrDescription! disposition! applicability! exclusions*` | Thuộc 1 CR revision; disposition in/out; cùng overlap không vừa in vừa out |
| AcceptanceCriterion | `statement! subject! precondition! stimulus? expectedOutcome! verificationMethod! acceptanceOwner!` | `ofChange 1`; `supportsObjective*`; criterion revision stable ID |
| AcceptanceGroup | `members+ composition! completionScope!` | `all_of/any_of/threshold` explicit; cấm tự đổi all thành any để đạt coverage |
| SuccessMeasure | `metric! comparator! target! observationWindow! population! method!` | Có đơn vị/statistic khi định lượng |
| BusinessImpact | `affectedScope! impactKind! magnitude? assumptions* evidence*` | positive/negative, direct/indirect; không tự khẳng định ROI từ thiếu số |
| PriorityDecision | `priority! basis! actor! decidedAt! scope!` | Priority và urgency tách biệt; không cấp quyền bypass gates |
| ChangeDependency | `sourceCR! targetCR! relation! rationale! requiredState?` | requires/blocks/conflicts/duplicates/related; không self-dependency |
| Risk | `event! likelihood? consequence! owner! mitigation? residualDisposition?` | Mức unknown phải giữ, không zero |
| Clarification | `question! affectedRefs+ owner! blockingGates+ response? decision?` | Response khác accepted decision |
| AuthorizationDecision | `crRevision! scopeDigest! criterionDigest! decision! actor! rationale!` | authorized/rejected/deferred; foundation DecisionRecord metadata |
| CancellationDecision | `crRevision! reason! affectedDeliveryRuns* unwindPlan? actor!` | Sau implementation cần corrective work/rollback assessment |
| FulfillmentDisposition | `scopeItemOrCriterion! outcome! evidence+ approver!` | delivered/duplicate/not-required/deferred; deferred không accepted |

`changeKind` core: enhancement, defect, corrective, compliance, refactor, architecture, operational. Phân loại không thay criteria/SRS authority. Compliance là nhãn nhu cầu; nguồn quy định phải được xác minh theo quy trình chuyên môn của project, ontology không tự chứng nhận hợp pháp.

## 2. Criterion có thể kiểm chứng

Criterion phải có đối tượng, context, expected outcome và cách xác nhận. `verificationMethod = test | demonstration | inspection | analysis | business-observation`; test không bắt buộc cho criterion về đào tạo/bàn giao, nhưng evidence và acceptance owner vẫn bắt buộc.

Ví dụ chưa đủ: “hệ thống nhanh hơn”. Candidate gap: metric/population/threshold/window. Ví dụ đủ: “Trong workload W, percentile P của thao tác O ≤ T, đo bằng M”. Không tự điền P/T từ TDD. Một criterion có nhiều outcomes MUST split thành obligations hoặc khai báo composition; không đánh dấu pass từ một outcome.

Negative criteria (“không lộ dữ liệu ngoài project”) là obligation riêng có violation predicate, scope và phương pháp kiểm. Criterion không có trigger vẫn hợp lệ nếu là invariant/inspection, nhưng phải chỉ rõ cách quan sát.

## 3. Lifecycle và quyền

| Transition | Guard | Evidence / effect |
|---|---|---|
| draft → submitted | Need, requester, owner, scope định danh | Submission event |
| submitted → in_review | Reviewer assignment và nguồn truy cập được | Review record |
| in_review → authorized | Scope/criteria clear; conflicts chặn giải quyết; role policy hợp lệ | AuthorizationDecision pin revision/digest |
| in_review → rejected/deferred | Lý do và owner thông báo | Decision; không tạo delivery-ready |
| deferred → in_review | Context/revision mới hoặc reopen decision | Historical decision giữ nguyên |
| authorized → cancelled | Quyền hợp lệ; impact của cancellation được assess | Stop new work; completed work có disposition/rollback plan |
| rejected/cancelled → draft mới | Human reopen + revision mới | Không sửa decision cũ |

Delivery status độc lập: not_started, in_progress, partially_verified, verified, partially_accepted, accepted. Các status này là projections từ evidence theo criteria/scope. CR authorized không đồng nghĩa accepted. Nghiệm thu một release/partition không tự đóng toàn bộ CR.

BA chịu trách nhiệm nội dung/criteria; người được policy chỉ định authorize; Dev/SA không tự thay business criterion để làm thiết kế pass. Người nghiệm thu có thể khác requester/authorizer và phải có role assignment tại acceptance time.

## 4. Rule catalog

| ID | Invariant / điều kiện | Evaluation | Gate |
|---|---|---|---|
| CR-001 | Authorized CR có ít nhất một need, in-scope item và criterion; không tính placeholder | shape + semantic review | G0 |
| CR-002 | Scope in/out không mâu thuẫn trên overlapping applicability | scope SAT/review | G0 |
| CR-003 | Criterion có observable outcome và verification method đủ scope; vague criterion tạo gap | typed fields + BA review | G0 |
| CR-004 | Criterion định lượng có metric, threshold, unit, population/window phù hợp | type/measurement check | G0 |
| CR-005 | Criterion thuộc đúng CR revision và criterion group có explicit composition | reference/group check | G0 |
| CR-006 | Dependency requires/blocks không có cycle chưa được xử lý bằng delivery group có kế hoạch | dependency DAG | G0 |
| CR-007 | Duplicate disposition phải chứng minh scope/criteria tương đương và dẫn accepted sources; text similarity không đủ | semantic equivalence review | G0 |
| CR-008 | Authorization pin scope/criteria; đổi nghĩa làm authorization applicability stale | semantic diff + digest | G1 |
| CR-009 | Cancel không xóa implemented state; phải xác định hậu quả/rollback/corrective obligations | delivery closure check | G6 |
| CR-010 | Mọi software criterion trong delivery scope có SRS operationalization hoặc blocker; technical enabler có derivation | mapping coverage | G1 |
| CR-011 | Criterion phi phần mềm có owner và evidence plan riêng, vẫn trong acceptance denominator | coverage partition | G6 |
| CR-012 | CR accepted chỉ khi mọi mandatory acceptance group đạt và BA receipt đúng revisions/scope | acceptance aggregation | G6 |
| CR-013 | Partial delivery/conditional acceptance không trở thành full acceptance; điều kiện chưa đạt giữ open | state derivation | G6 |
| CR-014 | Defect CR pin observed evidence và expected normative behavior hoặc upstream clarification trước thiết kế | source comparison | G1 |

## 5. Câu hỏi ontology phải trả lời

Ai đã cho phép đổi phạm vi này, ở revision nào? Tiêu chí nào chưa có requirement? Hai CR đang tranh chấp cùng outcome nào? Hạng mục deferred nào ngăn closure? Một acceptance receipt đang dùng criterion trước hay sau thay đổi? Vì sao một CR được kết luận duplicate?

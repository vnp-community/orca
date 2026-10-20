# Proposal tổng hợp: chuỗi tài liệu và ba luồng TDD agent

Quy trình mục tiêu: **CR/URD → SRS delta → đánh giá tác động TDD được SA/Dev duyệt → bộ TDD thực tế → change spec → code/test/nghiệm thu CR**. SAD là ràng buộc cố định. Runtime ontology v0.6 đã cut over và giữ compatibility v0.5; authoring benchmark và human acceptance vẫn là điều kiện bàn giao cuối.

## Những điểm cần thay đổi

| Hiện trạng/điểm cần sửa | Contract mục tiêu |
|---|---|
| Intake động chưa phân rõ thẩm quyền nguồn | CR/URD quy định nhu cầu và nghiệm thu; SRS quy định thay đổi; TDD quy định thiết kế; SAD ràng buộc kiến trúc. KG, repo và prompt là dữ liệu bổ trợ, không thay nguồn bắt buộc. |
| Routing chung hoặc quyết định từ KG | Báo cáo trên SRS delta và toàn bộ inventory TDD; mỗi TDD có thêm/cập nhật/không đổi, lý do và trích dẫn. SA/Dev phải duyệt. |
| Sinh spec trực tiếp từ KG baseline | Agent spec chỉ chạy sau khi kế hoạch được duyệt và TDD sau bước 4 đã materialize đủ. |
| WorkItem/trace ID được coi là đủ | Mỗi CS item phải có mục SRS, section TDD thực tế và kiểm tra gắn CR acceptance; kiểm cả nội dung, phiên bản và độ phủ. |
| Prompt đổi thiết kế được local approval | Prompt trái SRS phải quay về CR/SRS clarification rồi bước 3; không thể approve một TDD mâu thuẫn để dùng ở bước 5. |
| Đổi kiến trúc trong cùng flow | SAD readonly; phát hiện không tương thích thì block và tách CR cho SAD. |
| Format/schema pass được coi là ready | Phân biệt planning, design, spec, implementation và BA acceptance; mỗi tầng cần evidence riêng. |

## Đầu vào, đầu ra và trách nhiệm

| Bước | Đầu vào | Đầu ra | Owner |
|---|---|---|---|
| 1. Lập CR | Nhu cầu nghiệp vụ | CR/URD có ID, phạm vi, acceptance criteria, tham chiếu tài liệu hiện hành | BA |
| 2. Lập SRS | CR/URD, SRS hiện hành nếu có, SAD | SRS mới/cập nhật và delta thêm/sửa/bỏ có section refs | BA + Dev/SA |
| 3. Đánh giá tác động | SRS delta và lineage CR, SAD, inventory TDD hiện hành | ImpactReport mỗi TDD có action, lý do, evidence; ImpactApproval | SA/Dev; agent hỗ trợ |
| 4. Materialize TDD | Kế hoạch được duyệt, nguồn đã pin | TDD mới/cập nhật, TDD reuse không đổi, realization manifest | Dev/SA; agent hỗ trợ |
| 5. Sinh change spec | SRS delta, bộ TDD sau bước 4, SAD, approval và realization | Bảng CS chuẩn và OpenSpec projection | Dev/agent |
| 6. Implement/nghiệm thu | Change spec và CR criteria | Code, test results, acceptance receipts | Dev + BA |

Agent trong phạm vi repository tiêu thụ và kiểm CR/SRS; không tự nhận trách nhiệm lập hoặc duyệt tài liệu BA. Repo context giúp resolve đối tượng code, không bù thiết kế thiếu.

## Contract dữ liệu cần bổ sung

| Contract | Nội dung bắt buộc |
|---|---|
| DocumentRef / SectionRef | Document ID, role, revision, content hash; stable section ID, số điều hiển thị, anchor/evidence. Mã phải resolve đúng nội dung và phiên bản. |
| SrsDelta | CR lineage, before/after revision, action thêm/sửa/bỏ, FR/BR sections và CR acceptance refs. |
| ImpactReport | Baseline SRS/SAD/TDD hashes, complete inventory, action từng TDD, phạm vi section, rationale/citations, findings và kế hoạch materialize. |
| ImpactApproval | Identity/role người duyệt, decision/time, hash của immutable decision payload và baseline inputs. Agent không tự tạo human approval. |
| TddSetSnapshot / Realization | Map baseline→actual post-step-4 revision; actual section refs, plan conformance, unchanged refs và trạng thái thực hiện đủ. |
| ChangeSpecItem | CS ID, add/modify/remove, code target, SRS refs, actual TDD refs, verification criteria và CR acceptance refs. Không đồng nhất với KG ChangeSet. |
| Test / AcceptanceReceipt | CS/CR criteria refs, version, actual execution/assertion/result; Dev/BA acceptance evidence. Test plan không phải test result. |

Approval bind kế hoạch và baseline; sửa TDD đúng kế hoạch ở bước 4 không tự làm approval stale. Realization chứng minh bộ TDD mới tuân kế hoạch. Thay đổi ngoài kế hoạch hoặc drift SRS/SAD/baseline không được kiểm soát phải quay lại bước 3.

## Quyết định tác động và các cổng

- **Thêm TDD:** SRS cần thiết kế chưa có trong bộ hiện hành.
- **Cập nhật TDD:** SRS đổi hành vi, contract, schema hoặc ràng buộc của thiết kế đã có.
- **Không đổi TDD:** thiết kế hiện hành bao phủ yêu cầu; vẫn phải có TDD/section refs. Đổi tham số chỉ reuse khi giá trị mới nằm trong miền và bảo đảm đã thiết kế. Không đổi TDD vẫn có thể cần đổi code/config.

Thiếu thông tin hoặc mâu thuẫn là finding chặn quyết định/approval, không tạo action thứ tư. Report phải kiểm inventory đầy đủ, không chỉ lát cắt dễ tìm. Trước approval chỉ được phân tích/preview thiết kế; không sinh change-spec items/artifacts để bypass bằng nhãn draft. Sau approval, bước 5 vẫn chặn nếu add/update chưa hoàn tất, planned anchor chưa trở thành section thực tế, nguồn stale hoặc SAD không tương thích. SAD hash không đổi chưa đủ chứng minh semantic conformance.

## Change spec và trace

| Mã việc | Hành động | Đối tượng code | Mục SRS | Section TDD thực tế | Tiêu chí kiểm tra / CR acceptance |
|---|---|---|---|---|---|
| CS-xxx | thêm/sửa/bỏ | module/hàm/schema/policy cụ thể | SRS-FR/BR-xxx + revision | TDD-ID §x.y + revision | assertion/expected result + CR-AC-xxx |

Đây là schema bảng, không phải yêu cầu PBOM mới. Dòng thiếu SRS/TDD hoặc CR-derived verification là finding ngoài eligible change spec. Mâu thuẫn SRS–TDD trả về bước 3. OpenSpec proposal/design/specs/tasks là projection của bảng này: không invent thiết kế để lấp TDD thiếu.

Trace **CR → SRS → TDD section → CS → test** cần kiểm cả reference resolution và semantic coverage. ID hợp lệ không chứng minh thiết kế đủ, test đúng hoặc CR accepted; denominator phải giữ cả gaps/unknown. Các obligation/profile chuyên biệt tiếp tục mở rộng theo domain và risk, không giới hạn bởi mười ví dụ và không được waive các cổng nguồn/approval.

## Ba agent

1. **Impact + TDD agent:** nhận nguồn động đã xác định role, phân tích SRS delta với SAD và inventory, lập report để SA/Dev duyệt; sau duyệt mới materialize add/update và giữ no_change. CR bổ sung/bug fix cần SRS xác định behavior mong muốn.
2. **Change-spec agent:** đầu vào cố định SRS delta + actual related TDD + SAD, approval/realization và repo context. Kiểm gates → CS table → openspec-propose → fidelity/trace/CLI checks. Không tự sửa TDD hoặc SAD để làm spec hợp lệ.
3. **Prompt-edit agent:** resolve intent/target, phân presentation và semantic. Presentation giữ semantics và revalidate refs; semantic phải tuân SRS, impact approval và realization. Intent trái nguồn quay upstream, lưu provenance, không silent rewrite SRS.

## Tổng hợp ownership proposal

Cập nhật **13 change hiện có**, không tạo umbrella change trùng scope. Giữ tám capability mở rộng và ba flow, bổ sung contract tại owner đúng chỗ.

| Change | Phần chịu trách nhiệm |
|---|---|
| establish-tdd-ontology-contract | Identity/role/trace entities của chuỗi tài liệu |
| enforce-canonical-ontology-validation | Canonical types/properties/versioned document và section refs |
| establish-design-obligation-contract | Obligation có SRS/CR lineage và actual TDD resolution |
| compose-specialized-design-profiles | Compose checks mở rộng; không bypass workflow |
| resolve-semantic-design-impact | Inventory, closure, baseline→realized scope và invalidation |
| assess-and-route-tdd-design-work | Report ba action, human approval, fixed SAD và realization plan |
| extend-behavioral-and-integration-design-semantics | Behavior/transaction/integration/security contracts trong TDD |
| extend-data-and-operational-design-semantics | Migration/load/HA/data/ownership contracts trong TDD |
| validate-design-obligations-and-coverage | Executed checks, stage gates, dual-source coverage và acceptance evidence |
| build-dynamic-tdd-agent-flow | Step-4 execution/render actual TDD theo approved plan |
| build-prompt-tdd-edit-flow | Prompt authority/semantic edits và presentation consistency |
| build-tdd-kg-spec-change-flow | Stage-5 sources, CS table và OpenSpec projection |
| harden-tdd-agent-semantic-quality | Fidelity/publication, source-gated authoring và benchmark acceptance |

Thứ tự triển khai: ontology identities + canonical validation → obligation → profiles + semantic impact → assessment/approval → typed domain contracts và TDD/prompt materialization → coverage evaluator/gates → CS/OpenSpec integration + quality acceptance. Có thể xây evaluator interface sớm; chỉ bật flow production khi toàn bộ prerequisites đạt acceptance. Schema/runtime v0.6.0 là bản active; v0.5 chỉ là compatibility target/artifact.

## Điều kiện nghiệm thu

Bắt buộc negative cases: chưa duyệt impact; stale nguồn; thiếu một TDD cần materialize; no_change thiếu design refs hoặc vượt parameter bounds; CS thiếu một nguồn; IDs đúng nhưng behavior mâu thuẫn; kiến trúc trái SAD; prompt bypass SRS; test assertion khác CR criterion. Positive cases gồm mixed add/update/no_change, reuse TDD nhưng đổi config, expected Step-4 revisions giữ approval hợp lệ, novel specialist profile và full trace tới actual test/BA receipt.

Validation OpenSpec chỉ kiểm planning/format. Contract lõi của workflow đã được apply additive trong `data/ontology_v0.5/o5/workflow.py` và các integration points; 98 runtime tests không tự chứng minh full workflow acceptance. Các gate real authoring benchmark và human acceptance còn mở trong các change active; những phần đã có implementation evidence đã được archive.

Đợt apply hiện tại bổ sung resolver semantic fixed-point dùng chung cho dynamic/prompt/spec, manifest `SemanticResolution`, typed `Quantity`/`Migration`/`Measurement`/`PermissionDecision`, resolved inheritance/conditional/maturity registry, per-need validator execution, kiểm source freshness của TDD sau materialize, dual-source CS validation, specialist checks cho failure/security/recovery và projection OpenSpec theo đúng action `ADDED/MODIFIED/REMOVED`. Bộ regression hiện tại đạt 98/98; đây là bằng chứng contract/runtime fixture, chưa thay thế benchmark sinh proposal bằng model thật và rubric BA/SA.

## Chốt nội dung TDD và ontology phù hợp

**Nguồn Step 5 là SRS + bộ TDD sau đánh giá/cập nhật + SAD**, kèm CR acceptance lineage. SRS nói cần đổi gì; TDD nói thiết kế/cách thực hiện; SAD giới hạn kiến trúc. KG phải biểu diễn đủ cả source refs và thiết kế TDD; không dùng TDD/KG đơn độc để suy ra change intent.

TDD là bộ tài liệu thiết kế theo capability/module hoặc phạm vi có trách nhiệm rõ ràng, không bắt buộc một file duy nhất cho toàn hệ thống. Mỗi tài liệu có ID, version, owner, scope và sections có identity ổn định.

| Phần TDD | Nội dung thiết kế |
|---|---|
| Phạm vi và nguồn | Mục tiêu, boundaries, SRS FR/BR và CR criteria được phục vụ, SAD constraints, TDD dependencies |
| Phân rã thiết kế | Component/module, trách nhiệm, interfaces và topology phù hợp SAD |
| Hành vi | Luồng chính/ngoại lệ, business rules, algorithm, decision table, state transitions và invariants |
| Hợp đồng tích hợp | API/event/message, input/output/errors, auth, version compatibility, timeout/retry/idempotency/fallback |
| Dữ liệu | Entity/schema/index, integrity, transaction/consistency, migration/backfill/rollback/retention |
| Chất lượng và vận hành | Workload/capacity, SLO/budgets, concurrency, HA/recovery, observability, rollout/rollback |
| Bảo mật | Trust boundaries, threats/controls, quyền, credentials, data protection và audit |
| Quyết định và trách nhiệm | Alternatives/trade-offs, rationale, ownership, external consumers/compatibility, assumptions/open issues |
| Kiểm chứng và trace | Design checks, test scenarios/expected results, CR/SRS→design mapping, evidence và unresolved obligations |

Phạm vi/nguồn, thiết kế liên quan, decisions/gaps và trace là phần lõi. Các chi tiết chuyên biệt được chọn theo applicability/risk/profile; không áp mọi mục lên mọi TDD và không bỏ mục bắt buộc chỉ vì không có node. N/A phải có lý do/evidence. CS tasks và actual test results là artifacts downstream liên kết TDD, không thay thế thiết kế hoặc biến TDD thành change spec.

Rà soát manifest mở rộng v0.5 cho thấy có RequirementVersion/Delta, KnowledgeNeed, GenerationProfile/Run, ReadinessReport, PromptEditRequest và RepoSnapshot. Đây là nền hữu ích nhưng chưa chứng minh full document workflow: các identities/approval/realization/CS contracts mới chỉ là requirements trong proposals. Audit trước đã phát hiện mandatory semantics còn lọt validation và closure thiếu loại design. Vì vậy ontology hiện hành **chưa đủ để vận hành workflow chốt**; cần apply các model và executed checks dưới đây, không chỉ thêm tên lớp.

Agent impact phải xử lý cả **hai chiều**: từng SRS delta có design coverage hay missing design; từng TDD inventory có add/update/no_change và evidence. Resolver đi qua requirements→behavior/contracts/data/quality/security→dependent designs và consumers, giữ uncertainty. Đối chiếu trước/sau, gồm removal và negative requirements; không coi không tìm được match là no_change. SA/Dev duyệt conclusion trước materialization và spec generation.

## Bộ change cuối cùng cần apply

| Đợt | Change cần apply | Điều kiện bàn giao |
|---|---|---|
| 1 | establish-tdd-ontology-contract; enforce-canonical-ontology-validation | Canonical role/version/section identities, typed models và migration validated |
| 2 | establish-design-obligation-contract | SRS/CR-linked obligations và TDD resolution |
| 3 | compose-specialized-design-profiles; resolve-semantic-design-impact | Open-ended applicability, complete inventory/closure và source freshness |
| 4 | assess-and-route-tdd-design-work | Bidirectional delta coverage, per-TDD decisions, human approval và plan |
| 5 | extend-behavioral-and-integration-design-semantics; extend-data-and-operational-design-semantics | Applicable TDD contents có typed semantics và fixed-SAD conformance |
| 6 | build-dynamic-tdd-agent-flow; build-prompt-tdd-edit-flow | Actual TDD materialization/render và prompt authority gates |
| 7 | validate-design-obligations-and-coverage | Executed specialist checks, realization/source gates, semantic trace/coverage |
| 8 | build-tdd-kg-spec-change-flow; harden-tdd-agent-semantic-quality | SRS+actual TDD+SAD → CS table → OpenSpec; fidelity, publication và quality acceptance |

Đây là thứ tự bàn giao tích hợp; có thể xây interface/checkers sớm theo dependency. Những phần đã có evidence được giữ và revalidate, apply các pending/reopened tasks thay vì viết lại toàn bộ. **Không apply pbom-asset-entitlement-evidence** vì là rejected duplicate fixture. Chỉ archive sau full acceptance. Không còn một luồng sản xuất “TDD-only → change spec”.

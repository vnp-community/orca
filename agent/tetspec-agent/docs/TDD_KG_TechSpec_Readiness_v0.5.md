# Yêu cầu KG TDD đủ thông tin để sinh tech spec — đề xuất v0.5

Ngày thiết kế: 2026-10-06. Cập nhật triển khai: 2026-10-07. Trạng thái: contract và safeguards v0.5 đã cài trong runtime; benchmark model thật và nghiệm thu độc lập chưa hoàn tất. Tài liệu này bổ sung CR-0001 và là hợp đồng chung cho ontology, harness và bộ sinh tech spec. TDD ở đây là Technical Design Document.

## 1. Kết luận rà soát

Ontology v0.4 bao phủ dữ liệu, hợp đồng, hành vi, vận hành, truy vết và thay đổi; đây là nền phù hợp. Tuy nhiên, **KG hợp lệ về cấu trúc chưa đồng nghĩa KG đủ để sinh tech spec có thể triển khai**. Việc lưu nguyên văn SourceBlock giúp khôi phục tài liệu nhưng không chứng minh đã mô hình hoá tri thức thiết kế.

| Phát hiện | Bằng chứng trong kho | Hệ quả và sửa thiết kế |
|---|---|---|
| Harness và handoff chọn hai cách nhập khác nhau | Harness §2 bắt buộc agent chỉ đọc UKG; handoff §3 cho đọc lát cắt khi thiếu KG | G0 tiếp nhận theo KnowledgeNeed, chuẩn hoá mẩu tri thức trước khi cấp cho bộ soạn; không bắt buộc số hoá toàn bộ URD/SRS/SAD |
| Harness dùng ontology và mã luật cũ | v0.2/v0.3, R-10..R-70 và `tests` trong harness; v0.4 dùng S/V/M/G/T/L/K/C/P/X | Ghim phiên bản và ánh xạ luật; dùng `design implemented_by WorkItem`, `design/requirement verified_by TestCase` theo quan hệ v0.4; không phát cạnh chưa khai báo |
| Cổng thay đổi bảo vệ tương đối | `changeset.py` so tập lỗi sau/trước; `validate4.json`: 6.422 fail, trong đó 2.093 mức error; 4.657 insufficient_data | Giữ kiểm không phát sinh lỗi mới nhưng thêm cổng đủ thông tin tuyệt đối cho phạm vi change, kể cả phụ thuộc ngoài phạm vi |
| Ràng buộc đặc tả chưa được JSON Schema tự thực thi | Schema Requirement/Scenario/WorkItem chỉ required id/name; `x-mandatory-at-specified` là metadata | Cổng ghi phải chạy schema và validator ngữ nghĩa/maturity; kiểm đủ các khe bắt buộc, không chỉ kiểm parse JSON |
| Bộ kiểm chưa thực thi đầy đủ ý nghĩa của luật | `validate3.py` S-07 chưa kiểm pattern; M-02 trả not_applicable; nhiều luật logic/phiên bản thiếu dữ liệu | Công bố danh sách luật đã cài, chưa cài, không áp dụng theo điều kiện; luật bắt buộc chưa cài làm readiness bị blocked |
| Truy vết có cạnh chưa đảm bảo nội dung triển khai | Requirement có acceptance chuỗi; Scenario có main_flow/end_conditions chuỗi; WorkItem có DoD nhưng thiếu cấu trúc phụ thuộc triển khai | Thêm requirement version, kịch bản kiểm chứng, mapping capability và work package có cấu trúc |
| Dựng lại cấu trúc thiếu tri thức quan trọng | render_report: recall từ 47,8%, code 7,4%, prose 37,3% | Bổ sung logic/hợp đồng/rationale; không dùng recall từ làm thước đo readiness |
| Gói ngữ cảnh cắt ở 2 bước có thể mất phụ thuộc | Harness §6 | Dựng bao đóng theo loại quan hệ và nhu cầu; báo rõ phần bị cắt, thiếu dữ liệu hoặc vượt ngân sách |
| Cổng/nguồn gốc còn mâu thuẫn | “8 cổng” nhưng G0..G8 là 9; source_ref bắt buộc trong ontology nhưng harness cho nguồn sinh bằng cạnh | Có 9 cổng TDD; phân biệt bằng chứng trích xuất và nguồn gốc suy ra, ghi snapshot cùng đường truy vết |

Số liệu trên được đối chiếu với báo cáo có trong gói v0.4, chưa chạy lại toàn bộ pipeline. Kết quả 11/11 tệp trùng băm và 15/15 test là kết quả handoff; không suy ra readiness từ các kết quả đó. Hai bản đặc tả v0.4 trong docs và gói data hiện giống nhau. Gói data giữ vai trò snapshot lịch sử; thay đổi yêu cầu được ghi ở docs.

## 2. Yêu cầu cốt lõi

**REQ-KG-TS-01:** Với mỗi capability/change nằm trong phạm vi, KG TDD theo ontology phải cung cấp đủ tri thức có nguồn gốc và phiên bản để sinh proposal, design, implementation tasks và requirement/scenario deltas. Bộ sinh không phải tự đoán hợp đồng, hành vi hay quyết định kỹ thuật còn thiếu. Thông tin phụ thuộc repo được lấy từ snapshot repo có nguồn và ghép vào context pack; không bắt buộc mọi đường dẫn mã nguồn tồn tại trong KG thiết kế độc lập.

Readiness là kết quả theo `(baseline KG, change intent, scope, target profile, repo snapshot)`, không phải thuộc tính cố định của toàn KG. Có hai mức đầu ra:

- `draft`: cho phép phát tài liệu với câu hỏi/giả định hiển thị rõ; chưa sẵn sàng triển khai.
- `implementation_ready`: đủ mọi nhu cầu bắt buộc trong phạm vi và bao đóng phụ thuộc, không có blocker/mâu thuẫn/giả định chưa duyệt ảnh hưởng triển khai.

Maturity `specified` có Gap hợp lệ vẫn chưa đủ để đạt implementation_ready. Nhu cầu không áp dụng phải có điều kiện và lý do kiểm được; thiếu nguồn là insufficient_data, không biến thành not_applicable hoặc pass. Không chặn vì thiếu dữ liệu ngoài phạm vi nếu đã chứng minh không liên quan.

## 3. Ma trận thông tin tối thiểu

| Nhóm | KG phải có | Điều kiện áp dụng |
|---|---|---|
| Mục đích và phạm vi | Vấn đề, kết quả mong muốn, capability ID ổn định, goals/non-goals, in/out/deferred scope, ưu tiên và căn cứ | Mọi change |
| Baseline và delta | Phiên bản hiện tại/đích; added/modified/removed/renamed requirement; nội dung trước/sau và lý do; tên cũ/mới khi rename | Mọi change; greenfield khai báo baseline rỗng |
| Yêu cầu và nghiệm thu | Statement có modality; tác nhân, điều kiện, trigger/input, kết quả quan sát được; happy/negative/boundary scenarios theo rủi ro; acceptance đo được | Mỗi requirement bị tác động |
| Thiết kế và quyết định | Thành phần chủ, trách nhiệm, tương tác, phụ thuộc, phương án chọn/loại và rationale; constraints/risks; điểm quyết định còn mở | Mọi change |
| API/event/hợp đồng | Request/response/schema, validation, authn/authz/scope, error mapping, timeout, retry, idempotency, concurrency; version/compatibility | Khi thay đổi giao diện hoặc xử lý đồng thời; khe không áp dụng có lý do |
| Dữ liệu | Entity/attribute/type/nullability/key/index/invariant; read/write; migration/backfill, compatibility, retention/classification | Khi tác động lưu trữ hoặc dữ liệu |
| Hành vi | Flow/state/transition/guard/action; ordering/transaction boundary, failure/recovery; thuật toán với input/output và pre/postconditions | Khi capability có hành vi tương ứng |
| Chất lượng và vận hành | QualityTarget/SLO có đơn vị, phạm vi, tải và cách đo; bảo mật; logs/metrics/traces/alerts; deployment/config/rollout/rollback | Theo hồ sơ và tác động change |
| Triển khai và kiểm chứng | WorkItem có thiết kế đích, DoD, thứ tự/phụ thuộc; TestCase có input/setup/expected và liên kết requirement; lệnh kiểm tra hoặc nhu cầu repo tương ứng | Mọi change ở mức implementation_ready |
| Context repo | Repo/revision, stack/conventions, vị trí module hoặc nhu cầu tìm vị trí, OpenSpec config/schema/spec baseline và thay đổi đang mở | Khi sinh change cho repo cụ thể |
| Provenance | ID/phiên bản phần tử, nguồn/trích dẫn/băm hoặc đường suy ra; trạng thái duyệt; assumptions/issues/deviations và người chịu trách nhiệm | Mọi dữ kiện có ảnh hưởng đầu ra |

## 4. Delta ontology đề xuất

Tận dụng Requirement, Scenario, Decision, Alternative, WorkItem, TestCase, ChangeRequest, ChangeSet, ChangeOp, Baseline, Artefact và quan hệ v0.4. Không đồng nhất ChangeSet thao tác cập nhật graph với thư mục OpenSpec change triển khai phần mềm.

| Bổ sung | Thuộc tính chính | Liên kết và kiểm tra |
|---|---|---|
| `Capability` mới | stable_key, purpose, scope, owner, lifecycle | Capability → Requirement (`has_requirement`, 1-n); Requirement có tối đa một capability chủ trong cùng baseline |
| `RequirementDelta` mới | kind added/modified/removed/renamed, rationale, old_name/new_name | Delta → ChangeRequest (`for_change`, n-1); Delta → RequirementVersion (`before_requirement`, `after_requirement`, n-1); kind quyết định đầu nào bắt buộc |
| `RequirementVersion` mới | logical_requirement_id, revision, statement, modality, acceptance | Version → Baseline (`in_baseline`, n-1); Version → Scenario (`has_scenario`, 1-n); không sửa version đã ghim |
| Mở rộng `Scenario` | given, when, then, category, fixtures/input và expected_result | Kịch bản gắn đúng requirement version; expected có thể kiểm được; giữ actors/main_flow cho tương thích |
| Mở rộng `WorkItem` | deliverables, validation_refs | WorkItem → WorkItem (`depends_on_work`, n-n, acyclic); giữ cạnh v0.4 thiết kế → WorkItem `implemented_by` |
| `GenerationProfile` mới | artifact types, mandatory needs, applicability predicates, adapter/version | Profile → KnowledgeNeed (`requires_knowledge`, 1-n); KnowledgeNeed từ CR-0001 được dùng chung cho nhập và xuất |
| `GenerationRun` mới | baseline, intent, scope, repo_revision, profile_version, status draft/blocked/implementation_ready, manifest_hash | Run → Artefact (`produces`, 1-n), Run → Baseline (`uses_baseline`, n-1), Run → GenerationProfile (`uses_profile`, n-1) |
| `ReadinessReport` mới | per-need outcome, blockers, missing/conflicting IDs, truncated dependencies, rule implementation coverage | Run → Report (`has_readiness_report`, 1-1); không chỉ có tổng điểm |

Các quan hệ mới ở bảng là đề xuất, chưa hợp lệ với validator v0.4. Khi cài v0.5 phải khai báo miền/đích, bội số, inverse, chính sách tác động và sinh lại toàn bộ artefact từ nguồn ontology chuẩn. Nếu dùng `ext` trong pilot, phải có extension schema và phiên bản; không coi ext tuỳ ý là đủ contract.

SourceRef trích xuất bắt buộc document/version/block/quote_hash và quote kiểm được. Phần tử thiết kế sinh mới ghi origin=own, derivation inputs có ID/version, Decision/Assumption liên quan và GenerationRun; không bịa quote cho dữ kiện suy ra. Ngoại lệ này phải được phản ánh nhất quán trong schema, luật maturity và cổng ghi.

## 5. Cổng readiness TS trong runtime v0.5

| Mã | Điều kiện pass |
|---|---|
| TS-01 | Mọi capability trong scope có mục đích, ranh giới và owner rõ |
| TS-02 | Mọi requirement delta có before/after đúng kind và đúng baseline; rename có định danh ổn định |
| TS-03 | Mọi requirement thêm/sửa có statement, modality, acceptance và ít nhất một kịch bản Given/When/Then kiểm được; removal có lý do và kiểm chứng tác động |
| TS-04 | Mọi nhu cầu bắt buộc theo hồ sơ có dữ liệu hợp lệ, nguồn/phiên bản và trạng thái duyệt phù hợp; mandatory insufficient_data hoặc validator chưa cài đều là blocker |
| TS-05 | Mọi delta có thiết kế đáp ứng hoặc quyết định xử lý removal; thiết kế có rationale và constraints liên quan |
| TS-06 | Mọi phần thiết kế bị đổi có WorkItem, DoD và validation; DAG đầu việc không chu trình và không tham chiếu treo |
| TS-07 | Bao đóng phụ thuộc đủ hoặc có external contract đã ghim; không bỏ dữ liệu âm thầm vì token/hop limit |
| TS-08 | Không còn conflict, stale source hay Gap/Assumption/OpenIssue chưa duyệt ảnh hưởng triển khai; ngoại lệ chỉ được phép cho draft |
| TS-09 | Mỗi statement/scenario/task/quyết định đầu ra có mapping tới KG hoặc repo evidence; không thêm dữ kiện ngoài context pack |
| TS-10 | Artefact đúng target profile, khớp nhau về capability/scope/delta; baseline/spec repo không đổi trong lúc sinh; thay đổi snapshot buộc kiểm lại |

Implementation_ready yêu cầu 100% nhu cầu bắt buộc và 100% truy vết delta → thiết kế → task/validation trong scope; 0 blocker. Các nhu cầu conditional phải được đánh giá điều kiện, không được loại khỏi mẫu số để tăng tỷ lệ. Duyệt thủ công không thay thế thông tin bắt buộc; waive phải theo chính sách và không được xoá blocker về hợp đồng/hành vi cần triển khai.

## 6. Adapter OpenSpec qua skill openspec-propose

Hợp đồng này lấy từ skill openspec-propose cục bộ đã đọc; khi chạy thực tế phải đọc lại config và instructions/schema của repo đích. Đây chưa phải lời khẳng định tương thích mọi phiên bản OpenSpec.

| Artefact | Tri thức đầu vào |
|---|---|
| `proposal.md` | ChangeRequest.rationale, mục đích/phạm vi; capability added/modified; impact lên module/API/data/dependency |
| `design.md` | Baseline/current behavior, goals/non-goals, root cause khi phù hợp; Decision/Alternative/rationale; hợp đồng và hành vi; risks, migration, rollback, validation |
| `tasks.md` | WorkItem theo DAG, deliverables, DoD và validation; checklist đủ cụ thể để triển khai |
| `specs/<capability>/spec.md` | RequirementDelta/RequirementVersion/Scenario; sections ADDED/MODIFIED/REMOVED/RENAMED theo hướng dẫn repo; MODIFIED mang đầy đủ requirement đích |

Thứ tự: đọc `openspec/config.yaml`, specs và changes liên quan → ghim repo revision/spec hashes → chọn tên change → đánh giá readiness → dựng context pack → chạy skill → kiểm artefact và trace manifest. Ưu tiên CLI theo skill khi có; sinh proposal không áp ChangeSet lên KG và không tự triển khai code. Kho hiện không có openspec/, nên lượt rà soát này chỉ bổ sung thiết kế.

Manifest phụ trợ lưu đường dẫn/anchor của artefact, KG IDs/versions, repo evidence, baseline hash, profile/skill version, blockers và trạng thái. Không nhét chi tiết nội bộ harness vào nội dung tech spec nếu không cần cho người triển khai. CLI validation đạt vẫn cần kiểm ngữ nghĩa/độ đủ và mapping.

Ví dụ minh hoạ: đổi giới hạn upload SBOM từ 50 MiB lên 64 MiB. KG cần before/after byte chính xác (52.428.800 → 67.108.864), API và cấu hình Gateway/service bị tác động, hành vi khi vượt giới hạn, error code có nguồn, rủi ro memory và rollout, tests tại limit−1/limit/limit+1, task cập nhật từng consumer. Nếu chưa biết error code hoặc compatibility thì sinh draft kèm blocker; không tự chọn HTTP status. Example này không phải thay đổi PBOM đã được duyệt.

## 7. Context pack và nghiệm thu

Context pack gồm intent/scope, baseline KG và repo hashes, profile, requirements/deltas/scenarios, thiết kế/hợp đồng/quyết định, phụ thuộc và tác động, work/test plan, nguồn, readiness report. Retrieval đi theo từng KnowledgeNeed tới bao đóng cần thiết; nếu quá ngân sách thì chia work package, giữ manifest chung và kiểm liên package. Không coi đủ 2 hops là đủ tri thức.

Bộ nghiệm thu cần các ca độc lập:

1. Greenfield có baseline rỗng; thêm capability sinh đủ artefact, requirement/scenario/task có trace.
2. Sửa hợp đồng API: kiểm before/after, full modified requirement, compatibility và consumer impact.
3. Rename/remove: không mất logical ID, không còn reference treo; kế hoạch retirement/migration khi cần.
4. Thiếu error mapping, state guard, migration hoặc acceptance: báo đúng need, sinh draft/blocked, không implementation_ready.
5. Hai nguồn mâu thuẫn hoặc upstream/repo thay đổi giữa lượt: chỉ ra cả nguồn, yêu cầu Decision hoặc chạy lại readiness.
6. Giả định chưa duyệt và luật bắt buộc chưa cài: không pass âm thầm.
7. Phụ thuộc ở ngoài 2 hops và context bị cắt: vẫn phát hiện thiếu; chia pack hoặc blocked.
8. Sinh hai lần trên cùng snapshot: so tính tương đương ngữ nghĩa, trace coverage và blocker; không bắt buộc văn bản giống từng byte.
9. Round-trip tech spec → facts: không mất requirement/scenario/constraint bắt buộc và không thêm fact vô nguồn; recall từ chỉ là số đo phụ.

Trình tự triển khai: chốt profile và fixture → cài delta ontology/validator → cài intake và readiness → context pack/adapter OpenSpec → pilot một capability → bộ vàng độc lập → mở rộng bộ TDD. Tiêu chí nghiệm thu là kết quả trên các ca trên, không phải chỉ số lượng lớp hoặc tổng pass của KG.


## 8. Mục tiêu ba luồng agent và nguồn prompt

Ontology dùng chung MUST phục vụ: (1) sinh TDD từ nguồn động SRS/URD/SAD và CR/bổ sung/fix; (2) sinh spec change từ KG TDD; (3) sửa TDD bằng prompt. Profile quyết định knowledge needs cho từng luồng, trong khi identity/version/provenance/ChangeSet/impact dùng chung.

Prompt là nguồn yêu cầu chỉnh sửa có request/user ID, text/hash, target và base snapshot; không tự trở thành evidence của thượng nguồn. Semantic edits đi qua ChangeSet, presentation-only edits giữ semantic facts. KG sau sửa phải được kiểm readiness lại và invalidation tới TDD/spec artefacts bị tác động. Chi tiết normative tại [bộ OpenSpec changes](../openspec/README.md).

## 9. Đối chiếu implementation hiện tại

`data/ontology_v0.5/o5/tools/techspec_rules.py` giữ ý nghĩa TS-01..TS-10 ở mục 5. QG-01..QG-10 trong `quality5.py` là kiểm graph/evidence/contract bổ trợ, không thay tên hoặc ý nghĩa TS. Profile TDD/prompt yêu cầu QG; profile spec yêu cầu cả QG và TS. KnowledgeNeed được kiểm riêng từng nguồn, kèm applicability, searched/matched/no-match/conflict và evidence.

Authoring thực hiện bằng runner tại `specgen/authoring.py`; CLI OpenSpec cung cấp instructions và validation, không thay agent viết proposal. Lifecycle có staging semantic/CLI trước publish, recheck snapshot dưới lock, draft ngoài active changes khi chưa đủ điều kiện. Manifest ánh xạ assertions tới anchor thực và evidence; fixtures kiểm lỗi status code vô nguồn, thiếu scenario, stale KG/code và collision.

PBOM intake hiện bảo toàn các loại rule/quality/entity/attribute/error/state/transition; catalogue API thiếu hợp đồng được giữ reference-only. Nguồn supplied thiếu Appendix A/B/C và có mismatch URD metadata v1.0(Draft) với SRS tham chiếu v1.2. Không được suy ra method/path/schema/work/test còn thiếu hoặc tự duyệt nguồn. Render structured facts có semantic round-trip; chưa là chứng nhận thiết kế PBOM đầy đủ.

Báo cáo kiểm thử và acceptance hiện tại tại [implementation-status](../openspec/changes/harden-tdd-agent-semantic-quality/implementation-status.md). Các checkbox lịch sử không thay acceptance evidence; per-task reconciliation được ghi riêng. Benchmark 3+3 lần cùng config và rubric người review độc lập vẫn là điều kiện đóng change.

# Luồng harness sinh TDD và tech spec — v0.2

Ngày: 2026-10-06. Trạng thái: đề xuất thiết kế, chưa cài harness. Bản này thay thế luồng vận hành v0.1, tiếp nhận quyết định nguồn động trong CR-0001 và bổ sung đầu ra tech spec. Số đo lịch sử/chi phí trong v0.1 vẫn là tham khảo, chưa phải benchmark sinh TDD. Ontology thực thi hiện có là v0.4; extension v0.5 được mô tả trong [hợp đồng readiness](TDD_KG_TechSpec_Readiness_v0.5.md).

Bản trình bày thống nhất sau khi runtime chuyển sang v0.6 nằm tại [Thiết kế hệ thống TDD Agent, Knowledge Graph và Tech Spec](TDD_Agent_And_KG_Design_v0.6.md). Tài liệu này được giữ để bảo toàn lịch sử proposal v0.2.

## 1. Luồng tổng thể

```text
Nguồn KG / Word / Markdown / hợp đồng / câu trả lời người dùng
  → G0 Tiếp nhận theo KnowledgeNeed
  → G1 Kiểm tri thức thượng nguồn cần cho scope
  → G2 Chia TDD, phân sở hữu và kế hoạch nhu cầu downstream
  → G3 Chốt hạt nhân dùng chung
  → G4 Sinh KG TDD qua cổng ghi
  → G5 Kiểm liên TDD và readiness theo profile
  → G6 Kết xuất TDD, kiểm ngữ nghĩa vòng tròn
  → G7 Duyệt
  → G8 Ghim baseline
  → S0 Nhận change intent và snapshot repo/spec hiện tại
  → S1 Tính delta, impact, dựng context pack và kiểm readiness
  → S2 Sinh tech spec (ví dụ openspec-propose)
  → S3 Kiểm artefact, trace và tính nhất quán; công bố draft/implementation_ready
```

Có 9 cổng TDD (G0..G8) và 4 bước downstream (S0..S3). Không bắt buộc chạy S0..S3 cho mọi lần sinh TDD. G5 kiểm khả năng cung cấp tri thức theo profile; readiness cho change cụ thể được đánh giá lại tại S1 với repo context. Luồng không tự triển khai code.

### Ranh giới ngôn ngữ và tech stack

Các file Python trong prototype hiện tại chỉ là harness, validator và CLI
adapter. Chúng không phải bằng chứng để chọn Python cho backend của dự án.
Ngôn ngữ, framework, runtime, thư viện, build tool và deployment target phải
được ghi theo từng component/service trong SAD, repository snapshot hoặc danh
mục công nghệ được duyệt. Khi chưa có nguồn hoặc có mâu thuẫn, agent phải
ghi `unknown/conflict` và giữ output ở `draft/blocked`; không tự mặc định
Python. Một hệ thống nhiều ngôn ngữ được phép nếu mỗi component có stack và
consumer/contract tương ứng.

## 2. Điều kiện cổng

| Cổng | Hợp đồng |
|---|---|
| G0 | Dựng KnowledgeNeed từ blueprint/profile; ưu tiên KG đã duyệt, sau đó đọc lát cắt tài liệu/hợp đồng. Chuẩn hoá fact và provenance vào intake store trước khi cấp context. IntakeReport ghi found/missing/conflict/low_confidence và resolution |
| G1 | Kiểm tính hợp lệ và nhất quán của lát cắt cùng phụ thuộc. Thiếu nhu cầu bắt buộc tạo blocker/câu hỏi; mâu thuẫn ghi cả nguồn và Decision phân xử. Thiếu dữ liệu không liên quan scope không chặn |
| G2 | Một owner cho mỗi phần tử thiết kế thuộc scope; nguồn dùng chung có owner riêng và consumer references. Kế hoạch bao gồm nhu cầu TDD lẫn nhu cầu sinh tech spec, thứ tự và priority có nguồn |
| G3 | Registry ID, constants/units, errors, events, scopes và naming được ghim phiên bản. Thay đổi qua ChangeSet có duyệt và impact, thay vì kho chỉ đọc vĩnh viễn |
| G4 | Propose element/relation vào staging; kiểm JSON Schema, maturity, miền/đích, ID, provenance và luật đã cài trước commit. Sinh `implemented_by`, `verified_by` đúng chiều v0.4 ngay lúc sinh. Sửa tối đa số vòng cấu hình; hết vòng chuyển review với blocker |
| G5 | Kiểm toàn bộ snapshot cho cấu trúc và liên TDD; kiểm đủ thông tin theo scope/profile, có báo cáo luật chưa cài. Không dùng “không phát sinh error mới” làm điều kiện duy nhất |
| G6 | Kết xuất từ snapshot; writer không thêm fact. Kiểm round-trip requirement, hợp đồng, logic, constants và trace; lỗi critical chặn, không chấp nhận chỉ vì recall từ đạt |
| G7 | Duyệt decisions/assumptions/waivers và artefact theo chính sách. Draft có thể giữ vấn đề mở rõ ràng; không gắn nhãn implementation_ready khi còn blocker |
| G8 | Ghim baseline KG, nguồn và artefact hashes; giữ lịch sử và lineage. Khi nguồn đổi, đánh dấu stale và chỉ sinh lại phạm vi impact đủ bao đóng |
| S0 | Nhận change intent/scope và target profile; đọc config/specs/changes/conventions của repo; ghim revision/hash. Greenfield khai báo baseline rỗng |
| S1 | Dựng RequirementDelta trước/sau, impact, design/task/test plan; đánh giá TS-01..TS-10. Thiếu thông tin quay về intake/thiết kế, không tự suy đoán |
| S2 | Adapter cung cấp pack cho skill; OpenSpec tạo proposal/design/tasks/spec deltas theo config repo. Mỗi assertion có trace trong manifest phụ trợ |
| S3 | Kiểm định dạng bằng công cụ đích khi có, kiểm ngữ nghĩa và truy vết, kiểm snapshot chưa đổi. Lưu GenerationRun, ReadinessReport và artefact; ghi rõ draft/blocked/implementation_ready |

X-03/X-05: chỉ so khi có giá trị/phiên bản thượng nguồn; thiếu trả insufficient_data. Kết quả này có chặn hay không phụ thuộc KnowledgeNeed của scope, không có quy tắc “thiếu nguồn luôn không chặn”. Không miễn trừ lỗi cấu trúc không waivable bằng một OpenIssue.

## 3. Context và công cụ

Không đưa nguyên tài liệu vào mỗi lượt. Context pack có task/scope, blueprint/profile version, owned elements, upstream facts và sources, kernel version, already-written snapshot, rules và implementation coverage, dependency closure, readiness report, budget và danh sách bị cắt. Không giới hạn cứng ở hai bước quan hệ; truy theo KnowledgeNeed và semantics của relation. Quá ngân sách thì chia gói có manifest chung, không bỏ ngầm dependency.

| API đề xuất | Tác dụng |
|---|---|
| `resolve_knowledge_needs(scope, profile)` | Tìm tri thức từ KG/tài liệu/hợp đồng, trả IntakeReport; chưa cài |
| `get_context_pack(task_id, snapshot)` | Pack bất biến và manifest, khai báo thiếu/cắt |
| `kg_query`, `search_upstream` | Chỉ đọc, có giới hạn và provenance |
| `alloc_id`, `propose_element`, `propose_relation` | Cấp ID và ghi vào staging có kiểm chứng |
| `run_rules(scope, snapshot)` | Kết quả pass/fail/waived/not_applicable/insufficient_data và trạng thái implementation từng luật |
| `assess_readiness(intent, scope, profile, snapshot)` | Kiểm TS-01..TS-10, trả per-need blockers; chưa cài |
| `build_techspec_pack(...)` | Delta + impact + repo evidence + mapping, chỉ đọc; chưa cài |
| `render`, `validate_artifacts` | Kết xuất và kiểm output |
| `apply_changeset` | Cập nhật KG nguyên khối có preconditions và kiểm impact; không phải sinh OpenSpec change |

Nguồn trích xuất giữ quote/block/hash/document version. Dữ kiện sinh mới giữ các input ID/version, đường suy ra và Decision/Assumption; không dùng quote giả. Cổng ghi cần adapter giữa graph envelope (`props`, `sources`, `evidence`) và schema phẳng, được kiểm tra để tránh mất field.

## 4. Vận hành và nghiệm thu

Sổ việc ghi task ID, input snapshot/hash, trạng thái pending/running/blocked/completed, attempt và output IDs. Commit có idempotency key và precondition version để resume không nhân đôi dữ liệu; báo cáo tiến độ theo task hoàn thành. Giới hạn concurrency cấu hình, retry/backoff và ngân sách theo task. Phần ghi kernel/owner registry có kiểm soát đồng thời; mô hình triển khai cụ thể chưa chốt.

Pilot chọn một capability/TDD có nguồn thật; đo intake completeness, độ đúng fact/trace, blocker detection, rule coverage, readiness và token/latency. Đối chứng TDD v1.0 chỉ là dữ liệu tham khảo vì còn lỗi; người duyệt độc lập xác lập bộ vàng. Nghiệm thu downstream gồm ca thêm/sửa/xoá/đổi tên, thiếu nguồn, mâu thuẫn, source stale, dependency ngoài 2 hops và lỗi snapshot, theo §7 hợp đồng readiness.

Thứ tự: profile/fixture → ontology và validators → intake/context → G2..G5 pilot → render/review/baseline → adapter OpenSpec và S0..S3 → mở rộng bộ TDD. Chọn storage, ngưỡng confidence và chính sách duyệt bằng benchmark/pilot; chưa coi các ngưỡng lịch sử của v0.1 là tiêu chuẩn đã chốt. Ánh xạ ISO trong tài liệu cũ vẫn chưa được kiểm chứng bằng văn bản tiêu chuẩn.


## 5. Luồng thứ ba: sửa TDD bằng prompt

Bổ sung mục tiêu: cùng ontology hỗ trợ ba luồng độc lập có interfaces chung. Luồng 1 sinh TDD từ nguồn động và CR/enhancement/defect; luồng 2 sinh spec change từ KG TDD; luồng 3 chỉnh TDD hiện có bằng prompt của user.

Luồng 3: prompt + target + base snapshot → resolve edit intent/target → intake nếu chỉ có văn bản → preview ChangeSet và impact → validation/review theo authorization policy → atomic KG commit → staged render và semantic check → publish TDD version mới. Presentation-only edit dùng presentation overlay, giữ semantic graph hash. Prompt rõ trong phạm vi đã uỷ quyền được xử lý tới hoàn tất; target mơ hồ cần clarification trước mutation.

Nếu prompt đổi thiết kế khác thượng nguồn, lưu Decision/Deviation và resolution theo policy; không sửa SRS/SAD âm thầm. Thay đổi shared kernel xét mọi consumer; artefact downstream bị đánh dấu stale. Render fail sau graph commit ghi committed_pending_render và retry từ cùng snapshot, giữ tài liệu công bố cũ. Undo dùng compensating ChangeSet với version checks.

Đề xuất triển khai cụ thể nằm trong [kế hoạch OpenSpec](../openspec/README.md), gồm nền ontology và ba change cho ba luồng. Requirements/scenarios của các change là hợp đồng nghiệm thu; các extensions vẫn chưa được cài.

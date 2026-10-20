# Rà soát pilot PBOM và luồng KG → OpenSpec

Ngày: 2026-10-06. Đối chiếu trực tiếp dữ liệu lưu trong `data/test-pbom/run-20261006`, `data/ontology_v0.4/o4/kg4.json` và mã nguồn o5.

## Kết luận

Pilot hiện chứng minh thao tác trên một fixture trích ID bằng regex; chưa chứng minh ba luồng agent sinh/chỉnh TDD và sinh spec change. Nhãn `implementation_ready` và kết luận apply hoàn tất trước đây không có đủ bằng chứng. OpenSpec CLI validation đạt chỉ xác nhận một phần cấu trúc, không xác nhận chất lượng kỹ thuật hay tương đương `openspec-propose`.

## So sánh KG

| Chỉ số | KG gốc v0.4 | Pilot ban đầu | Pilot sau CR |
|---|---:|---:|---:|
| Node | 12.871 | 574 | 578 |
| Loại node có dữ liệu | 62 | 11 | 13 |
| Cạnh | 15.482 | 230 | 234 |
| Loại quan hệ có dữ liệu | 55 | 4 | 6 |
| Block | 6.341 | 0 | 0 |
| Node ngoài Block | 6.530 | 574 | 578 |

KG gốc chứa 13 Document/TDD và cấu trúc tài liệu; hai tập không cùng phạm vi sinh đầu vào. Vì vậy số lượng không phải ngưỡng pass/fail hoặc tỷ lệ recall. KG gốc cũng chứa nội dung chưa đầy đủ/chưa review, không phải gold standard. Cần đánh giá trên cùng phạm vi requirement/design và evidence trước khi định lượng độ phủ.

| Loại node | Gốc | Ban đầu | Sau CR |
|---|---:|---:|---:|
| Component | 343 | 3 | 3 |
| Module | 18 | 7 | 7 |
| Operation | 438 | 268 | 268 |
| Entity | 127 | 46 | 46 |
| Attribute | 1.048 | 0 | 0 |
| Constraint | 589 | 3 | 3 |
| Decision | 54 | 15 | 15 |
| Requirement | 79 | 0 | 0 |
| BusinessRule | 60 | 0 | 0 |
| QualityTarget | 110 | 0 | 0 |
| RequirementVersion | 0 | 187 | 188 |
| Scenario | 48 | 41 | 42 |
| WorkItem | 353 | 2 | 2 |
| TestCase | 418 | 1 | 1 |
| Capability | 0 | 1 | 1 |
| ChangeRequest / RequirementDelta | 0 / 0 | 0 / 0 | 1 / 1 |

Pilot thiếu 53 loại có dữ liệu trong KG gốc, gồm Interface, ErrorCode, State/Transition, DataStore, Index, Control, Threat, Risk, Migration và các loại khác. Có 510 ID chung; 168 ID đổi class: 78 Requirement, 51 BusinessRule và 39 QualityTarget đều bị gom thành RequirementVersion. Versioning không thay thế việc bảo toàn ngữ nghĩa rule và quality target.

230 cạnh ban đầu chỉ gồm `has_requirement` 187, `has_scenario` 41, `implemented_by` 1 và `verified_by` 1. Sau CR thêm `has_requirement` 1, `has_scenario` 1, `for_change` 1, `after_requirement` 1. Không có mạng thiết kế chi tiết tương ứng các quan hệ gốc như `has_attribute`, `reads`, `writes`, `may_fail_with`, `satisfies`, `depends_on`, `transition_from/to`. KG gốc có 4.794 cạnh `binds`; bỏ riêng loại này còn 10.688 cạnh, vẫn không đồng nghĩa tất cả là cạnh thiết kế.

Đối chiếu đầy đủ mọi loại: [nodes-comparison.csv](../data/test-pbom/run-20261006/review/nodes-comparison.csv), [relations-comparison.csv](../data/test-pbom/run-20261006/review/relations-comparison.csv).

## Kiểm lại sáu kết quả đã báo

| Kết quả | Điều thực sự được chứng minh |
|---|---|
| 574 node, 230 cạnh, implementation_ready | Số đếm đúng; readiness kiểm tra presence của facts, chưa kiểm tra đầy đủ nội dung thiết kế. |
| Intake 11/11, không blocker | Cùng facts tổng hợp được gắn cho cả URD/SRS/SAD; chưa chứng minh từng nguồn đáp ứng knowledge need, mâu thuẫn hoặc thiếu thông tin. |
| Round-trip 574/574 | Renderer in ID/class/name và kiểm lại tập ID. Props, quan hệ, provenance, điều kiện và acceptance không được kiểm round-trip. |
| API-P-01 → Submit SBOM timeout 30s | Script truyền sẵn target/path/value, chỉ sửa name. API-P-01 được SRS gắn với SSO/login; không cập nhật timeout contract và không chứng minh hiểu prompt. |
| CR entitlement evidence | URD BRL-47 và FR-AST-12, SRS FR-AST-12/INT-07 đã yêu cầu chứng từ. Chưa phân biệt duplicate/clarification với yêu cầu mới; không sửa thiết kế, task, test liên quan. |
| Sau CR 578 node | Đúng: thêm ChangeRequest, RequirementDelta, RequirementVersion, Scenario. Thêm bốn node không chứng minh impact closure hoặc consistency. |

## Lỗi trọng yếu trong agent sinh spec

1. **Không thực hiện propose:** `specgen/spec_change.py` chỉ render template. `repo_context` lấy hash/tên file, không đọc nội dung baseline specs, config, code hay hướng dẫn artifact để tổng hợp. Không có bước agent xây dựng/critique thiết kế như skill propose.
2. **Context pack không đủ:** gom requirement/scenario/capability thành design; duyệt cạnh một lượt phụ thuộc thứ tự; work/test lấy toàn KG thay vì scope. Capability chung có thể kéo impact ra hàng trăm node không chứng minh liên quan.
3. **Mất ngữ nghĩa:** design boilerplate; task là “Implement and validate ID” thay vì deliverable/dependency/DoD; scenario bỏ given, có fallback chung chung; thiếu contract, data constraint, error path, security, migration và alternatives. Chọn một capability cho cả change khiến nhiều capability không được tách đúng.
4. **Readiness tự chứng nhận:** TS-01…TS-10 được gán `implemented` trong code, không chạy bằng chứng kiểm tra từng rule. Validator chủ yếu kiểm file không rỗng/có scenario, không kiểm mỗi requirement có scenario và trace.
5. **Publish sai thứ tự:** đổi tên staging thành change trước khi chạy CLI; CLI fail vẫn có thể trả published. `allow_draft` không được áp dụng trong flow này; snapshot KG chưa được kiểm tính bất biến bằng hash thực tế.
6. **Nguồn đầu vào yếu:** pilot lấy lần xuất hiện ID đầu tiên, kể cả phần giới thiệu/tham chiếu; scenario nối requirement theo modulo, API dùng placeholder, WorkItem/TestCase giả được ghi extracted và tất cả node được gắn approved. Đây là lỗi chất lượng dữ liệu trước khi tới specgen.

## Thứ tự khắc phục và điều kiện kiểm thử lại

1. Thu hồi kết luận implementation-ready của pilot; phân biệt unit fixture, integration pipeline và agent evaluation. Rà lại các task đã đánh dấu hoàn tất bằng acceptance thực tế.
2. Sửa extraction/intake: giữ typed rule/quality/design; evidence đúng nguồn/đoạn; không tự approve; resolve ID về định nghĩa và relation theo nội dung. Render TDD đầy đủ và round-trip props/edges/evidence, không chỉ ID.
3. Sửa prompt/CR: target resolution và preview diff có lý do; phát hiện CR trùng baseline; cập nhật impacted contracts/design/work/tests; chạy consistency/readiness lại sau commit.
4. Xây context pack theo scoped requirements và thiết kế liên quan, đọc baseline specs/config/code; thiếu thông tin phải tạo blocker hoặc assumption minh bạch. Không suy diễn thiết kế từ số lượng node.
5. Dùng workflow propose thật để author artifacts theo instructions/templates/schema của repo; KG là nguồn evidence. Mỗi assertion có trace tới node/version/source; mỗi task có deliverable, dependency và acceptance; design có lựa chọn/rationale/risks/migration phù hợp change.
6. Chạy semantic review và OpenSpec validation trong staging trước publish; chặn nếu fail; kiểm repo/KG snapshot. Test nhiều capability, missing contracts, duplicate CR, dangling traces và cảnh báo scope rộng.
7. Benchmark trên cùng change intent và cùng baseline bằng propose trực tiếp và KG→propose. So sánh correctness, scenario coverage (happy/error/boundary), implementability, trace fidelity, scope, design decisions và task completeness; human review hoặc rubric độc lập, không chỉ file/CLI pass.

Không chỉnh runtime trong lần rà soát này; báo cáo và CSV là kết quả kiểm tra. Cần sửa cả upstream TDD/KG lẫn spec agent trước khi chạy lại pilot để có kết luận readiness đáng tin.

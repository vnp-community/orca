# PBOM: chốt phiên làm việc ontology v0.4 và Change Request CR-0001

Ngày chốt: 2026-10-06. Phạm vi: ontology số hoá TDD, đồ thị TDD, động cơ thay đổi, thiết kế luồng agent sinh TDD.

## 1. Đã làm xong

| Hạng mục | Kết quả | Tệp |
|---|---|---|
| Đồ thị TDD (13 TDD) | 6.520 phần tử, 10.666 cạnh; cộng 6.341 khối nguồn nguyên văn thành 12.871 phần tử, 15.482 cạnh | `kg4.json`, `blocks4.jsonl` |
| Ontology v0.4 | 88 lớp, 88 quan hệ, 49 từ vựng, 17 đơn vị, 74 rule, 5 hồ sơ loại tài liệu; sinh OWL, SHACL, 88 JSON Schema | `Ontology_v0.4_Specification.md`, `o4/*.yaml`, `*.ttl`, `schemas/` |
| Mô hình thay đổi | ChangeRequest, ChangeSet, ChangeOp; động cơ áp dụng nguyên khối, tính tác động, so rule trước và sau; 15/15 ca kiểm thử đạt | `tools/changeset.py`, `tests/test_changeset.py` |
| Dựng lại văn bản | Không mất mát: 11/11 tệp trùng băm. Chỉ từ cấu trúc: phủ 47,8% từ | `tools/blockstore.py`, `tools/render.py`, `render_report.json` |
| Rule liên tài liệu | Bắt được lệch thật 50 MiB và 50 MB (limits.sbom_max_bytes) | `findings_v4.csv` |
| Đề xuất luồng agent | Đã có bản đề xuất harness v0.1; thiết kế sửa: bước tiếp nhận tri thức theo nhu cầu (mục 3) | `TDD_Agent_Harness_Proposal_v0.1.md` |

Kết quả rule trên đồ thị v0.4: 161.883 đạt, 6.422 không đạt, 4.657 thiếu dữ liệu, 39.910 không áp dụng.

## 2. Mức đáp ứng ba mục tiêu (kết luận đã kiểm trên dữ liệu)

1. Số hoá và viết lại thành văn bản: một phần. Dựng lại không mất mát có; dựng lại từ cấu trúc mới phủ bảng (66%), mã nhúng 7%, đoạn văn 37%, tiêu đề 2%.
2. Thêm, sửa, xoá mà giữ toàn vẹn: có cơ chế, bảo vệ chưa đủ. Cổng chỉ bảo đảm "không tệ đi"; nền có 6.422 vi phạm; các rule L-01 đến L-09, X-03, X-05 chưa chạy được.
3. Đầu vào cho agent: có hợp đồng và mô hình thay đổi; chưa dựng được agent vì chưa có bước tiếp nhận tri thức và bộ vàng.

## 3. Quyết định thiết kế mới nhất (chưa cài)

Nguồn đầu vào của agent TDD phải động, không bắt buộc số hoá URD, SRS, SAD. Thêm bước tiếp nhận tri thức (cổng 0):

1. Danh sách nhu cầu tri thức (`KnowledgeNeed`): mỗi mục TDD cần biết gì từ thượng nguồn, suy ra từ hồ sơ TDD.
2. Lấy tri thức: ưu tiên KG nếu đã có; nếu chưa thì đọc lát cắt cần thiết từ văn bản gốc (Word hoặc Markdown). Mọi mẩu tri thức kèm vị trí, trích dẫn nguyên văn, độ tin cậy, trạng thái duyệt.
3. Kiểm tra đủ, thiếu, mâu thuẫn: đủ thì chuyển agent; thiếu hoặc không chắc thì lập `IntakeReport`, dừng và hỏi người dùng (trả lời, bổ sung tài liệu, hoặc cho dùng giả định ghi thành Gap `assumed` có chủ, có hạn); mâu thuẫn thì nêu cả hai nguồn kèm trích dẫn, người dùng chọn, ghi thành Decision.
4. Rule X-03, X-05 chạy có điều kiện: có giá trị thượng nguồn thì so, không có thì `insufficient_data` kèm cảnh báo, không chặn luồng.

Rủi ro đã biết: đọc trực tiếp từ Word kém chính xác hơn KG đã duyệt (cạnh tin cậy dưới 0,8 chỉ đúng khoảng 38% ở lần đo trước), nên mẩu dưới ngưỡng phải hỏi người dùng, không dùng im lặng.

## 4. Change Request CR-0001

| Trường | Giá trị |
|---|---|
| Mã | CR-0001 |
| Nguồn thay đổi | review_comment và clarification (kết quả kiểm tra v0.4 và quyết định thiết kế mục 3) |
| Người đề nghị | (điền) |
| Lý do | Hoàn thiện ba mục tiêu: số hoá có thể dựng lại, thay đổi giữ toàn vẹn, agent nhận nguồn động có cảnh báo |
| Tài liệu đích | Ontology (v0.5), TDD-01, TDD-04, TDD-07, bộ công cụ |
| Trạng thái | submitted |
| Người duyệt, ngày duyệt | (điền) |

### 4.1 Thay đổi ontology (v0.4 lên v0.5)

| # | Thao tác | Nội dung | Rule liên quan |
|---|---|---|---|
| O-1 | add | Lớp `KnowledgeNeed` (section_spec, slot, class, filter, min_confidence, required) và `IntakeReport` (need, status found/missing/conflict/low_confidence, sources, question, resolution) | mới: N-01..N-04 |
| O-2 | add | Rule N-01 mọi SectionSpec bắt buộc có KnowledgeNeed; N-02 mẩu tri thức từ văn bản phải có trích dẫn và độ tin cậy; N-03 mẩu dưới ngưỡng không được dùng khi chưa duyệt; N-04 khuyết phải thành câu hỏi hoặc Gap `assumed` | N-01..N-04 |
| O-3 | modify | X-03, X-05: thêm điều kiện áp dụng "có giá trị thượng nguồn trong gói tri thức"; không có thì `insufficient_data` kèm cảnh báo | X-03, X-05 |
| O-4 | add | `Section` và khe văn xuôi (`prose_only`) để khối đoạn văn và tiêu đề có chỗ đứng | P-03 |
| O-5 | add | Mô hình hoá mã nhúng: DDL thành Entity, Attribute, Index; cấu hình thành ConfigItem; chữ ký giao diện thành Operation, Message | P-04 |
| O-6 | add | Cài `supersede` và `merge` trong động cơ thay đổi | C-03 |
| O-7 | add | Bổ sung đơn vị ghép vào sổ đơn vị: yêu cầu/phút/người dùng, lần/5 giây, yêu cầu/giây/danh tính, lỗi serious hoặc critical, tỷ lệ | V-02 |
| O-8 | modify | Đổi các Control mô tả cảnh báo (nối Metric) thành AlertRule | S-02 |

### 4.2 Thay đổi nội dung TDD (đã kiểm chứng từ rule, đối chiếu văn bản gốc)

| # | Tài liệu | Thao tác | Nội dung | Căn cứ |
|---|---|---|---|---|
| T-1 | TDD-01 và TDD-04 | modify | Thống nhất ký hiệu `limits.sbom_max_bytes` là 50 MiB (52.428.800 byte); sửa "50 MB" ở TDD-04 (dòng cấu hình `max_body_bytes` và hằng số `SbomMaxBytes`) | X-01; đã đối chiếu văn bản gốc |
| T-2 | TDD-01 và TDD-04 | modify | Thống nhất kiểu của `limits.sbom_max_bytes` (hiện `int` và `int64`); đề xuất `int64` | X-01 |
| T-3 | TDD-04 và TDD-07 | kiểm tra | `projects.gate_failure_mode`: ghi `varchar(6)` ở TDD-04; công cụ báo `text` ở TDD-07 nhưng văn bản gốc chỉ thấy `varchar(6)`. Xác nhận là lỗi trích, nếu đúng thì không sửa TDD | X-02; chưa kết luận |
| T-4 | SAD, TDD liên quan | modify | Giải chu trình phụ thuộc giữa M-03 và M-04 | S-04 |
| T-5 | TDD-01 đến TDD-13 | add | Giao chủ và hạn cho 142 vấn đề mở thiếu hạn và 52 khuyết `tbd`; nối 51 giá trị giả định hoặc đề xuất với Assumption, OpenIssue hoặc Decision | G-02, G-03, G-04 |
| T-6 | TDD-01 đến TDD-13 | add | Trạng thái đầu và cuối cho 41 mô hình trạng thái; điều kiện trước và sau cho 24 thuật toán; chuyển 13 quy tắc tài sản thành bảng quyết định có chính sách khớp | L-01..L-09 |
| T-7 | TDD-01 đến TDD-13 | modify | Gộp 50 nhóm phần tử trùng tên trong cùng lớp và các thành phần xuất hiện dưới hai mã (ví dụ SVC-01 và Gateway Envoy) | X-04, S-05 |

### 4.3 Việc về công cụ và dữ liệu

| # | Việc | Rule |
|---|---|---|
| C-1 | Cài bước tiếp nhận tri thức (cổng 0): bộ đọc lát cắt từ Word, Markdown, KG; bản mẫu cho TDD-04 từ tệp SRS và SAD hiện có; đo hai thứ: tìm đủ nhu cầu không, và khi cố ý xoá bớt dữ liệu thì cảnh báo có hiện đúng không | N-01..N-04 |
| C-2 | Lập bộ vàng 20 đến 30 cặp (yêu cầu thay đổi, bộ thay đổi đúng), gồm ca thượng nguồn thiếu và mâu thuẫn; người viết khác người duyệt | |
| C-3 | Chạy lại bước trích theo lược đồ v0.4 để điền thuộc tính bắt buộc, Quantity, Gap | M-01 |
| C-4 | Lượt LLM nối 284/353 đầu việc và 129/418 ca kiểm thử với phần tử thiết kế | T-04, T-05 |
| C-5 | Cài rule L-01..L-07 khi có dữ liệu có cấu trúc (T-6); cài X-03, X-05 theo O-3 | L-01..L-07 |
| C-6 | Chuyển động cơ thay đổi sang kho giao dịch khi cần nhiều người dùng đồng thời | C-04 |

### 4.4 Thứ tự đề xuất

1. T-1, T-2, T-3 (nhỏ, đã có căn cứ).
2. O-1 đến O-3 cùng C-1 (bản mẫu tiếp nhận cho TDD-04).
3. C-2 (bộ vàng, cần người tham gia).
4. O-4, O-5 và C-3 (nâng độ phủ số hoá).
5. T-5, T-6, T-7 và C-5 (nâng sức bảo vệ của cổng).
6. Dựng agent và nghiệm thu bằng bộ vàng.

### 4.5 Tiêu chí hoàn thành

- Bản mẫu tiếp nhận: với TDD-04, tìm đủ nhu cầu tri thức từ SRS và SAD; khi xoá cố ý một nhu cầu thì `IntakeReport` báo thiếu và hỏi.
- Độ phủ dựng lại từ cấu trúc: khối mã từ 7% lên ít nhất 90%; toàn bộ từ 47,8% lên mục tiêu do người duyệt đặt (chưa chốt).
- Vi phạm nền giảm so với 6.422; không còn vi phạm mức `error` ngoài chu trình đã nêu.
- Agent đạt ngưỡng trên bộ vàng do người duyệt đặt (chưa chốt) và hỏi đúng lúc khi thượng nguồn thiếu.

## 5. Hạn chế cần giữ khi tiếp tục

- Một lượt trích LLM; chưa có lượt kiểm thứ hai độc lập. Ngưỡng và danh sách từ vựng là đề xuất, chưa được chuyên gia nghiệp vụ duyệt.
- Độ phủ dựng lại là cận trên theo từ, không phải so sánh câu chữ. Mẫu TDD hiệu chỉnh theo chính 13 TDD hiện có.
- 15 ca kiểm thử động cơ do tác giả viết, chưa phải bộ vàng độc lập.
- Ánh xạ sang tiêu chuẩn ISO chưa được kiểm chứng.
- Hồ sơ URD, SRS, SAD mới ở mức khung; theo quyết định mục 3 không bắt buộc số hoá.

## 6. Tệp cần giữ để mở lại phiên

- `ontology_v0.4.zip`: toàn bộ ontology, đồ thị, công cụ, kiểm thử, kết quả (đã gửi trước).
- `Ontology_v0.4_Specification.md`: đặc tả đầy đủ (mục 1.1 là bảng mức đáp ứng).
- `TDD_Agent_Harness_Proposal_v0.1.md`: đề xuất luồng agent (cần sửa theo mục 3 của tệp này).
- Tệp này: CR-0001.

Lưu ý: môi trường làm việc là tạm thời; chỉ các tệp đã gửi mới được giữ.


## 7. Bổ sung sau rà soát: đủ tri thức để sinh tech spec

Ngày: 2026-10-06. Bổ sung phạm vi CR-0001; trạng thái vẫn là submitted, chưa tuyên bố đã duyệt hoặc đã cài.

- Thêm yêu cầu REQ-KG-TS-01: KG TDD cung cấp đủ tri thức để sinh tech spec/OpenSpec change có thể triển khai. Xem [hợp đồng v0.5](TDD_KG_TechSpec_Readiness_v0.5.md) cho baseline/delta, capability, scenario, design, task/validation, provenance và readiness TS-01..TS-10.
- Thêm việc ontology: Capability, RequirementVersion/RequirementDelta, GenerationProfile/Run/ReadinessReport, mở rộng Scenario và WorkItem; khai báo schema/relations/rules và migration trước khi dùng trong KG thực thi.
- Thêm việc harness: intake theo KnowledgeNeed cho cả đầu vào TDD và nhu cầu downstream; context closure; readiness; adapter openspec-propose và trace manifest; nghiệm thu các ca thiếu/mâu thuẫn/stale/rename/remove.
- [Harness v0.2](TDD_Agent_Harness_Proposal_v0.2.md) thay thế luồng vận hành v0.1. Bản v0.1 và dữ liệu o4 giữ làm lịch sử.
- Làm rõ số liệu: 6.422 fail bao gồm 2.093 mức error theo validate4.json. Giảm tổng fail và không tạo lỗi mới chưa đủ nghiệm thu; scope implementation_ready phải không còn blocker bắt buộc hoặc luật bắt buộc chưa cài.
- Tiêu chí mới: sinh đủ proposal/design/tasks/spec deltas theo repo profile, 100% nhu cầu bắt buộc và truy vết trong scope, không sinh fact vô nguồn; ca thiếu dữ liệu phải trả draft/blocked đúng nhu cầu. Các thay đổi này là yêu cầu thiết kế, chưa phải kết quả benchmark.


## 8. Đề xuất OpenSpec và luồng sửa TDD bằng prompt

Đã lập [bốn OpenSpec changes](../openspec/README.md): shared ontology contract, dynamic TDD generation (initial/CR/enhancement/defect), TDD KG spec-change generation và prompt TDD editing. Mục tiêu cuối gồm ba luồng agent dùng chung ontology/ChangeSet/impact/readiness. Bộ change đạt format validation nhưng chưa triển khai; CR-0001 không tự chuyển approved/applied.

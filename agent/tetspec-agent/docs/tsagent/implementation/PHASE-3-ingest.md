# Giai đoạn 3: Nạp đầu vào và delta

Mục tiêu: đưa gói `tdd-kg/v1` và md (URD, SRS, TDD mẫu v1, solution, CR, supplement) vào một baseline nguồn, không mất dữ liệu, lỗi có vị trí; tính delta giữa hai baseline.

Tham chiếu: ONTOLOGY-TDD mục 2–8; TDD v2 FR-001, 002, 003, 017; NFR-012; TDD v1 mục 1.5 (mẫu md v1, chỉ để hiểu định dạng).

---

## T3.1 JSON Schema và bộ nạp `tdd-kg/v1`

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T2.1
- **File:** `tsagent/schemas/tdd-kg-v1.schema.json`, `tsagent/ingest/kg_json.py`, `tsagent/validate/input_rules.py`

Checklist:
- [x] JSON Schema theo ONTOLOGY-TDD mục 8
- [x] `load_kg_json(obj_or_path, feature)` → `SourcePackage` + chẩn đoán
- [x] I1 cạnh treo; I2 domain/range theo ontology gửi kèm; I3 thuộc tính bắt buộc; I4 mã trùng; I5 supersede khác lớp; I6 đúng một Document chính; I7 chuỗi nối thay mảng; I8 phiên bản không hỗ trợ; chẩn đoán có JSON pointer
- [x] Test `tests/unit/test_kg_json.py`: fixture minipay `slice`, `full`; mỗi luật có ca đạt và không đạt (TST-01, 02, 03)

Tiêu chí xong: test xanh.

---

## T3.2 Bộ phân tích md mới

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T1.2
- **File:** `tsagent/ingest/md/` (`tables.py`, `headings.py`, `frontmatter.py`, `prefixes.py`, `appendix.py`)

Checklist:
- [x] Front matter: `doc_id`, `doc_type`, `version`, `status` → nút Document (status `active/approved/released/baselined` → Baselined)
- [x] Bảng có cột đầu là mã có tiền tố đã biết (ONTOLOGY-TDD mục 2.2 và 4) → nút; tên cột là thuộc tính hoặc quan hệ; cột `Thuộc tính` dạng `k=v; k=v`
- [x] Cột quan hệ ngược (`Gắn vào`, `Chặn`, `Đáp ứng`, `Module tích hợp`, `Yêu cầu`, `Chi tiết hóa`)
- [x] Heading thực thể (`### 8.1 SCN-01: tiêu đề`) và dòng thuộc tính ngay sau; bảng `#`/`Bước` thành `steps` (mảng)
- [x] Ngữ cảnh Integration: thực thể dưới heading `INT-xx` tự nối vào Integration đó
- [x] Phụ lục có cấu trúc của solution: EP, FLD, RSP, AC, SM, TRN; supplement: AC-REV, FLD-REV, RSP-REV, TRN-REV, bắt buộc cột `Căn cứ`, tối đa 80 hàng
- [x] Ép kiểu số, boolean, danh sách có kiểm (không đoán bừa)
- [x] Lỗi có tệp, dòng, luật (NFR-012); tham chiếu mã không tồn tại, mã trùng hai lớp, tài liệu không có thực thể đều là lỗi
- [x] Test `tests/unit/test_md_parser.py` trên bộ md PAY và solution, supplement CR-001: số nút, số cạnh, kiểu dữ liệu, từng loại lỗi

Tiêu chí xong: bộ md PAY nạp ra đủ nút, cạnh như mô tả trong BASELINE-V1.md; test xanh.

---

## T3.3 Gộp nguồn và tạo baseline

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T3.1, T3.2
- **File:** `tsagent/ingest/merge.py`, `tsagent/ingest/__init__.py`

Checklist:
- [x] `ingest(documents, feature)` nhận hỗn hợp `.json` và `.md`; `.ttl` chỉ khi có plugin
- [x] Gộp nút trùng mã cùng lớp; khác lớp là lỗi có vị trí
- [x] Ontology của baseline = ontology gửi kèm gói JSON + lớp mở rộng (ONTOLOGY-TDD mục 4) khi có md
- [x] `baseline_id` = `b-` + 12 ký tự băm nội dung chuẩn hóa
- [x] Giới hạn đường dẫn đọc theo `TSAGENT_SOURCE_ROOTS`; tài liệu gửi thẳng ghi vào hộp thư
- [x] Ghi qua `save_baseline`
- [x] Test `tests/integration/test_ingest.py`: gói CR-001 (JSON + solution + supplement) thành một baseline; md-only (PAY) vẫn chạy; đường dẫn ngoài `SOURCE_ROOTS` bị từ chối

Tiêu chí xong: test xanh.

---

## T3.4 Delta, tập ảnh hưởng, CR ngầm định

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T3.3
- **File:** `tsagent/delta/`

Checklist:
- [x] `diff(prev, new)`: thêm, đổi (theo `content_hash`), bỏ; dùng `src_supersede` để ghép nút đổi mã
- [x] `impact(changed, depth=3)` bằng CTE đệ quy trên `src_edge`, bỏ `describedIn`, `supersedes`; trả các nút thuộc lớp kiểm thử được
- [x] CR ngầm định khi nguồn đổi mà không có CR khai báo; ghi kèm baseline (hoặc `src_derived_node`)
- [x] Test `tests/unit/test_delta.py`: các loại thay đổi; supersede đổi mã; tập ảnh hưởng; kịch bản `ingest` trước rồi `gen` không mất CR (TST-04)

Tiêu chí xong: test xanh.

---

## T3.5 Plugin Turtle (tùy chọn, FR-017)

- **Trạng thái:** Bỏ (2026-10-13): làm xong ngày 2026-10-12, gỡ khi chuyển sang repo tdd-agentic-ai vì không dùng RDF (DEVIATIONS D21)
- **Phụ thuộc:** T3.1
- **File:** `tsagent/ingest/ttl_plugin.py`

Checklist:
- [x] Chuyển Turtle (ontology `tdd:` Core) sang `SourcePackage`; thuộc tính nhiều giá trị thành mảng; cạnh tới nút không khai kiểu là chẩn đoán
- [x] Chỉ import `pyoxigraph` khi dùng; thiếu thì hướng dẫn cài `tsagent[rdf]`
- [x] Test (bỏ qua nếu không cài extras): `kg-slice.ttl` cũ của minipay cho cùng kết quả với gói JSON (TST-19)

Tiêu chí xong: test xanh khi cài extras.

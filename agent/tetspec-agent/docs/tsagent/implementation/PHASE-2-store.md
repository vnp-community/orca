# Giai đoạn 2: Kho SQLite

Mục tiêu: lưu nguồn bất biến và KG test có phiên bản; vùng đệm theo run; vòng đời, duyệt, baseline trong một giao dịch; toàn vẹn và sao lưu.

Tham chiếu: TDD v2 mục 5 (V1–V11), MECH-02, 03, 04, 08; ONTOLOGY-TEST-TSO mục 3, 10; hành vi cần giữ trong BASELINE-V1.md.

---

## T2.1 API nguồn (`src_*`)

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T1.5, T1.2
- **File:** `tsagent/store/source.py`

Checklist:
- [x] `save_baseline(pkg)`: một giao dịch ghi `src_baseline`, `src_onto_*`, `src_node`, `src_edge`, `src_supersede`; `supersedes_node_id` với baseline trước khi `content_hash` đổi
- [x] Baseline cùng băm đã có thì trả `reused`; phần suy diễn (CR ngầm định) ghi vào `src_derived_node`, không bỏ qua
- [x] `load_source(feature, baseline=None)` trả `SourcePackage`, giữ kiểu JSON của `props` và thuộc tính cạnh
- [x] `list_baselines`, `get_node`, `neighbors(code, depth, direction)` bằng CTE đệ quy
- [x] Test `tests/unit/test_store_source.py`: vòng lưu–đọc fixture minipay khớp từng nút, cạnh; trigger chặn sửa; CR ngầm định khi baseline đã có

Tiêu chí xong: test xanh.

---

## T2.2 API KG test, vùng đệm, phiên bản

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T1.5, T1.2
- **File:** `tsagent/store/tso.py`, `tsagent/store/versions.py`

Checklist:
- [x] `stage_put(run_id, step, entities)` và `stage_load(run_id, step)` trong `stage_item`
- [x] `commit_stage(run_id, feature, base_baseline)`: một giao dịch chuyển stage thành dòng `tso_*`; so `content_hash` với SpecVersion nền (dùng lại, hoặc `node_version+1` + `supersedes_id`); tạo hoặc ghi lại SpecVersion Draft; ghi `tso_based_on`, `tso_evidence`, `tso_lifecycle(Draft, proposed)`; xóa stage
- [x] `load_spec(feature, version=None)` dựng lại ba tầng
- [x] `diff_versions`, `history`, `introduced_codes`, đọc và ghi `tso_regression`
- [x] Test `tests/integration/test_store_versions.py`: phiên bản 1; CR sinh phiên bản 2 tại chỗ; nút không đổi dùng lại; Draft ghi lại tại chỗ không nhân bản; lỗi giữa giao dịch không ghi dở

Tiêu chí xong: test xanh; các kịch bản phiên bản trong BASELINE-V1.md có test tương ứng.

---

## T2.3 Vòng đời, duyệt, baseline

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T2.2
- **File:** `tsagent/store/lifecycle.py`

Checklist:
- [x] `set_review(feature, filter, status, by, note)`: lọc theo mã, lớp, tầng, `derivation`, `origin`, `confidence`; chỉ trên SpecVersion Draft
- [x] `baseline(feature, by)`: một giao dịch Draft → Baselined, phiên bản trước → Superseded, nút bỏ → Retired + `retired_by`; từ chối khi không xác định được CR; lưu `graph_hash`
- [x] Bất biến: sửa dòng Baselined qua API hay SQL trực tiếp đều bị chặn
- [x] Test `tests/integration/test_lifecycle.py`: duyệt theo bộ lọc; baseline; Retired có CR; từ chối khi thiếu CR; UPDATE trực tiếp bị trigger chặn (TST-13)

Tiêu chí xong: test xanh.

---

## T2.4 Toàn vẹn, snapshot, xuất

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T2.2
- **File:** `tsagent/store/ops.py`

Checklist:
- [x] `check(feature=None)`: khóa ngoại, luật hình dạng `+`, chuỗi `supersedes_id` thẳng, Retired có `retired_by`, băm SpecVersion Baselined, băm baseline nguồn; mỗi lỗi gắn đúng feature
- [x] `snapshot(path)`: backup API + `.sha256`
- [x] `restore(path, into)`: kiểm băm, chỉ vào file trống, chạy `check`
- [x] `export_json(feature, version)` và đọc lại không mất dữ liệu (FR-016)
- [x] `stats()`
- [x] Test `tests/integration/test_store_ops.py`: snapshot–restore; băm sai và DB không trống bị từ chối; từng loại lỗi toàn vẹn; lỗi feature A không chặn feature B; TST-16, TST-18

Tiêu chí xong: test xanh.

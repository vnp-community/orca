# Giai đoạn 1: Nền tảng

Mục tiêu: các thành phần dùng chung mà mọi module khác dựa vào: cấu hình, lỗi, log, kiểu miền, định nghĩa ontology, sinh mã, kết nối DB.

Tham chiếu: TDD v2 mục 5, 9 (MECH-02, 13, 15), 10; ADR-V2-01, 06; ONTOLOGY-TEST-TSO mục 3–6, 10.

---

## T1.1 Cấu hình, lỗi, log

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T0.4
- **File:** `tsagent/config.py`, `tsagent/errors.py`, `tsagent/logging.py`

Checklist:
- [x] `Config` đọc biến môi trường của TDD v2 mục 10 (`TSAGENT_DB`, `TSAGENT_SPECS_ROOT`, `TSAGENT_CACHE_DIR`, `TSAGENT_SNAPSHOT_DIR`, `TSAGENT_INBOX_DIR`, `TSAGENT_LLM`, `TSAGENT_LLM_STEPS`, `TSAGENT_LLM_CAP_*`, `TSAGENT_AUTO_APPROVE`, `TSAGENT_TOKEN`, `TSAGENT_MCP_REVIEW`, `TSAGENT_SOURCE_ROOTS`, `TSAGENT_MAX_BODY_KB`, `ANTHROPIC_*`, `OPENAI_*`, `TSAGENT_CLAUDE_MODEL`, `CLAUDE_BIN`); giá trị sai báo lỗi rõ
- [x] Hàm kiểm mã feature (`A-Z a-z 0-9 . _ -`, tối đa 64, không `..`)
- [x] Cây lỗi: `TsagentError` → `InputError` (kèm danh sách chẩn đoán tệp/vị trí/luật), `ValidationFailed` (kèm báo cáo), `ImmutableError`, `NotFound`, `BusyError`, `HandEditedError`, `LLMError` (transient, permanent, unknownOutcome, schema, budget, circuitOpen)
- [x] Log JSON có `runId`, `step`, `code`; che bí mật theo mẫu
- [x] Test `tests/unit/test_config.py`, `test_errors.py`, `test_logging.py` (bí mật bị che)

Tiêu chí xong: test xanh.

---

## T1.2 Kiểu miền trong bộ nhớ

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T1.1
- **File:** `tsagent/model/`

Checklist:
- [x] `SrcNode(code, cls, base_attrs, props, test_scope)`, `SrcEdge(src, rel, dst, props)`, `OntologyDecl(classes, relations)`, `SourcePackage(feature, nodes, edges, ontology, supersede, meta)`
- [x] Truy vấn trên `SourcePackage`: `of(cls)`, `targets(code, rel, cls?)`, `sources(code, rel)`, `participants(scenario)`; chỉ mục theo quan hệ để không duyệt tuyến tính
- [x] `content_hash()` chuẩn hóa (sắp khóa, bỏ trường biến động)
- [x] `TsoEntity` (lớp, mã, thuộc tính, quan hệ, provenance) và `Provenance(derivation, based_on, evidence, model, prompt_hash, confidence, run_id)`
- [x] Test `tests/unit/test_model.py`: băm ổn định, truy vấn

Tiêu chí xong: test xanh.

---

## T1.3 Định nghĩa ontology `tso.yaml`

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T1.1
- **File:** `tsagent/ontology/tso.yaml`, `tsagent/ontology/__init__.py`

Checklist:
- [x] Cấu trúc: `version`, `enums`, `common` (cột chung), `classes` (tầng, trường, kiểu, `!`/`+`/`*`/`?`), `relations` (từ, đến, bội số, khóa ngoại hay bảng nối), `source` (bảng `src_*`), `ops` (run, run_step, stage_item, rendered_file, schema_version)
- [x] Đủ lớp tầng A, B, C và lõi theo ONTOLOGY-TEST-TSO mục 3–6
- [x] Đủ enum: `derivation`, `reviewStatus`, `status`, `conditionKind` (21), `oracleKind`, `faultTiming`, `classKind`, `pathKind`, `priority`, `level`, `testType`, `verificationMethod`, `technique`, `stepKind`, `stubBehavior`, `docKind`, `origin`
- [x] `load_ontology()` kiểm tên duy nhất, quan hệ trỏ lớp có thật, enum tồn tại; lỗi có vị trí YAML
- [x] Test `tests/unit/test_ontology_yaml.py`: hợp lệ; quan hệ treo; enum thiếu

Tiêu chí xong: mọi lớp, trường, quan hệ của ONTOLOGY-TEST-TSO có trong YAML; test xanh.

---

## T1.4 Sinh mã (codegen)

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T1.3
- **File:** `tsagent/ontology/codegen.py`; đầu ra `tsagent/store/schema.sql`, `tsagent/gateway/views.sql`, `tsagent/schemas/v2/*.schema.json`, `tsagent/validate/shape_rules.json`

Checklist:
- [x] DDL bảng `tso_*`: cột chung, `NOT NULL`, `CHECK IN`, khóa ngoại, bảng nối, `UNIQUE(feature, code, node_version)`, chỉ mục khóa ngoại
- [x] DDL bảng `src_*` (TDD v2 mục 5.3), bảng vận hành, `tso_based_on`, `tso_evidence`, `tso_lifecycle`, `tso_spec_version`, `tso_spec_version_item`, `tso_regression`
- [x] Trigger: chặn `UPDATE`, `DELETE` mọi `src_*`; chặn sửa dòng `tso_*` Baselined (trừ chuyển Superseded, Retired); một SpecVersion Draft mỗi feature
- [x] View chỉ đọc `v_source_node`, `v_obligation`, `v_condition`, `v_requirement`, `v_coverage`, `v_testcase`, `v_trace`, `v_gap`
- [x] JSON Schema đầu ra LLM: `analysis`, `design`, `specification` (giới hạn độ dài, enum, trường bắt buộc)
- [x] Danh mục luật hình dạng (trường `+`)
- [x] Tệp sinh có header "không sửa tay" và băm `tso.yaml`; lệnh `make codegen`
- [x] Test golden `tests/unit/test_codegen.py` (lệch thì đỏ, nhắc chạy codegen); DDL chạy trên SQLite trống, `foreign_key_check` sạch

Tiêu chí xong: đổi `tso.yaml` rồi `make codegen` thì mọi tệp sinh đổi theo; test golden bắt việc quên chạy.

---

## T1.5 Kết nối DB, khởi tạo, migrate, giao dịch theo bước

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T1.4
- **File:** `tsagent/store/db.py`, `tsagent/store/migrations/`

Checklist:
- [x] `connect(path, readonly=False)`: WAL, `foreign_keys=ON`, `busy_timeout`, `synchronous=NORMAL`; đọc bằng URI `mode=ro`
- [x] `init_schema()` khi DB trống; ghi `schema_version` (phiên bản, băm `tso.yaml`)
- [x] `migrate()` chạy script theo thứ tự, kiểm `foreign_key_check`, rollback khi lỗi
- [x] `step_tx(conn, run_id, step)`: `BEGIN IMMEDIATE` … `COMMIT`, rollback khi lỗi, cập nhật `run_step`
- [x] Ghi đồng thời quá `busy_timeout` thì `BusyError` có hướng dẫn
- [x] Test `tests/unit/test_db.py`: khởi tạo, migrate giả lập, rollback, đọc khi đang ghi

Tiêu chí xong: test xanh; `sqlite3 tsagent.db .tables` thấy đủ bảng và view.

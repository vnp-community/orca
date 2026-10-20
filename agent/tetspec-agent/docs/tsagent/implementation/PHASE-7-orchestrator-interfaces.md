# Giai đoạn 7: Orchestrator, duyệt, MCP, CLI, đóng gói, tích hợp workflow

Mục tiêu: pipeline chạy end-to-end qua CLI và MCP, chạy tiếp được sau lỗi; đóng gói Docker; workflow minipay chạy lại bước testspec.

Tham chiếu: TDD v2 mục 6.2, 8 (SCN-01..07), 9, 10; FR-010..015; NFR-008, 010, 011; hành vi MCP cần giữ trong BASELINE-V1.md.

---

## T7.1 Orchestrator theo bước

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T6.4, T3.4
- **File:** `tsagent/orchestrator/run.py`, `tsagent/orchestrator/steps.py`

Checklist:
- [x] `runKey` = băm(feature, băm gói, `ontologyVersion`, phiên bản `tso.yaml`, chế độ LLM, mô hình, các trần)
- [x] Máy trạng thái Created → Ingested → Analyzed → Designed → Specified → Validated → Committed → Rendered, Failed (bảng `run`, `run_step`)
- [x] LLM gọi ngoài giao dịch; kết quả ghi stage trong giao dịch ngắn
- [x] Chạy tiếp từ bước sau bước commit cuối; `from_step` chạy lại một bước
- [x] `dry_run`: chạy hết, không commit
- [x] Luồng CR: chỉ phân tích lại chủ thể trong tập ảnh hưởng; tầng B, C chỉ cho điều kiện mới hoặc đổi; dùng lại nút không đổi; chọn hồi quy
- [x] Nhiều feature: cô lập lỗi
- [x] Test `tests/integration/test_orchestrator.py`: PAY và CR-001 với `FakeLLM`; dừng giữa mỗi bước rồi chạy tiếp, không ghi dở, không gọi lại LLM (TST-15); CR end-to-end (TST-12)

Tiêu chí xong: `tsagent gen --feature SCHEDULED-TRANSFER --src cr001-slice.tdd-kg.json solution.md` ra ba tài liệu.

---

## T7.2 Duyệt và baseline

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T2.3, T6.3
- **File:** `tsagent/orchestrator/review.py`, `tsagent/validate/tso_rules.py`

Checklist:
- [x] Danh sách chờ duyệt lọc theo lớp, `derivation`, `origin`, `confidence`, tầng; ưu tiên `llmProposal`, rồi `llm`
- [x] approve, reject, needs-info theo mã hoặc bộ lọc
- [x] Policy tự duyệt `TSAGENT_AUTO_APPROVE` (loại nút, ngưỡng); `reviewer=auto:<policy>`
- [x] Baseline: gate GS, Document TDD chính Baselined, nút bỏ có CR, nút `llm` đã duyệt theo policy (TSO-020, 021)
- [x] Test `tests/integration/test_review.py` (TST-14)

Tiêu chí xong: baseline CR-001 sau khi duyệt; phiên bản 2 qua một CR mẫu.

---

## T7.3 MCP gateway

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T7.1
- **File:** `tsagent/gateway/`, `tsagent/schemas/mcp-tools.schema.json`

Checklist:
- [x] MCP JSON-RPC 2.0 qua HTTP (Streamable HTTP, chế độ JSON) và stdio
- [x] Xác thực Bearer; bind `127.0.0.1` mặc định, bind ngoài bắt buộc token; kiểm `Origin`; giới hạn kích thước yêu cầu; cắt kết quả 30.000 ký tự, đọc tiếp bằng `offset`
- [x] Tool giữ tên cũ: `list_specs`, `get_spec`, `get_testcase`, `get_traceability`, `get_coverage`, `get_validation`, `get_version_history`, `diff_versions`, `search`, `get_neighbors`, `start_run`, `get_run_status`, `list_pending_review`
- [x] Tool mới: `get_analysis`, `list_gaps`, `sql_select` (kết nối `mode=ro`, authorizer chỉ cho đọc view `v_*`, timeout bằng progress handler, giới hạn dòng)
- [x] `start_run.documents` nhận `tdd-kg/v1` và md; `paths` chỉ trong `TSAGENT_SOURCE_ROOTS`; `from_step`, `force`, `dry_run`
- [x] `approve_nodes`, `reject_nodes`, `baseline_spec` chỉ khi `TSAGENT_MCP_REVIEW=1`
- [x] Endpoint `/health`
- [x] Test `tests/contract/test_mcp.py`: lược đồ vào ra từng tool; xác thực, Origin; `sql_select` từ chối bảng thật, lệnh ghi, quá thời gian (TST-17)

Tiêu chí xong: test hợp đồng xanh.

---

## T7.4 CLI

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T7.1, T2.4
- **File:** `tsagent/cli.py`

Checklist:
- [x] `init`, `gen` (`--src`, `--from-step`, `--steps`, `--dry-run`, `--force`), `render`, `history`, `diff`, `validate`
- [x] `review list|approve|reject|needs-info|baseline` với bộ lọc
- [x] `kg migrate|check|stats|snapshot|restore|export`
- [x] `ontology codegen|check`; `run status|resume`; `serve` (HTTP hoặc stdio); `selfcheck` (chạy thử không ghi)
- [x] Lỗi rõ khi DB đang bị ghi (`BusyError`)
- [x] Test `tests/integration/test_cli.py`

Tiêu chí xong: ví dụ lệnh trong README chạy được.

---

## T7.5 Đóng gói và triển khai

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T7.3, T7.4
- **File:** `deploy/Dockerfile`, `deploy/docker-compose.yml`, `deploy/.env.example`, `Makefile`

Checklist:
- [x] Dockerfile nhiều tầng (dev, runtime); volume cho file DB, cache, snapshot, specs
- [x] Binary một tệp bằng PyInstaller (`make build`), dựng trên bookworm cho khớp glibc
- [x] `make start-dev` / `stop-dev` (`MODE=src|bin`), health check
- [x] `.env.example` đủ biến TDD v2 mục 10, mặc định an toàn (LLM `none`, duyệt qua MCP tắt)
- [x] Kiểm chạy được trong container với DB trên volume

Tiêu chí xong: `make start-dev` lên dịch vụ, `/health` trả `ok`.

---

## T7.6 Tích hợp workflow minipay

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T7.3, T7.5
- **File:** `tmp/minipay-demo/workflow/testspec.py`, `tmp/minipay-demo/tests/test_testspec.py`, README của demo

Checklist:
- [x] Thay `to_turtle()` bằng xuất `tdd-kg/v1` (dùng `onto_classes`, `onto_relations` của `tdd_kg`; giữ `testScope=context`, Document chính, CR `affects`; thêm `supersede`, thuộc tính cạnh)
- [x] Gửi `kg-slice.tdd-kg.json` + `solution.md` (+ `supplement.md`) qua `start_run.documents`; lưu tệp gửi vào `run/testspec/`
- [x] Đọc kết quả qua tool giữ tên cũ; thêm tóm tắt từ `list_gaps`
- [x] Phiên rà soát (chế độ `agent`) được cấp thêm `get_analysis`, `list_gaps`
- [x] Duyệt và baseline ở `gate_testspec` qua `approve_nodes`, `baseline_spec`
- [x] Test workflow mock: chế độ `service` và `agent` chạy hết

Tiêu chí xong: `python -m workflow auto --mode mock --testspec-mode agent` chạy hết với tsagent v2.

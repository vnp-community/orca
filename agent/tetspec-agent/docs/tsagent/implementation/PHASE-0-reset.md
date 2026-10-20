# Giai đoạn 0: Đo mốc, lưu trữ, xóa v1, dựng khung mới

Mục tiêu: giữ lại mọi thứ cần để so sánh và tham khảo, rồi xóa sạch code v1 và dựng khung dự án v2 có hạ tầng test.

> Thư mục `backend/agents/testspec-agent/` hiện **chưa được commit vào git**. T0.1 và T0.2 phải xong trước T0.3, nếu không v1 mất hẳn.

---

## T0.1 Đo mốc v1 và xuất fixture

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** —
- **Đầu ra:** `implementation/BASELINE-V1.md`, `implementation/fixtures-v1/`

Checklist:
- [x] Chạy v1 trên MiniPay CR-001 chế độ `none` và `claude-cli`; ghi số TR, TD, CI, TC theo mức, số nút `pendingConfirmation`, số lượt LLM, thời gian
- [x] Ghi danh sách khoảng trống đã biết của v1 (đánh giá Test Requirement và KG-TSAGENT mục 4): trùng đồng thời, chết giữa chừng, ngân sách xấu nhất, phân quyền từng endpoint, test NFR "vượt ngưỡng + 1", TR chép tên FR, lan `pendingConfirmation`
- [x] Ghi danh sách **hành vi cần giữ** để viết lại thành test: phiên bản theo nút (V1–V11), Draft ghi lại tại chỗ, bất biến Baselined, Retired có CR, hand-edit bị từ chối, md giống từng byte khi chạy lại, chính sách lỗi LLM (429, 5xx, timeout, sai schema, 401/403 dừng run), giới hạn đường dẫn `TSAGENT_SOURCE_ROOTS`, MCP xác thực và Origin, cắt kết quả 30.000 ký tự, mã feature chống path traversal
- [x] Viết `scripts/export_tdd_kg_json.py` (đặt trong thư mục lưu trữ tạm, chuyển vào dự án mới ở T0.4) xuất gói `tdd-kg/v1` từ `tmp/minipay-demo` (`slice` CR-001 và `full`)
- [x] Lưu fixture: `cr001-slice.tdd-kg.json`, `full.tdd-kg.json`, `solution.md`, `srs.md`, `supplement.md` của CR-001; bộ md PAY (`urd.md`, `srs.md`, `tdd.md`, `solution.md`, `cr_timeout.md`)
- [x] Lưu ba tài liệu test specs v1 của CR-001 (đã có ở `output-docs/CR-001-llm/`) làm mốc so sánh nội dung

Tiêu chí xong: BASELINE-V1.md có số liệu, danh sách khoảng trống, danh sách hành vi cần giữ; fixture có đủ.

---

## T0.2 Lưu trữ v1

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T0.1
- **Đầu ra:** `implementation/ARCHIVE-V1.md`

Checklist:
- [x] Chủ dự án chọn nơi lưu: (a) commit v1 vào nhánh `archive/tsagent-v1` rồi tag `tsagent-v1-final`, hoặc (b) tệp nén có băm ở vị trí lưu trữ của dự án
- [x] Loại khỏi bản lưu: `.venv`, `.tsagent/`, `tmp/.claude-run/`, `__pycache__`, `bin/`, cache
- [x] Ghi vào ARCHIVE-V1.md: vị trí, commit hoặc băm tệp, ngày, danh sách thư mục, cách khôi phục và chạy lại v1 (cổng khác, store riêng)
- [x] Kiểm khôi phục: giải nén hoặc checkout vào thư mục tạm, `make init` và `make test` của v1 xanh (134 test)

Tiêu chí xong: khôi phục được v1 từ bản lưu và test xanh.

---

## T0.3 Xóa v1 và dựng khung dự án

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T0.2
- **File:** toàn bộ `backend/agents/testspec-agent/`

Checklist:
- [x] Xóa toàn bộ `tsagent/`, `tests/`, `examples/`, `specs/`, `deploy/`, `bin/`, `.tsagent/`, `testspec_agent.egg-info/`, `.venv/`; giữ `tmp/minipay-demo` (demo, gitignore)
- [x] `pyproject.toml` mới: Python ≥ 3.12; phụ thuộc `httpx`, `jsonschema`, `pyyaml`, `markdown-it-py`; dev `pytest`, `ruff`; extras `rdf = ["pyoxigraph"]` cho plugin Turtle; entry point `tsagent`
- [x] `Makefile`: `init` (tạo `.venv` trong thư mục agent, cài `-e .[dev]`, khởi tạo DB), `test`, `lint`, `codegen`, `serve`, `clean`; các mục `build`, `start-dev`, `stop-dev` để trống tới T7.5
- [x] Dựng các package rỗng theo PLAN mục 6, mỗi `__init__.py` có docstring trách nhiệm
- [x] `.gitignore` cho `.venv`, `.tsagent/`, `*.db`, `__pycache__`, `tmp/`
- [x] `README.md` khung: mục đích, cách cài (`make init`), trạng thái "đang xây mới", trỏ tới TDD v2 và kế hoạch
- [x] `make init && make test` chạy được (chưa có test thì 0 test, không lỗi)
- [x] `make lint` (ruff) cấu hình độ dài dòng, quy tắc import

Tiêu chí xong: thư mục agent chỉ còn khung v2; `make init`, `make test`, `make lint` chạy.

---

## T0.4 Hạ tầng test

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T0.3

Checklist:
- [x] Cấu trúc `tests/unit`, `tests/integration`, `tests/system`, `tests/contract`, `tests/fixtures`
- [x] Chép fixture từ T0.1 vào `tests/fixtures/minipay/` và `tests/fixtures/pay/`; chép `scripts/export_tdd_kg_json.py` vào dự án
- [x] `conftest.py`: fixture `tmp_db` (DB tạm đã khởi tạo lược đồ), `cfg` (cấu hình trỏ thư mục tạm), `minipay_pkg`, `pay_docs`
- [x] Đánh dấu pytest: `unit`, `integration`, `system`, `contract`, `llm_real` (bỏ qua mặc định)
- [x] Hướng dẫn chạy từng nhóm test trong README

Tiêu chí xong: `pytest -m unit` chạy được với fixture có sẵn.

# test-script-agent (TSA)

Sinh test script pytest bám sát test spec từ TCIR (`tcir/v1`) của tsagent, vào một git worktree riêng của repo test. Thiết kế: [docs/test-script-agent/TDD-TEST-SCRIPT-AGENT.md](../../docs/test-script-agent/TDD-TEST-SCRIPT-AGENT.md) (phương án C).

TSA không chạy test, không mở PR và không đọc source ứng dụng. Chạy script (runtime) và duyệt PR là các bước sau.

## Luồng

1. Đọc TCIR qua MCP (`testspec_tcir_get` của tdd-agent hoặc `get_tcir` của gateway tsagent) hoặc từ file JSON.
2. Kiểm TCIR. Case không có expected, không tự động hoá được, hoặc sai mã thì bị bỏ qua kèm lý do, TSA không đoán.
3. Tạo worktree trên nhánh `tsa/<feature>-v<specVersion>` từ `--base`, mặc định tại `<repo>/../<repo>.tsa-worktrees/`.
4. Lập kế hoạch: mỗi test set một file `tests/tsa/<feature>/test_tsa_<feature>_<set>.py`.
5. Sinh script:
   - `claude`: Claude CLI chạy `-p` trong worktree, chỉ có tool Read/Write/Edit/Glob/Grep (không Bash, không MCP).
   - `template`: khung tất định.
6. Kiểm tra tĩnh: compile, `ruff` (lỗi cú pháp, tên chưa định nghĩa), fixture tồn tại, không hard-code secret.
7. Đối chiếu spec (tất định): mỗi expected đúng một `assert` có `# @exp <TC>.E<n>` trong hàm `@pytest.mark.tc("<TC>")`, cùng toán tử, cùng giá trị nếu giá trị là một token. Bị chặn khi assertion thừa, nằm trong try/except, là `assert True`, hoặc test bị skip.
8. Tự sửa tối đa `--max-fix` vòng (mặc định 3), chỉ với lỗi ở bước 6–7. Diff guard: vòng sửa đổi một assertion đã khớp spec thì bị huỷ, file quay về bản trước và được đánh dấu `needs-review`.
9. Ghi `assertion-manifest.json`, `tsa-report.md`, rồi commit vào nhánh worktree.

## Cài đặt

```bash
uv venv --python 3.11 .venv && uv pip install --python .venv/bin/python -e '.[dev]'
```

## Dùng

Qua MCP của tdd-agent (dự án đã chạy test spec):

```bash
.venv/bin/tsa generate --feature TDD-P --repo ../qa-tests --base develop --mcp-command "tdd-agent -C /path/project mcp serve"
```

Qua gateway tsagent. Biến môi trường phải truyền trong lệnh vì server stdio không kế thừa môi trường:

```bash
.venv/bin/tsa generate --feature PAY --repo ../qa-tests --base develop --mcp-tool get_tcir --mcp-command "env TSAGENT_DB=/data/tsagent.db TSAGENT_LLM=none tsagent serve --stdio"
```

Từ file, khung template, một case:

```bash
.venv/bin/tsa generate --feature PAY --repo ../qa-tests --tcir-file tests/fixtures/pay-tcir.json --generator template --case TC-L1-PAY-SCN-01-errorHandling-fail-INT-01-c
```

Kiểm lại file đã sửa tay trong worktree:

```bash
.venv/bin/tsa check --tcir-file tcir.json --worktree ../qa-tests.tsa-worktrees/tsa-PAY-v1
```

Kết quả JSON in ra stdout, tiến trình in ra stderr. Mã thoát: 0 khi mọi file `generated`, 1 khi có file `needs-review`/`failed`, 2 khi lỗi (mã lỗi trong `error.code`).

Tuỳ chọn khác:

- `--level L1|L2|L3`: chỉ sinh case của một mức.
- `--only-approved`: chỉ case đã duyệt ở tsagent.
- `--reuse`: tiếp tục trong worktree hoặc nhánh đã có.
- `--no-commit`
- `--model`: model cho Claude CLI.

## Chạy như MCP server (Orca gọi qua Dev Server Agent)

```bash
tsa -C /path/project mcp serve --transport stdio
```

Tool: `tsa_start` (chạy nền, trả `job_id` ngay; lặp lại cùng `external` + feature + `spec_version` trả job cũ), `tsa_get`, `tsa_report`, `tsa_file` (chỉ đọc được script, manifest, báo cáo của job), `tsa_cancel`. Job lưu ở `<repo>/.git/tsa/jobs/`. Khi repo test cũng là repo ứng dụng, worktree chỉ checkout `tests_paths` (mặc định `tests`) và tệp cấu hình pytest ở gốc (sparse checkout theo worktree, repo chính không đổi). Giá trị mặc định của dự án đặt ở `.tsa/config.toml`:

```toml
[tsa]
generator = "claude"      # hoặc "template"
tests_paths = ["tests"]
# test_repo = "/path/qa-repo"   # repo test riêng
# tcir_command = "tdd-agent -C /path/project mcp serve --transport stdio"
```

Tích hợp với Orca: [TDD-REQ-TESTSCRIPT-001](../../docs/integrate/TDD-REQ-TESTSCRIPT-001-test-scripts-in-request-execution.md).

## Xem kết quả

Worktree hiện trong Orca như các worktree agent khác: diff so với nhánh gốc, `tests/tsa/<feature>/tsa-report.md` (tổng quan, độ phủ expected, vấn đề), `assertion-manifest.json` (expected → file:dòng).

## Test

```bash
.venv/bin/pytest
.venv/bin/ruff check src tests
```

Test dùng TCIR thật của bộ PAY (`tests/fixtures/pay-tcir.json`, lấy từ golden của tsagent) và một repo git tạm. Không gọi LLM.

## Giới hạn hiện tại

- Chỉ pytest.
- TCIR chưa có `target` cho từng expected, nên checker chỉ so toán tử và giá trị. Giá trị dạng văn xuôi chỉ được so toán tử, và báo cáo liệt kê để QA đọc lại.
- Orca gọi qua MCP (`tsa mcp serve`), không có `POST /jobs`.

# Giai đoạn 4: Client LLM

Mục tiêu: client LLM mới cho ba nhà cung cấp, chính sách lỗi theo INT-01, trả JSON đúng schema, cache để chạy lại cho kết quả giống nhau.

Tham chiếu: TDD v2 mục 6.1, MECH-05, 09, 11; TDD v1 mục 6.1 (chính sách INT-01, giữ nguyên giá trị); hành vi cần giữ trong BASELINE-V1.md.

---

## T4.1 Nhà cung cấp HTTP (anthropic, openai)

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T1.1
- **File:** `tsagent/llm/providers/http_base.py`, `anthropic.py`, `openai.py`, `tsagent/llm/policy.py`

Checklist:
- [x] Giao diện `Provider.send(system, user) -> text` và `tag`
- [x] httpx client có pool (tối đa 8), timeout tách connect, read, pool acquire
- [x] Retry theo ErrorMapping: 429 (tôn trọng `Retry-After`), 5xx, lỗi mạng là transient; timeout là unknownOutcome; 4xx là permanent; 401, 403 dừng run
- [x] Backoff mũ có jitter, giới hạn tổng thời gian
- [x] Circuit breaker (tỷ lệ lỗi, số lượt tối thiểu, thời gian mở, nửa mở)
- [x] Khóa API chỉ đọc từ biến môi trường, không vào log
- [x] Test `tests/unit/test_llm_http.py` với máy chủ HTTP giả: từng mã lỗi, retry, breaker, khóa không lọt log

Tiêu chí xong: test xanh.

---

## T4.2 Nhà cung cấp `claude-cli`

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T1.1
- **File:** `tsagent/llm/providers/claude_cli.py`

Checklist:
- [x] Gọi `claude -p --output-format json --tools "" --strict-mcp-config --no-session-persistence --disable-slash-commands --system-prompt …`, prompt qua stdin, chạy trong thư mục trống
- [x] Phân loại lỗi từ `is_error`, `api_error_status`; chưa đăng nhập, 401, 403 là lỗi dừng; timeout tiến trình là unknownOutcome
- [x] Không dùng `--bare` (tắt đăng nhập OAuth)
- [x] Test `tests/unit/test_llm_claude_cli.py` với `subprocess.run` giả và một tệp `claude` giả chạy thật (prompt lớn qua stdin)

Tiêu chí xong: test xanh; thử thật một lượt với `claude` đã đăng nhập (đánh dấu `llm_real`).

---

## T4.3 `complete_json`, cache, trần

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T4.1, T4.2, T1.4
- **File:** `tsagent/llm/client.py`, `tsagent/llm/cache.py`

Checklist:
- [x] `complete_json(step, system, user, schema_name)`: nối JSON Schema vào system prompt; kiểm đầu ra bằng `jsonschema`; đúng một lượt sửa kèm lỗi
- [x] Bọc dữ liệu nguồn trong thẻ `<data>`, vô hiệu thẻ đóng trong dữ liệu (MECH-05)
- [x] Cache tệp theo băm (provider, mô hình, system, schema, user); ghi nguyên tử
- [x] Trần lượt gọi theo bước (`analysis`, `design`, `specification`); hết trần ném `BudgetExceeded`
- [x] Đếm lượt gọi và cache hit, trả về cho `run_step`
- [x] Chế độ `none`: mọi lời gọi ném lỗi rõ để tầng trên dùng dự phòng
- [x] Test `tests/unit/test_llm_client.py`: sửa một lần; schema có trong prompt; cache dùng chung giữa client; trần theo bước; thẻ `<data>` không bị phá

Tiêu chí xong: test xanh.

---

## T4.4 LLM giả cho test

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T4.3
- **File:** `tsagent/llm/fake.py`, `tests/conftest.py`

Checklist:
- [x] `FakeLLM` theo kịch bản: trả JSON cố định, hàm sinh theo đầu vào, lỗi theo mã, chậm, JSON hỏng
- [x] Fixture `fake_llm` trong conftest
- [x] Cách bật qua cấu hình (`TSAGENT_LLM=fake`, chỉ khi chạy test)

Tiêu chí xong: các test của giai đoạn 5, 6 dùng được.

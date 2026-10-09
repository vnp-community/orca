# TASK-E2E-035: Stub `claude` cho CI và kiểm thử

**From Solution:** SOL-E2E-005 · **Resolves:** BUG-E2E-010
**Priority:** P1 · **Service/Area:** tests, ci
**Files:**
- tests/stages/e2e/orca-flows/fake_claude_cli/claude (mới, script)
- tests/stages/e2e/orca-flows/ai_stub_server.py (mới, nguồn HTTP)
- tests/stages/e2e/orca-flows/run_acceptance.py (`--ai-stub`)
**Depends on:** TASK-E2E-033
**Status:** `[ ]` CHƯA LÀM (cập nhật 2026-10-09, chưa commit)

---

## Context

CI không thể đăng nhập tài khoản thật; cần bằng chứng nguồn CLI chạy đúng mà vẫn xác định.

## Việc cần làm

- [ ] Script `claude` giả: nhận `-p`, `--output-format json`, `--tools`, `--system-prompt`; đọc stdin; in JSON xác định cho phân loại, Solution, Plan (nhận diện theo nội dung prompt); `auth status --json` trả `loggedIn` theo biến `FAKE_CLAUDE_LOGGED_IN`. — _chưa làm_
- [ ] Ghi lại đối số và `cwd` nhận được vào tệp để test kiểm tra `--tools ""` và cwd rỗng. — _chưa làm_
- [ ] Runner `--ai-stub`: đặt stub vào `PATH` của agent, khởi động lại agent, chạy test, rồi trả `PATH`. — _chưa làm_
- [ ] Bổ sung stub HTTP cho nguồn 1/2 (tuỳ chọn). — _chưa làm_

## Tiêu chí hoàn thành

- [ ] `python3 run_acceptance.py --ai-stub` chạy `RQL-09`, `AIS-02`, `AIS-04` thành PASS. — _chưa làm_
- [ ] Có test âm: `FAKE_CLAUDE_LOGGED_IN=0` cho `REQUEST_AI_NOT_CONFIGURED`. — _chưa làm_

## Kiểm thử

Chạy thật; báo cáo ghi rõ là stub.

## Kết quả thực thi

Chưa làm (stub `claude` cho CI). Xem TASK-026.

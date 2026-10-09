# TASK-E2E-033: Agent: `ai.complete` qua phiên đăng nhập CLI (nguồn mặc định)

**From Solution:** SOL-E2E-005 · **Resolves:** BUG-E2E-010
**Priority:** P0 · **Service/Area:** agent
**Files:**
- agent/src/relay/ai-complete-cli-session.ts (mới)
- agent/src/relay/ai-complete-source.ts (mới, hàm `resolveAiSource`)
- agent/src/relay/ai-complete-handler.ts
- agent/src/relay/ai-complete-cli-session.test.ts (mới)
**Depends on:** không
**Status:** `[x]` XONG (cập nhật 2026-10-09, chưa commit)

---

## Context

Quyết định 2026-10-09: dùng phiên đăng nhập của dev server thay cho khoá API. `ai.complete` hiện chỉ có HTTP + khoá. Cờ CLI đã đối chiếu với `claude --help` 2.1.266: `-p`, `--output-format json`, `--tools ""`, `--no-session-persistence`, `--system-prompt`. **Không dùng `--bare`** (bỏ qua OAuth/keychain).

## Việc cần làm

- [x] `resolveAiSource(model, params, env)`: env key → `resolvedApiKey` → `cli_session`; trả nguồn đã chọn hoặc lỗi `AI_NO_SOURCE`.
- [x] Provider CLI: spawn `claude -p --output-format json --tools "" --no-session-persistence --system-prompt <...>`; prompt qua stdin; `cwd` là thư mục tạm rỗng được tạo và xoá mỗi lần; môi trường tiến trình chỉ `HOME`, `PATH`, biến proxy.
- [x] Thời gian chờ (mặc định 120 giây), giới hạn kích thước đầu ra, huỷ tiến trình khi quá hạn; tối đa 2 tiến trình đồng thời (hàng đợi, từ chối khi đầy).
- [x] Phân tích JSON đầu ra → `{content, usage}`; ánh xạ lỗi: chưa đăng nhập → `AI_CLI_NOT_LOGGED_IN`, hết phiên → `AI_CLI_SESSION_EXPIRED`, hết hạn mức → `AI_CLI_QUOTA_EXHAUSTED`, còn lại → `AI_CLI_FAILED`.
- [x] Định dạng `json` của `ai.complete`: thêm chỉ dẫn hệ thống "chỉ trả JSON hợp lệ" như đường API.
- [x] Không ghi prompt hay đầu ra vào log; tracer chỉ ghi độ dài và thời gian.
- [x] Comment nêu lý do `--tools ""`, không `--bare`, cwd tạm (G9).

## Tiêu chí hoàn thành

- [~] Test với stub `claude` (TASK-E2E-035): cờ đúng, prompt qua stdin, cwd rỗng, môi trường tối giản. — _kiểm bằng executable `claude` giả ở ranh giới tiến trình, chưa phải stub TASK-035_
- [x] Test: hết giờ thì tiến trình bị giết; đầu ra quá lớn bị cắt; lỗi đăng nhập ánh xạ đúng mã.
- [x] Test thứ tự nguồn: env thắng `resolvedApiKey` thắng CLI.
- [x] Không có prompt trong log/argv.

## Kiểm thử

vitest với stub; một lần kiểm tay với tài khoản thật (ghi kết quả vào PR).

## Kết quả thực thi

Xong. `ai-complete-cli-session.ts`: `claude -p --output-format json --tools ""`, prompt qua stdin, thư mục tạm rỗng, env whitelist (không khoá API), hàng đợi giới hạn, kill cả nhóm tiến trình khi quá giờ; 23 test (3 test chạy `claude` thật với `ORCA_LIVE_CLAUDE=1` đã chạy PASS).

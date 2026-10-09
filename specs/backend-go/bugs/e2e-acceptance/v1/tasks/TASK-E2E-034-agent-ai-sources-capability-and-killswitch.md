# TASK-E2E-034: Agent: công bố `ai.sources`, thăm dò `claude auth status`, công tắc tắt

**From Solution:** SOL-E2E-005 · **Resolves:** BUG-E2E-010
**Priority:** P0 · **Service/Area:** agent, infra-fleet-service
**Files:**
- agent/src/relay/ai-sources-probe.ts (mới)
- agent/src/relay/agent-session-capabilities.ts
- backend-go/ci/check-agent-capability-golden.sh + tệp vàng
- backend-go/services/infra-fleet-service/internal (đọc `ai.sources` vào hồ sơ năng lực)
**Depends on:** TASK-E2E-033
**Status:** `[x]` XONG (cập nhật 2026-10-09, chưa commit)

---

## Context

`request-service` cần biết dev server có thể gọi AI hay không trước khi chạy, thay vì đoán từ lỗi.

## Việc cần làm

- [x] Thăm dò `claude auth status --json` (`loggedIn`, `authMethod`); cache 60 giây; không bao giờ in nội dung xác thực.
- [x] Khi khởi động agent: kiểm `claude --help` có `--tools` và `--no-session-persistence`; thiếu thì tắt nguồn CLI.
- [x] Công bố `ai.sources` (`env_key`, `cli_session`) trong `agent.capabilities`; cập nhật tệp vàng capability.
- [x] Công tắc `ORCA_AI_CLI_SESSION=off` (mặc định bật), ghi trong `deploy/agent/agent-runtime.env.example` cùng các công tắc khác.
- [~] `infra-fleet-service` lưu `ai.sources` trong hồ sơ năng lực và trả qua `GetDevServerCapabilities`; agent cũ thiếu trường → "không biết". — _đi qua JSON năng lực, không thêm cột riêng; agent cũ thiếu trường là "không biết"_

## Tiêu chí hoàn thành

- [x] Chưa đăng nhập: `ai.sources` không có `cli_session`.
- [x] Đã đăng nhập: có `cli_session`.
- [~] Công tắc tắt có hiệu lực sau khởi động lại. — _có test công tắc; chưa thử khởi động lại thật_

## Kiểm thử

vitest cho probe (stub `claude auth status`); kiểm tay.

## Kết quả thực thi

Xong. Báo cáo khả năng có `aiSources` (`env_key`, `cli_session`), công tắc `ORCA_AI_CLI_SESSION=off`; fixture vàng và bản sao ở infra-fleet cập nhật; test.

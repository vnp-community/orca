# Bug Reports — Agent vs HLD/CR/BL v1 (từ `audit/agent/`)

**Module:** `agent/src/**`
**Phát hiện:** 2026-08-08
**Ngữ cảnh:** Rút ra từ audit "code vs thiết kế" 5 mảng (`audit/agent/agent-vs-design-review.md` và 5 file chi tiết đi kèm) — chỉ những phát hiện là **lỗi code thật** (behavior sai, dead code nguy hiểm, khoảng trống bảo mật/logic) được đưa vào đây. Các sai lệch thuần tài liệu (đổi tên hàm, sai port trong doc, mâu thuẫn giữa các bản doc...) không được tạo thành bug — xem trực tiếp trong 5 file audit nếu cần cập nhật tài liệu.

---

## Danh Sách Bugs

| ID | Mức độ | Tiêu đề | Module | Status |
|----|--------|---------|--------|--------|
| [BUG-AG-HLD-001](./BUG-AG-HLD-001-ai-complete-no-credential-store-fallback.md) | 🔴 Critical | `ai.complete` không fallback đọc credential store, chỉ đọc `process.env` | `ai-complete-handler.ts` | ✅ Fixed (2026-08-09) |
| [BUG-AG-HLD-002](./BUG-AG-HLD-002-credential-fallback-injects-ciphertext.md) | 🔴 Critical | Credential fallback inject ciphertext Layer-1 thẳng vào biến env API key | `agent-spawner.ts` | ✅ Fixed (2026-08-09) |
| [BUG-AG-HLD-003](./BUG-AG-HLD-003-git-author-identity-global-mutable.md) | 🟠 High | Git author identity là global mutable config, không gắn request context | `preflight-handler.ts` | ✅ Fixed (2026-08-09) |
| [BUG-AG-HLD-004](./BUG-AG-HLD-004-duplicate-pr-create-implementations.md) | 🟠 High | Hai implementation `git.pr.create`/`github.pr.create` không đồng nhất idempotency | `agent-git-handler.ts`, `external-api-connector.ts` | ✅ Fixed (2026-08-09) |
| [BUG-AG-HLD-005](./BUG-AG-HLD-005-gh-rate-limit-breaker-not-wired.md) | 🟠 High | Circuit breaker `gh` CLI không bảo vệ đường PR/MR create thực tế | `runner.ts`, `gh-rate-limit-breaker.ts` | ✅ Fixed (2026-08-09) |
| [BUG-AG-HLD-006](./BUG-AG-HLD-006-agent-spawn-hardcoded-pty-size.md) | 🟡 Medium | `agent.spawn` không nhận `cols`/`rows`, luôn hardcode 220×50 | `agent-spawner.ts` | ✅ Fixed (2026-08-09) |
| [BUG-AG-HLD-007](./BUG-AG-HLD-007-resume-not-supported-opencode-gemini.md) | 🟡 Medium | Resume session không hoạt động cho OpenCode và Gemini | `agent-spawner.ts` | 🟡 Code done, smoke test pending |
| [BUG-AG-HLD-008](./BUG-AG-HLD-008-trust-preset-field-ignored.md) | 🟡 Medium | `trustPreset` khai báo nhưng không được đọc trong `buildAgentEnv()` | `agent-spawner.ts` | 🟡 Code done, smoke test pending |
| [BUG-AG-HLD-009](./BUG-AG-HLD-009-agent-exec-dead-duplicate-handler.md) | 🟡 Medium | `handleAgentExec()` dead code trùng chức năng, dễ sửa nhầm | `agent-exec-handler.ts` | ✅ Fixed (2026-08-09) |
| [BUG-AG-HLD-010](./BUG-AG-HLD-010-fs-watch-recursive-linux-top-level-only.md) | 🟢 Low | `fs.watch` chỉ watch top-level directory trên Linux | `fs-agent-extensions.ts` | ✅ Fixed (2026-08-09) |
| [BUG-AG-HLD-011](./BUG-AG-HLD-011-gitlab-github-methods-unwired.md) | 🟢 Low | 3 handler GitLab/GitHub đã implement nhưng không wire vào dispatcher | `external-api-connector.ts` | ✅ Fixed (2026-08-09) |
| [BUG-AG-HLD-012](./BUG-AG-HLD-012-ai-provider-handler-dead-code-false-encryption-claim.md) | 🟢 Low | Dead code claim mã hoá AES-256-GCM sai — rủi ro tiềm ẩn nếu bị wire nhầm | `ai-provider-handler.ts` | ✅ Fixed (2026-08-09) |

---

## Phân Loại theo Priority

### 🔴 Critical — Chặn tính năng core
- **BUG-AG-HLD-001**: `ai.complete` fail nếu agent không được spawn kèm sẵn API key env — ảnh hưởng AI task-planning, sinh commit message.
- **BUG-AG-HLD-002**: Nhánh fallback credential luôn inject key sai (ciphertext) — agent tự nhận trong comment là sẽ fail auth.

### 🟠 High — Đúng/an toàn dữ liệu, hoặc rủi ro tạo dữ liệu trùng
- **BUG-AG-HLD-003**: Rủi ro gán nhầm tác giả commit giữa các user dùng chung dev server agent.
- **BUG-AG-HLD-004**: Nguy cơ tạo PR trùng lặp tuỳ theo RPC method được gọi.
- **BUG-AG-HLD-005**: Circuit breaker rate-limit không bảo vệ đúng đường PR/MR/issue chính — rủi ro khi fan-out cao (Task Graph chạy nhiều task song song).

### 🟡 Medium — Ảnh hưởng UX / tính năng không hoạt động đúng như quảng cáo
- **BUG-AG-HLD-006**: PTY size sai lệch với terminal thật của user.
- **BUG-AG-HLD-007**: Resume session âm thầm không hoạt động cho 2/4 agent được quảng cáo hỗ trợ.
- **BUG-AG-HLD-008**: Trust preset không có tác dụng — cần xác minh args mặc định có an toàn không.
- **BUG-AG-HLD-009**: Dead code có docblock đầy đủ dễ khiến người bảo trì sửa nhầm chỗ không chạy.

### 🟢 Low — Technical debt / gap tính năng nhỏ / rủi ro tiềm ẩn
- **BUG-AG-HLD-010**: File-change event bị bỏ sót trong subfolder trên Linux (vi phạm yêu cầu cross-platform AGENTS.md).
- **BUG-AG-HLD-011**: 3 RPC method đã code xong nhưng không gọi được vì thiếu đăng ký dispatcher.
- **BUG-AG-HLD-012**: Dead code với comment bảo mật sai — an toàn hiện tại nhưng nguy hiểm nếu bị tái sử dụng nhầm trong tương lai.

---

## Ghi chú phạm vi

Danh sách này **không bao gồm** các phát hiện thuần về tài liệu từ audit gốc, ví dụ:
- Port Agent WS ghi sai trong doc (6768 vs 6769 thật) — đã có tiền lệ tương tự ở `specs/backend/bugs/terminal-management./BUG-BE-TM-004-agent-ws-server-port-mismatch-hld.md`, không lặp lại ở đây vì root cause nằm ở `backend/`, không phải `agent/`.
- Tên RPC method/namespace khác giữa doc và code (`aiProvider.*` vs `ai.provider.*`, `worktree.*` vs `git.worktree.*`...) — thuần quy ước đặt tên, không đổi hành vi.
- ADR-013/014/015/017/018/019 mô tả kiến trúc "v6.0" chưa triển khai — các ADR này tự khai báo trạng thái Proposed, không phải bug của code hiện tại.

Xem đầy đủ các phát hiện (kể cả doc-only) tại `audit/agent/agent-vs-design-review.md` và 5 file mảng chi tiết.

## Giải Pháp

Toàn bộ 12 bug đã được fix trong code thật ngày 2026-08-09 (16 task, xem [`tasks/00-index.md`](./tasks/00-index.md) và giải pháp gốc tại [`solutions/00-index.md`](./solutions/00-index.md), đối chiếu với `specs/agent/tdd/v5`). 10/12 bug verify đầy đủ bằng typecheck; BUG-007 và BUG-008 code đã xong nhưng **chưa smoke-test** với binary CLI thật (`opencode`/`gemini` không có trong môi trường thực thi) — xem "Phương Án Dự Phòng" trong từng task tương ứng nếu smoke test sau này phát hiện sai lệch. `vitest` không chạy được trong môi trường thực thi (thiếu `config/vitest.config.ts`, vấn đề hạ tầng có sẵn từ trước, không liên quan tới các fix này).

## Tham Khảo

- [Audit tổng `agent/` vs thiết kế](../../../../audit/agent/agent-vs-design-review.md)
- [Connection & Wire Protocol](../../../../audit/agent/connection-wire-protocol-vs-design-review.md)
- [RPC Dispatch & Lifecycle](../../../../audit/agent/rpc-dispatch-lifecycle-vs-design-review.md)
- [PTY / AI Agent CLI Integration](../../../../audit/agent/pty-ai-cli-vs-design-review.md)
- [Git & External API + SSH](../../../../audit/agent/git-ssh-external-api-vs-design-review.md)
- [Credential Relay / FS Watcher / Telemetry](../../../../audit/agent/credential-fswatch-telemetry-vs-design-review.md)

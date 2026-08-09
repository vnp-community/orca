# BUG-AG-HLD-008 — `trustPreset` khai báo trong `AgentEnvRequest` nhưng không được đọc trong `buildAgentEnv()`

**Mức độ:** 🟡 Medium
**Status:** 🔴 Open
**Module:** `agent/src/relay/agent-spawner.ts`
**Phát hiện:** 2026-08-08 (audit `agent/` code vs thiết kế — mảng "PTY / AI Agent CLI Integration")

---

## Mô tả

`AgentEnvRequest` (`agent-spawner.ts:190`) khai báo field `trustPreset` — dự kiến dùng để build args như `--trust`/`--dangerously-skip-permissions` theo mô tả BL-PRF-04.

Nhưng thân hàm `buildAgentEnv()` (`agent/src/relay/agent-spawner.ts:183-256`) **không hề đọc field này ở bất kỳ đâu**. `resolveAgentSpec().buildArgs()` (dòng 120-127 cho claude) build args cố định theo model, không nhận `trustPreset` làm tham số.

## Hậu quả

- Nếu caller (backend/frontend) truyền `trustPreset` với kỳ vọng nó sẽ điều khiển mức độ tin cậy/quyền hạn của AI agent khi spawn (ví dụ chế độ "auto-approve mọi tool call" vs "hỏi xác nhận từng bước"), giá trị này **bị bỏ qua hoàn toàn** — agent luôn spawn với args mặc định, không phản ánh preset mà UI hiển thị cho user.
- Rủi ro bảo mật tiềm tàng nếu logic ngược lại đúng: nếu UI hiển thị "trust preset: restricted" nhưng agent thực tế luôn chạy ở chế độ mặc định (có thể là permissive hơn), user có thể lầm tưởng agent đang bị giới hạn quyền trong khi không phải vậy — cần xác minh args mặc định của từng CLI là an toàn (không auto-skip permission) trước khi coi đây chỉ là "thiếu tính năng".

## Bằng chứng

```
agent/src/relay/agent-spawner.ts:190 → trustPreset khai báo trong AgentEnvRequest
agent/src/relay/agent-spawner.ts:183-256 → buildAgentEnv() không đọc trustPreset ở đâu cả
agent/src/relay/agent-spawner.ts:120-127 → buildArgs() cố định, không nhận trustPreset
```

## Đề xuất fix

- Xác nhận args mặc định hiện tại của từng agent (claude/codex/gemini/opencode/ollama) có an toàn hay không (không tự động skip permission prompt).
- Implement đọc `trustPreset` trong `buildAgentArgs`, map sang flag tương ứng của từng CLI (`--dangerously-skip-permissions` cho claude khi preset = full-trust, v.v.), hoặc xoá field khỏi interface nếu tính năng này không còn trong roadmap để tránh giả định sai.

## Tham khảo

- Audit: `audit/agent/pty-ai-cli-vs-design-review.md` §2.2, §2.3

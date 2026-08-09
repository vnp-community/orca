# BUG-AG-HLD-004 — Hai implementation `git.pr.create` và `github.pr.create` không đồng nhất hành vi

**Mức độ:** 🟠 High
**Status:** 🔴 Open
**Module:** `agent/src/relay/agent-git-handler.ts`, `agent/src/relay/external-api-connector.ts`
**Phát hiện:** 2026-08-08 (audit `agent/` code vs thiết kế — mảng "Git & External API Integration")

---

## Mô tả

RPC dispatch (`agent/src/relay/agent-rpc-dispatch.ts`) đăng ký **hai** method riêng biệt cho cùng một chức năng "tạo Pull Request":

- `case 'git.pr.create'` (dòng 411) → `agent-git-handler.handleGitPrCreate` (dòng 277-332) — tự build `GH_CONFIG_DIR` inline, gọi `gh pr create` trực tiếp, **không có idempotency check**.
- `case 'github.pr.create'` (dòng 488) → `external-api-connector.handleGitHubPrCreate` (dòng 130-191) — có `checkExistingPr()` (dòng 108-126) trước khi gọi `gh pr create`, tránh tạo trùng.

Hai implementation khác nhau, không chia sẻ code, cho cùng một hành động.

## Hậu quả

- Tuỳ theo caller (frontend/backend) gọi `git.pr.create` hay `github.pr.create`, hành vi khi PR đã tồn tại cho branch đó là khác nhau: một đường sẽ tạo **PR trùng lặp**, đường kia sẽ phát hiện và trả về PR đã có.
- Nguy cơ thực tế: người dùng bấm "Create PR" nhiều lần (double-click, retry sau timeout) qua đường `git.pr.create` sẽ tạo nhiều PR trùng cho cùng branch.

## Bằng chứng

```
agent/src/relay/agent-rpc-dispatch.ts:411 → case 'git.pr.create' → handleGitPrCreate (không idempotency)
agent/src/relay/agent-rpc-dispatch.ts:488 → case 'github.pr.create' → handleGitHubPrCreate (có idempotency)
agent/src/relay/agent-git-handler.ts:277-332 → handleGitPrCreate, không check PR tồn tại
agent/src/relay/external-api-connector.ts:108-126 → checkExistingPr(), 130-191 → handleGitHubPrCreate
```

## Đề xuất fix

Hợp nhất về một implementation duy nhất — khuyến nghị giữ `external-api-connector.handleGitHubPrCreate` (đã có idempotency-check), deprecate/xoá `agent-git-handler.handleGitPrCreate`, và định tuyến `git.pr.create` (nếu vẫn cần giữ tên method để tương thích ngược) sang cùng implementation.

## Tham khảo

- Audit: `audit/agent/git-ssh-external-api-vs-design-review.md` §2.1
- Liên quan: BUG-AG-HLD-005 (cả hai đường đều bypass circuit breaker/retry của `runner.ts`)

# BUG-AG-HLD-011 — `handleGitLabMrList`/`handleGitHubAuthStatus`/`handleGitLabAuthStatus` implement sẵn nhưng không wire vào dispatcher

**Mức độ:** 🟢 Low
**Status:** 🔴 Open
**Module:** `agent/src/relay/external-api-connector.ts`, `agent/src/relay/agent-rpc-dispatch.ts`
**Phát hiện:** 2026-08-08 (audit `agent/` code vs thiết kế — mảng "Git & External API Integration")

---

## Mô tả

`external-api-connector.ts` implement đầy đủ 3 handler:

- `handleGitLabMrList` (dòng 394-423)
- `handleGitHubAuthStatus` (dòng 311-339)
- `handleGitLabAuthStatus` (dòng 460-488)

Nhưng `agent-rpc-dispatch.ts` — đã kiểm tra toàn bộ các `case 'git...'`/`'gh...'`/`'gitlab...'` — **không có `case` nào gọi tới 3 hàm này**. Code tồn tại, hoạt động đúng nếu được gọi trực tiếp (có thể verify qua unit test), nhưng không thể truy cập được qua RPC vì thiếu đăng ký trong dispatcher.

## Hậu quả

- Tính năng "xem danh sách MR GitLab", "kiểm tra trạng thái đăng nhập GitHub/GitLab" — nếu UI có nút gọi các RPC method tương ứng (`gitlab.mr.list`, `github.auth.status`, `gitlab.auth.status`) — sẽ luôn nhận lỗi `MethodNotFound` dù logic backend đã sẵn sàng.
- Đây là gap tầm trung: không gây crash hay data corruption, nhưng là tính năng "gần xong" bị bỏ sót một bước cuối cùng, dễ bị quên vì compile/test vẫn pass (test có thể gọi hàm trực tiếp, không qua dispatcher).

## Bằng chứng

```
agent/src/relay/external-api-connector.ts:311-339 → handleGitHubAuthStatus
agent/src/relay/external-api-connector.ts:394-423 → handleGitLabMrList
agent/src/relay/external-api-connector.ts:460-488 → handleGitLabAuthStatus
agent/src/relay/agent-rpc-dispatch.ts → không có case 'gitlab.mr.list' | 'github.auth.status' | 'gitlab.auth.status'
```

## Đề xuất fix

Thêm 3 `case` tương ứng vào `route()` trong `agent-rpc-dispatch.ts`, trỏ tới 3 handler đã có sẵn — tương tự các `case` khác cùng nhóm `github.*`/`gitlab.*` đã được wire (dòng 488-543).

## Tham khảo

- Audit: `audit/agent/git-ssh-external-api-vs-design-review.md` §2.5

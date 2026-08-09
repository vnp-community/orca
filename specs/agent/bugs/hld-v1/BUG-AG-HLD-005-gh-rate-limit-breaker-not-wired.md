# BUG-AG-HLD-005 — Circuit breaker/retry cho `gh` CLI không bảo vệ đường PR/MR create thực tế

**Mức độ:** 🟠 High
**Status:** 🔴 Open
**Module:** `agent/src/main/git/runner.ts`, `agent/src/main/git/gh-rate-limit-breaker.ts`
**Phát hiện:** 2026-08-08 (audit `agent/` code vs thiết kế — mảng "Git & External API Integration")

---

## Mô tả

`agent/src/main/git/runner.ts` implement một bộ chạy `gh`/`glab` CLI tinh vi: circuit breaker chống rate-limit (`ghExecFileAsync`, dòng 1405-1495, dùng `classifyGhRateLimitBucket`/`getGhRateLimitBlockedUntilMs`/`notifyGhPrimaryRateLimit` từ `gh-rate-limit-breaker.ts`), retry transient 5xx/429 với backoff, idempotency-aware retry gate, cùng WSL routing đầy đủ.

Comment đầu `gh-rate-limit-breaker.ts:4-9` xác nhận mục đích thiết kế: chặn "90-repo Tasks-page fan-out storm" — tức bảo vệ khỏi việc gọi `gh` dồn dập từ nhiều request đồng thời làm cạn quota API GitHub.

Nhưng grep xác nhận **chỉ một caller duy nhất trong toàn bộ `agent/`**: `agent/src/main/text-generation/commit-message-text-generation.ts`. Toàn bộ bề mặt RPC `git.pr.create`/`github.pr.create`/`github.pr.merge`/`github.issue.*`/`gitlab.mr.*` — tức đường mà user thực sự dùng để tạo/merge PR, MR, issue — đều tự viết lại `execFile`/`spawn` thô trong `GitHandler`, `agent-git-handler.ts`, `external-api-connector.ts`, **hoàn toàn bypass** `runner.ts`.

## Hậu quả

- Cơ chế bảo vệ rate-limit được xây dựng riêng để chặn đúng kịch bản "fan-out storm" lại **không bảo vệ** con đường có khả năng gây fan-out cao nhất (tạo PR/MR/issue hàng loạt từ nhiều task/worktree cùng lúc) — chỉ bảo vệ một tính năng phụ (generate commit message).
- Nếu một luồng nghiệp vụ (ví dụ Task Graph chạy đồng loạt nhiều task, mỗi task tạo 1 PR khi xong) gây rate-limit thật với GitHub API, không có circuit breaker nào chặn lại — mỗi lần gọi sẽ tự thất bại riêng lẻ thay vì được breaker giữ lại và retry có kiểm soát.

## Bằng chứng

```
agent/src/main/git/runner.ts:1405-1495 → ghExecFileAsync (breaker + retry)
agent/src/main/git/gh-rate-limit-breaker.ts:4-9 → comment giải thích mục đích: chặn fan-out storm
Chỉ 1 caller: agent/src/main/text-generation/commit-message-text-generation.ts
agent/src/relay/git-handler.ts, agent-git-handler.ts, external-api-connector.ts → tự execFile/spawn riêng, không import runner.ts
```

## Đề xuất fix

Route `GitHandler`, `agent-git-handler.ts`, `external-api-connector.ts` qua `ghExecFileAsync`/`glabExecFileAsync` của `runner.ts` cho mọi lệnh `gh`/`glab`, để circuit breaker bảo vệ đúng đường PR/MR/issue chính. Nếu quyết định không cần dùng cho các đường này, cân nhắc bỏ `runner.ts`'s gh/git runner để tránh code chết gây hiểu nhầm về mức độ bảo vệ thực tế.

## Tham khảo

- Audit: `audit/agent/git-ssh-external-api-vs-design-review.md` §2.2
- Liên quan: BUG-AG-HLD-004

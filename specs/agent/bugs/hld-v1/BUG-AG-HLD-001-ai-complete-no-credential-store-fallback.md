# BUG-AG-HLD-001 — `ai.complete` không fallback đọc credential store, chỉ đọc `process.env`

**Mức độ:** 🔴 Critical
**Status:** 🔴 Open
**Module:** `agent/src/relay/ai-complete-handler.ts`
**Phát hiện:** 2026-08-08 (audit `agent/` code vs thiết kế — mảng "AI Provider Credential Relay")

---

## Mô tả

RPC `ai.complete` (dùng cho AI task-planning và sinh commit message) được thiết kế để resolve API key theo 2 tầng ưu tiên — comment đầu file `ai-complete-handler.ts:1-14` mô tả rõ:

1. Env var được inject sẵn lúc agent spawn (`ANTHROPIC_API_KEY`/`OPENAI_API_KEY`/`GOOGLE_API_KEY`)
2. Nếu không có, dùng `ORCA_ACCOUNT_ID` để tra `agent-credential-store.ts`

Nhưng hàm `resolveApiKey()` (`ai-complete-handler.ts:101-115`) **chỉ đọc `process.env['ANTHROPIC_API_KEY'|'OPENAI_API_KEY'|'GOOGLE_API_KEY']`** — hoàn toàn không có bất kỳ lookup nào vào `agent-credential-store.ts`. Tầng (2) đã được document nhưng chưa từng implement.

## Hậu quả

- Bất kỳ lời gọi `ai.complete` nào xảy ra trong một tiến trình agent **không được spawn kèm sẵn** API key env (ví dụ: agent khởi động độc lập, không qua `agent.spawn` với `resolvedApiKey`) sẽ luôn thất bại, dù credential đã được ghi hợp lệ vào `agent-credential-store` qua `ai.provider.writeCredential` trước đó.
- Ảnh hưởng trực tiếp: `TaskAIPlanner.decompose()` (AI task planning) và tính năng sinh commit message tự động — cả hai đều gọi `ai.complete` qua relay.

## Bằng chứng

```
ai-complete-handler.ts:1-14   → comment mô tả đúng 2 tầng resolve
ai-complete-handler.ts:101-115 → resolveApiKey() chỉ đọc process.env, không gọi credential store
```

## Đề xuất fix

Trong `resolveApiKey()`, khi `process.env[...]` rỗng, thêm bước fallback: đọc `ORCA_ACCOUNT_ID` từ context/params, gọi `readDecryptedKey()` (đã có sẵn trong `agent-credential-store.ts`, dùng bởi `agent-spawner.ts`) để lấy key đã giải mã trước khi trả lỗi "no API key".

## Tham khảo

- Audit: `audit/agent/credential-fswatch-telemetry-vs-design-review.md` §2.1
- Liên quan: BUG-AG-HLD-002 (fallback credential injection khi spawn cũng lỗi tương tự)

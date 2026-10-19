# TASK-E2E-025: Test `ai.complete` của agent: thứ tự khoá, không ghi log khoá, URL cơ sở cho kiểm thử

**From Solution:** SOL-E2E-005 · **Resolves:** BUG-E2E-010
**Priority:** P1 · **Service/Area:** agent
**Files:**
- agent/src/relay/ai-complete-handler.ts
- agent/src/relay/ai-complete-handler.test.ts
**Depends on:** TASK-E2E-033
**Status:** `[x]` XONG (cập nhật 2026-10-09, chưa commit)

---

## Context

Handler chấp nhận `resolvedApiKey` nhưng không có test về thứ tự ưu tiên và không-log; URL Anthropic cố định nên không kiểm thử được với stub. Thứ tự đầy đủ sau quyết định: env → `resolvedApiKey` → phiên CLI (TASK-E2E-033).

## Việc cần làm

- [x] Test: env thắng `resolvedApiKey`; `resolvedApiKey` thắng khi không có env; `accountId` chỉ có thì ném lỗi rõ.
- [x] Test: không có chuỗi khoá trong log/trace của handler.
- [x] Thêm biến `ORCA_AI_ANTHROPIC_BASE_URL` (mặc định URL chính thức) chỉ để kiểm thử; ghi comment lý do.

## Tiêu chí hoàn thành

- [x] vitest xanh; mặc định không đổi hành vi.

## Kiểm thử

vitest.

## Kết quả thực thi

Xong. `ai-complete-handler-keys.test.ts`: env thắng `resolvedApiKey`; `resolvedApiKey` khi không có env; chỉ `accountId` ném lỗi rõ; khoá không vào log; `ORCA_AI_ANTHROPIC_BASE_URL` chỉ để kiểm thử (mặc định API chính thức, có test).

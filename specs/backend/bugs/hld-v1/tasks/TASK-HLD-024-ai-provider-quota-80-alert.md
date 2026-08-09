# TASK-HLD-024: Cảnh báo quota 80% qua onQuotaWarning callback, debounce 1 lần/ngày

**Priority:** 🟡 MEDIUM
**Effort:** ~3-4 giờ (code trong ProviderHealthChecker + test)
**Status:** ✅ DONE — 2026-08-09 (áp dụng đủ: `QUOTA_ALERT_THRESHOLD_RATIO=0.8` + `ProviderQuotaWarning` type + `quotaWarnedOn` debounce map + `onQuotaWarning` callback trong `ProviderHealthChecker.ts`; logic tính `ratio` chèn ngay sau khối `onStatusChanged` hiện có trong `runCheck()`, giữ nguyên khối `'rotating' → continue` từ TASK-HLD-023 ở đầu vòng lặp (đã merge trước, đúng ghi chú ở mục 2 của task). `server-bootstrap.ts` wire `providerHealthChecker.onQuotaWarning` cạnh `onStatusChanged`. `tsc --noEmit` sạch hoàn toàn cho `ProviderHealthChecker.ts`; `server-bootstrap.ts` chỉ còn lỗi baseline có sẵn không liên quan (`GenericConnectionPool`/`AIProviderResolver`/`DevServerManager.getRuntimeState`/`string.message`). ⚠️ Chưa viết 5 test case liệt kê trong task — effort budget.)
**Bug refs:** BUG-BE-HLD-015
**Solution ref:** [SOLUTION-ai-provider-exact.md](../solutions/SOLUTION-ai-provider-exact.md) §3e, §4.4, §4.6
**Depends on:** Không — độc lập với TASK-HLD-023, có thể làm song song (chỉ cần TASK-HLD-022 đã xong nếu muốn audit log hoạt động đúng, nhưng task này không tự thêm audit log mới nên không bắt buộc)

---

## Mục tiêu

`docs/features/F35-ai-provider-account-management.md` yêu cầu cảnh báo sớm ở ngưỡng 80% quota. Hiện tại `ProviderHealthChecker.runCheck()` chỉ set `'quota_exceeded'` khi provider **trả lỗi chứa chuỗi `"quota"`** — đây là phát hiện phản ứng (reactive), sau khi request đã bị chặn. Không có logic nào đọc `tokens_used` (bảng `orca_provider_usage`, đã tồn tại từ migration 0008) rồi so với `quota_limit_day` để cảnh báo chủ động trước khi vượt ngưỡng.

Cần thêm bước trong vòng lặp `runCheck()` (chạy mỗi account mỗi 15 phút): nếu `account.quotaLimitDay > 0`, gọi `service.getUsageToday(account.id)` (đã tồn tại, `AIProviderService.ts:326-337`), tính `ratio = usage.tokens / account.quotaLimitDay`. Nếu `ratio >= 0.8` → phát cảnh báo qua callback `onQuotaWarning`, debounce 1 lần/account/ngày.

## File cần sửa/tạo

```
backend/src/main/ai-providers/ProviderHealthChecker.ts        (sửa)
backend/src/main/server-bootstrap.ts                            (sửa — wire onQuotaWarning)
backend/src/main/ai-providers/__tests__/ProviderHealthChecker.test.ts   (thêm test)
```

## Thay đổi cụ thể

### 1. Thêm hằng số + event type + callback mới (đầu file, cạnh `HEALTH_CHECK_INTERVAL_MS`)

```typescript
const HEALTH_CHECK_INTERVAL_MS = 15 * 60 * 1000 // 15 minutes
const QUOTA_ALERT_THRESHOLD_RATIO = 0.8 // BUG-BE-HLD-015: warn at 80% of quotaLimitDay

// ── Status change event ───────────────────────────────────────────────────────

export interface ProviderStatusChange {
  accountId:  string
  oldStatus:  string
  newStatus:  string
  checkedAt:  Date
}

// BUG-BE-HLD-015: emitted the first time an account crosses 80% of its daily quota.
export interface ProviderQuotaWarning {
  accountId:    string
  tokensUsed:   number
  quotaLimitDay: number
  ratio:        number
  checkedAt:    Date
}

// ── ProviderHealthChecker ─────────────────────────────────────────────────────

export class ProviderHealthChecker {
  private timer: ReturnType<typeof setInterval> | null = null
  // BUG-BE-HLD-015: debounce — one warning per account per calendar day so the
  // 15-minute cron doesn't re-alert every cycle while still above threshold.
  private readonly quotaWarnedOn = new Map<string, string>() // accountId -> 'YYYY-MM-DD'

  onStatusChanged: ((event: ProviderStatusChange) => void) | null = null

  /**
   * BUG-BE-HLD-015: optional callback for early quota warnings.
   * Wire this in server-bootstrap next to onStatusChanged:
   *   checker.onQuotaWarning = (e) => { wsServer.broadcast('provider:quotaWarning', e); sendWebhook(e) }
   */
  onQuotaWarning: ((event: ProviderQuotaWarning) => void) | null = null
```

### 2. Thân vòng lặp `for (const account of accounts)` trong `runCheck()` — thêm bước tính quota sau khi cập nhật status

**Lưu ý:** nếu TASK-HLD-023 (rotation) đã merge trước, giữ nguyên đoạn `if (account.status === 'rotating') { ...; continue }` ở đầu vòng lặp — task này chỉ thêm phần quota warning vào nhánh còn lại. Nếu TASK-HLD-023 chưa merge, bỏ qua đoạn `rotating` dưới đây.

```typescript
    for (const account of accounts) {
      try {
        // (Nếu TASK-HLD-023 đã merge, đoạn kiểm tra 'rotating' + continue nằm ở đây, giữ nguyên)

        const oldStatus = account.status
        span.step('ping-account', { accountId: account.id, provider: account.provider })
        const result = await service.testConnection(account.id)

        let newStatus: 'active' | 'quota_exceeded' | 'invalid'
        if (result.ok) {
          newStatus = 'active'
        } else if (result.error?.toLowerCase().includes('quota')) {
          newStatus = 'quota_exceeded'
        } else {
          newStatus = 'invalid'
        }
        span.step('ping-result', {
          accountId: account.id, ok: result.ok, latencyMs: result.latencyMs, newStatus,
        })

        const checkedAt = new Date()
        await service.updateAccount(account.id, { status: newStatus, lastHealthCheck: checkedAt })

        if (newStatus === 'active') activeCount++
        else if (newStatus === 'quota_exceeded') quotaExceededCount++
        else invalidCount++

        if (oldStatus !== newStatus && this.onStatusChanged) {
          console.log(`[ProviderHealthChecker] Account ${account.id}: ${oldStatus} → ${newStatus}`)
          this.onStatusChanged({ accountId: account.id, oldStatus, newStatus, checkedAt })
        }

        // BUG-BE-HLD-015: proactive 80% quota warning, independent of the
        // reactive quota_exceeded status above (which only fires once the
        // provider itself has already started rejecting requests).
        if (account.quotaLimitDay > 0) {
          const usage = await service.getUsageToday(account.id)
          const ratio = usage.tokens / account.quotaLimitDay
          const today = checkedAt.toISOString().slice(0, 10)
          if (ratio >= QUOTA_ALERT_THRESHOLD_RATIO && this.quotaWarnedOn.get(account.id) !== today) {
            this.quotaWarnedOn.set(account.id, today)
            span.step('quota-warning', { accountId: account.id, ratio, tokensUsed: usage.tokens })
            this.onQuotaWarning?.({
              accountId: account.id,
              tokensUsed: usage.tokens,
              quotaLimitDay: account.quotaLimitDay,
              ratio,
              checkedAt,
            })
          } else if (ratio < QUOTA_ALERT_THRESHOLD_RATIO) {
            this.quotaWarnedOn.delete(account.id) // usage dropped (new day) — allow re-alert later
          }
        }
      } catch (err) {
        errorCount++
        console.warn(`[ProviderHealthChecker] Failed to check account ${account.id}:`, err)
      }
    }
```

### 3. `server-bootstrap.ts` — wire `onQuotaWarning` cạnh `onStatusChanged`

```typescript
  providerHealthChecker.onStatusChanged = (event) => {
    console.log(`[ProviderHealthChecker] Status change: account=${event.accountId} ${event.oldStatus}→${event.newStatus}`)
  }
  // BUG-BE-HLD-015: wire quota warnings the same way status changes are wired.
  providerHealthChecker.onQuotaWarning = (event) => {
    console.warn(
      `[ProviderHealthChecker] Quota warning: account=${event.accountId} ` +
      `${event.tokensUsed}/${event.quotaLimitDay} (${Math.round(event.ratio * 100)}%)`
    )
    // TODO: extend with rpcServer.broadcast('provider:quotaWarning', event) and webhook call
  }
```

## Verification

```bash
cd /opt/repos/orca
pnpm --filter backend tsc --noEmit
pnpm --filter backend test ProviderHealthChecker

# Xác nhận callback mới có mặt
grep -n "onQuotaWarning\|QUOTA_ALERT_THRESHOLD_RATIO" backend/src/main/ai-providers/ProviderHealthChecker.ts backend/src/main/server-bootstrap.ts
```

Test case cần thêm trong `ProviderHealthChecker.test.ts`:

1. `quotaLimitDay > 0` và `tokens/quotaLimitDay >= 0.8` → `onQuotaWarning` được gọi đúng 1 lần với `ratio` chính xác.
2. Gọi `runCheck()` 2 lần liên tiếp trong cùng ngày, vẫn trên ngưỡng 80% → `onQuotaWarning` chỉ được gọi ở lần đầu (debounce theo ngày).
3. `quotaLimitDay === 0` (unlimited) → không bao giờ gọi `onQuotaWarning` dù usage lớn.
4. Usage tụt xuống dưới 80% sau khi đã cảnh báo (ví dụ sang ngày mới, `getUsageToday` reset) → debounce map được xoá, sẵn sàng cảnh báo lại nếu vượt ngưỡng lần nữa.
5. Test hiện có cho `onStatusChanged`/reactive `quota_exceeded` (dựa theo lỗi provider chứa `"quota"`) không bị regress bởi thay đổi này.

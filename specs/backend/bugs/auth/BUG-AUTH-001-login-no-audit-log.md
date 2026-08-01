# BUG-AUTH-001: Login không ghi Audit Log — BL-AUTH-01 incomplete

**Status:** ✅ FIXED — 2026-08-01  
**Fixed by:** TASK-AUTH-002  
**Implementation:** audit-logger.ts: AuditLogger created, wired into AuthManager.login()  

## Mức độ: 🟡 MEDIUM

## Tóm tắt

HLD BL-AUTH-01:
```
[SRV] IF fail: INSERT orca_audit_log { action: 'login.fail' }
[SRV] INSERT orca_audit_log { action: 'login.success' }
```

Thực tế `src/main/auth/auth-local-handler.ts:28-57`:
```typescript
async login(input, ipAddress, userAgent): Promise<LocalLoginResult> {
  // Validate input
  // Verify credentials: userStore.verifyPassword()
  if (!user) {
    return { success: false, error: 'invalid_credentials' }  ← không có audit log
  }
  const session = await this.sessionStore.createSession(...)
  return { success: true, sessionId: session.sessionId, user }  ← không có audit log
}
```

`AuthLocalHandler` không inject `AuditLogger` → không ghi login.success hoặc login.fail.

Audit log chỉ được dùng trong `AdminUserHandlers` và `AdminSessionHandlers` (admin actions), không có trong auth login flow.

## Ảnh hưởng

1. **BL-AUTH-05 (Audit Log)**: Query `GET /admin/api/audit?action=login.fail` → 0 records → không phát hiện brute force
2. Không có audit trail cho login events → compliance issue
3. Admin không thể xem lịch sử đăng nhập

## Fix đề xuất

Inject `AuditLogger` vào `AuthLocalHandler`:
```typescript
export class AuthLocalHandler {
  constructor(
    private readonly userStore: AuthUserStore,
    private readonly sessionStore: AuthSessionStore,
    private readonly auditLogger?: AuditLogger  // optional - backward compat
  ) {}

  async login(input, ipAddress, userAgent): Promise<LocalLoginResult> {
    // ...
    if (!user) {
      await this.auditLogger?.log({ 
        action: 'login.fail', 
        actorId: null, 
        metadata: { email: input.email, ip: ipAddress } 
      })
      return { success: false, error: 'invalid_credentials' }
    }
    await this.auditLogger?.log({ 
      action: 'login.success', 
      actorId: user.id, 
      metadata: { ip: ipAddress, userAgent } 
    })
    // ...
  }
}
```

## Files liên quan

- `src/main/auth/auth-local-handler.ts:28-57`: login handler thiếu audit
- `src/main/admin/audit-logger.ts`: AuditLogger exists but not wired to auth

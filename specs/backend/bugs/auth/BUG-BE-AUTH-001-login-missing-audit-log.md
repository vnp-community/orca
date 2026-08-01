# BUG-BE-AUTH-001: `auth-local-handler.ts` không ghi audit log khi login thành công hoặc thất bại

**Status:** ✅ FIXED — 2026-08-01  
**Fixed by:** TASK-AUTH-002  
**Implementation:** audit-logger.ts: fire-and-forget login audit  

## Mức độ: 🟡 MEDIUM

## Tóm tắt

HLD (BL-AUTH-01) yêu cầu:
```
├─ bcrypt.compare(password, user.passwordHash)
├─ IF fail: INSERT orca_audit_log { action: 'login.fail' }  ← DB
│           return 401 Unauthorized
├─ INSERT orca_sessions { token, userId, expiresAt, lastSeenAt }  ← DB
├─ INSERT orca_audit_log { action: 'login.success' }  ← DB
```

Nhưng `AuthLocalHandler.login()` không có bất kỳ lời gọi audit log nào:

```typescript
// auth-local-handler.ts:28-58
async login(input, ipAddress, userAgent): Promise<LocalLoginResult> {
  // Step 1: Validate
  // Step 2: Verify credentials
  const user = await this.userStore.verifyPassword(input.email, input.password)
  if (!user) {
    return { success: false, error: 'invalid_credentials' }
    // ← THIẾU: INSERT orca_audit_log { action: 'login.fail' }
  }
  // Step 3: Create session
  const session = await this.sessionStore.createSession(...)
  return { success: true, sessionId: session.sessionId, user }
  // ← THIẾU: INSERT orca_audit_log { action: 'login.success' }
}
```

## File liên quan

- [`src/main/auth/auth-local-handler.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/auth/auth-local-handler.ts) — Lines 28-58

## Ảnh hưởng

1. **BL-AUTH-05 Audit Log** không được gọi cho login events.
2. Admin không thể track failed login attempts → brute-force detection không hoạt động.
3. `GET /admin/api/audit?action=login.fail` không trả kết quả nào → audit dashboard trống.
4. Compliance requirement (SOC2, GDPR) yêu cầu login audit trail — bị vi phạm.

## Cách fix đề xuất

Inject `AuditLogger` vào `AuthLocalHandler`:

```typescript
export class AuthLocalHandler {
  constructor(
    private readonly userStore: AuthUserStore,
    private readonly sessionStore: AuthSessionStore,
    private readonly auditLogger: AuditLogger  // ← thêm
  ) {}

  async login(input, ipAddress, userAgent) {
    // ...
    if (!user) {
      this.auditLogger.log({
        action: 'login.fail',
        userEmail: input.email,
        ipAddress,
        detail: { reason: 'invalid_credentials' }
      })
      return { success: false, error: 'invalid_credentials' }
    }
    // ...
    this.auditLogger.log({
      action: 'login.success',
      userId: user.id,
      userEmail: user.email,
      ipAddress
    })
    return { success: true, sessionId: session.sessionId, user }
  }
}
```

## Liên quan đến luồng

- **BL-AUTH-01**: Login flow — audit log missing.
- **BL-AUTH-05**: Audit Log — login events không được ghi.

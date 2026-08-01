# BL-AUTH-02: Session Management & Isolation

**Domain:** Authentication & User Management  
**Priority:** P0  
**Actor chính:** Mọi user  
**Tham chiếu:** FR-11.2, UR-111, F23, F24

---

## Mô tả

Hệ thống quản lý vòng đời của auth session: validate, renew, revoke. Session middleware inject userId vào mọi request và WebSocket connection.

## Session Lifecycle

```
CREATE  → login success → INSERT orca_sessions (UUID, userId, expires_at = now+8h)
VALIDATE→ per-request  → SELECT WHERE token=? AND expires_at > now
RENEW   → per-request  → UPDATE last_seen_at = now (sliding window option)
REVOKE  → logout/admin → DELETE WHERE id = ?
EXPIRE  → background   → DELETE WHERE expires_at < now (cleanup job)
```

## requireAuth() Middleware

```typescript
// Flow:
1. Extract cookie: orca_session from request headers
2. Lookup session in orca_sessions (JOIN orca_users)
3. Check: expires_at > NOW AND user.is_active = 1
4. Update: last_seen_at = NOW
5. Inject: req.userId, req.userRole
6. Nếu invalid → 401 Unauthorized (HTTP) hoặc WS close(4001)
```

## WebSocket Session Routing

```
WS Upgrade request:
1. requireAuth() validates orca_session cookie
2. WsSessionRouter.route(userId, ws)
3. SessionManager.getOrCreate(userId) → fork process
4. Pipe WS ↔ Unix socket (~/.orca/users/<userId>/orca.sock)
5. On WS close → cleanup proxy
```

## Per-User Process Isolation

| Aspect | Implementation |
|--------|----------------|
| Process type | Node.js fork() |
| Data path | `~/.orca/users/<userId>/` |
| Socket | `~/.orca/users/<userId>/orca.sock` |
| Idle timeout | 4h (no WS connection) |
| Spawn timeout | 30s → error |
| Max respawns | 3 (then abandon + alert admin) |

## Source References

- `src/main/auth/auth-middleware.ts` — requireAuth, requireAdmin
- `src/main/session/session-manager.ts` — fork, idle timeout
- `src/main/session/ws-session-router.ts` — WS proxy routing

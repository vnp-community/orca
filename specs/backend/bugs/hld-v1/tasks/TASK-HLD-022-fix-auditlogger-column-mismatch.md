# TASK-HLD-022: Fix AuditLogger.log() insert sai cột so với schema thật orca_audit_log

**Priority:** 🔴 CRITICAL — **ƯU TIÊN CAO, làm trước TASK-HLD-023 và TASK-HLD-024**
**Effort:** ~1-2 giờ (SQL fix + verify migration 0005 + test)
**Status:** ✅ DONE — 2026-08-09 (xác nhận bug có thật qua đọc trực tiếp migration 0005 thật — cột thật `id, created_at, user_id, user_email, action, detail, ip_address`, không có `ip`/`user_agent`/`details_json` như INSERT cũ. Đã áp phương án (a): bỏ `userAgent` khỏi INSERT, đổi `ip`→`ip_address`, `details_json`→`detail`. Lỗi `IConnectionPool` còn lại là pre-existing baseline (đã thấy từ trước khi tôi bắt đầu sửa bất kỳ file nào trong phiên này). **Blocker cho TASK-HLD-023 đã giải quyết.**)
**Bug refs:** BUG-BE-HLD-014, BUG-BE-HLD-015 (prerequisite phát hiện trong lúc điều tra 2 bug này)
**Solution ref:** [SOLUTION-ai-provider-exact.md](../solutions/SOLUTION-ai-provider-exact.md) §2.6, §6.1
**Depends on:** Không — nhưng là **prerequisite bắt buộc** cho TASK-HLD-023

---

## Mục tiêu

`AuditLogger.log()` hiện tại insert vào các cột `(action, user_id, user_email, ip, user_agent, details_json, created_at)` — nhưng bảng `orca_audit_log` thật (tạo bởi migration `0005_add_auth_schema.ts`, lines 62-70) chỉ có các cột `id, created_at, user_id, user_email, action, detail, ip_address` — **không có `ip`, `user_agent`, `details_json`**.

Gọi `AuditLogger.log()` ở trạng thái hiện tại sẽ ném lỗi SQL `"no such column: ip"` khi thực thi thật. Lỗi này bị `.catch()` nuốt và chỉ log ra console (thiết kế "audit is best-effort", non-fatal) — nghĩa là **audit log âm thầm không ghi được gì**, không có exception nào lộ ra ngoài để phát hiện sớm.

Đây là bug tồn tại độc lập, không thuộc phạm vi BUG-BE-HLD-014/015, nhưng **bắt buộc phải sửa trước** vì TASK-HLD-023 (key rotation) thêm nhiều audit log call mới (`aiProvider.create`, `update`, `delete`, `writeCredential`, `rotateKey.started/completed/failed`) — nếu không sửa cột trước, toàn bộ audit trail mới đó cũng sẽ lỗi im lặng giống hệt.

## File cần sửa/tạo

```
backend/src/main/auth/audit-logger.ts   (sửa)
backend/src/main/auth/audit-logger.test.ts   (sửa/thêm test)
```

## Thay đổi cụ thể

### Code hiện tại có vấn đề

**File:** `backend/src/main/auth/audit-logger.ts` — Lines 33-64

```typescript
export class AuditLogger {
  constructor(private readonly pool: IConnectionPool) {}

  async log(entry: AuditEntry): Promise<void> {
    const now = Date.now()
    await this.pool.withConnection((db) =>
      db.query(
        `INSERT INTO orca_audit_log
           (action, user_id, user_email, ip, user_agent, details_json, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
        [entry.action, entry.userId, entry.userEmail, entry.ip, entry.userAgent ?? '', JSON.stringify(entry.details ?? {}), now]
      )
    ).catch((err: unknown) => {
      console.error('[AuditLogger] Write failed (non-fatal):', err)
    })
  }
}
```

### Schema thật (migration 0005_add_auth_schema.ts, lines 62-70) — tham khảo để đối chiếu

Bảng `orca_audit_log` thật có cột: `id, created_at, user_id, user_email, action, detail, ip_address`.

Sự khác biệt:
| Cột trong INSERT hiện tại | Cột thật trong bảng | Ghi chú |
|---|---|---|
| `ip` | `ip_address` | đổi tên |
| `user_agent` | *(không tồn tại)* | phải loại bỏ khỏi câu INSERT, hoặc ALTER thêm cột |
| `details_json` | `detail` | đổi tên |

### Giải pháp — sửa câu SQL trong `audit-logger.ts` để khớp cột thật (khuyến nghị, không cần migration mới)

```typescript
export class AuditLogger {
  constructor(private readonly pool: IConnectionPool) {}

  async log(entry: AuditEntry): Promise<void> {
    const now = Date.now()
    await this.pool.withConnection((db) =>
      db.query(
        // FIX BUG-BE-HLD-014/015 prerequisite: match the real orca_audit_log
        // schema from migration 0005 (id, created_at, user_id, user_email,
        // action, detail, ip_address) — the previous INSERT referenced
        // columns (ip, user_agent, details_json) that never existed, so every
        // call silently failed inside the .catch() below.
        `INSERT INTO orca_audit_log
           (action, user_id, user_email, ip_address, detail, created_at)
         VALUES (?, ?, ?, ?, ?, ?)`,
        [entry.action, entry.userId, entry.userEmail, entry.ip, JSON.stringify(entry.details ?? {}), now]
      )
    ).catch((err: unknown) => {
      console.error('[AuditLogger] Write failed (non-fatal):', err)
    })
  }
}
```

**Quyết định về `userAgent`:** vì bảng thật không có cột `user_agent`, và `AuditEntry.userAgent` là optional, có 2 lựa chọn — chọn lựa chọn (a) trừ khi có yêu cầu khác từ tech lead:

- **(a) Bỏ `userAgent` khỏi câu INSERT** (như code ở trên) — đơn giản nhất, không cần migration. Field `userAgent` trên `AuditEntry` type vẫn giữ nguyên (không breaking cho caller), chỉ đơn giản không được ghi vào DB. Ghi chú comment rõ lý do.
- **(b) Thêm migration mới `ALTER TABLE orca_audit_log ADD COLUMN user_agent TEXT`** — nếu tech lead xác nhận cần giữ lại thông tin user agent cho audit trail. Không nằm trong phạm vi task này trừ khi được yêu cầu — nếu chọn hướng này, tạo task riêng vì đụng vào schema migration cần review kỹ hơn.

## Verification

```bash
cd /opt/repos/orca
pnpm --filter backend tsc --noEmit
pnpm --filter backend test audit-logger

# Xác nhận không còn tham chiếu cột sai
grep -n "details_json\|user_agent" backend/src/main/auth/audit-logger.ts
# Expected: 0 kết quả (hoặc chỉ còn trong comment giải thích)

# Test thủ công: gọi AuditLogger.log() thật và xác nhận có insert thành công (không rơi vào catch)
```

Test case cần thêm/sửa trong `audit-logger.test.ts`:

1. `log()` với entry đầy đủ field → insert thành công, không throw, không log `console.error`.
2. Đọc lại row vừa insert từ `orca_audit_log` → xác nhận `action`, `user_id`, `user_email`, `ip_address`, `detail` (JSON parse lại đúng `details`), `created_at` khớp đúng giá trị đã truyền.
3. `log()` với `entry.details` là `undefined` → cột `detail` lưu `'{}'`.
4. Regression: chạy test với schema thật từ migration 0005 (không mock câu SQL) để bắt lại chính xác lỗi "no such column" nếu tái phát trong tương lai.

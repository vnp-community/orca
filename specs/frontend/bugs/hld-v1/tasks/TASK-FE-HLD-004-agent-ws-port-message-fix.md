# TASK-FE-HLD-004 — Sửa thông báo agent dùng `ORCA_HTTP_PORT` thay vì literal

**Solution:** [SOLUTION-FE-HLD-004](../solutions/SOLUTION-FE-HLD-004-agent-ws-port-message.md)
**Bug:** [BUG-FE-HLD-004](../BUG-FE-HLD-004-agent-ws-hardcoded-port-message.md)
**File:** `frontend/src/main/dev-server/agent-ws-server.ts`
**Estimated:** 10 phút
**Status:** ✅ DONE — 2026-08-09

---

## Mục tiêu

Sửa thông báo "Configure your agent with: …" đọc đúng `ORCA_HTTP_PORT` thay vì hardcode `6768`.

---

## Context

```bash
grep -n "6768\|AGENT_WS_PATH" frontend/src/main/dev-server/agent-ws-server.ts | head -10
grep -n "ORCA_HTTP_PORT" frontend/src/main/dev-server/dev-server-relay-bridge.ts frontend/src/main/dev-server/dev-server-manager.ts
```

Đối chứng pattern đúng (đã dùng ở 2 file cùng thư mục):
```typescript
const port = process.env['ORCA_HTTP_PORT'] ?? '6768'
```

---

## Thay đổi cần thực hiện

**File:** `frontend/src/main/dev-server/agent-ws-server.ts` — dòng ~103

**TÌM:**
```typescript
`Configure your agent with: ws://<orca-host>:6768${AGENT_WS_PATH}`
```

**THAY BẰNG:**
```typescript
const port = process.env['ORCA_HTTP_PORT'] ?? '6768'
`Configure your agent with: ws://<orca-host>:${port}${AGENT_WS_PATH}`
```

> [!IMPORTANT]
> Chỉ sửa đúng chuỗi thông báo này — không đổi port thật server đang lắng nghe (nằm ở chỗ khác), đây chỉ là sửa chuỗi hiển thị cho khớp với port thật.

---

## Verify

```bash
pnpm --filter frontend tsc --noEmit

# Test thủ công: set ORCA_HTTP_PORT khác default, xác nhận thông báo hiển thị đúng
ORCA_HTTP_PORT=9999 node -e "..." # hoặc chạy unit test tương ứng nếu có sẵn harness
```

---

## Definition of Done

- [x] Thông báo đọc `process.env['ORCA_HTTP_PORT'] ?? '6768'`, không còn literal `6768` cứng trong template string
- [~] `pnpm tsc --noEmit` — **không chạy được ở mức toàn package**: `tsconfig.json` hiện thiếu `include` cho phần lớn `src/main/**` (lỗi `TS6307`), một vấn đề có sẵn từ trước khi tách `frontend/` khỏi monorepo, không liên quan tới thay đổi của task này. Không sửa `tsconfig.json` trong task này (ngoài phạm vi, rủi ro cao, ảnh hưởng toàn package) — đã ghi nhận riêng, xem `NOTES.md` trong thư mục này.
- [x] Thêm test mới `agent-ws-server.test.ts` (2 case: default port 6768, override `ORCA_HTTP_PORT=9999`) — cả 2 **pass** qua `pnpm test -- agent-ws-server`

## Kết quả thực thi

- **File sửa:** `frontend/src/main/dev-server/agent-ws-server.ts` — đọc `process.env['ORCA_HTTP_PORT'] ?? '6768'` thay vì literal.
- **File mới:** `frontend/src/main/dev-server/agent-ws-server.test.ts` — 2 test case, cả 2 pass.
- **Ghi chú hạ tầng:** `frontend/` chưa có `test` script/vitest config riêng (package "isolated copy, split from monorepo" — xem `package.json` description) — đã bổ sung `frontend/config/vitest.config.ts` (copy từ `desktop/config/vitest.config.ts`) + script `test`/`test:watch` trong `package.json` để có thể chạy verify cho toàn bộ 14 task trong series này.

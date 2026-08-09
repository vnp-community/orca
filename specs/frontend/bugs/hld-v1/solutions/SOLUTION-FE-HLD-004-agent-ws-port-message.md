# SOLUTION: BUG-FE-HLD-004 — Thông báo cấu hình agent hardcode port 6768

**Source-verified:** ✅ Dựa trên source code thực tế
**TDD tham chiếu:** không có mục riêng — đây là fix cơ học, đối chứng bằng chính 2 file cùng thư mục (`dev-server-relay-bridge.ts`, `dev-server-manager.ts`) đã áp dụng đúng pattern.

---

## Root cause

```ts
// agent-ws-server.ts:103
`ws://<orca-host>:6768${AGENT_WS_PATH}`   // literal, không đọc env
```

## Fix

```diff
- `Configure your agent with: ws://<orca-host>:6768${AGENT_WS_PATH}`
+ const port = process.env['ORCA_HTTP_PORT'] ?? '6768'
+ `Configure your agent with: ws://<orca-host>:${port}${AGENT_WS_PATH}`
```

Đúng pattern đã dùng ở `dev-server-relay-bridge.ts:322` và `dev-server-manager.ts:408` trong cùng thư mục — không cần thiết kế mới, chỉ copy pattern.

## Test cần thêm

- `agent-ws-server.test.ts`: set `process.env['ORCA_HTTP_PORT'] = '9999'`, xác nhận thông báo hiển thị đúng `:9999`, không còn `:6768` cứng.

## Tóm tắt thay đổi

| File | Thay đổi |
|---|---|
| `main/dev-server/agent-ws-server.ts:103` | Đọc `process.env['ORCA_HTTP_PORT'] ?? '6768'` thay vì literal `6768` |

# BUG-FE-HLD-004 — Thông báo hướng dẫn cấu hình agent hardcode port 6768, bỏ qua `ORCA_HTTP_PORT`

**Mức độ:** 🟡 Medium
**Status:** 🔴 Open
**Module:** `frontend/src/main/dev-server/agent-ws-server.ts`
**Phát hiện:** 2026-08-08 (audit `frontend/` code vs thiết kế — `audit/frontend/02-platform-abstraction-and-coding-standards.md` §3)

---

## Mô tả

Chuẩn "Zero Hardcode" (`docs/features/README.md`) yêu cầu config qua env var, không hardcode trong source. Phần lớn `frontend/src/main` tuân thủ đúng — dùng pattern `process.env['ORCA_HTTP_PORT'] ?? '6768'` (default có fallback, xem `dev-server-relay-bridge.ts:322`, `dev-server-manager.ts:408`).

Nhưng `agent-ws-server.ts:103` build chuỗi hướng dẫn hiển thị cho operator ("Configure your agent with: …") bằng literal cứng:

```ts
`ws://<orca-host>:6768${AGENT_WS_PATH}`
```

— không đọc `process.env['ORCA_HTTP_PORT']` như các nơi khác trong cùng thư mục.

## Hậu quả

Nếu operator đã override `ORCA_HTTP_PORT` (ví dụ chạy nhiều instance Orca trên cùng host, hoặc port 6768 bị chiếm), thông báo hiển thị cho họ **sai port thật sự đang lắng nghe** — operator làm theo hướng dẫn sẽ cấu hình agent trỏ sai port, agent không connect được, và thông báo lỗi không gợi ý đúng nguyên nhân.

Mức độ thấp hơn 2 bug bảo mật (FE-HLD-001/002) vì đây chỉ là hiển thị sai, không phải lỗ hổng bảo mật hay hỏng chức năng chính — nhưng gây khó debug cho operator, đúng loại lỗi "Zero Hardcode" được đặt ra để tránh.

## Bằng chứng

```
agent-ws-server.ts:103            → literal 'ws://<orca-host>:6768${AGENT_WS_PATH}', không đọc env
dev-server-relay-bridge.ts:322    → đối chứng pattern đúng: process.env['ORCA_HTTP_PORT'] ?? '6768'
dev-server-manager.ts:408         → đối chứng pattern đúng, cùng file/domain
```

## Đề xuất fix

Sửa chuỗi thông báo dùng cùng pattern default-with-override đã có sẵn trong cùng thư mục:

```ts
const port = process.env['ORCA_HTTP_PORT'] ?? '6768'
`ws://<orca-host>:${port}${AGENT_WS_PATH}`
```

## Tham khảo

- Audit: `audit/frontend/02-platform-abstraction-and-coding-standards.md` §3

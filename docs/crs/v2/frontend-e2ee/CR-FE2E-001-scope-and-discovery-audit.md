# CR-FE2E-001 — Scope & Discovery Audit

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-FE2E-001 |
| **Tên** | Xác định phạm vi & audit toàn bộ nơi dùng E2EE pairing ở frontend |
| **Loại** | Discovery / Pre-work |
| **Priority** | P0 — Blocker cho các CR sau |
| **Phiên bản** | v5.1 |
| **Ngày tạo** | 2026-08-08 |
| **Trạng thái** | Proposed |
| **Phụ thuộc** | — |
| **Tác động HLD** | Không — chỉ audit |

---

## 1. Mục tiêu

Trước khi sửa bất kỳ dòng code nào, xác nhận **chính xác** những gì phụ thuộc vào `WebRuntimeClient`/E2EE pairing ở `frontend/`, để CR-FE2E-002/003 không phá use case B (Desktop Pair Code sharing) hoặc use case C (Mobile Companion qua backend).

## 2. Việc cần làm

### 2.1 Xác nhận runtime branch trong `main.tsx`

File: [frontend/src/renderer/src/web/main.tsx](../../../../frontend/src/renderer/src/web/main.tsx)

```ts
fetch('/auth/config')
  .then((res) => {
    if (res.ok) void bootstrapWebApp()          // ← use case A (multi-user backend)
    else renderOriginalPairCodeApp()             // ← use case B (Desktop pair-code sharing)
  })
  .catch(() => renderOriginalPairCodeApp())
```

- [ ] Xác nhận `/auth/config` **luôn** trả `200` khi `frontend` được `backend/` (multi-user Web Server) serve — grep route trong `backend/src/main` (`GET /auth/config`).
- [ ] Xác nhận **không có deployment nào khác** serve `frontend` mà vẫn trả 200 cho `/auth/config` nhưng lại không enforce login — nếu có, CR-FE2E-002 sẽ phá nó (không có session cookie → mất kết nối hoàn toàn).
- [ ] Xác nhận `renderOriginalPairCodeApp()` chỉ được dùng khi *không* có `backend` multi-user phía sau (Desktop app tự serve, hoặc bare relay) — không phải một cấu hình phụ của `backend`.

### 2.2 Inventory đầy đủ file liên quan (đã audit — xác nhận lại trước khi sửa)

```
frontend/src/renderer/src/web/
├── AddInstanceForm.tsx          — UI: thêm 1 Orca instance (dùng pairing offer)
├── OrcaInstanceSwitcher.tsx     — UI: chuyển giữa nhiều instance đã pair
├── WebConnect.tsx               — UI pairing chính (dùng bởi use case B)
├── web-pairing.ts               — parse orca://pair, decode offer, decide startup kind
├── web-e2ee.ts                  — Curve25519/NaCl: encrypt/decrypt/deriveSharedKey
├── web-runtime-client.ts        — WebRuntimeClient (WS + E2EE handshake)
├── web-runtime-environment.ts   — LƯU Ý: chứa CẢ hàm pairing lẫn hàm session-only
│                                   (createSessionWebRuntimeEnvironment) — không xoá cả file
├── web-session-client.ts        — WebSessionClient (cookie-auth) — KHÔNG đụng, đây là đích đến
├── login/LoginPage.tsx           — render PairCodeFallback (mục tiêu CR-FE2E-002)
├── login/PairCodeFallback.tsx   — entry point pairing trong multi-user login (mục tiêu CR-FE2E-002)
├── main.tsx                     — branch /auth/config (giữ nguyên, chỉ code-split ở CR-FE2E-003)
├── main-web-bootstrap.tsx       — bootstrapWebApp(), dùng bởi use case A
└── web-preload-api.ts           — getRuntimeClientForEnvironment(): chọn WebSessionClient
                                    hay WebRuntimeClient theo StoredWebRuntimeEnvironment
```

Tham chiếu ngoài `web/`: `frontend/src/renderer/src/runtime/runtime-file-client.ts` (1 comment mô tả hành vi cleanup của `WebRuntimeClient`, không phải import — không cần sửa).

- [ ] Chạy lại grep sau khi bắt đầu implement để bắt các import mới phát sinh:
  ```
  grep -rl "WebRuntimeClient\|WebPairingOffer\|parseWebPairingInput\|from '\.\./web-pairing'\|from '\./web-pairing'\|from '\./web-e2ee'\|StoredWebRuntimeEnvironment\|getPreferredWebPairingOffer\|AddInstanceForm\|OrcaInstanceSwitcher\|PairCodeFallback" frontend/src --include="*.ts" --include="*.tsx"
  ```

### 2.3 Xác nhận backend endpoint dùng chung mobile + browser

File: [backend/src/main/runtime/rpc/ws-transport.ts:48](../../../../backend/src/main/runtime/rpc/ws-transport.ts#L48) — comment xác nhận "the pairing server can also serve the browser client". 

- [ ] Xác nhận việc bỏ browser khỏi client của endpoint này **không** yêu cầu sửa backend (endpoint là protocol-agnostic, không phân biệt caller là mobile hay browser ở tầng transport).
- [ ] Ghi nhận rõ: backend giữ nguyên 100% — không có CR nào trong series này chạm `backend/src/main/runtime/*`.

### 2.4 Câu hỏi mở cần trả lời trước khi làm CR-FE2E-004

- [ ] "Share this Orca server → New Link" (nhắc tới trong [WebConnect.tsx:45](../../../../frontend/src/renderer/src/web/WebConnect.tsx#L45), dẫn tới Settings → Runtime Environments) — tính năng này có generate pairing link để chia sẻ **một session `backend` multi-user đang chạy** hay chỉ dùng cho Desktop instance? Nếu có nhánh nào của tính năng này chạy trong use case A (multi-user), CR-FE2E-002/003 phải né nó — xem CR-FE2E-004.

## 3. Acceptance Criteria

- [ ] Có bảng inventory đầy đủ (mục 2.2) được review bởi ít nhất 1 người hiểu cả `frontend` và `backend`.
- [ ] Có câu trả lời dứt khoát cho câu hỏi 2.4 trước khi CR-FE2E-004 bắt đầu.
- [ ] Không phát sinh thay đổi code ở CR này — chỉ tài liệu + xác nhận.

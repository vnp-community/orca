# CR v2 — Bỏ WebSocket RPC E2EE Pairing khỏi Browser (Frontend)

**Phiên bản:** v5.1 (cleanup trên nền v5.0)
**Ngày:** 2026-08-08
**Trạng thái:** ✅ Implemented — 2026-08-09 (5/5 CR, 11/11 task — xem `specs/frontend/crs/frontend-e2ee/tasks/TASK-FE2E-011-full-test-matrix-and-doc-updates.md` cho kết quả test matrix + 1 giới hạn còn tồn đọng ở CR-FE2E-003 AC-1)
**Kiến trúc liên quan:** [web-server-architecture.md](../../../hld/web-server-architecture.md) §5, [C2-containers.md](../../../hld/v1/C2-containers.md) (Communication Matrix), [F03-mobile-companion.md](../../../features/F03-mobile-companion.md), [F22-web-server-mode.md](../../../features/F22-web-server-mode.md), [F23-multi-user-auth.md](../../../features/F23-multi-user-auth.md)

---

## Bối cảnh

`frontend` (browser) hiện có **2 cách xác thực kết nối RPC** tới server (xem CR series [restructure_v1](../../v1/restructure_v1/README.md) và `CR-LOGIN-001`/`TASK-FE-007` đã triển khai trước đó):

1. **Session cookie** (`WebSessionClient` / `WebSocketRpcClient`) — dùng khi có `/auth/local` session hợp lệ. Đây là đường chính, **bắt buộc** trong Orca Web Server multi-user mode (F22/F23).
2. **E2EE pairing** (`WebRuntimeClient`, Curve25519/NaCl key exchange, xác thực bằng `deviceToken`) — dùng khi không có session cookie để kế thừa.

**Phát hiện quan trọng khi audit** ([frontend/src/renderer/src/web/main.tsx:87-101](../../../../frontend/src/renderer/src/web/main.tsx#L87-L101)): file entry point thật sự của web bundle (`web-index.html` → `src/web/main.tsx`) tự probe `GET /auth/config` lúc khởi động để quyết định dùng flow nào:

```
GET /auth/config
  ├── 200 OK  → đang chạy sau Orca Web Server multi-user (backend/)
  │             → bootstrapWebApp() — SSO/local login, session cookie luôn có
  │             → E2EE pairing chỉ còn lại 1 lối vào: PairCodeFallback trên LoginPage
  │               (comment code: "backward-compat", KHÔNG PHẢI đường chính)
  │
  └── 404 / lỗi mạng → "Desktop Pair Code sharing mode"
                → renderOriginalPairCodeApp() — CHỈ CÓ E2EE pairing (WebConnect),
                  không có server nào để login vào
```

**Nghĩa là `WebRuntimeClient`/E2EE pairing không phải một tính năng duy nhất — nó là transport của 2 use case khác nhau dùng chung code:**

| Use case | Server phía sau | Có cookie session không? | Có bị CR series này đụng tới? |
|---|---|---|---|
| A. Browser → Orca Web Server multi-user (`backend/`, kịch bản `deploy/`) | `backend` | Có, bắt buộc (F23) | ✅ Có — đây là phần bị bỏ |
| B. Browser pair trực tiếp vào 1 Desktop app / bare relay không chạy `backend` multi-user | Desktop app hoặc relay đơn lẻ | Không, không có khái niệm login | ❌ Không — **phải giữ nguyên** |
| C. Mobile Companion (F03) pair vào `backend` | `backend` | Không (native app, không có cookie) | ❌ Không — **backend giữ nguyên**, đây là lý do BE không đổi |

→ **Yêu cầu "bỏ ở browser mà không bỏ ở backend, không ảnh hưởng tính năng browser" chỉ khả thi nếu scope đúng vào use case A.** Xoá thẳng `web-runtime-client.ts`/`web-e2ee.ts`/`web-pairing.ts`/`WebConnect.tsx` sẽ phá use case B (không có cách nào khác để vào Orca trong kịch bản đó). CR series này **không xoá code dùng chung** — thay vào đó tách để use case A không còn tải/gọi tới E2EE pairing, còn use case B chạy y nguyên.

---

## Change Requests

| CR ID | Tên | Priority | Phụ thuộc |
|-------|-----|---------|-----------|
| [CR-FE2E-001](./CR-FE2E-001-scope-and-discovery-audit.md) | Scope & Discovery Audit | P0 | — |
| [CR-FE2E-002](./CR-FE2E-002-remove-paircode-fallback-from-login.md) | Bỏ PairCodeFallback khỏi LoginPage (multi-user path) | P0 | CR-FE2E-001 |
| [CR-FE2E-003](./CR-FE2E-003-lazy-split-pairing-bundle.md) | Code-split E2EE Pairing khỏi bundle multi-user | P1 | CR-FE2E-001, CR-FE2E-002 |
| [CR-FE2E-004](./CR-FE2E-004-share-link-decision.md) | Quyết định cho "Share this Orca server" (Runtime Environments) | P0 | CR-FE2E-001 |
| [CR-FE2E-005](./CR-FE2E-005-test-and-rollout-plan.md) | Test Matrix & Rollout | P0 | CR-FE2E-002, CR-FE2E-003, CR-FE2E-004 |

---

## Nguyên tắc thiết kế

1. **Backend không đổi 1 dòng nào.** `backend/src/main/runtime/{mobile-pairing-files,e2ee-keypair}.ts`, `backend/src/main/runtime/rpc/{e2ee-channel,e2ee-crypto,ws-transport}.ts` giữ nguyên — Mobile Companion (F03, P0) phụ thuộc hoàn toàn vào các file này.
2. **Không xoá code dùng chung giữa use case A và B.** `web-runtime-client.ts`, `web-e2ee.ts`, `web-pairing.ts`, `web-runtime-environment.ts`, `WebConnect.tsx`, `AddInstanceForm.tsx`, `OrcaInstanceSwitcher.tsx` tiếp tục tồn tại nguyên vẹn cho use case B (`renderOriginalPairCodeApp()`).
3. **"Bỏ" nghĩa là: không tải, không gọi, không hiển thị** trong nhánh multi-user (use case A) — bằng cách bỏ entry point (`PairCodeFallback`) và code-split để bundle multi-user không kéo theo `nacl`/E2EE crypto.
4. **Mọi thay đổi phải giữ use case B chạy y nguyên** — có test riêng xác nhận (CR-FE2E-005).
5. **Không tự ý quyết định số phận của "Share this Orca server"** (Settings → Runtime Environments) nếu chưa xác nhận nó có tách biệt khỏi login flow hay không — xem CR-FE2E-004.

---

## Kết quả kỳ vọng

- Người dùng multi-user Web Server: **không còn thấy** ô "Pairing URL or Code" trên trang login; bundle JS không tải `TweetNaCl`/E2EE code khi vào qua `/auth/config` 200.
- Người dùng Desktop Pair Code sharing (use case B) và Mobile Companion (use case C, F03): **không có thay đổi hành vi nào**, không cần test lại toàn bộ — chỉ cần regression smoke test theo CR-FE2E-005.
- `backend/` — 0 file thay đổi.

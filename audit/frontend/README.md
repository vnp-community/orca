# Audit — Frontend Code vs Thiết kế (Docs)

**Phạm vi:** `frontend/` (toàn bộ — `src/main`, `src/renderer`, `src/platform`, `src/preload`, `src/shared`)
**Đối chiếu với:** `docs/hld/web-server-architecture.md`, `docs/hld/v1/security.md`, `docs/hld/v1/C2-containers.md`, `docs/features/README.md` ("Coding Standards cho v5.0"), `AGENTS.md`
**Ngày:** 2026-08-08
**Phương pháp:** 4 agent audit độc lập (đọc code thật, đối chiếu từng câu doc, trích dẫn file:line cụ thể — không suy đoán) + kết quả module-inventory audit đã làm trong phiên trước ([xem mục 5](#5-module-inventory-đã-audit-ở-phiên-trước)).
**Quy mô codebase:** ~825k dòng source (3,865 file) + ~411k dòng test (1,871 file).

---

## Tóm tắt điều hành

Frontend nhìn chung **tuân thủ tốt** các nguyên tắc bảo mật cốt lõi (renderer sandbox, Zero Mock, E2EE dùng đúng thuật toán đã tài liệu hoá) và có kỷ luật code tốt (TODO/FIXME thấp, ít test bị skip, `any` type thấp). Tuy nhiên audit phát hiện **5 vấn đề nghiêm trọng cần xử lý** và một lượng lớn **doc bị lỗi thời (stale)** — đặc biệt ở tầng kiến trúc transport và routing, nơi doc mô tả một hệ thống không còn tồn tại trong code.

### Xếp hạng mức độ nghiêm trọng

| # | Mức độ | Phát hiện | Chi tiết |
|---|--------|-----------|----------|
| 1 | 🔴 **Critical (policy)** | **240 file** disable rule `max-lines` — vi phạm trực tiếp quy định "never" trong `AGENTS.md:15` | [04](./04-code-health-and-standards.md#1-max-lines-vi-phạm-chính-sách-agentsmd) |
| 2 | 🔴 **High (security)** | `deviceToken` (bearer credential cho E2EE pairing) lưu **plaintext trong `localStorage`** — trái với thiết kế "in-memory, ephemeral" ở `security.md` §3 | [01](./01-security-conformance.md#1-devicetoken-lưu-plaintext-trong-localstorage) |
| 3 | 🔴 **High (security)** | Luồng `git.push` streaming dùng `sessionStorage` Bearer token thay vì cookie `HttpOnly` đã tài liệu hoá — code mới, chưa commit, không có nơi nào set giá trị token (khả năng đang gửi Bearer rỗng) | [01](./01-security-conformance.md#2-git-push-streaming-dùng-sessionstorage-bearer-token) |
| 4 | 🟠 **High (architecture)** | `IPlatformServices` chỉ có adapter cho web — không có adapter Electron; **72 file** trong `src/main` import `electron` trực tiếp, vi phạm chuẩn v5.0 #3 | [02](./02-platform-abstraction-and-coding-standards.md#1-iplatformservices-chỉ-tồn-tại-cho-web) |
| 5 | 🟠 **Medium (security)** | Key derivation của `WebCredentialStore` **không** đưa `userId` vào, trái với tuyên bố "Per-user key từ userId + server secret" ở `security.md` §11 | [01](./01-security-conformance.md#3-credential-key-derivation-không-theo-userid) |
| 6 | 🟡 **Medium (doc drift)** | `web-server-architecture.md` §5.1 mô tả sai hoàn toàn wire protocol của `WebSocketRpcClient` — protocol nhị phân 13-byte trong doc thực ra thuộc về SSH relay multiplexer (`main/ssh/relay-protocol.ts`), một subsystem khác | [03](./03-hld-doc-drift.md#1-51-wire-protocol--sai-hoàn-toàn) |
| 7 | 🟡 **Medium (doc drift)** | §12 Routing & §11 Admin SPA mô tả react-router/URL routing — thực tế **không có router nào**, chỉ state-boolean branching; Admin SPA code tự ghi rõ "no react-router-dom" | [03](./03-hld-doc-drift.md#2-12-routing--không-có-router-nào-tồn-tại) |
| 8 | 🟢 **Low (test gap)** | 5 module provider mới phát hiện (`kimi`, `minimax`, `openclaude`, `command-code`, `droid`) + `sparse/` — **0 test** | [04](./04-code-health-and-standards.md#3-test-coverage-gap-ở-5-module-provider-mới) |

---

## Cấu trúc report

| File | Nội dung |
|---|---|
| [01-security-conformance.md](./01-security-conformance.md) | Đối chiếu với `docs/hld/v1/security.md` — session/cookie, E2EE, credential storage, renderer sandbox, admin audit log |
| [02-platform-abstraction-and-coding-standards.md](./02-platform-abstraction-and-coding-standards.md) | `IPlatformServices`, Zero Mock, Zero Hardcode, renderer-sandbox import hygiene |
| [03-hld-doc-drift.md](./03-hld-doc-drift.md) | Đối chiếu `web-server-architecture.md` §5, §9–§12, §14 với cấu trúc component/routing thật |
| [04-code-health-and-standards.md](./04-code-health-and-standards.md) | `max-lines` policy violation, test coverage, TODO/FIXME, `any` usage, skipped tests |
| [05-module-inventory-recap.md](./05-module-inventory-recap.md) | Recap audit module thừa/thiếu docs đã làm ở phiên trước (F04/F01/F41/F42 đã được bổ sung) |
| [recommendations.md](./recommendations.md) | Danh sách hành động ưu tiên, gộp cả 4 mảng trên |

---

## Những gì ĐÃ đúng thiết kế (không cần hành động)

Để tránh report chỉ toàn điểm trừ — các phần sau đã audit kỹ và **khớp hoàn toàn với doc**, không có vấn đề:

- **Renderer sandbox**: không có file production nào trong `src/renderer/src/**` import `electron`/`node:fs`/`node:child_process` trực tiếp — 100% qua `window.api`.
- **E2EE pairing crypto**: `web-e2ee.ts` dùng đúng TweetNaCl (`nacl.box`, Curve25519, XSalsa20-Poly1305, nonce 24 byte) như `security.md` §4 mô tả.
- **Zero Mock**: không tìm thấy mock data nào lẫn vào production code.
- **Credential form UI**: `CredentialInputForm.tsx` giữ token trong React state, clear ngay sau khi save, gửi qua đúng kênh RPC.
- **Admin audit log**: `AuditPage` gọi đúng `GET /admin/api/audit`, không có API xoá — khớp nguyên tắc append-only ở §9.
- **`.orig`/`.rej` merge artifact**: không có file nào còn sót lại trong `frontend/src`.
- **TODO/FIXME, `any` usage, skipped tests**: mật độ thấp so với quy mô ~825k dòng — không phải vấn đề.

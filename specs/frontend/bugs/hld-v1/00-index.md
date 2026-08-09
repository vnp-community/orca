# Bug Reports — Frontend vs HLD v1 (Design Conformance)

**Module:** `frontend/` (toàn bộ)
**Phát hiện:** 2026-08-08
**Nguồn:** [audit/frontend/](../../../../audit/frontend/) — audit đối chiếu code thật với `docs/hld/v1/security.md`, `docs/hld/web-server-architecture.md`, `docs/features/README.md` (Coding Standards v5.0), `AGENTS.md`
**Ngữ cảnh:** Chỉ gồm các lỗi liên quan trực tiếp đến **code** (hành vi/cấu trúc sai khác so với thiết kế đã công bố). Các lỗi thuần tuý "doc viết sai, code đúng" (vd. wire protocol §5.1, routing §12, Admin SPA route table) **không** đưa vào đây — xem `audit/frontend/03-hld-doc-drift.md` và `audit/frontend/recommendations.md` mục 6 để sửa docs riêng.

---

## Danh sách Bugs

| ID | Mức độ | Tiêu đề | Module | Status |
|----|--------|---------|--------|--------|
| [BUG-FE-HLD-001](./BUG-FE-HLD-001-device-token-plaintext-localstorage.md) | 🔴 Critical | `deviceToken` E2EE pairing lưu plaintext trong `localStorage` | `web-runtime-environment.ts`, `web-runtime-client.ts` | ✅ Fixed (XOR wrap-at-rest, xem [tasks/010-011](./tasks/TASK-FE-HLD-010-device-token-crypto-module.md)) |
| [BUG-FE-HLD-002](./BUG-FE-HLD-002-git-push-stream-bearer-token-broken.md) | 🔴 Critical | `git.push` streaming dùng `sessionStorage` Bearer token, không nơi nào set giá trị | `runtime-rpc-stream.ts`, `useGit.ts` | ✅ Fixed (route qua `pushRuntimeGit()` có sẵn, xoá `runtime-rpc-stream.ts` — xem [tasks/003](./tasks/TASK-FE-HLD-003-git-push-usegit-cutover-cleanup.md)) |
| [BUG-FE-HLD-003](./BUG-FE-HLD-003-credential-store-kdf-missing-userid.md) | 🟠 High | `WebCredentialStore` key derivation không đưa `userId` vào — phá "per-user isolation" đã công bố | `main/credentials/web-credential-store.ts` | ✅ Fixed (KDF V3 + lazy migration, xem [tasks/012-013](./tasks/TASK-FE-HLD-012-credential-store-kdf-v3.md)) |
| [BUG-FE-HLD-004](./BUG-FE-HLD-004-agent-ws-hardcoded-port-message.md) | 🟡 Medium | Thông báo cấu hình agent hardcode port 6768, bỏ qua `ORCA_HTTP_PORT` | `main/dev-server/agent-ws-server.ts` | ✅ Fixed |
| [BUG-FE-HLD-005](./BUG-FE-HLD-005-iplatformservices-electron-adapter-missing.md) | 🟠 High | `IPlatformServices` chỉ có adapter web; 72 file `src/main` import `electron` trực tiếp | `src/platform/`, `src/main/**` | ✅ Fixed (làm rõ doc + lint guard 4 module v5.0 — 72 file cũ giữ nguyên, xem [tasks/005-006](./tasks/TASK-FE-HLD-005-iplatformservices-doc-scope.md)) |
| [BUG-FE-HLD-006](./BUG-FE-HLD-006-max-lines-disable-agents-md-violation.md) | 🔴 Critical (policy) | 240 file disable `max-lines`, vi phạm trực tiếp `AGENTS.md:15` | `src/**` (240 file) | ⚠️ Partially Fixed — ratchet khôi phục nhưng **không bảo vệ được `frontend/`** (chưa `git add`), xem [tasks/007](./tasks/TASK-FE-HLD-007-max-lines-baseline-ratchet.md) |
| [BUG-FE-HLD-007](./BUG-FE-HLD-007-gitpanel-conflictpanel-not-implemented.md) | 🟡 Medium | `ConflictPanel` (F39 GitPanel) được doc hoá nhưng chưa implement | `components/workspace/git/` | ⛔ Blocked — chờ quyết định product owner, xem [tasks/014](./tasks/TASK-FE-HLD-014-conflictpanel-scope-decision.md) |

---

## Phân loại theo Priority

### 🔴 Critical — Xử lý ngay, ưu tiên trước khi merge/release
- **BUG-FE-HLD-001**: token bearer pairing lưu plaintext, không thu hồi được, rủi ro XSS trực tiếp
- **BUG-FE-HLD-002**: code **chưa commit**, đang gửi header auth rỗng/sai kiểu cho `git.push` — chặn trước khi vào baseline
- **BUG-FE-HLD-006**: vi phạm chính sách repo tường minh (`AGENTS.md`), 240 điểm — cần xác nhận có ratchet chặn tăng thêm hay không trước khi ưu tiên dọn hàng loạt

### 🟠 High — Xử lý trong 1-2 sprint tới
- **BUG-FE-HLD-003**: lỗ hổng crypto-isolation giữa các user, cần kế hoạch migration khi fix (đổi KDF làm blob cũ không giải mã được)
- **BUG-FE-HLD-005**: cần làm rõ phạm vi chuẩn trước khi quyết định có phải sửa 72 file hay chỉ là hiểu nhầm phạm vi

### 🟡 Medium — Dọn dần, không khẩn cấp
- **BUG-FE-HLD-004**: chỉ ảnh hưởng thông báo hiển thị, không phải chức năng chính
- **BUG-FE-HLD-007**: feature gap thật (F39 đang 🚧), cần xác nhận scope trước khi coi là bug hay backlog item

---

## Tác động tổng hợp

- **Bảo mật**: 3 bug (001, 002, 003) đều liên quan tới cách lưu/gửi credential — nên xử lý theo nhóm vì cùng root cause (thiếu 1 chuẩn nhất quán "không lưu secret ở client-readable storage" áp dụng cho toàn bộ codebase, chỉ mới áp dụng cho nhánh session-cookie).
- **Kiến trúc**: BUG-FE-HLD-005 ảnh hưởng khả năng mở rộng platform trong tương lai, không phải rủi ro vận hành hiện tại.
- **Chính sách**: BUG-FE-HLD-006 là vi phạm rõ ràng nhất, dễ đo lường nhất, nhưng cần xác nhận baseline trước khi coi là việc cần làm gấp.
- **Chức năng thiếu**: BUG-FE-HLD-007 là gap tính năng, không phải regression.

---

## Tham khảo

- [audit/frontend/README.md](../../../../audit/frontend/README.md) — báo cáo audit đầy đủ (bao gồm cả các mục "đã đúng thiết kế" không tạo bug)
- [audit/frontend/01-security-conformance.md](../../../../audit/frontend/01-security-conformance.md)
- [audit/frontend/02-platform-abstraction-and-coding-standards.md](../../../../audit/frontend/02-platform-abstraction-and-coding-standards.md)
- [audit/frontend/03-hld-doc-drift.md](../../../../audit/frontend/03-hld-doc-drift.md)
- [audit/frontend/04-code-health-and-standards.md](../../../../audit/frontend/04-code-health-and-standards.md)
- [CR-FE2E series](../../../../docs/crs/v2/frontend-e2ee/) — liên quan trực tiếp tới BUG-FE-HLD-001

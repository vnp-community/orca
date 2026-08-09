# Tasks — Frontend vs HLD v1 Bug Fixes

**Nguồn:** [solutions/](../solutions/)
**Mục tiêu:** Chia nhỏ mỗi giải pháp thành các tác vụ độc lập, AI có thể thực thi từng cái mà không cần context từ cái khác (mỗi task tự chứa: mục tiêu, file cần đọc trước, thay đổi chính xác, cách verify, Definition of Done).
**Đã thực thi:** 2026-08-09 — 12/14 task hoàn thành, 1 huỷ (kế hoạch sai, gộp vào task khác), 1 blocked (chờ quyết định product). Xem `NOTES.md` cho các phát hiện hạ tầng chung, và mục "Kết quả thực thi"/"Thay đổi so với kế hoạch gốc" trong từng file task cho chi tiết sai khác so với bản thiết kế ban đầu.

---

## Danh sách Tasks

| ID | Solution | Tiêu đề | File mục tiêu | Phụ thuộc | Est. | Kết quả |
|----|----------|---------|----------------|-----------|------|---------|
| [TASK-FE-HLD-001](./TASK-FE-HLD-001-git-push-add-stream-rpc.md) | SOL-002 | ~~Thêm `callRuntimeRpcStream()`~~ | `runtime/runtime-rpc-client.ts` | — | 30' | ❌ Huỷ — backend `git.push` không streaming, kế hoạch sai từ gốc |
| [TASK-FE-HLD-002](./TASK-FE-HLD-002-git-push-runtime-git-client.md) | SOL-002 | ~~Thêm `runtimeGitPush()`~~ | `runtime/runtime-git-client.ts` | — | 20' | ✅ Đã có sẵn — không cần làm |
| [TASK-FE-HLD-003](./TASK-FE-HLD-003-git-push-usegit-cutover-cleanup.md) | SOL-002 | Chuyển `useGit.ts` sang `pushRuntimeGit`, xoá `runtime-rpc-stream.ts` | `hooks/useGit.ts` | — | 20' | ✅ DONE — 9+24 test pass |
| [TASK-FE-HLD-004](./TASK-FE-HLD-004-agent-ws-port-message-fix.md) | SOL-004 | Sửa thông báo agent dùng `ORCA_HTTP_PORT` thay vì literal | `main/dev-server/agent-ws-server.ts` | — | 10' | ✅ DONE — 2 test mới pass |
| [TASK-FE-HLD-005](./TASK-FE-HLD-005-iplatformservices-doc-scope.md) | SOL-005 | Làm rõ phạm vi chuẩn #3 trong docs | `docs/features/README.md` | — | 10' | ✅ DONE |
| [TASK-FE-HLD-006](./TASK-FE-HLD-006-iplatformservices-lint-guard.md) | SOL-005 | Thêm lint rule chặn `import 'electron'` cho 4 module v5.0 | `.oxlintrc.json` (root) | TASK-FE-HLD-005 | 20' | ✅ DONE — xác nhận bằng test thủ công |
| [TASK-FE-HLD-007](./TASK-FE-HLD-007-max-lines-baseline-ratchet.md) | SOL-006 | Khôi phục baseline + ratchet script (đã tồn tại, bị xoá) | `config/max-lines-baseline.txt`, `config/scripts/check-max-lines-ratchet.mjs` | — | 30' | ⚠️ DONE khác kế hoạch — chỉ bảo vệ được phần đã `git add`, KHÔNG bảo vệ 240 file `frontend/` (chưa track) |
| [TASK-FE-HLD-008](./TASK-FE-HLD-008-max-lines-ci-wiring.md) | SOL-006 | Gắn ratchet check vào `package.json`/CI | `package.json` | TASK-FE-HLD-007 | 15' | ✅ DONE — hoá ra đã sẵn có, chỉ xác nhận |
| [TASK-FE-HLD-009](./TASK-FE-HLD-009-max-lines-agents-md-exception.md) | SOL-006 | Thêm cơ chế ngoại lệ tường minh vào `AGENTS.md` | `AGENTS.md` | TASK-FE-HLD-007 | 10' | ✅ DONE |
| [TASK-FE-HLD-010](./TASK-FE-HLD-010-device-token-crypto-module.md) | SOL-001 | Tạo module mã hoá `deviceToken` tại rest | `web/web-runtime-environment-crypto.ts` (mới) | — | 40' | ⚠️ DONE — đổi AES-GCM→XOR (tránh lan async qua ~15 call site), 4 test pass |
| [TASK-FE-HLD-011](./TASK-FE-HLD-011-device-token-wire-storage.md) | SOL-001 | Wire wrap/unwrap vào lưu/đọc environment + xử lý mất key | `web/web-runtime-environment.ts` | TASK-FE-HLD-010 | 30' | ✅ DONE — phát hiện + fix 1 regression ở `web-preload-api.test.ts` (44→4 fail, 4 còn lại pre-existing) |
| [TASK-FE-HLD-012](./TASK-FE-HLD-012-credential-store-kdf-v3.md) | SOL-003 | Thêm KDF V3 (userId + IV 12 byte) | `main/credentials/web-credential-store.ts` | — | 45' | ✅ DONE — 7 test mới pass (module chưa từng có test) |
| [TASK-FE-HLD-013](./TASK-FE-HLD-013-credential-store-lazy-migration.md) | SOL-003 | Lazy re-encrypt V1/V2 → V3 khi đọc + `migrateToV3()` | `main/credentials/web-credential-store.ts` | TASK-FE-HLD-012 | 30' | ✅ DONE — cùng lúc với 012, chung test file |
| [TASK-FE-HLD-014](./TASK-FE-HLD-014-conflictpanel-scope-decision.md) | SOL-007 | Nêu quyết định scope `ConflictPanel` | `docs/hld/web-server-architecture.md` | — | 15' | ⛔ BLOCKED — chờ product owner, đúng chủ ý (không tự quyết định) |

---

## Thứ Tự Thực Hiện

```
Sprint 1 — Không phụ thuộc, rủi ro thấp, chạy song song (làm trước — theo 00-index.md của solutions/):
  TASK-FE-HLD-004   agent-ws port message fix
  TASK-FE-HLD-005   IPlatformServices doc scope
  TASK-FE-HLD-007   max-lines baseline + ratchet script
  TASK-FE-HLD-014   ConflictPanel scope decision (chờ product owner song song, không block)

Sprint 2 — Sau Sprint 1:
  TASK-FE-HLD-001   callRuntimeRpcStream (nền tảng cho git.push fix)
  TASK-FE-HLD-006   (sau TASK-FE-HLD-005) lint guard electron
  TASK-FE-HLD-008   (sau TASK-FE-HLD-007) CI wiring
  TASK-FE-HLD-009   (sau TASK-FE-HLD-007) AGENTS.md exception mechanism
  TASK-FE-HLD-010   device-token crypto module

Sprint 3 — Sau Sprint 2:
  TASK-FE-HLD-002   (sau TASK-FE-HLD-001) runtimeGitPush
  TASK-FE-HLD-011   (sau TASK-FE-HLD-010) wire device-token storage
  TASK-FE-HLD-012   credential store KDF V3 (độc lập, nhưng để sau vì cần review kỹ nhất)

Sprint 4 — Sau Sprint 3:
  TASK-FE-HLD-003   (sau TASK-FE-HLD-002) useGit cutover + xoá file cũ
  TASK-FE-HLD-013   (sau TASK-FE-HLD-012) lazy migration + test cross-user
```

**Ưu tiên bảo mật/rủi ro cao đi trước trong mỗi sprint có thể chạy song song** — theo đúng thứ tự đã đề xuất ở [solutions/00-index.md](../solutions/00-index.md): BUG-002 (code chưa commit) → BUG-004/005 (rẻ, không rủi ro) → BUG-006 giai đoạn 1 → BUG-001 → BUG-003 → BUG-007 → BUG-006 giai đoạn 2-3.

---

## Format Mỗi Task File

Mỗi TASK file có cấu trúc chuẩn:
1. **Mục tiêu** — một câu ngắn
2. **Context** — file cần đọc trước
3. **Thay đổi cần thực hiện** — đoạn code cần tìm + code thay thế (copy-paste ready), hoặc file mới đầy đủ
4. **Verify** — lệnh kiểm tra kết quả (`pnpm tsc --noEmit`, test, hoặc thao tác thủ công)
5. **Definition of Done** — checklist rõ ràng

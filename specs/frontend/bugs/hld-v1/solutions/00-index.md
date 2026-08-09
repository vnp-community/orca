# Solutions — Frontend vs HLD v1 Bugs

**Nguồn:** [../00-index.md](../00-index.md) (7 bug) đối chiếu với [specs/frontend/tdd/v4](../../../tdd/v4/) và [specs/frontend/tdd/v5](../../../tdd/v5/) — TDD là nguồn xác định "thiết kế đúng" khi có xung đột với HLD, vì TDD mô tả implementation ở mức chi tiết code, còn HLD đôi khi aspirational (xem BUG-FE-HLD-007).

---

## Danh sách Solutions

| Bug | Solution | Loại fix | Cần migration dữ liệu? |
|---|---|---|---|
| [BUG-FE-HLD-001](../BUG-FE-HLD-001-device-token-plaintext-localstorage.md) | [SOLUTION-FE-HLD-001](./SOLUTION-FE-HLD-001-device-token-storage.md) | Mã hoá tại rest bằng session-scoped `CryptoKey` + thu hẹp phạm vi theo CR-FE2E | Không (tự re-pair nếu mất key) |
| [BUG-FE-HLD-002](../BUG-FE-HLD-002-git-push-stream-bearer-token-broken.md) | [SOLUTION-FE-HLD-002](./SOLUTION-FE-HLD-002-git-push-stream-auth.md) | Route qua `RuntimeClientTarget`/`callRuntimeRpcStream` đã thiết kế trong TDD-FE-03 §8, xoá kênh HTTP+Bearer tự chế | Không |
| [BUG-FE-HLD-003](../BUG-FE-HLD-003-credential-store-kdf-missing-userid.md) | [SOLUTION-FE-HLD-003](./SOLUTION-FE-HLD-003-credential-store-kdf.md) | KDF V3 đưa `userId` vào, IV về 12 byte, versioned envelope | **Có** — lazy re-encrypt V1/V2→V3 khi đọc |
| [BUG-FE-HLD-004](../BUG-FE-HLD-004-agent-ws-hardcoded-port-message.md) | [SOLUTION-FE-HLD-004](./SOLUTION-FE-HLD-004-agent-ws-port-message.md) | Đọc `ORCA_HTTP_PORT` thay vì literal | Không |
| [BUG-FE-HLD-005](../BUG-FE-HLD-005-iplatformservices-electron-adapter-missing.md) | [SOLUTION-FE-HLD-005](./SOLUTION-FE-HLD-005-iplatformservices-scope.md) | Sửa doc (`docs/features/README.md`) làm rõ phạm vi chuẩn #3 — **không sửa 72 file code** | Không |
| [BUG-FE-HLD-006](../BUG-FE-HLD-006-max-lines-disable-agents-md-violation.md) | [SOLUTION-FE-HLD-006](./SOLUTION-FE-HLD-006-max-lines-cleanup-plan.md) | Ratchet CI (chặn tăng thêm) + dọn dần theo domain, không big-bang | Không |
| [BUG-FE-HLD-007](../BUG-FE-HLD-007-gitpanel-conflictpanel-not-implemented.md) | [SOLUTION-FE-HLD-007](./SOLUTION-FE-HLD-007-conflictpanel-decision.md) | Quyết định scope trước (product owner) — TDD chưa từng đặc tả tính năng này | Không |

---

## Phát hiện xuyên suốt khi đối chiếu với TDD

1. **BUG-FE-HLD-002 có nguyên nhân gốc rõ ràng hơn sau khi đối chiếu TDD**: `runtime-rpc-stream.ts` là 1 transport phát sinh **ngoài** kiến trúc `RuntimeClientTarget`/`callRuntimeRpc` đã thiết kế trong TDD-FE-03 — không phải lỗi nhỏ lẻ mà là 1 đường code đi vòng qua toàn bộ authentication layer đã có. Fix đúng là xoá và route lại qua transport chuẩn, không phải "vá" auth header.
2. **BUG-FE-HLD-005 đổi kết luận sau khi đọc TDD**: TDD-FE-03 xác nhận `IPlatformServices`/`IRpcClient` từ đầu chỉ thiết kế cho web target (restructure_v1 addendum) — đây là doc-scope issue, không phải code thiếu. Fix rẻ hơn nhiều so với đánh giá ban đầu trong audit (sửa 1 đoạn doc thay vì đánh giá lại 72 file).
3. **BUG-FE-HLD-007 đổi bản chất sau khi đọc TDD**: `ConflictPanel` không tồn tại ở tầng TDD (chỉ có ở HLD) — nghĩa là tính năng này chưa bao giờ được đưa vào kế hoạch implementation chi tiết, khác hẳn 1 tính năng "đã thiết kế nhưng code viết thiếu". Fix đúng là quyết định scope trước, không nhảy thẳng vào code.
4. **BUG-FE-HLD-001/003 không có TDD nào đặc tả chi tiết cơ chế lưu trữ** — solution dựa trên suy luận từ nguyên tắc chung nhất quán đã thấy ở nơi khác trong TDD (`13-ai-provider-ui.md`: "credential NEVER logged or sent in plaintext"), áp dụng đối xứng cho `deviceToken`/`WebCredentialStore`.

## Thứ tự triển khai đề xuất

```
1. BUG-FE-HLD-002 (xoá code chưa commit, rủi ro cao nhất, fix rẻ nhất)
2. BUG-FE-HLD-004 (fix 1 dòng, không rủi ro)
3. BUG-FE-HLD-005 (sửa doc, không rủi ro)
4. BUG-FE-HLD-006 giai đoạn 1 (ratchet CI — rẻ, chặn nợ tăng thêm ngay)
5. BUG-FE-HLD-001 (cần thêm module crypto mới, test kỹ trước khi merge)
6. BUG-FE-HLD-003 (cần kế hoạch migration, review kỹ nhất trong nhóm này)
7. BUG-FE-HLD-007 (chờ quyết định product owner — không block các mục còn lại)
8. BUG-FE-HLD-006 giai đoạn 2-3 (dọn dần, chạy song song lâu dài)
```

---

## Kết quả thực thi thật (2026-08-09)

Đã thực thi qua [tasks/](../tasks/) — 12/14 task DONE, 1 huỷ (TASK-001, kế hoạch sai), 1 blocked (TASK-014, chờ product). **2 solution có thiết kế đổi đáng kể so với bản trên** sau khi đọc code thật:

- **SOLUTION-FE-HLD-001**: AES-GCM (`crypto.subtle`, async) → **XOR** (`crypto.getRandomValues`, đồng bộ) — tránh lan async qua ~15 call site bao gồm 1 module-scope initializer trong `web-preload-api.ts`. Chi tiết: [TASK-FE-HLD-010](../tasks/TASK-FE-HLD-010-device-token-crypto-module.md).
- **SOLUTION-FE-HLD-002**: không xây `callRuntimeRpcStream` mới — backend `git.push` không hề streaming (xác nhận qua đọc `backend/src/main/runtime/rpc/methods/git.ts`), `pushRuntimeGit()` đã có sẵn đúng transport. Chi tiết: [TASK-FE-HLD-001](../tasks/TASK-FE-HLD-001-git-push-add-stream-rpc.md) (huỷ) và [TASK-FE-HLD-003](../tasks/TASK-FE-HLD-003-git-push-usegit-cutover-cleanup.md) (fix thật).
- **SOLUTION-FE-HLD-006**: không viết ratchet script mới — khôi phục từ git history (đã tồn tại, bị xoá trong đợt tái cấu trúc). Phát hiện quan trọng: ratchet dựa vào `git ls-files`, `frontend/` chưa `git add` nên 240 vi phạm **chưa thực sự được bảo vệ**. Chi tiết: [TASK-FE-HLD-007](../tasks/TASK-FE-HLD-007-max-lines-baseline-ratchet.md).

Xem [tasks/00-index.md](../tasks/00-index.md) cho bảng trạng thái đầy đủ và [tasks/NOTES.md](../tasks/NOTES.md) cho danh sách file đã thay đổi + kết quả test cuối cùng (161/165 pass, 4 fail còn lại đều pre-existing).

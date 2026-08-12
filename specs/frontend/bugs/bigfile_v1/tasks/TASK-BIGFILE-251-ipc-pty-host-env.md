# TASK-BIGFILE-251 — Move: `pty-host-env.ts` (phạm vi đã sửa)

**Loại:** Move (cơ học) · **Effort:** M (lớn hơn ước tính gốc của
TASK-BIGFILE-003 vì phạm vi thực tế rộng hơn — 15 hàm helper thuần + 1 hằng
số không liền kề, không chỉ `buildPtyHostEnv` một mình)
**Phụ thuộc:** — · **Status:** ⬜ Chưa làm
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-011-ipc-pty.md` (mục
`pty-host-env.ts` cần cập nhật phạm vi theo task này khi merge)
**Thay thế:** `TASK-BIGFILE-003` (⛔ Superseded — xem
`TASK-BIGFILE-250-ipc-pty-pane-key-registry-investigate.md` § Cụm 3)

## Vì sao task này an toàn (khác Cụm 1/2/4/5 trong TASK-250)

Toàn bộ khối symbol dưới đây là **hàm thuần / type / hằng số** — không đọc
hay ghi `localProvider`, `ptyConnectionProviders`, `ptyOwnership`,
`paneKeyPtyId`, `ptyPaneKey`, `pendingByPaneKey`,
`paneSpawnReservationsByPaneKey`, `ptyPendingGenByPtyId`,
`rendererSerializerByPtyId`, hay `paneKeyTeardownListeners` (xác nhận bằng
`grep` trên đúng đoạn 545–1032 trong `TASK-BIGFILE-250`, 0 kết quả). Chúng
ĐƯỢC GỌI tại nhiều điểm rải rác trong `registerPtyHandlers`, nhưng vì là
hàm thuần, gọi từ nhiều nơi không tạo rủi ro — chỉ cần đổi `import`.

## Input

- File nguồn: `frontend/src/main/ipc/pty.ts` (5,185 dòng, xác nhận lần cuối
  2026-08-12 — đọc lại `wc -l` trước khi bắt đầu, không giả định số dòng cố
  định)
- Đọc đúng 2 đoạn (KHÔNG liền kề):
  - **Dòng 208–217**: hằng số `AGENT_HOOK_RUNTIME_ENV_KEYS` (định nghĩa gần
    đầu file, cạnh state pane-key-registry theo VỊ TRÍ dòng — nhưng KHÔNG
    liên quan logic pane-key. Xác nhận bằng grep: chỉ dùng tại dòng 750 và
    915, cả 2 đều nằm trong đoạn dưới, KHÔNG dùng ở đâu khác trong toàn
    file, kể cả bên trong `registerPtyHandlers`.)
  - **Dòng 545–1032**: toàn bộ khối host-env — type
    `BuildPtyHostEnvOptions` (545–565), các hàm helper (567–821, xem danh
    sách symbol dưới), `buildPtyHostEnv` (834–1032).
- Symbol cần chuyển (20 symbol, đếm lại số thực tế khi đọc — danh sách dưới
  đến từ audit 2026-08-12, có thể lệch nếu file đổi):
  - Hằng số: `AGENT_HOOK_RUNTIME_ENV_KEYS`, `CODEX_HOME_ENV_KEYS`
  - Type: `BuildPtyHostEnvOptions`, `GetSelectedCodexHomePath`, `PrepareClaudeAuth`
  - Hàm: `readInheritedPath`, `firstPathEntry`, `promoteAgentTeamsShimPath`,
    `deleteRequestedEnvKeys`, `shouldSkipCodexHomeEnvForWindowsShell`,
    `getCodexSelectionTargetForPty`, `getCompatibleSelectedCodexHomePath`,
    `readEnvWithProcessFallback`, `resolvePiAgentSourceDir`,
    `resolveScopedPiAgentSourceDir`, `clearPiAgentShadowEnv`,
    `exposePiManagedExtensionEnv`, `mergePtyEnvDeletions`,
    `getInheritedAgentHookEnvKeysToDelete`, `restoreOrStripOverlayEnv`,
    `isMimoLaunchCommand`, `resolveMimocodeSourceHome`,
    `resolveOpenCodeSourceConfigDir`, `buildPtyHostEnv`

## Output

- File mới: `frontend/src/main/ipc/pty-host-env.ts`
- `ipc/pty.ts` xoá 2 đoạn trên, thay bằng:
  ```ts
  export { buildPtyHostEnv, type BuildPtyHostEnvOptions } from './pty-host-env'
  import {
    CODEX_HOME_ENV_KEYS,
    getCodexSelectionTargetForPty,
    getCompatibleSelectedCodexHomePath,
    promoteAgentTeamsShimPath,
    deleteRequestedEnvKeys,
    mergePtyEnvDeletions,
    getInheritedAgentHookEnvKeysToDelete,
    shouldSkipCodexHomeEnvForWindowsShell,
    type GetSelectedCodexHomePath,
    type PrepareClaudeAuth
  } from './pty-host-env'
  ```
  (điều chỉnh danh sách import theo đúng symbol còn được `registerPtyHandlers`
  gọi trực tiếp — xem "Điểm gọi cần cập nhật import" dưới; symbol nào KHÔNG
  còn dùng ở `pty.ts` sau khi 251 xong thì không cần import lại.)

## Điểm gọi cần cập nhật import trong `registerPtyHandlers` (đổi `import`, KHÔNG đổi logic)

Xác nhận bằng grep 2026-08-12 (số dòng có thể lệch nếu file đổi — verify
lại khi thực thi):

| Symbol | Dòng gọi trong `registerPtyHandlers` |
|---|---|
| `buildPtyHostEnv` | 1540, 3023, 3877 |
| `getCodexSelectionTargetForPty` | 2946, 3672, 3843 |
| `getCompatibleSelectedCodexHomePath` | 1536, 3010, 3849 |
| `promoteAgentTeamsShimPath` | 3036, 3060, 3891, 3925 |
| `deleteRequestedEnvKeys` | 3059, 3924 |
| `shouldSkipCodexHomeEnvForWindowsShell` | 3017, 3031, 3856, 3885 |
| `mergePtyEnvDeletions` | 3049–3054, 3914–3920 |
| `getInheritedAgentHookEnvKeysToDelete` | 3051, 3920 |
| `CODEX_HOME_ENV_KEYS` | 3056, 3922 |
| `GetSelectedCodexHomePath` (type, tham số hàm) | 1462, 5152 |
| `PrepareClaudeAuth` (type, tham số hàm) | 1464, 5154 |

## Các bước

1. `gitnexus impact({target: "buildPtyHostEnv", direction: "upstream"})` —
   dừng nếu risk HIGH/CRITICAL (không kỳ vọng, vì đây là hàm thuần, nhưng
   xác nhận theo đúng quy tắc bắt buộc).
2. Đọc dòng 208–217 và 545–1032 trọn vẹn, copy nguyên văn toàn bộ 20 symbol
   + import cần thiết (chỉ import những gì thực sự dùng trong đúng đoạn
   này — xem import block đầu file dòng 1–158 của `pty.ts` để chọn đúng).
3. Tạo `pty-host-env.ts`: paste nguyên văn.
4. Sửa `ipc/pty.ts`:
   - Xoá 2 đoạn nguồn (208–217, 545–1032).
   - Thêm `export { buildPtyHostEnv, type BuildPtyHostEnvOptions } from
     './pty-host-env'` tại đúng vị trí cũ của khối 545–1032 (giữ vị trí
     tương đối so với phần còn lại của file để diff dễ review).
   - Thêm `import { ... } from './pty-host-env'` cho các symbol còn được
     `registerPtyHandlers` gọi trực tiếp (bảng trên).
5. `pnpm run typecheck` (3 target) — nếu thiếu 1 symbol trong import, lỗi
   "cannot find name X" sẽ chỉ đúng dòng cần sửa.

## Xác minh xong

- [ ] `pnpm run typecheck` (3 target: node/cli/web) pass
- [ ] `pnpm run lint` pass
- [ ] `gitnexus detect_changes({scope: "compare", base_ref: "main"})` —
      risk = low, chỉ đúng 20 symbol này bị đổi vị trí
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `ipc/pty.ts` giảm ~500 dòng
      (488 dòng liền kề 545–1032 + 10 dòng 208–217, trừ dòng export/import
      thêm vào)
- [ ] Kiểm thử thủ công: spawn 1 terminal local (xác nhận `buildPtyHostEnv`
      vẫn set đúng `ORCA_OPENCODE_CONFIG_DIR`/`CODEX_HOME`/Pi-OMP env — dễ
      hồi quy âm thầm nếu import thiếu 1 hằng số) — không có `pty.test.ts`
      tự động cho file này (xác nhận bằng `ls frontend/src/main/ipc/`).

## Rollback

```
git checkout -- frontend/src/main/ipc/pty.ts
rm frontend/src/main/ipc/pty-host-env.ts
```

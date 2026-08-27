# TASK-BIGFILE-082 — Merge: resolveAgentTerminalCreateOptions vào domain đã tách

**Loại:** Move — biến thể mới: gộp method PRIVATE vào domain ĐÃ TỒN TẠI thay
vì tạo file mới (khác 081 task Move trước, vốn luôn tạo `orca-runtime-*.ts`
mới) · Rủi ro thấp · **Effort:** S · **Phụ thuộc:** TASK-BIGFILE-073, 078
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

Sweep tiếp tục theo lựa chọn "nhặt candidate nhỏ" phát hiện
`resolveAgentTerminalCreateOptions` (63 dòng, private) — vốn đã là host
dependency CỦA `orca-runtime-terminal-create.ts` (TASK-073) qua closure
`resolveAgentTerminalCreateOptions: (workspace, opts) =>
this.resolveAgentTerminalCreateOptions(workspace, opts)`, gọi từ 2 chỗ
trong `createTerminal`. Nhận ra: 2 trong 4 dependency của method này
(`markLocalWorkspaceTrustedForAgent`/`markRemoteWorkspaceTrustedForAgent`)
ĐÃ LÀ host dep sẵn có của domain đó, và `getStore` cũng đã có — chỉ thiếu 1
dependency mới (`getAgentLaunchPlatformForWorkspace`). Thay vì tạo file mới
với overhead riêng, **gộp thẳng method này vào `RuntimeTerminalCreateCommands`
làm private method nội bộ** — loại bỏ hoàn toàn lớp host-dependency
indirection cho method này (2 call site trong `createTerminal`/
`splitPtyBackedTerminal`-adjacent path chuyển từ `this.host.X()` thành
`this.X()` trực tiếp).

## Kết quả thực thi (2026-08-12)

- Domain: `resolveAgentTerminalCreateOptions` (dòng gốc 2762–2828, 63 dòng)
  chuyển từ `orca-runtime.ts` vào cuối class `RuntimeTerminalCreateCommands`
  trong `orca-runtime-terminal-create.ts` làm private method.
- 1 host dependency MỚI thêm vào `RuntimeTerminalCreateCommandHost`:
  `getAgentLaunchPlatformForWorkspace` (thay thế hẳn
  `resolveAgentTerminalCreateOptions` trong interface — method cũ biến mất
  khỏi host, không còn indirection). 2 dependency cũ
  (`markLocalWorkspaceTrustedForAgent`/`markRemoteWorkspaceTrustedForAgent`)
  và `getStore` TÁI SỬ DỤNG nguyên trạng, không cần thêm.
- 2 call site nội bộ trong `orca-runtime-terminal-create.ts`
  (`createTerminal` background-spawn branch, `launchAgentTerminal`'s
  transitive `createTerminal` call) đổi từ
  `this.host.resolveAgentTerminalCreateOptions(...)` sang
  `this.resolveAgentTerminalCreateOptions(...)`.
- Áp dụng ngay bài học TASK-073/081 (capture `store` vào biến cục bộ để TS
  narrow qua nhiều statement) từ lúc viết, không đợi lỗi.
- `resolveBareAgentLaunchCommand` import lại đúng nguồn thật
  (`./orca-runtime-service-types`, không phải `./orca-runtime` — không được
  re-export ở đó, chỉ import nội bộ).
- 6 import move-only dọn ở `orca-runtime.ts` sau `tsc`: `buildAgentStartupPlan`,
  `repoIsRemote`, cả block `resolveTuiAgentLaunchArgs`/`resolveTuiAgentLaunchEnv`,
  `resolveLocalWindowsAgentStartupShell`, `TerminalCreateOptions` (import cục
  bộ — GIỮ nguyên ở khối `export type` vì sibling vẫn dùng),
  `resolveBareAgentLaunchCommand` (import cục bộ — cũng giữ export).
- Xác minh fidelity bằng diff nguyên văn so với `git show HEAD:...` — khớp,
  chỉ khác chỗ capture `store` cục bộ có chủ đích.
- `orca-runtime.ts`: 4,819 → **4,742 dòng**.
  `orca-runtime-terminal-create.ts`: 869 dòng (761 non-blank/non-comment,
  tăng từ ~795 dòng trước đó — đã đăng ký baseline sẵn từ TASK-073, không
  cần đăng ký thêm).
- Xác minh: `tsc --noEmit --composite false` giữ đúng baseline 251 lỗi (6 lỗi
  move-only sửa ngay). `oxlint` sạch (exit 0) cả 2 config. `max-lines-ratchet`:
  647 vi phạm pre-existing không đổi.

## Rủi ro còn lại / khuyến nghị

- Rủi ro thấp — logic không đổi, chỉ đổi vị trí sở hữu + loại bỏ 1 lớp host
  closure không cần thiết. Không cần kiểm thử thủ công riêng ngoài smoke
  test tạo agent terminal thông thường (đã nằm trong phạm vi khuyến nghị
  test của TASK-073).
- Mẫu "gộp method private vào domain đã tồn tại thay vì tạo file mới" này
  có thể lặp lại cho các trường hợp tương tự khác nếu tìm thấy (method
  private chỉ được dùng làm host dep của ĐÚNG MỘT domain đã tách) — hiệu quả
  hơn tạo file mới vì loại bỏ cả overhead host-interface lẫn 1 dòng
  composition wiring.

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **4,742 dòng (82.3% giảm)** qua 50 task.

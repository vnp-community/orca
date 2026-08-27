# TASK-BIGFILE-051 — Move: Mobile-session-tabs domain, cluster 1 (listAllMobileSessionTabs)

**Loại:** Move — composition pattern · **Effort:** L (rủi ro cao nhất trong
toàn bộ nỗ lực, ngang PTY-lifecycle core) · **Phụ thuộc:** không
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh: domain phức tạp hơn PTY-lifecycle core

Sau TASK-BIGFILE-050, người dùng chọn tiếp tục với mobile-session-tabs —
domain lớn còn lại cuối cùng. Rà soát ban đầu cho thấy domain này KHÔNG liền
mạch mà chia làm 3 cụm rải rác trong file:
1. `listAllMobileSessionTabs` + ~48 helper (~2,229 dòng, dòng cũ 1857-4085)
2. `createMobileSessionTerminal` (~629 dòng)
3. `notifyMobileSessionTabsChanged` (~534 dòng)

Cụm 1 một mình có **90+ dependency `this.X`** khác biệt (so với ~30 của
toàn bộ domain PTY-lifecycle-core) — được người dùng xác nhận rủi ro cao
hơn và chọn tách cụm 1 trước.

## Phát hiện quan trọng: composition block chen giữa

Rà kỹ hơn phát hiện `fileCommands`/`gitCommands` composition wiring (2 domain
KHÔNG liên quan, từ TASK-BIGFILE-008/009) nằm CHEN GIỮA khối
1857-4085 (dòng ~3865-4030 cũ) — làm ranh giới ban đầu sai. Sau khi loại trừ
đúng, domain thực chia làm 2 đoạn: Segment A (listAllMobileSessionTabs →
saveMobileMarkdownTab) + Segment B (resolveRuntimeGitTarget,
resolveRuntimeFileTarget, onMobileSessionTabsChanged) — 2,061 dòng thực,
28 host dependency thực (không phải 90+ như ước tính thô ban đầu — phần lớn
"90 lần this.gitCommands/this.fileCommands" hoá ra là chính khối composition
wiring bị loại trừ, không phải code thực của domain).

## Kết quả thực thi (2026-08-11)

- Domain: `listAllMobileSessionTabs`, `activateMobileSessionTab`,
  `closeMobileSessionTab`, `moveMobileSessionTab`,
  `updateMobileSessionPaneLayout`, `setMobileSessionTabProps`,
  `readMobileMarkdownTab`, `saveMobileMarkdownTab`,
  `onMobileSessionTabsChanged` (public) + ~48 private helper materialization/
  sync cho headless (không renderer) mobile session tabs.
- `fileCommands`/`gitCommands` composition (dòng ~3865-4030 cũ) — cố tình
  loại trừ, giữ nguyên vị trí trong `orca-runtime.ts`.
- 28 host dependency thực, bao gồm state field dùng chung với 2 cụm chưa
  tách: `mobileSessionTabsByWorktree` (Map), `mobileSessionTabListeners`
  (Set), `mobileSessionTabsNotifyCoalescer` — expose qua host getter thay vì
  di chuyển, vì cả 3 cụm cùng đọc/ghi state này.
- **8 method riêng tư hoá ra được gọi từ code KHÔNG thuộc domain này** (cụm
  2/3 chưa tách, hoặc PTY core khác) — phát hiện qua vòng lặp tsc, không
  phải phân tích tĩnh trước: `hasServeOwnedPtyBinding`,
  `getMobileSessionSnapshotTabIdentityKeys`,
  `publishPtyBackedMobileSessionTerminal`, `touchMobileSessionSnapshotsForPty`,
  `persistHeadlessTerminalSplit`, `markHeadlessBrowserSessionTabActive`,
  `persistHeadlessTerminalTitle`, và
  `hydrateHeadlessMobileSessionTabsFromWorkspaceSession`,
  `refreshMobileSessionPtyRecords`, `buildMaterializedHeadlessParentLayout`,
  `getHeadlessMobileSessionGroupId`, `buildHeadlessMobileSessionTabGroups`,
  `mergeMobileSessionSnapshotTabs`, `mergeMobileSessionTabGroups` — tất cả
  bỏ `private`, expose public + forwarding field trên `OrcaRuntimeService`,
  đúng mẫu TASK-BIGFILE-037/040/049.
- **Bug thực phát hiện qua tsc**: `resolveRuntimeGitTarget`/
  `resolveRuntimeFileTarget` (ban đầu tưởng chỉ nội bộ domain) hoá ra được
  gọi từ chính khối `fileCommands`/`gitCommands` composition bị loại trừ ở
  trên (`this.resolveRuntimeFileTarget(selector)` trong wiring object) —
  nếu không sửa, code sẽ biên dịch lỗi ngay (không phải lỗi runtime âm thầm)
  vì method không còn tồn tại trên `OrcaRuntimeService`. Sửa bằng cách public
  hoá + forwarding field, cùng mẫu trên.
- Notifier type ban đầu dùng index signature `[key: string]: unknown` khiến
  `focusTerminal`/`moveSessionTab`/v.v. bị suy ra kiểu `unknown` (không gọi
  được) — sửa bằng khai báo tường minh từng method cần dùng.
- ~9 chỗ narrowing bị mất do gọi `this.host.getStore()`/`getNotifier()`
  nhiều lần trong cùng method — sửa bằng capture `const store =`/
  `const notifier =` một lần, dùng lại xuyên suốt — đúng mẫu chuẩn toàn bộ
  effort.
- `orca-runtime.ts`: 13,949 → **11,992 dòng** (giảm ~1,957 dòng). File mới:
  2,256 dòng — domain LỚN THỨ HAI từng tách (sau worktree-creation's 2,349),
  đã đăng ký `config/max-lines-baseline.txt` + `eslint-disable max-lines`
  inline theo đúng quy trình AGENTS.md.
- Xác minh: `tsc --noEmit --composite false` 251 lỗi pre-existing không đổi
  (0 lỗi mới), `oxlint` sạch (exit 0) cả 2 config. `pnpm
  check:max-lines-ratchet`: diff giữa trước/sau giống hệt (không tạo thêm
  "New max-lines bypass" nào).

## Rủi ro còn lại / khuyến nghị

- Move cơ học thuần tuý (không đổi logic) — nhưng đây là domain PHỨC TẠP
  NHẤT đã tách trong toàn bộ effort (90+ dependency ban đầu, nhiều lần phát
  hiện method riêng tư cần public hoá qua vòng lặp tsc thay vì phân tích
  tĩnh trước). Khuyến nghị kiểm thử thủ công kỹ luồng mobile/web session tab
  (list, activate, close, move, markdown tab, pane layout) trước khi merge.
- 2 cụm còn lại (`createMobileSessionTerminal` ~629 dòng,
  `notifyMobileSessionTabsChanged` ~534 dòng) chưa tách — nhiều host
  dependency của cụm 1 trỏ NGƯỢC vào 2 cụm này (`createHeadlessMobileSessionTerminal`,
  `getMobileSessionTabsForWorktree`, `toMobileSessionTabsResult`, v.v.) — khi
  tách cụm 2/3, cần đảo hướng các forwarding field này thành gọi trực tiếp
  giữa 2 class mới (hoặc host injection 2 chiều).

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **11,992 dòng (55.2% giảm)** qua 20 task
(TASK-BIGFILE-036 đến 051).

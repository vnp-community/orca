# TASK-BIGFILE-053 — Move: Mobile-session-tabs domain, cluster 3 (notifyMobileSessionTabsChanged)

**Loại:** Move — composition pattern · **Effort:** M · **Phụ thuộc:**
TASK-BIGFILE-051, TASK-BIGFILE-052 (dùng chung forwarding field 2 chiều với
cả 2 cụm trước)
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

Cụm thứ 3 (cuối cùng) trong 3 cụm mobile-session-tabs (xem TASK-BIGFILE-052's
"Rủi ro còn lại"). Trung tâm là `notifyMobileSessionTabsChanged` +
`toMobileSessionTabsResult` (chuyển đổi snapshot nội bộ → payload client) và
các helper build agent-status/preserve-headless-tab đi kèm.

## Ranh giới xác nhận trước khi tách

- Range gốc: dòng 8896–9629 (734 dòng, kể cả dòng trắng cuối).
- `getSummaryForRuntimeWorktreeId`/`buildTerminalSummary` (dòng 8852/8873,
  ngay trước) — xác nhận KHÔNG thuộc cụm này, dùng bởi
  `listTerminals`/`getWorktreePs` (dòng 4669, 5017, 5939–6023), nằm ngoài
  range ứng viên.
- `getAgentStatusForHandle` trở đi (dòng 9633+) — xác nhận KHÔNG thuộc cụm
  này, hạ tầng agent-status-theo-pane-key chung, không riêng mobile.

## Kết quả thực thi (2026-08-11)

- Domain: `notifyMobileSessionTabsChanged` (public) + 23 helper riêng
  (`syncMobileSessionTabs`, `mergePreservedHeadlessMobileSessionTabs`,
  `buildPreservedHeadlessMobileSessionSnapshot`,
  `collectPreservedHeadlessMobileSessionTabs`,
  `shouldPreserveHeadlessMobileSessionTab`,
  `isHeadlessMobileSessionPublication`,
  `getMergedMobileSessionPublicationEpoch`, `notifyMobileSessionTabsRemoved`,
  `notifyMobileSessionTabsChangedNow`, `notifyMobileSessionTabSnapshots`,
  `getMobileSessionTabsForWorktree`, `resolveMobileMarkdownWorktreeId`,
  `getLiveBrowserTabsByPageId`, `collectReturnedSessionTabIds`,
  `sanitizeMobileSessionTabGroups`, `pruneMobileSessionTabGroupLayout`,
  `toMobileSessionTabsResult`, `buildPtyMobileAgentStatus`,
  `getFreshRetainedAgentStatusForMobileTab`, `findPtyForMobileTerminalTab`,
  `getPersistedSshPtyIdForMobileTerminalTab`, `getMobileTerminalPaneKey`,
  `mobileTerminalTabMatchesPty`).
- 18 host dependency (state getters + method deps), file mới:
  `orca-runtime-mobile-session-notify.ts` (849 dòng).
- **Phát hiện quan trọng: gần như toàn bộ method public trong cụm này được
  cụm 1 (`orca-runtime-mobile-session-tabs.ts`) gọi ngược qua host wiring**
  — không giống cụm 2 (chỉ 3/9 method cần public), ở đây 12/24 method cần
  public + forwarding field vì composition wiring của cụm 1 (được viết từ
  TASK-BIGFILE-051, TRƯỚC khi cụm 3 tồn tại dưới dạng file riêng) gọi
  `this.host.X(...)` cho: `getMobileSessionTabsForWorktree`,
  `getLiveBrowserTabsByPageId`, `toMobileSessionTabsResult`,
  `isHeadlessMobileSessionPublication`,
  `getPersistedSshPtyIdForMobileTerminalTab`, `resolveMobileMarkdownWorktreeId`,
  `findPtyForMobileTerminalTab` (bỏ sót ở lần viết đầu — gây 1 lỗi tsc
  `TS2339`, phát hiện + sửa ngay). Cộng thêm `syncMobileSessionTabs`,
  `notifyMobileSessionTabsChanged`, `notifyMobileSessionTabsChangedNow`,
  `notifyMobileSessionTabSnapshots` — gọi từ chính `orca-runtime.ts` (field
  initializer coalescer, `listTerminals`/graph-sync completion, các call
  site khác). 12 method còn lại (không có external ref) giữ private trong
  class mới.
- `RuntimeAgentRowSnapshot` (type nội bộ, dòng 663) — thêm `export`, dùng
  lại qua `import type { RuntimeAgentRowSnapshot } from './orca-runtime'`
  (dùng cả trong field `latestAgentStatusByPaneKey` ở `orca-runtime.ts` lẫn
  trong file mới).
- Import free-function/const: `createHash` (node:crypto, chỉ dùng trong
  cụm — move hẳn, xoá khỏi `orca-runtime.ts`), `normalizeCompatibleAgentStatusEntryForOwner`/
  `normalizeCompatibleAgentTitleForOwner` (chỉ dùng trong cụm — move hẳn,
  giữ lại `hasCompatibleAgentTitleIdentity` ở `orca-runtime.ts`),
  `FIRST_PANE_ID` (move hẳn), `BrowserTabInfo`/`RuntimeMobileSessionTabsRemovedResult`/
  `RuntimeMobileSessionMarkdownTab`/`RuntimeMobileSessionClientTab`/
  `RuntimeMobileSessionTabGroup`/`RuntimeMobileSessionSnapshotTab` (move
  hẳn khỏi import block của `orca-runtime.ts`, giữ nguyên các type khác
  cùng khối import vẫn dùng ở nơi khác). `join` (tưởng nhầm là `node:path`
  join khi quét identifier thô — kiểm tra lại thấy chỉ là
  `Array.prototype.join('|')`, không phải free-function import — false
  positive, bỏ qua).
- Thêm cả `eslint-disable max-lines` và `eslint-disable unicorn/no-useless-spread`
  inline (giống disable gốc ở đầu `orca-runtime.ts`) cho pattern clone
  `mobileSessionTabsByWorktree` map thành array trước khi lặp — vòng lặp có
  thể `.delete()` entry giữa lúc lặp.
- `orca-runtime.ts`: 11,432 → **10,760 dòng** (giảm 672 dòng). File mới:
  849 dòng — đã đăng ký `config/max-lines-baseline.txt` + `eslint-disable
  max-lines` inline theo đúng quy trình AGENTS.md.
- Xác minh: `tsc --noEmit --composite false` 251 lỗi pre-existing không đổi
  (0 lỗi mới — phát hiện + sửa 1 lỗi tạm thời do thiếu forwarding field cho
  `findPtyForMobileTerminalTab`). `oxlint` sạch (exit 0) cả 2 config (mặc
  định + react-doctor). `pnpm check:max-lines-ratchet`: 647 vi phạm
  pre-existing giống hệt trước/sau (xác nhận qua `git stash` so sánh),
  không có vi phạm mới ngoài file mới đã đăng ký.

## Rủi ro còn lại / khuyến nghị

- Move cơ học thuần tuý — cùng mức rủi ro như cụm 1/2 (zero test coverage).
  Khuyến nghị kiểm thử thủ công luồng: session.tabs snapshot đồng bộ khi
  renderer graph thay đổi, notify coalescing (title/status churn không làm
  spam), preserve headless tab khi renderer bỏ qua worktree, agent status
  hiển thị đúng trên mobile/web client (bao gồm suppress khi title không
  còn là agent), trước khi merge.
- Đây là task cuối cùng trong nhóm mobile-session-tabs (3/3 cụm đã tách
  xong) — không còn cụm con nào của domain này trong `orca-runtime.ts`.

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **10,760 dòng (59.8% giảm)** qua 23 task
(TASK-BIGFILE-036 đến 053).

# TASK-BIGFILE-052 — Move: Mobile-session-tabs domain, cluster 2 (createMobileSessionTerminal)

**Loại:** Move — composition pattern · **Effort:** M · **Phụ thuộc:**
TASK-BIGFILE-051 (dùng chung forwarding field với cluster 1)
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

Cụm thứ 2 trong 3 cụm mobile-session-tabs (xem TASK-BIGFILE-051's "Việc tiếp
theo"). Ước tính ban đầu ~629 dòng khớp gần đúng với thực tế.

## Phát hiện quan trọng: ranh giới ban đầu sai lần nữa

Rà ban đầu định gộp cả `waitForTerminalHandle`/`resolveHandleForTab` vào
cụm (nằm ngay sau `isReadyMobileTerminalSurface`), nhưng kiểm tra kỹ phát
hiện 2 method này KHÔNG được gọi từ bất kỳ đâu trong cụm — chỉ được gọi từ
`createTerminal` (method PTY core dùng chung cho cả desktop lẫn mobile,
dòng ~7158, TRƯỚC ranh giới cụm). Cùng mẫu với `waitForLeafPtyId`/
`countLeavesInTab` đứng ngay sau đó — cả 4 method này đều là hạ tầng
`createTerminal` chung, không thuộc domain mobile-session-terminal. Sau khi
loại trừ, cụm 2 hoá ra LIỀN MẠCH HOÀN TOÀN (591 dòng, không cần chia đoạn
A/B như lo ngại ban đầu).

## Kết quả thực thi (2026-08-11)

- Domain: `createMobileSessionTerminal` (public) +
  `runCreateMobileSessionTerminal`, `resolveMobileSessionTerminalCommand`,
  `createHeadlessMobileSessionTerminal`, `waitForMobileTerminalSurface`,
  `findMobileTerminalSurface`, `ensurePtyBackedMobileSurfaceForRendererTab`,
  `findLiveRegisteredPtyForRendererTab`, `hasLiveShellForRendererTab`,
  `isReadyMobileTerminalSurface` (private helper, trừ 3 method dưới).
- 21 host dependency, phần lớn đã là forwarding field từ cụm 1
  (`buildHeadlessMobileSessionTabGroups`, `getHeadlessMobileSessionGroupId`,
  `hydrateHeadlessMobileSessionTabsFromWorkspaceSession`,
  `publishPtyBackedMobileSessionTerminal`, `toMobileSessionTabsResult`,
  state field `mobileSessionTabsByWorktree`/`mobileSessionTabListeners`) —
  gọi xuyên qua `OrcaRuntimeService` vẫn hoạt động minh bạch, không cần sửa
  cụm 1.
- 3 method ban đầu private hoá ra cần public + forwarding field: 2 method
  (`createHeadlessMobileSessionTerminal`, `resolveMobileSessionTerminalCommand`)
  được cụm 1's host wiring gọi tới (`this.createHeadlessMobileSessionTerminal(...)`
  trong composition object của cụm 1) — nếu không sửa, build lỗi ngay lập
  tức (method không tồn tại). Method thứ 3
  (`ensurePtyBackedMobileSurfaceForRendererTab`) được gọi từ PTY registration
  code (registerPty) ở nơi khác trong `orca-runtime.ts`.
- `mobileTerminalCreateByMutationId`, `pendingMobileTerminalCreatesByKey`
  (2 Map riêng, chỉ domain này dùng) — chuyển hẳn thành private field của
  class mới.
- `MOBILE_TERMINAL_CREATE_RESULT_TTL_MS`, `MOBILE_TERMINAL_SURFACE_TIMEOUT_MS`,
  `MOBILE_TERMINAL_READY_FALLBACK_MS`, `isClientDisconnectedError` (hằng số
  + hàm tự do, chỉ domain này dùng) — chuyển hẳn.
- Thêm `eslint-disable unicorn/no-useless-spread` inline (giống disable gốc
  ở đầu `orca-runtime.ts`) cho pattern clone `graphSyncCallbacks` trước khi
  lặp — callback có thể tự huỷ đăng ký giữa lúc lặp.
- `orca-runtime.ts`: 11,994 → **11,432 dòng** (giảm ~562 dòng). File mới:
  742 dòng — đã đăng ký `config/max-lines-baseline.txt` +
  `eslint-disable max-lines` inline theo đúng quy trình AGENTS.md.
- Xác minh: `tsc --noEmit --composite false` 251 lỗi pre-existing không đổi
  (0 lỗi mới), `oxlint` sạch (exit 0) cả 2 config. `pnpm
  check:max-lines-ratchet`: diff giữa trước/sau giống hệt.

## Rủi ro còn lại / khuyến nghị

- Move cơ học thuần tuý — cùng mức rủi ro như cụm 1 (zero test coverage).
  Khuyến nghị kiểm thử thủ công luồng tạo terminal mobile/web (có/không
  authoritative window, có/không agent, idempotency qua clientMutationId,
  rollback khi lỗi) trước khi merge.
- Cụm 3 còn lại (`notifyMobileSessionTabsChanged`, ~534 dòng) — tương tự sẽ
  cần forwarding field 2 chiều với cụm 1 và có thể cụm 2.

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **11,432 dòng (57.2% giảm)** qua 22 task
(TASK-BIGFILE-036 đến 052).

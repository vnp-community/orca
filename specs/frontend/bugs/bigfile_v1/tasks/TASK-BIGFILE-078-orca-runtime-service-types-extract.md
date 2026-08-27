# TASK-BIGFILE-078 — Extract: pure type/function declarations ahead of the class

**Loại:** Extract — KHÔNG phải composition pattern, phương pháp khác theo yêu
cầu người dùng ("sử dụng phương pháp khác nếu cần") · Rủi ro thấp (không có
`this`, xác minh hoàn toàn qua `tsc`) · **Effort:** L (thay đổi rộng, nhưng
mỗi thay đổi đơn giản) · **Phụ thuộc:** không (độc lập với mọi domain đã tách)
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

Sau TASK-BIGFILE-077, sweep gap-analysis (đã sửa lại quy tắc regex lần nữa để
bắt cả block wiring `readonly X = new Runtime...Commands(`) không còn tìm
thấy cụm method lớn nào khác đáng kể — phần lớn "gap" còn lại là noise từ
forwarding-field list dài (ví dụ cụm `RuntimeBrowserCommands` với hàng chục
field). Theo đúng yêu cầu người dùng dùng "phương pháp khác": nhận ra rằng
**656 dòng TRƯỚC class `OrcaRuntimeService`** (dòng gốc 264–918) toàn bộ là
type declaration, free function, và 2 error class — hoàn toàn KHÔNG có `this`
(xác nhận bằng grep: chỉ 2 occurrence, cả hai đều là `this.code =`/`this.data
=` bên trong constructor của `RuntimeLineageError`, một class độc lập không
liên quan `OrcaRuntimeService`).

Đây không phải composition pattern Move (không cần host interface, không có
side hiệu ứng nào để wire) — mà là Extract thuần: di chuyển nguyên văn, import
lại để dùng nội bộ, re-export cho ~20 sibling file đã import các type/function
này từ `'./orca-runtime'`. Rủi ro cực thấp vì toàn bộ xác minh qua `tsc` —
không có logic runtime nào để kiểm thử thủ công.

## Kết quả thực thi (2026-08-12)

- Domain: TOÀN BỘ 264–918 dòng gốc — 19 type (`RuntimeAccountServices`,
  `RuntimeStore`, `RuntimeLeafRecord`, `RuntimePtyWorktreeRecord`,
  `TerminalCreateOptions`, `PtyForegroundAgentRefresh`,
  `RuntimePtyTitleTrackerEntry`, `RuntimeAgentRowSnapshot`, `RuntimeNotifier`,
  `TerminalHandleRecord`, `TerminalWaiter`, `ResolvedWorktree`,
  `TerminalWorkspaceLaunchScope`, `WorktreeLineageInput`,
  `ResolvedWorkspaceParent`, `WorktreeLineageResolution`,
  `RuntimeWorktreeScanResult`, `WorktreeLineageCandidate`), 10 free function
  (`isCursorAgentOrchestrationTarget`, `copySleepingAgentLaunchConfig`,
  `normalizeAgentLaunchCommandForMatch`, `resolveBareAgentLaunchCommand`,
  `getAgentLaunchPlatformForRepo`, `omitUndefinedProperties`,
  `getRuntimeFolderWorkspaceRootId`, `getRuntimeFolderWorkspaceInstanceIdentity`,
  `isRuntimeFolderWorkspaceIdForRepo`, `mergeRuntimeFolderWorkspace`,
  `listRuntimeFolderWorkspaces`, `addListenerToMap`), 2 error class
  (`RuntimeLineageError`, `WorktreeIdRequiresFullPathError`), 1 const
  (`AGENT_HOOK_RUNTIME_ENV_KEYS`) — sang file mới
  `orca-runtime-service-types.ts`.
- Xác minh trước khi chuyển: quét toàn bộ `~20` sibling composition-command
  file (script Python parse multi-line `import {...} from './orca-runtime'`,
  tránh lỗi trước đó của regex một-dòng bỏ sót import nhiều dòng) — hầu hết
  type/function đã được sibling file dùng, chỉ 5 cái
  (`isCursorAgentOrchestrationTarget`, `getAgentLaunchPlatformForRepo`,
  `resolveBareAgentLaunchCommand`, `RuntimeNotifier`,
  `AGENT_HOOK_RUNTIME_ENV_KEYS`) chỉ dùng nội bộ `OrcaRuntimeService`.
- **1 bug tự phát hiện khi soạn file mới**: viết nhầm `isWindowsAbsolutePathLike`
  thành một hàm stand-in tự chế thay vì import hàm thật từ
  `'../../shared/cross-platform-path'` — phát hiện qua tự-diff trước khi
  chạy `tsc` (không phải tsc bắt, vì cả 2 hàm cùng chữ ký nên không lỗi kiểu),
  sửa bằng cách import đúng hàm gốc (hàm này CŨNG được dùng ở nơi khác trong
  `orca-runtime.ts`, nên STAYS làm import kép — vừa ở file mới vừa ở
  `orca-runtime.ts`).
- `orca-runtime.ts`: thay khối cũ bằng `import type {...}` +
  `import {...}` (dùng nội bộ) và `export type {...}` + `export {...}`
  (re-export cho sibling) từ `'./orca-runtime-service-types'` — đúng mẫu
  đã có sẵn ở đáy file cho `orca-runtime-types.ts`.
- **46 lỗi `tsc` move-only ban đầu** (tất cả TS6133/TS6196 unused-import),
  toàn bộ do 2 nguồn: (1) import gốc ở đầu `orca-runtime.ts` (như `AgentStatus`,
  `TerminalTitleTracker`, `WorktreeLineage`, `SleepingAgentLaunchConfig`, …)
  chỉ được dùng BÊN TRONG các type/function đã chuyển đi, không dùng nơi nào
  khác trong class body; (2) import cục bộ tôi tự thêm cho file mới nhưng hoá
  ra chỉ cần cho khối `export type {...}` re-export (không có usage thật
  trong class body) — dọn sạch theo đúng danh sách `tsc` báo, giữ nguyên khối
  `export`. Sau khi dọn: `tsc` sạch ngay từ lần chạy thứ hai.
- `orca-runtime.ts`: 5,752 → **5,117 dòng (635 dòng, mức giảm 1 task lớn
  nhất từ trước đến nay)**. File mới `orca-runtime-service-types.ts`: 716
  dòng (615 non-blank/non-comment) — đã đăng ký
  `config/max-lines-baseline.txt` + `eslint-disable max-lines`.
- Xác minh: `tsc --noEmit --composite false` giữ đúng baseline 251 lỗi.
  `oxlint` sạch (exit 0) cả 2 config. `max-lines-ratchet`: 647 vi phạm
  pre-existing không đổi.

## Rủi ro còn lại / khuyến nghị

- Rủi ro thấp — KHÔNG có logic runtime nào thay đổi (pure type/function
  relocate, không mutate state, không side effect). Toàn bộ xác minh qua
  `tsc`; không cần kiểm thử thủ công riêng cho task này (nhưng vẫn nên chạy
  smoke test tổng quát trước khi merge, vì đây là thay đổi rộng chạm gần như
  mọi sibling file).
- Cùng phương pháp có thể áp dụng tiếp cho vùng SAU class (dòng gốc
  5677–5752 hiện tại, gồm `withTimeout`, `withTimeoutResult`,
  `getExplicitWorktreeIdSelector`, các const `DEFAULT_WORKTREE_LIST_LIMIT`/
  `DISCONNECTED_PTY_RECORD_MAX`/`PTY_CONTROLLER_LIST_TIMEOUT_MS`, và khối
  export cuối file) — chưa làm trong task này, để đánh giá riêng nếu tiếp tục.
- File `orca-runtime.ts` giờ chủ yếu là class body thuần (constructor + ~45
  domain composition wiring block + vài chục method utility nhỏ lẻ không đủ
  lớn để tách). Cụm lớn còn sót đáng chú ý nhất: `syncWindowGraph` (~145
  dòng, mutate `this.graph` trực tiếp, tương đương rủi ro `onPtyData`) — có
  thể là candidate tiếp theo cho composition-pattern Move nếu người dùng
  muốn tiếp tục.

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **5,117 dòng (80.9% giảm — vượt mốc 80%)** qua
46 task (TASK-BIGFILE-036 đến 078, trừ 057 đã huỷ rồi tái thực thi ở 067;
041, 063 là state-container Extract; 078 là type/function Extract).

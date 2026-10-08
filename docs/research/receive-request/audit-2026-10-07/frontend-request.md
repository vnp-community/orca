# Kiểm toán thực thi: frontend, feature `request-frontend`, FE-REQ-TASK-018 đến 023, 032, 036

Ngày: 2026-10-07. Phạm vi: 54 task (018:6, 019:7, 020:5, 021:6, 022:7, 023:7, 032:8, 036:8). Code ở `/opt/repos/orca/frontend/src`. Chỉ đọc và chạy lệnh, không sửa code.

## 1. Lệnh đã chạy và kết quả

| Lệnh (cwd `/opt/repos/orca/frontend`) | Kết quả |
|---|---|
| `pnpm exec vitest run` không `--config` | Không dùng được: script `test` trỏ `config/vitest.config.ts`; phải thêm `--config config/vitest.config.ts` |
| `pnpm exec vitest run --config config/vitest.config.ts src/shared/request src/renderer/src/components/request src/renderer/src/store/slices/request.test.ts src/renderer/src/lib/request-event-bus.test.ts src/renderer/src/components/task/__tests__/TaskDAGView.test.tsx` | 9 file, 161 test: 158 pass, 3 fail. Lỗi: `request-list-keyboard.test.ts` 2 test ("document is not defined": thiếu docblock `@vitest-environment happy-dom`, môi trường mặc định là node); `request-stage-timeline-model.test.ts` "cancelled marks current step as skipped" |
| `... src/renderer/src/components/task src/shared/task-status-normalization` | 43 file, 302 test: 289 pass, 13 fail. 13 fail đều ở `task-page-*` (default-repo-selection, source-switch-boundary, drawer-source-boundary), không thuộc phạm vi request. Thêm 1 unhandled error: thiếu package `jsdom` cho `TaskSourceBadge.test.tsx` |
| `... src/shared/task-hierarchy` | 18 test: 17 pass, 1 fail ("handles cycle in parentId without infinite loop": `expected 't2' to be null`) |
| `... src/renderer/src/components/request/solution src/shared/request` | 4 file, 96 test, pass hết |
| `pnpm exec tsc --noEmit -p tsconfig.json` | Chạy được, 184 dòng `error TS` (baseline task ghi 114, không đối chiếu được từng lỗi). Lỗi thuộc phạm vi request: `request-stage-timeline-model.test.ts(163)` ('hotfield' sai chính tả), `store/slices/request.test.ts(5,13)` (import thừa, `StateCreator<AppState>` gán `RequestSlice`), và 7 lỗi `store/slices/ui.ts` (1419, 1552, 1569, 1592, 1603, 1613, 1628): kiểu `previousViewBefore*` (ui.ts:634...) không có `'requests'` trong khi `activeView` đã thêm `'requests'`. Đây là lỗi do task 018-04 gây ra. Lạ: tsc không báo gì ở `TaskBoardView.tsx:11` dù `STATUS_ORDER: TaskStatus[]` chứa `'backlog'` đã bị gỡ khỏi `TaskStatus`; chưa giải thích được (có thể do cache `tsconfig.tsbuildinfo` composite) |

Không chạy được: e2e/Playwright (không có file e2e nào cho request trong phạm vi), không có dev server, không kiểm chứng giao diện thật.

## 2. Tóm tắt theo solution

| Solution | Số task | Đủ | Một phần | Chưa làm | Không kiểm chứng được | Tỉ lệ Đủ |
|---|---|---|---|---|---|---|
| SOL-018 foundation | 6 | 1 | 5 | 0 | 0 | 17% |
| SOL-019 list/detail | 7 | 0 | 2 | 5 | 0 | 0% |
| SOL-020 solution review | 5 | 1 | 0 | 4 | 0 | 20% |
| SOL-021 plan/phase | 6 | 0 | 2 | 4 | 0 | 0% |
| SOL-022 approval inbox | 7 | 0 | 0 | 7 | 0 | 0% |
| SOL-023 backlog | 7 | 0 | 2 | 5 | 0 | 0% |
| SOL-032 graph/lens | 8 | 0 | 0 | 8 | 0 | 0% |
| SOL-036 clarification/readiness | 8 | 0 | 0 | 8 | 0 | 0% |
| **Tổng** | **54** | **2 (4%)** | **11 (20%)** | **41 (76%)** | **0** | |

Trạng thái ghi trong file: 9 task `[x] DONE` (018-01 đến 018-06, 019-01, 020-01, 021-01), 1 task `TODO (blocked)` (032-07), 44 task `TODO`. Trong 9 task `[x]`, 7 task không phải "Đủ" (trạng thái sai): 018-02, 018-03, 018-04, 018-05, 018-06, 019-01, 021-01. Không task `[ ]` nào đã có code đủ. Mọi checkbox "Tiêu chí hoàn thành" trong 54 file vẫn là `[ ]`.

## 3. Bảng từng task

Ký hiệu đường dẫn: `S/` = `frontend/src/shared/`, `R/` = `frontend/src/renderer/src/`.

| Task | Trạng thái ghi | Verdict | Bằng chứng | Thiếu hoặc sai |
|---|---|---|---|---|
| 018-01 types, registry, parsers | [x] | Đủ | `S/request-types.ts`, `request-flow-registry.ts`, `request-rpc-methods.ts`, `request-errors.ts`, `request-wire-parsers.ts` + 4 file test, pass; không `any`, không import `renderer/` (grep) | Kiểu lệch hợp đồng WS, xem mục 5. Chưa kiểm từng mã lỗi CR-016 có test |
| 018-02 RPC client, event bus | [x] | Một phần | `R/runtime/request-rpc-client.ts` (`callRequestRpc`, `subscribeRequestEvents`), `R/lib/request-event-bus.ts` + test bus | Không có `request-rpc-client.test.ts`. `RequestEvent` thiếu `approvalId`, `subjectType`, `solutionId`, `phaseTaskId`, `planTaskId`, `trigger` của khung WS. Tên kênh `'request.subscribe'` viết cứng (client:98), không dùng `REQUEST_RPC_METHODS.SUBSCRIBE` |
| 018-03 hooks | [x] | Một phần | `R/hooks/useRequestFlowSupport.ts`, `useRequests.ts`, `useRequest.ts`, `useRequestActions.ts`, `useSolutions.ts`, `useApprovals.ts`, `useBacklog.ts`, `useRequestSubscription.ts` | Không có `useRequestEvents.ts` (có `useRequestSubscription` nhưng không component nào gọi). Không có test hook nào. Chỉ `useRequestFlowSupport` được dùng (trong RequestPage); 6 hook còn lại không có nơi gọi. Lỗi so hợp đồng: `useApprovals.ts` gửi `approvalId` (hợp đồng: `id`); `useBacklog.ts:55` đọc `result.value.items` (hợp đồng: `requestRows`/`groups`); `nextPage` không gửi `pageToken`; `useApprovals` tính `pendingApprovalCount` từ một trang |
| 018-04 store slice, routing | [x] | Một phần | `R/store/slices/request.ts`, `store/index.ts:58,128`, `store/types.ts:56,123`, `S/types.ts:3378`, `ui.ts:507-520`, test `request.test.ts` pass | 7 lỗi tsc mới ở `ui.ts` (`previousViewBefore*` thiếu `'requests'`); không có `previousViewBeforeRequests`, nên đóng trang luôn về `terminal` (RequestPage.tsx:41). Không mở rộng test `ui` |
| 018-05 page shell, sidebar | [x] | Một phần | `RequestPage.tsx`, `RequestUnsupportedNotice.tsx`, 4 badge, `request-status-presentation.ts`, `SidebarRequestNavButton.tsx` (đã thêm vào `SidebarNav.tsx:73`), `App.tsx:319,2491` | Thiếu `RequestPageHeader`, bộ chọn dự án, `request-locale-coverage.test.ts`, `docs/ui/pages/requests.md`. Ba tab chỉ là `<div data-testid>` rỗng (RequestPage.tsx:100-113). Khoá i18n không có trong 5 locale. Nút sidebar không bao giờ hiện, xem mục 4 |
| 018-06 gỡ `backlog` | [x] | Một phần | `S/task-types.ts:23-33` (đã bỏ `backlog`, thêm `plan|phase`, `requestId`), `S/task-status-normalization.ts` + test pass | `normalizeTask` không được gọi ở đâu (`useTasks.ts:63` vẫn `setTasks(response.tasks ?? [])`, `useTask.ts` không đổi). `'backlog'` vẫn còn ở `TaskBoardView.tsx:11`, `TaskDetail.tsx:31`, `TaskStatusBadge.tsx:9`, `TaskDAGView.tsx:32`; fallback badge vẫn `STATUS_CONFIG.todo` (dòng 26). Test cũ vẫn khẳng định cột `board-column-backlog` (TaskBoardView.test.tsx:104,117) và badge Backlog (TaskStatusBadge.test.tsx:37) |
| 019-01 stage timeline model | [x] | Một phần | `R/components/request/request-stage-timeline-model.ts` + test | Test fail (cancelled): `STATUS_TO_CURRENT_STEP` không có khoá `cancelled` nên `current=null`, bước không bao giờ `skipped` (model:120-122,142). Test dòng 163 sai tên `hotfield` (tsc). `CHILD_REQUEST_RULES` đặt trong model (dòng 50), task yêu cầu trong `request-flow-registry.ts` |
| 019-02 danh sách, lọc, phím | [ ] | Một phần | `R/components/request/request-list-keyboard.ts` + test | Chỉ có hàm phím (2 test fail do môi trường). Không có `RequestsTab`, `RequestListToolbar`, `RequestFilterBar`, `RequestList`, `RequestRow`, `RequestListStates` |
| 019-03 detail pane | [ ] | Chưa làm | `find src -ipath '*request*'`: không có file detail | |
| 019-04 xác nhận loại, lịch sử | [ ] | Chưa làm | như trên | |
| 019-05 related, spawn child | [ ] | Chưa làm | như trên (chỉ có `CHILD_REQUEST_RULES`) | |
| 019-06 create dialog, nút ở Tasks | [ ] | Chưa làm | grep `CreateRequestDialog`: 0 | |
| 019-07 i18n, e2e | [ ] | Chưa làm | 0 khoá `auto.components.request.*` trong 5 locale; không có e2e | |
| 020-01 solution view model | [x] | Đủ | `R/components/request/solution/solution-view-model.ts` + test, pass; chỉ import type | Dựa trên kiểu `Solution` lệch hợp đồng (mục 5) |
| 020-02 panel body | [ ] | Chưa làm | thư mục `solution/` chỉ có view-model | |
| 020-03 so sánh option | [ ] | Chưa làm | như trên | |
| 020-04 reject dialog, decision bar | [ ] | Chưa làm | như trên | |
| 020-05 i18n, e2e | [ ] | Chưa làm | không có khoá `SolutionPanel.*` ngoài tham chiếu rời rạc | |
| 021-01 lọc Plan/Phase, effective parent | [x] | Một phần | `S/task-hierarchy.ts` (`isPlanningTask`, `isWorkTask`, `computeEffectiveParents`) | Không nối vào `useTasks.ts`, `TaskGraph`, `TaskTreeView`, `TaskCard` (grep `showPlanningTasks`, `isPlanningTask`: chỉ trong file chuẩn). Test vòng lặp fail (`task-hierarchy.test.ts`) |
| 021-02 usePlanTree | [ ] | Một phần | `S/task-hierarchy.ts:118 buildPlanSubtree`, `R/components/request/plan/plan-approval-model.ts` | Không có `usePlanTree.ts`; không có `plan-approval-model.test.ts`; model không có nơi gọi |
| 021-03 plan tree components | [ ] | Chưa làm | thư mục `plan/` chỉ có model | |
| 021-04 approval bars | [ ] | Chưa làm | như trên | |
| 021-05 plan tab states, gate | [ ] | Chưa làm | | |
| 021-06 i18n, e2e | [ ] | Chưa làm | | |
| 022-01 approval inbox rules | [ ] | Chưa làm | grep `approval-inbox`: 0 | |
| 022-02 useApprovalInbox | [ ] | Chưa làm | chỉ có `useApprovals` của 018-03, không phải hook inbox | |
| 022-03 summaries, minute clock | [ ] | Chưa làm | không có `useRequestSummaries`, `useMinuteClock`, `request-relative-time` | |
| 022-04 row list keyboard | [ ] | Chưa làm | không có `useRowListKeyboardNavigation` | |
| 022-05 row list, toolbar | [ ] | Chưa làm | | |
| 022-06 tab wiring, count | [ ] | Chưa làm | tab approvals là placeholder (RequestPage.tsx:104-107); `pendingApprovalCount` chỉ là state | |
| 022-07 i18n, docs | [ ] | Chưa làm | | |
| 023-01 backlog wire types | [ ] | Một phần | `S/request-types.ts:220-250 BacklogItem`, `parseBacklogItem` (parsers:247) | Không có `request-backlog-types.ts` (+test). Kiểu không khớp `BacklogRequestRowView`/`BacklogGroupView` của hợp đồng |
| 023-02 useBacklog | [ ] | Một phần | `R/hooks/useBacklog.ts` | Đọc sai khoá phản hồi, không phân trang thật, không test, không nơi gọi |
| 023-03 reopen/cancel dialogs | [ ] | Chưa làm | | |
| 023-04 backlog tab shell | [ ] | Chưa làm | tab backlog là placeholder | |
| 023-05 request backlog table | [ ] | Chưa làm | | |
| 023-06 task, execute tables | [ ] | Chưa làm | | |
| 023-07 board hint, i18n, docs | [ ] | Chưa làm | | |
| 032-01 risk tokens, RiskBadge | [ ] | Chưa làm | `grep -n risk assets/main.css`: không có `--risk-*`; không có `risk-presentation`, `RiskBadge` | |
| 032-02 TaskDAGView token | [ ] | Chưa làm | `components/task/TaskDAGView.tsx`: 9 hex cứng (dòng 27-33 STATUS_COLORS, 105 `#6b7280`, 131 `#94a3b8`), 0 `var(--`; còn khoá `backlog` (dòng 32) | |
| 032-03 graph types, parser, lens | [ ] | Chưa làm | không có `graph-types.ts`, `graph-wire-parsers.ts`, `components/graph/`, `useGraphLens` | |
| 032-04 grouping, zoom, adapters | [ ] | Chưa làm | | |
| 032-05 canvas, mini layout | [ ] | Chưa làm | | |
| 032-06 panel, toolbar | [ ] | Chưa làm | | |
| 032-07 ELK (gated) | [ ] blocked | Chưa làm | `package.json`: không có `elkjs` | Đúng là bị chặn, không tính lỗi |
| 032-08 entry points, i18n, e2e | [ ] | Chưa làm | | |
| 036-01 artifact types, parsers | [ ] | Chưa làm | không có `request-artifact-types.ts`, `request-artifact-parsers.ts`; kênh `clarification.*`, `decision.*`, `impact.*`, `readiness.*` không có trong `request-rpc-methods.ts` | |
| 036-02 hooks | [ ] | Chưa làm | | |
| 036-03 clarification panel | [ ] | Chưa làm | | |
| 036-04 decision bar | [ ] | Chưa làm | | |
| 036-05 risk summary | [ ] | Chưa làm | | |
| 036-06 risk acceptance, gating | [ ] | Chưa làm | | |
| 036-07 readiness badge | [ ] | Chưa làm | | |
| 036-08 execution result panel | [ ] | Chưa làm | | |

## 4. Stub và vấn đề chất lượng

1. Nút sidebar không bao giờ hiện (lỗi wiring). `SidebarRequestNavButton.tsx:24` trả `null` nếu `requestFlowSupport !== 'supported'`, mặc định `'unknown'` (`slices/request.ts:57`). `useRequestFlowSupport` chỉ được gọi trong `RequestPage.tsx:38`, mà `RequestPage` chỉ mount khi `activeView==='requests'` (`App.tsx:2491`), và `activeView` chỉ đặt bằng chính nút đó. Không có chỗ nào khác probe `request.flowStatus` (grep). Trang Request không thể mở bằng UI.
2. Ba tab của `RequestPage.tsx:100-113` là `div` rỗng ghi chú "Placeholder — filled by CR-019/022/023".
3. `useRequestSubscription`, `useRequests`, `useRequest`, `useRequestActions`, `useSolutions`, `useApprovals`, `useBacklog` không có nơi gọi nào trong component. Do đó `pendingApprovalCount` không bao giờ cập nhật, không có realtime, badge sidebar luôn ẩn.
4. Lỗi giao thức trong hook: `useApprovals.ts` (approve/reject) gửi `approvalId` thay vì `id`; `useBacklog.ts:55` đọc `items`; `useBacklog` `nextPage` chỉ tăng `refetchTrigger`, không gửi `pageToken`, danh sách bị thay thế thay vì nối; `useApprovals` có `eslint-disable react-hooks/exhaustive-deps` và bỏ qua `filters.subjectType`.
5. `normalizeTask` (018-06) không được gọi, nên task cũ mang `backlog` từ dữ liệu có thể làm mất cột trên Board; `TaskBoardView.tsx:11`, `TaskDetail.tsx:31`, `TaskStatusBadge.tsx:9`, `TaskDAGView.tsx:32` còn `backlog`.
6. `request-stage-timeline-model.ts`: nhánh `else if (status === 'cancelled')` giống hệt nhánh `else` (dòng 120-124) và `STATUS_TO_CURRENT_STEP` thiếu `cancelled`, nên trạng thái `skipped` không bao giờ xảy ra.
7. 7 lỗi tsc mới ở `ui.ts` do `previousViewBefore*` thiếu `'requests'`; lỗi tsc ở 2 file test request.
8. Test hỏng: `request-list-keyboard.test.ts` (thiếu docblock môi trường), `request-stage-timeline-model.test.ts`, `task-hierarchy.test.ts` (vòng lặp). Test `task-page-*` (13 fail) không thuộc phạm vi nhưng cho thấy bộ test chung không xanh.
9. i18n: 0 khoá `auto.components.request.*` trong `en/es/ja/ko/zh.json` (kiểm bằng `grep -c components.request`). `translate()` rơi về chuỗi tiếng Anh viết cứng, 5 locale không có bản dịch. `RequestStatus.awaiting_*` và nhiều khoá khác chưa có cả fallback ở nơi dùng. Không có test phủ locale cho request.
10. Màu: không có `--risk-*` trong `assets/main.css`; `TaskDAGView.tsx` còn 9 hex cứng, 0 `var(--`.
11. Không có file `docs/ui/pages/requests.md` và mục trong `docs/ui/page-tree.md`.

## 5. Lệch giữa tài liệu và code

Đối chiếu với `specs/backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md`.

- Tên kênh WS: `REQUEST_RPC_METHODS` khớp hợp đồng cho `request.*` (flowStatus, create, get, list, cancel, reopen, returnToBacklog, classify, confirmType, changeType, typeHistory, links, spawnChild, generatePlan, startPhase, subscribe), `solution.list|generate|choose`, `approval.list|listPending|approve|reject`, `backlog.requests|tasks|execute`. Thiếu: `approval.get`, `approval.cancel`, `request.flow`, `request.checks`; thiếu toàn bộ `clarification.*`, `decision.*`, `impact.*`, `readiness.*`, và kênh graph. Khung push `request.event` không được tham chiếu trong code (chỉ qua `subscribeRuntimeStreamChannel` với method `request.subscribe`, đúng hợp đồng mục 3).
- `RequestStatus` frontend dùng `'submitted'` và `'awaiting_information'`; hợp đồng dùng `'new'` và không có `awaiting_information`. Sẽ làm status `new` từ backend thành `unknown`.
- `RequestType` frontend có `'unknown'`, bỏ qua `type: null` của hợp đồng (parser đưa `null` thành `unknown`, chấp nhận được).
- Nguồn Request: hợp đồng phẳng `sourceProvider/sourceRef/sourceUrl/sourceSite`; `parseRequest` đọc object lồng `source{provider,ref,url,site}`, nên mọi nguồn thành `undefined` với dữ liệu thật. `returnCategory` bị bỏ.
- `ApprovalView`: hợp đồng `requestedBy, decidedBy, decidedAt, dueAt, subjectDigest, stage`; `parseApproval` đọc `approverId, approvedAt, expiresAt, updatedAt`. `dueAt` (hạn duyệt) và người quyết định bị mất; `subjectDigest` tuy đọc đúng nhưng nếu thiếu thì `expectedDigest` rỗng.
- `SolutionView`: hợp đồng `options` là object (`{options[], recommendation, assumptions, openQuestions}`), `chosenOption` chỉ số 0-based, không có `content/rejectionReason/reviewedAt`; `parseSolution` giả định `options` là mảng và có các trường kia. Task 020-01 (view model) dựa trên kiểu này nên sẽ phải sửa khi nối dữ liệu thật.
- Backlog: hợp đồng trả `requestRows` (BacklogRequestRowView) hoặc `groups` (BacklogGroupView); code đọc `items`.
- `RequestEvent` thiếu các trường phụ của `RequestEventFrame` (mục 4 ở trên).
- Nhiều task nói hook `useRequestEvents.ts`; code đặt tên `useRequestSubscription.ts`.
- Task 018-06 mô tả bỏ `backlog` ở 4 component và gọi `normalizeTask`; code chỉ làm phần kiểu và hàm chuẩn hoá.
- Task 019-01 yêu cầu `CHILD_REQUEST_RULES` ở registry; code đặt trong model.
- Tiêu chí hoàn thành của cả 54 file đều chưa tick, kể cả các task `[x]` (trạng thái và tiêu chí không đồng bộ).

## 6. Việc còn lại (ưu tiên)

1. Sửa lỗi gốc của wiring: probe `request.flowStatus` ở mức app (ví dụ trong `SidebarNav` hoặc App) và gọi `useRequestSubscription` để nút sidebar hiện và badge đếm hoạt động.
2. Đồng bộ parser và kiểu với CONTRACT (status `new`, nguồn phẳng, ApprovalView, SolutionView, khoá phản hồi backlog, `id` cho approve/reject, `pageToken`), rồi sửa test.
3. Sửa 7 lỗi tsc `ui.ts` (thêm `'requests'` vào kiểu `previousViewBefore*` hoặc chuẩn hoá điều hướng) và 2 lỗi tsc test; sửa 3 test đỏ (docblock `happy-dom`, khoá `cancelled`, vòng lặp `computeEffectiveParents`).
4. Hoàn tất 018-06: gọi `normalizeTask` trong `useTasks`/`useTask`; bỏ `backlog` khỏi 4 component và test cũ.
5. Làm 019 (list/detail/create), 022 (inbox), 023 (backlog) để các tab hết là placeholder; sau đó 020, 021.
6. i18n: thêm khoá `auto.components.request.*` vào 5 locale và test phủ locale.
7. 032 và 036 chưa bắt đầu: tạo token `--risk-*`, thay hex trong `TaskDAGView.tsx`, thêm kiểu và hook artifact/graph.
8. Bỏ dấu `[x]` ở 018-02..06, 019-01, 021-01 cho đến khi đủ.

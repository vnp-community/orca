# CR-REQ-018 — Nền frontend Request: kiểu, hook, store, định tuyến; gỡ `backlog` khỏi `TaskStatus`

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-018 |
| **Tên** | Nền frontend Request: kiểu dùng chung, lớp gọi RPC, hook, store slice, định tuyến và điều hướng; gỡ `backlog` khỏi `TaskStatus` |
| **Loại** | Feature + Refactor |
| **Priority** | 🔴 P0 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-REQ-016 (kênh WS `request.*`, `solution.*`, `approval.*`, `backlog.*`); mô hình dữ liệu của CR-REQ-002 và CR-REQ-009 (để khớp kiểu) |
| **Mở khoá** | CR-REQ-019, 020, 021, 022, 023 |
| **Tác động** | `frontend/src/shared/` (file mới), `frontend/src/renderer/src/{runtime,hooks,store,components/request,components/task,i18n}`; `frontend/src/shared/types.ts` (`TopLevelView`) |

---

## 1. Bối cảnh và vấn đề

Frontend chưa có gì cho Request, Solution, Approval, Backlog (README v6 mục 1). Năm CR UI sau cần chung một nền, nếu không mỗi CR sẽ tự định nghĩa kiểu, tự gọi RPC và tự xử lý lỗi theo cách riêng.

Hiện trạng cần xử lý (đã đọc code ngày 2026-10-05):

- Trang điều hướng không có router; một `activeView` trong slice `ui` (`store/slices/ui.ts`) quyết định màn hình, mỗi view lớp phủ có `previousViewBeforeX` và hàm `openXPage`/`closeXPage` (mẫu: `openTaskPage`). `TopLevelView` khai báo ở `shared/types.ts:3360`; `TOP_LEVEL_VIEW_LOOKUP` trong `ui.ts` bắt buộc đủ khoá, nếu thiếu là lỗi biên dịch.
- Mẫu gọi RPC: `callRuntimeRpc(getActiveRuntimeTarget(settings), method, params)`; lỗi là `RuntimeRpcCallError` có `code`. Hook `useTaskSource` nuốt mọi lỗi và không hiện gì khi runtime không có kênh; `useTaskPermission` đặt `isSupported=false`.
- `TaskStatus` (`shared/task-types.ts`) còn `backlog`, trong khi backend không có giá trị này. `backlog` còn xuất hiện ở `TASK_STATUS_PROGRESS`, `TaskBoardView.tsx` (`STATUS_ORDER`), `TaskDetail.tsx` (`TASK_STATUSES`), `TaskStatusBadge.tsx` (`STATUS_CONFIG`), `TaskDAGView.tsx` (`STATUS_COLORS`) và các test `TaskBoardView.test.tsx`, `TaskStatusBadge.test.tsx`. Quyết định D4 và O3 của README: gỡ.
- `TaskType` ở frontend là `epic|story|task|subtask|bug|spike`, còn backend là `task|bug|feature|epic` (+ `plan|phase` theo D2). Hai bên đã lệch trước v6.

## 2. Giải pháp đề xuất

### 2.1 Kiểu dùng chung (CR này sở hữu)

File mới trong `frontend/src/shared/` (tên theo khái niệm, không dùng `utils`/`common`):

| File (mới) | Nội dung |
|---|---|
| `request-types.ts` | `RequestType` (11 loại, README 3.2), `RequestStatus` (11 giá trị, 3.3), `RequestSize`, `RequestUrgency`, `TypeSource`, `RequestSourceProvider`, `ReturnedFromStage`, `OrcaRequest`, `RequestTypeHistoryEntry`, `RequestLink` (+ `RequestLinkReason`), `Solution`, `SolutionKind`, `SolutionStatus`, `SolutionOption`, `Approval`, `ApprovalStatus`, `ApprovalSubjectType`, `BacklogView`, `RequestBacklogItem`, `TaskBacklogItem`, `ExecuteBacklogItem` |
| `request-flow-registry.ts` | `REQUEST_FLOW_REGISTRY: Record<RequestType, {analysisKind: SolutionKind \| null; plan: 'plan'\|'task_list'\|'single_task'\|'none'; phase: 'always'\|'size_l'\|'none'; gates: ApprovalSubjectType[]}>`, bản sao đúng bảng 3.4; `REQUEST_STATUS_ORDER` (thứ tự 3.3) |
| `request-rpc-methods.ts` | hằng `REQUEST_RPC_METHODS` (tên kênh WS, xem 2.2) |
| `request-errors.ts` | `RequestRpcErrorKind`, `RequestRpcError` (xem 2.2) |

Quy ước: tên trường camelCase ở frontend, ánh xạ từ snake_case của README 3.5 tại một hàm duy nhất `parseRequest`, `parseSolution`, `parseApproval` (file `request-wire-parsers.ts`, mới). Parser chịu thiếu trường: giá trị enum lạ rơi vào `'unknown'` thay vì ném lỗi, để UI không vỡ khi backend thêm loại mới. `SolutionOption` chưa có schema trong README (xem mục 7); CR này định nghĩa tạm `{id, title, summary, pros[], cons[], effort?, risk?, recommended?}` và parser giữ phần dư trong `raw`.

Thêm `'plan' | 'phase'` vào `TaskType` (`shared/task-types.ts`). Không dọn lệch `story|subtask|spike` ở CR này (xem mục 7).

### 2.2 Lớp gọi RPC

`renderer/src/runtime/request-rpc-client.ts` (mới): `callRequestRpc<T>(method, params)` bọc `callRuntimeRpc`, dùng `getActiveRuntimeTarget(useAppStore.getState().settings)`; hỗ trợ cả runtime local và `environment` (SSH, remote) vì đi qua cùng `callRuntimeRpc`.

Bảng kênh (đề xuất, vì README 3.6 chỉ ghi tên nhóm; cần CR-REQ-016 xác nhận):

| Nhóm | Phương thức |
|---|---|
| `request.*` | `create`, `get`, `list`, `classify`, `confirmType`, `changeType`, `cancel`, `reopen`, `returnToBacklog`, `spawnChild`, `generatePlan`, `startPhase` |
| `solution.*` | `generate`, `list`, `chooseOption` |
| `approval.*` | `request`, `approve`, `reject`, `cancel`, `get`, `list`, `listPending` |
| `backlog.*` | `list` với `view` ∈ `request\|task\|execute` |

Phân loại lỗi (`RequestRpcError.kind`), ánh xạ từ `RuntimeRpcCallError.code`. Các mã thô chưa kiểm chứng; parser phải có nhánh `unknown`:

| `kind` | Điều kiện | Hiển thị |
|---|---|---|
| `unsupported` | `method_not_found` | Không hiện lỗi; hook trả `supported=false`, UI ẩn tính năng (như `task.getSource`) |
| `forbidden` | `forbidden` (xem `isRuntimeScopeForbiddenError`) | Toast `auto.hooks.request.error.forbidden`; nút ghi bị khoá cho phiên |
| `not_found` | `not_found` | Màn "Request không còn tồn tại", nút Về danh sách |
| `conflict` | `version_conflict`, `aborted` | Toast `error.conflict` và tự tải lại bản mới nhất |
| `invalid_state` | `failed_precondition` | Toast `error.invalidState` kèm trạng thái hiện tại; tải lại |
| `validation` | `invalid_argument` | Hiện lỗi cạnh trường nếu có `field`, nếu không thì toast |
| `network` | lỗi truyền tải, timeout | Banner `error.network` với nút Thử lại |
| `unknown` | còn lại | Toast chung `error.unknown` + mã thô trong chi tiết |

Mọi lệnh ghi gửi kèm `version` đã đọc (README: `requests.version`, `approvals.version`) để chống ghi chồng.

### 2.3 Hook (mới, trong `renderer/src/hooks/`)

| Hook | Trả về | Ghi chú |
|---|---|---|
| `useRequestFlowSupport()` | `'unknown' \| 'supported' \| 'unsupported'` | Thăm dò một lần mỗi runtime bằng `request.list {limit:1}`; cache theo `getActiveRuntimeTarget`; reset khi đổi runtime |
| `useRequests(filters)` | `{requests, isLoading, error, nextPage, refetch, supported}` | Lọc: `projectId`, `status[]`, `type[]`, `sourceProvider[]`, `reporterId`, `q`; phân trang bằng `nextPageToken` (giống `task.list`) |
| `useRequest(id)` | `{request, history, children, parent, isLoading, error, refetch}` | |
| `useRequestActions()` | các hàm `confirmType, changeType, cancel, reopen, returnToBacklog, spawnChild` | Mỗi hàm trả `Result<T, RequestRpcError>`, không ném |
| `useSolutions(requestId)` | `{solutions, isLoading, generate, chooseOption, refetch}` | Dùng ở CR-REQ-020 |
| `useApprovals(filters)` | `{approvals, pendingCount, approve, reject, refetch}` | `reject(id, comment)` ném `validation` nếu `comment` rỗng ngay ở client |
| `useBacklog(view, filters)` | `{items, isLoading, error, refetch, supported}` | Dùng ở CR-REQ-023 |
| `useRequestEvents()` | void | Gắn một lần ở `App.tsx`, xem 2.5 |

Mọi hook huỷ yêu cầu khi unmount (cờ `cancelled`, như `useTaskSource`), không dùng `any`.

### 2.4 Store slice

`store/slices/request.ts` (mới), đăng ký ở `store/index.ts` và `store/types.ts` như `createTaskSlice`. Mọi action trả object một phần (không mutate), theo ghi chú trong `slices/task.ts` về lỗi thay thế toàn state.

```
RequestSlice {
  requestFlowSupport: 'unknown'|'supported'|'unsupported'
  requestsById: Record<string, OrcaRequest>
  pendingApprovalCount: number
  requestPage: { section: 'requests'|'approvals'|'backlog'
                 requestId: string|null
                 backlogView: BacklogView
                 listFilters: RequestListFilters }
  upsertRequests(items) / removeRequest(id)
  setPendingApprovalCount(n) / setRequestFlowSupport(v)
  openRequestPage(data?: Partial<RequestPageData>) / closeRequestPage()
  setRequestPageSection(s) / setRequestPageRequest(id|null)
}
```

Danh sách và chi tiết Solution/Approval không đưa vào store (giữ trong hook, SWR cục bộ) để tránh trạng thái lệch; store chỉ giữ `requestsById` (để chip trạng thái ở nơi khác cập nhật khi có sự kiện) và `pendingApprovalCount` (cho chấm số ở sidebar).

### 2.5 Thời gian thực

`lib/request-event-bus.ts` (mới) theo mẫu `lib/mcp-event-bus.ts` (`subscribeRequestEvents`, `emitRequestEvent`). `useRequestEvents()` thử mở luồng sự kiện (nếu CR-REQ-016 cung cấp, mẫu `mcp.events.subscribe`); nếu không có luồng thì polling mỗi 15 giây chỉ khi cửa sổ hiển thị và `RequestPage` đang mở hoặc sidebar cần số chờ duyệt (poll `approval.listPending` limit 1 mỗi 60 giây). Sự kiện dùng tên README 3.7 (`request.status_changed`, `approval.requested`, `approval.decided`, `solution.proposed`, `plan.generated`, `phase.started`, `phase.completed`).

### 2.6 Định tuyến và điều hướng

- Thêm `'requests'` vào `TopLevelView` (`shared/types.ts`) và `TOP_LEVEL_VIEW_LOOKUP` (`ui.ts`). Thêm `previousViewBeforeRequests` theo mẫu Tasks.
- Một view duy nhất `requests` chứa ba phân đoạn Tabs: Requests, Hộp duyệt, Backlog (`ui/tabs.tsx`). Không thêm ba view cấp cao vì chúng chia sẻ bộ lọc dự án và dữ liệu.
- `components/request/RequestPage.tsx` (mới), lazy từ `App.tsx` như `TaskPage`.
- Sidebar: nút "Requests" cạnh nút Tasks (`components/sidebar/SidebarNav.tsx`), kèm chấm số `pendingApprovalCount` (tối đa `99+`). Ẩn nút khi `requestFlowSupport === 'unsupported'`. Cờ `request_flow_enabled` (CR-REQ-025): cách frontend đọc cờ là câu hỏi mở.
- Điều hướng sâu: `openRequestPage({section:'requests', requestId})` dùng từ thông báo, từ hộp duyệt, từ trang Tasks.
- Bổ sung `docs/ui/pages/requests.md` và mục trong `docs/ui/page-tree.md` (mới/sửa) khi triển khai.

Bố cục khung (CR-REQ-019/022/023 điền thân):

```
RequestPage
├─ RequestPageHeader (tiêu đề, bộ chọn dự án, nút "Tạo Request")
├─ Tabs: Requests | Hộp duyệt (số) | Backlog
│    ├─ RequestsTab    (CR-REQ-019)
│    ├─ ApprovalInboxTab (CR-REQ-022)
│    └─ BacklogTab     (CR-REQ-023)
└─ RequestUnsupportedNotice (khi supported=false, thay toàn thân)
```

Thành phần dùng chung do CR này cung cấp: `RequestStatusBadge`, `RequestTypeBadge`, `RequestSourceBadge` (mẫu `TaskSourceBadge`, chỉ link http/https), `ApprovalStatusBadge`, và `request-status-presentation.ts` (trạng thái → icon lucide + token màu). Chỉ dùng token đã có: `status-success` cho `completed`/`approved`, `destructive` cho `cancelled`/`rejected`, `primary` cho trạng thái đang chạy, `muted-foreground` cho còn lại. Không dùng emoji như `TaskStatusBadge` hiện tại.

### 2.7 Gỡ `backlog` khỏi `TaskStatus` (O3) và tương thích

Thứ tự thực hiện, trong cùng PR với phần kiểu:

1. `shared/task-types.ts`: bỏ `'backlog'` khỏi `TaskStatus` và khoá `backlog` khỏi `TASK_STATUS_PROGRESS`.
2. `TaskBoardView.tsx`: bỏ `'backlog'` khỏi `STATUS_ORDER`. `TaskDetail.tsx`: bỏ khỏi `TASK_STATUSES`. `TaskStatusBadge.tsx`: bỏ khỏi `STATUS_CONFIG`. `TaskDAGView.tsx`: bỏ khỏi `STATUS_COLORS`; cập nhật hai test liên quan.
3. Tương thích dữ liệu: backend không bao giờ trả `backlog` (migration `0003` chỉ có 6 giá trị), nhưng cache/test cũ hoặc runtime cũ có thể. Thêm `normalizeTaskStatus(raw)` trong `shared/task-status-normalization.ts` (mới): `backlog` → `open`, giá trị lạ → `open`. Gọi tại `useTasks`/`useTask` khi nhận dữ liệu. `TaskStatusBadge` đổi fallback từ `todo` sang `open`.
4. Cột Board `backlog` biến mất; việc "chưa làm" nay xem ở màn Backlog (CR-REQ-023). Ghi chú trong `docs/ui/pages` nếu có nhắc cột.
5. `todo` giữ nguyên (ngoài phạm vi; xem mục 7).

## 3. Quyết định thiết kế

- Một `requests` view với ba tab thay vì ba view: chia sẻ bộ lọc, giảm chạm vào `TopLevelView`.
- Lỗi phân loại một chỗ (`request-errors.ts`) để mọi màn hiển thị nhất quán; component không đọc `code` thô.
- Runtime không hỗ trợ kênh: không hiện lỗi, ẩn mục (đúng cách `useTaskSource`). Khác với `useTaskPermission`, không hiện cảnh báo vì Request là tính năng bật theo cờ.
- Không đặt danh sách Solution/Approval vào store: tránh hai nguồn sự thật; sự kiện chỉ kích hoạt `refetch`.
- Parser chịu enum lạ: backend và frontend phát hành lệch pha (đã gặp với `backlog`).
- Phím tắt: CR này không thêm phím tắt toàn cục; phím tắt thuộc từng màn (CR-REQ-019 và sau).

## 4. Tiêu chí chấp nhận

- [ ] `shared/request-types.ts`, `request-flow-registry.ts`, `request-errors.ts` tồn tại; `REQUEST_FLOW_REGISTRY` khớp 11 dòng của README 3.4 (test so từng loại).
- [ ] `TaskStatus` không còn `'backlog'`; `rg "'backlog'" frontend/src/renderer/src/components/task frontend/src/shared/task-types.ts` không còn kết quả ngoài test chuẩn hoá.
- [ ] `normalizeTaskStatus('backlog')` trả `open`; Board không mất task khi nhận `backlog` từ dữ liệu cũ.
- [ ] `TopLevelView` có `'requests'`; `openRequestPage()` đặt `activeView='requests'` và `closeRequestPage()` quay về `previousViewBeforeRequests`.
- [ ] Hydrate `activeView: 'requests'` từ trạng thái lưu: build không cờ vẫn rơi về `terminal` khi `requestFlowSupport==='unsupported'`.
- [ ] Runtime trả `method_not_found` cho `request.list`: nút sidebar ẩn, không toast, không log lỗi.
- [ ] `forbidden`, `not_found`, `conflict`, `invalid_state`, `network` đều hiển thị đúng chuỗi i18n tương ứng ở hook test.
- [ ] Chuỗi i18n có đủ 5 locale (`en, es, ja, ko, zh`) và test phủ khoá `request-locale-coverage.test.ts` xanh.
- [ ] Chấm số chờ duyệt ở sidebar cập nhật khi có `approval.requested`/`approval.decided` hoặc sau chu kỳ polling.
- [ ] Không có màu hex hay lớp màu Tailwind thô trong file mới (chỉ token).

## 5. Kiểm thử

- Unit: `request-flow-registry.test.ts` (đủ 11 loại, cổng đúng 3.4); `request-wire-parsers.test.ts` (enum lạ → `unknown`, thiếu trường); `request-errors.test.ts` (bảng ánh xạ mã); `task-status-normalization.test.ts`; slice `request.test.ts` (action trả object một phần, `openRequestPage`/`closeRequestPage`).
- Hook: `useRequests` (phân trang, huỷ khi unmount, `supported=false`), `useApprovals.reject` (comment rỗng bị chặn), `useRequestFlowSupport` (cache theo runtime, reset khi đổi runtime).
- Component: `RequestStatusBadge`/`RequestTypeBadge` (đủ giá trị, chỉ token), `RequestSourceBadge` (chặn `javascript:`), `SidebarNav.test.tsx` mở rộng (ẩn/hiện nút, chấm số).
- Cập nhật: `TaskBoardView.test.tsx`, `TaskStatusBadge.test.tsx`, `TaskDAGView.test.tsx`.
- E2E (khi CR-REQ-016 chạy): mở app với runtime không có kênh, runtime có kênh; chuyển tab, reload giữ `activeView`.
- Chưa chạy bất kỳ test nào; danh sách trên là kế hoạch.

## 6. Rủi ro và điểm chưa kiểm chứng

- Tên phương thức WS và mã lỗi thô là đề xuất, chưa đối chiếu với CR-REQ-016.
- Chưa kiểm chứng `callRuntimeRpc` trả `method_not_found` đúng cho kênh `request.*` chưa wire (đã thấy mã này trong `desktop-only-rpc-error-suppressor.ts` cho kênh khác).
- Gỡ `backlog` chạm 5 file task và 2 test; có thể còn tham chiếu ở `docs/` và e2e chưa rà.
- `TaskType` frontend lệch backend; thêm `plan|phase` có thể làm `switch` không đầy đủ ở nơi khác (cần rà bằng `tsc`).
- Luồng sự kiện thật chưa rõ; polling có thể gây tải nếu nhiều cửa sổ.

## 7. Câu hỏi mở

1. README 3.6 không liệt kê tên phương thức WS, không có kênh phát sự kiện (`request.events.subscribe`?) và không có mã lỗi. Cần CR-REQ-016 chốt.
2. README không nói frontend đọc cờ `request_flow_enabled` thế nào (setting, capability, hay chỉ dựa `method_not_found`).
3. Schema `Solution.options` (JSON) chưa định nghĩa; UI cần ít nhất `title`, `summary`, ưu/nhược, công sức, rủi ro.
4. Cần trường quyền theo người xem (ví dụ `viewerCan: {confirmType, decide, cancel}`) hay UI chỉ dựa lỗi `forbidden`?
5. Có dọn lệch `TaskType` (`story|subtask|spike` ở frontend, `feature` ở backend) trong series này không? Đề xuất để CR riêng.
6. `TaskStatus` frontend còn `todo` mà backend không có; README không nhắc.
7. Liên kết `[STYLEGUIDE.md](../../STYLEGUIDE.md)` trong README v6 và AGENTS.md trỏ `docs/STYLEGUIDE.md` không tồn tại; file thật là `guides/STYLEGUIDE.md`.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/README.md` (D4, O3, 3.2 đến 3.8)
- `/opt/repos/orca/guides/STYLEGUIDE.md`
- `/opt/repos/orca/frontend/src/shared/task-types.ts`, `/opt/repos/orca/frontend/src/shared/types.ts` (`TopLevelView`)
- `/opt/repos/orca/frontend/src/renderer/src/store/slices/ui.ts`, `slices/task.ts`, `store/index.ts`, `store/types.ts`
- `/opt/repos/orca/frontend/src/renderer/src/runtime/runtime-rpc-client.ts`, `runtime-rpc-result.ts`
- `/opt/repos/orca/frontend/src/renderer/src/hooks/useTaskSource.ts`, `useTaskPermission.ts`, `useTasks.ts`
- `/opt/repos/orca/frontend/src/renderer/src/lib/mcp-event-bus.ts`, `web/web-mcp-api.ts`
- `/opt/repos/orca/frontend/src/renderer/src/components/task/{TaskBoardView,TaskDetail,TaskStatusBadge,TaskDAGView,TaskSourceBadge}.tsx`
- `/opt/repos/orca/frontend/src/renderer/src/components/sidebar/SidebarNav.tsx`
- `/opt/repos/orca/frontend/src/renderer/src/i18n/task-jira-link-locale-coverage.test.ts` (mẫu test phủ khoá)
- Mới: `frontend/src/shared/request-*.ts`, `renderer/src/runtime/request-rpc-client.ts`, `renderer/src/store/slices/request.ts`, `renderer/src/components/request/*`

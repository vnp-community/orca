# FE-REQ-SOL-022: Hộp duyệt (Approval inbox) trong tab của `RequestPage`

> 📋 Proposed. Chưa triển khai, chưa chạy test nào. Ngày soạn 2026-10-06.

**CR:** [CR-REQ-022](../../../../../../docs/crs/v6/request-frontend/CR-REQ-022-approval-inbox.md)
**Backend liên quan:** CR-REQ-009 (`ApprovalService`), CR-REQ-010 (quyền, hạn, thông báo), CR-REQ-016 (kênh WS)
**Service/Area:** `frontend/src/renderer/src` (components, hooks, lib, i18n)
**TDD tham chiếu:** [02-state-management](../../../../tdd/v5/02-state-management.md) (slice Zustand, trả object một phần), [03-runtime-client-layer](../../../../tdd/v5/03-runtime-client-layer.md) (`callRuntimeRpc`, runtime local và environment/SSH), [15-task-graph-ui](../../../../tdd/v5/15-task-graph-ui.md) (mẫu hook, badge, test), [05-ui-components](../../../../tdd/v5/05-ui-components.md)
**Phụ thuộc:** FE-REQ-SOL-018 (kiểu, `callRequestRpc`, `useRequestEvents`, slice `request`, `RequestPage`, `RequestTypeBadge`), FE-REQ-SOL-020 (`RejectReasonDialog`), FE-REQ-SOL-019/021 (đích điều hướng sâu)

## 1. Trạng thái hiện tại (re-verify)

Đã đọc ngày 2026-10-06:

- `frontend/src/renderer/src/components/request/` chưa tồn tại. Chưa có `useApprovals`, `request-rpc-client.ts`, slice `request`: toàn bộ nằm ở CR-REQ-018 (chưa làm). Solution này chỉ dùng tên 018 đã chốt, chỗ chưa chắc ghi "(khớp SOL-018 khi merge)".
- Mẫu đếm hạn: `hooks/useMcpApprovalDeadline.ts` tick 1 Hz và trả ms còn lại. Hộp duyệt cần mức phút, nhiều hàng; không dùng lại trực tiếp (mỗi hàng một interval là lãng phí), viết `useMinuteClock` dùng chung một timer.
- `lib/mcp-relative-time.ts` (`formatMcpRelativeTime`) dùng `Intl.RelativeTimeFormat(undefined, ...)`, theo locale trình duyệt, không theo ngôn ngữ UI của app (`i18n.language`), và tên gắn MCP. Không dùng lại; tạo `lib/request-relative-time.ts`.
- Phím tắt gửi form: `lib/screen-submit-shortcut.ts` đã có `isScreenSubmitShortcut(event)` (Mac `metaKey`, nơi khác `ctrlKey`, bỏ qua `isComposing`) và `getScreenSubmitShortcutLabel()`. `components/ShortcutKeyCombo.tsx` nhận `keys: string[]`.
- Xác nhận ngắn: `components/confirmation-dialog.tsx` có `useConfirmationDialog()` trả `(options: {title, description?, confirmLabel?, cancelLabel?, confirmVariant?}) => Promise<boolean>`. Dùng cho "Duyệt nhanh", không viết dialog mới.
- Primitive có sẵn trong `components/ui/`: `toggle-group`, `table`, `skeleton`, `dialog`, `tabs`, `badge`, `button`, `tooltip`, `sonner`. Không có `alert`, `switch`: banner lỗi viết bằng `div` dùng token (mẫu: SOL-011 MCP).
- `mcp` approval (`components/mcp/`, `McpGlobalLayer`) là khái niệm khác, không dùng chung dữ liệu hay component.

### Correction relative to CR-REQ-022 (do đối chiếu CR-REQ-016, 009, 010)

| # | CR-REQ-022 viết | Thực tế theo CR backend | Xử lý trong solution này |
|---|---|---|---|
| C1 | `approval.approve {approvalId, version}` | CR-016: `approval.approve {id, expectedVersion, expectedDigest, comment?}`; `approval.reject {id, expectedVersion, expectedDigest, comment}` | Dùng tên của CR-016. Hàng phải mang `subjectDigest` và `version`; thiếu `subjectDigest` thì ẩn "Duyệt nhanh" (an toàn) |
| C2 | `approval.listPending` có thể có `total`, tiêu đề Request, loại | `ListPendingForUser` trả `Approval` (CR-009 mục 2.6) không kèm Request, không `total`; tham số chỉ `subjectType?, pageSize, pageToken` | Lấy tiêu đề/loại/dự án qua `request.get` theo lô nhỏ có cache (task 03). Lọc dự án, quá hạn, nhóm subject làm ở client trên các trang đã tải |
| C3 | `approval.requested`/`approval.decided` thêm/bỏ hàng | CR-016 chỉ có luồng `request.subscribe` → `request.event {requestId, eventType, status, type, occurredAt}`; không có `approvalId` | Nhận sự kiện chỉ để tải lại trang đầu (debounce), không vá hàng tại chỗ |
| C4 | Lỗi `invalid_state`/`conflict` luôn làm hàng biến mất | `REQUEST_APPROVAL_VERSION_CONFLICT` và `REQUEST_APPROVAL_DIGEST_MISMATCH` khi Approval còn `pending` nhưng nội dung đã đổi | Tách hai nhóm: "đã đóng" (hàng biến mất) và "nội dung đã đổi" (tải lại hàng, toast yêu cầu mở xem) |
| C5 | Mã lỗi `APPROVAL_*` (CR-016 mục 2.8) | CR-009/010 dùng tiền tố `REQUEST_APPROVAL_*` | Bộ phân loại khớp theo hậu tố, chấp nhận cả hai tiền tố |
| C6 | Lý do từ chối tối thiểu 10 ký tự | Backend chỉ bắt buộc không rỗng (`REQUEST_APPROVAL_COMMENT_REQUIRED`) | Giữ 10 ký tự ở client (chỉ siết thêm, không xung đột); đây là truyền tham số `minLength` cho `RejectReasonDialog` |
| C7 | Chấm số sidebar dùng `total` hoặc trang đầu | Không có `total` | Đếm bằng một lời gọi `pageSize=100`: số hàng, `99+` nếu `>99` hoặc còn `nextPageToken` (yêu cầu thay đổi cho SOL-018, mục 4) |

## 2. Giải pháp

### 2.1 Cây file

```
frontend/src/renderer/src/
├─ components/request/approval/                       (mới)
│   ├─ ApprovalInboxTab.tsx          lắp ghép, nạp dữ liệu, điều hướng
│   ├─ ApprovalInboxToolbar.tsx      ApprovalSubjectFilter + ProjectFilter + OverdueToggle
│   ├─ ApprovalSubjectFilter.tsx     ui/toggle-group
│   ├─ ApprovalList.tsx              nhóm theo Request, xử lý phím j/k/Enter
│   ├─ ApprovalRow.tsx               một Approval, ba hành động
│   ├─ ApprovalSubjectIcon.tsx       icon lucide theo subject_type (token màu)
│   ├─ ApprovalDueLabel.tsx          hạn tương đối, icon cảnh báo khi quá hạn
│   ├─ ApprovalInboxStates.tsx       Skeleton, Empty, EmptyFiltered, ErrorState
│   └─ approval-inbox-rules.ts       quy tắc thuần (sắp xếp, duyệt nhanh, nhóm lọc, đích mở)
├─ hooks/
│   ├─ useApprovalInbox.ts           (mới) listPending phân trang, polling, sự kiện, approve/reject
│   ├─ useRequestSummaries.ts        (mới) request.get theo lô, cache requestsById
│   ├─ useMinuteClock.ts             (mới) một timer 60s dùng chung
│   └─ useRowListKeyboardNavigation.ts (mới) j/k/Enter, dùng lại ở CR-REQ-023
├─ lib/
│   ├─ request-relative-time.ts      (mới) theo i18n.language
│   └─ approval-decision-outcome.ts  (mới) mã lỗi → kết cục UI
└─ i18n/request-approval-locale-coverage.test.ts   (mới)
```

Sửa: `components/request/RequestPage.tsx` (SOL-018) gắn `ApprovalInboxTab`; `components/sidebar/SidebarNav.tsx` chỉ đọc `pendingApprovalCount` (SOL-018 thêm chấm số). Tên file đều theo khái niệm, không có `helpers/utils/common`.

### 2.2 Quy tắc miền (`approval-inbox-rules.ts`, thuần, không React)

```ts
export type ApprovalSubjectGroup = 'all'|'requestType'|'solution'|'plan'|'phase'|'preDeploy'|'other'
export const SUBJECT_GROUP: Record<ApprovalSubjectType, Exclude<ApprovalSubjectGroup,'all'>> = {
  request_type:'requestType', solution:'solution', plan:'plan', task_list:'plan',
  phase:'phase', pre_deploy:'preDeploy', findings:'other', answer:'other', unknown:'other' }
// Duyệt nhanh: bảng 8 giá trị theo CR-022 2.3 (solution, plan: false)
export function canQuickApprove(a: Pick<Approval,'subjectType'|'subjectDigest'|'status'>): boolean
export function compareApprovals(now: number): (a: Approval, b: Approval) => number
  // quá hạn trước, rồi dueAt tăng (null cuối), rồi createdAt giảm, rồi id (ổn định)
export function openTargetFor(a: Approval): { section:'requests'; requestId:string; focus:'type_confirmation'|'analysis'|'plan' }
export function groupByRequest(rows: Approval[]): Array<{ requestId: string; rows: Approval[] }>
  // thứ tự nhóm = vị trí hàng đầu tiên của nhóm sau khi sắp xếp
```

Gộp nhóm `solution`/`findings`/`answer` vào đích `analysis`, `plan`/`task_list`/`phase`/`pre_deploy` vào `plan`, `request_type` vào `type_confirmation` (CR-022 mục 2.3). `focus` cần `requestPage.focus` trong slice (mục 4).

### 2.3 Dữ liệu và kênh WS

| Mục đích | Kênh | Tham số | Phản hồi dùng |
|---|---|---|---|
| Tải trang | `approval.listPending` | `{pageSize: 50, pageToken?, subjectType?}` | `{approvals: Approval[], nextPageToken}` |
| Đếm chấm số | `approval.listPending` | `{pageSize: 100}` | độ dài và `nextPageToken` |
| Duyệt nhanh | `approval.approve` | `{id, expectedVersion, expectedDigest}` | `{approval, requestStatus}` |
| Từ chối | `approval.reject` | `{id, expectedVersion, expectedDigest, comment}` | như trên |
| Tóm tắt Request | `request.get` | `{id}` | `OrcaRequest` (title, number, type, projectId) |
| Sự kiện | `request.subscribe` | `{}` (không `id`) | khung `request.event` |

`subjectType` gửi xuống server chỉ khi nhóm lọc ánh xạ đúng một giá trị (`requestType`, `solution`, `phase`, `preDeploy`); nhóm `plan` (plan + task_list) và `other` lọc ở client để không bỏ sót. Mọi lời gọi qua `callRequestRpc` của SOL-018 (đi qua `callRuntimeRpc`, nên chạy được trên runtime local, environment và SSH; không giả định cục bộ).

`useApprovalInbox({subjectGroup, projectId, overdueOnly})` trả `{rows, groups, isLoading, isLoadingMore, error, hasMore, loadMore, refetch, approve, reject, supported}`:

- Tải trang đầu khi mount và khi đổi bộ lọc server (`subjectType`).
- Polling 30 giây chỉ khi `document.visibilityState === 'visible'` và tab đang mở; sự kiện `request.event` có `eventType` chứa `approval.` thì `refetch()` (debounce 500 ms). Sự kiện đến từ `useRequestEvents` của SOL-018 qua `subscribeRequestEvents`; hộp duyệt không tự mở stream thứ hai.
- Huỷ yêu cầu khi unmount (cờ `cancelled`, như `useTaskSource`).
- Sau mỗi lần tải xong gọi `setPendingApprovalCount(...)` của slice.
- `approve/reject` trả `{outcome}` (không ném), outcome lấy từ `approval-decision-outcome.ts`.

### 2.4 Kết cục lỗi (`approval-decision-outcome.ts`)

Khớp theo hậu tố mã, bỏ tiền tố `REQUEST_`/không tiền tố:

| Mã | Outcome | UI |
|---|---|---|
| `APPROVAL_ALREADY_DECIDED`, `APPROVAL_EXPIRED`, `APPROVAL_NOT_FOUND`, `REQUEST_NOT_FOUND` | `closed` | Bỏ hàng; toast trung tính `ApprovalRow.alreadyDecided` (không đỏ). `NOT_FOUND` của Request dùng toast "Request không còn tồn tại" |
| `APPROVAL_VERSION_CONFLICT`, `APPROVAL_DIGEST_MISMATCH`, `APPROVAL_STAGE_MISMATCH` | `changed` | Tải lại hàng; toast `ApprovalRow.changed` kèm nút "Mở" |
| `APPROVAL_NOT_APPROVER`, `APPROVAL_FORBIDDEN`, `APPROVAL_SELF_APPROVAL_FORBIDDEN`, `APPROVAL_AGENT_FORBIDDEN` | `forbidden` | Toast lỗi, hàng giữ nguyên, tải lại danh sách |
| `APPROVAL_COMMENT_REQUIRED` | `validation` | Lỗi cạnh ô lý do |
| `method_not_found` (SOL-018 `unsupported`) | `unsupported` | Không toast; slice đặt `requestFlowSupport='unsupported'`, tab ẩn |
| lỗi truyền tải | `network` | Banner "Thử lại" |
| còn lại | `unknown` | Toast chung kèm mã thô |

### 2.5 Hành vi giao diện

- **Hàng (`ApprovalRow`)**: icon + nhãn `ApprovalSubjectType.<type>`, tiêu đề Request (hoặc `#id` rút gọn khi chưa nạp được), `RequestTypeBadge`, `requestedBy` (`system` hiển thị "AI/Hệ thống"), thời điểm tạo tương đối, `ApprovalDueLabel`. Nút: Mở (luôn), Duyệt nhanh (khi `canQuickApprove`), Từ chối.
- **Duyệt nhanh**: `useConfirmationDialog()` với `title` = tên đối tượng, `description` = hậu quả theo `subjectType` (khoá `ApprovalRow.confirmApprove.<type>`), `confirmVariant: 'default'`. Sau xác nhận gọi `approve`. Không có duyệt hàng loạt, không phím tắt cho Duyệt/Từ chối (CR-022 2.3).
- **Từ chối**: `RejectReasonDialog` (SOL-020) với `minLength=10`; `Mod+Enter` gửi dùng `isScreenSubmitShortcut`; nhãn `getScreenSubmitShortcutLabel()` hoặc `ShortcutKeyCombo keys={[isMac ? '⌘' : 'Ctrl', 'Enter']}`.
- **Phím**: `useRowListKeyboardNavigation` gắn `onKeyDown` vào container danh sách (chỉ khi tiêu điểm nằm trong danh sách): `j`/`k` đổi hàng đang chọn (roving tabindex), `Enter` gọi `onOpen(row)`. Bỏ qua khi `event.target` là `input`, `textarea`, `select`, `[contenteditable]`, khi có `ctrlKey/metaKey/altKey`, khi `isComposing`. Không phụ thuộc Mac hay Windows vì không có phím sửa đổi.
- **Hạn**: `useMinuteClock()` cập nhật `now` mỗi 60 s; `formatRelativeDue(dueAt, now, i18n.language)` ("còn 3 giờ", "quá hạn 2 ngày"). Quá hạn: icon `AlertTriangle` màu `text-destructive`, nằm đầu danh sách.
- **Trạng thái**: Skeleton 6 hàng; rỗng (`Inbox` `size-7`); rỗng khi lọc + "Xoá bộ lọc"; `network` banner + "Thử lại" giữ danh sách cũ mờ (`opacity-60`); `forbidden` thông điệp riêng; `unsupported` tab không tồn tại (do SOL-018 ẩn).
- **Tải thêm**: nút "Tải thêm" khi `hasMore`; hàng mới chèn có thể nằm trên hàng đang xem do sắp xếp ở client, chấp nhận (mục 6).

### 2.6 i18n

Khoá đọc theo tên (không băm) dưới `auto.components.request.approval.`, đủ 5 locale `en, es, ja, ko, zh`: `ApprovalInboxTab.{title,empty,emptyFiltered,clearFilters}`, `ApprovalSubjectFilter.{all,requestType,solution,plan,phase,preDeploy,other}`, `ApprovalRow.{open,approve,reject,overdue,dueIn,overdueBy,requestedBy,systemActor,confirmApprove.<type>,alreadyDecided,changed,requestGone}`, `OverdueToggle.label`, `ApprovalInboxErrorState.{network,forbidden,retry}`, `ApprovalSubjectType.<8 giá trị>`, `ApprovalInboxTab.loadMore`. Test `request-approval-locale-coverage.test.ts` theo mẫu `task-jira-link-locale-coverage.test.ts`.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Không giữ danh sách Approval trong store; chỉ `pendingApprovalCount` | CR-022 mục 3; tránh hai nguồn sự thật |
| D2 | Sự kiện chỉ kích hoạt `refetch` | `request.event` không mang `approvalId`; vá tại chỗ sẽ lệch |
| D3 | `request.get` theo lô nhỏ (đồng thời 4) và cache `requestsById` | Không có endpoint nhúng; mẫu `useTaskDependencyEdges` (`FETCH_CONCURRENCY = 4`). Ghi nhận N+1, xem Q1 |
| D4 | Lọc dự án, quá hạn, nhóm `plan`/`other` ở client | `listPending` chỉ nhận `subjectType`; tránh sửa backend ở CR này |
| D5 | Duyệt nhanh bắt buộc `subjectDigest` | Giữ đúng hợp đồng `expectedDigest`; thiếu thì chỉ cho "Mở" |
| D6 | Hook `useRowListKeyboardNavigation` tách file, dùng chung với CR-REQ-023 | Hai màn có cùng quy ước j/k/Enter |
| D7 | Không dùng `ExecutionEngineBadge`, `useMcpApprovalDeadline`, `formatMcpRelativeTime` | Sai miền hoặc sai locale (mục 1) |

## 4. Phụ thuộc và thứ tự

1. SOL-018 phải cung cấp: `callRequestRpc`, kiểu `Approval` có `version`, `subjectDigest`, `dueAt`, `createdAt`; `subscribeRequestEvents`; slice có `pendingApprovalCount`, `setPendingApprovalCount`, `requestsById`, `upsertRequests`; `openRequestPage`.
2. **Yêu cầu bổ sung cho SOL-018** (ghi tại đây để người điều phối chuyển): (a) `requestPage.focus?: 'type_confirmation'|'analysis'|'plan'`; (b) lời gọi đếm chấm số dùng `pageSize=100` thay vì `limit 1`; (c) `Approval.subjectDigest` trong parser.
3. SOL-020 phải xuất `RejectReasonDialog` với props tối thiểu `{open, onOpenChange, title, minLength, onSubmit(comment): Promise<void>}` (khớp SOL-020 khi merge).
4. SOL-019/020/021 phải có tab đích tương ứng với `focus`; nếu chưa, "Mở" rơi về tab chi tiết Request mặc định.
5. Backend: CR-REQ-016 (`approval.*`, `request.subscribe`), CR-REQ-009/010 (dữ liệu và quyền). Phát triển trước bằng fake client; E2E chỉ sau khi các kênh chạy.

Thứ tự task: 022-01 → (022-02, 022-03, 022-04 song song) → 022-05 → 022-06 → 022-07. Chi tiết ở `tasks/PARTIAL-INDEX-022-023.md`.

## 5. Kiểm thử

Lệnh (chạy từ gốc repo, `frontend/package.json` chỉ có `test`, `test:watch`; tên workspace `orca-frontend`; chưa chạy): `pnpm --filter orca-frontend test <đường dẫn>`.

- Unit: `approval-inbox-rules.test.ts` (bảng 8 giá trị duyệt nhanh, sắp xếp, nhóm, đích mở), `request-relative-time.test.ts`, `approval-decision-outcome.test.ts` (cả hai tiền tố), `useRowListKeyboardNavigation.test.tsx`.
- Hook: `useApprovalInbox.test.tsx` (phân trang, polling, sự kiện, huỷ khi unmount, `unsupported`), `useRequestSummaries.test.tsx` (giới hạn đồng thời, cache, lỗi một Request không làm hỏng cả lô).
- Component: `ApprovalRow.test.tsx`, `ApprovalList.test.tsx`, `ApprovalInboxTab.test.tsx`, mở rộng `SidebarNav.test.tsx`.
- i18n: `request-approval-locale-coverage.test.ts`.
- E2E (cần CR-REQ-009/010/016 chạy): hai người dùng, người A duyệt, người B thấy hàng biến mất; mở từ hộp duyệt tới Solution.

## 6. Rủi ro và điểm chưa kiểm chứng

- Tên trường camelCase của `approvalView` chưa được CR-016 liệt kê; parser phải chịu thiếu trường (SOL-018).
- N+1 `request.get`: với 50 hàng thuộc nhiều Request có thể 50 lời gọi; giới hạn đồng thời 4 và cache. Cần backend nhúng tóm tắt nếu đo thấy chậm.
- Sắp xếp phía client chỉ đúng trên các trang đã tải; "Tải thêm" có thể chèn hàng quá hạn lên đầu.
- Chấm số cần lời gọi nền; nhiều cửa sổ nhân tải. Chưa đo.
- Chưa kiểm chứng `callRuntimeRpc` trả `method_not_found` đúng cho kênh `approval.*` chưa wire.
- Lệnh `pnpm lint` và `typecheck` ở root trỏ `config/tsconfig.*.json` không có trong repo hiện tại; chưa kiểm chứng đường chạy typecheck (thử `pnpm --filter orca-frontend exec tsc --noEmit -p tsconfig.json`).

## 7. Câu hỏi mở

1. Backend có nhúng `requestTitle`, `requestType`, `projectId`, tóm tắt chủ thể vào `approval.listPending` không (CR-022 Q1)? Nếu có, bỏ `useRequestSummaries` trong hộp duyệt.
2. `request_type` duyệt nhanh bằng `approval.approve` hay bắt buộc qua `request.confirmType` (CR-009 nói "một đường duy nhất" là `ConfirmRequestType`, CR-016 nói bắt buộc người)? Solution này dùng `approval.approve`; cần CR-005/009 xác nhận không bỏ sót `size`, `urgency`.
3. Tên trường `focus` và các giá trị đích do SOL-019/020/021 chốt.
4. Có cần tab "Đã xử lý gần đây" (CR-022 Q2)? Không nằm trong solution này.
5. Quyền theo `team:<id>` (CR-010): hộp duyệt tin vào `ListPendingForUser`, không tự tính.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/request-frontend/CR-REQ-022-approval-inbox.md`, `CR-REQ-018-request-frontend-foundation.md`
- `/opt/repos/orca/docs/crs/v6/gateway-and-mcp/CR-REQ-016-api-gateway-request-channels.md` (2.3, 2.4, 2.5, 2.8)
- `/opt/repos/orca/docs/crs/v6/approval/CR-REQ-009-generic-approval-domain-and-api.md`, `CR-REQ-010-approval-authorization-notification-expiry.md`
- `/opt/repos/orca/frontend/src/renderer/src/hooks/useMcpApprovalDeadline.ts`, `hooks/useTaskSource.ts`, `hooks/useTaskDependencyEdges.ts`
- `/opt/repos/orca/frontend/src/renderer/src/lib/screen-submit-shortcut.ts`, `lib/mcp-relative-time.ts`
- `/opt/repos/orca/frontend/src/renderer/src/components/ShortcutKeyCombo.tsx`, `components/confirmation-dialog.tsx`
- `/opt/repos/orca/frontend/src/renderer/src/components/sidebar/SidebarNav.tsx`, `SidebarNav.test.tsx`
- `/opt/repos/orca/frontend/src/renderer/src/i18n/task-jira-link-locale-coverage.test.ts` (mẫu)
- `/opt/repos/orca/guides/STYLEGUIDE.md`

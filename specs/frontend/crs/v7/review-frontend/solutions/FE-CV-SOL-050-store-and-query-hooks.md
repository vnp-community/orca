# FE-CV-SOL-050-store-and-query-hooks: Phân giải selector, slice `code-intel`, luồng push, hook hỗ trợ/truy vấn/index/reindex

> Trạng thái (2026-10-07): 09-14: 6 DONE. Lệch: hằng kênh/kiểu/mã lỗi ban đầu không khớp CONTRACT-codeintel-ui-api và đã được viết lại theo hợp đồng.

**CR:** [CR-CV-050](../../../../../../docs/crs/v7/review-frontend/CR-CV-050-review-frontend-foundation.md) (phần 2.4, 2.5, 2.6, 2.7)
**Area:** frontend (`frontend/src/renderer/src/{store/slices,hooks,lib}`)
**Hợp đồng áp dụng:** [`CONTRACT-codeintel-ui-api.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md) (U7, U8, §2.4 timeout và thử lại, §3.1 `status/reindex/reindexStatus/subscribe/settings.get`, §4.1, §5 push và quy tắc client, §6 cờ), [`CONTRACT-codeintel-proto-and-data-map.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md) (PQ-01, PQ-04, PQ-08, PQ-11, PQ-13, PQ-16, PQ-24; §7 G3/G4; §9 O-1).
**TDD tham chiếu:** [v5/02-state-management §3, §7 và Addendum MCP slices](../../../../tdd/v5/02-state-management.md), [v5/03-runtime-client-layer §2, Addendum `mcp.*`](../../../../tdd/v5/03-runtime-client-layer.md), [v5/07-hooks-and-ipc §15 Key Hook Patterns và Addendum hooks MCP](../../../../tdd/v5/07-hooks-and-ipc.md); [storage/README (không có `persist`, không IndexedDB)](../../../../storage/README.md).

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `store/index.ts` (:55-57 import, :124-126 spread slice MCP), `store/types.ts` (:53-55, :119-121), `store/slices/store-test-helpers.ts` (`createTestStore` :61, import từng slice :11-50, **không** có slice MCP), `store/slices/mcp-slice.ts` (`startMcpEvents` :231-248, `streamRefs`), `store/slices/mcp-reconnect.ts` (:5-15: base 1 s, max 30 s, slow 5 phút, khoẻ 30 s), `store/slices/worktrees.ts` (`buildWorktreePurgeState` :1960; gọi tại :2454, :2519, :2593, :4988; `removeWorktree` :3725-3742 với `prunePullRequestGenerationRecords?.(…)`), `store/slices/connectivity-status.ts` (:72 `isConnectivityLikeRpcError`, :86 `connections`), `lib/window-visibility-interval.ts` (:11 `installWindowVisibilityInterval`), `runtime/runtime-compatibility-cache.ts` (:205 `runtimeEnvironmentSupportsCapability`), `lib/worktree-runtime-owner.ts` (:162 `getRuntimeEnvironmentIdForWorktree`), `store/slices/generation-records-worktree-removal-leak.test.ts`, `shared/types.ts` (:247, :482-499), `test-support/mcp-test-store.ts`.

Xác nhận đúng như CR-CV-050:

- Không có `persist` Zustand hay IndexedDB trong renderer (storage/README mục "Headline findings" 1): cache Review **chỉ trong bộ nhớ**; trạng thái bền đi qua `codeIntel.reviewState.*` (SOL-052).
- `store-test-helpers.ts` liệt kê từng slice (thiếu thì test rò rỉ không dựng được store): slice mới **phải** thêm ở đây.
- `buildWorktreePurgeState` được gọi từ bốn nơi (:2454, :2519, :2593, :4988) và `removeWorktree` là đường riêng (:3737): hai đường cần `pruneCodeIntelWorktrees`.
- Mẫu luồng dùng chung ref-count: `startMcpEvents` (`streamRefs`, `teardownStream`).

**Correction relative to CR-CV-050 (hợp đồng thắng):**

| # | CR-050 ghi | Hợp đồng / mã thật | Quyết định |
|---|---|---|---|
| 1 | Cờ lấy từ capability `code-intel.v1` rồi thăm dò `codeIntel.status` | UI-API §6: **`settings.get`** lúc khởi động và mỗi 60 s khi tab Review mở; `effective.codeIntelEnabled=false` ⇒ ẩn mọi lối vào, **không gọi kênh nào khác**, không mở `subscribe`; `qualityGateEnabled=false` ⇒ ẩn phần chất lượng; capability tuỳ chọn | `useCodeIntelSupport` chỉ dựa `settings.get` (task 050-12); không thăm dò `status` |
| 2 | `codeIntelSupportByEnvironment` suy từ lỗi (`method_not_found` ⇒ unsupported…) | `CODEINTEL_UNAVAILABLE` (`CODE_INTEL_SERVICE_ADDR` rỗng) ⇒ `unsupported`; `CODEINTEL_DISABLED` ⇒ `disabled`; `CODEINTEL_NOT_AUTHORIZED` ở `settings.get` (phiên thiết bị, U8) ⇒ `disabled` ngầm | Bảng ánh xạ ở 050-12 |
| 3 | `changed` ⇒ xoá cache và tăng `codeIntelResyncCounter` (mọi lens tải lại) | UI-API §5: `changed` ⇒ huỷ cache worktree + đánh dấu chip index cũ, **không tự tải lại giữa lúc tương tác** (chip "Có dữ liệu mới"); chỉ `resync:true`/mở lại luồng mới tải lại view đang xem (`codeIntelResyncCounter`) | Hai bộ đếm: `codeIntelStaleSignalByWorktree` (nhắc người dùng) và `codeIntelResyncCounter` (buộc tải lại) |
| 4 | Backoff chậm dần tới 5 phút | UI-API §5: backoff 1 s → 30 s; gateway không tự mở lại luồng | Hằng số riêng `code-intel-stream-reconnect.ts`: 1 s → 30 s, luồng sống ≥ 30 s là khoẻ |
| 5 | `status` mỗi 30 s, `reindexStatus` mỗi 2 s khi polling | UI-API §5: **thêm** `quality.run` mỗi 2 s (nhóm quality) | Polling của solution này chỉ gồm `status` và `reindexStatus`; để chỗ cắm cho `quality.run` |
| 6 | `loadCodeIntelIndexStatus` đọc `IndexStatus[]` | `IndexStatus` đơn, `overall` 9 giá trị, `indexBasis[]`, `activeJob?`, `binding` | State theo `IndexStatus` đơn |
| 7 | Không có xử lý `CODEINTEL_TIMEOUT` | UI-API §2.4: `CODEINTEL_TIMEOUT | {"retryAfterMs":3000,"inProgress":true}` rồi hoàn tất nền; client tự thử lại tối đa **90 s** tổng | `useCodeIntelQuery` tự thử lại (050-13) |
| 8 | `subscribe` theo môi trường có worktree mở Review | PQ-11: một `codeIntel.subscribe` mỗi kết nối WS; **lần hai thay lần đầu** | Một luồng mỗi môi trường, **không** gửi `selectors` (mọi worktree người dùng đọc được); lọc theo worktree ở client |

## 2. Hợp đồng áp dụng (trích)

| Việc | Mục |
|---|---|
| `selector` = `{projectId, worktreeId}` ở mọi kênh; O-1 | PQ-04, UI-API §2.1, proto-and-data-map §9 O-1 |
| Cờ và phát hiện tính năng | UI-API §6, U7, U8, PQ-01, PQ-24 |
| Push, quy tắc client, resync, polling dự phòng | UI-API §5, PQ-11 |
| Timeout và thử lại | UI-API §2.4, PQ-13 |
| Reindex: trạng thái job, `outcome`, `percent: null` | UI-API §4.1 `ReindexJob`, PQ-16; cooldown `CODEINTEL_REINDEX_COOLDOWN` (UI-API §2.3) |
| Worktree id: `<repoId>::<path>` hợp lệ; `::workspace:` bị từ chối | PQ-04 |

## 3. Lệch giữa CR và hợp đồng

Bảng "Correction" mục 1 (8 dòng). Điểm mới ngoài CR: `reindex` có cooldown 5 phút sau job thành công (`CODEINTEL_REINDEX_COOLDOWN {retryAfterSeconds}` ⇒ `rate-limited`), CR-050/051 chưa có trạng thái này; hook reindex phải mang `cooldownUntil`.

## 4. Giải pháp

### 4.1 Cây file

```
frontend/src/renderer/src/
  lib/code-intel-worktree-selector.ts        (mới) resolveCodeIntelSelector(state, worktreeId)
  lib/code-intel-event-bus.ts                (mới, mẫu mcp-event-bus.ts) subscribe/emit; nuốt lỗi từng listener
  store/slices/code-intel.ts                 (mới) CodeIntelSlice + CODE_INTEL_WORKTREE_KEYED_STATE_KEYS
  store/slices/code-intel-stream-reconnect.ts (mới) hằng số và nextCodeIntelReconnectDelayMs
  store/slices/store-test-helpers.ts         (sửa) thêm createCodeIntelSlice
  store/index.ts, store/types.ts             (sửa) đăng ký slice
  store/slices/worktrees.ts                  (sửa) gọi pruneCodeIntelWorktrees ở HAI đường
  hooks/useCodeIntelSupport.ts, useCodeIntelQuery.ts, useCodeIntelPagedQuery.ts,
        useCodeIntelIndexStatus.ts, useCodeIntelReindex.ts, useCodeIntelEvents.ts   (mới)
  App.tsx                                    (sửa) gắn useCodeIntelEvents một lần
  test-support/code-intel-test-store.ts      (mới, mẫu mcp-test-store.ts)
```

### 4.2 Phân giải selector (O-1)

```ts
type CodeIntelSelectorResult =
  | { kind: 'ok'; projectId: string; worktreeId: string; environmentId: string }
  | { kind: 'unsupported'; reason: 'no-project' | 'no-environment' | 'workspace-scope' | 'unknown-worktree' }
resolveCodeIntelSelector(state: AppState, worktreeId: string | null | undefined): CodeIntelSelectorResult
```

Quy tắc đã đọc: `environmentId = getRuntimeEnvironmentIdForWorktree(state, worktreeId)` (null cho terminal nổi và folder workspace, `worktree-runtime-owner.ts:162-189`; null ⇒ `no-environment`, vì đích local không có gateway: UI-API §7); `projectId = worktree.projectId ?? repo.projectId` (cả hai tuỳ chọn, `shared/types.ts:247,487`; thiếu ⇒ `no-project`); id kiểu `workspace`/`folder` ⇒ `workspace-scope` (gateway từ chối `CODEINTEL_WORKTREE_REF_UNSUPPORTED`); chuẩn hoá bỏ tiền tố `id:` nếu có. Chưa kiểm chứng: có repo "kiểu project-service" nào có `projectId` ở môi trường thật không (O-1 mở).

### 4.3 Slice

```ts
CodeIntelSlice {
  codeIntelSupportByEnvironment: Record<envId, { state: 'unknown'|'enabled'|'disabled'|'unsupported'
      flags: { codeIntel: boolean; qualityGate: boolean; securityScan: boolean; aiReview: boolean }; checkedAt: number }>
  codeIntelIndexStatusByWorktree: Record<worktreeId, { status: IndexStatus | null; loadedAt: number; loading: boolean; error: CodeIntelErrorSnapshot | null }>
  codeIntelReindexByWorktree: Record<worktreeId, { jobId: string; status: ReindexJob['status']; mode; stage: string; percent: number | null
      message: string; startedAt: number; outcome?: string; errorCode?: string; cooldownUntil?: number }>
  codeIntelResultsByWorktree: Record<worktreeId, Record<queryKey, CachedCodeIntelResult>>   // LRU 16 mục / worktree, giữ ≤ 6 worktree
  codeIntelStaleSignalByWorktree: Record<worktreeId, { reason: PushChanged['reason']; at: number; headCommit?: string }>
  codeIntelEventsState: 'off' | 'connecting' | 'on' | 'polling'
  codeIntelResyncCounter: number
  // actions (trả object một phần, không mutate): loadCodeIntelIndexStatus, applyCodeIntelEvent, invalidateCodeIntelWorktree,
  // readCodeIntelResult, writeCodeIntelResult, startCodeIntelEvents, pruneCodeIntelWorktrees, resetCodeIntel
}
```

`CachedCodeIntelResult = { envelopeMeta: Omit<CodeIntelEnvelope<unknown>,'data'>; data: unknown; cachedAt: number; headCommit: string | null; stale: boolean }`. `queryKey = "<view>|<scopeKey>|<paramsHash>"`; `scopeKey` do caller truyền (SOL-051 dựng từ phạm vi). Giới hạn theo số mục (chưa phản ánh byte; xem rủi ro). `CODE_INTEL_WORKTREE_KEYED_STATE_KEYS` liệt kê khoá theo `worktreeId`; SOL-051/052/053/055/056 **bổ sung** khoá của chúng vào hằng này.

Dọn rò rỉ: `pruneCodeIntelWorktrees(liveIds)` gọi ở `removeWorktree` (cạnh `:3737`, optional chain như các slice khác) **và** thêm bản đồ đã lọc vào object trả về của `buildWorktreePurgeState` (mẫu `rightSidebarTabByWorktree` :2056-2062, :2262). Huỷ timer/hàng chờ module-level theo worktree. Test rò rỉ duyệt chính hằng `CODE_INTEL_WORKTREE_KEYED_STATE_KEYS` (mẫu `generation-records-worktree-removal-leak.test.ts`, `bulk-worktree-purge-terminal-maps-leak.test.ts`).

### 4.4 Luồng push

`startCodeIntelEvents(environmentId)` ref-count một luồng mỗi môi trường; chỉ mở khi `codeIntelSupportByEnvironment[env].flags.codeIntel === true`; mở bằng `window.api.codeIntel.subscribeEvents`. Quy tắc xử lý (UI-API §5): `changed` ⇒ `invalidateCodeIntelWorktree` (xoá cache, đặt `codeIntelStaleSignalByWorktree`, **không** tăng counter) trừ khi `resync:true`/`reason in (resync, overflow)` ⇒ tăng `codeIntelResyncCounter`; `reindexProgress` ⇒ `codeIntelReindexByWorktree` (`percent: null` giữ nguyên null); `quality.*` ⇒ chỉ phát lên bus (nhóm quality tiêu thụ). Luồng đóng ⇒ mở lại với backoff 1 s → 30 s và tăng counter khi nối lại thành công. `onUnsupported` hoặc lỗi `CODEINTEL_RATE_LIMITED` (vượt `CODE_INTEL_MAX_STREAMS`) ⇒ `codeIntelEventsState='polling'`: `status` mỗi 30 s cho worktree Review đang hiển thị và `reindexStatus` mỗi 2 s khi job `running`, qua `installWindowVisibilityInterval`.

### 4.5 Hook

| Hook | Trả về |
|---|---|
| `useCodeIntelSupport(environmentId?)` | `{state, flags, environmentId, refresh}`; gọi `settings.get` khi khởi động, mỗi 60 s **chỉ khi** có tab Review mở (ref-count `retainCodeIntelSettingsPolling`) |
| `useCodeIntelQuery(worktreeId, view, params, opts)` | `{data, meta, status: 'idle'\|'loading'\|'ready'\|'error', error, stale, truncated, staleSignal, refetch, applyNow}` |
| `useCodeIntelPagedQuery(worktreeId, view, params, {select})` | `{items, pages, hasNextPage, fetchNextPage, isFetchingNextPage, …}` cho `structure`, `dataFlows`, `findings`, `routes` |
| `useCodeIntelIndexStatus(worktreeId)` | `{status: IndexStatus \| null, overall, stale, tools, indexBasis, activeJob, loading, error, refresh}` |
| `useCodeIntelReindex(worktreeId)` | `{start(mode='incremental'), job, isStarting, isRunning, cooldownUntil, error}` |
| `useCodeIntelEvents()` | `void`; gắn một lần ở `App.tsx` |

`useCodeIntelQuery`: đọc cache trước; `enabled=false` hoặc `support.state !== 'enabled'` ⇒ không gọi; thử lại khi `kind==='timeout'` có `data.inProgress` (chờ `data.retryAfterMs ?? 3000`, tổng ≤ 90 s, sau đó `error`); huỷ khi gỡ (cờ `cancelled` như `useTaskSource.ts`); không `any`; kết quả sai hình dạng ⇒ `error.kind='tool-failed'` không vỡ UI; không gửi `ifNoneMatch`. `applyNow()` xoá `staleSignal` và tải lại (nút chip "Có dữ liệu mới").

`useCodeIntelReindex.start`: đặt `isStarting=true` **đồng bộ** (khoá nút ngay, rubric SSH), gọi `reindex {mode}`; `CODEINTEL_REINDEX_IN_PROGRESS {jobId}` ⇒ gắn vào job đó, không báo lỗi; `CODEINTEL_REINDEX_COOLDOWN {retryAfterSeconds}` ⇒ `cooldownUntil`; tiến độ từ push, dự phòng `reindexStatus`; kết thúc `succeeded` ⇒ `status {refresh:true}` và tăng counter.

## 5. Quyết định thiết kế

- Cache trong store có giới hạn (không Map module): cùng đường dọn khi xoá worktree, có test rò rỉ.
- Hai tín hiệu dữ liệu cũ (nhắc so với buộc tải lại) để không giật màn đang đọc (UI-API §5).
- Một luồng mỗi môi trường, không selectors: tránh `subscribe` lần hai thay lần đầu (PQ-11).
- Sao chép (không import) hằng backoff; `mcp-reconnect.ts` là singleton MCP.
- Cờ `settings.get` là nguồn duy nhất; lỗi đọc cờ coi như `unknown` (không ẩn vĩnh viễn, không mở luồng).
- SSH/remote: mọi lời gọi qua bridge theo `environmentId` của worktree; trạng thái tải chịu 50-200 ms.

## 6. Phụ thuộc chéo khu vực

| Cần | Ở đâu | Cổng |
|---|---|---|
| `settings.get`; cờ tenant | `BE-CV-SOL-073-settings-flag-and-rollout`, `BE-CV-SOL-013-authorization-flags-and-audit` | G3 (fake trước) |
| `status`, `reindex`, `reindexStatus` | `BE-CV-SOL-040-codeintel-view-channels`, `BE-CV-SOL-012-index-status-aggregation` | G3 |
| `subscribe` và sự kiện `changed/reindexProgress` | `BE-CV-SOL-040-codeintel-write-and-stream-channels`, `BE-CV-SOL-024-event-distribution`; agent: `AG-CV-SOL-004-reindex-and-index-notifications` | G3, đợt 4 |
| `reindex` cooldown 5 phút | `BE-CV-SOL-012-index-status-aggregation` (chủ job) | — |

Trước G3 dùng fake backend của FE-CV-TASK-073-02 (`code-intel-fake-backend.ts`; 050 không tạo bản thứ hai), kể cả push giả. Cờ: `useCodeIntelSupport` (hook này) và `useQualityFeatureFlags` (FE-CV-TASK-085-01) là hai hook đã có chủ; không tạo hook cờ thứ ba.

## 7. Tiêu chí chấp nhận

- [ ] `resolveCodeIntelSelector` trả `unsupported` (không gọi mạng) cho worktree không có `projectId`, terminal nổi, folder workspace; trả đủ `{projectId, worktreeId, environmentId}` cho worktree hợp lệ.
- [ ] Slice có trong `store/index.ts`, `types.ts`, `store-test-helpers.ts`; cache ≤ 16 mục/worktree, ≤ 6 worktree.
- [ ] Xoá worktree bằng `removeWorktree` **và** bằng `buildWorktreePurgeState` xoá mọi khoá của `CODE_INTEL_WORKTREE_KEYED_STATE_KEYS`; worktree khác không ảnh hưởng.
- [ ] `settings.get` `effective.codeIntelEnabled=false` ⇒ không `subscribe`, không kênh nào khác được gọi; `qualityGateEnabled=false` ⇒ `flags.qualityGate=false`.
- [ ] `changed` thường không tải lại view đang xem mà chỉ đặt tín hiệu; `resync:true` hoặc nối lại ⇒ tải lại; luồng chỉ mở một lần dù nhiều caller; thất bại ⇒ polling.
- [ ] `CODEINTEL_TIMEOUT` có `inProgress` ⇒ thử lại tự động tối đa 90 s tổng; `percent: null` hiển thị "chưa biết".
- [ ] `useCodeIntelReindex.start` khoá ngay; xử lý `IN_PROGRESS` và `COOLDOWN` đúng; không toast.
- [ ] Không hex; không `max-lines` disable; mọi chuỗi qua `translate()` (nếu có).

## 8. Kiểm thử

`lib/code-intel-worktree-selector.test.ts`; `store/slices/code-intel.test.ts` (mẫu `mcp-slice.test.ts`: `vi.hoisted` + `vi.mock` client; LRU, ref-count luồng, backoff bằng đồng hồ giả, polling); `store/slices/code-intel-worktree-removal-leak.test.ts`, `code-intel-bulk-purge-leak.test.ts`; `lib/code-intel-event-bus.test.ts`; `hooks/useCodeIntelSupport.test.tsx`, `useCodeIntelQuery.test.tsx`, `useCodeIntelPagedQuery.test.tsx`, `useCodeIntelIndexStatus.test.tsx`, `useCodeIntelReindex.test.tsx` (`// @vitest-environment happy-dom`, Testing Library). Chạy `pnpm --dir frontend test`. Chưa chạy.

## 9. Rủi ro và điểm chưa kiểm chứng

- `Worktree.projectId`/`Repo.projectId` tuỳ chọn: worktree cũ không có `projectId` sẽ không dùng được Review (O-1); chưa đo tỷ lệ.
- Cache giới hạn theo số mục, không theo byte (một kết quả ~1 500 nút có thể nặng); chưa đo.
- `settings.get` mỗi 60 s thêm một lời gọi mỗi môi trường khi Review mở (SSH ~200 ms) — chấp nhận.
- Hành vi `subscribe` lần hai thay lần đầu chỉ theo hợp đồng, chưa chạy với gateway thật.
- `useCodeIntelEvents` ở `App.tsx` chạm file lớn (~127 KB); thay đổi tối thiểu một dòng, kiểm bằng `impact` GitNexus trước khi sửa (theo CLAUDE.md, chưa chạy).

## 10. Câu hỏi mở

1. Mỗi môi trường có một luồng, nhưng một web client có thể giữ nhiều môi trường: giới hạn số luồng mở đồng thời?
2. Worktree không có `projectId`: hiển thị gì (ẩn lối vào hay thông báo "chưa gắn dự án")? Đề xuất ẩn (không lỗi), ghi vào telemetry (CR-095).
3. Khi `settings.get` lỗi tạm thời: giữ trạng thái cũ bao lâu?

## 11. Danh sách task

| Task | Tên | Priority |
|---|---|---|
| [FE-CV-TASK-050-09](../tasks/FE-CV-TASK-050-09-worktree-selector-resolution.md) | `resolveCodeIntelSelector` (O-1) | P0 |
| [FE-CV-TASK-050-10](../tasks/FE-CV-TASK-050-10-code-intel-slice-cache-and-worktree-purge.md) | Slice, cache LRU, dọn rò rỉ hai đường | P0 |
| [FE-CV-TASK-050-11](../tasks/FE-CV-TASK-050-11-event-stream-reconnect-bus-and-polling.md) | Luồng push, backoff, bus, polling dự phòng | P0 |
| [FE-CV-TASK-050-12](../tasks/FE-CV-TASK-050-12-use-code-intel-support-and-settings-polling.md) | `useCodeIntelSupport` qua `settings.get` | P0 |
| [FE-CV-TASK-050-13](../tasks/FE-CV-TASK-050-13-use-code-intel-query-and-paged-query.md) | `useCodeIntelQuery` và `useCodeIntelPagedQuery` | P0 |
| [FE-CV-TASK-050-14](../tasks/FE-CV-TASK-050-14-use-code-intel-index-status-and-reindex.md) | `useCodeIntelIndexStatus` và `useCodeIntelReindex` | P0 |

Thứ tự: 050-09 → 050-10 → 050-11, 050-12, 050-13 (song song) → 050-14. 050-09 chặn 050-07 (SOL-050-types-and-runtime-bridge).

## 12. Tham chiếu

`/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-ui-api.md`, `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md`, `/opt/repos/orca/docs/crs/v7/review-frontend/CR-CV-050-review-frontend-foundation.md`, `/opt/repos/orca/frontend/src/renderer/src/store/slices/{mcp-slice,mcp-reconnect,connectivity-status,worktrees,store-test-helpers}.ts`, `/opt/repos/orca/frontend/src/renderer/src/store/{index,types}.ts`, `/opt/repos/orca/frontend/src/renderer/src/lib/{worktree-runtime-owner,window-visibility-interval}.ts`, `/opt/repos/orca/frontend/src/shared/types.ts`, `/opt/repos/orca/specs/frontend/storage/README.md`.

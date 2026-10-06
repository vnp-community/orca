# FE-CV-SOL-087-quality-scorecard-and-state: Kiểu, state, hook, scorecard cổng, chạy kiểm tra, lens `quality`

> 📋 Proposed. Chưa triển khai. Viết ngày 2026-10-06 từ khảo sát code `frontend/src` và hợp đồng v7; chưa chạy test, build hay ứng dụng. Solution 1/3 của CR-CV-087 (hai solution còn lại: [diff annotations](./FE-CV-SOL-087-quality-diff-annotations.md), [trend/coverage/hotspot](./FE-CV-SOL-087-quality-trend-coverage-hotspot.md)).

**CR:** [CR-CV-087](../../../../../../docs/crs/v7/quality-visualization/CR-CV-087-quality-frontend-scorecard-and-annotations.md) (mục 2.1-2.5, 2.9, 2.11)
**Area:** frontend (`frontend/src/shared`, `frontend/src/renderer/src`)
**Làm sau:** FE-CV-SOL-088 (nền đồ hoạ), FE-CV-SOL-050-* (bridge, slice, hook, fake backend), FE-CV-SOL-051-review-workspace-shell (registry lens, header). Pha 1 của CR (scorecard, chạy kiểm tra) cần backend 081/082/085 ở dạng thật; trước đó dùng **fake backend G4** (`code-intel-fake-backend.ts`).
**TDD tham chiếu:** [v5/02-state-management](../../../../tdd/v5/02-state-management.md) (mẫu slice, mục 3 và cascade/leak test mục 7), [v5/03-runtime-client-layer](../../../../tdd/v5/03-runtime-client-layer.md), [v5/05-ui-components](../../../../tdd/v5/05-ui-components.md); API/storage: `specs/frontend/api/rpc-catalog.md`, `storage/browser-storage-catalog.md` (kết luận: state chất lượng **chỉ trong bộ nhớ**, không `localStorage`).

## 0. Hợp đồng áp dụng, lệch, phụ thuộc chéo

**Hợp đồng áp dụng:**

| Mục | Dùng cho |
|---|---|
| `CONTRACT-codeintel-ui-api.md` §3.2 (20 kênh `codeIntel.quality.*`; PQ-27) | 087-01, 087-03; tên và tham số thật (xem bảng 2.2) |
| §4.7 `QualityFinding/Step/Run/Gate/Waiver/RunnableProfile/Profile`, `CiComparison` (PQ-33, PQ-34, PQ-25) | 087-01 (mirror), 087-05 |
| §2.3 bảng lỗi + `^(CODEINTEL_[A-Z0-9_]+): (.*?)(?: \| (\{.*\}))?$` (PQ-02, PQ-03) | 087-01; kind `quality-disabled`, `profile-unknown`, `env-not-ready`, `run-in-progress`, `run-cancelled`, `rate-limited`, `conflict`, `forbidden`, `offline` |
| §2.4 timeout (đọc 20 s, ghi 8 s; `quality.summary` ngoại lệ) và `CODEINTEL_TIMEOUT` hậu tố `inProgress` | hook thử lại (CR-050 sở hữu cơ chế; 087 không lặp) |
| §5 push `quality.progress|finished|gateChanged` mang `projectId`, `worktreeId` (PQ-11); quy tắc client | 087-02 |
| §6 cờ: `settings.get` là nguồn chuẩn; không dò `typeof window.api.codeIntel` | 087-03 `useQualitySupport` |
| §4.7 "Quy tắc hiển thị bắt buộc"; U9 | 087-04, 087-05 |
| §2.5 quyền: `quality_read`, `review_write` (start/cancel), `quality_waive` | 087-06 xử lý `forbidden` tách đọc/chạy/miễn trừ |
| PQ-04 (`{projectId, worktreeId}`), O-1 | Bridge CR-050 gắn `projectId`; 087 chỉ truyền `worktreeId` cho hook |
| PQ-06 (hai nguồn tách, một dock) | không trộn danh sách (ở solution 2) |
| PQ-36/U8: phiên thiết bị bị từ chối | không có việc mobile ở solution này |

**Lệch giữa CR và hợp đồng** (hợp đồng thắng):

| # | CR-087 ghi | Hợp đồng | Quyết định |
|---|---|---|---|
| 1 | Kiểu `QualityFinding/Run/Gate` tự định nghĩa (2.1); thiếu `column`, `inScope`, `waiver` chỉ "đề xuất", `QualityRun` thiếu cờ cây bẩn | §4.7: có `column/endColumn`, `inScope`, `waiver?`, `fpVersion`, `stepId`; `QualityRun` có `dirty`, `workTreeChangedDuringRun`, `scopeWidened`, `treeFingerprint`, `steps[]`, `indexBasis[]`; `QualityGate.basedOn` có `headCommit`, `evaluatedAt`, `profileVersion` | Copy nguyên từ §4.7 (không dùng bản CR); giải rủi ro "không có cờ dirty" và "không có cột" |
| 2 | `quality.gate {scopeKey?}`, `quality.findings {offset, severity[]}`, `quality.waive {fingerprint, reason, expiresAt, note?}` | `quality.gate {base?, profileName?, turnKey?, record?, includeWaivedDetail?}` → `{gate, waivers, evaluatedAt, profileDefinitionDigest, comparison}`; `quality.findings {runId?, severities?, categories?, file?, inScope?, limit≤500, pageToken?}`; `quality.waive {subjectKind, subjectKey, action?, reason 1..1000, expiresAt ≤ 30 ngày, scope?}` | Theo hợp đồng; phân trang bằng `pageToken` (không offset); `expiresAt` tối đa 30 ngày (CR cho 90) |
| 3 | `quality.cancel`, danh sách profile, `worktreeId` trong push, `unwaive` là "thiếu" (CR 1.2, 7.5) | Có: `quality.cancel`, `quality.profile.get` trả `runnableProfiles[]`, push mang `worktreeId`, `action:'revoke'` | Dùng cả bốn; bỏ cơ chế ánh xạ `runId → worktree` dự phòng; có "Bỏ miễn trừ" thật |
| 4 | `useQualitySupport` dò bằng thăm dò `quality.gate` hoặc capability | §6: `settings.get.effective.qualityGateEnabled`; `CODEINTEL_QUALITY_GATE_DISABLED` ẩn phần chất lượng; `CODEINTEL_DISABLED` ẩn toàn bộ | `useQualitySupport` đọc settings từ CR-050; hai mã lỗi trên chuyển trạng thái về `disabled` |
| 5 | `QualityGate.reasons[].check` tự do, "bấm lý do để lọc" cần quy ước (7.3) | PQ-34: `reasons[]` có `code`, `params`, `category?`, `tool?` | Lọc theo `category`/`tool`; nhãn theo `code`+`params` qua `translate()`, fallback `check` |
| 6 | Cũ khi `basedOn.stale` hoặc `QualityRun.headCommit ≠ HEAD` hoặc `indexCommit ≠ HEAD` | `basedOn.stale` do backend tính theo `freshness.indexMustMatchHead` của profile | Cũ = `basedOn.stale` hoặc `headCommit` của run ≠ HEAD hiện tại; `indexCommit ≠ HEAD` chỉ hiển thị thông tin trong dòng nguồn (không banner) |
| 7 | Cảnh báo trước tạo review (CR 2.10) là việc của CR-087 | `FE-CV-SOL-085-source-control-quality-notice` sở hữu (§8.2 hợp đồng; CR-CV-085 2.9b định nghĩa `use-source-control-quality-gate.ts`, `source-control-quality-gate-notice.tsx`, prop `qualityNotice`) | **087 theo FE-CV-SOL-085 và không làm khe `preSubmitNotice` của CR-087**: tên khe chốt là `qualityNotice`; điểm gọi composer sản phẩm thật là `SourceControl.tsx:5247` và `ChecksPanel.tsx:3611` (`renderPullRequestComposer` là hàm test, `pr-create-dialog` là code chết); chỉ cung cấp `QualityGateChip`, `GateVerdictBadge`, lens deep link. Cả hai CR cùng đề xuất file `source-control-quality-gate-notice.tsx`: giữ bản của 085 |
| 8 | `quality.progress.percent` có thể `null` (CR README điểm 6) | §5: `percent: number \| null` | Spinner tĩnh khi `null` (không sửa `progress.tsx`) |
| 9 | Trạng thái kết thúc `succeeded|failed|cancelled` | `quality.finished.status` còn `interrupted` (QualityRun.status không có) | Xử lý `interrupted` như lần chạy không hoàn tất, nhãn riêng; báo mâu thuẫn hợp đồng (câu hỏi mở 1) |
| 10 | Quyền chạy kiểm tra chưa có capability (7.6) | `RunnableProfile.ready/missing` có sẵn; quyền: `forbidden` lúc gọi | Vẫn chưa có capability quyền; dùng `ready/missing` cho môi trường (không phải quyền) |

**Phụ thuộc chéo khu vực:**

| Cần | Từ |
|---|---|
| Kênh thật `codeIntel.quality.*` + push | `BE-CV-SOL-040-codeintel-quality-channels`, `BE-CV-SOL-040-codeintel-write-and-stream-channels` (subscribe) |
| `QualityRun`, `QualityFinding` (parser/fingerprint) | `BE-CV-SOL-082-quality-run-storage-and-ingest`; agent `AG-CV-SOL-081-quality-runner-core`, `AG-CV-SOL-081-quality-profile-catalog-and-preflight`, `AG-CV-SOL-082-quality-parsers-and-fingerprint` |
| `QualityGate`, waiver, trend | `BE-CV-SOL-085-quality-gate-evaluator-and-profiles`, `BE-CV-SOL-085-waivers-and-trend` |
| `CiComparison` | `BE-CV-SOL-086-ci-run-merge-and-comparison` |
| Cờ/`settings.get` | `BE-CV-SOL-073-settings-flag-and-rollout` (và CR-050 store) |
| Bridge, `useCodeIntelQuery`, `applyCodeIntelEvent`, `classifyCodeIntelError`, `CODE_INTEL_WORKTREE_KEYED_STATE_KEYS`, fake backend | `FE-CV-SOL-050-types-and-runtime-bridge`, `FE-CV-SOL-050-store-and-query-hooks`, `FE-CV-SOL-050-review-tab-wiring` |
| Registry lens, `ReviewHeaderBar`, `ReviewScope`, `usePerceivedLoadingStage`, `gitBranchCompareSummaryByWorktree` | `FE-CV-SOL-051-review-workspace-shell` |
| Deep link từ Source Control | `FE-CV-SOL-061-review-entry-points`, `FE-CV-SOL-085-source-control-quality-notice` |
| Gate store dùng chung với 085-FE | **087-02 sở hữu `selectQualityGate`/`loadQualityGate`**; `FE-CV-SOL-085-source-control-quality-notice` nên đọc từ đó (nếu 085-FE xong trước, 087-02 thừa kế hook của nó; điều phối ở câu hỏi mở 2) |
| Fake backend G4 | CR-050/CR-073 T2: mở rộng cho `quality.*` ở 087-08 |

## 1. Trạng thái hiện tại (Re-verify)

Đã đọc: `frontend/package.json`; `components/ui/` (liệt kê); `components/ui/progress.tsx` (26 dòng); `hooks/usePrefersReducedMotion.ts`; `components/right-sidebar/CreateHostedReviewComposer.tsx` (props :48-79; `createDisabled` tính trong component; chỉ dùng để xác nhận rằng khe thuộc 085); `components/editor/DiffViewer.tsx`; `store/slices/` (thư mục: có `editor.ts`, `diffComments.ts`, `agent-status.ts`... **không** có slice code-intel/quality); `lib/screen-submit-shortcut.ts` (`isScreenSubmitShortcut` có ở nhiều file dùng); `i18n/i18n.ts`; `i18n/task-jira-link-locale-coverage.test.ts`; `/opt/repos/orca/guides/STYLEGUIDE.md`.

**Xác nhận:**
- Không có chuỗi `codeIntel`, `quality` hay `review-map` nào trong `frontend/src` (grep `codeIntel` rỗng; `components/review-map/` và `components/quality-charts/` không tồn tại). Toàn bộ là mới và phụ thuộc CR-050/051.
- `ui/progress.tsx` không hỗ trợ giá trị không xác định và luôn có `transition-all duration-300 ease-out` ⇒ không dùng cho `percent: null`.
- Thư mục `store/slices/` dùng mẫu `StateCreator<AppState, [], [], Slice>` (TDD 02 mục 3); test rò rỉ dạng `*-worktree-removal-leak.test.ts` có sẵn (`generation-records-worktree-removal-leak.test.ts`, `editor-state-worktree-purge-leak.test.ts`).
- Công cụ a11y tự động: không có; kiểm bằng test cấu trúc.
- Hook `usePrefersReducedMotion` có; dùng cho spinner tĩnh (`motion-reduce:animate-none` là lớp CSS đủ; hook chỉ khi cần logic).

**Correction relative to CR-087:** bảng "Lệch" mục 0 là danh sách chính; thêm: (a) CR nói `Select` profile "mặc định = profile của cổng nếu có": `QualityGate.profile` là `"<name>@<scope>/v<version>"`, nên tên profile phải **tách** trước khi so với `RunnableProfile.id` (chưa chốt `id` có trùng `<name>` không; câu hỏi mở 3); (b) CR cho 4 khối `Collapsible` và danh sách trong lens; giữ danh sách ở dock (solution 2).

## 2. Giải pháp

### 2.1 Cây file

```
frontend/src/shared/
  code-intel-quality-types.ts           (mới) mirror §4.7: QualityFinding, QualityStep, QualityRun, QualityGate, QualityWaiver,
                                        RunnableProfile, QualityProfile, QualityTrendPoint, CoverageReport, CiComparison, GateResult
  code-intel-quality-wire-parsers.ts    (mới) parse tolerant: enum lạ -> 'unknown', số âm -> 0, mảng thiếu -> []
  code-intel-quality-errors.ts          (mới) parse hậu tố JSON của lỗi chất lượng (missing[], available[], runId, maxDays)
frontend/src/shared/code-intel-rpc-methods.ts, code-intel-errors.ts, code-intel-wire-parsers.ts   (sửa; CR-050 sở hữu file)
frontend/src/renderer/src/store/slices/code-intel-quality-state.ts   (mới) createCodeIntelQualityState(set, get)
frontend/src/renderer/src/store/slices/code-intel.ts                 (sửa, CR-050) trải đoạn state; thêm khoá vào CODE_INTEL_WORKTREE_KEYED_STATE_KEYS
frontend/src/renderer/src/hooks/useQualitySupport.ts, useQualityGate.ts, useQualityRun.ts, useQualityProfiles.ts   (mới)
frontend/src/renderer/src/components/review-map/quality/
  QualityLens.tsx  QualityLensToolbar.tsx  QualityRunControl.tsx  QualityRunProgress.tsx
  QualityScorecard.tsx  QualityGateVerdictHeader.tsx  QualityGateReasonRow.tsx  QualityStepList.tsx
  QualityProvenanceLine.tsx  QualityCiComparisonRow.tsx  QualityGateChip.tsx  QualityStateScreen.tsx
  quality-lens-blocks.ts               registry khối cho solution 3 (id, titleKey, lazy component, phase)
  quality-gate-copy.ts  quality-stale-model.ts  quality-run-scope-model.ts  quality-view-state.ts  quality-profile-selection.ts
```

Tên theo khái niệm; không `helpers/utils`; không `max-lines` disable. `components/review-map/` là thư mục CR-050/051 (chưa tồn tại); nếu FE-CV-SOL-051 đặt tên khác thì theo đó (chưa kiểm chứng).

### 2.2 Kênh (đúng §3.2; mọi kênh nhận `{projectId, worktreeId, ...}` do bridge điền)

| Kênh | Tham số dùng | Kết quả dùng | Nơi dùng |
|---|---|---|---|
| `quality.gate` | `base?`, `profileName?` | `gate`, `waivers`, `evaluatedAt`, `comparison` | scorecard, chip, 085-FE |
| `quality.runs` | `limit` (≤ 50), `source?` | `runs[]` | dòng nguồn, gắn lại lần chạy đang diễn ra |
| `quality.run` | `runId` | `QualityRun` | polling dự phòng, chi tiết bước |
| `quality.start` | `profile`, `scope`, `base?` | `{run}` | `QualityRunControl` |
| `quality.cancel` | `runId` | `{run}` | huỷ |
| `quality.profile.get` | `name?` | `{profile, origin, version, runnableProfiles}` | chọn profile, ngưỡng (solution 3) |

(Các kênh `findings`, `waive`, `trend`, `coverage` ở solutions 2 và 3.) Không dùng `ifNoneMatch` (kênh chất lượng trả đối tượng phẳng, **không** phong bì `CodeIntelEnvelope`; xem rủi ro 6.2).

### 2.3 State (`code-intel-quality-state.ts`)

```ts
type CachedQuality<T> = { data: T; fetchedAt: number; stale: boolean }
type ActiveQualityRun = { runId: string | null; profile: string; scope: QualityRun['scope']
  phase: 'starting' | 'queued' | 'running' | 'cancelling' | 'finished'
  status?: QualityRun['status'] | 'interrupted'
  stage: string; percent: number | null; stepIndex?: number; stepCount?: number; message: string; startedAt: number }
type QualityWorktreeState = {
  run: ActiveQualityRun | null
  gate: CachedQuality<{ gate: QualityGate; waivers: QualityWaiver[]; evaluatedAt: string; comparison: CiComparison[] }> | null
  runs: CachedQuality<QualityRun[]> | null
  profiles: CachedQuality<{ profile: QualityProfile; origin: 'repo'|'tenant'|'builtin'; version: number; runnable: RunnableProfile[] }> | null
  ui: { profile: string | null; runScope: QualityRun['scope']; source: 'structure' | 'quality'; annotationsOn: boolean
        severity: QualityFinding['severity'][]; category: QualityFinding['category'][]; onlyInScope: boolean; showWaived: boolean
        selectedFingerprint: string | null; openBlocks: string[] }
}
// state: codeIntelQualityByWorktree: Record<worktreeId, QualityWorktreeState>
// actions: startQualityRun, cancelQualityRun, loadQualityGate, loadQualityRuns, loadQualityProfiles, setQualityUi, invalidateQuality, attachQualityRun
```

- Mỗi worktree giữ **một** bản mỗi loại (không LRU), nên tổng bộ nhớ bị chặn theo cấu trúc; danh sách phát hiện/trend/coverage ở solutions 2 và 3 có khoá riêng nhưng cùng khoá `codeIntelQualityByWorktree` để cùng đường prune.
- `codeIntelQualityByWorktree` thêm vào `CODE_INTEL_WORKTREE_KEYED_STATE_KEYS` (CR-050 2.5) ⇒ hai đường xoá worktree (`removeWorktree`, `buildWorktreePurgeState`) tự dọn; test rò rỉ theo mẫu `*-worktree-removal-leak.test.ts`. Huỷ timer polling theo worktree khi prune.
- Sự kiện (mở rộng `applyCodeIntelEvent` của CR-050): `quality.progress` → nếu `worktreeId` trùng và (`run.runId` trùng hoặc `run` rỗng/`finished`) thì `attachQualityRun` (nhận cả lần chạy do client khác/tự động khởi chạy, nhờ push mang `worktreeId`) rồi cập nhật `stage/percent/message`; `quality.finished` → `phase:'finished'`, `status`, `invalidateQuality`, tải lại `gate`, `runs` và (nếu có người xem) `findings`; `quality.gateChanged` → chỉ tải lại `gate`; `codeIntel.changed` → đánh dấu `gate.stale=true` rồi tải lại `gate` (không xoá kết quả).
- Tải lại lười: chỉ nạp thứ đang có người xem (lens/dock/diff/PR notice).
- Mất push: khi `codeIntelEventsState` là `polling` hoặc sau `codeIntelResyncCounter` tăng: gọi `quality.run {runId}` mỗi 2 s cho lần chạy `queued|running|cancelling`; dừng khi kết thúc (hợp đồng §5, "dự phòng polling").
- Dữ liệu không lưu vào `localStorage`; `ui.*` chỉ trong bộ nhớ (reset khi tải lại cửa sổ; lựa chọn "chỉ per-viewer" có thể thêm sau, chưa cần).

### 2.4 Hook

`useQualitySupport()` → `'unknown'|'enabled'|'disabled'|'unsupported'`: `enabled` khi `useCodeIntelSupport()==='enabled'` **và** `settings.effective.qualityGateEnabled`; `CODEINTEL_QUALITY_GATE_DISABLED` ⇒ `disabled`; `CODEINTEL_UNAVAILABLE`/`unsupported` ⇒ `unsupported`; hai trạng thái sau **không hiển thị gì** (không tab, không chip, không toast). Không đọc `typeof window.api.codeIntel`. `useQualityGate(worktreeId)`, `useQualityRun(worktreeId)` (máy trạng thái), `useQualityProfiles(worktreeId)`; mọi hook không gọi mạng khi `enabled=false`, huỷ khi gỡ, mảng/đối tượng trả về ổn định (selector có `shallow`).

### 2.5 Scorecard

```
┌ Chất lượng · Profile [full ▾]  Phạm vi [Tệp đã đổi ▾]  [▶ Chạy kiểm tra]  ───────────────────────────┐
│ Nguồn: Cục bộ · chạy 14:02 · HEAD a41c9e0 · index d819812      ⚠ Kết quả cũ: HEAD hiện tại 9f3c2ab │
├──────────────────────────────────────────────────────────────────────────────────────────────────┤
│ ┌ CỔNG: CÓ CẢNH BÁO (chế độ chỉ báo, không chặn) ─────────────┐ ┌ Diff coverage (solution 3) ──┐ │
│ │ ⊗ Lỗi 2   ▲ Cảnh báo 5   ○ Thông tin 9     [thanh chồng]      │ │                              │ │
│ │ ⊗ typecheck  2 lỗi  ngưỡng 0           Chưa đạt     ▸ xem     │ └──────────────────────────────┘ │
│ │ ▲ coverage   diff 62% ngưỡng 80%         Cảnh báo   ▸ xem     │                                  │
│ │ ✓ lint       0 lỗi  ngưỡng 0             Đạt          (đã miễn trừ 1)                              │
│ │ ? security   chưa chạy                   Chưa rõ                                                   │
│ │ ▸ Các bước đã chạy (6): lint ✓ · typecheck ⊗ · test ⏱ hết thời gian · coverage ⚠ ...               │
│ └────────────────────────────────────────────────────────────────────────────────────────────┘   │
│ Cục bộ so với CI: cục bộ đạt, CI không đạt (không thể coi là đạt)                    [Xem CI ↗]     │
└──────────────────────────────────────────────────────────────────────────────────────────────────┘
```

Quy tắc hiển thị (test cấm từ khoá): `pass` → "Các kiểm tra của profile {profile} đều đạt" (không "an toàn", "sạch"); `warn` → "Có cảnh báo"; `fail` → "Chưa đạt"; `unknown` → "Chưa đủ dữ liệu để kết luận" kèm lý do đầu tiên hoặc "Backend không trả lý do"; `mode:'inform'` → "Chế độ chỉ báo: không chặn tạo review đã gửi"; `mode:'block'` → "Cấu hình chặn; giao diện hiện chỉ cảnh báo" (O9). Không một "điểm số". `waivedCount` hiển thị "(đã miễn trừ N)". `QualityStepList` hiển thị từng `QualityStep` (trạng thái bằng biểu tượng + chữ, `failureKind` (`format_drift`, `output_too_large`, `exit_unexpected`, `parser_error`, `env`) và `envReason`): `failed|timeout|env_not_ready` **không** được trình bày thành "0 phát hiện". Banner thêm khi `QualityRun.dirty`, `workTreeChangedDuringRun`, `scopeWidened` ("Cây làm việc có thay đổi chưa commit/ đổi trong lúc chạy; số dòng có thể lệch", "Phạm vi được mở rộng so với yêu cầu"). `outsideScopeCount > 0` → "N phát hiện ngoài phạm vi đã đổi (không tính vào cổng)" (chỉ nêu, vì cách backend tính nằm ở 085). `CiComparison.relation` (9 giá trị, PQ-25) có copy riêng; `local_pass_ci_fail` luôn hiển thị `reasonsHint[]`, **không bao giờ** dùng biểu tượng `pass` cho kết luận chung.

### 2.6 Chạy kiểm tra

Máy trạng thái và thang thời lượng như CR-087 2.5 với điều chỉnh theo hợp đồng: chọn từ `runnableProfiles` (`ready:false` → nút khoá và liệt kê `missing[] {check, reason, hint?}` ngay trên thanh, không cần chờ lỗi `ENV_NOT_READY`; `heavy` → chữ "nặng"; `scopes` giới hạn lựa chọn phạm vi); `starting` khoá nút ngay, spinner sau ≥ 200 ms (từ xa) hoặc ≥ 100 ms (cục bộ), nhãn giữ độ rộng cố định; `running`: `stage`, `message`, `Progress` khi `percent` là số, **spinner tĩnh** khi `null`, `stepIndex/stepCount` nếu có; nút "Huỷ" ghost (UX rule 3); `cancelling`: không khẳng định đã huỷ cho tới khi `finished.status==='cancelled'`; `finished`: `succeeded` tải lại; `failed|interrupted` → "Lần chạy không hoàn tất" + chi tiết sao chép được; `cancelled` → trung tính. Lỗi: `CODEINTEL_RUN_IN_PROGRESS` (data `runId`) → gắn vào lần chạy, không báo lỗi; `CODEINTEL_PROFILE_UNKNOWN` (data `available[]`) → làm mới danh sách, chọn mục hợp lệ đầu tiên; `CODEINTEL_ENV_NOT_READY` (data `missing[]`, `reason?`) → inline, nút "Kiểm tra lại"; `RATE_LIMITED|CONCURRENCY_LIMIT` (data `retryAfterSeconds?`) → inline; `forbidden` → khoá chạy, vẫn đọc ("Bạn không có quyền chạy kiểm tra", không dựng trước quyền).

### 2.7 Lens `quality`

Đăng ký trong `REVIEW_LENS_DEFINITIONS` (FE-CV-SOL-051) sau tab "Hợp đồng", chỉ khi `useQualitySupport()==='enabled'`; `React.lazy` để `quality-charts` không vào chunk khởi động; thêm `'quality'` vào `ReviewLensId` và một khe `trailing` ở `ReviewHeaderBar` cho `QualityGateChip` (hai sửa nhỏ ở file thuộc CR-051). Thứ tự khối: thanh công cụ chạy, dòng nguồn, scorecard, rồi `QUALITY_LENS_BLOCKS` (`Collapsible`; solution 3 đăng ký Phủ test, Xu hướng, Hotspot, Phụ thuộc). Danh sách phát hiện ở dock (solution 2), lens có liên kết "Xem {n} phát hiện". Bảng trạng thái (`quality-view-state.ts`): chưa chạy, `ENV_NOT_READY`, `PROFILE_UNKNOWN`, `RUN_IN_PROGRESS`, `RUN_CANCELLED`, index cũ, `failed|interrupted`, `offline` (giữ cache, khoá chạy), `forbidden`, `truncated`, thành công không phát hiện ("Không có phát hiện nào trong phạm vi các kiểm tra đã chạy ({profile})"); luôn **inline persistent**, toast chỉ cho xác nhận thoáng.

## 3. Quyết định thiết kế

- Slice mở rộng, không slice mới: dùng chung đường prune và cờ khoá worktree.
- Cờ chất lượng từ `settings.get`, không dò bằng gọi thử.
- Trạng thái "cũ" tin `basedOn.stale` của backend + so HEAD; không tự suy từ index.
- Hiển thị bước chạy (`QualityStep`) vì `failed|timeout|env_not_ready` không được hiểu thành "0 phát hiện".
- Không có ô nhập lệnh (O11): chỉ `RunnableProfile.id`.
- Không làm khe PR (thuộc 085-FE); cung cấp chip và deep link.
- `unknown` có biểu tượng, chữ và bảng copy riêng, không suy ra `pass`.

## 4. Tiêu chí chấp nhận

- [ ] Kiểu và parser khớp §4.7; enum lạ → `'unknown'`, không ném.
- [ ] `useQualitySupport` = `enabled` chỉ khi `qualityGateEnabled`; `disabled|unsupported` không tab, không chip, không toast.
- [ ] Scorecard: verdict, từng `reasons[]` (`code/params` → nhãn; `observed` so `threshold`; `result` bằng biểu tượng + chữ), `mode`, dòng nguồn (Cục bộ/CI, thời điểm, HEAD, index), bước chạy, `CiComparison`; `unknown` không dùng biểu tượng/màu `pass`; test quét copy không chứa "an toàn", "sạch".
- [ ] Cổng cũ hiển thị banner và chữ "cũ"; dữ liệu vẫn hiển thị.
- [ ] Chạy kiểm tra theo 2.6; đóng/mở lại tab gắn vào lần chạy đang diễn ra; mất push vẫn đồng bộ qua `quality.run`.
- [ ] `codeIntelQualityByWorktree` được dọn qua cả hai đường xoá worktree (test rò rỉ).
- [ ] Lens lazy; không tab khi `unsupported|disabled`.
- [ ] Mọi chuỗi `translate()` đủ 5 locale; không hex; Electron và web; không `components/code-review/*`; không `max-lines` disable; không dependency mới.

## 5. Kiểm thử

Vitest + Testing Library (`// @vitest-environment happy-dom` cho component); chạy `pnpm --filter orca-frontend test -- <đường dẫn>` (typecheck: `pnpm --filter orca-frontend exec tsc --noEmit -p tsconfig.json`, chưa kiểm chứng; `frontend/package.json` chỉ có `build`, `dev`, `test`, `test:watch`). **Chưa chạy.**

- Thuần: `quality-gate-copy` (bốn verdict; `unknown` không dùng từ của `pass`), `quality-stale-model`, `quality-run-scope-model` (mọi `ReviewScope`, giới hạn theo `RunnableProfile.scopes`), `quality-view-state` (thứ tự ưu tiên), `quality-profile-selection` (tách tên khỏi `"<name>@<scope>/v<n>"`), parser (§4.7).
- State/hook: `applyCodeIntelEvent` cho ba sự kiện kể cả `runId` do client khác; polling khi `polling`; `RUN_IN_PROGRESS` gắn lại; leak test hai đường xoá worktree; huỷ timer khi prune.
- Component: scorecard (bốn verdict × reasons, nhiều lần chạy, `dirty`, `failed` step), run control (khoá ngay, hoãn spinner, huỷ, `percent=null`, `ready:false`), lens (đăng ký có điều kiện).
- Fake backend: kịch bản trong 087-08.

## 6. Rủi ro và điểm chưa kiểm chứng

1. Toàn bộ UI phụ thuộc CR-050/051 chưa viết: tên hook/slice/registry có thể đổi.
2. Kênh chất lượng trả đối tượng phẳng, **không** `CodeIntelEnvelope` (§3.2 so §2.2): `useCodeIntelQuery` của CR-050 trả `meta`; cần xác nhận hook chấp nhận kết quả không phong bì.
3. `projectId` của worktree (O-1) do bridge CR-050 cung cấp; nếu không có, mọi lời gọi thất bại `CODEINTEL_INVALID_PARAMS`.
4. `RunnableProfile.id` so với tên trong `QualityGate.profile` chưa chốt.
5. `observed`/`threshold` là chuỗi do backend dựng (ngôn ngữ/định dạng chưa nêu): render nguyên văn, có thể lẫn ngôn ngữ.
6. Chưa đo: hiệu năng polling 2 s đa worktree; trần số worktree có Review mở.
7. Thông báo `interrupted` ở push không có trong `QualityRun.status`.

## 7. Câu hỏi mở

1. Hợp đồng: `QualityRun.status` thêm `interrupted` hay `finished.status` bỏ `interrupted`?
2. Ai sở hữu store của gate khi 085-FE và 087 cùng cần: đề xuất 087-02 (xem mục 0).
3. `RunnableProfile.id` có bằng `<name>` trong `QualityGate.profile`?
4. Format `observed`/`threshold` và có `code`-key cho mọi `reasons[]`?
5. Có capability quyền "canRunQuality" để ẩn nút trước lần bấm đầu (CR-087 7.6)?

## 8. Tasks

[FE-CV-TASK-087-01](../tasks/FE-CV-TASK-087-01-quality-types-parsers-and-channel-contract.md) đến [087-08](../tasks/FE-CV-TASK-087-08-quality-fake-backend-i18n-and-e2e.md).

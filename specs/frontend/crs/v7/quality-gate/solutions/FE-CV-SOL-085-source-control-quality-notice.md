# FE-CV-SOL-085-source-control-quality-notice: Cảnh báo cổng chất lượng ở Source Control (chỉ báo, không chặn)

> 📋 Proposed. Chưa triển khai. Viết ngày 2026-10-06 từ việc ĐỌC code `frontend/src` và hợp đồng v7; chưa chạy test, build hay ứng dụng.

**CR:** [CR-CV-085 mục 2.9b](../../../../../../docs/crs/v7/quality-gate/CR-CV-085-quality-gate.md) (phần frontend; phần backend là `BE-CV-SOL-085-*`)
**Area:** frontend (`frontend/src/renderer/src/components/right-sidebar`, `hooks`)
**Hợp đồng áp dụng:** [CONTRACT-codeintel-ui-api.md](../../../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md) mục 2.3 (lỗi), 3.2 (`quality.gate`, `quality.start`, `quality.profile.get`), 4.7 (`QualityGate`, `GateResult`), 5 (push `quality.finished`, `quality.gateChanged`), 6 (cờ); [CONTRACT-codeintel-proto-and-data-map.md](../../../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md) PQ-01, PQ-02, PQ-04, PQ-12, PQ-13, PQ-32, PQ-34, mục 7.1 (G4 fake backend), 8.3.
**TDD tham chiếu:** [v5/02-state-management](../../../../tdd/v5/02-state-management.md), [v5/05-ui-components](../../../../tdd/v5/05-ui-components.md), [v5/07-hooks-and-ipc](../../../../tdd/v5/07-hooks-and-ipc.md).

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `components/right-sidebar/SourceControl.tsx` (các dòng 1792, 2488, 2789, 3008, 3064, 3483, 5245 đến 5300), `CreateHostedReviewComposer.tsx` (372 dòng), `CreateHostedReviewComposerFields.tsx`, `source-control-commit-area.tsx` (616 dòng; `CommitAreaProps` ở dòng 85, khối `createPrIntentNotice` ở dòng 573), `ChecksPanel.tsx` (dòng 3611), `i18n/hosted-review-localized-copy.ts`, `i18n/i18n.ts`, `shared/types.ts:3793` (`GitBranchCompareSummary`), `shared/hosted-review.ts` (qua import), `components/code-review/code-review-panel.tsx` và `pr-create-dialog.tsx`, `guides/STYLEGUIDE.md`, `components/ui/` (liệt kê).

Xác nhận khớp CR-CV-085 2.9b: `handleCommit` ở `SourceControl.tsx:1792`, `handleCreatePullRequest` ở `:3008` (gọi `createHostedReview(...)` ở `:3064`), `runCreatePrIntent` ở `:3483`, `<CreateHostedReviewComposer` ở `:5247`; `createDisabled` tính trong `CreateHostedReviewComposer.tsx` (khoảng dòng 117-122); `localizedHostedReviewCopy(resolveSupportedHostedReviewCopyProvider(provider))` dùng ở `CreateHostedReviewComposer.tsx:113`.

**Xác minh `pr-create-dialog` là code chết:** `PrCreateDialog` chỉ được import bởi `components/code-review/code-review-panel.tsx:28`; `CodeReviewPanel` không được import ở nơi sống nào (grep trong `renderer/src`: chỉ còn chuỗi `data-testid="code-review-panel"` ở `task/TaskPromptEditor.tsx:222`). Vì vậy **không** dùng làm điểm chèn.

**Correction relative to CR-CV-085 2.9b (hợp đồng và mã thật thắng):**

| # | CR ghi | Thực tế / hợp đồng | Quyết định trong solution |
|---|---|---|---|
| 1 | Chỉ hai điểm chèn: `CreateHostedReviewComposer` trong `SourceControl.tsx` và `CommitArea` | Có **hai** nơi gọi composer: `SourceControl.tsx:5247` và `ChecksPanel.tsx:3611` (trạng thái "chưa có review" trong tab Checks) | Prop slot ở composer, truyền từ cả hai nơi; `CommitArea` là nhánh thứ ba (khi nút chính là `create_pr_intent`) |
| 2 | Hook gọi `codeIntel.settings.get` để biết `quality_gate_enabled` | Hợp đồng 6: `Settings.effective.qualityGateEnabled`; `CODEINTEL_QUALITY_GATE_DISABLED` ánh xạ `kind:'quality-disabled'` (bảng 2.3) | Hook đọc `effective.qualityGateEnabled`; gặp `quality-disabled` thì tự ngầm tắt, không toast |
| 3 | Timeout 3 s rồi `unknown` | Hợp đồng 2.4: đọc view 20 s phía gateway, ghi 8 s; `quality.gate` có `T/o` 8 s; WS `invokeTimeout` 25 s | Giữ 3 s **phía client** để không chặn UI; sau 3 s hiện `unknown` ("Không lấy được cổng") nhưng KHÔNG huỷ lời gọi, kết quả đến muộn vẫn cập nhật |
| 4 | `QualityGate.reasons[].result` ba giá trị | Hợp đồng 4.7 / PQ-34: bốn giá trị `pass\|warn\|fail\|unknown`, kèm `code`, `params`, `category`, `tool`, `waivedCount` | View-model dựng từ `code`+`params`; `observed`/`threshold` chỉ để chú thích tooltip |
| 5 | `gate_changed` push | Hợp đồng 5: `event:'quality.gateChanged'` với `previousVerdict`, `verdict`, `profile`, `headCommit`; session-client mất tên kênh nên chỉ dùng `obj.event` | Hook lọc theo `event` và `worktreeId` |
| 6 | Mở Review bằng `openReviewFromEntryPoint(worktreeId,'source-control',{lens:'quality'})` | `openReviewFromEntryPoint` thuộc FE-CV-SOL-061; `ReviewLensId` không có `'quality'` ở CR-051 gốc, được CR-087 thêm | Gọi qua hàm của 061; nếu lens `quality` chưa đăng ký thì mở Review không chọn lens (ghi trong task 06) |
| 7 | Hai CR cùng thêm khe vào composer: CR-085 `qualityNotice`, CR-CV-087 `preSubmitNotice` (kèm "ba nơi gọi" trong đó `renderPullRequestComposer`) | `renderPullRequestComposer` chỉ là hàm test trong `PullRequestComposer.generate-tooltip.test.tsx:15`, **không** phải nơi gọi sản phẩm; chỉ có hai nơi gọi sản phẩm (hàng 1) | Một khe duy nhất tên **`qualityNotice`** do solution này tạo; FE-CV-SOL-087-quality-scorecard-and-state tái dùng, không thêm khe thứ hai (xem câu hỏi mở 1) |
| 8 | `useCodeIntelSupport().state === 'enabled'` là đủ cho `visible` | Hợp đồng 6: phải có **cả** `codeIntelEnabled` và `qualityGateEnabled` (hai cờ hiệu lực) | Hook `useQualityFeatureFlags` (task 01) gộp hai cờ |

**Chưa kiểm chứng:** tên hàm của store/hook của FE-CV-SOL-050 (`codeIntelClient.call`, slice `code-intel`, bus sự kiện) chưa tồn tại khi soạn (`specs/frontend/crs/v7/` chưa có); mọi chỗ dựa vào chúng ghi "giả định theo CR-CV-050 2.3". `frontend/package.json` không có script typecheck/lint (chỉ `build`, `dev`, `test`, `test:watch`).

## 2. Giải pháp

### 2.1 Cây file

```
frontend/src/renderer/src/
  hooks/
    useQualityFeatureFlags.ts                      (mới) { code: boolean, quality: boolean, ai: boolean }
    useQualityFeatureFlags.test.ts                 (mới)
  components/right-sidebar/
    source-control-quality-gate-view-model.ts      (mới) hàm thuần: QualityGate -> NoticeViewModel
    source-control-quality-gate-view-model.test.ts (mới)
    use-source-control-quality-gate.ts             (mới) hook
    use-source-control-quality-gate.test.tsx       (mới)
    source-control-quality-gate-notice.tsx         (mới) khối hiển thị
    source-control-quality-gate-notice.test.tsx    (mới)
    CreateHostedReviewComposer.tsx                 (sửa) + prop qualityNotice?: React.ReactNode
    source-control-commit-area.tsx                 (sửa) + prop qualityNotice?: React.ReactNode
    SourceControl.tsx                              (sửa, vài dòng) gọi hook, truyền node
    ChecksPanel.tsx                                (sửa, vài dòng) như trên
  i18n/locales/{en,es,ja,ko,zh}.json               (sửa) khoá mới đọc theo tên
  i18n/quality-gate-notice-locale-coverage.test.ts (mới)
```

Không file nào mới trùng tên chung chung (`helpers`, `utils`...). Không thêm `max-lines` disable: `source-control-commit-area.tsx` đã có disable cũ (đầu file, được ghi chú là kế thừa); solution chỉ thêm khoảng 6 dòng ở đó, mọi logic nằm ở file mới (cùng tinh thần CR-085 và CR-061).

### 2.2 Chữ ký TypeScript

```ts
// hooks/useQualityFeatureFlags.ts
export type QualityFeatureFlags = {
  state: 'unknown' | 'ready'
  codeIntel: boolean   // Settings.effective.codeIntelEnabled && support.state === 'enabled'
  quality: boolean     // codeIntel && Settings.effective.qualityGateEnabled
  ai: boolean          // quality && Settings.effective.aiReviewEnabled
}
export function useQualityFeatureFlags(): QualityFeatureFlags

// source-control-quality-gate-view-model.ts
import type { QualityGate, GateResult } from '../../../../shared/code-intel-quality-types'
export type QualityNoticeReason = { check: string; result: GateResult; messageKey: string; params: Record<string,string> }
export type QualityNoticeViewModel =
  | { kind: 'hidden' }                                           // pass, hoặc cờ tắt, hoặc chưa tải
  | { kind: 'fail' | 'warn' | 'unknown'
      reasonCount: number; reasons: QualityNoticeReason[]        // <= 3 đầu để hiển thị
      stale: boolean; evaluatedAt: string | null
      unavailable?: 'timeout' | 'offline' | 'tool-failed'       // "không lấy được cổng"
      canRunChecks: boolean }
export function buildQualityNoticeViewModel(input: {
  gate: QualityGate | null; error: CodeIntelErrorKind | null; timedOut: boolean; running: boolean
}): QualityNoticeViewModel

// use-source-control-quality-gate.ts
export type SourceControlQualityGate = {
  visible: boolean                       // false => KHÔNG gọi RPC nào
  view: QualityNoticeViewModel
  loading: boolean
  running: boolean                       // đang chạy kiểm tra do nút "Chạy kiểm tra"
  runChecks: () => Promise<void>
  openReason: () => void                 // mở Review (lens quality nếu có)
}
export function useSourceControlQualityGate(args: {
  worktreeId: string | null
  headOid: string | null                 // GitBranchCompareSummary.headOid
  baseRef: string | null                 // GitBranchCompareSummary.baseRef khi status === 'ready'
}): SourceControlQualityGate
```

Kiểu `QualityGate`, `GateResult`, `CodeIntelErrorKind` do FE-CV-SOL-050-types-and-runtime-bridge sao chép từ hợp đồng mục 4.7 và 2.3; solution này **không** khai báo lại.

### 2.3 Hành vi hook

1. `visible = useQualityFeatureFlags().quality && worktreeId != null`. Khi `visible=false`: không đăng ký sự kiện, không gọi `codeIntelClient.call` (khẳng định bằng test đếm lời gọi). Lý do: hợp đồng 6 và U7, cờ tắt không được tạo tải.
2. Khi `visible`: gọi `codeIntel.quality.gate` với `{projectId, worktreeId, base: baseRef ?? undefined}` (tham số theo PQ-04: `projectId` lấy từ `Worktree.projectId`, trường tuỳ chọn ở `shared/types.ts:487`; thiếu `projectId` thì `visible=false`, xem rủi ro). Không gửi `record`, `turnKey`, `includeWaivedDetail` (ghi điểm xu hướng là việc của luồng lượt agent, FE-CV-SOL-089).
3. Làm mới khi: `headOid` đổi (debounce 400 ms); push `event:'quality.gateChanged'` hoặc `'quality.finished'` của đúng `worktreeId`; tab trở lại hiển thị (`document.visibilityState === 'visible'`); bỏ qua khi tab ẩn.
4. Timer 3 s kể từ khi bắt đầu lời gọi: hết hạn mà chưa có kết quả thì `timedOut=true` → view `unknown` kèm `unavailable:'timeout'`; lời gọi KHÔNG bị huỷ, kết quả đến sau vẫn thay view. Lỗi `CODEINTEL_TIMEOUT` có hậu tố `inProgress` (hợp đồng 2.4) → thử lại sau `retryAfterMs`, tổng tối đa 90 s.
5. Lỗi theo hợp đồng 2.3: `quality-disabled`, `disabled`, `unsupported`, `forbidden`, `no-binding` → `{kind:'hidden'}` (không toast); `offline` → giữ view cuối (stale) hoặc `unknown`/`offline`; `tool-failed`, `timeout`, `unknown` → `unknown` với `unavailable`.
6. `runChecks()`: (a) `quality.profile.get` không tên để lấy `profile.name` và `runnableProfiles` (hợp đồng 3.2, "không thêm kênh liệt kê"); (b) chọn profile mặc định `runnableProfiles.find(p => p.ready && !p.heavy)`; không có thì nút bị ẩn (`canRunChecks=false`); (c) `quality.start {profile, scope:'changed'}`; (d) `running=true` tới khi nhận `quality.finished` (hoặc `quality.progress` chuyển sang kết thúc), rồi tải lại cổng. Lỗi `CODEINTEL_ENV_NOT_READY`, `CODEINTEL_RUN_IN_PROGRESS` (gắn vào run đang chạy, không báo lỗi), `CODEINTEL_PROFILE_UNKNOWN`, `forbidden` → ghi nhãn tương ứng trong notice, không toast đỏ. Tiến độ chi tiết và nút huỷ là việc của `QualityRunControl` (FE-CV-SOL-087-quality-scorecard-and-state); notice chỉ báo "Đang chạy kiểm tra" và liên kết mở Review.
7. Không bao giờ ném lỗi ra render; không đọc `error.code` (luôn `internal`), chỉ phân loại do bộ phân loại của 050 (đọc tiền tố của `message`, hợp đồng U5).

### 2.4 Quy tắc hiển thị (view-model, không overclaim)

| `verdict` / tình huống | Hiển thị | Ghi chú |
|---|---|---|
| `pass` | **Không hiện khối** | Tránh cảm giác "được phê duyệt" (CR-085 quyết định 8); thông tin đủ ở scorecard |
| `warn` | Một dòng: "Cổng chất lượng: có cảnh báo (N lý do)." + "Xem lý do" | Biểu tượng cảnh báo trung tính, KHÔNG biểu tượng dấu tích |
| `fail` | "Cổng chất lượng: không đạt (N lý do). Bạn vẫn có thể {commit/tạo {PR\|MR}}." + "Xem lý do" | `{PR\|MR}` từ `localizedHostedReviewCopy(...).shortLabel`/`reviewLabel`; không chặn |
| `unknown` | "Chưa đủ dữ liệu để kết luận" + nút "Chạy kiểm tra" nếu `canRunChecks` | Biểu tượng khác `pass`, không bao giờ màu/biểu tượng "đạt" (hợp đồng 4.7) |
| Không lấy được cổng (timeout/offline) | "Chưa đủ dữ liệu để kết luận (không lấy được cổng)." | Vẫn `unknown`, không phải `pass` |
| `basedOn.stale` | Thêm dòng phụ "Chỉ mục có thể cũ so với HEAD" | Từ `QualityGate.basedOn.stale` |
| `mode:'block'` | Chữ KHÔNG đổi; vẫn cảnh báo | O9; hợp đồng 4.7 |

Cấm các cụm: "an toàn", "đã đáp ứng", "đạt yêu cầu", "AI đã review". Chữ cho `reasons[]` dựng từ `code`+`params` qua `translate()` với bảng khoá `auto.components.right.sidebar.qualityGateNotice.reason.<code>` (mã đã biết: `errors_over_limit`, `no_run`, `run_running`, `env_not_ready`, `stale_index`, `no_coverage`, `waived`; mã lạ rơi vào khoá `.reason.unknown` kèm `check` hiển thị nguyên văn, theo mẫu `ruleId` lạ ở hợp đồng 4.5). Mọi chuỗi từ backend (`check`, `params`) hiển thị văn bản thuần (U9), không HTML.

### 2.5 Wireframe

```
┌ Source Control ───────────────────────────────┐
│ [message ...........................]         │
│ ┌ ⊘ Cổng chất lượng: không đạt (2 lý do).   ┐ │   <- fail, role="status", token destructive
│ │ Bạn vẫn có thể tạo merge request.          │ │      (GitLab -> "merge request")
│ │ lint: 3 lỗi · test: chưa chạy              │ │
│ │ [Xem lý do]                                │ │
│ └────────────────────────────────────────────┘ │
│ [ Create MR ▾ ]                                │   <- primaryAction.disabled KHÔNG đổi
└────────────────────────────────────────────────┘

unknown:  ◌ Chưa đủ dữ liệu để kết luận   [Chạy kiểm tra]  [Xem lý do]
running:  ⟳ Đang chạy kiểm tra…           [Xem trong Review]
```

Màu/kiểu: dùng token `border`, `muted`/`muted-foreground`, `destructive` (cho `fail`, theo STYLEGUIDE "error states"), không hex; icon `lucide-react` (`ShieldAlert`, `CircleHelp`, `Loader2` cho spinner chuẩn), cỡ `size-3.5`; chữ `text-[11px]` như các notice lân cận trong `source-control-commit-area.tsx:573`. Khối đặt **trên hàng nút chính** của composer và dưới `createPrIntentNotice` trong `CommitArea`, không chiếm focus, không `role="alert"` (không ngắt người dùng): `role="status"` + `aria-live="polite"`. Nút "Chạy kiểm tra" là `Button variant="outline" size="xs"`, khoá ngay khi bấm (STYLEGUIDE UX rule 1, SSH), spinner chỉ hiện sau ~200 ms.

### 2.6 Điểm chèn (đã đọc)

1. `CreateHostedReviewComposer`: thêm `qualityNotice?: React.ReactNode` vào `CreateHostedReviewComposerProps`; render ngay trên `<div className={cn(RIGHT_SIDEBAR_SPLIT_ACTION_ROW_CLASS, 'pt-0.5')}>` (khoảng dòng 253). **Không** đưa vào biểu thức `createDisabled` (khoảng dòng 117) và không đổi `primaryAction.disabled`.
2. `CommitArea` (`source-control-commit-area.tsx`): thêm prop cùng tên, render sau khối `createPrIntentNotice` (dòng 573). Dùng cho nhánh nút chính là `create_pr_intent` (chuỗi commit → push → tạo PR, `runCreatePrIntent`), đồng thời cho commit thường.
3. `SourceControl.tsx`: gọi `useSourceControlQualityGate({worktreeId: activeWorktreeId, headOid: branchSummary?.headOid ?? null, baseRef: branchSummary?.status === 'ready' ? branchSummary.baseRef : null})` ở gần `branchSummary` (dòng 606) và truyền `<SourceControlQualityGateNotice .../>` vào cả `CreateHostedReviewComposer` (`:5247`) và `CommitArea` (`:5282`).
4. `ChecksPanel.tsx:3611`: cùng hook và cùng prop (worktreeId đã có ở `:191`; `headOid` lấy từ cùng slice `gitBranchCompareSummaryByWorktree`, chưa kiểm chứng ChecksPanel đã chọn slice đó).
5. `handleCommit` (1792), `handleCreatePullRequest` (3008), `runCreatePrIntent` (3483): **không đổi hành vi**. Chuỗi tự động commit → push → tạo PR không có hộp thoại xác nhận trong MVP (CR-085 Q2 vẫn mở).

## 3. Quyết định thiết kế

- `pass` không hiện khối; mọi trạng thái còn lại có chữ trung lập, không tô "xanh an toàn".
- Một khe duy nhất `qualityNotice` (ReactNode) thay vì truyền verdict xuống: composer và `CommitArea` không biết gì về mã hợp đồng, giữ nguyên test hiện có.
- Hook tự chịu trách nhiệm cờ: `visible=false` thì không gọi gì; không cần kiểm ở nơi gọi.
- Timeout 3 s chỉ là timeout **hiển thị** (không huỷ RPC) để không làm kết quả đúng bị mất; phù hợp SSH (độ trễ 50 đến 200 ms).
- Hai provider: mọi chữ "PR/MR" qua `localizedHostedReviewCopy`; GitLab hiện "merge request"; Azure DevOps/Gitea hiện "pull request" (hành vi của hàm hiện có).
- Electron và web: không dùng API riêng nền tảng; sự kiện đến qua bridge của 050 (web dùng `web-code-intel-api.ts`); cờ không dùng `typeof window.api.codeIntel` (hợp đồng 6, web bọc `window.api` bằng Proxy).
- Không thêm thư viện.

## 4. Phụ thuộc chéo khu vực

| Cần | Nơi | Ghi chú |
|---|---|---|
| Kênh `codeIntel.quality.gate`, `quality.profile.get`, `quality.start` thật | `BE-CV-SOL-040-codeintel-quality-channels`; nghiệp vụ `BE-CV-SOL-085-quality-gate-evaluator-and-profiles`, `BE-CV-SOL-085-waivers-and-trend` | Trước G3: dùng **fake backend** G4 (`code-intel-fake-backend.ts`, CR-073 T2) với fixture bốn verdict, cổng `stale`, lỗi `QUALITY_GATE_DISABLED`; fixture do FE-CV-SOL-073-flag-gating-and-web-e2e sở hữu, solution này đề xuất thêm ca |
| Run kiểm tra thật cho nút "Chạy kiểm tra" | `quality.start` ← `BE-CV-SOL-082-quality-run-storage-and-ingest`, `AG-CV-SOL-081-quality-runner-core`, `AG-CV-SOL-081-quality-profile-catalog-and-preflight` | Chưa có thì nút bị ẩn (`runnableProfiles` rỗng) |
| Bridge, slice, bus sự kiện, bộ phân loại lỗi, `useCodeIntelSupport` | `FE-CV-SOL-050-types-and-runtime-bridge`, `FE-CV-SOL-050-store-and-query-hooks` | G4 bắt buộc trước |
| `openReviewFromEntryPoint`, lens `quality` | `FE-CV-SOL-061-review-entry-points`, `FE-CV-SOL-051-review-workspace-shell`, `FE-CV-SOL-087-quality-scorecard-and-state` | Thiếu lens thì mở Review không lens |
| Settings `effective.*` | `BE-CV-SOL-073-settings-flag-and-rollout`, `FE-CV-SOL-073-flag-gating-and-web-e2e` | |
| `projectId` của worktree | `Worktree.projectId?` (tuỳ chọn, `shared/types.ts:487`) | Điểm mở O-1 của hợp đồng |
| AG | Không có solution AG riêng; agent chỉ tham gia gián tiếp qua runner (081) | |

## 5. Tiêu chí chấp nhận

- [ ] Cờ `quality` tắt: `codeIntelClient.call` không được gọi lần nào, không subscribe, không render khối; bật lại thì hiện lại.
- [ ] `fail` hiện thông báo, `primaryAction.disabled` và `createDisabled` không đổi; test hiện có của `CreateHostedReviewComposer`/`CommitArea`/`source-control-primary-action*` xanh.
- [ ] `pass` không render khối nào; `unknown`, lỗi mạng, timeout 3 s đều hiện "Chưa đủ dữ liệu để kết luận" với biểu tượng khác `pass`.
- [ ] Provider `gitlab` hiện "merge request"; `github` hiện "pull request".
- [ ] Hai nơi gọi composer (`SourceControl`, `ChecksPanel`) và `CommitArea` đều hiện khối với cùng dữ liệu.
- [ ] `mode:'block'` không đổi chữ và không chặn nút.
- [ ] Kết quả `CODEINTEL_QUALITY_GATE_DISABLED` làm khối biến mất lặng lẽ, không toast.
- [ ] Mọi chuỗi qua `translate()`; năm locale có khoá; không hex cứng.
- [ ] Thiếu `Worktree.projectId`: không gọi RPC, không lỗi.

## 6. Kiểm thử (Vitest + Testing Library)

- Hàm thuần `buildQualityNoticeViewModel`: bảng ca đủ `pass|warn|fail|unknown`, `stale`, `timedOut`, lỗi, mã lý do lạ, cắt còn 3 lý do.
- Hook (`renderHook`, giả `codeIntelClient`): cờ tắt → 0 lời gọi; `headOid` đổi → gọi lại có debounce; push `quality.gateChanged` của worktree khác bị bỏ qua; timeout 3 s với kết quả đến muộn; `runChecks` thành công/`ENV_NOT_READY`/`RUN_IN_PROGRESS`; huỷ khi unmount.
- Component (`// @vitest-environment happy-dom`): mỗi view; `role="status"`; nút khoá ngay; "Xem lý do" gọi hàm mở Review; văn bản thuần (reason chứa `<b>` hiện đúng chữ).
- Hồi quy composer: render có/không `qualityNotice` không đổi `disabled` của nút chính (mẫu `PullRequestComposer.generate-tooltip.test.tsx`).
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/right-sidebar/source-control-quality-gate src/renderer/src/hooks/useQualityFeatureFlags` (chưa chạy). E2E (tuỳ chọn, CR-073): `tests/e2e` theo mẫu `tasks-page.spec.ts`.

## 7. Rủi ro và điểm chưa kiểm chứng

- `Worktree.projectId` tuỳ chọn: workspace cũ chưa có thì tính năng ẩn (an toàn nhưng người dùng không thấy lý do).
- Ngưỡng nhiễu của cổng chưa đo (CR-085 mục 5/6): thông báo `fail` có thể gây mệt mỏi; giảm bằng `pass` im lặng và chữ ngắn.
- `SourceControl.tsx` (6 723 dòng) và `ChecksPanel.tsx` rất lớn: chỉ thêm vài dòng; chạy GitNexus `impact` trước khi sửa (chưa chạy).
- Cờ `Settings.effective` đọc mỗi 60 s khi tab Review mở (hợp đồng 6): ở Source Control không có tab Review mở nên cờ có thể cũ tới lần đọc kế tiếp (chấp nhận, ≤ 5 s ở backend cộng chu kỳ client).
- Sửa tiếp trên cùng commit sau khi chạy kiểm tra: backend không phát hiện (CR-085 2.3); notice phải hiện `evaluatedAt` ở tooltip để người dùng thấy độ cũ.
- `ChecksPanel.tsx` có thể không có `headOid` sẵn.

## 8. Câu hỏi mở

1. Tên khe composer: `qualityNotice` (CR-085) hay `preSubmitNotice` (CR-087). Solution này chọn `qualityNotice`; người soạn FE-CV-SOL-087-quality-scorecard-and-state cần dùng cùng tên, hoặc đổi tên bằng một PR duy nhất.
2. Có thêm hộp xác nhận một lần cho `runCreatePrIntent` khi cổng `fail` (CR-085 Q2): MVP không.
3. Notice có nên hiện cho commit thường (không phải Create PR)? Solution chọn có (CR-085 nêu "cảnh báo trước commit/Create PR").
4. Profile mặc định cho nút "Chạy kiểm tra": suy từ `runnableProfiles` (đầu tiên `ready` và không `heavy`) hay từ `QualityProfile.name`; chờ BE-CV-SOL-085/081 chốt.

## 9. Tham chiếu

`/opt/repos/orca/guides/STYLEGUIDE.md` (đường dẫn thật là `guides/`, không phải `docs/` như `AGENTS.md` ghi), `/opt/repos/orca/AGENTS.md`, `/opt/repos/orca/frontend/src/renderer/src/components/right-sidebar/SourceControl.tsx`, `CreateHostedReviewComposer.tsx`, `source-control-commit-area.tsx`, `ChecksPanel.tsx`, `/opt/repos/orca/frontend/src/renderer/src/i18n/hosted-review-localized-copy.ts`, `/opt/repos/orca/frontend/src/shared/types.ts`.

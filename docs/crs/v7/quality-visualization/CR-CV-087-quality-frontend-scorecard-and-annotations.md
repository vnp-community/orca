# CR-CV-087 — Frontend chất lượng: scorecard cổng, danh sách phát hiện kiểm tra, chú thích trên diff, chạy kiểm tra, coverage, xu hướng, hotspot, DSM, cảnh báo trước khi tạo review đã gửi

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-087 |
| **Tên** | Frontend kiểm soát chất lượng: lens `quality` (scorecard cổng có lý do và nguồn, độ phủ/diff coverage, xu hướng theo lượt, hotspot, DSM), danh sách phát hiện `QualityFinding` (ghép vào dock của CR-CV-059), chú thích phát hiện ngay trên dòng diff Monaco, nút "Chạy kiểm tra" có tiến độ và huỷ, trạng thái rỗng/lỗi, cảnh báo (không chặn) trước khi tạo review đã gửi |
| **Loại** | Feature |
| **Priority** | 🔴 P0 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-06 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | **CR-CV-088 (làm trước)**; CR-CV-050 (cầu nối, slice, hook, i18n, cờ), CR-CV-051 (khung, registry lens, chip index, trạng thái), CR-CV-053 (`pendingDiffReveal`, mở diff đúng dòng, `OVERLAY_ENCODING`), CR-CV-059 (dock Phát hiện), CR-CV-060 (mốc lượt), CR-CV-061 (điểm vào); backend: CR-CV-081 (agent `quality.*`), 082 (`QualityFinding`), 083 (coverage), 085 (`QualityGate`, lịch sử, miễn trừ), 086 (nguồn CI), 037 (dữ liệu hotspot/phụ thuộc), 040 (kênh `codeIntel.quality.*`). Các CR backend cùng nhóm chất lượng được viết đồng thời: tham chiếu bằng ID và README v7 mục 3.10, **không** đoán nội dung chưa tồn tại |
| **Mở khoá** | CR-CV-089 (đối chiếu "agent tự báo" hiển thị cạnh scorecard), 090 (báo cáo xuất lấy ảnh/số từ cùng hook), 095 (telemetry) |
| **Tác động** | `frontend/src/shared/code-intel-quality-types.ts` (mới) và mục quality trong `code-intel-rpc-methods.ts`, `code-intel-errors.ts`, `code-intel-wire-parsers.ts` (CR-CV-050 sở hữu); `renderer/src/components/review-map/quality/` (mới); `components/editor/DiffViewer.tsx` (một lời gọi hook) + `components/editor/quality-annotations/` (mới); `components/right-sidebar/CreateHostedReviewComposer.tsx` (một khe `preSubmitNotice`) và ba nơi gọi nó; `store/slices/code-intel-quality-state.ts` (mới, ghép vào slice `code-intel`); `hooks/useQuality*.ts` (mới); `assets/main.css` (lớp glyph); `i18n/locales/*.json`; sửa nhỏ ở CR-CV-051 (`ReviewLensId`, khe thanh đầu) |

---

## 1. Bối cảnh và vấn đề

Series v7 trả lời "agent đổi gì và ảnh hưởng tới đâu"; nghiên cứu [11](../../../research/view-code/11-additions-for-quality-control.md) C2/D3/D6 thêm "đạt chuẩn chưa". README v7 mục 3.10 đã định nghĩa hợp đồng: `QualityFinding`, `QualityRun`, `QualityGate`, kênh `codeIntel.quality.*`, push `codeIntel.quality.progress|finished`. Chưa có dòng UI nào cho chúng. CR này là phần giao diện, **chế độ chỉ báo** (O9): không chặn tạo review đã gửi hay commit.

Hiện trạng frontend đã đọc (2026-10-06):

- **Trình xem diff** `components/editor/DiffViewer.tsx` (488 dòng) nhận `worktreeId`, `relativePath`, giữ `modifiedEditor` trong state (:82), gắn `useDiffCommentDecorator` (:110) và có tự cuộn tới thay đổi đầu (:166-245). Cấu hình `DiffEditor` ở :444-478 **không** đặt `glyphMargin` (mặc định của Monaco cho diff editor chưa kiểm chứng). Sau CR-CV-053 sẽ có `revealLine/revealNonce`.
- **`useDiffCommentDecorator`** (`components/diff-comments/useDiffCommentDecorator.tsx`, 733 dòng, đã có `eslint-disable max-lines` từ trước ở :1-3): dựng **view zone** chứa thẻ React cho ghi chú, nút "+" tự định vị, `deltaDecorations` cho vùng kéo chọn (:239-250). Các view zone đẩy nội dung xuống nên **không phù hợp** cho hàng trăm phát hiện; và file không nên bị mở rộng thêm.
- **Cơ chế `createDecorationsCollection`** đã dùng ở `MonacoEditor.tsx:745` (quyết định xung đột). Không có nơi nào trong `frontend/src` dùng `setModelMarkers` (grep rỗng 2026-10-06), nên marker là cơ chế **mới** với repo; `glyphMargin` chỉ xuất hiện ở `IpynbViewer.tsx:415` (đặt `false`).
- **Chỗ "Create PR" thật** (không phải `components/code-review/pr-create-dialog.tsx`, là code chết nằm trong thư mục đã bị loại): `components/right-sidebar/CreateHostedReviewComposer.tsx` (372 dòng; khung nội dung `space-y-2.5` ở ~:200, trường nhập ~:230, hàng nút hành động ~:253) được gọi từ `SourceControl.tsx:5247` (nhánh `directCreatePrAction`), `ChecksPanel.tsx:3611` và `renderPullRequestComposer` (theo GitNexus `Called by: ChecksPanel, renderPullRequestComposer, SourceControlInner`). Nhánh `CommitArea` của Source Control cũng có thể dẫn tới "Create PR" qua `source-control-primary-create-pr-intent-action.ts` (321 dòng) và `source-control-create-pr-intent-flow.ts`; **chưa đọc kỹ**, cần liệt kê khi triển khai. `SourceControl.tsx` dài 6 723 dòng nên mọi thay đổi ở đó phải là một dòng nối prop.
- **`CreateHostedReviewComposer` dùng `localizedHostedReviewCopy(...)`** (`i18n/hosted-review-localized-copy`): nhãn review theo nhà cung cấp (GitHub, GitLab…); CR này dùng nhãn đó, không viết cứng "PR".
- **Hạ tầng dùng lại**: `hooks/usePrefersReducedMotion.ts`; `components/ui/{card,badge,tabs,popover,collapsible,toggle-group,select,skeleton,table,progress,tooltip}.tsx`; `@tanstack/react-virtual`; `lib/screen-submit-shortcut.ts` (`isScreenSubmitShortcut`) và `components/ShortcutKeyCombo.tsx` (CR-CV-059 đã xác nhận tồn tại; không đọc lại).
- **Chưa có** (đã rà `components/ui`): `alert`, `switch`, cây, lưới dữ liệu. `progress.tsx` không có trạng thái không xác định và luôn có chuyển tiếp 300 ms (CR-CV-088 mục 1.2).

### 1.1 Ranh giới với CR-CV-059 (tránh trùng việc)

| | CR-CV-059 | CR-CV-087 (CR này) |
|---|---|---|
| Đối tượng | `Finding`: **cấu trúc** suy ra từ đồ thị/phân tích tĩnh (`layer_violation`, `dependency_cycle`, `hotspot`, `missing_tenant_id`, `dead_code`) và lens Hợp đồng | `QualityFinding`: kết quả **chạy kiểm tra** (lint, typecheck, test, coverage, complexity, security, dependency, convention, architecture, ai) theo README 3.10 |
| Kênh | `codeIntel.findings`, `codeIntel.dismissFinding`, `codeIntel.contractDiff` | `codeIntel.quality.findings`, `codeIntel.quality.waive`, `codeIntel.quality.gate`, … |
| Hành động bỏ qua | "Bỏ qua" (`ignored`) / "Đã xử lý" (`resolved`), khoá `finding_key`, bảng `finding_dismissals` | **Miễn trừ** (`waive`): bắt buộc lý do **và hạn dùng** (`quality_waivers`, README 3.10), khoá `fingerprint` |
| Sở hữu UI | Dock `FindingsPanel`, thanh công cụ, `FindingRow`, lens Hợp đồng | `QualityFindingRow`, `QualityWaivePopover`, lọc theo severity/category, chú thích diff, lens `quality`, scorecard |

**Đề xuất gộp danh sách (v1, ít xâm lấn):** giữ **một dock** do CR-CV-059 sở hữu, thêm điều khiển nguồn `ToggleGroup` "Cấu trúc | Kiểm tra" ở đầu `FindingsToolbar`. Chọn "Cấu trúc" hiển thị `FindingsList` của 059; chọn "Kiểm tra" gắn `QualityFindingsList` của CR này vào cùng khung. Số đếm trên thanh tóm tắt và tab cộng riêng từng nguồn (không cộng gộp thành một con số). **Không** trộn hai nguồn trong cùng một danh sách ở v1 vì: (a) thang mức độ khác (`high|medium|low|info` so với `error|warning|info`), (b) hành động khác (bỏ qua so với miễn trừ có hạn), (c) khoá nhận dạng khác (`key` so với `fingerprint`). Chế độ "Tất cả" trộn nguồn để sau khi thống nhất bảng ánh xạ mức độ (7.1). Hai nguồn có thể mô tả cùng một vấn đề (category `architecture`/`convention` của `QualityFinding` với `layer_violation`): UI hiển thị cả hai kèm nhãn nguồn, **không tự khử trùng** (7.2).

### 1.2 Lệch giữa README v7 và nhu cầu UI (ghi ở "Điều chỉnh hợp đồng" của README folder)

- Kênh `codeIntel.quality.*` (README 3.10) **không có `cancel`** dù agent có `quality.cancel` và gRPC không có `CancelQualityRun`; UI cần huỷ.
- Không có kênh/RPC liệt kê **tên profile** (`quality.listProfiles` ở agent; gRPC chỉ `GetQualityProfile` một profile) cho bộ chọn profile.
- Push `quality.progress {runId, stage, percent|null, message}` và `finished {runId, status, summary}` **không mang `worktreeId`**; UI phải biết thuộc worktree nào.
- Mã lỗi `CODEINTEL_PROFILE_UNKNOWN`, `CODEINTEL_ENV_NOT_READY`, `CODEINTEL_RUN_IN_PROGRESS`, `CODEINTEL_RUN_CANCELLED` không có trong `CODE_INTEL_ERROR_CODES` của CR-CV-050 (test ở CR-CV-050 mục 4 khẳng định "đúng 10 mã"); cần mở rộng và sửa test đó.
- `QualityFinding` không có cột (column) nên không chú thích đúng ký tự được; chỉ cả dòng.
- `QualityGate.reasons[].check` là chuỗi tự do, không có khoá ánh xạ sang `category`/`tool` nên "bấm lý do để lọc phát hiện" cần quy ước (7.3).

## 2. Giải pháp đề xuất

### 2.1 Kiểu và hợp đồng phía frontend

File mới `frontend/src/shared/code-intel-quality-types.ts` (mirror README 3.10, enum lạ rơi về `'unknown'`, không ném; theo quy tắc của CR-CV-050 2.1):

```ts
type QualitySeverity = 'error' | 'warning' | 'info'            // + 'unknown' khi parse
type QualityCategory = 'lint' | 'typecheck' | 'test' | 'coverage' | 'complexity'
  | 'security' | 'dependency' | 'convention' | 'architecture' | 'ai'   // + 'unknown'
type QualityFinding = { fingerprint: string; ruleId: string; severity: QualitySeverity
  category: QualityCategory; file: string; line: number; endLine: number; message: string
  tool: string; toolVersion: string; fixHint?: string
  waiver?: { by: string; reason: string; expiresAt: string }   // ĐỀ XUẤT: backend gắn khi đã miễn trừ
}
type QualityRun = { id: string; worktreeId: string; headCommit: string; indexCommit: string
  scope: QualityRunScope; profile: string; status: 'queued'|'running'|'succeeded'|'failed'|'cancelled'
  source: 'local' | 'ci'; startedAt: string; finishedAt: string | null
  summary: { error: number; warning: number; info: number } }
type QualityGate = { verdict: 'pass'|'warn'|'fail'|'unknown'
  reasons: { check: string; observed: string; threshold: string; result: 'pass'|'warn'|'fail' }[]
  mode: 'inform' | 'block'; profile: string
  basedOn: { runIds: string[]; indexCommit: string; stale: boolean } }
type QualityRunScope = 'worktree' | 'changed' | 'commitRange'
// ĐỀ XUẤT (README chưa định nghĩa message; CR-CV-083/085/037 chốt):
type QualityTrendPoint = { turnId: string | null; headCommit: string; at: string; runId: string
  error: number; warning: number; info: number; coverage: number | null
  diffCoverage: number | null; verdict: QualityGate['verdict'] }
type CoverageReport = { runId: string; estimated: boolean; lines: { covered: number; total: number }
  files: { path: string; covered: number; total: number
           changedCovered: number | null; changedTotal: number | null
           uncoveredRanges: { start: number; end: number }[] }[]
  diff: { covered: number; total: number } | null }
```

Các kiểu có dấu "ĐỀ XUẤT" là điều UI **cần**; README chưa có message; sai khác thì chỉnh ở đây theo CR-CV-083/085 (6, 7.4). `unknown` hiển thị thành "Chưa rõ", không bị bỏ qua hay ép thành `info`.

Kênh dùng (README 3.10; tham số là đề xuất, mọi lời gọi mang `worktreeId`):

| Kênh | Tham số (ngoài `worktreeId`) | Dùng ở |
|---|---|---|
| `codeIntel.quality.gate` | `scopeKey?` | Scorecard, chip, cảnh báo trước khi tạo review |
| `codeIntel.quality.runs` | `limit`, `source?` | Dòng nguồn, danh sách lần chạy, đối chiếu `basedOn.runIds` |
| `codeIntel.quality.run` | `runId` | Trạng thái một lần chạy (đồng bộ lại khi mất push) |
| `codeIntel.quality.start` | `profile`, `scope`, `base?` | Nút "Chạy kiểm tra" |
| `codeIntel.quality.cancel` (**thiếu ở README**) | `runId` | Nút Huỷ |
| `codeIntel.quality.findings` | `runId?`, `severity[]?`, `category[]?`, `file?`, `offset`, `limit` | Danh sách, chỉ mục chú thích |
| `codeIntel.quality.waive` | `fingerprint`, `reason`, `expiresAt`, `note?` | Miễn trừ |
| `codeIntel.quality.coverage` | `runId?` | Panel phủ test |
| `codeIntel.quality.trend` | `limit` | Panel xu hướng |
| `codeIntel.quality.profile.get` | `name?` | Danh sách profile (cần trả danh sách khi bỏ `name`; 7.5) |

`CODE_INTEL_RPC_METHODS` và `CodeIntelRpcContract` mở rộng (CR-CV-050 sở hữu file; CR này thêm mục). `parseCodeIntelPushEvent` thêm hai biến thể `quality.progress`, `quality.finished`. `classifyCodeIntelError` thêm `kind`: `profile-unknown`, `env-not-ready`, `run-in-progress`, `run-cancelled` (mục "Điều chỉnh" bảng lỗi 2.3 của CR-CV-050).

**Phát hiện tính năng:** capability `quality-gate.v1` trong `status.get` khi cờ `quality_gate_enabled` bật (O9) và `code_intel_enabled` bật; dự phòng: thăm dò `codeIntel.quality.gate` một lần (cùng quy tắc `useCodeIntelSupport` của CR-CV-050 2.4: mã `CODEINTEL_*` nghĩa là bật; `method_not_found` nghĩa là chưa hỗ trợ; `forbidden` nghĩa là tắt). `hooks/useQualitySupport.ts` trả `unknown|enabled|unsupported|disabled`; hai trạng thái sau **không hiển thị gì** (không tab, không chip, không thông báo trước khi tạo review đã gửi).

### 2.2 Store: mở rộng slice `code-intel`

Không tạo slice thứ hai để dùng chung đường dọn khi xoá worktree. Thêm file `store/slices/code-intel-quality-state.ts` xuất `createCodeIntelQualityState(set, get)` trả đoạn state/action, được **trải** vào `createCodeIntelSlice` (giữ `code-intel.ts` khỏi phình thêm):

```
codeIntelQualityByWorktree: Record<worktreeId, {
  run: ActiveQualityRun | null          // lần chạy đang theo dõi
  gate: CachedQuality<QualityGate>|null
  runs: CachedQuality<QualityRun[]>|null
  findings: { runId: string; items: QualityFinding[]; total: number; truncated: boolean;
              byFile: Record<relativePath, QualityFinding[]> } | null     // trần nạp 5 000
  trend: CachedQuality<QualityTrendPoint[]> | null
  coverage: CachedQuality<CoverageReport> | null
  ui: { source: 'structure'|'quality'; severity: QualitySeverity[]; category: QualityCategory[]
        onlyChanged: boolean; showWaived: boolean; annotationsOn: boolean
        selectedFingerprint: string | null; profile: string | null; runScope: QualityRunScope }
}>
ActiveQualityRun = { runId: string|null; profile: string; scope: QualityRunScope
  phase: 'starting'|'queued'|'running'|'cancelling'|'finished'; status?: QualityRun['status']
  stage: string; percent: number | null; message: string; startedAt: number }
actions: startQualityRun(worktreeId, {profile, scope}) / cancelQualityRun(worktreeId)
  loadQualityGate / loadQualityFindings / loadQualityRuns / loadQualityTrend / loadQualityCoverage (đều {force?})
  waiveQualityFinding(worktreeId, fingerprint, {reason, expiresAt, note?})
  setQualityUi(worktreeId, patch) / invalidateQuality(worktreeId)
```

- **Dọn rò rỉ:** khoá `codeIntelQualityByWorktree` được thêm vào `CODE_INTEL_WORKTREE_KEYED_STATE_KEYS` (CR-CV-050 2.5) nên hai đường xoá worktree (`removeWorktree`, `buildWorktreePurgeState`) đã gọi `pruneCodeIntelWorktrees`; thêm test rò rỉ theo mẫu CR-CV-050 mục 5. Huỷ timer polling theo worktree khi prune.
- **Sự kiện:** mở rộng `applyCodeIntelEvent` (CR-CV-050 2.5): `quality.progress` cập nhật `run` của worktree sở hữu `runId`; `quality.finished` đặt `phase:'finished'`, rồi `invalidateQuality(worktreeId)` và tải lại `gate`, `runs`, `findings` nếu có lens/dock/trình diff đang dùng (tải lười: chỉ nạp thứ đang có người xem). Event `codeIntel.changed` (index đổi) **không** xoá kết quả chất lượng nhưng đánh dấu `gate` "có thể cũ" và tải lại `gate` (vì `basedOn.indexCommit` có thể đổi).
- **Ánh xạ `runId` → worktree:** push của README không mang `worktreeId` (1.2). Cho tới khi gateway thêm (đề nghị), client giữ `runId → worktreeId` từ kết quả `start`/`runs`; `runId` lạ (do client khác hoặc CR-CV-080 tự khởi chạy) thì làm mới `runs` của các worktree có Review mở (giới hạn tần suất) thay vì bỏ rơi.
- **Mất push:** khi `codeIntelEventsState` là `polling` hoặc sau nối lại (`codeIntelResyncCounter` tăng, CR-CV-050 2.6), gọi `codeIntel.quality.run {runId}` cho lần chạy đang theo dõi mỗi 2 s; dừng khi `status` kết thúc.
- **Cache chất lượng riêng**, trần 8 mục mỗi worktree, **không** dùng LRU 16 của kết quả đồ thị để không đẩy mất dữ liệu Ảnh hưởng.
- Hook (mới, `renderer/src/hooks/`): `useQualitySupport`, `useQualityGate(worktreeId)`, `useQualityRun(worktreeId)`, `useQualityFindings(worktreeId, filters)`, `useQualityTrend`, `useQualityCoverage`, `useQualityFindingsForFile(worktreeId, relativePath)` (chỉ mục `byFile`, mảng ổn định để diff không vẽ lại). Mọi hook không gọi mạng khi `enabled=false` hoặc hỗ trợ khác `enabled`; huỷ khi gỡ.

### 2.3 Lens "Chất lượng" và khung

Đăng ký `quality` trong `REVIEW_LENS_DEFINITIONS` (CR-CV-051 2.6), tab cuối sau "Hợp đồng", chỉ khi `useQualitySupport` là `enabled` (không tab chết). Cần hai sửa nhỏ ở file thuộc CR-CV-051: thêm `'quality'` vào `ReviewLensId` và một khe `trailing` ở `ReviewHeaderBar` để chèn `QualityGateChip` cạnh `ReviewRiskChip`. Lens tải lười (`React.lazy`) để `quality-charts` không vào chunk khởi động.

```
┌ Chất lượng · Profile [full ▾]  Phạm vi [Tệp đã đổi ▾]  [▶ Chạy kiểm tra] ────────────────────────────┐
│ Nguồn: Cục bộ · chạy 14:02 (3 phút trước) · HEAD a41c9e0 · index d819812 (cũ: HEAD đã tiến thêm)     │
├────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ ┌ CỔNG: CẢNH BÁO  (chế độ chỉ báo, không chặn) ───────────────────────┐ ┌ Diff coverage ───────────┐ │
│ │ ■ Lỗi 2  ▲ Cảnh báo 5  ● Thông tin 9   [thanh chồng]                 │ │ 62% ──■─────|── ngưỡng 80%│ │
│ │ ⊗ typecheck   2 lỗi    ngưỡng 0           Chưa đạt        ▸ xem     │ │ 124/200 dòng đã đổi được  │ │
│ │ ▲ coverage    diff 62% ngưỡng 80%         Cảnh báo        ▸ xem     │ │ phủ · ước lượng           │ │
│ │ ✓ lint        0 lỗi    ngưỡng 0           Đạt                        │ └──────────────────────────┘ │
│ │ ? security    chưa chạy                   Chưa rõ                    │                              │
│ └──────────────────────────────────────────────────────────────────────┘                              │
│ ▾ Phủ test: treemap theo thư mục (đậm = tỉ lệ chưa phủ cao)   ▾ Xu hướng theo lượt   ▾ Hotspot   ▾ DSM │
└────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

Thứ tự khối: (1) thanh công cụ chạy (2.4), (2) dòng nguồn (2.5), (3) scorecard và gauge, (4) bốn khối `Collapsible` (Phủ test, Xu hướng, Hotspot, Phụ thuộc), mỗi khối tự quản `loading|ready|empty|error` qua `ChartFrame` (CR-CV-088) và **chỉ nạp khi mở** (mặc định mở "Phủ test" nếu có coverage; còn lại thu gọn). Danh sách phát hiện **không** nằm trong lens mà ở dock (1.1); lens có liên kết "Xem {n} phát hiện" mở dock ở nguồn "Kiểm tra".

Component (thư mục `components/review-map/quality/`, tên theo khái niệm):

```
QualityLens.tsx · QualityLensToolbar.tsx (profile, phạm vi, QualityRunControl)
QualityRunControl.tsx · QualityRunProgress.tsx
QualityScorecard.tsx · QualityGateVerdictHeader.tsx · QualityGateReasonRow.tsx
QualityProvenanceLine.tsx · QualityGateChip.tsx
QualityCoveragePanel.tsx · QualityTrendPanel.tsx · QualityHotspotPanel.tsx · QualityDependencyPanel.tsx
QualityStateScreen.tsx (rỗng/lỗi theo kind, 2.9)
findings/QualityFindingsList.tsx · QualityFindingRow.tsx · QualityFindingsToolbar.tsx
findings/QualityWaivePopover.tsx · quality-finding-filter.ts · quality-finding-sort.ts
quality-view-state.ts · quality-gate-copy.ts · quality-run-scope-model.ts · quality-stale-model.ts
```

### 2.4 Scorecard cổng: kết luận, lý do, nguồn, không overclaim

`QualityScorecard` hiển thị `QualityGate` bằng `ui/card` + `GateVerdictBadge` (CR-CV-088) + `StackedSeverityBar` (từ `QualityRun.summary` của lần chạy dựng cổng) + danh sách `reasons[]`.

- **Văn bản kết luận** (`quality-gate-copy.ts`, nguồn duy nhất; test cấm các từ "an toàn", "sạch"): `pass` "Các kiểm tra của profile {profile} đều đạt", `warn` "Có cảnh báo", `fail` "Chưa đạt", **`unknown` "Chưa đủ dữ liệu để kết luận"** kèm lý do đầu tiên (ví dụ "Chưa có lần chạy cho HEAD này"). `unknown` hiển thị bằng `--quality-unknown` + biểu tượng `CircleHelp` + chữ, **không bao giờ** dùng biểu tượng hay màu của `pass`; nếu `reasons` rỗng và `unknown` thì dòng giải thích mặc định "Backend không trả lý do".
- **Lý do bấm xem được**: mỗi `QualityGateReasonRow` hiển thị `check`, `observed` so với `threshold` và `result` (biểu tượng + chữ); bấm mở dock ở nguồn "Kiểm tra" và đặt bộ lọc theo quy ước (7.3: `check` trùng `category` thì lọc theo category, ngược lại lọc theo `tool`, ngược lại chỉ cuộn tới danh sách và ghi "Không lọc được theo '{check}'").
- **Nguồn và độ tươi** (`QualityProvenanceLine`): `Nguồn: Cục bộ|CI` (từ `QualityRun.source` tra theo `basedOn.runIds` trong `runs`; nhiều lần chạy hiển thị từng chip; không tra được thì "Nguồn: chưa rõ"), thời điểm `finishedAt`, `HEAD {headCommit}` của lần chạy, `index {indexCommit}`. **Cũ** (`quality-stale-model.ts`, hàm thuần) khi: `basedOn.stale === true`, hoặc `QualityRun.headCommit` ≠ HEAD hiện tại (lấy `gitBranchCompareSummaryByWorktree[worktreeId].headOid`, CR-CV-051 2.3), hoặc `basedOn.indexCommit` ≠ HEAD. Cũ hiển thị banner inline (không toast): "Kết quả chạy tại {commit}; HEAD hiện tại {head}. Số liệu có thể không còn đúng." + nút "Chạy lại". Cổng vẫn hiển thị nhưng huy hiệu kèm chữ "cũ".
- **Chế độ**: `mode:'inform'` hiển thị "Chế độ chỉ báo: không chặn tạo review đã gửi". `mode:'block'` (nếu backend sau này trả) vẫn **chỉ cảnh báo** ở CR này (O9: chặn cứng cần quyết định riêng) và hiển thị "Cấu hình chặn; giao diện hiện chỉ cảnh báo" để không hứa điều chưa làm.
- **Không một con số duy nhất** (research 11 §7): không có "điểm 82/100"; chỉ kết luận + thành phần.
- `QualityGateChip` (thanh đầu Review, tab right sidebar của CR-CV-061 và `QualityGatePrNotice`): huy hiệu nhỏ biểu tượng + chữ ngắn, `Tooltip` liệt kê lý do đầu; bấm mở lens `quality`.

### 2.5 Nút "Chạy kiểm tra": profile, tiến độ, huỷ

`QualityRunControl` trong thanh công cụ lens (và nút trong dock khi chưa có lần chạy):

- **Profile**: `Select` từ danh sách profile (`profile.get` trả danh sách, 7.5); mặc định = profile của cổng nếu có, rồi profile cấu hình mặc định, rồi mục đầu; không có danh sách thì nút chạy bị vô hiệu kèm giải thích "Chưa có profile kiểm tra được cấu hình". **Không** có ô nhập lệnh (O11): chỉ **tên profile**.
- **Phạm vi** (`quality-run-scope-model.ts`, thuần): `ReviewScope` → tham số `start`: `branch` → `scope:'changed'` + `base = mergeBase`; `range` → `commitRange` + `base = baseCommit`; `hostedReview` → `commitRange` + `base = baseRefName` đã phân giải; mục phụ "Toàn bộ worktree" (`worktree`), có nhãn "chậm hơn". Mặc định `changed`.
- **Quyền**: người kích hoạt cần quyền ghi trên project (O11). Lỗi `forbidden` khoá nút, giữ đọc, hiển thị "Bạn không có quyền chạy kiểm tra" inline và `Tooltip`; **không** dựng trước quyền (chưa có capability báo quyền, 7.6).
- **Máy trạng thái** (`useQualityRun`):

| Pha | Hiển thị |
|---|---|
| `idle` | Nút "Chạy kiểm tra" (icon `Play`) |
| `starting` | Nút khoá **ngay** khi bấm (chống bấm đúp, rubric SSH); sau ~200 ms nếu đích từ xa (100 ms nếu cục bộ) mới hiện spinner trong nút; nhãn nút giữ độ rộng cố định |
| `queued` | "Đang chờ ở dev server" |
| `running` | `QualityRunProgress`: nhãn `stage`, `message` và `Progress` (có `percent`) hoặc spinner tĩnh khi `percent === null` (tiến độ không xác định; `progress.tsx` không hỗ trợ nên dùng spinner `Loader2` có `motion-reduce:animate-none`, không sửa `progress.tsx`); sau ≥ 3 s mới hiện nhãn giai đoạn (thang STYLEGUIDE); nút "Huỷ" (ghost, im lặng, theo UX rule 3 "Cancel không phải destructive") |
| `cancelling` | "Đang huỷ…", khoá nút huỷ, **không** khẳng định đã huỷ cho tới khi `finished` mang `status:'cancelled'` |
| `finished` | Theo `status`: `succeeded` tải lại cổng/phát hiện; `failed` thông báo inline "Lần chạy không hoàn tất" + chi tiết sao chép được (không phải kết quả "có phát hiện"); `cancelled` trung tính "Đã huỷ" (không phải lỗi) |

- Tiến độ lấy từ push `codeIntel.quality.progress`; mất push thì đồng bộ qua `quality.run` (2.2). Chuyển tab/đóng tab Review **không** huỷ lần chạy (nó chạy ở dev server); mở lại Review gắn vào lần chạy đang diễn ra (`runs` có `running`).
- `CODEINTEL_RUN_IN_PROGRESS`: gắn vào lần chạy đang chạy (lấy `runId` từ `error.data` nếu có, nếu không từ `runs`), không báo lỗi (như `reindex-in-progress` ở CR-CV-050).
- Mọi chuỗi qua `translate()`; không có phím tắt cho "Chạy kiểm tra" ở MVP (không hiển thị chip phím giả).

### 2.6 Danh sách phát hiện kiểm tra (`QualityFindingsList`)

Gắn vào dock của CR-CV-059 khi nguồn = "Kiểm tra" (1.1):

```
Lọc: [Mức: ✕Lỗi ▲Cảnh báo ●Thông tin] [Loại ▾ lint|typecheck|test|…] [☐ Chỉ tệp đã đổi] [☐ Hiện đã miễn trừ N] 🔍
 ⊗ no-unused-vars  'x' được khai báo nhưng không dùng           src/a/b.ts:42      oxlint 1.2   [Xem diff][Ghi chú][Miễn trừ ▾]
 ▲ TS2322          Kiểu 'string' không gán được cho 'number'    src/a/c.ts:7-9     tsc 5.x      …
   Gợi ý sửa: …                                                (fixHint, nếu có, thu gọn)
 Đang hiển thị 500/1 203 (còn lại tải thêm)      [Tải thêm]
```

- Sắp mặc định: mức (error → warning → info), `category`, `file`, `line`; ổn định (`fingerprint` làm khoá phụ). Lọc là hàm thuần `quality-finding-filter.ts` (mức, loại, chỉ tệp đã đổi từ `ChangeOverlay.changedFiles`, đã miễn trừ, tìm trong `message/ruleId/file`).
- **Ảo hoá** `@tanstack/react-virtual` khi > 50 hàng (cùng ngưỡng 059, chưa đo), `estimateSize` 44 px, `getItemKey = fingerprint`. Nạp theo trang `limit` 500 qua `quality.findings`; trần nạp 5 000 (`truncated` hiển thị "Đang hiển thị X/Y"). Không cắt im lặng.
- `QualityFindingRow`: `SeverityBadge` (biểu tượng + chữ), `ruleId`, `message` (kẹp 2 dòng, đầy đủ ở `Tooltip`), `file:line[-endLine]`, `tool toolVersion`. Phát hiện mức tệp (`line` bằng 0) hiển thị `file` không số dòng.
- **Liên kết dòng diff**: "Xem diff" gọi hàm mở diff đúng dòng của CR-CV-053 (`pendingDiffReveal`). `openReviewDiffAtSymbol` nhận `SymbolRef`, còn phát hiện chỉ có `(file, line)`: đề nghị CR-CV-053 tách lõi `openReviewDiffAtPath(worktreeId, relativePath, line, scope)` và để hàm theo symbol gọi nó; nếu chưa có, CR này dùng đường mở tệp của `openAnnotationLocation` (mẫu `check-annotation-open.ts`) và ghi giới hạn. Ngoài thay đổi (`file` không có trong `changedFiles`) thì nút thành "Mở tệp". Đường dẫn qua `resolveAnnotationPathInsideWorktree` (chặn thoát worktree).
- **Ghi chú**: nút "Ghi chú" dùng luồng của CR-CV-060 (đưa `message`, `ruleId`, `fixHint` vào nội dung gợi ý, không tự gửi); không tự tạo đường gửi mới. Khoá neo ghi chú là `fingerprint` kèm `(file, line)` lúc ghi.
- **Miễn trừ** (`QualityWaivePopover`): bắt buộc `reason` và `expiresAt` (chọn 7/30/90 ngày hoặc ngày cụ thể; đề xuất, README chỉ nói "có người, lý do, hết hạn"), tuỳ chọn ghi chú; gọi `codeIntel.quality.waive`; khoá nút ngay, cập nhật lạc quan, hoàn nguyên và báo inline khi lỗi (`forbidden`, `offline`, `conflict`). `Mod+Enter` xác nhận (`isScreenSubmitShortcut`, chip qua `ShortcutKeyCombo`; `metaKey` trên Mac, `ctrlKey` nơi khác). Dòng đã miễn trừ ẩn mặc định, đếm "đã miễn trừ N", hiển thị người và hạn khi bật; hết hạn thì backend trả lại phát hiện như mở. Không có "Hoàn tác" hứa hẹn nếu backend không có RPC bỏ miễn trừ (7.5).
- **Phím** (khi tiêu điểm trong danh sách và không ở ô nhập, `isEditableTarget`): `j`/`k` chuyển dòng, `Enter` xem diff, `w` mở miễn trừ, `n` ghi chú (nếu 060 cài), `Esc` đóng; đăng ký qua registry của CR-CV-052 nếu có để không va với `j/k/Space` của Thứ tự đọc và `d`/`r` của 059 (cùng dock, khác nguồn nên khác phím).

### 2.7 Chú thích phát hiện ngay trên dòng diff Monaco

**Phương án** (so sánh):

| Phương án | Ưu | Nhược | Quyết định |
|---|---|---|---|
| Mở rộng `useDiffCommentDecorator` (view zone) | Cùng cơ chế với ghi chú | Đẩy nội dung, nặng với nhiều phát hiện; file đã 733 dòng có `eslint-disable max-lines` (không được thêm) | Loại |
| `createDecorationsCollection` + CSS (lề glyph, nền dòng) | Mẫu có ở `MonacoEditor.tsx:745`; tô theo token CSS | Không có rê chuột/điều hướng phím sẵn; màu overview ruler cần chuỗi màu cụ thể (không nhận `var()`, xem rủi ro) | **Dùng cho dấu hình học ở lề** |
| `monaco.editor.setModelMarkers` (marker) | Gạch chân, hover có `source`/`code`, **F8/Shift+F8** nhảy tới vấn đề kế tiếp, thước tổng quan, trợ năng của Monaco | Màu do theme Monaco (`vs`/`vs-dark`) chứ không do token Orca; có thể bị hiểu là chẩn đoán của language service | **Dùng làm cơ chế chính** (kèm `source`+`code` hiển thị rõ nguồn là công cụ kiểm tra) |

Quyết định: **marker (chính) + decoration glyph (dấu hình học bổ sung)**, trong hook mới `components/editor/quality-annotations/useQualityFindingMarkers.ts` gọi một dòng từ `DiffViewer.tsx` (cạnh `useDiffCommentDecorator`):

- `quality-marker-model.ts` (thuần, có test): `buildQualityMarkers(findings, {lineCount, maxColumnOfLine})` → `IMarkerData[]` và `IModelDeltaDecoration[]`. Ánh xạ mức: `error` → `MarkerSeverity.Error`, `warning` → `Warning`, `info` → `Info`; `unknown` → `Hint`. Vùng: toàn dòng từ `line` tới `endLine` (không có cột trong `QualityFinding`; kẹp vào `[1, lineCount]`); phát hiện mức tệp (`line` bằng 0) đặt ở dòng 1. `message` kèm `fixHint` nếu có; `source = tool`, `code = ruleId`; owner cố định `'orca-quality'` để xoá sạch không đụng marker khác.
- Lề glyph: `glyphMarginClassName: 'orca-quality-glyph-{error|warning|info}'`, hình dạng khác nhau bằng `clip-path` trong CSS (`main.css`, dùng `var(--quality-*)`; bát giác/tam giác/tròn) để **không chỉ dựa vào màu** (CR-CV-088 2.4); `glyphMarginHoverMessage` = tóm tắt. Bật `glyphMargin` cho editor sửa đổi chỉ khi có phát hiện (`updateOptions`); hành vi mặc định chưa kiểm chứng (1).
- **Chỉ phía `modified`**; chỉ khi `useQualityFindingsForFile(worktreeId, relativePath)` có dữ liệu và bật `annotationsOn`.
- **Bảo vệ lệch dòng** (số dòng là của lần chạy, không phải nội dung đang mở): chỉ vẽ khi `QualityRun.headCommit` bằng HEAD hiện tại; nếu tệp đang có thay đổi chưa commit (`gitStatusByWorktree`) thì vẫn vẽ nhưng gắn dòng nhỏ trong `Tooltip` của chip "Số dòng theo lần chạy lúc {time}; có thể lệch"; **xoá toàn bộ marker và glyph ngay khi nội dung model đổi** (`onDidChangeModelContent`, vì marker không tự di chuyển theo chỉnh sửa) và hiển thị ghi chú "Chú thích kiểm tra đã ẩn vì nội dung đã đổi; chạy lại để cập nhật".
- **Dọn**: `setModelMarkers(model, 'orca-quality', [])` và `collection.clear()` khi gỡ hook, đổi model (`modelKey`), tắt `annotationsOn`.
- **Hover/click**: rê chuột trên marker hiển thị nội dung có sẵn của Monaco; bấm glyph chọn phát hiện trong dock (`setQualityUi({selectedFingerprint})`) bằng `editor.onMouseDown` với đích `GUTTER_GLYPH_MARGIN` (khả thi chưa kiểm chứng; chú thích của `useDiffCommentDecorator` nói glyph decoration không tiện dựng popover đẹp, nên CR này không dựng popover riêng, chỉ chọn dòng ở dock).
- **Phụ thuộc `pendingDiffReveal` (CR-CV-053):** hành động "Xem diff" từ danh sách mở tab diff và cuộn đúng dòng; chú thích tự xuất hiện vì hook gắn vào mọi `DiffViewer` của worktree. Không cần phát sự kiện con trỏ (đó là việc symbol của 053).
- **Điều hướng bàn phím:** F8/Shift+F8 là mặc định Monaco cho marker; **chưa kiểm chứng** hoạt động trong `DiffEditor` này, nên chưa hiển thị chip phím.
- Thước tổng quan (`overviewRuler`): decoration cần `color` chuỗi cụ thể và Monaco không hiểu `var(--…)`/`oklch`; marker đã tự có dấu ở thước theo theme Monaco, nên **không** đặt màu overview riêng ở MVP (7.7).

### 2.8 Phủ test, diff coverage, xu hướng, hotspot, DSM

Mỗi khối là `ChartFrame` (CR-CV-088) + dữ liệu qua hook; mô tả văn bản và bảng thay thế do CR-CV-088 yêu cầu.

| Khối | Dữ liệu | Hình | Nhãn/lưu ý |
|---|---|---|---|
| **Diff coverage** | `CoverageReport.diff` | `DiffCoverageGauge` (bullet + ngưỡng từ `QualityGate.reasons` có `check` coverage, nếu parse được; nếu không, không vẽ vạch ngưỡng) | `estimated: true` → huy hiệu "Ước lượng từ cạnh test, không phải đo độ phủ"; không có `diff` → "Chưa có dữ liệu độ phủ cho phạm vi này" (không 0%) |
| **Treemap phủ/độ phức tạp** | `CoverageReport.files` (kích thước = `total`, độ đậm = tỉ lệ chưa phủ) | `MetricTreemap` ≤ 400 ô, lớp phủ `changed` bằng `OVERLAY_ENCODING` | Bấm ô tệp mở diff ở đầu khoảng chưa phủ; danh sách "Dòng đã đổi chưa phủ" (top 20 tệp) bên dưới |
| **Xu hướng theo lượt** | `QualityTrendPoint[]` (≤ 50) | `TrendLineChart` (lỗi/cảnh báo/thông tin) + đánh dấu đổi `verdict` + sparkline coverage | Nhãn trục "Lượt N" khi `turnId` khớp "mốc lượt" của CR-CV-060 (`ReviewTurnMarker.turnId`), còn lại `commit rút gọn + giờ`; bấm một điểm có `turnId` khớp gọi chế độ so sánh của `ReviewTurnSwitcher` (tên action của 060 chưa chốt, 7.8). Một điểm duy nhất thì không vẽ đường, hiển thị số và "Cần ít nhất hai lượt để thấy xu hướng" |
| **Hotspot** | `QualityHotspot[]` (ĐỀ XUẤT: `{path, churn, complexity, findings, uncoveredRatio, score}`; CR-CV-037 chốt) | `HotspotHeatmap` ≤ 40 hàng × ≤ 6 cột | Không có dữ liệu thì khối ghi "Chưa có dữ liệu hotspot"; công cụ đo phức tạp chưa có CR riêng (nghiên cứu 11 B4 xếp "Sau"), nên cột `complexity` có thể trống: hiển thị "—", không 0 |
| **DSM** | Đồ thị cụm/module rút gọn: nguồn thật **chưa chốt** (7.9): ứng viên `codeintel.overview` (≤ 500 cụm, ≤ 5 000 cạnh) qua `codeIntel.architecture`/`structure`, hoặc `structuralFacts` của CR-CV-037 | `DependencyMatrix` ≤ 60 nút (top theo bậc + dòng "Khác"), thứ tự tô-pô, khối vòng đậm | Bấm ô → liệt kê cạnh và mở lens Cấu trúc tại thư mục nguồn (nếu CR-CV-054 có) |

Phân pha giao: **Pha 1** (cần CR-CV-081/082/085): scorecard, chạy kiểm tra, danh sách phát hiện, chú thích diff, cảnh báo trước khi tạo review đã gửi. **Pha 2** (cần CR-CV-083 và lịch sử của 085): coverage, diff coverage, xu hướng. **Pha 3** (cần dữ liệu của CR-CV-037): hotspot, DSM. Khối của pha chưa có dữ liệu hiển thị `empty` có lý do; không ẩn lặng lẽ để người dùng khỏi tưởng đã đủ.

### 2.9 Trạng thái rỗng và lỗi

`quality-view-state.ts` (thuần, có test) tính trạng thái; ưu tiên cao xuống thấp; luôn **persistent inline**, không toast (toast chỉ cho xác nhận thoáng "Đã miễn trừ"):

| Tình huống | Hiển thị |
|---|---|
| `unsupported`/`disabled` | Không tab, không chip (2.1) |
| Chưa có lần chạy nào | Rỗng: "Chưa chạy kiểm tra cho worktree này" + nút "Chạy kiểm tra" trực tiếp; cổng `unknown` |
| `CODEINTEL_ENV_NOT_READY` | Lỗi inline: "Dev server chưa sẵn sàng để chạy kiểm tra" + phần thiếu nếu backend trả (hình dạng chưa chốt) + hướng xử lý ("cài dependency trong worktree"), nút "Kiểm tra lại"; **không** tự cài (O11) |
| `CODEINTEL_PROFILE_UNKNOWN` | Inline: "Profile '{name}' không còn tồn tại"; tải lại danh sách, chọn lại; `Select` về mục hợp lệ đầu tiên |
| `CODEINTEL_RUN_IN_PROGRESS` | Gắn vào lần chạy đang diễn ra (2.5), không lỗi |
| `CODEINTEL_RUN_CANCELLED` / `status:'cancelled'` | Trung tính "Đã huỷ"; giữ kết quả cũ |
| Index cũ (`basedOn.stale`, `indexCommit` ≠ HEAD) | Banner cũ (2.4); dữ liệu vẫn hiển thị |
| Lần chạy `failed` | "Lần chạy không hoàn tất" + chi tiết sao chép được; **không** hiển thị thành "0 phát hiện" |
| `offline` | Giữ dữ liệu cache, đánh dấu cũ, nút chạy bị khoá (dùng trạng thái `offline` của khung, CR-CV-051) |
| `forbidden` | Khoá chạy/miễn trừ, giữ đọc |
| `truncated` | "Đang hiển thị X/Y" |
| Không có phát hiện sau lần chạy thành công | "Không có phát hiện nào trong phạm vi các kiểm tra đã chạy ({profile})" (không "mã sạch"/"an toàn") |

### 2.10 Cảnh báo trước khi tạo review đã gửi (chỉ cảnh báo, O9)

Điểm chèn thật (1): thêm prop `preSubmitNotice?: React.ReactNode` vào `CreateHostedReviewComposer` (hiển thị giữa `CreateHostedReviewComposerFields` và hàng nút ở ~:253); ba nơi gọi truyền `<QualityGatePrNotice worktreeId={…} />` (`SourceControl.tsx` ~:5247, `ChecksPanel.tsx` ~:3611, `renderPullRequestComposer`). Nhánh `CommitArea` khi nút chính là "Create PR" (intent) cần liệt kê khi triển khai (1). Mỗi nơi chỉ thêm **một prop**; file mới `components/right-sidebar/source-control-quality-gate-notice.tsx` chứa logic, để không phình `SourceControl.tsx` (6 723 dòng).

```
┌ ⚠ Cổng chất lượng: Chưa đạt (chế độ chỉ báo) ────────────────────┐
│ typecheck: 2 lỗi (ngưỡng 0) · diff coverage 62% (ngưỡng 80%)       │
│ Kết quả chạy lúc 14:02, HEAD a41c9e0 · còn mới                     │
│ [Xem trong Review]  [Chạy lại]            (không chặn "Tạo …")     │
└───────────────────────────────────────────────────────────────────┘
```

- Hiển thị khi `verdict` là `fail` hoặc `warn`; `unknown` hoặc chưa có lần chạy → một dòng trung tính nhỏ "Chưa chạy kiểm tra cho HEAD này" (chỉ khi đã cấu hình profile); `pass` → không hiển thị (không thưởng thêm dòng "đạt" gây ngộ nhận).
- **Không bao giờ** vô hiệu hoá hay trì hoãn nút tạo; không modal; không thay đổi `createDisabled` của composer. Tải cổng **không chặn** composer: lấy từ slice, nếu chưa có thì gọi `loadQualityGate` hoãn 200 ms (SSH), lỗi mạng thì không hiển thị gì (không giả định "đạt").
- Nhãn "review đã gửi" lấy từ `localizedHostedReviewCopy(...).reviewLabel` của composer (không cứng "PR"), tương thích GitLab (AGENTS.md).
- Kết quả cũ (`quality-stale-model`) thì ghi "Kết quả cũ: HEAD đã đổi từ {commit}" và nút "Chạy lại" trước nút "Xem trong Review".

### 2.11 i18n, theme, hai render target, SSH

Khoá `auto.components.reviewQuality.<Thành phần>.<tên>` và `auto.hooks.codeIntelQuality.<tên>`, đủ `en, es, ja, ko, zh`; test `i18n/code-intel-quality-locale-coverage.test.ts` (dùng chung file với 088). Chỉ token `--quality-*`, `--review-*`, `--card`…; không hex; không màu Tailwind thô. Mọi lời gọi qua `codeIntelClient`/`useCodeIntelQuery` nên chạy ở Electron và web như CR-CV-050 (Electron local phụ thuộc preload của 050). SSH 50–200 ms: nút khoá ngay, hiển thị chờ trễ ~200 ms, không chặn tiêu điểm; không có thao tác nào giữ UI chờ mạng (cập nhật lạc quan cho miễn trừ).

## 3. Quyết định thiết kế

- **Thêm lens `quality`, không nhét vào lens khác**: scorecard và biểu đồ không phải đồ thị mã; registry lens của 051 cho phép thêm mà không tab chết.
- **Danh sách phát hiện ở dock của 059, tách nguồn** thay vì trộn: thang mức độ, hành động và khoá khác nhau; trộn sau khi thống nhất.
- **`unknown` là trạng thái thật**, có biểu tượng, chữ và bảng copy riêng; không bao giờ suy ra `pass`.
- **Marker + glyph hình học**, không view zone: hàng trăm phát hiện, a11y của Monaco, không đụng file 733 dòng.
- **Xoá chú thích khi nội dung đổi** thay vì cố di chuyển: marker không tự theo chỉnh sửa; đúng hơn là ẩn và nói rõ.
- **Chỉ cảnh báo**, không chặn (O9), nhúng qua một khe prop để không sửa logic tạo review.
- **Slice mở rộng, không slice mới**, dùng chung đường prune và cờ `CODE_INTEL_WORKTREE_KEYED_STATE_KEYS`.
- **Chạy kiểm tra chỉ nhận tên profile** (O11); không có ô lệnh.
- **Pha hoá theo dữ liệu**: khối nào chưa có nguồn thì nói "chưa có dữ liệu", không đoán.

## 4. Tiêu chí chấp nhận

- [ ] Lens `quality` chỉ xuất hiện khi `useQualitySupport` là `enabled`; `unsupported`/`disabled` không hiển thị tab, chip hay thông báo.
- [ ] Scorecard hiển thị `verdict`, từng `reasons[]` (`check`, `observed`, `threshold`, `result` bằng biểu tượng + chữ), `mode`, và dòng nguồn (Cục bộ/CI, thời điểm, HEAD, index); `unknown` hiển thị "Chưa đủ dữ liệu để kết luận", không dùng biểu tượng/màu của `pass`; test quét copy không chứa "an toàn"/"sạch".
- [ ] Cổng cũ (HEAD ≠ `headCommit` của lần chạy, hoặc `basedOn.stale`) hiển thị banner và chữ "cũ"; dữ liệu vẫn hiển thị.
- [ ] "Chạy kiểm tra": chỉ chọn profile có tên; khoá nút ngay; spinner chờ ≥ ~200 ms từ xa; tiến độ theo `quality.progress` (có `percent` thì thanh, `null` thì spinner tĩnh với `prefers-reduced-motion`); Huỷ hoạt động và chỉ báo "đã huỷ" sau `finished(cancelled)`; đóng/mở lại tab gắn vào lần chạy đang diễn ra; mất push vẫn đồng bộ qua `quality.run`.
- [ ] `ENV_NOT_READY`, `PROFILE_UNKNOWN`, `RUN_IN_PROGRESS`, `RUN_CANCELLED`, index cũ, `failed`, `offline`, `forbidden` mỗi trường hợp có hiển thị đúng bảng 2.9, persistent inline, không toast.
- [ ] Danh sách phát hiện kiểm tra: lọc theo mức/loại/tệp đã đổi/đã miễn trừ/tìm kiếm; sắp ổn định; ảo hoá > 50; nạp theo trang, trần 5 000 có "X/Y"; "Xem diff" mở diff đúng dòng (phụ thuộc 053) hoặc mở tệp khi ngoài thay đổi; đường dẫn thoát worktree bị chặn.
- [ ] Miễn trừ bắt buộc lý do và hạn; lạc quan + hoàn nguyên; `Mod+Enter` theo nền tảng; dòng đã miễn trừ ẩn và đếm được.
- [ ] Dock của 059 có điều khiển nguồn "Cấu trúc | Kiểm tra"; số đếm hai nguồn tách biệt; không trộn hai danh sách ở v1.
- [ ] Chú thích diff: marker owner `'orca-quality'` với `source`/`code`; glyph hình dạng khác nhau theo mức; chỉ phía modified; xoá khi nội dung đổi hoặc tắt; dọn khi gỡ hook; không phát sinh khi chưa có lần chạy hoặc `annotationsOn` tắt; không tăng `useDiffCommentDecorator`.
- [ ] Diff coverage hiển thị `estimated` rõ ràng; không có dữ liệu thì "Chưa có dữ liệu", không 0%.
- [ ] Xu hướng cần ≥ 2 điểm; nhãn lượt theo `turnId` của 060; ≤ 50 điểm.
- [ ] Cảnh báo trước khi tạo review đã gửi: hiện ở cả ba nơi gọi composer khi `fail`/`warn`; **không** thay đổi trạng thái disabled/hành vi của nút tạo; tải cổng không chặn composer; nhãn theo nhà cung cấp.
- [ ] Khoá `codeIntelQualityByWorktree` được dọn qua cả hai đường xoá worktree (test rò rỉ).
- [ ] Không hex, không màu Tailwind thô; chuỗi `translate()` đủ 5 locale; hoạt động ở Electron và web; không `max-lines` disable mới; không dùng `components/code-review/*`; không thêm dependency.

## 5. Kiểm thử

Vitest (+ Testing Library, `happy-dom`); `pnpm --dir frontend test`. **Chưa chạy bất kỳ test nào; danh sách là kế hoạch.**

- Thuần: `quality-gate-copy` (đủ bốn verdict, `unknown` không dùng từ của `pass`), `quality-stale-model`, `quality-run-scope-model` (mọi `ReviewScope`), `quality-view-state` (thứ tự ưu tiên 2.9), `quality-finding-filter`/`quality-finding-sort`, `quality-marker-model` (ánh xạ mức, kẹp dòng, mức tệp, `unknown` → Hint, không ném khi `endLine < line`).
- Hook/slice: `useQualityRun` (máy trạng thái, mất push → polling, `RUN_IN_PROGRESS` gắn vào lần chạy), `applyCodeIntelEvent` cho `quality.progress|finished` kể cả `runId` lạ, `code-intel-quality-worktree-removal-leak.test.ts` (hai đường), cache 8 mục, huỷ timer khi prune, `waiveQualityFinding` lạc quan + hoàn nguyên.
- `classifyCodeIntelError` bốn `kind` mới; sửa test "đúng 10 mã" của CR-CV-050 thành 14 mã.
- Component: `QualityScorecard` (verdict × reasons, nguồn nhiều lần chạy, `unknown`), `QualityRunControl` (khoá ngay, hoãn spinner, huỷ, `percent=null`), `QualityFindingRow`/`QualityFindingsList` (ảo hoá số hàng DOM nhỏ hơn tổng, "X/Y"), `QualityWaivePopover` (lý do và hạn bắt buộc, `Mod+Enter` theo `navigator.userAgent` giả), `QualityGatePrNotice` (`fail`/`warn`/`unknown`/`pass`/mạng lỗi; không đổi `disabled` của nút), `CreateHostedReviewComposer` (khe `preSubmitNotice`; test hiện có không vỡ).
- Diff: `useQualityFindingMarkers` với editor/model giả (`setModelMarkers`, `createDecorationsCollection`, `onDidChangeModelContent`): đặt/xoá/dọn, khoá owner, lệch HEAD; `DiffViewer` test hiện có không vỡ.
- Phủ khoá i18n; quét "không hex".
- `code-intel-fake-backend.ts` (CR-CV-050) mở rộng cho kênh `quality.*` (bao gồm `percent: null`, mất push, `ENV_NOT_READY`, `PROFILE_UNKNOWN`).
- E2E (khi CR-CV-040/081 chạy): chạy một profile thật trên worktree, hiển thị cổng, mở diff có chú thích, huỷ giữa chừng; ở web và Electron.
- Kiểm tay: sáng/tối, `prefers-reduced-motion`, bàn phím (Tab vào dock, lưới nhiệt), F8 trong diff.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Hình dạng dữ liệu** (`QualityTrendPoint`, `CoverageReport`, `QualityHotspot`, `waiver` trong `QualityFinding`, danh sách profile, `error.data` của `ENV_NOT_READY`) là đề xuất UI cần; README chưa có message. Khác sẽ phải sửa mirror.
- **Marker và `DiffEditor`**: chưa kiểm chứng (a) `glyphMargin` mặc định ở diff editor, (b) F8 hoạt động, (c) marker hiển thị đúng ở model `modifiedModelPath`/`keepCurrentModifiedModel` của tab diff, (d) `onMouseDown` trên glyph. Cần thử nghiệm sớm; nếu hỏng, rút về decoration thuần (`className` nền dòng + `linesDecorationsClassName`).
- **Số dòng lệch**: kết quả là theo lần chạy; chạy trên cây làm việc bẩn nhưng `QualityRun` chỉ có `headCommit` (không cờ "dirty"); UI chỉ so HEAD và hiển thị cảnh báo mềm. Có thể vẫn hiển thị sai dòng.
- **Quyền chạy kiểm tra** (O11) chưa có capability để ẩn trước; người dùng không đủ quyền chỉ biết sau lần bấm đầu.
- **Va chạm khoá** `findings`/`waive` giữa 059 và 087 (cùng dock) nếu 059 đổi cấu trúc `FindingsToolbar`; cần phối hợp khi 059 được triển khai.
- **Thông báo trước tạo review đã gửi**: chưa đọc hết `source-control-primary-create-pr-intent-action.ts` nên có thể sót một nhánh dẫn tới tạo review mà không qua composer.
- **Độ trễ cổng**: tải `gate` lúc mở composer có thể chậm qua SSH; đã không chặn nhưng thông báo có thể xuất hiện muộn sau khi người dùng đã bấm.
- **Va chạm biểu tượng/màu** giữa cảnh báo và `untested` (CR-CV-088 2.4).
- Hotspot/DSM phụ thuộc dữ liệu của CR-CV-037 chưa viết xong và công cụ đo độ phức tạp chưa có CR riêng.
- Chưa đo hiệu năng danh sách 5 000 hàng, vẽ 700 ô DSM, hay chi phí `setModelMarkers` với 1 000 marker; ngân sách ở CR-CV-088 2.6.
- `quality_trend_points` theo "lượt" phụ thuộc "mốc lượt" của CR-CV-060 (lượt chưa lưu bền; README mục 8 điểm 15); điểm không có `turnId` chỉ gắn nhãn theo commit.

## 7. Câu hỏi mở

1. Bảng ánh xạ mức độ `Finding.severity` (`high|medium|low|info`) và `QualityFinding.severity` để có chế độ "Tất cả" trong dock (sau v1).
2. Khử trùng giữa phát hiện kiến trúc của rule pack (CR-CV-084, category `architecture`/`convention`) và `layer_violation` của CR-CV-037: ai khử, theo khoá nào?
3. Quy ước ánh xạ `QualityGate.reasons[].check` sang `category`/`tool` để bấm lý do lọc được; hoặc backend thêm `category` vào từng lý do.
4. Chốt `QualityTrendPoint`, `CoverageReport`, `QualityHotspot` với CR-CV-083/085/037; `trend` có phân biệt `source: local|ci` không?
5. Thêm `codeIntel.quality.cancel`/`CancelQualityRun`; liệt kê profile (`ListQualityProfiles` hay `profile.get` không tên trả danh sách); bỏ miễn trừ (`unwaive`) hay chỉ chờ hết hạn?
6. Capability quyền chạy kiểm tra ("canRunQuality") trong `status.get` hay phản hồi `forbidden` lần đầu là đủ?
7. Overview ruler của Monaco: định nghĩa theme Monaco riêng (`defineTheme`) với màu cụ thể của token, hay chấp nhận màu theo theme `vs`/`vs-dark`?
8. Tên action trong slice của CR-CV-060 để "bấm một điểm xu hướng → so sánh lượt".
9. Nguồn thật của DSM: `codeIntel.architecture` (mơ hồ giữa C4 và đồ thị cụm, README mục 8 điểm 10), `codeIntel.structure`, hay một kênh mới ở CR-CV-037?
10. Có thêm `QualityGateChip` vào hàng agent của dashboard (`DashboardAgentRow`, CR-CV-061 sở hữu) không? Nghiên cứu 11 C2 nhắc "hàng agent trong dashboard"; CR này chỉ cung cấp component.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (O9, O11, O12, mục 3.10, mục 8 điểm 13, 15)
- `/opt/repos/orca/docs/research/view-code/11-additions-for-quality-control.md` (§3 B1–B7, §4 C1–C2, C4, §5 D3, D6), `10-frontend-review-ux.md` (§5, §6, §7)
- `/opt/repos/orca/docs/crs/v7/quality-visualization/CR-CV-088-graphics-foundation-and-chart-primitives.md`
- `/opt/repos/orca/docs/crs/v7/review-frontend/` : `CR-CV-050` (2.1, 2.3, 2.5, 2.6), `CR-CV-051` (2.6 registry lens, 2.7 trạng thái), `CR-CV-053` (2.5, 2.6 `pendingDiffReveal`), `CR-CV-059` (2.3 dock, `finding_dismissals`), `CR-CV-060` (2.4 mốc lượt), `CR-CV-061` (điểm vào)
- CR cùng nhóm, tham chiếu theo ID (không đọc nội dung): CR-CV-080, 081, 082, 083, 084, 085, 086, 089, 090, 091, 092, 093, 094, 095
- `/opt/repos/orca/guides/STYLEGUIDE.md` (UX rule 1 thang thời lượng và SSH, rule 3 Cancel, "UI copy must not overclaim", persistent errors)
- `/opt/repos/orca/frontend/src/renderer/src/components/editor/DiffViewer.tsx` (:82, :110, :166-245, :444-478), `components/editor/MonacoEditor.tsx` (:745), `components/diff-comments/useDiffCommentDecorator.tsx` (:1-3, :239-250)
- `/opt/repos/orca/frontend/src/renderer/src/components/right-sidebar/CreateHostedReviewComposer.tsx`, `SourceControl.tsx` (:5245-5290), `ChecksPanel.tsx` (:3611), `source-control-primary-create-pr-intent-action.ts`, `source-control-create-pr-intent-flow.ts`
- `/opt/repos/orca/frontend/src/renderer/src/components/ui/progress.tsx`, `hooks/usePrefersReducedMotion.ts`, `components/editor/check-annotation-open.ts` (mẫu, qua CR-CV-053)
- Mới: `shared/code-intel-quality-types.ts`, `components/review-map/quality/*`, `components/editor/quality-annotations/{useQualityFindingMarkers.ts,quality-marker-model.ts}`, `components/right-sidebar/source-control-quality-gate-notice.tsx`, `store/slices/code-intel-quality-state.ts`, `hooks/useQuality{Support,Gate,Run,Findings,Trend,Coverage}.ts`

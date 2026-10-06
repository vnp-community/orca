# FE-CV-SOL-095-review-telemetry: Telemetry hiệu quả của Review và cổng chất lượng (chỉ enum/khoảng)

> 📋 Proposed. Chưa triển khai. Viết ngày 2026-10-06 từ việc ĐỌC code và hợp đồng v7; chưa chạy test hay ứng dụng.

**CR:** [CR-CV-095](../../../../../../docs/crs/v7/quality-gate/CR-CV-095-review-quality-telemetry.md) (phần frontend; số liệu server nằm ở `BE-CV-SOL-071-metrics-tracing-and-budgets`). Priority P2, Small.
**Area:** frontend + các bản sao `shared/` ngoài `frontend/` (cần chủ sở hữu từng gói duyệt)
**Hợp đồng áp dụng:** [CONTRACT-codeintel-ui-api.md](../../../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md) 4.6 (tập `lens` của `ReviewNoteAnchor`), 4.7 (`GateResult`), 5 (push); [CONTRACT-codeintel-proto-and-data-map.md](../../../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md) PQ-06 (`severity error|warning|info`), mục 7.2 (095-FE sau 061, 059), 8.2 (BE chỉ ghi số liệu server ở 071), 8.3. Hợp đồng **không định nghĩa kênh** telemetry (F9: chỉ client).
**TDD tham chiếu:** [v5/07-hooks-and-ipc](../../../../tdd/v5/07-hooks-and-ipc.md), [v5/02-state-management](../../../../tdd/v5/02-state-management.md).

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `shared/mcp-telemetry-events.ts` (57 dòng, zod `.strict()`, `mcpEventSchemas`), `shared/mcp-telemetry-events.test.ts`, `shared/telemetry-events.ts` (import dòng 17; `eventSchemas` dòng 1406; `...mcpEventSchemas` dòng 1490; học thuyết phiên bản dòng 1399-1404; `/* eslint-disable max-lines */` dòng 1 có sẵn), `renderer/src/lib/telemetry.ts` (`track`, `hasRuntimeBridge()` = `Boolean(window.api?.agentTrust)`, dòng 29 và 65), `lib/mcp-telemetry.ts` (hàm bọc + `bucket*`), `desktop/src/main/telemetry/validator.ts` (import `../../shared/telemetry-events`, `safeParse`, fail-closed), `web/web-preload-api.ts:873` (`telemetryTrack: () => Promise.resolve()`), `desktop/src/preload/index.ts:1843`.

**Bản sao `shared/` — kiểm chứng bằng `cmp`:** `telemetry-events.ts` và `mcp-telemetry-events.ts` tồn tại **giống byte-for-byte** ở **6** vị trí (không phải 4 như CR):

1. `frontend/src/shared/`
2. `desktop/src/shared/` (**bản mà validator của desktop main import lúc chạy**)
3. `backend/src/shared/`
4. `tests/vendor-shared/shared/`
5. `agent/src/shared/`
6. `mobile/src/vendor-shared/shared/`

(Còn bản trong `.claude/worktrees/…`: công cụ, bỏ qua.) **Không tìm thấy script đồng bộ**: đã tìm `vendor-shared`/`telemetry-events` trong `scripts`, `tools`, `config`, `.github`, `package.json` gốc và `frontend/package.json`; chỉ có `config/scripts` gốc chứa 3 tệp không liên quan, `.github/workflows/computer-e2e.yml:101` nhắc "vendor" bằng lời, `mobile/metro.config.js` trỏ `src/vendor-shared/shared`. Vì vậy phải copy tay hoặc thêm kiểm tra chống lệch (task 02). Ý nghĩa: lệch bản ở desktop làm validator **bỏ sự kiện im lặng** (chỉ cảnh báo giới hạn tần suất).

**Correction relative to CR-CV-095 (hợp đồng và mã thật thắng):**

| # | CR ghi | Thực tế | Quyết định |
|---|---|---|---|
| 1 | 4 bản sao (frontend, desktop, backend, tests/vendor-shared) | 6 bản, đều giống nhau | Cập nhật cả 6 (task 02) |
| 2 | `reason` triage lấy từ tập của CR-059 | Hợp đồng chỉ định `dismissFinding.reason` ≤ 500 ký tự tự do, không enum | Enum telemetry `not_applicable\|accepted_risk\|false_positive\|later\|other` do **frontend** ánh xạ từ lựa chọn lý do của UI (059/087); chuỗi tự do không bao giờ vào telemetry |
| 3 | `tool` enum | `QualityFinding.tool` là chuỗi tự do (hợp đồng 4.7) | Bộ ánh xạ `toToolBucket(tool)` ra enum đóng, ngoài danh sách → `other` |
| 4 | `lens` enum 9 giá trị | Hợp đồng 4.6 `ReviewNoteAnchor.lens` cùng 9 giá trị | Khớp |
| 5 | `source` hiển thị `local\|ci\|mixed` | `QualityRun.source: 'local'\|'ci'` và `CiComparison` | `mixed` khi cổng dựa trên cả hai (`basedOn.runIds` thuộc hai nguồn) |
| 6 | Web không có telemetry | Đã xác nhận: `telemetryTrack` no-op ở web; `track()` còn rẽ `callRuntimeRpc('telemetry.track')` khi có `window.api.agentTrust` | Không thêm đường khác; hàm bọc inert ở web |
| 7 | Mục 2.5 catalog tương tác tính năng | Chưa đọc `feature-interaction-categories.ts` | **Ngoài phạm vi** (câu hỏi mở 2) |
| 8 | `handlePullRequestCreated` ~:3077 | Hàm định nghĩa `SourceControl.tsx:2488`, được gọi ở 3078, 3116, 3295, 3318 (nhiều đường tạo) | Gắn vào **một** chỗ: bên trong/ngay sau `handlePullRequestCreated` |

**Chưa kiểm chứng:** tài liệu quyền riêng tư công khai (`PRIVACY_URL`) có cần cập nhật không (ngoài repo); `agent/src/shared` và `mobile` có chạy `eventSchemas` lúc chạy không (chỉ biết là bản sao).

## 2. Giải pháp

### 2.1 Cây file

```
frontend/src/shared/review-telemetry-events.ts              (mới) schema; BẢN SAO ở 5 nơi còn lại
frontend/src/shared/review-telemetry-events.test.ts         (mới) theo mcp-telemetry-events.test.ts
frontend/src/shared/telemetry-events.ts                     (sửa, +2 dòng) import + ...reviewEventSchemas; BẢN SAO ở 5 nơi
frontend/src/shared/telemetry-shared-copies-parity.test.ts  (mới) so byte 6 bản
frontend/src/renderer/src/lib/review-telemetry.ts           (mới) hàm bọc + bucket
frontend/src/renderer/src/lib/review-decision-tracker.ts    (mới) máy trạng thái thuần
(+ test cho mỗi file)
điểm gọi: SourceControl.tsx (handleCommit, handlePullRequestCreated), review-map/*, use-source-control-quality-gate (085), ReviewReportMenu (090), ReviewAiSummaryCard (093)
```

### 2.2 Schema (theo CR 2.2; chỉ `z.enum`/`z.boolean`, `.strict()`)

```ts
const lens = z.enum(['impact','architecture','dataflow','erd','storage','structure','contract','quality','requirements'])
const verdict = z.enum(['pass','warn','fail','unknown'])
const countBucket = z.enum(['0','1','2-3','4-10','11+'])
const largeBucket = z.enum(['0','1-3','4-10','11-30','31+'])
const latencyBucket = z.enum(['<1m','<5m','<30m','<4h','>=4h'])
export const reviewEventSchemas = {
  review_opened: z.object({ source: z.enum(['agent_row','source_control','cmd_k','right_sidebar','notification','restore']),
    scope: z.enum(['merge_base','committed','custom_base']), lens,
    index: z.enum(['fresh','stale','missing','unknown']), after_agent_turn: z.boolean() }).strict(),
  review_lens_viewed: z.object({ lens, dwell: z.enum(['<10s','<1m','<5m','>=5m']) }).strict(),
  quality_gate_viewed: z.object({ verdict, reasons: countBucket, stale: z.boolean(),
    surface: z.enum(['review','source_control']), source: z.enum(['local','ci','mixed']) }).strict(),
  review_decision_made: z.object({ decision: z.enum(['commit','create_review','send_to_agent','mark_reviewed','abandon']),
    latency: latencyBucket, used_review: z.boolean(), gate: z.enum(['pass','warn','fail','unknown','none']), open_findings: countBucket }).strict(),
  review_findings_summary: z.object({ shown: largeBucket, dismissed: largeBucket, waived: countBucket, resolved: largeBucket }).strict(),
  quality_finding_triaged: z.object({ action: z.enum(['dismiss','waive','restore','revoke']),
    reason: z.enum(['not_applicable','accepted_risk','false_positive','later','other']),
    severity: z.enum(['error','warning','info']), tool: toolEnum, blocking: z.boolean() }).strict(),
  review_report_exported: z.object({ format: z.enum(['markdown_copy','markdown_pr_insert','html_save']),
    provider: z.enum(['github','gitlab','azure-devops','gitea','none']), truncated: z.boolean() }).strict(),
  review_ai_summary: z.object({ outcome: z.enum(['ok','bad_output','error','disabled']), level: z.enum(['metadata','diff']),
    cache_hit: z.boolean(), feedback: z.enum(['none','useful','wrong']) }).strict()
} as const
```

Quy tắc: không id, đường dẫn, tên người/nhánh/lens tuỳ biến, thông điệp, số đếm thô, thời gian tuyệt đối; thêm giá trị enum cần phát hành schema **trước** client (validator bỏ giá trị lạ). Thay đổi phá vỡ → tên sự kiện mới `_v2` (học thuyết dòng 1399). `review_ai_summary` với `feedback` cập nhật là sự kiện thứ hai cùng tên (outcome `ok`).

### 2.3 Hàm bọc và bộ theo dõi quyết định

- `lib/review-telemetry.ts`: `trackReviewOpened`, `trackReviewLensViewed`, `trackQualityGateViewed`, `trackReviewDecisionMade`, `trackReviewFindingsSummary`, `trackQualityFindingTriaged`, `trackReviewReportExported`, `trackReviewAiSummary` + `bucketCount`, `bucketLarge`, `bucketLatencyMs`, `bucketDwellMs`, `toToolBucket`. Component **không** gọi `track()` trực tiếp (như `mcp-telemetry.ts`). Tính năng (cờ) tắt thì không có UI nên không có sự kiện; hàm bọc không tự kiểm cờ nhưng `review-decision-tracker` chỉ đăng ký khi `flags.quality || flags.codeIntel`.
- `lib/review-decision-tracker.ts` (thuần, bộ nhớ, **không bền**): `registerCompletion({worktreeId, doneAt})` (từ `AgentTurnCompletion` của FE-CV-SOL-061), `noteReviewOpened(worktreeId)`, `decide(worktreeId, decision, ctx)` → phát đúng **một** `review_decision_made` rồi xoá lượt chờ; lượt mới thay lượt cũ chưa quyết định → phát `abandon`. `used_review` = đã mở Review từ `doneAt`. Mất khi tắt app (chấp nhận); lượt không có hook không được đo.

### 2.4 Điểm gọi (chỉ vài dòng; logic ở file mới)

| Sự kiện | Nơi |
|---|---|
| `commit` | sau `handleCommit` trả `true` (`SourceControl.tsx:1792`) |
| `create_review` | trong/ngay sau `handlePullRequestCreated` (`:2488`), nơi hợp nhất nhiều đường tạo |
| `send_to_agent` | khi gửi ghi chú thành công (FE-CV-SOL-060) |
| `mark_reviewed` | đánh dấu đã xem hết (FE-CV-SOL-052-reading-order-and-progress) |
| `review_opened`, `review_lens_viewed` | `openReviewFromEntryPoint` (061) và khung 051 |
| `quality_gate_viewed` | hook cổng ở Source Control (FE-CV-TASK-085-03) và scorecard (087); khử trùng lặp theo (worktree, HEAD) trong phiên |
| `quality_finding_triaged`, `review_findings_summary` | 059/087 |
| `review_report_exported` | FE-CV-SOL-090 (menu, nút chèn) |
| `review_ai_summary` | FE-CV-SOL-093 (thẻ) |

Chạy GitNexus `impact` trước khi sửa `handleCommit` và `handlePullRequestCreated` (chưa chạy).

## 3. Quyết định thiết kế

- Chỉ client; số liệu theo tenant lấy từ bảng của tenant (CR 2.6), không đưa lên telemetry sản phẩm.
- Dùng đồng ý/opt-out sẵn có (`consent.ts`): không thêm công tắc.
- `tool` thay `ruleId`; một sự kiện quyết định mỗi lượt; đường cơ sở `used_review=false` để so sánh.
- Ngưỡng nâng `inform → block` (CR 2.8) là tiêu chí quản trị ghi vào tài liệu vận hành của FE-CV-SOL-073/BE-CV-SOL-073, **không** là code; solution này không triển khai nó.
- Không thêm thư viện.

## 4. Phụ thuộc chéo khu vực

| Cần | Nơi |
|---|---|
| Chủ sở hữu từng bản sao `shared/` | `desktop/` (validator), `backend/`, `agent/`, `mobile/`, `tests/` — ngoài `frontend/`, cần duyệt (quy ước 8.1) |
| Điểm vào, lens, `AgentTurnCompletion` | `FE-CV-SOL-051-review-workspace-shell`, `FE-CV-SOL-061-review-entry-points`, `FE-CV-SOL-052-reading-order-and-progress`, `FE-CV-SOL-060-review-notes-and-turn-compare` |
| Phát hiện bỏ qua/miễn trừ | `FE-CV-SOL-059-contract-lens-and-findings`, `FE-CV-SOL-087-quality-scorecard-and-state`; hàm bọc 085/090/093 |
| Số liệu server (không phải telemetry) | `BE-CV-SOL-071-metrics-tracing-and-budgets` |
| Kênh/fake backend | Không dùng kênh `codeIntel.*`; sự kiện phát từ hành vi UI, kiểm bằng giả `window.api.telemetryTrack`; không cần G3, chỉ cần các nơi phát có mặt |
| AG | Không có |

## 5. Tiêu chí chấp nhận

- [ ] Mỗi sự kiện `.strict()` chỉ enum/boolean; thêm khoá `repo_id`, `worktree_id`, `commit`, `path`, `rule_id`, `message`, `task_id`, `tenant_id`, `user`, `email`, `count` đều bị từ chối.
- [ ] `eventSchemas` chứa tám sự kiện; `track('review_opened', {…khoá lạ…})` không biên dịch.
- [ ] Sáu bản `telemetry-events.ts` và sáu bản `review-telemetry-events.ts` khớp byte (test parity); validator desktop chấp nhận payload hợp lệ và bỏ payload sai.
- [ ] Hàm chia khoảng test biên; không trả giá trị ngoài enum.
- [ ] Tracker: một lượt → đúng một `review_decision_made`; quyết định thứ hai bị bỏ; lượt mới thay lượt cũ → `abandon`; không phát khi cờ tắt.
- [ ] Tắt consent: không sự kiện nào ra khỏi desktop main (mock consent); web: gọi hàm bọc không lỗi.
- [ ] Không trường nào phụ thuộc nội dung người dùng nhập.
- [ ] ≤ 10 sự kiện mỗi lượt trong kịch bản mô phỏng (giới hạn 30/phút/tên, 1 000/phiên).

## 6. Kiểm thử (Vitest)

Schema (mẫu `mcp-telemetry-events.test.ts` + bảng khoá cấm), bucket, ánh xạ `toToolBucket`, tracker (đồng hồ giả), parity sáu bản (đọc nội dung các đường dẫn bằng `node:fs`; bỏ qua nếu một gói vắng, báo rõ), tích hợp: mock `window.api.telemetryTrack`, kịch bản "agent xong → mở Review → bỏ qua phát hiện → commit". Quét tĩnh: `review-telemetry.ts` không import kiểu có trường định danh. Chạy: `pnpm --filter orca-frontend test -- src/shared/review-telemetry src/renderer/src/lib/review` (chưa chạy); bản sao ngoài `frontend/` kiểm bằng test của gói đó (desktop: `src/main/telemetry/validator*.test.ts` nếu có, chưa kiểm chứng).

## 7. Rủi ro và điểm chưa kiểm chứng

- Thiên lệch mẫu: chỉ desktop đã đồng ý; không web, không CI; lượt agent không có hook không đo được.
- Lệch bản sao `shared/` làm sự kiện bị bỏ im lặng; không có script đồng bộ.
- `reason=false_positive` do người dùng tự gắn → thiên lệch.
- Tập `lens`/`reason`/`tool` phụ thuộc CR 051/059/082/087.
- `telemetry-events.ts` đã rất lớn (disable max-lines có sẵn): chỉ thêm hai dòng, không mở rộng ngoại lệ.

## 8. Câu hỏi mở

1. Cơ chế đồng bộ `shared/` chính thức (script, `pnpm` workspace, hay copy tay có test parity)?
2. Có làm mục 2.5 của CR (catalog tương tác tính năng) không?
3. Tenant doanh nghiệp có cần kill-switch telemetry riêng (CR Q3)?
4. Phát sự kiện cho người dùng chỉ dùng web (CR Q1: hiện không)?

## 9. Tham chiếu

`/opt/repos/orca/docs/crs/v7/quality-gate/CR-CV-095-review-quality-telemetry.md`, `/opt/repos/orca/frontend/src/shared/mcp-telemetry-events.ts`, `telemetry-events.ts`, `/opt/repos/orca/frontend/src/renderer/src/lib/telemetry.ts`, `mcp-telemetry.ts`, `/opt/repos/orca/desktop/src/main/telemetry/validator.ts`, `/opt/repos/orca/AGENTS.md`.

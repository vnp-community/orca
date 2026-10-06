# CR-CV-095 — Telemetry hiệu quả tính năng review và cổng chất lượng (chỉ enum/khoảng)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-095 |
| **Tên** | Bộ sự kiện telemetry thô theo mẫu `mcp-telemetry-events.ts` để đo "Review và cổng có hữu ích hay gây nhiễu": mở Review, thời gian từ agent xong tới quyết định, phát hiện bị bỏ qua/miễn trừ, kết luận cổng, báo nhầm, lens dùng nhiều; tiêu chí số để nâng cổng từ chỉ báo lên chặn |
| **Loại** | Feature (đo lường; không đổi hành vi sản phẩm) |
| **Priority** | ⚪ P2 (nhỏ) |
| **Effort** | Small |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-06 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-085 (kết luận cổng, miễn trừ), CR-CV-061 (điểm vào, `AgentTurnCompletion`), CR-CV-059 (bỏ qua phát hiện, lý do), CR-CV-051 (`ReviewLensId`); CR-CV-090/093 (hai sự kiện phụ) |
| **Mở khoá** | Quyết định nâng `inform → block` (CR-CV-085 Q1) |
| **Tác động** | `frontend/src/shared/review-telemetry-events.ts` (mới) + bản sao đồng bộ ở `desktop/src/shared/`, `backend/src/shared/`, `tests/vendor-shared/shared/`; `frontend/src/shared/telemetry-events.ts` (+ dòng `...reviewEventSchemas`); `frontend/src/renderer/src/lib/review-telemetry.ts` (mới, hàm bọc); vài điểm gọi nhỏ ở `components/review-map/` và `SourceControl.tsx`. Nguồn: [research 11](../../../research/view-code/11-additions-for-quality-control.md) E5 |

---

## 1. Bối cảnh và vấn đề

Research 11 E5: "để biết cổng có hữu ích hay gây nhiễu" cần đo thời gian từ "agent xong" đến "quyết định", tỉ lệ phát hiện bị bỏ qua/được xử lý, báo nhầm, theo mẫu `mcp-telemetry-events.ts` (chỉ enum/khoảng, không dữ liệu nhạy cảm). Đã đọc code ngày 2026-10-06:

1. **Mẫu schema** `frontend/src/shared/mcp-telemetry-events.ts` (57 dòng): object zod `.strict()` cho từng sự kiện, toàn bộ trường là `z.enum`/`z.boolean`, không chuỗi tự do, không id; spread vào `eventSchemas` ở `telemetry-events.ts` (`...mcpEventSchemas`, ~dòng 1490). Quy tắc của `telemetry-events.ts` (đầu tệp): "`.strict()` trên mọi schema; chuỗi tự do phải có `.max(N)`"; thay đổi phá vỡ phải đặt **tên sự kiện mới** (`_v2`), chỉ thêm trường tuỳ chọn thì sửa tại chỗ (đoạn "Schema-evolution / versioning doctrine", ~dòng 1397).
2. **Hàm bọc**: `frontend/src/renderer/src/lib/mcp-telemetry.ts` — component không gọi `track()` trực tiếp mà qua hàm bọc để "payload thô theo cấu trúc"; có hàm chia khoảng (`bucketLatencyMs`, `bucketScopeCount`). Test mẫu `mcp-telemetry-events.test.ts`: mỗi sự kiện có payload hợp lệ và bị **từ chối** khi thêm khoá `token`, `secret`, `client_name`, `redirect_url`, `argsPreview`, `approval_id`.
3. **Đường đi**: `lib/telemetry.ts` → `window.api.telemetryTrack` (Electron preload `desktop/src/preload/index.ts:1843`) hoặc `callRuntimeRpc('telemetry.track')` → validator của desktop main `desktop/src/main/telemetry/validator.ts` (**fail-closed**: sự kiện lạ, khoá thừa, enum sai, chuỗi quá dài đều bị bỏ + cảnh báo giới hạn tần suất) → PostHog. Validator import `desktop/src/shared/telemetry-events.ts` nên schema phải có ở đó.
4. **Bản sao `shared/`**: `telemetry-events.ts` và `mcp-telemetry-events.ts` tồn tại ở `frontend/src/shared`, `desktop/src/shared`, `backend/src/shared`, `tests/vendor-shared/shared`; hai bản đầu **giống byte-for-byte** (đã `diff`); hai bản sau chưa so. **Chưa tìm thấy script đồng bộ** (chưa tìm kỹ) — phải làm tay hoặc xác nhận cơ chế trước khi triển khai.
5. **Đồng ý/ngừng (opt-out)**: `desktop/src/main/telemetry/consent.ts` (`resolveConsent`): tắt bởi `DO_NOT_TRACK`, biến của Orca, các biến CI (`CI`, `GITHUB_ACTIONS`, `GITLAB_CI`…), hoặc người dùng tắt ở Privacy pane; trạng thái `pending_banner` khi chưa quyết định (`shared/telemetry-consent-types.ts`). Giới hạn tốc độ (`burst-cap.ts`): bucket theo tên sự kiện (mặc định 30/phút) và trần 1 000 sự kiện/phiên. `commonPropsSchema` gắn `app_version`, `platform`, `arch`, `os_release`, `install_id` (ẩn danh, dùng làm `distinctId`), `session_id`, `orca_channel`.
6. **Web không có telemetry**: `mcp-telemetry.ts` ghi "Web build: telemetryTrack is a no-op"; `hasRuntimeBridge()` cần `window.api.agentTrust` (chỉ có ở desktop). Hệ quả: dữ liệu telemetry **chỉ từ desktop đã đồng ý**, lệch (bias) so với toàn bộ người dùng.
7. **Catalog tương tác tính năng có sẵn**: `feature-interaction-catalog.ts` (có id `review-notes`), `feature_interaction_usage_bucket_reached` (milestone đếm cục bộ theo khoảng `count_1…count_1000_plus`), cần `feature_category` khớp `feature_id`.
8. **Backend không có kênh telemetry sản phẩm**: không có PostHog ở Go; dữ liệu phía server sẵn có là bảng (`quality_trend_points`, `finding_dismissals`, `quality_waivers`) và số đo vận hành (CR-CV-071).

Vấn đề: chưa có cách đo (a) người dùng có dùng Review không, (b) có ra quyết định nhanh hơn không, (c) cổng có bị bỏ qua/báo nhầm nhiều không — và chưa có ngưỡng số để quyết định chặn cứng (O9).

## 2. Giải pháp đề xuất

### 2.1 Nguyên tắc

Chỉ `z.enum`, `z.boolean` và khoảng (bucket) dạng enum; **không** có id (repo, worktree, commit, task, tenant, finding, rule), đường dẫn, tên người/nhánh/lens tuỳ biến, thông điệp, nội dung, số đếm thô (luôn qua khoảng), thời gian tuyệt đối. Mọi sự kiện `.strict()`. Số liệu chi tiết theo tenant (theo `ruleId`, theo người) lấy từ **bảng của chính tenant** (mục 2.6), không đưa lên telemetry sản phẩm.

### 2.2 Danh sách sự kiện (schema ở `review-telemetry-events.ts`, mới)

```ts
const lens = z.enum(['impact','architecture','dataflow','erd','storage','structure','contract','quality','requirements'])
const verdict = z.enum(['pass','warn','fail','unknown'])
const countBucket = z.enum(['0','1','2-3','4-10','11+'])
const largeBucket = z.enum(['0','1-3','4-10','11-30','31+'])
const latencyBucket = z.enum(['<1m','<5m','<30m','<4h','>=4h'])
const tool = z.enum(['oxlint','tsc','vitest','go-vet','go-test','golangci-lint','buf','opa','repo-rules','structure','security','other'])
const provider = z.enum(['github','gitlab','azure-devops','gitea','none'])
```

| Sự kiện | Trường (tất cả bắt buộc, `.strict()`) | Khi phát |
|---|---|---|
| `review_opened` | `source: enum('agent_row','source_control','cmd_k','right_sidebar','notification','restore')`, `scope: enum('merge_base','committed','custom_base')`, `lens` (lens đầu tiên), `index: enum('fresh','stale','missing','unknown')`, `after_agent_turn: boolean` | Sau khi tab Review mở (qua `openReviewFromEntryPoint`, CR-CV-061) |
| `review_lens_viewed` | `lens`, `dwell: enum('<10s','<1m','<5m','>=5m')` | Rời một lens, ở lại ≥ 2 s; tối đa một lần/lens/lần mở tab |
| `quality_gate_viewed` | `verdict`, `reasons: countBucket`, `stale: boolean`, `surface: enum('review','source_control')`, `source: enum('local','ci','mixed')` | Lần đầu thấy kết luận cho một (worktree, HEAD) trong phiên; dedupe ở client |
| `review_decision_made` | `decision: enum('commit','create_review','send_to_agent','mark_reviewed','abandon')`, `latency: latencyBucket`, `used_review: boolean`, `gate: enum('pass','warn','fail','unknown','none')`, `open_findings: countBucket` | Hành động đầu tiên sau một lượt agent xong (mục 2.3) |
| `review_findings_summary` | `shown: largeBucket`, `dismissed: largeBucket`, `waived: countBucket`, `resolved: largeBucket` | Khi đóng tab Review hoặc ra quyết định; một lần/lượt |
| `quality_finding_triaged` | `action: enum('dismiss','waive','restore','revoke')`, `reason: enum('not_applicable','accepted_risk','false_positive','later','other')`, `severity: enum('error','warning','info')`, `tool`, `blocking: boolean` | Mỗi hành động của người dùng trên một phát hiện (CR-CV-059/085) |
| `review_report_exported` | `format: enum('markdown_copy','markdown_pr_insert','html_save')`, `provider`, `truncated: boolean` | CR-CV-090 |
| `review_ai_summary` | `outcome: enum('ok','bad_output','error','disabled')`, `level: enum('metadata','diff')`, `cache_hit: boolean`, `feedback: enum('none','useful','wrong')` | CR-CV-093 (feedback gửi bằng sự kiện cập nhật cùng tên với `outcome='ok'`) |

`reason` lấy đúng tập lý do ở CR-CV-059 (`not_applicable|accepted_risk|false_positive|later`, "đề xuất") + `other`; nếu CR-CV-059 đổi tập thì cập nhật cùng lúc. `lens` là tập khởi điểm của `ReviewLensId` (CR-CV-051) cộng `quality`, `requirements` (CR-CV-087/092); thêm giá trị enum cần phát hành schema trước client (validator bỏ giá trị lạ).

Không có sự kiện riêng cho "tỉ lệ báo nhầm": tính từ `quality_finding_triaged{reason:'false_positive'}` chia cho `review_findings_summary.shown` (theo khoảng). `tool` thay cho `ruleId` để giữ độ phân giải thấp.

### 2.3 "Thời gian từ agent xong đến quyết định"

Hàm thuần `lib/review-decision-tracker.ts` (trạng thái trong bộ nhớ, **không lưu bền**): nhận `AgentTurnCompletion` (CR-CV-061 `selectAgentTurnCompletions`: `worktreeId`, `doneAt`) và đăng ký "lượt đang chờ quyết định" cho worktree. Khi có hành động quyết định đầu tiên cho worktree đó, tính `latency = now − doneAt`, ghi **một** `review_decision_made`, rồi xoá lượt chờ. Quyết định gồm: commit thành công (`handleCommit` trả `true`, `SourceControl.tsx:1792`), tạo review thành công (`handlePullRequestCreated`, gọi ở ~dòng 3077), gửi ghi chú lại cho agent (`DiffNotesSendMenu` thành công, CR-CV-060), đánh dấu đã xem hết (CR-CV-052), hoặc bỏ lượt (đóng worktree/bắt đầu lượt mới trước khi quyết định → `abandon`). `used_review` = có mở tab Review của worktree kể từ `doneAt`; nhờ đó so sánh nhóm có/không dùng Review (đường cơ sở). Đây là các **vài dòng gọi** ở ba nơi; mọi logic nằm trong tệp mới (tránh phình `SourceControl.tsx`, vốn đã ~6 700 dòng; CR-CV-061 mục 1.2).

Hạn chế đã biết: mất khi tắt ứng dụng giữa chừng; lượt không có hook (agent lạ) không có `AgentTurnCompletion` nên không được đo; nhiều lượt chồng nhau chỉ tính lượt mới nhất.

### 2.4 Hàm bọc phía renderer

`frontend/src/renderer/src/lib/review-telemetry.ts`: `trackReviewOpened`, `trackReviewLensViewed`, `trackQualityGateViewed`, `trackReviewDecisionMade`, `trackReviewFindingsSummary`, `trackQualityFindingTriaged`, `trackReviewReportExported`, `trackReviewAiSummary`, cùng `bucketCount(n)`, `bucketLarge(n)`, `bucketLatencyMs(ms)`, `bucketDwellMs(ms)`. Component **không** gọi `track()` trực tiếp (như `mcp-telemetry.ts`). Cờ tính năng tắt → các hàm không làm gì (không có UI nên cũng không có sự kiện). Web: `track` đã là no-op; không thêm đường khác.

### 2.5 Tích hợp catalog tương tác tính năng (tuỳ chọn)

Để có milestone dùng-nhiều cho cả tính năng (không chỉ sự kiện thô), thêm 3 id vào `FeatureInteractionId`: `code-review`, `quality-gate`, `review-ai-summary` (cùng danh mục trong `feature-interaction-categories.ts`; cần đọc bảng ánh xạ và test "mọi id có category" trước khi sửa — **chưa đọc**). Nếu chi phí lớn, bỏ mục này; sự kiện ở 2.2 đủ cho các câu hỏi của E5.

### 2.6 Số liệu phía server (không phải telemetry sản phẩm)

Bổ sung cho CR-CV-071 (chỉ đề xuất tên; không thêm hạ tầng): bộ đếm Prometheus có nhãn enum (`verdict`, `check_category`, `result`) — `codeintel_quality_gate_evaluations_total`, `codeintel_quality_waivers_total{kind}`, `codeintel_quality_gate_unknown_total{reason}`; **không** nhãn theo tenant/repo/rule (cardinality). Phân tích chi tiết theo tenant dùng truy vấn quản trị trên `quality_trend_points`, `quality_waivers`, `finding_dismissals`, `agent_turns` (dữ liệu của chính tenant, không bias như telemetry): tỉ lệ `unknown`, tỉ lệ miễn trừ trên `fail`, quy tắc bị bỏ qua nhiều nhất (`ruleId`/`finding_key`). Đây là nguồn chính để hiệu chỉnh ngưỡng; telemetry chỉ cho tín hiệu liên-tenant ở mức tổng quát.

### 2.7 Quyền riêng tư, đồng ý, opt-out

- Dùng nguyên cơ chế sẵn có: không thêm banner hay công tắc mới; sự kiện mới chịu cùng `resolveConsent` (tắt bởi `DO_NOT_TRACK`, CI, Privacy pane). Khi `effective !== 'enabled'` thì main không phát; hàm bọc không cần kiểm.
- Tài liệu riêng tư công khai (`PRIVACY_URL`, `lib/telemetry.ts`) liệt kê loại sự kiện: cần cập nhật khi phát hành (tài liệu đó nằm ngoài repo này; chưa kiểm chứng).
- `install_id` ẩn danh vẫn cho phép nối các sự kiện cùng một cài đặt theo thời gian; chấp nhận như các sự kiện hiện có, nhưng **cấm** thêm bất kỳ trường nào cho phép nối với repo/tenant/người.
- Doanh nghiệp có chính sách riêng: telemetry đã do desktop quản (không đổi). Có cần kill-switch theo tenant không là câu hỏi mở (Q3).
- Giới hạn tốc độ sẵn có (30/phút/tên, 1 000/phiên) đủ cho khối lượng dự kiến (vài sự kiện mỗi lượt); không cần ngoại lệ.

### 2.8 Ngưỡng nâng cổng từ chỉ báo lên chặn

Mục tiêu: có tiêu chí **số**, quyết bằng dữ liệu, không bằng cảm tính (research 11 §7: "báo nhầm làm mất niềm tin... chỉ nâng cấp khi dữ liệu đủ tốt"). Các số dưới đây là **đề xuất khởi điểm, chưa hiệu chỉnh**, áp **từng kiểm tra** (không phải cả cổng), trên cửa sổ ≥ 4 tuần ở chế độ `inform`:

| Chỉ số | Nguồn | Điều kiện để kiểm tra đó được xét chặn |
|---|---|---|
| Cỡ mẫu | server (`quality_trend_points`) | ≥ 200 lần đánh giá cổng và ≥ 50 kết luận `fail` của kiểm tra đó, từ ≥ 5 worktree và ≥ 3 người |
| Tỉ lệ báo nhầm | server (`finding_dismissals`/`quality_waivers` theo `reason=false_positive`) trên phát hiện của kiểm tra | ≤ 5% phát hiện hiển thị bị gắn `false_positive` |
| Tỉ lệ miễn trừ khi `fail` | server | ≤ 10% kết luận `fail` bị miễn trừ (CR-CV-085 `waived`) |
| Tỉ lệ `unknown` | server | ≤ 10% lượt đánh giá (dữ liệu đủ tin cậy mới chặn được; không chặn trên `unknown`) |
| Tín hiệu hữu ích | telemetry: `review_decision_made` sau `gate:'fail'` | ≥ 30% chuyển thành `send_to_agent` hoặc commit sau khi sửa (không phải `create_review` ngay), và `latency` trung vị của nhóm `used_review=true` không dài hơn nhóm `false` quá 20% |
| Độ ổn định | server | không có sự cố báo nhầm hàng loạt (≥ 10 báo nhầm cùng quy tắc trong 24 giờ) trong 2 tuần cuối |
| Thời gian chạy | CR-CV-071 | kiểm tra chạy xong trong ngân sách (không chặn người dùng chờ lâu) |

Chỉ khi **đủ mọi điều kiện** mới trình quyết định nâng một kiểm tra cụ thể lên `block` (và vẫn là CR riêng: chốt điểm chặn, ai được bỏ qua, quay lui). Telemetry chỉ là một chân; chân chính là dữ liệu server của chính tenant. Với tenant dùng web (không có telemetry), chỉ dùng dữ liệu server.

### 2.9 Cấu trúc file (mới)

`frontend/src/shared/review-telemetry-events.ts` (schema, ~70 dòng), `review-telemetry-events.test.ts` (theo mẫu `mcp-telemetry-events.test.ts`); `frontend/src/renderer/src/lib/review-telemetry.ts`, `review-decision-tracker.ts` (+ test); bản sao ở `desktop/src/shared/`, `backend/src/shared/`, `tests/vendor-shared/shared/` theo cơ chế đồng bộ được xác nhận. Tên theo khái niệm, không `helpers/utils`; không `max-lines` disable (`telemetry-events.ts` có ngoại lệ sẵn từ trước, không mở rộng thêm).

## 3. Quyết định thiết kế

1. **Chỉ client, không PostHog ở Go**: tránh một kênh thu thập mới; phía server dùng số đo vận hành và bảng của tenant.
2. **`tool` thay `ruleId`**: giữ độ phân giải thấp; chi tiết rule ở dữ liệu của tenant.
3. **Một sự kiện quyết định mỗi lượt**: tránh phình số lượng và phép tính trùng.
4. **Đường cơ sở `used_review=false`**: không có nhóm đối chứng thì không đánh giá được hiệu quả.
5. **Không thêm công tắc đồng ý mới**: dùng consent hiện có.
6. **Ngưỡng nâng cổng tách khỏi sản phẩm**: là tiêu chí quản trị, ghi vào tài liệu vận hành (CR-CV-073).

## 4. Tiêu chí chấp nhận

- [ ] Mỗi sự kiện ở 2.2 có schema `.strict()` chỉ gồm `z.enum`/`z.boolean`; test: payload hợp lệ qua; thêm khoá `repo_id`, `worktree_id`, `commit`, `path`, `rule_id`, `message`, `task_id`, `tenant_id`, `user`, `email`, `count` đều bị từ chối.
- [ ] Các schema được đăng ký trong `eventSchemas` và `EventMap` suy ra kiểu; gọi `track('review_opened', {…khoá lạ…})` không biên dịch được.
- [ ] Validator desktop (`validator.ts`) chấp nhận payload hợp lệ và bỏ payload sai (test bằng bản sao `desktop/src/shared`); các bản sao `shared/` khớp nội dung (kiểm tra bằng test/so sánh).
- [ ] Hàm chia khoảng có test biên (0, 1, 3, 10, 11, 30, 31; 59 999 ms/60 000 ms; …); không có hàm nào trả giá trị ngoài enum.
- [ ] `review-decision-tracker`: một lượt → đúng một `review_decision_made`; nhiều quyết định liên tiếp chỉ tính đầu tiên; `used_review` đúng; lượt bị thay bởi lượt mới → `abandon`; không phát sinh khi cờ tính năng tắt.
- [ ] Tắt telemetry (`DO_NOT_TRACK=1`, CI, Privacy pane): không sự kiện nào ra khỏi desktop main (test với consent giả); web build không phát lỗi khi gọi các hàm bọc.
- [ ] Không có trường nào phụ thuộc nội dung do người dùng nhập (ghi chú, lý do tự do, tên nhánh).
- [ ] Tài liệu vận hành (CR-CV-073) có mục "Tiêu chí nâng cổng" khớp bảng 2.8 và nêu rõ các số là khởi điểm chưa hiệu chỉnh.
- [ ] Giới hạn tốc độ không bị chạm trong kịch bản e2e (≤ 10 sự kiện/lượt).

## 5. Kiểm thử

- **Unit**: schema (mẫu `mcp-telemetry-events.test.ts`, cộng bảng khoá cấm ở trên), hàm chia khoảng, tracker (đồng hồ giả), mapping `ReviewLensId` → enum.
- **Hợp đồng**: test so khớp các bản sao `shared/` (đọc nội dung hai thư mục, hoặc dùng cơ chế sẵn có nếu tìm thấy).
- **Tích hợp renderer**: mock `window.api.telemetryTrack`, chạy kịch bản "agent xong → mở Review → bỏ qua phát hiện → commit" và khẳng định chuỗi sự kiện.
- **Quyền riêng tư**: quét tĩnh `review-telemetry.ts` không import kiểu có trường định danh; test snapshot payload.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Bias mẫu**: chỉ desktop đã đồng ý, bỏ qua web và CI; lượt agent không có hook không được đo. Không dùng telemetry làm nguồn duy nhất cho quyết định chặn.
- **Chưa tìm thấy cơ chế đồng bộ `shared/`** giữa 4 vị trí; sửa lệch một bản làm validator bỏ sự kiện im lặng (chỉ cảnh báo giới hạn tần suất). Cần kiểm chứng trước khi triển khai.
- Ngưỡng 2.8 là phỏng đoán; `reason=false_positive` do người dùng tự gắn nên có thiên lệch (người ngại thao tác sẽ không gắn).
- Tập `lens`/`reason`/`tool` là tập khởi điểm, phụ thuộc CR-CV-051/059/082/087 chưa triển khai.
- Chưa đọc `feature-interaction-categories.ts` và test ràng buộc (2.5).
- `used_review` chỉ biết trong phiên ứng dụng; đa thiết bị không gộp.

## 7. Câu hỏi mở

- **Q1.** Có phát sự kiện telemetry từ phía server (qua desktop không có) cho người dùng chỉ dùng web? Hiện không.
- **Q2.** Có cần sự kiện cho "chạy kiểm tra" (`quality_run_requested`, kích hoạt, kết quả)? Hữu ích để đo tải và adoption nhưng nằm ngoài danh sách E5; để CR-CV-081 quyết.
- **Q3.** Tenant doanh nghiệp có cần kill-switch telemetry theo tenant (ngoài consent của người dùng) không?
- **Q4.** Ngưỡng 2.8 do ai duyệt (sản phẩm hay kỹ thuật) và chu kỳ xét lại?
- **Q5.** Có đưa số đo vận hành của 2.6 vào CR-CV-071 hay tách?

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (O9; mục 8)
- `/opt/repos/orca/docs/research/view-code/11-additions-for-quality-control.md` (E5, §7)
- `/opt/repos/orca/frontend/src/shared/mcp-telemetry-events.ts`, `mcp-telemetry-events.test.ts`, `telemetry-events.ts` (đầu tệp; `eventSchemas` ~`:1406`; `commonPropsSchema` `:1654`), `telemetry-consent-types.ts`, `feature-interaction-catalog.ts`, `feature-interaction-usage-buckets.ts`
- `/opt/repos/orca/frontend/src/renderer/src/lib/mcp-telemetry.ts`, `lib/telemetry.ts`
- `/opt/repos/orca/desktop/src/main/telemetry/validator.ts`, `consent.ts`, `burst-cap.ts`; `/opt/repos/orca/desktop/src/preload/index.ts` (`:1843`)
- `/opt/repos/orca/docs/crs/v7/review-frontend/CR-CV-051-review-workspace-shell.md` (`ReviewLensId`), `CR-CV-059-contract-lens-and-findings.md`, `CR-CV-060-review-notes-send-to-agent-and-turn-compare.md`, `CR-CV-061-review-entry-points.md`
- `/opt/repos/orca/frontend/src/renderer/src/components/right-sidebar/SourceControl.tsx` (`handleCommit` `:1792`, `handlePullRequestCreated` ~`:3077`)
- `/opt/repos/orca/docs/crs/v7/quality-rollout/CR-CV-071-performance-budgets-metrics-tracing.md`, `CR-CV-073-e2e-feature-flag-rollout-runbook.md`
- CR cùng nhóm (chỉ ID): CR-CV-081, 082, 085, 087, 089, 090, 093
- `/opt/repos/orca/AGENTS.md`, `/opt/repos/orca/guides/STYLEGUIDE.md`

# FE-CV-SOL-093-ai-summary-panel: Thẻ "Tóm tắt do AI suy luận" (mặc định tắt)

> Trạng thái (2026-10-07): 5/6 task DONE, 1 PARTIAL, 0 BLOCKED, 0 TODO. Xem mục "Ghi chú triển khai" của từng task; code thật lệch spec ở các điểm đã ghi.

**CR:** [CR-CV-093](../../../../../../docs/crs/v7/quality-gate/CR-CV-093-ai-review-summary.md) (phần frontend; backend `BE-CV-SOL-093-ai-review-summary`). Priority P2, O13.
**Area:** frontend (`components/review-map/ai-summary/`)
**Hợp đồng áp dụng:** [CONTRACT-codeintel-ui-api.md](../../../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md) 2.3 (`ai-disabled`, `ai-error`, `rate-limited`, `timeout`), 2.4, 3.2 (`quality.summary` và ghi chú 120 s), 4.1 (`Settings.effective.aiReviewEnabled`, `tenant.aiReviewLevel`), 4.7 (`AiReviewSummary`); PQ-01 (`CODEINTEL_AI_REVIEW_DISABLED`), PQ-13 (timeout), PQ-23, mục 6.1, 7.1, 8.3.
**TDD tham chiếu:** [v5/05-ui-components](../../../../tdd/v5/05-ui-components.md), [v5/07-hooks-and-ipc](../../../../tdd/v5/07-hooks-and-ipc.md).

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `components/ui/` (`dialog.tsx`, `card.tsx`, `badge.tsx`, `button.tsx`, `collapsible.tsx`, `tooltip.tsx`), `right-sidebar/SourceControl.tsx:2767` vùng sinh mô tả PR bằng AI (đường **riêng**, không thay thế), `lib/telemetry.ts`, `guides/STYLEGUIDE.md` ("UI copy must not overclaim"), `i18n/i18n.ts`. Không có code AI-summary nào ở frontend.

**Correction relative to CR-CV-093 (hợp đồng và mã thật thắng):**

| # | CR ghi | Thực tế / hợp đồng | Quyết định |
|---|---|---|---|
| 1 | Thử một lần, `CODEINTEL_TIMEOUT` ở 30 s | Hợp đồng 3.2: `quality.summary` với `dryRun:false` trả `CODEINTEL_TIMEOUT` kèm `{"inProgress":true,"retryAfterMs":3000}` sau ≤ 24 s và hoàn tất nền (cache 24 h); client **thử lại** (PQ-13) | Hook thử lại mỗi `retryAfterMs` tới tối đa 90 s tổng; hiển thị "Đang tạo tóm tắt…" có thể huỷ |
| 2 | `CODEINTEL_INVALID_ARGUMENT` | PQ-03: bỏ, dùng `CODEINTEL_INVALID_PARAMS` | Phân loại theo bảng 2.3 |
| 3 | Biến `CODE_INTEL_AI_REVIEW_ENABLED` | PQ-23: `CODEINTEL_AI_REVIEW_ENABLED` (backend) | Không ảnh hưởng frontend; cờ đọc từ `Settings.effective.aiReviewEnabled` |
| 4 | `labels{ai_inferred, …}` snake_case | H1 camelCase; hợp đồng chỉ ghi `labels` | Parser nhận `aiInferred`, chịu `ai_inferred`; **không** phụ thuộc: nhãn "AI suy luận" luôn hiển thị theo `summary` có mặt, không theo cờ trường (câu hỏi mở 1) |
| 5 | `manifest`, `cache` | Hợp đồng không định nghĩa kiểu; backend: `AiReviewManifest` | Kiểu cục bộ **đề xuất** theo CR mục 2.3: `manifest{level, files[{path,bytes,hunks,withheld?}], findingsCount, redactions, totalBytes, estimatedTokens, provider?}`, `cache{hit, createdAt, expiresAt}`; parser chịu thiếu trường (câu hỏi mở 1) |
| 6 | `AiReviewSummary.modelReportedByAgent` | Hợp đồng 4.7 không có | Bỏ |
| 7 | Hai cờ tenant, xác nhận hai bước khi chuyển `diff` ở "UI quản trị" | `settings.set` thuộc admin; màn cài đặt do FE-CV-SOL-073-flag-gating-and-web-e2e hoặc chủ khác | **Ngoài phạm vi** solution này; thẻ chỉ đọc cấp hiện hành (câu hỏi mở 2) |
| 8 | Chèn vào báo cáo (CR-090) | `ReviewReportModel` không có mục AI | Qua `extraSections` của bộ dựng Markdown của FE-CV-SOL-090 (task 05); nằm **ngoài** mục "Cổng chất lượng" |
| 9 | Quyền `quality_read ∧ read_source` | Lỗi `forbidden` nếu thiếu | Nút ẩn/khoá khi `forbidden`, không toast đỏ |

**Chưa kiểm chứng:** hình dạng thật của `manifest`/`cache`; bao lâu backend hoàn tất nền (chưa đo); chất lượng tóm tắt (CR 2.9, đánh giá bằng tay trước khi bật cho tenant).

## 2. Giải pháp

### 2.1 Cây file

```
frontend/src/renderer/src/components/review-map/ai-summary/
  ai-summary-wire-parser.ts           (mới) parse AiReviewSummary/manifest/cache, lọc, giới hạn độ dài hiển thị
  use-review-ai-summary.ts            (mới) hook: dry-run, xác nhận, tạo, thử lại, huỷ
  ai-summary-consent-state.ts         (mới) xác nhận một lần mỗi phiên theo mức dữ liệu (bộ nhớ, không bền)
  AiSummaryDataPreviewDialog.tsx      (mới) manifest + xác nhận
  ReviewAiSummaryCard.tsx             (mới) thẻ thu gọn mặc định
  ai-summary-report-section.ts        (mới) mục Markdown có nhãn cho báo cáo (090)
  (mỗi file có .test.ts(x))
i18n/locales/*.json (sửa) + i18n/ai-summary-locale-coverage.test.ts (mới)
```

### 2.2 Chữ ký

```ts
export type AiSummaryManifest = { level: 'metadata'|'diff'; files: { path: string; bytes: number; hunks: number; withheld?: string }[]
  findingsCount: number; redactions: number; totalBytes: number; estimatedTokens: number; provider?: string; suspectedInjection?: boolean }
export type AiSummaryView = { summary: AiReviewSummary; manifest: AiSummaryManifest | null
  cache: { hit: boolean; createdAt?: string; expiresAt?: string } | null }

export function parseAiSummaryResponse(raw: unknown): { summary: AiReviewSummary | null; manifest: AiSummaryManifest | null; cache: AiSummaryView['cache'] }

export function useReviewAiSummary(args: { projectId: string|null; worktreeId: string|null; base?: string; locale: string }): {
  state: 'hidden'|'idle'|'previewing'|'awaiting-consent'|'generating'|'ready'|'error'
  allowedLevels: ('metadata'|'diff')[]               // <= Settings.tenant.aiReviewLevel
  manifest: AiSummaryManifest | null; view: AiSummaryView | null
  error: 'ai-disabled'|'no-relay'|'bad-output'|'rate-limited'|'timeout'|'forbidden'|'unknown' | null
  preview(level: 'metadata'|'diff'): Promise<void>  // dryRun:true, KHÔNG gọi agent
  confirmAndGenerate(opts?: { forceRefresh?: boolean }): Promise<void>
  cancel(): void
}
```

### 2.3 Hành vi

- `state:'hidden'` khi `flags.ai=false` (`Settings.effective.aiReviewEnabled`; mặc định tắt) hoặc thiếu `projectId`: **không render, không gọi RPC**. Gặp `ai-disabled`/`quality-disabled` → `hidden` lặng lẽ.
- **Không bao giờ tự tạo.** Luồng: nút "Tạo tóm tắt (AI)" → `preview(level)` gọi `quality.summary {dryRun:true}` → hiện `AiSummaryDataPreviewDialog` (danh sách tệp gửi đi, số lần che, ước lượng token, mục `withheld`, cảnh báo `suspectedInjection`, dòng "Nội dung sẽ gửi tới nhà cung cấp LLM mà dev server đang dùng") → người dùng xác nhận (bắt buộc lần đầu mỗi phiên và mỗi lần đổi mức `level`, `ai-summary-consent-state`) → `confirmAndGenerate` gọi `{dryRun:false}`.
- `level` mặc định `metadata`; `diff` chỉ chọn được khi `Settings.tenant.aiReviewLevel === 'diff'`.
- `CODEINTEL_TIMEOUT` có `inProgress` → thử lại mỗi `retryAfterMs` (3 s), tối đa 90 s tổng, huỷ được (`AbortSignal`: bỏ kết quả, không huỷ RPC phía server); hết hạn → lỗi `timeout` với nút "Thử lại". `RATE_LIMITED` hiện `retryAfterSeconds`. `AI_NO_RELAY` → "Chưa có dev server kết nối". `AI_BAD_OUTPUT` → "Không tạo được tóm tắt hợp lệ" (không bao giờ hiện văn bản thô). `SECRET_LEAK_BLOCKED` → `tool-failed` chung.
- Hiển thị **văn bản thuần** (React escape mặc định; cấm `dangerouslySetInnerHTML`, cấm render Markdown/HTML/liên kết từ `summary`, `risks[].text`, `readFirst[].why`); `refs`/`readFirst[].file` chỉ là chữ có thể bấm để mở tệp **nếu** nằm trong tập tệp đã đổi của overlay (kiểm lần hai ở client); `refsDropped>0` → "N tham chiếu không hợp lệ đã bị bỏ".
- **Nhãn bắt buộc**, luôn hiện khi có `summary`: tiêu đề "Tóm tắt do AI suy luận"; dòng phụ "Có thể sai hoặc thiếu. Hãy đối chiếu với diff. Mô hình: {model}. Dữ liệu gửi: {level}." Cấm: "AI đã review", "đã kiểm tra", "phê duyệt", "an toàn". Tóm tắt **không bao giờ** nằm trong khối cổng chất lượng và không ảnh hưởng `verdict`.
- Phản hồi: hai nút "Hữu ích" / "Không đúng" gửi telemetry chỉ-enum qua FE-CV-SOL-095 (không nội dung); không có kênh backend cho phản hồi.

```
┌ Tóm tắt do AI suy luận ─────────────────────────── [Tạo lại] ┐
│ Có thể sai hoặc thiếu. Hãy đối chiếu với diff.               │
│ Mô hình: claude-… · Dữ liệu gửi: metadata                    │
│ Thay đổi chính ... (văn bản thuần)                           │
│ Rủi ro:  • …   Nên đọc trước:  a.ts — lý do                  │
│ 1 tham chiếu không hợp lệ đã bị bỏ    [Hữu ích] [Không đúng] │
└──────────────────────────────────────────────────────────────┘
Xem trước dữ liệu gửi (Dialog): 12 tệp · 3 lần che · ~2 100 token
  [Hủy]                                   [Gửi và tạo tóm tắt]
```

Thẻ thu gọn mặc định (`ui/collapsible`); `Dialog` cho xác nhận (STYLEGUIDE: quyết định cần trước khi tiếp tục); nút chính `default` chỉ ở dialog; Hủy là ghost; token `card`, `border`, `muted-foreground`; icon `Sparkles`/`Loader2`; spinner hiện sau ~200 ms, nút khoá ngay. Chuỗi qua `translate()`; ngôn ngữ trả lời theo `locale` UI gửi lên (frontend chỉ truyền).

## 3. Quyết định thiết kế

- Hai lớp bảo vệ ở frontend: không bao giờ tự gọi; không bao giờ render nội dung mô hình như HTML.
- Xác nhận dữ liệu gửi là chức năng chính, không phải chi tiết kỹ thuật (dữ liệu rời dev server tới nhà cung cấp LLM).
- Không lưu tóm tắt ở bộ nhớ bền của trình duyệt (cache 24 h ở backend); không telemetry nội dung.
- Không thêm thư viện.

## 4. Phụ thuộc chéo khu vực

| Cần | Nơi |
|---|---|
| Kênh `quality.summary` | `BE-CV-SOL-093-ai-review-summary` (gồm ngoại lệ timeout `ai.complete` 120 s ở infra-fleet), `BE-CV-SOL-040-codeintel-quality-channels`; trước G3: fake backend G4 (fixture: `summary`, `manifest`, `TIMEOUT inProgress` rồi thành công, `AI_REVIEW_DISABLED`, `AI_BAD_OUTPUT`, `AI_NO_RELAY`, `RATE_LIMITED`) |
| Cờ AI, cờ chất lượng | `FE-CV-TASK-085-01`, `FE-CV-SOL-073-flag-gating-and-web-e2e` |
| Chỗ đặt thẻ | `FE-CV-SOL-051-review-workspace-shell` (thanh/ngăn tóm tắt) |
| Mục chèn vào báo cáo | `FE-CV-SOL-090-review-report-export` (`extraSections`) |
| Telemetry | `FE-CV-SOL-095-review-telemetry` (`review_ai_summary`) |
| AG | Đường `ai.complete` đã có ở agent (`agent/src/relay/ai-complete-handler.ts`); không có solution AG |

## 5. Tiêu chí chấp nhận

- [ ] `flags.ai=false`: thẻ không render và 0 lời gọi RPC.
- [ ] `preview` dùng `dryRun:true` và không bao giờ kích hoạt tạo; tạo chỉ sau xác nhận; xác nhận lại khi đổi `level`.
- [ ] `TIMEOUT inProgress` được thử lại tới 90 s, huỷ được; kết quả đến sau huỷ bị bỏ.
- [ ] `summary` chứa `<script>`, `<img onerror>`, liên kết Markdown, khối mã → hiển thị dạng chữ, không phần tử mới.
- [ ] `refs` ngoài tập tệp đã đổi không bấm được.
- [ ] Nhãn "AI suy luận", model, level luôn hiện; không cụm cấm.
- [ ] `AI_BAD_OUTPUT` không lộ văn bản thô.
- [ ] Thẻ không đổi `verdict` hay khối cổng (test cấu trúc: module không import hook cổng).
- [ ] Hoạt động Electron và web; năm locale đủ khoá.

## 6. Kiểm thử (Vitest + Testing Library)

Parser (thiếu trường, enum lạ, dài quá giới hạn), hook (đếm lời gọi: cờ tắt 0; preview không tạo; retry với đồng hồ giả; huỷ; từng mã lỗi), consent state, dialog (focus mặc định vào nút an toàn Hủy hay Gửi: chọn **Hủy** là mặc định để tránh gửi nhầm bằng Enter — ghi rõ trong PR), thẻ với bộ ca injection hiển thị (HTML, bidi, `</DATA-…>`), phần Markdown cho báo cáo (nhãn nằm ngoài "Cổng chất lượng"). Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/ai-summary` (chưa chạy). Đánh giá chất lượng mô hình (CR 2.9) là thủ công, ngoài CI.

## 7. Rủi ro và điểm chưa kiểm chứng

- Dữ liệu mã có thể rời tổ chức; frontend chỉ làm hết sức minh bạch, không thay chính sách tenant.
- Chống injection chủ yếu ở backend; frontend chỉ đảm bảo không thực thi/không render HTML.
- Hình dạng `manifest`/`cache` chưa chốt trong hợp đồng.
- Thời gian tạo có thể vài chục giây qua SSH: giao diện chờ dài, cần huỷ rõ ràng.

## 8. Câu hỏi mở

1. Hợp đồng chưa định nghĩa kiểu `manifest`, `cache`, `labels` (và tên `aiInferred`); cần cập nhật `CONTRACT-codeintel-ui-api.md` mục 4.7.
2. Ai làm màn quản trị đổi `aiReviewLevel` (xác nhận hai bước khi `diff`)?
3. Có cho chèn tóm tắt AI vào mô tả PR không, hay chỉ xem trong Orca (mặc định task 05 chỉ cung cấp hàm, không có nút trong composer)?

## 9. Tham chiếu

`/opt/repos/orca/docs/crs/v7/quality-gate/CR-CV-093-ai-review-summary.md`, `/opt/repos/orca/guides/STYLEGUIDE.md`, `/opt/repos/orca/AGENTS.md`, `/opt/repos/orca/agent/src/relay/ai-complete-handler.ts`.

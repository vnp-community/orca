# FE-CV-SOL-090-review-report-export: Xuất báo cáo review (Markdown/HTML) và chèn vào mô tả PR/MR

> 📋 Proposed. Chưa triển khai. Viết ngày 2026-10-06 từ việc ĐỌC code `frontend/src` và hợp đồng v7; chưa chạy test hay ứng dụng.

**CR:** [CR-CV-090](../../../../../../docs/crs/v7/quality-gate/CR-CV-090-exportable-review-report.md) (phần frontend; mô hình do `BE-CV-SOL-090-review-report-model`)
**Area:** frontend (`components/review-map/report/`, `components/right-sidebar/`)
**Hợp đồng áp dụng:** [CONTRACT-codeintel-ui-api.md](../../../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md) 2.3, 2.4, 3.2 (`quality.report`), 4.7 (`ReviewReportModel`); [CONTRACT-codeintel-proto-and-data-map.md](../../../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md) PQ-01, PQ-04, PQ-13, PQ-32, mục 7.1, 8.3.
**TDD tham chiếu:** [v5/05-ui-components](../../../../tdd/v5/05-ui-components.md), [v5/07-hooks-and-ipc](../../../../tdd/v5/07-hooks-and-ipc.md), [v5/08-editor-and-files](../../../../tdd/v5/08-editor-and-files.md).

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `right-sidebar/CreateHostedReviewComposer.tsx` (372 dòng; `body`/`setBody` là prop), `CreateHostedReviewComposerFields.tsx` (textarea mô tả dòng ~92-104, `disabled={fieldsLocked}`), `SourceControl.tsx` (composer `:5247`, `createHostedReview` `:3064` gửi `body: prBody`, `useTemplate` `:3073`), `ChecksPanel.tsx:3611` (nơi gọi composer thứ hai), `useCreatePullRequestDialogFields.ts` (`setBody` ở :156/:229/:298/:340; field revisions), `components/editor/MermaidBlock.tsx` + `mermaid-config.ts` (`securityLevel:'strict'`, `htmlLabels=false`, `DOMPurify`), `components/settings/mcp/McpCopyButton.tsx`, `mcp-audit-csv.ts` (Blob + `a[download]`), `web/web-preload-api.ts:2847` (`writeClipboardText` dùng `navigator.clipboard?.writeText?.` — im lặng nếu không có), `desktop/src/preload/index.ts:3711` (`ui.writeClipboardText`), `assets/main.css` (token `--background`, `--foreground`, `--border`, `--destructive`, `--muted-foreground`; `.dark`).

**Correction relative to CR-CV-090 (hợp đồng và mã thật thắng):**

| # | CR ghi | Thực tế / hợp đồng | Quyết định |
|---|---|---|---|
| 1 | Frontend dựng Mermaid (`review-report-diagram-mermaid.ts`) từ mô hình | Hợp đồng 4.7: `diagrams[].{kind, mermaid, alt[], truncated}` — **backend đã trả chuỗi Mermaid** (≤ 6 KiB) | Frontend không dựng; chỉ **kiểm tra** (guard) chuỗi trước khi chèn vào hàng rào Markdown và trước khi `mermaid.render` |
| 2 | `risk.level` chữ thường `low\|…` | PQ-32: `LOW\|MEDIUM\|HIGH\|CRITICAL\|UNKNOWN` chữ HOA | Parser nhận chữ hoa; lạ → `UNKNOWN` |
| 3 | Q4: chỉ `code_intel_enabled` bật vẫn xuất báo cáo không có cổng | PQ-01 và backend: `quality.report` thất bại `CODEINTEL_QUALITY_GATE_DISABLED` khi cờ chất lượng tắt (cả `ExportReviewReport`) | Menu xuất và nút chèn ẩn khi `flags.quality=false`; gặp `quality-disabled` thì ẩn lặng lẽ |
| 4 | Tên kênh và tham số snake_case (`max_findings`) | Hợp đồng 3.2: `quality.report {projectId, worktreeId, base?, profileName?, turnKey?, sections?, maxFindings≤50 (10), maxReadingSteps≤50 (15), includePeople (false), includeWaiverReasons (false)}` | Dùng đúng; ngoài khoảng bị từ chối, không kẹp ngầm |
| 5 | Kết quả có `generated_for` | Hợp đồng chỉ ghi `generatedFor` không định nghĩa kiểu | Parser chịu thiếu trường; hiển thị chân văn bản chỉ dựa `model.subject`/`reproducibility` (câu hỏi mở 1) |
| 6 | Lưu HTML bằng hộp thoại lưu của desktop main | Chưa đọc kênh lưu tệp của preload; có tiền lệ `mcp-audit-csv.ts` (Blob + `a[download]`) chạy ở renderer | Dùng Blob + `a[download]` (cả Electron lẫn web); hộp thoại native là câu hỏi mở 2 |
| 7 | Copy bằng `navigator.clipboard.writeText` với fallback | Preload web `writeClipboardText` **nuốt** khi thiếu `navigator.clipboard` (ngữ cảnh không bảo mật) | Dùng `window.api.ui.writeClipboardText`, rồi kiểm tra lại; nếu không chắc thành công, fallback `textarea` + `document.execCommand('copy')` (chưa có tiền lệ trong repo; chưa kiểm chứng) |
| 8 | Điểm chèn composer: `SourceControl.tsx` | Có hai nơi gọi sản phẩm: `SourceControl.tsx:5247` và `ChecksPanel.tsx:3611`; `pr-create-dialog` là code chết | Prop mới ở composer, truyền từ cả hai nơi |
| 9 | `useTemplate` tương tác với `body` | Chưa đọc nơi main/git-gateway ghép mẫu với `body` (chưa kiểm chứng) | Báo cáo chỉ **nối thêm/thay đúng khối có dấu**, không thay mô tả người dùng |

**Chưa kiểm chứng:** giới hạn mô tả PR/MR theo từng nhà cung cấp và việc render Mermaid ở GitHub/GitLab (CR mục 1.4); ngân sách 20 000 ký tự là bảo thủ, chưa đối chiếu tài liệu nhà cung cấp.

## 2. Giải pháp

### 2.1 Cây file

```
frontend/src/renderer/src/components/review-map/report/
  review-report-model-parser.ts        (mới) parse + chuẩn hoá ReviewReportModel
  review-report-markdown.ts            (mới) (model, {t, provider, maxChars, extraSections?}) -> {markdown, truncated}
  review-report-diagram-guard.ts       (mới) kiểm tra chuỗi Mermaid
  review-report-html.ts                (mới) tệp HTML tự đủ
  review-report-theme-tokens.ts        (mới) đọc token màu lúc xuất (light + dark)
  review-report-export-actions.ts      (mới) copy, tải HTML
  use-review-report.ts                 (mới) hook gọi quality.report
  ReviewReportMenu.tsx                 (mới) menu "Xuất báo cáo"
  merge-review-report-into-body.ts     (mới) thay/nối khối có dấu
  (mỗi file có .test.ts(x))
right-sidebar/CreateHostedReviewComposer.tsx / CreateHostedReviewComposerFields.tsx (sửa) + prop onInsertReviewReport
right-sidebar/SourceControl.tsx, ChecksPanel.tsx (sửa, vài dòng): truyền hàm
i18n/locales/*.json (sửa) + i18n/review-report-locale-coverage.test.ts (mới)
```

### 2.2 Chữ ký

```ts
export type ReviewReportModel = /* hợp đồng 4.7, sao chép bởi FE-CV-SOL-050 */
export function parseReviewReportModel(raw: unknown): ReviewReportModel   // thiếu trường -> mặc định rỗng + warnings; enum lạ -> 'unknown'/'UNKNOWN'

export type ReportTranslate = (key: string, fallback: string, opts?: Record<string, unknown>) => string
export function buildReviewReportMarkdown(model: ReviewReportModel, opts: {
  t: ReportTranslate; provider: HostedReviewProvider; maxChars?: number /* 20000 */
  includeDiagrams?: boolean; extraSections?: { id: string; markdown: string }[]   // 093 chèn tại đây
}): { markdown: string; truncated: boolean }

export function guardMermaidSource(src: string): { ok: true; src: string } | { ok: false; reason: 'too_large'|'fence'|'init_directive'|'html'|'control_chars' }
export function buildReviewReportHtml(model: ReviewReportModel, opts: { t: ReportTranslate; locale: string; tokens: ReportThemeTokens; svgByDiagram: (string|null)[] }): string

export function mergeReviewReportIntoBody(body: string, markdown: string): string
// thay đúng cặp <!-- orca-review:start --> … <!-- orca-review:end -->; chưa có thì nối cuối sau một dòng trống
```

### 2.3 Quy tắc dựng

- **Markdown:** khối bao bởi cặp dấu `orca-review`; mục theo thứ tự ưu tiên giữ khi cắt: tiêu đề + cổng + rủi ro → phát hiện → hợp đồng/ERD → thứ tự đọc → sơ đồ (bỏ trước) → chân văn bản luôn giữ; không cắt giữa bảng/khối mã; thêm dòng "Đã rút gọn; xem đầy đủ trong Orca" và `truncated=true`. Bảng thoát `|`, backtick, `<`, `[`. Chữ cổng không overclaim: `pass` → "Các kiểm tra bắt buộc đã chạy đều đạt"; `unknown` → "Chưa đủ dữ liệu để kết luận"; cấm "an toàn để merge", "đã duyệt". `warnings` (`no_gate`, `index_stale`, `overlay_partial`) dịch thành dòng chữ. Chữ "PR/MR" từ `localizedHostedReviewCopy`. Không tên người, không lý do miễn trừ (mặc định `includePeople=false`, `includeWaiverReasons=false`). Mã nguồn không có trong mô hình; `message` đã bị backend cắt ≤ 160.
- **Sơ đồ trong Markdown:** khối ```` ```mermaid ```` chỉ khi `guardMermaidSource` ok; kèm `<details>` danh sách `alt[]` (≤ 12 dòng); `diagrams[].truncated` hoặc guard hỏng → bỏ sơ đồ, vẫn giữ `alt[]`. Không nhúng ảnh/SVG vào Markdown.
- **HTML:** một tệp tự đủ, không `<script>`, không URL ngoài, `<meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src data:; style-src 'unsafe-inline'">`, `lang` theo locale, `<caption>` cho bảng, `<title>/<desc>` + văn bản thay thế cho mỗi SVG; mọi chuỗi từ mô hình escape HTML. SVG tạo bằng `mermaid.render` với `getMermaidConfig(isDark,false)` rồi `DOMPurify` (cùng `MermaidBlock`), nạp lười `import('mermaid')`. Màu **không hard-code trong nguồn**: `review-report-theme-tokens.ts` đo token bằng phần tử thử có/không lớp `dark` dưới `document.body` (`getComputedStyle().getPropertyValue('--background' …)`) và điền vào hai khối `:root` + `@media (prefers-color-scheme: dark)` của tệp xuất.
- **Xuất:** nút "Sao chép Markdown", "Sao chép cho mô tả PR/MR" (ngân sách 20 000), "Lưu HTML" (`orca-review-<repoSlug>-<head7>.html`, không đường dẫn tuyệt đối). Toast xác nhận cho sao chép (STYLEGUIDE: toast cho xác nhận thoáng qua); lỗi sao chép hiện inline/toast lỗi, không im lặng.
- **Hook:** `useReviewReport()` gọi `quality.report`; `CODEINTEL_TIMEOUT` có `inProgress` → thử lại sau `retryAfterMs`, tối đa 90 s; không gọi khi `flags.quality=false`; khoá nút ngay khi bấm, spinner hiện sau ~200 ms (SSH).

### 2.4 Tích hợp giao diện

```
ReviewSummaryBar (FE-CV-SOL-051):   [Xuất báo cáo ▾]  -> Sao chép Markdown | Sao chép cho mô tả MR | Lưu HTML
Form tạo PR/MR (composer):          Mô tả [textarea .................]   [Chèn báo cáo review]
```

`ReviewReportMenu` dùng `ui/dropdown-menu`; "Chèn báo cáo review" là `Button variant="outline" size="xs"` cạnh ô mô tả (trong `CreateHostedReviewComposerFields`), prop `onInsertReviewReport?: () => Promise<string | null>` ở composer; vắng prop thì không render. `SourceControl.tsx` và `ChecksPanel.tsx` chỉ truyền hàm gọi `useReviewReport` rồi `mergeReviewReportIntoBody`, cập nhật qua `setPrBody`. **Không chèn khi `generating`** (nút disable, tránh đua với field revisions của `useCreatePullRequestDialogFields`). Không tự chèn: chỉ khi người dùng bấm.

## 3. Quyết định thiết kế

- Backend trả mô hình, frontend dựng chữ (H6); một bộ dựng duy nhất, không catalog chuỗi thứ hai ở Go.
- Mermaid ở Markdown là mã, không ảnh; guard chặn chuỗi nguy hiểm (hàng rào ``` lồng, `%%{init`, thẻ HTML, ký tự điều khiển).
- Khối có dấu để chèn lại idempotent.
- Ngân sách ký tự bảo thủ một giá trị cho mọi provider tới khi kiểm chứng.
- Không dùng AI để dựng báo cáo; tóm tắt AI (FE-CV-SOL-093) chèn riêng có nhãn qua `extraSections`.
- Không thêm thư viện (mermaid, dompurify đã có).

## 4. Phụ thuộc chéo khu vực

| Cần | Nơi |
|---|---|
| Kênh `quality.report` | `BE-CV-SOL-090-review-report-model`, `BE-CV-SOL-040-codeintel-quality-channels`; trước G3 dùng fake backend G4 (fixture mô hình đủ/thiếu phần, `warnings`, lỗi `QUALITY_GATE_DISABLED`) |
| Cờ, bridge, kiểu, phân loại lỗi | `FE-CV-TASK-085-01`, `FE-CV-SOL-050-types-and-runtime-bridge`, `FE-CV-SOL-050-store-and-query-hooks` |
| Chỗ đặt menu | `FE-CV-SOL-051-review-workspace-shell` (`ReviewSummaryBar`) |
| Nguồn dữ liệu mô hình | `BE-CV-SOL-085-*`, `BE-CV-SOL-036-*`, `BE-CV-SOL-037-*` (qua backend) |
| Telemetry xuất | `FE-CV-SOL-095-review-telemetry` (`review_report_exported`) |
| AG | Không có |

## 5. Tiêu chí chấp nhận

- [ ] Markdown ≤ `maxChars` ở mọi đầu vào, đúng một cặp dấu `orca-review`, không cắt giữa bảng/khối mã.
- [ ] Chèn lần hai thay đúng khối cũ, giữ phần người dùng viết; không chèn khi `generating`; GitLab → "merge request".
- [ ] `unknown` hiển thị "Chưa đủ dữ liệu để kết luận"; không "đạt" khi thiếu dữ liệu; không tên người/lý do miễn trừ mặc định.
- [ ] HTML: không `<script>`, không URL ngoài, có CSP, escape (`<img onerror>`, `<script>` trong `message` hiện dạng chữ); mở offline; không hex trong nguồn bộ dựng.
- [ ] Mermaid xấu (hàng rào, `%%{init`, > 6 KiB) bị loại, `alt[]` vẫn có.
- [ ] Menu và nút chèn không render và không gọi RPC khi `flags.quality=false`; hoạt động Electron và web.
- [ ] Copy lỗi hiện thông báo; không im lặng.

## 6. Kiểm thử (Vitest + Testing Library)

Golden Markdown/HTML từ mô hình cố định (mỗi provider); cắt ở nhiều ngưỡng; thoát ký tự; XSS (`"><svg onload=…>`); guard Mermaid; `mergeReviewReportIntoBody` (có/không khối, nhiều khối, CRLF); hook (flags, retry `inProgress`); component menu và nút chèn (cạnh tranh với `generating`; `// @vitest-environment happy-dom`); hồi quy composer. Thủ công có ghi lại: dán vào PR GitHub và MR GitLab thử, ghi phiên bản giao diện. Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/report` (chưa chạy).

## 7. Rủi ro và điểm chưa kiểm chứng

- Mô tả PR công khai: tên tệp/bảng/service vẫn lộ; chỉ chèn khi người dùng bấm.
- PNG từ SVG Mermaid (nút lưu ảnh của CR) bị loại khỏi phạm vi solution này (phụ thuộc phông/`foreignObject`, chưa thử); câu hỏi mở 3.
- Văn bản phụ thuộc phiên bản frontend; chỉ mô hình tái lập (`modelDigest`).
- `useTemplate` có thể tác động thứ tự chèn.
- Đo token màu bằng phần tử thử: tên token trong `main.css` có thể đổi.

## 8. Câu hỏi mở

1. Kiểu `generatedFor` (hợp đồng chưa định nghĩa).
2. Có dùng hộp thoại lưu native của desktop thay cho `a[download]` không (chưa đọc kênh preload).
3. Có làm "Lưu ảnh sơ đồ (PNG)" ở v7 không.
4. Ngôn ngữ báo cáo: theo UI (đề xuất của CR, Q3).

## 9. Tham chiếu

`/opt/repos/orca/guides/STYLEGUIDE.md`, `/opt/repos/orca/AGENTS.md`, các file frontend ở mục 1, `/opt/repos/orca/docs/crs/v7/quality-gate/CR-CV-090-exportable-review-report.md`.

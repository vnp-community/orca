# CR-CV-090 — Báo cáo review xuất được (Markdown/HTML, trung lập GitHub/GitLab)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-090 |
| **Tên** | RPC `ExportReviewReport` trả mô hình báo cáo có cấu trúc; bộ dựng Markdown (cho mô tả PR/MR) và HTML độc lập ở frontend; sơ đồ bằng Mermaid/SVG; nút "Sao chép" và "Chèn vào mô tả" trong form tạo PR hiện có; tái lập theo commit + index |
| **Loại** | Feature |
| **Priority** | 🟠 P1 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-06 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-085 (cổng, lý do, miễn trừ), CR-CV-036 (overlay, rủi ro, thứ tự đọc), CR-CV-059 (phát hiện), CR-CV-051 (thanh tóm tắt, nơi đặt nút), CR-CV-013 (che bí mật, quyền) |
| **Mở khoá** | CR-CV-093 (chèn tóm tắt AI có nhãn), CR-CV-095 |
| **Tác động** | `backend-go/services/code-intel-service` (usecase dựng mô hình), `backend-go/proto/orca/codeintel/v1/review_report.proto`, `api-gateway` (kênh `codeIntel.quality.report`), `frontend/src/renderer/src/components/review-map/report/` (mới), `frontend/src/renderer/src/components/right-sidebar/CreateHostedReviewComposerFields.tsx` (thêm một nút, tuỳ chọn), không đổi `SourceControl.tsx` ngoài vài dòng truyền prop. Nguồn: [research 11](../../../research/view-code/11-additions-for-quality-control.md) C7 |

---

## 1. Bối cảnh và vấn đề

Research 11 C7 muốn một bản báo cáo ngắn dán vào mô tả PR để người không mở Orca vẫn thấy: thay đổi gì, rủi ro, cổng chất lượng, phát hiện chính, hợp đồng/ERD đổi, thứ tự đọc, kèm sơ đồ. Đã đọc code ngày 2026-10-06:

1. **Form tạo PR/MR thật** là `CreateHostedReviewComposer` (`frontend/src/renderer/src/components/right-sidebar/CreateHostedReviewComposer.tsx`) cùng `CreateHostedReviewComposerFields.tsx`, gắn vào `SourceControl.tsx` (~dòng 5247); trạng thái `title/body/draft/base` do hook `useCreatePullRequestDialogFields` giữ (`body`, `setBody`; có "field revisions" `PullRequestFieldRevisions` để AI sinh nội dung không đè chữ người dùng vừa sửa). `PrCreateDialog` ở `components/code-review/pr-create-dialog.tsx` là **code chết** (README v7 mục 1) nên không phải điểm tích hợp. Form đã có sinh tiêu đề/mô tả bằng AI (`aiGenerationEnabled`, `onGenerate`), và nhận tuỳ chọn `useTemplate` (mẫu mô tả của repo, `SourceControl.tsx:3073`); chưa đọc cách `useTemplate` tương tác với `body` do người dùng đã điền (chưa kiểm chứng) — báo cáo phải **nối thêm**, không thay thế.
2. **Trung lập nhà cung cấp.** `HostedReviewProvider` không chỉ GitHub: `github`, `gitlab`, `azure-devops`, `gitea` (`source-control-create-review-blocked-action.ts`); chữ "PR"/"MR" lấy từ `localizedHostedReviewCopy(resolveSupportedHostedReviewCopyProvider(provider))`.
3. **Sơ đồ.** `mermaid` 11 đã là phụ thuộc (`frontend/package.json:113`); `components/editor/MermaidBlock.tsx` gọi `mermaid.render` với `securityLevel: 'strict'` và lọc kết quả bằng `DOMPurify` (`USE_PROFILES: { svg: true }`), cấu hình ở `mermaid-config.ts` (`htmlLabels=false` mặc định). Bộ bố cục đồ thị lớn là việc của CR-CV-088; chưa có bộ xuất ảnh.
4. **Giới hạn độ dài mô tả PR/MR.** Không có hằng số nào trong repo (đã tìm `body`/`65536`/`maxBody` ở `frontend/src/shared/hosted-review*.ts`: không thấy). Theo hiểu biết chung: GitHub giới hạn mô tả PR ~65 536 ký tự, GitLab MR ~1 MiB; **chưa kiểm chứng** với tài liệu hiện hành và chưa biết giá trị của Azure DevOps/Gitea. Vì vậy báo cáo dùng ngân sách bảo thủ mặc định (mục 2.5).
5. **Dữ liệu nguồn** (đề xuất ở CR khác): `ChangeOverlay` có `risk`, `readingOrder`, `touchedTables`, `touchedContracts` (CR-CV-036 2.3); `ErdModel.changes[]` (README v7 mục 8 điểm 11); `Finding` (CR-CV-037) và `QualityFinding` (CR-CV-082); `QualityGate` (CR-CV-085).

Vấn đề: chưa có cách xuất kết quả review ra ngoài Orca một cách an toàn (không lộ secret/mã thừa), tái lập được, và dùng được ở cả GitHub lẫn GitLab.

## 2. Giải pháp đề xuất

### 2.1 Ranh giới backend/frontend

| Việc | Nơi | Lý do |
|---|---|---|
| Tập hợp dữ liệu, cắt giới hạn, che bí mật, đặt khoá tái lập | **Backend** (`ExportReviewReport`) | Có quyền và dữ liệu; một nơi thi hành che bí mật (CR-CV-013); cùng nguồn với các view nên số liệu khớp màn hình |
| Dịch nhãn, dựng Markdown/HTML | **Frontend** (hàm thuần) | Chuỗi i18n nằm ở frontend (`translate()`, README v7 mục 6); một bộ dựng duy nhất, không hai catalog chuỗi |
| Sơ đồ | **Frontend** | Backend Go không có bộ bố cục; thêm thư viện bố cục trái O5/O12. Sơ đồ trong Markdown là **mã Mermaid**; ảnh chỉ trong HTML tải về |

Hệ quả: RPC trả `ReviewReportModel` (JSON), không trả văn bản đã dịch. `ExportReviewReport` vẫn đúng tên theo README; ghi vào "Điều chỉnh hợp đồng" rằng kết quả là mô hình, không phải văn bản. Một tool MCP/CLI sau này cần bộ dựng riêng phía Go (ngoài phạm vi).

### 2.2 `ReviewReportModel` (proto `review_report.proto`, mới; hàm dựng thuần ở `domain/review_report_model.go`)

```jsonc
{ "schemaVersion": 1,
  "subject": { "branch": "…", "baseRef": "…", "headCommit": "…", "baseCommit": "…",
               "indexCommit": "…", "indexStale": false, "provider": "github|gitlab|…", "turnKey": null },
  "reproducibility": { "profileRef": "default@repo/v3", "toolVersions": {"gitnexus":"…","codegraph":"…"},
                       "runIds": ["…"], "modelDigest": "<sha256>" },
  "summary":   { "files": 12, "added": 340, "removed": 58, "symbols": 41, "flows": 3, "components": ["…"] },
  "risk":      { "level": "low|medium|high|unknown", "reasons": [{"code":"…","params":{}}] },
  "gate":      { "verdict": "pass|warn|fail|unknown", "reasons": [ … như QualityGate.reasons … ],
                 "waivers": { "count": 2, "earliestExpiry": "…" } },
  "findings":  { "counts": {"error":1,"warning":4,"info":0, "byCategory": {…}},
                 "top": [ { "ruleId":"…", "severity":"…", "category":"…", "file":"…", "line":12, "message":"…≤160", "origin":"quality|structure" } ] },
  "contracts": { "protoRpc": [ {"name":"…","change":"added|modified|removed","breaking":false} ],
                 "wsChannels": [ … ], "tables": [ {"table":"…","service":"…","op":"added|altered|dropped","breaking":false} ] },
  "readingOrder": [ { "n":1, "file":"…", "reason":"contract|dependency-of|…", "symbols":["…"] } ],   // ≤ 15
  "diagrams": [ { "kind":"components|erd|flow", "mermaid":"<chuỗi, ≤ 6 KiB>", "alt":["A → B", "…"], "truncated": false } ],
  "limits":    { "truncated": {"findings":false,"readingOrder":false,"diagrams":false}, "totalCounts": {…} },
  "warnings":  [ "index_stale", "no_gate", "overlay_partial" ] }
```

Quy tắc: mọi `code` là khoá ổn định để frontend dịch; **không** có câu tiếng Việt/Anh trong mô hình. `riskLevel` giữ thành phần lý do (không một con số duy nhất; research 11 §7). Khi dữ liệu thiếu thì phần đó vắng mặt kèm `warnings[]`, **không** điền giá trị giả ("0 phát hiện" khác "chưa chạy kiểm tra"; cổng `unknown` được xuất nguyên văn). Sắp xếp xác định: `(severity desc, ruleId, file, line)`; sơ đồ sắp theo tên.

`modelDigest` = sha256 của JSON chuẩn hoá (khoá sắp, **không** có `generatedAt`), nên cùng `(headCommit, baseCommit, indexCommit, profileRef, runIds, giới hạn)` cho cùng digest — đây là cơ sở "tái lập". `generatedAt` và phiên bản bộ dựng ghi riêng ở phần chân văn bản.

### 2.3 RPC và kênh

`ExportReviewReport(ExportReviewReportRequest) returns (ExportReviewReportResponse)` trong `orca.codeintel.v1.QualityGateService` (README v7 3.10). Request: `repo_binding_id`, `base_ref?` (mặc định merge-base, O7), `profile_name?`, `turn_key?`, `sections[]` (tập con của `summary,risk,gate,findings,contracts,readingOrder,diagrams`; mặc định tất cả), `max_findings` (≤ 50, mặc định 10), `max_reading_steps` (≤ 50, mặc định 15), `include_people` (mặc định `false`), `include_waiver_reasons` (mặc định `false`). Response: `model`, `generated_for` (commit, index, `stale`), `warnings[]`. Kênh WS `codeIntel.quality.report`. Quyền: `quality_read` (CR-CV-085 2.9) **và** `read`; không cần `read_source` vì không có mã nguồn. Cờ `quality_gate_enabled` tắt → `CODEINTEL_QUALITY_GATE_DISABLED`; phần cổng vắng, các phần khác vẫn xuất khi chỉ `code_intel_enabled` bật (xem Q4).

Nguồn dữ liệu đọc từ cùng use case/cache của các view (`GetChangeOverlay`, `ListFindings`, `ListQualityFindings`, `GetQualityGate`, `GetErd`, `GetContractDiff`); **không gọi agent thêm ngoài đường collector sẵn có** (singleflight, hạn mức của CR-CV-013 2.6). Thiếu snapshot thì tính qua collector như view; thất bại từng phần → `warnings[]`, không thất bại cả báo cáo. Audit: `codeintel.report.export` (`target: review:<binding>`, `outcome: allowed`) vì dữ liệu rời hệ thống theo ý người dùng (thêm vào bảng 2.4 của CR-CV-013; ghi vào "Điều chỉnh hợp đồng"); log `slog` có `audit=true`, kèm `modelDigest`, không kèm nội dung.

### 2.4 Không lộ secret và mã thừa

- Backend: mọi chuỗi tự do (`message`, tên symbol, tên bảng, lý do) qua `secret_redactor.go` (CR-CV-013 2.5), cắt `message` ≤ 160 ký tự, bỏ xuống dòng. **Không** có mã nguồn: chỉ đường dẫn tương đối gốc repo, tên symbol, số dòng. Đường dẫn khớp danh sách chặn nội dung (`.env`, `*.pem`, …) giữ tên nhưng đánh `contentWithheld`, không bao giờ kèm `message` của phát hiện thuộc tệp đó (message có thể trích mã).
- Không có đường dẫn tuyệt đối, `workspace_root`, hostname dev server, URL cục bộ, tên người dùng (trừ khi `include_people`; khi bật chỉ tên hiển thị, không email). Lý do miễn trừ chỉ xuất khi `include_waiver_reasons` (mô tả PR có thể công khai hơn Orca).
- Chuỗi hiển thị trong HTML được **escape** (nội dung phát hiện có thể chứa `<script>` từ mã); Markdown thoát ký tự đặc biệt (`|`, backtick, `<`, `[`) trong ô bảng.
- Không đưa nội dung tệp bị `.gitignore` vì dữ liệu nguồn không chứa chúng; test khẳng định (mục 5).

### 2.5 Dựng Markdown (cho mô tả PR/MR)

`components/review-map/report/review-report-markdown.ts` (hàm thuần: `(model, { t, provider, maxChars }) → { markdown, truncated }`):

- Cấu trúc: tiêu đề `## Review (Orca)`, dòng nguồn ("Đối chiếu commit `abc1234` so với `main`; chỉ mục tại `def5678`"), phần tóm tắt, rủi ro (danh sách lý do), cổng chất lượng (bảng `Kiểm tra | Quan sát | Ngưỡng | Kết quả`), phát hiện chính (bảng ≤ 10), hợp đồng/ERD, thứ tự đọc (danh sách đánh số, `<details>` cho phần dài), sơ đồ (khối ```` ```mermaid ````), chân văn bản (phiên bản, `modelDigest` rút gọn 12 ký tự, giờ tạo UTC).
- **Cặp dấu quản lý** `<!-- orca-review:start -->` … `<!-- orca-review:end -->` bao toàn khối để chèn lại thay đúng khối cũ (idempotent), không đụng phần người dùng viết.
- Chữ cổng không overclaim (STYLEGUIDE; ví dụ `pass` → "Các kiểm tra bắt buộc đã chạy đều đạt", `unknown` → "Chưa đủ dữ liệu để kết luận"); không dùng "an toàn để merge", "đã duyệt".
- **Ngân sách độ dài** `maxChars`: mặc định **20 000** cho mọi provider (thấp hơn mọi giới hạn đã biết ở mục 1.4 để chừa chỗ cho mô tả của người dùng và mẫu repo); cấu hình theo provider khi đã kiểm chứng. Cắt theo thứ tự ưu tiên giữ: tiêu đề + cổng + rủi ro → phát hiện (giảm dần) → hợp đồng/ERD → thứ tự đọc → sơ đồ (bỏ đầu tiên) → chân văn bản luôn giữ. Khi cắt thêm dòng "Đã rút gọn; xem đầy đủ trong Orca" và `truncated=true`. Không bao giờ cắt giữa bảng/khối mã.
- **Mermaid trong mô tả**: GitHub và GitLab render khối `mermaid` trong Markdown (theo hiểu biết chung, chưa kiểm chứng trên phiên bản hiện tại); Azure DevOps/Gitea chưa rõ. Nên mỗi sơ đồ kèm danh sách văn bản thay thế (`alt[]`, tối đa 12 dòng) trong `<details>` để nơi không render vẫn đọc được; có tuỳ chọn tắt sơ đồ trong Markdown. Không nhúng ảnh `data:`/SVG thô vào Markdown (bị lọc ở hầu hết nền tảng và làm phình mô tả).

### 2.6 Dựng HTML độc lập (tải về/mở trong trình duyệt)

`review-report-html.ts` (+ `review-report-diagram-svg.ts` gọi `mermaid.render` theo cấu hình `getMermaidConfig(isDark, false)` và `DOMPurify`): một tệp `.html` tự đủ: không script, không tài nguyên ngoài, có `<meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src data:; style-src 'unsafe-inline'">`, ngôn ngữ `lang` theo locale, tiêu đề cấp bậc đúng, bảng có `<caption>`, mỗi SVG có `<title>`/`<desc>` và danh sách văn bản thay thế, hỗ trợ `prefers-color-scheme`. Màu **không** hard-code trong nguồn: lúc xuất, đọc giá trị token đang áp dụng (`getComputedStyle(document.documentElement)` từ `main.css`: `--background`, `--foreground`, `--border`, `--destructive`…) và điền vào khối `:root` của tệp xuất (nhất quán STYLEGUIDE; không tạo màu mới). Nút "Lưu ảnh sơ đồ (PNG)" ở UI dùng canvas từ SVG (cần `htmlLabels=false` để canvas không bị "taint"; kiểm chứng ở Electron và web) — chỉ ở HTML/ứng dụng, không nhúng vào mô tả PR.

Lưu tệp: Electron qua hộp thoại lưu của desktop main; web qua `Blob` + `a[download]`. Tên tệp `orca-review-<repo>-<head7>.html`, không chứa đường dẫn tuyệt đối. (Tên kênh/preload cho lưu tệp: cần đọc `desktop/src/preload/index.ts` khi triển khai; chưa đọc ở CR này.)

### 2.7 Tích hợp giao diện

| Nơi | Thay đổi | Ghi chú |
|---|---|---|
| Review: thanh tóm tắt (CR-CV-051 `ReviewSummaryBar`) | Menu "Xuất báo cáo ▾": **Sao chép Markdown**, **Sao chép cho mô tả PR/MR** (ngân sách 2.5), **Lưu HTML** | `components/review-map/report/ReviewReportMenu.tsx`; dùng `ui/dropdown-menu`; hiện khi `useCodeIntelSupport().state==='enabled'` |
| Form tạo PR/MR | Nút "Chèn báo cáo review" cạnh ô mô tả (`CreateHostedReviewComposerFields.tsx`); prop mới tuỳ chọn `onInsertReviewReport?: () => Promise<string \| null>` | `SourceControl.tsx` chỉ truyền hàm; hàm gọi `ExportReviewReport` → Markdown → cập nhật `body` qua `setBody` **chỉ khi không đang sinh AI** (tránh đua với `PullRequestFieldRevisions`); nếu đã có khối `orca-review` thì thay đúng khối, nếu chưa thì nối cuối sau một dòng trống |
| Sao chép | `navigator.clipboard.writeText` (đã dùng ở `settings/mcp/McpCopyButton.tsx:35`) với fallback `execCommand('copy')`/chọn văn bản khi `clipboard` không có (ngữ cảnh web không an toàn) | Toast xác nhận ngắn (STYLEGUIDE: toast cho xác nhận thoáng qua) |

Nút chỉ hiện khi tính năng bật; khi tắt không render, không gọi RPC (như CR-CV-061). Mọi chuỗi qua `translate()`; hai render target (Electron, web); phím tắt không thêm ở CR này.

### 2.8 Cấu trúc file (mới)

Backend: `domain/review_report_model.go`, `usecase/export_review_report.go`, `adapter/grpc/review_report_server.go`. Frontend `components/review-map/report/`: `review-report-model.ts` (kiểu sinh từ proto), `review-report-markdown.ts`, `review-report-html.ts`, `review-report-diagram-mermaid.ts` (dựng mã Mermaid từ mô hình: `flowchart LR` cho component ≤ 30 nút, `erDiagram` ≤ 15 bảng; thoát nhãn), `review-report-diagram-svg.ts`, `ReviewReportMenu.tsx`, `use-review-report.ts`. Không `helpers/utils`; không `max-lines` disable.

## 3. Quyết định thiết kế

1. **RPC trả mô hình, frontend dựng chữ**: tránh catalog chuỗi thứ hai ở Go; chấp nhận rằng văn bản phụ thuộc phiên bản frontend (ghi trong chân văn bản).
2. **Sơ đồ ở Markdown là Mermaid, không ảnh**: không cần lưu trữ ảnh ở provider, tương thích GitHub/GitLab, thay thế bằng `alt[]`.
3. **Khối quản lý có dấu** để chèn lại an toàn thay vì chèn trùng.
4. **Ngân sách ký tự bảo thủ, một giá trị cho mọi provider** cho tới khi kiểm chứng từng nhà cung cấp.
5. **Mặc định không có tên người và lý do miễn trừ**: mô tả PR có thể công khai hơn Orca.
6. **Thiếu dữ liệu hiện nguyên trạng** (cổng `unknown`, phần vắng + cảnh báo), không điền "đạt".
7. **Không dùng AI để dựng báo cáo** (đây là dữ kiện); tóm tắt AI do CR-CV-093 chèn riêng, có nhãn.

## 4. Tiêu chí chấp nhận

- [ ] `ExportReviewReport` với cùng `(headCommit, baseCommit, indexCommit, profileRef, runIds, tham số)` cho cùng `modelDigest` (hai lần gọi, hai dialect, hai tiến trình).
- [ ] Mô hình không chứa: đường dẫn tuyệt đối, `workspace_root`, email, tên người (mặc định), lý do miễn trừ (mặc định), mã nguồn, chuỗi khớp mẫu secret của CR-CV-013 (test chèn token giả vào `message` → thành `[REDACTED]`).
- [ ] Phát hiện thuộc tệp trong danh sách chặn nội dung không kèm `message`.
- [ ] Thiếu run bắt buộc → báo cáo xuất cổng `unknown` và `warnings[]` chứa `no_gate`/lý do; không có chữ "đạt".
- [ ] Markdown: ≤ `maxChars` (20 000 mặc định) ở mọi đầu vào; không cắt giữa bảng/khối mã; chứa đúng một cặp `orca-review:start/end`; render đúng trên bản xem trước GitHub và GitLab (kiểm tay, ghi lại phiên bản).
- [ ] Chèn lần hai vào cùng mô tả **thay** khối cũ, giữ nguyên phần người dùng viết; không chèn khi đang sinh AI; chữ "PR"/"MR" theo provider (GitLab → "merge request").
- [ ] HTML: không `<script>`, không URL ngoài, có CSP; nội dung `<img onerror>` và `<script>` trong `message` bị escape (test XSS); mở được offline; có `lang`, `<caption>`, văn bản thay thế cho từng sơ đồ; không có giá trị màu hex trong nguồn bộ dựng.
- [ ] Sơ đồ Mermaid đã thoát nhãn (tên chứa `"`, `]`, `;`, xuống dòng không làm vỡ cú pháp); vượt 6 KiB hoặc số nút tối đa → `truncated=true` và bỏ sơ đồ thay vì cắt giữa.
- [ ] Nút xuất và nút chèn không render khi tính năng tắt; không gọi RPC khi tắt; chạy ở Electron và web.
- [ ] Audit `codeintel.report.export` được ghi; người không có quyền đọc nhận `CODEINTEL_NOT_AUTHORIZED`.

## 5. Kiểm thử

- **Unit (Go)**: dựng mô hình từ fixture (overlay, gate, findings) → golden JSON; thiếu từng nguồn; redaction; sắp xếp xác định; digest không phụ thuộc thứ tự map.
- **Unit (TS)**: golden Markdown/HTML từ mô hình cố định (mỗi `provider`), cắt theo `maxChars` ở nhiều ngưỡng, thoát ký tự, `alt[]`, cặp dấu quản lý; XSS (chuỗi `"><svg onload=…>`); bộ dựng Mermaid với tên "xấu".
- **Component (RTL)**: menu xuất, nút chèn (cạnh tranh với `generating`), fallback clipboard.
- **Thủ công có ghi lại**: dán Markdown vào PR thử trên GitHub và MR thử trên GitLab, ghi phiên bản giao diện và việc Mermaid render hay không; kiểm tra giới hạn mô tả bằng đọc tài liệu hiện hành rồi cập nhật hằng số.
- **Hiệu năng**: dựng báo cáo cho overlay lớn (2 000 tệp) dưới ngân sách (CR-CV-071 chốt); đo trước khi hứa.

## 6. Rủi ro và điểm chưa kiểm chứng

- Giới hạn mô tả PR/MR và việc render Mermaid chưa kiểm chứng (1.4, 2.5); ngân sách 20 000 là phỏng đoán.
- Cách `useTemplate` điền mô tả và nút chèn tương tác thế nào chưa đọc; có thể cần chỉnh thứ tự chèn.
- Mô tả PR là bề mặt công khai: một thông tin nội bộ (tên service, tên bảng, lý do rủi ro) có thể lộ cho người ngoài tổ chức nếu repo công khai; mặc định ẩn người/lý do miễn trừ nhưng **không** ẩn tên tệp/bảng. Cần người dùng chủ động bấm "Chèn" (không tự chèn).
- PNG từ SVG Mermaid phụ thuộc phông và `foreignObject`; chưa thử ở Electron lẫn web.
- Văn bản phụ thuộc phiên bản frontend nên không "tái lập" bit-for-bit giữa hai bản Orca; chỉ tái lập mô hình.
- Phụ thuộc CR đề xuất (036, 037, 085, 082): hình dạng thật có thể khác; mô hình ở 2.2 phải cập nhật khi họ chốt.
- Tên hàm chức năng lưu tệp ở desktop preload chưa đọc.

## 7. Câu hỏi mở

- **Q1.** Có cho cập nhật mô tả PR **đã tạo** (qua `ChecksPanel`/API provider) hay chỉ khi tạo? Hiện chỉ lúc tạo (và sao chép tay).
- **Q2.** Có cần tool MCP/CLI xuất báo cáo cho CI/bot (cần bộ dựng Go)? Hiện không.
- **Q3.** Ngôn ngữ báo cáo: theo locale UI của người xuất, hay cố định cho cả nhóm (tiếng Anh)? Đề xuất: theo UI, có tuỳ chọn.
- **Q4.** Khi chỉ `code_intel_enabled` bật (không `quality_gate_enabled`), có xuất báo cáo không có phần cổng? Đề xuất: có.
- **Q5.** Có nhúng ảnh PNG sơ đồ lên provider (upload attachment) không? Hiện không (cần API riêng từng provider, ngoài phạm vi).

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (3.10 `ExportReviewReport`; mục 8)
- `/opt/repos/orca/docs/research/view-code/11-additions-for-quality-control.md` (C7, §7), `/opt/repos/orca/docs/research/view-code/08-views-and-review-models.md`
- `/opt/repos/orca/docs/crs/v7/code-intel-sources/CR-CV-036-change-overlay.md` (2.3), `CR-CV-037-structure-analysis.md`, `CR-CV-038-contract-diff-and-static-security.md`
- `/opt/repos/orca/docs/crs/v7/review-frontend/CR-CV-051-review-workspace-shell.md` (2.5), `CR-CV-056-dataflow-lens.md`, `CR-CV-061-review-entry-points.md`
- `/opt/repos/orca/docs/crs/v7/code-intel-service-foundation/CR-CV-013-authorization-audit-and-quotas.md` (2.4, 2.5)
- `/opt/repos/orca/frontend/src/renderer/src/components/right-sidebar/CreateHostedReviewComposer.tsx`, `CreateHostedReviewComposerFields.tsx`, `useCreatePullRequestDialogFields.ts`, `SourceControl.tsx` (`:3073`, `:5247`), `source-control-create-review-blocked-action.ts`
- `/opt/repos/orca/frontend/src/renderer/src/components/editor/MermaidBlock.tsx`, `mermaid-config.ts`; `/opt/repos/orca/frontend/src/renderer/src/components/settings/mcp/McpCopyButton.tsx`
- `/opt/repos/orca/frontend/src/renderer/src/components/code-review/pr-create-dialog.tsx` (code chết)
- CR cùng nhóm (chỉ ID): CR-CV-082, 085, 087, 088, 093, 095
- `/opt/repos/orca/AGENTS.md`, `/opt/repos/orca/guides/STYLEGUIDE.md`

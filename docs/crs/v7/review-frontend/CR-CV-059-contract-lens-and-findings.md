# CR-CV-059 — Lens Hợp đồng và danh sách Phát hiện (Bỏ qua / Đã xử lý)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-059 |
| **Tên** | Lens Hợp đồng (bảng trước/sau cho proto, route, kênh WS, schema; đánh dấu phá vỡ tương thích) và danh sách Phát hiện (vi phạm lớp, vòng phụ thuộc, hotspot, thiếu `tenant_id`, mã chết) với Bỏ qua/Đã xử lý (`dismissFinding`), lọc, liên kết tới diff |
| **Loại** | Feature |
| **Priority** | 🟠 P1 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-050 (kiểu, `codeIntelClient.call`/`useCodeIntelQuery` cho kênh `codeIntel.contractDiff`, `codeIntel.findings`, `codeIntel.dismissFinding`, slice, i18n, `useCodeIntelSupport`), CR-CV-051 (khung, thanh tóm tắt, vùng đáy), CR-CV-053 (panel chi tiết, mở diff đúng dòng, liên kết hai chiều), CR-CV-057 (liên kết sang ERD); backend CR-CV-032 (proto, kênh `wscompat`), CR-CV-037 (vi phạm lớp, vòng, hotspot, mã chết), CR-CV-038 (contract diff, `tenant_id`), CR-CV-011 (`finding_dismissals`), CR-CV-040 (kênh `codeIntel.contractDiff`, `codeIntel.findings`, `codeIntel.dismissFinding`) |
| **Mở khoá** | CR-CV-060 (ghi chú gắn vào phát hiện), CR-CV-061 (số phát hiện ở tab right sidebar), CR-CV-062 (danh sách phát hiện trên mobile) |
| **Tác động** | `frontend/src/renderer/src/components/review-map/contract/` và `components/review-map/findings/` (mới), `components/review-map/ReviewLensTabs.tsx` (CR-CV-051 sở hữu), `store/slices/code-intel.ts` (khoá `contractDiff`, `findings`), `i18n/locales/*.json` |

---

## 1. Bối cảnh và vấn đề

Hai loại vấn đề thường lọt qua review diff chữ khi agent code: (1) **đổi hợp đồng** (xoá/đổi số trường proto, bỏ một route hoặc kênh `wscompat`, đổi kiểu cột) làm vỡ client ở nơi khác trong repo; (2) **vi phạm cấu trúc** (`usecase` import `adapter`, vòng phụ thuộc, truy vấn thiếu `tenant_id`, export không ai dùng, hotspot). Nghiên cứu [08 §7](../../../research/view-code/08-views-and-review-models.md) gọi chúng là R2/R3/R4/R7/R8; [10 §5, §6.7](../../../research/view-code/10-frontend-review-ux.md) đề xuất lens "Hợp đồng" (bảng hai cột trước/sau, đánh dấu phá vỡ tương thích, mỗi dòng liên kết tới diff) và danh sách "Phát hiện" ở đáy màn, mỗi dòng có "Bỏ qua/Đã xử lý".

Hiện trạng frontend (đã đọc code):

- Không có UI hợp đồng hay phát hiện nào; không có `components/review-map/`.
- Pattern gần nhất cho "danh sách có lọc + hành động từng dòng" là `components/right-sidebar/ChecksPanel.tsx` và `PullRequestPage.tsx` (hiển thị check, comment; `viewerCan` có trong `PullRequestPage.tsx` và `task-page-github-review-cells.tsx`). Dữ liệu của chúng là của nhà cung cấp git nên không tái dùng, chỉ tham khảo cách trình bày; chưa đọc kỹ từng dòng.
- `lib/screen-submit-shortcut.ts` (`isScreenSubmitShortcut`) và `components/ShortcutKeyCombo.tsx` đã có cho phím gửi biểu mẫu theo nền tảng (đã xác nhận file tồn tại).
- `components/ui/` có `table`, `badge`, `select`, `popover`, `textarea`, `toggle-group`, `tooltip`, `skeleton`, `scroll-area`, `collapsible` (đã đọc thư mục); `@tanstack/react-virtual` có trong `package.json` (README v7 mục 1).
- Backend (README v7): bảng `finding_dismissals {id, tenant_id, repo_binding_id, finding_key, reason, dismissed_by, at}`, RPC `ListFindings`, `DismissFinding`, `GetContractDiff`, kênh `codeIntel.findings`, `codeIntel.dismissFinding`, `codeIntel.contractDiff`. **Chưa có** message định nghĩa `Finding`/`ContractDiff` (README 3.6: message do CR sở hữu từng RPC định nghĩa); CR này nêu hình dạng UI cần để CR-CV-037/038 đối chiếu.

## 2. Giải pháp đề xuất

### 2.1 Hình dạng dữ liệu UI cần (đề xuất, chưa có trong hợp đồng)

```ts
type Finding = {
  key: string                 // ổn định giữa các lần chạy; KHÔNG chứa số dòng (xem 6)
  kind: 'layer_violation' | 'dependency_cycle' | 'hotspot' | 'missing_tenant_id' | 'dead_code'
  severity: 'high' | 'medium' | 'low' | 'info'
  title: string
  summary: string
  locations: { symbol?: SymbolRef; filePath: string; startLine?: number; endLine?: number }[]
  evidence?: string           // đoạn ngắn (câu SQL, cặp import), đã được backend che secret
  origin: 'introduced_in_scope' | 'preexisting' | 'unknown'   // có do thay đổi đang review không
  status: 'open' | 'dismissed'
  dismissal?: { disposition: 'ignored' | 'resolved'; reason: string; by: string; at: string }
}
type ContractChange = {
  id: string
  contractKind: 'proto_rpc' | 'proto_message' | 'proto_field' | 'route' | 'ws_channel' | 'db_schema'
  service: string
  name: string                // ví dụ InfraFleetService.RelayByDevServer, codeIntel.erd, infra.dev_servers.status
  change: 'added' | 'removed' | 'modified'
  compatibility: 'breaking' | 'compatible' | 'unknown'
  breakingReasons: string[]   // mã + mô tả: field_removed, field_number_reused, type_changed, rpc_removed, route_removed…
  before?: string             // chữ ký trước (proto/route/schema) đã chuẩn hoá để so sánh
  after?: string
  location: { filePath: string; startLine?: number }
  consumers?: { symbol: SymbolRef; side: 'client' | 'server' }[]
}
```

UI **không tự phân loại** `breaking`/`compatible`; chỉ hiển thị kết quả của backend (CR-CV-038). `unknown` là giá trị hợp lệ và hiển thị như "chưa xác định", không phải "tương thích", để không tạo cảm giác an toàn giả.

Phần bọc ngoài `CodeIntelResult<{findings: Finding[]}>`/`CodeIntelResult<{changes: ContractChange[]}>` có `stale`, `truncated`, `totalCount`. Hook (mới): `findings/use-code-intel-findings.ts` (`useCodeIntelFindings({worktreeId, filters})`) và `contract/use-code-intel-contract-diff.ts`; khoá cache `(worktreeId, headCommit, 'findings'|'contractDiff', paramsHash)`. Hai hook bỏ phản hồi cũ khi tham số đổi.

### 2.2 Lens Hợp đồng

Bố cục (một lens trong `ReviewLensTabs`, panel chi tiết ở cột phải của khung Review):

```
┌ Bộ lọc: [Loại ▾ proto|route|kênh|schema] [Service ▾] [☐ Chỉ phá vỡ] [☐ Chỉ do agent đổi] [🔍 tìm] ─────────┐
│ Tóm tắt:  14 thay đổi · 2 PHÁ VỠ · 1 chưa xác định · 11 tương thích                                        │
├──────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ ▾ infra-fleet-service · proto (5)                                                                           │
│  ┌────────────────────────────┬──────────────────────────────┬──────────────────────────────┬───────────┐ │
│  │ Hợp đồng                    │ Trước                         │ Sau                           │ Tương thích│ │
│  │ RelayByDevServer.timeout_ms │ int32 timeout_ms = 3;         │ (đã xoá)                      │ ⛔ Phá vỡ  │ │
│  │ StreamCodeIntelEvents       │ —                             │ rpc StreamCodeIntelEvents(...)│ ✓ Thêm mới │ │
│  └────────────────────────────┴──────────────────────────────┴──────────────────────────────┴───────────┘ │
│ ▸ api-gateway · kênh WS (4)      ▸ infra.dev_servers · schema (2) → [Mở trong ERD]                          │
└──────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

Component (thư mục `components/review-map/contract/`):

- `ContractDiffLens.tsx`: bộ lọc, tóm tắt (mỗi chip là bộ lọc, theo nguyên tắc 10 §6.1), nhóm theo `service` rồi `contractKind` (`ui/collapsible`).
- `ContractChangeTable.tsx` (`ui/table`): bốn cột Hợp đồng / Trước / Sau / Tương thích; ảo hoá bằng `@tanstack/react-virtual` khi >100 dòng.
- `ContractSignatureCell.tsx`: hiển thị chữ ký bằng font mono; tô phần khác nhau giữa trước và sau theo token (`contract-signature-diff.ts`, hàm thuần, so sánh theo token trên chữ ký đã chuẩn hoá; không dùng Monaco).
- `ContractCompatibilityBadge.tsx`: `breaking` dùng `destructive` kèm icon `ShieldAlert`, `compatible` dùng muted kèm `Check`, `unknown` dùng muted kèm `HelpCircle` và nhãn chữ "Chưa xác định". Màu không phải kênh duy nhất.
- `ContractChangeDetail.tsx` (cột phải): `breakingReasons[]` dịch qua `translate()` theo mã (mã lạ hiển thị nguyên văn kèm `unknown_reason`), danh sách `consumers` (client/server; bấm mở symbol qua CR-CV-053), liên kết "Xem diff" tới `location` (mở diff đúng dòng qua CR-CV-053; **lưu ý** luồng `openDiff` hiện có trong `store/slices/editor.ts` chưa có tham số dòng nên phụ thuộc CR-CV-053), và với `db_schema`: "Mở trong ERD" gọi `setReviewLens(worktreeId, 'erd')` + chọn bảng (CR-CV-057) thay vì lặp bảng cột.
- Nếu `consumers` rỗng với thay đổi `breaking` thì hiện dòng "Chưa tìm thấy nơi dùng (có thể ngoài repo hoặc chưa được lập chỉ mục)", **không** viết "không ai dùng".

### 2.3 Danh sách Phát hiện (`FindingsPanel`)

Đặt ở đáy khung Review (vùng dock do CR-CV-051 cung cấp, `react-resizable-panels`), thu gọn được; thanh tóm tắt có chip "N phát hiện" (mở/lọc). Thư mục `components/review-map/findings/`:

```
FindingsPanel
├─ FindingsToolbar          (chip kind + số lượng, severity, [☐ Chỉ do thay đổi này], [☐ Hiện đã bỏ qua/đã xử lý], tìm kiếm)
├─ FindingsList             (nhóm theo kind; ảo hoá khi >50 dòng)
│    └─ FindingRow          (icon kind, title, severity, vị trí file:dòng, nhãn origin, trạng thái)
│         actions: [Xem trong đồ thị] [Xem diff] [Ghi chú] [Bỏ qua ▾] [Đã xử lý]
├─ FindingDismissPopover    (lý do + ghi chú, Xác nhận)
└─ FindingsEmptyState / FindingsErrorState / FindingsSkeleton
```

Hành vi:

- **Lọc/sắp xếp**: mặc định sắp theo severity rồi `origin` (do thay đổi này lên trước), nhóm theo kind. Các bộ lọc là hàm thuần `finding-filter.ts` (kind, severity, origin, `status`, tìm trong title/file/symbol). Trạng thái bộ lọc thuộc slice (không `localStorage`).
- **Liên kết tới diff**: "Xem diff" mở diff của `locations[0].filePath` tại `startLine` (CR-CV-053); nếu file không nằm trong thay đổi (`origin='preexisting'`) thì nút đổi thành "Mở tệp". "Xem trong đồ thị" chọn nút tương ứng ở lens phù hợp: `layer_violation`/`dependency_cycle` → Cấu trúc/Ảnh hưởng (nếu symbol có trong đồ thị đang hiển thị, ngược lại hiện "Không có trong đồ thị hiện tại"), `hotspot`/`dead_code` → Cấu trúc, `missing_tenant_id` → không có nút (không có nút đồ thị cho truy vấn).
- **Ghi chú**: nút "Ghi chú" gọi luồng của CR-CV-060 với mặc định nội dung gợi ý từ `title`; không tự gửi.
- **Bỏ qua / Đã xử lý**: hai hành động khác nghĩa. "Bỏ qua" (`disposition='ignored'`) = người review chấp nhận hoặc cho là dương tính giả, bắt buộc chọn lý do (`not_applicable | accepted_risk | false_positive | later`; danh sách là đề xuất) và có thể ghi chú. "Đã xử lý" (`disposition='resolved'`) = đã được sửa, một cú bấm, ghi chú tuỳ chọn. Cả hai gọi `codeIntelClient.call(worktreeId, 'codeIntel.dismissFinding', { findingKey, disposition, reason, note })` (CR-CV-050 2.3; lỗi trả về `CodeIntelRpcError` với `kind` như `forbidden`, `offline`, `conflict`). Đang gọi: khoá nút ngay, hiện `Loader2` sau ~200 ms (độ trễ SSH, STYLEGUIDE UX rule 1); cập nhật lạc quan và hoàn nguyên khi lỗi kèm thông báo inline trên dòng. `Mod+Enter` (`isScreenSubmitShortcut`) xác nhận trong popover, chip phím qua `ShortcutKeyCombo`. Hoàn tác: dòng đã bỏ qua hiện nút "Mở lại" (`dismissed:false`); RPC hiện chỉ có `DismissFinding` nên cần tham số khôi phục (xem "Điều chỉnh hợp đồng").
- **Quyền và lỗi**: lỗi `forbidden` hiện "Bạn không có quyền bỏ qua phát hiện" inline; lỗi mạng giữ dòng ở trạng thái cũ và có "Thử lại".
- **Hiệu lực của việc bỏ qua**: lưu theo `repo_binding_id + finding_key` (README 3.5), không theo commit. Dòng đã bỏ qua mặc định ẩn, `FindingsToolbar` có đếm "đã bỏ qua N". Nếu sau này phát hiện **xuất hiện lại ở vị trí khác** (khoá khác), nó là phát hiện mới.
- **Chỉ báo trên đồ thị** (10 §6.7): slice xuất selector `selectOpenFindingsBySymbolKey(worktreeId)` để các lens (CR-CV-053/054/055) tô icon cảnh báo nhỏ trên nút có phát hiện mở; CR này chỉ cung cấp selector, không sửa lens khác.
- **Che dữ liệu nhạy cảm**: `evidence` (đoạn SQL, import) đi qua `maskSensitiveText` (hàm của CR-CV-058, `storage/storage-secret-masking.ts`; nếu hai lens cùng dùng thì chuyển lên `components/review-map/sensitive-text-masking.ts`) trước khi hiển thị; backend vẫn là lớp chính.

### 2.4 Phím tắt (khi tiêu điểm trong vùng danh sách; không ở ô nhập)

`j`/`k` chuyển dòng, `Enter` xem diff, `d` mở popover Bỏ qua, `r` "Đã xử lý", `Esc` đóng. Không dùng phím sửa đổi nên không cần nhánh Mac/Windows; chip hiện trong tooltip khi đã cài. Nếu CR-CV-052 có registry phím tắt chung thì đăng ký ở đó để không xung đột với `j/k/Space` của "Thứ tự đọc".

### 2.5 Trạng thái và lỗi

| Tình huống | Hiển thị |
|---|---|
| Đang tải | Ngưỡng 100 ms/1 s/3 s như STYLEGUIDE; ≥3 s nêu giai đoạn nếu có `stage`; qua SSH trì hoãn hiển thị ~200 ms, khoá điều khiển ngay |
| Không có phát hiện | "Không phát hiện vấn đề nào trong phạm vi index" (không viết "an toàn"); kèm ngày/commit index |
| Không có thay đổi hợp đồng | "Không có thay đổi hợp đồng nào được phát hiện trong phạm vi này"; ghi chú loại nguồn đã quét (proto/route/kênh/schema) để người dùng biết phạm vi |
| Chưa có index / công cụ thiếu | Phát hiện cấu trúc dựa vào đồ thị nên rỗng kèm trạng thái chuẩn của CR-CV-051 (Lập chỉ mục / hướng dẫn cài); hợp đồng proto/SQL có thể vẫn có (đọc file) |
| `truncated` | "Đang hiển thị X / Y" kèm gợi ý lọc; không cắt im lặng |
| `stale` | Banner chung của khung Review |
| Lỗi `CODEINTEL_*` | Persistent inline có "Thử lại", không toast; toast chỉ cho xác nhận thoáng qua ("Đã bỏ qua", kèm "Hoàn tác") |
| Dev server offline | Dùng trạng thái `offline` của khung Review (CR-CV-051): giữ dữ liệu cache, đánh dấu cũ; không dùng `ConnectionStatusBanner` |

### 2.6 i18n, token, hai render target

Chuỗi qua `translate()`, khoá tiền tố `auto.components.review.map.contract.` và `auto.components.review.map.findings.` (ví dụ `ContractCompatibilityBadge.breaking`, `ContractChangeDetail.noConsumers`, `FindingsToolbar.onlyIntroduced`, `FindingDismissPopover.reason.falsePositive`, `FindingRow.originPreexisting`) đủ 5 locale `en/es/ja/ko/zh`; mã `breakingReasons` và `kind` có bảng dịch riêng, mã lạ rơi về nguyên văn. Màu chỉ qua biến CSS (`destructive`, muted, accent); cần token cho severity trung gian thì CR-CV-050 thêm vào `main.css` (`:root` và `.dark`) và bind `@theme inline` (hiện `main.css` chỉ có `var(--warning, #f59e0b)` dùng fallback hex ở vài chỗ và không có `--warning` được định nghĩa trong các khối đã đọc; không nhân bản mẫu đó). Chạy cả Electron và web vì chỉ đi qua `codeIntelClient`/`useCodeIntelQuery` (Electron local phụ thuộc preload của CR-CV-050). Lens Hợp đồng đăng ký `id: 'contract'` trong `REVIEW_LENS_DEFINITIONS` (CR-CV-051 2.6) và nhận `ReviewLensProps`; `FindingsPanel` gắn vào vùng đáy của `ReviewWorkspace`, không phải lens.

## 3. Quyết định thiết kế

- **UI không phân loại tương thích**: đây là quy tắc nghiệp vụ cần kiểm thử ở backend một lần; UI chỉ hiển thị và có giá trị `unknown`.
- **Hai hành động Bỏ qua và Đã xử lý tách nhau**: "bỏ qua" là quyết định của người, "đã xử lý" là sự kiện; gộp thì mất thông tin để thống kê và để agent biết cái nào cần sửa tiếp.
- **Lưu hiệu lực theo `finding_key` ở backend** (O6, README 3.5), không `localStorage`, để thành viên khác cùng worktree thấy cùng trạng thái.
- **Mặc định ưu tiên phát hiện do thay đổi này** (`origin='introduced_in_scope'`): review sau khi agent code quan tâm cái agent gây ra; phát hiện cũ vẫn xem được bằng bộ lọc.
- **Không bịa "không ai dùng"** khi thiếu dữ liệu `consumers`.
- **Đặt danh sách ở đáy khung**, không phải một lens: phát hiện xuyên qua mọi lens (10 §6.7).

## 4. Tiêu chí chấp nhận

- [ ] Lens Hợp đồng hiển thị bảng Trước/Sau cho proto, route, kênh WS và schema, nhóm theo service và loại, có lọc theo loại/service/"chỉ phá vỡ"/"chỉ do thay đổi này"/tìm kiếm.
- [ ] Thay đổi `breaking` có icon + nhãn chữ + lý do bấm xem được; `unknown` hiển thị "Chưa xác định", không như "tương thích".
- [ ] Phần khác nhau giữa chữ ký trước/sau được làm nổi; thay đổi `db_schema` có nút "Mở trong ERD".
- [ ] Mỗi dòng có liên kết tới diff (đúng dòng khi CR-CV-053 hỗ trợ; nếu chưa, mở đúng tệp và ghi rõ giới hạn).
- [ ] `FindingsPanel` liệt kê phát hiện theo kind với bộ lọc kind/severity/origin/trạng thái/tìm kiếm; mặc định ưu tiên `introduced_in_scope`; ảo hoá khi >50 dòng.
- [ ] "Bỏ qua" bắt buộc chọn lý do; "Đã xử lý" một cú bấm; cả hai gọi `dismissFinding`, khoá nút ngay, cập nhật lạc quan, hoàn nguyên và báo inline khi lỗi, có "Mở lại".
- [ ] `Mod+Enter` xác nhận popover: `metaKey` trên Mac, `ctrlKey` nơi khác; nhãn phím khớp nền tảng.
- [ ] Phát hiện đã bỏ qua mặc định ẩn nhưng đếm được và hiện lại bằng bộ lọc; trạng thái dùng chung giữa các người xem cùng worktree (sau khi tải lại từ backend).
- [ ] Rỗng, đang tải, `truncated`, `stale`, lỗi, offline có UI riêng đúng bảng 2.5; không có câu "an toàn/không ai dùng" khi chỉ là "không phát hiện".
- [ ] `evidence` được che dữ liệu nhạy cảm trước khi render; test chứng minh.
- [ ] Selector `selectOpenFindingsBySymbolKey` có sẵn cho các lens khác.
- [ ] Không hex cứng; chuỗi `translate()` đủ 5 locale; không dùng `components/code-review/*`; không thêm thư viện; không thêm `max-lines` disable; hoạt động ở Electron và web.

## 5. Kiểm thử

Vitest + Testing Library theo mẫu repo (`renderToStaticMarkup` cho phần tĩnh như `DashboardAgentRow.test.tsx`):

- Hàm thuần: `contract-signature-diff` (token diff: thêm/xoá/đổi, chuỗi rỗng, chuẩn hoá khoảng trắng), `finding-filter` (kind, severity, origin, status, tìm kiếm, tổ hợp), `contract-grouping` (nhóm service/loại, đếm theo compatibility), `finding-sort` (severity → origin).
- Component: `ContractChangeTable` (4 cột; `breaking`/`unknown` có nhãn chữ), `ContractChangeDetail` (lý do, consumers rỗng không nói "không ai dùng", nút ERD chỉ với `db_schema`), `FindingRow` (hành động khác nhau theo `origin`), `FindingDismissPopover` (lý do bắt buộc với "Bỏ qua", `Mod+Enter` theo nền tảng bằng cách giả lập `navigator.userAgent`), `FindingsPanel` (rỗng/lỗi/đang tải).
- Hook/slice: `dismissFinding` lạc quan + hoàn nguyên khi lỗi, khoá nút trong lúc chờ, bỏ qua phản hồi cũ, dọn khoá `findings` khi xoá worktree (mẫu test rò rỉ như `store/slices/*-worktree-purge-leak.test.ts`).
- Bảo mật: `evidence` chứa chuỗi giống DSN không xuất hiện nguyên văn trong DOM.
- Giả lập `codeIntelClient.call` theo mẫu CR-CV-050. E2E cần backend CR-CV-037/038 (đợt 5): **chưa chạy**, đây là kế hoạch.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Chưa có message `Finding`/`ContractDiff`** trong hợp đồng; 2.1 là đề xuất UI cần; lệch sẽ phải điều chỉnh ở CR-CV-037/038.
- **`finding_dismissals` thiếu `disposition` và hành động khôi phục**: bảng chỉ có `reason`, RPC chỉ có `DismissFinding`. Cần thêm cột/tham số hoặc quy ước tiền tố trong `reason` (kém sạch).
- **Độ ổn định của `finding_key`**: nếu khoá gồm số dòng thì chỉnh sửa nhỏ làm phát hiện đã bỏ qua hiện lại. Khuyến nghị khoá = hash(kind + định danh logic như cặp package/symbol/tên bảng + truy vấn chuẩn hoá), không có dòng.
- **Độ chính xác phát hiện** (hotspot, mã chết, `tenant_id`) là heuristic; nhãn "gợi ý" cần nhất quán, nếu không người dùng tin quá mức.
- **Độ phủ `consumers`**: proto client↔server và kênh `wscompat` do CR-CV-032 nối; client ngoài repo sẽ không thấy.
- Nhảy đúng dòng trong diff phụ thuộc CR-CV-053 (hiện `EditorOpenTargetOptions` không có tham số dòng).
- Quyền bỏ qua phát hiện (ai được phép) chưa được README v7 nêu; CR-CV-013 (phân quyền) cần chốt.
- Số lượng phát hiện lớn (hotspot, mã chết) có thể hàng trăm; ngưỡng ảo hoá 50/100 là đề xuất, chưa đo.

## 7. Câu hỏi mở

1. "Đã xử lý" có nên tự biến mất khi lần phân tích sau không còn thấy phát hiện, và nếu phát hiện lại xuất hiện thì có "mở lại" tự động không?
2. Việc bỏ qua theo `repo_binding` (mọi người, mọi worktree) hay theo `(worktree, commit)` như `review_states`? Mặc định README là theo `repo_binding`.
3. `dismissFinding` có hỗ trợ hành động hàng loạt (bỏ qua mọi hotspot cùng thư mục) không?
4. Hợp đồng `ws_channel` phía frontend (tên kênh `codeIntel.*`, `window.api.codeIntel`) có thuộc phạm vi so sánh không, hay chỉ `wscompat` phía gateway?
5. Có cần xuất bảng hợp đồng ra Markdown để dán vào mô tả PR không (10 §5 có nhắc "sao chép/xuất" cho lens Luồng, không cho lens này)?

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (3.5 `finding_dismissals`, 3.6, 3.7, O6, O8)
- `/opt/repos/orca/docs/research/view-code/08-views-and-review-models.md` (§7 R2, R3, R4, R7, R8)
- `/opt/repos/orca/docs/research/view-code/10-frontend-review-ux.md` (§5 lens Hợp đồng, §6.1, §6.7, §7)
- `/opt/repos/orca/guides/STYLEGUIDE.md` (UX rule 1, toast so với inline, SSH/latency, reduced-motion)
- `/opt/repos/orca/frontend/src/renderer/src/lib/screen-submit-shortcut.ts`, `components/ShortcutKeyCombo.tsx`
- `/opt/repos/orca/frontend/src/renderer/src/store/slices/editor.ts` (`openDiff`, `EditorOpenTargetOptions`)
- `/opt/repos/orca/docs/crs/v7/review-frontend/CR-CV-050-review-frontend-foundation.md`, `CR-CV-051-review-workspace-shell.md`
- `/opt/repos/orca/frontend/src/renderer/src/components/dashboard/DashboardAgentRow.test.tsx` (mẫu test)
- Mới: `components/review-map/contract/{ContractDiffLens,ContractChangeTable,ContractSignatureCell,ContractCompatibilityBadge,ContractChangeDetail}.tsx`, `contract-signature-diff.ts`, `contract-grouping.ts`, `use-code-intel-contract-diff.ts`; `components/review-map/findings/{FindingsPanel,FindingsToolbar,FindingsList,FindingRow,FindingDismissPopover}.tsx`, `finding-filter.ts`, `finding-sort.ts`, `use-code-intel-findings.ts`

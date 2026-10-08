# FE-CV-SOL-059: Lens Hợp đồng và danh sách Phát hiện (Bỏ qua / Đã xử lý)

> 🚧 **In Progress.** Trạng thái (cập nhật 2026-10-08): 6/7 task DONE; PARTIAL 0. Chi tiết ở dòng `**Status:**` và "Ghi chú hoàn thiện" của từng task.

**CR:** [CR-CV-059](../../../../../../docs/crs/v7/review-frontend/CR-CV-059-contract-lens-and-findings.md)
**Area:** frontend (`components/review-map/contract/`, `components/review-map/findings/`, hook, selector slice)
**Hợp đồng áp dụng:** [CONTRACT-codeintel-ui-api.md](../../../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md) (U1, U3, U5, U7, U9; §3.1 kênh `findings`, `dismissFinding`, `contractDiff`; §4.5 `Finding`, `ContractChange`, `ContractDiff`, `Owner`; §4.3 `IndexFreshness`, `ViolationRef`; §2.3 lỗi gồm `CODEINTEL_VERSION_CONFLICT`, `CODEINTEL_NOT_AUTHORIZED`; §2.4 giới hạn chuỗi `reason`/`note` ≤ 500, `findingKey` ≤ 128), [CONTRACT-codeintel-proto-and-data-map.md](../../../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md) (**PQ-02, PQ-03, PQ-04, PQ-05, PQ-06, PQ-12, PQ-30, PQ-32**; §7; §8.2; §8.3; §9 O-1, O-7, O-18). **TDD:** [v5/02-state-management](../../../../tdd/v5/02-state-management.md), [v5/05-ui-components](../../../../tdd/v5/05-ui-components.md), [v5/07-hooks-and-ipc](../../../../tdd/v5/07-hooks-and-ipc.md).

## 1. Trạng thái hiện tại (re-verify)

**Đã xác minh trong phiên này:** `components/review-map/` chưa tồn tại; có `lib/screen-submit-shortcut.ts` (+ test) và `components/ShortcutKeyCombo.tsx`; `components/ui/` có `table`, `badge`, `select`, `popover`, `textarea`, `toggle-group`, `tooltip`, `skeleton`, `scroll-area`, `collapsible`; `@tanstack/react-virtual` có trong `frontend/package.json`; không có `maskSensitiveText` (SOL-057-01 tạo). `store/slices/store-test-helpers.ts` tồn tại (cần thêm slice nếu SOL-050 chưa thêm).
**Theo CR-CV-059 (chưa kiểm lại):** `ChecksPanel.tsx`/`PullRequestPage.tsx` là mẫu "danh sách có lọc + hành động"; `EditorOpenTargetOptions` chưa có tham số dòng.

### Lệch giữa CR và hợp đồng (hợp đồng thắng; PQ-06, PQ-30)

| # | CR-059 ghi | Hợp đồng | Quyết định |
|---|---|---|---|
| 1 | `Finding.key`, `kind` do UI giữ, `severity: high|medium|low|info`, `title/summary` chuỗi, `locations[]`, `status`, `dismissal.{disposition,reason,by,at}` | `findingKey`, `rule` (chuỗi mở), `kind` do **backend** điền, `severity: error|warning|info`, `titleKey`+`params`, `subject`, `evidence[] {path, line?, symbol?}`, `metrics`, `owner?`, `scope`, `confidence`, `dismissed?: {by, at, reason, disposition, note?}` | Dùng kiểu hợp đồng. UI dựng tiêu đề từ `titleKey`+`params` qua bảng dịch; `rule`/`titleKey` lạ ⇒ hiển thị nguyên `rule` |
| 2 | `origin: introduced_in_scope|preexisting|unknown` | `origin: introduced|touched|preexisting|unknown` | Bốn giá trị; "Chỉ do thay đổi này" = `introduced` (+ tuỳ chọn `touched`) |
| 3 | `Contract.contractKind: proto_rpc|…|db_schema`, `compatibility: breaking|compatible|unknown`, `breakingReasons[]`, `before`/`after`, `location`, `consumers {symbol, side}` | `kind: proto-service|proto-rpc|proto-message|proto-field|proto-enum|ws-channel|ws-channel-arg|route|route-field|sql-table|sql-column`; `compatibility: breaking|risky|compatible|unknown`; `ruleId`; `details: Record<string,string>`; `files[]`; `evidence: SourceRef[]`; `consumers {kind: rpc-client|ws-channel|route, service?, symbol?, path?}` | Thêm trạng thái `risky`; `before`/`after` **không** là trường hợp đồng (xem 2.2 và mục 7 "Thiếu hợp đồng"); `breakingReasons` ⇒ `ruleId` + `details` |
| 4 | Cần tham số khôi phục ("Mở lại") | `dismissFinding.action?: 'dismiss'|'restore'`, `disposition?: 'ignored'|'resolved'`, `reason?`, `note?` (≤ 500) ⇒ trả `{findingKey, dismissed, disposition?}` | Dùng `action:'restore'` |
| 5 | Lý do chọn trước `not_applicable|accepted_risk|false_positive|later` | `reason` là chuỗi tự do ≤ 500; hợp đồng không định nghĩa mã | Đặt **mã preset vào `reason`** (vd `false_positive`) và lời giải thích vào `note`; mã lạ hiển thị nguyên văn; câu hỏi mở 1 |
| 6 | Phạm vi của việc bỏ qua `repo_binding_id` | PQ-05: khoá `repo_id` (một repo nhiều worktree/binding); `finding_key` ≤ 128, không chứa số dòng; `Dismiss` **không bao giờ** miễn cổng với `error`/`blockingSeverities` | Copy UI nêu "áp cho cả repo"; không hứa miễn cổng |
| 7 | `FindingsPanel` có "Ghi chú", tab riêng | `Finding` và `QualityFinding` là hai nguồn, **một dock**, không trộn (PQ-06) | Dock do SOL-051 sở hữu; solution này chỉ thêm tab "Cấu trúc" (nguồn `Finding`); `QualityFinding` thuộc `FE-CV-SOL-087-*` |
| 8 | Danh sách lọc theo `kind` | `findings` lọc phía server theo `rules[]`, `severities`, `pathPrefix`, `includeDismissed`, `scope: all|changed (mặc định changed)`, `base`; trả `{findings, dismissedCount, indexFreshness}` + `nextPageToken`, `limit ≤ 200` | Server-side: `severities`, `pathPrefix`, `includeDismissed`, `scope`; `kind`/`origin`/tìm kiếm là client-side trên các trang đã tải; hiển thị "đang hiển thị X / total" nếu `nextPageToken` |
| 9 | Tên khoá i18n `auto.components.review.map.contract.|findings.` | README nhóm `auto.components.reviewMap.<Thành phần>.<tên>` | Theo README nhóm |
| 10 | Hook `use-code-intel-findings.ts`/`use-code-intel-contract-diff.ts` | README nhóm `useCodeIntel*.ts` | `useCodeIntelFindings.ts`, `useCodeIntelContractDiff.ts` |
| 11 | `contractDiff` không tham số `kinds` | `kinds?: ('proto'|'ws-channel'|'route'|'migration')[]`, `detail?: 'summary'|'full'`, `base?` | Bộ lọc "Loại" gửi `kinds`; `summary` cho chip đếm |
| 12 | `ContractDiff.migrations[]` không có | `migrations[] {service, dialects, files, statements, tables, findings}` | Tab/nhóm "Migration" cho bảng và phát hiện `sql.*`; "Mở trong ERD" cho `tables[]` |

## 2. Giải pháp

### 2.1 Cây file

```
components/review-map/contract/
  contract-grouping.ts          (mới) nhóm theo service → kind, đếm theo compatibility
  contract-detail-rows.ts       (mới) details → hàng key/value; nhận diện cặp before/after theo quy ước (2.2)
  contract-signature-diff.ts    (mới) diff token cho cặp chữ ký (nếu có)
  ContractDiffLens.tsx, ContractChangeTable.tsx, ContractSignatureCell.tsx,
  ContractCompatibilityBadge.tsx, ContractChangeDetail.tsx, ContractMigrationGroup.tsx   (mới)
components/review-map/findings/
  finding-view-model.ts         (mới) Finding → hàng hiển thị (titleKey+params → chuỗi dịch, evidence → vị trí)
  finding-filter.ts, finding-sort.ts   (mới)
  FindingsPanel.tsx, FindingsToolbar.tsx, FindingsList.tsx, FindingRow.tsx, FindingDismissPopover.tsx   (mới)
hooks/useCodeIntelFindings.ts, useCodeIntelContractDiff.ts, useFindingDismissal.ts   (mới)
store/slices/code-intel.ts      (sửa nhỏ: khoá `findingFilters`, `contractFilters`, selector selectOpenFindingsBySymbolKey)
```
Đăng ký lens `id:'contract'` (SOL-051); `FindingsPanel` gắn vào vùng dock đáy (SOL-051), **không** là lens.

### 2.2 Lens Hợp đồng

```
┌ [Loại ▾ proto|ws-channel|route|migration] [Service ▾] [☐ Chỉ phá vỡ] [☐ Chỉ do thay đổi này] [🔍] ──────────────┐
│ Tóm tắt (mỗi chip là bộ lọc): 14 thay đổi · 2 PHÁ VỠ · 1 RỦI RO · 1 chưa xác định · 10 tương thích               │
├──────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ ▾ infra-fleet-service · proto (5)                                                                             │
│ │ Hợp đồng (kind·name)            │ Trước                       │ Sau                    │ Tương thích       │ │
│ │ proto-field RelayByDevServer…   │ int32 timeout_ms = 3        │ (đã xoá)               │ ⛔ Phá vỡ [rule]  │ │
│ ▸ api-gateway · ws-channel (4)     ▸ Migration (2) → [Mở trong ERD]                                           │
└──────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```
- Nhóm theo `ContractChange.service` rồi `kind` (`ui/collapsible`); bảng `ui/table`, ảo hoá khi > 100 dòng. Chip tóm tắt từ `ContractDiff.summary {breaking, risky, compatible, unknown}`.
- **UI không tự phân loại**: chỉ ánh xạ `compatibility` sang nhãn + icon (`breaking` → `destructive` + `ShieldAlert`; `risky` → muted đậm + `AlertTriangle`; `compatible` → muted + `Check`; `unknown` → muted + `HelpCircle` + chữ "Chưa xác định"). `unknown` **không** dùng biểu tượng như `compatible`. `ruleId` lạ hiển thị nguyên văn.
- Cột Trước/Sau: hợp đồng `ContractChange` **không** có `before`/`after`; chỉ có `details: Record<string,string>`. Quy ước UI (chờ chốt, mục 7): nếu `details.before`/`details.after` tồn tại thì hiển thị chữ ký mono + diff token (`contract-signature-diff.ts`, không dùng Monaco); nếu không thì hiển thị bảng `details` key/value và `change` (`added|removed|modified`). Mọi chuỗi trong `details` là văn bản thuần (U9) và đi qua `maskSensitiveText`.
- `ContractChangeDetail` (cột phải, SOL-053): `ruleId` → dịch theo bảng (lạ ⇒ nguyên văn), `details`, `files[]` ("Xem diff" mở diff đúng dòng qua SOL-053 nếu tệp ∈ `changedFiles`, nếu không "Mở tệp"), `evidence[]`, `consumers[]` (`rpc-client|ws-channel|route`, bấm mở `symbol` qua SOL-053). `breaking` mà `consumers: []` ⇒ "Chưa tìm thấy nơi dùng (có thể ngoài repo hoặc chưa được lập chỉ mục)" — **không** viết "không ai dùng" (U-quy tắc §4.5).
- Nhóm Migration (`ContractDiff.migrations[]`): mỗi service một khối; `statements[] {table, op, column?, compatibility, ruleId, dialectOnly?}` hiển thị bảng gọn; `tables[]` có "Mở trong ERD" ⇒ `setReviewLens(…,'erd')` + `setErdService(service)` + `selectErdTable` (SOL-057); `findings[]` của migration (`sql.*`) dùng cùng `FindingRow`.

### 2.3 Danh sách Phát hiện (`Finding`, nguồn cấu trúc)

- `useCodeIntelFindings({worktreeId, filters})` → `{status, findings, dismissedCount, indexFreshness, nextPageToken, loadMore, error, reload}` (kênh `findings`; `limit` ≤ 200 mặc định 100; `scope` mặc định `'changed'`). Bỏ phản hồi cũ khi đổi tham số.
- `FindingRow`: icon theo `kind`, tiêu đề (`titleKey`+`params`), `severity` (`error|warning|info`, icon + chữ), vị trí từ `evidence[0]` (`path:line`), nhãn `origin` (`introduced` "Do thay đổi này", `touched` "Trong tệp đã đổi", `preexisting` "Có từ trước", `unknown`), `confidence`, `owner?.names`; hành động: `[Xem trong đồ thị] [Xem diff|Mở tệp] [Ghi chú] [Bỏ qua ▾] [Đã xử lý] ([Mở lại] khi `dismissed`)`.
- "Xem trong đồ thị": `layer_violation`/`dependency_cycle` → lens Cấu trúc/Ảnh hưởng nếu symbol có trong đồ thị đang hiển thị (ngược lại "Không có trong đồ thị hiện tại"); `hotspot`/`dead_code` → Cấu trúc; `missing_tenant_id`/`rls_removed` → nút "Mở trong ERD" khi có bảng trong `subject`/`params` (không đoán, chỉ khi `params.table` có), nếu không thì không có nút.
- **Bỏ qua** (`disposition:'ignored'`): bắt buộc chọn preset lý do (đặt vào `reason`), `note` tuỳ chọn; **Đã xử lý** (`'resolved'`): một cú bấm, `note` tuỳ chọn; **Mở lại**: `action:'restore'`. Cả ba gọi `codeIntel.dismissFinding` qua `useFindingDismissal`: khoá nút ngay, `Loader2` sau ~200 ms (SSH), cập nhật lạc quan, hoàn nguyên + thông báo inline trên dòng khi lỗi (`forbidden` → "Bạn không có quyền bỏ qua phát hiện"; `network/offline` giữ dòng cũ + "Thử lại"; `CODEINTEL_NOT_FOUND` → tải lại). Toast chỉ cho xác nhận thoáng qua ("Đã bỏ qua" + "Hoàn tác"). Copy nêu rõ "Bỏ qua áp cho cả repo" (PQ-05) và **không** hứa nó miễn cổng chất lượng.
- `Mod+Enter` xác nhận popover bằng `isScreenSubmitShortcut` (`metaKey` Mac, `ctrlKey` nơi khác), chip phím bằng `ShortcutKeyCombo`. Phím cục bộ khi tiêu điểm trong danh sách: `j`/`k`, `Enter`, `d`, `r`, `Esc` (không phím bổ trợ; bỏ qua ở ô nhập; không xung đột `j/k/Space` của "Thứ tự đọc" SOL-052 — dùng registry phím cục bộ nếu SOL-052 có).
- Chỉ báo trên đồ thị: slice xuất `selectOpenFindingsBySymbolKey(worktreeId)` (từ `evidence[].symbol.key`, loại `dismissed`) để lens khác tô icon cảnh báo; solution này chỉ cung cấp selector.
- `evidence[]` chỉ chứa `path/line/symbol` (không đoạn mã) ⇒ không cần che nhiều, nhưng chuỗi `subject`, `params` qua `maskSensitiveText`.
- Đếm "đã bỏ qua N" từ `dismissedCount`; hiện lại bằng `includeDismissed:true`; `indexFreshness` (`behind|dirty|missing`) hiển thị chip "Dữ liệu có thể cũ", **không** viết "không có vấn đề".

### 2.4 Trạng thái và lỗi

| Tình huống | Hiển thị |
|---|---|
| Đang tải | Ngưỡng 100 ms/1 s/3 s; qua SSH trì hoãn ~200 ms, khoá điều khiển ngay |
| Không có phát hiện | "Không phát hiện vấn đề nào trong phạm vi index" + commit/ngày index (không "an toàn") |
| Không có thay đổi hợp đồng | "Không có thay đổi hợp đồng nào được phát hiện trong phạm vi này" + loại nguồn đã quét |
| `index-missing`/`tool-unavailable` | Phát hiện cấu trúc rỗng kèm trạng thái chuẩn của khung (Lập chỉ mục/hướng dẫn); hợp đồng proto/SQL (đọc file) có thể vẫn có |
| `truncated`/`nextPageToken` | "Đang hiển thị X / Y" + gợi ý lọc; nút "Tải thêm" |
| `stale`/`offline` | Banner chung của khung; giữ dữ liệu cache |
| Lỗi `CODEINTEL_*` | Persistent inline + "Thử lại"; `CODEINTEL_DISABLED` ⇒ ẩn (khung); `CODEINTEL_QUALITY_GATE_DISABLED` không liên quan ở đây |

### 2.5 i18n, token, a11y

Khoá `auto.components.reviewMap.Contract*|Finding*` đủ en/es/ja/ko/zh; `ruleId`/`rule`/`titleKey` có bảng dịch riêng, lạ ⇒ nguyên văn (U-quy tắc "enum lạ"). Màu chỉ biến CSS (`destructive`, muted, accent); token severity trung gian do SOL-050 thêm (`main.css` hiện không định nghĩa `--warning` theo CR; không nhân bản `var(--warning, #f59e0b)`). Mọi trạng thái kèm icon + chữ. `role="table"`/`aria-sort` khi sắp xếp; dòng danh sách `role="listitem"` + `aria-label`.

## 3. Quyết định thiết kế

- **UI không phân loại tương thích**; `unknown`/`risky` là trạng thái hợp lệ, hiển thị trung thực.
- **Bỏ qua ≠ Đã xử lý ≠ miễn trừ cổng**: hai disposition khác nghĩa; waiver là kênh `quality.waive` (SOL-085/087), không lẫn.
- **Lưu hiệu lực ở backend theo `findingKey`**, không `localStorage`, để thành viên khác thấy cùng trạng thái.
- **Ưu tiên phát hiện do thay đổi này** (`origin='introduced'`), `touched`/`preexisting` xem bằng bộ lọc.
- **Hai nguồn trong một dock**: `Finding` ở tab riêng; không trộn `QualityFinding`.

## 4. Phụ thuộc chéo khu vực

| Cần | Solution đối ứng (§8.2) | Ghi chú |
|---|---|---|
| Danh mục proto + kênh `wscompat` | `BE-CV-SOL-032-proto-and-wscompat-contract-catalog` | Đầu vào của `contractDiff` |
| Vi phạm lớp/vòng/hotspot/mã chết + `finding_dismissals` | `BE-CV-SOL-037-structure-findings-and-dismissals`; agent: `AG-CV-SOL-037-structural-facts`; DB: `BE-CV-SOL-011-data-model-and-migrations` | `dismissFinding` ghi `finding_dismissals` (PQ-05) |
| Contract diff + `sql.*` thiếu `tenant_id` | `BE-CV-SOL-038-contract-diff`; `BE-CV-SOL-038-static-tenant-filter-rule` | P2 |
| Kênh `findings`, `contractDiff`, `dismissFinding` | `BE-CV-SOL-040-codeintel-view-channels`; `BE-CV-SOL-040-codeintel-write-and-stream-channels` (cổng nền **G3**: `-channel-foundation`) | Ghi dùng 8 s timeout, `review_write` |
| Bridge/store/fake backend G4 | `FE-CV-SOL-050-*`; `FE-CV-SOL-073-flag-gating-and-web-e2e` (task 073-02) | **Bắt đầu bằng fake backend G4** |
| Dock/khung/panel/mở diff | `FE-CV-SOL-051-…`, `FE-CV-SOL-053-…` | |
| Ghi chú gắn vào phát hiện; số phát hiện ở right sidebar; danh sách trên mobile | `FE-CV-SOL-060-…`, `FE-CV-SOL-061-…`, `FE-CV-SOL-062-…` | Mở khoá |
| Mở ERD | `FE-CV-SOL-057-erd-lens` | |
| Quyền ai được bỏ qua | `BE-CV-SOL-013-authorization-flags-and-audit` (O-18) | Hợp đồng: `review_write` |

Thứ tự (§7.2): 050 → 051 → 053 → 054..058 → **059** → 060 → 061. Đợt 5 (cùng `BE-CV-SOL-037/038`, `BE-CV-SOL-040` ghi + stream).

## 5. Tiêu chí chấp nhận

- [ ] Lens Hợp đồng hiển thị theo service/loại với lọc `kinds`, "Chỉ phá vỡ", "Chỉ do thay đổi này", tìm kiếm; chip tóm tắt từ `summary`.
- [ ] `breaking|risky|unknown|compatible` đều có icon + nhãn chữ; `unknown` không giống `compatible`; `breaking` có `ruleId` + `details` bấm xem được; không có logic phân loại ở UI.
- [ ] Cặp Trước/Sau dựng từ `details.before/after` nếu có, nếu không hiển thị `details`; nhóm Migration có "Mở trong ERD".
- [ ] `breaking` + `consumers: []` ⇒ "Chưa tìm thấy nơi dùng…", không "không ai dùng".
- [ ] `FindingsPanel` (nguồn `Finding`) lọc `severity/origin/kind/tìm kiếm/đã bỏ qua`, mặc định `scope=changed`, ưu tiên `introduced`, ảo hoá > 50 dòng, "Tải thêm" theo `nextPageToken`.
- [ ] Bỏ qua cần preset lý do (`reason`), Đã xử lý một cú bấm, Mở lại (`restore`); khoá nút ngay; cập nhật lạc quan + hoàn nguyên inline; `Mod+Enter` đúng nền tảng.
- [ ] Phát hiện đã bỏ qua mặc định ẩn nhưng đếm được (`dismissedCount`) và hiện lại bằng `includeDismissed`.
- [ ] `selectOpenFindingsBySymbolKey` có sẵn cho lens khác.
- [ ] Không câu "an toàn/không ai dùng/AI đã review"; rỗng/tải/`truncated`/`stale`/lỗi/offline đúng 2.4; không toast cho lỗi cần đọc.
- [ ] Không hex, `translate()` đủ 5 locale, không `components/code-review/*`, không thêm thư viện, không `max-lines` disable; Electron và web cùng mã.

## 6. Kiểm thử (Vitest + Testing Library)

Hàm thuần: `contract-grouping`, `contract-signature-diff` (thêm/xoá/đổi, rỗng, khoảng trắng), `contract-detail-rows` (có/không before/after), `finding-view-model` (`titleKey` lạ ⇒ `rule`), `finding-filter`, `finding-sort` (severity rồi origin). Component: `ContractChangeTable` (`unknown`/`risky` có chữ), `ContractChangeDetail` (consumers rỗng; nút ERD chỉ `sql-*`/migration), `FindingRow` (hành động theo `origin`), `FindingDismissPopover` (lý do bắt buộc với "Bỏ qua"; `Mod+Enter` theo nền tảng bằng giả lập `navigator.userAgent`), `FindingsPanel` (rỗng/lỗi/tải). Hook/slice: lạc quan + hoàn nguyên, khoá nút, bỏ phản hồi cũ, `CODEINTEL_VERSION_CONFLICT`/`NOT_FOUND` tải lại, dọn khoá `findings*`/`contract*` khi xoá worktree (mẫu `*-worktree-purge-leak.test.ts`). Bảo mật: chuỗi giống DSN trong `params`/`details` không xuất hiện nguyên văn. E2E (`tests/e2e/code-intel-web/lenses.web.e2e.ts`) trên fake backend; tích hợp thật cần `BE-CV-SOL-037/038` — **chưa chạy**.

## 7. Rủi ro, điểm chưa kiểm chứng và thiếu hợp đồng

- **Thiếu hợp đồng:** `ContractChange` không có `before`/`after`; đề xuất quy ước khoá `details.before|after` (chưa chốt với BE-CV-SOL-038); không có quy ước mã preset cho `reason`; không có danh sách `titleKey`/`ruleId` đóng (UI cần bảng dịch, mã lạ nguyên văn).
- Độ ổn định `findingKey` (không chứa số dòng, PQ-05) quyết định việc "đã bỏ qua" có hiện lại; heuristic hotspot/mã chết/`tenant_id` có tỉ lệ báo nhầm (O-7), nhãn "gợi ý" phải nhất quán.
- `consumers` chỉ phủ trong repo; client ngoài repo không thấy.
- Nhảy đúng dòng diff phụ thuộc SOL-053 (`pendingDiffReveal`).
- Quyền bỏ qua: hợp đồng `review_write`; O-18 (member) chưa chốt.
- Số phát hiện lớn (hàng trăm); ngưỡng ảo hoá 50/100 chưa đo.
- Cần `projectId` (O-1).

## 8. Câu hỏi mở

1. Mã preset lý do (`not_applicable|accepted_risk|false_positive|later`) có được backend/BE-CV-SOL-037 chốt làm giá trị hợp lệ của `reason` không, hay chỉ là chuỗi tự do?
2. "Đã xử lý" có nên tự biến mất khi lần phân tích sau không còn thấy, và mở lại tự động nếu xuất hiện lại?
3. `dismissFinding` có cần hỗ trợ hàng loạt (bỏ qua mọi hotspot cùng thư mục)?
4. `ws-channel` của phía **frontend** (`codeIntel.*`, `window.api.codeIntel`) có thuộc phạm vi so sánh hợp đồng không, hay chỉ `wscompat` phía gateway?
5. Có cần xuất bảng hợp đồng ra Markdown để dán vào mô tả PR không?

## 9. Tham chiếu

`/opt/repos/orca/docs/crs/v7/review-frontend/CR-CV-059-contract-lens-and-findings.md`, `/opt/repos/orca/docs/research/view-code/10-frontend-review-ux.md` (§5, §6.1, §6.7, §7), `/opt/repos/orca/guides/STYLEGUIDE.md`, `/opt/repos/orca/frontend/src/renderer/src/lib/screen-submit-shortcut.ts`, `/opt/repos/orca/frontend/src/renderer/src/components/ShortcutKeyCombo.tsx`, `/opt/repos/orca/frontend/src/renderer/src/store/slices/editor.ts`.

## 10. Ghi chú triển khai (2026-10-07)

**Sai lệch so với spec**

- Không sửa `store/slices/code-intel.ts`: bộ lọc là state cục bộ; `selectOpenFindingsBySymbolKey` là hàm thuần (`findings/open-findings-by-symbol.ts`) + hook `useOpenFindingsBySymbolKey`.
- Phân trang `findings` tự viết trên `useCodeIntelQuery` (hook phân trang chung không trả `dismissedCount`/`indexFreshness`); bỏ qua lạc quan qua map override, bị bỏ khi tải lại.
- `FindingsPanel` là component độc lập chờ dock đáy của khung (chưa có); "Ghi chú" dùng `ReviewNoteButton` (SOL-060) với neo `finding`.
- Chi tiết thay đổi hợp đồng hiển thị dưới bảng, không vào drawer symbol. "Xem diff" chỉ cho tệp ∈ `changedFiles`; "Mở trong ERD" chỉ khi lens `erd` đã có `load`; khoá bảng ERD giả định là tên bảng.
- Quy ước `details.before|after` giữ trong `contract-detail-rows.ts` (chưa chốt với BE-CV-SOL-038). Bảng `titleKey` đóng: chỉ `finding.<kind>.title`; còn lại hiển thị `rule`.
- Phím cục bộ `j/k/Enter/d/r/Esc` xử lý trong panel (chưa qua registry SOL-052). Bộ lọc dịch vụ dùng `<select>` gốc.
- Thiếu: e2e (không có `tests/e2e/code-intel-web`), nhóm Migration chưa render `findings[]` bằng `FindingRow`, chip "N phát hiện" ở thanh tóm tắt, tô icon cảnh báo trên nút lens khác.
- Khoá i18n: `auto.components.reviewMap.contract.*` và `.findings.*`; test phủ khoá mới `i18n/contract-findings-notes-locale-coverage.test.ts`.
- Fake backend: fixture `test-support/contract-findings-fixtures.ts`; hook test dùng `callFn` tiêm vào (mẫu `useCodeIntelPagedQuery.test.tsx`).

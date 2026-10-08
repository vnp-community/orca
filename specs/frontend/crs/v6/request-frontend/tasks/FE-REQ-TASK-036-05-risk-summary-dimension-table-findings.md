# FE-REQ-TASK-036-05: `RiskSummaryCard`, `SolutionDimensionTable`, `ImpactFindingList` và `ImpactEvidenceSheet`

**From Solution:** [FE-REQ-SOL-036](../solutions/FE-REQ-SOL-036-clarification-decision-readiness-impact-ui.md) mục 2.6
**Priority:** P1
**Area:** frontend / request / impact
**File:** `frontend/src/renderer/src/components/request/impact/{RiskSummaryCard,SolutionDimensionTable,ImpactFindingList,ImpactEvidenceSheet}.tsx`, `impact-dimension-model.ts` (mới); `components/request/solution/SolutionOptionCard.tsx`, `SolutionComparisonTable.tsx` (sửa, FE-REQ-TASK-020-02/020-03); test cùng tên
**Depends on:** FE-REQ-TASK-032-01 (`RiskBadge`), 032-05 (`GraphMini`), 032-06 (`GraphPanel`), FE-REQ-TASK-036-01, 036-02; FE-REQ-TASK-020-02, 020-03
**Status:** [~] PARTIAL — vitest impact/impact-dimension-model (6), impact-components (10), SolutionImpactSection (3) pass — thiếu: nút "Xem đồ thị" trên thẻ, gộp hàng Effort/Quay lui, GraphPanel cho finding

## Context

- CR-REQ-036 2.4: `RiskSummaryCard` trong mỗi `SolutionOptionCard` hiển thị `RiskBadge` (chữ, icon), điểm 0 đến 100, **ba lý do chính**, dòng "Đánh giá lúc <giờ>, dựa trên <công cụ>"; chưa đánh giá thì "Chưa đánh giá" (không Thấp); `stale` thì "Index cũ, chưa đánh giá được đầy đủ". Bảng chiều: Kiến trúc, Tương thích hợp đồng, Dữ liệu, Phạm vi ảnh hưởng, Bảo mật và quyền, Vận hành, Chất lượng, Bất định, Quy mô; cộng hàng Effort và Quay lui (SOL-020). Luật cứng: dòng "Nâng mức vì: <luật>". Giải thích AI có nhãn "Diễn giải bởi AI"; điểm và mức do quy tắc cố định: UI không có chỗ sửa điểm.
- CR-030 (đã đọc): `impact.get` trả mức, điểm, 3 lý do chính, độ tin cậy, tuổi index, `narrative`; `impact.compare {solutionId}` ma trận Option × 9 chiều; `impact.findings`, `impact.evidence` (đã cắt); `mode` `shadow` (chỉ hiển thị, nhãn "Tham khảo") hay `enforce`; độ tin cậy Option tối đa `medium` (chưa có diff, `AREA_UNRESOLVED` tăng chiều Bất định). Backend trả ba lý do sẵn, frontend không suy (câu hỏi mở 3 CR-036).
- `SolutionComparisonTable` hiện có (020-03) đánh dấu ô khác biệt; cột đề xuất đánh dấu, **không chọn sẵn**.
- `GraphMini` (032-05) tối đa 12 node; `GraphPanel` (032-06) mở trong `Sheet` với lens và node đúng (`meta.findingIds`).
- Bằng chứng thô là văn bản (kết quả truy vấn, đường gọi): hiển thị bằng `<pre>` với `whitespace-pre-wrap`, **không** HTML hay Markdown.
- Primitive: `ui/table.tsx`, `ui/sheet.tsx`, `ui/skeleton.tsx`, `ui/badge.tsx`, `ui/tooltip.tsx`, `ui/collapsible.tsx`. Token rủi ro từ 032-01.

## Việc cần làm

1. `impact-dimension-model.ts` (thuần): `IMPACT_DIMENSIONS = ['architecture','contract','data','blast_radius','security','operations','quality','uncertainty','size'] as const` (khớp 9 chiều; tên khoá backend chưa chốt: parser của 036-01 chuẩn hoá), `buildComparisonRows(options, comparison, sol020Rows): ComparisonRow[]` (ghép 9 chiều với hàng Effort, Quay lui), `markDifferences(rows)` (ô khác biệt khi mức hoặc điểm khác giữa các cột), `summarizeCard(summary): { levelKey, scoreText | null, reasons: string[3 tối đa], captionKey, stale, mode }` (không đưa ra "Thấp" khi `summary==null` hoặc `level==='unknown'`).
2. `RiskSummaryCard.tsx`: props `{ subjectType; subjectId; solutionId?; compact?: boolean }` dùng `useImpactAssessment`. Hiển thị: `RiskBadge`, điểm ("Điểm 62/100", chỉ khi `score!=null`), tối đa ba lý do (danh sách), dòng độ tin cậy ("Đánh giá lúc <giờ>, dựa trên <công cụ>"; thêm "Độ tin cậy Trung bình" khi có), nhãn "Tham khảo" khi `mode==='shadow'`, nhãn "Diễn giải bởi AI" bên cạnh `narrative` (thu gọn, mở được), dòng "Nâng mức vì: <luật>" từ `hardRules`. Trạng thái: `collecting` skeleton + nhãn giai đoạn ("Đang truy vấn đồ thị") hoãn 200 ms; `unsupported`/chưa bật: "Chưa đánh giá" (không có chỗ lỗi đỏ); `noDevServer`: "Chưa có dev server kết nối" + nút kết nối (tái dùng hành động có sẵn của app, đọc trước khi viết); `forbidden`: "Bạn không có quyền xem đánh giá"; `stale`: "Index cũ, chưa đánh giá được đầy đủ". Nút "Chạy đánh giá" (khi `status==='idle'` và có quyền) gọi `request()` (`impact.request`).
3. `SolutionOptionCard` (sửa): thêm `RiskSummaryCard` và `GraphMini` (lens `architecture` của Option, `subjectType='solution_option'`); nếu `GraphPanel` khả dụng, nút "Xem đồ thị" mở `Sheet` với `GraphPanel`. Không đổi chọn phương án.
4. `SolutionDimensionTable.tsx`: mở rộng `SolutionComparisonTable`: hàng là chiều, cột là phương án; mỗi ô `RiskBadge size="sm"` + điểm + ghi chú một dòng (`text-muted-foreground`); cột đề xuất có dấu "Đề xuất" (chữ), **không chọn sẵn** (không radio mặc định); ô khác biệt đánh dấu như bảng hiện có; dữ liệu thiếu "Chưa đánh giá". Sticky cột đầu; trên màn hẹp cuộn ngang trong `ScrollArea` (STYLEGUIDE: scrollbar tuỳ biến, chạy `node config/scripts/check-styled-scrollbars.mjs` nếu thêm cuộn).
5. `ImpactFindingList.tsx`: `useImpactAssessment().findings` nhóm theo chiều, sắp theo mức giảm dần; mỗi mục: chiều, `RiskBadge`, mô tả, độ phủ; nút "Bằng chứng" mở `ImpactEvidenceSheet`; nút "Xem trên đồ thị" mở `GraphPanel` (đúng lens và `meta.findingIds`) khi có `nodeIds`.
6. `ImpactEvidenceSheet.tsx`: `Sheet`, gọi `impact.evidence {findingId}` khi mở; hiển thị văn bản thuần (đã cắt phía backend; thêm dòng "Đã cắt bớt" nếu có cờ), nút sao chép (`navigator.clipboard`, có `try/catch`); `forbidden` hiện thông báo.
7. Gỡ màu cứng: chỉ token; kiểm `rg -n '#[0-9a-fA-F]{3,6}' frontend/src/renderer/src/components/request/impact` rỗng.
8. Cập nhật SOL-022: cờ `requiresImpactReview` cho `ApprovalRow` dùng `summarizeCard` (không sửa SOL-022; ghi chú trong PR để người sở hữu thêm `RiskBadge`).

## Bảng tham chiếu nhanh

| Tình huống | UI |
|---|---|
| Đang sinh đánh giá (`collecting`) | skeleton + nhãn giai đoạn "Đang truy vấn đồ thị", hoãn 200 ms |
| Chưa đánh giá hoặc `unsupported` | "Chưa đánh giá" (không Thấp), Duyệt như SOL-020 |
| `stale` | "Index cũ, chưa đánh giá được đầy đủ" |
| `shadow` | nhãn "Tham khảo" |
| Không dev server | "Chưa có dev server kết nối" + nút kết nối |
| `forbidden` | "Bạn không có quyền xem đánh giá" |

- Kênh WS và payload: `impact.get {subjectType, subjectId}`, `impact.compare {solutionId}`, `impact.findings {assessmentId, minLevel:'medium'}`, `impact.evidence {findingId}`, `impact.request {subjectType, subjectId}`.
- Phím tắt: không có phím tắt riêng; `Esc` đóng `Sheet` bằng Radix.
- Khoá i18n: `auto.components.request.impact.{card.score,card.confidence,card.assessedAt,card.narrativeAi,card.raisedBy,card.shadow,card.runAssessment,card.notAssessed,table.recommended,table.notAssessed,dimension.architecture,dimension.contract,dimension.data,dimension.blast_radius,dimension.security,dimension.operations,dimension.quality,dimension.uncertainty,dimension.size,findings.title,findings.evidence,findings.viewOnGraph,evidence.truncated,evidence.copy}`.

## Kiểm thử

- `impact-dimension-model.test.ts`: 9 chiều + 2 hàng; `markDifferences`; `summarizeCard(null)` không bao giờ trả "low"; ba lý do tối đa; `stale` và `shadow`.
- `RiskSummaryCard.test.tsx` (mock `useImpactAssessment`): có đủ mức (5), điểm, ba lý do, thời điểm và công cụ; thiếu đánh giá → "Chưa đánh giá" và không `low`; `collecting` skeleton; `stale` nhãn; `shadow` nhãn "Tham khảo"; `noDevServer`; `forbidden`; nhãn "Diễn giải bởi AI" khi có `narrative`.
- `SolutionDimensionTable.test.tsx`: đủ chiều; cột đề xuất đánh dấu và không chọn sẵn; ô khác biệt; thiếu dữ liệu.
- `ImpactFindingList.test.tsx`: nhóm và thứ tự; "Xem trên đồ thị" mở `GraphPanel` với `findingIds`; không có nút khi không có `nodeIds`.
- `ImpactEvidenceSheet.test.tsx`: `<script>` trong bằng chứng hiển thị như văn bản; `forbidden`.
- Không có điểm nhập nào sửa điểm (kiểm không có `input` điều khiển điểm).
- Chạy: `pnpm --filter orca-frontend test src/renderer/src/components/request/impact src/renderer/src/components/request/solution`.

## Tiêu chí hoàn thành

- [ ] `RiskSummaryCard` hiện mức (chữ và icon), điểm, ba lý do, thời điểm và công cụ; thiếu đánh giá thì "Chưa đánh giá", không Thấp.
- [ ] `SolutionDimensionTable` đủ chiều, đánh dấu cột đề xuất mà không chọn sẵn.
- [ ] Bằng chứng là văn bản thuần; "Xem trên đồ thị" mở đúng lens và node.
- [ ] `stale`, `shadow`, `collecting`, `noDevServer`, `forbidden`, `unsupported` đều có UI.
- [ ] UI không có chỗ nào sửa điểm rủi ro.
- [ ] Không hex; token rủi ro từ 032-01.

## Rủi ro và lưu ý

- Thẻ Thấp có thể gây tự tin sai khi dữ liệu thiếu cạnh xuyên gRPC, WS, outbox (CR-030 mục 6): luôn kèm công cụ, thời điểm, và nhãn "Tham khảo" ở `shadow`.
- `impact.compare` và khoá chiều chưa có trong CONTRACT: ghép bằng parser chịu thiếu; chiều lạ hiển thị tên thô.
- Độ tin cậy tối đa `medium` ở Option: không gán nhãn "Cao" cho độ tin cậy khi backend đã cắt.
- Bảng nhiều cột trên màn hẹp: cuộn ngang; mobile mặc định danh sách phát hiện (chưa kiểm tra).
- `SolutionOptionCard` và `SolutionComparisonTable` do 020 sở hữu: chỉ thêm khối mới, không đổi hành vi chọn.

## Ghi chú triển khai (2026-10-07)

`RiskSummaryCard`, `SolutionDimensionTable`, `ImpactFindingList`, `ImpactEvidenceSheet` xong; `SolutionImpactSection` gắn dưới `SolutionOptionCompare` (cho option đang chọn) thay vì trong từng `SolutionOptionCard` (thẻ là `button role=radio`, không lồng điều khiển). `ImpactFindingList.onViewOnGraph` có nhưng chưa nối với `GraphPanel`. `SolutionDimensionTable` chỉ hiện 9 chiều tác động (Effort/Quay lui vẫn ở `SolutionComparisonTable`). Nút kết nối dev server chỉ hiện khi truyền `onConnectDevServer`.

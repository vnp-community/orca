# CR-OR-020 — Tab "Outcome review" của Request: khung, thẻ kết luận, yêu cầu giao so với hỏi

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-OR-020 |
| **Tên** | Tab `outcome-review` (nhãn "Outcome review", ngay sau Execution) trong `RequestDetailPane`; thư mục `components/request/outcome-review/`; thanh đầu (đối tượng, snapshot KG, trạng thái), bốn thẻ kết luận, danh sách "Needs your decision", bảng "Asked vs delivered", danh sách "Release readiness"; trạng thái (chưa sinh, đang sinh, sẵn sàng, cũ, KG vắng, lỗi); hook, parser dây, i18n; chốt tên kênh `request.review.*` |
| **Loại** | Feature |
| **Priority** | 🔴 P0 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-20 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-OR-001 (RPC, mô hình), CR-OR-002 (mục `origin=agent`, KG); CR-OR-021/022 hoàn thiện các khối bên trong |
| **Mở khoá** | CR-OR-021, 022 |
| **Tác động** | `frontend/src/renderer/src/components/request/outcome-review/` (mới), `components/request/RequestDetailPane.tsx` (thêm tab), `components/request/request-stage-timeline-model.ts` và `NextActionBar` / `next-action-rules` (thêm hàng Review và gợi ý hành động), `frontend/src/renderer/src/hooks/useOutcomeReview.ts` (mới), `frontend/src/shared/outcome-review-types.ts` và `outcome-review-wire-parsers.ts` (mới), `i18n/` (khoá và test bao phủ locale) |

---

## 1. Bối cảnh và vấn đề

Đã đọc code ngày 2026-10-20:

1. `RequestDetailPane.tsx` dùng `Tabs` với `overview`, `analysis`, `plan`, `execution`, `history`, `related`; tab `execution`/`plan` hiện có điều kiện; điều hướng tab có thể đến từ `nextAction.tab`. File dài **379 dòng**; `request-stage-timeline-model.ts` dài **299 dòng**: sát ngưỡng tách file (handoff mục 2: file quá 300 dòng mã thì tách, hook pre-commit chặn khi chạm file đang vượt ngưỡng).
2. Hook dữ liệu nằm ở `frontend/src/renderer/src/hooks/` (`useExecutionSummary.ts`, `useRequestFlow.ts`), parser dây ở `frontend/src/shared/request-wire-parsers.ts`. **Bẫy đã biết**: parser dây gấp `task_list` thành `plan`, `findings`/`answer` thành `solution`; loại gốc nằm ở `rawSubjectType` (handoff 4.2). Tính năng mới không được dựa vào loại đã gấp.
3. `NextActionBar` dùng token sẵn có (`text-destructive`, `text-primary`, `text-muted-foreground`, `--status-success`), không đặt màu mới; chú thích "Why:" ngắn là quy ước.
4. i18n có từng test bao phủ locale theo feature (`request-locale-coverage.test.ts`, `review-shell-locale-coverage.test.ts`, …).
5. Người dùng đã xem mockup tương tác (thẻ kết luận, "Needs your decision", "Asked vs delivered", "System impact", "Release readiness", thanh quyết định). Mockup dùng dữ liệu mẫu; mọi con số và ADR trong đó là bịa.
6. **Chưa đọc**: `next-action-rules`, cách thanh trạng thái/cờ theo project được đọc ở frontend, các hook tab hiện có (`useRequestFlow`), `request-action-rules.ts`.

## 2. Giải pháp đề xuất

### 2.1 Điểm vào

- Tab `outcome-review` (nhãn **"Outcome review"**, README D18) nằm **ngay sau Execution**: Overview, Analysis, Plan, Execution, **Outcome review**, History, Related. Hiện khi cờ `outcome_review ≠ off` của project **và** Request đã có Plan (`awaiting_plan_approval`, `executing`, `completed`), **kể cả khi Execution chưa xong** (pha `design`, D17). `RequestDetailPane` chỉ thêm `TabsTrigger/TabsContent` và gọi `OutcomeReviewTab`; danh sách tab được tách sang `RequestDetailTabs.tsx` **trước** khi thêm để file không vượt ngưỡng. Tên khác "Review" để không nhầm với tab Review mức code của v7 trong tab group của workspace.
- Dòng thời gian: thêm hàng "Outcome review" sau Execution/Handover (hiện từ pha `design`, sang `delivered` khi Execution xong) (cả engine native và OpenSpec), trạng thái suy ra từ `review.state` (`none`, `draft`, `ready`, `stale`, `approved`); tách `request-stage-timeline-model.ts` thành hai file (mô hình luồng và hàng mở rộng) trước khi thêm.
- `NextActionBar`: khi Request `awaiting_plan_approval` hoặc `executing` mà chưa có review và cờ bật → gợi ý mềm "Review the design" (pha `design`); khi Request `completed` mà chưa có review `delivered` → gợi ý mềm "Generate after-code review"; khi review `ready` có `needsDecision` → `needs_you` đến tab `outcome-review`; khi `approved` không gợi ý gì thêm. Không chặn bất kỳ hành động nào khác.

### 2.2 Bố cục tab (từ trên xuống)

| Khối | Nội dung | Nguồn |
|---|---|---|
| Thanh đầu `OutcomeReviewHeader` | tên Request, loại/engine, `base → head`, chip `KG snapshot <id>` hoặc "KG unavailable", trạng thái review, thời điểm sinh, nút "Regenerate…" (CR-OR-012) | mô hình |
| **Kiểm soát kiến trúc và luồng** `OutcomeDesignControl` (đầu tab, trước thẻ kết luận khác; D13/D14) | Hai thẻ lớn: **Architecture** (containers changed, new/removed edges, **unplanned edges**) và **Flow** (new/removed/reordered calls, **unplanned calls**), mỗi thẻ có nhãn "Agent reported", trạng thái đối chiếu thiết kế ("Checked against design (snapshot …)" hoặc "Design not available"), và nút nhảy tới tab con tương ứng; khi `designChecked=false` các số "unplanned" hiển thị Unknown, không phải 0 | `verdict.architecture`, `verdict.flow` |
| Thẻ kết luận `OutcomeVerdictCards` | Requirements (`delivered/total`, chi tiết partial, not covered, unknown), Architecture ("n reported deviations", nhãn "Agent reported"), Quality gate (`pass|warn|fail|unknown|absent`), Release risk (`low…critical|unknown` + cờ `migration`, `breaking`, `config`) | `verdict` |
| `OutcomeNeedsDecision` | danh sách mục `needsDecision` kèm hành động (nội dung ở CR-OR-022) | `items` |
| Các tab diff `OutcomeSystemDiff` | CR-OR-021 | `items`, `diagrams` |
| `OutcomeReleaseReadiness` | cờ phát hành, migration, bàn giao (OpenSpec), cấu hình mặc định, thứ tự khởi động lại nếu có trong dữ kiện; các ô chưa xác định hiển thị "Unknown", không tích sẵn | tầng F |
| `OutcomeAgentRunFacts` | số task xong, thời gian, chi phí, số lần thử lại, các lần dừng | tầng F |

Chỉ thẻ kết luận, "Needs your decision" và thanh quyết định (CR-OR-022) nằm trên màn đầu; các khối còn lại cuộn xuống hoặc nằm trong tab diff.

### 2.3 Trạng thái

| Trạng thái | Hiển thị |
|---|---|
| Pha `design` (Execution chưa xong) | Dải "Design review. The after-code view appears when execution completes."; thẻ Architecture/Flow hiện số liệu **thiết kế** (containers in scope, design edges, design flows), không có số "not in design"; các tab con chỉ có góc nhìn Design; **không có** thanh quyết định, Approve hay "Needs your decision" (hiện dòng "Decisions open after execution completes") |
| Execution xong, chưa có review `delivered` | Dải "Execution finished. Generate the after-code review." + Estimate (phần Design đã có nên chỉ cần đọc code; hình Design tái dùng) + nút chính "Generate after-code review" |
| Chưa sinh | Khối mời ("Generate review") + **Estimate**: có dùng cache không, có cần context run, có sơ đồ nào phải vẽ; nút chính "Generate review"; ghi chú chi phí "chưa có số liệu" khi chưa đo |
| Đang sinh | Tầng F hiện ngay; các khối chờ context/hình có khung xương và dòng trạng thái trong vùng `aria-live=polite` (cùng cách `NextActionBar`); tiến độ từ push `request.review.progress` |
| Sẵn sàng | Như 2.2 |
| Cũ (`stale`) | Dải "This review was made before the Request changed" + nút "Regenerate…"; nội dung cũ vẫn xem được |
| Thiết kế chưa duyệt (`reviewedRatio` thấp, AM-3) | Dải "Design not reviewed: n of m design elements are approved. Findings based on them are advisory."; mục lệch dựa trên phần tử chưa duyệt hiển thị nhãn "Advisory" và **không** vào "Needs your decision" |
| KG vắng | Dải thông tin "Design knowledge was not available, so architecture checks are limited", liệt kê lý do (`kg_unavailable`) và cách bật; thẻ Architecture hiện "Unknown" |
| Một phần lỗi | Mỗi khối lỗi tự giải thích (ví dụ "Drawing failed, showing the table"), có thử lại |
| Lỗi toàn bộ | Mã `REQUEST_REVIEW_*` → câu hướng dẫn bằng i18n, không in chuỗi lỗi thô |
| Cờ tắt / thiếu quyền | Tab ẩn / thông báo quyền, không gọi RPC |

### 2.4 Chữ hiển thị (không overclaim)

| Dữ liệu | Hiển thị | Không dùng |
|---|---|---|
| Mục `origin=agent` | "Agent reported" + nhãn "Unverified" nếu thiếu bằng chứng | "Confirmed", "Verified" |
| `requirements.delivered = total` | "All acceptance criteria have passing checks" | "Requirements met", "Ready to ship" |
| `gate = unknown` | "Not enough data to conclude" | "Passed" |
| `risk = unknown` | "Risk could not be measured" | "Low" |
| Review `approved` | "Review approved by <name>" | "Safe to release" |

### 2.5 Dữ liệu và hook

`hooks/useOutcomeReview.ts`: lấy `GetOutcomeReview` theo `requestId`, `Estimate` khi chưa có, theo dõi push `request.review.changed|progress`, `refresh`, `generate`. Parser dây ở `shared/outcome-review-wire-parsers.ts` **không gấp loại** và chịu enum lạ (giữ giá trị thô, hiển thị "Unknown"). Chốt tên kênh gateway cuối cùng ở CR này theo README 3.5. Mọi gọi qua lớp gateway giống `useExecutionSummary` để chạy được ở Electron lẫn web.

### 2.6 Cấu trúc file (mới, `components/request/outcome-review/`)

`OutcomeReviewTab.tsx`, `OutcomeReviewHeader.tsx`, `OutcomeVerdictCards.tsx`, `OutcomeReleaseReadiness.tsx`, `OutcomeAgentRunFacts.tsx`, `OutcomeReviewStates.tsx`, `outcome-review-view-model.ts` (hàm thuần: mô hình → hiển thị), `outcome-review-copy.ts` (khoá chữ theo 2.4), cùng các test. Không `helpers/utils`; không `max-lines` disable; mỗi file ≤ ngưỡng dòng.

## 3. Quyết định thiết kế

1. **Trên cùng là kết luận và việc cần quyết**, chi tiết dưới: người duyệt thấy trong 5 giây.
2. **Ba mức chữ cho dữ liệu**: xác định, do agent báo, chưa xác minh.
3. **Hàm thuần cho mô hình hiển thị**: kiểm thử không cần DOM.
4. **Tách file trước khi thêm**: tránh bị hook chặn khi commit.
5. **Không đổi `request-wire-parsers.ts`** gấp loại; parser riêng.

## 4. Tiêu chí chấp nhận

- [ ] Cờ tắt: tab không hiện và không có lưu lượng `request.review.*`.
- [ ] Tab nằm ngay sau Execution (đúng thứ tự trên); ở `awaiting_plan_approval`/`executing` chỉ có phần Design, không có Approve; khi Execution xong có nút tạo review `delivered` và xuất hiện phần After code.
- [ ] Request `completed`, chưa sinh: thấy Estimate (cache hay không, có cần context run) **trước** khi bấm; bấm "Generate review" tạo review và hiện tầng F ngay.
- [ ] Mở lại tab không phát sinh lời gọi sinh/AI nào (chỉ `Get`).
- [ ] `OutcomeDesignControl` nằm trên cùng, hiển thị Architecture và Flow với số `unplanned` đúng; `designChecked=false` → Unknown, không phải 0; bấm thẻ nhảy tới đúng tab con.
- [ ] Thẻ Requirements khớp số liệu mô hình; AC không có dữ liệu hiển thị "Unknown", không bao giờ "Delivered".
- [ ] Mục `origin=agent` luôn có nhãn "Agent reported"; thiếu bằng chứng có nhãn "Unverified".
- [ ] Thiết kế chưa duyệt: dải "Design not reviewed" kèm tỉ lệ; mục `advisory` không nằm trong "Needs your decision" và không chặn Approve.
- [ ] KG vắng: thẻ Architecture "Unknown", dải giải thích, không có id KG nào hiển thị.
- [ ] `stale`: dải cảnh báo và nút sinh lại; nội dung cũ còn xem được.
- [ ] Mã lỗi `REQUEST_REVIEW_*` hiển thị qua i18n; không in chuỗi lỗi thô.
- [ ] Đủ khoá i18n 5 locale; test bao phủ locale mới đạt.
- [ ] Hoạt động ở Electron và web; điều hướng bằng bàn phím; vùng `aria-live` thông báo tiến độ; tab và bảng có nhãn truy cập.
- [ ] `RequestDetailPane.tsx` và `request-stage-timeline-model.ts` đều ≤ ngưỡng dòng sau thay đổi.

## 5. Kiểm thử

- **Unit**: view model (mọi tổ hợp trạng thái, enum lạ, thiếu dữ liệu), `outcome-review-copy` (không chứa từ cấm), parser dây (không gấp loại).
- **Component (RTL)**: mỗi trạng thái 2.3; focus và `aria-live`; hai render target.
- **Locale**: test bao phủ 5 locale cho khoá mới.
- **Luồng UI e2e**: bộ `ui-live` mới với agent giả (CR-OR-030); lưu ý quy tắc "quiet-checkout" của e2e (handoff mục 3.1).

## 6. Rủi ro và điểm chưa kiểm chứng

- Chưa đọc `next-action-rules`; cách thêm gợi ý mới có thể cần đổi kiểu `NextAction`.
- Cờ `outcome_review` theo project phải đến được frontend (qua `request.engineSettings*` hay kênh khác) — chưa xác minh, và màn hình cài đặt engine chưa có (handoff 6.4).
- Phụ thuộc vào kết luận của người dùng: mockup là dữ liệu mẫu; mọi nhãn cuối cùng phải chốt cùng STYLEGUIDE.
- Bản dịch es/ja/ko/zh là việc còn nợ của đợt trước (192 khoá); khoá mới nên có bản dịch ngay hoặc dùng cơ chế rơi về tiếng Anh đang có.

## 7. Câu hỏi mở

- **Q1.** Pha `design` có nên mở sớm hơn (từ `awaiting_analysis_approval`, khi mới có Solution) để review kiến trúc trước khi lập Plan không; mặc định: từ `awaiting_plan_approval`.
- **Q2.** Review nên nằm trong tab hay như bước của dòng thời gian (mở panel); mặc định: tab, dòng thời gian chỉ là lối vào.

## 8. Tham chiếu

- [README v9](../README.md) D1, D2, D11, mục 3.2–3.6
- `frontend/src/renderer/src/components/request/RequestDetailPane.tsx`, `NextActionBar.tsx`, `request-stage-timeline-model.ts`, `execution/`
- `frontend/src/renderer/src/hooks/useExecutionSummary.ts`, `frontend/src/shared/request-wire-parsers.ts`
- [handoff 2026-10-17](../../../handoff/2026-10-17-request-flow-and-qa-fixes.md) mục 2, 4.2
- [STYLEGUIDE](../../../../guides/STYLEGUIDE.md), [AGENTS.md](../../../../AGENTS.md)

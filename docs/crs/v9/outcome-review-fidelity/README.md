# Change Requests v9 — Độ tin cậy của hình vẽ trong Outcome Review (kiến trúc, luồng, trước/sau code)

> **Mục tiêu:** người duyệt mở Outcome Review của **bất kỳ Request nào** và thấy đúng ba thứ: (1) **thiết kế nào** đang được dùng làm chuẩn (trước code), (2) **những flow và container nào bị Request chạm tới** đã được vẽ hết hay còn thiếu cái nào (sau code), (3) khi một bước hỏng thì màn hình **nói rõ hỏng ở đâu** thay vì trống hoặc so với một chuẩn mỏng. Series này sửa phần *hình vẽ và so sánh với thiết kế*, không đổi quyết định D13–D17 của series v9 (xem `../README.md`).
>
> **Trạng thái series:** 📝 Đề xuất, chưa triển khai (soạn 2026-10-21 sau lần nghiệm thu Request #2318, BL-E05). Mọi chỗ ghi "chưa kiểm chứng" là điều cần xác nhận ở spike đầu pha tương ứng.
>
> **Bổ sung cho:** CR-OR-002 (context run, thiết kế khoá), CR-OR-010/011/012 (vẽ, kho artifact, sinh lại), CR-OR-021 (tab diff và trình xem). Không thay thế chúng.
>
> **Phạm vi:** `backend-go/services/request-service` (mô hình thiết kế, hợp đồng context, kiểm kết quả, mô hình sơ đồ), `api-gateway` (trường mới của kênh `request.review.*`), `prompts/outcome_review_context`, `frontend/src/renderer/src/components/request/outcome-review`, bộ kiểm e2e.

## 1. Hiện trạng đã xác nhận (Request #2318, 2026-10-21)

Số liệu lấy trực tiếp từ DB `request` của stack local, từ code và từ SAD của `vnp-qa-platform`.

| # | Quan sát | Bằng chứng |
|---|---|---|
| E1 | **Thiết kế trôi giữa các lần trích.** Lần 05:41: 3 flow thiết kế (8.1, nhập Sheet FR-NTF-05, Chat FR-NTF-03). Lần 06:43: 17 container, 40 cạnh, 1 flow (SAD 8.5, 2 tin nhắn). Lần 07:14 (sau khi người dùng regenerate): **5 container, 3 cạnh, 1 flow** (8.1, 2 tin nhắn). | bảng `request.outcome_review_contexts`, `kind = 'design'` |
| E2 | **SAD thật có rất ít sơ đồ tuần tự.** `docs/SAD.md` 80.214 byte, 10 khối mermaid, **chỉ 3 `sequenceDiagram`** (6.1 đăng nhập, 8.1 ghi nghiệp vụ, 8.2 chạy kế hoạch). **Hai flow của #2318 (Chat FR-NTF-03, nhập Sheet FR-NTF-05) không có sơ đồ nào**: chúng chỉ tồn tại dạng chữ trong SAD/SRS, nên chỉ agent mới trích được. | `awk` trên `SAD.md`; `SRS.md` dòng 563–565 |
| E3 | **Cơ sở thiết kế do agent chọn.** `mergeFlows` chỉ parse các khối mermaid mà agent "đã dùng" và chép vào `sourceBlocks`; flow dạng chữ do agent tự viết. Không có bước nào đếm xem SAD có bao nhiêu khối so với số agent trả. | `domain/outcome_review_design.go:626`, `prompts/outcome_review_context/_contract.tmpl` |
| E4 | **Regenerate "đọc lại" thay luôn thiết kế đã khoá.** `Force` bỏ qua cache và lock, `keepDesign` lưu bản mới. Đây là nguyên nhân E1 (lần 07:14). | `usecase/outcome_review_context_run.go:63,131,474` |
| E5 | **Hình kiến trúc thiết kế không theo Request.** Bản 06:45 chỉ có 5 container có tên và một hộp "+6 more". `capDiagram` giữ nút "đáng xem xét" trước, rồi theo **thứ tự ID**; ở biến thể design mọi nút đều `same` nên giữ theo ID. | `domain/outcome_review_diagram_model.go:447`; mô tả hình trong `review_artifact_blobs` |
| E6 | **Flow chưa được phủ.** #2318 chạm 2 flow, chỉ 1 flow delta được vẽ (Chat). Flow nhập Sheet chỉ có ở dạng chữ trong mục contracts, behavior, data. Không có nơi nào nói "đã vẽ 1/2". | bảng `outcome_reviews.model` (diagrams, items) |
| E7 | **Design và After code không ghép theo flow.** `DiagramColumn` chia hai cột theo `variant`; flow thiết kế tên "Flow 8.1", flow delta tên "Bug resolved…" nên không nối được. | `components/request/outcome-review/diff/OutcomeSystemDiff.tsx` |
| E8 | **So sánh với thiết kế theo cặp `from→to` trên mọi flow** (`HasCall`), và không nói cơ sở thiết kế yếu. Với thiết kế 5 container, verdict flow ra `unplannedCalls = 4`, `reportedDeviations = 4`; cạnh "ms-integration → Google Chat" bị `deviation` dù thiết kế 06:43 có cạnh đó. | `domain/outcome_review_design.go:124`, `model->verdict` |
| E9 | **Trần im lặng.** `MaxContextDiagrams = 16` (vượt thì **từ chối cả kết quả**), 4 hình mỗi lô vẽ (`MaxDrawingsPerRun`), tối đa 12 nút và 6 lane mỗi hình, phần dư thành "+N more". | `domain/outcome_review_context.go:37`, `review_artifact.go:14` |
| E10 | **Kết quả agent mong manh và đắt.** Review sau code của #2318 tốn **7,59 USD** gồm 6 run; hai lần bị từ chối `REQUEST_REVIEW_RESULT_INVALID` đốt **3,51 USD** (1,4768 + 2,0352), chưa kể một run bị kill không có số. Lý do: `expected` là chuỗi thay vì boolean; 4 container chỉ có trong code bị đưa vào `design.containers`. Một lỗi cục bộ làm hỏng cả kết quả; chỉ thử lại đúng một lần. Trước 2026-10-21 lý do từ chối **không được log hay lưu**. | bảng `outcome_review_runs`; log `agent result rejected` (commit `3d8e998a8`) |
| E11 | **Hỏng một bước thì không còn thiết kế để xem.** Context sau code lỗi ⇒ review chỉ còn số liệu Orca đo, Architecture/Flow trống, "KG unavailable"; từng cho Approve (đã chặn ở `c2e5052ad`, thông báo tab trống ở `64c258d29`). Phần còn thiếu: vẫn hiện thiết kế đã khoá kèm nhãn "chưa so với code". | ảnh chụp 99-at-failure, `outcome_review_decide.go` |
| E12 | **Huy hiệu số liệu có thể lệch hàng.** Ảnh chụp 06:5x: tab hiện "0 not in design" nhưng hàng đầu tiên mang nhãn "Not in design". Dữ liệu hiện tại là `unplannedEdges = 1`, `unplannedCalls = 4`; **chưa kiểm lại trên UI** sau khi sinh lại. | `diff/outcome-diff-view-model.ts:229`, ảnh chụp |
| E13 | **Chưa có kiểm tự động cho "có hình".** Không có test nào khẳng định Architecture/Flow có hình sau khi review sinh ra (đã thêm bước kiểm ở `5ea95a196`, chưa phủ nhiều flow). | `tests/stages/e2e/orca-flows/ui-live` |

## 2. Từ triệu chứng đến nguyên nhân

| Triệu chứng người dùng thấy | Nguyên nhân | Giải quyết ở |
|---|---|---|
| Architecture/Flow "không có gì", rồi có nhưng thiếu flow | E1, E4, E10, E11 | CR-OR-040, CR-OR-042 |
| Hình kiến trúc chỉ vài hộp rồi "+6 more" | E5 | CR-OR-040 |
| Không biết còn flow nào chưa vẽ | E6, E7, E9 | CR-OR-041 |
| Cạnh đúng bị gắn "deviation" | E1, E8 | CR-OR-041 |
| Tốn tiền cho lần chạy bị từ chối | E10 | CR-OR-042 |
| Số trên tab không khớp hàng | E12 | CR-OR-041 (UI) |

## 3. Quyết định

| # | Quyết định | Lý do / hệ quả |
|---|---|---|
| F1 | **Thiết kế là dữ liệu có phiên bản và đơn điệu**, không phải đầu ra làm lại mỗi lần. Khoá theo `(snapshot, docSha256, promptVersion, schema)`; thay thế chỉ khi bản mới **không thu hẹp**, hoặc người dùng xác nhận rõ. | Sửa E1, E4. Tránh lặp lại 17 → 5 container. |
| F2 | **Nguồn flow thiết kế theo thứ tự tin cậy:** (1) khối `sequenceDiagram` Orca tự parse từ tài liệu; (2) flow dạng chữ do agent trích, **bắt buộc có `flowRef` (mục SAD hoặc mã FR) và trích dẫn dòng**; (3) không có thì ghi "chưa có trong thiết kế", **không** coi cạnh sau code là `deviation`. | E2: hai flow của #2318 chỉ có dạng chữ nên không thể chỉ dựa vào parser. |
| F3 | **Orca đọc tài liệu thiết kế trực tiếp** (mở rộng cổng file hiện chỉ có `Stat`) để đếm khối mermaid và so với `sourceBlocks` agent trả; thiếu thì Orca tự bổ sung. | Sửa E3. SAD 80 KB nằm trong giới hạn đọc 256 KiB. |
| F4 | **Hình kiến trúc thiết kế vẽ "lát cắt của Request"** (container chạm tới + hàng xóm một bước), không vẽ theo thứ tự ID; phần còn lại có drilldown. | Sửa E5. |
| F5 | **Mỗi flow ứng viên phải được trả lời:** có hình delta, hoặc `unchanged`, `not_applicable`, `undrawable` kèm lý do và bằng chứng. | Sửa E6. Không còn trường hợp flow bị bỏ rơi im lặng. |
| F6 | **Ghép Design | After code theo `flowRef`**, không theo cột `variant`. | Sửa E7. |
| F7 | **So sánh cạnh với đúng flow đã ghép**; khi cơ sở thiết kế yếu (thu hẹp, thiếu flow, KG không duyệt) thì hạ nhãn xuống `advisory` và nói lý do. | Sửa E8. |
| F8 | **Chấp nhận một phần:** lỗi cục bộ (ép kiểu `expected`, container nguồn code trong thiết kế) thì **sửa hoặc bỏ phần đó kèm cảnh báo**; chỉ từ chối cả kết quả khi lỗi nặng (không phải JSON, sai schema, bằng chứng bịa). Thử lại có ngân sách tiền. | Sửa E10. |
| F9 | **Mọi trần phải nói ra** ("còn N chưa hiển thị") và cắt theo ưu tiên phủ, không từ chối cả kết quả chỉ vì vượt 16 hình. | Sửa E9. |
| F10 | **Trạng thái suy giảm có ích:** context hỏng thì vẫn hiện thiết kế đã khoá và số liệu Orca, gắn nhãn "chưa so với code"; vẫn không cho Approve. | Sửa E11. |
| F11 | **Lưu lý do từ chối và chi phí từng run** (đã khử bí mật), hiện trong khung "Lượt chạy của agent". | Sửa E10: chẩn đoán không cần chạy lại tốn tiền. |

## 4. Phân kỳ

| Pha | Nội dung | Phụ thuộc | Ước lượng |
|---|---|---|---|
| **6** (BE) | CR-OR-040: cơ sở thiết kế đơn điệu, tách phạm vi regenerate, Orca đọc SAD, `flowRef`, lát cắt kiến trúc | spike BE-OR-T601 | trung bình |
| **7** (BE) | CR-OR-041 + 042: flow ứng viên, hợp đồng trả lời, ghép theo `flowRef`, chấp nhận một phần, lưu lý do lỗi, suy giảm | pha 6 | lớn |
| **4** (FE) | huy hiệu cơ sở thiết kế, thanh phủ flow, thẻ ghép Design/After code, trạng thái suy giảm, i18n, e2e thật | pha 6 (trường mới), pha 7 | lớn |

Thứ tự khuyến nghị: **6 → 7 → 4**. Riêng T604 (tách phạm vi regenerate) và T603 (không thu hẹp) có thể tách ra làm trước vì sửa trực tiếp gốc của #2318.

## 5. Điểm mở cần xác nhận

| # | Điểm mở | Đề xuất mặc định |
|---|---|---|
| O1 | Quy tắc **flow ứng viên** | Hợp của (flow có tin nhắn chạm container có code đổi) và (flow nối với FR/AC của Request trong KG). Có thể chỉ theo code đổi nếu KG không nối được (chưa kiểm chứng API KG cho quan hệ FR → flow). |
| O2 | Trần số flow delta mỗi lần | 8 (cấu hình `REQUEST_REVIEW_MAX_FLOW_DIAGRAMS`); phần dư hiện "còn N". |
| O3 | Engine vẽ mặc định và chi phí | Hiện `REQUEST_REVIEW_DRAW_ENGINE=auto`. Run vẽ của #2318 tốn 0,52–0,92 USD cho 4–5 hình. Spike BE-OR-T713 đo chênh lệch trước khi đổi mặc định; **chưa quyết**. |
| O4 | Nâng phiên bản schema khoá thiết kế | Chấp nhận trích lại một lần cho Request đang có (khoảng 1,4 USD mỗi Request) hay phải đọc được lock cũ? Đề xuất: đọc được lock cũ, chỉ ghi lock mới ở định dạng mới. |
| O5 | Đường dẫn tài liệu thiết kế | Lấy từ khoá của `docSha256` mà agent khai; chưa kiểm chứng với project có SAD không ở `docs/SAD.md`. |

## 6. Cách kiểm chứng toàn series

- **Đơn vị:** các bảng ca trong từng tasks file; fixture lấy từ SAD thật của `vnp-qa-platform` (3 sequence, 10 mermaid) và từ ba lần trích thiết kế của #2318.
- **UI:** vitest cho thanh phủ, thẻ ghép, trạng thái suy giảm; kiểm huy hiệu bằng số hàng.
- **E2E thật trên BL-E05** (chi phí dự kiến 2–4 USD mỗi lần): tab Flow báo **đã vẽ 2/2 flow** (Chat FR-NTF-03 và nhập Sheet FR-NTF-05); regenerate phạm vi `code` giữ nguyên digest lock và **0 lời gọi `kg_*`**; thiết kế sau regenerate không nhỏ hơn trước; kết quả bị từ chối không còn đốt tiền hai lần vì lỗi cục bộ.
- **Tiêu chí hoàn tất:** mỗi dòng E1–E13 có một test hoặc một bước e2e trỏ tới.

## 7. Danh sách file

| File | Nội dung |
|---|---|
| `CR-OR-040-design-basis-determinism-and-scope.md` | Cơ sở thiết kế trước code |
| `CR-OR-041-flow-coverage-and-comparison.md` | Phủ flow, ghép, so sánh sau code |
| `CR-OR-042-result-resilience-and-degraded-states.md` | Kết quả agent, chi phí, trạng thái suy giảm |
| `specs/backend-go/crs/v9/outcome-review/solutions/BE-OR-SOL-005-design-basis-coverage-resilience.md` | Giải pháp backend |
| `specs/backend-go/crs/v9/outcome-review/tasks/phase-6-design-basis.md` | Checklist pha 6 |
| `specs/backend-go/crs/v9/outcome-review/tasks/phase-7-coverage-and-resilience.md` | Checklist pha 7 |
| `specs/frontend/crs/v9/outcome-review/solutions/FE-OR-SOL-004-coverage-and-pairing.md` | Giải pháp frontend |
| `specs/frontend/crs/v9/outcome-review/tasks/phase-4-coverage-ui-and-e2e.md` | Checklist pha 4 |

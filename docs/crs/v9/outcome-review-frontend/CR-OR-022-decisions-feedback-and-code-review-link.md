# CR-OR-022 — Quyết định, "gửi lại agent" bằng Request con và nối sang review code (v7)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-OR-022 |
| **Tên** | RPC `DecideOutcomeReviewItem`; quyết định `accept_exception|send_back|dismiss`; thanh quyết định (Approve / Request changes); server chặn duyệt khi còn mục cần quyết định; "Request changes" tạo **Request con** có ngữ cảnh; liên kết "Open in code review" tới lens của v7; khoá `item_key` bền qua lần sinh lại |
| **Loại** | Feature |
| **Priority** | 🔴 P0 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-20 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-OR-001 (mô hình, `item_key`), CR-OR-020 (khung), CR-OR-021 (các mục nằm ở tab con), CR-OR-012 (quy tắc quyết định mồ côi); v7 `openReviewFromEntryPoint` (tuỳ chọn) |
| **Mở khoá** | Cổng chặn tuỳ chọn ở CR sau |
| **Tác động** | `request-service` (`usecase/outcome_review_decide.go`, `domain/outcome_review_decision.go`, bảng `outcome_review_decisions`), `proto/orca/request/v1/outcome_review.proto`, `api-gateway` (`request.review.decide`), frontend `components/request/outcome-review/decision/` (mới), `components/review-map/entry/open-review-entry.ts` (thêm nguồn), `components/request/SpawnChildRequestDialog.tsx` (nhận dữ liệu điền sẵn) |

---

## 1. Bối cảnh và vấn đề

1. Người duyệt phải ra quyết định cho những thứ máy không tự giải được: ví dụ một cạnh lệch thiết kế (ADR), một thay đổi phá vỡ tương thích, một yêu cầu có dấu hiệu hồi quy. Mockup 2026-10-20 mô tả: "Accept as exception" / "Send back to agent"; **Approve** bị chặn khi còn quyết định mở, hiện lỗi cạnh nút.
2. Request `completed` là trạng thái cuối (`request_status.go`); **không mở lại** được để chạy tiếp. Mẫu hiện có để "làm tiếp" là Request con: `SpawnChildRequestDialog.tsx`, `request_child_rules.go`, `request_link.go`. **Chưa đọc** quy tắc cho phép tạo Request con từ Request đã `completed` (rủi ro mục 6).
3. v7 có lối vào chuẩn tới review code theo **worktree**: `openReviewFromEntryPoint(worktreeId, source, { lens, filter })`, no-op khi `codeIntelSupportState.state !== 'enabled'`; id lens đã có: `impact`, `architecture`, `dataflow`, `erd`, `storage`, `structure`, `contract` (và `requirements` theo thư mục `requirements/`, id chưa xác minh). `ReviewEntrySource` hiện gồm `agent-row|source-control|cmd-k|right-sidebar|notification` và ánh xạ sang enum telemetry (bản sao ở nhiều nơi, theo ghi chú CR-CV-095).
4. Request → worktree: task chạy trên worktree `<repo>-<nhánh>` riêng (handoff mục 3.7); giao diện Request hiện không có sẵn `worktreeId` (đã tìm ở `execution/` và `RequestDetailPane`: không thấy).
5. Quyết định phải sống sót khi sinh lại (CR-OR-012 2.4): khoá theo `item_key` ổn định.
6. Ghi chú gửi agent ở v7 (CR-CV-060) gửi vào phiên agent của worktree; khác bản chất với "gửi lại" ở mức Request.

## 2. Giải pháp đề xuất

### 2.1 Mô hình quyết định

**Chỉ ở pha `delivered`.** Ở pha `design` (Execution chưa xong) không có `needsDecision`, quyết định hay duyệt; các RPC trả `REQUEST_REVIEW_NOT_READY` và giao diện không hiện thanh quyết định (README D17).

`outcome_review_decisions` (README 3.4): `item_key`, `decision ∈ {accept_exception, send_back, dismiss}`, `note` (≤ 500, **bắt buộc** với `accept_exception` và `dismiss`), `decided_by`, `decided_at`; một dòng hiệu lực mỗi `(review, item_key)` (đổi ý ghi dòng mới, dòng cũ giữ làm lịch sử). `DecideOutcomeReviewItem(review_id, item_key, decision, note?, expected_version)` kiểm quyền, khoá lạc quan trên `outcome_reviews.version`, audit `request.review.decide` (không ghi `note`), sự kiện `orca.request.review.decided`.

Quy tắc: `accept_exception` chỉ cho mục `needsDecision`; **chấp nhận ngoại lệ không sửa mô hình** (mục vẫn hiển thị kèm nhãn "Accepted by <name>"). `dismiss` chỉ cho mục `origin=agent` (người duyệt cho rằng claim không đúng) và không áp lên mục `origin=fact`. `send_back` cho mọi mục.

### 2.2 Duyệt review

RPC riêng `ApproveOutcomeReview(review_id, note?, expected_version)` (kênh `request.review.approve`; đã bổ sung vào README 3.5) kiểm **ở server**: còn mục `needsDecision` chưa có quyết định hiệu lực → `REQUEST_REVIEW_DECISIONS_OPEN` (mã mới, ghi vào "Điều chỉnh hợp đồng"); review `stale` → `REQUEST_REVIEW_STALE_REVISION` ("regenerate first"). Thành công: `state=approved`, ghi người/giờ/ghi chú, sự kiện, audit. **Không** đổi `RequestStatus`, **không** tạo `Approval` của v6 (Q1).

### 2.3 Request changes: tạo Request con

Nút "Request changes" mở `SendBackDialog`: liệt kê các mục `send_back` (tiêu đề, chi tiết, bằng chứng), cho chọn bớt/thêm, ghi chú chung, rồi dẫn tới `SpawnChildRequestDialog` **điền sẵn**: tiêu đề "Address review findings for <tiêu đề Request>", loại gợi ý (mặc định để người dùng chọn; gợi ý `bug` nếu chủ yếu là `regression`, ngược lại `task`), mô tả dựng từ các mục (mỗi mục một khối: mục, bằng chứng, id KG kèm `snapshot`), liên kết với Request gốc theo quan hệ hiện có (`request_link.go`). Mô tả đi qua bộ che secret trước khi gửi. Không có lời gọi AI trong bước này.

Sau khi tạo: các mục `send_back` được đánh dấu với `childRequestId` (hiển thị liên kết "Sent back in <REQ-n>"); review gốc giữ `ready` và quyết định của bạn vẫn hiệu lực. Khi Request con `completed`, tab hiện dải "A follow-up Request finished. Regenerate this review to check." (CR-OR-012), không tự sinh lại.

### 2.4 Nối sang review code (v7)

Mỗi cạnh/tin nhắn lệch thiết kế (`deviation`, `missing`) và mỗi cạnh container mới mặc định là mục `needsDecision` (D14): người duyệt phải chấp nhận ngoại lệ (kèm ghi chú, nên nêu ADR) hoặc trả lại. Huy hiệu "n components changed" trên container mở lens `architecture` của v7 lọc theo container (`drilldown.containerId`; khả năng lọc theo container của lens chưa xác minh). Mỗi hàng diff có nút "Open in code review" (nếu có `worktreeId` và v7 bật), ánh xạ: Architecture → `architecture`, Flow → `dataflow`, Data → `erd`, Contracts → `contract`, Requirements → `requirements`, Config → (không có), mục có `file` → `impact`. Gọi qua `openReviewFromEntryPoint(worktreeId, 'request-outcome', { lens })`; thêm giá trị `'request-outcome'` vào `ReviewEntrySource` và ánh xạ telemetry (đụng enum telemetry: nhiều bản sao, Q2). Không có `worktreeId` hoặc v7 tắt → nút bị vô hiệu kèm giải thích ("Code review is not available for this Request"), **không** ẩn lặng lẽ. Nguồn `worktreeId`: danh sách `subject.heads[].taskId` → worktree của task (qua dữ liệu task đã có); nếu nhiều, hiển thị chọn.

### 2.5 Thanh quyết định (UI)

`OutcomeDecisionBar`: ô ghi chú, **Approve** (nút chính duy nhất), "Request changes" (phụ). Bấm Approve khi còn mục mở: lỗi cạnh nút "Resolve N open decisions first" (không im lặng, không vô hiệu hoá nút) và cuộn tới mục đầu tiên; xử lý theo quy tắc "tránh nút bị vô hiệu hoá" của CDS. Mỗi mục `needsDecision` có `OutcomeDecisionCard` với hai nút nhỏ ("Accept as exception", "Send back") và ô ghi chú bắt buộc khi chấp nhận. Sau khi mọi thứ xong: dòng xác nhận "Review approved by <name>" (không "safe to release").

### 2.6 Quyền

Xem: người xem Request. Ra quyết định/duyệt: người có quyền duyệt Request trên project (tên hành động chốt sau khi đọc `approval_authorization.go`); người ngoài chỉ đọc. Một người không được duyệt review mà chính họ đã tạo Request? Chính sách này do tenant (Q3), không thi hành ở MVP.

### 2.7 Cấu trúc file

Backend: `domain/outcome_review_decision.go`, `usecase/outcome_review_decide.go`, `usecase/outcome_review_approve.go`. Frontend `components/request/outcome-review/decision/`: `OutcomeDecisionBar.tsx`, `OutcomeDecisionCard.tsx`, `OutcomeSendBackDialog.tsx`, `outcome-send-back-draft.ts` (hàm thuần dựng mô tả Request con), `use-outcome-decisions.ts`, `outcome-code-review-link.ts` (ánh xạ lens, hàm thuần).

## 3. Quyết định thiết kế

1. **Gửi lại = Request con**, không mở lại Request đã xong (D10).
2. **Duyệt không đổi trạng thái Request** ở MVP; gắn nhãn rõ ràng để tránh hiểu nhầm là cổng phát hành.
3. **Server thi hành quy tắc quyết định mở**, UI chỉ phản chiếu.
4. **`item_key` ổn định** để quyết định sống sót khi sinh lại.
5. **Dùng lối vào chuẩn của v7**, không tự mở tab Review của v7.

## 4. Tiêu chí chấp nhận

- [ ] Pha `design`: không có thanh quyết định; gọi `DecideOutcomeReviewItem`/`ApproveOutcomeReview` trực tiếp trả `REQUEST_REVIEW_NOT_READY`.
- [ ] Approve khi còn mục `needsDecision` chưa quyết định → `REQUEST_REVIEW_DECISIONS_OPEN` từ server (kể cả gọi API trực tiếp bỏ qua UI).
- [ ] `accept_exception`/`dismiss` thiếu `note` bị từ chối; `dismiss` không áp lên mục `origin=fact`.
- [ ] Hai người quyết định cùng lúc → người sau nhận lỗi phiên bản (`expected_version`).
- [ ] Review `stale` không duyệt được.
- [ ] Sinh lại ngữ cảnh: quyết định của `item_key` còn nguyên vẫn hiệu lực; quyết định của `item_key` mất thành `orphaned`; mục mới cần quyết định đưa review `approved` về `ready`.
- [ ] "Request changes" tạo Request con có mô tả đúng các mục đã chọn, qua bộ che secret, có liên kết tới Request gốc; Request gốc không đổi trạng thái.
- [ ] Nút "Open in code review" gọi `openReviewFromEntryPoint` với đúng `lens`; vô hiệu có giải thích khi thiếu worktree hoặc v7 tắt.
- [ ] Không có chữ "safe to release" hay tương tự ở bất kỳ locale nào (test từ cấm).
- [ ] Audit và sự kiện được ghi cho quyết định và duyệt; `note` không nằm trong audit.
- [ ] Hoạt động ở Electron và web; điều khiển bằng bàn phím.

## 5. Kiểm thử

- **Unit (Go)**: bảng quy tắc quyết định, mồ côi, phiên bản lạc quan, quyền.
- **Repository (hai dialect)**: dòng hiệu lực mỗi `(review, item_key)`, lịch sử.
- **Unit (TS)**: `outcome-send-back-draft` (mô tả, che secret, giới hạn độ dài), `outcome-code-review-link`.
- **Component (RTL)**: thanh quyết định (lỗi cạnh nút, cuộn tới mục), thẻ quyết định, hộp thoại gửi lại, hai render target.
- **Tích hợp**: gửi lại → Request con tồn tại, liên kết hai chiều; v7 tắt/bật cho nút mở review code.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Chưa kiểm chứng** `request_child_rules.go` có cho Request con từ Request `completed` không; nếu không, phải đổi cách (ví dụ liên kết `relates` thay vì con) hoặc nới quy tắc (đổi backend).
- Ánh xạ Request → worktree trên frontend chưa tồn tại; có thể cần RPC trả danh sách worktree của Request.
- Thêm `'request-outcome'` vào `ReviewEntrySource` chạm enum telemetry có nhiều bản sao (CR-CV-095 ghi chưa có script đồng bộ).
- Id lens `requirements` chưa xác minh; lens `Config` không có tương ứng.
- Người duyệt tự duyệt Request của chính mình: không chặn ở MVP.

## 7. Câu hỏi mở

- **Q1.** Dùng `Approval` của v6 (`subject_type=request`, `stage=outcome_review`) thay vì bảng riêng? Cần xem CHECK của CR-REQ-009; mặc định: bảng riêng.
- **Q2.** Cách tránh đụng enum telemetry: dùng nguồn sẵn có hay thêm giá trị mới.
- **Q3.** Chính sách "người tạo không được tự duyệt" theo tenant.
- **Q4.** Sau khi duyệt, có nên đề xuất ChangeSet cho KG (skill `tdd-knowledge-writeback`, người duyệt xác nhận) như bước tuỳ chọn? Ngoài MVP; Orca không áp thay đổi vào KG.

## 8. Tham chiếu

- [README v9](../README.md) D10, mục 3.4, 3.6
- `backend-go/services/request-service/internal/domain/request_child_rules.go`, `request_link.go`, `request_status.go`
- `frontend/src/renderer/src/components/request/SpawnChildRequestDialog.tsx`
- `frontend/src/renderer/src/components/review-map/entry/open-review-entry.ts`, `review-map/review-lens-registry.ts`
- [CR-CV-060](../../v7/review-frontend/CR-CV-060-review-notes-send-to-agent-and-turn-compare.md), [CR-CV-095](../../v7/quality-gate/CR-CV-095-review-quality-telemetry.md)
- [CR-OR-012](../review-visuals/CR-OR-012-regenerate-review-artifacts.md)

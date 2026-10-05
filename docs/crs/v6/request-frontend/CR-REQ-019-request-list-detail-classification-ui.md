# CR-REQ-019 — Danh sách, chi tiết Request, xác nhận phân loại và "Tạo Request" từ Tasks

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-019 |
| **Tên** | Danh sách và chi tiết Request, hộp xác nhận phân loại, lịch sử đổi loại, Request con, nút "Tạo Request" từ issue Jira/GitHub trên trang Tasks |
| **Loại** | Feature |
| **Priority** | 🔴 P0 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-REQ-018 (kiểu, hook, store, `RequestPage`); backend CR-REQ-004 (tiếp nhận), CR-REQ-005 (phân loại, xác nhận, đổi loại), CR-REQ-006 (backlog, mở lại, hủy, Request con); kênh CR-REQ-016 |
| **Mở khoá** | CR-REQ-020, 021, 022, 023 |
| **Tác động** | `frontend/src/renderer/src/components/request/` (mới), `components/TaskPage.tsx` và `task-page-jira-issue-list.tsx` (thêm nút), `components/GitHubItemDialog.tsx`, `i18n/locales/*.json` |

---

## 1. Bối cảnh và vấn đề

Người dùng hiện chỉ có "Start work" trên issue Jira/GitHub (`handleUseJiraItem`, `handleUseWorkItem` ở `TaskPage.tsx`): tạo worktree và agent ngay, bỏ qua phân tích. Series v6 thêm bước Request: nguồn → phân loại (AI đề xuất, người xác nhận, quyết định D5) → luồng theo loại. Người dùng cần:

- Một danh sách Request lọc được và một màn chi tiết cho biết Request đang ở đâu trong luồng của loại đó (README 3.3, 3.4).
- Một hộp xác nhận loại thể hiện đề xuất của AI (loại, độ tin cậy, lý do) và cho sửa.
- Xem lịch sử đổi loại và quan hệ Request cha/con.
- Tạo Request từ issue Jira/GitHub đang xem, tách biệt rõ với "Start work".

Hiện chưa có UI nào như vậy (README mục 1). `RequestPage` và hook do CR-REQ-018 cung cấp; CR này điền `RequestsTab` và chi tiết.

## 2. Giải pháp đề xuất

### 2.1 Cây component

```
RequestsTab
├─ RequestListToolbar        (tìm kiếm, bộ lọc, nút "Tạo Request")
│    ├─ RequestFilterBar     (trạng thái, loại, nguồn, người báo cáo, "chờ tôi")
│    └─ CreateRequestButton  -> CreateRequestDialog
├─ RequestList               (ui/table.tsx; hàng = RequestRow)
│    └─ RequestRow           (số, tiêu đề, RequestTypeBadge, RequestStatusBadge, RequestSourceBadge, urgency, cập nhật)
├─ RequestListEmptyState / RequestListErrorState / RequestListSkeleton
└─ RequestDetailPane         (khi requestPage.requestId != null; thay danh sách trên màn hẹp)
     ├─ RequestDetailHeader  (#số, tiêu đề, trạng thái, nút hành động)
     ├─ RequestStageTimeline (các bước theo REQUEST_FLOW_REGISTRY của loại)
     ├─ TypeConfirmationCard (khi status = awaiting_type_confirmation)
     ├─ RequestOverviewTab   (nội dung, nguồn, người báo cáo, size, urgency)
     ├─ RequestAnalysisTab   (CR-REQ-020 điền)
     ├─ RequestPlanTab       (CR-REQ-021 điền)
     ├─ RequestHistoryTab    (RequestTypeHistoryList)
     └─ RequestRelatedTab    (RequestLinksList: cha, con, theo dõi)
```

Bố cục: danh sách bên trái (cố định 40% với `ui/resizable.tsx`), chi tiết bên phải; cửa sổ hẹp thì chi tiết thay danh sách và có nút "Quay lại danh sách". Chi tiết mở bằng `setRequestPageRequest(id)` để chia sẻ được liên kết sâu.

### 2.2 Danh sách

- Nguồn dữ liệu: `useRequests(filters)` gọi `request.list`. Cập nhật tại chỗ khi có `request.status_changed`, `request.type_changed`, `request.created` (CR-REQ-018 2.5).
- Cột: `#số`, tiêu đề, loại, trạng thái, nguồn (`source_provider` + `source_ref`, link gốc), mức khẩn (`urgency=urgent` hiện chip "Khẩn"), người báo cáo, cập nhật. Sắp xếp mặc định: `updated_at` giảm dần.
- Bộ lọc nhanh "Chờ tôi xác nhận" = `status=awaiting_type_confirmation`; "Đang chạy" = `analyzing|planning|executing`.
- Trạng thái đang tải: `RequestListSkeleton` 8 hàng (`ui/skeleton.tsx`). Tải thêm: nút "Tải thêm" khi có `nextPageToken`.
- Trạng thái rỗng: không có Request nào thì hiện `emptyNone` kèm nút "Tạo Request"; có bộ lọc mà không khớp thì `emptyFiltered` kèm nút "Xoá bộ lọc".
- Lỗi: `network` thì banner có "Thử lại"; `forbidden` thì thông báo không có quyền xem Request của dự án; `unsupported` thì `RequestPage` đã thay toàn thân bằng `RequestUnsupportedNotice`, danh sách không render.

### 2.3 Chi tiết và dòng thời gian

`RequestStageTimeline` dựng từ `REQUEST_FLOW_REGISTRY[type]` (không hardcode theo loại): các bước `classification`, `analysis` (nhãn theo `analysisKind`: Solution, Chẩn đoán, Findings, Answer), `plan` (Plan hoặc Danh sách task), `phase` (nếu `phase` là `always`, hoặc `size_l` và `size=L`), `execution`. Bước hiện tại lấy từ `status`; bước bị loại bỏ (ví dụ `question` không có Plan) không vẽ. `hotfix`: hiện chú thích "chẩn đoán nhanh, không cổng".

Hành động ở header, theo trạng thái và quyền (ẩn nút nếu `viewerCan` có và là `false`; nếu backend không trả `viewerCan` thì hiện và dựa vào lỗi `forbidden`, xem CR-REQ-018 mục 7):

| Nút | Hiện khi | RPC | Ghi chú |
|---|---|---|---|
| Xác nhận loại / Sửa loại | `awaiting_type_confirmation`; sửa loại còn ở mọi trạng thái đang xử lý | `request.confirmType`, `request.changeType` | Xem 2.4 |
| Hủy Request | chưa `completed`/`cancelled` | `request.cancel` | Hộp xác nhận, lý do tuỳ chọn |
| Mở lại | `request_backlog` | `request.reopen` | Xem CR-REQ-023 để gom hàng loạt |
| Trả về backlog | các trạng thái đang xử lý | `request.returnToBacklog` | Bắt buộc nhập lý do (`return_reason`); chọn `returned_from_stage` mặc định theo giai đoạn hiện tại |
| Tạo Request con | `spike`, `question`, `hotfix` đã có kết quả | `request.spawnChild` | Chọn loại con theo README 3.4 (đường nâng cấp) |

`request_backlog` hiện banner có `returned_from_stage`, `return_reason`, người trả, thời điểm.

### 2.4 Hộp xác nhận phân loại (`TypeConfirmationCard`)

Hiện khi `status = awaiting_type_confirmation`. Nội dung:

- Loại AI đề xuất (`RequestTypeBadge`), `confidence` (thanh `ui/progress.tsx` kèm `%`), `classification_reason` (đoạn văn, tối đa 6 dòng, nút "Xem thêm"), `size` và `urgency` đề xuất.
- Nếu `confidence` thấp hơn ngưỡng cấu hình `LOW_CONFIDENCE_THRESHOLD = 0.6` (hằng ở `request-flow-registry.ts`, chưa kiểm chứng giá trị): hiện dòng "AI không chắc, hãy kiểm tra".
- Bộ chọn loại (`ui/select.tsx`, 11 loại, mỗi mục kèm mô tả một dòng và tóm tắt luồng từ registry: có Plan/Phase không, cổng nào). Chọn loại khác đề xuất thì bật ô "Lý do" (tuỳ chọn) và nhãn "Sửa bởi người".
- Nút chính "Xác nhận loại" (`variant=default`); nút phụ "Phân loại lại" gọi `request.classify`.
- Hành động ghi `type_source=human` khi người dùng đổi; backend tạo dòng `request_type_history`.
- `Mod+Enter` (dùng `isScreenSubmitShortcut`) xác nhận từ ô lý do. Nhãn phím qua `ShortcutKeyCombo`: `⌘` + `Enter` trên Mac, `Ctrl` + `Enter` nơi khác.
- Đang phân loại (`classifying`): hiện `Loader2` kèm chữ "AI đang phân loại"; hết hạn dài thì hiện "Phân loại lại".
- Lỗi `invalid_state` (đã có người xác nhận trước): toast, tải lại, hộp biến mất.

Đổi loại khi Request đã đi xa: hộp xác nhận `ChangeTypeConfirmDialog` giải thích "Solution, Plan, Task đã có được giữ làm tham chiếu, Request quay về bước xác nhận loại" (README 3.3).

### 2.5 Lịch sử đổi loại và Request con

- `RequestHistoryTab`: danh sách từ `request_type_history`: `from_type → to_type`, người (`actor_kind` `ai` hiển thị "AI"), lý do, thời điểm; mới nhất ở trên. Rỗng: "Chưa đổi loại lần nào".
- `RequestRelatedTab`: nhóm theo `RequestLink.reason` (`spawned_by_spike`, `spawned_by_question`, `followup_hotfix`, `escalation`); mỗi dòng là Request kèm trạng thái, bấm để mở. Rỗng: "Không có Request liên quan".

### 2.6 Nút "Tạo Request" trên trang Tasks (khác "Start work")

- Jira: thêm nút phụ cạnh `onStartWorkspace` trong `task-page-jira-issue-list.tsx` và trong chi tiết issue Jira (`JiraIssueWorkspace.tsx`). GitHub: thêm trong hàng/chi tiết issue (`GitHubItemDialog.tsx`); không áp dụng cho PR. GitLab, Linear: chưa ở phạm vi này (README yêu cầu Jira/GitHub), kiến trúc `source_provider` cho phép thêm sau.
- Nhãn và biểu tượng khác hẳn "Start work": nhãn "Tạo Request" (`variant=outline`, icon `GitPullRequestCreateArrow` hoặc `Inbox` của lucide), tooltip: "Gửi issue vào luồng phân tích. Khác với Start work, không mở worktree ngay".
- `CreateRequestDialog` (`ui/dialog.tsx`): tiêu đề, nội dung điền sẵn từ issue (Jira `title`, `description`, `url`; GitHub `title`, mô tả, `url`), chọn dự án Orca (mặc định theo ánh xạ Jira key/site hiện có, mẫu `useJiraProjectPreselect.ts`; GitHub theo repo), chọn mức khẩn, tuỳ chọn gợi ý loại ("Để AI phân loại" mặc định).
- Gọi `request.create` với `source_provider` (`jira|github`), `source_ref` (Jira `key`, GitHub `owner/repo#number`), `source_url`, `source_site` (Jira `siteId`, GitHub để trống). Idempotent theo `request_idempotency`: nếu đã có Request cho issue này, backend trả Request cũ; UI hiện toast "Issue này đã có Request" kèm nút "Mở" thay vì tạo trùng.
- Sau khi tạo: `openRequestPage({section:'requests', requestId})`; người dùng quay lại Tasks bằng điều hướng hiện có.
- Không có kênh `request.create` (`unsupported`) thì không render nút. Chưa chọn được dự án thì nút "Tạo" bị khoá và hiện lý do.
- Có thể tạo Request thủ công (không từ issue) từ `CreateRequestButton` ở `RequestPageHeader` với `source_provider=manual`.

### 2.7 Trạng thái, phím tắt, i18n

- Phím tắt trong danh sách (chỉ khi tiêu điểm ở danh sách, không ở ô nhập): `j`/`k` di chuyển, `Enter` mở, `Escape` đóng chi tiết. Không cần phím sửa đổi; ở ô nhập thì bỏ qua. Chip hiển thị bằng `ShortcutKeyCombo` trong tooltip của nút "Đóng".
- Khoá i18n (đủ 5 locale, đọc theo tên, tiền tố `auto.components.request.`): `RequestsTab.emptyNone`, `RequestsTab.emptyFiltered`, `RequestsTab.loadMore`, `CreateRequestButton.label`, `CreateRequestDialog.title`, `CreateRequestDialog.duplicate`, `TypeConfirmationCard.title`, `TypeConfirmationCard.lowConfidence`, `TypeConfirmationCard.confirm`, `TypeConfirmationCard.reclassify`, `TypeConfirmationCard.classifying`, `ChangeTypeConfirmDialog.body`, `RequestDetailHeader.cancel`, `RequestDetailHeader.returnToBacklog`, `RequestDetailHeader.reasonRequired`, `RequestHistoryTab.empty`, `RequestRelatedTab.empty`, `RequestStageTimeline.hotfixNote`, và mô tả 11 loại `RequestType.<type>.label|description`.

## 3. Quyết định thiết kế

- Chi tiết dùng chung một `RequestDetailPane` với tab, các CR sau chỉ cắm nội dung vào `RequestAnalysisTab`, `RequestPlanTab`.
- Dòng thời gian dựng từ registry, không rẽ nhánh `if (type === ...)`, để thêm loại không phải sửa UI.
- Sửa loại luôn qua hộp xác nhận khi Request đã qua `awaiting_type_confirmation`, vì mất ngữ cảnh luồng.
- "Tạo Request" khác "Start work" cả về nhãn, kiểu nút, và hậu quả; không gộp thành một menu để tránh bấm nhầm.
- Chống tạo trùng ở backend (idempotency), UI chỉ báo.

## 4. Tiêu chí chấp nhận

- [ ] Danh sách hiện đúng cột, lọc theo trạng thái, loại, nguồn; sắp xếp mặc định theo cập nhật mới nhất.
- [ ] Rỗng, đang tải, lỗi `network`, lỗi `forbidden` đều có UI riêng; `unsupported` không hiện danh sách.
- [ ] Request `awaiting_type_confirmation` hiện `TypeConfirmationCard` với loại AI đề xuất, confidence (%), lý do.
- [ ] Đổi sang loại khác đề xuất rồi xác nhận gửi `type_source=human`; xuất hiện một dòng mới ở lịch sử.
- [ ] Dòng thời gian của `question` không có bước Plan; của `bug` có Phase chỉ khi `size=L`.
- [ ] "Trả về backlog" không gửi được khi lý do rỗng.
- [ ] Nút "Tạo Request" có trên issue Jira và issue GitHub, không có trên PR, không thay thế "Start work" và không gọi `task.create`/worktree.
- [ ] Tạo Request từ issue đã có Request cũ không tạo bản mới, hiện toast và nút "Mở".
- [ ] Ẩn "Tạo Request" khi runtime không hỗ trợ `request.create`.
- [ ] `Mod+Enter` xác nhận ở hộp phân loại: `metaKey` trên Mac, `ctrlKey` nơi khác; nhãn chip khớp.
- [ ] Mọi chuỗi mới có đủ 5 locale.

## 5. Kiểm thử

- Unit: dựng bước dòng thời gian từ registry cho cả 11 loại (bảng test); lọc/sắp xếp danh sách; ánh xạ issue → `CreateRequestParams` (Jira, GitHub); `isLowConfidence`.
- Component: `TypeConfirmationCard` (đề xuất, sửa loại, ô lý do, phím tắt), `RequestStageTimeline`, `CreateRequestDialog` (điền sẵn, trùng lặp), `RequestRow`, hộp trả về backlog (lý do bắt buộc), nút mới trong `task-page-jira-issue-list` (bổ sung vào test hiện có).
- Hook: `useRequests` phân trang, cập nhật theo sự kiện.
- E2E (cần backend CR-REQ-004/005): tạo Request từ Jira, AI phân loại, người xác nhận, thấy chuyển sang `analyzing`; đổi loại giữa chừng.
- Chưa chạy; tất cả là kế hoạch.

## 6. Rủi ro và điểm chưa kiểm chứng

- `TaskPage.tsx` đã khoảng 8.280 dòng và nằm trong `config/max-lines-baseline.txt`; chỉ thêm nút qua component riêng, không thêm logic lớn vào file này, không thêm `max-lines` disable.
- Định dạng `source_ref` GitHub (`owner/repo#number`) và `source_site` chưa chốt trong README; phải khớp khoá idempotency của CR-REQ-004.
- Ngưỡng `LOW_CONFIDENCE_THRESHOLD` là đề xuất, chưa có dữ liệu.
- Ánh xạ issue Jira → dự án Orca dựa tính năng map Jira key/site đã có; chưa kiểm chứng với Jira thật (README mục 7).
- Chưa rõ `GetRequest` có trả sẵn lịch sử, Request con, cha hay cần RPC riêng.

## 7. Câu hỏi mở

1. `request.get` trả kèm `history`, `links` không, hay cần `request.listHistory`? README 3.6 không có RPC riêng.
2. Hỗ trợ cả `viewerCan` hay chỉ dựa `forbidden` (xem CR-REQ-018 mục 7)?
3. Có cho tạo Request từ GitLab, Linear trong v6 không? README chỉ nêu Jira/GitHub.
4. Ai được "Sửa loại" sau khi đã có Plan: chỉ người báo cáo, hay người có quyền duyệt? CR-REQ-010 chưa nêu.
5. `SpawnChildRequest`: loại con nào hợp lệ cho từng loại cha (README 3.4 mô tả đường nâng cấp nhưng không có bảng cho UI)?

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/README.md` (D5, 3.2 đến 3.6)
- `/opt/repos/orca/docs/research/receive-request/request-classification-and-flows.md`
- `/opt/repos/orca/frontend/src/renderer/src/components/TaskPage.tsx` (`handleUseJiraItem` ~3553, `openComposerForJiraItem` ~3532, `onUse` ~4708)
- `/opt/repos/orca/frontend/src/renderer/src/components/task-page-jira-issue-list.tsx`, `JiraIssueWorkspace.tsx`, `GitHubItemDialog.tsx`
- `/opt/repos/orca/frontend/src/renderer/src/hooks/useJiraProjectPreselect.ts`
- `/opt/repos/orca/frontend/src/renderer/src/lib/screen-submit-shortcut.ts`, `components/ShortcutKeyCombo.tsx`
- `/opt/repos/orca/frontend/src/renderer/src/components/task/TaskSourceBadge.tsx`
- `/opt/repos/orca/frontend/src/shared/jira-types.ts` (`JiraIssue`), `shared/types.ts` (`GitHubWorkItem`)
- `/opt/repos/orca/guides/STYLEGUIDE.md`; `/opt/repos/orca/docs/ui/pages/tasks.md`
- Mới: `components/request/{RequestsTab,RequestList,RequestDetailPane,TypeConfirmationCard,CreateRequestDialog,RequestStageTimeline}.tsx`

# CR-REQ-022 — Hộp duyệt: gom mọi Approval chờ xử lý của người dùng

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-022 |
| **Tên** | Hộp duyệt (Approval inbox) gom mọi Approval `pending` của người dùng; duyệt nhanh, từ chối có lý do, mở đúng ngữ cảnh |
| **Loại** | Feature |
| **Priority** | 🟠 P1 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-REQ-018 (`useApprovals`, chấm số sidebar, tab); CR-REQ-019, 020, 021 (đích điều hướng sâu, `RejectReasonDialog`); backend CR-REQ-009 (`ListPendingForUser`), CR-REQ-010 (quyền, hạn, thông báo); kênh CR-REQ-016 |
| **Mở khoá** | Không |
| **Tác động** | `frontend/src/renderer/src/components/request/approval/` (mới), `components/sidebar/SidebarNav.tsx` (chấm số, do CR-REQ-018 thêm), `i18n/locales/*.json` |

---

## 1. Bối cảnh và vấn đề

Series v6 có nhiều cổng duyệt (`request_type`, `solution`, `findings`, `answer`, `plan`, `task_list`, `phase`, `pre_deploy`; README 3.4). Mỗi cổng sống trong tab riêng của từng Request (CR-REQ-019 đến 021). Người duyệt không muốn mở từng Request để biết việc nào đang chờ mình. `ApprovalService.ListPendingForUser` (README 3.6) tồn tại đúng cho nhu cầu này.

Hiện trạng: `DecisionGate` có HTTP `/v1/orchestration/gates` nhưng không có UI, và README O4 quy định hộp duyệt chỉ hiện `Approval`, không trộn `DecisionGate`. Frontend có hệ thống phê duyệt MCP riêng (`useMcpApprovalDeadline.ts`, `McpGlobalLayer`) cho cấp quyền token; đây là khái niệm khác, không dùng chung dữ liệu, chỉ tham khảo cách hiển thị hạn.

## 2. Giải pháp đề xuất

### 2.1 Cây component (tab "Hộp duyệt" của `RequestPage`)

```
ApprovalInboxTab
├─ ApprovalInboxToolbar
│    ├─ ApprovalSubjectFilter   (ui/toggle-group: Tất cả | Phân loại | Solution | Plan | Phase | Trước triển khai | Khác)
│    ├─ ProjectFilter           (dùng bộ chọn dự án của RequestPageHeader)
│    └─ OverdueToggle           ("Chỉ quá hạn")
├─ ApprovalList                 (nhóm theo Request, mỗi nhóm có tiêu đề Request)
│    └─ ApprovalRow
│         ├─ ApprovalSubjectIcon + nhãn subject_type
│         ├─ tóm tắt đối tượng cần duyệt (tên Plan/Phase, loại AI đề xuất, tiêu đề Solution)
│         ├─ RequestTypeBadge, người yêu cầu (`requested_by`), thời điểm, hạn (`due_at`)
│         └─ hành động: Mở, Duyệt nhanh (nếu hợp lệ), Từ chối
├─ ApprovalInboxEmptyState / ErrorState / Skeleton
└─ RejectReasonDialog           (dùng lại từ CR-REQ-020)
```

Bố cục: một cột danh sách đầy bề rộng, tối đa 50 mục mỗi trang, "Tải thêm" theo `nextPageToken`. Sắp xếp: quá hạn trước, rồi `due_at` tăng dần, rồi `requested_at` giảm dần. Tiêu đề tab hiện số chờ (`pendingApprovalCount`).

### 2.2 Nguồn dữ liệu

- `useApprovals({scope:'mine', status:'pending'})` gọi `approval.listPending` (`ListPendingForUser`). Kết quả tối thiểu cần: `id`, `request_id`, `subject_type`, `subject_id`, `stage`, `requested_by`, `due_at`, `version`, và để vẽ hàng cần thêm tiêu đề Request và loại (xem mục 7: dùng thêm `request.get` theo lô hoặc backend nhúng sẵn).
- Cập nhật tức thì: `approval.requested` (thêm hàng), `approval.decided` (bỏ hàng), cập nhật `pendingApprovalCount` trong store. Không có luồng sự kiện thì polling 30 giây khi tab hiển thị, và cập nhật chấm số sidebar mỗi 60 giây (CR-REQ-018 2.5).
- Không hiện `DecisionGate` (O4). Nếu sau này hợp nhất, thêm cột nguồn.

### 2.3 Hành động, quyền, và quy tắc từ chối

| Hành động | Hiện khi | Hành vi |
|---|---|---|
| Mở | luôn | `openRequestPage({section:'requests', requestId, focus: subject_type})` mở tab đúng: `request_type` mở hộp xác nhận; `solution`/`findings`/`answer` mở tab Phân tích; `plan`/`task_list`/`phase`/`pre_deploy` mở tab Plan |
| Duyệt nhanh | `subject_type` không đòi chọn nội dung: `request_type`, `task_list`, `phase`, `pre_deploy`, `answer`, `findings` | `approval.approve {approvalId, version}` sau hộp xác nhận ngắn (tên đối tượng, hậu quả) |
| Duyệt nhanh (ẩn) | `solution` (đòi chọn phương án), `plan` (cần xem cây) | Chỉ có "Mở"; chọn/xem phải ở màn chi tiết |
| Từ chối | luôn khi có quyền | `RejectReasonDialog`, **lý do bắt buộc** (tối thiểu 10 ký tự sau cắt khoảng trắng), `approval.reject {approvalId, comment, version}` |

- Không có duyệt hàng loạt trong v1: duyệt là quyết định có hậu quả (chạy agent), và Solution/Plan cần xem trước. Từ chối hàng loạt cũng không có (mỗi lý do khác nhau).
- Quyền: hiện nút khi người xem là người duyệt hợp lệ (backend chỉ trả Approval của người đó ở `ListPendingForUser`). Nếu `forbidden` khi ghi: toast, hàng giữ nguyên, tải lại danh sách. `Approval.status` đổi sang `approved`/`rejected`/`cancelled`/`expired` bởi người khác trước: lỗi `invalid_state` hoặc `conflict`, hàng biến mất kèm toast "Đã được xử lý bởi người khác".
- Tự duyệt (người yêu cầu cũng là người duyệt) do CR-REQ-010 quy định; UI không tự chặn.
- Phím tắt (khi tiêu điểm ở danh sách): `j`/`k` di chuyển, `Enter` mở. Không có phím tắt cho Duyệt hay Từ chối. `Mod+Enter` chỉ gửi trong `RejectReasonDialog`; nhãn `⌘`/`Ctrl` theo nền tảng bằng `ShortcutKeyCombo`.

### 2.4 Hạn và hết hạn

- `due_at` hiện thời gian tương đối ("còn 3 giờ", "quá hạn 2 ngày"); hàng quá hạn có icon cảnh báo (`destructive`) và nằm trên cùng.
- `expired` không còn trong danh sách `pending`; hàng tự biến mất sau `approval.decided`/làm mới. Có thể mở Request để thấy lịch sử.
- Hạn tính theo giờ máy chủ; UI chỉ hiển thị, tính tương đối bằng `Date.now()` và cập nhật mỗi phút (tham khảo `useMcpApprovalDeadline.ts`).

### 2.5 Trạng thái rỗng, tải, lỗi

| Tình huống | UI |
|---|---|
| Đang tải lần đầu | `ApprovalInboxSkeleton` 6 hàng |
| Không có Approval nào | Icon `Inbox` (cỡ `size-7`) + "Không có mục nào chờ bạn duyệt" |
| Có bộ lọc mà rỗng | "Không có mục khớp bộ lọc" + "Xoá bộ lọc" |
| `network` | Banner "Không tải được hộp duyệt" + "Thử lại"; giữ danh sách cũ mờ đi nếu có |
| `forbidden` | "Bạn không có quyền xem hộp duyệt" |
| `unsupported` | Tab bị ẩn (CR-REQ-018); chấm số sidebar không hiện |
| Mở đối tượng đã bị xoá (`not_found`) | Toast "Request không còn tồn tại" và bỏ hàng |

### 2.6 Chấm số sidebar và thông báo

- `pendingApprovalCount` hiển thị ở nút "Requests" (tối đa `99+`) và ở tiêu đề tab. Đếm từ `approval.listPending` (lấy `total` nếu có, không thì độ dài trang đầu, đánh dấu `99+` khi còn trang kế).
- Thông báo hệ thống (CR-REQ-010) khi bấm sẽ gọi `openRequestPage({section:'approvals'})`; CR này chỉ cung cấp điểm vào, không tạo thông báo.

### 2.7 i18n

Tiền tố `auto.components.request.approval.`, đủ 5 locale: `ApprovalInboxTab.title`, `ApprovalInboxTab.empty`, `ApprovalInboxTab.emptyFiltered`, `ApprovalSubjectFilter.{all,requestType,solution,plan,phase,preDeploy,other}`, `ApprovalRow.{open,approve,reject,overdue,dueIn,requestedBy}`, `ApprovalRow.confirmApprove`, `ApprovalRow.alreadyDecided`, `OverdueToggle.label`, `ApprovalInboxErrorState.{network,forbidden}`, và nhãn 8 giá trị `ApprovalSubjectType.<type>`.

## 3. Quyết định thiết kế

- Duyệt nhanh chỉ cho cổng không cần chọn nội dung; `solution` và `plan` bắt buộc qua màn chi tiết để người duyệt thấy phương án và cây.
- Không duyệt hàng loạt ở v1, tránh hậu quả diện rộng.
- Từ chối luôn có lý do, dùng chung một hộp với CR-REQ-020/021.
- Hộp duyệt không giữ trạng thái riêng ngoài `pendingApprovalCount`; danh sách luôn lấy từ máy chủ.
- Không trộn `DecisionGate` (O4).

## 4. Tiêu chí chấp nhận

- [ ] Hộp duyệt chỉ hiện Approval `pending` của chính người dùng, gom theo Request.
- [ ] Mỗi hàng hiện loại cổng, đối tượng, Request, người yêu cầu, hạn; quá hạn nằm trên cùng và có dấu hiệu riêng.
- [ ] "Mở" đưa tới đúng tab theo `subject_type`.
- [ ] `solution` và `plan` không có nút Duyệt nhanh; các cổng còn lại có và cần xác nhận ngắn.
- [ ] Từ chối với lý do rỗng: nút bị khoá, không có yêu cầu gửi đi; hợp lệ thì gửi `comment` và hàng biến mất.
- [ ] Approval đã bị người khác xử lý: hàng biến mất kèm toast, không báo lỗi đỏ.
- [ ] Nhận `approval.requested` thêm hàng, `approval.decided` bỏ hàng, chấm số sidebar đổi theo.
- [ ] Runtime không hỗ trợ `approval.*`: không có tab, không có chấm số, không toast.
- [ ] Rỗng, đang tải, lỗi mạng, lỗi quyền đều có UI riêng.
- [ ] `j`/`k`/`Enter` hoạt động khi tiêu điểm ở danh sách và bị bỏ qua khi ở ô nhập; `Mod+Enter` trong hộp từ chối đúng `metaKey` Mac và `ctrlKey` nơi khác.
- [ ] Mọi chuỗi mới có 5 locale.

## 5. Kiểm thử

- Unit: sắp xếp (quá hạn, `due_at`, `requested_at`); quy tắc "duyệt nhanh được hay không" theo `subject_type` (bảng test 8 giá trị); định dạng hạn tương đối; điều hướng sâu (`subject_type` → tab).
- Component: `ApprovalRow` (nút theo cổng), `ApprovalInboxTab` (rỗng, lọc, lỗi), `RejectReasonDialog` (lý do bắt buộc), phím tắt, chấm số sidebar (mở rộng `SidebarNav.test.tsx`).
- Hook: `useApprovals` (`approval.requested`/`approval.decided`, polling, `conflict`).
- E2E (cần CR-REQ-009/010): hai người dùng, một người duyệt trước, người kia thấy hàng biến mất; mở từ hộp duyệt đến Solution.
- Chưa chạy; kế hoạch.

## 6. Rủi ro và điểm chưa kiểm chứng

- Dữ liệu một hàng đủ để vẽ hay phải gọi thêm `request.get` theo lô (N+1) chưa rõ.
- Chấm số sidebar cần gọi nền định kỳ: tải server và pin trên máy khi nhiều cửa sổ; cần dùng sự kiện là chính.
- Phân giải "người duyệt hợp lệ" nằm ở backend (CR-REQ-010); nếu backend trả cả mục người dùng không duyệt được, hộp sẽ hiện nút vô ích.
- Duyệt nhanh `pre_deploy` hay `phase` là hành động mạnh; hộp xác nhận ngắn có thể chưa đủ, chưa có nghiên cứu người dùng.

## 7. Câu hỏi mở

1. `approval.listPending` có nhúng tiêu đề Request, loại, tóm tắt đối tượng, `total` không? README 3.6 không nêu hình dạng kết quả.
2. Có cần tab "Đã xử lý gần đây" (xem `approved`/`rejected`/`expired`)? README chỉ yêu cầu `pending`.
3. Duyệt hàng loạt cho các cổng không cần chọn có nên có ở v1.1?
4. `findings` và `answer` có đủ thông tin để duyệt nhanh, hay luôn phải mở?
5. Hộp duyệt cho người có quyền theo vai trò/nhóm (không chỉ gán cá nhân): ai thấy mục? CR-REQ-010 chưa chốt.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/README.md` (D3, O4, 3.4 đến 3.7)
- `/opt/repos/orca/frontend/src/renderer/src/hooks/useMcpApprovalDeadline.ts` (mẫu đếm hạn), `components/mcp/` (UI phê duyệt MCP, khác khái niệm)
- `/opt/repos/orca/frontend/src/renderer/src/components/sidebar/SidebarNav.tsx`
- `/opt/repos/orca/frontend/src/renderer/src/lib/screen-submit-shortcut.ts`, `components/ShortcutKeyCombo.tsx`
- `/opt/repos/orca/frontend/src/renderer/src/components/ui/{toggle-group,table,skeleton,dialog}.tsx`
- `/opt/repos/orca/guides/STYLEGUIDE.md`
- Mới: `components/request/approval/{ApprovalInboxTab,ApprovalList,ApprovalRow,ApprovalSubjectFilter}.tsx`

# Feature: request-frontend — Giao diện Request, Solution, Plan, Approval, Backlog

> Thuộc series [Change Requests v6](../README.md). Trạng thái: 📝 Đề xuất, chưa triển khai. Viết từ khảo sát code ngày 2026-10-05 (CR-REQ-032 và 036 bổ sung ngày 2026-10-06, xem [ADDENDUM-2026-10-06.md](./ADDENDUM-2026-10-06.md)); chưa chạy test hay ứng dụng.

Phạm vi: `frontend/src/renderer/src` và `frontend/src/shared`. Backend thuộc CR-REQ-001 đến 017, chỉ tham chiếu bằng mã CR. Nguyên tắc "UI sau cùng": bắt đầu khi RPC của CR-REQ-016 chạy được.

## Danh sách CR

| CR | Tên | Priority | Effort | Phụ thuộc chính |
|---|---|---|---|---|
| [CR-REQ-018](./CR-REQ-018-request-frontend-foundation.md) | Nền frontend: kiểu, hook, store, định tuyến; gỡ `backlog` khỏi `TaskStatus` | 🔴 P0 | Medium | CR-REQ-016 |
| [CR-REQ-019](./CR-REQ-019-request-list-detail-classification-ui.md) | Danh sách, chi tiết, xác nhận phân loại; "Tạo Request" từ Tasks | 🔴 P0 | Large | 018 |
| [CR-REQ-020](./CR-REQ-020-solution-review-ui.md) | Xem, so sánh, chọn, duyệt, từ chối Solution (cả Chẩn đoán, Findings, Answer) | 🔴 P0 | Large | 018, 019 |
| [CR-REQ-021](./CR-REQ-021-plan-phase-tree-and-approval-ui.md) | Cây Plan → Phase → Task, duyệt Plan và Phase, tiến độ, lọc khỏi Board | 🔴 P0 | Large | 018 đến 020 |
| [CR-REQ-022](./CR-REQ-022-approval-inbox.md) | Hộp duyệt gom mọi Approval `pending` | 🟠 P1 | Medium | 018, 020, 021 |
| [CR-REQ-023](./CR-REQ-023-backlog-screens.md) | Màn Backlog ba phân đoạn | 🟠 P1 | Medium | 018, 019, 021 |
| [CR-REQ-032](./CR-REQ-032-graph-canvas-and-lenses.md) | Canvas đồ thị và các lens (Luồng, Kiến trúc, Hợp đồng, Dữ liệu, Phạm vi ảnh hưởng, Kế hoạch, Thực thi) | 🟠 P1 | Large | 018, CR-REQ-030 |
| [CR-REQ-036](./CR-REQ-036-clarification-decision-readiness-impact-ui.md) | Giao diện hỏi lại, quyết định, sẵn sàng, tác động rủi ro, kết quả thực thi | 🟠 P1 | Large | 018 đến 021, CR-REQ-028, 029, 030 |

## Thứ tự thực thi

```
CR-REQ-018 ─▶ CR-REQ-019 ─▶ CR-REQ-020 ─▶ CR-REQ-021 ─▶ CR-REQ-022
                                                   └──▶ CR-REQ-023
```

Phụ thuộc thật: 022 chỉ cần 018 cộng điểm vào của 019 đến 021 (có thể làm sớm hơn và để liên kết sâu tạm trỏ về chi tiết Request); 023 cần 018 và 019 (mở lại, hủy), chỉ liên kết sang 021. Đợt gợi ý theo README v6: đợt 2 gồm 018, 019, 020; đợt 3 gồm 021; đợt 4 gồm 022, 023.

## Quyết định chung cho cả nhóm

- **Một view cấp cao `requests`** với ba tab (Requests, Hộp duyệt, Backlog) thay vì ba view; điều hướng bằng `openRequestPage` (mẫu `openTaskPage`). Sở hữu: CR-REQ-018.
- **Chủ sở hữu kiểu và hook:** `frontend/src/shared/request-types.ts`, `request-flow-registry.ts`, `request-errors.ts` và các hook `useRequests`, `useRequest`, `useRequestActions`, `useSolutions`, `useApprovals`, `useBacklog`, `useRequestEvents` thuộc CR-REQ-018; các CR sau chỉ dùng, không định nghĩa lại.
- **Gỡ `backlog` khỏi `TaskStatus`** (D4, O3) ở CR-REQ-018, kèm `normalizeTaskStatus` để dữ liệu cũ không làm mất task trên Board.
- **Runtime không hỗ trợ kênh:** không hiện gì và không báo lỗi (như `useTaskSource` với `task.getSource`); `useRequestFlowSupport` quyết định việc ẩn tab và nút sidebar.
- **Cổng duyệt từ chối bắt buộc nhập lý do** (tối thiểu 10 ký tự sau cắt khoảng trắng), dùng chung `RejectReasonDialog` (CR-REQ-020) ở Solution, Plan, Phase và hộp duyệt. Duyệt không có phím tắt.
- **Phím tắt đa nền tảng:** `isScreenSubmitShortcut` (`metaKey` Mac, `ctrlKey` nơi khác); nhãn bằng `ShortcutKeyCombo` (`⌘`/`Ctrl`). Phím điều hướng danh sách (`j`/`k`/`Enter`) bị bỏ qua khi tiêu điểm ở ô nhập.
- **i18n:** khoá đọc theo tên, tiền tố `auto.components.request.`, đủ `en, es, ja, ko, zh`, kèm test phủ khoá theo mẫu `task-jira-link-locale-coverage.test.ts`.
- **UI:** chỉ token và primitive có sẵn (`ui/*`, `lucide-react`); không hex, không emoji; trạng thái dùng icon và token (`status-success`, `destructive`, `primary`, `muted-foreground`).
- **SSH và remote:** mọi lời gọi qua `callRuntimeRpc` với `getActiveRuntimeTarget`, không giả định thực thi cục bộ; trạng thái tải chịu được độ trễ 50 đến 200 ms.
- **Không thêm `max-lines` disable**; `TaskPage.tsx` chỉ nhận thêm nút qua component riêng.

## Mâu thuẫn và thiếu sót của hợp đồng README v6 (chưa sửa README, đã ghi ở "Câu hỏi mở")

1. README 3.6 không có tên phương thức WS, kênh phát sự kiện thời gian thực, mã lỗi, hay cách phân trang.
2. Không nêu cách frontend đọc cờ `request_flow_enabled`.
3. `Solution.options` (JSON) không có schema; hình dạng kết quả `backlog.list`, `approval.listPending` chưa rõ.
4. D4 nói backlog "hiển thị ở frontend" nhưng 3.6 và CR-REQ-015 có `ListBacklog` ở backend.
5. Không có trường quyền theo người xem; chưa rõ Request/Plan nào lấy bằng `task.list` với `requestId`.
6. `TaskType` và `TaskStatus` frontend lệch backend (`story|subtask|spike`, `todo`) ngoài `backlog`.
7. Liên kết `STYLEGUIDE.md` trỏ `docs/STYLEGUIDE.md` (không tồn tại); file thật là `guides/STYLEGUIDE.md`.

# frontend v6: giao diện Request → Solution → Plan → Phase → Task

> **Trạng thái: 🚧 In Progress.** Rà soát ngày 2026-10-07. SOL-018 ✅ Done (6/6 tasks); SOL-019 🚧 In Progress (2/7 done hoặc partial); SOL-020 🚧 In Progress (1/5 done); SOL-021–023, 032, 036 🔴 Not Started. Tiến độ tổng: ~22% (12/54 tasks). Chưa chạy typecheck/lint/e2e toàn bộ.

CR nguồn: [`docs/crs/v6/request-frontend/`](../../../../docs/crs/v6/request-frontend/README.md) (cùng [`ADDENDUM-2026-10-06.md`](../../../../docs/crs/v6/request-frontend/ADDENDUM-2026-10-06.md)). Hợp đồng backend: [`CONTRACT-request-ui-api.md`](../../../backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md). Thiết kế giao diện: [`docs/research/receive-request/frontend-visualization-and-ux.md`](../../../../docs/research/receive-request/frontend-visualization-and-ux.md).

Cùng bộ: [backend-go](../../../backend-go/crs/v6/README.md) và [agent](../../../agent/crs/v6/README.md).

## 1. Cấu trúc

Một feature duy nhất [`request-frontend/`](./request-frontend/solutions/README.md) với `solutions/` và `tasks/`. ID: `FE-REQ-SOL-<NNN>` và `FE-REQ-TASK-<NNN>-<NN>` (NNN = số CR). "TDD" là Technical Design Document, không phải test-driven development.

| CR | Solution | Task | Nội dung |
|---|---|---|---|
| 018 | FE-REQ-SOL-018 ✅ | 6/6 ✅ | Nền: kiểu, registry luồng, RPC client, hook, store, định tuyến, khung `RequestPage`; **gỡ `backlog` khỏi `TaskStatus`** |
| 019 | FE-REQ-SOL-019 🚧 | 2/7 | Danh sách, chi tiết, xác nhận phân loại, Request con, "Tạo Request" từ trang Tasks |
| 020 | FE-REQ-SOL-020 🚧 | 1/5 | Xem, so sánh, chọn, duyệt, từ chối Solution |
| 021 | FE-REQ-SOL-021 🔴 | 0/6 (model partial) | Cây Plan → Phase → Task, duyệt, khoá nút Chạy; lọc Plan/Phase khỏi Board |
| 022 | FE-REQ-SOL-022 🔴 | 0/7 | Hộp duyệt chờ xử lý |
| 023 | FE-REQ-SOL-023 🔴 | 0/7 | Màn Backlog ba view |
| 032 | FE-REQ-SOL-032 🔴 | 0/8 | Canvas đồ thị và các lens (task 07 là tuỳ chọn, bị chặn bởi duyệt phụ thuộc) |
| 036 | FE-REQ-SOL-036 🔴 | 0/8 | Giao diện hỏi lại, quyết định, sẵn sàng, tác động rủi ro, kết quả thực thi |
| **Tổng** | **8** | **~12/54 (~22%)** | |

Chỉ mục bổ sung: [`PARTIAL-INDEX-022-023.md`](./request-frontend/solutions/PARTIAL-INDEX-022-023.md) và [`PARTIAL-INDEX-032-036.md`](./request-frontend/solutions/PARTIAL-INDEX-032-036.md) (cùng bản ở `tasks/`).

## 2. Thứ tự

```
018 ─▶ 019 ─▶ 020 ─▶ 021 ─▶ 022, 023
 │                           
 └─ 018-06 (gỡ backlog) và 021-01 làm được ngay, không cần backend
032-01 (token rủi ro) ─▶ 032-02 (TaskDAGView sang token) ─▶ 032-03..08
036 sau 018 và các CR backend 028, 029, 030
```

Nguyên tắc: UI chỉ làm sau khi RPC và kênh WS tương ứng chạy ở backend (đợt 2 và 3 của backend). Các việc không cần backend (kiểu, hook, gỡ `backlog`, token) làm trước được.

## 3. Điểm cần chốt

| # | Vấn đề | Nơi liên quan |
|---|---|---|
| 1 | **Token màu rủi ro thiếu trong `main.css`** (chỉ có `--destructive`, `--status-success`, `--chart-*`). Đề xuất thêm `--risk-low/medium/high/critical` và `--graph-edge-added/removed` (cả `:root` và `.dark`) và cập nhật `guides/STYLEGUIDE.md`. Chưa kiểm tra độ tương phản | 032-01 |
| 2 | **`docs/crs/v7/README.md` (mục O5) cấm thêm `elkjs`/`dagre` ở MVP** trái CR-032. Giải pháp: `GraphCanvas` có `LayoutEngine` cắm được, mặc định bố cục nội bộ; 032-07 tuỳ chọn. v7 cũng đề xuất token `--review-*` có thể lệch `--risk-*` | CR-032 |
| 3 | **Tên kênh và mã lỗi**: CR frontend ban đầu lệch CR-016 (ví dụ `solution.chooseOption` → `solution.choose`; `backlog.list` → `backlog.requests/tasks/execute`; `request.subscribe`; mã `REQUEST_*`, `APPROVAL_*`). Các solution đã theo CR-016 và CONTRACT; các chỗ ngoài CONTRACT ghi "(tạm)" (`clarification.*`, `decision.*`, `impact.*`, `readiness.*`, `execution.get`) | tất cả |
| 4 | **`task.list` chỉ nhận `projectId`, `pageToken`, `pageSize`**, không lọc theo Request: `usePlanTree` tải cả dự án và dựng cây ở client (tạm) | 021 |
| 5 | **`TaskTreeView` không tự nâng Task mồ côi lên gốc**; task 021-01 sửa bằng `parentId` hiệu dụng. `TaskDetail()` không nhận props nên khoá nút Chạy theo Phase đi qua store | 021 |
| 6 | **`RequestStatus` lên 12 giá trị** (`awaiting_information`); 036-01 sửa file của 018-01 và 018-02 | 018, 036 |
| 7 | **Thiếu trong CR-016**: kênh liệt kê Request con, đọc nội dung dài của Solution, `viewerCan`, `q`, `reporterId` cho `request.list`; `approval.listPending` không nhúng Request nên phải gọi `request.get` (N+1, có cache) | 019, 022 |
| 8 | **`impact.graph` cắt 50 node mặc định**; frontend phải gửi `maxNodes` (đề xuất 500, trần 2.000) | 032, 036 |
| 9 | **Clarification kiểu tệp**: backend v1 chỉ nhận văn bản tối đa 64 KB; không có kho tải lên và app không có component upload dùng lại được | 036 |
| 10 | **Scripts**: `frontend/package.json` chỉ có `build`, `dev`, `test`, `test:watch`; typecheck, lint, `verify:localization-*`, Playwright ở gốc có thể không chạy (thiếu `config/`, `playwright.config.*`). Lệnh trong task chưa kiểm chứng | tất cả |
| 11 | **`TaskDAGView.tsx` còn hex cứng**, `<select>` thô và `window.confirm` (ngoài phạm vi, chỉ sửa màu ở 032-02) | 032 |
| 12 | **Nút trên trang Tasks tên "Start workspace"**, không phải "Start work"; `GitHubWorkItem` không có `owner/repo` trực tiếp nên định dạng `source_ref` GitHub chưa chốt | 019 |
| 13 | **Hộp duyệt**: "Duyệt nhanh" phải bỏ với rủi ro từ Trung bình; `approval.approve` cần `{id, expectedVersion, expectedDigest}` và với rủi ro cao `viewedImpactDigest`, `acceptedFindingIds` | 022, 036 |
| 14 | **`TaskDetail` trong `Sheet`** chưa kiểm chứng chạy được ngoài `TaskPage` (023-06 có phương án thay) | 023 |
| 15 | **Bản dịch** es, ja, ko, zh cần người bản ngữ rà | i18n |

## 4. Giới hạn của bộ tài liệu này

- Task dài 36 đến 100 dòng, dòng dày. Đã lấy mẫu, chưa đọc toàn bộ 54 task.
- Hiệu năng xyflow với hàng trăm node, `var()` trong SVG, chế độ sáng và tối, màn hình hẹp, di động: chưa kiểm chứng. Các ngưỡng (50 node, 2.000 node, mức zoom, 10 ký tự lý do) là đề xuất.
- GitNexus `impact` và `detect_changes` chưa chạy; các task nhắc phải chạy trước khi sửa symbol có sẵn.

## Trạng thái triển khai (cập nhật 2026-10-08)

Trạng thái dưới đây được tổng hợp từ dòng `**Status:**` của từng task, **sau khi đã đối chiếu với code và chạy test thật** (không còn dấu DONE hàng loạt như đợt soạn spec). Quy ước: `[x] DONE` chỉ khi file tồn tại, test của task tồn tại và chạy qua, không thêm lỗi `tsc`/`oxlint` ở file đó; `[~] PARTIAL` ghi rõ phần thiếu; `[!] BLOCKED` ghi phụ thuộc; `[ ] TODO` chưa làm. Tên file thực tế có thể khác spec (ghi ở "Ghi chú triển khai" cuối mỗi task).

| Feature | CR | Task | DONE | PARTIAL | BLOCKED | TODO |
|---|---|---|---|---|---|---|
| `request-frontend` | CR-REQ-018 | 6 | 6 | 0 | 0 | 0 |
| `request-frontend` | CR-REQ-019 | 7 | 7 | 0 | 0 | 0 |
| `request-frontend` | CR-REQ-020 | 5 | 5 | 0 | 0 | 0 |
| `request-frontend` | CR-REQ-021 | 6 | 6 | 0 | 0 | 0 |
| `request-frontend` | CR-REQ-022 | 7 | 7 | 0 | 0 | 0 |
| `request-frontend` | CR-REQ-023 | 7 | 7 | 0 | 0 | 0 |
| `request-frontend` | CR-REQ-032 | 8 | 7 | 0 | 1 | 0 |
| `request-frontend` | CR-REQ-036 | 8 | 8 | 0 | 0 | 0 |
| **Tổng** | | **54** | **53** | **0** | **1** | **0** |

**Kiểm chứng tổng (2026-10-08):** toàn bộ test frontend: 27 file fail / 2 391 (baseline trước khi triển khai: 46 / 2 054); 81 test fail / 20 482 (baseline 281 / 17 864); `tsc -p frontend/tsconfig.json`: 117 lỗi (baseline 184), không lỗi nào ở file của series này. Các test fail còn lại thuộc nơi khác (ví dụ `no-top-level-translate` ở `FleetServerStatusBadge`, `remote-runtime-shared-control-boundary`, `selectors.test`). Hai file `WorktreeCard.*` thỉnh thoảng fail khi chạy gộp do tải (timeout), chạy riêng thì qua.

**Chưa kiểm chứng được trong môi trường này:** e2e Playwright (không có Chromium), build/bundle, `verify:localization-*` (thiếu script), kiểm tay trên Electron/thiết bị thật, typecheck của `mobile/` (chưa cài phụ thuộc), bản dịch es/ja/ko/zh cần người duyệt.

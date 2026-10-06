# Feature: quality-visualization — Đồ hoạ và giao diện kiểm soát chất lượng

> Thuộc series [Change Requests v7](../README.md). Trạng thái: 📝 Đề xuất, chưa triển khai. Viết từ khảo sát code ngày 2026-10-06; chưa chạy test hay ứng dụng. Nguồn: [nghiên cứu 11](../../../research/view-code/11-additions-for-quality-control.md) (§5 D1–D7, C2, D3, D6) và [10](../../../research/view-code/10-frontend-review-ux.md).

Phạm vi: `frontend/src/renderer/src` (component, hook, slice, i18n, `main.css`) và `frontend/src/shared` (kiểu quality). Backend (agent `quality.*`, `QualityGateService`, kênh `codeIntel.quality.*`) thuộc CR-CV-080 đến 086, 089 đến 095; chỉ tham chiếu bằng mã CR và README v7 mục 3.10.

## Danh sách CR (2)

| CR | Tên | Priority | Effort | Phụ thuộc chính |
|---|---|---|---|---|
| [CR-CV-088](./CR-CV-088-graphics-foundation-and-chart-primitives.md) | Nền đồ hoạ: kiểm kê, bảng quyết định thư viện/bố cục/engine, token màu, truy cập được, hiệu năng, `components/quality-charts/` | 🔴 P0 | Large | CR-CV-050, 053 (mã hoá lớp phủ); phối hợp 071 |
| [CR-CV-087](./CR-CV-087-quality-frontend-scorecard-and-annotations.md) | Frontend chất lượng: lens `quality` (scorecard, coverage, xu hướng, hotspot, DSM), phát hiện kiểm tra, chú thích trên diff, chạy kiểm tra, cảnh báo trước khi tạo review đã gửi | 🔴 P0 | Large | **CR-CV-088**, 050, 051, 053, 059, 060, 061; backend 081, 082, 083, 085, 086, 037, 040 |

## Thứ tự thực thi

```
CR-CV-050 ─▶ CR-CV-088 (nền đồ hoạ) ─▶ CR-CV-087 (frontend chất lượng)
              CR-CV-051, 053, 059, 060, 061 ──────┘     └─ backend 081, 082, 085 (pha 1); 083 (pha 2); 037 (pha 3)
```

- **088 trước 087** (README v7 mục 5: 088 "sau 050", 087 "cần 051, 053, 059, 085"). 088 không cần backend (làm bằng fixture); 087 chia **ba pha theo dữ liệu thật**: pha 1 (scorecard, chạy kiểm tra, danh sách, chú thích diff, cảnh báo trước khi tạo review đã gửi) cần CR-CV-081/082/085; pha 2 (coverage, diff coverage, xu hướng) cần CR-CV-083 và lịch sử của 085; pha 3 (hotspot, DSM) cần dữ liệu của CR-CV-037. Khối chưa có nguồn hiển thị "Chưa có dữ liệu", không ẩn và không đoán.
- 088 và 087 thuộc đợt 7/8 của README v7 mục 5; 088 đủ điều kiện ngay khi CR-CV-050 xong.

## Quyết định chung cho cả nhóm

- **Không thêm dependency** (O5, O12): tự viết SVG/HTML + hàm thuần. CR-CV-088 mục 2.2 là **bảng quyết định có căn cứ** để người duyệt O12 quyết lại: tự viết (đề xuất) so với chỉ module toán `d3-scale`/`d3-shape`/`d3-hierarchy` (ISC, ESM, **đã có trong lockfile** qua `mermaid`/`@xyflow`, chưa là dependency trực tiếp) so với thư viện biểu đồ tích hợp React (chưa kiểm chứng, không có trong lockfile); kèm điều kiện cụ thể để xem lại. `elkjs`/`dagre` chưa thêm; engine đồ thị lớn giữ xyflow + `onlyRenderVisibleElements`.
- **Token mới tối thiểu** ở `main.css`: `--quality-error|warning|info|pass|unknown` (+ nền/viền) và `--quality-heat-1..5` (thang trung tính, không hue mới); lớp phủ dùng lại `--review-*` của CR-CV-050. Chữ màu mức nghiêm trọng chỉ trên `card`/`background`; kiểm tương phản bằng test, không công cụ mới.
- **Không chỉ dựa vào màu**: bảng mã hoá duy nhất `severity-encoding.ts` (biểu tượng, hình, nét, nhãn chữ); bảng thay thế và mô tả văn bản là **bắt buộc** cho mọi hình; thang nhiệt luôn kèm số.
- **`unknown` là trạng thái thật**: hiển thị "Chưa đủ dữ liệu để kết luận" với biểu tượng riêng, không bao giờ như "đạt"; không "an toàn"/"sạch"; không một "điểm số" duy nhất.
- **Chế độ chỉ báo** (O9): cảnh báo trước khi tạo review đã gửi là một khe trong `CreateHostedReviewComposer` (ba nơi gọi), không chặn, không đổi hành vi nút; nhãn theo nhà cung cấp.
- **Danh sách phát hiện**: một dock (CR-CV-059), hai nguồn tách biệt "Cấu trúc | Kiểm tra" ở v1; hành động khác nhau (bỏ qua so với miễn trừ có hạn), không trộn danh sách.
- **Chú thích diff**: `setModelMarkers` (owner `orca-quality`) + glyph hình học, không view zone, không sửa `useDiffCommentDecorator`; phụ thuộc `pendingDiffReveal` của CR-CV-053; xoá khi nội dung đổi.
- **Slice**: mở rộng slice `code-intel` của CR-CV-050 bằng đoạn state tách file (`code-intel-quality-state.ts`), dùng chung đường prune khi xoá worktree.
- **Chạy kiểm tra chỉ theo tên profile** (O11); không ô nhập lệnh.
- **Hai render target, SSH, i18n 5 locale, không `max-lines` disable, không `components/code-review/*`, tên file theo khái niệm** (như README review-frontend).

## Phạm vi ngoài

- Backend/agent/gateway/proto (CR-CV-080 đến 086, 089 đến 095, 040).
- Chặn cứng Create PR/commit (O9: cần quyết định riêng); chỉ cảnh báo.
- Đồ thị Ảnh hưởng, C4, ERD, treemap cấu trúc (CR-CV-053, 055, 057, 054); chỉ dùng lại nền của 088 nếu muốn.
- Đưa thông báo cổng vào `DashboardAgentRow` (CR-CV-061 sở hữu; 087 chỉ cung cấp `QualityGateChip`), báo cáo xuất được (CR-CV-090), đối chiếu "agent tự báo" (CR-CV-089), truy vết yêu cầu (CR-CV-092), tóm tắt AI (CR-CV-093), telemetry (CR-CV-095).
- Mobile (CR-CV-062), `desktop/` (thay đổi preload thuộc CR-CV-050).
- Thêm công cụ đo coverage/phức tạp/bảo mật (O12; CR-CV-083, 091).
- Sửa `progress.tsx`, `table.tsx`, `ConnectionStatusBanner.tsx`, `DAGPreview.tsx`, `TaskDAGView.tsx`.

## Điểm lệch giữa hợp đồng/nghiên cứu/CR khác và code (cần chốt khi duyệt; chưa sửa README v7)

Đã đối chiếu với code ngày 2026-10-06.

1. **README v7 mục 3.10 kênh `codeIntel.quality.*` thiếu `cancel`** (agent có `quality.cancel`; gRPC `QualityGateService` không có `CancelQualityRun`). UI cần nút Huỷ. CR-CV-087 2.1.
2. **Không có RPC/kênh liệt kê profile** (agent `quality.listProfiles`; gRPC chỉ `GetQualityProfile`). Cần `ListQualityProfiles` hoặc `profile.get` không tên trả danh sách. CR-CV-087 2.5.
3. **Push `quality.progress|finished` không mang `worktreeId`** (chỉ `runId`), nên UI không ánh xạ được lần chạy do client khác/tự động (CR-CV-080) khởi chạy. Đề nghị thêm `worktreeId` vào push. CR-CV-087 2.2.
4. **Mã lỗi**: `CODEINTEL_PROFILE_UNKNOWN`, `ENV_NOT_READY`, `RUN_IN_PROGRESS`, `RUN_CANCELLED` chưa có trong `CODE_INTEL_ERROR_CODES` của CR-CV-050; test "đúng 10 mã" ở CR-CV-050 mục 4 cần thành 14 mã (cộng thêm các mã điểm 5 README mục 8 nếu được chốt).
5. **Message chưa tồn tại**: README 3.10 định nghĩa `QualityFinding/Run/Gate` nhưng **không** có `QualityTrendPoint`, `CoverageReport`, `QualityHotspot`, danh sách profile, và dạng "đã miễn trừ" của `QualityFinding`; `QualityFinding` không có cột; `QualityRun` không có cờ "chạy trên cây làm việc bẩn". CR-CV-087 2.1 nêu hình dạng UI cần.
6. **`QualityGate.reasons[].check` là chuỗi tự do**, không ánh xạ sang `category`/`tool` nên "bấm lý do để lọc" cần quy ước. CR-CV-087 7.3.
7. **Hai khái niệm "phát hiện"**: `Finding` (059, cấu trúc, thang `high|medium|low|info`, hành động bỏ qua/đã xử lý, bảng `finding_dismissals`) và `QualityFinding` (README 3.10, thang `error|warning|info`, miễn trừ có hạn, bảng `quality_waivers`); category `architecture`/`convention` có thể trùng `layer_violation`. Chưa ai chốt khử trùng hay ánh xạ mức độ.
8. **CR-CV-054 mục 1 viết "Không có mã treemap nào trong `renderer/src`" nhưng có**: `components/status-bar/workspace-space-layout.ts` (`buildTreemapLayout`) và `WorkspaceSpaceManagerPanel.tsx:770-925`. Thuật toán ở đó (chia đôi cân bằng) khác squarified của 054; đề xuất `treemap-squarified-layout.ts` đặt trong `components/quality-charts/` (088) để 054 nhập lại thay vì viết bản thứ hai (CR-CV-088 7.1).
9. **Token của CR-CV-050 2.10**: `--review-untested` sáng (`amber-600`) chỉ đạt 3,19:1 trên `card` và 2,93:1 trên `muted` (tính tay), dưới 3:1 cho đồ hoạ trên `muted`. CR-CV-088 2.3 và 7.2.
10. **Chỗ "Create PR" thật** là `CreateHostedReviewComposer.tsx` (gọi từ `SourceControl.tsx`, `ChecksPanel.tsx`, `renderPullRequestComposer`) và có thể cả nhánh `CommitArea`; `components/code-review/pr-create-dialog.tsx` (nơi tên gợi ý) là code chết cùng thư mục bị loại. CR-CV-087 2.10.
11. **`progress.tsx` không hỗ trợ tiến độ không xác định và luôn có chuyển tiếp**; `quality.progress.percent` cho phép `null` (README mục 8 điểm 6). CR-CV-087 2.5 dùng spinner tĩnh thay vì sửa file.
12. **`--warning` không được định nghĩa** trong `main.css` dù `main.css:1987-1989` dùng `var(--warning, #f59e0b)`; chưa có token cảnh báo/thông tin. CR-CV-088 2.3 thêm token ngữ nghĩa.
13. **CR-CV-071 chỉ có ngân sách backend**; ngân sách vẽ hình phía frontend (CR-CV-088 2.6) chưa có chỗ chứa trong `codeintel-budgets.json`.
14. **Không có công cụ a11y tự động trong repo** (không `axe-core`/`jest-axe`); kiểm tra truy cập dựa vào test cấu trúc và kiểm tay; kiểm tương phản bằng test tự viết (CR-CV-088 2.3).
15. **`main.css` có `--chart-1..5`** nhưng toàn sắc xanh, giá trị giống nhau ở sáng và tối; không dùng làm thang mức độ. Đã dùng bởi `WorkspaceSpaceManagerPanel.tsx` và `terminal.css`.

## Câu hỏi mở gộp (chi tiết ở mục 7 từng CR)

1. Người duyệt O12 chấp nhận tự viết toàn bộ (088 khuyến nghị) hay duyệt trước `d3-scale`/`d3-shape`/`d3-hierarchy`? Điều kiện xem lại ở 088 mục 2.2. (CR-CV-088 7.5)
2. Chủ sở hữu thuật toán treemap và cập nhật CR-CV-054. (088 7.1)
3. Bổ sung hợp đồng: `quality.cancel`, danh sách profile, `worktreeId` trong push, hình dạng trend/coverage/hotspot, "đã miễn trừ" và `unwaive`. (087 7.4, 7.5)
4. Ánh xạ mức độ và khử trùng giữa `Finding` (059) và `QualityFinding` để có chế độ "Tất cả" trong dock. (087 7.1, 7.2)
5. Nguồn thật của DSM và hotspot (CR-CV-037) và công cụ đo độ phức tạp (nghiên cứu 11 B4 xếp "Sau"). (087 7.9)
6. Capability quyền chạy kiểm tra; cách hiển thị khi không đủ quyền trước lần bấm đầu. (087 7.6)
7. Overview ruler Monaco với token Orca (cần `defineTheme` hay chấp nhận màu theme Monaco). (087 7.7)

# PARTIAL-INDEX: Solutions của CR-REQ-032 và CR-REQ-036 (frontend)

Phần này để người điều phối gộp vào `solutions/README.md`. Không phải README.

## Bảng CR → Solution

| CR | Tên | Solution | Task | Priority | Phụ thuộc chính |
|---|---|---|---|---|---|
| CR-REQ-032 | `GraphCanvas`, 7 lens, token rủi ro, `TaskDAGView` sang token | [FE-REQ-SOL-032](./FE-REQ-SOL-032-graph-canvas-and-lenses.md) | FE-REQ-TASK-032-01 đến 032-08 | 🟠 P1 | FE-REQ-SOL-018, 021; backend CR-REQ-030 (`impact.graph`, `impact.heatmap`) cho 4 lens backend; 032-07 (`elkjs`) chặn bởi duyệt phụ thuộc |
| CR-REQ-036 | UI Clarification, Decision, Readiness, thẻ rủi ro, RiskAcceptance, lệch kế hoạch, kết quả thực thi | [FE-REQ-SOL-036](./FE-REQ-SOL-036-clarification-decision-readiness-impact-ui.md) | FE-REQ-TASK-036-01 đến 036-08 | 🟠 P1 | FE-REQ-SOL-018, 020, 021, 032; backend CR-REQ-028, 029, 030 |

## Thứ tự phụ thuộc

```
FE-REQ-SOL-018 ─▶ FE-REQ-SOL-032 ─▶ FE-REQ-SOL-036
FE-REQ-SOL-021 ─▶ (lens plan của 032)        ▲
FE-REQ-SOL-020 ─────────────────────────────┘ (SolutionDecisionBar, RejectReasonDialog)
```

Trong 032: 032-01 (token) và 032-03 (kiểu, hook) song song; 032-02 (`TaskDAGView`) làm sớm được sau 032-01 và không cần backend; 032-07 chỉ làm khi có quyết định duyệt. 036 cần 032-01 (`RiskBadge`), 032-05 (`GraphMini`), 032-06 (`GraphPanel`); 036-01 và 036-02 có thể bắt đầu ngay sau 018.

## Quyết định chung của hai solution

- Một canvas, lens là cấu hình; `flow`, `plan`, `execution` dựng ở client; 4 lens còn lại cần `impact.graph` (CR-030). `risk='unknown'` không bao giờ vẽ như `low`.
- Token mới `--risk-*`, `--graph-edge-*` (đã xác nhận không có trong `main.css`); cập nhật `guides/STYLEGUIDE.md`.
- `elkjs`/`dagre` là phụ thuộc mới cần duyệt; README v7 O5 cấm ở MVP. `LayoutEngine` cắm được, mặc định bố cục tầng nội bộ; task 032-07 tuỳ chọn.
- `RequestStatus` thành 12 giá trị (`awaiting_information`, CR-REQ-028); parser chịu enum lạ.
- Không có kênh `decision.record`: ghi Decision đi cùng `solution.choose`; xác nhận lần hai là `decision.confirm`, so khớp tên NFC, trim, không phân biệt hoa thường (theo backend, khác CR-036).
- Rủi ro cao: `RiskAcceptance` từng phát hiện (`impact.accept`), `approval.approve` mang `viewedImpactDigest`, `acceptedFindingIds` (tạm); ghi đè `risk.override` lý do ≥ 20 ký tự.
- Drift: dùng Approval `phase` `stage=drift_review` (approve/reject), bỏ nút "Huỷ Phase" vì backend không có.
- Kết quả thực thi cần kênh `execution.get` (chưa có); thiếu thì giữ `stdout` cũ.
- Tất cả kênh mới ẩn êm khi `unsupported`; nháp trả lời Clarification chỉ ở bộ nhớ hook.
- Lệnh test: `pnpm --filter orca-frontend test <đường dẫn>`; i18n: `pnpm run verify:localization-catalog`, `pnpm run verify:localization-coverage` (root, chưa kiểm chứng chạy được).

## Mâu thuẫn và thiếu sót cần người điều phối chốt

1. **v7 O5 so với CR-REQ-032 mục 2.6:** `elkjs` bị cấm ở MVP v7. Giải pháp: `LayoutEngine` + task 032-07 chặn bởi duyệt; không sửa README v7.
2. **CR-036 so với CR-028:** không có `decision.record`; so khớp tên không phân biệt hoa thường; kiểu câu hỏi `single_choice|multi_choice|boolean` thay `single|multi|confirm`.
3. **CR-036 2.7 so với CR-030 2.7:** "Trả về Plan" thành `ReturnToBacklog(stage=phase)`; không có RPC huỷ Phase.
4. **CR-036 so với CR-030:** ngưỡng ghi đè là 20 (không phải 10); backend còn `impact.request|compare|findings|evidence|heatmap` và `risk.override` chưa có trong CR-036.
5. **CR-032 so với CR-030:** backend mặc định `max_nodes=50` (UI cần gửi `maxNodes`); lens `plan`/`execution` lấy rủi ro qua `impact.heatmap`/`impact.drift`.
6. **CONTRACT mục 7** chỉ nêu quy tắc thêm nhóm kênh; mọi kênh `clarification.*`, `decision.*`, `impact.*`, `readiness.*`, `execution.get` là "(tạm)" tới khi CONTRACT cập nhật; chuỗi `eventType` mới chưa chốt.
7. FE-REQ-SOL-022 cần sửa để bỏ "Duyệt nhanh" cho Approval từ mức Trung bình và hiện `RiskBadge` (ghi chú ở 036-05, 036-06); FE-REQ-SOL-019/023 cần nhãn "Chờ bổ sung" và `missing_info`.
8. `guides/STYLEGUIDE.md` cần sửa (token rủi ro); `request-errors` cần thêm hai `kind` (`pending`, `no_dev_server`).

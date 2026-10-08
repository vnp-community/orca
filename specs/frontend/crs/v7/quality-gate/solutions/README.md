# Solutions: quality-gate (frontend, v7)

> ✅ Đã xác minh 2026-10-07 (W1-B): SOL-085 7/7 DONE, 0 PARTIAL, SOL-089 6/7 DONE, 1 PARTIAL, SOL-090 8/8 DONE, 0 PARTIAL, SOL-092 7/7 DONE, 0 PARTIAL, SOL-093 5/6 DONE, 1 PARTIAL, SOL-095 5/7 DONE, 2 PARTIAL. Tổng 36 DONE, 6 PARTIAL, 0 BLOCKED, 0 TODO / 42. 431 test mục tiêu pass. PARTIAL = thiếu điểm mount/nối ở bề mặt thuộc agent khác (shell Review, SOL-051/052/059/060/061/087).

## Bảng CR → Solution

| CR | Solution | Nội dung | Ưu tiên | Số task | Trạng thái |
|---|---|---|---|---|---|
| CR-CV-085 | [FE-CV-SOL-085-source-control-quality-notice](./FE-CV-SOL-085-source-control-quality-notice.md) | Cảnh báo cổng chất lượng ở Source Control | P0 | 7 | 7 DONE / 0 PARTIAL |
| CR-CV-089 | [FE-CV-SOL-089-agent-turn-recorder](./FE-CV-SOL-089-agent-turn-recorder.md) | Ghi lượt agent và đối chiếu "agent tự báo" | P1 | 7 | 7 DONE (2026-10-08) |
| CR-CV-090 | [FE-CV-SOL-090-review-report-export](./FE-CV-SOL-090-review-report-export.md) | Xuất báo cáo review, chèn vào mô tả PR/MR | P1 | 8 | 8 DONE / 0 PARTIAL |
| CR-CV-092 | [FE-CV-SOL-092-requirement-trace-view](./FE-CV-SOL-092-requirement-trace-view.md) | Lens Yêu cầu (truy vết) | P2 | 7 | 7 DONE / 0 PARTIAL |
| CR-CV-093 | [FE-CV-SOL-093-ai-summary-panel](./FE-CV-SOL-093-ai-summary-panel.md) | Thẻ tóm tắt AI (mặc định tắt) | P2 | 6 | 6 DONE (2026-10-08) |
| CR-CV-095 | [FE-CV-SOL-095-review-telemetry](./FE-CV-SOL-095-review-telemetry.md) | Telemetry Review/cổng (enum/khoảng) | P2 | 7 | 7 DONE (2026-10-08) |

## Thứ tự phụ thuộc

```
FE-CV-SOL-050 (bridge, fake backend G4) ─▶ 051 (khung Review) ─▶ 085 ─┬▶ 089 (cần 060, 061)
                                                                       ├▶ 090 (cần 051; tạo `extraSections` cho 093)
                                                                       ├▶ 092 (cần 051, 053)
                                                                       ├▶ 093 (sau 090 task 02 nếu muốn chèn vào báo cáo)
                                                                       └▶ 095 (cần 061, 059; gắn vào 085/090/093)
```

Cổng hợp đồng (CONTRACT-codeintel-proto-and-data-map.md 7.1): G3 (gateway đăng ký 46 kênh) rồi G4 (bridge + fake backend). Trước G3 mọi solution chạy trên fake backend; fixture cần bổ sung (do FE-CV-SOL-073-flag-gating-and-web-e2e sở hữu): `quality.gate` bốn verdict, `quality.turn*`, `quality.report`, `quality.trace*`, `quality.summary` (kể cả `TIMEOUT inProgress`), lỗi `QUALITY_GATE_DISABLED` và `AI_REVIEW_DISABLED`.

## Quyết định chung

- Nhóm `quality.*` nằm sau **hai** cờ hiệu lực `codeIntelEnabled ∧ qualityGateEnabled` (PQ-01): `quality.report` (090) cũng thất bại `CODEINTEL_QUALITY_GATE_DISABLED` khi cờ chất lượng tắt (CR-090 Q4 bị hợp đồng thay thế). AI thêm `aiReviewEnabled`. Hook cờ dùng chung `useQualityFeatureFlags` do FE-CV-TASK-085-01 tạo.
- Một khe `qualityNotice` duy nhất trên `CreateHostedReviewComposer` và `CommitArea` (085). Nơi gọi composer sản phẩm: `SourceControl.tsx:5247` và `ChecksPanel.tsx:3611`; `pr-create-dialog` là code chết (chỉ `code-review-panel.tsx` import, bản thân panel không được import).
- Lỗi đọc từ tiền tố `message` (`CODEINTEL_X: …`), không đọc `error.code` (luôn `internal`); `CODEINTEL_TIMEOUT` với `inProgress` thì thử lại tới 90 s.
- `unknown` luôn "Chưa đủ dữ liệu để kết luận", icon khác `pass`; `mode:'block'` vẫn chỉ cảnh báo (O9); không "an toàn", "đã đáp ứng", "AI đã review"; AI/lời agent tự báo không bao giờ đi vào cổng.
- Chuỗi hiển thị từ backend là văn bản thuần (U9); mọi chuỗi UI qua `translate()` với khoá đọc theo tên + test phủ năm locale (en/es/ja/ko/zh).
- UI theo `guides/STYLEGUIDE.md` (đường dẫn thật là `guides/`, không phải `docs/`): token, shadcn, lucide, không hex, không emoji; Electron và web; SSH (khoá nút ngay, spinner sau ~200 ms).
- Không thêm thư viện; không `max-lines` disable mới (các file đã có disable cũ: `source-control-commit-area.tsx`, `telemetry-events.ts`, `agent-status.ts`).
- Telemetry: sáu bản sao `shared/` (xem FE-CV-SOL-095 mục 1), không script đồng bộ.

## Điểm hợp đồng thiếu hoặc mâu thuẫn (không tự sửa hợp đồng)

1. Kiểu `manifest`, `cache`, `labels` của `quality.summary` và `generatedFor` của `quality.report` không có trong `CONTRACT-codeintel-ui-api.md` 4.7.
2. Giá trị `scope` của `quality.trace.confirm` không liệt kê.
3. `quality.turn.record`: không nói ai điền `claims`; `promptDigest` khi prompt rỗng.
4. `maskSensitiveText` (hợp đồng 4.4) chưa tồn tại trong code.
5. CR-CV-087 và CR-CV-085 đặt tên khe composer khác nhau (`preSubmitNotice` / `qualityNotice`) và CR-087 tính nhầm `renderPullRequestComposer` (hàm test) là nơi gọi.
6. Hợp đồng 4.1 có `Settings.effective.aiReviewEnabled` nhưng không nói màn quản trị nào đổi `aiReviewLevel` (xác nhận hai bước).
7. CR-095 nói 4 bản sao `shared/telemetry-events.ts`; thực tế 6.

## Cách chạy kiểm thử (chung)

Vitest: `pnpm --filter orca-frontend test -- <đường dẫn>` (script `vitest run --config config/vitest.config.ts`, môi trường mặc định `node`; component dùng `// @vitest-environment happy-dom` + Testing Library). `frontend/package.json` không có script typecheck/lint. Trước khi sửa symbol: GitNexus `impact`; trước khi commit: `detect_changes` (chưa chạy). Đồng bộ khoá i18n: `desktop/config/scripts/verify-localization-catalog.mjs` (chưa kiểm chứng chạy được sau khi tách monorepo).

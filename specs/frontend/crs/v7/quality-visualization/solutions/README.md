# Solutions: quality-visualization (frontend, v7)

> Cập nhật 2026-10-07: CR-087 gồm 20 task — 18 DONE, 2 PARTIAL (087-08 thiếu e2e Playwright; 087-11 chưa gắn `MonacoEditor` thường). SOL-088 verified (088-01..09).

## Bảng CR → Solution

| CR | Solution | Nội dung | Tasks | Trạng thái |
|---|---|---|---|---|
| CR-CV-088 | [FE-CV-SOL-088-graphics-foundation-and-chart-primitives](./FE-CV-SOL-088-graphics-foundation-and-chart-primitives.md) | Token `--quality-*`/`--quality-heat-*`, bảng mã hoá không chỉ dựa vào màu, primitive biểu đồ tự viết SVG (quyết định A1, không dependency), test tương phản | 088-01..09 | ✅ verified 2026-10-07 |
| CR-CV-087 | [FE-CV-SOL-087-quality-scorecard-and-state](./FE-CV-SOL-087-quality-scorecard-and-state.md) | Kiểu/parser/lỗi, state `codeIntelQualityByWorktree`, hook, scorecard, chạy kiểm tra, lens `quality` | 087-01..08 | ✅ verified 2026-10-07 (08 PARTIAL: e2e) |
| CR-CV-087 | [FE-CV-SOL-087-quality-diff-annotations](./FE-CV-SOL-087-quality-diff-annotations.md) | Marker + glyph trên diff Monaco, danh sách phát hiện kiểm tra trong dock, miễn trừ/bỏ miễn trừ | 087-09..14 | ✅ verified 2026-10-07 (11 PARTIAL: MonacoEditor) |
| CR-CV-087 | [FE-CV-SOL-087-quality-trend-coverage-hotspot](./FE-CV-SOL-087-quality-trend-coverage-hotspot.md) | Diff coverage + treemap, xu hướng theo lượt, hotspot, DSM | 087-15..20 | ✅ verified 2026-10-07 |

## Thứ tự phụ thuộc

```
FE-CV-SOL-050-* (bridge, slice, fake backend G4) ─▶ FE-CV-SOL-051 (shell, registry lens)
        │
        ▼
FE-CV-SOL-088 (không cần backend) ─▶ FE-CV-SOL-087-scorecard-and-state ─┬▶ FE-CV-SOL-087-diff-annotations      (cần 053, 059)
                                                                         └▶ FE-CV-SOL-087-trend-coverage-hotspot (pha 2: BE 083/085; pha 3: BE 037)
```

Pha theo dữ liệu thật: pha 1 (scorecard, chạy kiểm tra, danh sách, chú thích) cần BE-CV-SOL-082/085 + AG-CV-SOL-081/082; pha 2 (coverage, xu hướng) cần BE-CV-SOL-083/085; pha 3 (hotspot, DSM) cần BE-CV-SOL-037 và `structure`. Trước đó dùng fake backend G4.

## Quyết định chung

- Không thêm dependency (O5, O12): tự viết SVG/HTML + hàm thuần; điều kiện xem lại sang `d3-scale/shape/hierarchy` ở SOL-088 2.1. `elkjs`/`dagre` không thêm.
- Cờ chất lượng từ `settings.get.effective.qualityGateEnabled` (hợp đồng §6); `CODEINTEL_QUALITY_GATE_DISABLED` ẩn phần chất lượng; không dò `typeof window.api.codeIntel`.
- Kênh `codeIntel.quality.*` theo §3.2; kênh chất lượng trả đối tượng phẳng (không phong bì `CodeIntelEnvelope`); lỗi đọc từ tiền tố `message` + hậu tố JSON (PQ-02).
- `unknown` luôn "Chưa đủ dữ liệu để kết luận" với biểu tượng riêng; không "an toàn/sạch"; không điểm số đơn; chế độ chỉ báo (O9).
- Hai nguồn phát hiện tách (PQ-06): `Finding` (dock CR-059) và `QualityFinding`; waiver ≠ dismissal (PQ-05).
- Marker Monaco (`orca-quality`) + glyph hình học; không view zone; chỉ vẽ khi phía modified khớp worktree.
- Mọi chuỗi qua `translate()` đủ 5 locale; Electron và web; SSH (khoá nút ngay, spinner trễ); không `components/code-review/*`; không `max-lines` disable.

## Lệch tổng hợp giữa CR/README và hợp đồng/code (chi tiết ở từng solution)

1. Cảnh báo trước tạo review đã gửi: CR-087 2.10 và CR-CV-085 2.9b cùng đề xuất; hợp đồng §8.2 giao `FE-CV-SOL-085-source-control-quality-notice`. 087 **không** làm khe, chỉ cung cấp chip/deep link.
2. Kênh/tham số/kiểu của CR-087 đã thay bằng §3.2/§4.7 (cancel, danh sách profile, `worktreeId` trong push, revoke đã có; `pageToken`; hạn miễn trừ ≤ 30 ngày).
3. `useDiffCommentDecorator` được gọi ở ba nơi (`DiffViewer`, `DiffSectionItem`, `MonacoEditor`), không chỉ `DiffViewer`.
4. Phát hiện/hạn chế ở code thật: `setModelMarkers` chưa dùng ở đâu; `--warning` không định nghĩa; TDD 08 ghi "không embed Monaco" trái code.

## Hợp đồng thiếu hoặc mâu thuẫn (báo cho chủ hợp đồng; không tự sửa)

1. Không có kênh/kiểu **hotspot** và không liệt kê khoá `Finding.metrics` cho `hotspot.file`; không nguồn "độ phức tạp".
2. **Đơn vị** các số coverage/ngưỡng (0..1 hay 0..100) không nêu.
3. `QualityRun.status` không có `interrupted` nhưng `quality.finished.status` có.
4. Quy ước cột 0/1-based của `QualityFinding.column/endColumn`; ý nghĩa `column=0`.
5. Định dạng `turnKey` so với `ReviewTurnMarker.turnId`; định dạng/ngôn ngữ `observed`/`threshold` của `QualityGate.reasons[]`; `RunnableProfile.id` so với tên trong `QualityGate.profile`.
6. Không capability quyền "chạy kiểm tra" (chỉ biết qua `forbidden`).

## Cách chạy kiểm thử (chung)

- `pnpm --filter orca-frontend test -- <đường dẫn>` (script `vitest run --config config/vitest.config.ts`; môi trường `node`, thêm `// @vitest-environment happy-dom` cho component).
- Typecheck (chưa kiểm chứng): `pnpm --filter orca-frontend exec tsc --noEmit -p tsconfig.json`.
- E2E web (chưa kiểm chứng chạy CI): `tests/playwright.web.config.ts`.
- Trước khi sửa symbol có sẵn (`DiffViewer`, `DiffSectionItem`): GitNexus `impact`; trước khi commit: `detect_changes` (chưa chạy khi soạn).

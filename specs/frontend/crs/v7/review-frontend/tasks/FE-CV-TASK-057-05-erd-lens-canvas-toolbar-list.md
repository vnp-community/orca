# FE-CV-TASK-057-05: `ErdLens`: toolbar, canvas xyflow, nút bảng, danh sách ảo hoá

**From Solution:** [FE-CV-SOL-057](../solutions/FE-CV-SOL-057-erd-lens.md) mục 2.1, 2.4, 2.5, 2.7, 2.8
**Priority:** P0
**Area:** frontend / renderer components
**File:** `frontend/src/renderer/src/components/review-map/erd/{ErdLens,ErdToolbar,ErdCanvas,ErdTableNode,ErdGhostTableNode,ErdSchemaGroupNode,ErdRelationEdge,ErdColumnRow,ErdTableList,ErdLegend,ErdWarningsStrip}.tsx` (mới) + test; `review-lens-registry.ts` (SOL-051, thêm một mục đăng ký `id:'erd'`)
**Depends on:** FE-CV-TASK-057-02, 057-03, 057-04; FE-CV-SOL-051-review-workspace-shell (`ReviewLensProps`, `ReviewViewStateScreen`, `usePerceivedLoadingStage`)
**Status:** [x] DONE (verified 2026-10-07: erd/*.test.tsx 25/25 PASS, tsc/oxlint sạch)

## Context

- Chỉ dùng token `main.css`, primitive `components/ui/*` (`select`, `toggle-group`, `input`, `badge`, `tooltip`, `table`, `skeleton`, `scroll-area`), icon `lucide-react` (`KeyRound`, `Link2`, `Loader2`); không hex, không emoji. `@xyflow/react` đã có; mock như mẫu test của `TaskDAGView`.
- Màu không phải kênh duy nhất: mọi trạng thái kèm ký hiệu + nhãn chữ.

## Việc cần làm

1. `ErdLens`: ghép hook (057-04) → view model (057-02) → filter/layout (057-03); xử lý trạng thái riêng: tải (theo ngưỡng 100 ms/1 s/3 s, trì hoãn 200 ms qua SSH, khoá điều khiển ngay), service không có migration (rỗng inline), `warnings[]` (`ErdWarningsStrip`), `truncated` (backend) khác "giới hạn vẽ", rỗng sau lọc + "Xoá bộ lọc", lỗi inline có "Thử lại" (không toast), chip "Có dữ liệu mới".
2. `ErdToolbar`: service select, dialect toggle (chỉ khi > 1), schema filter, ô tìm kiếm (phím `/`), "Đổi + liền kề | Tất cả", "Đồ thị | Danh sách", hiển thị "Tính đến migration …".
3. `ErdCanvas`: `onlyRenderVisibleElements`, `nodesConnectable={false}`, không MiniMap khi > 80 nút, `fitView`/`setCenter` `duration: 0` khi reduced-motion; `nodeTypes` khai báo ngoài render (tránh tạo lại).
4. `ErdTableNode`: header (tên, schema nhạt, chip trạng thái), cột với `+ ~ −` và "trước → sau", icon PK/FK, "+N cột" (`aria-expanded`), `role="group"` + `aria-label` tổng hợp, `tabIndex=0`; Enter chọn → `selectErdTable`. `ErdGhostTableNode` bấm → `setErdService`. `ErdRelationEdge`: `fk` liền, `logical` đứt, tooltip nêu `source` (`naming` ghi "suy luận").
5. `ErdTableList`: `@tanstack/react-virtual`, chọn từ Danh sách làm nổi/đưa khung nhìn tới nút; là lối thoát bàn phím/trình đọc màn hình.
6. Phím cục bộ (`/`, `f`, `g`/`l`, `Esc`) chỉ khi tiêu điểm trong lens và `!isEditableTarget`; không đăng ký vào `KEYBINDING_DEFINITIONS`.
7. Tất cả chuỗi qua `translate()` trong thân hàm (khoá `auto.components.reviewMap.Erd*`); thêm khoá vào `i18n/code-intel-locale-coverage.test.ts` ở task 057-06.

## Kiểm thử

- Testing Library (`// @vitest-environment happy-dom`): `ErdTableNode` (có `+`/`~`/`−` khi không áp CSS màu, "+N cột" mở bằng bàn phím), `ErdToolbar` (dialect chỉ khi > 1), `ErdTableList` (ảo hoá, chọn), `ErdLens` (tải→sẵn sàng, rỗng, lỗi + thử lại, `truncated`, `warnings`, chuỗi chứa `<script>` hiển thị như văn bản).
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/erd`.

## Tiêu chí hoàn thành

- [ ] Các tiêu chí 2–8 của SOL-057 mục 5 liên quan canvas/toolbar/danh sách.
- [ ] Không import `components/code-review/*`; không thêm thư viện; không `max-lines` disable (tách file nếu > ngưỡng).
- [ ] Reduced-motion tắt chuyển vị trí/animation khung nhìn.

## Rủi ro

- Hiệu năng 150 nút và chiều cao nút thay đổi khi mở rộng (chạy lại layout cục bộ có debounce) chưa đo.

## Ghi chú triển khai (2026-10-07)

- Tạo `ErdLens`, `ErdToolbar`, `ErdCanvas`, `ErdTableNode`, `ErdColumnRow`, `ErdGhostTableNode`, `ErdSchemaGroupNode`, `ErdRelationEdge`, `ErdTableList`, `ErdLegend`, `ErdWarningsStrip`, `erd-change-style.ts`; đăng ký `load` cho lens `erd` trong `review-lens-registry.ts`.
- Sai lệch: `ErdRelationEdge.tsx` không phải component edge tuỳ chỉnh mà là hàm dựng style cạnh + nhãn nguồn (tooltip `source` nằm ở chi tiết bảng; cạnh `naming` mang nhãn "suy luận"). Danh sách chỉ ảo hoá khi > 60 dòng (test happy-dom không có layout). Chi tiết bảng hiển thị trong cột phải của lens (chưa cắm vào `SymbolDetailPanel`).
- Đăng ký lens làm hỏng `ReviewLensTabs.test.tsx` (giả định `erd` chưa có loader): đã đổi test dùng lens giả không loader.

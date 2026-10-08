# FE-CV-TASK-057-02: Mô hình hiển thị ERD và gộp thay đổi cột (hàm thuần)

**From Solution:** [FE-CV-SOL-057](../solutions/FE-CV-SOL-057-erd-lens.md) mục 2.3
**Priority:** P0
**Area:** frontend / renderer (hàm thuần)
**File:** `frontend/src/renderer/src/components/review-map/erd/erd-view-model.ts`, `erd-column-changes.ts` (mới) + `*.test.ts`
**Depends on:** FE-CV-SOL-050-types-and-runtime-bridge (kiểu `ErdModel`, `ErdChange`, `ChangeOverlay` ở `shared/code-intel-types.ts`), FE-CV-TASK-057-01
**Status:** [x] DONE (verified 2026-10-07: erd-view-model.test.ts 11/11 PASS, tsc/oxlint sạch)

## Context

- Kiểu chuẩn: UI-API §4.4 (`ErdTable`, `ErdColumn`, `ErdRelation`, `ErdChange`, `ErdModel`, `ErdServiceInfo`) và §4.3 (`TouchedTable`). Enum lạ → `'unknown'` (U4).
- `ErdChange` không có `column` = thay đổi mức bảng; `removed` ở mức cột sinh cột "ma" (cột không còn trong `columns[]` cuối).

## Việc cần làm

1. `buildErdViewModel(model, overlay)` trả `{tables, relations, ghosts, degradedToTableLevel}` theo SOL-057 2.3: gộp `ErdChange` vào cột (`added|modified|removed`, `before`), `tableChange` (`added|removed|modified|untouched|adjacent`), `accessCount {read, write}` từ `accessedBy`.
2. Bảng bị DROP (có `ErdChange` mức bảng `removed`, không còn trong `tables[]`) → bảng "ma" tối thiểu (tên + nhãn "đã xoá").
3. `changes` rỗng và `overlay.touchedTables` chứa bảng của service này → `tableChange='modified'`, `fromTouchedOnly=true`, `degradedToTableLevel=true` (không suy diễn cột).
4. Quan hệ: `id` ổn định (`from.table.cols->to.service?.table.cols`); `crossService` → sinh `ghosts[{service, table}]` khử trùng lặp; quan hệ `kind:'fk'` xuyên service vẫn vẽ nét đứt + cờ `anomaly`.
5. Mọi chuỗi tự do (`defaultExpr`, `comment`, `checks[].expr`, `rls[].*Expr`) qua `maskSensitiveRecord` (057-01) và lưu cờ `masked`.
6. Không gọi store/RPC; không import từ `components/code-review/*`.

## Kiểm thử

- Fixture nhỏ dựng tay (3 bảng, 1 FK, 1 quan hệ xuyên service): cột thêm/đổi/xoá; bảng thêm/xoá; `changes` rỗng + `touchedTables`; enum lạ; `externalRefs`; chuỗi có DSN bị che; `degraded` của bảng được giữ.
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/erd/erd-view-model src/renderer/src/components/review-map/erd/erd-column-changes`.

## Tiêu chí hoàn thành

- [ ] Hàm thuần, bất biến (không mutate input), test xanh.
- [ ] Không bao giờ tạo cột mà backend không nêu (chỉ `removed` từ `ErdChange`).
- [ ] Nhãn trạng thái luôn có ký hiệu (`+ ~ −`) trong dữ liệu trả về để component không phụ thuộc màu.

## Rủi ro

- `ErdChange` không mô tả thay đổi quan hệ; cạnh đổi chưa tô được (câu hỏi mở 1 của SOL-057).

## Ghi chú triển khai (2026-10-07)

- Tạo `erd/erd-view-model.ts`, `erd/erd-column-changes.ts`, `erd/erd-model.fixture.ts` (fixture dùng chung cho test) + `erd-view-model.test.ts`.
- Sai lệch nhỏ: bảng DROP được thêm vào `tables[]` với cờ `dropped:true` (không đặt trong `ghosts`, vì `ghosts` chỉ dành cho bảng của service khác). `adjacent` tính ở đây (không chỉ ở bước lọc). Bảng/cột lạ (`kind:'unknown'`) bị bỏ qua, không tô.

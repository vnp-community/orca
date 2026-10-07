# FE-CV-TASK-057-03: Bố cục tất định, lọc và tìm kiếm bảng ERD (không thêm thư viện)

**From Solution:** [FE-CV-SOL-057](../solutions/FE-CV-SOL-057-erd-lens.md) mục 2.4
**Priority:** P0
**Area:** frontend / renderer (hàm thuần)
**File:** `frontend/src/renderer/src/components/review-map/erd/erd-layout.ts`, `erd-table-filter.ts`, `erd-repository-links.ts` (mới) + `*.test.ts`
**Depends on:** FE-CV-TASK-057-02
**Status:** [x] DONE

## Context

- O5/O12: không thêm `elkjs`/`dagre`; bố cục tự viết, tất định để test và giữ vị trí ổn định giữa lần mở.
- Hằng (chưa đo): `ERD_FOCUS_DEFAULT_THRESHOLD=25`, `ERD_MAX_RENDERED_TABLES=150`, `ERD_COLLAPSED_COLUMN_LIMIT=12`.

## Việc cần làm

1. `layoutErdTables({tables, relations, expandedTables, groupBySchema})` → `{positions: Map<key,{x,y,width,height}>, groups}`: thành phần liên thông theo FK; BFS tầng (bảng được tham chiếu ở tầng thấp hơn); một lượt barycenter; thành phần cô lập xếp lưới dưới; chiều cao theo số cột hiển thị.
2. `erd-table-filter.ts`: `filterErdTables({tables, query, schemas, mode: 'focus'|'all', limit})` → `{visible, dimmed, truncated: {shown, total}}`; chuẩn hoá không dấu/chữ thường; khớp tên bảng/cột/`type`/`canonicalType`/tên symbol; chế độ `focus` = bảng đổi + liền kề + khớp tìm kiếm; vượt `limit` thì ưu tiên (đổi, liền kề, khớp, nhiều quan hệ) và **luôn** trả `truncated` (không cắt im lặng).
3. `erd-repository-links.ts`: `groupAccessors(table, overlay)` → nhóm `read`/`write`/`readwrite`, cờ `changed` (symbol.key ∈ `overlay.changedSymbols`), `staleAccessWarning` khi có cột `removed|modified` mà tồn tại accessor chưa đổi (nhãn "gợi ý").
4. `selectCollapsedColumns(columns, query, expanded)` theo thứ tự PK, FK, có `change`, khớp tìm kiếm, còn lại.

## Kiểm thử

- Layout: cùng input ⇒ cùng output (so sánh sâu); không chồng nút; thành phần cô lập; `expanded` đổi chiều cao; đồ thị 40 bảng không ném; chu trình FK không lặp vô hạn.
- Filter: không dấu; `limit` + thứ tự ưu tiên; `truncated` đúng; chế độ liền kề kéo bảng khớp vào tập hiển thị.
- Links: nhóm read/write; chấm "đã đổi"; cảnh báo liên đới chỉ khi dữ liệu có.
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/erd/erd-layout src/renderer/src/components/review-map/erd/erd-table-filter src/renderer/src/components/review-map/erd/erd-repository-links`.

## Tiêu chí hoàn thành

- [ ] Ba mô-đun thuần, test xanh, không phụ thuộc DOM/React.
- [ ] Không thêm dependency vào `package.json`.
- [ ] Không có animation/timer trong mô-đun.

## Rủi ro

- Chất lượng bố cục chưa thử trên dữ liệu thật; ngưỡng là đề xuất. Nếu không đủ tốt, thêm `elkjs` cần duyệt riêng (O5), ngoài task này.

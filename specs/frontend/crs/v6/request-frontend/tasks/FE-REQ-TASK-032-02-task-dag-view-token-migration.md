# FE-REQ-TASK-032-02: Chuyển `TaskDAGView` sang token màu và icon trạng thái

**From Solution:** [FE-REQ-SOL-032](../solutions/FE-REQ-SOL-032-graph-canvas-and-lenses.md) mục 2.8
**Priority:** P1
**Area:** frontend / task
**File:** `frontend/src/renderer/src/components/task/TaskDAGView.tsx` (sửa); `frontend/src/renderer/src/hooks/useDocumentColorMode.ts` (mới); `frontend/src/renderer/src/components/task/task-dag-status-presentation.ts` (mới); `frontend/src/renderer/src/components/task/__tests__/TaskDAGView.test.tsx` (sửa); test `useDocumentColorMode.test.ts`, `task-dag-status-presentation.test.ts` (mới)
**Depends on:** FE-REQ-TASK-032-01 (token `--status-success-*` đã có; token rủi ro không cần cho task này), FE-REQ-TASK-018-06 (gỡ `backlog` khỏi `TaskStatus`; nếu chưa làm thì giữ nhánh `backlog` dự phòng)
**Status:** [ ] TODO

## Context

- `TaskDAGView.tsx` (273 dòng, đã đọc): `STATUS_COLORS` dòng 26 đến 34 toàn hex (`done`, `in_progress`, `blocked`, `review`, `todo`, `backlog`, `cancelled`); `buildDAGLayout` dòng 36 đến 140 gán `style: { background, border }` từng node (dòng 111 đến 117), chữ phụ `color: '#6b7280'` (dòng 105) và cạnh `stroke: '#94a3b8'` (dòng 131). Props: `{ tasks, dependencyEdges, onSelect, onEdgeAdded }`. `onConnect` ghi qua `callRuntimeRpc` `task.addEdge`; xem phần còn lại file trước khi sửa (dòng 140 đến 273).
- `TaskDAGView.test.tsx` mock `@xyflow/react` (`ReactFlow` nhận `nodes`, `edges`, `onNodeClick`, `onConnect`) và giữ `data-testid="mock-react-flow"`; các `data-testid` hiện có phải còn nguyên.
- Token có sẵn (`main.css`): `--status-success`, `-background`, `-border`; `--primary`; `--destructive`; `--ai-action-accent`; `--muted`, `--muted-foreground`, `--border`; `--chart-3` (xanh).
- Theme: lớp `.dark` trên `document.documentElement` (xem `lib/document-theme.ts`, `THEME_TRANSITION_DISABLED_CLASS`); xyflow v12 có prop `colorMode` (`'light'|'dark'|'system'`), **chưa kiểm chứng** hành vi trong repo.
- Không hex trong file này sau khi xong là tiêu chí chấp nhận của CR-REQ-032.

## Việc cần làm

1. `useDocumentColorMode(): 'light' | 'dark'`: đọc `document.documentElement.classList.contains('dark')` làm giá trị đầu (kèm kiểm `typeof document === 'undefined'` cho SSR/test), theo dõi bằng `MutationObserver` trên `documentElement` thuộc tính `class`; dọn khi unmount. Không đọc `matchMedia` vì app điều khiển theo lớp.
2. `task-dag-status-presentation.ts` (thuần): `getTaskDagStatusPresentation(status: string): { Icon: LucideIcon; nodeClass: string; labelKey: string }` theo bảng: `done` `CircleCheck` + `bg-status-success-background border-status-success-border`; `in_progress` `Loader` + `bg-primary/10 border-primary`; `blocked` `Ban` + `border-destructive bg-destructive/10`; `review` `Eye` + `border-ai-action-accent bg-ai-action-accent/10`; `open`, `todo` `Circle` + `bg-muted border-border`; `cancelled` `CircleSlash` + `bg-muted text-muted-foreground border-border`; trạng thái lạ rơi về `open`. `backlog` nếu còn thì cùng `open` (đã gỡ ở 018-06).
3. Thay `STATUS_COLORS` bằng `getTaskDagStatusPresentation`; nội dung node là React node nhỏ (`TaskDagNodeLabel`, cùng file hoặc file riêng cùng thư mục) hiển thị icon trạng thái (`aria-hidden`) cạnh tiêu đề và nhãn `title` (`aria-label` bằng nhãn trạng thái), chữ phụ `text-muted-foreground`. Node dùng `className` của xyflow (`className: nodeClass`) thay `style` màu; `style` chỉ còn kích thước (`width: 170`, `minHeight: 50`, `borderRadius`) nếu cần.
4. Cạnh: `style: { stroke: 'var(--border)', strokeWidth: 1.5 }`; cạnh `animated` giữ cho `in_progress` nhưng tắt khi `usePrefersReducedMotion()` true.
5. Thêm `colorMode={useDocumentColorMode()}` vào `ReactFlow`; bỏ tham số màu nền cứng của `Background`/`MiniMap` nếu có (`MiniMap` dùng `nodeColor` bằng hàm trả `var(--...)` hoặc bỏ); kiểm cả khi mở ở chế độ tối.
6. Giữ nguyên `onConnect`, `addDependency`, `onNodeClick` → `onSelect`, mọi `data-testid`, hành vi chống chu trình. Không đổi `window.confirm`/`<select>` thô (ngoài phạm vi, ghi trong PR).
7. Kiểm bằng `rg -n '#[0-9a-fA-F]{3,6}' frontend/src/renderer/src/components/task/TaskDAGView.tsx` rỗng.
8. Cập nhật docs: ghi vào `guides/STYLEGUIDE.md` (nếu có mục đồ thị ở 032-01) rằng `TaskDAGView` là ví dụ token đúng; không tạo file docs mới.

## Bảng tham chiếu nhanh

| Trạng thái task | Icon | Lớp token của node |
|---|---|---|
| `done` | `CircleCheck` | `bg-status-success-background border-status-success-border` |
| `in_progress` | `Loader` | `border-primary` + nền nhạt từ `--primary` |
| `blocked` | `Ban` | `border-destructive` |
| `review` | `Eye` | `border-ai-action-accent` |
| `open`, `todo` | `Circle` | `bg-muted border-border` |
| `cancelled` | `CircleSlash` | `bg-muted text-muted-foreground border-border` |

- Kênh WS: giữ nguyên `task.addEdge` (gọi trong `onConnect`), không thêm kênh mới.
- Phím tắt: không đổi (xyflow mặc định: kéo, cuộn).
- Khoá i18n: `auto.components.task.TaskDAGView.status.{open,in_progress,blocked,review,done,cancelled}` (nhãn trạng thái cho `aria-label`); dùng khoá của `TaskStatusBadge` nếu đã có để khỏi trùng, kiểm `rg "TaskStatusBadge" frontend/src/renderer/src/i18n/locales/en.json`.
- Trạng thái UI: rỗng (không có task) giữ nguyên hành vi hiện tại của `TaskDAGView`; lỗi `task.addEdge` giữ `toast` hiện có.

## Trình tự làm gợi ý

1. Chạy `TaskDAGView.test.tsx` hiện tại để có mốc xanh trước khi sửa.
2. Viết `task-dag-status-presentation.ts` và test; viết `useDocumentColorMode` và test.
3. Thay `STATUS_COLORS`, `style` hex, `stroke` cạnh; thêm `colorMode`.
4. Bổ sung assertion không hex và test icon trạng thái; chạy lại toàn bộ test thư mục `components/task/__tests__`.
5. Kiểm tay ở chủ đề sáng và tối, ghi kết quả `var()` trong SVG vào PR.

## Kiểm thử

- `TaskDAGView.test.tsx`: giữ các test hiện có (kết nối cạnh gọi `task.addEdge` đúng tham số); thêm: không node nào có `style.background` hay `style.border` chứa `#`; mỗi trạng thái có `className` token (`done` chứa `status-success`); node hiển thị nhãn trạng thái (không chỉ màu); cạnh `style.stroke === 'var(--border)'`.
- `task-dag-status-presentation.test.ts`: mọi trạng thái trong `TaskStatus` (`import` kiểu) có presentation; trạng thái lạ trả `open`; `labelKey` tồn tại ở 5 locale (phụ thuộc 032-08 nếu gom; có thể kiểm sơ bộ bằng `en.json`).
- `useDocumentColorMode.test.ts`: đổi lớp `dark` trên `document.documentElement` làm hook cập nhật; unmount dọn observer.
- Chạy: `pnpm --filter orca-frontend test src/renderer/src/components/task/__tests__/TaskDAGView src/renderer/src/components/task/task-dag-status-presentation src/renderer/src/hooks/useDocumentColorMode`.
- Kiểm tay: mở trang Tasks chế độ DAG ở sáng và tối, chụp ảnh; chuyển chủ đề không tải lại; xác nhận biến `var(--border)` trong SVG của xyflow thật sự có màu (điểm chưa kiểm chứng của CR).

## Tiêu chí hoàn thành

- [ ] `TaskDAGView.tsx` không còn hex; `TaskDAGView.test.tsx` xanh với assertion mới.
- [ ] Mỗi node có icon và chữ trạng thái, không chỉ màu.
- [ ] Sáng/tối đổi không cần tải lại; `prefers-reduced-motion` tắt `animated`.
- [ ] `data-testid`, `onConnect`, `addDependency` không đổi hành vi.
- [ ] Kiểm tay đã ghi nhận `var()` trong SVG xyflow hoạt động (hoặc ghi lỗi vào câu hỏi mở của SOL-032).

## Rủi ro và lưu ý

- `colorMode` của xyflow chỉ đổi biến CSS nội bộ của xyflow, không đổi màu `className` Tailwind; vì vậy vẫn phải dùng token cho node.
- Nếu `bg-primary/10` (độ mờ Tailwind trên biến CSS) không hoạt động với token hex của app, dùng `color-mix` qua lớp tiện ích trong `main.css` (như `--status-success-background`); không thêm hex.
- Task này **độc lập** với bố cục tự động và `elkjs`; có thể vào trước các task 032-03 trở đi và không phụ thuộc quyết định duyệt thư viện.
- `TaskGraph.tsx` nạp `TaskDAGView` bằng `lazy`; không đổi.
- Chuyển hẳn `TaskDAGView` sang `GraphCanvas` (bước 2) chưa cam kết (câu hỏi mở 4 của SOL-032).

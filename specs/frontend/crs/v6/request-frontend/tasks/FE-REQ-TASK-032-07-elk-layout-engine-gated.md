# FE-REQ-TASK-032-07: Bố cục `elkjs` trong Worker (CHẶN bởi quyết định duyệt phụ thuộc)

**From Solution:** [FE-REQ-SOL-032](../solutions/FE-REQ-SOL-032-graph-canvas-and-lenses.md) mục 2.7, mục 1 (mâu thuẫn v7 O5)
**Priority:** P2
**Area:** frontend / graph (layout) / dependencies
**File:** `frontend/package.json` (sửa, chỉ sau duyệt); `frontend/src/renderer/src/components/graph/elk-layout-engine.ts`, `elk-layout.worker.ts`, `graph-layout-selector.ts` (mới); test cùng tên; `guides/STYLEGUIDE.md` không đổi
**Depends on:** FE-REQ-TASK-032-05 (`LayoutEngine`); **quyết định duyệt** (người có thẩm quyền phụ thuộc; pháp chế cho EPL-2.0)
**Status:** [!] BLOCKED — chặn bởi quyết định duyệt phụ thuộc `elkjs` (README v7 O5 cấm thêm ở MVP); `package.json` không đổi

## Context

- AGENTS.md: không thêm phụ thuộc khi chưa duyệt; README v7 (`/opt/repos/orca/docs/crs/v7/README.md`, mục O5, đã đọc): "Thư viện bố cục/treemap mới (`elkjs`/`dagre`, `d3-*`): Không thêm ở MVP: bố cục tầng đơn giản + treemap SVG tự viết; thêm thư viện cần duyệt riêng". CR-REQ-032 mục 2.6 đề xuất `elkjs` (phương án lùi `dagre`/`@dagrejs/dagre`). **Mâu thuẫn** đã nêu trong SOL-032 mục 1.
- Cách giải quyết: task này **không bắt buộc** cho CR-REQ-032. Lens `flow`, `plan`, `execution` và 4 lens backend chạy được bằng `waveLayoutEngine` (032-05). Task chỉ làm khi người duyệt chọn một trong ba: (A) dùng `elkjs`; (B) dùng `dagre`; (C) không dùng, giữ bố cục tầng. Nếu (C), đóng task bằng ghi chú, không sửa `package.json`.
- `frontend/package.json` hiện **không có** `elkjs`, `dagre`, `@dagrejs/dagre` (đã đọc). Vite ở `frontend/vite.config.ts` (chưa đọc chi tiết cấu hình Worker; cần đọc trước khi làm).
- Bảng so sánh (CR-032 2.6, chưa kiểm chứng ngày phát hành, giấy phép, kích thước): `elkjs` hỗ trợ đồ thị lồng và cổng cạnh, chạy Worker, EPL-2.0, bundle lớn; `dagre` nhỏ, MIT, không lồng nhau.
- Mở rộng v7: CR-CV-053 và CR-CV-055 tự dựng canvas xyflow; `LayoutEngine` giữ nguyên nên không ép v7 dùng thư viện này.

## Việc cần làm

1. **Cổng duyệt (bước 0):** trước khi cài, ghi vào PR/issue: bundle (đo `pnpm --filter orca-frontend build` trước và sau, so kích thước chunk của `GraphCanvas`), thời gian bố cục trên 50, 500, 2.000 node (đo tay bằng dữ liệu giả của task 032-05), giấy phép EPL-2.0 (pháp chế), ngày phát hành mới nhất, hỗ trợ Worker trong Electron (renderer) và build web (`vite build`). Không tiếp tục khi chưa có chữ ký duyệt.
2. Sau duyệt (A): `pnpm --filter orca-frontend add elkjs` (hoặc cách quản lý phụ thuộc của repo: kiểm `pnpm-workspace.yaml`/`pnpm-lock.yaml` ở gốc trước khi cập nhật lock); chỉ cập nhật lockfile bằng lệnh `pnpm`, không sửa tay.
3. `elk-layout.worker.ts`: nhận message `{ id: number; nodes: {id, width, height, group: string|null}[]; edges: {from, to}[]; direction: 'LR'|'TB' }`, tạo đồ thị ELK compound theo `group` (`hierarchyHandling: 'INCLUDE_CHILDREN'`, `elk.algorithm: 'layered'`), trả `{ id, positions } | { id, error }`. Dùng `import ELK from 'elkjs/lib/elk-api'` với `workerUrl`/Worker của chính file (không `elk.bundled` chạy trên luồng chính). Cùng đầu vào cho cùng đầu ra (xác định).
4. `elk-layout-engine.ts`: `export const elkLayoutEngine: LayoutEngine` bọc Worker (tạo một Worker dùng chung, kèm `terminate()` khi không còn dùng sau 60 giây); từ chối (reject) khi quá 5 giây (hằng `ELK_LAYOUT_TIMEOUT_MS`) và khi Worker lỗi.
5. `graph-layout-selector.ts`: `selectLayoutEngine(lens: GraphLens, nodeCount: number, flags): LayoutEngine` trả `elkLayoutEngine` khi lens ∈ {`architecture`, `contract`, `data`, `impact`} và `flags.elkEnabled`, ngược lại `waveLayoutEngine`; **lùi về `waveLayoutEngine`** khi `elkLayoutEngine` reject (bắt lỗi trong `GraphCanvas`, không đỏ). Cờ `elkEnabled` mặc định `false` cho tới khi duyệt xong; nạp động bằng `import()` để `elkjs` chỉ vào chunk khi dùng.
6. Cache kết quả theo `(digest/payloadId, lens, openGroupsKey)` đã có ở 032-05; không thêm cache thứ hai.
7. Sau duyệt (B): thay `elkjs` bằng `@dagrejs/dagre` đồng bộ (không Worker nếu dưới 500 node; Worker tự bọc nếu trên), bỏ hỗ trợ lồng: chỉ xếp theo sóng, xếp cột theo `group`; tài liệu hoá hạn chế này trong comment ngắn.
8. Ghi vào SOL-032 mục 7 (câu hỏi mở 1) kết luận duyệt và số đo; **không sửa README v7 hay CR-REQ-032** (nêu ở báo cáo cuối cho người điều phối).

## Bảng tham chiếu nhanh

| Phương án duyệt | Việc làm | `package.json` |
|---|---|---|
| (A) `elkjs` | 032-07 đầy đủ (Worker, `LayoutEngine`) | thêm `elkjs` |
| (B) `@dagrejs/dagre` | bố cục sóng có cụm theo cột, không lồng | thêm `@dagrejs/dagre` |
| (C) không thư viện | đóng task, giữ `waveLayoutEngine` | không đổi |

- Kênh WS: không. Khoá i18n: không. Phím tắt: không.
- Trạng thái: bố cục lỗi hoặc quá 5 giây thì lùi về `waveLayoutEngine` âm thầm (không lỗi đỏ, ghi `console.warn` có tiền tố rõ).
- Cờ: `flags.elkEnabled` mặc định `false`; bật theo cấu hình build hoặc cài đặt thử nghiệm của app (chưa xác định nơi đặt cờ, ghi vào câu hỏi mở).

## Trình tự làm gợi ý

1. Chờ văn bản duyệt (A, B hoặc C); không cài gì trước đó.
2. Nếu (A) hoặc (B): đo bundle và thời gian bố cục, đính số đo vào PR.
3. Cài thư viện bằng `pnpm`, viết Worker và `LayoutEngine` kèm test mock Worker.
4. Thêm `graph-layout-selector.ts`, bật cờ `elkEnabled` sau cùng; kiểm lùi về wave khi lỗi.
5. Kiểm bundle: thư viện nằm chunk lười; ghi tên chunk vào PR.

## Kiểm thử

- `elk-layout-engine.test.ts`: mock Worker (`vi.stubGlobal('Worker', …)`): trả vị trí đúng; timeout 5 s (fake timers) reject; lỗi Worker reject; `terminate` sau 60 giây nhàn rỗi.
- `graph-layout-selector.test.ts`: lens client luôn `waveLayoutEngine`; lens backend với `elkEnabled=false` dùng wave; `elkEnabled=true` dùng elk; elk reject thì lùi wave (kiểm ở `GraphCanvas.test.tsx` thêm ca).
- Kiểm bundle (tay, ghi số): `pnpm --filter orca-frontend build` rồi so kích thước chunk; xác nhận `elkjs` **không** nằm trong chunk chính và chỉ tải khi vào lens cần cụm (tiêu chí chấp nhận CR-032).
- Kiểm thật trong Electron: `pnpm --filter orca-frontend dev` rồi mở lens kiến trúc với dữ liệu giả; kiểm Worker khởi động, không lỗi CSP (chưa kiểm chứng CSP của app cho Worker).
- Chạy: `pnpm --filter orca-frontend test src/renderer/src/components/graph/elk-layout-engine src/renderer/src/components/graph/graph-layout-selector`.

## Tiêu chí hoàn thành

- [ ] Có văn bản duyệt (A, B hoặc C) trước khi đổi `package.json`.
- [ ] (A/B) Thư viện chỉ nằm ở chunk nạp lười; `GraphCanvas` không sụp khi Worker lỗi (lùi về wave).
- [ ] (A/B) Số đo bundle và thời gian bố cục đã ghi.
- [ ] (C) Task đóng với ghi chú, `package.json` không đổi.
- [ ] Không mâu thuẫn với README v7 O5: người duyệt đã chấp nhận ngoại lệ bằng văn bản.

## Rủi ro và lưu ý

- EPL-2.0 là giấy phép copyleft yếu theo tệp; pháp chế phải xem trước khi phát hành bản đóng gói (Electron và web).
- Vite Worker (`new Worker(new URL('./elk-layout.worker.ts', import.meta.url), { type: 'module' })`) cần kiểm cấu hình build của cả bản desktop và bản web/admin của `frontend/vite.config.ts`.
- `elkjs` bundle lớn có thể làm chậm khởi động nếu bị kéo vào chunk chính: dùng `import()` động và `React.lazy` cho `GraphCanvas` (đã có).
- SSH/remote: bố cục chạy phía client nên không thêm vòng khứ hồi; dữ liệu lớn phải cắt ở backend (`maxNodes`).
- Nếu v7 cần bố cục khác (treemap), không dùng task này.

## Ghi chú triển khai (2026-10-07)

Đóng bằng ghi chú theo spec: `LayoutEngine` cắm được đã có (032-05) nên có thể thêm `elk-layout-engine.ts` sau khi duyệt mà không đổi `GraphCanvas`. Mọi lens chạy bằng `waveLayoutEngine`.

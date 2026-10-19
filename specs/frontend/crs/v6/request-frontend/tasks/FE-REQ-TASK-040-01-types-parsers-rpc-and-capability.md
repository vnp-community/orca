# FE-REQ-TASK-040-01: Kiểu, parser, kênh RPC và cờ tính năng `request.baseRef`

**From Solution:** [FE-REQ-SOL-040](../solutions/FE-REQ-SOL-040-request-base-branch-picker.md)
**Priority:** P1
**Area:** frontend / request
**File:** `frontend/src/shared/{request-types,request-wire-parsers,request-rpc-methods}.ts`, `frontend/src/renderer/src/hooks/useRequestFlowSupport.ts`, store (slice request), test cùng thư mục
**Depends on:** Backend TASK-REQ-039-02 và 039-05
**Status:** [x] DONE (2026-10-19)

---

## Context

- `parseRequest` không bao giờ ném lỗi; chuỗi thiếu hiện thành `''` ở nhiều trường, nhưng ở đây cần `undefined` để phân biệt gateway cũ.
- `useRequestFlowSupport` thăm dò một lần mỗi target và chỉ lưu `supported|unsupported`.

## Việc cần làm

1. `OrcaRequest`: `baseRef?`, `baseRefLocked?`, `baseRefLockReason?`, `baseRefUnverified?`; `parseRequest` giữ `undefined` khi thiếu/sai kiểu.
2. `REQUEST_RPC_METHODS.SET_BASE_REF = 'request.setBaseRef'`; kiểu tham số/kết quả cho `request.create` (thêm `baseRef?`) và `request.setBaseRef`.
3. `useRequestFlowSupport`: đọc `features` từ `request.flowStatus`, ghi `requestFlowFeatures` vào store (rỗng với gateway cũ); selector `supportsBaseRef`.
4. Hàm `validateBaseRefSyntax(raw): {ok:true, value} | {ok:false, reason}` đặt trong `shared/request-base-ref.ts` (mới) với đúng quy tắc của backend (SOL-039 mục 2.2) để UI báo lỗi sớm; một nguồn cho cả dialog và Overview.
5. Kiểm sau tạo: `verifyBaseRefEcho(requested, request)` trả `mismatch` khi gateway trả `baseRef` khác đã gửi (phòng thủ chiều sâu).

## Kiểm thử

- Parser: thiếu, sai kiểu, đủ trường. `validateBaseRefSyntax`: bảng ca khớp bảng ca backend (cùng dữ liệu kiểm thử, ghi chú nguồn).
- Hook: gateway cũ (không có `features`), gateway mới có `request.baseRef`.
- `pnpm vitest run` các file đổi.

## Tiêu chí hoàn thành

- [x] `parseRequest` không ném lỗi với mọi đầu vào; `undefined` phân biệt gateway cũ.
- [x] `supportsBaseRef` đúng theo `features`.
- [x] Quy tắc cú pháp UI khớp backend.

## Rủi ro và lưu ý

- Đổi store: chạy `gitnexus_impact` cho slice request.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.

## Ghi chú triển khai (2026-10-19)

**File đổi/tạo** (đường dẫn dưới `frontend/src/`):
- `shared/request-types.ts`: `OrcaRequest.baseRef?`, `baseRefLocked?`, `baseRefLockReason?` (kiểu mới `RequestBaseRefLockReason`: `proposal|task|execution|terminal|unknown|''`), `baseRefUnverified?`.
- `shared/request-wire-parsers.ts`: `parseRequest` giữ `undefined` khi thiếu/sai kiểu (chuỗi rỗng `''` vẫn là "mặc định"); mã khoá lạ thành `'unknown'` (vẫn là đã khoá).
- `shared/request-rpc-methods.ts`: `SET_BASE_REF = 'request.setBaseRef'` (đã có trong `channel_inventory.json` của gateway; test hợp đồng xanh).
- `shared/request-base-ref.ts` (mới): `validateBaseRefSyntax`, `verifyBaseRefEcho`, hằng `REQUEST_BASE_REF_FEATURE`, kiểu `RequestCreateBaseRefParams` / `RequestSetBaseRefParams` / `RequestCreateResult`, `parseRequestCreateResult`, `parseSetBaseRefResult`.
- `renderer/src/store/slices/request.ts`: `requestFlowFeatures: ReadonlySet<string>`, `setRequestFlowFeatures`, selector `supportsBaseRef(state)` (dùng `useAppStore(supportsBaseRef)`).
- `renderer/src/hooks/useRequestFlowSupport.ts`: đọc `features`, cache theo target gồm cả features, xoá features khi đổi target (không để cờ của gateway này lọt sang gateway khác), `unsupported`/`enabled:false` thì features rỗng, lỗi mạng giữ nguyên.
- Test: `shared/request-base-ref.test.ts` (mới), `request-wire-parsers.test.ts`, `store/slices/request.test.ts`, `hooks/useRequestFlowSupport.test.ts`.

**Quy tắc cú pháp**: cài đúng thứ tự và nghĩa của `backend-go/services/request-service/internal/domain/worktree_base_ref.go`, gồm ba chỗ JS khác Go: `TrimSpace` của Go theo `unicode.IsSpace` (không cắt U+FEFF, cắt U+0085/U+00A0), `IsControl` chỉ loại Cc, độ dài tính theo byte UTF-8 sau khi cắt, surrogate lẻ = UTF-8 sai. Bảng ca trong `request-base-ref.test.ts` sao từ `worktree_base_ref_test.go` (13 hợp lệ + 34 không hợp lệ = 47, có chú thích nguồn); ca "invalid utf-8" dùng surrogate lẻ vì chuỗi JS không chứa byte 0xff. Kết quả trả về là **mã** (`reason`: `blank|too_long|...|lock_component`, thêm `character` cho ký tự cấm), không phải câu chữ: component tự dịch mã thành thông điệp qua `translate` (chưa có khoá i18n cho 17 lý do; thuộc task 03/04 dùng nó).

**Blast radius** (grep, không có GitNexus): `parseRequest` thêm 4 trường tuỳ chọn, các nơi gọi (`CreateRequestDialog`, `RequestRelatedTab`, `useRequestSummaries`, `useRequest`, `useRequestActions`, `parseBacklogItem`) không đổi; `upsertRequests` trộn `{...cũ, ...mới}` nên một phản hồi thiếu trường sẽ xoá `baseRef` đã biết (giống mọi trường tuỳ chọn khác). Slice `request`: thêm khoá, không đổi khoá cũ; `useRequestFlowSupport` được gọi từ `RequestPage` và `useRequestEvents`; ~25 file đọc `requestFlowSupport` không bị ảnh hưởng.

**Kiểm chứng đã chạy**
- `cd frontend && npx vitest run --config config/vitest.config.ts src/shared/request-base-ref.test.ts src/shared/request-wire-parsers.test.ts src/shared/request-solution-wire-parser.test.ts src/shared/request-rpc-contract.test.ts src/renderer/src/hooks/useRequestFlowSupport.test.ts src/renderer/src/store/slices/request.test.ts` xanh.
- `go test ./internal/domain -run ParseWorktreeBaseRef` (request-service) xanh, cùng bảng ca.
- Kiểm kiểu: `cd frontend && npx tsc --noEmit -p tsconfig.json --composite false --incremental false` có 114 lỗi có sẵn (admin/fleet, store/ui, test mock `AppState`...); sau thay đổi còn 113, không lỗi mới (bớt 1 lỗi `beforeEach` không dùng ở `request.test.ts`). Dùng `--incremental false` để không ghi đè `frontend/tsconfig.tsbuildinfo`.
- Lint: `npx oxlint <các file đổi>` sạch. `pnpm run lint` nguyên bản không chạy được ở gốc repo vì repo đã tách (`config/scripts/verify-localization-catalog.mjs`, `config/oxlint-switch-exhaustiveness.json` nằm ở `desktop/`), và `check:max-lines-ratchet` báo lỗi có sẵn ở các file `desktop/`/`tests/` không thuộc task này.
- i18n: `verify-localization-catalog` và `audit-localization-coverage --check` hiện **thất bại sẵn trên HEAD** (284 tham chiếu khoá thiếu trong `en.json`, 1227 chuỗi cứng; chủ yếu `web/`, `DepartmentGate`...). Đã chạy hai script `desktop/config/scripts/*` trên bản sao `src/renderer/src` ở scratchpad (script đòi `<root>/src/{main,renderer/src}`) và so sánh đầu ra trước/sau: **giống hệt**, tức không thêm khoá thiếu hay chuỗi cứng. Thêm kiểm ngang hàng: không locale nào thiếu/thừa khoá hay lệch placeholder so với `en` do thay đổi này (631 khoá thiếu ở es/ja/ko/zh là có sẵn trên HEAD).

**Ngoài phạm vi / cần người khác làm**
- `runtime/request-rate-limit.ts` chưa liệt kê `SET_BASE_REF` trong `WRITE_METHODS` (và có thể trong `IDEMPOTENT_WRITES`, vì có `expectedVersion`): hiện nó bị xếp vào lớp `read` (không bị khoá nút ghi). Không sửa vì ngoài danh sách file được giao; cần thêm khi làm task 04 (`RequestBaseRefRow`).
- `components/request/create-request-from-issue.ts` `CreateRequestParams` chưa có `baseRef?`: task 03 (`CreateRequestDialog`) nên dùng `RequestCreateBaseRefParams`.
- Chưa kiểm với gateway thật (chỉ test đơn vị với `callRequestRpc` giả); tên trường `features` trong `request.flowStatus` theo brief, chưa đối chiếu mã gateway vì phần đó đang được agent khác sửa.

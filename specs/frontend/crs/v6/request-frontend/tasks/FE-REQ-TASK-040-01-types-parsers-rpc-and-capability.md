# FE-REQ-TASK-040-01: Kiểu, parser, kênh RPC và cờ tính năng `request.baseRef`

**From Solution:** [FE-REQ-SOL-040](../solutions/FE-REQ-SOL-040-request-base-branch-picker.md)
**Priority:** P1
**Area:** frontend / request
**File:** `frontend/src/shared/{request-types,request-wire-parsers,request-rpc-methods}.ts`, `frontend/src/renderer/src/hooks/useRequestFlowSupport.ts`, store (slice request), test cùng thư mục
**Depends on:** Backend TASK-REQ-039-02 và 039-05
**Status:** [ ] TODO

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

- [ ] `parseRequest` không ném lỗi với mọi đầu vào; `undefined` phân biệt gateway cũ.
- [ ] `supportsBaseRef` đúng theo `features`.
- [ ] Quy tắc cú pháp UI khớp backend.

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

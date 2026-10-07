# AG-CV-TASK-070-01: Cây mã nguồn mẫu `mini-repo` có ca đối kháng

**From Solution:** [AG-CV-SOL-070-golden-fixtures-and-parsers](../solutions/AG-CV-SOL-070-golden-fixtures-and-parsers.md)
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/codeintel/__fixtures__/mini-repo/**` (mới), `agent/src/relay/codeintel/mini-repo-shape.test.ts` (mới)
**Depends on:** không (làm đầu tiên)
**Status:** [x] DONE

## Context

CR-070 §2.2: chụp từ repo mẫu nhỏ thay vì Orca (D1). Chỉ **mã nguồn**; không `.gitnexus/`, `.codegraph/`.
Cần: Go hexagonal (`internal/{domain,usecase,adapter}`), một `.proto`, một `migrations/*.sql`, TypeScript (component + hàm gọi kênh), một route HTTP, **hai symbol trùng tên** ở hai tệp (ambiguous, va chạm `SymbolRef.key` PQ-20), chuỗi chứa `|`, `\n`, dấu nháy, tên Unicode (NFC/NFD), tệp rỗng, tên tệp có khoảng trắng, dòng rất dài.
`agent/src/relay/codeintel/` chưa tồn tại (đã kiểm). `agent/tsconfig.json` include `./src/relay/**/*`: tệp `.ts` trong `__fixtures__` sẽ bị `tsc` duyệt, nên mã mẫu TS phải biên dịch được hoặc đặt đuôi `.ts.txt`/loại trừ (xem Rủi ro).

## Việc cần làm

1. Tạo 15 đến 25 tệp theo danh sách trên; commit thứ hai (thay đổi nhỏ) được dựng bởi script chụp, không lưu `.git/` trong repo.
2. Quyết định cách tránh `tsc`/oxlint duyệt mã mẫu: đề xuất đặt mã TS mẫu với đuôi `.ts.fixture` (đổi tên khi sao ra thư mục tạm ở task 03) và Go/SQL giữ nguyên đuôi; ghi quyết định vào `mini-repo/README.fixture.md`... không: ghi vào đầu `mini-repo-shape.test.ts`.
3. Viết `mini-repo-shape.test.ts`: đếm tệp (15 ≤ n ≤ 25), tổng ≤ 200 KiB, có hai định danh trùng tên ở hai tệp khác nhau, có chuỗi chứa `|` và xuống dòng, có tên tệp chứa khoảng trắng, không có thư mục `.gitnexus`/`.codegraph`/`.git`, không chuỗi giống token (dùng `scanTextForLeaks` của task 02 nếu đã có, không thì regex tối thiểu).

## Kiểm thử

- `mini-repo-shape.test.ts` xanh; thử xoá một ca đối kháng thì đỏ.

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run <đường dẫn test>`. Chưa chạy.

## Tiêu chí hoàn thành

- [x] Có đủ ca đối kháng; không đường dẫn tuyệt đối thật; kích thước trong ngân sách.
- [x] Không thêm phụ thuộc, không `max-lines` disable, tên tệp theo khái niệm.

## Rủi ro và lưu ý

- Chưa biết GitNexus/CodeGraph có parse được cây này không cấu hình (task 03 kiểm).
- Không đặt mã mẫu có thể bị oxlint (`pnpm lint` gốc) coi là mã thật: kiểm cấu hình ignore của oxlint ở gốc (chưa đọc).

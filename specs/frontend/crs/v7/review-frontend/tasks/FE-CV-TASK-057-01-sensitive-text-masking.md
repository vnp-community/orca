# FE-CV-TASK-057-01: Mô-đun che chuỗi nhạy cảm dùng chung (`maskSensitiveText`)

**From Solution:** [FE-CV-SOL-057](../solutions/FE-CV-SOL-057-erd-lens.md) mục 2.1, 2.6, 3
**Priority:** P0
**Area:** frontend / renderer (hàm thuần)
**File:** `frontend/src/renderer/src/components/review-map/sensitive-text-masking.ts` (mới), `sensitive-text-masking.test.ts` (mới)
**Depends on:** không
**Status:** [x] DONE (verified 2026-10-07: sensitive-text-masking.test.ts 19/19 PASS, tsc/oxlint sạch)

## Context

- Hợp đồng: `StorageMap` và `ErdModel` đã được backend che, nhưng frontend **vẫn che lớp hai** (UI-API §4.4 cuối); mọi chuỗi tự do từ backend là văn bản không tin cậy (U9).
- Đã xác minh: không có `maskSensitiveText` nào trong `frontend/src` (grep trống). Lens ERD (057) cần nó sớm nhất (đợt 3) nên mô-đun được tạo ở đây rồi SOL-058, 059, 060 dùng lại (không đặt tên `utils`).
- Nếu Phần A (SOL-050..056) đã có mô-đun che tương đương khi bắt đầu task này thì dùng lại và đóng task (câu hỏi mở 5 của SOL-057).

## Việc cần làm

1. Xuất `maskSensitiveText(text: string): { text: string; masked: boolean }` (hàm thuần, idempotent): che userinfo của URL/DSN (`scheme://user:pass@host` → `scheme://•••@host`), cặp `key=value`/`key: value` với khoá `password|passwd|secret|token|apikey|api_key|dsn|private_key|credential`, chuỗi giống token (base64/hex ≥ 32 ký tự, không khoảng trắng, **không** che UUID chuẩn và đường dẫn Vault dạng `secret/data/...`), khối `-----BEGIN … PRIVATE KEY-----` tới `END`.
2. Xuất `maskSensitiveRecord(value, keys)` che các trường chuỗi được chọn của một object (dùng cho `ErdColumn.defaultExpr`, `comment`, `checks[].expr`, `rls[].usingExpr|withCheckExpr`).
3. `masked === true` để UI hiện biểu tượng `ShieldAlert` + tooltip (việc của component gọi); mô-đun **không** `console.warn` nội dung, không ghi đâu cả.
4. Danh sách khoá/mẫu là hằng bất biến có chú thích "cần thống nhất với bộ che backend (CR-CV-072)"; không hex, không i18n trong mô-đun (chuỗi UI ở component).

## Kiểm thử

- Bảng test: DSN có mật khẩu (`postgres://u:p@h/db`), `password=abc`, JSON `{"token":"…"}`, token hex 40 ký tự, PEM nhiều dòng, URL không userinfo (không đổi), UUID (không đổi), đường dẫn Vault (không đổi), chuỗi rỗng, chuỗi 1 MB không treo (thời gian tuyến tính), chạy hai lần cho cùng kết quả.
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/sensitive-text-masking`.

## Tiêu chí hoàn thành

- [ ] Hàm thuần, không phụ thuộc React/store; test xanh.
- [ ] Không che nhầm UUID/đường dẫn Vault/hash commit 40 hex **trong ngữ cảnh commit** (ghi rõ quy tắc: chỉ che hex dài khi đứng sau khoá nhạy cảm hoặc ≥ 64 ký tự).
- [ ] Không có nhánh phụ thuộc nền tảng.

## Rủi ro

- Che thừa gây khó đọc, che thiếu gây lộ; mẫu là đề xuất, chưa đối chiếu với backend. Regex cần chống ReDoS (test dữ liệu xấu).

## Ghi chú triển khai (2026-10-07)

- Tạo `frontend/src/renderer/src/components/review-map/sensitive-text-masking.ts` (+ `.test.ts`). `maskSensitiveText` và `maskSensitiveRecord` đúng chữ ký spec; quét có chặn độ dài (không backtrack vô hạn), test 1 MB dữ liệu xấu < 5 s.
- Quy tắc hex: chỉ che hex khi đứng sau khoá nhạy cảm hoặc ≥ 64 ký tự; chuỗi base64-like ≥ 32 ký tự chỉ bị che khi có đủ chữ hoa, chữ thường và chữ số (không đụng snake_case, đường dẫn, UUID). `masked` là `output !== input` nên lần chạy thứ hai trả `masked:false` (idempotent về văn bản).
- Chạy: `cd frontend && npx vitest run --config config/vitest.config.ts src/renderer/src/components/review-map/sensitive-text-masking.test.ts`.

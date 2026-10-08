# FE-CV-TASK-055-05: `validateC4OverrideDocument`

**From Solution:** [FE-CV-SOL-055-architecture-c4-lens](../solutions/FE-CV-SOL-055-architecture-c4-lens.md) mục 4.5
**Priority:** P1
**Area:** frontend / shared
**File:** `frontend/src/shared/c4-override-document.ts` (mới), test
**Depends on:** không
**Status:** [x] DONE (verified 2026-10-07: vitest shared/c4-override-document.test.ts 10/10)

## Context

- `yaml` ^2.8.4, `zod` ~4.4.3 có sẵn (`shared/orca-yaml.ts` mẫu dùng `yaml`). `document` ≤ 64 KiB (PQ-14(5)). Schema ngữ nghĩa chưa chốt (O-12).

## Việc cần làm

1. `validateC4OverrideDocument(text): {ok; issues: {line; column; message; severity}[]; bytes}`: (1) byte UTF-8 ≤ 65 536; (2) `parseDocument` với `uniqueKeys`, `prettyErrors`, `maxAliasCount:100`, schema `core`; `linePos` ⇒ dòng/cột; (3) gốc mapping; (4) hook `c4OverrideDocumentSchema` (zod) **tắt** cho tới khi O-12 chốt, giao diện ghi "Chưa kiểm tra ngữ nghĩa; máy chủ sẽ kiểm tra khi lưu".
2. Hàm đổi `(line,column)` ⇒ độ lệch ký tự cho `Textarea`.
3. Thuần, không phụ thuộc DOM.

## Kiểm thử

- Lỗi cú pháp (dòng/cột), khoá trùng, gốc là mảng, > 64 KiB (ký tự nhiều byte), alias quá nhiều, rỗng hợp lệ.

## Tiêu chí hoàn thành

- [ ] Không ném với đầu vào bất kỳ; test xanh.

## Rủi ro

- Kích thước bundle `yaml` trong renderer chưa đo: tải lười cùng lens.

## Ghi chú triển khai (2026-10-07)

Không import `zod`: hook `c4OverrideDocumentSchema` là `null` (O-12 mở). Thêm `offsetToLineColumn`/`lineColumnToOffset`. Kích thước kiểm theo byte UTF-8 trước khi parse. `maxAliasCount` áp dụng qua `doc.toJS`. `yaml` chưa đo bundle; lens đã lazy.

# BE-CV-TASK-011-01: Re-verify từ khoá SQL (`trigger`, `view`, `at`), phiên bản MySQL/TiDB, độ dài khoá

**From Solution:** BE-CV-SOL-011-data-model-and-migrations
**Priority:** P0
**Service:** `code-intel-service`
**File:** không sửa file sản phẩm; kết quả ghi vào mô tả PR. Script DDL thử đặt ở scratchpad (ngoài repo)
**Depends on:** BE-CV-TASK-010-05 (hạ tầng integration test hai dialect)
**Status:** [x] DONE

---

## Context

CR-CV-011 mục 6 và SOL-011 C5 nêu ba điều chưa kiểm chứng sẽ làm migration `0002` vỡ nếu sai: cột `trigger` (hợp đồng T7) có thể là từ dành riêng của MySQL 8; cột `view`, `at` (T3, T5); phiên bản MySQL/TiDB mục tiêu cho `CHECK` (8.0.16+) và `UNIQUE (active_key)` nhiều `NULL`. Hợp đồng chốt tên cột, nên nếu vướng phải đề nghị chủ hợp đồng đổi (không tự đổi).

## Việc cần làm

1. Dùng container MySQL của `common/testutil/mysql.go` (hoặc image `mysql:8.0` tạm) chạy DDL thử một bảng: cột `` trigger `` **không** nháy → ghi nhận lỗi/không lỗi; có nháy → ghi nhận. Làm tương tự cho `view`, `at`, `mode`, `stage`, `outcome`, `version`, `status`.
2. Postgres: tạo bảng thử với cùng tên cột không nháy; ghi nhận.
3. Chạy `SELECT VERSION()` trên image mục tiêu; xác nhận `CHECK` được thực thi (chèn giá trị sai bị từ chối); nếu có hạ tầng TiDB, thử lại (ghi "chưa kiểm chứng" nếu không có).
4. Tạo thử các UNIQUE dài (độ dài ước tính ở SOL-011 mục 2.B: 928, 1308, 800, 800 byte) trên `utf8mb4`; xác nhận không vượt giới hạn InnoDB.
5. Chèn hai dòng `active_key = NULL` vào chỉ mục UNIQUE ở cả hai DB; xác nhận được phép; chèn hai dòng cùng `active_key` không NULL → vi phạm; ghi **số lỗi**: Postgres SQLSTATE `23505` và tên ràng buộc, MySQL `1062` và nội dung `Message` (để adapter phân biệt `active_key` với PK).
6. Kiểm `jsonb` với `\u0000` và MySQL `JSON` với UTF-8 hỏng để xác nhận hành vi bỏ NUL/`utf8.Valid` của domain là cần.
7. Ghi bảng "đã kiểm" vào PR. Nếu `` `trigger` `` vẫn bị cản: dừng task 011-03 và mở yêu cầu đổi tên cột tới chủ hợp đồng.

## Kiểm thử

- Lệnh chỉ chạy trong container tạm/testcontainers; không chạm DB dùng chung. Không `go generate`.

## Tiêu chí hoàn thành

- [x] Có bảng kết quả từng từ khoá × hai DB.
- [x] Có phiên bản MySQL đã kiểm và kết luận về `CHECK`.
- [x] Có mã/chuỗi lỗi vi phạm `active_key` cho hai dialect.

## Rủi ro và lưu ý

- Ghi rõ phiên bản image đã dùng; kết quả có thể khác TiDB.

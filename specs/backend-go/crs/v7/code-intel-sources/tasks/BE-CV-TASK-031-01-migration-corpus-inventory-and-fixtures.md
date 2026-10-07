# BE-CV-TASK-031-01: Re-verify kho `*.up.sql` và dựng corpus fixture + golden

**From Solution:** BE-CV-SOL-031-sql-migration-parser
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/sqlmigration/testdata/corpus/` (mới, file `.up.sql` trích từ repo), `.../testdata/corpus/INVENTORY.md` (mới), `.../testdata/golden/` (mới)
**Depends on:** BE-CV-TASK-030-02 (không bắt buộc; có thể làm song song)
**Status:** [x] DONE

---

## Context

CR-CV-031 mục 1 có nhiều số `grep` thô gồm cả comment, và solution phát hiện hai lệch (C1: `DO $$` ở 7 file Postgres thay vì 4; C2: `logical FK` ở 16 file Postgres và 11 file MySQL). Task đầu tiên chốt dữ liệu thật trước khi viết parser.

## Việc cần làm

1. Chạy (chỉ đọc) từ gốc repo: `ls backend-go/services/*/migrations/postgres/*.up.sql | wc -l` (kỳ vọng 151), `…/mysql/…` (143); `grep -l 'DO \$' backend-go/services/*/migrations/postgres/*.up.sql`; `grep -l 'logical FK' backend-go/services/*/migrations/*/*.up.sql`; `ls -S` lấy file lớn nhất. Ghi kết quả vào `INVENTORY.md` kèm ngày và `git rev-parse HEAD`.
2. Mở từng file trong 7 file `DO $`; phân loại: khớp thành ngữ FOREACH/format hay khác; trích khối vào `testdata/corpus/do_blocks/` (mỗi khối một file) kèm nhãn `matches_idiom: true|false`.
3. Chọn corpus đại diện (copy nguyên văn, giữ tên): `infra-fleet-service` postgres 0001, 0019, 0030, 0034 + mysql 0001, 0019; `mcp-service` postgres 0001–0003; `task-service` postgres 0001, 0003, 0014 + mysql 0001, 0012; `auth-service` postgres 0011; `ai-provider-service` mysql 0003 (ALTER nhiều khoá, comment xen). Tổng < 200 KB.
4. Liệt kê trong `INVENTORY.md` mọi biến thể chú thích `logical FK` (cả hai dialect) làm đầu vào cho mẫu nhận dạng ở BE-CV-TASK-031-08.
5. Danh sách các mệnh đề DDL **thực** (không phải comment) mà `grep` thô đếm nhầm (ví dụ `ENABLE ROW LEVEL SECURITY` trong comment MySQL): ghi lại để so với số của parser ở BE-CV-TASK-031-06.

## Kiểm thử

- Không có test mã; kiểm bằng review: `INVENTORY.md` có đủ số liệu và commit; `find backend-go/services/code-intel-service/internal/adapter/sqlmigration/testdata -type f | xargs du -ch | tail -1` < 400 KB (kiểm sau khi tạo).

## Tiêu chí hoàn thành

- [x] `INVENTORY.md` ghi 151/143/294 (hoặc số đúng mới) và phân loại 7 khối `DO`.
- [x] Corpus có đủ trường hợp ở BE-CV-SOL-031 mục 6.
- [x] Biến thể `logical FK` đủ cả hai dialect.

## Rủi ro và lưu ý

- Corpus là bản sao: cập nhật khi migration nguồn đổi; golden test chính (BE-CV-TASK-031-06) quét thẳng thư mục thật để không phụ thuộc bản sao.
- Không đưa migration chứa dữ liệu thật; tất cả đều DDL/backfill nhỏ (đã được CR xác nhận không có secret, **chưa quét lại**).

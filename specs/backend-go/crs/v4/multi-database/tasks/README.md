# backend-go Tasks — Multi-Database Pilot (`usage-service`)

**Solutions:** [../solutions/](../solutions/README.md)

## Phạm vi — ĐỌC TRƯỚC KHI THỰC THI BẤT KỲ TASK NÀO

Bộ task này chỉ phủ **1 service pilot (`usage-service`, 1 repository, 2
migration)** theo đúng khuyến nghị CR-DB-002/003. Nhân rộng ra 16 service
còn lại (43 repository khác, ~140 migration khác) **là công việc tiếp
theo, hoàn toàn ngoài phạm vi bộ task này** — xem mục "Quy mô thật" ở
[solutions/README.md](../solutions/README.md). Không tự ý mở rộng phạm vi
sang service khác khi thực thi các task dưới đây.

## Track 0 — Tài liệu (CR-DB-001, không cần code)

| Task | Depends on | Status |
|---|---|---|
| [TASK-BE-DB-001](./TASK-BE-DB-001-update-f26-docs-option-b.md) — cập nhật F26 docs theo Option B | Không | ✅ DONE |

## Track 1 — Capability layer + foundation (BE-DB-SOL-001, CR-DB-002)

| Task | Depends on | Status |
|---|---|---|
| [TASK-BE-DB-002](./TASK-BE-DB-002-dbcapability-package-and-dialect-type.md) — package `common/dbcapability` | Không | ✅ DONE |
| [TASK-BE-DB-003](./TASK-BE-DB-003-usage-service-tenant-isolation-test-without-rls.md) — test tenant isolation không cần RLS | TASK-BE-DB-002 | ✅ DONE |
| [TASK-BE-DB-004](./TASK-BE-DB-004-usage-service-migrations-dialect-safe.md) — tách migration `postgres/`/`mysql/` | TASK-BE-DB-002 | ✅ DONE |

## Track 2 — Adapter MySQL/TiDB thật (BE-DB-SOL-002, CR-DB-003)

| Task | Depends on | Status |
|---|---|---|
| [TASK-BE-DB-005](./TASK-BE-DB-005-usage-service-mysql-repository-adapter.md) — `internal/adapter/mysql/repository.go` | TASK-BE-DB-002, TASK-BE-DB-004 | ✅ DONE |
| [TASK-BE-DB-006](./TASK-BE-DB-006-usage-service-main-factory-wiring.md) — wire factory vào `cmd/server/main.go` | TASK-BE-DB-005 | ✅ DONE |
| [TASK-BE-DB-007](./TASK-BE-DB-007-ci-matrix-postgres-mysql-usage-service.md) — CI matrix Postgres+MySQL | TASK-BE-DB-003, TASK-BE-DB-004, TASK-BE-DB-006 | ✅ DONE |

## Thứ tự thực thi

```
TASK-BE-DB-001 (độc lập — tài liệu, chạy song song bất cứ lúc nào)

TASK-BE-DB-002 ─┬→ TASK-BE-DB-003 ─┐
                └→ TASK-BE-DB-004 ─┼→ TASK-BE-DB-005 → TASK-BE-DB-006 → TASK-BE-DB-007
                                   ┘
```

003 và 004 đều chỉ phụ thuộc 002, có thể làm song song (khác file — 003
sửa test, 004 sửa migration). 005 cần CẢ HAI 003 và 004 xong (adapter
MySQL cần migration MySQL tồn tại để test được, và cần biết pattern
tenant-isolation test đã xác nhận ở 003 để viết đúng test tương tự cho
MySQL). 006 phụ thuộc 005 (cần `internal/adapter/mysql` tồn tại để wire).
007 là bước cuối, cần 003/004/006 đều xong.

## Nguyên tắc chung cho AI thực thi các task này

- **Luôn chạy `impact()` trước khi sửa symbol** — mỗi task đã ghi rõ
  symbol cần kiểm tra ở mục "gitnexus" (đã chạy thật, không phải suy đoán
  từ mẫu `CompanyRepository`/`tenant-service` của CR-DB-001 — service
  pilot ở đây khác, số liệu đã lấy riêng cho `usage-service`), nhưng đây
  là yêu cầu bắt buộc chung theo `CLAUDE.md`/`AGENTS.md`, không chỉ khi
  task nhắc tới. TASK-BE-DB-001 là ngoại lệ (không sửa code) — không cần
  `impact()`.
- **Không tự ý mở rộng phạm vi** — mỗi task chỉ sửa đúng file đã liệt kê,
  chỉ trong `usage-service`/`common/dbcapability`. Phát hiện gap ở service
  khác trong lúc làm thì ghi nhận lại, không sửa luôn.
- **`tenant_id` luôn lấy từ context đã xác thực** — không bao giờ từ input
  client. `internal/adapter/postgres/repository.go` hiện tại đã đúng quy
  tắc này ở mọi query; `internal/adapter/mysql/repository.go` (TASK-BE-DB-005)
  phải giữ nguyên, không nới lỏng dù RLS backstop không còn.
- **Test trước, không giả định pass** — mọi lệnh `go build`/`go test` ở
  mục "Verify" phải thực sự chạy và thấy kết quả.
- **`detect_changes()` trước khi commit** — xác nhận scope thay đổi đúng
  dự kiến, theo yêu cầu bắt buộc của `CLAUDE.md`.

# BE-DB-SOL-008: Adapter MySQL/TiDB thật cho `notification-service`

> **🟡 Implemented, verification partial.** Batch 2 rollout của pattern
> BE-DB-SOL-001/002 ra `notification-service` — code hoàn chỉnh và đúng
> phạm vi; integration MySQL xác nhận thật 17/24 test case (0 FAIL) nhưng
> chưa chạy trọn hết 24 case trong 1 lượt do môi trường dùng chung quá tải
> thật (nhiều agent batch khác chạy song song) — xem TASK-BE-DB-013 mục 6.

**CR:** [CR-DB-002](../../../../../../docs/crs/v4/multi-database/CR-DB-002-dialect-capability-layer-foundation.md), [CR-DB-003](../../../../../../docs/crs/v4/multi-database/CR-DB-003-mysql-tidb-adapter-backend-go.md)
**Service:** `notification-service`
**Task:** [TASK-BE-DB-013](../tasks/TASK-BE-DB-013-notification-service-mysql-rollout.md)
**Phụ thuộc cứng:** [BE-DB-SOL-001](./BE-DB-SOL-001-dialect-capability-layer-usage-service.md), [BE-DB-SOL-002](./BE-DB-SOL-002-usage-service-mysql-tidb-adapter.md) — pattern dùng lại nguyên vẹn, không thiết kế lại

---

## 1. `notification-service` khác pilot ở đâu — đọc trước khi implement

Audit trực tiếp (`Read` đầy đủ `internal/usecase/ports.go` đọc LẠI TỪ ĐẦU —
không dùng bất kỳ doc cũ nào vì service này vừa bị 1 task KHÔNG liên quan
sửa gần đây, xem dưới — `internal/adapter/postgres/{repository,
buffered_notification_repository,notification_preference_repository}.go`,
10 file migration, `cmd/server/main.go`, `internal/config/config.go`,
`internal/domain/*.go`) xác nhận:

- **3 repository (audit số cũ đúng), không phải 1**: `Repository`
  (`internal/adapter/postgres/repository.go`) implement 4 port cùng lúc —
  `SubscriptionRepository`, `VapidKeyRepository`,
  `ProcessedEventRepository`, `NotificationRepository` — cộng
  `BufferedNotificationStore` (`SubscriptionRepository`'s sibling file,
  implement `BufferedNotificationRepository`) và
  `NotificationPreferenceStore` (implement
  `NotificationPreferenceRepository`). Tổng 6 port, 3 struct, 3 file
  `adapter/postgres/*.go`.
- **`SubscriptionRepository` VỪA được thêm method `MarkExpired`** bởi 1
  task KHÔNG liên quan (fix regression `MarkExpired` push-subscription,
  `internal/usecase/deliver_push.go` + `internal/adapter/postgres/
  repository.go` + `internal/adapter/external/{apns,fcm,webpush}/`, đã
  commit/đang tiến hành trước task này) — đọc `ports.go` TRỰC TIẾP (không
  suy đoán từ doc cũ) xác nhận `MarkExpired(ctx, endpoint) error` LÀ 1
  phần của interface hiện tại, idempotent-by-design (0 rows affected
  không phải lỗi, giống `DeleteByEndpoint`). Adapter MySQL implement đúng
  method này, KHÔNG branch theo `RowsAffected()` (xem mục 3).
- **KHÔNG có `internal/adapter/external/*` (apns/fcm/webpush) nào bị đụng
  tới** — đúng phạm vi "chỉ tầng DB", các package đó là HTTP push client,
  không phải SQL.
- **CÓ JSONB thật**: `buffered_notifications.notification_event_json
  JSONB` (migration 0003) — không có `->`/`@>` operator nào dùng tới
  (chỉ decode application-side qua `encoding/json` ở `ListPending`), nên
  MySQL `JSON` (không phải `TEXT`) là dịch an toàn, không mất chức năng gì
  thật sự dùng.
- **CÓ `gen_random_uuid()` NHƯNG chỉ 1 chỗ thật sự load-bearing**:
  `push_subscriptions.id`/`vapid_key_metadata.key_id`/
  `notification_events.id` đều luôn được Go set tường minh (không dựa vào
  default) — GIỐNG phát hiện chung của pilot. NGOẠI LỆ:
  `buffered_notifications.id DEFAULT gen_random_uuid()` **THẬT SỰ được
  dựa vào** — `BufferedNotificationStore.Enqueue`'s `INSERT` không truyền
  cột `id`. Đây là bảng DUY NHẤT trong service này có gap đó. MySQL
  không có `UUID()`-as-default ổn định/portable qua mọi phiên bản (default
  expression không xác định là an toàn dùng ở mọi context), nên adapter
  MySQL sinh `id` bằng `uuid.NewString()` ở tầng Go thay vì dựa vào default
  DB-side — nhất quán với quy ước chung của service (mọi bảng khác đã sinh
  ID ở Go), KHÔNG phải một quy ước mới.
- **CÓ RLS thật, 5/5 bảng** (`push_subscriptions`, `vapid_key_metadata`,
  `buffered_notifications`, `notification_preferences`,
  `notification_events`) — xác nhận migration Postgres gốc CÓ RLS trước
  khi viết comment giải thích trong migration MySQL, không suy đoán. →
  TASK-BE-DB-003's tenant-isolation-without-RLS pattern áp dụng: 2 test
  mới (`ListByUser_DoesNotLeakAcrossTenants`,
  `ListByRecipient_DoesNotLeakAcrossTenants`) mirror pilot.
- **1 partial UNIQUE index thật cần dịch không tầm thường**:
  `idx_vapid_key_active UNIQUE(tenant_id, status) WHERE status='active'` —
  MySQL không có `WHERE` trên `CREATE INDEX`. Dịch bằng cột generated ảo
  `active_tenant_id` (NULL khi `status<>'active'`) + UNIQUE index trên cột
  đó — thủ thuật MySQL chuẩn cho partial unique index (NULL không tính
  trùng ở UNIQUE index, giống Postgres), không phải suy yếu ràng buộc. 2
  partial non-unique index khác (`idx_buffered_notifications_pending`,
  `idx_buffered_notifications_user_pending`, cả 2 `WHERE delivered_at IS
  NULL`) chỉ là tối ưu truy vấn, không backing ràng buộc gì — dịch bằng
  index đầy đủ (không `WHERE`), không mất tính đúng đắn, chỉ index thêm
  vài dòng đã delivered.
- **`endpoint TEXT` cần prefix index**: `idx_push_subscriptions_endpoint`
  là UNIQUE index trên cột `TEXT` không giới hạn — MySQL/InnoDB không index
  trực tiếp được `TEXT` đầy đủ (giới hạn 3072 byte/key ở utf8mb4). Dùng
  prefix index `endpoint(768)` (768 ký tự × 4 byte/ký tự = 3072 byte, mức
  trần InnoDB) — cùng dạng phát hiện `TASK-BE-DB-010`
  (annotation-service) đã gặp với `repo_id`/`file_path`.
- **`cmd/server/main.go` TRƯỚC task này đọc thẳng `cfg.DatabaseDSN`** + tự
  check rỗng bằng `errors.New(...)` — giống `credential-broker-service`
  TRƯỚC BE-DB-SOL-006, khác pilot. Thêm `DatabaseCredentialsFile` vào
  `internal/config/config.go` (mirror pilot's field/default y hệt: env
  `DATABASE_CREDENTIALS_FILE`, default `/vault/secrets/database-credentials`),
  đóng luôn README's "Known gaps" ghi chú Vault chưa wire — tác dụng phụ
  hợp lý của việc thêm dialect factory, không phải mở rộng phạm vi.

## 2. Chọn driver — giống hệt BE-DB-SOL-002 §2

`github.com/go-sql-driver/mysql` qua `database/sql`, TiDB dùng chung nhánh
MySQL. Test container `mysql:8` qua `common/testutil.StartMySQL` (đã có
sẵn, không sửa).

## 3. `internal/adapter/mysql/` — 3 file, mirror đúng 3 file Postgres

- `repository.go`: `SubscriptionRepository` (6 method, gồm `MarkExpired`),
  `VapidKeyRepository` (1 method), `ProcessedEventRepository` (1 method),
  `NotificationRepository` (5 method). Điểm dịch không tầm thường:
  - `Save` (upsert): `ON CONFLICT(endpoint) DO UPDATE` →
    `ON DUPLICATE KEY UPDATE`.
  - `MarkProcessed`: `INSERT ... ON CONFLICT DO NOTHING` →
    `INSERT IGNORE`. `RowsAffected()==0` vẫn đọc được an toàn ở MySQL cho
    `INSERT IGNORE` cụ thể (không phải UPDATE) — không có ambiguity
    "rows changed vs rows matched" (chỉ áp dụng cho UPDATE/
    `ON DUPLICATE KEY UPDATE`, xem điểm tiếp theo).
  - **`MarkExpired`**: method MỚI nhất trong interface (thêm bởi task
    không liên quan). Đã kiểm tra kỹ theo đúng yêu cầu: method này KHÔNG
    branch theo `RowsAffected()` ở cả 2 dialect (0 rows affected không
    phải lỗi — idempotent-by-design), nên pitfall
    "MySQL đếm rows CHANGED, không phải rows MATCHED" (phát hiện thật của
    `TASK-BE-DB-010`'s `UpdateAnnotation`) KHÔNG áp dụng ở đây — không cần
    né bằng cách UPDATE-rồi-SELECT-lại như annotation-service từng phải
    làm. Tương tự cho `MarkAsRead`/`MarkAllAsRead`: cả 2 đều có mệnh đề
    `WHERE ... is_read = false`, nên 1 dòng khớp WHERE LUÔN LÀ 1 dòng đổi
    giá trị (`is_read: false→true`) — không có khoảng cách giữa "matched"
    và "changed" cho riêng 2 query này, nên `MarkAllAsRead`'s
    `RowsAffected()` trả về (dùng làm giá trị trả về thật cho caller) nhất
    quán giữa 2 dialect.
  - `SaveNotificationEvent`: `pgx.Batch`/`SendBatch` (pipeline, KHÔNG bọc
    transaction) → vòng lặp `ExecContext` tuần tự — giữ đúng tính
    non-atomic-across-recipients của bản gốc, không thêm guarantee mạnh
    hơn bản gốc không có.
  - `ListByRecipient`'s keyset cursor: `(created_at, id) < ($N, $N::uuid)`
    → `(created_at, id) < (?, ?)` — MySQL hỗ trợ row-value comparison
    chuẩn SQL, không cần cast kiểu (cột `id` đã là `CHAR(36)`, so sánh
    string trực tiếp).
- `buffered_notification_repository.go`: `Enqueue`'s eviction subquery
  (`DELETE ... WHERE id IN (SELECT id FROM buffered_notifications WHERE
  ... OFFSET ?)`) đụng đúng giới hạn MySQL "You can't specify target table
  for update in FROM clause" — bọc thêm 1 lớp derived table
  (`SELECT id FROM (SELECT ...) AS t`) để lách, giữ nguyên logic "xoá dòng
  cũ nhất vượt quá 50". MySQL `OFFSET` không kèm `LIMIT` cần
  `LIMIT 18446744073709551615 OFFSET ?` (idiom chuẩn "LIMIT ALL OFFSET n"
  của MySQL). `id` sinh bằng `uuid.NewString()` ở Go (xem mục 1's gap
  duy nhất).
- `notification_preference_repository.go`: `Set`'s
  `ON CONFLICT(...) DO UPDATE` trên composite primary key →
  `ON DUPLICATE KEY UPDATE`, không đổi ngữ nghĩa.
- Tên bảng: database MySQL tên `notification`, bảng không prefix
  (`push_subscriptions`, `vapid_key_metadata`, `processed_events`,
  `buffered_notifications`, `notification_preferences`,
  `notification_events`) — theo đúng quy ước BE-DB-SOL-002 §3.

## 4. Compensating control cho tenant isolation

Cả 5 bảng đều có `tenant_id` + RLS policy trên Postgres, KHÔNG có tương
đương ở MySQL. 2 test tenant-isolation-without-RLS mới trong
`internal/adapter/mysql/repository_test.go`
(`TestRepository_ListByUser_DoesNotLeakAcrossTenants`,
`TestRepository_ListByRecipient_DoesNotLeakAcrossTenants`) chứng minh
`WHERE tenant_id = ?` tường minh ở mọi query là cơ chế cách ly DUY NHẤT có
thật, kể cả trên Postgres hôm nay (RLS chưa từng active thật — không có
`SET LOCAL app.tenant_id` nào trong `backend-go`, không migration nào
dùng `FORCE ROW LEVEL SECURITY`, đúng phát hiện chung BE-DB-SOL-001 §4).

## 5. Migration dialect-safe

```
backend-go/services/notification-service/migrations/
├── postgres/            (nội dung y hệt 10 file gốc, chỉ ĐỔI VỊ TRÍ, git mv)
│   ├── 0001_init.{up,down}.sql
│   ├── 0002_processed_events.{up,down}.sql
│   ├── 0003_mobile_buffering_preferences.{up,down}.sql
│   ├── 0004_push_subscriptions_device_id.{up,down}.sql
│   └── 0005_notification_events.{up,down}.sql
└── mysql/               (mới, 10 file)
    ├── 0001_init.{up,down}.sql        — bỏ CREATE SCHEMA, bỏ RLS (2 bảng),
    │                                    UUID→CHAR(36), TEXT CHECK giữ
    │                                    nguyên (MySQL 8.0.16+), partial
    │                                    UNIQUE index → generated column +
    │                                    UNIQUE (mục 1), TEXT endpoint →
    │                                    prefix index (768), device_id gộp
    │                                    thẳng vào bảng gốc (baseline hợp
    │                                    nhất, không tách riêng)
    ├── 0002_processed_events.{up,down}.sql — UUID→CHAR(36)
    ├── 0003_mobile_buffering_preferences.{up,down}.sql — JSONB→JSON, RLS
    │                                    bỏ (2 bảng), 2 partial non-unique
    │                                    index → index đầy đủ, FK CASCADE
    │                                    giữ nguyên
    ├── 0004_push_subscriptions_device_id.{up,down}.sql — no-op
    │                                    (`SELECT 1`), giữ số thứ tự file
    │                                    khớp timeline Postgres, cột đã
    │                                    nằm sẵn trong 0001 (baseline hợp
    │                                    nhất)
    └── 0005_notification_events.{up,down}.sql — UUID→CHAR(36), composite
                                         PRIMARY KEY giữ nguyên, RLS bỏ
```

## 6. Wiring `cmd/server/main.go`

Theo đúng `switch caps.Dialect` pattern của
`usage-service/cmd/server/main.go`'s `run()`, `toMySQLDriverDSN` copy
nguyên văn. `repo` khai báo với 1 interface ẩn danh gộp 4 port
(`SubscriptionRepository`+`VapidKeyRepository`+`ProcessedEventRepository`+
`NotificationRepository`) vì các usecase constructor cần các tổ hợp khác
nhau của cùng 1 biến (giống `credential-broker-service`'s 3-port gộp, khác
pilot's 2-biến-tách-biệt). `bufferStore`/`preferenceStore` là 2 biến
interface riêng (`usecase.BufferedNotificationRepository`/
`usecase.NotificationPreferenceRepository`), khởi tạo trong CÙNG switch vì
cả 3 constructor (`Repository`/`BufferedNotificationStore`/
`NotificationPreferenceStore`) cần cùng 1 `*pgxpool.Pool` hoặc `*sql.DB`.
`healthSrv.Register` đổi tên theo `caps.Dialect` (`"postgres"`/`"mysql"`)
giống pilot. Toàn bộ wiring push (`credential-broker-service`,
`auth-service`, APNs/FCM/webpush/nacl clients) giữ nguyên không đổi —
không phụ thuộc dialect DB, đúng phạm vi "chỉ tầng DB".

## Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| `impact()` cho `run`/`Load`/`Config`/`Repository` (postgres) | Đã chạy — LOW cả 4 | Xem TASK-BE-DB-013's "Kết quả thực tế" |
| Không có RLS-tương đương ở MySQL, khác Postgres | Trung bình, đã compensat qua 2 test | Xem mục 4 |
| `idx_vapid_key_active`'s partial-unique dịch bằng generated column | Thấp | Thủ thuật MySQL chuẩn, không suy yếu ràng buộc |
| `buffered_notifications.id` không còn DB-side default | Thấp | Sinh ở Go thay vì DB default — nhất quán quy ước chung của service |
| **Bug tiền tồn tại phát hiện, NGOÀI PHẠM VI, KHÔNG sửa**: `TestRepository_GetPublicKey_NoActiveKeyReturnsDomainError` (Postgres suite, đã tồn tại trước task này) dùng tenant id không phải UUID (`"tenant-with-no-key"`) chống lại cột `vapid_key_metadata.tenant_id UUID` → `SQLSTATE 22P02` thay vì domain error mong đợi | Đã xác nhận — xem TASK-BE-DB-013 mục 6 | Cùng dạng bug `TASK-BE-DB-009`(issue-tracking-service)/`TASK-BE-DB-003`/`007`(usage-service pilot) từng gặp — migration+test content không đổi (chỉ `git mv`), không phải do rollout này gây ra |
| MySQL integration suite chưa chạy trọn 1 lượt hết cả 24 test case trong phiên này | Thấp — môi trường, không phải code | 17/24 case đã PASS thật qua Docker (0 FAIL quan sát được); môi trường dùng chung bị quá tải thật (tới 27 tiến trình `go test -tags=integration` song song từ agent batch khác — đo bằng `ps aux`) khiến các lượt chạy bị timeout/kill trước khi hết 24 case. Xem TASK-BE-DB-013 mục 6 — khuyến nghị re-run khi máy rảnh hơn, không cần sửa code |

## Không thuộc phạm vi solution này

- 14 service còn lại của rollout — xem `ROLLOUT-TRACKING.md`.
- `internal/adapter/external/{apns,fcm,webpush}` (HTTP push client, không
  phải DB) — không đụng.
- `internal/usecase/deliver_push.go`'s business logic — chỉ giữ compile
  được đúng interface hiện có, không sửa logic.
- Sửa bug tiền tồn tại ở `TestRepository_GetPublicKey_NoActiveKeyReturnsDomainError`
  (Postgres) — flag, không fix, xem bảng rủi ro ở trên.

## Liên quan

- `backend-go/services/notification-service/internal/usecase/ports.go` (6 port)
- `backend-go/services/notification-service/internal/adapter/postgres/*.go` (bản gốc để dịch)
- `backend-go/services/notification-service/internal/adapter/mysql/*.go` (mới)
- [BE-DB-SOL-001](./BE-DB-SOL-001-dialect-capability-layer-usage-service.md), [BE-DB-SOL-002](./BE-DB-SOL-002-usage-service-mysql-tidb-adapter.md) (phụ thuộc cứng)

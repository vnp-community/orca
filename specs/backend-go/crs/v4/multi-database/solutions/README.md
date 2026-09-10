# backend-go Solutions — Multi-Database (v4)

**CRs:** [docs/crs/v4/multi-database/](../../../../../../docs/crs/v4/multi-database/README.md)
**TDD tham chiếu:** [`04-tech-stack.md`](../../../../tdd/architecture/04-tech-stack.md) §Data access, [`05-data-architecture.md`](../../../../tdd/architecture/05-data-architecture.md)

## Đánh giá trạng thái hiện tại (bắt buộc trước khi thiết kế — theo yêu cầu)

`docs/crs/v4/multi-database/CR-DB-001` đã **chốt quyết định chính thức**
(2026-09-09): **Option B — `backend-go` sẽ hỗ trợ multi-dialect thật**
(Postgres + MySQL/TiDB), ghi đè khuyến nghị Option A ban đầu của audit. Đây
không còn là "chờ quyết định" — CR-DB-002 và CR-DB-003 đã kích hoạt.

Re-verify thực tế trước khi thiết kế (đọc file thật, không suy đoán):

- `backend-go/common/dbcapability/` **chưa tồn tại** — `find` xác nhận 0
  kết quả, đây là package hoàn toàn mới.
- `backend-go/common/testutil.StartPostgres` tồn tại, **không có
  `StartMySQL`** tương ứng — CI matrix (CR-DB-002/003) phải thêm mới.
- **Không có CI pipeline nào cho `backend-go`** — `backend-go/Makefile`'s
  `opa-test` target tự ghi chú: *"Not actually wired into a CI pipeline
  here — none exists yet for backend-go at all"*. `.github/workflows/`
  không có file nào liên quan `backend-go`/`testcontainers`. Đây là gap có
  sẵn, không phải do bộ CR này gây ra — solution/task dưới đây coi CI
  matrix là **tạo mới**, không phải "mở rộng CI có sẵn".
- `ORCA_DB_URL` (biến env CR-DB-002/003 gốc đề xuất) **không tồn tại** ở
  `backend-go` — biến thật là `DATABASE_DSN` (`backend-go/common/config/config.go:27,49`),
  đọc qua `secrets.DatabaseCredentialsFromFile` (ưu tiên file Vault-Agent
  render, fallback `DATABASE_DSN` env). Hàm này **đã dialect-agnostic từ
  trước** — chỉ đọc/trả về 1 chuỗi DSN, không quan tâm scheme
  `postgres://` hay `mysql://`. Thiết kế dưới đây **dùng lại** cơ chế này,
  không thêm biến env mới.

### Service pilot: `usage-service`

CR-DB-002/003 đề xuất `usage-service` hoặc `annotation-service` (2 service
traffic thấp nhất theo `05-data-architecture.md:17`). So sánh thực tế qua
Read:

| | `usage-service` | `annotation-service` |
|---|---|---|
| Số migration | 2 (`0001_init`, `0002_outbox`) | 4 |
| Port trong `ports.go` | 1 (`Repository`) | 2 (`Repository`, `OPAClient`) |
| File `adapter/postgres/` | 1 (`repository.go`) | 1 (`repository.go`) |
| `gen_random_uuid()` trong migration | Không dùng — ID sinh ở Go (`uuid.NewString()`, `internal/usecase/record_usage_session.go:83`) | Chưa audit — ngoài phạm vi bộ CR này |
| `JSONB` | 1 cột (`usage.outbox_events.payload`) | Chưa audit |
| RLS | 2 policy (`usage.sessions`, `usage.daily_rollups`) | Chưa audit |

→ **Chọn `usage-service`** — bề mặt nhỏ hơn (không có `OPAClient` port thứ
2, ít migration hơn, không phụ thuộc `gen_random_uuid()`), rủi ro thử
nghiệm thấp nhất cho pilot đầu tiên.

Impact analysis thật đã chạy cho đúng symbol của `usage-service` (không
lặp lại mẫu `CompanyRepository`/`tenant-service` của CR-DB-001):

| Symbol | Direction | Risk | Impacted | Ghi chú |
|---|---|---|---|---|
| `Repository` (interface, `usage-service/internal/usecase/ports.go`) | upstream | LOW | 2 (1 direct) | usecase layer chỉ phụ thuộc interface — thêm implementation MySQL không đổi caller |
| `New` (constructor, `usage-service/internal/adapter/postgres/repository.go`) | upstream | LOW | 2 (1 direct, module `Usecase`) | Điểm gọi duy nhất là `cmd/server/main.go` — factory hoá không phá caller khác |

## Solutions

| Solution | CR | Service | Status |
|---|---|---|---|
| [BE-DB-SOL-001](./BE-DB-SOL-001-dialect-capability-layer-usage-service.md) | CR-DB-002 | `common/dbcapability` (mới), `usage-service` | 🔲 Designed — chưa implement |
| [BE-DB-SOL-002](./BE-DB-SOL-002-usage-service-mysql-tidb-adapter.md) | CR-DB-003 | `usage-service` | 🔲 Designed — chưa implement |

CR-DB-001 **không có solution ở đây** — đây là gap-analysis + quyết định
kiến trúc thuần tài liệu, đã chốt xong (Option B, 2026-09-09), không sinh
ra thiết kế code nào của riêng nó (giống cách
[automations' README](../../automations/solutions/README.md) giải thích
CR-AUTO-006 không có solution vì không thuộc phạm vi thiết kế code
backend-go). Việc còn lại của CR-DB-001 là cập nhật 3 file tài liệu —
xem [TASK-BE-DB-001](../tasks/TASK-BE-DB-001-update-f26-docs-option-b.md).

## Thứ tự implement

```
BE-DB-SOL-001 → làm trước — capability interface + factory + compensating
                tenant-isolation control + migration dialect-safe cho
                usage-service. Không adapter MySQL thật ở bước này.
BE-DB-SOL-002 → phụ thuộc CỨNG vào 001 (cần Capabilities/factory tồn tại
                trước khi viết adapter MySQL thật) — implement
                internal/adapter/mysql/ cho usage-service + wire factory
                + CI matrix.
```

## Quy mô thật — KHÔNG che giấu (bắt buộc nêu rõ theo yêu cầu)

Đây là **effort XL nếu nhân rộng toàn bộ**: CR-DB-002 tự ước tính 44 file
`adapter/postgres/*.go` × 17 service × 142 file migration cần port hoặc
viết dialect-safe nếu áp dụng cho mọi service. Bộ solution/task này **chỉ
phủ 1 service pilot (`usage-service`, 1 repository, 2 migration)** —
nhân rộng ra 16 service còn lại (43 repository khác, ~140 migration khác)
là công việc **hoàn toàn ngoài phạm vi** bộ tài liệu này, cần lặp lại đúng
quy trình (capability layer dùng chung đã có sẵn từ BE-DB-SOL-001, nhưng
từng service vẫn cần: audit lock-in riêng, viết adapter riêng, migration
dialect-safe riêng, `impact()` riêng cho từng constructor/interface trước
khi sửa).

## Nguyên tắc bảo mật xuyên suốt (kế thừa từ nhóm CR-STORAGE/CR-EVM/CR-AUTO)

`tenantID` cho mọi query mới ở `internal/adapter/mysql/` **luôn lấy từ
context đã xác thực** (propagate từ `api-gateway` qua gRPC metadata), y hệt
cách `internal/adapter/postgres/repository.go` hiện tại đã làm (mọi query
của nó đã có `tenant_id = $1` tường minh — không phải thêm mới, chỉ phải
**giữ nguyên** khi viết adapter dialect thứ 2). Khi dialect ≠ postgres,
RLS backstop biến mất — application-layer scoping đang có sẵn trở thành
**cơ chế duy nhất**, không còn "chính + backstop" — xem BE-DB-SOL-001 §3
và [TASK-BE-DB-003](../tasks/TASK-BE-DB-003-usage-service-tenant-isolation-test-without-rls.md).

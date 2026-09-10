# TASK-BE-DB-006: Wire dialect factory vào `usage-service/cmd/server/main.go`

**Solution:** BE-DB-SOL-001 §3 | **CR:** CR-DB-003
**Service:** `usage-service`
**Depends on:** TASK-BE-DB-005 (`internal/adapter/mysql` phải tồn tại)
**Status:** ✅ DONE (2026-09-09)

> **Kết quả thực tế:** `impact()` đã chạy thật —
> `mcp__gitnexus__impact({target:"run", direction:"upstream",
> file_path:"backend-go/services/usage-service/cmd/server/main.go"})` →
> risk **LOW**, impactedCount 1 (chỉ `main()` cùng file) — khớp đúng số
> liệu task doc đã ghi.
>
> Đã sửa `run()` đúng theo khung task doc: `dbcapability.DetectDialectFromDSN(dsn)`
> chọn nhánh Postgres/MySQL, mỗi nhánh tự tạo pool/db + đăng ký health
> checker riêng (`"postgres"`/`"mysql"`). **Lệch so với khung ban đầu**:
> biến `repo` (kiểu `usecase.Repository`) KHÔNG implement `common/outbox.Store`
> (thiếu `FetchUnpublished`/`MarkPublished` — 2 method đó chỉ có trên từng
> struct cụ thể `*usagepostgres.Repository`/`*usagemysql.Repository`, không
> có trong interface `usecase.Repository`) — code mẫu gốc gọi thẳng
> `outbox.NewRelay(repo, ...)` sẽ KHÔNG BUILD. Đã thêm biến thứ 2
> `outboxStore outbox.Store`, gán song song với `repo` trong cả 2 nhánh
> switch (`repo, outboxStore = pgRepo, pgRepo` / `myRepo, myRepo`), và đổi
> `outbox.NewRelay(repo, ...)` → `outbox.NewRelay(outboxStore, ...)`.
>
> Viết đầy đủ `toMySQLDriverDSN` (chưa có thân hàm trong solutions doc, task
> này viết mới hoàn toàn) — **phát hiện quan trọng khi implement**: gợi ý
> ban đầu "dùng `net/url.Parse`" KHÔNG hoạt động với chính DSN mẫu mà task
> này dùng ở mục Verify (`mysql://root:orca@tcp(localhost:3307)/usage`) —
> đã xác nhận thực nghiệm `net/url.Parse` trả lỗi `invalid port ":3307)"
> after host` vì dấu ngoặc đơn trong `tcp(...)` không phải cú pháp URL hợp
> lệ. Hàm viết lại xử lý 2 dạng: (1) host đã bọc sẵn `tcp(host:port)` — chỉ
> cắt tiền tố scheme, dùng nguyên vẹn (đây là dạng DSN quy ước xuyên suốt
> bộ CR-DB-002/003, xem `common/testutil.StartMySQL` và chính ví dụ Verify
> của task này); (2) URL thường không có `tcp(...)` — dùng `net/url.Parse`
> rồi bọc lại. Cả 2 dạng đều tự động thêm `?parseTime=true` nếu chưa có
> (phát hiện ở TASK-BE-DB-005: bắt buộc để scan cột `TIMESTAMP` vào
> `time.Time`/`sql.NullTime`).
>
> Test `TestToMySQLDriverDSN_ConvertsURLFormat` **lệch giá trị mong đợi
> chính xác** so với task doc (task doc muốn khớp đúng
> `"user:pass@tcp(host:3306)/usage"`) — vì hàm luôn thêm `?parseTime=true`,
> test đã đổi sang kiểm tra prefix + chứa `parseTime=true` thay vì so khớp
> chuỗi tuyệt đối, có ghi chú lý do ngay trong test. Thêm 1 test ngoài yêu
> cầu gốc: `TestToMySQLDriverDSN_ConvertsAlreadyWrappedTCPFormat` (dạng DSN
> thực tế dùng nhiều nhất trong bộ CR này). File test
> `cmd/server/main_test.go` là file MỚI không có trong "Files cần sửa" gốc
> của task (chỉ liệt kê `main.go`) — cần thiết để chứa 4 test case task tự
> yêu cầu ở mục "Test cases cần cover", bổ sung hợp lý.
>
> **Build/test thật đã chạy**: `go build ./...` sạch; `go test ./...` —
> **toàn bộ PASS** (`cmd/server`: 4/4 test DSN PASS; `internal/domain`,
> `internal/usecase`: PASS như trước, không đổi); `gofmt -l cmd/server/main.go`
> sạch. **Chạy thật usage-service với MySQL** (yêu cầu bổ sung ở mục
> Verify, không giả định): dựng container `mysql:8` thật qua Docker, chạy
> migration `migrations/mysql` (TASK-BE-DB-004), rồi
> `DATABASE_DSN=mysql://root:orca@tcp(localhost:3307)/usage go run
> ./cmd/server` — service start thành công, log xác nhận
> `"usage-service http (health) listening"` + `"usage-service grpc
> listening"` (NATS không có sẵn trong môi trường này nên log WARN "eventbus
> unavailable" — đúng hành vi graceful-degradation đã có sẵn, không phải
> lỗi mới). `curl http://localhost:18080/readyz` → **`{"mysql":"ok"}`,
> HTTP 200** — xác nhận đúng tiêu chí Verify. Container MySQL và process
> service đã dọn sau khi xác nhận xong.

---

## Mục tiêu

Thay đoạn `main.go` hiện tại (luôn `pgxpool.New` + `usagepostgres.New`)
bằng factory chọn dialect dựa trên `DATABASE_DSN` thật (không thêm biến
env mới — xem BE-DB-SOL-001 §1).

## Files cần sửa

1. `backend-go/services/usage-service/cmd/server/main.go` (MODIFY)
2. `backend-go/services/usage-service/go.mod` (MODIFY — thêm `github.com/stablyai/orca-go/common` đã có sẵn, chỉ cần import `dbcapability`, không phải dependency mới)

## Thay đổi cụ thể trong `main.go`'s `run()`

Đoạn hiện tại (đã Read đầy đủ):

```go
dsn, err := secrets.DatabaseCredentialsFromFile(cfg.DatabaseCredentialsFile)
if err != nil {
	return fmt.Errorf("resolving database credentials: %w", err)
}
pool, err := pgxpool.New(ctx, dsn)
if err != nil {
	return fmt.Errorf("connecting to postgres: %w", err)
}
defer pool.Close()

repo := usagepostgres.New(pool)

healthSrv := health.New()
healthSrv.Register("postgres", func() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return pool.Ping(ctx)
})
```

Đổi thành:

```go
dsn, err := secrets.DatabaseCredentialsFromFile(cfg.DatabaseCredentialsFile)
if err != nil {
	return fmt.Errorf("resolving database credentials: %w", err)
}
caps, err := dbcapability.DetectDialectFromDSN(dsn)
if err != nil {
	return fmt.Errorf("detecting database dialect: %w", err)
}

healthSrv := health.New()

var repo usecase.Repository
switch caps.Dialect {
case dbcapability.DialectPostgres:
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connecting to postgres: %w", err)
	}
	defer pool.Close()
	repo = usagepostgres.New(pool)
	healthSrv.Register("postgres", func() error {
		pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return pool.Ping(pingCtx)
	})
case dbcapability.DialectMySQL:
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("connecting to mysql: %w", err)
	}
	defer db.Close()
	repo = usagemysql.New(db)
	healthSrv.Register("mysql", func() error {
		pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return db.PingContext(pingCtx)
	})
default:
	return fmt.Errorf("unsupported database dialect: %s", caps.Dialect)
}
```

**Lưu ý bắt buộc kiểm tra khi implement**: `sql.Open("mysql", dsn)` của
`go-sql-driver/mysql` **không nhận DSN dạng URL `mysql://user:pass@host/db`
trực tiếp** — driver này dùng format riêng
`user:pass@tcp(host:port)/dbname?param=value` (không có scheme
`mysql://`). Phải viết 1 hàm chuyển đổi nhỏ (`toMySQLDriverDSN(dsn string)
string`) parse URL `mysql://...`/`tidb://...` (dùng `net/url.Parse`, vẫn
hợp lệ dù driver không nhận trực tiếp) rồi build lại đúng format driver
cần — đây là chi tiết BE-DB-SOL-001 §3 gọi tên hàm `toMySQLDSN` nhưng
chưa viết thân hàm; task này phải viết đầy đủ, có test riêng
(`TestToMySQLDriverDSN_ConvertsURLFormat`) vì đây là điểm dễ lỗi nhất khi
wire.

`repo` cần khai kiểu `usecase.Repository` tường minh (thay vì
`*usagepostgres.Repository` ngầm định trước đây) — biến các dòng dùng
`repo` phía dưới (`usecase.NewRecordUsageSession(repo)`, v.v.) đã nhận
interface sẵn nên không cần đổi gì thêm.

## Test cases cần cover

- `TestToMySQLDriverDSN_ConvertsURLFormat` — `mysql://user:pass@host:3306/usage` → `user:pass@tcp(host:3306)/usage`.
- `TestToMySQLDriverDSN_TiDBSchemeConvertsSameAsMySQL` — `tidb://...` cho ra cùng format.
- `TestToMySQLDriverDSN_RejectsUnparsableDSN` — chuỗi không phải URL hợp lệ trả lỗi rõ ràng, không panic.

## Verify

```bash
cd backend-go/services/usage-service && go build ./... && go test ./...
gofmt -l cmd/server/main.go
```

Chạy thật `usage-service` với `DATABASE_DSN=mysql://root:orca@tcp(localhost:3307)/usage`
trỏ vào container MySQL đã migrate (TASK-BE-DB-004's verify) và xác nhận
service start thành công, `/healthz` (hoặc endpoint health thật của
service — kiểm tra `common/health` để dùng đúng path) trả `mysql: ok`.

## gitnexus

`impact({target: "run", direction: "upstream", file_path: "services/usage-service/cmd/server/main.go"})`
— **đã chạy thật**: risk **LOW**, impacted 1 (chỉ `main()` cùng file gọi
`run()` — composition root, không có caller nào khác). An toàn để sửa
theo kết quả này.

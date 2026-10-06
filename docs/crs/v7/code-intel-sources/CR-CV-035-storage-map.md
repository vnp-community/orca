# CR-CV-035 — Bản đồ lưu trữ `StorageMap`: kho dữ liệu, kết nối service, topic event bus, và quy tắc che secret

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-035 |
| **Tên** | Dựng `StorageMap` (Store / Binding / Topic) từ compose, cấu hình service, thư mục adapter, migration và subject NATS; đặt quy tắc che secret là ràng buộc cứng |
| **Loại** | Feature (backend `code-intel-service`) |
| **Priority** | ⚪ P2 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-030 (cổng đọc file qua `fs.*`/`git.*`, chặn đường dẫn), CR-CV-031 (ErdModel: bảng/schema thuộc service nào), CR-CV-033 (danh sách container/service và component adapter). Nguồn ngoài (research 09): **E3** compose/deploy, **E4** cấu hình từng service, **E10** bản đồ topic, **E15** ADR (đọc `specs/backend-go/architecture/05-data-architecture.md` để đối chiếu, tuỳ chọn), **E17** hệ thống ngoài (Vault dùng chung) |
| **Mở khoá** | CR-CV-058 (lens Lưu trữ), `DataFlow.stores` của CR-CV-034 (dùng `Store.id`), CR-CV-072 (kiểm thử che secret) |
| **Tác động** | `backend-go/services/code-intel-service` (mới): package trích xuất lưu trữ, RPC `GetStorageMap`; `backend-go/proto/orca/codeintel/v1` (message `StorageMap`, `Store`, `Binding`, `Topic`); không sửa service khác, không sửa file compose |

---

## 1. Bối cảnh và vấn đề

View "Kiến trúc lưu trữ" ([08 §6](../../../research/view-code/08-views-and-review-models.md)) cần trả lời "service nào ghi vào kho nào, qua adapter nào, sự kiện nào chạy giữa các service". Khảo sát 2026-10-05 (đã đọc file thật):

**Compose, bốn nơi, nội dung rất khác nhau**

| File | Nội dung thật | Hệ quả cho bản đồ |
|---|---|---|
| `backend-go/docker-compose.yml` (58 dòng) | Chỉ hạ tầng: `postgres:16-alpine`, `hashicorp/vault:1.17` (chế độ dev), `nats:2.10-alpine` (`-js`), volume `postgres_data`. Chứa **literal** `POSTGRES_PASSWORD: orca` và `VAULT_DEV_ROOT_TOKEN_ID: dev-root-token` | Ví dụ sống cho việc phải che secret ngay cả khi giá trị là literal trong compose. Không có service Go nào |
| `/opt/repos/orca/deploy/dev/docker-compose.yml` (758 dòng, `name: orca-go`) | `postgres`, `nats`, **19 khối service ứng dụng** (17 service có DB + `git-gateway-service`, `api-gateway`; đếm bằng grep, comment trong file ghi "15 own a database" đã cũ), `frontend` (nginx), `vault-token-renewer`, **17 job `migrate-*`** (profile `migrate`). Mỗi service có `DATABASE_DSN: postgresql://orca:${POSTGRES_PASSWORD}@postgres:5432/<db>?sslmode=disable`; `NATS_URL: nats://nats:4222`; **Vault không chạy trong compose**, nằm ở máy dùng chung (`VAULT_ADDR`/`VAULT_TOKEN` trong khối `x-go-common-env`, địa chỉ IP nội bộ viết literal). Volume `orca-go-postgres-data`, `git-gateway-repos` (git-gateway giữ repo cục bộ) | Nguồn chính của `Store` ở dev. Dùng YAML anchor và merge key (`<<: *go-defaults`, `<<: *go-common-env`), nên parser phải giải anchor |
| `/opt/repos/orca/deploy/dev/docker/postgres/init-databases.sh` | Danh sách 17 database: `auth tenant project infra aiprovider workflow task orchestration automation annotation notification usage credential issuetracking scm mcp issuestatussync` | Nguồn tên DB (`Store.schemas`). Tên DB **không trùng tên service** (`infra`, `scm`, `credential`) nên không suy bằng tên |
| `/opt/repos/orca/deploy/prod/docker-compose.yml` (78 dòng) | **Chỉ một service `orca-server`** (image `${ORCA_IMAGE:-vnpblc/orca-server}`), volume `orca-data` (SQLite mặc định), mạng `orca-net`. Các dòng `ORCA_DB_URL=mysql://…@db:3306`, `postgresql://…@db:5432`, `tidb://…@tidb:4000` **đã comment** và trỏ tới service `db`/`tidb` không được định nghĩa. Không có service `backend-go` nào | Topology prod thật **không suy ra được** từ repo (khớp research 09 E3). Đây là stack `orca-server` (Node), không phải backend-go. Bản đồ prod chỉ có 1 store (`volume orca-data`) kèm cờ `unknown` |
| `/opt/repos/orca/deploy/old/` (`docker-compose.yml` 186 dòng, `docker-compose.orca.yml`, `docker-compose.orca.artifact.yml`) | Ba compose cũ: `postgres`, `orca`, `nginx`; volume `orca-data`, `orca-projects`, `orca-postgres-data`, `nginx-logs` | Di sản; hiển thị mờ, gắn `legacy` |

Lưu ý đường dẫn: `deploy/` nằm ở **gốc repo** (`/opt/repos/orca/deploy`), không nằm trong `backend-go/` (chỉ `backend-go/deploy/` có `postgres-init-databases.sh` và `alerts/`). README v7 ghi "deploy/dev…" đúng theo gốc repo.

**Cấu hình và adapter**
- `backend-go/common/config/config.go` định nghĩa `Base{ServiceName, GRPCPort, HTTPPort, DatabaseDSN, OTLPEndpoint}` (biến `GRPC_PORT` 9090, `HTTP_PORT` 8080, `DATABASE_DSN`, `OTLP_ENDPOINT`). Mỗi service có `internal/config/config.go` nhúng `Base` và thêm biến riêng. Ví dụ `infra-fleet-service/internal/config/config.go`: `DATABASE_CREDENTIALS_FILE` (mặc định `/vault/secrets/database-credentials`, tệp do Vault Agent render, ưu tiên hơn DSN), `NATS_URL` (mặc định `nats://localhost:4222`), `CREDENTIAL_BROKER_ADDR`, `AI_PROVIDER_SERVICE_ADDR`, `AUTH_SERVICE_ADDR`. 17 lời gọi `StringEnv("DATABASE_CREDENTIALS_FILE"…)` đếm được trong `internal/config` và `cmd`.
- Thư mục adapter lưu trữ (đếm từ cây thật, `services/*/internal/adapter`): `postgres` ở 17 service; `mysql` ở 16 (trừ `mcp-service`, và `api-gateway`/`git-gateway-service` không có DB); `eventbus` ở `ai-provider`, `automation`, `infra-fleet`, `issue-status-sync`, `notification`, `project`, `task`, `tenant`; `natsconsumer` ở `auth-service`; `vault` ở `auth-service`, `credential-broker-service`; `vaultsigner` ở `notification-service`; `cache` (LRU trong tiến trình) ở `tenant-service`. `mcp-service` chỉ có `migrations/postgres`; mọi service khác có cả `migrations/postgres` và `migrations/mysql`.
- **MySQL được code hỗ trợ nhưng không compose nào triển khai** (đã grep `mysql` trên mọi compose: chỉ có dòng comment ở prod). Bản đồ phải tách "kho được hỗ trợ bởi code" khỏi "kho được triển khai".
- **Redis/object store**: không có trong compose; chỉ xuất hiện trong comment (`tenant-service/cmd/server/main.go:186`, `api-gateway/internal/usecase/rate_limit.go:13`: "có thể thay bằng Redis"). Không tạo `Store` cho thứ chỉ có trong comment.

**Event bus**: `common/eventbus/eventbus.go` quy ước subject `orca.<service>.<entity>.<event>`, envelope `Event{ID, TenantID, OccurredAt, Version, Payload}`, JetStream, `Publisher.EnsureStream(name, subjects)`, `Consumer.Subscribe(streamName, consumerName, subject, fn)` (durable, at-least-once), `SubscribeEphemeral` cho fan-out theo replica. Subject xuất hiện dưới ba dạng trong mã:
1. Hằng/literal ở nơi phát hành: `infra-fleet-service/internal/domain/terminal_closed_event.go:12` (`const SubjectTerminalClosed = "orca.infrafleet.terminal.closed"`), `usecase/*.go` (ví dụ `task-service/internal/usecase/update_task.go`).
2. Bảng `SubjectBinding{StreamName, Subject}` ở phía nghe: `task-service/internal/adapter/eventbus/consumer.go` (`ORCHESTRATION` → `orca.orchestration.task.statuschanged`, `WORKFLOW` → `orca.workflow.step.completed`); tương tự `notification-service`, `automation-service`.
3. `EnsureStream(ctx, "PROJECT", []string{"orca.project.>"})` (`project-service/cmd/server/main.go:177`), `"MCP"` → `orca.mcp.>` (`mcp-service/cmd/server/main.go:128`).
Grep `"orca\.<x>\.<y>\.<z>"` trong `services/` và `common/` (trừ test, `gen`) ra khoảng 120 vị trí, gồm cả `api-gateway` (cầu nối push, `wscompat/channels_task_activity.go`, `wscompat/workspace_events.go`, `mcpserver/resources/task_events.go`) và `auth-service/internal/adapter/natsconsumer`. **Không phải mọi literal là một bên phát hoặc một bên nghe** (ví dụ `notification-service/internal/domain/notification_event.go` liệt kê subject để ánh xạ); phải phân loại theo ngữ cảnh. Có hai tiền tố cho cùng nhóm: `orca.infra.terminal_session.*` (publisher `infra-fleet-service/internal/adapter/eventbus/publisher.go`) và `orca.infrafleet.*` (domain) — bản đồ phải giữ nguyên tên, không "chuẩn hoá".

**Vault**: không phải kho dữ liệu mà là kho khoá. Mã nguồn dùng qua `common/secrets` (`NewClient` đọc `VAULT_ADDR`/`VAULT_TOKEN`); chỉ `credential-broker-service` giữ bí mật tenant, `auth-service` dùng Transit để ký JWT (comment ở `common/secrets`). Migration có cột `vault_ssh_role` (`infra-fleet-service/internal/domain/ssh_target.go:16`: "this service never stores raw key material").

**Secret có thật gần các file này**: `/opt/repos/orca/deploy/dev/.env` tồn tại trên máy khảo sát (đã bị `.gitignore` bởi `deploy/dev/.gitignore`), và `.env.example` ghi chú về token Vault. Vấn đề cốt lõi của CR này: **bản đồ lưu trữ là nơi dễ lộ secret nhất của cả series v7** vì nguồn của nó chính là cấu hình triển khai.

## 2. Giải pháp đề xuất

### 2.1 Mô hình dữ liệu (proto + domain, mới)

Giữ đúng tên ở [08 §6](../../../research/view-code/08-views-and-review-models.md), bổ sung trường bằng chứng và độ tin cậy (xem "Điều chỉnh hợp đồng" ở báo cáo):

```
StorageMap { stores: Store[], bindings: Binding[], topics: Topic[], sources: SourceRef[], redactedCount: int, asOfCommit }
Store   { id, kind:"postgres|mysql|redis|object|volume|vault|queue|other", name, env:"dev|prod|legacy",
          owner?: ServiceRef, schemas?: string[], deployed: bool, supportedByCode: bool,
          external: bool, evidence: SourceRef[], confidence:"declared|derived|inferred" }
Binding { service, store, access:"rw|ro", via: string /*thư mục adapter*/, configKey?: string /*tên biến, KHÔNG có giá trị*/,
          evidence: SourceRef[], confidence }
Topic   { name, stream?, publishers: ServiceRef[], subscribers: ServiceRef[], delivery:"durable|ephemeral|unknown",
          payload?: string /*tên kiểu*/, evidence: SourceRef[], confidence }
SourceRef { path /*tương đối gốc repo*/, line?: int, kind:"compose|config|adapter|migration|code" }
```

- `Store.id = "<kind>:<env>:<name>"` (ví dụ `postgres:dev:orca-go-postgres`, `vault:dev:shared-external`); ổn định để `DataFlow.stores` (CR-CV-034) tham chiếu.
- Mỗi DB logic là một phần tử trong `Store.schemas` của Store `postgres` (17 DB trong `init-databases.sh`); `Binding.service` → `Store` kèm DB nào (`schemas` ghi tên DB; owner = service sở hữu schema theo migration của CR-CV-031).
- `confidence`: `declared` (đọc thẳng từ compose/config), `derived` (suy từ cấu trúc thư mục/migration), `inferred` (heuristic subject/ngữ cảnh). UI phải hiển thị khác nhau (CR-CV-058).
- `Binding.configKey` chỉ là **tên** biến (`DATABASE_DSN`, `NATS_URL`, `DATABASE_CREDENTIALS_FILE`); không bao giờ giá trị.

### 2.2 Nguồn và thứ tự trích xuất (đều qua `fs.*`/`git.*` của CR-CV-030, phạm vi worktree)

1. **Compose**: đọc đúng danh sách cố định `backend-go/docker-compose.yml`, `deploy/dev/docker-compose.yml`, `deploy/prod/docker-compose.yml`, `deploy/old/docker-compose*.yml`, và `deploy/dev/docker/postgres/init-databases.sh`; không quét `deploy/**` tự do. Parse YAML (`gopkg.in/yaml.v3` đã có trong `go.sum` của một số service; thêm vào module `code-intel-service` là dependency mới cần duyệt như bình thường), giải anchor/merge key. Chỉ lấy các trường trong **allowlist** ở 2.3.
2. **Cấu hình service**: với mỗi service (danh sách từ CR-CV-033), đọc `internal/config/config.go`, trích **tên biến môi trường** từ các lời gọi `os.Getenv("X")`, `commonconfig.StringEnv("X", def)`, `intEnv("X", def)` bằng parser Go (`go/parser` + duyệt AST, không regex trên text) — lấy tên khoá, kiểu, và giá trị mặc định **chỉ khi** khoá không khớp mẫu nhạy cảm (2.3.3). Phân loại khoá: `*_ADDR` → phụ thuộc service (không phải kho), `NATS_URL` → Binding tới NATS, `DATABASE_*` → Binding tới Postgres/MySQL, `VAULT_*`/`*_CREDENTIALS_FILE` → Binding tới Vault.
3. **Adapter**: liệt kê thư mục con của `internal/adapter` và gán `via` theo bảng cố định: `postgres`→kho postgres, `mysql`→kho mysql, `eventbus`/`natsconsumer`→NATS, `vault`/`vaultsigner`→Vault, `cache`→bỏ qua (bộ nhớ trong tiến trình, không phải Store). Một service có `adapter/mysql` mà compose không có MySQL → Store `mysql` với `supportedByCode=true, deployed=false`.
4. **Migration**: gắn `Store.schemas` và owner từ `ErdModel` (CR-CV-031): mỗi service một schema (Postgres) hoặc một database (MySQL, không tiền tố).
5. **Topic**: xem 2.4.
6. **Hợp nhất**: `Store` trùng khoá (`kind`,`env`,`name`) gộp, giữ mọi `evidence`.

Số lệnh đọc file: khoảng 6 (compose + init script) + 2 mỗi service (config, liệt kê adapter) + quét subject (2.4). Tất cả có hạn mức của CR-CV-030; kết quả cache theo `(repo, commit, "storage", params_hash)` (CR-CV-022).

### 2.3 QUY TẮC CHE SECRET (ràng buộc cứng, kiểm thử ở CR-CV-072)

Nguyên tắc: **allowlist theo cấu trúc, không dựa vào denylist**. Mặc định mọi giá trị là bí mật trừ khi qua được cổng bên dưới.

1. **Tệp không bao giờ đọc**: `**/.env`, `**/.env.*` ngoại trừ `.env.example` (chỉ lấy tên khoá, bỏ giá trị và comment), `*.pem`, `*.key`, `*.p12`, `/vault/secrets/**`, mọi thứ ngoài worktree. Chặn **hai lớp**: CR-CV-030 (đường dẫn) và bộ thu của CR này (trước khi parse). `deploy/dev/.env` có thật trên máy dev nên đây không phải lý thuyết.
2. **Allowlist trường compose**: `services.<n>.{image, container_name, profiles, depends_on, networks, ports (chỉ cổng phía container), volumes (chỉ tên named volume và đích mount, bỏ nguồn bind-mount là đường dẫn máy chủ), environment (chỉ TÊN khoá), command (bỏ hẳn), healthcheck (bỏ), logging (bỏ)}`, top-level `volumes`, `networks`, `name`. Mọi trường khác bị bỏ.
3. **Giá trị biến môi trường**:
   - Khoá khớp `(?i)(SECRET|TOKEN|PASSWORD|PASSWD|PASSPHRASE|PRIVATE|CREDENTIAL|API_?KEY|AUTH|SALT|SIGNING|CURSOR_KEY|DSN)` → **không lưu giá trị**; lưu `{name, secretRef:true}`.
   - DSN/URL (`postgresql://`, `mysql://`, `tidb://`, `nats://`, `http(s)://`): parse bằng `net/url`; chỉ giữ `{scheme, host, port, path(tên DB)}`; **bỏ userinfo, query, fragment**. Lỗi parse → bỏ cả giá trị (fail-closed). Host là địa chỉ IP literal hoặc ngoài mạng compose → thay bằng `external:true`, không lưu host (compose dev chứa IP nội bộ của Vault dùng chung, không đưa lên UI).
   - Tham chiếu `${VAR}`, `${VAR:-x}`, `${VAR:?thông báo}`: chỉ lưu `VAR`; **bỏ** phần mặc định và thông báo (thông báo lỗi trong compose có thể chứa chỉ dẫn nội bộ).
   - Giá trị khác: chỉ lưu nếu khoá nằm trong allowlist khoá không nhạy cảm (`*_ADDR`, `*_PORT`, `*_ENABLED`, `ORCA_SERVER_DEPLOYMENT`, `FLEET_POLL_INTERVAL_SEC`, `EPHEMERAL_VM_SSH_MODE`, `OPA_BUNDLE_PATH`, `NATS_URL` sau khi qua quy tắc DSN) **và** không qua kiểm tra entropy. Còn lại thay bằng `«redacted»` và tăng `redactedCount`.
   - Kiểm tra entropy: chuỗi ≥ 20 ký tự có entropy Shannon > 3,5 bit/ký tự, hoặc khớp `[A-Za-z0-9+/=_-]{32,}`, hoặc tiền tố `hvs.`/`s.`/`ghp_`/`glpat-`/`xox`/`sk-` → redacted bất kể khoá.
4. **Comment YAML/Go bị bỏ hoàn toàn** (compose dev chứa IP nội bộ, chuyện sự cố và đường dẫn máy chủ trong comment).
5. **Literal trong compose cũng tuân quy tắc**: `POSTGRES_PASSWORD: orca`, `VAULT_DEV_ROOT_TOKEN_ID: dev-root-token` ở `backend-go/docker-compose.yml` phải ra `«redacted»` (khoá khớp mẫu), không có ngoại lệ "giá trị dev".
6. **Vault**: chỉ lưu `Store{kind:"vault", name, external, deployed}` và **tên đường dẫn/khoá** nếu tìm thấy ở dạng không phải giá trị bí mật (ví dụ tên Transit key `vapid-signing-*`, `vault_ssh_role` là tên cột). Không lưu `VAULT_TOKEN`, không lưu nội dung `orca-policy.hcl`.
7. **Quét lần cuối (defense in depth)**: trước khi ghi cache hoặc trả RPC, chạy bộ quét trên payload đã serialize (các mẫu ở mục 3 + userinfo trong URL `://[^/@\s]+:[^/@\s]+@`). Nếu thấy → **không trả view**, trả lỗi `CODEINTEL_SECRET_LEAK_BLOCKED` (mới; xem "Điều chỉnh hợp đồng"), ghi metric và log **chỉ đường dẫn tệp nguồn**, không log giá trị.
8. **Log/trace**: collector không log nội dung file; span chỉ có đường dẫn và số lượng.
9. **Không có "chế độ hiện secret"**, kể cả cho admin. Người dùng muốn xem giá trị phải mở file ở worktree (đã có quyền sẵn).

### 2.4 Trích xuất topic event bus

Heuristic hai bước, trên mã Go của worktree (qua `fs.*`; quét `services/*/internal/**/*.go` và `common/eventbus`, bỏ `_test.go`, `gen/`):

1. **Tìm subject**: literal khớp `^orca\.[a-z0-9_]+(\.[a-z0-9_]+){1,3}$` và mẫu wildcard `orca\.[a-z0-9_]+\.>`. Dùng `go/parser` để chỉ lấy literal trong `const`, `var`, composite literal `SubjectBinding{…}`, đối số `EnsureStream`/`Subscribe`/`Publish`, trường `Subject:`; bỏ literal trong comment và chuỗi nhật ký.
2. **Phân loại vai trò** (đặt `confidence`):
   - **publisher** (`declared`): literal là đối số/trường `Subject` của lời gọi `Publish`/`PublishDedup`/`outbox` (`Event{… Subject: …}`) hoặc nằm trong file `adapter/eventbus/publisher*.go`.
   - **subscriber** (`declared`): literal nằm trong `[]SubjectBinding{…}` hoặc là đối số của `Subscribe`/`SubscribeEphemeral`, hoặc file `adapter/eventbus/{consumer,subscriber}.go`, `adapter/natsconsumer/*.go`.
   - Literal khác (ví dụ `notification-service/internal/domain/notification_event.go`) → `confidence:"inferred"` và ghi vai trò `unknown`.
   - `api-gateway` (cầu nối push `wscompat`, `mcpserver/resources`) chứa nhiều literal subject; vai trò (subscriber) và kiểu giao (`durable`/`ephemeral`) **chưa kiểm chứng** từng điểm. Quy tắc: `delivery:"ephemeral"` chỉ khi thấy lời gọi `SubscribeEphemeral` (hoặc `Consumer.SubscribeEphemeral`) cùng subject; ngược lại `unknown`.
3. **Stream**: lấy từ `SubjectBinding.StreamName` hoặc `EnsureStream(name, subjects)`; subject wildcard `orca.project.>` phủ các subject cụ thể cùng tiền tố → gán `stream` cho chúng.
4. `Topic.payload` chỉ là **tên kiểu** nếu thấy struct payload kề bên (ví dụ `mirrorPayload`); không đọc dữ liệu thực.
5. Ranh giới: bản đồ nói "service X phát subject Y", **không** cam kết thông điệp có thật chạy ở runtime.

### 2.5 RPC và quyền

`GetStorageMap(GetStorageMapRequest{repo_binding_id, env_filter?, include_legacy=false}) returns (GetStorageMapResponse{StorageMap, meta})` (tên cuối theo CR-CV-020 về envelope `CodeIntelResult`; `view="storage"` là giá trị mới, xem điều chỉnh hợp đồng). Quyền như các view khác (CR-CV-013); feature flag `code_intel_enabled` (O8). Không có RPC ghi.

## 3. Quyết định thiết kế

1. **Đọc file thay vì hỏi Docker/DB thật**: không chạy `docker compose config` (sẽ nội suy `.env` và lộ secret) và không kết nối DB (E14 ngoài phạm vi). Đọc file thô là cách duy nhất không bao giờ chạm giá trị thật.
2. **Allowlist thay vì denylist**: danh sách tên khoá nhạy cảm không đầy đủ (xuất hiện thật: `MCP_CURSOR_KEY`, `OAUTH_STATE_SECRET`, `APNS_KEY_ID`...). Cổng mặc định-từ-chối an toàn hơn.
3. **Tách `deployed` và `supportedByCode`**: MySQL có adapter và migration nhưng không compose nào chạy; trộn hai khái niệm sẽ nói dối về kiến trúc.
4. **Prod = `unknown` có chủ ý**: compose prod chỉ có `orca-server`; vẽ topology Go cho prod là bịa. Hiển thị cảnh báo "cần xác nhận topology" (E3).
5. **`go/parser` cho Go, `yaml.v3` cho compose**: regex trên mã nguồn dễ nhầm literal trong comment và sai trước chuỗi nhiều dòng.
6. **Không dựng Store cho Redis/cache chỉ nằm trong comment**; `tenant-service` LRU là bộ nhớ trong tiến trình, bỏ qua.
7. **Fail-closed**: parse lỗi hoặc quét cuối thấy mẫu nghi ngờ → không trả view, không trả "một phần". Hiển thị tốt hơn ít còn hơn lộ.
8. Đặt trong `code-intel-service` (không phải agent): theo D6, chỉ cần đọc file nhỏ.

## 4. Tiêu chí chấp nhận

- [ ] `GetStorageMap` trên chính repo Orca trả: Store `postgres` (dev) với đủ 17 database; `nats` (dev, JetStream); `vault` (dev, `external:true`, `deployed:false` trong compose); `volume` `orca-go-postgres-data`, `git-gateway-repos`; ở prod chỉ `volume orca-data` kèm cờ `unknown`; `deploy/old` gắn `env:"legacy"` và ẩn mặc định.
- [ ] Store `mysql` xuất hiện với `supportedByCode=true, deployed=false`; không tạo Store `redis`.
- [ ] Mọi service có `adapter/postgres` có Binding tới Postgres với `configKey` = `DATABASE_DSN`; `mcp-service` không có Binding MySQL; `git-gateway-service`, `api-gateway` không có Binding DB.
- [ ] Topic: `orca.infrafleet.terminal.closed` có publisher `infra-fleet-service` và subscriber `notification-service`; `orca.orchestration.task.statuschanged` có subscriber `task-service`; subject tiền tố `orca.infra.*` và `orca.infrafleet.*` giữ nguyên tên; mỗi Topic có `confidence` và `evidence` (đường dẫn + dòng).
- [ ] **Che secret**: payload trả về chứa 0 lần các chuỗi `dev-root-token`, `orca` (mật khẩu), giá trị `.env` của `deploy/dev/.env` (fixture dùng bản sao `.env` giả có chuỗi dò), userinfo trong bất kỳ URL; `redactedCount` > 0 trên fixture; IP nội bộ của Vault không có trong payload.
- [ ] Tệp `.env` không bao giờ được yêu cầu qua `fs.*` (kiểm thử bằng agent giả ghi nhận mọi yêu cầu).
- [ ] Bộ quét cuối chặn khi cố tình chèn giá trị bí mật vào fixture; RPC trả `CODEINTEL_SECRET_LEAK_BLOCKED`, log không chứa giá trị.
- [ ] Cache theo `(repo, commit, "storage", params_hash)` bị huỷ khi commit đổi; cùng input cho ra cùng thứ tự `Store`/`Topic` (sắp xếp cố định).
- [ ] Lỗi parse một compose không làm hỏng cả view: file đó vào `sources` với trạng thái lỗi, các nguồn còn lại vẫn trả (trừ khi quét cuối chặn).
- [ ] `buf lint` và `buf breaking` pass; không khai báo RPC chưa có message.

## 5. Kiểm thử

- **Fixture vàng** (CR-CV-070): sao chép compose/config/adapter tối thiểu từ repo Orca vào `testdata/storage/` (đã che tay, chỉ chứa literal giả). Snapshot `StorageMap` JSON, so khớp khi đổi parser.
- **Unit parser**: YAML anchor/merge (`<<: *go-defaults`); biến `${POSTGRES_PASSWORD:?msg}`; DSN có userinfo, có query `sslmode`; DSN không parse được; khoá nhạy cảm literal; chuỗi entropy cao dưới khoá lạ; comment chứa IP.
- **Unit Go AST**: `StringEnv("X","def")` / `os.Getenv`; literal subject trong comment bị bỏ; `SubjectBinding` nhiều phần tử; `EnsureStream` wildcard.
- **Bảo mật** (cùng CR-CV-072): fuzz giá trị env; "canary" chuỗi bí mật phải không xuất hiện ở payload, cache, log, metric, span (quét bytes).
- **Hợp đồng**: so khớp danh sách service/adapter với cây thư mục thật (kiểm tra tồn tại thư mục, không hardcode).
- **Hai dialect DB** cho bảng cache của service (O1): chạy cùng bộ test trên Postgres và MySQL.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Chưa chạy hệ thống**: số liệu (19 service, 17 DB, 17 `migrate-*`, ~120 literal subject) là kết quả đọc/grep ngày 2026-10-05, chưa chạy compose. Người triển khai đọc lại file trước khi viết fixture.
- Heuristic phân loại publisher/subscriber có thể sai ở literal gián tiếp (hằng được truyền qua nhiều lớp, subject ghép chuỗi). Chưa đo tỷ lệ sai; đề xuất đo bằng cách đối chiếu tay 20 topic ở nghiệm thu.
- Chưa kiểm chứng: mọi service có `config.go` theo cùng mẫu `Getenv/StringEnv` (đã thấy ở `infra-fleet`, `task`; các service còn lại chưa đọc từng file). `api-gateway` có thêm `config_mcp.go`.
- `go/parser` cần đọc nguyên tệp (kích thước giới hạn bởi CR-CV-030); `cmd/server/main.go` có thể rất lớn (tìm thấy `EnsureStream` ở đó), cần hạn mức riêng.
- Topology prod thật chưa biết (E3); danh sách tên khoá nhạy cảm không đầy đủ nên cổng chính là allowlist; chưa kiểm chứng entropy ngưỡng 3,5 có báo nhầm với tên service dài hay không.
- Vault dùng chung nằm ngoài compose: chỉ biết qua biến cấu hình; không xác nhận được path/policy thật ngoài `orca-policy.hcl` (không đọc để tránh lộ).
- Cấu hình có thể đến từ nơi ngoài repo (Vault Agent render `/vault/secrets/database-credentials`, biến môi trường thật): bản đồ phản ánh **khai báo trong repo**, không phải runtime.
- Phụ thuộc kiểu `ErdModel` (CR-CV-031) và danh sách container (CR-CV-033) chưa chốt tên trường; khi hai CR đó đổi, cần cập nhật.

## 7. Câu hỏi mở

1. Compose prod thật (topology backend-go ở prod) lấy từ đâu — người dùng cung cấp mô tả/`.env.example`, hay chỉ hiển thị `unknown`? (E3)
2. Có cho hiển thị IP/host nội bộ của Vault dùng chung không? Mặc định đề xuất: không (`external:true`).
3. `deploy/old` có còn đáng hiển thị không, hay bỏ hẳn khỏi nguồn?
4. Có cần bật chế độ đối chiếu `information_schema` thật (E14) trong tương lai; nếu có phải thiết kế riêng vì chạm DB thật.
5. Hai tiền tố `orca.infra.*` và `orca.infrafleet.*` cùng tồn tại: có đánh dấu "không nhất quán" như một finding của CR-CV-037 không?

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md`; `/opt/repos/orca/docs/research/view-code/08-views-and-review-models.md` (§6), `09-external-inputs-required.md` (E3, E4, E10, E15, E17), `05-graph-schemas.md`
- `/opt/repos/orca/backend-go/docker-compose.yml`
- `/opt/repos/orca/deploy/dev/docker-compose.yml`, `/opt/repos/orca/deploy/dev/docker/postgres/init-databases.sh`, `/opt/repos/orca/deploy/dev/.env.example`, `/opt/repos/orca/deploy/dev/.gitignore`
- `/opt/repos/orca/deploy/prod/docker-compose.yml`, `/opt/repos/orca/deploy/old/docker-compose.yml`
- `/opt/repos/orca/backend-go/common/config/config.go`, `/opt/repos/orca/backend-go/common/eventbus/eventbus.go`, `/opt/repos/orca/backend-go/common/secrets`
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/config/config.go`, `…/internal/domain/terminal_closed_event.go`, `…/internal/adapter/eventbus/publisher.go`
- `/opt/repos/orca/backend-go/services/task-service/internal/adapter/eventbus/consumer.go`
- `/opt/repos/orca/backend-go/services/project-service/cmd/server/main.go` (dòng 177, 263), `/opt/repos/orca/backend-go/services/mcp-service/cmd/server/main.go` (dòng 128)
- `/opt/repos/orca/backend-go/services/tenant-service/internal/adapter/cache/lru_ttl_cache.go`
- `/opt/repos/orca/guides/STYLEGUIDE.md`, `/opt/repos/orca/AGENTS.md`
- CR liên quan trong series: CR-CV-030, 031, 033, 034, 058, 070, 072

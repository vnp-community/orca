# BE-CV-SOL-035: `GetStorageMap`: dựng `StorageMap` (Store, Binding, Topic) từ compose, cấu hình, adapter, subject NATS; che secret fail-closed

> **📋 Proposed.** Chưa triển khai, chưa chạy build/test/CLI nào. P2 (đợt 6). Mọi khẳng định về code hiện có là do người soạn **đọc file thật** ngày 2026-10-06; chỗ chưa kiểm ghi "chưa kiểm chứng". Phần mới ghi "(mới)".

**CR:** [CR-CV-035](../../../../../../docs/crs/v7/code-intel-sources/CR-CV-035-storage-map.md)
**Service:** `code-intel-service` (mới, do `BE-CV-SOL-010` dựng) · `proto/orca/codeintel/v1`
**Hợp đồng áp dụng:** [`CONTRACT-codeintel-proto-and-data-map.md`](../../CONTRACT-codeintel-proto-and-data-map.md): **PQ-03** (mã lỗi hợp nhất), **PQ-04** (`WorktreeSelector`, không còn `repo_binding_id` trong request), **PQ-07** (tên file proto `codeintel_storage.proto`), **PQ-12** (phong bì phẳng + ETag), **PQ-14** (response ≤ 2 MiB, TTL snapshot 7 ngày), **PQ-15** (`graph_snapshots`), **PQ-24** (cờ tenant), **PQ-29** (`SourceRef`, `ServiceRef` dùng chung); §2.1 dòng 13, §3.1 dòng `GetStorageMap`, §4.2 T3 (view `storage`), §6.3 (quyền `read`). `CONTRACT-codeintel-ui-api.md` §2.3 (`CODEINTEL_SECRET_LEAK_BLOCKED`), §3.1 (kênh `codeIntel.storage`), §4.4 (kiểu `StorageMap`). Không dùng `CONTRACT-codeintel-agent-rpc.md` trực tiếp (chỉ đọc file qua cổng của `BE-CV-SOL-030`).
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (layout, "không chia sẻ domain type", `domain/` chỉ stdlib), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (DB-per-service, quy ước migration), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (§Input validation, §Multi-tenancy), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (§Event conventions: dạng subject, durable/ephemeral), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md); [`services/infra-fleet-service`](../../../../tdd/services/infra-fleet-service.md) §7 (đường relay tới agent, qua cổng đọc file).

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc (2026-10-06): `backend-go/docker-compose.yml` (58 dòng), `deploy/dev/docker-compose.yml` (758 dòng; dòng 30–75 `x-go-image`/`x-go-common-env`, 125–168 khối `postgres`, danh sách khối service ở dòng 197–681, 17 khối `migrate-*` ở 681–750, `volumes` ở 754–757), `deploy/dev/docker/postgres/init-databases.sh` (biến `DATABASES` 17 tên), `deploy/prod/docker-compose.yml` (78 dòng, chỉ dòng 40 nhắc `mysql://` ở dạng comment), `deploy/old/*.yml` (chỉ đếm dòng), `deploy/dev/` (có `.env` và `.env.example`), `backend-go/common/config/config.go` (`Base`, `StringEnv`), `backend-go/common/eventbus/eventbus.go` (`Event`, `EnsureStream`, `Subscribe`, `SubscribeEphemeral`), `backend-go/services/infra-fleet-service/internal/config/config.go` (đầu file), danh sách thư mục `internal/adapter/*` của cả 19 service, mọi lời gọi `EnsureStream` trong `services/*/cmd/server/main.go` (kết quả grep; gồm cả stream trace `tracing.TraceStreamName`), `proto/buf.yaml`.

Xác nhận đúng với CR: `backend-go/docker-compose.yml` chứa literal `POSTGRES_PASSWORD: orca` (dòng 17) và `VAULT_DEV_ROOT_TOKEN_ID: dev-root-token` (dòng 35); `deploy/dev/.env` tồn tại; 17 job `migrate-*`; Vault không chạy trong compose dev (`VAULT_ADDR: http://172.20.2.21:8200` literal trong `x-go-common-env`, `VAULT_TOKEN` là tham chiếu `${VAULT_TOKEN:?…}`); thư mục adapter `postgres` có ở đúng 17 service, `mysql` ở 16 (trừ `mcp-service`), `eventbus` ở 8 service (`ai-provider`, `automation`, `infra-fleet`, `issue-status-sync`, `notification`, `project`, `task`, `tenant`), `natsconsumer` ở `auth-service`, `vault` ở `auth` và `credential-broker`, `vaultsigner` ở `notification`, `cache` ở `tenant-service`; `code-intel-service` **chưa tồn tại** (`backend-go/services/code-intel-service` không có); `gopkg.in/yaml.v3` là dependency **trực tiếp** của `api-gateway` và `infra-fleet-service` (go.mod), gián tiếp ở các service khác.

### Correction relative to CR-CV-035

| # | CR nói | Mã thật / tính toán | Xử lý trong solution này |
|---|--------|---------------------|--------------------------|
| C1 | `Store postgres:dev:orca-go-postgres` | `orca-go-postgres` là **`container_name`** của khối `postgres` (dòng 128 compose dev), không phải tên dự án + service | `Store.name` = `container_name` nếu có (trường nằm trong allowlist), không thì `<name dự án>-<tên service>` |
| C2 | "nats (dev, JetStream)" | Cờ `-js` nằm ở `command` của khối `nats`, mà allowlist 2.3.2 **bỏ hẳn** `command` | `JetStream` không đọc được từ compose; suy từ việc `common/eventbus` dùng JetStream, ghi `confidence:"derived"`, không `declared` |
| C3 | Quy tắc 3a: khoá khớp `DSN` thì không lưu giá trị; quy tắc 3b: DSN parse bằng `net/url`, giữ `scheme/host/port/path` | Hai quy tắc **mâu thuẫn** với `DATABASE_DSN` (khoá khớp `DSN`). Tên DB (`/infra`) cần thiết để biết service nào dùng DB nào | Thứ tự cố định ở 2.3: khoá thuộc `urlKeys` đi qua bộ trích cấu trúc **trước** khi áp mẫu khoá nhạy cảm; đầu ra chỉ `{scheme, hostClass, port, dbName}` |
| C4 | `net/url` parse DSN `postgresql://orca:${POSTGRES_PASSWORD}@postgres:5432/infra?sslmode=disable` | Giá trị thật ở compose dev (dòng 218, 265…) chứa `${…}` trong userinfo; ký tự `{` `}` không hợp lệ trong userinfo của `net/url` (chưa chạy, suy từ quy tắc RFC 3986 mà `net/url` áp dụng) | Thay mọi `${…}` bằng một token trung tính trước khi parse; token không bao giờ ra payload; lỗi parse vẫn fail-closed |
| C5 | Entropy Shannon > 3,5 bit/ký tự với chuỗi ≥ 20 ký tự thì redacted | Tính tay (python, không chạm repo) trên giá trị thật: `gcr.io/distroless/static-debian12:nonroot` = 4,01; `infra-fleet-service:9090` (giá trị `INFRA_FLEET_SERVICE_ADDR`) = 3,75; `credential-broker-service:9090` = 3,92; `hashicorp/vault:1.17` = 4,02. Ngưỡng 3,5 sẽ che **mọi địa chỉ service** | Allowlist khoá + **văn phạm giá trị theo khoá** thay entropy cho khoá đã biết; entropy chỉ chạy ở bộ quét cuối trên **token** (xem 2.3) và kèm điều kiện chữ hoa + chữ thường + chữ số |
| C6 | Bảng adapter→kho không có thư mục cho volume | `git-gateway-service/internal/adapter/{localfs,localgit}` tồn tại; compose dev mount `git-gateway-repos:/data/repos` (dòng 482) | Thêm quy tắc `localfs|localgit` → Binding tới Store `volume` đã khai ở top-level `volumes`; CR chỉ có Store, thiếu Binding |
| C7 | `GetStorageMapRequest{repo_binding_id, …}` | PQ-04: request nhận `selector` | Theo hợp đồng (mục 4) |
| C8 | `deploy/` ở "gốc repo" | Đúng; còn `backend-go/deploy/postgres-init-databases.sh` là **bản thứ hai** của danh sách DB (CR chỉ nhắc `deploy/dev/docker/postgres/init-databases.sh`); chưa so hai bản có trùng nhau | Chỉ dùng bản `deploy/dev/docker/postgres/init-databases.sh`; bản kia không là nguồn (ghi vào 7) |

## 2. Giải pháp

### 2.A Cây file (mới)

```
backend-go/proto/orca/codeintel/v1/codeintel_storage.proto
backend-go/services/code-intel-service/
  internal/domain/storagemap/{storage_map.go, store_identity.go, canonical_order.go}      # kiểu + id ổn định + sắp xếp
  internal/domain/secretmasking/
      sensitive_source_paths.go          # denylist đường dẫn: đọc TRƯỚC khi gọi cổng đọc file
      environment_value_policy.go        # thứ tự quyết định giá trị biến môi trường (2.3)
      connection_url_masking.go          # {scheme, hostClass, port, dbName}, thay ${…}
      token_shape_detection.go           # tiền tố token + entropy theo token
      storage_map_leak_scan.go           # quét cuối payload đã serialize
  internal/usecase/get_storage_map.go                                                       # + khai báo port ở ports.go
  internal/adapter/composesource/{compose_document.go, init_databases_script.go}            # yaml.v3, anchor/merge key
  internal/adapter/gosourcescan/{service_config_env_keys.go, adapter_directory_kinds.go, nats_subject_literals.go}  # go/parser
  internal/adapter/grpc/storage_map_handler.go
  testdata/storage/{compose-dev.yml, compose-prod.yml, compose-old.yml, backend-go-compose.yml, init-databases.sh, services/**, PROVENANCE.txt}
```

`domain/` chỉ import stdlib (arch/03). Parser YAML và `go/parser` nằm ở `adapter/` vì phụ thuộc thư viện ngoài (`yaml.v3`) dù thuần hàm; chúng cài port `ComposeDocumentParser`, `GoSourceScanner` khai báo ở `usecase/ports.go`. Tên gói cụ thể, không `helpers/utils/common` (AGENTS.md).

### 2.B Proto `codeintel_storage.proto` (mới; PQ-07, PQ-29)

Số field **do chủ sở hữu CR gán** theo thứ tự khai báo ở CR-035 §2.1 (hợp đồng §2 quy tắc số field); chốt ở TASK-035-02:

```proto
message StorageMap { repeated Store stores = 1; repeated Binding bindings = 2; repeated Topic topics = 3;
  repeated SourceRef sources = 4; int32 redacted_count = 5; string as_of_commit = 6; repeated string warnings = 7; }
message Store { string id = 1; string kind = 2; string name = 3; string env = 4; ServiceRef owner = 5;
  repeated string schemas = 6; bool deployed = 7; bool supported_by_code = 8; bool external = 9;
  repeated SourceRef evidence = 10; string confidence = 11; string change = 12; }   // change: dành sẵn, v7 không điền
message Binding { string service = 1; string store = 2; string access = 3; string via = 4; string config_key = 5;
  repeated SourceRef evidence = 6; string confidence = 7; string change = 8; string database = 9; } // database: (mới) xem 4
message Topic { string name = 1; string stream = 2; repeated ServiceRef publishers = 3; repeated ServiceRef subscribers = 4;
  string delivery = 5; string payload = 6; repeated SourceRef evidence = 7; string confidence = 8; }
message GetStorageMapRequest { WorktreeSelector selector = 1; string env_filter = 2; bool include_legacy = 3; }
message GetStorageMapResponse { StorageMap map = 1; ResultMeta meta = 2; }   // meta.view = VIEW_KIND_STORAGE (mới trong ViewKind)
```

`kind`, `env`, `access`, `delivery`, `confidence` là chuỗi chữ thường (PQ-32). `SourceRef.kind` dùng tập mở rộng `compose|config|adapter|migration|code` của PQ-29. Thêm một dòng `rpc GetStorageMap(GetStorageMapRequest) returns (GetStorageMapResponse);` vào `service CodeIntelService` ở `codeintel.proto`; không khai báo RPC nào khác.

### 2.C Nguồn và thứ tự trích xuất

Mọi đọc file đi qua `RepoSourceReader` của `BE-CV-SOL-030` (S1); solution này **không** gọi `fs.*`/`git.*`. Chữ ký của cổng do solution đó chốt; ở đây chỉ giả định bốn khả năng: đọc một đường dẫn tương đối gốc repo với hạn mức kích thước; liệt kê theo tiền tố/glob; tìm literal (grep) để thu hẹp tệp; trả `ContentID` (blob oid hoặc sha256) rẻ cho từng tệp. **Chưa kiểm chứng** từng khả năng; TASK-035-01 phải xác nhận trước khi viết nối.

1. **Compose + init script (danh sách cố định, không quét `deploy/**`)**: `backend-go/docker-compose.yml`, `deploy/dev/docker-compose.yml`, `deploy/prod/docker-compose.yml`, `deploy/old/docker-compose.yml`, `deploy/old/docker-compose.orca.yml`, `deploy/old/docker-compose.orca.artifact.yml`, `deploy/dev/docker/postgres/init-databases.sh`. `include_legacy=false` (mặc định) bỏ ba tệp `deploy/old/`: **không đọc**, không chỉ ẩn.
2. **Cấu hình service**: với mỗi service (danh sách từ `BE-CV-SOL-033-c4-component-view`, mềm; thiếu thì liệt kê `backend-go/services/*` qua cổng đọc), `internal/config/config.go` → tên khoá env bằng `go/parser` (`os.Getenv("X")`, `commonconfig.StringEnv("X", def)`, hàm `*Env("X", …)` cục bộ như `intEnv`/`boolEnv`). Chỉ lấy **tên khoá**; giá trị mặc định chỉ khi khoá nằm allowlist và khớp văn phạm (2.3). `api-gateway` còn `config_mcp.go` (CR nhắc, chưa đọc): đọc mọi `internal/config/*.go` không `_test.go`.
3. **Adapter**: liệt kê thư mục con `internal/adapter/` → bảng cố định: `postgres`→Store postgres, `mysql`→Store mysql, `eventbus|natsconsumer`→NATS, `vault|vaultsigner`→Vault, `localfs|localgit`→volume (C6), `cache`→bỏ (LRU trong tiến trình). Các thư mục còn lại (`grpc`, `grpcclient`, `opaclient`, …) không phải kho, bỏ.
4. **Migration → schema, owner**: lấy từ `ErdModel.{service, schema}` của `BE-CV-SOL-031-erd-model-and-access-scan` (mềm); thiếu thì `owner` rỗng và `warnings += "erd_unavailable"`.
5. **Topic**: 2.D.
6. **Hợp nhất**: Store trùng `(kind, env, name)` gộp, giữ mọi `evidence`; sắp xếp xác định (`canonical_order.go`: `kind`, `env`, `name`; Binding theo `(service, store, via)`; Topic theo `name`).

Quy tắc Store:

| Nguồn | Store |
|---|---|
| khối `postgres` compose dev | `postgres:dev:<container_name>`, `deployed=true`, `supportedByCode=true`, `schemas` = 17 tên trong `DATABASES` |
| khối `nats` | `queue:dev:<container_name|dự án-nats>`, `deployed=true`, `confidence:"derived"` cho JetStream (C2) |
| không có khối, nhưng `VAULT_ADDR` trong `x-go-common-env` | `vault:dev:shared-external`, `external=true`, `deployed=false` (không đọc host) |
| adapter `mysql` + không compose nào có MySQL | `mysql:dev:supported-by-code`, `supportedByCode=true`, `deployed=false` |
| top-level `volumes` | `volume:<env>:<tên>`; dev: `orca-go-postgres-data`, `git-gateway-repos`; prod: `orca-data` |
| compose prod | chỉ khối `orca-server` + volume `orca-data`; `warnings += "prod_topology_unknown"` (không dựng topology Go) |
| compose `old` | `env:"legacy"`, chỉ khi `include_legacy=true` |

Không tạo Store `redis`/object (chỉ có trong comment, `tenant-service/cmd/server/main.go:186` và `api-gateway/internal/usecase/rate_limit.go:13` theo CR, chưa đọc lại hai dòng này).

### 2.D Topic NATS (heuristic, mọi kết luận có `confidence`)

1. **Thu hẹp**: dùng grep của cổng đọc với mẫu `"orca\.[a-z0-9_]+\.` trên `backend-go/services/*/internal/**` và `backend-go/common/eventbus`, loại `_test.go`, `/gen/`. Thiếu grep → đọc theo thư mục `adapter/eventbus`, `adapter/natsconsumer`, `domain/*event*.go`, `cmd/server/main.go` và `warnings += "topics_partial_no_grep"` (không im lặng).
2. **Trích literal bằng `go/parser`** (không regex trên text): chỉ literal chuỗi trong `const`, `var`, composite literal `SubjectBinding{…}`, đối số `Publish|PublishDedup|EnsureStream|Subscribe|SubscribeEphemeral`, trường `Subject:`; khớp `^orca\.[a-z0-9_]+(\.[a-z0-9_]+){1,3}$` hoặc `orca\.[a-z0-9_]+\.>`. Comment và chuỗi log bị bỏ.
3. **Vai trò**: publisher (`declared`) khi literal là đối số/trường `Subject` của `Publish*`/`outbox` hoặc nằm trong `adapter/eventbus/publisher*.go`; subscriber (`declared`) khi trong `[]SubjectBinding{…}` hoặc đối số `Subscribe*` hoặc `adapter/{eventbus,natsconsumer}`; còn lại `inferred`, vai trò không gán. `delivery:"ephemeral"` chỉ khi thấy `SubscribeEphemeral` cùng subject; ngược lại `durable` khi qua `Subscribe` có tên consumer, còn không `unknown`.
4. **Stream**: từ `SubjectBinding.StreamName` hoặc `EnsureStream(name, subjects)`; subject cụ thể thuộc wildcard (`orca.project.>`) thừa hưởng `stream`. Hai tiền tố `orca.infra.*` và `orca.infrafleet.*` **giữ nguyên**, không chuẩn hoá (đã thấy `INFRAFLEET` ↔ `orca.infrafleet.>` ở `infra-fleet-service/cmd/server/main.go:268` và `INFRA` ↔ `orca.infra.agent.>` ở :558).
5. **Stream mới của series**: `CODEINTEL` (`orca.codeintel.>`, hợp đồng §5) chỉ xuất hiện sau khi `code-intel-service` merge; map phản ánh mã hiện tại, không bịa.
6. `Topic.payload` chỉ là tên kiểu nếu có struct kề; không đọc dữ liệu thật. Bản đồ không cam kết có thông điệp chạy ở runtime.

### 2.E Quy tắc che secret (ràng buộc cứng; thứ tự cố định, sửa C3–C5)

Mặc định mọi giá trị là bí mật; chỉ giá trị đi qua đủ cổng mới thoát.

1. **Denylist đường dẫn (trước khi đọc)**: `**/.env`, `**/.env.*` (trừ `.env.example`: chỉ lấy **tên khoá**, bỏ giá trị và comment), `*.pem`, `*.key`, `*.p12`, `/vault/secrets/**`, `orca-policy.hcl`, mọi thứ ngoài worktree. Kiểm ở **hai lớp**: cổng đọc file (`BE-CV-SOL-030`) và `sensitive_source_paths.go` trước mọi `Read`; test dùng cổng giả ghi lại mọi yêu cầu để khẳng định `.env` không bao giờ được hỏi.
2. **Allowlist trường compose**: `services.<n>.{image, container_name, profiles, depends_on, networks, ports (chỉ cổng phía container), volumes (tên named volume + đích mount; bỏ nguồn bind-mount là đường dẫn máy chủ), environment (chỉ TÊN khoá, giá trị theo bước 3)}`, top-level `volumes`, `networks`, `name`. `command`, `healthcheck`, `logging`, `labels`, `security_opt`, comment: **bỏ hẳn**. Trường cấu trúc (`image`, `container_name`, tên volume) qua văn phạm `^[a-z0-9][a-z0-9._/:@-]{0,200}$`, **không** qua entropy (C5).
3. **Giá trị biến môi trường, theo thứ tự**: (a) chứa `${…}`: chỉ lưu tên biến, **bỏ** mặc định và thông báo lỗi; (b) khoá thuộc `urlKeys` (`DATABASE_DSN`, `NATS_URL`, `VAULT_ADDR`, mọi `*_ADDR` có dạng `host:port` hoặc URL): thay `${…}` bằng token trung tính, parse `net/url`, giữ `{scheme, hostClass, port, dbName}`, **bỏ** userinfo/query/fragment; host IP literal hoặc ngoài mạng compose ⇒ `hostClass:"external"` và không lưu host; lỗi parse ⇒ bỏ cả giá trị; (c) khoá khớp `(?i)(SECRET|TOKEN|PASSWORD|PASSWD|PASSPHRASE|PRIVATE|CREDENTIAL|API_?KEY|AUTH|SALT|SIGNING|CURSOR_KEY|DSN)` mà không thuộc `urlKeys` ⇒ `{name, secretRef:true}`, không giá trị; (d) khoá thuộc allowlist không nhạy cảm (`*_PORT`, `*_ENABLED`, `ORCA_SERVER_DEPLOYMENT`, `FLEET_POLL_INTERVAL_SEC`, `EPHEMERAL_VM_SSH_MODE`, `OPA_BUNDLE_PATH`) **và** giá trị khớp văn phạm riêng của khoá (số, bool, đường dẫn tương đối `^[A-Za-z0-9._/-]+$`) ⇒ giữ; (e) còn lại ⇒ `«redacted»`, `redacted_count++`.
4. **Literal trong compose cũng tuân quy tắc**: `POSTGRES_PASSWORD: orca`, `VAULT_DEV_ROOT_TOKEN_ID: dev-root-token` (hai literal thật ở `backend-go/docker-compose.yml`) ra `secretRef:true` qua bước 3c, không ngoại lệ "giá trị dev".
5. **Vault**: chỉ `Store{kind:"vault", name, external, deployed}` và tên khoá/đường dẫn **không phải giá trị**; không `VAULT_TOKEN`, không nội dung policy.
6. **Quét cuối (`storage_map_leak_scan.go`)**: chạy trên payload **đã serialize** trước khi ghi cache hoặc trả RPC: (i) userinfo trong URL `://[^/@\s]+:[^/@\s]+@`; (ii) tiền tố `hvs.`, `ghp_`, `glpat-`, `xox`, `sk-`, `AKIA`; (iii) **token** (tách theo `/ : @ . , ; =` và khoảng trắng) dài ≥ 20, ký tự `[A-Za-z0-9+/=_-]`, có đủ chữ hoa, chữ thường, chữ số **và** entropy ≥ 4,0 (giá trị khởi điểm, hiệu chỉnh bằng fuzz; tên service toàn chữ thường nên không dính). Thấy ⇒ không trả view, trả `CODEINTEL_SECRET_LEAK_BLOCKED`, ghi metric và log **chỉ đường dẫn tệp nguồn**, không log giá trị.
7. **Không có "chế độ hiện secret"**, kể cả admin. Log/span chỉ có đường dẫn và số lượng; không log nội dung tệp.

### 2.F Use case, cache, RPC

`usecase.GetStorageMap`: (1) kiểm cờ + quyền `read` + phân giải `selector` (do `BE-CV-SOL-012/013` cung cấp, port có sẵn); (2) đọc nguồn qua cổng (song song có hạn mức, mỗi `Read` có timeout riêng); (3) parse từng nguồn độc lập: lỗi một compose ⇒ `warnings += "compose_parse_failed:<path>"`, các nguồn khác vẫn trả; (4) hợp nhất + sắp xếp; (5) quét cuối; (6) ghi `graph_snapshots(view="storage")` qua port của `BE-CV-SOL-022-snapshot-cache` với `params_hash = sha256(env_filter | include_legacy | parserVersion | các ContentID đã đọc, đã sắp xếp)`; (7) trả `ResultMeta{etag, head_commit, truncated, total_count}`. Khoá nội dung (`ContentID`) làm cache an toàn cả khi worktree bẩn; nếu cổng không trả được `ContentID` rẻ ⇒ không ghi snapshot DB, chỉ cache bộ nhớ TTL 30 s (cùng cách CR-036 §2.7). Singleflight theo khoá. Response ≤ 2 MiB (PQ-14), thực tế vài chục KiB.

Mã lỗi (PQ-03): `CODEINTEL_DISABLED`, `CODEINTEL_NOT_AUTHORIZED`, `CODEINTEL_DEV_SERVER_OFFLINE`, `CODEINTEL_TIMEOUT` (hậu tố `{"retryAfterMs":3000,"inProgress":true}`, hoàn tất nền), `CODEINTEL_PATH_NOT_ALLOWED`, `CODEINTEL_SECRET_LEAK_BLOCKED` (gRPC `Internal`: hợp đồng chưa ghi mã trạng thái, xem 4).

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|------------|-------|
| Đọc file thô, không `docker compose config`, không nối DB | `config` nội suy `.env` và lộ secret; E14 ngoài phạm vi |
| Allowlist + văn phạm theo khoá thay entropy toàn chuỗi | Số đo C5: ngưỡng 3,5 che mọi `*_ADDR`; văn phạm kiểm chứng được bằng test, entropy thì không |
| Thứ tự `urlKeys` trước mẫu khoá nhạy cảm | Giải mâu thuẫn C3; không cần tên DB thì bỏ được `DATABASE_DSN` hoàn toàn nhưng lens Lưu trữ sẽ mất liên kết service↔DB |
| Fail-closed ở hai tầng (parse lỗi bỏ giá trị; quét cuối chặn cả view) | "Ít còn hơn lộ" (CR §3.7) |
| Tách `deployed` và `supportedByCode` | MySQL có adapter, migration, không compose nào chạy |
| Prod = `prod_topology_unknown` | `deploy/prod/docker-compose.yml` chỉ có `orca-server`; vẽ topology Go là bịa |
| `include_legacy=false` thì **không đọc** `deploy/old/` | Giảm bề mặt đọc, giảm chi phí qua SSH 50–200 ms |
| Cache khoá theo `ContentID` | Worktree bẩn vẫn đúng; commit đổi ⇒ khoá đổi |
| Không hỗ trợ SSH relay riêng | Mọi dữ liệu đến qua agent (`direct-websocket`, Part A); không phụ thuộc provider git (GitHub/GitLab) nên không có nhánh riêng cho GitLab |

## 4. Lệch giữa CR và hợp đồng

| # | CR-CV-035 | Hợp đồng | Theo |
|---|-----------|----------|------|
| L1 | `GetStorageMapRequest{repo_binding_id, env_filter?, include_legacy}` | PQ-04: `selector` (`{project_id, worktree_ref}`) | Hợp đồng |
| L2 | `view="storage"` là "giá trị mới" của envelope | §2.3 `ResultMeta` phẳng (PQ-12); `view` lưu DB có `storage` (§4.2 T3) | Hợp đồng; thêm `VIEW_KIND_STORAGE` vào `ViewKind` (cần `BE-CV-SOL-020` chấp thuận) |
| L3 | `StorageMap` không có `warnings` | UI §4.4: `StorageMap.warnings: string[]` | Thêm `warnings=7` (lỗi parse từng tệp ghi ở đây, không ở `sources` vì `SourceRef` không có trạng thái) |
| L4 | `Store.owner?: ServiceRef` | UI §4.4: `owner?: {name}` | Gateway ánh xạ; proto giữ `ServiceRef` (PQ-29) |
| L5 | `Topic.publishers: ServiceRef[]` | UI: `publishers: string[]` | Gateway ánh xạ sang tên |
| L6 | Không có `change` | UI §4.4: `Store.change`, `Binding.change` | Dành sẵn trường, v7 **không điền** (request không có `base_ref`); báo chủ hợp đồng |
| L7 | `Binding` không có tên DB, mà lens cần "service nào dùng DB nào" | Hợp đồng và UI không có trường | Thêm `Binding.database` (đề xuất additive, `buf breaking` an toàn); **cần thêm vào hợp đồng §4.4 và UI §4.4** |
| L8 | `CODEINTEL_SECRET_LEAK_BLOCKED` | UI §2.3 có mã, **không** có gRPC status; CR gọi "(mới)" | Chọn `Internal` (lỗi hệ thống, không do người gọi); báo chủ hợp đồng bổ sung cột |
| L9 | Cache `(repo, commit, "storage", params_hash)` | PQ-15: `(tenant, repo_binding_id, view, head_commit, params_hash)` | Hợp đồng; `params_hash` gồm `ContentID` (2.F) |

## 5. Phụ thuộc chéo khu vực và thứ tự

| Cần | Từ | Dạng |
|-----|----|------|
| Khung service, `apperrors` Kind mới | `BE-CV-SOL-010-scaffold-code-intel-service` | cứng |
| `codeintel_common.proto` (`SourceRef`, `ServiceRef`, `WorktreeSelector`, `ResultMeta`, `ViewKind`) | `BE-CV-SOL-020-canonical-graph-model` | cứng (G0) |
| `RepoSourceReader` + chặn đường dẫn | `BE-CV-SOL-030-repo-file-access-gateway` | cứng |
| Phân giải `selector`, quyền, cờ | `BE-CV-SOL-012-target-resolution-and-bindings`, `BE-CV-SOL-013-authorization-flags-and-audit` | cứng |
| Ghi/đọc `graph_snapshots` | `BE-CV-SOL-022-snapshot-cache` (hoặc `BE-CV-SOL-011-repositories-and-maintenance`) | mềm: thiếu thì chỉ cache bộ nhớ |
| `ErdModel.{service,schema}` | `BE-CV-SOL-031-erd-model-and-access-scan` | mềm |
| Danh sách container/service | `BE-CV-SOL-033-c4-component-view` | mềm |
| Đăng ký kênh `codeIntel.storage` | `BE-CV-SOL-040-codeintel-view-channels` | **sau** solution này (kênh trong §3.1; `TestChannelInventory`) |
| Test che secret end-to-end | `BE-CV-SOL-072-security-tests-service-gateway` | sau |
| Phía agent / frontend | **không có** (AG: `—`; FE: `FE-CV-SOL-058-storage-lens` chỉ tiêu thụ) | — |

Thứ tự theo hợp đồng §7.2: 030 → 031 → 032 → 033 → 034 → **035** (P2, đợt 6, "khi đã có đủ cấu hình triển khai"); `DataFlow.stores` của `BE-CV-SOL-034-data-flow-model` dùng `Store.id` của solution này nên hai bên phải khớp quy tắc id `<kind>:<env>:<name>` (TASK-035-03).

## 6. Kiểm thử

- **Fixture vàng** (`testdata/storage/`, đã che tay, chỉ literal giả; `PROVENANCE.txt` ghi từng tệp lấy từ đâu và đã sửa gì): snapshot JSON `StorageMap`, so khớp khi đổi parser. Lệnh: `go test ./services/code-intel-service/internal/... -run 'StorageMap|SecretMasking|ComposeSource|GoSourceScan'`.
- **Unit parser**: anchor/merge key (`<<: *go-defaults`); `${POSTGRES_PASSWORD:?msg}`; DSN có `${…}` trong userinfo, có `?sslmode`; DSN hỏng; khoá nhạy cảm literal; comment chứa IP; `command` bị bỏ.
- **Unit Go AST**: `StringEnv`/`os.Getenv`/hàm `*Env` cục bộ; subject trong comment bỏ; `SubjectBinding` nhiều phần tử; `EnsureStream` wildcard.
- **Bảo mật** (đưa vào `BE-CV-SOL-072`): fuzz giá trị env; **canary** (chuỗi bí mật giả đặt trong `.env` fixture + literal compose) không xuất hiện ở payload, cache, log, metric, span (quét byte); cổng đọc giả khẳng định `.env` không bị hỏi; tiêu chí 2 tầng: chèn cố ý secret vào fixture ⇒ `CODEINTEL_SECRET_LEAK_BLOCKED`.
- **Cô lập tenant**: hai tenant, cùng `head_commit` và `params_hash`: không đọc chéo snapshot; mọi cuộc gọi cổng đọc mang tenant của người gọi.
- **Hợp đồng**: so danh sách service/adapter với cây thật qua cổng đọc (test chạy trên fixture cây thư mục chụp từ repo, không hardcode); `buf lint`, `buf breaking`.
- **Hai dialect DB**: solution không thêm truy vấn DB riêng (dùng port snapshot của `BE-CV-SOL-022`); ma trận `dialect:[postgres, mysql]` thuộc solution đó. Nếu 022 chưa merge, nhánh snapshot của solution này test bằng fake và ghi rõ.
- **Chưa chạy bất kỳ test nào.**

## 7. Rủi ro và điểm chưa kiểm chứng

- Chưa chạy hệ thống; số liệu (19 khối service, 17 DB, 17 `migrate-*`, ~120 literal subject) là kết quả đọc, chưa chạy compose.
- API thật của `RepoSourceReader` (liệt kê, grep, `ContentID`) chưa biết; toàn bộ 2.C/2.D/2.F phụ thuộc vào TASK-035-01.
- `net/url` với `${…}`: giả định lỗi parse (C4) chưa chạy; nếu parse được thì bước thay token là thừa nhưng vô hại.
- Ngưỡng entropy 4,0 ở quét cuối là giả định; chưa đo báo nhầm trên tên dài (`credential-broker-service`) và checksum, hash (`sha256`) hợp lệ có thể nằm trong tên.
- Heuristic publisher/subscriber sai ở literal gián tiếp (hằng truyền nhiều lớp, ghép chuỗi); chưa đo tỷ lệ; nghiệm thu đối chiếu tay 20 topic (CR §6).
- `api-gateway` có nhiều literal subject (cầu nối push `wscompat`, `mcpserver/resources`); vai trò từng điểm chưa kiểm chứng ⇒ để `inferred`.
- `cmd/server/main.go` có thể rất lớn (chứa `EnsureStream`); `go/parser` cần hạn mức kích thước riêng.
- Các service chưa đọc `internal/config/config.go` từng cái (chỉ `infra-fleet` và đầu `task`): giả định mẫu `Getenv/StringEnv` chung chưa kiểm chứng.
- `backend-go/deploy/postgres-init-databases.sh` (bản thứ hai danh sách DB) có thể lệch bản `deploy/dev/docker/postgres/`; chưa so.
- Cấu hình thật có thể nằm ngoài repo (Vault Agent render `/vault/secrets/database-credentials`): bản đồ chỉ phản ánh **khai báo trong repo**.
- Relay SSH (Part B) không có `fs.*`/`git.*` kiểu này ở MVP (O-5): chỉ `direct-websocket`.

## 8. Câu hỏi mở

1. Compose prod thật của backend-go lấy từ đâu (E3)? Mặc định `prod_topology_unknown`.
2. Có cho hiện host nội bộ của Vault dùng chung? Mặc định không (`external:true`).
3. `deploy/old/` còn đáng đọc? Mặc định có nhưng chỉ khi `include_legacy=true`.
4. Thêm `Binding.database` (L7) và `StorageMap.warnings` (L3) vào hợp đồng: chủ hợp đồng chấp thuận?
5. Hai tiền tố `orca.infra.*` / `orca.infrafleet.*` đánh dấu "không nhất quán" như finding của `BE-CV-SOL-037`? Mặc định không.
6. Gán gRPC status cho `CODEINTEL_SECRET_LEAK_BLOCKED` (L8).

## 9. Tiêu chí chấp nhận

- [ ] Trên fixture chụp từ repo: Store `postgres` dev có đủ 17 DB; `queue` NATS dev (JetStream `derived`); `vault` dev `external:true, deployed:false`; volume `orca-go-postgres-data`, `git-gateway-repos`; prod chỉ `volume orca-data` + `prod_topology_unknown`; `deploy/old` chỉ khi `include_legacy=true`.
- [ ] `mysql` hiện với `supportedByCode=true, deployed=false`; không có Store `redis`.
- [ ] Mọi service có `adapter/postgres` có Binding postgres với `config_key="DATABASE_DSN"` và `database` đúng; `mcp-service` không có Binding MySQL; `git-gateway-service`, `api-gateway` không có Binding DB; `git-gateway-service` có Binding tới volume.
- [ ] Topic `orca.infrafleet.terminal.closed` có publisher `infra-fleet-service`; `orca.orchestration.task.statuschanged` có subscriber `task-service`; tên `orca.infra.*`/`orca.infrafleet.*` giữ nguyên; mỗi Topic có `confidence` và `evidence` (đường dẫn + dòng).
- [ ] Payload chứa 0 lần `dev-root-token`, mật khẩu `orca`, giá trị `.env` canary, userinfo URL, IP Vault; `redacted_count > 0`.
- [ ] `.env` không bao giờ bị hỏi qua cổng đọc (cổng giả).
- [ ] Quét cuối chặn khi chèn secret; RPC trả `CODEINTEL_SECRET_LEAK_BLOCKED`; log không chứa giá trị.
- [ ] Lỗi parse một compose chỉ thêm `warnings`; các nguồn khác vẫn trả.
- [ ] Cùng đầu vào ⇒ cùng thứ tự `Store`/`Binding`/`Topic` qua 100 lần chạy; commit/`ContentID` đổi ⇒ khoá cache đổi.
- [ ] `buf lint`, `buf breaking` pass; không khai báo RPC chưa có message.

## 10. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/code-intel-sources/CR-CV-035-storage-map.md`, `/opt/repos/orca/docs/crs/v7/README.md` (§2 O1–O14, §6, §8)
- `/opt/repos/orca/backend-go/docker-compose.yml`, `/opt/repos/orca/deploy/dev/docker-compose.yml`, `/opt/repos/orca/deploy/dev/docker/postgres/init-databases.sh`, `/opt/repos/orca/deploy/prod/docker-compose.yml`, `/opt/repos/orca/deploy/old/`
- `/opt/repos/orca/backend-go/common/config/config.go`, `/opt/repos/orca/backend-go/common/eventbus/eventbus.go`
- `/opt/repos/orca/backend-go/services/*/cmd/server/main.go` (`EnsureStream`), `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/config/config.go`
- `/opt/repos/orca/backend-go/proto/buf.yaml`, `/opt/repos/orca/AGENTS.md`, `/opt/repos/orca/guides/STYLEGUIDE.md`
- Series: `BE-CV-SOL-010`, `020`, `022`, `030`, `031-*`, `033-*`, `034`, `040-codeintel-view-channels`, `072`; `FE-CV-SOL-058-storage-lens`

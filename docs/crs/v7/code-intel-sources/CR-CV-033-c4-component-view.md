# CR-CV-033 — C4 level 3 (Component): quy tắc suy từ cấu trúc hexagonal và ghi đè bằng `c4.yaml`

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-033 |
| **Tên** | Dựng `C4ComponentView` cho một container (service Go) từ `internal/{domain,usecase,adapter/*}`, quan hệ từ import/implements/proto/SQL, thành phần ngoài suy từ import, và hợp nhất với `c4.yaml` do người dùng ghi đè |
| **Loại** | Feature (suy luận + định dạng cấu hình + RPC `GetArchitecture`, `GetC4Overrides`, `SaveC4Overrides`) |
| **Priority** | 🟠 P1 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-030 (đọc file), CR-CV-032 (cạnh gọi RPC, kênh), CR-CV-031 (truy cập bảng, tuỳ chọn), CR-CV-011 (bảng `c4_overrides`), CR-CV-013 (phân quyền ghi), CR-CV-020 (`SymbolRef`); tuỳ chọn CR-CV-002 (cạnh `IMPORTS` của GitNexus) |
| **Mở khoá** | CR-CV-034 (gán bước luồng vào component), CR-CV-037 (luật lớp), CR-CV-055 (lens C4 + chỉnh `c4.yaml`) |
| **Tác động** | `backend-go/proto/orca/codeintel/v1/c4.proto` (mới), `code-intel-service/internal/domain/c4` (mới), `internal/usecase/derive_c4_view.go`, `merge_c4_overrides.go` (mới), `internal/adapter/gopackagegraph` (mới, dùng `go/parser`), `internal/adapter/c4overrides` (đọc/ghi `c4_overrides`) |
| **Phụ thuộc dữ liệu ngoài** | **E6** (quy tắc C4 do con người: tên, gộp/tách, mô tả, ranh giới, thành phần ngoài), **E17** (hệ thống ngoài làm "external"), E15 (ADR, chưa đọc), E4/E10 (tên topic, tuỳ chọn; thuộc CR-CV-035) |

---

## 1. Bối cảnh và vấn đề

08 §3: công cụ không có khái niệm "component"; `Community` của GitNexus chỉ gom theo mật độ gọi. Cách khả thi là dùng bố cục hexagonal làm quy tắc mặc định rồi cho người dùng ghi đè. Khảo sát code (2026-10-05):

1. **Bố cục.** Mọi service Go đã xem theo `internal/{domain,usecase,adapter,config}` và `cmd/server/main.go` (composition root). `infra-fleet-service`: 207 file Go không-test, ~1,05 MiB (domain 24, usecase 96, 15 thư mục `adapter/*`: `agentwsserver`, `backendrelaysshprovisioner`, `devserveragent`, `ephemeralsshconn`, `eventbus`, `grpc`, `grpcclient`, `metrics`, `mysql`, `portalloc`, `portevents`, `postgres`, `sshconn`, `sshrelay`, `webhook`). `api-gateway` có bộ adapter khác (`authclient`, `fanout`, `grpc`, `httpgateway`, `mcp*`, `originpolicy`, `wsbridge`, `wscompat`), nên danh sách adapter không cố định; quy tắc phải dựa vào cấu trúc, không dựa vào tên.
2. **Cổng (port) và cài đặt.** Cổng là interface ở `internal/usecase` (`infra-fleet-service/internal/usecase/ports.go`: 41 interface, kèm vài interface ở file khác). Bằng chứng "adapter cài đặt cổng" nằm ở dòng kiểm tra lúc biên dịch `var _ usecase.X = (*T)(nil)`: có 86 dòng như vậy trên toàn bộ `adapter/` của các service (đếm bằng `grep`), nhưng phân bố **không đều**: `infra-fleet-service/adapter/mysql` có 29 dòng (khối `var (...)` trong `shared.go`), `adapter/postgres` chỉ 3 (comment trong `mysql/shared.go` thừa nhận "postgres chỉ assert 2 kiểu"). Vì vậy dòng `var _` là bằng chứng mạnh nhưng **không đủ**; cần khớp tập phương thức làm bằng chứng phụ.
3. **GitNexus không cung cấp `implements` cho Go theo cấu trúc.** `cypher` trên Orca: cạnh `IMPLEMENTS` ở `infra-fleet-service/internal/adapter` chỉ có 3 cạnh do nhúng struct ("scope-resolution: inherits"), không có cạnh nào cho các `Repository` Postgres/MySQL (xem CR-CV-032 mục 1.5). Không dùng làm nguồn chính.
4. **Mô tả có sẵn.** Mỗi package adapter của `infra-fleet-service` mở đầu bằng comment `// Package <tên> …` mô tả mục đích (ví dụ `sshconn`: "connection-establishment half of relay-ssh mode — … Vault-signed short-lived certificate …"; `agentwsserver`: "INBOUND counterpart"; `eventbus`: "implements usecase.LifecycleEventPublisher against NATS"; `webhook`, `portalloc`, `metrics`, `portevents`, …). `internal/domain` và `internal/usecase` có package doc ở một file (`domain/dev_server.go`, `usecase/ports.go`). Đây là nguồn mô tả tự động rẻ và đúng phần lớn, nhưng là suy luận (có thể nói về chi tiết thay vì vai trò).
5. **Thư viện ngoài thể hiện hạ tầng.** Import trong `adapter/*` của `infra-fleet-service` (đọc bằng `grep`): `pgx/v5`, `nats-io/nats.go` (ở `adapter/postgres`, cho outbox), `go-sql-driver/mysql`, `golang.org/x/crypto/ssh` và `pkg/sftp` (`sshconn`, `sshrelay`, `backendrelaysshprovisioner`, `ephemeralsshconn`), `coder/websocket` (`agentwsserver`, `devserveragent`), `prometheus/client_golang` (`metrics`), `google.golang.org/grpc` (`grpcclient`). Các adapter như `eventbus` import `common/eventbus` thay vì thư viện NATS trực tiếp, nên quy tắc phải xét cả package `common/*`.
6. **Chưa có `c4.yaml` nào** trong repo (tìm `c4*.y*ml` ngoài `node_modules`: không thấy). `c4_overrides` mới chỉ có trong README v7 mục 3.5 (cột `container`, `document`, `version`).
7. Cạnh gọi RPC và đọc/ghi bảng lấy từ CR-CV-032 và CR-CV-031; luồng bước (CR-CV-034) cần gán vào component, nên `id` component phải ổn định.

## 2. Giải pháp đề xuất

### 2.1 Đơn vị vẽ và phạm vi MVP

- Một `C4ComponentView` cho **một container**; container = thư mục `backend-go/services/<svc>`. Bộ container khác (`agent`, `frontend`, `desktop`, README v7 08 §3) **ngoài phạm vi CR này**: chỉ xuất hiện như thành phần ngoài hoặc container không có component. Lý do: quy tắc hexagonal chỉ khảo sát cho Go; mã TypeScript cần quy tắc khác (Q1).
- Cố định ưu tiên làm thử: `infra-fleet-service`, `git-gateway-service`, `api-gateway` (08 §8).

### 2.2 Quy tắc suy container và component (mặc định)

Đầu vào: cây `internal/**` và `cmd/**` của service (đọc qua CR-CV-030) và thông tin package (`go/parser`: `package`, doc, các khai báo, `import`).

| Thư mục | Component | `kind` | Ghi chú |
|---|---|---|---|
| `internal/domain` | `domain` | `domain` | một component, kể cả khi có nhiều file |
| `internal/usecase` | `usecase` | `usecase` | một component; số cổng (interface) đưa vào `techHint`/số liệu, không tách mặc định |
| `internal/adapter/<n>` (mỗi thư mục lá có file Go không-test) | `adapter-<n>` | xem dưới | thư mục lồng (ví dụ `grpcclient/authclient`) thành component riêng `adapter-grpcclient-authclient` |
| `internal/config` | `config` | `config` | |
| `internal/<khác>` | `internal-<n>` | `other` | |
| `cmd/server` | `main` | `other` | **ẩn mặc định** (là gốc kết nối mọi thứ, chỉ gây nhiễu); vẫn dùng làm bằng chứng wiring |

`kind` của adapter, theo thứ tự ưu tiên: (a) package đăng ký server qua `Register<Svc>Server`/chứa phương thức cài đặt `Unimplemented<Svc>Server` → `grpc-server`; (b) có tham chiếu kiểu `<pkg>v1.<Svc>Client` (kết quả CR-CV-032) → `grpc-client`; (c) còn lại → `adapter`.

`techHint` suy từ import (bảng cấu hình được, mặc định):

| Import chứa | `techHint` | Thành phần ngoài (`kind`) |
|---|---|---|
| `github.com/jackc/pgx` | `Postgres (pgx)` | `database` |
| `github.com/go-sql-driver/mysql` | `MySQL/TiDB` | `database` |
| `github.com/nats-io/nats.go`, `.../common/eventbus` | `NATS` | `queue` |
| `golang.org/x/crypto/ssh`, `github.com/pkg/sftp` | `SSH/SFTP` | `service` (host đích; nhãn "dev server/host SSH") |
| `github.com/coder/websocket` | `WebSocket` | `service` (agent trên dev server khi package `devserveragent`/`agentwsserver`) |
| `github.com/prometheus/client_golang` | `Prometheus` | `external-api` |
| `.../common/secrets`, `github.com/hashicorp/vault` | `Vault` | `vault` (chưa kiểm chứng import ở adapter; `common/go.mod` có `hashicorp/vault/api`) |
| `google.golang.org/grpc` + client của service khác | `gRPC` | `service` (tên service đích từ CR-CV-032) |

`symbolCount`: số khai báo hàm/phương thức/kiểu không-test của package (đếm bằng `go/parser`, không lấy từ GitNexus để không phụ thuộc chỉ mục). `description`: theo thứ tự: `c4.yaml` → câu đầu của package doc (`descriptionSource:"package-doc"`) → rỗng. Không tóm tắt bằng LLM ở CR này (08 §3 nói tuỳ chọn; để ngoài).

`id` ổn định: `<kind-prefix>-<đường dẫn con>` chữ thường (ví dụ `adapter-devserveragent`), không phụ thuộc thứ tự quét; đổi tên thư mục là đổi `id` (ghi đè theo `id` cũ sẽ báo `C4_UNKNOWN_ID`, xem 2.5).

### 2.3 Quan hệ (`C4Relation`)

Mọi quan hệ mang `evidence` (danh sách `SymbolRef` hoặc `file:line`), `count`, `origin` (`derived|declared`), `confidence`.

| `kind` | Nguồn | Cách dựng |
|---|---|---|
| `uses` | import giữa các package trong `internal/` | tập import mỗi file (`go/parser` với `ImportsOnly`) gộp lên cặp component; bỏ `cmd/server`. Nếu chi phí đọc quá lớn, dùng `IMPORTS` của GitNexus (CR-CV-002) cho cạnh mức file (Q2) |
| `implements` | adapter → cổng ở `usecase` | (1) dòng `var _ usecase.X = (*T)(nil)` (confidence 1,0); (2) trùng tập tên phương thức của interface `X` với phương thức của kiểu `T` trong adapter, tất cả tên phải có (confidence 0,6, nhãn "suy luận"); (3) tham số khởi tạo trong `cmd/server/main.go` (ví dụ `usecase.NewRelayByDevServer(repo, agentClient)`) chỉ dùng làm bằng chứng phụ |
| `calls-rpc` | CR-CV-032 | từ component chứa `From` của `RpcEdge` tới `C4External{kind:"service"}` mang tên service đích; `evidence` = cạnh RPC; kênh agent-method sinh quan hệ tới external `agent` |
| `reads`/`writes` | CR-CV-031 `accessedBy` | component chứa `symbol` → external `database` (loại theo adapter `postgres`/`mysql`); `evidence` là tên bảng và `SymbolRef` |
| `publishes`/`subscribes` | import `common/eventbus`, `common/outbox` | chiều theo tên hàm dùng (`Publish`, `Subscribe`, `outbox.NewRelay`); tên topic để trống (E10, CR-CV-035 bổ sung) |

**Luật phụ thuộc lớp** (tính sẵn để CR-CV-037 dùng; tính ở đây vì đã có cạnh `uses`): hợp lệ `adapter→usecase`, `adapter→domain`, `usecase→domain`, `grpc-server→usecase`; vi phạm `domain→{usecase,adapter}`, `usecase→adapter`, `adapter→adapter` khác nhóm khi không qua cổng. Cạnh vi phạm có `violatesLayering:true` và vẫn được vẽ (nét đỏ ở UI). Danh sách ngoại lệ do CR-CV-037 giữ.

### 2.4 Thành phần ngoài (`C4External`)

- Suy từ bảng 2.2 (`database`, `queue`, `vault`, `external-api`) và từ `calls-rpc` (`service`). Mỗi external có `id` ổn định (`ext-postgres`, `ext-nats`, `ext-svc-<service>`, `ext-agent`).
- Hệ thống ngoài Orca (Jira, GitHub/GitLab, LLM; E17) chỉ hiện khi có trong `c4.yaml` hoặc khi có adapter rõ ràng (ví dụ `scm-integration-service`); không đoán từ tên.
- Số lượng bị cắt ở 40 external mỗi view (cấu hình được).

### 2.5 Định dạng `c4.yaml` (ghi đè)

Lưu ở bảng `c4_overrides(tenant_id, repo_binding_id, container, document, version, updated_by, updated_at)` (CR-CV-011). `document` là văn bản YAML. Nguồn tuỳ chọn thứ hai: tệp trong repo `docs/code-intel/c4/<container>.yaml` (đọc qua CR-CV-030) dùng làm bản seed khi DB chưa có bản ghi; **DB thắng** khi cả hai có (Q3). Lược đồ phiên bản 1:

```yaml
version: 1                      # bắt buộc; chỉ chấp nhận 1
container: infra-fleet-service  # phải trùng container của bản ghi
description: "Quản lý dev server, kết nối, phiên terminal/agent; chuyển lệnh tới agent."   # ≤ 500 ký tự

components:                     # ghi đè/bổ sung theo id; tối đa 200
  - id: adapter-devserveragent  # id mặc định (suy ra) hoặc id mới
    name: "Dev Server Agent client"
    description: "Gọi JSON-RPC tới agent qua WebSocket (chế độ direct/relay-websocket)."
    kind: adapter               # domain|usecase|adapter|grpc-server|grpc-client|config|other
    tech: "WebSocket JSON-RPC 2.0"
  - id: persistence             # component gộp
    name: "Persistence (Postgres/MySQL)"
    merge: [adapter-postgres, adapter-mysql]   # các id mặc định bị thay bằng component này
    kind: adapter
    tech: "pgx / database-sql"
  - id: adapter-sshrelay
    paths: ["internal/adapter/sshrelay", "internal/adapter/sshconn"]  # gộp theo thư mục (tương đối gốc service)
    name: "SSH relay"

hide: [main, adapter-portalloc, adapter-metrics]   # ẩn component (và quan hệ chạm tới nó)

externals:
  - id: ext-vault
    name: "Vault (SSH secrets engine)"
    kind: vault                 # service|database|queue|vault|external-api
    description: "Cấp chứng chỉ SSH ngắn hạn cho relay-ssh."

relations:
  add:
    - from: adapter-sshconn
      to: ext-vault
      kind: calls-rpc           # uses|implements|calls-rpc|reads|writes|publishes|subscribes
      label: "xin chứng chỉ SSH"
  remove:
    - from: usecase
      to: adapter-metrics
      kind: uses
```

Quy tắc hợp nhất (thực hiện ở `merge_c4_overrides.go`, thuần hàm, kiểm thử bảng):

1. Bắt đầu từ view suy luận (mọi phần tử `origin:"derived"`).
2. `components[]`: khớp theo `id`; trường có mặt ghi đè trường suy luận (`name`, `description`, `kind`, `tech`); phần tử có `merge` thay mọi id trong danh sách bằng một component mới (hợp `symbolCount`, hợp quan hệ, loại cạnh nội bộ); `paths` tạo component mới chứa các package đó (id mới, loại khỏi component mặc định); id không tồn tại và không có `merge`/`paths` → component rỗng `origin:"declared"` + cảnh báo `C4_UNKNOWN_ID`.
3. `hide[]`: loại component và các quan hệ chạm tới nó; id lạ → cảnh báo.
4. `externals[]`: thêm/ghi đè theo `id`.
5. `relations.add/remove`: khớp theo `(from,to,kind)`; điểm đầu/cuối phải là id sau bước 2–4, nếu không → cảnh báo bỏ qua.
6. Phần tử có chỉnh sửa mang `origin:"merged"` (có phần suy luận) hoặc `"declared"` (hoàn toàn khai báo); phần tử chưa chỉnh giữ `"derived"`. **UI luôn hiện nhãn "suy luận" cho `derived`**; mô tả từ package doc có nhãn riêng `descriptionSource`.
7. Lỗi không làm hỏng view: cảnh báo trả trong `warnings[]`; chỉ lỗi cấu trúc (không phải YAML, `version` khác 1, `container` sai) mới làm `SaveC4Overrides` từ chối.

Xác thực khi lưu: giải mã bằng `gopkg.in/yaml.v3` (đã có trong `go.mod` của `infra-fleet-service`; chưa kiểm tra các module khác) với `KnownFields(true)`; **cấm alias/anchor/tag tuỳ chỉnh** (duyệt `yaml.Node`, từ chối `AliasNode`); giới hạn: ≤ 64 KiB, ≤ 200 component, ≤ 40 external, ≤ 400 quan hệ, độ dài chuỗi ≤ 500 (mô tả) / 120 (tên), `id` khớp `^[a-z0-9][a-z0-9-]{0,63}$`; chuỗi được hiển thị như văn bản thuần (không HTML/Markdown). Ghi cần quyền (CR-CV-013), lưu kèm `version` kiểu khoá lạc quan (xung đột → `FailedPrecondition`), phát sự kiện audit; không có secret trong tài liệu (kiểm tra mẫu `password|token|secret|dsn=` cảnh báo, không chặn).

### 2.6 Proto (mới, `c4.proto`)

Theo 08 §3, với tiền tố `C4` để không va chạm tên khác trong cùng package (xem "Điều chỉnh hợp đồng"):

```proto
message GetArchitectureRequest { string repo_binding_id = 1; string container = 2; string ref = 3; bool include_hidden = 4; }
message GetArchitectureResponse { C4ComponentView view = 1; CodeIntelResultMeta meta = 2; }

message C4ComponentView {
  ContainerRef container = 1; repeated C4Component components = 2;
  repeated C4Relation relations = 3; repeated C4External externals = 4;
  repeated C4Warning warnings = 5; string overrides_version = 6; bool has_overrides = 7;
}
message ContainerRef { string id = 1; string name = 2; string path = 3; string kind = 4; /* service */ }
message C4Component {
  string id = 1; string name = 2; string kind = 3; string path = 4;
  string description = 5; string description_source = 6; /* c4.yaml | package-doc | none */
  int32 symbol_count = 7; string tech_hint = 8; string origin = 9; /* derived|merged|declared */
  repeated string package_paths = 10; bool hidden = 11;
}
message C4Relation {
  string from = 1; string to = 2; string kind = 3; repeated SymbolRef evidence = 4;
  int32 count = 5; string origin = 6; double confidence = 7; bool violates_layering = 8; string label = 9;
}
message C4External { string id = 1; string name = 2; string kind = 3; string description = 4; string origin = 5; }
message C4Warning { string code = 1; string message = 2; }

message GetC4OverridesRequest { string repo_binding_id = 1; string container = 2; }
message GetC4OverridesResponse { string document = 1; string version = 2; string updated_by = 3; string updated_at = 4; string seed_source = 5; }
message SaveC4OverridesRequest { string repo_binding_id = 1; string container = 2; string document = 3; string expected_version = 4; }
message SaveC4OverridesResponse { string version = 1; repeated C4Warning warnings = 2; }
```

`SymbolRef`, `CodeIntelResultMeta` do CR-CV-020. RPC `GetArchitecture`, `GetC4Overrides`, `SaveC4Overrides` đã có tên trong README mục 3.6; message do CR này sở hữu. Lưu ý: tên `GetArchitecture` còn gần nghĩa với `ArchitectureGraph` (cụm, 05 §2.1); CR này giả định `GetArchitecture` trả `C4ComponentView`, còn đồ thị cụm thuộc `GetStructure`/`codeintel.overview` (cần xác nhận).

### 2.7 Ví dụ `infra-fleet-service` (kết quả kỳ vọng, chưa chạy)

Container `infra-fleet-service` (207 file Go). Component mặc định kỳ vọng: `domain`, `usecase`, `config`, và 15 `adapter-*` (kể cả `adapter-grpc` kind `grpc-server`, `adapter-grpcclient` kind `grpc-client`), `main` ẩn. External suy ra: `ext-postgres`, `ext-mysql`, `ext-nats`, SSH/WebSocket tới dev server, `ext-svc-ai-provider-service`, `ext-svc-credential-broker-service` (từ `adapter/grpcclient`, đã đối chiếu: `infra-fleet-service → ai-provider-service, credential-broker-service` trong thăm dò CR-CV-032), `ext-agent`. Quan hệ kỳ vọng: `adapter-postgres/mysql --implements--> usecase` (hàng chục cổng); `adapter-grpc --uses--> usecase`; `adapter-devserveragent --calls-rpc--> ext-agent`. Với `c4.yaml` ở mục 2.5 thì `adapter-postgres` và `adapter-mysql` gộp thành `persistence`, `main`, `portalloc`, `metrics` ẩn, thêm `ext-vault`.

### 2.8 Chi phí và cache

Đọc ~207 file Go (1,05 MiB) cho `infra-fleet-service` qua CR-CV-030 trong lần đầu (cache theo blob oid sau đó); `go/parser` với `ImportsOnly` cho `uses`, parse đầy đủ chỉ file cần (có `var _`, interface ở `usecase`, `main.go`). Kết quả view cache ở `graph_snapshots(view="architecture", params_hash)` khoá thêm `overrides_version` (đổi `c4.yaml` thì không dùng lại view cũ). Ngân sách: ≤ 200 component, ≤ 2 000 quan hệ một view; vượt → `truncated`.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Component = package thư mục lá, không tách theo file | Đơn giản, ổn định, khớp 08 §3 |
| `implements` ưu tiên `var _ …`, bổ sung bằng khớp tên phương thức | Bằng chứng mạnh nhưng phân bố không đều (29 so với 3); GitNexus không đủ cho Go |
| Không vẽ `cmd/server` mặc định | Nối mọi thứ, vô nghĩa kiến trúc |
| Mô tả lấy từ package doc, gắn nhãn nguồn | Rẻ, nhưng là suy luận; có thể bị `c4.yaml` ghi đè |
| Lưu ghi đè ở DB, repo chỉ làm seed tuỳ chọn | README v7 đã có `c4_overrides`; lens C4 (CR-CV-055) cần sửa trực tiếp; vẫn xuất được ra tệp |
| Cảnh báo thay vì lỗi khi id không còn khớp | Code thay đổi thì `id` có thể biến mất; không để view hỏng |
| Cấm alias/anchor YAML, giới hạn kích thước | Chống lạm dụng tài nguyên với dữ liệu nhập từ người dùng |
| Chỉ container Go ở CR này | Quy tắc mới chỉ khảo sát được cho hexagonal Go |
| Tiền tố `C4` cho message | Tránh va chạm với tên chung (`Component`, `Relation`) trong package `orca.codeintel.v1` |

## 4. Tiêu chí chấp nhận

- [ ] `GetArchitecture(infra-fleet-service)` trả đủ component mặc định ở 2.2/2.7 (domain, usecase, config, mọi `adapter-*`), `main` ẩn, mỗi component có `symbolCount` > 0 và `description` từ package doc khi có.
- [ ] Quan hệ `implements` từ `adapter-mysql` tới `usecase` có `evidence` đủ các dòng `var _` ở `shared.go` (≥ 29); các cổng chỉ khớp theo tên có `confidence` 0,6 và nhãn "suy luận".
- [ ] `calls-rpc` tới `ext-svc-ai-provider-service` và `ext-svc-credential-broker-service` xuất hiện từ `adapter-grpcclient`; `ext-nats`, `ext-postgres`, `ext-mysql` suy ra từ import.
- [ ] Vi phạm lớp được đánh dấu `violatesLayering` khi có import `usecase→adapter` hoặc `domain→usecase` (kiểm thử trên cây thư mục mẫu có vi phạm).
- [ ] `SaveC4Overrides` từ chối: YAML có alias, `version` ≠ 1, vượt giới hạn, `container` sai; chấp nhận mẫu 2.5; xung đột `expected_version` trả `FailedPrecondition`; chỉ vai trò đủ quyền ghi được.
- [ ] Hợp nhất đúng: ghi đè tên/mô tả, `merge`, `paths`, `hide`, `externals`, `relations.add/remove`; id lạ sinh `C4_UNKNOWN_ID` mà view vẫn dựng.
- [ ] Mọi phần tử có `origin`; UI có đủ dữ liệu để hiện nhãn "suy luận".
- [ ] Đổi `c4.yaml` làm mất cache view cũ (`overrides_version` trong khoá).
- [ ] Không có nội dung file nguồn trong kết quả (chỉ `SymbolRef`/vị trí); không secret trong log.
- [ ] Không dùng tên `helpers`/`utils`/`common`/`misc`; không thêm `max-lines` disable.

## 5. Kiểm thử

- **Unit:** phân loại `kind`; sinh `id`; quy tắc `techHint`; trích câu đầu package doc; khớp tập phương thức interface/kiểu (thiếu một phương thức thì không khớp); luật phụ thuộc lớp; hợp nhất `c4.yaml` (bảng ca: `merge`, `paths`, `hide`, id lạ, `relations.remove`); xác thực YAML (alias, giới hạn, trường lạ).
- **Fixture vàng (CR-CV-070):** snapshot view của `infra-fleet-service` và một service nhỏ (`usage-service`) từ commit cố định; kèm `c4.yaml` mẫu.
- **Hợp đồng:** cạnh `calls-rpc`/`reads` khớp CR-CV-032/031 trên cùng commit.
- **Bảo mật:** `SaveC4Overrides` cần quyền; cô lập tenant (CR-CV-072).
- Chưa chạy bất kỳ test nào ở thời điểm viết.

## 6. Rủi ro và điểm chưa kiểm chứng

- Gom theo thư mục có thể quá thô (`usecase` 96 file là một khối) hoặc quá mịn (15 adapter); cần phản hồi người dùng, `c4.yaml` là van giảm áp.
- Khớp tên phương thức cho `implements` có thể trùng nhầm (nhiều interface nhỏ có tên phương thức chung); thấp độ tin cậy được gắn nhãn.
- Package doc có thể mô tả chi tiết cài đặt hơn là vai trò, hoặc lỗi thời; chưa đánh giá chất lượng trên toàn 15 adapter.
- Import Vault ở adapter chưa kiểm chứng (chỉ thấy trong doc `sshconn`, comment migration và `common/go.mod`).
- Chi phí đọc ~1,05 MiB/service qua RPC chưa đo; service lớn hơn (`project-service`, `auth-service`) chưa khảo sát.
- Quy tắc cho `api-gateway`/`git-gateway-service` (không có adapter DB) chưa thử; `api-gateway` có nhiều adapter lá (`mcp*`).
- `yaml.v3` chỉ xác nhận trong `infra-fleet-service/go.mod`.

## 7. Câu hỏi mở

- **Q1.** Container `agent`, `frontend`, `desktop` xử lý thế nào ở các CR sau (quy tắc TypeScript)? Hiện chỉ làm Go.
- **Q2.** Cạnh `uses` lấy từ `go/parser` (đọc nhiều file) hay từ `IMPORTS` của GitNexus (nhanh nhưng phụ thuộc CR-CV-002 và chỉ mục tươi)?
- **Q3.** `c4.yaml` seed trong repo có cần không, và đường dẫn chuẩn? (E6 đề xuất đặt trong repo).
- **Q4.** Ai duyệt `c4.yaml` (E6 nói "cần bạn cung cấp / duyệt")? Có cần quy trình duyệt hay chỉ cần quyền ghi?
- **Q5.** `GetArchitecture` có thật là C4 hay là đồ thị cụm (05 §2.1)?
- **Q6.** Có tạo nút "tóm tắt mô tả bằng LLM" (08 §3 tuỳ chọn) không; nếu có là CR riêng.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (mục 3.4, 3.5, 3.6); `/opt/repos/orca/docs/research/view-code/08-views-and-review-models.md` (§3, §8), `09-external-inputs-required.md` (E6, E15, E17)
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/usecase/ports.go`, `.../internal/adapter/mysql/shared.go`, `.../internal/adapter/postgres/*.go`, `.../internal/adapter/{sshconn,agentwsserver,eventbus,devserveragent}/*.go` (package doc)
- `/opt/repos/orca/backend-go/services/infra-fleet-service/cmd/server/main.go` (wiring), `/opt/repos/orca/backend-go/services/infra-fleet-service/go.mod`
- `/opt/repos/orca/backend-go/services/api-gateway/internal/adapter/` (danh sách adapter khác)
- GitNexus `cypher` đã chạy trên repo `orca` ngày 2026-10-05: `IMPLEMENTS` ở `infra-fleet-service/internal/adapter` (3 cạnh)
- CR liên quan: CR-CV-002, 011, 013, 020, 030, 031, 032, 034, 035, 037, 055, 070, 072 (xem `../README.md`)

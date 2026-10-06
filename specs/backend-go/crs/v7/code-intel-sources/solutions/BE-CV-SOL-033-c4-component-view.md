# BE-CV-SOL-033-c4-component-view: `C4ComponentView` suy từ cấu trúc hexagonal và RPC `GetArchitecture`

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. Phần 1/2 của CR-CV-033; phần ghi đè `c4.yaml` ở [`BE-CV-SOL-033-c4-overrides-yaml`](./BE-CV-SOL-033-c4-overrides-yaml.md) (dãy task `NN` liên tục: 01–06 ở đây, 07–11 ở solution kia).

**CR:** [CR-CV-033](../../../../../../docs/crs/v7/code-intel-sources/CR-CV-033-c4-component-view.md)
**Service:** `code-intel-service` — `proto/orca/codeintel/v1/codeintel_c4.proto` (mới) + `rpc GetArchitecture`, `internal/domain/c4`, `internal/usecase/derive_c4_view.go`, `internal/adapter/gopackagegraph`, handler gRPC
**TDD tham chiếu:** [`arch/02`](../../../../tdd/architecture/02-microservices-decomposition.md), [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) ("The dependency rule", "Standard package layout": chính là quy tắc suy component), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md)

---

## 0. Hợp đồng áp dụng

| Mã | Áp dụng | Mục |
|---|---|---|
| PQ-10 | `GetArchitecture` trả `C4ComponentView` + `containers[]` (kênh `codeIntel.architecture`); cụm GitNexus ở `GetClusterOverview` riêng | §1 |
| PQ-04, PQ-07, PQ-12, PQ-14, PQ-15 | `selector`; file `codeintel_c4.proto`; `ResultMeta`/etag; ≤ 2 MiB; view `architecture` (khoá thêm `overrides_version`) | §1, §4 T3 |
| PQ-29 | `ContainerRef` do file c4 sở hữu; `C4Warning` riêng; tiền tố `C4*` | §1 |
| §3.1, UI §4.4 | Request `container, include_hidden, if_none_match`; response `{meta, containers[], view}`; hình dạng JSON `C4*` | `CONTRACT-ui-api` §4.4 |
| PQ-03/02 | Lỗi `CODEINTEL_*` | §1 |
| §8.3-3/4 | Cache snapshot cô lập tenant; không nhánh dialect riêng ở view (adapter DB chỉ làm external) | §8.3 |

## 1. Trạng thái hiện tại (re-verify)

Đã đọc/chạy chỉ-đọc (2026-10-06): `ls backend-go/services/infra-fleet-service/internal/adapter/` (15 thư mục: `agentwsserver … webhook`); `grep -c "^\s*_ usecase\." …/adapter/mysql/shared.go` = **23**; `grep -rn "_ usecase\.\|var _ usecase\." …/adapter/postgres | wc -l` = **3**; `grep -c "^type .* interface" …/usecase/ports.go` = **41**; `infra-fleet-service/go.mod:17` `gopkg.in/yaml.v3`; `ls docs/code-intel` không tồn tại (không có seed `c4`); không có `c4*.y*ml`.

### Correction relative to CR-CV-033

| # | CR nói | Đọc được | Xử lý |
|---|---|---|---|
| C1 | `adapter/mysql` có 29 dòng `var _ usecase.X` | `grep` ra **23** dòng bắt đầu `_ usecase.` trong `shared.go` (khối `var (...)`); postgres 3 | Tiêu chí dùng "≥ số do chính bộ trích đếm, đối chiếu `grep` độc lập", bỏ hằng 29 |
| C2 | `GetArchitectureRequest{repo_binding_id=1, container=2, ref=3, include_hidden=4}`, response `view=1, meta=2` | PQ-04/§3.1: `selector`, response thêm `containers[]` | Request `selector=1, container=2, include_hidden=3, if_none_match=4`; response `meta=1, containers=2, view=3` (số đề xuất, CR-033 gán) |
| C3 | `ContainerRef.kind` chỉ `service` | UI §4.4: `service|agent|frontend|desktop|other` | Chỉ phát `service` ở MVP; enum chuỗi mở |
| C4 | `C4Component` thiếu `hidden` ở view mặc định | UI có `hidden: boolean`, `include_hidden` | `main` mặc định `hidden=true` và bị lọc trừ khi `include_hidden` |
| C5 | `overrides_version` kiểu string | `c4_overrides.version bigint` (T6) | Proto `string` (số thập phân) để UI nhận chuỗi; xem G1 |
| C6 | "`Process`... " không liên quan | — | — |

## 2. Giải pháp

### A. Proto (`codeintel_c4.proto`, mới) và RPC
`GetArchitectureRequest/Response`; `ContainerRef{id=1,name=2,path=3,kind=4}`, `C4ComponentView{container=1,components=2,relations=3,externals=4,warnings=5,overrides_version=6,has_overrides=7}`, `C4Component`(1–11), `C4Relation`(1–9), `C4External`(1–5), `C4Warning`(1–2): số **theo CR §2.6 (chuẩn)**. `GetC4Overrides*`/`SaveC4Overrides*` thuộc solution `c4-overrides-yaml` nhưng khai báo trong cùng file `codeintel_c4.proto` (PQ §2.1 hàng 11) — **task 02 tạo file, task 08 của overrides thêm message** (tránh hai PR sửa cùng file: thứ tự ở tasks/README).

### B. Cây file (mới)
```
internal/domain/c4/{component.go,relation.go,layer_rules.go,stable_ids.go}
internal/adapter/gopackagegraph/{package_scan.go,import_graph.go,port_implementations.go,package_doc.go}
internal/usecase/derive_c4_view.go          # suy luận thuần, chưa có override
internal/usecase/get_architecture.go        # ghép: catalog(032) + erd access(031) + overrides(033-yaml) + cache
internal/adapter/grpc/get_architecture_handler.go
```

### C. Quy tắc suy luận (CR 2.2–2.4; theo cấu trúc, không theo tên)
- Container = `backend-go/services/<svc>`; chỉ Go (Q1 CR: TS ngoài phạm vi).
- Component: `internal/domain`→`domain`, `internal/usecase`→`usecase`, mỗi thư mục lá `internal/adapter/<n>` (lồng → `adapter-<a>-<b>`), `internal/config`, `internal/<khác>`→`internal-<n>` kind `other`, `cmd/server`→`main` (ẩn). `id` = `<kind-prefix>-<đường dẫn con>` chữ thường, độc lập thứ tự quét. `kind` adapter: `grpc-server` (có `Register<Svc>Server`/nhúng `Unimplemented<Svc>Server`, dùng kết quả 032) > `grpc-client` (tham chiếu `<pkg>v1.<Svc>Client`) > `adapter`.
- `techHint`/external từ import (bảng cấu hình: `pgx`→Postgres, `go-sql-driver/mysql`→MySQL/TiDB, `nats-io/nats.go` hoặc `common/eventbus`/`common/outbox`→NATS, `x/crypto/ssh`/`pkg/sftp`→SSH/SFTP, `coder/websocket`→WebSocket, `prometheus/client_golang`→Prometheus, `common/secrets`/`hashicorp/vault`→Vault (**chưa kiểm chứng import ở adapter**), `google.golang.org/grpc` + client service khác→`ext-svc-<service>`); `ext-agent` khi có `devserveragent`/`agentwsserver`/kênh `agent-method`.
- Mô tả: `package doc` (câu đầu, `descriptionSource:"package-doc"`) hoặc rỗng; không LLM (Q6).
- Quan hệ: `uses` (imports nội bộ; Q2: `go/parser` `ImportsOnly`; GitNexus `IMPORTS` chỉ phương án sau), `implements` (ưu tiên `var _/_ usecase.X = (*T)(nil)` conf 1,0; khớp **toàn bộ** tập phương thức interface conf 0,6 nhãn suy luận; `main.go` chỉ làm bằng chứng phụ), `calls-rpc` (từ `RpcEdge` của 032), `reads/writes` (từ `accessedBy` của `BE-CV-SOL-031-erd-model-and-access-scan`; `readwrite` → hai cạnh hoặc một cạnh `reads` + `writes`: **do chủ CR-033 chọn**, đề xuất hai cạnh), `publishes/subscribes` (hàm `Publish`, `Subscribe`, `outbox.NewRelay`; topic để trống — thuộc 035). Mọi quan hệ có `evidence`, `count`, `origin`, `confidence`.
- Luật lớp tính sẵn: hợp lệ `adapter→usecase|domain`, `usecase→domain`, `grpc-server→usecase`; vi phạm `domain→{usecase,adapter}`, `usecase→adapter`, `adapter→adapter` khác nhóm không qua cổng → `violatesLayering:true` (danh sách ngoại lệ do `BE-CV-SOL-037` giữ).
- Ngân sách ≤ 200 component, ≤ 2 000 quan hệ, ≤ 40 external → `truncated`.

### D. `GetArchitecture` use case/handler
`RequireTenantID` → selector→binding → OPA `read` → `containers[]` (liệt kê `backend-go/services/*` có `internal/`) → `view` cho `container` (rỗng → `view=null`, chỉ trả `containers[]`) → hợp nhất override (solution kia, `MergeC4Overrides`) → ẩn `hidden` trừ `include_hidden` → cache `graph_snapshots(view="architecture")` với `params_hash` gồm `overrides_version` và tập file bẩn → `ResultMeta`. Kích thước ≤ 2 MiB. Timeout/`inProgress` như PQ-13.

## 3. Quyết định thiết kế
| Quyết định | Lý do |
|---|---|
| Component = thư mục lá | Đơn giản, ổn định (08 §3); `c4.yaml` làm van |
| `implements` ưu tiên `var _` | Mạnh nhưng phân bố không đều (23 so với 3) |
| `main` ẩn mặc định | Nối mọi thứ, vô nghĩa |
| Chỉ Go | Quy tắc mới khảo sát cho hexagonal Go |
| Khoá cache gồm `overrides_version` | Đổi `c4.yaml` không dùng nhầm view cũ |

## 4. Lệch giữa CR và hợp đồng
C1–C5; PQ-10 giải Q5 (GetArchitecture = C4); `erd.proto`→`codeintel_c4.proto` (PQ-07).

## 5. Phụ thuộc chéo khu vực
BE: `030`, `032` (RpcEdge, `Unimplemented`, kind `grpc-server|client`), `031-erd-model` (accessedBy), `033-c4-overrides-yaml` (merge, bảng `c4_overrides` ở `011`), `011`, `012`, `013`, `020`, `022`; tiêu thụ bởi `034`, `037`, `040-codeintel-view-channels` (`codeIntel.architecture`). FE: `FE-CV-SOL-055-architecture-c4-lens`. AG: không.

## 6. Tiêu chí chấp nhận
- [ ] `GetArchitecture(infra-fleet-service)`: `domain`, `usecase`, `config`, mọi `adapter-*` (15 thư mục), `main` ẩn; `symbolCount` > 0; `description` từ package doc khi có.
- [ ] `implements` từ adapter MySQL tới `usecase` có evidence = số dòng `_ usecase.` do `grep` độc lập; cổng chỉ khớp tên có conf 0,6 + nhãn.
- [ ] `calls-rpc` tới `ext-svc-ai-provider-service`, `ext-svc-credential-broker-service`; `ext-nats|postgres|mysql` từ import.
- [ ] `violatesLayering` bắt `usecase→adapter`, `domain→usecase` (cây mẫu có vi phạm).
- [ ] `has_overrides`/`overrides_version` đúng; đổi override làm miss cache.
- [ ] Không nội dung file nguồn; cô lập tenant (cache); không `helpers/utils/common/misc`; không `max-lines` disable; `buf` xanh.

## 7. Kiểm thử, rủi ro, câu hỏi mở
**Kiểm thử.** Unit phân loại kind/id/techHint/layer rules, khớp tập phương thức (thiếu một → không khớp); golden `infra-fleet-service` + `usage-service` (CR-070); hợp đồng với 031/032 trên cùng commit; cô lập tenant; lệnh `go test ./services/code-intel-service/internal/{domain/c4,adapter/gopackagegraph,usecase}/...` (chưa chạy).
**Rủi ro.** Gom thô/mịn; khớp tên trùng nhầm; package doc nói chi tiết hơn vai trò; chi phí đọc ~1,05 MiB/service qua RPC chưa đo; `api-gateway` (nhiều adapter lá `mcp*`) và `git-gateway-service` (không adapter DB) chưa thử.
**Hợp đồng thiếu (báo chủ).** G1: kiểu `version` của `c4` (`bigint` DB, chuỗi ở C4ComponentView, UI `expectedVersion`). G2: số field proto của request/response `GetArchitecture` chưa có trong hợp đồng.
**Mở.** Q1 (TS), Q2 (`uses` bằng GitNexus), Q3/Q4 xem solution overrides.

## 8. Tham chiếu
`docs/crs/v7/code-intel-sources/CR-CV-033-*.md`; hợp đồng PQ-04/07/10/12/14/15/29, §3.1, UI §4.4; `backend-go/services/infra-fleet-service/internal/{usecase/ports.go,adapter/mysql/shared.go,adapter/postgres/*.go}`.

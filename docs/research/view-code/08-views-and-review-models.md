# 08 — Các view hiển thị và mô hình review nhanh

Mục tiêu: từ thông tin code, hiển thị (1) cấu trúc code, (2) kiến trúc hệ thống mức thiết kế chi tiết (C4 level 3), (3) luồng dữ liệu/workflow, (4) ERD, (5) kiến trúc lưu trữ; và thêm các view giúp review code nhanh nhất.

Trạng thái: đề xuất (2026-10-05). Các số liệu repo dưới đây đã kiểm tra: 17 service Go có `migrations/` (588 file `.sql`); service theo bố cục hexagonal `domain/usecase/adapter`; `infra-fleet-service` có adapter `postgres`, `mysql`, `eventbus`. Những nguồn dữ liệu ngoài GitNexus/CodeGraph xem [09](./09-external-inputs-required.md).

## 1. Tổng quan mức tự động

| # | View | Nguồn chính | Mức tự động | Việc cần làm thêm |
|---|---|---|---|---|
| 1 | Cấu trúc code | GitNexus `Folder/File/CONTAINS/DEFINES`; CodeGraph `files` | Cao | Treemap/sunburst theo `symbolCount`, ngôn ngữ, khu vực |
| 2 | Kiến trúc C4 level 3 | Cấu trúc thư mục + `IMPLEMENTS` + lời gọi gRPC giữa service | Trung bình | Lớp quy tắc ánh xạ (§3) |
| 3 | Luồng dữ liệu/workflow | GitNexus `Process` (300 luồng) + proto + kênh `wscompat` | Trung bình | Nối luồng qua ranh giới service (§4) |
| 4 | ERD | `migrations/*.sql` | Cao nếu parse SQL; **không** lấy từ GitNexus | Parse migration theo thứ tự (§5) |
| 5 | Kiến trúc lưu trữ | compose, config từng service, migration, adapter | Thấp–trung bình | Phần lớn là suy luận từ cấu hình (§6) |

Hệ quả cho kiến trúc ([07](./07-architecture-decisions.md)): các view 2, 4, 5 chỉ cần đọc file nhỏ (SQL, proto, compose, cấu trúc thư mục). Có thể dùng RPC `fs.*`/`git.*` đã có của agent và parse ở `code-intel-service`, không cần mở rộng agent. Chỉ view 1 và 3 cần `codeintel.*` (graph).

## 2. View 1 — Cấu trúc code

- Dữ liệu: cây `Folder → File → Symbol` (GitNexus `CONTAINS`/`DEFINES`; CodeGraph `contains`).
- Hiển thị: treemap/sunburst; kích thước = số symbol (hoặc LOC); màu = ngôn ngữ hoặc khu vực (`frontend`, `backend-go`, `agent`, `desktop`, `mobile`).
- Drill-down: thư mục → file → symbol → mã nguồn (`codeintel.symbol`).
- Schema: `ModuleGraph`, `SymbolGraph` ([05 §2.2–2.3](./05-graph-schemas.md)).

## 3. View 2 — C4 level 3 (Component)

Công cụ không có khái niệm "component" của C4; cụm `Community` của GitNexus chỉ là gom theo mật độ gọi, không phải ranh giới kiến trúc. Cách khả thi:

- **Container** = một service (`backend-go/services/*`, `agent`, `frontend`, `desktop`).
- **Component** = package trong `internal/` theo quy ước hexagonal: `usecase`, `domain`, `adapter/*` (ví dụ ở `infra-fleet-service`: `agentwsserver`, `devserveragent`, `postgres`, `mysql`, `eventbus`, `grpc`, `grpcclient`, `sshrelay`…).
- **Quan hệ** lấy từ cạnh `implements`/`calls`:
  - port (interface trong `usecase`) ↔ adapter cài đặt nó;
  - `grpcclient` → service khác qua proto;
  - `adapter/grpc` (server) → `usecase`.
- Mỗi diagram vẽ **cho một container**, không vẽ cả hệ thống.
- Lớp quy tắc: file `c4.yaml` (hoặc cấu hình trong `code-intel-service`) cho phép chỉnh tên, gộp component, đặt mô tả, loại bỏ package nhiễu. Quy tắc mặc định suy từ thư mục; người dùng ghi đè khi cần.
- Schema đề xuất:
```
C4ComponentView { container: ContainerRef, components: Component[], relations: ComponentRelation[], externals: ExternalRef[] }
Component { id, name, kind:"usecase|domain|adapter|grpc-server|grpc-client|config|other", path, description?, symbolCount, techHint? }
ComponentRelation { from, to, kind:"uses|implements|calls-rpc|reads|writes|publishes|subscribes", evidence: SymbolRef[] , count }
ExternalRef { id, name, kind:"service|database|queue|vault|external-api" }
```
- Hạn chế: không tự suy ra được mục đích (mô tả) của component; cần chú thích tay hoặc tóm tắt bằng LLM (tuỳ chọn, ghi rõ nguồn).

## 4. View 3 — Luồng dữ liệu / workflow

- GitNexus `Process` là chuỗi gọi hàm từ entry đến terminal; thường dừng ở stub gRPC sinh tự động. Phải **nối** client method với server handler theo tên service + method trong file proto.
- Nối thêm các lớp: kênh `wscompat` (`channel` → handler → gRPC client), workflow do `workflow-service` định nghĩa (nếu lưu trong DB/seed thì thuộc nguồn ngoài, xem 09).
- Kết quả hiển thị:
  - **Sequence diagram** cho một luồng (có thể xuất Mermaid);
  - **DFD** (UI → gateway → service → store) cho một tính năng;
  - đánh dấu mỗi bước theo component C4 để liên kết với view 2.
- Schema đề xuất:
```
DataFlow { id, label, trigger:{kind:"ws-channel|http|grpc|event|cron", name}, steps: FlowStep[], stores: StoreAccess[] }
FlowStep { n, from: ComponentRef, to: ComponentRef, kind:"call|rpc|event|db-read|db-write|ws-push", method?, symbol?: SymbolRef, sync: bool }
StoreAccess { step, store: StoreRef, table?, op:"read|write" }
```

## 5. View 4 — ERD

- Parse các file `*.up.sql` theo thứ tự số (`0001_init.up.sql`, …), áp dụng `CREATE/ALTER/DROP` để ra trạng thái cuối: bảng, cột (kiểu, null, default), PK, FK, unique, index, `CHECK`, RLS policy. Không đọc DB thật.
- Mỗi service sở hữu một schema riêng và không FK chéo service (ví dụ `infra.dev_servers` do `infra-fleet-service` sở hữu độc quyền) → vẽ **ERD theo service**; liên kết giữa service là **nét đứt logic** (qua `tenant_id` hoặc id tham chiếu), suy từ tên cột/chú thích hoặc khai báo tay.
- Có cả `mysql` và `postgres` ở `infra-fleet-service` (`migrations/mysql`, `migrations/postgres`): parser phải hỗ trợ cả hai dialect.
- Liên kết ngược về code: "bảng này do hàm repository nào đọc/ghi" bằng cách quét tên bảng (`schema.table`) trong truy vấn của adapter; lưu thành cạnh `reads/writes`.
- Schema đề xuất:
```
ErdModel { service, dialect:"postgres|mysql", schema, tables: Table[], relations: Relation[], asOfMigration }
Table { name, schema, columns: Column[], pk:[], indexes: Index[], rls?: Policy[], comment?, accessedBy: SymbolRef[] }
Column { name, type, nullable, default?, isPk, isFk, comment? }
Relation { from:{table,cols}, to:{table,cols}, kind:"fk|logical", cardinality?, cross_service?: bool }
```

## 6. View 5 — Kiến trúc lưu trữ

- Ghép từ: compose (`backend-go/docker-compose.yml`, `deploy/dev`, `deploy/prod`), cấu hình mỗi service (DSN, topic, vault path), thư mục `adapter/{postgres,mysql,eventbus}`, migration.
- Hiển thị: sơ đồ "service → kho dữ liệu (loại, schema) → event bus (topic) → secret (Vault) → volume/object store".
- Đã xác nhận: Postgres (schema theo service, có RLS), MySQL (ở `infra-fleet-service`), event bus (`adapter/eventbus` publish health/agent status), Vault (ví dụ `vault_ssh_role`), volume `orca-data`. Các thành phần khác phải đọc từ cấu hình khi làm thật, không giả định.
- `deploy/prod/docker-compose.yml` chỉ có một service `orca-server`; cần xác định topology thật (xem 09).
- Schema đề xuất:
```
StorageMap { stores: Store[], bindings: Binding[], topics: Topic[] }
Store { id, kind:"postgres|mysql|redis|object|volume|vault|queue|other", name, env:"dev|prod", owner?: ServiceRef, schemas?: string[] }
Binding { service, store, access:"rw|ro", via:"adapter-package", configKey? }
Topic { name, publisher: ServiceRef, subscribers: ServiceRef[], payload?: string }
```

## 7. Các view review nhanh (lớp phủ lên view 1–5)

Xếp theo giá trị cho review:

| # | View | Cách làm | Nguồn |
|---|---|---|---|
| R1 | **Change overlay + blast radius** | Tô các component/luồng/bảng bị diff chạm và bị ảnh hưởng; mức rủi ro | `gitnexus detect-changes`, `impact`, diff từ `git.*` |
| R2 | **Dependency matrix + vi phạm lớp** | Ma trận phụ thuộc giữa service/package; phát hiện vòng; `usecase` import `adapter` | Cạnh `IMPORTS`; quy tắc từ C4 |
| R3 | **Hotspot** | Tần suất đổi × độ phức tạp × độ trung tâm | `git log`, số symbol, bậc nút |
| R4 | **Contract diff** | So proto, route, kênh `wscompat`, schema giữa hai commit; "migration chạm bảng nào, code nào đọc bảng đó" | proto, `Route`, migration, ERD |
| R5 | **Khoảng trống test** | Hàm đổi mà chưa có test phủ | GitNexus/CodeGraph đã tính ("no covering tests found") |
| R6 | **Thứ tự đọc** | Sắp symbol đã đổi theo phụ thuộc (callee trước, caller sau), kèm tóm tắt từng bước | `STEP_IN_PROCESS`, `CALLS` |
| R7 | **Bảo mật** | Luồng taint; truy vấn thiếu `tenant_id` | `gitnexus explain` (cần index `--pdg`), quét SQL |
| R8 | **Mã chết / owner** | Export không ai dùng; lớp phủ owner | Cạnh ngược, `CODEOWNERS` |

Schema đề xuất cho lớp phủ: `ChangeOverlay` ([05 §2.7](./05-graph-schemas.md)) mở rộng thêm `touchedTables[]`, `touchedContracts[]`, `uncoveredSymbols[]`, `violations[]`, `readingOrder[]`.

## 8. Thứ tự làm đề xuất

1. View 1 (cấu trúc) + View 4 (ERD) + R1 (change overlay): giá trị cao, ít phụ thuộc quy tắc.
2. R5, R2, R3.
3. View 3 (luồng) sau khi có nối proto client↔server.
4. View 2 (C4) bắt đầu với 2–3 service quan trọng (ví dụ `infra-fleet-service`, `git-gateway-service`, `api-gateway`), tinh chỉnh bằng `c4.yaml`.
5. View 5 (lưu trữ) khi đã có đủ cấu hình triển khai.
6. R4, R6, R7, R8.

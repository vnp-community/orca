# Code Intel Sources — Change Requests (v7)

> Nguồn dữ liệu ngoài GitNexus/CodeGraph cho view Xem code: file nhỏ của repo (SQL, proto, Go, cấu hình) đọc qua `fs.*`/`git.*` của agent và parse ở `code-intel-service`, cộng các lớp phủ review. Bối cảnh, quyết định D1–D7, mặc định O1–O8 và hợp đồng chung ở [README v7](../README.md); nghiên cứu ở [`docs/research/view-code/`](../../../research/view-code/README.md) (đặc biệt [04](../../../research/view-code/04-raw-data-and-pipeline.md), [05](../../../research/view-code/05-graph-schemas.md), [08](../../../research/view-code/08-views-and-review-models.md), [09](../../../research/view-code/09-external-inputs-required.md)).

| CR | Vấn đề | Priority | Effort | Trạng thái |
|----|--------|----------|--------|------------|
| [CR-CV-030](./CR-CV-030-repo-file-access-gateway.md) | Cổng đọc file repo qua `fs.*`/`git.*` (giới hạn, bỏ nhị phân, an toàn đường dẫn phía backend, cache theo blob) | 🔴 P0 | Medium | 📝 Đề xuất, chưa triển khai |
| [CR-CV-031](./CR-CV-031-sql-migration-to-erd.md) | Parse SQL migration (Postgres, MySQL) thành `ErdModel`, liên kết logic giữa service, cạnh bảng → code | 🔴 P0 | Large | 📝 Đề xuất, chưa triển khai |
| [CR-CV-032](./CR-CV-032-proto-and-channel-contracts.md) | Trích xuất proto, nối client↔server, danh mục kênh `wscompat` | 🟠 P1 | Large | 📝 Đề xuất, chưa triển khai |
| [CR-CV-033](./CR-CV-033-c4-component-view.md) | C4 level 3: quy tắc hexagonal + `c4.yaml` ghi đè | 🟠 P1 | Large | 📝 Đề xuất, chưa triển khai |
| [CR-CV-034](./CR-CV-034-data-flow-model.md) | Luồng dữ liệu: dựng `DataFlow` qua ranh giới service, xuất mô hình sequence/DFD | 🟠 P1 | Large | 📝 Đề xuất, chưa triển khai |
| [CR-CV-035](./CR-CV-035-storage-map.md) | Bản đồ lưu trữ từ compose, cấu hình, adapter, topic (không lộ secret) | ⚪ P2 | Large | 📝 Đề xuất, chưa triển khai |
| [CR-CV-036](./CR-CV-036-change-overlay.md) | Change overlay, thứ tự đọc, khoảng trống test, chấm rủi ro | 🔴 P0 | Large | 📝 Đề xuất, chưa triển khai |
| [CR-CV-037](./CR-CV-037-structure-analysis.md) | Phân tích cấu trúc: vi phạm lớp, vòng phụ thuộc, hotspot, mã chết, owner | 🟠 P1 | Large | 📝 Đề xuất, chưa triển khai |
| [CR-CV-038](./CR-CV-038-contract-diff-and-static-security.md) | Khác biệt hợp đồng/schema và kiểm tra bảo mật tĩnh (`tenant_id`) | ⚪ P2 | Large | 📝 Đề xuất, chưa triển khai |

(Tiêu đề, priority, effort của 035–038 theo bảng README v7 mục 4; nội dung do các CR đó tự quy định.)

## Thứ tự thực thi

```
CR-CV-030 ─▶ CR-CV-031 ─▶ CR-CV-032 ─▶ CR-CV-033 ─▶ CR-CV-034 ─▶ CR-CV-035
   │                          └──────────────▲ (033 cần 032; 034 cần 031, 032, 033)
   └▶ CR-CV-036 (cần 005, 020, 021, 030) ─▶ CR-CV-037 ─▶ CR-CV-038
```

- 030 là điều kiện của cả nhóm (mọi CR khác đọc file qua nó). 031 chạy được ngay sau 030 (đợt 2 của README v7). 032 và 033 làm cùng đợt 4; 034 sau cả ba. 035 (P2) khi đã có đủ cấu hình triển khai.
- 031 và 032 độc lập nhau về dữ liệu; thứ tự trên theo mức ưu tiên giá trị (ERD là MVP). Có thể làm song song nếu đủ người.
- Nhóm 036–038 phụ thuộc cả CR ngoài folder (005, 020, 021) nên không nằm trên chuỗi chính ở trên.

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|-----------|-------|
| S1 | Mọi đọc file đi qua một cổng `RepoSourceReader` (CR-CV-030); CR 031–038 không gọi `fs.*`/`git.*` trực tiếp | Đọc ở agent Part A không có confinement đường dẫn (chỉ `fs.writeFile` có); backend phải tự chặn. Một chỗ để kiểm thử bảo mật |
| S2 | Parse tĩnh ở `code-intel-service` bằng thư viện chuẩn khi được (`go/parser` cho Go) và bộ phân tích tự viết có chịu lỗi cho SQL/proto (đề xuất, **chưa chốt** ở 031 và 032) | Không thêm dependency; Dockerfile build `CGO_ENABLED=0`; tập cú pháp thực tế nhỏ |
| S3 | GitNexus chỉ làm tầng đối chứng/làm giàu cho view từ file | Trên backend Go: `IMPLEMENTS` chỉ do nhúng struct, `CALLS` tới stub gRPC khớp theo tên, 0 `Process` có bước ở use case/adapter DB/gRPC (xem CR-CV-032, 034) |
| S4 | Mọi suy luận có `origin`/`confidence` và nhãn "suy luận" ở UI; ghi đè của người dùng (`c4.yaml`, `erd-links.yaml`) thắng | README v7 mục 7: C4 và luồng là heuristic |
| S5 | Không lưu mã nguồn thô vào DB hay log; cache theo blob oid (file sạch) hoặc `sha256` (file bẩn) | Quy ước series: không đưa secret vào cache/log |
| S6 | Dialect: tách `ErdModel` Postgres và MySQL; luồng dữ liệu chọn adapter theo `dialect` (mặc định Postgres) | O1 |
| S7 | Message proto dùng tiền tố (`Erd*`, `C4*`, `DataFlow*`) thay vì tên chung của 08 | Tránh va chạm tên trong `orca.codeintel.v1` |
| S8 | Tên RPC lấy đúng README mục 3.6; không khai báo RPC chưa có message (mẫu v6 CR-REQ-001) | `buf breaking` |

## Dữ liệu ngoài (nghiên cứu 09) mà từng CR phụ thuộc

| CR | E |
|---|---|
| 030 | E1–E4, E8, E10 (đọc), **E5** (git log), E12 (CODEOWNERS, chưa xác nhận), E16 (qua CR-CV-012) |
| 031 | **E1**, **E7** (một phần có sẵn qua chú thích `logical FK`, phần còn lại cần người dùng), E14 tuỳ chọn, E15 chưa đọc |
| 032 | **E2**, **E8** |
| 033 | **E6**, E17, E15 chưa đọc, E4/E10 tuỳ chọn |
| 034 | **E8**, E2, E10 (một phần), **E9 chưa xác nhận** (ngoài phạm vi) |
| 035–038 | xem từng CR |

## Phạm vi ngoài feature này

Trích xuất từ GitNexus/CodeGraph (`agent-codeintel`); service nền, cache snapshot, collector (`code-intel-service-foundation`, `code-intel-graph-pipeline`); kênh `codeIntel.*` (`code-intel-gateway`); mọi render (Mermaid, treemap, React Flow) ở `review-frontend`; `relay-ssh` (Part B) và Windows agent (CR-CV-006 / chưa khảo sát); quy tắc C4 cho container TypeScript (`agent`, `frontend`, `desktop`); workflow do `workflow-service` định nghĩa (E9 chưa xác nhận).

## Điểm lệch giữa README v7/nghiên cứu và code, phát hiện khi viết feature này

- README v7 và đề bài nhắc "secureFs" cho `fs.*`; thực tế chỉ `fs.writeFile` có kiểm tra nằm trong `workDir` (`agent/src/relay/fs-agent-write-extensions.ts:43`), còn `fs.readDir`, `fs.readFile`, `fs.stat`, `fs.glob`, `fs.grep` dùng đường dẫn tuyệt đối nguyên trạng. Backend phải tự chặn (CR-CV-030).
- `specs/agent/api/agent-rpc-catalog-git-fs.md` lỗi thời so với code Part A: có thêm `git.status`, `git.diff`, `git.stage`, `git.fetch`, `git.upstreamStatus`, `git.init`, … (`agent-rpc-dispatch-git-status.ts`).
- `git.exec` Part A không có `ls-tree`, `cat-file`, `ls-files`, `merge-base`, `for-each-ref`, và cấm cờ trước subcommand (`-c core.quotePath=false`), cấm `|`, `!`, `<`, `>`, `;`, `&`, `$`, backtick, `\` trong tham số; stdout không giới hạn dung lượng.
- `fs.readFile` bỏ qua tham số `maxBytes` mà `git-gateway-service` gửi; `git.history` trả `items` còn `RelayExecutor.History` đọc `commits`; `fs.grep` nhận `root/pattern/maxResults` còn `RelayExecutor.Search` gửi `repoPath/isRegex/pathGlob` (không sửa ở đây).
- Số `.up.sql`: 294 (cộng 294 `.down.sql` = 588). `mcp-service` chỉ có `postgres`. Số thứ tự migration có khoảng trống (ví dụ `infra-fleet-service` thiếu `0007–0012`) nhưng không trùng. Một số migration sinh RLS bằng khối `DO $$ … EXECUTE format(...)` mà parser DDL thông thường không thấy.
- Tên schema Postgres không luôn bằng tên service (`credential`, `scm`, `usage`, `ai_provider`, `issuestatussync`, `issuetracking`).
- 17 file migration Postgres có chú thích `logical FK -> <service>` (nguồn E7 rẻ); `tenant_id` có ở hầu hết bảng nên không vẽ thành cạnh.
- 7 RPC của `InfraFleetService` (`ApplyTerraformPlan`, các `*FleetDefinition*`, `ExportFleetDefinitionYaml`) khai báo trong proto và có use case, nhưng không có phương thức gRPC nào cài đặt (kiểm bằng `grep`).
- gRPC client không chỉ ở `adapter/grpcclient` mà còn ở `adapter/scmstarcheck`, `adapter/serviceclients`, `cmd/server/main.go`, `wscompat`.
- Đăng ký kênh `wscompat` không chỉ ở `channels_*.go` (cả `workspace_events.go`, `register_production.go`); 6 điểm đăng ký dùng tên kênh biến; có kênh relay xuống method agent, có kênh cục bộ.
- Số liệu "76 Process của backend-go" tồn tại nhưng không có bước nào ở use case/adapter DB/gRPC (README v7 mục 1 chỉ nêu 300 `Process`).

## Điều chỉnh hợp đồng cần chốt khi duyệt (đề nghị sửa README v7 mục 3 và 8)

1. **Mã lỗi** (mục 3.3): thiếu mã cho "dev server offline" (`INFRA_DEV_SERVER_NOT_CONNECTED`); đề nghị thêm `CODEINTEL_DEV_SERVER_OFFLINE`, hoặc quy ước dùng `CODEINTEL_TOOL_UNAVAILABLE`.
2. **RPC** (mục 3.6): không có chỗ lộ catalog hợp đồng/kênh `wscompat` (CR-CV-032). Đề nghị mở rộng `GetRouteMap` hoặc thêm `GetContractCatalog`. `GetArchitecture` mơ hồ giữa `ArchitectureGraph` (cụm, 05) và `C4ComponentView` (CR-CV-033 giả định là C4).
3. **Tên message** (mục 3.4 và 08): tên chung `Table`, `Column`, `Relation`, `Component`, `ExternalRef`, `FlowStep`, `StoreAccess` va chạm trong một package; các CR này dùng `Erd*`, `C4*`, `DataFlow*` (giữ `StoreAccess`, `StoreRef`, `ComponentRef`).
4. **Bảng** (mục 3.5): `erd-links.yaml` (E7) không có chỗ lưu; hoặc để trong repo, hoặc thêm bảng như `c4_overrides` (CR-CV-031 Q2).
5. **README mục 1** nên ghi rõ chiều Go: `Process` không đi vào use case/adapter, nên "dùng GitNexus Process" cho luồng Go chỉ mang tính làm giàu.
6. **Quy ước envelope** `CodeIntelResultMeta` (phần đầu chung ở mục 3.2) cần tên message proto từ CR-CV-020; các CR ở đây dùng tên đó chưa chốt.

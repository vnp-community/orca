# BE-CV-SOL-012-index-status-aggregation: Tổng hợp trạng thái index (`GetIndexStatus`, `IndexStatus.overall`)

> **📋 Proposed.** Chưa chạy build/test nào. Phần thứ hai của CR-CV-012; cần [`BE-CV-SOL-012-target-resolution-and-bindings`](./BE-CV-SOL-012-target-resolution-and-bindings.md).

**CR:** [CR-CV-012](../../../../../../docs/crs/v7/code-intel-service-foundation/CR-CV-012-project-worktree-to-repo-binding.md) (mục 2.5, 2.7)
**Service:** `code-intel-service` (`internal/domain`, `internal/usecase`, `internal/adapter/{grpc,grpcclient}`)
**TDD tham chiếu:** [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (mục "Talking to the Dev Server Agent"), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (mục "Resilience patterns"), [`services/infra-fleet-service`](../../../../tdd/services/infra-fleet-service.md)

---

## Hợp đồng áp dụng

| Mục | Áp dụng |
|---|---|
| PQ-08 | `IndexStatus` (tổng hợp) ≠ `ToolIndexStatus`; `overall ∈ OFFLINE\|UNKNOWN\|NOT_INSTALLED\|BUILDING\|MISSING\|DEGRADED\|OVERLAY\|STALE\|READY`; `OVERLAY` giữa `DEGRADED` và `STALE` khi `indexScope=repo_root` ∧ `freshness=fresh_base`; trên dây `indexScope ∈ exact\|repo_root\|stale\|none`, `freshness ∈ fresh\|fresh_base\|stale\|unknown`; cột DB `index_scope ∈ exact\|repo_root\|unresolved` |
| PQ-19 | Hình dạng `codeintel.status` của CR-001 + CR-080 là chuẩn; `binding.worktreeMismatch` boolean, `indexes.codegraph.rootMismatch` |
| PQ-32 | `overall` chữ HOA |
| PQ-16 | `activeJob.percent` có thể `null` |
| PQ-13 | Timeout Go cho `codeintel.status` 30 s; trạng thái ghi/trạng thái phía gateway 8 s; `CODEINTEL_STATUS_TIMEOUT` 10 s do service tự đặt |
| §2.3 / UI-API §4.1 | `IndexStatus` JSON: `{overall, tools[], scopeMismatch, activeJob?, indexBasis[], lastStatusAt, errorCode?, binding}`; đường dẫn tuyệt đối không ra UI |
| Agent contract §4.1 | Tham số `workspaceRoot`, `baseRef?`; kết quả luôn thành công; `data.{binding,tools,indexes,sqliteReadAvailable,host,limits}`; `perf`, phong bì |
| §3.1 | `GetIndexStatus` (`read`, `refresh`) → `codeIntel.status` |
| PQ-02 | Lỗi nguyên nhân gốc `INFRA_AGENT_EXEC_FAILED` mất mã agent cho tới SOL-023 → `UNKNOWN` kèm `error_code` thô |

## Lệch giữa CR và hợp đồng

| # | CR-CV-012 nói | Hợp đồng | Xử lý |
|---|---|---|---|
| L1 | `data.tools[]` mảng, mỗi phần tử có `indexScope: exact\|repo_root\|none`, `state`, `registered`, `repoName`… | Agent §4.1: `data.tools{gitnexus,codegraph}` (khả dụng/phiên bản) và `data.indexes.<tool>` (`state`, `indexScope`, `freshness`, …), `data.binding.{gitnexus,codegraph}` (`name`,`path`) | Bộ giải mã theo agent contract (PQ-19 (1)); CR-012 ánh xạ từ đó |
| L2 | 9 trạng thái nhưng chuỗi thứ tự 8 có `scopeMismatch` | PQ-08: thêm `OVERLAY`; `scope_mismatch` là cờ riêng | Bảng ưu tiên mới ở mục 2.C |
| L3 | `indexScope` ba giá trị | PQ-08: bốn giá trị trên dây + `freshness` | `ToolIndexStatus` mang cả hai |
| L4 | `CODEINTEL_DEV_SERVER_OFFLINE` `FailedPrecondition` | `Unavailable` (PQ-03) | `KindUnavailable` cho RPC khác `GetIndexStatus` |
| L5 | `worktreeMismatch` trong từng chỉ mục | PQ-19 (2): `binding.worktreeMismatch` boolean; `indexes.codegraph.rootMismatch` | Bộ giải mã theo PQ-19 |
| L6 | Bắt buộc `activeJob` từ DB | `IndexStatus.active_job = {id, stage, percent?}` | Lấy từ `reindex_jobs` queued/running |

## Phụ thuộc chéo khu vực

| Khu vực | Solution | Quan hệ |
|---|---|---|
| BE | `BE-CV-SOL-012-target-resolution-and-bindings` | Trước: `ResolveTarget`, binding, client infra-fleet |
| BE | `BE-CV-SOL-011-*` | Trước: `SaveStatusCache`, `ReindexJobRepository` |
| BE | `BE-CV-SOL-013-agent-call-gate-and-quotas` | `GetIndexStatus` (class `light`) đi qua `AgentCallGate`; cổng tiêm, nil an toàn cho đến khi 013 xong |
| BE | `BE-CV-SOL-023-infra-fleet-codeintel-transport` | Sau/song song: mã lỗi agent qua trailer; timeout Go theo method |
| BE | `BE-CV-SOL-024-event-distribution`, `-021`, `-036`, `-080-auto-refresh-index` | Dùng/huỷ cache trạng thái; `index_basis`, `OVERLAY` |
| AG | `AG-CV-SOL-001-codeintel-agent-foundation`, `AG-CV-SOL-080-index-basis-and-reindex-triggers` | Nguồn `codeintel.status`; `indexScope/freshness` (CR-080) |
| AG | `AG-CV-SOL-070-golden-fixtures-and-parsers` | Tệp vàng `status` (G1) cho test; trước đó dùng fixture tay theo §4.1 |
| FE | `FE-CV-SOL-050-types-and-runtime-bridge` | Tiêu thụ `IndexStatus` (đổi từ mảng sang object) |

---

## 1. Trạng thái hiện tại (Re-verify)

Đã đọc: hợp đồng agent §2.2, §2.5, §4.1; UI-API §4.1; `infra-fleet-service/internal/usecase/relay_by_dev_server.go` (lỗi `INFRA_DEV_SERVER_NOT_CONNECTED` `FailedPrecondition` ngay khi không có phiên; `INFRA_AGENT_EXEC_FAILED` `Internal` cho mọi lỗi `Exec`), `devserveragent/client.go` (`execTimeoutForMethod`: 0 = mặc định `cfg.RequestTimeout` 30 s trừ `agent.execPrompt`), `common/apperrors/apperrors.go` (`ToGRPCStatus` bỏ nguyên nhân gốc), `proto/orca/infrafleet/v1/infrafleet.proto` (`RelayByDevServerRequest{dev_server_id, method, params_json}`; `IsDevServerConnected`).

### Correction relative to CR-CV-012

| # | CR nói | Mã thật / hợp đồng | Xử lý |
|---|---|---|---|
| C1 | `RelayByDevServer` trả kết quả `map` | Response là `RelayResponse` chứa JSON (`result_json` theo hợp đồng §3.3) | Giải mã vào `map[string]any` rồi vào struct; kiểm trường `data` |
| C2 | Hạn chót 10 s | Go cắt ở 30 s mặc định (PQ-13) | `context.WithTimeout(CODEINTEL_STATUS_TIMEOUT)` ở service, nhỏ hơn 30 s |
| C3 | Lỗi `INFRA_AGENT_EXEC_FAILED` có thể phân loại | `ToGRPCStatus` bỏ nguyên nhân; `status.Message = "INFRA_AGENT_EXEC_FAILED: failed to relay…"` | `UNKNOWN` với `error_code="INFRA_AGENT_EXEC_FAILED"` cho tới SOL-023; sau đó dùng `CODEINTEL_*` thật (`CODEINTEL_AGENT_UNSUPPORTED` cho `-32601`) |
| C4 | `activeJob` lấy từ DB | `ReindexJobRepository` (SOL-011) có `ListByBinding` | Dùng job `queued/running` mới nhất |

Chưa kiểm chứng: agent thực tế trả đủ `indexes.*.indexScope/freshness` (phụ thuộc CR-001/080); `Message` của `INFRA_DEV_SERVER_NOT_CONNECTED` đến service qua `status.Code() == FailedPrecondition` + tiền tố mã; hành vi `singleflight` khi ctx người gọi huỷ.

## 2. Giải pháp

### A. Giải mã `codeintel.status` (`internal/domain/agent_status.go`, mới)

```go
type AgentStatus struct {            // theo agent contract §4.1; trường lạ bỏ qua
    Binding AgentBinding; Tools map[string]AgentTool; Indexes map[string]AgentIndex
    HeadCommit string; Stale bool; Warnings []string
}
func DecodeAgentStatus(raw map[string]any) (AgentStatus, error) // CODEINTEL_RESULT_INVALID khi sai kiểu bắt buộc
```

Chuỗi/độ dài bị giới hạn (đường dẫn ≤ 4096, chuỗi ≤ 512); số âm bị từ chối; `stats` giữ số nguyên không âm. `percent` lỗi kiểu → `nil`.

### B. Ánh xạ sang proto (`ToolIndexStatus`) và binding

Mỗi công cụ (`gitnexus`, `codegraph`): `available/version/supported` từ `tools`, `state/indexedCommit/indexedAt/stats/pendingChanges/indicators` từ `indexes`, `indexScope`/`freshness` giữ nguyên chuỗi (enum lạ → `unknown`/`none`). Ghi vào binding: `gitnexus_repo = binding.gitnexus.name`, `codegraph_path = binding.codegraph.projectPath`, `index_scope` DB: `exact` nếu `indexScope=exact`; `repo_root` nếu `repo_root` hoặc `stale` với gốc khác; `unresolved` khi `none` (PQ-08 (4)). **Backend không tự chọn tên repo.** Đường dẫn tuyệt đối (`registeredPath`, `indexRoot`, `storagePath`, `binary`) **không** nằm trong `IndexStatus` trả đi.

### C. Bảng ưu tiên `overall` (đầu tiên khớp thì dừng)

| # | Điều kiện | `overall` |
|---|---|---|
| 1 | Cờ tắt | không tới đây (SOL-013 chặn trước) |
| 2 | `IsDevServerConnected=false` hoặc lỗi `FailedPrecondition`+`INFRA_DEV_SERVER_NOT_CONNECTED` | `OFFLINE` (trả `last_status` đã lưu + `last_status_at`; không phải lỗi RPC) |
| 3 | Gọi `codeintel.status` lỗi/hết hạn/`-32601` | `UNKNOWN` + `error_code` thô |
| 4 | Không công cụ nào `available` | `NOT_INSTALLED` |
| 5 | Có `reindex_jobs` `queued/running` | `BUILDING` + `active_job` |
| 6 | Mọi công cụ khả dụng có `state=missing` | `MISSING` |
| 7 | Có công cụ `ready` và công cụ khả dụng khác `missing/stale` | `DEGRADED` |
| 8 | Có công cụ `ready` với `indexScope=repo_root` ∧ `freshness=fresh_base` và không công cụ nào `exact/fresh` | `OVERLAY` |
| 9 | Có công cụ `stale` (hoặc `indexedCommit≠headCommit`) và không công cụ `ready` | `STALE` |
| 10 | còn lại | `READY` |

`scope_mismatch=true` khi bất kỳ công cụ `indexScope=repo_root` hoặc `binding.worktreeMismatch`. Thứ tự giữ nguyên ưu tiên (offline + job → `OFFLINE`). Hàm thuần `ComputeOverall(statuses, activeJob, offline, probeErr) Overall` để bảng test từng dòng.

### D. Thăm dò, cache, lưu

`GetIndexStatus.Execute(ctx, sel, refresh)`: `ResolveTarget` (lười tạo binding) → cache bộ nhớ `IndexStatus` 15 s (`CODEINTEL_STATUS_TTL`) khoá `binding.id` (+ tenant), `singleflight` theo khoá (ctx của lời gọi dẫn đầu tách khỏi huỷ của người đợi: dùng `context.WithoutCancel` + timeout riêng) → `AgentCallGate.Acquire(class=light)` (cổng tiêm; 013) → `RelayByDevServer{method:"codeintel.status", params_json:{"workspaceRoot", "baseRef"?}}`. Mỗi lần thăm dò thành công: `SaveStatusCache(last_status JSON ≤ 64 KiB (bỏ languages/pendingChanges rồi cắt), last_status_at)`. `refresh=true` bỏ qua cache. `Invalidate(bindingID)` cho SOL-024 gọi khi `indexChanged`.

Phản hồi gateway: `CODEINTEL_TIMEOUT` khi quá 8 s là việc gateway/SOL-040; service tự giới hạn 10 s cho agent nên với `GetIndexStatus` trả `UNKNOWN` thay vì lỗi (đã nêu hàng 3).

### E. Cây file (mới, tiền tố `backend-go/services/code-intel-service/`)

```
internal/domain/{agent_status.go,index_overall.go}      # DecodeAgentStatus, ComputeOverall
internal/usecase/{get_index_status.go,index_status_cache.go}
internal/adapter/grpc/index_status_server.go             # GetIndexStatus handler
internal/adapter/grpcclient/agent_status_prober.go       # RelayByDevServer → map
testdata/agent-status/*.json                              # fixture theo §4.1 (tay, thay bằng tệp vàng G1 khi có)
```

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | `OFFLINE` không phải lỗi của `GetIndexStatus` | UI cần chip xám + dữ liệu lần cuối |
| D2 | `codeintel.status` luôn thành công khi agent chạy; thiếu công cụ/chỉ mục là trạng thái | Mã lỗi agent bị `RelayByDevServer` làm mất |
| D3 | `ComputeOverall` hàm thuần | Kiểm bảng thứ tự ưu tiên dễ |
| D4 | `index_scope` DB ≠ `indexScope` dây | PQ-08 (4) |
| D5 | Không đưa đường dẫn tuyệt đối ra UI | Hợp đồng agent §4.1, H8 |
| D6 | `singleflight` tách huỷ | Một người đóng tab không huỷ thăm dò của người khác |

## 4. Tiêu chí chấp nhận

- [x] `GetIndexStatus` trả đúng 9 `overall` (mỗi dòng bảng 2.C một test), đúng ưu tiên khi nhiều điều kiện đúng (offline + job → `OFFLINE`).
- [x] `OVERLAY` khi `repo_root` + `fresh_base`; `scope_mismatch` đặt đúng.
- [x] Dev server offline: `OFFLINE` + `last_status` đã lưu, không lỗi; RPC khác (ví dụ `RequestReindex`/view) trả `CODEINTEL_DEV_SERVER_OFFLINE` (`Unavailable`).
- [x] 20 lời gọi đồng thời cùng binding → đúng một `RelayByDevServer`; `refresh=true` bỏ qua cache; cache hết hạn sau 15 s (đồng hồ giả).
- [x] Mỗi thăm dò thành công ghi `last_status`/`last_status_at`; trạng thái lưu ≤ 64 KiB.
- [x] `percent` không biết ra `null`; không đường dẫn tuyệt đối trong `IndexStatus`.
- [x] Cô lập tenant: binding/cache tenant khác không đọc được.
- [x] Hai dialect: `SaveStatusCache` đúng ở cả hai.

## 5. Kiểm thử

- **Unit:** `DecodeAgentStatus` (fixture đủ/thiếu/kiểu sai); `ComputeOverall` bảng; ánh xạ `index_scope` DB; cache + singleflight + đồng hồ giả; `refresh`.
- **Hợp đồng agent:** fixture theo §4.1 (đối chiếu tệp vàng `AG-CV-SOL-070` khi có; kiểm lệch phiên bản công cụ).
- **Integration (hai dialect):** `SaveStatusCache` và đọc lại.
- **Chưa chạy bất kỳ test nào.**

## 6. Rủi ro và điểm chưa kiểm chứng

- Agent có thể chưa trả `indexScope/freshness` đúng (phụ thuộc CR-001/080) → worktree liên kết luôn `STALE`/`UNKNOWN`; rủi ro sản phẩm lớn (O-6).
- Trước SOL-023 mọi lỗi agent thành `UNKNOWN`.
- Cache trạng thái 15 s/replica; nhiều bản sao tăng số lần thăm dò.
- Hành vi chip `UNKNOWN` khi agent cũ (`-32601`): `CODEINTEL_AGENT_UNSUPPORTED` chỉ sau SOL-023.

## 7. Câu hỏi mở

- **Q1.** Có hiển thị `host.loadavg1`/`limits` cho UI? Mặc định: không (chỉ backend).
- **Q2.** Điều kiện `OVERLAY` khi chỉ một công cụ `fresh_base`: mặc định theo bảng (một công cụ đủ).
- **Q3.** Chuyển cache trạng thái sang DB để dùng chung giữa bản sao? Mặc định: không.

## 8. Tham chiếu

- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md` PQ-08, PQ-13, PQ-16, PQ-19, PQ-32; §2.3, §3.1
- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-agent-rpc.md` §2.2, §2.5, §4.1
- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-ui-api.md` §4.1
- `/opt/repos/orca/docs/crs/v7/code-intel-service-foundation/CR-CV-012-project-worktree-to-repo-binding.md`
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/usecase/relay_by_dev_server.go`, `…/adapter/devserveragent/client.go`

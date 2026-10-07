# BE-CV-SOL-013-agent-call-gate-and-quotas: Cổng gọi agent, hạn mức tốc độ/đồng thời, admission reindex, che bí mật

> **📋 Proposed.** Chưa chạy build/test nào. Phần thứ hai của CR-CV-013; cần [`BE-CV-SOL-013-authorization-flags-and-audit`](./BE-CV-SOL-013-authorization-flags-and-audit.md) cho pipeline, audit, mã lỗi.

**CR:** [CR-CV-013](../../../../../../docs/crs/v7/code-intel-service-foundation/CR-CV-013-authorization-audit-and-quotas.md) (mục 2.5, 2.6)
**Service:** `code-intel-service` (`internal/domain`, `internal/usecase`, `internal/adapter/{callgate,grpcclient}`)
**TDD tham chiếu:** [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (mục "Resilience patterns"), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (mục "Input validation & supply chain", "Secrets"), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (mục "Talking to the Dev Server Agent"), [`services/api-gateway`](../../../../tdd/services/api-gateway.md) (mục rate limiting), [`services/infra-fleet-service`](../../../../tdd/services/infra-fleet-service.md)

---

## Hợp đồng áp dụng

| Mục | Áp dụng |
|---|---|
| §3 bước 6, 8 | Cổng agent (6); che bí mật (8) |
| PQ-03 (4) | `CODEINTEL_OUTPUT_TOO_LARGE` = đầu ra công cụ/agent/collector vượt trần (`FailedPrecondition`); `CODEINTEL_RESPONSE_TOO_LARGE` do gateway sinh |
| PQ-03 (8) | Hạn mức dùng `KindResourceExhausted` (không `LimitError` riêng) |
| PQ-02 (3) | Dữ liệu máy đọc ở hậu tố `" \| {"retryAfterSeconds":N,"scope":"…"}"` |
| PQ-13 | Collector tự chờ nối lại 20 s và gọi agent với timeout 90 s/30 s; cổng đặt **ngoài** lời gọi và nhả khe khi ctx hết hạn; khi service vượt 20 s trả `CODEINTEL_TIMEOUT` và tiếp tục nền (singleflight 100 s): khe giữ tới khi lời gọi nền kết thúc |
| PQ-16 | `RequestReindex`: một job/binding qua `UNIQUE(active_key)`; trạng thái DB 5 giá trị; `trigger='manual'` do service đặt |
| PQ-21 | Chỉ method trong agent contract §4–5 (bảng phân lớp bên dưới bám đúng danh sách) |
| H3, H8 | Không nhận lệnh/args từ UI; không log mã nguồn/bí mật |
| O-15, O-17, O-18 | Số liệu hạn mức là giả định chưa đo; không có `CancelReindex`; `member` được `reindex` có hạn mức |

## Lệch giữa CR và hợp đồng

| # | CR-CV-013 nói | Hợp đồng | Xử lý |
|---|---|---|---|
| L1 | `domain.LimitError{Code, RetryAfter}` + `status_mapping.go` riêng | PQ-03 (8) thêm Kind ở `common/apperrors` | Dùng `apperrors.New(KindResourceExhausted, code, msg, nil)` + `CodedData`; mapper chung ở SOL-013-authorization (task 013-07) |
| L2 | `retry_after_seconds=N` trong message | PQ-02 (3) hậu tố JSON `retryAfterSeconds` (UI-API §2.3: `data.retryAfterSeconds?`, `scope?`) | Hậu tố JSON |
| L3 | `CODEINTEL_FEATURE_DISABLED` | PQ-01 | Không liên quan ở solution này |
| L4 | Lớp `light`/`heavy`/`source_read` theo method CR-001 | Danh sách method cuối (§4–5): thêm `structuralFacts`, `codegraphSearch`, `files`, `quality.*`, `reindex*`, `watch` | Bảng phân lớp mục 2.B (đầy đủ) |
| L5 | `GetSymbol` > 200 KiB bị từ chối `OUTPUT_TOO_LARGE` | PQ-14 (1): response `GetSymbol` ≤ 320 KiB; mã `OUTPUT_TOO_LARGE` dành cho đầu ra công cụ vượt trần | Nội dung `source` > 200 KiB bị từ chối bằng `CODEINTEL_OUTPUT_TOO_LARGE` (≤ trần 320 KiB của response); ghi hai số riêng |
| L6 | Đếm reindex theo dev server "chấp nhận cuộc đua" | §4 T7 `active_key` chỉ cho binding | Giữ cách đếm không nguyên tử cho dev server/tenant; ghi rủi ro |
| L7 | Nghỉ sau job thành công 5 phút | UI-API `CODEINTEL_REINDEX_COOLDOWN` `retryAfterSeconds` | Giữ; trả `retryAfterSeconds` còn lại |
| L8 | Số liệu hạn mức mặc định | O-15: chưa đo | Cấu hình `CODEINTEL_*`; không coi là cam kết |

## Phụ thuộc chéo khu vực

| Khu vực | Solution | Quan hệ |
|---|---|---|
| BE | `BE-CV-SOL-013-authorization-flags-and-audit` | Trước/cùng: pipeline, `AuditRecorder`, `toStatus`, `CodedData` |
| BE | `BE-CV-SOL-010-*` | `KindResourceExhausted`; config |
| BE | `BE-CV-SOL-011-repositories-and-maintenance` | `CountActiveByDevServer/Tenant`, `LastSucceededFinishedAt`, `Create` job |
| BE | `BE-CV-SOL-012-index-status-aggregation` | `GetIndexStatus` đi qua cổng (lớp light) |
| BE | `BE-CV-SOL-021-agent-collector` | **Bắt buộc** bọc mọi `RelayByDevServer` của `codeintel.*` bằng `GatedAgentRelay`; cache trúng không tốn hạn mức |
| BE | `BE-CV-SOL-030-repo-file-access-gateway` | Lớp `source_read` cho `files`/`git` |
| BE | `BE-CV-SOL-021` (`RequestReindex`), `BE-CV-SOL-080-auto-refresh-index` | Dùng `ReindexAdmission` (manual); trigger tự động bỏ qua cooldown (chốt ở 080) |
| BE | `BE-CV-SOL-071-metrics-tracing-and-budgets` | Số liệu hạn mức; số đo thật thay giá trị khởi đầu |
| BE | `BE-CV-SOL-072-security-tests-service-gateway` | Test bỏ qua cổng/che bí mật |
| AG | `AG-CV-SOL-001-*`, `AG-CV-SOL-004-reindex-and-index-notifications`, `AG-CV-SOL-081-*` | Agent tự từ chối chồng lấn (`ORCA_CODEINTEL_MAX_REINDEX`, `ORCA_HEAVY_JOBS`); lớp này là tuyến đầu backend |
| FE | `FE-CV-SOL-050-*` | Hiển thị `rate-limited` + `retryAfterSeconds` |

---

## 1. Trạng thái hiện tại (Re-verify)

Đã đọc: `api-gateway/internal/usecase/rate_limit.go` (`NewRateLimiter(rps, burst)` bọc `rate.NewLimiter`, map theo tenant; `golang.org/x/time v0.15.0` là **phụ thuộc trực tiếp** ở `api-gateway/go.mod:21`, gián tiếp ở `task-service`, `mcp-service`), `mcp-service/internal/domain/secret_redactor.go` (9 mẫu regex, dấu `[REDACTED]`, `Redact(s) (string, bool)`), `common/apperrors/apperrors.go` (chưa có Kind hạn mức; SOL-010 task 02 thêm), `infra-fleet-service` `devserveragent/client.go` (timeout Go mặc định 30 s, không giới hạn đồng thời theo dev server ở lớp relay), hợp đồng agent §2.3 (agent tự giới hạn 3 tiến trình, `gitnexus ≤ 2`, hàng đợi ≤ 16, chờ ≤ 10 s) và §4–5 (danh sách method).

### Correction relative to CR-CV-013

| # | CR nói | Mã thật | Xử lý |
|---|---|---|---|
| C1 | "Không mã nguồn nào gọi `rate.NewLimiter` (grep rỗng)" | **Sai**: `api-gateway/internal/usecase/rate_limit.go:46` gọi `rate.NewLimiter` | Mẫu cài đặt có sẵn; `x/time` đã là phụ thuộc trực tiếp ở api-gateway; module mới thêm `golang.org/x/time v0.15.0` trực tiếp |
| C2 | Tách riêng `LimitError` vì `apperrors` thiếu Kind | PQ-03 (8) + SOL-010 task 02 | Không cần |
| C3 | Agent đã giới hạn song song | Có ở agent (§2.3), nhưng Go không biết trạng thái agent | Cổng backend là tuyến đầu; không phụ thuộc agent |
| C4 | `RelayByDevServer` không kiểm user/nhóm | Đúng (đã đọc) | Cổng không thay thế quyền; quyền ở SOL-013-authorization |

Chưa kiểm chứng: mọi con số hạn mức (O-15); `bucket` theo user cho nhiều bản sao; chi phí bộ che trên payload lớn; `.gitignore` do agent tôn trọng (CR-001).

## 2. Giải pháp

### A. `AgentCallGate` (`internal/usecase/agent_call_gate.go` + `internal/adapter/callgate/token_bucket_gate.go`, mới)

```go
type GateClass int // ClassLight, ClassHeavy, ClassSourceRead
type GateRequest struct{ TenantID, UserID, DevServerID string; Class GateClass }
type AgentCallGate interface {
    Acquire(ctx context.Context, r GateRequest) (release func(), err error) // luôn nhả khe khi ctx hết hạn; release idempotent
}
```

Hạn mức (mặc định khởi đầu, **chưa đo**, tất cả cấu hình):

| Hạn mức | Mặc định | Biến |
|---|---|---|
| Đồng thời mỗi dev server (mọi lớp) | 4 | `CODEINTEL_MAX_INFLIGHT_PER_DEV_SERVER` |
| Đồng thời `heavy` mỗi dev server | 2 | `CODEINTEL_MAX_HEAVY_PER_DEV_SERVER` |
| Đồng thời mỗi tenant | 16 | `CODEINTEL_MAX_INFLIGHT_PER_TENANT` |
| `heavy` mỗi người dùng | 30/phút, burst 10 | `CODEINTEL_HEAVY_PER_MINUTE` |
| `light` mỗi người dùng | 120/phút, burst 30 | `CODEINTEL_LIGHT_PER_MINUTE` |
| Chờ khe trống | 2 s | `CODEINTEL_GATE_WAIT` |
| `source_read` mỗi người dùng | như `light` (hằng ghi rõ) | — |

Thứ tự `Acquire`: (1) token bucket theo `(tenant,user,class)` (lỗi → `CODEINTEL_RATE_LIMITED`), (2) semaphore tenant, dev server, dev-server-heavy với chờ tối đa `GateWait` (hết → `CODEINTEL_CONCURRENCY_LIMIT`), (3) trả `release` nhả theo thứ tự ngược. Lỗi đi kèm `CodedData{"retryAfterSeconds":N,"scope":"user|devServer|tenant"}`.

### B. Phân lớp method agent (`internal/domain/agent_method_class.go`, mới)

| Lớp | Method (agent contract §4–5) |
|---|---|
| `light` | `codeintel.status`, `symbol`, `routes`, `reindex`, `reindexStatus`, `reindexCancel`, `watch`, `quality.runStatus`, `quality.cancel`, `quality.results`, `quality.coverage` |
| `heavy` | `codeintel.overview`, `processes`, `process`, `subgraph`, `impact`, `detectChanges`, `structuralFacts`, `codegraphSearch`, `quality.listProfiles`, `quality.run` |
| `source_read` | `codeintel.files` và `fs.*`/`git.*` do CR-030 |

Method lạ → `heavy` (bảo thủ) và log WARN; test kiểm mọi method của hợp đồng có lớp tường minh.

### C. `GatedAgentRelay` (`internal/adapter/grpcclient/gated_agent_relay.go`, mới)

Bọc cổng `AgentRelay{Call(ctx, devServerID, method string, params map[string]any) (map[string]any, error)}` của SOL-012/021: `class := ClassOf(method)`; `Acquire` → gọi `RelayByDevServer` → `release`. Cổng `AgentRelay` là **đường duy nhất** tới `infra-fleet`'s `RelayByDevServer` trong module (test AST: chỉ file này import `InfraFleetServiceClient.RelayByDevServer`). Khe giữ cả khi người gọi huỷ nếu lời gọi nền còn chạy (singleflight 100 s, PQ-13): `release` gọi khi lời gọi thật kết thúc, không khi ctx người gọi huỷ.

### D. Giới hạn thô L1 (`internal/usecase/user_rate_limiter.go`, mới)

`rate.Limiter` theo `(tenant,user)`: `CODEINTEL_USER_RPS` 20, burst 40, mọi RPC, bộ nhớ, dọn mục rảnh sau 10 phút. Lỗi `CODEINTEL_RATE_LIMITED` (`retryAfterSeconds` ≥ 1).

### E. `ReindexAdmission` (`internal/usecase/reindex_admission.go`, mới)

`Admit(ctx, target) error` trước `ReindexJobRepository.Create`: (1) user ≤ 6/giờ (`CODEINTEL_REINDEX_PER_USER_PER_HOUR`; `rate.Limiter`); (2) cooldown 5 phút sau job thành công của cùng binding (`LastSucceededFinishedAt`; `CODEINTEL_REINDEX_COOLDOWN` + `retryAfterSeconds` còn lại; **trigger tự động bỏ qua** — quyết định ở SOL-080); (3) `CountActiveByDevServer` < 1 (`CODEINTEL_REINDEX_PER_DEV_SERVER`); (4) `CountActiveByTenant` < 2 (`CODEINTEL_REINDEX_PER_TENANT`); (5) `Create` → vi phạm `active_key` → `CODEINTEL_REINDEX_IN_PROGRESS` (+ `jobId`, `stage` trong `CodedData`). Đếm → tạo không nguyên tử: có thể vượt tạm 1 job. Audit `codeintel.reindex.request` (allowed) / `codeintel.reindex.limit` (denied). Mode ở UI chỉ `incremental|full`, `trigger='manual'` do service đặt (PQ-16).

### F. Che bí mật và đường dẫn nhạy cảm

- `internal/domain/secret_redactor.go` (mới): viết lại theo `mcp-service/.../secret_redactor.go` (9 mẫu, `[REDACTED]`, trả `changed`), không import chéo service; vị trí gói dùng chung với gateway là O-16 (mặc định: giữ trong service; gateway tự quét cuối theo SOL-040/072).
- `internal/domain/sensitive_path.go` (mới): `IsSensitivePath(rel string) bool` theo danh sách chặn của CR mục 2.5 (không phân biệt hoa thường, chuẩn hoá `\`→`/`); khớp → `content=""`, `contentWithheld="sensitive_path"`, không lỗi.
- Áp dụng: `GetSymbol` (source, chữ ký; `redactionCount`), chuỗi lỗi/`stderr` agent trước log/`reindex_jobs.message`/UI (cắt 500), trường văn bản tự do của snapshot (không duyệt toàn payload), `source` > 200 KiB → `CODEINTEL_OUTPUT_TOO_LARGE`. Ghi chú review không che khi lưu và không vào log/audit.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Cổng ở backend độc lập agent | Go không thấy trạng thái agent; bảo vệ dev server dùng chung với agent lập trình |
| D2 | `GatedAgentRelay` là đường duy nhất tới `RelayByDevServer` | Không thể quên `Acquire` |
| D3 | Hạn mức đọc ở bộ nhớ, reindex ở DB | Đọc cần rẻ; reindex cần chính xác toàn cụm |
| D4 | Method lạ = `heavy` | Bảo thủ |
| D5 | Dùng `KindResourceExhausted` + hậu tố JSON | PQ-02/03 |
| D6 | Khe nhả khi lời gọi thật xong, không khi người gọi huỷ | Không dồn tải lên dev server khi người dùng huỷ liên tục |

## 4. Tiêu chí chấp nhận

- [x] `AgentCallGate`: 5 lời gọi `heavy` đồng thời cùng dev server (giới hạn 2) → 2 chạy, còn lại chờ ≤ 2 s rồi `CODEINTEL_CONCURRENCY_LIMIT`; khe nhả khi lời gọi xong/ctx huỷ (test rò rỉ); tenant khác không ảnh hưởng nhau; cache trúng không gọi `Acquire`.
- [x] Hạn mức theo người dùng: vượt `heavy` 30/phút → `CODEINTEL_RATE_LIMITED` kèm `retryAfterSeconds`.
- [x] Reindex: lần hai cùng binding → `CODEINTEL_REINDEX_IN_PROGRESS` (kèm `jobId`); sau thành công trong 5 phút → `CODEINTEL_REINDEX_COOLDOWN`; vượt 1 job/dev server hoặc 2/tenant → chặn; user thứ 7 trong giờ → `CODEINTEL_RATE_LIMITED`; từng nhánh có audit đúng.
- [x] Lỗi hạn mức ra `codes.ResourceExhausted`, message `CODEINTEL_X: … | {"retryAfterSeconds":N,"scope":"…"}`.
- [x] Bộ che: bảng mẫu (GitHub, AWS, Bearer, JWT, khối khoá riêng, `password=…`) → `[REDACTED]`; chuỗi lỗi agent không lọt bí mật vào log/`reindex_jobs.message`.
- [x] `GetSymbol` file nhạy cảm (`.env`, `*.pem`, `secrets/x.yaml`, `.AWS/credentials`) → `content=""`, `contentWithheld="sensitive_path"`, không lỗi; `source` > 200 KiB → `CODEINTEL_OUTPUT_TOO_LARGE`.
- [x] Test AST: chỉ `gated_agent_relay.go` gọi `RelayByDevServer`.
- [x] Cô lập tenant: bucket/semaphore theo tenant, không chia sẻ.
- [x] `go vet`, `make lint` xanh; không `max-lines` disable.

## 5. Kiểm thử

- **Unit:** gate (đồng thời, chờ, nhả khe, đồng hồ giả, nhiều tenant), bucket, L1, `ReindexAdmission` (đồng hồ giả, fake repo), phân lớp method (bảng đủ), bộ che (bảng vào/ra + dương tính giả), danh sách chặn (kể cả `\` Windows, hoa/thường).
- **An toàn:** AST bỏ-qua-cổng; `CodedData` hợp lệ; `-race` cho gate.
- **Integration (hai dialect):** `CountActiveByDevServer`/`ByTenant`, `UNIQUE(active_key)` đã có ở SOL-011; 20 `Admit` đồng thời cùng binding.
- **Chưa chạy bất kỳ test nào.**

## 6. Rủi ro và điểm chưa kiểm chứng

- Hạn mức ở bộ nhớ nhân theo số bản sao; reindex ở DB toàn cục.
- Đếm reindex theo dev server không nguyên tử.
- Mọi con số hạn mức là giả định (O-15); tải CPU thật dev server không đo.
- Regex che bảo thủ nhưng không đầy đủ (khoá dạng khác, bí mật nhúng trong mã).
- Bộ nhớ bucket tăng theo số `(tenant,user)`: dọn mục rảnh.
- Chi phí che trên payload lớn chưa đo; chỉ che trường văn bản tự do.

## 7. Câu hỏi mở

- **Q1.** Trigger tự động (`agent_done`, `head_change`) có bỏ qua cooldown/hạn mức người dùng? (chốt ở SOL-080).
- **Q2.** Chủ sở hữu bộ che/`pathsafety` chia sẻ với gateway (O-16).
- **Q3.** `CancelReindex` (O-17): chưa có RPC; admission không đụng.
- **Q4.** Số liệu hạn mức từ đo đạc (SOL-071).

## 8. Tham chiếu

- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md` PQ-02, PQ-03, PQ-13, PQ-14, PQ-16, PQ-21; §3; §9 O-15–O-18
- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-agent-rpc.md` §2.3, §2.5, §4–5
- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-ui-api.md` §2.3
- `/opt/repos/orca/docs/crs/v7/code-intel-service-foundation/CR-CV-013-authorization-audit-and-quotas.md`
- `/opt/repos/orca/backend-go/services/api-gateway/internal/usecase/rate_limit.go`, `backend-go/services/mcp-service/internal/domain/secret_redactor.go`

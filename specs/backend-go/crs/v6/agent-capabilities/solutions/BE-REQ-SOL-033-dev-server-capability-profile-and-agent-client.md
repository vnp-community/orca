# BE-REQ-SOL-033: Hồ sơ năng lực dev server ở `infra-fleet-service`, `HandshakeInfo` mang `features`, và client Go của `agent.execPrompt` mới

> **🚧 Đang triển khai: 3/6 task xong** (033-01, 02, 03 trong `infra-fleet-service`, kiểm chứng 2026-10-07; 033-04, 05 và phần `request-service` của 033-06 để đợt R2). Ghi chú triển khai: [IMPLEMENTATION-NOTES](../IMPLEMENTATION-NOTES.md). Tài liệu ngày 2026-10-06; số dòng và đường dẫn đã đối chiếu với `backend-go/services/infra-fleet-service` cùng ngày. `request-service` chưa có thư mục: mọi đường dẫn của nó là "(mới)". Đây chỉ là **phần backend** của CR-REQ-033; phần trong `agent/` do agent khác soạn (xem mục 2.K).

**CR:** [CR-REQ-033](../../../../../../docs/crs/v6/agent-capabilities/CR-REQ-033-agent-readonly-worktree-and-capability-report.md)
**Service:** `infra-fleet-service` (proto, usecase, adapter hai dialect, migration `0039`) · `request-service` (mới, chỉ là người gọi) · `proto/orca/infrafleet/v1`
**Phần agent (khu vực khác):** [`specs/agent/crs/v6/agent-capabilities/`](../../../../../agent/crs/v6/agent-capabilities/solutions/) (file `AG-REQ-SOL-033-*.md`; xem bảng giao diện ở mục 2.K)
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (port nằm ở usecase, adapter không bị import ngược), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (DB mỗi service, RLS, outbox), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (subject, consumer), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (suy giảm, cache), [`services/infra-fleet-service.md`](../../../../tdd/services/infra-fleet-service.md), [`services/task-service.md`](../../../../tdd/services/task-service.md)

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc ngày 2026-10-06:
- `proto/orca/infrafleet/v1/infrafleet.proto`: service `InfraFleetService` (dòng 13), `Relay` (86), `RelayByDevServer` (102), `GetHostCapabilities` (276), `ResolveConnectionResponse{connected, dev_server, repo_path, worktree_id, connection_id}` (dòng 562 đến 575).
- `internal/usecase/ports.go`: `HandshakeInfo` (dòng 85 đến 90) **chỉ có** `Platform, Arch, NodeVersion, AgentVersion`; `DevServerAgentClient` (dòng 411) có `Exec`, `Health`, `LastHandshakeInfo`, `IsConnected`.
- `internal/adapter/devserveragent/session.go`: `HandshakeInfo` (dòng 47 đến 58) có thêm `SessionID`, `Capabilities []string`, `SockPath`; `Client.LastHandshakeInfo` (`client.go` dòng 473 đến 492) chuyển sang bản `usecase` chỉ với bốn trường, nên `Capabilities` bị rơi ở ranh giới adapter.
- `internal/adapter/agentwsserver/server.go`: `inboundHandshakeParams` (dòng 73 đến 81) có `Capabilities`; dựng `HandshakeInfo` ở dòng 212 đến 219. `internal/adapter/sshrelay/provisioner.go`: struct handshake có `Capabilities` (dòng 64), gán ở dòng 255.
- `internal/usecase/get_host_capabilities.go`: mẫu đúng cho "method agent chưa có thì `domain.ErrAgentMethodNotFound`": `Exec(ctx, devServer, "host.capabilities", nil)`, `errors.Is(execErr, domain.ErrAgentMethodNotFound)` dịch sang `KindFailedPrecondition`.
- `internal/adapter/grpc/server.go`: hàm `New(...)` nhận hơn 40 tham số vị trí (dòng 156 trở đi); thêm use case là thêm tham số. `cmd/server/main.go` dòng 316 đến 333 dựng `agentOpts` rồi `infradevserveragent.New(agentCfg, logger, agentOpts...)`; option có dạng `WithRelaySSH`, `WithAgentTokens` (`client.go` dòng 149, 158).
- Migration cuối: `migrations/postgres/0038_session_origin.{up,down}.sql` và bản `mysql`; RLS mẫu `0030_dev_server_approval_status_and_groups.up.sql` dòng 25 đến 26 (`ENABLE ROW LEVEL SECURITY` + `CREATE POLICY tenant_isolation ... current_setting('app.tenant_id', true)::uuid`).
- `agent/src/relay/agent-rpc-dispatch-agent-exec.ts` dòng 66 đến 140: `agent.exec` nhận `{binary, args[], cwd, stdin, env, timeoutMs}`, **không qua shell**, `timeoutMs` kẹp trong `[1000, 300000]`. `agent-rpc-dispatch-git.ts` + `agent-git-handler.ts` dòng 41 đến 64: `git.exec` chỉ cho `status diff add restore commit push pull fetch branch checkout merge rebase stash log worktree remote tag show rev-parse config describe shortlog` và cấm ký tự `& | ; $ \` < > \ !` trong tham số.

### Correction relative to CR-REQ-033

1. CR mục 2.7 viết "mở rộng bản sao `usecase.HandshakeInfo`": đúng, nhưng còn một chỗ nữa phải sửa là phép chuyển đổi ở `devserveragent.Client.LastHandshakeInfo` (`client.go:489`), nếu không trường mới vẫn rơi. Cả hai đường `agentwsserver` và `sshrelay` phải điền trường mới, đường `relay-websocket` (Orca là bên khởi tạo) đọc từ kết quả handshake ở `session.go:267` (`runInitiatorHandshake`).
2. CR-REQ-029 mục 2.4 gợi ý "`agent.exec` `command -v`" cho hồ sơ `degraded`. `agent.exec` không có shell: lệnh đúng là `binary="sh", args=["-c","command -v go"]` trên Linux/macOS và `binary="where.exe", args=["go"]` khi `platform=win32`. Phần này thuộc SOL-029; solution này chỉ bảo đảm `platform` có trong hồ sơ `handshake_only` để bên gọi chọn đúng lệnh.
3. CR-REQ-029 Q3 (`ResolveConnection(worktree_id)` có trả đường dẫn worktree không): **không**, chỉ có `repo_path` và `worktree_id`. Đường dẫn worktree lấy từ `project-service.GetWorktree(worktree_id).path` (`project.proto` dòng 84, 523 đến 530); task đầu của Plan chưa có worktree vì `EnsureWorktree` của `task-service` (`ports.go:373`) chạy trong `Execute`. Không thêm trường ở `infra-fleet-service` cho việc này (xem câu hỏi mở Q3).
4. `BE-REQ-SOL-008` mục G và `TASK-REQ-008-02` dự kiến khoá `readOnly:true` do CR-REQ-033 chốt. CR-REQ-033 chốt thành `accessMode:"readonly"` (cộng `workspaceKind`, `reportChanges`, `resultBlock`, `maxOutputBytes`). Tên khoá của solution 008 cần đổi theo; solution này là nơi chốt hợp đồng phía Go (mục 2.I). Không sửa file của solution 008.
5. Số migration `0039` đúng với ngày viết; `ls migrations/{postgres,mysql}` lại trước khi tạo file vì CR khác có thể chạm `infra-fleet-service` (CR-REQ-031 nếu thêm danh mục service).

## 2. Giải pháp

### 2.A Cây file

```
proto/orca/infrafleet/v1/infrafleet.proto                      (sửa)  GetDevServerCapabilities + 2 message
infra-fleet-service/
  migrations/{postgres,mysql}/0039_dev_server_capability_profiles.{up,down}.sql   (mới)
  internal/domain/capability_profile.go                        (mới)  CapabilityProfile, ProfileSource, Fingerprint()
  internal/domain/agent_features.go                            (mới)  hằng tên feature, FeatureSet
  internal/usecase/capability_ports.go                         (mới)  CapabilityProfileStore, Clock (nếu chưa có)
  internal/usecase/refresh_dev_server_capabilities.go          (mới)
  internal/usecase/get_dev_server_capabilities.go              (mới)
  internal/usecase/ports.go                                    (sửa)  HandshakeInfo thêm 4 trường
  internal/adapter/{postgres,mysql}/capability_profile_repository.go (mới)
  internal/adapter/devserveragent/{session.go,client.go}       (sửa)  trường mới, callback sau attach
  internal/adapter/agentwsserver/server.go                     (sửa)  inboundHandshakeParams
  internal/adapter/sshrelay/provisioner.go                     (sửa)  struct handshake
  internal/adapter/grpc/server_capability.go                   (mới)  handler GetDevServerCapabilities
  internal/config/config.go                                    (sửa)  INFRA_CAPABILITY_PROFILE_TTL, INFRA_CAPABILITY_REFRESH_MIN_INTERVAL
  cmd/server/main.go                                           (sửa)  wiring
request-service/ (mới)
  internal/usecase/capability_ports.go                         (mới)  DevServerCapabilityReader, CapabilityProfile
  internal/adapter/grpcclient/infra_capability_client.go       (mới)  gọi GetDevServerCapabilities, giải profile_json
  internal/adapter/grpcclient/agent_prompt_relay.go            (sửa, tạo ở TASK-REQ-008-02)  tham số và kết quả mới
```

### 2.B Proto (`infrafleet.proto`, chỉ thêm, `buf breaking` FILE vẫn xanh)

```proto
// Đúng một trong connection_id hoặc dev_server_id; cả hai hoặc không có thì InvalidArgument.
message GetDevServerCapabilitiesRequest {
  string connection_id = 1;
  string dev_server_id = 2;
  bool   refresh       = 3; // true: bỏ qua cache và TTL, gọi agent.capabilities ngay
}
message DevServerCapabilityProfile {
  string dev_server_id         = 1;
  string source                = 2; // "probe" | "handshake_only"
  string agent_build_version   = 3; // rỗng nếu agent cũ
  int32  protocol_version      = 4; // 1 khi vắng
  repeated string features     = 5;
  string profile_json          = 6; // JSON của kết quả agent.capabilities (schemaVersion 1), "{}" khi handshake_only
  bool   degraded              = 7;
  google.protobuf.Timestamp probed_at = 8;
  bool   connected             = 9; // false: trả hồ sơ lưu gần nhất, không probe
}
rpc GetDevServerCapabilities(GetDevServerCapabilitiesRequest) returns (DevServerCapabilityProfile);
```
`profile_json` là chuỗi để `infra-fleet-service` không phải theo từng thay đổi schema của agent (`schemaVersion` đi kèm trong JSON). Bên đọc tự giải.

### 2.C Migration `0039_dev_server_capability_profiles`

Postgres (schema `infra`):

```sql
CREATE TABLE infra.dev_server_capability_profiles (
    dev_server_id        UUID PRIMARY KEY REFERENCES infra.dev_servers(id) ON DELETE CASCADE,
    tenant_id            UUID NOT NULL,
    source               TEXT NOT NULL CHECK (source IN ('probe','handshake_only')),
    agent_build_version  TEXT NOT NULL DEFAULT '',
    protocol_version     INT  NOT NULL DEFAULT 1,
    features             JSONB NOT NULL DEFAULT '[]'::jsonb,
    profile              JSONB NOT NULL DEFAULT '{}'::jsonb,
    fingerprint          CHAR(64) NOT NULL,
    probed_at            TIMESTAMPTZ NOT NULL,
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_dev_server_capability_profiles_tenant ON infra.dev_server_capability_profiles (tenant_id, probed_at DESC);
ALTER TABLE infra.dev_server_capability_profiles ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON infra.dev_server_capability_profiles
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
```
MySQL: `CHAR(36)`, `VARCHAR(16)` cho `source` + `CONSTRAINT ... CHECK`, `JSON NOT NULL` **không DEFAULT literal** (ghi `'[]'`/`'{}'` từ ứng dụng), `TIMESTAMP(6)`, `FOREIGN KEY (dev_server_id) REFERENCES dev_servers(id) ON DELETE CASCADE`. Bảng `dev_servers` có sẵn PK `id` ở cả hai dialect; kiểm bằng `\d`/`SHOW CREATE TABLE` trước khi viết FK. `down`: `DROP TABLE`. Không FK sang service khác. `fingerprint` là SHA-256 của `profile` đã chuẩn tắc (khoá sắp xếp), **loại** `probedAt`, `host.memFreeMb`, `host.diskFreeMb`, `host.loadAvg1` (số đo biến động, nếu không thì sự kiện `capabilities_changed` bắn mỗi lần probe).

### 2.D Domain (`internal/domain/capability_profile.go`)

```go
type ProfileSource string // "probe" | "handshake_only"
type CapabilityProfile struct {
    DevServerID, TenantID string
    Source                ProfileSource
    AgentBuildVersion     string
    ProtocolVersion       int
    Features              []string // sắp xếp, không trùng
    ProfileJSON           []byte   // nguyên văn kết quả agent.capabilities; "{}" khi handshake_only
    Fingerprint           string
    ProbedAt              time.Time
}
func (p CapabilityProfile) Degraded() bool { return p.Source == ProfileSourceHandshakeOnly }
func (p CapabilityProfile) HasFeature(name string) bool
func ComputeFingerprint(profileJSON []byte) (string, error) // loại các khoá biến động, chuẩn tắc hoá
```
Hằng feature (`agent_features.go`): `agent.execPrompt`, `agent.execPrompt.readonly`, `agent.execPrompt.workspaceKind`, `agent.execPrompt.changes`, `agent.execPrompt.resultBlock`, `agent.capabilities`, `ai.complete`, `ai.complete.usage` (khớp CR-REQ-033 mục 2.6). Tên lạ vẫn được lưu (không từ chối), bên đọc chỉ hỏi `HasFeature`.

### 2.E Use case

`RefreshDevServerCapabilities.Execute(ctx, tenantID, devServerID string) (CapabilityProfile, error)`:
1. `ResolveConnectionByDevServer`/`repo.GetDevServer` lấy `domain.DevServer`; không có thì `INFRA_DEV_SERVER_NOT_FOUND`.
2. `agent.IsConnected(devServerID)` false thì trả hồ sơ lưu gần nhất (có thể không có, `ErrCapabilityProfileNotFound`) với `connected=false`; **không** dial ra ngoài chỉ để dò.
3. `result, err := agent.Exec(ctx, devServer, "agent.capabilities", map[string]any{"envNames": cfg.ProbeEnvNames})`. `ProbeEnvNames` mặc định rỗng (agent tự kiểm ba khoá API); tên env do `request-service` cần được truyền qua `GetDevServerCapabilities` ở phiên bản sau, không ở v1.
4. Thành công: `source=probe`, `features` và `protocol_version` lấy từ `result.agent` và từ `HandshakeInfo` (ưu tiên handshake nếu có), `ComputeFingerprint`; `UpsertProfile`; nếu `fingerprint` khác bản cũ thì ghi outbox `orca.infra.dev_server.capabilities_changed {dev_server_id, fingerprint, features}` (payload không chứa `profile`).
5. `errors.Is(err, domain.ErrAgentMethodNotFound)`: dựng hồ sơ `handshake_only` từ `LastHandshakeInfo` (platform, arch, nodeVersion, agentVersion, `Capabilities` cũ, `Features` nếu có), `degraded=true`; lưu (cũng so `fingerprint`).
6. Lỗi khác (timeout, relay) trả `INFRA_CAPABILITY_PROBE_FAILED` (`KindUnavailable`) nhưng nếu có hồ sơ cũ thì `GetDevServerCapabilities` vẫn trả hồ sơ cũ kèm cờ `stale` suy ra từ `probed_at`.

`GetDevServerCapabilities.Execute(ctx, in) (CapabilityProfile, connected bool, err error)`: `refresh=true` hoặc `now - probed_at > TTL` hoặc không có hồ sơ thì gọi Refresh khi đang kết nối; ngược lại trả hồ sơ lưu. Chống bão: mỗi `dev_server_id` tối đa một Refresh đang chạy (map khoá + `singleflight`), và không quá một lần mỗi `INFRA_CAPABILITY_REFRESH_MIN_INTERVAL` (mặc định 5 phút) trừ khi `refresh=true` do admin.

### 2.F `HandshakeInfo` mang `Capabilities`, `Features`, `ProtocolVersion`, `BuildVersion`

```go
// usecase/ports.go và devserveragent/session.go (cùng bốn trường mới)
Capabilities    []string `json:"capabilities"`
Features        []string `json:"features"`
ProtocolVersion int      `json:"protocolVersion"` // 0 khi vắng, coi là 1
BuildVersion    string   `json:"buildVersion"`
```
Quy tắc: không đổi `AgentVersion` (`ResumeAgentSession`, `MinAgentVersion` so sánh nó), không `DisallowUnknownFields` ở bất kỳ nơi giải mã handshake nào (đã grep: không có). `LastHandshakeInfo` chuyển đủ trường. Test hồi quy: handshake cũ (không có trường mới) vẫn thành công, `ProtocolVersion` suy ra 1.

### 2.G Kích hoạt và TTL

- Sau `attachTransport` thành công (cả ba đường) gọi callback `OnSessionAttached(devServerID)` đăng ký bằng option `WithOnSessionAttached(func(...))` ở `devserveragent.New`. Callback chạy `go` riêng với `context.WithTimeout(10s)`, bỏ qua lỗi (chỉ log), không chặn handshake (vẫn dưới race 5 giây của agent).
- `INFRA_CAPABILITY_PROFILE_TTL` mặc định `24h`; `INFRA_CAPABILITY_REFRESH_MIN_INTERVAL` mặc định `5m` (đề xuất, chưa đo).
- Không làm job nền quét toàn bộ dev server ở v1 (tránh tải); làm mới theo sự kiện kết nối và theo yêu cầu.

### 2.H Phía `request-service`: người dùng hồ sơ

Port (đặt ở `usecase`, adapter ở `grpcclient`):

```go
type DevServerCapabilityReader interface {
    Get(ctx context.Context, ref DevServerRef, refresh bool) (CapabilityProfile, error)
}
type DevServerRef struct{ ConnectionID, DevServerID string } // đúng một trường khác rỗng
type CapabilityProfile struct {
    Source string; Degraded bool; Connected bool
    AgentBuildVersion string; ProtocolVersion int; Features []string
    Platform string // từ profile.host.platform hoặc handshake
    Tools map[string]ToolStatus // id -> {Installed, Version, Known}; Known=false khi handshake_only
    ClaudeInstalled bool; ClaudeAuth string // "logged_in"|"logged_out"|"unknown"
    ClaudeFlags struct{ Tools, PermissionMode, DisallowedTools bool }
    EnvPresent map[string]bool
    ProbedAt time.Time
}
func (p CapabilityProfile) Has(feature string) bool
```
Giải `profile_json` chịu lỗi: trường thiếu thành `unknown`, không bao giờ `false` cho thứ không đo được (nguyên tắc R4 của feature impact-risk cũng áp dụng: "không biết" khác "không có"). `schemaVersion` lớn hơn bản hiểu được thì chỉ dùng `features` và đánh `Degraded`.

Bảng chọn đường (khớp CR-REQ-033 mục 2.7), cài ở `AgentPromptRunner` (SOL-008) và `ReadinessGate` (SOL-029):

| Tình huống | Hành vi phía `request-service` |
|---|---|
| `features` có `agent.execPrompt.readonly` | `agent_readonly` gửi `accessMode=readonly`, `workspaceKind=repo_root`, `reportChanges=true`; ba lớp bảo vệ của CR-REQ-008 vẫn chạy |
| Agent cũ (không có `features`) | v1 của CR-REQ-008: `enforcement=prompt_only`; `REQUEST_REQUIRE_ENFORCED_READONLY=true` thì `REQUEST_ANALYSIS_AGENT_TOO_OLD` |
| `agent.capabilities` trả -32601 (`handshake_only`) | `ReadinessGate` coi `tools`, `claude.auth`, `env.present` là `unverified`; dùng `agent.exec` `sh -c "command -v ..."` (Linux/macOS) hoặc `where.exe` (Windows) |
| Bundle build từ `desktop/` | như agent cũ |

### 2.I Client Go của `agent.execPrompt` (mở rộng `AgentPromptRunner` của TASK-REQ-008-02)

```go
type AgentPromptInput struct {
    StepID, Prompt, WorkPath string // WorkPath: worktree hoặc repo_path hoặc thư mục scratch
    TimeoutMS    int
    Env          map[string]string // chỉ hai khoá cố định ORCA_REQUEST_ID, ORCA_PROJECT_ID
    AccessMode   AccessMode        // AccessWrite (mặc định, bỏ khỏi JSON) | AccessReadonly
    Workspace    WorkspaceKind     // WorkspaceWorktree (mặc định) | WorkspaceRepoRoot | WorkspaceScratch
    ReportChanges bool
    ResultNonce  string            // ^[A-Za-z0-9]{16,64}$; rỗng thì không gửi resultBlock
    MaxOutputBytes int             // 0: để agent dùng mặc định 4 MiB; trần 12 MiB
}
type AgentPromptResult struct {
    Stdout, Stderr string; ExitCode int; TimedOut, Truncated bool
    Warnings []string       // "READONLY_VIOLATION", "TRUST_PRESET_IGNORED_READONLY"
    Changes  *AgentChanges  // nil nếu không yêu cầu
    Parsed   *ParsedResult  // nil nếu không gửi nonce
}
type AgentChanges struct{ Available bool; Reason string; HeadBefore, HeadAfter string; HeadMoved bool
    ChangedFiles []ChangedFile; Truncated bool }
type ParsedResult struct{ OK bool; Value json.RawMessage; Code, Detail string } // Code: RESULT_BLOCK_MISSING|INVALID_JSON|TOO_LARGE|NOT_OBJECT
```
`params_json` dựng bằng struct có `omitempty` để agent cũ không nhận khoá lạ khi giá trị mặc định. Mã lỗi agent `READONLY_MODE_UNSUPPORTED`, `REPO_ROOT_REQUIRES_READONLY`, `WORKSPACE_*` (JSON-RPC `InvalidParams`, `error.data.reason`) được dịch sang lỗi có kiểu `ErrAgentReadonlyUnsupported`, `ErrAgentWorkspaceRejected`; `-32601` thành `ErrAgentMethodNotFound` (không bao giờ hạ cấp âm thầm sang chế độ ghi). `trustPreset` vẫn là hằng `"default"`; với `AccessReadonly` adapter **không gửi** `trustPreset` và test khẳng định không có `"full"` trong mọi tổ hợp.

### 2.J Sự kiện, mã lỗi, phân quyền

- Sự kiện: `orca.infra.dev_server.capabilities_changed` (outbox của `infra-fleet-service`, bảng `infra.outbox_events` có sẵn; ghi trong cùng giao dịch với `UpsertProfile` ở Postgres bằng `EnqueueOutboxEvent`-kiểu, MySQL tương tự). Không có consumer ở v1 (chỉ để quan sát và để UI làm mới sau).
- Lỗi: `INFRA_CAPABILITY_BAD_REQUEST` (`InvalidArgument`: cả hai hoặc không có id), `INFRA_DEV_SERVER_NOT_FOUND` (`NotFound`), `INFRA_CAPABILITY_PROBE_FAILED` (`Unavailable`), `INFRA_CAPABILITY_PROFILE_NOT_FOUND` (`NotFound` khi không kết nối và chưa có hồ sơ).
- Quyền: cùng phạm vi tenant như `ResolveConnection` (`tenant.RequireTenantID`; truy vấn luôn lọc `tenant_id`). Không cần `requireAdmin`: hồ sơ không chứa bí mật (biến môi trường chỉ có `present`, đã bảo đảm ở agent). `refresh=true` giới hạn tần suất như 2.E.

### 2.K Bảng giao diện với phần agent (khớp CR-REQ-033, không đổi tên)

| Mục | Method / trường | Phía agent soạn ở | Phía backend dùng ở |
|---|---|---|---|
| Tham số `agent.execPrompt` | `accessMode`, `workspaceKind`, `reportChanges`, `resultBlock{nonce}`, `maxOutputBytes` | `AG-REQ-SOL-033-*` | mục 2.I; `task-service/simple_executor.go` (SOL-029) |
| Kết quả thêm | `changes{available,reason,headBefore,headAfter,headMoved,changedFiles[],truncated}`, `parsed{ok,value\|code,detail}`, `warnings[]`, `truncated` | `AG-REQ-SOL-033-*` | 2.I, SOL-029 (`task_execution_records`), SOL-008 |
| Mã lỗi agent | `READONLY_MODE_UNSUPPORTED`, `REPO_ROOT_REQUIRES_READONLY`, mã `WORKSPACE_*` | `AG-REQ-SOL-033-*` | 2.I |
| RPC `agent.capabilities` | params `{tools?, envNames?, refresh?}`; kết quả `schemaVersion=1` (`agent`, `host`, `tools[]`, `claude`, `env[]`, `probedAt`, `partial`) | `AG-REQ-SOL-033-*` | 2.E, 2.H |
| Handshake | `protocolVersion`, `buildVersion`, `features[]` | `AG-REQ-SOL-033-*` | 2.F |
| `AGENT_VERSION` | tăng lên `2.2.0` (đề xuất) để `sshrelay` đẩy bundle mới | `AG-REQ-SOL-033-*` | `sshrelay/version_check.go` không đổi mã |

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| `profile_json` giữ nguyên văn, không tách cột công cụ | Schema agent đổi nhanh; chỉ `features`, `protocol_version`, `source` cần truy vấn |
| `fingerprint` loại số đo biến động | Tránh phát sự kiện mỗi lần probe |
| Không probe khi dev server không kết nối | Dial chỉ để dò làm chậm và tốn SSH; trả hồ sơ cũ kèm `connected=false` |
| Refresh theo sự kiện + yêu cầu, không quét nền | Đơn giản, không tăng tải; TTL 24h bù |
| Hồ sơ `handshake_only` mang `Known=false`, không `false` | "Không biết" khác "không có", tránh `env_defect` oan |
| `features` tách khỏi `capabilities` | `ptyReady` và các kiểm tra cũ không đổi |
| Không sửa mã của `agent/` ở solution này | Agent khác soạn; chỉ khoá hợp đồng ở 2.K |

## 4. Phụ thuộc và thứ tự

Không CR nào chặn. Thứ tự giao: agent trước (build `agent/out/agent.js`, tăng `AGENT_VERSION`), rồi task 01 đến 03 của solution này, rồi 04 và 05 (cần `request-service` có module: TASK-REQ-001-01 đến 05), cuối cùng 06. Backend chỉ gọi method mới khi `features` có, nên không có cửa sổ lỗi khi dev server nâng cấp lệch. Mở khoá: SOL-029 (`ReadinessGate` môi trường), SOL-008 (đường `agent_readonly` ép được), SOL-005 (báo thiếu khoá API sớm).

## 5. Kiểm thử

- **Unit:** `ComputeFingerprint` (khoá biến động không đổi fingerprint, thứ tự khoá không đổi); `RefreshDevServerCapabilities` với `DevServerAgentClient` giả (thành công; `ErrAgentMethodNotFound`; timeout; fingerprint không đổi thì không phát sự kiện; đổi thì phát một sự kiện); `GetDevServerCapabilities` (TTL, `refresh`, khoá chống bão, không kết nối); giải `profile_json` ở `request-service` (thiếu trường, `schemaVersion` lạ); `AgentPromptInput` sang JSON (omitempty, không có `trustPreset=full`).
- **Integration hai dialect (`-tags=integration`):** repository upsert, `ON DELETE CASCADE`, RLS Postgres bằng role không phải superuser, JSON Unicode, up/down sạch.
- **Hợp đồng:** `buf lint`, `buf breaking` cho `infrafleet.proto`; JSON golden của `agent.capabilities` dùng chung với agent (xem TASK-REQ-033-06); handshake cũ vẫn thành công.
- **Thủ công, có dev server (chưa kiểm chứng):** agent cũ cạnh agent mới; xác nhận `handshake_only`, rồi `probe` sau khi nâng cấp; đo độ trễ `agent.capabilities` (SSH và relay-websocket).
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/infra-fleet-service/...` và `go test -tags=integration ./services/infra-fleet-service/internal/adapter/...`.

## 6. Rủi ro và điểm chưa kiểm chứng

- Callback sau attach chạy khi agent vẫn đang khởi động: `agent.capabilities` có thể chậm (8 giây theo CR). Callback có timeout và không chặn, nhưng hồ sơ đầu tiên có thể trễ vài giây.
- `HandshakeInfo` có ba nơi dựng (`agentwsserver`, `sshrelay`, `session.go` initiator); sót một nơi thì hồ sơ thiếu `features` ở đúng đường kết nối đó. Cần test cho từng đường.
- `fingerprint` loại khoá biến động theo danh sách cứng; agent thêm số đo mới thì phải cập nhật danh sách (test golden bắt).
- Hành vi Claude CLI (`--tools`, `--permission-mode plan`) chưa kiểm chứng ở CR gốc; `request-service` không được coi `features` có `readonly` là bằng chứng ép được (cần thử nghiệm đối kháng trước khi bật `REQUEST_REQUIRE_ENFORCED_READONLY`).
- `go.work` và nhiều tham số vị trí của `grpc.New(...)`: thêm một use case làm đổi mọi nơi gọi, kể cả test `server_test.go`; dùng `gitnexus_impact` trên `New` trước khi sửa.

## 7. Câu hỏi mở

- **Q1.** Có cần RPC truyền `envNames` từ `request-service` tới agent (để `ReadinessGate` hỏi đúng tên biến của task)? v1 chỉ lấy ba khoá API mặc định; `requires.env_names` bất kỳ cần `agent.capabilities` với `envNames` (mở rộng `GetDevServerCapabilitiesRequest.env_names` là thay đổi thêm, tương thích).
- **Q2.** TTL 24 giờ và khoảng tối thiểu 5 phút là đề xuất, chưa đo.
- **Q3.** Cổng sẵn sàng cần `cwd` của worktree trước khi `task-service` gọi `EnsureWorktree`. Đã có `project-service.GetWorktree`; còn lại vấn đề task đầu chưa có worktree, SOL-029 mục 1 (điều 4) chốt: dùng `repo_path` cho kiểm tra ngữ nghĩa và bỏ Check nền.
- **Q4.** Tên khoá `readOnly` trong SOL-008 cần đổi thành `accessMode` theo CR-REQ-033; ai sửa (người điều phối).
- **Q5.** Có cần cache hồ sơ ở `request-service` (TTL ngắn) để cổng không gọi gRPC mỗi lần thử? Đề xuất không ở v1.

## 8. Tham chiếu

- `/opt/repos/orca/backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (dòng 13, 86, 102, 276, 562)
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/usecase/ports.go` (dòng 85, 296, 411), `get_host_capabilities.go`, `authorization.go`
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/adapter/devserveragent/session.go` (dòng 41 đến 75, 245, 267), `client.go` (dòng 149, 158, 301, 323, 473)
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/server.go` (dòng 68 đến 81, 212), `adapter/sshrelay/provisioner.go` (dòng 64, 255), `internal/adapter/grpc/server.go` (dòng 156), `internal/config/config.go`, `cmd/server/main.go` (dòng 316 đến 333)
- `/opt/repos/orca/backend-go/services/infra-fleet-service/migrations/postgres/0030_dev_server_approval_status_and_groups.up.sql`, `0038_session_origin.up.sql`, bản `mysql`
- `/opt/repos/orca/agent/src/relay/agent-rpc-dispatch-agent-exec.ts` (dòng 66 đến 140), `agent-git-handler.ts` (dòng 41 đến 64, 143 đến 230), `fs-agent-extensions.ts`
- `/opt/repos/orca/docs/crs/v6/agent-capabilities/CR-REQ-033-agent-readonly-worktree-and-capability-report.md`, `README.md`; `docs/crs/v6/execution-contract/CR-REQ-029-execution-contract-and-readiness-gate.md` (mục 2.4)
- `/opt/repos/orca/specs/backend-go/crs/v6/solution-analysis/solutions/BE-REQ-SOL-008-diagnosis-findings-answer-analysis.md` (mục C, G), `tasks/TASK-REQ-008-02-agent-prompt-relay-and-repo-probe.md`
- `/opt/repos/orca/guides/reference/git-compatibility.md`, `/opt/repos/orca/AGENTS.md`

# TASK-REQ-033-04: `request-service` đọc hồ sơ năng lực (`DevServerCapabilityReader`) và bảng chọn đường theo `features`

**From Solution:** [BE-REQ-SOL-033](../solutions/BE-REQ-SOL-033-dev-server-capability-profile-and-agent-client.md) mục 2.H
**Priority:** P1
**Service/Area:** `request-service` (mới) / usecase port, domain, adapter grpcclient
**File:** `internal/domain/dev_server_capability.go` (mới), `internal/usecase/capability_ports.go` (mới), `internal/adapter/grpcclient/infra_capability_client.go` (mới), `internal/domain/agent_route_selection.go` (mới), và các `_test.go`; config `INFRA_FLEET_SERVICE_ADDR` (đã thêm ở TASK-REQ-005-02, kiểm lại)
**Depends on:** TASK-REQ-033-03 (RPC có thật), TASK-REQ-001-01 (module `request-service`), TASK-REQ-001-05 (main, config), TASK-REQ-007-04 (`connection_resolver.go`, `withTenantMetadata`)
**Status:** [ ] TODO

> Để đợt R2 (2026-10-07): task nằm ở `request-service`, ngoài phạm vi đợt I1. Phía server đã sẵn: `GetDevServerCapabilities` phục vụ thật; `profile_json` có `schemaVersion`, `agent{buildVersion,protocolVersion}`, `host`, `tools[]`, `claude{auth,flags}`, `env[{name,present}]`, `partial` (đúng tên trường của agent), `features` là cột riêng, `degraded` cho hồ sơ `handshake_only`. Client cần gắn metadata tenant (infra-fleet đọc tenant từ context gRPC).

---

## Context

- `request-service` chưa có thư mục (`ls backend-go/services/request-service` báo không tồn tại ngày 2026-10-06); đường dẫn ở trên đều "(mới)". Layout theo TASK-REQ-001-01: `internal/{domain,usecase,adapter/grpcclient}`.
- Client gRPC tới `infra-fleet-service` đã được dựng cho `Relay`/`RelayByDevServer`/`ResolveConnection` ở TASK-REQ-005-02 và TASK-REQ-007-04 (file `connection_resolver.go`, hàm `withTenantMetadata` chuyển `tenant_id` vào metadata gRPC). Task này **dùng lại** cùng `infrafleetv1.InfraFleetServiceClient`, không mở kết nối mới.
- Bản đồ người dùng: SOL-029 `ReadinessGate` tầng môi trường (`requires.tools`, `requires.env_names`, `claude.auth=logged_in`), SOL-008 `RunAgentReadonlyAnalysis` (chọn đường `agent_readonly` theo `features`), SOL-005 (báo "thiếu khoá API" sớm thay cho `REQUEST_AI_NO_PROVIDER`).
- Schema `profile_json` do agent định nghĩa (CR-REQ-033 mục 2.6, `schemaVersion: 1`): `agent{buildVersion,protocolVersion}`, `host{platform,arch,nodeVersion,cpuCount,memTotalMb,memFreeMb,diskFreeMb,loadAvg1}`, `tools[{id,installed,version?}]`, `claude{installed,version,auth,flags{tools,permissionMode,disallowedTools}}`, `env[{name,present}]`, `probedAt`, `partial`. Tên trường `loggedIn` của `claude auth status --json` **chưa kiểm chứng** ở CR; `auth` đã được agent chuẩn hoá thành `logged_in|logged_out|unknown`.
- Nguyên tắc: "không biết" khác "không có". Hồ sơ `handshake_only` không có `tools`: mọi tra cứu công cụ trả `Known=false`, tuyệt đối không `Installed=false`.

## Việc cần làm

1. `internal/domain/dev_server_capability.go` (thuần, không import adapter):
   ```go
   type ToolStatus struct{ Known, Installed bool; Version string }
   type ClaudeFlags struct{ Tools, PermissionMode, DisallowedTools bool }
   type DevServerCapability struct {
       Source string; Degraded, Connected, Partial bool
       AgentBuildVersion string; ProtocolVersion int; Features []string
       Platform string; Tools map[string]ToolStatus
       ClaudeInstalled, ClaudeKnown bool; ClaudeAuth string; ClaudeFlags ClaudeFlags
       EnvPresent map[string]bool; EnvKnown bool
       ProbedAt time.Time
   }
   func (c DevServerCapability) HasFeature(name string) bool
   func (c DevServerCapability) Tool(id string) ToolStatus   // Known=false khi không có trong Tools
   func (c DevServerCapability) EnvVar(name string) (present, known bool)
   func ParseCapabilityProfile(source string, degraded, connected bool, buildVersion string, protocolVersion int, features []string, profileJSON []byte, probedAt time.Time) (DevServerCapability, error)
   ```
   `ParseCapabilityProfile` chịu lỗi: JSON rỗng hoặc `{}` cho hồ sơ chỉ có `Features`; `schemaVersion` lớn hơn 1 thì bỏ qua `Tools/Claude/Env`, đặt `Degraded=true`; trường sai kiểu thì bỏ trường đó, không trả lỗi (chỉ trả lỗi khi JSON hỏng cú pháp).
2. Port ở `usecase/capability_ports.go`: `DevServerCapabilityReader.Get(ctx, ref DevServerRef, refresh bool) (domain.DevServerCapability, error)` với `DevServerRef{ConnectionID, DevServerID string}`; lỗi có kiểu `ErrCapabilityNotAvailable` (dev server không kết nối và chưa có hồ sơ) để use case phân biệt với lỗi mạng.
3. `infra_capability_client.go`: `type InfraCapabilityClient struct{ client infrafleetv1.InfraFleetServiceClient; timeout time.Duration }`:
   - `Get` gọi `GetDevServerCapabilities` với `withTenantMetadata(ctx)`, timeout 15 giây (agent probe tối đa 8 giây), dịch `codes.NotFound` với code `INFRA_CAPABILITY_PROFILE_NOT_FOUND` sang `ErrCapabilityNotAvailable`
   - mọi lỗi khác bọc `fmt.Errorf("infra capability: %w", err)`.
4. `internal/domain/agent_route_selection.go`: hàm thuần `SelectReadonlyRoute(c DevServerCapability, requireEnforced bool) (ReadonlyRoute, error)`:
   - `c.HasFeature("agent.execPrompt.readonly")` thì `RouteAgentEnforced` (gửi `accessMode=readonly`, `workspaceKind=repo_root`, `reportChanges=true`);
   - ngược lại nếu `requireEnforced` thì `ErrAgentTooOld` (usecase dịch thành `REQUEST_ANALYSIS_AGENT_TOO_OLD`);
   - ngược lại `RoutePromptOnly` (hành vi v1 của CR-REQ-008, `enforcement=prompt_only`);
   - `ClaudeKnown && !(ClaudeFlags.Tools && ClaudeFlags.PermissionMode)` với route enforced thì hạ xuống `RoutePromptOnly` hoặc lỗi theo `requireEnforced` (không gửi tham số mà agent sẽ từ chối).
5. Hàm phụ `MissingTools(c DevServerCapability, required []string) (missing, unverified []string)`: `missing` = công cụ `Known && !Installed`; `unverified` = `!Known`. Đây là điều kiện đầu vào cho `ReadinessGate` (SOL-029): `missing` thì `env_defect`, `unverified` thì dùng đường dự phòng (SOL-029 task 06).
6. Hàm phụ `MissingEnv(c, names []string) (missing, unverified []string)` cùng quy tắc; chỉ trả tên biến, không bao giờ giá trị.
7. Cấu hình: `REQUEST_REQUIRE_ENFORCED_READONLY` (mặc định `false`) vào `config.go` nếu SOL-008 chưa thêm (kiểm TASK-REQ-008-03 trước).
8. Không cache ở v1 (SOL-033 Q5); mỗi lần gọi một RPC.

## Kiểm thử

- `TestParseCapabilityProfile_FullProbe`, `_HandshakeOnlyUnknownTools`, `_UnknownSchemaVersionDegrades`, `_WrongTypedFieldIgnored`, `_BrokenJSONErrors`, `_EmptyJSON`.
- `TestTool_UnknownIsNotMissing`, `TestMissingTools_Table` (đã cài, chưa cài, không biết), `TestMissingEnv_NeverReturnsValue`.
- `TestSelectReadonlyRoute_Table` đủ tổ hợp (feature có/không, `requireEnforced`, cờ Claude `false`, `ClaudeKnown=false`).
- `TestInfraCapabilityClient_NotFoundMapsToNotAvailable`, `_AddsTenantMetadata`, `_TimeoutBounded` bằng client fake (`infrafleetv1.InfraFleetServiceClient` giả như ở TASK-REQ-007-04).
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/domain/... ./services/request-service/internal/adapter/grpcclient/... -run "Capability|Route|MissingTools|MissingEnv"`.

## Tiêu chí hoàn thành

- [ ] Hồ sơ `handshake_only` không bao giờ sinh `missing` (chỉ `unverified`).
- [ ] `SelectReadonlyRoute` chọn đúng bốn đường của bảng ở solution 2.H.
- [ ] Client gửi `tenant_id` trong metadata; không mở kết nối thứ hai tới `infra-fleet-service`.
- [ ] Không import chéo `internal/` của service khác; không file tên `helpers`/`utils`/`common`/`misc`.
- [ ] Không log giá trị nào ngoài tên biến và `present`.

## Rủi ro và lưu ý

- Nếu SOL-008 task 02 (`agent_prompt_relay.go`) đã cài khoá `readOnly:true`, đổi thành `accessMode` ở task 05 chứ không ở đây.
- `Tool(id)` dựa trên `id` do agent đặt (danh sách cứng: `go node pnpm npm git openspec claude codegraph gitnexus rg make semgrep`); `requires.tools` của TaskSpec nằm ngoài danh sách này sẽ luôn `unverified`, nên cổng cần đường dự phòng cho công cụ lạ (quyết định ở SOL-029 task 06).
- Gọi RPC mỗi lần cổng chạy thêm một vòng mạng; chưa đo.

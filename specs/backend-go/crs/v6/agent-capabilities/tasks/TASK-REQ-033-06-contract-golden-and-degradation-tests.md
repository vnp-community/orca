# TASK-REQ-033-06: Test hợp đồng với agent (golden JSON) và kịch bản suy giảm agent cũ/mới

**From Solution:** [BE-REQ-SOL-033](../solutions/BE-REQ-SOL-033-dev-server-capability-profile-and-agent-client.md) mục 2.K, 5
**Priority:** P1
**Service/Area:** `infra-fleet-service`, `request-service` / test hợp đồng và tích hợp
**File:** `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/testdata/agent_capabilities_v1.golden.json` (mới), `backend-go/services/request-service/internal/adapter/grpcclient/testdata/{agent_capabilities_v1.golden.json,execprompt_params.golden.json,execprompt_result_full.golden.json}` (mới), `internal/usecase/capability_degradation_test.go` (mới, infra-fleet), `internal/domain/dev_server_capability_contract_test.go` (mới, request-service), `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/capability_integration_test.go` (mới, `-tags=integration`)
**Depends on:** TASK-REQ-033-03, TASK-REQ-033-04, TASK-REQ-033-05; bản agent (khu vực `agent`, `AG-REQ-SOL-033-*`) cung cấp file golden TypeScript tương ứng
**Status:** [ ] TODO

## Tiến độ (2026-10-07)

Phần `infra-fleet-service` đã làm và kiểm chứng; phần `request-service` để đợt R2 (cần 033-04, 033-05).

- Xong: golden `internal/adapter/devserveragent/testdata/agent_capabilities_v1.golden.json` (bản sao nguyên văn của `agent/src/relay/__fixtures__/agent-capabilities-golden.json`, có hai biến thể `full` và `partial` trong một file; không tạo file `unknown_schema` riêng, hồ sơ lạ schema do request-service suy giảm); test `capability_golden_test.go` (tên trường, fingerprint bỏ qua trường biến động, hình dạng JSON handshake của agent); `capability_degradation_test.go` (bốn kịch bản + báo cáo `partial`, khẳng định cả sự kiện không phát khi fingerprint không đổi); script `backend-go/ci/check-agent-capability-golden.sh` + target `make check-agent-capability-golden` (đã thử sửa một ký tự: script thoát 1); `.gitattributes` ép LF.
- Còn thiếu (R2): ba golden và `capability_degradation`/contract test của `request-service` (bước 2, 3, 6), `capability_integration_test.go` `-tags=integration` ghép hai đầu; script đã sẵn để so thêm bản sao của request-service (hiện báo "skip" nếu chưa có). CI chưa gọi script (không có workflow backend-go): người điều phối cần chốt chỗ chạy.
- Tên file golden lệch task (task gọi `agent_capabilities_v1.golden.json` + hai biến thể; agent đã có một fixture hai khoá): giữ fixture của agent làm nguồn để không có hai hợp đồng.


---

## Context

- Phía agent do agent khác soạn ở `specs/agent/crs/v6/agent-capabilities/`; CR-REQ-033 mục 5 đòi "test golden JSON của `agent.capabilities` dùng chung giữa agent (TypeScript) và decoder Go". Hai bên không import nhau, nên hợp đồng được khoá bằng **một bản golden được copy nguyên văn vào hai repo con** (agent test đọc bản của nó, Go đọc bản của nó) và một test kiểm tra hai bản giống hệt nhau do người điều phối chạy ở CI (hoặc script so sánh `sha256sum`).
- Hiện không có CI chạy chéo agent (`pnpm`) và Go; script so sánh là phương án nhỏ nhất. Chưa kiểm chứng workflow CI nào gọi được cả hai; ghi vào Rủi ro.
- Kịch bản suy giảm của CR mục 2.7: (a) agent mới có `features`; (b) agent cũ không có `features` nhưng có `agent.capabilities`? (không: agent cũ không có method này, trả -32601); (c) agent cũ -32601 thì `handshake_only`; (d) bundle `desktop/` coi như agent cũ.
- Bộ test integration của `devserveragent` dùng transport giả (`Transport` interface, `AttachTransport`), mẫu `last_handshake_info_test.go`.
- `GetHostCapabilities` (`usecase/get_host_capabilities.go`) là tiền lệ cho -32601 thành `FailedPrecondition`; hồ sơ năng lực khác: -32601 thành hồ sơ `handshake_only` (không lỗi).

## Việc cần làm

1. Tạo `agent_capabilities_v1.golden.json` (nội dung đúng ví dụ CR-REQ-033 mục 2.6): `schemaVersion:1`, `agent{buildVersion:"2.2.0",protocolVersion:2}`, `host{platform:"linux",arch:"x64",nodeVersion:"v22.x",cpuCount:8,memTotalMb,memFreeMb,diskFreeMb,loadAvg1}`, `tools[{id:"go",installed:true,version:"1.22.3"},{id:"openspec",installed:false}]`, `claude{installed:true,version,auth:"logged_in",flags{tools:true,permissionMode:true,disallowedTools:true}}`, `env[{name:"ANTHROPIC_API_KEY",present:true}]`, `probedAt`, `partial:false`. Thêm biến thể `agent_capabilities_v1_partial.golden.json` (`partial:true`, thiếu `claude`) và `..._unknown_schema.golden.json` (`schemaVersion:2`).
2. Test hợp đồng request-service: `TestParseCapabilityProfile_Golden` giải bản golden bằng `ParseCapabilityProfile` (TASK-REQ-033-04) và khẳng định từng trường (`Tool("go").Installed`, `ClaudeAuth`, `EnvVar("ANTHROPIC_API_KEY")`, `Partial`); `_GoldenPartial`; `_GoldenUnknownSchemaDegrades`.
3. Test hợp đồng `execPrompt`: `TestBuildParams_GoldenReadonly` so JSON sinh ra (khoá sắp xếp) với `execprompt_params.golden.json` ứng với input {readonly, repo_root, reportChanges, nonce cố định, maxOutputBytes}; `TestExecPrompt_ParsesFullResultGolden` giải `execprompt_result_full.golden.json` gồm `stdout`, `exitCode:0`, `warnings:[]`, `changes{available:true,headBefore,headAfter,headMoved:false,changedFiles:[{path,change:"modified"}],truncated:false}`, `parsed{ok:true,value:{...}}`.
4. Script so sánh: `backend-go/ci/` hoặc `scripts/` (tên cụ thể, ví dụ `check-agent-capability-golden.sh`, không `helpers`): so `sha256` của golden ở `agent/` (đường dẫn do task agent chốt) với bản ở `request-service` và `infra-fleet-service`; thoát khác 0 nếu lệch. Thêm vào `backend-go/Makefile` một target riêng (không đụng `proto-lint` đang bị `|| true`).
5. Kịch bản suy giảm cho `infra-fleet-service` (`capability_degradation_test.go`, dùng fake `DevServerAgentClient`):
   - `TestDegradation_NewAgent_ProbeStoresFeatures` (Exec trả golden, `HandshakeInfo.Features` có `agent.execPrompt.readonly`): `source=probe`, `degraded=false`.
   - `TestDegradation_OldAgent_MethodNotFound`: `Exec` trả `domain.ErrAgentMethodNotFound`; `source=handshake_only`, `degraded=true`, `features` rỗng, `platform` từ handshake.
   - `TestDegradation_OldAgentThenUpgrade_FingerprintChangesOnce`: lần một `handshake_only`, lần hai `probe`; đúng một sự kiện `capabilities_changed` cho mỗi chuyển.
   - `TestDegradation_DesktopBundleTreatedAsOld`: handshake không có `features` và -32601 cho mọi method mới.
6. Test tích hợp `request-service` ghép hai đầu: `TestSelectReadonlyRoute_FromGoldenProfile` (golden mới thì `RouteAgentEnforced`; hồ sơ `handshake_only` thì `RoutePromptOnly`, và `requireEnforced=true` thì `ErrAgentTooOld`).
7. (Chưa chạy được nếu không có dev server) mục thủ công: ghi danh sách kiểm vào PR: agent cũ cạnh agent mới trên một dev server thật; `GetDevServerCapabilities` trả đúng cả hai; đo độ trễ `agent.capabilities` qua SSH và relay-websocket.

## Kiểm thử

- Lệnh Go: `cd /opt/repos/orca/backend-go && go test ./services/infra-fleet-service/internal/usecase/... -run Degradation`, `go test ./services/request-service/internal/domain/... ./services/request-service/internal/adapter/grpcclient/... -run "Golden|Route"`, và `go test -tags=integration ./services/infra-fleet-service/internal/adapter/devserveragent/...`.
- Script: `bash scripts/check-agent-capability-golden.sh` (đường dẫn thật theo quyết định ở bước 4) chạy ở CI nhẹ.
- Bên agent chạy `cd agent && pnpm test` với golden tương ứng (thuộc task agent, chỉ tham chiếu).

## Tiêu chí hoàn thành

- [ ] Ba golden `agent_capabilities` giống hệt (hash) giữa agent, `request-service` và `infra-fleet-service`.
- [x] Bốn kịch bản suy giảm xanh; chuyển từ `handshake_only` sang `probe` phát đúng một sự kiện. (TestDegradation_* trong usecase)
- [ ] `execprompt_params.golden.json` khoá tên tham số khớp CR-REQ-033 mục 2.1 (`accessMode`, `workspaceKind`, `reportChanges`, `resultBlock`, `maxOutputBytes`).
- [x] Không test nào gọi mạng ngoài hoặc dev server thật. (phần infra-fleet)
- [x] Không file nào tên `helpers`, `utils`, `common`, `misc`.

## Thứ tự thực hiện gợi ý

1. Chốt đường dẫn golden bên agent với người soạn `AG-REQ-SOL-033-*` (nơi đặt tệp, tên tệp) trước khi tạo bản Go; ghi đường dẫn vào mô tả PR.
2. Viết ba tệp golden `agent_capabilities` trước, rồi test giải của `request-service` (bước 2); test đỏ đầu tiên giúp phát hiện sớm tên trường lệch.
3. Viết golden `execprompt_params`/`execprompt_result_full` sau khi TASK-REQ-033-05 có `buildParams` ổn định; chạy lại test với `-update` **một lần**, đọc diff bằng mắt rồi mới commit.
4. Viết kịch bản suy giảm (bước 5) với fake có sẵn từ TASK-REQ-033-03; không thêm fake mới nếu fake cũ đủ dùng.
5. Thêm script so hash và target Makefile cuối cùng; chạy thử bằng cách sửa tay một ký tự trong một bản golden và xác nhận script thoát khác 0.

## Kiểm tra thủ công sau khi xong (chưa kiểm chứng, cần dev server thật)

- Dev server chạy agent cũ: gọi `GetDevServerCapabilities` qua `grpcurl`, kỳ vọng `source=handshake_only`, `degraded=true`, `features` rỗng.
- Nâng agent (build `agent/out/agent.js`, tăng `AGENT_VERSION`, `scp`, `systemctl restart orca-agent`): gọi lại với `refresh=true`, kỳ vọng `source=probe` và một sự kiện `capabilities_changed` trong NATS.
- Đo độ trễ `agent.capabilities` qua SSH và relay-websocket; ghi số vào PR (CR chỉ ước 8 giây tối đa).

## Điểm cần người review soi kỹ

- Mọi golden có đúng một dòng cuối là xuống dòng và dùng LF (không CRLF): script so hash sẽ lệch trên Windows nếu Git tự đổi `autocrlf`; thêm `*.golden.json text eol=lf` vào `.gitattributes` của thư mục nếu cần.
- Test không được phụ thuộc đồng hồ thật (`time.Now`) hoặc thứ tự map; dùng `Clock` giả và so sánh JSON đã chuẩn tắc.
- Mỗi kịch bản suy giảm khẳng định cả **sự kiện không phát** (khi fingerprint không đổi) chứ không chỉ kết quả trả về.

## Rủi ro và lưu ý

- Copy golden là cách thủ công; nếu agent đổi trường mà không cập nhật bản Go thì chỉ script so sánh phát hiện. Nếu CI không chạy script, hợp đồng thực tế chưa được bảo vệ: người điều phối cần chốt chỗ chạy.
- Golden cố tình dùng số đo biến động (`memFreeMb`) để kiểm `ComputeFingerprint` bỏ qua chúng; đừng chuẩn hoá chúng trong file.
- Test thủ công trên dev server thật chưa chạy được ở thời điểm soạn; không ghi "đã kiểm chứng" cho tới khi chạy.

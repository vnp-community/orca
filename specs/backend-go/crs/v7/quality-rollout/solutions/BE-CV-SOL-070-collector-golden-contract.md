# BE-CV-SOL-070: Tệp vàng kết quả agent và kiểm thử hợp đồng phía `code-intel-service`

> 📋 Proposed. Chưa triển khai, chưa chạy. Viết ngày 2026-10-06 từ việc đọc CR-CV-070, ba hợp đồng v7 và code hiện có; `code-intel-service` và `proto/orca/codeintel/` **chưa tồn tại** (đã kiểm: `backend-go/services/` không có `code-intel-service`, `backend-go/proto/orca/` không có `codeintel`).

**CR:** [CR-CV-070](../../../../../../docs/crs/v7/quality-rollout/CR-CV-070-golden-fixtures-and-tool-contract-tests.md)
**Service:** `code-intel-service` (mới: `testdata/agent-results/`, test collector), `.github/workflows/code-intel-contract.yml` (mới)
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (mục "Testing implications": test `usecase/` với fake cổng, test `adapter/` tích hợp), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (mục "Talking to the Dev Server Agent": giữ giao thức agent hiện có, Option A), [`arch/10`](../../../../tdd/architecture/10-deployment-infrastructure.md) (mục CI/CD: CI theo module, PR chạy unit + integration)
**Task:** [`../tasks/README.md`](../tasks/README.md)

---

## 1. Hợp đồng áp dụng

| Nguồn | Mục / PQ | Dùng để |
|---|---|---|
| `CONTRACT-codeintel-agent-rpc.md` | §2.2 phong bì (`sources, headCommit, stale, truncated, totalCount, warnings, perf, data`), §2.6 `SymbolRef`, §3.2 mã lỗi agent, §3.4 ánh xạ `data.code` → `apperrors.Kind` và trailer `x-orca-agent-error-data-bin`, §4 bộ method | hình dạng tệp vàng; bảng method cần có tệp vàng; ca lỗi |
| `CONTRACT-codeintel-proto-and-data-map.md` | PQ-02 (bốn chặng mã lỗi), PQ-03 (tập mã lỗi), PQ-12 (phong bì phẳng + `etag`), PQ-14 (trần 2 MiB, `MaxCallRecvMsgSize` 16 MiB), PQ-17, PQ-19, **PQ-20** (agent chuẩn hoá `SymbolRef`, backend không cộng dòng khi `lineBase===1`, cùng hàm va chạm khoá `#<arity>` rồi `#L<startLine>`, cùng vector), PQ-21, §7.1 cổng **G1**, §8.2 (tên solution), §8.3 (kiểm chéo), §10 | quy tắc chuẩn hoá cần khoá bằng test; vị trí G1 |
| `CONTRACT-codeintel-ui-api.md` | §2.3 bảng lỗi tới client, §4.1 `SymbolRef` | khẳng định `SymbolRef.key` sau chuẩn hoá khớp kiểu UI |

Cổng **G1** (hợp đồng §7.1): "Tệp vàng agent (CR-070 `testdata/agent-results/*.json` + `mini-repo`) theo `CONTRACT-codeintel-agent-rpc.md`" mở khoá collector không cần dev server thật và fake backend của frontend. Solution này là bên **tiêu thụ** tệp vàng ở Go; bên **sinh** tệp là `AG-CV-SOL-070`.

## 2. Trạng thái hiện tại (re-verify)

Đã đọc (2026-10-06): `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go` (`Exec` trả `map[string]any`; `execTimeoutForMethod` chỉ ngoại lệ `agent.execPrompt`), `.../devserveragent/frame.go:24` (`MaxMessageSize` 16 MiB), `.../usecase/relay_by_dev_server.go` (kiểm tenant sở hữu dev server, rồi bọc mọi lỗi `Exec`), `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/catalog_test.go` (mẫu golden: cờ `-update`, `testdata/tools_list.golden.json`), `.github/workflows/backend-go-issue-status-sync.yml` (ma trận `dialect: [postgres, mysql]`, `go-version: "1.25"`), `backend-go/go.work` (`go 1.26.0`), `agent/vitest.config.ts` (`include: ['src/**/*.test.ts']`), `agent/package.json` (`name: orca-agent`, script `test: vitest run`), `.github/workflows/pr.yml` (dòng 120, 129: `vitest run --config config/vitest.config.ts`), thư mục `config/` ở gốc (chỉ `max-lines-baseline.txt`, `oxlint-react-doctor.json`, `patches`, `scripts` với 3 file; **không** có `config/vitest.config.ts`).

| # | Correction relative to CR-CV-070 | Bằng chứng |
|---|---|---|
| C1 | CR nói "CI không chạy test của `agent/`": đúng. Không workflow nào trong 26 file của `.github/workflows/` nhắc `agent/` hay `orca-agent` (grep `agent/\|orca-agent`: không có kết quả). `pr.yml` trỏ `config/vitest.config.ts` không tồn tại ở gốc | grep + `ls config` |
| C2 | CI backend là **18** workflow `backend-go-*` (không phải 17); workflow dùng `go-version: "1.25"` trong khi `go.work` ghi `go 1.26.0` (lệch đã biết; hành vi `GOTOOLCHAIN` khi lệch chưa kiểm chứng) | `ls .github/workflows`, `go.work` |
| C3 | Kết quả `Client.Exec` là `map[string]any`: collector phải giải mã từ map/JSON, không giả định kiểu Go của agent | `devserveragent/client.go` |
| C4 | Mẫu golden có sẵn trong repo dùng cờ `-update` để ghi lại (`catalog_test.go:17`). Với tệp vàng agent, Go test **không** được tự ghi lại: tệp do agent sinh (xem quyết định D2) | `catalog_test.go` |
| C5 | Chưa có `testdata/agent-results/`, collector (CR-021), `codeintel.proto` (CR-010): mọi tên Go dưới đây là **đề xuất (mới)** | `ls` |

## 3. Lệch giữa CR và hợp đồng

| # | CR-CV-070 nói | Hợp đồng nói (thắng) | Hệ quả |
|---|---|---|---|
| L1 | Lỗi trôi định dạng: `CODEINTEL_TOOL_FAILED` với `detail: "format_drift"`; phiên bản/schema lạ: `CODEINTEL_TOOL_UNAVAILABLE` với `detail: "schema_version_unsupported"` | Trường là **`reason`** (agent-rpc §3.2: `TOOL_FAILED.reason ∈ {…, format_drift}`; `TOOL_UNAVAILABLE.reason ∈ {…, schema_version_unsupported}`) | tệp vàng lỗi dùng `data.reason`, không `data.detail` |
| L2 | `codeintel.status` trả `compatibility ∈ verified|untested|incompatible` | agent-rpc §4.1 chỉ có `tools.<tool>.supported: bool`; **không có** `compatibility` | `untested` không có chỗ trên dây; ghi vào mục 11 (khoảng trống hợp đồng G1) |
| L3 | Kết quả agent là `CodeIntelResult<T>` tự định nghĩa | Phong bì ở agent-rpc §2.2 có thêm `perf` (thay `toolTimingsMs`), `warnings[]`, `sources[].lineBase`; UI nhận phong bì phẳng khác (ui-api §2.2) | tệp vàng là phong bì **agent** (có `perf`); collector phải bỏ `perf` trước khi lưu snapshot (PQ-12, CR-071) |
| L4 | `SymbolRef` kind chuẩn hoá chữ thường, test `key` ổn định | PQ-20 chốt agent chuẩn hoá; backend kiểm lại `key` và đường dẫn (`NormalizeRepoPath`), hợp nhất hai nguồn | thêm vector dùng chung `symbolref-key-vectors.json` (task 03) |
| L5 | Workflow `code-intel-contract.yml` chạy cả vitest của agent | hợp đồng §10 ghi việc CI cho `agent/` thuộc CR-070 nhưng không giao file workflow cho khu vực nào | đề xuất: BE sở hữu file workflow, `AG-CV-SOL-070` sở hữu test vitest và script chụp (câu hỏi mở Q2) |

## 4. Phụ thuộc chéo khu vực

| Khu vực | Solution | Quan hệ |
|---|---|---|
| AG | `AG-CV-SOL-070-golden-fixtures-and-parsers` | **sinh** `mini-repo`, fixture C1, script chụp `agent/scripts/capture-codeintel-fixtures.mjs`, và ghi C2 vào `backend-go/services/code-intel-service/testdata/agent-results/`; test vitest `TestResultMatchesServiceGolden` đọc cùng tệp. Phải xong trước task BE-CV-TASK-070-01 (có tệp để kiểm) |
| BE | `BE-CV-SOL-010-scaffold-code-intel-service` | module Go, `go.work`, workflow service (G0) |
| BE | `BE-CV-SOL-021-agent-collector` | mã collector và chuẩn hoá cần kiểm (task 02, 03, 05) |
| BE | `BE-CV-SOL-020-canonical-graph-model` | `SymbolRef`, hàm khoá và va chạm (task 03) |
| BE | `BE-CV-SOL-023-infra-fleet-codeintel-transport` | `AgentRPCError`, trailer lỗi (task 04) |
| FE | `FE-CV-SOL-073-flag-gating-and-web-e2e` | `code-intel-fake-backend.ts` dựng từ cùng tệp vàng (CR-073 D10) |
| BE | `BE-CV-SOL-071`, `072`, `073` | 071 dùng chế độ `--bench` (không tệp vàng trong git); 072 dùng cùng thư mục `testdata/` cho `path-attack-vectors.json`; 073 dùng tệp vàng làm dữ liệu agent giả ở e2e T1 |

## 5. Giải pháp

### 5.1 Bố cục `testdata/` (mới)

```
backend-go/services/code-intel-service/testdata/
  agent-results/
    MANIFEST.json                         # phiên bản công cụ, commit mini-repo, sha256 + bytes từng tệp, hợp đồng áp dụng ("agent-rpc §2.2")
    gitnexus-1.6.9/                       # một thư mục theo phiên bản công cụ (khớp SUPPORTED_TOOL_VERSIONS của agent)
      status-stale-repo-root.json         # codeintel.status (§4.1)
      overview.json  processes.json  process.json  subgraph.json
      impact-found.json  symbol-found.json  routes.json
      detectChanges.json  structuralFacts.json
    codegraph-1.4.1/
      status-ready.json  codegraphSearch.json  files.head.json
    merged/                               # kết quả hai nguồn trong cùng phong bì (sources[] có 2 mục)
      symbol-two-sources.json
    errors/                               # phong bì lỗi JSON-RPC: {code, message, data{code,...}}
      ambiguous-symbol.json  tool-unavailable-schema.json  tool-failed-format-drift.json
      index-missing.json  reindex-in-progress.json  timeout-queue-wait.json  output-too-large.json
      path-not-allowed.json  repo-not-registered.json  symbol-not-found.json
  symbolref-key-vectors.json              # PQ-20: đầu vào → key kỳ vọng, gồm va chạm `#arity` và `#L<line>`
  security/                               # thuộc BE-CV-SOL-072 (path-attack-vectors.json)
```

Quy tắc: mọi tệp ≤ 20 KiB, thư mục phiên bản ≤ 300 KiB (cùng ngân sách CR-070 §2.3, **giả định chưa đo**); không đường dẫn tuyệt đối thật, không chuỗi giống token; tên không dùng `helpers/utils/common/misc`.

### 5.2 Bộ kiểm thử Go (mới)

| Test | Nội dung | Task |
|---|---|---|
| `TestAgentResultsManifest` | `MANIFEST.json` khớp `sha256`/`bytes` thực tế của từng tệp; không tệp mồ côi; ngân sách kích thước | 01 |
| `TestAgentResultsHygiene` | quét `/home/`, `/opt/`, `C:\`, `-----BEGIN`, mẫu `ghp_`, `AKIA`, `fileHashes`, `CANARY-` trong mọi tệp | 01 |
| `TestCollectorNormalizesAgentGolden` | mỗi tệp vàng qua collector (CR-021) cho `SymbolRef.key` ổn định, `truncated`/`totalCount` đúng, `kind` chuẩn hoá, **không còn `perf`** trong đầu ra lưu snapshot | 02 |
| `TestCollectorMergesTwoSourcesGolden` | `merged/symbol-two-sources.json`: hợp nhất theo `key`, `gitnexusId`/`codegraphId` đủ | 02 |
| `TestAgentResultSchemaVersionTolerance` | trường lạ bị bỏ qua; thiếu trường bắt buộc (`sources`, `data`) bị từ chối `CODEINTEL_RESULT_INVALID`; kết quả không phải object bị từ chối | 03 |
| `TestToolVersionRecorded` | mọi kết quả sau chuẩn hoá giữ `sources[].version` và `indexedAt` | 03 |
| `TestSymbolRefKeyVectors` | chạy `symbolref-key-vectors.json` qua hàm khoá của domain; không cộng dòng lần hai khi `lineBase==1`; thiếu `lineBase` thì coi 0-based | 03 |
| `TestAgentErrorGoldenMapping` | mỗi `errors/*.json` → `apperrors.Kind` đúng bảng agent-rpc §3.4 và `data` đúng giới hạn trailer ≤ 4 KiB | 04 |
| `TestEveryAgentMethodHasGolden` | mỗi method công khai agent-rpc §4 và mỗi `data.code` §3.2 có ít nhất một tệp vàng; thêm method/mã mà quên tệp thì đỏ (cùng tinh thần v6 CR-REQ-025 D5) | 05 |

### 5.3 CI (mới, task 06)

`.github/workflows/code-intel-contract.yml`: job `contract` (chặn PR) chạy `go test ./...` ở `services/code-intel-service` với `testdata/agent-results` và `pnpm --filter orca-agent exec vitest run src/relay/codeintel` (test do `AG-CV-SOL-070` viết); một bước khẳng định `vitest list` có test codeintel ("CI xanh vì không chạy gì" bị bắt); job `live-contract` (`schedule` hằng đêm + `workflow_dispatch`, **không chặn**) gọi script chụp của agent vào thư mục tạm rồi so với tệp đã commit và đẩy artifact. Cách cài CodeGraph không tương tác trên runner **chưa kiểm chứng** (CR-070 Q5): job live chỉ chạy được phần GitNexus cho đến khi có.

## 6. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Tệp vàng C2 đặt cạnh service Go, agent đọc bằng đường dẫn tương đối từ gốc repo | một nguồn; đổi hình dạng làm đỏ cả hai phía cùng PR (CR-070 D7) |
| D2 | Go test **không có** cờ `-update` cho `agent-results/`; chỉ script chụp của agent ghi | tránh Go "học" từ chính đầu ra rồi tự khớp; tệp phản ánh agent thật |
| D3 | Tệp lỗi là phong bì JSON-RPC đầy đủ (`code` số + `data.code`), không chỉ `data` | kiểm cả `error.code` số dùng lại `AgentErrorCode` (agent-rpc §3.1) |
| D4 | Mỗi phiên bản công cụ một thư mục | thêm phiên bản mà quên tệp thì `TestEveryAgentMethodHasGolden` đỏ |
| D5 | Workflow đặt tên theo CR-070 (`code-intel-contract.yml`) tách khỏi `backend-go-code-intel-service.yml` (CR-010) | một file kiểm hợp đồng liên khu vực; file service kiểm module riêng |
| D6 | Không lưu mẫu từ Orca; mẫu lớn chỉ ở chế độ `--bench` ngoài git | Orca đổi từng commit (CR-070 Q1) |

## 7. Tiêu chí chấp nhận

- [ ] `testdata/agent-results/` có đủ tệp mục 5.1, `MANIFEST.json` khớp băm, ngân sách kích thước và quét vệ sinh xanh.
- [ ] Collector chuẩn hoá mọi tệp vàng; `perf` không có trong đầu ra lưu/gửi đi.
- [ ] Vector `SymbolRef.key` chạy ở Go và (bên agent) ở vitest cùng tệp, cùng kết quả.
- [ ] Mỗi mã lỗi agent §3.2 có tệp vàng và ánh xạ `Kind` đúng §3.4.
- [ ] `TestEveryAgentMethodHasGolden` xanh; xoá một tệp vàng thì đỏ (thử tay một lần, ghi vào PR).
- [ ] Workflow `code-intel-contract.yml` chạy tầng chặn trên PR đúng đường dẫn và có bước khẳng định `agent/` được test.
- [ ] Không `max-lines` disable (AGENTS.md); tên tệp theo khái niệm.

## 8. Kiểm thử

Bảng 5.2 là toàn bộ kế hoạch. Chạy: `cd backend-go/services/code-intel-service && go test ./... -run 'AgentResults|Golden|SymbolRefKey|ToolVersion'`; CI: workflow mục 5.3. **Chưa chạy** (service chưa có). Hai dialect: các test này không chạm DB nên không cần ma trận; riêng workflow vẫn nằm cạnh ma trận `dialect` của `backend-go-code-intel-service.yml` (CR-010).

## 9. Rủi ro và điểm chưa kiểm chứng

- Phụ thuộc thứ tự: BE-CV-TASK-070-02 trở đi cần collector (CR-021) và tệp từ agent; nếu agent chậm, tạm dùng tệp viết tay đánh dấu `"_synthetic": true` trong `MANIFEST.json` và đổi khi có tệp thật (quyết định ở Q1).
- Hình dạng `data` của `codeintel.overview/processes/subgraph` do agent định nghĩa (agent-rpc §4.2–4.4); nếu CR-002 đổi khi chạy thử `cypher` (O-3, mẫu `STEP_EDGES`, `MEMBER_CLUSTER`… chưa chạy) thì mọi tệp tương ứng phải chụp lại.
- CodeGraph có thể tự nâng cấp (`~/.codegraph/versions/`; cơ chế chưa kiểm chứng): tệp vàng gắn phiên bản nhưng dev server có thể trôi.
- `lineBase` thiếu → coi GitNexus 0-based (PQ-20): vector phải có ca này, nếu không backend có thể cộng dòng sai lặng lẽ.
- SSH/WSL/Windows: tệp vàng chụp ở Linux; CRLF chỉ có biến thể sinh tay (CR-070 mục 6); Windows trả `unsupported_platform` (O-14).
- Go CI 1.25 so với `go.work` 1.26: chưa kiểm chứng việc workflow mới build được với toolchain nào.

## 10. Câu hỏi mở

1. Tạm dùng tệp tổng hợp tay nếu agent chưa sinh được tệp thật, hay chặn BE-CV-TASK-070-02 chờ agent?
2. Ai sở hữu file `code-intel-contract.yml` (đề xuất BE) và ai bảo trì job chạy vitest `agent/`?
3. `untested` (CR-070 §2.6) có đưa vào hợp đồng agent `status` (thêm `compatibility`) hay bỏ (chỉ `supported`)? Cần sửa hợp đồng trước (PR riêng).
4. Có dùng `buf` + `protovalidate`/`protojson` golden cho phía proto của tệp vàng (so khi chuyển sang `CodeIntelResult` proto) không, hay chỉ JSON?

## 11. Khoảng trống hợp đồng ghi nhận

- Không có `compatibility` trong agent-rpc §4.1 (L2).
- agent-rpc §2.2 không liệt kê tập `warnings[]` ổn định (CR-070 dùng `tool_version_untested`, `index_built_with_old_extraction`); test cần tập đóng để khẳng định.
- Hợp đồng không giao file workflow CI cho khu vực nào (L5).

## 12. Tham chiếu

- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go`, `frame.go`; `.../usecase/relay_by_dev_server.go`
- `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/catalog_test.go`, `testdata/tools_list.golden.json` (mẫu golden)
- `.github/workflows/backend-go-issue-status-sync.yml`, `.github/workflows/pr.yml`; `agent/vitest.config.ts`, `agent/package.json`; `backend-go/go.work`
- `docs/crs/v7/quality-rollout/CR-CV-070-golden-fixtures-and-tool-contract-tests.md`, `docs/crs/v7/README.md` mục 6, 8
- Mẫu định dạng: `specs/backend-go/crs/v6/request-quality-rollout/solutions/BE-REQ-SOL-025-e2e-feature-flag-rollout.md`

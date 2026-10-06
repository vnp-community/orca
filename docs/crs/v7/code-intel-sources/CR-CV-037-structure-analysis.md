# CR-CV-037 — Phân tích cấu trúc: vi phạm lớp hexagonal, vòng phụ thuộc, hotspot, mã chết, owner và danh sách Phát hiện

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-037 |
| **Tên** | Tạo bộ phát hiện cấu trúc (`Finding`) từ đồ thị import/gọi và lịch sử git; cơ chế `finding_key` ổn định, `finding_dismissals` và RPC `ListFindings`/`DismissFinding` |
| **Loại** | Feature (backend `code-intel-service`; cần một method hẹp mới ở agent, xem 2.2) |
| **Priority** | 🟠 P1 (đợt 5) |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-036 (overlay: `violations[]`, `changedFiles`), CR-CV-020 (`SymbolRef`, `ModuleGraph`), CR-CV-021 (collector), CR-CV-011 (bảng `finding_dismissals`), CR-CV-013 (phân quyền, audit), CR-CV-030 (đọc file/`git.*`), CR-CV-002 (nguồn GitNexus; xem 2.2 về method mới), CR-CV-033 (component C4, tuỳ chọn, để gán lớp). Nguồn ngoài (research 09): **E5** lịch sử git (cần chốt khoảng thời gian), **E12** CODEOWNERS (**đã kiểm: repo không có file**), E6 `c4.yaml` (tuỳ chọn, cho quy tắc lớp tuỳ biến) |
| **Mở khoá** | CR-CV-036 (`violations[]`, `VIOLATION` trong điểm rủi ro), CR-CV-059 (lens Hợp đồng + danh sách Phát hiện), CR-CV-040 (kênh `codeIntel.findings`, `codeIntel.dismissFinding`) |
| **Tác động** | `backend-go/services/code-intel-service` (mới): package phát hiện, repository `finding_dismissals`, RPC `ListFindings`, `DismissFinding`; `backend-go/proto/orca/codeintel/v1`; `agent/src/relay` (**đề xuất** thêm một method hẹp ở CR-CV-001/002, xem 2.2); không sửa mã nguồn của các service được phân tích |

---

## 1. Bối cảnh và vấn đề

R2, R3 và R8 ([08 §7](../../../research/view-code/08-views-and-review-models.md)) cần các phát hiện cấu trúc để người review thấy "agent có phá kiến trúc không" mà không phải đọc từng tệp. Khảo sát 2026-10-05 (chạy CLI chỉ-đọc trên chỉ mục GitNexus 1.6.9 của Orca, đọc code thật):

**Vi phạm lớp hexagonal có thật.** Truy vấn `IMPORTS` giữa tệp `internal/usecase/**` và `internal/adapter/**` (loại `_test.go`) cho 6 tệp ở 3 service:
- `infra-fleet-service/internal/usecase/{ports.go, get_terminal_agent_status.go, attach_pty.go}` import `internal/adapter/eventbus` (đã đọc `ports.go:18` và `get_terminal_agent_status.go:10`);
- `ai-provider-service/internal/usecase/{ports.go, test_connection.go}` import `internal/adapter/eventbus` (đã đọc `ports.go:17`, `test_connection.go:8`);
- `mcp-service/internal/usecase/usecasetest/harness.go` import `internal/adapter/policyengine` (đây là bộ khung test, không phải vi phạm thật).
Truy vấn `internal/domain/**` import `usecase`/`adapter` cho **0** kết quả. Có coupling adapter→adapter thật (ví dụ `infra-fleet-service/internal/adapter/mysql/shared.go:5` import `adapter/sshrelay`), không nhất thiết là lỗi.
- **Cạm bẫy đo đạc**: GitNexus tạo cạnh `IMPORTS` **từ tệp nguồn tới từng tệp** của package được import (một dòng import `adapter/eventbus` cho ra 3 cạnh tới `agent_status_publisher.go`, `health_publisher.go`, `publisher.go`). Phải khử trùng theo `(tệp nguồn, thư mục package đích)`, nếu không số vi phạm bị nhân lên. Cạnh `IMPORTS` **không có số dòng** (chỉ có `confidence`, `reason`, `step`), nên bằng chứng chỉ ở mức tệp.

**Vòng phụ thuộc.** `gitnexus check --cycles --json -r orca` có sẵn và chạy được (đã chạy): `status:"cycles_found"`, `cycleCount: 93`, mỗi phần tử `cycles[].files[]` là danh sách tệp (tệp đầu lặp lại ở cuối). Phân bố theo thư mục gốc: `frontend` 31, `desktop` 27, `backend` 13, `agent` 10, `tests` 7, `mobile` 5; **không có** vòng ở `backend-go` (Go cấm vòng import giữa package; cạnh `IMPORTS` mức tệp-sang-tệp-trong-package không tạo vòng). Độ dài lớn nhất quan sát được là 5 tệp — chưa rõ công cụ có giới hạn độ dài khi tìm vòng (chưa kiểm chứng). Lệnh `check` **không** nằm trong whitelist D5 hiện tại (README mục 3.2 không có method nào bọc nó).

**Hotspot cần ba thành phần; độ phức tạp chưa có sẵn.**
- Tần suất đổi: `git log` qua `git.exec` (whitelist có `log`, `shortlog`, `show`). Đã đo trên repo Orca: `git log --since=90.days --no-merges --name-only --format=%x00%H%x09%ae` trả 886 commit, ~2,4 MB, 0,46 s (cục bộ); khoảng 180 ngày đã tới 6 509 commit (có nhập khối lượng lớn). Git cảnh báo `exhaustive rename detection was skipped due to too many files … diff.renameLimit` → các commit di chuyển hàng loạt làm mất lịch sử đường dẫn (ví dụ `src/renderer/...` và `frontend/src/renderer/...` đếm riêng). Tệp nhiễu đứng đầu tần suất: `package.json`, `TASKS-INDEX.md`, các `i18n/locales/*.json`.
- **Độ phức tạp**: GitNexus `Function` chỉ có `id, name, filePath, startLine, endLine, isExported, content, description` (đã in một bản ghi mẫu); CodeGraph `nodes` có `start_line/end_line/signature/is_exported` (research 04), **không có cyclomatic/cognitive complexity ở cả hai**. Chỉ đo được độ dài: `sum`/`max` của `endLine - startLine + 1` theo tệp. Truy vấn Cypher gom theo tệp chạy được (~1,7 s): ví dụ `infra-fleet-service/cmd/server/main.go` có 3 hàm, tổng 828 dòng, hàm dài nhất 785 dòng. **Không** hỗ trợ `WHERE (s:Function OR s:Method)` trong LadybugDB (lỗi parser đã gặp): phải chạy hai truy vấn theo nhãn rồi gộp.
- Độ trung tâm: in-degree theo `IMPORTS` chạy được toàn repo (1,9 s): `frontend/src/shared/types.ts` 1 394, `backend-go/common/apperrors/apperrors.go` 641.

**Mã chết.** Truy vấn "hàm export không có cạnh vào `CALLS|ACCESSES|IMPORTS`" dùng `NOT EXISTS { MATCH (x)-[r:CodeRelation]->(f) WHERE r.type IN [...] }` chạy được (~1,7 s). Trên `backend-go/services` (loại `_test.go` và `/cmd/`): **13 / 1 231** hàm export không có tham chiếu vào, ví dụ `NewStubComplexExecutor`, `ResolveAgentBinary` (xuất hiện ở `task-service` và `workflow-service`), `BuildAgentArgs`, `NewFleetDefinitionStore` (cả bản mysql và postgres; đã `grep` xác nhận chỉ được định nghĩa, không nơi nào gọi). Nếu không loại `_test.go`, kết quả đầy hàm `Test…` (mọi hàm test là `Function` export). Một mục trả về (`ErrServerNotApproved`) là biến lỗi nhưng mang nhãn `Function` → có sai lệch phân loại kind. Chưa kiểm chứng tỷ lệ đúng/sai ngoài mẫu nhỏ này.

**Owner.** **Không có tệp `CODEOWNERS`** ở gốc, `.github/`, `docs/` hay bất kỳ nơi nào (tìm `find . -iname 'CODEOWNERS*'` loại `node_modules`; chỉ có mã Go phân tích CODEOWNERS ở `backend-go/services/scm-integration-service/internal/usecase/codeowners.go`: `ParseCodeowners`, `MatchOwners`, luật "khớp cuối thắng"; nằm trong `internal` của module khác nên **không import được**). `.github/` chỉ có `ISSUE_TEMPLATE`, `pull_request_template.md`, `workflows`. Vì vậy owner mặc định phải suy từ lịch sử git.

**Không có `finding_dismissals` trong code** (README mục 3.5 chỉ định nghĩa bảng; chưa có service).

## 2. Giải pháp đề xuất

### 2.1 Mô hình `Finding` (proto + domain, mới)

```
Finding {
  finding_key: string,           // ổn định, xem 2.5
  rule: "layer.domain-imports-outer" | "layer.usecase-imports-adapter" | "layer.adapter-imports-adapter"
      | "cycle.import" | "hotspot.file" | "dead.unused-export",
  severity: "error" | "warning" | "info",
  titleKey: string, params: map<string,string>,       // i18n phía frontend, không có văn bản cứng
  subject: string,               // chuỗi đọc được, ví dụ "infra-fleet-service: usecase → adapter/eventbus"
  evidence: Evidence[],          // {path, line?, symbol?: SymbolRef}; line chỉ có khi nguồn cung cấp
  metrics: map<string,number>,   // hotspot: churn, complexity, centrality, score...
  owner?: Owner,                 // {source:"codeowners|history", names[], share?}
  scope: { service?, componentId?, layer? },
  introduced?: "yes"|"touched"|"unknown",   // so với overlay (CR-CV-036)
  dismissed?: { by, at, reason }, confidence: "high"|"medium"|"low", indexFreshness
}
```

RPC (tên theo README 3.6): `ListFindings(ListFindingsRequest{repo_binding_id, rules[], severities[], path_prefix, include_dismissed=false, scope: ALL|CHANGED, base_ref?, page_size, page_token}) returns (ListFindingsResponse{findings[], next_page_token, totalCount, truncated, sources[], indexFreshness})`; `DismissFinding(DismissFindingRequest{repo_binding_id, finding_key, action: DISMISS|RESTORE, reason}) returns (DismissFindingResponse{finding_key, dismissed})`. Message do CR này sở hữu; không khai báo RPC nào chưa có message (v6 CR-REQ-001).

### 2.2 Nguồn dữ liệu và method agent cần thêm (đề xuất)

README mục 3.2 hiện không có method nào phục vụ: cạnh `IMPORTS` toàn cục (vượt hạn mức `codeintel.subgraph` ≤ 4 000 cạnh, trong khi riêng backend-go đã có 38 337 cạnh `IMPORTS`), vòng (`check --cycles`), hàm không ai dùng, kích thước theo tệp. Đề xuất **một method hẹp** thuộc quyền sở hữu CR-CV-001/002 (whitelist, D5), không nhận `args` hay Cypher tự do:

`codeintel.structuralFacts {workspaceRoot, kind: "layerImports" | "cycles" | "importInDegree" | "fileSizes" | "unusedExports", pathPrefixes?: string[], limit, offset}`

| `kind` | Thực thi bên trong agent (mẫu cố định, chỉ đọc, đã thử cú pháp) | Kết quả |
|---|---|---|
| `layerImports` | `MATCH (a:File)-[r:CodeRelation {type:'IMPORTS'}]->(b:File) WHERE a.filePath CONTAINS $fromSeg AND b.filePath CONTAINS $toSeg AND NOT a.filePath ENDS WITH '_test.go' RETURN a.filePath, b.filePath` cho các cặp đoạn đường dẫn cố định (`/internal/usecase/`→`/internal/adapter/`, `/internal/domain/`→`/internal/usecase/`, `…→/internal/adapter/`, `/internal/adapter/`→`/internal/adapter/`) | cặp `(fromFile, toFile)`; **agent khử trùng theo thư mục đích** |
| `cycles` | `gitnexus check --cycles --json -r <repo>` (mới vào whitelist, chỉ cờ này) | `cycles[]` tệp, kèm `cycleCount` |
| `importInDegree` | `MATCH (a:File)-[r:CodeRelation {type:'IMPORTS'}]->(b:File) RETURN b.filePath, count(DISTINCT a) ORDER BY … LIMIT $n` | tệp, in-degree |
| `fileSizes` | hai truy vấn theo nhãn `Function` và `Method`: `MATCH (f:File)-[:CodeRelation {type:'DEFINES'}]->(s:Function) … RETURN f.filePath, count(s), sum(s.endLine-s.startLine+1), max(…)`; gộp ở agent | tệp, số hàm, tổng dòng, hàm dài nhất |
| `unusedExports` | `MATCH (f:Function) WHERE f.isExported = true AND … AND NOT EXISTS { MATCH (x)-[r:CodeRelation]->(f) WHERE r.type IN ['CALLS','ACCESSES','IMPORTS'] }` kèm loại `_test.go`, `/cmd/`, `usecasetest` | `SymbolRef` |

Mọi `kind` đi qua cổng chung của agent (giới hạn kích thước, `truncated`, `totalCount`, mã lỗi `CODEINTEL_*`). `limit` ≤ 5 000 dòng/lần. Chi phí mỗi lệnh ~1,7–1,9 s (đã đo trên chỉ mục Orca). **Nếu duyệt không thêm method này**, các tính năng tương ứng giảm xuống (xem 6): vòng và mã chết đòi hỏi truy cập đồ thị mà `codeintel.subgraph` hiện tại không cung cấp.

Nguồn khác: (a) lịch sử git qua `git.exec` (Part A): `git log --since=<N>.days --no-merges --name-only --format=%x00%H%x09%an` (không chứa ký tự bị chặn `& | ; $ \` < > \ !`; đã đối chiếu `SHELL_METACHARACTERS` ở `agent/src/relay/agent-git-handler.ts`); (b) đọc `CODEOWNERS` qua `fs.*` (CR-CV-030) ở các vị trí `CODEOWNERS`, `.github/CODEOWNERS`, `docs/CODEOWNERS`, `.gitlab/CODEOWNERS` (GitLab cũng hỗ trợ; AGENTS.md yêu cầu không gắn riêng GitHub).

### 2.3 Quy tắc phát hiện

**Lớp hexagonal (R2)** — áp dụng cho `backend-go/services/<svc>/internal/**` (mặc định, vì đây là bố cục có thật). Quy tắc suy từ thư mục, có thể tuỳ biến bằng mục `layers` trong `c4.yaml` (E6, CR-CV-033; xem 7):

| Quy tắc | Điều kiện (tệp A import package P, khử trùng theo (A, thư mục P)) | Mức |
|---|---|---|
| `layer.domain-imports-outer` | A trong `internal/domain/`, P trong `internal/usecase/`, `internal/adapter/` hoặc `internal/config/` | `error` |
| `layer.usecase-imports-adapter` | A trong `internal/usecase/` (không phải `usecasetest/`), P trong `internal/adapter/` | `warning` |
| `layer.adapter-imports-adapter` | A trong `internal/adapter/<x>/`, P trong `internal/adapter/<y>/` với `x ≠ y` | `info` (có thể chủ ý, ví dụ `adapter/mysql` import `adapter/sshrelay` để thoả cùng một interface) |

Loại trừ cố định: `_test.go`, `usecasetest/`, `testutil`, `cmd/`, `proto/gen`, `*.pb.go`. Gom theo `(service, package nguồn, package đích)` thành **một** finding, `evidence` liệt kê các tệp nguồn. Đối chiếu kết quả mong đợi trên Orca: 2 finding `usecase-imports-adapter` (infra-fleet → `adapter/eventbus` gồm 3 tệp; ai-provider → `adapter/eventbus` gồm 2 tệp), 0 `domain-imports-outer`, `usecasetest` bị loại.

**Vòng phụ thuộc** — `cycle.import`, mức `warning`. Nguồn `kind:"cycles"`; nếu không có method, tự tính SCC (Tarjan) trên `ModuleGraph` (CR-CV-020) chỉ trong phạm vi một cụm/thư mục khi đồ thị nhỏ. Mỗi SCC cỡ > 1 là một finding; `evidence` = các tệp theo thứ tự chuẩn (đã sắp). Vòng một tệp tự import bị bỏ. Báo cả độ dài vòng. Trên Orca hiện có 93 vòng (không ở `backend-go`).

**Hotspot (R3)** — `hotspot.file`, mức `info`. Cửa sổ thời gian `N` ngày (mặc định **90**, chốt E5; cấu hình tenant 30–365).
- `churn(f)` = số commit **không merge** chạm `f` trong cửa sổ, **bỏ qua** commit chạm > 100 tệp (nhập khối lượng/di chuyển/format hàng loạt, xem cảnh báo `renameLimit`); bỏ tệp nhiễu: `pnpm-lock.yaml`, `package-lock.json`, `go.sum`, `i18n/locales/*.json`, `proto/gen/**`, `**/*.md`, `docs/**`, `specs/**`. Có thêm `authors(f)` (số tác giả khác nhau, dùng `%an`, **không lưu email**).
- `complexity(f)` = proxy độ dài, vì **không có chỉ số phức tạp thật**: `0,5·pr(totalLines) + 0,5·pr(longestSymbol)` với `pr` là hạng phần trăm trong cùng repo (kind `fileSizes`). Ghi rõ nhãn "độ dài hàm" trên UI, không gọi là cyclomatic.
- `centrality(f)` = `pr(importInDegree)`.
- `score = pr(churn) · complexity · centrality` (0..1; nhân để chỉ tệp cao ở **cả ba**), `metrics` trả đủ ba thành phần và giá trị thô. Điều kiện vào danh sách: `churn ≥ 3` và `score ≥ 0,05`; trả tối đa 50 tệp, xếp giảm dần. `introduced` không áp dụng.
- Giới hạn lệnh: một lần `git.exec` cho cả cửa sổ (≤ ~2,5 MB / 886 commit đo được); nếu vượt 8 MiB hoặc quá thời hạn 60 s → chia theo khoảng thời gian (hai lần gọi `--since`/`--until`) và đánh dấu `confidence:"low"`.

**Mã chết (R8)** — `dead.unused-export`, mức `info`, mặc định **chỉ cho Go** (`backend-go/services/**`, `Function` export, loại `_test.go`, `/cmd/`, `usecasetest`, `proto/gen`). Chỉ xét `Function` (không xét `Method`: gọi qua interface làm báo sai). Các ngôn ngữ khác (TypeScript/React: component qua JSX, handler đăng ký bằng chuỗi, IPC) **tắt mặc định** vì chưa kiểm chứng độ chính xác. Mỗi finding mang `confidence:"medium"` và ghi rõ "không phát hiện tham chiếu nào trong chỉ mục; có thể được gọi qua reflection/đăng ký ngoài mã". Báo nhầm có thể bị `dismiss`.

**Owner (R8)**
1. Nếu có tệp CODEOWNERS (đường dẫn ở 2.2): parse theo cú pháp gitignore, **luật cuối thắng** (GitHub/GitLab); bỏ qua dòng trống, `#`; dòng một cột (không có owner) bỏ qua (tham khảo cách `codeowners.go` làm; viết lại trong module này, không import chéo service); tiêu đề section kiểu GitLab (`[Tên]`, `^[Tên]`) **chưa hỗ trợ ở MVP** (bỏ qua dòng, ghi cảnh báo). `Owner.source="codeowners"`.
2. Nếu **không có** file (trường hợp hiện tại của Orca): suy từ `git shortlog -sn --since=<N>.days --no-merges HEAD -- <path>` (bắt buộc truyền `HEAD`: không có đối số revision thì `shortlog` đọc stdin mà `git.exec` đóng stdin → rỗng). Owner = tác giả có ≥ 50 % commit; không ai đạt → danh sách top 2. `Owner.source="history"`, `share`. **Không lưu email**, chỉ tên hiển thị.
3. Owner chỉ gắn vào `Finding.owner` và `ChangedFile.owner` (CR-CV-036); không có finding riêng cho owner.

### 2.4 Tính toán, cache và hạn mức

- Mỗi `rule` là một "bộ phát hiện" chạy độc lập, có timeout riêng; lỗi một bộ không làm hỏng các bộ khác (`sources[]` ghi trạng thái). Kết quả hợp nhất lưu `graph_snapshots(view="findings", commit=indexedCommit, params_hash)` (CR-CV-022); **dismiss không nằm trong cache** (ghép lúc đọc).
- Tính khi có yêu cầu lần đầu hoặc sau `codeintel.indexChanged` (D4); singleflight theo khoá cache. Chi phí ước lượng: layerImports 4 truy vấn + cycles 1 + inDegree 1 + fileSizes 2 + unused 1 ≈ 9 × ~1,8 s + `git log` ≈ 17 s tuần tự; chạy song song theo hạn mức đồng thời (CR-CV-013). Đây là ước tính, chưa đo qua WS.
- Hạn mức: ≤ 2 000 finding/lần tính; `ListFindings` phân trang `page_size ≤ 200`; `truncated` + `totalCount`. Thứ tự ổn định: `severity` giảm dần, `rule`, `subject`.
- `scope=CHANGED`: chỉ finding có `evidence.path` ∈ `changedFiles` của overlay (CR-CV-036); `introduced:"yes"` nếu tệp ở trạng thái `added` (mới) hoặc, với quy tắc lớp, finding không tồn tại ở snapshot của `mergeBase` (nếu có cache); ngược lại `touched`. Không suy ra `introduced` khi thiếu snapshot cũ (`unknown`).
- Khi index cũ: trả finding kèm `indexFreshness`, `confidence` hạ một bậc; không từ chối.

### 2.5 `finding_key` ổn định và `finding_dismissals`

`finding_key = "<rule>:<16 hex đầu của sha256(canonicalSubject)>"` với `canonicalSubject` không chứa số dòng hay đường dẫn tuyệt đối:

| Quy tắc | `canonicalSubject` |
|---|---|
| `layer.*` | `<service>::<package nguồn tương đối>::<package đích tương đối>` (cấp package, **không** cấp tệp: thêm/bớt tệp trong package không đổi khoá) |
| `cycle.import` | danh sách tệp trong SCC, sắp từ điển, nối bằng `\n` (khoá đổi khi thành viên vòng đổi: vòng khác = phát hiện khác) |
| `hotspot.file` | `<đường dẫn tệp>` |
| `dead.unused-export` | `SymbolRef.key` (`kind:filePath:name`, README 3.4) |

Giới hạn đã biết: đổi tên tệp/symbol đổi khoá → mất trạng thái dismiss (chấp nhận, ghi ở 6). Khoá có tiền tố phiên bản quy tắc không cần thiết ở v1; nếu đổi cách chuẩn hoá thì đổi tên `rule` (ví dụ `layer.v2…`) để dismiss cũ không áp nhầm.

`finding_dismissals` (README 3.5: `id, tenant_id, repo_binding_id, finding_key, reason, dismissed_by, at`), CR-CV-011 sở hữu migration cho cả hai dialect. Ràng buộc bổ sung đề xuất: `UNIQUE (tenant_id, repo_binding_id, finding_key)`; chỉ mục `(tenant_id, repo_binding_id)`; `reason` bắt buộc, 1–500 ký tự; `finding_key` ≤ 128 ký tự.
- `DismissFinding(DISMISS)` là upsert (cập nhật `reason`, `dismissed_by`, `at`); `RESTORE` xoá dòng. Cả hai **idempotent** (at-least-once). Hành động ghi vào audit (CR-CV-013) kèm `finding_key` và `reason`, không kèm mã nguồn.
- Quyền: dismiss cần quyền ghi trên repo binding (cùng vai trò `SaveReviewState`); xem finding cần quyền đọc. Dismiss áp dụng cho **cả repo binding** (mọi worktree của repo), không chỉ worktree hiện tại.
- Dismiss không làm finding biến khỏi dữ liệu: `include_dismissed=false` mặc định chỉ ẩn khỏi danh sách; số lượng finding đã dismiss hiển thị trong `totalCount` riêng để người dùng thấy mình đã ẩn bao nhiêu.
- Không có hết hạn tự động ở MVP (xem 7).

## 3. Quyết định thiết kế

1. **Nguồn đồ thị qua một method hẹp có kiểu liệt kê** (không Cypher tự do): giữ nguyên D5; đồng thời giải quyết giới hạn ≤ 4 000 cạnh của `subgraph`.
2. **Khử trùng cạnh `IMPORTS` theo (tệp, thư mục package đích)** ở agent: tránh nhân lên vi phạm do cách GitNexus trải cạnh tới từng tệp trong package; cạnh không có số dòng nên bằng chứng ở mức tệp, dòng lấy lúc người dùng mở tệp (CR-CV-053).
3. **Hotspot ghi rõ proxy**: không bịa chỉ số phức tạp; độ dài hàm là đại diện trung thực và rẻ; dùng hạng phần trăm để không phụ thuộc đơn vị.
4. **Bỏ commit hàng loạt** thay vì dùng `--follow`: không có cách rẻ để theo dõi đổi tên trong repo có di chuyển lớn; chấp nhận undercount, cảnh báo.
5. **Owner mặc định từ lịch sử** vì không có CODEOWNERS; tự hỗ trợ CODEOWNERS khi repo thêm sau này.
6. **Mã chết chỉ Go, chỉ `Function`, mức `info`**: tránh báo nhầm phổ biến (interface, reflection, JSX); tăng phạm vi khi đo được tỷ lệ đúng.
7. **Dismiss theo repo binding, không theo commit**: finding là tính chất của mã; khoá cấp package/tệp/symbol ổn định hơn dòng.
8. **Không tạo `finding` cho owner/độ cũ**; chúng là thuộc tính và cờ.
9. **Chế độ lỗi từng phần** thay vì tất-cả-hoặc-không: giống overlay (CR-CV-036).

## 4. Tiêu chí chấp nhận

- [ ] `ListFindings(rules=[layer.usecase-imports-adapter])` trên Orca trả 2 finding (infra-fleet và ai-provider, cùng đích `adapter/eventbus`) với `evidence` đúng các tệp đã liệt kê ở mục 1, khử trùng (không nhân 3 lần), và **không** có `usecasetest/harness.go`; `layer.domain-imports-outer` trả 0.
- [ ] Mỗi finding lớp có `subject`, `evidence` (≥ 1 tệp), `severity` đúng bảng 2.3; cùng đầu vào cho cùng `finding_key` qua 100 lần chạy.
- [ ] `cycle.import`: số finding bằng `cycleCount` của `gitnexus check --cycles --json` sau khử trùng; không có finding nào ở `backend-go`; `evidence` sắp xếp xác định.
- [ ] `hotspot.file`: bỏ commit > 100 tệp và tệp nhiễu (`pnpm-lock.yaml`, `i18n/locales/*.json`, `docs/**`); `metrics` có `churn`, `complexity`, `centrality`, `score`; ≤ 50 tệp, giảm dần; UI/tài liệu ghi nhãn "độ dài hàm" cho complexity.
- [ ] `dead.unused-export`: không trả hàm trong `_test.go`, `/cmd/`, `usecasetest`; trả `NewFleetDefinitionStore` (hai bản); `confidence:"medium"`; chỉ Go ở cấu hình mặc định.
- [ ] Owner: không có CODEOWNERS → `Owner.source="history"`, không lưu email, dùng `HEAD` rõ ràng trong `shortlog`; có CODEOWNERS (fixture) → luật cuối thắng, khớp bộ test của `scm-integration-service` về hành vi.
- [ ] `DismissFinding` idempotent; sau dismiss `ListFindings` mặc định không còn finding đó và `totalCount` ẩn tăng; `RESTORE` đưa lại; vi phạm quyền/tenant bị từ chối; có audit.
- [ ] Một bộ phát hiện lỗi (ví dụ `cycles` timeout) không làm mất kết quả bộ khác; `sources[]` nêu rõ trạng thái; `confidence` hạ khi index cũ.
- [ ] `scope=CHANGED` chỉ trả finding chạm `changedFiles` và gắn `introduced` đúng (`yes` cho tệp mới, `unknown` khi thiếu snapshot cũ); `CR-CV-036.violations[]` lấy từ đây.
- [ ] Hạn mức: ≤ 2 000 finding, phân trang ≤ 200, `truncated`/`totalCount` đúng; migration `finding_dismissals` chạy ở Postgres và MySQL; mọi truy vấn có `tenant_id`.
- [ ] Mọi lệnh git thêm nằm trong whitelist Part A (`log`, `shortlog`) và không dùng ký tự bị chặn; không dùng cờ cấm.

## 5. Kiểm thử

- **Unit**: chuẩn hoá và băm `finding_key` (đổi dòng/tệp cùng package không đổi khoá); khử trùng cạnh theo thư mục; luật lớp với danh sách tệp giả (đủ ca: test, `usecasetest`, adapter↔adapter); Tarjan; `pr` hạng phần trăm và biên (`score` 0/1); parse CODEOWNERS (luật cuối thắng, `**`, thư mục, dòng không owner); parse `git log` với ký tự phân tách NUL (`%x00`) và tên tệp có dấu cách/Unicode (git mặc định trích dẫn đường dẫn có ký tự đặc biệt; cần xác minh cách giải mã, tham khảo `decodeGitCQuotedPath` ở `agent/src/shared/git-cquoted-path.ts`).
- **Hợp đồng công cụ** (CR-CV-070): đầu ra mẫu của `gitnexus check --cycles --json` và các truy vấn mẫu theo phiên bản 1.6.9 (đã quan sát: lỗi `function SPLIT does not exist` và không hỗ trợ `WHERE (s:A OR s:B)` → test chặn dùng nhầm cú pháp); mẫu truy vấn phải chạy được trên LadybugDB trong CI.
- **Fixture git thật**: repo tạm có commit hàng loạt (> 100 tệp) và tệp nhiễu; kiểm tra bỏ qua; tính `churn` đúng; thử git 2.25 và bản mới (`guides/reference/git-compatibility.md`).
- **Golden Orca (tuỳ chọn, chạy thủ công/nightly)**: so với 6 tệp/3 service ở mục 1; số hàm chết ~13 ± biến động theo commit.
- **Dismiss**: unique, idempotent, đồng thời (hai người dismiss cùng key), cô lập tenant, hai dialect, audit.
- **Bảo mật**: finding không chứa nội dung mã nguồn ngoài `SymbolRef`; không lộ email; cô lập tenant (CR-CV-072).
- **Hiệu năng** (CR-CV-071): tổng thời gian tính finding trên Orca < 60 s ở lần đầu (ngân sách tạm, cần đo).

## 6. Rủi ro và điểm chưa kiểm chứng

- **Chưa chạy hệ thống**; các số liệu 6 tệp, 93 vòng, 13/1 231, 886 commit đo trên máy khảo sát ngày 2026-10-05 qua CLI trực tiếp, chưa qua agent/WS; chỉ mục có thể lệch HEAD tại thời điểm đo (chưa kiểm tra `indexedCommit` so với HEAD).
- Method `codeintel.structuralFacts` là **đề xuất**; nếu bị từ chối thì: lớp chỉ làm được bằng đọc import qua `fs.*` (nặng: ≥ vài trăm lần đọc), vòng và mã chết bỏ khỏi MVP.
- `gitnexus check --cycles`: chưa rõ có giới hạn độ dài/số lượng vòng; chưa thử `--branch`; chưa biết thời gian chạy. Quan sát duy nhất: ra 93 vòng, dài ≤ 5.
- Truy vấn lớn qua `gitnexus cypher`: một lần thử kéo toàn bộ cạnh `IMPORTS` backend-go (38 337 dòng) cho JSON bị cắt giữa chuỗi khi đọc qua pipe (nguyên nhân chưa xác định; có thể do bộ đệm stdout khi tiến trình thoát). Mọi truy vấn của CR này đều gộp/`count`/`LIMIT` ở phía công cụ; vẫn cần kiểm thử đọc đầu ra lớn.
- Phát hiện vòng/lớp ở Go dùng cạnh mức tệp (`IMPORTS` tới từng tệp trong package); nếu GitNexus đổi cách tạo cạnh, kết quả đổi (đã ghi trong hợp đồng công cụ CR-CV-070).
- Phân loại `kind` của GitNexus có thể sai (`ErrServerNotApproved` mang nhãn `Function`); ảnh hưởng độ chính xác mã chết.
- Tỷ lệ báo nhầm của mã chết chưa đo (mẫu: 1 trường hợp xác nhận thật qua `grep`, 12 còn lại chưa kiểm tay).
- Đổi tên/di chuyển tệp làm mất lịch sử churn (git bỏ dò đổi tên vì quá nhiều tệp; hai cây `src/...` và `frontend/src/...` đã quan sát) và làm mất dismiss.
- `shortlog` có sắc thái stdin; chưa thử qua `git.exec` thật (suy từ mã: `child.stdin?.end()` ở `agent-git-handler.ts`).
- Repo lớn: 2,4 MB cho 90 ngày là ở Orca; repo khác có thể vượt `maxBuffer`/60 s; đã có phương án chia khoảng.
- Phụ thuộc **Part A/`direct-websocket`** (D2); không chạy ở `relay-ssh` MVP.
- CODEOWNERS section GitLab `[Section]` chưa hỗ trợ; nếu repo dùng sẽ cho owner sai.

## 7. Câu hỏi mở

1. Duyệt thêm `codeintel.structuralFacts` (và cho phép `check --cycles` vào whitelist), hay tách thành nhiều method riêng cho CR-CV-001/002?
2. Cho phép tuỳ biến quy tắc lớp ở đâu: `c4.yaml` (E6, bảng `c4_overrides`), một file riêng trong repo, hay chỉ mặc định? Có tắt từng quy tắc theo repo không?
3. Cửa sổ hotspot mặc định 90 ngày có phù hợp (E5)? Và có cần ngưỡng khác cho repo ít commit?
4. Có muốn tạo file `CODEOWNERS` trong repo Orca (E12) để owner chính xác; hay chấp nhận owner suy từ lịch sử?
5. Dismiss có cần hết hạn (ví dụ 90 ngày) hoặc tự bật lại khi finding đổi mức độ không?
6. Có xếp `layer.adapter-imports-adapter` thành finding hay bỏ (nhiều khả năng chủ ý)?
7. Mở rộng mã chết sang TypeScript khi nào (cần đo tỷ lệ đúng ở `frontend`)?

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md`; `/opt/repos/orca/docs/research/view-code/05-graph-schemas.md` (§2.2, 4), `08-views-and-review-models.md` (§7 R2, R3, R8), `09-external-inputs-required.md` (E5, E6, E12), `04-raw-data-and-pipeline.md`
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/usecase/ports.go` (dòng 18), `…/get_terminal_agent_status.go` (dòng 10), `…/adapter/mysql/shared.go` (dòng 5)
- `/opt/repos/orca/backend-go/services/ai-provider-service/internal/usecase/ports.go` (dòng 17), `…/test_connection.go` (dòng 8)
- `/opt/repos/orca/backend-go/services/mcp-service/internal/usecase/usecasetest/harness.go`
- `/opt/repos/orca/backend-go/services/scm-integration-service/internal/usecase/codeowners.go`, `…/suggest_pull_request_reviewers.go`
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/adapter/mysql/fleet_definition_repository.go`, `…/postgres/fleet_definition_repository.go`
- `/opt/repos/orca/agent/src/relay/agent-git-handler.ts` (whitelist, `SHELL_METACHARACTERS`, stdin), `/opt/repos/orca/agent/src/relay/agent-git-exec-validator.ts`
- CLI chỉ-đọc đã chạy: `gitnexus check --cycles --json -r orca`; `gitnexus cypher -r orca` (IMPORTS lớp, in-degree, `fileSizes`, `unusedExports`); `git log --since=…`
- `/opt/repos/orca/guides/reference/git-compatibility.md`, `/opt/repos/orca/guides/STYLEGUIDE.md`, `/opt/repos/orca/AGENTS.md`
- CR liên quan: CR-CV-001, 002, 011, 013, 020, 021, 022, 030, 033, 036, 040, 059, 070, 071, 072

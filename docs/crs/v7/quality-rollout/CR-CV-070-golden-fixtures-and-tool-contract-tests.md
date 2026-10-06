# CR-CV-070 — Fixture vàng và kiểm thử hợp đồng với phiên bản công cụ (GitNexus, CodeGraph)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-070 |
| **Tên** | Chụp mẫu đầu ra thật của `gitnexus` và `codegraph`, lưu thành fixture vàng nhỏ, kiểm thử parser theo từng phiên bản (gitnexus 1.6.9, codegraph 1.4.1), phát hiện trôi định dạng và `schemaVersion`, chạy trong CI |
| **Loại** | Chất lượng |
| **Priority** | 🔴 P0 |
| **Effort** | Medium (4 đến 6 ngày: repo mẫu, script chụp, parser test, guard phiên bản, CI hai tầng) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-001 (khung `codeintel.*`, danh sách phiên bản hỗ trợ), CR-CV-002 (parser GitNexus), CR-CV-003 (parser CodeGraph), CR-CV-005 (parser `detect-changes`) |
| **Mở khoá** | CR-CV-071 (benchmark dùng cùng bộ chụp), CR-CV-072 (fuzz parser), CR-CV-073 |
| **Tác động** | `agent/` (thư mục fixture, test vitest, script chụp, guard phiên bản trong `codeintel.status`); `backend-go/services/code-intel-service` (`testdata/` cho kết quả vàng phía service); `.github/workflows/` (workflow mới); không chạy `analyze` ở máy người dùng |

---

## 1. Bối cảnh và vấn đề

1. Toàn bộ dữ liệu v7 đến từ hai CLI của bên thứ ba mà ta không kiểm soát định dạng. README v7 mục 7 đã nêu rủi ro "`gitnexus cypher` trả markdown: parse dễ vỡ ... hoặc phiên bản công cụ đổi định dạng". Cần khoá hợp đồng bằng mẫu thật, không bằng trí nhớ.
2. Đã chạy thử **chỉ-đọc** ngày 2026-10-05 (gitnexus 1.6.9 từ `/usr/lib/node_modules/gitnexus`, codegraph 1.4.1 từ `~/.codegraph/versions/v1.4.1/bin`). Hình dạng quan sát được, **rộng hơn những gì nghiên cứu 04 ghi**, và có vài điểm dễ vỡ chưa ai ghi:

| Lệnh | Đầu ra quan sát | Điểm dễ vỡ |
|---|---|---|
| `gitnexus --version` | `1.6.9` | |
| `gitnexus cypher -r <repo> "<q>"` có hàng | `{ "markdown": "\| a \| b \|\n\| --- \| --- \|\n\| ... \|", "row_count": n }` | ô là chuỗi; mảng hiện là `[]` (ví dụ `c.keywords` cho `[]`) |
| `gitnexus cypher` **không có hàng** | mảng JSON trần `[]`, **không** phải `{markdown,row_count:0}` | hai hình dạng cho cùng một lệnh |
| `gitnexus cypher` lỗi truy vấn | `{ "error": "Prepare failed: Binder exception: ..." }` với **exit code 0** | không thể dựa vào mã thoát |
| `gitnexus cypher` ghi (`CREATE ...`) | `{ "error": "Write operations (CREATE, DELETE, SET, MERGE, REMOVE, DROP, ALTER, COPY, DETACH) are not allowed..." }` | công cụ tự chặn ghi, **nhưng `CALL show_tables() RETURN *` vẫn chạy** (bảng markdown): vẫn cần whitelist của ta, README v7 mục 6 đúng |
| `gitnexus cypher` thiếu `-r` khi registry nhiều repo | **stack trace JS** trên stderr (`Multiple repositories indexed. Specify which one with the "repo" parameter. Available: ...`), không phải JSON | lỗi dạng văn bản, kèm đường dẫn nội bộ của công cụ |
| `gitnexus context -r <repo> -l n <name>` | JSON: `status` (`found`), `symbol{uid,name,kind,filePath,startLine,endLine}`, `epistemic`, `incoming{<kiểu cạnh snake_case>: [{uid,name,filePath}]}`, ... ; có trạng thái `ambiguous` + `candidates` (nghiên cứu 04) | khoá nhóm cạnh là snake_case động (`has_method`, `accesses`...) |
| `gitnexus impact -r <repo> <target>` | JSON: `target{id,name,type,filePath}`, `direction`, `impactedCount`, `risk`, `epistemic`, `summary{direct,processes_affected,modules_affected}`, `byDepthCounts`, `affected_processes[]`, `affected_modules[{name,hits,impact}]`, `byDepth{"1":[{depth,id,...}]}` | không tìm thấy: `{ "error": "Target '...' not found", "target":{...}, "impactedCount":0, "risk":"UNKNOWN" }` với **exit code 0** |
| `gitnexus query -r <repo> -l n "<q>"` | JSON: `processes[]`, `process_symbols[]`, `definitions[{id,name,filePath,startLine,endLine}]` | |
| `gitnexus detect-changes` (`-s unstaged\|staged\|all\|compare`, `-b`, `-r`, `-l`) | **văn bản**: `Changes: 2 files, 2 symbols`, `Affected processes: 0`, `Risk level: low`, `Changed symbols:` rồi các dòng `  Symbol <tên> → <tệp>`; `--help` không có tuỳ chọn JSON | phải parse văn bản, hoặc dùng cypher/MCP; hình dạng này không ổn định |
| `gitnexus status` / `gitnexus list` | văn bản (có biểu tượng, có `⚠️ stale (re-run gitnexus analyze)`, `Indexed commit`, `Current commit`) | văn bản người đọc; không dùng để máy đọc |
| `<repo>/.gitnexus/meta.json` | JSON: `repoPath`, `lastCommit`, `indexedAt`, `branch`, `remoteUrl`, `stats{files,nodes,edges,communities,processes,embeddings}`, `capabilities`, `schemaVersion` (5), `cjkSegmentation`, `cacheKeys`, `fileHashes` (lớn) | `stats.communities` (10 252) lệch với đếm bằng truy vấn (9 039), chưa rõ nguyên nhân |
| `codegraph --version` / `status --json` | JSON: `initialized`, `version`, `projectPath`, `lastIndexed`, `fileCount`, `nodeCount`, `edgeCount`, `dbSizeBytes`, `journalMode`, `nodesByKind{}`, `languages[]`, `pendingChanges{added,modified,removed}`, `worktreeMismatch`, `index{builtWithVersion,builtWithExtractionVersion,currentExtractionVersion,reindexRecommended,state,pendingRefs}` | số liệu đổi theo từng lần đồng bộ (đã lệch `nodeCount` 295 761 → 295 910 trong cùng ngày) |
| `codegraph query <s> --json` | `[{ "node": {id,kind,name,qualifiedName,filePath,language,startLine,endLine,startColumn,endColumn,signature,visibility,isExported,isAsync,isStatic,isAbstract,updatedAt}, "score": n }]`; không có kết quả: `[]` | `id` là hash không ổn định giữa lần index (README v7 3.4) |
| `codegraph callers\|callees <s> --json` | `{ "symbol": "...", "callers"\|"callees": [{name,kind,filePath,startLine}] }` | **không tìm thấy**: văn bản có mã màu ANSI (`ℹ Symbol "x" not found`) dù có `--json` |
| `codegraph impact <s> --json` | `{ "symbol", "depth", "nodeCount", "edgeCount", "affected": [{name,kind,filePath,...}] }` | nghiên cứu 06 ghi "chưa xác nhận"; nay đã có JSON |
| `codegraph files --json` | `[{path,language,nodeCount,size}]` | rất lớn trên Orca (15 773 file) |
| `codegraph explore`, `node` | văn bản (mã nguồn có số dòng) | không dùng để vẽ |
| SQLite `.codegraph/codegraph.db` | bảng `schema_versions` (đọc chỉ-đọc: `(1,"Initial schema")`, `(8,"Initial schema includes all migrations")`), `project_metadata` (`index_state`, `indexed_with_version`, `indexed_with_extraction_version`=24,...) | tên khoá nội bộ có thể đổi; chỉ đọc nếu CR-CV-003 chọn phương án C |

3. Chưa có bất kỳ fixture hay test hợp đồng nào cho các lệnh này trong repo (`find` thư mục `fixtures`/`testdata`/`__fixtures__` chỉ thấy `tests/e2e/fixtures`, `desktop/src/main/daemon/fixtures`, `desktop/src/main/startup/__fixtures__`, `backend-go/.../mcpserver/{prompts,tools}/testdata`). Quy ước có sẵn: `__fixtures__` ở TypeScript, `testdata` ở Go.
4. CI hiện **không chạy test của `agent/`** (xem README feature mục 5): không workflow nào nhắc `agent/`; `pr.yml` gọi vitest với `config/vitest.config.ts` ở gốc repo, file không tồn tại. `agent/vitest.config.ts` có `include: ['src/**/*.test.ts']`. Nên CR này phải thêm job chạy, nếu không test parser chỉ là trang trí.
5. Hai bản `agent-tool-registry.ts` (`agent/src/relay`, `desktop/src/relay`) cùng tồn tại; chưa rõ bản nào chạy trên dev server. CR-CV-001 chốt; CR này đặt fixture theo thư mục của bản đó.

## 2. Giải pháp đề xuất

### 2.1 Hai tầng hợp đồng, một nguyên tắc

| Tầng | Hợp đồng | Fixture | Test | Chặn PR |
|---|---|---|---|---|
| C1 | đầu ra thô của CLI → parser của agent | mẫu thô theo phiên bản công cụ (2.3) | vitest ở `agent/` | Có |
| C2 | kết quả `codeintel.*` của agent → collector của service (`CodeIntelResult<T>`, README v7 3.2) | JSON vàng đã chuẩn hoá (2.5) | Go test ở `code-intel-service` **và** vitest của agent đối chiếu cùng tệp | Có |
| C3 | service → gateway → UI | thuộc CR-CV-040 (view) và CR-CV-050 | | |

Nguyên tắc: một tệp vàng C2 do agent **sinh** từ fixture C1 (agent test so khớp) và service **đọc** (collector test chuẩn hoá). Một thay đổi hình dạng làm đỏ cả hai phía, buộc sửa cùng PR.

### 2.2 Repo mẫu nhỏ làm nguồn chụp (mới)

Lý do: Orca thay đổi từng commit, chụp từ Orca cho fixture lớn, không lặp lại được, và `gitnexus analyze` trên Orca mất nhiều phút. Thay vào đó:

- Một cây mã nhỏ có chủ đích, ~15 đến 25 tệp, nhúng trong `agent/src/relay/codeintel/__fixtures__/mini-repo/` (đường dẫn cuối do CR-CV-001 chốt, đề xuất theo cấu trúc agent): Go (kiểu hexagonal `internal/{domain,usecase,adapter}` + một `.proto` + một `migrations/*.sql`), TypeScript (một component và một hàm gọi kênh), một route HTTP, hai symbol **trùng tên** ở hai tệp (để có `ambiguous`), một hàm có chuỗi chứa `|`, xuống dòng và dấu nháy, tên có Unicode.
- Không có `.gitnexus/` hay `.codegraph/` trong git (`.gitignore`); chỉ có **mã nguồn**. Script chụp dựng chỉ mục vào thư mục tạm.
- Mẫu gây lỗi cố ý (đối kháng): symbol tên chứa `|`, chuỗi chứa `\n`, dòng rất dài, tệp rỗng, tên tệp có khoảng trắng, một commit để `detect-changes` có hunk.

Hệ quả: `gitnexus analyze` và `codegraph init/index` **chỉ** chạy trên repo mẫu này, trong thư mục tạm, ở máy phát triển hoặc CI. Không bao giờ chạy trên repo người dùng ngoài `codeintel.reindex` có kiểm soát (README v7 mục 6).

### 2.3 Bố cục lưu fixture

```
agent/src/relay/codeintel/__fixtures__/
  mini-repo/                         # mã nguồn mẫu (2.2)
  gitnexus/
    1.6.9/
      MANIFEST.json
      cypher-rows.json               # {markdown,row_count}
      cypher-empty.json              # []
      cypher-error.json              # {error}
      cypher-pipe-newline-cells.json # ô có | và \n
      cypher-communities-array.json  # ô là mảng/chuỗi 'comm_x' có nháy đơn
      context-found.json  context-ambiguous.json  context-not-found.json
      impact-found.json   impact-not-found.json
      query-found.json
      detect-changes-unstaged.txt    # văn bản; kèm biến thể CRLF được sinh khi chạy test
      status-stale.txt  list.txt
      meta.json                      # đã loại fileHashes, cacheKeys
      no-repo-flag.stderr.txt        # "Multiple repositories indexed"
  codegraph/
    1.4.1/
      MANIFEST.json
      status.json  query.json  query-empty.json
      callers.json  callees.json  impact.json
      callers-not-found.ansi.txt     # đầu ra văn bản có ANSI
      files.head.json                # chỉ 20 mục đầu, đã cắt
      sqlite-schema.json             # schema_versions + project_metadata, nếu CR-003 đọc SQLite
```

`MANIFEST.json` mỗi thư mục phiên bản:

```jsonc
{ "tool": "gitnexus", "toolVersion": "1.6.9",
  "capturedAt": "2026-10-05T…Z", "hostOs": "linux-x64", "node": "…",
  "sourceRepo": "mini-repo", "sourceCommit": "<sha>",
  "markers": { "metaSchemaVersion": 5 },            // codegraph: {"extractionVersion":24,"schemaVersions":[1,8]}
  "files": { "cypher-rows.json": { "argv": ["cypher","-r","<repo>","<tham số hoá>"], "sha256": "…", "bytes": 412 } } }
```

Ghi `argv` dạng mẫu (không có đường dẫn tuyệt đối thật; `<repo>` và `<tmp>` là chỗ giữ), băm SHA-256 từng tệp, kích thước. **Ngân sách kích thước**: mỗi tệp ≤ 20 KiB, mỗi thư mục phiên bản ≤ 300 KiB, cắt bằng `-l`/`LIMIT`/`head`; test khẳng định (`TestFixtureBudget`). Không bao giờ lưu mẫu từ Orca, không lưu `fileHashes`, token hay đường dẫn thật của dev server (script chụp chạy bộ che; test quét `/home/`, `/opt/`, `C:\`, chuỗi giống token).

### 2.4 Script chụp (mô tả, chưa viết)

`agent/scripts/capture-codeintel-fixtures.mjs` (tên theo nội dung, không `utils`): 

1. Kiểm `gitnexus --version` và `codegraph --version` trùng phiên bản mục tiêu truyền vào (`--tool gitnexus@1.6.9`); không khớp thì dừng.
2. Sao `mini-repo/` ra thư mục tạm, `git init` + một commit, dựng chỉ mục bằng lệnh của chính công cụ **chỉ trong thư mục tạm** (đây là lần duy nhất `analyze`/`init` chạy; đã được người triển khai chủ động gọi, không có trong dòng sản phẩm).
3. Chạy **đúng tập lệnh trong whitelist** của CR-CV-001 (cùng hằng số `argv` mà parser dùng), ghi stdout, stderr và mã thoát vào tệp, rồi ghi `MANIFEST.json`.
4. Chạy bộ che (đường dẫn tuyệt đối → `<tmp>`, thời gian → giữ định dạng nhưng cố định, hash commit → giữ vì là của repo mẫu).
5. In `git diff --stat` để người duyệt thấy chính xác cái gì đổi khi chụp lại.

Chụp lại là hành động có chủ đích (PR riêng, nêu phiên bản), không tự động trong CI. Bản chụp thêm `*.expected.json` (đầu ra chuẩn hoá của parser, 2.5) do người duyệt kiểm tay lần đầu.

### 2.5 Kiểm thử parser theo phiên bản (C1) và kết quả vàng (C2)

Danh sách phiên bản hỗ trợ là **dữ liệu**, một nơi duy nhất trong agent (CR-CV-001): `SUPPORTED_TOOL_VERSIONS = { gitnexus: ['1.6.9'], codegraph: ['1.4.1'] }` cùng `SUPPORTED_SCHEMA_MARKERS = { gitnexus: { metaSchemaVersion: [5] }, codegraph: { extractionVersion: [24] } }`.

| Test (vitest, `agent/`) | Nội dung |
|---|---|
| `TestEverySupportedVersionHasFixtures` | mỗi phiên bản trong `SUPPORTED_TOOL_VERSIONS` có thư mục fixture và `MANIFEST.json` hợp lệ; thêm phiên bản mà thiếu fixture thì đỏ (cùng tinh thần v6 CR-REQ-025 D5) |
| `parseCypherOutput` | `{markdown,row_count}` → hàng; `[]` → 0 hàng; `{error}` → lỗi có mã `CODEINTEL_TOOL_FAILED`; ô chứa `|`, `\n`, dấu nháy; chuỗi `'comm_x'` bỏ nháy; số cột khớp tiêu đề; CRLF |
| `parseContext`, `parseImpact`, `parseQuery` | `found`, `ambiguous` (có `candidates`), `not found`/`risk UNKNOWN`; khoá nhóm cạnh động; thiếu khoá bắt buộc → `format_drift` |
| `parseDetectChanges` | văn bản → danh sách symbol + số tệp + mức rủi ro; biến thể CRLF; số liệu bằng 0; dòng lạ → `format_drift`, không bỏ qua |
| `parseCodegraphJson` | `status`, `query`, `callers`, `callees`, `impact`; `[]`; đầu ra ANSI "not found" (dù có `--json`) → coi là "không có kết quả" chứ không phải lỗi cú pháp JSON |
| Lỗi quy trình | thiếu `-r` (stack trace) → `CODEINTEL_REPO_NOT_REGISTERED` hoặc `CODEINTEL_TOOL_FAILED` đúng theo bảng, **không** rò đường dẫn nội bộ của công cụ trong thông điệp |
| Mã thoát 0 + `error` | mọi lệnh phải kiểm nội dung, không chỉ mã thoát |
| `TestParserGolden` | với mỗi tệp thô có `*.expected.json`: đầu ra chuẩn hoá khớp từng byte (khoá sắp xếp ổn định) |
| `TestResultMatchesServiceGolden` (C2) | kết quả `codeintel.*` dựng từ fixture bằng `CodeIntelResult` khớp tệp ở `backend-go/services/code-intel-service/testdata/agent-results/*.json` (đọc bằng đường dẫn tương đối từ gốc repo) |

| Test (Go, `code-intel-service`) | Nội dung |
|---|---|
| `TestCollectorNormalizesAgentGolden` | mỗi tệp trong `testdata/agent-results/` qua collector/chuẩn hoá (CR-CV-020, 021) cho `SymbolRef.key` ổn định, `truncated`/`totalCount` đúng, kind chuẩn hoá chữ thường |
| `TestAgentResultSchemaVersionTolerance` | trường lạ ở kết quả agent (phiên bản agent mới hơn) được bỏ qua; thiếu trường bắt buộc bị từ chối với lỗi rõ |
| `TestToolVersionRecorded` | mọi kết quả mang `sources[].version` và `indexedAt` |

### 2.6 Phát hiện trôi định dạng và `schemaVersion` khi chạy

Hai lớp, cả hai ở agent:

1. **Kiểm phiên bản trong `codeintel.status`.** Đọc `gitnexus --version`, `.gitnexus/meta.json.schemaVersion`; `codegraph status --json` trường `version`, `index.builtWithExtractionVersion`, `index.currentExtractionVersion`, `index.reindexRecommended`; (nếu CR-003 đọc SQLite) tối đa `schema_versions.version`. Đối chiếu `SUPPORTED_*`:

| Trạng thái | Điều kiện | Hành vi |
|---|---|---|
| `verified` | phiên bản và marker nằm trong tập hỗ trợ | phục vụ bình thường |
| `untested` | cùng major, patch/minor lạ, marker khớp | vẫn phục vụ; `status.warnings[]` có `tool_version_untested`; metric (CR-CV-071) |
| `incompatible` | major khác, hoặc marker (`schemaVersion`, `extractionVersion`) khác tập hỗ trợ | **không** phục vụ lệnh của công cụ đó: `CODEINTEL_TOOL_UNAVAILABLE` với `detail: "schema_version_unsupported"`, kèm phiên bản đã thấy và đã hỗ trợ |
| `reindexRecommended` | `index.reindexRecommended=true` hoặc `builtWithExtractionVersion != currentExtractionVersion` | phục vụ, `stale`/cảnh báo `index_built_with_old_extraction`; gợi ý `codeintel.reindex` |

2. **Kiểm hình dạng ở mỗi parser (shape guard).** Parser kiểm khoá bắt buộc, kiểu, và cột của bảng markdown. Sai hình dạng thì không đoán: trả `CODEINTEL_TOOL_FAILED` với `detail: "format_drift"`, tên lệnh (hằng số whitelist), phiên bản công cụ, và **không** kèm nội dung đầu ra thô; tăng `orca_codeintel_tool_format_drift_total{tool,command}` (CR-CV-071). Ghi log một lần mỗi `(tool, version, command)`.

Hệ quả cho kiểm thử: bất kỳ lần nâng phiên bản nào phải đi qua một PR gồm (a) chụp fixture mới, (b) cập nhật `SUPPORTED_*`, (c) chạy lại toàn bộ test.

### 2.7 CI

| Job | Chạy khi | Nội dung | Chặn |
|---|---|---|---|
| `code-intel-contract` (workflow mới `.github/workflows/code-intel-contract.yml`) | PR đụng `agent/src/relay/codeintel/**`, `backend-go/services/code-intel-service/**`, `backend-go/proto/orca/codeintel/**`, chính workflow | `pnpm --filter orca-agent exec vitest run src/relay/codeintel` (C1 + C2 phía agent) và `go test ./...` ở `code-intel-service` với `testdata/agent-results` (C2 phía Go). Không cần cài GitNexus/CodeGraph | **Có** |
| `code-intel-live-contract` (cùng workflow, `schedule` hằng đêm + `workflow_dispatch`) | định kỳ | Cài **bản mới nhất** của công cụ (GitNexus: gói npm `gitnexus`, như máy khảo sát cài ở `/usr/lib/node_modules/gitnexus`; CodeGraph: bộ cài riêng của nó, chưa xác minh cách cài không tương tác trong CI), dựng chỉ mục repo mẫu, chạy lại script chụp vào thư mục tạm, **so sánh với fixture đã commit**; khác thì mở báo cáo (artifact), không chặn | Không |
| Kiểm tồn tại workflow | trong cùng workflow | một bước khẳng định `agent/` được test: `pnpm --filter orca-agent exec vitest list` có test codeintel, tránh trường hợp "CI xanh vì không chạy gì" |

Số liệu: độ dài chạy ước lượng vài phút cho tầng chặn; tầng live ≥ 10 phút vì dựng chỉ mục; **chưa đo**.

### 2.8 Cách lưu mẫu lớn (không lưu trong git)

Mẫu lớn từ Orca (benchmark ở CR-CV-071, đo kích thước `cypher` tổng quan, `files --json` đầy đủ) **không** vào repo: script chụp có chế độ `--bench` ghi ra thư mục ngoài git (artifact CI hoặc `$TMPDIR`), kèm `MANIFEST` (phiên bản, commit, số node/edge, kích thước byte, thời gian). Bản kê này có thể đính vào PR dưới dạng tóm tắt, không phải dữ liệu.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Chụp từ repo mẫu bé, không từ Orca | Ổn định, nhỏ, có ca đối kháng cố ý; Orca đổi theo từng commit |
| D2 | Tầng offline chặn PR, tầng live không chặn | Công cụ ngoài không ổn định; nhưng phải biết sớm |
| D3 | Danh sách phiên bản hỗ trợ là dữ liệu và có test "mỗi phiên bản có fixture" | Thêm phiên bản mà quên fixture thì CI đỏ |
| D4 | Không tin mã thoát; kiểm nội dung (`error`, `[]`, ANSI) | Quan sát: lỗi trả `error` kèm exit 0; "not found" của `--json` là văn bản |
| D5 | Trôi định dạng → lỗi rõ + metric, không đoán | Kết quả sai lặng lẽ khó phát hiện hơn lỗi |
| D6 | `untested` vẫn phục vụ, `incompatible` thì không | Cân bằng giữa dùng được khi công cụ vá nhỏ và an toàn khi đổi schema |
| D7 | Một tệp vàng C2 dùng chung cho TS và Go | Thay đổi hợp đồng agent↔service làm đỏ hai phía cùng lúc |
| D8 | Không lưu `fileHashes`, token, đường dẫn thật | Mẫu vàng không được là kênh rò dữ liệu |

## 4. Tiêu chí chấp nhận

- [ ] `mini-repo/` và script chụp có mặt; script từ chối chạy khi phiên bản công cụ không khớp, và chỉ chạy `analyze`/`init` trong thư mục tạm.
- [ ] Fixture gitnexus 1.6.9 và codegraph 1.4.1 đủ danh sách ở 2.3, mỗi thư mục có `MANIFEST.json` với `argv` mẫu, `sha256`, `bytes`; `TestFixtureBudget` xanh (≤ 20 KiB/tệp, ≤ 300 KiB/phiên bản).
- [ ] Test quét fixture không có `/home/`, `/opt/`, `C:\`, chuỗi giống token, hay `fileHashes`.
- [ ] Parser xử lý đúng cả hai hình dạng rỗng của `cypher` (`[]` và `{markdown,row_count:0}`), ô chứa `|` và `\n`, `{error}` kèm exit 0, văn bản `detect-changes` (cả CRLF), đầu ra ANSI "not found" của `codegraph`.
- [ ] `TestEverySupportedVersionHasFixtures` xanh; thêm một phiên bản giả vào `SUPPORTED_TOOL_VERSIONS` mà không có fixture thì đỏ.
- [ ] `codeintel.status` trả `compatibility` ∈ `verified|untested|incompatible` đúng bảng 2.6; `schemaVersion` hoặc `extractionVersion` lạ → `CODEINTEL_TOOL_UNAVAILABLE` + `schema_version_unsupported`.
- [ ] Đầu ra sai hình dạng → `CODEINTEL_TOOL_FAILED` + `format_drift`, thông điệp không chứa đầu ra thô, metric tăng.
- [ ] Tệp vàng C2 trong `testdata/agent-results/` khớp phía agent (vitest) và phía service (Go test).
- [ ] Workflow `code-intel-contract.yml` chạy tầng chặn trên PR đúng đường dẫn và có bước khẳng định `agent/` thật sự được test.
- [ ] Workflow live chạy được qua `workflow_dispatch` (chưa kiểm chứng cách cài CodeGraph không tương tác trên runner).
- [ ] Không có tệp mẫu lớn nào từ Orca trong git (kiểm bằng giới hạn kích thước ở CI).

## 5. Kiểm thử

Bảng test ở 2.5, 2.6, 2.7 là toàn bộ kế hoạch kiểm thử của CR này. Thêm:

| Test | Nội dung |
|---|---|
| `fuzz` parser markdown (phối hợp CR-CV-072) | chuỗi ngẫu nhiên và biến dị từ `cypher-rows.json`: không panic/ném lỗi không bắt, luôn trả kết quả hoặc `format_drift` |
| Thử ngược | sửa tay một fixture (đổi tên cột, bớt một khoá) thì parser golden đỏ với thông điệp chỉ rõ tệp |

Chưa chạy: toàn bộ danh sách trên là kế hoạch.

## 6. Rủi ro và điểm chưa kiểm chứng

- Chưa chạy `gitnexus analyze` hay `codegraph init` trên repo mẫu (mọi quan sát ở mục 1 là từ chỉ mục có sẵn của Orca, chạy lệnh đọc). Chưa biết phiên bản công cụ có cho dựng chỉ mục trên cây tạm nhỏ không cần cấu hình, hay có ghi vào registry toàn cục `~/.gitnexus` (làm nhiễu registry 12 repo của máy khảo sát). Phải kiểm và nếu có thì dùng cờ/biến môi trường cô lập thư mục registry.
- Chưa biết `gitnexus query/trace` có đầu ra ổn định giữa phiên bản; `trace` chưa chạy thử (nghiên cứu 04 cũng ghi cần kiểm).
- Hình dạng `detect-changes` là **văn bản người đọc**; nếu công cụ đổi câu chữ (kể cả đổi `→`) parser vỡ. Phương án ổn định hơn (cypher/MCP) thuộc CR-CV-005; CR này chỉ khoá hình dạng hiện tại.
- CodeGraph có thể tự nâng cấp (thư mục `~/.codegraph/versions/v1.4.1/` ngụ ý nhiều phiên bản cạnh nhau): chưa kiểm chứng cơ chế; nếu có thì phiên bản trên dev server trôi mà ta không biết; guard 2.6 là lưới an toàn.
- Mã màu ANSI trong đầu ra "not found" phụ thuộc `TTY`/`NO_COLOR`; agent chạy với `stdio: pipe` (không TTY) thì có thể không có ANSI. Fixture hiện ghi lại hành vi quan sát trong môi trường chạy lệnh; cần chụp lại đúng điều kiện `spawn` của agent.
- `gitnexus` ngầm ghi stack trace JS khi thiếu `-r`; phiên bản sau có thể đổi sang JSON.
- Hai bản `agent-tool-registry.ts` (`agent/`, `desktop/`): nếu bản chạy thật là bản `desktop/`, vị trí fixture phải đổi.
- Windows/WSL: CRLF và đường dẫn `\` trong đầu ra chưa có mẫu thật; chỉ có biến thể CRLF sinh tay.
- SSH và remote (AGENTS.md): fixture chụp trên máy cục bộ; độ trễ và hành vi qua SSH không được kiểm ở đây.

## 7. Câu hỏi mở

1. Vị trí cuối cùng của fixture và parser: theo CR-CV-001 (`agent/src/relay/codeintel/` đề xuất). Có tách gói `agent-codeintel` riêng không?
2. Có dựng repo mẫu thành một repo git thật (submodule) hay giữ cây tệp thường trong repo chính?
3. Có chấp nhận tầng live dùng "bản mới nhất" (không pin) không, hay pin `~` minor? Pin làm mất khả năng báo sớm.
4. `detect-changes`: nếu CR-CV-005 chuyển sang `cypher`/MCP thì fixture văn bản có còn cần không?
5. Cách cài CodeGraph không tương tác trong CI (chưa biết gói/bộ cài).

## 8. Tham chiếu

- `agent/src/relay/agent-tool-registry.ts`, `agent/vitest.config.ts`, `agent/package.json`, `desktop/src/relay/agent-tool-registry.ts`
- `.gitnexus/meta.json` (khoá `schemaVersion`, `stats`, `capabilities`), `.codegraph/codegraph.db` (bảng `schema_versions`, `project_metadata`; đọc chỉ-đọc ngày 2026-10-05)
- `.github/workflows/pr.yml`, `.github/workflows/backend-go-issue-status-sync.yml`, `.github/workflows/backend-go-mcp-conformance.yml`, `backend-go/ci/mcp-conformance/run-go-conformance.sh`
- `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/testdata/tools_list.golden.json` (mẫu golden), `desktop/src/main/startup/__fixtures__`, `tests/e2e/fixtures`
- `docs/research/view-code/04-raw-data-and-pipeline.md`, `05-graph-schemas.md`, `06-gaps-risks-roadmap.md` mục 5, `02-local-mcp-interaction.md` mục 4
- `docs/crs/v7/README.md` mục 3.2, 3.4, 6, 7; `docs/crs/v6/request-quality-rollout/CR-REQ-025-e2e-tests-feature-flag-rollout.md` (mẫu D5)

# BE-CV-TASK-040-13: Kênh `codeIntel.subgraph`, `codeIntel.impact`, `codeIntel.symbol`

**From Solution:** BE-CV-SOL-040-codeintel-view-channels
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_codeintel_view_graph.go` (thêm), `channels_codeintel_view_graph_test.go`
**Depends on:** TASK-040-10; stub `GetSubgraph`, `GetImpact`, `GetSymbol` (BE-CV-SOL-020/021)
**Status:** [ ] TODO

---

## Context

UI-API 3.1: `subgraph` `center` = đúng một `{symbol}|{file}|{cluster}`, `depth` 1..3, `kinds ≤ 32`, `limit ≤ 1500`; `impact` `target` = `{key}` hoặc `{name, file?, kind?}`, `direction` mặc định upstream, `depth` 1..3; `symbol` `key ≤ 1024` **hoặc** (`name`,`file`), `includeSource` mặc định true, quyền `read_source`, trần phản hồi 320 KiB. `CODEINTEL_AMBIGUOUS_SYMBOL` mang `candidates ≤ 10` trong hậu tố (không diễn giải). `ImpactGraph` không có cạnh ở v7.

## Việc cần làm

1. `subgraphArgs{sel, Center subgraphCenter, Depth, Kinds, Limit}`: `subgraphCenter{Symbol,File,Cluster *string}` đúng một khác nil (`invalidParam("center","exactly_one")`); `File` qua `checkRelPath`; `Core.GetSubgraph` => `Env<SymbolGraph>`.
2. `impactArgs{sel, Target impactTarget, Direction, Depth, IncludeTests}`: `impactTarget{Key *string; Name *string; File *string; Kind *string}`: `key` xor `name` (với `file`/`kind` chỉ khi `name`); `key ≤ 1024`; `file` an toàn; map sang `oneof target` của proto; `Core.GetImpact`.
3. `symbolArgs{sel, Key, Name, File, IncludeSource *bool}`: `key` xor (`name` & `file`); `file` qua `checkRelPath` => `CODEINTEL_PATH_NOT_ALLOWED`; `Core.GetSymbol`; `spec.MaxResponse = 320<<10`; `source.text` nằm trong `data`.
4. Đăng ký ba kênh 20 s.

## Kiểm thử

- Validate union: hai `center`; `target` rỗng; `target.key`+`name`; `symbol` chỉ `name`; `file` `../a`, `%2e%2e/a`, `．．/a`; `depth` 0/4; `limit` 1501; `kinds` 33.
- Fake: `GetSymbol` trả proto có `proto.Size` = 320 KiB+1 => `RESPONSE_TOO_LARGE`; `AMBIGUOUS_SYMBOL` với 12 ứng viên => ≤ 10; `sourceOmitted` giữ enum chữ thường hoặc `null`; `incoming/outgoing` map rỗng `{}`; `risk` HOA.
- Metadata `x-orca-role`/tenant từ `Identity`; quyền `read_source` **không** kiểm ở gateway (test chỉ khẳng định gateway chuyển lỗi `NOT_AUTHORIZED`).
- Lệnh: `go test ./internal/adapter/wscompat/ -run 'CodeIntelView(Graph)'`.

## Tiêu chí hoàn thành

- [x] Union được kiểm "đúng một"; từ chối nêu tên trường.
- [x] `symbol` trần 320 KiB; đường dẫn xấu => `PATH_NOT_ALLOWED`.
- [x] Không cạnh trong `ImpactGraph`.

## Rủi ro và lưu ý

- `symbol` lộ mã nguồn qua WS: dựa hoàn toàn vào quyền `read_source` của service (gateway không có OPA, README gateway mục "No OPA"); phải có test từ chối ở service.
- `key` có thể chứa đường dẫn; không kiểm nhạy cảm ở gateway.

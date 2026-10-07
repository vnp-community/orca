# BE-CV-TASK-038-05: Quy tắc so sánh kênh `wscompat` (`ws.*`) và route HTTP (`route.*`)

**From Solution:** BE-CV-SOL-038-contract-diff
**Priority:** P2
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/contractdiff/{ws_channel_diff_rules.go, route_diff_rules.go}` và `_test.go` (mới)
**Depends on:** BE-CV-TASK-038-04
**Status:** [x] DONE

---

## Context

Kênh `wscompat` là hợp đồng giữa UI và gateway; tham số nằm trong struct cục bộ + `decodeArg[T](args, i)` (đã đọc `channels_git.go:57–76`). Catalog kênh do 032 trích bằng AST; ở đây chỉ so hai catalog. Route chỉ so khi `RouteCatalogExtractor` có (TASK-038-01).

## Việc cần làm

1. `ws_channel_diff_rules.go`: `DiffWsChannels(base, head []WsChannel) []ContractChange`: `ws.channel-removed` (nếu có kênh mới cùng RPC đích ⇒ `details.suggestRename`, vẫn `breaking`), `ws.channel-kind-changed`, `ws.arg-field-removed`, `ws.arg-field-renamed` (cùng vị trí và kiểu, đổi tag), `ws.arg-field-type-changed`, `ws.arg-position-changed` ⇒ `breaking`; `ws.arg-field-added` ⇒ `compatible`, hạ `risky` khi trường không `omitempty`; `ws.target-rpc-changed` ⇒ `risky`; `ws.channel-added` ⇒ `compatible`; kênh có `ArgsOpaque` (không đọc qua `decodeArg`) ⇒ `ws.args-opaque`, `unknown`. `kind` `ws-channel`/`ws-channel-arg`; `name` = tên kênh (+ `#<chỉ số>.<jsonName>` cho tham số).
2. `route_diff_rules.go`: `DiffRoutes(base, head []Route) []ContractChange`: xoá route/đổi method `breaking`; bỏ phần tử `responseKeys` `breaking`, thêm `compatible`; đổi `errorKeys`, thêm `middleware` `risky`; `kind` `route`/`route-field`. Nếu `RouteCatalogExtractor` không có ⇒ use case bỏ qua nhánh này (không gọi hàm), ghi `warnings`.
3. `consumers` để trống (C7: không có nguồn frontend tự động).
4. Kết quả xác định (sắp theo `name`, `ruleId`).

## Kiểm thử

- `go test ./services/code-intel-service/internal/domain/contractdiff/ -run 'Ws|Route'`: fixture `wscompat` (đổi tag `worktree`→`worktreeId`; xoá `paths`; thêm trường có/không `omitempty`; đổi kiểu `string`→`[]string`; kênh đổi `request`→`stream`; kênh `map[string]any` ⇒ `unknown`; thêm kênh; xoá kênh có kênh thay); route (nếu có catalog) từng ca; hoán vị ổn định.

## Tiêu chí hoàn thành

- [x] Mọi dòng bảng `ws.*` có test; `route.*` có test hoặc được đánh dấu "chờ extractor" trong PR.
- [x] `ws.args-opaque` không bao giờ ra `breaking`/`compatible`.

## Rủi ro và lưu ý

- Tỷ lệ `unknown` trên ~417 kênh chưa đo; đo ở golden (TASK-038-07) và ghi vào PR.

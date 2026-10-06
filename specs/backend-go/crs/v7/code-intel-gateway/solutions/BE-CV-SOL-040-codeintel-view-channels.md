# BE-CV-SOL-040-codeintel-view-channels: 15 kênh đọc `codeIntel.*` (status, structure, ..., contractDiff)

> **Proposed.** Chưa triển khai, chưa chạy test nào. Nối 15 kênh đọc vào `CodeIntelService` bằng runner/encoder của `BE-CV-SOL-040-codeintel-channel-foundation`.

**CR:** [CR-CV-040](../../../../../../docs/crs/v7/code-intel-gateway/CR-CV-040-api-gateway-codeintel-channels.md)
**Service:** `api-gateway` (`internal/adapter/wscompat`)
**TDD tham chiếu:** [`arch/02`](../../../../tdd/architecture/02-microservices-decomposition.md), [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (input validation, tenant), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (deadline), [`services/api-gateway.md`](../../../../tdd/services/api-gateway.md) mục 2

---

## Hợp đồng áp dụng

| Nguồn | Mục |
|---|---|
| `CONTRACT-codeintel-ui-api.md` | §3.1 (15 dòng: `status, structure, architecture, dataFlows, dataFlow, erd, storage, subgraph, impact, symbol, routes, changeOverlay, readingOrder, findings, contractDiff`), §2.2 phong bì, §2.4 giới hạn, §4 kiểu kết quả |
| `CONTRACT-codeintel-proto-and-data-map.md` | PQ-04 (selector), PQ-08 (`IndexStatus` đơn), PQ-10 (`architecture` = C4 + `containers`), PQ-12 (phong bì, `ifNoneMatch`), PQ-14 (2 MiB, `symbol` 320 KiB), PQ-19, §3.1 (RPC: `GetIndexStatus, GetStructure, GetArchitecture, ListDataFlows, GetDataFlow, GetErd, GetStorageMap, GetSubgraph, GetImpact, GetSymbol, GetRouteMap, GetChangeOverlay, GetReadingOrder, ListFindings, GetContractDiff`) |

## Lệch giữa CR và hợp đồng

| # | CR-CV-040 (2.4) | Hợp đồng | Theo |
|---|---|---|---|
| V1 | `architecture`: `level?`, `focus?` | `container?`, `includeHidden?`; kết quả `{containers, view}` (PQ-10) | hợp đồng |
| V2 | `dataFlow`: `format?` | `dialect?`, `detail?`, `maxServiceHops ≤ 8`, `maxSteps ≤ 200`, `includeSequence?`, `includeDfd?` | hợp đồng |
| V3 | `erd`: `service?`, `atCommit?` | `service?`, `dialect?`, `base?`, `head?`, `includeAccess?`, `includeInferred?`; không `service` => danh sách `services` | hợp đồng |
| V4 | `findings`: `kinds?`, `severity?` | `rules?`, `severities?`, `pathPrefix?`, `includeDismissed?`, `scope?`, `base?`, `limit ≤ 200` | hợp đồng |
| V5 | `impact.target` chuỗi; `symbol` `key` hoặc (`name`,`file`) | `target` = `{key}` hoặc `{name, file?, kind?}`; `center` của `subgraph` đúng một trong `{symbol}|{file}|{cluster}` | hợp đồng |
| V6 | `storage(worktreeId)` | `env?`, `includeLegacy?` | hợp đồng |
| V7 | `changeOverlay`: `base?`, `head?` | thêm `mode?`, `detail?` | hợp đồng |
| V8 | `status` có phong bì (ngầm) | `status` trả `IndexStatus` **phẳng**, không `data` (UI-API 3.1, 4.1) | hợp đồng |
| V9 | `contractDiff`: `base?`, `head?` | `base?`, `kinds?`, `detail?` (không `head`) | hợp đồng |
| V10 | mọi kênh `worktreeId` | `{projectId, worktreeId}` + `ifNoneMatch?` (PQ-04, PQ-12) | hợp đồng |

## Phụ thuộc chéo khu vực

| Hướng | Solution | Ghi chú |
|---|---|---|
| BE trước | `BE-CV-SOL-040-codeintel-channel-foundation` | runner, args, encoder, catalog |
| BE trước (RPC thật) | `BE-CV-SOL-012-index-status-aggregation` (`GetIndexStatus`), `BE-CV-SOL-020/021` (structure, subgraph, impact, symbol, routes), `BE-CV-SOL-033-c4-component-view` (architecture), `BE-CV-SOL-034-data-flow-model`, `BE-CV-SOL-031-erd-model-and-access-scan`, `BE-CV-SOL-035-storage-map`, `BE-CV-SOL-036-change-overlay-pipeline` + `-reading-order-and-risk`, `BE-CV-SOL-037-structure-findings-and-dismissals`, `BE-CV-SOL-038-contract-diff` | kênh chưa có RPC giữ placeholder `CODEINTEL_UNAVAILABLE` (mỗi task nối ngay khi stub có mặt) |
| FE sau | `FE-CV-SOL-050-types-and-runtime-bridge`, 051–059 | cần cùng bảng tham số/kết quả |
| AG | không | |

Thứ tự §7.2: 040 (nền+đọc) đợt 2; `changeOverlay/readingOrder` cần 036 (đợt 3); `architecture/dataFlow(s)` đợt 4; `findings` đợt 5; `contractDiff`, `storage` đợt 5–6.

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: các file liệt kê ở SOL-040-foundation mục 1 (cùng phiên, 2026-10-06). Xác nhận: chưa có kênh `codeIntel.*`, chưa có `CodeIntelServiceClient`; `normalizeNilSlices` không sửa proto (`registry.go:227`); mẫu fake client `fakeOrchestrationClient` ở `channels_orchestration_test.go:15` (theo CR, chưa mở trong phiên này).

### Correction relative to CR-CV-040

| # | CR nói | Thực tế | Xử lý |
|---|---|---|---|
| C1 | "view JSON camelCase qua struct" (2.2) | encoder `protoreflect` (foundation 2.5) thay struct viết tay | mỗi kênh chỉ dựng request proto + gọi `encodeEnvelope` |
| C2 | `limit` "trong ngân sách" | hợp đồng chỉ cho trần: `dataFlows ≤ 100`, `subgraph ≤ 1500`, `routes ≤ 500`, `findings ≤ 200`; `structure.limit` **không có trần** trong hợp đồng | trần có thì từ chối vượt; `structure.limit` chỉ kiểm `>= 1`, trần do CR-021 gán (Q1) |

## 2. Giải pháp

### 2.1 Mẫu một kênh (ví dụ `structure`)

```go
type structureArgs struct {
    codeIntelSelector
    Path        string `json:"path,omitempty"`
    Depth       *int   `json:"depth,omitempty"`
    Limit       *int   `json:"limit,omitempty"`
    PageToken   string `json:"pageToken,omitempty"`
    IfNoneMatch string `json:"ifNoneMatch,omitempty"`
}
func (a structureArgs) validate() error { /* sel, checkRelPath("path"), checkBoundedInt("depth",1,3), limit>=1, token<=512, ifNoneMatch<=80 */ }

registerCodeIntelUnary(r, d, "codeIntel.structure", func(ctx context.Context, c codeIntelCaller, _ Identity, in structureArgs) (any, error) {
    resp, err := c.Core.GetStructure(ctx, &codeintelv1.GetStructureRequest{Selector: toProtoSelector(in.codeIntelSelector), Path: in.Path, Depth: int32OrZero(in.Depth), ...})
    if err != nil { return nil, err }
    return finishCodeIntelResponse(spec, resp, func() (json.RawMessage, error) { return encodeEnvelope(resp.GetMeta(), resp.GetData(), resp.GetNextPageToken()) })
})
```
Tên trường request proto là **chưa chốt** (do chủ sở hữu CR RPC gán); mọi task ghi "theo proto thật khi có".

### 2.2 Bảng tham số và kiểm tra gateway (đầu vào cho ba loại task)

| Kênh | Trường (ngoài sel, `ifNoneMatch ≤ 80`) | Kiểm gateway |
|---|---|---|
| `status` | `refresh?` | không envelope; trả `IndexStatus` phẳng; **không** `ifNoneMatch` |
| `structure` | `path?`, `depth?`, `limit?`, `pageToken?` | `path` an toàn; `depth` 1..3; `limit >= 1`; `pageToken ≤ 512` |
| `architecture` | `container?`, `includeHidden?` | `container ≤ 512` (không phải đường dẫn tuyệt đối: `checkRelPath`) |
| `dataFlows` | `triggerKind?`, `query? ≤ 128`, `service?`, `limit? ≤ 100`, `pageToken?` | `limit` 1..100 |
| `dataFlow` | `flowId`, `dialect?`, `detail?`, `maxServiceHops? ≤ 8`, `maxSteps? ≤ 200`, `includeSequence?`, `includeDfd?` | `flowId` bắt buộc (`checkOpaqueID` ≤ 512, ghi chú Q2); enum `dialect` postgres\|mysql, `detail` service\|component |
| `erd` | `service?`, `dialect?`, `base?`, `head?`, `includeAccess?`, `includeInferred?` | refs qua `checkGitRefLike`; enum `dialect` |
| `storage` | `env?`, `includeLegacy?` | `env` dev\|prod\|legacy |
| `subgraph` | `center` (đúng một `symbol`/`file`/`cluster`), `depth?`, `kinds?`, `limit?` | đúng một khoá `center`; `center.file` qua `checkRelPath`; `depth` 1..3; `kinds ≤ 32`; `limit` 1..1500 |
| `impact` | `target` (`{key}` xor `{name, file?, kind?}`), `direction?`, `depth?`, `includeTests?` | `key ≤ 1024` hoặc `name` bắt buộc; `file` an toàn; `direction` upstream\|downstream; `depth` 1..3 |
| `symbol` | `key? ≤ 1024` **hoặc** (`name`,`file`), `includeSource?` | đúng một dạng; `file` qua `checkRelPath` => `PATH_NOT_ALLOWED`; trần phản hồi 320 KiB |
| `routes` | `limit? ≤ 500`, `pageToken?` | `limit` 1..500 |
| `changeOverlay` | `base?`, `head?`, `mode?`, `detail?` | refs; `mode` worktree\|committed; `detail` summary\|full |
| `readingOrder` | `base?`, `head?` | refs |
| `findings` | `rules?`, `severities?`, `pathPrefix?`, `includeDismissed?`, `scope?`, `base?`, `limit? ≤ 200`, `pageToken?` | `rules ≤ 32`; `severities` ⊂ error\|warning\|info; `pathPrefix` an toàn nếu khác rỗng; `scope` all\|changed; `limit` 1..200 |
| `contractDiff` | `base?`, `kinds?`, `detail?` | `kinds` ⊂ proto\|ws-channel\|route\|migration; `detail` summary\|full |

Quy tắc chung: `ifNoneMatch` truyền nguyên văn; `notModified` do encoder xử lý; `nextPageToken` nguyên văn; `AMBIGUOUS_SYMBOL` (`candidates ≤ 10`) đi qua `codeIntelChannelError` không diễn giải. Kênh `architecture` trả `{containers, view}` trong `data`.

### 2.3 Tệp mới

`channels_codeintel_view_status.go` (status), `..._view_graph.go` (structure, subgraph, impact, symbol, routes), `..._view_sources.go` (architecture, dataFlows, dataFlow, erd, storage), `..._view_change.go` (changeOverlay, readingOrder, findings, contractDiff), `..._view_*_test.go`, và `registerCodeIntelViewChannels` được thêm vào `registerCodeIntelChannels`.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Gateway từ chối `limit`/`depth` ngoài khoảng, không kẹp | UI-API 2.4, D3 của CR |
| D2 | Union (`center`, `target`) kiểm "đúng một" ở gateway | tránh mơ hồ xuống service; lỗi nêu tên trường |
| D3 | `status` không `ifNoneMatch` | trả `IndexStatus` phẳng, không etag (UI-API 3.1) |
| D4 | `symbol` có trần phản hồi riêng 320 KiB | PQ-14 |
| D5 | Không cache ở gateway | cache là của service (CR-022) |

## 4. Phụ thuộc và thứ tự

TASK-040-10 (mẫu + status/structure/routes) trước; 11–14 song song sau 10; 15 (conformance) cuối. Mỗi task chỉ nối khi stub RPC tương ứng đã sinh, nếu không để placeholder và đánh dấu "chặn bởi CR-xxx" trong PR.

## 5. Kiểm thử

Mỗi kênh: bảng validate (biên, enum, union, đường dẫn); fake client (`CodeIntelServiceClient` nhúng interface) kiểm ánh xạ tham số, deadline 8/20 s, `Unimplemented` => `UNAVAILABLE`; mảng rỗng `[]`; không khoá snake_case; `etag`/`notModified`; kênh `symbol` > 320 KiB => `RESPONSE_TOO_LARGE`; cô lập tenant (metadata từ `Identity`, `tenantId` giả bị từ chối). Hai dialect cho ít nhất `status` và `structure`. Chưa chạy bất kỳ test nào.

## 6. Rủi ro và điểm chưa kiểm chứng

- Tên trường proto request/response chưa tồn tại; mọi ánh xạ là dự kiến.
- Độ trễ đọc lạnh có thể vượt 20 s (collector ~1,8 s mỗi lần gọi CLI theo CR); gateway trả `CODEINTEL_TIMEOUT` + `inProgress`, client thử lại.
- `symbol` lộ mã nguồn; quyền `read_source` do service thi hành (không kiểm ở gateway), test từ chối ở CR-013/072.
- SSH: không chạm dev server (D2 feature).

## 7. Câu hỏi mở

- **Q1.** Trần `structure.limit` (hợp đồng không nêu).
- **Q2.** Độ dài tối đa của `flowId`, `container`, `service` (dùng 512 tạm, quyết định của solution).
- **Q3.** `erd` không `service` trả `{services}`: encoder xử lý hai kiểu `data`; chủ proto CR-031 xác nhận là một message `oneof` hay hai RPC.

## 8. Tham chiếu

- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-ui-api.md` (§3.1, §4)
- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md` (PQ-04/08/10/12/14, §3.1)
- `/opt/repos/orca/backend-go/services/api-gateway/internal/adapter/wscompat/registry.go`
- `/opt/repos/orca/docs/crs/v7/code-intel-gateway/CR-CV-040-api-gateway-codeintel-channels.md`

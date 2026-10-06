# BE-CV-TASK-040-11: Kênh `codeIntel.architecture`, `codeIntel.dataFlows`, `codeIntel.dataFlow`

**From Solution:** BE-CV-SOL-040-codeintel-view-channels
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_codeintel_view_sources.go` (mới), `channels_codeintel_view_sources_test.go` (mới)
**Depends on:** TASK-040-10 (mẫu); stub `GetArchitecture` (BE-CV-SOL-033-c4-component-view), `ListDataFlows`, `GetDataFlow` (BE-CV-SOL-034-data-flow-model)
**Status:** [ ] TODO

---

## Context

UI-API 3.1: `architecture` => `Env<{containers: ContainerRef[]; view: C4ComponentView|null}>` (PQ-10: C4, không phải đồ thị cụm); `dataFlows` => `Env<{flows}>` + `nextPageToken`, `total`; `dataFlow` => `Env<{flow, sequence?, dfd?}>`. Lệch với CR: V1, V2.

## Việc cần làm

1. `architectureArgs{sel, Container, IncludeHidden, IfNoneMatch}`: `container ≤ 512`, không đường dẫn tuyệt đối; `Core.GetArchitecture`; `data` = `{containers, view}`; `view` vắng => `null`.
2. `dataFlowsArgs{sel, TriggerKind, Query ≤128, Service, Limit ≤100, PageToken, IfNoneMatch}`; `limit` 1..100; `Core.ListDataFlows`; `totalCount` lấy từ `meta` (UI ghi `total`).
3. `dataFlowArgs{sel, FlowID (bắt buộc), Dialect, Detail, MaxServiceHops ≤8, MaxSteps ≤200, IncludeSequence, IncludeDfd, IfNoneMatch}`; enum `dialect` postgres|mysql, `detail` service|component; `Core.GetDataFlow`; `sequence`/`dfd` vắng khi không yêu cầu.
4. Đăng ký ba kênh (20 s) trong `registerCodeIntelViewChannels`.

## Kiểm thử

- Validate: `limit` 0/101, `maxServiceHops` 9, `maxSteps` 201, `dialect` `oracle`, `flowId` rỗng, `query` 129 ký tự.
- Fake: tham số sang proto đúng; `flow.steps` rỗng => `[]`; `gaps`, `stores`, `relatedProcesses` luôn mảng; `completeness` giữ chữ thường; `notModified` không có `data`.
- Quét không có khoá snake_case; lỗi `CODEINTEL_SYMBOL_NOT_FOUND | {"kind":"flow"}` đi qua nguyên.
- Lệnh: `go test ./internal/adapter/wscompat/ -run 'CodeIntelView(Sources)'`.

## Tiêu chí hoàn thành

- [ ] Ba kênh đúng shape UI-API 4.4; `architecture` không trả đồ thị cụm.
- [ ] Giới hạn bị từ chối, không kẹp.

## Rủi ro và lưu ý

- `dataFlow` nặng (≤ 200 bước, sequence+dfd): dễ chạm 2 MiB; `RESPONSE_TOO_LARGE` là hành vi đúng, client giảm `maxSteps`.
- Chỉ nối khi RPC có mặt; nếu CR-033/034 trễ, giữ placeholder.

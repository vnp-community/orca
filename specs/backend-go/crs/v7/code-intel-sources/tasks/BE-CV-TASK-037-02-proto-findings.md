# BE-CV-TASK-037-02: Proto `codeintel_findings.proto` (`Finding`, `ListFindings`, `DismissFinding`)

**From Solution:** BE-CV-SOL-037-structure-findings-and-dismissals
**Priority:** P1
**Service:** `code-intel-service` · `proto`
**File:** `backend-go/proto/orca/codeintel/v1/codeintel_findings.proto` (mới); `backend-go/proto/orca/codeintel/v1/codeintel.proto` (sửa: hai dòng `rpc`)
**Depends on:** BE-CV-SOL-010, BE-CV-SOL-020 (`SymbolRef`, `SourceInfo`, `ResultMeta`, `WorktreeSelector`), BE-CV-TASK-036-02 (`IndexFreshness`)
**Status:** [x] DONE

---

## Context

Hợp đồng §2.1 dòng 15, §3.1; PQ-05/06. `Finding` (cấu trúc/tĩnh) tách khỏi `QualityFinding`. Solution thêm `DetectorStatus` (mới, additive).

## Việc cần làm

1. Message `Finding`, `Evidence{path, line, symbol}`, `Owner{source, names[], share}`, `FindingScope`, `FindingDismissal{by, at, reason, disposition, note}`, `DetectorStatus{name, state, code}`; field theo thứ tự UI §4.5; số field gán theo thứ tự khai báo và ghi vào comment đầu tệp (không đổi sau).
2. `ListFindingsRequest{selector=1, rules, severities, path_prefix, include_dismissed, scope (enum FindingScope: UNSPECIFIED=0, ALL, CHANGED), base_ref, page_size, page_token}`; `ListFindingsResponse{findings, next_page_token, total_count, dismissed_count, truncated, sources, index_freshness, detectors}`; `DismissFindingRequest{selector=1, finding_key, action (enum: UNSPECIFIED=0, DISMISS, RESTORE), disposition, reason, note}`; `DismissFindingResponse{finding_key, dismissed, disposition}`.
3. Thêm hai `rpc` vào `service CodeIntelService`; không RPC khác.
4. Import `codeintel_change_overlay.proto` cho `IndexFreshness` (không vòng: findings ← change_overlay một chiều; kiểm quy tắc import hợp đồng §2.1 cuối — nếu cấm, chuyển `IndexFreshness` sang `codeintel_common.proto` qua `BE-CV-SOL-020`).
5. Ghi trong comment `detectors` chờ hợp đồng (solution mục 4, L2).

## Kiểm thử

- `buf lint`, `buf breaking --against '.git#branch=main,subdir=backend-go/proto'`.
- Round-trip trong `internal/adapter/grpc/findings_proto_test.go` (mới): `Finding` đủ trường, `metrics` double, `dismissed`.

## Tiêu chí hoàn thành

- [x] `buf` xanh; enum có `_UNSPECIFIED = 0`.
- [x] Mọi trường UI §4.5 có chỗ trong proto.
- [x] Không đặt `ChangeOverlay`/`Finding` hai nơi.

## Rủi ro và lưu ý

- `scope` mặc định UI là `changed`; proto `UNSPECIFIED` phải được use case hiểu là `CHANGED` (ghi ở TASK-037-06).

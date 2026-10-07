# BE-CV-TASK-036-02: Proto `codeintel_change_overlay.proto` (`ChangeOverlay`, `GetChangeOverlay`, `GetReadingOrder`)

**From Solution:** BE-CV-SOL-036-change-overlay-pipeline
**Priority:** P0
**Service:** `code-intel-service` · `proto`
**File:** `backend-go/proto/orca/codeintel/v1/codeintel_change_overlay.proto` (mới); `backend-go/proto/orca/codeintel/v1/codeintel.proto` (sửa: hai dòng `rpc`)
**Depends on:** BE-CV-SOL-010, BE-CV-SOL-020 (`SymbolRef`, `FlowSummary`, `ClusterRef`, `ResultMeta`, `WorktreeSelector`)
**Status:** [x] DONE

---

## Context

Hợp đồng §2.1 dòng 14 giao toàn bộ message cho CR-036 (PQ-09). Hình dạng trường theo UI §4.3; số field do chủ sở hữu CR gán theo thứ tự khai báo CR (không đổi sau). Solution `reading-order-and-risk` dùng lại cùng tệp này.

## Việc cần làm

1. Tạo tệp `package orca.codeintel.v1`, import `codeintel_common.proto`. Khai báo: `ChangeOverlay` (`scope, emptyReason, changed_files, changed_symbols, affected_flows, affected_clusters, touched_tables, touched_contracts, uncovered_symbols, violations, reading_order, components, risk, index_freshness, limits`), `OverlayScope`, `ChangedFile`, `ChangedSymbol`, `ReadingStep`, `ComponentGroup`, `RiskAssessment`, `RiskReason`, `IndexFreshness`, `TouchedTable`, `TouchedContract` (có `compatibility`, `breaking`), `ViolationRef`, `OverlayLimits` (`truncated` map bool + `total_counts` map int64). Tên field snake_case; giá trị liệt kê là `string` chữ thường, trừ `RiskAssessment.level` chữ HOA (PQ-32).
2. `GetChangeOverlayRequest{selector=1, base_ref=2, head_ref=3, mode=4, detail=5, if_none_match=6}`, `GetChangeOverlayResponse{overlay=1, meta=2}`; `GetReadingOrderRequest{selector=1, base_ref=2, head_ref=3, if_none_match=4}`, `GetReadingOrderResponse{steps=1, components=2, meta=3}` (hợp đồng §3.1).
3. Thêm hai `rpc` vào `service CodeIntelService` (`codeintel.proto`). Không RPC nào khác.
4. `ViewKind`: thêm `VIEW_KIND_CHANGE_OVERLAY`, `VIEW_KIND_READING_ORDER` nếu thiếu (đề nghị chủ `BE-CV-SOL-020` duyệt).
5. Ghi số field vào comment đầu tệp ("chốt ở PR này, không đổi").
6. Sinh stub theo quy trình `BE-CV-SOL-010`.

## Kiểm thử

- `buf lint`; `buf breaking --against '.git#branch=main,subdir=backend-go/proto'` (gọi `buf` trực tiếp).
- Round-trip `proto.Marshal/Unmarshal` ở `internal/adapter/grpc/change_overlay_proto_test.go` (mới) với overlay đủ trường; kiểm `risk.level` giữ chữ HOA.

## Tiêu chí hoàn thành

- [x] `buf lint/breaking` xanh; mọi RPC khai báo có message.
- [x] Tên file đúng PQ-07; không `ChangeOverlay` ở `codeintel_graph.proto` (PQ-09).
- [x] Đối chiếu UI §4.3: mọi trường UI có trường proto tương ứng.

## Rủi ro và lưu ý

- Gateway ánh xạ proto → JSON camelCase (H1); mảng rỗng phải là `[]` (kiểm ở `BE-CV-SOL-040`).

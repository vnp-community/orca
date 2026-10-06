# BE-CV-TASK-021-06: `CollectView`: điều phối view, giải mã, hợp nhất, giới hạn, redaction mã nguồn

**From Solution:** BE-CV-SOL-021-agent-collector
**Priority:** P0
**Service:** `code-intel-service`
**File:** `.../internal/usecase/{collect_view.go,collect_view_decoding.go,symbol_source_redaction.go}` (mới) và test
**Depends on:** TASK-021-03, BE-CV-TASK-020-06, BE-CV-TASK-020-07
**Status:** [ ] TODO

---

## Context

Bảng 2.E: view ↔ method; `stale` không hạ; redactor fail-closed.

## Việc cần làm

1. `ViewReader` + `CollectView`; nhánh `status` song song (`errgroup`), lỗi nhánh này chỉ thêm `status_unavailable`.
2. Giải mã `data` → domain (SymbolGraph, ModuleGraph, ImpactGraph, RouteMap, ArchitectureGraph, SymbolDetail); `ValidateAgentSymbolRef`, `MergeSymbolSets/MergeEdges`, `Limit*`.
3. `STRUCTURE`: chuyển `SymbolGraph` thành `ModuleGraph` (C3).
4. `SymbolSourceRedactor`: không có cài đặt → `source=nil`, `source_omitted="sensitive_path"`.
5. `ResultMeta`: stale/truncated OR, `total_count`, `sources` có `line_base`.

## Kiểm thử

- `go test ./internal/usecase/ -run CollectView -race` với gateway giả và tệp vàng.
- Ca: 5 000 nút → 1 500, `truncated`, `total_count=5000`; `status` lỗi không làm hỏng view; `AMBIGUOUS_SYMBOL`; không redactor → không có `source`; mỗi view gọi đúng tập method.

## Tiêu chí hoàn thành

- [ ] Mọi view bảng 2.E có test.
- [ ] Kết quả xác định.

## Rủi ro và lưu ý

- Mapping `STRUCTURE` là phỏng đoán (Q1 solution).

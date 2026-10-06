# BE-CV-TASK-036-03: Domain `changeoverlay`: ánh xạ kết quả agent, phân loại đường dẫn, `IndexFreshness`

**From Solution:** BE-CV-SOL-036-change-overlay-pipeline
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/changeoverlay/{change_overlay.go, changed_file.go, changed_symbol.go, index_freshness.go, path_classification.go, change_set_mapping.go}` và `_test.go` (mới)
**Depends on:** BE-CV-TASK-036-01; BE-CV-SOL-020 (kiểu `SymbolRef` bản domain)
**Status:** [ ] TODO

---

## Context

Hàm thuần, stdlib, không I/O (arch/03). Chuyển `detectChanges` thô của agent (hợp đồng §4.8) sang mô hình overlay (bảng 2.C solution). Bảng ánh xạ là chỗ dễ sai nhất; test bằng bảng từng dòng.

## Việc cần làm

1. `change_overlay.go`, `changed_file.go`, `changed_symbol.go`: kiểu domain (`OverlayScope`, `ChangedFile`, `ChangedSymbol`, `IndexFreshness`, `OverlayLimits`) tách khỏi proto.
2. `change_set_mapping.go`: `MapChangeSet(raw AgentChangeSet) (ChangeSet, []MappingWarning)`: `status` raw `M,T,U→modified`, `A→added|untracked`, `D→deleted`, `R→renamed`, `C→copied`; `binary⇒added=removed=0`; `mappingConfidence` từ `indexed`/`driftedFromIndex`; `ChangedSymbol.changeKind` (`deleted→unknown`); `flows` đếm từ `affectedFlows[].changedSymbolKeys`; loại symbol `kind:"doc"` khỏi tập thực thi; `scope.mode` từ `head.includesUncommitted`. Giá trị raw lạ ⇒ `modified` + cảnh báo, không panic.
3. `path_classification.go`: `Classify(path) PathClass{IsTest, IsGenerated, IsDoc, Area}` theo bảng 2.C (generated: `backend-go/proto/gen/**`, `*.pb.go`, `*_grpc.pb.go`; test: `*_test.go`, `*.test.*`, `*.spec.*`, `__tests__`, `tests/`; doc: `*.md`, `docs/**`, `specs/**`, `guides/**`). Dùng gói `path` (không `filepath`) để chạy giống nhau trên Windows; đường dẫn có `\` hoặc `..` ⇒ coi là không an toàn, bỏ và cảnh báo.
4. `index_freshness.go`: `ComputeFreshness(index, headOID string, dirtyFiles int, unindexed []string, behindCount *int, now time.Time)`; thứ tự ưu tiên `missing > behind > dirty > fresh`; `now` truyền vào (không `time.Now()`); `unindexed` cắt 100.
5. Kết quả sắp xếp xác định: `changedFiles` theo `|added|+|removed|` giảm dần rồi `path`; `changedSymbols` theo `(path, startLine, key)`.

## Kiểm thử

- `go test ./services/code-intel-service/internal/domain/changeoverlay/...`: bảng trạng thái (7 mã raw + untracked), `mappingConfidence` 3 ca, `ComputeFreshness` đủ tổ hợp (missing/behind/dirty/fresh, dirty khi behind), phân loại đường dẫn (kể cả `a/b_test.go`, `proto/gen/go/x.pb.go`, `docs/x.md`, `frontend/src/a.test.tsx`), symbol `deleted`, fixture `renamed` và `unborn`.
- Ổn định: 100 hoán vị đầu vào ⇒ cùng JSON.

## Tiêu chí hoàn thành

- [ ] Mọi dòng bảng 2.C có test.
- [ ] Không import ngoài stdlib; không `time.Now()`.
- [ ] Giá trị lạ không panic.

## Rủi ro và lưu ý

- Mã raw `U` (unmerged) ánh xạ `modified` làm mất thông tin xung đột; ghi cảnh báo `unmerged_path`.

# BE-CV-TASK-037-03: Domain `structurefinding`: `Finding`, `finding_key`, quy tắc lớp hexagonal, vòng import

**From Solution:** BE-CV-SOL-037-structure-findings-and-dismissals
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/structurefinding/{finding.go, finding_key.go, layer_import_rules.go, import_cycle_findings.go}` và `_test.go` (mới)
**Depends on:** BE-CV-TASK-037-01
**Status:** [ ] TODO

---

## Context

Hàm thuần stdlib. `rule`→`kind` theo bảng PQ-06 (backend điền). Đầu vào từ `structuralFacts` đã giải mã (kiểu domain, không JSON).

## Việc cần làm

1. `finding.go`: `Finding`, `Evidence`, `Owner`, `Severity` (`error|warning|info`), `Origin` (`introduced|touched|preexisting|unknown`), `Confidence`; `KindOf(rule string) string`: `layer.*→layer_violation`, `cycle.import→dependency_cycle`, `hotspot.file→hotspot`, `dead.unused-export→dead_code`, `sql.missing-tenant-filter|sql.insert-missing-tenant→missing_tenant_id`, `sql.drop-policy|sql.disable-rls→rls_removed`; rule lạ ⇒ `""` (không lỗi; `rule` là chuỗi mở).
2. `finding_key.go`: `NewKey(rule, canonicalSubject string) string` = `rule + ":" + hex(sha256(subject))[:16]`; kiểm `len ≤ 128`; `ValidKey(string) bool` theo `^[a-z][a-z0-9.\-]{0,63}:[0-9a-f]{16}$`; hàm `LayerSubject(service, srcPkg, dstPkg)`, `CycleSubject(files)` (sắp từ điển, nối `\n`), `FileSubject(path)`, `SymbolSubject(key)`.
3. `layer_import_rules.go`: `LayerFindings(rows []ImportRow) []Finding`: tách `services/<svc>/internal/<layer>/<pkg>` từ đường dẫn; áp 3 quy tắc (error/warning/info); loại `_test.go`, `usecasetest/`, `testutil`, `cmd/`, `proto/gen`, `*.pb.go`; **khử trùng** `(tệp nguồn, thư mục đích)` rồi gom theo `(service, pkg nguồn, pkg đích)`; `evidence` = tệp nguồn sắp từ điển (không `line`); `titleKey` `codeintel.finding.layer.<rule>`; `params{service, from, to}`; `scope{service, layer}`; `confidence:"high"`.
4. `import_cycle_findings.go`: `CycleFindings(cycles [][]string) []Finding`: bỏ tệp lặp ở cuối, bỏ vòng một tệp, sắp tệp, `params{length}`; `FallbackCycles(g ModuleGraph, maxNodes int)` (Tarjan; chỉ khi ≤ 500 nút, ngược lại trả "không tính").
5. Sắp xếp kết quả xác định: `severity` giảm dần, `rule`, `subject`.

## Kiểm thử

- `go test ./services/code-intel-service/internal/domain/structurefinding/ -run 'Key|Layer|Cycle|Kind'`: fixture `structuralFacts-layerImports.json` ⇒ đúng 2 finding `layer.usecase-imports-adapter`, 0 `domain-imports-outer`, không `usecasetest`; thêm/bớt tệp cùng package không đổi `finding_key`; đổi package đổi khoá; 100 lần giống nhau; vòng có tệp lặp cuối ⇒ độ dài đúng; `KindOf` cho 10 rule của PQ-06.

## Tiêu chí hoàn thành

- [ ] Các ca trên xanh; không import ngoài stdlib.
- [ ] Không có số dòng trong `canonicalSubject`.

## Rủi ro và lưu ý

- Layout khác `backend-go/services/<svc>/internal/**` (service mới) không khớp quy tắc ⇒ bị bỏ im lặng; thêm test "đường dẫn lạ không panic".

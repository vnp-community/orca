# BE-CV-TASK-036-04: Chọn K symbol, cổng làm giàu mềm và fan-out `impact` theo ngân sách thời gian

**From Solution:** BE-CV-SOL-036-change-overlay-pipeline
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/changeoverlay/impact_selection.go`, `impact_selection_test.go` (mới); `internal/usecase/change_overlay_ports.go` (mới); `internal/usecase/impact_fanout.go`, `impact_fanout_test.go` (mới)
**Depends on:** BE-CV-TASK-036-03; BE-CV-SOL-021 (collector: typed client `Impact`, `DetectChanges`); BE-CV-SOL-030 (`RepoSourceReader.Log`)
**Status:** [x] DONE

---

## Context

`impact` ≈ 1,8 s/lệnh, agent giới hạn `gitnexus` ≤ 2 tiến trình/dev server (hợp đồng agent §2.3); gateway chờ 20 s (PQ-13). Fan-out phải theo ngân sách, phần dở dang hoàn tất nền (solution 036 C7).

## Việc cần làm

1. `impact_selection.go`: `SelectForImpact(symbols []ChangedSymbol, max int) (selected, rest []ChangedSymbol)`: loại `doc`/test/generated/`deleted`; ưu tiên `isExported`, tầng `usecase|domain` (theo đường dẫn `internal/usecase|domain`), `linesChanged` giảm dần, `key` tăng dần. `max` mặc định 25 (cấu hình `CODEINTEL_OVERLAY_IMPACT_MAX`, tên đề xuất; ghi vào PR hợp đồng §6.2).
2. `change_overlay_ports.go`: interface `ChangeSetSource` (`DetectChanges`), `ImpactSource` (`Impact`), `TouchedTableSource`, `TouchedContractSource`, `ViolationSource`, `ComponentIndex`, `CommitDistanceReader` (hiện thực trên `RepoSourceReader.Log`), cùng lỗi sentinel `ErrSourceUnavailable`. Mỗi nguồn mềm có hiện thực `Unavailable` trả `ErrSourceUnavailable` (dùng khi 031/033/037/038 chưa merge).
3. `impact_fanout.go`: `RunImpacts(ctx, src ImpactSource, selected []ChangedSymbol, budget time.Duration, concurrency int)`: tối đa 2 đồng thời; mỗi lời gọi `{target:{key}, direction:"upstream", depth:2, includeTests:true, limit:300}`; trả `map[key]ImpactResult` + `failures map[key]error` + `pending []key`. Khi hết `budget`, các lời gọi đang chạy **không bị huỷ**: chuyển sang goroutine nền gắn `context` riêng (100 s) và ghi kết quả vào hàm callback `onLateResult` (để use case lưu cache). `CODEINTEL_AMBIGUOUS_SYMBOL` ⇒ bỏ; mọi lỗi khác ⇒ `failures`.
4. Không `time.Sleep` trong logic; dùng `context` + đồng hồ truyền vào để test.

## Kiểm thử

- `go test ./services/code-intel-service/internal/domain/changeoverlay/ -run Selection` (ổn định qua hoán vị, đúng thứ tự ưu tiên, 40 symbol ⇒ 25 chọn + 15 còn lại).
- `go test ./services/code-intel-service/internal/usecase/ -run ImpactFanout` với `ImpactSource` giả và đồng hồ giả: không bao giờ > 2 lời gọi đồng thời (đếm bằng atomic); hết ngân sách ⇒ `pending` đúng và `onLateResult` được gọi sau; lỗi từng phần; `-race`.

## Tiêu chí hoàn thành

- [x] Giới hạn đồng thời 2 được test.
- [x] Hết ngân sách không làm mất kết quả muộn.
- [x] Các port có hiện thực `Unavailable` để use case chạy khi nguồn mềm vắng.

## Rủi ro và lưu ý

- Goroutine nền phải có giới hạn tổng (singleflight ở TASK-036-05) để không tích luỹ.
- Gọi agent mang tenant/dev server của người gọi; không dùng lại `ctx` của request cho nền (dùng `context.WithoutCancel` + deadline 100 s, giữ giá trị tenant).

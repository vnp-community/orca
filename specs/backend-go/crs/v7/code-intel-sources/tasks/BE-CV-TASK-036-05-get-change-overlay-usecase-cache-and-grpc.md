# BE-CV-TASK-036-05: Use case `GetChangeOverlay`, cache snapshot, handler gRPC

**From Solution:** BE-CV-SOL-036-change-overlay-pipeline
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/get_change_overlay.go`, `get_change_overlay_test.go` (mới); `internal/adapter/grpc/change_overlay_handler.go`, `change_overlay_handler_test.go` (mới); `cmd/server/main.go` (sửa: đăng ký handler)
**Depends on:** BE-CV-TASK-036-02, 036-03, 036-04, **036-09** (domain thứ tự đọc/rủi ro/giới hạn); BE-CV-SOL-012, 013, 022
**Status:** [x] DONE

---

## Context

Điều phối theo solution 036 §2.D. Quyền kiểm **trước** cache. Hợp đồng §3: guard nội bộ, tenant, cờ, OPA `read`, phân giải `selector`.

## Việc cần làm

1. `GetChangeOverlay.Execute`: cổng (cờ ⇒ `CODEINTEL_DISABLED`; quyền ⇒ `CODEINTEL_NOT_AUTHORIZED`; `selector` ⇒ binding/`RepoRef`); tính `mode`/`head` (`head_ref != ""` ⇒ committed).
2. Cache: khoá `(tenant, binding, "changeOverlay", head_oid, params_hash)`; chỉ ghi DB khi cây sạch; ngược lại bộ nhớ 30 s; huỷ khi nhận `index.changed`/`reindex.finished`. Singleflight theo khoá (nền 100 s); `if_none_match` khớp ⇒ `not_modified`.
3. Gọi `DetectChanges` một lần (`withClusters:true`, `includeUntracked:true`); `warnings:["unborn_head"]` ⇒ overlay rỗng `emptyReason:"unborn-head"`; lỗi agent ⇒ ánh xạ mã `CODEINTEL_*` (không đổi tên) và trả (xem Q1 solution).
4. `MapChangeSet` ⇒ `SelectForImpact` ⇒ `RunImpacts` với budget = deadline còn lại − 2 s ⇒ làm giàu mềm (mỗi nguồn lỗi ⇒ thêm vào `missingSources`, không lỗi) ⇒ gọi `BuildReadingOrder`, `ScoreRisk`, `GroupComponents`, `ApplyLimits` (036-06…09) ⇒ `IndexFreshness` (`CommitDistanceReader` qua `Log` giới hạn 200, vượt ⇒ hiển thị "200+").
5. `onLateResult` ⇒ tính lại overlay đầy đủ và ghi cùng khoá cache.
6. `detail=summary` ⇒ chỉ `scope`, `totalCounts`, `risk`, `components`, `indexFreshness`.
7. Co payload ≤ 2 MiB theo thứ tự cố định (2.D bước 9); vẫn vượt ⇒ `CODEINTEL_RESPONSE_TOO_LARGE`.
8. Handler: domain ⇄ proto, lỗi ⇒ `apperrors` với `CODEINTEL_X: …` đầu message (PQ-02); `CODEINTEL_TIMEOUT` có hậu tố `{"retryAfterMs":3000,"inProgress":true}`; `ResultMeta` phẳng. Đăng ký ở `main.go`.

## Kiểm thử

- `go test ./services/code-intel-service/internal/usecase/ -run ChangeOverlay -race` với fake: fixture `clean|dirty|untracked|renamed|unborn|drifted|3000-files` (từ TASK-036-01); impact lỗi một phần ⇒ `truncated.impact`, `tested:"unknown"`, `risk.incomplete`; lần gọi thứ hai sau `onLateResult` ⇒ đủ; quyền sai không chạm agent/cache (đếm lời gọi); hai tenant cùng khoá không đọc chéo; hai lời gọi đồng thời ⇒ một `DetectChanges`.
- 3 000 tệp: `truncated.files`, `totalCounts` đúng, `proto.Size ≤ 2 MiB`, điểm rủi ro bằng điểm tính trên đủ 3 000.
- `go test ./services/code-intel-service/internal/adapter/grpc/ -run ChangeOverlay`: mã lỗi, hậu tố JSON ≤ 2 KiB.
- Hai dialect: chỉ qua port snapshot của `BE-CV-SOL-022` (ma trận ở đó); ghi rõ nếu 022 chưa có.

## Tiêu chí hoàn thành

- [x] Toàn bộ §9 của solution 036-change-overlay-pipeline đạt trên fixture.
- [x] Test ổn định 100 lần (byte-by-byte) ở đầu ra overlay.
- [x] `buf breaking` xanh; kênh `codeIntel.changeOverlay` chưa đăng ký ở gateway (việc của `BE-CV-SOL-040-codeintel-view-channels`).

## Rủi ro và lưu ý

- Lỗi vòng phụ thuộc gói: `usecase` chỉ gọi domain qua hàm thuần, không import adapter.
- Ngân sách 20 s chưa đo (O-15); giữ hằng số có thể cấu hình.

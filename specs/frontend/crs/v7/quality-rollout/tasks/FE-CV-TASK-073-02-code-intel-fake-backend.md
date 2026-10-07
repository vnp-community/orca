# FE-CV-TASK-073-02: Fake backend `createFakeCodeIntelBackend` (cổng G4)

**From Solution:** [FE-CV-SOL-073](../solutions/FE-CV-SOL-073-flag-gating-and-web-e2e.md) mục 2.4
**Priority:** P0
**Area:** frontend / test-support
**File:** `frontend/src/renderer/src/test-support/code-intel-fake-backend.ts`, `code-intel-fixtures.ts` (mới/mở rộng) + `code-intel-fake-backend.test.ts`
**Depends on:** FE-CV-SOL-050-types-and-runtime-bridge (kiểu §4, `CodeIntelRpcContract`, bộ phân loại lỗi); không cần backend
**Status:** [x] DONE

## Context

- Đã xác minh mẫu `test-support/mcp-fake-backend.ts` (`Handlers`, `failures`, `calls`, `listeners`, `emit`, lỗi `${code}: ${msg}`) và `mcp-fixtures.ts`.
- **Cổng G4** (§7.1): mọi lens frontend bắt đầu bằng fake backend. Nếu SOL-050 đã tạo khung, mở rộng, **không** tạo bản thứ hai.
- Hợp đồng: U1 một object `args[0]`, `DisallowUnknownFields`, U3 cấm tham số danh tính, §2.3 mã lỗi trên `message`, §3.1 46 kênh, §4 kiểu, §5 push có `event`.

## Việc cần làm

1. Bộ xử lý có kiểu cho `codeIntel.*` (26) và `codeIntel.quality.*` (20) mà frontend hiện dùng; kênh chưa cài ⇒ lỗi "is not yet implemented".
2. Ép hợp đồng: khoá lạ/`args[1+]`/`tenantId|userId|deviceId|role|devServerId|workspaceRoot|repo|args|command|cypher` ⇒ `CODEINTEL_INVALID_PARAMS: …`; kênh view thiếu `projectId`/`worktreeId` ⇒ lỗi; cờ tắt ⇒ `CODEINTEL_DISABLED` (trừ `settings.get|set`); `QUALITY_GATE_DISABLED`/`AI_REVIEW_DISABLED` theo mức.
3. API kịch bản (xem SOL-073 2.4): `setSettings, setIndex, setOverlay, setErd, setStorage, setFindings, setContractDiff, setReviewState, failNext, setRole, pushChanged, pushProgress, pushQuality, dropStream, streamCount, calls, reset`; `reviewState.save` kiểm `expectedVersion` (lệch ⇒ `CODEINTEL_VERSION_CONFLICT | {"currentVersion":n}`).
4. `subscribe`: ack `null`, khung `{event, projectId, worktreeId, occurredAt,…}`; `dropStream()` phát `changed{resync:true}` rồi đóng.
5. Phong bì phẳng (`etag`, `fromCache`, `generatedAt`, `sources`, `stale`, `truncated`, `totalCount`); `ifNoneMatch` tuỳ chọn (không bắt buộc hiểu).
6. Fixture: dựng theo kiểu §4, **một** nguồn cho mọi test; đối chiếu với `backend-go/services/code-intel-service/testdata/agent-results/` khi tệp vàng (G1) có (test bỏ qua có ghi "chưa đối chiếu" nếu chưa).

## Kiểm thử

- Test bộ xử lý: từ chối khoá lạ, mã lỗi trên `message`, cờ tắt, conflict phiên bản, push có `event`, `streamCount`, kênh chưa cài.
- `pnpm --filter orca-frontend test -- src/renderer/src/test-support/code-intel-fake-backend`.

## Tiêu chí hoàn thành

- [ ] Mọi kênh frontend dùng đều có bộ xử lý hoặc lỗi chưa cài rõ ràng.
- [ ] Không phụ thuộc `window.api` ngoài giao diện bridge.
- [ ] Test hợp đồng xanh.

## Rủi ro

- Fake lệch backend thật; cập nhật cùng bảng hợp đồng §3 khi kênh đổi.

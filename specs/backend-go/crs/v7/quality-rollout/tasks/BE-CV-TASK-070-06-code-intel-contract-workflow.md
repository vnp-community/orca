# BE-CV-TASK-070-06: Workflow `code-intel-contract.yml` (tầng chặn PR và tầng live không chặn)

**From Solution:** BE-CV-SOL-070
**Priority:** P0
**Service:** `.github/workflows`, `agent/` (chỉ để gọi test), `code-intel-service`
**File:** `.github/workflows/code-intel-contract.yml` (mới)
**Depends on:** BE-CV-TASK-070-01..05; `AG-CV-SOL-070` (test vitest, script chụp); BE-CV-SOL-010 (module service)
**Status:** `[x] DONE`

---

## Context

- Hiện **không** workflow nào chạy test `agent/`; `pr.yml` dòng 120 và 129 gọi `--config config/vitest.config.ts` không tồn tại ở gốc (đã kiểm: `config/` chỉ có `max-lines-baseline.txt`, `oxlint-react-doctor.json`, `patches`, `scripts`). `agent/vitest.config.ts` có `include: ['src/**/*.test.ts']`; package tên `orca-agent`.
- Mẫu trigger và ma trận: `.github/workflows/backend-go-issue-status-sync.yml` (`on.pull_request.paths`, `go-version: "1.25"`).
- Hợp đồng `go.work` 1.26.0 so với `setup-go` 1.25: chưa kiểm chứng toolchain nào thật sự chạy.

## Việc cần làm

1. Job `contract` (chặn PR). `on.pull_request.paths`: `agent/src/relay/codeintel/**`, `backend-go/services/code-intel-service/**`, `backend-go/proto/orca/codeintel/**`, `backend-go/common/**`, `.github/workflows/code-intel-contract.yml`. Bước: checkout; `setup-go`; `go test ./...` trong `backend-go/services/code-intel-service` (không tag); `pnpm` + Node, cài chỉ phụ thuộc `agent/`; `pnpm --filter orca-agent exec vitest run src/relay/codeintel`.
2. Bước khẳng định `agent/` thật sự được test: `pnpm --filter orca-agent exec vitest list src/relay/codeintel | grep -q .` (thất bại khi không có test, để không "xanh vì không chạy gì").
3. Giới hạn kích thước: bước đếm `testdata/agent-results` và fixture của agent, thất bại khi thư mục phiên bản > 300 KiB hoặc có tệp > 20 KiB.
4. Job `live-contract` (`schedule` hằng đêm + `workflow_dispatch`, `continue-on-error: true`): cài GitNexus (npm toàn cục `gitnexus`, theo AGENTS.md "npm 11 crash → `npm i -g gitnexus`"), chạy `agent/scripts/capture-codeintel-fixtures.mjs` vào thư mục tạm, `diff -r` với tệp đã commit, đẩy artifact. Phần CodeGraph để `if: false` tới khi biết cách cài không tương tác (chưa kiểm chứng).
5. Ghi rõ trong workflow: không chạy `analyze`/`init` ngoài thư mục tạm của repo mẫu.

## Kiểm thử

- Mở PR thử chỉ sửa một tệp vàng: job `contract` chạy và đỏ khi manifest lệch. Mở PR chỉ sửa tài liệu: job không chạy.
- Chạy `workflow_dispatch` cho job live một lần (kết quả có thể là "khác" vì công cụ trôi; không chặn).
- Chưa chạy: toàn bộ là kế hoạch; chạy trên runner thật mới biết thời gian (giả định vài phút cho tầng chặn).

## Tiêu chí hoàn thành

- [x] Tầng chặn chạy trên PR đúng đường dẫn, có bước khẳng định `agent/` được test.
- [x] Tầng live chạy được qua `workflow_dispatch` và không chặn.
- [x] Không thêm `max-lines` disable; không dùng `config/vitest.config.ts`.

## Rủi ro và lưu ý

- Tên file trùng ý với `backend-go-code-intel-service.yml` (CR-010); hai file có đường dẫn kích hoạt chồng nhau, chấp nhận chạy hai lần.
- Nếu `go-version` 1.25 không build được `go.work` 1.26.0, chuyển sang `go-version-file` hoặc 1.26 (Q ở SOL-070/README).

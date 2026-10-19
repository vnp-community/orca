# TASK-TDD-002-T0-05: Chốt phiên bản Orca dùng cho pilot: commit CLI, route gateway, skill và tài liệu

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 1 (nguyên tắc), mục 6 (H9)
**Checklist:** [T0.5](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P0 · **Area:** orca (git)
**Files:** `backend-go/cmd/orca-cli/**`, `backend-go/services/api-gateway/internal/adapter/httpgateway/request_routes*.go`, `skills/tdd-*/`, `specs/backend-go/crs/v8/**`
**Depends on:** không
**Status:** `[ ]` TODO

---

## Context

Công việc CLI `orca` (login, request, solution, approval, plan, clarification), 18 route REST ở gateway (`request_routes_flow.go`: 14 route của `/v1/requests/{id}/…`, 3 route `/v1/clarifications/{id}…`, 1 route `GET /v1/approvals/{id}`) và test đã chạy thật trên stack local nhưng **chưa commit**. Cùng chưa commit: bốn skill trong `skills/` và tài liệu `specs/backend-go/crs/v8`. Stack local đang chạy bản gateway đã redeploy từ thư mục làm việc, nên phải commit đúng bản đó để pilot chạy trên mã có lịch sử.
Luật repo: hook lint/format chạy khi commit, không dùng `--no-verify`; commit theo nhóm.

## Việc cần làm

1. Chạy `gofmt -l`, `go vet` và `go test ./...` cho `backend-go/cmd/orca-cli` và `backend-go/services/api-gateway`; ghi kết quả.
2. Nhóm 1 (gateway): `request_routes.go`, `request_routes_flow.go`, `request_routes_flow_test.go`. Thông điệp: `feat(api-gateway): REST routes for the whole Request flow`.
3. Nhóm 2 (CLI): `backend-go/cmd/orca-cli/**` (README, apiclient, command, output). Thông điệp: `feat(orca-cli): commands to drive a Request without the UI`.
4. Nhóm 3 (skill): `skills/tdd-knowledge`, `tdd-openspec-author`, `tdd-knowledge-writeback`, `tdd-orca-task`. Thông điệp: `feat(skills): tdd-agent knowledge skills`.
5. Nhóm 4 (spec): `specs/backend-go/crs/v8/**`. Thông điệp: `docs(specs): SOL-TDD-001/002 and checklists for tdd-agent integration`.
6. Mỗi commit kết thúc bằng dòng `Co-Authored-By` theo hướng dẫn của phiên; nếu chạy được `gitnexus detect_changes` thì ghi kết quả vào mô tả.
7. Sau commit, redeploy stack local bằng `bash deploy/dev/scripts/redeploy-local.sh api-gateway` và xác nhận `orca request list` vẫn chạy.

## Tiêu chí hoàn thành

- [ ] `git status` không còn tệp của luồng này (các tệp ngoài luồng như `tests/backend/` không bị đụng).
- [ ] Bốn commit qua hook không dùng `--no-verify`.
- [ ] Stack local chạy đúng bản đã commit; CLI chạy lại được một lệnh đọc.

## Kiểm thử

- `go test ./...` của hai module xanh; `go build` CLI.

## Rủi ro và ghi chú

- Không push: người dùng chưa yêu cầu.
- Nếu một tệp không thuộc luồng này nằm trong thay đổi (ví dụ `agent/tsconfig.tsbuildinfo`, `package.json`), không đưa vào commit.

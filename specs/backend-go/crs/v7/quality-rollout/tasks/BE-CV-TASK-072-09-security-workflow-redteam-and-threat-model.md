# BE-CV-TASK-072-09: Workflow `code-intel-security`, red-team MCP (có điều kiện) và tài liệu threat model

**From Solution:** BE-CV-SOL-072
**Priority:** P1
**Service:** `.github/workflows`, `code-intel-service`, `docs`
**File:** `.github/workflows/code-intel-security.yml` (mới), `backend-go/services/code-intel-service/internal/redteam/redteam_test.go` (mới, chỉ khi CR-041), `docs/guides/code-intel/code-intel-threat-model.md` (mới)
**Depends on:** BE-CV-TASK-072-01..08, `AG-CV-SOL-072-security-tests-agent`, BE-CV-SOL-041 (P2, tuỳ chọn)
**Status:** `[x] DONE`

---

## Context

- Mẫu: `.github/workflows/backend-go-issue-status-sync.yml` (ma trận `dialect`), `backend-go/ci/mcp-conformance/run-go-conformance.sh` (`FUZZTIME`), `mcp-service/internal/redteam/redteam_test.go` (RT01–RT17).
- MCP tắt ở v7 (O-11): red-team chỉ khi mở; `excluded_channels.yaml` có một dòng `codeIntel.*`.

## Việc cần làm

1. Workflow: job `security` (chặn PR; `on.pull_request.paths` như SOL-070 task 06) chạy `go test` các gói test bảo mật, fuzz `FUZZTIME=10s`, vitest bảo mật của agent; job `integration` ma trận `dialect: [postgres, mysql]` (`-tags=integration`); job canary (`-tags=e2e`); job `live-security` hằng đêm, không chặn.
2. Red-team MCP (nếu CR-041): nội dung độc trong repo mẫu bị `WrapUntrusted`, không tool ghi lộ ra, đọc không tin cậy xong thì tool open-world kế tiếp cần duyệt; còn không thì file không tạo.
3. Tài liệu threat model (tiếng Việt): tài sản A1–A5, tác nhân T1–T5, S1–S12, rủi ro còn lại (che regex, chỉ mục chứa secret, `fs.*` không giới hạn root, Windows/WSL, oracle thời gian).

## Kiểm thử

- Mở PR thử làm hỏng một vector ⇒ job đỏ. Chạy `workflow_dispatch` job live một lần.
- Chưa chạy.

## Tiêu chí hoàn thành

- [x] Tầng chặn chạy trên PR; tầng live không chặn.
- [x] Tài liệu có danh sách rủi ro còn lại.

## Rủi ro và lưu ý

- Go CI 1.25 so với `go.work` 1.26 chưa kiểm chứng; thời gian CI chưa đo.

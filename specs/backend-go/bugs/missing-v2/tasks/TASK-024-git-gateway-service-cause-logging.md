# TASK-024: Wire `apperrors.SetLogger` into `git-gateway-service`

**Solution:** [SOL-012](../solutions/SOL-012-git-gateway-service-cause-logging.md)
**Status:** [x] DONE (2026-09-15, code + deployed)

## Việc đã làm

1. Thêm import `"github.com/stablyai/orca-go/common/apperrors"` vào `git-gateway-service/cmd/server/main.go`.
2. Thêm `apperrors.SetLogger(logger)` ngay sau `slog.SetDefault(logger)`, đúng vị trí/pattern như `infra-fleet-service` (SOL-009/TASK-018).
3. `go build ./services/git-gateway-service/...` — clean.

## Verify (sau deploy)

- [x] Deploy qua `sync-to-server.sh 2026.09.15-batch2-fix` — 17 service + frontend `Up`, health check pass.
- [x] Logging đã hoạt động thật — thấy dòng `"apperrors: internal cause"` cho 1 lỗi khác (`WORKTREE_REPO_NOT_FOUND`) ngay sau deploy, xác nhận wiring đúng.
- [ ] Tái hiện đúng `GITGATEWAY_STATUS_FAILED` cụ thể, đọc `docker logs orca-go-git-gateway` tìm nguyên nhân thật.
- [ ] Cập nhật BUG-012.md với nguyên nhân thật tìm được.

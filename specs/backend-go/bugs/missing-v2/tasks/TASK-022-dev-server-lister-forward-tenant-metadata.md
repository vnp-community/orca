# TASK-022: Fix `InfraFleetDevServerLister.Exists`/`InfraFleetHostnameResolver.Hostname` to forward tenant metadata

**Solution:** [SOL-011](../solutions/SOL-011-dev-server-lister-forward-tenant-metadata.md)
**Status:** [x] DONE (2026-09-15, code + deployed)

## Việc đã làm

1. `gitnexus impact({target: "InfraFleetDevServerLister", direction: "upstream"})` → LOW risk, 3 impacted (wiring-only).
2. `gitnexus impact({target: "InfraFleetHostnameResolver", direction: "upstream"})` → LOW risk, 3 impacted.
3. Sửa `infra_fleet_dev_server_lister.go`'s `Exists`: gọi `withTenantMetadata(ctx)` trước khi gọi `ListDevServers`, trả lỗi sớm nếu không lấy được tenant.
4. Sửa `dev_server_hostname_resolver.go`'s `Hostname`: cùng thay đổi.
5. `go build ./services/project-service/...` — clean, không lỗi biên dịch.

## Verify (sau deploy)

- [x] Deploy qua `sync-to-server.sh 2026.09.15-bug010-fix` — 17 service + frontend `Up`, health check pass, `docker logs orca-go-project` xác nhận version mới.
- [ ] Thử lại `repo.rebindDevServer` cho repo "aiops-v3" (id `f5cf6986-9409-44c8-90f0-b7366d4c035c`) → `test-01` — xác nhận không còn `PROJECT_DEV_SERVER_LOOKUP_FAILED`. (cần user thao tác qua UI để confirm)
- [ ] `docker logs orca-go-infra-fleet` — xác nhận không còn `INFRA_NO_TENANT` cho `ListDevServers` gọi từ `project-service`.
- [ ] Xác nhận `dev_server_id` của repo đã update đúng trong DB, và tạo worktree cho "aiops-v3" không còn `WORKTREE_CREATE_FAILED`.
- [x] Cập nhật BUG-010.md/SOL-011.md từ "chưa deploy" → đã deploy.

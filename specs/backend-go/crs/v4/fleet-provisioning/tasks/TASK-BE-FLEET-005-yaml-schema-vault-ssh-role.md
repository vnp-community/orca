# TASK-BE-FLEET-005: YAML schema — thêm `vaultSshRole` vào `FleetServerSchema`

**Solution:** BE-FLEET-SOL-001 | **CR:** CR-FLEET-001
**Service:** `frontend` (không phải `backend-go`)
**Status:** ➡️ MOVED (2026-09-09) — xem [FE-TASK-FLEET-001](../../../../../frontend/crs/v4/fleet-provisioning/tasks/FE-TASK-FLEET-001-yaml-schema-vault-ssh-role.md)

---

Task này sửa `frontend/src/shared/fleet-config-parser.ts` — ngoài phạm vi
`specs/backend-go/`. Nội dung đầy đủ (thiết kế, vị trí field chính xác,
test plan) đã chuyển sang
[`specs/frontend/crs/v4/fleet-provisioning/tasks/FE-TASK-FLEET-001-yaml-schema-vault-ssh-role.md`](../../../../../frontend/crs/v4/fleet-provisioning/tasks/FE-TASK-FLEET-001-yaml-schema-vault-ssh-role.md),
cùng 1 solution tương ứng
[`FE-FLEET-SOL-001`](../../../../../frontend/crs/v4/fleet-provisioning/solutions/FE-FLEET-SOL-001-yaml-schema-vault-ssh-role.md).

**Lịch sử:** file này ban đầu (2026-09-09, phiên thực thi backend-go) chứa
toàn bộ nội dung task, đã SKIP đúng đắn vì phiên đó chỉ có quyền sửa
`backend-go/services/infra-fleet-service/`, `backend-go/proto/orca/infrafleet/v1/`,
`agent/src/relay/` — không có `frontend/`. Nội dung được giữ nguyên
byte-for-byte khi chuyển, chỉ cập nhật lại đường dẫn liên kết tương đối.

**Không có task nào trong `backend-go/`'s F31 phụ thuộc build vào file
này** — nó chỉ là điều kiện để client (UI/CLI) build được request hợp lệ
gửi tới `backend-go`'s `BulkProvisionFleet` RPC (đã DONE), không phải
điều kiện để `backend-go`'s code tự nó build/test được.

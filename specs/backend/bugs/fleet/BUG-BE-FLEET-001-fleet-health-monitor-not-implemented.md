# BUG-BE-FLEET-001: `FleetManager`, `FleetProvisioner` và YAML fleet inventory loader chưa được implement

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-FLEET-001  
**Note:** fleet-health-monitor.ts: BrowserWindow decoupled → onAlert callback  

## Mức độ: 🟡 MEDIUM (Partial Implementation)

## Cập nhật

`FleetHealthMonitor` thực tế ĐÃ tồn tại tại `src/main/ssh/fleet-health-monitor.ts`.
Tuy nhiên nó thiếu nhiều thành phần so với HLD. Xem **BUG-BE-FLEET-002** cho chi tiết health monitor gaps.

## Tóm tắt

HLD (BL-FLEET-01 → BL-FLEET-04) mô tả các components:
```
FleetManager.loadInventory()     ← parse /etc/orca/fleet.yaml
FleetProvisioner.provision()     ← SFTP upload + SSH exec per server
BL-FLEET-04: Onboarding Wizard  ← POST /admin/api/fleet/servers/test
```

Grep toàn bộ `src/` không tìm thấy:
```
FleetManager                → No results
FleetProvisioner            → No results
fleet.yaml                  → No results (YAML loader)
/etc/orca/fleet.yaml        → No results
POST /admin/api/fleet       → No results
```

## Ảnh hưởng

1. **BL-FLEET-01**: YAML fleet inventory không được load tự động khi start.
2. **BL-FLEET-02**: Bulk provisioning (SFTP upload relay binary) không có.
3. **BL-FLEET-04**: Onboarding Wizard (test connection + provision single server) không có REST API.
4. `fleet-bootstrap-service.ts` và `dev-server-provisioner.ts` có nhưng không có YAML loader.

## Files không tồn tại (theo HLD)

- `src/main/fleet/fleet-manager.ts` — YAML inventory loader
- `src/main/fleet/fleet-provisioner.ts` — Bulk SFTP provisioning
- REST routes: `POST /admin/api/fleet/servers/test`, `POST /admin/api/fleet/provision`

## Liên quan đến luồng

- **BL-FLEET-01**: Fleet YAML inventory — không có.
- **BL-FLEET-02**: Bulk provisioning — không có.
- **BL-FLEET-04**: Onboarding wizard — không có.

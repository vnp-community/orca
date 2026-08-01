# BUG-FE-FLEET-001: Fleet Dashboard (Admin SPA) không tồn tại trong Renderer — không có server inventory, không có health UI

## Mức độ: 🔴 HIGH (Feature Missing)

## Tóm tắt

HLD (BL-FLEET-01 → BL-FLEET-04) mô tả Admin SPA:
```
[Admin SPA] Fleet → server inventory table
    - Color-coded health status: green/yellow/red
    - CPU/RAM/Disk per server
    - "Provision All" button
    
[Admin SPA] Fleet → "Add Server" → Onboarding Wizard
    Step 1-6: SSH host → key → test → deploy → tags → finish

[Admin SPA] Fleet health dashboard:
    CPU/RAM/Disk charts (real-time)
    Webhook alerts (Slack/PagerDuty)
```

Grep `src/renderer/` không tìm thấy:
```
FleetPanel        → No results
FleetDashboard    → No results
DevServerHealth   → No results
fleet.*inventory  → No results
onboarding.*wizard → No results (fleet specific)
```

## Ảnh hưởng

1. **BL-FLEET-01**: Fleet inventory table — không có.
2. **BL-FLEET-03**: Real-time health dashboard — không có.
3. **BL-FLEET-04**: Onboarding Wizard UI — không có.
4. Admin không thể quản lý dev server fleet từ browser.

## Files không tồn tại

- `src/renderer/src/components/admin/fleet/fleet-dashboard.tsx`
- `src/renderer/src/components/admin/fleet/server-health-card.tsx`
- `src/renderer/src/components/admin/fleet/onboarding-wizard.tsx`
- `src/renderer/src/pages/admin/fleet.tsx`

## Liên quan đến luồng

- **BL-FLEET-01 → BL-FLEET-04**: Toàn bộ Fleet UI không có.

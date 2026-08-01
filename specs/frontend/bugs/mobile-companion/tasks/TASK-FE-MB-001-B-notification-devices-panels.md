# TASK-FE-MB-001-B: `NotificationSettings` + `PairedDevicesPanel` (BL-MB-02, BL-MB-04)

**Domain:** mobile-companion  
**Solution Ref:** SOL-FE-MB-001 §Components 2 & 4  
**Bug:** BUG-FE-MB-001  
**Priority:** 🟠 P1  
**Estimated:** 50 phút  
**Status:** ✅ DONE — Implemented

---

## Mục tiêu

Tạo 2 components:
1. `NotificationSettings` — toggles cho loại push notification
2. `PairedDevicesPanel` — danh sách thiết bị đã paired với revoke + push toggle

---

## Files cần tạo

- **TẠO MỚI:** `src/renderer/src/components/mobile/NotificationSettings.tsx`
- **TẠO MỚI:** `src/renderer/src/components/mobile/PairedDevicesPanel.tsx`

---

## `NotificationSettings.tsx`

```typescript
// BL-MB-02: Notification preferences
// Toggles: Agent Completed, Agent Error, Workflow Completed, Workflow Failed, Push Quota Warning
type NotificationPrefs = {
  agentCompleted: boolean
  agentError: boolean
  workflowCompleted: boolean
  workflowFailed: boolean
  quotaWarning: boolean
}

// Load: window.api.mobile.getNotificationPrefs()
// Save: window.api.mobile.updateNotificationPrefs(prefs)
// Auto-save on toggle (debounced 500ms)

// UI: List của Switch rows với label + description
```

## `PairedDevicesPanel.tsx`

```typescript
// BL-MB-04: Manage paired devices
// State: PairedDevice[] { id, name, platform, lastSeenAt, pushEnabled }

// Load: window.api.mobile.listDevices()
// Revoke: window.api.mobile.revokeDevice(deviceId) → filter out from list
// Toggle push: window.api.mobile.updateNotificationPrefs({ agentCompleted: push, ... })

// UI per device:
// [📱 iPhone 15 (iOS)]  Last seen: 2h ago  [Push 🔔 Toggle]  [Revoke ×]
```

---

## Verify

```bash
grep -n "NotificationSettings\|PairedDevicesPanel" \
  src/renderer/src/components/mobile/NotificationSettings.tsx \
  src/renderer/src/components/mobile/PairedDevicesPanel.tsx
```

## Depends on
Không có

## Blocking
TASK-FE-MB-001-C (MobileCompanionPage)

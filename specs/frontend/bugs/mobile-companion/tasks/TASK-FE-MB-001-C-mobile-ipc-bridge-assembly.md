# TASK-FE-MB-001-C: Mobile IPC Bridge + `MobileCompanionPage` assembly (BL-MB-01~04)

**Domain:** mobile-companion  
**Solution Ref:** SOL-FE-MB-001 §Assembly + IPC  
**Bug:** BUG-FE-MB-001  
**Priority:** 🟠 P1  
**Estimated:** 45 phút  
**Status:** ✅ DONE — Implemented

---

## Mục tiêu

1. Thêm `mobile.*` vào IPC/preload bridge  
2. Tạo `MobileCompanionPage` — tabs layout chứa Pairing + Notifications + Devices

---

## Files cần tạo/sửa

- **MODIFY:** `src/preload/index.ts` — thêm `mobile.*`
- **MODIFY:** `src/renderer/src/web/web-preload-api.ts` — thêm `mobile.*`
- **TẠO MỚI:** `src/renderer/src/components/mobile/MobileCompanionPage.tsx`

---

## Bước 1: Preload bridge

```typescript
// Thêm vào contextBridge exposeInMainWorld 'api':
mobile: {
  createPairSession:        () => ipcRenderer.invoke('mobile:createPairSession'),
  listDevices:              () => ipcRenderer.invoke('mobile:listDevices'),
  revokeDevice:             (id) => ipcRenderer.invoke('mobile:revokeDevice', id),
  getNotificationPrefs:     () => ipcRenderer.invoke('mobile:getNotificationPrefs'),
  updateNotificationPrefs:  (prefs) => ipcRenderer.invoke('mobile:updateNotificationPrefs', prefs),
  onDevicePaired:           (cb) => ipcRenderer.on('mobile:devicePaired', (_e, d) => cb(d)),
  offDevicePaired:          (cb) => ipcRenderer.removeListener('mobile:devicePaired', cb),
}
```

## Bước 2: `MobileCompanionPage.tsx`

```typescript
// 3-tab layout:
// Tab 1: "Pair New Device" → <MobilePairingPage />
// Tab 2: "Notifications"   → <NotificationSettings />
// Tab 3: "My Devices"      → <PairedDevicesPanel />

// Dùng shadcn <Tabs> component
```

## Bước 3: Route vào Settings

Tìm Settings page (hoặc sidebar menu) và thêm:
```typescript
// Trong Settings tabs:
<TabsTrigger value="mobile">📱 Mobile</TabsTrigger>
<TabsContent value="mobile">
  <MobileCompanionPage />
</TabsContent>
```

---

## Verify

```bash
grep -n "mobile.createPairSession\|MobileCompanionPage" \
  src/preload/index.ts \
  src/renderer/src/components/mobile/MobileCompanionPage.tsx
```

## Depends on
TASK-FE-MB-001-A, TASK-FE-MB-001-B

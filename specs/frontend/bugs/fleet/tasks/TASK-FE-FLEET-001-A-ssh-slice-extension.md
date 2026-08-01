# TASK-FE-FLEET-001-A: SSH Slice Extension — Fleet state (CR-001~006)

**Domain:** fleet  
**Solution Ref:** SOL-FE-FLEET-001B Phần 1  
**Bug:** BUG-FE-FLEET-001  
**Priority:** 🔴 P0  
**Estimated:** 40 phút  
**Status:** ✅ DONE — SSH slice already has FleetImportStatus, ServerHealthMetrics, FleetAlerts in ssh.ts

---

## Mục tiêu

Mở rộng `ssh.ts` Zustand slice với toàn bộ Fleet state theo TDD-FE-10 §2 (6 CRs).

---

## Files cần sửa

- `src/renderer/src/store/slices/ssh.ts`

---

## Các bước thực thi

Thêm vào cuối `createSshSlice` (KHÔNG xóa existing state):

### Types cần thêm (trước slice)

```typescript
export type FleetImportStatus = { phase, totalServers, importedServers, skippedServers, failedServers, errors, configFilePath }
export type SshTargetGroup = { name, targets, tags }
export type ServerHealthMetrics = { serverId, lastCheckedAt, isReachable, uptimeSeconds, relayVersion, nodeVersion, diskUsagePercent, cpuUsagePercent, memUsagePercent }
export type FleetAlert = { id, serverId, serverLabel, type: 'disconnected'|'error'|'relay-outdated', message, timestamp, dismissed }
export type BootstrapStatus = { serverId, currentStep, totalSteps: 7, status, errorMessage?, stepLogs }
export type BulkProvisioningProgress = { total, completed, failed, inProgress: string[] }
```

### Actions cần thêm

```
CR-001: fleetImportStatus, setFleetImportStatus(), clearFleetImportStatus()
CR-002: sshTargetGroups[], setSshTargetGroups(), activeGroupFilter, setActiveGroupFilter()
CR-003: bulkProvisioningProgress, setBulkProvisioningProgress()
CR-004: bootstrapStatusByServer{}, setBootstrapStatus(serverId, status)
CR-005: serverHealthMetrics{}, updateServerHealth(serverId, metrics), lastFleetHealthCheck, setLastFleetHealthCheck(), fleetAlerts[], addFleetAlert(), dismissFleetAlert()
CR-006: currentUser, setCurrentUser(), accessPolicy, setAccessPolicy()
```

Xem full code trong SOL-FE-FLEET-001B §Phần 1.

---

## Verify

```bash
grep -n "fleetImportStatus\|serverHealthMetrics\|bootstrapStatusByServer\|fleetAlerts" \
  src/renderer/src/store/slices/ssh.ts
```

## Depends on
Không có

## Blocking
TASK-FE-FLEET-001-B, TASK-FE-FLEET-001-C, TASK-FE-FLEET-001-E

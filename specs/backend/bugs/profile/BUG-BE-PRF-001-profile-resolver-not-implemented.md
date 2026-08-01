# BUG-BE-PRF-001: `ProfileResolver` và 3-layer merge chưa được implement — Profile phụ thuộc vào orca_company/orca_departments chưa có

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-PRF-001  
**Note:** ProfileResolver.ts: 3-layer merge (global→team→project→user) with 60s cache  

## Mức độ: 🔴 HIGH (Feature Missing)

## Tóm tắt

HLD (BL-PRF-02) mô tả:
```
[ProfileResolver.resolve(userId)]
    → Check in-memory cache (TTL 60s)
    → Server DB: 3 SELECT queries (company + dept + user)
    → deepMerge(company ← dept ← user)
    → Cache + return ResolvedProfile
```

Grep toàn bộ `src/` không tìm thấy:
```
ProfileResolver      → No results
orca_company         → No results
orca_departments     → No results
deepMerge.*profile   → No results
profileCache         → No results
```

## Ảnh hưởng

1. **BL-PRF-01 → BL-PRF-04** hoàn toàn chưa implement.
2. `ProfileAwareAgentSpawner.spawn()` không có → agent spawn không có profile injection.
3. Company-level/Department-level settings không có effect.
4. `maxConcurrentAgents` không được enforce.

## Files không tồn tại (theo HLD)

- `src/main/profile/profile-resolver.ts` — chưa tạo
- `src/main/profile/profile-aware-agent-spawner.ts` — chưa tạo
- DB migration: `orca_company`, `orca_departments` tables — chưa tạo
- REST routes: `PATCH /api/profiles/company`, `PATCH /api/profiles/departments/:deptId` — chưa tạo

## Liên quan đến luồng

- **BL-PRF-01**: Profile CRUD — không có.
- **BL-PRF-02**: 3-layer merge resolution — không có.
- **BL-PRF-03**: Project-Dev Server assignment (phần ProjectServerRouter) — không có.
- **BL-PRF-04**: Profile-aware agent spawning — không có.

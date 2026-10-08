# Orca — Test Specification Suite

**Phiên bản:** 1.1  
**Ngày cập nhật:** 2026-08-01  
**Phạm vi:** Toàn bộ business logic (74 nghiệp vụ) + 19 data flow domains  
**Tham chiếu:** PRD v3.0, SRS, URD, Actor-BL Mapping, Feature-BL Mapping, F36/F37/F38/F39  
**Tổng test cases:** 480+ test cases, 19 domains

---

## Tài liệu nền tảng

| File | Mô tả |
|------|-------|
| [TR-000-test-requirements.md](./TR-000-test-requirements.md) | Yêu cầu kiểm thử toàn hệ thống (v1.1 — 105+ TR items) |
| [TD-000-test-design.md](./TD-000-test-design.md) | Thiết kế kiểm thử, kỹ thuật, chiến lược (v1.1 — 10 domains) |

---

## Test Cases theo Domain

| # | Domain | Priority | Files | BL Coverage |
|---|--------|----------|-------|-------------|
| 01 | [Auth](./01-auth/) | P0 | 5 files | BL-AUTH-01→05 |
| 02 | [Worktree](./02-worktree/) | P0 | 5 files | BL-WT-01→05 |
| 03 | [Agent](./03-agent/) | P0 | 5 files | BL-AG-01→05 |
| 04 | [Terminal](./04-terminal/) | P0 | 4 files | BL-TM-01→04 |
| 05 | [Remote Dev / SSH](./05-remote-dev/) | P1 | 4 files | BL-SSH-01→04 |
| 06 | [Code Review](./06-code-review/) | P1 | 5 files | BL-CR-01→05 |
| 07 | [Project Integration](./07-project-integration/) | P1 | 4 files | BL-PI-01→04 |
| 08 | [Mobile Companion](./08-mobile/) | P0 | 4 files | BL-MB-01→04 |
| 09 | [Automation](./09-automation/) | P2 | 4 files | BL-AT-01→04 |
| 10 | [Design & Browser](./10-design-browser/) | P1 | 3 files | BL-DB-01→03 |
| 11 | [CLI & Headless](./11-cli-headless/) | P1 | 3 files | BL-CLI-01→03 |
| 12 | [Fleet Management](./12-fleet/) | P1 | 4 files | BL-FLEET-01→04 |
| 13 | [Agent WebSocket](./13-agent-ws/) | P1 | 3 files | BL-AWS-01→03 |
| 14 | [Remote Integration](./14-remote-integration/) | P1 | 3 files | BL-INT-01→03 |
| 15 | [Profile Hierarchy](./15-profile/) | P0 | 4 files | BL-PRF-01→04 |
| 16 | [AI Provider Mgmt](./16-ai-providers/) | P0 | 3 files | BL-AIP-01→03 |
| 17 | [Workflow Orchestration](./17-workflow/) | P1 | **3 files ↑** | BL-WF-01→03 |
| 18 | [Task Graph](./18-task-graph/) | P0 | **4 files ↑** | BL-TG-01→04 |
| 19 | [Project Workspace](./19-project-workspace/) | P0 | **4 files ↑** | BL-PW-01→04 |

---

## Thống kê

| Metric | Value |
|--------|-------|
| Total Domains | 19 |
| Total BL Items | 74 |
| Total Data Flows | 19 |
| Total Test Files | 82 |
| Estimated Test Cases | **480+** (was 370+) |
| P0 (Critical) | 27 BL items |
| P1 (Important) | 42 BL items |
| P2 (Nice to have) | 5 BL items |
| **TR Items (v1.1)** | **105+** (WF: +6, TG: +7, PW: +9) |
| **New TCs (v1.1)** | **~110 new/enhanced** |

### Domains cập nhật v1.1 (2026-08-01)

| Domain | Changes |
|--------|---------|
| **17-workflow** | TC-WF-001: 4→7 TCs; TC-WF-002: 6→10 TCs; TC-WF-003: 3→7 TCs |
| **18-task-graph** | TC-TG-001: 9→12 TCs; TC-TG-002: 4→8 TCs; TC-TG-003: 5→7 TCs; TC-TG-004: 4→8 TCs |
| **19-project-workspace** | TC-PW-001: 6→8 TCs; TC-PW-002: 8→9 TCs; TC-PW-003: 14→19 TCs; TC-PW-004: 4→8 TCs |

---

## Hướng dẫn thực thi

### Thứ tự ưu tiên

1. **P0 Domains** (phải pass trước release):
   - 01-auth, 02-worktree, 03-agent, 04-terminal, 08-mobile, 15-profile, 16-ai-providers, 18-task-graph, 19-project-workspace

2. **P1 Domains** (cần pass cho production):
   - 05-remote-dev, 06-code-review, 07-project-integration, 10-design-browser, 11-cli-headless, 12-fleet, 13-agent-ws, 14-remote-integration, 17-workflow

3. **P2 Domains** (nice to have):
   - 09-automation

### Convention ID

```
TC-{DOMAIN}-{NUMBER}-{SCENARIO}

Ví dụ:
TC-AUTH-001-01  → Domain=AUTH, File=001, SubCase=01
TC-WT-002-03    → Domain=WT, File=002, SubCase=03
TC-WF-002-04    → Domain=WF, File=002, SubCase=04
TC-TG-003-06    → Domain=TG, File=003, SubCase=06
TC-PW-003-11    → Domain=PW, File=003, SubCase=11
```

### Trạng thái Test

- ✅ PASS — Test passed
- ❌ FAIL — Test failed
- ⏭️ SKIP — Environment không có
- 🔄 PENDING — Chưa implement
- 🔒 BLOCKED — Blocked by dependency

---

*Orca v5.0 Test Suite — Updated 2026-08-01 (v1.1) — Covers 74 BL items, 19 data flow domains, 480+ test cases*

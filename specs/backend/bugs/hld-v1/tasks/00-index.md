# hld-v1/tasks — Mục lục 33 task thực thi

**Nguồn:** chia nhỏ từ 12 file trong [`../solutions/`](../solutions/00-index.md) (đã căn cứ `specs/backend/tdd/v4`+`v5`). Mỗi task là 1 đơn vị công việc atomic, AI coding agent có thể nhận và thực thi độc lập (đã kèm code cụ thể trích từ solution, không cần tự khám phá lại).
**Quy ước ID:** `TASK-HLD-001` → `TASK-HLD-033`, đánh số cố định theo domain, không trùng lặp.

## Bảng đầy đủ (theo thứ tự thực thi khuyến nghị)

| # | Task | Priority | Depends on | Bug ref |
|---|------|----------|------------|---------|
| 1 | [TASK-HLD-001](./TASK-HLD-001-add-getuserrole-helper-wire-bootstrap.md) — Thêm `getUserRole` helper + wire bootstrap | 🔴 CRITICAL | None | BUG-001,002 (prereq) |
| 2 | [TASK-HLD-002](./TASK-HLD-002-fix-requireadmin-profile-rpc-handler.md) — Fix `requireAdmin` trong profile-rpc-handler | 🔴 CRITICAL | 001 | BUG-001 |
| 3 | [TASK-HLD-003](./TASK-HLD-003-fix-requireowneroradmin-project-rpc-handler.md) — Fix `requireOwnerOrAdmin` trong project-rpc-handler | 🟠 HIGH | 001 | BUG-002 |
| 4 | [TASK-HLD-004](./TASK-HLD-004-apply-rbac-patch-to-desktop-copy.md) — Áp lại patch RBAC cho bản sao `desktop/` | 🟠 HIGH | 002, 003 | BUG-001,002 |
| 5 | [TASK-HLD-005](./TASK-HLD-005-design-permission-service-phase2.md) — Thiết kế `PermissionService` (phase 2, follow-up) | 🟢 LOW | None (không block) | BUG-003 |
| 6 | [TASK-HLD-006](./TASK-HLD-006-guard-multiuser-gh-cli-local-exec.md) — Guard `ORCA_MULTI_USER` cho gh/glab cục bộ | 🔴 CRITICAL | None | BUG-004 |
| 7 | [TASK-HLD-007](./TASK-HLD-007-pass-userid-to-relay-pty-spawn.md) — Truyền `userId` vào `relay.call('pty.spawn')` | 🟠 HIGH | None | BUG-005 |
| 8 | [TASK-HLD-008](./TASK-HLD-008-agent-pty-handler-build-gh-config-dir.md) — Agent set `GH_CONFIG_DIR` từ userId | 🟠 HIGH | 007 | BUG-005 |
| 9 | [TASK-HLD-009](./TASK-HLD-009-roadmap-migrate-github-gitlab-rpc-to-relay.md) — Roadmap migrate toàn bộ RPC GitHub/GitLab sang relay | 🟢 LOW | None (không block) | BUG-004 |
| 10 | [TASK-HLD-010](./TASK-HLD-010-implement-list-all-active-sessions.md) — Implement `listAllActiveSessions` | 🟠 HIGH | None | BUG-006 |
| 11 | [TASK-HLD-011](./TASK-HLD-011-create-admin-policy-handlers.md) — Tạo `admin-policy-handlers.ts` | 🟠 HIGH | None | BUG-007 |
| 12 | [TASK-HLD-012](./TASK-HLD-012-wire-admin-policy-handlers-and-audit-actions.md) — Wire policy handlers + audit actions | 🟠 HIGH | 011 | BUG-007 |
| 13 | [TASK-HLD-013](./TASK-HLD-013-fix-workflow-orchestrator-executestep-type-mismatch.md) — **BLOCKER**: fix type-mismatch `executeStep()` | 🔴 CRITICAL | None | (phát hiện phụ, chặn 014-016) |
| 14 | [TASK-HLD-014](./TASK-HLD-014-add-provider-selection-per-step.md) — Provider selection theo step | 🟠 HIGH | 013 | BUG-008 |
| 15 | [TASK-HLD-015](./TASK-HLD-015-add-workflow-paused-status-migration.md) — `'paused'` status + migration 0014 | 🟠 HIGH | 013 | BUG-009 |
| 16 | [TASK-HLD-016](./TASK-HLD-016-add-workflow-pause-resume-rpc.md) — RPC `workflow.pause`/`resume` | 🟠 HIGH | 015 | BUG-009 |
| 17 | [TASK-HLD-017](./TASK-HLD-017-fleet-health-monitor-real-metrics.md) — FleetHealthMonitor CPU/RAM/disk/latency thật | 🟡 MEDIUM | None | BUG-010 |
| 18 | [TASK-HLD-018](./TASK-HLD-018-implement-fleet-provision-cli.md) — CLI `orca fleet provision` | 🟡 MEDIUM | None | BUG-012 |
| 19 | [TASK-HLD-019](./TASK-HLD-019-fleet-bootstrap-diskcheck-sha256.md) — Disk-check + SHA256 verify bootstrap | 🟡 MEDIUM | None | BUG-013 |
| 20 | [TASK-HLD-020](./TASK-HLD-020-session-auto-respawn-backoff.md) — Session auto-respawn + backoff | 🟡 MEDIUM | None | BUG-011 |
| 21 | [TASK-HLD-021](./TASK-HLD-021-session-idle-timeout-env-var.md) — Đọc `SESSION_IDLE_TIMEOUT_MS` | 🟡 MEDIUM | None | BUG-011 |
| 22 | [TASK-HLD-022](./TASK-HLD-022-fix-auditlogger-column-mismatch.md) — **Prereq**: fix `AuditLogger` sai cột | 🔴 CRITICAL | None | (phát hiện phụ, chặn 023) |
| 23 | [TASK-HLD-023](./TASK-HLD-023-ai-provider-key-rotation.md) — AI Provider key rotation | 🟡 MEDIUM | 022 | BUG-014 |
| 24 | [TASK-HLD-024](./TASK-HLD-024-ai-provider-quota-80-alert.md) — Quota 80% alert | 🟡 MEDIUM | None | BUG-015 |
| 25 | [TASK-HLD-025](./TASK-HLD-025-fix-db-schema-docs-table-names.md) — Sửa docs tên bảng DB (chỉ docs) | 🟢 LOW | None | BUG-016 |
| 26 | [TASK-HLD-026](./TASK-HLD-026-electron-adapter-scope-decision.md) — Quyết định phạm vi ElectronAdapter (không code) | 🟢 LOW | None | BUG-017 |
| 27 | [TASK-HLD-027](./TASK-HLD-027-implement-electron-adapter-skeleton.md) — Implement ElectronAdapter (nếu 026 chọn build) | 🟢 LOW (conditional) | 026 | BUG-017 |
| 28 | [TASK-HLD-028](./TASK-HLD-028-implement-getstagedcommitcontext.md) — `getStagedCommitContext` (quick win) | 🟡 MEDIUM | None | BUG-018 |
| 29 | [TASK-HLD-029](./TASK-HLD-029-agent-git-handler-extended-8-methods.md) — Agent: 8 method git mở rộng | 🟡 MEDIUM | None | BUG-018 |
| 30 | [TASK-HLD-030](./TASK-HLD-030-wire-devserver-git-provider-to-extended-methods.md) — Wire DevServerGitProvider → 8 method | 🟡 MEDIUM | 029 | BUG-018 |
| 31 | [TASK-HLD-031](./TASK-HLD-031-fix-f29-doc-keepalive-closecode.md) — Sửa docs F29 (keepalive/close-code) | 🟢 LOW | None | BUG-019 |
| 32 | [TASK-HLD-032](./TASK-HLD-032-implement-agent-version-mismatch-check.md) — Version-mismatch check thật | 🟡 MEDIUM | None | BUG-019 |
| 33 | [TASK-HLD-033](./TASK-HLD-033-project-devserver-rebind.md) — Cho phép rebind Dev Server | 🟡 MEDIUM | 002, 003 | BUG-020 |

## Nhóm theo Sprint khuyến nghị (song song hoá tối đa trong mỗi sprint)

```
Sprint 1 — Security (bắt buộc trước, review kỹ):
  001 → 002, 003 (song song) → 004
  006, 007 → 008 (song song với nhóm 001-004)
  022 (độc lập, làm song song)

Sprint 2 — Blocker + Feature gap "đã claim xong":
  013 (BLOCKER — làm đầu sprint) → 014, 015 → 016
  010, 011 → 012 (song song với nhóm workflow)
  023 (sau 022), 024 (song song)

Sprint 3 — Reliability & Fleet:
  017, 018, 019 (độc lập, song song hoàn toàn)
  020, 021 (độc lập, song song)
  028, 029 → 030
  032 (độc lập)

Sprint 4 — Dọn dẹp / quyết định / không gấp:
  005, 009, 025, 026 → 027, 031
  033 (sau khi 002+003 đã merge)
```

## Tham khảo

- Solution gốc: [`../solutions/00-index.md`](../solutions/00-index.md)
- Bug tickets: [`../00-index.md`](../00-index.md)
- Audit report: [`audit/backend/backend-vs-design-review.md`](../../../../../audit/backend/backend-vs-design-review.md)

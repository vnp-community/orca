# hld-v1/solutions — Mục lục giải pháp

**Nguồn căn cứ kiến trúc:** `specs/backend/tdd/v4/` (Web Server baseline) + `specs/backend/tdd/v5/` (Enterprise features F33–F40), đối chiếu với code thật qua CodeGraph/GitNexus tại thời điểm viết (2026-08-09).
**Quy ước:** mỗi file `SOLUTION-*-exact.md` chứa code-level fix thật (File/Lines + đoạn code sai + đoạn code fix), theo đúng format `specs/backend/bugs/auth/solutions/SOLUTION-auth-exact.md` đã có trong repo.

## Bảng ánh xạ Bug → Solution

| Bug ID | Solution file | TDD căn cứ |
|---|---|---|
| BUG-BE-HLD-001, 002, 003 | [SOLUTION-rbac-exact.md](./SOLUTION-rbac-exact.md) | tdd/v4/05-auth-layer.md, tdd/v5/14-profile-hierarchy.md, tdd/v5/15-project-binding.md |
| BUG-BE-HLD-004, 005 | [SOLUTION-github-gitlab-relay-exact.md](./SOLUTION-github-gitlab-relay-exact.md) | tdd/v5/05-ssh-relay.md, tdd/v5/07-runtime-service.md, docs/hld/dev-server-architecture.md §12 |
| BUG-BE-HLD-006, 007 | [SOLUTION-admin-panel-exact.md](./SOLUTION-admin-panel-exact.md) | tdd/v4/07-admin-panel.md, tdd/v4/10-database-layer.md |
| BUG-BE-HLD-008, 009 | [SOLUTION-workflow-exact.md](./SOLUTION-workflow-exact.md) | tdd/v5/17-workflow-orchestration.md |
| BUG-BE-HLD-010, 012, 013 | [SOLUTION-fleet-exact.md](./SOLUTION-fleet-exact.md) | tdd/v4/08-dev-server-manager.md, tdd/v4/12-ssh-relay.md, tdd/v5/13-dev-server-onboarding.md |
| BUG-BE-HLD-011 | [SOLUTION-session-manager-exact.md](./SOLUTION-session-manager-exact.md) | tdd/v4/06-multi-user-sandbox.md |
| BUG-BE-HLD-014, 015 | [SOLUTION-ai-provider-exact.md](./SOLUTION-ai-provider-exact.md) | tdd/v5/16-ai-provider-management.md |
| BUG-BE-HLD-016 | [SOLUTION-db-migration-naming-exact.md](./SOLUTION-db-migration-naming-exact.md) | tdd/v5/15-project-binding.md (chỉ sửa tài liệu, không sửa migration đã chạy) |
| BUG-BE-HLD-017 | [SOLUTION-platform-electron-adapter-exact.md](./SOLUTION-platform-electron-adapter-exact.md) | tdd/v5/10-platform-layer.md, tdd/v4/15-platform-abstraction.md |
| BUG-BE-HLD-018 | [SOLUTION-remote-git-ui-exact.md](./SOLUTION-remote-git-ui-exact.md) | tdd/v5/20-remote-git-ui.md |
| BUG-BE-HLD-019 | [SOLUTION-agent-ws-protocol-exact.md](./SOLUTION-agent-ws-protocol-exact.md) | tdd/v5/05-ssh-relay.md, tdd/v5/04-rpc-server.md |
| BUG-BE-HLD-020 | [SOLUTION-project-devserver-rebind-exact.md](./SOLUTION-project-devserver-rebind-exact.md) | tdd/v5/15-project-binding.md (phụ thuộc BUG-BE-HLD-002 cho phần RBAC) |

## Phát hiện phụ quan trọng lộ ra khi viết solution (ngoài phạm vi 20 bug gốc)

Trong lúc đọc code thật để viết fix chính xác, các agent phát hiện thêm **4 vấn đề mới** cần lưu ý riêng khi triển khai:

1. **[`SOLUTION-workflow-exact.md`] `WorkflowOrchestrator.executeStep()` type-mismatch có thể khiến MỌI step throw `UNSUPPORTED_STEP_TYPE`** — orchestrator kỳ vọng `Record<string, StepExecutorFn>` nhưng `server-bootstrap.ts` truyền vào 1 instance class `StepExecutors` (chỉ có `.execute()`). Nếu đúng, đây là bug runtime nghiêm trọng hơn cả BUG-BE-HLD-008/009 — cần verify ưu tiên cao trước khi merge fix provider-selection (nếu không sẽ vá lên dead code). Xem mục §0 trong file.
2. **[`SOLUTION-github-gitlab-relay-exact.md`] `agent/src/relay/pty-handler.ts`'s `spawn()` không đọc `userId`** — nếu chỉ sửa Backend truyền `userId` mà không sửa Agent nhận và dùng nó để build `GH_CONFIG_DIR`, fix sẽ là no-op. Cũng phát hiện 3 handler Agent đã viết (`github.auth.status`, `gitlab.auth.status`, `gitlab.mr.list`) nhưng **chưa đăng ký case** trong `agent-rpc-dispatch.ts` — dead code hiện có.
3. **[`SOLUTION-rbac-exact.md`] `backend/src/main/{profile,project}/*.ts` có bản sao byte-for-byte ở `desktop/src/main/...`** — cùng lỗ hổng RBAC (BUG-BE-HLD-001/002) tồn tại song song ở Desktop, kèm test có sẵn cần cập nhật (`desktop/src/main/project/__tests__/project-rpc.test.ts`). Vá `backend/` mà bỏ qua `desktop/` sẽ để lại lỗ hổng gốc.
4. **[`SOLUTION-ai-provider-exact.md`] `AuditLogger.log()` insert sai cột so với schema thật của `orca_audit_log`** (migration 0005) — cần fix trước khi thêm audit log mới cho AI Provider CRUD/rotate, nếu không audit log mới cũng sẽ lỗi im lặng.

## Thứ tự merge khuyến nghị

```
Ưu tiên 1 (Security — merge trước, riêng biệt, review kỹ):
  SOLUTION-rbac-exact.md               (BUG-001/002/003 — bao gồm patch cả backend/ và desktop/)
  SOLUTION-github-gitlab-relay-exact.md (BUG-004/005 — áp dụng "fix tối thiểu" §2-3 trước, roadmap §4 sau)

Ưu tiên 2 (Feature gap có claim sai "đã hoàn thành"):
  SOLUTION-admin-panel-exact.md        (BUG-006/007)
  SOLUTION-workflow-exact.md           (BUG-008/009 — verify phát hiện phụ #1 TRƯỚC KHI merge)
  SOLUTION-ai-provider-exact.md        (BUG-014/015 — fix AuditLogger schema mismatch TRƯỚC)

Ưu tiên 3 (Reliability & Fleet):
  SOLUTION-session-manager-exact.md    (BUG-011)
  SOLUTION-fleet-exact.md              (BUG-010/012/013)

Ưu tiên 4 (Feature gap trung bình, ít rủi ro bảo mật):
  SOLUTION-remote-git-ui-exact.md      (BUG-018)
  SOLUTION-project-devserver-rebind-exact.md (BUG-020 — sau khi BUG-002 đã merge)

Ưu tiên 5 (Tài liệu / kiến trúc dài hạn, không gấp):
  SOLUTION-db-migration-naming-exact.md       (BUG-016 — chỉ sửa docs)
  SOLUTION-agent-ws-protocol-exact.md         (BUG-019 — chủ yếu sửa docs + 1 phần code version-check)
  SOLUTION-platform-electron-adapter-exact.md (BUG-017 — cần PO xác nhận scope trước khi build)
```

## Tham khảo

- Bug tickets gốc: [`../00-index.md`](../00-index.md)
- Audit report đầy đủ: [`audit/backend/backend-vs-design-review.md`](../../../../../audit/backend/backend-vs-design-review.md)
- Bug index toàn cục: [`specs/backend/bugs/BUG-SOLUTION-STATUS.md`](../../BUG-SOLUTION-STATUS.md)

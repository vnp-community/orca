# BUG-FE-PW-002: `orcaProjects.linkSourceProject`/`unlinkSourceProject`/`getProjectData`/`list` hoàn chỉnh ở backend — không có component UI nào gọi tới

## Mức độ: 🔴 HIGH

## Trạng thái: ✅ FIXED (2026-09-01) — link/unlink/xem danh sách xong, xem [SOL-FE-PW-002](./solutions/SOL-FE-PW-002-linked-projects-tab-in-project-settings.md).
⚠️ Đọc chéo-user (`getProjectData`) khi mở 1 Project đã link vẫn **chưa làm** — phạm vi riêng, cần
`gitnexus impact` trên `WorkspaceContextValue` trước (xem mục "Không làm ở solution này").

## Tóm tắt

Backend (`backend/src/main/project/orca-project-sharing-rpc-handler.ts` +
`OrcaProjectSourceProjectService.ts`) có đầy đủ 4 RPC method, có test, có audit trail
(`Tracers.orcaProjectSharingFlow`), có RBAC đúng. Đây là cơ chế DUY NHẤT để đưa 1 `Project` cũ
(đa-host, per-user, sidebar chính) vào phạm vi chia sẻ của 1 `OrcaProject`.

Grep xác nhận:
```
grep -rn "linkSourceProject\|unlinkSourceProject\|orcaProjects\.\(list\|getProjectData\)" \
  frontend/src desktop/src
→ (0 kết quả)
```

Không component nào trong `frontend/src` gọi 4 RPC method này qua `callRuntimeRpc`.

## Thực tế trong UI

`ProjectSettings.tsx` (component thật, render bởi `ProjectSwitcher.tsx`, có test) hiện có 3 tab:
`General` (stub, chưa có form), `Members` (`MemberManager.tsx`, đầy đủ, dùng
`project.getMembers/addMember/updateMemberRole/removeMember`), `Repos` (`RepoMemberManager.tsx`,
functional-role per-repo). **Không có tab nào cho "Linked Projects"** — đây chính là chỗ tự nhiên
nhất để đặt tính năng, theo đúng pattern đã có (mỗi tab = 1 component RPC-driven riêng biệt).

## Ảnh hưởng

- Tính năng chia sẻ cốt lõi của OrcaProject **không thể dùng được bởi người dùng cuối** — chỉ
  dev/QA gọi RPC tay được (Postman/CLI/test).
- Kết hợp với [BUG-FE-PW-001](./BUG-FE-PW-001-create-project-dialog-no-duplicate-repo-warning.md):
  người dùng bình thường không có đường nào (kể cả biết tính năng tồn tại) để dùng thử "link" thay
  vì luôn phải tạo Repo Go-native mới song song với dữ liệu cũ.
- Đây cùng pattern đã ghi nhận nhiều lần trong `docs/guides/authorization/
  asset-hierarchy-and-permission-model.md` mục 7 — RBAC/sharing hoàn chỉnh ở backend, mồ côi ở UI.

## Root cause

Comment trong code backend: *"Spread into ALL_RPC_METHODS at bootstrap (wiring done by the Wave 3
integration agent)"* — đây là phần được thêm sau, **không có TDD nào theo sau** ghi nhận contract
này ở tầng frontend (grep xác nhận 0 hit `linkSourceProject`/`OrcaProjectSourceProject` trong toàn
bộ `specs/backend/tdd` và `specs/frontend/tdd`). TDD-FE-12 (viết trước khi tính năng backend này
tồn tại) không có mục nào cho "Linked Projects" — không phải lỗi bị bỏ sót có chủ đích, mà là
thiếu 1 bước cập nhật TDD sau khi backend thêm tính năng mới.

## Liên quan

- **TDD-FE-12** §4 ProjectSettings Dialog — cần bổ sung tab thứ 4 "Linked Projects"
- **BUG-FE-PW-001** — phụ thuộc lẫn nhau, nên fix theo cùng 1 lượt
- `specs/backend/tdd/v5/15-project-binding.md` — có `ProjectMember` API nhưng KHÔNG có
  `OrcaProjectSourceProject`/`orcaProjects.*` — xác nhận gap TDD ở cả backend, không riêng frontend
- `docs/guides/authorization/asset-hierarchy-and-permission-model.md` mục 5 (mô tả đầy đủ 4 RPC
  method + luồng đọc-chéo-user thật)
- `docs/guides/bugs/bug-be-project-002-orcaprojectsourceproject-no-ui.md` (bản ghi nhận gốc)

# TASK-BIGFILE-049 — Move: Managed-worktree creation/activation domain (PTY-lifecycle core)

**Loại:** Move — composition pattern · **Effort:** L (lớn nhất trong toàn bộ
BUG-FE-BIGFILE-002) · **Phụ thuộc:** TASK-BIGFILE-047, 048 (ranh giới an
toàn hơn đã tách trước)
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md` (Giai
đoạn 3 — domain rủi ro cao nhất, tách theo lựa chọn rõ ràng của người dùng
chấp nhận rủi ro)

## Bối cảnh: PTY-lifecycle core, rủi ro cao, zero test coverage

Đây là domain người dùng đã chủ động chọn TRÁNH nhiều lần trong suốt nỗ lực
BUG-FE-BIGFILE-002 (không có test coverage). Sau khi các domain "an toàn"
(git-sourcecontrol, project-groups, worktree-base-status) đã tách hết, người
dùng được hỏi lại và chọn "Bắt đầu PTY-lifecycle core (rủi ro cao)" — chấp
nhận rủi ro để tiếp tục giảm kích thước `orca-runtime.ts`.

## Kết quả thực thi (2026-08-11)

- Domain: `activateManagedWorktree`, `createManagedWorktree` (~990 dòng, một
  mình đã lớn hơn phần lớn các domain đã tách trước đó), `createManagedRemoteWorktree`
  (~305 dòng), cùng ~10 private helper chuyên phục vụ startup/provisioning
  (`buildStartupForDraft`, `buildStartupForAgent`,
  `markLocalWorkspaceTrustedForAgent`, `markRemoteWorkspaceTrustedForAgent`,
  `recordCreatedWorktreeLineage`, `pasteStartupDraftWhenReady`,
  `sendStartupFollowupWhenReady`, `createDefaultTabTerminals`,
  `provisionManagedWorktreeTerminals`, `waitForStartupFollowupReady`,
  `waitForStartupDraftReady`) — tổng ~1,860 dòng liền mạch, cộng thêm cụm
  hàm/type module-scope riêng phục vụ tạo worktree (~220 dòng, dòng
  1029-1476 gốc: `resolveCreateBranchName`, `canCheckoutExistingLocalBranch`,
  toàn bộ cụm khớp PR/review đã chọn, `pathExists`, `hasLocalWorktreeBaseRef`,
  v.v.).
- Phương pháp: xây dựng bảng tra cứu `this.X` đầy đủ bằng script (không đoán
  thủ công), đọc toàn bộ ~1,860 dòng + ~220 dòng helper trước khi viết bất kỳ
  dòng code nào, phân loại chính xác từng free-function/type import (move
  hẳn / ở lại-dùng-chung / export+import-lại) bằng script cross-reference
  với các import statement của `orca-runtime.ts`.
- ~30 host dependency — con số lớn nhất trong tất cả domain đã tách, phản
  ánh đúng vai trò domain này là trung tâm điều phối PTY (chạm tới graph
  store, PTY controller, mobile-session-tabs, tạo terminal, worktree
  lineage, remote-fetch cache).
- Phát hiện quan trọng giữa chừng: 3 helper (`buildStartupForAgent`,
  `markLocalWorkspaceTrustedForAgent`, `markRemoteWorkspaceTrustedForAgent`)
  tưởng chỉ dùng nội bộ domain này, hoá ra còn được gọi từ
  `resolveAgentTerminalCreateOptions` (đường tạo terminal chung, KHÔNG thuộc
  domain này, ở lại `orca-runtime.ts`) — bảng tra cứu tự động ban đầu bỏ sót
  vì đây là gọi method nội bộ (`this.X`), không phải free-function import.
  Xử lý bằng cách bỏ `private`, expose public trên class mới, thêm forwarding
  field trên `OrcaRuntimeService` — đúng mẫu đã dùng nhiều lần trước
  (getBrowserDriver/setBrowserDriver ở TASK-037,
  listRepoWorktreesForResolution ở TASK-040).
- `prefetchManagedWorktreeCreateBase` gốc truyền `runtime: this` (toàn bộ
  `OrcaRuntimeService`) vào `prefetchWorktreeCreateBase` — sau khi tách,
  `this` không còn là `OrcaRuntimeService` nữa. Sửa bằng cách truyền
  `runtime: this.host` — `host` đã thoả mãn cấu trúc tối thiểu
  `WorktreeCreateBasePrefetchRuntime` cần (4 method remote-fetch).
- Nhiều lỗi hẹp phạm vi kiểu narrowing (`this.host.getStore()` gọi nhiều lần
  trong cùng method mất khả năng TS narrow so với field readonly gốc) — sửa
  bằng capture `const store = this.host.getStore()` một lần đầu method, dùng
  lại xuyên suốt — đúng mẫu chuẩn của toàn bộ effort.
- `orca-runtime.ts`: 16,676 → **14,567 dòng** (giảm ~2,109 dòng, mức giảm
  lớn nhất trong một task duy nhất). File mới: 2,349 dòng — domain LỚN NHẤT
  từng tách trong BUG-FE-BIGFILE-002 (vượt cả Linear 2,140 và mobile-floor
  2,201) — đã đăng ký `config/max-lines-baseline.txt` +
  `eslint-disable max-lines` inline theo đúng quy trình AGENTS.md.
- Xác minh: `tsc --noEmit --composite false` 251 lỗi pre-existing không đổi
  (0 lỗi mới), `oxlint` sạch (exit 0) cả 2 config (`default` + `react-doctor`,
  có 1 lần sửa `import { BrowserWindow }` → `import type { BrowserWindow }`
  do chỉ dùng làm type). `pnpm check:max-lines-ratchet`: diff giữa trước/sau
  giống hệt (không tạo thêm "New max-lines bypass" nào ngoài 647 lỗi môi
  trường có sẵn, không liên quan tới domain này).

## Rủi ro còn lại / khuyến nghị

- Đây là extraction CƠ HỌC thuần tuý (Move, không đổi logic) — rủi ro hành
  vi runtime chủ yếu đến từ khối lượng chuyển đổi thủ công lớn, KHÔNG phải
  từ thay đổi ý nghĩa code. TypeScript compiler là lưới an toàn duy nhất
  (không có test tự động) — mọi wiring sai kiểu đã bị `tsc` bắt được và sửa.
  Khuyến nghị: kiểm thử thủ công kỹ luồng tạo worktree (local + remote/SSH,
  có/không startup agent, có/không setup hook, sparse checkout) trước khi
  merge.
- `createManagedWorktree` (~990 dòng) tự nó là ứng viên refactor tiếp theo
  (method quá dài) — không xử lý ở đây để giữ task này là Move thuần.
- Domain PTY-adjacent còn lại chưa tách: cụm `stopTerminalsForWorktree` và
  cụm managed-worktree list/show/sleep nhỏ (xem TASK-BIGFILE-048's notes) —
  vẫn cố tình chưa động tới.

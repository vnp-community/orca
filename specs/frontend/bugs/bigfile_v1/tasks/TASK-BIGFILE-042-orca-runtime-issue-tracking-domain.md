# TASK-BIGFILE-042 — Move (composition): GitHub/GitLab issue-tracking domain

**Loại:** Move — composition pattern (KHÔNG phải barrel, class có `this`)
· **Effort:** L · **Phụ thuộc:** TASK-BIGFILE-008, 009, 041
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md` (Giai
đoạn 3, mở rộng ngoài 5 domain gốc — theo yêu cầu tiếp tục tới khi mọi
file <2000 dòng)

## Kết quả thực thi (2026-08-10)

- Domain ban đầu định làm "Linear/Jira/GitHub" gộp chung theo ước tính
  cụm từ khoá ở TASK-035, nhưng khảo sát thực tế cho thấy **Linear là 1
  domain RIÊNG, rất lớn** (import surface ~10 khối riêng: linear/client,
  linear/issues, linear/issue-context, linear/issue-context-errors,
  linear/projects, linear/teams) — không nằm trong dải dòng đã khảo sát,
  và **Jira wrapper method cũng KHÔNG nằm trong dải dòng đã chọn** dù
  import Jira nằm ngay cạnh (phát hiện qua `tsc` báo toàn bộ khối import
  Jira "unused" sau khi tách). Quyết định: **chỉ tách GitHub + GitLab**
  trong task này, để lại Linear và Jira cho task riêng sau.
- Domain thực tế cũng lẫn với "repo hooks / setup-script inspection"
  (`getSetupHookTrustPayload`, `getRepoHooks`, `checkRepoHooks`,
  `inspectRepoSetupScriptImports`, `readRepoIssueCommand`,
  `writeRepoIssueCommand`) nằm NGAY SAU trong cùng 1 dải dòng liên tục —
  cố ý cắt bỏ ranh giới ở đây (dòng 11130/11132), để lại phần hooks cho
  task riêng, tránh phình phạm vi giữa chừng.
- Domain có ít dependency ngoài hơn hẳn TASK-037/039 (chỉ 6):
  `resolveRepoSelector`, `resolveWorktreeSelector`,
  `getLocalGitExecutionOptionArgs`, `getHostedReviewExecutionOptions`,
  `getStats`, `getStore` — do đây là các method API wrapper thuần, gọi
  hàm ngoài (`github/client`, `gitlab/client`...) chứ không thao tác
  field nội bộ phức tạp như 2 domain trước.
- `resolveHostedReviewTarget` (private, dùng nội bộ, 0 nơi gọi ngoài) di
  chuyển theo luôn vào class mới.
- Đăng ký `config/max-lines-baseline.txt` cho file mới (~1,119 dòng tính
  theo max-lines, vượt ngưỡng 300 mặc định của oxlint — **lưu ý quan
  trọng**: ngưỡng oxlint thực tế là 300 dòng (đã trừ blank/comment), KHÔNG
  phải 2000 như hiểu nhầm ban đầu trong phiên này — hầu hết file mới tách
  ra đều cần baseline entry trừ khi rất nhỏ).
- `orca-runtime.ts`: 21,269 → **20,247 dòng** (giảm ~1,022 dòng). File
  mới: 1,210 dòng.
- Xác minh: `tsc --noEmit --composite false` 251 lỗi pre-existing không
  đổi (0 lỗi mới), `oxlint` sạch (exit 0) cả 2 config.

## Việc tiếp theo (không nằm trong task này)

- Linear integration (domain riêng biệt, ước tính lớn — cần khảo sát field
  usage/method boundary riêng, y hệt cách đã làm ở đây).
- Jira wrapper methods (vị trí thực tế trong class chưa xác định — cần
  grep riêng theo tên hàm Jira, KHÔNG giả định nằm cạnh GitHub/GitLab).
- Repo hooks / setup-script inspection (dòng ~11132–11424 cũ, đã xác định
  ranh giới, chưa tách).

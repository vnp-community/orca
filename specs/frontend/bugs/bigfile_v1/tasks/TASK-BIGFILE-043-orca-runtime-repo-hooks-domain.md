# TASK-BIGFILE-043 — Move (composition): Repo hooks / setup-script domain

**Loại:** Move — composition pattern (KHÔNG phải barrel, class có `this`)
· **Effort:** S · **Phụ thuộc:** TASK-BIGFILE-042 (ranh giới xác định
ngay sau khi tách xong domain 042)
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md` (Giai
đoạn 3, mở rộng ngoài 5 domain gốc)

## Kết quả thực thi (2026-08-10)

- Domain nhỏ, sạch: 10 method (`getSetupHookTrustPayload`,
  `getSharedSetupHookTrustPayload`, `getRepoHooks`, `checkRepoHooks`,
  `inspectRepoSetupScriptImports`, `readRepoIssueCommand`,
  `readRemoteIssueCommandOverride`, `readRemoteSharedIssueCommand`,
  `writeRepoIssueCommand`, `ensureRemoteOrcaDirIgnored`), chỉ **1
  dependency ngoài**: `resolveRepoSelector`.
- File mới lệch ngưỡng `max-lines` rất nhẹ (302/300 dòng) — đăng ký
  baseline kèm ghi chú "có thể dọn nhẹ để xuống dưới ngưỡng, không cần
  tách thêm".
- `orca-runtime.ts`: 20,247 → **19,958 dòng** (giảm ~289 dòng, lần đầu
  xuống dưới 20,000 dòng). File mới: 336 dòng.
- Xác minh: `tsc --noEmit --composite false` 251 lỗi pre-existing không
  đổi (0 lỗi mới), `oxlint` sạch (exit 0) cả 2 config.

# AG-CV-SOL-084: Rule pack quy ước Orca (`ORCA-xxx`): luật diff, luật script, `ruleResults[]`

> 📋 Proposed, chưa triển khai. Ngày soạn 2026-10-06. "Đã đọc" = đọc code/CR; định dạng đầu ra các script `check-*` trừ `check-max-lines-ratchet`, `check-styled-scrollbars`, `check-reliability-gates` **chưa kiểm chứng**.

**CR:** [CR-CV-084](../../../../../../docs/crs/v7/quality-signals/CR-CV-084-project-convention-rule-pack.md). Task AG-CV-TASK-084-01 đến 07. **Khu vực:** `agent/src/relay/`. **Feature:** `quality-signals`.
**TDD/Spec:** [TDD-AG-01](../../../../tdd/v5/01-architecture.md), [api/agent-rpc-catalog-git-fs.md](../../../../api/agent-rpc-catalog-git-fs.md).

## 1. Hợp đồng áp dụng

| Mục ([`CONTRACT-codeintel-agent-rpc.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-agent-rpc.md)) | Áp vào |
|---|---|
| §5.1 profile `repo-rules`, `repo-rules-scripts` (kind `repo-rules`); §5.2 chỉ tên profile; §9.5 profile là dữ liệu của người phát hành agent (luật **không** đọc từ worktree) | tasks 02, 07 |
| §5.5 `QualityFinding` (category `convention`, `tool:"orca-rules"`, fingerprint ở agent) | tasks 03-05 |
| PQ-26: `ruleId` `ORCA-NNN`; luật không chạy được **không** sinh finding `ORCA-0xx-unavailable` mà báo trong `ruleResults[]` (`status ∈ ran|skipped_scope|script_not_found|env_not_ready|disabled`) và làm cổng `unknown`; **ID `ORCA-008`, `ORCA-009`, `ORCA-016`, `ORCA-017` dành riêng, không triển khai** | tasks 02, 06 |
| §4.8 mẫu git (`-c core.quotePath=false`, `--unified=0`, không `--merge-base`) | task 03 (dùng AG-CV-TASK-083-03) |
| §2.4/§9.5 `cwd` đúng, env sạch, `shell:false` | tasks 05, 07 |
| §8.3, AGENTS.md (không `max-lines` disable, tên file cụ thể) | tất cả |

## 2. Lệch giữa CR và hợp đồng

| # | CR-CV-084 | Hợp đồng | Solution theo |
|---|---|---|---|
| 1 | Luật không chạy được sinh finding `info` `ORCA-0xx-unavailable` (2.3) | PQ-26: **không**; dùng `ruleResults[]` | Hợp đồng |
| 2 | `ORCA-016`, `ORCA-017` có dòng trong bảng 2.4 ("n/a"/"ghi nhận") | PQ-26: dành riêng, không triển khai (cả `008`, `009`) | Không có trong pack; test khẳng định vắng |
| 3 | Fingerprint riêng `sha256(ruleId+file+matchedText)` (2.2) | §5.5: công thức chung ở agent (CR-082) | Dùng pipeline 082; `anchor` = văn bản dòng **thêm** đã chuẩn hoá (đó là `matchedText` mở rộng); `anchorOverride` cho luật mức tệp |
| 4 | ORCA-001/002/003 (ratchet, scrollbar, reliability) chạy như luật `script` | PQ-26 tách `orca-check/<script>/<kind>` (profile `repo-check-*` của CR-081/082) khỏi `ORCA-NNN` | **Không chạy lại** 3 script này trong `repo-rules-scripts` (tránh finding trùng): ORCA-001..003 là bí danh siêu dữ liệu trỏ tới profile `repo-check-*`; `repo-rules-scripts` chỉ chạy ORCA-004..006. Câu hỏi mở 1 |
| 5 | Pack lưu `quality-rule-pack-orca.yaml` | — | Mã TS (`quality-rule-pack-orca.ts`): agent đóng gói một `agent.js` bằng esbuild (đã đọc `agent/build.mjs`), không loader YAML |
| 6 | `ruleResults[]` "trả mỗi lần chạy" (2.5) | PQ-26 nêu mảng nhưng §5.3/§5.5 **không nêu chỗ đặt** | Đề xuất `ruleResults?: {ruleId,status,durationMs}[]` trong `QualityStepResult` (xuất qua `quality.results view=steps`); **cần sửa hợp đồng** (câu hỏi mở 2) |
| 7 | Diff luật dùng `git diff <mergeBase>`; `worktree` scope không có `base` | §5.2: `base` bắt buộc chỉ cho `changed|commitRange` | Scope `worktree`: bước diff **`skipped`** (`skipReason:"base_required"`); đề xuất, câu hỏi mở 3 |
| 8 | `severity` mặc định ghi đè bởi tenant | Gate ở backend | Agent trả mặc định của pack |
| 9 | ORCA-012: loại `backend-go/common/**` "tồn tại" | — | Chỉ xét tệp **mới thêm** (`status A`/đổi tên đến), không cần danh sách tồn tại |

## 3. Phụ thuộc chéo khu vực

| Đối ứng | Quan hệ |
|---|---|
| `AG-CV-SOL-083` task 03 (`quality-diff-changed-lines.ts`) | Nguồn dòng thêm có văn bản; điều kiện trước của task 03/04 |
| `AG-CV-SOL-081-A/B` | Profile qua catalog (`registerBuiltinProfiles`), chạy qua executor (script rules), `quality-run-planning` (chọn luật theo tệp đổi) |
| `AG-CV-SOL-082` | `RawQualityFinding`, pipeline, fingerprint, `anchorOverride`, parser `orca-check` |
| `BE-CV-SOL-085-…` | Cổng map `ruleId` ORCA-xxx sang kiểm tra; mức `severity` của tenant; dùng `ruleResults` để `unknown` |
| `BE-CV-SOL-038-static-tenant-filter-rule` | Luật `tenant_id` SQL thuộc đó, **không** nhân đôi ở đây |
| Thứ tự: `082 → 084` (đợt 8) |

## 4. Re-verify (đã đọc, 2026-10-06)

Đã đọc: `config/scripts/check-max-lines-ratchet.mjs` (khối `main`, `--init`, `--prune`, `BASELINE_PATH='config/max-lines-baseline.txt'`, `process.cwd()`), `desktop/config/scripts/check-styled-scrollbars.mjs` (`main(root = process.cwd())`), `check-reliability-gates.mjs`, danh sách `ls desktop/config/scripts` và `config/scripts`, `agent/package.json` (`yaml ^2.8.4`, `jsonc-parser ^3.3.1`), `agent/build.mjs` (esbuild một bundle), `.oxlintrc.json:89-101`, AGENTS.md.

| Điểm | Hiện trạng | Hệ quả |
|---|---|---|
| Vị trí script | Gốc `config/scripts/` chỉ có `check-max-lines-ratchet.mjs`(+test), `rebuild-native-deps.mjs`; các script còn lại ở `desktop/config/scripts/` | `locate` thử cả hai, ghi `script_not_found` nếu không thấy |
| `check-max-lines-ratchet.mjs` | Đối số `--init`/`--prune` **ghi** `config/max-lines-baseline.txt` | argv script rules cố định, không đối số |
| Dependency script | `check-reliability-gates` dùng `jsonc-parser`; `verify-localization-*`/`audit-localization-coverage` dùng TypeScript API (chú thích trong script, theo CR) | Chạy bằng `process.execPath` với `cwd` đúng; thiếu module → `env_not_ready` |
| `yaml`, `jsonc-parser` | Có trong `agent/package.json` | Không thêm phụ thuộc (O12) nếu cần parse JSON |
| Luật chưa tồn tại ở agent | không có file `quality-rule-*` | Mọi thứ là mới |

Correction relative to CR: CR-084 1.2.2 nói `diff` hai baseline "gốc nhiều hơn 260 dòng": không kiểm chứng lại (chỉ dùng như cảnh báo chọn baseline).

## 5. Giải pháp

### 5.1 Cây file (mới)

```
quality-rule-pack-schema.ts / .test.ts            task 02  RulePack, Rule, loader/validate
quality-rule-pack-orca.ts                         task 02  dữ liệu pack (TS const)
quality-rule-diff-matcher.ts / .test.ts           task 03  added-line-regex, added-file-name, file-content-regex + ReDoS guard
quality-rule-diff-runner.ts / .test.ts            task 04  chạy ORCA-007, 010-015 trên dòng thêm
quality-rule-script-locator.ts, quality-rule-script-runner.ts / .test.ts   task 05
quality-rule-results.ts / .test.ts                task 06  ruleResults[], skipped_scope
(profile repo-rules, repo-rules-scripts)          task 07
__fixtures__/quality-rules/                       task 01
```

### 5.2 Pack (task 02) — kiểu

```ts
export type RuleKind = 'diff' | 'script' | 'profile-ref'
export type Rule = {
  id: string                         // ^ORCA-\d{3}$, bất biến; vắng ORCA-008/009/016/017
  title: string; kind: RuleKind; category: 'convention'
  severity: 'error'|'warning'|'info'; enabled: boolean
  scope: { include: string[]; exclude: string[]; fileStatus: ('added'|'modified'|'renamed')[] }
  match?: { type: 'added-line-regex'|'added-file-name'|'file-content-regex'; pattern: string; maxLineLength: number }
  script?: { name: string; locate: string[]; cwd: 'repoRoot'|'desktop'; args: [] }     // args luôn rỗng
  ref?: { profileId: string }                                                       // ORCA-001..003
  message: string; fixHint: string; source: { doc: string; script?: string }
}
export type RulePack = { version: number; pack: 'orca-conventions'; rules: Rule[] }
```
`toolVersion = "orca-conventions@<version>"`, `tool:"orca-rules"`. Mức mặc định (đề xuất, chưa đo): ORCA-004/006/007/010/011 error; ORCA-005/012/013/014 warning; ORCA-015 info (hoặc toàn bộ `info` giai đoạn đo nhiễu — câu hỏi mở 4 của CR).

### 5.3 Danh mục luật khởi đầu (task 04/05/07)

| ID | Luật | Cách | Ghi chú |
|---|---|---|---|
| ORCA-001..003 | ratchet max-lines / scrollbar / reliability | `profile-ref` → `repo-check-*` | không chạy ở đây |
| ORCA-004 | `verify-localization-catalog.mjs` | script, khi `**/i18n/**` đổi | `ruleId` `ORCA-004` |
| ORCA-005 | `audit-localization-coverage.mjs --check` | script, khi `*.tsx` renderer đổi | arg `--check` hằng |
| ORCA-006 | `check-feature-wall-assets.mjs` | script, khi `resources/onboarding/feature-wall/**` đổi | |
| ORCA-007 | `.d.ts` mới dưới `**/{preload,shared}/**` | `added-file-name` | đúng đường dẫn `desktop/src/...` |
| ORCA-010 | thêm `max-lines` disable | `added-line-regex` | bỏ `*.test.*`, `*.md` |
| ORCA-011 | nâng `max-lines` trong `mobile/.oxlintrc.json` | so JSON base vs HEAD | |
| ORCA-012 | tên tệp/thư mục mơ hồ (`helpers`, `utils`, `common`, `misc`, `shared-stuff`, `*-helpers.ts`, `*-utils.ts`) | `added-file-name`, chỉ tệp mới | |
| ORCA-013 | hex cứng trong UI mới | `added-line-regex` ở renderer | báo nhầm cao |
| ORCA-014 | `metaKey` cứng | `added-line-regex` + loại nếu cùng tệp có kiểm nền tảng | |
| ORCA-015 | lệnh Git mới thiếu tương thích | bảng cờ "cần Git mới hơn" | chỉ `info` |

## 6. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| 1 | Pack đóng gói trong agent, không đọc từ worktree | Chống regex độc hại (ReDoS) từ nhánh không tin cậy (R1) |
| 2 | Chỉ dòng **thêm** | Không phạt nợ cũ (R2) |
| 3 | Không chạy lại 3 script đã có profile | Tránh finding trùng với 082 |
| 4 | Không finding "unavailable" | PQ-26; cổng `unknown` qua `ruleResults` |
| 5 | Regex có `maxLineLength` + thời gian tối đa | ReDoS |
| 6 | ID bất biến, không tái dùng | Waiver bền (R5) |
| 7 | Định vị script động | Cây repo lệch (CR 1.2) |

## 7. Tiêu chí chấp nhận

- [ ] Pack nạp được; `id` sai/trùng/rơi vào `008|009|016|017` làm nạp thất bại.
- [ ] Mỗi luật có fixture vi phạm/không vi phạm; ORCA-010 bắt `eslint-disable max-lines` và `oxlint-disable-next-line max-lines`.
- [ ] Diff có một dòng cũ vi phạm + một dòng mới vi phạm → đúng 1 finding.
- [ ] ORCA-012 báo `frontend/src/lib/helpers.ts` mới, không báo tệp đã tồn tại (không phải `added`).
- [ ] `fingerprint` không đổi khi chèn dòng phía trên; đổi khi nội dung vi phạm đổi.
- [ ] Không tìm thấy script ở cả hai vị trí → `ruleResults[].status:"script_not_found"`, không finding; cổng nhận `unknown`.
- [ ] Script chạy đúng `cwd`; argv không có `--init`/`--prune`; không có đường nào cho client truyền pattern/đường dẫn.
- [ ] Pattern backtracking nặng bị cắt, agent không treo.
- [ ] ORCA-004..006 chỉ chạy khi tệp liên quan đổi (`skipped_scope` còn lại).

## 8. Kiểm thử

Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-rule-pack-schema.test.ts src/relay/quality-rule-diff-matcher.test.ts src/relay/quality-rule-diff-runner.test.ts src/relay/quality-rule-script-runner.test.ts src/relay/quality-rule-results.test.ts`; sau cùng `pnpm test`. Repo tạm cấu trúc rút gọn `config/scripts` + `desktop/config/scripts`; script giả thoát 0/1/hết hạn; bất biến: luật diff không ghi tệp/không dùng mạng.

## 9. Rủi ro và chưa kiểm chứng

- Chưa chạy script thật (trừ đọc mã 3 script); định dạng/thời gian/mã thoát của `verify-localization-*`, `audit-localization-coverage`, `check-feature-wall-assets` chưa biết (task 01).
- Báo nhầm cao ở ORCA-013/014/015; mặc định hạ mức.
- Cây repo đang lệch gốc/`desktop` (README v7 điểm 20); nếu đổi, định vị script phải đổi.
- Hex/`metaKey` chỉ quét `frontend/`; `desktop/src/renderer` có bản trùng (CR Q4).

## 10. Câu hỏi mở

1. ORCA-001..003 là bí danh (mặc định) hay chạy lại script với `ruleId` `ORCA-00x`?
2. Thêm `ruleResults` vào `QualityStepResult` ở hợp đồng.
3. `scope=worktree` với luật diff: `skipped` hay quét `HEAD..cây làm việc`?

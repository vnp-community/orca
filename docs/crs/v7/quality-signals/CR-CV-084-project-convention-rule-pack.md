# CR-CV-084 — Rule pack quy ước dự án Orca: biến script `check-*`, bước `pr.yml` và luật AGENTS.md thành luật chạy được trên diff

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-084 |
| **Tên** | Đóng gói quy ước của Orca thành rule pack có `ruleId` ổn định `ORCA-xxx`: luật loại "script" (chạy script sẵn có qua profile) và luật loại "diff" (agent tự quét dòng thêm trong diff); sinh `QualityFinding` category `convention` |
| **Loại** | Feature (agent + dữ liệu rule pack; không đổi backend ngoài ánh xạ finding) |
| **Priority** | 🟠 P1 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-06 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-081 (profile `repo-rules`, tiến độ, timeout, `CODEINTEL_ENV_NOT_READY`), CR-CV-082 (`QualityFinding`, `fingerprint`, parser), CR-CV-030/036 (danh sách tệp đổi, trạng thái `added/modified`, `isTest/isGenerated`) |
| **Mở khoá** | CR-CV-085 (kiểm tra `convention` trong cổng), CR-CV-087 (chú thích trên diff), CR-CV-089 |
| **Tác động** | `agent/src/relay` (file mới `quality-rule-pack-loader.ts`, `quality-rule-diff-matcher.ts`, `quality-rule-script-runner.ts`, `quality-rule-pack-orca.yaml`), tài liệu quy ước ở `guides/`; **không** sửa script `check-*` hiện có |

---

## 1. Bối cảnh và vấn đề

Đã đọc code/cấu hình ngày 2026-10-06 (đường dẫn tương đối gốc repo `/opt/repos/orca`).

### 1.1 "Chuẩn chất lượng" của Orca nằm rải rác

| Nguồn | Nội dung thật | Vị trí đã đọc |
|---|---|---|
| `.github/workflows/pr.yml` (154 dòng, job `verify`) | theo thứ tự: `oxlint --format github`; `check:styled-scrollbars`; `check:reliability-gates`; `check:max-lines-ratchet`; guard `.d.ts` (shell `find src/preload src/shared -name '*.d.ts'`); `check:feature-wall-assets`; `verify:macos-entitlements`; `typecheck`; **ma trận tương thích Git** (tải Git 2.25.5 từ kernel.org + `alpine/git` 2.38.1, 2.49.1 chạy `vitest ... src/shared/git-binary-compatibility.test.ts`); `pnpm test`; `build:unpack`; `smoke-packaged-cli.mjs` | `.github/workflows/pr.yml` |
| `package.json` gốc, script `lint` | `oxlint && lint:switch-exhaustiveness && check-styled-scrollbars && check:reliability-gates && check:max-lines-ratchet && verify:localization-catalog && verify:localization-coverage` | `package.json:14` |
| Script thật | `check-styled-scrollbars.mjs`, `check-reliability-gates.mjs`, `check-max-lines-ratchet.mjs`, `check-feature-wall-assets.mjs`, `verify-localization-catalog.mjs`, `audit-localization-coverage.mjs --check`, `verify-macos-entitlements.mjs`, `lint-react-doctor-changed.mjs`; cùng cấu hình `reliability-gates.jsonc`, `oxlint-switch-exhaustiveness.json`, `oxlint-react-doctor.json` | `desktop/config/scripts/`, `desktop/config/` |
| `AGENTS.md` | tên file mơ hồ; cấm `max-lines` disable và ngoại lệ chỉ qua `config/max-lines-baseline.txt`; không hex mới trong UI; phím tắt không cứng `metaKey`; Git 2.25 + `GitCapabilityCache`; chuỗi UI qua `translate()` (README v7 mục 6, STYLEGUIDE); GitLab không chỉ GitHub; `.d.ts` → `.ts` | `AGENTS.md` |
| `.oxlintrc.json` | `max-lines` 300 (mặc định), 400 (`*.tsx`), 600 (`*.mjs`), 800 cho test (script ratchet nhắc lại `defaultLimitForPath`) | `.oxlintrc.json:89-101`, `check-max-lines-ratchet.mjs` |

### 1.2 Lệch cấu trúc repo (phát hiện khi đọc, ảnh hưởng thiết kế)

1. **Script ở gốc và ở `desktop/` không giống nhau.** `config/scripts/` ở gốc chỉ có `check-max-lines-ratchet.mjs`, `check-max-lines-ratchet.test.mjs`, `rebuild-native-deps.mjs`. Các script `check-styled-scrollbars`, `check-reliability-gates`, `check-feature-wall-assets`, `verify-localization-*`… chỉ có ở **`desktop/config/scripts/`**. Nhưng `package.json` gốc (`lint`, `check:*`) và `pr.yml` vẫn gọi `config/scripts/<tên>.mjs` ở gốc → chúng **không chạy được từ gốc** theo cây hiện tại (chưa chạy thử vì cấm chạy lint trong CR này). Repo đã tách thành `frontend/`, `desktop/`, `agent/`, `backend/`, `packages/*` (`pnpm-workspace.yaml`) nhưng `package.json` gốc và `pr.yml` chưa theo kịp.
2. Hai bản `max-lines-baseline.txt`: gốc `config/max-lines-baseline.txt` (có các dòng `inline frontend/src/main/runtime/...` kèm ghi chú "NEEDS PR REVIEW") và `desktop/config/max-lines-baseline.txt` (359 dòng, 336 `inline` + 19 `mobile-config`). `diff` cho thấy gốc nhiều hơn 260 dòng so với bản `desktop`. Script ratchet đọc `config/max-lines-baseline.txt` **tương đối cwd** và `git ls-files '*.ts' '*.tsx' '*.mjs'` (quét toàn repo, không theo diff).
3. Guard `.d.ts` trong `pr.yml` tìm `src/preload src/shared` ở gốc; gốc **không có `src/`** (có `desktop/src/preload`, `desktop/src/shared`), nên lệnh `find` ra rỗng → guard luôn "pass". Tệp tham chiếu trong thông báo (`docs/preload-typecheck-hole.md`) cũng không tồn tại (đã `ls`).
4. Hệ quả: rule pack **không được giả định** đường dẫn script cố định; phải **phát hiện** vị trí (mục 2.3) và ghi `status:"script_not_found"` thay vì pass.

### 1.3 Vấn đề

Các luật này chỉ chạy ở CI sau khi có PR (research 11 §3.2) hoặc thủ công. Agent sinh code vi phạm quy ước (file `helpers.ts`, `eslint-disable max-lines`, hex cứng) chỉ bị phát hiện muộn. Cần (a) chạy sớm trên dev server, (b) kết quả chuẩn hoá thành `QualityFinding`, (c) chỉ xét **phần agent vừa thêm** để tránh nhiễu từ nợ cũ (baseline đã có ~355 tệp quá cỡ được miễn).

## 2. Giải pháp đề xuất

### 2.1 Hai loại luật

| Loại | Cách chạy | Ưu | Nhược |
|---|---|---|---|
| `script` | profile của CR-CV-081 chạy **đúng script có sẵn của repo** (`node <path>`), đọc mã thoát + stdout | Độ chính xác bằng CI; không trùng logic | Thực thi mã của worktree (rủi ro O11/research 11 §7, cùng mức `pnpm lint`); thô theo tệp/dòng; cần Node + dependency (`jsonc-parser`, TypeScript) |
| `diff` | agent tự quét **dòng được thêm** của tệp đổi theo mẫu cố định trong rule pack | Nhanh, không chạy mã repo, có tệp/dòng chính xác, chỉ phạt phần mới | Dễ báo nhầm; tự bảo trì |

Luật `diff` dùng **dữ liệu rule pack do Orca đóng gói cùng agent** (`quality-rule-pack-orca.yaml`), **không đọc định nghĩa luật từ worktree** (regex tuỳ ý từ nhánh không tin cậy = ReDoS/giả mạo). Ghi đè theo repo (thêm/tắt luật) để ngỏ, tách ở Q2.

### 2.2 Định dạng khai báo rule pack (mới)

YAML (agent đã có phụ thuộc `yaml ^2.8.4` và `jsonc-parser ^3.3.1` trong `agent/package.json`; **không cần thêm thư viện**). Lược đồ:

```yaml
version: 1                       # tăng khi đổi luật; ghi vào toolVersion của finding
pack: orca-conventions
rules:
  - id: ORCA-010                 # ổn định, không tái sử dụng, không đổi nghĩa
    title: "Không thêm max-lines disable"
    kind: diff                   # diff | script
    category: convention
    severity: error              # mặc định; tenant có thể hạ/nâng (CR-CV-085)
    enabled: true
    languages: [ts, tsx, mjs]
    scope:
      include: ["**/*.ts", "**/*.tsx", "**/*.mjs", "mobile/.oxlintrc.json"]
      exclude: ["**/node_modules/**", "**/*.test.*"]
      fileStatus: [added, modified]
    match:
      type: added-line-regex     # added-line-regex | added-file-name | file-content-regex
      pattern: '(?:eslint|oxlint)-disable(?:-next-line|-line)?\b[^\n]*\bmax-lines\b'
      maxLineLength: 1000
    message: "Thêm max-lines disable bị cấm; tách file thay vì bỏ qua."
    fixHint: "Tách file theo khái niệm (AGENTS.md: Lint Rules). Ngoại lệ chỉ qua config/max-lines-baseline.txt được review."
    source: { doc: "AGENTS.md#lint-rules-do-not-disable-max-lines", script: "check-max-lines-ratchet.mjs" }
    falsePositives: "Chuỗi trong test/fixture nhắc tới directive"
```

- `kind: script` có khối `script: { name: check-styled-scrollbars, locate: [...], args: [], parse: exit-code|github-annotations|json }`.
- Ràng buộc nạp: `id` khớp `^ORCA-\d{3}$`, duy nhất; `pattern` chỉ dùng tập con an toàn (không lookbehind lồng, không nhóm lặp lồng) và kiểm bằng thời gian chạy tối đa mỗi dòng; dòng dài hơn `maxLineLength` bị bỏ qua (và đếm `skippedLongLines`).
- `fingerprint` = `sha256(ruleId + "\0" + file + "\0" + normalize(matchedText))` rút gọn; **không chứa số dòng** (README 3.10). Cùng dòng vi phạm dịch chỗ vẫn là một finding; hai dòng giống hệt trong cùng tệp thêm hậu tố đếm.
- Kết quả: `QualityFinding{ruleId:"ORCA-010", category:"convention", severity, file, line, endLine, message, tool:"orca-rules", toolVersion:"orca-conventions@1", fixHint}`.

### 2.3 Chạy qua profile của CR-CV-081

Hai profile (tên do CR-CV-081 chốt; đề xuất): `repo-rules` (luật `diff`, nhanh, không cần cài dependency) và `repo-rules-scripts` (luật `script`, cần Node/pnpm và `node_modules`; thiếu → `CODEINTEL_ENV_NOT_READY`).

- **Đầu vào cho luật `diff`:** agent lấy `git diff --unified=0 <mergeBase>` (cùng cách CR-CV-083 2.5; nội bộ, không qua `git.exec`) + `git status --porcelain` cho tệp untracked (coi mọi dòng là thêm). Chỉ dòng **thêm** (và tên tệp mới) được so khớp → vi phạm có sẵn không bị tính (đúng ý "phần agent vừa làm"). Tệp binary, > 1 MiB, hoặc thuộc `isGenerated` bị bỏ qua.
- **Định vị script (`locate`):** thử theo thứ tự `<root>/config/scripts/<name>.mjs`, `<root>/desktop/config/scripts/<name>.mjs`; đọc hai bản `max-lines-baseline.txt` tương tự. Không thấy → `status:"script_not_found"`, finding `info` `ORCA-0xx-unavailable`, **không** coi là pass (cổng sẽ `unknown`, README 3.10).
- **Chạy script:** `node <path>` với `cwd` đúng thư mục chứa `config/` mà script giả định (`ratchet` dùng `config/max-lines-baseline.txt` tương đối cwd; `check-feature-wall-assets.mjs` dùng `import.meta.dirname/../..`), timeout từ profile, đầu ra cắt theo giới hạn của CR-CV-081. Quy ước mã thoát: 0 pass, khác 0 fail (đã thấy `check-max-lines-ratchet.mjs` in `::error::`; định dạng đầu ra các script khác **chưa kiểm chứng** → parser mặc định `exit-code`: một finding cấp repo, `file` rỗng, kèm 2 KiB cuối của stdout/stderr đã che secret).

### 2.4 Danh mục luật (khởi đầu, `ruleId` ổn định)

Cột "Cách kiểm": `S` = chạy script có sẵn; `D` = luật diff mới; `S+D` = cả hai (D cho phản hồi nhanh, S là xác nhận). Mức mặc định là **đề xuất**, chưa đo.

| ID | Luật | Nguồn | Cách kiểm | Mức | Báo nhầm dự kiến |
|---|---|---|---|---|---|
| ORCA-001 | Ratchet `max-lines`: tệp mới/đổi vượt ngân sách mà không nằm trong baseline | `check-max-lines-ratchet.mjs`, `pr.yml`, AGENTS.md | S (toàn repo, `git ls-files`); D thay thế qua ORCA-010/011 | error | Thấp khi chạy đúng cwd/baseline; **cao nếu dùng nhầm baseline gốc vs `desktop`** (1.2.2) |
| ORCA-002 | Scrollbar không tạo kiểu (JSX/CSS) | `check-styled-scrollbars.mjs` (+ `styled-scrollbars/styled-scrollbar-jsx-check.mjs`) | S | error | Thấp; chạy toàn cây, không theo diff |
| ORCA-003 | Manifest `reliability-gates.jsonc` hợp lệ | `check-reliability-gates.mjs` (dùng `jsonc-parser`) | S; chỉ chạy khi `reliability-gates.jsonc` hoặc mã gate đổi | error | Thấp |
| ORCA-004 | Catalog bản dịch hợp lệ | `verify-localization-catalog.mjs` | S; khi `**/i18n/**` đổi | error | Thấp |
| ORCA-005 | Độ phủ localization (chuỗi UI chưa qua `translate()`/catalog) | `audit-localization-coverage.mjs --check`, allowlist `localization-coverage-allowlist.json` | S; khi `*.tsx` ở renderer đổi | warning | Trung bình (allowlist cũ); chậm (dùng TypeScript API, comment trong script) |
| ORCA-006 | Ngân sách tài sản feature-wall (≤ 11 MiB, đủ 12 tile × 3 tệp) | `check-feature-wall-assets.mjs` | S; khi `resources/onboarding/feature-wall/**` đổi | error | Thấp |
| ORCA-007 | `.d.ts` do dự án sở hữu dưới `preload`/`shared` | bước shell trong `pr.yml`, AGENTS.md "Prefer `.ts` over `.d.ts`" | D (`added-file-name` `**/{preload,shared}/**/*.d.ts`, loại `node_modules`) | error | Thấp. Khác CI: đúng đường dẫn `desktop/src/...` (1.2.3) |
| ORCA-010 | Thêm `max-lines` disable (`eslint-disable`/`oxlint-disable`, cả `-line`) | AGENTS.md, ratchet `hasMaxLinesDisable` | D `added-line-regex` | error | Thấp-TB: chuỗi trong test/tài liệu; bỏ qua `*.test.*`, `*.md`, và file tự tham chiếu của ratchet |
| ORCA-011 | Nâng `max-lines` theo tệp trong `mobile/.oxlintrc.json` | AGENTS.md, ratchet (`mobile-config`) | D khi `mobile/.oxlintrc.json` đổi: so giá trị `max-lines` mỗi override giữa base và HEAD (parse JSON) | error | Thấp |
| ORCA-012 | Tên tệp/thư mục mơ hồ: `helpers`, `utils`, `common`, `misc`, `shared-stuff` (và `*-helpers.ts`, `*-utils.ts`) | AGENTS.md "File and Module Naming" | D `added-file-name`, chỉ tệp **mới/đổi tên** (không đụng tệp đã có) | warning | Trung bình: thư mục `common` hợp lệ có sẵn ở `backend-go/common` (module chung đã tồn tại, nên chỉ xét đường dẫn **mới thêm**, loại `backend-go/common/**` tồn tại) |
| ORCA-013 | Giá trị màu hex cứng trong UI mới | AGENTS.md "Design System", `STYLEGUIDE.md` (token ở `main.css`) | D `added-line-regex` `#(?:[0-9a-fA-F]{3,4}\|[0-9a-fA-F]{6}\|[0-9a-fA-F]{8})\b` trong `frontend/src/renderer/**/*.{ts,tsx,css}`, loại `main.css`, `*.test.*`, neo `href="#..."`, `url(#...)` | warning | **Cao** (SVG `fill`, regex, id băm, CSS biến); giữ warning, cho phép hạ `info` |
| ORCA-014 | `metaKey` cứng cho phím tắt | AGENTS.md "Cross-Platform Support" | D `added-line-regex` `\.metaKey\b` ở renderer/main; bỏ qua nếu **cùng tệp** có chuỗi `navigator.userAgent.includes('Mac')`/`isMac`/`CmdOrCtrl` | warning | Cao-TB: dùng hợp lệ trong nhánh nền tảng đã kiểm |
| ORCA-015 | Lệnh Git mới thiếu đường tương thích | AGENTS.md "Git Binary Compatibility", `guides/reference/git-compatibility.md` (baseline 2.25) | D: tệp đổi có chuỗi cờ/lệnh thuộc bảng "cần Git mới hơn" (lấy từ bảng Capabilities của tài liệu, ví dụ `worktree list -z`, `rev-parse --path-format`, `for-each-ref --exclude`) mà không tham chiếu `GitCapabilityCache`; **chỉ khi** tệp import/gọi `git` | info | Cao; chỉ nhắc "kiểm tra tương thích", không kết luận |
| ORCA-016 | Ma trận tương thích Git (Git 2.25.5/2.38.1/2.49.1) | `pr.yml`, `src/shared/git-binary-compatibility.test.ts` | **không đóng gói** (tải mã nguồn Git, Docker) — chỉ gợi ý "chạy ở CI" khi ORCA-015 báo | n/a | n/a |
| ORCA-017 | Chuỗi UI JSX mới không qua `translate()` | AGENTS.md/README v7 mục 6; STYLEGUIDE | Dùng ORCA-005 (S) làm nguồn chính; **không** viết regex JSX riêng trong bản đầu | — | Ghi nhận để tránh trùng; regex JSX ngoài phạm vi (báo nhầm rất cao) |

Luật AGENTS.md **không kiểm máy được** (không đưa vào pack, ghi rõ ở tài liệu): "SSH use case", "GitLab không chỉ GitHub", "nhận xét giải thích *why*", "rate limit `gh`". Luật `tenant_id` trong SQL thuộc CR-CV-038 (`sql.missing-tenant-filter`), không nhân đôi ở đây.

**Quan hệ với lint hiện có:** `oxlint`, `tsc`, `lint:switch-exhaustiveness` đã có parser ở CR-CV-082; rule pack **không** chạy lại chúng (tránh finding trùng).

### 2.5 Hợp nhất với CR-CV-082 và cổng

- Finding dùng chung bảng `quality_findings`; `category:"convention"`. Chỉ khác ở `tool:"orca-rules"`. `fingerprint` ổn định (2.2) cho phép `waive` (CR-CV-085) và `finding_dismissals`.
- Mỗi lần chạy trả `ruleResults[]` gồm cả luật **không chạy được**: `{ruleId, status:"ran"|"skipped_scope"|"script_not_found"|"env_not_ready"|"disabled", durationMs}`. Thiếu dữ liệu thì cổng `unknown` chứ không `pass`.
- `severity` mặc định ở pack; tenant ghi đè qua `QualityProfile` của CR-CV-085 (rule pack không tự quyết chặn: chế độ `inform`, O9).
- Phiên bản: `toolVersion = "orca-conventions@<version>"`; đổi luật làm tăng `version`, fixture vàng cập nhật (CR-CV-070 mở rộng).

### 2.6 An toàn và hiệu năng

- Luật `diff` không thực thi mã repo; chỉ đọc đầu ra `git diff`. Luật `script` chạy qua cổng chạy có giới hạn của CR-CV-081 (timeout, CPU/RAM, không biến môi trường secret, người kích hoạt có quyền ghi).
- Đầu ra của script có thể chứa đường dẫn tuyệt đối/secret: che theo quy tắc chung (README mục 6) trước khi lưu.
- Luật `diff`: ước ≤ vài trăm ms cho diff điển hình; trần mỗi lần chạy: ≤ 5 000 tệp, ≤ 20 MiB diff; vượt → `truncated:true`.
- Không lưu mã nguồn trong finding ngoài `matchedText` đã cắt ≤ 200 ký tự và che chuỗi dạng secret.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| R1 | Pack do Orca đóng gói, không đọc luật từ worktree | Chống regex độc hại/giả mạo từ nhánh không tin cậy |
| R2 | Luật `diff` chỉ xét dòng **thêm** | Nợ cũ (baseline ~355 tệp) không gây nhiễu; khớp "agent vừa làm gì" |
| R3 | Không viết lại logic script phức tạp (scrollbar, reliability, localization) | Tránh lệch với CI; chạy script thật |
| R4 | Định vị script động và báo `script_not_found` | Cây repo hiện lệch gốc/`desktop/` (1.2); không giả pass |
| R5 | `ruleId` `ORCA-xxx` bất biến, không dùng lại | Waiver/dismissal phải bền |
| R6 | Chỉ `convention`, không chạm `lint/typecheck` | Tránh trùng CR-CV-082 |
| R7 | Dùng `yaml`/`jsonc-parser` sẵn có của agent | Không thêm phụ thuộc (O12) |
| R8 | Ma trận Git và build/smoke không đóng gói | Cần tải/Docker/artefact, thuộc CI chính thức |

## 4. Tiêu chí chấp nhận

- [ ] Rule pack nạp được; id sai định dạng/trùng làm nạp thất bại với lỗi rõ ràng (kiểm thử).
- [ ] Mỗi luật trong 2.4 có fixture "vi phạm" và "không vi phạm"; ORCA-010 bắt cả `eslint-disable max-lines` lẫn `oxlint-disable-next-line max-lines`, bỏ qua dạng trong `-- Why:` của rule khác.
- [ ] Luật `diff` chỉ báo dòng thêm: diff chứa một dòng cũ vi phạm và một dòng mới vi phạm → 1 finding.
- [ ] ORCA-012 không báo cho tệp đã tồn tại (`backend-go/common/**`) và báo cho `frontend/src/lib/helpers.ts` mới.
- [ ] `fingerprint` không đổi khi chèn dòng phía trên vi phạm; đổi khi nội dung vi phạm đổi.
- [ ] Không tìm thấy script (`config/scripts/...` và `desktop/config/scripts/...`) → `ruleResults[].status:"script_not_found"`, không finding pass, cổng nhận `unknown`.
- [ ] Script chạy với đúng `cwd`; ratchet dùng baseline đúng thư mục (kiểm thử trên cây giả có hai baseline).
- [ ] Pattern gây backtracking nặng bị cắt bởi giới hạn thời gian/độ dài dòng, không treo agent.
- [ ] Không có đường nào cho client truyền pattern/đường dẫn script (chỉ tên profile).
- [ ] Đầu ra script chứa chuỗi giống token/DSN được che trước khi lưu.
- [ ] Luật ORCA-005/ORCA-006… chỉ chạy khi tệp liên quan trong diff (`scope`), ghi `skipped_scope` còn lại.
- [ ] Chạy trên SSH (qua agent) cho cùng kết quả như cục bộ; không dùng `\`/`/` cứng (dùng `path.join`).

## 5. Kiểm thử

- Đơn vị: bộ `quality-rule-diff-matcher` với bảng diff nhỏ (thêm/sửa/xoá/đổi tên/CRLF/dòng quá dài); parser `git diff -U0`.
- Fixture vàng: kết quả cả pack trên diff chuẩn; cập nhật khi `version` đổi (CR-CV-070).
- Tích hợp: repo git tạm cấu trúc rút gọn `config/scripts` và `desktop/config/scripts`; script giả mã thoát 0/1/hết hạn.
- Bất biến: không luật `diff` nào chạm mạng/ghi tệp.
- **Chưa chạy** các script thật trong CR này (cấm lint/test); S1 spike: chạy từng `check-*` trong `desktop/` ở nhánh sạch để lấy định dạng đầu ra, thời gian, mã thoát.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Chưa kiểm chứng** các script ở `desktop/config/scripts` chạy được độc lập từ `desktop/` (đường dẫn `ROOT = ..`, phụ thuộc `typescript` "TS 7 native CLI; AST consumers need legacy JS API" — chú thích trong `verify-localization-catalog.mjs`/`audit-localization-coverage.mjs`).
- Định dạng đầu ra, mã thoát và thời gian của các script **chưa đo** (ngoài `check-max-lines-ratchet.mjs`).
- **Cây repo đang lệch** (1.2): nếu repo sửa lại gốc/`desktop`, định vị script phải theo. Cần xác nhận cách repo thật sự chạy `pnpm lint` hiện nay.
- Báo nhầm cao ở ORCA-013/014/015 có thể làm mất niềm tin cổng (research 11 §7); mặc định `warning`/`info`, đo tỉ lệ bỏ qua qua CR-CV-095 trước khi nâng.
- Tên "luật cấm helpers/utils" có thể va chạm đường dẫn đã tồn tại hợp lệ; chỉ xét tệp mới.
- `config/max-lines-baseline.txt` ở gốc chứa mục `frontend/src/...` có thể chứa đường dẫn đã lỗi thời; ORCA-001 dựa vào bản đúng cây (Q3).
- Hex/`metaKey` chỉ quét `frontend/` (renderer); `desktop/src/renderer` có bản trùng (đã thấy `desktop/src/renderer/src/store/slices/github.ts`) — cần chốt đâu là nguồn thật (Q4).

## 7. Câu hỏi mở

- **Q1.** Mức mặc định các luật (đặc biệt ORCA-005, 013, 014) có ổn không, hay bắt đầu toàn bộ ở `info` để đo nhiễu?
- **Q2.** Có cho repo khai báo luật bổ sung (file trong repo) sau khi có cơ chế ký/duyệt không?
- **Q3.** Bản baseline nào là nguồn thật: `config/` (gốc) hay `desktop/config/`?
- **Q4.** Đường dẫn UI nguồn thật là `frontend/src/renderer` hay `desktop/src/renderer`?
- **Q5.** Có đặt bảng "lệnh/cờ Git cần Git mới hơn" (ORCA-015) ở dữ liệu máy đọc được cạnh `git-compatibility.md` để pack đọc không?
- **Q6.** Khi `pr.yml` đổi, ai cập nhật rule pack (liên kết `source` ↔ bước CI)?

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (O9-O13, 3.10, mục 8 #19)
- `/opt/repos/orca/docs/research/view-code/11-additions-for-quality-control.md` (B6, §7)
- `/opt/repos/orca/AGENTS.md`, `/opt/repos/orca/guides/STYLEGUIDE.md`, `/opt/repos/orca/guides/reference/git-compatibility.md`
- `/opt/repos/orca/.github/workflows/pr.yml`, `/opt/repos/orca/package.json`, `/opt/repos/orca/.oxlintrc.json`
- `/opt/repos/orca/config/scripts/check-max-lines-ratchet.mjs`, `/opt/repos/orca/config/max-lines-baseline.txt`
- `/opt/repos/orca/desktop/config/scripts/` (`check-styled-scrollbars.mjs`, `check-reliability-gates.mjs`, `check-feature-wall-assets.mjs`, `verify-localization-catalog.mjs`, `audit-localization-coverage.mjs`), `/opt/repos/orca/desktop/config/max-lines-baseline.txt`, `/opt/repos/orca/desktop/config/reliability-gates.jsonc`
- `/opt/repos/orca/agent/package.json` (`yaml`, `jsonc-parser`)
- `/opt/repos/orca/docs/crs/v7/code-intel-sources/CR-CV-038-contract-diff-and-static-security.md` (`sql.missing-tenant-filter`)
- Cùng folder: `./README.md`; CR-CV-081, CR-CV-082 (tham chiếu theo ID)

# CR-CV-091 — Quét bảo mật và phụ thuộc (tuỳ chọn, mặc định tắt): lỗ hổng, bí mật trên diff, diff phụ thuộc/giấy phép

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-091 |
| **Tên** | Profile quét bảo mật/phụ thuộc chạy qua bộ chạy kiểm tra (CR-CV-081): lỗ hổng Go và pnpm, quét bí mật chỉ trên diff **không bao giờ lưu giá trị bí mật**, diff phụ thuộc/giấy phép từ `pnpm-lock.yaml`/`go.mod`/`go.sum`; parser → `QualityFinding` category `security` và `dependency`; bật/tắt theo tenant |
| **Loại** | Feature (tuỳ chọn; agent + cấu hình tenant) |
| **Priority** | ⚪ P2 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-06 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-081 (profile, `CODEINTEL_ENV_NOT_READY`, giới hạn tài nguyên), CR-CV-082 (`QualityFinding`, parser, fixture), CR-CV-013 (quyền, audit), `tenant_settings` (README mục 8 điều chỉnh 10). Sau CR-CV-085 theo thứ tự (README mục 5) |
| **Mở khoá** | CR-CV-085 (kiểm tra `security`/`dependency` trong cổng), CR-CV-090 (báo cáo), CR-CV-093 (AI cấm gửi bí mật) |
| **Tác động** | `agent/src/relay` (file mới `quality-parse-govulncheck.ts`, `quality-parse-osv-scanner.ts`, `quality-parse-secret-scanner.ts`, `quality-dependency-diff-lockfiles.ts`, `quality-secret-redaction.ts`), cấu hình profile; `code-intel-service` (cờ tenant, hiển thị). **Công cụ mới trên dev server, không thêm phụ thuộc vào repo Orca** |

---

## 1. Bối cảnh và vấn đề

Đã kiểm tra ngày 2026-10-06 (chỉ đọc):

1. **Repo hiện không có công cụ quét nào.** `grep` `govulncheck|gosec|gitleaks|semgrep|trivy|osv-scanner|codeql` trong `.github/`, `backend-go/Makefile`, `package.json` → không có kết quả; không có `.github/dependabot.yml`, không có `.gitleaks.toml`. Workflow CI: `pr.yml` và các `backend-go-*.yml` (chỉ build/vet/test). Kiểm tra `which` trên máy đang soạn chỉ thấy `golangci-lint` (`/home/ubuntu/go/bin`); không có `govulncheck`, `gosec`, `gitleaks`, `semgrep`, `trivy`, `osv-scanner` (**máy này chưa chắc là dev server mục tiêu**).
2. **Phụ thuộc phạm vi lớn:** `pnpm-lock.yaml` gốc (13 170 dòng), thêm `mobile/pnpm-lock.yaml` riêng; workspace pnpm gồm `frontend, backend, agent, desktop, emulator, packages/dev-agent-transport` (`pnpm-workspace.yaml`); Go có **21 `go.sum`** (đếm `find backend-go -maxdepth 3 -name go.sum`) vì mỗi service là một module trong `go.work`.
3. **Chỗ liên quan đã có:** `.golangci.yml` bật `gocritic`, `errorlint`, `staticcheck`... (không phải bảo mật chuyên dụng; `gosec` không bật); CI `pr.yml` đã có bước `git diff --exit-code package.json pnpm-lock.yaml` sau cài (phát hiện lockfile drift ở CI); CR-CV-038 có luật tĩnh riêng `sql.missing-tenant-filter` (không trùng); `gitnexus explain` tìm taint nhưng cần `analyze --pdg` (xem CR-CV-094).
4. README v7 O12 yêu cầu: công cụ/phụ thuộc mới **không thêm mặc định, cần duyệt riêng; quét bảo mật/phụ thuộc mặc định tắt**. Research 11 B5 và mục 7: mỗi công cụ cần cài trên dev server và duyệt.
5. Rủi ro riêng: kết quả quét bí mật **bản thân là dữ liệu nhạy cảm**; lưu giá trị bí mật vào DB, log, cache hay gửi cho AI sẽ biến công cụ phòng thủ thành nguồn rò rỉ (README mục 6: "Không đưa secret vào kết quả/cache/log").

## 2. Giải pháp đề xuất

### 2.1 Nguyên tắc

- **Mặc định tắt**: cờ `quality_security_scan_enabled` trong `tenant_settings`, mặc định `false`, chỉ có hiệu lực khi `code_intel_enabled` và `quality_gate_enabled` (cờ phụ O9) cùng bật. Bật cần quyền quản trị tenant (kiểm bằng CR-CV-013) và ghi audit.
- **Profile có tên, không lệnh tự do** (O11): `security-go-vuln`, `security-deps-osv`, `security-secrets-diff`, `dependency-diff`. Mỗi profile liệt kê công cụ + phiên bản chấp nhận được; thiếu công cụ → `CODEINTEL_ENV_NOT_READY` kèm `missingTools[]` và gợi ý cài (không tự cài; không chạy `curl | sh`).
- **Không thêm phụ thuộc vào repo Orca**: công cụ cài trên dev server; riêng `dependency-diff` tự parse, không cần công cụ.

### 2.2 Lựa chọn công cụ theo hệ (cần duyệt; chưa chạy thử)

Mục "chưa kiểm chứng" = tôi chưa chạy hay đọc `--help` của công cụ này trong CR; việc xác nhận cờ là bước spike S1.

| Mục tiêu | Go | pnpm/TS | Ghi chú |
|---|---|---|---|
| Lỗ hổng phụ thuộc | `govulncheck` (Go team; phân tích có/không **gọi** hàm lỗi, giảm nhiễu); chạy `-format json` theo từng module trong `go.work` | `osv-scanner` đọc `pnpm-lock.yaml` (hỗ trợ khai báo là chưa kiểm chứng cho phiên bản cài) | Cả hai cần mạng tới nguồn dữ liệu lỗ hổng (vuln.go.dev, osv.dev) hoặc DB offline: **quyết định chính sách ra ngoài** (Q2) |
| Lỗ hổng mã tự viết | `gosec` (tĩnh) — **không** đề xuất bước đầu: ồn, trùng `staticcheck`/`errorlint` | `semgrep` — nặng, cần luật; ngoài phạm vi | Để P3 |
| Bí mật | `gitleaks` hoặc tương đương, chạy trên **nội dung diff** (không lịch sử) với chế độ che (`--redact`) | như Go | Phương án không cần cài: bộ quét tích hợp của agent bằng vài mẫu định dạng cao (khoá riêng PEM, `AKIA…`, `ghp_…`, DSN có mật khẩu), entropy tắt; ít bắt hơn nhưng không thêm công cụ (Q3) |
| Giấy phép | `go-licenses` (ngoài; chưa kiểm chứng) | `pnpm licenses list --json` (cần `node_modules`; chưa kiểm chứng) | Chỉ làm khi người dùng yêu cầu; MVP không giấy phép |

Quy ước phiên bản: ghi `toolVersion` vào mọi finding (README mục 6); danh sách phiên bản được hỗ trợ nằm trong cấu hình profile, ngoài phạm vi sẽ bị `unsupported_tool_version`.

### 2.3 Profile và cách chạy

```yaml
security-go-vuln:
  kind: security
  modules: from-go-work            # đọc go.work; không nhận từ client
  scope: changed-modules           # mặc định chỉ module có go.mod/go.sum/mã đổi
  command: ["govulncheck", "-format", "json", "./..."]   # cwd = module
  timeoutSec: 600
  requires: ["govulncheck"]
  network: required                # nêu rõ để chính sách mạng quyết định
security-deps-osv:
  kind: security
  command: ["osv-scanner", "--format", "json", "--lockfile", "<lockfile>"]  # từng lockfile
  lockfiles: ["pnpm-lock.yaml", "mobile/pnpm-lock.yaml"]
  scope: changed-lockfiles
  requires: ["osv-scanner"]
  network: required
security-secrets-diff:
  kind: security
  input: added-lines-of-diff       # xem 2.5
  engine: builtin|gitleaks
dependency-diff:
  kind: dependency
  lockfiles: ["pnpm-lock.yaml", "mobile/pnpm-lock.yaml"]
  gomod: from-go-work
  requires: []
```

- Dòng lệnh trên là minh hoạ; **cờ chính xác phải xác nhận ở S1** (chưa chạy `--help`). Chạy trong cùng khuôn giới hạn của CR-CV-081 (timeout, CPU/RAM, môi trường đã lọc secret, thư mục tạm `0700`).
- `scope: changed-*`: quét bắt buộc chỉ khi tệp phụ thuộc đổi hoặc người dùng chọn `scope:"worktree"` (quét toàn bộ). Lý do: lỗ hổng sẵn có là nợ cũ, không phải do agent; `govulncheck` còn phải biên dịch module (chậm).
- Biên dịch Go cần dependency có sẵn/tải được; nếu `go build` không chạy được (thiếu module cache, offline) → `CODEINTEL_ENV_NOT_READY` với `reason:"go_modules_unavailable"`.

### 2.4 Parser → `QualityFinding`

Parser ở agent (hàm thuần: JSON công cụ vào, danh sách finding ra), fixture vàng theo phiên bản (CR-CV-082/070):

| Nguồn | `ruleId` | `category` | `severity` mặc định | Ghi chú |
|---|---|---|---|---|
| `govulncheck` (vuln được gọi) | `SEC-GOVULN/<GO-ID>` | `security` | `error` | `message` có module, phiên bản, `fixedVersion`; `file/line` từ vị trí gọi nếu có |
| `govulncheck` (chỉ import/require, không gọi) | `SEC-GOVULN/<GO-ID>` | `security` | `info` | giảm nhiễu; hạ bậc có chủ đích |
| `osv-scanner` | `SEC-OSV/<OSV-ID>` | `security` | theo CVSS: ≥ 7 `error`, 4–6.9 `warning`, thấp/không rõ `info` | `file` = lockfile; gom theo `(vuln, package)`; **phân biệt dev/prod** nếu công cụ cung cấp, nếu không thì ghi `scopeUnknown` |
| Quét bí mật | `SEC-SECRET/<loại>` (vd. `private-key`, `aws-access-key`, `generic-api-key`) | `security` | `error` | **xem 2.5**; `message` chỉ gồm loại + vị trí |
| Diff phụ thuộc | `DEP-ADDED`, `DEP-REMOVED`, `DEP-MAJOR-BUMP`, `DEP-DOWNGRADE`, `DEP-LOCKFILE-DRIFT`, `DEP-NEW-SOURCE` (phụ thuộc từ git URL/tarball) | `dependency` | `info`, trừ `DEP-LOCKFILE-DRIFT` và `DEP-NEW-SOURCE` là `warning` | giấy phép: `DEP-LICENSE-DENY` chỉ khi bật 2.6 |

`fingerprint` ổn định và không chứa số dòng (README 3.10): lỗ hổng = `sha256(ruleId + package + lockfileOrModule)`; bí mật = `sha256(ruleId + file + thứ tự xuất hiện trong tệp)` **không dẫn xuất từ giá trị**; phụ thuộc = `sha256(ruleId + package + từ/đến phiên bản)`.

### 2.5 Quét bí mật chỉ trên diff, không lưu giá trị

- **Đầu vào:** chỉ các **dòng được thêm** trong `git diff --unified=0 <mergeBase>` + tệp untracked (cùng cách CR-CV-083 2.5/CR-CV-084 2.3). Không quét lịch sử, không quét tệp không đổi, bỏ qua tệp nhị phân và `node_modules`, `*.lock`, `pnpm-lock.yaml`, `go.sum` (băm trông như khoá).
- **Giá trị không rời khỏi tiến trình quét:** nếu dùng `gitleaks`, bắt buộc chế độ che (`--redact`; xác nhận ở S1) và **parser từ chối** mọi bản ghi có trường `Secret`/`Match` chưa che (kiểm bằng khẳng định: chuỗi đã chuyển qua `quality-secret-redaction.ts`). Với bộ quét tích hợp, giá trị chỉ tồn tại trong biến cục bộ và được so mẫu, không chuyển ra ngoài hàm.
- **Kết quả chỉ chứa:** `ruleId` (loại), `file`, `line`, `endLine`, `message` cố định theo loại ("Có vẻ là AWS access key ID"), `fixHint` ("Thu hồi khoá, chuyển sang Vault/biến môi trường"), **không** có ngữ cảnh dòng, độ dài hay bốn ký tự đầu/cuối. (Độ dài/tiền tố đủ để dò ngược nên cấm.)
- **Không ghi đĩa:** đầu ra thô của công cụ đọc thẳng từ pipe/tệp tạm `0600` rồi xoá ngay; không vào log runner, không vào `quality.results` thô, không vào cache snapshot, không vào `agent_turns`/prompt AI (CR-CV-093 chặn bằng kiểm cùng bộ che).
- **Quy trình xử lý khi có phát hiện:** UI báo "bí mật có thể đã lộ trong worktree; thu hồi và xoay khoá", không có nút "xem giá trị". Bí mật đã commit vào lịch sử cần thu hồi, ngoài phạm vi (CR chỉ quét diff đang review).
- Kiểm thử có hệ thống để chặn hồi quy: một bộ test "canary" nhúng chuỗi bí mật giả có dấu hiệu nhận biết và khẳng định chuỗi đó **không** xuất hiện ở bất kỳ đầu ra/log/payload nào (mở rộng CR-CV-072 "che secret").
- `quality_waivers` (CR-CV-085) cho phép miễn một phát hiện (lý do, người, hết hạn); miễn dựa trên `fingerprint`, không trên giá trị.

### 2.6 Diff phụ thuộc và giấy phép từ lockfile (không cần công cụ ngoài)

`quality-dependency-diff-lockfiles.ts` (mới) so `base` và `head` của từng lockfile:

- **pnpm:** parse `pnpm-lock.yaml` bằng thư viện `yaml` đã có trong `agent/package.json`; lấy tập `packages`/`importers` (phiên bản lockfile cần xác nhận ở S1; cấu trúc khác nhau giữa v6/v9), tính tập thêm/bớt/đổi phiên bản theo `name@version`; đánh dấu bước nhảy major, hạ phiên bản, nguồn không phải registry (git/tarball). Nội dung `base` đọc bằng `git show <base>:<path>` nội bộ (agent) hoặc `git.branchDiff includePatch` (CR-CV-030) nếu đi qua backend; ưu tiên ở agent vì lockfile lớn (~13 k dòng).
- **Go:** parse `go.mod` (khối `require`, `replace`, `exclude`; cú pháp ổn định) cho từng module trong `go.work`; `go.sum` chỉ để phát hiện thay đổi (không phân tích băm). `replace` mới hoặc `replace` tới đường dẫn cục bộ → `warning`.
- **Lockfile drift:** `package.json` (của gói) đổi phần phụ thuộc mà lockfile không đổi (hoặc ngược lại) → `DEP-LOCKFILE-DRIFT` (`warning`) — phản chiếu bước `pr.yml` "git diff --exit-code package.json pnpm-lock.yaml" nhưng chạy sớm; không thay thế CI.
- **Giấy phép:** bật riêng (`licenses.enabled`), cần `node_modules` đã cài (`pnpm licenses list --json`) hoặc metadata registry; danh sách `deny`/`review` giấy phép do tenant cấu hình (mặc định rỗng, tức không phán xét). Mặc định MVP **tắt**; chưa kiểm chứng đầu ra `pnpm licenses`.
- Trần: ≤ 2 000 thay đổi phụ thuộc/lần, `truncated` + `totalCount`.

### 2.7 Bật/tắt, phân quyền, mạng

- Cờ `quality_security_scan_enabled` theo tenant (mặc định `false`); tuỳ chọn cờ theo project (`allowedProfiles`). Tắt → profile `security-*`/`dependency-diff` không xuất hiện trong `quality.listProfiles` (CR-CV-081) và `StartQualityRun` trả `CODEINTEL_DISABLED`.
- Người kích hoạt cần quyền ghi project (O11); quét `scope:"worktree"` toàn bộ cần quyền cao hơn (đề xuất admin project) vì tốn tài nguyên (Q4).
- **Mạng:** profile ghi `network: required`; nếu chính sách tenant cấm egress, trả `CODEINTEL_ENV_NOT_READY (reason: "network_policy")`, không thử âm thầm. Danh sách gói gửi tới dịch vụ ngoài (osv.dev, vuln.go.dev) là tên + phiên bản phụ thuộc (không phải mã nguồn): ghi rõ ở tài liệu để tenant quyết (Q2).
- Hạn mức đồng thời: tính vào hạn mức run của dev server (CR-CV-081/013); quét nặng (`govulncheck` biên dịch) xếp hàng thấp ưu tiên.

### 2.8 Báo nhầm dự kiến

| Nguồn | Báo nhầm | Giảm thiểu |
|---|---|---|
| Bí mật | Fixture/test, `.env.example`, tài liệu, băm/UUID | Loại đường dẫn `**/*.test.*`, `**/fixtures/**`, `docs/**`, `*.md` (mặc định, cấu hình được); chỉ mẫu định dạng cao ở bộ quét tích hợp; waiver theo fingerprint |
| `osv-scanner` | Lỗ hổng ở devDependency/công cụ build (Electron, vite) không vào bản chạy; không có `reachable` | Mặc định `warning` thay vì `error` khi không rõ; hiển thị `scopeUnknown`; gom theo gói |
| `govulncheck` | "chỉ import" so với "được gọi" | Hạ `info` cho import-only (2.4) |
| Diff phụ thuộc | `DEP-MAJOR-BUMP` do `pnpm update` hợp lệ | `info`, không chặn |
| Lặp 21 module | Cùng một dependency báo 21 lần | Gom theo `(vuln, module path)` và liệt kê module bị ảnh hưởng |

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| S1 | Mặc định tắt, bật theo tenant | O12; cần cài công cụ và chính sách mạng |
| S2 | Không tự cài công cụ | Chạy mã tải về = rủi ro chuỗi cung ứng; chỉ báo `ENV_NOT_READY` |
| S3 | Bí mật: chỉ vị trí + loại; không dẫn xuất từ giá trị (kể cả băm, độ dài) | Chặn rò rỉ ngược |
| S4 | Chỉ quét dòng thêm của diff cho bí mật; chỉ module/lockfile đổi cho lỗ hổng | Tránh nợ cũ, tiết kiệm tài nguyên |
| S5 | `dependency-diff` không cần công cụ | Có `yaml` sẵn; tín hiệu rẻ, ít rủi ro |
| S6 | `import-only` hạ `info` | Giảm nhiễu (tin cậy cổng, research 11 §7) |
| S7 | Không thêm `gosec`/`semgrep` bước đầu | Ồn/nặng; trùng một phần với CR-CV-038, golangci |

## 4. Tiêu chí chấp nhận

- [ ] Với `quality_security_scan_enabled=false`: `quality.listProfiles` không liệt kê profile bảo mật; `StartQualityRun` cho các profile đó trả `CODEINTEL_DISABLED`.
- [ ] Bật cờ cần quyền admin và sinh bản ghi audit (CR-CV-013).
- [ ] Công cụ thiếu → `CODEINTEL_ENV_NOT_READY` với `missingTools[]`; không có lệnh cài chạy ngầm.
- [ ] Parser `govulncheck` và `osv-scanner` đúng trên fixture theo phiên bản; vuln import-only có `severity:"info"`.
- [ ] Quét bí mật chỉ nhận dòng **thêm** của diff; dòng không đổi chứa bí mật giả không bị báo.
- [ ] **Test canary:** chuỗi bí mật giả đặt trong diff **không** xuất hiện ở bất kỳ: finding, `quality.results` thô, log agent/backend, `payload`, snapshot, audit, thông điệp lỗi, tệp tạm sau khi chạy (kiểm bằng tìm kiếm chuỗi).
- [ ] Finding bí mật không chứa độ dài, tiền tố/hậu tố, hay ngữ cảnh dòng.
- [ ] `fingerprint` bí mật không đổi khi chèn dòng phía trên; không phụ thuộc giá trị bí mật.
- [ ] `dependency-diff` đúng cho: thêm/bớt/nâng major/hạ phiên bản, `replace` mới trong `go.mod`, drift `package.json` ↔ lockfile, nguồn git/tarball.
- [ ] Lỗ hổng cùng một dependency ở nhiều module Go được gom một finding có danh sách module.
- [ ] Chính sách mạng cấm egress → lỗi `network_policy` rõ ràng, không treo.
- [ ] Quét `scope:"changed"` không chạy gì khi không có tệp phụ thuộc đổi (và ghi `skipped_scope`).
- [ ] Waiver theo `fingerprint` ẩn finding khỏi cổng nhưng vẫn còn trong lịch sử.

## 5. Kiểm thử

- Fixture vàng JSON từng công cụ theo phiên bản (mở rộng CR-CV-070); fixture `pnpm-lock.yaml` nhỏ (hai phiên bản lockfile nếu cần) và `go.mod` mẫu.
- Test canary bí mật (CR-CV-072), chạy cả đường lỗi (công cụ crash giữa chừng, timeout, huỷ) để chắc tệp tạm bị xoá.
- Kiểm thử bất biến "parser từ chối bản ghi chứa trường bí mật chưa che".
- Tích hợp: repo git tạm có thay đổi phụ thuộc, công cụ giả (stub) trả JSON mẫu.
- Không chạy lúc soạn CR; chỉ mô tả.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Chưa chạy** bất kỳ công cụ nào, chưa đọc `--help` của `govulncheck`/`osv-scanner`/`gitleaks` (không có trên máy soạn); cờ, định dạng JSON và hỗ trợ `pnpm-lock.yaml` của `osv-scanner` phải xác nhận ở S1.
- **Mạng/quyền riêng tư:** tên gói gửi ra dịch vụ ngoài; dev server có thể bị chặn egress.
- `govulncheck` cần biên dịch module: tốn CPU/RAM, cạnh tranh với agent đang chạy; chưa đo.
- Phiên bản định dạng `pnpm-lock.yaml` (v6/v9) và monorepo (nhiều `importers`) có thể làm `dependency-diff` sai; mobile có lockfile riêng.
- Quét bí mật tích hợp bắt ít hơn công cụ chuyên dụng; ngược lại `gitleaks` ồn; không có dữ liệu tỉ lệ báo nhầm thực tế (chưa đo).
- Che bí mật bất hoàn hảo: mẫu nhận diện mới có thể lọt; vì vậy nguyên tắc "không lưu bất cứ thứ gì dẫn xuất từ giá trị" quan trọng hơn che.
- Nếu worktree là nhánh không tin cậy, chạy `govulncheck`/`go build` thực thi codegen/`go:generate`? (`go build` không chạy `go:generate`, nhưng cgo/`replace` cục bộ có thể; mức rủi ro tương đương CR-CV-081, chưa phân tích riêng).
- Quan hệ với CR-CV-038 (`sql.missing-tenant-filter`) và taint của GitNexus (CR-CV-094) chưa thống nhất định dạng; `category:"security"` dùng chung.

## 7. Câu hỏi mở

- **Q1.** Duyệt công cụ nào: `govulncheck`, `osv-scanner`, `gitleaks` (hay chỉ bộ quét tích hợp)?
- **Q2.** Chính sách egress: cho phép gọi `vuln.go.dev`/`osv.dev` hay cần DB offline/mirror nội bộ?
- **Q3.** Chấp nhận bộ quét bí mật tích hợp (ít mẫu) làm mặc định, `gitleaks` là tuỳ chọn?
- **Q4.** Ai được chạy quét toàn bộ `scope:"worktree"`?
- **Q5.** Danh sách giấy phép `deny/review` của tổ chức?
- **Q6.** Có thêm `dependabot.yml`/CI quét song song (ngoài Orca) không — để Orca không thành "CI thứ hai"?

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (O9-O13, mục 3.10, mục 6)
- `/opt/repos/orca/docs/research/view-code/11-additions-for-quality-control.md` (B5, §7)
- `/opt/repos/orca/.github/workflows/pr.yml`, `/opt/repos/orca/backend-go/Makefile`, `/opt/repos/orca/backend-go/.golangci.yml`, `/opt/repos/orca/backend-go/go.work`
- `/opt/repos/orca/pnpm-lock.yaml`, `/opt/repos/orca/mobile/pnpm-lock.yaml`, `/opt/repos/orca/pnpm-workspace.yaml`, `/opt/repos/orca/agent/package.json` (`yaml`)
- `/opt/repos/orca/docs/crs/v7/code-intel-sources/CR-CV-038-contract-diff-and-static-security.md`
- `/opt/repos/orca/docs/crs/v7/quality-rollout/README.md` (CR-CV-072 che secret)
- `/opt/repos/orca/AGENTS.md`
- Cùng folder: `./README.md`, `./CR-CV-083-coverage-and-diff-coverage.md` (2.5 lấy dòng thêm), `./CR-CV-084-project-convention-rule-pack.md`; CR-CV-081, CR-CV-082 (theo ID)

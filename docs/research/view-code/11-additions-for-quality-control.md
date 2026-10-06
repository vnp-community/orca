# 11 — Cần bổ sung gì để xem code và kiểm soát chất lượng code sau khi agent sinh, bằng đồ hoạ trực quan

Trạng thái: nghiên cứu / đề xuất (2026-10-06). Trả lời câu hỏi: ngoài những gì đã có trong [01–10](./README.md) và series CR [v7](../../crs/v7/README.md), **còn thiếu gì** để biến "xem code" thành "kiểm soát chất lượng". Các khẳng định về repo đã kiểm tra ngày 2026-10-06 (ghi rõ phạm vi tìm kiếm); phần còn lại là đề xuất.

## 1. Nhận định chính

Series v7 (43 CR) trả lời câu hỏi **"agent đã đổi gì, ảnh hưởng tới đâu, cấu trúc/kiến trúc/ERD/luồng ra sao"**: đó là *hiểu* thay đổi. Chưa trả lời **"thay đổi này đạt chuẩn chưa"**: đó cần *tín hiệu chất lượng* (kết quả kiểm tra, độ phủ test, độ phức tạp, bảo mật, tuân thủ yêu cầu) và *cổng chất lượng* (ngưỡng, đạt/không đạt, miễn trừ). GitNexus và CodeGraph chỉ cho cấu trúc và quan hệ; chúng không chạy lint, test hay quét bảo mật.

Có 4 khoảng trống lớn, xếp theo mức ảnh hưởng:

1. **Index luôn chậm hơn code vừa sinh** (vận hành, ảnh hưởng mọi view).
2. **Thiếu tín hiệu chất lượng thật** (lint/typecheck/test/coverage/phức tạp/bảo mật).
3. **Thiếu cổng chất lượng và đối chiếu với yêu cầu** (ngưỡng, kết luận, truy vết tới Request/Task).
4. **Thiếu nền đồ hoạ** cho biểu đồ thống kê và bố cục đồ thị lớn.

## 2. Khoảng trống 1 — Index phải bắt kịp code agent vừa sinh

Review ngay sau khi agent xong, nên index cũ làm mọi view sai hoặc rỗng. Hiện trạng đã kiểm tra:

- `gitnexus analyze` Orca mất nhiều phút; GitNexus **không có index riêng cho worktree** (chỉ index của checkout chính đã đăng ký) — ghi nhận ở [CR-CV-001/012](../../crs/v7/README.md) mục 8 điểm 18.
- `codegraph status --json` có trường `worktreeMismatch` và `pendingChanges{added,modified,removed}`, và có lệnh `codegraph sync` (đồng bộ thay đổi); GitNexus có `parse-cache`/`parsedfile-cache` trong `.gitnexus/` (có thể hỗ trợ phân tích gia tăng; **chưa kiểm chứng**).
- Mặc định O3 của v7 là "người dùng bấm làm mới"; không đủ cho kiểm soát chất lượng.

**Cần bổ sung**

| # | Việc | Ghi chú |
|---|---|---|
| A1 | Chiến lược index cho worktree của agent: (a) index riêng từng worktree, (b) `codegraph sync` gia tăng + `analyze` ở nền, (c) phân tích nhanh chỉ các file đổi bằng bộ parse nhẹ rồi phủ lên index nền ở merge-base | Phải đo chi phí (thời gian, đĩa) trên Orca; chọn (b) làm mặc định nếu `sync` đủ nhanh (chưa đo) |
| A2 | Tự kích hoạt làm mới khi agent báo xong (sự kiện `agent.hook` / `agent-status`) thay vì chỉ thủ công | Đổi mặc định O3; cần hạn mức và huỷ khi agent chạy tiếp |
| A3 | Hiển thị rõ "kết luận dựa trên index nào" (commit, mức `exact`/`repo_root`/`stale`) trên mọi kết quả chất lượng | Tránh kết luận sai khi index lệch |

## 3. Khoảng trống 2 — Tín hiệu chất lượng thật

### 3.1 Repo đã có gì (kiểm tra 2026-10-06)

| Nhóm | Có sẵn | Nơi |
|---|---|---|
| Lint/format TS | `oxlint`, `oxfmt`; `lint:switch-exhaustiveness`, `lint:react-doctor` | `package.json` scripts |
| Typecheck | `tsc --noEmit` (node, cli, web) | `typecheck*` scripts |
| Test | `vitest` (unit), Playwright (e2e), Go `go test` (unit và `-tags=integration`) | `test*` scripts, `backend-go/Makefile` |
| Go | `go vet`, `golangci-lint` với `errcheck, gosimple, govet, ineffassign, staticcheck, unused, gofmt, goimports, bodyclose, noctx, errorlint, gocritic`; `buf lint`/`breaking`; `opa test` | `backend-go/.golangci.yml`, `Makefile` |
| Kiểm tra theo quy ước dự án | `check:max-lines-ratchet`, `check:styled-scrollbars`, `check:reliability-gates`, `verify:localization-coverage`, guard `.d.ts`, ma trận tương thích Git, ngân sách feature-wall | `desktop/config/scripts/*` (các script này **không** nằm ở `config/scripts/` gốc; xem lưu ý dưới bảng), `pr.yml` |
| CI | `pr.yml` (lint → typecheck → test → build) và 17 workflow `backend-go-*` | `.github/workflows` |

**Lưu ý đường dẫn (phát hiện khi soạn CR-CV-084, đã xác nhận 2026-10-06):** repo đã tách `frontend/`, `desktop/`, `agent/`, `backend/`. `config/scripts/` ở gốc chỉ còn `check-max-lines-ratchet.mjs`, `check-max-lines-ratchet.test.mjs`, `rebuild-native-deps.mjs`; các script `check-*` còn lại, `vitest.config.ts` và `max-lines-baseline.txt` nằm ở `desktop/config/`. `package.json` gốc và `.github/workflows/pr.yml` vẫn tham chiếu `config/...` nên một số script ở gốc có thể không chạy được như ghi (chưa chạy thử). Con số "354 dòng" của `max-lines-baseline.txt` ở dưới là của bản gốc; bản ở `desktop/config/` là bản khác.

### 3.2 Chưa thấy (trong phạm vi `.github`, `backend-go/Makefile`, `package.json`, `.golangci.yml`)

- **Độ phủ test:** không thấy cấu hình coverage (`@vitest/coverage-*`, `-coverprofile`). Chỉ có suy đoán "hàm có test phủ" từ cạnh test của GitNexus.
- **Độ phức tạp/trùng lặp:** `.golangci.yml` không bật `gocyclo`/`gocognit`/`dupl`; phía TS chưa thấy công cụ (có thể có luật trong oxlint, **chưa kiểm chứng**).
- **Bảo mật/phụ thuộc:** không thấy `govulncheck`, `gosec`, `gitleaks`, `semgrep`, `trivy`, `osv-scanner`, `codeql`; thư mục `.github` không có `dependabot.yml`.
- **Chạy kiểm tra trước khi có PR:** các kiểm tra chỉ chạy ở CI (sau khi có PR) hoặc do người dùng chạy tay; không có đường để UI hỏi "chạy kiểm tra trên worktree này ngay".

### 3.3 Cần bổ sung

| # | Việc | Lý do / ghi chú |
|---|---|---|
| B1 | **Bộ chạy kiểm tra** (quality runner) trên dev server: method hẹp của agent chạy các *profile kiểm tra* có tên (ví dụ `lint`, `typecheck`, `unit`, `go-vet`, `go-lint`, `proto`, `repo-rules`) trong worktree, có tiến trình, huỷ, timeout, giới hạn tài nguyên | Phải là profile có tên do cấu hình định nghĩa, **không** nhận lệnh tuỳ ý; nhưng lệnh thực chạy là script của chính repo (xem §7 về rủi ro) |
| B2 | **Chuẩn hoá kết quả** về một mô hình chung `QualityFinding{rule, severity, file, line, message, tool, fixHint?}` (gần SARIF); parser cho oxlint, tsc, vitest, go vet/test, golangci-lint, buf | Một mô hình để UI hiển thị và lọc đồng nhất; mỗi parser cần fixture theo phiên bản công cụ |
| B3 | **Độ phủ**: bật thu thập coverage (Go: `-coverprofile`; TS: thêm `@vitest/coverage-v8`, phụ thuộc mới cần duyệt) và tính *diff coverage* (độ phủ trên dòng đã đổi) | Nếu chưa bật được, giữ fallback cạnh test của GitNexus và ghi rõ "ước lượng" |
| B4 | **Độ phức tạp, trùng lặp, kích thước**: bật `gocyclo`/`gocognit`/`dupl` trong golangci-lint (hoặc đo bằng truy vấn đồ thị), thêm đo cho TS; kết hợp với `max-lines-baseline.txt` đã có (354 dòng) | Số liệu đưa vào hotspot (R3) |
| B5 | **Quét bảo mật/phụ thuộc** (tuỳ chọn): `govulncheck`, quét bí mật (gitleaks hoặc tương đương), `osv-scanner` cho `pnpm-lock.yaml`/`go.sum`, diff phụ thuộc/giấy phép | Chưa có trong repo; mỗi công cụ cần cài trên dev server và duyệt |
| B6 | **Đóng gói quy ước dự án thành rule pack**: các script `config/scripts/check-*` và luật AGENTS.md (tên file mơ hồ, `max-lines` disable, hex cứng, `metaKey` cứng, lệnh git không kiểm tương thích) thành các luật chạy được trên diff | Đây là "chuẩn chất lượng" riêng của Orca; hiện nằm rải rác trong script và tài liệu |
| B7 | **Kết quả CI/PR**: gộp kết quả CI (đã có `ChecksPanel`, `GitHubItemDialog`, polling checks) với kết quả chạy cục bộ; hỗ trợ GitLab, không chỉ GitHub (AGENTS.md) | Tránh hai nguồn sự thật mâu thuẫn; ghi rõ nguồn (cục bộ/CI) và commit |

## 4. Khoảng trống 3 — Cổng chất lượng và đối chiếu yêu cầu

| # | Việc | Ghi chú |
|---|---|---|
| C1 | **Mô hình cổng chất lượng**: `QualityProfile` (kiểm tra bắt buộc, ngưỡng coverage/phức tạp, mức severity chặn), kết luận `pass/warn/fail`, **lý do hiển thị được** cho từng kết luận, miễn trừ có người/lý do/hết hạn (mở rộng `finding_dismissals`) | Đặt theo tenant/project; cần quyền chỉnh |
| C2 | **Tích hợp vào luồng làm việc**: cảnh báo trước khi "Create PR"/commit nếu cổng `fail` (không chặn cứng ở MVP); hiển thị trạng thái cổng ở `SourceControl` và hàng agent trong dashboard | Tránh chặn bất ngờ; mặc định chỉ cảnh báo |
| C3 | **Truy vết yêu cầu**: nối thay đổi với Request/Task/Plan/Phase (series [v6](../../crs/v6/README.md): `task_sources`, `Request`, tiêu chí chấp nhận) và hiển thị "yêu cầu gì → phần code nào → test nào" | Cần dữ liệu tiêu chí chấp nhận có cấu trúc; hiện v6 chỉ là đề xuất |
| C4 | **Lịch sử và xu hướng**: lưu kết quả theo `(worktree, commit, lượt agent)` để vẽ xu hướng (số phát hiện, coverage, độ phức tạp) và so sánh giữa các lượt | Gắn với "mốc lượt" của [CR-CV-060](../../crs/v7/review-frontend/CR-CV-060-review-notes-send-to-agent-and-turn-compare.md) vì lượt agent chưa được lưu bền |
| C5 | **Dấu vết agent** (provenance): agent/model nào sinh, prompt, các lệnh đã chạy, *agent tự nói test pass nhưng kết quả chạy lại thế nào* | Nguồn khả dĩ: `agent.hook`, `agent-status`, phiên AI Vault; chưa kiểm chứng đủ dữ liệu; đối chiếu độc lập với B1 |
| C6 | **Đánh giá bằng AI** (tuỳ chọn): tóm tắt diff, giải thích rủi ro, đề xuất mục cần đọc; dùng đường `ai.complete` của agent đã có (relay); nhãn rõ "AI suy luận", không dùng làm căn cứ chặn | Cần khung nhắc và giới hạn dữ liệu gửi đi (không gửi secret) |
| C7 | **Báo cáo review xuất được**: Markdown/HTML ngắn (tóm tắt, phát hiện, cổng, ảnh đồ thị) dán vào mô tả PR (trung lập GitHub/GitLab) | Hữu ích cho người không mở Orca |

## 5. Khoảng trống 4 — Nền đồ hoạ

Hiện trạng frontend (kiểm tra 2026-10-06): có `@xyflow/react`, `mermaid`, `monaco-editor`, `@tanstack/react-virtual`, `react-resizable-panels`; `components/ui/` có card, table, progress, badge, tabs, tooltip, skeleton, command…, **không có** thư viện biểu đồ (không thấy `recharts`/`d3`/`visx`/`echarts` trong `package.json`) và không có component biểu đồ, cây, lưới dữ liệu hay timeline.

| # | Việc | Ghi chú |
|---|---|---|
| D1 | **Quyết định thư viện biểu đồ**: tự viết SVG (mặc định O5, đủ cho treemap, thanh, sparkline, điểm số) hay thêm thư viện; liệt kê hình cần vẽ trước khi quyết | Nếu thêm: cần duyệt, kích thước bundle, Electron + web |
| D2 | **Bố cục tự động cho đồ thị lớn**: xyflow không tự bố cục; bố cục tầng đơn giản đủ cho ≤ ~100 nút, trên đó cần `elkjs`/`dagre` hoặc bộ vẽ chuyên dụng (cytoscape/sigma) cho ≤ 1 500 nút | Chưa quyết (O5); nên làm thử trên dữ liệu Orca để biết đâu là giới hạn thực |
| D3 | **Bộ hình cho kiểm soát chất lượng**: bảng điểm cổng (scorecard), bản đồ nhiệt hotspot, ma trận phụ thuộc (DSM), biểu đồ xu hướng theo lượt, treemap phủ test/độ phức tạp, đồng hồ diff coverage | Mỗi hình cần quy tắc đọc rõ và mô tả bằng chữ |
| D4 | **Token màu cho mức nghiêm trọng và trạng thái** (lỗi/cảnh báo/thông tin/đạt, đã đổi/bị ảnh hưởng/chưa test) trong `main.css` (`:root`, `.dark`, `@theme inline`) | STYLEGUIDE cấm hex cứng và để màu cho trạng thái; chưa kiểm tra token hiện có đủ chưa |
| D5 | **Truy cập được**: không chỉ dựa vào màu (thêm hình dạng/nét/nhãn), tương phản đủ ở sáng/tối, điều hướng bàn phím, `prefers-reduced-motion`, mô tả văn bản cho mỗi đồ thị | Bắt buộc theo rubric STYLEGUIDE |
| D6 | **Chú thích tại chỗ**: hiện phát hiện (lint, test thất bại, coverage thiếu) ngay trên dòng diff Monaco, tái dùng cơ chế decorator của `diff-comments` | Cần API cuộn/làm nổi dòng cho `DiffViewer` (chưa có, xem [CR-CV-053](../../crs/v7/review-frontend/CR-CV-053-impact-lens-and-symbol-detail.md)) |
| D7 | **Hiệu năng giao diện**: ảo hoá danh sách phát hiện, vẽ lười, giới hạn nút, canvas/WebGL nếu SVG chậm; thử với dữ liệu cỡ Orca và độ trễ SSH | Cần ngân sách (xem [CR-CV-071](../../crs/v7/quality-rollout/CR-CV-071-performance-budgets-metrics-tracing.md)) |

## 6. Cần bổ sung ở backend / vận hành (ngoài nội dung 3–5)

| # | Việc | Ghi chú |
|---|---|---|
| E1 | Mở rộng `code-intel-service` (hoặc service riêng `quality-service`) lưu `quality_runs`, `quality_findings`, `quality_profiles`, `waivers`; hai dialect DB theo quy ước | Tách service nếu vòng đời/nhịp thay đổi khác; quyết định ở [07](./07-architecture-decisions.md) cần cập nhật |
| E2 | Công việc dài (test/lint hàng chục giây đến phút): chạy nền, truyền tiến độ, huỷ, giới hạn đồng thời theo dev server | Timeout mặc định 30 s của `Exec` không đủ; dùng cơ chế stream/notification như `codeintel.reindexProgress` |
| E3 | Sẵn sàng môi trường: worktree phải có dependency đã cài (`node_modules`, module Go) và công cụ cần thiết trước khi chạy kiểm tra; kiểm tra bằng `preflight` | Nếu thiếu, trả lỗi rõ thay vì kết quả sai |
| E4 | Phiên bản công cụ ghi vào kết quả; fixture vàng cho từng parser (mở rộng [CR-CV-070](../../crs/v7/quality-rollout/CR-CV-070-golden-fixtures-and-tool-contract-tests.md)) | Chống trôi định dạng |
| E5 | Đo hiệu quả tính năng: thời gian từ "agent xong" đến "quyết định", tỉ lệ phát hiện bị bỏ qua/được xử lý, báo nhầm; dùng telemetry thô theo mẫu `mcp-telemetry-events.ts` (chỉ enum/khoảng, không dữ liệu nhạy cảm) | Để biết cổng có hữu ích hay gây nhiễu |
| E6 | Đánh giá các tool đã có của GitNexus chưa dùng trong v7: `check`, `shape_check`, `api_impact`, `route_map`, `tool_map`, `explain`, `pdg_query`, `group_*` (đa repo) | Tên lấy từ danh sách tool MCP của GitNexus trong phiên này; **chưa chạy thử** từng tool, cần đánh giá xem thay được phần phân tích tự viết (CR-CV-037/038) không |

## 7. Rủi ro riêng của kiểm soát chất lượng

- **Chạy mã không tin cậy:** `pnpm lint/test`, `go test` thực thi script và mã test trong worktree (có thể do agent sinh, hoặc từ nhánh bên ngoài). Cần: chỉ chạy profile có tên được cấu hình, giới hạn thời gian/CPU/bộ nhớ, không truyền biến môi trường chứa secret, cân nhắc cách ly (container/user riêng) và quyết định ai được kích hoạt. Agent hiện đã chạy tiến trình tuỳ ý theo yêu cầu của người dùng, nên đây là *mở rộng bề mặt* chứ không phải bề mặt mới; vẫn cần chốt chính sách.
- **Báo nhầm làm mất niềm tin:** cổng chất lượng gây nhiễu sẽ bị bỏ qua. Bắt đầu ở chế độ *chỉ báo*, đo tỉ lệ bị bỏ qua (E5), chỉ nâng cấp thành chặn khi dữ liệu đủ tốt.
- **Kết luận sai từ index cũ** (khoảng trống 1): luôn nêu nguồn và độ tươi.
- **"Điểm số" gây hiểu lầm:** tránh một con số duy nhất; hiển thị các thành phần và lý do.
- **Tốn tài nguyên dev server:** test và index nặng cạnh công việc của agent; cần hạn mức và ưu tiên.
- **Phạm vi lan:** có thể nở thành "một CI thứ hai"; giữ ranh giới: *phản hồi nhanh trước PR trên dev server*, CI vẫn là cổng chính thức.

## 8. Ưu tiên đề xuất

| Mức | Việc | Lý do |
|---|---|---|
| Phải có để "kiểm soát chất lượng" có nghĩa | A1–A3 (index bắt kịp), B1–B2 (chạy kiểm tra + chuẩn hoá phát hiện), C1 (cổng chất lượng, chế độ chỉ báo), D3–D5 (hình cơ bản, token, truy cập), E2–E4 | Không có chúng, view chỉ là bức ảnh cũ của code và không có kết luận |
| Nên có | B3 (coverage), B6 (rule pack của Orca), B7 (gộp CI), C2 (cảnh báo trước PR), C4 (xu hướng theo lượt), C5 (provenance), C7 (báo cáo), D1–D2, D6, E5–E6 | Tăng giá trị rõ rệt, chi phí vừa phải |
| Sau | B4–B5 (phức tạp/bảo mật bằng công cụ mới), C3 (truy vết yêu cầu, phụ thuộc v6), C6 (AI review), D7 (tối ưu sâu) | Phụ thuộc công cụ/quyết định ngoài phạm vi hoặc chưa kiểm chứng |

## 9. Đề xuất CR bổ sung cho series v7 (chưa soạn)

Gợi ý thêm folder `quality-gate` (ID `CR-CV-080..`) và mở rộng vài CR hiện có. Chỉ là danh sách ý định; cần bạn xác nhận phạm vi trước khi soạn.

| Đề xuất | Nội dung | Liên quan |
|---|---|---|
| CR-CV-080 | Chiến lược index cho worktree của agent và tự làm mới khi agent xong (A1–A3) | sửa O3; mở rộng CR-CV-004, 012 |
| CR-CV-081 | Bộ chạy kiểm tra trên agent: profile có tên, tiến độ, huỷ, giới hạn tài nguyên (B1, E2, E3) | cùng nhóm `agent-codeintel` |
| CR-CV-082 | Mô hình `QualityFinding` và parser (oxlint, tsc, vitest, go vet/test, golangci-lint, buf) kèm fixture (B2, E4) | mở rộng CR-CV-070 |
| CR-CV-083 | Thu thập coverage và diff coverage (B3) | cần duyệt thêm phụ thuộc |
| CR-CV-084 | Rule pack quy ước dự án từ `config/scripts/check-*` và AGENTS.md (B6) | |
| CR-CV-085 | Cổng chất lượng: profile, kết luận, lý do, miễn trừ, lưu lịch sử theo lượt (C1, C4) | mở rộng CR-CV-011, 037 |
| CR-CV-086 | Gộp kết quả CI/PR (GitHub và GitLab) với kết quả cục bộ (B7) | `scm-integration-service`, `ChecksPanel` |
| CR-CV-087 | Frontend: scorecard cổng, chú thích phát hiện trên diff, biểu đồ xu hướng, token màu (C2, D3, D4, D6) | mở rộng CR-CV-051, 059 |
| CR-CV-088 | Nền đồ hoạ: quyết định thư viện biểu đồ/bố cục, bộ hình chuẩn, truy cập, hiệu năng (D1, D2, D5, D7) | mở rộng CR-CV-050 |
| CR-CV-089 | Dấu vết agent và đối chiếu "agent tự báo" với kết quả chạy lại (C5) | phụ thuộc dữ liệu lượt agent |
| CR-CV-090 | Báo cáo review xuất được (C7) | |
| Sau | Quét bảo mật/phụ thuộc (B5), truy vết yêu cầu v6 (C3), AI review (C6), đánh giá tool GitNexus chưa dùng (E6) | |

## 10. Điều bạn cần quyết định hoặc cung cấp

1. Định nghĩa "chất lượng đạt" của Orca: dùng bộ kiểm tra hiện có (lint/typecheck/test/script `check-*`) hay thêm coverage, phức tạp, bảo mật? Ngưỡng ban đầu là bao nhiêu?
2. Cổng chỉ báo hay được phép chặn (Create PR/commit)? Ai được chỉnh ngưỡng và miễn trừ?
3. Được phép thêm công cụ/phụ thuộc mới trên dev server và trong repo (coverage, quét bảo mật, thư viện biểu đồ/bố cục) không?
4. Chính sách chạy kiểm tra trên dev server: ai kích hoạt, cách ly, hạn mức tài nguyên, có cho phép trên nhánh không tin cậy không?
5. Chiến lược index cho worktree (A1): chấp nhận tốn đĩa/thời gian để index từng worktree, hay dùng gia tăng + phủ diff?
6. Có đưa dữ liệu lên LLM để đánh giá (C6) không; nếu có thì loại dữ liệu nào bị cấm?
7. Nguồn dữ liệu "yêu cầu" để truy vết (C3): chờ series v6 hay dùng `task_sources` hiện có?
8. Các mục E1–E17 ở [09](./09-external-inputs-required.md) vẫn còn hiệu lực (C4/ERD/lưu trữ, topology prod, `CODEOWNERS` chưa có trong repo).

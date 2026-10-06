# CR-CV-094 — Spike: đánh giá các tool GitNexus chưa dùng trong v7 (`check`, `shape_check`, `api_impact`, `route_map`, `tool_map`, `explain`, `pdg_query`, `group_*`)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-094 |
| **Tên** | Spike có thời hạn: với từng tool GitNexus chưa dùng, xác định mục đích, đường gọi (CLI hay chỉ MCP), điều kiện, và quyết định đi/không đi thay hoặc bổ sung phần phân tích tự viết của CR-CV-037/038 |
| **Loại** | Spike / nghiên cứu (không viết code sản phẩm) |
| **Priority** | ⚪ P2 |
| **Effort** | Small (5 ngày làm việc, một người) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-06 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-002 (cách agent gọi GitNexus, whitelist), CR-CV-037/038 (phần tự viết cần so sánh), CR-CV-070 (nơi nhận fixture nếu "đi"). Khuyến nghị làm sau CR-CV-085 ổn định (README mục 5) |
| **Mở khoá** | Quyết định giữ/bỏ phần tự viết của CR-CV-037 (vòng, mã chết), CR-CV-038 (tĩnh), hợp đồng `codeintel.structuralFacts` (CR-CV-001/002), CR-CV-041 (danh mục tool MCP) |
| **Tác động** | Chỉ tài liệu: kết quả và quyết định ghi vào mục 9 của chính CR này và thư mục mẫu đầu ra (2.4). Không sửa `agent/`, `backend-go/`, `frontend/` |

---

## 1. Bối cảnh và vấn đề

Research 11 E6 liệt kê các tool GitNexus "chưa dùng trong v7", với ghi chú "tên lấy từ danh sách tool MCP; **chưa chạy thử**". Đã kiểm chứng ngày 2026-10-06 (chỉ đọc):

1. **CLI `gitnexus 1.6.9`** (`gitnexus --version`; `gitnexus --help` liệt kê lệnh): chỉ có `setup, uninstall, analyze, index, serve, mcp, list, status, doctor, clean, remove, wiki, augment, publish, query, context, impact, trace, cypher, detect-changes, check, eval-server, group`. **Không có** lệnh CLI cho `shape_check`, `api_impact`, `route_map`, `tool_map`, `explain`, `pdg_query` (chạy `gitnexus shape-check --help` v.v. chỉ in lại trợ giúp chung vì lệnh không tồn tại). Các tool này hiện chỉ thấy ở **MCP** (`gitnexus mcp`).
2. **`gitnexus check`**: `--cycles` (phát hiện import vòng, "fail when any are found"), `--json`, `-r/--repo`, `--branch`. Mô tả MCP: "Currently detects directed cycles between File nodes connected by IMPORTS edges; deterministic cycle paths and a cycle count suitable for CI". CR-CV-037 đã đưa `gitnexus check --cycles --json -r <repo>` vào whitelist cho `codeintel.structuralFacts kind:"cycles"` và ghi "Trên Orca hiện có 93 vòng".
3. **`gitnexus group`**: `create, add, remove, list, status, sync, impact, query, contracts`. `group sync` "Sync Contract Registry — extract contracts and build cross-links" (cờ `--json`, `--exact-only`, `--skip-embeddings`, `--allow-stale`, `--verbose`) **ghi** Contract Registry (`contracts.json`), không thuần đọc; `group create` ghi `group.yaml`. Trên máy khảo sát `gitnexus group list` in "No groups configured".
4. **`explain` và `pdg_query`** cần chỉ mục dựng bằng `gitnexus analyze --pdg` ("opt-in; off by default"); `.gitnexus/meta.json` của Orca **không có** dấu hiệu lớp PDG (đếm `pdg` = 0), nên trên chỉ mục hiện tại hai tool trả "no taint layer / no PDG layer" (theo mô tả tool). `explain` liệt kê luồng taint theo loại sink (command-injection, code-injection, path-traversal, sql-injection, xss) — có cảnh báo "absent flows are NOT proof of safety". `pdg_query` có hai chế độ `controls` (điều kiện chạy; cờ `guard`) và `flows` (REACHING_DEF), luôn gắn một `target`.
5. **`route_map`, `shape_check`, `api_impact`** làm việc trên nút `Route` (có `responseKeys` trích từ lời gọi `.json({...})` khi lập chỉ mục) và người tiêu thụ; mô tả tool nói rõ nguồn là các route kiểu HTTP/Express-like. README v7 mục 1 ghi chỉ mục Orca có **92 `Route`** nhưng **chưa ai kiểm tra** chúng thuộc phần nào (TS `backend/`? gateway Go?). Hợp đồng chính của Orca là gRPC proto và kênh `wscompat` (CR-CV-032/038) — không phải route HTTP kiểu Express; khả năng các tool này vô dụng cho phần Go cao.
6. **`tool_map`**: "MCP/RPC tool definitions: handler files và mô tả". Orca có bảng tool ở `agent/src/relay/agent-tool-registry.ts` và `mcp-service`; có thể giúp danh mục tool (liên quan CR-CV-041, parity MCP ở CR-CV-040) — chưa thử.
7. **Ràng buộc kiến trúc v7:** D5/CR-CV-001 chỉ cho agent gọi **CLI** qua whitelist; "phiên MCP stdio dài hạn với GitNexus (phương án B của nghiên cứu 02)" nằm trong **phạm vi ngoài** của feature `agent-codeintel`. Vì sáu trong chín tool chỉ có ở MCP, "đi" đồng nghĩa mở lại quyết định này hoặc tìm đường tương đương qua `cypher`.
8. **An toàn khi chạy thử:** `gitnexus analyze` mặc định ghi `AGENTS.md`/`CLAUDE.md`/skills vào cây làm việc và ghi vào `.gitnexus/`; chỉ mục Orca có nhiều tệp `lbug.wal.missing-shadow.*` và nhiều tiến trình MCP có thể giữ DB (`agent-codeintel/README.md` F9). Vì vậy **không chạy `analyze --pdg` trên `/opt/repos/orca`**; spike làm trên bản sao (2.3).

Mục tiêu spike: trả lời bằng chứng "tool nào đáng đưa vào v7, tool nào bỏ", thay vì suy đoán, trước khi CR-CV-037/038 đầu tư thêm vào phần tự viết trùng chức năng.

## 2. Giải pháp đề xuất

### 2.1 Bảng đánh giá cần điền (kết quả vào mục 9)

Cột "Đường gọi" ghi điều đã kiểm chứng ở 1; các cột còn lại là **câu hỏi spike**.

| Tool | Mục đích (theo mô tả công cụ) | Đường gọi đã kiểm chứng | Có thể thay/bổ sung | Điều kiện |
|---|---|---|---|---|
| `check` | Vòng import giữa các `File` (`IMPORTS`) | CLI: `gitnexus check --cycles --json -r <repo> [--branch]` (`--help` đã đọc) | Thay Tarjan SCC dự phòng của CR-CV-037 §2.3 (đã đang dùng làm nguồn chính) | Chỉ mục thường; không `--pdg` |
| `shape_check` | So khoá phản hồi của route với thuộc tính mà consumer truy cập (MISMATCH) | **Chỉ MCP** | Có thể bổ sung CR-CV-038 cho route HTTP TS nếu Orca có (chưa rõ); không thay so sánh proto/`wscompat` | Cần `Route` có `responseKeys` và consumer |
| `api_impact` | Báo cáo tác động trước khi sửa route (consumer, middleware, luồng, rủi ro LOW/MEDIUM/HIGH) | **Chỉ MCP** | Có thể làm giàu `ChangeOverlay.touchedContracts[kind:"route"]` của CR-CV-036 | Như trên |
| `route_map` | Route ↔ handler ↔ consumer, chuỗi middleware | **Chỉ MCP** (README v7 CR-CV-002 đã có `codeintel.routes` qua Cypher `RT_LIST`) | Có thể trùng `codeintel.routes`; kiểm xem thêm gì (consumer/middleware) | Như trên |
| `tool_map` | Định nghĩa tool MCP/RPC, file xử lý, mô tả | **Chỉ MCP** | Có thể phục vụ danh mục tool cho CR-CV-041 hoặc kiểm tra "tool đăng ký ↔ tài liệu" | Cần nút `Tool` trong chỉ mục (chưa biết Orca có không) |
| `explain` | Phát hiện taint (nguồn → sink) và đường đi | **Chỉ MCP** | Bổ sung finding `category:"security"` (CR-CV-091/038); **không** thay `sql.missing-tenant-filter` | **Cần `analyze --pdg`**; chưa biết hỗ trợ ngôn ngữ nào (Go/TS?) |
| `pdg_query` | "Điều kiện nào chặn X chạy", "biến Y chảy đâu" trong một hàm | **Chỉ MCP** | Ứng viên cho panel chi tiết symbol (CR-CV-053), không cho cổng | Cần `--pdg`; luôn có `target` |
| `group_list` | Liệt kê nhóm repo | CLI: `gitnexus group list [name]` | Không áp dụng cho Orca một repo (nhưng xem câu hỏi dưới) | Nhóm phải được tạo trước |
| `group_sync` | Dựng Contract Registry, liên kết hợp đồng giữa repo | CLI: `gitnexus group sync <name> [--json ...]` — **ghi** dữ liệu | Có thể bổ sung nối hợp đồng liên-repo nếu v7 sau này review nhiều repo | Cần group + chỉ mục các repo thành viên |

### 2.2 Câu hỏi spike (phải trả lời được)

**Chung**
- G1. Đầu ra có **máy đọc được ổn định** không (JSON thật hay markdown như `cypher`)? Có `exit code` đúng khi lỗi không (xem README mục 8 #7: `cypher` lỗi vẫn `exit 0`)?
- G2. Thời gian và RAM mỗi lần gọi trên chỉ mục cỡ Orca (247 556 nút), qua CLI (~1,8 s/lần đã đo ở khảo sát) hoặc qua phiên MCP; có vượt ngân sách của CR-CV-071 không?
- G3. Có đọc-chỉ đảm bảo không (tool nào ghi gì ở đâu)? Có chọn được repo bằng `-r` an toàn như CR-CV-001 F4 không?
- G4. Nếu chỉ có MCP: chi phí để đưa một phiên `gitnexus mcp` stdio vào agent (vòng đời, đồng thời, nhiều tiến trình giữ DB) so với lợi ích; có đường tương đương bằng `cypher` mẫu cố định không?

**Theo tool**
- `check`: JSON gồm gì (`cycles[]`, `cycleCount`)? Khớp với 93 vòng đã đo ở CR-CV-037? So với tính SCC tự viết: giống/khác ở những vòng nào? Có tôn trọng `.gitnexusignore`/loại thư mục sinh? Có `--branch` cho worktree không?
- `route_map`/`shape_check`/`api_impact`: 92 `Route` của Orca là gì (thư mục nào, ngôn ngữ nào)? Có `responseKeys`? Có consumer? Có tìm ra bất kỳ MISMATCH có thật nào? Có phủ gRPC/`wscompat` không (dự kiến không)? Nếu không phủ Go thì giá trị còn lại là gì?
- `tool_map`: trả những tool nào của Orca (agent tool registry, `mcp-service`)? So với nguồn thật (`agent-tool-registry.ts`, `backend-go/services/mcp-service`) khớp bao nhiêu phần trăm?
- `explain`/`pdg_query`: `analyze --pdg` trên bản sao tốn bao lâu/dung lượng (so chỉ mục thường)? Hỗ trợ Go và TS không? Với một mẫu tiêm lỗi biết trước (ví dụ nối chuỗi SQL, `exec.Command` với đầu vào request) có bắt được không; tỉ lệ báo nhầm trên mẫu 20 finding ngẫu nhiên? Có biểu diễn được "thiếu bộ lọc `tenant_id`" (CR-CV-038 §2.5)? (Dự kiến: không — đó không phải taint.)
- `group_*`: có tình huống thực của Orca cần nhiều repo (ví dụ khi review PR chạm `frontend` và `backend-go` nếu tách repo) không? Hiện là một repo (`orca`, 247 556 nút, `meta.json`), nên nhiều khả năng **không đi**.

### 2.3 Cách chạy (an toàn, chỉ-đọc với repo thật)

1. Không chạm `/opt/repos/orca`: tạo **bản sao thử** (`git worktree add` hoặc `git clone --shared` vào thư mục tạm), `gitnexus analyze --index-only --skip-agents-md` (và biến thể `--pdg`) **chỉ trên bản sao** — chi phí/ghi vào `.gitnexus/` của bản sao. (Spike này khác CR-CV-004 mục "chưa ai chạy `analyze`": nó chính là một đo `analyze` ban đầu; nên điều phối cùng.)
2. Với chỉ mục thật `/opt/repos/orca` (chỉ-đọc): lệnh được phép là `--help`, `--version`, `gitnexus list`, `gitnexus status`, `gitnexus check --cycles --json -r <path>` (đọc DB; cân nhắc khoá/WAL, chạy lúc không có phiên MCP khác — **chưa thử**), `gitnexus group list`.
3. Tool MCP: gọi qua một phiên `gitnexus mcp` stdio cục bộ trên bản sao hoặc qua MCP client có sẵn; ghi lại yêu cầu/đáp ứng nguyên văn.
4. Mỗi lần gọi ghi: lệnh/tham số, phiên bản (`gitnexus --version`), thời gian, kích thước đầu ra, mã thoát, chỉ mục dùng (commit, `indexedAt`, `--pdg` hay không).

### 2.4 Cách lưu mẫu đầu ra

- Thư mục (mới, khi thực hiện spike): `/opt/repos/orca/docs/research/view-code/spikes/094-gitnexus-unused-tools/` — mỗi tool một tệp `<tool>.<yyyy-mm-dd>.json|md` chứa **mẫu đã cắt** (≤ 200 KiB/tệp, nhiều nhất 50 phần tử mỗi mảng) kèm đầu trang ghi `command`, `toolVersion`, `index commit`, `elapsedMs`, `exitCode`.
- Che đường dẫn tuyệt đối ngoài repo, tên người dùng, token, giá trị cấu hình; **không** dán mã nguồn dài (chỉ vị trí `file:line`).
- Bảng tổng kết một trang `RESULTS.md` (kết quả từng câu hỏi ở 2.2, thời gian đo, nhận định).
- Nếu quyết định **đi**: mẫu được thăng cấp thành fixture vàng của CR-CV-070 (kèm phiên bản công cụ) trong cùng PR triển khai, không để ở thư mục research.

### 2.5 Quyết định đi/không đi (tiêu chí)

Một tool **đi** khi đủ cả:
1. **Giá trị:** trả thông tin mà phần tự viết của CR-CV-037/038 hoặc CR-CV-036 không có, hoặc thay được một phần tự viết với **độ chính xác ≥** (so với sự thật kiểm tay trên ≥ 20 mẫu) và chi phí bảo trì thấp hơn.
2. **Dùng được:** đầu ra máy đọc được ổn định (G1), chạy trong ngân sách thời gian (G2) và an toàn (G3).
3. **Khớp kiến trúc:** gọi được qua CLI cố định hoặc cách khác tương đương đã được chấp thuận (G4); nếu cần phiên MCP thì phải có quyết định riêng sửa D5/phạm vi ngoài của `agent-codeintel`.
4. **Không buộc chi phí chỉ mục lớn:** với `explain`/`pdg_query`, chi phí thêm của `--pdg` chấp nhận được (thời gian, dung lượng) hoặc chỉ bật tuỳ chọn.

Ngược lại **không đi**; ghi lý do và điều kiện xem lại (ví dụ phiên bản GitNexus mới, ngôn ngữ được hỗ trợ). Kết quả trung gian "đi có điều kiện" phải nêu điều kiện đo được.

### 2.6 Thời hạn

Hộp thời gian **5 ngày làm việc**: ngày 1 chuẩn bị bản sao + `check`; ngày 2 `route_map/shape_check/api_impact/tool_map` (mốc kiểm tra: nếu 92 `Route` không liên quan Orca, dừng ba tool route-liên-quan); ngày 3–4 `--pdg` + `explain/pdg_query`; ngày 5 `group_*`, tổng kết và quyết định. Hết hạn mà chưa đủ bằng chứng → mặc định **không đi** cho phần còn lại và ghi điểm chưa kiểm chứng; không gia hạn mà không có quyết định mới.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| P1 | Spike tài liệu, không code sản phẩm | Phạm vi nhỏ, tránh đầu tư trước khi có bằng chứng |
| P2 | Thử trên bản sao, không chạy `analyze` ở repo thật | Tránh ghi `AGENTS.md`/`CLAUDE.md` và DB bẩn (agent-codeintel F9) |
| P3 | Mặc định "không đi" khi hết hạn | Giữ v7 gọn; tool MCP-only kéo theo quyết định kiến trúc |
| P4 | Lưu mẫu cắt gọn và che | Tài liệu không phình, không lộ dữ liệu |

## 4. Tiêu chí chấp nhận (cho spike)

- [ ] Mỗi trong 9 tool có hàng điền đủ trong bảng mục 9: mục đích, đường gọi, kết quả thử, quyết định (đi / đi có điều kiện / không đi) và lý do.
- [ ] Các câu hỏi G1–G4 có câu trả lời kèm số đo (thời gian, kích thước đầu ra) hoặc ghi rõ "không đo được vì …".
- [ ] `check`: đã so sánh vòng với 93 vòng của CR-CV-037 và kết luận giữ/bỏ Tarjan dự phòng.
- [ ] 92 `Route` của Orca được liệt kê theo nguồn (thư mục/ngôn ngữ) và kết luận nhóm route-liên-quan.
- [ ] `--pdg`: thời gian, dung lượng chỉ mục thêm, ngôn ngữ hỗ trợ, và kết quả trên ≥ 3 mẫu lỗi biết trước + 20 mẫu ngẫu nhiên (tỉ lệ báo nhầm).
- [ ] Mẫu đầu ra lưu đúng 2.4, đã che đường dẫn/secret, kèm phiên bản công cụ.
- [ ] Không có thay đổi nào ở `/opt/repos/orca` ngoài thư mục tài liệu (kiểm `git status` trước/sau; không có thay đổi `.gitnexus/`, `AGENTS.md`, `CLAUDE.md` do `analyze`).
- [ ] Quyết định được phản ánh bằng một CR cập nhật (CR-CV-037/038/001/002/041) hoặc ghi "không cần đổi".
- [ ] Nếu có tool chỉ MCP được "đi": đề xuất đường tích hợp và nêu đúng mục D5/phạm vi ngoài cần sửa.

## 5. Kiểm thử

Không có kiểm thử mã. Kiểm tra chất lượng spike: (1) người thứ hai chạy lại ít nhất `check` và một tool MCP bằng lệnh đã ghi, ra kết quả cùng bậc; (2) độ chính xác đo trên mẫu có đáp án kiểm tay (ghi cách chọn mẫu); (3) soát lại để chắc không lộ secret trong mẫu lưu.

## 6. Rủi ro và điểm chưa kiểm chứng

- Chưa chạy thử **bất kỳ** tool nào ngoài `--help`/`--version`; mọi mô tả "mục đích" lấy từ mô tả công cụ và `--help`, chưa phải hành vi thực.
- Phiên bản `1.6.9` có thể khác bản MCP đang chạy ở nơi khác; bảng đường gọi phải lặp lại khi nâng phiên bản.
- `analyze --pdg` có thể tốn rất nhiều thời gian/dung lượng trên Orca (chưa đo) và có thể chỉ hỗ trợ một số ngôn ngữ.
- `check` trên chỉ mục đang được tiến trình khác giữ có thể xung đột (WAL); chưa thử.
- 92 `Route` có thể phần lớn không phải API của Orca (ví dụ từ test hoặc thư viện), làm kết quả `route_map` nhiễu.
- `group sync` ghi ngoài repo (`~/.gitnexus`?) — vị trí ghi chưa kiểm; chỉ thử trên môi trường tạm với `HOME` riêng.
- Quyết định "đi" với tool MCP-only mở lại D5; không được âm thầm bỏ whitelist.

## 7. Câu hỏi mở

- **Q1.** Có chấp nhận một phiên MCP stdio dài hạn (đã loại ở `agent-codeintel`) nếu bằng chứng ủng hộ không?
- **Q2.** Orca có cần phân tích taint (`explain`) trong v7 hay để tới series bảo mật (CR-CV-091)?
- **Q3.** Ai sở hữu việc cập nhật CR-CV-037/038 sau spike?
- **Q4.** Chỉ mục `--pdg` có đặt thành tuỳ chọn theo repo binding (chi phí) không?

## 8. Tham chiếu

- `/opt/repos/orca/docs/research/view-code/11-additions-for-quality-control.md` (E6)
- `/opt/repos/orca/docs/crs/v7/README.md` (O12, mục 7, mục 8 #7-#8)
- `/opt/repos/orca/docs/crs/v7/agent-codeintel/README.md` (F3, F9, phạm vi ngoài), `/opt/repos/orca/docs/crs/v7/agent-codeintel/CR-CV-001-codeintel-agent-foundation.md`
- `/opt/repos/orca/docs/crs/v7/code-intel-sources/CR-CV-037-structure-analysis.md` (2.2, 2.3), `/opt/repos/orca/docs/crs/v7/code-intel-sources/CR-CV-038-contract-diff-and-static-security.md`, `/opt/repos/orca/docs/crs/v7/code-intel-sources/CR-CV-036-change-overlay.md`
- `/opt/repos/orca/.gitnexus/meta.json` (không có lớp PDG), kết quả `gitnexus --help`, `gitnexus check --help`, `gitnexus group --help`, `gitnexus group sync --help`, `gitnexus group impact --help`, `gitnexus analyze --help` (1.6.9)
- Cùng folder: `./README.md`, `./CR-CV-091-security-and-dependency-scanning.md`

## 9. Kết quả spike (điền khi thực hiện)

| Tool | Đường gọi thử | Thời gian / kích thước | Kết quả | Quyết định | Lý do / điều kiện xem lại |
|---|---|---|---|---|---|
| `check` | | | | | |
| `shape_check` | | | | | |
| `api_impact` | | | | | |
| `route_map` | | | | | |
| `tool_map` | | | | | |
| `explain` | | | | | |
| `pdg_query` | | | | | |
| `group_list` | | | | | |
| `group_sync` | | | | | |

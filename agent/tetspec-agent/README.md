# tdd-agent

Đọc tài liệu yêu cầu của dự án (URD, SRS, SAD) và sinh ra **tài liệu thiết kế kỹ thuật (TDD)** cùng một **knowledge graph (KG)** lưu trong SQLite ngay trong dự án. Từ đó có thể:

- đổi yêu cầu → phân tích tác động → change spec → bộ **OpenSpec**;
- chỉnh TDD bằng một câu lệnh tự nhiên;
- mở KG cho các công cụ AI qua **MCP**.

Mọi thứ nằm trong thư mục `.tdd-agent/` của dự án, dùng giống một công cụ lập chỉ mục mã nguồn: cài một lần, chạy ở đâu cũng được trong dự án.

## Cài đặt

Cần [Claude Code](https://claude.com/claude-code) (`claude`) đã đăng nhập (để đọc văn xuôi bằng mô hình).

### Cách 1 (khuyên dùng): một lệnh với `uv`

Cần [`uv`](https://docs.astral.sh/uv/) và SSH key đã có quyền đọc repo (cùng điều kiện với `git clone`). Python ≥ 3.11 do `uv` tự lo.

```bash
uv tool install git+ssh://git@github.com/blcvn/tdd-agentic-ai.git
```

Kiểm tra: `tdd-agent --help`. Cập nhật: thêm `--force` vào đúng lệnh trên. Gỡ: xem [Gỡ cài đặt](#gỡ-cài-đặt). Cài bản khác: thêm `@<nhánh|tag|commit>` vào cuối URL.

### Cách 2: script `install.sh`

Dùng khi máy chưa có `uv` (script cài `uv` giúp), hoặc không có SSH key mà có token GitHub đọc được repo:

```bash
export GITHUB_TOKEN=<token>
curl -fsSL -H "Authorization: Bearer $GITHUB_TOKEN" \
  https://raw.githubusercontent.com/blcvn/tdd-agentic-ai/main/install.sh | sh
```

Chạy lại đúng lệnh đó để cập nhật (gỡ: xem [Gỡ cài đặt](#gỡ-cài-đặt)). Chọn bản khác: `TDD_AGENT_REF=<nhánh|tag|commit>`.

### Cách 3: từ bản clone (khi phát triển)

```bash
git clone git@github.com:blcvn/tdd-agentic-ai.git
cd tdd-agentic-ai
uv tool install .
```

Cập nhật: `uv tool install --force .`.

### Công cụ đi kèm

Luồng thay đổi yêu cầu (bên dưới) còn cần CLI [`openspec`](https://github.com/Fission-AI/OpenSpec) trong `PATH`.

### Gỡ cài đặt

Cả ba cách cài đều dùng `uv tool`, nên gỡ bằng một lệnh:

```bash
uv tool uninstall tdd-agent
```

Hoặc, nếu đã cài bằng `install.sh`:

```bash
curl -fsSL -H "Authorization: Bearer $GITHUB_TOKEN" \
  https://raw.githubusercontent.com/blcvn/tdd-agentic-ai/main/install.sh | sh -s -- --uninstall
```

Gỡ công cụ **không** xóa dữ liệu dự án. Muốn dọn hết, làm thêm (theo thứ tự này, vì sau khi gỡ thì không còn lệnh `tdd-agent`):

| Việc | Lệnh | Ghi chú |
|---|---|---|
| Tắt đăng ký MCP | `tdd-agent mcp disable` | Chạy **trước** khi gỡ, trong từng dự án đã `mcp enable`. |
| Xóa dữ liệu dự án | `rm -rf .tdd-agent` | Mất cấu hình, KG và lịch sử chạy. `docs/tdd/` (TDD đã sinh) được giữ nguyên. |
| Gỡ `uv` (nếu muốn) | xem [hướng dẫn của uv](https://docs.astral.sh/uv/getting-started/installation/#uninstallation) | Chỉ khi `install.sh` đã cài `uv` giúp và bạn không dùng nó cho việc khác. |

Kiểm tra: `command -v tdd-agent || echo "đã gỡ"`.

## Dùng ngay (hai lệnh)

Trong thư mục dự án của bạn:

```bash
cd P
tdd-agent init --urd docs/URD.md --srs docs/SRS.md --sad docs/SAD.md
```

Lệnh này tạo `.tdd-agent/`, đọc ba tài liệu, xây KG (`.tdd-agent/knowledge.db`) và ghi **`docs/tdd/TDD-P.md`**. Chạy mất khoảng 2 phút với tài liệu cỡ PBOM vì có gọi mô hình. Muốn thử nhanh, không gọi mô hình:

```bash
tdd-agent init --urd docs/URD.md --srs docs/SRS.md --sad docs/SAD.md --no-prose
```

Xem tình trạng bất cứ lúc nào:

```bash
tdd-agent status
```

Sửa tài liệu nguồn xong, làm mới KG và TDD:

```bash
tdd-agent generate
```

(`init` lần nữa cũng được: nó giữ nguyên cấu hình và chỉ chạy lại khi tài liệu đã đổi.)

### TDD sinh ra trông thế nào

Tài liệu Markdown viết cho tech leader: 11 phần của thiết kế (phạm vi, phân rã, hành vi, hợp đồng giao diện, dữ liệu, giao dịch và chịu lỗi, di trú, chất lượng và vận hành, bảo mật, quyết định và chỗ còn thiếu, kiểm chứng và truy vết) cộng phụ lục. Mỗi dòng ghi nguồn (ví dụ `SRS §5.1`). Chỗ dữ liệu chưa có được ghi **THIẾU** kèm lý do, không bao giờ được điền bằng suy đoán. Bản mẫu: [docs/samples/TDD-PBOM.standard.sample.md](docs/samples/TDD-PBOM.standard.sample.md).

## Các lệnh

| Việc cần làm | Lệnh |
|---|---|
| Thiết lập dự án, sinh KG và TDD | `tdd-agent init --urd … --srs … --sad …` |
| Làm mới sau khi sửa tài liệu | `tdd-agent generate` |
| Xem nguồn nào đã đổi, KG có gì | `tdd-agent status` |
| Chỉnh TDD bằng prompt (một phần tử) | `tdd-agent edit "Set the timeout of API-I-01 to 25 seconds"` |
| Chỉnh một mục của TDD bằng lời tự do | `tdd-agent edit "Trong mục 1.3 Phi mục tiêu, thêm: không có app di động native"` (thêm, viết lại, rút gọn, chia ý; chạy được cả với mục đang THIẾU) |
| Hoàn tác một lần chỉnh | `tdd-agent edit --undo edit-1` |
| Mở KG cho công cụ AI (MCP) | `tdd-agent mcp enable` |
| Đổi yêu cầu → OpenSpec | `tdd-agent change docs/cr.md` (xem dưới) |
| Kiểm tra máy đã đủ công cụ chưa | `tdd-agent doctor` |
| Tìm trong KG (không dấu cũng được, mã một phần cũng được) | `tdd-agent kg search "canh bao"` |
| Dọn bản làm việc không còn dùng | `tdd-agent clean` (chỉ liệt kê; `--yes` mới xoá) |
| Chi phí của một lần chạy | `tdd-agent metrics --run change-<tên>` |
| Đổi yêu cầu → OpenSpec | `tdd-agent change start …` (xem dưới) |
| Sinh test specs cho một TDD (chạy tay) | `tdd-agent run start testspec --request req.json` với `{"params": {"tdd": "TDD-P"}}` |

Trong terminal, lệnh in văn bản dễ đọc; khi đầu ra không phải terminal (script, pipe) hoặc khi thêm `--json`, lệnh in JSON ra stdout và tóm tắt ra stderr (`--human` ép in văn bản). Lệnh tự tìm dự án bằng cách đi lên các thư mục cha đến khi gặp `.tdd-agent/config.toml`; dùng `-C <thư mục>` để chỉ dự án khác.

## Đổi yêu cầu → bộ OpenSpec

Đầu vào là **change request (CR)** viết bằng Markdown, cộng với SRS, SAD và các TDD của dự án. Không cần sửa SRS trước.

### CR đầu tiên của một dự án

1. `tdd-agent init …` xong (có SRS và SAD), rồi `tdd-agent doctor` báo không có dòng `FAIL`.
2. Viết CR: tiêu đề, nội dung thay đổi, và tiêu chí nghiệm thu dạng `- AC-<mã>: GIVEN … WHEN … THEN …` (tiêu chí không bắt buộc ngay, nhưng thiếu thì cổng G4 dừng và nhờ bổ sung).
3. Chạy một lệnh:

```bash
tdd-agent change docs/cr-t14.md          # tên change tự đặt từ tiêu đề; thêm --owner "nhóm A" để khỏi bị hỏi
```

Lệnh kiểm tra công cụ, đặt tên, bắt đầu và chạy tới chỗ cần dừng (người duyệt, blocker hoặc chờ kết quả test). Chạy lại cùng lệnh thì **tiếp tục** change đó. Các bước bên dưới chỉ để can thiệp:
Luồng: mô hình đọc CR đối chiếu với SRS và đề xuất sửa (mỗi sửa kèm câu trích của CR, luật kiểm và vá SRS) → BA xác nhận SRS delta (G1) → đánh giá tác động lên các TDD (G2) → sinh TDD mới (G3) → bảng change spec (G4) → viết OpenSpec, SA duyệt rồi publish (G5) → test và nghiệm thu (G6, G7). Ngay sau G4, cổng GT sinh **test specs** (ba tài liệu ISO/IEC/IEEE 29119-3 trong `docs/testspec/`) bằng tsagent ở một run con chạy nền, song song với G5; cần đặt `[testspec] llm` trong `.tdd-agent/config.toml` (`claude-cli`, `anthropic`, `openai` hoặc `none`, không có mặc định). Các cổng cần người duyệt: G1 BA, G2 SA/Dev, G5 SA, G7 BA/owner; khai báo ngay lúc `init`:

```bash
tdd-agent change status --why          # đang ở đâu; nếu bị chặn: nguyên nhân gốc và lệnh sửa
tdd-agent change update --code-target REQ=file::symbol --owner "nhóm A" --acceptance AC-X   # bổ sung điều cổng còn thiếu
tdd-agent change waive --sad-element ADR-1 --requirement FR-3 --actor bob --reason "…"      # SA bỏ qua một phát hiện sai của bước kiểm tra SAD
tdd-agent baseline                      # viết baseline OpenSpec của các yêu cầu bị sửa (luồng cũng tự làm)
tdd-agent change advance --retry        # chạy lại change đã "failed" vì lỗi chương trình (Ctrl+C không làm hỏng change: chạy lại là tiếp tục)
```

Luồng: mô hình đọc CR đối chiếu với SRS và đề xuất sửa (mỗi sửa kèm câu trích của CR, luật kiểm và vá SRS) → xác nhận SRS delta (G1) → đánh giá tác động lên các TDD (G2) → sinh TDD mới (G3) → bảng change spec (G4) → viết OpenSpec và publish (G5) → test và nghiệm thu (G6, G7). Mỗi change có `.tdd-agent/changes/<tên>/review.md` tóm tắt việc nó làm cho người duyệt.

### Chế độ duyệt

Mặc định **tdd-agent tự duyệt** các cổng G1, G2, G5, G7 sau khi các luật đã đạt, và ghi người duyệt là `tdd-agent:auto` (mức `auto`) vào lịch sử, vào `change status` và vào `trace-manifest.json`. Hai thứ không bao giờ bị bỏ qua ở mọi chế độ: **luật** (blocker vẫn chặn) và **G6** (cần biên nhận test thật, không ai giả được). SAD vẫn chỉ đọc.

Rủi ro: không ai đọc quyết định trước khi publish. Muốn người thật quyết, khai báo người duyệt (chế độ chuyển sang `manual`) hoặc chỉ định cổng nhạy cảm:

```toml
# .tdd-agent/config.toml
[approvals]
mode = "auto"            # auto | manual (để trống: manual nếu có người duyệt khai báo, ngược lại auto)
required = ["G5", "G7"]  # ở chế độ auto, các cổng vẫn đợi người (cần khai báo người duyệt)

[[approvals.actors]]
id = "alice"
kind = "human"
roles = ["BA", "BA/owner"]
```

Hoặc lúc khởi tạo: `tdd-agent init --approver alice:human:BA,BA/owner --approver bob:human:SA/Dev,SA`. `tdd-agent config` cho biết chế độ hiệu lực và lý do. Ở chế độ `manual`, G1 BA, G2 SA/Dev, G5 SA, G7 BA/owner duyệt bằng `change decision` rồi `change approve`.

### Chế độ tự động và phần CR chưa nói

Ở chế độ tự động, nếu CR thiếu tiêu chí nghiệm thu, phần tử thiết kế đáp ứng yêu cầu bị sửa, hoặc chỗ trong code, `tdd-agent` để mô hình **đề xuất** (mỗi đề xuất trích câu của CR hoặc nêu lý do, và qua luật kiểm số, mã, trích dẫn) để change đi tiếp tới OpenSpec. Mọi đề xuất được ghi riêng ở mục "Proposed by tdd-agent, not given by a person" của `review.md`; ghi đè bằng `change update`. Ở chế độ `manual` không đề xuất gì: cổng hỏi như cũ. Tắt hẳn: `[change] propose_missing = false`.

### Chọn agent

Mặc định tdd-agent dùng Claude Code. Dùng Codex: `tdd-agent init --agent codex` (hoặc `[llm] default = "codex_cli"`); có thể trộn theo vai (`[llm.roles] critic = "codex_cli"`). `tdd-agent doctor` kiểm tra lệnh đã cài; Codex chạy với `--ignore-user-config --ignore-rules` nên không phụ thuộc cấu hình riêng của bạn (`[llm.codex_cli] isolate = false` để tắt). Mỗi lời gọi Codex tốn nền khoảng 13 nghìn token.

### Giới hạn đã biết

- Yêu cầu sửa (`modified`) và xoá cần baseline OpenSpec; tdd-agent tự viết từ câu của SRS (câu không có SHALL/MUST, ví dụ tiếng Việt, được giữ nguyên sau một câu dẫn cố định). Không có câu nào để ghi thì `BASELINE_UNWRITABLE`.
- Mô hình đọc CR có thể không ánh xạ hết các ý; phần không ánh xạ được liệt kê, không tự bịa.
- Kết quả viết gói OpenSpec do mô hình, có tối đa `[openspec] max_revisions` (mặc định 2) vòng sửa theo phản biện; hết vòng mà vẫn trượt thì dừng ở G5 kèm lý do.
- Thêm `--allow-draft` ở `change start` để xem bản NHÁP khi spec chưa đủ điều kiện (không bao giờ được duyệt hay publish). Chi tiết: [docs/CLI_Project_Usage.md](docs/CLI_Project_Usage.md).

## Mở KG qua MCP

```bash
tdd-agent mcp enable                 # ghi .mcp.json; mặc định có tool đọc và ghi
tdd-agent mcp enable --read-only     # chỉ tool đọc
```

Claude Code đọc `.mcp.json` ở thư mục dự án. Client khác chạy `tdd-agent -C <dự án> mcp serve`. Tắt: `tdd-agent mcp disable`.

## Trong thư mục `.tdd-agent/`

| Tệp | Ghi chú |
|---|---|
| `config.toml` | Cấu hình dự án (đường dẫn tài liệu, mã TDD, người duyệt). Nên commit. |
| `knowledge.db`, `operational.db` | KG và lịch sử chạy (SQLite). Không commit; sinh lại bằng `generate`. |
| `tsagent.db`, `tsagent/` | KG test (test specs có phiên bản) và cache LLM của tsagent. Không commit. |
| `.gitignore`, `last-generate.json`, `logs/`, `runs/`, `staging/` | Tự tạo; trạng thái cục bộ. |

## Phát triển

```bash
uv venv && source .venv/bin/activate
uv pip install -e ".[dev]"
pytest
```

Mã nguồn ở `src/tdd_agent/` (`project.py` là các lệnh theo dự án, `render/` là bộ sinh TDD, `agents/` là các agent, `orchestrator/` là luồng cổng G0–G7). Thiết kế: [docs/TDD_Agent_And_KG_Design_v0.6.md](docs/TDD_Agent_And_KG_Design_v0.6.md). Các thay đổi đặc tả nằm trong `openspec/`.

## Agent sinh test specs (tsagent)

Repo kèm **tsagent v2**: từ KG của TDD (gói `tdd-kg/v1`) và md (solution, SRS) sinh ba tài liệu test specs theo ISO/IEC/IEEE 29119-3 (Test Requirements, Test Design, Test Cases), lưu KG test (ontology `tso:`) trong SQLite, có phiên bản theo nút, duyệt, baseline, MCP gateway và CLI `tsagent`.

| Phần | Vị trí |
| --- | --- |
| Mã | `src/tsagent/` |
| Test | `tests/testspec_agent/` (`.venv/bin/pytest tests/testspec_agent`) |
| Script đo, so sánh, dựng fixture | `scripts/tsagent/` |
| Container | `deploy/tsagent/` |
| Hướng dẫn, thiết kế, triển khai | [docs/tsagent/README.md](docs/tsagent/README.md), [docs/tsagent/TDD-TSAGENT-v2.md](docs/tsagent/TDD-TSAGENT-v2.md), [docs/tsagent/implementation/](docs/tsagent/implementation/README.md) |

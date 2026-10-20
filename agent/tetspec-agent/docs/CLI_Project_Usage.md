# Dùng `tdd-agent` trong một dự án

`tdd-agent` dùng như một công cụ lập chỉ mục mã nguồn: cài một lần, rồi chạy trong thư mục dự án. Trạng thái nằm trong `.tdd-agent/` của dự án, knowledge graph (KG) là một tệp SQLite cục bộ.

## Cài đặt

```bash
uv tool install /đường/dẫn/tới/tdd-agentic-ai     # từ thư mục mã nguồn của công cụ
```

Cần `claude` (đã đăng nhập) trong `PATH` để đọc văn xuôi bằng mô hình. Chạy với `--no-prose` thì không gọi mô hình.

## Quy trình

```bash
cd P                                              # thư mục dự án
tdd-agent init --urd docs/URD.md --srs docs/SRS.md --sad docs/SAD.md
tdd-agent status                                  # nguồn nào đã đổi, KG có gì, TDD gần nhất
```

`init` làm tất cả trong một lệnh: tạo `.tdd-agent/`, đọc URD/SRS/SAD, xây KG (SQLite cục bộ) và ghi `docs/tdd/TDD-<TÊN>.md`. Không cần chạy `generate` cho lần đầu.

| Lệnh | Việc làm |
|---|---|
| `tdd-agent init` | Tạo `.tdd-agent/config.toml`, `.tdd-agent/.gitignore`, `docs/tdd/`, rồi sinh KG và TDD từ các tài liệu nguồn. Chạy lại trên dự án đã có: giữ nguyên cấu hình và chỉ làm mới khi tài liệu đã đổi. |
| `tdd-agent generate` | Làm mới KG và TDD sau khi sửa tài liệu (như `init` chạy lại, có thêm các cờ bên dưới). Nguồn không đổi thì không chạy lại (`--force` để ép). |
| `tdd-agent status` | Dự án, các nguồn (đã đổi chưa), KG (phần tử, quan hệ, chỗ còn thiếu), TDD gần nhất, số phần tử chờ rà soát. |

Tuỳ chọn của `init`: `--urd/--srs/--sad` (lưu vào cấu hình), `--name`, `--tdd-id`, `--docs-dir`, `--approver` (xem bên dưới), `--no-prose` (không gọi mô hình, được lưu vào cấu hình), `--prose-limit N`, `--no-generate` (chỉ tạo khung, chưa sinh KG và TDD), `--force` (ghi đè cấu hình). Không truyền tài liệu nào thì `init` chỉ tạo khung và nhắc việc cần làm.

Tuỳ chọn của `generate`: `--urd/--srs/--sad` (đè lên cấu hình), `--id` (mã TDD), `--out` (thư mục xuất), `--no-prose`, `--prose-limit N`, `--force`.

Lệnh nào cũng tự tìm dự án bằng cách đi lên các thư mục cha đến khi gặp `.tdd-agent/config.toml` (như git tìm `.git`). Dùng `-C <thư mục>` để chỉ định dự án khác.

## Chế độ duyệt: tự động hay có người

Mặc định **tdd-agent tự duyệt** các cổng của luồng thay đổi (G1 SRS delta, G2 tác động, G5 gói OpenSpec, G7 nghiệm thu) khi các luật đã đạt. Mỗi lần duyệt được ghi với người duyệt `tdd-agent:auto` và mức `auto`, gắn với `decision_hash` của quyết định mà nó phủ; mức này hiện ở `change status`, `run show`, `review.md` và `trace-manifest.json` của gói OpenSpec. Không bao giờ bị bỏ qua, ở cả hai chế độ: **luật** (blocker vẫn chặn), **SAD chỉ đọc**, **G6** (cần biên nhận test thật; ở dự án không khai báo người duyệt nào, `receipt --actor` nhận tên người đã chạy test và ghi rõ là tự khai). Agent của pipeline không bao giờ là người duyệt.

Rủi ro cần biết: ở chế độ `auto` không ai đọc quyết định trước khi gói OpenSpec được publish và TDD được ghi. Rà soát các phần tử `needs_review` trước G5 cũng do tdd-agent làm (kiểm `QG-03` không còn chặn). Nếu cần người thật, dùng một trong hai cách:

```toml
# .tdd-agent/config.toml
[approvals]
mode = "auto"            # auto | manual; để trống: manual nếu có [[approvals.actors]], ngược lại auto
required = ["G5", "G7"]  # ở chế độ auto, các cổng vẫn đợi người (cần khai báo người duyệt); G1 G2 G5 G7

[[approvals.actors]]
id = "alice"
kind = "human"            # human | coding_agent
roles = ["BA", "BA/owner"]
```

```bash
tdd-agent init --approver alice:human:BA,BA/owner --approver bob:human:SA/Dev,SA --approver claude:coding_agent
```

`--approver` chỉ có tác dụng khi `init` tạo cấu hình mới; dự án đã có thì thêm khối `[[approvals.actors]]` vào `config.toml`. Chế độ `manual`: G1 BA, G2 SA/Dev, G5 SA, G7 BA/owner đọc quyết định (`change decision`) rồi duyệt (`change approve`); duyệt bởi `coding_agent` được ghi ở mức "agent". Đổi chế độ hoặc danh sách người duyệt **không** làm các cổng đã chạy trở nên cũ. `tdd-agent config` cho biết chế độ hiệu lực và lý do, và agent đang làm việc cho từng vai.

## Luồng thay đổi yêu cầu (CR → impact → change spec → OpenSpec)

**Đầu vào:** change request (CR) viết bằng Markdown hoặc gõ thẳng, cộng với SRS trước, SAD và các TDD của dự án (những gì `init` đã sinh). Bạn **không** phải sửa SRS trước.

Ví dụ file CR `docs/cr-t14.md`:

```markdown
# CR-PBOM-1: Thêm mốc nhắc T-14 cho Entitlement thuê bao

Chủ tài sản cần được nhắc thêm một lần vào 14 ngày trước khi Entitlement thuê bao hết hạn (mốc T-14).

- AC-REM-14: GIVEN một Entitlement thuê bao hết hạn sau 14 ngày WHEN job hằng ngày chạy THEN gửi đúng một nhắc nhở T-14
```

```bash
tdd-agent doctor                                                      # đủ công cụ chưa (agent, OpenSpec, tài liệu, baseline)
tdd-agent change docs/cr-t14.md                                       # một lệnh: kiểm tra, đặt tên, bắt đầu, chạy tới chỗ phải dừng; chạy lại để tiếp tục
tdd-agent change status --why                                         # đang ở cổng nào; bị chặn thì nguyên nhân gốc và lệnh sửa
tdd-agent change update --code-target "FR-AST-13=internal/x.go::Milestones" --owner "nhóm A" --acceptance AC-REM-14 --satisfied-by FR-AST-13=SVC-02
tdd-agent change waive --sad-element ADR-1 --requirement FR-AST-13 --actor bob --reason "…"   # SA bỏ qua một phát hiện sai của kiểm tra SAD
tdd-agent baseline                                                    # baseline OpenSpec của yêu cầu bị sửa (luồng cũng tự làm)
tdd-agent kg conflicts / kg resolve <khoá> --actor … --role SA --decision "…" --rationale "…"   # mâu thuẫn trong KG
tdd-agent change advance --retry                                      # chạy lại change "failed" vì lỗi chương trình (Ctrl+C chỉ dừng, chạy lại là tiếp tục)
tdd-agent clean                                                       # liệt kê bản làm việc không còn dùng; --yes mới xoá
tdd-agent metrics --run change-<tên>                                  # số lời gọi, token, thời gian của change
```

Các lệnh cấp thấp `change start/status/advance/decision/approve/update` vẫn dùng được (`change start <tên> --cr <file> …` đặt tên tay). `change <file>` cho phép `--name --owner --purpose --scope --acceptance --code-target --verify --satisfied-by --yes`.

Các cổng:

| Cổng | Việc làm | Ai quyết định |
|---|---|---|
| G0 Tiếp nhận | Ghim CR, SRS trước, SAD. Mô hình đọc CR đối chiếu với các dòng yêu cầu của SRS và **đề xuất sửa** (đổi một đoạn câu, thêm dòng, bỏ dòng), mỗi sửa kèm một câu trích nguyên văn của CR. Luật kiểm: id có thật, đoạn cần thay xuất hiện đúng một lần, số và id mới phải có trong CR, câu trích phải nằm trong CR. SRS được vá bằng luật thành revision kế tiếp (bản đề xuất), rồi đọc lại để kiểm. | Tự động |
| G1 SRS delta | So SRS trước với bản đề xuất ra danh sách thêm, sửa, bỏ. Màn duyệt hiện từng sửa cùng câu trích của CR, diff và các dòng còn dùng thuật ngữ cũ khi CR đổi tên. | BA (hoặc tự động) |
| G2 Tác động | Quét **toàn bộ** TDD của dự án: `update`, `add` hoặc `no_change` (kèm lý do); kiểm xung đột với SAD (phát hiện sai thì SA `change waive`). | SA/Dev (hoặc tự động) |
| G3 Hiện thực TDD | Sinh revision TDD mới và ghi vào `docs/tdd/`. | Tự động |
| G4 Bảng change spec | Mỗi dòng gồm SRS, TDD thực tế, tiêu chí của CR, vị trí code, cách kiểm chứng. Cách kiểm chứng đọc từ tiêu chí `GIVEN … WHEN … THEN …` của CR. | Tự động |
| G5 Viết OpenSpec | Viết, phản biện, kiểm; nếu phản biện trượt, viết lại kèm phát hiện (tối đa `[openspec] max_revisions` vòng, ngôn ngữ `[openspec] language`); rồi duyệt và publish vào `openspec/changes/<tên>`. Yêu cầu bị sửa cần baseline OpenSpec: tự viết từ SRS gốc. | SA (hoặc tự động) |
| G6, G7 | Kết quả test thật (G6, không bao giờ tự động) rồi nghiệm thu. | Dev/QA, BA/owner (hoặc tự động ở G7) |
| GT Test specs | Khởi chạy run con `testspec` sinh ba tài liệu test specs từ change spec rồi trả về ngay; G5 và việc sinh code không chờ (xem [Test specs](#test-specs-tsagent)). | Tự động |
| G5 Viết OpenSpec | Viết, phản biện, kiểm; SA duyệt rồi mới publish vào `openspec/changes/<tên>`. | SA |
| G6, G7 | Kết quả test thật rồi nghiệm thu. | Dev/QA, BA/owner |

- Bản SRS đề xuất và diff nằm ở `.tdd-agent/changes/<tên>/SRS.proposed.md` và `srs.diff` để BA xem trước khi duyệt G1. Luồng **không ghi đè** `docs/SRS.md`; khi change được chấp nhận, bạn đưa bản đề xuất vào SRS của dự án.
- CR có câu nào không gán được cho yêu cầu nào thì được liệt kê riêng ("NOT mapped"), không bị ép vào một sửa.
- Nếu không có sửa nào được đề xuất, hoặc sửa dùng số/id không có trong CR, G0 dừng và không ghi gì (`CR_NO_REQUIREMENT_CHANGE`, `EDIT_NUMBER_NOT_IN_CR`, `EDIT_ID_NOT_IN_CR`).
- Cổng nào báo thiếu (ví dụ vị trí code, vì dự án chưa có code) thì dùng `change update --code-target REQ=đường-dẫn::symbol`, `--verify REQ="GIVEN …"` hoặc `--new-in REQ=TDD-ID`. Các cổng G0–G3 đã duyệt không phải duyệt lại.
- Yêu cầu mới (thêm dòng) chưa có thiết kế nên spec ở bản NHÁP cho tới khi TDD có thiết kế tương ứng; yêu cầu được SỬA cần baseline `openspec/specs/<tên change>/spec.md`: luồng tự viết từ câu của SRS gốc (câu không có SHALL/MUST được giữ nguyên sau một câu dẫn cố định); `tdd-agent baseline` để xem hoặc `--force` để viết lại. Một yêu cầu được sửa còn cần một phần tử thiết kế đang đáp ứng nó (thành phần, thao tác, thực thể): SRS thường có dòng nói điều đó; nếu không, `change update --satisfied-by REQ=SVC-…`. `--allow-draft` ghi bản NHÁP để xem (`.tdd-agent/staging/<run>/drafts/…`), không bao giờ được duyệt hay publish.
- Đã tự sửa SRS bằng tay thì dùng `--srs-after <file>`: delta đọc thẳng từ file đó, không qua bước mô hình.
- G6 cần kết quả test thật: `tdd-agent receipt <run> <CS-id> --criteria AC-1 --result pass --actor <ai>`, rồi G7 duyệt như trên. `change list`, `change advance` để xem và chạy tiếp.

## Chỉnh TDD bằng prompt

**Một mục của tài liệu, lời tự do.** `tdd-agent edit "Trong mục 2.4 Phân bổ triển khai, ghi: dev dùng Docker Compose; staging và production dùng Kubernetes"` — gọi tên mục bằng số (`mục 1.3`, `section 2.4`) và nói điều cần làm: thêm, viết lại, rút gọn, chia thành gạch đầu dòng. Hoạt động cả với mục đang THIẾU. Mô hình viết lại chữ từ lời yêu cầu và nội dung hiện có của mục; luật từ chối nếu nó thêm số hoặc mã mà yêu cầu không nêu. Chữ được lưu trong KG (phần tử `NOTE-<tdd>-<mục>`), đánh dấu trong tài liệu là do người dùng viết, còn sau khi sinh lại, và hoàn tác được bằng `edit --undo <run>`. Động từ của bạn quyết định thêm hay thay ("thêm", "bổ sung" → thêm; "viết lại", "sửa", "rút gọn" → thay). Prompt nêu mã phần tử (ví dụ `API-I-01`) thì đi đường sửa phần tử bên dưới.

**Một phần tử.**

```bash
tdd-agent edit "Set the timeout of API-I-01 to 25 seconds"
tdd-agent edit --undo edit-1
```

- Mặc định chỉ được `modify` trên mọi phần tử. Mở rộng bằng `--allow add --allow remove`, giới hạn bằng `--scope <id>`.
- Prompt **không** ghi đè được giá trị đã đọc từ tài liệu nguồn (kể cả giá trị được trích nguyên văn trong bằng chứng): lệnh dừng và yêu cầu làm rõ qua SRS/CR. Prompt mơ hồ thì lệnh hỏi lại, không đoán.
- Edit thành công ghi lại `docs/tdd/<mã>.md`; phần tử bị sửa quay về trạng thái chờ rà soát. `--undo` hoàn tác và render lại.

## MCP server

```bash
tdd-agent mcp enable                 # mặc định đọc và ghi: KG, bằng chứng, nguồn, run, trace + run_start, run_resume, changeset_propose
tdd-agent mcp enable --read-only     # chỉ các tool đọc
tdd-agent mcp enable --approvals --client-actor claude   # thêm approval_submit (claude phải là approver)
tdd-agent mcp disable
```

`enable` ghi `.mcp.json` ở thư mục dự án (Claude Code đọc tệp này) và mục `[mcp]` trong cấu hình. Client khác chạy `tdd-agent -C <dự án> mcp serve` (stdio) hoặc `--transport http` (cần token trong biến môi trường `TDD_AGENT_MCP_TOKEN`).

### Client từ xa (workflow Orca)

Workflow Orca gọi các tool qua dev server, không giữ lời gọi mở trong lúc gate chạy:

| Tool | Dùng để |
|---|---|
| `run_start(flow, request, sources, external)` | Nguồn có thể là `{id, revision, role, path}`, đường dẫn tương đối trong dự án. `external = {system, executionId, stepId, userId}` ghi việc của client để tìm lại run. |
| `run_dispatch(run_id)` | Chạy tiếp run ở tiến trình nền (`run advance --heartbeat`) rồi trả ngay; không làm gì nếu run đã xong hoặc tiến trình nền còn sống. |
| `run_get(run_id)` | Thêm `children` (run con) và `process` (`pid`, `alive`, `heartbeat_at`, `finished_at`). |
| `run_list(..., parent_run_id, external_execution_id, external_step_id)` | Tìm run con hoặc run của một execution. |
| `approval_submit(..., advance=false)` | Ghi duyệt rồi trả ngay; chạy tiếp bằng `run_dispatch`. |
| `run_cancel(run_id, reason)` | Dừng tiến trình nền, đánh `failed` (`CANCELLED_BY_CLIENT`) run và các run con chưa xong. |

Người duyệt của Orca là một approver chuyển tiếp quyết định của người dùng Orca, đăng ký như sau:

```bash
tdd-agent init --approver orca-workflow:human:BA,SA/Dev,SA,BA/owner ...   # hoặc thêm [[approvals.actors]] vào .tdd-agent/config.toml
tdd-agent mcp enable --approvals --client-actor orca-workflow
```

`rationale` của mỗi lần duyệt từ Orca chứa `orca:user=<id>` để biết người duyệt thật.

## Chạy lại khi tài liệu đổi

Sửa SRS rồi `tdd-agent generate` (hoặc `tdd-agent init` lần nữa): tài liệu nguồn được ghi nhận với revision mới (1, 2, 3…), KG được cập nhật (quan hệ cũ của revision trước được thay, không nhân đôi) và `docs/tdd/<mã>.md` được ghi đè. Chỉ tài liệu **có nội dung mới** mới được đọc lại (tức là mới gọi mô hình): sửa SRS thì SAD không bị đọc lại, nên không còn `SAD_READONLY` do mô hình diễn đạt lại; `generate --force` đọc lại tất cả. Nếu SAD thật sự đổi mà phần tử SAD bị ảnh hưởng, mã chặn nêu rõ `SAD_CHANGED:<phần tử>`.

## Test specs (tsagent)

Khi G4 xong, cổng GT khởi chạy một **run con** (workflow `testspec`, cổng TS1) trong tiến trình nền rồi trả về; run CR đi tiếp G5 ngay. Run con:

1. đọc snapshot KG mà G4 vừa ghi (thiết kế sau G3 + các dòng change spec) **qua MCP server của tdd-agent** (tool đọc `kg_elements`, `kg_neighbors`), chỉ phần đồ thị cần cho phạm vi; mặc định tự chạy server của dự án qua stdio;
2. lấy phạm vi: phần tử mà `srs_refs`/`tdd_refs` của change spec trỏ tới (đối tượng kiểm) + láng giềng tới 2 bước (ngữ cảnh); tiêu chí của CR thành tiêu chí chấp nhận;
3. chiếu sang gói `tdd-kg/v1`, chạy tsagent (LLM song song, nhóm lỗi được thử lại), lưu KG test vào `.tdd-agent/tsagent.db`;
4. ghi `docs/testspec/<TDD>/01-test-requirements.md`, `02-test-design.md`, `03-test-cases.md` (Draft, **chỉ để đọc**: chưa có bước duyệt hay nối với G6).

**Bắt buộc chọn chế độ LLM** trong `.tdd-agent/config.toml` (không có mặc định):

```toml
[testspec]
enabled = true
llm = "claude-cli"     # claude-cli | anthropic | openai | none
parallel = 4
group_retries = 2
specs_dir = "docs/testspec"
kg_mcp = ""            # rỗng: `tdd-agent -C <dự án> mcp serve` qua stdio; hoặc URL MCP HTTP của tdd-agent ở xa
kg_mcp_token_env = "TDD_AGENT_MCP_TOKEN"   # bearer token cho kg_mcp HTTP
```

- Chưa đặt `llm`: run con dừng ngay với `LLM_NOT_CONFIGURED`; run CR không bị ảnh hưởng. Đặt xong thì `tdd-agent run advance <run con>`.
- `anthropic`/`openai` thiếu `ANTHROPIC_API_KEY`/`OPENAI_API_KEY` hoặc `ANTHROPIC_MODEL`/`OPENAI_MODEL`: dừng ngay (`LLM_KEY_MISSING`, `LLM_MODEL_MISSING`).
- `none`: chỉ chạy luật, đầu tài liệu ghi `llm: none` và một dòng cảnh báo.
- Nhóm gọi LLM lỗi được thử lại tối đa `group_retries` lần; sau đó hơn 50 % số nhóm của một bước vẫn lỗi thì run con dừng (`TESTSPEC_FAILED:LLM_GROUPS_FAILED`), ít hơn thì xong kèm cảnh báo.
- Đổi `parallel`, `group_retries` có hiệu lực từ run con kế tiếp, không làm cổng nào của CR phải chạy lại; đổi `enabled`, `llm`, `specs_dir` thì GT chạy lại ở lần `change advance` sau.
- `enabled = false`: không sinh test specs, luồng CR như cũ.
- Không đọc được KG qua MCP (server không chạy, sai URL): run con dừng với `TESTSPEC_KG_MCP`; thiếu token cho `kg_mcp` HTTP: `TESTSPEC_KG_MCP_TOKEN`.

`tdd-agent change status` có thêm dòng `Test specs: …` (đang sinh / xong + đường dẫn / lỗi + cách sửa). Chi tiết run con: `tdd-agent run show <run con>`. Log của tiến trình nền: `.tdd-agent/logs/testspec-<run con>.log`. Tiến trình bị dừng giữa chừng thì `tdd-agent run advance <run con>` chạy tiếp từ bước dở.

**Chạy tay** (chưa có change spec, hoặc muốn sinh lại cho một TDD):

```bash
echo '{"params": {"tdd": "TDD-P"}}' > req.json        # thêm "snapshot": "snap-…" để chọn snapshot, mặc định mới nhất
tdd-agent run start testspec --request req.json
tdd-agent run show <run>
```

Mỗi lần chạy giữ gói đầu vào, tóm tắt và bản chụp ba tài liệu ở `.tdd-agent/runs/<run>/testspec/`. Từ một phiên bản test specs truy ngược được snapshot KG thiết kế, run CR và change spec đã sinh ra nó (`run.params.origin` trong `tsagent.db`).

### Ngữ cảnh bổ sung từ Plan hoặc Solution

> Đã hiện thực (2026-10-19). Thiết kế ở [SOLUTION mục 14](tsagent/integrate/SOLUTION-INTEGRATE-TDD-AGENT.md#14-ngữ-cảnh-plan-hoặc-solution-đề-xuất-04) và [TDD 0.7](tsagent/integrate/TDD-INTEGRATE-TDD-AGENT.md). Mặc định `none` giữ nguyên hành vi trước đó.

Test specs mặc định chỉ biết KG thiết kế (SRS, SAD, TDD) và tiêu chí CR. Khi thay đổi đến từ một Request của Orca, có thể đưa thêm **tài liệu của Request đó** vào, chọn **một** trong hai chế độ trong `[testspec]`:

```toml
[testspec]
context_mode = "none"        # none | plan | solution
context_path = ""            # tệp hoặc thư mục trong repo; mỗi run ghi đè được bằng params.context_path
```

| `context_mode` | Đọc | Dùng để | Tài liệu |
| --- | --- | --- | --- |
| `none` (mặc định) | Không gì thêm | Như hiện tại | — |
| `solution` | Tài liệu Solution (JSON như Orca lưu), **đã duyệt hay chưa đều dùng được** | Biết **vì sao và với giả định nào**: ràng buộc đã chốt, giả định chưa xác nhận (thành TestGap có chủ), rủi ro, tiêu chí chấp nhận. Chỉ dùng **phương án đã chọn** | `context_path = ".tdd-agent/context/solution-REQ-2.json"` |
| `plan` | Plan của Request: OpenSpec change gồm `tasks.md`, `specs/**/spec.md`, **`proposal.md`** và **`design.md`** | Biết **kiểm bằng cách nào và ở đâu**: mỗi câu `Done when` của task và mỗi scenario thành một tiêu chí chấp nhận (kèm `Code:` và `Verify:`); `proposal.md` cho thay đổi và lý do; `design.md` cho quyết định, rủi ro/đánh đổi và ngoài phạm vi | `context_path = ".tdd-agent/context/plan-REQ-2"` |

Chạy tay, chọn chế độ cho **một lần chạy**:

```bash
echo '{"params": {"tdd": "TDD-NOTES", "context_path": ".tdd-agent/context/solution-REQ-2.json", "context_ref": "REQ-2"}}' > req.json
tdd-agent --set testspec.llm=claude-cli --set testspec.context_mode=solution run start testspec --request req.json
```

- Tài liệu phải nằm **trong repo** của dự án. Đường dẫn ra ngoài (kể cả qua symlink) bị từ chối. OpenSpec change của Orca nằm trong worktree proposal cạnh repo, nên chép vào `.tdd-agent/context/plan-<ref>/` trước.
- **Mỗi run một chế độ**, do cấu hình quyết định; `params.context_mode` bị bỏ qua. Cần cả hai thì chạy hai lần, mỗi lần một chế độ.
- Không tìm thấy hoặc sai định dạng tài liệu: run dừng trước khi gọi LLM với `TESTSPEC_CONTEXT_MISSING`, `TESTSPEC_CONTEXT_PATH` hay `TESTSPEC_CONTEXT_INVALID` (không âm thầm chạy như `none`).
- Giả định **cần xác nhận** của Solution vẫn là TestGap, không thành yêu cầu. Mọi nút ngữ cảnh ghi `derivation` = `solution` hoặc `plan`; test specs vẫn là Draft.
- Plan thiếu `proposal.md` hoặc `design.md` vẫn chạy (cảnh báo trong `context.notes`); `tasks.md` là bắt buộc.
- Sửa tài liệu ngữ cảnh làm GT và TS1 chạy lại như khi change spec đổi. Tóm tắt run (`tdd-agent run show <run>`) có khối `context`: số nút theo loại, yêu cầu được gắn, mục **không gắn được** (`unanchored`) và cảnh báo (`notes`).

#### Tích hợp với Orca qua MCP

Orca không ghi tệp vào repo của dự án: nó gửi tài liệu **trong lời gọi MCP**, qua cầu `mcp.callTool` của Dev Server Agent. Hai tool mới trên `tdd-agent mcp serve`:

| Tool | Loại | Việc |
| --- | --- | --- |
| `testspec_start(tdd?, snapshot?, context, external?)` | ghi (cần `[mcp] write = true`) | Ghi tài liệu vào `.tdd-agent/context/<loại>-<ref>/` rồi tạo run con `testspec`; trả `run_id`. Không chạy: gọi `run_dispatch` |
| `testspec_get(feature?, document?, offset?)` | đọc | Cấu hình hiệu lực (có `context_mode`), danh sách test specs, TestGap, nội dung `requirements`, `design`, `cases` (phân trang) |

```jsonc
// 1. Orca hỏi chế độ của dự án
testspec_get()        // → { "config": { "context_mode": "solution", … }, "features": [ … ] }

// 2. Gửi đúng loại tài liệu của chế độ đó (kind PHẢI trùng context_mode)
testspec_start({ "context": { "kind": "solution", "ref": "REQ-2", "document": { …Solution JSON… } },
                 "external": { "system": "orca", "executionId": "…", "stepId": "…" } })   // → { "run_id": "run-…", … }
// Plan: "context": { "kind": "plan", "ref": "REQ-2", "files": { "tasks.md": "…", "proposal.md": "…", "design.md": "…", "specs/healthz/spec.md": "…" } }

// 3. Chạy và theo dõi bằng các tool có sẵn
run_dispatch({ "run_id": "run-…" });  run_get({ "run_id": "run-…" })     // tới completed hoặc blocked

// 4. Đọc kết quả
testspec_get({ "feature": "TDD-NOTES" })                               // số liệu, TestGap, khối context
testspec_get({ "feature": "TDD-NOTES", "document": "cases" })           // 03-test-cases.md
```

- `kind` khác `context_mode` → `TESTSPEC_CONTEXT_MODE_MISMATCH`, không tạo run. Chưa đặt `[testspec] llm` → `LLM_NOT_CONFIGURED` ngay, không tạo run.
- Gọi lại cùng `external` và cùng nội dung trả run cũ (`reused: true`); nội dung đổi tạo run mới.
- Tên tệp chỉ gồm `solution.json`, `tasks.md`, `proposal.md`, `design.md`, `specs/<tên>/spec.md`; tối đa 256 KiB mỗi tệp, 40 tệp, 2 MiB cả lời gọi. Sai thì từ chối và không ghi gì.
- `run_dispatch` chạy `tdd-agent run advance` ở tiến trình riêng và đọc `.tdd-agent/config.toml`: đặt `[testspec] llm` và `context_mode` trong tệp đó (cờ `--set` của server MCP không truyền sang). Phía Orca: hai tool đã có trong allowlist mặc định của cầu MCP (`mcpBridge.servers["tdd-agent"].tools`). Chi tiết: [SOLUTION mục 14.11 và 14.13](tsagent/integrate/SOLUTION-INTEGRATE-TDD-AGENT.md#14-ngữ-cảnh-plan-hoặc-solution-đề-xuất-04).

## Trong `.tdd-agent/`

| Tệp | Ghi chú |
|---|---|
| `config.toml` | Cấu hình dự án. Nên commit. |
| `knowledge.db`, `operational.db` | KG và lịch sử các lần chạy (SQLite). Không commit; sinh lại bằng `generate`. |
| `changes/<tên>/` | `SRS.proposed.md`, `srs.diff` (CR đọc ra) và `review.md` (change làm gì, cho người duyệt). |
| `last-generate.json`, `logs/`, `runs/`, `staging/` | Trạng thái và vết chạy cục bộ. Không commit; `tdd-agent clean` dọn những gì không còn dùng. |
| `last-generate.json`, `logs/`, `runs/` | Trạng thái và vết chạy cục bộ. Không commit. |
| `tsagent.db`, `tsagent/` | KG test có phiên bản của tsagent, cache LLM. Không commit. |

`tdd-agent kg stats`, `kg search`, `kg conflicts`, `review --list`, `run list` … dùng cùng KG này khi chạy trong dự án. `kg search "canh bao"` tìm trong KG bằng chỉ mục full-text (không dấu, mã một phần cũng được); `kg reindex` và `kg index-check` dựng lại và kiểm chỉ mục (chỉ mục là dữ liệu dẫn xuất, không đổi nội dung KG).

## Cấu hình

`.tdd-agent/config.toml` chỉ cần ghi những gì khác mặc định (xem `tdd-agent config`). Ví dụ:

```toml
[project]
name = "P"
tdd_id = "TDD-P"
docs_dir = "docs/tdd"
prose = true

[project.sources]
urd = "docs/URD.md"
srs = "docs/SRS.md"
sad = "docs/SAD.md"

[project.id_classes]       # tiền tố mã riêng của dự án -> lớp phần tử (bổ sung cho bảng mặc định: FR, BRL, NFR, ADR, SVC, BO …)
EIF = "Interface"

[render]
language = "vi"            # vi | en
source_detail = "section"  # section = "SRS §5.1" | line = "SRS@1 d.463"

[llm]
default = "claude_code"    # claude_code | codex_cli ; theo vai: [llm.roles] author / critic / extractor

[approvals]
mode = "auto"              # xem phần "Chế độ duyệt"

[openspec]
max_revisions = 2          # số vòng viết lại gói OpenSpec khi phản biện trượt
language = ""              # ngôn ngữ văn xuôi của gói (để trống: theo mô hình)

[retrieval]
lexical = false            # true: seed gợi ý lấy từ chỉ mục full-text của KG (vẫn phải qua bước xác minh)
```

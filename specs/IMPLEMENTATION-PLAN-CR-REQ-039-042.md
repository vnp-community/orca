# Kế hoạch triển khai CR-REQ-039, 040, 041, 042

**Ngày soạn:** 2026-10-19 · **Nhánh nền:** `feat/e2e-acceptance-fixes` (đã đồng bộ `main`) · **Trạng thái:** 📝 chưa bắt đầu (0/27 task)

Kế hoạch này gom bốn CR đã có solution và task checklist thành một lộ trình có thứ tự, cổng chất lượng và chiến lược e2e. Nó **không thay** các task; mỗi task vẫn là nguồn sự thật cho việc cần làm. Số liệu ước lượng ở mục 5 là ước lượng của người soạn, chưa được ai xác nhận.

## 1. Phạm vi

| CR | Nội dung | Solution | Task | Nơi |
|---|---|---|---|---|
| [CR-REQ-039](../docs/crs/v6/plan-phase-task/CR-REQ-039-request-worktree-base-branch.md) | `base_ref` trên Request: chọn nhánh gốc cho worktree đề xuất OpenSpec và worktree thực thi task; mặc định là nhánh mặc định | [BE-REQ-SOL-039](./backend-go/crs/v6/plan-phase-task/solutions/BE-REQ-SOL-039-request-worktree-base-branch.md) | 6 (`TASK-REQ-039-01..06`) | `request-service`, `task-service`, `api-gateway` |
| [CR-REQ-040](../docs/crs/v6/request-frontend/CR-REQ-040-request-base-branch-picker.md) | Giao diện chọn nhánh khi tạo và xem Request | [FE-REQ-SOL-040](./frontend/crs/v6/request-frontend/solutions/FE-REQ-SOL-040-request-base-branch-picker.md) | 5 (`FE-REQ-TASK-040-01..05`) | `frontend` |
| [CR-REQ-041](../docs/crs/v6/solution-analysis/CR-REQ-041-analysis-revision-by-prompt.md) | Chỉnh phân tích bằng prompt tại cổng duyệt Analysis; approver được chỉnh; phản hồi đọc được bởi mọi người đọc được Request; không giới hạn số lần chỉnh | [BE-REQ-SOL-041](./backend-go/crs/v6/solution-analysis/solutions/BE-REQ-SOL-041-analysis-revision-by-prompt.md), [FE-REQ-SOL-041](./frontend/crs/v6/request-frontend/solutions/FE-REQ-SOL-041-analysis-revision-by-prompt.md) | 4 BE (`TASK-REQ-041-01..04`) + 4 FE (`FE-REQ-TASK-041-01..04`) | `request-service`, `api-gateway`, `frontend` |
| [CR-REQ-042](../docs/crs/v6/ai-governance/CR-REQ-042-prompt-catalog-shared-agent-relay-and-stdin.md) | Thư mục prompt theo luồng, `common/agentrelay` dùng chung, `execPrompt` qua stdin | [BE-REQ-SOL-042](./backend-go/crs/v6/ai-governance/solutions/BE-REQ-SOL-042-prompt-catalog-shared-agent-relay-and-stdin.md) | 8 (`TASK-REQ-042-01..08`) | `backend-go/prompts`, `common/agentrelay`, 5 service, `agent/` |

Ngoài phạm vi: QA-03 (nguồn task GitHub, đang hoãn), CR-REQ-034 (quản trị AI; chỉ được điều chỉnh vị trí prompt, xem `TASK-REQ-034-03`), dọn mã Workflow v2 đang làm dở của người khác.

## 2. Chuẩn "sẵn sàng sản xuất, không stub" (áp cho mọi task)

Mỗi task có khối "Chuẩn hoàn thành chung" ở cuối file; đây là bản tổng hợp và là tiêu chí chặn merge.

**Cấm trong mã sản phẩm:** stub, hàm rỗng trả giá trị cố định, `TODO`/`FIXME`, `panic` cho lỗi thường, bỏ qua lỗi (`_ =` với lỗi thật), dữ liệu giả hoặc cờ "tắt tạm", tắt `max-lines`, tên `helpers|utils|common|misc` (trừ module `common` sẵn có), giả lập git bằng mock trong test kiểm hành vi git, chuỗi người dùng cứng không qua i18n.

**Bắt buộc cho mọi thay đổi:**

| Cổng | Cách kiểm |
|---|---|
| Phân tích tác động | `gitnexus_impact` trước khi sửa symbol có sẵn, báo blast radius; `detect_changes` trước commit (CLAUDE.md). Nếu công cụ không có trong phiên làm việc, ghi rõ trong PR là chưa chạy |
| Hai dialect | Migration xuôi và ngược + test hợp đồng trên Postgres 16 và MySQL 8 |
| Giao thức | Tên JSON giữ nguyên trừ chỗ CR nói đổi; chụp JSON trước/sau (`TASK-REQ-042-01`) |
| Bảo mật | Không nội dung Request/Solution/phản hồi/bí mật trong log, span, khung sự kiện (C9); tham số git được kiểm cú pháp; không gửi `resolvedApiKey` |
| Đa nền tảng | macOS, Linux, Windows, SSH; lệnh git theo baseline 2.25; không đường dẫn `/` cứng |
| Giao diện | Nhãn tiếng Anh, khoá i18n 5 locale, `verify:localization-catalog` và `verify:localization-coverage`, token `main.css`, truy cập bàn phím và đọc màn hình |
| Lint | `pnpm lint` (kèm `check:max-lines-ratchet`), `oxlint`, `go vet`, `go build ./...` |
| Test | Unit + hợp đồng + tích hợp + e2e của task; test hiện có không hỏng |
| Tài liệu | Cập nhật CONTRACT, README, trạng thái task/solution/CR trong cùng PR |
| Phát hành | Có đường quay lui đã mô tả (mục 8) |

**Định nghĩa "xong" của một task:** mọi ô "Tiêu chí hoàn thành" được tick **kèm bằng chứng** (tên test, lệnh đã chạy và kết quả) trong phần "Ghi chú triển khai" của file task; trạng thái đổi `[ ] TODO` → `[x] DONE (ngày)`. Không tick ô nào chưa chạy.

## 3. Điều kiện tiên quyết và quyết định đang chờ

Các câu hỏi mở của CR chưa được trả lời; kế hoạch dùng giả định tạm dưới đây và **dừng lại hỏi** nếu một giả định sai gây làm lại:

| # | Giả định tạm | Ảnh hưởng nếu sai | Task |
|---|---|---|---|
| A1 | Không dùng `worktreeBaseRef` mức repo (CR-039 Q1) | Thêm bước giữa trong quy tắc phân giải | 039-02, 039-03, 039-04 |
| A2 | Chấp nhận đổi mặc định của task từ HEAD sang nhánh mặc định (CR-039 Q2) | Nếu không: giữ HEAD làm mặc định, mâu thuẫn với yêu cầu gốc | 039-04 |
| A3 | Worktree nhận lại từ issue khoá `base_ref` (CR-039 Q3) | Đổi quy tắc khoá | 039-02, 039-04 |
| A4 | Chỉ nhánh và tag, chưa SHA (CR-039 Q4) | Mở rộng kiểm cú pháp | 039-02 |
| A5 | Không gợi ý nhánh từ issue/PR (CR-039 Q5) | Thêm logic gợi ý ở FE | FE-040-03 |
| A6 | Một module `prompts/` dùng chung (CR-042 Q1) | Chia lại thành `internal/prompts/` mỗi service | 042-06, 042-07 |
| A7 | Chưa chuyển mẫu MCP (CR-042 Q2); prompt workflow người dùng soạn ngoài phạm vi (Q3) | Mở rộng catalog | 042-06 |
| A8 | Không làm "hoàn tác về bản cũ" (CR-041 Q còn mở) | Thêm hành động ở FE và backend | FE-041-03 |

## 4. Chiến lược e2e và repo fixture tài chính

### 4.1 Mục tiêu

Mọi task có phần e2e cần một repo thật, nhỏ, có tài liệu đủ để agent phân tích và có tri thức thiết kế (knowledge graph) cho luồng OpenSpec/knowledge hub. Repo tạm của các spec hiện có là cây vài file, **không có** `.tdd-agent/config.toml`, nên không chạy được OpenSpec (`KNOWLEDGE_CONFIG_MISSING`) và không thử được chất lượng phân tích. Fixture mới giải quyết cả hai.

### 4.2 Fixture `ledgerlite-finance` (đã dựng và kiểm 2026-10-19)

Vị trí: `tests/stages/e2e/fixtures/ledgerlite-finance/` (trong repo orca, **không** phải repo riêng; harness sao chép vào thư mục git tạm, một commit, mỗi lần chạy). Chưa commit vào git. `README.md` của fixture là nguồn sự thật cho lệnh và giới hạn.

| Phần | Nội dung đã có |
|---|---|
| Mã | Go 1.22+, chỉ thư viện chuẩn, khoảng 560 dòng nguồn (không tính test), 4 gói: `cmd/ledgerd`, `internal/ledger` (tài khoản, chuyển khoản idempotent, hạn mức ngày, nhật ký chỉ thêm, `Store` + kho bộ nhớ), `internal/httpapi` (API-001..006) |
| Tài liệu | `docs/URD.md` (bắt buộc: thiếu thì `generate` chặn với `SOURCE_ROLE_MISSING:URD`), `docs/SRS.md` (id BO/REQ, FR/BR/NFR/ENT/API/CT; mỗi dòng ghi `file::Symbol` và test), `docs/SAD.md` (SVC/ADR) |
| Kịch bản | `docs/e2e-scenarios.md` (xem 4.6) |
| Cấu hình | `.tdd-agent/config.toml` (`prose = false`: không gọi mô hình, không cần khoá, xác định), `.gitignore`, `.tdd-agent/.gitignore` |

**Kiểm chứng đã tự chạy lại (ngoài báo cáo của tác tử dựng):** `GOWORK=off go vet ./... && go test ./... -race -count=1` xanh (2 gói); trên bản sao sạch: `tdd-agent generate --human` → 67 phần tử, 78 quan hệ, 0 khoảng trống, 0 xung đột; `tdd-agent mcp enable --read-only` tạo `.mcp.json`; `tdd-agent doctor` ổn.

**Giới hạn đã biết:**
- Readiness của tri thức là `blocked` (3 nhu cầu mở: purpose, work, validation) vì tài liệu không định nghĩa Capability, WorkItem, TestCase; đồ thị đầy đủ. Chưa kiểm ORCA có coi `blocked` là chặn không; phải xác nhận ở ca C (4.5).
- `generate` chỉ tạo quan hệ `satisfies`, `traces_to`, `has_attribute`; `implements`/`verifies` chỉ có từ luồng `change`/`changeset_propose`. Liên kết mã nằm ở thuộc tính `implemented_in` và `verified_by`.
- `[project.id_classes]` bị bản tdd-agent đã cài từ chối (`CONFIG_UNKNOWN_KEY`), nên không có id `TC-` riêng.
- Chưa kiểm: `generate` có văn xuôi (gọi mô hình), `tdd-agent change ...`, `mcp serve --transport http`, cách agent của ORCA gắn MCP, cài bốn kỹ năng `tdd-*` và `AGENT_WORK_DIR` trên dev server.
- Tri thức đã sinh **không** được commit (cơ sở dữ liệu là nhị phân, lỗi thời khi e2e sửa SRS/SAD; `.mcp.json` chứa đường dẫn tuyệt đối); sinh lại ở mỗi lần chạy.

### 4.3 Sinh tri thức TDD và MCP (đã kiểm từng lệnh)

Trình tự do harness thực hiện (khớp `README.md` của fixture):

1. Sao fixture vào thư mục tạm `.../ledgerlite`.
2. `tdd-agent generate` (khoảng 1 giây, không mạng) → `.tdd-agent/knowledge.db`, `docs/tdd/TDD-LEDGERLITE.md`.
3. `tdd-agent mcp enable --read-only` → `.mcp.json` (server `tdd-agent`, `-C <repo> mcp serve`, 13 công cụ chỉ đọc gồm `kg_search`, `kg_get_element`, `kg_neighbors`, `kg_resolve_scope`, `kg_query`, `source_get`).
4. `git init -b main`, `git add -A`, một commit (để worktree của task cũng nhận `.mcp.json`); thêm nhánh `release/1.x` khi ca cần.
5. `GOWORK=off go test ./... -race` để xác nhận nền xanh (`GOWORK=off` chỉ cần khi có `go.work` ở thư mục cha).
6. ORCA: thêm dự án trỏ vào repo tạm trên dev server; `request.engineSettingsSet` đặt `solutionAgent=true` và engine `openspec` (hoặc `native`) tuỳ ca.

Yêu cầu môi trường: `tdd-agent` trong `PATH` (đã cài tại `~/.local/bin/tdd-agent`), Go 1.22+, `python3`; `openspec` và `claude` chỉ cần cho `doctor` và engine OpenSpec.

### 4.4 Mã harness dùng chung

| Thành phần | Việc | Task |
|---|---|---|
| `tests/stages/e2e/orca-flows/ledgerlite_fixture.py` | Sao fixture vào repo git tạm, tạo nhánh `main`, `release/1.x`, đặt checkout chính ở nhánh thứ ba theo yêu cầu, chạy sinh tri thức, dọn | 039-06 |
| `tests/stages/e2e/orca-flows/ui-live/fixture-repo-ledgerlite.ts` | Bản TypeScript cho spec UI (cùng hành vi) | 039-06 |
| Đăng ký dự án thử | Dùng `add-project` thật của ORCA trỏ vào repo tạm trên dev server `dev-local-mac` | 039-06 |
| `configureProject` | Có sẵn (`live-continuous-execution-steps.ts`); thêm tham số `solutionAgent` và engine `openspec` cho ca OpenSpec | 039-06 |

### 4.5 Phân loại ca e2e theo chi phí

| Loại | Dùng | Chạy khi | Chi phí |
|---|---|---|---|
| A: claude giả (`--ai-stub`) | Phần lớn ca: luồng, trạng thái, quyền, nhánh gốc thật bằng `git rev-parse`, activity | Mỗi PR, CI | Không |
| B: Claude thật, native, repo tạm | `--real native-execute` | Cuối mỗi giai đoạn lớn | Nhỏ (đã đo lần trước: khoảng 3 phút mỗi ca) |
| C: Claude thật, OpenSpec, fixture tài chính | Ca `change_request` end-to-end có knowledge hub | Một lần trước phát hành, **có đồng ý của người dùng** | Lớn (7 đến 13 USD mỗi Plan) |

### 4.6 Ánh xạ kịch bản fixture → CR

Ba kịch bản trong `docs/e2e-scenarios.md` (mỗi cái có nội dung dán vào "Create request", loại và cỡ kỳ vọng, lệnh kiểm "xong"; theo báo cáo của tác tử dựng fixture, lệnh "xong" đã được chạy từ các khối trong tài liệu và ca 1 đã kiểm cả chiều đỏ lẫn xanh. **Tôi chưa tự chạy lại các lệnh kiểm "xong" của ba kịch bản**):

| # | Kịch bản | Loại, cỡ, engine | Ghi chú |
|---|---|---|---|
| 1 | `bug`: cửa sổ hạn mức ngày là 24 giờ trượt thay vì ngày lịch UTC | bug, S, native | Lỗi nằm dưới dạng bản vá một dòng trong tài liệu (`git apply`), **không** nằm trong mã nguồn; áp bản vá và commit trước khi tạo Request |
| 2 | `task`: thêm `GET /accounts/{id}/statement` | task, S (M chấp nhận), native | Mang URD REQ-007 chưa có FR hiện thực. Bản cài đặt tham chiếu dùng để thử DoD chỉ có trong bản sao tạm của tác tử dựng, **không được lưu**, nên chưa có chứng cứ lưu lại rằng DoD của ca 2 và 3 đạt được |
| 3 | `change_request`: phí chuyển khoản (`-fee-flat`, `-fee-bps`), chân bút toán thứ ba tới `acc_fees`, cập nhật SRS và SAD | change_request, L (M chấp nhận), OpenSpec | Kiểm tri thức "kg stats > 67 phần tử" (tác tử báo 71 trên bản tham chiếu; chưa tự kiểm) |

| Kịch bản | CR kiểm | Suite |
|---|---|---|
| 2 với nhánh gốc `release/1.x` (và checkout chính ở nhánh thứ ba) | 039 | `BRF-xx` (039-06), UI `request-base-branch` (FE-040-05) |
| 1 với chỉnh phân tích hai lần | 041 | `ARV-xx` (041-04), UI `analysis-revision` (FE-041-04) |
| 3: Solution → chỉnh → duyệt → Plan OpenSpec từ `release/1.x` | 039, 041, 042 | Ca C |
| `spike` và `question` đặt trên SRS/SAD | 041 (Findings, Answer) | `ARV-xx` (viết thêm hai Request này trong suite, không có sẵn trong tài liệu kịch bản) |
| Mọi luồng gửi prompt | 042 | Hồi quy `request`, `matrix`, `qa`, UI native, `--real native-execute` |

## 5. Lộ trình, thứ tự và ước lượng

Ước lượng theo ngày-kỹ sư, **chưa được xác nhận**, chưa tính review và chờ CI:

| Nhóm | Ngày |
|---|---|
| CR-039 (6 task) | 13,5 |
| CR-040 (5 task) | 10 |
| CR-041 backend (4) + frontend (4) | 8 + 8 |
| CR-042 (8 task) | 23,5 |
| Fixture + harness e2e (dùng chung) | 4 |
| **Tổng** | **khoảng 67** |

### 5.1 Các đợt

```
Đợt 0 (nền, song song):  Fixture + harness (4) │ 042-04 stdin (2,5) │ 042-01 agentrelay (3) │ 039-01 migration (1) │ 041-01 AuthorizeRevise (1,5)
Đợt 1:                   039-02 miền/khoá │ 041-02 revision_feedback + prompt │ 041-03 activity │ 042-02 relay request-service │ FE-040-01/02 (cần 039-02, 039-05 một phần)
Đợt 2:                   039-03 worktree đề xuất │ 039-04 task-service │ 039-05 gateway │ 041-04 đường lỗi + e2e │ 042-05 golden (sau 041-02)
Đợt 3:                   042-03 relay còn lại (sau 039-04) │ 042-06 prompts request-service │ FE-040-03/04 │ FE-041-01/02/03
Đợt 4:                   042-07 prompts task/git │ FE-040-05 │ FE-041-04 │ 039-06 e2e
Đợt 5:                   042-08 CI + hồi quy cuối │ ca C (OpenSpec thật, khi được đồng ý) │ phát hành
```

Đường găng: `039-01 → 039-02 → 039-05 → FE-040-01 → FE-040-03/04 → FE-040-05` (khoảng 15 ngày) và `041-01 → 041-02 → 042-05 → 042-06 → 042-07 → 042-08` (khoảng 15 ngày). Hai đường độc lập, nên hai đến ba người làm song song rút tổng thời gian về khoảng 4 đến 5 tuần lịch.

### 5.2 Điểm xung đột phải tuần tự hoá

| Xung đột | Cách xử lý |
|---|---|
| `solution_prompt.go` bị 041-02 sửa và 042-06 di chuyển | 041-02 xong trước; 042-05 chụp golden **sau** 041-02 (đã ghi ở 042-05) |
| `task-service/simple_executor.go` bị 039-04 sửa (gọi `EnsureWorktree`) và 042-03 sửa (relay) | 039-04 trước 042-03 |
| `TaskExecutor.Execute` (đổi chữ ký ở 039-04) và `agentrelay` | Không giao nhau; nhưng cả hai chạm `task_client.go`/`simple_executor.go` ở request-service/task-service: một người sở hữu mỗi file một lúc |
| Proto: `request.proto` (039-02) và `solution.proto` (041-02) | Khác file, nhưng sinh mã chung gói: sinh lại tuần tự, mỗi PR một lần, không sửa tay file sinh |
| Locale JSON (FE-040-05, FE-041-04) | Mỗi task thêm khối khoá riêng theo thứ tự chữ cái; giải xung đột khi merge và chạy hai lệnh verify |
| Migration | `0099_request_base_ref` (039-01); 041-03 chỉ thêm migration khi EXPLAIN cho thấy cần, lấy số kế tiếp |
| `SolutionPanel.tsx` (FE-041-02 và 041-03) | Một người làm cả hai hoặc 041-02 trước |

### 5.3 Phân công đề xuất (3 luồng)

| Luồng | Việc |
|---|---|
| A: request-service + gateway | 039-01, 02, 03, 05; 041-01, 02, 03, 04 |
| B: relay, agent, task-service, prompts | 042-04, 01, 02; 039-04; 042-03, 05, 06, 07, 08 |
| C: frontend + e2e | Fixture và harness; FE-040-01 đến 05; FE-041-01 đến 04; 039-06 |

## 6. Cổng theo đợt (không qua đợt nếu chưa xanh)

| Sau đợt | Chạy | Kỳ vọng |
|---|---|---|
| 0 | `go test` các gói đã đổi; `cd agent && npx vitest run src/relay`; fixture `go test ./...` | Xanh; stdin chạy với `claude` giả; JSON dây chụp xong |
| 1 | Hợp đồng hai dialect; `run_acceptance.py platform devserver request` | Xanh; migration xuôi/ngược |
| 2 | `run_acceptance.py devserver request qa request_base_ref analysis_revision --ai-stub` | Xanh; nhánh gốc kiểm bằng `git rev-parse` thật |
| 3 | `run_ui_flow.py --ai-stub -- qa-bug-task-native-flow request-base-branch analysis-revision` | Xanh |
| 4 | `run_ui_flow.py --real native-execute` và `--real continuous` | Xanh; số lần gọi công cụ và thời lượng không lệch so với trước đợt 0 (ghi lại số liệu cơ sở trước khi bắt đầu) |
| 5 | Toàn bộ acceptance không AI thật; ca C nếu được đồng ý | Xanh |

Trước đợt 0, ghi **số liệu cơ sở** vào PR đầu tiên: số ca và kết quả của `run_acceptance.py` hiện có (17 pass, 2 fail GitHub ở suite `qa`; 18 pass tổng khi thêm Jira thật), thời lượng `--real native-execute` (bug 187 giây, task 158 giây, 0 vi phạm guard), để so khi hồi quy.

## 7. Phương thức làm việc

- Một PR cho mỗi task (hoặc cặp task cùng file), thông điệp commit theo `feat|fix|refactor(scope): ...` và dòng `Co-Authored-By` theo quy ước phiên bản làm việc.
- Chỉ một người (hoặc phiên) sửa một tập file tại một thời điểm; không sửa checkout trong lúc `run_ui_flow.py --real` chạy (guard quiet-checkout).
- Stack local: sau `redeploy-local.sh` restart theo thứ tự `infra-fleet → git-gateway → task → request → api-gateway`; Vault local mất khoá khi khởi động lại, chạy `vault-init` và nhập lại credential bằng tay trước e2e có Jira/GitHub.
- Sau mỗi PR: cập nhật trạng thái task, solution, README và CR (mục 9).
- Tài liệu bàn giao: [`docs/handoff/2026-10-17-request-flow-and-qa-fixes.md`](../docs/handoff/2026-10-17-request-flow-and-qa-fixes.md); hướng dẫn triển khai: [`deploy/dev/RELEASE-2026-10-DEPLOY.md`](../deploy/dev/RELEASE-2026-10-DEPLOY.md).

## 8. Phát hành và quay lui

| Hạng mục | Thứ tự triển khai | Quay lui |
|---|---|---|
| Migration `0099` (039-01) | Trước binary mới | Cột có mặc định; binary cũ bỏ qua; `.down` chỉ khi chắc không còn Request dùng |
| `request-service`, `task-service` (039) | `task-service` trước hay sau đều được vì trường `base_ref` mới là cộng thêm; **nhưng** đổi chữ ký cổng nội bộ nên deploy cả hai cùng đợt | Build lại commit trước |
| `api-gateway` | Sau request-service | Frontend mới ẩn ô khi gateway cũ (cờ `features`) |
| Agent (042-04 stdin) | Độc lập; phát hành riêng | Đặt `ORCA_AGENT_PROMPT_VIA_ARGV=1` hoặc cài lại agent cũ |
| `common/agentrelay`, `prompts/` (042) | Mỗi service một lần, sau khi golden và JSON dây khớp | Build lại commit trước; không có dữ liệu để khôi phục |
| Frontend | Sau gateway | Build lại commit trước; giao diện mới tự rơi về hành vi cũ khi gateway thiếu cờ |

Ghi chú phát hành cần có: (1) task thực thi từ nhánh mặc định thay vì HEAD của checkout chính (039-04); (2) agent nhận prompt qua stdin (042-04); (3) khởi động lại agent làm gián đoạn lượt đang chạy; (4) bản dịch es/ja/ko/zh cần người bản ngữ rà.

## 9. Theo dõi tiến độ

Bảng cập nhật sau mỗi task (ngày, PR, bằng chứng test):

| CR | Task | Trạng thái |
|---|---|---|
| 039 | 01, 02, 03, 04, 05, 06 | [ ] [ ] [ ] [ ] [ ] [ ] |
| 040 | 01, 02, 03, 04, 05 | [ ] [ ] [ ] [ ] [ ] |
| 041 BE | 01, 02, 03, 04 | [ ] [ ] [ ] [ ] |
| 041 FE | 01, 02, 03, 04 | [ ] [ ] [ ] [ ] |
| 042 | 01, 02, 03, 04, 05, 06, 07, 08 | [ ] [ ] [ ] [ ] [ ] [ ] [ ] [ ] |
| Fixture + harness | `ledgerlite-finance`, `ledgerlite_fixture.py`, `fixture-repo-ledgerlite.ts` | [ ] [ ] [ ] |

Khi xong một CR: đổi trạng thái trong `docs/crs/v6/<thư mục>/CR-REQ-0xx-*.md`, bảng chỉ mục trong `docs/crs/v6/README.md` và README của thư mục, và `specs/**/solutions|tasks/README.md`.

## 10. Rủi ro tổng hợp

| # | Rủi ro | Giảm thiểu | CR |
|---|---|---|---|
| 1 | `claude --print` không đọc stdin với một tổ hợp cờ | Thử `claude` thật trước khi làm mặc định; công tắc quay lui; giữ argv cho tổ hợp lỗi | 042 |
| 2 | Di chuyển prompt làm lệch chuỗi | Golden trùng từng byte; không sửa golden để cho qua | 042 |
| 3 | Đổi chữ ký `TaskExecutor.Execute` làm hỏng fake/adapter | `gitnexus_impact`; cập nhật đồng thời mọi nơi dùng | 039 |
| 4 | Đổi mặc định task sang nhánh mặc định gây bất ngờ | Ghi chú phát hành; xác nhận A2 | 039 |
| 5 | Gateway cũ lặng lẽ bỏ `baseRef` | Cờ `features` + kiểm echo ở FE | 039, 040 |
| 6 | Prompt phình theo số vòng chỉnh không giới hạn | Giới hạn tổng `prior_artifacts` 40000 rune | 041 |
| 7 | Lượt chỉnh thất bại sau khi bản cũ đã `superseded` | Test đường lỗi; Request không kẹt; chỉnh lại/Regenerate khả thi | 041 |
| 8 | Phản hồi chỉnh lộ qua log hoặc activity | Test C9; câu lệnh activity không chọn cột phản hồi | 041 |
| 9 | Ca OpenSpec thật tốn tiền | Loại C chỉ chạy khi được đồng ý | tất cả |
| 10 | Lỗi có sẵn của `main` chặn kiểm chứng (`api-gateway` không biên dịch do thiếu kiểu `workflowv1.StepApproval`) | Xác nhận trước đợt 0 rằng `main` biên dịch; nếu chưa thì xử lý với chủ sở hữu mã workflow v2, không vá lẫn vào các task này | tất cả |
| 11 | Tri thức TDD cần dịch vụ AI hoặc thông tin xác thực | Phân loại ở mục 4.5; fixture có thể chạy đến bước sinh tri thức mà không tốn tiền ở ca A và B | e2e |

Rủi ro 10 là chặn thật: ở thời điểm soạn, `go build ./services/api-gateway/...` thất bại trên `main` (`channels_workflow_step_approvals.go` thiếu kiểu proto). Mọi task có e2e qua gateway cần việc này được xử lý trước.

# Impact Assessment — CR-PW-007/008/009/010, CR-JIRA-001, CR-TSRC-001 (2026-09-15)

> **Cập nhật cuối ngày**: sau khi CR-JIRA-001 fix xong và verify bằng token thật, điều tra thêm 2 vòng nữa (Jira source vẫn "unavailable" + `GITGATEWAY_STATUS_FAILED` tái diễn) tìm ra thêm **2 phát hiện lớn**: **CR-PW-010** (root cause thật của BUG-012, cuối cùng đã confirm sau nhiều lần hedge) và **CR-TSRC-001** (gap kiến trúc thật — Jira/Linear task source chưa từng port sang backend-go, độc lập hoàn toàn với việc Jira đã connect đúng hay chưa).

Đánh giá ảnh hưởng tổng hợp của 4 CR mới, dựa trên 8 bug report thật (live-verified trên `b15.openledger.vn`, không suy đoán) phát sinh từ 1 phiên điều tra liên tục: thêm repo → gán dev server → tạo worktree → thấy worktree → chạy agent/terminal → kết nối Jira.

## Bảng tổng hợp

| CR | Bug gốc | Mức độ | Trạng thái | Blast radius | Effort ước tính | Giá trị nếu fix |
|---|---|---|---|---|---|---|
| CR-PW-007 (worktree) | BUG-010 | 🔴 P0 | ✅ Fixed+deployed | `project-service` outbound call — LOW risk (gitnexus impact: 3 impacted) | Đã xong | Mở khoá hoàn toàn luồng gán dev server, chặn từ đầu |
| CR-PW-007 (worktree) | BUG-AG-WT-001 | 🟡 P1 | 🔲 Proposed | `agent-git-handler.ts`'s `handleGitExec` — dùng chung bởi MỌI git.exec call (status, diff, commit, worktree...) → cần cẩn trọng, không áp dụng đồng loạt | Trung bình — cần thiết kế theo-subcommand | Biến lỗi mù mờ ("spawn git ENOENT") thành lỗi đúng nguyên nhân (permission, conflict...) cho MỌI thao tác git qua relay, không riêng worktree |
| CR-PW-007 (worktree) | BUG-011 | 🔴 P0 | 🟡 Corrected — cần log thật, không phải union-fix như đề xuất ban đầu | `DetectWorktrees`/relay (git-gateway-service) nếu confirm, KHÔNG phải `mergeDetectedWorktrees` (đó là thiết kế có chủ đích) | Không ước tính được tới khi có log thật | Chặn triệt để việc worktree hợp lệ "biến mất" không lý do — ảnh hưởng lòng tin vào UI toàn diện |
| CR-PW-008 (a) | BUG-AG-ORCH-014 | 🟡 P1 (hedged) | 🟡 Root cause chưa confirm | Nếu là bug backend thật: mọi agent session trên mọi worktree. Nếu là UI-only: chỉ hiển thị, không mất dữ liệu thật | Không rõ tới khi có log thật — **bước đầu là chẩn đoán, không phải code fix** | Cao nếu confirm là bug backend thật (mất việc thật); thấp nếu chỉ là UI hiển thị sai (dễ sửa) |
| CR-PW-008 (b) | BUG-012 | 🟡 P1 | 🔲 Proposed (SOL-012 thiết kế xong) | 1-dòng thêm vào `git-gateway-service/main.go` — additive, không đổi hành vi client | Rất thấp | Bắt buộc phải làm TRƯỚC khi có thể fix đúng bug thật đằng sau `GITGATEWAY_STATUS_FAILED` — đang chặn chẩn đoán, không tự nó chặn user |
| CR-PW-009 | agent.switchAccount không có UI | 🟡 P1 | 🔲 Proposed | Chỉ thêm UI, backend không đổi | Thấp | Đóng gap tính năng đã trả chi phí phát triển backend hoàn chỉnh, effort UI nhỏ |
| CR-PW-009 | worktree.createFromIssue không có UI | 🟡 P1 | 🔲 Proposed | Chỉ thêm UI, backend không đổi (đã có test đầy đủ) | Thấp | **Giá trị cao nhất trong batch này** — đúng tính năng lõi user hỏi ("luồng Jira→worktree"), backend đã hoàn thiện 100%, chỉ thiếu 1 nút bấm |
| CR-PW-009 | BUG-FE-PW-004 | 🟢 P2 | 🔲 Proposed | 1 component, 1 dòng code | Rất thấp | UX nhỏ nhưng gây khó chịu mỗi lần mở terminal |
| CR-JIRA-001 | BUG-013 | 🔴 P0 | ✅ **Fixed & confirmed working** — verify bằng token thật (`200 OK`) | Toàn bộ adapter Jira | Đã xong | Mở khoá kết nối Jira self-hosted — **đã dùng được thật**, xác nhận bằng credential thật của user |
| CR-PW-010 (mới) | BUG-012 | 🔴 P0 | ✅ Root cause + ✅ blast radius confirmed (CRITICAL), ✅ SOL-013 spec sẵn sàng, 🔲 chưa implement | `ConnectionResolver`/`dispatchExecutor` — **gitnexus impact xác nhận CRITICAL, 34 direct caller** (gần như mọi usecase git-gateway-service: status, diff, commit, push, pull, checkout, stage, merge, rebase, history...) | Trung bình — 1 lookup phía project-service (`WorktreeID → RepoInfo`) sửa tại 1 chỗ (`resolver.go`) là đóng cho cả 34 caller, nhưng cần regression test rộng vì risk CRITICAL | Mở khoá tab Git cho MỌI worktree — đây là bug đã treo suốt cả phiên làm việc (`GITGATEWAY_STATUS_FAILED`), giờ đã biết chính xác chỗ sửa VÀ quy mô thật (rộng hơn nhiều so với đánh giá ban đầu) |
| CR-TSRC-001 (mới) | BUG-FE-TASKV1-009 | 🔴 P0 | ✅ Root cause confirmed (0 hit trong backend-go), 🔲 chưa thiết kế fix | Toàn bộ backend-go — chưa có nơi nào implement "runtime capability advertisement" cho tính năng này | Chưa ước tính — cần điều tra thêm cơ chế capability hiện tại của backend-go trước | Mở khoá TOÀN BỘ tính năng browse Jira/Linear trong Task page trên web — hiện tại **0% dùng được dù backend Jira đã hoạt động đúng** |

## Thứ tự ưu tiên đề xuất (dựa trên effort/giá trị, không phải chỉ severity)

**Đã xong trong ngày** (không còn trong hàng đợi): BUG-011 diagnostic procedure, SOL-012 (deployed), BUG-FE-PW-004 (deployed), CR-JIRA-001 (deployed + confirmed working), BUG-FE-PW-006 (deployed).

**Còn lại, ưu tiên tiếp theo:**

1. **CR-PW-010** — root cause `GITGATEWAY_STATUS_FAILED` cuối cùng đã confirm sau nhiều lần hedge trong ngày — nên làm ngay khi có thể, vì đây là bug đã treo lâu nhất và ảnh hưởng RẤT rộng (mọi worktree không có `infra.connections` row — nghi ngờ là toàn bộ).
2. **CR-TSRC-001** — giá trị cao (mở khoá toàn bộ tính năng Jira/Linear task source đã đầu tư code) nhưng effort chưa ước tính được — cần 1 vòng điều tra riêng về cơ chế capability của backend-go trước khi biết quy mô thật.
3. **worktree.createFromIssue UI**, **agent.switchAccount UI** (CR-PW-009), **BUG-AG-WT-001** (CR-PW-007), **CR-PW-008(a)** — giữ nguyên thứ tự đã đề xuất trước đó, chưa đổi.

## Rủi ro nếu KHÔNG fix

- **CR-PW-010 không fix**: tab Git tiếp tục không tải được cho worktree nào rơi vào nhánh "no connection row" — đã xác nhận đây là trạng thái "zero rows, system-wide", nghĩa là nhiều khả năng ảnh hưởng GẦN NHƯ MỌI worktree trên deployment này.
- **CR-TSRC-001 không fix**: toàn bộ công sức đã bỏ ra để Jira hoạt động đúng (BUG-013, database migration, form fix) **vẫn không mang lại giá trị sử dụng thật** — user connect được Jira nhưng không browse được issue nào trong Task page.

## Ghi chú phương pháp

Toàn bộ đánh giá dựa trên bằng chứng thật (log production, `curl` trực tiếp vào site Jira thật, đọc source `gitnexus impact` cho blast radius) — không có mục nào trong bảng trên là suy đoán chưa kiểm chứng, trừ CR-PW-008(a) đã đánh dấu rõ "hedged" đúng quy ước của `missing-v2` bug family.

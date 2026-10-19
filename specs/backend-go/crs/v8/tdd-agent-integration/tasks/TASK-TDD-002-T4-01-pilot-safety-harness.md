# TASK-TDD-002-T4-01: An toàn cho pilot: chặn push, dấu vân tay, ảnh chụp dữ liệu, watchdog

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 3.7, 5 (tiêu chí 6)
**Checklist:** [T4.1](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P0 · **Area:** tests/stages/e2e/orca-flows
**Files:** `agent_push_block.py` (`PushBlockedAgent`), `ui-live/live-main-checkout-guard.ts` (`captureCheckout`, `removeGrowth`), `live-main-checkout-rules.ts` (`checkoutViolations`, `restoredProblems`), mới `pilot_guard.py`, `report_orphan_requests.py`, `report_orphan_worktrees.py`
**Depends on:** T0.5, T2.12
**Status:** `[ ]` TODO

---

## Context

Pilot chạy agent `claude --dangerously-skip-permissions` trên repo có remote GitHub thật (vnp-qa-platform). Bộ e2e đã có các biện pháp cho repo của người dùng: chặn push bằng biến môi trường `GIT_CONFIG_*` của tiến trình agent (`PushBlockedAgent`: `remote.origin.pushurl` và `pushInsteadOf` trỏ về đường dẫn không tồn tại, không đụng `.git/config` hay `~/.gitconfig`), dấu vân tay thư mục chính (HEAD, nhánh, tệp theo băm, mọi ref, danh sách worktree) so trước/sau, watchdog giữa run, so ảnh chụp dữ liệu người dùng. Các hàm hiện nằm trong mã TypeScript/Python của bộ e2e và gắn với repo `deepseek-harness`; pilot cần dùng được cho repo khác.

## Việc cần làm

1. Dùng `PushBlockedAgent` (cách `run_ui_flow.py` dùng) hoặc thêm điểm vào độc lập `python3 agent_push_block.py start|status|restore` để khởi động lại `dev-local-mac` với khối chặn push và khôi phục sau pilot; kiểm mọi tiến trình agent mang đủ `GIT_CONFIG_*` (`ps eww`).
2. Chứng minh chặn thật: trong worktree pilot chạy `git push --dry-run origin HEAD` và `git push --dry-run <url GitHub gõ tay>` phải thất bại cục bộ.
3. Viết `pilot_guard.py snapshot|compare --repo <đường dẫn> --out <file>`: ghi HEAD, nhánh, `git status --porcelain=v1 -z` kèm băm nội dung từng tệp đổi/untracked, `git for-each-ref`, `git worktree list`, `git config --local --list`; `compare` báo mọi khác biệt ngoài tập được phép (worktree/nhánh mới do run tạo). Tái dùng quy tắc của `live-main-checkout-rules.ts` làm đặc tả.
4. Ảnh chụp dữ liệu người dùng trong Orca: danh sách project, Request (số, trạng thái) qua REST/CLI trước và sau; so khác biệt (chỉ cho phép project/Request do pilot tạo).
5. Watchdog giữa run: vòng lặp `pilot_guard.py watch --interval 15` so với ảnh chụp, thoát khác 0 và in cảnh báo khi thư mục chính đổi.
6. Chạy `report_orphan_requests.py` và `report_orphan_worktrees.py` làm bước kiểm cuối.

## Tiêu chí hoàn thành

- [ ] `git push --dry-run` thất bại cục bộ trong worktree pilot khi agent chạy với khối chặn.
- [ ] `pilot_guard.py` chạy được trên repo bất kỳ và có test với repo tạm (đổi tệp, thêm nhánh, thêm worktree).
- [ ] Quy trình lặp được bằng lệnh, có tài liệu ngắn.

## Kiểm thử

- Test `pilot_guard.py` (unittest/pytest) với repo tạm; thử tay chặn push.

## Rủi ro và ghi chú

- Chặn push chỉ nằm trong môi trường agent; người dùng vẫn push được bằng shell của họ.
- B4 vẫn mở: các biện pháp này phát hiện và dừng, không ngăn mọi thao tác ghi ngoài worktree.

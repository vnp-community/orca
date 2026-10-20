# MiniPay CR-001: test specs sinh bằng tsagent v2 (chạy lại 2026-10-13)

Đầu vào: patch TDD đã duyệt và `solution.md` của lần chạy workflow thật (Claude) 2026-10-09, giữ ở `tmp/minipay-demo/changes/CR-001/run/`. Chỉ bước `testspec` + `gate_testspec` chạy lại với tsagent v2: `TSAGENT_LLM=claude-cli`, `TSAGENT_LLM_PARALLEL=4`, DB và cache LLM mới (không dùng kết quả cũ), chế độ `service` (không có lượt bổ sung `supplement.md`).

| Thư mục | Nội dung |
| --- | --- |
| `testspec/01-test-requirements.md`, `02-test-design.md`, `03-test-cases.md` | Ba tài liệu, phiên bản 1 Baselined, CR-001 |
| `testspec/kg-slice.tdd-kg.json` | Gói `tdd-kg/v1` workflow gửi cho tsagent |
| `testspec/solution-sent.md` | solution gửi kèm (có front matter) |
| `testspec/testspec.json` | Tóm tắt của workflow |
| `solution/solution.md`, `tdd/tdd_patch.approved.json` | Đầu vào |

So sánh với v1: [../../COMPARE-E2E.md](../../COMPARE-E2E.md).

# CR v8: tích hợp tdd-agent vào luồng Request của Orca

| Khu vực | Solution | Checklist | Trạng thái |
|---|---|---|---|
| [tdd-agent-integration](tdd-agent-integration/) | [SOL-TDD-002](tdd-agent-integration/solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md): tdd-agent là knowledge hub; agent do Orca gọi tự khai thác knowledge qua MCP và skill; `plan generate` trả bộ OpenSpec (bản 2) | [CHECKLIST-TDD-002](tdd-agent-integration/tasks/CHECKLIST-TDD-002-knowledge-hub.md) | đang áp dụng; tasks chi tiết: [tasks/README.md](tdd-agent-integration/tasks/README.md) |
| | [SOL-TDD-001](tdd-agent-integration/solutions/SOL-TDD-001-tdd-agent-in-request-flow.md): dùng pipeline `tdd-agent change` sinh OpenSpec | [CHECKLIST-TDD-001](tdd-agent-integration/tasks/CHECKLIST-TDD-001-pilot-vnp-qa-platform.md) | tạm dừng sau spike; giữ làm tham chiếu |

Bộ skill: `skills/tdd-knowledge`, `skills/tdd-openspec-author`, `skills/tdd-knowledge-writeback`, `skills/tdd-orca-task`. Pilot: `vnp-qa-platform` (BL-H01, BL-H02). Giai đoạn 2 (T2.1 đến T2.15) đã có code và test (mặc định tắt, không đổi hành vi khi tắt); hợp đồng đã đổi: [tdd-agent-integration/CONTRACTS.md](tdd-agent-integration/CONTRACTS.md); hướng dẫn vận hành: [docs/guides/request/knowledge-hub-setup.md](../../../../docs/guides/request/knowledge-hub-setup.md). Giai đoạn 3 đến 5 chưa làm.

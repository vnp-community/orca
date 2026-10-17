# tdd-project (fixture)

Tiny tdd-agent project for the split-topology e2e: URD/SRS/SAD from `tdd-agentic-ai/tests/test_agents_change_flow.py`,
a CR (`docs/CR-1.md`) and an SRS edited for it (`docs/SRS-v2.md`). The dev server image seeds it under
`/srv/projects/tdd-project` with `tdd-agent init --no-prose --no-generate` (no language model: deterministic),
test specs with `[testspec] llm = "none"`, and MCP write + approvals for the client `orca-workflow`.

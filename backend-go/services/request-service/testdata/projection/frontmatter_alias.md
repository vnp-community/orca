---
orca_schema: 1
kind: solution
id: SOL-142.2
request: &a REQ-142@r2
supersedes_alias: *a
status: proposed
supersedes: SOL-142.1
digest: "sha256:5b810c9753238d5951e1669cd594c740c53ed05aaed4597a98cce3a9e27825dc"
generated_by: {kind: native, tool: ai.complete, model: claude-x, run: run-1}
---
# Đăng nhập bằng SSO

<!-- orca:begin summary digest=sha256:4dd9ddf72835527f3914eda201f1912435b164f6767532647a5f51b231645d9d -->
## Summary
- recommendation:
  - option_id: opt-1
  - reason: ít rủi ro

```orca-json
{"recommendation":{"option_id":"opt-1","reason":"ít rủi ro"}}
```
<!-- orca:end summary -->

<!-- orca:begin options digest=sha256:a89c46866e0b95d3d45eff0254801b155292117ce47421314f5de3a149edff2e -->
## Options
- options:
  - #1:
    - affected_areas:
      - #1:
        - description: route
        - name: api-gateway
    - approach: Cách làm chi tiết
    - effort:
      - hours_estimate: 8
      - size: M
    - id: opt-1
    - risk:
      - description: thấp
      - level: low
    - summary: Tóm tắt
    - title: Phương án 1
  - #2:
    - affected_areas:
      - #1:
        - description: route
        - name: api-gateway
    - approach: Cách làm chi tiết
    - breaking_change: true
    - effort:
      - hours_estimate: 8
      - size: M
    - id: opt-2
    - risk:
      - description: thấp
      - level: low
    - summary: Tóm tắt
    - title: Phương án 2

```orca-json
{"options":[{"affected_areas":[{"description":"route","name":"api-gateway"}],"approach":"Cách làm chi tiết","effort":{"hours_estimate":8,"size":"M"},"id":"opt-1","risk":{"description":"thấp","level":"low"},"summary":"Tóm tắt","title":"Phương án 1"},{"affected_areas":[{"description":"route","name":"api-gateway"}],"approach":"Cách làm chi tiết","breaking_change":true,"effort":{"hours_estimate":8,"size":"M"},"id":"opt-2","risk":{"description":"thấp","level":"low"},"summary":"Tóm tắt","title":"Phương án 2"}]}
```
<!-- orca:end options -->

<!-- orca:begin requirement-coverage digest=sha256:6bd297b460f16e10ae8cdc8f11c63d3260f335d08bcbe1fa32f4a1bae3b57119 -->
## Requirement coverage
- requirement_coverage:
  - #1:
    - ac_id: AC-1
    - option_ids:
      - #1: opt-1
      - #2: opt-2
    - status: covered
  - #2:
    - ac_id: AC-2
    - note: để sau
    - option_ids:
    - status: out_of_scope

```orca-json
{"requirement_coverage":[{"ac_id":"AC-1","option_ids":["opt-1","opt-2"],"status":"covered"},{"ac_id":"AC-2","note":"để sau","option_ids":[],"status":"out_of_scope"}]}
```
<!-- orca:end requirement-coverage -->

<!-- orca:begin assumptions digest=sha256:56494cf40779067aed2d64255de8317e10277ffd6ca5af65b01f7625455f0b13 -->
## Assumptions
- assumptions:
  - #1:
    - id: A-1
    - needs_confirmation: true
    - text: IdP đã có

```orca-json
{"assumptions":[{"id":"A-1","needs_confirmation":true,"text":"IdP đã có"}]}
```
<!-- orca:end assumptions -->

<!-- orca:begin open-questions digest=sha256:ba0e5298a7708272d7e6e422398a20e31d34c041d2d2833ac1b21850d2b6dbe4 -->
## Open questions
- open_questions:
  - #1:
    - blocking: false
    - id: Q-1
    - text: Có cần MFA?

```orca-json
{"open_questions":[{"blocking":false,"id":"Q-1","text":"Có cần MFA?"}]}
```
<!-- orca:end open-questions -->

<!-- orca:begin details digest=sha256:478145d4a786d4bf102585a446288319d347c3403d2e33579ff2ec044eac9a70 -->
## Details
- constraints:
  - #1: không đổi DB
- evidence_refs:
  - #1: file:main.go:10
- non_functional:
  - #1:
    - kind: security
    - text: TLS 1.3
- schema_version: 1
- test_strategy: e2e

```orca-json
{"constraints":["không đổi DB"],"evidence_refs":["file:main.go:10"],"non_functional":[{"kind":"security","text":"TLS 1.3"}],"schema_version":1,"test_strategy":"e2e"}
```
<!-- orca:end details -->

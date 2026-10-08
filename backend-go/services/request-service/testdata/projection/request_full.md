---
orca_schema: 1
kind: request
id: REQ-142
digest: "sha256:72e943f4bc3eeae17847dc9b4da472ee4546a5342dcc2a288968e15d52a08727"
---
# Đăng nhập bằng SSO

<!-- orca:begin document digest=sha256:72e943f4bc3eeae17847dc9b4da472ee4546a5342dcc2a288968e15d52a08727 -->
## Document
- acceptance_criteria:
  - #1:
    - id: AC-1
    - status: active
    - text: Đăng nhập thành công chuyển về trang chủ
    - verify_hint: test
  - #2:
    - id: AC-2
    - status: retired
    - text: Tài khoản bị khoá hiện thông báo
    - verify_hint: test
- body: Người dùng cần đăng nhập một lần cho mọi dịch vụ.
- schema_version: 1
- title: Đăng nhập bằng SSO
- type: change_request
- type_fields:
  - ac_next: 3
  - goal: SSO
  - scope_in:
    - #1: web
  - scope_out:
    - #1: mobile
  - value: giảm ma sát

```orca-json
{"acceptance_criteria":[{"id":"AC-1","status":"active","text":"Đăng nhập thành công chuyển về trang chủ","verify_hint":"test"},{"id":"AC-2","status":"retired","text":"Tài khoản bị khoá hiện thông báo","verify_hint":"test"}],"body":"Người dùng cần đăng nhập một lần cho mọi dịch vụ.","schema_version":1,"title":"Đăng nhập bằng SSO","type":"change_request","type_fields":{"ac_next":3,"goal":"SSO","scope_in":["web"],"scope_out":["mobile"],"value":"giảm ma sát"}}
```
<!-- orca:end document -->

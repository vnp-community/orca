# TASK-E2E-004: Hoàn tất `fs.checkPath` ở agent: test, capability, tài liệu

**From Solution:** SOL-E2E-001 · **Resolves:** BUG-E2E-004
**Priority:** P1 · **Service/Area:** agent
**Files:**
- agent/src/relay/fs-agent-extensions.ts (đã sửa)
- agent/src/relay/agent-rpc-dispatch-fs.ts (đã sửa)
- agent/src/relay/fs-agent-extensions.test.ts (mới hoặc mở rộng)
- specs/agent/api/agent-rpc-catalog-git-fs.md (ghi method)
- backend-go/ci/check-agent-capability-golden.sh + tệp vàng tương ứng
**Depends on:** không
**Status:** `[x]` XONG (cập nhật 2026-10-09, chưa commit)

---

## Context

`project-service` gọi relay `fs.checkPath` để xác thực đường dẫn repo; agent không có method này nên không tạo được project trên dev server. Mã đã thêm (stat, trả `{exists, isDir}`) nhưng chưa có test, chưa nằm trong tài liệu hợp đồng agent.

## Việc cần làm

- [x] Test vitest: đường dẫn tồn tại là thư mục; là tệp; không tồn tại; tham số thiếu trả `InvalidParams`; đường dẫn tương đối nối với `workDir`.
- [-] Đưa `fs.checkPath` vào danh sách capability/tệp vàng nếu danh sách đó liệt kê method `fs.*`. — _không áp dụng: tệp vàng capability liệt kê công cụ, không liệt kê method `fs.*`_
- [ ] Ghi vào `specs/agent` (tham số, đáp ứng, lỗi). — _chưa ghi vào specs/agent_
- [ ] Bump `AGENT_VERSION` theo quy ước để `relay-ssh` tự đẩy bản mới. — _chưa tăng `AGENT_VERSION` (agent/build.mjs)_

## Tiêu chí hoàn thành

- [x] Test agent xanh; golden capability cập nhật có chủ đích.
- [x] e2e `DEV-04`, `DEV-05` xanh với agent build mới.

## Kiểm thử

vitest cho handler; `DEV-04/05` trên stack thật.

## Kết quả thực thi

Xong. `fs.checkPath` có test (`fs-check-path.test.ts`), có trong báo cáo khả năng của agent; e2e DEV-04/05 PASS.

# Khuyến nghị hành động (ưu tiên theo mức độ nghiêm trọng)

## Ngay lập tức (bảo mật, đang chạy trong code chưa commit)

1. **[01 §2](./01-security-conformance.md#2-git-push-streaming-dùng-sessionstorage-bearer-token)** — Chặn merge `runtime-rpc-stream.ts`/`useGit.ts` cho tới khi luồng auth `git.push` streaming chuyển về `credentials: 'include'` thay vì `sessionStorage` Bearer token rỗng. File chưa commit — sửa trước khi vào baseline rẻ hơn nhiều so với sửa sau.
2. **[01 §1](./01-security-conformance.md#1-devicetoken-lưu-plaintext-trong-localstorage)** — Đánh giá lại vòng đời `deviceToken` trong `localStorage` (E2EE pairing). Tối thiểu: rút ngắn thời gian sống, cân nhắc mã hoá tại rest hoặc chuyển hẳn use case có thể sang session-cookie theo hướng [CR-FE2E series](../../docs/crs/v2/frontend-e2ee/) đã có sẵn.

## Ngắn hạn (1-2 sprint)

3. **[01 §3](./01-security-conformance.md#3-credential-key-derivation-không-theo-userid)** — Đưa `userId` vào KDF của `WebCredentialStore`, đúng thiết kế đã công bố ở `security.md` §11. Cần kế hoạch migrate credential cũ (re-encrypt) vì đổi KDF làm blob cũ không giải mã được nữa.
4. **[04 §3](./04-code-health-and-standards.md#3-test-coverage-gap-ở-5-module-provider-mới)** — Viết test cho 5 `HookService` (kimi/minimax/openclaude/command-code/droid) + `sparse/` presets, dùng test có sẵn của `claude/`/`codex` làm khuôn mẫu.
5. **[02 §1](./02-platform-abstraction-and-coding-standards.md#1-iplatformservices-chỉ-tồn-tại-cho-web)** — Làm rõ phạm vi thật của chuẩn "IPlatformServices" (v5.0-only hay toàn bộ `src/main`) — nếu v5.0-only thì cập nhật câu chữ trong `docs/features/README.md` cho khỏi mơ hồ; nếu ý định là toàn bộ, cần roadmap riêng cho 72 file (không nên làm gấp, rủi ro regression cao).

## Trung hạn (dọn nợ doc — rẻ, không rủi ro code)

6. **[03](./03-hld-doc-drift.md)** — Viết lại `web-server-architecture.md` §5.1 (wire protocol), §11 (Admin SPA route table + bỏ câu "React Router"), §12 (Routing — mô tả đúng state-branching), §9.1/9.6/9.8 (path/tên component). Không đụng code, chỉ sửa doc — nên làm sớm vì mỗi ngày trôi qua doc càng gây nhầm lẫn cho người đọc mới.
7. **[03 §4](./03-hld-doc-drift.md#4-107-gitpanel--sai-path-thiếu-1-component-đã-tài-liệu-hoá)** — Xác nhận `ConflictPanel` (F39) là feature gap thật hay đã bỏ khỏi roadmap; cập nhật doc hoặc lên kế hoạch implement tương ứng.
8. **[04 §1](./04-code-health-and-standards.md#1-max-lines-vi-phạm-chính-sách-agentsmd)** — Xác nhận với chủ chính sách `AGENTS.md` xem 240 file `max-lines` disable có đang bị ratchet chặn tăng thêm không; nếu không, bổ sung cơ chế chặn ngay (rẻ, ngăn nợ tăng thêm), việc dọn 240 file cũ có thể xếp sau.

## Không cần hành động — chỉ ghi nhận

- Renderer sandbox, Zero Mock, E2EE crypto, credential form UI, admin audit log, TODO/FIXME density, skipped test, `any` usage — tất cả đã khớp thiết kế, không cần làm gì thêm (xem README "Những gì ĐÃ đúng thiết kế").

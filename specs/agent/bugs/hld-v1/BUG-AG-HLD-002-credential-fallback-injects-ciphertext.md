# BUG-AG-HLD-002 — Credential fallback (không có `resolvedApiKey`) inject ciphertext Layer-1 thẳng vào biến env API key

**Mức độ:** 🔴 Critical
**Status:** 🔴 Open
**Module:** `agent/src/relay/agent-spawner.ts` (`buildAgentEnv`)
**Phát hiện:** 2026-08-08 (audit `agent/` code vs thiết kế — mảng "AI Provider Credential Relay")

---

## Mô tả

Khi spawn AI agent, `buildAgentEnv()` set API key theo 2 nhánh:

1. **Có `resolvedApiKey`** (Orca Server đã inject plaintext qua spawn params): set thẳng vào `ANTHROPIC_API_KEY`/`OPENAI_API_KEY`/`GEMINI_API_KEY` (`agent-spawner.ts:227-230`) — hoạt động đúng.
2. **Không có `resolvedApiKey`** (fallback, dòng 231-243): gọi `readDecryptedKey()` — hàm này chỉ gỡ lớp mã hoá **ngoài** (Layer 2, do agent tự bọc khi ghi credential) và trả về nguyên `encryptedBlob` (Layer 1 — **vẫn còn mã hoá bởi SubtleCrypto phía browser**, chưa từng được giải mã ở bất kỳ đâu trong `agent/`). Chuỗi ciphertext này bị set thẳng vào biến env API key (dòng 240).

Code tự cảnh báo chính lỗi này trong comment: *"buildAgentEnv: injecting Layer1 blob for … — agent may fail auth if key not plaintext"* (dòng 242) — tức đội phát triển đã biết nhánh này không hoạt động đúng nhưng chưa fix.

## Hậu quả

- Bất kỳ agent nào được spawn qua nhánh fallback (không có `resolvedApiKey` từ Orca Server) sẽ nhận một API key **không hợp lệ** (ciphertext thay vì plaintext) → AI CLI (claude/codex/gemini) báo lỗi authentication ngay khi khởi động, dù credential đã được lưu đúng.
- Đây là lỗi triển khai thật (không phải chỉ lệch tài liệu) — nhánh này về bản chất không thể hoạt động vì không có bước giải mã Layer-1 nào tồn tại trong `agent/`.

## Bằng chứng

```
agent-spawner.ts:172-181  → comment: Orca Server chịu trách nhiệm inject resolvedApiKey plaintext
agent-spawner.ts:225-230  → nhánh có resolvedApiKey: set thẳng plaintext (đúng)
agent-spawner.ts:231-243  → nhánh fallback: readDecryptedKey() trả Layer-1 ciphertext, set thẳng vào env (sai)
agent-spawner.ts:242      → comment tự thừa nhận "agent may fail auth if key not plaintext"
```

## Đề xuất fix

Một trong hai hướng:
- **(a)** Hoàn thiện giải mã Layer-1 phía agent — cần trao đổi với đội bảo mật vì hiện tại việc giải mã Layer-1 được thiết kế để chỉ xảy ra ở nơi giữ session key (theo mô hình double-encryption); nếu agent tự giải mã được Layer-1, cần xác nhận lại điều đó có phá vỡ mô hình bảo mật "Orca Server không thấy plaintext" hay không.
- **(b)** Nếu `resolvedApiKey` là con đường bắt buộc duy nhất hợp lệ, xoá hẳn nhánh fallback và trả lỗi rõ ràng ("credential chưa sẵn sàng, cần Orca Server cung cấp resolvedApiKey") thay vì âm thầm set một giá trị chắc chắn sai.

## Tham khảo

- Audit: `audit/agent/credential-fswatch-telemetry-vs-design-review.md` §2.1
- Liên quan: BUG-AG-HLD-001

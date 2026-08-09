# BUG-AG-HLD-012 — `ai-provider-handler.ts` là dead code, tự claim mã hoá AES-256-GCM nhưng thân hàm ghi JSON thô không mã hoá

**Mức độ:** 🟢 Low (latent risk — sẽ thành Critical nếu bị wire lại nhầm)
**Status:** 🔴 Open
**Module:** `agent/src/relay/ai-provider-handler.ts`
**Phát hiện:** 2026-08-08 (audit `agent/` code vs thiết kế — mảng "AI Provider Credential Relay")

---

## Mô tả

`ai-provider-handler.ts` là một implementation credential-store song song với `agent-credential-store.ts` (bản đang chạy thật). Xác nhận bằng `gitnexus impact({target:"aiProviderHandlers", direction:"upstream"})`: **0 caller** trong cả `agent/` lẫn `desktop/` — không có nơi nào import file này.

Điểm đáng lo ngại: comment đầu file (`ai-provider-handler.ts:1-9`) tự nhận thực hiện mã hoá AES-256-GCM at-rest, nhưng thân hàm bên dưới **chỉ ghi thẳng** `{encryptedBlob, iv, updatedAt}` ra file JSON — không có bước mã hoá bổ sung nào ở tầng agent (khác với `agent-credential-store.ts` thật, vốn double-encrypt đúng bằng AES-256-GCM riêng).

## Hậu quả

- **Hiện tại**: không có hậu quả runtime vì code chết, không được gọi.
- **Rủi ro tiềm ẩn**: nếu trong tương lai một ai đó (nhầm lẫn vì thấy path `~/.orca/ai-providers/` khớp đúng với tài liệu F35/BL-AIP-01 hơn `agent-credential-store.ts`) wire lại file này vào dispatcher thay cho bản đang chạy, hệ thống sẽ ghi credential blob gần như plaintext ra đĩa trong khi docblock khiến người review tưởng nó đã được mã hoá đúng chuẩn — một lỗ hổng bảo mật nghiêm trọng bị che giấu bởi comment sai.

## Bằng chứng

```
agent/src/relay/ai-provider-handler.ts:17 → PROVIDER_STORE_DIR khớp đúng path tài liệu (~/.orca/ai-providers/)
agent/src/relay/ai-provider-handler.ts:1-9 → comment claim AES-256-GCM at-rest
gitnexus impact(aiProviderHandlers, upstream) → impactedCount: 0 (cả agent/ và desktop/)
```

## Đề xuất fix

Xoá `ai-provider-handler.ts` hoàn toàn (dead code, đồng thời có comment sai lệch nguy hiểm) — không hợp nhất, vì `agent-credential-store.ts` đã là bản đúng và đang chạy thật. Nếu có lý do giữ lại, tối thiểu phải sửa comment để không claim sai về mã hoá, và thêm cảnh báo rõ "KHÔNG DÙNG — dead code, xem `agent-credential-store.ts`".

## Tham khảo

- Audit: `audit/agent/credential-fswatch-telemetry-vs-design-review.md` §2.1
- Liên quan: BUG-AG-HLD-001, BUG-AG-HLD-002

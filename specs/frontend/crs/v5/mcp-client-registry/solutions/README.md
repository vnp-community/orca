# frontend Solutions — MCP Client Registry (v5)

**CR:** [CR-MCP-014](../../../../../../docs/crs/v5/mcp-client-registry/CR-MCP-014-external-mcp-server-registry.md) · **Hợp đồng:** [CONTRACT-mcp-ui-api.md](../../../../backend-go/crs/v5/CONTRACT-mcp-ui-api.md) · **Quy ước FE v5:** [../../README.md](../../README.md) · **Phía BE:** [BE-MCP-SOL-014](../../../../../backend-go/crs/v5/mcp-client-registry/solutions/BE-MCP-SOL-014-external-mcp-server-registry.md)

## Solutions

| Solution | CR | Area | Effort | Status |
|---|---|---|---|---|
| [FE-MCP-SOL-011](./FE-MCP-SOL-011-external-mcp-servers-ui.md) | CR-MCP-014 (phần UI) | Tab `external-servers` trong Settings > MCP (`components/settings/mcp/*`), sub-namespace `window.api.mcp.externalServer`, type bổ sung `shared/mcp-external-server-types.ts` | Medium (4–5 ngày) | 🔲 Designed — chưa implement |

## Re-verify (CR/TDD vs mã frontend thật, 2026-10-01)

| Khẳng định | Kết quả | Hệ quả |
|---|---|---|
| "Secret nhập qua cùng cơ chế CredentialInput của AI Provider" | Cơ chế thật: `ai-provider/CredentialInput.tsx` + `lib/credential-crypto.ts` `encryptCredential` → `{encryptedBlob, iv}`; `ProviderForm` gọi `aiProvider.writeCredential`. Component **không tái dùng được nguyên xi** (prop `AIProviderType`, ngưỡng 10 ký tự, hook sau early return) | **D1:** không dùng; `McpExternalServerSecretField` (uncontrolled input) gửi plaintext qua WS/TLS |
| Envelope được mã hoá bằng khoá phiên | `auth` slice không có `sessionToken` ⇒ fallback `'fallback-dev-token'` (khoá công khai); BE broker coi envelope là opaque | **D1:** bỏ phong bì cho MCP; UI không được tuyên bố "mã hoá đầu cuối", copy "Sent over TLS and encrypted at rest by the server"; điểm yếu AI-provider là follow-up riêng |
| `Settings > MCP` tồn tại | Chưa: chỉ có `McpConfigSection` theo repo (`RepositoryPane.tsx:393`); section `mcp` do FE-MCP-SOL-001 tạo | Solution này gắn tab vào pane của FE-001, **không** sửa `McpConfigSection` |
| Có `ProfileSourceBadge` để hiển thị lớp profile | Component có (`company/dept/user/concat`) nhưng API không trả nguồn cho server MCP (`GetResolvedProfile` chỉ trả JSON đã gộp) và `McpExternalServer` không có trường tương ứng | Không hiển thị ở v1; ghi là tương lai |
| Primitive UI | Không có `switch/alert/alert-dialog`; badge variant không có `warning` | Dùng `div` token + icon; `outline/secondary/dot/destructive` |
| Gọi backend | `callRuntimeResult` qua `createAdminApi()` (`web/web-preload-api.ts:2668`) là mẫu; `admin.listTeams` có (`:2696`) | Thêm `externalServer` vào `createMcpApi()` của FE-001 |
| STYLEGUIDE | Thực tế ở `/opt/repos/orca/guides/STYLEGUIDE.md` (README FE v5 ghi `guides/STYLEGUIDE.md` — đúng nếu tính từ gốc repo) | — |

## Thứ tự & phụ thuộc

```
FE-MCP-SOL-001 (mcp-types, window.api.mcp, mcp-slice, section `mcp`) ──► FE-MCP-SOL-011
BE-MCP-SOL-014 (6 kênh externalServer.*) ──► FE-MCP-SOL-011
CONTRACT `setSecret{…, value}` (D1: plaintext qua TLS)  ── đã chốt, không còn chặn
CONTRACT `probe.approvedTools?` (đã vào CONTRACT)       ── mở khoá diff; có phương án dự phòng không diff
```
Làm FE-001 trước (kiểu + API + section); FE-011 có thể dựng với mock `window.api.mcp.externalServer` khi BE chưa sẵn, nhưng phần secret chỉ hoàn tất khi BE-014 có test chứng minh giá trị không bị log/echo và broker đã có category `mcp_external_secret`.

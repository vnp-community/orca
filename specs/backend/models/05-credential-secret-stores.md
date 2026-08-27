# Credential / Secret Stores — 5 cơ chế độc lập, không có "vault" trung tâm

Không giống dữ liệu nghiệp vụ (SQL / `orca-data.json`), secret được lưu theo **loại secret + runtime mode**,
mỗi loại 1 cơ chế mã hoá riêng. Không có bảng SQL nào chứa secret dạng plaintext hay mã hoá tập trung.

## 1. `WebCredentialStore` — server mode multi-user (`credentials/web-credential-store.ts`)

- Chỉ dùng khi `ORCA_MULTI_USER=1` (`credentials/index.ts` — `isWebCredentialMode()`).
- AES-256-GCM, per-user, file riêng cho từng service:
  `<userDataPath>/users/<userId>/credentials/<service>.enc`.
- Key mã hoá: `scryptSync(serverSecret, salt, 32)` — `serverSecret` từ env `ORCA_SERVER_SECRET`.
- Wire format V2: `magic(4) + salt(32) + iv(16) + authTag(16) + ciphertext`. V1 legacy (key cố định) vẫn decrypt
  được để tương thích ngược; có hàm re-encrypt V1→V2 tại startup (`TASK-RI-001`).
- Phạm vi: token tích hợp `bitbucket | azure-devops | gitea | linear | jira`.

## 2. Electron `safeStorage` — desktop mode (OS keychain qua Chromium)

Nhiều điểm dùng rải rác, không tập trung 1 module:

- `integration-credential-file.ts` — mã hoá/giải mã token tích hợp chung (`readStoredCredentialToken`).
- `persistence.ts` — mã hoá tại chỗ 3 field trong `orca-data.json`
  (`settings.opencodeSessionCookie`, `settings.httpProxyUrl`, `ui.browserKagiSessionLink` — xem
  [03](./03-electron-desktop-store.md) §5).
- `session-manager.ts`, `speech/openai-api-key-store.ts`, `minimax/minimax-cookie-store.ts`, `linear/client.ts`,
  `runtime/rpc/methods/credentials.ts` — mỗi module tự gọi `safeStorage` cho secret riêng của nó.

## 3. OS Keychain trực tiếp — Claude/Codex CLI OAuth (`desktop/src/main/claude-accounts/keychain.ts`)

Không qua `safeStorage` — gọi thẳng CLI `security` (macOS Keychain) qua `execFile`, service name
`"Claude Code-credentials"` / `"Orca Claude Code Managed Credentials"`. Lưu credential OAuth của CLI
Claude Code (và tương tự cho Codex ở `codex-accounts/`).

## 4. AI Provider key — encrypted blob qua relay (`ai-providers/AIProviderService.ts`)

- `orca_ai_provider_accounts` (SQL) chỉ chứa **metadata** — không bao giờ chứa key.
- Key thật: client mã hoá thành `{encryptedBlob, iv}` (`CredentialWriteRequest`,
  `shared/ai-provider-types.ts:67`), gửi qua RPC "relay" tới Dev Server / companion server
  (`ai-provider-rpc-handler.ts`) — **plaintext key không bao giờ đi qua chặng relay này** (comment `SECURITY`
  trong code).
- File thật lưu trên Dev Server: `${accountId}.enc`.

## 5. SSH passphrase — chỉ in-memory, không persist

`ipc/ssh-passphrase.ts` — flow request/response tương tác thuần in-memory (`pendingRequests` Map), hỏi UI
renderer nhập passphrase mỗi lần kết nối thay vì lưu lại. **Không có vault riêng cho SSH private key** —
`SshTarget.identityFile`/`identityAgent` chỉ lưu đường dẫn tới key/agent socket có sẵn trên máy (xem
[06](./06-shared-domain-types.md) §1).

## Tổng hợp theo runtime mode

| Loại secret | Desktop (Electron) | Server (`ORCA_MULTI_USER=1`) |
|---|---|---|
| Token tích hợp (Linear/Jira/Bitbucket/Azure DevOps/Gitea) | `safeStorage`, file riêng theo service (`integration-credential-file.ts`) | `WebCredentialStore`, AES-256-GCM per-user |
| Claude/Codex CLI OAuth | OS Keychain trực tiếp | *(không xác định trong khảo sát này — cần kiểm tra riêng nếu cần)* |
| AI provider API key | Client mã hoá → relay → Dev Server `.enc` | tương tự |
| SSH passphrase | In-memory-only, không persist | tương tự |
| SSH private key / identity | Không lưu — chỉ path tới file có sẵn | tương tự |
| Field nhạy cảm khác trong settings | `safeStorage` tại chỗ trong `orca-data.json` | N/A (không dùng `orca-data.json`) |

**Không có bảng SQL nào** trong danh mục [02](./02-sql-schema-catalog.md) chứa secret — kể cả
`orca_ai_provider_accounts` (chỉ metadata) và `orca_access_policies` (chỉ policy, không phải credential).

# TASK-BE-FLEET-007: Agent RPC `terraform.apply` — tái dùng `runRecipeCommand`

**Solution:** BE-FLEET-SOL-002 | **CR:** CR-FLEET-002
**Service:** `agent`
**Depends on:** Không (độc lập — có thể chạy song song TASK-BE-FLEET-006)
**Status:** ✅ DONE (2026-09-09)

> **Kết quả thực tế:** `runRecipeCommand`'s chữ ký thật (đọc qua
> `codegraph_explore`) khác đáng kể so với sketch trong task —
> `agent/src/shared/ephemeral-vm-recipe-process.ts` yêu cầu `repoPath`
> (không phải `cwd`), `mode: 'create'|'suspend'|'resume'|'destroy'` (bắt
> buộc), và `context: EphemeralVmRecipeContext` (bắt buộc `recipeId` +
> `repoPath`) — dùng làm nguồn cho các biến môi trường `ORCA_VM_MODE`/
> `ORCA_RECIPE_ID`/... Đã điều chỉnh `handleTerraformApply` dùng
> `mode: 'create'` (gần nghĩa nhất với "provision infra mới") và
> `context: { recipeId: 'terraform.apply', repoPath: params.workingDir }` —
> `recipeId` này chỉ phục vụ logging/env var, không tra cứu registry recipe
> thật nào.
>
> **Nơi đăng ký RPC:** tạo domain dispatcher riêng
> `agent/src/relay/agent-rpc-dispatch-terraform.ts` (mirror
> `agent-rpc-dispatch-vm.ts`'s convention 1-file-per-domain, KHÔNG nhét vào
> `agent-rpc-dispatch-vm.ts` vì `terraform.*` là domain khác `vm.*`), wire vào
> `route()` trong `agent-rpc-dispatch.ts` (import + gọi tuần tự, giống mọi
> domain khác — thêm sau `dispatchVmRpc`, trước `dispatchHiddenTargetRpc`).
> `terraform.apply` là request/response thường (KHÔNG streaming) — apply +
> `terraform output -json` chạy xong trước khi handler resolve, khác hẳn
> `vm.provision`'s fire-and-forget + `stream.chunk`/`stream.end`.
>
> **Bảo mật:** handler KHÔNG tự đọc credential từ đâu cả — `params.env` chỉ
> pass-through (hiện tại luôn `undefined` vì `TerraformRunner`,
> TASK-BE-FLEET-006, chưa có cách gửi credential — bị chặn bởi
> TASK-BE-FLEET-009 chưa được security review). Ghi rõ trong doc comment của
> `agent-terraform-handler.ts`.
>
> **Capability probe `terraform version`:** GHI NHẬN LÀ GAP, KHÔNG tự viết —
> đúng như task's câu "ghi nhận là gap cần 1 task riêng... KHÔNG tự viết 1
> cache mới trong task này nếu vượt phạm vi ước lượng effort ban đầu". Agent
> hiện chưa có `GitCapabilityCache`-tương-đương cho binary bên thứ 3
> (terraform/tofu) — cần 1 task riêng nếu muốn thêm.
>
> **Build/test thật đã chạy**: `npx vitest run
> src/relay/agent-terraform-handler.test.ts` — 9/9 PASS. `npx vitest run
> src/relay/agent-rpc-dispatch-vm.test.ts src/relay/agent-terraform-handler.test.ts`
> — 20/20 PASS (không phá test `vm.*` sẵn có). `npx tsc --noEmit -p .` — có
> lỗi TS nhưng TOÀN BỘ đều ở file KHÔNG liên quan (đã xác nhận bằng grep tên
> file lỗi — không file nào là `agent-terraform-handler.ts`/
> `agent-rpc-dispatch-terraform.ts`/`agent-rpc-dispatch.ts`), là lỗi tiền tồn
> tại (thiếu field `AgentConfig`/`AgentBinarySpec` ở nhiều file test khác,
> `OutboundSshSession` thiếu `hostKeyFingerprint`, v.v. — không do task này
> gây ra).

---

## ⚠️ Cảnh báo bảo mật — đọc trước khi implement

`terraform apply` cần credential cloud provider trong environment tiến trình. Task này **không** giải quyết
việc credential đến từ đâu — tham số `env`/credential injection nếu cần phải để dạng placeholder/TODO rõ ràng,
trỏ về TASK-BE-FLEET-009 (chưa sẵn sàng implement, cần security review trước). **Không** tự ý đọc credential
từ 1 file cấu hình local hay biến môi trường cố định của agent process — đó là chính xác kiểu thiết kế CR gốc
cảnh báo "Cao".

## Mục tiêu

Thêm RPC handler `terraform.apply` ở agent, tái dùng `runRecipeCommand`
(`agent/src/shared/ephemeral-vm-recipe-process.ts:23`) đã có cho `vm.provision` — KHÔNG viết process runner
mới.

## Files cần sửa

1. `agent/src/relay/agent-terraform-handler.ts` (MỚI)
2. `agent/src/relay/agent-terraform-handler.test.ts` (MỚI)
3. File dispatch RPC chính của agent (nơi `vm.provision`/`git.commit` được đăng ký — xác nhận file thật, ứng
   viên là nơi `agent-ephemeral-vm-handler.ts`'s `handleVmProvision` được wire vào dispatcher; đọc lại
   `agent-rpc-dispatch-git.ts` hoặc dispatcher tương đương cho `vm.*` methods trước khi quyết định nơi đăng ký
   `terraform.apply`) (MODIFY — đăng ký handler mới)

## Đọc kỹ trước khi code

```ts
// agent/src/shared/ephemeral-vm-recipe-process.ts:23
export async function runRecipeCommand(args: {
  // đọc đầy đủ chữ ký thật ở đây trước khi gọi — KHÔNG đoán tham số.
  // agent-ephemeral-vm-handler.ts:163 gọi hàm này cho vm.provision, dùng
  // làm ví dụ call site thật.
}): Promise</* đọc kiểu trả về thật */> { ... }
```

## `agent-terraform-handler.ts`

```ts
// agent/src/relay/agent-terraform-handler.ts
// Reuses runRecipeCommand (ephemeral-vm-recipe-process.ts) — same process
// runner agent-ephemeral-vm-handler.ts's handleVmProvision already uses.
// terraform apply/output are both "run 1 external binary, capture
// stdout/stderr/exit code" — the same shape runRecipeCommand already
// solves, no new runner needed (CR-FLEET-002 §"Changes Required").
import { runRecipeCommand } from '../shared/ephemeral-vm-recipe-process'

export interface TerraformApplyParams {
  workingDir: string
  varsFile?: string
}

export interface TerraformApplyResult {
  outputJson: string
}

export async function handleTerraformApply(params: TerraformApplyParams): Promise<TerraformApplyResult> {
  const applyArgs = params.varsFile
    ? `-var-file=${params.varsFile} -auto-approve`
    : '-auto-approve'

  const applyResult = await runRecipeCommand({
    // Thay đúng field name/shape theo chữ ký thật đọc được ở trên —
    // placeholder dưới đây minh hoạ Ý ĐỊNH, không phải contract khoá cứng.
    command: `terraform apply ${applyArgs}`,
    cwd: params.workingDir,
  })
  if (applyResult.exitCode !== 0) {
    throw new Error(`terraform apply failed: ${applyResult.stderr}`)
  }

  const outputResult = await runRecipeCommand({
    command: 'terraform output -json',
    cwd: params.workingDir,
  })
  if (outputResult.exitCode !== 0) {
    throw new Error(`terraform output failed: ${outputResult.stderr}`)
  }

  return { outputJson: outputResult.stdout }
}
```

**Capability probe `terraform`/`tofu` version — bắt buộc trước lần `apply` đầu tiên trên 1 control host, theo
nguyên tắc `docs/reference/git-compatibility.md` áp dụng tương tự cho binary bên thứ 3 không kiểm soát version.**
Thêm 1 bước `runRecipeCommand({ command: 'terraform version' })` trước `apply`, cache kết quả theo host (nếu
agent đã có 1 cơ chế cache tương tự `GitCapabilityCache` cho binary khác, tái dùng — nếu chưa, ghi nhận là gap
cần 1 task riêng, KHÔNG tự viết 1 cache mới trong task này nếu vượt phạm vi ước lượng effort ban đầu).

## Test cases cần cover

- `TestHandleTerraformApply_Success_ReturnsOutputJson` — mock `runRecipeCommand` trả `exitCode: 0` cho cả 2
  lệnh, xác nhận `outputJson` khớp `outputResult.stdout`.
- `TestHandleTerraformApply_ApplyFails_ThrowsWithStderr` — `apply`'s `exitCode !== 0` → throw, không gọi
  `terraform output`.
- `TestHandleTerraformApply_OutputFails_ThrowsWithStderr` — `apply` OK nhưng `output` fail → throw.
- `TestHandleTerraformApply_NoVarsFile_OmitsVarFileFlag` — `varsFile` undefined → command không có
  `-var-file=`.

## Verify

```bash
cd agent && npx vitest run src/relay/agent-terraform-handler.test.ts
npx tsc --noEmit -p .
```

## gitnexus

`codegraph_explore "runRecipeCommand"` trước khi thêm call site mới — xác nhận chữ ký tham số/kiểu trả về
thật, không đoán từ ví dụ `handleVmProvision`. Nếu repo cũng có GitNexus index cho `agent/`, chạy
`impact({target: "runRecipeCommand", direction: "upstream"})` trước khi thêm caller mới (dùng chung 1 runner
với nhiều recipe khác — CR-FLEET-002 đã ghi rõ cần bước này).

## Blocking

TASK-BE-FLEET-006's `TerraformRunner` implementation thật (không phải interface — interface độc lập) phụ
thuộc RPC này tồn tại để gọi qua agent client.

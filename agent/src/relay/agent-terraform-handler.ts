// src/relay/agent-terraform-handler.ts
// Reuses runRecipeCommand (shared/ephemeral-vm-recipe-process.ts) — same
// process runner agent-ephemeral-vm-handler.ts's handleVmProvision already
// uses for `vm.provision`. `terraform apply`/`terraform output` are both
// "run 1 external binary, capture stdout/stderr/exit code" — the same shape
// runRecipeCommand already solves, no new runner needed (CR-FLEET-002
// §"Changes Required").
//
// SECURITY: this handler does NOT source cloud-provider credentials from
// anywhere (no local config file, no fixed agent-process env var). Until
// TASK-BE-FLEET-009's credential-storage design is reviewed, credentials
// must already be present in the agent process's own environment (e.g.
// operator-provisioned) — `params.env` is a pass-through only, never a
// place this handler resolves secrets itself.
import { runRecipeCommand } from '../shared/ephemeral-vm-recipe-process'
import type { EphemeralVmRecipeContext } from '../shared/ephemeral-vm-recipe-runner'

export type TerraformApplyParams = {
  workingDir: string
  varsFile?: string
  // Pass-through only — see the SECURITY note above. Never populated by this
  // handler; the caller (TerraformRunner, TASK-BE-FLEET-006) decides what (if
  // anything) goes here, and today that is nothing until TASK-BE-FLEET-009
  // lands.
  env?: NodeJS.ProcessEnv
}

export type TerraformApplyResult = {
  outputJson: string
}

// buildContext gives runRecipeCommand the EphemeralVmRecipeContext shape it
// requires (recipeId/repoPath) even though terraform.apply isn't a VM-recipe
// lifecycle call — recipeId is used only for the ORCA_RECIPE_ID env var and
// logging, not looked up against a real recipe registry.
function buildContext(workingDir: string): EphemeralVmRecipeContext {
  return { recipeId: 'terraform.apply', repoPath: workingDir }
}

export async function handleTerraformApply(
  params: TerraformApplyParams
): Promise<TerraformApplyResult> {
  const applyArgs = params.varsFile ? `-var-file=${params.varsFile} -auto-approve` : '-auto-approve'
  const context = buildContext(params.workingDir)

  const applyResult = await runRecipeCommand({
    command: `terraform apply ${applyArgs}`,
    repoPath: params.workingDir,
    mode: 'create',
    context,
    env: params.env
  })
  if (applyResult.exitCode !== 0) {
    throw new Error(`terraform apply failed: ${applyResult.stderr}`)
  }

  const outputResult = await runRecipeCommand({
    command: 'terraform output -json',
    repoPath: params.workingDir,
    mode: 'create',
    context,
    env: params.env
  })
  if (outputResult.exitCode !== 0) {
    throw new Error(`terraform output failed: ${outputResult.stderr}`)
  }

  return { outputJson: outputResult.stdout }
}

// validateTerraformApplyParams mirrors the other domain dispatchers'
// validateXParams helpers (e.g. validateVmProvisionParams in
// agent-ephemeral-vm-handler.ts) — a minimal runtime guard on the params
// object the JSON-RPC layer hands over as `unknown`.
export function validateTerraformApplyParams(params: unknown): TerraformApplyParams {
  if (!params || typeof params !== 'object') {
    throw new Error('terraform.apply requires params')
  }
  const p = params as Record<string, unknown>
  if (typeof p['workingDir'] !== 'string' || p['workingDir'] === '') {
    throw new Error('terraform.apply requires a non-empty workingDir')
  }
  if (p['varsFile'] !== undefined && typeof p['varsFile'] !== 'string') {
    throw new Error('terraform.apply: varsFile must be a string when provided')
  }
  return {
    workingDir: p['workingDir'],
    varsFile: typeof p['varsFile'] === 'string' ? p['varsFile'] : undefined
  }
}

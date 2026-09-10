import { describe, it, expect, vi, beforeEach } from 'vitest'

const runRecipeCommand = vi.fn()

vi.mock('../shared/ephemeral-vm-recipe-process', () => ({
  runRecipeCommand: (...args: unknown[]) => runRecipeCommand(...args)
}))

beforeEach(() => {
  runRecipeCommand.mockReset()
})

describe('handleTerraformApply', () => {
  it('returns outputJson from terraform output -json when apply succeeds', async () => {
    const { handleTerraformApply } = await import('./agent-terraform-handler')
    runRecipeCommand
      .mockResolvedValueOnce({ stdout: '', stderr: '', exitCode: 0, signal: null })
      .mockResolvedValueOnce({
        stdout: '{"instance_ip":{"value":"1.2.3.4"}}',
        stderr: '',
        exitCode: 0,
        signal: null
      })

    const result = await handleTerraformApply({ workingDir: '/tmp/tf-work' })

    expect(result).toEqual({ outputJson: '{"instance_ip":{"value":"1.2.3.4"}}' })
    expect(runRecipeCommand).toHaveBeenCalledTimes(2)
  })

  it('omits -var-file when varsFile is not provided', async () => {
    const { handleTerraformApply } = await import('./agent-terraform-handler')
    runRecipeCommand
      .mockResolvedValueOnce({ stdout: '', stderr: '', exitCode: 0, signal: null })
      .mockResolvedValueOnce({ stdout: '{}', stderr: '', exitCode: 0, signal: null })

    await handleTerraformApply({ workingDir: '/tmp/tf-work' })

    expect(runRecipeCommand).toHaveBeenNthCalledWith(
      1,
      expect.objectContaining({ command: 'terraform apply -auto-approve' })
    )
  })

  it('includes -var-file=<path> when varsFile is provided', async () => {
    const { handleTerraformApply } = await import('./agent-terraform-handler')
    runRecipeCommand
      .mockResolvedValueOnce({ stdout: '', stderr: '', exitCode: 0, signal: null })
      .mockResolvedValueOnce({ stdout: '{}', stderr: '', exitCode: 0, signal: null })

    await handleTerraformApply({ workingDir: '/tmp/tf-work', varsFile: 'prod.tfvars' })

    expect(runRecipeCommand).toHaveBeenNthCalledWith(
      1,
      expect.objectContaining({
        command: 'terraform apply -var-file=prod.tfvars -auto-approve'
      })
    )
  })

  it('throws with apply stderr and does not call terraform output when apply fails', async () => {
    const { handleTerraformApply } = await import('./agent-terraform-handler')
    runRecipeCommand.mockResolvedValueOnce({
      stdout: '',
      stderr: 'no credentials',
      exitCode: 1,
      signal: null
    })

    await expect(handleTerraformApply({ workingDir: '/tmp/tf-work' })).rejects.toThrow(
      'terraform apply failed: no credentials'
    )
    expect(runRecipeCommand).toHaveBeenCalledTimes(1)
  })

  it('throws with output stderr when apply succeeds but terraform output fails', async () => {
    const { handleTerraformApply } = await import('./agent-terraform-handler')
    runRecipeCommand
      .mockResolvedValueOnce({ stdout: '', stderr: '', exitCode: 0, signal: null })
      .mockResolvedValueOnce({ stdout: '', stderr: 'state locked', exitCode: 1, signal: null })

    await expect(handleTerraformApply({ workingDir: '/tmp/tf-work' })).rejects.toThrow(
      'terraform output failed: state locked'
    )
  })
})

describe('validateTerraformApplyParams', () => {
  it('returns validated params when workingDir is present', async () => {
    const { validateTerraformApplyParams } = await import('./agent-terraform-handler')
    expect(validateTerraformApplyParams({ workingDir: '/tmp/tf-work' })).toEqual({
      workingDir: '/tmp/tf-work',
      varsFile: undefined
    })
  })

  it('carries varsFile through when provided', async () => {
    const { validateTerraformApplyParams } = await import('./agent-terraform-handler')
    expect(
      validateTerraformApplyParams({ workingDir: '/tmp/tf-work', varsFile: 'prod.tfvars' })
    ).toEqual({ workingDir: '/tmp/tf-work', varsFile: 'prod.tfvars' })
  })

  it('throws when params is missing', async () => {
    const { validateTerraformApplyParams } = await import('./agent-terraform-handler')
    expect(() => validateTerraformApplyParams(undefined)).toThrow()
  })

  it('throws when workingDir is missing', async () => {
    const { validateTerraformApplyParams } = await import('./agent-terraform-handler')
    expect(() => validateTerraformApplyParams({})).toThrow()
  })
})

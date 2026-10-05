import { describe, expect, it, vi } from 'vitest'
import { createTaskForExternalIssue } from './external-issue-task'

describe('createTaskForExternalIssue', () => {
  it('calls task.createFromSource with the issue and project', async () => {
    const call = vi.fn().mockResolvedValue({})
    const ok = await createTaskForExternalIssue(call, {
      provider: 'jira',
      ref: 'ENG-1',
      title: 'ENG-1 fix login',
      url: 'https://x.atlassian.net/browse/ENG-1',
      taskProjectId: 'proj-1'
    })
    expect(ok).toBe(true)
    expect(call).toHaveBeenCalledWith('task.createFromSource', {
      title: 'ENG-1 fix login',
      projectId: 'proj-1',
      provider: 'jira',
      ref: 'ENG-1',
      url: 'https://x.atlassian.net/browse/ENG-1'
    })
  })

  it('forwards the Jira site to task.createFromSource', async () => {
    const call = vi.fn().mockResolvedValue({})
    await createTaskForExternalIssue(call, {
      provider: 'jira',
      ref: 'ENG-1',
      taskProjectId: 'p',
      site: 'site-1'
    })
    expect(call).toHaveBeenCalledWith(
      'task.createFromSource',
      expect.objectContaining({ site: 'site-1' })
    )
  })

  it('falls back to the key as title and skips without a project', async () => {
    const call = vi.fn().mockResolvedValue({})
    expect(await createTaskForExternalIssue(call, { provider: 'jira', ref: 'ENG-2' })).toBe(false)
    expect(call).not.toHaveBeenCalled()
    await createTaskForExternalIssue(call, { provider: 'jira', ref: 'ENG-2', taskProjectId: 'p' })
    expect(call.mock.calls[0][1]).toMatchObject({ title: 'ENG-2' })
  })

  it('never throws when the runtime call fails', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const call = vi.fn().mockRejectedValue(new Error('no task-service'))
    await expect(
      createTaskForExternalIssue(call, { provider: 'jira', ref: 'ENG-3', taskProjectId: 'p' })
    ).resolves.toBe(false)
    warn.mockRestore()
  })
})

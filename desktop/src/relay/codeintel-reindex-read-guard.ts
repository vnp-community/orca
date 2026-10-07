import fs from 'node:fs'
import { CodeIntelError } from './codeintel-errors'
import { CodeIntelRepoBinding } from './codeintel-repo-resolution'
import { activeJobs } from './codeintel-reindex-job'
import { closeCodeGraphDb } from './codegraph-sqlite-reader'

export function getActiveReindexJobForRepo(repoRoot: string, tool?: 'gitnexus' | 'codegraph') {
  const targetPath = fs.existsSync(repoRoot) ? fs.realpathSync(repoRoot) : repoRoot
  for (const job of activeJobs.values()) {
    if (job.state === 'running' || job.state === 'queued') {
      const jobPath = fs.existsSync(job.repoRoot) ? fs.realpathSync(job.repoRoot) : job.repoRoot
      if (jobPath === targetPath) {
        if (!tool || job.tools.includes(tool)) {
          return job
        }
      }
    }
  }
  return null
}

export function assertReadable(binding: CodeIntelRepoBinding, tool: 'gitnexus' | 'codegraph'): void {
  const job = getActiveReindexJobForRepo(binding.toplevel, tool)
  if (!job) {
    return
  }

  if (tool === 'gitnexus') {
    throw new CodeIntelError('CODEINTEL_REINDEX_IN_PROGRESS', 'GitNexus reindex in progress', {
      jobId: job.jobId,
      state: job.state,
      stage: (job as any).stage || 'running'
    })
  }

  if (tool === 'codegraph') {
    closeCodeGraphDb(binding.toplevel)
  }
}

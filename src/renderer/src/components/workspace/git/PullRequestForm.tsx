/**
 * PullRequestForm.tsx — GitHub PR creation form (TASK-045)
 *
 * Features:
 * - Title, body, base branch fields
 * - AI PR description generation
 * - Submit → git.pr.create RPC
 * - Shows PR URL after creation
 *
 * @module renderer/components/workspace/git/PullRequestForm
 */

import { useState, useCallback } from 'react'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../../runtime/runtime-rpc-client'
import { useAppStore } from '../../../store'
import { useWorkspace } from '../../../context/WorkspaceContext'

// ── Types ──────────────────────────────────────────────────────────────────────

interface PrCreateResult {
  url: string
  exitCode: number
}

// ── Styles ────────────────────────────────────────────────────────────────────

const S = {
  container: {
    display: 'flex',
    flexDirection: 'column' as const,
    gap: '12px',
    padding: '4px 0',
  },
  field: { display: 'flex', flexDirection: 'column' as const, gap: '4px' },
  label: {
    fontSize: '11px',
    fontWeight: 600,
    color: '#6c7086',
    textTransform: 'uppercase' as const,
    letterSpacing: '0.5px',
  },
  input: {
    padding: '7px 10px',
    background: '#1e1e2e',
    border: '1px solid #313244',
    borderRadius: '5px',
    color: '#cdd6f4',
    fontSize: '13px',
    fontFamily: 'inherit',
  },
  textarea: {
    padding: '7px 10px',
    background: '#1e1e2e',
    border: '1px solid #313244',
    borderRadius: '5px',
    color: '#cdd6f4',
    fontSize: '13px',
    fontFamily: 'inherit',
    minHeight: '120px',
    resize: 'vertical' as const,
  },
  buttonRow: { display: 'flex', gap: '8px', flexWrap: 'wrap' as const },
  btn: (variant: 'primary' | 'secondary') => ({
    padding: '7px 16px',
    borderRadius: '5px',
    border: 'none',
    cursor: 'pointer',
    fontWeight: 600,
    fontSize: '13px',
    background: variant === 'primary' ? '#89b4fa' : '#313244',
    color: variant === 'primary' ? '#1e1e2e' : '#cdd6f4',
    transition: 'opacity 0.15s',
  }),
  checkbox: { display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' },
  checkLabel: { color: '#cdd6f4', fontSize: '13px' },
  successBox: {
    padding: '12px',
    background: 'rgba(166,227,161,0.1)',
    borderRadius: '6px',
    border: '1px solid rgba(166,227,161,0.3)',
    display: 'flex',
    flexDirection: 'column' as const,
    gap: '6px',
  },
  prUrl: {
    color: '#89b4fa',
    textDecoration: 'underline',
    cursor: 'pointer',
    fontSize: '13px',
    wordBreak: 'break-all' as const,
  },
  error: { color: '#f38ba8', fontSize: '12px', padding: '2px 0' },
}

// ── PullRequestForm component ─────────────────────────────────────────────────

export function PullRequestForm({
  projectId,
  worktreePath,
  currentBranch = 'main',
}: {
  projectId: string
  worktreePath: string
  currentBranch?: string
}) {
  const [title, setTitle] = useState('')
  const [body, setBody] = useState('')
  const [base, setBase] = useState('main')
  const [draft, setDraft] = useState(false)
  const [isGenerating, setIsGenerating] = useState(false)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [prUrl, setPrUrl] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const { emit } = useWorkspace()

  const rpcTarget = useCallback(
    () => getActiveRuntimeTarget(useAppStore.getState().settings),
    []
  )

  // Generate AI PR description
  const handleGenerateDescription = useCallback(async () => {
    setIsGenerating(true)
    setError(null)
    try {
      // Get diff between current branch and base
      const diffResult = await callRuntimeRpc<{ stdout: string }>(rpcTarget(), 'git.diff', {
        projectId, worktreePath, staged: false
      })
      const diff = diffResult.stdout?.slice(0, 6000) ?? ''

      // Use git.generateCommitMessage as a proxy for AI description generation
      const result = await callRuntimeRpc<{ message: string }>(
        rpcTarget(), 'git.generateCommitMessage', {
          projectId, worktreePath, devServerId: 'default'
        }
      ).catch((): { message: string } => ({ message: '' }))

      // Build PR description from commit-message-style result
      if (!title) setTitle(result.message.split('\n')[0] ?? '')
      setBody([
        '## Summary',
        '',
        result.message,
        '',
        '## Changes',
        '',
        `Branch: \`${currentBranch}\` → \`${base}\``,
        '',
        diff ? `<details><summary>Diff preview</summary>\n\n\`\`\`diff\n${diff.slice(0, 2000)}\n\`\`\`\n</details>` : '',
      ].filter(l => l !== undefined).join('\n'))
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setIsGenerating(false)
    }
  }, [projectId, worktreePath, base, currentBranch, title, rpcTarget])

  // Submit PR
  const handleSubmit = useCallback(async () => {
    if (!title.trim()) {
      setError('PR title is required')
      return
    }
    setIsSubmitting(true)
    setError(null)
    try {
      const result = await callRuntimeRpc<PrCreateResult>(rpcTarget(), 'git.pr.create', {
        projectId,
        worktreePath,
        title: title.trim(),
        body: body.trim(),
        base,
        draft,
        head: currentBranch,
      })
      setPrUrl(result.url)
      emit('git.pr.created', { projectId, url: result.url, title })
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setIsSubmitting(false)
    }
  }, [title, body, base, draft, currentBranch, projectId, worktreePath, rpcTarget, emit])

  // Success state
  if (prUrl) {
    return (
      <div id="git-pr-success" style={S.successBox}>
        <div style={{ color: '#a6e3a1', fontWeight: 700 }}>✓ Pull Request Created</div>
        <a
          id="git-pr-url"
          href={prUrl}
          target="_blank"
          rel="noopener noreferrer"
          style={S.prUrl}
        >
          {prUrl}
        </a>
        <button
          type="button"
          style={S.btn('secondary')}
          onClick={() => { setPrUrl(null); setTitle(''); setBody('') }}
        >
          Create another
        </button>
      </div>
    )
  }

  return (
    <div id="git-pr-form" style={S.container}>
      {/* Title */}
      <div style={S.field}>
        <label htmlFor="git-pr-title" style={S.label}>Title</label>
        <input
          id="git-pr-title"
          type="text"
          style={S.input}
          placeholder="PR title…"
          value={title}
          onChange={e => setTitle(e.target.value)}
        />
      </div>

      {/* Body */}
      <div style={S.field}>
        <label htmlFor="git-pr-body" style={S.label}>Description</label>
        <textarea
          id="git-pr-body"
          style={S.textarea}
          placeholder="Describe your changes…"
          value={body}
          onChange={e => setBody(e.target.value)}
        />
      </div>

      {/* Base branch */}
      <div style={S.field}>
        <label htmlFor="git-pr-base" style={S.label}>Base Branch</label>
        <input
          id="git-pr-base"
          type="text"
          style={S.input}
          value={base}
          onChange={e => setBase(e.target.value)}
        />
      </div>

      {/* Draft checkbox */}
      <label style={S.checkbox} htmlFor="git-pr-draft">
        <input
          id="git-pr-draft"
          type="checkbox"
          checked={draft}
          onChange={e => setDraft(e.target.checked)}
        />
        <span style={S.checkLabel}>Create as draft</span>
      </label>

      {error && <div style={S.error}>⚠ {error}</div>}

      {/* Action buttons */}
      <div style={S.buttonRow}>
        <button
          id="git-ai-pr-description-btn"
          type="button"
          style={S.btn('secondary')}
          onClick={handleGenerateDescription}
          disabled={isGenerating}
        >
          {isGenerating ? '⟳ Generating…' : '✨ AI Description'}
        </button>
        <button
          id="git-pr-submit-btn"
          type="button"
          style={S.btn('primary')}
          onClick={handleSubmit}
          disabled={isSubmitting || !title.trim()}
        >
          {isSubmitting ? 'Creating…' : draft ? 'Create Draft PR' : 'Create Pull Request'}
        </button>
      </div>
    </div>
  )
}

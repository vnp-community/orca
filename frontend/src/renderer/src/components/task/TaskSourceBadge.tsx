import { useTaskSource } from '../../hooks/useTaskSource'

const PROVIDER_LABEL: Record<string, string> = {
  jira: 'Jira',
  linear: 'Linear',
  github: 'GitHub',
  gitlab: 'GitLab'
}

// Only http(s) URLs become links — the url comes from the backend and must not
// be able to inject a javascript: href.
function safeHref(url: string | undefined): string | null {
  if (!url) {
    return null
  }
  try {
    const parsed = new URL(url)
    return parsed.protocol === 'https:' || parsed.protocol === 'http:' ? parsed.href : null
  } catch {
    return null
  }
}

export function TaskSourceBadge({ taskId }: { taskId: string }): React.JSX.Element | null {
  const source = useTaskSource(taskId)
  if (!source) {
    return null
  }
  const label = `${PROVIDER_LABEL[source.provider] ?? source.provider} ${source.ref}`
  const href = safeHref(source.url)
  const className =
    'inline-flex items-center rounded-md border border-border px-2 py-0.5 text-xs text-muted-foreground'
  return href ? (
    <a
      href={href}
      target="_blank"
      rel="noreferrer noopener"
      className={`${className} hover:text-foreground`}
      data-testid="task-source-badge"
    >
      {label}
    </a>
  ) : (
    <span className={className} data-testid="task-source-badge">
      {label}
    </span>
  )
}

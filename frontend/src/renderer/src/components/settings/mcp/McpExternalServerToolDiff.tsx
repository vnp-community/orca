import { translate } from '@/i18n/i18n'
import { UntrustedToolText } from './UntrustedToolText'
import { hasToolChanges, type McpToolDiff } from './mcp-tool-diff'

function Group({
  title,
  children
}: {
  title: string
  children: React.ReactNode
}): React.JSX.Element {
  return (
    <section className="space-y-1">
      <h4 className="text-xs font-medium text-muted-foreground uppercase">{title}</h4>
      {children}
    </section>
  )
}

/** Plain-text diff against the last approved tool set; every string is untrusted. */
export function McpExternalServerToolDiff({ diff }: { diff: McpToolDiff }): React.JSX.Element {
  if (!hasToolChanges(diff)) {
    return (
      <p className="text-sm text-muted-foreground">
        {translate('auto.mcp.external.diff.none', 'No changes since the last approval.')}
      </p>
    )
  }
  return (
    <div className="space-y-3">
      {diff.added.length ? (
        <Group title={translate('auto.mcp.external.diff.added', 'Added')}>
          {diff.added.map((t) => (
            <div key={t.name} className="rounded-md border border-border p-2">
              <UntrustedToolText text={t.name} className="font-mono" />
              <UntrustedToolText text={t.description} />
            </div>
          ))}
        </Group>
      ) : null}
      {diff.removed.length ? (
        <Group title={translate('auto.mcp.external.diff.removed', 'Removed')}>
          {diff.removed.map((t) => (
            <div key={t.name} className="rounded-md border border-border p-2">
              <UntrustedToolText text={t.name} className="font-mono" />
            </div>
          ))}
        </Group>
      ) : null}
      {diff.changed.length ? (
        <Group title={translate('auto.mcp.external.diff.changed', 'Description changed')}>
          {diff.changed.map((t) => (
            <div key={t.name} className="space-y-1 rounded-md border border-border p-2">
              <UntrustedToolText text={t.name} className="font-mono" />
              <div className="grid gap-2 sm:grid-cols-2">
                <div>
                  <p className="text-xs text-muted-foreground">
                    {translate('auto.mcp.external.diff.before', 'Approved')}
                  </p>
                  <UntrustedToolText text={t.before} />
                </div>
                <div>
                  <p className="text-xs text-muted-foreground">
                    {translate('auto.mcp.external.diff.after', 'Now')}
                  </p>
                  <UntrustedToolText text={t.after} />
                </div>
              </div>
            </div>
          ))}
        </Group>
      ) : null}
    </div>
  )
}

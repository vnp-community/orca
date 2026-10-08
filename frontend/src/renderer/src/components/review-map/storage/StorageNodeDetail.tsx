/**
 * StorageNodeDetail.tsx — FE-CV-TASK-058-05
 *
 * Detail of one node. Secrets show only key name and Vault path; there is no way to reveal
 * or copy a value. Evidence offers "View diff" only for files in the change set.
 */

import { X } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import { STORAGE_MARK_SYMBOL, type StorageMark } from './storage-change-marks'
import { copyKeyName } from './StorageSecretNode'
import { normalizeStoragePath } from './storage-change-marks'
import type { StorageEdge, StorageNode } from './storage-view-model'

export type StorageNodeDetailProps = {
  node: StorageNode
  nodes: readonly StorageNode[]
  edges: readonly StorageEdge[]
  mark?: StorageMark
  changedPaths: ReadonlySet<string>
  onOpenDiff: (path: string, line?: number) => void
  onSelectNode: (id: string) => void
  onOpenErd: (service: string) => void
  onClose: () => void
}

const t = translate

export function StorageNodeDetail(p: StorageNodeDetailProps): React.JSX.Element {
  const { node } = p
  const byId = new Map(p.nodes.map((n) => [n.id, n]))
  const related = p.edges.filter((e) => e.from === node.id || e.to === node.id)
  const canOpenErd =
    node.lane === 'store' &&
    Boolean(node.owner) &&
    (node.kind === 'postgres' || node.kind === 'mysql')
  const neighbour = (e: StorageEdge): StorageNode | undefined =>
    byId.get(e.from === node.id ? e.to : e.from)
  return (
    <aside
      className="flex min-h-0 w-80 shrink-0 flex-col gap-3 overflow-y-auto border-l p-3 text-xs"
      aria-label={t('auto.components.reviewMap.StorageNodeDetail.label', 'Detail: {{name}}', {
        name: node.name
      })}
    >
      <div className="flex items-start gap-2">
        <h3 className="truncate text-sm font-semibold">{node.name}</h3>
        <Button
          type="button"
          variant="ghost"
          size="icon-xs"
          className="ml-auto"
          aria-label={t('auto.components.reviewMap.StorageNodeDetail.close', 'Close detail')}
          onClick={p.onClose}
        >
          <X className="size-3.5" aria-hidden="true" />
        </Button>
      </div>
      <div className="flex flex-wrap gap-1">
        <Badge variant="outline">{node.lane}</Badge>
        {node.kind ? <Badge variant="outline">{node.kind}</Badge> : null}
        {node.env ? <Badge variant="outline">{node.env}</Badge> : null}
        {p.mark ? (
          <Badge variant="secondary">
            <span aria-hidden="true">{STORAGE_MARK_SYMBOL[p.mark]}</span>{' '}
            {t(`auto.components.reviewMap.StorageCanvas.mark.${p.mark}`, p.mark)}
          </Badge>
        ) : null}
        {node.confidence === 'inferred' || node.confidence === 'derived' ? (
          <Badge variant="secondary">
            {t(
              `auto.components.reviewMap.StorageCanvas.confidence.${node.confidence}`,
              node.confidence
            )}
          </Badge>
        ) : null}
      </div>
      {node.lane === 'secret' ? (
        <section className="space-y-1">
          {node.vaultPath ? (
            <p>
              {t('auto.components.reviewMap.StorageNodeDetail.vaultPath', 'Vault path: {{path}}', {
                path: node.vaultPath
              })}
            </p>
          ) : null}
          <p className="text-muted-foreground">
            {t(
              'auto.components.reviewMap.StorageNodeDetail.neverShown',
              'The value is never shown or copied.'
            )}
          </p>
          <Button
            type="button"
            variant="outline"
            size="xs"
            onClick={() => void copyKeyName(node.name)}
          >
            {t('auto.components.reviewMap.StorageNodeDetail.copyKey', 'Copy key name')}
          </Button>
        </section>
      ) : (
        <dl className="grid grid-cols-[auto_1fr] gap-x-2 gap-y-0.5">
          {node.owner ? (
            <>
              <dt className="text-muted-foreground">
                {t('auto.components.reviewMap.StorageNodeDetail.owner', 'Owner')}
              </dt>
              <dd>{node.owner}</dd>
            </>
          ) : null}
          {node.schemas?.length ? (
            <>
              <dt className="text-muted-foreground">
                {t('auto.components.reviewMap.StorageNodeDetail.schemas', 'Schemas')}
              </dt>
              <dd>{node.schemas.join(', ')}</dd>
            </>
          ) : null}
          {node.delivery ? (
            <>
              <dt className="text-muted-foreground">
                {t('auto.components.reviewMap.StorageNodeDetail.delivery', 'Delivery')}
              </dt>
              <dd>{node.delivery}</dd>
            </>
          ) : null}
          {node.stream ? (
            <>
              <dt className="text-muted-foreground">
                {t('auto.components.reviewMap.StorageNodeDetail.stream', 'Stream')}
              </dt>
              <dd>{node.stream}</dd>
            </>
          ) : null}
          {node.payload ? (
            <>
              <dt className="text-muted-foreground">
                {t('auto.components.reviewMap.StorageNodeDetail.payload', 'Payload')}
              </dt>
              <dd>{node.payload}</dd>
            </>
          ) : null}
          {node.lane === 'store' ? (
            <>
              <dt className="text-muted-foreground">
                {t('auto.components.reviewMap.StorageNodeDetail.status', 'Status')}
              </dt>
              <dd>
                {[
                  node.deployed === false
                    ? t('auto.components.reviewMap.StorageCanvas.notDeployed', 'not deployed')
                    : t('auto.components.reviewMap.StorageNodeDetail.deployed', 'deployed'),
                  node.supportedByCode === false
                    ? t('auto.components.reviewMap.StorageCanvas.noCodeSupport', 'no code support')
                    : null,
                  node.external
                    ? t('auto.components.reviewMap.StorageCanvas.external', 'external')
                    : null
                ]
                  .filter(Boolean)
                  .join(' · ')}
              </dd>
            </>
          ) : null}
        </dl>
      )}
      {canOpenErd ? (
        <Button
          type="button"
          variant="outline"
          size="xs"
          className="w-fit"
          onClick={() => p.onOpenErd(node.owner as string)}
        >
          {t('auto.components.reviewMap.StorageNodeDetail.openErd', 'Open ERD of {{service}}', {
            service: node.owner
          })}
        </Button>
      ) : null}
      {related.length > 0 ? (
        <section>
          <h4 className="mb-1 font-medium">
            {t('auto.components.reviewMap.StorageNodeDetail.connections', 'Connections')}
          </h4>
          <ul className="space-y-0.5">
            {related.map((e) => {
              const other = neighbour(e)
              if (!other) {
                return null
              }
              return (
                <li key={e.id} className="flex items-center gap-1">
                  <span className="text-muted-foreground">
                    {e.kind === 'binding' ? e.access : e.kind}
                  </span>
                  <Button
                    type="button"
                    variant="link"
                    size="xs"
                    onClick={() => p.onSelectNode(other.id)}
                  >
                    {other.name}
                  </Button>
                  {e.via ? <span className="truncate text-muted-foreground">{e.via}</span> : null}
                </li>
              )
            })}
          </ul>
        </section>
      ) : null}
      {node.evidencePaths.length > 0 ? (
        <section>
          <h4 className="mb-1 font-medium">
            {t('auto.components.reviewMap.StorageNodeDetail.evidence', 'Evidence')}
          </h4>
          <ul>
            {node.evidencePaths.map((path) => {
              const changed = p.changedPaths.has(normalizeStoragePath(path))
              return (
                <li key={path} className="flex items-center gap-2">
                  <span className="truncate">{path}</span>
                  <Button
                    type="button"
                    variant="ghost"
                    size="xs"
                    className="ml-auto"
                    disabled={!changed}
                    onClick={() => p.onOpenDiff(path)}
                  >
                    {t('auto.components.reviewMap.StorageNodeDetail.viewDiff', 'View diff')}
                  </Button>
                </li>
              )
            })}
          </ul>
        </section>
      ) : null}
    </aside>
  )
}

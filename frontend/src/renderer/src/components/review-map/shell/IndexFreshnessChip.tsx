import { useState } from 'react'
import { Check, CircleHelp, CloudOff, Layers, Loader2, TriangleAlert, XCircle } from 'lucide-react'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { translate } from '@/i18n/i18n'
import { useNow } from '@/components/dashboard/useNow'
import type { IndexStatusView } from '../review-wire-types'
import { ReindexButton, type ReindexButtonProps } from './ReindexButton'

type ChipTone = 'ok' | 'warn' | 'muted' | 'busy' | 'error'

export type IndexChipModel = {
  overall: IndexStatusView['overall']
  tone: ChipTone
  label: string
  /** Extra sentence for the popover (OVERLAY explanation). */
  note: string | null
}

function shortSha(sha: string | undefined): string {
  return sha ? sha.slice(0, 7) : ''
}

function relativeTime(iso: string | undefined, now: number): string {
  if (!iso) {
    return ''
  }
  const t = Date.parse(iso)
  if (Number.isNaN(t)) {
    return ''
  }
  const minutes = Math.max(0, Math.round((now - t) / 60_000))
  const fmt = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' })
  if (minutes < 60) {
    return fmt.format(-minutes, 'minute')
  }
  if (minutes < 60 * 24) {
    return fmt.format(-Math.round(minutes / 60), 'hour')
  }
  return fmt.format(-Math.round(minutes / (60 * 24)), 'day')
}

/** Pure: one model per `overall`. `percent:null` never renders as a number. */
export function describeIndexChip(status: IndexStatusView | null, now: number): IndexChipModel {
  const overall = status?.overall ?? 'UNKNOWN'
  const tool = status?.tools.find((t) => t.indexedCommit) ?? status?.tools[0]
  const sha = shortSha(tool?.indexedCommit)
  const when = relativeTime(tool?.indexedAt, now)
  switch (overall) {
    case 'READY':
      return {
        overall,
        tone: 'ok',
        note: null,
        label:
          [sha, when].filter(Boolean).join(' · ') ||
          translate('auto.components.reviewMap.shell.index.ready', 'Index ready')
      }
    case 'STALE':
      return {
        overall,
        tone: 'muted',
        note: null,
        label: translate(
          'auto.components.reviewMap.shell.index.stale',
          'Index out of date {{sha}}',
          { sha }
        )
      }
    case 'OVERLAY':
      return {
        overall,
        tone: 'warn',
        label: translate(
          'auto.components.reviewMap.shell.index.overlay',
          'Main checkout index + diff'
        ),
        note: translate(
          'auto.components.reviewMap.shell.index.overlayNote',
          'Based on the main checkout index plus the diff; line numbers may be off.'
        )
      }
    case 'BUILDING': {
      const p = status?.activeJob?.percent
      return {
        overall,
        tone: 'busy',
        note: null,
        label:
          typeof p === 'number'
            ? translate(
                'auto.components.reviewMap.shell.index.buildingPercent',
                'Indexing {{percent}}%',
                { percent: Math.round(p) }
              )
            : translate('auto.components.reviewMap.shell.index.building', 'Indexing')
      }
    }
    case 'MISSING':
      return {
        overall,
        tone: 'warn',
        note: null,
        label: translate('auto.components.reviewMap.shell.index.missing', 'No index yet')
      }
    case 'NOT_INSTALLED':
      return {
        overall,
        tone: 'warn',
        note: null,
        label: translate('auto.components.reviewMap.shell.index.notInstalled', 'Tool not installed')
      }
    case 'DEGRADED':
      return {
        overall,
        tone: 'warn',
        note: null,
        label: translate('auto.components.reviewMap.shell.index.degraded', 'Index partly available')
      }
    case 'OFFLINE':
      return {
        overall,
        tone: 'error',
        note: null,
        label: translate('auto.components.reviewMap.shell.index.offline', 'Offline')
      }
    default:
      return {
        overall: 'UNKNOWN',
        tone: 'muted',
        note: null,
        label: translate('auto.components.reviewMap.shell.index.unknown', 'Unknown')
      }
  }
}

const TONE_CLASS: Record<ChipTone, string> = {
  ok: 'text-foreground',
  warn: 'text-quality-warning',
  muted: 'bg-muted text-muted-foreground',
  busy: 'text-muted-foreground',
  error: 'text-destructive'
}

function ChipIcon({ overall }: { overall: IndexChipModel['overall'] }): React.JSX.Element {
  const cls = 'size-3.5'
  switch (overall) {
    case 'READY':
      return <Check className={cls} aria-hidden />
    case 'STALE':
    case 'MISSING':
    case 'NOT_INSTALLED':
    case 'DEGRADED':
      return <TriangleAlert className={cls} aria-hidden />
    case 'OVERLAY':
      return <Layers className={cls} aria-hidden />
    case 'BUILDING':
      return <Loader2 className={`${cls} animate-spin motion-reduce:animate-none`} aria-hidden />
    case 'OFFLINE':
      return <CloudOff className={cls} aria-hidden />
    default:
      return <CircleHelp className={cls} aria-hidden />
  }
}

type Props = {
  status: IndexStatusView | null
  reindex: Omit<ReindexButtonProps, 'status'>
}

export function IndexFreshnessChip({ status, reindex }: Props): React.JSX.Element {
  const now = useNow(60_000)
  const [open, setOpen] = useState(false)
  const model = describeIndexChip(status, now)
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          data-overall={model.overall}
          aria-label={translate(
            'auto.components.reviewMap.shell.index.aria',
            'Index status: {{label}}',
            { label: model.label }
          )}
          className={`inline-flex h-7 items-center gap-1.5 rounded-md border px-2 text-xs ${TONE_CLASS[model.tone]}`}
        >
          <ChipIcon overall={model.overall} />
          <span>{model.label}</span>
        </button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-80 space-y-3 text-xs">
        {model.note ? <p>{model.note}</p> : null}
        {status?.errorCode ? (
          <p className="flex items-center gap-1 text-destructive">
            <XCircle className="size-3.5" aria-hidden />
            {status.errorCode}
          </p>
        ) : null}
        <ul className="space-y-2">
          {(status?.tools ?? []).map((t) => (
            <li key={t.tool} className="space-y-0.5">
              <div className="font-medium">
                {t.tool}
                {t.version ? <span className="ml-1 text-muted-foreground">{t.version}</span> : null}
              </div>
              <div className="text-muted-foreground">
                {t.state}
                {t.indexedAt ? ` · ${relativeTime(t.indexedAt, now)}` : ''}
              </div>
              {t.pendingChanges ? (
                <div className="text-muted-foreground">
                  +{t.pendingChanges.added} ~{t.pendingChanges.modified} -{t.pendingChanges.removed}
                </div>
              ) : null}
            </li>
          ))}
        </ul>
        {(status?.indexBasis ?? []).length > 0 ? (
          <ul className="space-y-0.5 text-muted-foreground">
            {status!.indexBasis.map((b) => (
              <li key={b.tool}>
                {b.tool}: {b.refreshState} · {b.indexPolicy}
              </li>
            ))}
          </ul>
        ) : null}
        <ReindexButton {...reindex} status={status} />
      </PopoverContent>
    </Popover>
  )
}

import { lazy, Suspense, type LazyExoticComponent } from 'react'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Skeleton } from '@/components/ui/skeleton'
import { translate } from '@/i18n/i18n'
import type {
  ReviewLensComponent,
  ReviewLensDefinition,
  ReviewLensProps
} from '../review-lens-registry'

// One lazy wrapper per `load` function so switching tabs does not remount the lens.
const lazyByLoader = new WeakMap<object, LazyExoticComponent<ReviewLensComponent>>()

function lazyLens(def: ReviewLensDefinition): LazyExoticComponent<ReviewLensComponent> | null {
  if (!def.load) {
    return null
  }
  let c = lazyByLoader.get(def.load)
  if (!c) {
    c = lazy(def.load)
    lazyByLoader.set(def.load, c)
  }
  return c
}

function LensFallback(): React.JSX.Element {
  return (
    <div className="space-y-2 p-4" role="status" aria-busy>
      <Skeleton className="h-6 w-1/3" />
      <Skeleton className="h-40 w-full" />
    </div>
  )
}

type Props = {
  lenses: readonly ReviewLensDefinition[]
  activeLensId: string | null
  onSelectLens: (id: string) => void
  lensProps: ReviewLensProps
}

export function ReviewLensTabs({
  lenses,
  activeLensId,
  onSelectLens,
  lensProps
}: Props): React.JSX.Element {
  const active = lenses.find((l) => l.id === activeLensId) ?? null
  const Lens = active ? lazyLens(active) : null
  return (
    <Tabs value={activeLensId ?? ''} onValueChange={onSelectLens} className="min-h-0 flex-1 gap-0">
      <TabsList variant="line" className="w-full justify-start overflow-x-auto">
        {lenses.map((l) => (
          <TabsTrigger key={l.id} value={l.id} data-lens={l.id}>
            {translate(l.labelKey, l.labelFallback)}
          </TabsTrigger>
        ))}
      </TabsList>
      <div
        role="tabpanel"
        className="min-h-0 flex-1 overflow-auto"
        data-lens-body={active?.id ?? ''}
      >
        {active && Lens ? (
          <Suspense fallback={<LensFallback />}>
            <Lens {...lensProps} />
          </Suspense>
        ) : active ? (
          <p className="p-6 text-sm text-muted-foreground" data-lens-placeholder={active.id}>
            {translate(
              'auto.components.reviewMap.shell.lens.unavailable',
              '{{lens}} is not available yet.',
              {
                lens: translate(active.labelKey, active.labelFallback)
              }
            )}
          </p>
        ) : null}
      </div>
    </Tabs>
  )
}

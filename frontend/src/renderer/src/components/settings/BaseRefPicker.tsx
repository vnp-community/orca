import { Button } from '../ui/button'
import { RepoRefPicker, type RepoRefPickerHeaderContext } from '../git-ref-picker/RepoRefPicker'
import { translate } from '@/i18n/i18n'

type BaseRefPickerProps = {
  repoId: string
  currentBaseRef?: string
  onSelect: (ref: string) => void
  onUsePrimary?: () => void
}

// Why a thin layer: Settings and Source Control keep their summary and "Use Primary" button, while the search, the
// default lookup and the list live in the shared RepoRefPicker (also used by the Request dialog).
export function BaseRefPicker({
  repoId,
  currentBaseRef,
  onSelect,
  onUsePrimary
}: BaseRefPickerProps): React.JSX.Element {
  const renderHeader = ({
    defaultBaseRef,
    remotesHint
  }: RepoRefPickerHeaderContext): React.JSX.Element => {
    // Why: never seed 'origin/main'; a repo without that ref would show a fabricated default.
    const effectiveBaseRef = currentBaseRef ?? defaultBaseRef
    return (
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <div className="text-sm font-medium text-foreground">
            {effectiveBaseRef ??
              translate('auto.components.settings.BaseRefPicker.ee110e1830', 'No default base ref')}
          </div>
          <p className="text-xs text-muted-foreground">
            {currentBaseRef
              ? translate(
                  'auto.components.settings.BaseRefPicker.2f3cda96f5',
                  'Pinned for this repo'
                )
              : defaultBaseRef
                ? translate(
                    'auto.components.settings.BaseRefPicker.086ce7f369',
                    'Following primary branch ({{value0}})',
                    { value0: defaultBaseRef }
                  )
                : translate(
                    'auto.components.settings.BaseRefPicker.9a14ec7400',
                    'Pick a base branch below'
                  )}
          </p>
          {/* Why: passive hint that fork workflows have other remotes worth searching (e.g. `upstream`). Host-agnostic and
              remote-name-agnostic by design; suppressed when remoteCount <= 1 or when the lookup failed.
              See docs/upstream-base-ref-design.md §4. */}
          {remotesHint}
        </div>
        {onUsePrimary && (
          <Button variant="outline" size="sm" onClick={onUsePrimary} disabled={!currentBaseRef}>
            {translate('auto.components.settings.BaseRefPicker.773a5687a3', 'Use Primary')}
          </Button>
        )}
      </div>
    )
  }

  return (
    <RepoRefPicker
      repoId={repoId}
      value={currentBaseRef}
      onSelect={onSelect}
      renderHeader={renderHeader}
    />
  )
}

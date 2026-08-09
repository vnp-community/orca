// Regression test for SOL-FE2E-004 (CR-FE2E-004): "Share this Orca server" must
// stay hidden for every web client (both the multi-user backend path and the
// bare Desktop-pair-code path share `isWebClient === true`) and visible only
// on Desktop. Settings.tsx wires `canGeneratePairingUrl={!isWebClient}` — this
// test protects that prop's effect directly on RuntimeEnvironmentsPane, so a
// future accidental change to either side is caught without needing to render
// the (much heavier) Settings.tsx tree.
import { describe, expect, it, vi } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import type { GlobalSettings } from '../../../../shared/types'
import { TooltipProvider } from '@/components/ui/tooltip'
import { RuntimeEnvironmentsPane } from './RuntimeEnvironmentsPane'

function settings(): GlobalSettings {
  return { activeRuntimeEnvironmentId: null } as GlobalSettings
}

describe('RuntimeEnvironmentsPane — share-link visibility (SOL-FE2E-004)', () => {
  it('hides "Share this Orca server" when canGeneratePairingUrl is false (web client)', () => {
    const html = renderToStaticMarkup(
      <TooltipProvider>
        <RuntimeEnvironmentsPane
          settings={settings()}
          switchRuntimeEnvironment={vi.fn()}
          canGeneratePairingUrl={false}
          allowLocalRuntime={false}
        />
      </TooltipProvider>
    )
    expect(html).not.toContain('Share this Orca server')
  })

  it('shows "Share this Orca server" when canGeneratePairingUrl is true (Desktop)', () => {
    const html = renderToStaticMarkup(
      <TooltipProvider>
        <RuntimeEnvironmentsPane
          settings={settings()}
          switchRuntimeEnvironment={vi.fn()}
          canGeneratePairingUrl={true}
          allowLocalRuntime={true}
        />
      </TooltipProvider>
    )
    expect(html).toContain('Share this Orca server')
  })
})

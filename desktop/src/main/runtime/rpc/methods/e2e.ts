import { defineMethod, type RpcMethod } from '../core'
import { createE2EConfig, type E2EConfig } from '../../../../shared/e2e-config'

// Why: mirrors desktop/src/preload/e2e-config.ts's `preloadE2EConfig`
// construction exactly (same env vars, same createE2EConfig call) so the RPC
// surface reports the same E2E config the preload bridge's `e2e.getConfig()`
// already exposes, without a second config shape to keep in sync.
export const E2E_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'e2e.getConfig',
    params: null,
    handler: (): E2EConfig => {
      return createE2EConfig({
        headless: process.env.ORCA_E2E_HEADLESS === '1',
        exposeStore: false,
        userDataDir: process.env.ORCA_E2E_USER_DATA_DIR ?? null,
        terminalParkingDelayMs: Number(process.env.ORCA_E2E_TERMINAL_PARKING_DELAY_MS) || null
      })
    }
  })
]

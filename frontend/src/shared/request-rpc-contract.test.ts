import { existsSync, readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { REQUEST_RPC_METHODS } from './request-rpc-methods'

// Why: frontend tests mock callRequestRpc, so a channel the gateway never registered
// (BUG-E2E-001: clarification.*) passed every test. The gateway's golden inventory is
// the single source of truth for which channels exist.
const INVENTORY_RELATIVE_PATH =
  'backend-go/services/api-gateway/internal/adapter/wscompat/testdata/channel_inventory.json'
// Why the override: lets CI point at the golden file of another checkout, and lets the
// "removing a channel turns this test red" check run without editing the backend file.
const INVENTORY_PATH =
  process.env.ORCA_GATEWAY_CHANNEL_INVENTORY ??
  resolve(__dirname, '../../..', INVENTORY_RELATIVE_PATH)

type InventoryEntry = { name: string; kind: string }

/**
 * Channels the UI declares but the gateway does not register yet. Each one is a
 * contract name whose backend CR has not shipped; the hooks that call them must
 * treat `unsupported` as "feature not available" (see useDecisions, useImpactAssessment,
 * useTaskReadiness, usePlanHeatmap, useGraphLens). An entry that the
 * gateway starts registering fails the test below so this list can only shrink.
 */
const PENDING_GATEWAY_CHANNELS: Readonly<Record<string, string>> = {
  'impact.graph': 'CR-REQ-030 impact channels are not registered at the gateway yet',
  'impact.heatmap': 'CR-REQ-030 impact channels are not registered at the gateway yet',
  'impact.drift': 'CR-REQ-030 impact channels are not registered at the gateway yet',
  'impact.get': 'CR-REQ-030 impact channels are not registered at the gateway yet',
  'impact.request': 'CR-REQ-030 impact channels are not registered at the gateway yet',
  'impact.compare': 'CR-REQ-030 impact channels are not registered at the gateway yet',
  'impact.findings': 'CR-REQ-030 impact channels are not registered at the gateway yet',
  'impact.evidence': 'CR-REQ-030 impact channels are not registered at the gateway yet',
  'impact.accept': 'CR-REQ-030 impact channels are not registered at the gateway yet',
  'risk.override': 'CR-REQ-030 risk override is not registered at the gateway yet',
  'request.readiness': 'CR-REQ-028 request readiness is not registered at the gateway yet',
  'readiness.check': 'CR-REQ-029 task readiness channels are not registered at the gateway yet',
  'readiness.get': 'CR-REQ-029 task readiness channels are not registered at the gateway yet',
  'readiness.list': 'CR-REQ-029 task readiness channels are not registered at the gateway yet'
}

function loadInventory(): InventoryEntry[] {
  if (!existsSync(INVENTORY_PATH)) {
    throw new Error(
      `Gateway channel inventory not found at ${INVENTORY_PATH}. ` +
        'Run the frontend tests from a full checkout (the contract test reads the backend golden file).'
    )
  }
  return JSON.parse(readFileSync(INVENTORY_PATH, 'utf8')) as InventoryEntry[]
}

describe('request RPC channel contract with the gateway', () => {
  const inventory = loadInventory()
  const registered = new Set(inventory.map((e) => e.name))
  const declared = Object.values(REQUEST_RPC_METHODS) as string[]

  it('reads a non-trivial golden inventory', () => {
    expect(inventory.length).toBeGreaterThan(100)
    expect(registered.has('request.list')).toBe(true)
  })

  it('has every channel the Request UI calls registered at the gateway', () => {
    const missing = declared.filter((m) => !registered.has(m) && !(m in PENDING_GATEWAY_CHANNELS))
    expect(
      missing,
      `Channels in REQUEST_RPC_METHODS missing from the gateway inventory: ${missing.join(', ')}. ` +
        `Either the backend must register them (then update ${INVENTORY_RELATIVE_PATH} with ` +
        'UPDATE_GOLDEN=1 go test ./internal/adapter/wscompat -run TestChannelInventoryMatchesGolden) ' +
        'or add them to PENDING_GATEWAY_CHANNELS with a reason.'
    ).toEqual([])
  })

  it('keeps clarification.* registered, the channels BUG-E2E-001 lost', () => {
    for (const m of [
      'clarification.list',
      'clarification.get',
      'clarification.answer',
      'clarification.cancel'
    ]) {
      expect(declared, `${m} must be declared by the UI`).toContain(m)
      expect(registered.has(m), `${m} must be registered at the gateway`).toBe(true)
    }
  })

  it('drops allowlist entries once the gateway registers the channel', () => {
    const stale = Object.keys(PENDING_GATEWAY_CHANNELS).filter((m) => registered.has(m))
    expect(
      stale,
      `Registered at the gateway now, remove from PENDING_GATEWAY_CHANNELS: ${stale.join(', ')}`
    ).toEqual([])
  })

  it('only allowlists channels the UI actually declares', () => {
    const unknown = Object.keys(PENDING_GATEWAY_CHANNELS).filter((m) => !declared.includes(m))
    expect(
      unknown,
      `Not in REQUEST_RPC_METHODS, remove from PENDING_GATEWAY_CHANNELS: ${unknown.join(', ')}`
    ).toEqual([])
  })

  it('registers execution.get and the push channels the task and dev-server views subscribe to', () => {
    const kindOf = new Map(inventory.map((e) => [e.name, e.kind]))
    expect(kindOf.get(REQUEST_RPC_METHODS.EXECUTION_GET)).toBe('unary')
    expect(kindOf.get('task.activity.subscribe')).toBe('stream')
    expect(kindOf.get('devServer.subscribeStatus')).toBe('stream')
  })

  it('declares streaming channels with the kind the UI uses them with', () => {
    const kindOf = new Map(inventory.map((e) => [e.name, e.kind]))
    expect(kindOf.get(REQUEST_RPC_METHODS.SUBSCRIBE)).toBe('stream')
    const unary = declared.filter((m) => m !== REQUEST_RPC_METHODS.SUBSCRIBE && registered.has(m))
    const notUnary = unary.filter((m) => kindOf.get(m) !== 'unary')
    expect(notUnary).toEqual([])
  })
})

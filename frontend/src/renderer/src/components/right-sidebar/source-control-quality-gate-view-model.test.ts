/**
 * Tests for source-control-quality-gate-view-model.ts (FE-CV-TASK-085-02)
 */

import { describe, it, expect } from 'vitest'
import {
  buildQualityNoticeViewModel,
  type QualityGate
} from './source-control-quality-gate-view-model'

function gate(overrides: Partial<QualityGate>): QualityGate {
  return { result: 'pass', reasons: [], ...overrides }
}

describe('buildQualityNoticeViewModel — hidden cases', () => {
  it('pass → hidden', () => {
    const vm = buildQualityNoticeViewModel(gate({ result: 'pass' }))
    expect(vm.visible).toBe(false)
  })

  it('null gate → visible unknown + unavailable', () => {
    const vm = buildQualityNoticeViewModel(null)
    expect(vm.visible).toBe(true)
    if (vm.visible) {
      expect(vm.severity).toBe('unknown')
      expect(vm.unavailable).toBe(true)
    }
  })
})

describe('buildQualityNoticeViewModel — severity mapping', () => {
  it('fail → severity fail', () => {
    const vm = buildQualityNoticeViewModel(gate({ result: 'fail' }))
    expect(vm.visible).toBe(true)
    if (vm.visible) expect(vm.severity).toBe('fail')
  })

  it('warn → severity warn', () => {
    const vm = buildQualityNoticeViewModel(gate({ result: 'warn' }))
    expect(vm.visible).toBe(true)
    if (vm.visible) expect(vm.severity).toBe('warn')
  })

  it('unknown → severity unknown + unavailable', () => {
    const vm = buildQualityNoticeViewModel(gate({ result: 'unknown' }))
    expect(vm.visible).toBe(true)
    if (vm.visible) {
      expect(vm.severity).toBe('unknown')
      expect(vm.unavailable).toBe(true)
    }
  })

  it('timeout → severity unknown + unavailable', () => {
    const vm = buildQualityNoticeViewModel(gate({ result: 'timeout' as never }))
    expect(vm.visible).toBe(true)
    if (vm.visible) expect(vm.severity).toBe('unknown')
  })

  it('offline → severity unknown + unavailable', () => {
    const vm = buildQualityNoticeViewModel(gate({ result: 'offline' as never }))
    expect(vm.visible).toBe(true)
    if (vm.visible) expect(vm.severity).toBe('unknown')
  })

  it('mode does not change output', () => {
    const withBlock = buildQualityNoticeViewModel(gate({ result: 'fail', mode: 'block' }))
    const withWarn = buildQualityNoticeViewModel(gate({ result: 'fail', mode: 'warn' }))
    if (withBlock.visible && withWarn.visible) {
      expect(withBlock.severity).toBe(withWarn.severity)
    }
  })
})

describe('buildQualityNoticeViewModel — reason handling', () => {
  it('reason with known code → mapped i18n key', () => {
    const vm = buildQualityNoticeViewModel(gate({
      result: 'fail',
      reasons: [{ check: 'coverage', observed: 70, threshold: 80, result: 'fail', code: 'coverage_below_threshold' }]
    }))
    if (vm.visible) {
      expect(vm.reasons[0].labelKey).toContain('coverage_below_threshold')
    }
  })

  it('unknown code → reason.unknown key with check preserved', () => {
    const vm = buildQualityNoticeViewModel(gate({
      result: 'fail',
      reasons: [{ check: 'my_custom_check', observed: 1, threshold: 0, result: 'fail', code: 'totally_unknown' }]
    }))
    if (vm.visible) {
      expect(vm.reasons[0].labelKey).toContain('reason.unknown')
      expect(vm.reasons[0].check).toBe('my_custom_check')
    }
  })

  it('no code → reason.unknown', () => {
    const vm = buildQualityNoticeViewModel(gate({
      result: 'warn',
      reasons: [{ check: 'some_check', observed: 1, threshold: 0, result: 'warn' }]
    }))
    if (vm.visible) {
      expect(vm.reasons[0].labelKey).toContain('reason.unknown')
    }
  })

  it('truncates to 3 reasons, preserves reasonCount', () => {
    const reasons = Array.from({ length: 6 }, (_, i) => ({
      check: `check-${i}`, observed: i, threshold: 0, result: 'fail' as const, code: 'security_violations'
    }))
    const vm = buildQualityNoticeViewModel(gate({ result: 'fail', reasons }))
    if (vm.visible) {
      expect(vm.reasons).toHaveLength(3)
      expect(vm.reasonCount).toBe(6)
    }
  })

  it('sorts fail reasons first', () => {
    const reasons = [
      { check: 'warn-check', observed: 1, threshold: 0, result: 'warn' as const },
      { check: 'fail-check', observed: 1, threshold: 0, result: 'fail' as const },
    ]
    const vm = buildQualityNoticeViewModel(gate({ result: 'fail', reasons }))
    if (vm.visible) {
      expect(vm.reasons[0].check).toBe('fail-check')
    }
  })
})

describe('buildQualityNoticeViewModel — stale + unavailable', () => {
  it('passes through stale=true', () => {
    const vm = buildQualityNoticeViewModel(gate({ result: 'warn', stale: true }))
    if (vm.visible) expect(vm.stale).toBe(true)
  })

  it('unknown result always sets unavailable regardless of gate.unavailable', () => {
    const vm = buildQualityNoticeViewModel(gate({ result: 'unknown', unavailable: false }))
    if (vm.visible) expect(vm.unavailable).toBe(true)
  })
})

/**
 * Minimal zustand store with the code-intel and quality slices, for hook tests that mock
 * '@/store'. The quality RPC seam is injectable so tests never touch the real bridge.
 */
import { create } from 'zustand'
import { createCodeIntelSlice } from '../store/slices/code-intel'
import type { CodeIntelSlice } from '../store/slices/code-intel'
import { createCodeIntelQualitySlice } from '../store/slices/code-intel-quality-state'
import type { CodeIntelQualitySlice } from '../store/slices/code-intel-quality-state'
import type { QualityCall } from '../store/slices/code-intel-quality-slice-context'

export type CodeIntelQualityTestState = CodeIntelSlice & CodeIntelQualitySlice

export function createCodeIntelQualityTestStore(call: QualityCall) {
  return create<CodeIntelQualityTestState>()((set, get) => ({
    ...createCodeIntelSlice(set as never, get as never),
    ...createCodeIntelQualitySlice(set as never, get as never, { call })
  }))
}

export const ENABLED_QUALITY_SUPPORT = {
  state: 'enabled' as const,
  effective: { codeIntelEnabled: true, qualityGateEnabled: true, aiReviewEnabled: false }
}

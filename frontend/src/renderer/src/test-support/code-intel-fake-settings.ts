// Default tenant settings and flag cascade for the fake code-intel backend (contract §4.1, §6).
import type { CodeIntelSettings } from '../../../shared/code-intel-types'

export function defaultFakeSettings(): CodeIntelSettings {
  return {
    effective: {
      codeIntelEnabled: true,
      qualityGateEnabled: true,
      qualitySecurityScanEnabled: false,
      aiReviewEnabled: false
    },
    tenant: {
      codeIntelEnabled: true,
      qualityGateEnabled: true,
      qualitySecurityScanEnabled: false,
      indexPolicy: 'auto_in_place',
      aiReviewLevel: 'off',
      aiReviewModel: '',
      agentTurnStorePromptExcerpt: false,
      agentClaimTextEnabled: false,
      hotspotWindowDays: 90
    }
  }
}

// Why: quality needs code-intel and AI needs quality, so a tenant flag cannot outlive its parent.
export function withDerivedEffective(settings: CodeIntelSettings): CodeIntelSettings {
  const t = settings.tenant
  const quality = t.codeIntelEnabled && t.qualityGateEnabled
  return {
    ...settings,
    effective: {
      codeIntelEnabled: t.codeIntelEnabled,
      qualityGateEnabled: quality,
      qualitySecurityScanEnabled: quality && t.qualitySecurityScanEnabled,
      aiReviewEnabled: quality && t.aiReviewLevel !== 'off'
    }
  }
}

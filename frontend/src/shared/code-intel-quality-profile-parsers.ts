/**
 * code-intel-quality-profile-parsers.ts — FE-CV-TASK-087-01
 *
 * Never-throw parsers for runnable profiles and quality profiles (§4.7, `quality.profile.get`).
 *
 * @module shared/code-intel-quality-profile-parsers
 */

import {
  count,
  finiteOrNull,
  list,
  oneOf,
  optFinite,
  optStr,
  rec,
  str,
  strList
} from './code-intel-quality-parse-primitives'
import type {
  MissingCheck,
  QualityProfile,
  QualityProfileDefinition,
  QualityProfileResponse,
  RunnableProfile
} from './code-intel-quality-types'

const MODES = new Set(['inform', 'block'])
const ORIGINS = new Set(['repo', 'tenant', 'builtin'])
const COUNT_SCOPES = new Set(['changedFiles', 'all'])

// ---------------------------------------------------------------------------
// Profiles
// ---------------------------------------------------------------------------

function parseMissing(raw: unknown): MissingCheck {
  const r = rec(raw)
  return {
    check: str(r.check),
    reason: str(r.reason),
    ...(optStr(r.hint) !== undefined ? { hint: optStr(r.hint) } : {})
  }
}

export function parseRunnableProfile(raw: unknown): RunnableProfile {
  const r = rec(raw)
  const suite = strList(r.suite)
  return {
    id: str(r.id),
    title: str(r.title),
    kind: str(r.kind),
    // Why: an unreadable `ready` must not enable running; fail closed.
    ready: r.ready === true,
    heavy: r.heavy === true,
    scopes: strList(r.scopes),
    missing: list(r.missing).map(parseMissing),
    ...(suite.length > 0 ? { suite } : {})
  }
}

function parseProfileDefinition(raw: unknown): QualityProfileDefinition {
  const r = rec(raw)
  const findings = rec(r.findings)
  const coverage = rec(r.coverage)
  const structure = rec(r.structure)
  const freshness = rec(r.freshness)
  return {
    schemaVersion: count(r.schemaVersion),
    checks: list(r.checks).map((c) => {
      const x = rec(c)
      return {
        id: str(x.id),
        profile: str(x.profile),
        category: str(x.category),
        required: x.required === true,
        ...(optFinite(x.maxErrors) !== undefined ? { maxErrors: optFinite(x.maxErrors) } : {}),
        ...(optFinite(x.maxWarnings) !== undefined
          ? { maxWarnings: optFinite(x.maxWarnings) }
          : {}),
        ...(optFinite(x.maxFailed) !== undefined ? { maxFailed: optFinite(x.maxFailed) } : {})
      }
    }),
    findings: {
      countScope: oneOf(findings.countScope, COUNT_SCOPES),
      blockingSeverities: strList(findings.blockingSeverities),
      warnBudget: count(findings.warnBudget)
    },
    coverage: {
      required: coverage.required === true,
      diffCoverageWarnBelow: finiteOrNull(coverage.diffCoverageWarnBelow),
      diffCoverageFailBelow: finiteOrNull(coverage.diffCoverageFailBelow)
    },
    structure: {
      newLayerViolationErrorFails: structure.newLayerViolationErrorFails === true,
      newLayerViolationWarningWarns: structure.newLayerViolationWarningWarns === true,
      newCyclesWarn: structure.newCyclesWarn === true
    },
    freshness: { indexMustMatchHead: freshness.indexMustMatchHead === true }
  }
}

export function parseQualityProfile(raw: unknown): QualityProfile {
  const r = rec(raw)
  return {
    name: str(r.name),
    mode: oneOf(r.mode, MODES),
    definition: parseProfileDefinition(r.definition),
    version: count(r.version)
  }
}

export function parseQualityProfileResponse(raw: unknown): QualityProfileResponse {
  const r = rec(raw)
  return {
    profile: parseQualityProfile(r.profile),
    origin: oneOf(r.origin, ORIGINS),
    version: count(r.version),
    runnableProfiles: list(r.runnableProfiles).map(parseRunnableProfile)
  }
}

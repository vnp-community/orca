export const MAX_FINDINGS_PER_STEP = parseInt(process.env.ORCA_QUALITY_MAX_FINDINGS_PER_STEP || '5000', 10)
export const MAX_FINDINGS_PER_RUN = parseInt(process.env.ORCA_QUALITY_MAX_FINDINGS_PER_RUN || '20000', 10)
export const MESSAGE_MAX_BYTES = parseInt(process.env.ORCA_QUALITY_MESSAGE_MAX_BYTES || '2048', 10)
export const FIX_HINT_MAX_BYTES = parseInt(process.env.ORCA_QUALITY_FIX_HINT_MAX_BYTES || '500', 10)

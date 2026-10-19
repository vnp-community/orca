// Form fields cho TaskKGContext trong task create/edit (CR-ORCA-002)
//
// Fields submit snake_case JSON để match API (SOL-ORCA-002 §2.4):
//   crs_id, feature_ref, service, repo_target
// Empty string → undefined (không gửi null lên API)

import type { FC } from 'react'
import type { TaskKGContext } from '../../types/task-types'

interface Props {
  value: TaskKGContext
  onChange: (ctx: TaskKGContext) => void
}

export const TaskFormKGFields: FC<Props> = ({ value, onChange }) => (
  <fieldset className="task-form-kg-fields" id="task-form-kg-fields">
    <legend>
      KG Context <span className="optional">(Optional)</span>
    </legend>

    <div className="form-field">
      <label htmlFor="task-crs-id">CRS ID</label>
      <input
        id="task-crs-id"
        type="text"
        placeholder="CRS-045"
        value={value.crs_id ?? ''}
        onChange={(e) => onChange({ ...value, crs_id: e.target.value || undefined })}
      />
      <span className="form-field__hint">
        PIKB CRS implementing this task — enables KG context auto-inject
      </span>
    </div>

    <div className="form-field">
      <label htmlFor="task-feature-ref">Feature Ref</label>
      <input
        id="task-feature-ref"
        type="text"
        placeholder="F27"
        value={value.feature_ref ?? ''}
        onChange={(e) => onChange({ ...value, feature_ref: e.target.value || undefined })}
      />
    </div>

    <div className="form-field">
      <label htmlFor="task-service">Target Service</label>
      <input
        id="task-service"
        type="text"
        placeholder="ai-service"
        value={value.service ?? ''}
        onChange={(e) => onChange({ ...value, service: e.target.value || undefined })}
      />
    </div>

    <div className="form-field">
      <label htmlFor="task-repo-target">Repo Target</label>
      <input
        id="task-repo-target"
        type="text"
        placeholder="vnp-workspace"
        value={value.repo_target ?? ''}
        onChange={(e) => onChange({ ...value, repo_target: e.target.value || undefined })}
      />
      <span className="form-field__hint">
        Used for <code>{'// implements: CRS-XXX'}</code> convention verify
      </span>
    </div>
  </fieldset>
)

-- CR-REQ-028, MySQL 8.0.16+ (CHECK enforcement, SKIP LOCKED needs 8.0.1+).
-- MySQL names the inline status CHECK itself (requests_chk_N), so look the name up before dropping it.
SET @status_check := (
    SELECT tc.CONSTRAINT_NAME FROM information_schema.TABLE_CONSTRAINTS tc
    JOIN information_schema.CHECK_CONSTRAINTS cc ON cc.CONSTRAINT_SCHEMA = tc.CONSTRAINT_SCHEMA AND cc.CONSTRAINT_NAME = tc.CONSTRAINT_NAME
    WHERE tc.TABLE_SCHEMA = DATABASE() AND tc.TABLE_NAME = 'requests' AND tc.CONSTRAINT_TYPE = 'CHECK'
      AND cc.CHECK_CLAUSE LIKE '%awaiting_type_confirmation%'
    LIMIT 1);
SET @drop_sql := IF(@status_check IS NULL, 'DO 0', CONCAT('ALTER TABLE requests DROP CHECK `', @status_check, '`'));
PREPARE drop_status_check FROM @drop_sql;
EXECUTE drop_status_check;
DEALLOCATE PREPARE drop_status_check;

ALTER TABLE requests ADD CONSTRAINT requests_status_check CHECK (status IN (
    'new','classifying','awaiting_type_confirmation','analyzing','awaiting_analysis_approval','planning',
    'awaiting_plan_approval','executing','completed','request_backlog','cancelled','awaiting_information'));

-- open_key is NULL unless the row is open, and NULLs do not collide: one open clarification per request.
CREATE TABLE clarifications (
    id CHAR(36) PRIMARY KEY,
    tenant_id CHAR(36) NOT NULL,
    request_id CHAR(36) NOT NULL,
    seq INT NOT NULL,
    source VARCHAR(32) NOT NULL CHECK (source IN ('readiness','solution_open_question','plan_assumption','task_blocked','manual')),
    source_ref VARCHAR(255) NOT NULL DEFAULT '',
    status VARCHAR(16) NOT NULL CHECK (status IN ('open','answered','expired','cancelled')),
    resume_status VARCHAR(16) NOT NULL CHECK (resume_status IN ('analyzing','planning','executing')),
    round INT NOT NULL DEFAULT 1,
    asked_request_revision INT NOT NULL,
    answered_request_revision INT NULL,
    due_at TIMESTAMP(6) NOT NULL,
    reminded_at TIMESTAMP(6) NULL,
    cancel_reason TEXT NOT NULL,
    created_by VARCHAR(64) NOT NULL,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    answered_at TIMESTAMP(6) NULL,
    version BIGINT NOT NULL DEFAULT 1,
    open_key VARCHAR(36) GENERATED ALWAYS AS (IF(status = 'open', request_id, NULL)) STORED,
    UNIQUE KEY clarifications_request_seq (tenant_id, request_id, seq),
    UNIQUE KEY clarifications_one_open (tenant_id, open_key),
    INDEX clarifications_expiry_scan (tenant_id, status, due_at),
    INDEX clarifications_due_scan (status, due_at),
    INDEX clarifications_by_request (tenant_id, request_id, created_at)
) ENGINE=InnoDB;

CREATE TABLE clarification_questions (
    id CHAR(36) PRIMARY KEY,
    tenant_id CHAR(36) NOT NULL,
    clarification_id CHAR(36) NOT NULL,
    seq INT NOT NULL,
    question_key VARCHAR(120) NOT NULL,
    kind VARCHAR(16) NOT NULL CHECK (kind IN ('text','single_choice','multi_choice','file','boolean')),
    prompt TEXT NOT NULL,
    reason TEXT NOT NULL,
    options JSON NULL,
    suggested_default JSON NULL,
    required TINYINT(1) NOT NULL DEFAULT 1,
    target_path VARCHAR(160) NULL,
    answer JSON NULL,
    answer_source VARCHAR(20) NULL CHECK (answer_source IN ('user','default_accepted')),
    answered_by VARCHAR(64) NULL,
    answered_at TIMESTAMP(6) NULL,
    CONSTRAINT chk_clarification_questions_reason CHECK (reason <> ''),
    CONSTRAINT fk_clarification_questions_clarification FOREIGN KEY (clarification_id) REFERENCES clarifications(id) ON DELETE CASCADE,
    UNIQUE KEY clarification_questions_seq (tenant_id, clarification_id, seq)
) ENGINE=InnoDB;

CREATE TABLE clarification_assignees (
    clarification_id CHAR(36) NOT NULL,
    tenant_id CHAR(36) NOT NULL,
    principal_kind VARCHAR(16) NOT NULL CHECK (principal_kind IN ('user','team','role','reporter')),
    principal_id VARCHAR(64) NOT NULL DEFAULT '',
    PRIMARY KEY (clarification_id, principal_kind, principal_id),
    CONSTRAINT fk_clarification_assignees_clarification FOREIGN KEY (clarification_id) REFERENCES clarifications(id) ON DELETE CASCADE,
    INDEX clarification_assignees_principal (tenant_id, principal_kind, principal_id)
) ENGINE=InnoDB;

-- live_key is NULL once superseded: one live decision per subject, as the Postgres partial index.
CREATE TABLE decisions (
    id CHAR(36) PRIMARY KEY,
    tenant_id CHAR(36) NOT NULL,
    request_id CHAR(36) NOT NULL,
    seq INT NOT NULL,
    subject_kind VARCHAR(20) NOT NULL CHECK (subject_kind IN ('solution_option','plan_assumption','other')),
    subject_id VARCHAR(80) NOT NULL,
    subject_digest VARCHAR(80) NOT NULL DEFAULT '',
    question TEXT NOT NULL,
    options JSON NOT NULL,
    recommended_option_id VARCHAR(64) NULL,
    recommendation_reason TEXT NOT NULL,
    chosen_option_id VARCHAR(64) NULL,
    chooser_id VARCHAR(64) NULL,
    chosen_at TIMESTAMP(6) NULL,
    rationale TEXT NOT NULL,
    risk_level VARCHAR(8) NOT NULL DEFAULT 'normal' CHECK (risk_level IN ('normal','high')),
    confirmed_by VARCHAR(64) NULL,
    confirmed_at TIMESTAMP(6) NULL,
    status VARCHAR(16) NOT NULL CHECK (status IN ('open','chosen','effective','superseded')),
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    live_key VARCHAR(120) GENERATED ALWAYS AS (IF(status IN ('open','chosen','effective'), CONCAT(subject_kind, ':', subject_id), NULL)) STORED,
    UNIQUE KEY decisions_request_seq (tenant_id, request_id, seq),
    UNIQUE KEY decisions_one_live (tenant_id, live_key),
    INDEX decisions_by_request (tenant_id, request_id, created_at)
) ENGINE=InnoDB;

CREATE TABLE decision_history (
    id CHAR(36) PRIMARY KEY,
    tenant_id CHAR(36) NOT NULL,
    decision_id CHAR(36) NOT NULL,
    action VARCHAR(16) NOT NULL CHECK (action IN ('chosen','rechosen','confirmed','superseded')),
    option_id VARCHAR(64) NULL,
    actor_id VARCHAR(64) NULL,
    rationale TEXT NOT NULL,
    at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT fk_decision_history_decision FOREIGN KEY (decision_id) REFERENCES decisions(id) ON DELETE CASCADE,
    INDEX decision_history_by_decision (tenant_id, decision_id, at)
) ENGINE=InnoDB;

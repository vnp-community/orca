-- CR-REQ-028: awaiting_information, Clarification and Decision records.
DO $$
DECLARE
    c text;
BEGIN
    SELECT conname INTO c FROM pg_constraint
    WHERE conrelid = 'request.requests'::regclass AND contype = 'c' AND pg_get_constraintdef(oid) LIKE '%awaiting_type_confirmation%';
    IF c IS NOT NULL THEN
        EXECUTE format('ALTER TABLE request.requests DROP CONSTRAINT %I', c);
    END IF;
END
$$;
ALTER TABLE request.requests ADD CONSTRAINT requests_status_check CHECK (status IN (
    'new','classifying','awaiting_type_confirmation','analyzing','awaiting_analysis_approval','planning',
    'awaiting_plan_approval','executing','completed','request_backlog','cancelled','awaiting_information'));

CREATE TABLE request.clarifications (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    request_id UUID NOT NULL,
    seq INT NOT NULL,
    source TEXT NOT NULL CHECK (source IN ('readiness','solution_open_question','plan_assumption','task_blocked','manual')),
    source_ref TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('open','answered','expired','cancelled')),
    resume_status TEXT NOT NULL CHECK (resume_status IN ('analyzing','planning','executing')),
    round INT NOT NULL DEFAULT 1,
    asked_request_revision INT NOT NULL,
    answered_request_revision INT NULL,
    due_at TIMESTAMPTZ NOT NULL,
    reminded_at TIMESTAMPTZ NULL,
    cancel_reason TEXT NOT NULL DEFAULT '',
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    answered_at TIMESTAMPTZ NULL,
    version BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT clarifications_request_seq UNIQUE (tenant_id, request_id, seq)
);
CREATE UNIQUE INDEX clarifications_one_open ON request.clarifications (tenant_id, request_id) WHERE status = 'open';
CREATE INDEX clarifications_expiry_scan ON request.clarifications (tenant_id, status, due_at);
CREATE INDEX clarifications_by_request ON request.clarifications (tenant_id, request_id, created_at);

CREATE TABLE request.clarification_questions (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    clarification_id UUID NOT NULL REFERENCES request.clarifications(id) ON DELETE CASCADE,
    seq INT NOT NULL,
    question_key TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('text','single_choice','multi_choice','file','boolean')),
    prompt TEXT NOT NULL,
    reason TEXT NOT NULL CHECK (reason <> ''),
    options JSONB NULL,
    suggested_default JSONB NULL,
    required BOOLEAN NOT NULL DEFAULT true,
    target_path TEXT NULL,
    answer JSONB NULL,
    answer_source TEXT NULL CHECK (answer_source IN ('user','default_accepted')),
    answered_by TEXT NULL,
    answered_at TIMESTAMPTZ NULL,
    CONSTRAINT clarification_questions_seq UNIQUE (tenant_id, clarification_id, seq)
);

-- reporter assignees carry the empty principal_id because a primary key cannot hold NULL.
CREATE TABLE request.clarification_assignees (
    clarification_id UUID NOT NULL REFERENCES request.clarifications(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL,
    principal_kind TEXT NOT NULL CHECK (principal_kind IN ('user','team','role','reporter')),
    principal_id TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (clarification_id, principal_kind, principal_id)
);
CREATE INDEX clarification_assignees_principal ON request.clarification_assignees (tenant_id, principal_kind, principal_id);

CREATE TABLE request.decisions (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    request_id UUID NOT NULL,
    seq INT NOT NULL,
    subject_kind TEXT NOT NULL CHECK (subject_kind IN ('solution_option','plan_assumption','other')),
    subject_id TEXT NOT NULL,
    subject_digest TEXT NOT NULL DEFAULT '',
    question TEXT NOT NULL DEFAULT '',
    options JSONB NOT NULL,
    recommended_option_id TEXT NULL,
    recommendation_reason TEXT NOT NULL DEFAULT '',
    chosen_option_id TEXT NULL,
    chooser_id TEXT NULL,
    chosen_at TIMESTAMPTZ NULL,
    rationale TEXT NOT NULL DEFAULT '',
    risk_level TEXT NOT NULL DEFAULT 'normal' CHECK (risk_level IN ('normal','high')),
    confirmed_by TEXT NULL,
    confirmed_at TIMESTAMPTZ NULL,
    status TEXT NOT NULL CHECK (status IN ('open','chosen','effective','superseded')),
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT decisions_request_seq UNIQUE (tenant_id, request_id, seq)
);
CREATE UNIQUE INDEX decisions_one_live ON request.decisions (tenant_id, subject_kind, subject_id)
    WHERE status IN ('open','chosen','effective');
CREATE INDEX decisions_by_request ON request.decisions (tenant_id, request_id, created_at);

-- Append-only audit of choices.
CREATE TABLE request.decision_history (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    decision_id UUID NOT NULL REFERENCES request.decisions(id) ON DELETE CASCADE,
    action TEXT NOT NULL CHECK (action IN ('chosen','rechosen','confirmed','superseded')),
    option_id TEXT NULL,
    actor_id TEXT NULL,
    rationale TEXT NOT NULL DEFAULT '',
    at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX decision_history_by_decision ON request.decision_history (tenant_id, decision_id, at);

DO $$
DECLARE
    t text;
BEGIN
    FOREACH t IN ARRAY ARRAY['clarifications', 'clarification_questions', 'clarification_assignees', 'decisions', 'decision_history'] LOOP
        EXECUTE format('ALTER TABLE request.%I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE request.%I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format('DROP POLICY IF EXISTS tenant_isolation ON request.%I', t);
        EXECUTE format('CREATE POLICY tenant_isolation ON request.%I FOR ALL USING (tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid)', t);
    END LOOP;
END
$$;

-- The expiry and reminder sweeps list due clarifications across tenants (read only), like the outbox relay;
-- every write then happens in a per-tenant transaction.
CREATE POLICY relay_scan ON request.clarifications
    AS PERMISSIVE FOR SELECT
    USING (current_setting('app.relay', true) = 'on');

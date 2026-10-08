ALTER TABLE requests ADD COLUMN classification_attempts INT NOT NULL DEFAULT 0, ADD CONSTRAINT chk_requests_classification_attempts CHECK (classification_attempts >= 0);

ALTER TABLE request.requests ADD COLUMN classification_attempts INT NOT NULL DEFAULT 0 CHECK (classification_attempts >= 0);

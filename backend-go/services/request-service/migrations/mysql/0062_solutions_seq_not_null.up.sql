-- Every writer of solutions now assigns seq (insertSolutionRow), so the column can stop being optional.
UPDATE solutions s
JOIN (SELECT id, ROW_NUMBER() OVER (PARTITION BY tenant_id, request_id ORDER BY created_at, id) AS rn FROM solutions WHERE seq IS NULL) n ON n.id = s.id
SET s.seq = n.rn
WHERE s.seq IS NULL;
ALTER TABLE solutions MODIFY seq INT NOT NULL;

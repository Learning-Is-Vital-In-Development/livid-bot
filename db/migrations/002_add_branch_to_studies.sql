ALTER TABLE studies
ADD COLUMN IF NOT EXISTS branch TEXT NOT NULL DEFAULT '';

ALTER TABLE studies
DROP CONSTRAINT IF EXISTS studies_name_key;

CREATE UNIQUE INDEX IF NOT EXISTS studies_branch_name_key ON studies(branch, name);

CREATE INDEX IF NOT EXISTS idx_studies_status_branch ON studies(status, branch);

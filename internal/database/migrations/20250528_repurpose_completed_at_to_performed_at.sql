-- +migrate Up
-- Repurpose completed_at to performed_at and update status logic

-- Copy data from completed_at to performed_at (GORM already created performed_at column)
UPDATE chore_histories 
SET performed_at = completed_at,
    status = CASE 
        WHEN completed_at IS NOT NULL THEN 1  -- Task was completed
        ELSE 2  -- Task was skipped
    END
WHERE performed_at IS NULL;  -- Only update if performed_at is empty

-- if we don't have completed_at then will used updated_at :
UPDATE chore_histories
SET performed_at = updated_at
WHERE performed_at IS NULL;


-- Drop the old completed_at column now that data is migrated
ALTER TABLE chore_histories DROP COLUMN completed_at;

-- +migrate Down
-- Rollback: recreate completed_at column and copy data back
ALTER TABLE chore_histories ADD COLUMN completed_at DATETIME;

-- Copy data back from performed_at to completed_at
UPDATE chore_histories SET completed_at = performed_at;

-- Reset status to NULL (original state)
UPDATE chore_histories SET status = NULL;

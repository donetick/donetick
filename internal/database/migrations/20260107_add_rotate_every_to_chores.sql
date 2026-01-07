-- +migrate Up
-- Add rotate_every column to chores table for scheduled rotation
-- This column specifies how many completions should occur before rotating the assignee
-- NULL or 0 means rotate on every completion (default behavior)
ALTER TABLE chores ADD COLUMN IF NOT EXISTS rotate_every INTEGER DEFAULT NULL;

-- +migrate Down
ALTER TABLE chores DROP COLUMN IF EXISTS rotate_every;

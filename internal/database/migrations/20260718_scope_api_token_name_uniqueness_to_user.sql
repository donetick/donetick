-- +migrate Up
-- This migration originally executed `ALTER TABLE api_tokens DROP CONSTRAINT ...`,
-- which SQLite cannot parse and crashed startup on SQLite deployments (issue #729).
-- The constraint drop is now handled by the dialect-aware Go migration
-- migrations/20260719_drop_api_token_name_unique.go. This file is kept as a no-op
-- so deployments that already recorded it keep valid migration bookkeeping.
SELECT 1;

-- +migrate Down

-- +migrate Up
ALTER TABLE api_tokens DROP CONSTRAINT IF EXISTS uni_api_tokens_name;

-- +migrate Down

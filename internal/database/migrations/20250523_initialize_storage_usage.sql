-- +migrate Up
INSERT INTO storage_usages (user_id,circle_id,used_bytes)
SELECT u.id, u.circle_id, 0 FROM users u;

-- +migrate Down
DELETE FROM storage_usages WHERE user_id IN (SELECT id FROM users);
-- This migration initializes the storage_usages table by setting the used_bytes to 0 for each user in their respective circles.
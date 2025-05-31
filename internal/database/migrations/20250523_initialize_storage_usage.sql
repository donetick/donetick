-- +migrate Up
INSERT INTO storage_usages (user_id,circle_id,used_bytes) SELECT u.id, uc.circle_id, 0 FROM users u LEFT JOIN user_circles uc ON u.id = uc.user_id;


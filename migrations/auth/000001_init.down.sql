DROP INDEX IF EXISTS idx_user_tokens_revoked_at;
DROP INDEX IF EXISTS idx_user_tokens_expires_at;
DROP INDEX IF EXISTS idx_user_tokens_user_id;

DROP INDEX IF EXISTS idx_user_otps_expires_at;
DROP INDEX IF EXISTS idx_user_otps_email;
DROP INDEX IF EXISTS idx_user_otps_user_id;

DROP TABLE IF EXISTS user_tokens;
DROP TABLE IF EXISTS user_otps;
DROP TABLE IF EXISTS users;

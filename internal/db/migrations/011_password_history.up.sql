-- Track password history to prevent password reuse
CREATE TABLE password_history (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_password_history_user_id_created_at ON password_history(user_id, created_at DESC);

-- Cleanup trigger to keep only last 10 passwords per user
CREATE OR REPLACE FUNCTION cleanup_old_passwords()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM password_history
    WHERE user_id = NEW.user_id
    AND id NOT IN (
        SELECT id FROM password_history
        WHERE user_id = NEW.user_id
        ORDER BY created_at DESC
        LIMIT 10
    );
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_cleanup_passwords
AFTER INSERT ON password_history
FOR EACH ROW
EXECUTE FUNCTION cleanup_old_passwords();

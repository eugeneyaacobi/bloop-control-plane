-- Add email verification and role tracking to users
ALTER TABLE users ADD COLUMN IF NOT EXISTS verified_at TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'customer';

-- Backfill existing seeded users as verified owners
UPDATE users SET verified_at = NOW(), role = 'owner' WHERE id IN ('user_gene', 'user_eugene-yaacobi-in');

-- Mark users who already have credentials as verified (they registered through the old flow)
UPDATE users SET verified_at = NOW() WHERE password_set = true AND verified_at IS NULL;

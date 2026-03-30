-- Migration 004: User credentials table for password authentication
-- Adds user_credentials table and extends users table with username and password_set flag

-- Add columns to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS username TEXT UNIQUE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_set BOOLEAN NOT NULL DEFAULT false;

-- Create user_credentials table
CREATE TABLE IF NOT EXISTS user_credentials (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    password_hash TEXT NOT NULL,
    password_algorithm TEXT NOT NULL DEFAULT 'argon2id',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for fast user credential lookup
CREATE INDEX IF NOT EXISTS idx_user_credentials_user_id ON user_credentials(user_id);

-- Ensure one credential per user
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_credentials_one_per_user ON user_credentials(user_id);

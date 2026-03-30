-- Migration 007: Tunnel Management API - Add CRUD support
-- Adds timestamp columns for tracking tunnel creation and updates

-- Add created_at and updated_at columns to tunnels table
ALTER TABLE tunnels ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- Create index on hostname for faster uniqueness checks during create/update
CREATE INDEX IF NOT EXISTS idx_tunnels_hostname ON tunnels(hostname);

-- Create index on account_id for faster account-scoped queries
CREATE INDEX IF NOT EXISTS idx_tunnels_account_id ON tunnels(account_id);

-- Create index on status for filtering tunnels by status
CREATE INDEX IF NOT EXISTS idx_tunnels_status ON tunnels(status);

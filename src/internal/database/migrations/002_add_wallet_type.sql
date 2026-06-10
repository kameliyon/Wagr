-- +goose Up
-- Add wallet_type column to users table to support multiple wallet types

-- Add wallet_type column with default value for existing rows
ALTER TABLE users
ADD COLUMN wallet_type VARCHAR(20) NOT NULL DEFAULT 'midnight';

-- Create index for wallet_type lookups
CREATE INDEX IF NOT EXISTS idx_users_wallet_type ON users(wallet_type);

-- Add constraint to ensure only valid wallet types are used
ALTER TABLE users
ADD CONSTRAINT check_wallet_type CHECK (wallet_type IN ('midnight', 'hedera'));

-- Note: The unique constraint on wallet_address remains, as addresses
-- should still be unique across all wallet types. However, if needed in
-- the future, we could change this to a composite unique constraint on
-- (wallet_type, wallet_address) if the same address can appear across
-- different blockchain networks.

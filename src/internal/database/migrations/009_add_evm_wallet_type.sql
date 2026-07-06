-- +goose Up
-- Migrate any legacy 'midnight' wallet_type rows to 'hedera' before tightening the constraint
UPDATE users SET wallet_type = 'hedera' WHERE wallet_type = 'midnight';

-- Replace the constraint to support EVM wallets and drop the defunct 'midnight' type
ALTER TABLE users
DROP CONSTRAINT check_wallet_type;

ALTER TABLE users
ADD CONSTRAINT check_wallet_type CHECK (wallet_type IN ('hedera', 'evm'));

-- +goose Down
ALTER TABLE users
DROP CONSTRAINT check_wallet_type;

ALTER TABLE users
ADD CONSTRAINT check_wallet_type CHECK (wallet_type IN ('midnight', 'hedera'));

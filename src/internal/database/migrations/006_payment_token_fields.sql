ALTER TABLE league_members
  ADD COLUMN IF NOT EXISTS payment_token    VARCHAR(10)              DEFAULT NULL,
  ADD COLUMN IF NOT EXISTS transaction_hash VARCHAR(255)             DEFAULT NULL,
  ADD COLUMN IF NOT EXISTS paid_at          TIMESTAMP WITH TIME ZONE DEFAULT NULL;

ALTER TABLE league_members
  ADD CONSTRAINT chk_payment_token
    CHECK (payment_token IS NULL OR payment_token IN ('hbar', 'usdc'));

CREATE INDEX IF NOT EXISTS idx_league_members_payment_status
  ON league_members(league_id, payment_status);

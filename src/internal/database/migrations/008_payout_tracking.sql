-- +goose Up
-- Track end-of-season payout execution state on leagues
ALTER TABLE leagues
    ADD COLUMN IF NOT EXISTS payout_status VARCHAR(50) DEFAULT 'pending'
        CONSTRAINT chk_payout_status CHECK (payout_status IN ('pending', 'executed', 'failed')),
    ADD COLUMN IF NOT EXISTS payout_tx_hash VARCHAR(255) DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS payouts_executed_at TIMESTAMP WITH TIME ZONE DEFAULT NULL;

-- Store final rank and payout amounts per member for record-keeping
ALTER TABLE league_members
    ADD COLUMN IF NOT EXISTS final_rank INT DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS payout_amount_cents BIGINT DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS payout_tx_hash VARCHAR(255) DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS payout_paid_at TIMESTAMP WITH TIME ZONE DEFAULT NULL;

-- Weekly bonus payout events; UNIQUE constraint prevents double-paying the same roster in the same week
CREATE TABLE IF NOT EXISTS weekly_payout_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    league_id UUID NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    week INT NOT NULL,
    roster_id INT NOT NULL,
    platform_user_id VARCHAR(255) NOT NULL,
    payout_type VARCHAR(50) NOT NULL
        CONSTRAINT chk_weekly_payout_type CHECK (payout_type IN ('weekly_high_score', 'score_threshold')),
    points DECIMAL(10, 2) NOT NULL,
    amount_cents BIGINT NOT NULL,
    wallet_address VARCHAR(255) NOT NULL,
    tx_hash VARCHAR(255) DEFAULT NULL,
    executed_at TIMESTAMP WITH TIME ZONE DEFAULT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (league_id, week, roster_id, payout_type)
);

CREATE INDEX IF NOT EXISTS idx_weekly_payout_events_league_week ON weekly_payout_events (league_id, week);

CREATE OR REPLACE TRIGGER update_weekly_payout_events_updated_at
    BEFORE UPDATE ON weekly_payout_events
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

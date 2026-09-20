-- +goose Up
-- 'executing' marks a league claimed by the season-end job just before it sends the
-- payout transaction, so a crash or DB failure after the tx can't lead to a second payout.
ALTER TABLE leagues DROP CONSTRAINT IF EXISTS chk_payout_status;
ALTER TABLE leagues
    ADD CONSTRAINT chk_payout_status CHECK (payout_status IN ('pending', 'executing', 'executed', 'failed'));

-- The season-end job used to write status = 'complete' before checking escrow, stranding
-- unfunded leagues as complete + pending where the job never selects them again. Put them
-- back to in_season so the next season-end run retries the payout.
UPDATE leagues
SET status = 'in_season', updated_at = NOW()
WHERE status = 'complete'
  AND payout_status = 'pending'
  AND cancelled_at IS NULL;

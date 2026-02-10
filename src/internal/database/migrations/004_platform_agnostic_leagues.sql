-- Migration 004: Platform-Agnostic Fantasy Sports Integration
-- This migration transforms Sleeper-specific tables into platform-agnostic tables
-- that can support Sleeper, ESPN, Yahoo, and any future fantasy platforms.

-- ============================================================================
-- STEP 1: Create new platform-agnostic tables
-- ============================================================================

-- Platform profiles: Links WAGR users to their fantasy platform accounts
CREATE TABLE IF NOT EXISTS platform_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform VARCHAR(50) NOT NULL,           -- 'sleeper', 'espn', 'yahoo'
    platform_user_id VARCHAR(255) NOT NULL,  -- Platform-specific user ID
    platform_username VARCHAR(255),          -- Platform username
    display_name VARCHAR(255),
    avatar_url TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, platform, platform_user_id)
);

CREATE INDEX idx_platform_profiles_user_id ON platform_profiles(user_id);
CREATE INDEX idx_platform_profiles_platform ON platform_profiles(platform);
CREATE INDEX idx_platform_profiles_platform_user_id ON platform_profiles(platform, platform_user_id);

-- Leagues: Fantasy leagues imported into WAGR
CREATE TABLE IF NOT EXISTS leagues (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    platform VARCHAR(50) NOT NULL,              -- 'sleeper', 'espn', 'yahoo'
    platform_league_id VARCHAR(255) NOT NULL,   -- Platform-specific league ID
    name VARCHAR(255) NOT NULL,
    sport VARCHAR(50) NOT NULL,                 -- 'nfl', 'nba', 'mlb', etc.
    season VARCHAR(10) NOT NULL,                -- '2024', '2024-25', etc.
    status VARCHAR(50),                         -- 'pre_draft', 'in_season', 'complete'
    total_rosters INT,
    scoring_type VARCHAR(50),                   -- 'ppr', 'standard', 'half_ppr', etc.
    entry_fee_cents BIGINT DEFAULT 0,           -- Entry fee in cents (e.g., $50 = 5000)
    payout_structure JSONB,                     -- JSON array of payout rules
    blockchain_contract_address VARCHAR(255),   -- Smart contract address if deployed
    metadata JSONB DEFAULT '{}',                -- Platform-specific fields
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(platform, platform_league_id)
);

CREATE INDEX idx_leagues_platform ON leagues(platform);
CREATE INDEX idx_leagues_platform_league_id ON leagues(platform, platform_league_id);
CREATE INDEX idx_leagues_sport_season ON leagues(sport, season);

-- League members: Users who are members of imported leagues
CREATE TABLE IF NOT EXISTS league_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    league_id UUID NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,  -- NULL if not a WAGR user
    platform VARCHAR(50) NOT NULL,
    platform_user_id VARCHAR(255) NOT NULL,
    platform_username VARCHAR(255),
    display_name VARCHAR(255),
    avatar_url TEXT,
    is_owner BOOLEAN DEFAULT FALSE,             -- League commissioner/owner
    roster_id INT,                              -- Team/roster number in the league
    wins INT DEFAULT 0,
    losses INT DEFAULT 0,
    ties INT DEFAULT 0,
    total_points DECIMAL(10, 2) DEFAULT 0,
    wallet_address VARCHAR(255),                -- Their crypto wallet for payouts
    payment_status VARCHAR(50) DEFAULT 'unpaid', -- 'unpaid', 'paid', 'refunded'
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(league_id, platform_user_id)
);

CREATE INDEX idx_league_members_league_id ON league_members(league_id);
CREATE INDEX idx_league_members_user_id ON league_members(user_id);
CREATE INDEX idx_league_members_platform_user_id ON league_members(platform, platform_user_id);

-- Platform credentials: OAuth tokens and API keys for authenticated platforms
CREATE TABLE IF NOT EXISTS platform_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform VARCHAR(50) NOT NULL,
    access_token TEXT,
    refresh_token TEXT,
    token_expires_at TIMESTAMP WITH TIME ZONE,
    api_key TEXT,
    secret TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, platform)
);

CREATE INDEX idx_platform_credentials_user_id ON platform_credentials(user_id);

-- ============================================================================
-- STEP 2: Migrate existing Sleeper data (if old tables exist)
-- ============================================================================

-- Migrate sleeper_profiles to platform_profiles
DO $$
BEGIN
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'sleeper_profiles') THEN
        INSERT INTO platform_profiles (user_id, platform, platform_user_id, platform_username, display_name, avatar_url, created_at, updated_at)
        SELECT
            user_id,
            'sleeper' as platform,
            sleeper_user_id as platform_user_id,
            sleeper_username as platform_username,
            display_name,
            avatar_url,
            created_at,
            updated_at
        FROM sleeper_profiles
        ON CONFLICT (user_id, platform, platform_user_id) DO NOTHING;
    END IF;
END $$;

-- Migrate old leagues table to new leagues table (if old table exists with sleeper_league_id column)
DO $$
BEGIN
    IF EXISTS (
        SELECT FROM information_schema.columns
        WHERE table_name = 'leagues'
        AND column_name = 'sleeper_league_id'
    ) THEN
        -- Rename old leagues table temporarily
        ALTER TABLE leagues RENAME TO leagues_old;

        -- Recreate leagues table with new schema (already done above)

        -- Migrate data from old table
        INSERT INTO leagues (
            id, platform, platform_league_id, name, sport, season,
            entry_fee_cents, payout_structure, created_at, updated_at
        )
        SELECT
            id,
            'sleeper' as platform,
            sleeper_league_id as platform_league_id,
            name,
            sport,
            season,
            entry_fee_cents,
            payout_structure,
            created_at,
            updated_at
        FROM leagues_old
        ON CONFLICT (platform, platform_league_id) DO NOTHING;
    END IF;
END $$;

-- Migrate old league_members table (if it exists with old schema)
DO $$
BEGIN
    IF EXISTS (
        SELECT FROM information_schema.columns
        WHERE table_name = 'league_members'
        AND column_name = 'sleeper_user_id'
    ) THEN
        -- Rename old league_members table
        ALTER TABLE league_members RENAME TO league_members_old;

        -- Recreate league_members table (already done above)

        -- Migrate data
        INSERT INTO league_members (
            id, league_id, user_id, platform, platform_user_id,
            display_name, roster_id, wallet_address, payment_status,
            created_at, updated_at
        )
        SELECT
            id,
            league_id,
            user_id,
            'sleeper' as platform,
            sleeper_user_id as platform_user_id,
            display_name,
            roster_id,
            wallet_address,
            payment_status,
            created_at,
            updated_at
        FROM league_members_old
        ON CONFLICT (league_id, platform_user_id) DO NOTHING;
    END IF;
END $$;

-- ============================================================================
-- STEP 3: Rename old sleeper_profiles table for rollback safety
-- ============================================================================

DO $$
BEGIN
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'sleeper_profiles') THEN
        ALTER TABLE sleeper_profiles RENAME TO sleeper_profiles_old;
    END IF;
END $$;

-- ============================================================================
-- STEP 4: Create updated_at trigger for new tables
-- ============================================================================

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Drop triggers if they exist and recreate
DROP TRIGGER IF EXISTS update_platform_profiles_updated_at ON platform_profiles;
CREATE TRIGGER update_platform_profiles_updated_at
    BEFORE UPDATE ON platform_profiles
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_leagues_updated_at ON leagues;
CREATE TRIGGER update_leagues_updated_at
    BEFORE UPDATE ON leagues
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_league_members_updated_at ON league_members;
CREATE TRIGGER update_league_members_updated_at
    BEFORE UPDATE ON league_members
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_platform_credentials_updated_at ON platform_credentials;
CREATE TRIGGER update_platform_credentials_updated_at
    BEFORE UPDATE ON platform_credentials
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- STEP 5: Add comments for documentation
-- ============================================================================

COMMENT ON TABLE platform_profiles IS 'Links WAGR users to their fantasy platform accounts (Sleeper, ESPN, Yahoo)';
COMMENT ON TABLE leagues IS 'Fantasy leagues imported into WAGR for payment management';
COMMENT ON TABLE league_members IS 'Members of imported leagues with their stats and payment info';
COMMENT ON TABLE platform_credentials IS 'OAuth tokens and API keys for authenticated fantasy platforms';

COMMENT ON COLUMN leagues.entry_fee_cents IS 'Entry fee in cents (e.g., $50.00 = 5000 cents)';
COMMENT ON COLUMN leagues.payout_structure IS 'JSON array defining payout distribution (e.g., [{"place": 1, "amount_cents": 30000}, {"place": 2, "amount_cents": 15000}])';
COMMENT ON COLUMN league_members.payment_status IS 'Payment status: unpaid, paid, refunded';

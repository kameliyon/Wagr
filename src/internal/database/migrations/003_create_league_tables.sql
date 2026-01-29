-- Migration 003: Create league management tables
-- This migration adds tables for managing Sleeper fantasy league imports

-- Table: sleeper_profiles
-- Links WAGR users to their Sleeper accounts
CREATE TABLE IF NOT EXISTS sleeper_profiles (
    id SERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    sleeper_user_id TEXT NOT NULL UNIQUE,
    sleeper_username TEXT NOT NULL,
    sleeper_display_name TEXT,
    sleeper_avatar TEXT,
    linked_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Index for fast lookups by user_id
CREATE INDEX idx_sleeper_profiles_user_id ON sleeper_profiles(user_id);

-- Table: leagues
-- Stores imported Sleeper leagues
CREATE TABLE IF NOT EXISTS leagues (
    id SERIAL PRIMARY KEY,
    sleeper_league_id TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    season TEXT NOT NULL,
    sport TEXT NOT NULL DEFAULT 'nfl',
    status TEXT NOT NULL,
    total_rosters INTEGER NOT NULL,
    imported_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    imported_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_synced_at TIMESTAMP
);

-- Index for fast lookups by sleeper_league_id
CREATE INDEX idx_leagues_sleeper_league_id ON leagues(sleeper_league_id);

-- Index for filtering by season and status
CREATE INDEX idx_leagues_season_status ON leagues(season, status);

-- Table: league_members
-- Tracks league membership, linking leagues to users and Sleeper rosters
CREATE TABLE IF NOT EXISTS league_members (
    id SERIAL PRIMARY KEY,
    league_id INTEGER NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    sleeper_user_id TEXT NOT NULL,
    roster_id INTEGER,
    display_name TEXT,
    is_owner BOOLEAN NOT NULL DEFAULT FALSE,
    joined_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(league_id, sleeper_user_id)
);

-- Index for fast lookups by league_id
CREATE INDEX idx_league_members_league_id ON league_members(league_id);

-- Index for fast lookups by user_id
CREATE INDEX idx_league_members_user_id ON league_members(user_id);

-- Index for fast lookups by sleeper_user_id
CREATE INDEX idx_league_members_sleeper_user_id ON league_members(sleeper_user_id);

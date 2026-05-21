-- +goose Up
-- Migration 005: Add team_name to league_members
-- Stores the per-league custom team name (e.g. Sleeper metadata.team_name).
-- Falls back to display_name in the application layer when empty.

ALTER TABLE league_members
  ADD COLUMN IF NOT EXISTS team_name TEXT NOT NULL DEFAULT '';

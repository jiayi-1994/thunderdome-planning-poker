-- +goose Up
ALTER TABLE thunderdome.poker ALTER COLUMN voting_duration_seconds SET DEFAULT 300;
ALTER TABLE thunderdome.poker_story ALTER COLUMN voting_duration_seconds SET DEFAULT 300;

-- Upgrade the previous default for future rounds, preserving other configured durations.
-- Round snapshots are untouched so active deadlines and historical rounds stay unchanged.
UPDATE thunderdome.poker SET voting_duration_seconds = 300 WHERE voting_duration_seconds = 120;

-- +goose Down
ALTER TABLE thunderdome.poker ALTER COLUMN voting_duration_seconds SET DEFAULT 120;
ALTER TABLE thunderdome.poker_story ALTER COLUMN voting_duration_seconds SET DEFAULT 120;
-- Preserve configured durations on rollback because prior custom five-minute settings cannot be distinguished.

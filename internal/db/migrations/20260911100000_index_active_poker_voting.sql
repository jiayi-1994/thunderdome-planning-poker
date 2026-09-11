-- +goose Up
CREATE INDEX poker_story_active_votestart_idx ON thunderdome.poker_story (votestart_time) WHERE active;

-- +goose Down
DROP INDEX thunderdome.poker_story_active_votestart_idx;

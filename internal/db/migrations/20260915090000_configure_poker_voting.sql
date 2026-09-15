-- +goose Up
ALTER TABLE thunderdome.poker ADD COLUMN voting_duration_seconds integer NOT NULL DEFAULT 120
    CHECK (voting_duration_seconds BETWEEN 60 AND 3600 AND voting_duration_seconds % 60 = 0);
-- Snapshot the duration for each round. Existing rounds keep their original two minutes.
ALTER TABLE thunderdome.poker_story ADD COLUMN voting_duration_seconds integer NOT NULL DEFAULT 120
    CHECK (voting_duration_seconds BETWEEN 60 AND 3600);

-- +goose StatementBegin
CREATE FUNCTION thunderdome.snapshot_poker_voting_duration() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        SELECT voting_duration_seconds INTO NEW.voting_duration_seconds FROM thunderdome.poker WHERE id = NEW.poker_id;
    ELSIF (NEW.active AND NOT OLD.active) OR NEW.votestart_time IS DISTINCT FROM OLD.votestart_time THEN
        SELECT voting_duration_seconds INTO NEW.voting_duration_seconds FROM thunderdome.poker WHERE id = NEW.poker_id;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER snapshot_poker_voting_duration BEFORE INSERT OR UPDATE ON thunderdome.poker_story
FOR EACH ROW EXECUTE FUNCTION thunderdome.snapshot_poker_voting_duration();

-- Reduce selectable cards without touching votes, estimates, or completed story points.
ALTER TABLE thunderdome.poker ALTER COLUMN point_values_allowed SET DEFAULT ARRAY['0', '1/2', '1', '2', '3', '5', '8'];
UPDATE thunderdome.poker p SET point_values_allowed = COALESCE(
    (SELECT array_agg(value ORDER BY position) FROM unnest(p.point_values_allowed) WITH ORDINALITY AS card(value, position)
     WHERE value = ANY(ARRAY['0', '1/2', '1', '2', '3', '5', '8'])),
    ARRAY['0', '1/2', '1', '2', '3', '5', '8']
);
UPDATE thunderdome.estimation_scale SET values = ARRAY['0', '1/2', '1', '2', '3', '5', '8']
WHERE scale_type = 'thunderdome_default';

-- +goose Down
DROP TRIGGER snapshot_poker_voting_duration ON thunderdome.poker_story;
DROP FUNCTION thunderdome.snapshot_poker_voting_duration();
ALTER TABLE thunderdome.poker_story DROP COLUMN voting_duration_seconds;
ALTER TABLE thunderdome.poker DROP COLUMN voting_duration_seconds;
-- Keep the reduced deck on rollback; original user selections require the pre-deploy database backup.

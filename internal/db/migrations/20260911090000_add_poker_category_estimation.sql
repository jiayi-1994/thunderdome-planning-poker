-- +goose Up
ALTER TABLE thunderdome.poker_story ADD COLUMN estimation jsonb;

-- Clear the saved breakdown whenever voting is restarted by the existing procedure.
-- +goose StatementBegin
CREATE FUNCTION thunderdome.clear_poker_estimation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF (NEW.active AND NOT OLD.active) OR NEW.votestart_time IS DISTINCT FROM OLD.votestart_time THEN
        NEW.estimation = NULL;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER clear_poker_estimation BEFORE UPDATE ON thunderdome.poker_story
FOR EACH ROW EXECUTE FUNCTION thunderdome.clear_poker_estimation();

-- +goose Down
DROP TRIGGER clear_poker_estimation ON thunderdome.poker_story;
DROP FUNCTION thunderdome.clear_poker_estimation();
ALTER TABLE thunderdome.poker_story DROP COLUMN estimation;

-- +goose Up
ALTER TABLE thunderdome.poker_jira_sync DROP CONSTRAINT poker_jira_sync_status_check;
ALTER TABLE thunderdome.poker_jira_sync ADD CONSTRAINT poker_jira_sync_status_check
    CHECK (status IN ('awaiting_save', 'pending', 'succeeded', 'failed', 'skipped', 'cancelled'));

-- Old automatic tasks for unsaved rounds must not run after the upgrade.
UPDATE thunderdome.poker_jira_sync j SET status = 'awaiting_save', attempts = 0,
    last_error = '', updated_at = clock_timestamp()
FROM thunderdome.poker_story s
WHERE s.id = j.story_id AND s.votestart_time = j.vote_start_time AND s.points = ''
    AND NOT s.active AND NOT s.skipped AND j.status IN ('pending', 'failed');

-- Retain the revealed voters while waiting for the facilitator to click Save.
-- Only the transaction that saves points makes a task runnable.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION thunderdome.queue_poker_jira_sync() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    config thunderdome.poker_jira_settings;
    grouped boolean;
    saved boolean;
    participants jsonb;
BEGIN
    IF (NEW.active AND NOT OLD.active) OR NEW.votestart_time IS DISTINCT FROM OLD.votestart_time THEN
        DELETE FROM thunderdome.poker_jira_sync WHERE story_id = NEW.id;
        RETURN NEW;
    END IF;
    IF NEW.skipped THEN
        UPDATE thunderdome.poker_jira_sync SET status = 'cancelled', updated_at = clock_timestamp()
            WHERE story_id = NEW.id AND status IN ('awaiting_save', 'pending', 'failed');
        RETURN NEW;
    END IF;
    saved = NEW.points <> '' AND NEW.points IS DISTINCT FROM OLD.points;
    IF NEW.active OR NOT (saved OR
        (OLD.active AND NOT NEW.active AND NEW.voteend_time IS DISTINCT FROM OLD.voteend_time)
    ) THEN RETURN NEW; END IF;
    SELECT * INTO config FROM thunderdome.poker_jira_settings WHERE poker_id = NEW.poker_id AND enabled;
    IF NOT FOUND THEN RETURN NEW; END IF;
    grouped = EXISTS (
        SELECT 1 FROM jsonb_array_elements(NEW.votes) v WHERE v->>'category' IN ('testing', 'frontend', 'backend')
    );
    IF NOT grouped AND NOT saved THEN RETURN NEW; END IF;
    -- Preserve a successful write made before the upgrade when Save confirms the same value.
    IF saved AND EXISTS (
        SELECT 1 FROM thunderdome.poker_jira_sync WHERE story_id = NEW.id
        AND vote_start_time = NEW.votestart_time AND status = 'succeeded' AND points = NEW.points
        AND instance_id = config.instance_id AND field_id = config.field_id AND host = config.host
    ) THEN RETURN NEW; END IF;
    SELECT j.participants INTO participants FROM thunderdome.poker_jira_sync j
        WHERE j.story_id = NEW.id AND j.vote_start_time = NEW.votestart_time;
    IF participants IS NULL THEN
        SELECT coalesce(jsonb_agg(jsonb_build_object('id', user_id, 'spectator', spectator)), '[]'::jsonb)
            INTO participants FROM thunderdome.poker_user WHERE poker_id = NEW.poker_id;
    END IF;
    INSERT INTO thunderdome.poker_jira_sync (
        story_id, poker_id, instance_id, vote_start_time, issue_key, issue_link, host, field_id, votes, participants, points, status
    ) VALUES (
        NEW.id, NEW.poker_id, config.instance_id, NEW.votestart_time, coalesce(NEW.reference_id, ''),
        coalesce(NEW.link, ''), config.host, config.field_id, NEW.votes, participants,
        CASE WHEN saved THEN NEW.points ELSE '' END, CASE WHEN saved THEN 'pending' ELSE 'awaiting_save' END
    ) ON CONFLICT (story_id) DO UPDATE SET
        instance_id = EXCLUDED.instance_id, vote_start_time = EXCLUDED.vote_start_time,
        issue_key = EXCLUDED.issue_key, issue_link = EXCLUDED.issue_link, host = EXCLUDED.host,
        field_id = EXCLUDED.field_id, votes = EXCLUDED.votes, participants = EXCLUDED.participants,
        points = EXCLUDED.points, status = EXCLUDED.status, attempts = 0, last_error = '', next_attempt_at = now(), updated_at = now();
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- Do not turn unconfirmed results into writes during rollback.
UPDATE thunderdome.poker_jira_sync SET status = 'cancelled', updated_at = clock_timestamp() WHERE status = 'awaiting_save';
ALTER TABLE thunderdome.poker_jira_sync DROP CONSTRAINT poker_jira_sync_status_check;
ALTER TABLE thunderdome.poker_jira_sync ADD CONSTRAINT poker_jira_sync_status_check
    CHECK (status IN ('pending', 'succeeded', 'failed', 'skipped', 'cancelled'));

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION thunderdome.queue_poker_jira_sync() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    config thunderdome.poker_jira_settings;
    grouped boolean;
    participants jsonb;
BEGIN
    IF (NEW.active AND NOT OLD.active) OR NEW.votestart_time IS DISTINCT FROM OLD.votestart_time THEN
        DELETE FROM thunderdome.poker_jira_sync WHERE story_id = NEW.id;
        RETURN NEW;
    END IF;
    IF NEW.skipped THEN
        UPDATE thunderdome.poker_jira_sync SET status = 'cancelled', updated_at = clock_timestamp()
            WHERE story_id = NEW.id AND status IN ('pending', 'failed');
        RETURN NEW;
    END IF;
    IF NEW.active OR NOT (
        (OLD.active AND NOT NEW.active AND NEW.voteend_time IS DISTINCT FROM OLD.voteend_time) OR
        (NEW.points <> '' AND NEW.points IS DISTINCT FROM OLD.points)
    ) THEN
        RETURN NEW;
    END IF;
    SELECT * INTO config FROM thunderdome.poker_jira_settings WHERE poker_id = NEW.poker_id AND enabled;
    IF NOT FOUND THEN RETURN NEW; END IF;
    grouped = EXISTS (
        SELECT 1 FROM jsonb_array_elements(NEW.votes) v WHERE v->>'category' IN ('testing', 'frontend', 'backend')
    );
    -- Legacy voting still needs a facilitator's final point selection.
    IF NOT grouped AND NEW.points = '' THEN RETURN NEW; END IF;
    IF grouped AND EXISTS (
        SELECT 1 FROM thunderdome.poker_jira_sync WHERE story_id = NEW.id AND vote_start_time = NEW.votestart_time
    ) THEN RETURN NEW; END IF;
    SELECT coalesce(jsonb_agg(jsonb_build_object('id', user_id, 'spectator', spectator)), '[]'::jsonb)
        INTO participants FROM thunderdome.poker_user WHERE poker_id = NEW.poker_id;
    INSERT INTO thunderdome.poker_jira_sync (
        story_id, poker_id, instance_id, vote_start_time, issue_key, issue_link, host, field_id, votes, participants, points
    ) VALUES (
        NEW.id, NEW.poker_id, config.instance_id, NEW.votestart_time, coalesce(NEW.reference_id, ''),
        coalesce(NEW.link, ''), config.host, config.field_id, NEW.votes, participants,
        CASE WHEN grouped THEN '' ELSE NEW.points END
    ) ON CONFLICT (story_id) DO UPDATE SET
        instance_id = EXCLUDED.instance_id, vote_start_time = EXCLUDED.vote_start_time,
        issue_key = EXCLUDED.issue_key, issue_link = EXCLUDED.issue_link, host = EXCLUDED.host,
        field_id = EXCLUDED.field_id, votes = EXCLUDED.votes, participants = EXCLUDED.participants,
        points = EXCLUDED.points, status = 'pending', attempts = 0, last_error = '', next_attempt_at = now(), updated_at = now();
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

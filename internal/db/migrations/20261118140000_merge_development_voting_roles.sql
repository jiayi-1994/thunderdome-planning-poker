-- +goose Up
-- Merge development ballots on open rounds. The last ballot wins because SetVote appends updates.
UPDATE thunderdome.poker_story s
SET votes = (
    SELECT jsonb_agg(
        CASE WHEN category IN ('frontend', 'backend')
             THEN jsonb_set(ballot, '{category}', '"development"'::jsonb)
             ELSE ballot END
        ORDER BY position
    )
    FROM (
        SELECT ballot, position, ballot->>'category' AS category,
            max(position) FILTER (WHERE ballot->>'category' IN ('frontend', 'backend', 'development'))
                OVER (PARTITION BY ballot->>'warriorId') AS last_development
        FROM jsonb_array_elements(s.votes) WITH ORDINALITY AS current_ballots(ballot, position)
    ) votes
    WHERE category IS NULL OR category NOT IN ('frontend', 'backend', 'development')
        OR position = last_development
)
WHERE s.active AND EXISTS (
    SELECT 1 FROM jsonb_array_elements(s.votes) v WHERE v->>'category' IN ('frontend', 'backend')
);

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
        SELECT 1 FROM jsonb_array_elements(NEW.votes) v WHERE v->>'category' IN ('testing', 'development', 'frontend', 'backend')
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
-- Role merges are retained because a development ballot no longer records its original discipline.
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

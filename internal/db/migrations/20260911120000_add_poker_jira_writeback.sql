-- +goose Up
CREATE TABLE thunderdome.poker_jira_settings (
    poker_id uuid PRIMARY KEY REFERENCES thunderdome.poker(id) ON DELETE CASCADE,
    instance_id uuid NOT NULL REFERENCES thunderdome.jira_instance(id) ON DELETE CASCADE,
    field_id text NOT NULL CHECK (field_id ~ '^customfield_[0-9]+$'),
    field_name text NOT NULL,
    host text NOT NULL,
    enabled boolean NOT NULL DEFAULT true
);

-- One durable task per story. A new voting round replaces the previous task.
CREATE TABLE thunderdome.poker_jira_sync (
    story_id uuid PRIMARY KEY REFERENCES thunderdome.poker_story(id) ON DELETE CASCADE,
    poker_id uuid NOT NULL REFERENCES thunderdome.poker(id) ON DELETE CASCADE,
    instance_id uuid NOT NULL REFERENCES thunderdome.jira_instance(id) ON DELETE CASCADE,
    vote_start_time timestamptz NOT NULL,
    issue_key text NOT NULL,
    issue_link text NOT NULL,
    host text NOT NULL,
    field_id text NOT NULL,
    votes jsonb NOT NULL,
    participants jsonb NOT NULL,
    points text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'succeeded', 'failed', 'skipped', 'cancelled')),
    attempts integer NOT NULL DEFAULT 0,
    last_error text NOT NULL DEFAULT '',
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX poker_jira_sync_pending_idx ON thunderdome.poker_jira_sync(next_attempt_at) WHERE status = 'pending';

-- Enqueue in the same transaction that ends voting, including countdown expiry.
-- Snapshot voters so later participation changes cannot change the value on retry.
-- +goose StatementBegin
CREATE FUNCTION thunderdome.queue_poker_jira_sync() RETURNS trigger LANGUAGE plpgsql AS $$
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
CREATE TRIGGER queue_poker_jira_sync AFTER UPDATE ON thunderdome.poker_story
FOR EACH ROW EXECUTE FUNCTION thunderdome.queue_poker_jira_sync();

-- +goose Down
DROP TRIGGER queue_poker_jira_sync ON thunderdome.poker_story;
DROP FUNCTION thunderdome.queue_poker_jira_sync();
DROP TABLE thunderdome.poker_jira_sync;
DROP TABLE thunderdome.poker_jira_settings;

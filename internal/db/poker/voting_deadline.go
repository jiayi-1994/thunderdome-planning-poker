package poker

import (
	"context"
	"fmt"

	"github.com/StevenWeathers/thunderdome-planning-poker/thunderdome"
)

// EndExpiredStoryVoting closes expired rounds even when no clients remain connected.
// The database locks the exact rounds being expired and updates stories and games atomically.
func (d *Service) EndExpiredStoryVoting(ctx context.Context) ([]*thunderdome.PokerVotingExpiration, error) {
	rows, err := d.DB.QueryContext(ctx, `WITH expired AS MATERIALIZED (
		SELECT p.id AS poker_id, s.id AS story_id, s.votestart_time
		FROM thunderdome.poker_story s
		JOIN thunderdome.poker p ON p.id = s.poker_id AND p.active_story_id = s.id
		WHERE s.active AND NOT p.voting_locked AND p.end_time IS NULL
		AND s.votestart_time <= now() - ($1::double precision * interval '1 second')
		ORDER BY s.votestart_time LIMIT 100
		FOR UPDATE OF s, p SKIP LOCKED
	), ended_stories AS (
		UPDATE thunderdome.poker_story s
		SET active = false, voteend_time = e.votestart_time + ($1::double precision * interval '1 second'), updated_date = now()
		FROM expired e WHERE s.id = e.story_id
		RETURNING s.id
	)
	UPDATE thunderdome.poker p SET voting_locked = true, updated_date = now(), last_active = now()
	FROM expired e, ended_stories s WHERE p.id = e.poker_id AND s.id = e.story_id
	RETURNING p.id, e.story_id, e.votestart_time`, thunderdome.PokerVotingDuration.Seconds())
	if err != nil {
		return nil, fmt.Errorf("expire poker voting: %w", err)
	}
	expirations := make([]*thunderdome.PokerVotingExpiration, 0)
	for rows.Next() {
		event := &thunderdome.PokerVotingExpiration{}
		if err := rows.Scan(&event.PokerID, &event.StoryID, &event.VoteStartTime); err != nil {
			rows.Close()
			return nil, err
		}
		expirations = append(expirations, event)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	return expirations, nil
}

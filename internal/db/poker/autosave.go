package poker

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/StevenWeathers/thunderdome-planning-poker/thunderdome"
)

// autoSaveStory runs while the completion transaction still owns the story and game locks.
// Saving points queues Jira work through the same trigger used by manual Save.
func autoSaveStory(ctx context.Context, tx *sql.Tx, pokerID, storyID string, timedOut bool) error {
	var rawVotes, rawUsers []byte
	if err := tx.QueryRowContext(ctx, `SELECT s.votes,
		(SELECT coalesce(jsonb_agg(jsonb_build_object('id', user_id, 'active', active, 'spectator', spectator)), '[]'::jsonb)
		 FROM thunderdome.poker_user WHERE poker_id = s.poker_id)
		FROM thunderdome.poker_story s WHERE s.id = $1 AND s.poker_id = $2`, storyID, pokerID).Scan(&rawVotes, &rawUsers); err != nil {
		return err
	}
	var votes []*thunderdome.Vote
	var users []*thunderdome.PokerUser
	if err := json.Unmarshal(rawVotes, &votes); err != nil {
		return err
	}
	if err := json.Unmarshal(rawUsers, &users); err != nil {
		return err
	}
	points, estimation := thunderdome.PokerAutoSavePoints(votes, users, timedOut)
	if points == "" {
		return nil
	}
	var snapshot any
	if estimation != nil {
		encoded, err := json.Marshal(estimation)
		if err != nil {
			return err
		}
		snapshot = string(encoded)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE thunderdome.poker_story
		SET points = $3, estimation = $4::jsonb, updated_date = now()
		WHERE id = $1 AND poker_id = $2`, storyID, pokerID, points, snapshot); err != nil {
		return err
	}
	// Use the same participant snapshot for the saved total and asynchronous Jira calculation.
	if _, err := tx.ExecContext(ctx, `UPDATE thunderdome.poker_jira_sync SET participants = $3::jsonb
		WHERE story_id = $1 AND poker_id = $2`, storyID, pokerID, string(rawUsers)); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `UPDATE thunderdome.poker SET active_story_id = NULL,
		updated_date = now(), last_active = now() WHERE id = $1`, pokerID)
	return err
}

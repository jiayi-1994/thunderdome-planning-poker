package jira

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/StevenWeathers/thunderdome-planning-poker/internal/db"
	"github.com/StevenWeathers/thunderdome-planning-poker/thunderdome"
)

func (s *Service) GetPokerJiraSettings(ctx context.Context, pokerID string) (thunderdome.PokerJiraSettings, error) {
	var settings thunderdome.PokerJiraSettings
	err := s.DB.QueryRowContext(ctx, `SELECT enabled, instance_id, field_id, field_name, host
		FROM thunderdome.poker_jira_settings WHERE poker_id = $1`, pokerID).
		Scan(&settings.Enabled, &settings.InstanceID, &settings.FieldID, &settings.FieldName, &settings.Host)
	if errors.Is(err, sql.ErrNoRows) {
		return settings, nil
	}
	return settings, err
}

func (s *Service) SavePokerJiraSettings(ctx context.Context, pokerID, userID string, settings thunderdome.PokerJiraSettings) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Serialize configuration changes with voting completion and in-flight writes.
	rows, err := tx.QueryContext(ctx, `SELECT id FROM thunderdome.poker_story WHERE poker_id = $1 ORDER BY id FOR UPDATE`, pokerID)
	if err != nil {
		return err
	}
	for rows.Next() {
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if settings.Enabled {
		var owner string
		if err := tx.QueryRowContext(ctx, `SELECT user_id FROM thunderdome.jira_instance WHERE id = $1`, settings.InstanceID).Scan(&owner); err != nil || owner != userID {
			return fmt.Errorf("只能使用自己的 Jira 账号配置")
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO thunderdome.poker_jira_settings(poker_id, instance_id, field_id, field_name, host, enabled)
			VALUES ($1,$2,$3,$4,$5,true) ON CONFLICT (poker_id) DO UPDATE SET
			instance_id = EXCLUDED.instance_id, field_id = EXCLUDED.field_id, field_name = EXCLUDED.field_name,
			host = EXCLUDED.host, enabled = true`, pokerID, settings.InstanceID, settings.FieldID, settings.FieldName, settings.Host)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE thunderdome.poker_jira_settings SET enabled = false WHERE poker_id = $1`, pokerID)
	}
	if err != nil {
		return err
	}
	// Settings apply to future saves; never silently resend old estimates to a new target.
	_, err = tx.ExecContext(ctx, `UPDATE thunderdome.poker_jira_sync j SET status = 'cancelled', updated_at = clock_timestamp()
		WHERE j.poker_id = $1 AND j.status IN ('awaiting_save', 'pending', 'failed') AND NOT EXISTS (
			SELECT 1 FROM thunderdome.poker_jira_settings c WHERE c.poker_id = j.poker_id AND c.enabled
			AND c.instance_id = j.instance_id AND c.field_id = j.field_id AND c.host = j.host
		)`, pokerID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) RetryPokerJiraSync(ctx context.Context, pokerID, storyID string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM thunderdome.poker_story WHERE id = $1 AND poker_id = $2 FOR UPDATE`, storyID, pokerID).Scan(&id); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE thunderdome.poker_jira_sync j SET
		status = 'pending', attempts = 0, last_error = '', next_attempt_at = clock_timestamp(), updated_at = clock_timestamp(),
		issue_key = coalesce(s.reference_id, ''), issue_link = coalesce(s.link, '')
		FROM thunderdome.poker_story s, thunderdome.poker_jira_settings c
		WHERE j.story_id = $1 AND j.poker_id = $2 AND s.id = j.story_id
		AND NOT s.active AND NOT s.skipped AND s.points <> '' AND s.votestart_time = j.vote_start_time
		AND c.poker_id = j.poker_id AND c.enabled AND c.instance_id = j.instance_id
		AND c.field_id = j.field_id AND c.host = j.host AND j.status = 'failed'`, storyID, pokerID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("当前回写任务无法重试，请检查设置或重新评点")
	}
	return tx.Commit()
}

// ProcessPokerJiraSync processes one durable task. The story row is locked during the
// bounded (10 second) HTTP request so a restarted round cannot be overtaken by an old write.
// SKIP LOCKED lets other workers continue with other stories; no game-wide lock is held.
func (s *Service) ProcessPokerJiraSync(ctx context.Context, write func(context.Context, thunderdome.JiraInstance, thunderdome.PokerJiraWrite) error) (*thunderdome.PokerJiraSyncEvent, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	event := &thunderdome.PokerJiraSyncEvent{Sync: &thunderdome.PokerJiraSync{}}
	var request thunderdome.PokerJiraWrite
	var instance thunderdome.JiraInstance
	var votesJSON, participantsJSON []byte
	err = tx.QueryRowContext(ctx, `SELECT j.story_id, j.poker_id, j.vote_start_time, j.issue_key, j.issue_link,
		j.host, j.field_id, j.points, j.votes, j.participants, j.attempts,
		i.id, i.user_id, i.host, i.client_mail, i.access_token, i.jira_data_center, i.auth_method
		FROM thunderdome.poker_jira_sync j
		JOIN thunderdome.poker_story s ON s.id = j.story_id AND s.votestart_time = j.vote_start_time
			AND NOT s.active AND NOT s.skipped AND s.points <> ''
		JOIN thunderdome.poker_jira_settings c ON c.poker_id = j.poker_id AND c.enabled
			AND c.instance_id = j.instance_id AND c.field_id = j.field_id AND c.host = j.host
		JOIN thunderdome.jira_instance i ON i.id = j.instance_id
		WHERE j.status = 'pending' AND j.next_attempt_at <= now()
		ORDER BY j.next_attempt_at, j.story_id LIMIT 1 FOR UPDATE OF s SKIP LOCKED`).Scan(
		&event.StoryID, &event.PokerID, &event.VoteStartTime, &request.IssueKey, &request.Link,
		&request.Host, &request.FieldID, &request.Points, &votesJSON, &participantsJSON, &event.Sync.Attempts,
		&instance.ID, &instance.UserID, &instance.Host, &instance.ClientMail, &instance.AccessToken, &instance.JiraDataCenter, &instance.AuthMethod)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var votes []*thunderdome.Vote
	var participants []*thunderdome.PokerUser
	if err := json.Unmarshal(votesJSON, &votes); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(participantsJSON, &participants); err != nil {
		return nil, err
	}
	if estimation := thunderdome.CalculatePokerEstimation(votes, participants); estimation != nil {
		request.Points = estimation.Total
	}
	event.Sync.IssueKey = request.IssueKey
	event.Sync.Points = request.Points
	event.Sync.Status = "succeeded"
	event.Sync.Attempts++
	if _, valid := thunderdome.NumericPokerVote(request.Points); !valid {
		event.Sync.Status = "skipped"
		event.Sync.Error = "本轮没有有效数字评分，未回写 Jira"
	} else {
		token, tokenErr := db.Decrypt(instance.AccessToken, s.AESHashKey)
		if tokenErr != nil {
			err = fmt.Errorf("Jira 账号凭据无法读取，请重新配置")
		} else {
			instance.AccessToken = token
			err = write(ctx, instance, request)
		}
		if err != nil {
			event.Sync.Error = err.Error()
			event.Sync.Status = "pending"
			if event.Sync.Attempts >= 3 {
				event.Sync.Status = "failed"
			}
		}
	}
	delay := time.Duration(5*event.Sync.Attempts*event.Sync.Attempts) * time.Second
	err = tx.QueryRowContext(ctx, `UPDATE thunderdome.poker_jira_sync SET points = $2, status = $3,
		attempts = $4, last_error = $5, next_attempt_at = clock_timestamp() + ($6::double precision * interval '1 second'), updated_at = clock_timestamp()
		WHERE story_id = $1 RETURNING updated_at`, event.StoryID, event.Sync.Points, event.Sync.Status,
		event.Sync.Attempts, event.Sync.Error, delay.Seconds()).Scan(&event.Sync.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return event, nil
}

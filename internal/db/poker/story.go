package poker

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/StevenWeathers/thunderdome-planning-poker/thunderdome"

	"go.uber.org/zap"
)

// GetStories retrieves stories for given poker game
func (d *Service) GetStories(pokerID string, userID string) []*thunderdome.Story {
	var stories = make([]*thunderdome.Story, 0)
	users := d.GetUsers(pokerID)
	storyRows, storiesErr := d.DB.Query(
		`SELECT
			id, name, type, reference_id, link, description, acceptance_criteria, priority,
			points, active, skipped, votestart_time, voteend_time, votes, estimation,
			row_number() OVER (ORDER BY position ASC) as position,
			(SELECT jsonb_build_object('status', j.status, 'issueKey', j.issue_key, 'points', j.points,
				'error', j.last_error, 'attempts', j.attempts, 'updatedAt', j.updated_at)
			 FROM thunderdome.poker_jira_sync j WHERE j.story_id = poker_story.id
			 AND j.vote_start_time = poker_story.votestart_time AND NOT poker_story.active) AS jira_sync,
			(SELECT j.participants FROM thunderdome.poker_jira_sync j WHERE j.story_id = poker_story.id
			 AND j.vote_start_time = poker_story.votestart_time AND NOT poker_story.active) AS voting_participants,
			voting_duration_seconds
			FROM thunderdome.poker_story WHERE poker_id = $1 ORDER BY position
		`,
		pokerID,
	)
	if storiesErr == nil {
		defer storyRows.Close()
		for storyRows.Next() {
			var votingDurationSeconds int
			var v string
			var estimation []byte
			var jiraSync []byte
			var votingParticipants []byte
			var referenceID sql.NullString
			var link sql.NullString
			var description sql.NullString
			var acceptanceCriteria sql.NullString
			var p = &thunderdome.Story{
				Votes:   make([]*thunderdome.Vote, 0),
				Active:  false,
				Skipped: false,
			}
			if err := storyRows.Scan(
				&p.ID, &p.Name, &p.Type, &referenceID, &link, &description, &acceptanceCriteria, &p.Priority,
				&p.Points, &p.Active, &p.Skipped, &p.VoteStartTime, &p.VoteEndTime, &v, &estimation, &p.Position, &jiraSync, &votingParticipants, &votingDurationSeconds,
			); err != nil {
				d.Logger.Error("get poker stories query error", zap.Error(err),
					zap.String("PokerID", pokerID), zap.String("UserID", userID))
			} else {
				if len(jiraSync) > 0 {
					if err := json.Unmarshal(jiraSync, &p.JiraSync); err != nil {
						d.Logger.Error("decode poker Jira sync", zap.Error(err))
					}
				}
				p.VoteDeadline = p.VoteStartTime.Add(time.Duration(votingDurationSeconds) * time.Second)
				p.ReferenceID = referenceID.String
				p.Link = link.String
				p.Description = description.String
				p.AcceptanceCriteria = acceptanceCriteria.String
				err = json.Unmarshal([]byte(v), &p.Votes)
				if err != nil {
					d.Logger.Error("get poker stories query scan error", zap.Error(err),
						zap.String("PokerID", pokerID), zap.String("UserID", userID))
				}

				if !p.Active {
					if len(estimation) > 0 {
						if err := json.Unmarshal(estimation, &p.Estimation); err != nil {
							d.Logger.Error("decode poker estimation", zap.Error(err))
						}
					} else {
						participants := users
						if len(votingParticipants) > 0 {
							if err := json.Unmarshal(votingParticipants, &participants); err != nil {
								d.Logger.Error("decode poker voting participants", zap.Error(err))
							}
						}
						p.Estimation = thunderdome.CalculatePokerEstimation(p.Votes, participants)
					}
				}

				// don't send others vote values to client, prevent sneaky devs from peaking at votes
				for i := range p.Votes {
					if p.Active && p.Votes[i].UserID != userID {
						p.Votes[i].VoteValue = ""
					}
				}

				stories = append(stories, p)
			}
		}
	}

	return stories
}

// CreateStory adds a new story to the game
func (d *Service) CreateStory(pokerID string, name string, storyType string, referenceID string, link string, description string, acceptanceCriteria string, priority int32) ([]*thunderdome.Story, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tx, err := d.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	_, err = d.insertStory(ctx, tx, pokerID, &thunderdome.Story{
		Name: name, Type: storyType, ReferenceID: referenceID, Link: link,
		Description: description, AcceptanceCriteria: acceptanceCriteria, Priority: priority,
	})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return d.GetStories(pokerID, ""), nil
}

// ActivateStoryVoting sets the story by ID to active, wipes any previous votes/points, and disables votingLock
func (d *Service) ActivateStoryVoting(pokerID string, storyID string) ([]*thunderdome.Story, error) {
	if _, err := d.DB.Exec(
		`CALL thunderdome.poker_story_activate($1, $2);`, pokerID, storyID,
	); err != nil {
		d.Logger.Error("CALL thunderdome.poker_story_activate error", zap.Error(err),
			zap.String("PokerID", pokerID), zap.String("StoryID", storyID))
		return nil, fmt.Errorf("activate poker voting: %w", err)
	}

	stories := d.GetStories(pokerID, "")

	return stories, nil
}

// SetVote sets a users vote for the story
func (d *Service) SetVote(pokerID string, userID string, storyID string, voteValue string, category string) ([]*thunderdome.Story, bool, error) {
	if !thunderdome.ValidPokerPointValue(voteValue) {
		return nil, false, fmt.Errorf("请选择 0、1/2、1、2、3、5 或 8 分")
	}
	if category != "" {
		if !thunderdome.ValidPokerCategory(category) {
			return nil, false, fmt.Errorf("invalid vote category")
		}
	}
	var rawVotes []byte
	err := d.DB.QueryRow(
		`UPDATE thunderdome.poker_story s SET votes = (
			SELECT coalesce(jsonb_agg(v), '[]'::jsonb)
			FROM jsonb_array_elements(s.votes) v
			WHERE NOT (v->>'warriorId' = $2 AND coalesce(v->>'category', '') = $4)
		) || jsonb_build_array(jsonb_build_object('warriorId', $2::text, 'vote', $3::text, 'category', $4::text)),
		updated_date = NOW()
		FROM thunderdome.poker p
		WHERE s.id = $1 AND s.poker_id = $5 AND p.id = s.poker_id
		AND s.active AND NOT p.voting_locked AND p.active_story_id = s.id AND p.end_time IS NULL
		AND s.votestart_time + (s.voting_duration_seconds * interval '1 second') > clock_timestamp()
		AND $3 = ANY(p.point_values_allowed)
		AND EXISTS (SELECT 1 FROM thunderdome.poker_user u
			WHERE u.poker_id = p.id AND u.user_id::text = $2 AND NOT u.spectator)
		RETURNING s.votes`, storyID, userID, voteValue, category, pokerID).Scan(&rawVotes)
	if err != nil {
		return nil, false, fmt.Errorf("set poker vote: %w", err)
	}
	var votes []*thunderdome.Vote
	if err := json.Unmarshal(rawVotes, &votes); err != nil {
		return nil, false, err
	}
	allVoted := thunderdome.AllPokerUsersVoted(votes, d.GetUsers(pokerID))
	return d.GetStories(pokerID, ""), allVoted, nil
}

// RetractVote removes a users vote for the story
func (d *Service) RetractVote(pokerID string, userID string, storyID string, category string) ([]*thunderdome.Story, error) {
	if category != "" && !thunderdome.ValidPokerCategory(category) {
		return nil, fmt.Errorf("invalid vote category")
	}
	result, err := d.DB.Exec(
		`UPDATE thunderdome.poker_story s SET votes = (
			SELECT coalesce(jsonb_agg(v), '[]'::jsonb) FROM jsonb_array_elements(s.votes) v
			WHERE NOT (v->>'warriorId' = $2 AND coalesce(v->>'category', '') = $3)
		), updated_date = NOW()
		FROM thunderdome.poker p
		WHERE s.id = $1 AND s.poker_id = $4 AND p.id = s.poker_id
		AND s.active AND NOT p.voting_locked AND p.active_story_id = s.id AND p.end_time IS NULL
		AND s.votestart_time + (s.voting_duration_seconds * interval '1 second') > clock_timestamp()
		AND EXISTS (SELECT 1 FROM thunderdome.poker_user u
			WHERE u.poker_id = p.id AND u.user_id::text = $2 AND NOT u.spectator)`, storyID, userID, category, pokerID)
	if err != nil {
		return nil, fmt.Errorf("retract poker vote: %w", err)
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		return nil, fmt.Errorf("voting is not active")
	}

	stories := d.GetStories(pokerID, "")

	return stories, nil
}

// EndStoryVoting sets story to active: false
func (d *Service) EndStoryVoting(pokerID string, storyID string) ([]*thunderdome.Story, error) {
	if _, err := d.DB.Exec(
		`CALL thunderdome.poker_plan_voting_stop($1, $2);`, pokerID, storyID); err != nil {
		d.Logger.Error("CALL thunderdome.poker_plan_voting_stop error", zap.Error(err),
			zap.String("PokerID", pokerID), zap.String("StoryID", storyID))
		return nil, fmt.Errorf("end poker voting: %w", err)
	}

	stories := d.GetStories(pokerID, "")

	return stories, nil
}

// SkipStory sets story to active: false and unsets games activeStoryId
func (d *Service) SkipStory(pokerID string, storyID string) ([]*thunderdome.Story, error) {
	// Scope the update to the selected story. The legacy procedure marks every story
	// skipped, which would also cancel unrelated stories' pending Jira writes.
	result, err := d.DB.Exec(`WITH skipped AS (
		UPDATE thunderdome.poker_story s SET updated_date = now(), active = false, skipped = true, voteend_time = now()
		FROM thunderdome.poker p WHERE s.id = $2 AND s.poker_id = $1 AND p.id = s.poker_id
		AND p.active_story_id = s.id AND p.end_time IS NULL RETURNING s.id
	)
	UPDATE thunderdome.poker SET updated_date = now(), last_active = now(), voting_locked = true, active_story_id = NULL
	WHERE id = $1 AND EXISTS (SELECT 1 FROM skipped)`, pokerID, storyID)
	if err != nil {
		return nil, fmt.Errorf("skip poker story: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if count != 1 {
		return nil, fmt.Errorf("story is not the current story")
	}

	stories := d.GetStories(pokerID, "")

	return stories, nil
}

// UpdateStory updates the story by ID
func (d *Service) UpdateStory(pokerID string, storyID string, name string, storyType string, referenceID string, link string, description string, acceptanceCriteria string, priority int32) ([]*thunderdome.Story, error) {
	sanitizedDescription := d.HTMLSanitizerPolicy.Sanitize(description)
	sanitizedAcceptanceCriteria := d.HTMLSanitizerPolicy.Sanitize(acceptanceCriteria)
	// default priority should be 99 for sort order purposes
	if priority == 0 {
		priority = 99
	}
	// set PlanID to true
	if _, err := d.DB.Exec(
		`UPDATE thunderdome.poker_story
    SET
        updated_date = NOW(),
        name = $2,
        type = $3,
        reference_id = $4,
        link = $5,
        description = $6,
        acceptance_criteria = $7,
        priority = $8
    WHERE id = $1;`,
		storyID, name, storyType, referenceID, link, sanitizedDescription, sanitizedAcceptanceCriteria, priority); err != nil {
		d.Logger.Error("error getting poker story", zap.Error(err),
			zap.String("PokerID", pokerID), zap.String("StoryID", storyID))
	}

	stories := d.GetStories(pokerID, "")

	return stories, nil
}

// DeleteStory removes a story from the current game by ID
func (d *Service) DeleteStory(pokerID string, storyID string) ([]*thunderdome.Story, error) {
	if _, err := d.DB.Exec(
		`CALL thunderdome.poker_story_delete($1, $2);`, pokerID, storyID); err != nil {
		d.Logger.Error("CALL thunderdome.poker_story_delete error", zap.Error(err),
			zap.String("PokerID", pokerID), zap.String("StoryID", storyID))
	}

	stories := d.GetStories(pokerID, "")

	return stories, nil
}

// ArrangeStory sets the position of the story relative to the story it's being placed before
func (d *Service) ArrangeStory(pokerID string, storyID string, beforeStoryID string) ([]*thunderdome.Story, error) {
	if beforeStoryID == "" {
		_, err := d.DB.Exec(`UPDATE thunderdome.poker_story SET
			position = (SELECT max(position) FROM thunderdome.poker_story WHERE poker_id = $1) + 1
			WHERE id = $2;`,
			pokerID, storyID)
		if err != nil {
			d.Logger.Error("poker ArrangeStory get beforeStoryId error", zap.Error(err),
				zap.String("PokerID", pokerID), zap.String("StoryID", storyID))
		}
	} else {
		_, err := d.DB.Exec(
			`UPDATE thunderdome.poker_story SET position = (
			  -- find position of item referenced in before argument (default to 0)
			  with "before_position" as (
				select coalesce(
				  (select "position" from thunderdome.poker_story where id = $3),
				  -- in case item was not found, use last item in list and add 1 to add item to end of list
				  (select max("position") + 1 from thunderdome.poker_story where poker_id = $1),
				  -- in case no item exists, use 0
				  0
				) as "position"
			  ),
			  -- find position of previous item relative to "before item"
			  "before_prev_position" as (
				select coalesce(
				  (
					select w."position"
					from thunderdome.poker_story w, "before_position" b
					where
					  w.poker_id = $1 and
					  -- positions may not be integers, so we cannot simply deduct 1
					  -- this is why we find the first item with a smaller position
					  w."position" < b."position"
					order by "position" desc limit 1
				  ),
				  -- in case previous position does not exist (before item was first in list), simply deduct 1
				  (select b.position - 1 from "before_position" b)
				) as "position"
			  )
			  -- average both positions to fit new row into gap
			  select (b.position + p.position) / 2
			  from "before_position" b, "before_prev_position" p
			) WHERE id = $2;`,
			pokerID, storyID, beforeStoryID)
		if err != nil {
			d.Logger.Error("poker ArrangeStory error", zap.Error(err),
				zap.String("PokerID", pokerID), zap.String("StoryID", storyID),
				zap.String("BeforeStoryID", beforeStoryID))
		}
	}

	stories := d.GetStories(pokerID, "")

	return stories, nil
}

// FinalizeStory sets story to active: false and updates the points
func (d *Service) FinalizeStory(pokerID string, storyID string, points string) ([]*thunderdome.Story, error) {
	users := d.GetUsers(pokerID)
	tx, err := d.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var rawVotes, votingParticipants []byte
	if err := tx.QueryRow(`SELECT s.votes,
		(SELECT j.participants FROM thunderdome.poker_jira_sync j WHERE j.story_id = s.id AND j.vote_start_time = s.votestart_time)
		FROM thunderdome.poker_story s
		JOIN thunderdome.poker p ON p.id = s.poker_id
		WHERE s.id = $1 AND p.id = $2 AND p.active_story_id = s.id
		AND p.voting_locked AND NOT s.active AND p.end_time IS NULL FOR UPDATE OF p, s`,
		storyID, pokerID).Scan(&rawVotes, &votingParticipants); err != nil {
		return nil, fmt.Errorf("finalize poker story: %w", err)
	}
	var votes []*thunderdome.Vote
	if err := json.Unmarshal(rawVotes, &votes); err != nil {
		return nil, err
	}
	if len(votingParticipants) > 0 {
		if err := json.Unmarshal(votingParticipants, &users); err != nil {
			return nil, err
		}
	}
	if len(votes) == 0 {
		return nil, fmt.Errorf("cannot finalize a story without votes")
	}
	var snapshot any
	if estimation := thunderdome.CalculatePokerEstimation(votes, users); estimation != nil {
		if estimation.Total == "" {
			return nil, fmt.Errorf("at least one numeric vote is required")
		}
		points = estimation.Total
		encoded, err := json.Marshal(estimation)
		if err != nil {
			return nil, err
		}
		snapshot = string(encoded)
	}
	if _, err := tx.Exec(`UPDATE thunderdome.poker_story SET points = $3, estimation = $4::jsonb,
		active = false, updated_date = NOW() WHERE id = $1 AND poker_id = $2`, storyID, pokerID, points, snapshot); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`UPDATE thunderdome.poker SET active_story_id = NULL,
		updated_date = NOW(), last_active = NOW() WHERE id = $1`, pokerID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	stories := d.GetStories(pokerID, "")

	return stories, nil
}

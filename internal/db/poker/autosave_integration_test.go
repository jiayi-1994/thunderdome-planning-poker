package poker

import (
	"context"
	"database/sql"
	"testing"
)

func testPokerAutoSave(t *testing.T, database *sql.DB, svc *Service) {
	const game = "00000000-0000-0000-0000-000000000001"
	const story = "00000000-0000-0000-0000-000000000002"
	users := []string{
		"00000000-0000-0000-0000-000000000011", "00000000-0000-0000-0000-000000000012",
		"00000000-0000-0000-0000-000000000013", "00000000-0000-0000-0000-000000000014",
	}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := database.Exec(query, args...); err != nil {
			t.Fatal(err)
		}
	}
	original := svc.GetUsers(game)
	defer func() {
		for _, user := range original {
			exec(`UPDATE thunderdome.poker_user SET active = $3, spectator = $4 WHERE poker_id = $1 AND user_id = $2`, game, user.ID, user.Active, user.Spectator)
		}
	}()
	for _, tt := range []struct {
		name      string
		voters    int
		timedOut  bool
		manualEnd bool
		wantSaved bool
	}{
		{"manual with one of four", 1, false, true, true},
		{"timeout below half", 1, true, false, false},
		{"timeout exactly half", 2, true, false, true},
		{"timeout above half", 3, true, false, true},
		{"all voted", 4, false, true, true},
		{"late finish cannot bypass timeout quorum", 1, true, true, false},
		{"empty timeout", 0, true, false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			exec(`UPDATE thunderdome.poker_user SET active = true, spectator = false WHERE poker_id = $1`, game)
			if _, err := svc.ActivateStoryVoting(game, story); err != nil {
				t.Fatal(err)
			}
			for _, id := range users[:tt.voters] {
				if _, _, err := svc.SetVote(game, id, story, "0", "testing"); err != nil {
					t.Fatal(err)
				}
			}
			if tt.timedOut {
				exec(`UPDATE thunderdome.poker_story SET votestart_time = now() - interval '301 seconds' WHERE id = $1`, story)
			}
			if tt.manualEnd {
				if _, err := svc.EndStoryVoting(game, story); err != nil {
					t.Fatal(err)
				}
			} else if events, err := svc.EndExpiredStoryVoting(context.Background()); err != nil || len(events) != 1 {
				t.Fatalf("expected one expired round: %+v %v", events, err)
			}
			ended := svc.GetStories(game, users[0])[0]
			if ended.Active || (ended.Points == "0") != tt.wantSaved {
				t.Fatalf("unexpected saved result: %+v", ended)
			}
			var current sql.NullString
			var locked bool
			if err := database.QueryRow(`SELECT active_story_id, voting_locked FROM thunderdome.poker WHERE id = $1`, game).Scan(&current, &locked); err != nil {
				t.Fatal(err)
			}
			if !locked || current.Valid == tt.wantSaved {
				t.Fatal("saved rounds must clear the current story; unsaved rounds must retain manual controls")
			}
			if events, err := svc.EndExpiredStoryVoting(context.Background()); err != nil || len(events) != 0 {
				t.Fatal("completion was not idempotent", err)
			}
			if !tt.wantSaved && tt.voters > 0 {
				if plans, err := svc.FinalizeStory(game, story, "999"); err != nil || plans[0].Points != "0" {
					t.Fatal("low turnout must retain manual Save with server-calculated points", err)
				}
			}
		})
	}
}

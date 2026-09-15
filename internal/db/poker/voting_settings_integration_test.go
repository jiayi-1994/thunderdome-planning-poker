package poker

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func testPokerVotingSettings(t *testing.T, database *sql.DB, svc *Service) {
	const game = "00000000-0000-0000-0000-000000000001"
	const story = "00000000-0000-0000-0000-000000000002"
	const user = "00000000-0000-0000-0000-000000000011"
	deck := []string{"0", "1/2", "1", "2", "3", "5", "8"}
	save := func(seconds *int) error {
		return svc.UpdateGame(game, "Configured countdown", deck, false, "ceil", false, "", "", "", seconds)
	}
	defer func() {
		seconds := 120
		if err := save(&seconds); err != nil {
			t.Error(err)
		}
	}()
	for _, seconds := range []int{-60, 0, 59, 61, 3601, 3660} {
		if err := save(&seconds); err == nil {
			t.Fatalf("accepted invalid duration %d", seconds)
		}
	}
	seconds := 300
	if err := save(&seconds); err != nil {
		t.Fatal(err)
	}
	if err := save(nil); err != nil {
		t.Fatal(err)
	}
	gameState, err := svc.GetGameByID(game, user)
	if err != nil || gameState.VotingDurationSeconds != 300 {
		t.Fatalf("duration did not survive reload/legacy update: %+v %v", gameState, err)
	}
	if _, err := svc.ActivateStoryVoting(game, story); err != nil {
		t.Fatal(err)
	}
	// Rewind two minutes: a configured five-minute round must still accept votes.
	if _, err := database.Exec(`UPDATE thunderdome.poker_story SET votestart_time = now() - interval '121 seconds' WHERE id = $1`, story); err != nil {
		t.Fatal(err)
	}
	round := svc.GetStories(game, user)[0]
	if round.VoteDeadline.Sub(round.VoteStartTime) != 5*time.Minute {
		t.Fatal("round did not snapshot configured duration")
	}
	seconds = 60
	if err := save(&seconds); err != nil {
		t.Fatal(err)
	}
	if reloaded := svc.GetStories(game, user)[0]; !reloaded.VoteDeadline.Equal(round.VoteDeadline) {
		t.Fatal("editing settings changed an active deadline")
	}
	if _, _, err := svc.SetVote(game, user, story, "5", "testing"); err != nil {
		t.Fatal("round closed after old two-minute limit", err)
	}
	if _, err := svc.RetractVote(game, user, story, "testing"); err != nil {
		t.Fatal(err)
	}
	if expired, err := svc.EndExpiredStoryVoting(context.Background()); err != nil || len(expired) != 0 {
		t.Fatalf("round expired too early: %+v %v", expired, err)
	}
	if _, err := svc.ActivateStoryVoting(game, story); err != nil {
		t.Fatal(err)
	}
	if next := svc.GetStories(game, user)[0]; next.VoteDeadline.Sub(next.VoteStartTime) != time.Minute {
		t.Fatal("new round did not use revised duration")
	}
	if _, err := database.Exec(`UPDATE thunderdome.poker_story SET votestart_time = now() - interval '61 seconds' WHERE id = $1`, story); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.SetVote(game, user, story, "5", "testing"); err == nil {
		t.Fatal("late vote accepted")
	}
	if _, err := svc.RetractVote(game, user, story, "testing"); err == nil {
		t.Fatal("late retraction accepted")
	}
	if expired, err := svc.EndExpiredStoryVoting(context.Background()); err != nil || len(expired) != 1 {
		t.Fatalf("one-minute round did not expire: %+v %v", expired, err)
	}
	ended := svc.GetStories(game, user)[0]
	if ended.Active || !ended.VoteEndTime.Equal(ended.VoteDeadline) {
		t.Fatal("round did not end at its configured deadline")
	}
	// Restore the default for the remaining Jira regression scenarios.
	seconds = 120
	if err := save(&seconds); err != nil {
		t.Fatal(err)
	}
}

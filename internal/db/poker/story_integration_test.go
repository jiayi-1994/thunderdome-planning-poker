package poker

import (
	"database/sql"
	"os"
	"regexp"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.uber.org/zap"
)

// Run against an empty, disposable PostgreSQL database with POKER_TEST_DATABASE_URL.
// Refuse to touch a database that already contains application data.
func TestCategoryVotingDatabase(t *testing.T) {
	dsn := os.Getenv("POKER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set POKER_TEST_DATABASE_URL to an empty test database")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var exists bool
	if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name = 'thunderdome')`).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("test database must not contain the thunderdome schema")
	}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.Exec(query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`CREATE SCHEMA thunderdome;
		CREATE TABLE thunderdome.users (id uuid PRIMARY KEY, name text, type text DEFAULT 'GUEST', avatar text DEFAULT '', email text, picture text);
		CREATE TABLE thunderdome.poker (id uuid PRIMARY KEY, active_story_id uuid, voting_locked boolean DEFAULT false, end_time timestamptz, updated_date timestamptz DEFAULT now(), last_active timestamptz DEFAULT now());
		CREATE TABLE thunderdome.poker_user (poker_id uuid, user_id uuid, active boolean DEFAULT true, spectator boolean DEFAULT false);
		CREATE TABLE thunderdome.poker_story (id uuid PRIMARY KEY, poker_id uuid, name text DEFAULT '', type text DEFAULT '', reference_id text, link text, description text, acceptance_criteria text, priority integer DEFAULT 99, points varchar(8) DEFAULT '', active boolean DEFAULT true, skipped boolean DEFAULT false, votestart_time timestamptz DEFAULT now(), voteend_time timestamptz DEFAULT now(), votes jsonb DEFAULT '[]', position numeric DEFAULT 1, updated_date timestamptz DEFAULT now());`)
	defer db.Exec(`DROP SCHEMA thunderdome CASCADE`)
	migration, err := os.ReadFile("../migrations/20260911090000_add_poker_category_estimation.sql")
	if err != nil {
		t.Fatal(err)
	}
	exec(strings.Split(string(migration), "-- +goose Down")[0])
	procedures, err := os.ReadFile("../migrations/20230823233842_create_funcs_procs_triggers.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"poker_story_activate", "poker_plan_voting_stop"} {
		procedure := regexp.MustCompile(`(?s)CREATE OR REPLACE PROCEDURE thunderdome\.` + name + `\(.*?\$\$;`).FindString(string(procedures))
		if procedure == "" {
			t.Fatalf("procedure %s not found", name)
		}
		exec(procedure)
	}
	const game = "00000000-0000-0000-0000-000000000001"
	const story = "00000000-0000-0000-0000-000000000002"
	const otherGame = "00000000-0000-0000-0000-000000000003"
	const otherStory = "00000000-0000-0000-0000-000000000004"
	const one = "00000000-0000-0000-0000-000000000011"
	const two = "00000000-0000-0000-0000-000000000012"
	const three = "00000000-0000-0000-0000-000000000013"
	const observer = "00000000-0000-0000-0000-000000000014"
	exec(`INSERT INTO thunderdome.poker (id, active_story_id) VALUES ($1, $2), ($3, $4)`, game, story, otherGame, otherStory)
	exec(`INSERT INTO thunderdome.poker_story (id, poker_id) VALUES ($1, $2), ($3, $4)`, story, game, otherStory, otherGame)
	for _, id := range []string{one, two, three, observer} {
		exec(`INSERT INTO thunderdome.users (id, name) VALUES ($1::uuid, $1::text)`, id)
		exec(`INSERT INTO thunderdome.poker_user (poker_id, user_id, spectator) VALUES ($1, $2, $3)`, game, id, id == observer)
	}
	svc := &Service{DB: db, Logger: otelzap.New(zap.NewNop())}
	vote := func(user, category, value string) bool {
		t.Helper()
		_, allVoted, err := svc.SetVote(game, user, story, value, category)
		if err != nil {
			t.Fatal(err)
		}
		return allVoted
	}
	vote(one, "testing", "1")
	vote(one, "testing", "2")
	vote(two, "testing", "3")
	vote(one, "frontend", "5")
	if vote(three, "frontend", "?") {
		t.Fatal("auto-finish before backend has voted")
	}
	if _, err := svc.RetractVote(game, one, story, "frontend"); err != nil {
		t.Fatal(err)
	}
	restored := svc.GetStories(game, one)[0]
	if len(restored.Votes) != 3 {
		t.Fatalf("updating/retracting one category corrupted ballots: %+v", restored.Votes)
	}
	if restored.Estimation != nil {
		t.Fatal("live average leaked before reveal")
	}
	for _, ballot := range restored.Votes {
		if ballot.UserID != one && ballot.VoteValue != "" {
			t.Fatal("another participant's vote leaked")
		}
		if ballot.UserID == one && ballot.VoteValue != "2" {
			t.Fatal("own ballot did not survive reload")
		}
	}
	vote(one, "frontend", "5")
	vote(one, "backend", "3")
	vote(two, "backend", "5")
	if !vote(three, "backend", "8") {
		t.Fatal("complete disciplines and active participants should finish")
	}
	for _, invalid := range []struct{ game, user, story, value, category string }{
		{game, observer, story, "100", "testing"},
		{game, one, otherStory, "100", "testing"},
		{game, one, story, "NaN", "testing"},
		{game, one, story, "2", "unknown"},
	} {
		if _, _, err := svc.SetVote(invalid.game, invalid.user, invalid.story, invalid.value, invalid.category); err == nil {
			t.Fatalf("invalid vote accepted: %+v", invalid)
		}
	}
	if _, err := svc.FinalizeStory(game, story, "100"); err == nil {
		t.Fatal("finalized before reveal")
	}
	revealed, err := svc.EndStoryVoting(game, story)
	if err != nil {
		t.Fatal(err)
	}
	if revealed[0].Estimation.Total != "12.83" {
		t.Fatalf("unexpected estimate: %+v", revealed[0].Estimation)
	}
	if _, _, err := svc.SetVote(game, one, story, "100", "testing"); err == nil {
		t.Fatal("accepted vote after reveal")
	}
	if _, err := svc.RetractVote(game, one, story, "testing"); err == nil {
		t.Fatal("retracted vote after reveal")
	}
	finalized, err := svc.FinalizeStory(game, story, "999")
	if err != nil {
		t.Fatal(err)
	}
	if finalized[0].Points != "12.83" {
		t.Fatal("trusted client total instead of calculating category averages")
	}
	exec(`UPDATE thunderdome.poker_user SET spectator = true WHERE user_id = $1`, two)
	reloaded := (&Service{DB: db, Logger: svc.Logger}).GetStories(game, one)[0]
	if reloaded.Estimation.Total != "12.83" || reloaded.Estimation.Categories[0].Count != 2 {
		t.Fatal("historical breakdown changed with spectator status")
	}
	restarted, err := svc.ActivateStoryVoting(game, story)
	if err != nil {
		t.Fatal(err)
	}
	if restarted[0].Points != "" || restarted[0].Estimation != nil || len(restarted[0].Votes) != 0 {
		t.Fatal("restart did not clear all ballots and the saved breakdown")
	}
	if _, err := svc.EndStoryVoting(game, story); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.FinalizeStory(game, story, "0"); err == nil {
		t.Fatal("finalized without any ballots")
	}
	if _, err := svc.ActivateStoryVoting(game, story); err != nil {
		t.Fatal(err)
	}
	vote(one, "testing", "0")
	if _, err := svc.EndStoryVoting(game, story); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.FinalizeStory(game, story, "0"); err == nil {
		t.Fatal("missing categories were treated as zero")
	}
	// Verify the migration is reversible after actual data has been saved.
	exec(strings.Split(string(migration), "-- +goose Down")[1])
}

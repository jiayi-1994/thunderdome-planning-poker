package poker

import (
	"context"
	"database/sql"
	"sync"
	"testing"

	"github.com/StevenWeathers/thunderdome-planning-poker/thunderdome"
	"github.com/microcosm-cc/bluemonday"
)

func testPokerStoryImport(t *testing.T, database *sql.DB, poker *Service) {
	t.Helper()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := database.Exec(query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`ALTER TABLE thunderdome.poker_story ALTER COLUMN id SET DEFAULT gen_random_uuid(), ALTER COLUMN active SET DEFAULT false`)
	poker.HTMLSanitizerPolicy = bluemonday.UGCPolicy()
	const game = "00000000-0000-0000-0000-000000000080"
	const other = "00000000-0000-0000-0000-000000000081"
	exec(`INSERT INTO thunderdome.poker(id) VALUES ($1), ($2)`, game, other)
	add := func(target, key, link string) error {
		_, err := poker.CreateStory(target, "Imported story", "Story", key, link, "", "", 99)
		return err
	}
	count := func(target string) int {
		t.Helper()
		var n int
		if err := database.QueryRow(`SELECT count(*) FROM thunderdome.poker_story WHERE poker_id=$1`, target).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	t.Run("repeat import preserves saved story", func(t *testing.T) {
		if err := add(game, "TEST-1", "https://jira.example.com/jira/browse/TEST-1"); err != nil {
			t.Fatal(err)
		}
		exec(`UPDATE thunderdome.poker_story SET name='Original', points='5', votes='[{"warriorId":"one","vote":"5"}]' WHERE poker_id=$1`, game)
		if err := add(game, " test-1 ", "https://JIRA.example.com/jira/browse/test-1/?source=board#details"); err != nil {
			t.Fatal(err)
		}
		if got := count(game); got != 1 {
			t.Fatalf("repeat import created %d stories, want 1", got)
		}
		var preserved bool
		if err := database.QueryRow(`SELECT name='Original' AND points='5' AND votes->0->>'vote'='5' FROM thunderdome.poker_story WHERE poker_id=$1`, game).Scan(&preserved); err != nil || !preserved {
			t.Fatalf("existing result changed: %v", err)
		}
	})
	t.Run("concurrent imports are idempotent", func(t *testing.T) {
		database.SetMaxOpenConns(8)
		defer database.SetMaxOpenConns(1)
		var wg sync.WaitGroup
		start := make(chan struct{})
		errors := make(chan error, 8)
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				errors <- add(other, "TEST-2", "https://jira.example.com/browse/TEST-2")
			}()
		}
		close(start)
		wg.Wait()
		close(errors)
		for err := range errors {
			if err != nil {
				t.Fatal(err)
			}
		}
		if got := count(other); got != 1 {
			t.Fatalf("concurrent import created %d stories, want 1", got)
		}
	})
	t.Run("different Jira instances and meetings remain independent", func(t *testing.T) {
		before := count(other)
		for _, link := range []string{"https://other.example.com/browse/TEST-2", "https://jira.example.com/second/browse/TEST-2", "https://jira.example.com/jira/browse/TEST-1"} {
			key := "TEST-2"
			if link == "https://jira.example.com/jira/browse/TEST-1" {
				key = "TEST-1"
			}
			if err := add(other, key, link); err != nil {
				t.Fatal(err)
			}
		}
		if got := count(other); got != before+3 {
			t.Fatalf("distinct imports collapsed: %d", got)
		}
	})
	t.Run("unlinked manual stories are not deduplicated by title", func(t *testing.T) {
		before := count(other)
		for range 2 {
			if err := add(other, "", ""); err != nil {
				t.Fatal(err)
			}
		}
		if got := count(other); got != before+2 {
			t.Fatalf("manual stories collapsed: %d", got)
		}
	})
	t.Run("missing meeting returns an error", func(t *testing.T) {
		if err := add("00000000-0000-0000-0000-000000000099", "TEST-1", "https://jira.example.com/browse/TEST-1"); err == nil {
			t.Fatal("failed insertion was reported as successful")
		}
	})
	exec(`ALTER TABLE thunderdome.poker ADD COLUMN owner_id uuid, ALTER COLUMN id SET DEFAULT gen_random_uuid();
		ALTER TABLE thunderdome.poker_story ADD CONSTRAINT test_reject_story CHECK (name <> 'Reject import')`)
	for _, team := range []bool{false, true} {
		scope := "personal"
		if team {
			scope = "team"
		}
		t.Run(scope+" creation deduplicates the initial batch and rolls back failed imports", func(t *testing.T) {
			create := func(name string, stories []*thunderdome.Story) (*thunderdome.Poker, error) {
				const owner = "00000000-0000-0000-0000-000000000011"
				const scale = "00000000-0000-0000-0000-000000000050"
				if team {
					return poker.TeamCreateGame(context.Background(), other, owner, name, scale, []string{"1", "2"}, stories, true, "ceil", "", "", false)
				}
				return poker.CreateGame(context.Background(), owner, name, scale, []string{"1", "2"}, stories, true, "ceil", "", "", false)
			}
			game, err := create("Import batch", []*thunderdome.Story{
				{Name: "Original", ReferenceID: "TEST-1", Link: "https://jira.example.com/browse/TEST-1"},
				{Name: "Duplicate", ReferenceID: "TEST-1", Link: "https://jira.example.com/browse/TEST-1"},
				{Name: "Other", ReferenceID: "TEST-2", Link: "https://jira.example.com/browse/TEST-2"},
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(game.Stories) != 2 || count(game.ID) != 2 || game.Stories[0].Name != "Original" {
				t.Fatal("initial batch was not deduplicated")
			}
			if _, err := create("Failed import "+scope, []*thunderdome.Story{{Name: "First"}, {Name: "Reject import"}}); err == nil {
				t.Fatal("failed initial import was accepted")
			}
			var leftovers int
			if err := database.QueryRow(`SELECT count(*) FROM thunderdome.poker WHERE name=$1`, "Failed import "+scope).Scan(&leftovers); err != nil || leftovers != 0 {
				t.Fatalf("partial meeting left after failed import: %d, %v", leftovers, err)
			}
		})
	}
}

package poker

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	jiraclient "github.com/StevenWeathers/thunderdome-planning-poker/internal/atlassian/jira"
	jiradb "github.com/StevenWeathers/thunderdome-planning-poker/internal/db/jira"
	"github.com/StevenWeathers/thunderdome-planning-poker/thunderdome"
)

func testPokerJiraWriteback(t *testing.T, database *sql.DB, poker *Service) {
	t.Helper()
	const game = "00000000-0000-0000-0000-000000000001"
	const story = "00000000-0000-0000-0000-000000000002"
	const otherGame = "00000000-0000-0000-0000-000000000003"
	const one = "00000000-0000-0000-0000-000000000011"
	const two = "00000000-0000-0000-0000-000000000012"
	const three = "00000000-0000-0000-0000-000000000013"
	const key = "01234567890123456789012345678901"
	ctx := context.Background()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := database.Exec(query, args...); err != nil {
			t.Fatal(err)
		}
	}
	var written []float64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/2/issue/TEST-1" || r.Method != "PUT" {
			t.Errorf("unexpected Jira target %s %s", r.Method, r.URL.Path)
		}
		user, token, ok := r.BasicAuth()
		if !ok || user != "jira.user" || token != "test-token" {
			t.Error("saved credentials were not decrypted for Jira")
		}
		var payload struct {
			Fields map[string]float64 `json:"fields"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		written = append(written, payload.Fields["customfield_10016"])
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	jira := &jiradb.Service{DB: database, Logger: poker.Logger, AESHashKey: key}
	instance, err := jira.CreateInstance(ctx, one, server.URL, "jira.user", "test-token", true, "basic")
	if err != nil {
		t.Fatal(err)
	}
	stored, err := jira.GetInstanceByID(ctx, instance.ID)
	if err != nil || stored.AuthMethod != "basic" || !stored.JiraDataCenter || stored.AccessToken != "test-token" {
		t.Fatalf("Server credentials did not survive storage: %v", err)
	}
	var encrypted string
	if err := database.QueryRow(`SELECT access_token FROM thunderdome.jira_instance WHERE id = $1`, instance.ID).Scan(&encrypted); err != nil || encrypted == "test-token" {
		t.Fatal("Jira password was not encrypted at rest")
	}
	updated, err := jira.UpdateInstance(ctx, instance.ID, server.URL, "jira.user", "test-token", "")
	if err != nil || updated.AuthMethod != "basic" || !updated.JiraDataCenter {
		t.Fatalf("update omitted authentication and lost the saved method: %v", err)
	}
	listed, err := jira.FindInstancesByUserID(ctx, one)
	if err != nil || len(listed) != 1 || listed[0].AuthMethod != "basic" {
		t.Fatalf("list lost authentication method: %v", err)
	}
	settings := thunderdome.PokerJiraSettings{Enabled: true, InstanceID: instance.ID, FieldID: "customfield_10016", FieldName: "Story Points", Host: server.URL}
	if err := jira.SavePokerJiraSettings(ctx, game, two, settings); err == nil {
		t.Fatal("another user's Jira connection was accepted")
	}
	if err := jira.SavePokerJiraSettings(ctx, game, one, settings); err != nil {
		t.Fatal(err)
	}
	exec(`UPDATE thunderdome.poker_story SET reference_id = 'TEST-1', link = $2 WHERE id = $1`, story, server.URL+"/browse/TEST-1")
	exec(`UPDATE thunderdome.poker_user SET spectator = false WHERE user_id = $1`, two)
	start := func() {
		t.Helper()
		if _, err := poker.ActivateStoryVoting(game, story); err != nil {
			t.Fatal(err)
		}
	}
	vote := func(user, category, points string) {
		t.Helper()
		if _, _, err := poker.SetVote(game, user, story, points, category); err != nil {
			t.Fatal(err)
		}
	}
	end := func() {
		t.Helper()
		if _, err := poker.EndStoryVoting(game, story); err != nil {
			t.Fatal(err)
		}
	}
	save := func() *thunderdome.Story {
		t.Helper()
		plans, err := poker.FinalizeStory(game, story, "999")
		if err != nil {
			t.Fatal(err)
		}
		return plans[0]
	}
	process := func() *thunderdome.PokerJiraSyncEvent {
		t.Helper()
		// Re-create the service to verify tasks survive process restarts.
		fresh := &jiradb.Service{DB: database, Logger: poker.Logger, AESHashKey: key}
		event, err := fresh.ProcessPokerJiraSync(ctx, jiraclient.WritePoints)
		if err != nil {
			t.Fatal(err)
		}
		return event
	}

	t.Run("manual completion waits for Save and writes the confirmed total once", func(t *testing.T) {
		start()
		vote(one, "testing", "2")
		vote(two, "testing", "3")
		vote(one, "frontend", "5")
		vote(one, "backend", "3")
		vote(two, "backend", "5")
		vote(three, "backend", "8")
		end()
		plan := poker.GetStories(game, one)[0]
		if plan.JiraSync == nil || plan.JiraSync.Status != "awaiting_save" {
			t.Fatal("ending voting did not wait for Save")
		}
		if event := process(); event != nil || len(written) != 0 {
			t.Fatal("ending voting wrote to Jira before Save")
		}
		if err := jira.RetryPokerJiraSync(ctx, game, story); err == nil {
			t.Fatal("retry bypassed Save")
		}
		// Changing attendance after the vote must not alter the result sent or displayed.
		exec(`UPDATE thunderdome.poker_user SET spectator = true WHERE user_id = $1`, two)
		if total := poker.GetStories(game, one)[0].Estimation.Total; total != "12.83" {
			t.Fatalf("revealed result changed after attendance update: %s", total)
		}
		if plan := save(); plan.Points != "12.83" || plan.JiraSync == nil || plan.JiraSync.Status != "pending" {
			t.Fatal("Save did not atomically queue the confirmed total")
		}
		event := process()
		if event == nil || event.Sync.Status != "succeeded" || event.Sync.Points != "12.83" || len(written) != 1 || written[0] != 12.83 {
			t.Fatalf("wrong writeback result: %+v, %v", event, written)
		}
		if next := process(); next != nil {
			t.Fatal("successful writeback was duplicated")
		}
		if _, err := poker.FinalizeStory(game, story, "999"); err == nil {
			t.Fatal("a duplicate Save was accepted")
		}
		if next := process(); next != nil {
			t.Fatal("saving the same total resent the Jira update")
		}
	})

	t.Run("countdown completion waits for Save and excludes nonvoters", func(t *testing.T) {
		start()
		vote(one, "testing", "5")
		exec(`UPDATE thunderdome.poker_story SET votestart_time = now() - interval '121 seconds' WHERE id = $1`, story)
		if _, err := poker.EndExpiredStoryVoting(ctx); err != nil {
			t.Fatal(err)
		}
		if event := process(); event != nil {
			t.Fatal("countdown expiry wrote to Jira before Save")
		}
		save()
		event := process()
		if event == nil || event.Sync.Points != "5" || event.Sync.Status != "succeeded" || written[len(written)-1] != 5 {
			t.Fatalf("countdown did not write partial participation total: %+v", event)
		}
	})

	t.Run("automatic completion still requires Save", func(t *testing.T) {
		users := poker.GetUsers(game)
		defer func() {
			for _, user := range users {
				exec(`UPDATE thunderdome.poker_user SET spectator = $2 WHERE poker_id = $3 AND user_id = $1`, user.ID, user.Spectator, game)
			}
		}()
		exec(`UPDATE thunderdome.poker_user SET spectator = (user_id <> $1) WHERE poker_id = $2`, one, game)
		start()
		if _, allVoted, err := poker.SetVote(game, one, story, "3", "testing"); err != nil || !allVoted {
			t.Fatal("automatic completion condition was not met", err)
		}
		// The WebSocket auto-finish handler uses this same completion operation.
		end()
		if event := process(); event != nil {
			t.Fatal("automatic completion wrote to Jira before Save")
		}
		save()
		if event := process(); event == nil || event.Sync.Points != "3" || event.Sync.Status != "succeeded" {
			t.Fatal("Save did not write the auto-finished estimate")
		}
	})

	t.Run("legacy final point selection also requires Save", func(t *testing.T) {
		start()
		vote(one, "", "3")
		end()
		if event := process(); event != nil {
			t.Fatal("legacy completion wrote to Jira before Save")
		}
		if _, err := poker.FinalizeStory(game, story, "5"); err != nil {
			t.Fatal(err)
		}
		if event := process(); event == nil || event.Sync.Points != "5" || event.Sync.Status != "succeeded" {
			t.Fatal("legacy Save did not write the selected points")
		}
	})

	t.Run("upgrade parks unsaved work and guards processing and retry", func(t *testing.T) {
		migration, err := os.ReadFile("../migrations/20260915150000_require_save_for_jira_writeback.sql")
		if err != nil {
			t.Fatal(err)
		}
		for _, status := range []string{"pending", "failed"} {
			start()
			vote(one, "testing", "2")
			end()
			// Simulate a task left by the old automatic completion trigger.
			exec(`UPDATE thunderdome.poker_jira_sync SET status = $2, attempts = 2 WHERE story_id = $1`, story, status)
			if event := process(); event != nil {
				t.Fatal("worker accepted an unsaved legacy task")
			}
			if err := jira.RetryPokerJiraSync(ctx, game, story); err == nil {
				t.Fatal("retry accepted an unsaved legacy task")
			}
			exec(strings.Split(string(migration), "-- +goose Down")[0])
			plan := poker.GetStories(game, one)[0]
			if plan.JiraSync == nil || plan.JiraSync.Status != "awaiting_save" || plan.JiraSync.Attempts != 0 {
				t.Fatal("upgrade did not park the old task")
			}
			if event := process(); event != nil {
				t.Fatal("upgraded task ran before Save")
			}
			save()
			if event := process(); event == nil || event.Sync.Status != "succeeded" || event.Sync.Points != "2" {
				t.Fatal("upgraded task did not run after Save")
			}
		}
		start()
		vote(one, "testing", "2")
		end()
		exec(`UPDATE thunderdome.poker_jira_sync SET status = 'succeeded', points = '2', attempts = 1 WHERE story_id = $1`, story)
		exec(strings.Split(string(migration), "-- +goose Down")[0])
		if plan := save(); plan.JiraSync.Status != "succeeded" {
			t.Fatal("upgrade discarded a previously successful write")
		}
		if event := process(); event != nil {
			t.Fatal("Save duplicated the same successful pre-upgrade write")
		}
	})

	t.Run("old round is cancelled before a new round", func(t *testing.T) {
		start()
		vote(one, "testing", "8")
		end()
		save()
		start()
		if event := process(); event != nil {
			t.Fatal("a previous round wrote after restart")
		}
		if plan := poker.GetStories(game, one)[0]; plan.JiraSync != nil {
			t.Fatal("old sync result appeared in new round")
		}
		vote(one, "testing", "2")
		end()
		save()
		if event := process(); event == nil || event.Sync.Points != "2" {
			t.Fatalf("new round was not written: %+v", event)
		}
	})

	t.Run("zero is written and abstention never overwrites Jira", func(t *testing.T) {
		start()
		vote(one, "testing", "0")
		end()
		save()
		if event := process(); event == nil || event.Sync.Status != "succeeded" || written[len(written)-1] != 0 {
			t.Fatal("numeric zero was not written")
		}
		count := len(written)
		start()
		// Historical abstentions must still be handled safely after removing the card.
		exec(`UPDATE thunderdome.poker_story SET votes = jsonb_build_array(jsonb_build_object('warriorId', $2::text, 'category', 'testing', 'vote', '?')) WHERE id = $1`, story, one)
		end()
		if _, err := poker.FinalizeStory(game, story, "0"); err == nil {
			t.Fatal("Save accepted a round without numeric votes")
		}
		if event := process(); event != nil || len(written) != count {
			t.Fatal("abstention overwrote Jira")
		}
		start()
		end()
		if event := process(); event != nil {
			t.Fatal("empty voting round was queued")
		}
	})

	t.Run("failures persist and retry the same snapshot", func(t *testing.T) {
		start()
		vote(one, "testing", "8")
		end()
		save()
		for attempt := 1; attempt <= 3; attempt++ {
			event, err := jira.ProcessPokerJiraSync(ctx, func(_ context.Context, _ thunderdome.JiraInstance, write thunderdome.PokerJiraWrite) error {
				if write.Points != "8" {
					t.Errorf("retry changed the estimate to %s", write.Points)
				}
				return fmt.Errorf("Jira 账号没有编辑权限（403）")
			})
			if err != nil || event == nil || event.Sync.Attempts != attempt {
				t.Fatalf("attempt not persisted: %+v %v", event, err)
			}
			expected := "pending"
			if attempt == 3 {
				expected = "failed"
			}
			if event.Sync.Status != expected {
				t.Fatalf("unexpected status %s", event.Sync.Status)
			}
			if attempt < 3 {
				if event := process(); event != nil {
					t.Fatal("retried without waiting for backoff")
				}
				exec(`UPDATE thunderdome.poker_jira_sync SET next_attempt_at = now() WHERE story_id = $1`, story)
			}
		}
		if plan := poker.GetStories(game, one)[0]; plan.JiraSync == nil || plan.JiraSync.Status != "failed" || plan.Estimation.Total != "8" {
			t.Fatal("failure lost local results or was not visible")
		}
		if err := jira.RetryPokerJiraSync(ctx, otherGame, story); err == nil {
			t.Fatal("retried a story belonging to another game")
		}
		if err := jira.RetryPokerJiraSync(ctx, game, story); err != nil {
			t.Fatal(err)
		}
		if event := process(); event == nil || event.Sync.Status != "succeeded" || event.Sync.Points != "8" {
			t.Fatal("manual retry did not recover")
		}
	})

	t.Run("disabled settings cancel pending work", func(t *testing.T) {
		start()
		vote(one, "testing", "8")
		end()
		save()
		settings.Enabled = false
		if err := jira.SavePokerJiraSettings(ctx, game, one, settings); err != nil {
			t.Fatal(err)
		}
		if event := process(); event != nil {
			t.Fatal("disabled writeback still sent an update")
		}
		settings.Enabled = true
		if err := jira.SavePokerJiraSettings(ctx, game, one, settings); err != nil {
			t.Fatal(err)
		}
		if event := process(); event != nil {
			t.Fatal("enabling writeback silently resent an old estimate")
		}
	})

	t.Run("changing settings while waiting does not send without Save", func(t *testing.T) {
		start()
		vote(one, "testing", "5")
		end()
		settings.Enabled = false
		if err := jira.SavePokerJiraSettings(ctx, game, one, settings); err != nil {
			t.Fatal(err)
		}
		if plan := poker.GetStories(game, one)[0]; plan.JiraSync.Status != "cancelled" {
			t.Fatal("disabling writeback did not cancel the waiting task")
		}
		settings.Enabled = true
		if err := jira.SavePokerJiraSettings(ctx, game, one, settings); err != nil {
			t.Fatal(err)
		}
		if event := process(); event != nil {
			t.Fatal("settings confirmation bypassed the result Save button")
		}
		save()
		if event := process(); event == nil || event.Sync.Status != "succeeded" || event.Sync.Points != "5" {
			t.Fatal("explicit Save did not apply the current settings")
		}
	})

	t.Run("mismatched Jira link cannot target another issue", func(t *testing.T) {
		exec(`UPDATE thunderdome.poker_story SET link = $2 WHERE id = $1`, story, server.URL+"/browse/OTHER-2")
		start()
		vote(one, "testing", "3")
		end()
		save()
		count := len(written)
		event := process()
		if event == nil || event.Sync.Status != "pending" || len(written) != count {
			t.Fatal("mismatched issue link was written")
		}
	})

	t.Run("switching or skipping a story does not write unfinished votes", func(t *testing.T) {
		const nextStory = "00000000-0000-0000-0000-000000000005"
		exec(`INSERT INTO thunderdome.poker_story(id, poker_id, active) VALUES ($1, $2, false)`, nextStory, game)
		exec(`UPDATE thunderdome.poker_story SET link = $2 WHERE id = $1`, story, server.URL+"/browse/TEST-1")
		start()
		vote(one, "testing", "8")
		if _, err := poker.ActivateStoryVoting(game, nextStory); err != nil {
			t.Fatal(err)
		}
		if event := process(); event != nil {
			t.Fatal("switching stories wrote an unfinished round")
		}
		if _, err := poker.SkipStory(game, nextStory); err != nil {
			t.Fatal(err)
		}
		if event := process(); event != nil {
			t.Fatal("skipping an unscored story wrote to Jira")
		}
		start()
		vote(one, "testing", "3")
		end()
		save()
		if _, err := poker.ActivateStoryVoting(game, nextStory); err != nil {
			t.Fatal(err)
		}
		if _, err := poker.SkipStory(game, nextStory); err != nil {
			t.Fatal(err)
		}
		if event := process(); event == nil || event.Sync.Status != "succeeded" || event.Sync.Points != "3" {
			t.Fatal("skipping another story discarded a completed estimate")
		}
		start()
		vote(one, "testing", "5")
		end()
		if _, err := poker.SkipStory(game, story); err != nil {
			t.Fatal(err)
		}
		if event := process(); event != nil {
			t.Fatal("a skipped story's pending write was not cancelled")
		}
		var status string
		if err := database.QueryRow(`SELECT status FROM thunderdome.poker_jira_sync WHERE story_id = $1`, story).Scan(&status); err != nil || status != "cancelled" {
			t.Fatalf("skipped task did not retain a cancelled status: %s %v", status, err)
		}
	})
}

package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/StevenWeathers/thunderdome-planning-poker/thunderdome"
)

func TestWritePointsCloudAndDataCenter(t *testing.T) {
	for _, dataCenter := range []bool{false, true} {
		t.Run(fmt.Sprint("dataCenter=", dataCenter), func(t *testing.T) {
			for _, points := range []string{"12.83", "0"} {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					version := "3"
					if dataCenter {
						version = "2"
						if r.Header.Get("Authorization") != "Bearer test-token" {
							t.Error("wrong Data Center auth")
						}
					} else {
						user, password, ok := r.BasicAuth()
						if !ok || user != "test@example.com" || password != "test-token" {
							t.Error("wrong Cloud auth")
						}
					}
					if r.URL.Path != "/jira/rest/api/"+version+"/issue/TEST-123" || r.Method != "PUT" {
						t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
					}
					var payload map[string]map[string]float64
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Error(err)
					}
					expected, _ := thunderdome.NumericPokerVote(points)
					if len(payload) != 1 || len(payload["fields"]) != 1 || payload["fields"]["customfield_10016"] != expected {
						t.Errorf("unexpected update: %#v", payload)
					}
					w.WriteHeader(http.StatusNoContent)
				}))
				instance := thunderdome.JiraInstance{Host: server.URL + "/jira/", ClientMail: "test@example.com", AccessToken: "test-token", JiraDataCenter: dataCenter}
				write := thunderdome.PokerJiraWrite{Host: instance.Host, Link: server.URL + "/jira/browse/TEST-123", IssueKey: "TEST-123", FieldID: "customfield_10016", Points: points}
				err := WritePoints(context.Background(), instance, write)
				server.Close()
				if err != nil || calls != 1 {
					t.Fatalf("write failed: calls=%d, err=%v", calls, err)
				}
			}
		})
	}
}

func TestWritePointsRejectsUnsafeOrInvalidTargets(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer server.Close()
	instance := thunderdome.JiraInstance{Host: server.URL}
	valid := thunderdome.PokerJiraWrite{Host: server.URL, Link: server.URL + "/browse/TEST-1", IssueKey: "TEST-1", FieldID: "customfield_10016", Points: "5"}
	for _, mutate := range []func(*thunderdome.PokerJiraWrite){
		func(w *thunderdome.PokerJiraWrite) { w.Link = "https://another.example/browse/TEST-1" },
		func(w *thunderdome.PokerJiraWrite) { w.Link = server.URL + "/browse/OTHER-1" },
		func(w *thunderdome.PokerJiraWrite) { w.Host = "https://changed.example" },
		func(w *thunderdome.PokerJiraWrite) { w.IssueKey = "../../other" },
		func(w *thunderdome.PokerJiraWrite) { w.FieldID = "summary" },
		func(w *thunderdome.PokerJiraWrite) { w.Points = "NaN" },
		func(w *thunderdome.PokerJiraWrite) { w.Points = "" },
	} {
		write := valid
		mutate(&write)
		if err := WritePoints(context.Background(), instance, write); err == nil {
			t.Errorf("accepted invalid write: %#v", write)
		}
	}
	if called {
		t.Fatal("invalid destination received a request")
	}
}

func TestNumericFieldsAndRedactedFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"id":"summary","name":"Summary","schema":{"type":"string"}},{"id":"customfield_10016","name":"Story Points","schema":{"type":"number"}},{"id":"customfield_2","name":"Text","schema":{"type":"string"}}]`)
	}))
	fields, err := NumericFields(context.Background(), thunderdome.JiraInstance{Host: server.URL})
	server.Close()
	if err != nil || len(fields) != 1 || fields[0].ID != "customfield_10016" {
		t.Fatalf("unexpected numeric fields: %+v %v", fields, err)
	}
	for _, status := range []int{400, 401, 403, 404, 429, 500} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			fmt.Fprint(w, "secret-token from upstream")
		}))
		_, err := NumericFields(context.Background(), thunderdome.JiraInstance{Host: server.URL})
		server.Close()
		if err == nil || strings.Contains(err.Error(), "secret-token") || !strings.Contains(err.Error(), fmt.Sprint(status)) {
			t.Fatalf("unhelpful or unsafe error for %d: %v", status, err)
		}
	}
}

func TestJiraDoesNotForwardCredentialsOnRedirect(t *testing.T) {
	called := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	_, err := NumericFields(context.Background(), thunderdome.JiraInstance{Host: server.URL, AccessToken: "secret-token"})
	if err == nil || called {
		t.Fatal("redirect must not forward a Jira credential")
	}
}

package jira

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/StevenWeathers/thunderdome-planning-poker/thunderdome"
)

func TestImportOptionsAuthenticationAndPicker(t *testing.T) {
	const query = "迭代 41 & +?#=已完成"
	for _, tc := range []struct {
		name, auth string
		dataCenter bool
	}{
		{"cloud", "", false}, {"server password", "basic", true}, {"DC legacy PAT", "", true}, {"DC PAT", "pat", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("unexpected mutating request: %s", r.Method)
				}
				if tc.dataCenter && tc.auth != "basic" {
					if r.Header.Get("Authorization") != "Bearer secret-token" {
						t.Error("incorrect PAT authentication")
					}
				} else {
					username, password, ok := r.BasicAuth()
					if !ok || username != "jira.user" || password != "secret-token" {
						t.Error("incorrect basic authentication")
					}
				}
				version := "3"
				if tc.dataCenter {
					version = "2"
				}
				switch r.URL.Path {
				case "/jira/rest/api/" + version + "/issuetype":
					fmt.Fprint(w, `[{"id":"10001","name":"Story","subtask":false,"privateField":"secret-token"},{"id":"10002","name":"子任务","subtask":true}]`)
				case "/jira/rest/greenhopper/1.0/sprint/picker":
					if r.URL.Query().Get("query") != query || r.URL.Query().Get("excludeCompleted") != "false" || len(r.URL.Query()) != 2 {
						t.Errorf("incorrect picker query: %s", r.URL.RawQuery)
					}
					fmt.Fprint(w, `{"suggestions":[{"id":577,"name":"Zaku Sprint41","stateKey":"ACTIVE","boardName":"Edge 敏捷看板"},{"id":576,"name":"Zaku Sprint40","stateKey":"CLOSED"}],"allMatches":[{"id":577,"name":"duplicate","stateKey":"ACTIVE"},{"id":578,"name":"Zaku Sprint42","stateKey":"FUTURE"},{"id":579,"name":"Unknown state","stateKey":"UNKNOWN"}]}`)
				default:
					t.Errorf("unexpected endpoint: %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer remote.Close()
			instance := thunderdome.JiraInstance{Host: remote.URL + "/jira/", ClientMail: "jira.user", AccessToken: "secret-token", JiraDataCenter: tc.dataCenter, AuthMethod: tc.auth}
			types, err := IssueTypes(context.Background(), instance)
			if err != nil || len(types) != 2 || types[0].ID != "10001" || types[1].Name != "子任务" || !types[1].Subtask {
				t.Fatalf("issue types failed: %+v %v", types, err)
			}
			sprints, err := Sprints(context.Background(), instance, query)
			if err != nil || len(sprints) != 4 {
				t.Fatalf("sprint merge failed: %+v %v", sprints, err)
			}
			if sprints[0].ID != 577 || sprints[0].Name != "Zaku Sprint41" || sprints[0].BoardName != "Edge 敏捷看板" || sprints[0].State != "active" || sprints[1].State != "closed" || sprints[2].State != "future" || sprints[3].State != "" {
				t.Fatalf("incorrect sprint normalization: %+v", sprints)
			}
		})
	}
}

func TestImportOptionsEmptyLists(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/issuetype") {
			fmt.Fprint(w, `[]`)
		} else {
			fmt.Fprint(w, `{"suggestions":[],"allMatches":[]}`)
		}
	}))
	defer remote.Close()
	instance := thunderdome.JiraInstance{Host: remote.URL}
	types, err := IssueTypes(context.Background(), instance)
	if err != nil || types == nil || len(types) != 0 {
		t.Fatalf("empty issue types: %+v %v", types, err)
	}
	sprints, err := Sprints(context.Background(), instance, "")
	if err != nil || sprints == nil || len(sprints) != 0 {
		t.Fatalf("empty sprints: %+v %v", sprints, err)
	}
}

func TestImportOptionsRejectMalformedLists(t *testing.T) {
	for _, tc := range []struct {
		name, typesBody, sprintsBody string
	}{
		{"HTML", "<html>secret-token</html>", "<html>secret-token</html>"},
		{"null", `null`, `null`},
		{"missing fields", `{}`, `{}`},
		{"missing ID", `[{"name":"Story"}]`, `{"suggestions":[{"name":"Sprint"}]}`},
		{"missing name", `[{"id":"10001"}]`, `{"suggestions":[{"id":577}]}`},
		{"wrong field type", `[{"id":10001,"name":"Story"}]`, `{"suggestions":[{"id":"577","name":"Sprint"}]}`},
		{"wrong matches shape", `{"values":[]}`, `{"suggestions":[],"allMatches":{}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/issuetype") {
					fmt.Fprint(w, tc.typesBody)
				} else {
					fmt.Fprint(w, tc.sprintsBody)
				}
			}))
			defer remote.Close()
			instance := thunderdome.JiraInstance{Host: remote.URL, AccessToken: "secret-token"}
			_, typesErr := IssueTypes(context.Background(), instance)
			_, sprintsErr := Sprints(context.Background(), instance, "")
			for _, err := range []error{typesErr, sprintsErr} {
				if err == nil || !strings.Contains(err.Error(), "JQL") || strings.Contains(err.Error(), "secret-token") {
					t.Fatalf("malformed response was accepted or exposed: %v", err)
				}
			}
		})
	}
}

func TestImportOptionsRedactedUpstreamFailures(t *testing.T) {
	for _, status := range []int{401, 403, 404, 429, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				fmt.Fprint(w, "secret-token / internal private response")
			}))
			defer remote.Close()
			instance := thunderdome.JiraInstance{Host: remote.URL, AccessToken: "secret-token"}
			_, typesErr := IssueTypes(context.Background(), instance)
			_, sprintsErr := Sprints(context.Background(), instance, "")
			for _, err := range []error{typesErr, sprintsErr} {
				if err == nil || !strings.Contains(err.Error(), fmt.Sprint(status)) || strings.Contains(err.Error(), "secret-token") || strings.Contains(err.Error(), remote.URL) {
					t.Fatalf("unsafe or unhelpful upstream error: %v", err)
				}
			}
		})
	}
}

func TestImportOptionsRejectOversizedResponses(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, strings.Repeat(" ", maxImportOptionsResponseBytes+1))
	}))
	defer remote.Close()
	instance := thunderdome.JiraInstance{Host: remote.URL}
	_, typesErr := IssueTypes(context.Background(), instance)
	_, sprintsErr := Sprints(context.Background(), instance, "")
	for _, err := range []error{typesErr, sprintsErr} {
		if err == nil || !strings.Contains(err.Error(), "过大") {
			t.Fatalf("response size was not bounded: %v", err)
		}
	}
}

func TestImportOptionsNeverFollowRedirects(t *testing.T) {
	forwarded := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded++ }))
	defer target.Close()
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/secret-token", http.StatusFound)
	}))
	defer remote.Close()
	instance := thunderdome.JiraInstance{Host: remote.URL, AccessToken: "secret-token"}
	_, typesErr := IssueTypes(context.Background(), instance)
	_, sprintsErr := Sprints(context.Background(), instance, "")
	for _, err := range []error{typesErr, sprintsErr} {
		if err == nil || !strings.Contains(err.Error(), "重定向") || strings.Contains(err.Error(), "secret-token") || forwarded != 0 {
			t.Fatalf("redirect followed or exposed: forwarded=%d err=%v", forwarded, err)
		}
	}
}

func TestImportOptionsRespectCancellation(t *testing.T) {
	for _, sprint := range []bool{false, true} {
		remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		instance := thunderdome.JiraInstance{Host: remote.URL, AccessToken: "secret-token"}
		var err error
		if sprint {
			_, err = Sprints(ctx, instance, "")
		} else {
			_, err = IssueTypes(ctx, instance)
		}
		cancel()
		remote.Close()
		if err == nil || strings.Contains(err.Error(), "secret-token") || strings.Contains(err.Error(), remote.URL) {
			t.Fatalf("request cancellation missing or unsafe: %v", err)
		}
	}
}

func TestImportOptionsRejectLongQueryBeforeRequest(t *testing.T) {
	requests := 0
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++ }))
	defer remote.Close()
	_, err := Sprints(context.Background(), thunderdome.JiraInstance{Host: remote.URL}, strings.Repeat("中", MaxSprintQueryLength+1))
	if err == nil || requests != 0 {
		t.Fatal("oversized query reached Jira")
	}
}

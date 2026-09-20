package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/StevenWeathers/thunderdome-planning-poker/thunderdome"
)

func callJiraImportOptions(handler http.HandlerFunc, userID, sessionID, instanceID, query string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/?"+url.Values{"query": {query}}.Encode(), nil)
	req.SetPathValue("userId", userID)
	req.SetPathValue("instanceId", instanceID)
	req = req.WithContext(context.WithValue(req.Context(), contextKeyUserID, sessionID))
	res := httptest.NewRecorder()
	handler(res, req)
	return res
}

func TestJiraImportOptionsOwnershipAndValidation(t *testing.T) {
	const ownerID = "00000000-0000-0000-0000-000000000011"
	const otherID = "00000000-0000-0000-0000-000000000012"
	const instanceID = "00000000-0000-0000-0000-000000000021"
	requests := 0
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++ }))
	defer remote.Close()
	stub := &jiraInstanceStub{instance: thunderdome.JiraInstance{UserID: ownerID, Host: remote.URL}}
	svc := &Service{JiraDataSvc: stub}
	for _, handler := range []http.HandlerFunc{svc.handleJiraIssueTypes(), svc.handleJiraSprints()} {
		for _, tc := range []struct {
			name, user, session, instance string
			status                        int
		}{
			{"other session claims owner path", ownerID, otherID, instanceID, 404},
			{"owner session claims other path", otherID, ownerID, instanceID, 404},
			{"other user claims own path with owner instance", otherID, otherID, instanceID, 404},
			{"missing session", ownerID, "", instanceID, 404},
			{"invalid user ID", "not-uuid", ownerID, instanceID, 400},
			{"invalid instance ID", ownerID, ownerID, "not-uuid", 400},
		} {
			t.Run(tc.name, func(t *testing.T) {
				res := callJiraImportOptions(handler, tc.user, tc.session, tc.instance, "")
				if res.Code != tc.status || requests != 0 || stub.saved {
					t.Fatalf("rejected request reached Jira: status=%d requests=%d", res.Code, requests)
				}
			})
		}
	}
	res := callJiraImportOptions(svc.handleJiraSprints(), ownerID, ownerID, instanceID, strings.Repeat("中", 201))
	if res.Code != http.StatusBadRequest || requests != 0 {
		t.Fatalf("oversized query reached Jira: status=%d requests=%d", res.Code, requests)
	}
}

func TestJiraImportOptionsResponses(t *testing.T) {
	const userID = "00000000-0000-0000-0000-000000000011"
	const instanceID = "00000000-0000-0000-0000-000000000021"
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret-token" || r.Method != http.MethodGet {
			t.Error("incorrect saved credentials or mutating request")
		}
		switch r.URL.Path {
		case "/jira/rest/api/2/issuetype":
			fmt.Fprint(w, `[{"id":"10001","name":"Story","subtask":false,"description":"private-description"}]`)
		case "/jira/rest/greenhopper/1.0/sprint/picker":
			if r.URL.Query().Get("query") != "迭代 &41" {
				t.Error("query was not forwarded safely")
			}
			fmt.Fprint(w, `{"suggestions":[{"id":577,"name":"Sprint41","stateKey":"CLOSED","boardName":"Edge","private":"secret-token"}],"allMatches":[]}`)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer remote.Close()
	stub := &jiraInstanceStub{instance: thunderdome.JiraInstance{UserID: userID, Host: remote.URL + "/jira", JiraDataCenter: true, AuthMethod: "pat", AccessToken: "secret-token"}}
	svc := &Service{JiraDataSvc: stub}
	for _, tc := range []struct {
		name, expected string
		handler        http.HandlerFunc
	}{
		{"issue types", `[{"id":"10001","name":"Story","subtask":false}]`, svc.handleJiraIssueTypes()},
		{"sprints", `[{"id":577,"name":"Sprint41","state":"closed","boardName":"Edge"}]`, svc.handleJiraSprints()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := callJiraImportOptions(tc.handler, userID, userID, instanceID, "迭代 &41")
			var result struct {
				Success bool            `json:"success"`
				Data    json.RawMessage `json:"data"`
			}
			if res.Code != http.StatusOK || json.Unmarshal(res.Body.Bytes(), &result) != nil || !result.Success || string(result.Data) != tc.expected {
				t.Fatalf("incorrect data envelope: %d %s", res.Code, res.Body.String())
			}
			if strings.Contains(res.Body.String(), "secret-token") || strings.Contains(res.Body.String(), "private-description") || stub.saved {
				t.Fatal("private data exposed or saved instance changed")
			}
		})
	}
}

func TestJiraImportOptionsUpstreamFailureDoesNotExpireSession(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, "secret-token private-error")
	}))
	defer remote.Close()
	stub := &jiraInstanceStub{instance: thunderdome.JiraInstance{UserID: "00000000-0000-0000-0000-000000000011", Host: remote.URL, AccessToken: "secret-token"}}
	svc := &Service{JiraDataSvc: stub}
	for _, handler := range []http.HandlerFunc{svc.handleJiraIssueTypes(), svc.handleJiraSprints()} {
		res := callJiraInstance(handler, http.MethodGet, "")
		var result standardJsonResponse
		if res.Code != http.StatusUnprocessableEntity || json.Unmarshal(res.Body.Bytes(), &result) != nil || result.Success || !strings.Contains(result.Error, "401") || !strings.Contains(result.Error, "JQL") || strings.Contains(res.Body.String(), "secret-token") || strings.Contains(res.Body.String(), "private-error") {
			t.Fatalf("unsafe Jira error response: %d %s", res.Code, res.Body.String())
		}
	}
}

package http

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/StevenWeathers/thunderdome-planning-poker/thunderdome"
)

func TestJiraCannotSaveRejectedCredentials(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, "secret-password from upstream")
	}))
	defer remote.Close()
	for _, method := range []string{"POST", "PUT"} {
		t.Run(method, func(t *testing.T) {
			stub := &jiraInstanceStub{instance: thunderdome.JiraInstance{UserID: "00000000-0000-0000-0000-000000000011", JiraDataCenter: true, AuthMethod: "basic", AccessToken: "old-password"}}
			svc := &Service{JiraDataSvc: stub}
			body := fmt.Sprintf(`{"host":%q,"client_mail":"jira.user","access_token":"secret-password","jira_data_center":true,"auth_method":"basic"}`, remote.URL)
			handler := svc.handleJiraInstanceCreate()
			if method == "PUT" {
				handler = svc.handleJiraInstanceUpdate()
			}
			res := callJiraInstance(handler, method, body)
			if res.Code != 422 || stub.saved || stub.instance.AccessToken != "old-password" || !strings.Contains(res.Body.String(), "401") || strings.Contains(res.Body.String(), "secret-password") {
				t.Fatalf("invalid credentials saved or exposed: status=%d saved=%v", res.Code, stub.saved)
			}
		})
	}
}

func TestSavedJiraConnectionOwnershipAndPrivacy(t *testing.T) {
	requests := 0
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != "GET" || r.URL.Path != "/rest/api/2/myself" {
			t.Error("test modified Jira data")
		}
		username, password, ok := r.BasicAuth()
		if !ok || username != "jira.user" || password != "secret-password" {
			t.Error("stored credentials were not used")
		}
		fmt.Fprint(w, `{"name":"jira.user","displayName":"Test User","active":true,"emailAddress":"private@example.com"}`)
	}))
	defer remote.Close()
	stub := &jiraInstanceStub{instance: thunderdome.JiraInstance{UserID: "00000000-0000-0000-0000-000000000011", Host: remote.URL, ClientMail: "jira.user", AccessToken: "secret-password", JiraDataCenter: true, AuthMethod: "basic"}}
	svc := &Service{JiraDataSvc: stub}
	res := callJiraInstance(svc.handleJiraInstanceTest(), "POST", "")
	if res.Code != 200 || !strings.Contains(res.Body.String(), `"connected":true`) || requests != 1 || stub.saved {
		t.Fatalf("connection was not tested: %d", res.Code)
	}
	for _, secret := range []string{"secret-password", "private@example.com", "access_token"} {
		if strings.Contains(res.Body.String(), secret) {
			t.Fatal("connection test leaked credentials or unrelated profile data")
		}
	}
	stub.instance.UserID = "another-user"
	res = callJiraInstance(svc.handleJiraInstanceTest(), "POST", "")
	if res.Code != 404 || requests != 1 {
		t.Fatal("tested another user's credentials")
	}
}

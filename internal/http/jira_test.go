package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/StevenWeathers/thunderdome-planning-poker/thunderdome"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.uber.org/zap"
)

type jiraInstanceStub struct {
	JiraDataSvc
	instance thunderdome.JiraInstance
	saved    bool
}

func (s *jiraInstanceStub) GetInstanceByID(context.Context, string) (thunderdome.JiraInstance, error) {
	return s.instance, nil
}

func (s *jiraInstanceStub) CreateInstance(_ context.Context, userID, host, mail, token string, dataCenter bool, method string) (thunderdome.JiraInstance, error) {
	s.saved = true
	s.instance = thunderdome.JiraInstance{UserID: userID, Host: host, ClientMail: mail, AccessToken: token, JiraDataCenter: dataCenter, AuthMethod: method}
	return s.instance, nil
}

func (s *jiraInstanceStub) UpdateInstance(_ context.Context, _, host, mail, token, method string) (thunderdome.JiraInstance, error) {
	s.saved = true
	s.instance.Host, s.instance.ClientMail, s.instance.AccessToken, s.instance.AuthMethod = host, mail, token, method
	return s.instance, nil
}

func TestJiraInstanceCreateAuthentication(t *testing.T) {
	for _, tc := range []struct {
		name, username, method string
		dataCenter, accepted   bool
	}{
		{"cloud email", "jira@example.com", "", false, true},
		{"cloud username rejected", "jira.user", "", false, false},
		{"cloud PAT rejected", "jira@example.com", "pat", false, false},
		{"legacy DC username", "jira.user", "", true, true},
		{"PAT without username", "", "pat", true, true},
		{"server password", "jira.user", "basic", true, true},
		{"password needs username", "", "basic", true, false},
		{"colon in username", "jira:user", "basic", true, false},
		{"unknown auth", "jira.user", "other", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := &jiraInstanceStub{}
			svc := &Service{JiraDataSvc: stub}
			body, _ := json.Marshal(jiraInstanceRequestBody{Host: "http://jira.example.com", ClientMail: tc.username, AccessToken: "test-secret", JiraDataCenter: tc.dataCenter, AuthMethod: tc.method})
			res := callJiraInstance(svc.handleJiraInstanceCreate(), "POST", string(body))
			if tc.accepted {
				if res.Code != 200 || !stub.saved || stub.instance.AuthMethod != tc.method {
					t.Fatalf("valid Jira credentials rejected: %d %s", res.Code, res.Body.String())
				}
			} else if res.Code != 400 || stub.saved || strings.Contains(res.Body.String(), "test-secret") {
				t.Fatalf("invalid credentials accepted or leaked: %d", res.Code)
			}
		})
	}
}

func TestJiraUpdateRetainsSavedPlatformAndAuth(t *testing.T) {
	stub := &jiraInstanceStub{instance: thunderdome.JiraInstance{UserID: "00000000-0000-0000-0000-000000000011", JiraDataCenter: true, AuthMethod: "basic"}}
	svc := &Service{JiraDataSvc: stub}
	body := `{"host":"http://jira.example.com","client_mail":"jira.user","access_token":"test-password"}`
	res := callJiraInstance(svc.handleJiraInstanceUpdate(), "PUT", body)
	if res.Code != 200 || !stub.saved || stub.instance.AuthMethod != "basic" || !stub.instance.JiraDataCenter {
		t.Fatalf("legacy update lost Server authentication: %d %s", res.Code, res.Body.String())
	}
	stub.saved = false
	stub.instance.UserID = "another-user"
	res = callJiraInstance(svc.handleJiraInstanceUpdate(), "PUT", body)
	if res.Code != 404 || stub.saved {
		t.Fatal("updated another user's Jira credentials")
	}
}

func callJiraInstance(handler http.HandlerFunc, method, body string) *httptest.ResponseRecorder {
	const userID = "00000000-0000-0000-0000-000000000011"
	req := httptest.NewRequest(method, "/", strings.NewReader(body))
	req.SetPathValue("userId", userID)
	req.SetPathValue("instanceId", "00000000-0000-0000-0000-000000000021")
	req = req.WithContext(context.WithValue(req.Context(), contextKeyUserID, userID))
	res := httptest.NewRecorder()
	handler(res, req)
	return res
}

func TestJiraSearchFailureReturnsOneResponse(t *testing.T) {
	requests := 0
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"errorMessages":["Authentication failed"]}`))
	}))
	defer remote.Close()
	stub := &jiraInstanceStub{instance: thunderdome.JiraInstance{UserID: "00000000-0000-0000-0000-000000000011", Host: remote.URL, ClientMail: "jira.user", AccessToken: "test-password", JiraDataCenter: true, AuthMethod: "basic"}}
	svc := &Service{JiraDataSvc: stub, Logger: otelzap.New(zap.NewNop())}
	res := callJiraInstance(svc.handleJiraStoryJQLSearch(), "POST", `{"jql":"project = TEST"}`)
	var result standardJsonResponse
	if res.Code != 500 || json.Unmarshal(res.Body.Bytes(), &result) != nil || result.Success || requests != 1 {
		t.Fatal("Jira failure was followed by a success response")
	}
	stub.instance.UserID = "another-user"
	res = callJiraInstance(svc.handleJiraStoryJQLSearch(), "POST", `{"jql":"project = TEST"}`)
	if res.Code != 404 || requests != 1 {
		t.Fatal("used another user's Jira credentials")
	}
}

package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/StevenWeathers/thunderdome-planning-poker/thunderdome"
)

type pokerJiraAuthStub struct {
	PokerDataSvc
	denied bool
}

func (s pokerJiraAuthStub) ConfirmFacilitator(string, string) error {
	if s.denied {
		return errors.New("not facilitator")
	}
	return nil
}

type pokerJiraDataStub struct {
	JiraDataSvc
	instance thunderdome.JiraInstance
	saved    *thunderdome.PokerJiraSettings
}

func (s *pokerJiraDataStub) GetInstanceByID(context.Context, string) (thunderdome.JiraInstance, error) {
	return s.instance, nil
}
func (s *pokerJiraDataStub) FindInstancesByUserID(context.Context, string) ([]thunderdome.JiraInstance, error) {
	return []thunderdome.JiraInstance{s.instance}, nil
}
func (s *pokerJiraDataStub) GetPokerJiraSettings(context.Context, string) (thunderdome.PokerJiraSettings, error) {
	return thunderdome.PokerJiraSettings{}, nil
}
func (s *pokerJiraDataStub) SavePokerJiraSettings(_ context.Context, _, _ string, settings thunderdome.PokerJiraSettings) error {
	s.saved = &settings
	return nil
}

func TestPokerJiraAuthorizationAndFields(t *testing.T) {
	const userID = "00000000-0000-0000-0000-000000000011"
	const instanceID = "00000000-0000-0000-0000-000000000021"
	const pokerID = "00000000-0000-0000-0000-000000000001"
	requests := 0
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != "GET" || r.URL.Path != "/rest/api/3/field" {
			t.Error("configuration must only read field metadata")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id":"customfield_10016","name":"Story Points","schema":{"type":"number"}}]`))
	}))
	defer remote.Close()
	jira := &pokerJiraDataStub{instance: thunderdome.JiraInstance{ID: instanceID, UserID: userID, Host: remote.URL, AccessToken: "private-token", ClientMail: "private@example.com"}}
	svc := &Service{PokerDataSvc: pokerJiraAuthStub{}, JiraDataSvc: jira}
	call := func(handler http.HandlerFunc, method, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, "/?instanceId="+instanceID, strings.NewReader(body))
		req.SetPathValue("battleId", pokerID)
		req = req.WithContext(context.WithValue(req.Context(), contextKeyUserID, userID))
		res := httptest.NewRecorder()
		handler(res, req)
		return res
	}
	validBody := `{"enabled":true,"instanceId":"` + instanceID + `","fieldId":"customfield_10016","host":"https://untrusted.example","fieldName":"untrusted"}`
	svc.PokerDataSvc = pokerJiraAuthStub{denied: true}
	if res := call(svc.handleSavePokerJiraSettings(nil), "PUT", validBody); res.Code != 403 || requests != 0 || jira.saved != nil {
		t.Fatal("non-facilitator configured Jira writeback")
	}
	svc.PokerDataSvc = pokerJiraAuthStub{}
	jira.instance.UserID = "00000000-0000-0000-0000-000000000012"
	if res := call(svc.handleSavePokerJiraSettings(nil), "PUT", validBody); res.Code != 403 || requests != 0 {
		t.Fatal("another user's credentials reached Jira")
	}
	if res := call(svc.handlePokerJiraFields(), "GET", ""); res.Code != 403 || requests != 0 {
		t.Fatal("another user's Jira fields were exposed")
	}
	jira.instance.UserID = userID
	if res := call(svc.handleSavePokerJiraSettings(nil), "PUT", strings.ReplaceAll(validBody, "customfield_10016", "summary")); res.Code != 400 || jira.saved != nil {
		t.Fatal("non-numeric field was accepted")
	}
	if res := call(svc.handleSavePokerJiraSettings(nil), "PUT", validBody); res.Code != 200 {
		t.Fatalf("valid configuration rejected: %s", res.Body.String())
	}
	if jira.saved == nil || jira.saved.Host != remote.URL || jira.saved.FieldName != "Story Points" {
		t.Fatal("trusted client-supplied Jira host or field name")
	}
	res := call(svc.handleGetPokerJiraSettings(), "GET", "")
	if res.Code != 200 || strings.Contains(res.Body.String(), "private-token") || strings.Contains(res.Body.String(), "private@example.com") || strings.Contains(res.Body.String(), "access_token") {
		t.Fatal("credentials leaked in configuration response")
	}
	var payload map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	before := requests
	if res := call(svc.handleSavePokerJiraSettings(nil), "PUT", `{"enabled":false}`); res.Code != 200 || requests != before || jira.saved.Enabled {
		t.Fatal("disabling writeback should not contact Jira")
	}
}

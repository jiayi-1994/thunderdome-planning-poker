package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/StevenWeathers/thunderdome-planning-poker/thunderdome"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.uber.org/zap"
)

const defaultJiraUser = "00000000-0000-0000-0000-000000000011"
const defaultJiraGame = "00000000-0000-0000-0000-000000000001"

type defaultJiraStore struct {
	JiraDataSvc
	instances                        []thunderdome.JiraInstance
	listErr, saveErr                 error
	saved                            *thunderdome.PokerJiraSettings
	listedUser, savedUser, savedGame string
	deadline                         time.Time
}

func (s *defaultJiraStore) FindInstancesByUserID(ctx context.Context, userID string) ([]thunderdome.JiraInstance, error) {
	s.listedUser = userID
	s.deadline, _ = ctx.Deadline()
	return s.instances, s.listErr
}
func (s *defaultJiraStore) SavePokerJiraSettings(_ context.Context, gameID, userID string, settings thunderdome.PokerJiraSettings) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.savedGame, s.savedUser, s.saved = gameID, userID, &settings
	return nil
}

type defaultPokerStore struct {
	PokerDataSvc
	created, teamCreated bool
	createErr            error
}

func (s *defaultPokerStore) GetDefaultPublicEstimationScale(context.Context) (*thunderdome.EstimationScale, error) {
	return &thunderdome.EstimationScale{ID: "00000000-0000-0000-0000-000000000050", Values: []string{"0", "1/2", "1", "2", "3", "5", "8"}}, nil
}
func (s *defaultPokerStore) CreateGame(context.Context, string, string, string, []string, []*thunderdome.Story, bool, string, string, string, bool) (*thunderdome.Poker, error) {
	s.created = true
	return &thunderdome.Poker{ID: defaultJiraGame}, s.createErr
}
func (s *defaultPokerStore) TeamCreateGame(context.Context, string, string, string, string, []string, []*thunderdome.Story, bool, string, string, string, bool) (*thunderdome.Poker, error) {
	s.teamCreated = true
	return &thunderdome.Poker{ID: defaultJiraGame}, s.createErr
}

type defaultProjectStore struct {
	ProjectDataSvc
	gameID string
}

func (s *defaultProjectStore) AssociatePoker(_ context.Context, _, gameID string) error {
	s.gameID = gameID
	return nil
}

type defaultSubscriptionStore struct {
	SubscriptionDataSvc
	err error
}

func (s defaultSubscriptionStore) CheckActiveSubscriber(context.Context, string) error { return s.err }

func defaultJiraRequest() *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Planning","pointValuesAllowed":["1","3","5"],"pointAverageRounding":"ceil"}`))
	ctx := context.WithValue(r.Context(), contextKeyUserID, defaultJiraUser)
	ctx = context.WithValue(ctx, contextKeyUserType, "REGISTERED")
	role := "MEMBER"
	ctx = context.WithValue(ctx, contextKeyUserTeamRoles, &thunderdome.UserTeamRoleInfo{TeamRole: &role})
	r = r.WithContext(ctx)
	r.SetPathValue("userId", defaultJiraUser)
	return r
}

func TestNewPokerEnablesJiraForEveryCreationEntryPoint(t *testing.T) {
	for _, entry := range []string{"personal", "team", "project"} {
		t.Run(entry, func(t *testing.T) {
			requests := 0
			remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Method != "GET" || r.URL.Path != "/rest/api/2/field" {
					t.Errorf("creation must only read fields: %s %s", r.Method, r.URL.Path)
				}
				username, password, ok := r.BasicAuth()
				if !ok || username != "jira.user" || password != "private-token" {
					t.Error("saved Server authentication was not used")
				}
				w.Write([]byte(`[{"id":"customfield_13565","name":"CloseHours","schema":{"type":"number"}},{"id":"customfield_10016","name":" Story Points ","schema":{"type":"number"}}]`))
			}))
			defer remote.Close()
			jiraStore := &defaultJiraStore{instances: []thunderdome.JiraInstance{{ID: "instance", UserID: defaultJiraUser, Host: remote.URL, JiraDataCenter: true, AuthMethod: "basic", ClientMail: "jira.user", AccessToken: "private-token"}}}
			pokerStore, projectStore := &defaultPokerStore{}, &defaultProjectStore{}
			svc := &Service{Config: &Config{SubscriptionsEnabled: true}, SubscriptionDataSvc: defaultSubscriptionStore{}, PokerDataSvc: pokerStore, JiraDataSvc: jiraStore, ProjectDataSvc: projectStore}
			r := defaultJiraRequest()
			handler := svc.handlePokerCreate()
			if entry == "team" {
				r.SetPathValue("teamId", "team")
			}
			if entry == "project" {
				r.SetPathValue("projectId", "00000000-0000-0000-0000-000000000060")
				handler = svc.handleCreateProjectPokerGame()
			}
			res := httptest.NewRecorder()
			handler(res, r)
			if res.Code != http.StatusOK {
				t.Fatalf("create failed: %d %s", res.Code, res.Body.String())
			}
			if entry == "team" && !pokerStore.teamCreated || entry != "team" && !pokerStore.created {
				t.Fatal("wrong creation path")
			}
			if entry == "project" && projectStore.gameID != defaultJiraGame {
				t.Fatal("project association lost")
			}
			if requests != 1 || jiraStore.saved == nil || !jiraStore.saved.Enabled || jiraStore.saved.FieldID != "customfield_10016" || jiraStore.saved.Host != remote.URL {
				t.Fatalf("writeback not initialized correctly: %+v", jiraStore.saved)
			}
			if jiraStore.savedGame != defaultJiraGame || jiraStore.savedUser != defaultJiraUser || jiraStore.listedUser != defaultJiraUser {
				t.Fatal("wrong owner or game")
			}
			if jiraStore.deadline.IsZero() || time.Until(jiraStore.deadline) > 5*time.Second {
				t.Fatal("initialization must have a bounded deadline")
			}
			var body struct {
				Meta pokerCreationMeta `json:"meta"`
			}
			if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil || !body.Meta.JiraWritebackEnabled || body.Meta.JiraWritebackWarning != "" {
				t.Fatalf("wrong response: %s", res.Body.String())
			}
			if strings.Contains(res.Body.String(), "private-token") || strings.Contains(res.Body.String(), "jira.user") {
				t.Fatal("credentials leaked")
			}
		})
	}
}

func TestNewPokerJiraDefaultsDoNotGuessOrFailCreation(t *testing.T) {
	for _, test := range []struct {
		name, fields                       string
		status                             int
		count                              int
		foreignOwner, listError, saveError bool
		wantWarning, wantRead              bool
	}{
		{name: "no connection"},
		{name: "multiple connections", count: 2, wantWarning: true},
		{name: "foreign credentials", count: 1, foreignOwner: true, wantWarning: true},
		{name: "list error", listError: true, wantWarning: true},
		{name: "unavailable Jira", count: 1, status: 503, wantWarning: true, wantRead: true},
		{name: "unauthorized Jira", count: 1, status: 401, wantWarning: true, wantRead: true},
		{name: "missing Story Points", count: 1, fields: `[{"id":"customfield_13565","name":"CloseHours","schema":{"type":"number"}}]`, wantWarning: true, wantRead: true},
		{name: "nonnumeric Story Points", count: 1, fields: `[{"id":"customfield_10016","name":"Story Points","schema":{"type":"string"}}]`, wantWarning: true, wantRead: true},
		{name: "duplicate Story Points", count: 1, fields: `[{"id":"customfield_10016","name":"Story Points","schema":{"type":"number"}},{"id":"customfield_10017","name":"story points","schema":{"type":"number"}}]`, wantWarning: true, wantRead: true},
		{name: "settings save error", count: 1, fields: `[{"id":"customfield_10016","name":"Story Points","schema":{"type":"number"}}]`, saveError: true, wantWarning: true, wantRead: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			requests := 0
			remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Method != "GET" {
					t.Error("unexpected write")
				}
				if test.status != 0 {
					w.WriteHeader(test.status)
				}
				w.Write([]byte(test.fields))
			}))
			defer remote.Close()
			store := &defaultJiraStore{}
			for i := 0; i < test.count; i++ {
				store.instances = append(store.instances, thunderdome.JiraInstance{ID: "instance", UserID: defaultJiraUser, Host: remote.URL, AccessToken: "private-token", ClientMail: "private@example.com"})
			}
			if test.foreignOwner {
				store.instances[0].UserID = "another-user"
			}
			if test.listError {
				store.listErr = errors.New("private database error")
			}
			if test.saveError {
				store.saveErr = errors.New("private database error")
			}
			svc := &Service{Config: &Config{}, JiraDataSvc: store, PokerDataSvc: &defaultPokerStore{}}
			res := httptest.NewRecorder()
			svc.handlePokerCreate()(res, defaultJiraRequest())
			if res.Code != http.StatusOK {
				t.Fatalf("optional Jira failure broke game creation: %s", res.Body.String())
			}
			var body struct {
				Meta pokerCreationMeta `json:"meta"`
			}
			json.Unmarshal(res.Body.Bytes(), &body)
			if body.Meta.JiraWritebackEnabled || (body.Meta.JiraWritebackWarning != "") != test.wantWarning || store.saved != nil {
				t.Fatalf("unexpected initialization: %s", res.Body.String())
			}
			if (requests > 0) != test.wantRead {
				t.Fatalf("unexpected remote access: %d", requests)
			}
			if strings.Contains(res.Body.String(), "private") {
				t.Fatal("private error or credentials leaked")
			}
		})
	}
}

func TestNewPokerJiraDefaultsRespectCreatorAndSubscription(t *testing.T) {
	store := &defaultJiraStore{instances: []thunderdome.JiraInstance{{UserID: defaultJiraUser}}}
	svc := &Service{JiraDataSvc: store, Config: &Config{SubscriptionsEnabled: true}, SubscriptionDataSvc: defaultSubscriptionStore{err: errors.New("not subscribed")}}
	r := defaultJiraRequest()
	if result := svc.initializePokerJiraWriteback(r, defaultJiraGame, "another-user"); result.JiraWritebackEnabled || store.listedUser != "" {
		t.Fatal("creation on behalf of another user accessed credentials")
	}
	r = r.WithContext(context.WithValue(r.Context(), contextKeyUserType, "REGISTERED"))
	if result := svc.initializePokerJiraWriteback(r, defaultJiraGame, defaultJiraUser); result.JiraWritebackEnabled || result.JiraWritebackWarning == "" || store.saved != nil {
		t.Fatal("subscription requirement bypassed")
	}
}

func TestFailedPokerCreationDoesNotInitializeJira(t *testing.T) {
	store := &defaultJiraStore{}
	svc := &Service{Config: &Config{}, JiraDataSvc: store, PokerDataSvc: &defaultPokerStore{createErr: errors.New("create failed")}, Logger: otelzap.New(zap.NewNop())}
	res := httptest.NewRecorder()
	svc.handlePokerCreate()(res, defaultJiraRequest())
	if res.Code != http.StatusInternalServerError || store.listedUser != "" {
		t.Fatal("initialized Jira for a failed game creation")
	}
}

func TestNewPokerJiraDefaultsHonorRequestDeadline(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer remote.Close()
	store := &defaultJiraStore{instances: []thunderdome.JiraInstance{{ID: "instance", UserID: defaultJiraUser, Host: remote.URL, ClientMail: "jira.user", AccessToken: "private-token"}}}
	svc := &Service{Config: &Config{}, JiraDataSvc: store}
	ctx, cancel := context.WithTimeout(defaultJiraRequest().Context(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	result := svc.initializePokerJiraWriteback(defaultJiraRequest().WithContext(ctx), defaultJiraGame, defaultJiraUser)
	if result.JiraWritebackEnabled || result.JiraWritebackWarning == "" || store.saved != nil {
		t.Fatal("timeout must leave writeback disabled and explain how to recover")
	}
	if time.Since(started) > 2*time.Second {
		t.Fatal("initialization did not honor the caller's shorter deadline")
	}
}

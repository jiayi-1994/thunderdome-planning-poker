package jiradatacenter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchAuthentication(t *testing.T) {
	for _, method := range []string{"", "pat", "basic"} {
		t.Run("auth="+method, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/jira/rest/api/2/search" {
					t.Errorf("unexpected API path: %s", r.URL.Path)
				}
				if method == "basic" {
					user, password, ok := r.BasicAuth()
					if !ok || user != "jira.user" || password != "test-password" {
						t.Error("wrong username/password authentication")
					}
				} else if r.Header.Get("Authorization") != "Bearer test-password" {
					t.Error("legacy PAT authentication changed")
				}
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"total":1,"issues":[{"key":"TEST-1","fields":{"summary":"Test story"}}]}`))
			}))
			defer server.Close()
			client, err := New(Config{InstanceHost: server.URL + "/jira/", ClientMail: "jira.user", AccessToken: "test-password", AuthMethod: method})
			if err != nil {
				t.Fatal(err)
			}
			result, err := client.StoriesJQLSearch(context.Background(), "project = TEST", nil, 0, 10)
			if err != nil || calls != 1 || result == nil || result.Total != 1 || result.Issues[0].Key != "TEST-1" {
				t.Fatalf("search failed: result=%v calls=%d err=%v", result, calls, err)
			}
		})
	}
}

func TestUnknownAuthenticationIsRejected(t *testing.T) {
	if _, err := New(Config{InstanceHost: "https://jira.example.com", AuthMethod: "unknown"}); err == nil {
		t.Fatal("unknown authentication silently accepted")
	}
}

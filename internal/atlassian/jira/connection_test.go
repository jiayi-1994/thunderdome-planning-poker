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

func TestCheckConnectionAuthentication(t *testing.T) {
	for _, tc := range []struct {
		name, auth string
		dataCenter bool
	}{
		{"cloud", "", false}, {"server password", "basic", true}, {"DC legacy", "", true}, {"DC PAT", "pat", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				version := "3"
				if tc.dataCenter {
					version = "2"
				}
				if r.Method != "GET" || r.URL.Path != "/jira/rest/api/"+version+"/myself" {
					t.Errorf("unexpected connection probe: %s %s", r.Method, r.URL.Path)
				}
				if tc.dataCenter && tc.auth != "basic" {
					if r.Header.Get("Authorization") != "Bearer test-secret" {
						t.Error("wrong PAT authentication")
					}
				} else {
					username, password, ok := r.BasicAuth()
					if !ok || username != "jira.user" || password != "test-secret" {
						t.Error("wrong basic authentication")
					}
				}
				fmt.Fprint(w, `{"name":"jira.user","displayName":"Test User","active":true}`)
			}))
			defer remote.Close()
			status, err := CheckConnection(context.Background(), thunderdome.JiraInstance{Host: remote.URL + "/jira/", ClientMail: "jira.user", AccessToken: "test-secret", JiraDataCenter: tc.dataCenter, AuthMethod: tc.auth})
			if err != nil || status == nil || !status.Connected || status.DisplayName != "Test User" {
				t.Fatalf("connection failed: %v %v", status, err)
			}
		})
	}
}

func TestCheckConnectionRejectsErrorsAndNonUserResponses(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		status                     int
		body, loginReason, message string
	}{
		{"password", 401, "secret-token", "", "401"},
		{"permissions", 403, "secret-token", "", "403"},
		{"CAPTCHA", 401, "secret-token", "AUTHENTICATION_DENIED", "验证码"},
		{"wrong path", 404, "secret-token", "", "404"},
		{"rate limited", 429, "secret-token", "", "429"},
		{"upstream", 500, "secret-token", "", "500"},
		{"SSO HTML", 200, "<html>secret-token</html>", "", "用户信息"},
		{"anonymous JSON", 200, `{"displayName":"Guest"}`, "", "用户信息"},
		{"inactive user", 200, `{"name":"jira.user","active":false}`, "", "停用"},
		{"oversized", 200, strings.Repeat("x", 1024*1024+1), "", "用户信息"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Seraph-LoginReason", tc.loginReason)
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer remote.Close()
			_, err := CheckConnection(context.Background(), thunderdome.JiraInstance{Host: remote.URL, AccessToken: "secret-token"})
			if err == nil || !strings.Contains(err.Error(), tc.message) || strings.Contains(err.Error(), "secret-token") || strings.Contains(err.Error(), remote.URL) {
				t.Fatalf("unhelpful or unsafe error: %v", err)
			}
		})
	}
}

func TestCheckConnectionDoesNotFollowRedirects(t *testing.T) {
	forwarded := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded = true }))
	defer target.Close()
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) }))
	defer remote.Close()
	_, err := CheckConnection(context.Background(), thunderdome.JiraInstance{Host: remote.URL, AccessToken: "secret-token"})
	if err == nil || forwarded || !strings.Contains(err.Error(), "重定向") {
		t.Fatal("followed a credential-bearing redirect")
	}
}

func TestCheckConnectionTimeoutAndTLS(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer remote.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := CheckConnection(ctx, thunderdome.JiraInstance{Host: remote.URL})
	if err == nil || !strings.Contains(err.Error(), "超时") {
		t.Fatalf("timeout missing: %v", err)
	}
	tlsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer tlsServer.Close()
	_, err = CheckConnection(context.Background(), thunderdome.JiraInstance{Host: tlsServer.URL})
	if err == nil || !strings.Contains(err.Error(), "证书") {
		t.Fatalf("certificate error missing: %v", err)
	}
}

package http

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/StevenWeathers/thunderdome-planning-poker/internal/cookie"
	"github.com/StevenWeathers/thunderdome-planning-poker/thunderdome"
)

type profileSessionAuth struct {
	AuthDataSvc
	valid bool
}

func (a profileSessionAuth) GetSessionUserByID(_ context.Context, id string) (*thunderdome.User, error) {
	if !a.valid || id != "session-id" {
		return nil, sql.ErrNoRows
	}
	return &thunderdome.User{ID: "user-id", Type: thunderdome.RegisteredUserType}, nil
}

type profileSessionUsers struct{ UserDataSvc }

func (profileSessionUsers) GetUserByID(_ context.Context, id string) (*thunderdome.User, error) {
	return &thunderdome.User{ID: id}, nil
}

func (profileSessionUsers) GetGuestUserByID(_ context.Context, id string) (*thunderdome.User, error) {
	return &thunderdome.User{ID: id, Type: thunderdome.GuestUserType}, nil
}

func TestSessionProfileRenewsOnlyValidLoginCookies(t *testing.T) {
	for _, tc := range []struct {
		name      string
		guest     bool
		valid     bool
		wantCode  int
		wantRenew bool
	}{
		{name: "authenticated", valid: true, wantCode: http.StatusOK, wantRenew: true},
		{name: "revoked or expired", wantCode: http.StatusUnauthorized},
		{name: "guest", guest: true, wantCode: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cookies := cookie.New(cookie.Config{
				SessionCookieName: "sessionId", SecureCookieName: "guestId", SecureCookieFlag: true,
				CookieHashKey: "test-key",
			})
			svc := Service{Config: &Config{}, Cookie: cookies, AuthDataSvc: profileSessionAuth{valid: tc.valid}, UserDataSvc: profileSessionUsers{}}
			loginResponse := httptest.NewRecorder()
			issue := cookies.CreateSessionCookie
			id := "session-id"
			if tc.guest {
				issue, id = cookies.CreateUserCookie, "guest-id"
			}
			if err := issue(loginResponse, id); err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodGet, "https://example.test/api/auth/user", nil)
			req.AddCookie(loginResponse.Result().Cookies()[0])
			res := httptest.NewRecorder()
			svc.userOnly(svc.handleSessionUserProfile())(res, req)
			if res.Code != tc.wantCode {
				t.Fatalf("got status %d, want %d", res.Code, tc.wantCode)
			}
			issued := res.Result().Cookies()
			if !tc.wantRenew {
				if len(issued) != 0 {
					t.Fatal("session was renewed without an authenticated login")
				}
				return
			}
			if len(issued) != 1 || issued[0].Name != "sessionId" || issued[0].MaxAge != 400*24*60*60 || !issued[0].HttpOnly || !issued[0].Secure {
				t.Fatalf("unexpected renewal cookie attributes: %d cookies", len(issued))
			}
			if res.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("session renewal response must not be cached")
			}
			renewed := httptest.NewRequest(http.MethodGet, "https://example.test/api/auth/user", nil)
			renewed.AddCookie(issued[0])
			if got, err := cookies.ValidateSessionCookie(httptest.NewRecorder(), renewed); err != nil || got != id {
				t.Fatal("renewal must preserve the existing server session")
			}
		})
	}
}

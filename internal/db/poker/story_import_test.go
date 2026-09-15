package poker

import "testing"

func TestJiraStoryIdentity(t *testing.T) {
	for _, tc := range []struct{ key, link, want string }{
		{" test-1 ", " https://JIRA.example.com:443/jira/browse/test-1/?a=1#details ", "https://jira.example.com/jira/browse/TEST-1"},
		{"", "http://jira.example.com:80/browse/TEST-1", "http://jira.example.com/browse/TEST-1"},
		{"TEST-1", "https://jira.example.com:8443/jira/browse/TEST-1", "https://jira.example.com:8443/jira/browse/TEST-1"},
		{"TEST-1", "https://jira.example.com/browse/TEST-2", ""},
		{"TEST-1", "https://jira.example.com/other/TEST-1", ""},
		{"TEST-1", "https://jira.example.com/browse/TEST-1/more", ""},
		{"TEST-1", "https://user@jira.example.com/browse/TEST-1", ""},
		{"TEST-1", "", ""},
		{"", "not a URL", ""},
	} {
		t.Run(tc.link, func(t *testing.T) {
			if got := jiraStoryIdentity(tc.key, tc.link); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

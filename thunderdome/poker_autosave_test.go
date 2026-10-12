package thunderdome

import "testing"

func TestPokerAutoSavePoints(t *testing.T) {
	users := []*PokerUser{
		{ID: "one", Active: true}, {ID: "two", Active: true},
		{ID: "three", Active: true}, {ID: "four", Active: true},
		{ID: "spectator", Active: true, Spectator: true}, {ID: "offline"},
	}
	ballot := func(id, category, value string) *Vote {
		return &Vote{UserID: id, Category: category, VoteValue: value}
	}
	for _, tt := range []struct {
		name     string
		timedOut bool
		votes    []*Vote
		want     string
	}{
		{"manual below half", false, []*Vote{ballot("one", "testing", "5")}, "5"},
		{"timeout below half", true, []*Vote{ballot("one", "testing", "5")}, ""},
		{"timeout exactly half", true, []*Vote{ballot("one", "testing", "3"), ballot("two", "testing", "5")}, "4"},
		{"timeout above half", true, []*Vote{ballot("one", "testing", "2"), ballot("two", "frontend", "3"), ballot("three", "backend", "5")}, "10"},
		{"multiple categories count once", true, []*Vote{ballot("one", "testing", "2"), ballot("one", "frontend", "3"), ballot("one", "backend", "5")}, ""},
		{"zero counts as a vote", true, []*Vote{ballot("one", "testing", "0"), ballot("two", "testing", "0")}, "0"},
		{"abstentions do not count", true, []*Vote{ballot("one", "testing", "5"), ballot("two", "testing", "?")}, ""},
		{"spectator offline unknown do not satisfy quorum", true, []*Vote{ballot("one", "testing", "5"), ballot("spectator", "testing", "5"), ballot("offline", "testing", "5"), ballot("unknown", "testing", "5")}, ""},
		{"offline estimate retained once quorum met", true, []*Vote{ballot("one", "testing", "2"), ballot("two", "testing", "3"), ballot("offline", "backend", "5")}, "7.5"},
		{"empty manual round", false, nil, ""},
		{"empty expired round", true, nil, ""},
		{"manual abstentions", false, []*Vote{ballot("one", "testing", "?")}, ""},
		{"legacy majority", false, []*Vote{ballot("one", "", "3"), ballot("two", "", "3"), ballot("three", "", "5")}, "3"},
		{"legacy tie favors higher score", true, []*Vote{ballot("one", "", "3"), ballot("two", "", "5")}, "5"},
		{"legacy fraction", false, []*Vote{ballot("one", "", "1/2")}, "0.5"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := PokerAutoSavePoints(tt.votes, users, tt.timedOut)
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
	t.Run("odd participant count needs a rounded-up half", func(t *testing.T) {
		for voters := 1; voters <= 2; voters++ {
			votes := []*Vote{ballot("one", "testing", "5")}
			if voters == 2 {
				votes = append(votes, ballot("two", "testing", "5"))
			}
			points, _ := PokerAutoSavePoints(votes, users[:3], true)
			if (points != "") != (voters == 2) {
				t.Fatalf("unexpected autosave for %d of 3 voters: %q", voters, points)
			}
		}
	})
}

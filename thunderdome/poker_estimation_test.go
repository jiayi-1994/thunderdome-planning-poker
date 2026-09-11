package thunderdome

import "testing"

func TestCalculatePokerEstimation(t *testing.T) {
	users := []*PokerUser{{ID: "one", Active: true}, {ID: "two", Active: true}, {ID: "three", Active: true}, {ID: "observer", Spectator: true}}
	ballot := func(user, category, value string) *Vote {
		return &Vote{UserID: user, Category: category, VoteValue: value}
	}
	tests := []struct {
		name     string
		votes    []*Vote
		total    string
		averages []string
		counts   []int
	}{
		{"different group sizes", []*Vote{ballot("one", "testing", "2"), ballot("two", "testing", "3"), ballot("one", "frontend", "5"), ballot("one", "backend", "3"), ballot("two", "backend", "5"), ballot("three", "backend", "8")}, "12.83", []string{"2.5", "5", "5.33"}, []int{2, 1, 3}},
		{"abstentions and spectators excluded", []*Vote{ballot("one", "testing", "0"), ballot("two", "testing", "?"), ballot("three", "testing", ""), ballot("observer", "testing", "100"), ballot("missing", "testing", "100"), ballot("one", "frontend", "1/2"), ballot("two", "frontend", "☕️"), ballot("one", "backend", "2.5")}, "3", []string{"0", "0.5", "2.5"}, []int{1, 1, 1}},
		{"missing category is incomplete", []*Vote{ballot("one", "testing", "2"), ballot("one", "frontend", "3"), ballot("one", "backend", "?")}, "", []string{"2", "3", ""}, []int{1, 1, 0}},
		{"zero total", []*Vote{ballot("one", "testing", "0"), ballot("one", "frontend", "0"), ballot("one", "backend", "0")}, "0", []string{"0", "0", "0"}, []int{1, 1, 1}},
		{"round sum once", []*Vote{ballot("one", "testing", "1"), ballot("two", "testing", "0"), ballot("three", "testing", "0"), ballot("one", "frontend", "1"), ballot("two", "frontend", "0"), ballot("three", "frontend", "0"), ballot("one", "backend", "1"), ballot("two", "backend", "0"), ballot("three", "backend", "0")}, "1", []string{"0.33", "0.33", "0.33"}, []int{3, 3, 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculatePokerEstimation(tt.votes, users)
			if result == nil || result.Total != tt.total {
				t.Fatalf("got %+v, want total %q", result, tt.total)
			}
			for i, group := range result.Categories {
				if group.Category != PokerVoteCategories[i] || group.Average != tt.averages[i] || group.Count != tt.counts[i] {
					t.Errorf("unexpected group: %+v", group)
				}
			}
		})
	}
	if CalculatePokerEstimation([]*Vote{ballot("one", "", "8")}, users) != nil {
		t.Fatal("legacy votes must not be assigned to a category")
	}
}

func TestNumericPokerVote(t *testing.T) {
	for _, value := range []string{"", " ", "?", "☕️", "XS", "NaN", "+Inf", "-1", "3abc"} {
		if _, valid := NumericPokerVote(value); valid {
			t.Errorf("accepted invalid numeric vote %q", value)
		}
	}
	for _, value := range []string{"0", "0.5", "1/2", "13"} {
		if _, valid := NumericPokerVote(value); !valid {
			t.Errorf("rejected numeric vote %q", value)
		}
	}
}

func TestAllPokerUsersVoted(t *testing.T) {
	users := []*PokerUser{{ID: "one", Active: true}, {ID: "two", Active: true}, {ID: "observer", Active: true, Spectator: true}, {ID: "offline"}}
	votes := []*Vote{{UserID: "one", Category: "testing", VoteValue: "2"}, {UserID: "two", Category: "frontend", VoteValue: "3"}}
	if AllPokerUsersVoted(votes, users) {
		t.Fatal("must wait for the third category even when everyone has voted")
	}
	votes = append(votes, &Vote{UserID: "two", Category: "backend", VoteValue: "0"})
	if !AllPokerUsersVoted(votes, users) {
		t.Fatal("each user may vote in only their own discipline; ignore spectators and offline users")
	}
	users = append(users, &PokerUser{ID: "new", Active: true})
	if AllPokerUsersVoted(votes, users) {
		t.Fatal("must wait for active participants")
	}
	if AllPokerUsersVoted(nil, nil) {
		t.Fatal("an empty room cannot finish voting")
	}
}

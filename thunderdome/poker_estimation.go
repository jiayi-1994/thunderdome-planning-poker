package thunderdome

import (
	"math"
	"strconv"
	"strings"
)

var PokerVoteCategories = []string{"testing", "frontend", "backend"}

type PokerCategoryAverage struct {
	Category string `json:"category"`
	Average  string `json:"average"`
	Count    int    `json:"count"`
}

type PokerEstimation struct {
	Categories []PokerCategoryAverage `json:"categories"`
	Total      string                 `json:"total"`
}

func ValidPokerCategory(category string) bool {
	for _, candidate := range PokerVoteCategories {
		if category == candidate {
			return true
		}
	}
	return false
}

// NumericPokerVote excludes abstentions and hidden votes, while retaining explicit zeroes.
func NumericPokerVote(value string) (float64, bool) {
	if value == "1/2" {
		return 0.5, true
	}
	if strings.TrimSpace(value) == "" {
		return 0, false
	}
	n, err := strconv.ParseFloat(value, 64)
	return n, err == nil && !math.IsNaN(n) && !math.IsInf(n, 0) && n >= 0
}

func formatPokerPoints(value float64) string {
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(value, 'f', 2, 64), "0"), ".")
}

// CalculatePokerEstimation averages each discipline separately and rounds only the final sum.
// A nil result identifies legacy ballots; an empty Total identifies an incomplete estimate.
func CalculatePokerEstimation(votes []*Vote, users []*PokerUser) *PokerEstimation {
	grouped := false
	eligible := make(map[string]bool, len(users))
	for _, user := range users {
		eligible[user.ID] = !user.Spectator
	}
	for _, vote := range votes {
		if ValidPokerCategory(vote.Category) {
			grouped = true
		}
	}
	if !grouped {
		return nil
	}
	result := &PokerEstimation{Categories: make([]PokerCategoryAverage, 0, 3)}
	total, complete := 0.0, true
	for _, category := range PokerVoteCategories {
		group := PokerCategoryAverage{Category: category}
		sum := 0.0
		for _, vote := range votes {
			if vote.Category != category || !eligible[vote.UserID] {
				continue
			}
			if value, valid := NumericPokerVote(vote.VoteValue); valid {
				sum += value
				group.Count++
			}
		}
		if group.Count == 0 {
			complete = false
		} else {
			average := sum / float64(group.Count)
			group.Average = formatPokerPoints(average)
			total += average
		}
		result.Categories = append(result.Categories, group)
	}
	if complete && !math.IsInf(total, 0) {
		result.Total = formatPokerPoints(total)
	}
	return result
}

func AllPokerUsersVoted(votes []*Vote, users []*PokerUser) bool {
	voters := make(map[string]bool)
	for _, vote := range votes {
		voters[vote.UserID] = true
	}
	participants := 0
	for _, user := range users {
		if user.Active && !user.Spectator {
			participants++
			if !voters[user.ID] {
				return false
			}
		}
	}
	if estimation := CalculatePokerEstimation(votes, users); estimation != nil && estimation.Total == "" {
		return false
	}
	return participants > 0
}

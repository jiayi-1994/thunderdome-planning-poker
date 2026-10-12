package thunderdome

// PokerAutoSavePoints returns an empty value when a round should remain unsaved.
// Timeout quorum counts distinct active nonspectators with a numeric ballot.
// Category totals retain the existing estimation rules, including votes from users who left.
func PokerAutoSavePoints(votes []*Vote, users []*PokerUser, timedOut bool) (string, *PokerEstimation) {
	eligible := make(map[string]bool, len(users))
	active := make(map[string]bool, len(users))
	for _, user := range users {
		if !user.Spectator {
			eligible[user.ID] = true
			if user.Active {
				active[user.ID] = true
			}
		}
	}
	voters := make(map[string]bool)
	counts := make(map[float64]int)
	for _, vote := range votes {
		if !eligible[vote.UserID] || (vote.Category != "" && !ValidPokerCategory(vote.Category)) {
			continue
		}
		if value, valid := NumericPokerVote(vote.VoteValue); valid {
			if active[vote.UserID] {
				voters[vote.UserID] = true
			}
			counts[value]++
		}
	}
	if timedOut && (len(active) == 0 || len(voters)*2 < len(active)) {
		return "", nil
	}
	if estimation := CalculatePokerEstimation(votes, users); estimation != nil {
		return estimation.Total, estimation
	}
	// Legacy ballots use the same default as Save: most frequent, higher score on a tie.
	points, mostVotes := 0.0, 0
	for value, count := range counts {
		if count > mostVotes || (count == mostVotes && value > points) {
			points, mostVotes = value, count
		}
	}
	if mostVotes == 0 {
		return "", nil
	}
	return formatPokerPoints(points), nil
}

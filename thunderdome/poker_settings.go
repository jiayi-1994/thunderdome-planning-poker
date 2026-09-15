package thunderdome

// ValidPokerPointValue applies to individual cards, not computed totals or historical votes.
func ValidPokerPointValue(value string) bool {
	switch value {
	case "0", "1/2", "1", "2", "3", "5", "8":
		return true
	default:
		return false
	}
}

func ValidPokerVotingDuration(seconds int) bool {
	return seconds >= 60 && seconds <= 3600 && seconds%60 == 0
}

package thunderdome

import "time"

// PokerJiraSettings authorizes one game's future estimates to use a user's Jira connection.
type PokerJiraSettings struct {
	Enabled    bool   `json:"enabled"`
	InstanceID string `json:"instanceId"`
	FieldID    string `json:"fieldId"`
	FieldName  string `json:"fieldName"`
	Host       string `json:"host"`
}

type JiraNumericField struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// PokerJiraSync contains no credentials and can be shown to every participant.
type PokerJiraSync struct {
	Status    string    `json:"status"`
	IssueKey  string    `json:"issueKey"`
	Points    string    `json:"points"`
	Error     string    `json:"error,omitempty"`
	Attempts  int       `json:"attempts"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type PokerJiraSyncEvent struct {
	PokerID       string         `json:"-"`
	StoryID       string         `json:"planId"`
	VoteStartTime time.Time      `json:"voteStartTime"`
	Sync          *PokerJiraSync `json:"sync"`
}

// PokerJiraWrite captures the selected destination and the server-calculated point value.
type PokerJiraWrite struct {
	IssueKey string
	Link     string
	Host     string
	FieldID  string
	Points   string
}

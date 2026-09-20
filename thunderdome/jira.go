package thunderdome

import (
	"time"
)

type JiraInstance struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	Host           string    `json:"host"`
	ClientMail     string    `json:"client_mail"`
	AccessToken    string    `json:"access_token"`
	JiraDataCenter bool      `json:"jira_data_center"` // Checkbox for enabling Jira Data Center
	AuthMethod     string    `json:"auth_method"`      // Empty preserves legacy Cloud basic / Data Center PAT authentication.
	CreatedDate    time.Time `json:"created_date"`
	UpdatedDate    time.Time `json:"updated_date"`
}

type JiraConnectionStatus struct {
	Connected   bool   `json:"connected"`
	DisplayName string `json:"display_name,omitempty"`
}

// JiraIssueTypeOption contains only the fields needed by the import filter.
type JiraIssueTypeOption struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Subtask bool   `json:"subtask"`
}

// JiraSprintOption uses normalized active, future, or closed states.
type JiraSprintOption struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	State     string `json:"state"`
	BoardName string `json:"boardName"`
}

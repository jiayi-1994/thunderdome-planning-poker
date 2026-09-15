package poker

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/StevenWeathers/thunderdome-planning-poker/thunderdome"
)

var jiraImportKey = regexp.MustCompile(`^[A-Z][A-Z0-9_]*-[0-9]+$`)

// jiraStoryIdentity scopes an issue key to its Jira base URL, including context path.
// Unlinked/manual stories have no Jira identity and may share the same title.
func jiraStoryIdentity(referenceID, link string) string {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	path := strings.TrimRight(u.EscapedPath(), "/")
	base, key, ok := strings.Cut(path, "/browse/")
	key = strings.ToUpper(key)
	if !ok || !jiraImportKey.MatchString(key) || (strings.TrimSpace(referenceID) != "" && strings.ToUpper(strings.TrimSpace(referenceID)) != key) {
		return ""
	}
	host := strings.ToLower(u.Host)
	if (u.Scheme == "http" && u.Port() == "80") || (u.Scheme == "https" && u.Port() == "443") {
		host = strings.TrimSuffix(host, ":"+u.Port())
	}
	return u.Scheme + "://" + host + base + "/browse/" + key
}

// insertStory serializes additions within a meeting. The identity check runs in
// a separate statement after the lock so concurrent imports see committed rows.
// It preserves existing stories, votes and writeback jobs on duplicate imports.
func (d *Service) insertStory(ctx context.Context, tx *sql.Tx, pokerID string, story *thunderdome.Story) (bool, error) {
	var lockedID string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM thunderdome.poker WHERE id=$1 FOR UPDATE`, pokerID).Scan(&lockedID); err != nil {
		return false, fmt.Errorf("lock meeting for story import: %w", err)
	}
	if identity := jiraStoryIdentity(story.ReferenceID, story.Link); identity != "" {
		rows, err := tx.QueryContext(ctx, `SELECT reference_id, link FROM thunderdome.poker_story WHERE poker_id=$1 AND link IS NOT NULL`, pokerID)
		if err != nil {
			return false, err
		}
		duplicate := false
		for rows.Next() {
			var referenceID, link sql.NullString
			if err := rows.Scan(&referenceID, &link); err != nil {
				rows.Close()
				return false, err
			}
			if jiraStoryIdentity(referenceID.String, link.String) == identity {
				duplicate = true
				break
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return false, err
		}
		if duplicate {
			return false, nil
		}
	}
	priority := story.Priority
	if priority == 0 {
		priority = 99
	}
	err := tx.QueryRowContext(ctx, `INSERT INTO thunderdome.poker_story
		(poker_id, name, type, reference_id, link, description, acceptance_criteria, priority, position)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,
			COALESCE((SELECT max(position) FROM thunderdome.poker_story WHERE poker_id=$1), -1)+1)
		RETURNING id`, pokerID, story.Name, story.Type, story.ReferenceID, story.Link,
		d.HTMLSanitizerPolicy.Sanitize(story.Description), d.HTMLSanitizerPolicy.Sanitize(story.AcceptanceCriteria), priority).Scan(&story.ID)
	if err != nil {
		return false, fmt.Errorf("insert poker story: %w", err)
	}
	return true, nil
}

package jira

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/StevenWeathers/thunderdome-planning-poker/thunderdome"
)

const MaxSprintQueryLength = 200

const maxImportOptionsResponseBytes = 1024 * 1024

// IssueTypes reads the issue types available to the saved Jira account.
func IssueTypes(ctx context.Context, instance thunderdome.JiraInstance) ([]thunderdome.JiraIssueTypeOption, error) {
	req, err := newJiraRequest(ctx, instance, http.MethodGet, "issuetype", nil)
	if err != nil {
		return nil, err
	}
	body, err := readImportOptions(req)
	if err != nil {
		return nil, err
	}
	var options []thunderdome.JiraIssueTypeOption
	if json.Unmarshal(body, &options) != nil || options == nil {
		return nil, fmt.Errorf("Jira 未返回有效的问题类型列表，可使用手动 JQL 搜索")
	}
	for _, option := range options {
		if strings.TrimSpace(option.ID) == "" || strings.TrimSpace(option.Name) == "" {
			return nil, fmt.Errorf("Jira 未返回有效的问题类型列表，可使用手动 JQL 搜索")
		}
	}
	return options, nil
}

// Sprints uses Jira's picker API, including completed sprints. The picker is an
// internal Jira API and may be unavailable on some installations; callers can
// continue using manual JQL when that happens.
func Sprints(ctx context.Context, instance thunderdome.JiraInstance, query string) ([]thunderdome.JiraSprintOption, error) {
	if utf8.RuneCountInString(query) > MaxSprintQueryLength {
		return nil, fmt.Errorf("Sprint 搜索关键词不能超过 200 个字符")
	}
	params := url.Values{"excludeCompleted": {"false"}, "query": {query}}
	req, err := newJiraPathRequest(ctx, instance, http.MethodGet, "/rest/greenhopper/1.0/sprint/picker?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	body, err := readImportOptions(req)
	if err != nil {
		return nil, err
	}
	type pickerSprint struct {
		ID        int    `json:"id"`
		Name      string `json:"name"`
		StateKey  string `json:"stateKey"`
		BoardName string `json:"boardName"`
	}
	var response struct {
		Suggestions []pickerSprint `json:"suggestions"`
		AllMatches  []pickerSprint `json:"allMatches"`
	}
	if json.Unmarshal(body, &response) != nil || (response.Suggestions == nil && response.AllMatches == nil) {
		return nil, fmt.Errorf("Jira 未返回有效的 Sprint 列表，可使用手动 JQL 搜索")
	}
	options := make([]thunderdome.JiraSprintOption, 0, len(response.Suggestions)+len(response.AllMatches))
	seen := make(map[int]bool)
	for _, sprint := range append(response.Suggestions, response.AllMatches...) {
		if sprint.ID <= 0 || strings.TrimSpace(sprint.Name) == "" {
			return nil, fmt.Errorf("Jira 未返回有效的 Sprint 列表，可使用手动 JQL 搜索")
		}
		if seen[sprint.ID] {
			continue
		}
		seen[sprint.ID] = true
		state := strings.ToLower(sprint.StateKey)
		if state != "active" && state != "future" && state != "closed" {
			state = ""
		}
		options = append(options, thunderdome.JiraSprintOption{
			ID: sprint.ID, Name: sprint.Name, State: state, BoardName: sprint.BoardName,
		})
	}
	return options, nil
}

func readImportOptions(req *http.Request) ([]byte, error) {
	client := http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	response, err := client.Do(req)
	if err != nil {
		// Transport errors and Jira response bodies may contain credentials or
		// private URLs. Only return application-owned descriptions.
		if errors.Is(err, context.Canceled) {
			return nil, fmt.Errorf("Jira 筛选选项请求已取消")
		}
		return nil, fmt.Errorf("无法加载 Jira 筛选选项，请检查网络后重试，或使用手动 JQL 搜索")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		if response.StatusCode >= 300 && response.StatusCode < 400 {
			return nil, fmt.Errorf("Jira 筛选接口返回重定向，请检查 Jira 地址或 SSO 配置，或使用手动 JQL 搜索")
		}
		return nil, fmt.Errorf("无法加载 Jira 筛选选项（HTTP %d），请检查 Jira 版本、账号权限或使用手动 JQL 搜索", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxImportOptionsResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("读取 Jira 筛选选项失败或超时，请重试，或使用手动 JQL 搜索")
	}
	if len(body) > maxImportOptionsResponseBytes {
		return nil, fmt.Errorf("Jira 筛选选项响应过大，请缩小搜索范围，或使用手动 JQL 搜索")
	}
	return body, nil
}

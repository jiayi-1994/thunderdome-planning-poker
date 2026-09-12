package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/StevenWeathers/thunderdome-planning-poker/thunderdome"
)

var pointFieldPattern = regexp.MustCompile(`^customfield_[0-9]+$`)
var issueKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*-[0-9]+$`)

// These endpoints have the same payload in Cloud (v3) and Data Center (v2).
// https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issues/#api-rest-api-3-issue-issueidorkey-put
// https://developer.atlassian.com/server/jira/platform/updating-an-issue-via-the-jira-rest-apis-6848604/
func pointsRequest(ctx context.Context, instance thunderdome.JiraInstance, method, resource string, body io.Reader) (*http.Response, error) {
	req, err := newJiraRequest(ctx, instance, method, resource, body)
	if err != nil {
		return nil, err
	}
	client := http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	res, err := client.Do(req)
	if err != nil {
		// Do not expose request URLs, tokens, or untrusted response bodies to participants.
		return nil, fmt.Errorf("Jira 连接失败或请求超时")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		res.Body.Close()
		switch res.StatusCode {
		case http.StatusBadRequest:
			return nil, fmt.Errorf("Jira 拒绝更新，请检查点数字段及其编辑权限（400）")
		case http.StatusUnauthorized:
			return nil, fmt.Errorf("Jira 账号认证失败，请检查认证方式、用户名及密码或 Token（401）")
		case http.StatusForbidden:
			return nil, fmt.Errorf("Jira 账号没有访问或编辑权限（403）")
		case http.StatusNotFound:
			return nil, fmt.Errorf("Jira 需求不存在或账号无权访问（404）")
		default:
			return nil, fmt.Errorf("Jira 请求失败（HTTP %d）", res.StatusCode)
		}
	}
	return res, nil
}

// NumericFields lists only numeric custom fields, so users need not guess a site-specific ID.
func NumericFields(ctx context.Context, instance thunderdome.JiraInstance) ([]thunderdome.JiraNumericField, error) {
	res, err := pointsRequest(ctx, instance, http.MethodGet, "field", nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var fields []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Schema struct {
			Type string `json:"type"`
		} `json:"schema"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&fields); err != nil {
		return nil, fmt.Errorf("无法读取 Jira 字段列表")
	}
	result := make([]thunderdome.JiraNumericField, 0)
	for _, field := range fields {
		if field.Schema.Type == "number" && pointFieldPattern.MatchString(field.ID) {
			result = append(result, thunderdome.JiraNumericField{ID: field.ID, Name: field.Name})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

// WritePoints sets only the configured numeric field. Repeating this operation is idempotent.
func WritePoints(ctx context.Context, instance thunderdome.JiraInstance, write thunderdome.PokerJiraWrite) error {
	if !pointFieldPattern.MatchString(write.FieldID) || !issueKeyPattern.MatchString(write.IssueKey) {
		return fmt.Errorf("Jira 点数字段或需求编号无效")
	}
	base, err := url.Parse(strings.TrimRight(instance.Host, "/"))
	link, linkErr := url.Parse(write.Link)
	if err != nil || linkErr != nil || base.Host == "" || link.User != nil ||
		strings.TrimRight(instance.Host, "/") != strings.TrimRight(write.Host, "/") ||
		link.Scheme != base.Scheme || !strings.EqualFold(link.Host, base.Host) ||
		link.Path != strings.TrimRight(base.Path, "/")+"/browse/"+write.IssueKey {
		return fmt.Errorf("需求链接与所选 Jira 实例或需求编号不一致，请检查需求链接")
	}
	points, valid := thunderdome.NumericPokerVote(write.Points)
	if !valid {
		return fmt.Errorf("本轮没有有效数字评分，未回写 Jira")
	}
	body, err := json.Marshal(map[string]any{"fields": map[string]float64{write.FieldID: points}})
	if err != nil {
		return fmt.Errorf("点数格式无效")
	}
	res, err := pointsRequest(ctx, instance, http.MethodPut, "issue/"+write.IssueKey, bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 64<<10))
	return nil
}

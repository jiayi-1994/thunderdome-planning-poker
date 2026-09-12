package jira

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/StevenWeathers/thunderdome-planning-poker/thunderdome"
)

func newJiraRequest(ctx context.Context, instance thunderdome.JiraInstance, method, resource string, body io.Reader) (*http.Request, error) {
	base, err := url.Parse(instance.Host)
	if err != nil || base.Host == "" || (base.Scheme != "https" && base.Scheme != "http") || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return nil, fmt.Errorf("Jira 地址无效，请检查账号配置")
	}
	version := "3"
	if instance.JiraDataCenter {
		version = "2"
	}
	endpoint := strings.TrimRight(base.String(), "/") + "/rest/api/" + version + "/" + resource
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("无法创建 Jira 请求")
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if instance.JiraDataCenter && instance.AuthMethod != "basic" {
		req.Header.Set("Authorization", "Bearer "+instance.AccessToken)
	} else {
		req.SetBasicAuth(instance.ClientMail, instance.AccessToken)
	}
	return req, nil
}

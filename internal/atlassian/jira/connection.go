package jira

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/StevenWeathers/thunderdome-planning-poker/thunderdome"
)

// CheckConnection authenticates with the read-only current-user endpoint.
// Cloud: https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-myself/
// Server: https://developer.atlassian.com/server/jira/platform/rest/v10000/api-group-myself/
func CheckConnection(ctx context.Context, instance thunderdome.JiraInstance) (*thunderdome.JiraConnectionStatus, error) {
	req, err := newJiraRequest(ctx, instance, http.MethodGet, "myself", nil)
	if err != nil {
		return nil, err
	}
	client := http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	res, err := client.Do(req)
	if err != nil {
		// Never return the transport error: it may contain a URL or credentials.
		var dnsError *net.DNSError
		var certError *tls.CertificateVerificationError
		var netError net.Error
		switch {
		case errors.As(err, &certError):
			return nil, fmt.Errorf("Jira TLS 证书校验失败，请检查证书和服务端信任链")
		case errors.As(err, &dnsError):
			return nil, fmt.Errorf("无法解析 Jira 域名，请检查地址和服务器 DNS")
		case errors.As(err, &netError) && netError.Timeout():
			return nil, fmt.Errorf("Jira 连接超时，请检查地址、端口和网络连通性")
		default:
			return nil, fmt.Errorf("无法连接 Jira，请检查地址、端口和服务器网络")
		}
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		if strings.EqualFold(res.Header.Get("X-Seraph-LoginReason"), "AUTHENTICATION_DENIED") {
			return nil, fmt.Errorf("Jira 已阻止 API 登录，请先到 Jira 网页完成验证码或解除账号锁定")
		}
		switch res.StatusCode {
		case http.StatusUnauthorized:
			return nil, fmt.Errorf("Jira 认证失败：请检查用户名、密码或 Token 及认证方式（401）")
		case http.StatusForbidden:
			return nil, fmt.Errorf("Jira 拒绝访问：请检查账号权限、SSO 或验证码限制（403）")
		case http.StatusNotFound:
			return nil, fmt.Errorf("找不到 Jira API，请检查地址、路径前缀及 Cloud / Server 类型（404）")
		case http.StatusTooManyRequests:
			return nil, fmt.Errorf("Jira 请求过于频繁，请稍后重试（429）")
		default:
			if res.StatusCode >= 300 && res.StatusCode < 400 {
				return nil, fmt.Errorf("Jira 返回重定向，请填写最终访问地址或检查 SSO 配置")
			}
			return nil, fmt.Errorf("Jira 服务返回异常状态（HTTP %d）", res.StatusCode)
		}
	}
	const maxBody = 1024 * 1024
	body, err := io.ReadAll(io.LimitReader(res.Body, maxBody+1))
	if err != nil {
		return nil, fmt.Errorf("读取 Jira 响应失败或超时，请检查网络后重试")
	}
	var user struct {
		Name        string `json:"name"`
		Key         string `json:"key"`
		AccountID   string `json:"accountId"`
		DisplayName string `json:"displayName"`
		Active      *bool  `json:"active"`
	}
	if len(body) > maxBody || json.Unmarshal(body, &user) != nil || (user.Name == "" && user.Key == "" && user.AccountID == "") {
		return nil, fmt.Errorf("Jira 未返回有效用户信息，请检查地址或 SSO 登录跳转")
	}
	if user.Active != nil && !*user.Active {
		return nil, fmt.Errorf("Jira 账号已停用，请联系 Jira 管理员")
	}
	return &thunderdome.JiraConnectionStatus{Connected: true, DisplayName: user.DisplayName}, nil
}

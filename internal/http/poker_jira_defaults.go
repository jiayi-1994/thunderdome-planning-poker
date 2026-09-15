package http

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/StevenWeathers/thunderdome-planning-poker/internal/atlassian/jira"
	"github.com/StevenWeathers/thunderdome-planning-poker/thunderdome"
)

type pokerCreationMeta struct {
	JiraWritebackEnabled bool   `json:"jiraWritebackEnabled"`
	JiraWritebackWarning string `json:"jiraWritebackWarning,omitempty"`
}

// initializePokerJiraWriteback runs only for newly created games. It reads Jira
// metadata and stores local settings; the existing Save action queues point writes.
// Optional integration failures must not make a successfully created game look failed.
func (s *Service) initializePokerJiraWriteback(r *http.Request, pokerID, ownerID string) pokerCreationMeta {
	result := pokerCreationMeta{}
	userID, _ := r.Context().Value(contextKeyUserID).(string)
	if userID == "" || userID != ownerID || s.JiraDataSvc == nil {
		return result
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	instances, err := s.JiraDataSvc.FindInstancesByUserID(ctx, userID)
	if err != nil {
		result.JiraWritebackWarning = "会议已创建，但读取 Jira 配置失败，尚未开启点数回写。请在“Jira 点数回写”中检查配置。"
		return result
	}
	if len(instances) == 0 {
		return result
	}
	userType, _ := r.Context().Value(contextKeyUserType).(string)
	if s.Config.SubscriptionsEnabled && userType != thunderdome.AdminUserType {
		if err := s.SubscriptionDataSvc.CheckActiveSubscriber(ctx, userID); err != nil {
			result.JiraWritebackWarning = "会议已创建，但当前账号未开通 Jira 回写所需的订阅，尚未开启点数回写。"
			return result
		}
	}
	if len(instances) != 1 {
		result.JiraWritebackWarning = "会议已创建，但账号配置了多个 Jira 实例。请在“Jira 点数回写”中选择本次使用的实例并开启回写。"
		return result
	}
	instance := instances[0]
	// Never authorize another user's stored credentials, including creation on behalf of someone else.
	if instance.UserID != userID {
		result.JiraWritebackWarning = "会议已创建，但 Jira 配置不属于当前账号，尚未开启点数回写。"
		return result
	}
	fields, err := jira.NumericFields(ctx, instance)
	if err != nil {
		result.JiraWritebackWarning = "会议已创建，但无法读取 Jira 的 Story Points 字段，尚未开启点数回写。请在“Jira 点数回写”中检查连接和权限。"
		return result
	}
	var storyPoints []thunderdome.JiraNumericField
	for _, field := range fields {
		if strings.EqualFold(strings.TrimSpace(field.Name), "Story Points") {
			storyPoints = append(storyPoints, field)
		}
	}
	if len(storyPoints) != 1 {
		result.JiraWritebackWarning = "会议已创建，但未找到唯一的 Story Points 数字字段，尚未开启点数回写。请在“Jira 点数回写”中检查字段配置。"
		return result
	}
	settings := thunderdome.PokerJiraSettings{
		Enabled: true, InstanceID: instance.ID, Host: instance.Host,
		FieldID: storyPoints[0].ID, FieldName: storyPoints[0].Name,
	}
	if err := s.JiraDataSvc.SavePokerJiraSettings(ctx, pokerID, userID, settings); err != nil {
		result.JiraWritebackWarning = "会议已创建，但保存 Jira 回写设置失败。请在“Jira 点数回写”中重新开启。"
		return result
	}
	result.JiraWritebackEnabled = true
	return result
}

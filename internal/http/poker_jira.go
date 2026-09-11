package http

import (
	"encoding/json"
	"net/http"

	"github.com/StevenWeathers/thunderdome-planning-poker/internal/atlassian/jira"
	"github.com/StevenWeathers/thunderdome-planning-poker/internal/http/poker"
	"github.com/StevenWeathers/thunderdome-planning-poker/thunderdome"
)

type pokerJiraConnectionChoice struct {
	ID   string `json:"id"`
	Host string `json:"host"`
}

type pokerJiraConfigurationResponse struct {
	Settings  thunderdome.PokerJiraSettings `json:"settings"`
	Instances []pokerJiraConnectionChoice   `json:"instances"`
}

type pokerJiraSettingsRequest struct {
	Enabled    bool   `json:"enabled"`
	InstanceID string `json:"instanceId"`
	FieldID    string `json:"fieldId"`
}

func (s *Service) pokerJiraFacilitator(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	pokerID := r.PathValue("battleId")
	userID, ok := r.Context().Value(contextKeyUserID).(string)
	if !ok || validate.Var(pokerID, "required,uuid") != nil {
		s.Failure(w, r, http.StatusBadRequest, Errorf(EINVALID, "评点房间无效"))
		return "", "", false
	}
	if err := s.PokerDataSvc.ConfirmFacilitator(pokerID, userID); err != nil {
		s.Failure(w, r, http.StatusForbidden, Errorf(EUNAUTHORIZED, "只有主持人可以配置或重试 Jira 回写"))
		return "", "", false
	}
	return pokerID, userID, true
}

// handleGetPokerJiraSettings returns the game's settings and the caller's connection choices without credentials.
//
// @Summary Get poker Jira writeback settings
// @Description Requires a game facilitator. Connection choices belong to the caller and exclude credentials.
// @Tags poker, jira
// @Produce json
// @Param battleId path string true "Poker game ID"
// @Success 200 {object} standardJsonResponse{data=pokerJiraConfigurationResponse}
// @Failure 403 {object} standardJsonResponse
// @Security ApiKeyAuth
// @Router /battles/{battleId}/jira-writeback [get]
func (s *Service) handleGetPokerJiraSettings() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pokerID, userID, ok := s.pokerJiraFacilitator(w, r)
		if !ok {
			return
		}
		settings, err := s.JiraDataSvc.GetPokerJiraSettings(r.Context(), pokerID)
		if err != nil {
			s.Failure(w, r, http.StatusInternalServerError, Errorf(EINTERNAL, "读取 Jira 回写设置失败"))
			return
		}
		instances, err := s.JiraDataSvc.FindInstancesByUserID(r.Context(), userID)
		if err != nil {
			s.Failure(w, r, http.StatusInternalServerError, Errorf(EINTERNAL, "读取 Jira 账号失败"))
			return
		}
		connections := make([]pokerJiraConnectionChoice, 0, len(instances))
		for _, instance := range instances {
			connections = append(connections, pokerJiraConnectionChoice{ID: instance.ID, Host: instance.Host})
		}
		s.Success(w, r, http.StatusOK, pokerJiraConfigurationResponse{settings, connections}, nil)
	}
}

func (s *Service) ownPokerJiraInstance(w http.ResponseWriter, r *http.Request, userID, instanceID string) (thunderdome.JiraInstance, bool) {
	if validate.Var(instanceID, "required,uuid") != nil {
		s.Failure(w, r, http.StatusBadRequest, Errorf(EINVALID, "请选择 Jira 实例"))
		return thunderdome.JiraInstance{}, false
	}
	instance, err := s.JiraDataSvc.GetInstanceByID(r.Context(), instanceID)
	if err != nil || instance.UserID != userID {
		s.Failure(w, r, http.StatusForbidden, Errorf(EUNAUTHORIZED, "只能使用自己的 Jira 账号配置"))
		return thunderdome.JiraInstance{}, false
	}
	return instance, true
}

// handlePokerJiraFields lists the selected connection's numeric custom fields.
//
// @Summary List numeric Jira fields for poker writeback
// @Tags poker, jira
// @Produce json
// @Param battleId path string true "Poker game ID"
// @Param instanceId query string true "Caller-owned Jira connection ID"
// @Success 200 {object} standardJsonResponse{data=[]thunderdome.JiraNumericField}
// @Failure 403 {object} standardJsonResponse
// @Failure 502 {object} standardJsonResponse
// @Security ApiKeyAuth
// @Router /battles/{battleId}/jira-writeback/fields [get]
func (s *Service) handlePokerJiraFields() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, userID, ok := s.pokerJiraFacilitator(w, r)
		if !ok {
			return
		}
		instance, ok := s.ownPokerJiraInstance(w, r, userID, r.URL.Query().Get("instanceId"))
		if !ok {
			return
		}
		fields, err := jira.NumericFields(r.Context(), instance)
		if err != nil {
			s.Failure(w, r, http.StatusBadGateway, Errorf(EINTERNAL, err.Error()))
			return
		}
		s.Success(w, r, http.StatusOK, fields, nil)
	}
}

// handleSavePokerJiraSettings enables or disables automatic point writeback after voting ends.
//
// @Summary Configure automatic Jira point writeback
// @Description Facilitators can authorize their own Jira connection for future voting completions. The selected field must be numeric.
// @Tags poker, jira
// @Accept json
// @Produce json
// @Param battleId path string true "Poker game ID"
// @Param settings body pokerJiraSettingsRequest true "Jira writeback settings"
// @Success 200 {object} standardJsonResponse{data=thunderdome.PokerJiraSettings}
// @Failure 400 {object} standardJsonResponse
// @Failure 403 {object} standardJsonResponse
// @Failure 502 {object} standardJsonResponse
// @Security ApiKeyAuth
// @Router /battles/{battleId}/jira-writeback [put]
func (s *Service) handleSavePokerJiraSettings(pokerSvc *poker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pokerID, userID, ok := s.pokerJiraFacilitator(w, r)
		if !ok {
			return
		}
		var request pokerJiraSettingsRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&request); err != nil {
			s.Failure(w, r, http.StatusBadRequest, Errorf(EINVALID, "Jira 回写设置格式无效"))
			return
		}
		settings := thunderdome.PokerJiraSettings{Enabled: request.Enabled, InstanceID: request.InstanceID, FieldID: request.FieldID}
		if settings.Enabled {
			instance, ok := s.ownPokerJiraInstance(w, r, userID, settings.InstanceID)
			if !ok {
				return
			}
			fields, err := jira.NumericFields(r.Context(), instance)
			if err != nil {
				s.Failure(w, r, http.StatusBadGateway, Errorf(EINTERNAL, err.Error()))
				return
			}
			found := false
			for _, field := range fields {
				if field.ID == settings.FieldID {
					settings.FieldName = field.Name
					found = true
					break
				}
			}
			if !found {
				s.Failure(w, r, http.StatusBadRequest, Errorf(EINVALID, "请选择 Jira 中有效的数字字段"))
				return
			}
			settings.Host = instance.Host
		}
		if err := s.JiraDataSvc.SavePokerJiraSettings(r.Context(), pokerID, userID, settings); err != nil {
			s.Failure(w, r, http.StatusInternalServerError, Errorf(EINTERNAL, "保存 Jira 回写设置失败"))
			return
		}
		if pokerSvc != nil {
			for _, story := range s.PokerDataSvc.GetStories(pokerID, "") {
				if story.JiraSync != nil {
					pokerSvc.PublishJiraSync(&thunderdome.PokerJiraSyncEvent{PokerID: pokerID, StoryID: story.ID, VoteStartTime: story.VoteStartTime, Sync: story.JiraSync})
				}
			}
		}
		s.Success(w, r, http.StatusOK, settings, nil)
	}
}

// handleRetryPokerJiraSync requeues a failed write without changing its saved estimate.
//
// @Summary Retry failed Jira point writeback
// @Tags poker, jira
// @Produce json
// @Param battleId path string true "Poker game ID"
// @Param planId path string true "Story ID"
// @Success 202 {object} standardJsonResponse
// @Failure 403 {object} standardJsonResponse
// @Failure 409 {object} standardJsonResponse
// @Security ApiKeyAuth
// @Router /battles/{battleId}/plans/{planId}/jira-retry [post]
func (s *Service) handleRetryPokerJiraSync(pokerSvc *poker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pokerID, _, ok := s.pokerJiraFacilitator(w, r)
		if !ok {
			return
		}
		storyID := r.PathValue("planId")
		if validate.Var(storyID, "required,uuid") != nil {
			s.Failure(w, r, http.StatusBadRequest, Errorf(EINVALID, "需求编号无效"))
			return
		}
		if err := s.JiraDataSvc.RetryPokerJiraSync(r.Context(), pokerID, storyID); err != nil {
			s.Failure(w, r, http.StatusConflict, Errorf(ECONFLICT, "当前回写任务无法重试，请检查设置或重新评点"))
			return
		}
		stories := s.PokerDataSvc.GetStories(pokerID, "")
		for _, story := range stories {
			if story.ID == storyID && story.JiraSync != nil {
				pokerSvc.PublishJiraSync(&thunderdome.PokerJiraSyncEvent{PokerID: pokerID, StoryID: storyID, VoteStartTime: story.VoteStartTime, Sync: story.JiraSync})
				break
			}
		}
		s.Success(w, r, http.StatusAccepted, nil, nil)
	}
}

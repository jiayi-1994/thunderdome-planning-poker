package http

import (
	"net/http"
	"unicode/utf8"

	"github.com/StevenWeathers/thunderdome-planning-poker/internal/atlassian/jira"
	"github.com/StevenWeathers/thunderdome-planning-poker/thunderdome"
)

func (s *Service) jiraImportOptionsInstance(w http.ResponseWriter, r *http.Request) (thunderdome.JiraInstance, bool) {
	userID, instanceID := r.PathValue("userId"), r.PathValue("instanceId")
	if validate.Var(userID, "required,uuid") != nil || validate.Var(instanceID, "required,uuid") != nil {
		s.Failure(w, r, http.StatusBadRequest, Errorf(EINVALID, "Invalid Jira instance or user ID"))
		return thunderdome.JiraInstance{}, false
	}
	sessionUserID, _ := r.Context().Value(contextKeyUserID).(string)
	if sessionUserID == "" || sessionUserID != userID {
		s.Failure(w, r, http.StatusNotFound, Errorf(ENOTFOUND, "Jira instance not found"))
		return thunderdome.JiraInstance{}, false
	}
	instance, err := s.JiraDataSvc.GetInstanceByID(r.Context(), instanceID)
	if err != nil || instance.UserID != sessionUserID {
		s.Failure(w, r, http.StatusNotFound, Errorf(ENOTFOUND, "Jira instance not found"))
		return thunderdome.JiraInstance{}, false
	}
	return instance, true
}

// handleJiraIssueTypes returns the caller's Jira issue type choices.
//
//	@Summary	Get Jira issue types
//	@Tags		jira
//	@Produce	json
//	@Param		userId		path		string	true	"the user ID owning the Jira instance"
//	@Param		instanceId	path		string	true	"the Jira instance ID"
//	@Success	200			{object}	standardJsonResponse{data=[]thunderdome.JiraIssueTypeOption}
//	@Failure	422			{object}	standardJsonResponse
//	@Security	ApiKeyAuth
//	@Router		/users/{userId}/jira-instances/{instanceId}/issue-types [get]
func (s *Service) handleJiraIssueTypes() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		instance, ok := s.jiraImportOptionsInstance(w, r)
		if !ok {
			return
		}
		options, err := jira.IssueTypes(r.Context(), instance)
		if err != nil {
			s.Failure(w, r, http.StatusUnprocessableEntity, Errorf(EINVALID, err.Error()))
			return
		}
		s.Success(w, r, http.StatusOK, options, nil)
	}
}

// handleJiraSprints returns sprint choices matching a search term.
//
//	@Summary	Get Jira sprint suggestions
//	@Tags		jira
//	@Produce	json
//	@Param		userId		path		string	true	"the user ID owning the Jira instance"
//	@Param		instanceId	path		string	true	"the Jira instance ID"
//	@Param		query		query		string	false	"Sprint name, up to 200 characters"
//	@Success	200			{object}	standardJsonResponse{data=[]thunderdome.JiraSprintOption}
//	@Failure	422			{object}	standardJsonResponse
//	@Security	ApiKeyAuth
//	@Router		/users/{userId}/jira-instances/{instanceId}/sprints [get]
func (s *Service) handleJiraSprints() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		if utf8.RuneCountInString(query) > jira.MaxSprintQueryLength {
			s.Failure(w, r, http.StatusBadRequest, Errorf(EINVALID, "Sprint 搜索关键词不能超过 200 个字符"))
			return
		}
		instance, ok := s.jiraImportOptionsInstance(w, r)
		if !ok {
			return
		}
		options, err := jira.Sprints(r.Context(), instance, query)
		if err != nil {
			s.Failure(w, r, http.StatusUnprocessableEntity, Errorf(EINVALID, err.Error()))
			return
		}
		s.Success(w, r, http.StatusOK, options, nil)
	}
}

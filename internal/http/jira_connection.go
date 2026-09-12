package http

import (
	"net/http"

	"github.com/StevenWeathers/thunderdome-planning-poker/internal/atlassian/jira"
)

// handleJiraInstanceTest checks a caller-owned saved connection without changing Jira data.
//
// @Summary Test Jira connection
// @Description Validate the saved Jira credentials with the current-user API
// @Tags jira
// @Produce json
// @Param userId path string true "the user ID owning the Jira instance"
// @Param instanceId path string true "the Jira instance ID"
// @Success 200 {object} standardJsonResponse{data=thunderdome.JiraConnectionStatus}
// @Failure 422 {object} standardJsonResponse
// @Security ApiKeyAuth
// @Router /users/{userId}/jira-instances/{instanceId}/test [post]
func (s *Service) handleJiraInstanceTest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, instanceID := r.PathValue("userId"), r.PathValue("instanceId")
		if validate.Var(userID, "required,uuid") != nil || validate.Var(instanceID, "required,uuid") != nil {
			s.Failure(w, r, http.StatusBadRequest, Errorf(EINVALID, "Invalid Jira instance or user ID"))
			return
		}
		instance, err := s.JiraDataSvc.GetInstanceByID(r.Context(), instanceID)
		if err != nil || instance.UserID != userID {
			s.Failure(w, r, http.StatusNotFound, Errorf(ENOTFOUND, "Jira instance not found"))
			return
		}
		status, err := jira.CheckConnection(r.Context(), instance)
		if err != nil {
			// Upstream 401 is a Jira validation failure, not an expired Thunderdome session.
			s.Failure(w, r, http.StatusUnprocessableEntity, Errorf(EINVALID, err.Error()))
			return
		}
		s.Success(w, r, http.StatusOK, status, nil)
	}
}

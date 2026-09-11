package poker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/StevenWeathers/thunderdome-planning-poker/internal/atlassian/jira"
	"github.com/StevenWeathers/thunderdome-planning-poker/internal/wshub"
	"github.com/StevenWeathers/thunderdome-planning-poker/thunderdome"
	"go.uber.org/zap"
)

func (s *Service) PublishJiraSync(event *thunderdome.PokerJiraSyncEvent) {
	payload, err := json.Marshal(event)
	if err != nil {
		s.logger.Error("encode poker Jira sync", zap.Error(err))
		return
	}
	s.hub.Broadcast(wshub.Message{Room: event.PokerID, Data: wshub.CreateSocketEvent("jira_sync_updated", string(payload), "")})
}

func (s *Service) watchJiraWritebacks(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		queryCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		event, err := s.JiraService.ProcessPokerJiraSync(queryCtx, jira.WritePoints)
		cancel()
		if err != nil {
			s.logger.Error("process poker Jira writeback", zap.Error(err))
		} else if event != nil {
			s.PublishJiraSync(event)
			// Drain ready tasks without adding a one-second delay for each story.
			if ctx.Err() == nil {
				continue
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

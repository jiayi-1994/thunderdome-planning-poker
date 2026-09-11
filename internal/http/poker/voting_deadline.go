package poker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/StevenWeathers/thunderdome-planning-poker/internal/wshub"
	"go.uber.org/zap"
)

func (s *Service) watchVotingDeadlines(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		// Check immediately on startup to recover rounds that expired while the server was down.
		s.expireVoting(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) expireVoting(ctx context.Context) {
	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	expirations, err := s.PokerService.EndExpiredStoryVoting(queryCtx)
	if err != nil {
		s.logger.Error("check poker voting deadlines", zap.Error(err))
		return
	}
	for _, event := range expirations {
		event.Stories = s.PokerService.GetStories(event.PokerID, "")
		for _, story := range event.Stories {
			// A restart may have happened after the expiration transaction committed.
			if story.ID != event.StoryID || story.Active || !story.VoteStartTime.Equal(event.VoteStartTime) {
				continue
			}
			payload, err := json.Marshal(event)
			if err != nil {
				s.logger.Error("encode poker expiration", zap.Error(err))
				break
			}
			s.hub.Broadcast(wshub.Message{Room: event.PokerID, Data: wshub.CreateSocketEvent("voting_expired", string(payload), "")})
			break
		}
	}
}

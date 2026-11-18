package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"play-music/internal/model"
	"play-music/internal/store"
)

// RunPushScheduler runs the campaign sender and the device-revocation cleanup.
func (s *Server) RunPushScheduler(ctx context.Context) {
	go s.every(ctx, 20*time.Second, s.cleanupInactivePush)
	s.every(ctx, 15*time.Second, s.runPushCampaign)
}

func (s *Server) every(ctx context.Context, interval time.Duration, fn func(context.Context)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		fn(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// runPushCampaign sends at most one due campaign per tick. An interrupted or
// ambiguous send is marked unknown for manual review, never requeued.
func (s *Server) runPushCampaign(ctx context.Context) {
	if !s.pushEnabled() {
		return
	}
	if err := s.store.FailStalePushCampaigns(ctx); err != nil {
		s.log.Warn("push: mark stale campaigns", "err", err)
		return
	}
	campaign, err := s.store.ClaimPushCampaign(ctx)
	if errors.Is(err, store.ErrNotFound) {
		return
	}
	if err != nil {
		s.log.Warn("push: claim campaign", "err", err)
		return
	}

	work, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	msg := pushMessage{Title: campaign.Title, Body: campaign.Body, URL: campaign.URL}
	selected, queued, err := s.sendCampaign(work, campaign.ID, msg)
	if err == nil {
		s.finishCampaign(ctx, campaign.ID, store.CampaignCompleted, selected, queued, "")
		return
	}
	var failure *relayFailure
	if errors.As(err, &failure) {
		state := store.CampaignUnknown
		if failure.StatusCode >= 400 && failure.StatusCode < 500 {
			state = store.CampaignPartial
		}
		s.finishCampaign(ctx, campaign.ID, state, selected, queued,
			fmt.Sprintf("Resultado do lote incompleto (HTTP %d); confira antes de reenviar", failure.StatusCode))
		return
	}
	s.finishCampaign(ctx, campaign.ID, store.CampaignUnknown, selected, queued, "Não foi possível selecionar os destinatários")
}

func (s *Server) sendCampaign(ctx context.Context, campaignID string, msg pushMessage) (selected, queued int, err error) {
	payload, err := msg.payload()
	if err != nil {
		return 0, 0, err
	}
	ids, err := s.store.ActivePushRelayIDs(ctx)
	if err != nil {
		return 0, 0, err
	}
	selected = len(ids)
	for start := 0; start < len(ids); start += 100 {
		end := min(start+100, len(ids))
		var result relaySendResult
		status, e := s.relay(ctx, http.MethodPost, "/api/send", "",
			map[string]any{"payload": payload, "subscriptionIds": ids[start:end]}, &result)
		if e != nil {
			return selected, queued, &relayFailure{StatusCode: status}
		}
		queued += result.Data.Queued
		if err = s.store.UpdatePushCampaignProgress(ctx, campaignID, selected, queued); err != nil {
			return selected, queued, err
		}
	}
	return selected, queued, nil
}

func (s *Server) finishCampaign(parent context.Context, id, status string, selected, queued int, detail string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 5*time.Second)
	defer cancel()
	if err := s.store.FinishPushCampaign(ctx, id, status, selected, queued, detail); err != nil {
		s.log.Warn("push: finish campaign", "id", id, "err", err)
	}
}

// cleanupInactivePush revokes opted-out devices at the relay. The relay call
// happens outside any transaction: the claim only leases the row by bumping
// updated_at.
func (s *Server) cleanupInactivePush(ctx context.Context) {
	if !s.pushEnabled() {
		return
	}
	sub, err := s.store.ClaimInactivePushSubscription(ctx)
	if err != nil {
		return
	}
	if err := s.revokeRemoteSubscription(ctx, sub); err != nil {
		return
	}
	if err := s.store.DeleteInactivePushSubscription(ctx, sub.Endpoint); err != nil {
		s.log.Warn("push: delete inactive subscription", "endpoint", sub.Endpoint, "err", err)
	}
}

// revokeRemoteSubscription deletes the device at the relay. A 401 means the
// stored management token is no longer valid, so it enrolls the stored
// subscription data again to obtain a fresh token and retries the delete.
func (s *Server) revokeRemoteSubscription(ctx context.Context, sub *model.PushSubscription) error {
	body := map[string]string{"endpoint": sub.Endpoint}
	status, err := s.relay(ctx, http.MethodDelete, s.relayPath("/subscriptions"), sub.ManagementToken, body, nil)
	if err == nil {
		return nil
	}
	if status != http.StatusUnauthorized {
		return err
	}
	var stored pushSubscription
	if err := json.Unmarshal(sub.SubscriptionData, &stored); err != nil {
		return err
	}
	credentials, _, err := s.enroll(ctx, stored)
	if err != nil {
		return err
	}
	_, err = s.relay(ctx, http.MethodDelete, s.relayPath("/subscriptions"), credentials.token, body, nil)
	return err
}

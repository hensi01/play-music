package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"play-music/internal/model"
)

const lockEndpointSQL = "SELECT pg_advisory_xact_lock(hashtextextended($1,0))"

// deactivatePushSQL is the state change both deactivations make. It rotates
// operation_id, which is what stops a late enrollment reply from reactivating a
// device that has since been turned off.
const deactivatePushSQL = `
	UPDATE push_subscriptions
	SET active=false, operation_id=md5(random()::text || clock_timestamp()::text), updated_at=now()`

// Campaign statuses. The store writes them, the scheduler transitions between
// them, and the API returns them, so the vocabulary lives in one place. A
// campaign in `sending` when the process dies is `unknown`, never requeued.
const (
	CampaignPending   = "pending"
	CampaignSending   = "sending"
	CampaignCompleted = "completed"
	CampaignPartial   = "partial"
	CampaignCancelled = "cancelled"
	CampaignUnknown   = "unknown"
)

func (s *Store) PushAccessActive(ctx context.Context, userID string) (bool, error) {
	var active bool
	err := s.pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM push_subscriptions WHERE user_id=$1 AND active)", userID).Scan(&active)
	return active, err
}

func (s *Store) PushEndpointActive(ctx context.Context, endpoint, userID string) (bool, error) {
	var active bool
	err := s.pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM push_subscriptions WHERE endpoint=$1 AND user_id=$2 AND active)",
		endpoint, userID).Scan(&active)
	return active, err
}

// ReservePushEnrollment records an inactive row for the endpoint before the
// relay is contacted, so a lost relay response can still be recovered. Fails
// with ErrOwnedByOther when the device belongs to a different account.
func (s *Store) ReservePushEnrollment(ctx context.Context, endpoint, userID, operationID string, subscriptionData []byte) error {
	return dbTx(ctx, s, func(q queryer) error {
		if _, err := q.Exec(ctx, lockEndpointSQL, endpoint); err != nil {
			return err
		}
		var owner *string
		err := q.QueryRow(ctx,
			"SELECT user_id FROM push_subscriptions WHERE endpoint=$1 FOR UPDATE", endpoint).Scan(&owner)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil && owner != nil && *owner != userID {
			return ErrOwnedByOther
		}
		_, err = q.Exec(ctx, `
			INSERT INTO push_subscriptions(endpoint,user_id,relay_id,management_token,subscription_data,active,operation_id)
			VALUES($1,$2,$3,'',$4,false,$5)
			ON CONFLICT(endpoint) DO UPDATE SET user_id=excluded.user_id,
				subscription_data=excluded.subscription_data, active=false,
				operation_id=excluded.operation_id, updated_at=now()
			WHERE push_subscriptions.user_id=excluded.user_id OR push_subscriptions.user_id IS NULL`,
			endpoint, userID, "pending-"+NewID(), subscriptionData, operationID)
		return err
	})
}

// ActivatePushSubscription stores the relay credentials, but only while the
// enrollment reserve is still the current operation for this user and device.
func (s *Store) ActivatePushSubscription(ctx context.Context, endpoint, userID, operationID, relayID, managementToken string, subscriptionData []byte) error {
	return dbTx(ctx, s, func(q queryer) error {
		if _, err := q.Exec(ctx, lockEndpointSQL, endpoint); err != nil {
			return err
		}
		var owner *string
		var current string
		err := q.QueryRow(ctx,
			"SELECT user_id,operation_id FROM push_subscriptions WHERE endpoint=$1 FOR UPDATE",
			endpoint).Scan(&owner, &current)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrStale
		}
		if err != nil {
			return err
		}
		if owner == nil || *owner != userID || current != operationID {
			return ErrStale
		}
		tag, err := q.Exec(ctx, `
			INSERT INTO push_subscriptions(endpoint,user_id,relay_id,management_token,subscription_data,active)
			VALUES($1,$2,$3,$4,$5,true)
			ON CONFLICT(endpoint) DO UPDATE SET relay_id=excluded.relay_id,
				management_token=excluded.management_token,
				subscription_data=excluded.subscription_data, active=true, updated_at=now()
			WHERE push_subscriptions.user_id=excluded.user_id OR push_subscriptions.user_id IS NULL`,
			endpoint, userID, relayID, managementToken, subscriptionData)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrStale
		}
		return nil
	})
}

// DeactivatePushSubscription turns one device off. A device that is not stored
// at all is not an error; a device owned by somebody else is ErrOwnedByOther.
func (s *Store) DeactivatePushSubscription(ctx context.Context, endpoint, userID string) error {
	return dbTx(ctx, s, func(q queryer) error {
		if _, err := q.Exec(ctx, lockEndpointSQL, endpoint); err != nil {
			return err
		}
		var owner *string
		err := q.QueryRow(ctx,
			"SELECT user_id FROM push_subscriptions WHERE endpoint=$1 FOR UPDATE", endpoint).Scan(&owner)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if owner == nil || *owner != userID {
			return ErrOwnedByOther
		}
		_, err = q.Exec(ctx, deactivatePushSQL+" WHERE endpoint=$1 AND user_id=$2", endpoint, userID)
		return err
	})
}

// DeactivateUserPushSubscriptions turns off every device of a user (account
// deletion). The scheduler revokes them remotely afterwards.
func (s *Store) DeactivateUserPushSubscriptions(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, deactivatePushSQL+" WHERE user_id=$1", userID)
	return err
}

// ActivePushRelayIDs returns the relay ids of every active subscription of a
// known user, ordered so the batching is deterministic.
func (s *Store) ActivePushRelayIDs(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT relay_id FROM push_subscriptions WHERE user_id IS NOT NULL AND active ORDER BY relay_id")
	if err != nil {
		return nil, err
	}
	return collect(rows, func(row scanner) (string, error) {
		var id string
		err := row.Scan(&id)
		return id, err
	})
}

// ClaimInactivePushSubscription leases one inactive device for remote cleanup.
// The claim bumps updated_at, which also spaces out retries of a failing
// endpoint, so the relay call never runs inside an open transaction.
func (s *Store) ClaimInactivePushSubscription(ctx context.Context) (*model.PushSubscription, error) {
	var sub model.PushSubscription
	err := s.pool.QueryRow(ctx, `
		UPDATE push_subscriptions SET updated_at=now()
		WHERE endpoint = (
			SELECT endpoint FROM push_subscriptions
			WHERE (NOT active OR user_id IS NULL) AND updated_at < now()-interval '2 minutes'
			ORDER BY updated_at FOR UPDATE SKIP LOCKED LIMIT 1
		)
		RETURNING endpoint, management_token, subscription_data`).
		Scan(&sub.Endpoint, &sub.ManagementToken, &sub.SubscriptionData)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (s *Store) DeleteInactivePushSubscription(ctx context.Context, endpoint string) error {
	_, err := s.pool.Exec(ctx,
		"DELETE FROM push_subscriptions WHERE endpoint=$1 AND (NOT active OR user_id IS NULL)", endpoint)
	return err
}

const pushCampaignCols = `
	id, title, body, url, scheduled_at, status, selected, queued, detail`

func scanPushCampaign(row scanner) (*model.PushCampaign, error) {
	var c model.PushCampaign
	err := row.Scan(&c.ID, &c.Title, &c.Body, &c.URL, &c.ScheduledAt,
		&c.Status, &c.Selected, &c.Queued, &c.Detail)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) CreatePushCampaign(ctx context.Context, c *model.PushCampaign, createdBy string) error {
	// The status is the store's to decide: a new campaign is pending, whatever
	// the caller left in the struct.
	c.Status = CampaignPending
	_, err := s.pool.Exec(ctx, `
		INSERT INTO push_campaigns(id,created_by,title,body,url,scheduled_at,status)
		VALUES($1,$2,$3,$4,$5,$6,$7)`,
		c.ID, createdBy, c.Title, c.Body, c.URL, c.ScheduledAt, c.Status)
	return err
}

func (s *Store) ListPushCampaigns(ctx context.Context) ([]model.PushCampaign, error) {
	rows, err := s.pool.Query(ctx, "SELECT "+pushCampaignCols+
		" FROM push_campaigns ORDER BY created_at DESC LIMIT 100")
	if err != nil {
		return nil, err
	}
	campaigns, err := collect(rows, scanPushCampaign)
	if err != nil {
		return nil, err
	}
	out := make([]model.PushCampaign, len(campaigns))
	for i := range campaigns {
		out[i] = *campaigns[i]
	}
	return out, nil
}

// CancelPushCampaign cancels a pending campaign; ErrNotPending when it already
// left the queue.
func (s *Store) CancelPushCampaign(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE push_campaigns SET status=$2,finished_at=now()
		WHERE id=$1 AND status=$3`, id, CampaignCancelled, CampaignPending)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrNotPending
	}
	return nil
}

// FailStalePushCampaigns marks campaigns whose send was interrupted as unknown:
// an ambiguous send is never requeued automatically.
func (s *Store) FailStalePushCampaigns(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE push_campaigns
		SET status=$1,detail='Envio interrompido; verifique antes de reenviar',finished_at=now()
		WHERE status=$2 AND started_at < now()-interval '5 minutes'`, CampaignUnknown, CampaignSending)
	return err
}

// ClaimPushCampaign marks the next due campaign as sending and returns it.
// ErrNotFound when nothing is due.
func (s *Store) ClaimPushCampaign(ctx context.Context) (*model.PushCampaign, error) {
	c, err := scanPushCampaign(s.pool.QueryRow(ctx, `
		UPDATE push_campaigns SET status=$1,started_at=now()
		WHERE id=(
			SELECT id FROM push_campaigns
			WHERE status=$2 AND scheduled_at<=now()
			ORDER BY scheduled_at,id FOR UPDATE SKIP LOCKED LIMIT 1
		)
		RETURNING `+pushCampaignCols, CampaignSending, CampaignPending))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

// UpdatePushCampaignProgress records how many devices were selected and queued
// so far, while the campaign is still sending.
func (s *Store) UpdatePushCampaignProgress(ctx context.Context, id string, selected, queued int) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE push_campaigns SET selected=$2,queued=$3
		WHERE id=$1 AND status=$4`, id, selected, queued, CampaignSending)
	return err
}

func (s *Store) FinishPushCampaign(ctx context.Context, id, status string, selected, queued int, detail string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE push_campaigns
		SET status=$2,selected=$3,queued=$4,detail=$5,finished_at=now()
		WHERE id=$1 AND status=$6`, id, status, selected, queued, detail, CampaignSending)
	return err
}

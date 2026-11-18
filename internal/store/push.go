package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"play-music/internal/model"
)

const lockEndpointSQL = "SELECT pg_advisory_xact_lock(hashtextextended($1,0))"

// SubscriptionKeys are the browser encryption keys of one device. They identify
// the device: the same browser re-registering keeps them, and a new registration
// changes them.
type SubscriptionKeys struct {
	P256dh string `json:"p256dh"`
	Auth   string `json:"auth"`
}

// sameSubscriptionKeys reports whether stored subscription data belongs to the
// device described by keys. Unparseable stored data counts as a different device.
func sameSubscriptionKeys(stored []byte, keys SubscriptionKeys) bool {
	var previous struct {
		Keys SubscriptionKeys `json:"keys"`
	}
	if json.Unmarshal(stored, &previous) != nil {
		return false
	}
	return previous.Keys == keys
}

// rotateOperationIDSQL replaces the operation id with a fresh one, which is what
// stops a late enrollment reply from reactivating a device that has since been
// turned off or unlinked.
const rotateOperationIDSQL = "md5(random()::text || clock_timestamp()::text)"

// linkSubscriptionUserSQL attaches an account to a browser subscription. The
// design allows one device to serve several accounts, so this is idempotent.
const linkSubscriptionUserSQL = "INSERT INTO push_subscription_users(endpoint,user_id) VALUES($1,$2) ON CONFLICT DO NOTHING"

// relinkSubscriptionOwnerSQL points the row at whichever account still uses the
// device, and turns it inactive once none is left.
const relinkSubscriptionOwnerSQL = `
	UPDATE push_subscriptions
	SET user_id=(SELECT user_id FROM push_subscription_users WHERE endpoint=$1 LIMIT 1),
	    active=EXISTS(SELECT 1 FROM push_subscription_users WHERE endpoint=$1),
	    operation_id=` + rotateOperationIDSQL + `, updated_at=now()
	WHERE endpoint=$1`

func relinkSubscriptionOwner(ctx context.Context, q queryer, endpoint string) error {
	_, err := q.Exec(ctx, relinkSubscriptionOwnerSQL, endpoint)
	return err
}

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

// PushAccessActive reports whether the user has any active subscription. A
// browser subscription can belong to several accounts, so the association lives
// in push_subscription_users, not on the row itself.
func (s *Store) PushAccessActive(ctx context.Context, userID string) (bool, error) {
	var active bool
	err := s.pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM push_subscriptions p JOIN push_subscription_users u ON u.endpoint=p.endpoint WHERE u.user_id=$1 AND p.active)",
		userID).Scan(&active)
	return active, err
}

func (s *Store) PushEndpointActive(ctx context.Context, endpoint, userID string) (bool, error) {
	var active bool
	err := s.pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM push_subscriptions p JOIN push_subscription_users u ON u.endpoint=p.endpoint WHERE p.endpoint=$1 AND u.user_id=$2 AND p.active)",
		endpoint, userID).Scan(&active)
	return active, err
}

// ReservePushEnrollment records an inactive row for the endpoint before the
// relay is contacted, so a lost relay response can still be recovered.
//
// A browser subscription can belong to several accounts, so a device that is
// already active with the same keys only needs the account linked: linked is
// true and the caller must not enroll again. Different keys mean the device
// re-registered, which is ErrStale.
func (s *Store) ReservePushEnrollment(ctx context.Context, endpoint, userID, operationID string, subscriptionData []byte, keys SubscriptionKeys) (linked bool, err error) {
	err = dbTx(ctx, s, func(q queryer) error {
		if _, err := q.Exec(ctx, lockEndpointSQL, endpoint); err != nil {
			return err
		}
		var stored []byte
		var active bool
		err := q.QueryRow(ctx,
			"SELECT subscription_data,active FROM push_subscriptions WHERE endpoint=$1 FOR UPDATE",
			endpoint).Scan(&stored, &active)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil && active {
			if !sameSubscriptionKeys(stored, keys) {
				return ErrStale
			}
			if _, err := q.Exec(ctx, linkSubscriptionUserSQL, endpoint, userID); err != nil {
				return err
			}
			linked = true
			return nil
		}
		_, err = q.Exec(ctx, `
			INSERT INTO push_subscriptions(endpoint,user_id,relay_id,management_token,subscription_data,active,operation_id)
			VALUES($1,$2,$3,'',$4,false,$5)
			ON CONFLICT(endpoint) DO UPDATE SET user_id=excluded.user_id,
				subscription_data=excluded.subscription_data, active=false,
				operation_id=excluded.operation_id, updated_at=now()`,
			endpoint, userID, "pending-"+NewID(), subscriptionData, operationID)
		return err
	})
	return linked, err
}

// ActivatePushSubscription stores the relay credentials, but only while the
// enrollment reserve is still the current operation for this user and device,
// and links the account to the shared device.
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
				subscription_data=excluded.subscription_data, active=true, updated_at=now()`,
			endpoint, userID, relayID, managementToken, subscriptionData)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrStale
		}
		_, err = q.Exec(ctx, linkSubscriptionUserSQL, endpoint, userID)
		return err
	})
}

// DeactivatePushSubscription unlinks one account from a shared device. The
// device itself stays registered while any other account still uses it, and is
// only marked inactive when the last one leaves. An unknown device is not an
// error; the scheduler revokes inactive devices at the relay.
func (s *Store) DeactivatePushSubscription(ctx context.Context, endpoint, userID string) error {
	return dbTx(ctx, s, func(q queryer) error {
		if _, err := q.Exec(ctx, lockEndpointSQL, endpoint); err != nil {
			return err
		}
		tag, err := q.Exec(ctx,
			"DELETE FROM push_subscription_users WHERE endpoint=$1 AND user_id=$2", endpoint, userID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			// A reservation that never became active belongs to one account only,
			// so releasing it here is what stops a late enrollment reply.
			_, err = q.Exec(ctx, `
				UPDATE push_subscriptions
				SET user_id=NULL, operation_id=`+rotateOperationIDSQL+`, updated_at=now()
				WHERE endpoint=$1 AND user_id=$2 AND NOT active`, endpoint, userID)
			return err
		}
		return relinkSubscriptionOwner(ctx, q, endpoint)
	})
}

// DeleteUserWithPushSubscriptions deletes the account and unlinks it from every
// device it was registered on. A device stays active while another account still
// uses it; the scheduler revokes the ones left inactive.
func (s *Store) DeleteUserWithPushSubscriptions(ctx context.Context, userID string) error {
	return dbTx(ctx, s, func(q queryer) error {
		rows, err := q.Query(ctx,
			"SELECT endpoint FROM push_subscription_users WHERE user_id=$1 ORDER BY endpoint", userID)
		if err != nil {
			return err
		}
		endpoints, err := collect(rows, func(row scanner) (string, error) {
			var endpoint string
			err := row.Scan(&endpoint)
			return endpoint, err
		})
		if err != nil {
			return err
		}
		for _, endpoint := range endpoints {
			if _, err := q.Exec(ctx, lockEndpointSQL, endpoint); err != nil {
				return err
			}
			if _, err := q.Exec(ctx,
				"DELETE FROM push_subscription_users WHERE endpoint=$1 AND user_id=$2", endpoint, userID); err != nil {
				return err
			}
			if err := relinkSubscriptionOwner(ctx, q, endpoint); err != nil {
				return err
			}
		}
		return notFoundIfEmpty(q.Exec(ctx, "DELETE FROM users WHERE id=$1", userID))
	})
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
	id, title, body, url, scheduled_at, status, selected, queued, detail, repeat_hours`

func scanPushCampaign(row scanner) (*model.PushCampaign, error) {
	var c model.PushCampaign
	err := row.Scan(&c.ID, &c.Title, &c.Body, &c.URL, &c.ScheduledAt,
		&c.Status, &c.Selected, &c.Queued, &c.Detail, &c.RepeatHours)
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
		INSERT INTO push_campaigns(id,created_by,title,body,url,scheduled_at,status,repeat_hours)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)`,
		c.ID, createdBy, c.Title, c.Body, c.URL, c.ScheduledAt, c.Status, c.RepeatHours)
	return err
}

// ListPushCampaigns returns the most recent campaigns plus every repeating one
// still in flight, so the admin view never hides an active repeat.
func (s *Store) ListPushCampaigns(ctx context.Context) ([]model.PushCampaign, error) {
	rows, err := s.pool.Query(ctx, "SELECT "+pushCampaignCols+`
		FROM push_campaigns
		WHERE (repeat_hours > 0 AND status IN ($1,$2))
		   OR id IN (SELECT id FROM push_campaigns ORDER BY created_at DESC LIMIT 100)
		ORDER BY created_at DESC`, CampaignPending, CampaignSending)
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

// CancelPushCampaign cancels a pending campaign, and also stops a repeating one
// that is mid-send. ErrNotPending when there is nothing left to cancel.
func (s *Store) CancelPushCampaign(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE push_campaigns
		SET status=CASE WHEN status=$3 THEN $2 ELSE status END,
		    finished_at=CASE WHEN status=$3 THEN now() ELSE finished_at END,
		    repeat_hours=0
		WHERE id=$1 AND (status=$3 OR (status=$4 AND repeat_hours>0))`,
		id, CampaignCancelled, CampaignPending, CampaignSending)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrNotPending
	}
	return nil
}

// FailStalePushCampaigns marks campaigns whose send was interrupted as unknown:
// an ambiguous send is never requeued automatically. A stale repeat stops too,
// since resuming it could send the same notification twice.
func (s *Store) FailStalePushCampaigns(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE push_campaigns
		SET status=$1,repeat_hours=0,
		    detail='Envio interrompido; repetição encerrada. Verifique antes de reenviar',
		    finished_at=now()
		WHERE status=$2 AND started_at < now()-interval '5 minutes'`, CampaignUnknown, CampaignSending)
	return err
}

// ClaimPushCampaign marks the next due campaign as sending, publishes it to the
// inbox and returns it. ErrNotFound when nothing is due.
func (s *Store) ClaimPushCampaign(ctx context.Context) (*model.PushCampaign, error) {
	c, err := scanPushCampaign(s.pool.QueryRow(ctx, `
		WITH claimed AS (
			UPDATE push_campaigns SET status=$1,started_at=now()
			WHERE id=(
				SELECT id FROM push_campaigns
				WHERE status=$2 AND scheduled_at<=now()
				ORDER BY scheduled_at,id FOR UPDATE SKIP LOCKED LIMIT 1
			)
			RETURNING `+pushCampaignCols+`
		), published AS (
			INSERT INTO notification_inbox(id,title,body,url,published_at)
			SELECT $3,title,body,url,now() FROM claimed
			RETURNING id
		)
		SELECT `+pushCampaignCols+` FROM claimed CROSS JOIN published`,
		CampaignSending, CampaignPending, NewID()))
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

// FinishPushCampaign records the outcome. A completed repeat goes back to
// pending with its next scheduled time; anything else stops repeating, because
// resuming an interrupted send risks a duplicate notification.
func (s *Store) FinishPushCampaign(ctx context.Context, id, status string, selected, queued int, detail string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE push_campaigns
		SET status=CASE WHEN $2=$4 AND repeat_hours>0 THEN $5 ELSE $2 END,
		    scheduled_at=CASE WHEN $2=$4 AND repeat_hours>0
		        THEN now() + repeat_hours * interval '1 hour' ELSE scheduled_at END,
		    selected=$3,queued=$6,detail=$7,finished_at=now(),started_at=NULL,
		    repeat_hours=CASE WHEN $2=$4 THEN repeat_hours ELSE 0 END
		WHERE id=$1 AND status=$8`,
		id, status, selected, CampaignCompleted, CampaignPending, queued, detail, CampaignSending)
	return err
}

// NotificationInbox returns the most recent published notifications.
func (s *Store) NotificationInbox(ctx context.Context) ([]model.Notification, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id,title,body,url,published_at
		FROM notification_inbox ORDER BY published_at DESC,id DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	return collect(rows, func(row scanner) (model.Notification, error) {
		var n model.Notification
		err := row.Scan(&n.ID, &n.Title, &n.Body, &n.URL, &n.PublishedAt)
		return n, err
	})
}

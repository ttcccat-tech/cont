package storage

import (
	"database/sql"
	"encoding/json"
	"log"
	"time"

	"github.com/lib/pq"
)

// WebhookSubscription represents a webhook subscription for an org
type WebhookSubscription struct {
	ID         string    `json:"id"`
	OrgID      string    `json:"org_id"`
	URL        string    `json:"url" binding:"required,url"`
	EventTypes []string  `json:"event_types" binding:"required,min=1,dive,oneof=api_key.approved api_key.rejected alert.triggered subscription.expired"`
	Secret     string    `json:"secret,omitempty"` // HMAC signing secret, write-only
	Active     bool      `json:"active"`
	CreatedAt  time.Time `json:"created_at"`
}

// WebhookDelivery represents a webhook delivery attempt record
type WebhookDelivery struct {
	ID              string     `json:"id"`
	OrgID           string     `json:"org_id"`
	WebhookID       string     `json:"webhook_id"`
	EventType       string     `json:"event_type"`
	Payload         string     `json:"payload"`
	Status          string     `json:"status"` // pending, success, failed, retrying
	Attempts        int        `json:"attempts"`
	LastAttempt     *time.Time `json:"last_attempt,omitempty"`
	NextRetry       *time.Time `json:"next_retry,omitempty"`
	LastError       string     `json:"last_error,omitempty"`
	ResponseStatus  int        `json:"response_status,omitempty"`
	ResponseBody    string     `json:"response_body,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// ── Webhook Subscription store methods ───────────────────────────────────

// ListWebhookSubscriptions returns all webhook subscriptions for an org
func (s *Store) ListWebhookSubscriptions(orgID string) ([]WebhookSubscription, error) {
	query := `
		SELECT id, org_id, url, event_types, secret, active, created_at
		FROM webhook_subscriptions
		WHERE org_id = $1
		ORDER BY created_at DESC`
	rows, err := s.db.Query(query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WebhookSubscription
	for rows.Next() {
		var sub WebhookSubscription
		var eventTypes []byte
		var secret sql.NullString
		var active sql.NullBool
		var created sql.NullString
		if err := rows.Scan(&sub.ID, &sub.OrgID, &sub.URL, &eventTypes, &secret, &active, &created); err != nil {
			return nil, err
		}
		jsonScanSlice(&sub.EventTypes, eventTypes)
		if secret.Valid {
			sub.Secret = secret.String
		}
		if active.Valid {
			sub.Active = active.Bool
		}
		if created.Valid {
			if t, err := time.Parse("2006-01-02T15:04:05Z", created.String); err == nil {
				sub.CreatedAt = t
			}
		}
		out = append(out, sub)
	}
	return out, nil
}

// GetWebhookSubscription returns a single webhook subscription
func (s *Store) GetWebhookSubscription(id, orgID string) (*WebhookSubscription, error) {
	query := `
		SELECT id, org_id, url, event_types, secret, active, created_at
		FROM webhook_subscriptions
		WHERE id = $1 AND org_id = $2`
	var sub WebhookSubscription
	var eventTypes []byte
	var secret sql.NullString
	var active sql.NullBool
	var created sql.NullString
	err := s.db.QueryRow(query, id, orgID).Scan(&sub.ID, &sub.OrgID, &sub.URL, &eventTypes, &secret, &active, &created)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	jsonScanSlice(&sub.EventTypes, eventTypes)
	if secret.Valid {
		sub.Secret = secret.String
	}
	if active.Valid {
		sub.Active = active.Bool
	}
	if created.Valid {
		if t, err := time.Parse("2006-01-02T15:04:05Z", created.String); err == nil {
			sub.CreatedAt = t
		}
	}
	return &sub, nil
}

// CreateWebhookSubscription creates a new webhook subscription
func (s *Store) CreateWebhookSubscription(sub *WebhookSubscription) (*WebhookSubscription, error) {
	err := s.db.QueryRow(`
		INSERT INTO webhook_subscriptions (org_id, url, event_types, secret, active)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`,
		sub.OrgID, sub.URL, pq.Array(sub.EventTypes), nullString(sub.Secret), true,
	).Scan(&sub.ID, &sub.CreatedAt)
	if err != nil {
		return nil, err
	}
	sub.Active = true
	return sub, nil
}

// DeleteWebhookSubscription deletes a webhook subscription and its deliveries
func (s *Store) DeleteWebhookSubscription(id, orgID string) error {
	_, err := s.db.Exec(`DELETE FROM webhook_subscriptions WHERE id=$1 AND org_id=$2`, id, orgID)
	return err
}

func (s *Store) UpdateWebhookSubscription(id, orgID, url string, eventTypes []string, active bool) error {
	_, err := s.db.Exec(`
		UPDATE webhook_subscriptions
		SET url=$3, event_types=$4, active=$5
		WHERE id=$1 AND org_id=$2`,
		id, orgID, url, pq.Array(eventTypes), active)
	return err
}

// ── Webhook Delivery store methods ─────────────────────────────────────────

// ListWebhookDeliveries returns delivery records for a webhook subscription
func (s *Store) ListWebhookDeliveries(webhookID, orgID string, limit, offset int) ([]WebhookDelivery, error) {
	query := `
		SELECT id, org_id, webhook_id, event_type, payload, status, attempts,
		       last_attempt, next_retry, last_error, response_status, response_body, created_at
		FROM webhook_deliveries
		WHERE webhook_id = $1 AND org_id = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4`
	rows, err := s.db.Query(query, webhookID, orgID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WebhookDelivery
	for rows.Next() {
		var d WebhookDelivery
		var payload []byte
		var lastAttempt, nextRetry sql.NullString
		var lastError, responseBody sql.NullString
		var responseStatus sql.NullInt64
		var created sql.NullString
		if err := rows.Scan(&d.ID, &d.OrgID, &d.WebhookID, &d.EventType, &payload,
			&d.Status, &d.Attempts, &lastAttempt, &nextRetry, &lastError, &responseStatus, &responseBody, &created); err != nil {
			return nil, err
		}
		d.Payload = string(payload)
		if lastAttempt.Valid {
			if t, err := time.Parse("2006-01-02T15:04:05Z", lastAttempt.String); err == nil {
				d.LastAttempt = &t
			}
		}
		if nextRetry.Valid {
			if t, err := time.Parse("2006-01-02T15:04:05Z", nextRetry.String); err == nil {
				d.NextRetry = &t
			}
		}
		if lastError.Valid {
			d.LastError = lastError.String
		}
		if responseStatus.Valid {
			d.ResponseStatus = int(responseStatus.Int64)
		}
		if responseBody.Valid {
			d.ResponseBody = responseBody.String
		}
		if created.Valid {
			if t, err := time.Parse("2006-01-02T15:04:05Z", created.String); err == nil {
				d.CreatedAt = t
			}
		}
		out = append(out, d)
	}
	return out, nil
}

// GetWebhookDelivery returns a single delivery record
func (s *Store) GetWebhookDelivery(id, orgID string) (*WebhookDelivery, error) {
	query := `
		SELECT id, org_id, webhook_id, event_type, payload, status, attempts,
		       last_attempt, next_retry, last_error, response_status, response_body, created_at
		FROM webhook_deliveries
		WHERE id = $1 AND org_id = $2`
	var d WebhookDelivery
	var payload []byte
	var lastAttempt, nextRetry sql.NullString
	var lastError, responseBody sql.NullString
	var responseStatus sql.NullInt64
	var created sql.NullString
	err := s.db.QueryRow(query, id, orgID).Scan(&d.ID, &d.OrgID, &d.WebhookID, &d.EventType, &payload,
		&d.Status, &d.Attempts, &lastAttempt, &nextRetry, &lastError, &responseStatus, &responseBody, &created)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	d.Payload = string(payload)
	if lastAttempt.Valid {
		if t, err := time.Parse("2006-01-02T15:04:05Z", lastAttempt.String); err == nil {
			d.LastAttempt = &t
		}
	}
	if nextRetry.Valid {
		if t, err := time.Parse("2006-01-02T15:04:05Z", nextRetry.String); err == nil {
			d.NextRetry = &t
		}
	}
	if lastError.Valid {
		d.LastError = lastError.String
	}
	if responseStatus.Valid {
		d.ResponseStatus = int(responseStatus.Int64)
	}
	if responseBody.Valid {
		d.ResponseBody = responseBody.String
	}
	if created.Valid {
		if t, err := time.Parse("2006-01-02T15:04:05Z", created.String); err == nil {
			d.CreatedAt = t
		}
	}
	return &d, nil
}

// CreateWebhookDelivery creates a new delivery record
func (s *Store) CreateWebhookDelivery(d *WebhookDelivery) (*WebhookDelivery, error) {
	err := s.db.QueryRow(`
		INSERT INTO webhook_deliveries (org_id, webhook_id, event_type, payload, status, attempts)
		VALUES ($1, $2, $3, $4, 'pending', 0)
		RETURNING id, created_at`,
		d.OrgID, d.WebhookID, d.EventType, d.Payload,
	).Scan(&d.ID, &d.CreatedAt)
	if err != nil {
		return nil, err
	}
	d.Status = "pending"
	d.Attempts = 0
	return d, nil
}

// UpdateWebhookDelivery updates a delivery record after an attempt
func (s *Store) UpdateWebhookDelivery(d *WebhookDelivery) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	var nextRetry *string
	if d.NextRetry != nil {
		s := d.NextRetry.UTC().Format("2006-01-02T15:04:05Z")
		nextRetry = &s
	}
	var lastError *string
	if d.LastError != "" {
		s := d.LastError
		lastError = &s
	}
	var responseBody *string
	if d.ResponseBody != "" {
		s := d.ResponseBody
		responseBody = &s
	}
	_, err := s.db.Exec(`
		UPDATE webhook_deliveries SET
			status = $3, attempts = $4, last_attempt = $5, next_retry = $6,
			last_error = $7, response_status = $8, response_body = $9
		WHERE id = $1 AND org_id = $2`,
		d.ID, d.OrgID, d.Status, d.Attempts, now, nextRetry, lastError, d.ResponseStatus, responseBody,
	)
	return err
}

// GetPendingWebhookDeliveries returns deliveries that are due for retry
func (s *Store) GetPendingWebhookDeliveries(limit int) ([]WebhookDelivery, error) {
	query := `
		SELECT id, org_id, webhook_id, event_type, payload, status, attempts,
		       last_attempt, next_retry, last_error, response_status, response_body, created_at
		FROM webhook_deliveries
		WHERE status IN ('pending', 'retrying') AND (next_retry IS NULL OR next_retry <= NOW())
		ORDER BY created_at ASC
		LIMIT $1`
	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WebhookDelivery
	for rows.Next() {
		var d WebhookDelivery
		var payload []byte
		var lastAttempt, nextRetry sql.NullString
		var lastError, responseBody sql.NullString
		var responseStatus sql.NullInt64
		var created sql.NullString
		if err := rows.Scan(&d.ID, &d.OrgID, &d.WebhookID, &d.EventType, &payload,
			&d.Status, &d.Attempts, &lastAttempt, &nextRetry, &lastError, &responseStatus, &responseBody, &created); err != nil {
			return nil, err
		}
		d.Payload = string(payload)
		if lastAttempt.Valid {
			if t, err := time.Parse("2006-01-02T15:04:05Z", lastAttempt.String); err == nil {
				d.LastAttempt = &t
			}
		}
		if nextRetry.Valid {
			if t, err := time.Parse("2006-01-02T15:04:05Z", nextRetry.String); err == nil {
				d.NextRetry = &t
			}
		}
		if lastError.Valid {
			d.LastError = lastError.String
		}
		if responseStatus.Valid {
			d.ResponseStatus = int(responseStatus.Int64)
		}
		if responseBody.Valid {
			d.ResponseBody = responseBody.String
		}
		if created.Valid {
			if t, err := time.Parse("2006-01-02T15:04:05Z", created.String); err == nil {
				d.CreatedAt = t
			}
		}
		out = append(out, d)
	}
	return out, nil
}

// FireWebhooks finds all matching active subscriptions and enqueues deliveries
func (s *Store) FireWebhooks(orgID, eventType string, payload interface{}) error {
	// Get active subscriptions for this org matching the event type
	query := `
		SELECT id, org_id, url, event_types, secret, active, created_at
		FROM webhook_subscriptions
		WHERE org_id = $1 AND active = true`
	rows, err := s.db.Query(query, orgID)
	if err != nil {
		return err
	}
	defer rows.Close()

	payloadBytes, _ := json.Marshal(payload)
	for rows.Next() {
		var sub WebhookSubscription
		var eventTypes []byte
		var secret sql.NullString
		var active sql.NullBool
		var created sql.NullString
		if err := rows.Scan(&sub.ID, &sub.OrgID, &sub.URL, &eventTypes, &secret, &active, &created); err != nil {
			continue
		}
		jsonScanSlice(&sub.EventTypes, eventTypes)

		// Check if this subscription handles this event type
		handled := false
		for _, t := range sub.EventTypes {
			if t == eventType || t == "*" {
				handled = true
				break
			}
		}
		if !handled {
			continue
		}

		// Create delivery record
		d := &WebhookDelivery{
			OrgID:     orgID,
			WebhookID: sub.ID,
			EventType: eventType,
			Payload:   string(payloadBytes),
		}
		if _, err := s.CreateWebhookDelivery(d); err != nil {
			log.Printf("failed to create webhook delivery for event %s: %v", eventType, err)
			continue
		}
		log.Printf("enqueued webhook delivery: event=%s webhook=%s", eventType, sub.ID)
	}
	return nil
}

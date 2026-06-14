package engine

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/ttcccat-tech/cont/admin-api/storage"
)

const (
	maxAttempts     = 5
	deliveryBatch  = 20
	deliveryTick   = 5 * time.Second
	baseRetryDelay = 30 * time.Second
)

// WebhookDeliveryEngine processes pending webhook deliveries in the background.
type WebhookDeliveryEngine struct {
	store    *storage.Store
	interval time.Duration
	stopCh   chan struct{}
	wg       sync.WaitGroup
	httpCli  *http.Client
}

// NewWebhookDeliveryEngine creates a new engine that processes pending deliveries.
func NewWebhookDeliveryEngine(store *storage.Store) *WebhookDeliveryEngine {
	return &WebhookDeliveryEngine{
		store:    store,
		interval: deliveryTick,
		stopCh:   make(chan struct{}),
		httpCli: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Start begins the background delivery loop.
func (e *WebhookDeliveryEngine) Start() {
	e.wg.Add(1)
	go e.run()
	log.Printf("[webhook-delivery] started, processing every %v", e.interval)
}

// Stop gracefully stops the engine.
func (e *WebhookDeliveryEngine) Stop() {
	close(e.stopCh)
	e.wg.Wait()
	log.Printf("[webhook-delivery] stopped")
}

func (e *WebhookDeliveryEngine) run() {
	defer e.wg.Done()
	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()
	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.processPending()
		}
	}
}

func (e *WebhookDeliveryEngine) processPending() {
	deliveries, err := e.store.GetPendingWebhookDeliveries(deliveryBatch)
	if err != nil {
		log.Printf("[webhook-delivery] GetPendingWebhookDeliveries error: %v", err)
		return
	}

	for _, d := range deliveries {
		e.deliver(&d)
	}
}

func (e *WebhookDeliveryEngine) deliver(d *storage.WebhookDelivery) {
	// Get the webhook subscription for URL + secret
	sub, err := e.store.GetWebhookSubscription(d.WebhookID, d.OrgID)
	if err != nil || sub == nil {
		log.Printf("[webhook-delivery] could not load subscription %s: %v", d.WebhookID, err)
		e.markFailed(d, "subscription not found")
		return
	}

	// Build request
	req, err := http.NewRequest(http.MethodPost, sub.URL, bytes.NewBufferString(d.Payload))
	if err != nil {
		log.Printf("[webhook-delivery] bad URL %s: %v", sub.URL, err)
		e.markFailed(d, fmt.Sprintf("bad URL: %v", err))
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Cont-Webhook/1.0")
	req.Header.Set("X-Webhook-Event", d.EventType)
	req.Header.Set("X-Webhook-Delivery-ID", d.ID)

	// Sign with HMAC-SHA256 if secret is set
	if sub.Secret != "" {
		sig := e.signPayload(d.Payload, sub.Secret)
		req.Header.Set("X-Webhook-Signature", sig)
	}

	resp, err := e.httpCli.Do(req)
	d.Attempts++
	now := time.Now()

	if err != nil {
		log.Printf("[webhook-delivery] delivery %s failed (attempt %d): %v", d.ID, d.Attempts, err)
		e.handleRetry(d, now, fmt.Sprintf("request error: %v", err))
		return
	}

	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	bodyStr := string(body)
	if bodyStr == "" {
		bodyStr = "(empty)"
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("[webhook-delivery] delivery %s succeeded (attempt %d, status=%d)", d.ID, d.Attempts, resp.StatusCode)
		d.Status = "success"
		d.LastAttempt = &now
		d.ResponseStatus = resp.StatusCode
		d.ResponseBody = bodyStr
		d.NextRetry = nil
		e.store.UpdateWebhookDelivery(d)
		return
	}

	// Non-2xx — may be retryable
	log.Printf("[webhook-delivery] delivery %s got %d (attempt %d): %s", d.ID, resp.StatusCode, d.Attempts, bodyStr)
	d.LastAttempt = &now
	d.ResponseStatus = resp.StatusCode
	d.ResponseBody = bodyStr
	d.LastError = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncate(bodyStr, 200))
	e.handleRetry(d, now, d.LastError)
}

func (e *WebhookDeliveryEngine) handleRetry(d *storage.WebhookDelivery, now time.Time, lastError string) {
	d.LastError = lastError
	d.LastAttempt = &now

	if d.Attempts >= maxAttempts {
		d.Status = "failed"
		d.NextRetry = nil
		log.Printf("[webhook-delivery] delivery %s exhausted retries (%d attempts)", d.ID, d.Attempts)
	} else {
		d.Status = "retrying"
		// Exponential backoff: 30s, 60s, 120s, 240s, 480s
		delay := baseRetryDelay * time.Duration(1<<uint(d.Attempts-1))
		next := now.Add(delay)
		d.NextRetry = &next
		log.Printf("[webhook-delivery] delivery %s scheduled retry in %v", d.ID, delay)
	}

	e.store.UpdateWebhookDelivery(d)
}

func (e *WebhookDeliveryEngine) markFailed(d *storage.WebhookDelivery, reason string) {
	d.Status = "failed"
	d.LastError = reason
	now := time.Now()
	d.LastAttempt = &now
	d.NextRetry = nil
	e.store.UpdateWebhookDelivery(d)
}

// signPayload computes HMAC-SHA256 of payload with the given secret.
// Returns "sha256=<hex>" format, compatible with GitHub/GitLab webhook signatures.
func (e *WebhookDeliveryEngine) signPayload(payload, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// TriggerWebhook is called by alert engine / API to fire a webhook event.
func TriggerWebhook(store *storage.Store, orgID, eventType string, eventPayload interface{}) {
	if err := store.FireWebhooks(orgID, eventType, eventPayload); err != nil {
		log.Printf("[webhook-delivery] FireWebhooks error for event %s: %v", eventType, err)
	}
}

// WebhookEvent is the standard payload format for webhook deliveries.
type WebhookEvent struct {
	Event     string      `json:"event"`
	Timestamp time.Time   `json:"timestamp"`
	OrgID     string      `json:"org_id"`
	Data      interface{} `json:"data"`
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// MarshalWebhookEvent builds a webhook event payload for delivery.
func MarshalWebhookEvent(eventType, orgID string, data interface{}) map[string]interface{} {
	return map[string]interface{}{
		"event":     eventType,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"org_id":    orgID,
		"data":      data,
	}
}

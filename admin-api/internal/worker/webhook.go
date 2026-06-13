package worker

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/ttcccat-tech/cont/admin-api/storage"
)

// WebhookWorker processes webhook deliveries with retry logic
type WebhookWorker struct {
	store       *storage.Store
	poolSize     int
	pollInterval time.Duration
	wg           sync.WaitGroup
	stopCh       chan struct{}
}

const (
	maxAttempts     = 3
	retryDelayBase  = 1 * time.Second
	retryDelayMax   = 30 * time.Second
	httpTimeout     = 10 * time.Second
)

// NewWebhookWorker creates a new webhook worker
func NewWebhookWorker(store *storage.Store, poolSize int) *WebhookWorker {
	if poolSize <= 0 {
		poolSize = 10
	}
	return &WebhookWorker{
		store:       store,
		poolSize:    poolSize,
		pollInterval: 5 * time.Second,
		stopCh:     make(chan struct{}),
	}
}

// Start begins the worker pool
func (w *WebhookWorker) Start() {
	log.Printf("[webhook-worker] starting with pool size %d", w.poolSize)
	for i := 0; i < w.poolSize; i++ {
		w.wg.Add(1)
		go w.worker(i)
	}
}

// Stop gracefully shuts down the worker pool
func (w *WebhookWorker) Stop() {
	log.Printf("[webhook-worker] stopping...")
	close(w.stopCh)
	w.wg.Wait()
	log.Printf("[webhook-worker] stopped")
}

func (w *WebhookWorker) worker(id int) {
	defer w.wg.Done()
	log.Printf("[webhook-worker:%d] started", id)

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			log.Printf("[webhook-worker:%d] received stop signal", id)
			return
		case <-ticker.C:
			w.processBatch()
		}
	}
}

func (w *WebhookWorker) processBatch() {
	deliveries, err := w.store.GetPendingWebhookDeliveries(w.poolSize * 2)
	if err != nil {
		log.Printf("[webhook-worker] failed to fetch pending deliveries: %v", err)
		return
	}

	for _, d := range deliveries {
		select {
		case <-w.stopCh:
			return
		default:
			w.processDelivery(&d)
		}
	}
}

func (w *WebhookWorker) processDelivery(d *storage.WebhookDelivery) {
	// Get webhook subscription for URL and secret
	sub, err := w.store.GetWebhookSubscription(d.WebhookID, d.OrgID)
	if err != nil || sub == nil || !sub.Active {
		// Mark as failed if subscription not found or inactive
		d.Status = "failed"
		d.LastError = "subscription not found or inactive"
		w.store.UpdateWebhookDelivery(d)
		return
	}

	// Attempt delivery
	statusCode, responseBody, err := w.deliverWebhook(sub.URL, sub.Secret, d.EventType, d.Payload)

	now := time.Now()
	d.Attempts++
	d.LastAttempt = &now
	d.ResponseStatus = statusCode
	d.ResponseBody = truncateString(responseBody, 1000)

	if err != nil || statusCode < 200 || statusCode >= 300 {
		d.LastError = ""
		if err != nil {
			d.LastError = err.Error()
		} else {
			d.LastError = fmt.Sprintf("HTTP %d: %s", statusCode, truncateString(responseBody, 200))
		}

		if d.Attempts >= maxAttempts {
			d.Status = "failed"
			d.NextRetry = nil
			log.Printf("[webhook-worker] delivery %s failed permanently after %d attempts: %s",
				d.ID, d.Attempts, d.LastError)
		} else {
			// Exponential backoff: 1s -> 5s -> 30s
			delaySecs := powInt(5, d.Attempts-1)
			delay := time.Duration(delaySecs) * time.Second
			if delay > retryDelayMax {
				delay = retryDelayMax
			}
			next := now.Add(delay)
			d.NextRetry = &next
			d.Status = "retrying"
			log.Printf("[webhook-worker] delivery %s attempt %d failed, retrying in %v: %s",
				d.ID, d.Attempts, delay, d.LastError)
		}
	} else {
		d.Status = "success"
		d.NextRetry = nil
		log.Printf("[webhook-worker] delivery %s succeeded (HTTP %d)", d.ID, statusCode)
	}

	if err := w.store.UpdateWebhookDelivery(d); err != nil {
		log.Printf("[webhook-worker] failed to update delivery %s: %v", d.ID, err)
	}
}

func (w *WebhookWorker) deliverWebhook(url, secret, eventType, payload string) (int, string, error) {
	// Build request body
	body := map[string]interface{}{
		"event":     eventType,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"data":      json.RawMessage(payload),
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return 0, "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return 0, "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Cont-Webhook/1.0")
	req.Header.Set("X-Webhook-Event", eventType)
	req.Header.Set("X-Webhook-Timestamp", time.Now().UTC().Format(time.RFC3339))

	// Sign payload with HMAC-SHA256 if secret is provided
	if secret != "" {
		sig := hmacSHA256(secret, bodyBytes)
		req.Header.Set("X-Webhook-Signature", "sha256="+sig)
	}

	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	return resp.StatusCode, string(respBody), nil
}

func hmacSHA256(secret string, payload []byte) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

func powInt(base, exp int) int {
	result := 1
	for i := 0; i < exp; i++ {
		result *= base
	}
	return result
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

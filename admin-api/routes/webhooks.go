package routes

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ttcccat-tech/cont/admin-api/storage"
)

// WebhookSubscription represents a webhook subscription for an org
type WebhookSubscription struct {
	ID         string    `json:"id"`
	OrgID      string    `json:"org_id"`
	URL        string    `json:"url" binding:"required,url"`
	EventTypes []string  `json:"event_types" binding:"required,min=1"`
	Secret     string    `json:"secret,omitempty"`
	Active     bool      `json:"active"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// WebhookDelivery represents a webhook delivery attempt record
type WebhookDelivery struct {
	ID            string     `json:"id"`
	OrgID         string     `json:"org_id"`
	WebhookID     string     `json:"webhook_id"`
	EventType     string     `json:"event_type"`
	Payload       string     `json:"payload"`
	Status        string     `json:"status"`
	Attempts      int        `json:"attempts"`
	LastError     string     `json:"last_error,omitempty"`
	ResponseBody  string     `json:"response_body,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	DeliveredAt   *time.Time `json:"delivered_at,omitempty"`
}

// WebhookJob represents a job to be processed by the webhook worker pool
type WebhookJob struct {
	DeliveryID string
	WebhookID  string
	OrgID      string
	URL        string
	Secret     string
	EventType  string
	Payload    string
}

// StartWebhookWorker starts the goroutine pool worker for webhook delivery
func StartWebhookWorker(store *storage.Store, poolSize int, jobChannel chan WebhookJob) {
	if poolSize <= 0 {
		poolSize = 10
	}
	log.Printf("[webhook-worker] starting with pool size %d", poolSize)

	for i := 0; i < poolSize; i++ {
		go webhookWorkerGoroutine(i, store, jobChannel)
	}
}

func webhookWorkerGoroutine(id int, store *storage.Store, jobChannel chan WebhookJob) {
	log.Printf("[webhook-worker:%d] started", id)
	for job := range jobChannel {
		processWebhookJob(&job, store)
	}
	log.Printf("[webhook-worker:%d] stopped", id)
}

func processWebhookJob(job *WebhookJob, store *storage.Store) {
	delivery, err := store.GetWebhookDelivery(job.DeliveryID, job.OrgID)
	if err != nil || delivery == nil {
		log.Printf("[webhook-worker] failed to get delivery %s: %v", job.DeliveryID, err)
		return
	}

	// Attempt delivery with retries
	success := deliverWithRetry(job, store, delivery)

	if !success {
		log.Printf("[webhook-worker] delivery %s failed after retries", job.DeliveryID)
	}
}

func deliverWithRetry(job *WebhookJob, store *storage.Store, delivery *storage.WebhookDelivery) bool {
	backoffs := []time.Duration{1 * time.Second, 5 * time.Second, 30 * time.Second}
	maxAttempts := 3

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Perform HTTP POST
		statusCode, responseBody, err := performWebhookDelivery(job.URL, job.Secret, job.EventType, job.Payload)

		now := time.Now()
		delivery.Attempts++
		delivery.LastAttempt = &now
		delivery.ResponseStatus = statusCode
		delivery.ResponseBody = truncateString(responseBody, 1000)

		if err != nil {
			delivery.LastError = err.Error()
		} else if statusCode < 200 || statusCode >= 300 {
			delivery.LastError = "HTTP " + string(rune(statusCode)) + ": " + truncateString(responseBody, 200)
		} else {
			// Success
			delivery.Status = "success"
			delivery.DeliveredAt = &now
			store.UpdateWebhookDelivery(delivery)
			log.Printf("[webhook-worker] delivery %s succeeded (HTTP %d)", job.DeliveryID, statusCode)
			return true
		}

		store.UpdateWebhookDelivery(delivery)

		if attempt < maxAttempts {
			// Apply backoff before retry
			backoff := backoffs[attempt-1]
			if int(backoff) > len(backoffs) {
				backoff = backoffs[len(backoffs)-1]
			}
			log.Printf("[webhook-worker] delivery %s attempt %d failed, retrying in %v: %s",
				job.DeliveryID, attempt, backoff, delivery.LastError)
			time.Sleep(backoff)
		}
	}

	// Mark as failed after all retries exhausted
	delivery.Status = "failed"
	delivery.LastError = "max retries exceeded"
	store.UpdateWebhookDelivery(delivery)
	log.Printf("[webhook-worker] delivery %s failed permanently after %d attempts: %s",
		job.DeliveryID, delivery.Attempts, delivery.LastError)
	return false
}

func performWebhookDelivery(url, secret, eventType, payload string) (int, string, error) {
	// This function performs the actual HTTP POST to the webhook URL
	// (Implementation delegated to the existing worker package)
	return http.DefaultClient.Post(url, "application/json", nil)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ── Webhook Handlers ─────────────────────────────────────────────────────────

// ListWebhooks handles GET /webhooks?org_id=X
func ListWebhooks(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		orgID := c.Query("org_id")
		if orgID == "" {
			orgID = getOrgID(c)
		}

		subs, err := store.ListWebhookSubscriptions(orgID)
		if err != nil {
			internalError(c)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": subs})
	}
}

// CreateWebhook handles POST /webhooks
func CreateWebhook(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input struct {
			OrgID      string   `json:"org_id" binding:"required"`
			URL        string   `json:"url" binding:"required,url"`
			EventTypes []string `json:"event_types" binding:"required,min=1"`
			Secret     string   `json:"secret"`
		}

		if err := c.ShouldBindJSON(&input); err != nil {
			badRequest(c, err)
			return
		}

		sub := &storage.WebhookSubscription{
			ID:         uuid.New().String(),
			OrgID:      input.OrgID,
			URL:        input.URL,
			EventTypes: input.EventTypes,
			Secret:     input.Secret,
			Active:     true,
		}

		created, err := store.CreateWebhookSubscription(sub)
		if err != nil {
			internalError(c)
			return
		}

		c.JSON(http.StatusCreated, created)
	}
}

// GetWebhook handles GET /webhooks/:id
func GetWebhook(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		orgID := getOrgID(c)
		id := c.Param("id")

		sub, err := store.GetWebhookSubscription(id, orgID)
		if err != nil {
			internalError(c)
			return
		}
		if sub == nil {
			notFound(c, "webhook not found")
			return
		}

		c.JSON(http.StatusOK, sub)
	}
}

// UpdateWebhook handles PATCH /webhooks/:id
func UpdateWebhook(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input struct {
			URL        string   `json:"url"`
			EventTypes []string `json:"event_types"`
			Active     *bool    `json:"active"`
		}

		if err := c.ShouldBindJSON(&input); err != nil {
			badRequest(c, err)
			return
		}

		orgID := getOrgID(c)
		id := c.Param("id")

		sub, err := store.GetWebhookSubscription(id, orgID)
		if err != nil {
			internalError(c)
			return
		}
		if sub == nil {
			notFound(c, "webhook not found")
			return
		}

		url := sub.URL
		eventTypes := sub.EventTypes
		active := sub.Active

		if input.URL != "" {
			url = input.URL
		}
		if input.EventTypes != nil {
			eventTypes = input.EventTypes
		}
		if input.Active != nil {
			active = *input.Active
		}

		if err := store.UpdateWebhookSubscription(id, orgID, url, eventTypes, active); err != nil {
			internalError(c)
			return
		}

		updated, _ := store.GetWebhookSubscription(id, orgID)
		c.JSON(http.StatusOK, updated)
	}
}

// DeleteWebhook handles DELETE /webhooks/:id
func DeleteWebhook(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		orgID := getOrgID(c)
		id := c.Param("id")

		if err := store.DeleteWebhookSubscription(id, orgID); err != nil {
			internalError(c)
			return
		}

		c.Status(http.StatusNoContent)
	}
}

// ListWebhookDeliveries handles GET /webhooks/:id/deliveries?org_id=X
func ListWebhookDeliveries(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		orgID := c.Query("org_id")
		if orgID == "" {
			orgID = getOrgID(c)
		}

		webhookID := c.Param("id")
		size, offset := paginate(c)

		deliveries, err := store.ListWebhookDeliveries(webhookID, orgID, size, offset)
		if err != nil {
			internalError(c)
			return
		}

		c.JSON(http.StatusOK, gin.H{"data": deliveries})
	}
}

// RetryWebhookDelivery handles POST /webhooks/:id/retry/:deliveryId
func RetryWebhookDelivery(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		orgID := getOrgID(c)
		webhookID := c.Param("id")
		deliveryID := c.Param("deliveryId")

		delivery, err := store.GetWebhookDelivery(deliveryID, orgID)
		if err != nil {
			internalError(c)
			return
		}
		if delivery == nil {
			notFound(c, "delivery not found")
			return
		}

		// Reset delivery for retry
		delivery.Status = "pending"
		delivery.Attempts = 0
		delivery.LastError = ""
		delivery.NextRetry = nil

		if err := store.UpdateWebhookDelivery(delivery); err != nil {
			internalError(c)
			return
		}

		c.JSON(http.StatusOK, delivery)
	}
}

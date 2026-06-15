package engine

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ttcccat-tech/cont/admin-api/storage"
)

// Alerter periodically evaluates alert rules and fires notifications.
type Alerter struct {
	store       *storage.Store
	interval    time.Duration
	stopCh      chan struct{}
	wg          sync.WaitGroup
	metricsURL  string
	lastFiredAt map[int64]time.Time // ruleID -> last fired timestamp (for suppression)
	mu          sync.RWMutex
}

// NewAlerter creates a new Alerter that evaluates rules every interval.
func NewAlerter(store *storage.Store, interval time.Duration, metricsURL string) *Alerter {
	return &Alerter{
		store:       store,
		interval:    interval,
		stopCh:      make(chan struct{}),
		metricsURL:  metricsURL,
		lastFiredAt: make(map[int64]time.Time),
	}
}

// Start begins the periodic alert evaluation loop.
func (a *Alerter) Start() {
	a.wg.Add(1)
	go a.run()
	log.Printf("[alerter] started, evaluating every %v", a.interval)
}

// Stop gracefully stops the alerter.
func (a *Alerter) Stop() {
	close(a.stopCh)
	a.wg.Wait()
	log.Printf("[alerter] stopped")
}

func (a *Alerter) run() {
	defer a.wg.Done()
	// Run immediately on start, then on interval
	a.evaluate()
	ticker := time.NewTicker(a.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			a.evaluate()
		case <-a.stopCh:
			return
		}
	}
}

func (a *Alerter) evaluate() {
	rules, err := a.store.ListAlertRules()
	if err != nil {
		log.Printf("[alerter] failed to list rules: %v", err)
		return
	}

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		a.evaluateRule(&rule)
	}
}
func (a *Alerter) evaluateRule(rule *storage.AlertRule) {
	// Check suppression window
	a.mu.RLock()
	lastFired, ok := a.lastFiredAt[rule.ID]
	a.mu.RUnlock()
	if ok && rule.AlertSuppressSeconds > 0 {
		if time.Since(lastFired) < time.Duration(rule.AlertSuppressSeconds)*time.Second {
			return // suppressed
		}
	}

	var triggered bool
	var triggeredValue float64

	if len(rule.Conditions) > 0 {
		// Multi-condition mode: evaluate all conditions with AND/OR logic
		triggered = a.evaluateConditions(rule)
		if triggered {
			// Use the primary metric value for notification
			val, _ := a.fetchConditionMetric(rule.Conditions[0])
			triggeredValue = val
		}
	} else {
		// Legacy single-condition mode
		value, err := a.fetchMetric(rule)
		if err != nil {
			log.Printf("[alerter] rule %d (%s): failed to fetch metric: %v", rule.ID, rule.Name, err)
			return
		}
		// For usage_quota, use PercentageThreshold (default 80) instead of ThresholdValue
		threshold := rule.ThresholdValue
		if rule.MetricType == "usage_quota" {
			threshold = rule.PercentageThreshold
			if threshold == 0 {
				threshold = 80.0 // default
			}
		}
		triggered = a.checkCondition(value, threshold, rule.Operator)
		triggeredValue = value
	}

	if !triggered {
		return
	}

	// Fire notification
	a.fireAlert(rule, triggeredValue)
}

// evaluateConditions evaluates multiple conditions with AND/OR logic.
// The Logic field of each condition (except the last) determines how it combines
// with the next condition. Default logic is AND.
func (a *Alerter) evaluateConditions(rule *storage.AlertRule) bool {
	if len(rule.Conditions) == 0 {
		return false
	}

	// Evaluate first condition
	cond := rule.Conditions[0]
	firstValue, err := a.fetchConditionMetric(cond)
	if err != nil {
		log.Printf("[alerter] rule %d (%s): failed to fetch metric for condition: %v", rule.ID, rule.Name, err)
		return false
	}
	// For usage_quota, use PercentageThreshold (default 80)
	threshold := cond.ThresholdValue
	if cond.MetricType == "usage_quota" {
		threshold = cond.PercentageThreshold
		if threshold == 0 {
			threshold = 80.0
		}
	}
	result := a.checkCondition(firstValue, threshold, cond.Operator)

	// Evaluate remaining conditions with AND/OR logic
	for i := 1; i < len(rule.Conditions); i++ {
		cond := rule.Conditions[i]
		value, err := a.fetchConditionMetric(cond)
		if err != nil {
			log.Printf("[alerter] rule %d (%s): failed to fetch metric for condition %d: %v", rule.ID, rule.Name, i, err)
			return false
		}
		// For usage_quota, use PercentageThreshold (default 80)
		threshold = cond.ThresholdValue
		if cond.MetricType == "usage_quota" {
			threshold = cond.PercentageThreshold
			if threshold == 0 {
				threshold = 80.0
			}
		}
		condResult := a.checkCondition(value, threshold, cond.Operator)

		// Determine logic from previous condition
		prevLogic := "AND"
		if i-1 < len(rule.Conditions) {
			prevLogic = rule.Conditions[i-1].Logic
		}

		switch prevLogic {
		case "OR":
			result = result || condResult
		default: // AND
			result = result && condResult
		}
	}

	return result
}

// fetchConditionMetric fetches the metric value for a single condition.
func (a *Alerter) fetchConditionMetric(cond storage.Condition) (float64, error) {
	if a.metricsURL != "" {
		// Fetch from Prometheus endpoint
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(a.metricsURL)
		if err != nil {
			return 0, fmt.Errorf("metrics request failed: %w", err)
		}
		defer resp.Body.Close()
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(resp.Body); err != nil {
			return 0, fmt.Errorf("read body failed: %w", err)
		}
		return parseMetricFromPrometheusWithCond(buf.Bytes(), cond)
	}
	// Fallback: compute from proxy metrics
	return a.computeConditionMetric(cond)
}

// computeConditionMetric computes metric value for a condition from proxy metrics.
func (a *Alerter) computeConditionMetric(cond storage.Condition) (float64, error) {
	if cond.MetricType == "error_rate" || cond.MetricType == "latency" {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(a.metricsURL)
		if err != nil {
			return 0, fmt.Errorf("proxy metrics request failed: %w", err)
		}
		defer resp.Body.Close()
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(resp.Body); err != nil {
			return 0, fmt.Errorf("read proxy metrics failed: %w", err)
		}
		return parseMetricFromPrometheusWithCond(buf.Bytes(), cond)
	}
	if cond.MetricType == "usage_quota" {
		if cond.QuotaMetricType == "consumer" {
			return a.computeConsumerUsageQuotaMetric(cond.ServiceName)
		}
		return a.computeUsageQuotaMetric(cond.ServiceName)
	}
	return 0.0, fmt.Errorf("unknown metric type: %s", cond.MetricType)
}

// computeUsageQuotaMetric computes the usage percentage (0-100) for an org's quota.
// ServiceName is used as the org_id; if empty, defaults to zero-UUID admin org.
func (a *Alerter) computeUsageQuotaMetric(orgID string) (float64, error) {
	if orgID == "" {
		orgID = "00000000-0000-0000-0000-000000000000"
	}

	// Get monthly usage from Redis
	monthly, err := a.store.Redis().GetMonthlyUsage(context.Background(), orgID)
	if err != nil {
		return 0, fmt.Errorf("failed to get monthly usage: %w", err)
	}

	// Get plan quota
	var planLimit int64 = 100000
	if orgID != "00000000-0000-0000-0000-000000000000" {
		org, err := a.store.GetOrganization(orgID)
		if err == nil && org != nil {
			plan, err := a.store.GetPlanByName(org.Plan)
			if err == nil && plan != nil {
				planLimit = int64(plan.RequestLimit)
			}
		}
	}

	if planLimit <= 0 {
		return 0, nil
	}
	percent := float64(monthly) / float64(planLimit) * 100
	if percent > 100 {
		percent = 100
	}
	return percent, nil
}

// computeConsumerUsageQuotaMetric computes the usage percentage (0-100) for a consumer's quota.
// ServiceName is used as the consumer_id; looks up org_id from the consumer record.
func (a *Alerter) computeConsumerUsageQuotaMetric(consumerID string) (float64, error) {
	if consumerID == "" {
		return 0, fmt.Errorf("consumer ID is required")
	}

	// Look up consumer to get org_id
	consumer, err := a.store.GetConsumer(consumerID, "")
	if err != nil {
		return 0, fmt.Errorf("failed to get consumer %s: %w", consumerID, err)
	}
	if consumer == nil {
		return 0, fmt.Errorf("consumer %s not found", consumerID)
	}

	// Get monthly usage for this consumer from Redis
	monthly, err := a.store.Redis().GetConsumerMonthlyUsage(context.Background(), consumerID)
	if err != nil {
		return 0, fmt.Errorf("failed to get consumer monthly usage: %w", err)
	}

	// Get plan limit (consumer-specific or org-level)
	planLimit := int64(100000) // default
	org, err := a.store.GetOrganization(consumer.OrgID)
	if err == nil && org != nil {
		plan, err := a.store.GetPlanByName(org.Plan)
		if err == nil && plan != nil {
			planLimit = int64(plan.RequestLimit)
		}
	}

	if planLimit <= 0 {
		return 0, nil
	}
	percent := float64(monthly) / float64(planLimit) * 100
	if percent > 100 {
		percent = 100
	}
	return percent, nil
}

func (a *Alerter) fetchMetric(rule *storage.AlertRule) (float64, error) {
	// usage_quota is always computed from Redis, never from Prometheus
	if rule.MetricType == "usage_quota" {
		return a.computeUsageQuotaMetric(rule.OrgID)
	}
	if a.metricsURL == "" {
		// Fallback: compute from proxy metrics endpoint
		return a.computeFromProxyMetrics(rule)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(a.metricsURL)
	if err != nil {
		return 0, fmt.Errorf("metrics request failed: %w", err)
	}
	defer resp.Body.Close()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return 0, fmt.Errorf("read body failed: %w", err)
	}

	return parseMetricFromPrometheus(buf.Bytes(), rule)
}

func (a *Alerter) computeFromProxyMetrics(rule *storage.AlertRule) (float64, error) {
	// usage_quota is always computed from Redis
	if rule.MetricType == "usage_quota" {
		return a.computeUsageQuotaMetric(rule.OrgID)
	}
	client := &http.Client{Timeout: 5 * time.Second}

	// Try upstream health metrics
	if rule.MetricType == "error_rate" || rule.MetricType == "latency" {
		resp, err := client.Get(a.metricsURL)
		if err != nil {
			return 0, fmt.Errorf("proxy metrics request failed: %w", err)
		}
		defer resp.Body.Close()

		var buf bytes.Buffer
		if _, err := buf.ReadFrom(resp.Body); err != nil {
			return 0, fmt.Errorf("read proxy metrics failed: %w", err)
		}

		return parseMetricFromPrometheus(buf.Bytes(), rule)
	}

	return 0, fmt.Errorf("unknown metric type: %s", rule.MetricType)
}

// parseMetricFromPrometheus extracts a metric value from Prometheus text format.
// For upstream_target_up: returns 1.0 if target is up, 0.0 if down.
// For latency/error_rate: returns the gauge or counter value.
func parseMetricFromPrometheus(body []byte, rule *storage.AlertRule) (float64, error) {
	metricName := rule.MetricType
	if rule.MetricType == "error_rate" {
		metricName = "cont_upstream_target_up"
	} else if rule.MetricType == "latency" {
		metricName = "cont_nginx_requests_total" // fallback, actual impl would need histogram
	}

	serviceLabel := rule.ServiceName

	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		// Skip comments and blank lines
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		// Look for the metric line
		if !strings.HasPrefix(line, metricName) {
			continue
		}
		// If serviceName is specified, must match
		if serviceLabel != "" {
			if !strings.Contains(line, `service_name="`+serviceLabel+`"`) {
				continue
			}
		}
		// Parse value from end of line (format: metric{labels} VALUE)
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		valStr := strings.TrimSpace(parts[1])
		var val float64
		if _, err := fmt.Sscanf(valStr, "%f", &val); err == nil {
			return val, nil
		}
	}

	// Default value when metric not found (assume healthy/zero error)
	return 0.0, nil
}

// parseMetricFromPrometheusWithCond extracts a metric value for a specific condition.
func parseMetricFromPrometheusWithCond(body []byte, cond storage.Condition) (float64, error) {
	metricName := cond.MetricType
	if cond.MetricType == "error_rate" {
		metricName = "cont_upstream_target_up"
	} else if cond.MetricType == "latency" {
		metricName = "cont_nginx_requests_total"
	}

	serviceLabel := cond.ServiceName

	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, metricName) {
			continue
		}
		if serviceLabel != "" {
			if !strings.Contains(line, `service_name="`+serviceLabel+`"`) {
				continue
			}
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		valStr := strings.TrimSpace(parts[1])
		var val float64
		if _, err := fmt.Sscanf(valStr, "%f", &val); err == nil {
			return val, nil
		}
	}
	return 0.0, nil
}

func (a *Alerter) checkCondition(value, threshold float64, operator string) bool {
	switch operator {
	case ">":
		return value > threshold
	case "<":
		return value < threshold
	case ">=":
		return value >= threshold
	case "<=":
		return value <= threshold
	case "==":
		return value == threshold
	default:
		log.Printf("[alerter] unknown operator %s, treating as false", operator)
		return false
	}
}

func (a *Alerter) fireAlert(rule *storage.AlertRule, currentValue float64) {
	// Generate a trace ID for this alert firing event
	b := make([]byte, 16)
	rand.Read(b)
	traceID := hex.EncodeToString(b)

	log.Printf("[alerter] rule %d (%s) triggered: %s %s %f (current: %f) [trace=%s]",
		rule.ID, rule.Name, rule.MetricType, rule.Operator, rule.ThresholdValue, currentValue, traceID)

	var wg sync.WaitGroup
	if rule.SlackWebhookURL != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.sendSlack(rule, currentValue)
		}()
	}
	if rule.DiscordWebhookURL != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.sendDiscord(rule, currentValue)
		}()
	}
	if rule.EmailWebhookURL != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.sendEmail(rule, currentValue)
		}()
	}

	wg.Wait()

	// Broadcast SSE event to all connected admin clients
	triggeredAt := time.Now().UTC().Format(time.RFC3339)
	storage.Hub.BroadcastAll("alert_triggered", map[string]interface{}{
		"rule_id":        rule.ID,
		"rule_name":      rule.Name,
		"metric_type":    rule.MetricType,
		"operator":       rule.Operator,
		"threshold":      rule.ThresholdValue,
		"current_value":  currentValue,
		"service_name":   rule.ServiceName,
		"triggered_at":   triggeredAt,
	})

	// Trigger webhook subscriptions for alert.triggered events
	// Use default org since alert_rules table has no org_id column (global alerts)
	TriggerWebhook(a.store, "00000000-0000-0000-0000-000000000000", "alert.triggered", map[string]interface{}{
		"rule_id":       rule.ID,
		"rule_name":     rule.Name,
		"metric_type":   rule.MetricType,
		"operator":      rule.Operator,
		"threshold":     rule.ThresholdValue,
		"current_value": currentValue,
		"service_name":  rule.ServiceName,
		"triggered_at":  triggeredAt,
		"trace_id":      traceID,
	})

	// Persist last triggered timestamp and value to DB
	if err := a.store.UpdateAlertRuleTriggered(rule.ID, triggeredAt, currentValue); err != nil {
		log.Printf("[alerter] failed to update triggered state for rule %d: %v", rule.ID, err)
	}

	// Persist alert history record
	history := &storage.AlertHistory{
		RuleID:      rule.ID,
		RuleName:    rule.Name,
		OrgID:       nil, // alerter runs globally, org_id is set per-rule if needed
		MetricType:  rule.MetricType,
		Operator:    rule.Operator,
		Threshold:   rule.ThresholdValue,
		ActualValue: currentValue,
		Message:     fmt.Sprintf("Alert rule '%s' triggered: %s %s %f (actual: %f)", rule.Name, rule.MetricType, rule.Operator, rule.ThresholdValue, currentValue),
		TraceID:     traceID,
	}
	if err := a.store.CreateAlertHistory(history); err != nil {
		log.Printf("[alerter] failed to create alert history for rule %d: %v", rule.ID, err)
	}

	// Update last fired timestamp
	a.mu.Lock()
	a.lastFiredAt[rule.ID] = time.Now()
	a.mu.Unlock()
}

// ── Slack notification ────────────────────────────────────────────────────────

func (a *Alerter) sendSlack(rule *storage.AlertRule, value float64) {
	payload := map[string]interface{}{
		"text": fmt.Sprintf("🚨 *Cont Alert: %s*", rule.Name),
		"blocks": []map[string]interface{}{
			{
				"type": "section",
				"text": map[string]interface{}{
					"type": "mrkdwn",
					"text": fmt.Sprintf("*Alert:* %s\n*Condition:* %s %s %s\n*Current value:* %.4f\n*Duration:* %ds",
						rule.Name,
						rule.MetricType,
						rule.Operator,
						fmt.Sprintf("%.4f", rule.ThresholdValue),
						value,
						rule.DurationSeconds,
					),
				},
			},
		},
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", rule.SlackWebhookURL, bytes.NewReader(body))
	if err != nil {
		log.Printf("[alerter] slack: failed to create request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[alerter] slack: failed to send: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		log.Printf("[alerter] slack: received status %d", resp.StatusCode)
	} else {
		log.Printf("[alerter] slack: alert sent for rule %d", rule.ID)
	}
}

// ── Discord notification ─────────────────────────────────────────────────────

func (a *Alerter) sendDiscord(rule *storage.AlertRule, value float64) {
	embed := map[string]interface{}{
		"title":       fmt.Sprintf("🚨 %s", rule.Name),
		"description": fmt.Sprintf("**Condition:** %s %s %.4f\n**Current value:** %.4f\n**Duration:** %ds",
			rule.MetricType, rule.Operator, rule.ThresholdValue, value, rule.DurationSeconds),
		"color": 15158332, // red
		"footer": map[string]string{"text": "Cont Alert Engine"},
		"timestamp": time.Now().Format(time.RFC3339),
	}
	if rule.Description != "" {
		embed["fields"] = []map[string]string{
			{"name": "Description", "value": rule.Description},
		}
	}
	payload := map[string]interface{}{"embeds": []map[string]interface{}{embed}}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", rule.DiscordWebhookURL, bytes.NewReader(body))
	if err != nil {
		log.Printf("[alerter] discord: failed to create request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[alerter] discord: failed to send: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		log.Printf("[alerter] discord: received status %d", resp.StatusCode)
	} else {
		log.Printf("[alerter] discord: alert sent for rule %d", rule.ID)
	}
}

// ── Email notification (generic webhook) ─────────────────────────────────────

func (a *Alerter) sendEmail(rule *storage.AlertRule, value float64) {
	subject := fmt.Sprintf("Cont Alert: %s", rule.Name)
	body := fmt.Sprintf("Alert: %s\n\nCondition: %s %s %s\nCurrent value: %.4f\nDuration: %ds\n\nDescription: %s",
		rule.Name,
		rule.MetricType,
		rule.Operator,
		fmt.Sprintf("%.4f", rule.ThresholdValue),
		value,
		rule.DurationSeconds,
		rule.Description,
	)
	payload := map[string]string{
		"subject": subject,
		"body":    body,
		"from":    "cont-alerter@cont.internal",
		"to":      rule.EmailWebhookURL, // URL could be mailto: or a webhook endpoint
	}
	jsonPayload, _ := json.Marshal(payload)

	// If it's a mailto: URL, just log (can't actually send email from server)
	if strings.HasPrefix(rule.EmailWebhookURL, "mailto:") {
		log.Printf("[alerter] email: would send to %s: %s", rule.EmailWebhookURL, body)
		return
	}

	// Treat as HTTP webhook
	req, err := http.NewRequest("POST", rule.EmailWebhookURL, bytes.NewReader(jsonPayload))
	if err != nil {
		log.Printf("[alerter] email: failed to create request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[alerter] email: failed to send: %v", err)
		return
	}
	defer resp.Body.Close()
	log.Printf("[alerter] email: alert sent for rule %d (status %d)", rule.ID, resp.StatusCode)
}

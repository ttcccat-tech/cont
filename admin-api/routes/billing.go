package routes

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/billingportal/session"
	stripecheckout "github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/stripe/stripe-go/v76/customer"
	"github.com/stripe/stripe-go/v76/subscription"
	stripewebhook "github.com/stripe/stripe-go/v76/webhook"
	"github.com/ttcccat-tech/cont/admin-api/storage"
)

var stripeEnabled = os.Getenv("STRIPE_SECRET_KEY") != ""

// getPlanPriceID returns the Stripe Price ID for a given plan name and billing cycle
func getPlanPriceID(planName, billingCycle string) string {
	switch planName {
	case "pro":
		if billingCycle == "yearly" {
			return os.Getenv("STRIPE_PRICE_PRO_YEARLY")
		}
		return os.Getenv("STRIPE_PRICE_PRO_MONTHLY")
	case "enterprise":
		if billingCycle == "yearly" {
			return os.Getenv("STRIPE_PRICE_ENTERPRISE_YEARLY")
		}
		return os.Getenv("STRIPE_PRICE_ENTERPRISE_MONTHLY")
	}
	return ""
}

// ── GET /billing/plans ────────────────────────────────────────────────────

func ListPlans(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		plans, err := store.ListPlans()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if plans == nil {
			plans = []storage.Plan{}
		}
		c.JSON(200, plans)
	}
}

// ── GET /billing/subscription ───────────────────────────────────────────────

func GetSubscription(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		orgID := c.GetString("org_id")
		if orgID == "" {
			// No org_id from context — look up via user
			userID, _ := c.Get("sub")
			if uid, ok := userID.(string); ok {
				org, err := store.GetOrganizationByUserID(uid)
				if err == nil && org != nil {
					orgID = org.ID
				}
			}
			if orgID == "" {
				c.JSON(400, gin.H{"error": "organization ID required"})
				return
			}
		}

		subscr, err := store.GetSubscriptionByOrg(orgID)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if subscr == nil {
			if err := store.EnsureFreeSubscription(orgID); err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			subscr, _ = store.GetSubscriptionByOrg(orgID)
		}
		c.JSON(200, subscr)
	}
}

// ── POST /billing/checkout ─────────────────────────────────────────────────

func CreateCheckoutSession(store *storage.Store, frontendBaseURL string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !stripeEnabled {
			c.JSON(400, gin.H{"error": "Stripe is not configured. Set STRIPE_SECRET_KEY environment variable."})
			return
		}

		orgID := c.GetString("org_id")
		if orgID == "" {
			c.JSON(400, gin.H{"error": "organization ID required"})
			return
		}

		var req struct {
			PlanName     string `json:"plan_name" binding:"required,oneof=pro enterprise"`
			BillingCycle string `json:"billing_cycle" binding:"required,oneof=monthly yearly"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		org, err := store.GetOrganization(orgID)
		if err != nil || org == nil {
			c.JSON(404, gin.H{"error": "organization not found"})
			return
		}

		priceID := getPlanPriceID(req.PlanName, req.BillingCycle)
		if priceID == "" {
			c.JSON(400, gin.H{"error": "Stripe price not configured. Set STRIPE_PRICE_PRO_MONTHLY, STRIPE_PRICE_PRO_YEARLY, etc."})
			return
		}

		baseURL := frontendBaseURL
		if baseURL == "" {
			baseURL = "http://localhost:5173"
		}

		// Get or create Stripe customer
		var customerID string
		existingSub, _ := store.GetSubscriptionByOrg(orgID)
		if existingSub != nil && existingSub.StripeCustomerID != "" {
			customerID = existingSub.StripeCustomerID
		} else {
			params := &stripe.CustomerParams{
				Email: stripe.String(org.Name + "@cont.internal"),
				Metadata: map[string]string{
					"org_id":   orgID,
					"org_name": org.Name,
				},
			}
			cust, err := customer.New(params)
			if err != nil {
				c.JSON(500, gin.H{"error": fmt.Sprintf("failed to create Stripe customer: %v", err)})
				return
			}
			customerID = cust.ID
		}

		successURL := fmt.Sprintf("%s/settings?billing=success&session_id={CHECKOUT_SESSION_ID}", baseURL)
		cancelURL := fmt.Sprintf("%s/settings?billing=cancelled", baseURL)

		params := &stripe.CheckoutSessionParams{
			Customer: stripe.String(customerID),
			Mode:     stripe.String(string(stripe.CheckoutSessionModeSubscription)),
			LineItems: []*stripe.CheckoutSessionLineItemParams{
				{
					Price:    stripe.String(priceID),
					Quantity: stripe.Int64(1),
				},
			},
			SuccessURL: stripe.String(successURL),
			CancelURL:  stripe.String(cancelURL),
			Metadata: map[string]string{
				"org_id":    orgID,
				"plan_name": req.PlanName,
				"price_id":  priceID,
			},
			AllowPromotionCodes: stripe.Bool(true),
			SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
				Metadata: map[string]string{
					"org_id":    orgID,
					"plan_name": req.PlanName,
				},
			},
		}

		sess, err := stripecheckout.New(params)
		if err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("failed to create checkout session: %v", err)})
			return
		}

		c.JSON(200, gin.H{"url": sess.URL})
	}
}

// ── POST /billing/portal ────────────────────────────────────────────────────

func CreatePortalSession(store *storage.Store, frontendBaseURL string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !stripeEnabled {
			c.JSON(400, gin.H{"error": "Stripe is not configured"})
			return
		}

		orgID := getOrgID(c)
		if orgID == "" {
			c.JSON(400, gin.H{"error": "organization ID required"})
			return
		}

		subscr, err := store.GetSubscriptionByOrg(orgID)
		if err != nil || subscr == nil || subscr.StripeCustomerID == "" {
			c.JSON(404, gin.H{"error": "no billing account found. Subscribe to a paid plan first."})
			return
		}

		baseURL := frontendBaseURL
		if baseURL == "" {
			baseURL = "http://localhost:5173"
		}

		params := &stripe.BillingPortalSessionParams{
			Customer:  stripe.String(subscr.StripeCustomerID),
			ReturnURL: stripe.String(baseURL + "/settings"),
		}
		sess, err := session.New(params)
		if err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("failed to create portal session: %v", err)})
			return
		}

		c.JSON(200, gin.H{"url": sess.URL})
	}
}

// ── POST /webhooks/stripe ──────────────────────────────────────────────────

func HandleStripeWebhook(store *storage.Store, webhookSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !stripeEnabled {
			c.JSON(400, gin.H{"error": "Stripe is not configured"})
			return
		}

		payload, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(400, gin.H{"error": "failed to read body"})
			return
		}

		sigHeader := c.GetHeader("Stripe-Signature")
		var event stripe.Event
		if webhookSecret != "" {
			event, err = stripewebhook.ConstructEvent(payload, sigHeader, webhookSecret)
			if err != nil {
				c.JSON(400, gin.H{"error": fmt.Sprintf("webhook signature verification failed: %v", err)})
				return
			}
		} else {
			if err := json.Unmarshal(payload, &event); err != nil {
				c.JSON(400, gin.H{"error": "invalid JSON"})
				return
			}
		}

		switch event.Type {
		case "checkout.session.completed":
			var sess stripe.CheckoutSession
			if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
				log.Printf("Webhook: failed to parse checkout session: %v", err)
				break
			}
			orgID := ""
			planName := "pro"
			if sess.Metadata != nil {
				orgID = sess.Metadata["org_id"]
				if p, ok := sess.Metadata["plan_name"]; ok && p != "" {
					planName = p
				}
			}
			if orgID == "" {
				break
			}

			var subscriptionID string
			if sess.Subscription != nil {
				subscriptionID = sess.Subscription.ID
			}
			var custID string
			if sess.Customer != nil {
				custID = sess.Customer.ID
			}

			if subscriptionID != "" {
				subscr, err := subscription.Get(subscriptionID, nil)
				if err == nil && subscr != nil {
					priceID := ""
					billingCycle := "monthly"
					if len(subscr.Items.Data) > 0 {
						priceID = subscr.Items.Data[0].Price.ID
						if subscr.Items.Data[0].Price.Recurring != nil &&
							subscr.Items.Data[0].Price.Recurring.Interval == "year" {
							billingCycle = "yearly"
						}
					}
					currentPeriodStart := ""
					currentPeriodEnd := ""
					if subscr.CurrentPeriodStart != 0 {
						currentPeriodStart = fmt.Sprintf("%d", subscr.CurrentPeriodStart)
					}
					if subscr.CurrentPeriodEnd != 0 {
						currentPeriodEnd = fmt.Sprintf("%d", subscr.CurrentPeriodEnd)
					}

					updatedSub := &storage.Subscription{
						ID:                   subscriptionID,
						OrgID:                orgID,
						PlanName:             planName,
						StripeCustomerID:     custID,
						StripeSubscriptionID: subscriptionID,
						StripePriceID:        priceID,
						Status:               string(subscr.Status),
						BillingCycle:         billingCycle,
						CurrentPeriodStart:   currentPeriodStart,
						CurrentPeriodEnd:     currentPeriodEnd,
						CancelAtPeriodEnd:    subscr.CancelAtPeriodEnd,
					}
					if err := store.UpsertSubscription(updatedSub); err != nil {
						log.Printf("Webhook: failed to upsert subscription: %v", err)
					}
					store.UpdateOrganizationPlan(orgID, planName)
					log.Printf("Webhook: subscription activated for org %s, plan %s", orgID, planName)
				}
			}

		case "customer.subscription.updated":
			var subscr stripe.Subscription
			if err := json.Unmarshal(event.Data.Raw, &subscr); err != nil {
				log.Printf("Webhook: failed to parse subscription update: %v", err)
				break
			}
			custID := ""
			if subscr.Customer != nil {
				custID = subscr.Customer.ID
			}
			subs, _ := store.ListSubscriptions()
			var orgID string
			for _, s := range subs {
				if s.StripeCustomerID == custID {
					orgID = s.OrgID
					break
				}
			}
			if orgID == "" {
				break
			}

			priceID := ""
			billingCycle := "monthly"
			if len(subscr.Items.Data) > 0 {
				priceID = subscr.Items.Data[0].Price.ID
				if subscr.Items.Data[0].Price.Recurring != nil &&
					subscr.Items.Data[0].Price.Recurring.Interval == "year" {
					billingCycle = "yearly"
				}
			}

			planName := "pro"
			if strings.Contains(priceID, "enterprise") {
				planName = "enterprise"
			}

			currentPeriodStart := ""
			currentPeriodEnd := ""
			if subscr.CurrentPeriodStart != 0 {
				currentPeriodStart = fmt.Sprintf("%d", subscr.CurrentPeriodStart)
			}
			if subscr.CurrentPeriodEnd != 0 {
				currentPeriodEnd = fmt.Sprintf("%d", subscr.CurrentPeriodEnd)
			}

			updatedSub := &storage.Subscription{
				ID:                   subscr.ID,
				OrgID:                orgID,
				PlanName:             planName,
				StripeCustomerID:     custID,
				StripeSubscriptionID: subscr.ID,
				StripePriceID:        priceID,
				Status:               string(subscr.Status),
				BillingCycle:         billingCycle,
				CurrentPeriodStart:   currentPeriodStart,
				CurrentPeriodEnd:     currentPeriodEnd,
				CancelAtPeriodEnd:    subscr.CancelAtPeriodEnd,
			}
			store.UpsertSubscription(updatedSub)
			store.UpdateOrganizationPlan(orgID, planName)
			log.Printf("Webhook: subscription updated for org %s, status %s", orgID, subscr.Status)

		case "customer.subscription.deleted":
			var subscr stripe.Subscription
			if err := json.Unmarshal(event.Data.Raw, &subscr); err != nil {
				break
			}
			custID := ""
			if subscr.Customer != nil {
				custID = subscr.Customer.ID
			}
			subs, _ := store.ListSubscriptions()
			for _, s := range subs {
				if s.StripeCustomerID == custID {
					store.DeleteSubscription(s.ID)
					store.UpdateOrganizationPlan(s.OrgID, "free")
					store.EnsureFreeSubscription(s.OrgID)
					log.Printf("Webhook: subscription canceled for org %s, reverted to free", s.OrgID)
					break
				}
			}

		case "invoice.payment_failed":
			var inv stripe.Invoice
			if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
				break
			}
			log.Printf("Webhook: invoice payment failed for customer %s", inv.Customer.ID)

		default:
			log.Printf("Webhook: unhandled event type %s", event.Type)
		}

		c.JSON(200, gin.H{"received": true})
	}
}

// ListSubscriptions returns all subscriptions (admin only)
func ListSubscriptions(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		subs, err := store.ListSubscriptions()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if subs == nil {
			subs = []storage.Subscription{}
		}
		c.JSON(200, subs)
	}
}

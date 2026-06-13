package storage

import (
	"database/sql"
	"time"
)

// ── Plans ──────────────────────────────────────────────────────────────────

type Plan struct {
	ID             string `json:"id"` // stripe price id (e.g., price_xxx)
	Name           string `json:"name"` // free | pro | enterprise
	DisplayName    string `json:"display_name"`
	PriceMonthly   int64  `json:"price_monthly"` // in cents, 0 for free
	PriceYearly    int64  `json:"price_yearly"`  // in cents, 0 for free
	Features       string `json:"features"` // JSON array as string
	WorkspaceLimit int    `json:"workspace_limit"`
	UserLimit      int    `json:"user_limit"`
	RequestLimit   int64  `json:"request_limit"` // per month, -1 = unlimited
	CreatedAt      string `json:"created_at,omitempty"`
}

// ── Subscriptions ──────────────────────────────────────────────────────────

type Subscription struct {
	ID                       string         `json:"id"` // stripe subscription id
	OrgID                    string         `json:"org_id"`
	PlanName                 string         `json:"plan_name"` // free | pro | enterprise
	StripeCustomerID         string         `json:"stripe_customer_id"`
	StripeSubscriptionID     string         `json:"stripe_subscription_id"`
	StripePriceID            string         `json:"stripe_price_id"`
	Status                   string         `json:"status"` // active | canceled | past_due | trialing | incomplete
	BillingCycle             string         `json:"billing_cycle"` // monthly | yearly
	CurrentPeriodStart       string         `json:"current_period_start"`
	CurrentPeriodEnd         string         `json:"current_period_end"`
	CancelAtPeriodEnd        bool           `json:"cancel_at_period_end"`
	TrialEnd                 *string        `json:"trial_end,omitempty"`
	CreatedAt                string         `json:"created_at,omitempty"`
	UpdatedAt                string         `json:"updated_at,omitempty"`
}

// ── Plan store methods ─────────────────────────────────────────────────────

var DefaultPlans = []Plan{
	{
		ID:             "price_free",
		Name:           "free",
		DisplayName:    "Free",
		PriceMonthly:   0,
		PriceYearly:    0,
		Features:       `["Up to 3 workspaces","Up to 5 users","1,000 API requests/month","Community support"]`,
		WorkspaceLimit: 3,
		UserLimit:      5,
		RequestLimit:   1000,
	},
	{
		ID:             "price_pro_monthly",
		Name:           "pro",
		DisplayName:    "Pro",
		PriceMonthly:   2900,
		PriceYearly:    29000,
		Features:       `["Unlimited workspaces","Up to 25 users","100,000 API requests/month","Email support","RBAC & SSO","Audit logs"]`,
		WorkspaceLimit: -1,
		UserLimit:      25,
		RequestLimit:   100000,
	},
	{
		ID:             "price_enterprise_monthly",
		Name:           "enterprise",
		DisplayName:    "Enterprise",
		PriceMonthly:   9900,
		PriceYearly:    99000,
		Features:       `["Unlimited workspaces","Unlimited users","Unlimited API requests","Priority support","SLA guarantee","Custom integrations"]`,
		WorkspaceLimit: -1,
		UserLimit:      -1,
		RequestLimit:   -1,
	},
}

// InitDefaultPlans inserts default plan records if none exist
func (s *Store) InitDefaultPlans() error {
	for _, p := range DefaultPlans {
		_, err := s.db.Exec(`
			INSERT INTO plans (id, name, display_name, price_monthly, price_yearly, features, workspace_limit, user_limit, request_limit)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (id) DO NOTHING`,
			p.ID, p.Name, p.DisplayName, p.PriceMonthly, p.PriceYearly, p.Features, p.WorkspaceLimit, p.UserLimit, p.RequestLimit)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListPlans() ([]Plan, error) {
	rows, err := s.db.Query(`
		SELECT id, name, display_name, price_monthly, price_yearly, features, workspace_limit, user_limit, request_limit, created_at
		FROM plans ORDER BY price_monthly ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Plan
	for rows.Next() {
		var p Plan
		if err := rows.Scan(&p.ID, &p.Name, &p.DisplayName, &p.PriceMonthly, &p.PriceYearly,
			&p.Features, &p.WorkspaceLimit, &p.UserLimit, &p.RequestLimit, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetPlanByName returns a plan by its name (free/pro/enterprise).
func (s *Store) GetPlanByName(name string) (*Plan, error) {
	row := s.db.QueryRow(`
		SELECT id, name, display_name, price_monthly, price_yearly, features, workspace_limit, user_limit, request_limit, created_at
		FROM plans WHERE name=$1`, name)
	var p Plan
	err := row.Scan(&p.ID, &p.Name, &p.DisplayName, &p.PriceMonthly, &p.PriceYearly,
		&p.Features, &p.WorkspaceLimit, &p.UserLimit, &p.RequestLimit, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ── Subscription store methods ─────────────────────────────────────────────

func (s *Store) GetSubscriptionByOrg(orgID string) (*Subscription, error) {
	row := s.db.QueryRow(`
		SELECT id, org_id, plan_name, stripe_customer_id, stripe_subscription_id, stripe_price_id,
		       status, billing_cycle, current_period_start, current_period_end, cancel_at_period_end, trial_end, created_at, updated_at
		FROM subscriptions WHERE org_id=$1`, orgID)
	var sub Subscription
	var trialEnd sql.NullString
	err := row.Scan(&sub.ID, &sub.OrgID, &sub.PlanName, &sub.StripeCustomerID, &sub.StripeSubscriptionID,
		&sub.StripePriceID, &sub.Status, &sub.BillingCycle, &sub.CurrentPeriodStart, &sub.CurrentPeriodEnd,
		&sub.CancelAtPeriodEnd, &trialEnd, &sub.CreatedAt, &sub.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if trialEnd.Valid {
		sub.TrialEnd = &trialEnd.String
	}
	return &sub, nil
}

func (s *Store) UpsertSubscription(sub *Subscription) error {
	// Ensure org_id is not empty
	if sub.OrgID == "" {
		sub.OrgID = "00000000-0000-0000-0000-000000000000"
	}
	err := s.db.QueryRow(`
		INSERT INTO subscriptions (id, org_id, plan_name, stripe_customer_id, stripe_subscription_id, stripe_price_id,
			status, billing_cycle, current_period_start, current_period_end, cancel_at_period_end, trial_end, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NOW(),NOW())
		ON CONFLICT (id) DO UPDATE SET
			plan_name=EXCLUDED.plan_name,
			stripe_customer_id=EXCLUDED.stripe_customer_id,
			stripe_subscription_id=EXCLUDED.stripe_subscription_id,
			stripe_price_id=EXCLUDED.stripe_price_id,
			status=EXCLUDED.status,
			billing_cycle=EXCLUDED.billing_cycle,
			current_period_start=EXCLUDED.current_period_start,
			current_period_end=EXCLUDED.current_period_end,
			cancel_at_period_end=EXCLUDED.cancel_at_period_end,
			trial_end=EXCLUDED.trial_end,
			updated_at=NOW()
		RETURNING id, created_at, updated_at`,
		sub.ID, sub.OrgID, sub.PlanName, sub.StripeCustomerID, sub.StripeSubscriptionID, sub.StripePriceID,
		sub.Status, sub.BillingCycle, sub.CurrentPeriodStart, sub.CurrentPeriodEnd, sub.CancelAtPeriodEnd, sub.TrialEnd,
	).Scan(&sub.ID, &sub.CreatedAt, &sub.UpdatedAt)
	return err
}

func (s *Store) DeleteSubscription(id string) error {
	_, err := s.db.Exec(`DELETE FROM subscriptions WHERE id=$1`, id)
	return err
}

func (s *Store) ListSubscriptions() ([]Subscription, error) {
	rows, err := s.db.Query(`
		SELECT id, org_id, plan_name, stripe_customer_id, stripe_subscription_id, stripe_price_id,
		       status, billing_cycle, current_period_start, current_period_end, cancel_at_period_end, trial_end, created_at, updated_at
		FROM subscriptions ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Subscription
	for rows.Next() {
		var sub Subscription
		var trialEnd sql.NullString
		if err := rows.Scan(&sub.ID, &sub.OrgID, &sub.PlanName, &sub.StripeCustomerID, &sub.StripeSubscriptionID,
			&sub.StripePriceID, &sub.Status, &sub.BillingCycle, &sub.CurrentPeriodStart, &sub.CurrentPeriodEnd,
			&sub.CancelAtPeriodEnd, &trialEnd, &sub.CreatedAt, &sub.UpdatedAt); err != nil {
			return nil, err
		}
		if trialEnd.Valid {
			sub.TrialEnd = &trialEnd.String
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}

func (s *Store) UpdateOrganizationPlan(id, plan string) error {
	_, err := s.db.Exec(`UPDATE organizations SET plan=$1, updated_at=NOW() WHERE id=$2`, plan, id)
	return err
}

// EnsureFreeSubscription creates a free-tier subscription for an org if none exists
func (s *Store) EnsureFreeSubscription(orgID string) error {
	existing, err := s.GetSubscriptionByOrg(orgID)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil // already has subscription
	}
	freePlan := DefaultPlans[0] // "free"
	periodStart := time.Now().UTC().Format(time.RFC3339)
	periodEnd := time.Now().UTC().AddDate(0, 1, 0).Format(time.RFC3339)
	sub := &Subscription{
		ID:                   "free_" + orgID,
		OrgID:                orgID,
		PlanName:             freePlan.Name,
		StripeCustomerID:     "",
		StripeSubscriptionID: "",
		StripePriceID:        "",
		Status:               "active",
		BillingCycle:         "monthly",
		CurrentPeriodStart:   periodStart,
		CurrentPeriodEnd:     periodEnd,
		CancelAtPeriodEnd:    false,
		TrialEnd:             nil,
	}
	return s.UpsertSubscription(sub)
}
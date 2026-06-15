package migrator

import (
	"database/sql"
	"log"
)

func RegisterAllMigrations(m *Manager) {
	// v001: Initial schema (services, routes, upstreams, targets, consumers, plugins)
	m.Register(Migration{
		Version:     1,
		Description: "Initial schema: core entities",
		Up: `
CREATE TABLE IF NOT EXISTS services (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT UNIQUE,
    protocol TEXT DEFAULT 'http',
    host TEXT NOT NULL,
    port INTEGER DEFAULT 80,
    path TEXT DEFAULT '/',
    url TEXT,
    retries INTEGER DEFAULT 5,
    connect_timeout INTEGER DEFAULT 60000,
    read_timeout INTEGER DEFAULT 60000,
    write_timeout INTEGER DEFAULT 60000,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS routes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT UNIQUE,
    service_id UUID REFERENCES services(id) ON DELETE CASCADE,
    protocols TEXT[] DEFAULT '{http,https}',
    hosts TEXT[],
    paths TEXT[],
    methods TEXT[],
    strip_path BOOLEAN DEFAULT true,
    preserve_host BOOLEAN DEFAULT false,
    regex_priority INTEGER DEFAULT 0,
    https_redirect_status_code INTEGER DEFAULT 426,
    connection_timeout INTEGER DEFAULT 60000,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS upstreams (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT UNIQUE NOT NULL,
    algorithm TEXT DEFAULT 'roundrobin',
    slots INTEGER DEFAULT 10000,
    healthchecks TEXT,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS targets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    upstream_id UUID REFERENCES upstreams(id) ON DELETE CASCADE,
    target TEXT NOT NULL,
    weight INTEGER DEFAULT 100,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS consumers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username TEXT UNIQUE,
    custom_id TEXT,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS plugins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    route_id UUID REFERENCES routes(id) ON DELETE CASCADE,
    service_id UUID REFERENCES services(id) ON DELETE CASCADE,
    consumer_id UUID REFERENCES consumers(id) ON DELETE CASCADE,
    config JSONB DEFAULT '{}',
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(route_id, service_id, consumer_id, name)
);
CREATE INDEX IF NOT EXISTS idx_routes_service ON routes(service_id);
CREATE INDEX IF NOT EXISTS idx_plugins_route ON plugins(route_id);
CREATE INDEX IF NOT EXISTS idx_plugins_service ON plugins(service_id);
CREATE INDEX IF NOT EXISTS idx_plugins_consumer ON plugins(consumer_id);
CREATE INDEX IF NOT EXISTS idx_targets_upstream ON targets(upstream_id);
`,
		Down: `
DROP TABLE IF EXISTS plugins;
DROP TABLE IF EXISTS targets;
DROP TABLE IF EXISTS consumers;
DROP TABLE IF EXISTS upstreams;
DROP TABLE IF EXISTS routes;
DROP TABLE IF EXISTS services;
`,
	})

	// v002: Workspaces
	m.Register(Migration{
		Version:     2,
		Description: "Workspaces table",
		Up: `
CREATE TABLE IF NOT EXISTS workspaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT UNIQUE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
`,
		Down: `DROP TABLE IF EXISTS workspaces;`,
	})

	// v003: Users + Auth Groups
	m.Register(Migration{
		Version:     3,
		Description: "Users and Auth Groups",
		Up: `
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    display_name TEXT,
    email TEXT UNIQUE,
    role TEXT DEFAULT 'user',
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE TABLE IF NOT EXISTS auth_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT UNIQUE NOT NULL,
    label TEXT,
    description TEXT,
    permissions JSONB DEFAULT '[]',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS user_auth_groups (
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    group_id UUID REFERENCES auth_groups(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (user_id, group_id)
);
CREATE INDEX IF NOT EXISTS idx_user_auth_groups_user ON user_auth_groups(user_id);
CREATE INDEX IF NOT EXISTS idx_user_auth_groups_group ON user_auth_groups(group_id);
`,
		Down: `
DROP TABLE IF EXISTS user_auth_groups;
DROP TABLE IF EXISTS auth_groups;
DROP TABLE IF EXISTS users;
`,
	})

	// v004: Resources + Audit Logs
	m.Register(Migration{
		Version:     4,
		Description: "Resources and Audit Logs",
		Up: `
CREATE TABLE IF NOT EXISTS resources (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    path TEXT NOT NULL,
    type TEXT
);
CREATE TABLE IF NOT EXISTS audit_logs (
    id SERIAL PRIMARY KEY,
    audit_type TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id TEXT,
    actor_user_id TEXT,
    actor_username TEXT,
    description TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created ON audit_logs(created_at DESC);
`,
		Down: `
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS resources;
`,
	})

	// v005: Alert Rules + Alert History
	m.Register(Migration{
		Version:     5,
		Description: "Alert Rules and Alert History",
		Up: `
CREATE TABLE IF NOT EXISTS alert_rules (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    metric_type TEXT NOT NULL,
    service_name TEXT,
    threshold_value REAL NOT NULL,
    operator TEXT NOT NULL,
    duration_seconds INTEGER DEFAULT 60,
    enabled BOOLEAN DEFAULT true,
    notification_channels TEXT,
    slack_webhook_url TEXT,
    email_webhook_url TEXT,
    discord_webhook_url TEXT,
    alert_suppress_seconds INTEGER DEFAULT 300,
    last_triggered_at TIMESTAMPTZ,
    last_triggered_value REAL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS alert_history (
    id SERIAL PRIMARY KEY,
    rule_id INTEGER REFERENCES alert_rules(id) ON DELETE CASCADE,
    rule_name TEXT NOT NULL,
    org_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    metric_type TEXT NOT NULL,
    operator TEXT NOT NULL,
    threshold REAL NOT NULL,
    actual_value REAL NOT NULL,
    triggered_at TIMESTAMPTZ DEFAULT NOW(),
    message TEXT,
    trace_id TEXT
);
CREATE INDEX IF NOT EXISTS idx_alert_history_rule_id ON alert_history(rule_id);
CREATE INDEX IF NOT EXISTS idx_alert_history_triggered_at ON alert_history(triggered_at DESC);
`,
		Down: `
DROP TABLE IF EXISTS alert_history;
DROP TABLE IF EXISTS alert_rules;
`,
	})

	// v006: API Key Requests + Config Snapshots
	m.Register(Migration{
		Version:     6,
		Description: "API Key Requests and Config Snapshots",
		Up: `
CREATE TABLE IF NOT EXISTS api_key_requests (
    id SERIAL PRIMARY KEY,
    key_name TEXT NOT NULL,
    consumer_name TEXT,
    description TEXT,
    status TEXT DEFAULT 'pending',
    applicant_user_id TEXT,
    applicant_username TEXT,
    reviewed_by TEXT,
    reviewed_at TIMESTAMPTZ,
    generated_key TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS config_snapshots (
    id SERIAL PRIMARY KEY,
    version_label TEXT NOT NULL,
    config_data TEXT,
    diff_from_prev TEXT,
    actor_user_id TEXT,
    actor_username TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
`,
		Down: `
DROP TABLE IF EXISTS config_snapshots;
DROP TABLE IF EXISTS api_key_requests;
`,
	})

	// v007: OAuth Providers + OAuth States
	m.Register(Migration{
		Version:     7,
		Description: "OAuth Providers and OAuth States",
		Up: `
CREATE TABLE IF NOT EXISTS oauth_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider TEXT UNIQUE NOT NULL,
    client_id TEXT NOT NULL,
    client_secret TEXT NOT NULL,
    issuer_url TEXT,
    authorization_url TEXT,
    token_url TEXT,
    userinfo_url TEXT,
    jwks_url TEXT,
    scopes TEXT DEFAULT 'openid email profile',
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS oauth_states (
    state TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    redirect_uri TEXT,
    expires_at TIMESTAMPTZ NOT NULL
);
`,
		Down: `
DROP TABLE IF EXISTS oauth_states;
DROP TABLE IF EXISTS oauth_providers;
`,
	})

	// v008: Login Attempts + OAuth user fields
	m.Register(Migration{
		Version:     8,
		Description: "Login Attempts and OAuth user fields",
		Up: `
CREATE TABLE IF NOT EXISTS login_attempts (
    id SERIAL PRIMARY KEY,
    username TEXT NOT NULL,
    ip_address TEXT,
    success BOOLEAN DEFAULT false,
    attempted_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_login_attempts_user ON login_attempts(username, attempted_at DESC);
CREATE INDEX IF NOT EXISTS idx_login_attempts_ip ON login_attempts(ip_address, attempted_at DESC);
ALTER TABLE users ADD COLUMN IF NOT EXISTS oauth_provider TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS oauth_subject TEXT;
CREATE INDEX IF NOT EXISTS idx_users_oauth ON users(oauth_provider, oauth_subject);
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;
`,
		Down: `
ALTER TABLE users ALTER COLUMN password_hash SET NOT NULL;
ALTER TABLE users DROP COLUMN IF EXISTS oauth_subject;
ALTER TABLE users DROP COLUMN IF EXISTS oauth_provider;
DROP INDEX IF EXISTS idx_users_oauth;
DROP TABLE IF EXISTS login_attempts;
`,
	})

	// v009: Consumer Credentials
	m.Register(Migration{
		Version:     9,
		Description: "Consumer Credentials (key-auth, basic-auth, hmac-auth)",
		Up: `
CREATE TABLE IF NOT EXISTS consumer_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    consumer_id UUID NOT NULL REFERENCES consumers(id) ON DELETE CASCADE,
    credential_type TEXT NOT NULL CHECK (credential_type IN ('key-auth', 'basic-auth', 'hmac-auth')),
    key TEXT NOT NULL,
    secret TEXT,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_consumer_cred_key_type ON consumer_credentials(credential_type, key);
CREATE INDEX IF NOT EXISTS idx_consumer_cred_consumer ON consumer_credentials(consumer_id);
`,
		Down: `DROP TABLE IF EXISTS consumer_credentials;`,
	})

	// v010: Workspace AuthGroups + User-Workspaces
	m.Register(Migration{
		Version:     10,
		Description: "Workspace AuthGroups binding and User-Workspaces",
		Up: `
CREATE TABLE IF NOT EXISTS workspace_auth_groups (
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    auth_group_id UUID NOT NULL REFERENCES auth_groups(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'viewer' CHECK (role IN ('viewer', 'editor', 'admin')),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (workspace_id, auth_group_id)
);
CREATE INDEX IF NOT EXISTS idx_wag_workspace ON workspace_auth_groups(workspace_id);
CREATE INDEX IF NOT EXISTS idx_wag_group ON workspace_auth_groups(auth_group_id);
CREATE TABLE IF NOT EXISTS user_workspaces (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'viewer' CHECK (role IN ('viewer', 'editor', 'admin')),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (user_id, workspace_id)
);
CREATE INDEX IF NOT EXISTS idx_uw_user ON user_workspaces(user_id);
CREATE INDEX IF NOT EXISTS idx_uw_workspace ON user_workspaces(workspace_id);
`,
		Down: `
DROP TABLE IF EXISTS user_workspaces;
DROP TABLE IF EXISTS workspace_auth_groups;
`,
	})

	// v011: Resource Permissions
	m.Register(Migration{
		Version:     11,
		Description: "Resource-level RBAC permissions",
		Up: `
CREATE TABLE IF NOT EXISTS resource_permissions (
    subject_type TEXT NOT NULL CHECK (subject_type IN ('user', 'group')),
    subject_id TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    permission TEXT NOT NULL DEFAULT 'read' CHECK (permission IN ('deny', 'read', 'write')),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (subject_type, subject_id, resource_id)
);
CREATE INDEX IF NOT EXISTS idx_rp_subject ON resource_permissions(subject_type, subject_id);
CREATE INDEX IF NOT EXISTS idx_rp_resource ON resource_permissions(resource_id);
`,
		Down: `DROP TABLE IF EXISTS resource_permissions;`,
	})

	// v012: Organizations (SaaS multi-tenancy)
	m.Register(Migration{
		Version:     12,
		Description: "Organizations (SaaS multi-tenancy Phase 1)",
		Up: `
CREATE TABLE IF NOT EXISTS organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT UNIQUE NOT NULL,
    plan TEXT DEFAULT 'free' CHECK (plan IN ('free', 'pro', 'enterprise')),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
ALTER TABLE users ADD COLUMN IF NOT EXISTS org_id UUID REFERENCES organizations(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_users_org ON users(org_id);
`,
		Down: `
ALTER TABLE users DROP COLUMN IF EXISTS org_id;
DROP TABLE IF EXISTS organizations;
`,
	})

	// v013: OTP + last_login_at
	m.Register(Migration{
		Version:     13,
		Description: "OTP for email verification and last_login_at",
		Up: `
CREATE TABLE IF NOT EXISTS otps (
    id SERIAL PRIMARY KEY,
    email TEXT NOT NULL,
    code TEXT NOT NULL,
    purpose TEXT NOT NULL CHECK (purpose IN ('register', 'reset-password')),
    expires_at TIMESTAMPTZ NOT NULL,
    verified BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_otps_email ON otps(email, purpose);
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMPTZ;
`,
		Down: `
ALTER TABLE users DROP COLUMN IF EXISTS last_login_at;
DROP TABLE IF EXISTS otps;
`,
	})

	// v014: Multi-tenant org_id on entity tables
	m.Register(Migration{
		Version:     14,
		Description: "Multi-tenant org_id on entity tables",
		Up: `
ALTER TABLE services   ADD COLUMN IF NOT EXISTS org_id UUID REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE routes      ADD COLUMN IF NOT EXISTS org_id UUID REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE upstreams   ADD COLUMN IF NOT EXISTS org_id UUID REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE targets     ADD COLUMN IF NOT EXISTS org_id UUID REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE consumers   ADD COLUMN IF NOT EXISTS org_id UUID REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE plugins     ADD COLUMN IF NOT EXISTS org_id UUID REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS org_id UUID REFERENCES organizations(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_services_org   ON services(org_id);
CREATE INDEX IF NOT EXISTS idx_routes_org     ON routes(org_id);
CREATE INDEX IF NOT EXISTS idx_upstreams_org ON upstreams(org_id);
CREATE INDEX IF NOT EXISTS idx_targets_org    ON targets(org_id);
CREATE INDEX IF NOT EXISTS idx_consumers_org  ON consumers(org_id);
CREATE INDEX IF NOT EXISTS idx_plugins_org    ON plugins(org_id);
CREATE INDEX IF NOT EXISTS idx_workspaces_org ON workspaces(org_id);
`,
		Down: `
ALTER TABLE workspaces DROP COLUMN IF EXISTS org_id;
ALTER TABLE plugins     DROP COLUMN IF EXISTS org_id;
ALTER TABLE consumers   DROP COLUMN IF EXISTS org_id;
ALTER TABLE targets     DROP COLUMN IF EXISTS org_id;
ALTER TABLE upstreams   DROP COLUMN IF EXISTS org_id;
ALTER TABLE routes      DROP COLUMN IF EXISTS org_id;
ALTER TABLE services   DROP COLUMN IF EXISTS org_id;
`,
	})

	// v015: Plugin scope + API Key enhanced fields
	m.Register(Migration{
		Version:     15,
		Description: "Plugin global scoping and API Key enhanced fields",
		Up: `
ALTER TABLE plugins ADD COLUMN IF NOT EXISTS scope TEXT NOT NULL DEFAULT 'service' CHECK (scope IN ('global','workspace','service','route','consumer'));
CREATE INDEX IF NOT EXISTS idx_plugins_scope ON plugins(scope);
ALTER TABLE api_key_requests ADD COLUMN IF NOT EXISTS reason TEXT;
ALTER TABLE api_key_requests ADD COLUMN IF NOT EXISTS scopes TEXT;
ALTER TABLE api_key_requests ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;
ALTER TABLE api_key_requests ADD COLUMN IF NOT EXISTS key_value TEXT;
`,
		Down: `
ALTER TABLE api_key_requests DROP COLUMN IF EXISTS key_value;
ALTER TABLE api_key_requests DROP COLUMN IF EXISTS expires_at;
ALTER TABLE api_key_requests DROP COLUMN IF EXISTS scopes;
ALTER TABLE api_key_requests DROP COLUMN IF EXISTS reason;
ALTER TABLE plugins DROP COLUMN IF EXISTS scope;
`,
	})

	// v016: Alert rule conditions + triggered state
	m.Register(Migration{
		Version:     16,
		Description: "Alert rule conditions and triggered state",
		Up: `
ALTER TABLE alert_rules ADD COLUMN IF NOT EXISTS last_triggered_at TIMESTAMPTZ;
ALTER TABLE alert_rules ADD COLUMN IF NOT EXISTS last_triggered_value REAL;
ALTER TABLE alert_rules ADD COLUMN IF NOT EXISTS conditions JSONB DEFAULT '[{"metric_type":"","service_name":"","threshold_value":0,"operator":">","logic":"AND"}]'::jsonb;
`,
		Down: `
ALTER TABLE alert_rules DROP COLUMN IF EXISTS conditions;
ALTER TABLE alert_rules DROP COLUMN IF EXISTS last_triggered_value;
ALTER TABLE alert_rules DROP COLUMN IF EXISTS last_triggered_at;
`,
	})

	// v017: Billing/Plan (Stripe integration)
	m.Register(Migration{
		Version:     17,
		Description: "Billing Plans and Subscriptions (Stripe)",
		Up: `
CREATE TABLE IF NOT EXISTS plans (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    display_name TEXT NOT NULL,
    price_monthly INTEGER DEFAULT 0,
    price_yearly INTEGER DEFAULT 0,
    features TEXT DEFAULT '[]',
    workspace_limit INTEGER DEFAULT 3,
    user_limit INTEGER DEFAULT 5,
    request_limit BIGINT DEFAULT 1000,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS subscriptions (
    id TEXT PRIMARY KEY,
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    plan_name TEXT NOT NULL DEFAULT 'free',
    stripe_customer_id TEXT,
    stripe_subscription_id TEXT UNIQUE,
    stripe_price_id TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    billing_cycle TEXT DEFAULT 'monthly',
    current_period_start TIMESTAMPTZ,
    current_period_end TIMESTAMPTZ,
    cancel_at_period_end BOOLEAN DEFAULT false,
    trial_end TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_subscriptions_org ON subscriptions(org_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_stripe_sub ON subscriptions(stripe_subscription_id);
`,
		Down: `
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS plans;
`,
	})

	// v018: Notifications SSE
	m.Register(Migration{
		Version:     18,
		Description: "Notifications for SSE event store",
		Up: `
CREATE TABLE IF NOT EXISTS notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    type TEXT NOT NULL,
    payload TEXT DEFAULT '{}',
    read BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_notifications_user ON notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_user_unread ON notifications(user_id, read) WHERE read = false;
`,
		Down: `DROP TABLE IF EXISTS notifications;`,
	})

	// v019: Webhook Subscriptions + Deliveries
	m.Register(Migration{
		Version:     19,
		Description: "Webhook Subscriptions and Deliveries",
		Up: `
CREATE TABLE IF NOT EXISTS webhook_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL,
    url TEXT NOT NULL,
    event_types TEXT[] NOT NULL,
    secret TEXT,
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_webhook_subscriptions_org ON webhook_subscriptions(org_id);
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL,
    webhook_id UUID NOT NULL REFERENCES webhook_subscriptions(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    payload TEXT NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'success', 'failed', 'retrying')),
    attempts INT NOT NULL DEFAULT 0,
    last_attempt TIMESTAMPTZ,
    next_retry TIMESTAMPTZ,
    last_error TEXT,
    response_status INT,
    response_body TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_webhook ON webhook_deliveries(webhook_id);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_status ON webhook_deliveries(status, next_retry);
`,
		Down: `
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhook_subscriptions;
`,
	})

	// v020: gRPC Services + Methods
	m.Register(Migration{
		Version:     20,
		Description: "gRPC Services and Methods",
		Up: `
CREATE TABLE IF NOT EXISTS grpc_services (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    package TEXT DEFAULT '',
    proto_file TEXT DEFAULT '',
    upstream_id UUID,
    enabled BOOLEAN DEFAULT true,
    org_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(org_id, name)
);
CREATE INDEX IF NOT EXISTS idx_grpc_services_org ON grpc_services(org_id);
CREATE INDEX IF NOT EXISTS idx_grpc_services_upstream ON grpc_services(upstream_id);
CREATE TABLE IF NOT EXISTS grpc_methods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id UUID NOT NULL REFERENCES grpc_services(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    method_type TEXT DEFAULT 'unary' CHECK (method_type IN ('unary', 'client_streaming', 'server_streaming', 'bidirectional')),
    input_type TEXT DEFAULT '',
    output_type TEXT DEFAULT '',
    enabled BOOLEAN DEFAULT true,
    org_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(service_id, name)
);
CREATE INDEX IF NOT EXISTS idx_grpc_methods_service ON grpc_methods(service_id);
`,
		Down: `
DROP TABLE IF EXISTS grpc_methods;
DROP TABLE IF EXISTS grpc_services;
`,
	})

	// v021: Consumer credential TTL
	m.Register(Migration{
		Version:     21,
		Description: "Consumer credential TTL (expires_at)",
		Up:          `ALTER TABLE consumer_credentials ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;`,
		Down:        `ALTER TABLE consumer_credentials DROP COLUMN IF EXISTS expires_at;`,
	})

	// v022: OAuth providers updated_at column
	m.Register(Migration{
		Version:     22,
		Description: "OAuth providers updated_at column",
		Up:   `DO $$ BEGIN ALTER TABLE oauth_providers ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ; EXCEPTION WHEN OTHERS THEN NULL; END $$;`,
		Down: `ALTER TABLE oauth_providers DROP COLUMN IF EXISTS updated_at;`,
	})

	// v023: Alert rules org_id + threshold_type/percentage_threshold for multi-tenant usage quota alerting
	m.Register(Migration{
		Version:     23,
		Description: "Add org_id, threshold_type, percentage_threshold to alert_rules for multi-tenant usage quota alerting",
		Up: `
DO $$ BEGIN
  ALTER TABLE alert_rules ADD COLUMN IF NOT EXISTS org_id TEXT DEFAULT '00000000-0000-0000-0000-000000000000';
  ALTER TABLE alert_rules ADD COLUMN IF NOT EXISTS threshold_type TEXT DEFAULT 'absolute';
  ALTER TABLE alert_rules ADD COLUMN IF NOT EXISTS percentage_threshold REAL DEFAULT 0;
EXCEPTION WHEN OTHERS THEN NULL;
END $$;
`,
		Down: `ALTER TABLE alert_rules DROP COLUMN IF EXISTS percentage_threshold; ALTER TABLE alert_rules DROP COLUMN IF EXISTS threshold_type; ALTER TABLE alert_rules DROP COLUMN IF EXISTS org_id;`,
	})

	// v024: Alert rules quota_metric_type for usage quota per-org vs per-consumer
	m.Register(Migration{
		Version:     24,
		Description: "Add quota_metric_type to alert_rules for per-org vs per-consumer usage quota",
		Up:          `ALTER TABLE alert_rules ADD COLUMN IF NOT EXISTS quota_metric_type TEXT DEFAULT 'org';`,
		Down:        `ALTER TABLE alert_rules DROP COLUMN IF EXISTS quota_metric_type;`,
	})

	// v025: Services upstream_id column
	m.Register(Migration{
		Version:     25,
		Description: "Add upstream_id to services for binding a service to an upstream",
		Up: `
DO $$ BEGIN
  ALTER TABLE services ADD COLUMN IF NOT EXISTS upstream_id UUID REFERENCES upstreams(id) ON DELETE SET NULL;
EXCEPTION WHEN OTHERS THEN NULL;
END $$;
`,
		Down: `ALTER TABLE services DROP COLUMN IF EXISTS upstream_id;`,
	})

	// v026: JWT Credential Support
	m.Register(Migration{
		Version:     26,
		Description: "JWT Credential support (algorithm field for jwt credential type)",
		Up: `
ALTER TABLE consumer_credentials ADD COLUMN IF NOT EXISTS algorithm TEXT;
ALTER TABLE consumer_credentials DROP CONSTRAINT IF EXISTS consumer_credentials_credential_type_check;
ALTER TABLE consumer_credentials ADD CONSTRAINT consumer_credentials_credential_type_check CHECK (credential_type IN ('key-auth', 'basic-auth', 'hmac-auth', 'jwt'));
`,
		Down: `
ALTER TABLE consumer_credentials DROP COLUMN IF EXISTS algorithm;
ALTER TABLE consumer_credentials DROP CONSTRAINT IF EXISTS consumer_credentials_credential_type_check;
ALTER TABLE consumer_credentials ADD CONSTRAINT consumer_credentials_credential_type_check CHECK (credential_type IN ('key-auth', 'basic-auth', 'hmac-auth'));
`,
	})

	// v027: Webhook tables
	m.Register(Migration{
		Version:     27,
		Description: "Add webhook_subscriptions and webhook_deliveries tables",
		Up: `
CREATE TABLE IF NOT EXISTS webhook_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL,
    url TEXT NOT NULL,
    event_types TEXT[] NOT NULL,
    secret TEXT,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL,
    webhook_id UUID NOT NULL REFERENCES webhook_subscriptions(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    payload TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'success', 'failed', 'retrying')),
    attempts INT NOT NULL DEFAULT 0,
    last_error TEXT,
    response_body TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    delivered_at TIMESTAMPTZ
);
`,
		Down: `
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhook_subscriptions;
`,
	})
}

// Migrate runs all pending migrations
func Migrate(db *sql.DB) error {
	m := NewManager(db)
	RegisterAllMigrations(m)
	return m.Run()
}

// RollbackLast rolls back the most recent migration
func RollbackLast(db *sql.DB) error {
	m := NewManager(db)
	RegisterAllMigrations(m)
	applied, err := m.Status()
	if err != nil {
		return err
	}
	if len(applied) == 0 {
		log.Println("No migrations to rollback")
		return nil
	}
	return m.Rollback(applied[len(applied)-1])
}

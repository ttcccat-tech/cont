import axios from 'axios'

const API_BASE = import.meta.env.VITE_API_BASE || '/api'
export const analyticsClient = axios.create({ baseURL: API_BASE })

// ── Legacy alias (for code that imports 'api' from kong.ts) ──
// Points to same /api base — routes through nginx to Cont backend.
export const kongClient = analyticsClient

// ── Storage keys ──────────────────────────────────────────
const WS_KEY = 'cont_ws'
const TOKEN_KEY = 'cont_token'
const PERMS_KEY = 'cont_perms'

// ── Workspace helpers ─────────────────────────────────────
export function getKongWorkspace(): string {
  return sessionStorage.getItem(WS_KEY) || 'default'
}
export function setKongWorkspace(ws: string) {
  sessionStorage.setItem(WS_KEY, ws)
}
// Cont Admin API is single-workspace, no ?workspace= query param injection needed
const wsPrefix = (path: string) => path

// ── Token helpers ──────────────────────────────────────────
export function getToken(): string | null { return localStorage.getItem(TOKEN_KEY) }
export function setToken(token: string): void { localStorage.setItem(TOKEN_KEY, token) }
export function clearAuth(): void {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(PERMS_KEY)
}
export function getUserPerms(): Record<string, unknown> {
  try {
    const raw = localStorage.getItem(PERMS_KEY)
    return raw ? JSON.parse(raw) : {}
  } catch { return {} }
}
export function setUserPerms(perms: Record<string, unknown>): void {
  localStorage.setItem(PERMS_KEY, JSON.stringify(perms))
}

// ── Analytics interceptor ─────────────────────────────────
analyticsClient.interceptors.request.use(config => {
  const token = getToken()
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})
analyticsClient.interceptors.response.use(r => r, err => {
  if (err.response?.status === 401) { clearAuth(); window.location.href = '/login' }
  return Promise.reject(err)
})

// ── Types ─────────────────────────────────────────────────
export interface KongUpstream {
  id?: string; name: string; algorithm?: string; slots?: number
  healthchecks?: string; enabled?: boolean; created_at?: number
}

export interface TargetHealth {
  id: string; target: string; weight: number; enabled: boolean
  healthy: boolean; port: number; host: string
}

export interface KongTarget {
  id?: string; target?: string; weight?: number; enabled?: boolean; upstream_id?: string
  created_at?: string
}

export interface UpstreamHealth {
  upstream_id: string; upstream_name: string; algorithm: string
  enabled: boolean; targets: TargetHealth[]
}

export interface CircuitBreakerConfig {
  upstream_id: string
  enabled: boolean
  trip_threshold: number
  recovery_timeout: number
  half_open_max_requests: number
  half_open_success_rate: number
}

export interface KongService {
  id?: string; name: string; url?: string; host: string; port: number
  path?: string; protocol?: string; retries?: number; connect_timeout?: number
  read_timeout?: number; write_timeout?: number; enabled?: boolean; created_at?: number
}

export interface GrpcService {
  id?: string; name: string; package?: string; proto_file?: string
  upstream_id?: string; enabled?: boolean; created_at?: string; updated_at?: string
}

export interface GrpcMethod {
  id?: string; service_id?: string; name: string; method_type?: string
  input_type?: string; output_type?: string; enabled?: boolean
  created_at?: string; updated_at?: string
}

export interface KongRoute {
  id?: string; name?: string; service?: { id: string }
  protocol?: string; paths?: string[]; methods?: string[]
  strip_path?: boolean; preserve_host?: boolean; hosts?: string[]
  created_at?: number; updated_at?: number
}

export interface KongPlugin {
  id?: string; name: string; service?: { id: string }
  consumer?: { id: string }; route?: { id: string }
  config?: Record<string, unknown>; enabled?: boolean; created_at?: number
  scope?: string // global, workspace, service, route, consumer
}

// Plugin type schema from the built-in registry
export interface PluginSchema {
  name: string
  version?: string
  label?: string
  description?: string
  access_phase?: boolean
  log_phase?: boolean
  pre_proxy?: boolean
  post_proxy?: boolean
  config_schema?: Record<string, unknown>
}

export interface KongConsumer {
  id?: string; username: string; custom_id?: string; created_at?: number
}

export interface KongStatus {
  memory: { lua_shared_dicts?: Record<string, { allocated_slabs: string; capacity: string }>
    workers_lua_vms?: Array<{ pid: number; http_allocated_gc: string }> }
  server: { connections_active: number; connections_accepted: number; connections_handled: number
    connections_waiting: number; connections_reading: number; connections_writing: number; total_requests: number }
  database: { reachable: boolean }
}

export interface KongInfo { version: string; tagline: string; plugins?: { enabled_in_cluster: string[] } }

export type PermissionMode = 'deny' | 'read' | 'write'
export interface PermissionEntry { resource_id: string; mode: PermissionMode }
export interface Resource { id: string; name: string; path: string; type?: string }
export interface ResourcePermission { subject_type?: string; subject_id?: string; resource_id: string; permission: string; resource_name?: string }
export interface AuthGroup { id?: string; name: string; label: string; description?: string; permissions?: PermissionEntry[]; created_at?: number }
export interface Workspace { id: string; name: string; label: string; description?: string; kong_workspace_id?: string; created_at?: number; group_ids?: string[] }
export interface WorkspaceUserAssignment { workspace_id: string; workspace_name?: string; user_id: string; username: string; display_name?: string; email?: string; role: string; assigned_at?: string }
export interface AuditEntry { id: number; audit_type: string; target_type: string; target_id: string; actor_username: string; actor_user_id: string; description: string; created_at: string }

function parsePromMetrics(text: string): Record<string, number> {
  const result: Record<string, number> = {}
  for (const line of text.split('\n')) {
    if (line.startsWith('#') || !line.includes(' ')) continue
    const idx = line.indexOf(' ')
    if (idx === -1) continue
    const metricWithLabels = line.slice(0, idx)
    const value = parseFloat(line.slice(idx + 1).trim().split(/\s/)[0])
    if (!isNaN(value)) {
      // Normalize labels: keep only {state=...} from Kong's full {node_id=...,subsystem=...,state=...}
      // Chart looks up {state="active"}, {state="accepted"} etc. — extract state label only.
      const labelStart = metricWithLabels.indexOf('{')
      const baseMetric = labelStart !== -1 ? metricWithLabels.slice(0, labelStart) : metricWithLabels
      if (labelStart !== -1) {
        // Store the full metric+labels key
        result[metricWithLabels] = value
        const labelEnd = metricWithLabels.lastIndexOf('}')
        const labels = metricWithLabels.slice(labelStart + 1, labelEnd)
        // Extract specific label(s) that charts look up
        const stateMatch = labels.match(/state="([^"]+)"/)
        if (stateMatch) {
          result[`${baseMetric}{state="${stateMatch[1]}"}`] = value
        }
        const sdMatch = labels.match(/shared_dict="([^"]+)"/)
        if (sdMatch) {
          result[`${baseMetric}{shared_dict="${sdMatch[1]}"}`] = value
        }
      } else {
        result[baseMetric] = value
      }
    }
  }
  return result
}

// ── Analytics API (all /api/* calls) ─────────────────────
export const login = (username: string, password: string) =>
  analyticsClient.post('/auth/login', { username, password }).then(r => r.data)
export const getMe = () => analyticsClient.get('/auth/me').then(r => r.data)

// Notifications
export interface Notification {
  id: string
  user_id: string
  type: string
  payload: string
  read: boolean
  created_at: string
}
export const listNotifications = (limit = 50, offset = 0) =>
  analyticsClient.get<Notification[]>(`/auth/notifications?limit=${limit}&offset=${offset}`).then(r => r.data)
export const markNotificationRead = (id: string) =>
  analyticsClient.put(`/auth/notifications/${id}/read`).then(r => r.data)
export const markAllNotificationsRead = () =>
  analyticsClient.put('/auth/notifications/read-all').then(r => r.data)
export const getUnreadCount = () =>
  analyticsClient.get<{ unread: number }>('/auth/notifications/unread-count').then(r => r.data)

// OAuth2 Provider Management
export interface OAuth2Provider {
  id?: string
  provider: string
  client_id: string
  client_secret?: string
  issuer_url?: string
  authorization_url?: string
  token_url: string
  userinfo_url?: string
  jwks_url?: string
  scopes?: string
  enabled: boolean
  created_at?: string
  updated_at?: string
}
export const listOAuthProviders = () => analyticsClient.get<OAuth2Provider[]>('/auth/oauth/providers').then(r => r.data)
export const getOAuthProvider = (provider: string) => analyticsClient.get<OAuth2Provider>(`/auth/oauth/providers/${provider}`).then(r => r.data)
export const createOAuthProvider = (data: Partial<OAuth2Provider>) => analyticsClient.post<OAuth2Provider>('/auth/oauth/providers', data).then(r => r.data)
export const updateOAuthProvider = (provider: string, data: Partial<OAuth2Provider>) => analyticsClient.put<OAuth2Provider>(`/auth/oauth/providers/${provider}`, data).then(r => r.data)
export const deleteOAuthProvider = (provider: string) => analyticsClient.delete(`/auth/oauth/providers/${provider}`)

export const getGroups = () => analyticsClient.get<AuthGroup[]>('/groups').then(r => r.data)
export const createGroup = (data: Partial<AuthGroup>) => analyticsClient.post<AuthGroup>('/groups', data).then(r => r.data)
export const updateGroup = (id: string, data: Partial<AuthGroup>) => analyticsClient.patch<AuthGroup>(`/groups/${id}`, data).then(r => r.data)
export const deleteGroup = (id: string) => analyticsClient.delete(`/groups/${id}`)
export const getGroupMembers = (id: string) => analyticsClient.get<{members: {id:string;username:string;display_name:string;email:string;role:string}[]}>(`/groups/${id}/members`).then(r => r.data)
export const setGroupMembers = (id: string, userIds: string[]) => analyticsClient.put(`/groups/${id}/members`, {user_ids: userIds}).then(r => r.data)
export const getGroupResourcePermissions = (id: string) => analyticsClient.get<{permissions: ResourcePermission[]}>(`/groups/${id}/resource-permissions`).then(r => r.data?.permissions ?? [])
export const setGroupResourcePermissions = (id: string, permissions: ResourcePermission[]) => analyticsClient.put(`/groups/${id}/resource-permissions`, permissions).then(r => r.data)
export const getUserResourcePermissions = (id: string) => analyticsClient.get<{permissions: ResourcePermission[]}>(`/users/${id}/resource-permissions`).then(r => r.data?.permissions ?? [])
export const setUserResourcePermissions = (id: string, permissions: ResourcePermission[]) => analyticsClient.put(`/users/${id}/resource-permissions`, permissions).then(r => r.data)

export const listWorkspaces = () => analyticsClient.get<Workspace[]>('/workspaces').then(r => r.data)
export const listMyWorkspaces = () => analyticsClient.get<Workspace[]>('/workspaces/mine').then(r => r.data)
export const createWorkspace = (data: { name: string; label: string; description?: string }) =>
  analyticsClient.post<Workspace>('/workspaces', data).then(r => r.data)
export const updateWorkspace = (id: string, data: { label?: string; description?: string }) =>
  analyticsClient.patch<Workspace>(`/workspaces/${id}`, data).then(r => r.data)
export const deleteWorkspace = (id: string) => analyticsClient.delete(`/workspaces/${id}`)

// Workspace user assignment management
export const getWorkspaceUsers = (workspaceId: string) =>
  analyticsClient.get<{ data: WorkspaceUserAssignment[] }>(`/workspaces/${workspaceId}/users`).then(r => r.data?.data ?? [])
export const setWorkspaceUser = (workspaceId: string, userId: string, role: string) =>
  analyticsClient.put(`/workspaces/${workspaceId}/users`, { user_id: userId, role }).then(r => r.data)
export const removeWorkspaceUser = (workspaceId: string, userId: string) =>
  analyticsClient.delete(`/workspaces/${workspaceId}/users/${userId}`)

// User workspace assignments (GET /workspaces/users/:userId)
export const getUserWorkspaces = (userId: string) =>
  analyticsClient.get<{ data: WorkspaceUserAssignment[] }>(`/workspaces/users/${userId}`).then(r => r.data?.data ?? [])

export const listResources = () => analyticsClient.get<{ resources: Resource[] }>('/resources').then(r => r.data?.resources ?? [])

export const getAuditLogs = (params?: Record<string, string>) => {
  const searchParams = new URLSearchParams(params || {})
  const query = searchParams.toString() ? '?' + searchParams.toString() : ''
  return analyticsClient.get<{ data: AuditEntry[]; total: number }>(`/audit${query}`).then(r => r.data)
}

export const exportAuditLogsCSV = (params?: Record<string, string>) => {
  const searchParams = new URLSearchParams(params || {})
  const query = searchParams.toString() ? '?' + searchParams.toString() : ''
  const url = `${analyticsClient.getUri()}/audit/export${query}`
  window.open(url, '_blank')
}

export const getAlertRules = () => analyticsClient.get('/alerts/rules').then(r => r.data)
export const createAlertRule = (payload: Record<string, unknown>) => analyticsClient.post('/alerts/rules', payload).then(r => r.data)
export const updateAlertRule = (id: string, payload: Record<string, unknown>) => analyticsClient.patch(`/alerts/rules/${id}`, payload).then(r => r.data)
export const deleteAlertRule = (id: string) => analyticsClient.delete(`/alerts/rules/${id}`)

export const getUsers = () => analyticsClient.get('/users').then(r => r.data)
export const createUser = (payload: Record<string, unknown>) => analyticsClient.post('/users', payload).then(r => r.data)
export const inviteUser = (email: string, groupId: string) =>
  analyticsClient.post('/users/invite', { email, group_id: groupId }).then(r => r.data)
export const updateUser = (id: string, payload: Record<string, unknown>) =>
  analyticsClient.put(`/users/${id}`, payload).then(r => r.data)
export const deleteUser = (id: string) => analyticsClient.delete(`/users/${id}`).then(r => r.data)
export const changePassword = (userId: string, oldPassword: string, newPassword: string) =>
  analyticsClient.put(`/users/${userId}/password`, { oldPassword, newPassword }).then(r => r.data)

export const getConfigSnapshots = (limit = 50) => analyticsClient.get(`/config/snapshots?limit=${limit}`).then(r => r.data)
export const getConfigSnapshot = (id: number) => analyticsClient.get(`/config/snapshots/${id}`).then(r => r.data)
export const createConfigSnapshot = (version_label?: string) =>
  analyticsClient.post('/config/snapshots', { version_label }).then(r => r.data)
export const rollbackSnapshot = (id: number) => analyticsClient.post(`/config/snapshots/${id}/rollback`).then(r => r.data)
export const diffSnapshots = (id1: number, id2: number) => analyticsClient.get(`/config/snapshots/diff?id1=${id1}&id2=${id2}`).then(r => r.data)
export const deleteConfigSnapshot = (id: number) => analyticsClient.delete(`/config/snapshots/${id}`).then(r => r.data)

export const generateRSAKeyPair = () => analyticsClient.post<{ publicKey: string; privateKey: string }>('/crypto/rsa-keypair').then(r => r.data)
export const getCurrentConfig = () => analyticsClient.get('/config/current').then(r => r.data)

// ── Billing / Stripe ───────────────────────────────────────
export interface Plan {
  id: string
  name: string
  display_name: string
  price_monthly: number
  price_yearly: number
  features: string
  workspace_limit: number
  user_limit: number
  request_limit: number
}

export interface Subscription {
  id: string
  org_id: string
  plan_name: string
  stripe_customer_id: string
  stripe_subscription_id: string
  stripe_price_id: string
  status: string
  billing_cycle: string
  current_period_start: string
  current_period_end: string
  cancel_at_period_end: boolean
  trial_end?: string
}

export const getPlans = () => analyticsClient.get<Plan[]>('/billing/plans').then(r => r.data)
export const getSubscription = () => analyticsClient.get<Subscription>('/billing/subscription').then(r => r.data)
export const getUsage = () => analyticsClient.get<{org_id: string; plan: string; used: number; limit: number; percent_used: number; reset_at: string}>('/billing/usage').then(r => r.data)
export const createCheckoutSession = (planName: string, billingCycle: string) =>
  analyticsClient.post<{ url: string }>('/billing/checkout', { plan_name: planName, billing_cycle: billingCycle }).then(r => r.data)
export const createPortalSession = () =>
  analyticsClient.post<{ url: string }>('/billing/portal').then(r => r.data)

export const getStatus = () => kongClient.get<KongStatus>(wsPrefix('/status')).then(r => r.data)
export const getInfo = () => kongClient.get<KongInfo>(wsPrefix('/')).then(r => r.data)
export const getMetrics = () => kongClient.get(wsPrefix('/metrics'), { transformResponse: [(d) => d] }).then(r => {
  const text = typeof r.data === 'string' ? r.data : String(r.data)
  return parsePromMetrics(text)
})

// ── Cont Admin API ────────────────────────────────────────
// All entities use analyticsClient (which goes through nginx /api/* to Cont backend.
// JWT token injected automatically by analyticsClient interceptor.
export const api = {
  listUpstreams: () => analyticsClient.get<KongUpstream[]>(wsPrefix('/upstreams')).then(r => r.data?.data ?? []),
  getUpstream: (id: string) => analyticsClient.get<KongUpstream>(wsPrefix(`/upstreams/${id}`)).then(r => r.data),
  getUpstreamHealth: (id: string) => analyticsClient.get<UpstreamHealth>(wsPrefix(`/upstreams/${id}/health`)).then(r => r.data),
  createUpstream: (data: Partial<KongUpstream>) =>
    analyticsClient.post<KongUpstream>(wsPrefix('/upstreams'), data).then(r => r.data),
  updateUpstream: (id: string, data: Partial<KongUpstream>) =>
    analyticsClient.patch<KongUpstream>(wsPrefix(`/upstreams/${id}`), data).then(r => r.data),
  deleteUpstream: (id: string) =>
    analyticsClient.delete(wsPrefix(`/upstreams/${id}`)),

  listUpstreamTargets: (upstreamId: string) =>
    analyticsClient.get<KongTarget[]>(wsPrefix(`/upstreams/${upstreamId}/targets`)).then(r => r.data?.data ?? []),
  createUpstreamTarget: (upstreamId: string, data: Partial<KongTarget>) =>
    analyticsClient.post<KongTarget>(wsPrefix(`/upstreams/${upstreamId}/targets`), data).then(r => r.data),
  updateUpstreamTarget: (upstreamId: string, targetId: string, data: Partial<KongTarget>) =>
    analyticsClient.patch<KongTarget>(wsPrefix(`/upstreams/${upstreamId}/targets/${targetId}`), data).then(r => r.data),
  deleteUpstreamTarget: (upstreamId: string, targetId: string) =>
    analyticsClient.delete(wsPrefix(`/upstreams/${upstreamId}/targets/${targetId}`)),

  // Circuit Breaker
  getCircuitBreaker: (upstreamId: string) =>
    analyticsClient.get<CircuitBreakerConfig>(wsPrefix(`/upstreams/${upstreamId}/circuit-breaker`)).then(r => r.data),
  setCircuitBreaker: (upstreamId: string, data: Partial<CircuitBreakerConfig>) =>
    analyticsClient.post<CircuitBreakerConfig>(wsPrefix(`/upstreams/${upstreamId}/circuit-breaker`), data).then(r => r.data),

  listServices: () => analyticsClient.get<KongService[]>(wsPrefix('/services')).then(r => r.data?.data ?? []),
  getService: (id: string) => analyticsClient.get<KongService>(wsPrefix(`/services/${id}`)).then(r => r.data),
  createService: (data: Partial<KongService>) => analyticsClient.post<KongService>(wsPrefix('/services'), data).then(r => r.data),
  updateService: (id: string, data: Partial<KongService>) => analyticsClient.patch<KongService>(wsPrefix(`/services/${id}`), data).then(r => r.data),
  deleteService: (id: string) => analyticsClient.delete(wsPrefix(`/services/${id}`)),

  listRoutes: () => analyticsClient.get<KongRoute[]>(wsPrefix('/routes')).then(r => r.data?.data ?? []),
  getRoute: (id: string) => analyticsClient.get<KongRoute>(wsPrefix(`/routes/${id}`)).then(r => r.data),
  createRoute: (data: Partial<KongRoute>) => analyticsClient.post<KongRoute>(wsPrefix('/routes'), data).then(r => r.data),
  updateRoute: (id: string, data: Partial<KongRoute>) => analyticsClient.patch<KongRoute>(wsPrefix(`/routes/${id}`), data).then(r => r.data),
  deleteRoute: (id: string) => analyticsClient.delete(wsPrefix(`/routes/${id}`)),

  listPlugins: () => analyticsClient.get<KongPlugin[]>(wsPrefix('/plugins')).then(r => r.data?.data ?? []),
  getPlugin: (id: string) => analyticsClient.get<KongPlugin>(wsPrefix(`/plugins/${id}`)).then(r => r.data),
  createPlugin: (data: Partial<KongPlugin>) => analyticsClient.post<KongPlugin>(wsPrefix('/plugins'), data).then(r => r.data),
  updatePlugin: (id: string, data: Partial<KongPlugin>) => analyticsClient.patch<KongPlugin>(wsPrefix(`/plugins/${id}`), data).then(r => r.data),
  deletePlugin: (id: string) => analyticsClient.delete(wsPrefix(`/plugins/${id}`)),

  listConsumers: () => analyticsClient.get<KongConsumer[]>(wsPrefix('/consumers')).then(r => r.data?.data ?? []),
  createConsumer: (data: Partial<KongConsumer>) => analyticsClient.post<KongConsumer>(wsPrefix('/consumers'), data).then(r => r.data),
  updateConsumer: (id: string, data: Partial<KongConsumer>) => analyticsClient.patch<KongConsumer>(wsPrefix(`/consumers/${id}`), data).then(r => r.data),
  deleteConsumer: (id: string) => analyticsClient.delete(wsPrefix(`/consumers/${id}`)),

  listJWTCredentials: (consumerId: string) =>
    analyticsClient.get<unknown[]>(wsPrefix(`/consumers/${consumerId}/jwt`)).then(r => r.data?.data ?? []),
  createJWTCredential: (consumerId: string, data: unknown) =>
    analyticsClient.post(wsPrefix(`/consumers/${consumerId}/jwt`), data).then(r => r.data),
  deleteJWTCredential: (consumerId: string, credentialId: string) =>
    analyticsClient.delete(wsPrefix(`/consumers/${consumerId}/jwt/${credentialId}`)),

  listKeyAuthCredentials: (consumerId: string) =>
    analyticsClient.get<unknown[]>(wsPrefix(`/consumers/${consumerId}/key-auth/credentials`)).then(r => r.data ?? []),
  createKeyAuthCredential: (consumerId: string, data?: unknown) =>
    analyticsClient.post(wsPrefix(`/consumers/${consumerId}/key-auth/credentials`), data || {}).then(r => r.data),
  updateKeyAuthCredential: (consumerId: string, credentialId: string, data: unknown) =>
    analyticsClient.patch(wsPrefix(`/consumers/${consumerId}/key-auth/credentials/${credentialId}`), data).then(r => r.data),
  deleteKeyAuthCredential: (consumerId: string, credentialId: string) =>
    analyticsClient.delete(wsPrefix(`/consumers/${consumerId}/key-auth/credentials/${credentialId}`)),

  // gRPC Services
  listGrpcServices: () => analyticsClient.get<GrpcService[]>(wsPrefix('/grpc-services')).then(r => r.data?.data ?? []),
  getGrpcService: (id: string) => analyticsClient.get<GrpcService>(wsPrefix(`/grpc-services/${id}`)).then(r => r.data),
  createGrpcService: (data: Partial<GrpcService>) => analyticsClient.post<GrpcService>(wsPrefix('/grpc-services'), data).then(r => r.data),
  updateGrpcService: (id: string, data: Partial<GrpcService>) => analyticsClient.patch<GrpcService>(wsPrefix(`/grpc-services/${id}`), data).then(r => r.data),
  deleteGrpcService: (id: string) => analyticsClient.delete(wsPrefix(`/grpc-services/${id}`)),
  listGrpcMethods: (serviceId: string) => analyticsClient.get<GrpcMethod[]>(wsPrefix(`/grpc-services/${serviceId}/methods`)).then(r => r.data?.data ?? []),
  createGrpcMethod: (serviceId: string, data: Partial<GrpcMethod>) =>
    analyticsClient.post<GrpcMethod>(wsPrefix(`/grpc-services/${serviceId}/methods`), data).then(r => r.data),
  deleteGrpcMethod: (serviceId: string, methodId: string) =>
    analyticsClient.delete(wsPrefix(`/grpc-services/${serviceId}/methods/${methodId}`)),

  // Plugin registry (available plugin types)
  getPluginRegistry: () => analyticsClient.get<{ plugins: PluginSchema[] }>('/internal/plugin-registry').then(r => r.data?.plugins ?? []),
}

export default api
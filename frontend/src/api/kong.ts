import axios from 'axios'

const KONG_BASE = import.meta.env.VITE_KONG_BASE || ''
const ADMIN_TOKEN = import.meta.env.VITE_KONG_ADMIN_TOKEN || ''

export const kongClient = axios.create({
  baseURL: KONG_BASE,
  headers: ADMIN_TOKEN ? { 'Kong-Admin-Token': ADMIN_TOKEN } : {},
})

const API_BASE = import.meta.env.VITE_API_BASE || '/api'
export const analyticsClient = axios.create({ baseURL: API_BASE })

// ── Storage keys ──────────────────────────────────────────
const WS_KEY = 'kgo_ws'
const TOKEN_KEY = 'cont_token'
const PERMS_KEY = 'cont_perms'

// ── Workspace helpers ─────────────────────────────────────
export function getKongWorkspace(): string {
  return sessionStorage.getItem(WS_KEY) || 'default'
}
export function setKongWorkspace(ws: string) {
  sessionStorage.setItem(WS_KEY, ws)
}
// No-op: cont Admin API is single-workspace, no ?workspace= query param injection needed
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

// ── Kong Admin interceptor ─────────────────────────────────
// NOTE: Kong Admin API does NOT accept ?workspace= query param.
// Workspace-scoped requests use path format: /workspaces/{ws}/routes, etc.
// cont system is single-workspace (default), so we use direct paths.
const isWorkspaceRequest = (url: string) => url.startsWith('/workspaces/')

kongClient.interceptors.request.use(config => {
  // Only add workspace query param for Kong Manager endpoints, not Admin API
  // Admin API uses path-based workspace scoping: /workspaces/{name}/...
  const ws = getKongWorkspace()
  if (ws && !isWorkspaceRequest(config.url || '')) {
    // For Admin API (services, routes, plugins, consumers...), no workspace query param needed
    // Kong Admin API v1 uses: /routes, /services, /plugins (not workspace-scoped by default)
  }
  return config
})
kongClient.interceptors.response.use(r => r, err => {
  if (err.response?.status === 401) { clearAuth(); window.location.href = '/login' }
  return Promise.reject(err)
})

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
export interface KongService {
  id?: string; name: string; url?: string; host: string; port: number
  path?: string; protocol?: string; retries?: number; connect_timeout?: number
  read_timeout?: number; write_timeout?: number; enabled?: boolean; created_at?: number
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
export interface AuthGroup { id?: string; name: string; label: string; description?: string; permissions?: PermissionEntry[]; created_at?: number }
export interface Workspace { id: string; name: string; label: string; description?: string; kong_workspace_id?: string; created_at?: number; group_ids?: string[] }
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

export const getGroups = () => analyticsClient.get<AuthGroup[]>('/groups').then(r => r.data)
export const createGroup = (data: Partial<AuthGroup>) => analyticsClient.post<AuthGroup>('/groups', data).then(r => r.data)
export const updateGroup = (id: string, data: Partial<AuthGroup>) => analyticsClient.patch<AuthGroup>(`/groups/${id}`, data).then(r => r.data)
export const deleteGroup = (id: string) => analyticsClient.delete(`/groups/${id}`)
export const getGroupMembers = (id: string) => analyticsClient.get<{members: {id:string;username:string;display_name:string;email:string;role:string}[]}>(`/groups/${id}/members`).then(r => r.data)
export const setGroupMembers = (id: string, userIds: string[]) => analyticsClient.put(`/groups/${id}/members`, {user_ids: userIds}).then(r => r.data)

export const listWorkspaces = () => analyticsClient.get<Workspace[]>('/workspaces').then(r => r.data)
export const listMyWorkspaces = () => analyticsClient.get<Workspace[]>('/workspaces/mine').then(r => r.data)
export const createWorkspace = (data: { name: string; label: string; description?: string }) =>
  analyticsClient.post<Workspace>('/workspaces', data).then(r => r.data)
export const updateWorkspace = (id: string, data: { label?: string; description?: string }) =>
  analyticsClient.patch<Workspace>(`/workspaces/${id}`, data).then(r => r.data)
export const deleteWorkspace = (id: string) => analyticsClient.delete(`/workspaces/${id}`)

export const listResources = () => analyticsClient.get<{ resources: Resource[] }>('/resources').then(r => r.data?.resources ?? [])

export const getAuditLogs = () => analyticsClient.get<AuditEntry[]>('/audit').then(r => r.data)

export const getAlertRules = () => analyticsClient.get('/alerts/rules').then(r => r.data)
export const createAlertRule = (payload: Record<string, unknown>) => analyticsClient.post('/alerts/rules', payload).then(r => r.data)
export const updateAlertRule = (id: string, payload: Record<string, unknown>) => analyticsClient.patch(`/alerts/rules/${id}`, payload).then(r => r.data)
export const deleteAlertRule = (id: string) => analyticsClient.delete(`/alerts/rules/${id}`)

export const getApiKeyRequests = () => analyticsClient.get('/apikeys/requests').then(r => r.data)
export const approveApiKeyRequest = (id: string, expiresAt?: string) =>
  analyticsClient.post(`/apikeys/approve/${id}`, expiresAt ? { expires_at: expiresAt } : {}).then(r => r.data)
export const rejectApiKeyRequest = (id: string) => analyticsClient.post(`/apikeys/reject/${id}`).then(r => r.data)

export const getHealthServices = () => analyticsClient.get('/health/services').then(r => r.data)
export const triggerHealthCheck = () => analyticsClient.post('/health/check', {}).then(r => r.data)

export const getStatsOverview = () => analyticsClient.get('/stats/overview').then(r => r.data)

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

export const getStatus = () => kongClient.get<KongStatus>(wsPrefix('/status')).then(r => r.data)
export const getInfo = () => kongClient.get<KongInfo>(wsPrefix('/')).then(r => r.data)
export const getMetrics = () => kongClient.get(wsPrefix('/metrics'), { transformResponse: [(d) => d] }).then(r => {
  const text = typeof r.data === 'string' ? r.data : String(r.data)
  return parsePromMetrics(text)
})

// ── Kong Admin API ────────────────────────────────────────
export const api = {
  listServices: () => kongClient.get<{ data: KongService[] }>(wsPrefix('/services')).then(r => r.data),
  getService: (id: string) => kongClient.get<KongService>(wsPrefix(`/services/${id}`)).then(r => r.data),
  createService: (data: Partial<KongService>) => kongClient.post<KongService>(wsPrefix('/services'), data).then(r => r.data),
  updateService: (id: string, data: Partial<KongService>) => kongClient.patch<KongService>(wsPrefix(`/services/${id}`), data).then(r => r.data),
  deleteService: (id: string) => kongClient.delete(wsPrefix(`/services/${id}`)),

  listRoutes: () => kongClient.get<{ data: KongRoute[] }>(wsPrefix('/routes')).then(r => r.data),
  getRoute: (id: string) => kongClient.get<KongRoute>(wsPrefix(`/routes/${id}`)).then(r => r.data),
  createRoute: (data: Partial<KongRoute>) => kongClient.post<KongRoute>(wsPrefix('/routes'), data).then(r => r.data),
  updateRoute: (id: string, data: Partial<KongRoute>) => kongClient.patch<KongRoute>(wsPrefix(`/routes/${id}`), data).then(r => r.data),
  deleteRoute: (id: string) => kongClient.delete(wsPrefix(`/routes/${id}`)),

  listPlugins: () => kongClient.get<{ data: KongPlugin[] }>(wsPrefix('/plugins')).then(r => r.data),
  getPlugin: (id: string) => kongClient.get<KongPlugin>(wsPrefix(`/plugins/${id}`)).then(r => r.data),
  createPlugin: (data: Partial<KongPlugin>) => kongClient.post<KongPlugin>(wsPrefix('/plugins'), data).then(r => r.data),
  updatePlugin: (id: string, data: Partial<KongPlugin>) => kongClient.patch<KongPlugin>(wsPrefix(`/plugins/${id}`), data).then(r => r.data),
  deletePlugin: (id: string) => kongClient.delete(wsPrefix(`/plugins/${id}`)),

  listConsumers: () => kongClient.get<{ data: KongConsumer[] }>(wsPrefix('/consumers')).then(r => r.data),
  createConsumer: (data: Partial<KongConsumer>) => kongClient.post<KongConsumer>(wsPrefix('/consumers'), data).then(r => r.data),
  updateConsumer: (id: string, data: Partial<KongConsumer>) => kongClient.patch<KongConsumer>(wsPrefix(`/consumers/${id}`), data).then(r => r.data),
  deleteConsumer: (id: string) => kongClient.delete(wsPrefix(`/consumers/${id}`)),

  listJWTCredentials: (consumerId: string) =>
    kongClient.get<{ data: unknown[] }>(wsPrefix(`/consumers/${consumerId}/jwt`)).then(r => r.data),
  createJWTCredential: (consumerId: string, data: unknown) =>
    kongClient.post(wsPrefix(`/consumers/${consumerId}/jwt`), data).then(r => r.data),
  deleteJWTCredential: (consumerId: string, credentialId: string) =>
    kongClient.delete(wsPrefix(`/consumers/${consumerId}/jwt/${credentialId}`)),

  listKeyAuthCredentials: (consumerId: string) =>
    kongClient.get<{ data: unknown[] }>(wsPrefix(`/consumers/${consumerId}/key-auth`)).then(r => r.data),
  createKeyAuthCredential: (consumerId: string, data?: unknown) =>
    kongClient.post(wsPrefix(`/consumers/${consumerId}/key-auth`), data || {}).then(r => r.data),
  deleteKeyAuthCredential: (consumerId: string, credentialId: string) =>
    kongClient.delete(wsPrefix(`/consumers/${consumerId}/key-auth/${credentialId}`)),
}

export default api

import { useState, useEffect } from 'react'
import { Form, Input, Button, Alert, Divider, message, Spin } from 'antd'
import { UserOutlined, LockOutlined, GoogleOutlined, SafetyCertificateOutlined } from '@ant-design/icons'
import { useNavigate, useLocation } from 'react-router-dom'
import axios from 'axios'

const API_BASE = import.meta.env.VITE_API_BASE || '/api'
import { setToken, setUserPerms } from '../api/kong'

interface LoginValues {
  username: string
  password: string
}

// OAuth2/OIDC Provider type
export type SSOProvider = 'mock' | 'ldap' | 'oauth2'

export interface SSOLoginResponse {
  token: string
  user: {
    id: string
    username: string
    display_name: string
    email: string
    groups: Array<{ name: string; label: string }>
    created_at: string
  }
  permissions: Record<string, { mode: string; level: number }>
}

// OAuth2 Provider config from backend
export interface OAuth2Provider {
  provider: string
  client_id: string
  issuer_url: string
  authorization_url: string
  token_url: string
  userinfo_url: string
  scopes: string
  enabled: boolean
}

// SSO Service interface
export interface ISSOService {
  provider: SSOProvider
  login(): Promise<SSOLoginResponse>
}

// Mock SSO Service
class MockSSOService implements ISSOService {
  provider: SSOProvider = 'mock'

  async login(): Promise<SSOLoginResponse> {
    const res = await axios.post<SSOLoginResponse>(`${API_BASE}/auth/sso/mock`, {})
    return res.data
  }
}

// LDAP Service stub
export class LDAPService implements ISSOService {
  provider: SSOProvider = 'ldap'

  async login(): Promise<SSOLoginResponse> {
    throw new Error('LDAP login not implemented — configure LDAP server settings first')
  }
}

// OAuth2 Service — real redirect-based OAuth2/OIDC flow
export class OAuth2Service implements ISSOService {
  provider: SSOProvider = 'oauth2'
  private providerName: string

  constructor(providerName = 'google') {
    this.providerName = providerName
  }

  async login(): Promise<SSOLoginResponse> {
    // Redirect to backend OAuth initiation endpoint
    // The backend will redirect to the IdP, then back to /auth/:provider/callback
    // which will redirect back here with ?token=<jwt>
    const callbackURI = encodeURIComponent(window.location.origin + '/login')
    window.location.href = `${API_BASE}/auth/${this.providerName}?redirect_uri=${callbackURI}`
    // This function returns a promise that will be resolved by the OAuth callback handler
    return new Promise((resolve, reject) => {
      // Store resolver so the callback can resolve this promise
      (window as any).__oauthResolver = { resolve, reject }
    })
  }
}

// OAuth callback handler — called on page load if token is in URL
export function handleOAuthCallback(): string | null {
  const params = new URLSearchParams(window.location.search)
  const token = params.get('token')
  if (token) {
    // Clean URL
    window.history.replaceState({}, '', window.location.pathname)
    return token
  }
  return null
}

// Factory: get SSO service instance by provider type
export function getSSOService(provider: SSOProvider, providerName?: string): ISSOService {
  switch (provider) {
    case 'mock':
      return new MockSSOService()
    case 'ldap':
      return new LDAPService()
    case 'oauth2':
      return new OAuth2Service(providerName || 'google')
    default:
      return new MockSSOService()
  }
}

// SSO button config
export const SSO_PROVIDERS: Array<{
  provider: SSOProvider
  label: string
  icon: React.ReactNode
  providerName?: string
}> = [
  {
    provider: 'mock',
    label: 'SSO 登入 (Mock)',
    icon: <SafetyCertificateOutlined />
  }
  // OAuth2 buttons are dynamically added from /auth/oauth/providers
]

export default function Login() {
  const navigate = useNavigate()
  const location = useLocation()
  const [error, setError] = useState<string>('')
  const [loading, setLoading] = useState(false)
  const [ssoLoading, setSsoLoading] = useState(false)
  const [ssoError, setSsoError] = useState<string>('')
  const [oauthProviders, setOauthProviders] = useState<OAuth2Provider[]>([])
  const [loadingProviders, setLoadingProviders] = useState(true)

  // Fetch available OAuth providers on mount
  useEffect(() => {
    axios.get(`${API_BASE}/auth/oauth/providers`)
      .then(res => {
        setOauthProviders((res.data || []).filter((p: OAuth2Provider) => p.enabled))
        setLoadingProviders(false)
      })
      .catch(() => setLoadingProviders(false))
  }, [])

  // Handle OAuth callback — if token in URL, store it and navigate
  useEffect(() => {
    const token = handleOAuthCallback()
    if (token) {
      setToken(token)
      // Fetch permissions and user info
      axios.get(`${API_BASE}/auth/me`, {
        headers: { Authorization: `Bearer ${token}` }
      }).then(res => {
        setUserPerms(res.data.permissions || {})
        message.success(`SSO 登入成功：${res.data.username}`)
        navigate('/dashboard')
      }).catch(() => {
        // Token exists but /auth/me failed — still navigate
        setUserPerms({})
        navigate('/dashboard')
      })
    }
  }, [location])

  const onFinish = async (values: LoginValues) => {
    setLoading(true)
    setError('')
    try {
      const res = await axios.post(`${API_BASE}/auth/login`, {
        username: values.username,
        password: values.password
      })
      const { token, permissions } = res.data
      setToken(token)
      setUserPerms(permissions || {})
      navigate('/dashboard')
    } catch (err: any) {
      const msg = err?.response?.data?.error || '登入失敗'
      setError(msg)
    } finally {
      setLoading(false)
    }
  }

  const handleSSOLogin = async (provider: SSOProvider, providerName?: string) => {
    setSsoLoading(true)
    setSsoError('')
    try {
      const service = getSSOService(provider, providerName)
      const result = await service.login()
      setToken(result.token)
      setUserPerms(result.permissions || {})
      message.success(`SSO 登入成功：${result.user.display_name}`)
      navigate('/dashboard')
    } catch (err: any) {
      const msg = err?.message || 'SSO 登入失敗'
      setSsoError(msg)
    } finally {
      setSsoLoading(false)
    }
  }

  return (
    <div style={{
      minHeight: '100vh',
      background: 'var(--primary)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
    }}>
      <div style={{
        background: 'var(--secondary)',
        border: '1px solid var(--accent)',
        borderRadius: 12,
        padding: '40px 32px',
        width: 360,
      }}>
        <div style={{ textAlign: 'center', marginBottom: 32 }}>
          <div style={{
            width: 56,
            height: 56,
            borderRadius: 12,
            background: 'linear-gradient(135deg, var(--highlight), var(--accent))',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: 24,
            fontWeight: 700,
            margin: '0 auto 12px',
          }}>
            K
          </div>
          <h2 style={{ color: 'var(--text)', margin: 0 }}>Cont</h2>
          <p style={{ color: 'var(--muted)', fontSize: 13, margin: '4px 0 0' }}>
            API Gateway Management Platform
          </p>
        </div>

        {error && (
          <Alert type="error" message={error} style={{ marginBottom: 16 }} />
        )}

        <Form
          name="login"
          onFinish={onFinish}
          layout="vertical"
          requiredMark={false}
        >
          <Form.Item
            name="username"
            rules={[{ required: true, message: 'Please enter username' }]}
          >
            <Input
              prefix={<UserOutlined style={{ color: 'var(--muted)' }} />}
              placeholder="Username"
              size="large"
            />
          </Form.Item>

          <Form.Item
            name="password"
            rules={[{ required: true, message: 'Please enter password' }]}
          >
            <Input.Password
              prefix={<LockOutlined style={{ color: 'var(--muted)' }} />}
              placeholder="Password"
              size="large"
            />
          </Form.Item>

          <Form.Item style={{ marginBottom: 0 }}>
            <Button
              type="primary"
              htmlType="submit"
              size="large"
              loading={loading}
              block
            >
              Sign In
            </Button>
          </Form.Item>
        </Form>

        <Divider style={{ margin: '24px 0 16px', color: 'var(--muted)', borderColor: 'var(--accent)' }}>
          <span style={{ fontSize: 12, padding: '0 8px' }}>or</span>
        </Divider>

        {ssoError && (
          <Alert type="error" message={ssoError} style={{ marginBottom: 12 }} />
        )}

        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {SSO_PROVIDERS.map(({ provider, label, icon }) => (
            <Button
              key={provider}
              size="large"
              icon={icon}
              loading={ssoLoading}
              onClick={() => handleSSOLogin(provider)}
              block
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                gap: 8,
              }}
            >
              {label}
            </Button>
          ))}

          {/* Dynamically loaded OAuth2 providers */}
          {loadingProviders ? (
            <div style={{ textAlign: 'center', padding: '8px 0' }}>
              <Spin size="small" />
            </div>
          ) : oauthProviders.map(p => (
            <Button
              key={p.provider}
              size="large"
              icon={<GoogleOutlined />}
              loading={ssoLoading}
              onClick={() => handleSSOLogin('oauth2', p.provider)}
              block
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                gap: 8,
              }}
            >
              {p.provider.charAt(0).toUpperCase() + p.provider.slice(1)} 登入
            </Button>
          ))}
        </div>

        <div style={{ marginTop: 24, fontSize: 12, color: 'var(--muted)', textAlign: 'center' }}>
          <p>Demo: admin / admin123 (full access) or user / user123 (limited)</p>
        </div>
      </div>
    </div>
  )
}
import { useState } from 'react'
import { Form, Input, Button, Alert, Divider, message } from 'antd'
import { UserOutlined, LockOutlined, SafetyCertificateOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import axios from 'axios'

const API_BASE = import.meta.env.VITE_API_BASE || '/api'
import { setToken, setUserPerms } from '../api/kong'

interface LoginValues {
  username: string
  password: string
}

// SSO Provider types (extensible interface for future LDAP/OAuth)
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

// SSO Service interface — extend this for real LDAP/OAuth providers
export interface ISSOService {
  provider: SSOProvider
  login(): Promise<SSOLoginResponse>
}

// Mock SSO Service — calls /api/auth/sso/mock backend endpoint
class MockSSOService implements ISSOService {
  provider: SSOProvider = 'mock'

  async login(): Promise<SSOLoginResponse> {
    const res = await axios.post<SSOLoginResponse>(`${API_BASE}/auth/sso/mock`, {})
    return res.data
  }
}

// LDAP Service stub — placeholder for future LDAP integration
export class LDAPService implements ISSOService {
  provider: SSOProvider = 'ldap'

  async login(): Promise<SSOLoginResponse> {
    // TODO: Implement real LDAP bind + user lookup
    // 1. Connect to LDAP server
    // 2. Bind with service account or user credentials
    // 3. Search for user DN
    // 4. Verify password via bind
    // 5. Map LDAP groups to local permissions
    // 6. Return SSOLoginResponse
    throw new Error('LDAP login not implemented — configure LDAP server settings first')
  }
}

// OAuth2 Service stub — placeholder for future OAuth2/OIDC integration
export class OAuth2Service implements ISSOService {
  provider: SSOProvider = 'oauth2'

  async login(): Promise<SSOLoginResponse> {
    // TODO: Implement real OAuth2 flow
    // 1. Redirect to IdP authorization endpoint
    // 2. Handle callback with authorization code
    // 3. Exchange code for access token
    // 4. Fetch user info from IdP userinfo endpoint
    // 5. Map OAuth groups/roles to local permissions
    // 6. Return SSOLoginResponse
    throw new Error('OAuth2 login not implemented — configure OAuth2 client settings first')
  }
}

// Factory: get SSO service instance by provider type
export function getSSOService(provider: SSOProvider): ISSOService {
  switch (provider) {
    case 'mock':
      return new MockSSOService()
    case 'ldap':
      return new LDAPService()
    case 'oauth2':
      return new OAuth2Service()
    default:
      return new MockSSOService()
  }
}

// SSO button config — label + icon per provider
export const SSO_PROVIDERS: Array<{
  provider: SSOProvider
  label: string
  icon: React.ReactNode
}> = [
  {
    provider: 'mock',
    label: 'SSO 登入 (Mock)',
    icon: <SafetyCertificateOutlined />
  }
  // Future providers (commented out, ready to enable):
  // { provider: 'ldap',   label: 'LDAP 登入',   icon: <KeyOutlined /> },
  // { provider: 'oauth2', label: 'OAuth 2.0',  icon: <GoogleOutlined /> },
]

export default function Login() {
  const navigate = useNavigate()
  const [error, setError] = useState<string>('')
  const [loading, setLoading] = useState(false)
  const [ssoLoading, setSsoLoading] = useState(false)
  const [ssoError, setSsoError] = useState<string>('')

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

  const handleSSOLogin = async (provider: SSOProvider) => {
    setSsoLoading(true)
    setSsoError('')
    try {
      const service = getSSOService(provider)
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
          <h2 style={{ color: 'var(--text)', margin: 0 }}>kgo</h2>
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

        {/* SSO Section */}
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
        </div>

        <div style={{ marginTop: 24, fontSize: 12, color: 'var(--muted)', textAlign: 'center' }}>
          <p>Demo: admin / admin123 (full access) or user / user123 (limited)</p>
        </div>
      </div>
    </div>
  )
}
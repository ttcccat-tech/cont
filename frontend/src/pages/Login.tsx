import { useState, useEffect } from 'react'
import { Form, Input, Button, Alert, Divider, message, Spin } from 'antd'
import { UserOutlined, LockOutlined, GoogleOutlined, SafetyCertificateOutlined, MailOutlined } from '@ant-design/icons'
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

  // Registration state
  const [isRegister, setIsRegister] = useState(false)
  const [regStep, setRegStep] = useState<1 | 2>(1) // 1=send-otp, 2=verify-otp
  const [regEmail, setRegEmail] = useState('')
  const [regLoading, setRegLoading] = useState(false)
  const [regCountdown, setRegCountdown] = useState(0)
  const [regError, setRegError] = useState('')
  const [showForgotPassword, setShowForgotPassword] = useState(false)
  const [forgotStep, setForgotStep] = useState<'send' | 'verify' | 'done'>('send')
  const [forgotEmail, setForgotEmail] = useState('')
  const [forgotCode, setForgotCode] = useState('')
  const [forgotLoading, setForgotLoading] = useState(false)
  const [forgotCountdown, setForgotCountdown] = useState(0)
  const [forgotError, setForgotError] = useState('')

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

  // ── Registration flow ─────────────────────────────────────────────────────

  const handleSendOTP = async (values: { email: string }) => {
    setRegLoading(true)
    setRegError('')
    try {
      await axios.post(`${API_BASE}/auth/register/send-otp`, {
        email: values.email,
        purpose: 'register',
      })
      setRegEmail(values.email)
      setRegStep(2)
      setRegCountdown(600) // 10 minutes
      message.success('驗證碼已發送至您的信箱')
    } catch (err: any) {
      setRegError(err?.response?.data?.error || '發送驗證碼失敗')
    } finally {
      setRegLoading(false)
    }
  }

  const handleVerifyOTP = async (values: { code: string; username: string; password: string; display_name?: string }) => {
    setRegLoading(true)
    setRegError('')
    try {
      const res = await axios.post(`${API_BASE}/auth/register/verify-otp`, {
        email: regEmail,
        code: values.code,
        purpose: 'register',
        username: values.username,
        password: values.password,
        display_name: values.display_name || values.username,
      })
      const { token } = res.data
      setToken(token)
      setUserPerms(res.data.permissions || {})
      message.success('帳戶註冊成功！即將跳轉至管理平台...')
      setTimeout(() => navigate('/dashboard'), 1500)
    } catch (err: any) {
      setRegError(err?.response?.data?.error || '驗證失敗，請確認驗證碼是否正確')
    } finally {
      setRegLoading(false)
    }
  }

  const handleResendOTP = async () => {
    setRegCountdown(600)
    setRegError('')
    try {
      await axios.post(`${API_BASE}/auth/register/send-otp`, {
        email: regEmail,
        purpose: 'register',
      })
      message.success('驗證碼已重新發送')
    } catch (err: any) {
      setRegError(err?.response?.data?.error || '發送失敗')
    }
  }

  const switchToRegister = () => {
    setIsRegister(true)
    setRegStep(1)
    setRegEmail('')
    setRegError('')
  }

  const switchToLogin = () => {
    setIsRegister(false)
    setRegStep(1)
    setRegEmail('')
    setRegError('')
  }

  if (isRegister && regStep === 2) {
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
            <h2 style={{ color: 'var(--text)', margin: 0 }}>驗證您的信箱</h2>
            <p style={{ color: 'var(--muted)', fontSize: 13, margin: '4px 0 0' }}>
              已發送驗證碼至 {regEmail}
            </p>
          </div>

          {regError && <Alert type="error" message={regError} style={{ marginBottom: 16 }} />}

          <Form
            onFinish={handleVerifyOTP}
            layout="vertical"
            requiredMark={false}
          >
            <Form.Item
              name="code"
              rules={[{ required: true, message: '請輸入驗證碼' }, { len: 6, message: '驗證碼為 6 位數' }]}
            >
              <Input
                placeholder="6 位驗證碼"
                size="large"
                maxLength={6}
                style={{ textAlign: 'center', letterSpacing: 4, fontSize: 18 }}
              />
            </Form.Item>

            <Form.Item
              name="username"
              rules={[{ required: true, message: '請輸入帳號' }, { min: 3, message: '帳號至少 3 個字元' }]}
            >
              <Input prefix={<UserOutlined />} placeholder="帳號" size="large" />
            </Form.Item>

            <Form.Item
              name="password"
              rules={[{ required: true, message: '請輸入密碼' }, { min: 6, message: '密碼至少 6 位' }]}
            >
              <Input.Password prefix={<LockOutlined />} placeholder="密碼" size="large" />
            </Form.Item>

            <Form.Item
              name="display_name"
            >
              <Input prefix={<UserOutlined />} placeholder="顯示名稱（選填）" size="large" />
            </Form.Item>

            <Form.Item style={{ marginBottom: 0 }}>
              <Button type="primary" htmlType="submit" size="large" loading={regLoading} block>
                註冊並登入
              </Button>
            </Form.Item>
          </Form>

          <div style={{ marginTop: 16, display: 'flex', justifyContent: 'space-between', fontSize: 12 }}>
            <Button type="link" size="small" onClick={() => setRegStep(1)} style={{ color: 'var(--muted)' }}>
              重新輸入信箱
            </Button>
            <Button type="link" size="small" onClick={switchToLogin} style={{ color: 'var(--muted)' }}>
              返回登入
            </Button>
          </div>
        </div>
      </div>
    )
  }

  if (isRegister) {
    // Step 1: enter email to receive OTP
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
            <h2 style={{ color: 'var(--text)', margin: 0 }}>建立新帳戶</h2>
            <p style={{ color: 'var(--muted)', fontSize: 13, margin: '4px 0 0' }}>
              輸入 email 收取驗證碼
            </p>
          </div>

          {regError && <Alert type="error" message={regError} style={{ marginBottom: 16 }} />}

          <Form
            onFinish={handleSendOTP}
            layout="vertical"
            requiredMark={false}
          >
            <Form.Item
              name="email"
              rules={[{ required: true, message: '請輸入 email' }, { type: 'email', message: '格式不正確' }]}
            >
              <Input
                prefix={<MailOutlined style={{ color: 'var(--muted)' }} />}
                placeholder="電子郵件"
                size="large"
                type="email"
              />
            </Form.Item>

            <Form.Item style={{ marginBottom: 0 }}>
              <Button type="primary" htmlType="submit" size="large" loading={regLoading} block>
                發送驗證碼
              </Button>
            </Form.Item>
          </Form>

          <div style={{ marginTop: 16, textAlign: 'center' }}>
            <Button type="link" onClick={switchToLogin} style={{ color: 'var(--muted)', fontSize: 12 }}>
              已有帳戶？立即登入
            </Button>
          </div>
        </div>
      </div>
    )
  }

  // Login form (default)
  if (showForgotPassword) {
    if (forgotStep === 'done') {
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
            textAlign: 'center',
          }}>
            <div style={{ fontSize: 48, marginBottom: 16 }}>✅</div>
            <h2 style={{ color: 'var(--text)', marginBottom: 8 }}>密碼已重設</h2>
            <p style={{ color: 'var(--muted)', marginBottom: 24 }}>請使用新密碼登入</p>
            <Button type="primary" size="large" block onClick={() => {
              setShowForgotPassword(false)
              setForgotStep('send')
              setForgotEmail('')
              setForgotCode('')
            }}>
              返回登入
            </Button>
          </div>
        </div>
      )
    }

    if (forgotStep === 'verify') {
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
                width: 56, height: 56, borderRadius: 12,
                background: 'linear-gradient(135deg, var(--highlight), var(--accent))',
                display: 'flex', alignItems: 'center', justifyContent: 'center',
                fontSize: 24, fontWeight: 700, margin: '0 auto 12px',
              }}>K</div>
              <h2 style={{ color: 'var(--text)', margin: 0 }}>Cont</h2>
              <p style={{ color: 'var(--muted)', marginTop: 8 }}>輸入驗證碼</p>
            </div>

            {forgotError && <Alert type="error" message={forgotError} style={{ marginBottom: 12 }} />}

            <Form
              onFinish={async (values) => {
                setForgotLoading(true)
                setForgotError('')
                try {
                  await axios.post(`${API_BASE}/auth/password-reset/verify`, {
                    email: forgotEmail,
                    code: values.code,
                    new_password: values.password,
                  })
                  setForgotStep('done')
                } catch (err: any) {
                  setForgotError(err.response?.data?.error || '驗證失敗')
                } finally {
                  setForgotLoading(false)
                }
              }}
              layout="vertical"
              requiredMark={false}
            >
              <Form.Item label="驗證碼" name="code" rules={[{ required: true, message: '請輸入驗證碼' }]}>
                <Input.OTP size="large" />
              </Form.Item>
              <Form.Item label="新密碼" name="password"
                rules={[{ required: true, message: '請輸入新密碼' }, { min: 6, message: '密碼至少 6 位' }]}>
                <Input.Password size="large" placeholder="新密碼" />
              </Form.Item>
              <Form.Item style={{ marginBottom: 0 }}>
                <Button type="primary" htmlType="submit" size="large" loading={forgotLoading} block>
                  重設密碼
                </Button>
              </Form.Item>
            </Form>

            <div style={{ marginTop: 16, textAlign: 'center' }}>
              <Button type="link" size="small" onClick={() => setForgotStep('send')} style={{ color: 'var(--muted)' }}>
                重新發送驗證碼
              </Button>
              {forgotCountdown > 0 && <span style={{ color: 'var(--muted)', marginLeft: 8 }}>({forgotCountdown}s)</span>}
            </div>
          </div>
        </div>
      )
    }

    // forgotStep === 'send'
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
              width: 56, height: 56, borderRadius: 12,
              background: 'linear-gradient(135deg, var(--highlight), var(--accent))',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              fontSize: 24, fontWeight: 700, margin: '0 auto 12px',
            }}>K</div>
            <h2 style={{ color: 'var(--text)', margin: 0 }}>Cont</h2>
            <p style={{ color: 'var(--muted)', marginTop: 8 }}>輸入註冊信箱以重設密碼</p>
          </div>

          {forgotError && <Alert type="error" message={forgotError} style={{ marginBottom: 12 }} />}

          <Form
            onFinish={async (values) => {
              setForgotLoading(true)
              setForgotError('')
              try {
                await axios.post(`${API_BASE}/auth/password-reset/send`, { email: values.email })
                setForgotEmail(values.email)
                setForgotStep('verify')
                setForgotCountdown(60)
                const timer = setInterval(() => {
                  setForgotCountdown(c => {
                    if (c <= 1) { clearInterval(timer); return 0 }
                    return c - 1
                  })
                }, 1000)
              } catch (err: any) {
                setForgotError(err.response?.data?.error || '發送失敗')
              } finally {
                setForgotLoading(false)
              }
            }}
            layout="vertical"
            requiredMark={false}
          >
            <Form.Item label="電子郵件" name="email"
              rules={[{ required: true, message: '請輸入信箱' }, { type: 'email', message: '請輸入有效信箱' }]}>
              <Input prefix={<MailOutlined />} size="large" placeholder="admin@cont.dev" />
            </Form.Item>
            <Form.Item style={{ marginBottom: 0 }}>
              <Button type="primary" htmlType="submit" size="large" loading={forgotLoading} block>
                發送驗證碼
              </Button>
            </Form.Item>
          </Form>

          <div style={{ marginTop: 16, textAlign: 'center' }}>
            <Button type="link" size="small" onClick={() => setShowForgotPassword(false)} style={{ color: 'var(--muted)' }}>
              返回登入
            </Button>
          </div>
        </div>
      </div>
    )
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

          <div style={{ textAlign: 'right', marginBottom: 8 }}>
            <Button type="link" size="small" onClick={() => setShowForgotPassword(true)} style={{ color: 'var(--muted)', padding: 0 }}>
              忘記密碼？
            </Button>
          </div>

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
          <p style={{ marginTop: 12 }}>
            还没有帐户？{' '}
            <Button type="link" size="small" onClick={switchToRegister} style={{ color: 'var(--highlight)', padding: 0, height: 'auto', fontSize: 12 }}>
              立即注册
            </Button>
          </p>
        </div>
      </div>
    </div>
  )
}
import { useEffect, useState } from 'react'
import { useLocation } from 'react-router-dom'
import { Card, Form, Input, Switch, Select, Button, Space, Tag, message, Divider, Row, Col, InputNumber, Alert, Tabs, Table, Modal, Popconfirm } from 'antd'
import { SaveOutlined, ReloadOutlined, PlusOutlined, DeleteOutlined, EditOutlined } from '@ant-design/icons'
import BillingPortal from '../components/BillingPortal'
import api, { listOAuthProviders, createOAuthProvider, updateOAuthProvider, deleteOAuthProvider, OAuth2Provider } from '../api/kong'

interface KongConfig {
  // Connection settings
  proxy_listen: string
  admin_listen: string
  // Timeouts
  proxy_connect_timeout: number
  proxy_send_timeout: number
  proxy_read_timeout: number
  admin_send_timeout: number
  admin_read_timeout: number
  // General
  log_level: string
  plugins: string[]
  database: string
  // CORS (set via plugin but also documented here)
  // Request size
  client_max_body_size: string
  // Real IP
  real_ip_header: string
  real_ip_recursive: string
}

const LOG_LEVELS = ['debug', 'info', 'notice', 'warn', 'error', 'crit', 'alert', 'emerg']

const CURRENT_CONFIG: Record<string, any> = {
  'KONG_PROXY_LISTEN': '0.0.0.0:8000',
  'KONG_ADMIN_LISTEN': '0.0.0.0:8001',
  'KONG_LOG_LEVEL': 'info',
  'KONG_CLIENT_MAX_BODY_SIZE': '10m',
}

export default function SettingsPage() {
  const [form] = Form.useForm()
  const [loading, setLoading] = useState(false)
  const [saved, setSaved] = useState(false)
  const location = useLocation()

  // Auto-open billing tab when URL is /billing
  const defaultActiveKey = location.pathname === '/billing' ? 'billing' : 'system'

  useEffect(() => {
    form.setFieldsValue({
      proxy_listen: '0.0.0.0:8000',
      admin_listen: '0.0.0.0:8001',
      proxy_connect_timeout: 60000,
      proxy_send_timeout: 60000,
      proxy_read_timeout: 60000,
      admin_send_timeout: 60000,
      admin_read_timeout: 60000,
      log_level: 'info',
      client_max_body_size: '10m',
      real_ip_header: 'X-Forwarded-For',
      real_ip_recursive: 'on',
    })
  }, [])

  const handleSave = async () => {
    try {
      const values = await form.validateFields()
      setLoading(true)
      // Kong 3.x settings via Admin API node-level config endpoint
      // Note: Many settings require restart. Here we save to localStorage as UI-level prefs.
      localStorage.setItem('cont_settings', JSON.stringify(values))
      message.success('設定已儲存')
      setSaved(true)
      setTimeout(() => setSaved(false), 2000)
    } catch {
      message.error('請檢查輸入格式')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div>
      <div style={{ display:'flex', justifyContent:'space-between', marginBottom:24 }}>
        <h1>系統設定</h1>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => form.resetFields()}>重置</Button>
          <Button type="primary" icon={<SaveOutlined />} onClick={handleSave} loading={loading}>
            {saved ? '已儲存！' : '儲存設定'}
          </Button>
        </Space>
      </div>

      <Tabs defaultActiveKey={defaultActiveKey} items={[
        {
          key: 'system',
          label: '系統設定',
          children: (
            <Row gutter={[16, 16]}>
              {/* Listening Ports */}
              <Col xs={24} lg={12}>
                <Card title="監聽端口" style={{ background:'var(--secondary)', border:'none' }}>
                  <Form form={form} layout="vertical">
                    <Form.Item name="proxy_listen" label="Proxy 監聽（對外流量）" extra="Kong 代理接收客戶端請求的端口">
                      <Input placeholder="0.0.0.0:8000" />
                    </Form.Item>
                    <Form.Item name="admin_listen" label="Admin API 監聽" extra="Kong 管理介面的監聽端口">
                      <Input placeholder="0.0.0.0:8001" />
                    </Form.Item>
                  </Form>
                </Card>
              </Col>

              {/* Timeouts */}
              <Col xs={24} lg={12}>
                <Card title="連線逾時設定（毫秒）" style={{ background:'var(--secondary)', border:'none' }}>
                  <Form form={form} layout="vertical">
                    <Row gutter={8}>
                      <Col span={12}>
                        <Form.Item name="proxy_connect_timeout" label="Proxy 連線逾時" extra="建立連線後，發請求的逾時（ms）">
                          <InputNumber style={{width:'100%'}} min={0} step={1000} />
                        </Form.Item>
                      </Col>
                      <Col span={12}>
                        <Form.Item name="proxy_read_timeout" label="Proxy 讀取逾時">
                          <InputNumber style={{width:'100%'}} min={0} step={1000} />
                        </Form.Item>
                      </Col>
                    </Row>
                    <Row gutter={8}>
                      <Col span={12}>
                        <Form.Item name="proxy_send_timeout" label="Proxy 傳送逾時">
                          <InputNumber style={{width:'100%'}} min={0} step={1000} />
                        </Form.Item>
                      </Col>
                      <Col span={12}>
                        <Form.Item name="admin_read_timeout" label="Admin 讀取逾時">
                          <InputNumber style={{width:'100%'}} min={0} step={1000} />
                        </Form.Item>
                      </Col>
                    </Row>
                  </Form>
                </Card>
              </Col>

              {/* Request& Log */}
              <Col xs={24} lg={12}>
                <Card title="請求限制" style={{ background:'var(--secondary)', border:'none' }}>
                  <Form form={form} layout="vertical">
                    <Form.Item name="client_max_body_size" label="客戶端請求 Body 上限" extra="例如：10m = 10MB，0 = 不限制">
                      <Input placeholder="10m" />
                    </Form.Item>
                  </Form>
                </Card>
              </Col>

              {/* Log & IP */}
              <Col xs={24} lg={12}>
                <Card title="日誌與安全" style={{ background:'var(--secondary)', border:'none' }}>
                  <Form form={form} layout="vertical">
                    <Form.Item name="log_level" label="日誌等級">
                      <Select>
                        {LOG_LEVELS.map(l => <Select.Option key={l} value={l}>{l.toUpperCase()}</Select.Option>)}
                      </Select>
                    </Form.Item>
                    <Form.Item name="real_ip_header" label="真實客戶端 IP Header" extra="前面有代理時用於取得真實 IP">
                      <Input placeholder="X-Forwarded-For" />
                    </Form.Item>
                    <Form.Item name="real_ip_recursive" label="遞迴解析 IP" valuePropName="checked">
                      <Switch checkedChildren="開" unCheckedChildren="關" />
                    </Form.Item>
                  </Form>
                </Card>
              </Col>

              {/* Cont System Info */}
              <Col xs={24}>
                <Card title="系統資訊" style={{ background:'var(--secondary)', border:'none' }}>
                  <Row gutter={[16,8]}>
                    <Col xs={24} sm={8}>
                      <Space direction="vertical">
                        <span style={{color:'var(--muted)', fontSize:12}}>版本</span>
                        <Tag color="green" style={{fontSize:14}}>Cont v2.0</Tag>
                      </Space>
                    </Col>
                    <Col xs={24} sm={8}>
                      <Space direction="vertical">
                        <span style={{color:'var(--muted)', fontSize:12}}>資料庫</span>
                        <Tag color="blue">PostgreSQL 15</Tag>
                      </Space>
                    </Col>
                    <Col xs={24} sm={8}>
                      <Space direction="vertical">
                        <span style={{color:'var(--muted)', fontSize:12}}>API Gateway</span>
                        <Tag color="purple">Cont Proxy</Tag>
                      </Space>
                    </Col>
                  </Row>
                </Card>
              </Col>
            </Row>
          ),
        },
        {
          key: 'billing',
          label: '方案與計費',
          children: <BillingPortal />,
        },
        {
          key: 'oauth',
          label: 'OAuth 設定',
          children: <OAuthSettingsTab />,
        },
      ]} />
    </div>
  )
}

// ── OAuth Provider Settings Tab ──────────────────────────────────────────────

function OAuthSettingsTab() {
  const [providers, setProviders] = useState<OAuth2Provider[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<OAuth2Provider | null>(null)
  const [form] = Form.useForm()

  const loadProviders = async () => {
    setLoading(true)
    try {
      const data = await listOAuthProviders()
      setProviders(data || [])
    } catch {
      message.error('載入 OAuth 設定失敗')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { loadProviders() }, [])

  const handleCreate = () => { setEditing(null); form.resetFields(); setModalOpen(true) }
  const handleEdit = (p: OAuth2Provider) => { setEditing(p); form.setFieldsValue(p); setModalOpen(true) }

  const handleModalOk = async () => {
    try {
      const vals = await form.validateFields()
      if (editing) {
        await updateOAuthProvider(editing.provider, vals)
        message.success('OAuth 設定已更新')
      } else {
        await createOAuthProvider(vals)
        message.success('OAuth Provider 已建立')
      }
      setModalOpen(false)
      loadProviders()
    } catch (e: unknown) {
      const err = e as { response?: { data?: { error?: string } } }
      message.error(err?.response?.data?.error || '儲存失敗')
    }
  }

  const handleDelete = async (provider: string) => {
    try {
      await deleteOAuthProvider(provider)
      message.success('已刪除')
      loadProviders()
    } catch {
      message.error('刪除失敗')
    }
  }

  const columns = [
    { title: 'Provider', dataIndex: 'provider', key: 'provider' },
    { title: 'Client ID', dataIndex: 'client_id', key: 'client_id', render: (v: string) => v || <span style={{color:'var(--muted)'}}>—</span> },
    { title: 'Issuer', dataIndex: 'issuer_url', key: 'issuer_url', render: (v: string) => v || <span style={{color:'var(--muted)'}}>—</span> },
    { title: 'Token URL', dataIndex: 'token_url', key: 'token_url', render: (v: string) => v || <span style={{color:'var(--muted)'}}>—</span> },
    { title: 'Scopes', dataIndex: 'scopes', key: 'scopes', render: (v: string) => v || 'openid email profile' },
    { title: '狀態', dataIndex: 'enabled', key: 'enabled', render: (enabled: boolean) => enabled ? <Tag color="green">啟用</Tag> : <Tag color="default">停用</Tag> },
    {
      title: '操作',
      key: 'action',
      render: (_: unknown, record: OAuth2Provider) => (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => handleEdit(record)}>編輯</Button>
          <Popconfirm title="確定刪除？" onConfirm={() => handleDelete(record.provider)}>
            <Button size="small" danger icon={<DeleteOutlined />}>刪除</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <div style={{display:'flex', justifyContent:'space-between', marginBottom:16}}>
        <div>
          <Alert
            message="OAuth 2.0 / OIDC Single Sign-On 設定"
            description="設定第三方 OAuth2 provider（如 Google）以啟用 SSO。設定完成後，使用者可在登入頁面看到 OAuth 登入選項。"
            type="info"
            showIcon
            style={{marginBottom:16}}
          />
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>新增 Provider</Button>
      </div>

      <Table
        dataSource={providers}
        columns={columns}
        rowKey="provider"
        loading={loading}
        pagination={false}
      />

      <Modal
        title={editing ? '編輯 OAuth Provider' : '新增 OAuth Provider'}
        open={modalOpen}
        onOk={handleModalOk}
        onCancel={() => setModalOpen(false)}
        okText="儲存"
        width={560}
      >
        <Form form={form} layout="vertical" style={{marginTop:16}}>
          <Form.Item name="provider" label="Provider 名稱" rules={[{ required: true }]} extra="英文唯一識別碼，如 google, github">
            <Input placeholder="google" disabled={!!editing} />
          </Form.Item>
          <Form.Item name="client_id" label="Client ID" rules={[{ required: true }]} extra="OAuth provider 頒發的 Client ID">
            <Input placeholder=".apps.googleusercontent.com" />
          </Form.Item>
          <Form.Item name="client_secret" label="Client Secret" extra={editing ? '留空則保持不變' : 'OAuth provider 頒發的 Client Secret'} rules={editing ? [] : [{ required: true }]}>
            <Input.Password placeholder={editing ? '（不變）' : ''} />
          </Form.Item>
          <Form.Item name="issuer_url" label="Issuer URL" extra="OpenID Connect Issuer，如 https://accounts.google.com">
            <Input placeholder="https://accounts.google.com" />
          </Form.Item>
          <Form.Item name="authorization_url" label="Authorization URL" extra="Authorization endpoint">
            <Input placeholder="https://accounts.google.com/o/oauth2/v2/auth" />
          </Form.Item>
          <Form.Item name="token_url" label="Token URL" rules={[{ required: true }]} extra="Token endpoint">
            <Input placeholder="https://oauth2.googleapis.com/token" />
          </Form.Item>
          <Form.Item name="userinfo_url" label="UserInfo URL" extra="UserInfo endpoint（可選，預設使用 OIDC userinfo）">
            <Input placeholder="https://openidconnect.googleapis.com/v1/userinfo" />
          </Form.Item>
          <Form.Item name="scopes" label="Scopes" extra="空白則預設 openid email profile">
            <Input placeholder="openid email profile" />
          </Form.Item>
          <Form.Item name="enabled" label="啟用" valuePropName="checked" initialValue={false}>
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
import { useEffect, useState } from 'react'
import { Table, Button, Space, Tag, message, Modal, Form, Input, Select, Switch, Popconfirm, InputNumber, Divider } from 'antd'
import { PlusOutlined, ReloadOutlined, DeleteOutlined, SettingOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import api, { KongPlugin, KongService, KongRoute, KongConsumer } from '../api/kong'
import { useAuth } from '../context/AuthContext'

// Plugin configuration schemas
interface PluginField {
  name: string
  label: string
  type: 'input' | 'number' | 'switch' | 'select' | 'tags' | 'textarea'
  options?: string[]
  placeholder?: string
  valuePropName?: string
  min?: number
  max?: number
  step?: number
  suffix?: string
  rows?: number
}

const PLUGIN_SCHEMAS: Record<string, PluginField[]> = {
  'rate-limiting': [
    { name: 'minute', label: '每分鐘請求上限', type: 'number', min: 1, placeholder: '100' },
    { name: 'hour', label: '每小時請求上限', type: 'number', min: 1, placeholder: '1000' },
    { name: 'day', label: '每日請求上限', type: 'number', min: 1 },
    { name: 'policy', label: '儲存策略', type: 'select', options: ['local', 'redis', 'cluster'], placeholder: 'local' },
    { name: 'hide_client_headers', label: '隱藏限制相關 Header', type: 'switch', valuePropName: 'checked' },
  ],
  'cors': [
    { name: 'origins', label: '允許來源（陣列）', type: 'tags', placeholder: '* or https://example.com' },
    { name: 'methods', label: '允許 HTTP 方法', type: 'tags', placeholder: 'GET, POST, PUT...', options: ['GET','POST','PUT','DELETE','PATCH','OPTIONS','HEAD'] },
    { name: 'headers', label: '允許 Header', type: 'tags', placeholder: 'Authorization...' },
    { name: 'exposed_headers', label: '暴露 Header（可選）', type: 'tags' },
    { name: 'credentials', label: '允許 credentials', type: 'switch', valuePropName: 'checked' },
    { name: 'max_age', label: '預檢請求快取秒數', type: 'number', min: 0, placeholder: '3600' },
    { name: 'preflight_continue', label: '穿過 OPTIONS 預檢', type: 'switch', valuePropName: 'checked' },
  ],
  'jwt': [
    { name: 'claims_to_verify', label: '驗證聲明（可選）', type: 'select', options: ['exp','nbf','iat','jti'] },
    { name: 'uri_param_names', label: 'URI 參數名（可選）', type: 'tags' },
    { name: 'cookie_names', label: 'Cookie 名稱（可選）', type: 'tags' },
    { name: 'header_names', label: 'Header 名稱（可選）', type: 'tags' },
    { name: 'claims_verify', label: '驗證 JWT 簽名', type: 'switch', valuePropName: 'checked' },
  ],
  'key-auth': [
    { name: 'key_names', label: 'Key 參數名', type: 'tags', placeholder: 'apikey' },
    { name: 'key_in_query', label: '允許出現在 Query String', type: 'switch', valuePropName: 'checked' },
    { name: 'key_in_header', label: '允許出現在 Header', type: 'switch', valuePropName: 'checked' },
    { name: 'hide_credentials', label: '隱藏 Key 不轉發', type: 'switch', valuePropName: 'checked' },
  ],
  'prometheus': [
    { name: 'per_consumer', label: '按消費者統計', type: 'switch', valuePropName: 'checked' },
    { name: 'metrics', label: '額外指標（JSON）', type: 'textarea', rows: 2, placeholder: '[]' },
  ],
  'proxy-cache': [
    { name: 'response_code', label: '快取回應碼', type: 'tags', placeholder: '200, 301, 404' },
    { name: 'request_method', label: '快取請求方法', type: 'tags', placeholder: 'GET, POST' },
    { name: 'content_type', label: '快取 Content-Type', type: 'tags', placeholder: 'application/json' },
    { name: 'cache_ttl', label: '快取 TTL（秒）', type: 'number', min: 0, placeholder: '300' },
    { name: 'strategy', label: '儲存策略', type: 'select', options: ['memory', 'redis'], placeholder: 'memory' },
  ],
  'ip-restriction': [
    { name: 'allow', label: '允許 IP（陣列）', type: 'tags', placeholder: '192.168.1.1' },
    { name: 'deny', label: '拒絕 IP（陣列）', type: 'tags', placeholder: '10.0.0.1' },
  ],
  'acl': [
    { name: 'allow', label: '允許 Groups', type: 'tags', placeholder: 'admin' },
    { name: 'deny', label: '拒絕 Groups', type: 'tags' },
  ],
  'oauth2': [
    { name: 'scopes', label: '授權範圍', type: 'tags', placeholder: 'read write' },
    { name: 'mandatory_scope', label: '必填授權範圍', type: 'switch', valuePropName: 'checked' },
    { name: 'token_expiration', label: 'Token 過期秒數', type: 'number', min: 0, placeholder: '7200' },
    { name: 'authorization_code_expiration', label: '授權碼過期秒數', type: 'number', min: 0, placeholder: '300' },
    { name: 'enable_authorization_code', label: '啟用授權碼模式', type: 'switch', valuePropName: 'checked' },
    { name: 'enable_implicit_grant', label: '啟用隱式授權', type: 'switch', valuePropName: 'checked' },
    { name: 'enable_client_credentials', label: '啟用客戶端憑證', type: 'switch', valuePropName: 'checked' },
    { name: 'enable_password_grant', label: '啟用密碼授權', type: 'switch', valuePropName: 'checked' },
    { name: 'auth_header_name', label: '認證 Header 名', type: 'input', placeholder: 'Authorization' },
    { name: 'token_prefix', label: 'Token 前綴', type: 'input', placeholder: 'Bearer' },
  ],
  'rate-limiting-advanced': [
    { name: 'limit', label: '請求上限（陣列）', type: 'tags', placeholder: '100, 200' },
    { name: 'window_size', label: '時間窗口（秒）', type: 'tags', placeholder: '60, 3600' },
    { name: 'sync_rate', label: 'Redis 同步頻率', type: 'number', min: 0, placeholder: '0' },
    { name: 'strategy', label: '策略', type: 'select', options: ['local', 'redis', 'cluster'], placeholder: 'local' },
    { name: 'hide_client_headers', label: '隱藏限制 Header', type: 'switch', valuePropName: 'checked' },
  ],
  'proxy-cache-advanced': [
    { name: 'response_code', label: '快取回應碼', type: 'tags', placeholder: '200, 301, 404' },
    { name: 'request_method', label: '快取請求方法', type: 'tags', placeholder: 'GET, POST' },
    { name: 'content_type', label: '快取 Content-Type', type: 'tags', placeholder: 'application/json' },
    { name: 'cache_ttl', label: '快取 TTL（秒）', type: 'number', min: 0, placeholder: '300' },
    { name: 'strategy', label: '儲存策略', type: 'select', options: ['memory', 'redis'], placeholder: 'memory' },
    { name: 'memory_cache_threshold', label: '記憶體快取閾值', type: 'number', min: 1, placeholder: '10000' },
  ],
  'gzip': [
    { name: 'enabled', label: '啟用壓縮', type: 'switch', valuePropName: 'checked' },
    { name: 'level', label: '壓縮級別', type: 'number', min: 0, max: 9, placeholder: '5' },
    { name: 'minimum_body_size', label: '最小 body 大小（bytes）', type: 'number', min: 0, placeholder: '12' },
  ],
  'websocket-size-limit': [
    { name: 'size', label: 'WebSocket 訊息大小上限（bytes）', type: 'number', min: 1, placeholder: '65535' },
  ],
  'request-transformer': [
    { name: 'add.headers', label: '新增 Header', type: 'tags', placeholder: 'X-Custom-Header: value' },
    { name: 'add.querystring', label: '新增 Query', type: 'tags', placeholder: 'key=value' },
    { name: 'add.form', label: '新增 Form 參數', type: 'tags', placeholder: 'key=value' },
    { name: 'remove.headers', label: '移除 Header', type: 'tags', placeholder: 'X-Old-Header' },
    { name: 'remove.querystring', label: '移除 Query 參數', type: 'tags', placeholder: 'old_param' },
    { name: 'replace.headers', label: '替換 Header', type: 'tags', placeholder: 'X-Old: X-New' },
  ],
  'response-transformer': [
    { name: 'add.headers', label: '新增 Response Header', type: 'tags', placeholder: 'X-Custom-Header: value' },
    { name: 'add.json', label: '新增 JSON 屬性', type: 'textarea', rows: 2, placeholder: '{"key": "value"}' },
    { name: 'remove.headers', label: '移除 Response Header', type: 'tags', placeholder: 'X-Old-Header' },
  ],
  'correlation-id': [
    { name: 'header_name', label: 'Header 名稱', type: 'input', placeholder: 'Kong-Request-ID' },
    { name: 'generator', label: 'ID 生成方式', type: 'select', options: ['uuid', 'uuid#counter', 'tracking'], placeholder: 'uuid' },
    { name: 'echo_downstream', label: '向下游迴聲', type: 'switch', valuePropName: 'checked' },
  ],
  'session': [
    { name: 'secret', label: 'Session 密鑰', type: 'input', placeholder: 'secret' },
    { name: 'cookie_name', label: 'Cookie 名稱', type: 'input', placeholder: 'session' },
    { name: 'cookie_lifetime', label: 'Cookie 生命週期（秒）', type: 'number', min: 0, placeholder: '3600' },
    { name: 'cookie_renew', label: 'Renew 時間（秒）', type: 'number', min: 0, placeholder: '600' },
    { name: 'cookie_secure', label: 'Secure Cookie', type: 'switch', valuePropName: 'checked' },
    { name: 'cookie_samesite', label: 'SameSite 策略', type: 'select', options: ['Strict', 'Lax', 'Off'], placeholder: 'Lax' },
    { name: 'storage', label: '儲存方式', type: 'select', options: ['cookie', 'redis', 'memcached'], placeholder: 'cookie' },
  ],
  'syslog': [
    { name: 'host', label: 'Syslog 主機', type: 'input', placeholder: '127.0.0.1' },
    { name: 'port', label: 'Syslog 端口', type: 'number', min: 1, max: 65535, placeholder: '514' },
    { name: 'facility', label: 'Syslog facility', type: 'select', options: ['0','1','2','3','4','5','6','7','8','9','10','11','12','13','14','15','16','17','18','19','20','21','22','23'], placeholder: '0' },
    { name: 'log_level', label: '日誌級別', type: 'select', options: ['debug','info','notice','warning','err','crit','alert','emerg'], placeholder: 'info' },
  ],
  'loggly': [
    { name: 'host', label: 'Loggly 主機', type: 'input', placeholder: 'logs-01.loggly.com' },
    { name: 'port', label: 'Loggly 端口', type: 'number', min: 1, max: 65535, placeholder: '514' },
    { name: 'token', label: 'Loggly Token', type: 'input', placeholder: 'your-customer-token' },
  ],
  'bot-detection': [
    { name: 'allow', label: '允許名單', type: 'tags', placeholder: 'friendly-bot' },
    { name: 'deny', label: '拒絕名單', type: 'tags', placeholder: 'malicious-bot' },
  ],
  'ldap-auth': [
    { name: 'ldap_host', label: 'LDAP 主機', type: 'input', placeholder: 'ldap.example.com' },
    { name: 'ldap_port', label: 'LDAP 端口', type: 'number', min: 1, max: 65535, placeholder: '389' },
    { name: 'base_dn', label: 'Base DN', type: 'input', placeholder: 'dc=example,dc=com' },
    { name: 'search_filter', label: '搜尋過濾', type: 'input', placeholder: '(uid=$(username))' },
    { name: 'bind_dn', label: 'Bind DN', type: 'input', placeholder: 'cn=readonly,dc=example,dc=com' },
    { name: 'ssl_verify', label: 'SSL 驗證', type: 'switch', valuePropName: 'checked' },
  ],
}

const ALL_PLUGINS = [
  // Auth plugins
  'jwt', 'key-auth', 'oauth2', 'acl', 'ldap-auth',
  // Security / Rate limiting
  'cors', 'ip-restriction', 'bot-detection', 'rate-limiting', 'rate-limiting-advanced',
  // Caching / Protocol
  'proxy-cache', 'proxy-cache-advanced', 'gzip', 'websocket-size-limit',
  // Observability
  'prometheus', 'datadog', 'zipkin', 'opentelemetry', 'statsd',
  // Logging
  'http-log', 'tcp-log', 'udp-log', 'file-log', 'syslog', 'loggly',
  // Request/Response
  'request-transformer', 'response-transformer', 'correlation-id',
  // Sessions
  'session',
]

function renderField(field: PluginField, value: any, onChange: (val: any) => void) {
  switch (field.type) {
    case 'number':
      return (
        <InputNumber
          style={{ width: '100%' }}
          value={value}
          onChange={onChange}
          min={field.min}
          max={field.max}
          step={field.step}
          placeholder={field.placeholder}
          addonAfter={field.suffix}
        />
      )
    case 'switch':
      return <Switch checked={value} onChange={onChange} />
    case 'tags':
      return (
        <Select
          mode="tags"
          style={{ width: '100%' }}
          value={value || []}
          onChange={onChange}
          placeholder={field.placeholder}
          tokenSeparators={[',']}
        >
          {(field.options || []).map(o => <Select.Option key={o} value={o}>{o}</Select.Option>)}
        </Select>
      )
    case 'select':
      return (
        <Select style={{ width: '100%' }} value={value} onChange={onChange} placeholder={field.placeholder}>
          {(field.options || []).map(o => <Select.Option key={o} value={o}>{o}</Select.Option>)}
        </Select>
      )
    case 'textarea':
      return <Input.TextArea rows={field.rows || 3} value={value} onChange={e => onChange(e.target.value)} placeholder={field.placeholder} />
    default:
      return <Input value={value} onChange={e => onChange(e.target.value)} placeholder={field.placeholder} />
  }
}

export default function PluginsPage() {
  const [plugins, setPlugins] = useState<KongPlugin[]>([])
  const [services, setServices] = useState<KongService[]>([])
  const [routes, setRoutes] = useState<KongRoute[]>([])
  const [consumers, setConsumers] = useState<KongConsumer[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [configOpen, setConfigOpen] = useState(false)
  const [editingPlugin, setEditingPlugin] = useState<KongPlugin | null>(null)
  const [form] = Form.useForm()
  const [configForm] = Form.useForm()
  const [submitting, setSubmitting] = useState(false)
  const { canWrite, canDelete } = useAuth()

  const fetchAll = () => {
    setLoading(true)
    Promise.all([api.listPlugins(), api.listServices(), api.listRoutes(), api.listConsumers()])
      .then(([plugins, services, routes, consumers]) => {
        setPlugins(plugins)
        setServices(services)
        setRoutes(routes)
        setConsumers(consumers)
      })
      .catch(() => message.error('無法連接 Kong Admin API'))
      .finally(() => setLoading(false))
  }

  useEffect(() => { fetchAll() }, [])

  const openCreate = () => {
    form.resetFields()
    form.setFieldsValue({ enabled: true, scope: 'global' })
    setEditingPlugin(null)
    setModalOpen(true)
  }

  const openConfig = (plugin: KongPlugin) => {
    setEditingPlugin(plugin)
    const schema = PLUGIN_SCHEMAS[plugin.name] || []
    if (schema.length === 0) {
      message.info('此插件尚無預設配置欄位，可使用 JSON 編輯')
    }
    configForm.setFieldsValue({ config: plugin.config || {} })
    setConfigOpen(true)
  }

  const handleDelete = async (id: string) => {
    try { await api.deletePlugin(id); message.success('刪除成功'); fetchAll() }
    catch { message.error('刪除失敗') }
  }

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields()
      setSubmitting(true)
      const payload: any = { name: values.name, enabled: values.enabled }
      if (values.config) payload.config = values.config
      if (values.scope === 'service' && values.service_id) payload.service = { id: values.service_id }
      else if (values.scope === 'route' && values.route_id) payload.route = { id: values.route_id }
      else if (values.scope === 'consumer' && values.consumer_id) payload.consumer = { id: values.consumer_id }

      if (editingPlugin?.id) {
        await api.updatePlugin(editingPlugin.id, payload)
      } else {
        await api.createPlugin(payload)
      }
      message.success(editingPlugin ? '更新成功' : '插件建立成功')
      setModalOpen(false); fetchAll()
    } catch (e: any) {
      if (!e.errorFields) {
        const raw = e?.response?.data
        let kongMsg = ''
        if (typeof raw === 'string') kongMsg = raw
        else if (raw?.message) kongMsg = raw.message
        else if (typeof raw === 'object') kongMsg = JSON.stringify(raw)
        message.error('操作失敗: ' + (kongMsg || e.message || ''))
      }
    } finally { setSubmitting(false) }
  }

  const handleConfigSave = async () => {
    if (!editingPlugin?.id) return
    try {
      const { config } = await configForm.validateFields()
      let parsedConfig = config
      if (typeof config === 'string') parsedConfig = JSON.parse(config)
      await api.updatePlugin(editingPlugin.id, { config: parsedConfig })
      message.success('配置已更新')
      setConfigOpen(false); fetchAll()
    } catch (e: any) {
      if (!e.errorFields) message.error('配置更新失敗: ' + (e.message || ''))
    }
  }

  const pluginScopes = (p: KongPlugin) => {
    if (p.service?.id) return 'Service'
    if (p.route?.id) return 'Route'
    if (p.consumer?.id) return 'Consumer'
    return 'Global'
  }

  const columns: ColumnsType<KongPlugin> = [
    { title: '插件名', dataIndex: 'name', key: 'name', render: v => <Tag color="purple">{v}</Tag> },
    { title: '範圍', key: 'scope', render: (_, p) => <Tag color={pluginScopes(p) === 'Global' ? 'gold' : 'cyan'}>{pluginScopes(p)}</Tag> },
    { title: '已啟用', dataIndex: 'enabled', key: 'enabled', render: v => <Tag color={v ? 'green' : 'red'}>{v ? '是' : '否'}</Tag> },
    {
      title: '操作', key: 'action', width: 180,
      render: (_, record) => (
        <Space>
          <Button size="small" icon={<SettingOutlined />} onClick={() => openConfig(record)}>配置</Button>
          {canDelete('plugins') && (
            <Popconfirm title="確認刪除此插件？" onConfirm={() => record.id && handleDelete(record.id)}>
              <Button size="small" danger icon={<DeleteOutlined />}>刪除</Button>
            </Popconfirm>
          )}
        </Space>
      )
    }
  ]

  const selectedPlugin = Form.useWatch('name', form)
  const currentSchema = selectedPlugin ? (PLUGIN_SCHEMAS[selectedPlugin] || []) : []

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <h1>插件管理</h1>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={fetchAll}>刷新</Button>
          {canWrite('plugins') && (
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新增插件</Button>
          )}
        </Space>
      </div>

      <Table
        columns={columns}
        dataSource={plugins as any}
        rowKey="id"
        loading={loading}
        pagination={{ pageSize: 10 }}
        locale={{ emptyText: '暫無插件，點擊「新增插件」開始' }}
      />

      {/* 新增/編輯插件 Modal */}
      <Modal
        title={editingPlugin ? '編輯插件' : '新增插件'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        width={600}
        footer={null}
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }} onFinish={handleSubmit}>
          {editingPlugin ? (
            <Form.Item label="插件名"><Tag color="purple">{editingPlugin.name}</Tag></Form.Item>
          ) : (
            <Form.Item name="name" label="插件名稱" rules={[{ required: true }]}>
              <Select showSearch placeholder="選擇插件" filterOption={(i, o) => (o.children as any).toLowerCase().includes(i.toLowerCase())}>
                {ALL_PLUGINS.map(n => <Select.Option key={n} value={n}>{n}</Select.Option>)}
              </Select>
            </Form.Item>
          )}
          <Form.Item name="scope" label="應用範圍" initialValue="global">
            <Select>
              <Select.Option value="global">全域（全所有流量）</Select.Option>
              <Select.Option value="service">指定服務</Select.Option>
              <Select.Option value="route">指定路由</Select.Option>
              <Select.Option value="consumer">指定消費者</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item noStyle shouldUpdate={(prev, curr) => prev.scope !== curr.scope}>
            {({ getFieldValue }) => {
              const scope = getFieldValue('scope')
              if (scope === 'service') return (
                <Form.Item name="service_id" label="選擇服務"><Select placeholder="選擇服務">{services.map(s => <Select.Option key={s.id} value={s.id}>{s.name} ({s.protocol}://{s.host})</Select.Option>)}</Select></Form.Item>
              )
              if (scope === 'route') return (
                <Form.Item name="route_id" label="選擇路由"><Select placeholder="選擇路由">{routes.map(r => <Select.Option key={r.id} value={r.id}>{r.name || r.id} {r.paths ? r.paths.join(', ') : ''}</Select.Option>)}</Select></Form.Item>
              )
              if (scope === 'consumer') return (
                <Form.Item name="consumer_id" label="選擇消費者"><Select placeholder="選擇消費者">{consumers.map(c => <Select.Option key={c.id} value={c.id}>{c.username}</Select.Option>)}</Select></Form.Item>
              )
              return null
            }}
          </Form.Item>
          <Form.Item name="enabled" label="啟用狀態" valuePropName="checked" initialValue>
            <Switch checkedChildren="開" unCheckedChildren="關" />
          </Form.Item>

          {/* 快速配置欄位（直接在此設定主要參數） */}
          {selectedPlugin && PLUGIN_SCHEMAS[selectedPlugin] && PLUGIN_SCHEMAS[selectedPlugin].length > 0 && (
            <Divider style={{ margin: '12px 0' }}>快速配置</Divider>
          )}
          {selectedPlugin && PLUGIN_SCHEMAS[selectedPlugin] && PLUGIN_SCHEMAS[selectedPlugin].map(field => (
            <Form.Item
              key={field.name}
              name={['config', field.name]}
              label={field.label}
              valuePropName={field.type === 'switch' ? 'checked' : 'value'}
            >
              {field.type === 'number' && (
                <InputNumber style={{ width: '100%' }} min={field.min} max={field.max} step={field.step} placeholder={field.placeholder} addonAfter={field.suffix} />
              )}
              {field.type === 'switch' && <Switch checkedChildren="開" unCheckedChildren="關" />}
              {field.type === 'textarea' && <Input.TextArea rows={field.rows || 3} placeholder={field.placeholder} />}
              {(field.type === 'tags' || field.type === 'select') && (
                <Select style={{ width: '100%' }} mode={field.type === 'tags' ? 'tags' : undefined} placeholder={field.placeholder} tokenSeparators={[',']}>
                  {(field.options || []).map(o => <Select.Option key={o} value={o}>{o}</Select.Option>)}
                </Select>
              )}
              {!field.type || field.type === 'text' && <Input placeholder={field.placeholder} />}
            </Form.Item>
          ))}
          <Form.Item style={{ marginBottom: 0, marginTop: 8 }}>
            <Space style={{ display: 'flex', justifyContent: 'flex-end' }}>
              <Button onClick={() => setModalOpen(false)}>取消</Button>
              <Button type="primary" htmlType="submit" loading={submitting}>
                {editingPlugin ? '更新' : '建立'}
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>

      {/* 插件配置 Modal */}
      <Modal
        title={<>插件配置：<Tag color="purple">{editingPlugin?.name}</Tag></>}
        open={configOpen}
        onOk={handleConfigSave}
        onCancel={() => setConfigOpen(false)}
        width={600}
        okText="儲存配置"
      >
        <Form form={configForm} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item
            name="config"
            label="配置參數"
            extra={
              <span style={{ color: 'var(--muted)', fontSize: 12 }}>
                空白表示使用 Kong 預設值。查看完整參數：<a href="https://docs.konghq.com/hub/" target="_blank" rel="noreferrer">Kong Hub 文檔</a>
              </span>
            }
          >
            {currentSchema.length > 0 ? (
              <div style={{ display: 'grid', gap: 12 }}>
                {currentSchema.map(field => (
                  <Form.Item
                    key={field.name}
                    name={[field.name]}
                    label={field.label}
                    valuePropName={field.type === 'switch' ? 'checked' : 'value'}
                    style={{ marginBottom: 0 }}
                  >
                    {renderField(field, configForm.getFieldValue(field.name), (val) => configForm.setFieldValue(field.name, val))}
                  </Form.Item>
                ))}
              </div>
            ) : (
              <Input.TextArea
                rows={6}
                placeholder={'{"key": "value"}'}
                onChange={e => {
                  try { JSON.parse(e.target.value); configForm.setFieldValue('config', e.target.value) } catch {}
                }}
              />
            )}
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
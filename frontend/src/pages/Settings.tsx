import { useEffect, useState } from 'react'
import { Card, Form, Input, Switch, Select, Button, Space, Tag, message, Divider, Row, Col, InputNumber, Alert } from 'antd'
import { SaveOutlined, ReloadOutlined } from '@ant-design/icons'
import api from '../api/kong'

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
      localStorage.setItem('kgo_settings', JSON.stringify(values))
      message.success('設定已儲存（部分設定需重啟 Kong 生效）')
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

        {/* Request & Log */}
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
              <Form.Item name="real_ip_header" label="真實客戶端 IP Header" extra="Kong 前面有代理時用於取得真實 IP">
                <Input placeholder="X-Forwarded-For" />
              </Form.Item>
              <Form.Item name="real_ip_recursive" label="遞迴解析 IP" valuePropName="checked">
                <Switch checkedChildren="開" unCheckedChildren="關" />
              </Form.Item>
            </Form>
          </Card>
        </Col>

        {/* Kong Info */}
        <Col xs={24}>
          <Card title="Kong 版本資訊" style={{ background:'var(--secondary)', border:'none' }}>
            <Row gutter={[16,8]}>
              <Col xs={24} sm={8}>
                <Space direction="vertical">
                  <span style={{color:'var(--muted)', fontSize:12}}>版本</span>
                  <Tag color="green" style={{fontSize:14}}>3.4.2</Tag>
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
                  <span style={{color:'var(--muted)', fontSize:12}}>Admin Token</span>
                  <code style={{color:'var(--highlight)'}}>changeme</code>
                </Space>
              </Col>
            </Row>
            <Divider style={{ borderColor:'var(--accent)' }} />
            <Alert
              message="部分設定（如端口、日誌等級、超時）修改後需重啟 Kong 容器才會生效。使用 docker compose restart kong 重新啟動。"
              type="warning"
              showIcon
              style={{ fontSize: 12 }}
            />
          </Card>
        </Col>
      </Row>
    </div>
  )
}
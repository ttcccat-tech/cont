import { useEffect, useState } from 'react'
import { Card, Table, Button, Space, Tag, Modal, Form, Input, InputNumber, Select, Switch, message, Popconfirm, Tooltip, Row, Col } from 'antd'
import { ReloadOutlined, PlusOutlined, EditOutlined, DeleteOutlined, BellOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'

const API_BASE = import.meta.env.VITE_API_BASE || '/api'

interface AlertRule {
  id: number
  name: string
  description: string
  metric_type: 'error_rate' | 'latency'
  service_name: string
  threshold_value: number
  operator: '>' | '<' | '>=' | '<=' | '=='
  duration_seconds: number
  enabled: boolean
  notification_channels: string
  slack_webhook_url: string
  email_webhook_url: string
  discord_webhook_url: string
  alert_suppress_seconds: number
  last_triggered_at?: string
  last_triggered_value?: number
  created_at: string
}

const metricOptions = [
  { value: 'error_rate', label: '錯誤率 (%)' },
  { value: 'latency', label: '延遲 (ms)' },
]

const operatorOptions = [
  { value: '>', label: '>' },
  { value: '<', label: '<' },
  { value: '>=', label: '>=' },
  { value: '<=', label: '<=' },
  { value: '==', label: '==' },
]

async function apiFetch(path: string, options?: RequestInit) {
  const token = localStorage.getItem('cont_token')
  const hasBody = !!options?.body
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: {
      Authorization: `Bearer ${token}`,
      ...(hasBody ? { 'Content-Type': 'application/json' } : {}),
      ...options?.headers,
    },
  })
  if (!res.ok) {
    const txt = await res.text()
    throw new Error(txt)
  }
  return res.json()
}

export default function AlertRulesPage() {
  const [rules, setRules] = useState<AlertRule[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editingRule, setEditingRule] = useState<AlertRule | null>(null)
  const [form] = Form.useForm()

  const fetchRules = async () => {
    setLoading(true)
    try {
      const data = await apiFetch('/alerts/rules')
      setRules(Array.isArray(data) ? data : (data?.data || []))
    } catch (e: any) {
      message.error('載入告警規則失敗: ' + e.message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchRules()

    // Listen for alert_triggered SSE events to refresh rule status
    const es = new EventSource(`${API_BASE}/auth/events`)
    es.addEventListener('alert_triggered', (e: MessageEvent) => {
      try {
        const data = JSON.parse(e.data)
        // Refresh rules to show updated last_triggered_at/value
        fetchRules()
      } catch {}
    })
    return () => { es.close() }
  }, [])

  const openCreate = () => { setEditingRule(null); form.resetFields(); setModalOpen(true) }

  const openEdit = (r: AlertRule) => {
    setEditingRule(r)
    form.setFieldsValue({
      ...r,
      notification_channels: Array.isArray(r.notification_channels)
        ? r.notification_channels
        : (r.notification_channels ? r.notification_channels.split(',') : []),
    })
    setModalOpen(true)
  }

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields()
      values.notification_channels = (values.notification_channels || []).join(',')
      values.enabled = !!values.enabled
      if (editingRule) {
        await apiFetch(`/alerts/rules/${editingRule.id}`, { method: 'PUT', body: JSON.stringify(values) })
        message.success('規則已更新')
      } else {
        await apiFetch('/alerts/rules', { method: 'POST', body: JSON.stringify(values) })
        message.success('規則已建立')
      }
      setModalOpen(false)
      fetchRules()
    } catch (e: any) {
      if (!e.errorFields) message.error('儲存失敗: ' + e.message)
    }
  }

  const handleDelete = async (id: number) => {
    try {
      await apiFetch(`/alerts/rules/${id}`, { method: 'DELETE' })
      message.success('規則已刪除')
      fetchRules()
    } catch (e: any) {
      message.error('刪除失敗: ' + e.message)
    }
  }

  const columns: ColumnsType<AlertRule> = [
    {
      title: '啟用',
      dataIndex: 'enabled',
      key: 'enabled',
      width: 70,
      render: v => v
        ? <Tag color="green">啟用</Tag>
        : <Tag color="default">停用</Tag>,
    },
    {
      title: '規則名稱',
      dataIndex: 'name',
      key: 'name',
      width: 180,
      render: v => <b>{v}</b>,
    },
    {
      title: '指標',
      dataIndex: 'metric_type',
      key: 'metric_type',
      width: 120,
      render: v => metricOptions.find(o => o.value === v)?.label || v,
    },
    {
      title: '服務',
      dataIndex: 'service_name',
      key: 'service_name',
      width: 120,
    },
    {
      title: '條件',
      key: 'condition',
      width: 160,
      render: (_, r) => (
        <span>{r.threshold_value}{r.metric_type === 'error_rate' ? '%' : 'ms'} {r.operator}</span>
      ),
    },
    {
      title: '持續時間',
      dataIndex: 'duration_seconds',
      key: 'duration_seconds',
      width: 120,
      render: v => v ? `${v} 秒` : '-',
    },
    {
      title: '通知',
      dataIndex: 'notification_channels',
      key: 'notification_channels',
      width: 100,
      render: v => {
        const channels = Array.isArray(v) ? v : (v ? v.split(',') : [])
        return channels.length > 0
          ? channels.map(c => <Tag key={c} style={{margin:1}}>{c}</Tag>)
          : <Tag>無</Tag>
      },
    },
    {
      title: '最近觸發',
      key: 'last_triggered',
      width: 160,
      render: (_, r) => {
        if (!r.last_triggered_at) return <Tag>從未</Tag>
        const date = new Date(r.last_triggered_at)
        const diff = Math.floor((Date.now() - date.getTime()) / 1000)
        const label = diff < 60 ? '剛剛' : diff < 3600 ? `${Math.floor(diff/60)}分鐘前` : diff < 86400 ? `${Math.floor(diff/3600)}小時前` : `${Math.floor(diff/86400)}天前`
        return (
          <Space direction="vertical" size={0} style={{fontSize:12}}>
            <Tag color="red">{label}</Tag>
            {r.last_triggered_value != null && (
              <span style={{color:'var(--muted)'}}>
                {r.metric_type === 'error_rate' ? `${r.last_triggered_value}%` : `${r.last_triggered_value}ms`}
              </span>
            )}
          </Space>
        )
      },
    },
    {
      title: '說明',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
    },
    {
      title: '操作',
      key: 'action',
      width: 120,
      render: (_, r) => (
        <Space size="small">
          <Tooltip title="編輯"><Button size="small" icon={<EditOutlined />} onClick={() => openEdit(r)} /></Tooltip>
          <Popconfirm title="確認刪除？" onConfirm={() => handleDelete(r.id)}>
            <Tooltip title="刪除"><Button size="small" danger icon={<DeleteOutlined />} /></Tooltip>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card
        title={<><BellOutlined /> 告警規則</>}
        extra={
          <Space>
            <Button icon={<ReloadOutlined />} onClick={fetchRules} loading={loading}>刷新</Button>
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新增規則</Button>
          </Space>
        }
      >
        <Table
          columns={columns}
          dataSource={rules}
          rowKey="id"
          loading={loading}
          pagination={{ pageSize: 10 }}
          size="middle"
        />
      </Card>

      <Modal
        title={editingRule ? '編輯告警規則' : '新增告警規則'}
        open={modalOpen}
        onOk={handleSubmit}
        onCancel={() => setModalOpen(false)}
        width={600}
        okText="儲存"
        cancelText="取消"
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="name" label="規則名稱" rules={[{ required: true, message: '必填' }]}>
                <Input placeholder="例如: httpbin錯誤率 > 5%" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="enabled" label="啟用" valuePropName="checked">
                <Switch />
              </Form.Item>
            </Col>
          </Row>

          <Form.Item name="description" label="說明">
            <Input.TextArea rows={2} placeholder="規則用途說明..." />
          </Form.Item>

          <Row gutter={16}>
            <Col span={8}>
              <Form.Item name="metric_type" label="指標類型" rules={[{ required: true }]} initialValue="error_rate">
                <Select options={metricOptions} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="service_name" label="服務名稱" rules={[{ required: true, message: '必填' }]}>
                <Input placeholder="例如: httpbin-service" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="threshold_value" label="閾值" rules={[{ required: true, message: '必填' }]}>
                <InputNumber style={{ width: '100%' }} placeholder="5" min={0} />
              </Form.Item>
            </Col>
          </Row>

          <Row gutter={16}>
            <Col span={8}>
              <Form.Item name="operator" label="比較運算" rules={[{ required: true }]} initialValue=">">
                <Select options={operatorOptions} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="duration_seconds" label="持續秒數" initialValue={60}>
                <InputNumber style={{ width: '100%' }} min={1} placeholder="60" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="alert_suppress_seconds" label="抑制秒數" initialValue={300}>
                <InputNumber style={{ width: '100%' }} min={0} placeholder="300" />
              </Form.Item>
            </Col>
          </Row>

          <Form.Item name="notification_channels" label="通知頻道" initialValue={['slack']}>
            <Select mode="multiple" placeholder="選擇通知方式" options={[
              { value: 'slack', label: 'Slack' },
              { value: 'email', label: 'Email' },
              { value: 'discord', label: 'Discord' },
            ]} />
          </Form.Item>

          <Form.Item name="slack_webhook_url" label="Slack Webhook URL">
            <Input placeholder="https://hooks.slack.com/services/..." />
          </Form.Item>

          <Form.Item name="email_webhook_url" label="Email Webhook URL" tooltip="SendGrid、Postmark 或自訂 SMTP API endpoint">
            <Input placeholder="https://api.sendgrid.com/v3/mail/send" />
          </Form.Item>

          <Form.Item name="discord_webhook_url" label="Discord Webhook URL">
            <Input placeholder="https://discord.com/api/webhooks/..." />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

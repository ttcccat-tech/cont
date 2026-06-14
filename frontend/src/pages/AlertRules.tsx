import { useEffect, useState } from 'react'
import { Card, Table, Button, Space, Tag, Modal, Form, Input, InputNumber, Select, Switch, message, Popconfirm, Tooltip, Row, Col, Divider } from 'antd'
import { ReloadOutlined, PlusOutlined, EditOutlined, DeleteOutlined, BellOutlined, HistoryOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'

const API_BASE = import.meta.env.VITE_API_BASE || '/api'

interface Condition {
  metric_type: 'error_rate' | 'latency' | 'usage_quota'
  service_name: string
  threshold_value: number
  operator: '>' | '<' | '>=' | '<=' | '=='
  logic: 'AND' | 'OR'
  quota_metric_type?: 'org' | 'consumer'
  percentage_threshold?: number
}

interface AlertRule {
  id: number
  name: string
  description: string
  conditions: Condition[]
  metric_type: 'error_rate' | 'latency' | 'usage_quota'
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
  { value: 'usage_quota', label: '用量配額 (%)' },
]

const quotaThresholdOptions = [
  { value: 80, label: '80%' },
  { value: 90, label: '90%' },
  { value: 100, label: '100%' },
]

const quotaMetricTypeOptions = [
  { value: 'org', label: '組織 (Org)' },
  { value: 'consumer', label: '消費者 (Consumer)' },
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

  // Stable ref for fetchRules to avoid stale closure in SSE handler
  const fetchRulesRef = React.useRef(fetchRules)
  React.useEffect(() => { fetchRulesRef.current = fetchRules }, [fetchRules])

  useEffect(() => {
    fetchRulesRef.current()

    // Listen for alert_triggered SSE events to refresh rule status
    const es = new EventSource(`${API_BASE}/auth/events`)
    es.addEventListener('alert_triggered', () => {
      // Refresh rules to show updated last_triggered_at/value
      fetchRulesRef.current()
    })
    return () => { es.close() }
  }, [])

  const openCreate = () => { setEditingRule(null); form.resetFields(); setModalOpen(true) }

  const openEdit = (r: AlertRule) => {
    setEditingRule(r)
    const conditions = r.conditions && r.conditions.length > 0
      ? r.conditions
      : [{
          metric_type: r.metric_type || 'error_rate',
          service_name: r.service_name || '',
          threshold_value: r.threshold_value || 0,
          operator: r.operator || '>',
          logic: 'AND',
          quota_metric_type: 'org',
          percentage_threshold: r.threshold_value || 0,
        }]
    form.setFieldsValue({
      ...r,
      conditions,
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
      // Build conditions array from flat fields or use existing conditions
      if (values.conditions && values.conditions.length > 0) {
        // Normalize each condition
        values.conditions = values.conditions.map((c: Condition, idx: number) => ({
          metric_type: c.metric_type || 'error_rate',
          service_name: c.service_name || '',
          threshold_value: c.threshold_value ?? 0,
          operator: c.operator || '>',
          logic: idx === 0 ? 'AND' : (c.logic || 'AND'),
          quota_metric_type: c.quota_metric_type || 'org',
          percentage_threshold: c.threshold_value ?? 0,
        }))
      } else {
        // Fallback: single condition from flat fields
        values.conditions = [{
          metric_type: values.metric_type || 'error_rate',
          service_name: values.service_name || '',
          threshold_value: values.threshold_value ?? 0,
          operator: values.operator || '>',
          logic: 'AND',
          quota_metric_type: 'org',
          percentage_threshold: values.threshold_value ?? 0,
        }]
      }
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
      width: 200,
      render: (_, r) => {
        if (r.conditions && r.conditions.length > 0) {
          const parts = r.conditions.map((c, i) => {
            const logic = i === 0 ? '' : ` ${c.logic} `
            if (c.metric_type === 'usage_quota') {
              return `${logic}用量配額 ${c.operator} ${c.threshold_value}% (${(c as any).quota_metric_type || 'org'})`
            }
            return `${logic}${c.metric_type === 'error_rate' ? '錯誤率' : '延遲'} ${c.operator} ${c.threshold_value}${c.metric_type === 'error_rate' ? '%' : 'ms'}`
          })
          return <Tooltip title={r.conditions.map(c => `${c.metric_type} ${c.operator} ${c.threshold_value} (${c.service_name})`).join(', ')}><span>{r.conditions.length} 條件</span></Tooltip>
        }
        return <span>{r.threshold_value}{r.metric_type === 'error_rate' ? '%' : r.metric_type === 'usage_quota' ? '%' : 'ms'} {r.operator}</span>
      },
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
            <Button icon={<HistoryOutlined />} onClick={() => window.location.href = '/alert-history'}>歷史</Button>
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
        width={700}
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

          <Divider>條件設定</Divider>

          <Form.List name="conditions">
            {(fields, { add, remove }) => (
              <>
                {fields.map(({ key, name, ...rest }, idx) => (
                  <Card key={key} size="small" style={{ marginBottom: 12, background: 'var(--secondary)' }}>
                    <Row gutter={8} align="middle">
                      <Col>
                        <Tag color={idx === 0 ? 'default' : 'blue'} style={{ marginBottom: 0 }}>
                          {idx === 0 ? '第一條件' : `結合 #${idx + 1}`}
                        </Tag>
                      </Col>
                      {idx > 0 && (
                        <Col>
                          <Form.Item {...rest} name={[name, 'logic']} initialValue="AND" style={{ marginBottom: 0 }}>
                            <Select style={{ width: 80 }} options={[
                              { value: 'AND', label: 'AND' },
                              { value: 'OR', label: 'OR' },
                            ]} />
                          </Form.Item>
                        </Col>
                      )}
                      <Col flex="auto" />
                      {idx > 0 && (
                        <Col>
                          <Button type="text" danger size="small" onClick={() => remove(name)}>移除</Button>
                        </Col>
                      )}
                    </Row>
                    <Row gutter={8}>
                      <Col span={7}>
                        <Form.Item {...rest} name={[name, 'metric_type']} label="指標" rules={[{ required: true }]} initialValue="error_rate" style={{ marginBottom: 0 }}>
                          <Select options={metricOptions} />
                        </Form.Item>
                      </Col>
                      <Col span={7}>
                        <Form.Item {...rest} name={[name, 'service_name']} label="服務名稱" rules={[{ required: true }]} style={{ marginBottom: 0 }}>
                          <Input placeholder="httpbin-service" />
                        </Form.Item>
                      </Col>
                      <Col span={5}>
                        <Form.Item {...rest} name={[name, 'operator']} label="比較" rules={[{ required: true }]} initialValue=">" style={{ marginBottom: 0 }}>
                          <Select options={operatorOptions} />
                        </Form.Item>
                      </Col>
                      <Col span={5}>
                        <Form.Item noStyle shouldUpdate={(prev, curr) => prev?.[name]?.metric_type !== curr?.[name]?.metric_type}>
                          {({ getFieldValue }) => {
                            const mType = getFieldValue([name, 'metric_type'])
                            if (mType === 'usage_quota') {
                              return (
                                <Form.Item {...rest} name={[name, 'threshold_value']} label="閾值" rules={[{ required: true }]} initialValue={80} style={{ marginBottom: 0 }}>
                                  <Select options={quotaThresholdOptions} />
                                </Form.Item>
                              )
                            }
                            return (
                              <Form.Item {...rest} name={[name, 'threshold_value']} label="閾值" rules={[{ required: true }]} style={{ marginBottom: 0 }}>
                                <InputNumber style={{ width: '100%' }} min={0} placeholder="5" />
                              </Form.Item>
                            )
                          }}
                        </Form.Item>
                      </Col>
                    </Row>
                    <Form.Item noStyle shouldUpdate={(prev, curr) => prev?.[name]?.metric_type !== curr?.[name]?.metric_type}>
                      {({ getFieldValue }) => {
                        const mType = getFieldValue([name, 'metric_type'])
                        if (mType === 'usage_quota') {
                          return (
                            <Row gutter={8} style={{ marginTop: 8 }}>
                              <Col span={7} offset={7}>
                                <Form.Item {...rest} name={[name, 'quota_metric_type']} label="配額類型" rules={[{ required: true }]} initialValue="org" style={{ marginBottom: 0 }}>
                                  <Select options={quotaMetricTypeOptions} />
                                </Form.Item>
                              </Col>
                            </Row>
                          )
                        }
                        return null
                      }}
                    </Form.Item>
                  </Card>
                ))}
                <Button type="dashed" onClick={() => add({
                  metric_type: 'error_rate',
                  service_name: '',
                  threshold_value: 0,
                  operator: '>',
                  logic: 'AND',
                  quota_metric_type: 'org',
                  percentage_threshold: 0,
                })} block style={{ marginBottom: 16 }}>
                  + 新增條件
                </Button>
              </>
            )}
          </Form.List>

          <Divider>通知設定</Divider>

          <Row gutter={16}>
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
            <Col span={8}>
              <Form.Item name="notification_channels" label="通知頻道" initialValue={['slack']}>
                <Select mode="multiple" placeholder="選擇通知方式" options={[
                  { value: 'slack', label: 'Slack' },
                  { value: 'email', label: 'Email' },
                  { value: 'discord', label: 'Discord' },
                ]} />
              </Form.Item>
            </Col>
          </Row>

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

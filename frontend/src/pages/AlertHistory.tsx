import { useEffect, useState } from 'react'
import { Table, Tag, Space, Card, Typography, Spin, message } from 'antd'
import { ReloadOutlined } from '@ant-design/icons'
import { analyticsClient, getToken } from '../api/kong'

const { Title } = Typography

const API_BASE = import.meta.env.VITE_API_BASE || '/api'

interface AlertHistoryItem {
  id: number
  rule_id: number
  rule_name: string
  org_id: string
  metric_type: string
  operator: string
  threshold: number
  actual_value: number
  triggered_at: string
  message: string
}

export default function AlertHistory() {
  const [data, setData] = useState<AlertHistoryItem[]>([])
  const [loading, setLoading] = useState(true)
  const [total, setTotal] = useState(0)

  const load = async (limit = 50, offset = 0) => {
    setLoading(true)
    try {
      const resp = await analyticsClient.get<{ history: AlertHistoryItem[]; total: number }>(
        `/alerts/history?limit=${limit}&offset=${offset}`
      )
      setData(resp.data.history || [])
      setTotal(resp.data.total || 0)
    } catch (e: unknown) {
      const err = e as { response?: { data?: { error?: string } } }
      message.error(err?.response?.data?.error || '載入告警歷史失敗')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()

    // Listen for SSE alert_triggered events to prepend new history entries immediately
    const token = getToken()
    if (!token) return

    const es = new EventSource(`${API_BASE}/auth/events`)
    es.addEventListener('alert_triggered', (e: MessageEvent) => {
      try {
        const evt = JSON.parse(e.data)
        // Prepend new trigger entry to the top of the list
        const newEntry: AlertHistoryItem = {
          id: Date.now(), // temporary ID; server will assign real ID on next load
          rule_id: evt.rule_id || 0,
          rule_name: evt.rule_name || '未知規則',
          org_id: '',
          metric_type: evt.metric_type || '',
          operator: evt.operator || '>',
          threshold: evt.threshold || 0,
          actual_value: typeof evt.current_value === 'number' ? evt.current_value : 0,
          triggered_at: evt.triggered_at || new Date().toISOString(),
          message: `${evt.rule_name || '規則'} — ${evt.metric_type || ''} ${evt.operator || '>' } ${evt.threshold || 0} (目前: ${evt.current_value})`,
        }
        setData(prev => [newEntry, ...prev])
        setTotal(prev => prev + 1)
        message.info({
          content: `📋 告警歷史已即時更新：${evt.rule_name}`,
          duration: 4,
        })
      } catch {}
    })
    return () => { es.close() }
  }, [])

  const columns = [
    {
      title: '規則名稱',
      dataIndex: 'rule_name',
      key: 'rule_name',
      render: (name: string, record: AlertHistoryItem) => (
        <Space direction="vertical" size={0}>
          <span style={{ fontWeight: 600 }}>{name}</span>
          <span style={{ color: 'var(--muted)', fontSize: 12 }}>ID: {record.rule_id}</span>
        </Space>
      ),
    },
    {
      title: '指標',
      dataIndex: 'metric_type',
      key: 'metric_type',
      render: (v: string) => <Tag color={v === 'error_rate' ? 'red' : 'blue'}>{v}</Tag>,
    },
    {
      title: '條件',
      key: 'condition',
      render: (_: unknown, record: AlertHistoryItem) => (
        <span>
          {record.operator} {record.threshold}
        </span>
      ),
    },
    {
      title: '觸發值',
      dataIndex: 'actual_value',
      key: 'actual_value',
      render: (v: number) => <Tag color="orange">{v.toFixed(4)}</Tag>,
    },
    {
      title: '觸發時間',
      dataIndex: 'triggered_at',
      key: 'triggered_at',
      render: (v: string) => new Date(v).toLocaleString('zh-TW', { timeZone: 'Asia/Taipei' }),
    },
    {
      title: '訊息',
      dataIndex: 'message',
      key: 'message',
      ellipsis: true,
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <div>
          <Title level={4} style={{ margin: 0 }}>告警歷史</Title>
          <span style={{ color: 'var(--muted)', fontSize: 13 }}>共 {total} 筆記錄</span>
        </div>
        <Space>
          <Tag>{data.length} 筆已載入</Tag>
          <ReloadOutlined onClick={() => load()} style={{ cursor: 'pointer', fontSize: 16 }} />
        </Space>
      </div>

      <Card styles={{ body: { padding: 0 } }}>
        {loading ? (
          <div style={{ textAlign: 'center', padding: 48 }}><Spin size="large" /></div>
        ) : (
          <Table
            dataSource={data}
            columns={columns}
            rowKey="id"
            pagination={false}
            size="middle"
          />
        )}
      </Card>
    </div>
  )
}

import { useEffect, useState } from 'react'
import { Table, Tag, Space, Card, Typography, Spin, message, Button, Drawer, Descriptions, Badge, Empty } from 'antd'
import { ReloadOutlined, WifiOutlined, CloseCircleOutlined, SyncOutlined, ClockCircleOutlined } from '@ant-design/icons'
import { analyticsClient, api } from '../api/kong'
import type { WebhookSubscription, WebhookDelivery } from '../api/kong'

const { Title, Text } = Typography

const API_BASE = import.meta.env.VITE_API_BASE || '/api'

export default function WebhookDeliveries() {
  const [subscriptions, setSubscriptions] = useState<WebhookSubscription[]>([])
  const [selectedWebhook, setSelectedWebhook] = useState<WebhookSubscription | null>(null)
  const [deliveries, setDeliveries] = useState<WebhookDelivery[]>([])
  const [loadingSubs, setLoadingSubs] = useState(true)
  const [loadingDeliveries, setLoadingDeliveries] = useState(false)
  const [detailVisible, setDetailVisible] = useState(false)
  const [selectedDelivery, setSelectedDelivery] = useState<WebhookDelivery | null>(null)

  // Load webhook subscriptions
  const loadSubscriptions = async () => {
    setLoadingSubs(true)
    try {
      const data = await api.listWebhooks()
      setSubscriptions(data)
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      message.error(err?.response?.data?.message || '載入 Webhook 失敗')
    } finally {
      setLoadingSubs(false)
    }
  }

  // Load deliveries for a webhook
  const loadDeliveries = async (webhookId: string) => {
    setLoadingDeliveries(true)
    try {
      const data = await api.listWebhookDeliveries(webhookId)
      setDeliveries(data)
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      message.error(err?.response?.data?.message || '載入 Delivery 記錄失敗')
    } finally {
      setLoadingDeliveries(false)
    }
  }

  useEffect(() => {
    loadSubscriptions()
  }, [])

  const handleSelectWebhook = (webhook: WebhookSubscription) => {
    setSelectedWebhook(webhook)
    loadDeliveries(webhook.id)
  }

  const statusConfig: Record<string, { color: string; icon: React.ReactNode; label: string }> = {
    success: { color: 'green', icon: <WifiOutlined />, label: '成功' },
    failed: { color: 'red', icon: <CloseCircleOutlined />, label: '失敗' },
    retrying: { color: 'orange', icon: <SyncOutlined spin />, label: '重試中' },
    pending: { color: 'blue', icon: <ClockCircleOutlined />, label: '待處理' },
  }

  const columns = [
    {
      title: '狀態',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status: string) => {
        const cfg = statusConfig[status] || { color: 'default', icon: null, label: status }
        return <Badge status={cfg.color as 'success' | 'error' | 'processing' | 'default'} text={cfg.label} />
      },
    },
    {
      title: '事件類型',
      dataIndex: 'event_type',
      key: 'event_type',
      width: 160,
      render: (v: string) => <Tag color="blue">{v}</Tag>,
    },
    {
      title: '嘗試次數',
      dataIndex: 'attempts',
      key: 'attempts',
      width: 100,
      render: (n: number) => n === 0 ? <Text type="secondary">—</Text> : <Text>{n} 次</Text>,
    },
    {
      title: '最近嘗試',
      dataIndex: 'last_attempt',
      key: 'last_attempt',
      width: 170,
      render: (v?: string) => v ? new Date(v).toLocaleString('zh-TW', { timeZone: 'Asia/Taipei' }) : <Text type="secondary">—</Text>,
    },
    {
      title: '下次重試',
      dataIndex: 'next_retry',
      key: 'next_retry',
      width: 170,
      render: (v?: string) => v ? (
        <Tag color="orange">{new Date(v).toLocaleString('zh-TW', { timeZone: 'Asia/Taipei' })}</Tag>
      ) : <Text type="secondary">—</Text>,
    },
    {
      title: '錯誤訊息',
      dataIndex: 'last_error',
      key: 'last_error',
      ellipsis: true,
      render: (v?: string) => v ? <Text type="danger" ellipsis={{ tooltip: v }}>{v}</Text> : <Text type="secondary">—</Text>,
    },
    {
      title: '建立時間',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 170,
      render: (v: string) => new Date(v).toLocaleString('zh-TW', { timeZone: 'Asia/Taipei' }),
    },
    {
      title: '',
      key: 'action',
      width: 80,
      render: (_: unknown, record: WebhookDelivery) => (
        <Button size="small" type="link" onClick={() => { setSelectedDelivery(record); setDetailVisible(true) }}>
          詳情
        </Button>
      ),
    },
  ]

  // Summary stats
  const stats = {
    success: deliveries.filter(d => d.status === 'success').length,
    failed: deliveries.filter(d => d.status === 'failed').length,
    retrying: deliveries.filter(d => d.status === 'retrying').length,
    pending: deliveries.filter(d => d.status === 'pending').length,
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <div>
          <Title level={4} style={{ margin: 0 }}>Webhook Deliveries</Title>
          <span style={{ color: 'var(--muted)', fontSize: 13 }}>
            查看所有已訂閱的 webhook delivery 歷史與失敗重試記錄
          </span>
        </div>
        <Space>
          <ReloadOutlined onClick={loadSubscriptions} style={{ cursor: 'pointer', fontSize: 16 }} />
        </Space>
      </div>

      {/* Subscription list */}
      <Card
        title="Webhook 訂閱"
        style={{ marginBottom: 24 }}
        styles={{ body: { padding: 0 } }}
        extra={<Text type="secondary">{subscriptions.length} 個訂閱</Text>}
      >
        {loadingSubs ? (
          <div style={{ textAlign: 'center', padding: 48 }}><Spin /></div>
        ) : subscriptions.length === 0 ? (
          <Empty description="尚無 Webhook 訂閱" style={{ padding: 48 }} />
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column' }}>
            {subscriptions.map(sub => (
              <div
                key={sub.id}
                onClick={() => handleSelectWebhook(sub)}
                style={{
                  padding: '12px 24px',
                  cursor: 'pointer',
                  borderBottom: '1px solid var(--border)',
                  background: selectedWebhook?.id === sub.id ? 'var(--primary-bg)' : 'transparent',
                  display: 'flex',
                  alignItems: 'center',
                  gap: 12,
                }}
              >
                <Tag color={sub.active ? 'green' : 'default'}>{sub.active ? '啟用' : '停用'}</Tag>
                <Text strong style={{ flex: 1 }} ellipsis>{{ ...sub }.url || sub.url}</Text>
                <Space size="small">
                  {sub.event_types.map(t => (
                    <Tag key={t} color="blue" style={{ fontSize: 11 }}>{t}</Tag>
                  ))}
                </Space>
              </div>
            ))}
          </div>
        )}
      </Card>

      {/* Deliveries table */}
      {selectedWebhook && (
        <Card
          title={<Space>
            <span>Delivery 記錄</span>
            <Text type="secondary">—</Text>
            <Text ellipsis style={{ maxWidth: 300 }}>{{ ...selectedWebhook }.url || selectedWebhook.url}</Text>
          </Space>}
          styles={{ body: { padding: 0 } }}
          extra={
            <Space size="middle">
              <Tag color="green">{stats.success} 成功</Tag>
              <Tag color="red">{stats.failed} 失敗</Tag>
              <Tag color="orange">{stats.retrying} 重試中</Tag>
              <Tag color="blue">{stats.pending} 待處理</Tag>
              <ReloadOutlined onClick={() => loadDeliveries(selectedWebhook.id)} style={{ cursor: 'pointer', fontSize: 16 }} />
            </Space>
          }
        >
          {loadingDeliveries ? (
            <div style={{ textAlign: 'center', padding: 48 }}><Spin size="large" /></div>
          ) : deliveries.length === 0 ? (
            <Empty description="尚無 delivery 記錄" style={{ padding: 48 }} />
          ) : (
            <Table
              dataSource={deliveries}
              columns={columns}
              rowKey="id"
              pagination={{ pageSize: 20 }}
              size="middle"
              scroll={{ x: 1000 }}
            />
          )}
        </Card>
      )}

      {/* Delivery detail drawer */}
      <Drawer
        title="Delivery 詳情"
        open={detailVisible}
        onClose={() => setDetailVisible(false)}
        width={560}
      >
        {selectedDelivery && (
          <Descriptions column={1} bordered size="small">
            <Descriptions.Item label="ID">{selectedDelivery.id}</Descriptions.Item>
            <Descriptions.Item label="Webhook ID">{selectedDelivery.webhook_id}</Descriptions.Item>
            <Descriptions.Item label="事件類型">
              <Tag color="blue">{selectedDelivery.event_type}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label="狀態">
              {(() => {
                const cfg = statusConfig[selectedDelivery.status] || { color: 'default', label: selectedDelivery.status }
                return <Badge status={cfg.color as 'success' | 'error' | 'processing' | 'default'} text={cfg.label} />
              })()}
            </Descriptions.Item>
            <Descriptions.Item label="嘗試次數">{selectedDelivery.attempts}</Descriptions.Item>
            <Descriptions.Item label="HTTP 狀態">
              {selectedDelivery.response_status ? (
                <Tag color={selectedDelivery.response_status >= 200 && selectedDelivery.response_status < 300 ? 'green' : 'red'}>
                  {selectedDelivery.response_status}
                </Tag>
              ) : <Text type="secondary">—</Text>}
            </Descriptions.Item>
            <Descriptions.Item label="最近嘗試">
              {selectedDelivery.last_attempt
                ? new Date(selectedDelivery.last_attempt).toLocaleString('zh-TW', { timeZone: 'Asia/Taipei' })
                : '—'}
            </Descriptions.Item>
            <Descriptions.Item label="下次重試">
              {selectedDelivery.next_retry
                ? new Date(selectedDelivery.next_retry).toLocaleString('zh-TW', { timeZone: 'Asia/Taipei' })
                : '—'}
            </Descriptions.Item>
            <Descriptions.Item label="錯誤訊息">
              {selectedDelivery.last_error || '—'}
            </Descriptions.Item>
            <Descriptions.Item label="Response Body">
              <pre style={{ maxHeight: 200, overflow: 'auto', fontSize: 11, background: 'var(--bg)', padding: 8, borderRadius: 4 }}>
                {selectedDelivery.response_body || '—'}
              </pre>
            </Descriptions.Item>
            <Descriptions.Item label="建立時間">
              {new Date(selectedDelivery.created_at).toLocaleString('zh-TW', { timeZone: 'Asia/Taipei' })}
            </Descriptions.Item>
            <Descriptions.Item label="Payload">
              <pre style={{ maxHeight: 200, overflow: 'auto', fontSize: 11, background: 'var(--bg)', padding: 8, borderRadius: 4 }}>
                {(() => {
                  try { return JSON.stringify(JSON.parse(selectedDelivery.payload), null, 2) }
                  catch { return selectedDelivery.payload }
                })()}
              </pre>
            </Descriptions.Item>
          </Descriptions>
        )}
      </Drawer>
    </div>
  )
}

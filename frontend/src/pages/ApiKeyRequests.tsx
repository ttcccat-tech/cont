import { useEffect, useState } from 'react'
import { Card, Table, Button, Space, Tag, Modal, Form, Input, message, Tabs, Popconfirm, Tooltip } from 'antd'
import { PlusOutlined, CheckOutlined, CloseOutlined, ReloadOutlined, KeyOutlined, UserOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { getUserPerms } from '../api/kong'

const API_BASE = import.meta.env.VITE_API_BASE || '/api'

interface ApiKeyRequest {
  id: number
  key_name: string
  consumer_name: string
  description: string
  status: 'pending' | 'approved' | 'rejected'
  applicant_user_id: string
  applicant_username: string
  reviewed_by?: string
  reviewed_at?: string
  created_at: string
  updated_at: string
}

const statusConfig = {
  pending: { color: 'orange', label: '待審批' },
  approved: { color: 'green', label: '已核准' },
  rejected: { color: 'red', label: '已拒絕' },
}

async function apiFetch(path: string, options?: RequestInit) {
  const token = localStorage.getItem('kgo_token')
  const hasBody = !!options?.body
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: {
      Authorization: `Bearer ${token}`,
      ...(hasBody ? { 'Content-Type': 'application/json' } : {}),
      ...options?.headers,
    },
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export default function ApiKeyRequestsPage() {
  const [allRequests, setAllRequests] = useState<ApiKeyRequest[]>([])
  const [myRequests, setMyRequests] = useState<ApiKeyRequest[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [form] = Form.useForm()
  const [submitting, setSubmitting] = useState(false)
  const perms = getUserPerms()
  const isAdmin = perms.users || perms.groups

  const fetchAll = () => {
    setLoading(true)
    apiFetch('/apikeys/requests')
      .then(data => setAllRequests(Array.isArray(data) ? data : data.data || []))
      .catch(() => message.error('載入全部申請失敗'))
      .finally(() => setLoading(false))
  }

  const fetchMine = () => {
    apiFetch('/apikeys/requests/mine')
      .then(data => setMyRequests(Array.isArray(data) ? data : data.data || []))
      .catch(() => message.error('載入我的申請失敗'))
  }

  useEffect(() => {
    if (isAdmin) fetchAll()
    fetchMine()
  }, [])

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields()
      setSubmitting(true)
      await apiFetch('/apikeys/requests', {
        method: 'POST',
        body: JSON.stringify(values),
      })
      message.success('申請已提交，請等待審批')
      setModalOpen(false)
      form.resetFields()
      fetchMine()
    } catch (e: any) {
      if (!e.errorFields) message.error('提交失敗: ' + (e.message || '未知錯誤'))
    } finally {
      setSubmitting(false)
    }
  }

  const handleApprove = async (id: number) => {
    try {
      await apiFetch(`/apikeys/requests/${id}/approve`, { method: 'PUT' })
      message.success('已核准申請')
      fetchAll()
      fetchMine()
    } catch (e: any) {
      message.error('核准失敗: ' + (e.message || '未知錯誤'))
    }
  }

  const handleReject = async (id: number) => {
    try {
      await apiFetch(`/apikeys/requests/${id}/reject`, { method: 'PUT' })
      message.success('已拒絕申請')
      fetchAll()
      fetchMine()
    } catch (e: any) {
      message.error('拒絕失敗: ' + (e.message || '未知錯誤'))
    }
  }

  const columns: ColumnsType<ApiKeyRequest> = [
    {
      title: '狀態',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (v: string) => {
        const cfg = statusConfig[v as keyof typeof statusConfig] || statusConfig.pending
        return <Tag color={cfg.color}>{cfg.label}</Tag>
      },
    },
    {
      title: '金鑰名稱',
      dataIndex: 'key_name',
      key: 'key_name',
      width: 160,
      render: v => <b style={{ color: 'var(--highlight)' }}>{v}</b>,
    },
    {
      title: '消費者名稱',
      dataIndex: 'consumer_name',
      key: 'consumer_name',
      width: 150,
    },
    {
      title: '說明',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
      render: v => <span style={{ color: 'var(--muted)', fontSize: 12 }}>{v || '—'}</span>,
    },
    {
      title: '申請人',
      dataIndex: 'applicant_username',
      key: 'applicant_username',
      width: 120,
      render: v => <Tag icon={<UserOutlined />}>{v || '—'}</Tag>,
    },
    {
      title: '申請時間',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 170,
      render: v => (
        <span style={{ fontFamily: 'monospace', fontSize: 11, color: 'var(--muted)' }}>
          {new Date(v).toLocaleString('zh-TW', { hour12: false })}
        </span>
      ),
    },
    ...(isAdmin ? [{
      title: '操作',
      key: 'action',
      width: 150,
      render: (_: any, r: ApiKeyRequest) => {
        if (r.status !== 'pending') {
          return <span style={{ color: 'var(--muted)', fontSize: 12 }}>已處理</span>
        }
        return (
          <Space size={4}>
            <Tooltip title="核准">
              <Button
                size="small"
                type="link"
                icon={<CheckOutlined />}
                style={{ color: '#52c41a', padding: 0 }}
                onClick={() => handleApprove(r.id)}
              />
            </Tooltip>
            <Popconfirm
              title="確定要拒絕此申請嗎？"
              onConfirm={() => handleReject(r.id)}
              okText="確認"
              cancelText="取消"
            >
              <Tooltip title="拒絕">
                <Button
                  size="small"
                  type="link"
                  icon={<CloseOutlined />}
                  style={{ color: '#ff4d4f', padding: 0 }}
                />
              </Tooltip>
            </Popconfirm>
          </Space>
        )
      },
    }] : []),
  ]

  const myColumns: ColumnsType<ApiKeyRequest> = columns.filter(col => col.key !== 'action')

  const tabItems = [
    {
      key: 'mine',
      label: <span><UserOutlined /> 我的申請</span>,
      children: (
        <div>
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
            <span style={{ color: 'var(--muted)', fontSize: 13 }}>共 {myRequests.length} 筆申請</span>
            <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalOpen(true)}>
              申請 API Key
            </Button>
          </div>
          <Table
            columns={myColumns}
            dataSource={myRequests}
            rowKey="id"
            loading={loading}
            pagination={{ pageSize: 10, size: 'small' }}
            locale={{ emptyText: '尚無申請記錄' }}
            size="small"
          />
        </div>
      ),
    },
    ...(isAdmin ? [{
      key: 'all',
      label: <span><KeyOutlined /> 全部申請</span>,
      children: (
        <div>
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
            <span style={{ color: 'var(--muted)', fontSize: 13 }}>
              待審批: {allRequests.filter(r => r.status === 'pending').length} 筆
            </span>
            <Button icon={<ReloadOutlined />} onClick={fetchAll} loading={loading}>
              刷新
            </Button>
          </div>
          <Table
            columns={columns}
            dataSource={allRequests}
            rowKey="id"
            loading={loading}
            pagination={{ pageSize: 15, size: 'small' }}
            locale={{ emptyText: '尚無申請記錄' }}
            size="small"
          />
        </div>
      ),
    }] : []),
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 24 }}>
        <h1>API Key 申請管理</h1>
        <Space>
          {!isAdmin && (
            <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalOpen(true)}>
              申請 API Key
            </Button>
          )}
        </Space>
      </div>

      <Card style={{ background: 'var(--secondary)', border: 'none' }} bodyStyle={{ padding: 0 }}>
        <Tabs
          items={tabItems}
          defaultActiveKey="mine"
          style={{ padding: '0 16px' }}
        />
      </Card>

      {/* Apply Modal */}
      <Modal
        title="申請 API Key"
        open={modalOpen}
        onOk={handleSubmit}
        confirmLoading={submitting}
        onCancel={() => { setModalOpen(false); form.resetFields() }}
        okText="提交申請"
        cancelText="取消"
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item
            name="key_name"
            label="金鑰名稱"
            rules={[{ required: true, message: '請輸入金鑰名稱' }]}
          >
            <Input placeholder="例如: my-app-prod-key" />
          </Form.Item>
          <Form.Item
            name="service_name"
            label="服務名稱"
          >
            <Input placeholder="例如: httpbin-service（可不填）" />
          </Form.Item>
          <Form.Item
            name="description"
            label="說明"
          >
            <Input.TextArea
              rows={3}
              placeholder="請說明此 API Key 的用途..."
            />
          </Form.Item>
          <Form.Item name="workspace_id" hidden initialValue="433e874d-448b-4cf9-a89f-aaecb947c652">
            <Input type="hidden" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

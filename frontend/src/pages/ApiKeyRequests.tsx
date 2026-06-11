import { useEffect, useState } from 'react'
import { Card, Table, Button, Space, Tag, Modal, Form, Input, message, Tabs, Popconfirm, Tooltip, Select, DatePicker, Alert } from 'antd'
import { PlusOutlined, CheckOutlined, CloseOutlined, ReloadOutlined, KeyOutlined, UserOutlined, CheckCircleOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { getUserPerms } from '../api/kong'
import dayjs from 'dayjs'

const API_BASE = import.meta.env.VITE_API_BASE || '/api'

interface ApiKeyRequest {
  id: number
  key_name: string
  consumer_name: string
  description: string
  reason: string
  scopes: string
  expires_at: string
  status: 'pending' | 'approved' | 'rejected'
  applicant_user_id: string
  applicant_username: string
  reviewed_by?: string
  reviewed_at?: string
  key_value?: string
  created_at: string
  updated_at: string
}

const statusConfig = {
  pending: { color: 'orange', label: '待審批' },
  approved: { color: 'green', label: '已核准' },
  rejected: { color: 'red', label: '已拒絕' },
}

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
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export default function ApiKeyRequestsPage() {
  const [allRequests, setAllRequests] = useState<ApiKeyRequest[]>([])
  const [myRequests, setMyRequests] = useState<ApiKeyRequest[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [keyModalOpen, setKeyModalOpen] = useState(false)
  const [approvedKey, setApprovedKey] = useState('')
  const [form] = Form.useForm()
  const [submitting, setSubmitting] = useState(false)
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([])
  const perms = getUserPerms()
  const isAdmin = perms.users || perms.groups

  const fetchAll = () => {
    setLoading(true)
    apiFetch('/api-keys')
      .then(data => setAllRequests(Array.isArray(data) ? data : data.data || []))
      .catch(() => message.error('載入全部申請失敗'))
      .finally(() => setLoading(false))
  }

  const fetchMine = () => {
    apiFetch('/api-keys/mine')
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
      const payload: any = {
        key_name: values.key_name,
        consumer_name: values.consumer_name || '',
        description: values.description || '',
        reason: values.reason || '',
        scopes: values.scopes || '',
      }
      if (values.expires_at) {
        payload.expires_at = values.expires_at.format('YYYY-MM-DDTHH:mm:ssZ')
      }
      setSubmitting(true)
      await apiFetch('/api-keys', {
        method: 'POST',
        body: JSON.stringify(payload),
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
      const result = await apiFetch(`/api-keys/${id}/approve`, { method: 'PUT' })
      message.success('已核准申請')
      // If key_value returned, show the key modal
      if (result.key_value) {
        setApprovedKey(result.key_value)
        setKeyModalOpen(true)
      }
      fetchAll()
      fetchMine()
    } catch (e: any) {
      message.error('核准失敗: ' + (e.message || '未知錯誤'))
    }
  }

  const handleBatchApprove = async () => {
    if (selectedRowKeys.length === 0) return
    setLoading(true)
    try {
      for (const id of selectedRowKeys) {
        await apiFetch(`/api-keys/${id}/approve`, { method: 'PUT' })
      }
      message.success(`已核准 ${selectedRowKeys.length} 筆申請`)
      setSelectedRowKeys([])
      fetchAll()
      fetchMine()
    } catch (e: any) {
      message.error('批次核准失敗: ' + (e.message || '未知錯誤'))
    } finally {
      setLoading(false)
    }
  }

  const handleBatchReject = async () => {
    if (selectedRowKeys.length === 0) return
    setLoading(true)
    try {
      for (const id of selectedRowKeys) {
        await apiFetch(`/api-keys/${id}/reject`, { method: 'PUT' })
      }
      message.success(`已拒絕 ${selectedRowKeys.length} 筆申請`)
      setSelectedRowKeys([])
      fetchAll()
      fetchMine()
    } catch (e: any) {
      message.error('批次拒絕失敗: ' + (e.message || '未知錯誤'))
    } finally {
      setLoading(false)
    }
  }

  const handleReject = async (id: number) => {
    try {
      await apiFetch(`/api-keys/${id}/reject`, { method: 'PUT' })
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
      title: '申請原因',
      dataIndex: 'reason',
      key: 'reason',
      width: 150,
      ellipsis: true,
      render: v => v || '—',
    },
    {
      title: 'Scopes',
      dataIndex: 'scopes',
      key: 'scopes',
      width: 120,
      ellipsis: true,
      render: v => v ? <Tag>{v}</Tag> : '—',
    },
    {
      title: '消費者名稱',
      dataIndex: 'consumer_name',
      key: 'consumer_name',
      width: 130,
    },
    {
      title: '說明',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
      render: v => <span style={{ color: 'var(--muted)', fontSize: 12 }}>{v || '—'}</span>,
    },
    {
      title: '有效期',
      dataIndex: 'expires_at',
      key: 'expires_at',
      width: 130,
      render: v => v ? (
        <span style={{ fontSize: 11, color: 'var(--muted)' }}>
          {dayjs(v).format('YYYY-MM-DD')}
        </span>
      ) : '—',
    },
    {
      title: '申請人',
      dataIndex: 'applicant_username',
      key: 'applicant_username',
      width: 100,
      render: v => <Tag icon={<UserOutlined />}>{v || '—'}</Tag>,
    },
    {
      title: '申請時間',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 160,
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

  // My requests table: show key_value for approved items
  const myApprovedColumns: ColumnsType<ApiKeyRequest> = myColumns.map(col => {
    if (col.dataIndex === 'key_value' || col.key === 'action') return col
    if (col.dataIndex === 'status') {
      return {
        ...col,
        render: (v: string, r: ApiKeyRequest) => {
          const cfg = statusConfig[v as keyof typeof statusConfig] || statusConfig.pending
          return (
            <Space>
              <Tag color={cfg.color}>{cfg.label}</Tag>
              {v === 'approved' && r.key_value && (
                <Tooltip title="金鑰已核發">
                  <CheckCircleOutlined style={{ color: '#52c41a' }} />
                </Tooltip>
              )}
            </Space>
          )
        },
      }
    }
    return col
  })

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
            columns={myApprovedColumns}
            dataSource={myRequests}
            rowKey="id"
            loading={loading}
            pagination={{ pageSize: 10, size: 'small' }}
            locale={{ emptyText: '尚無申請記錄' }}
            size="small"
            expandable={{
              expandedRowRender: (record) => {
                if (record.status === 'approved' && record.key_value) {
                  return (
                    <div style={{ background: '#f0fff4', padding: '12px 16px', borderRadius: 6 }}>
                      <div style={{ marginBottom: 8, fontWeight: 600, color: '#52c41a' }}>
                        <CheckCircleOutlined /> API Key 已核發
                      </div>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                        <code style={{ background: '#fff', padding: '4px 8px', borderRadius: 4, fontSize: 13, flex: 1, overflow: 'hidden', textOverflow: 'ellipsis' }}>
                          {record.key_value}
                        </code>
                        <Button size="small" icon={<KeyOutlined />} onClick={() => navigator.clipboard.writeText(record.key_value).then(() => message.success('已複製'))}>
                          複製
                        </Button>
                      </div>
                      <div style={{ fontSize: 11, color: '#888', marginTop: 6 }}>
                        請妥善保存，離開此頁後將不再顯示完整金鑰
                      </div>
                    </div>
                  )
                }
                return null
              },
              rowExpandable: (record) => record.status === 'approved' && !!record.key_value,
            }}
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
            <Space>
              <span style={{ color: 'var(--muted)', fontSize: 13 }}>
                待審批: {allRequests.filter(r => r.status === 'pending').length} 筆
                {selectedRowKeys.length > 0 && ` | 已選: ${selectedRowKeys.length} 筆`}
              </span>
              {selectedRowKeys.length > 0 && (
                <>
                  <Button size="small" type="primary" icon={<CheckOutlined />} onClick={handleBatchApprove} loading={loading}>
                    批次核准
                  </Button>
                  <Button size="small" danger icon={<CloseOutlined />} onClick={handleBatchReject} loading={loading}>
                    批次拒絕
                  </Button>
                </>
              )}
            </Space>
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
            rowSelection={{
              selectedRowKeys,
              onChange: (keys) => setSelectedRowKeys(keys),
              getCheckboxProps: (record) => ({
                disabled: record.status !== 'pending',
              }),
            }}
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
        width={520}
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
            name="consumer_name"
            label="Consumer 名稱"
            tooltip="用於關聯到此 Consumer，可不填則自動以金鑰名稱建立"
          >
            <Input placeholder="例如: my-app-consumer（可不填）" />
          </Form.Item>
          <Form.Item
            name="reason"
            label="申請原因"
            rules={[{ required: true, message: '請說明申請原因' }]}
            tooltip="必填，管理員審批時會參考"
          >
            <Select
              placeholder="選擇申請原因類型"
              options={[
                { value: 'production', label: '生產環境使用' },
                { value: 'development', label: '開發/測試環境' },
                { value: 'integration', label: '第三方整合' },
                { value: 'monitoring', label: '監控/分析用途' },
                { value: 'other', label: '其他' },
              ]}
            />
          </Form.Item>
          <Form.Item
            name="scopes"
            label="Scopes（權限範圍）"
            tooltip="可指定此 Key 的 API 存取範圍，如: read:services write:routes"
          >
            <Input placeholder="例如: read:services write:routes（可留空）" />
          </Form.Item>
          <Form.Item
            name="expires_at"
            label="有效期至"
            tooltip="設定金鑰到期日，到期後自動失效（可不填則為永久）"
          >
            <DatePicker
              style={{ width: '100%' }}
              format="YYYY-MM-DD"
              placeholder="選擇到期日（可選）"
              disabledDate={(current) => current && current < dayjs().endOf('day')}
            />
          </Form.Item>
          <Form.Item
            name="description"
            label="說明"
          >
            <Input.TextArea
              rows={3}
              placeholder="請詳細說明此 API Key 的用途與需要的權限..."
            />
          </Form.Item>
          <Form.Item name="workspace_id" hidden initialValue="433e874d-448b-4cf9-a89f-aaecb947c652">
            <Input type="hidden" />
          </Form.Item>
        </Form>
      </Modal>

      {/* Approved Key Display Modal */}
      <Modal
        title={<span style={{ color: '#52c41a' }}><CheckCircleOutlined /> API Key 已核發</span>}
        open={keyModalOpen}
        onOk={() => setKeyModalOpen(false)}
        onCancel={() => setKeyModalOpen(false)}
        okText="我已保存金鑰"
        cancelText="取消"
        footer={(_, { OkBtn, CancelBtn }) => (
          <>
            <CancelBtn />
            <OkBtn />
          </>
        )}
      >
        <Alert
          type="success"
          message="您的 API Key 申請已核准"
          description="以下是您的 API Key，請妥善保存。離開此頁後將不再顯示完整金鑰。"
          style={{ marginBottom: 16 }}
        />
        <div style={{ background: '#f0fff4', padding: '16px', borderRadius: 8, textAlign: 'center' }}>
          <code style={{ fontSize: 16, wordBreak: 'break-all', color: '#237804' }}>
            {approvedKey}
          </code>
        </div>
        <div style={{ marginTop: 12, fontSize: 12, color: '#888', textAlign: 'center' }}>
          金鑰只會在此顯示一次，請複製後妥善保存
        </div>
      </Modal>
    </div>
  )
}